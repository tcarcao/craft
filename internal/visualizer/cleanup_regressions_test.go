package visualizer

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func TestLookupLanguage_MatchesWholeWordsOnly(t *testing.T) {
	cases := map[string]string{
		"go":          "go",
		"Golang/gin":  "go",
		"java":        "java",
		"javascript":  "nodejs",
		"Node.js":     "nodejs",
		"TypeScript":  "javascript",
		"C# 12":       "dot_net",
		"django":      "", // must not match "go"
		"mongo-shell": "", // must not match "go"
	}
	for lang, want := range cases {
		l, ok := lookupLanguage(lang)
		if want == "" {
			if ok {
				t.Errorf("lookupLanguage(%q) = %q, want no match", lang, l.sprite)
			}
			continue
		}
		if !ok || l.sprite != want {
			t.Errorf("lookupLanguage(%q) = %q (ok=%v), want %q", lang, l.sprite, ok, want)
		}
	}
}

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

	out := GenerateC4ContainerDiagramWithFocusAndContexts(doc, C4ModeTransparent, nil, []string{"Ordering"}, false)

	if !strings.Contains(out, `System_Boundary(Orders`) {
		t.Errorf("focused context's service Orders should be internal:\n%s", out)
	}
	if strings.Contains(out, `System_Ext(Orders`) {
		t.Errorf("Orders rendered external despite hosting the focused context:\n%s", out)
	}
}
