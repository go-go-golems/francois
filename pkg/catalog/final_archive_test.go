package catalog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestFinalArchiveQueriesAndPublicationIdentity(t *testing.T) {
	e := setup(t)
	enableJournal(t, e)
	ctx := context.Background()
	admin := e.user("editor", "admin", false)
	publish := func(id, title, version string) store.Release {
		t.Helper()
		rel, _, err := e.srv.Publisher.CreateDraft(ctx, admin.user.ID, uploadOf(id, title, version))
		if err != nil {
			t.Fatal(err)
		}
		rel, err = e.store.PublishRelease(ctx, rel.ID, admin.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		return rel
	}
	first := publish("alpha", "Alpha tool", "1.0.0")
	publish("beta", "Beta graphic", "1.0.0")
	for _, id := range []string{"alpha", "beta"} {
		a, err := e.store.Artifact(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		a.Tags = []string{"graphics"}
		if _, err = e.store.UpdateArtifact(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	publish("alpha", "", "1.1.0")
	hidden := publish("hidden", "Never show this", "1.0.0")
	a, _ := e.store.Artifact(ctx, hidden.ArtifactID)
	a.Visibility = "hidden"
	if _, err := e.store.UpdateArtifact(ctx, a); err != nil {
		t.Fatal(err)
	}
	draft, _, err := e.srv.Publisher.CreateDraft(ctx, admin.user.ID, uploadOf("draft", "Not published", "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	_ = draft
	p, err := e.store.CreatePost(ctx, store.Post{Slug: "writing", Title: "Journal entry", Body: "Writing", CreatedBy: admin.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.SetPostState(ctx, p.ID, "published"); err != nil {
		t.Fatal(err)
	}
	cards, err := e.srv.cards(ctx, listed)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].ID != "alpha" || cards[0].Number != 1 || !cards[0].FirstPublished.Equal(*first.PublishedAt) || cards[1].Number != 2 {
		t.Fatalf("archive %+v", cards)
	}
	for _, c := range cards {
		number, published, err := e.store.ArchivePosition(ctx, c.ID)
		if err != nil || number != c.Number || !published.Equal(*c.FirstPublished) {
			t.Fatalf("detail/archive mismatch for %s: %d %v %v", c.ID, number, published, err)
		}
	}
	_, all, _ := e.do(nil, "portal.test", "GET", "/", nil, nil)
	if bytes.Index(all, []byte(`href="/apps/beta"`)) > bytes.Index(all, []byte(`href="/apps/alpha"`)) || bytes.Contains(all, []byte("Journal entry")) || bytes.Contains(all, []byte("Never show this")) || bytes.Contains(all, []byte("Not published")) {
		t.Fatalf("archive order/visibility: %s", all)
	}
	_, filtered, _ := e.do(nil, "portal.test", "GET", "/?q=alpha&tag=graphics&sort=oldest", nil, nil)
	if !bytes.Contains(filtered, []byte(`class="archive-number">001`)) || bytes.Contains(filtered, []byte(`href="/apps/beta"`)) || !bytes.Contains(filtered, []byte(`q=alpha&amp;sort=oldest`)) {
		t.Fatalf("filters/identity: %s", filtered)
	}
	_, empty, _ := e.do(nil, "portal.test", "GET", "/?q=%22%3E%3Cscript%3E", nil, nil)
	if !bytes.Contains(empty, []byte("No experiments match")) || bytes.Contains(empty, []byte(`<script>`)) {
		t.Fatalf("empty/search escaping: %s", empty)
	}
	_, journal, _ := e.do(nil, "portal.test", "GET", "/journal", nil, nil)
	if !strings.Contains(string(journal), "Journal entry") || strings.Contains(string(journal), `href="/apps/alpha"`) {
		t.Fatalf("journal separation: %s", journal)
	}
}
