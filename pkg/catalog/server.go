// Package catalog is the hosted subscriber platform: a trusted portal/admin
// origin plus isolated per-instance application origins, served by one
// process over PostgreSQL.
package catalog

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/content"
	"github.com/go-go-golems/francois/pkg/catalog/hosted"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/payments"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/catalog/web"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// Site is one portal origin and the application domain whose instance
// origins it frames. Every site serves the same catalog, users and instances;
// only absolute URLs (sign-in redirect, Stripe return, instance origins)
// differ.
type Site struct {
	// PublicURL is the trusted portal origin, e.g. https://francois.example.
	PublicURL *url.URL
	// AppDomain hosts instance origins as i-<id>.<AppDomain>. It may equal
	// the portal hostname: the portal is the bare name, instances are its
	// single-label children.
	AppDomain string
	// AppScheme/AppPort default to the portal's scheme and port.
	AppScheme string
	AppPort   string
	// OIDC signs users in with this site's callback URL.
	OIDC *identity.Provider
}

func (c Site) PortalOrigin() string { return c.PublicURL.Scheme + "://" + c.PublicURL.Host }

func (c Site) AppOrigin(hostname string) string {
	host := hostname + "." + c.AppDomain
	if c.AppPort != "" {
		host += ":" + c.AppPort
	}
	return c.AppScheme + "://" + host
}

// instanceName returns the instance label of an application host on this
// site, e.g. "i-abc" for i-abc.apps.example, and whether the host belongs to
// it. The second result reports a host on this domain with the wrong port.
func (c Site) instanceName(host string) (string, bool, bool) {
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		h, port = host, ""
	}
	label, found := strings.CutSuffix(h, "."+strings.ToLower(c.AppDomain))
	if !found || label == "" || strings.Contains(label, ".") {
		return "", false, false
	}
	if port != c.AppPort {
		return "", true, false
	}
	return label, false, true
}

type Config struct {
	// Sites are the portal origins served by this process. The first is the
	// primary site; all sites must share its scheme because session cookies
	// use the __Host- prefix on HTTPS.
	Sites []Site
	// EmailLogin is nil while email-only supporter sign-in is disabled.
	EmailLogin *identity.EmailLogin
	// Provider IDs are server-configured ZITADEL identities, never browser input.
	GoogleIDP string
	GitHubIDP string
	// TrialAllowed enables access while a subscription is trialing. Prices
	// come from the plans in the settings document.
	TrialAllowed bool
	// GrantTTL bounds instance runtime grants; zero means 30 minutes.
	GrantTTL time.Duration
	// SignupAllowed lets /auth/login?signup=1 open the identity provider's
	// registration form. Invite-only deployments leave it false.
	SignupAllowed bool
	// LegalContact is the address /privacy and /terms tell people to write
	// to; empty points them to the consent screen's support address.
	LegalContact string
	// TrustProxyHeaders takes visitors' addresses from X-Real-Ip or
	// X-Forwarded-For (set by the ingress) for demo rate limits.
	TrustProxyHeaders bool
	// FontDir is a private directory of licensed WOFF2 web fonts (see
	// fonts.go); empty serves none.
	FontDir string
	// MCP serves the MCP tools at /mcp on every portal for remote agents
	// holding a personal API token.
	MCP bool
	// RedirectHosts are retired portal hosts: every request to one is
	// redirected (308, method and body kept) to the primary portal origin.
	RedirectHosts []string
	// HSTSMaxAge is sent as Strict-Transport-Security on every response
	// when the primary site is HTTPS; zero sends none.
	HSTSMaxAge time.Duration
	// ContactEmail is the public contact address shown on storefront pages
	// and in the SPA; empty shows none.
	ContactEmail string
}

