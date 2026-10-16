package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Me reports the caller and, for a token, its scopes. Any valid token may call it.
func (c *Client) Me(ctx context.Context) (Me, error) {
	var out Me
	return out, c.get(ctx, "/api/me", nil, &out)
}

// Studio is the content API (/api/studio, scope studio): artifacts and their
// pages, releases and attachments, media, journal posts and creators.
type Studio struct{ c *Client }

func (s Studio) Me(ctx context.Context) (StudioMe, error) {
	var out StudioMe
	return out, s.c.get(ctx, "/api/studio/me", nil, &out)
}

func (s Studio) Artifacts(ctx context.Context) ([]ArtifactRow, error) {
	var out []ArtifactRow
	return out, s.c.get(ctx, "/api/studio/artifacts", nil, &out)
}

func (s Studio) Artifact(ctx context.Context, id string) (ArtifactDetail, error) {
	var out ArtifactDetail
	return out, s.c.get(ctx, "/api/studio/artifacts/"+esc(id), nil, &out)
}

func (s Studio) CreateArtifact(ctx context.Context, in ArtifactCreate) (Artifact, error) {
	var out Artifact
	return out, s.c.send2(ctx, http.MethodPost, "/api/studio/artifacts", in, &out)
}

func (s Studio) PatchArtifact(ctx context.Context, id string, in ArtifactPatch) (Artifact, error) {
	var out Artifact
	return out, s.c.send2(ctx, http.MethodPatch, "/api/studio/artifacts/"+esc(id), in, &out)
}

// SaveDraft stores an unpublished version of the artifact's public page.
func (s Studio) SaveDraft(ctx context.Context, id string, page Page) error {
	return s.c.send2(ctx, http.MethodPut, "/api/studio/artifacts/"+esc(id)+"/draft", page, nil)
}

func (s Studio) PublishDraft(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodPost, "/api/studio/artifacts/"+esc(id)+"/draft/publish", nil, nil)
}

func (s Studio) DiscardDraft(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/artifacts/"+esc(id)+"/draft", nil, nil)
}

// Releases lists releases; artifact "" lists all (administrators only).
func (s Studio) Releases(ctx context.Context, artifact string) ([]Release, error) {
	var out []Release
	q := url.Values{}
	if artifact != "" {
		q.Set("artifact", artifact)
	}
	return out, s.c.get(ctx, "/api/studio/releases", q, &out)
}

// ReleaseUpload is a release to check or create. HTML is required; Title,
// Description and Tags only seed a new artifact.
type ReleaseUpload struct {
	ArtifactID, Version, Title, Description, Changelog string
	Tags                                               []string
	HTML, Manifest                                     []byte
	FreeSample                                         bool
}

func (r ReleaseUpload) parts() []multipartPart {
	p := []multipartPart{
		{field: "artifactId", value: r.ArtifactID}, {field: "version", value: r.Version},
		{field: "title", value: r.Title}, {field: "description", value: r.Description},
		{field: "changelog", value: r.Changelog}, {field: "tags", value: strings.Join(r.Tags, ",")},
		{field: "freeSample", value: strconv.FormatBool(r.FreeSample)},
	}
	if len(r.Manifest) > 0 {
		p = append(p, multipartPart{field: "manifestJson", value: string(r.Manifest)})
	}
	return append(p, multipartPart{field: "html", filename: r.ArtifactID + ".html", content: bytes.NewReader(r.HTML)})
}

// CheckRelease validates without storing anything.
func (s Studio) CheckRelease(ctx context.Context, r ReleaseUpload) (CheckResult, error) {
	var out CheckResult
	return out, s.c.upload(ctx, "/api/studio/releases/check", r.parts(), &out)
}

// CreateRelease creates a draft release (and the artifact when it is new).
func (s Studio) CreateRelease(ctx context.Context, r ReleaseUpload) (Release, error) {
	var out Release
	return out, s.c.upload(ctx, "/api/studio/releases", r.parts(), &out)
}

