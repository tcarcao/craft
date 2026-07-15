package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// TestProject_ArchModifier_QuotedValueUnquoted covers Critical 1 from the
// task-8a ripple audit: ArchModifier.Value() returns a raw *SyntaxToken that
// may be SyntaxKindString (quotes included, per the Bug 8a round-trip fix).
// projectSimpleArchComponentFromView and projectFlowArchComponentFromView
// must route the value through stringAwareText so the projected modifier
// value is unquoted content, not raw quoted source text.
func TestProject_ArchModifier_QuotedValueUnquoted(t *testing.T) {
	src := `arch MyArch {
    presentation:
        WebApp[label:"some value", plain:bareword]
}`
	g, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	if len(doc.Architectures) != 1 {
		t.Fatalf("expected 1 arch, got %d", len(doc.Architectures))
	}
	comps := doc.Architectures[0].Presentation
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comps))
	}
	mods := comps[0].Modifiers
	if len(mods) != 2 {
		t.Fatalf("expected 2 modifiers, got %d: %+v", len(mods), mods)
	}
	if mods[0].Key != "label" || mods[0].Value != "some value" {
		t.Errorf("expected label modifier value 'some value' (unquoted), got key=%q value=%q", mods[0].Key, mods[0].Value)
	}
	if mods[1].Key != "plain" || mods[1].Value != "bareword" {
		t.Errorf("expected plain modifier value 'bareword', got key=%q value=%q", mods[1].Key, mods[1].Value)
	}
}

// TestProject_ArchModifier_FlowComponent_QuotedValueUnquoted covers the same
// Critical 1 defect on the flow-chain path (projectFlowArchComponentFromView),
// a > b chains being a structurally separate code path from the simple
// component projection above.
func TestProject_ArchModifier_FlowComponent_QuotedValueUnquoted(t *testing.T) {
	src := `arch MyArch {
    gateway:
        APIGateway[label:"rate limited"] > Backend
}`
	g, li, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	if len(doc.Architectures) != 1 {
		t.Fatalf("expected 1 arch, got %d", len(doc.Architectures))
	}
	comps := doc.Architectures[0].Gateway
	if len(comps) != 1 {
		t.Fatalf("expected 1 top-level component, got %d", len(comps))
	}
	chain := comps[0].Chain
	if len(chain) != 2 {
		t.Fatalf("expected 2-element chain, got %d: %+v", len(chain), chain)
	}
	mods := chain[0].Modifiers
	if len(mods) != 1 || mods[0].Key != "label" || mods[0].Value != "rate limited" {
		t.Errorf("expected label modifier value 'rate limited' (unquoted), got %+v", mods)
	}
}

// TestParse_ArchModifier_QuotedValue_RoundTrips locks in that the raw source
// (with quotes) still round-trips byte-for-byte through the green tree, even
// though the projected content above is unquoted — the two must not regress
// together.
func TestParse_ArchModifier_QuotedValue_RoundTrips(t *testing.T) {
	src := `arch MyArch {
    presentation:
        WebApp[label:"some value"]
}`
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %v", diags)
	}
	got := reassembleGreen(g)
	if got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}
}
