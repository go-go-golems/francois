// Package content renders storefront text and validates storefront media.
// Everything it produces is shown on the trusted portal origin, so Markdown
// is rendered without raw HTML and media types are detected from bytes.
package content

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// MediaPrefix is the portal path that serves uploaded media. The portal's
// Content-Security-Policy allows images only from its own origin, so only
// images under this prefix are rendered as images.
const MediaPrefix = "/media/"

// Markdown renders GitHub-flavoured Markdown. Raw HTML is omitted and
// dangerous link schemes are dropped (goldmark's default without
// html.WithUnsafe); images outside MediaPrefix become plain links, and
// portal media that are videos become video players.
type Markdown struct{ md goldmark.Markdown }

func NewMarkdown() *Markdown {
	return &Markdown{md: goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID(), parser.WithASTTransformers(util.Prioritized(embedTransformer{}, 100))),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(imageRenderer{}, 100))),
	)}
}

// Embed describes a portal media item a document references, so the
// renderer can tell videos from images.
type Embed struct {
	Type   string // media type
	Poster string // poster media ID for videos, or ""
}

var mediaRef = regexp.MustCompile(`/media/(med_[A-Za-z0-9]+)`)

// MediaRefs lists the media IDs a document references by URL.
func MediaRefs(source string) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, m := range mediaRef.FindAllStringSubmatch(source, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

var embedsKey = parser.NewContextKey()

// Render returns sanitised HTML. embeds maps referenced media IDs to their
// types; a nil map renders every portal media reference as an image.
// Rendering errors cannot leak input: on failure the text is shown escaped.
func (m *Markdown) Render(source string, embeds map[string]Embed) template.HTML {
	var buf bytes.Buffer
	pc := parser.NewContext()
	pc.Set(embedsKey, embeds)
	if err := m.md.Convert([]byte(source), &buf, parser.WithContext(pc)); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(source) + "</p>") // #nosec G203 -- escaped above
	}
	return template.HTML(buf.String()) // #nosec G203 -- goldmark without WithUnsafe emits no raw HTML
}

// Section is a rendered editorial block. Only conventional level-two labels
// are lifted out; arbitrary headings remain ordinary sanitized Markdown.
type Section struct {
	Label string
	HTML  template.HTML
}

func (m *Markdown) Sections(source string, embeds map[string]Embed) []Section {
	input := []byte(source)
	pc := parser.NewContext()
	pc.Set(embedsKey, embeds)
	doc := m.md.Parser().Parse(text.NewReader(input), parser.WithContext(pc))
	sections := []Section{}
	group := ast.NewDocument()
	label := ""
	flush := func() {
		if group.ChildCount() == 0 {
			return
		}
		var buf bytes.Buffer
		if err := m.md.Renderer().Render(&buf, input, group); err != nil {
			sections = append(sections, Section{Label: label, HTML: template.HTML("<p>" + template.HTMLEscapeString(source) + "</p>")}) // #nosec G203 -- escaped
		} else {
			sections = append(sections, Section{Label: label, HTML: template.HTML(buf.String())}) // #nosec G203 -- same safe renderer as Render
		}
		group = ast.NewDocument()
	}
	for node := doc.FirstChild(); node != nil; {
		next := node.NextSibling()
		name := ""
		if heading, ok := node.(*ast.Heading); ok && heading.Level == 2 {
			switch strings.ToLower(strings.TrimSpace(plainText(heading, input))) {
			case "what it is":
				name = "What it is"
			case "why we made it":
				name = "Why we made it"
			case "related":
				name = "Related"
			case "sources":
				name = "Sources"
			}
		}
		if name != "" {
			flush()
			label = name
		} else {
			group.AppendChild(group, node)
		}
		node = next
	}
	flush()
	return sections
}

// embedTransformer marks image nodes that refer to portal videos, using the
// embeds passed to Render, so the renderer can emit a video player.
type embedTransformer struct{}

func (embedTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	embeds, _ := pc.Get(embedsKey).(map[string]Embed)
	if len(embeds) == 0 {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := n.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		id, ok := portalMedia(string(img.Destination))
		if e, found := embeds[id]; ok && found && IsVideo(e.Type) {
			img.SetAttributeString("video", true)
			if e.Poster != "" {
				img.SetAttributeString("poster", e.Poster)
			}
		}
		return ast.WalkSkipChildren, nil
	})
}

// portalMedia returns the media ID of a /media/<id> URL.
func portalMedia(dest string) (string, bool) {
	if !strings.HasPrefix(dest, MediaPrefix) || strings.ContainsAny(dest[len(MediaPrefix):], "/?#\\") || len(dest) == len(MediaPrefix) {
		return "", false
	}
	return dest[len(MediaPrefix):], true
}

// imageRenderer renders portal media as lazy images (or video players) and
// any other image reference as a link to it.
type imageRenderer struct{}

func (imageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, renderImage)
}

func renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	dest := string(n.Destination)
	alt := plainText(n, source)
	if _, ok := portalMedia(dest); ok {
		if _, video := n.AttributeString("video"); video {
			_, _ = w.WriteString(`<video src="`)
			_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
			_, _ = w.WriteString(`" controls preload="metadata" playsinline`)
			if poster, ok := n.AttributeString("poster"); ok {
				_, _ = w.WriteString(` poster="` + MediaPrefix + template.HTMLEscapeString(poster.(string)) + `"`)
			}
			_, _ = w.WriteString(` aria-label="` + template.HTMLEscapeString(alt) + `"></video>`)
			return ast.WalkSkipChildren, nil
		}
		_, _ = w.WriteString(`<img src="`)
		_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
		_, _ = w.WriteString(`" alt="`)
		_, _ = w.WriteString(template.HTMLEscapeString(alt))
		_, _ = w.WriteString(`" loading="lazy">`)
		return ast.WalkSkipChildren, nil
	}
	if html.IsDangerousURL(n.Destination) {
		_, _ = w.WriteString(template.HTMLEscapeString(alt))
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
	_, _ = w.WriteString(`">`)
	if alt == "" {
		alt = dest
	}
	_, _ = w.WriteString(template.HTMLEscapeString(alt))
	_, _ = w.WriteString(`</a>`)
	return ast.WalkSkipChildren, nil
}

func plainText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if t, ok := c.(*ast.Text); ok {
				b.Write(t.Segment.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// Excerpt returns the first paragraph of plain text, at most limit runes,
// for cards and link previews.
func Excerpt(source string, limit int) string {
	text := strings.TrimSpace(source)
	if i := strings.Index(text, "\n\n"); i >= 0 {
		text = text[:i]
	}
	text = strings.Join(strings.Fields(strings.NewReplacer("#", "", "*", "", "_", "", "`", "").Replace(text)), " ")
	r := []rune(text)
	if len(r) > limit {
		return strings.TrimSpace(string(r[:limit-1])) + "…"
	}
	return text
}
