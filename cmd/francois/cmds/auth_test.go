package cmds

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	f := &fakeAPI{bodies: map[string]string{}, reply: map[string]string{"GET /api/me": meReply, "DELETE /api/me/token": `{"revoked":true}`}}
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

	if rows, err = executeJSON(t, "catalog", "auth", "logout", "--format", "json"); err != nil || len(rows) != 1 || rows[0]["logged_out"] != true || rows[0]["token"] != "revoked" {
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

// deviceAPI scripts the device endpoints: polls answer from replies in order.
func deviceAPI(t *testing.T, replies ...string) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization")+" "+string(b))
		switch r.URL.Path {
		case "/api/device/authorize":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"deviceCode":"fdc_x","userCode":"WDJB-MJHT","verificationUri":"http://` + r.Host + `/device",` +
				`"verificationUriComplete":"http://` + r.Host + `/device?code=WDJB-MJHT","expiresIn":600,"interval":0}`))
		case "/api/device/token":
			reply := replies[0]
			replies = replies[1:]
			if strings.HasPrefix(reply, "{\"token\"") {
				_, _ = w.Write([]byte(reply))
				return
			}
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":{"code":"` + reply + `","message":"m"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func fastPolls(t *testing.T) {
	floor, slow := devicePollFloor, deviceSlowDown
	devicePollFloor, deviceSlowDown = time.Millisecond, time.Millisecond
	t.Cleanup(func() { devicePollFloor, deviceSlowDown = floor, slow })
}

func TestAuthLoginDevice(t *testing.T) {
	isolateConfig(t)
	fastPolls(t)
	issued := `{"token":"frt_new","info":{"id":"tok_9","name":"cli","scopes":["studio","account"],"createdAt":"2026-10-21T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
		`"user":{"id":"usr_a","name":"Ada","email":"a@x","role":"creator","disabled":false}}`
	srv, calls := deviceAPI(t, "authorization_pending", "slow_down", "authorization_pending", issued)
	rows, err := executeJSON(t, "catalog", "auth", "login", "--server", srv.URL, "--no-browser", "--name", "cli", "--days", "30", "--format", "json")
	if err != nil || len(rows) != 1 || rows[0]["user"] != "Ada" || rows[0]["expires"] != "2099-01-01" {
		t.Fatalf("login %v %v", rows, err)
	}
	if len(*calls) != 5 || !strings.Contains((*calls)[0], `"clientName":"cli"`) || !strings.Contains((*calls)[0], `"days":30`) ||
		!strings.Contains((*calls)[0], `"scopes":null`) || strings.Contains((*calls)[0], "Bearer") || !strings.Contains((*calls)[1], `"deviceCode":"fdc_x"`) {
		t.Fatalf("calls %q", *calls)
	}
	f, _, _ := credentials.Load()
	if e := f.Servers[srv.URL]; e.Token != "frt_new" || e.TokenID != "tok_9" || f.Default != srv.URL {
		t.Fatalf("stored %+v", f)
	}
}

func TestAuthLoginDeviceRefusals(t *testing.T) {
	fastPolls(t)
	for reply, want := range map[string]string{"access_denied": "denied", "expired_token": "expired"} {
		isolateConfig(t)
		srv, _ := deviceAPI(t, "authorization_pending", reply)
		if _, err := executeJSON(t, "catalog", "auth", "login", "--server", srv.URL, "--no-browser", "--format", "json"); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: %v", reply, err)
		}
		if f, _, _ := credentials.Load(); len(f.Servers) != 0 {
			t.Fatalf("%s stored %+v", reply, f)
		}
	}
}
