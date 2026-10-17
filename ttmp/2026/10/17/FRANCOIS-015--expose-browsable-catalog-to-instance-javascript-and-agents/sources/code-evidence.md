# Primary code evidence

Captured from working tree based on commit `0b1c302394491e977241a558b2392e5fe50be43f`. Other concurrent ticket work may be present; excerpts show actual bytes at capture time. No claim of production configuration is made.

## pkg/catalog/pages.go:406

```text
406: func (s *Server) artifactPage(w http.ResponseWriter, r *http.Request) {
407: 	ctx := r.Context()
408: 	p := principal(ctx)
409: 	a, err := s.Store.Artifact(ctx, r.PathValue("id"))
410: 	if errors.Is(err, store.ErrNotFound) {
411: 		s.notFound(w, r)
412: 		return
413: 	}
414: 	if err != nil {
415: 		s.pageFailed(w, r, err)
416: 		return
417: 	}
418: 	draft := r.URL.Query().Get("draft") == "1" && p != nil && s.canEditArtifact(ctx, p.User, a.ID)
419: 	rels, err := s.Store.Releases(ctx, a.ID, "published")
420: 	if err != nil {
421: 		s.pageFailed(w, r, err)
422: 		return
423: 	}
424: 	if !draft && (a.Visibility != "listed" || len(rels) == 0) {
425: 		s.notFound(w, r)
426: 		return
427: 	}
428: 	page := a.Page
429: 	if draft && a.PageDraft != nil {
430: 		page = *a.PageDraft
431: 	}
432: 	credits, err := s.Store.ArtifactCreators(ctx, []string{a.ID})
433: 	if err != nil {
434: 		s.pageFailed(w, r, err)
435: 		return
```

## pkg/catalog/pages.go:579

```text
579: func (s *Server) postPage(w http.ResponseWriter, r *http.Request) {
580: 	ctx := r.Context()
581: 	post, err := s.Store.PostBySlug(ctx, r.PathValue("slug"))
582: 	if errors.Is(err, store.ErrNotFound) {
583: 		s.notFound(w, r)
584: 		return
585: 	}
586: 	if err != nil {
587: 		s.pageFailed(w, r, err)
588: 		return
589: 	}
590: 	pr := principal(ctx)
591: 	draft := post.State != "published"
592: 	if draft && (pr == nil || !s.canEditPost(ctx, pr.User, post)) {
593: 		s.notFound(w, r)
594: 		return
595: 	}
596: 	body := map[string]any{"Post": post, "Body": s.markdown(ctx, post.Body)}
597: 	var author *store.Creator
598: 	if post.AuthorID != "" {
599: 		if c, err := s.Store.Creator(ctx, post.AuthorID); err == nil {
600: 			author = &c
```

## pkg/catalog/routes.go:244

```text
244: func (s *Server) mayDownload(w http.ResponseWriter, r *http.Request, rel store.Release) bool {
245: 	p := principal(r.Context())
246: 	if p != nil && p.User.Disabled {
247: 		httpx.Fail(w, 403, billing.ReasonDisabled, "This account is disabled")
248: 		return false
249: 	}
250: 	if rel.FreeSample {
251: 		return true
252: 	}
253: 	if p == nil {
254: 		httpx.Fail(w, 401, "login_required", "Sign in to download")
255: 		return false
256: 	}
257: 	a, err := s.Store.Artifact(r.Context(), rel.ArtifactID)
258: 	if err != nil {
259: 		httpx.Fail(w, 503, "unavailable", "Access check failed")
260: 		return false
261: 	}
262: 	e, doc, err := s.entitlement(r.Context(), p.User)
263: 	if err != nil {
264: 		httpx.Fail(w, 503, "unavailable", "Access check failed")
265: 		return false
266: 	}
267: 	if !e.CanRun(a, doc) {
268: 		httpx.Fail(w, 403, "plan_required", "Your plan does not include this application")
269: 		return false
270: 	}
271: 	return true
272: }
273: 
274: // download authorizes before opening or writing any bytes; denials are JSON.
275: func (s *Server) download(w http.ResponseWriter, r *http.Request) {
276: 	rel, err := s.Store.Release(r.Context(), r.PathValue("release"))
277: 	if err != nil || rel.State != "published" {
278: 		httpx.Fail(w, 404, "release_unavailable", "Release not available")
279: 		return
280: 	}
281: 	var file *store.ReleaseFile
282: 	for i := range rel.Files {
283: 		if rel.Files[i].ID == r.PathValue("file") {
284: 			file = &rel.Files[i]
285: 		}
286: 	}
287: 	if file == nil {
288: 		httpx.Fail(w, 404, "not_found", "File not found")
289: 		return
290: 	}
291: 	if !s.mayDownload(w, r, rel) {
292: 		return
293: 	}
294: 	data, err := s.Publisher.Blobs.Read(*file)
295: 	if err != nil {
296: 		log.Error().Err(err).Str("file", file.ID).Msg("release file unreadable")
297: 		httpx.Fail(w, 503, "file_unavailable", "File temporarily unavailable")
298: 		return
299: 	}
300: 	w.Header().Set("Content-Type", "application/octet-stream")
301: 	w.Header().Set("Content-Disposition", `attachment; filename="`+safeName.ReplaceAllString(file.Name, "_")+`"`)
302: 	w.Header().Set("Cache-Control", "private, no-store")
```

