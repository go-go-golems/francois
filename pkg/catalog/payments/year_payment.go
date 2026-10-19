package payments

import (
	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/pkg/errors"
)

// projectYearPayment interprets a retrieved Stripe snapshot, never the event
// name. Completed checkout can still have an unsettled payment. All refresh
// paths persist these facts and access dates in the same transaction.
func projectYearPayment(attempt string, cs CheckoutSession) (billing.YearSupport, error) {
	out := billing.YearSupport{AttemptID: attempt, PaymentState: "pending"}
	if !cs.YearFunding {
		return out, errors.New("checkout is not the expected year payment")
	}
	switch cs.Status {
	case "open":
		out.CheckoutState = "open"
	case "expired":
		out.CheckoutState = "expired"
		out.PaymentState = "failed"
	case "complete":
		out.CheckoutState = "completed"
		switch {
		case cs.Refunded:
			out.PaymentState = "refunded"
		case cs.PaymentPaid && cs.PaidAt != nil:
			out.PaymentState = "paid"
			out.PaidAt = cs.PaidAt
			until := cs.PaidAt.AddDate(1, 0, 0)
			out.Until = &until
		case cs.PaymentFailed:
			out.PaymentState = "failed"
		}
	default:
		return out, errors.New("unknown Checkout session state")
	}
	return out, nil
}
