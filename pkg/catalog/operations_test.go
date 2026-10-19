package catalog

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

func TestOperationsAffectOpenViewsAndPreserveData(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("operator", "admin", false)
	release := e.publishArtifact(admin, "board", "")
	if _, err := e.store.Pool.Exec(ctx, `UPDATE releases SET manifest=manifest || '{"assets":{},"comments":{}}'::jsonb WHERE id=$1`, release); err != nil {
		t.Fatal(err)
	}
	code, inst := e.portal(&admin, "POST", "/api/instances", map[string]string{"releaseId": release})
	if code != 201 {
		t.Fatalf("create: %d %v", code, inst)
	}
	g, code, _ := e.launch(admin, inst)
	if code != 200 {
		t.Fatalf("launch %d", code)
	}
	v := e.view(g, true)
	if code, out := e.rpc(v, "db.set", map[string]any{"path": "notes/one", "body": map[string]any{"text": "keep me"}}); code != 200 {
		t.Fatalf("seed %d %v", code, out)
	}
	_, out := e.rpc(v, "comments.create", map[string]any{"anchor": map[string]string{"id": "one"}, "text": "hello"})
	thread := out["threadId"]
	asset, err := e.store.PutAsset(ctx, inst["id"].(string), admin.user.ID, "image/png", []byte("bytes"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	_, out = e.rpc(v, "assets.list", nil)
	blob := out["assets"].([]any)[0].(map[string]any)["url"].(string)
	d := testDoc(100, settings.Model{Selector: "test/fake"})
	d.Models.Tiers["default"] = "test/fake"
	save := func() {
		t.Helper()
		if _, err := e.store.SaveSettings(ctx, "", d); err != nil {
			t.Fatal(err)
		}
	}
	d.Operations.SamplingEnabled = false
	d.Operations.AcceptNewVisits = false
	d.Operations.StorageMode = "read-only"
	d.Operations.Notice = "Maintenance <soon>"
	save()
	if code, out := e.call(g, v, "paused"); code != 503 || errCode(out) != "sampling_paused" {
		t.Fatalf("sample %d %v", code, out)
	}
	if code, out := e.portal(&admin, "POST", "/api/instances/open", map[string]string{"releaseId": release}); code != 201 || out["id"] != inst["id"] {
		t.Fatalf("resume %d %v", code, out)
	}
	other := e.user("other", "subscriber", true)
	if code, out := e.portal(&other, "POST", "/api/instances/open", map[string]string{"releaseId": release}); code != 503 || errCode(out) != "visits_paused" {
		t.Fatalf("new visit %d %v", code, out)
	}
	if code, out := e.portal(&admin, "POST", "/api/instances", map[string]string{"releaseId": release}); code != 503 || errCode(out) != "visits_paused" {
		t.Fatalf("explicit create %d %v", code, out)
	}
	for _, method := range []string{"db.set", "db.update", "db.delete", "db.acquire", "comments.create", "comments.reply", "comments.resolve", "comments.delete", "assets.delete"} {
		code, out := e.rpc(v, method, map[string]any{"path": "notes/one", "body": map[string]any{}, "threadId": thread, "id": asset.ID})
		if code != 503 || errCode(out) != "storage_read_only" {
			t.Errorf("%s: %d %v", method, code, out)
		}
	}
	if code, out := e.rpc(v, "db.get", map[string]string{"path": "notes/one"}); code != 200 || out["exists"] != true {
		t.Fatalf("read-only read %d %v", code, out)
	}
	if code, _, _ := e.do(nil, g.host, "POST", "/api/assets", strings.NewReader("new"), map[string]string{"Authorization": "Bearer " + g.token, "Origin": g.origin, "Content-Type": "image/png"}); code != 503 {
		t.Fatalf("upload during pause %d", code)
	}
	if code, _, _ := e.do(nil, g.host, "GET", blob, nil, nil); code != 200 {
		t.Fatalf("read-only blob %d", code)
	}
	d.Operations.StorageMode = "off"
	save()
	if code, out := e.rpc(v, "assets.list", nil); code != 503 || errCode(out) != "storage_unavailable" {
		t.Fatalf("off %d %v", code, out)
	}
	if code, _, _ := e.do(nil, g.host, "GET", blob, nil, nil); code != 503 {
		t.Fatalf("old signed blob bypassed pause: %d", code)
	}
	d.Operations = settings.Operations{AcceptNewVisits: true, SamplingEnabled: true, StorageMode: "read-write"}
	save()
	if code, out := e.rpc(v, "db.get", map[string]string{"path": "notes/one"}); code != 200 || out["exists"] != true {
		t.Fatalf("restored %d %v", code, out)
	}
	e.view(g, false)
	e.view(g, false)
	if code, out := e.app(g, "POST", "/api/views", nil, nil); code != 429 || errCode(out) != "view_limit" {
		t.Fatalf("view cap %d %v", code, out)
	}
	calls, err := e.store.ModelCalls(ctx, admin.user.ID, 10)
	if err != nil || len(calls) != 0 {
		t.Fatalf("paused request reserved usage: %v %v", calls, err)
	}
}

func TestStorageQuotaSerializesGrowthAndAllowsCleanup(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("owner", "admin", false)
	release := e.publishArtifact(admin, "board", "")
	inst, err := e.store.CreateInstance(ctx, admin.user.ID, release, "Test", "workspace", 10)
	if err != nil {
		t.Fatal(err)
	}
	d := testDoc(100)
	d.Limits.StorageBytesPerInstance = 100
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- e.store.WriteDocument(ctx, inst.ID, admin.user.ID, fmt.Sprintf("notes/%d", i), "set", map[string]any{"s": strings.Repeat("x", 60)}, 500)
		}()
	}
	wg.Wait()
	close(results)
	successes, limited := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, store.ErrLimit) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || limited != 1 {
		t.Fatalf("quota race success=%d limited=%d", successes, limited)
	}
	var path string
	if err = e.store.Pool.QueryRow(ctx, `SELECT path FROM instance_documents WHERE instance_id=$1`, inst.ID).Scan(&path); err != nil {
		t.Fatal(err)
	}
	d.Limits.StorageBytesPerInstance = 20
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	if err = e.store.WriteDocument(ctx, inst.ID, admin.user.ID, path, "set", map[string]any{}, 500); err != nil {
		t.Fatalf("shrink above cap %v", err)
	}
	if _, err = e.store.PutAsset(ctx, inst.ID, admin.user.ID, "image/png", []byte("1234567890"), 1024); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.store.CreateThread(ctx, inst.ID, admin.user.ID, []byte(`{}`), "too much text"); !errors.Is(err, store.ErrLimit) {
		t.Fatalf("comment bytes ignored: %v", err)
	}
	if _, err = e.store.AcquireLease(ctx, inst.ID, admin.user.ID, "notes/lock", "holder", 1000, strings.Repeat("x", 30)); !errors.Is(err, store.ErrLimit) {
		t.Fatalf("lease bytes ignored: %v", err)
	}
	if err = e.store.WriteDocument(ctx, inst.ID, admin.user.ID, path, "delete", nil, 500); err != nil {
		t.Fatalf("cleanup %v", err)
	}
	// An expired/missing policy gate must not silently allow direct writes.
	d.Operations.StorageMode = "read-only"
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	err = e.store.WriteDocument(ctx, inst.ID, admin.user.ID, "notes/no", "set", map[string]any{}, 500)
	var paused *store.StoragePaused
	if !errors.As(err, &paused) {
		t.Fatalf("direct write bypassed pause: %v", err)
	}
}

