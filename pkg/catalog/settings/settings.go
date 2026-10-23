// Package settings validates the platform's nonsecret, versioned policy
// document: approved models, allowance, and hosted resource limits.
package settings

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
)

type Model struct {
	// Selector is a Pi provider/model identifier without a thinking suffix.
	Selector string   `json:"selector"`
	Label    string   `json:"label"`
	Thinking []string `json:"thinking"`
}

type Models struct {
	Allowed []Model           `json:"allowed"`
	Tiers   map[string]string `json:"tiers"`
}

type Allowance struct {
	PerUserConcurrency int `json:"perUserConcurrency"`
	GlobalConcurrency  int `json:"globalConcurrency"`
	MaxToolRounds      int `json:"maxToolRounds"`
	MaxOutputTokens    int `json:"maxOutputTokens"`
}

type Operations struct {
	AcceptNewVisits bool   `json:"acceptNewVisits"`
	SamplingEnabled bool   `json:"samplingEnabled"`
	StorageMode     string `json:"storageMode"`
	Notice          string `json:"notice"`
}

// Message returns the operator notice or the operation-specific default.
func (o Operations) Message(fallback string) string {
	if strings.TrimSpace(o.Notice) != "" {
		return o.Notice
	}
	return fallback
}

type Limits struct {
	ViewsPerUser            int   `json:"viewsPerUser"`
	StorageBytesPerInstance int64 `json:"storageBytesPerInstance"`
	DocumentsPerInstance    int   `json:"documentsPerInstance"`
	AssetBytesPerInstance   int64 `json:"assetBytesPerInstance"`
	MaxHTMLBytes            int64 `json:"maxHtmlBytes"`
	MaxMediaBytes           int64 `json:"maxMediaBytes"`
	MaxAttachmentBytes      int64 `json:"maxAttachmentBytes"`
}

type Publishing struct {
	EarlyAccessDays int `json:"earlyAccessDays"`
	// AllowPinnedAfterWithdraw keeps already-pinned instances running when a
	// release is withdrawn from the catalog.
	AllowPinnedAfterWithdraw bool `json:"allowPinnedAfterWithdraw"`
}

// PlanPrice is one way to pay for a plan: a Stripe recurring Price.
type PlanPrice struct {
	Interval string `json:"interval"` // month | year
	PriceID  string `json:"priceId"`
	Label    string `json:"label"` // display text such as "€5 / month"
}

// Plan holds the resource limits for Free or Paying access.
type Plan struct {
	Key                string      `json:"key"`
	Name               string      `json:"name"`
	Prices             []PlanPrice `json:"prices"`
	Instances          int         `json:"instances"`
	MembersPerInstance int         `json:"membersPerInstance"`
	DailyTokens        int         `json:"dailyTokens"`
}

// Demo configures short-lived instances for visitors without an account.
type Demo struct {
	Enabled           bool `json:"enabled"`
	TTLMinutes        int  `json:"ttlMinutes"`
	MaxActive         int  `json:"maxActive"`
	PerUserPerHour    int  `json:"perUserPerHour"`
	PerAddressPerHour int  `json:"perAddressPerHour"`
	Tokens            int  `json:"tokens"`
	DailyBudget       int  `json:"dailyBudget"`
}

// Storefront controls public presentation; the journal is opt-in.
type Storefront struct {
	JournalEnabled bool   `json:"journalEnabled"`
	Title          string `json:"title"`
	Tagline        string `json:"tagline"`
	Intro          string `json:"intro"` // Markdown
	// Copy overrides registered site text (CopyKeys) by key.
	Copy map[string]string `json:"copy,omitempty"`
}

type Document struct {
	Operations Operations `json:"operations"`
	Models     Models     `json:"models"`
	Allowance  Allowance  `json:"allowance"`
	Limits     Limits     `json:"limits"`
	Publishing Publishing `json:"publishing"`
	Plans      []Plan     `json:"plans"`
	Demo       Demo       `json:"demo"`
	Storefront Storefront `json:"storefront"`
}

// Defaults is the deployment default used before an administrator saves a
// revision. It allows no models: hosted sampling stays off until approved.
func Defaults() Document {
	return Document{
		Operations: Operations{AcceptNewVisits: true, SamplingEnabled: true, StorageMode: "read-write"},
		Models:     Models{Allowed: []Model{}, Tiers: map[string]string{}},
		Allowance:  Allowance{PerUserConcurrency: 1, GlobalConcurrency: 6, MaxToolRounds: 5, MaxOutputTokens: 8192},
		Limits:     Limits{ViewsPerUser: 3, StorageBytesPerInstance: 16 << 20, DocumentsPerInstance: 500, AssetBytesPerInstance: 256 << 20, MaxHTMLBytes: 16 << 20, MaxMediaBytes: 25 << 20, MaxAttachmentBytes: 100 << 20},
		Publishing: Publishing{AllowPinnedAfterWithdraw: true},
		Plans:      []Plan{FreePlan(), PayingPlan()},
		Demo:       Demo{PerUserPerHour: 3, TTLMinutes: 20, MaxActive: 20, PerAddressPerHour: 3, Tokens: 5000, DailyBudget: 200000},
	}
}

// FreePlan supplies modest hosted limits to ordinary registered accounts.
func FreePlan() Plan {
	return Plan{Key: "free", Name: "Free", Prices: []PlanPrice{}, Instances: 1, MembersPerInstance: 1, DailyTokens: 10000}
}