## pkg/catalog/hosted/runtime.go:113

```text
113: 	status int
114: 	code   string
115: 	msg    string
116: }
117: 
118: func (d *denied) Error() string { return d.msg }
119: 
120: // Status exposes the HTTP mapping for portal handlers.
121: func (d *denied) Status() (int, string, string) { return d.status, d.code, d.msg }
122: 
123: func bearer(r *http.Request) string {
124: 	h := r.Header.Get("Authorization")
125: 	if strings.HasPrefix(h, "Bearer ") {
126: 		return strings.TrimPrefix(h, "Bearer ")
127: 	}
128: 	return ""
129: }
130: 
131: // authenticate resolves a scoped grant into a principal and rechecks
132: // membership, instance state and entitlement on every request.
133: func (rt *Runtime) authenticate(ctx context.Context, token string, inst store.Instance) (Principal, error) {
134: 	if token == "" {
135: 		return Principal{}, &denied{401, "grant_required", "A runtime grant is required"}
136: 	}
137: 	g, err := rt.Store.ResolveGrant(ctx, token, inst.ID)
138: 	if err != nil {
139: 		return Principal{}, &denied{401, "grant_invalid", "The runtime grant is invalid or expired"}
140: 	}
141: 	return rt.principal(ctx, g.UserID, inst.ID, store.Hash(token), g.SessionHash)
142: }
143: 
144: func (rt *Runtime) principal(ctx context.Context, userID, instanceID string, grant, session []byte) (Principal, error) {
145: 	u, err := rt.Store.User(ctx, userID)
146: 	if err != nil {
147: 		return Principal{}, err
148: 	}
149: 	inst, err := rt.Store.MemberInstance(ctx, userID, instanceID)
150: 	if errors.Is(err, store.ErrNotFound) {
151: 		return Principal{}, &denied{403, "not_member", "You are not a member of this instance"}
152: 	}
153: 	if err != nil {
154: 		return Principal{}, err
155: 	}
156: 	if inst.State != "active" {
157: 		return Principal{}, &denied{403, "instance_suspended", "This instance is suspended"}
158: 	}
159: 	if err = rt.Authorize(ctx, u, inst); err != nil {
160: 		return Principal{}, err
161: 	}
162: 	rel, err := rt.Store.Release(ctx, inst.ReleaseID)
163: 	if err != nil {
164: 		return Principal{}, err
165: 	}
166: 	return Principal{User: u, Instance: inst, Role: inst.Role, Release: rel, GrantHash: grant, Session: session}, nil
167: }
168: 
169: // Denied builds an authorization error for the Authorizer implementation.
170: func Denied(status int, code, msg string) error { return &denied{status, code, msg} }
171: 
172: func failErr(w http.ResponseWriter, err error) {
173: 	var paused *store.StoragePaused
174: 	if errors.As(err, &paused) {
175: 		code := "storage_read_only"
176: 		if paused.Mode == "off" {
177: 			code = "storage_unavailable"
178: 		}
179: 		httpx.Fail(w, 503, code, paused.Message)
```

