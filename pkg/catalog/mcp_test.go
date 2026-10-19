package catalog

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/mcpserver"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/go-go-mcp/pkg/embeddable"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpSession serves the catalog's MCP tools over streamable HTTP, built the
// same way francois mcp builds them, and connects an MCP client.
func (e *env) mcpSession(secret string, s mcpserver.Settings) *mcp.ClientSession {
	e.t.Helper()
	c := client.New(e.ts.URL, secret, client.WithHost("portal.test"))
	opts, err := mcpserver.Options(func(context.Context) (*client.Client, error) { return c, nil }, s)
	if err != nil {
		e.t.Fatal(err)
	}
	cfg := embeddable.NewServerConfig()
	for _, o := range append([]embeddable.ServerOption{embeddable.WithName("francois"), embeddable.WithDefaultTransport("streamable_http")}, opts...) {
		if err := o(cfg); err != nil {
			e.t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	if err := embeddable.MountHTTPHandlers(mux, cfg); err != nil {
		e.t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	e.t.Cleanup(srv.Close)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String(), res.IsError
}

func TestMCPTools(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	ada := e.user("ada", "admin", false)
	sam := e.user("sam", "subscriber", true)
	cleo := e.user("cleo", "creator", false)
	adminSecret, _ := e.mint(&ada, 30, "studio", "admin:write")
	cleoSecret, _ := e.mint(&cleo, 0, "studio")

	// Tools follow the token's scopes and the selection.
	names := func(s mcpserver.Settings) []string { return mcpserver.ToolNames(s) }
	studio := names(mcpserver.Settings{Scopes: []string{"studio"}})
	if !slices.Contains(studio, "push_release") || slices.Contains(studio, "find_customers") || slices.Contains(studio, "my_account") {
		t.Fatalf("studio token tools %v", studio)
	}
	readOnly := names(mcpserver.Settings{Scopes: []string{"admin:read"}, ReadOnly: true})
	if !slices.Contains(readOnly, "find_customers") || slices.Contains(readOnly, "grant_complimentary_access") {
		t.Fatalf("admin:read read-only tools %v", readOnly)
	}
	account := names(mcpserver.Settings{Scopes: []string{"account"}, Toolsets: []string{"account"}})
	if !slices.Contains(account, "create_my_instance") || slices.Contains(account, "list_artifacts") {
		t.Fatalf("account tools %v", account)
	}

	// An agent with an admin token finds a customer and grants access.
	dir := t.TempDir()
	s := e.mcpSession(adminSecret, mcpserver.Settings{Mode: mcpserver.Local, Scopes: []string{"studio", "admin:write"}, AllowDirs: []string{dir}})
	list, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var grant *mcp.Tool
	for _, tl := range list.Tools {
		if tl.Name == "grant_complimentary_access" {
			grant = tl
		}
	}
	if grant == nil || grant.Annotations == nil || grant.Annotations.ReadOnlyHint {
		t.Fatalf("grant tool %+v", grant)
	}
	schema, _ := json.Marshal(grant.InputSchema)
	if !strings.Contains(string(schema), `"userId"`) || !strings.Contains(string(schema), `"required"`) {
		t.Fatalf("grant schema %s", schema)
	}
	if text, isErr := callTool(t, s, "find_customers", map[string]any{"query": "sam"}); isErr || !strings.Contains(text, sam.user.ID) {
		t.Fatalf("find_customers %v %s", isErr, text)
	}
	if text, isErr := callTool(t, s, "grant_complimentary_access", map[string]any{"userId": sam.user.ID, "days": 30, "reason": "beta", "plan": "subscriber"}); isErr || !strings.Contains(text, "Granted subscriber") {
		t.Fatalf("grant %v %s", isErr, text)
	}
	entries, _ := e.store.AuditQuery(ctx, auditFilter("user.access_grant"))
	if len(entries) == 0 || entries[0].TokenID == nil {
		t.Fatalf("grant not attributed to the token: %+v", entries)
	}
	// API errors reach the model as tool errors with the code.
	if text, isErr := callTool(t, s, "grant_complimentary_access", map[string]any{"userId": sam.user.ID, "days": 30, "reason": "x", "plan": "nope"}); !isErr || !strings.Contains(text, "invalid_request") {
		t.Fatalf("bad plan %v %s", isErr, text)
	}

	// Pushing a release reads files only under the allowed directory.
	htmlPath := filepath.Join(dir, "app.html")
	if err := os.WriteFile(htmlPath, []byte(testHTML), 0o600); err != nil {
		t.Fatal(err)
	}
	if text, isErr := callTool(t, s, "push_release", map[string]any{"artifactId": "loom", "version": "1.0.0", "htmlPath": htmlPath, "title": "Loom", "publish": true}); isErr || !strings.Contains(text, "published") {
		t.Fatalf("push %v %s", isErr, text)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0o600)
	link := filepath.Join(dir, "innocent.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	rels, _ := e.store.Releases(ctx, "loom")
	for _, p := range []string{outside, link} {
		if text, isErr := callTool(t, s, "add_attachment", map[string]any{"releaseId": rels[0].ID, "path": p}); !isErr || !strings.Contains(text, "outside the allowed directories") {
			t.Fatalf("attachment from %s: %v %s", p, isErr, text)
		}
	}

	// A creator's studio token: the API still refuses what the role forbids.
	cs := e.mcpSession(cleoSecret, mcpserver.Settings{Mode: mcpserver.Local, Scopes: []string{"studio"}, AllowDirs: []string{dir}})
	if text, isErr := callTool(t, cs, "create_creator", map[string]any{"slug": "x", "name": "X"}); !isErr || !strings.Contains(text, "admin") {
		t.Fatalf("creator creating a profile %v %s", isErr, text)
	}
}

func auditFilter(action string) store.AuditFilter { return store.AuditFilter{Action: action, Limit: 5} }

// bearerTransport sends every request to the portal host with a token.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Host = "portal.test"
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

// remoteMCP connects to /mcp the way production traffic arrives: through an
// ingress that reaches the pod on a non-loopback address with the public
// Host. (go-sdk rejects a non-localhost Host on a loopback listener as DNS
// rebinding; httptest listens on loopback.)
func (e *env) remoteMCP(token string) (*mcp.ClientSession, error) {
	podAddr := &net.TCPAddr{IP: net.ParseIP("10.42.0.7"), Port: 8080}
	ingress := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.srv.Handler().ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, podAddr)))
	}))
	e.t.Cleanup(ingress.Close)
	return mcp.NewClient(&mcp.Implementation{Name: "remote", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: ingress.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token}}, MaxRetries: -1}, nil)
}

