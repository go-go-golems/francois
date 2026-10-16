package catalog

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
)

func (s *Server) adminRoutes(m *http.ServeMux) {
	a := func(h http.HandlerFunc) http.HandlerFunc { return s.guard(admin, scopeAdmin, h) }
	m.HandleFunc("GET /api/admin/overview", a(s.adminOverview))
	m.HandleFunc("GET /api/admin/users", a(s.adminUsers))
	m.HandleFunc("GET /api/admin/users/{id}", a(s.adminUser))
	m.HandleFunc("PATCH /api/admin/users/{id}", a(s.adminPatchUser))
	m.HandleFunc("POST /api/admin/users/{id}/revoke-sessions", a(s.adminRevoke))
	m.HandleFunc("GET /api/admin/users/{id}/tokens", a(s.adminUserTokens))
	m.HandleFunc("DELETE /api/admin/tokens/{id}", a(s.adminRevokeToken))
	m.HandleFunc("POST /api/admin/users/{id}/access-grants", a(s.adminGrant))
	m.HandleFunc("DELETE /api/admin/users/{id}/access-grants", a(s.adminRevokeGrants))
	m.HandleFunc("DELETE /api/admin/access-grants/{id}", a(s.adminRevokeGrant))
	m.HandleFunc("GET /api/admin/subscriptions", a(s.adminSubscriptions))
	m.HandleFunc("POST /api/admin/subscriptions/{id}/cancel", a(s.adminSubscriptionCancel(true)))
	m.HandleFunc("POST /api/admin/subscriptions/{id}/resume", a(s.adminSubscriptionCancel(false)))
	m.HandleFunc("DELETE /api/admin/instances/{id}", a(s.adminDeleteInstance))
	m.HandleFunc("GET /api/admin/instances", a(s.adminInstances))
	m.HandleFunc("GET /api/admin/instances/{id}", a(s.adminInstance))
	m.HandleFunc("POST /api/admin/instances/{id}/suspend", a(s.adminInstanceState("suspended")))
	m.HandleFunc("POST /api/admin/instances/{id}/resume", a(s.adminInstanceState("active")))
	m.HandleFunc("GET /api/admin/settings", a(s.adminSettings))
	m.HandleFunc("PATCH /api/admin/settings", a(s.adminSaveSettings))
	m.HandleFunc("GET /api/admin/billing", a(s.adminBilling))
	m.HandleFunc("POST /api/admin/billing/reconcile", a(s.adminReconcile))
	m.HandleFunc("POST /api/admin/billing/events/{id}/retry", a(s.adminRetryEvent))
	m.HandleFunc("GET /api/admin/usage", a(s.adminUsage))
	m.HandleFunc("GET /api/admin/analytics", a(s.adminAnalytics))
	m.HandleFunc("POST /api/admin/usage/adjustments", a(s.adminAdjust))
	m.HandleFunc("GET /api/admin/audit", a(s.adminAudit))
}

func actor(r *http.Request) string { return principal(r.Context()).User.ID }

