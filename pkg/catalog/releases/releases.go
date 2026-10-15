// Package releases validates authored HTML and stores immutable release
// files. The admin UI and the CLI share this one publication path.
package releases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// Blobs stores immutable files beneath a private root keyed by content hash.
type Blobs struct{ Root string }

func OpenBlobs(dir string) (*Blobs, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "blobs"), 0o700); err != nil {
		return nil, errors.Wrap(err, "create release directory")
	}
	return &Blobs{Root: abs}, nil
}

var keyPattern = regexp.MustCompile(`^blobs/[0-9a-f]{2}/[0-9a-f]{64}$`)

func (b *Blobs) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid storage key")
	}
	return filepath.Join(b.Root, filepath.FromSlash(key)), nil
}

// Put writes data to a temporary file, syncs it and renames it into its
// content-addressed key. Existing identical blobs are reused.
func (b *Blobs) Put(data []byte) (string, string, error) {
	h := sha256.Sum256(data)
	sum := hex.EncodeToString(h[:])
	key := "blobs/" + sum[:2] + "/" + sum
	final, err := b.path(key)
	if err != nil {
		return "", "", err
	}
	if existing, e := os.ReadFile(final); e == nil && bytes.Equal(existing, data) {
		return key, sum, nil
	}
	if err = os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return "", "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(final), ".upload-*")
	if err != nil {
		return "", "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", "", err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", "", err
	}
	if err = tmp.Close(); err != nil {
		return "", "", err
	}
	return key, sum, os.Rename(tmp.Name(), final)
}

// PutReader streams r (at most limit bytes) into a temporary file while
// hashing it, then renames it into its content-addressed key. It returns
// ErrTooLarge when r holds more than limit bytes.
func (b *Blobs) PutReader(r io.Reader, limit int64) (string, string, int64, error) {
	tmp, err := os.CreateTemp(filepath.Join(b.Root, "blobs"), ".upload-*")
	if err != nil {
		return "", "", 0, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit+1))
	if err == nil && size > limit {
		err = ErrTooLarge
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", "", 0, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	key := "blobs/" + sum[:2] + "/" + sum
	final, err := b.path(key)
	if err != nil {
		return "", "", 0, err
	}
	if fi, e := os.Stat(final); e == nil && fi.Size() == size {
		return key, sum, size, nil
	}
	if err = os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return "", "", 0, err
	}
	return key, sum, size, os.Rename(tmp.Name(), final)
}

// ErrTooLarge reports an upload over its size limit.
var ErrTooLarge = errors.New("file exceeds the size limit")

// Open returns the stored file for streaming (Range requests seek instead
// of reading it whole). The content-addressed key was verified on write;
// Read verifies the hash for files small enough to check on every use.
func (b *Blobs) Open(key string) (*os.File, error) {
	p, err := b.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p) // #nosec G304 -- key is validated against keyPattern
}

// Read returns the stored bytes after verifying their recorded hash.
func (b *Blobs) Read(f store.ReleaseFile) ([]byte, error) {
	p, err := b.path(f.StorageKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, errors.Wrap(err, "read release file")
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != f.SHA256 {
		return nil, errors.New("release file hash mismatch")
	}
	return data, nil
}

// Missing reports referenced files that are absent or corrupted.
func (b *Blobs) Missing(files []store.ReleaseFile) []store.ReleaseFile {
	out := []store.ReleaseFile{}
	for _, f := range files {
		if _, err := b.Read(f); err != nil {
			out = append(out, f)
		}
	}
	return out
}

var artifactID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// ValidArtifactID reports whether id is an acceptable artifact identifier.
func ValidArtifactID(id string) bool { return artifactID.MatchString(id) }

var versionLabel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,39}$`)

// Known capability namespaces the hosted runtime can grant.
var capabilities = map[string]bool{"sample": true, "db": true, "user": true, "room": true, "comments": true, "assets": true, "artifact": true, "downloads": true, "files": true}

type Upload struct {
	ArtifactID  string
	Title       string
	Description string
	Tags        []string
	Version     string
	Changelog   string
	HTML        []byte
	Manifest    []byte
	FreeSample  bool
	MaxHTML     int64
	// Creators are credited on the artifact when this upload creates it.
	Creators []string
}

// Check is one publication validation finding shown to the operator.
type Check struct {
	Code    string `json:"code"`
	Level   string `json:"level"` // error|warning|info
	Message string `json:"message"`
}

var (
	bootstrapMarker  = regexp.MustCompile(`id=["']francois-view["']|/static/runtime\.js`)
	credentialMarker = regexp.MustCompile(`(?i)(sk_(live|test)_[A-Za-z0-9]{8,}|rk_(live|test)_[A-Za-z0-9]{8,}|whsec_[A-Za-z0-9]{8,}|-----BEGIN [A-Z ]*PRIVATE KEY-----|"refresh_token"\s*:|AKIA[0-9A-Z]{16}|sk-ant-[A-Za-z0-9_-]{10,}|sk-[A-Za-z0-9]{32,})`)
	sqliteMarker     = []byte("SQLite format 3\x00")
	externalScript   = regexp.MustCompile(`(?i)<script[^>]+src=["'](https?:)?//([^/"']+)`)
)

