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

func TestParse_UnsupportedKeywordEmitsWarning(t *testing.T) {
	// v2 does not yet support `services` — should emit a warning and not crash.
	src := `actor user Foo
services {
    SomeService {}
}`
	f, diags := syntax.Parse(src)
	if len(f.Actors) != 1 {
		t.Errorf("expected 1 actor, got %d", len(f.Actors))
	}
	if len(diags) == 0 {
		t.Error("expected a diagnostic for unsupported keyword")
	}
	for _, d := range diags {
		if d.Severity != "error" && d.Severity != "warning" {
			t.Errorf("unexpected severity %q", d.Severity)
		}
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
