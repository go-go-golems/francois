// Package host serves the local artifact runtime and its view-owned operations.
package host

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/go-go-golems/francois/pkg/store"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

func id(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b)
}

type View struct {
	ID            string
	Artifact      string
	Version       int64
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	connected     bool
	events        chan any
	calls         map[string]*call
	subscriptions map[string]json.RawMessage
	presence      map[string]map[string]any
	rooms         map[string]bool
}
type call struct {
	cancel context.CancelFunc
	tools  map[string]chan piworker.ToolResult
	names  map[string]bool
}
type Host struct {
	Store       *store.Store
	Backend     piworker.Backend
	Owner       string
	ProjectRoot string
	ctx         context.Context
	cancel      context.CancelFunc
	group       *errgroup.Group
	mu          sync.Mutex
	views       map[string]*View
	servers     []*http.Server
	artifacts   map[string]string
	active      chan struct{}
	queued      chan struct{}
}

func New(ctx context.Context, s *store.Store, b piworker.Backend, root string) (*Host, error) {
	owner, err := s.Identity(ctx, id("u_"))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	return &Host{Store: s, Backend: b, Owner: owner, ProjectRoot: root, ctx: ctx, cancel: cancel, group: g, views: map[string]*View{}, artifacts: map[string]string{}, active: make(chan struct{}, 2), queued: make(chan struct{}, 10)}, nil
}
func (h *Host) Run(listen string) error {
	defer func() {
		h.cancel()
		h.mu.Lock()
		servers := append([]*http.Server(nil), h.servers...)
		h.mu.Unlock()
		for _, s := range servers {
			_ = s.Close()
		}
	}()
	ip, _, err := net.SplitHostPort(listen)
	if err != nil {
		return err
	}
	if net.ParseIP(ip) == nil || !net.ParseIP(ip).IsLoopback() {
		return errors.New("listen must use a loopback IP address")
	}
	artifacts, err := h.Store.List(h.ctx)
	if err != nil {
		return err
	}
	for _, a := range artifacts {
		if err = h.mount(a); err != nil {
			return err
		}
	}
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	h.start(l, "")
	log.Info().Str("address", l.Addr().String()).Msg("Francois ready")
	<-h.ctx.Done()
	h.cancel()
	h.mu.Lock()
	servers := append([]*http.Server(nil), h.servers...)
	for _, v := range h.views {
		v.cancel()
	}
	h.mu.Unlock()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range servers {
		if e := s.Shutdown(shutdown); e != nil {
			_ = s.Close()
		}
	}
	return h.group.Wait()
}
func (h *Host) mount(a store.Artifact) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.artifacts[a.ID]; ok {
		return nil
	}
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(a.Port)))
	if err != nil {
		return errors.Wrapf(err, "bind artifact %s port %d", a.ID, a.Port)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err = h.Store.SetPort(h.ctx, a.ID, port); err != nil {
		_ = l.Close()
		return err
	}
	h.artifacts[a.ID] = "http://" + l.Addr().String()
	h.startLocked(l, a.ID)
	return nil
}
func (h *Host) start(l net.Listener, artifact string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.startLocked(l, artifact)
}
func (h *Host) startLocked(l net.Listener, artifact string) {
	s := &http.Server{Handler: h.Handler(artifact, l.Addr().String()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	h.servers = append(h.servers, s)
	h.group.Go(func() error {
		err := s.Serve(l)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})
}
func (h *Host) Handler(artifact, address string) http.Handler {
	mux := http.NewServeMux()
	h.routes(mux, artifact)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != address {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || u.Host != address || u.Scheme != "http" {
				http.Error(w, "invalid origin", http.StatusForbidden)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(dst); err != nil {
		return err
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	respond(w, map[string]any{"error": piworker.Failure{Code: code, Message: message}})
}
func (h *Host) view(w http.ResponseWriter, r *http.Request, artifact string) *View {
	h.mu.Lock()
	v := h.views[r.PathValue("view")]
	h.mu.Unlock()
	if v == nil || v.Artifact != artifact || r.Header.Get("X-Francois-View") != v.ID {
		fail(w, 404, "unavailable", "View not found")
		return nil
	}
	return v
}
func (v *View) emit(event any) bool {
	select {
	case v.events <- event:
		return true
	case <-v.ctx.Done():
		return false
	default:
		v.cancel()
		return false
	}
}
func (h *Host) broadcast(artifact string, event any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, v := range h.views {
		if v.Artifact == artifact {
			v.emit(event)
		}
	}
}
func (h *Host) closeView(v *View) {
	v.cancel()
	v.mu.Lock()
	v.connected = false
	for _, c := range v.calls {
		c.cancel()
	}
	v.mu.Unlock()
	h.mu.Lock()
	delete(h.views, v.ID)
	h.mu.Unlock()
	h.roomBroadcast(v.Artifact)
}
func (h *Host) sample(w http.ResponseWriter, r *http.Request, artifact string) {
	v := h.view(w, r, artifact)
	if v == nil {
		return
	}
	a, err := h.Store.Get(r.Context(), artifact)
	if err != nil {
		return
	}
	if _, ok := a.Manifest["sample"]; !ok {
		fail(w, 403, "not_declared", "Sampling is not declared")
		return
	}
	if len(h.Backend.Models()) == 0 {
		fail(w, 503, "sampling_disabled", "The model worker is unavailable; restart the host after checking Pi configuration")
		return
	}
	var req piworker.Request
	if err = readJSON(w, r, &req); err != nil {
		fail(w, 400, "invalid_request", err.Error())
		return
	}
	if len(req.Images) > 0 {
		var declaration struct {
			Images bool `json:"images"`
		}
		_ = json.Unmarshal(a.Manifest["sample"], &declaration)
		if !declaration.Images {
			fail(w, 400, "images_unavailable", "Images are not declared")
			return
		}
	}
	if req.CallID == "" || len(req.CallID) > 100 {
		fail(w, 400, "invalid_request", "Invalid call id")
		return
	}
	if err = validateSample(req); err != nil {
		fail(w, 400, "invalid_request", err.Error())
		return
	}
	select {
	case h.queued <- struct{}{}:
	default:
		fail(w, 429, "rate_limited", "The model request queue is full")
		return
	}
	ctx, cancel := context.WithTimeout(v.ctx, 5*time.Minute)
	c := &call{cancel: cancel, tools: map[string]chan piworker.ToolResult{}, names: map[string]bool{}}
	for _, t := range req.Tools {
		c.names[t.Name] = true
	}
	v.mu.Lock()
	if !v.connected || v.calls[req.CallID] != nil {
		v.mu.Unlock()
		cancel()
		<-h.queued
		fail(w, 409, "invalid_request", "View disconnected or call id already active")
		return
	}
	v.calls[req.CallID] = c
	v.mu.Unlock()
	h.group.Go(func() error {
		defer func() { cancel(); <-h.queued; v.mu.Lock(); delete(v.calls, req.CallID); v.mu.Unlock() }()
		select {
		case h.active <- struct{}{}:
			defer func() { <-h.active }()
		case <-ctx.Done():
			v.emit(piworker.Event{Type: "error", CallID: req.CallID, Error: &piworker.Failure{Code: "cancelled", Message: "Stopped while queued"}})
			return nil
		}
		settled := false
		emit := func(e piworker.Event) {
			if settled {
				return
			}
			if e.Type == "result" || e.Type == "error" {
				settled = true
			}
			v.emit(e)
		}
		execute := func(ctx context.Context, event piworker.Event) (piworker.ToolResult, error) {
			if !c.names[event.Name] {
				return piworker.ToolResult{}, errors.New("Unknown page tool")
			}
			child, stop := context.WithTimeout(ctx, 150*time.Second)
			defer stop()
			ch := make(chan piworker.ToolResult, 1)
			v.mu.Lock()
			if c.tools[event.ToolCallID] != nil {
				v.mu.Unlock()
				return piworker.ToolResult{}, errors.New("Duplicate tool call")
			}
			c.tools[event.ToolCallID] = ch
			v.mu.Unlock()
			defer func() { v.mu.Lock(); delete(c.tools, event.ToolCallID); v.mu.Unlock() }()
			v.emit(event)
			select {
			case result := <-ch:
				return result, nil
			case <-child.Done():
				return piworker.ToolResult{}, child.Err()
			}
		}
		err := h.Backend.Sample(ctx, req, emit, execute)
		if !settled {
			code := "upstream_error"
			if ctx.Err() != nil {
				code = "cancelled"
			}
			message := "Model request failed"
			if err != nil {
				message = err.Error()
			}
			emit(piworker.Event{Type: "error", CallID: req.CallID, Error: &piworker.Failure{Code: code, Message: message}})
		}
		return nil
	})
	w.WriteHeader(http.StatusAccepted)
	respond(w, map[string]string{"callId": req.CallID})
}

var toolName = regexp.MustCompile(`^[-A-Za-z0-9_]{1,128}$`)

func validateSample(r piworker.Request) error {
	if len(r.Images) > 4 {
		return errors.New("At most four images")
	}
	total := 0
	for _, image := range r.Images {
		raw, err := base64.StdEncoding.DecodeString(image.Data)
		if err != nil || image.MimeType != "image/jpeg" || len(raw) == 0 {
			return errors.New("Invalid prepared image")
		}
		total += len(raw)
	}
	if total > 8<<20 {
		return errors.New("Prepared images exceed 8 MiB")
	}
	var text string
	if json.Unmarshal(r.Input, &text) != nil {
		var turns []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(r.Input, &turns); err != nil || len(turns) == 0 {
			return errors.New("Input must be text or turns")
		}
		if turns[0].Role != "user" || turns[len(turns)-1].Role != "user" {
			return errors.New("Turns must begin and end with user")
		}
		for _, t := range turns {
			if t.Role != "user" && t.Role != "assistant" {
				return errors.New("Invalid turn role")
			}
			text += t.Content
		}
	}
	if strings.TrimSpace(text) == "" || len(text) > 262144 {
		return errors.New("Prompt is empty or exceeds 262144 UTF-8 bytes")
	}
	if r.Options.ModelTier != "quick" && r.Options.ModelTier != "default" && r.Options.ModelTier != "complex" {
		return errors.New("Invalid model tier")
	}
	if r.Options.Effort != "" && r.Options.Effort != "low" && r.Options.Effort != "medium" && r.Options.Effort != "high" {
		return errors.New("Invalid effort")
	}
	if len(r.Tools) > 16 {
		return errors.New("At most 16 tools")
	}
	names := map[string]bool{}
	for _, t := range r.Tools {
		if !toolName.MatchString(t.Name) || names[t.Name] || len(t.Description) > 1024 || len(t.InputSchema) > 4096 {
			return errors.New("Invalid tool descriptor")
		}
		if len(t.InputSchema) > 0 {
			var schema map[string]any
			if json.Unmarshal(t.InputSchema, &schema) != nil || schema["type"] != "object" {
				return errors.New("Tool schema must describe an object")
			}
		}
		names[t.Name] = true
	}
	return nil
}
func contextForView(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}