// Validate rejects site lists the host dispatcher cannot route
// unambiguously.
func (c Config) Validate() error {
	for _, id := range []string{c.GoogleIDP, c.GitHubIDP} {
		if strings.ContainsAny(id, " \t\r\n") {
			return errors.New("ZITADEL identity provider IDs must not contain whitespace")
		}
	}
	if len(c.Sites) == 0 {
		return errors.New("at least one site is required")
	}
	if _, err := loadFonts(c.FontDir); err != nil {
		return err
	}
	portals := map[string]bool{}
	domains := map[string]bool{}
	for _, site := range c.Sites {
		u := site.PublicURL
		if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" && u.Path != "/" || u.RawQuery != "" {
			return errors.New("a site URL must be an origin such as https://francois.example")
		}
		if u.Scheme != c.Sites[0].PublicURL.Scheme {
			return errors.Errorf("site %s must use the primary site's scheme %s", u, c.Sites[0].PublicURL.Scheme)
		}
		d := strings.ToLower(site.AppDomain)
		if d == "" || strings.HasPrefix(d, ".") || strings.HasSuffix(d, ".") || strings.ContainsAny(d, "/:* ") {
			return errors.Errorf("site %s needs an application domain such as apps.example", u)
		}
		if c.EmailLogin != nil && len(site.PortalOrigin()+"/auth/email/verify?flow="+strings.Repeat("0", 64)+"&code={{.Code}}") > 200 {
			return errors.Errorf("site %s exceeds ZITADEL email link template length", u)
		}
		if site.OIDC == nil {
			return errors.Errorf("site %s has no OIDC provider", u)
		}
		host := strings.ToLower(u.Host)
		if portals[host] || domains[d] {
			return errors.Errorf("site %s repeats a portal host or application domain", u)
		}
		portals[host], domains[d] = true, true
	}
	for _, raw := range c.RedirectHosts {
		h := strings.ToLower(raw)
		if h == "" || strings.ContainsAny(h, "/*? \t") {
			return errors.Errorf("redirect host %q must be a host name such as old.example", raw)
		}
		if portals[h] || portals[strings.TrimPrefix(h, "www.")] {
			return errors.Errorf("redirect host %s is also a portal", raw)
		}
		for _, site := range c.Sites {
			// Any port: AppPort may still be empty here, defaulted in New.
			if _, misdirected, ok := site.instanceName(h); ok || misdirected {
				return errors.Errorf("redirect host %s is an instance host of %s", raw, site.PublicURL)
			}
		}
	}
	return nil
}

func (c Config) primary() *Site { return &c.Sites[0] }

type Server struct {
	Config        Config
	Store         *store.Store
	Publisher     *releases.Publisher
	Payments      *payments.Service // nil when billing is not configured
	Backend       piworker.Backend
	Runtime       *hosted.Runtime
	portal        http.Handler
	legal         *template.Template
	pages         map[string]*template.Template
	md            *content.Markdown
	emailLimiter  *limiter
	demoLimiter   *limiter
	deviceLimiter *limiter
	fonts         *fonts
	resizes       singleflight.Group
	resizeSlots   chan struct{}
	share         shareCache
}

func (c Config) Secure() bool { return c.primary().PublicURL.Scheme == "https" }

