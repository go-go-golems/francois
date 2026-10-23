package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

func TestLegalPagesFromCopy(t *testing.T) {
	e := setup(t, func(c *Config) { c.LegalContact = "legal@example.com" })
	for page, heading := range map[string]string{"privacy": "Privacy policy", "terms": "Terms of service"} {
		code, b, _ := e.do(nil, "portal.test", "GET", "/"+page, nil, nil)
		body := string(b)
		for _, want := range []string{`class="site-footer"`, `<h1>` + heading + `</h1>`, `portal.test is a hosted catalog`, `<a href="mailto:legal@example.com">legal@example.com</a>`, `Last updated 16 October 2026.`} {
			if code != 200 || !strings.Contains(body, want) {
				t.Fatalf("/%s %d lacks %q", page, code, want)
			}
		}
		if strings.Contains(body, "{site}") || strings.Contains(body, "{contact}") {
			t.Fatalf("/%s left a placeholder", page)
		}
	}
	doc := settings.Defaults()
	if err := doc.SetCopy("legal.terms", "Last updated 1 January 2027.\n\n## New\n\nBe kind on {site}."); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.SaveSettings(context.Background(), "", doc); err != nil {
		t.Fatal(err)
	}
	_, b, _ := e.do(nil, "portal.test", "GET", "/terms", nil, nil)
	if !strings.Contains(string(b), "Be kind on portal.test.") || strings.Contains(string(b), "Acceptable use") {
		t.Fatal("edited terms not shown")
	}
}
