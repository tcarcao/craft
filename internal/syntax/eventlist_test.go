package syntax_test

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func listensTrigger(t *testing.T, src string) syntax.TriggerDecl {
	t.Helper()
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	return syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
}

func itemValues(items []syntax.EventItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Value)
	}
	return out
}

func TestListensList_OneLine(t *testing.T) {
	tr := listensTrigger(t, "use_case \"U\" {\n    when seshat listens sns.a-changed, \"b changed\", c.C\n        seshat writes the row\n}")
	if got := itemValues(tr.EventItems()); !slices.Equal(got, []string{"sns.a-changed", "b changed", "c.C"}) {
		t.Errorf("items = %v", got)
	}
	if tr.EventValue() != "sns.a-changed" || tr.EventRef() != "sns.a-changed" {
		t.Errorf("first item: value %q ref %q", tr.EventValue(), tr.EventRef())
	}
	if items := tr.EventItems(); !items[1].IsString || items[1].Ref != "" || items[2].Ref != "c.C" {
		t.Errorf("item kinds = %+v", items)
	}
}

func TestListensList_ContinuesAfterATrailingComma(t *testing.T) {
	src := "use_case \"U\" {\n    when seshat listens sns.a,\n            // the vas topics\n            sns.b,\n            sns.c every \"5m\"\n        seshat writes the row\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := itemValues(sc.Trigger().EventItems()); !slices.Equal(got, []string{"sns.a", "sns.b", "sns.c"}) {
		t.Errorf("items = %v", got)
	}
	if _, e, ok := sc.Trigger().ListenSchedule(); !ok || e != "5m" {
		t.Errorf("schedule = %q %v", e, ok)
	}
	if n := len(sc.Steps()); n != 1 {
		t.Errorf("%d steps, want 1: a continuation line is never a step", n)
	}
}

func TestListensList_SingleItemKeepsTheV223Tree(t *testing.T) {
	tr := listensTrigger(t, "use_case \"U\" {\n    when seshat listens sns.a\n        seshat writes the row\n}")
	if tr.Node().ChildNode(syntax.SyntaxKindEventList) != nil {
		t.Error("a single item must not be wrapped in an EventList")
	}
	if got := itemValues(tr.EventItems()); !slices.Equal(got, []string{"sns.a"}) {
		t.Errorf("items = %v", got)
	}
}

func TestListensList_DanglingCommaBeforeAStep(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when seshat listens sns.a,\n        seshat writes the row\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" || diags[0].Range.Start.Line != 1 {
		t.Fatalf("diags = %v", diags)
	}
}
