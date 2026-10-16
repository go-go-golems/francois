package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/pkg/errors"
)

// Admin is the operator API (/api/admin; scopes admin:read for reads,
// admin:write for changes): customers, grants, tokens, subscriptions,
// instances, settings and plans, billing, usage and the audit log.
type Admin struct{ c *Client }

// Overview is the operator dashboard summary.
func (a Admin) Overview(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	return out, a.c.get(ctx, "/api/admin/overview", nil, &out)
}

// Users searches customers.
func (a Admin) Users(ctx context.Context, q UserQuery) ([]UserRow, error) {
	v := url.Values{}
	for k, s := range map[string]string{"q": q.Q, "role": q.Role, "plan": q.Plan} {
		if s != "" {
			v.Set(k, s)
		}
	}
	if q.Limit > 0 {
		v.Set("limit", itoa(q.Limit))
	}
	var out []UserRow
	return out, a.c.get(ctx, "/api/admin/users", v, &out)
}

// User returns one customer in full.
func (a Admin) User(ctx context.Context, id string) (Customer, error) {
	var out Customer
	return out, a.c.get(ctx, "/api/admin/users/"+esc(id), nil, &out)
}

// PatchUser changes a user's role or disabled flag.
func (a Admin) PatchUser(ctx context.Context, id string, in UserPatch) error {
	return a.c.send2(ctx, http.MethodPatch, "/api/admin/users/"+esc(id), in, nil)
}

// RevokeSessions ends a user's sessions and live views, and with tokens also
// their API tokens.
func (a Admin) RevokeSessions(ctx context.Context, id string, tokens bool) (Revoked, error) {
	var out Revoked
	return out, a.c.send2(ctx, http.MethodPost, "/api/admin/users/"+esc(id)+"/revoke-sessions", map[string]bool{"tokens": tokens}, &out)
}

// GrantAccess gives a customer complimentary access.
func (a Admin) GrantAccess(ctx context.Context, userID string, in GrantRequest) (AccessGrant, error) {
	var out AccessGrant
	return out, a.c.send2(ctx, http.MethodPost, "/api/admin/users/"+esc(userID)+"/access-grants", in, &out)
}

// RevokeGrant revokes one complimentary grant.
func (a Admin) RevokeGrant(ctx context.Context, grantID string) (AccessGrant, error) {
	var out AccessGrant
	return out, a.c.send2(ctx, http.MethodDelete, "/api/admin/access-grants/"+esc(grantID), nil, &out)
}

// RevokeAllGrants revokes every live grant of a customer.
func (a Admin) RevokeAllGrants(ctx context.Context, userID string) error {
	return a.c.send2(ctx, http.MethodDelete, "/api/admin/users/"+esc(userID)+"/access-grants", nil, nil)
}

// UserTokens lists a user's API tokens (metadata only).
func (a Admin) UserTokens(ctx context.Context, userID string) ([]Token, error) {
	var out []Token
	return out, a.c.get(ctx, "/api/admin/users/"+esc(userID)+"/tokens", nil, &out)
}

// RevokeToken revokes any user's token.
func (a Admin) RevokeToken(ctx context.Context, id string) error {
	return a.c.send2(ctx, http.MethodDelete, "/api/admin/tokens/"+esc(id), nil, nil)
}

// Subscriptions lists subscriptions across customers; status "" lists all.
func (a Admin) Subscriptions(ctx context.Context, status string, limit int) ([]SubscriptionRow, error) {
	v := url.Values{}
	if status != "" {
		v.Set("status", status)
	}
	if limit > 0 {
		v.Set("limit", itoa(limit))
	}
	var out []SubscriptionRow
	return out, a.c.get(ctx, "/api/admin/subscriptions", v, &out)
}

// SetCancelAtPeriodEnd schedules (true) or withdraws (false) a subscription's
// end at its period end. It moves no money; refunds stay in Stripe.
func (a Admin) SetCancelAtPeriodEnd(ctx context.Context, subscriptionID string, cancel bool) error {
	action := "resume"
	if cancel {
		action = "cancel"
	}
	return a.c.send2(ctx, http.MethodPost, "/api/admin/subscriptions/"+esc(subscriptionID)+"/"+action, nil, nil)
}

func (a Admin) Instances(ctx context.Context, q string) ([]Instance, error) {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	var out []Instance
	return out, a.c.get(ctx, "/api/admin/instances", v, &out)
}

func (a Admin) Instance(ctx context.Context, id string) (Instance, error) {
	var out Instance
	return out, a.c.get(ctx, "/api/admin/instances/"+esc(id), nil, &out)
}

