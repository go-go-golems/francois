package cmds

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// fakeAPI records requests and answers from a route table.
type fakeAPI struct {
	mu     sync.Mutex
	calls  []string
	bodies map[string]string
	reply  map[string]string
}

func (f *fakeAPI) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer frt_test" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		key := r.Method + " " + r.URL.Path
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, key)
		f.bodies[key] = string(b)
		body, ok := f.reply[key]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no route"}}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAPICommandTree(t *testing.T) {
	root := &cobra.Command{Use: "francois"}
	if err := Register(root); err != nil {
		t.Fatal(err)
	}
	for _, path := range [][]string{
		{"catalog", "whoami"}, {"catalog", "releases", "push"}, {"catalog", "users", "grant"}, {"catalog", "plans", "set"},
		{"catalog", "subscriptions", "cancel"}, {"catalog", "audit", "list"}, {"catalog", "account", "create-instance"},
		{"catalog", "db", "users", "promote"}, {"catalog", "db", "settings"}, {"catalog", "db", "publish"},
	} {
		if c, _, err := root.Find(path); err != nil || c.Name() != path[len(path)-1] {
			t.Errorf("missing command %v", path)
		}
	}
	if c, _, _ := root.Find([]string{"catalog", "push"}); c != nil && c.Name() == "push" {
		t.Error("catalog push should have moved to catalog releases push")
	}
}

func TestAPICommandsCallTheAPI(t *testing.T) {
	f := &fakeAPI{bodies: map[string]string{}, reply: map[string]string{
		"GET /api/me": `{"user":{"id":"usr_a","name":"Ada","email":"a@x","role":"admin","disabled":false},"token":{"id":"tok_1","name":"t","scopes":["admin:write"],"createdAt":"2026-10-15T00:00:00Z"},"creator":null}`,
		"POST /api/admin/users/usr_s/access-grants": `{"id":"agr_1","userId":"usr_s","until":"2026-11-15T00:00:00Z","reason":"beta","grantedBy":"usr_a","createdAt":"2026-10-15T00:00:00Z","plan":"pro"}`,
		"DELETE /api/admin/access-grants/agr_1":     `{"id":"agr_1","userId":"usr_s","until":"2026-11-15T00:00:00Z","reason":"beta","grantedBy":"usr_a","createdAt":"2026-10-15T00:00:00Z","plan":"pro"}`,
	}}
	srv := f.server(t)
	t.Setenv("FRANCOIS_SERVER", srv.URL)
	t.Setenv("FRANCOIS_TOKEN", "frt_test")

	rows, err := executeJSON(t, "catalog", "whoami", "--format", "json")
	if err != nil || len(rows) != 1 || rows[0]["user"].(map[string]any)["role"] != "admin" {
		t.Fatalf("whoami %v %v", rows, err)
	}
	rows, err = executeJSON(t, "catalog", "users", "grant", "usr_s", "--days", "30", "--reason", "beta", "--plan", "pro", "--format", "json")
	if err != nil || len(rows) != 1 || rows[0]["id"] != "agr_1" {
		t.Fatalf("grant %v %v", rows, err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(f.bodies["POST /api/admin/users/usr_s/access-grants"]), &body)
	if body["days"] != float64(30) || body["reason"] != "beta" || body["plan"] != "pro" {
		t.Fatalf("grant body %v", body)
	}

	// Destructive commands need --yes when stdin is not a terminal.
	if _, err = executeJSON(t, "catalog", "users", "revoke-grant", "agr_1", "--format", "json"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("revoke without --yes: %v", err)
	}
	if _, err = executeJSON(t, "catalog", "users", "revoke-grant", "agr_1", "--yes", "--format", "json"); err != nil {
		t.Fatalf("revoke with --yes: %v", err)
	}
	// API errors come back with their code.
	if _, err = executeJSON(t, "catalog", "instances", "show", "ins_missing", "--format", "json"); err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("missing instance: %v", err)
	}
}
