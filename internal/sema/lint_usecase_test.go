package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

// analyzeSingleFileForTest runs the real single-file pipeline (AnalyzeFile ->
// MergeWorkspaceSymbols -> LintWorkspace) over src and returns the lint-pass
// diagnostics. Mirrors buildLintWorkspace in lint_contextmap_test.go, folding
// the LintWorkspace call in since these tests only care about the resulting
// diagnostics.
func analyzeSingleFileForTest(t *testing.T, src string) []model.Diagnostic {
	t.Helper()
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	return LintWorkspace(perFileTrees, ws, lis)
}

// Two domains both own a BC named Billing, so a bare `Billing` in a use_case
// action is genuinely ambiguous and must be reported rather than silently
// dropped from the dependency graph.
func TestAmbiguousBC_InUseCaseAction(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Subscriptions asks Billing for a charge
}`
	diags := analyzeSingleFileForTest(t, src)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/sema/ambiguous-bc, got %+v", diags)
	}
}

func TestAmbiguousBC_QualifiedTargetIsClean(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Subscriptions asks re/billing for a charge
}`
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/sema/ambiguous-bc" {
			t.Errorf("qualified target must not be ambiguous, got %+v", d)
		}
	}
}
