package catalog

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/httpx"
	"github.com/go-go-golems/francois/pkg/catalog/store"
	"github.com/pkg/errors"
)

// Device login: `francois catalog auth login` asks for a code pair, the user
// approves the user code at /device in a signed-in browser, and the CLI's next
// poll receives a personal API token minted from that approval. Tokens are
// still only created from a browser session; the poll carries no credential
// but the device code.

const (
	deviceCodePrefix = "fdc_"
	deviceInterval   = 5 * time.Second
	deviceDays       = 90 // requested lifetime when the CLI names none
)

func (s *Server) deviceRoutes(m *http.ServeMux) {
	m.HandleFunc("POST /api/device/authorize", s.guard(public, noToken, s.deviceAuthorize))
	m.HandleFunc("POST /api/device/token", s.guard(public, noToken, s.deviceToken))
	m.HandleFunc("GET /api/device/requests/{code}", s.guard(session, noToken, s.deviceRequest))
	m.HandleFunc("POST /api/device/requests/{code}/approve", s.guard(enabled, noToken, s.deviceApprove))
	m.HandleFunc("POST /api/device/requests/{code}/deny", s.guard(session, noToken, s.deviceDeny))
}

func knownScope(sc string) bool {
	switch sc {
	case store.ScopeStudio, store.ScopeAccount, store.ScopeAdminRead, store.ScopeAdminWrite:
		return true
	}
	return false
}

