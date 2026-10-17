package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func enableJournal(t *testing.T, e *env) {
	t.Helper()
	rev, err := e.store.ActiveSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	rev.Document.Storefront.JournalEnabled = true
	if _, err = e.store.SaveSettings(t.Context(), "", rev.Document); err != nil {
		t.Fatal(err)
	}
}

func TestJournalHiddenByDefaultAndAdminToggle(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("writer", "admin", false)
	subscriber := e.user("reader", "subscriber", false)
	other := e.user("other", "creator", false)
	e.publishArtifact(admin, "board", "")
	creator, err := e.store.CreateCreator(ctx, store.Creator{Slug: "writer", Name: "Writer", UserID: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	post, err := e.store.CreatePost(ctx, store.Post{Slug: "hidden-note", Title: "Hidden journal note", Body: "Blog body", AuthorID: creator.ID, CreatedBy: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.SetPostState(ctx, post.ID, "published"); err != nil {
		t.Fatal(err)
	}
	draft, err := e.store.CreatePost(ctx, store.Post{Slug: "draft-note", Title: "Still a draft", Body: "Draft body", CreatedBy: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	// A pre-setting saved document is also opt-out by default. No migration.
	if _, err = e.store.Pool.Exec(ctx, `UPDATE platform_settings SET document=document #- '{storefront,journalEnabled}'::text[]`); err != nil {
		t.Fatal(err)
	}
	rev, err := e.store.ActiveSettings(ctx)
	if err != nil || rev.Document.Storefront.JournalEnabled {
		t.Fatalf("old saved settings must hide blog: %v", err)
	}
	check := func(enabled bool) {
		t.Helper()
		for _, path := range []string{"/journal", "/journal/hidden-note", "/journal/feed.xml"} {
			code, body, _ := e.do(nil, "portal.test", "GET", path, nil, nil)
			want := 404
			if enabled {
				want = 200
			}
			if code != want {
				t.Fatalf("%s enabled=%v got %d", path, enabled, code)
			}
			if !enabled && strings.Contains(string(body), post.Title) {
				t.Fatalf("hidden post leaked through %s", path)
			}
		}
		for _, path := range []string{"/", "/apps/board", "/creators/writer"} {
			code, body, _ := e.do(nil, "portal.test", "GET", path, nil, nil)
			if code != 200 {
				t.Fatalf("artifact browsing affected: %s %d", path, code)
			}
			if strings.Contains(string(body), `href="/journal"`) != enabled {
				t.Fatalf("journal navigation disagrees on %s", path)
			}
			if path == "/creators/writer" && strings.Contains(string(body), post.Title) != enabled {
				t.Fatal("creator page disagrees")
			}
		}
		code, body, h := e.do(nil, "portal.test", "GET", "/feed.xml", nil, nil)
		if code != 200 || !strings.Contains(string(body), "/apps/board") || strings.Contains(string(body), post.Title) != enabled || strings.Contains(string(body), draft.Title) {
			t.Fatalf("combined feed visibility %d %s", code, body)
		}
		if h.Get("Cache-Control") != "no-cache" {
			t.Fatal("feed must revalidate toggle")
		}
		_, session := e.portal(nil, "GET", "/api/session", nil)
		if session["journalEnabled"] != enabled {
			t.Fatal("SPA journal setting disagrees")
		}
	}
	check(false)
	for _, user := range []*tactor{nil, &subscriber, &other} {
		if code, _, _ := e.do(user, "portal.test", "GET", "/journal/hidden-note", nil, nil); code != 404 {
			t.Fatal("hidden post accessible to a noneditor")
		}
	}
	code, body, h := e.do(&admin, "portal.test", "GET", "/journal/hidden-note", nil, nil)
	if code != 200 || !strings.Contains(string(body), "blog is hidden from visitors") || h.Get("Cache-Control") != "private, no-store" || h.Get("X-Robots-Tag") == "" {
		t.Fatalf("editor preview: %d %v", code, h)
	}
	if code, _ := e.portal(&admin, "GET", "/api/studio/posts", nil); code != 200 {
		t.Fatal("Studio post editing unavailable")
	}
	set := func(enabled bool) {
		t.Helper()
		_, raw, headers := e.do(&admin, "portal.test", "GET", "/api/admin/settings", nil, nil)
		var current struct {
			Active struct{ Document map[string]any }
		}
		if err := json.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
		current.Active.Document["storefront"].(map[string]any)["journalEnabled"] = enabled
		encoded, _ := json.Marshal(current.Active.Document)
		if code, _, _ := e.do(&subscriber, "portal.test", "PATCH", "/api/admin/settings", strings.NewReader(string(encoded)), map[string]string{"If-Match": headers.Get("ETag")}); code != 403 {
			t.Fatalf("subscriber edited admin setting: %d", code)
		}
		if code, body, _ := e.do(&admin, "portal.test", "PATCH", "/api/admin/settings", strings.NewReader(string(encoded)), map[string]string{"If-Match": headers.Get("ETag")}); code != 200 {
			t.Fatalf("admin setting: %d %s", code, body)
		}
	}
	set(true)
	check(true)
	if code, _, _ := e.do(nil, "portal.test", "GET", "/journal/draft-note", nil, nil); code != 404 {
		t.Fatal("enabling blog exposed drafts")
	}
	set(false)
	check(false)
	if preserved, err := e.store.Post(ctx, post.ID); err != nil || preserved.State != "published" {
		t.Fatal("hiding blog changed post data")
	}
}
