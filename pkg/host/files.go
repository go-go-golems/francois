package host

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

func (h *Host) listFiles(dir string, raw json.RawMessage) (any, error) {
	if h.ProjectRoot == "" {
		return nil, errors.New("Project files are not configured")
	}
	if dir == "" {
		dir = "."
	}
	if !fs.ValidPath(dir) {
		return nil, errors.New("Invalid file path")
	}
	var args struct {
		Recursive bool   `json:"recursive"`
		Cursor    string `json:"cursor"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(h.ProjectRoot)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	entries := []map[string]any{}
	err = fs.WalkDir(root.FS(), dir, func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == dir {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, map[string]any{"path": name, "isDirectory": d.IsDir(), "size": info.Size()})
		if len(entries) > 10000 {
			return errors.New("Project listing exceeds 10000 entries")
		}
		if !args.Recursive && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i]["path"].(string) < entries[j]["path"].(string) })
	offset := 0
	binding := dir + ":" + strconv.FormatBool(args.Recursive)
	if args.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(args.Cursor)
		if err != nil {
			return nil, errors.New("Invalid cursor")
		}
		var cursor struct {
			Binding string
			Offset  int
		}
		if err = json.Unmarshal(b, &cursor); err != nil || cursor.Binding != binding || cursor.Offset < 0 || cursor.Offset > len(entries) {
			return nil, errors.New("Invalid cursor")
		}
		offset = cursor.Offset
	}
	end := offset + 500
	if end > len(entries) {
		end = len(entries)
	}
	result := map[string]any{"entries": entries[offset:end]}
	if end < len(entries) {
		b, _ := json.Marshal(struct {
			Binding string
			Offset  int
		}{binding, end})
		result["cursor"] = base64.RawURLEncoding.EncodeToString(b)
	}
	return result, nil
}
func (h *Host) readFile(w http.ResponseWriter, r *http.Request, artifact string) {
	a, err := h.Store.Get(r.Context(), artifact)
	if err != nil {
		return
	}
	if _, ok := a.Manifest["files"]; !ok || h.ProjectRoot == "" {
		fail(w, 404, "not_found", "File not found")
		return
	}
	name := r.URL.Query().Get("path")
	if !fs.ValidPath(name) || name == "." {
		fail(w, 400, "bad_request", "Invalid file path")
		return
	}
	offset := int64(0)
	length := int64(25 << 20)
	if s := r.URL.Query().Get("offset"); s != "" {
		offset, err = strconv.ParseInt(s, 10, 64)
		if err != nil || offset < 0 {
			fail(w, 400, "bad_request", "Invalid offset")
			return
		}
	}
	if s := r.URL.Query().Get("length"); s != "" {
		length, err = strconv.ParseInt(s, 10, 64)
		if err != nil || length <= 0 || length > 25<<20 {
			fail(w, 400, "bad_request", "Invalid read length")
			return
		}
	}
	root, err := os.OpenRoot(h.ProjectRoot)
	if err != nil {
		fail(w, 404, "not_found", "File not found")
		return
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(name)
	if err != nil {
		fail(w, 404, "not_found", "File not found")
		return
	}
	defer func() { _ = f.Close() }()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > 512<<20 {
		fail(w, 400, "bad_request", "Not a readable regular file")
		return
	}
	// Hash the opened inode so revision identifies the full byte sequence across chunks.
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		fail(w, 503, "unavailable", "File read failed")
		return
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		fail(w, 400, "bad_request", "Invalid range")
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, length))
	if err != nil {
		fail(w, 503, "unavailable", "File read failed")
		return
	}
	after, err := f.Stat()
	if err != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		fail(w, 409, "changed", "File changed during read")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-File-Size", strconv.FormatInt(before.Size(), 10))
	w.Header().Set("X-File-Revision", hex.EncodeToString(hash.Sum(nil)))
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(path.Base(name), `"`, "")+`"`)
	_, _ = w.Write(data)
}
