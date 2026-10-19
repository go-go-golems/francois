package settings

import (
	"strings"
	"testing"
)

func TestPlanValidation(t *testing.T) {
	base := func() Document {
		d := Defaults()
		d.Models.Allowed = []Model{{Selector: "p/m", Label: "M"}}
		d.Plans[1] = Plan{Key: "subscriber", Name: "Pro", Prices: []PlanPrice{{Interval: "month", PriceID: "price_a"}}, Instances: 3, MembersPerInstance: 2}
		return d
	}
	if d := base(); d.Validate() != nil {
		t.Fatalf("base invalid: %v", d.Validate())
	}
	if defaults := Defaults(); defaults.Validate() != nil {
		t.Fatalf("defaults invalid: %v", defaults.Validate())
	}
	cases := map[string]func(*Document){
		"no free plan":    func(d *Document) { d.Plans = d.Plans[1:] },
		"free with price": func(d *Document) { d.Plans[0].Prices = []PlanPrice{{Interval: "month", PriceID: "price_x"}} },
		"duplicate key":   func(d *Document) { d.Plans[1].Key = "free" },
		"bad key":         func(d *Document) { d.Plans[1].Key = "Pro Plan" },
		"duplicate rank": func(d *Document) {
			d.Plans = append(d.Plans, Plan{Key: "x", Name: "X", MembersPerInstance: 1})
		},
		"bad interval":   func(d *Document) { d.Plans[1].Prices[0].Interval = "week" },
		"not a price id": func(d *Document) { d.Plans[1].Prices[0].PriceID = "prod_x" },
		"repeated price": func(d *Document) {
			d.Plans = append(d.Plans, Plan{Key: "x", Name: "X", MembersPerInstance: 1, Prices: []PlanPrice{{Interval: "year", PriceID: "price_a"}}})
		},
		"zero members":         func(d *Document) { d.Plans[1].MembersPerInstance = 0 },
		"demo bad ttl":         func(d *Document) { d.Demo.TTLMinutes = 1 },
		"long storefront":      func(d *Document) { d.Storefront.Title = strings.Repeat("x", 81) },
		"too many lanes":       func(d *Document) { d.Allowance.GlobalConcurrency = 7 },
		"multiple user calls":  func(d *Document) { d.Allowance.PerUserConcurrency = 2 },
		"invalid storage mode": func(d *Document) { d.Operations.StorageMode = "maybe" },
		"notice too long":      func(d *Document) { d.Operations.Notice = strings.Repeat("x", 501) },
		"missing byte cap":     func(d *Document) { d.Limits.StorageBytesPerInstance = 0 },
		"missing view cap":     func(d *Document) { d.Limits.ViewsPerUser = 0 },
		"tiny media limit":     func(d *Document) { d.Limits.MaxMediaBytes = 10 },
	}
	for name, mutate := range cases {
		d := base()
		mutate(&d)
		if d.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// Arbitrary grant-only classes are rejected, rather than interpreted by rank.
	d := base()
	d.Plans = append(d.Plans, Plan{Key: "friends", Name: "Friends", MembersPerInstance: 1})
	if err := d.Validate(); err == nil {
		t.Fatal("third customer class accepted")
	}

}

func TestParseRejectsOldShape(t *testing.T) {
	old := `{"models":{"allowed":[],"tiers":{}},"allowance":{"dailyTokens":5,"perUserConcurrency":2,"globalConcurrency":8,"maxToolRounds":5},"limits":{"instancesPerUser":10,"membersPerInstance":10,"documentsPerInstance":5000,"assetBytesPerInstance":1,"maxHtmlBytes":2048},"publishing":{}}`
	if _, err := Parse([]byte(old)); err == nil {
		t.Fatal("a pre-plans document must be migrated (store migration 005), not silently accepted")
	}
}
