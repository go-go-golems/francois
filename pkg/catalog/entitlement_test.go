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
	d.Models.Allowed = []settings.Model{{Selector: "p/cheap", Label: "Cheap", Credits: 1}, {Selector: "p/big", Label: "Big", Credits: 5}}
	d.Plans = append(d.Plans,
		settings.Plan{Key: "supporter", Name: "Supporter", Rank: 10, Public: true, Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_sup_m"}, {Interval: "year", PriceID: "price_sup_y"}},
			Instances: 3, MembersPerInstance: 2, MonthlyCredits: 100, Models: []string{"p/cheap"}},
		settings.Plan{Key: "studio", Name: "Studio", Rank: 20, Public: true, Prices: []settings.PlanPrice{{Interval: "month", PriceID: "price_studio_m"}},
			Instances: 10, MembersPerInstance: 10, MonthlyCredits: 1000},
		settings.Plan{Key: "friends", Name: "Friends", Rank: 15, Instances: 5, MembersPerInstance: 5},
	)
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
		{"monthly supporter", store.User{Role: "subscriber"}, sub("price_sup_m"), "supporter", SourceSubscription},
		{"yearly supporter", store.User{Role: "subscriber"}, sub("price_sup_y"), "supporter", SourceSubscription},
		{"studio", store.User{Role: "subscriber"}, sub("price_studio_m"), "studio", SourceSubscription},
		{"unknown price", store.User{Role: "subscriber"}, sub("price_gone"), "free", SourceFree},
		{"trial", store.User{Role: "subscriber"}, billing.Decision{Allowed: true, Reason: billing.ReasonTrial, PriceID: "price_sup_m"}, "supporter", SourceTrial},
		{"grant without plan", store.User{Role: "subscriber"}, comp(""), "studio", SourceComplimentary},
		{"grant with plan", store.User{Role: "subscriber"}, comp("friends"), "friends", SourceComplimentary},
		{"grant with unknown plan", store.User{Role: "subscriber"}, comp("gone"), "studio", SourceComplimentary},
		{"creator", store.User{Role: "creator"}, billing.Decision{}, "studio", SourceStaff},
		{"admin", store.User{Role: "admin"}, billing.Decision{}, "studio", SourceStaff},
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
	art := func(required string) store.Artifact { return store.Artifact{RequiredPlan: required} }
	e := func(plan, source string) Entitlement {
		p, _ := doc.Plan(plan)
		return Entitlement{Plan: p, Source: source}
	}
	free, sup, studio := e("free", SourceFree), e("supporter", SourceSubscription), e("studio", SourceSubscription)
	staff := Entitlement{Plan: doc.Highest(), Source: SourceStaff}
	guest := Entitlement{Plan: demoPlan(doc), Source: SourceGuest}
	disabled := Entitlement{Plan: doc.Free(), Source: SourceDisabled}
	checks := []struct {
		name string
		e    Entitlement
		a    store.Artifact
		want bool
	}{
		{"free runs default artifact", free, art(""), false},
		{"supporter runs default artifact (lowest paid)", sup, art(""), true},
		{"supporter runs studio artifact", sup, art("studio"), false},
		{"studio runs studio artifact", studio, art("studio"), true},
		{"free runs free artifact", free, art("free"), true},
		{"unknown plan falls back to lowest paid", sup, art("gone"), true},
		{"staff run everything", staff, art("studio"), true},
		{"guests run no workspace", guest, art("free"), false},
		{"disabled run nothing", disabled, art("free"), false},
	}
	for _, c := range checks {
		if got := c.e.CanRun(c.a, doc); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if !sup.AllowsModel("p/cheap") || sup.AllowsModel("p/big") || !studio.AllowsModel("p/big") {
		t.Error("plan model restriction")
	}
	if !staff.Paid() || free.Paid() || guest.Paid() || !sup.Paid() {
		t.Error("paid")
	}
	// Without a paid plan, only staff can run artifacts that need one.
	only := settings.Defaults()
	if (Entitlement{Plan: only.Free(), Source: SourceFree}).CanRun(art(""), only) {
		t.Error("free ran an artifact with no paid plan defined")
	}
}

func TestCheckoutPrice(t *testing.T) {
	doc := planDoc()
	cases := []struct {
		plan, interval, want string
		ok                   bool
	}{
		{"supporter", "month", "price_sup_m", true},
		{"supporter", "year", "price_sup_y", true},
		{"supporter", "", "price_sup_m", true},
		{"studio", "year", "", false},
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
