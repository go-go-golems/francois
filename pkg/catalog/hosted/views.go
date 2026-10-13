package hosted

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
)

// View is one live browser page of one user in one instance.
type View struct {
	ID        string
	P         Principal
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	connected bool
	events    chan any
	calls     map[string]*call
	presence  map[string]map[string]any
	rooms     map[string]bool
}

type call struct {
	cancel context.CancelFunc
	tools  map[string]chan piworker.ToolResult
	names  map[string]bool
}

func (v *View) emit(event any) bool {
	select {
	case v.events <- event:
		return true
	case <-v.ctx.Done():
		return false
	default:
		v.cancel() // a stalled consumer loses its view rather than blocking peers
		return false
	}
}

// view re-authenticates the caller (current role, membership, entitlement)
// and returns the caller's own view. A renewed grant of the same user may
// keep using an existing view.
func (rt *Runtime) view(w http.ResponseWriter, r *http.Request, inst store.Instance) (*View, Principal) {
	p, err := rt.authenticate(r.Context(), bearer(r), inst)
	if err != nil {
		failErr(w, err)
		return nil, p
	}
	rt.mu.Lock()
	v := rt.views[r.PathValue("view")]
	rt.mu.Unlock()
	if v == nil || v.P.Instance.ID != inst.ID || v.P.User.ID != p.User.ID || r.Header.Get("X-Francois-View") != v.ID {
		httpx.Fail(w, 404, "unavailable", "View not found")
		return nil, p
	}
	return v, p
}

func (rt *Runtime) createView(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	p, err := rt.authenticate(r.Context(), bearer(r), inst)
	if err != nil {
		failErr(w, err)
		return
	}
	ctx, cancel := context.WithCancel(rt.ctx)
	v := &View{ID: store.NewID("v_"), P: p, ctx: ctx, cancel: cancel, events: make(chan any, 128), calls: map[string]*call{}, presence: map[string]map[string]any{}, rooms: map[string]bool{"": true}}
	rt.mu.Lock()
	rt.views[v.ID] = v
	rt.mu.Unlock()
	rt.group.Go(func() error {
		timer := time.NewTimer(15 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			v.mu.Lock()
			up := v.connected
			v.mu.Unlock()
			if !up {
				rt.closeView(v)
			}
		case <-v.ctx.Done():
			rt.closeView(v)
		}
		return nil
	})
	httpx.OK(w, map[string]any{"viewId": v.ID, "version": 1})
}

func (rt *Runtime) closeView(v *View) {
	v.cancel()
	v.mu.Lock()
	v.connected = false
	for _, c := range v.calls {
		c.cancel()
	}
	v.mu.Unlock()
	rt.mu.Lock()
	_, present := rt.views[v.ID]
	delete(rt.views, v.ID)
	rt.mu.Unlock()
	if present {
		rt.roomBroadcast(v.P)
	}
}

// events streams view events and re-verifies authorization periodically, so
// revoked membership, suspended accounts or lost entitlement close the view.
func (rt *Runtime) events(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	v, _ := rt.view(w, r, inst)
	if v == nil {
		return
	}
	flush, ok := w.(http.Flusher)
	if !ok {
		httpx.Fail(w, 500, "unavailable", "Streaming unavailable")
		return
	}
	v.mu.Lock()
	if v.connected {
		v.mu.Unlock()
		httpx.Fail(w, 409, "unavailable", "View already connected")
		return
	}
	v.connected = true
	v.mu.Unlock()
	defer rt.closeView(v)
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = io.WriteString(w, "data: {\"type\":\"ready\"}\n\n")
	flush.Flush()
	rt.roomBroadcast(v.P)
	keep := time.NewTicker(15 * time.Second)
	defer keep.Stop()
	recheck := time.NewTicker(30 * time.Second)
	defer recheck.Stop()
	for {
		select {
		case e := <-v.events:
			b, err := json.Marshal(e)
			if err != nil {
				return
			}
			if _, err = fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return
			}
			flush.Flush()
		case <-keep.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flush.Flush()
		case <-recheck.C:
			if _, err := rt.principal(r.Context(), v.P.User.ID, inst.ID, nil, v.P.Session); err != nil {
				b, _ := json.Marshal(map[string]any{"type": "revoked"})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
				flush.Flush()
				return
			}
		case <-r.Context().Done():
			return
		case <-v.ctx.Done():
			b, _ := json.Marshal(map[string]any{"type": "revoked"})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flush.Flush()
			return
		}
	}
}

// broadcast sends an event to every live view of one instance only.
func (rt *Runtime) broadcast(instance string, event any) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, v := range rt.views {
		if v.P.Instance.ID == instance {
			v.emit(event)
		}
	}
}

var roomName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,47}$`)
var topicName = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,47}$`)

func (rt *Runtime) person(p Principal) any {
	if rt.declared(p, "user") {
		return p.User.ID
	}
	return nil
}

// roomBroadcast publishes per-instance peer lists. isMe compares verified
// users while sameTab compares views.
func (rt *Runtime) roomBroadcast(p Principal) {
	if !rt.declared(p, "room") {
		return
	}
	instance := p.Instance.ID
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, target := range rt.views {
		if target.P.Instance.ID != instance {
			continue
		}
		target.mu.Lock()
		rooms := []string{}
		for room := range target.rooms {
			rooms = append(rooms, room)
		}
		target.mu.Unlock()
		for _, room := range rooms {
			peers := []map[string]any{}
			for _, v := range rt.views {
				if v.P.Instance.ID != instance {
					continue
				}
				v.mu.Lock()
				if v.connected && v.rooms[room] {
					data := map[string]any{}
					for k, val := range v.presence[room] {
						data[k] = val
					}
					peers = append(peers, map[string]any{"peer": v.ID, "by": rt.person(v.P), "isMe": v.P.User.ID == target.P.User.ID, "sameTab": v.ID == target.ID, "kind": "viewer", "guest": false, "presence": data})
				}
				v.mu.Unlock()
			}
			target.emit(map[string]any{"type": "room_peers", "room": room, "peers": peers})
		}
	}
}
