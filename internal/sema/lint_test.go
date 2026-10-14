package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"

	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
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

// parseTree is a test helper that parses a craft source string into a syntax
// tree. Diagnostics from parsing are ignored so tests focus on lint rules.
func parseTree(src string) syntax.SyntaxNode {
	g, _, _ := syntax.Parse(src)
	return syntax.Root(g)
}

// ── dead-event ────────────────────────────────────────────────────────────────

func TestLint_DeadEvent_PublishedButNotConsumed(t *testing.T) {
	src := `
use_case "Test" {
  when Customer creates Order
    Auth notifies "Order Created"
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
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
	src := `
use_case "Test" {
  when Customer creates Order
    Auth notifies "Order Created"
  when "Order Created"
    Auth validates receipt
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code == "craft/lint/dead-event" && containsStr(d.Message, `"Order Created"`) {
			t.Errorf("unexpected craft/lint/dead-event for consumed event 'Order Created'")
		}
	}
}

// ── unused-actor ─────────────────────────────────────────────────────────────

func TestLint_UnusedActor_DefinedButNotUsed(t *testing.T) {
	src := `
actor user Admin
use_case "Test" {
  when Customer creates Order
    Auth validates email
}
`
	ws := wsWithActors("Admin")
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, ws)
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
	src := `
actor user Admin
use_case "Test" {
  when Admin creates Order
    Auth validates email
}
`
	ws := wsWithActors("Admin")
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, ws)
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" && containsStr(d.Message, `"Admin"`) {
			t.Errorf("unexpected craft/lint/unused-actor for used actor 'Admin'")
		}
	}
}

func TestLint_UnusedActor_UsedAsListensTrigger(t *testing.T) {
	// Actors used as the subject of `when <Actor> listens "<event>"` must not
	// be reported as unused. The parser stores the subject in Context for
	// domain_listen triggers regardless of whether it is an actor or a context.
	src := `
actor system NotificationListener
use_case "Test" {
  when NotificationListener listens "Account Frozen"
    Auth validates receipt
}
`
	ws := wsWithActors("NotificationListener")
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, ws)
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" && containsStr(d.Message, `"NotificationListener"`) {
			t.Errorf("unexpected craft/lint/unused-actor for actor used in listens trigger")
		}
	}
}

func TestLint_UnusedActor_NoActors_NoFire(t *testing.T) {
	src := ``
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code == "craft/lint/unused-actor" {
			t.Errorf("unexpected unused-actor diagnostic when no actors defined")
		}
	}
}

// ── event-past-tense ─────────────────────────────────────────────────────────

