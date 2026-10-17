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

// EmailLogin uses ZITADEL's Session API only as a short-lived proof of
// mailbox possession. Its privileged credential never reaches the browser.
type EmailLogin struct {
	issuer, organization, token string
	client                      *http.Client
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
		client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
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
		return errors.Errorf("identity service returned HTTP %d", res.StatusCode)
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

// CheckUser refuses stale local email data, inactive accounts and identities
// outside the explicitly configured organization. No email linking occurs.
func (e *EmailLogin) CheckUser(ctx context.Context, subject, email string) error {
	var response struct {
		User struct {
			ID      string `json:"userId"`
			State   string `json:"state"`
			Details struct {
				Organization string `json:"resourceOwner"`
			} `json:"details"`
			Human *struct {
				Email struct {
					Address  string `json:"email"`
					Verified bool   `json:"isVerified"`
				} `json:"email"`
			} `json:"human"`
		} `json:"user"`
	}
	if subject == "" {
		return errors.New("missing identity")
	}
	if err := e.call(ctx, http.MethodGet, "/v2/users/"+url.PathEscape(subject), nil, &response); err != nil {
		return err
	}
	u := response.User
	if u.ID != subject || u.State != "USER_STATE_ACTIVE" || u.Details.Organization != e.organization || u.Human == nil || !u.Human.Email.Verified || !strings.EqualFold(u.Human.Email.Address, email) {
		return errors.New("identity is not eligible for email sign-in")
	}
	return nil
}

func (e *EmailLogin) SendLink(ctx context.Context, subject, template string) (string, error) {
	if len(template) > 200 {
		return "", errors.New("email link template exceeds 200 characters")
	}
	body := map[string]any{"checks": map[string]any{"user": map[string]string{"userId": subject}}, "lifetime": "300s",
		"challenges": map[string]any{"otpEmail": map[string]any{"sendCode": map[string]string{"urlTemplate": template}}}}
	var response struct {
		ID string `json:"sessionId"`
	}
	if err := e.call(ctx, http.MethodPost, "/v2/sessions", body, &response); err != nil {
		return "", err
	}
	if response.ID == "" {
		return "", errors.New("identity response missing session")
	}
	return response.ID, nil
}

// Verify requires a new successful email factor for this exact temporary
// session. A checked user factor by itself never establishes authentication.
func (e *EmailLogin) Verify(ctx context.Context, session, subject, code string, created time.Time) error {
	body := map[string]any{"checks": map[string]any{"otpEmail": map[string]string{"code": code}}}
	path := "/v2/sessions/" + url.PathEscape(session)
	if err := e.call(ctx, http.MethodPatch, path, body, nil); err != nil {
		return err
	}
	var response struct {
		Session struct {
			ID      string    `json:"id"`
			Expires time.Time `json:"expirationDate"`
			Factors struct {
				User struct {
					ID           string `json:"id"`
					Organization string `json:"organizationId"`
				} `json:"user"`
				Email struct {
					Verified time.Time `json:"verifiedAt"`
				} `json:"otpEmail"`
			} `json:"factors"`
		} `json:"session"`
	}
	if err := e.call(ctx, http.MethodGet, path, nil, &response); err != nil {
		return err
	}
	s := response.Session
	now := time.Now()
	if s.ID != session || s.Factors.User.ID != subject || s.Factors.User.Organization != e.organization ||
		s.Factors.Email.Verified.IsZero() || s.Factors.Email.Verified.Before(created) || s.Factors.Email.Verified.After(now.Add(time.Minute)) || !s.Expires.After(now) {
		return errors.New("identity response lacks matching fresh email proof")
	}
	return nil
}

func (e *EmailLogin) DeleteSession(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	return e.call(ctx, http.MethodDelete, "/v2/sessions/"+url.PathEscape(id), nil, nil)
}
