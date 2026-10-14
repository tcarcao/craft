package analysis_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestProject_ConsecutiveScenarioLines_LastNameWins(t *testing.T) {
	src := "use_case \"U\" {\n  scenario \"first\"\n  scenario \"second\"\n  when b listens nmfbus.X\n    b records it\n}"
	g, _, _ := syntax.Parse(src)
	doc := projectFile(syntax.Root(g), "")
	scs := doc.UseCases[0].Scenarios
	if len(scs) != 1 {
		t.Fatalf("got %d scenarios, want 1: %+v", len(scs), scs)
	}
	if scs[0].Name != "second" {
		t.Errorf("Name = %q, want %q", scs[0].Name, "second")
	}
}

func TestProject_TrailingOrphanScenario_NoGhostOneDiagnostic(t *testing.T) {
	src := "use_case \"U\" {\n  when b listens nmfbus.X\n    b records it\n  scenario \"x\"\n}\n"
	g, _, _ := syntax.Parse(src)
	doc := projectFile(syntax.Root(g), "")
	if n := len(doc.UseCases[0].Scenarios); n != 1 {
		t.Fatalf("got %d scenarios, want 1", n)
	}
	diags := analysis.File("", src).Diagnostics
	if len(diags) != 1 || diags[0].Code != "craft/syntax/scenario-without-when" {
		t.Fatalf("diagnostics = %+v, want exactly scenario-without-when", diags)
	}
}
