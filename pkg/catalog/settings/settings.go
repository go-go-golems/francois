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
	Credits  int      `json:"credits"`
}

type Models struct {
	Allowed []Model           `json:"allowed"`
	Tiers   map[string]string `json:"tiers"`
}

type Allowance struct {
	PerUserConcurrency int `json:"perUserConcurrency"`
	GlobalConcurrency  int `json:"globalConcurrency"`
	MaxToolRounds      int `json:"maxToolRounds"`
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

// Plan is a subscription tier. Ranks order plans: a plan includes every
// artifact that requires a plan of equal or lower rank. The free plan has
// rank 0 and no prices; other plans without prices can only be granted.
type Plan struct {
	Key                string      `json:"key"`
	Name               string      `json:"name"`
	Rank               int         `json:"rank"`
	Description        string      `json:"description"`
	Highlights         []string    `json:"highlights"`
	Public             bool        `json:"public"`
	Prices             []PlanPrice `json:"prices"`
	Instances          int         `json:"instances"`
	MembersPerInstance int         `json:"membersPerInstance"`
	MonthlyCredits     int         `json:"monthlyCredits"`
	DailyCredits       int         `json:"dailyCredits"`
	// Models restricts approved selectors; empty allows every approved model.
	Models []string `json:"models"`
}

// Demo configures short-lived instances for visitors without an account.
type Demo struct {
	Enabled           bool     `json:"enabled"`
	TTLMinutes        int      `json:"ttlMinutes"`
	MaxActive         int      `json:"maxActive"`
	PerUserPerHour    int      `json:"perUserPerHour"`
	PerAddressPerHour int      `json:"perAddressPerHour"`
	Credits           int      `json:"credits"`
	DailyBudget       int      `json:"dailyBudget"`
	Models            []string `json:"models"`
}

// Storefront is the public wording of the front page.
type Storefront struct {
	Title   string `json:"title"`
	Tagline string `json:"tagline"`
	Intro   string `json:"intro"` // Markdown
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
		Allowance:  Allowance{PerUserConcurrency: 1, GlobalConcurrency: 6, MaxToolRounds: 5},
		Limits:     Limits{ViewsPerUser: 3, StorageBytesPerInstance: 16 << 20, DocumentsPerInstance: 500, AssetBytesPerInstance: 256 << 20, MaxHTMLBytes: 16 << 20, MaxMediaBytes: 25 << 20, MaxAttachmentBytes: 100 << 20},
		Publishing: Publishing{AllowPinnedAfterWithdraw: true},
		Plans:      []Plan{FreePlan()},
		Demo:       Demo{PerUserPerHour: 3, TTLMinutes: 20, MaxActive: 20, PerAddressPerHour: 3, Credits: 5, DailyBudget: 200, Models: []string{}},
	}
}

