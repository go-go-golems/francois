package hosted

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	catalogweb "github.com/go-go-golems/francois/pkg/catalog/web"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/pkg/errors"
	xhtml "golang.org/x/net/html"
)

// Handler serves one instance's isolated origin.
func (rt *Runtime) Handler(inst store.Instance) http.Handler {
	origin := rt.Config.AppOrigin(inst.Hostname)
	m := http.NewServeMux()
	m.HandleFunc("GET /static/{name}", func(w http.ResponseWriter, r *http.Request) {
		data, mime, ok := catalogweb.AppAsset(r.PathValue("name"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime)
		_, _ = w.Write(data)
	})
	m.HandleFunc("GET /launch", func(w http.ResponseWriter, r *http.Request) {
		// Trusted bootstrap: no paid bytes, frameable only by the portal.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "frame-ancestors "+rt.Config.PortalOrigin)
		_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en" data-portal="%s"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><link rel="stylesheet" href="/static/launch.css"></head><body><p id="francois-launch">Connecting…</p><script src="/static/launch.js"></script></body></html>`,
			html.EscapeString(rt.Config.PortalOrigin), html.EscapeString(inst.Title))
	})
	m.HandleFunc("POST /api/launch/exchange", func(w http.ResponseWriter, r *http.Request) { rt.exchange(w, r, inst, origin) })
	m.HandleFunc("GET /page", func(w http.ResponseWriter, r *http.Request) { rt.page(w, r, inst, origin) })
	m.HandleFunc("POST /api/views", func(w http.ResponseWriter, r *http.Request) { rt.createView(w, r, inst) })
	m.HandleFunc("GET /api/views/{view}/events", func(w http.ResponseWriter, r *http.Request) { rt.events(w, r, inst) })
	m.HandleFunc("POST /api/views/{view}/rpc", func(w http.ResponseWriter, r *http.Request) { rt.rpc(w, r, inst) })
	m.HandleFunc("POST /api/views/{view}/calls", func(w http.ResponseWriter, r *http.Request) { rt.sample(w, r, inst) })
	m.HandleFunc("POST /api/views/{view}/calls/{call}/cancel", func(w http.ResponseWriter, r *http.Request) { rt.cancelCall(w, r, inst) })
	m.HandleFunc("POST /api/views/{view}/calls/{call}/tools/{tool}/result", func(w http.ResponseWriter, r *http.Request) { rt.toolResult(w, r, inst) })
	m.HandleFunc("POST /api/assets", func(w http.ResponseWriter, r *http.Request) { rt.upload(w, r, inst) })
	m.HandleFunc("GET /_blob/{id}", func(w http.ResponseWriter, r *http.Request) { rt.blob(w, r, inst) })
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/launch", http.StatusFound)
			return
		}
		httpx.Fail(w, 404, "not_found", "Not found")
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if o := r.Header.Get("Origin"); o != "" && o != origin {
			httpx.Fail(w, 403, "invalid_origin", "Cross-origin requests are not accepted")
			return
		}
		m.ServeHTTP(w, r)
	})
}

