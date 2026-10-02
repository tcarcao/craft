package visualizer

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// A context-only focus used to mark every service external, because the
// focused contexts were recorded but never read.
func TestC4_FocusContextKeepsHostingServiceInternal(t *testing.T) {
	doc := &craft.CraftDoc{
		Services: []craft.Service{
			{Name: "Orders", Contexts: []string{"Ordering"}},
			{Name: "Billing", Contexts: []string{"Invoicing"}},
		},
		UseCases: []craft.UseCase{{
			Name: "place order",
			Scenarios: []craft.Scenario{{
				ID:      "s1",
				Trigger: craft.Trigger{Type: craft.TriggerTypeExternal, Actor: "Customer", Verb: "places"},
				Actions: []craft.Action{{
					Type: craft.ActionTypeSync, Context: "Ordering", Verb: "asks", TargetContext: "Invoicing",
				}},
			}},
		}},
	}

	res, err := New(nil).Render(doc, Request{View: ViewC4, Backend: PlantUML, Format: FormatSource,
		C4: &C4Options{FocusContexts: []string{"Ordering"}}})
	if err != nil {
		t.Fatal(err)
	}
	out := string(res.Data)

	if !strings.Contains(out, `System_Boundary(Orders`) {
		t.Errorf("focused context's service Orders should be internal:\n%s", out)
	}
	if strings.Contains(out, `System_Ext(Orders`) {
		t.Errorf("Orders rendered external despite hosting the focused context:\n%s", out)
	}
}
