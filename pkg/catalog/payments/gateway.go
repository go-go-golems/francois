// Package payments integrates Stripe Customers, hosted Checkout, the
// Customer Portal and signed webhooks. Webhooks are durable refresh requests;
// the worker always retrieves current Customer state before projecting it.
package payments

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/pkg/errors"
	stripe "github.com/stripe/stripe-go/v86"
)

type CheckoutRequest struct {
	YearFunding                            bool
	SupportAmountCents                     int64
	CustomerID, PriceID, UserID, AttemptID string
	SuccessURL, CancelURL, IdempotencyKey  string
}

type CheckoutSession struct {
	YearFunding bool
	PaymentPaid bool
	Refunded    bool
	AmountCents int64
	PaidAt      *time.Time
	ID          string
	URL         string
	Status      string // open|complete|expired
	CustomerID  string
	ExpiresAt   time.Time
}

// Event is the minimal routing data extracted from a verified webhook.
type Event struct {
	ID, Type                              string
	Livemode                              bool
	Created                               time.Time
	CustomerID, SubscriptionID, SessionID string
	Supported                             bool
}

// Gateway is the external Stripe boundary; tests supply a fake.
type Gateway interface {
	CreateCustomer(ctx context.Context, userID, deployment, email, name, idempotencyKey string) (string, error)
	// CustomerDeployment returns the francois_deployment tag of a Customer,
	// empty for Customers this service did not create.
	CustomerDeployment(ctx context.Context, customerID string) (string, error)
	CreateCheckout(ctx context.Context, r CheckoutRequest) (CheckoutSession, error)
	RetrieveCheckout(ctx context.Context, id string) (CheckoutSession, error)
	// ExpireCheckout closes an open Session so it can no longer be paid.
	ExpireCheckout(ctx context.Context, id string) error
	CreatePortal(ctx context.Context, customerID, returnURL string) (string, error)
	SubscriptionCustomer(ctx context.Context, subscriptionID string) (string, error)
	// SetCancelAtPeriodEnd schedules (or withdraws) the end of a subscription
	// at the end of its current period. It moves no money.
	SetCancelAtPeriodEnd(ctx context.Context, subscriptionID string, cancel bool) error
	CustomerSubscriptions(ctx context.Context, customerID string) ([]billing.Subscription, error)
	ConstructEvent(payload []byte, signature string) (Event, error)
}

type stripeGateway struct {
	client  *stripe.Client
	secrets []string
}

var _ Gateway = (*stripeGateway)(nil)

// NewStripe builds the production gateway. Webhook secrets support rotation
// (current and previous).
func NewStripe(secretKey string, webhookSecrets []string) (Gateway, error) {
	if !strings.HasPrefix(secretKey, "sk_") && !strings.HasPrefix(secretKey, "rk_") {
		return nil, errors.New("Stripe secret key must be an sk_ or rk_ key")
	}
	for _, s := range webhookSecrets {
		if !strings.HasPrefix(s, "whsec_") {
			return nil, errors.New("Stripe webhook secrets must be whsec_ values")
		}
	}
	return &stripeGateway{client: stripe.NewClient(secretKey), secrets: webhookSecrets}, nil
}

