package catalog

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

func (s *Server) instanceJSON(r *http.Request, i store.Instance) map[string]any {
	return map[string]any{"id": i.ID, "title": i.Title, "kind": i.Kind, "state": i.State, "role": i.Role, "artifactId": i.ArtifactID, "version": i.Version,
		"releaseId": i.ReleaseID, "ownerName": i.OwnerName, "ownerUserId": i.OwnerUserID, "members": i.Members, "origin": s.site(r).AppOrigin(i.Hostname),
		"createdAt": i.CreatedAt, "updatedAt": i.UpdatedAt}
}

func (s *Server) instances(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.MemberInstances(r.Context(), principal(r.Context()).User.ID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Instances unavailable")
		return
	}
	out := []map[string]any{}
	for _, i := range list {
		out = append(out, s.instanceJSON(r, i))
	}
	httpx.OK(w, out)
}

// entitled writes a 403 unless the caller currently has hosted access.
func (s *Server) entitled(w http.ResponseWriter, r *http.Request) bool {
	d, err := s.access(r.Context(), principal(r.Context()).User)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Access check failed")
		return false
	}
	if !d.Allowed {
		httpx.Fail(w, 403, d.Reason, "A current subscription is required")
		return false
	}
	return true
}

func (s *Server) createInstance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ReleaseID string `json:"releaseId"`
		Title     string `json:"title"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if !s.entitled(w, r) {
		return
	}
	rel, err := s.Store.Release(r.Context(), in.ReleaseID)
	if err != nil || rel.State != "published" {
		httpx.Fail(w, 404, "release_unavailable", "Release not available")
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		a, _ := s.Store.Artifact(r.Context(), rel.ArtifactID)
		in.Title = a.Title
	}
	if len(in.Title) > 120 {
		httpx.Fail(w, 400, "invalid_request", "Title is too long")
		return
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Settings unavailable")
		return
	}
	inst, err := s.Store.CreateInstance(r.Context(), principal(r.Context()).User.ID, rel.ID, in.Title, "workspace", rev.Document.Limits.InstancesPerUser)
	if errors.Is(err, store.ErrLimit) {
		httpx.Fail(w, 409, "instance_limit", "You have reached the instance limit")
		return
	}
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not create instance")
		return
	}
	httpx.JSON(w, 201, s.instanceJSON(r, inst))
}

// member loads an instance only when the caller belongs to it.
func (s *Server) member(w http.ResponseWriter, r *http.Request) (store.Instance, bool) {
	inst, err := s.Store.MemberInstance(r.Context(), principal(r.Context()).User.ID, r.PathValue("id"))
	if err != nil {
		httpx.Fail(w, 404, "not_found", "Instance not found")
		return inst, false
	}
	return inst, true
}

func (s *Server) owner(w http.ResponseWriter, r *http.Request) (store.Instance, bool) {
	inst, ok := s.member(w, r)
	if !ok {
		return inst, false
	}
	if inst.Role != "owner" && principal(r.Context()).User.Role != "admin" {
		httpx.Fail(w, 403, "owner_required", "Only the instance owner can do this")
		return inst, false
	}
	return inst, true
}

func (s *Server) instance(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.member(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	members, err := s.Store.Members(ctx, inst.ID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Instance unavailable")
		return
	}
	if inst.Role != "owner" {
		for i := range members {
			members[i].Email = "" // co-members see names, not addresses
		}
	}
	rel, err := s.Store.Release(ctx, inst.ReleaseID)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Instance unavailable")
		return
	}
	upgrades := []map[string]any{}
	if inst.Role == "owner" {
		rels, _ := s.Store.Releases(ctx, rel.ArtifactID, "published")
		for _, c := range rels {
			if c.ID != rel.ID && rel.PublishedAt != nil && c.PublishedAt != nil && c.PublishedAt.After(*rel.PublishedAt) {
				upgrades = append(upgrades, publicRelease(c))
			}
		}
	}
	out := s.instanceJSON(r, inst)
	out["membersList"] = members
	out["release"] = publicRelease(rel)
	out["upgrades"] = upgrades
	if err := s.authorizeInstance(ctx, principal(ctx).User, inst); err != nil {
		out["launchBlocked"] = err.Error()
	}
	if inst.State != "active" {
		out["launchBlocked"] = "This instance is suspended"
	}
	httpx.OK(w, out)
}

func (s *Server) renameInstance(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.owner(w, r)
	if !ok {
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || strings.TrimSpace(in.Title) == "" || len(in.Title) > 120 {
		httpx.Fail(w, 400, "invalid_request", "A title of at most 120 characters is required")
		return
	}
	if _, err := s.Store.Pool.Exec(r.Context(), `UPDATE instances SET title=$2,updated_at=now() WHERE id=$1`, inst.ID, strings.TrimSpace(in.Title)); err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not rename")
		return
	}
	httpx.OK(w, map[string]bool{"ok": true})
}

func (s *Server) deleteInstance(w http.ResponseWriter, r *http.Request) {
	inst, err := s.Store.MemberInstance(r.Context(), principal(r.Context()).User.ID, r.PathValue("id"))
	if err != nil || inst.Role != "owner" {
		httpx.Fail(w, 404, "not_found", "Instance not found")
		return
	}
	s.Runtime.RevokeInstance(inst.ID)
	if err = s.Store.DeleteInstance(r.Context(), inst.ID); err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not delete")
		return
	}
	httpx.OK(w, map[string]bool{"deleted": true})
}

var challengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// launch issues a one-use launch code bound to this portal session, user,
// instance, exact app origin and the bootstrap's S256 challenge.
func (s *Server) launch(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Challenge string `json:"challenge"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || !challengePattern.MatchString(in.Challenge) {
		httpx.Fail(w, 400, "invalid_request", "A S256 challenge is required")
		return
	}
	p := principal(r.Context())
	if inst.State != "active" {
		httpx.Fail(w, 403, "instance_suspended", "This instance is suspended")
		return
	}
	if err := s.authorizeInstance(r.Context(), p.User, inst); err != nil {
		writeDenied(w, err)
		return
	}
	origin := s.site(r).AppOrigin(inst.Hostname)
	code, err := s.Store.CreateLaunchCode(r.Context(), store.LaunchCode{SessionHash: p.Session.Hash, UserID: p.User.ID, InstanceID: inst.ID, Origin: origin, Challenge: in.Challenge}, 60*time.Second)
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Could not launch")
		return
	}
	httpx.OK(w, map[string]any{"code": code, "origin": origin, "expiresIn": 60})
}

