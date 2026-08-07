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

// The reviewer flagged that the subject-side branch of the sync_action check
// had no dedicated test: both tests above only exercise the target position.
// Here the target (Subscriptions) is unambiguous and the subject (Billing)
// is the one declared in two domains, isolating the subject branch.
func TestAmbiguousBC_SyncActionSubjectAmbiguous(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Billing asks Subscriptions for a charge
}`
	diags := analyzeSingleFileForTest(t, src)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/sema/ambiguous-bc for ambiguous subject, got %+v", diags)
	}
}

// buildDependencyEdges resolves an async_action subject (the notifies party)
// the same way it resolves a sync_action subject or target, and silently
// drops the ambiguous case in the same way. A bare Billing here must be
// reported for the same reason it is reported as an asks target or subject.
func TestAmbiguousBC_InAsyncNotifiesSubject(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Billing notifies "Charged"
}`
	diags := analyzeSingleFileForTest(t, src)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/sema/ambiguous-bc for ambiguous notifies subject, got %+v", diags)
	}
}

// The notifies subject is unambiguous here (Subscriptions is declared in
// only one domain), so no diagnostic should fire. This is the async-site
// counterpart to TestAmbiguousBC_QualifiedTargetIsClean: it proves the
// predicate does not over-fire on a clean bare name. The qualified form of
// the same clean case is covered by TestAmbiguousBC_QualifiedSubjectsAreClean.
func TestAmbiguousBC_UnambiguousNotifiesSubjectIsClean(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Subscriptions notifies "Charged"
}`
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/sema/ambiguous-bc" {
			t.Errorf("unambiguous notifies subject must not be ambiguous, got %+v", d)
		}
	}
}

// buildDependencyEdges resolves a domain_listen trigger's context the same
// way it resolves a sync_action subject or target, and silently drops the
// ambiguous case in the same way. A bare Billing here must be reported.
func TestAmbiguousBC_InListensTrigger(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when Billing listens "Charged"
    Subscriptions validates charge
}`
	diags := analyzeSingleFileForTest(t, src)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/sema/ambiguous-bc for ambiguous listens context, got %+v", diags)
	}
}

// Clean-case counterpart to TestAmbiguousBC_InListensTrigger: the trigger
// context (Subscriptions) is unambiguous, so no diagnostic should fire. The
// qualified form of the same clean case is covered by
// TestAmbiguousBC_QualifiedSubjectsAreClean.
func TestAmbiguousBC_UnambiguousListensContextIsClean(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when Subscriptions listens "Charged"
    Billing validates charge
}`
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/sema/ambiguous-bc" {
			t.Errorf("unambiguous listens context must not be ambiguous, got %+v", d)
		}
	}
}

// The reviewer raised the risk that walking both actions and triggers could
// produce two diagnostics at the same position for one ambiguous name. Here
// the same name (Billing) is both the subject and the target of one
// sync_action, so it is checked twice in the same scenario. The two hits are
// at different source columns (the subject and the target are different
// tokens), so this must produce exactly two diagnostics, not one collapsed
// report and not more than two.
func TestAmbiguousBC_SubjectAndTargetSameName_NoDuplicateAtSamePosition(t *testing.T) {
	src := `domain re {
  Billing
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Billing asks Billing for a charge
}`
	diags := analyzeSingleFileForTest(t, src)
	var ambiguous []model.Diagnostic
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			ambiguous = append(ambiguous, d)
		}
	}
	if len(ambiguous) != 2 {
		t.Fatalf("expected exactly 2 craft/sema/ambiguous-bc (subject and target), got %d: %+v", len(ambiguous), ambiguous)
	}
	if ambiguous[0].Range.Start.Character == ambiguous[1].Range.Start.Character {
		t.Errorf("subject and target diagnostics must not share a position, got both at character %d", ambiguous[0].Range.Start.Character)
	}
}

// Task 6b: the action subject and the domain_listen trigger context accept
// the qualified <domain>/<name> form, so the fix craft/sema/ambiguous-bc
// recommends ("qualify it as <domain>/<name>") is expressible at every site
// the diagnostic fires from. Each case below is the ambiguous fixture from
// the tests above with the ambiguous bare Billing qualified; the diagnostic
// must fall silent. This is the test that proves the point of the task: an
// ambiguous name the user cannot disambiguate is a broken message.
func TestAmbiguousBC_QualifiedSubjectsAreClean(t *testing.T) {
	const domains = `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

`
	cases := []struct {
		name string
		body string
	}{
		{
			name: "asks subject",
			body: "use_case \"X\" {\n  when U does x\n    re/Billing asks Subscriptions to renew\n}",
		},
		{
			name: "notifies subject",
			body: "use_case \"X\" {\n  when U does x\n    re/Billing notifies \"Charged\"\n}",
		},
		{
			name: "listens trigger context",
			body: "use_case \"X\" {\n  when re/Billing listens \"Charged\"\n    Subscriptions validates charge\n}",
		},
		{
			name: "returns target",
			body: "use_case \"X\" {\n  when U does x\n    Subscriptions returns to re/Billing charge result\n}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, d := range analyzeSingleFileForTest(t, domains+tc.body) {
				if d.Code == "craft/sema/ambiguous-bc" {
					t.Errorf("qualified ref must not be ambiguous, got %+v", d)
				}
				// Guards against passing vacuously: if the qualified form
				// stopped parsing, the absence of the ambiguity diagnostic
				// above would prove nothing.
				if d.Code == "craft/syntax/unexpected-token" {
					t.Errorf("qualified ref must parse cleanly, got %+v", d)
				}
			}
		})
	}
}
