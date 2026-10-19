package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestEarlyAccessAcrossRunDownloadAndDemo(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	doc := testDoc(100, settings.Model{Selector: "test/fake"})
	doc.Publishing.EarlyAccessDays = 7
	doc.Demo.Enabled = true
	if _, err := e.store.SaveSettings(ctx, "", doc); err != nil {
		t.Fatal(err)
	}
	admin := e.user("early-admin", "admin", false)
	paid := e.user("early-paid", "subscriber", true)
	free := e.user("early-free", "subscriber", false)
	release := e.publishArtifact(admin, "early", "")
	a, err := e.store.Artifact(ctx, "early")
	if err != nil || a.EarlyAccessUntil == nil || time.Until(*a.EarlyAccessUntil) < 6*24*time.Hour {
		t.Fatalf("first publication period %+v %v", a, err)
	}
	initial := *a.EarlyAccessUntil
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/early", map[string]any{"demoEnabled": true}); code != 200 {
		t.Fatal(code, out)
	}
	for _, actor := range []tactor{free, paid} {
		want := 403
		if actor.user.ID == paid.user.ID {
			want = 201
		}
		if code, out := e.portal(&actor, "POST", "/api/instances", map[string]string{"releaseId": release}); code != want {
			t.Fatalf("create %s: %d %v", actor.user.ID, code, out)
		}
		want = 403
		if actor.user.ID == paid.user.ID {
			want = 200
		}
		if code, _, _ := e.do(&actor, "portal.test", "GET", downloadPath(t, e, release), nil, nil); code != want {
			t.Fatalf("download %s: %d", actor.user.ID, code)
		}
	}
	if code, out := e.portal(&free, "POST", "/api/demos", map[string]string{"artifactId": "early"}); code != 403 || errCode(out) != "early_access" {
		t.Fatalf("early demo %d %v", code, out)
	}
	// Free samples cannot bypass the artifact's explicit early-access window.
	if _, err = e.store.Pool.Exec(ctx, `UPDATE releases SET free_sample=true WHERE id=$1`, release); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := e.do(&free, "portal.test", "GET", downloadPath(t, e, release), nil, nil); code != 403 {
		t.Fatal("free sample bypassed early access", code)
	}
	// A later release never restarts the artifact's early-access clock.
	code, rel := e.release(admin, "early", "1.1.0")
	if code != 201 {
		t.Fatal(code, rel)
	}
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+rel["id"].(string)+"/publish", nil); code != 200 {
		t.Fatal(code, out)
	}
	a, err = e.store.Artifact(ctx, "early")
	if err != nil || !a.EarlyAccessUntil.Equal(initial) {
		t.Fatal("later release restarted early access", err)
	}
	past := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/early", map[string]string{"earlyAccessUntil": past}); code != 200 {
		t.Fatal(code, out)
	}
	if code, out := e.portal(&free, "POST", "/api/instances", map[string]string{"releaseId": release}); code != 201 {
		t.Fatalf("expired window create %d %v", code, out)
	}
	if code, _, _ := e.do(&free, "portal.test", "GET", downloadPath(t, e, release), nil, nil); code != 200 {
		t.Fatal("expired window download", code)
	}
	if code, out := e.portal(&free, "POST", "/api/demos", map[string]string{"artifactId": "early"}); code != 201 {
		t.Fatalf("expired window demo %d %v", code, out)
	}
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/early", map[string]string{"earlyAccessUntil": ""}); code != 200 {
		t.Fatal(code, out)
	}
	a, err = e.store.Artifact(ctx, "early")
	if err != nil || a.EarlyAccessUntil != nil {
		t.Fatal("clear early access", err)
	}
	// Direct date policy is independent of arbitrary named plans.
	until := time.Now().Add(time.Hour)
	a = store.Artifact{EarlyAccessUntil: &until}
	if (Entitlement{Source: SourceFree}).CanRun(a, doc) || !(Entitlement{Source: SourceSubscription}).CanRun(a, doc) {
		t.Fatal("early-access entitlement")
	}
}
