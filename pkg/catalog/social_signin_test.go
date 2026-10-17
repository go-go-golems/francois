package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestSharedSocialSignin(t *testing.T) {
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "jwks_uri": issuer.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(issuer.Close)
	e := setup(t, func(c *Config) {
		c.GoogleIDP = "google-id"
		c.GitHubIDP = "github-id"
		c.SignupAllowed = true
		for i := range c.Sites {
			c.Sites[i].OIDC = identity.New(identity.Config{Issuer: issuer.URL, ClientID: "client", RedirectURL: c.Sites[i].PortalOrigin() + "/auth/callback", ExtraScopes: []string{"urn:zitadel:iam:org:id:org"}})
		}
	})
	_, body, _ := emailHTTP(e, "GET", "/signin?return=%2Fstudio%3Ftab%3Dposts%26sort%3Dnew", "", nil)
	page := string(body)
	for _, want := range []string{"Continue with Google", "Continue with GitHub", "to create an account"} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(page, "administrator") || strings.Contains(page, "signin-staff") {
		t.Fatal("role-specific sign-in remains")
	}
	links := regexp.MustCompile(`href="(/auth/login[^" ]*)"`).FindAllSubmatch(body, -1)
	if len(links) != 2 {
		t.Fatalf("expected two SSO links, got %d", len(links))
	}
	for _, link := range links {
		target, err := url.Parse(html.UnescapeString(string(link[1])))
		if err != nil {
			t.Fatal(err)
		}
		if target.Query().Get("return") != "/studio?tab=posts&sort=new" {
			t.Fatalf("lost destination: %s", link[1])
		}
	}
	for _, provider := range []string{"google", "github"} {
		status, _, headers := emailHTTP(e, "GET", "/auth/login?provider="+provider+"&return=%2Fstudio%3Ftab%3Dposts&idp=untrusted", "", nil)
		if status != 303 {
			t.Fatalf("%s status %d", provider, status)
		}
		target, err := url.Parse(headers.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		q := target.Query()
		scopes := " " + q.Get("scope") + " "
		if target.Host != strings.TrimPrefix(issuer.URL, "http://") || !strings.Contains(scopes, " urn:zitadel:iam:org:idp:id:"+provider+"-id ") || !strings.Contains(scopes, " urn:zitadel:iam:org:id:org ") || strings.Contains(scopes, "untrusted") {
			t.Fatalf("unsafe provider routing: %s", target)
		}
		response := http.Response{Header: headers}
		binding := ""
		for _, cookie := range response.Cookies() {
			if cookie.Name == e.srv.flowCookie() {
				binding = cookie.Value
			}
		}
		if binding == "" {
			t.Fatal("missing browser binding")
		}
		flow, err := e.store.ConsumeLoginFlow(context.Background(), q.Get("state"), store.Hash(binding))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(flow.Verifier))
		if flow.ReturnPath != "/studio?tab=posts" || flow.Nonce != q.Get("nonce") || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(digest[:]) {
			t.Fatalf("lost return/nonce/PKCE binding")
		}
	}
	if status, _, _ := emailHTTP(e, "GET", "/auth/login?provider=untrusted", "", nil); status != 400 {
		t.Fatalf("unknown provider %d", status)
	}
	e.srv.Config.GitHubIDP = ""
	if status, _, _ := emailHTTP(e, "GET", "/auth/login?provider=github", "", nil); status != 503 {
		t.Fatalf("disabled provider %d", status)
	}
	_, body, _ = emailHTTP(e, "GET", "/signin", "", nil)
	if strings.Contains(string(body), "Continue with GitHub") {
		t.Fatal("disabled provider advertised")
	}
}
