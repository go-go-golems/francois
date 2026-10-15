package catalog

import (
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// Release attachments: supplementary files on a release (manuals, archives,
// notes). Editors add, relabel and remove them in any release state; they
// are downloaded under the release's download rule (mayDownload) once it is
// published, and by editors at any time through the studio.

const (
	maxAttachments   = 20
	maxAttachmentLen = 200 // name and label, in characters
)

// attachmentName is the base name of an uploaded file, trimmed to
// maxAttachmentLen characters; "" when nothing usable remains.
func attachmentName(raw string) string {
	name := strings.TrimSpace(path.Base(strings.ReplaceAll(raw, "\\", "/")))
	if name == "." || name == "/" || !utf8.ValidString(name) {
		return ""
	}
	if r := []rune(name); len(r) > maxAttachmentLen {
		name = string(r[:maxAttachmentLen])
	}
	return name
}

func (s *Server) studioUploadAttachment(w http.ResponseWriter, r *http.Request) {
	rel, ok := s.releaseFor(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	rev, err := s.Store.ActiveSettings(ctx)
	if err != nil {
		unavailable(w, err)
		return
	}
	limit := rev.Document.Limits.MaxAttachmentBytes
	f, h, ok := s.uploadedFile(w, r, limit)
	if !ok {
		return
	}
	defer func() { _ = f.Close() }()
	name := attachmentName(h.Filename)
	label := strings.TrimSpace(r.FormValue("label"))
	if name == "" || utf8.RuneCountInString(label) > maxAttachmentLen {
		httpx.Fail(w, 400, "invalid_request", "The file needs a name, and labels are limited to 200 characters")
		return
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		unavailable(w, err)
		return
	}
	key, sum, size, err := s.Publisher.Blobs.PutReader(f, limit)
	if errors.Is(err, releases.ErrTooLarge) {
		httpx.Fail(w, 413, "too_large", "The file exceeds "+mib(limit))
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	a, err := s.Store.CreateAttachment(ctx, store.Attachment{ReleaseID: rel.ID, Name: name, Label: label, MediaType: http.DetectContentType(head[:n]),
		Size: size, SHA256: sum, StorageKey: key, CreatedBy: actor(r)}, maxAttachments)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "too_many_attachments", "A release holds at most 20 attachments")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	log.Info().Str("release", rel.ID).Str("attachment", a.ID).Int64("size", size).Msg("Attachment added")
	httpx.JSON(w, 201, a)
}

// editableAttachment loads an attachment whose artifact the caller edits.
func (s *Server) editableAttachment(w http.ResponseWriter, r *http.Request) (store.Attachment, bool) {
	ctx := r.Context()
	a, err := s.Store.Attachment(ctx, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "Attachment not found")
		return a, false
	}
	if err != nil {
		unavailable(w, err)
		return a, false
	}
	rel, err := s.Store.Release(ctx, a.ReleaseID)
	if err != nil {
		unavailable(w, err)
		return a, false
	}
	if !s.canEditArtifact(ctx, me(r), rel.ArtifactID) {
		httpx.Fail(w, 403, "not_editor", "You are not credited on this artifact")
		return a, false
	}
	return a, true
}

func (s *Server) studioPatchAttachment(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editableAttachment(w, r)
	if !ok {
		return
	}
	var in struct {
		Label    *string `json:"label"`
		Position *int    `json:"position"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || (in.Label != nil && utf8.RuneCountInString(*in.Label) > maxAttachmentLen) {
		httpx.Fail(w, 400, "invalid_request", "Labels are limited to 200 characters")
		return
	}
	if in.Label != nil {
		*in.Label = strings.TrimSpace(*in.Label)
	}
	out, err := s.Store.UpdateAttachment(r.Context(), a.ID, in.Label, in.Position)
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, out)
}

func (s *Server) studioDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editableAttachment(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteAttachment(r.Context(), a.ID); err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"deleted": true})
}

// studioDownloadAttachment lets editors download in any release state.
func (s *Server) studioDownloadAttachment(w http.ResponseWriter, r *http.Request) {
	if a, ok := s.editableAttachment(w, r); ok {
		s.serveAttachment(w, r, a)
	}
}

// downloadAttachment is the public route: published releases only, under
// the release's download rule.
func (s *Server) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rel, err := s.Store.Release(ctx, r.PathValue("release"))
	if err != nil || rel.State != "published" {
		httpx.Fail(w, 404, "release_unavailable", "Release not available")
		return
	}
	a, err := s.Store.Attachment(ctx, r.PathValue("id"))
	if err != nil || a.ReleaseID != rel.ID {
		httpx.Fail(w, 404, "not_found", "Attachment not found")
		return
	}
	if s.mayDownload(w, r, rel) {
		s.serveAttachment(w, r, a)
	}
}

// serveAttachment sends the file as a download (never rendered on the
// portal origin), with Range support for resumed downloads.
func (s *Server) serveAttachment(w http.ResponseWriter, r *http.Request, a store.Attachment) {
	f, err := s.Publisher.Blobs.Open(a.StorageKey)
	if err != nil {
		log.Error().Err(err).Str("attachment", a.ID).Msg("attachment unreadable")
		httpx.Fail(w, 503, "file_unavailable", "File temporarily unavailable")
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", "application/octet-stream")
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": a.Name})
	if disposition == "" {
		disposition = `attachment; filename="` + safeName.ReplaceAllString(a.Name, "_") + `"`
	}
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-SHA256", a.SHA256)
	http.ServeContent(w, r, "", a.CreatedAt, f)
}

// publicAttachments describes attachments without storage or user details.
func publicAttachments(rel store.Release) []map[string]any {
	out := []map[string]any{}
	for _, a := range rel.Attachments {
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "label": a.Label, "mediaType": a.MediaType, "size": a.Size, "sha256": a.SHA256, "position": a.Position,
			"href": "/api/releases/" + rel.ID + "/attachments/" + a.ID + "/download"})
	}
	return out
}