## pkg/catalog/hosted/runtime.go:194

```text
194: }
195: 
196: // available intersects the manifest with capabilities the hosted runtime
197: // implements; files and catalog republishing are not offered.
198: func available(m map[string]json.RawMessage) map[string]json.RawMessage {
199: 	out := map[string]json.RawMessage{}
200: 	for name, decl := range m {
201: 		switch name {
202: 		case "sample", "user", "db", "room", "comments", "assets", "downloads":
203: 			out[name] = decl
204: 		}
205: 	}
206: 	return out
207: }
208: 
209: func (rt *Runtime) declared(p Principal, capability string) bool {
210: 	_, ok := available(p.Release.Manifest)[capability]
211: 	return ok
212: }
213: 
214: // Revocation closes live views (and their calls) matching a predicate.
215: func (rt *Runtime) revoke(match func(*View) bool) int {
```

## pkg/catalog/hosted/rpc.go:46

```text
46: func (e *errCode) Error() string { return e.msg }
47: 
48: func readOnly() error { return &errCode{403, "read_only", "Your role in this instance is read-only"} }
49: 
50: func (rt *Runtime) rpc(w http.ResponseWriter, r *http.Request, inst store.Instance) {
51: 	v, p := rt.view(w, r, inst)
52: 	if v == nil {
53: 		return
54: 	}
55: 	var req rpcRequest
56: 	if err := httpx.Read(w, r, &req, 16<<20); err != nil {
57: 		httpx.Fail(w, 400, "invalid_argument", err.Error())
58: 		return
59: 	}
60: 	capability := strings.Split(req.Method, ".")[0]
61: 	if !rt.declared(p, capability) {
62: 		httpx.Fail(w, 403, "not_declared", "Capability not declared or not available in hosted instances")
63: 		return
64: 	}
65: 	var args rpcArgs
66: 	if err := json.Unmarshal(req.Args, &args); err != nil {
67: 		httpx.Fail(w, 400, "invalid_argument", "Invalid arguments")
68: 		return
69: 	}
70: 	result, err := rt.operation(r.Context(), v, p, req.Method, args)
71: 	if err != nil {
72: 		var paused *store.StoragePaused
73: 		if errors.As(err, &paused) {
74: 			failErr(w, err)
75: 			return
```

## pkg/catalog/hosted/sample.go:211

```text
211: 				usage.Rounds++
212: 				usage.ModelUsed = e.Usage.Model
213: 			}
214: 			return
215: 		}
216: 		if settled {
217: 			return
218: 		}
219: 		if e.Type == "result" {
220: 			settled, ok = true, true
221: 			if e.Result != nil && e.Result.ModelUsed != "" {
222: 				usage.ModelUsed = e.Result.ModelUsed
223: 			}
224: 		}
225: 		if e.Type == "error" {
226: 			settled = true
227: 			if e.Error != nil {
228: 				code = e.Error.Code
229: 			}
230: 		}
231: 		v.emit(e)
232: 	}
233: 	execute := func(ctx context.Context, event piworker.Event) (piworker.ToolResult, error) {
234: 		if !c.names[event.Name] {
235: 			return piworker.ToolResult{}, errors.New("Unknown page tool")
236: 		}
237: 		child, stop := context.WithTimeout(ctx, 150*time.Second)
238: 		defer stop()
239: 		ch := make(chan piworker.ToolResult, 1)
240: 		v.mu.Lock()
241: 		if c.tools[event.ToolCallID] != nil {
242: 			v.mu.Unlock()
243: 			return piworker.ToolResult{}, errors.New("Duplicate tool call")
244: 		}
245: 		c.tools[event.ToolCallID] = ch
246: 		v.mu.Unlock()
247: 		defer func() { v.mu.Lock(); delete(c.tools, event.ToolCallID); v.mu.Unlock() }()
248: 		v.emit(event)
249: 		select {
250: 		case result := <-ch:
251: 			return result, nil
252: 		case <-child.Done():
253: 			return piworker.ToolResult{}, child.Err()
254: 		}
255: 	}
256: 	err := rt.Backend.Sample(ctx, req, emit, execute)
257: 	if !settled {
258: 		code = "upstream_error"
259: 		if ctx.Err() != nil {
260: 			code = "cancelled"
261: 		}
262: 		message := "Model request failed"
263: 		if err != nil && code != "cancelled" {
264: 			message = err.Error()
```

