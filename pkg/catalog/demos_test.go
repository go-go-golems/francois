package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// demoFrom starts a demo as an anonymous visitor from address addr and
// returns the visitor (with the session cookie the server set) and the
// response.
func (e *env) demoFrom(addr, artifact string) (tactor, int, map[string]any) {
	e.t.Helper()
	code, b, h := e.do(nil, "portal.test", "POST", "/api/demos", strings.NewReader(`{"artifactId":"`+artifact+`"}`),
		map[string]string{"Origin": "http://portal.test", "X-Real-Ip": addr})
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	var visitor tactor
	for _, c := range (&http.Response{Header: h}).Cookies() {
		if c.Name == "francois_session_dev" && c.Value != "" {
			visitor.token = c.Value
		}
	}
	if visitor.token != "" {
		_, s := e.portal(&visitor, "GET", "/api/session", nil)
		visitor.csrf, _ = s["csrfToken"].(string)
		if u, ok := s["user"].(map[string]any); ok {
			visitor.user.ID, _ = u["id"].(string)
			visitor.user.Role, _ = u["role"].(string)
		}
	}
	return visitor, code, out
}

func (e *env) call(g grant, v *view, id string) (int, map[string]any) {
	// Sequential allowance checks wait for the preceding asynchronous call's
	// concurrency slot; concurrency behavior itself is tested independently.
	deadline := time.Now().Add(time.Second)
	for {
		code, out := e.app(g, "POST", "/api/views/"+v.id+"/calls", map[string]any{"callId": id, "input": "hi", "options": map[string]string{"modelTier": "default"}}, map[string]string{"X-Francois-View": v.id})
		if code != 429 || errCode(out) != "rate_limited" || time.Now().After(deadline) {
			return code, out
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDemos(t *testing.T) {
	e := setup(t)
	e.srv.Config.TrustProxyHeaders = true
	ctx := context.Background()
	doc := testDoc(100, settings.Model{Selector: "test/fake", Label: "Fake", Credits: 1})
	doc.Models.Tiers = map[string]string{"default": "test/fake"}
	doc.Demo = settings.Demo{PerUserPerHour: 3, Enabled: true, TTLMinutes: 30, MaxActive: 3, PerAddressPerHour: 2, Credits: 2, DailyBudget: 3, Models: []string{}}
	if _, err := e.store.SaveSettings(ctx, "", doc); err != nil {
		t.Fatal(err)
	}
	admin := e.user("ada", "admin", false)
	sub := e.user("sam", "subscriber", true)
	release := e.publishArtifact(admin, "board", "")
	expect := func(what string, code int, out map[string]any, want int, wantCode string) {
		t.Helper()
		if code != want || (wantCode != "" && errCode(out) != wantCode) {
			t.Fatalf("%s: %d %v (want %d %s)", what, code, out, want, wantCode)
		}
	}

	_, code, out := e.demoFrom("10.0.0.1", "board")
	expect("artifact without demo", code, out, 403, "demo_disabled")
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]any{"demoEnabled": true}); code != 200 {
		t.Fatalf("enable demo: %d %v", code, out)
	}
	code, b, _ := e.do(nil, "portal.test", "POST", "/api/demos", strings.NewReader(`{"artifactId":"board"}`), map[string]string{"Origin": "https://evil.test"})
	if code != 403 || !strings.Contains(string(b), "invalid_origin") {
		t.Fatalf("cross-origin demo: %d %s", code, b)
	}

	// An anonymous visitor gets a guest session and an expiring instance.
	guest, code, out := e.demoFrom("10.0.0.1", "board")
	expect("guest demo", code, out, 201, "")
	if guest.token == "" || guest.user.Role != "guest" || out["expiresAt"] == nil {
		t.Fatalf("guest session %+v %v", guest, out)
	}
	demo := map[string]any{"id": out["id"]}
	g, status, errc := e.launch(guest, demo)
	if status != 200 {
		t.Fatalf("guest launch %d %s", status, errc)
	}
	// Guests have no plan: no workspaces, no studio, not in the user list.
	code, out = e.portal(&guest, "POST", "/api/instances", map[string]string{"releaseId": release})
	expect("guest workspace", code, out, 403, "plan_required")
	code, out = e.portal(&guest, "GET", "/api/studio/artifacts", nil)
	expect("guest studio", code, out, 403, "creator_required")
	if _, b, _ := e.do(&admin, "portal.test", "GET", "/api/admin/users", nil, nil); strings.Contains(string(b), `"guest"`) {
		t.Fatal("guest listed for administrators")
	}

	// The demo ledger allows demo.credits; the shared budget demo.dailyBudget.
	v := e.view(g, true)
	for i, want := range []int{202, 202, 402} {
		if code, out := e.call(g, v, "g"+string(rune('a'+i))); code != want {
			t.Fatalf("guest call %d: %d %v", i, code, out)
		}
	}
	// A signed-in subscriber's demo uses its own ledger, not the monthly one.
	code, out = e.portal(&sub, "POST", "/api/demos", map[string]string{"artifactId": "board"})
	expect("subscriber demo", code, out, 201, "")
	sg, status, _ := e.launch(sub, map[string]any{"id": out["id"]})
	if status != 200 {
		t.Fatalf("subscriber demo launch %d", status)
	}
	sv := e.view(sg, true)
	if code, out := e.call(sg, sv, "s1"); code != 202 {
		t.Fatalf("subscriber demo call: %d %v", code, out)
	}
	waitSettled(t, e, sub.user.ID, 1)
	if code, out := e.call(sg, sv, "s2"); code != 429 || errCode(out) != "demo_limit" {
		t.Fatalf("shared daily budget: %d %v", code, out)
	}
	if a, _ := e.store.AllowanceFor(ctx, sub.user.ID, 100, 0); a.Used != 0 || a.Reserved != 0 {
		t.Fatalf("demo charged the monthly allowance: %+v", a)
	}

	// Per-address rate limit, then the global cap on live demos.
	_, code, out = e.demoFrom("10.0.0.1", "board")
	expect("second demo from address", code, out, 201, "")
	_, code, out = e.demoFrom("10.0.0.1", "board")
	expect("rate limit", code, out, 429, "rate_limited")
	_, code, out = e.demoFrom("10.0.0.2", "board")
	expect("capacity", code, out, 429, "demo_limit")

	// Expired demos stop launching and are suspended by the sweep.
	if _, err := e.store.Pool.Exec(ctx, `UPDATE instances SET expires_at=now()-interval '1 minute' WHERE id=$1`, demo["id"]); err != nil {
		t.Fatal(err)
	}
	if _, status, errc = e.launch(guest, demo); status != 403 || errc != "demo_expired" {
		t.Fatalf("expired launch %d %s", status, errc)
	}
	ids, err := e.store.ExpireDemos(ctx, demoKeep)
	if err != nil || len(ids) != 1 || ids[0] != demo["id"] {
		t.Fatalf("expire %v %v", ids, err)
	}
	if inst, _ := e.store.Instance(ctx, demo["id"].(string)); inst.State != "suspended" {
		t.Fatalf("expired demo state %s", inst.State)
	}
	_, code, out = e.demoFrom("10.0.0.3", "board")
	expect("slot freed by expiry", code, out, 201, "")
}

// waitSettled waits until the user's model calls are no longer reserved.
func waitSettled(t *testing.T, e *env, user string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		calls, _ := e.store.ModelCalls(context.Background(), user, 10)
		done := 0
		for _, c := range calls {
			if c.State == "completed" {
				done++
			}
		}
		if done >= n {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("calls of %s did not settle", user)
}

// Concurrent demo reservations from different visitors, each on its own demo
// ledger, never overspend the shared daily budget.
func TestSharedDemoBudgetUnderConcurrency(t *testing.T) {
	e := setup(t)
	const limit, racers = 3, 12
	visitors := make([]tactor, racers)
	for i := range visitors {
		visitors[i] = e.user("visitor"+strconv.Itoa(i), "", false)
	}
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i, v := range visitors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.store.Reserve(t.Context(), store.ReserveRequest{UserID: v.user.ID, InstanceID: "inst" + strconv.Itoa(i), CallID: "c", Model: "test/fake",
				Credits: 1, Included: 2, Period: "demo:inst" + strconv.Itoa(i), Shared: true, SharedLimit: limit})
		}()
	}
	wg.Wait()
	reserved := 0
	for i, err := range errs {
		switch {
		case err == nil:
			reserved++
		case !errors.Is(err, store.ErrSharedBudget):
			t.Fatalf("visitor %d: %v", i, err)
		}
	}
	if reserved != limit {
		t.Fatalf("%d reservations against a shared budget of %d", reserved, limit)
	}
}
