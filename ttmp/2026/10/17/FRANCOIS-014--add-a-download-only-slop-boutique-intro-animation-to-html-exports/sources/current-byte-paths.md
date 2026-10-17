# Current download and hosted HTML paths


Inspected repository revision: `1bd0682925e4e890bbb40ecbca35f54862edca04`. Read-only source excerpts; no user data or runtime bootstrap values.


## pkg/catalog/routes.go

```go

func (s *Server) download(w http.ResponseWriter, r *http.Request) {
	rel, err := s.Store.Release(r.Context(), r.PathValue("release"))
	if err != nil || rel.State != "published" {
		httpx.Fail(w, 404, "release_unavailable", "Release not available")
		return
	}
	var file *store.ReleaseFile
	for i := range rel.Files {
		if rel.Files[i].ID == r.PathValue("file") {
			file = &rel.Files[i]
		}
	}
	if file == nil {
		httpx.Fail(w, 404, "not_found", "File not found")
		return
	}
	if !s.mayDownload(w, r, rel) {
		return
	}
	data, err := s.Publisher.Blobs.Read(*file)
	if err != nil {
		log.Error().Err(err).Str("file", file.ID).Msg("release file unreadable")
		httpx.Fail(w, 503, "file_unavailable", "File temporarily unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeName.ReplaceAllString(file.Name, "_")+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("X-Content-SHA256", file.SHA256)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

```

## pkg/catalog/hosted/routes.go

```go

func (rt *Runtime) page(w http.ResponseWriter, r *http.Request, inst store.Instance, portal, origin string) {
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
		"grant": token, "grantExpiresAt": g.ExpiresAt, "portalOrigin": portal, "origin": origin,
		"instance": map[string]any{"id": inst.ID, "title": p.Instance.Title, "role": p.Role},
	})
	// Escape "<" so JSON cannot terminate the script element.
	boot = bytes.ReplaceAll(boot, []byte("<"), []byte(`\u003c`))
	injected := append([]byte(`<script type="application/json" id="francois-view">`), boot...)
	injected = append(injected, []byte(`</script><script src="`+html.EscapeString(catalogweb.AppURL("runtime.js"))+`"></script>`)...)
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

```
