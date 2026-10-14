package catalog

import (
	"context"
	"errors"
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
	a.Title, a.Featured, a.RequiredPlan, a.DemoEnabled = "Board, renamed", true, "studio", true
	if _, err = st.UpdateArtifact(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.srv.Publisher.CreateDraft(ctx, admin.user.ID, uploadOf("board", "", "1.1.0")); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if a, _ = st.Artifact(ctx, "board"); a.Title != "Board, renamed" || !a.Featured || a.RequiredPlan != "studio" || !a.DemoEnabled {
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
