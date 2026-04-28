package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/sema"
)

// helper: build a minimal WorkspaceSymbols with actors.
func wsWithActors(names ...string) sema.WorkspaceSymbols {
	actors := make(map[string]sema.ActorSymbol, len(names))
	for _, n := range names {
		actors[n] = sema.ActorSymbol{Name: n, URI: "file:///test.craft", Line: 1}
	}
	return sema.WorkspaceSymbols{
		Actors:          actors,
		Domains:         map[string]sema.DomainSymbol{},
		BoundedContexts: map[string]sema.DomainSymbol{},
		Services:        map[string]sema.ServiceSymbol{},
		UseCases:        map[string]sema.UseCaseSymbol{},
	}
}

// ── dead-event ────────────────────────────────────────────────────────────────

func TestLint_DeadEvent_PublishedButNotConsumed(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{
						Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
						Actions: []*ast.ActionDecl{
							{ActionType: "async_action", Event: "Order Created", Line: 2},
						},
					},
				},
			},
		},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, wsWithActors())
	found := false
	for _, d := range diags {
		if d.Code == "craft/lint/dead-event" && containsStr(d.Message, `"Order Created"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/lint/dead-event for 'Order Created', got: %v", diags)
	}
}

func TestLint_DeadEvent_PublishedAndConsumed(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{
						Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
						Actions: []*ast.ActionDecl{
							{ActionType: "async_action", Event: "Order Created", Line: 2},
						},
					},
					{
						Trigger: ast.TriggerDecl{TriggerType: "event", Event: "Order Created", Line: 4},
					},
				},
			},
		},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code == "craft/lint/dead-event" && containsStr(d.Message, `"Order Created"`) {
			t.Errorf("unexpected craft/lint/dead-event for consumed event 'Order Created'")
		}
	}
}

// ── unused-actor ─────────────────────────────────────────────────────────────

func TestLint_UnusedActor_DefinedButNotUsed(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{{Name: "Admin", Type: ast.ActorTypeUser, Line: 1}},
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"}},
				},
			},
		},
	}

	ws := wsWithActors("Admin")
	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, ws)
	found := false
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" && containsStr(d.Message, `"Admin"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/lint/unused-actor for 'Admin', got: %v", diags)
	}
}

func TestLint_UnusedActor_UsedAsTrigger(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{{Name: "Admin", Type: ast.ActorTypeUser, Line: 1}},
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Admin"}},
				},
			},
		},
	}

	ws := wsWithActors("Admin")
	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, ws)
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" && containsStr(d.Message, `"Admin"`) {
			t.Errorf("unexpected craft/lint/unused-actor for used actor 'Admin'")
		}
	}
}

func TestLint_UnusedActor_NoActors_NoFire(t *testing.T) {
	f := &ast.File{}
	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" {
			t.Errorf("unexpected unused-actor diagnostic when no actors defined")
		}
	}
}

// ── event-past-tense ─────────────────────────────────────────────────────────

func TestLint_EventPastTense_NonPastTense(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{
						Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
						Actions: []*ast.ActionDecl{
							{ActionType: "async_action", Event: "Order Processing", Line: 2},
						},
					},
				},
			},
		},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, wsWithActors())
	found := false
	for _, d := range diags {
		if d.Code == "craft/lint/event-not-past-tense" && containsStr(d.Message, `"Order Processing"`) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/lint/event-not-past-tense for 'Order Processing', got: %v", diags)
	}
}

func TestLint_EventPastTense_PastTense(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{
			{
				Name: "Test",
				Scenarios: []*ast.ScenarioDecl{
					{
						Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
						Actions: []*ast.ActionDecl{
							{ActionType: "async_action", Event: "Order Created", Line: 2},
						},
					},
				},
			},
		},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///test.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code == "craft/lint/event-not-past-tense" && containsStr(d.Message, `"Order Created"`) {
			t.Errorf("unexpected craft/lint/event-not-past-tense for past-tense event 'Order Created'")
		}
	}
}

// ── range precision tests ─────────────────────────────────────────────────────