func writeDenied(w http.ResponseWriter, err error) {
	type statusErr interface {
		Status() (int, string, string)
	}
	if d, ok := err.(statusErr); ok {
		st, code, msg := d.Status()
		httpx.Fail(w, st, code, msg)
		return
	}
	httpx.Fail(w, 503, "unavailable", "Access check failed")
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.owner(w, r)
	if !ok {
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || (in.Role != "editor" && in.Role != "viewer") {
		httpx.Fail(w, 400, "invalid_request", "Email and an editor/viewer role are required")
		return
	}
	u, err := s.Store.UserByVerifiedEmail(r.Context(), strings.TrimSpace(in.Email))
	if err != nil {
		httpx.Fail(w, 404, "user_not_found", "No registered subscriber with that verified email")
		return
	}
	rev, err := s.Store.ActiveSettings(r.Context())
	if err != nil {
		httpx.Fail(w, 503, "unavailable", "Settings unavailable")
		return
	}
	actor := principal(r.Context()).User.ID
	if err = s.Store.SetMember(r.Context(), inst.ID, u.ID, in.Role, actor, rev.Document.Limits.MembersPerInstance); err != nil {
		switch {
		case errors.Is(err, store.ErrLimit):
			httpx.Fail(w, 409, "member_limit", "Member limit reached")
		case errors.Is(err, store.ErrConflict):
			httpx.Fail(w, 409, "conflict", "The owner's role cannot change")
		default:
			httpx.Fail(w, 503, "unavailable", "Could not share")
		}
		return
	}
	_ = s.Store.Audit(r.Context(), actor, "instance.member.set", inst.ID, map[string]any{"user": u.ID, "role": in.Role})
	// A role change takes effect on the member's next request; reconnect views.
	s.Runtime.RevokeMember(inst.ID, u.ID)
	httpx.OK(w, map[string]any{"userId": u.ID, "name": u.DisplayName, "role": in.Role})
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.owner(w, r)
	if !ok {
		return
	}
	user := r.PathValue("user")
	if err := s.Store.RemoveMember(r.Context(), inst.ID, user); err != nil {
		httpx.Fail(w, 404, "not_found", "Member not found")
		return
	}
	s.Runtime.RevokeMember(inst.ID, user)
	_ = s.Store.Audit(r.Context(), principal(r.Context()).User.ID, "instance.member.remove", inst.ID, map[string]any{"user": user})
	httpx.OK(w, map[string]bool{"removed": true})
}

func (s *Server) upgrade(w http.ResponseWriter, r *http.Request) {
	inst, ok := s.owner(w, r)
	if !ok {
		return
	}
	var in struct {
		ReleaseID string `json:"releaseId"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if err := s.Store.UpgradeInstance(r.Context(), inst.ID, in.ReleaseID); err != nil {
		httpx.Fail(w, 409, "conflict", "Upgrade requires a newer published release of the same application")
		return
	}
	_ = s.Store.Audit(r.Context(), principal(r.Context()).User.ID, "instance.upgrade", inst.ID, map[string]any{"from": inst.ReleaseID, "to": in.ReleaseID})
	s.Runtime.RevokeInstance(inst.ID) // views reload with the new code
	httpx.OK(w, map[string]bool{"ok": true})
}
