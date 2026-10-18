package store

import (
	"context"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

func (s *Store) CustomerFor(ctx context.Context, user string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT stripe_customer_id FROM billing_customers WHERE user_id=$1`, user).Scan(&id)
	return id, notFound(err)
}

// SaveCustomer persists the first mapping and returns the stored Customer,
// so a retried creation adopts the same Customer.
func (s *Store) SaveCustomer(ctx context.Context, user, customer string) (string, error) {
	if _, err := s.Pool.Exec(ctx, `INSERT INTO billing_customers(user_id,stripe_customer_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, user, customer); err != nil {
		return "", err
	}
	return s.CustomerFor(ctx, user)
}

func (s *Store) UserForCustomer(ctx context.Context, customer string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `SELECT user_id FROM billing_customers WHERE stripe_customer_id=$1`, customer).Scan(&id)
	return id, notFound(err)
}

func (s *Store) Subscriptions(ctx context.Context, user string) ([]billing.Subscription, error) {
	rows, err := s.Pool.Query(ctx, `SELECT stripe_subscription_id,stripe_customer_id,price_id,status,quantity,item_count,access_until,trial_until,cancel_at_period_end,canceled_at,refreshed_at,support_amount_cents,started_at
FROM billing_subscriptions WHERE user_id=$1 ORDER BY refreshed_at DESC`, user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (billing.Subscription, error) {
		var s billing.Subscription
		err := r.Scan(&s.ID, &s.CustomerID, &s.PriceID, &s.Status, &s.Quantity, &s.ItemCount, &s.AccessUntil, &s.TrialUntil, &s.CancelAtPeriodEnd, &s.CanceledAt, &s.RefreshedAt, &s.SupportAmountCents, &s.StartedAt)
		return s, err
	})
}

// SubscriptionRow is a subscription projection with its customer.
type SubscriptionRow struct {
	billing.Subscription
	UserID string `json:"userId"`
	Email  string `json:"email"`
}

const subscriptionRowColumns = `b.stripe_subscription_id,b.stripe_customer_id,b.price_id,b.status,b.quantity,b.item_count,b.access_until,b.trial_until,b.cancel_at_period_end,b.canceled_at,b.refreshed_at,b.support_amount_cents,b.started_at,b.user_id,u.email`

func scanSubscriptionRow(r pgx.Row) (SubscriptionRow, error) {
	var s SubscriptionRow
	err := r.Scan(&s.ID, &s.CustomerID, &s.PriceID, &s.Status, &s.Quantity, &s.ItemCount, &s.AccessUntil, &s.TrialUntil, &s.CancelAtPeriodEnd, &s.CanceledAt, &s.RefreshedAt, &s.SupportAmountCents, &s.StartedAt, &s.UserID, &s.Email)
	return s, notFound(err)
}

