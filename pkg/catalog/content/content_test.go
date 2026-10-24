package content

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestMarkdown(t *testing.T) {
	md := NewMarkdown()
	cases := []struct {
		name, in string
		want     []string
		absent   []string
	}{
		{"gfm table", "| a | b |\n|---|---|\n| 1 | 2 |", []string{"<table>", "<td>1</td>"}, nil},
		{"heading id", "## How it works", []string{`<h2 id="how-it-works">`}, nil},
		{"raw html dropped", "hello <script>alert(1)</script> <b>x</b>", []string{"hello"}, []string{"<script", "<b>"}},
		{"html block dropped", "<div onclick=\"x()\">hi</div>", nil, []string{"<div", "onclick"}},
		{"javascript link dropped", "[x](javascript:alert(1))", nil, []string{"javascript:"}},
		{"media image", "![A cover](/media/med_abc)", []string{`<img src="/media/med_abc" alt="A cover" loading="lazy">`}, nil},
		{"external image becomes link", "![remote](https://example.com/a.png)", []string{`<a href="https://example.com/a.png">remote</a>`}, []string{"<img"}},
		{"nested media path is not media", "![x](/media/a/../../admin)", nil, []string{"<img"}},
		{"javascript image dropped", "![x](javascript:alert(1))", []string{"x"}, []string{"javascript:", "<img", "<a"}},
		{"alt escaped", "![\"><script>](/media/med_1)", nil, []string{"<script>"}},
		{"video with poster", "![A walkthrough](/media/med_vid)", []string{`<video src="/media/med_vid" controls preload="metadata" playsinline poster="/media/med_img" aria-label="A walkthrough"></video>`}, []string{"<img"}},
		{"video without poster", "![](/media/med_vid2)", []string{`<video src="/media/med_vid2"`}, []string{"poster="}},
		{"image stays image", "![x](/media/med_img)", []string{`<img src="/media/med_img"`}, []string{"<video"}},
		{"unknown media stays image", "![x](/media/med_gone)", []string{`<img src="/media/med_gone"`}, nil},
		{"video alt escaped", "![\"><script>](/media/med_vid)", nil, []string{"<script>"}},
	}
	embeds := map[string]Embed{"med_vid": {Type: "video/mp4", Poster: "med_img"}, "med_vid2": {Type: "video/webm"}, "med_img": {Type: "image/png"}}
	for _, c := range cases {
		out := string(md.Render(c.in, embeds))
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: %q missing from %q", c.name, w, out)
			}
		}
		for _, a := range c.absent {
			if strings.Contains(out, a) {
				t.Errorf("%s: %q present in %q", c.name, a, out)
			}
		}
	}
}

func TestMediaRefs(t *testing.T) {
	got := MediaRefs("![a](/media/med_a) [b](/media/med_b) ![a again](/media/med_a) /media/other")
	if strings.Join(got, ",") != "med_a,med_b" {
		t.Fatalf("refs %v", got)
	}
	if out := string(NewMarkdown().Render("![v](/media/med_v)", nil)); !strings.Contains(out, "<img") {
		t.Fatalf("nil embeds should render images: %s", out)
	}
}

func TestExcerpt(t *testing.T) {
	if got := Excerpt("# Title\n\nsecond", 50); got != "Title" {
		t.Fatalf("excerpt %q", got)
	}
	if got := Excerpt("**a** "+strings.Repeat("word ", 40), 20); len([]rune(got)) > 20 || !strings.HasSuffix(got, "…") {
		t.Fatalf("excerpt %q", got)
	}
}

func TestDetectMedia(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.Black)
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	info, err := DetectMedia(bytes.NewReader(buf.Bytes()))
	if err != nil || info.Type != "image/png" || info.Width != 3 || info.Height != 2 {
		t.Fatalf("png: %+v %v", info, err)
	}
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	if info, err = DetectMedia(bytes.NewReader(webp)); err != nil || info.Type != "image/webp" {
		t.Fatalf("webp: %+v %v", info, err)
	}
	mp4 := append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), make([]byte, 32)...)
	if info, err = DetectMedia(bytes.NewReader(mp4)); err != nil || info.Type != "video/mp4" || !IsVideo(info.Type) {
		t.Fatalf("mp4: %+v %v", info, err)
	}
	for name, data := range map[string][]byte{
		"svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":  []byte(`<!doctype html><script>alert(1)</script>`),
		"text":  []byte("hello"),
		"empty": nil,
	} {
		if _, err := DetectMedia(bytes.NewReader(data)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := DetectMedia(bytes.NewReader([]byte("\x89PNG\r\n\x1a\n broken"))); err == nil {
		t.Error("broken png accepted")
	}
}

func TestEditorialSectionsKeepMarkdownSafety(t *testing.T) {
	source := "Introduction.\n\n## What it is\n\nA **board**.\n\n![clip](/media/med_vid)\n\n## Why we made it\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(1))\n\n## Other heading\n\nDetails.\n\n## Sources\n\n[Reference](https://example.com)"
	sections := NewMarkdown().Sections(source, map[string]Embed{"med_vid": {Type: "video/mp4"}})
	if len(sections) != 4 || sections[0].Label != "" || sections[1].Label != "What it is" || sections[2].Label != "Why we made it" || sections[3].Label != "Sources" {
		t.Fatalf("sections %+v", sections)
	}
	if !strings.Contains(string(sections[1].HTML), "<strong>board</strong>") || !strings.Contains(string(sections[1].HTML), "<video") {
		t.Fatalf("lost markdown/media: %s", sections[1].HTML)
	}
	if strings.Contains(string(sections[2].HTML), "<script") || strings.Contains(string(sections[2].HTML), "javascript:") || !strings.Contains(string(sections[2].HTML), `<h2 id="other-heading">`) {
		t.Fatalf("unsafe or missing supplemental content: %s", sections[2].HTML)
	}
}

// TestCardCrops: a card keeps the middle of the image and crops the rest
// evenly, tall or wide.
func TestCardCrops(t *testing.T) {
	bands := func(w, h int, vertical bool) []byte {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := y * 3 / h
				if vertical {
					i = x * 3 / w
				}
				img.Set(x, y, []color.Color{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}}[i])
			}
		}
		var b bytes.Buffer
		_ = png.Encode(&b, img)
		return b.Bytes()
	}
	for name, src := range map[string][]byte{"tall": bands(400, 1200, false), "wide": bands(3000, 400, true)} {
		data, err := Card(bytes.NewReader(src))
		if err != nil {
			t.Fatal(name, err)
		}
		img, format, err := image.Decode(bytes.NewReader(data))
		if err != nil || format != "jpeg" || img.Bounds().Dx() != CardWidth || img.Bounds().Dy() != CardHeight {
			t.Fatalf("%s: %v %s %v", name, err, format, img.Bounds())
		}
		// Only the green middle band survives the crop.
		for _, p := range []image.Point{{2, 2}, {CardWidth - 3, CardHeight - 3}, {CardWidth / 2, CardHeight / 2}} {
			r, g, b, _ := img.At(p.X, p.Y).RGBA()
			if g>>8 < 200 || r>>8 > 60 || b>>8 > 60 {
				t.Fatalf("%s: pixel %v is %d,%d,%d", name, p, r>>8, g>>8, b>>8)
			}
		}
	}
}
