package cmds

import (
	"context"
	"os"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/studioclient"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// CatalogPush uploads a release to a hosted catalog through the studio API
// with a personal API token, so creators can publish without database
// access.
type CatalogPush struct{ *glazed.CommandDescription }

var _ glazed.GlazeCommand = (*CatalogPush)(nil)

func newCatalogPush() *CatalogPush {
	return &CatalogPush{glazed.NewCommandDescription("push",
		glazed.WithShort("Upload a release to a hosted catalog with a personal API token"),
		glazed.WithLong("Creates a draft release through the studio API (the same checks as the browser upload) and, with --publish, publishes it. "+
			"Create a token in the studio (Studio → Tokens) and pass it with --token-file or FRANCOIS_TOKEN."),
		glazed.WithFlags(
			fields.New("server", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Portal origin, e.g. https://slop.boutique")),
			fields.New("token-file", fields.TypeString, fields.WithHelp("File containing the API token (default: $FRANCOIS_TOKEN)")),
			fields.New("artifact-id", fields.TypeString, fields.WithRequired(true)),
			fields.New("version", fields.TypeString, fields.WithRequired(true)),
			fields.New("html", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Authored HTML file")),
			fields.New("manifest", fields.TypeString, fields.WithHelp("Capability manifest JSON file")),
			fields.New("title", fields.TypeString, fields.WithHelp("Title, for a new artifact")),
			fields.New("description", fields.TypeString, fields.WithHelp("Short description, for a new artifact")),
			fields.New("tags", fields.TypeStringList, fields.WithHelp("Tags, for a new artifact")),
			fields.New("changelog", fields.TypeString),
			fields.New("free-sample", fields.TypeBool, fields.WithDefault(false)),
			fields.New("check", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Only validate; store nothing")),
			fields.New("publish", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Publish the new draft")),
		))}
}

func (c *CatalogPush) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		Server      string   `glazed:"server"`
		TokenFile   string   `glazed:"token-file"`
		ArtifactID  string   `glazed:"artifact-id"`
		Version     string   `glazed:"version"`
		HTML        string   `glazed:"html"`
		Manifest    string   `glazed:"manifest"`
		Title       string   `glazed:"title"`
		Description string   `glazed:"description"`
		Tags        []string `glazed:"tags"`
		Changelog   string   `glazed:"changelog"`
		Free        bool     `glazed:"free-sample"`
		Check       bool     `glazed:"check"`
		Publish     bool     `glazed:"publish"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	token := os.Getenv("FRANCOIS_TOKEN")
	if s.TokenFile != "" {
		raw, err := os.ReadFile(s.TokenFile)
		if err != nil {
			return errors.Wrap(err, "read token file")
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		return errors.New("an API token is required (--token-file or FRANCOIS_TOKEN)")
	}
	html, err := readHTML(s.HTML)
	if err != nil {
		return err
	}
	var manifest []byte
	if s.Manifest != "" {
		if manifest, err = os.ReadFile(s.Manifest); err != nil {
			return err
		}
	}
	rel := studioclient.Release{ArtifactID: s.ArtifactID, Version: s.Version, Title: s.Title, Description: s.Description, Tags: s.Tags,
		Changelog: s.Changelog, HTML: html, Manifest: manifest, FreeSample: s.Free}
	client := studioclient.New(s.Server, token)
	report := func(checks []studioclient.Check) {
		for _, ch := range checks {
			log.Info().Str("level", ch.Level).Str("code", ch.Code).Msg(ch.Message)
		}
	}
	if s.Check {
		checks, err := client.Check(ctx, rel)
		report(checks)
		return err
	}
	res, err := client.Push(ctx, rel, s.Publish)
	report(res.Checks)
	var apiErr *studioclient.Error
	if errors.As(err, &apiErr) {
		report(apiErr.Checks)
	}
	if err != nil {
		return err
	}
	return out.AddRow(ctx, types.NewRow(types.MRP("release_id", res.ReleaseID), types.MRP("artifact", s.ArtifactID), types.MRP("version", s.Version), types.MRP("state", res.State)))
}
