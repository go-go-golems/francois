// Package billing derives hosted access from projected Stripe state and
// integrates Stripe Customers, Checkout, Portal and webhooks.
package billing

import "time"

// Subscription is the projection of one Stripe subscription.
type Subscription struct {
	ID                string     `json:"id"`
	CustomerID        string     `json:"customerId"`
	PriceID           string     `json:"priceId"`
	Status            string     `json:"status"`
	Quantity          int        `json:"quantity"`
	ItemCount         int        `json:"itemCount"`
	AccessUntil       *time.Time `json:"accessUntil,omitempty"`
	TrialUntil        *time.Time `json:"trialUntil,omitempty"`
	CancelAtPeriodEnd bool       `json:"cancelAtPeriodEnd"`
	CanceledAt        *time.Time `json:"canceledAt,omitempty"`
	RefreshedAt       time.Time  `json:"refreshedAt"`
}

// Comp is an administrator-granted complimentary access window.
type Comp struct {
	Until   time.Time
	Revoked bool
}

type Account struct {
	Disabled bool
}

type Policy struct {
	// AcceptedPrices are server-configured recurring Price IDs.
	AcceptedPrices map[string]bool
	// TrialAllowed enables access while trialing; no trial is offered by default.
	TrialAllowed bool
}

type Decision struct {
	Allowed bool       `json:"allowed"`
	Reason  string     `json:"reason"`
	Until   *time.Time `json:"until,omitempty"`
	Status  string     `json:"status,omitempty"`
	// CancelScheduled is true when access ends at Until without renewal.
	CancelScheduled bool `json:"cancelScheduled"`
}

// Stable reason codes.
const (
	ReasonSubscriber    = "subscriber"
	ReasonTrial         = "subscriber_trial"
	ReasonComplimentary = "complimentary"
	ReasonDisabled      = "account_disabled"
	ReasonConflict      = "billing_conflict"
	ReasonPastDue       = "payment_past_due"
	ReasonIncomplete    = "payment_incomplete"
	ReasonRequired      = "subscription_required"
	ReasonExpired       = "subscription_expired"
)

var standing = map[string]bool{"active": true, "trialing": true, "past_due": true, "unpaid": true, "paused": true}

// Access decides whether an account may use hosted instances, model calls,
// and paid downloads now. It is pure and table-tested.
func (p Policy) Access(a Account, subs []Subscription, comps []Comp, now time.Time) Decision {
	if a.Disabled {
		return Decision{Reason: ReasonDisabled}
	}
	eligible := []Subscription{}
	incomplete := false
	for _, s := range subs {
		if !p.AcceptedPrices[s.PriceID] || s.Quantity != 1 || s.ItemCount != 1 {
			continue
		}
		if s.Status == "incomplete" {
			incomplete = true
		}
		if standing[s.Status] {
			eligible = append(eligible, s)
		}
	}
	var best *Decision
	for _, c := range comps {
		if !c.Revoked && now.Before(c.Until) {
			until := c.Until
			if best == nil || until.After(*best.Until) {
				best = &Decision{Allowed: true, Reason: ReasonComplimentary, Until: &until}
			}
		}
	}
	if len(eligible) > 1 {
		if best != nil {
			return *best
		}
		return Decision{Reason: ReasonConflict}
	}
	if len(eligible) == 1 {
		s := eligible[0]
		d := Decision{Status: s.Status, CancelScheduled: s.CancelAtPeriodEnd}
		switch {
		case s.Status == "active" && s.AccessUntil != nil && now.Before(*s.AccessUntil):
			d.Allowed, d.Reason, d.Until = true, ReasonSubscriber, s.AccessUntil
		case s.Status == "trialing" && p.TrialAllowed && s.TrialUntil != nil && now.Before(*s.TrialUntil):
			d.Allowed, d.Reason, d.Until = true, ReasonTrial, s.TrialUntil
		case s.Status == "active":
			d.Reason = ReasonExpired
		case s.Status == "past_due":
			d.Reason = ReasonPastDue
		default:
			d.Reason = ReasonRequired
		}
		if d.Allowed || best == nil {
			return d
		}
		best.Status = s.Status
		return *best
	}
	if best != nil {
		return *best
	}
	if incomplete {
		return Decision{Reason: ReasonIncomplete, Status: "incomplete"}
	}
	return Decision{Reason: ReasonRequired}
}

// OpenStatus reports subscription states that must go to billing management
// rather than a second Checkout.
func OpenStatus(status string) bool {
	return standing[status] || status == "incomplete"
}
