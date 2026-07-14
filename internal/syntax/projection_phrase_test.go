package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/internal/syntax"
)

// TestProject_ActionPhrase_TightPunctuation_NoInsertedSpaces is the TDD RED
// test for Bug 2: projection built the phrase/description fields by
// space-joining action tokens instead of using ActionDecl.PhraseText()
// (already correct per Task 1), so flexible prose with tight punctuation
// like `asks X for (1! & 2!)` got spurious inserted spaces
// (`( 1 ! & 2 ! )`) in generated output. The fix must make projection's
// phrase/description match the exact source spacing.
func TestProject_ActionPhrase_TightPunctuation_NoInsertedSpaces(t *testing.T) {
	src := `use_case "Test" {
  when Customer initiates flow
    A asks B for (1! & 2!)
}`
	g, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	if len(doc.UseCases) != 1 || len(doc.UseCases[0].Scenarios) != 1 {
		t.Fatalf("unexpected projection shape: %+v", doc.UseCases)
	}
	actions := doc.UseCases[0].Scenarios[0].Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	got := actions[0].Phrase
	want := "(1! & 2!)"
	if got != want {
		t.Errorf("phrase spacing corrupted:\nwant: %q\ngot:  %q", want, got)
	}
	if actions[0].Description == "" {
		t.Fatalf("expected non-empty description")
	}
	if got := actions[0].Description; !strings.Contains(got, want) {
		t.Errorf("description should contain exact-spaced phrase %q, got %q", want, got)
	}
}
