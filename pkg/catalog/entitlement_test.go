package catalog

import (
	"testing"
	"time"

	"github.com/go-go-golems/francois/pkg/catalog/billing"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"github.com/go-go-golems/francois/pkg/catalog/store"
)

func planDoc() settings.Document {
	d := settings.Defaults()
	d.Models.Allowed = []settings.Model{{Selector: "p/cheap", Label: "Cheap"}, {Selector: "p/big", Label: "Big"}}
	d.Plans[1] = settings.Plan{Key: "subscriber", Name: "Paying", Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_sup_m"}, {Interval: "year", PriceID: "price_sup_y"}, {Interval: "month", PriceID: "price_studio_m"}}, Instances: 10, MembersPerInstance: 10, DailyTokens: 1000}
	if err := d.Validate(); err != nil {
		panic(err)
	}
	return d
}

func TestResolveEntitlement(t *testing.T) {
	doc := planDoc()
	until := time.Now().Add(time.Hour)
	sub := func(price string) billing.Decision {
		return billing.Decision{Allowed: true, Reason: billing.ReasonSubscriber, PriceID: price, Until: &until}
	}
	comp := func(plan string) billing.Decision {
		return billing.Decision{Allowed: true, Reason: billing.ReasonComplimentary, Plan: plan, Until: &until}
	}
	cases := []struct {
		name         string
		user         store.User
		d            billing.Decision
		plan, source string
	}{
		{"free", store.User{Role: "subscriber"}, billing.Decision{Reason: billing.ReasonRequired}, "free", SourceFree},
		{"monthly supporter", store.User{Role: "subscriber"}, sub("price_sup_m"), "subscriber", SourceSubscription},
		{"yearly supporter", store.User{Role: "subscriber"}, sub("price_sup_y"), "subscriber", SourceSubscription},
		{"subscriber", store.User{Role: "subscriber"}, sub("price_studio_m"), "subscriber", SourceSubscription},
		{"approved inline price", store.User{Role: "subscriber"}, sub("price_inline"), "subscriber", SourceSubscription},
		{"trial", store.User{Role: "subscriber"}, billing.Decision{Allowed: true, Reason: billing.ReasonTrial, PriceID: "price_sup_m"}, "subscriber", SourceTrial},
		{"grant without plan", store.User{Role: "subscriber"}, comp(""), "subscriber", SourceComplimentary},
		{"grant with plan", store.User{Role: "subscriber"}, comp("friends"), "subscriber", SourceComplimentary},
		{"grant with unknown plan", store.User{Role: "subscriber"}, comp("gone"), "subscriber", SourceComplimentary},
		{"creator", store.User{Role: "creator"}, billing.Decision{}, "subscriber", SourceStaff},
		{"admin", store.User{Role: "admin"}, billing.Decision{}, "subscriber", SourceStaff},
		{"guest", store.User{Role: "guest"}, billing.Decision{}, "demo", SourceGuest},
		{"disabled subscriber", store.User{Role: "subscriber", Disabled: true}, sub("price_studio_m"), "free", SourceDisabled},
		{"disabled admin", store.User{Role: "admin", Disabled: true}, billing.Decision{}, "free", SourceDisabled},
	}
	for _, c := range cases {
		e := resolveEntitlement(c.user, c.d, doc)
		if e.Plan.Key != c.plan || e.Source != c.source {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, e.Plan.Key, e.Source, c.plan, c.source)
		}
	}
}

func TestCanRunAndModels(t *testing.T) {
	doc := planDoc()
	for _, source := range []string{SourceFree, SourceSubscription, SourceComplimentary, SourceStaff, SourceGuest, SourceDisabled} {
		e := Entitlement{Plan: doc.Free(), Source: source}
		want := source != SourceGuest && source != SourceDisabled
		if e.CanRun(store.Artifact{}, doc) != want {
			t.Errorf("%s access", source)
		}
		if !e.AllowsModel("p/big") {
			t.Errorf("%s should use the global model policy", source)
		}
	}
}

func TestCheckoutPrice(t *testing.T) {
	doc := planDoc()
	cases := []struct {
		plan, interval, want string
		ok                   bool
	}{
		{"subscriber", "month", "price_sup_m", true},
		{"subscriber", "year", "price_sup_y", true},
		{"subscriber", "", "price_sup_m", true},

		{"free", "", "", false},
		{"friends", "", "", false},
		{"nope", "", "", false},
	}
	for _, c := range cases {
		got, ok := checkoutPrice(doc, c.plan, c.interval)
		if got != c.want || ok != c.ok {
			t.Errorf("%s/%s: %q %v", c.plan, c.interval, got, ok)
		}
	}
}

func TestSubscriberNeverSelectsGrantOnlyPlan(t *testing.T) {
	doc := planDoc()
	doc.Plans = append(doc.Plans, settings.Plan{Key: "private", Name: "Private", MembersPerInstance: 1, DailyTokens: 999999})
	until := time.Now().Add(time.Hour)
	got := resolveEntitlement(store.User{Role: "subscriber"}, billing.Decision{Allowed: true, Reason: billing.ReasonSubscriber, Until: &until}, doc)
	if got.Plan.Key != "subscriber" {
		t.Fatalf("subscriber selected %q", got.Plan.Key)
	}
	if err := doc.Validate(); err == nil {
		t.Fatal("private third class accepted")
	}
}
