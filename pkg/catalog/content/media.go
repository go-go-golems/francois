package content

import (
	"bytes"
	"image"
	_ "image/gif"  // register decoder for DecodeConfig
	_ "image/jpeg" // register decoder for DecodeConfig
	_ "image/png"  // register decoder for DecodeConfig
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

// DetectMedia sniffs the type and, for PNG, JPEG and GIF, the dimensions.
func DetectMedia(data []byte) (MediaInfo, error) {
	if len(data) == 0 {
		return MediaInfo{}, ErrMediaType
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	t := http.DetectContentType(head)
	if !MediaTypes[t] {
		return MediaInfo{}, ErrMediaType
	}
	info := MediaInfo{Type: t}
	switch t {
	case "image/png", "image/jpeg", "image/gif":
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return MediaInfo{}, errors.Wrap(err, "the image could not be read")
		}
		info.Width, info.Height = cfg.Width, cfg.Height
	}
	return info, nil
}

// IsVideo reports whether a media type is a video.
func IsVideo(mediaType string) bool { return mediaType == "video/mp4" || mediaType == "video/webm" }
