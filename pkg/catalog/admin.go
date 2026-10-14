package catalog

import (
	"encoding/json"
	"net/http"
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
	a := func(h http.HandlerFunc) http.HandlerFunc { return s.guard(admin, h) }
	m.HandleFunc("GET /api/admin/overview", a(s.adminOverview))
	m.HandleFunc("GET /api/admin/releases", a(s.adminReleases))
	m.HandleFunc("POST /api/admin/releases", a(s.adminUpload))
	m.HandleFunc("POST /api/admin/releases/check", a(s.adminCheck))
	m.HandleFunc("POST /api/admin/releases/{id}/publish", a(s.adminPublish))
	m.HandleFunc("POST /api/admin/releases/{id}/withdraw", a(s.adminWithdraw))
	m.HandleFunc("POST /api/admin/releases/{id}/preview", a(s.adminPreview))
	m.HandleFunc("DELETE /api/admin/releases/{id}", a(s.adminDeleteDraft))
	m.HandleFunc("PATCH /api/admin/artifacts/{id}", a(s.adminArtifact))
	m.HandleFunc("GET /api/admin/users", a(s.adminUsers))
	m.HandleFunc("GET /api/admin/users/{id}", a(s.adminUser))
	m.HandleFunc("PATCH /api/admin/users/{id}", a(s.adminPatchUser))
	m.HandleFunc("POST /api/admin/users/{id}/revoke-sessions", a(s.adminRevoke))
	m.HandleFunc("POST /api/admin/users/{id}/access-grants", a(s.adminGrant))
	m.HandleFunc("DELETE /api/admin/users/{id}/access-grants", a(s.adminRevokeGrants))
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
	err := s.Store.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM users WHERE role='admin'),(SELECT count(*) FROM users WHERE disabled),
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

func (s *Server) adminReleases(w http.ResponseWriter, r *http.Request) {
	arts, err := s.Store.Artifacts(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	rels, err := s.Store.Releases(r.Context(), "")
	if err != nil {
		unavailable(w, err)
		return
	}
	pinned := map[string]int{}
	rows, err := s.Store.Pool.Query(r.Context(), `SELECT release_id,count(*) FROM instances WHERE kind='workspace' GROUP BY 1`)
	if err == nil {
		for rows.Next() {
			var id string
			var n int
			if rows.Scan(&id, &n) == nil {
				pinned[id] = n
			}
		}
		rows.Close()
	}
	list := []map[string]any{}
	for _, rel := range rels {
		item := publicRelease(rel)
		item["createdAt"] = rel.CreatedAt
		item["withdrawnAt"] = rel.WithdrawnAt
		item["freeSample"] = rel.FreeSample
		item["pinnedInstances"] = pinned[rel.ID]
		item["manifest"] = rel.Manifest
		list = append(list, item)
	}
	httpx.OK(w, map[string]any{"artifacts": arts, "releases": list})
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

// adminCheck validates an upload without storing anything.
func (s *Server) adminCheck(w http.ResponseWriter, r *http.Request) {
	u, err := s.upload(w, r)
	if err != nil {
		httpx.Fail(w, 400, "invalid_upload", err.Error())
		return
	}
	manifest, checks := releases.Validate(u)
	httpx.OK(w, map[string]any{"checks": checks, "blocking": releases.Blocking(checks), "manifest": manifest, "size": len(u.HTML)})
}

func (s *Server) adminUpload(w http.ResponseWriter, r *http.Request) {
	u, err := s.upload(w, r)
	if err != nil {
		httpx.Fail(w, 400, "invalid_upload", err.Error())
		return
	}
	rel, checks, err := s.Publisher.CreateDraft(r.Context(), actor(r), u)
	var invalid *releases.ErrInvalid
	switch {
	case errors.As(err, &invalid):
		httpx.JSON(w, 422, map[string]any{"error": httpx.Failure{Code: "invalid_release", Message: invalid.Error()}, "checks": checks})
		return
	case errors.Is(err, store.ErrConflict):
		httpx.JSON(w, 409, map[string]any{"error": httpx.Failure{Code: "version_exists", Message: err.Error()}, "checks": checks})
		return
	case err != nil:
		unavailable(w, err)
		return
	}
	item := publicRelease(rel)
	item["checks"] = checks
	httpx.JSON(w, 201, item)
}

func (s *Server) adminPublish(w http.ResponseWriter, r *http.Request) {
	rel, err := s.Store.PublishRelease(r.Context(), r.PathValue("id"), actor(r))
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "conflict", "Only a draft can be published")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, publicRelease(rel))
}

func (s *Server) adminWithdraw(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SuspendPinned bool `json:"suspendPinned"`
	}
	_ = httpx.Read(w, r, &in, 1024)
	n, err := s.Store.WithdrawRelease(r.Context(), r.PathValue("id"), actor(r), in.SuspendPinned)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "conflict", "Only a published release can be withdrawn")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	if in.SuspendPinned {
		list, _ := s.Store.AllInstances(r.Context(), "", 500)
		for _, i := range list {
			if i.ReleaseID == r.PathValue("id") {
				s.Runtime.RevokeInstance(i.ID)
			}
		}
	}
	httpx.OK(w, map[string]any{"withdrawn": true, "suspendedInstances": n})
}

