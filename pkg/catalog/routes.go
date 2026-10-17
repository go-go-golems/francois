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
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/catalog/web"
	"github.com/pkg/errors"
)

func (s *Server) routes() http.Handler {
	m := http.NewServeMux()
	spa := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(web.PortalIndex)
	}
	for _, p := range []string{"GET /account", "GET /instances/", "GET /studio", "GET /studio/", "GET /admin", "GET /admin/"} {
		m.HandleFunc(p, spa)
	}
	// Server-rendered storefront.
	m.HandleFunc("GET /{$}", s.guard(public, noToken, s.feedPage))
	m.HandleFunc("GET /apps", s.guard(public, noToken, s.feedPage))
	m.HandleFunc("GET /apps/{$}", s.guard(public, noToken, s.feedPage))
	m.HandleFunc("GET /apps/{id}", s.guard(public, noToken, s.artifactPage))
	m.HandleFunc("GET /creators/{slug}", s.guard(public, noToken, s.creatorPage))
	m.HandleFunc("GET /journal", s.guard(public, noToken, s.feedPage))
	m.HandleFunc("GET /feed.xml", s.combinedFeed)
	m.HandleFunc("GET /journal/feed.xml", s.journalFeed)
	m.HandleFunc("GET /apps/feed.xml", s.releaseFeed)
	m.HandleFunc("GET /journal/{slug}", s.guard(public, noToken, s.postPage))
	m.HandleFunc("GET /media/{id}", s.media)
	m.HandleFunc("GET /pricing", s.guard(public, noToken, s.pricingPage))
	m.HandleFunc("GET /api/plans", s.publicPlans)
	m.HandleFunc("POST /api/demos", s.guard(public, noToken, s.createDemo))
	m.HandleFunc("GET /static/{name}", func(w http.ResponseWriter, r *http.Request) {
		asset, ok := web.PortalAsset(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		asset.ServeHTTP(w, r)
	})
	m.HandleFunc("GET /privacy", s.legalPage("privacy", "Privacy policy"))
	m.HandleFunc("GET /terms", s.legalPage("terms", "Terms of service"))
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
	m.HandleFunc("GET /fonts/fonts.css", s.fontCSS)
	m.HandleFunc("GET /fonts/{name}", s.fontFile)
	m.HandleFunc("GET /signin", s.guard(public, noToken, s.signinPage))
	m.HandleFunc("POST /auth/email/request", s.requestEmail)
	m.HandleFunc("GET /auth/email/verify", s.inspectEmail)
	m.HandleFunc("POST /auth/email/verify", s.verifyEmail)
	m.HandleFunc("GET /auth/login", s.login)
	m.HandleFunc("GET /auth/callback", s.callback)
	m.HandleFunc("POST /auth/logout", s.guard(session, noToken, s.logout))
	m.HandleFunc("GET /api/session", s.guard(public, noToken, s.sessionInfo))
	m.HandleFunc("GET /api/catalog", s.guard(public, noToken, s.catalog))
	m.HandleFunc("GET /api/artifacts/{id}", s.guard(public, noToken, s.artifact))
	m.HandleFunc("GET /api/releases/{release}/files/{file}/download", s.guard(public, noToken, s.download))
	m.HandleFunc("GET /api/releases/{release}/attachments/{id}/download", s.guard(public, noToken, s.downloadAttachment))
	m.HandleFunc("HEAD /api/releases/{release}/attachments/{id}/download", s.guard(public, noToken, s.downloadAttachment))
	m.HandleFunc("HEAD /api/releases/{release}/files/{file}/download", s.guard(public, noToken, s.download))
	m.HandleFunc("GET /api/account", s.guard(session, scopeAccount, s.account))
	m.HandleFunc("POST /api/billing/checkout", s.guard(enabled, scopeAccount, s.checkout))
	m.HandleFunc("POST /api/billing/portal", s.guard(session, scopeAccount, s.portalSession))
	m.HandleFunc("POST /api/billing/refresh", s.guard(session, scopeAccount, s.refresh))
	m.HandleFunc("POST /webhooks/stripe", s.webhook)
	m.HandleFunc("GET /api/instances", s.guard(session, scopeAccount, s.instances))
	m.HandleFunc("POST /api/instances/open", s.guard(enabled, scopeAccount, s.createInstance))
	m.HandleFunc("POST /api/instances", s.guard(enabled, scopeAccount, s.createInstance))
	m.HandleFunc("GET /api/instances/{id}", s.guard(session, scopeAccount, s.instance))
	m.HandleFunc("PATCH /api/instances/{id}", s.guard(enabled, scopeAccount, s.renameInstance))
	m.HandleFunc("DELETE /api/instances/{id}", s.guard(enabled, scopeAccount, s.deleteInstance))
	m.HandleFunc("POST /api/instances/{id}/launch", s.guard(enabled, noToken, s.launch)) // bound to the browser session
	m.HandleFunc("POST /api/instances/{id}/members", s.guard(enabled, scopeAccount, s.addMember))
	m.HandleFunc("DELETE /api/instances/{id}/members/{user}", s.guard(enabled, scopeAccount, s.removeMember))
	m.HandleFunc("POST /api/instances/{id}/upgrade", s.guard(enabled, scopeAccount, s.upgrade))
	s.adminRoutes(m)
	s.studioRoutes(m)
	s.tokenRoutes(m)
	if s.Config.MCP {
		if err := s.mountMCP(m); err != nil {
			log.Error().Err(err).Msg("mount /mcp")
		}
	}
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { httpx.Fail(w, 404, "not_found", "Unknown API route") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		site := s.site(r)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-src "+site.AppScheme+"://*."+site.AppDomain+portSuffix(site.AppPort)+"; img-src 'self' data:; media-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		m.ServeHTTP(w, r)
	})
}

