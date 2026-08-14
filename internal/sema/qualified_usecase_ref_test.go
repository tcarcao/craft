package sema_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// analyzeWorkspaceForTest runs the real cross-file pipeline (syntax.Parse ->
// AnalyzeFile -> MergeWorkspaceSymbols -> AnalyzeWorkspace) over a single
// source string and returns the resolution map plus the workspace diagnostics.
// The use-case participant slot is only resolved by AnalyzeWorkspace, so a
// single-file helper that stops at AnalyzeFile cannot exercise it.
func analyzeWorkspaceForTest(t *testing.T, uri, src string) (sema.ResolutionMap, []model.Diagnostic) {
	t.Helper()
	g, li, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	syms, _ := sema.AnalyzeFile(uri, tree, li)
	perFile := map[string]sema.Symbols{uri: syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	rm, diags := sema.AnalyzeWorkspace(perFile, ws)
	return rm, diags
}

// lineOf returns the 1-based line number of the first line of src containing
// needle. Reference sites are recorded with 1-based lines, so this is what
// ResolveUseCaseRef expects.
func lineOf(t *testing.T, src, needle string) int {
	t.Helper()
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("%q not found in source", needle)
	return 0
}

func unresolvedRefs(diags []model.Diagnostic) []model.Diagnostic {
	var out []model.Diagnostic
	for _, d := range diags {
		if d.Code == "craft/sema/unresolved-reference" {
			out = append(out, d)
		}
	}
	return out
}

// The whole point of the qualified form: a bounded-context name declared in two
// domains has no unambiguous bare spelling, so `<domain>/<name>` is the only way
// to write the reference at all. Before the fix, resolveUseCaseRef looked the
// participant up by bare name only, so the qualified form fell through every
// branch and was reported as an unknown participant.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantResolves(t *testing.T) {
	src := `actor user Customer

domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks re/Billing to charge the card
}`
	uri := "file:///a.craft"
	rm, diags := analyzeWorkspaceForTest(t, uri, src)

	if got := unresolvedRefs(diags); len(got) != 0 {
		t.Fatalf("qualified participant must resolve, got %d unresolved-reference diagnostics: %+v", len(got), got)
	}

	target, ok := sema.ResolveUseCaseRef(rm, uri, "re/Billing", lineOf(t, src, "re/Billing"))
	if !ok {
		t.Fatal("expected re/Billing to be present in the resolution map")
	}
	if target.Kind != "bounded_context" {
		t.Errorf("expected kind bounded_context, got %q", target.Kind)
	}
	if target.Domain == nil {
		t.Fatal("expected the owning domain to be populated")
	}
	if target.Domain.Name != "re" {
		t.Errorf("expected the qualifier to select domain re, got %q", target.Domain.Name)
	}
}

// Both domains declare Billing. Each qualified form must select its own domain,
// which is the ambiguity the bare form cannot express.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantSelectsTheNamedDomain(t *testing.T) {
	src := `actor user Customer

domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks re/Billing to charge the card
    Subscriptions asks ops/Billing to reconcile the ledger
}`
	uri := "file:///a.craft"
	rm, diags := analyzeWorkspaceForTest(t, uri, src)

	if got := unresolvedRefs(diags); len(got) != 0 {
		t.Fatalf("both qualified participants must resolve, got: %+v", got)
	}

	for _, tc := range []struct{ ref, domain string }{
		{"re/Billing", "re"},
		{"ops/Billing", "ops"},
	} {
		target, ok := sema.ResolveUseCaseRef(rm, uri, tc.ref, lineOf(t, src, tc.ref))
		if !ok {
			t.Errorf("expected %s to resolve", tc.ref)
			continue
		}
		if target.Domain == nil || target.Domain.Name != tc.domain {
			t.Errorf("expected %s to select domain %q, got %+v", tc.ref, tc.domain, target.Domain)
		}
	}
}

// Positions stay indexed by bare BC name, matching every other bounded-context
// target, so go-to-definition on a qualified participant lands on the
// declaration inside the domain body rather than on nothing.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantCarriesBCPosition(t *testing.T) {
	src := `actor user Customer

domain re {
  Subscriptions
}

use_case "Charge card" {
  when Customer submits Payment
    Customer asks re/Subscriptions to renew
}`
	uri := "file:///a.craft"
	rm, _ := analyzeWorkspaceForTest(t, uri, src)

	target, ok := sema.ResolveUseCaseRef(rm, uri, "re/Subscriptions", lineOf(t, src, "re/Subscriptions"))
	if !ok {
		t.Fatal("expected re/Subscriptions to resolve")
	}
	if want := lineOf(t, src, "  Subscriptions"); target.BCLine != want {
		t.Errorf("expected BCLine %d (the declaration inside the domain body), got %d", want, target.BCLine)
	}
	if target.BCURI != uri {
		t.Errorf("expected BCURI %q, got %q", uri, target.BCURI)
	}
}