// Validate inspects an upload. Errors block the draft; warnings are shown.
func Validate(u Upload) (map[string]json.RawMessage, []Check) {
	checks := []Check{}
	add := func(level, code, msg string) { checks = append(checks, Check{Code: code, Level: level, Message: msg}) }
	if !artifactID.MatchString(u.ArtifactID) {
		add("error", "artifact_id", "Artifact ID must be lowercase letters, digits, - or _")
	}
	if !versionLabel.MatchString(u.Version) {
		add("error", "version", "Version must be 1–40 characters of letters, digits, . _ + -")
	}
	if strings.TrimSpace(u.Title) == "" || len(u.Title) > 120 {
		add("error", "title", "Title is required (at most 120 characters)")
	}
	if len(u.Description) > 2000 || len(u.Changelog) > 8000 {
		add("error", "text_length", "Description or changelog is too long")
	}
	limit := u.MaxHTML
	if limit <= 0 {
		limit = 16 << 20
	}
	if len(u.HTML) == 0 || int64(len(u.HTML)) > limit {
		add("error", "html_size", "HTML must be between 1 byte and the configured size limit")
	}
	lower := strings.ToLower(string(u.HTML))
	if !strings.Contains(lower, "<html") {
		add("error", "html_document", "Upload a complete HTML document")
	}
	if bytes.Contains(u.HTML, sqliteMarker) {
		add("error", "database_copy", "The file contains an embedded SQLite database")
	}
	if bootstrapMarker.Match(u.HTML) {
		add("error", "runtime_bootstrap", "The HTML contains an injected Francois view bootstrap; upload authored source, not a served page")
	}
	if credentialMarker.Match(u.HTML) {
		add("error", "credential_like", "The HTML contains credential-like text; remove secrets before publishing")
	}
	hosts := map[string]bool{}
	for _, m := range externalScript.FindAllStringSubmatch(string(u.HTML), -1) {
		hosts[strings.ToLower(m[2])] = true
	}
	for h := range hosts {
		add("info", "external_script", "Loads scripts from "+h)
	}
	manifest := map[string]json.RawMessage{"sample": json.RawMessage(`{}`)}
	if len(bytes.TrimSpace(u.Manifest)) > 0 {
		if len(u.Manifest) > 64<<10 {
			add("error", "manifest_size", "Manifest exceeds 64 KiB")
		} else if err := json.Unmarshal(u.Manifest, &manifest); err != nil {
			add("error", "manifest_json", "Manifest must be a JSON object of capability declarations")
		}
	}
	for name := range manifest {
		switch {
		case !capabilities[name]:
			add("error", "manifest_unknown", "Unknown capability "+name)
		case name == "files":
			add("warning", "files_disabled", "Server project files are not available to hosted instances")
		case name == "artifact":
			add("warning", "artifact_publish", "Hosted instances cannot republish catalog code; artifact.publish is disabled")
		}
	}
	return manifest, checks
}

func Blocking(checks []Check) bool {
	for _, c := range checks {
		if c.Level == "error" {
			return true
		}
	}
	return false
}