func unavailable(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("admin request failed")
	httpx.Fail(w, 503, "unavailable", "Request failed")
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var users, admins, disabled, instances, drafts, published int
	err := s.Store.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users WHERE role<>'guest'),(SELECT count(*) FROM users WHERE role='admin'),(SELECT count(*) FROM users WHERE disabled),
 (SELECT count(*) FROM instances WHERE kind='workspace'),(SELECT count(*) FROM releases WHERE state='draft'),(SELECT count(*) FROM releases WHERE state='published')`).Scan(&users, &admins, &disabled, &instances, &drafts, &published)
	if err != nil {
		unavailable(w, err)
		return
	}
	usage, err := s.Store.UsageSummary(ctx)
	if err != nil {
		unavailable(w, err)
		return
	}
	health, err := s.Store.BillingHealth(ctx)
	if err != nil {
		unavailable(w, err)
		return
	}
	views := 0
	for _, n := range s.Runtime.ActiveViews() {
		views += n
	}
	httpx.OK(w, map[string]any{"users": users, "admins": admins, "disabled": disabled, "instances": instances, "drafts": drafts, "published": published,
		"activeViews": views, "usage": usage, "billing": health, "billingEnabled": s.Payments != nil, "modelsAvailable": len(s.Backend.Models())})
}

// upload parses a bounded multipart form shared by check and draft creation.
func (s *Server) upload(w http.ResponseWriter, r *http.Request) (releases.Upload, error) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		return releases.Upload{}, err
	}
	limit := rev.Document.Limits.MaxHTMLBytes
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	if err = r.ParseMultipartForm(limit + (1 << 20)); err != nil {
		return releases.Upload{}, errors.Wrap(err, "upload exceeds the size limit or is malformed")
	}
	u := releases.Upload{ArtifactID: strings.TrimSpace(r.FormValue("artifactId")), Title: strings.TrimSpace(r.FormValue("title")), Description: r.FormValue("description"),
		Version: strings.TrimSpace(r.FormValue("version")), Changelog: r.FormValue("changelog"), FreeSample: r.FormValue("freeSample") == "true", MaxHTML: limit}
	for _, t := range strings.Split(r.FormValue("tags"), ",") {
		if t = strings.TrimSpace(t); t != "" && len(u.Tags) < 12 {
			u.Tags = append(u.Tags, strings.ToLower(t))
		}
	}
	f, _, err := r.FormFile("html")
	if err != nil {
		return u, errors.New("choose an HTML file")
	}
	defer func() { _ = f.Close() }()
	if u.HTML, err = releases.ReadLimited(f, limit); err != nil {
		return u, err
	}
	if mf, _, err := r.FormFile("manifest"); err == nil {
		defer func() { _ = mf.Close() }()
		if u.Manifest, err = releases.ReadLimited(mf, 64<<10); err != nil {
			return u, err
		}
	} else if m := r.FormValue("manifestJson"); m != "" {
		u.Manifest = []byte(m)
	}
	return u, nil
}

// adminUsers searches customers: q matches email, name, ID or subject; role
// and plan (key) narrow the result; limit is at most 500.
func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	// Role is filtered in SQL. The plan is only known after resolving each
	// user's entitlement, so read candidates in pages until limit match.
	const page = 200
	out := []map[string]any{}
	for offset := 0; len(out) < limit; offset += page {
		users, err := s.Store.UserPage(r.Context(), store.UserFilter{Search: q.Get("q"), Role: q.Get("role"), Limit: page, Offset: offset})
		if err != nil {
			unavailable(w, err)
			return
		}
		for _, u := range users {
			e, err := s.entitlementIn(r.Context(), u, rev.Document)
			if err != nil {
				unavailable(w, err)
				return
			}
			if q.Get("plan") != "" && e.Plan.Key != q.Get("plan") {
				continue
			}
			out = append(out, map[string]any{"user": u, "access": e.Decision, "plan": e.Plan.Name, "planKey": e.Plan.Key, "source": e.Source})
			if len(out) == limit {
				break
			}
		}
		if len(users) < page {
			break
		}
	}
	httpx.OK(w, out)
}

func (s *Server) adminUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, err := s.Store.User(ctx, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "User not found")
		return
	}
	e, doc, _ := s.entitlement(ctx, u)
	subs, _ := s.Store.Subscriptions(ctx, u.ID)
	grants, _ := s.Store.AccessGrants(ctx, u.ID)
	insts, _ := s.Store.MemberInstances(ctx, u.ID)
	sessions, _ := s.Store.ActiveSessions(ctx, u.ID)
	customer, _ := s.Store.CustomerFor(ctx, u.ID)
	allowance, _ := s.Store.AllowanceFor(ctx, u.ID, e.Plan.MonthlyCredits, e.Plan.DailyCredits)
	calls, _ := s.Store.ModelCalls(ctx, u.ID, 20)
	audit, _ := s.Store.AuditLog(ctx, u.ID, 20)
	list := []map[string]any{}
	for _, i := range insts {
		list = append(list, s.instanceJSON(r, i))
	}
	plans := []map[string]any{}
	for _, p := range doc.Plans {
		plans = append(plans, map[string]any{"key": p.Key, "name": p.Name})
	}
	httpx.OK(w, map[string]any{"user": u, "access": e.Decision, "entitlement": e, "plans": plans, "subscriptions": subs, "complimentary": grants, "instances": list, "sessions": sessions,
		"stripeCustomerId": customer, "allowance": allowance, "calls": calls, "audit": audit})
}

func (s *Server) adminPatchUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if id == actor(r) && (in.Disabled != nil && *in.Disabled || in.Role != nil && *in.Role != "admin") {
		httpx.Fail(w, 409, "self_lockout", "You cannot disable or demote your own account")
		return
	}
	if in.Role != nil {
		if err := s.Store.SetUserRole(r.Context(), id, *in.Role); err != nil {
			httpx.Fail(w, 400, "invalid_request", err.Error())
			return
		}
		_ = s.Store.Audit(r.Context(), actor(r), "user.role", id, map[string]any{"role": *in.Role})
	}
	if in.Disabled != nil {
		if err := s.Store.SetUserDisabled(r.Context(), id, *in.Disabled); err != nil {
			httpx.Fail(w, 404, "not_found", "User not found")
			return
		}
		if *in.Disabled {
			s.Runtime.RevokeUser(id)
		}
		_ = s.Store.Audit(r.Context(), actor(r), "user.disabled", id, map[string]any{"disabled": *in.Disabled})
	}
	httpx.OK(w, map[string]bool{"ok": true})
}

// adminRevoke ends a user's sessions, runtime grants and live views, and with
// {"tokens": true} also revokes their personal API tokens. The body is optional.
func (s *Server) adminRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Tokens bool `json:"tokens"`
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024))
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		err = json.Unmarshal(raw, &in)
	}
	if err != nil {
		httpx.Fail(w, 400, "invalid_request", "Expected an optional JSON body {\"tokens\": bool}")
		return
	}
	n, err := s.Store.RevokeUserSessions(r.Context(), id)
	if err != nil {
		unavailable(w, err)
		return
	}
	var tokens int64
	if in.Tokens {
		if tokens, err = s.Store.RevokeUserTokens(r.Context(), id); err != nil {
			unavailable(w, err)
			return
		}
	}
	views := s.Runtime.RevokeUser(id)
	_ = s.Store.Audit(r.Context(), actor(r), "user.revoke_sessions", id, map[string]any{"sessions": n, "views": views, "tokens": tokens})
	httpx.OK(w, map[string]any{"sessions": n, "views": views, "tokens": tokens})
}

// adminUserTokens lists a user's personal API tokens (metadata only).
func (s *Server) adminUserTokens(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.Store.User(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.Fail(w, 404, "not_found", "User not found")
			return
		}
		unavailable(w, err)
		return
	}
	list, err := s.Store.Tokens(r.Context(), id)
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

// adminRevokeToken revokes any user's token, for example one that leaked.
func (s *Server) adminRevokeToken(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.RevokeTokenByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "No live token with this ID")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "token.revoke", t.UserID, map[string]any{"token": t.ID, "name": t.Name, "scopes": t.Scopes})
	httpx.OK(w, map[string]bool{"revoked": true})
}

func (s *Server) adminGrant(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Days   int    `json:"days"`
		Reason string `json:"reason"`
		Plan   string `json:"plan"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || in.Days < 1 || in.Days > 3660 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 500 {
		httpx.Fail(w, 400, "invalid_request", "Days (1–3660) and a reason are required")
		return
	}
	if in.Plan != "" {
		rev, err := s.Store.ActiveSettings(r.Context())
		if err != nil {
			unavailable(w, err)
			return
		}
		if p, ok := rev.Document.Plan(in.Plan); !ok || p.Key == "free" {
			httpx.Fail(w, 400, "invalid_request", "Unknown plan")
			return
		}
	}
	if _, err := s.Store.User(r.Context(), r.PathValue("id")); err != nil {
		httpx.Fail(w, 404, "not_found", "User not found")
		return
	}
	g, err := s.Store.GrantAccess(r.Context(), r.PathValue("id"), actor(r), time.Now().Add(time.Duration(in.Days)*24*time.Hour), in.Reason, in.Plan)
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "user.access_grant", g.UserID, map[string]any{"until": g.Until, "reason": in.Reason, "plan": in.Plan})
	httpx.OK(w, g)
}