// legalUpdated is the date shown on /privacy and /terms; change it with
// their text in web/legal.tmpl.
const legalUpdated = "16 October 2026"

// legalPage serves the privacy policy or terms as plain HTML, named after
// the site the request arrived on, so identity-provider consent screens and
// crawlers can read them without JavaScript.
func (s *Server) legalPage(page, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		data := map[string]string{"Page": page, "Title": title, "Site": s.site(r).PublicURL.Hostname(), "Contact": s.Config.LegalContact, "Updated": legalUpdated}
		if err := s.legal.Execute(w, data); err != nil {
			log.Error().Err(err).Str("page", page).Msg("render legal page")
		}
	}
}

func portSuffix(p string) string {
	if p == "" {
		return ""
	}
	return ":" + p
}

func (s *Server) sessionInfo(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	site := s.site(r)
	out := map[string]any{"authenticated": p != nil, "loginAvailable": site.OIDC.Configured() || s.Config.EmailLogin != nil, "signupAvailable": site.OIDC.Configured() && s.Config.SignupAllowed, "billingEnabled": s.Payments != nil, "appDomain": site.AppDomain,
		"typeCredit": s.fonts.present}
	// The SPA shows the same brand in its crumb as the server-rendered pages.
	if rev, err := s.Store.ActiveSettings(r.Context()); err == nil {
		out["brand"] = s.brand(r, rev.Document)
	}
	if p != nil {
		e, _, err := s.entitlement(r.Context(), p.User)
		if err != nil {
			httpx.Fail(w, 503, "unavailable", "Account unavailable")
			return
		}
		out["user"] = map[string]any{"id": p.User.ID, "name": p.User.DisplayName, "email": p.User.Email, "emailVerified": p.User.EmailVerified, "role": p.User.Role, "disabled": p.User.Disabled}
		out["csrfToken"] = p.Session.CSRF
		out["access"] = e.Decision
		out["plan"] = map[string]any{"key": e.Plan.Key, "name": e.Plan.Name, "rank": e.Plan.Rank, "source": e.Source, "paid": e.Paid(), "instances": e.Plan.Instances}
	}
	httpx.OK(w, out)
}

