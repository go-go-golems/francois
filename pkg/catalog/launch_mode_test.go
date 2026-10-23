package catalog

import (
	"bytes"
	"testing"
)

// TestLaunchMode defaults artifacts to the tab mode, lets editors switch to
// inline, rejects other values and publishes the mode in the catalog API.
func TestLaunchMode(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	e.publishArtifact(admin, "board", "")
	code, out := e.portal(&admin, "GET", "/api/studio/artifacts/board", nil)
	if code != 200 || out["artifact"].(map[string]any)["launchMode"] != "tab" {
		t.Fatalf("default launch mode: %d %v", code, out)
	}
	if code, out = e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]any{"launchMode": "popup"}); code != 400 {
		t.Fatalf("unknown launch mode accepted: %d %v", code, out)
	}
	if code, out = e.portal(&admin, "PATCH", "/api/studio/artifacts/board", map[string]any{"launchMode": "inline"}); code != 200 || out["launchMode"] != "inline" {
		t.Fatalf("switch to inline: %d %v", code, out)
	}
	if _, cat, _ := e.do(nil, "portal.test", "GET", "/api/catalog", nil, nil); !bytes.Contains(cat, []byte(`"launchMode":"inline"`)) {
		t.Fatalf("catalog lacks the launch mode: %s", cat)
	}
}
