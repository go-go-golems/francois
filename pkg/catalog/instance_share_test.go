package catalog

import (
	"context"
	"net/http"
	"testing"
)

// TestSharedInstanceLinkFallsBackToTheApp: an instance link opened by
// someone who cannot use the instance leads to the application's page;
// members get the instance page, and unlisted applications are not revealed.
func TestSharedInstanceLinkFallsBackToTheApp(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin := e.user("editor", "admin", false)
	owner := e.user("olga", "subscriber", true)
	stranger := e.user("sam", "subscriber", true)
	rel, _, err := e.srv.Publisher.CreateDraft(ctx, admin.user.ID, uploadOf("loom", "Loom", "1.0.0"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.store.PublishRelease(ctx, rel.ID, admin.user.ID, 0); err != nil {
		t.Fatal(err)
	}
	inst, err := e.store.CreateInstance(ctx, owner.user.ID, rel.ID, "Mine", "workspace", 10)
	if err != nil {
		t.Fatal(err)
	}
	client := *e.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	e.client = &client
	page := "/instances/" + inst.ID
	for name, a := range map[string]*tactor{"signed out": nil, "not a member": &stranger} {
		if code, _, h := e.do(a, "portal.test", "GET", page, nil, nil); code != http.StatusFound || h.Get("Location") != "/apps/loom" {
			t.Fatalf("%s: %d %q", name, code, h.Get("Location"))
		}
	}
	if code, body, _ := e.do(&owner, "portal.test", "GET", page, nil, nil); code != 200 || len(body) == 0 {
		t.Fatalf("owner: %d", code)
	}
	for _, p := range []string{"/instances/ins_missing", "/instances/new"} {
		if code, _, _ := e.do(&stranger, "portal.test", "GET", p, nil, nil); code != 200 {
			t.Fatalf("%s: %d", p, code)
		}
	}
	a, err := e.store.Artifact(ctx, "loom")
	if err != nil {
		t.Fatal(err)
	}
	a.Visibility = "hidden"
	if _, err = e.store.UpdateArtifact(ctx, a); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := e.do(&stranger, "portal.test", "GET", page, nil, nil); code != 200 {
		t.Fatalf("hidden app redirected: %d", code)
	}
}