## ui/src/runtime.ts:790

```text
790:   sample,
791:   user,
792:   comments,
793:   db: installDB(transport),
794:   room: installRooms(transport),
795:   ...installFiles(transport, request),
796:   artifact: Object.freeze({
797:     publish: (html: unknown) =>
798:       typeof html === "string"
799:         ? transport.rpc("artifact.publish", { html })
800:         : Promise.reject(
801:             error("capability_removed", "Files-form publishing is not enabled"),
802:           ),
803:   }),
804:   assets: Object.freeze({
805:     async upload(blob: Blob, options: { type?: string } = {}) {
806:       return request("/api/assets", {
807:         method: "POST",
808:         headers: { "Content-Type": options.type ?? blob.type },
809:         body: blob,
810:       });
811:     },
812:     list: () => transport.rpc("assets.list"),
813:     delete: (key: string) =>
814:       transport.rpc("assets.delete", {
815:         id: key.split("?")[0].split("/").at(-1),
816:       }),
817:   }),
818: };
819: Object.defineProperty(window, "claude", {
820:   value: Object.freeze({
821:     use: async (name: string) =>
822:       name === "user" || manifest[name] !== undefined
823:         ? (capabilities[name] ?? null)
824:         : null,
825:   }),
826:   writable: false,
827:   configurable: false,
828: });
```

## pkg/piworker/validate.go:65

```text
65: 		}
66: 		if len(t.InputSchema) > 0 {
67: 			var schema map[string]any
68: 			if json.Unmarshal(t.InputSchema, &schema) != nil || schema["type"] != "object" {
69: 				return errors.New("Tool schema must describe an object")
70: 			}
71: 		}
72: 		names[t.Name] = true
73: 	}
74: 	return nil
75: }
```

## examples/imports/rag-workbench/source/agent.js:474

```text
474:       }
475:       if (mode !== 'readonly') this.snaps.set(part, this.host.snapshot());
476:       const logs = [];
477:       const log = (...a) => logs.push(a.map(x => (typeof x === 'string' ? x : toYAML(x))).join(' '));
478:       const cons = { log, info: log, warn: (...a) => log('warn:', ...a), error: (...a) => log('error:', ...a) };
479:       const chat = this.makeChat(msg);
480:       const scope = this.host.scope({ readonly: mode === 'readonly', chat, log });
481:       const names = ['rag', 'gui', 'wb', 'ws', 'ui', 'chat', 'ref', 'log', 'console'];
482:       let ok = true, out;
483:       try {
484:         const fn = new Function(...names, `"use strict";return (async () => {\n${code}\n})();\n//# sourceURL=agent-eval.js`);
485:         const res = await fn(scope.rag, scope.gui, scope.wb, scope.ws, scope.ui, chat, scope.ref, log, cons);
486:         out = res === undefined ? '# returned undefined' : typeof res === 'string' ? res : toYAML(res);
487:       } catch (e) {
```

## pkg/catalog/releases/releases.go:123

```text
123: func (b *Blobs) Open(key string) (*os.File, error) {
124: 	p, err := b.path(key)
125: 	if err != nil {
126: 		return nil, err
127: 	}
128: 	return os.Open(p) // #nosec G304 -- key is validated against keyPattern
129: }
130: 
131: // Read returns the stored bytes after verifying their recorded hash.
132: func (b *Blobs) Read(f store.ReleaseFile) ([]byte, error) {
133: 	p, err := b.path(f.StorageKey)
134: 	if err != nil {
135: 		return nil, err
136: 	}
137: 	data, err := os.ReadFile(p)
138: 	if err != nil {
139: 		return nil, errors.Wrap(err, "read release file")
140: 	}
141: 	h := sha256.Sum256(data)
142: 	if hex.EncodeToString(h[:]) != f.SHA256 {
143: 		return nil, errors.New("release file hash mismatch")
144: 	}
145: 	return data, nil
146: }
147: 
148: // Missing reports referenced files that are absent or corrupted.
```
