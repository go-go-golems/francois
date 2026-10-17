package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"golang.org/x/sync/errgroup"
)

type signupUpstream struct {
	emailUpstream
	exists, emailVerified, enrolled, failEnrollment, collision bool
	creates, resends, enrollments                              int
	organization                                               string
}

func (p *signupUpstream) serveSignup(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v2/sessions") {
		p.serve(w, r)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer test-login-service" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var in map[string]json.RawMessage
	if r.Method == "POST" {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	send := func(raw json.RawMessage) {
		var v struct {
			URL string `json:"urlTemplate"`
		}
		_ = json.Unmarshal(raw, &v)
		p.link = strings.ReplaceAll(v.URL, "{{.Code}}", "registration-code")
	}
	switch {
	case r.URL.Path == "/v2/users":
		var query struct {
			Limit int `json:"limit"`
		}
		_ = json.Unmarshal(in["query"], &query)
		var filters []map[string]map[string]string
		_ = json.Unmarshal(in["queries"], &filters)
		if query.Limit != 1 || len(filters) != 2 || filters[0]["organizationIdQuery"]["organizationId"] != "org" || filters[1]["emailQuery"]["method"] != "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE" {
			http.Error(w, "invalid scope", http.StatusBadRequest)
			return
		}
		if p.collision {
			_, _ = w.Write([]byte(`{"result":[{"userId":"unrelated"}]}`))
		} else {
			_, _ = w.Write([]byte(`{"result":[]}`))
		}
	case r.URL.Path == "/v2/users/human":
		if p.exists {
			http.Error(w, "conflict", http.StatusConflict)
			return
		}
		var email struct {
			Address  string          `json:"email"`
			Verified bool            `json:"isVerified"`
			Send     json.RawMessage `json:"sendCode"`
		}
		_ = json.Unmarshal(in["email"], &email)
		var org struct {
			ID string `json:"orgId"`
		}
		_ = json.Unmarshal(in["organization"], &org)
		if email.Verified || org.ID != "org" || in["password"] != nil || len(email.Send) == 0 {
			http.Error(w, "unsafe creation", http.StatusBadRequest)
			return
		}
		_ = json.Unmarshal(in["userId"], &p.user.Subject)
		p.user.Email = email.Address
		p.exists = true
		p.creates++
		send(email.Send)
		_ = json.NewEncoder(w).Encode(map[string]string{"userId": p.user.Subject})
	case strings.HasSuffix(r.URL.Path, "/email/resend"):
		p.resends++
		send(in["sendCode"])
		_, _ = w.Write([]byte(`{}`))
	case strings.HasSuffix(r.URL.Path, "/email/verify"):
		var code string
		_ = json.Unmarshal(in["verificationCode"], &code)
		p.checks++
		if code != "registration-code" {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		p.emailVerified = true
		_, _ = w.Write([]byte(`{}`))
	case strings.HasSuffix(r.URL.Path, "/authentication_methods"):
		methods := []string{}
		if p.enrolled {
			methods = append(methods, "AUTHENTICATION_METHOD_TYPE_OTP_EMAIL")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"authMethodTypes": methods})
	case strings.HasSuffix(r.URL.Path, "/otp_email"):
		if !p.emailVerified || p.failEnrollment {
			http.Error(w, "not ready", http.StatusPreconditionFailed)
			return
		}
		p.enrollments++
		p.enrolled = true
		_, _ = w.Write([]byte(`{}`))
	default:
		if !p.exists {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"userId": p.user.Subject, "state": "USER_STATE_ACTIVE", "details": map[string]string{"resourceOwner": p.organization}, "human": map[string]any{"email": map[string]any{"email": p.user.Email, "isVerified": p.emailVerified}}}})
	}
}

func setupSignup(t *testing.T) (*env, *signupUpstream) {
	t.Helper()
	up := &signupUpstream{organization: "org"}
	ts := httptest.NewServer(http.HandlerFunc(up.serveSignup))
	t.Cleanup(ts.Close)
	provider, err := identity.NewEmailLogin(ts.URL, "org", "test-login-service")
	if err != nil {
		t.Fatal(err)
	}
	return setup(t, func(c *Config) { c.EmailLogin = provider; c.SignupAllowed = true }), up
}

