package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/go-go-golems/francois/pkg/web"
	"golang.org/x/net/html"
)

func (h *Host) routes(m *http.ServeMux, artifact string) {
	m.HandleFunc("GET /static/{name}", func(w http.ResponseWriter, r *http.Request) {
		var data []byte
		var mime string
		switch r.PathValue("name") {
		case "app.js":
			data = web.App
			mime = "text/javascript"
		case "runtime.js":
			data = web.Runtime
			mime = "text/javascript"
		case "icon.svg":
			data = web.Icon
			mime = "image/svg+xml"
		case "style.css":
			data = web.Style
			mime = "text/css"
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
	})
	m.HandleFunc("GET /api/models", func(w http.ResponseWriter, r *http.Request) { respond(w, h.Backend.Models()) })
	m.HandleFunc("GET /api/artifacts", func(w http.ResponseWriter, r *http.Request) {
		list, err := h.Store.List(r.Context())
		if err != nil {
			fail(w, 500, "unavailable", err.Error())
			return
		}
		out := []map[string]any{}
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, a := range list {
			if artifact != "" && a.ID != artifact {
				continue
			}
			out = append(out, map[string]any{"id": a.ID, "title": a.Title, "version": a.Version, "url": h.artifacts[a.ID], "manifest": a.Manifest, "updatedAt": a.UpdatedAt})
		}
		respond(w, out)
	})
	if artifact == "" {
		m.HandleFunc("POST /api/artifacts", h.importArtifact)
		m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(web.Index)
		})
		return
	}
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		a, err := h.Store.Get(r.Context(), artifact)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		source, err := h.Store.HTML(r.Context(), artifact, a.Version)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		available := map[string]json.RawMessage{}
		for name, decl := range a.Manifest {
			switch name {
			case "sample", "user", "db", "artifact", "assets", "downloads", "room", "comments":
				available[name] = decl
			case "files":
				if h.ProjectRoot != "" {
					available[name] = decl
				}
			}
		}
		var owner any
		if _, ok := a.Manifest["user"]; ok {
			owner = h.Owner
		}
		boot, _ := json.Marshal(map[string]any{"artifactId": artifact, "version": a.Version, "manifest": available, "owner": owner, "models": h.Backend.Models()})
		injected := []byte(`<script type="application/json" id="francois-view">` + string(boot) + `</script><script src="/static/runtime.js"></script>`)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(inject(source, injected))
	})
	m.HandleFunc("POST /api/views", func(w http.ResponseWriter, r *http.Request) {
		_, err := h.Store.Get(r.Context(), artifact)
		if err != nil {
			fail(w, 404, "unavailable", "Artifact missing")
			return
		}
		var loaded struct {
			Version int64 `json:"version"`
		}
		if err := readJSON(w, r, &loaded); err != nil || loaded.Version < 1 {
			fail(w, 400, "invalid_request", "A loaded page version is required")
			return
		}
		if _, err := h.Store.HTML(r.Context(), artifact, loaded.Version); err != nil {
			fail(w, 400, "invalid_request", "Page version does not exist")
			return
		}
		ctx, cancel := contextForView(h.ctx)
		v := &View{ID: id("v_"), Artifact: artifact, Version: loaded.Version, ctx: ctx, cancel: cancel, events: make(chan any, 128), calls: map[string]*call{}, subscriptions: map[string]json.RawMessage{}, presence: map[string]map[string]any{}, rooms: map[string]bool{"": true}}
		h.mu.Lock()
		h.views[v.ID] = v
		h.mu.Unlock()
		// A view that never connects must not remain in memory.
		h.group.Go(func() error {
			timer := time.NewTimer(15 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				v.mu.Lock()
				up := v.connected
				v.mu.Unlock()
				if !up {
					h.closeView(v)
				}
			case <-v.ctx.Done():
			}
			return nil
		})
		respond(w, map[string]any{"viewId": v.ID, "version": loaded.Version})
	})
	m.HandleFunc("GET /api/views/{view}/events", func(w http.ResponseWriter, r *http.Request) {
		v := h.view(w, r, artifact)
		if v == nil {
			return
		}
		flush, ok := w.(http.Flusher)
		if !ok {
			fail(w, 500, "unavailable", "Streaming unavailable")
			return
		}
		v.mu.Lock()
		if v.connected {
			v.mu.Unlock()
			fail(w, 409, "unavailable", "View already connected")
			return
		}
		v.connected = true
		v.mu.Unlock()
		defer h.closeView(v)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")
		_, _ = io.WriteString(w, "data: {\"type\":\"ready\"}\n\n")
		flush.Flush()
		h.roomBroadcast(artifact)
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
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
			case <-tick.C:
				if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
					return
				}
				flush.Flush()
			case <-r.Context().Done():
				return
			case <-v.ctx.Done():
				return
			}
		}
	})
	m.HandleFunc("POST /api/views/{view}/calls", func(w http.ResponseWriter, r *http.Request) { h.sample(w, r, artifact) })
	m.HandleFunc("POST /api/views/{view}/calls/{call}/cancel", func(w http.ResponseWriter, r *http.Request) {
		v := h.view(w, r, artifact)
		if v == nil {
			return
		}
		v.mu.Lock()
		if c := v.calls[r.PathValue("call")]; c != nil {
			c.cancel()
		}
		v.mu.Unlock()
		respond(w, map[string]bool{"cancelled": true})
	})
	m.HandleFunc("POST /api/views/{view}/calls/{call}/tools/{tool}/result", func(w http.ResponseWriter, r *http.Request) {
		v := h.view(w, r, artifact)
		if v == nil {
			return
		}
		var result piworker.ToolResult
		if err := readJSON(w, r, &result); err != nil || len(result.Value) > 32768 {
			fail(w, 400, "invalid_request", "Invalid tool result")
			return
		}
		v.mu.Lock()
		defer v.mu.Unlock()
		c := v.calls[r.PathValue("call")]
		if c == nil {
			fail(w, 404, "cancelled", "Call has ended")
			return
		}
		ch := c.tools[r.PathValue("tool")]
		if ch == nil {
			fail(w, 404, "invalid_request", "Unknown tool call")
			return
		}
		select {
		case ch <- result:
			respond(w, map[string]bool{"accepted": true})
		default:
			fail(w, 409, "invalid_request", "Tool result already received")
		}
	})
	m.HandleFunc("POST /api/views/{view}/rpc", func(w http.ResponseWriter, r *http.Request) { h.rpc(w, r, artifact) })
	m.HandleFunc("GET /shell", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(web.Index)
	})
	m.HandleFunc("GET /api/threads", func(w http.ResponseWriter, r *http.Request) { h.listThreads(w, r, artifact) })
	m.HandleFunc("GET /_blob/{id}", func(w http.ResponseWriter, r *http.Request) { h.blob(w, r, artifact) })
	m.HandleFunc("POST /api/assets", func(w http.ResponseWriter, r *http.Request) { h.upload(w, r, artifact) })
	m.HandleFunc("GET /api/files", func(w http.ResponseWriter, r *http.Request) { h.readFile(w, r, artifact) })
}
func inject(source, script []byte) []byte {
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	offset := 0
	for {
		t := tokenizer.Next()
		raw := tokenizer.Raw()
		offset += len(raw)
		if t == html.ErrorToken {
			break
		}
		if t == html.StartTagToken {
			tag, _ := tokenizer.TagName()
			if strings.EqualFold(string(tag), "head") {
				out := append([]byte{}, source[:offset]...)
				out = append(out, script...)
				return append(out, source[offset:]...)
			}
		}
	}
	return append(append([]byte{}, script...), source...)
}
func (h *Host) importArtifact(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID       string                     `json:"id"`
		Title    string                     `json:"title"`
		HTML     string                     `json:"html"`
		Manifest map[string]json.RawMessage `json:"manifest"`
	}
	// HTML imports may be larger than ordinary capability requests.
	r.Body = http.MaxBytesReader(w, r.Body, 18<<20)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		fail(w, 400, "invalid_content", err.Error())
		return
	}
	if !strings.Contains(strings.ToLower(input.HTML), "<html") {
		fail(w, 400, "invalid_content", "Import a complete HTML document")
		return
	}
	if input.Title == "" {
		input.Title = input.ID
	}
	if input.Manifest == nil {
		input.Manifest = map[string]json.RawMessage{"sample": json.RawMessage(`{}`)}
	}
	a, err := h.Store.Import(r.Context(), input.ID, input.Title, []byte(input.HTML), input.Manifest)
	if err != nil {
		fail(w, 400, "invalid_content", err.Error())
		return
	}
	if err = h.mount(a); err != nil {
		fail(w, 500, "unavailable", err.Error())
		return
	}
	respond(w, a)
}
