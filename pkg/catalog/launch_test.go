package catalog

import (
	"bytes"
	"context"
	"encoding/xml"
	"html"
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func noRedirect(e *env) *http.Client {
	c := *e.client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func (e *env) raw(host, method, path string, headers map[string]string) *http.Response {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.ts.URL+path, strings.NewReader("{}"))
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := noRedirect(e).Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	_ = res.Body.Close()
	return res
}

// TestRedirectHost retires a portal host: every request, whatever its
// method or path, goes to the primary portal with path and query kept.
func TestRedirectHost(t *testing.T) {
	e := setup(t, func(c *Config) { c.RedirectHosts = []string{"Old.Test"} })
	for _, method := range []string{"GET", "POST"} {
		res := e.raw("old.test", method, "/apps/board?x=1", nil)
		if res.StatusCode != 308 || res.Header.Get("Location") != "http://portal.test/apps/board?x=1" {
			t.Fatalf("%s: %d %q", method, res.StatusCode, res.Header.Get("Location"))
		}
	}
	if res := e.raw("portal.test", "GET", "/", nil); res.StatusCode != 200 {
		t.Fatalf("portal itself: %d", res.StatusCode)
	}

	u, _ := url.Parse("http://portal.test")
	site := Site{PublicURL: u, AppDomain: "apps.test", OIDC: identity.New(identity.Config{})}
	for host, want := range map[string]string{"portal.test": "also a portal", "www.portal.test": "also a portal", "i-abc.apps.test": "instance host", "a/b": "host name"} {
		err := Config{Sites: []Site{site}, RedirectHosts: []string{host}}.Validate()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", host, err, want)
		}
	}
	// AppPort is defaulted from the portal port only in New, after Validate.
	ported, _ := url.Parse("http://localhost:8090")
	site = Site{PublicURL: ported, AppDomain: "apps.localhost", OIDC: identity.New(identity.Config{})}
	if err := (Config{Sites: []Site{site}, RedirectHosts: []string{"i-x.apps.localhost:8090"}}).Validate(); err == nil || !strings.Contains(err.Error(), "instance host") {
		t.Errorf("ported instance host: got %v", err)
	}
}

