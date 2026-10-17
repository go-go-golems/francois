package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetCaching(t *testing.T) {
	a, ok := PortalAsset("catalog.js")
	if !ok {
		t.Fatal("missing portal bundle")
	}
	get := func(method, url, encoding, etag string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, nil)
		r.Header.Set("Accept-Encoding", encoding)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	plain := get("GET", a.URL(), "identity", "")
	compressed := get("GET", a.URL(), "br, gzip", "")
	if plain.Code != 200 || !bytes.Equal(plain.Body.Bytes(), CatalogJS) || compressed.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("incorrect asset representation")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(z)
	_ = z.Close()
	if err != nil || !bytes.Equal(decoded, CatalogJS) || compressed.Body.Len() >= plain.Body.Len()/2 {
		t.Fatal("gzip must round trip and materially reduce the bundle")
	}
	for _, w := range []*httptest.ResponseRecorder{plain, compressed} {
		if w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || w.Header().Get("Vary") != "Accept-Encoding" || w.Header().Get("ETag") == "" {
			t.Fatal(w.Header())
		}
	}
	if plain.Header().Get("ETag") == compressed.Header().Get("ETag") {
		t.Fatal("strong ETags must distinguish wire representations")
	}
	for _, encoding := range []string{"identity", "gzip"} {
		first := get("GET", a.URL(), encoding, "")
		unchanged := get("GET", a.URL(), encoding, first.Header().Get("ETag"))
		if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 {
			t.Fatalf("revalidation: %d %d", unchanged.Code, unchanged.Body.Len())
		}
	}
	if got := get("GET", a.URL(), "gzip", plain.Header().Get("ETag")); got.Code != 200 {
		t.Fatal("identity validator incorrectly matched gzip")
	}
	if got := get("HEAD", a.URL(), "gzip", ""); got.Code != 200 || got.Body.Len() != 0 || got.Header().Get("Content-Length") == "" {
		t.Fatal("invalid HEAD response")
	}
	for _, url := range []string{"/static/catalog.js", "/static/catalog.js?v=outdated"} {
		if got := get("GET", url, "", ""); got.Header().Get("Cache-Control") != "public, no-cache" {
			t.Fatal("unversioned or mismatched URLs must revalidate")
		}
	}
	updated := newAsset("catalog.js", "text/javascript", append(bytes.Clone(CatalogJS), '\n'))
	if updated.URL() == a.URL() || !bytes.Contains(PortalIndex, []byte(a.URL())) {
		t.Fatal("updated bytes must change the URL used by fresh HTML")
	}
	if _, ok := PortalAsset("runtime.js"); ok {
		t.Fatal("instance asset exposed by portal allowlist")
	}
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"": false, "br": false, "gzip": true, "gzip;q=0": false,
		"gzip;q=0.5, br": true, "br, *;q=1": true,
		"*;q=1, gzip;q=0": false, "gzip;q=invalid": false,
		"gzip;q=2": false, "gzip;q=NaN": false,
	} {
		if got := acceptsGzip(strings.Split(header, "\n")); got != want {
			t.Errorf("%q: got %v, want %v", header, got, want)
		}
	}
}
