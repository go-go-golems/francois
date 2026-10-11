package host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/go-go-golems/francois/pkg/store"
)

func setup(t *testing.T) (*Host, *httptest.Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Import(ctx, "test", "Test", []byte(`<!doctype html><html><head><script>window.sawClaude=!!window.claude</script></head><body>test</body></html>`), map[string]json.RawMessage{"sample": json.RawMessage(`{}`), "db": json.RawMessage(`{}`), "files": json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(ctx, s, &piworker.Fake{}, "")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	server.Config.Handler = h.Handler("test", server.Listener.Addr().String())
	server.Start()
	t.Cleanup(func() { cancel(); h.cancel(); server.Close(); _ = h.group.Wait(); _ = s.Close() })
	return h, server
}
func post(t *testing.T, client *http.Client, url, view string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if view != "" {
		req.Header.Set("X-Francois-View", view)
	}
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var out map[string]any
	if err = json.NewDecoder(r.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return r.StatusCode, out
}
func TestInjectionOriginAndBrowserTools(t *testing.T) {
	_, server := setup(t)
	client := server.Client()
	r, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if bytes.Index(source, []byte("/static/runtime.js")) > bytes.Index(source, []byte("window.sawClaude")) {
		t.Fatal("runtime injected after page script")
	}
	request, _ := http.NewRequest("POST", server.URL+"/api/views", strings.NewReader(`{"version":1}`))
	request.Header.Set("Origin", "http://attacker.test")
	r, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatal("foreign origin admitted")
	}
	_, created := post(t, client, server.URL+"/api/views", "", map[string]int{"version": 1})
	view := created["viewId"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, _ = http.NewRequestWithContext(ctx, "GET", server.URL+"/api/views/"+view+"/events", nil)
	request.Header.Set("X-Francois-View", view)
	stream, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	scan := bufio.NewScanner(stream.Body)
	next := func() map[string]any {
		for scan.Scan() {
			line := scan.Text()
			if strings.HasPrefix(line, "data: ") {
				var e map[string]any
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
					t.Fatal(err)
				}
				return e
			}
		}
		t.Fatal("stream ended", scan.Err())
		return nil
	}
	if next()["type"] != "ready" {
		t.Fatal("missing ready")
	}
	status, _ := post(t, client, server.URL+"/api/views/"+view+"/calls", view, map[string]any{"callId": "c1", "input": "test", "options": map[string]string{"modelTier": "quick"}, "tools": []map[string]any{{"name": "eval", "description": "edit scene", "inputSchema": map[string]string{"type": "object"}}}})
	if status != 202 {
		t.Fatalf("call rejected %d", status)
	}
	var tool map[string]any
	for {
		e := next()
		if e["type"] == "tool" {
			tool = e
			break
		}
	}
	if tool["callId"] != "c1" || tool["name"] != "eval" {
		t.Fatal("wrong tool owner", tool)
	}
	status, _ = post(t, client, server.URL+"/api/views/"+view+"/calls/c1/tools/"+tool["toolCallId"].(string)+"/result", view, map[string]any{"value": map[string]bool{"ok": true}, "isError": false})
	if status != 200 {
		t.Fatal("tool result rejected")
	}
	for {
		e := next()
		if e["type"] == "result" {
			if e["result"].(map[string]any)["text"] != "The browser tool completed successfully." {
				t.Fatal(e)
			}
			break
		}
	}
}
func TestRootedFilesDenyTraversalAndSymlink(t *testing.T) {
	h, server := setup(t)
	dir := t.TempDir()
	h.ProjectRoot = dir
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../private.txt", "escape.txt"} {
		r, err := server.Client().Get(server.URL + "/api/files?path=" + name)
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
		if r.StatusCode == 200 {
			t.Fatalf("read escaped root: %s", name)
		}
	}
	r, err := server.Client().Get(server.URL + "/api/files?path=ok.txt&offset=2&length=2")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	data, _ := io.ReadAll(r.Body)
	if string(data) != "cd" || r.Header.Get("X-File-Revision") == "" {
		t.Fatalf("wrong range: %s", data)
	}
}