func (s Studio) PublishRelease(ctx context.Context, id string) (Release, error) {
	var out Release
	return out, s.c.send2(ctx, http.MethodPost, "/api/studio/releases/"+esc(id)+"/publish", nil, &out)
}

// WithdrawRelease withdraws a published release; suspendPinned also suspends
// the workspaces pinned to it. It returns how many were suspended.
func (s Studio) WithdrawRelease(ctx context.Context, id string, suspendPinned bool) (int, error) {
	var out struct {
		Withdrawn          bool `json:"withdrawn"`
		SuspendedInstances int  `json:"suspendedInstances"`
	}
	err := s.c.send2(ctx, http.MethodPost, "/api/studio/releases/"+esc(id)+"/withdraw", map[string]bool{"suspendPinned": suspendPinned}, &out)
	return out.SuspendedInstances, err
}

// PreviewRelease opens a preview instance of a draft.
func (s Studio) PreviewRelease(ctx context.Context, id string) (Instance, error) {
	var out Instance
	return out, s.c.send2(ctx, http.MethodPost, "/api/studio/releases/"+esc(id)+"/preview", nil, &out)
}

// DeleteRelease deletes an unreferenced draft.
func (s Studio) DeleteRelease(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/releases/"+esc(id), nil, nil)
}

// AddAttachment uploads a file to a release; name is the file name shown to
// subscribers.
func (s Studio) AddAttachment(ctx context.Context, releaseID, name, label string, content io.Reader) (Attachment, error) {
	var out Attachment
	return out, s.c.upload(ctx, "/api/studio/releases/"+esc(releaseID)+"/attachments",
		[]multipartPart{{field: "label", value: label}, {field: "file", filename: name, content: content}}, &out)
}

// AddAttachmentFile uploads a local file.
func (s Studio) AddAttachmentFile(ctx context.Context, releaseID, path, label string) (Attachment, error) {
	f, err := os.Open(path) // #nosec G304 -- the caller names the file to upload
	if err != nil {
		return Attachment{}, err
	}
	defer func() { _ = f.Close() }()
	return s.AddAttachment(ctx, releaseID, filepath.Base(path), label, f)
}

func (s Studio) PatchAttachment(ctx context.Context, id string, label *string, position *int) (Attachment, error) {
	var out Attachment
	in := map[string]any{}
	if label != nil {
		in["label"] = *label
	}
	if position != nil {
		in["position"] = *position
	}
	return out, s.c.send2(ctx, http.MethodPatch, "/api/studio/attachments/"+esc(id), in, &out)
}

func (s Studio) DeleteAttachment(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/attachments/"+esc(id), nil, nil)
}

// DownloadAttachment streams an attachment to w.
func (s Studio) DownloadAttachment(ctx context.Context, id string, w io.Writer) (int64, error) {
	return s.c.download(ctx, "/api/studio/attachments/"+esc(id)+"/download", w)
}

func (s Studio) Media(ctx context.Context) ([]Media, error) {
	var out []Media
	return out, s.c.get(ctx, "/api/studio/media", nil, &out)
}

// UploadMedia adds an image or video to the media library.
func (s Studio) UploadMedia(ctx context.Context, name, alt string, content io.Reader) (Media, error) {
	var out Media
	return out, s.c.upload(ctx, "/api/studio/media", []multipartPart{{field: "alt", value: alt}, {field: "file", filename: name, content: content}}, &out)
}

// UploadMediaFile uploads a local image or video.
func (s Studio) UploadMediaFile(ctx context.Context, path, alt string) (Media, error) {
	f, err := os.Open(path) // #nosec G304 -- the caller names the file to upload
	if err != nil {
		return Media{}, err
	}
	defer func() { _ = f.Close() }()
	return s.UploadMedia(ctx, filepath.Base(path), alt, f)
}

// PatchMedia sets alt text and, for a video, its poster image.
func (s Studio) PatchMedia(ctx context.Context, id string, alt, poster *string) error {
	in := map[string]any{}
	if alt != nil {
		in["alt"] = *alt
	}
	if poster != nil {
		in["poster"] = *poster
	}
	return s.c.send2(ctx, http.MethodPatch, "/api/studio/media/"+esc(id), in, nil)
}

