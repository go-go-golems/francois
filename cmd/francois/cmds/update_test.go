package cmds

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/store"
	"github.com/spf13/cobra"
)

func executeJSON(t *testing.T, args ...string) ([]map[string]any, error) {
	t.Helper()
	root := &cobra.Command{Use: "francois", SilenceUsage: true, SilenceErrors: true}
	if err := Register(root); err != nil {
		t.Fatal(err)
	}
	root.SetArgs(args)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original; _ = reader.Close(); _ = writer.Close() }()
	err = root.Execute()
	_ = writer.Close()
	os.Stdout = original
	data, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode output %q: %v", data, err)
	}
	return rows, nil
}
func TestUpdatePublishesHTMLAndPreservesArtifact(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	oldHTML := []byte("<html><body>original</body></html>")
	manifest := map[string]json.RawMessage{"sample": json.RawMessage(`{}`), "db": json.RawMessage(`{}`)}
	if _, err = db.Import(ctx, "polis", "Polis IDE", oldHTML, manifest); err != nil {
		t.Fatal(err)
	}
	if err = db.SetPort(ctx, "polis", 33921); err != nil {
		t.Fatal(err)
	}
	if err = db.Write(ctx, "polis", "tasks/one", "set", map[string]any{"title": "keep me"}); err != nil {
		t.Fatal(err)
	}
	owner, err := db.Identity(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "updated.html")
	newHTML := []byte("<html><body>updated</body></html>")
	if err = os.WriteFile(file, newHTML, 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := executeJSON(t, "update", "--id", "polis", "--file", file, "--data-dir", dir, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["version"] != float64(2) || rows[0]["previous_version"] != float64(1) || rows[0]["id"] != "polis" {
		t.Fatalf("unexpected rows: %v", rows)
	}
	a, err := db.Get(ctx, "polis")
	if err != nil {
		t.Fatal(err)
	}
	if a.Version != 2 || a.Port != 33921 || a.Title != "Polis IDE" || !reflect.DeepEqual(a.Manifest, manifest) {
		t.Fatalf("artifact changed: %+v", a)
	}
	for v, want := range map[int64][]byte{1: oldHTML, 2: newHTML} {
		got, err := db.HTML(ctx, "polis", v)
		if err != nil || string(got) != string(want) {
			t.Fatalf("version %d: %q %v", v, got, err)
		}
	}
	doc, err := db.Document(ctx, "polis", "tasks/one")
	if err != nil || doc.Body["title"] != "keep me" {
		t.Fatalf("document lost: %+v %v", doc, err)
	}
	after, err := db.Identity(ctx, "new-owner")
	if err != nil || after != owner {
		t.Fatal("owner changed", err)
	}
	if _, err = executeJSON(t, "update", "--id", "missing", "--file", file, "--data-dir", dir, "--format", "json"); err == nil || !strings.Contains(err.Error(), "use import") {
		t.Fatalf("missing artifact error: %v", err)
	}
	if err = os.WriteFile(file, []byte("not an HTML document"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = executeJSON(t, "update", "--id", "polis", "--file", file, "--data-dir", dir, "--format", "json"); err == nil {
		t.Fatal("invalid HTML accepted")
	}
	a, err = db.Get(ctx, "polis")
	if err != nil || a.Version != 2 {
		t.Fatal("failed update changed version", err)
	}
	root := &cobra.Command{Use: "francois"}
	if err = Register(root); err != nil {
		t.Fatal(err)
	}
	command, _, err := root.Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"file", "id", "data-dir", "format", "output-fields", "max-output-rows"} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("missing flag %s", name)
		}
	}
}
