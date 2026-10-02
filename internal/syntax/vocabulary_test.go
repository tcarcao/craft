package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// The parser reads its verbs from internal/lang. This guards that every verb
// there really parses in its construct, so a new verb cannot be added to the
// vocabulary without the grammar accepting it.
func TestParserAcceptsEveryLangVerb(t *testing.T) {
	for _, v := range lang.EdgeVerbs() {
		src := "domain d { a b }\ncontext_map d {\n    a " + v.Verb + " b\n}\n"
		g, _, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Errorf("context_map verb %q: diagnostics %v", v.Verb, diags)
			continue
		}
		edges := syntax.AsFile(syntax.Root(g)).ContextMaps()[0].Edges()
		if len(edges) != 1 || edges[0].Verb() != v.Verb {
			t.Errorf("context_map verb %q not recognised", v.Verb)
		}
	}
	for _, verb := range lang.GlossaryVerbs() {
		src := "domain d { a b }\nglossary d {\n    a/term " + verb + " b/term\n}\n"
		g, _, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Errorf("glossary verb %q: diagnostics %v", verb, diags)
			continue
		}
		rels := syntax.AsFile(syntax.Root(g)).Glossaries()[0].Relations()
		if len(rels) != 1 || rels[0].Verb() != verb {
			t.Errorf("glossary verb %q not recognised", verb)
		}
	}
	for _, verb := range lang.ProtocolVerbs() {
		src := "use_case \"U\" {\n    when Bob does it\n        A asks B to do it [" + verb + " /x]\n}\n"
		g, _, _ := syntax.Parse(src)
		uc := syntax.AsFile(syntax.Root(g)).UseCases()[0]
		if got := uc.Scenarios()[0].Actions()[0].OpVerb(); got != verb {
			t.Errorf("protocol verb %q parsed as %q", verb, got)
		}
	}
}