// A qualifier naming a domain that does not exist stays unresolved. The fix
// widens what resolves; it must not make every slash-bearing name resolve.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantUnknownDomain(t *testing.T) {
	src := `actor user Customer

domain re {
  Billing
  Subscriptions
}

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks nope/Billing to charge the card
}`
	diags := mustHaveOneUnresolved(t, "file:///a.craft", src)
	if !strings.Contains(diags.Message, "nope/Billing") {
		t.Errorf("expected the diagnostic to name nope/Billing, got %q", diags.Message)
	}
}

// The domain exists but does not declare that bounded context. Resolving the
// qualifier must check membership, not merely that the domain is known.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantDomainDoesNotDeclareBC(t *testing.T) {
	src := `actor user Customer

domain re {
  Billing
  Subscriptions
}

domain ops {
  Reconciliation
}

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks ops/Billing to charge the card
}`
	diags := mustHaveOneUnresolved(t, "file:///a.craft", src)
	if !strings.Contains(diags.Message, "ops/Billing") {
		t.Errorf("expected the diagnostic to name ops/Billing, got %q", diags.Message)
	}
}

func mustHaveOneUnresolved(t *testing.T, uri, src string) model.Diagnostic {
	t.Helper()
	_, diags := analyzeWorkspaceForTest(t, uri, src)
	got := unresolvedRefs(diags)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 unresolved-reference diagnostic, got %d: %+v", len(got), got)
	}
	return got[0]
}

// Guard against the qualified branch swallowing the bare form: bare names must
// still resolve through the actor / domain / bounded-context / service chain.
func TestAnalyzeWorkspace_BareUseCaseParticipantsStillResolve(t *testing.T) {
	src := `actor user Customer

domain re {
  Billing
  Subscriptions
}

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks Billing to charge the card
}`
	uri := "file:///a.craft"
	rm, diags := analyzeWorkspaceForTest(t, uri, src)
	if got := unresolvedRefs(diags); len(got) != 0 {
		t.Fatalf("bare participants must still resolve, got: %+v", got)
	}
	target, ok := sema.ResolveUseCaseRef(rm, uri, "Billing", lineOf(t, src, "asks Billing"))
	if !ok || target.Kind != "bounded_context" {
		t.Errorf("expected bare Billing to resolve as a bounded context, got %+v (ok=%v)", target, ok)
	}
	if target, ok := sema.ResolveUseCaseRef(rm, uri, "Customer", lineOf(t, src, "when Customer")); !ok || target.Kind != "actor" {
		t.Errorf("expected bare Customer to resolve as an actor, got %+v (ok=%v)", target, ok)
	}
}

// A qualified reference to a bounded context declared in another FILE has to
// resolve too — the participant slot is resolved against the merged workspace
// symbol table, not against the file that spells the reference.
func TestAnalyzeWorkspace_QualifiedUseCaseParticipantResolvesCrossFile(t *testing.T) {
	domains := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}`
	useCases := `actor user Customer

use_case "Charge card" {
  when Customer submits Payment
    Subscriptions asks ops/Billing to reconcile the ledger
}`
	perFile := map[string]sema.Symbols{}
	for uri, src := range map[string]string{
		"file:///domains.craft":  domains,
		"file:///usecases.craft": useCases,
	} {
		g, li, _ := syntax.Parse(src)
		syms, _ := sema.AnalyzeFile(uri, syntax.Root(g), li)
		perFile[uri] = syms
	}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	rm, diags := sema.AnalyzeWorkspace(perFile, ws)

	if got := unresolvedRefs(diags); len(got) != 0 {
		t.Fatalf("cross-file qualified participant must resolve, got: %+v", got)
	}
	target, ok := sema.ResolveUseCaseRef(rm, "file:///usecases.craft", "ops/Billing", lineOf(t, useCases, "ops/Billing"))
	if !ok {
		t.Fatal("expected ops/Billing to resolve across files")
	}
	if target.Domain == nil || target.Domain.Name != "ops" {
		t.Errorf("expected domain ops, got %+v", target.Domain)
	}
	if target.BCURI != "file:///domains.craft" {
		t.Errorf("expected the position to point at the declaring file, got %q", target.BCURI)
	}
}
