package catalog

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestBrowser covers the landing page browser: hero copy, month groups,
// sort orders, tag counts, and the download / early-access rules for each
// kind of visitor.
func TestBrowser(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("editor", "admin", false)
	free := e.user("fred", "", false)
	paying := e.user("pat", "subscriber", true)
	publish := func(id, title string, freeSample bool, published time.Time, html string) {
		t.Helper()
		up := uploadOf(id, title, "1.0.0")
		up.FreeSample, up.HTML = freeSample, []byte(html)
		rel, _, err := e.srv.Publisher.CreateDraft(ctx, admin.user.ID, up)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.store.PublishRelease(ctx, rel.ID, admin.user.ID, 0); err != nil {
			t.Fatal(err)
		}
		if _, err = e.store.Pool.Exec(ctx, `UPDATE releases SET published_at=$2 WHERE id=$1`, rel.ID, published); err != nil {
			t.Fatal(err)
		}
	}
	sep := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	oct := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	publish("zeta", "Zeta loom", true, sep, testHTML)
	publish("alpha", "Alpha tool", false, oct, testHTML+strings.Repeat(" ", 4096))
	publish("early", "Early bird", false, oct.Add(time.Hour), testHTML)
	for id, tags := range map[string][]string{"zeta": {"graphics"}, "alpha": {"graphics", "tools"}, "early": {"tools"}} {
		a, err := e.store.Artifact(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		a.Tags = tags
		if id == "early" {
			until := time.Now().Add(72 * time.Hour)
			a.EarlyAccessUntil = &until
		}
		if id == "alpha" {
			a.LaunchMode = "inline"
		}
		if _, err = e.store.UpdateArtifact(ctx, a); err != nil {
			t.Fatal(err)
		}
	}

	_, page, _ := e.do(nil, "portal.test", "GET", "/", nil, nil)
	html := string(page)
	for _, want := range []string{
		`<h1>These artifacts are the results of our research.`,
		`<span class="tag-label">graphics</span> <span class="tag-count">2</span>`,
		`<span class="tag-label">All</span> <span class="tag-count">3</span>`,
		`<span>October 2026</span><span data-browser-group-count>2 artifacts</span>`,
		`<span>September 2026</span><span data-browser-group-count>1 artifact</span>`,
		`portal.test/apps/zeta · `,
		`data-launch="inline"`,
		`<span class="result-count" data-browser-count aria-live="polite">3 artifacts</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("landing page lacks %q", want)
		}
	}
	order := func(page string, ids ...string) {
		t.Helper()
		last := -1
		for _, id := range ids {
			i := strings.Index(page, `<a href="/apps/`+id+`">`)
			if i < 0 || i < last {
				t.Fatalf("order: %s before position %d (%v)", id, last, ids)
			}
			last = i
		}
	}
	order(html, "early", "alpha", "zeta")
	for sortBy, ids := range map[string][]string{"oldest": {"zeta", "alpha", "early"}, "az": {"alpha", "early", "zeta"}, "large": {"alpha"}, "small": {"zeta"}} {
		_, p, _ := e.do(nil, "portal.test", "GET", "/?sort="+sortBy, nil, nil)
		order(string(p), ids...)
	}
	if _, p, _ := e.do(nil, "portal.test", "GET", "/?sort=az", nil, nil); !bytes.Contains(p, []byte(`<span>A</span><span data-browser-group-count>1 artifact</span>`)) {
		t.Error("A–Z groups by initial")
	}
	if _, p, _ := e.do(nil, "portal.test", "GET", "/?sort=bogus", nil, nil); !bytes.Contains(p, []byte(`<option value="newest" selected>`)) {
		t.Error("unknown sort falls back to newest")
	}

	// Download and early access, per visitor. Rows are split on <article.
	row := func(page []byte, id string) string {
		for _, r := range strings.Split(string(page), "<article") {
			if strings.Contains(r, `<a href="/apps/`+id+`">`) {
				return r
			}
		}
		t.Fatalf("no row for %s", id)
		return ""
	}
	direct := regexp.MustCompile(`href="/api/releases/rel_[0-9a-f]+/files/[^"]+/download" download=`)
	for _, v := range []struct {
		who                *tactor
		zeta, alpha, early string
	}{
		{nil, "direct", "signin", "note"},
		{&free, "direct", "direct", "note"},
		{&paying, "direct", "direct", "direct"},
		{&admin, "direct", "direct", "direct"},
	} {
		_, p, _ := e.do(v.who, "portal.test", "GET", "/", nil, nil)
		who := "guest"
		if v.who != nil {
			who = v.who.user.Email
		}
		for id, want := range map[string]string{"zeta": v.zeta, "alpha": v.alpha, "early": v.early} {
			r := row(p, id)
			got := "none"
			switch {
			case strings.Contains(r, `class="ri-early"`):
				got = "note"
			case direct.MatchString(r):
				got = "direct"
			case strings.Contains(r, `href="/signin?return=%2Fapps%2F`+id+`">Download`):
				got = "signin"
			}
			if got != want {
				t.Errorf("%s sees %s download as %s, want %s", who, id, got, want)
			}
		}
	}
	if _, p, _ := e.do(nil, "portal.test", "GET", "/", nil, nil); !strings.Contains(row(p, "early"), `Early access for subscribers until `) || !strings.Contains(row(p, "early"), `<a class="u" href="/pricing">Subscribe →</a>`) {
		t.Error("early access row does not invite to subscribe")
	}

	// The app page carries the launch mode and copy labels for public.ts,
	// and renders the subscribe invitation itself for visitors who do not pay.
	_, app, _ := e.do(nil, "portal.test", "GET", "/apps/alpha", nil, nil)
	for _, want := range []string{`data-launch="inline"`, `data-label-start="Run artifact →"`, `data-label-demo="Try it →"`, `>Sign in to run</a>`} {
		if !bytes.Contains(app, []byte(want)) {
			t.Errorf("app page lacks %q", want)
		}
	}
	_, early, _ := e.do(&free, "portal.test", "GET", "/apps/early", nil, nil)
	if !bytes.Contains(early, []byte(`<p class="runner-early">In early access for subscribers until `)) || !bytes.Contains(early, []byte(`<a class="site-button is-primary" href="/pricing">Subscribe →</a>`)) || !bytes.Contains(early, []byte(`data-launch="tab"`)) {
		t.Error("early access app page does not invite a free account to subscribe")
	}
	if _, early, _ = e.do(&paying, "portal.test", "GET", "/apps/early", nil, nil); bytes.Contains(early, []byte(`class="runner-early"`)) {
		t.Error("paying visitors are invited to subscribe")
	}
}
