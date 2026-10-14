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
	// TrialAllowed enables access while a subscription is trialing.
	TrialAllowed bool
	// BillingRequired is false when no Stripe configuration exists; access
	// then comes only from complimentary grants.
	PriceID string
	// GrantTTL bounds instance runtime grants; zero means 30 minutes.
	GrantTTL time.Duration
	// SignupAllowed lets /auth/login?signup=1 open the identity provider's
	// registration form. Invite-only deployments leave it false.
	SignupAllowed bool
	// LegalContact is the address /privacy and /terms tell people to write
	// to; empty points them to the consent screen's support address.
	LegalContact string
}

// Validate rejects site lists the host dispatcher cannot route
// unambiguously.
func (c Config) Validate() error {
	if len(c.Sites) == 0 {
		return errors.New("at least one site is required")
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
		if site.OIDC == nil {
			return errors.Errorf("site %s has no OIDC provider", u)
		}
		host := strings.ToLower(u.Host)
		if portals[host] || domains[d] {
			return errors.Errorf("site %s repeats a portal host or application domain", u)
		}
		portals[host], domains[d] = true, true
	}
	return nil
}

func (c Config) primary() *Site { return &c.Sites[0] }

type Server struct {
	Config    Config
	Store     *store.Store
	Publisher *releases.Publisher
	Payments  *payments.Service // nil when billing is not configured
	Backend   piworker.Backend
	Runtime   *hosted.Runtime
	portal    http.Handler
	legal     *template.Template
	pages     map[string]*template.Template
	md        *content.Markdown
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
	srv := &Server{Config: cfg, Store: s, Publisher: p, Payments: pay, Backend: b, legal: template.Must(template.New("legal").Parse(web.LegalTemplate)),
		pages: parsePages(), md: content.NewMarkdown()}
	srv.Runtime = hosted.New(ctx, s, p, b, srv.authorizeInstance, hosted.Config{GrantTTL: cfg.GrantTTL})
	if pay != nil {
		pay.OnChange = func(user string) {
			// Lost entitlement closes live views promptly; new requests
			// are rechecked anyway.
			u, err := s.User(context.Background(), user)
			if err != nil {
				return
			}
			if d, err := s.Access(context.Background(), srv.policy(), u, time.Now()); err == nil && !d.Allowed {
				srv.Runtime.RevokeUser(user)
			}
		}
	}
	srv.portal = srv.routes()
	return srv
}

func (s *Server) policy() billing.Policy {
	return billing.Policy{AcceptedPrices: map[string]bool{s.Config.PriceID: s.Config.PriceID != ""}, TrialAllowed: s.Config.TrialAllowed}
}

type siteKey struct{}

// site is the portal site a request arrived on.
func (s *Server) site(r *http.Request) *Site {
	if site, ok := r.Context().Value(siteKey{}).(*Site); ok {
		return site
	}
	return s.Config.primary()
}

// Handler dispatches by exact Host: a site's portal origin, www.<portal> as a
// redirect to it, or a registered instance hostname under a site's
// application domain. An instance opened through a site is framed by that
// site's portal only.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		for i := range s.Config.Sites {
			site := &s.Config.Sites[i]
			portal := strings.ToLower(site.PublicURL.Host)
			if host == portal {
				s.portal.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), siteKey{}, site)))
				return
			}
			if host == "www."+portal {
				http.Redirect(w, r, site.PortalOrigin()+r.URL.RequestURI(), http.StatusPermanentRedirect)
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

// authorizeInstance is the single hosted-access rule: preview instances are
// for the artifact's editors; workspaces need current entitlement for owner and caller.
func (s *Server) authorizeInstance(ctx context.Context, u store.User, inst store.Instance) error {
	if u.Disabled {
		return hosted.Denied(403, billing.ReasonDisabled, "This account is disabled")
	}
	rel, err := s.Store.Release(ctx, inst.ReleaseID)
	if err != nil {
		return err
	}
	if inst.Kind == "preview" {
		if !s.canEditArtifact(ctx, u, rel.ArtifactID) {
			return hosted.Denied(403, "not_editor", "Preview instances are for the artifact's editors")
		}
		return nil
	}
	if rel.State == "withdrawn" {
		rev, err := s.Store.ActiveSettings(ctx)
		if err != nil {
			return err
		}
		if !rev.Document.Publishing.AllowPinnedAfterWithdraw {
			return hosted.Denied(403, "release_unavailable", "This release was withdrawn")
		}
	}
	now := time.Now()
	d, err := s.Store.Access(ctx, s.policy(), u, now)
	if err != nil {
		return err
	}
	if !d.Allowed {
		return hosted.Denied(403, d.Reason, "A current subscription is required")
	}
	if inst.OwnerUserID != u.ID {
		owner, err := s.Store.User(ctx, inst.OwnerUserID)
		if err != nil {
			return err
		}
		od, err := s.Store.Access(ctx, s.policy(), owner, now)
		if err != nil {
			return err
		}
		if !od.Allowed {
			return hosted.Denied(403, "owner_inactive", "The instance owner's subscription is not active; the instance is frozen")
		}
	}
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
