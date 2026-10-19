package catalog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
)

func TestSupporterPresentation(t *testing.T) {
	e := setup(t)
	u := e.user("supporter", "subscriber", false)
	_, b, _ := e.do(&u, "portal.test", "GET", "/pricing", nil, nil)
	if !strings.Contains(string(b), `value="42"`) || !strings.Contains(string(b), "999,999.99 USD") {
		t.Fatal("missing Final contribution design")
	}
	until := time.Now().Add(24 * time.Hour)
	start := time.Now().Add(-24 * time.Hour)
	_, err := e.store.Pool.Exec(context.Background(), `INSERT INTO billing_subscriptions(stripe_subscription_id,user_id,stripe_customer_id,price_id,status,quantity,item_count,access_until,support_amount_cents,started_at) VALUES('sub_support',$1,'cus_support','price_dynamic','active',1,1,$2,4200,$3)`, u.user.ID, until, start)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/pricing", "/"} {
		_, b, _ = e.do(&u, "portal.test", "GET", path, nil, nil)
		if strings.Contains(string(b), "Keep the lab running.") || strings.Contains(string(b), `data-support-monthly`) || strings.Contains(string(b), `class="site-support"`) {
			t.Fatalf("supporter solicited at %s", path)
		}
	}
	_, s := e.portal(&u, "GET", "/api/session", nil)
	if s["supporter"] != true {
		t.Fatal("session missing supporter state")
	}
	_, a := e.portal(&u, "GET", "/api/account", nil)
	sup := a["support"].(map[string]any)
	if sup["active"] != true || sup["amountCents"] != float64(4200) {
		t.Fatalf("support summary %v", sup)
	}
	if a["entitlement"].(map[string]any)["source"] != SourceSubscription {
		t.Fatal("confirmed contribution did not enter the Subscriber tier")
	}
}

func TestSupportStatus(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	sub := billing.Subscription{SupportAmountCents: 4200, Status: "active", Quantity: 1, ItemCount: 1, AccessUntil: &future, CancelAtPeriodEnd: true}
	if !supportStatus([]billing.Subscription{sub}, planDoc(), now).Active {
		t.Fatal("paid-through cancellation lost support")
	}
	for _, mutate := range []func(*billing.Subscription){func(s *billing.Subscription) { s.Status = "past_due" }, func(s *billing.Subscription) { s.AccessUntil = &past }, func(s *billing.Subscription) { s.Quantity = 2 }, func(s *billing.Subscription) { s.SupportAmountCents = 0 }} {
		candidate := sub
		mutate(&candidate)
		if supportStatus([]billing.Subscription{candidate}, planDoc(), now).Active {
			t.Fatal("unconfirmed/expired support accepted")
		}
	}
}