func (s *Server) adminRevokeGrants(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.RevokeAccessGrants(r.Context(), id); err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "user.access_revoke", id, nil)
	if u, err := s.Store.User(r.Context(), id); err == nil {
		if e, _, err := s.entitlement(r.Context(), u); err == nil && !e.Paid() {
			s.Runtime.RevokeUser(id)
		}
	}
	httpx.OK(w, map[string]bool{"ok": true})
}

// adminRevokeGrant revokes one complimentary grant and, when the customer is
// no longer paid, ends their live views.
func (s *Server) adminRevokeGrant(w http.ResponseWriter, r *http.Request) {
	g, err := s.Store.RevokeAccessGrant(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "No live grant with this ID")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "user.access_revoke", g.UserID, map[string]any{"grant": g.ID, "plan": g.Plan})
	if u, err := s.Store.User(r.Context(), g.UserID); err == nil {
		if e, _, err := s.entitlement(r.Context(), u); err == nil && !e.Paid() {
			s.Runtime.RevokeUser(g.UserID)
		}
	}
	httpx.OK(w, g)
}

// adminSubscriptions lists subscription projections across customers.
func (s *Server) adminSubscriptions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := s.Store.AllSubscriptions(r.Context(), r.URL.Query().Get("status"), limit)
	if err != nil {
		unavailable(w, err)
		return
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	out := []map[string]any{}
	for _, sub := range list {
		plan := ""
		if p, ok := rev.Document.PlanForPrice(sub.PriceID); ok {
			plan = p.Key
		}
		out = append(out, map[string]any{"subscription": sub.Subscription, "userId": sub.UserID, "email": sub.Email, "plan": plan})
	}
	httpx.OK(w, out)
}

