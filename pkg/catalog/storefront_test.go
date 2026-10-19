package catalog

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// TestContentStore covers the storefront tables added by migration 004.
func TestContentStore(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	st := e.store
	admin := e.user("ada", "admin", false)

	// Uploading a release creates the artifact once; a later release keeps
	// the metadata an editor changed, and needs no title.
	if code, out := e.upload(admin, testHTML); code != 201 {
		t.Fatalf("upload %d %v", code, out)
	}
	a, err := st.Artifact(ctx, "board")
	if err != nil || a.Title != "Board" || a.Visibility != "listed" || a.Page.Gallery == nil {
		t.Fatalf("artifact %+v %v", a, err)
	}
	a.Title, a.Featured, a.DemoEnabled = "Board, renamed", true, true
	if _, err = st.UpdateArtifact(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.srv.Publisher.CreateDraft(ctx, admin.user.ID, uploadOf("board", "", "1.1.0")); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if a, _ = st.Artifact(ctx, "board"); a.Title != "Board, renamed" || !a.Featured || !a.DemoEnabled {
		t.Fatalf("metadata changed by upload: %+v", a)
	}
	if _, err = st.CreateArtifact(ctx, store.Artifact{ID: "board", Title: "dup"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate artifact: %v", err)
	}

	// Page draft → publish → discard.
	if err = st.PublishPageDraft(ctx, "board"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("publish without draft: %v", err)
	}
	if err = st.SavePageDraft(ctx, "board", store.Page{Body: "# Draft"}); err != nil {
		t.Fatal(err)
	}
	if a, _ = st.Artifact(ctx, "board"); a.Page.Body != "" || a.PageDraft == nil || a.PageDraft.Body != "# Draft" {
		t.Fatalf("draft %+v", a)
	}
	if err = st.PublishPageDraft(ctx, "board"); err != nil {
		t.Fatal(err)
	}
	if a, _ = st.Artifact(ctx, "board"); a.Page.Body != "# Draft" || a.PageDraft != nil {
		t.Fatalf("published %+v", a)
	}
	if err = st.SavePageDraft(ctx, "missing", store.Page{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("draft for missing artifact: %v", err)
	}

	// Media references block deletion.
	m, err := st.CreateMedia(ctx, store.Media{SHA256: "00", StorageKey: "blobs/00/" + zeros, MediaType: "image/png", Size: 1, UploadedBy: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SavePageDraft(ctx, "board", store.Page{Body: "see ![x](/media/" + m.ID + ")"}); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteMedia(ctx, m.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("delete referenced from draft body: %v", err)
	}
	if err = st.SavePageDraft(ctx, "board", store.Page{Gallery: []string{m.ID}}); err != nil {
		t.Fatal(err)
	}
	if used, _ := st.MediaReferenced(ctx, m.ID); !used {
		t.Fatal("gallery reference not found")
	}
	if err = st.DiscardPageDraft(ctx, "board"); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteMedia(ctx, m.ID); err != nil {
		t.Fatalf("delete unreferenced: %v", err)
	}

	// Creators, credits and the link between a creator and an account.
	cre := e.user("cleo", "", false)
	c, err := st.CreateCreator(ctx, store.Creator{Slug: "cleo", Name: "Cleo", UserID: cre.user.ID, Links: []store.Link{{Label: "site", URL: "https://example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateCreator(ctx, store.Creator{Slug: "cleo", Name: "Other"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate slug: %v", err)
	}
	other, _ := st.CreateCreator(ctx, store.Creator{Slug: "otto", Name: "Otto"})
	if ok, _ := st.IsArtifactCreator(ctx, "board", cre.user.ID); ok {
		t.Fatal("creator before credit")
	}
	if err = st.SetArtifactCreators(ctx, "board", []string{other.ID, c.ID}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.IsArtifactCreator(ctx, "board", cre.user.ID); !ok {
		t.Fatal("credited creator not recognised")
	}
	credits, err := st.ArtifactCreators(ctx, []string{"board"})
	if err != nil || len(credits["board"]) != 2 || credits["board"][0].Slug != "otto" || credits["board"][1].Links[0].URL != "https://example.com" {
		t.Fatalf("credits %+v %v", credits, err)
	}
	if got, _ := st.CreatorForUser(ctx, cre.user.ID); got.ID != c.ID {
		t.Fatalf("creator for user %+v", got)
	}

	// Posts: drafts stay out of the published list; slugs are unique.
	p, err := st.CreatePost(ctx, store.Post{Slug: "hello", Title: "Hello", Body: "hi", AuthorID: c.ID, CreatedBy: cre.user.ID})
	if err != nil || p.State != "draft" {
		t.Fatalf("post %+v %v", p, err)
	}
	if _, err = st.CreatePost(ctx, store.Post{Slug: "hello", Title: "Again", CreatedBy: cre.user.ID}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate slug: %v", err)
	}
	if list, _ := st.Posts(ctx, "published", "", 0); len(list) != 0 {
		t.Fatalf("draft listed: %+v", list)
	}
	if p, err = st.SetPostState(ctx, p.ID, "published"); err != nil || p.PublishedAt == nil {
		t.Fatalf("publish %+v %v", p, err)
	}
	first := *p.PublishedAt
	_, _ = st.SetPostState(ctx, p.ID, "draft")
	if p, _ = st.SetPostState(ctx, p.ID, "published"); !p.PublishedAt.Equal(first) {
		t.Fatal("republishing moved the publication date")
	}
	if list, _ := st.Posts(ctx, "published", c.ID, 0); len(list) != 1 {
		t.Fatalf("published by author: %+v", list)
	}
}

const zeros = "0000000000000000000000000000000000000000000000000000000000000000"

const testHTML = `<!doctype html><html><head><title>Board</title></head><body>board</body></html>`

// uploadOf is a minimal valid release upload.
func uploadOf(artifact, title, version string) releases.Upload {
	return releases.Upload{ArtifactID: artifact, Title: title, Version: version, HTML: []byte(testHTML), Manifest: []byte(`{"sample":{}}`), MaxHTML: 1 << 20}
}

func TestArtifactPageLatestDownload(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	sub := e.user("sam", "subscriber", true)
	var releases []map[string]any
	for _, version := range []string{"1.0.0", "1.1.0"} {
		freeSample := "false"
		if version == "1.1.0" {
			freeSample = "true"
		}
		code, rel := e.multipartTo(admin, "/api/studio/releases", map[string]string{
			"artifactId": "board", "title": "Board", "version": version,
			"manifestJson": `{}`, "freeSample": freeSample,
		}, "html", "board.html", []byte(testHTML))
		if code != 201 {
			t.Fatalf("upload %s: %d %v", version, code, rel)
		}
		releases = append(releases, rel)
	}
	// Publish the earlier-created draft last: public pages follow publication time.
	for _, rel := range []map[string]any{releases[1], releases[0]} {
		if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rel["id"].(string)+"/publish", nil); code != 200 {
			t.Fatalf("publish: %d %v", code, out)
		}
	}
	oldID, latestID := releases[1]["id"].(string), releases[0]["id"].(string)
	for _, who := range []*tactor{&admin, &sub} {
		code, body, _ := e.do(who, "portal.test", "GET", "/apps/board", nil, nil)
		s := string(body)
		if code != 200 || strings.Count(s, ">Get the file ↓</a>") != 1 || !strings.Contains(s, "/api/releases/"+latestID+"/files/") ||
			strings.Contains(s, "/api/releases/"+oldID+"/") || strings.Contains(s, ">Manifest</a>") {
			t.Fatalf("expected one current HTML download: %d %s", code, body)
		}
	}
	// A historical free sample must not surface as the current paid download.
	if code, body, _ := e.do(nil, "portal.test", "GET", "/apps/board", nil, nil); code != 200 || bytes.Contains(body, []byte("Get the file")) {
		t.Fatalf("anonymous download: %d %s", code, body)
	}
	// History remains downloadable through its authorized API, including manifests.
	for _, raw := range releases[1]["files"].([]any) {
		file := raw.(map[string]any)
		url := "/api/releases/" + oldID + "/files/" + file["id"].(string) + "/download"
		if code, _, _ := e.do(&sub, "portal.test", "GET", url, nil, nil); code != 200 {
			t.Fatalf("historical %s download: %d", file["role"], code)
		}
	}
}

// TestStorefrontPages covers the server-rendered public pages and media.
func TestStorefrontPages(t *testing.T) {
	e := setup(t)
	enableJournal(t, e)
	ctx := context.Background()
	st := e.store
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	sub := e.user("sam", "subscriber", true)
	get := func(a *tactor, path string) (int, string, http.Header) {
		code, b, h := e.do(a, "portal.test", "GET", path, nil, nil)
		return code, string(b), h
	}

	code, rel := e.upload(admin, testHTML)
	if code != 201 {
		t.Fatalf("upload %d %v", code, rel)
	}
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rel["id"].(string)+"/publish", nil); code != 200 {
		t.Fatalf("publish %d %v", code, out)
	}
	png := tinyPNG(t)
	key, sum, err := e.srv.Publisher.Blobs.Put(png)
	if err != nil {
		t.Fatal(err)
	}
	cover, _ := st.CreateMedia(ctx, store.Media{SHA256: sum, StorageKey: key, MediaType: "image/png", Size: int64(len(png)), Width: 2, Height: 2, Alt: "the board", UploadedBy: admin.user.ID})
	a, _ := st.Artifact(ctx, "board")
	a.Description, a.Tags, a.Featured = "A shared board.", []string{"tools"}, true
	_, _ = st.UpdateArtifact(ctx, a)
	_ = st.SavePageDraft(ctx, "board", store.Page{Body: "## Published body\n\n<script>alert(1)</script>", Cover: cover.ID})
	_ = st.PublishPageDraft(ctx, "board")
	_ = st.SavePageDraft(ctx, "board", store.Page{Body: "## Draft body"})
	c, _ := st.CreateCreator(ctx, store.Creator{Slug: "cleo", Name: "Cleo Maker", Bio: "Makes *boards*.", UserID: cleo.user.ID})
	_ = st.SetArtifactCreators(ctx, "board", []string{c.ID})

	// Home, catalog, tag filter.
	if code, b, h := get(nil, "/"); code != 200 || !strings.Contains(b, "Board") || h.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("home %d %s", code, b)
	}
	if code, b, _ := get(nil, "/apps?tag=tools"); code != 200 || !strings.Contains(b, `href="/apps/board"`) || !strings.Contains(b, `/media/`+cover.ID) {
		t.Fatalf("catalog %d %s", code, b)
	}
	if _, b, _ := get(nil, "/apps?tag=none"); strings.Contains(b, `href="/apps/board"`) {
		t.Fatal("tag filter ignored")
	}

	// Artifact page: write-up rendered without raw HTML, OG tags, credits.
	code, b, h := get(nil, "/apps/board")
	if code != 200 || !strings.Contains(b, `<h2 id="published-body">Published body</h2>`) || strings.Contains(b, "<script>alert") ||
		!strings.Contains(b, `og:image" content="http://portal.test/media/`+cover.ID) || !strings.Contains(b, `href="/creators/cleo"`) || strings.Contains(b, "Draft body") {
		t.Fatalf("artifact page %d %s", code, b)
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "media-src 'self'") {
		t.Fatalf("CSP %q", h.Get("Content-Security-Policy"))
	}
	// Downloads are offered to entitled users only.
	if strings.Contains(b, "Get the file") {
		t.Fatal("download offered anonymously")
	}
	if _, b, _ = get(&sub, "/apps/board"); !strings.Contains(b, "Get the file") || !strings.Contains(b, "data-signout") {
		t.Fatal("subscriber page misses download or sign-out")
	}

	// Draft preview only for editors: admin and the credited creator.
	for _, who := range []*tactor{nil, &sub} {
		if _, b, _ = get(who, "/apps/board?draft=1"); strings.Contains(b, "Draft body") || strings.Contains(b, "Draft preview") {
			t.Fatalf("draft leaked to non-editor: %s", b)
		}
	}
	for _, who := range []*tactor{&admin, &cleo} {
		if _, b, _ = get(who, "/apps/board?draft=1"); !strings.Contains(b, "Draft body") || !strings.Contains(b, "Draft preview") {
			t.Fatalf("editor draft preview: %s", b)
		}
	}

	// Hidden or unpublished artifacts are not found, except as an editor's draft.
	a, _ = st.Artifact(ctx, "board")
	a.Visibility = "hidden"
	_, _ = st.UpdateArtifact(ctx, a)
	if code, _, _ = get(nil, "/apps/board"); code != 404 {
		t.Fatalf("hidden artifact %d", code)
	}
	if code, _, _ = get(&cleo, "/apps/board?draft=1"); code != 200 {
		t.Fatalf("hidden artifact draft for editor %d", code)
	}
	_, _ = st.CreateArtifact(ctx, store.Artifact{ID: "unreleased", Title: "Unreleased"})
	if code, _, _ = get(nil, "/apps/unreleased"); code != 404 {
		t.Fatalf("unpublished artifact %d", code)
	}
	if code, _, _ = get(nil, "/apps/missing"); code != 404 {
		t.Fatalf("missing artifact %d", code)
	}

	// Creator page lists only listed artifacts.
	if code, b, _ = get(nil, "/creators/cleo"); code != 200 || !strings.Contains(b, "<em>boards</em>") || strings.Contains(b, `href="/apps/board"`) {
		t.Fatalf("creator page %d %s", code, b)
	}
	if code, _, _ = get(nil, "/creators/nobody"); code != 404 {
		t.Fatalf("unknown creator %d", code)
	}

	// Journal: drafts are visible to their author only; the feed lists published posts.
	post, _ := st.CreatePost(ctx, store.Post{Slug: "first", Title: "First notes", Summary: "How it started", Body: "Hello **world**", AuthorID: c.ID, CreatedBy: cleo.user.ID})
	if code, _, _ = get(nil, "/journal/first"); code != 404 {
		t.Fatalf("draft post public %d", code)
	}
	if code, b, _ = get(&cleo, "/journal/first"); code != 200 || !strings.Contains(b, "Draft preview") {
		t.Fatalf("draft post for author %d", code)
	}
	if _, b, _ = get(nil, "/journal/feed.xml"); strings.Contains(b, "First notes") {
		t.Fatal("draft in feed")
	}
	_, _ = st.SetPostState(ctx, post.ID, "published")
	if code, b, _ = get(nil, "/journal/first"); code != 200 || !strings.Contains(b, "<strong>world</strong>") || !strings.Contains(b, `href="/creators/cleo"`) {
		t.Fatalf("published post %d %s", code, b)
	}
	if code, b, h = get(nil, "/journal/feed.xml"); code != 200 || !strings.Contains(b, "<link>http://portal.test/journal/first</link>") || !strings.HasPrefix(h.Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("feed %d %s", code, b)
	}
	if _, b, _ = get(nil, "/journal"); !strings.Contains(b, "First notes") {
		t.Fatal("journal index misses post")
	}

	// Media: exact bytes, sandboxed, cacheable, ranges.
	code, b, h = get(nil, "/media/"+cover.ID)
	if code != 200 || b != string(png) || h.Get("Content-Type") != "image/png" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") ||
		!strings.Contains(h.Get("Cache-Control"), "immutable") || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("media %d %v", code, h)
	}
	if code, _, _ = e.do(nil, "portal.test", "GET", "/media/"+cover.ID, nil, map[string]string{"Range": "bytes=0-3"}); code != 206 {
		t.Fatalf("media range %d", code)
	}
	if code, _, _ = get(nil, "/media/med_missing"); code != 404 {
		t.Fatalf("missing media %d", code)
	}

	// SPA routes still return the shell.
	if code, b, _ = get(nil, "/studio"); code != 200 || !strings.Contains(b, `id="root"`) {
		t.Fatalf("studio shell %d", code)
	}
}

func tinyPNG(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
