package catalog

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

type emailPage struct {
	State, CSRF, Return, Flow, Code, Message string
	Available, OIDC, Google, GitHub, Signup  bool
}

func (s *Server) emailCookie() string {
	if s.Config.Secure() {
		return "__Host-francois_email"
	}
	return "francois_email_dev"
}

func (s *Server) emailBinding(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(s.emailCookie()); err == nil && validEmailSecret(c.Value) {
		s.setCookie(w, s.emailCookie(), c.Value, int(store.EmailSigninTTL.Seconds()))
		return c.Value
	}
	binding := store.RandomHex(24)
	s.setCookie(w, s.emailCookie(), binding, int(store.EmailSigninTTL.Seconds()))
	return binding
}

func validEmailSecret(v string) bool {
	if len(v) != 48 && len(v) != 64 {
		return false
	}
	for _, c := range v {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func (s *Server) renderEmail(w http.ResponseWriter, r *http.Request, status int, d emailPage) {
	d.Available = s.Config.EmailLogin != nil
	d.OIDC = s.site(r).OIDC.Configured()
	d.Google = d.OIDC && s.Config.GoogleIDP != ""
	d.GitHub = d.OIDC && s.Config.GitHubIDP != ""
	d.Signup = s.Config.SignupAllowed && (d.Google || d.GitHub)
	s.render(w, r, status, "signin", pageData{Title: "Sign in", Sensitive: true, Body: d})
}

func (s *Server) signinPage(w http.ResponseWriter, r *http.Request) {
	binding := s.emailBinding(w, r)
	s.renderEmail(w, r, http.StatusOK, emailPage{CSRF: binding, Return: safeReturn(r.URL.Query().Get("return"))})
}

// emailForm checks double-submit CSRF against a host-only HttpOnly cookie.
// No authenticated session is required to initiate sign-in.
func (s *Server) emailForm(w http.ResponseWriter, r *http.Request) (string, bool) {
	// A no-referrer document makes Chromium serialize a form navigation's
	// Origin as null. Accept only browser-declared same-origin navigations in
	// that case; the independent cookie/form CSRF check below still applies.
	sameOriginNavigation := r.Header.Get("Origin") == "null" && r.Header.Get("Sec-Fetch-Site") == "same-origin" && r.Header.Get("Sec-Fetch-Mode") == "navigate"
	if !s.sameOrigin(r) && !sameOriginNavigation {
		s.renderEmail(w, r, 403, emailPage{State: "error", Message: "Please start sign-in again on this site."})
		return "", false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		s.renderEmail(w, r, 400, emailPage{State: "error", Message: "Please start sign-in again."})
		return "", false
	}
	c, err := r.Cookie(s.emailCookie())
	if err != nil || !validEmailSecret(c.Value) || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(c.Value)) != 1 {
		s.renderEmail(w, r, 403, emailPage{State: "error", Message: "Please start sign-in again in the browser where you requested it."})
		return "", false
	}
	return c.Value, true
}

// admitEmail spends limits before lookup and never holds its lock during I/O.
func (s *Server) admitEmail(r *http.Request, email string) (bool, bool) {
	l := s.emailLimiter
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	ip := "ip:" + s.clientAddress(r)
	recipient := "mail:" + string(store.Hash(email))
	if !l.available("global", 100, time.Hour, now) || !l.available(ip, 20, time.Hour, now) {
		return false, true
	}
	l.hits["global"] = append(l.hits["global"], now)
	l.hits[ip] = append(l.hits[ip], now)
	if !l.available(recipient, 3, time.Hour, now) {
		return false, false
	}
	l.hits[recipient] = append(l.hits[recipient], now)
	return true, false
}

func (s *Server) requestEmail(w http.ResponseWriter, r *http.Request) {
	binding, ok := s.emailForm(w, r)
	if !ok {
		return
	}
	if s.Config.EmailLogin == nil {
		s.renderEmail(w, r, 503, emailPage{State: "error", Message: "Email sign-in is temporarily unavailable. Please try again later."})
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		s.renderEmail(w, r, 400, emailPage{CSRF: binding, Return: safeReturn(r.PostForm.Get("return")), Message: "Enter a valid email address."})
		return
	}
	s.setCookie(w, s.emailCookie(), binding, int(store.EmailSigninTTL.Seconds()))
	allowed, limited := s.admitEmail(r, email)
	if limited {
		w.Header().Set("Retry-After", "3600")
		s.renderEmail(w, r, 429, emailPage{State: "error", Message: "Too many sign-in requests. Please try again later."})
		return
	}
	// A shared deadline also provides a consistent acknowledgement boundary
	// for unknown accounts and provider failures. Cancellation ends it early.
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if allowed {
		if err = s.issueEmail(ctx, r, email, binding); err != nil && !errors.Is(err, store.ErrNotFound) {
			log.Warn().Err(err).Msg("Email sign-in request unavailable")
		}
	}
	<-ctx.Done()
	s.renderEmail(w, r, 200, emailPage{State: "sent"})
}

func (s *Server) issueEmail(ctx context.Context, r *http.Request, email, binding string) error {
	provider := s.Config.EmailLogin
	u, err := s.Store.UserByVerifiedEmail(ctx, email)
	if err != nil {
		return err
	}
	if u.Role == "guest" || u.Issuer != provider.Issuer() {
		return store.ErrNotFound
	}
	if err = provider.CheckUser(ctx, u.Subject, u.Email); err != nil {
		return err
	}
	token := store.RandomHex(32)
	_, old, err := s.Store.ReplaceEmailSignin(ctx, token, store.Hash(binding), u, s.site(r).PortalOrigin(), safeReturn(r.PostForm.Get("return")))
	if err != nil {
		return err
	}
	if old != "" {
		_ = provider.DeleteSession(ctx, old)
	}
	template := s.site(r).PortalOrigin() + "/auth/email/verify?flow=" + token + "&code={{.Code}}"
	id, err := provider.SendLink(ctx, u.Subject, template)
	if err != nil {
		return err
	}
	attached, err := s.Store.AttachEmailSession(ctx, token, id)
	if err != nil || !attached {
		_ = provider.DeleteSession(ctx, id)
	}
	return err
}

func (s *Server) cleanupEmailSession(id string) {
	if id == "" || s.Config.EmailLogin == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Config.EmailLogin.DeleteSession(ctx, id); err != nil {
		log.Warn().Err(err).Msg("Temporary email sign-in session cleanup failed; it expires automatically")
	}
}

func (s *Server) inspectEmail(w http.ResponseWriter, r *http.Request) {
	flow, code := r.URL.Query().Get("flow"), r.URL.Query().Get("code")
	c, err := r.Cookie(s.emailCookie())
	if s.Config.EmailLogin == nil || err != nil || !validEmailSecret(flow) || len(flow) != 64 || !validEmailSecret(c.Value) || len(code) == 0 || len(code) > 256 {
		s.invalidEmail(w, r)
		return
	}
	ready, err := s.Store.EmailSigninReady(r.Context(), flow, store.Hash(c.Value), s.site(r).PortalOrigin())
	if err != nil || !ready {
		s.invalidEmail(w, r)
		return
	}
	s.renderEmail(w, r, 200, emailPage{State: "confirm", CSRF: c.Value, Flow: flow, Code: code})
}

func (s *Server) invalidEmail(w http.ResponseWriter, r *http.Request) {
	s.renderEmail(w, r, 400, emailPage{State: "error", Message: "This link is expired, already used, or open in a different browser. Request a new link in the browser where you want to sign in."})
}

func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	binding, ok := s.emailForm(w, r)
	if !ok {
		return
	}
	flow, code := r.PostForm.Get("flow"), r.PostForm.Get("code")
	if s.Config.EmailLogin == nil || !validEmailSecret(flow) || len(flow) != 64 || len(code) == 0 || len(code) > 256 {
		s.invalidEmail(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	f, err := s.Store.ClaimEmailSignin(ctx, flow, store.Hash(binding), s.site(r).PortalOrigin())
	if err != nil {
		s.invalidEmail(w, r)
		return
	}
	defer s.cleanupEmailSession(f.ProviderSession)
	if f.Issuer != s.Config.EmailLogin.Issuer() {
		s.invalidEmail(w, r)
		return
	}
	if err = s.Config.EmailLogin.Verify(ctx, f.ProviderSession, f.Subject, code, f.CreatedAt); err != nil {
		log.Warn().Err(err).Msg("Email sign-in proof rejected")
		s.invalidEmail(w, r)
		return
	}
	if err = s.Config.EmailLogin.CheckUser(ctx, f.Subject, f.Email); err != nil {
		s.invalidEmail(w, r)
		return
	}
	token, err := s.Store.CreateEmailSession(ctx, f)
	if err != nil {
		s.invalidEmail(w, r)
		return
	}
	s.setCookie(w, s.emailCookie(), "", -1)
	s.setCookie(w, s.sessionCookie(), token, int(store.SessionAbsolute.Seconds()))
	http.Redirect(w, r, safeReturn(f.ReturnPath), http.StatusSeeOther)
}

// signinURL preserves an intended local destination without leaking codes
// from authentication pages into navigation links.
func signinURL(ret string) string { return "/signin?return=" + url.QueryEscape(safeReturn(ret)) }
