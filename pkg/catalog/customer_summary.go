package catalog

import (
	"context"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"time"
)

func customerClass(e Entitlement) string {
	if e.Paid() {
		return "Paying"
	}
	return "Free"
}

// customerSummary explains account eligibility. Lane availability and
// individual shared-workspace owner restrictions are checked at request time.
func (s *Server) customerSummary(ctx context.Context, u store.User, e Entitlement, doc settings.Document, a store.Allowance) (map[string]any, error) {
	support, err := s.userSupport(ctx, u, doc)
	if err != nil {
		return nil, err
	}
	var owned int
	if err = s.Store.Pool.QueryRow(ctx, `SELECT count(*)FROM instances WHERE owner_user_id=$1 AND kind='workspace'`, u.ID).Scan(&owned); err != nil {
		return nil, err
	}
	reason := "Available model lanes are checked when a request starts"
	eligible := true
	switch {
	case u.Disabled:
		eligible = false
		reason = "Account disabled"
	case !doc.Operations.SamplingEnabled:
		eligible = false
		reason = doc.Operations.Message("AI is paused for everyone")
	case len(doc.Models.Allowed) == 0:
		eligible = false
		reason = "No model is enabled"
	case a.Remaining == 0:
		eligible = false
		reason = "Daily generated-token ceiling reached"
	}
	return map[string]any{"class": customerClass(e), "enabled": !u.Disabled, "source": e.Source, "accessUntil": e.Decision.Until, "support": support, "asOf": time.Now().UTC(),
		"ai":        map[string]any{"eligible": eligible, "reason": reason, "maxOutputTokens": doc.Allowance.MaxOutputTokens},
		"allowance": a, "workspaces": map[string]any{"owned": owned, "limit": e.Plan.Instances, "membersPerInstance": e.Plan.MembersPerInstance}}, nil
}

// customerStanding classifies payment/access independently of the enabled
// switch. It is for admin presentation only; runtime authorization still
// resolves the original disabled user and denies access.
func (s *Server) customerStanding(ctx context.Context, u store.User, doc settings.Document) (Entitlement, error) {
	u.Disabled = false
	return s.entitlementIn(ctx, u, doc)
}
