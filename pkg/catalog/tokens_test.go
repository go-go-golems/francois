package catalog

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/studioclient"
)

func TestTokensAndPush(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	sam := e.user("sam", "subscriber", true)
	if code, out := e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": "cleo", "name": "Cleo", "userId": cleo.user.ID}); code != 201 {
		t.Fatalf("profile %d %v", code, out)
	}
	code, out := e.portal(&sam, "POST", "/api/studio/tokens", map[string]any{"name": "laptop"})
	if code != 403 || errCode(out) != "creator_required" {
		t.Fatalf("subscriber token %d %v", code, out)
	}
	code, out = e.portal(&cleo, "POST", "/api/studio/tokens", map[string]any{"name": "laptop", "days": 30})
	if code != 201 {
		t.Fatalf("create token %d %v", code, out)
	}
	secret := out["token"].(string)
	tokenID := out["info"].(map[string]any)["id"].(string)
	if !strings.HasPrefix(secret, "frt_") || len(secret) != 4+43 {
		t.Fatalf("token format %q", secret)
	}
	bearer := func(method, path, body string) (int, string) {
		var r io.Reader
		if body != "" {
			r = strings.NewReader(body)
		}
		code, b, _ := e.do(nil, "portal.test", method, path, r, map[string]string{"Authorization": "Bearer " + secret})
		return code, string(b)
	}

	// The token works on studio routes only, without Origin or CSRF, and
	// cannot manage tokens.
	if code, b := bearer("GET", "/api/studio/artifacts", ""); code != 200 {
		t.Fatalf("token on studio %d %s", code, b)
	}
	for _, path := range []string{"/api/account", "/api/instances", "/api/admin/users"} {
		if code, b := bearer("GET", path, ""); code != 401 || !strings.Contains(b, "token_not_allowed") {
			t.Fatalf("token on %s: %d %s", path, code, b)
		}
	}
	if code, b := bearer("POST", "/api/studio/tokens", `{"name":"more"}`); code != 403 || !strings.Contains(b, "session_required") {
		t.Fatalf("token minting tokens %d %s", code, b)
	}

	// Push a new artifact and publish it.
	client := studioclient.New(e.ts.URL, secret)
	client.Host = "portal.test"
	res, err := client.Push(ctx, studioclient.Release{ArtifactID: "loom", Title: "Loom", Version: "1.0.0", HTML: []byte(testHTML), Manifest: []byte(`{"db":{}}`)}, true)
	if err != nil || res.State != "published" || res.ReleaseID == "" {
		t.Fatalf("push %+v %v", res, err)
	}
	if ok, _ := e.store.IsArtifactCreator(ctx, "loom", cleo.user.ID); !ok {
		t.Fatal("pushed artifact not credited to the token owner")
	}
	checks, err := client.Check(ctx, studioclient.Release{ArtifactID: "loom", Version: "1.0.1", HTML: []byte("not html")})
	if err != nil || len(checks) == 0 {
		t.Fatalf("check %v %v", checks, err)
	}
	_, err = client.Push(ctx, studioclient.Release{ArtifactID: "loom", Version: "1.0.0", HTML: []byte(testHTML)}, false)
	var apiErr *studioclient.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "version_exists" {
		t.Fatalf("duplicate version %v", err)
	}
	// Another creator's artifact stays out of reach.
	if _, err = e.store.Pool.Exec(ctx, `INSERT INTO catalog_artifacts(id,title) VALUES('other','Other')`); err != nil {
		t.Fatal(err)
	}
	_, err = client.Push(ctx, studioclient.Release{ArtifactID: "other", Version: "1.0.0", HTML: []byte(testHTML)}, false)
	if !errors.As(err, &apiErr) || apiErr.Code != "not_editor" {
		t.Fatalf("push to someone else's artifact %v", err)
	}

	// Listing shows use; revoking ends the token.
	_, list, _ := e.do(&cleo, "portal.test", "GET", "/api/studio/tokens", nil, nil)
	if !strings.Contains(string(list), `"lastUsedAt"`) || strings.Contains(string(list), secret) {
		t.Fatalf("token list %s", list)
	}
	if code, out := e.portal(&cleo, "DELETE", "/api/studio/tokens/"+tokenID, nil); code != 200 {
		t.Fatalf("revoke %d %v", code, out)
	}
	if code, b := bearer("GET", "/api/studio/artifacts", ""); code != 401 || !strings.Contains(b, "token_invalid") {
		t.Fatalf("revoked token %d %s", code, b)
	}
	if code, _ := e.portal(&admin, "DELETE", "/api/studio/tokens/"+tokenID, nil); code != 404 {
		t.Fatal("revoked someone else's token")
	}
}
