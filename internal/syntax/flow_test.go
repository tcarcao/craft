package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const flowSrc = `flow lead-message "Lead generation (phone and message)" {
    priority: P1
    owner: seeker-engagement
    metric: "Leads"
    note "counted once per seeker and advert"
    finding "the phone reveal is not tracked on iOS"
}
`

func flowCodes(diags []model.Diagnostic) string { return strings.Join(codes(diags), ",") }

func TestFlow_Parse(t *testing.T) {
	g, _, diags := syntax.Parse(flowSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	fs := syntax.AsFile(syntax.Root(g)).Flows()
	if len(fs) != 1 {
		t.Fatalf("flows = %d", len(fs))
	}
	f := fs[0]
	if f.Ref() != "lead-message" || f.Name() != "Lead generation (phone and message)" {
		t.Errorf("ref=%q name=%q", f.Ref(), f.Name())
	}
	var props []string
	for _, p := range f.Properties() {
		props = append(props, p.Key()+"="+p.Value())
		if !p.Known() {
			t.Errorf("%q should be known", p.Key())
		}
	}
	if got, want := strings.Join(props, ";"), "priority=P1;owner=seeker-engagement;metric=Leads"; got != want {
		t.Errorf("properties = %q, want %q", got, want)
	}
	if v, ok := f.Property(lang.FlowOwner); !ok || v != "seeker-engagement" {
		t.Errorf("owner = %q, %v", v, ok)
	}
	if ns := f.Notes(); len(ns) != 2 || ns[1].Kind() != lang.NoteFinding {
		t.Errorf("notes = %+v", ns)
	}
}

// `flow` is contextual: a flow needs `<ref> "<name>" {` on its line, so the
// word keeps working as a bounded context, a subject, a verb or a phrase word.
func TestFlow_WordStaysAName(t *testing.T) {
	src := "domain d {\n    flow\n}\nuse_case \"U\" {\n  when flow starts the flow\n    flow asks billing to flow the money\n    flow notifies flow.Started\n}\nservice flow {\n  contexts: flow\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if fs := syntax.AsFile(syntax.Root(g)).Flows(); len(fs) != 0 {
		t.Fatalf("flows = %d, want none", len(fs))
	}
}

func TestFlow_MissingName(t *testing.T) {
	for _, src := range []string{
		"flow lead-message {\n    priority: P1\n}\n",
		"flow lead-message \"\" {\n    priority: P1\n}\n",
	} {
		g, _, diags := syntax.Parse(src)
		if flowCodes(diags) != "craft/syntax/missing-label" {
			t.Errorf("%q: diags = %v", src, diags)
		}
		if fs := syntax.AsFile(syntax.Root(g)).Flows(); len(fs) != 1 || fs[0].Ref() != "lead-message" {
			t.Errorf("%q: flows = %+v", src, fs)
		}
	}
}

func TestFlow_MissingRef(t *testing.T) {
	g, _, diags := syntax.Parse("flow \"Lead generation\" {\n    priority: P1\n}\n")
	if flowCodes(diags) != "craft/syntax/unexpected-token" {
		t.Fatalf("diags = %v", diags)
	}
	fs := syntax.AsFile(syntax.Root(g)).Flows()
	if len(fs) != 1 || fs[0].Ref() != "" || fs[0].Name() != "Lead generation" {
		t.Fatalf("flows = %+v", fs)
	}
}

func TestFlow_UnknownProperty(t *testing.T) {
	g, _, diags := syntax.Parse("flow f \"F\" {\n    priority: P1\n    team: growth\n}\n")
	if flowCodes(diags) != "craft/syntax/unknown-flow-property" || diags[0].Severity != model.SeverityWarning {
		t.Fatalf("diags = %v", diags)
	}
	if r := diags[0].Range; r.Start.Line != 2 || r.Start.Character != 4 {
		t.Errorf("range = %+v", r)
	}
	props := syntax.AsFile(syntax.Root(g)).Flows()[0].Properties()
	if len(props) != 2 || props[1].Known() || props[1].Value() != "growth" {
		t.Errorf("properties = %+v", props)
	}
}

func TestFlow_BadLines(t *testing.T) {
	cases := []struct{ src, want string }{
		{"flow f \"F\" {\n    priority:\n}\n", "craft/syntax/unexpected-token"},
		{"flow f \"F\" {\n    metric: Leads per day\n}\n", "craft/syntax/unexpected-token"},
		{"flow f \"F\" {\n    the lead flow\n}\n", "craft/syntax/unexpected-token"},
		{"flow f \"F\" {\n    priority: P1\n", "craft/syntax/unclosed-block"},
	}
	for _, c := range cases {
		_, _, diags := syntax.Parse(c.src)
		if flowCodes(diags) != c.want {
			t.Errorf("%q: diags = %v, want %s", c.src, diags, c.want)
		}
	}
}

func TestFlow_StopsAtNextTopLevel(t *testing.T) {
	src := "flow a \"A\" {\n    priority: P1\nflow b \"B\" {\n    priority: P2\n}\nuse_case \"U\" {\n  when A does\n    x does y\n}\n"
	g, _, diags := syntax.Parse(src)
	if flowCodes(diags) != "craft/syntax/unclosed-block" {
		t.Fatalf("diags = %v", diags)
	}
	file := syntax.AsFile(syntax.Root(g))
	if fs := file.Flows(); len(fs) != 2 || fs[1].Ref() != "b" {
		t.Fatalf("flows = %+v", fs)
	}
	if len(file.UseCases()) != 1 {
		t.Fatalf("use cases = %d", len(file.UseCases()))
	}
}

// An unclosed use case stops at a flow at column 1.
func TestFlow_EndsUnclosedUseCase(t *testing.T) {
	src := "use_case \"U\" {\n  when A does\n    x does y\nflow a \"A\" {\n    priority: P1\n    owner: o\n}\n"
	g, _, diags := syntax.Parse(src)
	if flowCodes(diags) != "craft/syntax/unclosed-block" {
		t.Fatalf("diags = %v", diags)
	}
	if fs := syntax.AsFile(syntax.Root(g)).Flows(); len(fs) != 1 {
		t.Fatalf("flows = %+v", fs)
	}
}
