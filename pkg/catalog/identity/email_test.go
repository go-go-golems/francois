package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmailCredentialDoesNotFollowRedirectsOrLeakErrors(t *testing.T) {
	hit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer target.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/secret-code", http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	p, _ := NewEmailLogin(upstream.URL, "org", "private-token")
	_, err := p.ResolveEmail(context.Background(), "alice@example.test", "", false)
	if err == nil || hit || strings.Contains(err.Error(), "secret-code") || strings.Contains(err.Error(), "private-token") {
		t.Fatalf("unsafe redirect/error: hit=%v err=%v", hit, err)
	}
	for _, issuer := range []string{"http://untrusted.example", "https://user:password@example.com", "https://example.com/?x=y"} {
		if _, err = NewEmailLogin(issuer, "org", "token"); err == nil {
			t.Errorf("accepted %s", issuer)
		}
	}
}