func TestOpenWorkspaceAndTourResumeUnderConcurrency(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("owner", "admin", false)
	release := e.publishArtifact(admin, "board", "")
	ids := make(chan string, 10)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i, err := e.store.OpenWorkspace(ctx, admin.user.ID, release, "Board", 10, true, true)
			if err != nil {
				ids <- "error:" + err.Error()
				return
			}
			ids <- i.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id == "" {
			id = got
		}
		if got != id || strings.HasPrefix(got, "error:") {
			t.Fatalf("duplicate workspace %q %q", id, got)
		}
	}
	tour, err := e.store.CreateDemo(ctx, admin.user.ID, release, "Tour", time.Now().Add(time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.store.CreateDemo(ctx, admin.user.ID, release, "Tour", time.Now().Add(2*time.Hour), 10)
	if err != nil || again.ID != tour.ID || !again.ExpiresAt.Equal(*tour.ExpiresAt) {
		t.Fatalf("tour expiry reset: %+v %v", again, err)
	}
	d := testDoc(100)
	d.Demo.Enabled = true
	d.Demo.PerUserPerHour = 1
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]bool{"demoEnabled": true}); code != 200 {
		t.Fatalf("demo enabled %d %v", code, out)
	}
	d.Operations.AcceptNewVisits = false
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		code, out := e.portal(&admin, "POST", "/api/demos", map[string]string{"artifactId": "board"})
		if code != 201 || out["id"] != tour.ID {
			t.Fatalf("resume paused tour %d %v", code, out)
		}
	}
	// A refresh does not spend a new start; expiry does require a new admission.
	var raw []byte
	_ = e.store.Pool.QueryRow(ctx, `SELECT to_jsonb(expires_at) FROM instances WHERE id=$1`, tour.ID).Scan(&raw)
	var expiry string
	_ = json.Unmarshal(raw, &expiry)
	if expiry == "" {
		t.Fatal("tour expiry missing")
	}
}

