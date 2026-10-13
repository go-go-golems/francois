package catalog

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/payments"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/catalog/web"
	"github.com/pkg/errors"
)

func (s *Server) routes() http.Handler {
	m := http.NewServeMux()
	spa := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(web.Index)
	}
	for _, p := range []string{"GET /{$}", "GET /apps/", "GET /account", "GET /instances/", "GET /admin", "GET /admin/"} {
		m.HandleFunc(p, spa)
	}
	m.HandleFunc("GET /static/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, mime, ok := web.PortalAsset(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { httpx.OK(w, map[string]string{"status": "ok"}) })
	m.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Store.Ping(ctx); err != nil {
			httpx.Fail(w, 503, "database_unavailable", "Database unavailable")
			return
		}
		httpx.OK(w, map[string]any{"status": "ready", "models": len(s.Backend.Models()) > 0})
	})
	m.HandleFunc("GET /auth/login", s.login)
	m.HandleFunc("GET /auth/callback", s.callback)
	m.HandleFunc("POST /auth/logout", s.guard(session, s.logout))
	m.HandleFunc("GET /api/session", s.guard(public, s.sessionInfo))
	m.HandleFunc("GET /api/catalog", s.guard(public, s.catalog))
	m.HandleFunc("GET /api/artifacts/{id}", s.guard(public, s.artifact))
	m.HandleFunc("GET /api/releases/{release}/files/{file}/download", s.guard(public, s.download))
	m.HandleFunc("HEAD /api/releases/{release}/files/{file}/download", s.guard(public, s.download))
	m.HandleFunc("GET /api/account", s.guard(session, s.account))
	m.HandleFunc("POST /api/billing/checkout", s.guard(enabled, s.checkout))
	m.HandleFunc("POST /api/billing/portal", s.guard(session, s.portalSession))
	m.HandleFunc("POST /api/billing/refresh", s.guard(session, s.refresh))
	m.HandleFunc("POST /webhooks/stripe", s.webhook)
	m.HandleFunc("GET /api/instances", s.guard(session, s.instances))
	m.HandleFunc("POST /api/instances", s.guard(enabled, s.createInstance))
	m.HandleFunc("GET /api/instances/{id}", s.guard(session, s.instance))
	m.HandleFunc("PATCH /api/instances/{id}", s.guard(enabled, s.renameInstance))
	m.HandleFunc("DELETE /api/instances/{id}", s.guard(enabled, s.deleteInstance))
	m.HandleFunc("POST /api/instances/{id}/launch", s.guard(enabled, s.launch))
	m.HandleFunc("POST /api/instances/{id}/members", s.guard(enabled, s.addMember))
	m.HandleFunc("DELETE /api/instances/{id}/members/{user}", s.guard(enabled, s.removeMember))
	m.HandleFunc("POST /api/instances/{id}/upgrade", s.guard(enabled, s.upgrade))
	s.adminRoutes(m)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { httpx.Fail(w, 404, "not_found", "Unknown API route") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-src "+s.Config.AppScheme+"://*."+s.Config.AppDomain+portSuffix(s.Config.AppPort)+"; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		m.ServeHTTP(w, r)
	})
}

func portSuffix(p string) string {
	if p == "" {
		return ""
	}
	return ":" + p
}

func (s *Server) access(ctx context.Context, u store.User) (billing.Decision, error) {
	return s.Store.Access(ctx, s.policy(), u, time.Now())
}

func (s *Server) sessionInfo(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	out := map[string]any{"authenticated": p != nil, "loginAvailable": s.OIDC.Configured(), "signupAvailable": s.OIDC.Configured() && s.Config.SignupAllowed, "billingEnabled": s.Payments != nil, "appDomain": s.Config.AppDomain}
	if p != nil {
		d, err := s.access(r.Context(), p.User)
		if err != nil {
			httpx.Fail(w, 503, "unavailable", "Account unavailable")
			return
		}
		out["user"] = map[string]any{"id": p.User.ID, "name": p.User.DisplayName, "email": p.User.Email, "emailVerified": p.User.EmailVerified, "role": p.User.Role, "disabled": p.User.Disabled}
		out["csrfToken"] = p.Session.CSRF
		out["access"] = d
	}
	httpx.OK(w, out)
}

