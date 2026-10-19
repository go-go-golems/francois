package catalog

import (
	"context"
	"encoding/json"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestDailyTokensReserveSettleOverrideAndRecovery(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	u := e.user("tokens", "subscriber", false)
	admin := e.user("token-admin", "admin", false)
	reserve := func(call string, n int) (store.Reservation, error) {
		return e.store.Reserve(ctx, store.ReserveRequest{UserID: u.user.ID, InstanceID: "fixture", CallID: call, Model: "test/fake", Tokens: n, Limit: 100})
	}
	const racers = 8
	var group errgroup.Group
	rs := make(chan store.Reservation, racers)
	for i := 0; i < racers; i++ {
		group.Go(func() error {
			r, err := reserve(store.NewID("test_"), 30)
			if err == nil {
				rs <- r
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}
	close(rs)
	total := 0
	var reservations []store.Reservation
	for r := range rs {
		total += r.Tokens
		reservations = append(reservations, r)
	}
	if total != 100 {
		t.Fatalf("atomic reservation total %d", total)
	}
	for _, r := range reservations {
		if err := e.store.SettleCall(ctx, r, false, "unstarted", store.CallUsage{}); err != nil {
			t.Fatal(err)
		}
	}
	r, err := reserve("measured", 80)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.store.StartCall(ctx, r); err != nil {
		t.Fatal(err)
	}
	usage := store.CallUsage{Input: 1000, Output: 20, CacheRead: 900, Rounds: 1}
	if err = e.store.SettleCall(ctx, r, true, "", usage); err != nil {
		t.Fatal(err)
	}
	if err = e.store.SettleCall(ctx, r, true, "", usage); err != nil {
		t.Fatal(err)
	}
	a, err := e.store.AllowanceFor(ctx, u.user.ID, 100)
	if err != nil || a.Used != 20 || a.Remaining != 80 || a.Reserved != 0 {
		t.Fatalf("measured settlement %+v %v", a, err)
	}
	limit := 25
	if err = e.store.SetTokenCeiling(ctx, u.user.ID, admin.user.ID, &limit, "temporary ceiling"); err != nil {
		t.Fatal(err)
	}
	r, err = reserve("partial", 80)
	if err != nil || r.Tokens != 5 {
		t.Fatalf("partial budget %+v %v", r, err)
	}
	if err = e.store.StartCall(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, _, err = e.store.RecoverCalls(ctx); err != nil {
		t.Fatal(err)
	}
	a, err = e.store.AllowanceFor(ctx, u.user.ID, 100)
	if err != nil || a.Used != 25 || a.Remaining != 0 || a.Reserved != 0 {
		t.Fatalf("unknown recovery %+v %v", a, err)
	}
	if err = e.store.SetTokenCeiling(ctx, u.user.ID, admin.user.ID, nil, "reset"); err != nil {
		t.Fatal(err)
	}
	a, err = e.store.AllowanceFor(ctx, u.user.ID, 100)
	if err != nil || a.Override != nil || a.Limit != 100 || a.Used != 25 {
		t.Fatalf("reset %+v %v", a, err)
	}
	now := time.Date(2030, 12, 31, 23, 59, 0, 0, time.UTC)
	if store.Period(now) != "2030-12-31" || !store.PeriodReset(now).Equal(time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("UTC daily boundary")
	}
}

func TestAdminSummaryExplainsEffectiveAccess(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("summary-admin", "admin", false)
	free := e.user("summary-free", "subscriber", false)
	doc := testDoc(100, settings.Model{Selector: "test/fake"})
	doc.Models.Tiers = map[string]string{"default": "test/fake"}
	if _, err := e.store.SaveSettings(ctx, admin.user.ID, doc); err != nil {
		t.Fatal(err)
	}
	get := func() map[string]any {
		t.Helper()
		code, raw := e.portal(&admin, "GET", "/api/admin/users/"+free.user.ID, nil)
		if code != 200 {
			t.Fatalf("detail %d %v", code, raw)
		}
		return raw["summary"].(map[string]any)
	}
	c := get()
	if c["class"] != "Free" || c["enabled"] != true || c["ai"].(map[string]any)["eligible"] != true {
		t.Fatalf("free summary %v", c)
	}
	zero := 0
	if err := e.store.SetTokenCeiling(ctx, free.user.ID, admin.user.ID, &zero, "stop AI"); err != nil {
		t.Fatal(err)
	}
	c = get()
	if c["ai"].(map[string]any)["reason"] != "Daily generated-token ceiling reached" {
		t.Fatalf("ceiling reason %v", c)
	}
	doc.Operations.SamplingEnabled = false
	if _, err := e.store.SaveSettings(ctx, admin.user.ID, doc); err != nil {
		t.Fatal(err)
	}
	c = get()
	if c["ai"].(map[string]any)["reason"] != "AI is paused for everyone" {
		t.Fatalf("pause precedence %v", c)
	}
	paid := e.user("summary-paid-disabled", "subscriber", true)
	if code, out := e.portal(&admin, "PATCH", "/api/admin/users/"+paid.user.ID, map[string]bool{"disabled": true}); code != 200 {
		t.Fatal(code, out)
	}
	code, out := e.portal(&admin, "GET", "/api/admin/users/"+paid.user.ID, nil)
	if code != 200 {
		t.Fatal(code, out)
	}
	paidSummary := out["summary"].(map[string]any)
	if paidSummary["class"] != "Paying" || paidSummary["enabled"] != false || paidSummary["ai"].(map[string]any)["reason"] != "Account disabled" {
		t.Fatalf("disabled paying customer %v", paidSummary)
	}
	if _, err := e.store.Pool.Exec(ctx, `ALTER TABLE daily_token_usage RENAME TO unavailable_usage`); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.portal(&admin, "GET", "/api/admin/users/"+free.user.ID, nil); code != 503 {
		t.Fatalf("failed quota query masqueraded as valid state: %d", code)
	}
}

func TestTokenPolicyMigrationPreservesHistory(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	doc := testDoc(10, settings.Model{Selector: "test/fake"})
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err = json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	for _, x := range legacy["plans"].([]any) {
		p := x.(map[string]any)
		delete(p, "dailyTokens")
		p["monthlyCredits"] = 10
		p["dailyCredits"] = 2
	}
	allowance := legacy["allowance"].(map[string]any)
	delete(allowance, "maxOutputTokens")
	models := legacy["models"].(map[string]any)["allowed"].([]any)
	models[0].(map[string]any)["credits"] = 1
	demo := legacy["demo"].(map[string]any)
	delete(demo, "tokens")
	demo["credits"] = 5
	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.Pool.Exec(ctx, `DELETE FROM platform_settings`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.Pool.Exec(ctx, `INSERT INTO platform_settings(document)VALUES($1)`, raw); err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.Pool.Exec(ctx, `DROP TABLE daily_token_usage,token_ceiling_overrides; ALTER TABLE catalog_artifacts DROP COLUMN early_access_until; ALTER TABLE catalog_artifacts ADD COLUMN required_plan text NOT NULL DEFAULT ''; ALTER TABLE model_calls DROP COLUMN reserved_tokens,DROP COLUMN charged_tokens; DELETE FROM schema_migrations WHERE name IN('017_daily_tokens.sql','018_early_access.sql')`); err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	active, err := e.store.ActiveSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.Document.Free().DailyTokens != 10000 || active.Document.Paying().DailyTokens != 100000 || active.Document.Allowance.MaxOutputTokens != 8192 {
		t.Fatalf("migration defaults %+v", active.Document)
	}
}
