package host

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

var accepted = map[string]int64{"image/png": 20 << 20, "image/jpeg": 20 << 20, "image/gif": 20 << 20, "image/webp": 20 << 20, "video/mp4": 20 << 20, "video/webm": 20 << 20, "application/pdf": 20 << 20, "font/woff2": 20 << 20, "font/woff": 20 << 20, "font/ttf": 20 << 20, "font/otf": 20 << 20, "text/csv": 20 << 20, "text/markdown": 20 << 20, "application/json": 20 << 20, "text/plain": 20 << 20, "text/css": 16 << 20, "text/javascript": 16 << 20}

func (h *Host) upload(w http.ResponseWriter, r *http.Request, artifact string) {
	a, err := h.Store.Get(r.Context(), artifact)
	if err != nil {
		return
	}
	if _, ok := a.Manifest["assets"]; !ok {
		fail(w, 403, "not_declared", "Assets not declared")
		return
	}
	kind := r.Header.Get("Content-Type")
	maxBytes, ok := accepted[kind]
	if !ok {
		fail(w, 400, "unsupported_type", "Unsupported asset type (SVG is not enabled)")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
	if err != nil || len(data) == 0 {
		fail(w, 413, "too_large", "Invalid asset size")
		return
	}
	if strings.HasPrefix(kind, "text/") || kind == "application/json" {
		if !utf8.Valid(data) {
			fail(w, 400, "invalid_request", "Text assets require UTF-8")
			return
		}
	}
	var count, total int64
	if err = h.Store.DB.QueryRowContext(r.Context(), `SELECT count(*),coalesce(sum(length(bytes)),0) FROM assets WHERE artifact=?`, artifact).Scan(&count, &total); err != nil {
		fail(w, 500, "store_unavailable", "Asset store unavailable")
		return
	}
	if count >= 1000 || total+int64(len(data)) > 512<<20 {
		fail(w, 413, "quota_or_state", "Asset quota reached")
		return
	}
	key := id("b_")
	_, err = h.Store.DB.ExecContext(r.Context(), `INSERT INTO assets VALUES(?,?,?,?,?)`, key, artifact, kind, data, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		fail(w, 500, "store_unavailable", "Could not save asset")
		return
	}
	respond(w, map[string]any{"id": key, "url": "/_blob/" + key, "sizeBytes": len(data), "contentType": kind})
}
func (h *Host) blob(w http.ResponseWriter, r *http.Request, artifact string) {
	var kind string
	var data []byte
	if err := h.Store.DB.QueryRowContext(r.Context(), `SELECT type,bytes FROM assets WHERE artifact=? AND id=?`, artifact, r.PathValue("id")).Scan(&kind, &data); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", kind)
	if !strings.HasPrefix(kind, "image/") && !strings.HasPrefix(kind, "font/") && !strings.HasPrefix(kind, "video/") {
		w.Header().Set("Content-Disposition", "attachment")
	}
	_, _ = w.Write(data)
}
func (h *Host) listAssets(ctx context.Context, artifact string) (any, error) {
	rows, err := h.Store.DB.QueryContext(ctx, `SELECT id,type,length(bytes),created_at FROM assets WHERE artifact=? ORDER BY created_at`, artifact)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	total := int64(0)
	for rows.Next() {
		var key, kind, created string
		var size int64
		if err = rows.Scan(&key, &kind, &size, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": key, "url": "/_blob/" + key, "contentType": kind, "sizeBytes": size, "createdAt": created})
		total += size
	}
	return map[string]any{"assets": out, "usage": map[string]any{"files": len(out), "bytes": total, "maxFiles": 1000, "maxBytes": 512 << 20}}, rows.Err()
}
