// Emit the isolated browser fixture's settings, using production-shaped defaults.
package main

import (
	"encoding/json"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"os"
)

func main() {
	doc := settings.Defaults()
	doc.Plans = append(doc.Plans, settings.Plan{Key: "subscriber", Name: "Subscriber", Rank: 1, Public: true, Prices: []settings.PlanPrice{}, Highlights: []string{}, Models: []string{}, Instances: 10, MembersPerInstance: 10, MonthlyCredits: 2000})
	if err := json.NewEncoder(os.Stdout).Encode(doc); err != nil {
		panic(err)
	}
}
