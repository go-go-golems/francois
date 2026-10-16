// Package mcpserver exposes the hosted catalog's API as MCP tools, built on
// go-go-mcp's embeddable server. Every tool calls pkg/catalog/client, so the
// server's guard, role and object checks apply to each call; the MCP layer
// only chooses which tools to offer.
package mcpserver

import (
	"context"
	"slices"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/go-go-mcp/pkg/embeddable"
	"github.com/pkg/errors"
)

// Mode selects how file inputs work.
type Mode int

const (
	// Local runs next to the operator (stdio): upload tools read files under
	// the allowed directories.
	Local Mode = iota
	// Remote runs inside catalog serve: uploads take inline content only.
	Remote
)

// Toolsets group tools; an empty selection means all the token's scopes allow.
const (
	ToolsetStudio    = "studio"
	ToolsetCustomers = "customers"
	ToolsetAccount   = "account"
)

// ClientFunc returns the API client for a tool call. Local mode returns one
// client; remote mode builds one from the caller's token.
type ClientFunc func(ctx context.Context) (*client.Client, error)

// Settings decide which tools are registered.
type Settings struct {
	Mode Mode
	// Scopes of the token (local mode). Tools needing another scope are not
	// registered. Nil registers every tool (remote mode, which instead
	// attaches a per-tool scope policy).
	Scopes    []string
	Toolsets  []string
	ReadOnly  bool
	AllowDirs []string // local mode; default: the working directory
}

type env struct {
	clients  ClientFunc
	settings Settings
}

func (e *env) client(ctx context.Context) (*client.Client, error) { return e.clients(ctx) }

// Options returns the go-go-mcp server options for the selected tools.
func Options(clients ClientFunc, s Settings) ([]embeddable.ServerOption, error) {
	if clients == nil {
		return nil, errors.New("a client function is required")
	}
	e := &env{clients: clients, settings: s}
	var opts []embeddable.ServerOption
	for _, t := range catalogue() {
		if !s.includes(t) {
			continue
		}
		opts = append(opts, t.option(e))
		if s.Mode == Remote {
			policy := embeddable.ToolAuthorizationPolicy{Public: t.scope == ""}
			if t.scope != "" {
				policy.RequiredScopes = []string{t.scope}
			}
			opts = append(opts, embeddable.WithToolAuthorization(t.name, policy))
		}
	}
	if len(opts) == 0 {
		return nil, errors.New("no tools match the token's scopes and the selected toolsets")
	}
	return opts, nil
}

// ToolNames lists the tools Options would register, for listings and tests.
func ToolNames(s Settings) []string {
	var names []string
	for _, t := range catalogue() {
		if s.includes(t) {
			names = append(names, t.name)
		}
	}
	return names
}

func (s Settings) includes(t spec) bool {
	if s.ReadOnly && !t.readOnly {
		return false
	}
	if len(s.Toolsets) > 0 && t.toolset != "" && !slices.Contains(s.Toolsets, t.toolset) {
		return false
	}
	if s.Scopes != nil && t.scope != "" && !allows(s.Scopes, t.scope) {
		return false
	}
	if s.Mode == Remote && t.localOnly {
		return false
	}
	return true
}

func allows(scopes []string, need string) bool {
	return slices.Contains(scopes, need) || (need == "admin:read" && slices.Contains(scopes, "admin:write"))
}
