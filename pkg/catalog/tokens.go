package catalog

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

// maxAdminTokenDays bounds the lifetime of tokens that reach the admin API.
const maxAdminTokenDays = 90

func (s *Server) tokenRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/me", s.guard(session, anyToken, s.apiMe))
	// Tokens are managed from a browser session only: a token cannot mint or
	// revoke tokens.
	m.HandleFunc("GET /api/tokens", s.guard(session, noToken, s.tokens))
	m.HandleFunc("POST /api/tokens", s.guard(enabled, noToken, s.createToken))
	m.HandleFunc("DELETE /api/tokens/{id}", s.guard(session, noToken, s.revokeToken))
}

// apiMe tells a client who it is acting as and, for a token, what the token
// may reach. Any valid token may call it.
func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	p := principal(r.Context())
	u := p.User
	out := map[string]any{
		"user":    map[string]any{"id": u.ID, "name": u.DisplayName, "email": u.Email, "role": u.Role, "disabled": u.Disabled},
		"token":   p.Token,
		"creator": nil,
	}
	if c, ok, err := s.ownProfile(r.Context(), u); err != nil {
		unavailable(w, err)
		return
	} else if ok {
		out["creator"] = c
	}
	httpx.OK(w, out)
}

func (s *Server) tokens(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Tokens(r.Context(), actor(r))
	if err != nil {
		unavailable(w, err)
		return
	}
	httpx.OK(w, list)
}

// createToken issues a personal API token. The scopes must be ones the
// caller's role may hold; tokens that reach the admin API must expire within
// maxAdminTokenDays.
func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name   string   `json:"name"`
		Days   int      `json:"days"`
		Scopes []string `json:"scopes"`
	}
	if err := httpx.Read(w, r, &in, 2048); err != nil || strings.TrimSpace(in.Name) == "" || len(in.Name) > 80 || in.Days < 0 || in.Days > 366 {
		httpx.Fail(w, 400, "invalid_request", "A name (≤80) and a lifetime of 0–366 days (0 = no expiry) are required")
		return
	}
	allowed := store.ScopesForRole(me(r).Role)
	if len(allowed) == 0 {
		httpx.Fail(w, 403, "tokens_unavailable", "This account cannot create API tokens")
		return
	}
	var scopes []string
	admin := false
	for _, sc := range in.Scopes {
		if !slices.Contains(allowed, sc) {
			httpx.Fail(w, 400, "invalid_scopes", "Scopes must be chosen from: "+strings.Join(allowed, ", "))
			return
		}
		if !slices.Contains(scopes, sc) {
			scopes = append(scopes, sc)
		}
		admin = admin || store.IsAdminScope(sc)
	}
	if len(scopes) == 0 {
		httpx.Fail(w, 400, "invalid_scopes", "Choose at least one scope from: "+strings.Join(allowed, ", "))
		return
	}
	if admin && (in.Days < 1 || in.Days > maxAdminTokenDays) {
		httpx.Fail(w, 400, "invalid_request", "Tokens with an admin scope must expire within 1–90 days")
		return
	}
	var expires *time.Time
	if in.Days > 0 {
		t := time.Now().Add(time.Duration(in.Days) * 24 * time.Hour)
		expires = &t
	}
	secret, t, err := s.Store.CreateToken(r.Context(), actor(r), strings.TrimSpace(in.Name), scopes, expires)
	if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "token.create", t.ID, map[string]any{"name": t.Name, "scopes": t.Scopes, "expiresAt": t.ExpiresAt})
	httpx.JSON(w, 201, map[string]any{"token": secret, "info": t})
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.RevokeToken(r.Context(), actor(r), r.PathValue("id")); err != nil {
		httpx.Fail(w, 404, "not_found", "Token not found")
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "token.revoke", r.PathValue("id"), nil)
	httpx.OK(w, map[string]bool{"revoked": true})
}
