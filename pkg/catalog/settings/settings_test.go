package settings

import (
	"strings"
	"testing"
)

func TestPlanValidation(t *testing.T) {
	base := func() Document {
		d := Defaults()
		d.Models.Allowed = []Model{{Selector: "p/m", Label: "M", Credits: 1}}
		d.Plans = append(d.Plans, Plan{Key: "pro", Name: "Pro", Rank: 1, Prices: []PlanPrice{{Interval: "month", PriceID: "price_a"}}, Instances: 3, MembersPerInstance: 2})
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
		"free with rank":  func(d *Document) { d.Plans[0].Rank = 3 },
		"paid at rank 0":  func(d *Document) { d.Plans[1].Rank = 0 },
		"duplicate key":   func(d *Document) { d.Plans[1].Key = "free" },
		"bad key":         func(d *Document) { d.Plans[1].Key = "Pro Plan" },
		"duplicate rank": func(d *Document) {
			d.Plans = append(d.Plans, Plan{Key: "x", Name: "X", Rank: 1, MembersPerInstance: 1})
		},
		"bad interval":   func(d *Document) { d.Plans[1].Prices[0].Interval = "week" },
		"not a price id": func(d *Document) { d.Plans[1].Prices[0].PriceID = "prod_x" },
		"repeated price": func(d *Document) {
			d.Plans = append(d.Plans, Plan{Key: "x", Name: "X", Rank: 2, MembersPerInstance: 1, Prices: []PlanPrice{{Interval: "year", PriceID: "price_a"}}})
		},
		"unapproved model": func(d *Document) { d.Plans[1].Models = []string{"p/other"} },
		"zero members":     func(d *Document) { d.Plans[1].MembersPerInstance = 0 },
		"demo bad ttl":     func(d *Document) { d.Demo.TTLMinutes = 1 },
		"demo unapproved":  func(d *Document) { d.Demo.Models = []string{"p/other"} },
		"long storefront":  func(d *Document) { d.Storefront.Title = strings.Repeat("x", 81) },
		"tiny media limit": func(d *Document) { d.Limits.MaxMediaBytes = 10 },
	}
	for name, mutate := range cases {
		d := base()
		mutate(&d)
		if d.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// A paid-rank plan without prices is valid: it can only be granted.
	d := base()
	d.Plans = append(d.Plans, Plan{Key: "friends", Name: "Friends", Rank: 5, MembersPerInstance: 1})
	if err := d.Validate(); err != nil {
		t.Fatalf("grant-only plan: %v", err)
	}
	if d.RequiredRank("") != 1 || d.RequiredRank("friends") != 5 || d.RequiredRank("nope") != 1 || d.Highest().Key != "friends" {
		t.Fatalf("ranks: %d %d %d %s", d.RequiredRank(""), d.RequiredRank("friends"), d.RequiredRank("nope"), d.Highest().Key)
	}
	if p, ok := d.PlanForPrice("price_a"); !ok || p.Key != "pro" || !d.Prices()["price_a"] {
		t.Fatal("price lookup")
	}
}

func TestParseRejectsOldShape(t *testing.T) {
	old := `{"models":{"allowed":[],"tiers":{}},"allowance":{"monthlyCredits":5,"perUserConcurrency":2,"globalConcurrency":8,"maxToolRounds":5},"limits":{"instancesPerUser":10,"membersPerInstance":10,"documentsPerInstance":5000,"assetBytesPerInstance":1,"maxHtmlBytes":2048},"publishing":{}}`
	if _, err := Parse([]byte(old)); err == nil {
		t.Fatal("a pre-plans document must be migrated (store migration 005), not silently accepted")
	}
}