// SetInstanceSuspended suspends or resumes an instance.
func (a Admin) SetInstanceSuspended(ctx context.Context, id string, suspended bool) error {
	action := "resume"
	if suspended {
		action = "suspend"
	}
	return a.c.send2(ctx, http.MethodPost, "/api/admin/instances/"+esc(id)+"/"+action, nil, nil)
}

func (a Admin) DeleteInstance(ctx context.Context, id string) error {
	return a.c.send2(ctx, http.MethodDelete, "/api/admin/instances/"+esc(id), nil, nil)
}

func (a Admin) Settings(ctx context.Context) (Settings, error) {
	var out Settings
	return out, a.c.get(ctx, "/api/admin/settings", nil, &out)
}

// SaveSettings activates doc if base is still the newest revision; otherwise
// the error has code settings_conflict.
func (a Admin) SaveSettings(ctx context.Context, doc settings.Document, base int64) (SettingsRevision, error) {
	var out SettingsRevision
	return out, a.c.do(ctx, request{method: http.MethodPatch, path: "/api/admin/settings", body: doc,
		headers: map[string]string{"If-Match": strconv.Quote(strconv.FormatInt(base, 10))}}, &out)
}

// EditSettings applies edit to the current document and saves it, re-reading
// and re-applying when another revision was activated in between.
func (a Admin) EditSettings(ctx context.Context, edit func(*settings.Document) error) (SettingsRevision, error) {
	for attempt := 0; attempt < 3; attempt++ {
		cur, err := a.Settings(ctx)
		if err != nil {
			return SettingsRevision{}, err
		}
		doc := cur.Active.Document
		if err = edit(&doc); err != nil {
			return SettingsRevision{}, err
		}
		if err = doc.Validate(); err != nil {
			return SettingsRevision{}, err
		}
		rev, err := a.SaveSettings(ctx, doc, cur.Active.Revision)
		if IsCode(err, "settings_conflict") {
			continue
		}
		return rev, err
	}
	return SettingsRevision{}, errors.New("settings kept changing; try again")
}

// EditPlan edits one plan of the settings document.
func (a Admin) EditPlan(ctx context.Context, key string, edit func(*settings.Plan) error) (SettingsRevision, error) {
	return a.EditSettings(ctx, func(d *settings.Document) error {
		i := slices.IndexFunc(d.Plans, func(p settings.Plan) bool { return p.Key == key })
		if i < 0 {
			return &Error{Status: 404, Code: "not_found", Message: "no plan " + key}
		}
		return edit(&d.Plans[i])
	})
}

// Billing is the Stripe event inbox and checkout overview.
func (a Admin) Billing(ctx context.Context, state string) (json.RawMessage, error) {
	v := url.Values{}
	if state != "" {
		v.Set("state", state)
	}
	var out json.RawMessage
	return out, a.c.get(ctx, "/api/admin/billing", v, &out)
}

// Reconcile re-syncs one customer from Stripe.
func (a Admin) Reconcile(ctx context.Context, userID string) error {
	return a.c.send2(ctx, http.MethodPost, "/api/admin/billing/reconcile", map[string]string{"userId": userID}, nil)
}

func (a Admin) RetryEvent(ctx context.Context, id string) error {
	return a.c.send2(ctx, http.MethodPost, "/api/admin/billing/events/"+esc(id)+"/retry", nil, nil)
}

// Usage returns the usage summary and recent model calls, optionally for one user.
func (a Admin) Usage(ctx context.Context, userID string, limit int) (Usage, error) {
	v := url.Values{}
	if userID != "" {
		v.Set("user", userID)
	}
	if limit > 0 {
		v.Set("limit", itoa(limit))
	}
	var out Usage
	return out, a.c.get(ctx, "/api/admin/usage", v, &out)
}

// AdjustCredits adds (or with a negative value removes) credits for the
// current period.
func (a Admin) AdjustCredits(ctx context.Context, userID string, credits int, reason string) error {
	return a.c.send2(ctx, http.MethodPost, "/api/admin/usage/adjustments", map[string]any{"userId": userID, "credits": credits, "reason": reason}, nil)
}

func (a Admin) Audit(ctx context.Context, q AuditQuery) ([]AuditEntry, error) {
	v := url.Values{}
	for k, s := range map[string]string{"subject": q.Subject, "actor": q.Actor, "action": q.Action} {
		if s != "" {
			v.Set(k, s)
		}
	}
	if q.Before > 0 {
		v.Set("before", strconv.FormatInt(q.Before, 10))
	}
	if q.Limit > 0 {
		v.Set("limit", itoa(q.Limit))
	}
	var out []AuditEntry
	return out, a.c.get(ctx, "/api/admin/audit", v, &out)
}
