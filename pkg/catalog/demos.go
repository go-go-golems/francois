package catalog

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// Visitor demos: a short-lived instance of an artifact's latest release for
// someone who has no account (or no plan). The visitor gets a guest account
// and a portal session bounded by the demo lifetime, then launches the
// instance through the normal launch handshake. authorizeInstance admits a
// demo only for its owner and only until it expires; model calls draw on a
// per-demo ledger and a shared daily budget (Server.budget).

// demoKeep is how long expired demos are kept (suspended) before deletion,
// so the daily budget and usage reports can still see their calls.
const demoKeep = 7 * 24 * time.Hour

// limiter is a per-key sliding-window counter.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

// available prunes expired hits and checks a limit without spending a start.
// The caller holds mu through lookup, creation and recording successful starts.
func (l *limiter) available(key string, limit int, window time.Duration, now time.Time) bool {
	cut := now.Add(-window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	l.hits[key] = kept
	// Bound memory: forget idle keys occasionally.
	if len(l.hits) > 10000 {
		for k, v := range l.hits {
			if len(v) == 0 || v[len(v)-1].Before(cut) {
				delete(l.hits, k)
			}
		}
	}
	return len(kept) < limit
}

// left reports whether key has a hit left in its window, without spending it.
func (l *limiter) left(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.available(key, limit, window, time.Now())
}

// take spends one of key's hits, or reports that none are left.
func (l *limiter) take(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if !l.available(key, limit, window, now) {
		return false
	}
	l.hits[key] = append(l.hits[key], now)
	return true
}

// clientAddress is the visitor's address: the reverse proxy's X-Real-Ip
// (or first X-Forwarded-For hop) when the deployment trusts proxy headers,
// otherwise the connection's remote address.
func (s *Server) clientAddress(r *http.Request) string {
	if s.Config.TrustProxyHeaders {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-Ip")); ip != "" {
			return ip
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.TrimSpace(strings.Split(fwd, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) createDemo(w http.ResponseWriter, r *http.Request) {
	// Public endpoint that creates state and may set a cookie: refuse
	// cross-site requests explicitly (guard checks Origin only for
	// session-level routes).
	if !s.sameOrigin(r) {
		httpx.Fail(w, 403, "invalid_origin", "Cross-origin request rejected")
		return
	}
	var in struct {
		ArtifactID string `json:"artifactId"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || in.ArtifactID == "" {
		httpx.Fail(w, 400, "invalid_request", "An artifact is required")
		return
	}
	ctx := r.Context()
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Demos are unavailable")
		return
	}
	cfg := rev.Document.Demo
	a, err := s.Store.Artifact(ctx, in.ArtifactID)
	if err != nil || !cfg.Enabled || !a.DemoEnabled || a.Visibility != "listed" {
		httpx.Fail(w, 403, "demo_disabled", "There is no demo for this application")
		return
	}
	if a.EarlyAccess(time.Now()) {
		p := principal(ctx)
		if p == nil {
			httpx.Fail(w, 403, "early_access", "This artifact is in early access for Paying customers")
			return
		}
		e, err := s.entitlementIn(ctx, p.User, rev.Document)
		if err != nil {
			unavailable(w, err)
			return
		}
		if !e.Paid() {
			httpx.Fail(w, 403, "early_access", "This artifact is in early access for Paying customers")
			return
		}
	}
	rels, err := s.Store.Releases(ctx, a.ID, "published")
	if err != nil || len(rels) == 0 {
		httpx.Fail(w, 403, "demo_disabled", "There is no demo for this application")
		return
	}
	// One process serves tours. Serialize resume/check/create so simultaneous
	// Try requests spend one start, and failed creation spends none.
	s.demoLimiter.mu.Lock()
	defer s.demoLimiter.mu.Unlock()
	p := principal(ctx)
	if p != nil && p.User.Disabled {
		httpx.Fail(w, 403, "account_disabled", "This account is disabled")
		return
	}
	if p != nil {
		inst, err := s.Store.LiveDemo(ctx, p.User.ID, a.ID)
		if err == nil {
			httpx.JSON(w, 201, map[string]any{"id": inst.ID, "expiresAt": inst.ExpiresAt})
			return
		}
		if !errors.Is(err, store.ErrNotFound) {
			httpx.Fail(w, 503, "unavailable", "Could not open the visit")
			return
		}
	}
	if !rev.Document.Operations.AcceptNewVisits {
		httpx.Fail(w, 503, "visits_paused", rev.Document.Operations.Message("New visits are temporarily paused."))
		return
	}
	if p != nil && !s.demoLimiter.available("user:"+p.User.ID, cfg.PerUserPerHour, time.Hour, time.Now()) {
		httpx.Fail(w, 429, "rate_limited", "Please come back a little later.")
		return
	}
	if !s.demoLimiter.available("address:"+s.clientAddress(r), cfg.PerAddressPerHour, time.Hour, time.Now()) {
		httpx.Fail(w, 429, "rate_limited", "Too many demos from your network; try again later")
		return
	}
	ttl := time.Duration(cfg.TTLMinutes) * time.Minute
	var owner store.User
	if p != nil {
		owner = p.User
	} else {
		if owner, err = s.Store.CreateGuest(ctx); err != nil {
			httpx.Fail(w, 503, "unavailable", "Could not start the demo")
			return
		}
		token, _, err := s.Store.CreateGuestSession(ctx, owner.ID, ttl)
		if err != nil {
			httpx.Fail(w, 503, "unavailable", "Could not start the demo")
			return
		}
		s.setCookie(w, s.sessionCookie(), token, int(ttl.Seconds()))
	}
	inst, err := s.Store.CreateDemo(ctx, owner.ID, rels[0].ID, "Demo · "+a.Title, time.Now().Add(ttl), cfg.MaxActive)
	if errors.Is(err, store.ErrDemoCapacity) {
		httpx.Fail(w, 429, "demo_limit", "All demo slots are in use; try again in a few minutes")
		return
	}
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not start the demo")
		return
	}
	now := time.Now()
	if p != nil {
		key := "user:" + p.User.ID
		s.demoLimiter.hits[key] = append(s.demoLimiter.hits[key], now)
	}
	key := "address:" + s.clientAddress(r)
	s.demoLimiter.hits[key] = append(s.demoLimiter.hits[key], now)
	log.Info().Str("artifact", a.ID).Str("instance", inst.ID).Bool("guest", p == nil).Msg("Demo started")
	httpx.JSON(w, 201, map[string]any{"id": inst.ID, "expiresAt": inst.ExpiresAt})
}

// expireDemos runs until ctx ends, suspending expired demos every minute.
func (s *Server) expireDemos(ctx context.Context) error {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		ids, err := s.Store.ExpireDemos(ctx, demoKeep)
		if err != nil && ctx.Err() == nil {
			log.Warn().Err(err).Msg("Expire demos")
		}
		for _, id := range ids {
			s.Runtime.RevokeInstance(id)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
