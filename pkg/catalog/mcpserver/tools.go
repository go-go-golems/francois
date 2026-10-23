package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/pkg/errors"
)

const (
	scopeStudio  = "studio"
	scopeAccount = "account"
	scopeRead    = "admin:read"
	scopeWrite   = "admin:write"
)

// Read-only and changing tools; destructive ones remove or end something.
const (
	ro          = true
	rw          = false
	destructive = true
	additive    = false
)

type idIn struct {
	ID string `json:"id" jsonschema:"the object's ID"`
}

func catalogue() []spec {
	var all []spec
	all = append(all, identityTools()...)
	all = append(all, studioTools()...)
	all = append(all, customerTools()...)
	all = append(all, accountTools()...)
	return all
}

func identityTools() []spec {
	return []spec{
		tool("whoami", "", "", ro, additive, "Show which user this server acts as, their role, and the token's scopes. Call this first to learn what you may do.",
			func(ctx context.Context, e *env, _ none) (client.Me, string, error) {
				c, err := e.client(ctx)
				if err != nil {
					return client.Me{}, "", err
				}
				me, err := c.Me(ctx)
				return me, fmt.Sprintf("Acting as %s (%s).", me.User.Name, me.User.Role), err
			}),
	}
}

// --- studio -------------------------------------------------------------

type artifactPatchIn struct {
	ID               string    `json:"id" jsonschema:"artifact ID"`
	Title            *string   `json:"title,omitempty"`
	Description      *string   `json:"description,omitempty"`
	Tags             *[]string `json:"tags,omitempty" jsonschema:"replaces all tags; [] clears them"`
	Visibility       *string   `json:"visibility,omitempty" jsonschema:"listed or hidden"`
	Featured         *bool     `json:"featured,omitempty"`
	SortOrder        *int      `json:"sortOrder,omitempty"`
	EarlyAccessUntil *string   `json:"earlyAccessUntil,omitempty" jsonschema:"RFC3339 date; empty clears early access"`
	DemoEnabled      *bool     `json:"demoEnabled,omitempty"`
	Creators         *[]string `json:"creators,omitempty" jsonschema:"creator profile IDs; replaces all"`
}

type pageIn struct {
	ID      string   `json:"id" jsonschema:"artifact ID"`
	Body    string   `json:"body" jsonschema:"Markdown body of the public page"`
	Cover   string   `json:"cover,omitempty" jsonschema:"cover media ID"`
	Gallery []string `json:"gallery,omitempty" jsonschema:"gallery media IDs"`
}

type releasesIn struct {
	Artifact string `json:"artifact,omitempty" jsonschema:"artifact ID; required for creators"`
}

type releaseIn struct {
	ArtifactID   string          `json:"artifactId"`
	Version      string          `json:"version" jsonschema:"semantic version, e.g. 1.3.0"`
	HTMLPath     string          `json:"htmlPath,omitempty" jsonschema:"local HTML file (local server only)"`
	HTML         string          `json:"html,omitempty" jsonschema:"the HTML document itself (instead of htmlPath)"`
	ManifestPath string          `json:"manifestPath,omitempty" jsonschema:"local capability manifest JSON file (local server only)"`
	Manifest     json.RawMessage `json:"manifest,omitempty" jsonschema:"capability manifest object (instead of manifestPath)"`
	Title        string          `json:"title,omitempty" jsonschema:"title, for a new artifact"`
	Description  string          `json:"description,omitempty" jsonschema:"description, for a new artifact"`
	Tags         []string        `json:"tags,omitempty"`
	Changelog    string          `json:"changelog,omitempty"`
	FreeSample   bool            `json:"freeSample,omitempty" jsonschema:"downloadable without a subscription"`
}

type pushIn struct {
	releaseIn
	AttachmentPaths []string `json:"attachmentPaths,omitempty" jsonschema:"local files to attach (local server only)"`
	Publish         bool     `json:"publish,omitempty" jsonschema:"publish after the draft is created"`
}

func (e *env) upload(r releaseIn) (client.ReleaseUpload, error) {
	u := client.ReleaseUpload{ArtifactID: r.ArtifactID, Version: r.Version, Title: r.Title, Description: r.Description, Tags: r.Tags, Changelog: r.Changelog, FreeSample: r.FreeSample}
	switch {
	case r.HTMLPath != "":
		html, err := e.readLocal(r.HTMLPath)
		if err != nil {
			return u, err
		}
		u.HTML = html
	case r.HTML != "":
		u.HTML = []byte(r.HTML)
	default:
		return u, errors.New("give htmlPath or html")
	}
	switch {
	case r.ManifestPath != "":
		m, err := e.readLocal(r.ManifestPath)
		if err != nil {
			return u, err
		}
		u.Manifest = m
	case len(r.Manifest) > 0 && string(r.Manifest) != "null":
		u.Manifest = r.Manifest
	}
	return u, nil
}