func TestCombinedFeedAndSeparatePublicPages(t *testing.T) {
	e := setup(t)
	enableJournal(t, e)
	ctx := t.Context()
	admin := e.user("writer", "admin", false)
	release := e.publishArtifact(admin, "board", "")
	p, err := e.store.CreatePost(ctx, store.Post{Slug: "hello", Title: "A new note", Summary: "A short note", Body: "Private draft body", CreatedBy: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, before, _ := e.do(nil, "portal.test", "GET", "/feed.xml", nil, nil)
	if strings.Contains(string(before), p.Title) {
		t.Fatal("draft post leaked")
	}
	if _, err = e.store.SetPostState(ctx, p.ID, "published"); err != nil {
		t.Fatal(err)
	}
	code, body, h := e.do(nil, "boutique.test", "GET", "/feed.xml", nil, nil)
	var feed rssDoc
	if err = xml.Unmarshal(body, &feed); err != nil || code != 200 || len(feed.Channel.Items) != 2 {
		t.Fatalf("combined %d %s %v", code, body, err)
	}
	if !strings.Contains(h.Get("Content-Type"), "application/rss+xml") || feed.Channel.Items[0].Link != "http://boutique.test/journal/hello" {
		t.Fatalf("feed order/origin %v", feed)
	}
	for _, path := range []string{"/", "/apps", "/journal"} {
		code, page, _ := e.do(&admin, "portal.test", "GET", path, nil, nil)
		hasPost := strings.Contains(string(page), "A new note")
		hasApp := strings.Contains(string(page), "/apps/board")
		if code != 200 || (path == "/journal" && (!hasPost || hasApp)) || (path != "/journal" && (hasPost || !hasApp)) {
			t.Fatalf("separate page %s %d %s", path, code, page)
		}
		for _, removed := range []string{`class="hero"`, `>Made by<`, `>Instances</a>`, `>Pricing</a>`} {
			if strings.Contains(string(page), removed) {
				t.Errorf("%s exposes %s", path, removed)
			}
		}
	}
	if _, err = e.store.Pool.Exec(ctx, `UPDATE catalog_artifacts SET visibility='hidden' WHERE id=(SELECT artifact_id FROM releases WHERE id=$1)`, release); err != nil {
		t.Fatal(err)
	}
	_, body, _ = e.do(nil, "portal.test", "GET", "/feed.xml", nil, nil)
	if strings.Contains(string(body), "/apps/board") {
		t.Fatal("hidden artifact leaked in combined feed")
	}
}

func TestTourStartsFollowUserAcrossAddresses(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	e.srv.Config.TrustProxyHeaders = true
	admin := e.user("tour-admin", "admin", false)
	visitor := e.user("tour-user", "subscriber", false)
	for _, id := range []string{"board", "second"} {
		e.publishArtifact(admin, id, "")
		if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/"+id, map[string]bool{"demoEnabled": true}); code != 200 {
			t.Fatalf("enable %d %v", code, out)
		}
	}
	d := testDoc(100)
	d.Demo.Enabled = true
	d.Demo.PerUserPerHour = 1
	d.Demo.PerAddressPerHour = 10
	if _, err := e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	start := func(artifact, address string) (int, map[string]any) {
		code, body, _ := e.do(&visitor, "portal.test", "POST", "/api/demos", strings.NewReader(`{"artifactId":"`+artifact+`"}`), map[string]string{"X-Real-Ip": address})
		var out map[string]any
		_ = json.Unmarshal(body, &out)
		return code, out
	}
	code, first := start("board", "10.1.0.1")
	if code != 201 {
		t.Fatalf("start %d %v", code, first)
	}
	if code, out := start("second", "10.1.0.2"); code != 429 || errCode(out) != "rate_limited" {
		t.Fatalf("changed address bypassed user cap %d %v", code, out)
	}
	if code, out := start("board", "10.1.0.3"); code != 201 || out["id"] != first["id"] || out["expiresAt"] != first["expiresAt"] {
		t.Fatalf("resume spent a start or reset expiry %d %v", code, out)
	}
}

func TestDocumentCountIsAtomic(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	owner := e.user("document-owner", "admin", false)
	release := e.publishArtifact(owner, "board", "")
	inst, err := e.store.CreateInstance(ctx, owner.user.ID, release, "Count", "workspace", 10)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- e.store.WriteDocument(ctx, inst.ID, owner.user.ID, fmt.Sprintf("count/%d", i), "set", map[string]any{}, 1)
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "document limit") {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("document admission count %d", success)
	}
}

