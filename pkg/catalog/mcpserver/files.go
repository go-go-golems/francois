package mcpserver

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// maxInline bounds inline (base64) uploads in remote mode.
const maxInline = 10 << 20

// localPath resolves a path a model asked to upload and refuses anything
// outside the allowed directories, after following symlinks. Without this a
// model could be talked into publishing ~/.ssh/id_ed25519 as an attachment.
func (e *env) localPath(p string) (string, error) {
	if e.settings.Mode != Local {
		return "", errors.New("file paths are not accepted by the remote server; send the content inline")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", errors.Wrap(err, "file not found")
	}
	dirs := e.settings.AllowDirs
	if len(dirs) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dirs = []string{wd}
	}
	for _, d := range dirs {
		root, err := filepath.EvalSymlinks(d)
		if err != nil {
			continue
		}
		if rel, err := filepath.Rel(root, resolved); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return resolved, nil
		}
	}
	return "", errors.Errorf("%s is outside the allowed directories (%s)", p, strings.Join(dirs, ", "))
}

// readLocal reads an allowed local file.
func (e *env) readLocal(p string) ([]byte, error) {
	resolved, err := e.localPath(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(resolved) // #nosec G304 -- confined to the allowed directories above
}

// openUpload returns a file to upload: a local path (local mode) or inline
// base64 content (either mode).
func (e *env) openUpload(path, contentBase64, filename string) (io.ReadCloser, string, error) {
	if path != "" {
		resolved, err := e.localPath(path)
		if err != nil {
			return nil, "", err
		}
		f, err := os.Open(resolved) // #nosec G304 -- confined to the allowed directories
		if err != nil {
			return nil, "", err
		}
		return f, filepath.Base(resolved), nil
	}
	if contentBase64 == "" || filename == "" {
		return nil, "", errors.New("give a path, or contentBase64 with a filename")
	}
	if base64.StdEncoding.DecodedLen(len(contentBase64)) > maxInline {
		return nil, "", errors.New("inline content is limited to 10 MiB")
	}
	raw, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return nil, "", errors.Wrap(err, "contentBase64")
	}
	return io.NopCloser(bytes.NewReader(raw)), filepath.Base(filename), nil
}
