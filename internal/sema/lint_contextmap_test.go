package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// buildLintWorkspace runs the real single-file pipeline (AnalyzeFile ->
// MergeWorkspaceSymbols) over src and returns the pieces LintWorkspace needs:
// the per-file syntax tree map, the merged workspace symbol table, and the
// per-file line-index map. Mirrors analyzeRelationshipSrc in
// validate_relationship_test.go, but stops short of AnalyzeWorkspace since
// these tests exercise the lint pass directly.
func buildLintWorkspace(t *testing.T, src string) (map[string]syntax.SyntaxNode, WorkspaceSymbols, map[string]green.LineIndex) {
	t.Helper()
	g, li, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	uri := "file:///lint.craft"
	syms, _ := AnalyzeFile(uri, tree, li)
	perFile := map[string]Symbols{uri: syms}
	ws, _ := MergeWorkspaceSymbols(perFile)
	return map[string]syntax.SyntaxNode{uri: tree}, ws, map[string]green.LineIndex{uri: li}
}

func countCode(diags []model.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

// TestContextMapConsistency_SeparateWaysViolation: a separate_ways edge whose
// endpoints DO communicate (billing asks vas) must fire exactly one
// separate-ways-violation warning.
func TestContextMapConsistency_SeparateWaysViolation(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing separate_ways vas }
use_case "Test" {
  when Customer creates Order
    billing asks vas to check eligibility
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/separate-ways-violation"); n != 1 {
		t.Fatalf("expected 1 craft/lint/separate-ways-violation, got %d: %+v", n, diags)
	}
	for _, d := range diags {
		if d.Code == "craft/lint/separate-ways-violation" && d.Severity != model.SeverityWarning {
			t.Errorf("expected warning severity, got %q", d.Severity)
		}
	}
}

// TestContextMapConsistency_SeparateWaysNoCommunication: a separate_ways edge
// whose endpoints have NO communication between them must fire zero
// diagnostics — absence of communication never warns (the communication view
// is partial).
func TestContextMapConsistency_SeparateWaysNoCommunication(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing separate_ways vas }
use_case "Test" {
  when Customer creates Order
    billing validates something
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/separate-ways-violation"); n != 0 {
		t.Fatalf("expected 0 craft/lint/separate-ways-violation, got %d: %+v", n, diags)
	}
}

// TestContextMapConsistency_SeparateWaysViolation_Async: a separate_ways edge
// whose endpoints communicate via the ASYNC notifies/listens event-pairing
// path (not `asks`) must also fire exactly one separate-ways-violation
// warning. This is the crux of buildDependencyEdges' async publisher/listener
// pairing (lint.go's publishersByEvent/listenersByEvent matching), which had
// no direct test coverage prior to this one.
func TestContextMapConsistency_SeparateWaysViolation_Async(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing separate_ways vas }
use_case "Publish" {
  when Customer creates Order
    billing notifies "OrderBilled"
}
use_case "Consume" {
  when vas listens "OrderBilled"
    vas validates payload
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/separate-ways-violation"); n != 1 {
		t.Fatalf("expected 1 craft/lint/separate-ways-violation, got %d: %+v", n, diags)
	}
	for _, d := range diags {
		if d.Code == "craft/lint/separate-ways-violation" && d.Severity != model.SeverityWarning {
			t.Errorf("expected warning severity, got %q", d.Severity)
		}
	}
}

// TestContextMapConsistency_SeparateWaysNoCommunication_Async: an async
// notifies/listens pair on an event NOT shared between the separate_ways
// endpoints must fire zero diagnostics — the two BCs never actually pair up,
// so no dependency edge is recorded between them.
func TestContextMapConsistency_SeparateWaysNoCommunication_Async(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing separate_ways vas }
use_case "Publish" {
  when Customer creates Order
    billing notifies "OrderBilled"
}
use_case "Consume" {
  when vas listens "SomeUnrelatedEvent"
    vas validates payload
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/separate-ways-violation"); n != 0 {
		t.Fatalf("expected 0 craft/lint/separate-ways-violation, got %d: %+v", n, diags)
	}
}

// ── R2a/R2b: relationship-direction-inverted / relationship-bidirectional ──
//
// billing customer_supplier vas: billing=LEFT=upstream=supplier,
// vas=RIGHT=downstream=customer. Expected dependency direction is
// vas -> billing (downstream depends on upstream, i.e. RIGHT -> LEFT).

// TestContextMapConsistency_DirectionInverted: only `billing asks vas`
// (upstream depends on downstream = billing -> vas = inverted) with no
// correct-direction traffic must fire exactly one relationship-direction-inverted
// warning, and zero relationship-bidirectional.
func TestContextMapConsistency_DirectionInverted(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing customer_supplier vas }
use_case "Test" {
  when Customer creates Order
    billing asks vas to check eligibility
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/relationship-direction-inverted"); n != 1 {
		t.Fatalf("expected 1 craft/lint/relationship-direction-inverted, got %d: %+v", n, diags)
	}
	if n := countCode(diags, "craft/lint/relationship-bidirectional"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-bidirectional, got %d: %+v", n, diags)
	}
	for _, d := range diags {
		if d.Code == "craft/lint/relationship-direction-inverted" && d.Severity != model.SeverityWarning {
			t.Errorf("expected warning severity, got %q", d.Severity)
		}
	}
}

