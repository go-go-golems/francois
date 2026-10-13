package payments

import (
	"context"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var (
	// ErrSubscriptionOpen sends the user to billing management instead of a
	// second Checkout.
	ErrSubscriptionOpen = errors.New("an open subscription already exists")
	ErrNoCustomer       = errors.New("no billing account yet")
	ErrForeignSession   = errors.New("checkout session does not belong to this account")
)

type Config struct {
	PriceID    string
	Livemode   bool
	PublicURL  string
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

func (s *Service) Policy(trial bool) billing.Policy {
	return billing.Policy{AcceptedPrices: map[string]bool{s.Config.PriceID: true}, TrialAllowed: trial}
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
	id, err = s.Gateway.CreateCustomer(ctx, u.ID, u.Email, u.DisplayName, s.Config.Deployment+"-customer-"+u.ID)
	if err != nil {
		return "", errors.Wrap(err, "create Stripe customer")
	}
	return s.Store.SaveCustomer(ctx, u.ID, id)
}

// Checkout returns a hosted Checkout URL for the single configured plan.
// Double clicks and retries resolve to the same admitted attempt/Session.
func (s *Service) Checkout(ctx context.Context, u store.User) (string, error) {
	if customer, err := s.Store.CustomerFor(ctx, u.ID); err == nil {
		subs, err := s.Gateway.CustomerSubscriptions(ctx, customer)
		if err != nil {
			return "", errors.Wrap(err, "refresh subscriptions")
		}
		for _, sub := range subs {
			if sub.PriceID == s.Config.PriceID && billing.OpenStatus(sub.Status) {
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
	attempt, _, err := s.Store.AdmitCheckout(ctx, u.ID, "subscriber")
	if err != nil {
		return "", err
	}
	if attempt.State == "open" && attempt.SessionURL != nil && attempt.ExpiresAt != nil && time.Now().Before(*attempt.ExpiresAt) {
		return *attempt.SessionURL, nil
	}
	// Pending: first try or an uncertain earlier try; the same key returns
	// the same Session from Stripe.
	cs, err := s.Gateway.CreateCheckout(ctx, CheckoutRequest{
		CustomerID: customer, PriceID: s.Config.PriceID, UserID: u.ID, AttemptID: attempt.ID,
		SuccessURL:     s.Config.PublicURL + "/account?checkout={CHECKOUT_SESSION_ID}",
		CancelURL:      s.Config.PublicURL + "/account",
		IdempotencyKey: s.Config.Deployment + "-checkout-" + attempt.ID,
	})
	if err != nil {
		return "", errors.Wrap(err, "create Checkout session")
	}
	if err = s.Store.OpenCheckout(ctx, attempt.ID, cs.ID, cs.URL, cs.ExpiresAt); err != nil {
		return "", err
	}
	return cs.URL, nil
}

// Portal creates a Customer Portal session for the caller's own Customer.
func (s *Service) Portal(ctx context.Context, u store.User) (string, error) {
	customer, err := s.Store.CustomerFor(ctx, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		return "", ErrNoCustomer
	}
	if err != nil {
		return "", err
	}
	return s.Gateway.CreatePortal(ctx, customer, s.Config.PublicURL+"/account")
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
		return true, s.Store.CompleteEvent(ctx, e, "", nil, "ignored")
	}
	user, err := s.Store.UserForCustomer(ctx, customer)
	if errors.Is(err, store.ErrNotFound) {
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
	if e.SessionID != "" {
		if a, aerr := s.Store.CheckoutBySession(ctx, e.SessionID); aerr == nil {
			state := "completed"
			if e.Type == "checkout.session.expired" {
				state = "expired"
			}
			_ = s.Store.FinishCheckout(ctx, a.ID, state, "")
		}
	}
	if err = s.Store.CompleteEvent(ctx, e, user, subs, "processed"); err != nil {
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
