package catalog

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/mcpserver"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/go-go-golems/go-go-mcp/pkg/embeddable"
	"github.com/pkg/errors"
)

// mountMCP serves the catalog's MCP tools at /mcp (streamable HTTP,
// stateless) for remote agents that hold a personal API token. Each tool call
// goes through an API client bound to the caller's token that calls this
// server in-process, so guard, roles, object checks and auditing apply
// exactly as for any other API request.
func (s *Server) mountMCP(m *http.ServeMux) error {
	tools, err := mcpserver.Options(s.mcpClient, mcpserver.Settings{Mode: mcpserver.Remote})
	if err != nil {
		return err
	}
	cfg := embeddable.NewServerConfig()
	for _, o := range append([]embeddable.ServerOption{
		embeddable.WithName("francois"),
		embeddable.WithDefaultTransport("streamable_http"),
		embeddable.WithHTTPAuthVerifier(&tokenVerifier{s: s, resource: s.Config.Sites[0].PortalOrigin() + "/mcp"}),
	}, tools...) {
		if err := o(cfg); err != nil {
			return err
		}
	}
	return embeddable.MountHTTPHandlers(m, cfg)
}

// mcpClient builds the API client for one tool call from the verified token
// and the site the request came to.
func (s *Server) mcpClient(ctx context.Context) (*client.Client, error) {
	p, ok := embeddable.GetAuthPrincipal(ctx)
	token, _ := p.Claims["token"].(string)
	if !ok || token == "" {
		return nil, errors.New("no verified token")
	}
	origin := s.Config.Sites[0].PortalOrigin()
	if site, ok := ctx.Value(siteKey{}).(*Site); ok {
		origin = site.PortalOrigin()
	}
	return client.New(origin, token, client.WithHandler(s.Handler())), nil
}

// tokenVerifier authenticates /mcp with personal API tokens.
type tokenVerifier struct {
	s        *Server
	resource string
}

var _ embeddable.HTTPAuthVerifier = (*tokenVerifier)(nil)

func (v *tokenVerifier) ValidateBearerToken(ctx context.Context, token string) (embeddable.AuthPrincipal, error) {
	t, u, err := v.s.Store.ResolveToken(ctx, token)
	if err != nil {
		return embeddable.AuthPrincipal{}, errors.New("invalid, expired or revoked token")
	}
	// admin:write implies admin:read, as in guard; go-go-mcp's per-tool
	// policies compare scope names literally.
	scopes := slices.Clone(t.Scopes)
	if slices.Contains(scopes, store.ScopeAdminWrite) && !slices.Contains(scopes, store.ScopeAdminRead) {
		scopes = append(scopes, store.ScopeAdminRead)
	}
	set, err := embeddable.ParseScopeSet(scopes)
	if err != nil {
		return embeddable.AuthPrincipal{}, err
	}
	// The middleware requires an expiration. A token without one is checked
	// again on every request, so one minute only states it was valid now.
	exp := time.Now().Add(time.Minute)
	if t.ExpiresAt != nil {
		exp = *t.ExpiresAt
	}
	// The raw token stays in process memory: tool calls replay it to the
	// in-process API.
	return embeddable.AuthPrincipal{Subject: u.ID, ClientID: t.ID, Scopes: set, Email: u.Email, DisplayName: u.DisplayName, Expiration: exp,
		Claims: map[string]any{"token": token}}, nil
}

func (v *tokenVerifier) ProtectedResourceMetadata() map[string]any {
	return map[string]any{"resource": v.resource}
}

func (v *tokenVerifier) WWWAuthenticateHeader() string {
	return embeddable.BearerChallenge(embeddable.ProtectedResourceMetadataURL(v.resource))
}
