package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	stripe "github.com/stripe/stripe-go/v86"
)

func TestEnterpriseAmount(t *testing.T) {
	for dollars, want := range map[int64]bool{4: false, 7: false, 8: true, 12: false, 16: true, 256: true, 300: false, 4096: true, 8192: false} {
		if got := billing.EnterpriseAmount(dollars * 100); got != want {
			t.Errorf("%d USD: %v, want %v", dollars, got, want)
		}
	}
	if billing.EnterpriseAmount(25650) || !billing.EnterpriseAmount(billing.EnterpriseDefaultCents) {
		t.Error("cents or default wrong")
	}
}

func TestEnterpriseSupportStripePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		for key, want := range map[string]string{
			"mode": "subscription", "line_items[0][price_data][unit_amount]": "102400", "line_items[0][price_data][recurring][interval]": "month",
			"line_items[0][price_data][product_data][name]":         "Enterprise support",
			"subscription_data[metadata][francois_monthly_support]": "true", "subscription_data[metadata][francois_support_tier]": "enterprise",
			"tax_id_collection[enabled]": "true", "billing_address_collection": "required", "customer_update[name]": "auto", "customer_update[address]": "auto",
		} {
			if got := r.Form.Get(key); got != want {
				t.Errorf("%s=%q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_enterprise","url":"https://checkout.test/enterprise","status":"open"}`))
	}))
	defer server.Close()
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(server.URL), HTTPClient: server.Client()})
	g := &stripeGateway{client: stripe.NewClient("sk_test_fixture", stripe.WithBackends(&stripe.Backends{API: backend}))}
	if _, err := g.CreateCheckout(context.Background(), CheckoutRequest{CustomerID: "cus_fixture", SupportAmountCents: 102400, SupportTier: billing.TierEnterprise, IdempotencyKey: "attempt-enterprise"}); err != nil {
		t.Fatal(err)
	}
}

func TestIndividualSupportHasNoTier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		for _, key := range []string{"subscription_data[metadata][francois_support_tier]", "tax_id_collection[enabled]", "billing_address_collection"} {
			if r.Form.Get(key) != "" {
				t.Errorf("individual support sends %s", key)
			}
		}
		if r.Form.Get("line_items[0][price_data][product_data][name]") != "Monthly support" {
			t.Error("individual product name changed")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_support","url":"https://checkout.test/support","status":"open"}`))
	}))
	defer server.Close()
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(server.URL), HTTPClient: server.Client()})
	g := &stripeGateway{client: stripe.NewClient("sk_test_fixture", stripe.WithBackends(&stripe.Backends{API: backend}))}
	if _, err := g.CreateCheckout(context.Background(), CheckoutRequest{CustomerID: "cus_fixture", SupportAmountCents: 4200, IdempotencyKey: "attempt-individual"}); err != nil {
		t.Fatal(err)
	}
}

func TestEnterpriseProjection(t *testing.T) {
	s := &stripe.Subscription{Metadata: map[string]string{"francois_monthly_support": "true", "francois_support_tier": "enterprise"}, Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{Quantity: 1, Price: &stripe.Price{ID: "price_dynamic", Currency: stripe.CurrencyUSD, UnitAmount: 25600, Recurring: &stripe.PriceRecurring{Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1}}}}}}
	if p := Project(s); p.SupportAmountCents != 25600 || p.SupportTier != billing.TierEnterprise {
		t.Fatalf("projection %+v", p)
	}
	// The tier only rides on a valid support subscription.
	delete(s.Metadata, "francois_monthly_support")
	if p := Project(s); p.SupportTier != "" || p.SupportAmountCents != 0 {
		t.Fatalf("tier without support %+v", p)
	}
}

func TestSupportEnterpriseService(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	g := newFake()
	svc := NewService(g, s, Config{Deployment: "test"})
	u := user(t, s, "acme")
	if _, err := svc.SupportEnterprise(ctx, u, 30000, nil, "https://portal.test"); err == nil || g.creates != 0 {
		t.Fatal("300 USD accepted as enterprise")
	}
	if _, err := svc.SupportEnterprise(ctx, u, 51200, nil, "https://portal.test"); err != nil {
		t.Fatal(err)
	}
	r := g.requests[0]
	if r.SupportAmountCents != 51200 || r.SupportTier != billing.TierEnterprise || r.YearFunding {
		t.Fatalf("request %+v", r)
	}
	attempts, _ := s.CheckoutAttempts(ctx, u.ID, 1)
	if len(attempts) != 1 || attempts[0].Price != "support:enterprise:usd:51200" {
		t.Fatalf("attempts %+v", attempts)
	}
}
