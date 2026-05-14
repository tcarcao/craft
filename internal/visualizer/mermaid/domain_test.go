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
	// Detailed mode preserves step numbering: 1. thinks (internal), 2. asks (sync)
	if !strings.Contains(out, `BC1 -- "1. thinks" --> BC1`) {
		t.Errorf("expected numbered internal edge for step 1:\n%s", out)
	}
	if !strings.Contains(out, `BC1 -- "2. asks" --> BC2`) {
		t.Errorf("expected numbered sync edge for step 2:\n%s", out)
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
