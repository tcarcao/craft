package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
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
			{Name: "Payments", Line: 1, BoundedContexts: []ast.BoundedContextEntry{{Name: "ProcessPayment"}}},
			{Name: "Payments", Line: 5, BoundedContexts: []ast.BoundedContextEntry{{Name: "RefundPayment"}}},
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
			{Name: "Customer", Line: 3, BoundedContexts: []ast.BoundedContextEntry{{Name: "Authentication"}}},
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
			{Name: "Payments", Line: 3, BoundedContexts: []ast.BoundedContextEntry{{Name: "ProcessPayment"}}},
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

func TestAnalyzeFile_ServiceEndLinePropagated(t *testing.T) {
	f := &ast.File{
		Services: []*ast.ServiceDecl{
			{Name: "Svc", Contexts: []string{"Ctx"}, Line: 2, EndLine: 5},
		},
	}
	syms, _ := sema.AnalyzeFile("file:///a.craft", f)
	if len(syms.Services) == 0 {
		t.Fatal("expected 1 service symbol")
	}
	if syms.Services[0].EndLine != 5 {
		t.Errorf("expected EndLine=5, got %d", syms.Services[0].EndLine)
	}
}

func TestAnalyzeFile_DomainEndLinePropagated(t *testing.T) {
	f := &ast.File{
		Domains: []*ast.DomainDecl{
			{Name: "Commerce", BoundedContexts: []ast.BoundedContextEntry{{Name: "Orders"}}, Line: 1, EndLine: 4},
		},
	}
	syms, _ := sema.AnalyzeFile("file:///a.craft", f)
	if len(syms.Domains) == 0 {
		t.Fatal("expected 1 domain symbol")
	}
	if syms.Domains[0].EndLine != 4 {
		t.Errorf("expected EndLine=4, got %d", syms.Domains[0].EndLine)
	}
}

func TestAnalyzeFile_UseCaseEndLinePropagated(t *testing.T) {
	f := &ast.File{
		Actors: []*ast.ActorDecl{{Name: "Foo", Type: ast.ActorTypeUser}},
		UseCases: []*ast.UseCaseDecl{
			{Name: "DoThing", Line: 2, EndLine: 6},
		},
	}
	syms, _ := sema.AnalyzeFile("file:///a.craft", f)
	if len(syms.UseCases) == 0 {
		t.Fatal("expected 1 use case symbol")
	}
	if syms.UseCases[0].EndLine != 6 {
		t.Errorf("expected EndLine=6, got %d", syms.UseCases[0].EndLine)
	}
}

func TestAnalyzeFile_ServiceIsGroupedPropagated(t *testing.T) {
	src := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	f, _ := syntax.Parse(src)
	syms, _ := sema.AnalyzeFile("file:///test.craft", f)
	if len(syms.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(syms.Services))
	}
	if !syms.Services[0].IsGrouped {
		t.Error("expected IsGrouped=true for service inside services { } block")
	}
}

func TestAnalyzeFile_ServiceIsGrouped_TopLevel(t *testing.T) {
	src := "service OrderSvc {\n  contexts: Orders\n}"
	f, _ := syntax.Parse(src)
	syms, _ := sema.AnalyzeFile("file:///test.craft", f)
	if len(syms.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(syms.Services))
	}
	if syms.Services[0].IsGrouped {
		t.Error("expected IsGrouped=false for top-level service")
	}
}

func TestAnalyzeFile_DomainIsGroupedPropagated(t *testing.T) {
	src := "domains {\n  Commerce {\n    Orders\n  }\n}"
	f, _ := syntax.Parse(src)
	syms, _ := sema.AnalyzeFile("file:///test.craft", f)
	if len(syms.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(syms.Domains))
	}
	if !syms.Domains[0].IsGrouped {
		t.Error("expected IsGrouped=true for domain inside domains { } block")
	}
}

func TestAnalyzeFile_DomainIsGrouped_TopLevel(t *testing.T) {
	src := "domain Commerce {\n  Orders\n}"
	f, _ := syntax.Parse(src)
	syms, _ := sema.AnalyzeFile("file:///test.craft", f)
	if len(syms.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(syms.Domains))
	}
	if syms.Domains[0].IsGrouped {
		t.Error("expected IsGrouped=false for top-level domain")
	}
}

func TestMergeWorkspaceSymbols_BCPositions(t *testing.T) {
	// domain Auth {\n  Login\n  Logout\n}
	// Login at line 2, col 3; Logout at line 3, col 3
	src := "domain Auth {\n  Login\n  Logout\n}"
	f, parseDiags := syntax.Parse(src)
	if len(parseDiags) != 0 {
		t.Fatalf("parse diagnostics: %v", parseDiags)
	}
	syms, _ := sema.AnalyzeFile("file:///test.craft", f)
	ws, _ := sema.MergeWorkspaceSymbols(map[string]sema.Symbols{"file:///test.craft": syms})

	for _, tc := range []struct {
		name string
		line int
		col  int
	}{
		{"Login", 2, 3},
		{"Logout", 3, 3},
	} {
		pos, ok := ws.BCPositions[tc.name]
		if !ok {
			t.Errorf("BCPositions missing %q", tc.name)
			continue
		}
		if pos.Line != tc.line {
			t.Errorf("BCPositions[%q].Line: got %d want %d", tc.name, pos.Line, tc.line)
		}
		if pos.Column != tc.col {
			t.Errorf("BCPositions[%q].Column: got %d want %d", tc.name, pos.Column, tc.col)
		}
		if pos.URI != "file:///test.craft" {
			t.Errorf("BCPositions[%q].URI: got %q", tc.name, pos.URI)
		}
	}
}

func TestResolveUseCaseRef_BoundedContextCarriesBCPosition(t *testing.T) {
	// BC "Login" is inside domain "Auth". When resolved via a use-case ref,
	// the target must carry BCLine/BCColumn/BCURI pointing at the Login line.
	src := "domain Auth {\n  Login\n}\nuse_case \"T\" {\n  when Login initiates x\n    Login validates y\n}"
	f, _ := syntax.Parse(src)
	perFile := map[string]sema.Symbols{"file:///t.craft": func() sema.Symbols { s, _ := sema.AnalyzeFile("file:///t.craft", f); return s }()}
	rm, _ := sema.AnalyzeWorkspace(perFile, func() sema.WorkspaceSymbols { ws, _ := sema.MergeWorkspaceSymbols(perFile); return ws }())

	target, ok := sema.ResolveUseCaseRef(rm, "file:///t.craft", "Login", 6)
	if !ok {
		t.Fatal("ResolveUseCaseRef: Login not resolved")
	}
	if target.Kind != "bounded_context" {
		t.Fatalf("Kind: got %q want bounded_context", target.Kind)
	}
	if target.BCLine != 2 {
		t.Errorf("BCLine: got %d want 2", target.BCLine)
	}
	if target.BCURI != "file:///t.craft" {
		t.Errorf("BCURI: got %q want file:///t.craft", target.BCURI)
	}
}