type withdrawIn struct {
	ID            string `json:"id" jsonschema:"release ID"`
	SuspendPinned bool   `json:"suspendPinned,omitempty" jsonschema:"also suspend workspaces pinned to this release"`
}

type uploadIn struct {
	Path          string `json:"path,omitempty" jsonschema:"local file (local server only)"`
	ContentBase64 string `json:"contentBase64,omitempty" jsonschema:"file content, base64 (≤10 MiB), with filename"`
	Filename      string `json:"filename,omitempty"`
}

type attachIn struct {
	ReleaseID string `json:"releaseId"`
	Label     string `json:"label,omitempty" jsonschema:"label shown to subscribers"`
	uploadIn
}

type mediaIn struct {
	Alt string `json:"alt,omitempty" jsonschema:"alt text"`
	uploadIn
}

type mediaPatchIn struct {
	ID     string  `json:"id"`
	Alt    *string `json:"alt,omitempty"`
	Poster *string `json:"poster,omitempty" jsonschema:"poster image media ID (videos)"`
}

type postIn struct {
	ID string `json:"id,omitempty" jsonschema:"post ID (update only)"`
	client.PostInput
}

type creatorIn struct {
	ID string `json:"id,omitempty" jsonschema:"creator ID (update only)"`
	client.CreatorInput
}

type markdownIn struct {
	Markdown string `json:"markdown"`
}

type html struct {
	HTML string `json:"html"`
}