// TestTransportSecurity sends HSTS on HTTPS deployments and upgrades
// plain-HTTP requests reported by a trusted ingress.
func TestTransportSecurity(t *testing.T) {
	https := func(c *Config) {
		for i := range c.Sites {
			c.Sites[i].PublicURL.Scheme = "https"
		}
		c.HSTSMaxAge, c.TrustProxyHeaders = 24*time.Hour, true
	}
	e := setup(t, https)
	if res := e.raw("portal.test", "GET", "/pricing", nil); res.Header.Get("Strict-Transport-Security") != "max-age=86400; includeSubDomains" {
		t.Fatalf("HSTS: %q", res.Header.Get("Strict-Transport-Security"))
	}
	res := e.raw("boutique.test", "GET", "/apps?x=1", map[string]string{"X-Forwarded-Proto": "http"})
	if res.StatusCode != 308 || res.Header.Get("Location") != "https://boutique.test/apps?x=1" {
		t.Fatalf("upgrade: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	if res := e.raw("evil.test", "GET", "/", map[string]string{"X-Forwarded-Proto": "http"}); res.StatusCode != 421 {
		t.Fatalf("unknown host upgraded: %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	plain := setup(t)
	if res := plain.raw("portal.test", "GET", "/", map[string]string{"X-Forwarded-Proto": "http"}); res.StatusCode != 200 || res.Header.Get("Strict-Transport-Security") != "" {
		t.Fatalf("plain HTTP deployment: %d %q", res.StatusCode, res.Header.Get("Strict-Transport-Security"))
	}
}

// TestCrawlFiles serves robots.txt and a sitemap whose every URL renders,
// and keeps instance hosts and account screens out of search.
func TestCrawlFiles(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("ada", "admin", false)
	alice := e.user("alice", "subscriber", true)
	_, rel := e.upload(admin, testHTML)
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rel["id"].(string)+"/publish", nil); code != 200 {
		t.Fatalf("publish %d %v", code, out)
	}
	c, _ := e.store.CreateCreator(ctx, store.Creator{Slug: "cleo", Name: "Cleo"})
	_ = e.store.SetArtifactCreators(ctx, "board", []string{c.ID})
	_, _ = e.store.CreateArtifact(ctx, store.Artifact{ID: "unreleased", Title: "Unreleased"})

	code, b, h := e.do(nil, "boutique.test", "GET", "/robots.txt", nil, nil)
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/plain") || !strings.Contains(string(b), "Disallow: /studio\n") || !strings.Contains(string(b), "Sitemap: http://boutique.test/sitemap.xml\n") {
		t.Fatalf("robots: %d %s", code, b)
	}

	sitemapLocs := func() []string {
		code, b, h := e.do(nil, "portal.test", "GET", "/sitemap.xml", nil, nil)
		var doc sitemapDoc
		if err := xml.Unmarshal(b, &doc); code != 200 || err != nil || !strings.HasPrefix(h.Get("Content-Type"), "application/xml") {
			t.Fatalf("sitemap: %d %v %s", code, err, b)
		}
		locs := []string{}
		for _, u := range doc.URLs {
			locs = append(locs, u.Loc)
		}
		return locs
	}
	locs := sitemapLocs()
	joined := strings.Join(locs, " ")
	for _, want := range []string{"http://portal.test/ ", "http://portal.test/apps/board ", "http://portal.test/creators/cleo ", "http://portal.test/pricing "} {
		if !strings.Contains(joined+" ", want) {
			t.Fatalf("sitemap lacks %s: %v", want, locs)
		}
	}
	if strings.Contains(joined, "unreleased") || strings.Contains(joined, "/journal") {
		t.Fatalf("sitemap lists a page visitors cannot see: %v", locs)
	}
	enableJournal(t, e)
	post, _ := e.store.CreatePost(ctx, store.Post{Slug: "first", Title: "First", Body: "Hello", AuthorID: c.ID, CreatedBy: admin.user.ID})
	if _, err := e.store.SetPostState(ctx, post.ID, "published"); err != nil {
		t.Fatal(err)
	}
	locs = sitemapLocs()
	if joined = strings.Join(locs, " "); !strings.Contains(joined, "http://portal.test/journal/first") {
		t.Fatalf("published post missing: %v", locs)
	}
	// The sitemap cannot drift from the pages: every URL renders for a visitor.
	for _, loc := range locs {
		if code, _, _ := e.do(nil, "portal.test", "GET", strings.TrimPrefix(loc, "http://portal.test"), nil, nil); code != 200 {
			t.Fatalf("sitemap URL %s: %d", loc, code)
		}
	}

	// Instance hosts are never indexed.
	_, inst := e.portal(&alice, "POST", "/api/instances", map[string]string{"releaseId": rel["id"].(string)})
	host := strings.TrimPrefix(inst["origin"].(string), "http://")
	code, b, h = e.do(nil, host, "GET", "/robots.txt", nil, nil)
	if code != 200 || string(b) != "User-agent: *\nDisallow: /\n" || h.Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("instance robots: %d %q %q", code, b, h.Get("X-Robots-Tag"))
	}
	if _, _, h = e.do(nil, "portal.test", "GET", "/studio", nil, nil); h.Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("SPA shell indexable: %q", h.Get("X-Robots-Tag"))
	}
}