// adminPreview creates an admin-owned review instance on an app origin; the
// uploaded HTML never runs in the portal DOM.
func (s *Server) adminPreview(w http.ResponseWriter, r *http.Request) {
	rel, err := s.Store.Release(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Release not found")
		return
	}
	inst, err := s.Store.PreviewInstance(r.Context(), actor(r), rel.ID)
	if errors.Is(err, store.ErrNotFound) {
		inst, err = s.Store.CreateInstance(r.Context(), actor(r), rel.ID, "Preview · "+rel.ArtifactID+" "+rel.Version, "preview", 0)
		if err == nil {
			_ = s.Store.Audit(r.Context(), actor(r), "release.preview", rel.ID, map[string]any{"instance": inst.ID})
		}
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, s.instanceJSON(r, inst))
}

func (s *Server) adminDeleteDraft(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDraft(r.Context(), r.PathValue("id")); err != nil {
		httpx.Fail(w, 409, "conflict", "Only an unreferenced draft can be deleted")
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "release.delete_draft", r.PathValue("id"), nil)
	httpx.OK(w, map[string]bool{"deleted": true})
}

func (s *Server) adminArtifact(w http.ResponseWriter, r *http.Request) {
	a, err := s.Store.Artifact(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Application not found")
		return
	}
	var in struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Tags        []string `json:"tags"`
		Visibility  *string  `json:"visibility"`
	}
	if err = httpx.Read(w, r, &in, 16<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if in.Title != nil {
		a.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.Tags != nil {
		a.Tags = in.Tags
	}
	if in.Visibility != nil {
		a.Visibility = *in.Visibility
	}
	if a.Title == "" || len(a.Title) > 120 || len(a.Description) > 2000 || (a.Visibility != "listed" && a.Visibility != "hidden") {
		httpx.Fail(w, 400, "invalid_request", "Invalid title, description or visibility")
		return
	}
	if a, err = s.Store.UpsertArtifact(r.Context(), a); err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "artifact.update", a.ID, map[string]any{"visibility": a.Visibility})
	httpx.OK(w, a)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.Users(r.Context(), r.URL.Query().Get("q"), 200)
	if err != nil {
		unavailable(w, err)
		return
	}
	out := []map[string]any{}
	for _, u := range users {
		d, err := s.access(r.Context(), u)
		if err != nil {
			unavailable(w, err)
			return
		}
		out = append(out, map[string]any{"user": u, "access": d})
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
	d, _ := s.access(ctx, u)
	subs, _ := s.Store.Subscriptions(ctx, u.ID)
	grants, _ := s.Store.AccessGrants(ctx, u.ID)
	insts, _ := s.Store.MemberInstances(ctx, u.ID)
	sessions, _ := s.Store.ActiveSessions(ctx, u.ID)
	customer, _ := s.Store.CustomerFor(ctx, u.ID)
	rev, _ := s.Store.ActiveSettings(ctx)
	allowance, _ := s.Store.AllowanceFor(ctx, u.ID, rev.Document.Allowance.MonthlyCredits, rev.Document.Allowance.DailyCredits)
	calls, _ := s.Store.ModelCalls(ctx, u.ID, 20)
	audit, _ := s.Store.AuditLog(ctx, u.ID, 20)
	list := []map[string]any{}
	for _, i := range insts {
		list = append(list, s.instanceJSON(r, i))
	}
	httpx.OK(w, map[string]any{"user": u, "access": d, "subscriptions": subs, "complimentary": grants, "instances": list, "sessions": sessions,
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

func (s *Server) adminRevoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n, err := s.Store.RevokeUserSessions(r.Context(), id)
	if err != nil {
		unavailable(w, err)
		return
	}
	views := s.Runtime.RevokeUser(id)
	_ = s.Store.Audit(r.Context(), actor(r), "user.revoke_sessions", id, map[string]any{"sessions": n, "views": views})
	httpx.OK(w, map[string]any{"sessions": n, "views": views})
}

func (s *Server) adminGrant(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Days   int    `json:"days"`
		Reason string `json:"reason"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || in.Days < 1 || in.Days > 3660 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 500 {
		httpx.Fail(w, 400, "invalid_request", "Days (1–3660) and a reason are required")
		return
	}
	if _, err := s.Store.User(r.Context(), r.PathValue("id")); err != nil {
		httpx.Fail(w, 404, "not_found", "User not found")
		return
	}
	g, err := s.Store.GrantAccess(r.Context(), r.PathValue("id"), actor(r), time.Now().Add(time.Duration(in.Days)*24*time.Hour), in.Reason)
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "user.access_grant", g.UserID, map[string]any{"until": g.Until, "reason": in.Reason})
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
		if d, err := s.access(r.Context(), u); err == nil && !d.Allowed {
			s.Runtime.RevokeUser(id)
		}
	}
	httpx.OK(w, map[string]bool{"ok": true})
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
	httpx.OK(w, map[string]any{"active": rev, "history": history, "providerModels": models, "secrets": secrets, "defaults": settings.Defaults()})
}

func (s *Server) adminSaveSettings(w http.ResponseWriter, r *http.Request) {
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
	rev, err := s.Store.SaveSettings(r.Context(), actor(r), doc)
	if err != nil {
		unavailable(w, err)
		return
	}
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
	httpx.OK(w, map[string]any{"enabled": s.Payments != nil, "priceId": s.Config.PriceID, "health": health, "events": events, "checkouts": attempts})
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
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	if err = s.Store.AdjustAllowance(r.Context(), in.UserID, actor(r), in.Credits, rev.Document.Allowance.MonthlyCredits, in.Reason); err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"ok": true})
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.AuditLog(r.Context(), r.URL.Query().Get("subject"), 200)
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
