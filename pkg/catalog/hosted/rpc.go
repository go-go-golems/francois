package hosted

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	local "github.com/go-go-golems/francois/pkg/store"
	"github.com/pkg/errors"
)

type rpcRequest struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args"`
}

type rpcArgs struct {
	Path     string         `json:"path"`
	Body     map[string]any `json:"body"`
	Query    local.Query    `json:"query"`
	Holder   string         `json:"holder"`
	TTL      int64          `json:"ttlMs"`
	Data     any            `json:"data"`
	Topic    string         `json:"topic"`
	Room     string         `json:"room"`
	Patch    map[string]any `json:"patch"`
	ID       string         `json:"id"`
	IDs      []string       `json:"ids"`
	Q        string         `json:"q"`
	Anchor   any            `json:"anchor"`
	Text     string         `json:"text"`
	ThreadID string         `json:"threadId"`
	Resolved bool           `json:"resolved"`
}

// errCode lets operations choose a stable error code.
type errCode struct {
	status int
	code   string
	msg    string
}

func (e *errCode) Error() string { return e.msg }

func readOnly() error { return &errCode{403, "read_only", "Your role in this instance is read-only"} }

func (rt *Runtime) rpc(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	v, p := rt.view(w, r, inst)
	if v == nil {
		return
	}
	var req rpcRequest
	if err := httpx.Read(w, r, &req, 16<<20); err != nil {
		httpx.Fail(w, 400, "invalid_argument", err.Error())
		return
	}
	capability := strings.Split(req.Method, ".")[0]
	if !rt.declared(p, capability) {
		httpx.Fail(w, 403, "not_declared", "Capability not declared or not available in hosted instances")
		return
	}
	var args rpcArgs
	if err := json.Unmarshal(req.Args, &args); err != nil {
		httpx.Fail(w, 400, "invalid_argument", "Invalid arguments")
		return
	}
	result, err := rt.operation(r.Context(), v, p, req.Method, args)
	if err != nil {
		var ec *errCode
		if errors.As(err, &ec) {
			httpx.Fail(w, ec.status, ec.code, ec.msg)
			return
		}
		httpx.Fail(w, 400, "invalid_argument", err.Error())
		return
	}
	httpx.OK(w, result)
}

// writable: editors/owners write shared data; every member may write only
// their own private data/users/<uid> namespace.
func writable(p Principal, path string) error {
	if owner := store.PrivateOwner(path); owner != "" {
		if owner != p.User.ID {
			return &errCode{403, "permission_denied", "Private document belongs to another person"}
		}
		return nil
	}
	if !p.CanEdit() {
		return readOnly()
	}
	return nil
}

