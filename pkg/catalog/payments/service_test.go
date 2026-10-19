package payments

import (
	"context"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

type fakeGateway struct {
	requests  []CheckoutRequest
	mu        sync.Mutex
	customers map[string]string // idempotency key -> customer
	sessions  map[string]CheckoutSession
	subs      map[string][]billing.Subscription // customer -> subscriptions
	creates   int
	failOnce  bool
	deploy    map[string]string // customer -> francois_deployment tag
	// onCreate runs once at the start of the next CreateCheckout, outside the
	// lock, to interleave a concurrent request.
	onCreate func()
}

func newFake() *fakeGateway {
	return &fakeGateway{customers: map[string]string{}, sessions: map[string]CheckoutSession{}, subs: map[string][]billing.Subscription{}, deploy: map[string]string{}}
}

func (f *fakeGateway) CreateCustomer(_ context.Context, user, deployment, _, _, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.customers[key]; ok {
		return c, nil
	}
	c := "cus_" + user
	f.customers[key] = c
	f.deploy[c] = deployment
	return c, nil
}

func (f *fakeGateway) CustomerDeployment(_ context.Context, c string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deploy[c], nil
}

// CreateCheckout honours idempotency keys like Stripe: the same key returns
// the same Session.
func (f *fakeGateway) CreateCheckout(_ context.Context, r CheckoutRequest) (CheckoutSession, error) {
	f.mu.Lock()
	hook := f.onCreate
	f.onCreate = nil
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.sessions[r.IdempotencyKey]; ok {
		return s, nil
	}
	f.creates++
	f.requests = append(f.requests, r)
	s := CheckoutSession{ID: "cs_" + store.RandomHex(4), URL: "https://checkout.test/" + r.AttemptID, Status: "open", CustomerID: r.CustomerID, ExpiresAt: time.Now().Add(time.Hour)}
	f.sessions[r.IdempotencyKey] = s
	if f.failOnce {
		f.failOnce = false
		return CheckoutSession{}, errors.New("network timeout after creation")
	}
	return s, nil
}

func (f *fakeGateway) RetrieveCheckout(_ context.Context, id string) (CheckoutSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return CheckoutSession{}, errors.New("no such session")
}
func (f *fakeGateway) ExpireCheckout(_ context.Context, id string) error {
	return f.setStatus(id, "open", "expired")
}

// setStatus moves a Session from one status to another, like Stripe, which
// only expires open Sessions.
func (f *fakeGateway) setStatus(id, from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, s := range f.sessions {
		if s.ID == id {
			if s.Status != from {
				return errors.Errorf("session %s is %s", id, s.Status)
			}
			s.Status = to
			f.sessions[k] = s
			return nil
		}
	}
	return errors.New("no such session")
}

func (f *fakeGateway) status(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.ID == id {
			return s.Status
		}
	}
	return ""
}

func (f *fakeGateway) CreatePortal(_ context.Context, c, _ string) (string, error) {
	return "https://portal.test/" + c, nil
}
func (f *fakeGateway) SubscriptionCustomer(_ context.Context, id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c, list := range f.subs {
		for _, s := range list {
			if s.ID == id {
				return c, nil
			}
		}
	}
	return "", errors.New("unknown subscription")
}
func (f *fakeGateway) CustomerSubscriptions(_ context.Context, c string) ([]billing.Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]billing.Subscription(nil), f.subs[c]...), nil
}

// SetCancelAtPeriodEnd flips the flag on the fake Stripe subscription, as
// Stripe does; the projection only changes when the worker refreshes.
func (f *fakeGateway) SetCancelAtPeriodEnd(_ context.Context, id string, cancel bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c, subs := range f.subs {
		for i := range subs {
			if subs[i].ID == id {
				f.subs[c][i].CancelAtPeriodEnd = cancel
				return nil
			}
		}
	}
	return errors.New("no such subscription")
}

func (f *fakeGateway) ConstructEvent([]byte, string) (Event, error) {
	return Event{}, errors.New("unused")
}

