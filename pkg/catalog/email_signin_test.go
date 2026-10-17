package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"golang.org/x/sync/errgroup"
)

type emailUpstream struct {
	mu                      sync.Mutex
	user                    store.User
	link                    string
	issued, checks, deletes int
	verified                bool
	omitProof               bool
}

func (p *emailUpstream) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer test-login-service" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasPrefix(r.URL.Path, "/v2/users/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": p.user.Subject, "state": "USER_STATE_ACTIVE", "details": map[string]string{"resourceOwner": "org"}, "human": map[string]any{"email": map[string]any{"email": p.user.Email, "isVerified": true}}}})
	case r.Method == "POST":
		var in struct {
			Challenges struct {
				Email struct {
					Send struct {
						Template string `json:"urlTemplate"`
					} `json:"sendCode"`
				} `json:"otpEmail"`
			} `json:"challenges"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		p.link = strings.ReplaceAll(in.Challenges.Email.Send.Template, "{{.Code}}", "12345678")
		p.issued++
		p.verified = false
		_ = json.NewEncoder(w).Encode(map[string]string{"sessionId": fmt.Sprint(p.issued)})
	case r.Method == "PATCH":
		p.checks++
		var in struct {
			Checks struct {
				Email struct {
					Code string `json:"code"`
				} `json:"otpEmail"`
			} `json:"checks"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Checks.Email.Code != "12345678" {
			http.Error(w, "invalid", http.StatusBadRequest)
			return
		}
		p.verified = true
		_, _ = w.Write([]byte(`{}`))
	case r.Method == "DELETE":
		p.deletes++
		_, _ = w.Write([]byte(`{}`))
	default:
		factors := map[string]any{"user": map[string]string{"id": p.user.Subject, "organizationId": "org"}}
		if p.verified && !p.omitProof {
			factors["otpEmail"] = map[string]any{"verifiedAt": time.Now().UTC()}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"session": map[string]any{"id": strings.TrimPrefix(r.URL.Path, "/v2/sessions/"), "expirationDate": time.Now().Add(time.Minute), "factors": factors}})
	}
}
func setupEmail(t *testing.T) (*env, *emailUpstream, store.User) {
	t.Helper()
	up := &emailUpstream{}
	ts := httptest.NewServer(http.HandlerFunc(up.serve))
	t.Cleanup(ts.Close)
	provider, err := identity.NewEmailLogin(ts.URL, "org", "test-login-service")
	if err != nil {
		t.Fatal(err)
	}
	e := setup(t, func(c *Config) { c.EmailLogin = provider })
	u, err := e.store.UpsertIdentity(context.Background(), store.Identity{Issuer: ts.URL, Subject: "subscriber", Email: "alice@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	up.user = u
	return e, up, u
}
func emailHTTP(e *env, method, path, binding string, form url.Values) (int, []byte, http.Header) {
	e.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, e.ts.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Host = "portal.test"
	if binding != "" {
		req.AddCookie(&http.Cookie{Name: "francois_email_dev", Value: binding})
	}
	req.Header.Set("Origin", "http://portal.test")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := *e.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b, res.Header
}
func issueTestEmail(t *testing.T, e *env, up *emailUpstream, binding string) url.Values {
	t.Helper()
	req := httptest.NewRequest("POST", "http://portal.test/auth/email/request", strings.NewReader("email=alice%40example.test&return=%2Faccount"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()
	if err := e.srv.issueEmail(context.Background(), req, "alice@example.test", binding); err != nil {
		t.Fatal(err)
	}
	up.mu.Lock()
	link := up.link
	up.mu.Unlock()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func TestEmailSigninScannerReplayAndOwnership(t *testing.T) {
	e, up, u := setupEmail(t)
	code, body, h := emailHTTP(e, "GET", "/signin?return=/account", "", nil)
	if code != 200 || !strings.Contains(string(body), "No password needed") || h.Get("Cache-Control") != "no-store" || h.Get("Referrer-Policy") != "no-referrer" || strings.Contains(string(body), "<script") {
		t.Fatalf("sign-in response %d %s", code, body)
	}
	var binding string
	res := http.Response{Header: h}
	for _, cookie := range res.Cookies() {
		if cookie.Name == "francois_email_dev" {
			binding = cookie.Value
		}
	}
	if binding == "" {
		t.Fatal("no browser binding")
	}
	query := issueTestEmail(t, e, up, binding)
	link := "/auth/email/verify?" + query.Encode()
	for range 2 {
		if code, _, _ := emailHTTP(e, "GET", link, binding, nil); code != 200 {
			t.Fatalf("scanner GET %d", code)
		}
	}
	up.mu.Lock()
	checks := up.checks
	up.mu.Unlock()
	if checks != 0 {
		t.Fatal("GET verified email code")
	}
	if code, _, _ := emailHTTP(e, "GET", link, store.RandomHex(24), nil); code != 400 {
		t.Fatalf("other browser GET %d", code)
	}
	form := url.Values{"flow": {query.Get("flow")}, "code": {query.Get("code")}, "csrf": {binding}}
	wrong := url.Values{"flow": form["flow"], "code": form["code"], "csrf": {store.RandomHex(24)}}
	if code, _, _ := emailHTTP(e, "POST", "/auth/email/verify", binding, wrong); code != 403 {
		t.Fatalf("CSRF %d", code)
	}
	code, _, h = emailHTTP(e, "POST", "/auth/email/verify", binding, form)
	if code != 303 || h.Get("Location") != "/account" {
		t.Fatalf("redemption %d %v", code, h)
	}
	var token string
	res = http.Response{Header: h}
	for _, c := range res.Cookies() {
		if c.Name == "francois_session_dev" {
			token = c.Value
		}
	}
	sess, actual, err := e.store.ResolveSession(context.Background(), token)
	if err != nil || actual.ID != u.ID || sess.IDToken != "" {
		t.Fatalf("wrong identity/session %+v %v", actual, err)
	}
	if code, _, _ := emailHTTP(e, "POST", "/auth/email/verify", binding, form); code != 400 {
		t.Fatalf("replay %d", code)
	}
	up.mu.Lock()
	defer up.mu.Unlock()
	if up.checks != 1 || up.deletes != 1 {
		t.Fatalf("checks/deletes %d/%d", up.checks, up.deletes)
	}
}

func TestEmailSigninRejectsMissingProofAndChangedIdentity(t *testing.T) {
	e, up, u := setupEmail(t)
	for _, mode := range []string{"missing proof", "changed email", "promoted", "disabled", "incorrect code", "expired", "other portal"} {
		t.Run(mode, func(t *testing.T) {
			// Restore the initial account and provider state for each attempted login.
			_, err := e.store.Pool.Exec(context.Background(), `UPDATE users SET role='subscriber',disabled=false,email=$2 WHERE id=$1`, u.ID, u.Email)
			if err != nil {
				t.Fatal(err)
			}
			up.mu.Lock()
			up.user = u
			up.omitProof = false
			up.mu.Unlock()
			binding := store.RandomHex(24)
			q := issueTestEmail(t, e, up, binding)
			switch mode {
			case "missing proof":
				up.mu.Lock()
				up.omitProof = true
				up.mu.Unlock()
			case "changed email":
				up.mu.Lock()
				up.user.Email = "other@example.test"
				up.mu.Unlock()
			case "promoted":
				if err = e.store.SetUserRole(context.Background(), u.ID, "admin"); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if err = e.store.SetUserDisabled(context.Background(), u.ID, true); err != nil {
					t.Fatal(err)
				}
			case "incorrect code":
				q.Set("code", "bad")
			case "expired":
				_, _ = e.store.Pool.Exec(context.Background(), `UPDATE email_signins SET expires_at=now()-interval '1 second'`)
			case "other portal":
				_, _ = e.store.Pool.Exec(context.Background(), `UPDATE email_signins SET portal_origin='http://boutique.test'`)
			}
			form := url.Values{"flow": {q.Get("flow")}, "code": {q.Get("code")}, "csrf": {binding}}
			if code, _, _ := emailHTTP(e, "POST", "/auth/email/verify", binding, form); code != 400 {
				t.Fatalf("%s accepted: %d", mode, code)
			}
			n, err := e.store.ActiveSessions(context.Background(), u.ID)
			if err != nil || n != 0 {
				t.Fatalf("unauthenticated session created %d %v", n, err)
			}
		})
	}
}

func TestEmailFlowConcurrencyAndRoleRevocation(t *testing.T) {
	e, up, u := setupEmail(t)
	ctx := context.Background()
	binding := store.RandomHex(24)
	q := issueTestEmail(t, e, up, binding)
	var mu sync.Mutex
	wins := 0
	var claimed store.EmailSignin
	var group errgroup.Group
	for range 8 {
		group.Go(func() error {
			f, err := e.store.ClaimEmailSignin(ctx, q.Get("flow"), store.Hash(binding), "http://portal.test")
			if err == nil {
				mu.Lock()
				wins++
				claimed = f
				mu.Unlock()
			} else if !strings.Contains(err.Error(), "not found") {
				return err
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil || wins != 1 {
		t.Fatalf("claimants %d %v", wins, err)
	}
	token, err := e.store.CreateEmailSession(ctx, claimed)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.store.SetUserRole(ctx, u.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.store.ResolveSession(ctx, token); err == nil {
		t.Fatal("promoted email session remained valid")
	}
	if _, err = e.store.CreateEmailSession(ctx, claimed); err == nil {
		t.Fatal("session created after promotion")
	}
	// A send that finishes after replacement must not attach to the new flow.
	_, _ = e.store.Pool.Exec(ctx, `UPDATE users SET role='subscriber' WHERE id=$1`, u.ID)
	old, replacement := store.RandomHex(32), store.RandomHex(32)
	if _, _, err = e.store.ReplaceEmailSignin(ctx, old, store.Hash(binding), u, "http://portal.test", "/"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.store.ReplaceEmailSignin(ctx, replacement, store.Hash(binding), u, "http://portal.test", "/"); err != nil {
		t.Fatal(err)
	}
	if attached, err := e.store.AttachEmailSession(ctx, old, "late-session"); err != nil || attached {
		t.Fatalf("stale send attached %v %v", attached, err)
	}
}

func TestEmailSignupEligibilityAndLimits(t *testing.T) {
	e, up, u := setupEmail(t)
	ctx := context.Background()
	binding := store.RandomHex(24)
	for _, mode := range []string{"unknown", "staff", "ambiguous"} {
		email := "unknown@example.test"
		if mode == "staff" {
			email = u.Email
			_ = e.store.SetUserRole(ctx, u.ID, "creator")
		}
		if mode == "ambiguous" {
			email = u.Email
			_ = e.store.SetUserRole(ctx, u.ID, "subscriber")
			_, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: u.Issuer, Subject: "another", Email: u.Email, EmailVerified: true})
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest("POST", "http://portal.test/auth/email/request", nil)
		if err := e.srv.issueEmail(ctx, req, email, binding); err == nil {
			t.Fatalf("%s eligible", mode)
		}
	}
	up.mu.Lock()
	issued := up.issued
	up.mu.Unlock()
	if issued != 0 {
		t.Fatal("mail sent for ineligible account")
	}
	req := httptest.NewRequest("POST", "http://portal.test/auth/email/request", nil)
	for i := 0; i < 3; i++ {
		if ok, _ := e.srv.admitEmail(req, "alice@example.test"); !ok {
			t.Fatal("premature recipient limit")
		}
	}
	if ok, limited := e.srv.admitEmail(req, "alice@example.test"); ok || limited {
		t.Fatal("recipient exhaustion must remain generic")
	}
	for i := 4; i < 20; i++ {
		e.srv.admitEmail(req, fmt.Sprintf("%d@example.test", i))
	}
	if ok, limited := e.srv.admitEmail(req, "other@example.test"); ok || !limited {
		t.Fatal("IP limit missing")
	}
}

func TestEmailSessionPromotionRace(t *testing.T) {
	e, _, u := setupEmail(t)
	ctx := context.Background()
	f := store.EmailSignin{UserID: u.ID, Issuer: u.Issuer, Subject: u.Subject, Email: u.Email, ExpiresAt: time.Now().Add(time.Minute)}
	for range 16 {
		if err := e.store.SetUserRole(ctx, u.ID, "subscriber"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var token string
		var group errgroup.Group
		group.Go(func() error {
			<-start
			var err error
			token, err = e.store.CreateEmailSession(ctx, f)
			if err == store.ErrNotFound {
				return nil
			}
			return err
		})
		group.Go(func() error { <-start; return e.store.SetUserRole(ctx, u.ID, "admin") })
		close(start)
		if err := group.Wait(); err != nil {
			t.Fatal(err)
		}
		if token != "" {
			if _, _, err := e.store.ResolveSession(ctx, token); err == nil {
				t.Fatal("racing promotion left an email session active")
			}
		}
	}
}

func TestEmailFormNoReferrerNavigationOrigin(t *testing.T) {
	e, _, _ := setupEmail(t)
	binding := store.RandomHex(24)
	for _, tt := range []struct {
		name, origin, site, mode, csrf string
		status                         int
	}{
		{"no-referrer same origin", "null", "same-origin", "navigate", binding, 400},
		{"cross site null origin", "null", "cross-site", "navigate", binding, 403},
		{"same site is insufficient", "null", "same-site", "navigate", binding, 403},
		{"missing metadata", "null", "", "", binding, 403},
		{"cross site origin", "https://attacker.test", "cross-site", "navigate", binding, 403},
		{"null fetch is not navigation", "null", "same-origin", "cors", binding, 403},
		{"still requires CSRF", "null", "same-origin", "navigate", "wrong", 403},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://portal.test/auth/email/request", strings.NewReader(url.Values{"csrf": {tt.csrf}, "email": {"invalid"}}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Sec-Fetch-Site", tt.site)
			req.Header.Set("Sec-Fetch-Mode", tt.mode)
			req.AddCookie(&http.Cookie{Name: "francois_email_dev", Value: binding})
			w := httptest.NewRecorder()
			e.srv.Handler().ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("form status=%d, want %d", w.Code, tt.status)
			}
		})
	}
}
