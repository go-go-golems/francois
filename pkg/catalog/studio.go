package catalog

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/content"
	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// The studio API manages storefront content: artifacts and their pages,
// releases, media, posts and creator profiles. Every route needs the creator
// or admin role; object rules (canEditArtifact, canEditPost) decide which
// objects a creator may change. User, billing and settings administration
// stay under /api/admin.

const (
	maxBodyBytes = 100 << 10
	maxGallery   = 24
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var planKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func (s *Server) studioRoutes(m *http.ServeMux) {
	c := func(h http.HandlerFunc) http.HandlerFunc { return s.guard(creator, scopeStudio, h) }
	m.HandleFunc("GET /api/studio/me", c(s.studioMe))
	m.HandleFunc("GET /api/studio/artifacts", c(s.studioArtifacts))
	m.HandleFunc("POST /api/studio/artifacts", c(s.studioCreateArtifact))
	m.HandleFunc("GET /api/studio/artifacts/{id}", c(s.studioArtifact))
	m.HandleFunc("PATCH /api/studio/artifacts/{id}", c(s.studioPatchArtifact))
	m.HandleFunc("PUT /api/studio/artifacts/{id}/draft", c(s.studioSaveDraft))
	m.HandleFunc("POST /api/studio/artifacts/{id}/draft/publish", c(s.studioPublishDraft))
	m.HandleFunc("DELETE /api/studio/artifacts/{id}/draft", c(s.studioDiscardDraft))
	m.HandleFunc("GET /api/studio/releases", c(s.studioReleases))
	m.HandleFunc("POST /api/studio/releases", c(s.studioUpload))
	m.HandleFunc("POST /api/studio/releases/check", c(s.studioCheck))
	m.HandleFunc("POST /api/studio/releases/{id}/publish", c(s.studioPublish))
	m.HandleFunc("POST /api/studio/releases/{id}/withdraw", c(s.studioWithdraw))
	m.HandleFunc("POST /api/studio/releases/{id}/preview", c(s.studioPreview))
	m.HandleFunc("DELETE /api/studio/releases/{id}", c(s.studioDeleteDraft))
	m.HandleFunc("POST /api/studio/releases/{id}/attachments", c(s.studioUploadAttachment))
	m.HandleFunc("PATCH /api/studio/attachments/{id}", c(s.studioPatchAttachment))
	m.HandleFunc("DELETE /api/studio/attachments/{id}", c(s.studioDeleteAttachment))
	m.HandleFunc("GET /api/studio/attachments/{id}/download", c(s.studioDownloadAttachment))
	m.HandleFunc("GET /api/studio/media", c(s.studioMedia))
	m.HandleFunc("POST /api/studio/media", c(s.studioUploadMedia))
	m.HandleFunc("PATCH /api/studio/media/{id}", c(s.studioPatchMedia))
	m.HandleFunc("DELETE /api/studio/media/{id}", c(s.studioDeleteMedia))
	m.HandleFunc("GET /api/studio/posts", c(s.studioPosts))
	m.HandleFunc("POST /api/studio/posts", c(s.studioCreatePost))
	m.HandleFunc("GET /api/studio/posts/{id}", c(s.studioPost))
	m.HandleFunc("PATCH /api/studio/posts/{id}", c(s.studioPatchPost))
	m.HandleFunc("DELETE /api/studio/posts/{id}", c(s.studioDeletePost))
	m.HandleFunc("POST /api/studio/posts/{id}/publish", c(s.studioPostState("published")))
	m.HandleFunc("POST /api/studio/posts/{id}/unpublish", c(s.studioPostState("draft")))
	m.HandleFunc("GET /api/studio/creators", c(s.studioCreators))
	m.HandleFunc("POST /api/studio/creators", c(s.studioCreateCreator))
	m.HandleFunc("PATCH /api/studio/creators/{id}", c(s.studioPatchCreator))
	m.HandleFunc("DELETE /api/studio/creators/{id}", c(s.studioDeleteCreator))
	m.HandleFunc("POST /api/studio/markdown", c(s.studioMarkdown))
	m.HandleFunc("GET /api/studio/plans", c(s.studioPlans))
}

func me(r *http.Request) store.User { return principal(r.Context()).User }

func isAdmin(r *http.Request) bool { return me(r).Role == "admin" }

// editable loads an artifact the caller may edit, writing 404 or 403.
func (s *Server) editable(w http.ResponseWriter, r *http.Request, id string) (store.Artifact, bool) {
	a, err := s.Store.Artifact(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "Artifact not found")
		return a, false
	}
	if err != nil {
		unavailable(w, err)
		return a, false
	}
	if !s.canEditArtifact(r.Context(), me(r), a.ID) {
		httpx.Fail(w, 403, "not_editor", "You are not credited on this artifact")
		return a, false
	}
	return a, true
}