func (rt *Runtime) operation(ctx context.Context, v *View, p Principal, method string, args rpcArgs) (any, error) {
	instance := p.Instance.ID
	switch method {
	case "db.get":
		if err := local.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		return rt.Store.Document(ctx, instance, p.User.ID, args.Path)
	case "db.query":
		return rt.Store.QueryDocuments(ctx, instance, p.User.ID, args.Query)
	case "db.set", "db.update", "db.delete":
		if err := local.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		if err := writable(p, args.Path); err != nil {
			return nil, err
		}
		if method != "db.delete" && args.Body == nil {
			return nil, errors.New("Document body must be an object")
		}
		if err := local.ValidateDepth(args.Body, 0); err != nil {
			return nil, err
		}
		rev, err := rt.Store.ActiveSettings(ctx)
		if err != nil {
			return nil, err
		}
		if err = rt.Store.WriteDocument(ctx, instance, p.User.ID, args.Path, strings.TrimPrefix(method, "db."), args.Body, rev.Document.Limits.DocumentsPerInstance); err != nil {
			return nil, err
		}
		// Invalidate only this instance's subscriptions.
		rt.broadcast(instance, map[string]any{"type": "db_changed"})
		return map[string]bool{"ok": true}, nil
	case "db.acquire":
		if err := local.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		if err := writable(p, args.Path); err != nil {
			return nil, err
		}
		if args.TTL == 0 {
			args.TTL = 30000
		}
		ok, err := rt.Store.AcquireLease(ctx, instance, p.User.ID, args.Path, args.Holder, args.TTL, args.Data)
		return map[string]bool{"acquired": ok}, err
	case "user.profiles":
		return rt.profiles(ctx, p, args.IDs)
	case "user.search":
		return rt.search(ctx, p, args.Q)
	case "room.presence":
		v.mu.Lock()
		if !v.rooms[args.Room] {
			v.mu.Unlock()
			return nil, errors.New("Room not joined")
		}
		next := map[string]any{}
		for k, val := range v.presence[args.Room] {
			next[k] = val
		}
		for k, val := range args.Patch {
			if val == nil {
				delete(next, k)
			} else {
				next[k] = val
			}
		}
		b, err := json.Marshal(next)
		if err != nil || len(b) > 4096 {
			v.mu.Unlock()
			return nil, errors.New("Presence exceeds 4 KiB")
		}
		v.presence[args.Room] = next
		v.mu.Unlock()
		rt.roomBroadcast(p)
		return map[string]bool{"ok": true}, nil
	case "room.join", "room.leave":
		if !roomName.MatchString(args.Room) {
			return nil, errors.New("Invalid room name")
		}
		v.mu.Lock()
		if method == "room.join" {
			if len(v.rooms) >= 17 && !v.rooms[args.Room] {
				v.mu.Unlock()
				return nil, errors.New("Room limit reached")
			}
			v.rooms[args.Room] = true
		} else {
			delete(v.rooms, args.Room)
			delete(v.presence, args.Room)
		}
		v.mu.Unlock()
		rt.roomBroadcast(p)
		return map[string]bool{"ok": true}, nil
	case "room.emit":
		if !topicName.MatchString(args.Topic) {
			return nil, errors.New("Invalid topic")
		}
		data, err := json.Marshal(args.Data)
		if err != nil || len(data) > 4096 {
			return nil, errors.New("Event exceeds 4 KiB")
		}
		v.mu.Lock()
		joined := v.rooms[args.Room]
		v.mu.Unlock()
		if !joined {
			return nil, errors.New("Room not joined")
		}
		by := rt.person(p)
		rt.mu.Lock()
		defer rt.mu.Unlock()
		for _, peer := range rt.views {
			if peer.P.Instance.ID != instance {
				continue
			}
			peer.mu.Lock()
			in := peer.rooms[args.Room]
			peer.mu.Unlock()
			if in {
				peer.emit(map[string]any{"type": "room_event", "room": args.Room, "topic": args.Topic, "data": args.Data, "peer": v.ID, "by": by,
					"isMe": peer.P.User.ID == p.User.ID, "sameTab": peer.ID == v.ID, "kind": "viewer", "guest": false})
			}
		}
		return map[string]bool{"ok": true}, nil
	case "comments.create":
		if !p.CanEdit() {
			return nil, readOnly()
		}
		if args.Text == "" || len(args.Text) > 4096 || args.Anchor == nil {
			return nil, errors.New("Comment requires text (at most 4 KiB) and an anchor")
		}
		raw, err := json.Marshal(args.Anchor)
		if err != nil || len(raw) > 4096 {
			return nil, errors.New("Invalid comment anchor")
		}
		thread, comment, err := rt.Store.CreateThread(ctx, instance, p.User.ID, raw, args.Text)
		return map[string]string{"threadId": thread, "commentId": comment}, err
	case "comments.reply":
		if !p.CanEdit() {
			return nil, readOnly()
		}
		if args.Text == "" || len(args.Text) > 4096 {
			return nil, errors.New("Reply cannot be empty or exceed 4 KiB")
		}
		return map[string]bool{"ok": true}, rt.Store.Reply(ctx, instance, args.ThreadID, p.User.ID, args.Text)
	case "comments.resolve":
		if !p.CanEdit() {
			return nil, readOnly()
		}
		return map[string]bool{"ok": true}, rt.Store.ResolveThread(ctx, instance, args.ThreadID, args.Resolved)
	case "comments.delete":
		return map[string]bool{"ok": true}, rt.Store.DeleteThread(ctx, instance, args.ThreadID, p.User.ID, p.Role == "owner")
	case "assets.list":
		list, err := rt.Store.Assets(ctx, instance)
		if err != nil {
			return nil, err
		}
		rev, err := rt.Store.ActiveSettings(ctx)
		if err != nil {
			return nil, err
		}
		out := []map[string]any{}
		var total int64
		for _, a := range list {
			total += a.Size
			out = append(out, map[string]any{"id": a.ID, "url": rt.signBlob(instance, a.ID), "contentType": a.MediaType, "sizeBytes": a.Size, "createdAt": a.CreatedAt})
		}
		return map[string]any{"assets": out, "usage": map[string]any{"files": len(out), "bytes": total, "maxFiles": 1000, "maxBytes": rev.Document.Limits.AssetBytesPerInstance}}, nil
	case "assets.delete":
		if !p.CanEdit() {
			return nil, readOnly()
		}
		ok, err := rt.Store.DeleteAsset(ctx, instance, args.ID, p.User.ID, p.Role == "owner")
		return map[string]bool{"deleted": ok}, err
	}
	return nil, errors.New("Unsupported operation")
}

// profiles returns public profiles only for co-members of this instance.
func (rt *Runtime) profiles(ctx context.Context, p Principal, ids []string) (any, error) {
	if len(ids) > 100 {
		return nil, errors.New("At most 100 profiles")
	}
	members, err := rt.Store.Members(ctx, p.Instance.ID)
	if err != nil {
		return nil, err
	}
	byID := map[string]store.Member{}
	for _, m := range members {
		byID[m.UserID] = m
	}
	out := map[string]any{}
	for _, id := range ids {
		m, ok := byID[id]
		if !ok {
			out[id] = map[string]any{"id": id, "name": "", "avatarUrl": "", "color": "#737373", "isMe": false, "guest": false, "email": nil}
			continue
		}
		out[id] = profile(store.User{ID: m.UserID, DisplayName: m.Name, Email: m.Email}, p.User.ID)
	}
	return out, nil
}

func (rt *Runtime) search(ctx context.Context, p Principal, q string) (any, error) {
	if !rt.declared(p, "user") {
		return []any{}, nil
	}
	members, err := rt.Store.Members(ctx, p.Instance.ID)
	if err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := []any{}
	for _, m := range members {
		if q == "" || strings.Contains(strings.ToLower(m.Name), q) {
			out = append(out, profile(store.User{ID: m.UserID, DisplayName: m.Name, Email: m.Email}, p.User.ID))
		}
	}
	return out, nil
}