// TestPageMetadata covers the head, icons, error pages and the configurable
// contact address.
func TestPageMetadata(t *testing.T) {
	e := setup(t)
	code, b, _ := e.do(nil, "portal.test", "GET", "/", nil, nil)
	page := string(b)
	for _, want := range []string{`<meta property="og:type" content="website">`, `<meta name="twitter:card" content="summary_large_image">`, `<link rel="manifest" href="/manifest.webmanifest">`, `rel="apple-touch-icon"`, `<link rel="stylesheet" href="/fonts/fonts.css?v=`} {
		if code != 200 || !strings.Contains(page, want) {
			t.Fatalf("home lacks %s", want)
		}
	}
	if strings.Contains(page, "mailto:") {
		t.Fatal("contact shown without --contact-email")
	}
	if _, b, _ = e.do(nil, "portal.test", "GET", "/apps", nil, nil); !strings.Contains(string(b), `<link rel="canonical" href="http://portal.test/">`) {
		t.Fatal("/apps is not canonicalized to /")
	}
	if _, b, _ = e.do(nil, "portal.test", "GET", "/terms", nil, nil); !strings.Contains(string(b), `<link rel="canonical" href="http://portal.test/terms">`) || !strings.Contains(string(b), `/static/catalog.css?v=`) {
		t.Fatal("legal page head")
	}

	code, b, h := e.do(nil, "portal.test", "GET", "/no-such-page", nil, nil)
	if code != 404 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") || !strings.Contains(string(b), "Nothing here.") {
		t.Fatalf("catch-all 404: %d %q", code, h.Get("Content-Type"))
	}
	if code, _, _ = e.do(nil, "portal.test", "POST", "/no-such-page", nil, nil); code != 404 {
		t.Fatalf("POST to unknown path: %d", code)
	}
	if code, _, _ = e.do(nil, "portal.test", "GET", "/api/nothing", nil, nil); code != 404 {
		t.Fatalf("unknown API route: %d", code)
	}

	w := httptest.NewRecorder()
	e.srv.unavailable(w, httptest.NewRequest("GET", "http://portal.test/", nil))
	if w.Code != 503 || !strings.Contains(w.Body.String(), "Temporarily unavailable.") {
		t.Fatalf("unavailable page: %d", w.Code)
	}

	code, b, h = e.do(nil, "portal.test", "GET", "/favicon.ico", nil, nil)
	if code != 200 || h.Get("Content-Type") != "image/x-icon" || len(b) == 0 {
		t.Fatalf("favicon: %d %q", code, h.Get("Content-Type"))
	}
	code, b, _ = e.do(nil, "boutique.test", "GET", "/manifest.webmanifest", nil, nil)
	if code != 200 || !strings.Contains(string(b), `"name":"boutique.test"`) {
		t.Fatalf("manifest: %d %s", code, b)
	}

	withContact := setup(t, func(c *Config) { c.ContactEmail = "hello@example.test" })
	if _, b, _ = withContact.do(nil, "portal.test", "GET", "/pricing", nil, nil); !strings.Contains(string(b), `href="mailto:hello@example.test"`) {
		t.Fatal("configured contact missing from /pricing")
	}
	if _, b, _ = withContact.do(nil, "portal.test", "GET", "/api/session", nil, nil); !strings.Contains(string(b), `"contact":"hello@example.test"`) {
		t.Fatalf("session contact: %s", b)
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestMediaVariants resizes still images to the fixed widths on request,
// caches the result, and serves the original whenever no variant applies.
func TestMediaVariants(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	opaque := image.NewRGBA(image.Rect(0, 0, 800, 500))
	draw.Draw(opaque, opaque.Bounds(), image.NewUniform(color.RGBA{200, 100, 50, 255}), image.Point{}, draw.Src)
	upload := func(name string, data []byte) string {
		code, out := e.multipartTo(admin, "/api/studio/media", map[string]string{"alt": name}, "file", name, data)
		if code != 201 {
			t.Fatalf("upload %s: %d %v", name, code, out)
		}
		return out["id"].(string)
	}
	id := upload("a.png", encodePNG(t, opaque))
	get := func(id, query string) (image.Config, string, http.Header, []byte) {
		code, b, h := e.do(nil, "portal.test", "GET", "/media/"+id+query, nil, nil)
		cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
		if code != 200 || err != nil {
			t.Fatalf("%s%s: %d %v", id, query, code, err)
		}
		return cfg, format, h, b
	}
	cfg, format, h, first := get(id, "?w=320&v=2")
	if format != "jpeg" || cfg.Width != 320 || cfg.Height != 200 || h.Get("Content-Type") != "image/jpeg" || !strings.HasSuffix(h.Get("ETag"), `-w320-v2"`) {
		t.Fatalf("variant: %s %dx%d %q %q", format, cfg.Width, cfg.Height, h.Get("Content-Type"), h.Get("ETag"))
	}
	if _, _, _, again := get(id, "?w=320"); !bytes.Equal(first, again) {
		t.Fatal("variant not served from cache")
	}
	for _, q := range []string{"?w=999", "?w=1200", "?w=x", ""} {
		if cfg, format, _, _ := get(id, q); format != "png" || cfg.Width != 800 {
			t.Fatalf("%s: want the original, got %s %d", q, format, cfg.Width)
		}
	}
	transparent := upload("b.png", encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 400, 400))))
	if cfg, format, _, _ := get(transparent, "?w=160"); format != "png" || cfg.Width != 160 {
		t.Fatalf("transparent variant: %s %d", format, cfg.Width)
	}
	var gif bytes.Buffer
	_ = stdgif.Encode(&gif, image.NewPaletted(image.Rect(0, 0, 400, 300), color.Palette{color.Black, color.White}), nil)
	if _, format, _, _ := get(upload("c.gif", gif.Bytes()), "?w=160"); format != "gif" {
		t.Fatalf("GIF resized to %s", format)
	}

	want := "/media/m?w=160&v=2 160w, /media/m?w=320&v=2 320w, /media/m?w=640&v=2 640w, /media/m 800w"
	if got := srcset(store.Media{ID: "m", MediaType: "image/png", Width: 800, Height: 500}); got != want {
		t.Fatalf("srcset %q", got)
	}
	if got := srcset(store.Media{ID: "m", MediaType: "image/gif", Width: 800, Height: 500}); got != "" {
		t.Fatalf("GIF srcset %q", got)
	}

	// The landing page row thumbnail offers every variant for its 402px
	// column.
	expect := func(what string, code int, out map[string]any, want int) {
		t.Helper()
		if code != want {
			t.Fatalf("%s: %d %v", what, code, out)
		}
	}
	code, out := e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": "thumbs", "title": "Thumbs", "version": "1.0.0", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
	expect("release", code, out, 201)
	code, out = e.portal(&admin, "POST", "/api/studio/releases/"+out["id"].(string)+"/publish", nil)
	expect("publish", code, out, 200)
	code, out = e.portal(&admin, "PUT", "/api/studio/artifacts/thumbs/draft", map[string]any{"body": "Text.", "cover": id})
	expect("draft", code, out, 200)
	code, out = e.portal(&admin, "POST", "/api/studio/artifacts/thumbs/draft/publish", nil)
	expect("publish page", code, out, 200)
	_, home, _ := e.do(nil, "portal.test", "GET", "/", nil, nil)
	if !bytes.Contains(home, []byte(`<img src="/media/`+id+`?w=640&amp;v=2" srcset="/media/`+id+`?w=160&amp;v=2 160w, `)) || !bytes.Contains(home, []byte(`/media/`+id+` 800w" sizes="(max-width: 900px) 100vw, 402px"`)) {
		t.Fatal("landing page thumbnail lacks its srcset")
	}
}

// TestShareImage gives pages without an image a generated 1200×630 card
// named after the configured brand.
func TestShareImage(t *testing.T) {
	e := setup(t)
	_, b, _ := e.do(nil, "portal.test", "GET", "/pricing", nil, nil)
	page := string(b)
	i := strings.Index(page, `<meta property="og:image" content="http://portal.test/share.png?v=`)
	if i < 0 || !strings.Contains(page, `<meta property="og:image:width" content="1200">`) || !strings.Contains(page, `content="summary_large_image"`) {
		t.Fatal("default share image missing from head")
	}
	u := page[i+len(`<meta property="og:image" content="http://portal.test`):]
	u = html.UnescapeString(u[:strings.Index(u, `"`)])
	code, img, h := e.do(nil, "portal.test", "GET", u, nil, nil)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(img))
	if code != 200 || err != nil || format != "png" || cfg.Width != 1200 || cfg.Height != 630 || !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Fatalf("share image: %d %v %s %dx%d %q", code, err, format, cfg.Width, cfg.Height, h.Get("Cache-Control"))
	}
	if p := os.Getenv("SHARE_IMAGE_OUT"); p != "" {
		_ = os.WriteFile(p, img, 0o600)
	}
}
