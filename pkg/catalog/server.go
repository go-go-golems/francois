// Package catalog is the hosted subscriber platform: a trusted portal/admin
// origin plus isolated per-instance application origins, served by one
// process over PostgreSQL.
package catalog

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/hosted"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/payments"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	// PublicURL is the trusted portal origin, e.g. https://francois.example.
	PublicURL *url.URL
	// AppDomain hosts instance origins as i-<id>.<AppDomain>.
	AppDomain string
	// AppScheme/AppPort default to the portal's scheme and port.
	AppScheme string
	AppPort   string
	// TrialAllowed enables access while a subscription is trialing.
	TrialAllowed bool
	// BillingRequired is false when no Stripe configuration exists; access
	// then comes only from complimentary grants.
	PriceID string
	// GrantTTL bounds instance runtime grants; zero means 30 minutes.
	GrantTTL time.Duration
}

type Server struct {
	Config    Config
	Store     *store.Store
	Publisher *releases.Publisher
	OIDC      *identity.Provider
	Payments  *payments.Service // nil when billing is not configured
	Backend   piworker.Backend
	Runtime   *hosted.Runtime
	portal    http.Handler
}

func (c Config) Secure() bool { return c.PublicURL.Scheme == "https" }

func (c Config) PortalOrigin() string { return c.PublicURL.Scheme + "://" + c.PublicURL.Host }

func (c Config) AppOrigin(hostname string) string {
	host := hostname + "." + c.AppDomain
	if c.AppPort != "" {
		host += ":" + c.AppPort
	}
	return c.AppScheme + "://" + host
}

func New(ctx context.Context, cfg Config, s *store.Store, p *releases.Publisher, oidc *identity.Provider, pay *payments.Service, b piworker.Backend) *Server {
	if cfg.AppScheme == "" {
		cfg.AppScheme = cfg.PublicURL.Scheme
	}
	if cfg.AppPort == "" {
		cfg.AppPort = cfg.PublicURL.Port()
	}
	srv := &Server{Config: cfg, Store: s, Publisher: p, OIDC: oidc, Payments: pay, Backend: b}
	srv.Runtime = hosted.New(ctx, s, p, b, srv.authorizeInstance, hosted.Config{PortalOrigin: cfg.PortalOrigin(), AppOrigin: cfg.AppOrigin, GrantTTL: cfg.GrantTTL})
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

// Handler dispatches by exact Host: the portal origin, or a registered
// instance hostname under the application domain.
func (s *Server) Handler() http.Handler {
	portalHost := s.Config.PublicURL.Host
	suffix := "." + s.Config.AppDomain
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if host == strings.ToLower(portalHost) {
			s.portal.ServeHTTP(w, r)
			return
		}
		name := host
		if h, port, err := net.SplitHostPort(host); err == nil {
			if port != s.Config.AppPort {
				httpx.Fail(w, 421, "misdirected", "Unknown host")
				return
			}
			name = h
		} else if s.Config.AppPort != "" {
			httpx.Fail(w, 421, "misdirected", "Unknown host")
			return
		}
		if !strings.HasSuffix(name, suffix) || strings.Count(strings.TrimSuffix(name, suffix), ".") != 0 {
			httpx.Fail(w, 421, "misdirected", "Unknown host")
			return
		}
		inst, err := s.Store.InstanceByHost(r.Context(), strings.TrimSuffix(name, suffix))
		if err != nil {
			httpx.Fail(w, 404, "not_found", "Unknown instance")
			return
		}
		s.Runtime.Handler(inst).ServeHTTP(w, r)
	})
}

// authorizeInstance is the single hosted-access rule: preview instances are
// admin-only; workspaces need current entitlement for owner and caller.
func (s *Server) authorizeInstance(ctx context.Context, u store.User, inst store.Instance) error {
	if u.Disabled {
		return hosted.Denied(403, billing.ReasonDisabled, "This account is disabled")
	}
	if inst.Kind == "preview" {
		if u.Role != "admin" {
			return hosted.Denied(403, "admin_required", "Preview instances are for administrators")
		}
		return nil
	}
	rel, err := s.Store.Release(ctx, inst.ReleaseID)
	if err != nil {
		return err
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
	log.Info().Str("address", l.Addr().String()).Str("portal", s.Config.PortalOrigin()).Str("apps", "*."+s.Config.AppDomain).Msg("Francois catalog ready")
	return g.Wait()
}