// TestRemoteMCP checks /mcp inside catalog serve: personal tokens
// authenticate, tool calls go through the API in-process, scopes apply and
// file paths are refused.
func TestRemoteMCP(t *testing.T) {
	e := setup(t, func(c *Config) { c.MCP = true })
	ctx := context.Background()
	ada := e.user("ada", "admin", false)
	sam := e.user("sam", "subscriber", true)
	cleo := e.user("cleo", "creator", false)
	adminSecret, adminToken := e.mint(&ada, 30, "studio", "admin:write")
	cleoSecret, _ := e.mint(&cleo, 0, "studio")

	if _, err := e.remoteMCP(""); err == nil {
		t.Fatal("connected without a token")
	}
	code, _, h := e.do(nil, "portal.test", "POST", "/mcp", strings.NewReader(`{}`), nil)
	if code != 401 || !strings.Contains(h.Get("WWW-Authenticate"), "resource_metadata") {
		t.Fatalf("anonymous /mcp: %d %q", code, h.Get("WWW-Authenticate"))
	}

	s, err := e.remoteMCP(adminSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if text, isErr := callTool(t, s, "whoami", nil); isErr || !strings.Contains(text, "admin") {
		t.Fatalf("whoami %v %s", isErr, text)
	}
	if text, isErr := callTool(t, s, "grant_complimentary_access", map[string]any{"userId": sam.user.ID, "days": 7, "reason": "remote", "plan": "subscriber"}); isErr {
		t.Fatalf("grant %s", text)
	}
	entries, _ := e.store.AuditQuery(ctx, auditFilter("user.access_grant"))
	if len(entries) == 0 || entries[0].TokenID == nil || *entries[0].TokenID != adminToken {
		t.Fatalf("remote grant not attributed to the token: %+v", entries)
	}
	// Remote servers take content inline, never local paths.
	if text, isErr := callTool(t, s, "push_release", map[string]any{"artifactId": "loom", "version": "1.0.0", "htmlPath": "/etc/passwd"}); !isErr || !strings.Contains(text, "not accepted by the remote server") {
		t.Fatalf("remote path %v %s", isErr, text)
	}
	if text, isErr := callTool(t, s, "push_release", map[string]any{"artifactId": "loom", "version": "1.0.0", "html": testHTML, "title": "Loom"}); isErr || !strings.Contains(text, "draft") {
		t.Fatalf("remote inline push %v %s", isErr, text)
	}

	// A studio token cannot use customer tools.
	cs, err := e.remoteMCP(cleoSecret)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "find_customers", Arguments: map[string]any{"query": "sam"}})
	if err == nil && !res.IsError {
		t.Fatalf("studio token used a customer tool: %+v", res)
	}
	if text, isErr := callTool(t, cs, "list_artifacts", nil); isErr || !strings.Contains(text, "artifacts") {
		t.Fatalf("studio tool %v %s", isErr, text)
	}
}
