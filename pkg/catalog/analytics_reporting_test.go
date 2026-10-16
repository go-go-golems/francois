package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestAnalyticsFiltersAndCompleteStacks(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("report-admin", "admin", false)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)
	// More categories than the browsing-table limit, with two days of data.
	_, err := e.store.Pool.Exec(ctx, `INSERT INTO traffic_views(created_at,site,path,visitor,session,referrer,device,browser)
 SELECT $1::timestamptz+(i%2)*interval '1 day','http://portal.test','/page/'||i,'v'||i,'s'||i,
 CASE WHEN i%2=0 THEN '' ELSE 'search.example' END,'desktop','other' FROM generate_series(1,110) i`, from)
	if err != nil {
		t.Fatal(err)
	}
	// A second portal must not leak into a site-filtered report.
	if err = e.store.RecordView(ctx, store.TrafficView{Site: "http://boutique.test", Path: "/page/2", Visitor: "other", Session: "other"}); err != nil {
		t.Fatal(err)
	}
	a, err := e.store.Analytics(ctx, from, to, store.TrafficFilter{Site: "http://portal.test"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Traffic.Views != 110 || len(a.Groups["pages"]) != 100 {
		t.Fatalf("%+v pages=%d", a.Traffic, len(a.Groups["pages"]))
	}
	for _, name := range []string{"pages", "referrers"} {
		perDay := map[string]int64{}
		keys := map[string]bool{}
		var total int64
		for _, p := range a.Series[name] {
			perDay[p.Day] += p.Views
			keys[fmt.Sprint(p.Other)+p.Key] = true
			total += p.Views
		}
		if total != 110 || len(keys) > 4 {
			t.Fatalf("%s total=%d keys=%v", name, total, keys)
		}
		for _, day := range a.Groups["daily"] {
			if day.Views != perDay[day.Key] {
				t.Fatalf("daily mismatch %s", name)
			}
		}
	}
	var other int64
	for _, p := range a.Series["pages"] {
		if p.Other {
			other += p.Views
		}
	}
	if other != 107 {
		t.Fatalf("remainder lost categories: %d", other)
	}
	direct := ""
	for _, tc := range []struct {
		f    store.TrafficFilter
		want int64
	}{
		{store.TrafficFilter{Referrer: &direct}, 55},
		{store.TrafficFilter{Path: "/page/2", Referrer: &direct}, 1},
		{store.TrafficFilter{Path: "/page/1", Referrer: &direct}, 0},
	} {
		a, err = e.store.Analytics(ctx, from, to, tc.f)
		if err != nil || a.Traffic.Views != tc.want {
			t.Fatalf("%+v: %+v %v", tc.f, a.Traffic, err)
		}
		var total int64
		for _, p := range a.Series["pages"] {
			total += p.Views
		}
		if total != tc.want {
			t.Fatal(total)
		}
	}
	// Half-open range includes the first midnight and excludes the second day's midnight.
	a, err = e.store.Analytics(ctx, from, from.AddDate(0, 0, 1), store.TrafficFilter{})
	if err != nil || a.Traffic.Views != 55 {
		t.Fatal(a.Traffic, err)
	}
	q := url.Values{"from": {"2026-10-01"}, "to": {"2026-10-03"}, "referrer": {""}, "path": {"/page/2"}}
	code, body, _ := e.do(&admin, "portal.test", "GET", "/api/admin/analytics?"+q.Encode(), nil, nil)
	if code != 200 || json.Unmarshal(body, &a) != nil || a.Traffic.Views != 1 || a.Referrer == nil || *a.Referrer != "" {
		t.Fatalf("%d %s", code, body)
	}
	for _, q := range []string{"path=not-a-path", "path=%2Fapps%3Ftoken%3Dsecret", "referrer=https%3A%2F%2Fsearch.example", "path=" + url.QueryEscape("/"+string(make([]byte, 201)))} {
		code, _, _ := e.do(&admin, "portal.test", "GET", "/api/admin/analytics?"+q, nil, nil)
		if code != 400 {
			t.Fatal(q, code)
		}
	}
}

func TestArtifactUsageSurvivesDeletionAndMigration(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("artifact-admin", "admin", false)
	if code, out := e.upload(admin, testHTML); code != 201 {
		t.Fatal(code, out)
	}
	rels, err := e.store.Releases(ctx, "board", "draft")
	if err != nil {
		t.Fatal(err)
	}
	// Store creation uses a valid release relationship; publication isn't relevant to this ledger fixture.
	inst, err := e.store.CreateInstance(ctx, admin.user.ID, rels[0].ID, "Usage fixture", "workspace", 10)
	if err != nil {
		t.Fatal(err)
	}
	reserve := func(call, instance, artifact string) {
		t.Helper()
		r, err := e.store.Reserve(ctx, store.ReserveRequest{UserID: admin.user.ID, InstanceID: instance, ArtifactID: artifact, CallID: call, Model: "fake/model", Credits: 2, Included: 100})
		if err != nil {
			t.Fatal(err)
		}
		if err = e.store.StartCall(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err = e.store.SettleCall(ctx, r, true, "", store.CallUsage{Input: 10, Output: 5, CacheRead: 3, Cost: 0.01}); err != nil {
			t.Fatal(err)
		}
	}
	reserve("legacy-known", inst.ID, "")
	reserve("legacy-orphan", "already-deleted", "")
	// Reapply the actual migration to data in its pre-010 shape, not a copy of the SQL.
	if _, err = e.store.Pool.Exec(ctx, `ALTER TABLE model_calls DROP COLUMN artifact_id; DELETE FROM schema_migrations WHERE name='010_model_call_artifacts.sql'`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	reserve("new-snapshot", inst.ID, inst.ArtifactID)
	if err = e.store.DeleteInstance(ctx, inst.ID); err != nil {
		t.Fatal(err)
	}
	a, err := e.store.Analytics(ctx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), store.TrafficFilter{})
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]store.ModelGroup{}
	for _, g := range a.Models["artifacts"] {
		groups[g.Key] = g
	}
	if groups["board"].Calls != 2 || groups["board"].Credits != 4 || groups["board"].Input != 20 || groups["board"].CacheRead != 6 || groups["board"].Label != "Board" || groups[""].Calls != 1 {
		t.Fatalf("%+v", groups)
	}
}
