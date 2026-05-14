package visualizer

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/pkg/craft"
)

// minimalDoc returns the smallest CraftDoc that produces all three diagram
// flavours without errors. Tests extend it as needed.
func minimalDoc() *craft.CraftDoc {
	return &craft.CraftDoc{
		Services: []craft.Service{{
			Name:     "Svc",
			Contexts: []string{"Ctx"},
			Language: "go",
		}},
		UseCases: []craft.UseCase{{
			Name: "test",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type:  craft.TriggerTypeExternal,
					Actor: "User",
					Verb:  "does",
				},
				Actions: []craft.Action{{
					Type:    craft.ActionTypeInternal,
					Context: "Ctx",
					Phrase:  "thing",
				}},
			}},
		}},
		Actors: []craft.Actor{{Name: "User", Type: craft.ActorTypeUser}},
	}
}

func TestGeneratedPUML_HasNoHandwrittenSkinparam(t *testing.T) {
	viz := New()
	// Use FormatPUML for all flavours so the test does not require the
	// `plantuml` binary on $PATH — we only need to inspect the generated
	// PUML source text.
	cases := []struct {
		name string
		gen  func() ([]byte, error)
	}{
		{"domain-detailed", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(
				minimalDoc(), DomainModeDetailed, FormatPUML)
			return b, err
		}},
		{"domain-architecture", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(
				minimalDoc(), DomainModeArchitecture, FormatPUML)
			return b, err
		}},
		{"sequence", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithTypeAndModeAndFormat(
				minimalDoc(), DiagramTypeSequence, DomainModeDetailed, FormatPUML)
			return b, err
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			puml, err := tc.gen()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if strings.Contains(string(puml), "skinparam handwritten") {
				t.Fatalf("generated PUML must not contain `skinparam handwritten` (triggers PlantUML deprecation banner)\n--- PUML ---\n%s", puml)
			}
		})
	}
}
