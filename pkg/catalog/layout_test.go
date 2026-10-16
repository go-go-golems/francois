package catalog

import (
	"bytes"
	"testing"
)

// TestArealLayout checks the page conventions from Ramon's Areal design:
// titles end with a period, the subtitle is separate, the footer is one
// line and a failed sign-in is reported under the statement.
func TestArealLayout(t *testing.T) {
	e := setup(t)
	pages := map[string][]string{
		"/apps":                {`aria-label="Applications and writing"`},
		"/journal":             {`aria-label="Applications and writing"`},
		"/pricing":             {`<h1>Support.</h1>`},
		"/?login_error=denied": {`class="notice bad login-error"`, `Sign-in did not complete. Please try again.`},
	}
	for path, wants := range pages {
		_, page, _ := e.do(nil, "portal.test", "GET", path, nil, nil)
		for _, want := range append(wants, `<div class="foot-links"><a href="/privacy">Privacy</a><a href="/terms">Terms</a></div>`) {
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
