package payments

import (
	"context"
	"strconv"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

var (
	// ErrSubscriptionOpen sends the user to billing management instead of a
	// second Checkout.
	ErrSubscriptionOpen = errors.New("an open subscription already exists")
	ErrNoCustomer       = errors.New("no billing account yet")
	ErrCheckoutExpired  = errors.New("the earlier checkout expired; try again")
	ErrForeignSession   = errors.New("checkout session does not belong to this account")
	// ErrCheckoutSuperseded: a concurrent request for another plan replaced
	// this Checkout.
	ErrCheckoutSuperseded = errors.New("checkout superseded by another plan")
)

type Config struct {
	Livemode   bool
	Deployment string // stable idempotency namespace
}

type Service struct {
	Gateway Gateway
	Store   *store.Store
	Config  Config
	wake    chan struct{}
	// OnChange is notified after a user's projection changes (revocation hook).
	OnChange func(user string)
}

func NewService(g Gateway, s *store.Store, cfg Config) *Service {
	return &Service{Gateway: g, Store: s, Config: cfg, wake: make(chan struct{}, 1)}
}

func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// ensureCustomer maps the user to exactly one Customer. A stable idempotency
// key means a retry after a failed database write adopts the same Customer.
func (s *Service) ensureCustomer(ctx context.Context, u store.User) (string, error) {
	id, err := s.Store.CustomerFor(ctx, u.ID)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return "", err
	}
	id, err = s.Gateway.CreateCustomer(ctx, u.ID, s.Config.Deployment, u.Email, u.DisplayName, s.Config.Deployment+"-customer-"+u.ID)
	if err != nil {
		return "", errors.Wrap(err, "create Stripe customer")
	}
	return s.Store.SaveCustomer(ctx, u.ID, id)
}

// Checkout returns a hosted Checkout URL for price that returns to portal
// (the origin the user started from). accepted lists every plan price: an
// open subscription on any of them sends the user to the billing portal
// (plan changes happen there) instead of a second subscription. Double
// clicks and retries for one price resolve to the same admitted
// attempt/Session; a request for another price retires the earlier attempt
// first, so its Session can no longer be paid.
func (s *Service) Checkout(ctx context.Context, u store.User, price string, accepted map[string]bool, portal string) (string, error) {
	return s.checkout(ctx, u, price, accepted, portal, 0, false)
}

// Support creates monthly USD support in the single Subscriber tier.
func (s *Service) Support(ctx context.Context, u store.User, cents int64, accepted map[string]bool, portal string) (string, error) {
	if cents < 100 || cents > 999900 || cents%100 != 0 {
		return "", errors.New("monthly support must be 1–9999 whole US dollars")
	}
	return s.checkout(ctx, u, "support:usd:"+strconv.FormatInt(cents, 10), accepted, portal, cents, false)
}

// FundYear opens a one-time USD payment at Stripe's maximum charge amount.
func (s *Service) FundYear(ctx context.Context, u store.User, portal string) (string, error) {
	return s.checkout(ctx, u, billing.YearFundingKey, nil, portal, 0, true)
}

