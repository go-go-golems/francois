// Package cmds contains the Glazed application commands.
package cmds

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/francois/pkg/host"
	"github.com/go-go-golems/francois/pkg/piworker"
	"github.com/go-go-golems/francois/pkg/store"
	"github.com/go-go-golems/glazed/pkg/cli"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type Settings struct {
	DataDir     string `glazed:"data-dir"`
	Backend     string `glazed:"backend"`
	Listen      string `glazed:"listen"`
	PiAgentDir  string `glazed:"pi-agent-dir"`
	Worker      string `glazed:"worker"`
	Node        string `glazed:"node"`
	Quick       string `glazed:"model-quick"`
	Default     string `glazed:"model-default"`
	Complex     string `glazed:"model-complex"`
	ProjectRoot string `glazed:"project-root"`
}

func flags() []*fields.Definition {
	return []*fields.Definition{
		fields.New("data-dir", fields.TypeString, fields.WithDefault("./var/francois"), fields.WithHelp("Persistent artifact data directory")),
		fields.New("backend", fields.TypeChoice, fields.WithChoices("pi", "fake"), fields.WithDefault("pi"), fields.WithHelp("Pi models or deterministic offline test backend")),
		fields.New("listen", fields.TypeString, fields.WithDefault("127.0.0.1:8080"), fields.WithHelp("Loopback library address")),
		fields.New("pi-agent-dir", fields.TypeString, fields.WithDefault("~/.pi/agent"), fields.WithHelp("Pi config files; the SDK may also use credential environment fallback")),
		fields.New("worker", fields.TypeString, fields.WithDefault("worker/dist/main.js"), fields.WithHelp("Built Pi worker entrypoint")),
		fields.New("node", fields.TypeString, fields.WithDefault("node"), fields.WithHelp("Node executable")),
		fields.New("model-quick", fields.TypeString, fields.WithHelp("Pi provider/model selector for quick calls")),
		fields.New("model-default", fields.TypeString, fields.WithHelp("Pi provider/model selector for default calls")),
		fields.New("model-complex", fields.TypeString, fields.WithHelp("Pi provider/model selector for complex calls")),
		fields.New("project-root", fields.TypeString, fields.WithHelp("Read-only root for declared project files")),
	}
}
func expand(p string) (string, error) {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	return filepath.Abs(p)
}
func backend(ctx context.Context, s Settings) (piworker.Backend, error) {
	if s.Backend == "fake" {
		return &piworker.Fake{}, nil
	}
	worker, err := expand(s.Worker)
	if err != nil {
		return nil, err
	}
	dir, err := expand(s.PiAgentDir)
	if err != nil {
		return nil, err
	}
	cwd, err := expand(filepath.Join(s.DataDir, "worker"))
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(cwd, 0700); err != nil {
		return nil, err
	}
	return piworker.Start(ctx, s.Node, worker, dir, cwd, map[string]string{"quick": s.Quick, "default": s.Default, "complex": s.Complex})
}

type Serve struct{ *glazed.CommandDescription }

var _ glazed.BareCommand = (*Serve)(nil)

func (s *Serve) Run(ctx context.Context, parsed *values.Values) error {
	var settings Settings
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, &settings); err != nil {
		return err
	}
	db, err := store.Open(ctx, settings.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	b, err := backend(ctx, settings)
	if err != nil {
		return err
	}
	defer func() { _ = b.Close() }()
	for _, m := range b.Models() {
		if m.Tier != "" {
			log.Info().Str("tier", m.Tier).Str("provider", m.Provider).Str("model", m.ID).Msg("Selected model")
		}
	}
	h, err := host.New(ctx, db, b, settings.ProjectRoot)
	if err != nil {
		return err
	}
	return h.Run(settings.Listen)
}

type Import struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*Import)(nil)

func (i *Import) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		DataDir  string `glazed:"data-dir"`
		File     string `glazed:"file"`
		ID       string `glazed:"id"`
		Title    string `glazed:"title"`
		Manifest string `glazed:"manifest"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	data, err := readHTML(s.File)
	if err != nil {
		return err
	}
	manifest := map[string]json.RawMessage{"sample": json.RawMessage(`{}`)}
	if s.Manifest != "" {
		raw, err := os.ReadFile(s.Manifest)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &manifest); err != nil {
			return err
		}
	}
	if s.Title == "" {
		s.Title = s.ID
	}
	db, err := store.Open(ctx, s.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	a, err := db.Import(ctx, s.ID, s.Title, data, manifest)
	if err != nil {
		return err
	}
	return out.AddRow(ctx, types.NewRow(types.MRP("id", a.ID), types.MRP("title", a.Title), types.MRP("version", a.Version)))
}

type Models struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*Models)(nil)

func (m *Models) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s Settings
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	b, err := backend(ctx, s)
	if err != nil {
		return err
	}
	defer func() { _ = b.Close() }()
	for _, model := range b.Models() {
		if err = out.AddRow(ctx, types.NewRow(types.MRP("provider", model.Provider), types.MRP("id", model.ID), types.MRP("name", model.Name), types.MRP("tier", model.Tier), types.MRP("reasoning", model.Reasoning), types.MRP("images", model.Images), types.MRP("thinking_levels", model.ThinkingLevels))); err != nil {
			return err
		}
	}
	return nil
}
func Register(root *cobra.Command) error {
	serve := &Serve{glazed.NewCommandDescription("serve", glazed.WithShort("Serve the library and isolated artifact origins"), glazed.WithFlags(flags()...))}
	imp := &Import{glazed.NewCommandDescription("import", glazed.WithShort("Import an HTML artifact into persistent storage"), glazed.WithFlags(fields.New("file", fields.TypeString, fields.WithRequired(true)), fields.New("id", fields.TypeString, fields.WithRequired(true)), fields.New("title", fields.TypeString), fields.New("manifest", fields.TypeString, fields.WithHelp("JSON capabilities manifest file")), fields.New("data-dir", fields.TypeString, fields.WithDefault("./var/francois"))))}
	update := &Update{glazed.NewCommandDescription("update", glazed.WithShort("Publish a new HTML version of an existing artifact"), glazed.WithLong("Replace HTML while preserving the artifact ID, port, manifest and stored data. Refresh open pages to load the new version. Concurrent publication fails rather than overwriting a newer version."), glazed.WithFlags(fields.New("file", fields.TypeString, fields.WithRequired(true)), fields.New("id", fields.TypeString, fields.WithRequired(true)), fields.New("data-dir", fields.TypeString, fields.WithDefault("./var/francois"))))}
	for _, command := range []glazed.Command{serve, imp, update} {
		c, err := cli.BuildCobraCommandFromCommand(command)
		if err != nil {
			return err
		}
		root.AddCommand(c)
	}
	group := &cobra.Command{Use: "models", Short: "Inspect the configured model connection"}
	command, err := cli.BuildCobraCommandFromCommand(&Models{glazed.NewCommandDescription("list", glazed.WithShort("List authenticated Pi models and tier selections"), glazed.WithFlags(flags()...))})
	if err != nil {
		return err
	}
	group.AddCommand(command)
	root.AddCommand(group)
	return registerCatalog(root)
}
