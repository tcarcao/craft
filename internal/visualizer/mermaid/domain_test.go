package mermaid

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/pkg/craft"
)

func domainFixture() *craft.CraftDoc {
	return &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1", "BC2"}}},
		Actors:   []craft.Actor{{Name: "User", Type: craft.ActorTypeUser}},
		UseCases: []craft.UseCase{{
			Name: "Alpha flow",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type: craft.TriggerTypeExternal, Actor: "User", Verb: "starts",
				},
				Actions: []craft.Action{
					{Type: craft.ActionTypeInternal, Context: "BC1", Phrase: "thinks"},
					{Type: craft.ActionTypeSync, Context: "BC1", TargetContext: "BC2", Phrase: "asks"},
				},
			}},
		}},
	}
}

func TestDomainMermaid_DetailedHeader(t *testing.T) {
	out, err := Domain(domainFixture(), false)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	if !strings.HasPrefix(out, "flowchart LR\n") {
		t.Errorf("expected output to start with 'flowchart LR\\n', got:\n%s", out)
	}
}

func TestDomainMermaid_DetailedEmitsNumberedEdges(t *testing.T) {
	out, err := Domain(domainFixture(), false)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	// Detailed mode preserves step numbering: position 1 is the trigger
	// (User starts -> BC1), then 2. thinks (internal), 3. asks (sync).
	if !strings.Contains(out, `BC1 -- "2. thinks" --> BC1`) {
		t.Errorf("expected numbered internal edge for step 2:\n%s", out)
	}
	if !strings.Contains(out, `BC1 -- "3. asks" --> BC2`) {
		t.Errorf("expected numbered sync edge for step 3:\n%s", out)
	}
}

func TestDomainMermaid_ArchitectureUsesSubgraphs(t *testing.T) {
	out, err := Domain(domainFixture(), true) // architecture mode
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	if !strings.Contains(out, `subgraph Svc["Svc"]`) {
		t.Errorf("expected service subgraph wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, "BC1") || !strings.Contains(out, "BC2") {
		t.Errorf("expected both contexts inside subgraph, got:\n%s", out)
	}
}

func TestDomainMermaid_ArchitectureDropsSelfLoops(t *testing.T) {
	out, err := Domain(domainFixture(), true)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	// Self-loops (X --> X) are noise in architecture mode — must be filtered.
	if strings.Contains(out, "BC1 --> BC1") {
		t.Errorf("self-loop emitted in architecture mode:\n%s", out)
	}
}

func TestDomainMermaid_EmptyDoc(t *testing.T) {
	out, err := Domain(&craft.CraftDoc{}, false)
	if err != nil {
		t.Fatalf("empty doc: %v", err)
	}
	if !strings.HasPrefix(out, "flowchart LR\n") {
		t.Errorf("empty doc must still emit header, got:\n%s", out)
	}
}

func TestDomainMermaid_ExternalTriggerEdge(t *testing.T) {
	// Reproduces the "CRON floats disconnected" bug from vas.craft.
	doc := &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1"}}},
		Actors:   []craft.Actor{{Name: "CRON", Type: craft.ActorTypeSystem}},
		UseCases: []craft.UseCase{{
			Name: "Tick",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type: craft.TriggerTypeExternal, Actor: "CRON", Verb: "advances", Phrase: "schedule",
				},
				Actions: []craft.Action{
					{Type: craft.ActionTypeInternal, Context: "BC1", Phrase: "does work"},
				},
			}},
		}},
	}
	out, err := Domain(doc, false)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	want := `CRON -- "1. advances schedule" --> BC1`
	if !strings.Contains(out, want) {
		t.Fatalf("expected external trigger edge %q in output:\n%s", want, out)
	}
	if !strings.Contains(out, `BC1 -- "2. does work" --> BC1`) {
		t.Fatalf("expected action step 2 in output:\n%s", out)
	}
}

func TestDomainMermaid_OmitsUnreferencedActors(t *testing.T) {
	// Model declares two actors but only Bob triggers a scenario.
	// Alice must NOT appear in the rendered nodes.
	doc := &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1"}}},
		Actors: []craft.Actor{
			{Name: "Bob", Type: craft.ActorTypeUser},
			{Name: "Alice", Type: craft.ActorTypeUser},
		},
		UseCases: []craft.UseCase{{
			Name: "Alpha",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type: craft.TriggerTypeExternal, Actor: "Bob", Verb: "starts",
				},
				Actions: []craft.Action{
					{Type: craft.ActionTypeInternal, Context: "BC1", Phrase: "work"},
				},
			}},
		}},
	}
	out, err := Domain(doc, false)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	if !strings.Contains(out, `Bob["Bob"]`) {
		t.Fatalf("expected Bob node (the triggering actor) in output:\n%s", out)
	}
	if strings.Contains(out, `Alice["Alice"]`) {
		t.Fatalf("Alice should not appear (no scenario triggers her):\n%s", out)
	}
}

func TestDomainMermaid_ListenTriggerRoutedThroughPublisher(t *testing.T) {
	doc := &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1", "BC2"}}},
		Actors:   []craft.Actor{{Name: "User", Type: craft.ActorTypeUser}},
		UseCases: []craft.UseCase{
			{
				Name: "Publish",
				Scenarios: []craft.Scenario{{
					ID: "s1",
					Trigger: craft.Trigger{
						Type: craft.TriggerTypeExternal, Actor: "User", Verb: "starts",
					},
					Actions: []craft.Action{
						{Type: craft.ActionTypeAsync, Context: "BC1", Event: "EventX"},
					},
				}},
			},
			{
				Name: "Consume",
				Scenarios: []craft.Scenario{{
					ID: "s2",
					Trigger: craft.Trigger{
						Type:    craft.TriggerTypeDomainListen,
						Context: "BC2",
						Event:   "EventX",
					},
					Actions: []craft.Action{
						{Type: craft.ActionTypeInternal, Context: "BC2", Phrase: "handles"},
					},
				}},
			},
		},
	}
	out, err := Domain(doc, false)
	if err != nil {
		t.Fatalf("Domain: %v", err)
	}
	if !strings.Contains(out, `BC1 -- "3. EventX" --> BC2`) {
		t.Fatalf("expected listen-trigger edge BC1 -> BC2 (EventX) in output:\n%s", out)
	}
}