// adminSubscriptionCancel schedules or withdraws the end of a subscription at
// its period end. Refunds, immediate cancellation and price changes stay in
// the Stripe dashboard.
func (s *Server) adminSubscriptionCancel(cancel bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Payments == nil {
			httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
			return
		}
		id := r.PathValue("id")
		err := s.Payments.SetCancelAtPeriodEnd(r.Context(), id, cancel)
		if errors.Is(err, store.ErrNotFound) {
			httpx.Fail(w, 404, "not_found", "Subscription not found")
			return
		}
		if err != nil {
			log.Error().Err(err).Str("subscription", id).Msg("set cancel_at_period_end")
			httpx.Fail(w, 502, "billing_unavailable", "Stripe did not accept the change")
			return
		}
		action := "billing.cancel_at_period_end"
		if !cancel {
			action = "billing.resume"
		}
		_ = s.Store.Audit(r.Context(), actor(r), action, id, nil)
		httpx.OK(w, map[string]any{"queued": true, "cancelAtPeriodEnd": cancel})
	}
}

// adminDeleteInstance deletes any instance and its data, ending its views.
func (s *Server) adminDeleteInstance(w http.ResponseWriter, r *http.Request) {
	inst, err := s.Store.Instance(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "Instance not found")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	s.Runtime.RevokeInstance(inst.ID)
	if err = s.Store.DeleteInstance(r.Context(), inst.ID); err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "instance.delete", inst.ID, map[string]any{"owner": inst.OwnerUserID, "kind": inst.Kind, "title": inst.Title})
	httpx.OK(w, map[string]bool{"deleted": true})
}

