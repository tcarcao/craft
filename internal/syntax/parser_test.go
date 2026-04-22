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
	// v2 does not yet support `domain` — should emit a warning and not crash.
	src := `actor user Foo
domain SomeDomain {}`
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
