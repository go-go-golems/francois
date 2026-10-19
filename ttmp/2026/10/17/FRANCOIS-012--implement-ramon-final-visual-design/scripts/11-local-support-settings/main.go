// Emit the isolated browser fixture's settings, using production-shaped defaults.
package main

import (
	"encoding/json"
	"github.com/go-go-golems/francois/pkg/catalog/settings"
	"os"
)

func main() {
	doc := settings.Defaults()
	doc.Plans[1] = settings.Plan{Key: "subscriber", Name: "Subscriber", Prices: []settings.PlanPrice{}, Instances: 10, MembersPerInstance: 10, DailyTokens: 2000}
	if err := json.NewEncoder(os.Stdout).Encode(doc); err != nil {
		panic(err)
	}
}
