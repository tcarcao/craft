package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestLinks(t *testing.T) {
	src := "use_case \"U\" {\n    when Seller promotes an advert\n        includes \"Site session preamble\"\n        alt \"pay now\"\n            continues \"Seller pays at the checkout\"\n        includes ships the order\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	steps := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Steps()
	l, ok := steps[0].Link()
	if !ok || l.Kind() != lang.LinkIncludes || l.Title() != "Site session preamble" {
		t.Fatalf("first step = %+v", steps[0])
	}
	blk, _ := steps[1].Block()
	if l, ok := blk.Branches()[0].Steps()[0].Link(); !ok || l.Kind() != lang.LinkContinues {
		t.Error("continues inside an alt")
	}
	if a, ok := steps[2].Action(); !ok || a.SubjectName() != "includes" {
		t.Error("`includes ships the order` is an action whose subject is includes")
	}
}

func TestLinks_MissingTitle(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        includes\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/missing-label" {
		t.Fatalf("diags = %v", diags)
	}
}

func TestLinks_ExtraTokensAfterTitle(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        continues \"B\" now\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" {
		t.Fatalf("diags = %v", diags)
	}
}
