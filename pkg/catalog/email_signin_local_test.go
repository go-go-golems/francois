package catalog

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// Explicit opt-in; never uses production credentials or external mail.
var localEmailFixture = flag.String("email-login-fixture", "", "Private fixture JSON from ticket 013's isolated local provisioning script")

func TestEmailSigninRealZitadel(t *testing.T) {
	if *localEmailFixture == "" {
		t.Skip("requires isolated local email-login-fixture")
	}
	raw, err := os.ReadFile(*localEmailFixture)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Organization, Subject string }
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	secret, err := os.ReadFile(filepath.Join(filepath.Dir(*localEmailFixture), "login.pat"))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := identity.NewEmailLogin("http://localhost:18083", fixture.Organization, strings.TrimSpace(string(secret)))
	if err != nil {
		t.Fatal(err)
	}
	e := setup(t, func(c *Config) { c.EmailLogin = provider })
	ctx := context.Background()
	user, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: provider.Issuer(), Subject: fixture.Subject, Email: "supporter@slop.test", EmailVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	status, _, headers := emailHTTP(e, "GET", "/signin?return=/account", "", nil)
	if status != 200 {
		t.Fatalf("signin=%d", status)
	}
	binding := ""
	res := http.Response{Header: headers}
	for _, c := range res.Cookies() {
		if c.Name == "francois_email_dev" {
			binding = c.Value
		}
	}
	status, body, _ := emailHTTP(e, "POST", "/auth/email/request", binding, url.Values{"email": {"supporter@slop.test"}, "csrf": {binding}, "return": {"/account"}})
	if status != 200 || !strings.Contains(string(body), "Check your email") {
		t.Fatalf("send=%d", status)
	}
	var link *url.URL
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && link == nil {
		var listing struct {
			Messages []struct {
				ID string `json:"ID"`
			} `json:"messages"`
		}
		mailGet(t, "http://localhost:18025/api/v1/messages", &listing)
		for _, m := range listing.Messages {
			var message struct {
				Text string
				To   []struct{ Address string }
			}
			mailGet(t, "http://localhost:18025/api/v1/message/"+url.PathEscape(m.ID), &message)
			if !strings.Contains(message.Text, "/auth/email/verify?") {
				continue
			}
			match := regexp.MustCompile(`http://portal\.test/auth/email/verify\?[^\s<>"\x60]+`).FindString(message.Text)
			if match != "" {
				candidate, err := url.Parse(match)
				if err == nil {
					ready, _ := e.store.EmailSigninReady(ctx, candidate.Query().Get("flow"), store.Hash(binding), "http://portal.test")
					if ready {
						link = candidate
						break
					}
				}
			}
		}
		if link == nil {
			time.Sleep(250 * time.Millisecond)
		}
	}
	if link == nil {
		t.Fatal("no delivered matching link in isolated mail sink; inspect private test logs")
	}
	// Actual email scanners may GET this link before the human does.
	if status, _, _ := emailHTTP(e, "GET", link.RequestURI(), binding, nil); status != 200 {
		t.Fatalf("delivered GET=%d", status)
	}
	var providerID string
	if err = e.store.Pool.QueryRow(ctx, `SELECT provider_session_id FROM email_signins WHERE user_id=$1`, user.ID).Scan(&providerID); err != nil {
		t.Fatal(err)
	}
	q := link.Query()
	status, _, headers = emailHTTP(e, "POST", "/auth/email/verify", binding, url.Values{"csrf": {binding}, "flow": {q.Get("flow")}, "code": {q.Get("code")}})
	if status != 303 || headers.Get("Location") != "/account" {
		t.Fatalf("real redemption=%d; check server diagnostics (not the code)", status)
	}
	token := ""
	res = http.Response{Header: headers}
	for _, c := range res.Cookies() {
		if c.Name == "francois_session_dev" {
			token = c.Value
		}
	}
	sess, actual, err := e.store.ResolveSession(ctx, token)
	if err != nil || actual.ID != user.ID || sess.IDToken != "" {
		t.Fatalf("same-user session failed: %v", err)
	}
	if err = provider.Verify(ctx, providerID, user.Subject, q.Get("code"), time.Now().Add(-time.Minute)); err == nil {
		t.Fatal("provider accepted a terminated session/code replay")
	}
	if status, _, _ := emailHTTP(e, "GET", link.RequestURI(), binding, nil); status != 400 {
		t.Fatal("portal link replay accepted")
	}
	// Empty-ID-token logout must stay local and revoke the actual portal session.
	actor := tactor{user: actual, token: token, csrf: sess.CSRF}
	if code, reply := e.portal(&actor, "POST", "/auth/logout", map[string]any{}); code != 200 || reply["redirect"] != nil {
		t.Fatalf("email logout: %d %v", code, reply)
	}
	if _, _, err = e.store.ResolveSession(ctx, token); err == nil {
		t.Fatal("logout did not revoke session")
	}
	t.Log("PASS: real ZITADEL v4.16.1 SMTP delivery, scanner-safe GET, password-free code verification, same-user portal session, terminated-provider replay rejection and local logout")
}

func mailGet(t *testing.T, endpoint string, out any) {
	t.Helper()
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("mail sink HTTP %d", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1024*1024)).Decode(out); err != nil {
		t.Fatal(err)
	}
}
