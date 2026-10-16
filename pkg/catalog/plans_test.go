package catalog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// subscribe projects an active Stripe subscription on price for the user.
func (e *env) subscribe(u tactor, price string) {
	e.t.Helper()
	until := time.Now().Add(30 * 24 * time.Hour)
	_, err := e.store.Pool.Exec(context.Background(), `INSERT INTO billing_subscriptions(stripe_subscription_id,user_id,stripe_customer_id,price_id,status,quantity,item_count,access_until)
VALUES($1,$2,$3,$4,'active',1,1,$5) ON CONFLICT(stripe_subscription_id) DO UPDATE SET price_id=EXCLUDED.price_id`, "sub_"+u.user.ID, u.user.ID, "cus_"+u.user.ID, price, until)
	if err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) publishArtifact(admin tactor, id, required string) string {
	e.t.Helper()
	code, rel := e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": id, "title": id, "version": "1.0.0", "manifestJson": `{"db":{}}`}, "html", "a.html", []byte(testHTML))
	if code != 201 {
		e.t.Fatalf("upload %s: %d %v", id, code, rel)
	}
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rel["id"].(string)+"/publish", nil); code != 200 {
		e.t.Fatalf("publish %s: %d %v", id, code, out)
	}
	if required != "" {
		if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/"+id, map[string]string{"requiredPlan": required}); code != 200 {
			e.t.Fatalf("plan %s: %d %v", id, code, out)
		}
	}
	return rel["id"].(string)
}

