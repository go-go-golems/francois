package catalog

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Licensed web fonts. The storefront typeface (ABC Areal by Dinamo, made
// for Are.na) is free but may not be put in public repositories, so the
// binary embeds no font files: the deployment points --font-dir at a
// private directory of WOFF2 files and the portal serves them itself
// (self-hosted @font-face is what the web license allows). Without a font
// directory, /fonts/fonts.css is empty and pages use the fallback stack.

// fontFace describes one known WOFF2 file.
type fontFace struct {
	Family string
	Weight int
	Italic bool
}

// knownFonts lists the files the portal will serve; nothing else in the
// directory is reachable.
var knownFonts = map[string]fontFace{
	"ABCAreal-Regular.woff2":           {"ABC Areal", 400, false},
	"ABCAreal-RegularItalic.woff2":     {"ABC Areal", 400, true},
	"ABCAreal-Medium.woff2":            {"ABC Areal", 500, false},
	"ABCAreal-MediumItalic.woff2":      {"ABC Areal", 500, true},
	"ABCAreal-Bold.woff2":              {"ABC Areal", 700, false},
	"ABCArealMono-Regular.woff2":       {"ABC Areal Mono", 400, false},
	"ABCArealMono-Medium.woff2":        {"ABC Areal Mono", 500, false},
	"ABCArealMono-RegularItalic.woff2": {"ABC Areal Mono", 400, true},
}

// fonts are the web fonts loaded at startup. Each file and the stylesheet
// carry a content hash; a request naming the current hash (?v=) is cached
// for a year, an unversioned one keeps short revalidating caching.
type fonts struct {
	files   map[string][]byte
	hashes  map[string]string
	css     []byte
	cssHash string
	loaded  time.Time
	present bool
}

// preloadFonts are the faces used above the fold on every storefront page.
var preloadFonts = []string{"ABCAreal-Regular.woff2", "ABCAreal-Medium.woff2"}

func contentHash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b))[:16] }

// cssURL is the versioned stylesheet URL for server-rendered pages.
func (f *fonts) cssURL() string { return "/fonts/fonts.css?v=" + f.cssHash }

// fileURL is the versioned URL of a loaded font file.
func (f *fonts) fileURL(name string) string { return "/fonts/" + name + "?v=" + f.hashes[name] }

// preloads are the versioned URLs of the loaded above-the-fold faces.
func (f *fonts) preloads() []string {
	out := []string{}
	for _, name := range preloadFonts {
		if _, ok := f.files[name]; ok {
			out = append(out, f.fileURL(name))
		}
	}
	return out
}

// loadFonts reads the known WOFF2 files from dir. An empty dir disables
// web fonts; a configured dir without any known file is an error, so a
// broken mount is noticed at startup.
func loadFonts(dir string) (*fonts, error) {
	f := &fonts{files: map[string][]byte{}, hashes: map[string]string{}, css: []byte("/* no web fonts configured */\n"), loaded: time.Now()}
	f.cssHash = contentHash(f.css)
	if dir == "" {
		return f, nil
	}
	var css bytes.Buffer
	for _, name := range sortedFontNames() {
		data, err := os.ReadFile(filepath.Join(dir, name)) // #nosec G304 -- fixed allowlist of names
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, errors.Wrap(err, "read font")
		}
		if !bytes.HasPrefix(data, []byte("wOF2")) {
			return nil, errors.Errorf("font %s is not a WOFF2 file", name)
		}
		face := knownFonts[name]
		style := "normal"
		if face.Italic {
			style = "italic"
		}
		f.files[name], f.hashes[name] = data, contentHash(data)
		fmt.Fprintf(&css, "@font-face{font-family:%q;src:url(%s) format(\"woff2\");font-weight:%d;font-style:%s;font-display:swap}\n", face.Family, f.fileURL(name), face.Weight, style)
	}
	if len(f.files) == 0 {
		return nil, errors.Errorf("no known WOFF2 fonts in %s", dir)
	}
	f.css, f.present = css.Bytes(), true
	f.cssHash = contentHash(f.css)
	return f, nil
}

func sortedFontNames() []string {
	names := make([]string, 0, len(knownFonts))
	for n := range knownFonts {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic CSS
	return names
}

func (s *Server) fontCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if r.URL.Query().Get("v") == s.fonts.cssHash {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, "", s.fonts.loaded, bytes.NewReader(s.fonts.css))
}

func (s *Server) fontFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	data, ok := s.fonts.files[name]
	if !ok || strings.ContainsAny(name, "/\\") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/woff2")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	if r.URL.Query().Get("v") == s.fonts.hashes[name] {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(w, r, "", s.fonts.loaded, bytes.NewReader(data))
}
