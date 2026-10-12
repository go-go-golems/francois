package cmds

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/go-go-golems/francois/pkg/store"
	glazed "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"
	"github.com/pkg/errors"
)

// readHTML shares the complete-document and size rules between import and update.
func readHTML(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "open HTML")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if err != nil {
		return nil, errors.Wrap(err, "read HTML")
	}
	if len(data) == 0 || len(data) > 16<<20 {
		return nil, errors.New("HTML must be between 1 byte and 16 MiB")
	}
	if !strings.Contains(strings.ToLower(string(data)), "<html") {
		return nil, errors.New("requires a complete HTML document")
	}
	return data, nil
}

type Update struct{ *glazed.CommandDescription }
type UpdateSettings struct {
	DataDir string `glazed:"data-dir"`
	File    string `glazed:"file"`
	ID      string `glazed:"id"`
}

var _ glazed.GlazeCommand = (*Update)(nil)

func (u *Update) RunIntoGlazeProcessor(ctx context.Context, p *values.Values, out middlewares.Processor) error {
	var s UpdateSettings
	if err := p.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		return err
	}
	data, err := readHTML(s.File)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, s.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	a, err := db.Get(ctx, s.ID)
	if err != nil {
		return errors.Wrapf(err, "find artifact %q (use import for a new artifact)", s.ID)
	}
	version, err := db.Publish(ctx, a.ID, a.Version, data)
	if err != nil {
		return errors.Wrap(err, "publish HTML update; a concurrent update may have won")
	}
	return out.AddRow(ctx, types.NewRow(types.MRP("id", a.ID), types.MRP("title", a.Title), types.MRP("previous_version", a.Version), types.MRP("version", version)))
}
