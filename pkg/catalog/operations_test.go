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
	d := testDoc(100, settings.Model{Selector: "test/fake", Credits: 1})
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
			i, err := e.store.OpenWorkspace(ctx, admin.user.ID, release, "Board", 10, true)
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

func TestCombinedFeedAndMinimalPublicStream(t *testing.T) {
	e := setup(t)
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
		if code != 200 || !strings.Contains(string(page), "A new note") || !strings.Contains(string(page), "/apps/board") {
			t.Fatalf("merged page %s %d %s", path, code, page)
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
