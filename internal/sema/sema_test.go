package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
)

// parseTreeFor is a test helper that parses a craft source string and returns
// the lossless syntax tree. Parse diagnostics are ignored so tests focus on
// the sema rule under test.
func parseTreeFor(src string) syntax.SyntaxNode {
	g, _, _ := syntax.Parse(src)
	return syntax.Root(g)
}

func TestAnalyzeFile_NoDuplicates(t *testing.T) {
	src := `
actor user Alice
actor system Bob
`
	syms, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(syms.Actors) != 2 {
		t.Fatalf("expected 2 actor symbols, got %d", len(syms.Actors))
	}
}

func TestAnalyzeFile_DuplicateActorName(t *testing.T) {
	src := `
actor user Alice
actor system Alice
`
	syms, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
	src := `
domain Payments {
  ProcessPayment
}
domain Payments {
  RefundPayment
}
`
	syms, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
	src := `
actor user Customer
domain Customer {
  Authentication
}
`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
	src := `
actor user Alice
domain Payments {
  ProcessPayment
}
`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
	src := `
services {
  PaymentService {
    contexts: Payment
    language: golang
  }
  UserService {
    contexts: User, Auth
  }
}
`
	syms, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(syms.Services) != 2 {
		t.Fatalf("expected 2 service symbols, got %d", len(syms.Services))
	}
}

func TestAnalyzeFile_DuplicateServiceName(t *testing.T) {
	src := `
services {
  PaymentService {
    contexts: Payment
  }
  PaymentService {
    contexts: Billing
  }
}
`
	syms, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
	src := `
use_case "User Login" {
  when Customer creates Session
    Auth validates credentials
}
use_case "User Login" {
  when Customer creates Session
    Auth validates token
}
`
	_, diags := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
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
		Actors: []sema.ActorSymbol{{Name: "Business_User", Type: "user", Line: 1, URI: "file:///a.craft"}},
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
		Actors: []sema.ActorSymbol{{Name: "Admin", Type: "user", Line: 1, URI: "file:///a.craft"}},
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
		Actors: []sema.ActorSymbol{{Name: "Customer", Type: "user", Line: 1, URI: "file:///a.craft"}},
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
	t.Skip("TODO(Task 10): EndLine() returns 0 until ast.go is wired to LineIndex")
	src := `
services {
  Svc {
    contexts: Ctx
  }
}
`
	syms, _ := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	if len(syms.Services) == 0 {
		t.Fatal("expected 1 service symbol")
	}
	// EndLine must be > 0 (the exact line depends on parser).
	if syms.Services[0].EndLine == 0 {
		t.Errorf("expected EndLine>0, got %d", syms.Services[0].EndLine)
	}
}

func TestAnalyzeFile_DomainEndLinePropagated(t *testing.T) {
	t.Skip("TODO(Task 10): EndLine() returns 0 until ast.go is wired to LineIndex")
	src := `
domain Commerce {
  Orders
}
`
	syms, _ := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	if len(syms.Domains) == 0 {
		t.Fatal("expected 1 domain symbol")
	}
	if syms.Domains[0].EndLine == 0 {
		t.Errorf("expected EndLine>0, got %d", syms.Domains[0].EndLine)
	}
}

func TestAnalyzeFile_UseCaseEndLinePropagated(t *testing.T) {
	t.Skip("TODO(Task 10): EndLine() returns 0 until ast.go is wired to LineIndex")
	src := `
use_case "DoThing" {
  when Foo initiates Bar
    Auth validates x
}
`
	syms, _ := sema.AnalyzeFile("file:///a.craft", parseTreeFor(src))
	if len(syms.UseCases) == 0 {
		t.Fatal("expected 1 use case symbol")
	}
	if syms.UseCases[0].EndLine == 0 {
		t.Errorf("expected EndLine>0, got %d", syms.UseCases[0].EndLine)
	}
}

func TestAnalyzeFile_ServiceIsGroupedPropagated(t *testing.T) {
	src := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	syms, _ := sema.AnalyzeFile("file:///test.craft", parseTreeFor(src))
	if len(syms.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(syms.Services))
	}
	if !syms.Services[0].IsGrouped {
		t.Error("expected IsGrouped=true for service inside services { } block")
	}
}

func TestAnalyzeFile_ServiceIsGrouped_TopLevel(t *testing.T) {
	src := "service OrderSvc {\n  contexts: Orders\n}"
	syms, _ := sema.AnalyzeFile("file:///test.craft", parseTreeFor(src))
	if len(syms.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(syms.Services))
	}
	if syms.Services[0].IsGrouped {
		t.Error("expected IsGrouped=false for top-level service")
	}
}

func TestAnalyzeFile_DomainIsGroupedPropagated(t *testing.T) {
	src := "domains {\n  Commerce {\n    Orders\n  }\n}"
	syms, _ := sema.AnalyzeFile("file:///test.craft", parseTreeFor(src))
	if len(syms.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(syms.Domains))
	}
	if !syms.Domains[0].IsGrouped {
		t.Error("expected IsGrouped=true for domain inside domains { } block")
	}
}

func TestAnalyzeFile_DomainIsGrouped_TopLevel(t *testing.T) {
	src := "domain Commerce {\n  Orders\n}"
	syms, _ := sema.AnalyzeFile("file:///test.craft", parseTreeFor(src))
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
	syms, _ := sema.AnalyzeFile("file:///test.craft", parseTreeFor(src))
	ws, _ := sema.MergeWorkspaceSymbols(map[string]sema.Symbols{"file:///test.craft": syms})

	// Line/Column are 0 until Task 10 wires up source-accurate positions.
	for _, name := range []string{"Login", "Logout"} {
		pos, ok := ws.BCPositions[name]
		if !ok {
			t.Errorf("BCPositions missing %q", name)
			continue
		}
		if pos.URI != "file:///test.craft" {
			t.Errorf("BCPositions[%q].URI: got %q", name, pos.URI)
		}
	}
}

func TestResolveUseCaseRef_BoundedContextCarriesBCPosition(t *testing.T) {
	// BC "Login" is inside domain "Auth". When resolved via a use-case ref,
	// the target must carry BCLine/BCColumn/BCURI pointing at the Login line.
	// NOTE: Line values are 0 until Task 10 wires up source-accurate positions.
	src := "domain Auth {\n  Login\n}\nuse_case \"T\" {\n  when Login initiates x\n    Login validates y\n}"
	tree := parseTreeFor(src)
	perFile := map[string]sema.Symbols{"file:///t.craft": func() sema.Symbols {
		s, _ := sema.AnalyzeFile("file:///t.craft", tree)
		return s
	}()}
	rm, _ := sema.AnalyzeWorkspace(perFile, func() sema.WorkspaceSymbols {
		ws, _ := sema.MergeWorkspaceSymbols(perFile)
		return ws
	}())

	// triggerLine is 0 until Task 10 wires up real line numbers.
	target, ok := sema.ResolveUseCaseRef(rm, "file:///t.craft", "Login", 0)
	if !ok {
		t.Fatal("ResolveUseCaseRef: Login not resolved")
	}
	if target.Kind != "bounded_context" {
		t.Fatalf("Kind: got %q want bounded_context", target.Kind)
	}
	// BCLine is 0 until Task 10 wires up source-accurate positions.
	if target.BCURI != "file:///t.craft" {
		t.Errorf("BCURI: got %q want file:///t.craft", target.BCURI)
	}
}
