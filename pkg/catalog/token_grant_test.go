package catalog

import (
	"slices"
	"testing"
	"time"
)

func TestTokenGrant(t *testing.T) {
	for _, c := range []struct {
		name   string
		role   string
		scopes []string
		days   int
		code   string
		want   []string
	}{
		{"subscriber account", "subscriber", []string{"account"}, 0, "", []string{"account"}},
		{"subscriber studio", "subscriber", []string{"studio"}, 30, "invalid_scopes", nil},
		{"guest", "guest", []string{"account"}, 30, "tokens_unavailable", nil},
		{"no scopes", "creator", nil, 30, "invalid_scopes", nil},
		{"duplicates", "creator", []string{"studio", "studio", "account"}, 30, "", []string{"studio", "account"}},
		{"admin no expiry", "admin", []string{"admin:read"}, 0, "invalid_request", nil},
		{"admin too long", "admin", []string{"admin:write"}, 91, "invalid_request", nil},
		{"admin 90 days", "admin", []string{"studio", "admin:write"}, 90, "", []string{"studio", "admin:write"}},
		{"negative days", "creator", []string{"studio"}, -1, "invalid_request", nil},
		{"too many days", "creator", []string{"studio"}, 367, "invalid_request", nil},
	} {
		scopes, expires, err := tokenGrant(c.role, c.scopes, c.days)
		if c.code != "" {
			if err == nil || err.code != c.code {
				t.Errorf("%s: got %v, want %s", c.name, err, c.code)
			}
			continue
		}
		if err != nil || !slices.Equal(scopes, c.want) {
			t.Errorf("%s: %v %v", c.name, scopes, err)
		}
		if (c.days == 0) != (expires == nil) {
			t.Errorf("%s: expiry %v for %d days", c.name, expires, c.days)
		}
		if expires != nil && time.Until(*expires) < time.Duration(c.days)*24*time.Hour-time.Minute {
			t.Errorf("%s: expiry %v too early", c.name, expires)
		}
	}
}
