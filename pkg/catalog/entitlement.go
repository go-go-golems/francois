package catalog

import (
	"context"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/hosted"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// Entitlement sources.
const (
	SourceDisabled      = "disabled"
	SourceStaff         = "staff"
	SourceGuest         = "guest"
	SourceSubscription  = "subscription"
	SourceTrial         = "trial"
	SourceComplimentary = "complimentary"
	SourceFree          = "free"
)

// Entitlement is what an account may do now and why: the plan that applies
// and where it comes from. It is the only mapping from billing state, grants
// and roles to permissions; every check asks it.
type Entitlement struct {
	Plan     settings.Plan    `json:"plan"`
	Source   string           `json:"source"`
	Decision billing.Decision `json:"decision"`
}

// resolveEntitlement is pure: account, billing decision and settings in,
// entitlement out.
//
//	disabled              → nothing
//	creator or admin      → Paying limits (staff permissions are separate)
//	guest (demo visitor)  → the demo allowance, no workspaces
//	standing subscription → the single subscriber tier
//	complimentary grant   → Paying limits
//	otherwise             → the free plan
func resolveEntitlement(u store.User, d billing.Decision, doc settings.Document) Entitlement {
	switch {
	case u.Disabled:
		return Entitlement{Plan: doc.Free(), Source: SourceDisabled, Decision: d}
	case u.Role == "admin" || u.Role == "creator":
		return Entitlement{Plan: doc.Paying(), Source: SourceStaff, Decision: d}
	case u.Role == "guest":
		return Entitlement{Plan: demoPlan(doc), Source: SourceGuest, Decision: d}
	}
	if d.Allowed {
		switch d.Reason {
		case billing.ReasonSubscriber, billing.ReasonTrial:
			source := SourceSubscription
			if d.Reason == billing.ReasonTrial {
				source = SourceTrial
			}
			plan, ok := doc.Plan("subscriber")
			if ok {
				return Entitlement{Plan: plan, Source: source, Decision: d}
			}
		case billing.ReasonComplimentary:
			p := doc.Paying()
			return Entitlement{Plan: p, Source: SourceComplimentary, Decision: d}
		}
	}
	return Entitlement{Plan: doc.Free(), Source: SourceFree, Decision: d}
}

// demoPlan is the allowance of a demo visitor.
func demoPlan(doc settings.Document) settings.Plan {
	return settings.Plan{Key: "demo", Name: "Demo", MembersPerInstance: 1, DailyTokens: doc.Demo.Tokens,
		Prices: []settings.PlanPrice{}}
}

// Paid reports access beyond the free plan (staff, subscription, grant).
func (e Entitlement) Paid() bool {
	switch e.Source {
	case SourceStaff, SourceSubscription, SourceTrial, SourceComplimentary:
		return true
	}
	return false
}

// CanRun reports whether the plan includes an artifact.
func (e Entitlement) CanRun(a store.Artifact, doc settings.Document) bool {
	switch e.Source {
	case SourceDisabled, SourceGuest:
		return false
	case SourceStaff:
		return true
	}
	return !a.EarlyAccess(time.Now()) || e.Paid()
}

// AllowsModel reports whether the plan may use an approved selector.
func (e Entitlement) AllowsModel(selector string) bool { return true }

func (s *Server) policyFor(doc settings.Document) billing.Policy {
	return billing.Policy{AcceptedPrices: doc.Prices(), TrialAllowed: s.Config.TrialAllowed}
}

// entitlementIn resolves a user's entitlement under a given settings document.
func (s *Server) entitlementIn(ctx context.Context, u store.User, doc settings.Document) (Entitlement, error) {
	d, err := s.Store.Access(ctx, s.policyFor(doc), u, time.Now())
	if err != nil {
		return Entitlement{}, err
	}
	return resolveEntitlement(u, d, doc), nil
}

// entitlement resolves a user's entitlement under the active settings.
func (s *Server) entitlement(ctx context.Context, u store.User) (Entitlement, settings.Document, error) {
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		return Entitlement{}, settings.Document{}, err
	}
	e, err := s.entitlementIn(ctx, u, rev.Document)
	return e, rev.Document, err
}

// budget is the hosted runtime's Budgeter: a demo instance draws on its own
// per-demo ledger and the shared daily demo budget; everything else on the
// caller's plan.
func (s *Server) budget(ctx context.Context, u store.User, inst store.Instance) (hosted.Budget, error) {
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		return hosted.Budget{}, err
	}
	doc := rev.Document
	if inst.Kind == "demo" {
		return hosted.Budget{Limit: doc.Demo.Tokens, Shared: true, SharedLimit: doc.Demo.DailyBudget,
			Period: "demo:" + inst.ID}, nil
	}
	e, err := s.entitlementIn(ctx, u, doc)
	if err != nil {
		return hosted.Budget{}, err
	}
	return hosted.Budget{Limit: e.Plan.DailyTokens}, nil
}