// TestLint_DeadEvent_Range_QuotedString verifies that the squiggle for a dead
// event published as a quoted string token covers the surrounding quotes.
// EventColumn points to the opening `"` and EventIsString=true, so the range
// length must be len(event)+2 not len(event).
func TestLint_DeadEvent_Range_QuotedString(t *testing.T) {
	// Source layout (1-based columns):
	//   col 1: "A" (subject)  col 3: " " col 4: "n" (notifies start)  col 12: " "  col 13: `"`
	// EventColumn=13 (opening `"`), event="Order Placed" (12 chars) → quoted token is 14 chars.
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{{
			Name: "T",
			Scenarios: []*ast.ScenarioDecl{{
				Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
				Actions: []*ast.ActionDecl{{
					ActionType:    "async_action",
					Event:         "Order Placed",
					EventColumn:   13, // 1-based column of opening `"`
					EventIsString: true,
					Line:          2,
				}},
			}},
		}},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///t.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/dead-event" {
			continue
		}
		wantStart := 12 // 0-based: col 13 → 12
		wantEnd := 12 + 14 // len("Order Placed") + 2 quotes = 14
		if d.Range.Start.Character != wantStart {
			t.Errorf("dead-event start: got %d, want %d", d.Range.Start.Character, wantStart)
		}
		if d.Range.End.Character != wantEnd {
			t.Errorf("dead-event end: got %d, want %d", d.Range.End.Character, wantEnd)
		}
		return
	}
	t.Fatal("expected craft/lint/dead-event diagnostic, got none")
}

// TestLint_DeadEvent_Range_BareIdent verifies that a bare-ident event (no quotes)
// does NOT get the +2 adjustment.
func TestLint_DeadEvent_Range_BareIdent(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{{
			Name: "T",
			Scenarios: []*ast.ScenarioDecl{{
				Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
				Actions: []*ast.ActionDecl{{
					ActionType:    "async_action",
					Event:         "OrderPlaced",
					EventColumn:   5, // 1-based
					EventIsString: false,
					Line:          2,
				}},
			}},
		}},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///t.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/dead-event" {
			continue
		}
		wantStart := 4 // 0-based: col 5 → 4
		wantEnd := 4 + 11 // len("OrderPlaced") = 11, no quotes
		if d.Range.Start.Character != wantStart {
			t.Errorf("dead-event bare start: got %d, want %d", d.Range.Start.Character, wantStart)
		}
		if d.Range.End.Character != wantEnd {
			t.Errorf("dead-event bare end: got %d, want %d", d.Range.End.Character, wantEnd)
		}
		return
	}
	t.Fatal("expected craft/lint/dead-event diagnostic, got none")
}

// TestLint_EventPastTense_Range_QuotedString verifies the squiggle for a
// non-past-tense event written as a quoted string covers the surrounding quotes.
func TestLint_EventPastTense_Range_QuotedString(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{{
			Name: "T",
			Scenarios: []*ast.ScenarioDecl{{
				Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
				Actions: []*ast.ActionDecl{{
					ActionType:    "async_action",
					Event:         "Order Processing",
					EventColumn:   9, // 1-based column of opening `"`
					EventIsString: true,
					Line:          2,
				}},
			}},
		}},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///t.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/event-not-past-tense" {
			continue
		}
		wantStart := 8  // 0-based
		wantEnd := 8 + 18 // len("Order Processing") + 2 = 18
		if d.Range.Start.Character != wantStart {
			t.Errorf("past-tense start: got %d, want %d", d.Range.Start.Character, wantStart)
		}
		if d.Range.End.Character != wantEnd {
			t.Errorf("past-tense end: got %d, want %d", d.Range.End.Character, wantEnd)
		}
		return
	}
	t.Fatal("expected craft/lint/event-not-past-tense diagnostic, got none")
}

// TestLint_EventPastTense_Range_BareIdent verifies bare-ident events are not padded.
func TestLint_EventPastTense_Range_BareIdent(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{{
			Name: "T",
			Scenarios: []*ast.ScenarioDecl{{
				Trigger: ast.TriggerDecl{TriggerType: "external", Actor: "Customer"},
				Actions: []*ast.ActionDecl{{
					ActionType:    "async_action",
					Event:         "Processing",
					EventColumn:   7,
					EventIsString: false,
					Line:          2,
				}},
			}},
		}},
	}

	diags := sema.LintWorkspace(map[string]*ast.File{"file:///t.craft": f}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/event-not-past-tense" {
			continue
		}
		wantStart := 6           // 0-based: col 7 → 6
		wantEnd := 6 + 10        // len("Processing") = 10
		if d.Range.Start.Character != wantStart {
			t.Errorf("past-tense bare start: got %d, want %d", d.Range.Start.Character, wantStart)
		}
		if d.Range.End.Character != wantEnd {
			t.Errorf("past-tense bare end: got %d, want %d", d.Range.End.Character, wantEnd)
		}
		return
	}
	t.Fatal("expected craft/lint/event-not-past-tense diagnostic, got none")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
