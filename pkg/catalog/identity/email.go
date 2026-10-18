package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// EmailLogin uses ZITADEL email verification as mailbox proof for every login.
// Its privileged credential never reaches the browser.
type EmailLogin struct {
	issuer, organization, token string
	client                      *http.Client
}

type responseError int

func (e responseError) Error() string {
	return errors.Errorf("identity service returned HTTP %d", e).Error()
}

func NewEmailLogin(issuer, organization, token string) (*EmailLogin, error) {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("email sign-in requires a ZITADEL issuer origin")
	}
	loopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	secureTransport := u.Scheme == "https" || u.Scheme == "http" && loopback
	if !secureTransport {
		return nil, errors.New("email sign-in requires HTTPS (HTTP is allowed only on loopback)")
	}
	if organization == "" || strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("email sign-in requires an organization and login service token")
	}
	return &EmailLogin{issuer: strings.TrimSuffix(issuer, "/"), organization: organization, token: token,
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (e *EmailLogin) Issuer() string { return e.issuer }

// call deliberately excludes URLs, response bodies and transport errors from
// returned diagnostics: email codes may appear in upstream error responses.
func (e *EmailLogin) call(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return errors.New("encode identity request")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, e.issuer+path, bytes.NewReader(data))
	if err != nil {
		return errors.New("create identity request")
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		return errors.New("identity service unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return responseError(res.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(res.Body, 256*1024+1))
	if err != nil || len(data) > 256*1024 {
		return errors.New("invalid identity response")
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		return errors.New("invalid identity response")
	}
	return nil
}