// AllSubscriptions lists subscription projections across customers, newest
// first; status "" matches every status.
func (s *Store) AllSubscriptions(ctx context.Context, status string, limit int) ([]SubscriptionRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.Pool.Query(ctx, `SELECT `+subscriptionRowColumns+` FROM billing_subscriptions b JOIN users u ON u.id=b.user_id
WHERE $1='' OR b.status=$1 ORDER BY b.refreshed_at DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (SubscriptionRow, error) { return scanSubscriptionRow(r) })
}

// SubscriptionByID returns one subscription projection.
func (s *Store) SubscriptionByID(ctx context.Context, id string) (SubscriptionRow, error) {
	return scanSubscriptionRow(s.Pool.QueryRow(ctx, `SELECT `+subscriptionRowColumns+` FROM billing_subscriptions b JOIN users u ON u.id=b.user_id WHERE b.stripe_subscription_id=$1`, id))
}

func (s *Store) Comps(ctx context.Context, user string) ([]billing.Comp, error) {
	grants, err := s.AccessGrants(ctx, user)
	if err != nil {
		return nil, err
	}
	out := make([]billing.Comp, len(grants))
	for i, g := range grants {
		out[i] = billing.Comp{Until: g.Until, Revoked: g.RevokedAt != nil, Plan: g.Plan}
	}
	return out, nil
}

// Access evaluates the pure policy over the user's current records.
func (s *Store) Access(ctx context.Context, p billing.Policy, u User, now time.Time) (billing.Decision, error) {
	subs, err := s.Subscriptions(ctx, u.ID)
	if err != nil {
		return billing.Decision{}, err
	}
	comps, err := s.Comps(ctx, u.ID)
	if err != nil {
		return billing.Decision{}, err
	}
	decision := p.Access(billing.Account{Disabled: u.Disabled}, subs, comps, now)
	year, err := s.YearSupport(ctx, u.ID, now)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return billing.Decision{}, err
	}
	if !u.Disabled && year.Until != nil && (!decision.Allowed || decision.Until == nil || year.Until.After(*decision.Until)) {
		return billing.Decision{Allowed: true, Reason: billing.ReasonSubscriber, Until: year.Until, Status: "active"}, nil
	}
	return decision, nil
}

// UpsertSubscriptions writes projections keyed by Stripe subscription ID. It
// only runs inside the event-completion transaction or a reconciliation.
func upsertSubscriptions(ctx context.Context, tx pgx.Tx, user string, subs []billing.Subscription, live bool) error {
	for _, sub := range subs {
		if _, err := tx.Exec(ctx, `INSERT INTO billing_subscriptions(stripe_subscription_id,user_id,stripe_customer_id,price_id,status,quantity,item_count,access_until,trial_until,cancel_at_period_end,canceled_at,livemode,refreshed_at,support_amount_cents,started_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now(),$13,$14)
ON CONFLICT(stripe_subscription_id) DO UPDATE SET price_id=EXCLUDED.price_id,status=EXCLUDED.status,quantity=EXCLUDED.quantity,item_count=EXCLUDED.item_count,
 access_until=EXCLUDED.access_until,trial_until=EXCLUDED.trial_until,cancel_at_period_end=EXCLUDED.cancel_at_period_end,canceled_at=EXCLUDED.canceled_at,refreshed_at=now(),support_amount_cents=EXCLUDED.support_amount_cents,started_at=EXCLUDED.started_at
WHERE billing_subscriptions.user_id=EXCLUDED.user_id`,
			sub.ID, user, sub.CustomerID, sub.PriceID, sub.Status, sub.Quantity, sub.ItemCount, sub.AccessUntil, sub.TrialUntil, sub.CancelAtPeriodEnd, sub.CanceledAt, live, sub.SupportAmountCents, sub.StartedAt); err != nil {
			return err
		}
	}
	return nil
}

// Checkout attempts.

type CheckoutAttempt struct {
	SupportPaidAt *time.Time `json:"supportPaidAt,omitempty"`
	SupportUntil  *time.Time `json:"supportUntil,omitempty"`
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	Price         string     `json:"price"`
	State         string     `json:"state"`
	SessionID     *string    `json:"sessionId,omitempty"`
	SessionURL    *string    `json:"-"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	ErrorCode     *string    `json:"errorCode,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

const attemptColumns = `id,user_id,price,state,session_id,session_url,expires_at,error_code,created_at,support_paid_at,support_until`

func scanAttempt(r pgx.Row) (CheckoutAttempt, error) {
	var a CheckoutAttempt
	err := r.Scan(&a.ID, &a.UserID, &a.Price, &a.State, &a.SessionID, &a.SessionURL, &a.ExpiresAt, &a.ErrorCode, &a.CreatedAt, &a.SupportPaidAt, &a.SupportUntil)
	return a, notFound(err)
}

// AdmitCheckout returns the user's admitted attempt, creating a pending one
// for price when none is open. created reports whether this call reserved
// it; an existing attempt may be for another price.
func (s *Store) AdmitCheckout(ctx context.Context, user, price string) (CheckoutAttempt, bool, error) {
	// Expire open attempts whose Session lifetime has passed.
	if _, err := s.Pool.Exec(ctx, `UPDATE checkout_attempts SET state='expired',updated_at=now() WHERE user_id=$1 AND state='open' AND expires_at < now()`, user); err != nil {
		return CheckoutAttempt{}, false, err
	}
	a, err := scanAttempt(s.Pool.QueryRow(ctx, `INSERT INTO checkout_attempts(id,user_id,price,state) VALUES($1,$2,$3,'pending') ON CONFLICT DO NOTHING RETURNING `+attemptColumns, NewID("chk_"), user, price))
	if err == nil {
		return a, true, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return a, false, err
	}
	a, err = scanAttempt(s.Pool.QueryRow(ctx, `SELECT `+attemptColumns+` FROM checkout_attempts WHERE user_id=$1 AND state IN ('pending','open')`, user))
	return a, false, err
}

// OpenCheckout records the Session of an admitted attempt. ErrNotFound means
// the attempt was retired meanwhile (superseded by another price).
func (s *Store) OpenCheckout(ctx context.Context, id, session, url string, expires time.Time) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE checkout_attempts SET state='open',session_id=$2,session_url=$3,expires_at=$4,updated_at=now() WHERE id=$1 AND state IN ('pending','open')`, id, session, url, expires)
	if err == nil && tag.RowsAffected() == 0 {
		err = ErrNotFound
	}
	return err
}