func TestLint_EventPastTense_NonPastTense(t *testing.T) {
	src := `
use_case "Test" {
  when Customer creates Order
    Auth notifies "Order Processing"
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
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
	src := `
use_case "Test" {
  when Customer creates Order
    Auth notifies "Order Created"
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
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
// length must be len(name)+2 not len(name).
func TestLint_DeadEvent_Range_QuotedString(t *testing.T) {
	// Source: the notifies line is "  Auth notifies "Order Placed""
	// We parse it so positions come from the actual lexer, then just verify the
	// end character is start + len("Order Placed") + 2.
	src := `
use_case "T" {
  when Customer creates Order
    Auth notifies "Order Placed"
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///t.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/dead-event" {
			continue
		}
		// The event token is a quoted string, so the range should span len+2 chars.
		wantLen := len("Order Placed") + 2 // 14
		gotLen := d.Range.End.Character - d.Range.Start.Character
		if gotLen != wantLen {
			t.Errorf("dead-event range length: got %d, want %d (start=%d end=%d)",
				gotLen, wantLen, d.Range.Start.Character, d.Range.End.Character)
		}
		return
	}
	t.Fatal("expected craft/lint/dead-event diagnostic, got none")
}

// TestLint_DeadEvent_Range_BareIdent verifies that a bare-ident event (no quotes)
// does NOT get the +2 adjustment.
func TestLint_DeadEvent_Range_BareIdent(t *testing.T) {
	src := `
use_case "T" {
  when Customer creates Order
    Auth notifies OrderPlaced
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///t.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/dead-event" {
			continue
		}
		wantLen := len("OrderPlaced") // 11, no quotes
		gotLen := d.Range.End.Character - d.Range.Start.Character
		if gotLen != wantLen {
			t.Errorf("dead-event bare range length: got %d, want %d (start=%d end=%d)",
				gotLen, wantLen, d.Range.Start.Character, d.Range.End.Character)
		}
		return
	}
	t.Fatal("expected craft/lint/dead-event diagnostic, got none")
}

// TestLint_EventPastTense_Range_QuotedString verifies the squiggle for a
// non-past-tense event written as a quoted string covers the surrounding quotes.
func TestLint_EventPastTense_Range_QuotedString(t *testing.T) {
	src := `
use_case "T" {
  when Customer creates Order
    Auth notifies "Order Processing"
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///t.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/event-not-past-tense" {
			continue
		}
		wantLen := len("Order Processing") + 2 // 18
		gotLen := d.Range.End.Character - d.Range.Start.Character
		if gotLen != wantLen {
			t.Errorf("past-tense range length: got %d, want %d (start=%d end=%d)",
				gotLen, wantLen, d.Range.Start.Character, d.Range.End.Character)
		}
		return
	}
	t.Fatal("expected craft/lint/event-not-past-tense diagnostic, got none")
}

// TestLint_EventPastTense_Range_BareIdent verifies bare-ident events are not padded.
func TestLint_EventPastTense_Range_BareIdent(t *testing.T) {
	src := `
use_case "T" {
  when Customer creates Order
    Auth notifies Processing
}
`
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///t.craft": parseTree(src)}, wsWithActors())
	for _, d := range diags {
		if d.Code != "craft/lint/event-not-past-tense" {
			continue
		}
		wantLen := len("Processing") // 10
		gotLen := d.Range.End.Character - d.Range.Start.Character
		if gotLen != wantLen {
			t.Errorf("past-tense bare range length: got %d, want %d (start=%d end=%d)",
				gotLen, wantLen, d.Range.Start.Character, d.Range.End.Character)
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

// ── indistinguishable-scenarios ──────────────────────────────────────────────

func indistinguishable(t *testing.T, src string) []string {
	t.Helper()
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{"file:///test.craft": parseTree(src)}, wsWithActors())
	var out []string
	for _, d := range diags {
		if d.Code == "craft/lint/indistinguishable-scenarios" {
			out = append(out, d.Message)
		}
	}
	return out
}

func TestLint_IndistinguishableScenarios(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"unnamed twins warn", "use_case \"U\" {\n  when A does x\n    S1 acts\n  when A does x\n    S2 acts\n}\n", 1},
		{"both named", "use_case \"U\" {\n  scenario \"one\"\n  when A does x\n    S1 acts\n  scenario \"two\"\n  when A does x\n    S2 acts\n}\n", 0},
		{"differing triggers", "use_case \"U\" {\n  when A does x\n    S1 acts\n  when A does y\n    S2 acts\n}\n", 0},
		{"whitespace only", "use_case \"U\" {\n  when A  does   x\n    S1 acts\n  when A does x\n    S2 acts\n}\n", 1},
		{"separate use cases", "use_case \"U\" {\n  when A does x\n    S1 acts\n}\nuse_case \"V\" {\n  when A does x\n    S2 acts\n}\n", 0},
		{"listens repeated", "use_case \"U\" {\n  when x listens e.E\n    S1 acts\n  when x listens e.E\n    S2 acts\n}\n", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := indistinguishable(t, c.src); len(got) != c.want {
				t.Errorf("want %d warnings, got %v", c.want, got)
			}
		})
	}
}

func TestLint_IndistinguishableScenarios_Ranges(t *testing.T) {
	src := "use_case \"U\" {\n    when CRON fires the schedule\n        sp a\n    when CRON fires the schedule\n        sp b\n    scenario \"named twin\"\n    when CRON fires the schedule\n        sp c\n}\n"
	g, li, _ := syntax.Parse(src)
	uri := "file:///test.craft"
	diags := sema.LintWorkspace(map[string]syntax.SyntaxNode{uri: syntax.Root(g)}, wsWithActors(), map[string]green.LineIndex{uri: li})
	var lines []int
	for _, d := range diags {
		if d.Code != "craft/lint/indistinguishable-scenarios" {
			continue
		}
		lines = append(lines, d.Range.Start.Line)
		if d.Range.Start.Character != 4 || d.Range.End.Character != 8 {
			t.Errorf("range = %+v, want the 4-char `when` at column 4", d.Range)
		}
		if want := `scenario repeats the trigger "when CRON fires the schedule"; use alt "…" for different outcomes, or scenario "…" names for different ways in`; d.Message != want {
			t.Errorf("message = %q", d.Message)
		}
	}
	if len(lines) != 2 || lines[0] != 3 || lines[1] != 6 {
		t.Errorf("warning lines = %v, want [3 6]", lines)
	}
}
