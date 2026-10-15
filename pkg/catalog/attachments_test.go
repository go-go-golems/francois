package catalog

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestAttachments(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	sub := e.user("sue", "subscriber", false)
	free := e.user("fay", "subscriber", false)
	e.subscribe(sub, "price_test_pro")
	expect := func(what string, code int, out map[string]any, want int, wantCode string) {
		t.Helper()
		if code != want || (wantCode != "" && errCode(out) != wantCode) {
			t.Fatalf("%s: %d %v (want %d %s)", what, code, out, want, wantCode)
		}
	}

	// A draft release with attachments: only editors see and download them.
	code, rel := e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": "loom", "title": "Loom", "version": "1.0.0", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
	expect("draft", code, rel, 201, "")
	relID := rel["id"].(string)
	path := "/api/studio/releases/" + relID + "/attachments"
	code, man := e.multipartTo(admin, path, map[string]string{"label": "User manual"}, "file", "dir/../Manual für Loom.pdf", []byte("%PDF-1.4 the manual"))
	expect("attach", code, man, 201, "")
	if man["name"] != "Manual für Loom.pdf" || man["label"] != "User manual" || man["mediaType"] != "application/pdf" || man["position"].(float64) != 0 {
		t.Fatalf("attachment %v", man)
	}
	manID := man["id"].(string)
	code, zip := e.multipartTo(admin, path, nil, "file", "plates.zip", bytes.Repeat([]byte("z"), 3000))
	expect("attach 2", code, zip, 201, "")
	if zip["position"].(float64) != 1 {
		t.Fatalf("second position %v", zip)
	}
	code, out := e.multipartTo(cleo, path, nil, "file", "x.txt", []byte("x"))
	expect("non-editor attaches", code, out, 403, "not_editor")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/attachments/"+manID, map[string]string{"label": "mine"})
	expect("non-editor relabels", code, out, 403, "not_editor")
	pub := "/api/releases/" + relID + "/attachments/" + manID + "/download"
	if code, _, _ := e.do(&sub, "portal.test", "GET", pub, nil, nil); code != 404 {
		t.Fatalf("draft attachment public download %d", code)
	}
	code, body, h := e.do(&admin, "portal.test", "GET", "/api/studio/attachments/"+manID+"/download", nil, nil)
	if code != 200 || string(body) != "%PDF-1.4 the manual" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") || !strings.Contains(h.Get("Content-Disposition"), "filename*=utf-8''Manual%20f%C3%BCr%20Loom.pdf") {
		t.Fatalf("editor download %d %q %v", code, body, h)
	}

	// Published: the release's download rule applies.
	code, out = e.portal(&admin, "POST", "/api/studio/releases/"+relID+"/publish", nil)
	expect("publish", code, out, 200, "")
	code, out = e.portal(&admin, "PATCH", "/api/studio/artifacts/loom", map[string]string{"requiredPlan": "pro"})
	expect("require pro", code, out, 200, "")
	if code, _, _ := e.do(nil, "portal.test", "GET", pub, nil, nil); code != 401 {
		t.Fatalf("anonymous download %d", code)
	}
	if code, b, _ := e.do(&free, "portal.test", "GET", pub, nil, nil); code != 403 || !strings.Contains(string(b), "plan_required") {
		t.Fatalf("free plan download %d %s", code, b)
	}
	code, body, h = e.do(&sub, "portal.test", "GET", pub, nil, map[string]string{"Range": "bytes=0-3"})
	if code != 206 || string(body) != "%PDF" || h.Get("Content-Type") != "application/octet-stream" || h.Get("X-Content-SHA256") == "" {
		t.Fatalf("subscriber range download %d %q %v", code, body, h)
	}
	if code, _, _ := e.do(&sub, "portal.test", "GET", "/api/releases/"+relID+"/attachments/"+zip["id"].(string)+"x/download", nil, nil); code != 404 {
		t.Fatalf("unknown attachment %d", code)
	}

	// Published releases still accept changes to attachments.
	code, out = e.portal(&admin, "PATCH", "/api/studio/attachments/"+zip["id"].(string), map[string]any{"label": "Plate pack", "position": -1})
	if code != 200 || out["label"] != "Plate pack" {
		t.Fatalf("relabel %d %v", code, out)
	}
	_, page, _ := e.do(nil, "portal.test", "GET", "/apps/loom", nil, nil)
	for _, want := range []string{"Plate pack", "User manual", "plates.zip · 2 KB", "Attachments are included with the Pro plan."} {
		if !bytes.Contains(page, []byte(want)) {
			t.Fatalf("anonymous artifact page lacks %q", want)
		}
	}
	if bytes.Contains(page, []byte("/attachments/")) {
		t.Fatal("anonymous page links attachments")
	}
	if bytes.Index(page, []byte("Plate pack")) > bytes.Index(page, []byte("User manual")) {
		t.Fatal("position order ignored")
	}
	_, page, _ = e.do(&sub, "portal.test", "GET", "/apps/loom", nil, nil)
	if !bytes.Contains(page, []byte(`href="`+pub+`"`)) {
		t.Fatal("subscriber page lacks the download link")
	}
	_, api, _ := e.do(nil, "portal.test", "GET", "/api/artifacts/loom", nil, nil)
	if !bytes.Contains(api, []byte(`"label":"User manual"`)) || bytes.Contains(api, []byte("storage")) || bytes.Contains(api, []byte(admin.user.ID)) {
		t.Fatalf("public API %s", api)
	}
	code, out = e.portal(&admin, "DELETE", "/api/studio/attachments/"+manID, nil)
	expect("delete", code, out, 200, "")
	if code, _, _ := e.do(&sub, "portal.test", "GET", pub, nil, nil); code != 404 {
		t.Fatalf("deleted attachment %d", code)
	}

	// Limits: count per release, and size from the settings.
	for i := 0; i < 19; i++ {
		if code, out = e.multipartTo(admin, path, nil, "file", "n"+strconv.Itoa(i)+".txt", []byte("n")); code != 201 {
			t.Fatalf("attachment %d: %d %v", i, code, out)
		}
	}
	code, out = e.multipartTo(admin, path, nil, "file", "one-too-many.txt", []byte("n"))
	expect("21st attachment", code, out, 409, "too_many_attachments")
	code, d2 := e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": "loom", "version": "1.0.1", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
	expect("draft 2", code, d2, 201, "")
	doc := testDoc(100)
	doc.Limits.MaxAttachmentBytes = 1 << 20
	if _, err := e.store.SaveSettings(t.Context(), "", doc); err != nil {
		t.Fatal(err)
	}
	code, out = e.multipartTo(admin, "/api/studio/releases/"+d2["id"].(string)+"/attachments", nil, "file", "big.bin", make([]byte, 1<<20+1))
	expect("over the attachment limit", code, out, 413, "too_large")
	code, out = e.portal(&admin, "DELETE", "/api/studio/releases/"+d2["id"].(string), nil)
	expect("delete draft", code, out, 200, "")
}