// publicRelease is release metadata without storage keys or bytes.
func publicRelease(r store.Release) map[string]any {
	// Only capabilities the hosted runtime provides are advertised.
	caps := releaseCaps(r)
	access := "subscriber_required"
	if r.FreeSample {
		access = "free_sample"
	}
	return map[string]any{"id": r.ID, "artifactId": r.ArtifactID, "version": r.Version, "changelog": r.Changelog, "state": r.State,
		"publishedAt": r.PublishedAt, "capabilities": caps, "files": r.Files, "attachments": publicAttachments(r), "downloadAccess": access}
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
	credits, err := s.Store.ArtifactCreators(r.Context(), nil)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Catalog unavailable")
		return
	}
	out := []map[string]any{}
	for _, a := range arts {
		rel, ok := latest[a.ID]
		if a.Visibility != "listed" || !ok {
			continue
		}
		creators := []map[string]string{}
		for _, c := range credits[a.ID] {
			creators = append(creators, map[string]string{"slug": c.Slug, "name": c.Name})
		}
		out = append(out, map[string]any{"id": a.ID, "title": a.Title, "description": a.Description, "tags": a.Tags, "updatedAt": a.UpdatedAt, "releases": counts[a.ID], "latest": publicRelease(rel),
			"cover": a.Page.Cover, "creators": creators, "featured": a.Featured, "requiredPlan": a.RequiredPlan, "demo": a.DemoEnabled, "url": "/apps/" + a.ID})
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

// mayDownload applies the download rule for a published release's files and
// attachments, writing the JSON denial when it fails: anyone may download a
// free sample; otherwise the caller must be signed in with a plan that can
// run the artifact.
func (s *Server) mayDownload(w http.ResponseWriter, r *http.Request, rel store.Release) bool {
	p := principal(r.Context())
	if p != nil && p.User.Disabled {
		httpx.Fail(w, 403, billing.ReasonDisabled, "This account is disabled")
		return false
	}
	if rel.FreeSample {
		return true
	}
	if p == nil {
		httpx.Fail(w, 401, "login_required", "Sign in to download")
		return false
	}
	a, err := s.Store.Artifact(r.Context(), rel.ArtifactID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Access check failed")
		return false
	}
	e, doc, err := s.entitlement(r.Context(), p.User)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Access check failed")
		return false
	}
	if !e.CanRun(a, doc) {
		httpx.Fail(w, 403, "plan_required", "Your plan does not include this application")
		return false
	}
	return true
}

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
	if !s.mayDownload(w, r, rel) {
		return
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
	e, doc, err := s.entitlement(ctx, p.User)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	subs, err := s.Store.Subscriptions(ctx, p.User.ID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	allowance, err := s.Store.AllowanceFor(ctx, p.User.ID, e.Plan.MonthlyCredits, e.Plan.DailyCredits)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Account unavailable")
		return
	}
	grants, _ := s.Store.AccessGrants(ctx, p.User.ID)
	_, cerr := s.Store.CustomerFor(ctx, p.User.ID)
	attempts, _ := s.Store.CheckoutAttempts(ctx, p.User.ID, 5)
	sessions, _ := s.Store.ActiveSessions(ctx, p.User.ID)
	models := []map[string]any{}
	for _, m := range doc.Models.Allowed {
		if e.AllowsModel(m.Selector) {
			models = append(models, map[string]any{"selector": m.Selector, "label": m.Label, "credits": m.Credits, "thinking": m.Thinking})
		}
	}
	httpx.OK(w, map[string]any{
		"user":           map[string]any{"id": p.User.ID, "name": p.User.DisplayName, "email": p.User.Email, "emailVerified": p.User.EmailVerified, "role": p.User.Role, "disabled": p.User.Disabled, "createdAt": p.User.CreatedAt},
		"access":         e.Decision,
		"entitlement":    e,
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
	case errors.Is(err, payments.ErrCheckoutSuperseded):
		httpx.Fail(w, 409, "checkout_superseded", "A checkout for another plan replaced this one")
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
		Plan     string `json:"plan"`
		Interval string `json:"interval"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil {
		httpx.Fail(w, 400, "invalid_request", "A plan is required")
		return
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Billing is temporarily unavailable")
		return
	}
	price, ok := checkoutPrice(rev.Document, in.Plan, in.Interval)
	if !ok {
		httpx.Fail(w, 400, "invalid_request", "That plan cannot be bought")
		return
	}
	p := principal(r.Context())
	if !p.User.EmailVerified {
		httpx.Fail(w, 403, "email_unverified", "Verify your email address with the identity provider first")
		return
	}
	if p.User.Role == "guest" {
		httpx.Fail(w, 403, "guest", "Sign in to subscribe")
		return
	}
	u, err := s.Payments.Checkout(r.Context(), p.User, price, rev.Document.Prices(), s.site(r).PortalOrigin())
	if err != nil {
		s.billingFailure(w, err)
		return
	}
	httpx.OK(w, map[string]string{"url": u})
}

// checkoutPrice picks the server-side price for a public plan and interval
// (the first price of that interval; any price when no interval is given).
// The client never names a price.
func checkoutPrice(doc settings.Document, key, interval string) (string, bool) {
	p, ok := doc.Plan(key)
	if !ok || !p.Public {
		return "", false
	}
	for _, pr := range p.Prices {
		if interval == "" || pr.Interval == interval {
			return pr.PriceID, true
		}
	}
	return "", false
}

func (s *Server) portalSession(w http.ResponseWriter, r *http.Request) {
	if s.Payments == nil {
		httpx.Fail(w, 503, "billing_unavailable", "Billing is not configured")
		return
	}
	u, err := s.Payments.Portal(r.Context(), principal(r.Context()).User, s.site(r).PortalOrigin())
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
