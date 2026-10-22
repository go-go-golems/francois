package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/pkg/errors"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The default share image is the 1200×630 link preview of pages that have
// no image of their own. It is drawn from the configured brand, tagline and
// host, so it follows the settings without a designed asset per deployment.
// The licensed web fonts are WOFF2, which Go cannot rasterize, so it uses the
// Go fonts.

const shareW, shareH = 1200, 630

type shareCard struct{ Brand, Tagline, Host string }

// key identifies the rendered image; it versions the og:image URL so link
// preview caches refresh when the brand or tagline changes.
func (c shareCard) key() string {
	h := sha256.Sum256([]byte(c.Brand + "\x00" + c.Tagline + "\x00" + c.Host))
	return hex.EncodeToString(h[:8])
}

type shareCache struct {
	mu   sync.Mutex
	key  string
	data []byte
}

func (s *Server) shareCard(r *http.Request, doc settings.Document) shareCard {
	return shareCard{Brand: s.brand(r, doc), Tagline: doc.Storefront.Tagline, Host: s.site(r).PublicURL.Host}
}

// shareURL is the absolute, versioned URL of the default share image.
func (s *Server) shareURL(r *http.Request, doc settings.Document) string {
	return s.site(r).PortalOrigin() + "/share.png?v=" + s.shareCard(r, doc).key()
}

func (s *Server) shareImage(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		http.Error(w, "Image unavailable", http.StatusServiceUnavailable)
		return
	}
	c := s.shareCard(r, rev.Document)
	key := c.key()
	s.share.mu.Lock()
	data := s.share.data
	if s.share.key != key {
		if data, err = renderShare(c); err != nil {
			s.share.mu.Unlock()
			log.Error().Err(err).Msg("share image")
			http.Error(w, "Image unavailable", http.StatusServiceUnavailable)
			return
		}
		s.share.key, s.share.data = key, data
	}
	s.share.mu.Unlock()
	w.Header().Set("Content-Type", "image/png")
	if r.URL.Query().Get("v") == key {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	w.Header().Set("ETag", `"`+key+`"`)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

func face(ttf []byte, size float64) (font.Face, error) {
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// wrap breaks text into at most n lines no wider than width, ending a cut
// line with an ellipsis.
func wrap(f font.Face, text string, width fixed.Int26_6, n int) []string {
	if n < 1 {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		next := strings.TrimSpace(line + " " + word)
		if line == "" || font.MeasureString(f, next) <= width {
			line = next
			continue
		}
		if len(lines) == n-1 {
			for font.MeasureString(f, line+"…") > width && strings.Contains(line, " ") {
				line = line[:strings.LastIndex(line, " ")]
			}
			return append(lines, strings.TrimRight(line, ".,;:")+"…")
		}
		lines = append(lines, line)
		line = word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func renderShare(c shareCard) ([]byte, error) {
	title, err := face(gomedium.TTF, 76)
	if err != nil {
		return nil, errors.Wrap(err, "load title font")
	}
	body, err := face(goregular.TTF, 34)
	if err != nil {
		return nil, errors.Wrap(err, "load body font")
	}
	mono, err := face(gomono.TTF, 26)
	if err != nil {
		return nil, errors.Wrap(err, "load mono font")
	}
	ink, muted := image.NewUniform(color.RGBA{0x11, 0x11, 0x11, 0xff}), image.NewUniform(color.RGBA{0x5e, 0x5e, 0x5e, 0xff})
	img := image.NewRGBA(image.Rect(0, 0, shareW, shareH))
	draw.Draw(img, img.Bounds(), image.White, image.Point{}, draw.Src)

	// The mark from icon.svg (a 32-unit grid), drawn at 3 px per unit.
	const pad, unit = 80, 3
	rect := func(x, y, w, h int, c image.Image) {
		draw.Draw(img, image.Rect(pad+x*unit, pad+y*unit, pad+(x+w)*unit, pad+(y+h)*unit), c, image.Point{}, draw.Src)
	}
	rect(0, 0, 32, 32, ink)
	for _, r := range [][4]int{{9, 7, 5, 18}, {9, 7, 14, 5}, {9, 15, 10, 4}} {
		rect(r[0], r[1], r[2], r[3], image.White)
	}

	width := fixed.I(shareW - 2*pad)
	y := 300
	d := &font.Drawer{Dst: img, Src: ink, Face: title}
	for _, l := range wrap(title, c.Brand, width, 2) {
		d.Dot = fixed.P(pad, y)
		d.DrawString(l)
		y += 84
	}
	d.Src, d.Face = muted, body
	y += 4
	// The tagline takes the lines left above the host line, at most three.
	bottom := shareH - pad
	if c.Host != c.Brand {
		bottom -= 70
	}
	for _, l := range wrap(body, c.Tagline, width, min(3, (bottom-y)/46+1)) {
		d.Dot = fixed.P(pad, y)
		d.DrawString(l)
		y += 46
	}
	if c.Host != c.Brand {
		d.Face = mono
		d.Dot = fixed.P(pad, shareH-pad+10)
		d.DrawString(c.Host)
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