func (s *Store) FinishCheckout(ctx context.Context, id, state, code string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE checkout_attempts SET state=$2,error_code=NULLIF($3,''),updated_at=now() WHERE id=$1 AND state IN ('pending','open')`, id, state, code)
	return err
}

func (s *Store) CheckoutBySession(ctx context.Context, session string) (CheckoutAttempt, error) {
	return scanAttempt(s.Pool.QueryRow(ctx, `SELECT `+attemptColumns+` FROM checkout_attempts WHERE session_id=$1`, session))
}

func (s *Store) CheckoutAttempts(ctx context.Context, user string, limit int) ([]CheckoutAttempt, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM checkout_attempts WHERE ($1='' OR user_id=$1) ORDER BY created_at DESC LIMIT $2`, user, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (CheckoutAttempt, error) { return scanAttempt(r) })
}

// DueCheckouts lists pending/open attempts for reconciliation.
func (s *Store) DueCheckouts(ctx context.Context) ([]CheckoutAttempt, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+attemptColumns+` FROM checkout_attempts WHERE state IN ('pending','open') AND updated_at < now() - interval '2 minutes' LIMIT 50`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (CheckoutAttempt, error) { return scanAttempt(r) })
}

// Stripe event inbox.

type StripeEvent struct {
	EventID        string     `json:"eventId"`
	Type           string     `json:"type"`
	CustomerID     string     `json:"customerId"`
	SubscriptionID string     `json:"subscriptionId"`
	SessionID      string     `json:"sessionId"`
	Livemode       bool       `json:"livemode"`
	State          string     `json:"state"`
	Attempts       int        `json:"attempts"`
	ErrorCode      *string    `json:"errorCode,omitempty"`
	NextAttemptAt  time.Time  `json:"nextAttemptAt"`
	ReceivedAt     time.Time  `json:"receivedAt"`
	ProcessedAt    *time.Time `json:"processedAt,omitempty"`
	StripeCreated  *time.Time `json:"stripeCreatedAt,omitempty"`
	LeaseToken     string     `json:"-"`
}