func (g *stripeGateway) CreateCustomer(ctx context.Context, userID, deployment, email, name, key string) (string, error) {
	p := &stripe.CustomerCreateParams{Name: stripe.String(name), Metadata: map[string]string{"francois_user_id": userID, "francois_deployment": deployment}}
	if email != "" {
		p.Email = stripe.String(email)
	}
	p.SetIdempotencyKey(key)
	c, err := g.client.V1Customers.Create(ctx, p)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func session(s *stripe.CheckoutSession) CheckoutSession {
	out := CheckoutSession{ID: s.ID, URL: s.URL, Status: string(s.Status), ExpiresAt: time.Unix(s.ExpiresAt, 0).UTC()}
	if s.Customer != nil {
		out.CustomerID = s.Customer.ID
	}
	out.AmountCents = s.AmountTotal
	out.YearFunding = s.Mode == stripe.CheckoutSessionModePayment && s.Currency == stripe.CurrencyUSD && s.AmountTotal == billing.YearFundingCents && s.Metadata["francois_year_funding"] == "true"
	out.PaymentPaid = s.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid
	if s.PaymentIntent != nil && s.PaymentIntent.LatestCharge != nil {
		c := s.PaymentIntent.LatestCharge
		out.Refunded = c.Refunded || (c.Amount > 0 && c.AmountRefunded >= c.Amount) || c.Disputed
		if c.Created > 0 {
			t := time.Unix(c.Created, 0).UTC()
			out.PaidAt = &t
		}
	}
	return out
}

func (g *stripeGateway) CreateCheckout(ctx context.Context, r CheckoutRequest) (CheckoutSession, error) {
	p := &stripe.CheckoutSessionCreateParams{
		Customer:          stripe.String(r.CustomerID),
		ClientReferenceID: stripe.String(r.UserID),
		Mode:              stripe.String(stripe.CheckoutSessionModeSubscription),
		SuccessURL:        stripe.String(r.SuccessURL),
		CancelURL:         stripe.String(r.CancelURL),
		LineItems:         []*stripe.CheckoutSessionCreateLineItemParams{{Price: stripe.String(r.PriceID), Quantity: stripe.Int64(1)}},
		Metadata:          map[string]string{"francois_user_id": r.UserID, "checkout_attempt_id": r.AttemptID},
		SubscriptionData:  &stripe.CheckoutSessionCreateSubscriptionDataParams{Metadata: map[string]string{"francois_user_id": r.UserID}},
	}
	if r.SupportAmountCents > 0 {
		p.LineItems = []*stripe.CheckoutSessionCreateLineItemParams{{Quantity: stripe.Int64(1), PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
			Currency: stripe.String("usd"), UnitAmount: stripe.Int64(r.SupportAmountCents),
			Recurring:   &stripe.CheckoutSessionCreateLineItemPriceDataRecurringParams{Interval: stripe.String("month")},
			ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{Name: stripe.String("Support slop.boutique")},
		}}}
		p.SubscriptionData.Metadata["francois_monthly_support"] = "true"
	}
	if r.YearFunding {
		p.Mode = stripe.String(stripe.CheckoutSessionModePayment)
		p.SubscriptionData = nil
		p.LineItems = []*stripe.CheckoutSessionCreateLineItemParams{{Quantity: stripe.Int64(1), PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{Currency: stripe.String("usd"), UnitAmount: stripe.Int64(billing.YearFundingCents), ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{Name: stripe.String("Fund a year of slop.boutique")}}}}
		p.Metadata["francois_year_funding"] = "true"
		p.InvoiceCreation = &stripe.CheckoutSessionCreateInvoiceCreationParams{Enabled: stripe.Bool(true)}
	}

	p.SetIdempotencyKey(r.IdempotencyKey)
	s, err := g.client.V1CheckoutSessions.Create(ctx, p)
	if err != nil {
		return CheckoutSession{}, err
	}
	return session(s), nil
}

func (g *stripeGateway) RetrieveCheckout(ctx context.Context, id string) (CheckoutSession, error) {
	p := &stripe.CheckoutSessionRetrieveParams{}
	p.AddExpand("payment_intent.latest_charge")
	s, err := g.client.V1CheckoutSessions.Retrieve(ctx, id, p)
	if err != nil {
		return CheckoutSession{}, err
	}
	return session(s), nil
}

func (g *stripeGateway) ExpireCheckout(ctx context.Context, id string) error {
	_, err := g.client.V1CheckoutSessions.Expire(ctx, id, &stripe.CheckoutSessionExpireParams{})
	return err
}

func (g *stripeGateway) SetCancelAtPeriodEnd(ctx context.Context, id string, cancel bool) error {
	_, err := g.client.V1Subscriptions.Update(ctx, id, &stripe.SubscriptionUpdateParams{CancelAtPeriodEnd: stripe.Bool(cancel)})
	return err
}

func (g *stripeGateway) CreatePortal(ctx context.Context, customer, returnURL string) (string, error) {
	s, err := g.client.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{Customer: stripe.String(customer), ReturnURL: stripe.String(returnURL)})
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (g *stripeGateway) CustomerDeployment(ctx context.Context, id string) (string, error) {
	c, err := g.client.V1Customers.Retrieve(ctx, id, &stripe.CustomerRetrieveParams{})
	if err != nil {
		return "", err
	}
	return c.Metadata["francois_deployment"], nil
}

func (g *stripeGateway) SubscriptionCustomer(ctx context.Context, id string) (string, error) {
	s, err := g.client.V1Subscriptions.Retrieve(ctx, id, &stripe.SubscriptionRetrieveParams{})
	if err != nil {
		return "", err
	}
	if s.Customer == nil {
		return "", errors.New("subscription has no customer")
	}
	return s.Customer.ID, nil
}

