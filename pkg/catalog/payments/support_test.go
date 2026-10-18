package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"
)

func TestMonthlySupportStripePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		for key, want := range map[string]string{"mode": "subscription", "line_items[0][price_data][currency]": "usd", "line_items[0][price_data][unit_amount]": "4200", "line_items[0][price_data][recurring][interval]": "month", "line_items[0][quantity]": "1", "subscription_data[metadata][francois_monthly_support]": "true"} {
			if got := r.Form.Get(key); got != want {
				t.Errorf("%s=%q, want %q", key, got, want)
			}
		}
		if r.Form.Get("line_items[0][price]") != "" {
			t.Error("custom amount still sends fixed price")
		}
		if r.Header.Get("Idempotency-Key") != "attempt-one" {
			t.Error("missing stable idempotency key")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_support","url":"https://checkout.test/support","status":"open"}`))
	}))
	defer server.Close()
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{URL: stripe.String(server.URL), HTTPClient: server.Client()})
	g := &stripeGateway{client: stripe.NewClient("sk_test_fixture", stripe.WithBackends(&stripe.Backends{API: backend}))}
	if _, err := g.CreateCheckout(context.Background(), CheckoutRequest{CustomerID: "cus_fixture", SupportAmountCents: 4200, IdempotencyKey: "attempt-one"}); err != nil {
		t.Fatal(err)
	}
}

func TestMonthlySupportProjection(t *testing.T) {
	s := &stripe.Subscription{StartDate: 1700000000, Metadata: map[string]string{"francois_monthly_support": "true"}, Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{Quantity: 1, Price: &stripe.Price{ID: "price_dynamic", Currency: stripe.CurrencyUSD, UnitAmount: 4200, Recurring: &stripe.PriceRecurring{Interval: stripe.PriceRecurringIntervalMonth, IntervalCount: 1}}}}}}
	p := Project(s)
	if p.SupportAmountCents != 4200 || p.PriceID != "price_dynamic" || p.StartedAt == nil {
		t.Fatalf("wrong projection %+v", p)
	}
	s.Metadata = nil
	if Project(s).SupportAmountCents != 0 {
		t.Fatal("unmarked price accepted as contribution")
	}
	s.Metadata = map[string]string{"francois_monthly_support": "true"}
	s.Items.Data[0].Price.Currency = stripe.CurrencyEUR
	if Project(s).SupportAmountCents != 0 {
		t.Fatal("foreign currency accepted as USD")
	}
}