// deviceAuthorize starts a device login for the CLI.
func (s *Server) deviceAuthorize(w http.ResponseWriter, r *http.Request) {
	addr := s.clientAddress(r)
	if !s.deviceLimiter.take("authorize:"+addr, 20, time.Hour) || !s.deviceLimiter.take("authorize", 600, time.Hour) {
		httpx.Fail(w, 429, "rate_limited", "Too many login attempts; try again later")
		return
	}
	var in struct {
		ClientName string   `json:"clientName"`
		Scopes     []string `json:"scopes"`
		Days       *int     `json:"days"`
	}
	if err := httpx.Read(w, r, &in, 2048); err != nil {
		httpx.Fail(w, 400, "invalid_request", "Invalid JSON body")
		return
	}
	name, days := strings.TrimSpace(in.ClientName), deviceDays
	if in.Days != nil {
		days = *in.Days
	}
	if name == "" || len(name) > 80 || days < 0 || days > 366 {
		httpx.Fail(w, 400, "invalid_request", "A client name (≤80) and a lifetime of 0–366 days (0 = no expiry) are required")
		return
	}
	scopes := []string{}
	for _, sc := range in.Scopes {
		if !knownScope(sc) {
			httpx.Fail(w, 400, "invalid_scopes", "Unknown scope "+sc)
			return
		}
		if !slices.Contains(scopes, sc) {
			scopes = append(scopes, sc)
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		unavailable(w, err)
		return
	}
	device := deviceCodePrefix + base64.RawURLEncoding.EncodeToString(b)
	origin := s.site(r).PortalOrigin()
	var code string
	var err error
	for range 3 {
		code = store.NewUserCode()
		err = s.Store.CreateDeviceAuthorization(r.Context(), device, code, store.DeviceAuthorization{Origin: origin, ClientName: name,
			RequestedScopes: scopes, RequestedDays: days, RequesterAddress: addr, Interval: deviceInterval})
		if !errors.Is(err, store.ErrConflict) {
			break
		}
	}
	if err != nil {
		unavailable(w, err)
		return
	}
	shown := store.FormatUserCode(code)
	httpx.JSON(w, 201, map[string]any{"deviceCode": device, "userCode": shown, "verificationUri": origin + "/device",
		"verificationUriComplete": origin + "/device?code=" + shown, "expiresIn": int(store.DeviceTTL.Seconds()), "interval": int(deviceInterval.Seconds())})
}

// deviceToken answers the CLI's poll.
func (s *Server) deviceToken(w http.ResponseWriter, r *http.Request) {
	if !s.deviceLimiter.take("poll:"+s.clientAddress(r), 1200, time.Hour) {
		httpx.Fail(w, 429, "rate_limited", "Too many polls; try again later")
		return
	}
	var in struct {
		DeviceCode string `json:"deviceCode"`
	}
	if err := httpx.Read(w, r, &in, 1024); err != nil || !strings.HasPrefix(in.DeviceCode, deviceCodePrefix) {
		httpx.Fail(w, 400, "expired_token", "Unknown or expired device code; start the login again")
		return
	}
	poll, err := s.Store.PollDevice(r.Context(), in.DeviceCode, s.site(r).PortalOrigin(), func(u store.User, scopes []string, days int) ([]string, *time.Time, error) {
		if u.Disabled {
			return nil, nil, errors.New("account disabled")
		}
		scopes, expires, gerr := tokenGrant(u.Role, scopes, days)
		if gerr != nil {
			return nil, nil, gerr
		}
		return scopes, expires, nil
	})
	if err != nil {
		unavailable(w, err)
		return
	}
	switch poll.State {
	case store.DevicePending:
		httpx.Fail(w, 400, "authorization_pending", "Waiting for approval in the browser")
	case store.DeviceSlow:
		httpx.Fail(w, 400, "slow_down", "Polling too fast; wait longer between polls")
	case store.DeviceDenied:
		httpx.Fail(w, 400, "access_denied", "The login was denied")
	case store.DeviceIssued:
		t := poll.Token
		_ = s.Store.Audit(r.Context(), poll.User.ID, "token.create", t.ID, map[string]any{"name": t.Name, "scopes": t.Scopes, "expiresAt": t.ExpiresAt, "via": "device"})
		httpx.OK(w, map[string]any{"token": poll.Secret, "info": t, "user": meUser(poll.User)})
	default:
		httpx.Fail(w, 400, "expired_token", "Unknown or expired device code; start the login again")
	}
}

// findDevice loads the pending login named in the path on this site. Unknown
// codes spend the caller's lookup budget, so codes cannot be enumerated.
func (s *Server) findDevice(w http.ResponseWriter, r *http.Request) (string, store.DeviceAuthorization, bool) {
	key := "lookup:" + actor(r)
	if !s.deviceLimiter.left(key, 30, time.Hour) {
		httpx.Fail(w, 429, "rate_limited", "Too many unknown codes; try again later")
		return "", store.DeviceAuthorization{}, false
	}
	miss := func() {
		s.deviceLimiter.take(key, 30, time.Hour)
		httpx.Fail(w, 404, "not_found", "This code is invalid or has expired")
	}
	code, ok := store.NormalizeUserCode(r.PathValue("code"))
	if !ok {
		miss()
		return "", store.DeviceAuthorization{}, false
	}
	d, err := s.Store.DeviceRequest(r.Context(), code, s.site(r).PortalOrigin())
	if errors.Is(err, store.ErrNotFound) {
		miss()
		return "", d, false
	}
	if err != nil {
		unavailable(w, err)
		return "", d, false
	}
	return code, d, true
}

// suggestGrant pre-selects the approval form: the requested scopes the role
// allows, or every scope the role allows when the CLI asked for none (the
// default), and a lifetime that admin scopes accept.
func suggestGrant(allowed, requested []string, days int) ([]string, int) {
	out := []string{}
	for _, sc := range requested {
		if slices.Contains(allowed, sc) && !slices.Contains(out, sc) {
			out = append(out, sc)
		}
	}
	if len(requested) == 0 {
		out = append(out, allowed...)
	}
	if slices.ContainsFunc(out, store.IsAdminScope) && (days < 1 || days > maxAdminTokenDays) {
		days = maxAdminTokenDays
	}
	return out, days
}

// deviceRequest describes a pending login to the approval page.
func (s *Server) deviceRequest(w http.ResponseWriter, r *http.Request) {
	code, d, ok := s.findDevice(w, r)
	if !ok {
		return
	}
	allowed := append([]string{}, store.ScopesForRole(me(r).Role)...)
	suggested, days := suggestGrant(allowed, d.RequestedScopes, d.RequestedDays)
	httpx.OK(w, map[string]any{"userCode": store.FormatUserCode(code), "clientName": d.ClientName, "requestedScopes": d.RequestedScopes,
		"requestedDays": d.RequestedDays, "requestedFrom": d.RequesterAddress, "createdAt": d.CreatedAt, "expiresAt": d.ExpiresAt,
		"allowedScopes": allowed, "suggestedScopes": suggested, "suggestedDays": days, "maxAdminDays": maxAdminTokenDays})
}

// deviceApprove grants a pending login within tokenGrant's rules. The token
// is minted when the CLI next polls, after checking the approver again.
func (s *Server) deviceApprove(w http.ResponseWriter, r *http.Request) {
	code, d, ok := s.findDevice(w, r)
	if !ok {
		return
	}
	var in struct {
		Scopes []string `json:"scopes"`
		Days   int      `json:"days"`
	}
	if err := httpx.Read(w, r, &in, 2048); err != nil {
		httpx.Fail(w, 400, "invalid_request", "Invalid JSON body")
		return
	}
	scopes, _, gerr := tokenGrant(me(r).Role, in.Scopes, in.Days)
	if gerr != nil {
		gerr.write(w)
		return
	}
	if err := s.Store.ApproveDevice(r.Context(), code, s.site(r).PortalOrigin(), actor(r), scopes, in.Days); errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "This code is invalid or has expired")
		return
	} else if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "token.device_approve", actor(r), map[string]any{"client": d.ClientName, "scopes": scopes, "days": in.Days, "requestedFrom": d.RequesterAddress})
	httpx.OK(w, map[string]bool{"approved": true})
}

func (s *Server) deviceDeny(w http.ResponseWriter, r *http.Request) {
	code, d, ok := s.findDevice(w, r)
	if !ok {
		return
	}
	if err := s.Store.DenyDevice(r.Context(), code, s.site(r).PortalOrigin(), actor(r)); errors.Is(err, store.ErrNotFound) {
		httpx.Fail(w, 404, "not_found", "This code is invalid or has expired")
		return
	} else if err != nil {
		unavailable(w, err)
		return
	}
	_ = s.Store.Audit(r.Context(), actor(r), "token.device_deny", actor(r), map[string]any{"client": d.ClientName, "requestedFrom": d.RequesterAddress})
	httpx.OK(w, map[string]bool{"denied": true})
}
