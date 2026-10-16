package catalog

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/client"
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
	code, out := e.portal(&sam, "POST", "/api/tokens", map[string]any{"name": "laptop", "scopes": []string{"studio"}})
	if code != 400 || errCode(out) != "invalid_scopes" {
		t.Fatalf("subscriber studio token %d %v", code, out)
	}
	code, out = e.portal(&cleo, "POST", "/api/tokens", map[string]any{"name": "laptop", "days": 30, "scopes": []string{"studio"}})
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

	// A studio token works on studio routes, without Origin or CSRF; other
	// surfaces need other scopes, and no token can manage tokens.
	if code, b := bearer("GET", "/api/studio/artifacts", ""); code != 200 {
		t.Fatalf("token on studio %d %s", code, b)
	}
	for _, path := range []string{"/api/account", "/api/instances", "/api/admin/users"} {
		if code, b := bearer("GET", path, ""); code != 403 || !strings.Contains(b, "token_scope") {
			t.Fatalf("token on %s: %d %s", path, code, b)
		}
	}
	if code, b := bearer("POST", "/api/tokens", `{"name":"more","scopes":["studio"]}`); code != 401 || !strings.Contains(b, "token_not_allowed") {
		t.Fatalf("token minting tokens %d %s", code, b)
	}

	// Push a new artifact and publish it.
	studio := client.New(e.ts.URL, secret, client.WithHost("portal.test"), client.WithStrictDecoding()).Studio
	dir := t.TempDir()
	manual, notes := filepath.Join(dir, "Manual.pdf"), filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(manual, []byte("%PDF-1.4 manual"), 0o600)
	_ = os.WriteFile(notes, []byte("release notes"), 0o600)
	res, err := studio.Push(ctx, client.ReleaseUpload{ArtifactID: "loom", Title: "Loom", Version: "1.0.0", HTML: []byte(testHTML), Manifest: []byte(`{"db":{}}`)},
		[]string{manual, notes}, true)
	if err != nil || res.Release.State != "published" || res.Release.ID == "" || len(res.Attachments) != 2 || res.Attachments[0].Name != "Manual.pdf" {
		t.Fatalf("push %+v %v", res, err)
	}
	if rel, _ := e.store.Release(ctx, res.Release.ID); len(rel.Attachments) != 2 || rel.Attachments[1].Size != int64(len("release notes")) {
		t.Fatalf("pushed attachments %+v", rel.Attachments)
	}
	if ok, _ := e.store.IsArtifactCreator(ctx, "loom", cleo.user.ID); !ok {
		t.Fatal("pushed artifact not credited to the token owner")
	}
	checked, err := studio.CheckRelease(ctx, client.ReleaseUpload{ArtifactID: "loom", Version: "1.0.1", HTML: []byte("not html")})
	if err != nil || len(checked.Checks) == 0 {
		t.Fatalf("check %v %v", checked, err)
	}
	_, err = studio.Push(ctx, client.ReleaseUpload{ArtifactID: "loom", Version: "1.0.0", HTML: []byte(testHTML)}, nil, false)
	var apiErr *client.Error
	if !errors.As(err, &apiErr) || apiErr.Code != "version_exists" {
		t.Fatalf("duplicate version %v", err)
	}
	// Another creator's artifact stays out of reach.
	if _, err = e.store.Pool.Exec(ctx, `INSERT INTO catalog_artifacts(id,title) VALUES('other','Other')`); err != nil {
		t.Fatal(err)
	}
	_, err = studio.Push(ctx, client.ReleaseUpload{ArtifactID: "other", Version: "1.0.0", HTML: []byte(testHTML)}, nil, false)
	if !errors.As(err, &apiErr) || apiErr.Code != "not_editor" {
		t.Fatalf("push to someone else's artifact %v", err)
	}

	// Listing shows use; revoking ends the token.
	_, list, _ := e.do(&cleo, "portal.test", "GET", "/api/tokens", nil, nil)
	if !strings.Contains(string(list), `"lastUsedAt"`) || !strings.Contains(string(list), `"scopes":["studio"]`) || strings.Contains(string(list), secret) {
		t.Fatalf("token list %s", list)
	}
	if code, out := e.portal(&cleo, "DELETE", "/api/tokens/"+tokenID, nil); code != 200 {
		t.Fatalf("revoke %d %v", code, out)
	}
	if code, b := bearer("GET", "/api/studio/artifacts", ""); code != 401 || !strings.Contains(b, "token_invalid") {
		t.Fatalf("revoked token %d %s", code, b)
	}
	if code, _ := e.portal(&admin, "DELETE", "/api/tokens/"+tokenID, nil); code != 404 {
		t.Fatal("revoked someone else's token")
	}
}