func TestPlans(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	doc := settings.Defaults()
	doc.Plans = append(doc.Plans,
		settings.Plan{Key: "supporter", Name: "Supporter", Rank: 10, Public: true, Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_sup"}}, Instances: 1, MembersPerInstance: 2},
		settings.Plan{Key: "studio", Name: "Studio", Rank: 20, Public: true, Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_studio"}}, Instances: 5, MembersPerInstance: 10},
	)
	if _, err := e.store.SaveSettings(ctx, "", doc); err != nil {
		t.Fatal(err)
	}
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	sup := e.user("sue", "subscriber", false)
	stu := e.user("stan", "subscriber", false)
	free := e.user("fay", "subscriber", false)
	e.subscribe(sup, "price_sup")
	e.subscribe(stu, "price_studio")
	basic := e.publishArtifact(admin, "basic", "")
	premium := e.publishArtifact(admin, "premium", "studio")
	create := func(a tactor, release string) (int, map[string]any) {
		return e.portal(&a, "POST", "/api/instances", map[string]string{"releaseId": release})
	}
	expect := func(what string, code int, out map[string]any, want int, wantCode string) {
		t.Helper()
		if code != want || (wantCode != "" && errCode(out) != wantCode) {
			t.Fatalf("%s: %d %v (want %d %s)", what, code, out, want, wantCode)
		}
	}

	// The session reports the resolved plan.
	_, s := e.portal(&sup, "GET", "/api/session", nil)
	if plan := s["plan"].(map[string]any); plan["key"] != "supporter" || plan["source"] != SourceSubscription {
		t.Fatalf("session plan %v", plan)
	}

	// Plan rank gates artifacts; the free plan runs nothing paid.
	code, out := create(free, basic)
	expect("free creates", code, out, 403, "plan_required")
	code, supInst := create(sup, basic)
	expect("supporter creates basic", code, supInst, 201, "")
	code, out = create(sup, premium)
	expect("supporter creates premium", code, out, 403, "plan_required")
	code, out = create(sup, basic)
	expect("supporter over instance limit", code, out, 409, "instance_limit")
	code, out = create(stu, premium)
	expect("studio creates premium", code, out, 201, "")
	code, out = create(cleo, premium)
	expect("staff create premium", code, out, 201, "")

	// Downloads follow the same rule.
	if code, _, _ := e.do(&sup, "portal.test", "GET", downloadPath(t, e, premium), nil, nil); code != 403 {
		t.Fatalf("supporter downloads premium: %d", code)
	}
	if code, _, _ := e.do(&stu, "portal.test", "GET", downloadPath(t, e, premium), nil, nil); code != 200 {
		t.Fatalf("studio downloads premium: %d", code)
	}

	// The owner's plan sets the member limit (supporter: owner + 1).
	code, out = e.portal(&sup, "POST", "/api/instances/"+supInst["id"].(string)+"/members", map[string]string{"email": "stan@example.com", "role": "viewer"})
	expect("first member", code, out, 200, "")
	code, out = e.portal(&sup, "POST", "/api/instances/"+supInst["id"].(string)+"/members", map[string]string{"email": "fay@example.com", "role": "viewer"})
	expect("member limit", code, out, 409, "member_limit")

	// A downgrade keeps data; workspaces beyond the new limit stop launching
	// and so do artifacts the new plan does not include.
	_, a1 := create(stu, basic)
	_, a2 := create(stu, basic)
	e.subscribe(stu, "price_sup")
	_, status, errc := e.launch(stu, a1) // second-oldest workspace of stan (premium is first)
	if status != 403 || errc != "over_plan_limit" {
		t.Fatalf("over-limit launch: %d %s", status, errc)
	}
	if _, status, errc = e.launch(stu, a2); status != 403 || errc != "over_plan_limit" {
		t.Fatalf("over-limit launch 2: %d %s", status, errc)
	}
	insts, _ := e.store.MemberInstances(ctx, stu.user.ID)
	if len(insts) < 3 {
		t.Fatalf("downgrade deleted instances: %d", len(insts))
	}
	e.subscribe(stu, "price_studio")
	if _, status, errc = e.launch(stu, a2); status != 200 {
		t.Fatalf("launch after upgrade: %d %s", status, errc)
	}

	// Complimentary access can name a plan.
	code, out = e.portal(&admin, "POST", "/api/admin/users/"+free.user.ID+"/access-grants", map[string]any{"days": 7, "reason": "friend", "plan": "supporter"})
	expect("grant supporter", code, out, 200, "")
	code, out = create(free, basic)
	expect("granted supporter creates basic", code, out, 201, "")
	code, out = create(free, premium)
	expect("granted supporter creates premium", code, out, 403, "plan_required")
	code, out = e.portal(&admin, "POST", "/api/admin/users/"+free.user.ID+"/access-grants", map[string]any{"days": 7, "reason": "x", "plan": "nope"})
	expect("grant unknown plan", code, out, 400, "invalid_request")

	// Pricing lists public plans and marks the caller's; the public API
	// never exposes price IDs.
	if code, b, _ := e.do(&sup, "portal.test", "GET", "/pricing", nil, nil); code != 200 || !strings.Contains(string(b), "Studio") || !strings.Contains(string(b), "Current plan") {
		t.Fatalf("pricing %d", code)
	}
	if _, b, _ := e.do(nil, "portal.test", "GET", "/api/plans", nil, nil); strings.Contains(string(b), "price_") || !strings.Contains(string(b), `"supporter"`) {
		t.Fatalf("public plans %s", b)
	}
	if _, b, _ := e.do(&sup, "portal.test", "GET", "/apps/premium", nil, nil); !strings.Contains(string(b), `data-rank="20"`) || !strings.Contains(string(b), `data-plan="Studio"`) || strings.Contains(string(b), "Studio plan") {
		t.Fatalf("artifact access metadata/minimal copy %s", b)
	}

	// Account shows the plan and its allowance.
	_, acct := e.portal(&free, "GET", "/api/account", nil)
	if ent := acct["entitlement"].(map[string]any); ent["source"] != SourceComplimentary || ent["plan"].(map[string]any)["key"] != "supporter" {
		t.Fatalf("account entitlement %v", ent)
	}
}

func downloadPath(t *testing.T, e *env, release string) string {
	t.Helper()
	r, err := e.store.Release(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	var f store.ReleaseFile
	for _, x := range r.Files {
		if x.Role == "html" {
			f = x
		}
	}
	return "/api/releases/" + release + "/files/" + f.ID + "/download"
}