// ownProfile is the caller's creator profile; creators need one to create
// artifacts and posts, because that is how their ownership is recorded.
func (s *Server) ownProfile(ctx context.Context, u store.User) (store.Creator, bool, error) {
	c, err := s.Store.CreatorForUser(ctx, u.ID)
	if errors.Is(err, store.ErrNotFound) {
		return c, false, nil
	}
	return c, err == nil, err
}

func (s *Server) studioMe(w http.ResponseWriter, r *http.Request) {
	u := me(r)
	out := map[string]any{"user": map[string]any{"id": u.ID, "name": u.DisplayName, "role": u.Role}, "creator": nil}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	out["limits"] = map[string]int64{"maxMediaBytes": rev.Document.Limits.MaxMediaBytes, "maxAttachmentBytes": rev.Document.Limits.MaxAttachmentBytes}
	if c, ok, err := s.ownProfile(r.Context(), u); err != nil {
		unavailable(w, err)
		return
	} else if ok {
		out["creator"] = c
	}
	httpx.OK(w, out)
}

// artifactSummary is a studio list row.
func artifactSummary(a store.Artifact, credits []store.Creator, rels []store.Release) map[string]any {
	ids := []string{}
	for _, c := range credits {
		ids = append(ids, c.ID)
	}
	var latest, drafts = "", 0
	for _, rel := range rels {
		if rel.State == "draft" {
			drafts++
		}
		if rel.State == "published" && latest == "" {
			latest = rel.Version
		}
	}
	return map[string]any{"id": a.ID, "title": a.Title, "description": a.Description, "tags": a.Tags, "visibility": a.Visibility,
		"featured": a.Featured, "sortOrder": a.SortOrder, "requiredPlan": a.RequiredPlan, "demoEnabled": a.DemoEnabled,
		"cover": a.Page.Cover, "hasDraft": a.PageDraft != nil, "creators": ids, "releases": len(rels), "draftReleases": drafts,
		"latestVersion": latest, "updatedAt": a.UpdatedAt}
}

func (s *Server) studioArtifacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	arts, err := s.Store.Artifacts(ctx)
	if err != nil {
		unavailable(w, err)
		return
	}
	credits, err := s.Store.ArtifactCreators(ctx, nil)
	if err != nil {
		unavailable(w, err)
		return
	}
	rels, err := s.Store.Releases(ctx, "")
	if err != nil {
		unavailable(w, err)
		return
	}
	byArtifact := map[string][]store.Release{}
	for _, rel := range rels {
		byArtifact[rel.ArtifactID] = append(byArtifact[rel.ArtifactID], rel)
	}
	u := me(r)
	out := []map[string]any{}
	for _, a := range arts {
		if u.Role != "admin" && !credited(credits[a.ID], u.ID) {
			continue
		}
		out = append(out, artifactSummary(a, credits[a.ID], byArtifact[a.ID]))
	}
	httpx.OK(w, out)
}

func credited(list []store.Creator, user string) bool {
	for _, c := range list {
		if c.UserID == user {
			return true
		}
	}
	return false
}

func (s *Server) studioCreateArtifact(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID, Title, Description string
	}
	if err := httpx.Read(w, r, &in, 8<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	in.ID, in.Title = strings.TrimSpace(in.ID), strings.TrimSpace(in.Title)
	if !releases.ValidArtifactID(in.ID) || in.Title == "" || len(in.Title) > 120 || len(in.Description) > 2000 {
		httpx.Fail(w, 400, "invalid_request", "An ID of lowercase letters, digits, - or _ and a title of at most 120 characters are required")
		return
	}
	ctx := r.Context()
	profile, hasProfile, err := s.ownProfile(ctx, me(r))
	if err != nil {
		unavailable(w, err)
		return
	}
	if !isAdmin(r) && !hasProfile {
		httpx.Fail(w, 409, "creator_profile_required", "Ask an administrator to link your account to a creator profile first")
		return
	}
	var credit []string
	if hasProfile {
		credit = []string{profile.ID}
	}
	a, err := s.Store.CreateArtifact(ctx, store.Artifact{ID: in.ID, Title: in.Title, Description: in.Description, Visibility: "listed"}, credit...)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "artifact_exists", "That ID is taken")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(ctx, actor(r), "artifact.create", a.ID, nil)
	httpx.JSON(w, 201, a)
}