// TestContextMapConsistency_DirectionCorrect: replacing the inverted use case
// with `vas asks billing` (correct direction: downstream depends on
// upstream) must fire zero direction-inverted and zero bidirectional
// diagnostics — a correct-only relationship is silent.
func TestContextMapConsistency_DirectionCorrect(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing customer_supplier vas }
use_case "Test" {
  when Customer creates Order
    vas asks billing to check eligibility
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/relationship-direction-inverted"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-direction-inverted, got %d: %+v", n, diags)
	}
	if n := countCode(diags, "craft/lint/relationship-bidirectional"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-bidirectional, got %d: %+v", n, diags)
	}
}

// TestContextMapConsistency_Bidirectional: both `billing asks vas` AND
// `vas asks billing` present must fire exactly one relationship-bidirectional
// hint, and zero relationship-direction-inverted.
func TestContextMapConsistency_Bidirectional(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing customer_supplier vas }
use_case "Forward" {
  when Customer creates Order
    vas asks billing to check eligibility
}
use_case "Backward" {
  when Customer cancels Order
    billing asks vas to refund
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/relationship-bidirectional"); n != 1 {
		t.Fatalf("expected 1 craft/lint/relationship-bidirectional, got %d: %+v", n, diags)
	}
	if n := countCode(diags, "craft/lint/relationship-direction-inverted"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-direction-inverted, got %d: %+v", n, diags)
	}
	for _, d := range diags {
		if d.Code == "craft/lint/relationship-bidirectional" && d.Severity != model.SeverityHint {
			t.Errorf("expected hint severity, got %q", d.Severity)
		}
	}
}

// TestContextMapConsistency_PublishedLanguageAsync_Correct: a
// published_language edge (LEFT=publisher/upstream, RIGHT=consumer/
// downstream) exercised via notifies/listens with RIGHT listening to LEFT's
// event (downstream depends on upstream = correct direction) must fire zero
// diagnostics. Proves async dependency direction composes with the rule.
func TestContextMapConsistency_PublishedLanguageAsync_Correct(t *testing.T) {
	src := `domain re { billing vas }
context_map re { billing published_language vas }
use_case "Publish" {
  when Customer creates Order
    billing notifies "OrderBilled"
}
use_case "Consume" {
  when vas listens "OrderBilled"
    vas validates payload
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/relationship-direction-inverted"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-direction-inverted, got %d: %+v", n, diags)
	}
	if n := countCode(diags, "craft/lint/relationship-bidirectional"); n != 0 {
		t.Fatalf("expected 0 craft/lint/relationship-bidirectional, got %d: %+v", n, diags)
	}
}

// ── R3: unclassified-communication ──

// TestContextMapConsistency_UnclassifiedCommunication: billing and ledger
// communicate (billing asks ledger) but have NO context_map edge between them
// at all (no context_map block references this pair) -> exactly one
// unclassified-communication hint, anchored on the communicating action.
func TestContextMapConsistency_UnclassifiedCommunication(t *testing.T) {
	src := `domain re { billing ledger }
use_case "Test" {
  when Customer creates Order
    billing asks ledger to record entry
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/unclassified-communication"); n != 1 {
		t.Fatalf("expected 1 craft/lint/unclassified-communication, got %d: %+v", n, diags)
	}
	for _, d := range diags {
		if d.Code == "craft/lint/unclassified-communication" && d.Severity != model.SeverityHint {
			t.Errorf("expected hint severity, got %q", d.Severity)
		}
	}
}

// TestContextMapConsistency_ClassifiedCommunication_NoUnclassifiedHint: the
// same communicating pair, but now classified by a context_map edge (any
// verb, e.g. customer_supplier) -> zero unclassified-communication hints.
func TestContextMapConsistency_ClassifiedCommunication_NoUnclassifiedHint(t *testing.T) {
	src := `domain re { billing ledger }
context_map re { billing customer_supplier ledger }
use_case "Test" {
  when Customer creates Order
    billing asks ledger to record entry
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/unclassified-communication"); n != 0 {
		t.Fatalf("expected 0 craft/lint/unclassified-communication, got %d: %+v", n, diags)
	}
}

// TestContextMapConsistency_NonCommunicatingClassifiedPair_NoUnclassifiedHint:
// a classified pair (context_map edge present) that does NOT communicate in
// any use case must not produce an unclassified-communication hint (nor any
// other diagnostic) — absence of communication never warns.
func TestContextMapConsistency_NonCommunicatingClassifiedPair_NoUnclassifiedHint(t *testing.T) {
	src := `domain re { billing ledger }
context_map re { billing customer_supplier ledger }
use_case "Test" {
  when Customer creates Order
    billing validates something
}
`
	perFileTrees, ws, lis := buildLintWorkspace(t, src)
	diags := LintWorkspace(perFileTrees, ws, lis)
	if n := countCode(diags, "craft/lint/unclassified-communication"); n != 0 {
		t.Fatalf("expected 0 craft/lint/unclassified-communication, got %d: %+v", n, diags)
	}
}
