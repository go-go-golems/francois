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
	}
	for _, c := range cases {
		out := string(md.Render(c.in))
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
	info, err := DetectMedia(buf.Bytes())
	if err != nil || info.Type != "image/png" || info.Width != 3 || info.Height != 2 {
		t.Fatalf("png: %+v %v", info, err)
	}
	webp := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	if info, err = DetectMedia(webp); err != nil || info.Type != "image/webp" {
		t.Fatalf("webp: %+v %v", info, err)
	}
	mp4 := append([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom"), make([]byte, 32)...)
	if info, err = DetectMedia(mp4); err != nil || info.Type != "video/mp4" || !IsVideo(info.Type) {
		t.Fatalf("mp4: %+v %v", info, err)
	}
	for name, data := range map[string][]byte{
		"svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":  []byte(`<!doctype html><script>alert(1)</script>`),
		"text":  []byte("hello"),
		"empty": nil,
	} {
		if _, err := DetectMedia(data); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := DetectMedia([]byte("\x89PNG\r\n\x1a\n broken")); err == nil {
		t.Error("broken png accepted")
	}
}