// publicRelease is release metadata without storage keys or bytes.
func publicRelease(r store.Release) map[string]any {
	// Only capabilities the hosted runtime provides are advertised.
	caps := []string{}
	for _, k := range []string{"sample", "db", "room", "user", "assets", "comments", "downloads"} {
		if _, ok := r.Manifest[k]; ok {
			caps = append(caps, k)
		}
	}
	access := "subscriber_required"
	if r.FreeSample {
		access = "free_sample"
	}
	return map[string]any{"id": r.ID, "artifactId": r.ArtifactID, "version": r.Version, "changelog": r.Changelog, "state": r.State,
		"publishedAt": r.PublishedAt, "capabilities": caps, "files": r.Files, "downloadAccess": access}
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	arts, err := s.Store.Artifacts(r.Context())
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Catalog unavailable")
		return
	}
	rels, err := s.Store.Releases(r.Context(), "", "published")
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Catalog unavailable")
		return
	}
	latest := map[string]store.Release{}
	counts := map[string]int{}
	for _, rel := range rels {
		counts[rel.ArtifactID]++
		if cur, ok := latest[rel.ArtifactID]; !ok || rel.PublishedAt.After(*cur.PublishedAt) {
			latest[rel.ArtifactID] = rel
		}
	}
	out := []map[string]any{}
	for _, a := range arts {
		rel, ok := latest[a.ID]
		if a.Visibility != "listed" || !ok {
			continue
		}
		out = append(out, map[string]any{"id": a.ID, "title": a.Title, "description": a.Description, "tags": a.Tags, "updatedAt": a.UpdatedAt, "releases": counts[a.ID], "latest": publicRelease(rel)})
	}
	httpx.OK(w, out)
}

func (s *Server) artifact(w http.ResponseWriter, r *http.Request) {
	a, err := s.Store.Artifact(r.Context(), r.PathValue("id"))
	if err != nil || a.Visibility != "listed" {
		httpx.Fail(w, 404, "not_found", "Application not found")
		return
	}
	rels, err := s.Store.Releases(r.Context(), a.ID, "published")
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Catalog unavailable")
		return
	}
	if len(rels) == 0 {
		httpx.Fail(w, 404, "not_found", "Application not found")
		return
	}
	list := []map[string]any{}
	for _, rel := range rels {
		list = append(list, publicRelease(rel))
	}
	httpx.OK(w, map[string]any{"id": a.ID, "title": a.Title, "description": a.Description, "tags": a.Tags, "releases": list})
}

var safeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// download authorizes before opening or writing any bytes; denials are JSON.
func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	rel, err := s.Store.Release(r.Context(), r.PathValue("release"))
	if err != nil || rel.State != "published" {
		httpx.Fail(w, 404, "release_unavailable", "Release not available")
		return
	}
	var file *store.ReleaseFile
	for i := range rel.Files {
		if rel.Files[i].ID == r.PathValue("file") {
			file = &rel.Files[i]
		}
	}
	if file == nil {
		httpx.Fail(w, 404, "not_found", "File not found")
		return
	}
	p := principal(r.Context())
	if p != nil && p.User.Disabled {
		httpx.Fail(w, 403, billing.ReasonDisabled, "This account is disabled")
		return
	}
	if !rel.FreeSample {
		if p == nil {
			httpx.Fail(w, 401, "login_required", "Sign in to download")
			return
		}
		d, err := s.access(r.Context(), p.User)
		if err != nil {
			httpx.Fail(w, 503, "unavailable", "Access check failed")
			return
		}
		if !d.Allowed {
			httpx.Fail(w, 403, d.Reason, "A current subscription is required to download")
			return
		}
	}
	data, err := s.Publisher.Blobs.Read(*file)
	if err != nil {
		log.Error().Err(err).Str("file", file.ID).Msg("release file unreadable")
		httpx.Fail(w, 503, "file_unavailable", "File temporarily unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeName.ReplaceAllString(file.Name, "_")+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-SHA256", file.SHA256)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	ctx := r.Context()
	d, err := s.access(ctx, p.User)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	subs, err := s.Store.Subscriptions(ctx, p.User.ID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	allowance, err := s.Store.AllowanceFor(ctx, p.User.ID, rev.Document.Allowance.MonthlyCredits, rev.Document.Allowance.DailyCredits)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	grants, _ := s.Store.AccessGrants(ctx, p.User.ID)
	_, cerr := s.Store.CustomerFor(ctx, p.User.ID)
	attempts, _ := s.Store.CheckoutAttempts(ctx, p.User.ID, 5)
	sessions, _ := s.Store.ActiveSessions(ctx, p.User.ID)
	models := []map[string]any{}
	for _, m := range rev.Document.Models.Allowed {
		models = append(models, map[string]any{"selector": m.Selector, "label": m.Label, "credits": m.Credits, "thinking": m.Thinking})
	}
	httpx.OK(w, map[string]any{
		"user":           map[string]any{"id": p.User.ID, "name": p.User.DisplayName, "email": p.User.Email, "emailVerified": p.User.EmailVerified, "role": p.User.Role, "disabled": p.User.Disabled, "createdAt": p.User.CreatedAt},
		"access":         d,
		"subscriptions":  subs,
		"complimentary":  grants,
		"allowance":      allowance,
		"models":         models,
		"hasCustomer":    cerr == nil,
		"checkouts":      attempts,
		"sessions":       sessions,
		"billingEnabled": s.Payments != nil,
		"identityIssuer": p.User.Issuer,
	})
}

func (s *Server) billingFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, payments.ErrSubscriptionOpen):
		httpx.Fail(w, 409, "subscription_open", "You already have a subscription; manage it from billing")
	case errors.Is(err, payments.ErrNoCustomer):
		httpx.Fail(w, 409, "no_billing_account", "There is no billing account yet; subscribe first")
	case errors.Is(err, payments.ErrForeignSession):
		httpx.Fail(w, 403, "foreign_session", "That checkout does not belong to this account")
	default:
		log.Error().Err(err).Msg("billing request failed")
		httpx.Fail(w, 503, "billing_unavailable", "Billing is temporarily unavailable")
	}
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	var in struct {
		Plan string `json:"plan"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || in.Plan != "subscriber" {
		httpx.Fail(w, 400, "invalid_request", "Only the subscriber plan is available")
		return
	}
	p := principal(r.Context())
	if !p.User.EmailVerified {
		httpx.Fail(w, 403, "email_unverified", "Verify your email address with the identity provider first")
		return
	}
	u, err := s.Payments.Checkout(r.Context(), p.User)
	if err != nil {
		s.billingFailure(w, err)
		return
	}
	httpx.OK(w, map[string]string{"url": u})
}

func (s *Server) portalSession(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	u, err := s.Payments.Portal(r.Context(), principal(r.Context()).User)
	if err != nil {
		s.billingFailure(w, err)
		return
	}
	httpx.OK(w, map[string]string{"url": u})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	var in struct {
		SessionID string `json:"sessionId"`
	}
	_ = httpx.Read(w, r, &in, 1024)
	if err := s.Payments.Refresh(r.Context(), principal(r.Context()).User, in.SessionID); err != nil {
		s.billingFailure(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"queued": true})
}

// webhook verifies the raw body (never re-serialized) and persists the
// event before acknowledging; database failure returns 5xx for retry.
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 404, "not_found", "Webhooks are not configured")
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
	if err != nil {
		httpx.Fail(w, 400, "invalid_request", "Body too large")
		return
	}
	e, err := s.Payments.Ingest(r.Context(), payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		if e.ID == "" || strings.Contains(err.Error(), "mode") {
			log.Warn().Str("event", e.ID).Msg("rejected Stripe webhook")
			httpx.Fail(w, 400, "invalid_signature", "Invalid webhook")
			return
		}
		log.Error().Err(err).Str("event", e.ID).Msg("persist Stripe event")
		httpx.Fail(w, 503, "unavailable", "Could not persist event")
		return
	}
	log.Info().Str("event", e.ID).Str("type", e.Type).Bool("supported", e.Supported).Msg("Stripe event received")
	httpx.OK(w, map[string]bool{"received": true})
}
