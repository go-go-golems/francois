package identity

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// SendRegistration only operates on a subject reserved by our registration
// store. Email search detects collisions; it never adopts another identity.
// The returned session is empty for a first-use email-verification proof.
func (e *EmailLogin) SendRegistration(ctx context.Context, subject, email, template string) (string, error) {
	if subject == "" || len(email) > 200 || len(template) > 200 {
		return "", errors.New("invalid registration request")
	}
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
	path := "/v2/users/" + url.PathEscape(subject)
	err := e.call(ctx, http.MethodGet, path, nil, &response)
	if err == responseError(http.StatusNotFound) {
		var matches struct {
			Result []jsonUserID `json:"result"`
		}
		body := map[string]any{"query": map[string]any{"limit": 1}, "queries": []any{
			map[string]any{"organizationIdQuery": map[string]string{"organizationId": e.organization}},
			map[string]any{"emailQuery": map[string]string{"emailAddress": email, "method": "TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE"}},
		}}
		if err = e.call(ctx, http.MethodPost, "/v2/users", body, &matches); err != nil {
			return "", err
		}
		if len(matches.Result) != 0 {
			return "", errors.New("registration conflicts with an existing identity")
		}
		var created jsonUserID
		body = map[string]any{"userId": subject, "username": email,
			"organization": map[string]string{"orgId": e.organization},
			"profile":      map[string]string{"givenName": "New", "familyName": "User", "displayName": "New user"},
			"email":        map[string]any{"email": email, "sendCode": map[string]string{"urlTemplate": template}}}
		if err = e.call(ctx, http.MethodPost, "/v2/users/human", body, &created); err != nil {
			return "", err
		}
		if created.ID != subject {
			return "", errors.New("registration response has mismatched identity")
		}
		return "", nil
	}
	if err != nil {
		return "", err
	}
	u := response.User
	if u.ID != subject || u.Details.Organization != e.organization || u.Human == nil || !strings.EqualFold(u.Human.Email.Address, email) ||
		(u.State != "USER_STATE_ACTIVE" && u.State != "USER_STATE_INITIAL") {
		return "", errors.New("registration identity mismatch")
	}
	if !u.Human.Email.Verified {
		return "", e.call(ctx, http.MethodPost, path+"/email/resend", map[string]any{"sendCode": map[string]string{"urlTemplate": template}}, nil)
	}
	if err = e.CheckUser(ctx, subject, email); err != nil {
		return "", err
	}
	if err = e.EnsureEmailOTP(ctx, subject); err != nil {
		return "", err
	}
	return e.SendLink(ctx, subject, template)
}

type jsonUserID struct {
	ID string `json:"userId"`
}

func (e *EmailLogin) VerifyRegistration(ctx context.Context, subject, code string) error {
	return e.call(ctx, http.MethodPost, "/v2/users/"+url.PathEscape(subject)+"/email/verify", map[string]string{"verificationCode": code}, nil)
}

// EnsureEmailOTP is used only after verification on registration-owned users.
// Re-reading after an enrollment error also handles concurrent confirmations.
func (e *EmailLogin) EnsureEmailOTP(ctx context.Context, subject string) error {
	path := "/v2/users/" + url.PathEscape(subject)
	hasOTP := func() (bool, error) {
		var methods struct {
			Types []string `json:"authMethodTypes"`
		}
		if err := e.call(ctx, http.MethodGet, path+"/authentication_methods", nil, &methods); err != nil {
			return false, err
		}
		for _, method := range methods.Types {
			if method == "AUTHENTICATION_METHOD_TYPE_OTP_EMAIL" {
				return true, nil
			}
		}
		return false, nil
	}
	ok, err := hasOTP()
	if err != nil || ok {
		return err
	}
	err = e.call(ctx, http.MethodPost, path+"/otp_email", struct{}{}, nil)
	if err == nil {
		return nil
	}
	if ok, _ = hasOTP(); ok {
		return nil
	}
	return err
}