func (s *Server) studioArtifact(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editable(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	ctx := r.Context()
	credits, err := s.Store.ArtifactCreators(ctx, []string{a.ID})
	if err != nil {
		unavailable(w, err)
		return
	}
	rels, err := s.releaseItems(ctx, a.ID)
	if err != nil {
		unavailable(w, err)
		return
	}
	ids := []string{}
	for _, c := range credits[a.ID] {
		ids = append(ids, c.ID)
	}
	httpx.OK(w, map[string]any{"artifact": a, "creators": ids, "releases": rels})
}

func (s *Server) studioPatchArtifact(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editable(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var in struct {
		Title        *string   `json:"title"`
		Description  *string   `json:"description"`
		Tags         []string  `json:"tags"`
		Visibility   *string   `json:"visibility"`
		Featured     *bool     `json:"featured"`
		SortOrder    *int      `json:"sortOrder"`
		RequiredPlan *string   `json:"requiredPlan"`
		DemoEnabled  *bool     `json:"demoEnabled"`
		Creators     *[]string `json:"creators"`
	}
	if err := httpx.Read(w, r, &in, 16<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if in.Title != nil {
		a.Title = strings.TrimSpace(*in.Title)
	}
	if in.Description != nil {
		a.Description = *in.Description
	}
	if in.Tags != nil {
		a.Tags = normalizeTags(in.Tags)
	}
	if in.Visibility != nil {
		a.Visibility = *in.Visibility
	}
	if in.Featured != nil {
		a.Featured = *in.Featured
	}
	if in.SortOrder != nil {
		a.SortOrder = *in.SortOrder
	}
	if in.RequiredPlan != nil {
		a.RequiredPlan = *in.RequiredPlan
	}
	if in.DemoEnabled != nil {
		a.DemoEnabled = *in.DemoEnabled
	}
	if a.Title == "" || len(a.Title) > 120 || len(a.Description) > 2000 || (a.Visibility != "listed" && a.Visibility != "hidden") ||
		(a.RequiredPlan != "" && !planKeyPattern.MatchString(a.RequiredPlan)) || a.SortOrder < -10000 || a.SortOrder > 10000 {
		httpx.Fail(w, 400, "invalid_request", "Invalid title, description, visibility, plan or sort order")
		return
	}
	ctx := r.Context()
	if in.RequiredPlan != nil && a.RequiredPlan != "" {
		rev, err := s.Store.ActiveSettings(ctx)
		if err != nil {
			unavailable(w, err)
			return
		}
		if _, ok := rev.Document.Plan(a.RequiredPlan); !ok {
			httpx.Fail(w, 400, "invalid_request", "Unknown plan "+a.RequiredPlan)
			return
		}
	}
	if in.Creators != nil {
		if err := s.checkCredits(ctx, r, *in.Creators); err != nil {
			httpx.Fail(w, 400, "invalid_request", err.Error())
			return
		}
	}
	a, err := s.Store.UpdateArtifact(ctx, a)
	if err != nil {
		unavailable(w, err)
		return
	}
	if in.Creators != nil {
		if err = s.Store.SetArtifactCreators(ctx, a.ID, *in.Creators); err != nil {
			unavailable(w, err)
			return
		}
	}
	_ = s.Store.Audit(ctx, actor(r), "artifact.update", a.ID, map[string]any{"visibility": a.Visibility, "requiredPlan": a.RequiredPlan, "demo": a.DemoEnabled})
	a, _ = s.Store.Artifact(ctx, a.ID)
	httpx.OK(w, a)
}

// checkCredits validates a new credit list: known creators, no duplicates,
// at least one, and a creator may not remove their own credit (they would
// lose access to the artifact they are editing).
func (s *Server) checkCredits(ctx context.Context, r *http.Request, ids []string) error {
	if len(ids) == 0 || len(ids) > 12 {
		return errors.New("credit between one and twelve creators")
	}
	all, err := s.Store.Creators(ctx)
	if err != nil {
		return err
	}
	known := map[string]store.Creator{}
	for _, c := range all {
		known[c.ID] = c
	}
	seen := map[string]bool{}
	self := isAdmin(r)
	for _, id := range ids {
		c, ok := known[id]
		if !ok || seen[id] {
			return errors.Errorf("unknown or repeated creator %q", id)
		}
		seen[id] = true
		if c.UserID == me(r).ID {
			self = true
		}
	}
	if !self {
		return errors.New("you cannot remove your own credit")
	}
	return nil
}

func normalizeTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && len(t) <= 32 && !seen[t] && len(out) < 12 {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// checkImage verifies that id is empty or an image (avatars render as <img>).
func (s *Server) checkImage(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	m, err := s.Store.Media(ctx, id)
	if err != nil {
		return errors.Errorf("unknown media %q", id)
	}
	if content.IsVideo(m.MediaType) {
		return errors.New("an avatar must be an image")
	}
	return nil
}

// checkMedia verifies that every referenced media ID exists.
func (s *Server) checkMedia(ctx context.Context, ids ...string) error {
	want := []string{}
	for _, id := range ids {
		if id != "" {
			want = append(want, id)
		}
	}
	found, err := s.Store.MediaByIDs(ctx, want)
	if err != nil {
		return err
	}
	for _, id := range want {
		if _, ok := found[id]; !ok {
			return errors.Errorf("unknown media %q", id)
		}
	}
	return nil
}

func (s *Server) studioSaveDraft(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editable(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	var in store.Page
	if err := httpx.Read(w, r, &in, maxBodyBytes+16<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if len(in.Body) > maxBodyBytes || len(in.Gallery) > maxGallery {
		httpx.Fail(w, 400, "invalid_request", "The write-up is limited to 100 KB and the gallery to 24 items")
		return
	}
	if err := s.checkMedia(r.Context(), append([]string{in.Cover}, in.Gallery...)...); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if err := s.Store.SavePageDraft(r.Context(), a.ID, in); err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"saved": true})
}

func (s *Server) studioPublishDraft(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editable(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	err := s.Store.PublishPageDraft(r.Context(), a.ID)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "no_draft", "There is no draft to publish")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "artifact.page.publish", a.ID, nil)
	httpx.OK(w, map[string]bool{"published": true})
}

func (s *Server) studioDiscardDraft(w http.ResponseWriter, r *http.Request) {
	a, ok := s.editable(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := s.Store.DiscardPageDraft(r.Context(), a.ID); err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"discarded": true})
}

// releaseItems lists an artifact's releases (all states) with pin counts.
func (s *Server) releaseItems(ctx context.Context, artifact string) ([]map[string]any, error) {
	rels, err := s.Store.Releases(ctx, artifact)
	if err != nil {
		return nil, err
	}
	pinned := map[string]int{}
	rows, err := s.Store.Pool.Query(ctx, `SELECT release_id,count(*) FROM instances WHERE kind='workspace' GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		var n int
		if rows.Scan(&id, &n) == nil {
			pinned[id] = n
		}
	}
	rows.Close()
	list := []map[string]any{}
	for _, rel := range rels {
		item := publicRelease(rel)
		item["createdAt"] = rel.CreatedAt
		item["withdrawnAt"] = rel.WithdrawnAt
		item["freeSample"] = rel.FreeSample
		item["pinnedInstances"] = pinned[rel.ID]
		item["manifest"] = rel.Manifest
		atts := publicAttachments(rel)
		for i, a := range atts {
			a["href"] = "/api/studio/attachments/" + rel.Attachments[i].ID + "/download"
			a["createdAt"] = rel.Attachments[i].CreatedAt
		}
		item["attachments"] = atts
		list = append(list, item)
	}
	return list, nil
}

func (s *Server) studioReleases(w http.ResponseWriter, r *http.Request) {
	artifact := r.URL.Query().Get("artifact")
	if artifact != "" {
		if _, ok := s.editable(w, r, artifact); !ok {
			return
		}
	} else if !isAdmin(r) {
		httpx.Fail(w, 400, "invalid_request", "Name an artifact")
		return
	}
	list, err := s.releaseItems(r.Context(), artifact)
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

// mayUpload decides whether the caller may add a release for artifactID:
// editors of an existing artifact, or anyone with a creator profile (or an
// administrator) for a new one.
// It returns whether the artifact is new, the caller's creator profile (nil if none) and
// whether the upload may proceed.
func (s *Server) mayUpload(w http.ResponseWriter, r *http.Request, artifactID string) (bool, *store.Creator, bool) {
	ctx := r.Context()
	_, err := s.Store.Artifact(ctx, artifactID)
	if err == nil {
		if !s.canEditArtifact(ctx, me(r), artifactID) {
			httpx.Fail(w, 403, "not_editor", "You are not credited on this artifact")
			return false, nil, false
		}
		return false, nil, true
	}
	if !errors.Is(err, store.ErrNotFound) {
		unavailable(w, err)
		return false, nil, false
	}
	c, has, err := s.ownProfile(ctx, me(r))
	if err != nil {
		unavailable(w, err)
		return false, nil, false
	}
	if !isAdmin(r) && !has {
		httpx.Fail(w, 409, "creator_profile_required", "Ask an administrator to link your account to a creator profile first")
		return false, nil, false
	}
	if has {
		return true, &c, true
	}
	return true, nil, true
}

func (s *Server) studioCheck(w http.ResponseWriter, r *http.Request) {
	u, err := s.upload(w, r)
	if err != nil {
		httpx.Fail(w, 400, "invalid_upload", err.Error())
		return
	}
	if _, _, ok := s.mayUpload(w, r, u.ArtifactID); !ok {
		return
	}
	manifest, checks, err := s.Publisher.Check(r.Context(), u)
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]any{"checks": checks, "blocking": releases.Blocking(checks), "manifest": manifest, "size": len(u.HTML)})
}

func (s *Server) studioUpload(w http.ResponseWriter, r *http.Request) {
	u, err := s.upload(w, r)
	if err != nil {
		httpx.Fail(w, 400, "invalid_upload", err.Error())
		return
	}
	ctx := r.Context()
	var rel store.Release
	var checks []releases.Check
	for try := 0; ; try++ {
		isNew, profile, ok := s.mayUpload(w, r, u.ArtifactID)
		if !ok {
			return
		}
		u.Creators = nil
		if isNew && profile != nil {
			u.Creators = []string{profile.ID}
		}
		rel, checks, err = s.Publisher.CreateDraft(ctx, actor(r), u)
		// A concurrent upload created the artifact (crediting its creator)
		// after mayUpload: authorize again as an editor of that artifact.
		if !errors.Is(err, releases.ErrArtifactTaken) || try == 1 {
			break
		}
	}
	var invalid *releases.ErrInvalid
	switch {
	case errors.As(err, &invalid):
		httpx.JSON(w, 422, map[string]any{"error": httpx.Failure{Code: "invalid_release", Message: invalid.Error()}, "checks": checks})
		return
	case errors.Is(err, store.ErrConflict):
		httpx.JSON(w, 409, map[string]any{"error": httpx.Failure{Code: "version_exists", Message: err.Error()}, "checks": checks})
		return
	case err != nil:
		unavailable(w, err)
		return
	}
	item := publicRelease(rel)
	item["checks"] = checks
	httpx.JSON(w, 201, item)
}

// releaseFor loads a release whose artifact the caller may edit.
func (s *Server) releaseFor(w http.ResponseWriter, r *http.Request) (store.Release, bool) {
	rel, err := s.Store.Release(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "Release not found")
		return rel, false
	}
	if err != nil {
		unavailable(w, err)
		return rel, false
	}
	if !s.canEditArtifact(r.Context(), me(r), rel.ArtifactID) {
		httpx.Fail(w, 403, "not_editor", "You are not credited on this artifact")
		return rel, false
	}
	return rel, true
}

func (s *Server) studioPublish(w http.ResponseWriter, r *http.Request) {
	rel, ok := s.releaseFor(w, r)
	if !ok {
		return
	}
	rel, err := s.Store.PublishRelease(r.Context(), rel.ID, actor(r))
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "conflict", "Only a draft can be published")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, publicRelease(rel))
}

func (s *Server) studioWithdraw(w http.ResponseWriter, r *http.Request) {
	rel, ok := s.releaseFor(w, r)
	if !ok {
		return
	}
	var in struct {
		SuspendPinned bool `json:"suspendPinned"`
	}
	_ = httpx.Read(w, r, &in, 1024)
	n, err := s.Store.WithdrawRelease(r.Context(), rel.ID, actor(r), in.SuspendPinned)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "conflict", "Only a published release can be withdrawn")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	if in.SuspendPinned {
		list, _ := s.Store.AllInstances(r.Context(), "", 500)
		for _, i := range list {
			if i.ReleaseID == rel.ID {
				s.Runtime.RevokeInstance(i.ID)
			}
		}
	}
	httpx.OK(w, map[string]any{"withdrawn": true, "suspendedInstances": n})
}

// studioPreview creates an editor-owned review instance on an app origin; the
// uploaded HTML never runs in the portal DOM.
func (s *Server) studioPreview(w http.ResponseWriter, r *http.Request) {
	rel, ok := s.releaseFor(w, r)
	if !ok {
		return
	}
	inst, err := s.Store.PreviewInstance(r.Context(), actor(r), rel.ID)
	if errors.Is(err, store.ErrNotFound) {
		inst, err = s.Store.CreateInstance(r.Context(), actor(r), rel.ID, "Preview · "+rel.ArtifactID+" "+rel.Version, "preview", 0)
		if err == nil {
			_ = s.Store.Audit(r.Context(), actor(r), "release.preview", rel.ID, map[string]any{"instance": inst.ID})
		}
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, s.instanceJSON(r, inst))
}

func (s *Server) studioDeleteDraft(w http.ResponseWriter, r *http.Request) {
	rel, ok := s.releaseFor(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteDraft(r.Context(), rel.ID); err != nil {
		httpx.Fail(w, 409, "conflict", "Only an unreferenced draft can be deleted")
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "release.delete_draft", rel.ID, nil)
	httpx.OK(w, map[string]bool{"deleted": true})
}

func (s *Server) studioMedia(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.MediaList(r.Context(), "")
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

func (s *Server) studioUploadMedia(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	limit := rev.Document.Limits.MaxMediaBytes
	f, _, ok := s.uploadedFile(w, r, limit)
	if !ok {
		return
	}
	defer func() { _ = f.Close() }()
	info, err := content.DetectMedia(f)
	if err != nil {
		httpx.Fail(w, 415, "media_type", err.Error())
		return
	}
	alt := strings.TrimSpace(r.FormValue("alt"))
	if len(alt) > 300 {
		httpx.Fail(w, 400, "invalid_request", "Alt text is limited to 300 characters")
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
	m, err := s.Store.CreateMedia(r.Context(), store.Media{SHA256: sum, StorageKey: key, MediaType: info.Type, Size: size, Width: info.Width, Height: info.Height, Alt: alt, UploadedBy: actor(r)})
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.JSON(w, 201, m)
}

// uploadedFile parses a multipart upload of at most limit bytes and returns
// its "file" part. Parts beyond 1 MiB are spooled to disk by the parser, so
// large files are never held in memory.
func (s *Server) uploadedFile(w http.ResponseWriter, r *http.Request, limit int64) (multipart.File, *multipart.FileHeader, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Fail(w, 413, "too_large", "The file exceeds "+mib(limit))
		} else {
			httpx.Fail(w, 400, "invalid_upload", "The upload form is malformed")
		}
		return nil, nil, false
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		httpx.Fail(w, 400, "invalid_upload", "Choose a file")
		return nil, nil, false
	}
	return f, h, true
}

func mib(n int64) string { return strconv.FormatInt(n>>20, 10) + " MiB" }

// ownMedia loads media the caller uploaded (or any media for admins).
func (s *Server) ownMedia(w http.ResponseWriter, r *http.Request) (store.Media, bool) {
	m, err := s.Store.Media(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Media not found")
		return m, false
	}
	if !isAdmin(r) && m.UploadedBy != actor(r) {
		httpx.Fail(w, 403, "not_uploader", "Only the uploader or an administrator can change this media")
		return m, false
	}
	return m, true
}

func (s *Server) studioPatchMedia(w http.ResponseWriter, r *http.Request) {
	m, ok := s.ownMedia(w, r)
	if !ok {
		return
	}
	var in struct {
		Alt    *string `json:"alt"`
		Poster *string `json:"poster"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || (in.Alt != nil && len(*in.Alt) > 300) {
		httpx.Fail(w, 400, "invalid_request", "Alt text is limited to 300 characters")
		return
	}
	ctx := r.Context()
	if in.Poster != nil && *in.Poster != "" {
		p, err := s.Store.Media(ctx, *in.Poster)
		if !content.IsVideo(m.MediaType) || err != nil || content.IsVideo(p.MediaType) {
			httpx.Fail(w, 400, "invalid_poster", "A poster is an image from the media library, set on a video")
			return
		}
	}
	if in.Alt != nil {
		if err := s.Store.SetMediaAlt(ctx, m.ID, strings.TrimSpace(*in.Alt)); err != nil {
			unavailable(w, err)
			return
		}
	}
	if in.Poster != nil {
		if err := s.Store.SetMediaPoster(ctx, m.ID, *in.Poster); err != nil {
			unavailable(w, err)
			return
		}
	}
	httpx.OK(w, map[string]bool{"saved": true})
}

func (s *Server) studioDeleteMedia(w http.ResponseWriter, r *http.Request) {
	m, ok := s.ownMedia(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteMedia(r.Context(), m.ID)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "media_in_use", "The media is still used by a page, post or profile")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, map[string]bool{"deleted": true})
}

func (s *Server) studioPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.Store.Posts(ctx, "", "", 0)
	if err != nil {
		unavailable(w, err)
		return
	}
	out := []store.Post{}
	for _, p := range list {
		if s.canEditPost(ctx, me(r), p) {
			out = append(out, p)
		}
	}
	httpx.OK(w, out)
}

type postInput struct {
	Slug     *string  `json:"slug"`
	Title    *string  `json:"title"`
	Summary  *string  `json:"summary"`
	Body     *string  `json:"body"`
	Cover    *string  `json:"cover"`
	AuthorID *string  `json:"authorId"`
	Tags     []string `json:"tags"`
}

// applyPost merges input into p and validates it. Creators may only author
// posts as their own profile.
func (s *Server) applyPost(ctx context.Context, r *http.Request, p *store.Post, in postInput) error {
	if in.Slug != nil {
		p.Slug = strings.TrimSpace(*in.Slug)
	}
	if in.Title != nil {
		p.Title = strings.TrimSpace(*in.Title)
	}
	if in.Summary != nil {
		p.Summary = strings.TrimSpace(*in.Summary)
	}
	if in.Body != nil {
		p.Body = *in.Body
	}
	if in.Cover != nil {
		p.Cover = *in.Cover
	}
	if in.Tags != nil {
		p.Tags = normalizeTags(in.Tags)
	}
	if in.AuthorID != nil {
		p.AuthorID = *in.AuthorID
	}
	if !slugPattern.MatchString(p.Slug) || p.Title == "" || len(p.Title) > 160 || len(p.Summary) > 400 || len(p.Body) > maxBodyBytes {
		return errors.New("a slug of lowercase letters, digits and -, a title of at most 160 characters, a summary of at most 400 and a body of at most 100 KB are required")
	}
	if err := s.checkMedia(ctx, p.Cover); err != nil {
		return err
	}
	if p.AuthorID != "" {
		if _, err := s.Store.Creator(ctx, p.AuthorID); err != nil {
			return errors.New("unknown author")
		}
	}
	if !isAdmin(r) && in.AuthorID != nil {
		own, has, err := s.ownProfile(ctx, me(r))
		if err != nil || !has || own.ID != p.AuthorID {
			return errors.New("you can only author posts as your own profile")
		}
	}
	return nil
}

func (s *Server) studioCreatePost(w http.ResponseWriter, r *http.Request) {
	var in postInput
	if err := httpx.Read(w, r, &in, maxBodyBytes+16<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	ctx := r.Context()
	p := store.Post{CreatedBy: actor(r)}
	if own, has, err := s.ownProfile(ctx, me(r)); err != nil {
		unavailable(w, err)
		return
	} else if has {
		p.AuthorID = own.ID
	}
	if err := s.applyPost(ctx, r, &p, in); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	p, err := s.Store.CreatePost(ctx, p)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "slug_taken", "That slug is already used")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.JSON(w, 201, p)
}

// editablePost loads a post the caller may edit.
func (s *Server) editablePost(w http.ResponseWriter, r *http.Request) (store.Post, bool) {
	p, err := s.Store.Post(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "Post not found")
		return p, false
	}
	if err != nil {
		unavailable(w, err)
		return p, false
	}
	if !s.canEditPost(r.Context(), me(r), p) {
		httpx.Fail(w, 403, "not_editor", "You did not write this post")
		return p, false
	}
	return p, true
}