// Project converts one Stripe subscription into the local projection using
// item-level billing periods.
func Project(s *stripe.Subscription) billing.Subscription {
	out := billing.Subscription{ID: s.ID, Status: string(s.Status), CancelAtPeriodEnd: s.CancelAtPeriodEnd || s.CancelAt > 0}
	if s.Customer != nil {
		out.CustomerID = s.Customer.ID
	}
	if s.Items != nil {
		out.ItemCount = len(s.Items.Data)
		if len(s.Items.Data) == 1 {
			item := s.Items.Data[0]
			if item.Price != nil {
				out.PriceID = item.Price.ID
				if s.Metadata["francois_monthly_support"] == "true" && item.Price.Currency == stripe.CurrencyUSD && item.Price.Recurring != nil && item.Price.Recurring.Interval == stripe.PriceRecurringIntervalMonth && item.Price.Recurring.IntervalCount == 1 && item.Quantity == 1 {
					out.SupportAmountCents = item.Price.UnitAmount
				}
			}
			out.Quantity = int(item.Quantity)
			if item.CurrentPeriodEnd > 0 {
				t := time.Unix(item.CurrentPeriodEnd, 0).UTC()
				out.AccessUntil = &t
			}
		}
	}
	if s.StartDate > 0 {
		t := time.Unix(s.StartDate, 0).UTC()
		out.StartedAt = &t
	}
	if s.TrialEnd > 0 {
		t := time.Unix(s.TrialEnd, 0).UTC()
		out.TrialUntil = &t
	}
	if s.CanceledAt > 0 {
		t := time.Unix(s.CanceledAt, 0).UTC()
		out.CanceledAt = &t
	}
	return out
}

// CustomerSubscriptions retrieves every subscription of the Customer in any
// status, following pagination, so replacements and old rows both converge.
func (g *stripeGateway) CustomerSubscriptions(ctx context.Context, customer string) ([]billing.Subscription, error) {
	out := []billing.Subscription{}
	list := g.client.V1Subscriptions.List(ctx, &stripe.SubscriptionListParams{Customer: stripe.String(customer), Status: stripe.String("all")})
	for s, err := range list.All(ctx) {
		if err != nil {
			return nil, err
		}
		out = append(out, Project(s))
	}
	return out, nil
}

var supported = map[string]bool{
	"charge.refunded":               true,
	"customer.subscription.created": true, "customer.subscription.updated": true, "customer.subscription.deleted": true,
	"customer.subscription.paused": true, "customer.subscription.resumed": true,
	"checkout.session.completed": true, "checkout.session.expired": true,
	"checkout.session.async_payment_succeeded": true, "checkout.session.async_payment_failed": true,
	"invoice.paid": true, "invoice.payment_failed": true, "invoice.payment_action_required": true, "invoice.finalization_failed": true,
}

func (g *stripeGateway) ConstructEvent(payload []byte, signature string) (Event, error) {
	var last error
	for _, secret := range g.secrets {
		e, err := stripe.ConstructEvent(payload, signature, secret)
		if err == nil {
			return Extract(e), nil
		}
		last = err
	}
	return Event{}, errors.Wrap(last, "invalid Stripe signature")
}

// Extract classifies a verified event and pulls out safe routing IDs.
func Extract(e stripe.Event) Event {
	out := Event{ID: e.ID, Type: string(e.Type), Livemode: e.Livemode, Created: time.Unix(e.Created, 0).UTC(), Supported: supported[string(e.Type)]}
	if !out.Supported || e.Data == nil {
		return out
	}
	var obj struct {
		ID           string          `json:"id"`
		Object       string          `json:"object"`
		Customer     json.RawMessage `json:"customer"`
		Subscription json.RawMessage `json:"subscription"`
		Parent       *struct {
			SubscriptionDetails *struct {
				Subscription json.RawMessage `json:"subscription"`
			} `json:"subscription_details"`
		} `json:"parent"`
	}
	if json.Unmarshal(e.Data.Raw, &obj) != nil {
		return out
	}
	out.CustomerID = expandable(obj.Customer)
	switch obj.Object {
	case "subscription":
		out.SubscriptionID = obj.ID
	case "checkout.session":
		out.SessionID = obj.ID
		out.SubscriptionID = expandable(obj.Subscription)
	case "invoice":
		out.SubscriptionID = expandable(obj.Subscription)
		if obj.Parent != nil && obj.Parent.SubscriptionDetails != nil {
			out.SubscriptionID = expandable(obj.Parent.SubscriptionDetails.Subscription)
		}
	}
	return out
}

func expandable(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var id string
	if json.Unmarshal(raw, &id) == nil {
		return id
	}
	var obj struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &obj)
	return obj.ID
}