func (s *Server) adminInstances(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.AllInstances(r.Context(), r.URL.Query().Get("q"), 300)
	if err != nil {
		unavailable(w, err)
		return
	}
	views := s.Runtime.ActiveViews()
	out := []map[string]any{}
	for _, i := range list {
		item := s.instanceJSON(r, i)
		item["activeViews"] = views[i.ID]
		out = append(out, item)
	}
	httpx.OK(w, out)
}

func (s *Server) adminInstance(w http.ResponseWriter, r *http.Request) {
	inst, err := s.Store.Instance(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Instance not found")
		return
	}
	members, _ := s.Store.Members(r.Context(), inst.ID)
	usage, _ := s.Store.InstanceUsage(r.Context(), inst.ID)
	audit, _ := s.Store.AuditLog(r.Context(), inst.ID, 20)
	var calls []store.ModelCall
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT id,state,model,credits,error_code,created_at FROM model_calls WHERE instance_id=$1 AND state IN ('failed','review') ORDER BY created_at DESC LIMIT 20`, inst.ID)
	if err == nil {
		for rows.Next() {
			var c store.ModelCall
			if rows.Scan(&c.ID, &c.State, &c.Model, &c.Credits, &c.ErrorCode, &c.CreatedAt) == nil {
				calls = append(calls, c)
			}
		}
		rows.Close()
	}
	out := s.instanceJSON(r, inst)
	out["membersList"] = members
	out["usage"] = usage
	out["activeViews"] = s.Runtime.ActiveViews()[inst.ID]
	out["recentFailures"] = calls
	out["audit"] = audit
	httpx.OK(w, out)
}

func (s *Server) adminInstanceState(state string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.Store.SetInstanceState(r.Context(), id, state); err != nil {
			httpx.Fail(w, 404, "not_found", "Instance not found")
			return
		}
		if state == "suspended" {
			s.Runtime.RevokeInstance(id)
		}
		_ = s.Store.Audit(r.Context(), actor(r), "instance."+state, id, nil)
		httpx.OK(w, map[string]string{"state": state})
	}
}

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	history, _ := s.Store.SettingsHistory(r.Context(), 10)
	models := []map[string]any{}
	for _, m := range uniqueModels(s.Backend.Models()) {
		models = append(models, map[string]any{"selector": m.Provider + "/" + m.ID, "name": m.Name, "thinkingLevels": m.ThinkingLevels, "images": m.Images})
	}
	// Secret configuration is reported as configured/not configured only.
	secrets := map[string]bool{"oidc": s.site(r).OIDC.Configured(), "stripe": s.Payments != nil, "modelProvider": len(s.Backend.Models()) > 0}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(rev.Revision, 10)))
	httpx.OK(w, map[string]any{"active": rev, "history": history, "providerModels": models, "secrets": secrets, "defaults": settings.Defaults()})
}

// adminSaveSettings replaces the settings document. If-Match must name the
// revision the edit started from (the ETag of GET /api/admin/settings), so a
// save cannot silently undo one made in between.
func (s *Server) adminSaveSettings(w http.ResponseWriter, r *http.Request) {
	match := r.Header.Get("If-Match")
	if match == "" {
		httpx.Fail(w, 428, "revision_required", "Send If-Match with the settings revision you edited")
		return
	}
	unq, err := strconv.Unquote(match)
	if err != nil {
		unq = match
	}
	base, err := strconv.ParseInt(unq, 10, 64)
	if err != nil || base < 0 {
		httpx.Fail(w, 400, "invalid_request", "If-Match must be a settings revision")
		return
	}
	var raw json.RawMessage
	if err := httpx.Read(w, r, &raw, 64<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	doc, err := settings.Parse(raw)
	if err != nil {
		httpx.Fail(w, 422, "invalid_settings", err.Error())
		return
	}
	rev, err := s.Store.SaveSettingsIf(r.Context(), actor(r), base, doc)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 412, "settings_conflict", "Settings changed since you loaded them; reload and apply your change again")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(rev.Revision, 10)))
	httpx.OK(w, rev)
}

func (s *Server) adminBilling(w http.ResponseWriter, r *http.Request) {
	health, err := s.Store.BillingHealth(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	events, _ := s.Store.Events(r.Context(), r.URL.Query().Get("state"), 100)
	attempts, _ := s.Store.CheckoutAttempts(r.Context(), "", 50)
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	prices := []string{}
	for id := range rev.Document.Prices() {
		prices = append(prices, id)
	}
	sort.Strings(prices)
	httpx.OK(w, map[string]any{"enabled": s.Payments != nil, "prices": prices, "health": health, "events": events, "checkouts": attempts})
}

func (s *Server) adminReconcile(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	var in struct {
		UserID string `json:"userId"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || in.UserID == "" {
		httpx.Fail(w, 400, "invalid_request", "userId is required")
		return
	}
	if err := s.Payments.ReconcileUser(r.Context(), in.UserID); err != nil {
		httpx.Fail(w, 404, "no_customer", "That user has no Stripe Customer")
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "billing.reconcile", in.UserID, nil)
	httpx.OK(w, map[string]bool{"queued": true})
}