// PayingPlan supplies the single higher resource allowance.
func PayingPlan() Plan {
	return Plan{Key: "subscriber", Name: "Paying", Prices: []PlanPrice{}, Instances: 10, MembersPerInstance: 5, DailyTokens: 100000}
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
	if d.Operations.StorageMode != "read-write" && d.Operations.StorageMode != "read-only" && d.Operations.StorageMode != "off" {
		return errors.New("storageMode must be read-write, read-only or off")
	}
	if utf8.RuneCountInString(d.Operations.Notice) > 500 {
		return errors.New("notice must be at most 500 characters")
	}
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
	if a.PerUserConcurrency < 1 || a.PerUserConcurrency > 1 || a.GlobalConcurrency < 1 || a.GlobalConcurrency > 6 {
		return errors.New("concurrency must be 1 per user and 1–6 globally")
	}
	if a.MaxOutputTokens < 1 || a.MaxOutputTokens > 65536 {
		return errors.New("maxOutputTokens must be 1–65536")
	}
	if a.MaxToolRounds < 1 || a.MaxToolRounds > 10 {
		return errors.New("maxToolRounds must be 1–10")
	}
	l := d.Limits
	if l.ViewsPerUser < 1 || l.ViewsPerUser > 20 {
		return errors.New("viewsPerUser must be 1–20")
	}
	if l.StorageBytesPerInstance < 1 || l.StorageBytesPerInstance > 4<<30 {
		return errors.New("storageBytesPerInstance must be 1 byte–4 GiB")
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
	if l.MaxMediaBytes < 1<<20 || l.MaxMediaBytes > 1<<30 {
		return errors.New("maxMediaBytes must be 1–1024 MiB")
	}
	if l.MaxAttachmentBytes < 1<<20 || l.MaxAttachmentBytes > 2<<30 {
		return errors.New("maxAttachmentBytes must be 1 MiB–2 GiB")
	}
	if d.Publishing.EarlyAccessDays < 0 || d.Publishing.EarlyAccessDays > 365 {
		return errors.New("earlyAccessDays must be 0–365")
	}
	if err := d.validatePlans(); err != nil {
		return err
	}
	return d.validateDemo()
}

func (d *Document) validatePlans() error {
	if len(d.Plans) != 2 {
		return errors.New("only Free and Paying classes are supported")
	}
	keys, prices := map[string]bool{}, map[string]bool{}
	for i := range d.Plans {
		p := &d.Plans[i]
		if (p.Key != "free" && p.Key != "subscriber") || keys[p.Key] {
			return errors.New("customer classes must have unique free or subscriber keys")
		}
		keys[p.Key] = true
		if p.Key == "free" {
			p.Name = "Free"
			if len(p.Prices) > 0 {
				return errors.New("Free cannot have payment prices")
			}
		} else {
			p.Name = "Paying"
		}
		if p.Prices == nil {
			p.Prices = []PlanPrice{}
		}
		for _, pr := range p.Prices {
			if (pr.Interval != "month" && pr.Interval != "year") || !strings.HasPrefix(pr.PriceID, "price_") || prices[pr.PriceID] || len(pr.Label) > 40 {
				return errors.New("invalid or repeated Stripe price")
			}
			prices[pr.PriceID] = true
		}
		if p.Instances < 0 || p.Instances > 1000 || p.MembersPerInstance < 1 || p.MembersPerInstance > 100 || p.DailyTokens < 0 || p.DailyTokens > 100000000 {
			return errors.New("class limits: workspaces 0–1000, members 1–100, daily tokens 0–100000000")
		}
	}
	if !keys["free"] || !keys["subscriber"] {
		return errors.New("Free class is required")
	}
	return nil
}

func (d *Document) validateDemo() error {
	m := &d.Demo
	if m.TTLMinutes < 5 || m.TTLMinutes > 24*60 || m.MaxActive < 0 || m.MaxActive > 1000 || m.PerUserPerHour < 1 || m.PerUserPerHour > 100 || m.PerAddressPerHour < 1 || m.PerAddressPerHour > 100 {
		return errors.New("demo: ttlMinutes 5–1440, maxActive 0–1000, perUserPerHour/perAddressPerHour 1–100")
	}
	if m.Tokens < 0 || m.Tokens > 100_000_000 || m.DailyBudget < 0 || m.DailyBudget > 100_000_000 {
		return errors.New("demo: generated tokens and dailyBudget 0–100,000,000")
	}
	f := d.Storefront
	if len(f.Title) > 80 || len(f.Tagline) > 240 || len(f.Intro) > 20<<10 {
		return errors.New("storefront: title ≤80, tagline ≤240 and intro ≤20 KB")
	}
	if err := validateCopy(f.Copy); err != nil {
		return err
	}
	return nil
}

// Plan finds a plan by key.
func (d Document) Plan(key string) (Plan, bool) {
	for _, p := range d.Plans {
		if p.Key == key {
			return p, true
		}
	}
	return Plan{}, false
}

// Free is the free plan (Validate guarantees it exists).
func (d Document) Free() Plan {
	if p, ok := d.Plan("free"); ok {
		return p
	}
	return FreePlan()
}

// Paying returns the designated Paying resource limits.
func (d Document) Paying() Plan {
	p, ok := d.Plan("subscriber")
	if ok {
		return p
	}
	return d.Free()
}

// PlanForPrice finds the plan a Stripe price belongs to.
func (d Document) PlanForPrice(priceID string) (Plan, bool) {
	for _, p := range d.Plans {
		for _, pr := range p.Prices {
			if pr.PriceID == priceID {
				return p, true
			}
		}
	}
	return Plan{}, false
}

// Prices lists every plan price, the set of prices that grant access.
func (d Document) Prices() map[string]bool {
	out := map[string]bool{}
	for _, p := range d.Plans {
		for _, pr := range p.Prices {
			out[pr.PriceID] = true
		}
	}
	return out
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
