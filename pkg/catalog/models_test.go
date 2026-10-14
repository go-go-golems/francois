package catalog

import (
	"testing"

	"github.com/go-go-golems/francois/pkg/piworker"
)

func TestUniqueModels(t *testing.T) {
	glm := piworker.Model{Provider: "lunaroute", ID: "glm-5.3", Name: "GLM 5.3"}
	tiered := func(tier string) piworker.Model { m := glm; m.Tier = tier; return m }
	flash := piworker.Model{Provider: "lunaroute", ID: "glm-5.3-flash"}
	got := uniqueModels([]piworker.Model{tiered("quick"), tiered("default"), tiered("complex"), flash, glm})
	if len(got) != 2 || got[0].ID != "glm-5.3" || got[0].Tier != "" || got[1].ID != "glm-5.3-flash" {
		t.Fatalf("got %+v", got)
	}
}
