// Package hosted serves application instances on their own origins. Every
// operation runs with an explicit, server-verified instance context
// (user, instance, release, role) instead of a global local owner.
package hosted

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/releases"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Authorizer checks current entitlement for a caller in an instance; it is
// supplied by the portal so policy lives in one place.
type Authorizer func(ctx context.Context, user store.User, inst store.Instance) error

type Config struct {
	// PortalOrigin is the only parent allowed to hand out launch codes.
	PortalOrigin string
	// AppOrigin maps an instance hostname to its exact origin.
	AppOrigin func(hostname string) string
	GrantTTL  time.Duration
}

type Runtime struct {
	Store     *store.Store
	Publisher *releases.Publisher
	Backend   piworker.Backend
	Authorize Authorizer
	Config    Config

	ctx    context.Context
	group  *errgroup.Group
	key    []byte
	mu     sync.Mutex
	views  map[string]*View
	active map[string]int // per-user running model calls
	global int
}

func New(ctx context.Context, s *store.Store, p *releases.Publisher, b piworker.Backend, auth Authorizer, cfg Config) *Runtime {
	if cfg.GrantTTL == 0 {
		cfg.GrantTTL = 30 * time.Minute
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	g, ctx := errgroup.WithContext(ctx)
	return &Runtime{Store: s, Publisher: p, Backend: b, Authorize: auth, Config: cfg, ctx: ctx, group: g, key: key, views: map[string]*View{}, active: map[string]int{}}
}

// Wait blocks until background calls finish after the parent context ends.
func (rt *Runtime) Wait() error { return rt.group.Wait() }

// Principal is the verified context of one hosted request.
type Principal struct {
	User      store.User
	Instance  store.Instance
	Role      string
	Release   store.Release
	GrantHash []byte
	Session   []byte
}

func (p Principal) CanEdit() bool { return p.Role == "owner" || p.Role == "editor" }

// denied carries an HTTP status and stable code through authentication.
type denied struct {
	status int
	code   string
	msg    string
}

func (d *denied) Error() string { return d.msg }

// Status exposes the HTTP mapping for portal handlers.
func (d *denied) Status() (int, string, string) { return d.status, d.code, d.msg }

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// authenticate resolves a scoped grant into a principal and rechecks
// membership, instance state and entitlement on every request.
func (rt *Runtime) authenticate(ctx context.Context, token string, inst store.Instance) (Principal, error) {
	if token == "" {
		return Principal{}, &denied{401, "grant_required", "A runtime grant is required"}
	}
	g, err := rt.Store.ResolveGrant(ctx, token, inst.ID)
	if err != nil {
		return Principal{}, &denied{401, "grant_invalid", "The runtime grant is invalid or expired"}
	}
	return rt.principal(ctx, g.UserID, inst.ID, store.Hash(token), g.SessionHash)
}

func (rt *Runtime) principal(ctx context.Context, userID, instanceID string, grant, session []byte) (Principal, error) {
	u, err := rt.Store.User(ctx, userID)
	if err != nil {
		return Principal{}, err
	}
	inst, err := rt.Store.MemberInstance(ctx, userID, instanceID)
	if errors.Is(err, store.ErrNotFound) {
		return Principal{}, &denied{403, "not_member", "You are not a member of this instance"}
	}
	if err != nil {
		return Principal{}, err
	}
	if inst.State != "active" {
		return Principal{}, &denied{403, "instance_suspended", "This instance is suspended"}
	}
	if err = rt.Authorize(ctx, u, inst); err != nil {
		return Principal{}, err
	}
	rel, err := rt.Store.Release(ctx, inst.ReleaseID)
	if err != nil {
		return Principal{}, err
	}
	return Principal{User: u, Instance: inst, Role: inst.Role, Release: rel, GrantHash: grant, Session: session}, nil
}

// Denied builds an authorization error for the Authorizer implementation.
func Denied(status int, code, msg string) error { return &denied{status, code, msg} }

func failErr(w http.ResponseWriter, err error) {
	var d *denied
	if errors.As(err, &d) {
		httpx.Fail(w, d.status, d.code, d.msg)
		return
	}
	log.Error().Err(err).Msg("hosted request failed")
	httpx.Fail(w, 500, "unavailable", "Request failed")
}

// available intersects the manifest with capabilities the hosted runtime
// implements; files and catalog republishing are not offered.
func available(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for name, decl := range m {
		switch name {
		case "sample", "user", "db", "room", "comments", "assets", "downloads":
			out[name] = decl
		}
	}
	return out
}

func (rt *Runtime) declared(p Principal, capability string) bool {
	_, ok := available(p.Release.Manifest)[capability]
	return ok
}

// Revocation closes live views (and their calls) matching a predicate.
func (rt *Runtime) revoke(match func(*View) bool) int {
	rt.mu.Lock()
	targets := []*View{}
	for _, v := range rt.views {
		if match(v) {
			targets = append(targets, v)
		}
	}
	rt.mu.Unlock()
	for _, v := range targets {
		v.cancel()
	}
	return len(targets)
}

func (rt *Runtime) RevokeUser(user string) int {
	return rt.revoke(func(v *View) bool { return v.P.User.ID == user })
}
func (rt *Runtime) RevokeInstance(inst string) int {
	return rt.revoke(func(v *View) bool { return v.P.Instance.ID == inst })
}
func (rt *Runtime) RevokeMember(inst, user string) int {
	return rt.revoke(func(v *View) bool { return v.P.Instance.ID == inst && v.P.User.ID == user })
}
func (rt *Runtime) RevokeSession(session []byte) int {
	return rt.revoke(func(v *View) bool { return hmac.Equal(v.P.Session, session) })
}

// ActiveViews counts live views per instance for administrators.
func (rt *Runtime) ActiveViews() map[string]int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	out := map[string]int{}
	for _, v := range rt.views {
		out[v.P.Instance.ID]++
	}
	return out
}

// signBlob creates a short-lived capability URL for one asset, because
// <img src> cannot carry the runtime grant header.
func (rt *Runtime) signBlob(instance, id string) string {
	exp := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	m := hmac.New(sha256.New, rt.key)
	m.Write([]byte(instance + "|" + id + "|" + exp))
	return "/_blob/" + id + "?e=" + exp + "&s=" + hex.EncodeToString(m.Sum(nil))
}

func (rt *Runtime) verifyBlob(instance, id, exp, sig string) bool {
	n, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > n {
		return false
	}
	m := hmac.New(sha256.New, rt.key)
	m.Write([]byte(instance + "|" + id + "|" + exp))
	want := hex.EncodeToString(m.Sum(nil))
	return hmac.Equal([]byte(want), []byte(sig))
}
