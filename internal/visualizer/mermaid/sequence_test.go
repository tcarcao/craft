package mermaid

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// sequenceFixture builds a minimal model exercising actor + listen trigger +
// sync action + async notification — covers the four edge shapes the
// sequence generator must emit.
func sequenceFixture() *craft.CraftDoc {
	return &craft.CraftDoc{
		Services: []craft.Service{{Name: "Svc", Contexts: []string{"BC1", "BC2"}}},
		Actors:   []craft.Actor{{Name: "User", Type: craft.ActorTypeUser}},
		UseCases: []craft.UseCase{
			{
				Name: "Alpha flow",
				Scenarios: []craft.Scenario{{
					ID: "s1",
					Trigger: craft.Trigger{
						Type: craft.TriggerTypeExternal, Actor: "User", Verb: "starts", Phrase: "things",
					},
					Actions: []craft.Action{
						{Type: craft.ActionTypeSync, Context: "BC1", TargetContext: "BC2", Phrase: "asks for"},
						{Type: craft.ActionTypeAsync, Context: "BC2", Event: "EventX"},
					},
				}},
			},
		},
	}
}

func TestSequenceMermaid_Header(t *testing.T) {
	out, err := Sequence(sequenceFixture())
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	if !strings.HasPrefix(out, "sequenceDiagram\n") {
		t.Errorf("expected output to start with 'sequenceDiagram\\n', got:\n%s", out)
	}
}

func TestSequenceMermaid_DeclaresParticipants(t *testing.T) {
	out, err := Sequence(sequenceFixture())
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	for _, name := range []string{"User", "BC1", "BC2"} {
		needle := "participant " + name
		if !strings.Contains(out, needle) {
			t.Errorf("expected %q in output:\n%s", needle, out)
		}
	}
}

func TestSequenceMermaid_EmitsSyncArrow(t *testing.T) {
	out, err := Sequence(sequenceFixture())
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	if !strings.Contains(out, "BC1->>BC2: asks for") {
		t.Errorf("expected sync arrow 'BC1->>BC2: asks for', got:\n%s", out)
	}
}

func TestSequenceMermaid_EmitsAsyncNotification(t *testing.T) {
	out, err := Sequence(sequenceFixture())
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	if !strings.Contains(out, "BC2->>BC2: notifies \"EventX\"") {
		t.Errorf("expected async self-notify line, got:\n%s", out)
	}
}

func TestSequenceMermaid_UseCaseAsNoteOver(t *testing.T) {
	out, err := Sequence(sequenceFixture())
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	if !strings.Contains(out, "Note over User,BC2: Alpha flow") {
		t.Errorf("expected use-case Note over divider, got:\n%s", out)
	}
}

func TestSequenceMermaid_OmitsUnreferencedActors(t *testing.T) {
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
	out, err := Sequence(doc)
	if err != nil {
		t.Fatalf("Sequence: %v", err)
	}
	if !strings.Contains(out, "participant Bob") {
		t.Fatalf("expected 'participant Bob' (the triggering actor):\n%s", out)
	}
	if strings.Contains(out, "participant Alice") {
		t.Fatalf("Alice should not appear as a participant (no scenario triggers her):\n%s", out)
	}
}

func TestSequenceMermaid_EmptyDocStillValid(t *testing.T) {
	out, err := Sequence(&craft.CraftDoc{})
	if err != nil {
		t.Fatalf("Sequence on empty doc: %v", err)
	}
	if !strings.HasPrefix(out, "sequenceDiagram\n") {
		t.Errorf("empty doc must still emit valid header, got:\n%s", out)
	}
}
