package catalog

import (
	"net/url"
	"strings"
	"testing"

	"github.com/go-go-golems/francois/pkg/catalog/identity"
)

func TestConfigValidate(t *testing.T) {
	site := func(raw, domain string) Site {
		u, _ := url.Parse(raw)
		return Site{PublicURL: u, AppDomain: domain, OIDC: identity.New(identity.Config{})}
	}
	primary := site("https://francois.example", "apps.francois.example")
	for name, tc := range map[string]struct {
		sites []Site
		err   string
	}{
		"two sites":             {sites: []Site{primary, site("https://slop.example", "slop.example")}},
		"none":                  {err: "at least one site"},
		"path":                  {sites: []Site{site("https://francois.example/x", "apps.example")}, err: "origin"},
		"mixed scheme":          {sites: []Site{primary, site("http://slop.example", "slop.example")}, err: "scheme"},
		"repeated portal":       {sites: []Site{primary, site("https://francois.example", "other.example")}, err: "repeats"},
		"repeated app domain":   {sites: []Site{primary, site("https://slop.example", "apps.francois.example")}, err: "repeats"},
		"wildcard app domain":   {sites: []Site{site("https://slop.example", "*.slop.example")}, err: "application domain"},
		"missing OIDC provider": {sites: []Site{{PublicURL: primary.PublicURL, AppDomain: "apps.example"}}, err: "OIDC"},
	} {
		err := Config{Sites: tc.sites}.Validate()
		if tc.err == "" && err != nil || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("%s: got %v, want %q", name, err, tc.err)
		}
	}
}

func TestInstanceName(t *testing.T) {
	u, _ := url.Parse("https://slop.example")
	s := Site{PublicURL: u, AppDomain: "slop.example"}
	for host, want := range map[string]string{"i-abc.slop.example": "i-abc", "slop.example": "", "a.b.slop.example": "", "i-abc.slop.example.evil": "", "i-abc.slop.example:8443": ""} {
		name, _, ok := s.instanceName(host)
		if name != want || ok != (want != "") {
			t.Errorf("%s: got %q %v, want %q", host, name, ok, want)
		}
	}
	if _, misdirected, _ := s.instanceName("i-abc.slop.example:8443"); !misdirected {
		t.Error("wrong port must be misdirected")
	}
}
