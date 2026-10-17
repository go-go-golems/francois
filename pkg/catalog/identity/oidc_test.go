package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// discovery serves a minimal OpenID Provider configuration. endSession may be
// empty to model an issuer without RP-initiated logout.
func discovery(t *testing.T, endSession bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		doc := map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/oauth/v2/authorize",
			"token_endpoint":                        srv.URL + "/oauth/v2/token",
			"jwks_uri":                              srv.URL + "/oauth/v2/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		if endSession {
			doc["end_session_endpoint"] = srv.URL + "/oidc/v1/end_session"
		}
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAuthURLIncludesExtraScopes(t *testing.T) {
	srv := discovery(t, true)
	p := New(Config{Issuer: srv.URL, ClientID: "client-1", RedirectURL: "https://portal.example/auth/callback",
		ExtraScopes: []string{"urn:zitadel:iam:org:id:123"}})
	target, err := p.AuthURL(context.Background(), "state", "nonce", strings.Repeat("v", 43), "", "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	scopes := strings.Fields(u.Query().Get("scope"))
	want := map[string]bool{"openid": true, "profile": true, "email": true, "urn:zitadel:iam:org:id:123": true}
	for _, s := range scopes {
		delete(want, s)
	}
	if len(want) != 0 {
		t.Fatalf("scope %q is missing %v", u.Query().Get("scope"), want)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE challenge missing: %s", target)
	}
}

func TestAuthURLSelectsIdentityProviderWithoutDroppingSecurityParameters(t *testing.T) {
	srv := discovery(t, true)
	p := New(Config{Issuer: srv.URL, ClientID: "client-1", RedirectURL: "https://portal.example/auth/callback", ExtraScopes: []string{"urn:zitadel:iam:org:id:123"}})
	for _, id := range []string{"google-id", "github-id"} {
		target, err := p.AuthURL(context.Background(), "state", "nonce", strings.Repeat("v", 43), "create", id)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(target)
		q := u.Query()
		for _, scope := range []string{"openid", "profile", "email", "urn:zitadel:iam:org:id:123", "urn:zitadel:iam:org:idp:id:" + id} {
			if !strings.Contains(" "+q.Get("scope")+" ", " "+scope+" ") {
				t.Fatalf("missing scope %s: %s", scope, target)
			}
		}
		if q.Get("state") != "state" || q.Get("nonce") != "nonce" || q.Get("prompt") != "create" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("redirect_uri") != "https://portal.example/auth/callback" {
			t.Fatalf("lost auth parameters: %s", target)
		}
	}
}

func TestLogoutURL(t *testing.T) {
	srv := discovery(t, true)
	p := New(Config{Issuer: srv.URL, ClientID: "client-1", RedirectURL: "https://portal.example/auth/callback"})
	target, err := p.LogoutURL(context.Background(), "id.token.value", "https://portal.example/", "")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/oidc/v1/end_session" {
		t.Fatalf("unexpected endpoint %s", target)
	}
	q := u.Query()
	if q.Get("id_token_hint") != "id.token.value" || q.Get("client_id") != "client-1" || q.Get("post_logout_redirect_uri") != "https://portal.example/" {
		t.Fatalf("unexpected logout parameters %v", q)
	}
	if q.Has("state") {
		t.Fatalf("empty state must be omitted: %v", q)
	}

	// Without a hint (sessions created before the column existed) the
	// client_id still identifies the registered post-logout redirect.
	target, err = p.LogoutURL(context.Background(), "", "https://portal.example/", "")
	if err != nil {
		t.Fatal(err)
	}
	if u, _ = url.Parse(target); u.Query().Has("id_token_hint") {
		t.Fatalf("empty hint must be omitted: %s", target)
	}
}

func TestLogoutURLWithoutEndSessionEndpoint(t *testing.T) {
	srv := discovery(t, false)
	p := New(Config{Issuer: srv.URL, ClientID: "client-1", RedirectURL: "https://portal.example/auth/callback"})
	target, err := p.LogoutURL(context.Background(), "id.token.value", "https://portal.example/", "")
	if err != nil || target != "" {
		t.Fatalf("want no redirect, got %q, %v", target, err)
	}
}

func TestValidateScopes(t *testing.T) {
	if err := ValidateScopes([]string{"urn:zitadel:iam:org:id:1", "offline_access"}); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{""}, {"a b"}, {"a\tb"}} {
		if err := ValidateScopes(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