func studioTools() []spec {
	st := func(ctx context.Context, e *env) (client.Studio, error) {
		c, err := e.client(ctx)
		if err != nil {
			return client.Studio{}, err
		}
		return c.Studio, nil
	}
	return []spec{
		tool("list_artifacts", ToolsetStudio, scopeStudio, ro, additive, "List the artifacts (hosted HTML applications) you may edit, with release counts and latest version.",
			func(ctx context.Context, e *env, _ none) (items[client.ArtifactRow], string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return items[client.ArtifactRow]{}, "", err
				}
				l, err := s.Artifacts(ctx)
				return items[client.ArtifactRow]{l}, fmt.Sprintf("%d artifacts.", len(l)), err
			}),
		tool("get_artifact", ToolsetStudio, scopeStudio, ro, additive, "Show an artifact: metadata, public page, page draft, creators and releases.",
			func(ctx context.Context, e *env, in idIn) (client.ArtifactDetail, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.ArtifactDetail{}, "", err
				}
				d, err := s.Artifact(ctx, in.ID)
				return d, "", err
			}),
		tool("create_artifact", ToolsetStudio, scopeStudio, rw, additive, "Create an empty artifact. Usually push_release creates the artifact with its first release instead.",
			func(ctx context.Context, e *env, in client.ArtifactCreate) (client.Artifact, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Artifact{}, "", err
				}
				a, err := s.CreateArtifact(ctx, in)
				return a, "Created " + a.ID + ".", err
			}),
		tool("update_artifact", ToolsetStudio, scopeStudio, rw, additive, "Change an artifact's title, description, tags, visibility, featured flag, order, early-access end date, demo flag or creators. Only the fields given change.",
			func(ctx context.Context, e *env, in artifactPatchIn) (client.Artifact, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Artifact{}, "", err
				}
				a, err := s.PatchArtifact(ctx, in.ID, client.ArtifactPatch{Title: in.Title, Description: in.Description, Tags: in.Tags, Visibility: in.Visibility,
					Featured: in.Featured, SortOrder: in.SortOrder, EarlyAccessUntil: in.EarlyAccessUntil, DemoEnabled: in.DemoEnabled, Creators: in.Creators})
				return a, "Updated " + in.ID + ".", err
			}),
		tool("save_page_draft", ToolsetStudio, scopeStudio, rw, additive, "Save a draft of an artifact's public page (Markdown body, cover, gallery). The public page changes only after publish_page_draft.",
			func(ctx context.Context, e *env, in pageIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				g := in.Gallery
				if g == nil {
					g = []string{}
				}
				err = s.SaveDraft(ctx, in.ID, client.Page{Body: in.Body, Cover: in.Cover, Gallery: g})
				return done{err == nil}, "Draft saved.", err
			}),
		tool("publish_page_draft", ToolsetStudio, scopeStudio, rw, additive, "Publish an artifact's page draft to its public page.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.PublishDraft(ctx, in.ID)
				return done{err == nil}, "Page published.", err
			}),
		tool("discard_page_draft", ToolsetStudio, scopeStudio, rw, destructive, "Discard an artifact's page draft.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DiscardDraft(ctx, in.ID)
				return done{err == nil}, "Draft discarded.", err
			}),
		tool("list_releases", ToolsetStudio, scopeStudio, ro, additive, "List releases of an artifact with state, attachments and pinned instances.",
			func(ctx context.Context, e *env, in releasesIn) (items[client.Release], string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return items[client.Release]{}, "", err
				}
				l, err := s.Releases(ctx, in.Artifact)
				return items[client.Release]{l}, fmt.Sprintf("%d releases.", len(l)), err
			}),
		tool("check_release", ToolsetStudio, scopeStudio, ro, additive, "Validate an HTML release without storing it; returns the findings.",
			func(ctx context.Context, e *env, in releaseIn) (client.CheckResult, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.CheckResult{}, "", err
				}
				u, err := e.upload(in)
				if err != nil {
					return client.CheckResult{}, "", err
				}
				r, err := s.CheckRelease(ctx, u)
				return r, fmt.Sprintf("%d findings; blocking=%v.", len(r.Checks), r.Blocking), err
			}),
		tool("push_release", ToolsetStudio, scopeStudio, rw, additive, "Upload a new release of an artifact as a draft (creating the artifact if new), attach files, and optionally publish it.",
			func(ctx context.Context, e *env, in pushIn) (client.Release, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Release{}, "", err
				}
				u, err := e.upload(in.releaseIn)
				if err != nil {
					return client.Release{}, "", err
				}
				var paths []string
				for _, p := range in.AttachmentPaths {
					resolved, err := e.localPath(p)
					if err != nil {
						return client.Release{}, "", err
					}
					paths = append(paths, resolved)
				}
				res, err := s.Push(ctx, u, paths, in.Publish)
				return res.Release, fmt.Sprintf("Release %s %s is %s with %d attachments.", in.ArtifactID, in.Version, res.Release.State, len(res.Attachments)), err
			}),
		tool("publish_release", ToolsetStudio, scopeStudio, rw, additive, "Publish a draft release so subscribers can run it.",
			func(ctx context.Context, e *env, in idIn) (client.Release, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Release{}, "", err
				}
				r, err := s.PublishRelease(ctx, in.ID)
				return r, "Published.", err
			}),
		tool("withdraw_release", ToolsetStudio, scopeStudio, rw, destructive, "Withdraw a published release; optionally suspend workspaces pinned to it.",
			func(ctx context.Context, e *env, in withdrawIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				n, err := s.WithdrawRelease(ctx, in.ID, in.SuspendPinned)
				return done{err == nil}, fmt.Sprintf("Withdrawn; %d instances suspended.", n), err
			}),
		tool("delete_draft_release", ToolsetStudio, scopeStudio, rw, destructive, "Delete an unused draft release.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DeleteRelease(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("preview_release", ToolsetStudio, scopeStudio, rw, additive, "Open a private preview instance of a draft release; returns its origin.",
			func(ctx context.Context, e *env, in idIn) (client.Instance, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Instance{}, "", err
				}
				i, err := s.PreviewRelease(ctx, in.ID)
				return i, "Preview at " + i.Origin + ".", err
			}),
		tool("add_attachment", ToolsetStudio, scopeStudio, rw, additive, "Attach a file (manual, archive, notes) to a release. Attachments follow the release's download rule.",
			func(ctx context.Context, e *env, in attachIn) (client.Attachment, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Attachment{}, "", err
				}
				f, name, err := e.openUpload(in.Path, in.ContentBase64, in.Filename)
				if err != nil {
					return client.Attachment{}, "", err
				}
				defer func() { _ = f.Close() }()
				a, err := s.AddAttachment(ctx, in.ReleaseID, name, in.Label, f)
				return a, "Attached " + name + ".", err
			}),
		tool("delete_attachment", ToolsetStudio, scopeStudio, rw, destructive, "Remove an attachment from a release.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DeleteAttachment(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("list_media", ToolsetStudio, scopeStudio, ro, additive, "List the media library (images and videos for pages and posts).",
			func(ctx context.Context, e *env, _ none) (items[client.Media], string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return items[client.Media]{}, "", err
				}
				l, err := s.Media(ctx)
				return items[client.Media]{l}, fmt.Sprintf("%d media.", len(l)), err
			}),
		tool("upload_media", ToolsetStudio, scopeStudio, rw, additive, "Upload an image (PNG, JPEG, GIF, WebP) or video (MP4, WebM) to the media library.",
			func(ctx context.Context, e *env, in mediaIn) (client.Media, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Media{}, "", err
				}
				f, name, err := e.openUpload(in.Path, in.ContentBase64, in.Filename)
				if err != nil {
					return client.Media{}, "", err
				}
				defer func() { _ = f.Close() }()
				m, err := s.UploadMedia(ctx, name, in.Alt, f)
				return m, "Uploaded " + m.ID + ".", err
			}),
		tool("update_media", ToolsetStudio, scopeStudio, rw, additive, "Set a media item's alt text or a video's poster image.",
			func(ctx context.Context, e *env, in mediaPatchIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.PatchMedia(ctx, in.ID, in.Alt, in.Poster)
				return done{err == nil}, "Updated.", err
			}),
		tool("delete_media", ToolsetStudio, scopeStudio, rw, destructive, "Delete an unused media item.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DeleteMedia(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("list_posts", ToolsetStudio, scopeStudio, ro, additive, "List journal posts you may edit.",
			func(ctx context.Context, e *env, _ none) (items[client.Post], string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return items[client.Post]{}, "", err
				}
				l, err := s.Posts(ctx)
				return items[client.Post]{l}, fmt.Sprintf("%d posts.", len(l)), err
			}),
		tool("get_post", ToolsetStudio, scopeStudio, ro, additive, "Show a journal post with its Markdown body.",
			func(ctx context.Context, e *env, in idIn) (client.Post, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Post{}, "", err
				}
				p, err := s.Post(ctx, in.ID)
				return p, "", err
			}),
		tool("create_post", ToolsetStudio, scopeStudio, rw, additive, "Create a draft journal post (slug, title, Markdown body).",
			func(ctx context.Context, e *env, in postIn) (client.Post, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Post{}, "", err
				}
				p, err := s.CreatePost(ctx, in.PostInput)
				return p, "Created draft " + p.ID + ".", err
			}),
		tool("update_post", ToolsetStudio, scopeStudio, rw, additive, "Edit a journal post; fields given replace the old ones.",
			func(ctx context.Context, e *env, in postIn) (client.Post, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Post{}, "", err
				}
				p, err := s.PatchPost(ctx, in.ID, in.PostInput)
				return p, "Updated.", err
			}),
		tool("publish_post", ToolsetStudio, scopeStudio, rw, additive, "Publish a journal post.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.SetPostPublished(ctx, in.ID, true)
				return done{err == nil}, "Published.", err
			}),
		tool("unpublish_post", ToolsetStudio, scopeStudio, rw, additive, "Take a journal post back to draft.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.SetPostPublished(ctx, in.ID, false)
				return done{err == nil}, "Unpublished.", err
			}),
		tool("delete_post", ToolsetStudio, scopeStudio, rw, destructive, "Delete a journal post.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DeletePost(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("list_creators", ToolsetStudio, scopeStudio, ro, additive, "List creator profiles.",
			func(ctx context.Context, e *env, _ none) (items[client.Creator], string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return items[client.Creator]{}, "", err
				}
				l, err := s.Creators(ctx)
				return items[client.Creator]{l}, fmt.Sprintf("%d creators.", len(l)), err
			}),
		tool("create_creator", ToolsetStudio, scopeStudio, rw, additive, "Create a creator profile (administrators only); userId links an account.",
			func(ctx context.Context, e *env, in creatorIn) (client.Creator, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Creator{}, "", err
				}
				cr, err := s.CreateCreator(ctx, in.CreatorInput)
				return cr, "Created " + cr.ID + ".", err
			}),
		tool("update_creator", ToolsetStudio, scopeStudio, rw, additive, "Edit a creator profile; userId and sortOrder are administrator-only.",
			func(ctx context.Context, e *env, in creatorIn) (client.Creator, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return client.Creator{}, "", err
				}
				cr, err := s.PatchCreator(ctx, in.ID, in.CreatorInput)
				return cr, "Updated.", err
			}),
		tool("delete_creator", ToolsetStudio, scopeStudio, rw, destructive, "Delete a creator profile (administrators only).",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = s.DeleteCreator(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("render_markdown", ToolsetStudio, scopeStudio, ro, additive, "Render Markdown with the public page renderer, to preview a page or post.",
			func(ctx context.Context, e *env, in markdownIn) (html, string, error) {
				s, err := st(ctx, e)
				if err != nil {
					return html{}, "", err
				}
				h, err := s.Markdown(ctx, in.Markdown)
				return html{h}, "", err
			}),
	}
}

