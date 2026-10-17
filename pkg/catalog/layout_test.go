package catalog

import (
	"bytes"
	"testing"
)

// TestFinalLayout checks the public shell and the separate archive/journal routes.
func TestFinalLayout(t *testing.T) {
	e := setup(t)
	pages := map[string][]string{
		"/apps":                {`aria-label="Experiments"`},
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
