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
	mu        sync.Mutex
	customers map[string]string // idempotency key -> customer
	sessions  map[string]CheckoutSession
	subs      map[string][]billing.Subscription // customer -> subscriptions
	creates   int
	failOnce  bool
	deploy    map[string]string // customer -> francois_deployment tag
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
	defer f.mu.Unlock()
	if s, ok := f.sessions[r.IdempotencyKey]; ok {
		return s, nil
	}
	f.creates++
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
