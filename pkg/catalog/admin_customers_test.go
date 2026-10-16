package catalog

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestCustomerManagement covers the FRANCOIS-007 phase 2 admin endpoints.
func TestCustomerManagement(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	ada := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	sam := e.user("sam", "subscriber", false)

	// Revoke one complimentary grant, keep the other.
	_, first := e.portal(&ada, "POST", "/api/admin/users/"+sam.user.ID+"/access-grants", map[string]any{"days": 30, "reason": "beta"})
	_, second := e.portal(&ada, "POST", "/api/admin/users/"+sam.user.ID+"/access-grants", map[string]any{"days": 60, "reason": "press", "plan": "pro"})
	if code, out := e.portal(&ada, "DELETE", "/api/admin/access-grants/"+first["id"].(string), nil); code != 200 || out["revokedAt"] == nil {
		t.Fatalf("revoke one grant %d %v", code, out)
	}
	if code, _ := e.portal(&ada, "DELETE", "/api/admin/access-grants/"+first["id"].(string), nil); code != 404 {
		t.Fatalf("revoking a revoked grant: %d", code)
	}
	grants, _ := e.store.AccessGrants(ctx, sam.user.ID)
	live := 0
	for _, g := range grants {
		if g.RevokedAt == nil {
			live++
			if g.ID != second["id"] {
				t.Fatalf("wrong grant left live: %+v", g)
			}
		}
	}
	if live != 1 {
		t.Fatalf("live grants %d", live)
	}

	// Subscriptions across customers, with plan and email.
	e.subscribe(sam, "price_test_pro")
	code, raw, _ := e.do(&ada, "portal.test", "GET", "/api/admin/subscriptions?status=active", nil, nil)
	var subs []struct {
		Subscription        struct{ ID, Status string }
		UserID, Email, Plan string
	}
	if code != 200 || json.Unmarshal(raw, &subs) != nil || len(subs) != 1 || subs[0].Plan != "pro" || subs[0].Email != "sam@example.com" || subs[0].UserID != sam.user.ID {
		t.Fatalf("subscriptions %d %s", code, raw)
	}
	// Without Stripe configured, cancellation is unavailable rather than faked.
	if code, out := e.portal(&ada, "POST", "/api/admin/subscriptions/"+subs[0].Subscription.ID+"/cancel", nil); code != 503 || errCode(out) != "billing_unavailable" {
		t.Fatalf("cancel without billing %d %v", code, out)
	}

	// User filters.
	code, raw, _ = e.do(&ada, "portal.test", "GET", "/api/admin/users?role=creator", nil, nil)
	if code != 200 || !strings.Contains(string(raw), cleo.user.ID) || strings.Contains(string(raw), sam.user.ID) {
		t.Fatalf("role filter %d %s", code, raw)
	}
	// Staff resolve to the highest plan, so combine plan and role to find paying customers.
	code, raw, _ = e.do(&ada, "portal.test", "GET", "/api/admin/users?plan=pro&role=subscriber", nil, nil)
	if code != 200 || !strings.Contains(string(raw), sam.user.ID) || strings.Contains(string(raw), `"`+cleo.user.ID+`"`) {
		t.Fatalf("plan filter %d %s", code, raw)
	}

	// Admin instance deletion.
	rel := e.publishArtifact(ada, "polis", "")
	code, inst := e.portal(&sam, "POST", "/api/instances", map[string]string{"releaseId": rel})
	if code != 201 {
		t.Fatalf("instance %d %v", code, inst)
	}
	if code, out := e.portal(&ada, "DELETE", "/api/admin/instances/"+inst["id"].(string), nil); code != 200 {
		t.Fatalf("admin delete instance %d %v", code, out)
	}
	if code, _ := e.portal(&ada, "DELETE", "/api/admin/instances/"+inst["id"].(string), nil); code != 404 {
		t.Fatal("deleting a deleted instance")
	}

	// Audit filters: actor, action and paging.
	code, raw, _ = e.do(&ada, "portal.test", "GET", "/api/admin/audit?actor="+ada.user.ID+"&action=instance.delete", nil, nil)
	var audit []struct {
		ID      int64
		Action  string
		Subject string
	}
	if code != 200 || json.Unmarshal(raw, &audit) != nil || len(audit) != 1 || audit[0].Subject != inst["id"] {
		t.Fatalf("audit filter %d %s", code, raw)
	}
	code, raw, _ = e.do(&ada, "portal.test", "GET", "/api/admin/audit?actor="+ada.user.ID+"&limit=2", nil, nil)
	var page []struct{ ID int64 }
	if code != 200 || json.Unmarshal(raw, &page) != nil || len(page) != 2 {
		t.Fatalf("audit page %d %s", code, raw)
	}
	code, raw, _ = e.do(&ada, "portal.test", "GET", "/api/admin/audit?limit=500&before="+strconv.FormatInt(page[1].ID, 10), nil, nil)
	var older []struct{ ID int64 }
	if code != 200 || json.Unmarshal(raw, &older) != nil || len(older) == 0 || older[0].ID >= page[1].ID {
		t.Fatalf("audit before %d %s", code, raw)
	}
}

// TestSettingsIfMatch checks that a settings edit cannot undo a concurrent one.
func TestSettingsIfMatch(t *testing.T) {
	e := setup(t)
	ada := e.user("ada", "admin", false)
	code, raw, h := e.do(&ada, "portal.test", "GET", "/api/admin/settings", nil, nil)
	if code != 200 || h.Get("ETag") == "" {
		t.Fatalf("settings %d etag %q", code, h.Get("ETag"))
	}
	etag := h.Get("ETag")
	var cur struct {
		Active struct {
			Revision int64
			Document map[string]any
		}
	}
	if err := json.Unmarshal(raw, &cur); err != nil {
		t.Fatal(err)
	}
	doc, _ := json.Marshal(cur.Active.Document)
	patch := func(ifMatch string) (int, string, string) {
		headers := map[string]string{}
		if ifMatch != "" {
			headers["If-Match"] = ifMatch
		}
		code, b, h := e.do(&ada, "portal.test", "PATCH", "/api/admin/settings", strings.NewReader(string(doc)), headers)
		return code, string(b), h.Get("ETag")
	}
	if code, b, _ := patch(""); code != 428 || !strings.Contains(b, "revision_required") {
		t.Fatalf("no If-Match %d %s", code, b)
	}
	code, b, next := patch(etag)
	if code != 200 || next == "" || next == etag {
		t.Fatalf("save %d %s etag %q", code, b, next)
	}
	// A second editor still holding the old revision is refused.
	if code, b, _ := patch(etag); code != 412 || !strings.Contains(b, "settings_conflict") {
		t.Fatalf("stale save %d %s", code, b)
	}
	if code, b, _ := patch(next); code != 200 {
		t.Fatalf("save from the new revision %d %s", code, b)
	}
	// The unconditional store path (operator CLI) still works.
	if _, err := e.store.SaveSettings(context.Background(), "", testDoc(50)); err != nil {
		t.Fatal(err)
	}
}