// exchange redeems a one-use, browser-bound launch code for a runtime grant.
func (rt *Runtime) exchange(w http.ResponseWriter, r *http.Request, inst store.Instance, origin string) {
	if r.Header.Get("Origin") != origin {
		httpx.Fail(w, 403, "invalid_origin", "Exchange must come from the instance origin")
		return
	}
	var in struct {
		Code     string `json:"code"`
		Verifier string `json:"verifier"`
	}
	if err := httpx.Read(w, r, &in, 4096); err != nil || in.Code == "" || len(in.Verifier) < 43 || len(in.Verifier) > 128 {
		httpx.Fail(w, 400, "invalid_request", "Code and verifier are required")
		return
	}
	code, err := rt.Store.ConsumeLaunchCode(r.Context(), in.Code, origin)
	if err != nil || code.InstanceID != inst.ID {
		httpx.Fail(w, 401, "launch_invalid", "Launch code is invalid, expired or already used")
		return
	}
	sum := sha256.Sum256([]byte(in.Verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != code.Challenge {
		httpx.Fail(w, 401, "launch_invalid", "Launch verifier does not match")
		return
	}
	if _, err = rt.principal(r.Context(), code.UserID, inst.ID, nil, code.SessionHash); err != nil {
		failErr(w, err)
		return
	}
	expires := time.Now().Add(rt.Config.GrantTTL)
	token, err := rt.Store.CreateGrant(r.Context(), store.Grant{SessionHash: code.SessionHash, UserID: code.UserID, InstanceID: inst.ID, ExpiresAt: expires})
	if err != nil {
		httpx.Fail(w, 401, "session_ended", "The portal session has ended")
		return
	}
	httpx.OK(w, map[string]any{"grant": token, "expiresAt": expires})
}

func (rt *Runtime) models(p Principal) []piworker.Model {
	rev, err := rt.Store.ActiveSettings(rt.ctx)
	if err != nil {
		return []piworker.Model{}
	}
	backend := map[string]piworker.Model{}
	for _, m := range rt.Backend.Models() {
		backend[m.Provider+"/"+m.ID] = m
	}
	out := []piworker.Model{}
	for _, a := range rev.Document.Models.Allowed {
		m, ok := backend[a.Selector]
		if !ok {
			continue
		}
		m.ThinkingLevels = a.Thinking
		m.Name = a.Label
		m.Tier = ""
		for tier, sel := range rev.Document.Models.Tiers {
			if sel == a.Selector && tier == "default" {
				m.Tier = tier
			}
		}
		out = append(out, m)
	}
	return out
}

// profile is the public co-member view of a user: no email or billing data.
func profile(u store.User, me string) map[string]any {
	name := u.DisplayName
	if name == "" {
		name = strings.Split(u.Email, "@")[0]
	}
	sum := sha256.Sum256([]byte(u.ID))
	hue := int(sum[0]) * 360 / 256
	return map[string]any{"id": u.ID, "name": name, "color": fmt.Sprintf("hsl(%d 60%% 45%%)", hue), "avatarUrl": avatar(name, hue), "isMe": u.ID == me, "guest": false, "email": nil}
}

func avatar(name string, hue int) string {
	initial := "?"
	if name != "" {
		initial = strings.ToUpper(string([]rune(name)[0]))
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="hsl(%d 60%% 45%%)"/><text x="16" y="21" font-family="sans-serif" font-size="15" fill="#fff" text-anchor="middle">%s</text></svg>`, hue, html.EscapeString(initial))
	return "data:image/svg+xml," + url.PathEscape(svg)
}

// page returns the authored HTML with the hosted runtime injected. It is the
// only route that returns release bytes, and only to a valid grant.
func (rt *Runtime) page(w http.ResponseWriter, r *http.Request, inst store.Instance, origin string) {
	token := bearer(r)
	p, err := rt.authenticate(r.Context(), token, inst)
	if err != nil {
		failErr(w, err)
		return
	}
	source, err := rt.Publisher.HTML(p.Release)
	if err != nil {
		failErr(w, err)
		return
	}
	g, err := rt.Store.ResolveGrant(r.Context(), token, inst.ID)
	if err != nil {
		failErr(w, err)
		return
	}
	me := profile(p.User, p.User.ID)
	me["isOwner"] = p.Role == "owner"
	me["canEdit"] = p.CanEdit()
	me["role"] = p.Role
	boot, _ := json.Marshal(map[string]any{
		"hosted": true, "artifactId": p.Release.ArtifactID, "version": 1, "release": p.Release.Version,
		"manifest": available(p.Release.Manifest), "owner": p.User.ID, "me": me, "models": rt.models(p),
		"grant": token, "grantExpiresAt": g.ExpiresAt, "portalOrigin": rt.Config.PortalOrigin, "origin": origin,
		"instance": map[string]any{"id": inst.ID, "title": p.Instance.Title, "role": p.Role},
	})
	// Escape "<" so JSON cannot terminate the script element.
	boot = bytes.ReplaceAll(boot, []byte("<"), []byte(`<`))
	injected := []byte(`<script type="application/json" id="francois-view">` + string(boot) + `</script><script src="/static/runtime.js"></script>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(inject(source, injected))
}

func inject(source, script []byte) []byte {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(source))
	offset := 0
	for {
		t := tokenizer.Next()
		offset += len(tokenizer.Raw())
		if t == xhtml.ErrorToken {
			break
		}
		if t == xhtml.StartTagToken {
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

var accepted = map[string]int64{"image/png": 20 << 20, "image/jpeg": 20 << 20, "image/gif": 20 << 20, "image/webp": 20 << 20, "video/mp4": 20 << 20, "video/webm": 20 << 20, "application/pdf": 20 << 20, "font/woff2": 20 << 20, "font/woff": 20 << 20, "text/csv": 20 << 20, "text/markdown": 20 << 20, "application/json": 20 << 20, "text/plain": 20 << 20}

func (rt *Runtime) upload(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	p, err := rt.authenticate(r.Context(), bearer(r), inst)
	if err != nil {
		failErr(w, err)
		return
	}
	if !rt.declared(p, "assets") {
		httpx.Fail(w, 403, "not_declared", "Assets not declared")
		return
	}
	if !p.CanEdit() {
		httpx.Fail(w, 403, "read_only", "Viewers cannot upload assets")
		return
	}
	kind := r.Header.Get("Content-Type")
	limit, ok := accepted[kind]
	if !ok {
		httpx.Fail(w, 400, "unsupported_type", "Unsupported asset type")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil || len(data) == 0 {
		httpx.Fail(w, 413, "too_large", "Invalid asset size")
		return
	}
	rev, err := rt.Store.ActiveSettings(r.Context())
	if err != nil {
		failErr(w, err)
		return
	}
	a, err := rt.Store.PutAsset(r.Context(), inst.ID, p.User.ID, kind, data, rev.Document.Limits.AssetBytesPerInstance)
	if errors.Is(err, store.ErrLimit) {
		httpx.Fail(w, 413, "quota_or_state", "Asset quota reached")
		return
	}
	if err != nil {
		failErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"id": a.ID, "url": rt.signBlob(inst.ID, a.ID), "sizeBytes": a.Size, "contentType": a.MediaType})
}

func (rt *Runtime) blob(w http.ResponseWriter, r *http.Request, inst store.Instance) {
	id := r.PathValue("id")
	if !rt.verifyBlob(inst.ID, id, r.URL.Query().Get("e"), r.URL.Query().Get("s")) {
		http.NotFound(w, r)
		return
	}
	kind, data, err := rt.Store.AssetBytes(r.Context(), inst.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", kind)
	if !strings.HasPrefix(kind, "image/") && !strings.HasPrefix(kind, "font/") && !strings.HasPrefix(kind, "video/") {
		w.Header().Set("Content-Disposition", "attachment")
	}
	_, _ = w.Write(data)
}
