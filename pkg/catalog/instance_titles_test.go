package catalog

import (
	"context"
	"testing"
)

func TestInstanceUsesCurrentArtifactTitle(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	code, rel := e.upload(admin, testHTML)
	if code != 201 {
		t.Fatalf("upload: %d %v", code, rel)
	}
	release := rel["id"].(string)
	if code, out := e.portal(&admin, "POST", "/api/studio/releases/"+release+"/publish", nil); code != 200 {
		t.Fatalf("publish: %d %v", code, out)
	}
	inst, err := e.store.CreateInstance(context.Background(), admin.user.ID, release, "My private workbook", "workspace", 10)
	if err != nil {
		t.Fatal(err)
	}
	if code, out := e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]string{"title": "Current public title"}); code != 200 {
		t.Fatalf("rename artifact: %d %v", code, out)
	}
	code, out := e.portal(&admin, "GET", "/api/instances/"+inst.ID, nil)
	if code != 200 || out["title"] != "My private workbook" || out["artifactTitle"] != "Current public title" {
		t.Fatalf("instance must expose current artifact title separately: %d %v", code, out)
	}
}
