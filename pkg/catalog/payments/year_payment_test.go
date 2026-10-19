package payments

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

func TestYearPaymentLifecycleWithMissedWebhooks(t *testing.T) {
	for _, outcome := range []string{"paid", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			ctx := context.Background()
			s := testStore(t)
			g := newFake()
			svc := NewService(g, s, Config{Deployment: "test"})
			u := user(t, s, "async-"+outcome)
			if _, err := svc.FundYear(ctx, u, "https://portal.test"); err != nil {
				t.Fatal(err)
			}
			req := g.requests[0]
			cs := g.sessions[req.IdempotencyKey]
			cs.Status = "complete"
			g.sessions[req.IdempotencyKey] = cs
			// The Session's local expiry passes before its completed webhook arrives.
			if _, err := s.Pool.Exec(ctx, `UPDATE checkout_attempts SET expires_at=now()-interval '1 day' WHERE user_id=$1`, u.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.FundYear(ctx, u, "https://portal.test"); !errors.Is(err, store.ErrPaymentPending) {
				t.Fatal("completed Stripe session must block another payment", err)
			}
			if g.creates != 1 {
				t.Fatal("clock expiry admitted a second year payment")
			}
			customer, err := s.CustomerFor(ctx, u.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.InsertEvent(ctx, store.StripeEvent{EventID: "evt_completed", Type: "checkout.session.completed", CustomerID: customer, SessionID: cs.ID}); err != nil {
				t.Fatal(err)
			}
			if did, err := svc.processOne(ctx); err != nil || !did {
				t.Fatal("completed webhook", did, err)
			}
			attempts, err := s.CheckoutAttempts(ctx, u.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if attempts[0].State != "completed" || attempts[0].PaymentState == nil || *attempts[0].PaymentState != "pending" || attempts[0].SupportUntil != nil {
				t.Fatalf("unsettled projection %+v", attempts[0])
			}
			if _, err = svc.FundYear(ctx, u, "https://portal.test"); !errors.Is(err, store.ErrPaymentPending) {
				t.Fatal("second year payment admitted", err)
			}
			if _, err = svc.Support(ctx, u, 4200, nil, "https://portal.test"); !errors.Is(err, store.ErrPaymentPending) {
				t.Fatal("different payment bypassed in-flight year", err)
			}
			// No success/failure webhook and no browser return. Even an old pending
			// payment remains eligible; reconciliation work is batch-limited, not aged out.
			if _, err = s.Pool.Exec(ctx, `UPDATE checkout_attempts SET updated_at=now()-interval '40 days' WHERE user_id=$1`, u.ID); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			if outcome == "paid" {
				cs.PaymentPaid = true
				cs.PaidAt = &now
			} else {
				cs.PaymentFailed = true
			}
			g.sessions[req.IdempotencyKey] = cs
			svc.reconcile(ctx)
			if did, err := svc.processOne(ctx); err != nil || !did {
				t.Fatal("lost-webhook reconciliation", did, err)
			}
			attempts, err = s.CheckoutAttempts(ctx, u.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if *attempts[0].PaymentState != outcome {
				t.Fatalf("final state %+v", attempts[0])
			}
			if outcome == "paid" {
				standing, err := s.YearSupport(ctx, u.ID, time.Now())
				if err != nil || !standing.Until.Equal(now.AddDate(1, 0, 0)) {
					t.Fatal("paid customer remained without access", err)
				}
				if _, err = svc.FundYear(ctx, u, "https://portal.test"); !errors.Is(err, store.ErrYearFunded) {
					t.Fatal("confirmed overlap", err)
				}
			} else {
				if _, err = svc.FundYear(ctx, u, "https://portal.test"); err != nil {
					t.Fatal("confirmed failure did not release admission", err)
				}
				if g.creates != 2 {
					t.Fatal("new checkout missing after failure")
				}
			}
		})
	}
}

func TestYearPaymentProjection(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		session CheckoutSession
		want    string
	}{
		{"open", CheckoutSession{Status: "open", YearFunding: true}, "pending"},
		{"completed unpaid", CheckoutSession{Status: "complete", YearFunding: true}, "pending"},
		{"paid missing timestamp", CheckoutSession{Status: "complete", YearFunding: true, PaymentPaid: true}, "pending"},
		{"paid", CheckoutSession{Status: "complete", YearFunding: true, PaymentPaid: true, PaidAt: &now}, "paid"},
		{"failed", CheckoutSession{Status: "complete", YearFunding: true, PaymentFailed: true}, "failed"},
		{"expired", CheckoutSession{Status: "expired", YearFunding: true}, "failed"},
		{"refunded", CheckoutSession{Status: "complete", YearFunding: true, PaymentPaid: true, PaidAt: &now, Refunded: true}, "refunded"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := projectYearPayment("attempt", c.session)
			if err != nil || p.PaymentState != c.want {
				t.Fatalf("%+v %v", p, err)
			}
			if (p.Until != nil) != (c.want == "paid") {
				t.Fatal("access and payment disagree")
			}
		})
	}
	if _, err := projectYearPayment("attempt", CheckoutSession{Status: "complete"}); err == nil {
		t.Fatal("foreign payment accepted")
	}
}

func TestWebhookDeliveryContract(t *testing.T) {
	for _, name := range []string{"checkout.session.async_payment_succeeded", "checkout.session.async_payment_failed", "charge.refunded"} {
		if !supported[name] || !strings.Contains(WebhookEventsCSV(), name) {
			t.Fatal("missing delivery event", name)
		}
	}
}
