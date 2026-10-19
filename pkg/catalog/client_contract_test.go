package catalog

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

// TestClientContract calls the client's methods against the real server with
// strict decoding: a response field the client types do not know fails here,
// so server changes cannot silently be dropped by the client.
func TestClientContract(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	ada := e.user("ada", "admin", false)
	sam := e.user("sam", "subscriber", true)
	vera := e.user("vera", "subscriber", true)
	adminSecret, _ := e.mint(&ada, 30, "studio", "account", "admin:write")
	samSecret, _ := e.mint(&sam, 0, "account")
	opts := []client.Option{client.WithHost("portal.test"), client.WithStrictDecoding()}
	c := client.New(e.ts.URL, adminSecret, opts...)
	sc := client.New(e.ts.URL, samSecret, opts...)

	ok := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	code := func(what string, err error, want string) {
		t.Helper()
		if !client.IsCode(err, want) {
			t.Fatalf("%s: %v, want %s", what, err, want)
		}
	}

	me, err := c.Me(ctx)
	ok("me", err)
	if me.User.Role != "admin" || !me.Token.Allows("admin:read") {
		t.Fatalf("me %+v", me)
	}

	// Studio.
	s := c.Studio
	_, err = s.Me(ctx)
	ok("studio me", err)
	_, err = s.CreateArtifact(ctx, client.ArtifactCreate{ID: "loom", Title: "Loom"})
	ok("create artifact", err)
	title := "Loom 2"
	_, err = s.PatchArtifact(ctx, "loom", client.ArtifactPatch{Title: &title, Tags: client.Strings([]string{"weaving"})})
	ok("patch artifact", err)
	cleared, err := s.PatchArtifact(ctx, "loom", client.ArtifactPatch{Tags: client.Strings([]string{})})
	ok("clear tags", err)
	if len(cleared.Tags) != 0 || cleared.Title != title {
		t.Fatalf("clearing tags: %+v", cleared)
	}
	ok("save draft", s.SaveDraft(ctx, "loom", client.Page{Body: "# Loom", Gallery: []string{}}))
	ok("publish draft", s.PublishDraft(ctx, "loom"))
	ok("save draft again", s.SaveDraft(ctx, "loom", client.Page{Body: "draft", Gallery: []string{}}))
	ok("discard draft", s.DiscardDraft(ctx, "loom"))
	upload := client.ReleaseUpload{ArtifactID: "loom", Version: "1.0.0", HTML: []byte(testHTML), Manifest: []byte(`{"db":{}}`)}
	_, err = s.CheckRelease(ctx, upload)
	ok("check release", err)
	rel, err := s.CreateRelease(ctx, upload)
	ok("create release", err)
	att, err := s.AddAttachment(ctx, rel.ID, "notes.txt", "Notes", strings.NewReader("release notes"))
	ok("attach", err)
	label := "Release notes"
	_, err = s.PatchAttachment(ctx, att.ID, &label, nil)
	ok("patch attachment", err)
	var buf bytes.Buffer
	n, err := s.DownloadAttachment(ctx, att.ID, &buf)
	ok("download attachment", err)
	if n != int64(len("release notes")) || buf.String() != "release notes" {
		t.Fatalf("download %d %q", n, buf.String())
	}
	_, err = s.PreviewRelease(ctx, rel.ID)
	ok("preview", err)
	pub, err := s.PublishRelease(ctx, rel.ID)
	ok("publish release", err)
	if pub.State != "published" {
		t.Fatalf("published %+v", pub)
	}
	_, err = s.Releases(ctx, "loom")
	ok("releases", err)
	_, err = s.Artifact(ctx, "loom")
	ok("artifact", err)
	_, err = s.Artifacts(ctx)
	ok("artifacts", err)
	draft, err := s.CreateRelease(ctx, client.ReleaseUpload{ArtifactID: "loom", Version: "1.1.0", HTML: []byte(testHTML)})
	ok("second draft", err)
	ok("delete draft", s.DeleteRelease(ctx, draft.ID))
	_, err = s.CreateRelease(ctx, upload)
	code("duplicate version", err, "version_exists")
	m, err := s.UploadMedia(ctx, "board.png", "the board", bytes.NewReader(tinyPNG(t)))
	ok("upload media", err)
	alt := "the whole board"
	ok("patch media", s.PatchMedia(ctx, m.ID, &alt, nil))
	_, err = s.Media(ctx)
	ok("media", err)
	ok("delete media", s.DeleteMedia(ctx, m.ID))
	cr, err := s.CreateCreator(ctx, client.CreatorInput{Slug: client.String("ada"), Name: client.String("Ada"), UserID: client.String(ada.user.ID)})
	ok("create creator", err)
	_, err = s.PatchCreator(ctx, cr.ID, client.CreatorInput{Bio: client.String("Builds looms.")})
	ok("patch creator", err)
	_, err = s.Creators(ctx)
	ok("creators", err)
	post, err := s.CreatePost(ctx, client.PostInput{Slug: client.String("hello"), Title: client.String("Hello"), Body: client.String("Hi"), Summary: client.String("A summary")})
	ok("create post", err)
	patched, err := s.PatchPost(ctx, post.ID, client.PostInput{Title: client.String("Hello again"), Summary: client.String("")})
	ok("patch post", err)
	if patched.Summary != "" || patched.Body != "Hi" {
		t.Fatalf("explicit empty summary not applied, or omitted body changed: %+v", patched)
	}
	ok("publish post", s.SetPostPublished(ctx, post.ID, true))
	_, err = s.Post(ctx, post.ID)
	ok("post", err)
	_, err = s.Posts(ctx)
	ok("posts", err)
	ok("unpublish post", s.SetPostPublished(ctx, post.ID, false))
	ok("delete post", s.DeletePost(ctx, post.ID))
	html, err := s.Markdown(ctx, "*hi*")
	ok("markdown", err)
	if !strings.Contains(html, "<em>hi</em>") {
		t.Fatalf("markdown %q", html)
	}
	_, err = s.Plans(ctx)
	ok("plans", err)

	// Account, as the subscriber.
	acc := sc.Account
	_, err = acc.Get(ctx)
	ok("account", err)
	inst, err := acc.CreateInstance(ctx, rel.ID, "Mine")
	ok("create instance", err)
	ok("rename instance", acc.RenameInstance(ctx, inst.ID, "Ours"))
	_, err = acc.AddMember(ctx, inst.ID, "vera@example.com", "viewer")
	ok("add member", err)
	_, err = acc.Instance(ctx, inst.ID)
	ok("instance", err)
	ok("remove member", acc.RemoveMember(ctx, inst.ID, vera.user.ID))
	_, err = acc.Instances(ctx)
	ok("instances", err)
	_, err = acc.Checkout(ctx, "subscriber", "month")
	code("checkout without billing", err, "billing_unavailable")
	_, err = sc.Studio.Artifacts(ctx)
	code("account token on studio", err, "token_scope")

	// Admin.
	a := c.Admin
	_, err = a.Overview(ctx)
	ok("overview", err)
	_, err = a.Users(ctx, client.UserQuery{Q: "sam"})
	ok("users", err)
	_, err = a.User(ctx, sam.user.ID)
	ok("user", err)
	off := false
	ok("patch user", a.PatchUser(ctx, sam.user.ID, client.UserPatch{Disabled: &off}))
	g, err := a.GrantAccess(ctx, sam.user.ID, client.GrantRequest{Days: 30, Reason: "beta"})
	ok("grant", err)
	_, err = a.RevokeGrant(ctx, g.ID)
	ok("revoke grant", err)
	ok("revoke all grants", a.RevokeAllGrants(ctx, vera.user.ID))
	toks, err := a.UserTokens(ctx, sam.user.ID)
	ok("user tokens", err)
	if len(toks) != 1 || toks[0].Scopes[0] != "account" {
		t.Fatalf("tokens %+v", toks)
	}
	_, err = a.Subscriptions(ctx, "", 10)
	ok("subscriptions", err)
	code("cancel without billing", a.SetCancelAtPeriodEnd(ctx, "sub_x", true), "billing_unavailable")
	_, err = a.Instances(ctx, "")
	ok("admin instances", err)
	_, err = a.Instance(ctx, inst.ID)
	ok("admin instance", err)
	ok("suspend", a.SetInstanceSuspended(ctx, inst.ID, true))
	ok("resume", a.SetInstanceSuspended(ctx, inst.ID, false))
	_, err = a.Settings(ctx)
	ok("settings", err)
	_, err = a.EditPlan(ctx, "subscriber", func(p *settings.Plan) error { p.DailyTokens = 777; return nil })
	ok("edit plan", err)
	cur, _ := a.Settings(ctx)
	if p, _ := cur.Active.Document.Plan("subscriber"); p.DailyTokens != 777 {
		t.Fatalf("plan edit lost: %+v", p)
	}
	_, err = a.SaveSettings(ctx, cur.Active.Document, cur.Active.Revision-1)
	code("stale settings", err, "settings_conflict")
	_, err = a.Billing(ctx, "")
	ok("billing", err)
	code("reconcile without billing", a.Reconcile(ctx, sam.user.ID), "billing_unavailable")
	_, err = a.Usage(ctx, "", 10)
	ok("usage", err)
	ok("adjust", a.SetTokenCeiling(ctx, sam.user.ID, new(int), "goodwill"))
	_, err = a.Audit(ctx, client.AuditQuery{Actor: ada.user.ID, Limit: 5})
	ok("audit", err)
	ok("admin delete instance", a.DeleteInstance(ctx, inst.ID))
	_, err = s.WithdrawRelease(ctx, rel.ID, false)
	ok("withdraw", err)
	ok("delete creator", s.DeleteCreator(ctx, cr.ID))
	ok("revoke token", a.RevokeToken(ctx, toks[0].ID))
	_, err = sc.Me(ctx)
	code("revoked token", err, "token_invalid")
	r, err := a.RevokeSessions(ctx, vera.user.ID, true)
	ok("revoke sessions", err)
	if r.Sessions == 0 {
		t.Fatalf("revoke sessions %+v", r)
	}
}
