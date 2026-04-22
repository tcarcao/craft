package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/sema"
)

func TestAnalyzeFile_NoDuplicates(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{
			{Name: "Alice", Type: ast.ActorTypeUser, Line: 1},
			{Name: "Bob", Type: ast.ActorTypeSystem, Line: 2},
		},
	}
	syms, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(syms.Actors) != 2 {
		t.Fatalf("expected 2 actor symbols, got %d", len(syms.Actors))
	}
}

func TestAnalyzeFile_DuplicateActorName(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{
			{Name: "Alice", Type: ast.ActorTypeUser, Line: 1},
			{Name: "Alice", Type: ast.ActorTypeSystem, Line: 3},
		},
	}
	syms, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/duplicate-name" {
		t.Errorf("expected code craft/sema/duplicate-name, got %q", diags[0].Code)
	}
	if diags[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", diags[0].Severity)
	}
	if len(syms.Actors) != 1 {
		t.Errorf("expected 1 symbol (first declaration only), got %d", len(syms.Actors))
	}
}

func TestAnalyzeFile_DuplicateDomainName(t *testing.T) {
	f := &ast.File{
		Domains: []*ast.DomainDecl{
			{Name: "Payments", Line: 1, BoundedContexts: []string{"ProcessPayment"}},
			{Name: "Payments", Line: 5, BoundedContexts: []string{"RefundPayment"}},
		},
	}
	syms, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/duplicate-name" {
		t.Errorf("expected code craft/sema/duplicate-name, got %q", diags[0].Code)
	}
	if diags[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", diags[0].Severity)
	}
	// Duplicate is not added to symbols.
	if len(syms.Domains) != 1 {
		t.Errorf("expected 1 domain symbol (first declaration only), got %d", len(syms.Domains))
	}
	if syms.Domains[0].Name != "Payments" {
		t.Errorf("expected symbol name Payments, got %q", syms.Domains[0].Name)
	}
}

func TestAnalyzeFile_CrossKindNameReuse(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{
			{Name: "Customer", Type: ast.ActorTypeUser, Line: 1},
		},
		Domains: []*ast.DomainDecl{
			{Name: "Customer", Line: 3, BoundedContexts: []string{"Authentication"}},
		},
	}
	_, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/cross-kind-name-reuse" {
		t.Errorf("expected code craft/sema/cross-kind-name-reuse, got %q", diags[0].Code)
	}
	if diags[0].Severity != "warning" {
		t.Errorf("expected warning severity, got %q", diags[0].Severity)
	}
}

func TestAnalyzeFile_NoCrossKindWarningWhenNamesDiffer(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{
			{Name: "Alice", Type: ast.ActorTypeUser, Line: 1},
		},
		Domains: []*ast.DomainDecl{
			{Name: "Payments", Line: 3, BoundedContexts: []string{"ProcessPayment"}},
		},
	}
	_, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for distinct names, got: %v", diags)
	}
}

// TestAnalyzeFile_DuplicateName exists as an alias for backward compat with
// older test references; the canonical test is TestAnalyzeFile_DuplicateActorName.
func TestAnalyzeFile_DuplicateName(t *testing.T) {
	TestAnalyzeFile_DuplicateActorName(t)
}