// --- customers (admin) --------------------------------------------------

type findIn struct {
	Query string `json:"query,omitempty" jsonschema:"email, name, user ID or identity subject"`
	Role  string `json:"role,omitempty" jsonschema:"subscriber, creator or admin"`
	Plan  string `json:"plan,omitempty" jsonschema:"plan key"`
	Limit int    `json:"limit,omitempty"`
}

type roleIn struct {
	UserID string `json:"userId"`
	Role   string `json:"role" jsonschema:"subscriber, creator or admin"`
}

type disabledIn struct {
	UserID   string `json:"userId"`
	Disabled bool   `json:"disabled"`
}

type revokeIn struct {
	UserID string `json:"userId"`
	Tokens bool   `json:"tokens,omitempty" jsonschema:"also revoke the user's API tokens"`
}

type grantIn struct {
	UserID string `json:"userId" jsonschema:"customer user ID from find_customers"`
	Days   int    `json:"days" jsonschema:"length of the grant in days, 1 to 3660"`
	Reason string `json:"reason" jsonschema:"why the grant is made; stored in the audit log"`
}

type userIDIn struct {
	UserID string `json:"userId"`
}

type subsIn struct {
	Status string `json:"status,omitempty" jsonschema:"active, trialing, past_due, canceled, …"`
	Limit  int    `json:"limit,omitempty"`
}

