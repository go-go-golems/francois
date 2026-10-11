package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestVersionsDocumentsAndDurability(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Import(ctx, "test", "Test", []byte("<!doctype html><html><head></head><body>v1</body></html>"), map[string]json.RawMessage{"db": json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, a.ID, 1, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Publish(ctx, a.ID, 1, []byte("loser")); err == nil || err.Error() != "conflict" {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err = s.Write(ctx, a.ID, "tasks/a", "set", map[string]any{"nested": map[string]any{"first": true}, "rank": float64(2)}); err != nil {
		t.Fatal(err)
	}
	if err = s.Write(ctx, a.ID, "tasks/a", "update", map[string]any{"nested": map[string]any{"second": true}}); err != nil {
		t.Fatal(err)
	}
	if err = s.Write(ctx, a.ID, "tasks/missing", "update", map[string]any{}); err == nil {
		t.Fatal("missing update succeeded")
	}
	got, err := s.Document(ctx, a.ID, "tasks/a")
	if err != nil {
		t.Fatal(err)
	}
	nested := got.Body["nested"].(map[string]any)
	if nested["first"] != true || nested["second"] != true {
		t.Fatalf("merge lost values: %v", nested)
	}
	for _, name := range []string{"tasks/b", "tasks/a/children/z"} {
		if err = s.Write(ctx, a.ID, name, "set", map[string]any{"rank": float64(1)}); err != nil {
			t.Fatal(err)
		}
	}
	docs, err := s.Query(ctx, a.ID, Query{Path: "tasks", Order: "rank", Limit: 1})
	if err != nil || len(docs) != 1 || docs[0].ID != "b" {
		t.Fatalf("wrong collection window: %v %v", docs, err)
	}
	ok, err := s.Acquire(ctx, a.ID, "tasks/a", "tab1", 1000, nil)
	if err != nil || !ok {
		t.Fatalf("first acquire: %v %v", ok, err)
	}
	ok, err = s.Acquire(ctx, a.ID, "tasks/a", "tab2", 1000, nil)
	if err != nil || ok {
		t.Fatalf("contending acquire: %v %v", ok, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	got, err = s.Document(ctx, a.ID, "tasks/a")
	if err != nil || !got.Exists {
		t.Fatalf("lost persisted document: %v %v", got, err)
	}
	if err = s.Write(ctx, a.ID, "tasks/a", "delete", nil); err != nil {
		t.Fatal(err)
	}
	child, err := s.Document(ctx, a.ID, "tasks/a/children/z")
	if err != nil || !child.Exists {
		t.Fatal("delete erased subcollection")
	}
}
func TestPathValidation(t *testing.T) {
	for _, path := range []string{"", "tasks", "a//b", "a/b/c", "a/b\\c"} {
		if ValidatePath(path, true) == nil {
			t.Errorf("accepted invalid doc path %q", path)
		}
	}
	if err := ValidatePath("data/users/u_1/profile", true); err != nil {
		t.Fatal(err)
	}
}
