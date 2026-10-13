package hosted

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
)

// resolveModel maps the page's request onto an approved model. Explicit
// models outside the approved set are rejected, never silently replaced.
func resolveModel(doc settings.Document, req piworker.Request, available map[string]piworker.Model) (settings.Model, string, *errCode) {
	selector, thinking := req.Options.Model, ""
	if selector == "" {
		selector = doc.Models.Tiers[req.Options.ModelTier]
		if selector == "" {
			return settings.Model{}, "", &errCode{503, "sampling_disabled", "No approved model is configured for the " + req.Options.ModelTier + " tier"}
		}
	} else if i := strings.LastIndex(selector, ":"); i > 0 {
		selector, thinking = selector[:i], selector[i+1:]
	}
	m, ok := doc.Model(selector)
	if !ok {
		return m, "", &errCode{403, "model_not_allowed", "Model " + selector + " is not included in this plan"}
	}
	if _, ok = available[selector]; !ok {
		return m, "", &errCode{503, "model_unavailable", "Model " + selector + " is temporarily unavailable"}
	}
	level := thinking
	if level == "" {
		level = req.Options.Effort
	}
	if level != "" && len(m.Thinking) > 0 {
		allowed := false
		for _, t := range m.Thinking {
			allowed = allowed || t == level
		}
		if !allowed {
			return m, "", &errCode{403, "model_not_allowed", "Thinking level " + level + " is not included for " + selector}
		}
	}
	out := selector
	if thinking != "" {
		out += ":" + thinking
	}
	return m, out, nil
}

func (rt *Runtime) admit(user string, perUser, global int) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.active[user] >= perUser || rt.global >= global {
		return false
	}
	rt.active[user]++
	rt.global++
	return true
}

func (rt *Runtime) release(user string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.active[user]--
	if rt.active[user] <= 0 {
		delete(rt.active, user)
	}
	rt.global--
}

// sample admits one model call charged to the caller's allowance: validate,
// resolve an approved model, admit concurrency, reserve credits, then run.
func (rt *Runtime) sample(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	v, p := rt.view(w, r, inst)
	if v == nil {
		return
	}
	if !rt.declared(p, "sample") {
		httpx.Fail(w, 403, "not_declared", "Sampling is not declared")
		return
	}
	var req piworker.Request
	if err := httpx.Read(w, r, &req, 16<<20); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	if req.CallID == "" || len(req.CallID) > 100 {
		httpx.Fail(w, 400, "invalid_request", "Invalid call id")
		return
	}
	if err := piworker.ValidateRequest(req); err != nil {
		httpx.Fail(w, 400, "invalid_request", err.Error())
		return
	}
	rev, err := rt.Store.ActiveSettings(r.Context())
	if err != nil {
		failErr(w, err)
		return
	}
	availableModels := map[string]piworker.Model{}
	for _, m := range rt.Backend.Models() {
		availableModels[m.Provider+"/"+m.ID] = m
	}
	model, selector, ec := resolveModel(rev.Document, req, availableModels)
	if ec != nil {
		httpx.Fail(w, ec.status, ec.code, ec.msg)
		return
	}
	if len(req.Images) > 0 {
		var decl struct {
			Images bool `json:"images"`
		}
		_ = json.Unmarshal(p.Release.Manifest["sample"], &decl)
		if !decl.Images || !availableModels[model.Selector].Images {
			httpx.Fail(w, 400, "images_unavailable", "Images are not available for this request")
			return
		}
	}
	req.Options.Model = selector
	req.MaxRounds = rev.Document.Allowance.MaxToolRounds
	a := rev.Document.Allowance
	if !rt.admit(p.User.ID, a.PerUserConcurrency, a.GlobalConcurrency) {
		httpx.Fail(w, 429, "rate_limited", "Too many model calls are running; try again shortly")
		return
	}
	res, err := rt.Store.Reserve(r.Context(), store.ReserveRequest{UserID: p.User.ID, InstanceID: inst.ID, CallID: req.CallID, Model: selector,
		Credits: model.Credits, Included: a.MonthlyCredits, Daily: a.DailyCredits, SettingsRevision: rev.Revision})
	if err != nil {
		rt.release(p.User.ID)
		switch {
		case errors.Is(err, store.ErrAllowance):
			httpx.Fail(w, 402, "allowance_exhausted", "Your included model allowance for this period is used up")
		case errors.Is(err, store.ErrDaily):
			httpx.Fail(w, 429, "daily_limit", "The daily model limit has been reached")
		default:
			failErr(w, err)
		}
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
		rt.release(p.User.ID)
		_ = rt.Store.SettleCall(context.Background(), res, false, "rejected", store.CallUsage{})
		httpx.Fail(w, 409, "invalid_request", "View disconnected or call id already active")
		return
	}
	v.calls[req.CallID] = c
	v.mu.Unlock()
	rt.group.Go(func() error {
		defer func() {
			cancel()
			rt.release(p.User.ID)
			v.mu.Lock()
			delete(v.calls, req.CallID)
			v.mu.Unlock()
		}()
		rt.run(ctx, v, c, req, res)
		return nil
	})
	httpx.JSON(w, http.StatusAccepted, map[string]any{"callId": req.CallID})
}

