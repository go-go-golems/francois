package catalog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

// tinyMP4 is an ISO-BMFF file header (ftyp box with an mp4 brand) padded to
// size bytes: enough for type sniffing and Range requests.
func tinyMP4(size int) []byte {
	b := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00isommp41")
	return append(b, bytes.Repeat([]byte{0x2a}, size-len(b))...)
}

func TestVideos(t *testing.T) {
	e := setup(t)
	admin := e.user("ada", "admin", false)
	expect := func(what string, code int, out map[string]any, want int, wantCode string) {
		t.Helper()
		if code != want || (wantCode != "" && errCode(out) != wantCode) {
			t.Fatalf("%s: %d %v (want %d %s)", what, code, out, want, wantCode)
		}
	}

	code, vid := e.multipartTo(admin, "/api/studio/media", map[string]string{"alt": "walkthrough"}, "file", "a.mp4", tinyMP4(4096))
	expect("upload video", code, vid, 201, "")
	if vid["mediaType"] != "video/mp4" || vid["size"].(float64) != 4096 {
		t.Fatalf("video media %v", vid)
	}
	videoID := vid["id"].(string)
	code, img := e.multipartTo(admin, "/api/studio/media", map[string]string{"alt": "still"}, "file", "a.png", tinyPNG(t))
	expect("upload image", code, img, 201, "")
	imageID := img["id"].(string)

	// Streaming: Range requests are answered from the file.
	code, b, h := e.do(nil, "portal.test", "GET", "/media/"+videoID, nil, map[string]string{"Range": "bytes=100-199"})
	if code != 206 || len(b) != 100 || h.Get("Content-Range") != "bytes 100-199/4096" || h.Get("ETag") == "" {
		t.Fatalf("range %d len=%d %v", code, len(b), h)
	}

	// Posters: only an image, only on a video.
	code, out := e.portal(&admin, "PATCH", "/api/studio/media/"+imageID, map[string]string{"poster": videoID})
	expect("poster on an image", code, out, 400, "invalid_poster")
	code, out = e.portal(&admin, "PATCH", "/api/studio/media/"+videoID, map[string]string{"poster": videoID})
	expect("video as poster", code, out, 400, "invalid_poster")
	code, out = e.portal(&admin, "PATCH", "/api/studio/media/"+videoID, map[string]string{"poster": imageID})
	expect("set poster", code, out, 200, "")
	if m, _ := e.store.Media(t.Context(), videoID); m.Poster != imageID || m.Alt != "walkthrough" {
		t.Fatalf("poster not saved or alt lost: %+v", m)
	}
	code, out = e.portal(&admin, "DELETE", "/api/studio/media/"+imageID, nil)
	expect("delete a poster in use", code, out, 409, "media_in_use")

	// Markdown embeds the video with its poster, in previews and pages.
	code, out = e.portal(&admin, "POST", "/api/studio/markdown", map[string]string{"markdown": "![walk](/media/" + videoID + ")"})
	if code != 200 || !strings.Contains(out["html"].(string), `<video src="/media/`+videoID+`" controls preload="metadata" playsinline poster="/media/`+imageID+`"`) {
		t.Fatalf("markdown video %d %v", code, out)
	}
	code, cr := e.portal(&admin, "POST", "/api/studio/creators", map[string]any{"slug": "ada", "name": "Ada", "userId": admin.user.ID})
	expect("profile", code, cr, 201, "")
	code, out = e.portal(&admin, "PATCH", "/api/studio/creators/"+cr["id"].(string), map[string]any{"avatar": videoID})
	expect("video avatar", code, out, 400, "invalid_request")
	code, out = e.release(admin, "film", "1.0.0")
	expect("release without title", code, out, 422, "")
	code, out = e.multipartTo(admin, "/api/studio/releases", map[string]string{"artifactId": "film", "title": "Film", "version": "1.0.0", "manifestJson": `{}`}, "html", "a.html", []byte(testHTML))
	expect("release", code, out, 201, "")
	code, out = e.portal(&admin, "POST", "/api/studio/releases/"+out["id"].(string)+"/publish", nil)
	expect("publish", code, out, 200, "")
	code, out = e.portal(&admin, "PUT", "/api/studio/artifacts/film/draft", map[string]any{"body": "Watch:\n\n![walk](/media/" + videoID + ")", "cover": videoID})
	expect("draft", code, out, 200, "")
	code, out = e.portal(&admin, "POST", "/api/studio/artifacts/film/draft/publish", nil)
	expect("publish page", code, out, 200, "")
	_, page, _ := e.do(nil, "portal.test", "GET", "/apps/film", nil, nil)
	for _, want := range []string{
		`<video src="/media/` + videoID + `" autoplay muted loop playsinline controls`,                                // cover
		`<video src="/media/` + videoID + `" controls preload="metadata" playsinline poster="/media/` + imageID + `"`, // body
	} {
		if !bytes.Contains(page, []byte(want)) {
			t.Fatalf("artifact page lacks %q", want)
		}
	}
	_, list, _ := e.do(nil, "portal.test", "GET", "/apps", nil, nil)
	poster := `<img src="/media/` + imageID + `?w=640&amp;v=2" srcset="/media/` + imageID + `?w=160&amp;v=2 160w, `
	if !bytes.Contains(list, []byte(poster)) || !bytes.Contains(list, []byte(`?w=1200&amp;v=2 1200w" sizes="(max-width: 900px) 100vw, 402px" alt="walkthrough"`)) {
		t.Fatalf("catalogue card does not use the poster")
	}

	// The upload limit comes from the settings.
	doc := settings.Defaults()
	doc.Limits.MaxMediaBytes = 1 << 20
	if _, err := e.store.SaveSettings(context.Background(), "", doc); err != nil {
		t.Fatal(err)
	}
	code, out = e.multipartTo(admin, "/api/studio/media", nil, "file", "big.mp4", tinyMP4(1<<20+4096))
	expect("over the media limit", code, out, 413, "too_large")
}