// A forbidden withdrawn pin must recover without losing data or needing a
// second workspace, including while creation is paused and at the count cap.
func TestOpenWorkspaceRecoversWithdrawnPin(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("pin-owner", "admin", false)
	old := e.publishArtifact(admin, "board", "")
	code, original := e.portal(&admin, "POST", "/api/instances", map[string]string{"releaseId": old})
	if code != 201 {
		t.Fatalf("create %d %v", code, original)
	}
	id := original["id"].(string)
	if err := e.store.WriteDocument(ctx, id, admin.user.ID, "notes/keep", "set", map[string]any{"text": "saved"}, 500); err != nil {
		t.Fatal(err)
	}
	code, next := e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": "board", "title": "Board", "version": "2.0.0", "manifestJson": `{"db":{}}`}, "html", "a.html", []byte(testHTML))
	if code != 201 {
		t.Fatalf("upload %d %v", code, next)
	}
	current := next["id"].(string)
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+current+"/publish", nil); code != 200 {
		t.Fatalf("publish %d %v", code, out)
	}
	if _, err := e.store.Pool.Exec(ctx, `UPDATE releases SET state='withdrawn' WHERE id=$1`, old); err != nil {
		t.Fatal(err)
	}
	kept, err := e.store.OpenWorkspace(ctx, admin.user.ID, current, "Board", 1, false, true)
	if err != nil || kept.ID != id || kept.ReleaseID != old {
		t.Fatalf("permitted pin changed: %+v %v", kept, err)
	}
	d := testDoc(100)
	d.Publishing.AllowPinnedAfterWithdraw = false
	d.Operations.AcceptNewVisits = false
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	recovered, err := e.store.OpenWorkspace(ctx, admin.user.ID, current, "Board", 1, false, false)
	if err != nil || recovered.ID != id || recovered.ReleaseID != current {
		t.Fatalf("withdrawn recovery: %+v %v", recovered, err)
	}
	if code, out := e.portal(&admin, "POST", "/api/instances/open", map[string]string{"releaseId": current}); code != 201 || out["id"] != id {
		t.Fatalf("open %d %v", code, out)
	}
	if _, code, out := e.launch(admin, original); code != 200 {
		t.Fatalf("recovered launch %d %v", code, out)
	}
	var body string
	if err = e.store.Pool.QueryRow(ctx, `SELECT body->>'text' FROM instance_documents WHERE instance_id=$1 AND path='notes/keep'`, id).Scan(&body); err != nil || body != "saved" {
		t.Fatalf("saved document lost: %q %v", body, err)
	}
	var count int
	if err = e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM instances WHERE owner_user_id=$1 AND kind='workspace'`, admin.user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("workspace count %d %v", count, err)
	}
}

func TestConcurrentTourRequestsSpendOneStart(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("tour-admin", "admin", false)
	visitor := e.user("tour-visitor", "subscriber", false)
	for _, id := range []string{"board", "second", "third"} {
		e.publishArtifact(admin, id, "")
		if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/"+id, map[string]bool{"demoEnabled": true}); code != 200 {
			t.Fatalf("enable %d %v", code, out)
		}
	}
	d := testDoc(100)
	d.Demo.Enabled = true
	d.Demo.PerUserPerHour = 2
	d.Demo.PerAddressPerHour = 2
	if _, err := e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	// Hold creation until the concurrent requests are in flight, so an initial
	// lookup cannot happen to observe an already-committed tour by timing alone.
	tx, err := e.store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('demos'))`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		code int
		out  map[string]any
	}
	results := make(chan result, 6)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			code, out := e.portal(&visitor, "POST", "/api/demos", map[string]string{"artifactId": "board"})
			results <- result{code, out}
		}()
	}
	close(start)
	time.Sleep(100 * time.Millisecond)
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	close(results)
	var id any
	for r := range results {
		if r.code != 201 {
			t.Fatalf("concurrent resume %d %v", r.code, r.out)
		}
		if id == nil {
			id = r.out["id"]
		}
		if r.out["id"] != id {
			t.Fatalf("duplicate tour %v %v", id, r.out)
		}
	}
	if code, out := e.portal(&visitor, "POST", "/api/demos", map[string]string{"artifactId": "second"}); code != 201 {
		t.Fatalf("second actual tour blocked: %d %v", code, out)
	}
	if code, out := e.portal(&visitor, "POST", "/api/demos", map[string]string{"artifactId": "third"}); code != 429 || errCode(out) != "rate_limited" {
		t.Fatalf("third tour exceeded cap: %d %v", code, out)
	}
}