// Publisher is the single release-creation service.
type Publisher struct {
	Store *store.Store
	Blobs *Blobs
}

// ErrInvalid wraps validation findings that block a draft.
type ErrInvalid struct{ Checks []Check }

func (e *ErrInvalid) Error() string {
	for _, c := range e.Checks {
		if c.Level == "error" {
			return c.Message
		}
	}
	return "invalid release"
}

// prepare completes an upload for an existing artifact: the title is only
// required when the upload creates the artifact.
func (p *Publisher) prepare(ctx context.Context, u *Upload) (bool, error) {
	a, err := p.Store.Artifact(ctx, u.ArtifactID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if u.Title == "" {
		u.Title = a.Title
	}
	return true, nil
}

// Check validates an upload without storing anything.
func (p *Publisher) Check(ctx context.Context, u Upload) (map[string]json.RawMessage, []Check, error) {
	if _, err := p.prepare(ctx, &u); err != nil {
		return nil, nil, err
	}
	manifest, checks := Validate(u)
	return manifest, checks, nil
}

// ErrArtifactTaken: another upload created the artifact between the caller's
// authorization and this draft. The caller must authorize again as an editor
// of the existing artifact before retrying.
var ErrArtifactTaken = errors.New("the artifact was created by another upload")

// CreateDraft validates, stores blobs atomically and only then commits the
// draft metadata. A crash between both leaves an unreferenced blob. A release
// for an existing artifact leaves its catalog metadata alone; the upload's
// title, description and tags only seed a new artifact.
func (p *Publisher) CreateDraft(ctx context.Context, actor string, u Upload) (store.Release, []Check, error) {
	exists, err := p.prepare(ctx, &u)
	if err != nil {
		return store.Release{}, nil, err
	}
	manifest, checks := Validate(u)
	if Blocking(checks) {
		return store.Release{}, checks, &ErrInvalid{Checks: checks}
	}
	if !exists {
		a := store.Artifact{ID: u.ArtifactID, Title: u.Title, Description: u.Description, Tags: u.Tags, Visibility: "listed"}
		if _, err = p.Store.CreateArtifact(ctx, a, u.Creators...); errors.Is(err, store.ErrConflict) {
			return store.Release{}, checks, ErrArtifactTaken
		}
		if err != nil {
			return store.Release{}, checks, err
		}
	}
	htmlKey, htmlSum, err := p.Blobs.Put(u.HTML)
	if err != nil {
		return store.Release{}, checks, err
	}
	rawManifest, _ := json.MarshalIndent(manifest, "", "  ")
	manKey, manSum, err := p.Blobs.Put(rawManifest)
	if err != nil {
		return store.Release{}, checks, err
	}
	r := store.Release{ArtifactID: u.ArtifactID, Version: u.Version, Changelog: u.Changelog, Manifest: manifest, FreeSample: u.FreeSample, CreatedBy: actor,
		Files: []store.ReleaseFile{
			{Role: "html", Name: u.ArtifactID + "-" + u.Version + ".html", MediaType: "text/html", Size: int64(len(u.HTML)), SHA256: htmlSum, StorageKey: htmlKey},
			{Role: "manifest", Name: u.ArtifactID + "-" + u.Version + ".manifest.json", MediaType: "application/json", Size: int64(len(rawManifest)), SHA256: manSum, StorageKey: manKey},
		}}
	r, err = p.Store.CreateDraft(ctx, r)
	if err != nil {
		return r, checks, err
	}
	_ = p.Store.Audit(ctx, actor, "release.draft", r.ID, map[string]any{"artifact": r.ArtifactID, "version": r.Version, "sha256": htmlSum})
	return r, checks, nil
}

// HTML returns a release's authored HTML bytes.
func (p *Publisher) HTML(r store.Release) ([]byte, error) {
	for _, f := range r.Files {
		if f.Role == "html" {
			return p.Blobs.Read(f)
		}
	}
	return nil, errors.New("release has no HTML file")
}

// ReadLimited reads at most max+1 bytes so callers can reject oversize input.
func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file exceeds the size limit")
	}
	return data, nil
}
