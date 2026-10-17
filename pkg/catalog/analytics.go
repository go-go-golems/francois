package catalog

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// Only document navigations are counted, never credentials in auth URLs,
// downloads, assets, feeds, APIs, or isolated application content.
func trafficPage(path string) bool {
	for _, p := range []string{"/api/", "/auth/", "/static/", "/media/", "/fonts/", "/webhooks/"} {
		if strings.HasPrefix(path, p) {
			return false
		}
	}
	return path != "/signin" && path != "/healthz" && path != "/readyz" && !strings.HasSuffix(path, ".xml")
}
func trafficAgent(ua string) (string, string, bool) {
	ua = strings.ToLower(ua)
	for _, s := range []string{"bot", "crawler", "spider", "curl/", "wget/", "headless"} {
		if strings.Contains(ua, s) {
			return "", "", true
		}
	}
	device := "desktop"
	if strings.Contains(ua, "mobile") {
		device = "mobile"
	}
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet") {
		device = "tablet"
	}
	browser := "other"
	for _, b := range []struct{ match, name string }{{"edg/", "Edge"}, {"firefox/", "Firefox"}, {"chrome/", "Chrome"}, {"safari/", "Safari"}} {
		if strings.Contains(ua, b.match) {
			browser = b.name
			break
		}
	}
	return device, browser, false
}
func bounded(s string) string { return string([]rune(s)[:min(len([]rune(s)), 200)]) }
func trafficReferrer(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return bounded(strings.ToLower(u.Hostname()))
}
func (s *Server) trafficID(w http.ResponseWriter, r *http.Request, name string, age int, refresh bool) string {
	id := ""
	if c, err := r.Cookie(name); err == nil && len(c.Value) == 32 {
		if _, err = hex.DecodeString(c.Value); err == nil {
			id = c.Value
		}
	}
	if id == "" {
		id = store.RandomHex(16)
		refresh = true
	}
	if refresh {
		s.setCookie(w, name, id, age)
	}
	return id
}

type trafficResponse struct {
	http.ResponseWriter
	status int
}

var _ http.ResponseWriter = (*trafficResponse)(nil)

func (w *trafficResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		if strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *trafficResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *trafficResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) trackTraffic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		device, browser, bot := trafficAgent(r.UserAgent())
		if r.Method != "GET" || !trafficPage(r.URL.Path) || bot || r.Header.Get("DNT") == "1" || r.Header.Get("Sec-GPC") == "1" {
			next.ServeHTTP(w, r)
			return
		}
		prefix := "francois_"
		if s.Config.Secure() {
			prefix = "__Host-francois_"
		}
		visitor := s.trafficID(w, r, prefix+"visitor", 90*86400, false)
		session := s.trafficID(w, r, prefix+"visit", 1800, true)
		out := &trafficResponse{ResponseWriter: w}
		next.ServeHTTP(out, r)
		if out.status != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			return
		}
		// Detached from browser cancellation but strictly bounded. No goroutine or queue.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 250*time.Millisecond)
		defer cancel()
		user := ""
		if p, err := s.resolve(r.WithContext(ctx)); err == nil && p != nil {
			user = p.User.ID
		}
		q := r.URL.Query()
		err := s.Store.RecordView(ctx, store.TrafficView{Site: s.site(r).PortalOrigin(), Path: bounded(r.URL.Path), Visitor: visitor, Session: session, UserID: user, Referrer: trafficReferrer(r.Referer()), Source: bounded(q.Get("utm_source")), Medium: bounded(q.Get("utm_medium")), Campaign: bounded(q.Get("utm_campaign")), Device: device, Browser: browser})
		if err != nil {
			log.Warn().Err(err).Msg("record traffic view")
		}
	})
}

func (s *Server) adminAnalytics(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -30)
	var err error
	if raw := r.URL.Query().Get("from"); raw != "" {
		from, err = time.Parse("2006-01-02", raw)
		if err != nil {
			httpx.Fail(w, 400, "invalid_range", "Use YYYY-MM-DD dates")
			return
		}
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		to, err = time.Parse("2006-01-02", raw)
		if err != nil {
			httpx.Fail(w, 400, "invalid_range", "Use YYYY-MM-DD dates")
			return
		}
	}
	if !from.Before(to) || to.Sub(from) > 90*24*time.Hour {
		httpx.Fail(w, 400, "invalid_range", "Range must be positive and at most 90 days; to is exclusive")
		return
	}
	q := r.URL.Query()
	f := store.TrafficFilter{Site: q.Get("site"), Path: q.Get("path")}
	if q.Has("referrer") {
		ref := q.Get("referrer")
		f.Referrer = &ref
	}
	if len([]rune(f.Site)) > 200 || len([]rune(f.Path)) > 200 || f.Path != "" && (!strings.HasPrefix(f.Path, "/") || strings.ContainsAny(f.Path, "?#")) || f.Referrer != nil && (len([]rune(*f.Referrer)) > 200 || strings.ContainsAny(*f.Referrer, "/?#@ ")) {
		httpx.Fail(w, 400, "invalid_filter", "Use a path without query/fragment and a referring hostname; filters are limited to 200 characters")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	a, err := s.Store.Analytics(ctx, from, to, f)
	if err != nil {
		unavailable(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpx.OK(w, a)
}
func (s *Server) pruneTraffic(ctx context.Context) error {
	for {
		cleanup, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := s.Store.PruneTraffic(cleanup)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Warn().Err(err).Msg("prune traffic")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Hour):
		}
	}
}
