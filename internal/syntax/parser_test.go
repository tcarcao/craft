package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/syntax"
)

func TestParse_IndividualActor(t *testing.T) {
	f, diags := syntax.Parse("actor user Customer_Support")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(f.Actors))
	}
	a := f.Actors[0]
	if a.Name != "Customer_Support" || a.Type != ast.ActorTypeUser {
		t.Errorf("got %+v", a)
	}
}

func TestParse_ActorsBlock(t *testing.T) {
	src := `actors {
    user Alice
    system Bob
    service DB
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(f.Actors))
	}
}

func TestParse_MixedActors(t *testing.T) {
	src := `actors {
    user Alice
    system Bob
}

actor service DB`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 3 {
		t.Fatalf("expected 3 actors, got %d", len(f.Actors))
	}
}

func TestParse_UseCaseParsedWithActor(t *testing.T) {
	// S6: use_case is now supported — should parse cleanly alongside an actor.
	src := `actor user Foo
use_case "DoSomething" {
    when User does something
}`
	f, diags := syntax.Parse(src)
	if len(f.Actors) != 1 {
		t.Errorf("expected 1 actor, got %d", len(f.Actors))
	}
	if len(diags) != 0 {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
	if len(f.UseCases) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(f.UseCases))
	}
	uc := f.UseCases[0]
	if uc.Name != "DoSomething" {
		t.Errorf("expected use case name 'DoSomething', got %q", uc.Name)
	}
	if len(uc.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(uc.Scenarios))
	}
}

func TestParse_IndividualDomain(t *testing.T) {
	src := `domain ECommerce {
    User
    Product
    Order
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(f.Domains))
	}
	d := f.Domains[0]
	if d.Name != "ECommerce" {
		t.Errorf("expected name ECommerce, got %q", d.Name)
	}
	if len(d.BoundedContexts) != 3 {
		t.Errorf("expected 3 bounded contexts, got %d", len(d.BoundedContexts))
	}
}

func TestParse_DomainsBlock(t *testing.T) {
	src := `domains {
    Auth {
        Login
        Logout
    }
    Billing {
        Invoice
    }
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(f.Domains))
	}
	if f.Domains[0].Name != "Auth" || f.Domains[1].Name != "Billing" {
		t.Errorf("unexpected domain names: %v, %v", f.Domains[0].Name, f.Domains[1].Name)
	}
}

func TestParse_ActorsAndDomains(t *testing.T) {
	src := `actor user Customer
domain User {
    Authentication
    Profile
}`
	f, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(f.Actors) != 1 || len(f.Domains) != 1 {
		t.Errorf("expected 1 actor and 1 domain, got %d actors, %d domains", len(f.Actors), len(f.Domains))
	}
}

func TestParse_ExposureSimple(t *testing.T) {
	src := `actor user Business_User

exposure default {
    to: Business_User
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if exp.Name != "default" {
		t.Errorf("exposure name: got %q, want %q", exp.Name, "default")
	}
	if len(exp.To) != 1 || exp.To[0] != "Business_User" {
		t.Errorf("exposure.to: got %v", exp.To)
	}
}

func TestParse_ExposureWithThrough(t *testing.T) {
	src := `exposure api {
    to: Business_User, Customer_Support
    through: APIGateway
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if exp.Name != "api" {
		t.Errorf("exposure name: got %q, want %q", exp.Name, "api")
	}
	if len(exp.To) != 2 || exp.To[0] != "Business_User" || exp.To[1] != "Customer_Support" {
		t.Errorf("exposure.to: got %v", exp.To)
	}
	if len(exp.Through) != 1 || exp.Through[0] != "APIGateway" {
		t.Errorf("exposure.through: got %v", exp.Through)
	}
}

func TestParse_ExposureWithContexts(t *testing.T) {
	src := `exposure web {
    to: Business_User
    contexts: Authentication, Profile
    through: LoadBalancer
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(f.Exposures))
	}
	exp := f.Exposures[0]
	if len(exp.Contexts) != 2 {
		t.Errorf("exposure.contexts: got %v", exp.Contexts)
	}
}

func TestParse_MultipleExposures(t *testing.T) {
	src := `exposure PublicAPI {
    to: Business_User
    through: APIGateway
}

exposure InternalAPI {
    to: InternalSystem
    through: InternalGateway
}
`
	f, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected error: [%s] %s", d.Code, d.Message)
		}
	}
	if len(f.Exposures) != 2 {
		t.Fatalf("expected 2 exposures, got %d", len(f.Exposures))
	}
	if f.Exposures[0].Name != "PublicAPI" || f.Exposures[1].Name != "InternalAPI" {
		t.Errorf("wrong exposure names: %v", []string{f.Exposures[0].Name, f.Exposures[1].Name})
	}
}
