package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPathAllowDirs(t *testing.T) {
	base := t.TempDir()
	uploads := filepath.Join(base, "uploads")
	if err := os.Mkdir(uploads, 0o700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(uploads, "app.html")
	outside := filepath.Join(base, "secret.txt")
	for _, f := range []string{inside, outside} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(uploads, "link.txt")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)

	for _, dirs := range [][]string{{"uploads"}, {"./uploads/"}, {uploads}} {
		e := &env{settings: Settings{Mode: Local, AllowDirs: dirs}}
		if got, err := e.localPath("uploads/app.html"); err != nil || got != inside {
			t.Errorf("allow %v: %q %v", dirs, got, err)
		}
		for _, p := range []string{"secret.txt", "uploads/link.txt", "uploads/../secret.txt"} {
			if _, err := e.localPath(p); err == nil || !strings.Contains(err.Error(), "outside the allowed directories") {
				t.Errorf("allow %v: %s was accepted (%v)", dirs, p, err)
			}
		}
	}
	// The default is the working directory.
	e := &env{settings: Settings{Mode: Local}}
	if _, err := e.localPath("secret.txt"); err != nil {
		t.Errorf("default working directory: %v", err)
	}
	if _, err := (&env{settings: Settings{Mode: Remote}}).localPath(inside); err == nil {
		t.Error("remote mode accepted a path")
	}
}
