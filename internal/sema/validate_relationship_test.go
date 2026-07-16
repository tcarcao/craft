package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// analyzeRelationshipSrc runs the real cross-file pipeline
// (AnalyzeFile -> MergeWorkspaceSymbols -> AnalyzeWorkspace) over a single
// source string and returns the workspace-pass diagnostics — mirroring the
// broken-fixture harness in internal/parser_diff/diagnostics_test.go. Whitebox
// so it can also exercise resolveBCRef directly.
func analyzeRelationshipSrc(t *testing.T, src string) []model.Diagnostic {
	t.Helper()
	g, li, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	uri := "file:///rel.craft"
	syms, _ := AnalyzeFile(uri, tree, li)
	perFile := map[string]Symbols{uri: syms}
	ws, _ := MergeWorkspaceSymbols(perFile)
	_, diags := AnalyzeWorkspace(perFile, ws)
	return diags
}

func relDiagCount(diags []model.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

const (
	codeEndpointNotBC         = "craft/sema/edge-endpoint-not-bc"
	codeSelfRelationship      = "craft/sema/self-relationship"
	codeUnresolvedBC          = "craft/sema/unresolved-bc"
	codeAmbiguousBC           = "craft/sema/ambiguous-bc"
	codeRedundantRelationship = "craft/lint/redundant-relationship"
)

// TestRelationship_ValidEndpoints: both endpoints are BCs of the scoping
// domain — no relationship diagnostics at all.
func TestRelationship_ValidEndpoints(t *testing.T) {
	src := "domain re { billing vas }\ncontext_map re { billing customer_supplier vas }"
	diags := analyzeRelationshipSrc(t, src)
	for _, code := range []string{codeEndpointNotBC, codeSelfRelationship, codeUnresolvedBC, codeAmbiguousBC} {
		if n := relDiagCount(diags, code); n != 0 {
			t.Errorf("expected 0 %s, got %d: %+v", code, n, diags)
		}
	}
}

// TestRelationship_EndpointNotBC: an endpoint that names a declared domain
// (not a bounded context) => exactly one edge-endpoint-not-bc error.
func TestRelationship_EndpointNotBC(t *testing.T) {
	src := "domain re { billing vas }\ndomain payments { charging }\ncontext_map re { billing customer_supplier payments }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeEndpointNotBC); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeEndpointNotBC, n, diags)
	}
	for _, d := range diags {
		if d.Code == codeEndpointNotBC && d.Severity != model.SeverityError {
			t.Errorf("expected error severity, got %q", d.Severity)
		}
	}
}

// TestRelationship_Self: both endpoints resolve to the same BC => exactly one
// self-relationship error and no endpoint diagnostics.
func TestRelationship_Self(t *testing.T) {
	src := "domain re { billing vas }\ncontext_map re { billing partnership billing }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeSelfRelationship); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeSelfRelationship, n, diags)
	}
	if n := relDiagCount(diags, codeEndpointNotBC); n != 0 {
		t.Errorf("expected 0 %s, got %d", codeEndpointNotBC, n)
	}
	for _, d := range diags {
		if d.Code == codeSelfRelationship && d.Severity != model.SeverityError {
			t.Errorf("expected error severity, got %q", d.Severity)
		}
	}
}

// TestRelationship_AmbiguousBC: a bare BC name declared in two domains,
// referenced unscoped => one ambiguous-bc error on that endpoint.
func TestRelationship_AmbiguousBC(t *testing.T) {
	src := "domain a { billing }\ndomain b { billing }\ndomain c { vas }\ncontext_map { billing partnership vas }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeAmbiguousBC); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeAmbiguousBC, n, diags)
	}
	if n := relDiagCount(diags, codeSelfRelationship); n != 0 {
		t.Errorf("ambiguous endpoint must not trigger self-relationship, got %d", n)
	}
}

