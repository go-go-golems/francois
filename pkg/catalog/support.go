package catalog

import (
	"context"
	"net/http"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

type supportView struct {
	Kind            string     `json:"kind"`
	Active          bool       `json:"active"`
	AmountCents     int64      `json:"amountCents"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	Until           *time.Time `json:"until,omitempty"`
	CancelScheduled bool       `json:"cancelScheduled"`
	Experiments     int        `json:"experiments"`
}

// Support presentation follows confirmed payment state across monthly and year funding.
func supportStatus(subs []billing.Subscription, doc settings.Document, now time.Time) supportView {
	out := supportView{Kind: "month"}
	for _, sub := range subs {
		_, plan := doc.PlanForPrice(sub.PriceID)
		if (sub.SupportAmountCents <= 0 && !plan) || sub.Status != "active" || sub.Quantity != 1 || sub.ItemCount != 1 || sub.AccessUntil == nil || !now.Before(*sub.AccessUntil) {
			continue
		}
		out.Active = true
		if out.StartedAt == nil || sub.StartedAt != nil && sub.StartedAt.Before(*out.StartedAt) {
			out.StartedAt = sub.StartedAt
		}
		out.AmountCents += sub.SupportAmountCents
		if out.Until == nil || sub.AccessUntil.Before(*out.Until) {
			out.Until = sub.AccessUntil
			out.CancelScheduled = sub.CancelAtPeriodEnd
		}
	}
	return out
}

func (s *Server) userSupport(ctx context.Context, user store.User, doc settings.Document) (supportView, error) {
	subs, err := s.Store.Subscriptions(ctx, user.ID)
	if err != nil {
		return supportView{}, err
	}
	return s.supportSummary(ctx, user, subs, doc)
}

func (s *Server) supportSummary(ctx context.Context, user store.User, subs []billing.Subscription, doc settings.Document) (supportView, error) {
	out := supportStatus(subs, doc, time.Now())
	year, err := s.Store.YearSupport(ctx, user.ID, time.Now())
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return supportView{}, err
	}
	if year.Until != nil {
		out.Active = true
		out.Kind = "year"
		out.AmountCents = billing.YearFundingCents
		out.Until = year.Until
		out.CancelScheduled = true
		if out.StartedAt == nil || year.PaidAt != nil && year.PaidAt.Before(*out.StartedAt) {
			out.StartedAt = year.PaidAt
		}
	}
	return out, nil
}

func (s *Server) supportCheckout(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	var in struct {
		Kind        string `json:"kind"`
		AmountCents int64  `json:"amountCents"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || (in.Kind != "year" && (in.Kind != "" && in.Kind != "month" || in.AmountCents < 100 || in.AmountCents > 999900 || in.AmountCents%100 != 0)) {
		httpx.Fail(w, 400, "invalid_amount", "Choose 1–9999 whole US dollars per month")
		return
	}
	p := principal(r.Context())
	if p.User.Role == "guest" || !p.User.EmailVerified {
		httpx.Fail(w, 403, "email_unverified", "Sign in with a verified email address to support the lab")
		return
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Billing is temporarily unavailable")
		return
	}
	var target string
	if in.Kind == "year" {
		target, err = s.Payments.FundYear(r.Context(), p.User, s.site(r).PortalOrigin())
	} else {
		target, err = s.Payments.Support(r.Context(), p.User, in.AmountCents, rev.Document.Prices(), s.site(r).PortalOrigin())
	}
	if err != nil {
		s.billingFailure(w, err)
		return
	}
	httpx.OK(w, map[string]string{"url": target})
}
