package catalog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFonts(t *testing.T) {
	if f, err := loadFonts(""); err != nil || f.present || !strings.Contains(string(f.css), "no web fonts") {
		t.Fatalf("no dir: %+v %v", f, err)
	}
	empty := t.TempDir()
	if _, err := loadFonts(empty); err == nil {
		t.Fatal("a configured directory without fonts must fail")
	}
	bad := t.TempDir()
	_ = os.WriteFile(filepath.Join(bad, "ABCAreal-Regular.woff2"), []byte("<html>"), 0o600)
	if _, err := loadFonts(bad); err == nil {
		t.Fatal("a non-WOFF2 file must fail")
	}
}

func TestFontsServedAndCredited(t *testing.T) {
	e := setup(t)
	// Without fonts: empty stylesheet, no credit.
	code, css, _ := e.do(nil, "portal.test", "GET", "/fonts/fonts.css", nil, nil)
	if code != 200 || strings.Contains(string(css), "@font-face") {
		t.Fatalf("css without fonts %d %s", code, css)
	}
	if _, page, _ := e.do(nil, "portal.test", "GET", "/", nil, nil); bytes.Contains(page, []byte("foot-credit")) {
		t.Fatal("credit shown without the typeface")
	}

	dir := t.TempDir()
	woff := append([]byte("wOF2"), bytes.Repeat([]byte{1}, 60)...)
	_ = os.WriteFile(filepath.Join(dir, "ABCAreal-Regular.woff2"), woff, 0o600)
	_ = os.WriteFile(filepath.Join(dir, "ABCAreal-Medium.woff2"), woff, 0o600)
	_ = os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("not served"), 0o600)
	f, err := loadFonts(dir)
	if err != nil {
		t.Fatal(err)
	}
	e.srv.fonts = f

	_, css, _ = e.do(nil, "portal.test", "GET", "/fonts/fonts.css", nil, nil)
	for _, want := range []string{`font-family:"ABC Areal";src:url(/fonts/ABCAreal-Medium.woff2?v=` + f.hashes["ABCAreal-Medium.woff2"] + `) format("woff2");font-weight:500`, "ABCAreal-Regular.woff2"} {
		if !strings.Contains(string(css), want) {
			t.Fatalf("css lacks %q: %s", want, css)
		}
	}
	code, body, h := e.do(nil, "portal.test", "GET", "/fonts/ABCAreal-Regular.woff2", nil, nil)
	if code != 200 || h.Get("Content-Type") != "font/woff2" || !bytes.Equal(body, woff) {
		t.Fatalf("font %d %v", code, h)
	}
	if h.Get("Cache-Control") != "public, max-age=604800" {
		t.Fatalf("unversioned font cache: %q", h.Get("Cache-Control"))
	}
	// Versioned URLs, as the stylesheet and pages use them, are immutable.
	for _, u := range []string{f.fileURL("ABCAreal-Regular.woff2"), f.cssURL()} {
		if _, _, h = e.do(nil, "portal.test", "GET", u, nil, nil); h.Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatalf("%s cache: %q", u, h.Get("Cache-Control"))
		}
	}
	if _, page, _ := e.do(nil, "portal.test", "GET", "/", nil, nil); !bytes.Contains(page, []byte(`<link rel="preload" href="`+f.fileURL("ABCAreal-Medium.woff2")+`" as="font" type="font/woff2" crossorigin>`)) {
		t.Fatal("above-the-fold faces are not preloaded")
	}
	for _, p := range []string{"/fonts/secret.txt", "/fonts/ABCAreal-Bold.woff2", "/fonts/..%2Fsecret.txt"} {
		if code, _, _ := e.do(nil, "portal.test", "GET", p, nil, nil); code != 404 {
			t.Fatalf("%s: %d", p, code)
		}
	}
	if _, page, _ := e.do(nil, "portal.test", "GET", "/", nil, nil); !bytes.Contains(page, []byte(`Set in <a href="https://abcdinamo.com/free/areal">ABC Areal</a>`)) || !bytes.Contains(page, []byte(`href="`+f.cssURL()+`"`)) {
		t.Fatal("home page lacks the font stylesheet or the credit")
	}
}
