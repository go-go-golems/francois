package catalog

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
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
		s.setCookie(w, s.emailCookie(), c.Value, int(store.EmailLinkTTL.Seconds()))
		return c.Value
	}
	binding := store.RandomHex(24)
	s.setCookie(w, s.emailCookie(), binding, int(store.EmailLinkTTL.Seconds()))
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
	d.Signup = s.Config.SignupAllowed && (d.Available || d.Google || d.GitHub)
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

// reserveEmail counts attempts globally/by client and reserves recipient capacity.
// A failed issuance refunds only its own recipient slot, not the abuse limits.
// A nil callback with allowed=true means recipient exhaustion: acknowledge generically.
func (s *Server) reserveEmail(r *http.Request, email string) (func(bool), bool) {
	l := s.emailLimiter
	l.mu.Lock()
	now := time.Now()
	ip := "ip:" + s.clientAddress(r)
	recipient := "mail:" + string(store.Hash(email))
	if !l.available("global", 100, time.Hour, now) || !l.available(ip, 20, time.Hour, now) {
		l.mu.Unlock()
		return nil, false
	}
	l.hits["global"] = append(l.hits["global"], now)
	l.hits[ip] = append(l.hits[ip], now)
	if !l.available(recipient, 10, time.Hour, now) {
		l.mu.Unlock()
		return nil, true
	}
	l.hits[recipient] = append(l.hits[recipient], now)
	l.mu.Unlock()
	return func(sent bool) {
		if sent {
			return
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		for i, t := range l.hits[recipient] {
			if t.Equal(now) {
				l.hits[recipient] = append(l.hits[recipient][:i], l.hits[recipient][i+1:]...)
				break
			}
		}
	}, true
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
	if err != nil || addr.Address != email || len(email) > 200 || strings.ContainsAny(email, "\r\n") {
		s.renderEmail(w, r, 400, emailPage{CSRF: binding, Return: safeReturn(r.PostForm.Get("return")), Message: "Enter a valid email address."})
		return
	}
	s.setCookie(w, s.emailCookie(), binding, int(store.EmailLinkTTL.Seconds()))
	finish, allowed := s.reserveEmail(r, email)
	if !allowed {
		w.Header().Set("Retry-After", "3600")
		s.renderEmail(w, r, 429, emailPage{State: "error", Message: "Too many sign-in requests. Please try again later."})
		return
	}
	if finish == nil {
		s.renderEmail(w, r, 200, emailPage{State: "sent"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	err = s.issueEmail(ctx, r, email, binding)
	finish(err == nil)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, identity.ErrEmailUnavailable) {
		log.Warn().Err(err).Msg("Email sign-in request unavailable")
		s.renderEmail(w, r, 503, emailPage{State: "error", Message: "We couldn’t send your link. Please try again."})
		return
	}
	s.renderEmail(w, r, 200, emailPage{State: "sent"})
}

func (s *Server) issueEmail(ctx context.Context, r *http.Request, email, binding string) error {
	provider := s.Config.EmailLogin
	users, err := s.Store.UsersByEmail(ctx, email)
	if err != nil {
		return err
	}
	if len(users) > 1 || len(users) == 1 && (users[0].Disabled || users[0].Role == "guest" || users[0].Issuer != provider.Issuer()) {
		return store.ErrNotFound
	}
	who, err := provider.ResolveEmail(ctx, email, store.NewID("email_"), s.Config.SignupAllowed)
	if err != nil {
		return err
	}
	id := store.Identity{Issuer: provider.Issuer(), Subject: who.Subject, Email: who.Email, Name: who.Name}
	token := store.RandomHex(32)
	if err = s.Store.PrepareEmailLink(ctx, token, store.Hash(binding), id, s.Config.SignupAllowed, s.site(r).PortalOrigin(), safeReturn(r.PostForm.Get("return"))); err != nil {
		return err
	}
	template := s.site(r).PortalOrigin() + "/auth/email/verify?flow=" + token + "&code={{.Code}}"
	if err = provider.SendEmailLink(ctx, who.Subject, template); err == nil {
		err = s.Store.SetEmailLinkReady(ctx, token)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = s.Store.DiscardEmailLink(cleanup, token)
	}
	return err
}

func (s *Server) inspectEmail(w http.ResponseWriter, r *http.Request) {
	flow, code := r.URL.Query().Get("flow"), r.URL.Query().Get("code")
	c, err := r.Cookie(s.emailCookie())
	if s.Config.EmailLogin == nil || err != nil || !validEmailSecret(flow) || len(flow) != 64 || !validEmailSecret(c.Value) || len(code) == 0 || len(code) > 256 {
		s.invalidEmail(w, r)
		return
	}
	ready, err := s.Store.EmailLinkReady(r.Context(), flow, store.Hash(c.Value), s.site(r).PortalOrigin())
	if err != nil || !ready {
		s.invalidEmail(w, r)
		return
	}
	s.renderEmail(w, r, 200, emailPage{State: "confirm", CSRF: c.Value, Flow: flow, Code: code})
}

func (s *Server) invalidEmail(w http.ResponseWriter, r *http.Request) {
	s.renderEmail(w, r, 400, emailPage{State: "error", Message: "This link is expired, already used, or open in another browser. Request a new link here and open it in this browser."})
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
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	f, err := s.Store.ClaimEmailLink(ctx, flow, store.Hash(binding), s.site(r).PortalOrigin())
	if err != nil || f.Issuer != s.Config.EmailLogin.Issuer() {
		s.invalidEmail(w, r)
		return
	}
	who, err := s.Config.EmailLogin.VerifyEmailLink(ctx, f.Subject, f.Email, code)
	if err != nil {
		log.Warn().Err(err).Msg("Email sign-in proof rejected")
		s.invalidEmail(w, r)
		return
	}
	token, err := s.Store.CompleteEmailLink(ctx, f, store.Identity{Issuer: f.Issuer, Subject: who.Subject, Email: who.Email, Name: who.Name, EmailVerified: true}, s.Config.SignupAllowed)
	if err != nil {
		s.invalidEmail(w, r)
		return
	}
	s.setCookie(w, s.emailCookie(), "", -1)
	s.setCookie(w, s.sessionCookie(), token, int(store.SessionAbsolute.Seconds()))
	http.Redirect(w, r, safeReturn(f.ReturnPath), http.StatusSeeOther)
}

// signinURL preserves a local destination without copying authentication codes.
func signinURL(ret string) string { return "/signin?return=" + url.QueryEscape(safeReturn(ret)) }
