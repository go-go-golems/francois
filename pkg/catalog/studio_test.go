package catalog

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/pkg/errors"
)

// multipartTo posts a multipart form with one file to the portal.
func (e *env) multipartTo(a tactor, path string, fields map[string]string, field, name string, data []byte) (int, map[string]any) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if field != "" {
		f, _ := w.CreateFormFile(field, name)
		_, _ = f.Write(data)
	}
	_ = w.Close()
	code, b, _ := e.do(&a, "portal.test", "POST", path, &buf, map[string]string{"Content-Type": w.FormDataContentType()})
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return code, out
}

func (e *env) release(a tactor, artifact, version string) (int, map[string]any) {
	return e.multipartTo(a, "/api/studio/releases", map[string]string{"artifactId": artifact, "version": version, "manifestJson": `{"sample":{}}`}, "html", "a.html", []byte(testHTML))
}

func TestStudio(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	otto := e.user("otto", "creator", false)
	sam := e.user("sam", "subscriber", true)
	expect := func(what string, code int, out map[string]any, want int, wantCode string) {
		t.Helper()
		if code != want || (wantCode != "" && errCode(out) != wantCode) {
			t.Fatalf("%s: %d %v (want %d %s)", what, code, out, want, wantCode)
		}
	}

	code, out := e.portal(&sam, "GET", "/api/studio/artifacts", nil)
	expect("subscriber in studio", code, out, 403, "creator_required")
	code, out = e.portal(&cleo, "POST", "/api/studio/artifacts", map[string]string{"id": "loom", "title": "Loom"})
	expect("creator without profile", code, out, 409, "creator_profile_required")

	// Profiles: only admins create them and link accounts.
	code, out = e.portal(&cleo, "POST", "/api/studio/creators", map[string]any{"slug": "cleo", "name": "Cleo"})
	expect("creator creates profile", code, out, 403, "admin_required")
	code, cp := e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": "cleo", "name": "Cleo", "userId": cleo.user.ID})
	expect("admin creates profile", code, cp, 201, "")
	code, op := e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": "otto", "name": "Otto", "userId": otto.user.ID})
	expect("admin creates profile 2", code, op, 201, "")
	code, out = e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": "otto", "name": "Again"})
	expect("duplicate slug", code, out, 409, "slug_taken")
	cleoID, ottoID := cp["id"].(string), op["id"].(string)
	code, out = e.portal(&cleo, "PATCH", "/api/studio/creators/"+cleoID, map[string]any{"bio": "Makes looms."})
	expect("own profile edit", code, out, 200, "")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/creators/"+cleoID, map[string]any{"userId": otto.user.ID})
	expect("creator relinks profile", code, out, 400, "invalid_request")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/creators/"+cleoID, map[string]any{"links": []map[string]string{{"label": "x", "url": "javascript:alert(1)"}}})
	expect("javascript link", code, out, 400, "invalid_request")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/creators/"+ottoID, map[string]any{"bio": "hijack"})
	expect("other profile edit", code, out, 403, "not_editor")

	// Artifacts: a creator owns what they create.
	code, out = e.portal(&cleo, "POST", "/api/studio/artifacts", map[string]string{"id": "loom", "title": "Loom"})
	expect("create artifact", code, out, 201, "")
	code, out = e.portal(&cleo, "POST", "/api/studio/artifacts", map[string]string{"id": "loom", "title": "Loom"})
	expect("duplicate artifact", code, out, 409, "artifact_exists")
	_, list, _ := e.do(&otto, "portal.test", "GET", "/api/studio/artifacts", nil, nil)
	if bytes.Contains(list, []byte(`"loom"`)) {
		t.Fatal("other creator lists loom")
	}
	_, list, _ = e.do(&cleo, "portal.test", "GET", "/api/studio/artifacts", nil, nil)
	if !bytes.Contains(list, []byte(`"loom"`)) {
		t.Fatalf("owner list %s", list)
	}
	for _, req := range []struct{ method, path string }{{"GET", "/api/studio/artifacts/loom"}, {"PATCH", "/api/studio/artifacts/loom"}, {"PUT", "/api/studio/artifacts/loom/draft"}, {"GET", "/api/studio/releases?artifact=loom"}} {
		code, out = e.portal(&otto, req.method, req.path, map[string]any{"title": "x"})
		expect("other creator "+req.method+" "+req.path, code, out, 403, "not_editor")
	}
	code, out = e.portal(&cleo, "PATCH", "/api/studio/artifacts/loom", map[string]any{"creators": []string{ottoID}})
	expect("drop own credit", code, out, 400, "invalid_request")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/artifacts/loom", map[string]any{"description": "Weaves.", "tags": []string{"Tools", "tools", " art "}, "featured": true, "earlyAccessUntil": "2030-01-01T00:00:00Z"})
	if code != 200 || out["description"] != "Weaves." || len(out["tags"].([]any)) != 2 || out["earlyAccessUntil"] != "2030-01-01T00:00:00Z" {
		t.Fatalf("patch artifact %d %v", code, out)
	}
	code, out = e.portal(&cleo, "PATCH", "/api/studio/artifacts/loom", map[string]any{"earlyAccessUntil": "Bad Date"})
	expect("bad plan key", code, out, 400, "invalid_request")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/artifacts/loom", map[string]any{"earlyAccessUntil": "not-a-date"})
	expect("unknown plan", code, out, 400, "invalid_request")

	// Releases: editors upload, preview, publish; others cannot.
	code, rel := e.release(cleo, "loom", "1.0.0")
	expect("creator release", code, rel, 201, "")
	relID := rel["id"].(string)
	code, out = e.release(otto, "loom", "1.0.1")
	expect("other creator release", code, out, 403, "not_editor")
	code, out = e.portal(&otto, "POST", "/api/studio/releases/"+relID+"/publish", nil)
	expect("other creator publish", code, out, 403, "not_editor")
	code, preview := e.portal(&cleo, "POST", "/api/studio/releases/"+relID+"/preview", nil)
	expect("creator preview", code, preview, 200, "")
	if _, status, _ := e.launch(cleo, preview); status != 200 {
		t.Fatalf("creator preview launch %d", status)
	}
	code, out = e.portal(&otto, "POST", "/api/studio/releases/"+relID+"/preview", nil)
	expect("other creator preview", code, out, 403, "not_editor")
	code, out = e.portal(&cleo, "POST", "/api/studio/releases/"+relID+"/publish", nil)
	expect("creator publish", code, out, 200, "")
	code, out = e.release(otto, "fresh", "1.0.0")
	expect("new artifact without title", code, out, 422, "invalid_release")
	code, out = e.multipartTo(otto, "/api/studio/releases", map[string]string{"artifactId": "fresh", "title": "Fresh", "version": "1.0.0", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
	expect("new artifact by upload", code, out, 201, "")
	if ok, _ := e.store.IsArtifactCreator(t.Context(), "fresh", otto.user.ID); !ok {
		t.Fatal("upload of a new artifact did not credit its creator")
	}

	// Media and page drafts.
	png := tinyPNG(t)
	code, out = e.multipartTo(cleo, "/api/studio/media", map[string]string{"alt": "loom"}, "file", "a.png", png)
	expect("upload media", code, out, 201, "")
	mediaID := out["id"].(string)
	code, out = e.multipartTo(cleo, "/api/studio/media", nil, "file", "a.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	expect("svg media", code, out, 415, "media_type")
	code, out = e.portal(&cleo, "PUT", "/api/studio/artifacts/loom/draft", map[string]any{"body": "x", "cover": "med_nope"})
	expect("unknown cover", code, out, 400, "invalid_request")
	code, out = e.portal(&cleo, "PUT", "/api/studio/artifacts/loom/draft", map[string]any{"body": "## Weaving\n\n![loom](/media/" + mediaID + ")", "cover": mediaID})
	expect("save draft", code, out, 200, "")
	if _, b, _ := e.do(nil, "portal.test", "GET", "/apps/loom", nil, nil); bytes.Contains(b, []byte("Weaving")) {
		t.Fatal("draft visible before publish")
	}
	code, out = e.portal(&cleo, "POST", "/api/studio/artifacts/loom/draft/publish", nil)
	expect("publish draft", code, out, 200, "")
	code, out = e.portal(&cleo, "POST", "/api/studio/artifacts/loom/draft/publish", nil)
	expect("publish without draft", code, out, 409, "no_draft")
	if _, b, _ := e.do(nil, "portal.test", "GET", "/apps/loom", nil, nil); !bytes.Contains(b, []byte(`id="weaving"`)) {
		t.Fatalf("published page %s", b)
	}
	code, out = e.portal(&otto, "PATCH", "/api/studio/media/"+mediaID, map[string]string{"alt": "x"})
	expect("other uploader alt", code, out, 403, "not_uploader")
	code, out = e.portal(&cleo, "DELETE", "/api/studio/media/"+mediaID, nil)
	expect("delete used media", code, out, 409, "media_in_use")

	// Posts.
	code, post := e.portal(&cleo, "POST", "/api/studio/posts", map[string]any{"slug": "notes", "title": "Notes", "body": "Hello"})
	expect("create post", code, post, 201, "")
	if post["authorId"] != cleoID {
		t.Fatalf("post author %v", post["authorId"])
	}
	postID := post["id"].(string)
	code, out = e.portal(&cleo, "POST", "/api/studio/posts", map[string]any{"slug": "notes", "title": "Again"})
	expect("duplicate post slug", code, out, 409, "slug_taken")
	code, out = e.portal(&cleo, "PATCH", "/api/studio/posts/"+postID, map[string]any{"authorId": ottoID})
	expect("author as someone else", code, out, 400, "invalid_request")
	code, out = e.portal(&otto, "PATCH", "/api/studio/posts/"+postID, map[string]any{"title": "Mine"})
	expect("other creator edits post", code, out, 403, "not_editor")
	code, out = e.portal(&cleo, "POST", "/api/studio/posts/"+postID+"/publish", nil)
	if code != 200 || out["state"] != "published" {
		t.Fatalf("publish post %d %v", code, out)
	}
	_, ps, _ := e.do(&otto, "portal.test", "GET", "/api/studio/posts", nil, nil)
	if bytes.Contains(ps, []byte(postID)) {
		t.Fatal("other creator lists the post")
	}
	_, ps, _ = e.do(&admin, "portal.test", "GET", "/api/studio/posts", nil, nil)
	if !bytes.Contains(ps, []byte(postID)) {
		t.Fatal("admin misses the post")
	}

	// Markdown preview uses the public renderer.
	code, out = e.portal(&cleo, "POST", "/api/studio/markdown", map[string]string{"markdown": "# Hi <script>x</script>"})
	if code != 200 || !strings.Contains(out["html"].(string), `<h1 id="hi`) || strings.Contains(out["html"].(string), "<script>") {
		t.Fatalf("markdown %d %v", code, out)
	}

	// Administrators edit everything; studio routes are not admin routes.
	code, out = e.portal(&admin, "PATCH", "/api/studio/artifacts/loom", map[string]any{"visibility": "hidden"})
	expect("admin edits creator artifact", code, out, 200, "")
	code, out = e.portal(&cleo, "GET", "/api/admin/users", nil)
	expect("creator in admin API", code, out, 403, "admin_required")
}

// Two creators uploading the first release of the same new artifact at once:
// whoever creates it is credited, the other is refused as a non-editor and
// never replaces the credits.
func TestConcurrentNewArtifactUpload(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	cleo := e.user("cleo", "creator", false)
	otto := e.user("otto", "creator", false)
	profile := map[string]string{}
	for _, c := range []tactor{cleo, otto} {
		code, p := e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": c.user.DisplayName, "name": c.user.DisplayName, "userId": c.user.ID})
		if code != 201 {
			t.Fatalf("profile %d %v", code, p)
		}
		profile[c.user.ID] = p["id"].(string)
	}
	for round := 0; round < 8; round++ {
		id := "race-" + strconv.Itoa(round)
		racers := []tactor{cleo, otto}
		codes, outs := make([]int, 2), make([]map[string]any, 2)
		var wg sync.WaitGroup
		for i, c := range racers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], outs[i] = e.multipartTo(c, "/api/studio/releases", map[string]string{"artifactId": id, "title": "Race", "version": "1.0.0", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
			}()
		}
		wg.Wait()
		winner := -1
		for i := range racers {
			switch {
			case codes[i] == 201 && winner == -1:
				winner = i
			case codes[i] == 403 && errCode(outs[i]) == "not_editor":
			default:
				t.Fatalf("%s upload %d: %d %v", id, i, codes[i], outs[i])
			}
		}
		if winner == -1 {
			t.Fatalf("%s: nobody won %v", id, codes)
		}
		credits, err := e.store.ArtifactCreators(t.Context(), []string{id})
		if err != nil || len(credits[id]) != 1 || credits[id][0].ID != profile[racers[winner].user.ID] {
			t.Fatalf("%s credits %v %v (winner %s)", id, credits[id], err, racers[winner].user.DisplayName)
		}
	}
}

// Force the window between mayUpload's new-artifact decision and prepare's
// fresh lookup: another creator has already won by the time prepare runs.
func TestNewArtifactAuthorizationMustBeRechecked(t *testing.T) {
	e := setup(t)
	ctx := t.Context()
	admin := e.user("winner", "admin", false)
	e.publishArtifact(admin, "taken", "")
	u := uploadOf("taken", "Taken", "2.0.0")
	u.RequireNewArtifact = true
	_, _, err := e.srv.Publisher.CreateDraft(ctx, admin.user.ID, u)
	if !errors.Is(err, releases.ErrArtifactTaken) {
		t.Fatalf("stale new-artifact authorization was accepted: %v", err)
	}
	var count int
	err = e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM releases WHERE artifact_id=$1`, "taken").Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("stale attempt wrote a release: %d %v", count, err)
	}
}