// InsertEvent durably records an event once; duplicates are accepted.
func (s *Store) InsertEvent(ctx context.Context, e StripeEvent) (bool, error) {
	state := "pending"
	if e.State == "ignored" {
		state = "ignored"
	}
	tag, err := s.Pool.Exec(ctx, `INSERT INTO stripe_events(event_id,type,customer_id,subscription_id,session_id,livemode,state,stripe_created_at,processed_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $7='ignored' THEN now() END) ON CONFLICT DO NOTHING`,
		e.EventID, e.Type, e.CustomerID, e.SubscriptionID, e.SessionID, e.Livemode, state, e.StripeCreated)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// EnqueueRefresh inserts a synthetic refresh request for a Customer, used by
// reconciliation and operator retries so both share the webhook path.
func (s *Store) EnqueueRefresh(ctx context.Context, customer, reason string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO stripe_events(event_id,type,customer_id) VALUES($1,$2,$3)`, NewID("refresh_"), "francois.refresh."+reason, customer)
	return err
}

// ClaimEvent leases one due pending event.
func (s *Store) ClaimEvent(ctx context.Context, lease time.Duration) (StripeEvent, error) {
	token := RandomHex(16)
	var e StripeEvent
	err := s.Pool.QueryRow(ctx, `UPDATE stripe_events SET lease_token=$1,lease_until=now()+$2::interval,attempts=attempts+1
WHERE event_id=(SELECT event_id FROM stripe_events WHERE state='pending' AND next_attempt_at<=now() AND (lease_until IS NULL OR lease_until<now())
 ORDER BY next_attempt_at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING event_id,type,customer_id,subscription_id,session_id,livemode,state,attempts`, token, lease.String()).Scan(
		&e.EventID, &e.Type, &e.CustomerID, &e.SubscriptionID, &e.SessionID, &e.Livemode, &e.State, &e.Attempts)
	e.LeaseToken = token
	return e, notFound(err)
}

// CompleteEvent writes the Customer's current subscription projections and
// marks the event processed in one transaction, if the lease is still owned.
func (s *Store) CompleteEvent(ctx context.Context, e StripeEvent, user string, subs []billing.Subscription, years []billing.YearSupport, state string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owned bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM stripe_events WHERE event_id=$1 AND lease_token=$2 FOR UPDATE)`, e.EventID, e.LeaseToken).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return errors.Wrap(ErrConflict, "event lease lost")
	}
	if user != "" {
		if err = upsertSubscriptions(ctx, tx, user, subs, e.Livemode); err != nil {
			return err
		}
	}
	for _, year := range years {
		if _, err = tx.Exec(ctx, `UPDATE checkout_attempts SET support_paid_at=$3,support_until=$4,updated_at=now() WHERE id=$1 AND user_id=$2 AND price=$5`, year.AttemptID, user, year.PaidAt, year.Until, billing.YearFundingKey); err != nil {
			return err
		}
	}

	if _, err = tx.Exec(ctx, `UPDATE stripe_events SET state=$2,processed_at=now(),lease_token=NULL,lease_until=NULL,error_code=NULL WHERE event_id=$1`, e.EventID, state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FailEvent schedules a retry with backoff and a sanitized error code.
func (s *Store) FailEvent(ctx context.Context, e StripeEvent, code string, backoff time.Duration, terminal bool) error {
	state := "pending"
	if terminal {
		state = "error"
	}
	_, err := s.Pool.Exec(ctx, `UPDATE stripe_events SET state=$3,error_code=$4,next_attempt_at=now()+$5::interval,lease_token=NULL,lease_until=NULL WHERE event_id=$1 AND lease_token=$2`,
		e.EventID, e.LeaseToken, state, code, backoff.String())
	return err
}

func (s *Store) RetryEvent(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE stripe_events SET state='pending',next_attempt_at=now(),error_code=NULL WHERE event_id=$1 AND state IN ('error','pending')`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Events(ctx context.Context, state string, limit int) ([]StripeEvent, error) {
	rows, err := s.Pool.Query(ctx, `SELECT event_id,type,customer_id,subscription_id,session_id,livemode,state,attempts,error_code,next_attempt_at,received_at,processed_at,stripe_created_at
FROM stripe_events WHERE $1='' OR state=$1 ORDER BY received_at DESC LIMIT $2`, state, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (StripeEvent, error) {
		var e StripeEvent
		err := r.Scan(&e.EventID, &e.Type, &e.CustomerID, &e.SubscriptionID, &e.SessionID, &e.Livemode, &e.State, &e.Attempts, &e.ErrorCode, &e.NextAttemptAt, &e.ReceivedAt, &e.ProcessedAt, &e.StripeCreated)
		return e, err
	})
}

// BillingHealth summarizes convergence for the admin dashboard.
type BillingHealth struct {
	Pending       int        `json:"pending"`
	Errors        int        `json:"errors"`
	OldestPending *time.Time `json:"oldestPending,omitempty"`
	OpenCheckouts int        `json:"openCheckouts"`
	Customers     int        `json:"customers"`
	ActiveSubs    int        `json:"activeSubscriptions"`
}

func (s *Store) BillingHealth(ctx context.Context) (BillingHealth, error) {
	var h BillingHealth
	err := s.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM stripe_events WHERE state='pending'),
 (SELECT count(*) FROM stripe_events WHERE state='error'),
 (SELECT min(received_at) FROM stripe_events WHERE state='pending'),
 (SELECT count(*) FROM checkout_attempts WHERE state IN ('pending','open')),
 (SELECT count(*) FROM billing_customers),
 (SELECT count(*) FROM billing_subscriptions WHERE status='active')`).Scan(&h.Pending, &h.Errors, &h.OldestPending, &h.OpenCheckouts, &h.Customers, &h.ActiveSubs)
	return h, err
}

// KnownCustomers with nonterminal subscriptions, for scheduled reconciliation.
func (s *Store) CustomersToReconcile(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT stripe_customer_id FROM billing_subscriptions WHERE status NOT IN ('canceled','incomplete_expired') AND refreshed_at < now() - interval '6 hours' UNION SELECT c.stripe_customer_id FROM billing_customers c JOIN checkout_attempts a ON a.user_id=c.user_id WHERE a.support_until>now() AND a.updated_at<now()-interval '6 hours'`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