// New expects a configuration that passed Validate.
func New(ctx context.Context, cfg Config, s *store.Store, p *releases.Publisher, pay *payments.Service, b piworker.Backend) *Server {
	cfg.Sites = append([]Site(nil), cfg.Sites...)
	for i := range cfg.Sites {
		site := &cfg.Sites[i]
		if site.AppScheme == "" {
			site.AppScheme = site.PublicURL.Scheme
		}
		if site.AppPort == "" {
			site.AppPort = site.PublicURL.Port()
		}
	}
	srv := &Server{Config: cfg, Store: s, Publisher: p, Payments: pay, Backend: b, legal: template.Must(template.New("legal").Funcs(pageFuncs).Parse(web.LegalTemplate)),
		pages: parsePages(), md: content.NewMarkdown(), demoLimiter: newLimiter(), emailLimiter: newLimiter(), deviceLimiter: newLimiter(), resizeSlots: make(chan struct{}, 2)}
	var err error
	if srv.fonts, err = loadFonts(cfg.FontDir); err != nil {
		// Validate already checked the directory; serve no fonts rather than fail.
		log.Error().Err(err).Msg("Web fonts unavailable")
		srv.fonts, _ = loadFonts("")
	}
	srv.Runtime = hosted.New(ctx, s, p, b, srv.authorizeInstance, srv.budget, hosted.Config{GrantTTL: cfg.GrantTTL})
	if pay != nil {
		pay.OnChange = func(user string) {
			// Losing paid access closes live views promptly; every request
			// is rechecked anyway, so a plan change applies on the next one.
			u, err := s.User(context.Background(), user)
			if err != nil {
				return
			}
			if e, _, err := srv.entitlement(context.Background(), u); err == nil && !e.Paid() {
				srv.Runtime.RevokeUser(user)
			}
		}
	}
	srv.portal = srv.trackTraffic(srv.routes())
	return srv
}

type siteKey struct{}

// site is the portal site a request arrived on.
func (s *Server) site(r *http.Request) *Site {
	if site, ok := r.Context().Value(siteKey{}).(*Site); ok {
		return site
	}
	return s.Config.primary()
}

// servesHost reports whether host is one this server answers for: a portal,
// its www. alias, a redirect host or a host under an application domain.
func (s *Server) servesHost(host string) bool {
	for _, h := range s.Config.RedirectHosts {
		if host == strings.ToLower(h) {
			return true
		}
	}
	for _, site := range s.Config.Sites {
		portal := strings.ToLower(site.PublicURL.Host)
		if _, misdirected, ok := site.instanceName(host); host == portal || host == "www."+portal || ok || misdirected {
			return true
		}
	}
	return false
}

// Handler dispatches by exact Host: a retired redirect host, a site's portal
// origin, www.<portal> as a redirect to it, or a registered instance hostname
// under a site's application domain. An instance opened through a site is
// framed by that site's portal only. On HTTPS deployments every response
// carries HSTS, and a plain-HTTP request reported by a trusted ingress is
// upgraded first.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if s.Config.Secure() && s.Config.HSTSMaxAge > 0 {
			w.Header().Set("Strict-Transport-Security", "max-age="+strconv.Itoa(int(s.Config.HSTSMaxAge.Seconds()))+"; includeSubDomains")
		}
		if s.Config.Secure() && s.Config.TrustProxyHeaders && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "http") && s.servesHost(host) {
			http.Redirect(w, r, "https://"+host+r.URL.RequestURI(), http.StatusPermanentRedirect) // #nosec G710 -- host is one this server is configured to serve
			return
		}
		for _, h := range s.Config.RedirectHosts {
			if host == strings.ToLower(h) {
				http.Redirect(w, r, s.Config.primary().PortalOrigin()+r.URL.RequestURI(), http.StatusPermanentRedirect) // #nosec G710 -- the destination is always the primary portal origin
				return
			}
		}
		for i := range s.Config.Sites {
			site := &s.Config.Sites[i]
			portal := strings.ToLower(site.PublicURL.Host)
			if host == portal {
				s.portal.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), siteKey{}, site)))
				return
			}
			if host == "www."+portal {
				http.Redirect(w, r, site.PortalOrigin()+r.URL.RequestURI(), http.StatusPermanentRedirect) // #nosec G710 -- the destination is always this site's portal origin; RequestURI adds only path and query
				return
			}
		}
		for i := range s.Config.Sites {
			site := &s.Config.Sites[i]
			name, misdirected, ok := site.instanceName(host)
			if misdirected {
				httpx.Fail(w, 421, "misdirected", "Unknown host")
				return
			}
			if !ok {
				continue
			}
			inst, err := s.Store.InstanceByHost(r.Context(), name)
			if err != nil {
				httpx.Fail(w, 404, "not_found", "Unknown instance")
				return
			}
			s.Runtime.Handler(inst, site.PortalOrigin(), site.AppOrigin(inst.Hostname)).ServeHTTP(w, r)
			return
		}
		httpx.Fail(w, 421, "misdirected", "Unknown host")
	})
}