func (s *Service) checkout(ctx context.Context, u store.User, price string, accepted map[string]bool, portal string, supportAmount int64, yearFunding bool) (string, error) {
	if customer, err := s.Store.CustomerFor(ctx, u.ID); err == nil && !yearFunding {
		subs, err := s.Gateway.CustomerSubscriptions(ctx, customer)
		if err != nil {
			return "", errors.Wrap(err, "refresh subscriptions")
		}
		for _, sub := range subs {
			if (accepted[sub.PriceID] || sub.SupportAmountCents > 0) && billing.OpenStatus(sub.Status) {
				_ = s.Store.EnqueueRefresh(ctx, customer, "checkout")
				s.Wake()
				return "", ErrSubscriptionOpen
			}
		}
	}
	customer, err := s.ensureCustomer(ctx, u)
	if err != nil {
		return "", err
	}
	attempt, _, err := s.Store.AdmitCheckout(ctx, u.ID, price)
	if err != nil {
		return "", err
	}
	if attempt.Price != price {
		if err = s.supersede(ctx, attempt); err != nil {
			return "", err
		}
		if attempt, _, err = s.Store.AdmitCheckout(ctx, u.ID, price); err != nil {
			return "", err
		}
		if attempt.Price != price {
			return "", ErrCheckoutSuperseded
		}
	}
	if attempt.State == "open" && attempt.SessionURL != nil && attempt.ExpiresAt != nil && time.Now().Before(*attempt.ExpiresAt) {
		return *attempt.SessionURL, nil
	}
	// Once a Session ID is known, never recreate it with the old idempotency
	// key: Stripe can evict that key while an async payment is still settling.
	if attempt.SessionID != nil {
		if err = s.supersede(ctx, attempt); err != nil {
			return "", err
		}
		return "", ErrCheckoutExpired
	}
	// Pending: first try or an uncertain earlier try; the same key returns
	// the same Session from Stripe.
	successPath := "/account"
	if supportAmount > 0 || yearFunding {
		successPath = "/thank-you"
	}
	cs, err := s.Gateway.CreateCheckout(ctx, CheckoutRequest{
		CustomerID: customer, PriceID: price, SupportAmountCents: supportAmount, YearFunding: yearFunding, UserID: u.ID, AttemptID: attempt.ID,
		SuccessURL:     portal + successPath + "?checkout={CHECKOUT_SESSION_ID}",
		CancelURL:      portal + "/account",
		IdempotencyKey: s.Config.Deployment + "-checkout-" + attempt.ID,
	})
	if err != nil {
		return "", errors.Wrap(err, "create Checkout session")
	}
	if err = s.Store.OpenCheckout(ctx, attempt.ID, cs.ID, cs.URL, cs.ExpiresAt); errors.Is(err, store.ErrNotFound) {
		// A request for another price retired this attempt while the Session
		// was being created; its URL must not reach the browser.
		if xerr := s.Gateway.ExpireCheckout(ctx, cs.ID); xerr != nil {
			log.Warn().Err(xerr).Str("session", cs.ID).Msg("expire superseded Checkout session")
		}
		return "", ErrCheckoutSuperseded
	}
	if err != nil {
		return "", err
	}
	return cs.URL, nil
}

// supersede retires an admitted attempt for another price. An open Session is
// expired at Stripe; a completed one means a subscription is on its way, so
// the user goes to billing management. A pending attempt without a Session
// is retired as is: if an uncertain try did create one, its URL never reached
// the browser, and OpenCheckout refuses it if that try is still running.
func (s *Service) supersede(ctx context.Context, a store.CheckoutAttempt) error {
	if a.SessionID != nil {
		cs, err := s.Gateway.RetrieveCheckout(ctx, *a.SessionID)
		if err != nil {
			return errors.Wrap(err, "retrieve earlier Checkout session")
		}
		switch cs.Status {
		case "complete":
			_ = s.Store.FinishCheckout(ctx, a.ID, "completed", "")
			if cs.CustomerID != "" {
				_ = s.Store.EnqueueRefresh(ctx, cs.CustomerID, "checkout")
				s.Wake()
			}
			if a.Price == billing.YearFundingKey {
				return store.ErrPaymentPending
			}
			return ErrSubscriptionOpen
		case "open":
			if err = s.Gateway.ExpireCheckout(ctx, cs.ID); err != nil {
				return errors.Wrap(err, "expire earlier Checkout session")
			}
		}
	}
	return s.Store.FinishCheckout(ctx, a.ID, "expired", "superseded")
}

