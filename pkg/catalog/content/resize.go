package content

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/pkg/errors"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register decoder for DecodeConfig and Decode
)

// VariantWidths are the widths a still image is resized to on request. A
// fixed set keeps the derived files bounded: a client cannot ask for every
// width in turn.
var VariantWidths = []int{160, 320, 640, 1200}

// maxResizePixels bounds the decoded size (4 bytes per pixel) of an image
// the server resizes; larger images are served as uploaded.
const maxResizePixels = 40_000_000

// ErrNoVariant means the original should be served: the type is not resized
// (GIFs keep their animation), the image is not wider than the request, or
// it is too large to decode safely.
var ErrNoVariant = errors.New("no resized variant")

// Resizable reports whether media of this type and size gets variants.
func Resizable(mediaType string, width, height int) bool {
	switch mediaType {
	case "image/png", "image/jpeg", "image/webp":
		return width > 0 && height > 0 && width*height <= maxResizePixels
	}
	return false
}

// VariantWidth reports whether w is one of VariantWidths.
func VariantWidth(w int) bool {
	for _, v := range VariantWidths {
		if v == w {
			return true
		}
	}
	return false
}

// Resize scales an image to width w, keeping its aspect ratio. Opaque images
// become JPEG; images with transparency stay PNG. It returns the encoded
// bytes and their media type, or ErrNoVariant.
func Resize(r io.ReadSeeker, w int) ([]byte, string, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return nil, "", errors.Wrap(err, "read image header")
	}
	if format == "gif" || cfg.Width <= w || cfg.Width*cfg.Height > maxResizePixels {
		return nil, "", ErrNoVariant
	}
	if _, err = r.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, "", errors.Wrap(err, "decode image")
	}
	b := src.Bounds()
	h := max(1, (b.Dy()*w+b.Dx()/2)/b.Dx())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var out bytes.Buffer
	if dst.Opaque() {
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 82})
		return out.Bytes(), "image/jpeg", err
	}
	err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, dst)
	return out.Bytes(), "image/png", err
}
