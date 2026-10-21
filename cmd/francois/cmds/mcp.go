package cmds

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/mcpserver"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/go-go-mcp/pkg/embeddable"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// MCP runs the catalog's MCP server over stdio for a local agent (Claude
// Code). Tools are filtered by the token's scopes before registration.
type MCP struct{ *glazed.CommandDescription }

var _ glazed.BareCommand = (*MCP)(nil)

func (m *MCP) Run(ctx context.Context, v *values.Values) error {
	a := in{v}
	c, err := connect(a)
	if err != nil {
		return err
	}
	me, err := c.Me(ctx)
	if err != nil {
		return errors.Wrap(loginHint(err), "check the token")
	}
	if me.Token == nil {
		return errors.New("the MCP server needs a personal API token")
	}
	s := mcpserver.Settings{Mode: mcpserver.Local, Scopes: me.Token.Scopes, Toolsets: a.L("toolsets"), ReadOnly: a.B("read-only"), AllowDirs: a.L("allow-dir")}
	if a.B("list-tools") {
		for _, n := range mcpserver.ToolNames(s) {
			fmt.Println(n)
		}
		return nil
	}
	opts, err := mcpserver.Options(func(context.Context) (*client.Client, error) { return c, nil }, s)
	if err != nil {
		return err
	}
	cfg := embeddable.NewServerConfig()
	for _, o := range append([]embeddable.ServerOption{
		embeddable.WithName("francois"),
		embeddable.WithVersion("dev"),
		embeddable.WithServerDescription("Manage the hosted catalog at " + c.Server() + " as " + me.User.Name),
		embeddable.WithDefaultTransport("stdio"),
	}, opts...) {
		if err := o(cfg); err != nil {
			return err
		}
	}
	backend, err := embeddable.NewBackend(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "francois mcp: %d tools for %s (%s) on %s\n", len(mcpserver.ToolNames(s)), me.User.Name, me.User.Role, c.Server())
	return backend.Start(ctx)
}

func mcpCommand() (*cobra.Command, error) {
	cmd := &MCP{glazed.NewCommandDescription("mcp",
		glazed.WithShort("Run an MCP server over stdio for a local agent such as Claude Code"),
		glazed.WithLong("Offers the catalog's API as MCP tools, filtered by the token's scopes. Every tool calls the HTTP API, so the server's permission checks apply. "+
			"Upload tools read files only under --allow-dir (default: the working directory). See `francois help catalog-mcp`."),
		glazed.WithFlags(append([]*fields.Definition{
			list("toolsets", "Limit to studio, customers and/or account"),
			boolean("read-only", "Offer only read-only tools"),
			list("allow-dir", "Directories upload tools may read (default: the working directory)"),
			boolean("list-tools", "Print the tools that would be offered and exit"),
		}, connectionFlags()...)...),
	)}
	return cli.BuildCobraCommandFromCommand(cmd)
}
