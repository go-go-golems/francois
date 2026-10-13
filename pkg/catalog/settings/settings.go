// Package settings validates the platform's nonsecret, versioned policy
// document: approved models, allowance, and hosted resource limits.
package settings

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type Model struct {
	// Selector is a Pi provider/model identifier without a thinking suffix.
	Selector string   `json:"selector"`
	Label    string   `json:"label"`
	Thinking []string `json:"thinking"`
	Credits  int      `json:"credits"`
}

type Models struct {
	Allowed []Model           `json:"allowed"`
	Tiers   map[string]string `json:"tiers"`
}

type Allowance struct {
	MonthlyCredits     int `json:"monthlyCredits"`
	DailyCredits       int `json:"dailyCredits"`
	PerUserConcurrency int `json:"perUserConcurrency"`
	GlobalConcurrency  int `json:"globalConcurrency"`
	MaxToolRounds      int `json:"maxToolRounds"`
}

type Limits struct {
	InstancesPerUser      int   `json:"instancesPerUser"`
	MembersPerInstance    int   `json:"membersPerInstance"`
	DocumentsPerInstance  int   `json:"documentsPerInstance"`
	AssetBytesPerInstance int64 `json:"assetBytesPerInstance"`
	MaxHTMLBytes          int64 `json:"maxHtmlBytes"`
}

type Publishing struct {
	// AllowPinnedAfterWithdraw keeps already-pinned instances running when a
	// release is withdrawn from the catalog.
	AllowPinnedAfterWithdraw bool `json:"allowPinnedAfterWithdraw"`
}

type Document struct {
	Models     Models     `json:"models"`
	Allowance  Allowance  `json:"allowance"`
	Limits     Limits     `json:"limits"`
	Publishing Publishing `json:"publishing"`
}

// Defaults is the deployment default used before an administrator saves a
// revision. It allows no models: hosted sampling stays off until approved.
func Defaults() Document {
	return Document{
		Models:     Models{Allowed: []Model{}, Tiers: map[string]string{}},
		Allowance:  Allowance{MonthlyCredits: 0, DailyCredits: 0, PerUserConcurrency: 2, GlobalConcurrency: 8, MaxToolRounds: 5},
		Limits:     Limits{InstancesPerUser: 10, MembersPerInstance: 10, DocumentsPerInstance: 5000, AssetBytesPerInstance: 256 << 20, MaxHTMLBytes: 16 << 20},
		Publishing: Publishing{AllowPinnedAfterWithdraw: true},
	}
}

var selector = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._:-]*$`)
var thinkingLevels = map[string]bool{"off": true, "minimal": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true}

// Parse decodes strictly (unknown fields fail) and validates.
func Parse(raw []byte) (Document, error) {
	var d Document
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, errors.Wrap(err, "settings must be a JSON object with known fields")
	}
	return d, d.Validate()
}

func (d *Document) Validate() error {
	if d.Models.Tiers == nil {
		d.Models.Tiers = map[string]string{}
	}
	if len(d.Models.Allowed) > 32 {
		return errors.New("at most 32 approved models")
	}
	seen := map[string]bool{}
	for i, m := range d.Models.Allowed {
		if !selector.MatchString(m.Selector) || strings.Count(m.Selector, ":") > 0 {
			return errors.Errorf("models.allowed[%d].selector must be provider/model without a thinking suffix", i)
		}
		if seen[m.Selector] {
			return errors.Errorf("model %s is listed twice", m.Selector)
		}
		seen[m.Selector] = true
		if m.Credits < 1 || m.Credits > 1000 {
			return errors.Errorf("model %s credits must be 1–1000", m.Selector)
		}
		if len(m.Label) > 80 {
			return errors.Errorf("model %s label is too long", m.Selector)
		}
		for _, t := range m.Thinking {
			if !thinkingLevels[t] {
				return errors.Errorf("model %s has unknown thinking level %q", m.Selector, t)
			}
		}
	}
	for tier, sel := range d.Models.Tiers {
		if tier != "quick" && tier != "default" && tier != "complex" {
			return errors.Errorf("unknown tier %q", tier)
		}
		if !seen[sel] {
			return errors.Errorf("tier %s maps to %q, which is not an approved model", tier, sel)
		}
	}
	a := d.Allowance
	if a.MonthlyCredits < 0 || a.MonthlyCredits > 1_000_000 || a.DailyCredits < 0 || a.DailyCredits > 1_000_000 {
		return errors.New("allowance credits must be 0–1,000,000")
	}
	if a.PerUserConcurrency < 1 || a.PerUserConcurrency > 16 || a.GlobalConcurrency < 1 || a.GlobalConcurrency > 256 {
		return errors.New("concurrency must be 1–16 per user and 1–256 globally")
	}
	if a.MaxToolRounds < 1 || a.MaxToolRounds > 10 {
		return errors.New("maxToolRounds must be 1–10")
	}
	l := d.Limits
	if l.InstancesPerUser < 1 || l.InstancesPerUser > 1000 || l.MembersPerInstance < 1 || l.MembersPerInstance > 100 {
		return errors.New("instance and member limits must be positive and bounded")
	}
	if l.DocumentsPerInstance < 1 || l.DocumentsPerInstance > 100_000 {
		return errors.New("documentsPerInstance must be 1–100,000")
	}
	if l.AssetBytesPerInstance < 0 || l.AssetBytesPerInstance > 4<<30 {
		return errors.New("assetBytesPerInstance must be 0–4 GiB")
	}
	if l.MaxHTMLBytes < 1024 || l.MaxHTMLBytes > 16<<20 {
		return errors.New("maxHtmlBytes must be 1 KiB–16 MiB")
	}
	return nil
}

// Model finds an approved model by selector (without thinking suffix).
func (d Document) Model(sel string) (Model, bool) {
	for _, m := range d.Models.Allowed {
		if m.Selector == sel {
			return m, true
		}
	}
	return Model{}, false
}
