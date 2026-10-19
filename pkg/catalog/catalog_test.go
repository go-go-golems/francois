package catalog

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/jackc/pgx/v5"
)

const marker = "PAID-MARKER-7f3a"

// testDB creates an isolated database on the server named by
// TEST_DATABASE_URL, e.g. postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable
func testDB(t *testing.T) *store.Store {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := "catalog_test_" + store.RandomHex(6)
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + name
	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	if _, err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}

type env struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	store  *store.Store
	client *http.Client
}

type tactor struct {
	user  store.User
	token string
	csrf  string
}

func setup(t *testing.T, opts ...func(*Config)) *env {
	s := testDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	blobs, err := releases.OpenBlobs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	public, _ := url.Parse("http://portal.test")
	// The second site uses its portal hostname as application domain, like
	// a single-wildcard custom domain does in production.
	boutique, _ := url.Parse("http://boutique.test")
	cfg := Config{Sites: []Site{
		{PublicURL: public, AppDomain: "apps.test", OIDC: identity.New(identity.Config{})},
		{PublicURL: boutique, AppDomain: "boutique.test", OIDC: identity.New(identity.Config{})},
	}}
	for _, o := range opts {
		o(&cfg)
	}
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSettings(ctx, "", testDoc(100)); err != nil {
		t.Fatal(err)
	}
	srv := New(ctx, cfg, s, &releases.Publisher{Store: s, Blobs: blobs}, nil, &piworker.Fake{})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { cancel(); ts.Close(); _ = srv.Runtime.Wait() })
	return &env{t: t, srv: srv, ts: ts, store: s, client: ts.Client()}
}

// testDoc is a settings document with the free plan and a paid "subscriber" plan
// (10 workspaces, 10 members, monthly credits as given).
func testDoc(monthly int, models ...settings.Model) settings.Document {
	d := settings.Defaults()
	if len(models) > 0 {
		d.Models.Allowed = models
	}
	d.Plans = append(d.Plans, settings.Plan{Key: "subscriber", Name: "Pro", Rank: 10, Public: true, Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_test_pro", Label: "5 / month"}},
		Instances: 10, MembersPerInstance: 10, MonthlyCredits: monthly})
	return d
}

func (e *env) user(name string, role string, access bool) tactor {
	ctx := context.Background()
	u, err := e.store.UpsertIdentity(ctx, store.Identity{Issuer: "https://issuer.test", Subject: "sub-" + name, Email: name + "@example.com", EmailVerified: true, Name: name})
	if err != nil {
		e.t.Fatal(err)
	}
	if role != "" && role != "subscriber" {
		if err = e.store.SetUserRole(ctx, u.ID, role); err != nil {
			e.t.Fatal(err)
		}
		u.Role = role
	}
	if access {
		if _, err = e.store.GrantAccess(ctx, u.ID, u.ID, time.Now().Add(time.Hour), "test", ""); err != nil {
			e.t.Fatal(err)
		}
	}
	token, sess, err := e.store.CreateSession(ctx, u.ID, "")
	if err != nil {
		e.t.Fatal(err)
	}
	return tactor{user: u, token: token, csrf: sess.CSRF}
}

