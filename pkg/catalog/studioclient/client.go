// Package studioclient uploads releases to a hosted catalog's studio API
// with a personal API token, for `francois catalog push`.
package studioclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Release describes one upload. Title, Description and Tags only matter
// when the upload creates the artifact.
type Release struct {
	ArtifactID, Version, Title, Description, Changelog string
	Tags                                               []string
	HTML, Manifest                                     []byte
	FreeSample                                         bool
	// Attachments are local file paths uploaded to the draft before it is
	// published.
	Attachments []string
}

// Attachment is a file attached to a release.
type Attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Check is a server-side validation finding.
type Check struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// Result is the outcome of a push.
type Result struct {
	ReleaseID, State string
	Checks           []Check
	Attachments      []Attachment
}

// Error is a studio API failure with its stable code.
type Error struct {
	Status  int
	Code    string
	Message string
	Checks  []Check
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

type Client struct {
	// Server is the portal origin, e.g. https://slop.boutique.
	Server string
	Token  string
	HTTP   *http.Client
	// Host overrides the Host header (tests, or a server reached through a
	// tunnel under another name).
	Host string
}

func New(server, token string) *Client {
	return &Client{Server: strings.TrimRight(server, "/"), Token: token, HTTP: &http.Client{Timeout: 2 * time.Minute}}
}

func (c *Client) do(ctx context.Context, method, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.Server+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if c.Host != "" {
		req.Host = c.Host
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		var f struct {
			Error  struct{ Code, Message string } `json:"error"`
			Checks []Check                        `json:"checks"`
		}
		_ = json.Unmarshal(raw, &f)
		if f.Error.Code == "" {
			f.Error.Code, f.Error.Message = "http_"+strconv.Itoa(resp.StatusCode), strings.TrimSpace(string(raw))
		}
		return &Error{Status: resp.StatusCode, Code: f.Error.Code, Message: f.Error.Message, Checks: f.Checks}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func form(r Release) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{"artifactId": r.ArtifactID, "version": r.Version, "title": r.Title, "description": r.Description,
		"changelog": r.Changelog, "tags": strings.Join(r.Tags, ","), "freeSample": strconv.FormatBool(r.FreeSample)}
	if len(r.Manifest) > 0 {
		fields["manifestJson"] = string(r.Manifest)
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	f, err := w.CreateFormFile("html", r.ArtifactID+".html")
	if err != nil {
		return nil, "", err
	}
	if _, err = f.Write(r.HTML); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), w.Close()
}

// Check validates a release without storing it.
func (c *Client) Check(ctx context.Context, r Release) ([]Check, error) {
	body, ct, err := form(r)
	if err != nil {
		return nil, err
	}
	var out struct {
		Checks []Check `json:"checks"`
	}
	err = c.do(ctx, http.MethodPost, "/api/studio/releases/check", ct, body, &out)
	return out.Checks, err
}

// Attach uploads the file at path to a release. The body is streamed, so
// large files are not held in memory.
func (c *Client) Attach(ctx context.Context, releaseID, path, label string) (Attachment, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator names the file
	if err != nil {
		return Attachment{}, err
	}
	defer func() { _ = f.Close() }()
	pr, pw := io.Pipe()
	w := multipart.NewWriter(pw)
	go func() {
		err := w.WriteField("label", label)
		var part io.Writer
		if err == nil {
			part, err = w.CreateFormFile("file", filepath.Base(path))
		}
		if err == nil {
			_, err = io.Copy(part, f)
		}
		if err == nil {
			err = w.Close()
		}
		_ = pw.CloseWithError(err)
	}()
	var out Attachment
	err = c.do(ctx, http.MethodPost, "/api/studio/releases/"+releaseID+"/attachments", w.FormDataContentType(), pr, &out)
	_ = pr.Close()
	return out, err
}

// Push creates a draft release, uploads its attachments and, when publish
// is set, publishes it.
func (c *Client) Push(ctx context.Context, r Release, publish bool) (Result, error) {
	body, ct, err := form(r)
	if err != nil {
		return Result{}, err
	}
	var rel struct {
		ID     string  `json:"id"`
		State  string  `json:"state"`
		Checks []Check `json:"checks"`
	}
	if err = c.do(ctx, http.MethodPost, "/api/studio/releases", ct, body, &rel); err != nil {
		return Result{}, err
	}
	res := Result{ReleaseID: rel.ID, State: rel.State, Checks: rel.Checks}
	for _, p := range r.Attachments {
		a, err := c.Attach(ctx, rel.ID, p, "")
		if err != nil {
			return res, errors.Wrapf(err, "the draft was created but attaching %s failed", p)
		}
		res.Attachments = append(res.Attachments, a)
	}
	if publish {
		var pub struct {
			State string `json:"state"`
		}
		if err = c.do(ctx, http.MethodPost, "/api/studio/releases/"+rel.ID+"/publish", "", nil, &pub); err != nil {
			return res, errors.Wrap(err, "the draft was created but publishing failed")
		}
		res.State = pub.State
	}
	return res, nil
}