// TestResolveBCRef exercises the resolution helper directly (whitebox).
func TestResolveBCRef(t *testing.T) {
	ws := WorkspaceSymbols{
		Domains: map[string]DomainSymbol{
			"re":       {Name: "re", BoundedContexts: []string{"billing", "vas"}},
			"payments": {Name: "payments", BoundedContexts: []string{"charging"}},
		},
		Services: map[string]ServiceSymbol{"gateway": {Name: "gateway"}},
		Actors:   map[string]ActorSymbol{"customer": {Name: "customer"}},
	}

	tests := []struct {
		name        string
		scope, ref  string
		wantResolve string
		wantKind    string
		wantAmbig   bool
	}{
		{"qualified bc", "", "re/billing", "re/billing", "bc", false},
		{"qualified missing bc", "", "re/nope", "", "", false},
		{"qualified missing domain", "", "nope/billing", "", "", false},
		{"bare scope-first", "re", "billing", "re/billing", "bc", false},
		{"bare single owner", "", "charging", "payments/charging", "bc", false},
		{"bare is domain", "", "payments", "payments", "domain", false},
		{"bare is service", "", "gateway", "gateway", "service", false},
		{"bare is actor", "", "customer", "customer", "actor", false},
		{"bare unresolved", "", "ghost", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotR, gotK, gotA := resolveBCRef(ws, tt.scope, tt.ref)
			if gotR != tt.wantResolve || gotK != tt.wantKind || gotA != tt.wantAmbig {
				t.Errorf("resolveBCRef(%q,%q) = (%q,%q,%v), want (%q,%q,%v)",
					tt.scope, tt.ref, gotR, gotK, gotA, tt.wantResolve, tt.wantKind, tt.wantAmbig)
			}
		})
	}

	// Ambiguity requires two owning domains.
	ambWS := WorkspaceSymbols{Domains: map[string]DomainSymbol{
		"a": {Name: "a", BoundedContexts: []string{"billing"}},
		"b": {Name: "b", BoundedContexts: []string{"billing"}},
	}}
	if r, k, amb := resolveBCRef(ambWS, "", "billing"); !amb || k != "bc" || r != "" {
		t.Errorf("ambiguous case = (%q,%q,%v), want (\"\",\"bc\",true)", r, k, amb)
	}
}

// TestRedundantRelationship_SymmetricDuplicate: the same symmetric pair
// declared twice in reverse order collapses to exactly one
// redundant-relationship warning, and no other relationship diagnostics fire.
func TestRedundantRelationship_SymmetricDuplicate(t *testing.T) {
	src := "domain re { a b }\ncontext_map re {\n  a partnership b\n  b partnership a\n}\n"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeRedundantRelationship); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeRedundantRelationship, n, diags)
	}
	for _, code := range []string{codeEndpointNotBC, codeSelfRelationship, codeUnresolvedBC, codeAmbiguousBC} {
		if n := relDiagCount(diags, code); n != 0 {
			t.Errorf("expected 0 %s, got %d: %+v", code, n, diags)
		}
	}
	for _, d := range diags {
		if d.Code == codeRedundantRelationship && d.Severity != model.SeverityWarning {
			t.Errorf("expected warning severity, got %q", d.Severity)
		}
	}
}

// TestRedundantRelationship_DirectionalNeverFires: a directional verb
// declared in both orders is not the same undirected fact, so it must never
// trigger redundant-relationship.
func TestRedundantRelationship_DirectionalNeverFires(t *testing.T) {
	src := "domain re { a b }\ncontext_map re {\n  a customer_supplier b\n  b customer_supplier a\n}\n"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeRedundantRelationship); n != 0 {
		t.Fatalf("expected 0 %s, got %d: %+v", codeRedundantRelationship, n, diags)
	}
}

// TestRedundantRelationship_FirstOccurrenceNeverWarns: a single symmetric
// edge (no duplicate) never fires the redundancy warning.
func TestRedundantRelationship_FirstOccurrenceNeverWarns(t *testing.T) {
	src := "domain re { a b }\ncontext_map re { a partnership b }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeRedundantRelationship); n != 0 {
		t.Fatalf("expected 0 %s, got %d: %+v", codeRedundantRelationship, n, diags)
	}
}
