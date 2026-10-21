package client

import (
	"context"
	"net/http"
)

// DeviceCode starts a device login: show UserCode and VerificationURIComplete
// to the user, then poll with DeviceCode every Interval seconds.
type DeviceCode struct {
	DeviceCode              string `json:"deviceCode"`
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
	ExpiresIn               int    `json:"expiresIn"`
	Interval                int    `json:"interval"`
}

// DeviceToken is the token issued once the login was approved; Token is its
// secret, shown only here.
type DeviceToken struct {
	Token string `json:"token"`
	Info  Token  `json:"info"`
	User  MeUser `json:"user"`
}

// DeviceAuthorize starts a device login (POST /api/device/authorize). It needs
// no token. Empty scopes let the approver choose, defaulting to every scope
// their role allows; days nil means the server default (90).
func (c *Client) DeviceAuthorize(ctx context.Context, clientName string, scopes []string, days *int) (DeviceCode, error) {
	var out DeviceCode
	in := map[string]any{"clientName": clientName, "scopes": scopes}
	if days != nil {
		in["days"] = *days
	}
	return out, c.send2(ctx, http.MethodPost, "/api/device/authorize", in, &out)
}

// DevicePoll asks once whether the login was approved (POST
// /api/device/token). Until then it fails with the codes
// authorization_pending, slow_down, access_denied or expired_token.
func (c *Client) DevicePoll(ctx context.Context, deviceCode string) (DeviceToken, error) {
	var out DeviceToken
	return out, c.send2(ctx, http.MethodPost, "/api/device/token", map[string]string{"deviceCode": deviceCode}, &out)
}

// RevokeSelf revokes the client's own token (DELETE /api/me/token).
func (c *Client) RevokeSelf(ctx context.Context) error {
	return c.send2(ctx, http.MethodDelete, "/api/me/token", nil, nil)
}
