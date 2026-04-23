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

// S5: service-related sema tests.

func TestAnalyzeFile_ServiceSymbolsCollected(t *testing.T) {
	f := &ast.File{
		Services: []*ast.ServiceDecl{
			{Name: "PaymentService", Contexts: []string{"Payment"}, Language: "golang", Line: 1},
			{Name: "UserService", Contexts: []string{"User", "Auth"}, Line: 3},
		},
	}
	syms, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(syms.Services) != 2 {
		t.Fatalf("expected 2 service symbols, got %d", len(syms.Services))
	}
}

func TestAnalyzeFile_DuplicateServiceName(t *testing.T) {
	f := &ast.File{
		Services: []*ast.ServiceDecl{
			{Name: "PaymentService", Contexts: []string{"Payment"}, Line: 1},
			{Name: "PaymentService", Contexts: []string{"Billing"}, Line: 5},
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
	if len(syms.Services) != 1 {
		t.Errorf("expected 1 service symbol (first declaration only), got %d", len(syms.Services))
	}
}

func TestAnalyzeWorkspace_ResolvesServiceContext(t *testing.T) {
	fileA := sema.Symbols{
		Domains: []sema.DomainSymbol{
			{Name: "Auth", BoundedContexts: []string{"Login", "Registration"}, Line: 1, URI: "file:///a.craft"},
		},
	}
	fileB := sema.Symbols{
		Services: []sema.ServiceSymbol{
			{Name: "UserService", Contexts: []string{"Login"}, Line: 1, URI: "file:///b.craft"},
		},
	}
	perFile := map[string]sema.Symbols{
		"file:///a.craft": fileA,
		"file:///b.craft": fileB,
	}
	ws, mergeDiags := sema.MergeWorkspaceSymbols(perFile)
	rm, diags := sema.AnalyzeWorkspace(perFile, ws)
	allDiags := append(mergeDiags, diags...)

	if len(allDiags) != 0 {
		t.Fatalf("unexpected workspace diagnostics: %v", allDiags)
	}

	domSym, ok := sema.ResolveServiceContext(rm, "file:///b.craft", "UserService", "Login")
	if !ok {
		t.Fatal("expected Login to resolve to a domain")
	}
	if domSym.Name != "Auth" {
		t.Errorf("expected domain 'Auth', got %q", domSym.Name)
	}
}

func TestAnalyzeWorkspace_UnresolvedContext(t *testing.T) {
	fileB := sema.Symbols{
		Services: []sema.ServiceSymbol{
			{Name: "UserService", Contexts: []string{"UnknownContext"}, Line: 1, URI: "file:///b.craft"},
		},
	}
	perFile := map[string]sema.Symbols{
		"file:///b.craft": fileB,
	}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)

	if len(diags) != 1 {
		t.Fatalf("expected 1 unresolved-reference diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/unresolved-reference" {
		t.Errorf("expected code craft/sema/unresolved-reference, got %q", diags[0].Code)
	}
	if diags[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", diags[0].Severity)
	}
}

func TestAnalyzeFile_DuplicateUseCaseName(t *testing.T) {
	f := &ast.File{
		UseCases: []*ast.UseCaseDecl{
			{Name: "User Login", Line: 1},
			{Name: "User Login", Line: 7},
		},
	}
	_, diags := sema.AnalyzeFile("file:///a.craft", f)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/duplicate-use-case-name" {
		t.Errorf("expected code craft/sema/duplicate-use-case-name, got %q", diags[0].Code)
	}
	if diags[0].Severity != "error" {
		t.Errorf("expected error severity, got %q", diags[0].Severity)
	}
}

func TestAnalyzeWorkspace_ExposureValidTarget(t *testing.T) {
	syms := sema.Symbols{
		Actors: []sema.ActorSymbol{{Name: "Business_User", Type: ast.ActorTypeUser, Line: 1, URI: "file:///a.craft"}},
		Exposures: []sema.ExposureSymbol{
			{Name: "default", To: []string{"Business_User"}, Through: []string{"APIGateway"}, Line: 3, URI: "file:///a.craft"},
		},
	}
	perFile := map[string]sema.Symbols{"file:///a.craft": syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	for _, d := range diags {
		if d.Code == "craft/sema/invalid-exposure-target" {
			t.Errorf("unexpected invalid-exposure-target diagnostic: %v", d)
		}
	}
}

func TestAnalyzeWorkspace_ExposureTo_TargetIsDomain(t *testing.T) {
	syms := sema.Symbols{
		Domains: []sema.DomainSymbol{{Name: "Payments", Line: 1, URI: "file:///a.craft"}},
		Exposures: []sema.ExposureSymbol{
			{Name: "default", To: []string{"Payments"}, Line: 5, URI: "file:///a.craft"},
		},
	}
	perFile := map[string]sema.Symbols{"file:///a.craft": syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/invalid-exposure-target" {
			found = true
			if d.Severity != "error" {
				t.Errorf("expected error severity, got %q", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected invalid-exposure-target diagnostic for domain in `to:`, got none: %v", diags)
	}
}

func TestAnalyzeWorkspace_ExposureTo_TargetIsService(t *testing.T) {
	syms := sema.Symbols{
		Services: []sema.ServiceSymbol{{Name: "UserService", Line: 1, URI: "file:///a.craft"}},
		Exposures: []sema.ExposureSymbol{
			{Name: "default", To: []string{"UserService"}, Line: 5, URI: "file:///a.craft"},
		},
	}
	perFile := map[string]sema.Symbols{"file:///a.craft": syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/invalid-exposure-target" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-exposure-target diagnostic for service in `to:`, got none: %v", diags)
	}
}

func TestAnalyzeWorkspace_ExposureThrough_TargetIsActor(t *testing.T) {
	syms := sema.Symbols{
		Actors: []sema.ActorSymbol{{Name: "Admin", Type: ast.ActorTypeUser, Line: 1, URI: "file:///a.craft"}},
		Exposures: []sema.ExposureSymbol{
			{Name: "default", To: []string{"APIUser"}, Through: []string{"Admin"}, Line: 5, URI: "file:///a.craft"},
		},
	}
	perFile := map[string]sema.Symbols{"file:///a.craft": syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/invalid-exposure-target" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-exposure-target diagnostic for actor in `through:`, got none: %v", diags)
	}
}

func TestAnalyzeWorkspace_ExposureContexts_TargetIsActor(t *testing.T) {
	syms := sema.Symbols{
		Actors: []sema.ActorSymbol{{Name: "Customer", Type: ast.ActorTypeUser, Line: 1, URI: "file:///a.craft"}},
		Exposures: []sema.ExposureSymbol{
			{Name: "default", To: []string{"ExternalUser"}, Contexts: []string{"Customer"}, Line: 5, URI: "file:///a.craft"},
		},
	}
	perFile := map[string]sema.Symbols{"file:///a.craft": syms}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	_, diags := sema.AnalyzeWorkspace(perFile, ws)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/invalid-exposure-target" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-exposure-target diagnostic for actor in `contexts:`, got none: %v", diags)
	}
}