// Portal creates a Customer Portal session for the caller's own Customer that
// returns to portal.
func (s *Service) Portal(ctx context.Context, u store.User, portal string) (string, error) {
	customer, err := s.Store.CustomerFor(ctx, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNoCustomer
	}
	if err != nil {
		return "", err
	}
	return s.Gateway.CreatePortal(ctx, customer, portal+"/account")
}

// Refresh enqueues an owner-scoped reconciliation. An optional Checkout
// session reference must belong to the caller's attempts.
func (s *Service) Refresh(ctx context.Context, u store.User, sessionID string) error {
	if sessionID != "" {
		a, err := s.Store.CheckoutBySession(ctx, sessionID)
		if err != nil || a.UserID != u.ID {
			return ErrForeignSession
		}
	}
	customer, err := s.Store.CustomerFor(ctx, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNoCustomer
	}
	if err != nil {
		return err
	}
	if err = s.Store.EnqueueRefresh(ctx, customer, "account"); err != nil {
		return err
	}
	s.Wake()
	return nil
}

// Ingest verifies a raw webhook body and durably records it.
func (s *Service) Ingest(ctx context.Context, payload []byte, signature string) (Event, error) {
	e, err := s.Gateway.ConstructEvent(payload, signature)
	if err != nil {
		return e, err
	}
	if e.Livemode != s.Config.Livemode {
		return e, errors.New("event mode does not match deployment")
	}
	rec := store.StripeEvent{EventID: e.ID, Type: e.Type, CustomerID: e.CustomerID, SubscriptionID: e.SubscriptionID, SessionID: e.SessionID, Livemode: e.Livemode, StripeCreated: &e.Created}
	if !e.Supported {
		rec.State = "ignored"
	}
	if _, err = s.Store.InsertEvent(ctx, rec); err != nil {
		return e, err
	}
	if e.Supported {
		s.Wake()
	}
	return e, nil
}