// authorizeInstance is the single hosted-access rule, applied at launch and
// on every runtime request:
//   - preview instances: the artifact's editors;
//   - demo instances: their owner, until they expire;
//   - workspaces: caller and owner plans include the artifact, and the
//     workspace is within the owner's plan limit (oldest first).
func (s *Server) authorizeInstance(ctx context.Context, u store.User, inst store.Instance) error {
	if u.Disabled {
		return hosted.Denied(403, billing.ReasonDisabled, "This account is disabled")
	}
	rel, err := s.Store.Release(ctx, inst.ReleaseID)
	if err != nil {
		return err
	}
	switch inst.Kind {
	case "preview":
		if !s.canEditArtifact(ctx, u, rel.ArtifactID) {
			return hosted.Denied(403, "not_editor", "Preview instances are for the artifact's editors")
		}
		return nil
	case "demo":
		if inst.OwnerUserID != u.ID {
			return hosted.Denied(403, "not_member", "This demo belongs to another visitor")
		}
		if inst.ExpiresAt == nil || !time.Now().Before(*inst.ExpiresAt) {
			return hosted.Denied(403, "demo_expired", "This demo has ended")
		}
		a, err := s.Store.Artifact(ctx, rel.ArtifactID)
		if err != nil {
			return err
		}
		if a.EarlyAccess(time.Now()) {
			e, _, err := s.entitlement(ctx, u)
			if err != nil {
				return err
			}
			if !e.Paid() {
				return hosted.Denied(403, "early_access", "This artifact is in early access for Paying customers")
			}
		}
		return nil
	}
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		return err
	}
	doc := rev.Document
	if rel.State == "withdrawn" && !doc.Publishing.AllowPinnedAfterWithdraw {
		return hosted.Denied(403, "release_unavailable", "This release was withdrawn")
	}
	a, err := s.Store.Artifact(ctx, rel.ArtifactID)
	if err != nil {
		return err
	}
	caller, err := s.entitlementIn(ctx, u, doc)
	if err != nil {
		return err
	}
	if !caller.CanRun(a, doc) {
		return hosted.Denied(403, "plan_required", "This artifact is in early access for Paying customers")
	}
	if inst.OwnerUserID != u.ID {
		ou, err := s.Store.User(ctx, inst.OwnerUserID)
		if err != nil {
			return err
		}
		owner, err := s.entitlementIn(ctx, ou, doc)
		if err != nil {
			return err
		}
		if !owner.CanRun(a, doc) {
			return hosted.Denied(403, "owner_inactive", "The instance owner's plan does not include this application; the instance is frozen")
		}
	}
	// Workspace limits govern creation; a downgrade does not freeze an
	// arbitrary older/newer subset of already-saved workspaces.

	return nil
}

// Run serves HTTP and runs workers under one errgroup until ctx ends.
func (s *Server) Run(ctx context.Context, listen string) error {
	if released, review, err := s.Store.RecoverCalls(ctx); err != nil {
		return errors.Wrap(err, "recover model calls")
	} else if released+review > 0 {
		log.Warn().Int64("released", released).Int64("review", review).Msg("Recovered open model calls")
	}
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := srv.Serve(l)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
	if s.Payments != nil {
		g.Go(func() error { return s.Payments.Run(ctx) })
	}
	g.Go(func() error { return s.expireDemos(ctx) })
	g.Go(func() error { return s.pruneTraffic(ctx) })
	g.Go(func() error {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
		return s.Runtime.Wait()
	})
	for _, site := range s.Config.Sites {
		log.Info().Str("address", l.Addr().String()).Str("portal", site.PortalOrigin()).Str("apps", "*."+site.AppDomain).Msg("Francois catalog ready")
	}
	return g.Wait()
}
