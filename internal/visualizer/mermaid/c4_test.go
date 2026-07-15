package mermaid

import (
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func c4Fixture() *craft.CraftDoc {
	return &craft.CraftDoc{
		Services: []craft.Service{
			{Name: "Sched", Contexts: []string{"Scheduler"}, Language: "go"},
			{Name: "App", Contexts: []string{"Application"}, Language: "go"},
		},
		Actors: []craft.Actor{
			{Name: "User", Type: craft.ActorTypeUser},
			{Name: "Timer", Type: craft.ActorTypeSystem},
		},
		UseCases: []craft.UseCase{{
			Name: "Tick",
			Scenarios: []craft.Scenario{{
				ID: "s1",
				Trigger: craft.Trigger{
					Type: craft.TriggerTypeExternal, Actor: "Timer", Verb: "fires",
				},
				Actions: []craft.Action{
					{Type: craft.ActionTypeAsync, Context: "Scheduler", Event: "Tick"},
				},
			}}},
		},
	}
}

func TestC4Mermaid_Header(t *testing.T) {
	out, err := C4(c4Fixture(), false)
	if err != nil {
		t.Fatalf("C4: %v", err)
	}
	if !strings.HasPrefix(out, "C4Container\n") {
		t.Errorf("expected output to start with 'C4Container\\n', got:\n%s", out)
	}
}

func TestC4Mermaid_PersonAndSystemActors(t *testing.T) {
	out, err := C4(c4Fixture(), false)
	if err != nil {
		t.Fatalf("C4: %v", err)
	}
	if !strings.Contains(out, `Person(User, "User"`) {
		t.Errorf("expected Person(User, ...) for user actor:\n%s", out)
	}
	if !strings.Contains(out, `System_Ext(Timer, "Timer"`) {
		t.Errorf("expected System_Ext(Timer, ...) for system actor:\n%s", out)
	}
}

func TestC4Mermaid_ServiceBoundariesContainContexts(t *testing.T) {
	out, err := C4(c4Fixture(), false)
	if err != nil {
		t.Fatalf("C4: %v", err)
	}
	if !strings.Contains(out, `System_Boundary(Sched, "Sched")`) {
		t.Errorf("expected System_Boundary(Sched, ...):\n%s", out)
	}
	if !strings.Contains(out, `Container(Scheduler, "Scheduler"`) {
		t.Errorf("expected Container(Scheduler, ...):\n%s", out)
	}
}

func TestC4Mermaid_DoesNotEmitSpriteSyntax(t *testing.T) {
	out, err := C4(c4Fixture(), false)
	if err != nil {
		t.Fatalf("C4: %v", err)
	}
	// Sprites are unsupported in Mermaid C4 — must be silently dropped, not pasted through.
	if strings.Contains(out, "$sprite") {
		t.Errorf("output must not contain '$sprite=' references:\n%s", out)
	}
	if strings.Contains(out, "!include") {
		t.Errorf("output must not contain PlantUML !include directives:\n%s", out)
	}
}

func TestC4Mermaid_EventQueueWhenAsyncPresent(t *testing.T) {
	out, err := C4(c4Fixture(), false)
	if err != nil {
		t.Fatalf("C4: %v", err)
	}
	if !strings.Contains(out, "ContainerQueue(Event_Queue") {
		t.Errorf("expected ContainerQueue(Event_Queue) when async actions present:\n%s", out)
	}
}

func TestC4Mermaid_EmptyDoc(t *testing.T) {
	out, err := C4(&craft.CraftDoc{}, false)
	if err != nil {
		t.Fatalf("empty doc: %v", err)
	}
	if !strings.HasPrefix(out, "C4Container\n") {
		t.Errorf("empty doc must still emit header, got:\n%s", out)
	}
}
