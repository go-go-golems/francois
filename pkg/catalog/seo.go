package catalog

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/web"
)

// Crawling metadata is generated per request from the site the request
// arrived on, so no host name appears in code. The sitemap lists only pages
// that render for a visitor, using the same rules as the pages themselves.

// privatePaths are the account, studio, API and sign-in surfaces kept out of
// search; public pages stay crawlable.
var privatePaths = []string{"/api/", "/auth/", "/account", "/admin", "/studio", "/instances/", "/signin", "/thank-you", "/mcp"}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range privatePaths {
		b.WriteString("Disallow: " + p + "\n")
	}
	b.WriteString("Sitemap: " + s.site(r).PortalOrigin() + "/sitemap.xml\n")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(b.String()))
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapDoc struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		http.Error(w, "Sitemap unavailable", http.StatusServiceUnavailable)
		return
	}
	cards, err := s.cards(ctx, listed)
	if err != nil {
		http.Error(w, "Sitemap unavailable", http.StatusServiceUnavailable)
		return
	}
	var newest time.Time
	later := func(cur *time.Time, t *time.Time) {
		if t != nil && t.After(*cur) {
			*cur = *t
		}
	}
	apps := []sitemapURL{}
	creators := map[string]time.Time{}
	for _, c := range cards {
		later(&newest, c.Published)
		apps = append(apps, sitemapURL{Loc: "/apps/" + url.PathEscape(c.ID), LastMod: day(c.Published)})
		for _, cr := range c.Creators {
			t := creators[cr.Slug]
			later(&t, c.Published)
			creators[cr.Slug] = t
		}
	}
	journal := []sitemapURL{}
	if rev.Document.Storefront.JournalEnabled {
		posts, err := s.Store.Posts(ctx, "published", "", 0)
		if err != nil {
			http.Error(w, "Sitemap unavailable", http.StatusServiceUnavailable)
			return
		}
		var latest time.Time
		for _, p := range posts {
			later(&latest, p.PublishedAt)
			journal = append(journal, sitemapURL{Loc: "/journal/" + url.PathEscape(p.Slug), LastMod: day(p.PublishedAt)})
		}
		later(&newest, &latest)
		journal = append([]sitemapURL{{Loc: "/journal", LastMod: day(&latest)}}, journal...)
	}
	urls := []sitemapURL{{Loc: "/", LastMod: day(&newest)}}
	urls = append(urls, apps...)
	slugs := make([]string, 0, len(creators))
	for slug := range creators {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		t := creators[slug]
		urls = append(urls, sitemapURL{Loc: "/creators/" + url.PathEscape(slug), LastMod: day(&t)})
	}
	urls = append(urls, journal...)
	urls = append(urls, sitemapURL{Loc: "/pricing"}, sitemapURL{Loc: "/privacy"}, sitemapURL{Loc: "/terms"})
	origin := s.site(r).PortalOrigin()
	for i := range urls {
		urls[i].Loc = origin + urls[i].Loc
	}
	out, err := xml.MarshalIndent(sitemapDoc{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}, "", "  ")
	if err != nil {
		http.Error(w, "Sitemap unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

// day formats a sitemap lastmod; zero or missing times are omitted.
func day(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// manifest names the installed site after the configured storefront brand.
func (s *Server) manifest(w http.ResponseWriter, r *http.Request) {
	brand := s.site(r).PublicURL.Hostname()
	if rev, err := s.Store.ActiveSettings(r.Context()); err == nil {
		brand = s.brand(r, rev.Document)
	}
	out, _ := json.Marshal(map[string]any{
		"name": brand, "short_name": brand, "start_url": "/", "display": "browser",
		"background_color": "#ffffff", "theme_color": "#111111",
		"icons": []map[string]string{
			{"src": web.PortalURL("apple-touch-icon.png"), "sizes": "180x180", "type": "image/png"},
			{"src": web.PortalURL("icon-512.png"), "sizes": "512x512", "type": "image/png"},
			{"src": web.PortalURL("icon.svg"), "sizes": "any", "type": "image/svg+xml"},
		},
	})
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(out)
}

// rootAsset serves an embedded portal asset at a fixed root path browsers
// request on their own, such as /favicon.ico.
func rootAsset(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		asset, ok := web.PortalAsset(name)
		if !ok {
			http.NotFound(w, r)
			return
		}
		asset.ServeHTTP(w, r)
	}
}
