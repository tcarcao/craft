package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestReceives_Trigger(t *testing.T) {
	for _, src := range []string{
		"use_case \"U\" {\n    when graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
		"use_case \"U\" {\n    when ad/graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
	} {
		g, _, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Fatalf("unexpected diags: %v", diags)
		}
		tr := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
		if tr.Kind() != lang.TriggerReceive {
			t.Fatalf("Kind = %s", tr.Kind())
		}
		if tr.Connector() != "a" || tr.PhraseText() != "reindex sweep" || tr.OpText() != "reindexAdverts" {
			t.Errorf("connector=%q phrase=%q op=%q", tr.Connector(), tr.PhraseText(), tr.OpText())
		}
		if tr.ContextName() == "" || tr.ActorName() != "" {
			t.Errorf("context=%q actor=%q", tr.ContextName(), tr.ActorName())
		}
		if got, want := tr.SourceText(), tr.ContextName()+" receives a reindex sweep [reindexAdverts]"; got != want {
			t.Errorf("SourceText = %q, want %q", got, want)
		}
	}
}

// A receives trigger without an operation bracket still parses; the phrase
// runs to the end of the line.
func TestReceives_TriggerWithoutOperation(t *testing.T) {
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when graphql receives the sweep\n        graphql returns the ids\n}")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tr := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
	if tr.Kind() != lang.TriggerReceive || tr.Connector() != "the" || tr.PhraseText() != "sweep" || tr.OpText() != "" {
		t.Errorf("kind=%s connector=%q phrase=%q op=%q", tr.Kind(), tr.Connector(), tr.PhraseText(), tr.OpText())
	}
}
