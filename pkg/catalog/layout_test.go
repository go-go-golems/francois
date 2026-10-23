package catalog

import (
	"bytes"
	"context"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

// TestFinalLayout checks the public shell and the separate archive/journal routes.
func TestFinalLayout(t *testing.T) {
	e := setup(t)
	enableJournal(t, e)
	pages := map[string][]string{
		"/apps":                {`aria-label="Artifacts"`},
		"/journal":             {`aria-label="Journal"`},
		"/pricing":             {`<h1>Support the lab.</h1>`},
		"/?login_error=denied": {`class="notice bad login-error"`, `Sign-in did not complete. Please try again.`},
	}
	for path, wants := range pages {
		_, page, _ := e.do(nil, "portal.test", "GET", path, nil, nil)
		for _, want := range append(wants, `class="site-footer"`, `For humans`, `href="/privacy"`, `href="/terms"`) {
			if !bytes.Contains(page, []byte(want)) {
				t.Errorf("%s lacks %q", path, want)
			}
		}
		if bytes.Contains(page, []byte("For machines")) || bytes.Contains(page, []byte(`<span class="muted">—`)) {
			t.Errorf("%s still has the old footer or a two-tone title", path)
		}
	}
	if _, page, _ := e.do(nil, "portal.test", "GET", "/?login_error=denied", nil, nil); bytes.Contains(page, []byte("(denied)")) {
		t.Error("the raw error code is shown")
	}
}

// TestSiteCopy renders registered defaults until staff override them in
// storefront.copy, and escapes text while rendering Markdown keys.
func TestSiteCopy(t *testing.T) {
	e := setup(t)
	_, page, _ := e.do(nil, "portal.test", "GET", "/", nil, nil)
	for _, want := range []string{
		`<p class="site-statement">A lab for computation and intelligence as material and medium. Run by Manuel and Ramon.</p>`,
		`<h2 id="support-heading">Keep the lab running.</h2>`,
	} {
		if !bytes.Contains(page, []byte(want)) {
			t.Errorf("default copy missing %q", want)
		}
	}
	doc := settings.Defaults()
	doc.Storefront.Copy = map[string]string{"header.statement": "Fish & <chips>.", "callout.body": "Pay **what** you like."}
	if _, err := e.store.SaveSettings(context.Background(), "", doc); err != nil {
		t.Fatal(err)
	}
	_, page, _ = e.do(nil, "portal.test", "GET", "/", nil, nil)
	for _, want := range []string{`<p class="site-statement">Fish &amp; &lt;chips&gt;.</p>`, `Pay <strong>what</strong> you like.`} {
		if !bytes.Contains(page, []byte(want)) {
			t.Errorf("override missing %q", want)
		}
	}
	if !bytes.Contains(page, []byte(`content="Fish &amp; &lt;chips&gt;."`)) {
		t.Error("landing page description does not follow the header statement")
	}
}
