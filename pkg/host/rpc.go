package host

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-go-golems/francois/pkg/store"
	"github.com/pkg/errors"
)

type rpcRequest struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args"`
}

func (h *Host) rpc(w http.ResponseWriter, r *http.Request, artifact string) {
	v := h.view(w, r, artifact)
	if v == nil {
		return
	}
	var req rpcRequest
	if err := readJSON(w, r, &req); err != nil {
		fail(w, 400, "invalid_argument", err.Error())
		return
	}
	a, err := h.Store.Get(r.Context(), artifact)
	if err != nil {
		fail(w, 404, "unavailable", "Artifact missing")
		return
	}
	capability := strings.Split(req.Method, ".")[0]
	if _, ok := a.Manifest[capability]; !ok {
		fail(w, 403, "not_declared", "Capability not declared")
		return
	}
	result, err := h.operation(r, v, req)
	if err != nil {
		code := "invalid_argument"
		if req.Method == "artifact.publish" {
			code = "invalid_content"
			if err.Error() == "conflict" {
				code = "conflict"
			}
		}
		fail(w, 400, code, err.Error())
		return
	}
	respond(w, result)
}
func (h *Host) operation(r *http.Request, v *View, req rpcRequest) (any, error) {
	var args struct {
		Path         string         `json:"path"`
		Body         map[string]any `json:"body"`
		Query        store.Query    `json:"query"`
		Subscription string         `json:"subscription"`
		Kind         string         `json:"kind"`
		Holder       string         `json:"holder"`
		TTL          int64          `json:"ttlMs"`
		Data         any            `json:"data"`
		HTML         string         `json:"html"`
		Topic        string         `json:"topic"`
		Room         string         `json:"room"`
		Patch        map[string]any `json:"patch"`
		ID           string         `json:"id"`
		Anchor       any            `json:"anchor"`
		Text         string         `json:"text"`
		ThreadID     string         `json:"threadId"`
		Resolved     bool           `json:"resolved"`
	}
	if err := json.Unmarshal(req.Args, &args); err != nil {
		return nil, err
	}
	ctx := r.Context()
	switch req.Method {
	case "db.get":
		if err := store.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		if !h.readable(args.Path) {
			return store.Document{Path: args.Path, Exists: false}, nil
		}
		return h.Store.Document(ctx, v.Artifact, args.Path)
	case "db.query":
		docs, err := h.Store.Query(ctx, v.Artifact, args.Query)
		if err != nil {
			return nil, err
		}
		visible := []store.Document{}
		for _, d := range docs {
			if h.readable(d.Path) {
				visible = append(visible, d)
			}
		}
		return visible, nil
	case "db.set", "db.update", "db.delete":
		if err := store.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		if !h.readable(args.Path) {
			return nil, errors.New("Private document belongs to another person")
		}
		if req.Method != "db.delete" && args.Body == nil {
			return nil, errors.New("Document body must be an object")
		}
		if err := validateDepth(args.Body, 0); err != nil {
			return nil, err
		}
		if err := h.Store.Write(ctx, v.Artifact, args.Path, strings.TrimPrefix(req.Method, "db."), args.Body); err != nil {
			return nil, err
		}
		h.broadcast(v.Artifact, map[string]any{"type": "db_changed"})
		return map[string]bool{"ok": true}, nil
	case "db.acquire":
		if err := store.ValidatePath(args.Path, true); err != nil {
			return nil, err
		}
		if !h.readable(args.Path) {
			return nil, errors.New("Private document belongs to another person")
		}
		if args.TTL == 0 {
			args.TTL = 30000
		}
		acquired, err := h.Store.Acquire(ctx, v.Artifact, args.Path, args.Holder, args.TTL, args.Data)
		return map[string]bool{"acquired": acquired}, err
	case "artifact.publish":
		if len(args.HTML) > 16<<20 || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(args.HTML)), "<!doctype html>") {
			return nil, errors.New("publish requires a complete HTML page")
		}
		version, err := h.Store.Publish(ctx, v.Artifact, v.Version, []byte(args.HTML))
		if err != nil {
			return nil, err
		}
		h.broadcast(v.Artifact, map[string]any{"type": "reload", "version": version})
		return map[string]int64{"version": version}, nil
	case "room.presence":
		v.mu.Lock()
		copyPatch := map[string]any{}
		for k, val := range v.presence {
			copyPatch[k] = val
		}
		for k, val := range args.Patch {
			if val == nil {
				delete(copyPatch, k)
			} else {
				copyPatch[k] = val
			}
		}
		b, err := json.Marshal(copyPatch)
		if err != nil || len(b) > 4096 {
			v.mu.Unlock()
			return nil, errors.New("Presence exceeds 4 KiB")
		}
		v.presence = copyPatch
		v.mu.Unlock()
		h.roomBroadcast(v.Artifact)
		return map[string]bool{"ok": true}, nil
	case "room.join", "room.leave":
		if !roomName.MatchString(args.Room) {
			return nil, errors.New("Invalid room name")
		}
		v.mu.Lock()
		if req.Method == "room.join" {
			if len(v.rooms) >= 17 && !v.rooms[args.Room] {
				v.mu.Unlock()
				return nil, errors.New("Room limit reached")
			}
			v.rooms[args.Room] = true
		} else {
			delete(v.rooms, args.Room)
		}
		v.mu.Unlock()
		h.roomBroadcast(v.Artifact)
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
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, peer := range h.views {
			if peer.Artifact != v.Artifact {
				continue
			}
			peer.mu.Lock()
			joined := peer.rooms[args.Room]
			peer.mu.Unlock()
			if joined {
				peer.emit(map[string]any{"type": "room_event", "room": args.Room, "topic": args.Topic, "data": args.Data, "peer": v.ID, "by": h.Owner, "isMe": true, "sameTab": peer.ID == v.ID, "kind": "viewer", "guest": false})
			}
		}
		return map[string]bool{"ok": true}, nil
	case "comments.create", "comments.reply", "comments.resolve", "comments.delete":
		return h.commentOperation(r, v, req.Method, args.ThreadID, args.Text, args.Anchor, args.Resolved)
	case "assets.list":
		return h.listAssets(ctx, v.Artifact)
	case "assets.delete":
		result, err := h.Store.DB.ExecContext(ctx, `DELETE FROM assets WHERE id=? AND artifact=?`, args.ID, v.Artifact)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		return map[string]bool{"deleted": n > 0}, err
	case "files.list":
		return h.listFiles(args.Path, req.Args)
	}
	return nil, errors.New("Unsupported operation")
}
func (h *Host) readable(path string) bool {
	if !strings.HasPrefix(path, "data/users/") {
		return true
	}
	parts := strings.Split(path, "/")
	return len(parts) >= 3 && parts[2] == h.Owner
}
func validateDepth(v any, depth int) error {
	if depth > 32 {
		return errors.New("Document exceeds 32 levels")
	}
	switch x := v.(type) {
	case map[string]any:
		for _, v := range x {
			if err := validateDepth(v, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, v := range x {
			if err := validateDepth(v, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