func issueSignup(t *testing.T, e *env, up *signupUpstream, binding string) url.Values {
	t.Helper()
	r := httptest.NewRequest("POST", "http://portal.test/auth/email/request", strings.NewReader("return=%2Faccount"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()
	if err := e.srv.issueEmail(context.Background(), r, "new@example.test", binding); err != nil {
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
func confirmSignup(e *env, binding string, q url.Values) (int, http.Header) {
	form := url.Values{"flow": {q.Get("flow")}, "code": {q.Get("code")}, "csrf": {binding}}
	status, _, h := emailHTTP(e, "POST", "/auth/email/verify", binding, form)
	return status, h
}
func assertNoSignupUser(t *testing.T, e *env) {
	t.Helper()
	rows, err := e.store.UsersByEmail(context.Background(), "new@example.test")
	if err != nil || len(rows) != 0 {
		t.Fatalf("unexpected local users: %d %v", len(rows), err)
	}
}

func TestEmailSignupFirstProofAndSubsequentLogin(t *testing.T) {
	e, up := setupSignup(t)
	binding := store.RandomHex(24)
	q := issueSignup(t, e, up, binding)
	assertNoSignupUser(t, e)
	for range 2 {
		if code, _, _ := emailHTTP(e, "GET", "/auth/email/verify?"+q.Encode(), binding, nil); code != 200 {
			t.Fatalf("scanner GET %d", code)
		}
	}
	if up.checks != 0 || up.enrollments != 0 {
		t.Fatal("GET caused verification or enrollment")
	}
	if status, _ := confirmSignup(e, store.RandomHex(24), q); status != 400 {
		t.Fatalf("wrong browser %d", status)
	}
	status, h := confirmSignup(e, binding, q)
	if status != 303 || h.Get("Location") != "/account" {
		t.Fatalf("confirmation %d", status)
	}
	var token string
	res := http.Response{Header: h}
	for _, c := range res.Cookies() {
		if c.Name == e.srv.sessionCookie() {
			token = c.Value
		}
	}
	_, u, err := e.store.ResolveSession(context.Background(), token)
	if err != nil || u.Role != "subscriber" || !u.EmailVerified || u.Disabled {
		t.Fatalf("signup account %+v %v", u, err)
	}
	if !up.enrolled || up.enrollments != 1 || up.creates != 1 {
		t.Fatal("registration/factor enrollment missing")
	}
	if status, _ = confirmSignup(e, binding, q); status != 400 {
		t.Fatalf("replay %d", status)
	}
	q = issueSignup(t, e, up, binding)
	if status, _ = confirmSignup(e, binding, q); status != 303 {
		t.Fatalf("subsequent OTP login %d", status)
	}
	if up.creates != 1 || up.enrollments != 1 || up.issued != 1 {
		t.Fatal("repeat login recreated user or factor")
	}
}

func TestEmailSignupReplacementBadCodeAndRecovery(t *testing.T) {
	e, up := setupSignup(t)
	binding := store.RandomHex(24)
	old := issueSignup(t, e, up, binding)
	q := issueSignup(t, e, up, binding)
	if status, _ := confirmSignup(e, binding, old); status != 400 {
		t.Fatalf("replaced link %d", status)
	}
	q.Set("code", "wrong")
	if status, _ := confirmSignup(e, binding, q); status != 400 {
		t.Fatalf("bad code %d", status)
	}
	assertNoSignupUser(t, e)
	q = issueSignup(t, e, up, binding)
	up.failEnrollment = true
	if status, _ := confirmSignup(e, binding, q); status != 400 {
		t.Fatalf("failed enrollment %d", status)
	}
	assertNoSignupUser(t, e)
	up.failEnrollment = false
	q = issueSignup(t, e, up, binding) // Provider verified; resume through fresh session OTP.
	if status, _ := confirmSignup(e, binding, q); status != 303 {
		t.Fatalf("recovered confirmation %d", status)
	}
	if up.creates != 1 || up.resends != 2 || up.issued != 1 {
		t.Fatalf("unexpected retry calls: creates %d resends %d sessions %d", up.creates, up.resends, up.issued)
	}
}

func TestEmailSignupRejectsDisabledAmbiguousAndProviderCollisions(t *testing.T) {
	for _, mode := range []string{"signup disabled", "disabled local", "unverified local", "ambiguous local", "provider collision"} {
		t.Run(mode, func(t *testing.T) {
			e, up := setupSignup(t)
			ctx := context.Background()
			if mode == "signup disabled" {
				e.srv.Config.SignupAllowed = false
			}
			if mode == "provider collision" {
				up.collision = true
			}
			if strings.Contains(mode, "local") {
				u, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "existing", Email: "new@example.test", EmailVerified: mode != "unverified local"})
				if err != nil {
					t.Fatal(err)
				}
				if mode == "disabled local" {
					_, err = e.store.Pool.Exec(ctx, `UPDATE users SET disabled=true WHERE id=$1`, u.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
				if mode == "ambiguous local" {
					_, err = e.store.Pool.Exec(ctx, `INSERT INTO users(id,issuer,subject,email,email_verified) VALUES($1,$2,'another',$3,true)`, store.NewID("usr_"), u.Issuer, u.Email)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			r := httptest.NewRequest("POST", "http://portal.test/auth/email/request", nil)
			if err := e.srv.issueEmail(ctx, r, "new@example.test", store.RandomHex(24)); err == nil {
				t.Fatal("blocked address accepted")
			}
			if up.creates != 0 || up.enrollments != 0 || up.issued != 0 {
				t.Fatal("blocked request mutated provider")
			}
		})
	}
}

func TestEmailSignupConfirmationRechecksState(t *testing.T) {
	for _, mode := range []string{"disabled signup", "changed email", "wrong organization", "expired", "local collision"} {
		t.Run(mode, func(t *testing.T) {
			e, up := setupSignup(t)
			binding := store.RandomHex(24)
			q := issueSignup(t, e, up, binding)
			ctx := context.Background()
			switch mode {
			case "disabled signup":
				e.srv.Config.SignupAllowed = false
			case "changed email":
				up.user.Email = "changed@example.test"
			case "wrong organization":
				up.organization = "other"
			case "expired":
				_, err := e.store.Pool.Exec(ctx, `UPDATE email_registrations SET expires_at=$1`, time.Now().Add(-time.Minute))
				if err != nil {
					t.Fatal(err)
				}
			case "local collision":
				_, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: up.user.Issuer, Subject: "other", Email: "new@example.test", EmailVerified: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			if status, _ := confirmSignup(e, binding, q); status != 400 {
				t.Fatalf("unsafe completion %d", status)
			}
			var sessions int
			if err := e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&sessions); err != nil || sessions != 0 {
				t.Fatalf("unexpected sessions %d %v", sessions, err)
			}
		})
	}
}

func TestEmailRegistrationConcurrentClaimAndCompletion(t *testing.T) {
	e, up := setupSignup(t)
	binding := store.RandomHex(24)
	q := issueSignup(t, e, up, binding)
	ctx := context.Background()
	const requests = 8
	results := make(chan error, requests)
	var group errgroup.Group
	for range requests {
		group.Go(func() error {
			_, err := e.store.ClaimEmailRegistration(ctx, q.Get("flow"), store.Hash(binding), "http://portal.test")
			results <- err
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	won := 0
	for range requests {
		if <-results == nil {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("claim winners %d", won)
	}
	f := store.EmailSignin{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: up.user.Subject, Email: "new@example.test", ExpiresAt: time.Now().Add(time.Minute)}
	for range requests {
		group.Go(func() error { _, err := e.store.CompleteEmailRegistration(ctx, f); results <- err; return nil })
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	won = 0
	for range requests {
		if <-results == nil {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("completion winners %d", won)
	}
}

func TestEmailSignupAndOIDCRejectTheSecondIdentityInEitherOrder(t *testing.T) {
	for _, signupFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "signup first", false: "OIDC first"}[signupFirst], func(t *testing.T) {
			e, _ := setupSignup(t)
			ctx := context.Background()
			f := store.EmailSignin{Issuer: e.srv.Config.EmailLogin.Issuer(), Subject: "email-signup", Email: "new@example.test", ExpiresAt: time.Now().Add(time.Minute)}
			id := store.Identity{Issuer: f.Issuer, Subject: "social-login", Email: "NEW@example.test", EmailVerified: true}
			signup := func() error { _, err := e.store.CompleteEmailRegistration(ctx, f); return err }
			oidc := func() error { _, err := e.store.UpsertIdentity(ctx, id); return err }
			first, second := signup, oidc
			if !signupFirst {
				first, second = oidc, signup
			}
			if err := first(); err != nil {
				t.Fatal(err)
			}
			if err := second(); err != store.ErrNotFound {
				t.Fatalf("second identity was not rejected: %v", err)
			}
			users, err := e.store.UsersByEmail(ctx, f.Email)
			if err != nil || len(users) != 1 {
				t.Fatalf("local users %d: %v", len(users), err)
			}
			// The winning identity can refresh its profile without losing its role.
			winner := users[0]
			if err = e.store.SetUserRole(ctx, winner.ID, "creator"); err != nil {
				t.Fatal(err)
			}
			refreshed, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: winner.Issuer, Subject: winner.Subject, Email: winner.Email, EmailVerified: true, Name: "Updated"})
			if err != nil || refreshed.Role != "creator" || refreshed.DisplayName != "Updated" {
				t.Fatalf("same-identity refresh %+v %v", refreshed, err)
			}
		})
	}
}

func TestOIDCRefreshCannotTakeAnotherAccountsEmail(t *testing.T) {
	e, _ := setupSignup(t)
	ctx := context.Background()
	owner, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: "issuer", Subject: "owner", Email: "owned@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: "issuer", Subject: "other", Email: "other@example.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.store.UpsertIdentity(ctx, store.Identity{Issuer: other.Issuer, Subject: other.Subject, Email: "OWNED@example.test", EmailVerified: true})
	if err != store.ErrNotFound {
		t.Fatalf("colliding refresh %v", err)
	}
	unchanged, err := e.store.User(ctx, other.ID)
	if err != nil || unchanged.Email != other.Email {
		t.Fatalf("profile changed despite collision: %+v %v", unchanged, err)
	}
	users, err := e.store.UsersByEmail(ctx, owner.Email)
	if err != nil || len(users) != 1 {
		t.Fatalf("email owners %d %v", len(users), err)
	}
}
