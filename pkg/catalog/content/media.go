package content

import (
	"image"
	_ "image/gif"  // register decoder for DecodeConfig
	_ "image/jpeg" // register decoder for DecodeConfig
	_ "image/png"  // register decoder for DecodeConfig
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// MediaTypes are the storefront media types. SVG is excluded because it can
// carry script; types are detected from the bytes, never taken from the
// client.
var MediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"video/mp4":  true,
	"video/webm": true,
}

// ErrMediaType rejects bytes that are not an accepted media type.
var ErrMediaType = errors.New("only PNG, JPEG, GIF, WebP, MP4 and WebM files are accepted")

type MediaInfo struct {
	Type          string
	Width, Height int
}

// DetectMedia sniffs the type from the first 512 bytes and, for PNG, JPEG
// GIF and WebP, decodes the dimensions. It leaves r positioned at the start.
func DetectMedia(r io.ReadSeeker) (MediaInfo, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return MediaInfo{}, err
	}
	if n == 0 {
		return MediaInfo{}, ErrMediaType
	}
	t := http.DetectContentType(head[:n])
	if !MediaTypes[t] {
		return MediaInfo{}, ErrMediaType
	}
	info := MediaInfo{Type: t}
	if _, err = r.Seek(0, io.SeekStart); err != nil {
		return MediaInfo{}, err
	}
	switch t {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		cfg, _, err := image.DecodeConfig(r)
		// WebP variants the decoder does not know (such as animation) are
		// still accepted, without dimensions, as before WebP was decoded.
		if err != nil && t != "image/webp" {
			return MediaInfo{}, errors.Wrap(err, "the image could not be read")
		}
		info.Width, info.Height = cfg.Width, cfg.Height
		if _, err = r.Seek(0, io.SeekStart); err != nil {
			return MediaInfo{}, err
		}
	}
	return info, nil
}

// IsVideo reports whether a media type is a video.
func IsVideo(mediaType string) bool { return mediaType == "video/mp4" || mediaType == "video/webm" }