func (s *Server) studioPost(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.editablePost(w, r); ok {
		httpx.OK(w, p)
	}
}

func (s *Server) studioPatchPost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.editablePost(w, r)
	if !ok {
		return
	}
	var in postInput
	if err := httpx.Read(w, r, &in, maxBodyBytes+16<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if err := s.applyPost(r.Context(), r, &p, in); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	p, err := s.Store.UpdatePost(r.Context(), p)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "slug_taken", "That slug is already used")
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, p)
}

func (s *Server) studioDeletePost(w http.ResponseWriter, r *http.Request) {
	p, ok := s.editablePost(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeletePost(r.Context(), p.ID); err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "post.delete", p.ID, map[string]any{"slug": p.Slug})
	httpx.OK(w, map[string]bool{"deleted": true})
}

func (s *Server) studioPostState(state string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.editablePost(w, r)
		if !ok {
			return
		}
		p, err := s.Store.SetPostState(r.Context(), p.ID, state)
		if err != nil {
			unavailable(w, err)
			return
		}
		_ = s.Store.Audit(r.Context(), actor(r), "post."+map[string]string{"published": "publish", "draft": "unpublish"}[state], p.ID, nil)
		httpx.OK(w, p)
	}
}

func (s *Server) studioCreators(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Creators(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

type creatorInput struct {
	Slug      *string      `json:"slug"`
	Name      *string      `json:"name"`
	Bio       *string      `json:"bio"`
	Avatar    *string      `json:"avatar"`
	Links     []store.Link `json:"links"`
	UserID    *string      `json:"userId"`
	SortOrder *int         `json:"sortOrder"`
}

func (s *Server) applyCreator(ctx context.Context, r *http.Request, c *store.Creator, in creatorInput) error {
	if (in.UserID != nil || in.SortOrder != nil) && !isAdmin(r) {
		return errors.New("only administrators link accounts and order creators")
	}
	if in.Slug != nil {
		c.Slug = strings.TrimSpace(*in.Slug)
	}
	if in.Name != nil {
		c.Name = strings.TrimSpace(*in.Name)
	}
	if in.Bio != nil {
		c.Bio = *in.Bio
	}
	if in.Avatar != nil {
		c.Avatar = *in.Avatar
	}
	if in.Links != nil {
		c.Links = in.Links
	}
	if in.UserID != nil {
		c.UserID = *in.UserID
	}
	if in.SortOrder != nil {
		c.SortOrder = *in.SortOrder
	}
	if !slugPattern.MatchString(c.Slug) || c.Name == "" || len(c.Name) > 80 || len(c.Bio) > 20<<10 || len(c.Links) > 8 {
		return errors.New("a slug of lowercase letters, digits and -, a name of at most 80 characters, a bio of at most 20 KB and at most 8 links are required")
	}
	for _, l := range c.Links {
		u, err := url.Parse(l.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || l.Label == "" || len(l.Label) > 40 || len(l.URL) > 500 {
			return errors.Errorf("link %q needs a label and an http(s) URL", l.Label)
		}
	}
	if err := s.checkImage(ctx, c.Avatar); err != nil {
		return err
	}
	if c.UserID != "" {
		if _, err := s.Store.User(ctx, c.UserID); err != nil {
			return errors.New("unknown user")
		}
	}
	return nil
}

func (s *Server) studioCreateCreator(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		httpx.Fail(w, 403, "admin_required", "Only administrators create creator profiles")
		return
	}
	var in creatorInput
	if err := httpx.Read(w, r, &in, 32<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	c := store.Creator{}
	if err := s.applyCreator(r.Context(), r, &c, in); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	c, err := s.Store.CreateCreator(r.Context(), c)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "slug_taken", err.Error())
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "creator.create", c.ID, map[string]any{"user": c.UserID})
	httpx.JSON(w, 201, c)
}

func (s *Server) studioPatchCreator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, err := s.Store.Creator(ctx, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Creator not found")
		return
	}
	if !isAdmin(r) && c.UserID != actor(r) {
		httpx.Fail(w, 403, "not_editor", "You can only edit your own profile")
		return
	}
	var in creatorInput
	if err = httpx.Read(w, r, &in, 32<<10); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if err = s.applyCreator(ctx, r, &c, in); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	c, err = s.Store.UpdateCreator(ctx, c)
	if errors.Is(err, store.ErrConflict) {
		httpx.Fail(w, 409, "slug_taken", err.Error())
		return
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(ctx, actor(r), "creator.update", c.ID, map[string]any{"user": c.UserID})
	httpx.OK(w, c)
}

func (s *Server) studioDeleteCreator(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		httpx.Fail(w, 403, "admin_required", "Only administrators delete creator profiles")
		return
	}
	if err := s.Store.DeleteCreator(r.Context(), r.PathValue("id")); err != nil {
		httpx.Fail(w, 404, "not_found", "Creator not found")
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "creator.delete", r.PathValue("id"), nil)
	httpx.OK(w, map[string]bool{"deleted": true})
}

// studioMarkdown renders with the public renderer so previews match pages.
func (s *Server) studioMarkdown(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Markdown string `json:"markdown"`
	}
	if err := httpx.Read(w, r, &in, maxBodyBytes+4096); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	httpx.OK(w, map[string]string{"html": string(s.markdown(r.Context(), in.Markdown))})
}

// studioPlans lists every plan (including grant-only ones) for the required
// plan picker.
func (s *Server) studioPlans(w http.ResponseWriter, r *http.Request) {
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		unavailable(w, err)
		return
	}
	out := []map[string]any{}
	for _, p := range rev.Document.Plans {
		out = append(out, map[string]any{"key": p.Key, "name": p.Name, "rank": p.Rank, "public": p.Public})
	}
	httpx.OK(w, out)
}
