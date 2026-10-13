package releases

import (
	"os"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestValidateExamplesAndBlockers(t *testing.T) {
	for _, f := range []string{"../../../examples/polis.html", "../../../examples/capabilities.html"} {
		html, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		_, checks := Validate(Upload{ArtifactID: "polis", Title: "POLIS", Version: "1.0.0", HTML: html})
		if Blocking(checks) {
			t.Fatalf("%s blocked: %+v", f, checks)
		}
	}
	base := `<!doctype html><html><head></head><body>`
	for name, body := range map[string]string{
		"bootstrap":  `<script type="application/json" id="francois-view">{}</script>`,
		"credential": `const key = "sk_live_abcdefghijklmnop"`,
		"private key": "-----BEGIN RSA PRIVATE KEY-----",
	} {
		_, checks := Validate(Upload{ArtifactID: "x", Title: "X", Version: "1", HTML: []byte(base + body + "</body></html>")})
		if !Blocking(checks) {
			t.Fatalf("%s not blocked", name)
		}
	}
	if _, checks := Validate(Upload{ArtifactID: "x", Title: "X", Version: "1", HTML: []byte(base + "</html>"), Manifest: []byte(`{"shell":{}}`)}); !Blocking(checks) {
		t.Fatal("unknown capability not blocked")
	}
}

func TestBlobsAreContentAddressedAndVerified(t *testing.T) {
	b, err := OpenBlobs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key, sum, err := b.Put([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	f := store.ReleaseFile{StorageKey: key, SHA256: sum}
	if data, err := b.Read(f); err != nil || string(data) != "hello" {
		t.Fatalf("read: %v %q", err, data)
	}
	p, _ := b.path(key)
	if err = os.WriteFile(p, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Read(f); err == nil {
		t.Fatal("tampered blob was accepted")
	}
	if len(b.Missing([]store.ReleaseFile{f})) != 1 {
		t.Fatal("tampered blob not reported")
	}
	if _, err = b.path("../../etc/passwd"); err == nil {
		t.Fatal("path traversal key accepted")
	}
}