// FreePlan is the plan of every account without paid or granted access:
// browse, read and try demos, but no workspaces and no model credits.
func FreePlan() Plan {
	return Plan{Key: "free", Name: "Free", Public: true, Highlights: []string{}, Prices: []PlanPrice{}, MembersPerInstance: 1, Models: []string{}}
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
	if a.PerUserConcurrency < 1 || a.PerUserConcurrency > 1 || a.GlobalConcurrency < 1 || a.GlobalConcurrency > 6 {
		return errors.New("concurrency must be 1 per user and 1–6 globally")
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
	if err := d.validatePlans(seen); err != nil {
		return err
	}
	return d.validateDemo(seen)
}

var planKey = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

func (d *Document) validatePlans(approved map[string]bool) error {
	if len(d.Plans) == 0 || len(d.Plans) > 12 {
		return errors.New("define between one and twelve plans, including the free plan")
	}
	keys, ranks, prices := map[string]bool{}, map[int]bool{}, map[string]bool{}
	free := false
	for i := range d.Plans {
		p := &d.Plans[i]
		if p.Highlights == nil {
			p.Highlights = []string{}
		}
		if p.Prices == nil {
			p.Prices = []PlanPrice{}
		}
		if p.Models == nil {
			p.Models = []string{}
		}
		if !planKey.MatchString(p.Key) || keys[p.Key] {
			return errors.Errorf("plan %d needs a unique key of lowercase letters, digits and -", i)
		}
		keys[p.Key] = true
		if p.Name == "" || len(p.Name) > 40 || len(p.Description) > 400 || len(p.Highlights) > 8 {
			return errors.Errorf("plan %s needs a name (≤40), a description (≤400) and at most 8 highlights", p.Key)
		}
		for _, h := range p.Highlights {
			if len(h) > 120 {
				return errors.Errorf("plan %s has a highlight longer than 120 characters", p.Key)
			}
		}
		if p.Rank < 0 || p.Rank > 100 || ranks[p.Rank] {
			return errors.Errorf("plan %s needs a unique rank between 0 and 100", p.Key)
		}
		ranks[p.Rank] = true
		if p.Key == "free" {
			free = true
			if p.Rank != 0 || len(p.Prices) > 0 {
				return errors.New("the free plan has rank 0 and no prices")
			}
		} else if p.Rank == 0 {
			return errors.Errorf("rank 0 is the free plan's; plan %s needs a higher rank", p.Key)
		}
		for _, pr := range p.Prices {
			if (pr.Interval != "month" && pr.Interval != "year") || !strings.HasPrefix(pr.PriceID, "price_") || prices[pr.PriceID] || len(pr.Label) > 40 {
				return errors.Errorf("plan %s has an invalid or repeated price (interval month|year, a unique price_… ID, label ≤40)", p.Key)
			}
			prices[pr.PriceID] = true
		}
		if p.Instances < 0 || p.Instances > 1000 || p.MembersPerInstance < 1 || p.MembersPerInstance > 100 {
			return errors.Errorf("plan %s: instances 0–1000 and members per instance 1–100", p.Key)
		}
		if p.MonthlyCredits < 0 || p.MonthlyCredits > 1_000_000 || p.DailyCredits < 0 || p.DailyCredits > 1_000_000 {
			return errors.Errorf("plan %s: credits must be 0–1,000,000", p.Key)
		}
		for _, m := range p.Models {
			if !approved[m] {
				return errors.Errorf("plan %s allows %q, which is not an approved model", p.Key, m)
			}
		}
	}
	if !free {
		return errors.New("a plan with key free is required")
	}
	return nil
}

func (d *Document) validateDemo(approved map[string]bool) error {
	m := &d.Demo
	if m.Models == nil {
		m.Models = []string{}
	}
	if m.TTLMinutes < 5 || m.TTLMinutes > 24*60 || m.MaxActive < 0 || m.MaxActive > 1000 || m.PerUserPerHour < 1 || m.PerUserPerHour > 100 || m.PerAddressPerHour < 1 || m.PerAddressPerHour > 100 {
		return errors.New("demo: ttlMinutes 5–1440, maxActive 0–1000, perUserPerHour/perAddressPerHour 1–100")
	}
	if m.Credits < 0 || m.Credits > 10_000 || m.DailyBudget < 0 || m.DailyBudget > 1_000_000 {
		return errors.New("demo: credits 0–10,000 and dailyBudget 0–1,000,000")
	}
	for _, sel := range m.Models {
		if !approved[sel] {
			return errors.Errorf("demo allows %q, which is not an approved model", sel)
		}
	}
	f := d.Storefront
	if len(f.Title) > 80 || len(f.Tagline) > 240 || len(f.Intro) > 20<<10 {
		return errors.New("storefront: title ≤80, tagline ≤240 and intro ≤20 KB")
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

// Highest is the plan with the highest rank.
func (d Document) Highest() Plan {
	best := d.Free()
	for _, p := range d.Plans {
		if p.Rank > best.Rank {
			best = p
		}
	}
	return best
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

// RequiredRank is the rank an artifact requires: its named plan's rank, or,
// when the key is empty or unknown, the lowest rank above free. With no
// plan above free, only staff can run it.
func (d Document) RequiredRank(key string) int {
	if p, ok := d.Plan(key); ok && key != "" {
		return p.Rank
	}
	lowest := -1
	for _, p := range d.Plans {
		if p.Rank > 0 && (lowest < 0 || p.Rank < lowest) {
			lowest = p.Rank
		}
	}
	if lowest < 0 {
		return 1 << 30
	}
	return lowest
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