type queryIn struct {
	Query string `json:"query,omitempty" jsonschema:"instance ID, owner, title, owner email or artifact ID"`
}

type planIn struct {
	Key                string `json:"key" jsonschema:"plan key from list_plans"`
	Instances          *int   `json:"instances,omitempty" jsonschema:"workspaces per subscriber"`
	MembersPerInstance *int   `json:"membersPerInstance,omitempty"`
	DailyTokens        *int   `json:"dailyTokens,omitempty"`
}

type storefrontIn struct {
	Title   *string `json:"title,omitempty"`
	Tagline *string `json:"tagline,omitempty"`
	Intro   *string `json:"intro,omitempty" jsonschema:"Markdown"`
	// Copy changes editable site text by key; an empty string restores the
	// default. Keys are listed in the settings' copyKeys.
	Copy map[string]string `json:"copy,omitempty" jsonschema:"Site text by key (header.statement, home.hero, callout.*, runner.*, earlyaccess.*); empty restores the default"`
}

type stateIn struct {
	State string `json:"state,omitempty" jsonschema:"pending, processed, ignored or error"`
}

type usageIn struct {
	UserID string `json:"userId,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type adjustIn struct {
	UserID string `json:"userId"`
	Limit  *int   `json:"limit" jsonschema:"absolute daily generated-token ceiling; null resets to class default"`
	Reason string `json:"reason"`
}

type auditIn struct {
	Subject string `json:"subject,omitempty"`
	Actor   string `json:"actor,omitempty" jsonschema:"acting user ID"`
	Action  string `json:"action,omitempty" jsonschema:"e.g. user.access_grant"`
	Before  int64  `json:"before,omitempty" jsonschema:"entries older than this entry ID"`
	Limit   int    `json:"limit,omitempty"`
}

type raw struct {
	Data json.RawMessage `json:"data"`
}

func customerTools() []spec {
	ad := func(ctx context.Context, e *env) (client.Admin, error) {
		c, err := e.client(ctx)
		if err != nil {
			return client.Admin{}, err
		}
		return c.Admin, nil
	}
	return []spec{
		tool("admin_overview", ToolsetCustomers, scopeRead, ro, additive, "Shop overview: users, instances, releases, usage this month and billing health.",
			func(ctx context.Context, e *env, _ none) (raw, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return raw{}, "", err
				}
				r, err := a.Overview(ctx)
				return raw{r}, "", err
			}),
		tool("find_customers", ToolsetCustomers, scopeRead, ro, additive, "Search customers by email, name or ID, optionally by role or plan. Returns user IDs for the other customer tools.",
			func(ctx context.Context, e *env, in findIn) (items[client.UserRow], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[client.UserRow]{}, "", err
				}
				l, err := a.Users(ctx, client.UserQuery{Q: in.Query, Role: in.Role, Plan: in.Plan, Limit: in.Limit})
				return items[client.UserRow]{l}, fmt.Sprintf("%d customers.", len(l)), err
			}),
		tool("get_customer", ToolsetCustomers, scopeRead, ro, additive, "Everything about one customer: access, plan, subscriptions, grants, instances, sessions, allowance, recent calls and audit.",
			func(ctx context.Context, e *env, in userIDIn) (client.Customer, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.Customer{}, "", err
				}
				c, err := a.User(ctx, in.UserID)
				return c, "", err
			}),
		tool("set_customer_role", ToolsetCustomers, scopeWrite, rw, additive, "Set a user's role: subscriber, creator (studio access) or admin.",
			func(ctx context.Context, e *env, in roleIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.PatchUser(ctx, in.UserID, client.UserPatch{Role: &in.Role})
				return done{err == nil}, "Role set to " + in.Role + ".", err
			}),
		tool("set_customer_disabled", ToolsetCustomers, scopeWrite, rw, destructive, "Disable (or re-enable) an account. Disabling ends its live instance views.",
			func(ctx context.Context, e *env, in disabledIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.PatchUser(ctx, in.UserID, client.UserPatch{Disabled: &in.Disabled})
				return done{err == nil}, fmt.Sprintf("Disabled=%v.", in.Disabled), err
			}),
		tool("revoke_customer_sessions", ToolsetCustomers, scopeWrite, rw, destructive, "Sign a user out everywhere: end sessions and live views, and optionally revoke their API tokens.",
			func(ctx context.Context, e *env, in revokeIn) (client.Revoked, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.Revoked{}, "", err
				}
				r, err := a.RevokeSessions(ctx, in.UserID, in.Tokens)
				return r, "", err
			}),
		tool("grant_complimentary_access", ToolsetCustomers, scopeWrite, rw, additive, "Give complimentary Paying access for a number of days. Use find_customers first to get the userId.",
			func(ctx context.Context, e *env, in grantIn) (client.AccessGrant, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.AccessGrant{}, "", err
				}
				g, err := a.GrantAccess(ctx, in.UserID, client.GrantRequest{Days: in.Days, Reason: in.Reason})
				return g, fmt.Sprintf("Granted Paying access until %s.", g.Until.Format("2006-01-02")), err
			}),
		tool("revoke_access_grant", ToolsetCustomers, scopeWrite, rw, destructive, "Revoke one complimentary grant by its grant ID (see get_customer).",
			func(ctx context.Context, e *env, in idIn) (client.AccessGrant, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.AccessGrant{}, "", err
				}
				g, err := a.RevokeGrant(ctx, in.ID)
				return g, "Revoked.", err
			}),
		tool("list_customer_tokens", ToolsetCustomers, scopeRead, ro, additive, "List a user's API tokens (names, scopes, use; never secrets).",
			func(ctx context.Context, e *env, in userIDIn) (items[client.Token], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[client.Token]{}, "", err
				}
				l, err := a.UserTokens(ctx, in.UserID)
				return items[client.Token]{l}, fmt.Sprintf("%d tokens.", len(l)), err
			}),
		tool("revoke_token", ToolsetCustomers, scopeWrite, rw, destructive, "Revoke any user's API token, for example one that leaked.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.RevokeToken(ctx, in.ID)
				return done{err == nil}, "Revoked.", err
			}),
		tool("list_subscriptions", ToolsetCustomers, scopeRead, ro, additive, "List Stripe subscriptions with customer email, plan, status and period end.",
			func(ctx context.Context, e *env, in subsIn) (items[client.SubscriptionRow], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[client.SubscriptionRow]{}, "", err
				}
				l, err := a.Subscriptions(ctx, in.Status, in.Limit)
				return items[client.SubscriptionRow]{l}, fmt.Sprintf("%d subscriptions.", len(l)), err
			}),
		tool("cancel_subscription_at_period_end", ToolsetCustomers, scopeWrite, rw, destructive, "Stop a subscription from renewing; access continues until the period ends. Refunds and price changes are done in the Stripe dashboard.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.SetCancelAtPeriodEnd(ctx, in.ID, true)
				return done{err == nil}, "Cancellation scheduled.", err
			}),
		tool("resume_subscription", ToolsetCustomers, scopeWrite, rw, additive, "Withdraw a scheduled cancellation so the subscription renews.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.SetCancelAtPeriodEnd(ctx, in.ID, false)
				return done{err == nil}, "Renewal restored.", err
			}),
		tool("list_instances", ToolsetCustomers, scopeRead, ro, additive, "List all instances (running copies of releases), optionally filtered.",
			func(ctx context.Context, e *env, in queryIn) (items[client.Instance], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[client.Instance]{}, "", err
				}
				l, err := a.Instances(ctx, in.Query)
				return items[client.Instance]{l}, fmt.Sprintf("%d instances.", len(l)), err
			}),
		tool("get_instance", ToolsetCustomers, scopeRead, ro, additive, "Show an instance with members, storage usage, recent failures and audit.",
			func(ctx context.Context, e *env, in idIn) (client.Instance, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.Instance{}, "", err
				}
				i, err := a.Instance(ctx, in.ID)
				return i, "", err
			}),
		tool("suspend_instance", ToolsetCustomers, scopeWrite, rw, destructive, "Suspend an instance; its live views end.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.SetInstanceSuspended(ctx, in.ID, true)
				return done{err == nil}, "Suspended.", err
			}),
		tool("resume_instance", ToolsetCustomers, scopeWrite, rw, additive, "Resume a suspended instance.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.SetInstanceSuspended(ctx, in.ID, false)
				return done{err == nil}, "Resumed.", err
			}),
		tool("delete_instance", ToolsetCustomers, scopeWrite, rw, destructive, "Delete any instance and all its data. This cannot be undone.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.DeleteInstance(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("list_plans", ToolsetCustomers, scopeRead, ro, additive, "List subscription plans with prices, limits and credits.",
			func(ctx context.Context, e *env, _ none) (items[settings.Plan], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[settings.Plan]{}, "", err
				}
				s, err := a.Settings(ctx)
				return items[settings.Plan]{s.Active.Document.Plans}, fmt.Sprintf("%d plans.", len(s.Active.Document.Plans)), err
			}),
		tool("get_settings", ToolsetCustomers, scopeRead, ro, additive, "Show the active settings document (models, allowances, limits, plans, demos, storefront) and its revision.",
			func(ctx context.Context, e *env, _ none) (client.SettingsRevision, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.SettingsRevision{}, "", err
				}
				s, err := a.Settings(ctx)
				return s.Active, fmt.Sprintf("Revision %d.", s.Active.Revision), err
			}),
		tool("update_plan", ToolsetCustomers, scopeWrite, rw, additive, "Change Free or Paying workspace/member limits and daily generated tokens. Only the fields given change; safe against concurrent edits.",
			func(ctx context.Context, e *env, in planIn) (client.SettingsRevision, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.SettingsRevision{}, "", err
				}
				rev, err := a.EditPlan(ctx, in.Key, func(p *settings.Plan) error {
					set(&p.Instances, in.Instances)
					set(&p.MembersPerInstance, in.MembersPerInstance)
					set(&p.DailyTokens, in.DailyTokens)

					return nil
				})
				return rev, fmt.Sprintf("Settings revision %d is active.", rev.Revision), err
			}),
		tool("set_storefront_text", ToolsetCustomers, scopeWrite, rw, additive, "Change the storefront's title, tagline, introduction (Markdown) or site text (copy keys).",
			func(ctx context.Context, e *env, in storefrontIn) (client.SettingsRevision, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.SettingsRevision{}, "", err
				}
				rev, err := a.EditSettings(ctx, func(d *settings.Document) error {
					set(&d.Storefront.Title, in.Title)
					set(&d.Storefront.Tagline, in.Tagline)
					set(&d.Storefront.Intro, in.Intro)
					for k, v := range in.Copy {
						if err := d.SetCopy(k, v); err != nil {
							return err
						}
					}
					return nil
				})
				return rev, fmt.Sprintf("Settings revision %d is active.", rev.Revision), err
			}),
		tool("billing_status", ToolsetCustomers, scopeRead, ro, additive, "Billing health, recent Stripe events and checkout attempts.",
			func(ctx context.Context, e *env, in stateIn) (raw, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return raw{}, "", err
				}
				r, err := a.Billing(ctx, in.State)
				return raw{r}, "", err
			}),
		tool("reconcile_customer_billing", ToolsetCustomers, scopeWrite, rw, additive, "Re-sync a customer's subscriptions from Stripe.",
			func(ctx context.Context, e *env, in userIDIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.Reconcile(ctx, in.UserID)
				return done{err == nil}, "Reconciliation queued.", err
			}),
		tool("retry_billing_event", ToolsetCustomers, scopeWrite, rw, additive, "Retry a failed Stripe event.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.RetryEvent(ctx, in.ID)
				return done{err == nil}, "Queued.", err
			}),
		tool("usage_summary", ToolsetCustomers, scopeRead, ro, additive, "Model usage this month and recent model calls, optionally for one user.",
			func(ctx context.Context, e *env, in usageIn) (client.Usage, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return client.Usage{}, "", err
				}
				u, err := a.Usage(ctx, in.UserID, in.Limit)
				return u, "", err
			}),
		tool("set_customer_token_ceiling", ToolsetCustomers, scopeWrite, rw, additive, "Set the daily generated-token ceiling; null restores the customer class default.",
			func(ctx context.Context, e *env, in adjustIn) (done, string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.SetTokenCeiling(ctx, in.UserID, in.Limit, in.Reason)
				return done{err == nil}, "Daily token ceiling updated.", err
			}),
		tool("audit_log", ToolsetCustomers, scopeRead, ro, additive, "Read the audit log, newest first; filter by subject, actor or action. Entries made through a token carry its tokenId.",
			func(ctx context.Context, e *env, in auditIn) (items[client.AuditEntry], string, error) {
				a, err := ad(ctx, e)
				if err != nil {
					return items[client.AuditEntry]{}, "", err
				}
				l, err := a.Audit(ctx, client.AuditQuery{Subject: in.Subject, Actor: in.Actor, Action: in.Action, Before: in.Before, Limit: in.Limit})
				return items[client.AuditEntry]{l}, fmt.Sprintf("%d entries.", len(l)), err
			}),
	}
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// --- account ------------------------------------------------------------

type createInstanceIn struct {
	ReleaseID string `json:"releaseId" jsonschema:"a published release ID"`
	Title     string `json:"title,omitempty"`
}

type renameIn struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type memberIn struct {
	ID    string `json:"id" jsonschema:"instance ID"`
	Email string `json:"email,omitempty" jsonschema:"member's email (add)"`
	Role  string `json:"role,omitempty" jsonschema:"editor or viewer (add; default viewer)"`
	User  string `json:"userId,omitempty" jsonschema:"member's user ID (remove)"`
}

func accountTools() []spec {
	acc := func(ctx context.Context, e *env) (client.Account, error) {
		c, err := e.client(ctx)
		if err != nil {
			return client.Account{}, err
		}
		return c.Account, nil
	}
	return []spec{
		tool("my_account", ToolsetAccount, scopeAccount, ro, additive, "Your plan, subscriptions, complimentary grants and credit allowance.",
			func(ctx context.Context, e *env, _ none) (raw, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return raw{}, "", err
				}
				r, err := a.Get(ctx)
				return raw{r}, "", err
			}),
		tool("list_my_instances", ToolsetAccount, scopeAccount, ro, additive, "List your instances and those shared with you.",
			func(ctx context.Context, e *env, _ none) (items[client.Instance], string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return items[client.Instance]{}, "", err
				}
				l, err := a.Instances(ctx)
				return items[client.Instance]{l}, fmt.Sprintf("%d instances.", len(l)), err
			}),
		tool("create_my_instance", ToolsetAccount, scopeAccount, rw, additive, "Create a private instance of a published release. Open it in the browser to use it.",
			func(ctx context.Context, e *env, in createInstanceIn) (client.Instance, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return client.Instance{}, "", err
				}
				i, err := a.CreateInstance(ctx, in.ReleaseID, in.Title)
				return i, "Created " + i.ID + ".", err
			}),
		tool("rename_my_instance", ToolsetAccount, scopeAccount, rw, additive, "Rename one of your instances.",
			func(ctx context.Context, e *env, in renameIn) (done, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.RenameInstance(ctx, in.ID, in.Title)
				return done{err == nil}, "Renamed.", err
			}),
		tool("delete_my_instance", ToolsetAccount, scopeAccount, rw, destructive, "Delete one of your instances and its data.",
			func(ctx context.Context, e *env, in idIn) (done, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.DeleteInstance(ctx, in.ID)
				return done{err == nil}, "Deleted.", err
			}),
		tool("add_instance_member", ToolsetAccount, scopeAccount, rw, additive, "Share one of your instances with another subscriber by email.",
			func(ctx context.Context, e *env, in memberIn) (client.Member, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return client.Member{}, "", err
				}
				role := in.Role
				if role == "" {
					role = "viewer"
				}
				m, err := a.AddMember(ctx, in.ID, strings.TrimSpace(in.Email), role)
				return m, "Shared with " + m.Name + ".", err
			}),
		tool("remove_instance_member", ToolsetAccount, scopeAccount, rw, destructive, "Stop sharing one of your instances with someone.",
			func(ctx context.Context, e *env, in memberIn) (done, string, error) {
				a, err := acc(ctx, e)
				if err != nil {
					return done{}, "", err
				}
				err = a.RemoveMember(ctx, in.ID, in.User)
				return done{err == nil}, "Removed.", err
			}),
	}
}
