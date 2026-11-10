package syntax_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// `alias:` (v2.38) is a known, repeatable property of a page and of a flow.
func TestAlias_Parse(t *testing.T) {
	src := "page p \"P\" {\n    product_domain: x\n    alias: performance\n    alias: \"ad performance\"\n}\n" +
		"flow f \"F\" {\n    priority: P1\n    owner: o\n    alias: promotion\n    alias: \"promoted ad\"\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	file := syntax.AsFile(syntax.Root(g))
	pg := file.Pages()[0]
	if got := pg.PropertyValues(lang.PageAlias); !slices.Equal(got, []string{"performance", "ad performance"}) {
		t.Errorf("page aliases = %v", got)
	}
	for _, p := range pg.Properties() {
		if p.Key() == lang.PageAlias && (!p.Known() || p.KeyToken().Kind() != syntax.SyntaxKindKwAlias) {
			t.Errorf("page alias key kind = %v", p.KeyToken().Kind())
		}
	}
	fl := file.Flows()[0]
	if got := fl.PropertyValues(lang.FlowAlias); !slices.Equal(got, []string{"promotion", "promoted ad"}) {
		t.Errorf("flow aliases = %v", got)
	}
	for _, p := range fl.Properties() {
		if p.Key() == lang.FlowAlias && (!p.Known() || p.KeyToken().Kind() != syntax.SyntaxKindKwAlias) {
			t.Errorf("flow alias key kind = %v", p.KeyToken().Kind())
		}
	}
}

// A comma list on an `alias:` line is an error that says to write one alias
// per line; the first value is kept.
func TestAlias_CommaListOnOneLine(t *testing.T) {
	for _, src := range []string{
		"page p \"P\" {\n    product_domain: x\n    alias: performance, stats\n}\n",
		"flow f \"F\" {\n    priority: P1\n    owner: o\n    alias: performance, stats\n}\n",
	} {
		g, _, diags := syntax.Parse(src)
		if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" || !strings.Contains(diags[0].Message, "one alias per line") {
			t.Fatalf("diags = %+v", diags)
		}
		file := syntax.AsFile(syntax.Root(g))
		var got []string
		if ps := file.Pages(); len(ps) == 1 {
			got = ps[0].PropertyValues(lang.PageAlias)
		} else {
			got = file.Flows()[0].PropertyValues(lang.FlowAlias)
		}
		if !slices.Equal(got, []string{"performance"}) {
			t.Errorf("aliases = %v", got)
		}
	}
}

// The unknown-property warnings list `alias:` among the keys.
func TestAlias_UnknownPropertyMessage(t *testing.T) {
	for _, src := range []string{"page p \"P\" {\n    product_domain: x\n    aka: y\n}\n", "flow f \"F\" {\n    priority: P1\n    owner: o\n    aka: y\n}\n"} {
		_, _, diags := syntax.Parse(src)
		if len(diags) != 1 || !strings.Contains(diags[0].Message, "`alias:`") {
			t.Errorf("diags = %+v", diags)
		}
	}
}
