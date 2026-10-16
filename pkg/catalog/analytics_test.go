package catalog

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestTrafficPolicy(t *testing.T) {
	for _, p := range []string{"/api/session", "/auth/callback", "/media/x", "/static/catalog.js", "/healthz", "/readyz", "/apps/feed.xml", "/journal/feed.xml"} {
		if trafficPage(p) {
			t.Errorf("counts %s", p)
		}
	}
	for _, p := range []string{"/", "/apps", "/journal/hello", "/account", "/admin/analytics"} {
		if !trafficPage(p) {
			t.Errorf("excludes %s", p)
		}
	}
	if got := trafficReferrer("https://name:secret@SEARCH.example/path?token=secret#frag"); got != "search.example" {
		t.Fatal(got)
	}
	if trafficReferrer("javascript:secret") != "" {
		t.Fatal("invalid referrer")
	}
	d, b, bot := trafficAgent("Mozilla/5.0 Mobile Chrome/100 Safari/1")
	if d != "mobile" || b != "Chrome" || bot {
		t.Fatal(d, b, bot)
	}
	if _, _, bot := trafficAgent("Googlebot/1"); !bot {
		t.Fatal("bot included")
	}
	if len([]rune(bounded(strings.Repeat("é", 201)))) != 200 {
		t.Fatal("unicode truncation")
	}
}
func TestRSSSerialization(t *testing.T) {
	doc := rssDoc{Version: "2.0"}
	doc.Channel.Title = "A & B"
	doc.Channel.Items = []rssItem{{Title: "<hello>", GUID: rssGUID{Value: "urn:francois:release:r1"}}}
	w := httptest.NewRecorder()
	writeRSS(w, doc)
	var parsed rssDoc
	if err := xml.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Channel.Title != "A & B" || parsed.Channel.Items[0].Title != "<hello>" || !strings.Contains(w.Body.String(), `isPermaLink="false"`) {
		t.Fatal(w.Body.String())
	}
}
func TestTrafficAnalytics(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("analytics-admin", "admin", false)
	sub := e.user("analytics-sub", "subscriber", false)
	headers := map[string]string{"User-Agent": "Mozilla/5.0 Mobile Chrome/100", "Referer": "https://search.example/private?secret=value"}
	code, _, h := e.do(&sub, "portal.test", "GET", "/?utm_source=news&utm_medium=email&utm_campaign=launch&token=secret", nil, headers)
	if code != 200 {
		t.Fatal(code)
	}
	res := &http.Response{Header: h}
	cookies := res.Cookies()
	var cookieParts []string
	for _, c := range cookies {
		if strings.Contains(c.Name, "visitor") || strings.Contains(c.Name, "visit") {
			cookieParts = append(cookieParts, c.Name+"="+c.Value)
			if !c.HttpOnly || c.MaxAge <= 0 {
				t.Fatal(c)
			}
		}
	}
	if len(cookieParts) != 2 {
		t.Fatal(cookies)
	}
	headers["Cookie"] = strings.Join(cookieParts, "; ")
	e.do(&sub, "portal.test", "GET", "/pricing", nil, headers)
	for _, pair := range [][2]string{{"DNT", "1"}, {"Sec-GPC", "1"}, {"User-Agent", "Googlebot"}} {
		e.do(nil, "portal.test", "GET", "/", nil, map[string]string{pair[0]: pair[1]})
	}
	e.do(nil, "portal.test", "HEAD", "/", nil, nil)
	e.do(nil, "portal.test", "GET", "/missing", nil, nil)
	e.do(nil, "portal.test", "GET", "/journal/feed.xml", nil, nil)
	e.do(nil, "boutique.test", "GET", "/", nil, nil)
	from := time.Now().UTC().AddDate(0, 0, -1)
	to := from.AddDate(0, 0, 2)
	resv, err := e.store.Reserve(ctx, store.ReserveRequest{UserID: sub.user.ID, InstanceID: "test", CallID: "test", Model: "fake/model", Credits: 2, Included: 100, Period: "demo:test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = e.store.StartCall(ctx, resv); err != nil {
		t.Fatal(err)
	}
	if err = e.store.SettleCall(ctx, resv, true, "", store.CallUsage{Input: 10, Output: 5, CacheRead: 3, CacheWrite: 2, Cost: 0.01, ModelUsed: "fake/actual"}); err != nil {
		t.Fatal(err)
	}
	a, err := e.store.Analytics(ctx, from, to, "http://portal.test")
	if err != nil {
		t.Fatal(err)
	}
	if a.Traffic.Views != 2 || a.Traffic.Visitors != 1 || a.Traffic.Sessions != 1 || a.Traffic.Users != 1 || a.Traffic.Bounces != 0 {
		t.Fatalf("%+v", a.Traffic)
	}
	if a.Groups["referrers"][0].Key != "search.example" || a.Models["models"][0].CacheRead != 3 || a.Models["models"][0].Credits != 2 {
		t.Fatalf("%+v", a)
	}
	all, err := e.store.Analytics(ctx, from, to, "")
	if err != nil || all.Traffic.Views != 3 || all.Traffic.Bounces != 1 {
		t.Fatalf("%+v %v", all, err)
	}
	for _, who := range []*tactor{nil, &sub} {
		code, _, _ := e.do(who, "portal.test", "GET", "/api/admin/analytics", nil, nil)
		if code != 401 && code != 403 {
			t.Fatal(code)
		}
	}
	code, body, _ := e.do(&admin, "portal.test", "GET", "/api/admin/analytics?site=http%3A%2F%2Fportal.test", nil, nil)
	if code != 200 || json.Unmarshal(body, &a) != nil || a.Traffic.Views != 2 {
		t.Fatalf("%d %s", code, body)
	}
	for _, q := range []string{"from=bad", "from=2026-01-01&to=2026-01-01", "from=2026-01-01&to=2026-12-01"} {
		code, _, _ := e.do(&admin, "portal.test", "GET", "/api/admin/analytics?"+q, nil, nil)
		if code != 400 {
			t.Fatal(q, code)
		}
	}
	if _, err = e.store.Pool.Exec(ctx, `UPDATE traffic_views SET created_at=now()-interval '91 days'`); err != nil {
		t.Fatal(err)
	}
	if err = e.store.PruneTraffic(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = e.store.Pool.QueryRow(ctx, "SELECT count(*) FROM traffic_views").Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}
func TestReleaseFeed(t *testing.T) {
	e := setup(t)
	admin := e.user("feed-admin", "admin", false)
	if code, out := e.upload(admin, testHTML); code != 201 {
		t.Fatal(code, out)
	}
	_, b, _ := e.do(nil, "portal.test", "GET", "/apps/feed.xml", nil, nil)
	if strings.Contains(string(b), "Board 1.0.0") {
		t.Fatal("draft leak")
	}
	rels, err := e.store.Releases(context.Background(), "board", "draft")
	if err != nil {
		t.Fatal(err)
	}
	code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rels[0].ID+"/publish", nil)
	if code != 200 {
		t.Fatal(code, out)
	}
	code, b, _ = e.do(nil, "boutique.test", "GET", "/apps/feed.xml", nil, nil)
	var doc rssDoc
	if err = xml.Unmarshal(b, &doc); err != nil || code != 200 || len(doc.Channel.Items) != 1 {
		t.Fatalf("%d %s %v", code, b, err)
	}
	if doc.Channel.Items[0].Link != "http://boutique.test/apps/board" || doc.Channel.Items[0].GUID.Permalink {
		t.Fatal(doc)
	}
	if _, err = e.store.Pool.Exec(context.Background(), `UPDATE catalog_artifacts SET visibility='hidden' WHERE id='board'`); err != nil {
		t.Fatal(err)
	}
	_, b, _ = e.do(nil, "portal.test", "GET", "/apps/feed.xml", nil, nil)
	if strings.Contains(string(b), "Board 1.0.0") {
		t.Fatal("unlisted leak")
	}
}
