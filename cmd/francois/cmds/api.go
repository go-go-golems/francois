package cmds

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// API commands talk to a hosted catalog over HTTP with a personal API token,
// never to its database. They share the connection flags below and print
// their results through Glazed (--output json|yaml|csv|table, --fields).

func connectionFlags() []*fields.Definition {
	return []*fields.Definition{
		fields.New("server", fields.TypeString, fields.WithHelp("Portal origin, e.g. https://slop.boutique (default: $FRANCOIS_SERVER)")),
		fields.New("token-file", fields.TypeString, fields.WithHelp("File containing the API token (default: $FRANCOIS_TOKEN holds the token itself)")),
		fields.New("host", fields.TypeString, fields.WithHelp("Host header override, for tunnels")),
	}
}

// in reads a command's parsed flags and arguments by name.
type in struct{ v *values.Values }

func (a in) raw(name string) any {
	f, ok := a.v.GetField(schema.DefaultSlug, name)
	if !ok || f == nil {
		return nil
	}
	return f.Value
}

func (a in) S(name string) string { s, _ := a.raw(name).(string); return s }
func (a in) B(name string) bool   { b, _ := a.raw(name).(bool); return b }
func (a in) I(name string) int {
	switch n := a.raw(name).(type) {
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
func (a in) L(name string) []string { l, _ := a.raw(name).([]string); return l }

// optS is nil when a string flag is empty: "leave unchanged" in patches.
func (a in) optS(name string) *string {
	if s := a.S(name); s != "" {
		return &s
	}
	return nil
}

// optL is nil when a list flag is not given ("leave unchanged"); clearFlag,
// when set, yields an explicit empty list.
func (a in) optL(name, clearFlag string) *[]string {
	if clearFlag != "" && a.B(clearFlag) {
		return &[]string{}
	}
	if l := a.L(name); len(l) > 0 {
		return &l
	}
	return nil
}

// optB parses a tri-state flag ("", "true", "false").
func (a in) optB(name string) (*bool, error) {
	s := a.S(name)
	if s == "" {
		return nil, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return nil, errors.Errorf("--%s must be true or false", name)
	}
	return &b, nil
}

// optI parses an optional integer flag given as a string.
func (a in) optI(name string) (*int, error) {
	s := a.S(name)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, errors.Errorf("--%s must be a number", name)
	}
	return &n, nil
}

// apiCommand is a Glazed command that calls the API and prints the result.
type apiCommand struct {
	*glazed.CommandDescription
	run         func(ctx context.Context, c *client.Client, a in) (any, error)
	destructive bool
}

var _ glazed.GlazeCommand = (*apiCommand)(nil)

func (cmd *apiCommand) RunIntoGlazeProcessor(ctx context.Context, v *values.Values, out middlewares.Processor) error {
	a := in{v}
	if cmd.destructive && !a.B("yes") {
		if err := confirm(cmd.Name); err != nil {
			return err
		}
	}
	c, err := connect(a)
	if err != nil {
		return err
	}
	res, err := cmd.run(ctx, c, a)
	if err != nil {
		return err
	}
	return emit(ctx, out, res)
}

// connect builds a client from the connection flags and the environment.
func connect(a in) (*client.Client, error) {
	server := a.S("server")
	if server == "" {
		server = os.Getenv("FRANCOIS_SERVER")
	}
	if server == "" {
		return nil, errors.New("set --server or FRANCOIS_SERVER to the portal origin, e.g. https://slop.boutique")
	}
	token := os.Getenv("FRANCOIS_TOKEN")
	if f := a.S("token-file"); f != "" {
		raw, err := os.ReadFile(f) // #nosec G304 -- the operator names the token file
		if err != nil {
			return nil, errors.Wrap(err, "read token file")
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		return nil, errors.New("an API token is required (--token-file or FRANCOIS_TOKEN); create one on the account page")
	}
	var opts []client.Option
	if h := a.S("host"); h != "" {
		opts = append(opts, client.WithHost(h))
	}
	return client.New(server, token, opts...), nil
}

// confirm asks on a terminal; elsewhere the caller must pass --yes.
func confirm(name string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) { // #nosec G115 -- a file descriptor fits in int
		return errors.Errorf("%s changes or removes data; pass --yes to confirm", name)
	}
	fmt.Fprintf(os.Stderr, "%s changes or removes data. Continue? [y/N] ", name)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		return errors.New("cancelled")
	}
	return nil
}

// emit prints a result: one row per list element, one row for an object, and
// {"ok": true} for calls without a body.
func emit(ctx context.Context, out middlewares.Processor, res any) error {
	if res == nil {
		return out.AddRow(ctx, types.NewRow(types.MRP("ok", true)))
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	var list []map[string]any
	if json.Unmarshal(raw, &list) == nil {
		for _, m := range list {
			if err := out.AddRow(ctx, types.NewRowFromMap(m)); err != nil {
				return err
			}
		}
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		return out.AddRow(ctx, types.NewRowFromMap(obj))
	}
	return out.AddRow(ctx, types.NewRow(types.MRP("value", res)))
}

// def describes one API command.
type def struct {
	name, short, long string
	flags             []*fields.Definition
	destructive       bool
	run               func(ctx context.Context, c *client.Client, a in) (any, error)
}

func arg(name, help string) *fields.Definition {
	return fields.New(name, fields.TypeString, fields.WithIsArgument(true), fields.WithRequired(true), fields.WithHelp(help))
}

func flag(name, help string) *fields.Definition {
	return fields.New(name, fields.TypeString, fields.WithHelp(help))
}

func required(name, help string) *fields.Definition {
	return fields.New(name, fields.TypeString, fields.WithRequired(true), fields.WithHelp(help))
}

func list(name, help string) *fields.Definition {
	return fields.New(name, fields.TypeStringList, fields.WithHelp(help))
}

func boolean(name, help string) *fields.Definition {
	return fields.New(name, fields.TypeBool, fields.WithDefault(false), fields.WithHelp(help))
}

func integer(name string, def int, help string) *fields.Definition {
	return fields.New(name, fields.TypeInteger, fields.WithDefault(def), fields.WithHelp(help))
}

// fieldsDef shortens the field definition type in command tables.
type fieldsDef = fields.Definition

// single builds one API command.
func single(d def) (*cobra.Command, error) {
	flags := append(append([]*fields.Definition{}, d.flags...), connectionFlags()...)
	if d.destructive {
		flags = append(flags, boolean("yes", "Do not ask for confirmation"))
	}
	opts := []glazed.CommandDescriptionOption{glazed.WithShort(d.short), glazed.WithFlags(flags...)}
	if d.long != "" {
		opts = append(opts, glazed.WithLong(d.long))
	}
	c, err := cli.BuildCobraCommandFromCommand(&apiCommand{CommandDescription: glazed.NewCommandDescription(d.name, opts...), run: d.run, destructive: d.destructive})
	return c, errors.Wrap(err, d.name)
}

// group builds a cobra group of API commands.
func group(use, short string, defs ...def) (*cobra.Command, error) {
	g := &cobra.Command{Use: use, Short: short}
	for _, d := range defs {
		c, err := single(d)
		if err != nil {
			return nil, errors.Wrap(err, use)
		}
		g.AddCommand(c)
	}
	return g, nil
}