func (s Studio) DeleteMedia(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/media/"+esc(id), nil, nil)
}

func (s Studio) Posts(ctx context.Context) ([]Post, error) {
	var out []Post
	return out, s.c.get(ctx, "/api/studio/posts", nil, &out)
}

func (s Studio) Post(ctx context.Context, id string) (Post, error) {
	var out Post
	return out, s.c.get(ctx, "/api/studio/posts/"+esc(id), nil, &out)
}

func (s Studio) CreatePost(ctx context.Context, in PostInput) (Post, error) {
	var out Post
	return out, s.c.send2(ctx, http.MethodPost, "/api/studio/posts", in, &out)
}

func (s Studio) PatchPost(ctx context.Context, id string, in PostInput) (Post, error) {
	var out Post
	return out, s.c.send2(ctx, http.MethodPatch, "/api/studio/posts/"+esc(id), in, &out)
}

// SetPostPublished publishes or unpublishes a post.
func (s Studio) SetPostPublished(ctx context.Context, id string, published bool) error {
	action := "unpublish"
	if published {
		action = "publish"
	}
	return s.c.send2(ctx, http.MethodPost, "/api/studio/posts/"+esc(id)+"/"+action, nil, nil)
}

func (s Studio) DeletePost(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/posts/"+esc(id), nil, nil)
}

func (s Studio) Creators(ctx context.Context) ([]Creator, error) {
	var out []Creator
	return out, s.c.get(ctx, "/api/studio/creators", nil, &out)
}

func (s Studio) CreateCreator(ctx context.Context, in CreatorInput) (Creator, error) {
	var out Creator
	return out, s.c.send2(ctx, http.MethodPost, "/api/studio/creators", in, &out)
}

func (s Studio) PatchCreator(ctx context.Context, id string, in CreatorInput) (Creator, error) {
	var out Creator
	return out, s.c.send2(ctx, http.MethodPatch, "/api/studio/creators/"+esc(id), in, &out)
}

func (s Studio) DeleteCreator(ctx context.Context, id string) error {
	return s.c.send2(ctx, http.MethodDelete, "/api/studio/creators/"+esc(id), nil, nil)
}

// Markdown renders Markdown with the public page renderer.
func (s Studio) Markdown(ctx context.Context, md string) (string, error) {
	var out struct {
		HTML string `json:"html"`
	}
	return out.HTML, s.c.send2(ctx, http.MethodPost, "/api/studio/markdown", map[string]string{"markdown": md}, &out)
}

func (s Studio) Plans(ctx context.Context) ([]PlanName, error) {
	var out []PlanName
	return out, s.c.get(ctx, "/api/studio/plans", nil, &out)
}

// PushResult is the outcome of Push.
type PushResult struct {
	Release     Release
	Attachments []Attachment
}

// Push creates a draft release, uploads the attachment files, and with
// publish publishes it. A failure after the draft exists says so; the draft
// can be inspected or deleted in the studio.
func (s Studio) Push(ctx context.Context, r ReleaseUpload, attachments []string, publish bool) (PushResult, error) {
	for _, a := range attachments {
		if fi, err := os.Stat(a); err != nil || fi.IsDir() {
			return PushResult{}, errors.Errorf("attachment %s is not a readable file", a)
		}
	}
	rel, err := s.CreateRelease(ctx, r)
	if err != nil {
		return PushResult{}, err
	}
	res := PushResult{Release: rel}
	for _, a := range attachments {
		att, err := s.AddAttachmentFile(ctx, rel.ID, a, "")
		if err != nil {
			return res, errors.Wrapf(err, "the draft %s was created but attaching %s failed", rel.ID, a)
		}
		res.Attachments = append(res.Attachments, att)
	}
	if publish {
		pub, err := s.PublishRelease(ctx, rel.ID)
		if err != nil {
			return res, errors.Wrapf(err, "the draft %s was created but publishing failed", rel.ID)
		}
		pub.Checks = rel.Checks
		res.Release = pub
	}
	return res, nil
}
