package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func firstAction(t *testing.T, line string) syntax.ActionDecl {
	t.Helper()
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
	if len(diags) != 0 {
		t.Fatalf("%s: %v", line, diags)
	}
	return syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Actions()[0]
}

func TestOpMarkers(t *testing.T) {
	cases := []struct {
		line, text, verb string
		inferred         bool
		source           string
	}{
		{"g asks s to initiate the chat  [inferred POST /api/v1/x]", "POST /api/v1/x", "POST", true, "g asks s to initiate the chat [inferred POST /api/v1/x]"},
		{"g asks s to initiate the chat  [inferred]", "", "", true, "g asks s to initiate the chat [inferred]"},
		{"o asks f for the graph  [SQL investment flat]", "SQL investment flat", "SQL", false, ""},
		{`o asks s to take it  [ADDR "SNAPSHOT_DESTINATION"]`, `ADDR "SNAPSHOT_DESTINATION"`, "ADDR", false, ""},
		{"o asks s to take it  [inferred, x]", "inferred, x", "", false, ""},
		{"o notifies a.Done  [inferred TOPIC done]", "TOPIC done", "TOPIC", true, "o notifies a.Done [inferred TOPIC done]"},
		{"o returns the rows  [inferred]", "", "", true, "o returns the rows [inferred]"},
		{"o reads the rows  [inferred x]", "x", "", true, "o reads the rows [inferred x]"},
	}
	for _, c := range cases {
		a := firstAction(t, c.line)
		if a.OpText() != c.text || a.OpVerb() != c.verb || a.IsInferred() != c.inferred {
			t.Errorf("%s: text %q verb %q inferred %v", c.line, a.OpText(), a.OpVerb(), a.IsInferred())
		}
		if c.source != "" && a.SourceText() != c.source {
			t.Errorf("%s: SourceText %q", c.line, a.SourceText())
		}
	}
}

func TestOpMarkers_InferredIsAPayloadWordOnReceivesAndSurfaces(t *testing.T) {
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when g receives a sweep  [inferred x]\n        g does it\n}\nsurface g {\n    query the list  [inferred y]\n}")
	if len(diags) != 0 {
		t.Fatalf("%v", diags)
	}
	f := syntax.AsFile(syntax.Root(g))
	if got := f.UseCases()[0].Scenarios()[0].Trigger().OpText(); got != "inferred x" {
		t.Errorf("receives op = %q", got)
	}
	if got := f.Surfaces()[0].Operations()[0].OpText(); got != "inferred y" {
		t.Errorf("surface op = %q", got)
	}
}

func TestOpMarkers_EmptyEgress(t *testing.T) {
	for _, line := range []string{"o asks s to take it  [ADDR]", "o asks s for it  [SQL]", "o asks s for it  [a, SQL]", "o asks s for it  [inferred ADDR]"} {
		_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
		if len(diags) != 1 || diags[0].Code != "craft/syntax/empty-op-annotation" {
			t.Errorf("%s: %v", line, diags)
		}
	}
}
