package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Asset is a public embedded file, hashed and compressed once at startup.
// It never contains a session, grant or authored application HTML.
type Asset struct {
	name, mime, hash string
	data, compressed []byte
}

var _ http.Handler = (*Asset)(nil)

func newAsset(name, mime string, data []byte) *Asset {
	a := &Asset{name: name, mime: mime, hash: fmt.Sprintf("%x", sha256.Sum256(data)), data: data}
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	_, _ = z.Write(data) // bytes.Buffer cannot fail.
	_ = z.Close()
	if buf.Len() < len(data) {
		a.compressed = buf.Bytes()
	}
	return a
}

func (a *Asset) URL() string { return "/static/" + a.name + "?v=" + a.hash }

func (a *Asset) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", a.mime)
	w.Header().Set("Cache-Control", "public, no-cache")
	if r.URL.Query().Get("v") == a.hash {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	w.Header().Add("Vary", "Accept-Encoding")
	data, etag := a.data, a.hash
	if len(a.compressed) > 0 && acceptsGzip(r.Header.Values("Accept-Encoding")) {
		data, etag = a.compressed, a.hash+"-gzip"
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Header().Set("ETag", strconv.Quote(etag))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	http.ServeContent(w, r, a.name, time.Time{}, bytes.NewReader(data))
}

func acceptsGzip(headers []string) bool {
	wildcard := false
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			parts := strings.Split(part, ";")
			coding := strings.TrimSpace(parts[0])
			q := 1.0
			for _, param := range parts[1:] {
				key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
				if ok && strings.EqualFold(key, "q") {
					n, err := strconv.ParseFloat(value, 64)
					if err != nil || n < 0 || n > 1 {
						q = 0
					} else {
						q = n
					}
				}
			}
			if strings.EqualFold(coding, "gzip") {
				return q > 0
			}
			if coding == "*" {
				wildcard = q > 0
			}
		}
	}
	return wildcard
}

func versionRefs(html []byte, assets map[string]*Asset) []byte {
	for name, asset := range assets {
		html = bytes.ReplaceAll(html, []byte(`"/static/`+name+`"`), []byte(`"`+asset.URL()+`"`))
	}
	return html
}