// run records provider start, streams events to the caller's view only,
// aggregates private usage frames and settles the reservation exactly once.
func (rt *Runtime) run(ctx context.Context, v *View, c *call, req piworker.Request, res store.Reservation) {
	settleCtx := context.Background()
	if err := rt.Store.StartCall(settleCtx, res); err != nil {
		_ = rt.Store.SettleCall(settleCtx, res, false, "start_failed", store.CallUsage{})
		v.emit(piworker.Event{Type: "error", CallID: req.CallID, Error: &piworker.Failure{Code: "upstream_error", Message: "Could not start the model call"}})
		return
	}
	usage := store.CallUsage{}
	settled, ok, code := false, false, ""
	emit := func(e piworker.Event) {
		if e.Type == "usage" {
			if e.Usage != nil {
				usage.Input += e.Usage.Input
				usage.Output += e.Usage.Output
				usage.CacheRead += e.Usage.CacheRead
				usage.CacheWrite += e.Usage.CacheWrite
				usage.Cost += e.Usage.Cost
				usage.Rounds++
				usage.ModelUsed = e.Usage.Model
			}
			return
		}
		if settled {
			return
		}
		if e.Type == "result" {
			settled, ok = true, true
			if e.Result != nil && e.Result.ModelUsed != "" {
				usage.ModelUsed = e.Result.ModelUsed
			}
		}
		if e.Type == "error" {
			settled = true
			if e.Error != nil {
				code = e.Error.Code
			}
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
	err := rt.Backend.Sample(ctx, req, emit, execute)
	if !settled {
		code = "upstream_error"
		if ctx.Err() != nil {
			code = "cancelled"
		}
		message := "Model request failed"
		if err != nil && code != "cancelled" {
			message = err.Error()
		}
		emit(piworker.Event{Type: "error", CallID: req.CallID, Error: &piworker.Failure{Code: code, Message: message}})
	}
	if err := rt.Store.SettleCall(settleCtx, res, ok, code, usage); err != nil {
		log.Error().Err(err).Str("call", res.ID).Msg("settle model call")
	}
}

func (rt *Runtime) cancelCall(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	v, _ := rt.view(w, r, inst)
	if v == nil {
		return
	}
	v.mu.Lock()
	if c := v.calls[r.PathValue("call")]; c != nil {
		c.cancel()
	}
	v.mu.Unlock()
	httpx.OK(w, map[string]bool{"cancelled": true})
}

func (rt *Runtime) toolResult(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	v, _ := rt.view(w, r, inst)
	if v == nil {
		return
	}
	var result piworker.ToolResult
	if err := httpx.Read(w, r, &result, 64<<10); err != nil || len(result.Value) > 32768 {
		httpx.Fail(w, 400, "invalid_request", "Invalid tool result")
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	c := v.calls[r.PathValue("call")]
	if c == nil {
		httpx.Fail(w, 404, "cancelled", "Call has ended")
		return
	}
	ch := c.tools[r.PathValue("tool")]
	if ch == nil {
		httpx.Fail(w, 404, "invalid_request", "Unknown tool call")
		return
	}
	select {
	case ch <- result:
		httpx.OK(w, map[string]bool{"accepted": true})
	default:
		httpx.Fail(w, 409, "invalid_request", "Tool result already received")
	}
}