func testStore(t *testing.T) *store.Store {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "payments_test_" + store.RandomHex(6)
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + name
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	if _, err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

func user(t *testing.T, s *store.Store, name string) store.User {
	u, err := s.UpsertIdentity(context.Background(), store.Identity{Issuer: "https://issuer.test", Subject: name, Email: name + "@example.com", EmailVerified: true, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// accepted is the set of plan prices the tests subscribe to.
var accepted = map[string]bool{"price_ok": true}

func TestCheckoutIdempotencyAndOwnership(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	maya, eve := user(t, s, "maya"), user(t, s, "eve")

	// Two rapid requests resolve to one attempt and one Session.
	var wg sync.WaitGroup
	urls := make([]string, 2)
	for i := range urls {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u, err := svc.Checkout(ctx, maya, "price_ok", accepted, "http://portal.test")
			if err != nil {
				t.Errorf("checkout %d: %v", i, err)
			}
			urls[i] = u
		}(i)
	}
	wg.Wait()
	if urls[0] != urls[1] || g.creates != 1 {
		t.Fatalf("double click produced %d sessions: %v", g.creates, urls)
	}
	attempts, _ := s.CheckoutAttempts(ctx, maya.ID, 10)
	if len(attempts) != 1 {
		t.Fatalf("attempts: %d", len(attempts))
	}

	// A timeout after creation leaves the attempt pending; the retry reuses
	// the same idempotency key and Session.
	g.failOnce = true
	if _, err := svc.Checkout(ctx, eve, "price_ok", accepted, "http://portal.test"); err == nil {
		t.Fatal("expected uncertain failure")
	}
	u, err := svc.Checkout(ctx, eve, "price_ok", accepted, "http://portal.test")
	if err != nil || g.creates != 2 {
		t.Fatalf("retry after timeout: %v creates=%d", err, g.creates)
	}
	if u == "" {
		t.Fatal("no URL after retry")
	}

	// A Session reference from another account is refused.
	mayaAttempt, _ := s.CheckoutAttempts(ctx, maya.ID, 1)
	if err := svc.Refresh(ctx, eve, *mayaAttempt[0].SessionID); !errors.Is(err, ErrForeignSession) {
		t.Fatalf("foreign session accepted: %v", err)
	}

	// An open subscription routes to billing management, not a second purchase.
	customer, _ := s.CustomerFor(ctx, maya.ID)
	until := time.Now().Add(30 * 24 * time.Hour)
	g.subs[customer] = []billing.Subscription{{ID: "sub_a", CustomerID: customer, PriceID: "price_ok", Status: "active", Quantity: 1, ItemCount: 1, AccessUntil: &until}}
	if _, err := svc.Checkout(ctx, maya, "price_ok", accepted, "http://portal.test"); !errors.Is(err, ErrSubscriptionOpen) {
		t.Fatalf("second purchase offered: %v", err)
	}
	// The success return alone never grants access.
	d, _ := s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, maya, time.Now())
	if d.Allowed {
		t.Fatal("access granted before webhook projection")
	}
}

func TestCheckoutIsBoundToPrice(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	both := map[string]bool{"price_month": true, "price_year": true}
	maya, eve := user(t, s, "maya"), user(t, s, "eve")
	checkout := func(u store.User, price string) (string, error) {
		return svc.Checkout(ctx, u, price, both, "http://portal.test")
	}
	sessionOf := func(u store.User) (store.CheckoutAttempt, string) {
		t.Helper()
		a, err := s.CheckoutAttempts(ctx, u.ID, 1)
		if err != nil || len(a) != 1 || a[0].SessionID == nil {
			t.Fatalf("latest attempt: %v %v", a, err)
		}
		return a[0], *a[0].SessionID
	}

	monthly, err := checkout(maya, "price_month")
	if err != nil {
		t.Fatal(err)
	}
	_, monthSession := sessionOf(maya)
	if again, _ := checkout(maya, "price_month"); again != monthly {
		t.Fatalf("same price, new Session: %s %s", again, monthly)
	}

	// Another price retires the open attempt and expires its Session.
	yearly, err := checkout(maya, "price_year")
	if err != nil || yearly == monthly {
		t.Fatalf("switch to yearly: %q %v", yearly, err)
	}
	if st := g.status(monthSession); st != "expired" {
		t.Fatalf("earlier Session still %s", st)
	}
	attempts, _ := s.CheckoutAttempts(ctx, maya.ID, 10)
	if len(attempts) != 2 || attempts[1].State != "expired" || attempts[1].ErrorCode == nil || *attempts[1].ErrorCode != "superseded" || attempts[0].Price != "price_year" {
		t.Fatalf("attempts after switch: %+v", attempts)
	}
	if again, _ := checkout(maya, "price_year"); again != yearly {
		t.Fatalf("same price after switch, new Session: %s %s", again, yearly)
	}

	// A paid earlier Session sends the user to billing management.
	yearAttempt, yearSession := sessionOf(maya)
	if err = g.setStatus(yearSession, "open", "complete"); err != nil {
		t.Fatal(err)
	}
	if _, err = checkout(maya, "price_month"); !errors.Is(err, ErrSubscriptionOpen) {
		t.Fatalf("switch after payment: %v", err)
	}
	if a, _ := s.CheckoutBySession(ctx, yearSession); a.ID != yearAttempt.ID || a.State != "completed" {
		t.Fatalf("paid attempt: %+v", a)
	}

	// A switch while the first Session is still being created: the first
	// request must not hand out its URL, and its Session is expired.
	var second string
	g.onCreate = func() {
		var err error
		if second, err = checkout(eve, "price_year"); err != nil {
			t.Errorf("concurrent switch: %v", err)
		}
	}
	if first, err := checkout(eve, "price_month"); !errors.Is(err, ErrCheckoutSuperseded) {
		t.Fatalf("superseded request returned %q %v", first, err)
	}
	attempts, _ = s.CheckoutAttempts(ctx, eve.ID, 10)
	if len(attempts) != 2 || attempts[0].State != "open" || attempts[0].Price != "price_year" || attempts[0].SessionURL == nil || *attempts[0].SessionURL != second {
		t.Fatalf("attempts after race: %+v", attempts)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, cs := range g.sessions {
		if cs.ID != *attempts[0].SessionID && cs.CustomerID == "cus_"+eve.ID && cs.Status != "expired" {
			t.Fatalf("superseded Session %s is %s", cs.ID, cs.Status)
		}
	}
}

func TestWorkerConvergesOnCurrentState(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	maya := user(t, s, "maya")
	if _, err := s.SaveCustomer(ctx, maya.ID, "cus_maya"); err != nil {
		t.Fatal(err)
	}
	past, future := time.Now().Add(-time.Hour), time.Now().Add(30*24*time.Hour)
	// Old subscription A was canceled; replacement B is active.
	g.subs["cus_maya"] = []billing.Subscription{
		{ID: "sub_a", CustomerID: "cus_maya", PriceID: "price_ok", Status: "canceled", Quantity: 1, ItemCount: 1, AccessUntil: &past},
		{ID: "sub_b", CustomerID: "cus_maya", PriceID: "price_ok", Status: "active", Quantity: 1, ItemCount: 1, AccessUntil: &future},
	}
	// B's event arrives first, then A's delayed deletion; duplicates too.
	for _, e := range []store.StripeEvent{
		{EventID: "evt_b", Type: "customer.subscription.created", CustomerID: "cus_maya", SubscriptionID: "sub_b"},
		{EventID: "evt_a", Type: "customer.subscription.deleted", SubscriptionID: "sub_a"},
		{EventID: "evt_b", Type: "customer.subscription.created", CustomerID: "cus_maya", SubscriptionID: "sub_b"},
	} {
		if _, err := s.InsertEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	for {
		worked, err := svc.processOne(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	d, _ := s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, maya, time.Now())
	if !d.Allowed || d.Reason != billing.ReasonSubscriber {
		t.Fatalf("replacement hidden by old subscription event: %+v", d)
	}
	subs, _ := s.Subscriptions(ctx, maya.ID)
	if len(subs) != 2 {
		t.Fatalf("expected one row per subscription, got %d", len(subs))
	}

	// Customers of other applications or other Francois deployments sharing
	// the Stripe account are ignored at once, without retries.
	g.deploy["cus_foreign"] = "francois-elsewhere"
	for _, id := range []string{"evt_untagged", "evt_foreign"} {
		c := map[string]string{"evt_untagged": "cus_other_app", "evt_foreign": "cus_foreign"}[id]
		if _, err := s.InsertEvent(ctx, store.StripeEvent{EventID: id, Type: "invoice.paid", CustomerID: c}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.processOne(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if events, _ := s.Events(ctx, "pending", 10); len(events) != 0 {
		t.Fatalf("foreign customer events are pending: %+v", events)
	}

	// Our own Customer without a local mapping never creates a user; the
	// event is retried in case the mapping write is still in flight.
	g.deploy["cus_unknown"] = "test"
	if _, err := s.InsertEvent(ctx, store.StripeEvent{EventID: "evt_x", Type: "invoice.paid", CustomerID: "cus_unknown"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.processOne(ctx); err != nil {
		t.Fatal(err)
	}
	events, _ := s.Events(ctx, "pending", 10)
	if len(events) != 1 || events[0].ErrorCode == nil || *events[0].ErrorCode != "unknown_customer" {
		t.Fatalf("unknown customer event: %+v", events)
	}

	// Terminal cancellation removes access on the next refresh.
	g.subs["cus_maya"][1].Status = "canceled"
	if err := svc.ReconcileUser(ctx, maya.ID); err != nil {
		t.Fatal(err)
	}
	for {
		worked, _ := svc.processOne(ctx)
		if !worked {
			break
		}
	}
	d, _ = s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, maya, time.Now())
	if d.Allowed {
		t.Fatalf("canceled subscription still grants access: %+v", d)
	}
}

func TestSetCancelAtPeriodEnd(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	maya := user(t, s, "maya")
	if _, err := s.SaveCustomer(ctx, maya.ID, "cus_maya"); err != nil {
		t.Fatal(err)
	}
	g.deploy["cus_maya"] = "test"
	future := time.Now().Add(30 * 24 * time.Hour)
	g.subs["cus_maya"] = []billing.Subscription{{ID: "sub_m", CustomerID: "cus_maya", PriceID: "price_ok", Status: "active", Quantity: 1, ItemCount: 1, AccessUntil: &future}}
	drain := func() {
		for {
			worked, err := svc.processOne(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !worked {
				return
			}
		}
	}
	if err := svc.ReconcileUser(ctx, maya.ID); err != nil {
		t.Fatal(err)
	}
	drain()

	for _, cancel := range []bool{true, false} {
		if err := svc.SetCancelAtPeriodEnd(ctx, "sub_m", cancel); err != nil {
			t.Fatal(err)
		}
		drain()
		sub, err := s.SubscriptionByID(ctx, "sub_m")
		if err != nil || sub.CancelAtPeriodEnd != cancel || sub.UserID != maya.ID {
			t.Fatalf("cancel=%v: projection %+v %v", cancel, sub, err)
		}
		// Access continues until the period ends either way.
		if d, _ := s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, maya, time.Now()); !d.Allowed {
			t.Fatalf("cancel=%v removed access before the period end: %+v", cancel, d)
		}
	}
	if err := svc.SetCancelAtPeriodEnd(ctx, "sub_missing", true); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown subscription: %v", err)
	}
	rows, err := s.AllSubscriptions(ctx, "active", 10)
	if err != nil || len(rows) != 1 || rows[0].Email == "" {
		t.Fatalf("all subscriptions %+v %v", rows, err)
	}
}

func TestMonthlySupportCheckout(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	u := user(t, s, "support")
	for _, amount := range []int64{0, 99, 150, 1000000} {
		if _, err := svc.Support(ctx, u, amount, accepted, "https://portal.test"); err == nil {
			t.Fatalf("accepted %d", amount)
		}
	}
	first, err := svc.Support(ctx, u, 4200, accepted, "https://portal.test")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Support(ctx, u, 4200, accepted, "https://portal.test")
	if err != nil || again != first || g.creates != 1 {
		t.Fatal("support checkout retry not idempotent", err)
	}
	r := g.requests[0]
	if r.SupportAmountCents != 4200 || r.SuccessURL != "https://portal.test/thank-you?checkout={CHECKOUT_SESSION_ID}" {
		t.Fatalf("wrong checkout: %+v", r)
	}
	_, err = svc.Support(ctx, u, 4300, accepted, "https://portal.test")
	if err != nil || g.creates != 2 {
		t.Fatal("amount change failed", err)
	}
	if g.status(g.sessions[r.IdempotencyKey].ID) != "expired" {
		t.Fatal("earlier amount remains payable")
	}
	customer, err := s.CustomerFor(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	g.subs[customer] = []billing.Subscription{{ID: "sub_support", SupportAmountCents: 4300, Status: "active"}}
	if _, err = svc.Support(ctx, u, 4400, accepted, "https://portal.test"); !errors.Is(err, ErrSubscriptionOpen) {
		t.Fatal("duplicate contribution allowed", err)
	}
}

func TestYearFundingConfirmationAndRefund(t *testing.T) {
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	ctx := context.Background()
	u := user(t, s, "year-funder")
	first, err := svc.FundYear(ctx, u, "https://portal.test")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.FundYear(ctx, u, "https://portal.test")
	if err != nil || first != again || g.creates != 1 {
		t.Fatal("year checkout retry not idempotent", err)
	}
	req := g.requests[0]
	if !req.YearFunding || req.SupportAmountCents != 0 {
		t.Fatal("wrong year checkout choice")
	}
	cs := g.sessions[req.IdempotencyKey]
	cs.Status = "complete"
	cs.YearFunding = true
	paidAt := time.Now().UTC().Truncate(time.Second)
	cs.PaidAt = &paidAt
	refresh := func() {
		t.Helper()
		if err := svc.Refresh(ctx, u, cs.ID); err != nil {
			t.Fatal(err)
		}
		if did, err := svc.processOne(ctx); err != nil || !did {
			t.Fatal("year refresh failed", err)
		}
	}
	g.sessions[req.IdempotencyKey] = cs
	refresh()
	if _, err = s.YearSupport(ctx, u.ID, time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("unpaid checkout granted standing", err)
	}
	cs.PaymentPaid = true
	g.sessions[req.IdempotencyKey] = cs
	refresh()
	standing, err := s.YearSupport(ctx, u.ID, time.Now())
	if err != nil || standing.Until == nil || !standing.Until.Equal(paidAt.AddDate(1, 0, 0)) {
		t.Fatal("paid year not projected", err)
	}
	if _, err = svc.FundYear(ctx, u, "https://portal.test"); !errors.Is(err, store.ErrYearFunded) {
		t.Fatal("overlapping year funding admitted", err)
	}
	refresh()
	replayed, err := s.YearSupport(ctx, u.ID, time.Now())
	if err != nil || !replayed.Until.Equal(*standing.Until) {
		t.Fatal("replay extended year", err)
	}
	decision, err := s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, u, time.Now())
	if err != nil || !decision.Allowed || decision.Reason != billing.ReasonSubscriber {
		t.Fatal("year not in subscriber tier", err)
	}
	cs.Refunded = true
	g.sessions[req.IdempotencyKey] = cs
	refresh()
	if _, err = s.YearSupport(ctx, u.ID, time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("refunded year kept standing", err)
	}
	decision, err = s.Access(ctx, billing.Policy{AcceptedPrices: accepted}, u, time.Now())
	if err != nil || decision.Allowed {
		t.Fatal("refund retained paid access", err)
	}
	other := user(t, s, "monthly-year-funder")
	customer, err := svc.ensureCustomer(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	g.subs[customer] = []billing.Subscription{{ID: "monthly", SupportAmountCents: 4200, Status: "active"}}
	if _, err = svc.FundYear(ctx, other, "https://portal.test"); err != nil {
		t.Fatal("monthly subscriber cannot fund year", err)
	}
}
