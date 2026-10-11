package cmds

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"io"
	"os"
	"testing"
)

func TestModelStructuredOutputAndServeFlags(t *testing.T) {
	root := &cobra.Command{Use: "francois"}
	if err := Register(root); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = old; _ = read.Close(); _ = write.Close() }()
	root.SetArgs([]string{"models", "list", "--backend", "fake", "--format", "json", "--output-fields", "id,provider"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	_ = write.Close()
	os.Stdout = old
	data, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err, out.String())
	}
	if len(rows) != 1 || rows[0]["id"] != "fake" {
		t.Fatalf("unexpected rows %v", rows)
	}
	serve, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"listen", "worker", "pi-agent-dir", "model-quick", "project-root"} {
		if serve.Flags().Lookup(flag) == nil {
			t.Fatalf("missing %s", flag)
		}
	}
}
