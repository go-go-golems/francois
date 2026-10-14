// Command 03-parse-settings reads a settings document on stdin and parses it
// with the catalog's strict parser, printing the plans. Used to check that
// migration 005 produces valid documents.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/go-go-golems/francois/pkg/catalog/settings"
)

func main() {
	raw, _ := io.ReadAll(os.Stdin)
	d, err := settings.Parse(raw)
	if err != nil {
		fmt.Println("parse error:", err)
		os.Exit(1)
	}
	for _, p := range d.Plans {
		fmt.Printf("plan %s rank=%d prices=%v instances=%d members=%d credits=%d/%d\n", p.Key, p.Rank, p.Prices, p.Instances, p.MembersPerInstance, p.MonthlyCredits, p.DailyCredits)
	}
	fmt.Printf("limits=%+v allowance=%+v demo.enabled=%v\n", d.Limits, d.Allowance, d.Demo.Enabled)
}
