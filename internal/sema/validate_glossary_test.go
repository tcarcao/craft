package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

const (
	codeGlossaryEndpointNotBC = "craft/sema/glossary-endpoint-not-bc"
	codeGlossaryUnresolvedBC  = "craft/sema/glossary-unresolved-bc"
	codeGlossaryAmbiguousBC   = "craft/sema/glossary-ambiguous-bc"
	codeGlossarySelfRelation  = "craft/sema/glossary-self-relation"
)

// TestGlossary_ValidEndpoints: both term-node endpoints resolve to distinct
// bounded contexts of the scoping domain — no glossary diagnostics at all.
func TestGlossary_ValidEndpoints(t *testing.T) {
	src := "domain re { billing subscriptions }\nglossary re { billing/Invoice same_as subscriptions/Invoice }"
	diags := analyzeRelationshipSrc(t, src)
	for _, code := range []string{codeGlossaryEndpointNotBC, codeGlossaryUnresolvedBC, codeGlossaryAmbiguousBC, codeGlossarySelfRelation} {
		if n := relDiagCount(diags, code); n != 0 {
			t.Errorf("expected 0 %s, got %d: %+v", code, n, diags)
		}
	}
}

// TestGlossary_EndpointNotBC: a term node whose BC part names a declared
// domain (not a bounded context) => exactly one glossary-endpoint-not-bc.
func TestGlossary_EndpointNotBC(t *testing.T) {
	src := "domain re { billing }\ndomain payments { charging }\nglossary re { billing/Invoice same_as payments/Invoice }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeGlossaryEndpointNotBC); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeGlossaryEndpointNotBC, n, diags)
	}
	for _, d := range diags {
		if d.Code == codeGlossaryEndpointNotBC && d.Severity != model.SeverityError {
			t.Errorf("expected error severity, got %q", d.Severity)
		}
	}
}

// TestGlossary_SelfRelation: both term nodes resolve to the same canonical
// bc/term identity => exactly one glossary-self-relation error.
func TestGlossary_SelfRelation(t *testing.T) {
	src := "domain re { billing }\nglossary re { billing/Invoice same_as billing/Invoice }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeGlossarySelfRelation); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeGlossarySelfRelation, n, diags)
	}
	if n := relDiagCount(diags, codeGlossaryEndpointNotBC); n != 0 {
		t.Errorf("expected 0 %s, got %d", codeGlossaryEndpointNotBC, n)
	}
	for _, d := range diags {
		if d.Code == codeGlossarySelfRelation && d.Severity != model.SeverityError {
			t.Errorf("expected error severity, got %q", d.Severity)
		}
	}
}

// TestGlossary_AmbiguousBC: a bare BC name declared in two domains, used
// unscoped in a term node => one glossary-ambiguous-bc error on that
// endpoint, and no self-relation false-positive.
func TestGlossary_AmbiguousBC(t *testing.T) {
	src := "domain a { billing }\ndomain b { billing }\ndomain c { vas }\nglossary { billing/Invoice same_as vas/Invoice }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeGlossaryAmbiguousBC); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeGlossaryAmbiguousBC, n, diags)
	}
	if n := relDiagCount(diags, codeGlossarySelfRelation); n != 0 {
		t.Errorf("ambiguous endpoint must not trigger self-relation, got %d", n)
	}
}

// TestGlossary_OneSegmentNodeUnresolved: a bare one-segment node (no BC
// part at all) resolves its (empty) BC ref to nothing, surfacing as
// glossary-unresolved-bc rather than any shape diagnostic.
func TestGlossary_OneSegmentNodeUnresolved(t *testing.T) {
	src := "domain re { billing }\nglossary re { Invoice same_as billing/Invoice }"
	diags := analyzeRelationshipSrc(t, src)
	if n := relDiagCount(diags, codeGlossaryUnresolvedBC); n != 1 {
		t.Fatalf("expected 1 %s, got %d: %+v", codeGlossaryUnresolvedBC, n, diags)
	}
	for _, code := range []string{codeGlossaryEndpointNotBC, codeGlossaryAmbiguousBC, codeGlossarySelfRelation} {
		if n := relDiagCount(diags, code); n != 0 {
			t.Errorf("expected 0 %s, got %d: %+v", code, n, diags)
		}
	}
}

// TestResolveTermNode exercises the resolution helper directly (whitebox).
func TestResolveTermNode(t *testing.T) {
	ws := WorkspaceSymbols{
		Domains: map[string]DomainSymbol{
			"re":       {Name: "re", BoundedContexts: []string{"billing", "subscriptions"}},
			"payments": {Name: "payments", BoundedContexts: []string{"charging"}},
		},
	}

	tests := []struct {
		name        string
		scope, node string
		wantCanon   string
		wantKind    string
		wantAmbig   bool
		wantTerm    string
	}{
		{"qualified bc/term", "", "re/billing/Invoice", "re/billing/Invoice", "bc", false, "Invoice"},
		{"scope-first bare bc/term", "re", "billing/Invoice", "re/billing/Invoice", "bc", false, "Invoice"},
		{"bc part is a domain", "", "payments/Invoice", "payments/Invoice", "domain", false, "Invoice"},
		{"one-segment node", "", "Invoice", "/Invoice", "", false, "Invoice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCanon, gotKind, gotAmbig, gotTerm := resolveTermNode(ws, tt.scope, tt.node)
			if gotCanon != tt.wantCanon || gotKind != tt.wantKind || gotAmbig != tt.wantAmbig || gotTerm != tt.wantTerm {
				t.Errorf("resolveTermNode(%q,%q) = (%q,%q,%v,%q), want (%q,%q,%v,%q)",
					tt.scope, tt.node, gotCanon, gotKind, gotAmbig, gotTerm,
					tt.wantCanon, tt.wantKind, tt.wantAmbig, tt.wantTerm)
			}
		})
	}
}
