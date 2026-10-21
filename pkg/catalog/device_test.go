package catalog

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/client"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func TestUserCodes(t *testing.T) {
	for range 50 {
		c := store.NewUserCode()
		if n, ok := store.NormalizeUserCode(strings.ToLower(store.FormatUserCode(c))); !ok || n != c {
			t.Fatalf("round trip %q → %q %v", c, n, ok)
		}
	}
	for in, want := range map[string]string{"wdjb-mjht": "WDJBMJHT", " WDJB MJHT ": "WDJBMJHT", "WDJB-MJH": "", "AEIO-UWDJ": "", "WDJB-MJHT-B": ""} {
		got, ok := store.NormalizeUserCode(in)
		if (want == "") == ok || (ok && got != want) {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
}

func TestSuggestGrant(t *testing.T) {
	admin := store.ScopesForRole("admin")
	for _, c := range []struct {
		allowed, requested []string
		days               int
		want               []string
		wantDays           int
	}{
		{admin, nil, 90, admin, 90},
		{admin, nil, 0, admin, 90}, // admin scopes need an expiry
		{store.ScopesForRole("creator"), nil, 0, []string{"studio", "account"}, 0},
		{store.ScopesForRole("subscriber"), []string{"studio", "account"}, 30, []string{"account"}, 30},
		{store.ScopesForRole("subscriber"), []string{"studio"}, 30, []string{}, 30},
	} {
		got, days := suggestGrant(c.allowed, c.requested, c.days)
		if !slices.Equal(got, c.want) || days != c.wantDays {
			t.Errorf("%v %v %d: got %v %d", c.allowed, c.requested, c.days, got, days)
		}
	}
}

// deviceStart begins a device login on host as the CLI does.
func (e *env) deviceStart(host string, body map[string]any) map[string]any {
	e.t.Helper()
	code, out := e.portalAt(host, nil, "POST", "/api/device/authorize", body)
	if code != 201 {
		e.t.Fatalf("authorize %d %v", code, out)
	}
	return out
}

// devicePoll polls once after making the previous poll look old enough.
func (e *env) devicePoll(host, deviceCode string) (int, map[string]any) {
	e.t.Helper()
	if _, err := e.store.Pool.Exec(context.Background(), `UPDATE device_authorizations SET last_polled_at = now() - interval '1 hour'`); err != nil {
		e.t.Fatal(err)
	}
	return e.portalAt(host, nil, "POST", "/api/device/token", map[string]any{"deviceCode": deviceCode})
}

func TestDeviceLogin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	cleo := e.user("cleo", "creator", false)

	start := e.deviceStart("portal.test", map[string]any{"clientName": "francois CLI on laptop"})
	device, _ := start["deviceCode"].(string)
	user, _ := start["userCode"].(string)
	if !strings.HasPrefix(device, "fdc_") || len(user) != 9 || start["verificationUriComplete"] != "http://portal.test/device?code="+user ||
		start["interval"] != float64(5) || start["expiresIn"] != float64(600) {
		t.Fatalf("start %v", start)
	}
	if code, out := e.devicePoll("portal.test", device); code != 400 || errCode(out) != "authorization_pending" {
		t.Fatalf("pending %d %v", code, out)
	}
	// Polling again at once is too fast.
	if code, out := e.portalAt("portal.test", nil, "POST", "/api/device/token", map[string]any{"deviceCode": device}); code != 400 || errCode(out) != "slow_down" {
		t.Fatalf("slow_down %d %v", code, out)
	}

	// The approval page sees the request; by default every scope the role allows.
	path := "/api/device/requests/" + strings.ToLower(strings.ReplaceAll(user, "-", ""))
	code, req := e.portal(&cleo, "GET", path, nil)
	if code != 200 || req["clientName"] != "francois CLI on laptop" || req["requestedFrom"] == "" || req["suggestedDays"] != float64(90) {
		t.Fatalf("request %d %v", code, req)
	}
	if s, _ := req["suggestedScopes"].([]any); len(s) != 2 || s[0] != "studio" || s[1] != "account" {
		t.Fatalf("suggested %v", req["suggestedScopes"])
	}
	// A token cannot approve: only a browser session can mint tokens.
	if code, out := e.portal(&cleo, "POST", "/api/tokens", map[string]any{"name": "t", "days": 1, "scopes": []string{"studio"}}); code != 201 {
		t.Fatalf("session token %d %v", code, out)
	} else {
		c, b, _ := e.do(nil, "portal.test", "POST", path+"/approve", strings.NewReader(`{"scopes":["studio"],"days":30}`), map[string]string{"Authorization": "Bearer " + out["token"].(string)})
		if c != 401 || !strings.Contains(string(b), "token_not_allowed") {
			t.Fatalf("token approving %d %s", c, b)
		}
	}
	// Approval without the CSRF header fails like any browser mutation.
	if c, b, _ := e.do(nil, "portal.test", "POST", path+"/approve", strings.NewReader(`{"scopes":["studio"],"days":30}`), map[string]string{"Cookie": "francois_session_dev=" + cleo.token}); c != 403 {
		t.Fatalf("approve without csrf %d %s", c, b)
	}
	if code, out := e.portal(&cleo, "POST", path+"/approve", map[string]any{"scopes": []string{"admin:read"}, "days": 30}); code != 400 || errCode(out) != "invalid_scopes" {
		t.Fatalf("creator approving admin %d %v", code, out)
	}
	if code, out := e.portal(&cleo, "POST", path+"/approve", map[string]any{"scopes": []string{"studio"}, "days": 30}); code != 200 {
		t.Fatalf("approve %d %v", code, out)
	}
	// Decided requests are no longer shown.
	if code, _ := e.portal(&cleo, "GET", path, nil); code != 404 {
		t.Fatalf("decided request visible: %d", code)
	}

	code, got := e.devicePoll("portal.test", device)
	secret, _ := got["token"].(string)
	info, _ := got["info"].(map[string]any)
	if code != 200 || !strings.HasPrefix(secret, "frt_") || info["name"] != "francois CLI on laptop" || got["user"].(map[string]any)["role"] != "creator" {
		t.Fatalf("issued %d %v", code, got)
	}
	if code, out := e.devicePoll("portal.test", device); code != 400 || errCode(out) != "expired_token" {
		t.Fatalf("second claim %d %v", code, out)
	}
	c, b, _ := e.do(nil, "portal.test", "GET", "/api/me", nil, map[string]string{"Authorization": "Bearer " + secret})
	if c != 200 || !strings.Contains(string(b), `"scopes":["studio"]`) {
		t.Fatalf("token works %d %s", c, b)
	}
	audit, err := e.store.Pool.Query(ctx, `SELECT action FROM admin_audit WHERE actor_id=$1 ORDER BY created_at`, cleo.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for audit.Next() {
		var a string
		_ = audit.Scan(&a)
		actions = append(actions, a)
	}
	if !slices.Contains(actions, "token.device_approve") || !slices.Contains(actions, "token.create") {
		t.Fatalf("audit %v", actions)
	}
}

func TestDeviceLoginRefusals(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	ada := e.user("ada", "admin", false)
	sam := e.user("sam", "subscriber", true)
	path := func(start map[string]any) string { return "/api/device/requests/" + start["userCode"].(string) }

	// Deny.
	s := e.deviceStart("portal.test", map[string]any{"clientName": "cli"})
	if code, out := e.portal(&sam, "POST", path(s)+"/deny", nil); code != 200 {
		t.Fatalf("deny %d %v", code, out)
	}
	if code, out := e.devicePoll("portal.test", s["deviceCode"].(string)); code != 400 || errCode(out) != "access_denied" {
		t.Fatalf("denied %d %v", code, out)
	}
	if code, out := e.devicePoll("portal.test", s["deviceCode"].(string)); code != 400 || errCode(out) != "expired_token" {
		t.Fatalf("after denial %d %v", code, out)
	}

	// Admin scopes: allowed, defaulted, but only with an expiry of 1–90 days.
	s = e.deviceStart("portal.test", map[string]any{"clientName": "cli", "days": 0})
	if code, req := e.portal(&ada, "GET", path(s), nil); code != 200 || len(req["suggestedScopes"].([]any)) != 4 || req["suggestedDays"] != float64(90) {
		t.Fatalf("admin suggestion %d %v", code, req)
	}
	if code, out := e.portal(&ada, "POST", path(s)+"/approve", map[string]any{"scopes": []string{"admin:write"}, "days": 0}); code != 400 || errCode(out) != "invalid_request" {
		t.Fatalf("admin without expiry %d %v", code, out)
	}
	if code, out := e.portal(&ada, "POST", path(s)+"/approve", map[string]any{"scopes": []string{"admin:write"}, "days": 90}); code != 200 {
		t.Fatalf("admin approve %d %v", code, out)
	}
	// Disabled between approval and claim: no token.
	if err := e.store.SetUserDisabled(ctx, ada.user.ID, true); err != nil {
		t.Fatal(err)
	}
	if code, out := e.devicePoll("portal.test", s["deviceCode"].(string)); code != 400 || errCode(out) != "access_denied" {
		t.Fatalf("disabled approver %d %v", code, out)
	}
	var n int
	_ = e.store.Pool.QueryRow(ctx, `SELECT count(*) FROM api_tokens WHERE user_id=$1`, ada.user.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("token minted for a disabled approver: %d", n)
	}

	// Codes belong to the site they were issued on.
	s = e.deviceStart("portal.test", map[string]any{"clientName": "cli"})
	if code, _ := e.portalAt("boutique.test", &sam, "GET", path(s), nil); code != 404 {
		t.Fatalf("other site sees the code: %d", code)
	}
	if code, out := e.devicePoll("boutique.test", s["deviceCode"].(string)); code != 400 || errCode(out) != "expired_token" {
		t.Fatalf("other site polls %d %v", code, out)
	}

	// Expiry.
	if _, err := e.store.Pool.Exec(ctx, `UPDATE device_authorizations SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.portal(&sam, "GET", path(s), nil); code != 404 {
		t.Fatalf("expired request visible: %d", code)
	}
	if code, out := e.devicePoll("portal.test", s["deviceCode"].(string)); code != 400 || errCode(out) != "expired_token" {
		t.Fatalf("expired %d %v", code, out)
	}

	// Bad input and unknown codes.
	if code, out := e.portalAt("portal.test", nil, "POST", "/api/device/authorize", map[string]any{"clientName": "cli", "scopes": []string{"root"}}); code != 400 || errCode(out) != "invalid_scopes" {
		t.Fatalf("unknown scope %d %v", code, out)
	}
	if code, out := e.portalAt("portal.test", nil, "POST", "/api/device/authorize", map[string]any{"clientName": ""}); code != 400 || errCode(out) != "invalid_request" {
		t.Fatalf("no name %d %v", code, out)
	}
	if code, out := e.portal(&sam, "GET", "/api/device/requests/BCDF-GHJK", nil); code != 404 || errCode(out) != "not_found" {
		t.Fatalf("unknown code %d %v", code, out)
	}
	if code, _ := e.portal(nil, "GET", "/api/device/requests/BCDF-GHJK", nil); code != 401 {
		t.Fatalf("anonymous lookup %d", code)
	}
	// Unknown codes are budgeted per user.
	for range 30 {
		e.portal(&sam, "GET", "/api/device/requests/BCDF-GHJK", nil)
	}
	if code, out := e.portal(&sam, "GET", "/api/device/requests/BCDF-GHJK", nil); code != 429 {
		t.Fatalf("lookup budget %d %v", code, out)
	}
}

func TestDeviceAuthorizeRateLimit(t *testing.T) {
	e := setup(t)
	for range 20 {
		e.deviceStart("portal.test", map[string]any{"clientName": "cli"})
	}
	if code, out := e.portalAt("portal.test", nil, "POST", "/api/device/authorize", map[string]any{"clientName": "cli"}); code != 429 || errCode(out) != "rate_limited" {
		t.Fatalf("21st start %d %v", code, out)
	}
}

func TestTokenRevokesItself(t *testing.T) {
	e := setup(t)
	cleo := e.user("cleo", "creator", false)
	mint := func() string {
		code, out := e.portal(&cleo, "POST", "/api/tokens", map[string]any{"name": "t", "days": 1, "scopes": []string{"studio"}})
		if code != 201 {
			t.Fatalf("mint %d %v", code, out)
		}
		return out["token"].(string)
	}
	a, b := mint(), mint()
	bearer := func(tok, method, path string) (int, string) {
		c, body, _ := e.do(nil, "portal.test", method, path, nil, map[string]string{"Authorization": "Bearer " + tok})
		return c, string(body)
	}
	if code, body := bearer(a, "DELETE", "/api/me/token"); code != 200 {
		t.Fatalf("revoke self %d %s", code, body)
	}
	if code, body := bearer(a, "GET", "/api/me"); code != 401 || !strings.Contains(body, "token_invalid") {
		t.Fatalf("revoked token still works %d %s", code, body)
	}
	if code, body := bearer(b, "GET", "/api/me"); code != 200 {
		t.Fatalf("other token affected %d %s", code, body)
	}
	if code, out := e.portal(&cleo, "DELETE", "/api/me/token", nil); code != 400 || errCode(out) != "token_required" {
		t.Fatalf("session %d %v", code, out)
	}
}

// TestDeviceLoginClient drives the flow through the typed client with strict
// decoding, so the client types follow the server's responses.
func TestDeviceLoginClient(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	cleo := e.user("cleo", "creator", false)
	c := client.New(e.ts.URL, "", client.WithHost("portal.test"), client.WithStrictDecoding())
	dc, err := c.DeviceAuthorize(ctx, "francois CLI on laptop", nil, nil)
	if err != nil || dc.Interval != 5 || dc.UserCode == "" {
		t.Fatalf("authorize %+v %v", dc, err)
	}
	if _, err = c.DevicePoll(ctx, dc.DeviceCode); !client.IsCode(err, "authorization_pending") {
		t.Fatalf("pending %v", err)
	}
	if code, out := e.portal(&cleo, "POST", "/api/device/requests/"+dc.UserCode+"/approve", map[string]any{"scopes": []string{"studio", "account"}, "days": 90}); code != 200 {
		t.Fatalf("approve %d %v", code, out)
	}
	_, _ = e.store.Pool.Exec(ctx, `UPDATE device_authorizations SET last_polled_at = now() - interval '1 hour'`)
	tok, err := c.DevicePoll(ctx, dc.DeviceCode)
	if err != nil || tok.User.Role != "creator" || len(tok.Info.Scopes) != 2 || tok.Info.ExpiresAt == nil {
		t.Fatalf("issued %+v %v", tok, err)
	}
	authed := client.New(e.ts.URL, tok.Token, client.WithHost("portal.test"), client.WithStrictDecoding())
	if me, err := authed.Me(ctx); err != nil || me.Token == nil || me.Token.ID != tok.Info.ID {
		t.Fatalf("me %+v %v", me, err)
	}
	if err = authed.RevokeSelf(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = authed.Me(ctx); !client.IsCode(err, "token_invalid") {
		t.Fatalf("after revoke %v", err)
	}
}
