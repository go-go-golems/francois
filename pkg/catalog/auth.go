package catalog

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// Principal is the authenticated portal caller.
type Principal struct {
	Session store.Session
	User    store.User
}

type principalKey struct{}

func (s *Server) sessionCookie() string {
	if s.Config.Secure() {
		return "__Host-francois_session"
	}
	return "francois_session_dev"
}

func (s *Server) flowCookie() string {
	if s.Config.Secure() {
		return "__Host-francois_flow"
	}
	return "francois_flow_dev"
}

func (s *Server) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: s.Config.Secure(), SameSite: http.SameSiteLaxMode})
}

// resolve loads the session principal, if any.
func (s *Server) resolve(r *http.Request) (*Principal, error) {
	c, err := r.Cookie(s.sessionCookie())
	if err != nil || c.Value == "" {
		return nil, nil
	}
	sess, u, err := s.Store.ResolveSession(r.Context(), c.Value)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Principal{Session: sess, User: u}, nil
}

func principal(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}

// Access levels declared per route.
const (
	public = iota
	session
	enabled
	admin
)

// guard enforces authentication, account state, role and CSRF + Origin for
// mutations. Hiding UI elements is never the authorization.
func (s *Server) guard(level int, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.resolve(r)
		if err != nil {
			log.Error().Err(err).Msg("resolve session")
			httpx.Fail(w, 503, "unavailable", "Session store unavailable")
			return
		}
		if level >= session && p == nil {
			httpx.Fail(w, 401, "login_required", "Sign in first")
			return
		}
		if level >= enabled && p.User.Disabled {
			httpx.Fail(w, 403, "account_disabled", "This account is disabled")
			return
		}
		if level >= admin && p.User.Role != "admin" {
			httpx.Fail(w, 403, "admin_required", "Administrator role required")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && level >= session {
			if !s.sameOrigin(r) {
				httpx.Fail(w, 403, "invalid_origin", "Cross-origin request rejected")
				return
			}
			token := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(token), []byte(p.Session.CSRF)) != 1 {
				httpx.Fail(w, 403, "csrf_failed", "Missing or invalid CSRF token")
				return
			}
		}
		h(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}
}

func (s *Server) sameOrigin(r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" {
		return o == s.Config.PortalOrigin()
	}
	// Browsers always send Origin on cross-origin POSTs; fall back to Fetch
	// metadata for older same-origin requests.
	site := r.Header.Get("Sec-Fetch-Site")
	return site == "" || site == "same-origin"
}

// safeReturn keeps post-login redirects on local portal paths.
func safeReturn(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.HasPrefix(p, "/auth/") {
		return "/"
	}
	u, err := url.Parse(p)
	if err != nil || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	return u.RequestURI()
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.OIDC.Configured() {
		httpx.Fail(w, 503, "login_unavailable", "Sign-in is not configured")
		return
	}
	state, nonce, verifier, binding := store.RandomHex(24), store.RandomHex(16), store.RandomHex(32), store.RandomHex(24)
	ret := safeReturn(r.URL.Query().Get("return"))
	if err := s.Store.SaveLoginFlow(r.Context(), state, store.LoginFlow{BindingHash: store.Hash(binding), Nonce: nonce, Verifier: verifier, ReturnPath: ret}, 10*time.Minute); err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not start sign-in")
		return
	}
	prompt := ""
	if r.URL.Query().Get("signup") == "1" && s.Config.SignupAllowed {
		prompt = "create"
	}
	target, err := s.OIDC.AuthURL(r.Context(), state, nonce, verifier, prompt)
	if err != nil {
		log.Warn().Err(err).Msg("OIDC discovery failed")
		http.Redirect(w, r, "/?login_error=provider_unavailable", http.StatusSeeOther)
		return
	}
	s.setCookie(w, s.flowCookie(), binding, 600)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fail := func(code string) {
		s.setCookie(w, s.flowCookie(), "", -1)
		http.Redirect(w, r, "/?login_error="+code, http.StatusSeeOther)
	}
	if q.Get("error") != "" {
		fail("denied")
		return
	}
	c, err := r.Cookie(s.flowCookie())
	if err != nil || c.Value == "" || q.Get("state") == "" || q.Get("code") == "" {
		fail("flow_missing")
		return
	}
	flow, err := s.Store.ConsumeLoginFlow(r.Context(), q.Get("state"), store.Hash(c.Value))
	if err != nil {
		fail("flow_invalid")
		return
	}
	id, idToken, err := s.OIDC.Exchange(r.Context(), q.Get("code"), flow.Verifier, flow.Nonce)
	if err != nil {
		log.Warn().Err(err).Msg("OIDC exchange failed")
		fail("exchange_failed")
		return
	}
	u, err := s.Store.UpsertIdentity(r.Context(), id)
	if err != nil {
		fail("account_unavailable")
		return
	}
	token, _, err := s.Store.CreateSession(r.Context(), u.ID, idToken)
	if err != nil {
		fail("session_unavailable")
		return
	}
	s.setCookie(w, s.flowCookie(), "", -1)
	s.setCookie(w, s.sessionCookie(), token, int(store.SessionAbsolute.Seconds()))
	http.Redirect(w, r, flow.ReturnPath, http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	if err := s.Store.RevokeSession(r.Context(), p.Session.Hash); err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not sign out")
		return
	}
	s.Runtime.RevokeSession(p.Session.Hash)
	s.setCookie(w, s.sessionCookie(), "", -1)
	out := map[string]any{"ok": true}
	// End the identity provider session too, so the next sign-in asks for
	// credentials instead of silently reusing it. The portal session is
	// already gone if this fails; the browser then simply stays signed out
	// of Francois only.
	if s.OIDC.Configured() {
		target, err := s.OIDC.LogoutURL(r.Context(), p.Session.IDToken, s.Config.PortalOrigin()+"/", "")
		if err != nil {
			log.Warn().Err(err).Msg("Identity provider logout unavailable")
		} else if target != "" {
			out["redirect"] = target
		}
	}
	httpx.OK(w, out)
}