func TestFailedTourCreationDoesNotSpendStart(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("tour-admin", "admin", false)
	visitor := e.user("tour-visitor", "subscriber", false)
	e.publishArtifact(admin, "board", "")
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]bool{"demoEnabled": true}); code != 200 {
		t.Fatalf("enable %d %v", code, out)
	}
	d := testDoc(100)
	d.Demo.Enabled = true
	d.Demo.PerUserPerHour = 1
	d.Demo.PerAddressPerHour = 1
	d.Demo.MaxActive = 0
	if _, err := e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	if code, out := e.portal(&visitor, "POST", "/api/demos", map[string]string{"artifactId": "board"}); code != 429 || errCode(out) != "demo_limit" {
		t.Fatalf("capacity rejection %d %v", code, out)
	}
	d.Demo.MaxActive = 20
	if _, err := e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	if code, out := e.portal(&visitor, "POST", "/api/demos", map[string]string{"artifactId": "board"}); code != 201 {
		t.Fatalf("failed creation spent start: %d %v", code, out)
	}
}

func TestExpiredLeasesReleaseStorageQuota(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	owner := e.user("lease-owner", "admin", false)
	release := e.publishArtifact(owner, "board", "")
	inst, err := e.store.CreateInstance(ctx, owner.user.ID, release, "Board", "workspace", 1)
	if err != nil {
		t.Fatal(err)
	}
	d := testDoc(100)
	d.Limits.StorageBytesPerInstance = 64
	if _, err = e.store.SaveSettings(ctx, "", d); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.store.AcquireLease(ctx, inst.ID, owner.user.ID, "notes/old", "holder", 60000, strings.Repeat("x", 40)); err != nil || !ok {
		t.Fatalf("lease %v %v", ok, err)
	}
	body := map[string]any{"s": strings.Repeat("y", 40)}
	if err = e.store.WriteDocument(ctx, inst.ID, owner.user.ID, "notes/saved", "set", body, 500); !errors.Is(err, store.ErrLimit) {
		t.Fatalf("live lease not counted: %v", err)
	}
	if _, err = e.store.Pool.Exec(ctx, `UPDATE instance_leases SET expires_at=now()-interval '1 second' WHERE instance_id=$1`, inst.ID); err != nil {
		t.Fatal(err)
	}
	if err = e.store.WriteDocument(ctx, inst.ID, owner.user.ID, "notes/saved", "set", body, 500); err != nil {
		t.Fatalf("expired lease retained quota: %v", err)
	}
	if ok, err := e.store.AcquireLease(ctx, inst.ID, owner.user.ID, "notes/new", "holder", 60000, "z"); err != nil || !ok {
		t.Fatalf("new lease %v %v", ok, err)
	}
	var stale int
	if err = e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM instance_leases WHERE instance_id=$1 AND path='notes/old'`, inst.ID).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("expired lease not purged: %d %v", stale, err)
	}
}
