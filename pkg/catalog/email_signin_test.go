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

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"golang.org/x/sync/errgroup"
)

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

type mailboxUser struct {
	Subject, Email, Name, State, Org, Code string
	Verified                               bool
}
type mailboxProvider struct {
	mu                            sync.Mutex
	users                         map[string]*mailboxUser
	links                         []string
	creates, sends, verifications int
	failSend                      bool
}

func (p *mailboxProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer test-login-service" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	emit := func(u *mailboxUser) map[string]any {
		return map[string]any{"userId": u.Subject, "state": u.State, "details": map[string]string{"resourceOwner": u.Org}, "human": map[string]any{"email": map[string]any{"email": u.Email, "isVerified": u.Verified}, "profile": map[string]string{"displayName": u.Name}}}
	}
	switch r.URL.Path {
	case "/v2/users":
		var in struct {
			Query struct {
				Limit int `json:"limit"`
			} `json:"query"`
			Queries []map[string]map[string]string `json:"queries"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Query.Limit != 2 || len(in.Queries) != 2 || in.Queries[0]["organizationIdQuery"]["organizationId"] != "org" || in.Queries[1]["emailQuery"]["method"] != "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE" {
			http.Error(w, "bad search", http.StatusBadRequest)
			return
		}
		result := []any{}
		email := in.Queries[1]["emailQuery"]["emailAddress"]
		for _, u := range p.users {
			if strings.EqualFold(email, u.Email) && u.Org == "org" {
				result = append(result, emit(u))
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
		return
	case "/v2/users/human":
		var in struct {
			Subject string `json:"userId"`
			Email   struct {
				Address  string          `json:"email"`
				Return   json.RawMessage `json:"returnCode"`
				Send     json.RawMessage `json:"sendCode"`
				Verified bool            `json:"isVerified"`
			} `json:"email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Email.Verified || len(in.Email.Return) == 0 || len(in.Email.Send) != 0 {
			http.Error(w, "unexpected creation email", http.StatusBadRequest)
			return
		}
		for _, u := range p.users {
			if u.Email == in.Email.Address {
				http.Error(w, "conflict", http.StatusConflict)
				return
			}
		}
		p.users[in.Subject] = &mailboxUser{Subject: in.Subject, Email: in.Email.Address, Name: "New user", State: "USER_STATE_ACTIVE", Org: "org"}
		p.creates++
		_ = json.NewEncoder(w).Encode(map[string]string{"userId": in.Subject})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v2/users/"), "/")
	u := p.users[parts[0]]
	if u == nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		_ = json.NewEncoder(w).Encode(map[string]any{"user": emit(u)})
	case strings.HasSuffix(r.URL.Path, "/email/send"):
		if p.failSend {
			http.Error(w, "mail unavailable", http.StatusServiceUnavailable)
			return
		}
		var in struct {
			Send struct {
				URL string `json:"urlTemplate"`
			} `json:"sendCode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		p.sends++
		u.Code = fmt.Sprintf("code-%d", p.sends)
		p.links = append(p.links, strings.ReplaceAll(in.Send.URL, "{{.Code}}", u.Code))
		_, _ = w.Write([]byte(`{}`))
	case strings.HasSuffix(r.URL.Path, "/email/verify"):
		var in struct {
			Code string `json:"verificationCode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Code == "" || in.Code != u.Code {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		p.verifications++
		u.Verified = true
		u.Code = ""
		_, _ = w.Write([]byte(`{}`))
	default:
		http.Error(w, "unexpected provider endpoint", http.StatusNotFound)
	}
}
func setupMailbox(t *testing.T) (*env, *mailboxProvider) {
	t.Helper()
	up := &mailboxProvider{users: map[string]*mailboxUser{}}
	ts := httptest.NewServer(http.HandlerFunc(up.serve))
	t.Cleanup(ts.Close)
	client, err := identity.NewEmailLogin(ts.URL, "org", "test-login-service")
	if err != nil {
		t.Fatal(err)
	}
	return setup(t, func(c *Config) { c.EmailLogin = client; c.SignupAllowed = true }), up
}
func existingMailbox(p *mailboxProvider, verified bool) {
	p.users["existing"] = &mailboxUser{Subject: "existing", Email: "new@example.test", Name: "Actual Name", State: "USER_STATE_ACTIVE", Org: "org", Verified: verified}
}
func issueMailbox(t *testing.T, e *env, p *mailboxProvider, binding string) url.Values {
	t.Helper()
	r := httptest.NewRequest("POST", "http://portal.test/auth/email/request", strings.NewReader("return=%2Faccount"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	if err := e.srv.issueEmail(context.Background(), r, "new@example.test", binding); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.links[len(p.links)-1])
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
func continueMailbox(e *env, binding string, q url.Values) (int, http.Header) {
	code, _, h := emailHTTP(e, "POST", "/auth/email/verify", binding, url.Values{"csrf": {binding}, "flow": {q.Get("flow")}, "code": {q.Get("code")}})
	return code, h
}
func sessionMailbox(t *testing.T, e *env, h http.Header) store.User {
	t.Helper()
	res := http.Response{Header: h}
	for _, c := range res.Cookies() {
		if c.Name == e.srv.sessionCookie() {
			_, u, err := e.store.ResolveSession(context.Background(), c.Value)
			if err != nil {
				t.Fatal(err)
			}
			return u
		}
	}
	t.Fatal("no session cookie")
	return store.User{}
}

func TestUnifiedEmailOnboarding(t *testing.T) {
	for _, mode := range []string{"new address", "provider verified", "provider unverified", "local unverified", "local stale email", "creator", "admin"} {
		t.Run(mode, func(t *testing.T) {
			e, p := setupMailbox(t)
			ctx := context.Background()
			binding := store.RandomHex(24)
			if mode != "new address" {
				existingMailbox(p, mode != "provider unverified")
			}
			var before store.User
			if strings.HasPrefix(mode, "local") || mode == "creator" || mode == "admin" {
				address := "new@example.test"
				if mode == "local stale email" {
					address = "old@example.test"
				}
				var err error
				before, err = e.store.UpsertIdentity(ctx, store.Identity{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "existing", Email: address, EmailVerified: mode != "local unverified"})
				if err != nil {
					t.Fatal(err)
				}
				if mode == "creator" || mode == "admin" {
					if err = e.store.SetUserRole(ctx, before.ID, mode); err != nil {
						t.Fatal(err)
					}
				}
				e.srv.Config.SignupAllowed = false // Existing identities do not require signup.
			}
			q := issueMailbox(t, e, p, binding)
			var count int
			if err := e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role<>'guest'`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if before.ID == "" && count != 0 {
				t.Fatal("local account created before proof")
			}
			for range 2 {
				code, _, h := emailHTTP(e, "GET", "/auth/email/verify?"+q.Encode(), binding, nil)
				if code != 200 || h.Get("Cache-Control") != "no-store" || h.Get("Referrer-Policy") != "no-referrer" {
					t.Fatalf("confirmation GET %d", code)
				}
			}
			if p.verifications != 0 {
				t.Fatal("GET consumed proof")
			}
			if code, _ := continueMailbox(e, store.RandomHex(24), q); code != 400 {
				t.Fatalf("other browser %d", code)
			}
			status, h := continueMailbox(e, binding, q)
			if status != 303 || h.Get("Location") != "/account" {
				t.Fatalf("confirmation %d", status)
			}
			u := sessionMailbox(t, e, h)
			if !u.EmailVerified || u.Email != "new@example.test" {
				t.Fatalf("unreconciled profile %+v", u)
			}
			if before.ID != "" && before.ID != u.ID {
				t.Fatal("recreated local identity")
			}
			expectedRole := "subscriber"
			if mode == "creator" || mode == "admin" {
				expectedRole = mode
			}
			if u.Role != expectedRole {
				t.Fatalf("role %s", u.Role)
			}
			if status, _ = continueMailbox(e, binding, q); status != 400 {
				t.Fatalf("replay %d", status)
			}
			e.srv.Config.SignupAllowed = false
			q = issueMailbox(t, e, p, binding)
			if status, _ = continueMailbox(e, binding, q); status != 303 {
				t.Fatalf("returning login %d", status)
			}
			if p.creates > 1 || len(p.users) != 1 {
				t.Fatal("duplicate provider identity")
			}
		})
	}
}

func TestEmailLinkReconcilesConcurrentSSO(t *testing.T) {
	e, p := setupMailbox(t)
	existingMailbox(p, true)
	binding := store.RandomHex(24)
	q := issueMailbox(t, e, p, binding)
	u, err := e.store.UpsertIdentity(context.Background(), store.Identity{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "existing", Email: "new@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	e.srv.Config.SignupAllowed = false
	status, h := continueMailbox(e, binding, q)
	if status != 303 {
		t.Fatalf("SSO reconciliation %d", status)
	}
	if sessionMailbox(t, e, h).ID != u.ID {
		t.Fatal("did not reuse SSO identity")
	}
}

func TestEmailLinkGuards(t *testing.T) {
	for _, mode := range []string{"disabled", "role changed", "wrong organization", "changed email", "inactive", "expired", "wrong origin", "wrong code", "signup disabled", "different local identity"} {
		t.Run(mode, func(t *testing.T) {
			e, p := setupMailbox(t)
			existingMailbox(p, true)
			binding := store.RandomHex(24)
			ctx := context.Background()
			var u store.User
			var err error
			if mode == "disabled" || mode == "role changed" {
				u, err = e.store.UpsertIdentity(ctx, store.Identity{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "existing", Email: "new@example.test"})
				if err != nil {
					t.Fatal(err)
				}
			}
			q := issueMailbox(t, e, p, binding)
			switch mode {
			case "disabled":
				err = e.store.SetUserDisabled(ctx, u.ID, true)
			case "role changed":
				err = e.store.SetUserRole(ctx, u.ID, "admin")
			case "wrong organization":
				p.users["existing"].Org = "other"
			case "changed email":
				p.users["existing"].Email = "changed@example.test"
			case "inactive":
				p.users["existing"].State = "USER_STATE_INACTIVE"
			case "expired":
				_, err = e.store.Pool.Exec(ctx, `UPDATE email_links SET expires_at=now()-interval '1 minute'`)
			case "wrong origin":
				_, err = e.store.Pool.Exec(ctx, `UPDATE email_links SET portal_origin='http://boutique.test'`)
			case "wrong code":
				q.Set("code", "invalid")
			case "signup disabled":
				e.srv.Config.SignupAllowed = false
			case "different local identity":
				_, err = e.store.UpsertIdentity(ctx, store.Identity{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "other", Email: "new@example.test", EmailVerified: true})
			}
			if err != nil {
				t.Fatal(err)
			}
			if code, _ := continueMailbox(e, binding, q); code != 400 {
				t.Fatalf("guard %s: %d", mode, code)
			}
			var sessions int
			if err = e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 0 {
				t.Fatalf("sessions %d %v", sessions, err)
			}
		})
	}
}

func TestEmailDeliveryFailureAndQuotaRefund(t *testing.T) {
	e, p := setupMailbox(t)
	existingMailbox(p, true)
	binding := store.RandomHex(24)
	q := issueMailbox(t, e, p, binding)
	p.failSend = true
	form := url.Values{"email": {"new@example.test"}, "csrf": {binding}}
	code, body, _ := emailHTTP(e, "POST", "/auth/email/request", binding, form)
	if code != 503 || !strings.Contains(string(body), "try again") {
		t.Fatalf("send failure %d", code)
	}
	// The failed candidate did not remove the earlier usable link.
	ready, err := e.store.EmailLinkReady(context.Background(), q.Get("flow"), store.Hash(binding), "http://portal.test")
	if err != nil || !ready {
		t.Fatal("prior link destroyed")
	}
	req := httptest.NewRequest("POST", "http://portal.test/auth/email/request", nil)
	for range 10 {
		finish := e.srv.reserveEmail(req, "new@example.test")
		if finish == nil {
			t.Fatal("failure consumed send budget")
		}
		finish(true)
	}
	if e.srv.reserveEmail(req, "new@example.test") != nil {
		t.Fatal("recipient limit not enforced")
	}
	p.failSend = false
	if status, _ := continueMailbox(e, binding, q); status != 303 {
		t.Fatalf("previous link %d", status)
	}
}

func TestEmailLinkCSRFAndConcurrentClaim(t *testing.T) {
	e, p := setupMailbox(t)
	existingMailbox(p, true)
	binding := store.RandomHex(24)
	code, _, _ := emailHTTP(e, "POST", "/auth/email/request", binding, url.Values{"email": {"new@example.test"}, "csrf": {store.RandomHex(24)}})
	if code != 403 || p.sends != 0 {
		t.Fatal("CSRF accepted")
	}
	q := issueMailbox(t, e, p, binding)
	results := make(chan error, 8)
	var group errgroup.Group
	for range 8 {
		group.Go(func() error {
			_, err := e.store.ClaimEmailLink(context.Background(), q.Get("flow"), store.Hash(binding), "http://portal.test")
			results <- err
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	wins := 0
	for range 8 {
		if <-results == nil {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("claim winners %d", wins)
	}
}

func TestEmailProviderAmbiguityDoesNotPickAnIdentity(t *testing.T) {
	e, p := setupMailbox(t)
	existingMailbox(p, true)
	p.users["other"] = &mailboxUser{Subject: "other", Email: "new@example.test", Org: "org", State: "USER_STATE_ACTIVE", Verified: true}
	err := e.srv.issueEmail(context.Background(), httptest.NewRequest("POST", "http://portal.test/auth/email/request", nil), "new@example.test", store.RandomHex(24))
	if err == nil || p.sends != 0 {
		t.Fatal("ambiguous provider identities accepted")
	}
}
