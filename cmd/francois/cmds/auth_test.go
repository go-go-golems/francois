package cmds

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/credentials"
	"github.com/spf13/cobra"
)

const meReply = `{"user":{"id":"usr_a","name":"Ada","email":"a@x","role":"creator","disabled":false},` +
	`"token":{"id":"tok_1","name":"laptop","scopes":["studio","account"],"createdAt":"2026-10-15T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},"creator":null}`

// isolateConfig gives the test an empty credentials directory and no
// connection environment.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "francois")
	t.Setenv("FRANCOIS_CONFIG_DIR", dir)
	t.Setenv("FRANCOIS_SERVER", "")
	t.Setenv("FRANCOIS_TOKEN", "")
	return dir
}

// withStdin feeds input to os.Stdin for the duration of the test.
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString(input)
	_ = w.Close()
	original := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = original; _ = r.Close() })
}

// executeText runs a command and returns its raw stdout.
func executeText(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "francois", SilenceUsage: true, SilenceErrors: true}
	if err := Register(root); err != nil {
		t.Fatal(err)
	}
	root.SetArgs(args)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	err = root.Execute()
	_ = writer.Close()
	os.Stdout = original
	data, _ := io.ReadAll(reader)
	_ = reader.Close()
	return string(data), err
}

func TestAuthLoginWithTokenStoresAndLoads(t *testing.T) {
	dir := isolateConfig(t)
	f := &fakeAPI{bodies: map[string]string{}, reply: map[string]string{"GET /api/me": meReply}}
	srv := f.server(t)

	withStdin(t, "frt_test\n")
	rows, err := executeJSON(t, "catalog", "auth", "login", "--with-token", "--server", srv.URL+"/", "--format", "json")
	if err != nil || len(rows) != 1 || rows[0]["user"] != "Ada" || rows[0]["scopes"] != "studio account" || rows[0]["default"] != true {
		t.Fatalf("login %v %v", rows, err)
	}
	file, _, err := credentials.Load()
	e, ok := file.Servers[srv.URL]
	if err != nil || !ok || e.Token != "frt_test" || e.TokenID != "tok_1" || file.Default != srv.URL {
		t.Fatalf("stored %+v %v", file, err)
	}
	if st, err := os.Stat(filepath.Join(dir, "credentials.json")); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v %v", st, err)
	}

	// Without any flag or environment, commands use the stored login.
	if rows, err = executeJSON(t, "catalog", "whoami", "--format", "json"); err != nil || len(rows) != 1 {
		t.Fatalf("whoami from the stored login %v %v", rows, err)
	}
	if rows, err = executeJSON(t, "catalog", "auth", "status", "--format", "json"); err != nil || len(rows) != 1 || rows[0]["used"] != "yes" {
		t.Fatalf("status %v %v", rows, err)
	}
	if rows, err = executeJSON(t, "catalog", "auth", "status", "--check", "--format", "json"); err != nil || rows[0]["check"] != "valid" {
		t.Fatalf("status --check %v %v", rows, err)
	}
	if out, err := executeText(t, "catalog", "auth", "token"); err != nil || out != "frt_test\n" {
		t.Fatalf("token %q %v", out, err)
	}

	// FRANCOIS_TOKEN still wins, and status says so.
	t.Setenv("FRANCOIS_TOKEN", "frt_env")
	if rows, err = executeJSON(t, "catalog", "auth", "status", "--format", "json"); err != nil || rows[0]["used"] != "no, overridden by FRANCOIS_TOKEN" {
		t.Fatalf("status with FRANCOIS_TOKEN %v %v", rows, err)
	}
	t.Setenv("FRANCOIS_TOKEN", "")

	if rows, err = executeJSON(t, "catalog", "auth", "logout", "--format", "json"); err != nil || len(rows) != 1 || rows[0]["logged_out"] != true {
		t.Fatalf("logout %v %v", rows, err)
	}
	if file, _, _ = credentials.Load(); len(file.Servers) != 0 || file.Default != "" {
		t.Fatalf("after logout %+v", file)
	}
	if _, err = executeJSON(t, "catalog", "whoami", "--format", "json"); err == nil || !strings.Contains(err.Error(), "auth login") {
		t.Fatalf("whoami after logout: %v", err)
	}
}

func TestAuthLoginRejectsBadInput(t *testing.T) {
	isolateConfig(t)
	withStdin(t, "not-a-token\n")
	if _, err := executeJSON(t, "catalog", "auth", "login", "--with-token", "--server", "http://127.0.0.1:1", "--format", "json"); err == nil || !strings.Contains(err.Error(), "frt_") {
		t.Fatalf("bad token: %v", err)
	}
	if _, err := executeJSON(t, "catalog", "auth", "login", "--with-token", "--format", "json"); err == nil || !strings.Contains(err.Error(), "--server") {
		t.Fatalf("no server: %v", err)
	}
}

func TestRejectedTokenSuggestsLogin(t *testing.T) {
	isolateConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":"token_invalid","message":"The API token is invalid, expired or revoked"}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("FRANCOIS_SERVER", srv.URL)
	t.Setenv("FRANCOIS_TOKEN", "frt_old")
	if _, err := executeJSON(t, "catalog", "whoami", "--format", "json"); err == nil || !strings.Contains(err.Error(), "auth login") || !strings.Contains(err.Error(), "token_invalid") {
		t.Fatalf("rejected token: %v", err)
	}
}

func TestExpiredStoredTokenIsNotSent(t *testing.T) {
	isolateConfig(t)
	f := credentials.File{}
	f.Put("https://slop.boutique", credentials.Entry{Token: "frt_x", ExpiresAt: ptrTime("2020-01-01T00:00:00Z")}, true)
	if err := credentials.Save(f); err != nil {
		t.Fatal(err)
	}
	if _, err := executeJSON(t, "catalog", "whoami", "--format", "json"); err == nil || !strings.Contains(err.Error(), "expired on 2020-01-01") {
		t.Fatalf("expired: %v", err)
	}
}

func ptrTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}