func (s *Server) adminRetryEvent(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.RetryEvent(r.Context(), r.PathValue("id")); err != nil {
		httpx.Fail(w, 404, "not_found", "Event not retryable")
		return
	}
	if s.Payments != nil {
		s.Payments.Wake()
	}
	_ = s.Store.Audit(r.Context(), actor(r), "billing.retry_event", r.PathValue("id"), nil)
	httpx.OK(w, map[string]bool{"queued": true})
}

func (s *Server) adminUsage(w http.ResponseWriter, r *http.Request) {
	summary, err := s.Store.UsageSummary(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	calls, err := s.Store.ModelCalls(r.Context(), r.URL.Query().Get("user"), limit)
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]any{"summary": summary, "calls": calls})
}

func (s *Server) adminAdjust(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UserID  string `json:"userId"`
		Credits int    `json:"credits"`
		Reason  string `json:"reason"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || in.UserID == "" || in.Credits == 0 || in.Credits < -100000 || in.Credits > 100000 || strings.TrimSpace(in.Reason) == "" {
		httpx.Fail(w, 400, "invalid_request", "userId, nonzero credits (±100000) and a reason are required")
		return
	}
	if _, err := s.Store.User(r.Context(), in.UserID); err != nil {
		httpx.Fail(w, 404, "not_found", "User not found")
		return
	}
	u, err := s.Store.User(r.Context(), in.UserID)
	if err != nil {
		unavailable(w, err)
		return
	}
	e, _, err := s.entitlement(r.Context(), u)
	if err != nil {
		unavailable(w, err)
		return
	}
	if err = s.Store.AdjustAllowance(r.Context(), in.UserID, actor(r), in.Credits, e.Plan.MonthlyCredits, in.Reason); err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"ok": true})
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 200
	}
	list, err := s.Store.AuditQuery(r.Context(), store.AuditFilter{Subject: q.Get("subject"), Actor: q.Get("actor"), Action: q.Get("action"), Before: before, Limit: limit})
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

// uniqueModels keeps the first entry per provider/id selector. The Pi worker
// lists its local quick/default/complex tier models ahead of the catalog, so
// a model that backs several tiers appears several times; the hosted catalog
// sets tiers in its own settings and needs each model once.
func uniqueModels(in []piworker.Model) []piworker.Model {
	seen := map[string]bool{}
	out := make([]piworker.Model, 0, len(in))
	for _, m := range in {
		sel := m.Provider + "/" + m.ID
		if seen[sel] {
			continue
		}
		seen[sel] = true
		m.Tier = ""
		out = append(out, m)
	}
	return out
}