// do sends a request with an explicit Host; portal mutations carry Origin
// and CSRF like the SPA does.
func (e *env) do(a *tactor, host, method, path string, body io.Reader, headers map[string]string) (int, []byte, http.Header) {
	e.t.Helper()
	req, err := http.NewRequest(method, e.ts.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Host = host
	if a != nil && (host == "portal.test" || host == "boutique.test") {
		req.AddCookie(&http.Cookie{Name: "francois_session_dev", Value: a.token})
		if method != "GET" {
			req.Header.Set("Origin", "http://"+host)
			req.Header.Set("X-CSRF-Token", a.csrf)
		}
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, b, r.Header
}

func (e *env) portal(a *tactor, method, path string, v any) (int, map[string]any) {
	return e.portalAt("portal.test", a, method, path, v)
}

func (e *env) portalAt(host string, a *tactor, method, path string, v any) (int, map[string]any) {
	var body io.Reader
	if v != nil {
		raw, _ := json.Marshal(v)
		body = bytes.NewReader(raw)
	}
	code, b, _ := e.do(a, host, method, path, body, nil)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return code, out
}

func errCode(m map[string]any) string {
	if e, ok := m["error"].(map[string]any); ok {
		c, _ := e["code"].(string)
		return c
	}
	return ""
}

func (e *env) upload(a tactor, html string) (int, map[string]any) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("artifactId", "board")
	_ = w.WriteField("title", "Board")
	_ = w.WriteField("version", "1.0.0")
	_ = w.WriteField("manifestJson", `{"sample":{},"db":{},"user":{"scopes":["profile"]},"room":{}}`)
	f, _ := w.CreateFormFile("html", "board.html")
	_, _ = f.Write([]byte(html))
	_ = w.Close()
	code, b, _ := e.do(&a, "portal.test", "POST", "/api/studio/releases", &buf, map[string]string{"Content-Type": w.FormDataContentType()})
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return code, out
}

func pkce() (string, string) {
	verifier := base64.RawURLEncoding.EncodeToString([]byte(store.RandomHex(16)))
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

type grant struct {
	host, origin, token string
}

// launch performs the portal launch and app-origin exchange.
func (e *env) launch(a tactor, instance map[string]any) (grant, int, string) {
	return e.launchAt("portal.test", a, instance)
}

func (e *env) launchAt(portal string, a tactor, instance map[string]any) (grant, int, string) {
	verifier, challenge := pkce()
	code, out := e.portalAt(portal, &a, "POST", "/api/instances/"+instance["id"].(string)+"/launch", map[string]string{"challenge": challenge})
	if code != 200 {
		return grant{}, code, errCode(out)
	}
	origin := out["origin"].(string)
	host := strings.TrimPrefix(origin, "http://")
	raw, _ := json.Marshal(map[string]string{"code": out["code"].(string), "verifier": verifier})
	st, b, _ := e.do(nil, host, "POST", "/api/launch/exchange", bytes.NewReader(raw), map[string]string{"Origin": origin})
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	if st != 200 {
		return grant{}, st, errCode(m)
	}
	return grant{host: host, origin: origin, token: m["grant"].(string)}, 200, ""
}

func (e *env) app(g grant, method, path string, v any, extra map[string]string) (int, map[string]any) {
	var body io.Reader
	if v != nil {
		raw, _ := json.Marshal(v)
		body = bytes.NewReader(raw)
	}
	h := map[string]string{"Authorization": "Bearer " + g.token, "Origin": g.origin}
	for k, val := range extra {
		h[k] = val
	}
	code, b, _ := e.do(nil, g.host, method, path, body, h)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return code, out
}

type view struct {
	g      grant
	id     string
	events chan map[string]any
}

func (e *env) view(g grant, connect bool) *view {
	code, out := e.app(g, "POST", "/api/views", map[string]int{"version": 1}, nil)
	if code != 200 {
		e.t.Fatalf("create view: %d %v", code, out)
	}
	v := &view{g: g, id: out["viewId"].(string), events: make(chan map[string]any, 256)}
	if connect {
		req, _ := http.NewRequest("GET", e.ts.URL+"/api/views/"+v.id+"/events", nil)
		req.Host = g.host
		req.Header.Set("Authorization", "Bearer "+g.token)
		req.Header.Set("X-Francois-View", v.id)
		r, err := e.client.Do(req)
		if err != nil || r.StatusCode != 200 {
			e.t.Fatalf("connect view: %v", err)
		}
		ready := make(chan struct{})
		go func() {
			defer r.Body.Close()
			s := bufio.NewScanner(r.Body)
			once := sync.Once{}
			for s.Scan() {
				line := s.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				m := map[string]any{}
				_ = json.Unmarshal([]byte(line[6:]), &m)
				if m["type"] == "ready" {
					once.Do(func() { close(ready) })
					continue
				}
				v.events <- m
			}
		}()
		<-ready
	}
	return v
}

func (e *env) rpc(v *view, method string, args any) (int, map[string]any) {
	return e.app(v.g, "POST", "/api/views/"+v.id+"/rpc", map[string]any{"method": method, "args": args}, map[string]string{"X-Francois-View": v.id})
}

func waitFor(t *testing.T, v *view, match func(map[string]any) bool) map[string]any {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case m := <-v.events:
			if match(m) {
				return m
			}
		case <-timeout:
			t.Fatal("timed out waiting for event")
		}
	}
}

