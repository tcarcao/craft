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

func TestGeneratedPUML_DeclaresUnicodeCapableFont(t *testing.T) {
	viz := New()
	doc := minimalDoc()
	// Inject an em-dash into a use case name so the test fixture exercises the path.
	doc.UseCases[0].Name = "Time-based — scheduled"
	cases := []struct {
		name string
		gen  func() ([]byte, error)
	}{
		{"domain-detailed", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(doc, DomainModeDetailed, FormatPUML)
			return b, err
		}},
		{"domain-architecture", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithModeAndFormat(doc, DomainModeArchitecture, FormatPUML)
			return b, err
		}},
		{"sequence", func() ([]byte, error) {
			b, _, err := viz.GenerateDomainDiagramWithTypeAndModeAndFormat(
				doc, DiagramTypeSequence, DomainModeDetailed, FormatPUML)
			return b, err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			puml, err := tc.gen()
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if !strings.Contains(string(puml), "skinparam defaultFontName") {
				t.Fatalf("generated PUML must declare a Unicode-capable defaultFontName\n--- PUML ---\n%s", puml)
			}
		})
	}
}

func TestDomainDiagram_DomainsNotClassifiedAsActors(t *testing.T) {
	// Model where a bounded context ("BC1") is also a trigger actor —
	// reproduces the "VASFulfillment as stickman" bug from vas.craft.
	doc := &craft.CraftDoc{
		Services: []craft.Service{{
			Name:     "Svc",
			Contexts: []string{"BC1"},
		}},
		UseCases: []craft.UseCase{{
			Name: "starts",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type:  craft.TriggerTypeExternal,
					Actor: "BC1",
					Verb:  "begins",
				},
				Actions: []craft.Action{{
					Type:    craft.ActionTypeInternal,
					Context: "BC1",
					Phrase:  "work",
				}},
			}},
		}},
	}
	puml, _, err := New().GenerateDomainDiagramWithModeAndFormat(doc, DomainModeDetailed, FormatPUML)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	pumlStr := string(puml)
	// BC1 must be declared as a frame (domain), never as `actor BC1` /
	// `boundary BC1` / `participant BC1`.
	forbidden := []string{"actor BC1", "boundary BC1", "participant BC1"}
	for _, f := range forbidden {
		if strings.Contains(pumlStr, f) {
			t.Fatalf("domain BC1 leaked into actor declarations as %q\n--- PUML ---\n%s", f, pumlStr)
		}
	}
	if !strings.Contains(pumlStr, `frame "BC1"`) {
		t.Fatalf("domain BC1 missing frame declaration\n--- PUML ---\n%s", pumlStr)
	}
}

func TestDomainArchitectureDiagram_NoSelfLoops(t *testing.T) {
	doc := &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1", "BC2"}}},
		UseCases: []craft.UseCase{{
			Name: "u",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type: craft.TriggerTypeExternal, Actor: "User", Verb: "x",
				},
				Actions: []craft.Action{
					{Type: craft.ActionTypeInternal, Context: "BC1", Phrase: "thinks"},
					{Type: craft.ActionTypeSync, Context: "BC1", TargetContext: "BC2", Phrase: "asks"},
				},
			}},
		}},
		Actors: []craft.Actor{{Name: "User", Type: craft.ActorTypeUser}},
	}
	puml, _, err := New().GenerateDomainDiagramWithModeAndFormat(doc, DomainModeArchitecture, FormatPUML)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Scan each connection line. A self-loop has identical source and dest aliases.
	for _, line := range strings.Split(string(puml), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, " --> ") {
			continue
		}
		// Format: "alias --> alias" (optionally followed by a label)
		parts := strings.SplitN(line, " --> ", 2)
		if len(parts) != 2 {
			continue
		}
		from := strings.TrimSpace(parts[0])
		to := strings.TrimSpace(parts[1])
		// Drop everything after the second token in `to` (any " : label" or trailing tokens).
		if idx := strings.Index(to, " "); idx > 0 {
			to = to[:idx]
		}
		if from == to {
			t.Fatalf("self-loop %q in architecture-mode domain diagram\n--- PUML ---\n%s", line, puml)
		}
	}
}

func TestC4Diagram_IncludesCRONActor(t *testing.T) {
	doc := &craft.CraftDoc{
		Services: []craft.Service{{
			Name:     "Sched",
			Contexts: []string{"Scheduler"},
		}},
		UseCases: []craft.UseCase{{
			Name: "tick",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type:  craft.TriggerTypeExternal,
					Actor: "CRON",
					Verb:  "fires",
				},
				Actions: []craft.Action{{
					Type:    craft.ActionTypeInternal,
					Context: "Scheduler",
					Phrase:  "ticks",
				}},
			}},
		}},
		Actors: []craft.Actor{{Name: "CRON", Type: craft.ActorTypeSystem}},
	}
	puml, _, err := New().GenerateC4WithFormat(doc, C4ModeBoundaries, false, FormatPUML)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(string(puml), "System_Ext(CRON,") {
		t.Fatalf("CRON actor missing from generated C4 PUML\n--- PUML ---\n%s", puml)
	}
}
