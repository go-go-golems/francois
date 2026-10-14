// Package content renders storefront text and validates storefront media.
// Everything it produces is shown on the trusted portal origin, so Markdown
// is rendered without raw HTML and media types are detected from bytes.
package content

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// MediaPrefix is the portal path that serves uploaded media. The portal's
// Content-Security-Policy allows images only from its own origin, so only
// images under this prefix are rendered as images.
const MediaPrefix = "/media/"

// Markdown renders GitHub-flavoured Markdown. Raw HTML is omitted and
// dangerous link schemes are dropped (goldmark's default without
// html.WithUnsafe); images outside MediaPrefix become plain links.
type Markdown struct{ md goldmark.Markdown }

func NewMarkdown() *Markdown {
	return &Markdown{md: goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(imageRenderer{}, 100))),
	)}
}

// Render returns sanitised HTML. Rendering errors cannot leak input: on
// failure the text is shown escaped.
func (m *Markdown) Render(source string) template.HTML {
	var buf bytes.Buffer
	if err := m.md.Convert([]byte(source), &buf); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(source) + "</p>") // #nosec G203 -- escaped above
	}
	return template.HTML(buf.String()) // #nosec G203 -- goldmark without WithUnsafe emits no raw HTML
}

// imageRenderer renders portal media as lazy images and any other image
// reference as a link to it.
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
	if strings.HasPrefix(dest, MediaPrefix) && !strings.ContainsAny(dest[len(MediaPrefix):], "/?#\\") {
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