func TestHostedPlatform(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	alice := e.user("alice", "subscriber", true)
	bob := e.user("bob", "subscriber", true)
	carol := e.user("carol", "subscriber", false)
	html := `<!doctype html><html><head><title>Board</title></head><body>` + marker + `</body></html>`

	// Admin publication; subscribers cannot reach admin APIs.
	if code, out := e.upload(alice, html); code != 403 || errCode(out) != "creator_required" {
		t.Fatalf("subscriber upload: %d %v", code, out)
	}
	code, rel := e.upload(admin, html)
	if code != 201 {
		t.Fatalf("admin upload: %d %v", code, rel)
	}
	releaseID := rel["id"].(string)
	if code, _ := e.upload(admin, html); code != 409 {
		t.Fatalf("duplicate version should conflict: %d", code)
	}
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+releaseID+"/publish", nil); code != 200 {
		t.Fatalf("publish: %d %v", code, out)
	}
	if _, b, _ := e.do(&admin, "portal.test", "GET", "/api/studio/releases", nil, nil); !bytes.Contains(b, []byte(releaseID)) {
		t.Fatalf("admin release list misses the release: %s", b)
	}
	// CSRF and Origin are enforced on cookie-authenticated mutations.
	if code, _, _ := e.do(&admin, "portal.test", "POST", "/api/studio/releases/"+releaseID+"/publish", nil, map[string]string{"X-CSRF-Token": "nope"}); code != 403 {
		t.Fatalf("bad CSRF accepted: %d", code)
	}
	if code, _, _ := e.do(&admin, "portal.test", "POST", "/api/instances", strings.NewReader(`{}`), map[string]string{"Origin": "http://i-x.apps.test"}); code != 403 {
		t.Fatalf("app-origin request to portal accepted: %d", code)
	}

	// Catalog metadata never carries bytes; downloads withhold them.
	_, cat, _ := e.do(nil, "portal.test", "GET", "/api/catalog", nil, nil)
	if bytes.Contains(cat, []byte(marker)) || !bytes.Contains(cat, []byte("board")) {
		t.Fatalf("catalog leaked or missed release: %s", cat)
	}
	r, _ := e.store.Release(context.Background(), releaseID)
	var htmlFile store.ReleaseFile
	for _, f := range r.Files {
		if f.Role == "html" {
			htmlFile = f
		}
	}
	dl := "/api/releases/" + releaseID + "/files/" + htmlFile.ID + "/download"
	if code, b, _ := e.do(nil, "portal.test", "GET", dl, nil, nil); code != 401 || bytes.Contains(b, []byte(marker)) {
		t.Fatalf("anonymous download: %d %s", code, b)
	}
	if code, b, _ := e.do(&carol, "portal.test", "GET", dl, nil, nil); code != 403 || bytes.Contains(b, []byte(marker)) {
		t.Fatalf("unentitled download: %d", code)
	}
	code, b, hdr := e.do(&alice, "portal.test", "GET", dl, nil, nil)
	sum := sha256.Sum256(b)
	if code != 200 || hex.EncodeToString(sum[:]) != htmlFile.SHA256 || !strings.Contains(hdr.Get("Content-Disposition"), "attachment") {
		t.Fatalf("entitled download: %d %s", code, hdr)
	}

	// Instances: entitlement required; private by default.
	if code, out := e.portal(&carol, "POST", "/api/instances", map[string]string{"releaseId": releaseID}); code != 403 || errCode(out) != "plan_required" {
		t.Fatalf("unentitled create: %d %v", code, out)
	}
	_, instA := e.portal(&alice, "POST", "/api/instances", map[string]string{"releaseId": releaseID})
	_, instB := e.portal(&bob, "POST", "/api/instances", map[string]string{"releaseId": releaseID})
	if instA["id"] == nil || instB["id"] == nil || instA["origin"] == instB["origin"] {
		t.Fatalf("instances: %v %v", instA, instB)
	}
	if code, _ := e.portal(&bob, "GET", "/api/instances/"+instA["id"].(string), nil); code != 404 {
		t.Fatalf("bob sees alice's instance: %d", code)
	}
	if _, code, _ := e.launch(bob, instA); code != 404 {
		t.Fatalf("bob launched alice's instance: %d", code)
	}

	// Launch exchange: replay and wrong verifier fail.
	verifier, challenge := pkce()
	_, out := e.portal(&alice, "POST", "/api/instances/"+instA["id"].(string)+"/launch", map[string]string{"challenge": challenge})
	origin := out["origin"].(string)
	host := strings.TrimPrefix(origin, "http://")
	wrong, _ := json.Marshal(map[string]string{"code": out["code"].(string), "verifier": verifier + "x"})
	if code, _, _ := e.do(nil, host, "POST", "/api/launch/exchange", bytes.NewReader(wrong), map[string]string{"Origin": origin}); code != 401 {
		t.Fatalf("wrong verifier accepted: %d", code)
	}
	right, _ := json.Marshal(map[string]string{"code": out["code"].(string), "verifier": verifier})
	if code, _, _ := e.do(nil, host, "POST", "/api/launch/exchange", bytes.NewReader(right), map[string]string{"Origin": origin}); code != 401 {
		t.Fatalf("consumed code (after wrong verifier) replayed: %d", code)
	}
	_, out = e.portal(&alice, "POST", "/api/instances/"+instA["id"].(string)+"/launch", map[string]string{"challenge": challenge})
	right, _ = json.Marshal(map[string]string{"code": out["code"].(string), "verifier": verifier})
	otherHost := strings.TrimPrefix(instB["origin"].(string), "http://")
	if code, _, _ := e.do(nil, otherHost, "POST", "/api/launch/exchange", bytes.NewReader(right), map[string]string{"Origin": instB["origin"].(string)}); code != 401 {
		t.Fatalf("code redeemed on another instance origin: %d", code)
	}

	ga, code, why := e.launch(alice, instA)
	if code != 200 {
		t.Fatalf("alice launch: %d %s", code, why)
	}
	gb, _, _ := e.launch(bob, instB)
	// Grants are instance-scoped.
	if code, _, _ := e.do(nil, gb.host, "GET", "/page", nil, map[string]string{"Authorization": "Bearer " + ga.token}); code != 401 {
		t.Fatalf("grant crossed instances: %d", code)
	}
	if code, b, _ := e.do(nil, ga.host, "GET", "/page", nil, nil); code != 401 || bytes.Contains(b, []byte(marker)) {
		t.Fatalf("page without grant: %d", code)
	}
	code, page, _ := e.do(nil, ga.host, "GET", "/page", nil, map[string]string{"Authorization": "Bearer " + ga.token})
	if code != 200 || !bytes.Contains(page, []byte(marker)) || !bytes.Contains(page, []byte(alice.user.ID)) {
		t.Fatalf("page: %d", code)
	}
	// A grant is not a portal credential.
	if code, _, _ := e.do(nil, "portal.test", "GET", "/api/account", nil, map[string]string{"Authorization": "Bearer " + ga.token}); code != 401 {
		t.Fatalf("grant accepted by portal: %d", code)
	}

	// Documents are instance-scoped.
	va := e.view(ga, true)
	if code, out := e.rpc(va, "db.set", map[string]any{"path": "tasks/one", "body": map[string]any{"title": "alice task"}}); code != 200 {
		t.Fatalf("db.set: %d %v", code, out)
	}
	vb := e.view(gb, false)
	if _, out := e.rpc(vb, "db.get", map[string]any{"path": "tasks/one"}); out["exists"] == true {
		t.Fatal("instance B read instance A's document")
	}

	// Sharing: bob becomes a viewer of A.
	if code, out := e.portal(&alice, "POST", "/api/instances/"+instA["id"].(string)+"/members", map[string]string{"email": "bob@example.com", "role": "viewer"}); code != 200 {
		t.Fatalf("share: %d %v", code, out)
	}
	if code, _ := e.portal(&bob, "POST", "/api/instances/"+instA["id"].(string)+"/members", map[string]string{"email": "carol@example.com", "role": "editor"}); code != 403 {
		t.Fatalf("viewer shared the instance: %d", code)
	}
	gab, code, why := e.launch(bob, instA)
	if code != 200 {
		t.Fatalf("bob launch shared: %d %s", code, why)
	}
	vab := e.view(gab, true)
	if _, out := e.rpc(vab, "db.get", map[string]any{"path": "tasks/one"}); out["exists"] != true {
		t.Fatal("viewer cannot read shared document")
	}
	if code, out := e.rpc(vab, "db.set", map[string]any{"path": "tasks/two", "body": map[string]any{"x": 1}}); code != 403 || errCode(out) != "read_only" {
		t.Fatalf("viewer wrote shared document: %d %v", code, out)
	}
	private := "data/users/" + bob.user.ID + "/note"
	if code, out := e.rpc(vab, "db.set", map[string]any{"path": private, "body": map[string]any{"secret": "bob"}}); code != 200 {
		t.Fatalf("viewer private write: %d %v", code, out)
	}
	if _, out := e.rpc(va, "db.get", map[string]any{"path": private}); out["exists"] == true {
		t.Fatal("owner read a member's private document")
	}
	if code, out := e.rpc(va, "db.set", map[string]any{"path": private, "body": map[string]any{"x": 1}}); code != 403 {
		t.Fatalf("owner wrote member's private document: %d %v", code, out)
	}

	// Room identity: isMe compares users, sameTab compares views.
	e.rpc(va, "room.join", map[string]any{"room": "main"})
	e.rpc(vab, "room.join", map[string]any{"room": "main"})
	peers := waitFor(t, va, func(m map[string]any) bool {
		list, _ := m["peers"].([]any)
		return m["type"] == "room_peers" && m["room"] == "main" && len(list) == 2
	})
	for _, p := range peers["peers"].([]any) {
		p := p.(map[string]any)
		mine := p["by"] == alice.user.ID
		if p["isMe"] != mine || p["sameTab"] != (p["peer"] == va.id) {
			t.Fatalf("peer identity wrong: %v", p)
		}
	}
	e.rpc(vab, "room.emit", map[string]any{"room": "main", "topic": "wave", "data": "hi"})
	ev := waitFor(t, va, func(m map[string]any) bool { return m["type"] == "room_event" })
	if ev["by"] != bob.user.ID || ev["isMe"] != false {
		t.Fatalf("room event identity: %v", ev)
	}
	_, prof := e.rpc(va, "user.profiles", map[string]any{"ids": []string{bob.user.ID, carol.user.ID}})
	if bp, _ := prof[bob.user.ID].(map[string]any); bp == nil || bp["name"] != "bob" || bp["email"] != nil {
		t.Fatalf("co-member profile: %v", prof)
	}
	if cp, _ := prof[carol.user.ID].(map[string]any); cp == nil || cp["name"] != "" {
		t.Fatalf("non-member profile leaked: %v", prof)
	}

	// Removing bob revokes his access immediately.
	if code, _ := e.portal(&alice, "DELETE", "/api/instances/"+instA["id"].(string)+"/members/"+bob.user.ID, nil); code != 200 {
		t.Fatalf("remove member: %d", code)
	}
	if code, _ := e.rpc(vab, "db.get", map[string]any{"path": "tasks/one"}); code != 401 && code != 403 {
		t.Fatalf("removed member still reads: %d", code)
	}

	// Model allowance: one credit admits exactly one of two racing calls.
	doc := testDoc(1, settings.Model{Selector: "test/fake", Label: "Fake", Credits: 1})
	doc.Models.Tiers = map[string]string{"default": "test/fake"}
	doc.Allowance = settings.Allowance{PerUserConcurrency: 1, GlobalConcurrency: 6, MaxToolRounds: 3}
	_, err := e.store.SaveSettings(context.Background(), admin.user.ID, doc)
	if err != nil {
		t.Fatal(err)
	}
	va2 := e.view(ga, true)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i, v := range []*view{va, va2} {
		wg.Add(1)
		go func(i int, v *view) {
			defer wg.Done()
			code, _ := e.app(ga, "POST", "/api/views/"+v.id+"/calls", map[string]any{"callId": "race-" + string(rune('a'+i)), "input": "hello", "options": map[string]string{"modelTier": "default"}}, map[string]string{"X-Francois-View": v.id})
			codes <- code
		}(i, v)
	}
	wg.Wait()
	close(codes)
	got := map[int]int{}
	for c := range codes {
		got[c]++
	}
	if got[202] != 1 || got[402]+got[429] != 1 {
		t.Fatalf("allowance race: %v", got)
	}
	if code, out := e.app(ga, "POST", "/api/views/"+va.id+"/calls", map[string]any{"callId": "m1", "input": "hi", "options": map[string]string{"modelTier": "default", "model": "other/model"}}, map[string]string{"X-Francois-View": va.id}); code != 403 || errCode(out) != "model_not_allowed" {
		t.Fatalf("disallowed model: %d %v", code, out)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		calls, _ := e.store.ModelCalls(context.Background(), alice.user.ID, 10)
		if len(calls) == 1 && calls[0].State == "completed" {
			if calls[0].OutputTokens == 0 || calls[0].Credits != 1 {
				t.Fatalf("usage not recorded: %+v", calls[0])
			}
			var artifact string
			if err := e.store.Pool.QueryRow(context.Background(), `SELECT artifact_id FROM model_calls WHERE id=$1`, calls[0].ID).Scan(&artifact); err != nil || artifact != "board" {
				t.Fatalf("runtime artifact attribution: %q %v", artifact, err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("call not settled: %+v", calls)
		}
		time.Sleep(50 * time.Millisecond)
	}
	allow, _ := e.store.AllowanceFor(context.Background(), alice.user.ID, 1, 0)
	if allow.Used != 1 || allow.Reserved != 0 || allow.Remaining != 0 {
		t.Fatalf("ledger: %+v", allow)
	}

	// Logout revokes derived grants.
	if code, _ := e.portal(&alice, "POST", "/auth/logout", nil); code != 200 {
		t.Fatalf("logout: %d", code)
	}
	if code, _ := e.rpc(va2, "db.get", map[string]any{"path": "tasks/one"}); code != 401 {
		t.Fatalf("grant survived logout: %d", code)
	}
}

// TestSecondSite serves one catalog under two portal origins: absolute URLs,
// Origin checks, launch codes and framing follow the site a request arrives
// on.
func TestSecondSite(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	alice := e.user("alice", "subscriber", true)
	_, rel := e.upload(admin, `<!doctype html><html><head><title>Board</title></head><body>`+marker+`</body></html>`)
	releaseID := rel["id"].(string)
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+releaseID+"/publish", nil); code != 200 {
		t.Fatalf("publish: %d %v", code, out)
	}

	code, b, hdr := e.do(nil, "boutique.test", "GET", "/api/session", nil, nil)
	if code != 200 || !bytes.Contains(b, []byte(`"appDomain":"boutique.test"`)) || !strings.Contains(hdr.Get("Content-Security-Policy"), "frame-src http://*.boutique.test;") {
		t.Fatalf("second site session: %d %s %s", code, b, hdr.Get("Content-Security-Policy"))
	}

	// www.<portal> redirects to the portal, keeping the path.
	req, _ := http.NewRequest("GET", e.ts.URL+"/account?x=1", nil)
	req.Host = "www.boutique.test"
	noFollow := *e.client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := noFollow.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != 308 || res.Header.Get("Location") != "http://boutique.test/account?x=1" {
		t.Fatalf("www redirect: %d %s", res.StatusCode, res.Header.Get("Location"))
	}

	// The other site's origin is cross-origin for this portal.
	if code, _, _ := e.do(&alice, "boutique.test", "POST", "/api/instances", strings.NewReader(`{}`), map[string]string{"Origin": "http://portal.test"}); code != 403 {
		t.Fatalf("other site's Origin accepted: %d", code)
	}

	// An instance created on one site is reachable under both.
	_, inst := e.portal(&alice, "POST", "/api/instances", map[string]string{"releaseId": releaseID})
	primary, _ := inst["origin"].(string)
	label := strings.TrimSuffix(strings.TrimPrefix(primary, "http://"), ".apps.test")
	_, again := e.portalAt("boutique.test", &alice, "GET", "/api/instances/"+inst["id"].(string), nil)
	if again["origin"] != "http://"+label+".boutique.test" {
		t.Fatalf("second site origin: %v (primary %s)", again["origin"], primary)
	}

	g, code, why := e.launchAt("boutique.test", alice, inst)
	if code != 200 || g.origin != "http://"+label+".boutique.test" {
		t.Fatalf("launch through second site: %d %s %v", code, why, g)
	}
	code, b, hdr = e.do(nil, g.host, "GET", "/launch", nil, nil)
	if code != 200 || hdr.Get("Content-Security-Policy") != "frame-ancestors http://boutique.test" || !bytes.Contains(b, []byte(`data-portal="http://boutique.test"`)) {
		t.Fatalf("launch page framing: %d %s", code, hdr.Get("Content-Security-Policy"))
	}
	code, b, _ = e.do(nil, g.host, "GET", "/page", nil, map[string]string{"Authorization": "Bearer " + g.token})
	if code != 200 || !bytes.Contains(b, []byte(`"portalOrigin":"http://boutique.test"`)) {
		t.Fatalf("page boot: %d", code)
	}

	// A launch code issued through one site is bound to that site's
	// instance origin.
	verifier, challenge := pkce()
	_, out := e.portalAt("boutique.test", &alice, "POST", "/api/instances/"+inst["id"].(string)+"/launch", map[string]string{"challenge": challenge})
	raw, _ := json.Marshal(map[string]string{"code": out["code"].(string), "verifier": verifier})
	if code, _, _ := e.do(nil, label+".apps.test", "POST", "/api/launch/exchange", bytes.NewReader(raw), map[string]string{"Origin": primary}); code != 401 {
		t.Fatalf("code redeemed on the other site's origin: %d", code)
	}
}

// TestLegalPages serves the privacy policy and terms as plain HTML under the
// site the request arrived on.
func TestLegalPages(t *testing.T) {
	e := setup(t)
	for _, tc := range []struct{ host, path, want string }{
		{"portal.test", "/privacy", "Privacy policy · portal.test"},
		{"boutique.test", "/privacy", "Privacy policy · boutique.test"},
		{"boutique.test", "/terms", "Terms of service · boutique.test"},
	} {
		code, b, hdr := e.do(nil, tc.host, "GET", tc.path, nil, nil)
		if code != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "text/html") || !bytes.Contains(b, []byte(tc.want)) || !bytes.Contains(b, []byte("<h2>Contact</h2>")) {
			t.Fatalf("%s%s: %d %q", tc.host, tc.path, code, hdr.Get("Content-Type"))
		}
	}
	if code, b, _ := e.do(nil, "portal.test", "GET", "/terms", nil, nil); code != 200 || !bytes.Contains(b, []byte("Accounts are by invitation only")) {
		t.Fatalf("terms body: %d", code)
	}
}
