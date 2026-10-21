package cmds

import (
	"context"
	"os"

	"github.com/go-go-golems/francois/pkg/catalog/client"
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
			"Needs a token with the studio scope: run `francois catalog auth login`, or create one on the account page (/account → API tokens) and pass it with --token-file or FRANCOIS_TOKEN."),
		glazed.WithFlags(append([]*fields.Definition{
			fields.New("artifact-id", fields.TypeString, fields.WithRequired(true)),
			fields.New("version", fields.TypeString, fields.WithRequired(true)),
			fields.New("html", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Authored HTML file")),
			fields.New("manifest", fields.TypeString, fields.WithHelp("Capability manifest JSON file")),
			fields.New("title", fields.TypeString, fields.WithHelp("Title, for a new artifact")),
			fields.New("description", fields.TypeString, fields.WithHelp("Short description, for a new artifact")),
			fields.New("tags", fields.TypeStringList, fields.WithHelp("Tags, for a new artifact")),
			fields.New("changelog", fields.TypeString),
			fields.New("free-sample", fields.TypeBool, fields.WithDefault(false)),
			fields.New("attach", fields.TypeStringList, fields.WithHelp("Files to attach to the release (manuals, archives, notes); repeatable")),
			fields.New("check", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Only validate; store nothing")),
			fields.New("publish", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Publish the new draft")),
		}, connectionFlags()...)...))}
}

func (c *CatalogPush) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s struct {
		ArtifactID  string   `glazed:"artifact-id"`
		Version     string   `glazed:"version"`
		HTML        string   `glazed:"html"`
		Manifest    string   `glazed:"manifest"`
		Title       string   `glazed:"title"`
		Description string   `glazed:"description"`
		Tags        []string `glazed:"tags"`
		Changelog   string   `glazed:"changelog"`
		Free        bool     `glazed:"free-sample"`
		Attach      []string `glazed:"attach"`
		Check       bool     `glazed:"check"`
		Publish     bool     `glazed:"publish"`
	}
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	api, err := connect(in{p})
	if err != nil {
		return err
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
	rel := client.ReleaseUpload{ArtifactID: s.ArtifactID, Version: s.Version, Title: s.Title, Description: s.Description, Tags: s.Tags,
		Changelog: s.Changelog, HTML: html, Manifest: manifest, FreeSample: s.Free}
	studio := api.Studio
	report := func(checks []client.Check) {
		for _, ch := range checks {
			log.Info().Str("level", ch.Level).Str("code", ch.Code).Msg(ch.Message)
		}
	}
	if s.Check {
		res, err := studio.CheckRelease(ctx, rel)
		report(res.Checks)
		return err
	}
	res, err := studio.Push(ctx, rel, s.Attach, s.Publish)
	report(res.Release.Checks)
	var apiErr *client.Error
	if errors.As(err, &apiErr) {
		report(apiErr.Checks)
	}
	if err != nil {
		return loginHint(err)
	}
	return out.AddRow(ctx, types.NewRow(types.MRP("release_id", res.Release.ID), types.MRP("artifact", s.ArtifactID), types.MRP("version", s.Version), types.MRP("state", res.Release.State),
		types.MRP("attachments", len(res.Attachments))))
}
