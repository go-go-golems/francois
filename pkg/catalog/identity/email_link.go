package identity

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

var ErrEmailUnavailable = errors.New("email identity unavailable")

type EmailIdentity struct {
	Subject, Email, Name string
}

type emailUser struct {
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
		Profile struct {
			Name string `json:"displayName"`
		} `json:"profile"`
	} `json:"human"`
}

func (e *EmailLogin) readEmailUser(ctx context.Context, subject, email string, verified bool) (EmailIdentity, error) {
	var out struct {
		User emailUser `json:"user"`
	}
	if err := e.call(ctx, http.MethodGet, "/v2/users/"+url.PathEscape(subject), nil, &out); err != nil {
		return EmailIdentity{}, err
	}
	u := out.User
	if u.ID != subject || u.Details.Organization != e.organization || u.Human == nil || !strings.EqualFold(u.Human.Email.Address, email) ||
		(u.State != "USER_STATE_ACTIVE" && u.State != "USER_STATE_INITIAL") || (verified && (!u.Human.Email.Verified || u.State != "USER_STATE_ACTIVE")) {
		return EmailIdentity{}, ErrEmailUnavailable
	}
	return EmailIdentity{Subject: u.ID, Email: u.Human.Email.Address, Name: u.Human.Profile.Name}, nil
}

// ResolveEmail selects one identity in our organization. A match is not a
// login: every account still needs a fresh code delivered to its address.
func (e *EmailLogin) ResolveEmail(ctx context.Context, email, newSubject string, signup bool) (EmailIdentity, error) {
	var out struct {
		Result []emailUser `json:"result"`
	}
	search := map[string]any{"query": map[string]any{"limit": 2}, "queries": []any{
		map[string]any{"organizationIdQuery": map[string]string{"organizationId": e.organization}},
		map[string]any{"emailQuery": map[string]string{"emailAddress": email, "method": "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE"}},
	}}
	if err := e.call(ctx, http.MethodPost, "/v2/users", search, &out); err != nil {
		return EmailIdentity{}, err
	}
	if len(out.Result) == 1 {
		return e.readEmailUser(ctx, out.Result[0].ID, email, false)
	}
	if len(out.Result) > 1 || !signup {
		return EmailIdentity{}, ErrEmailUnavailable
	}
	body := map[string]any{"userId": newSubject, "username": email, "organization": map[string]string{"orgId": e.organization},
		"profile": map[string]string{"givenName": "New", "familyName": "User", "displayName": "New user"},
		"email":   map[string]any{"email": email, "returnCode": struct{}{}}}
	var created struct {
		ID string `json:"userId"`
	}
	err := e.call(ctx, http.MethodPost, "/v2/users/human", body, &created)
	if err == responseError(http.StatusConflict) {
		return e.ResolveEmail(ctx, email, "", false)
	}
	if err != nil {
		return EmailIdentity{}, err
	}
	if created.ID != newSubject || newSubject == "" {
		return EmailIdentity{}, errors.New("identity creation returned a mismatched subject")
	}
	return e.readEmailUser(ctx, created.ID, email, false)
}

func (e *EmailLogin) SendEmailLink(ctx context.Context, subject, template string) error {
	if len(template) > 200 {
		return errors.New("email link template exceeds 200 characters")
	}
	return e.call(ctx, http.MethodPost, "/v2/users/"+url.PathEscape(subject)+"/email/send", map[string]any{"sendCode": map[string]string{"urlTemplate": template}}, nil)
}

func (e *EmailLogin) VerifyEmailLink(ctx context.Context, subject, email, code string) (EmailIdentity, error) {
	if _, err := e.readEmailUser(ctx, subject, email, false); err != nil {
		return EmailIdentity{}, err
	}
	if err := e.call(ctx, http.MethodPost, "/v2/users/"+url.PathEscape(subject)+"/email/verify", map[string]string{"verificationCode": code}, nil); err != nil {
		return EmailIdentity{}, err
	}
	return e.readEmailUser(ctx, subject, email, true)
}
