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

func TestAnalyzeFile_DuplicateName(t *testing.T) {
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
	// Duplicate is not added to symbols.
	if len(syms.Actors) != 1 {
		t.Errorf("expected 1 symbol (first declaration only), got %d", len(syms.Actors))
	}
}