// Run drains the durable inbox and reconciles periodically until ctx ends.
func (s *Service) Run(ctx context.Context) error {
	poll := time.NewTicker(5 * time.Second)
	defer poll.Stop()
	reconcile := time.NewTicker(10 * time.Minute)
	defer reconcile.Stop()
	for {
		for {
			worked, err := s.processOne(ctx)
			if err != nil && ctx.Err() == nil {
				log.Warn().Err(err).Msg("billing worker")
			}
			if !worked || ctx.Err() != nil {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.wake:
		case <-poll.C:
		case <-reconcile.C:
			s.reconcile(ctx)
		}
	}
}

func (s *Service) processOne(ctx context.Context) (bool, error) {
	e, err := s.Store.ClaimEvent(ctx, 2*time.Minute)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	backoff := time.Duration(1<<min(e.Attempts, 10)) * 5 * time.Second
	terminal := e.Attempts >= 12
	call, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	customer := e.CustomerID
	if customer == "" && e.SubscriptionID != "" {
		if customer, err = s.Gateway.SubscriptionCustomer(call, e.SubscriptionID); err != nil {
			return true, s.Store.FailEvent(ctx, e, "subscription_lookup_failed", backoff, terminal)
		}
	}
	if customer == "" {
		return true, s.Store.CompleteEvent(ctx, e, "", nil, nil, "ignored")
	}
	user, err := s.Store.UserForCustomer(ctx, customer)
	if errors.Is(err, store.ErrNotFound) {
		// A Stripe account can be shared with other applications and other
		// Francois deployments; their Customers are not ours to retry.
		dep, derr := s.Gateway.CustomerDeployment(call, customer)
		if derr != nil {
			return true, s.Store.FailEvent(ctx, e, "stripe_unavailable", backoff, terminal)
		}
		if dep != s.Config.Deployment {
			return true, s.Store.CompleteEvent(ctx, e, "", nil, nil, "ignored")
		}
		// Never create users from event data; retry in case the mapping
		// write is still in flight, then surface for operators.
		return true, s.Store.FailEvent(ctx, e, "unknown_customer", backoff, terminal)
	}
	if err != nil {
		return true, err
	}
	subs, err := s.Gateway.CustomerSubscriptions(call, customer)
	if err != nil {
		return true, s.Store.FailEvent(ctx, e, "stripe_unavailable", backoff, terminal)
	}
	attempts, err := s.Store.YearCheckouts(call, user)
	if err != nil {
		return true, err
	}
	var years []billing.YearSupport
	for _, attempt := range attempts {
		cs, err := s.Gateway.RetrieveCheckout(call, *attempt.SessionID)
		if err != nil {
			return true, s.Store.FailEvent(ctx, e, "stripe_unavailable", backoff, terminal)
		}
		if cs.CustomerID != customer {
			return true, s.Store.FailEvent(ctx, e, "checkout_owner_mismatch", backoff, terminal)
		}
		year, err := projectYearPayment(attempt.ID, cs)
		if err != nil {
			return true, s.Store.FailEvent(ctx, e, "invalid_year_checkout", backoff, terminal)
		}

		years = append(years, year)
	}

	if e.SessionID != "" {
		if a, aerr := s.Store.CheckoutBySession(ctx, e.SessionID); aerr == nil && a.Price != billing.YearFundingKey {
			state := "completed"
			if e.Type == "checkout.session.expired" {
				state = "expired"
			}
			_ = s.Store.FinishCheckout(ctx, a.ID, state, "")
		}
	}
	if err = s.Store.CompleteEvent(ctx, e, user, subs, years, "processed"); err != nil {
		return true, err
	}
	if s.OnChange != nil {
		s.OnChange(user)
	}
	return true, nil
}

// reconcile re-enqueues refreshes through the same inbox path as webhooks and
// resolves Checkout attempts left pending by crashes or lost returns.
func (s *Service) reconcile(ctx context.Context) {
	customers, err := s.Store.CustomersToReconcile(ctx)
	if err == nil {
		for _, c := range customers {
			_ = s.Store.EnqueueRefresh(ctx, c, "scheduled")
		}
	}
	attempts, err := s.Store.DueCheckouts(ctx)
	if err != nil {
		return
	}
	for _, a := range attempts {
		if a.SessionID == nil {
			if time.Since(a.CreatedAt) > time.Hour {
				_ = s.Store.FinishCheckout(ctx, a.ID, "failed", "creation_uncertain")
			}
			continue
		}
		cs, err := s.Gateway.RetrieveCheckout(ctx, *a.SessionID)
		if err != nil {
			continue
		}
		switch cs.Status {
		case "complete":
			_ = s.Store.FinishCheckout(ctx, a.ID, "completed", "")
			if cs.CustomerID != "" {
				_ = s.Store.EnqueueRefresh(ctx, cs.CustomerID, "checkout")
			}
		case "expired":
			_ = s.Store.FinishCheckout(ctx, a.ID, "expired", "")
		}
	}
	s.Wake()
}

// ReconcileUser is the operator/admin entry point; it uses the same queue.
// SetCancelAtPeriodEnd asks Stripe to end (or keep) a customer's subscription
// at the end of its period, then refreshes the projection through the normal
// event path, so the local state always comes from Stripe.
func (s *Service) SetCancelAtPeriodEnd(ctx context.Context, subscriptionID string, cancel bool) error {
	sub, err := s.Store.SubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return err
	}
	if err = s.Gateway.SetCancelAtPeriodEnd(ctx, subscriptionID, cancel); err != nil {
		return err
	}
	if err = s.Store.EnqueueRefresh(ctx, sub.CustomerID, "operator"); err != nil {
		return err
	}
	s.Wake()
	return nil
}

func (s *Service) ReconcileUser(ctx context.Context, user string) error {
	customer, err := s.Store.CustomerFor(ctx, user)
	if err != nil {
		return err
	}
	if err = s.Store.EnqueueRefresh(ctx, customer, "operator"); err != nil {
		return err
	}
	s.Wake()
	return nil
}
