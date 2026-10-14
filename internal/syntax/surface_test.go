package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const surfaceSrc = `surface concierge {
    callers: Advisor, unknown
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair"
    finding "a read-only credential is refused only at persistence"
    query the seeker list, unpaged, newest first  [getUserList]
    query the seeker audit trail                  [getUserLogs]
    command delete a seeker's account             [deleteAccount]
    query for the daily totals                    [GET /totals]
}
`

func TestSurface_Parse(t *testing.T) {
	g, _, diags := syntax.Parse(surfaceSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	ss := syntax.AsFile(syntax.Root(g)).Surfaces()
	if len(ss) != 1 {
		t.Fatalf("surfaces = %d", len(ss))
	}
	s := ss[0]
	if s.Subject() != "concierge" || strings.Join(s.Callers(), ",") != "Advisor,unknown" || s.Spec() != "obidozoe/docs/openapi.yaml" {
		t.Errorf("subject=%q callers=%v spec=%q", s.Subject(), s.Callers(), s.Spec())
	}
	if ns := s.Notes(); len(ns) != 2 || ns[1].Kind() != lang.NoteFinding {
		t.Errorf("notes = %+v", ns)
	}
	ops := s.Operations()
	var got []string
	for _, op := range ops {
		got = append(got, string(op.Kind())+"|"+op.Connector()+"|"+op.PhraseText()+"|"+op.OpText())
	}
	want := "query||the seeker list, unpaged, newest first|getUserList;" +
		"query||the seeker audit trail|getUserLogs;" +
		"command||delete a seeker's account|deleteAccount;" +
		"query|for|the daily totals|GET /totals"
	if strings.Join(got, ";") != want {
		t.Errorf("ops:\n%s\nwant:\n%s", strings.Join(got, ";"), want)
	}
	if v, p := ops[3].OpVerb(), ops[3].OpPayload(); v != "GET" || p != "/totals" {
		t.Errorf("verb=%q payload=%q", v, p)
	}
}

func TestSurface_QualifiedSubject(t *testing.T) {
	g, _, diags := syntax.Parse("surface obido/concierge {\n    query the list  [getUserList]\n}\n")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	ss := syntax.AsFile(syntax.Root(g)).Surfaces()
	if len(ss) != 1 || ss[0].Subject() != "obido/concierge" || ss[0].SubjectToken().Text() != "obido" {
		t.Fatalf("surfaces = %+v", ss)
	}
}

func TestSurface_WordStaysAName(t *testing.T) {
	src := "domain d {\n    surface\n}\nuse_case \"U\" {\n  when O does\n    surface asks query to run\n}"
	if _, _, diags := syntax.Parse(src); len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
}

func TestSurface_StopsAtNextTopLevel(t *testing.T) {
	// An unclosed surface must not swallow the next declaration.
	src := "surface concierge {\n    query the list  [getUserList]\nsurface other {\n    command x  [y]\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unclosed-block" {
		t.Fatalf("diags = %v", diags)
	}
	if ss := syntax.AsFile(syntax.Root(g)).Surfaces(); len(ss) != 2 || len(ss[1].Operations()) != 1 {
		t.Fatalf("surfaces = %d", len(ss))
	}
}

// An unclosed use case or domain stops at a declaration that starts at
// column 1, so the next surface (or any other top-level block) is not
// swallowed into its body.
func TestUnclosedBlock_StopsAtColumnOneDeclaration(t *testing.T) {
	const surface = "surface concierge {\n    query the list  [getUserList]\n}\n"
	for name, tc := range map[string]struct {
		src      string
		line     int // 1-based line of the unclosed-block diagnostic
		surfaces int
		domains  int
	}{
		"use case then surface":       {"use_case \"U\" {\n    when O does\n        x asks y for z\n" + surface, 1, 1, 0},
		"use case then domain":        {"use_case \"U\" {\n    when O does\n        x asks y for z\ndomain d {\n    c\n}\n", 1, 0, 1},
		"named scenario then surface": {"use_case \"U\" {\n    scenario \"s\"\n    when O does\n        alt \"a\"\n            x asks y for z\n" + surface, 1, 1, 0},
		"domain then surface":         {"domain d {\n    c\n" + surface, 1, 1, 1},
		"domain then use case":        {"domain d {\n    c\nuse_case \"U\" {\n    when O does\n        x does it\n}\n", 1, 0, 1},
		"domain then domain":          {"domain d {\n    c\ndomain e {\n    f\n}\n", 1, 0, 2},
	} {
		g, _, diags := syntax.Parse(tc.src)
		var unclosed []int
		for _, d := range diags {
			if d.Code == "craft/syntax/unclosed-block" {
				unclosed = append(unclosed, d.Range.Start.Line+1)
			}
		}
		if len(unclosed) != 1 || unclosed[0] != tc.line {
			t.Errorf("%s: unclosed-block lines = %v, want [%d]; diags = %v", name, unclosed, tc.line, diags)
		}
		file := syntax.AsFile(syntax.Root(g))
		if n := len(file.Surfaces()); n != tc.surfaces {
			t.Errorf("%s: surfaces = %d, want %d", name, n, tc.surfaces)
		}
		if n := len(file.Domains()); n != tc.domains {
			t.Errorf("%s: domains = %d, want %d", name, n, tc.domains)
		}
	}
}

// `actor <kind> <name>` at column 1 is a declaration too: an unclosed domain
// or use case stops there, and the actor is declared. A context line that is
// just `actor` still parses as a context.
func TestUnclosedBlock_StopsAtColumnOneActor(t *testing.T) {
	for name, src := range map[string]string{
		"domain then actor":   "domain d {\n    c\nactor user Seeker\n",
		"use case then actor": "use_case \"U\" {\n    when O does\n        x asks y for z\nactor system CRON\n",
	} {
		g, _, diags := syntax.Parse(src)
		var unclosed []int
		for _, d := range diags {
			if d.Code == "craft/syntax/unclosed-block" {
				unclosed = append(unclosed, d.Range.Start.Line+1)
			}
		}
		if len(unclosed) != 1 || unclosed[0] != 1 {
			t.Errorf("%s: unclosed-block lines = %v, want [1]; diags = %v", name, unclosed, diags)
		}
		if n := len(syntax.AsFile(syntax.Root(g)).Actors()); n != 1 {
			t.Errorf("%s: actors = %d, want 1", name, n)
		}
	}
	g, _, diags := syntax.Parse("domain d {\nactor\nc\n}\n")
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if n := len(syntax.AsFile(syntax.Root(g)).Actors()); n != 0 {
		t.Errorf("a bare `actor` context line became %d actors", n)
	}
}

// Indented, `surface` is still a step subject, and an unindented context list
// still parses.
func TestUnclosedBlock_IndentedSurfaceIsAnAction(t *testing.T) {
	src := "use_case \"U\" {\n    when O does\n        surface asks concierge for x\n}\ndomain d {\nservice\nc\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	file := syntax.AsFile(syntax.Root(g))
	acts := file.UseCases()[0].Scenarios()[0].Actions()
	if len(acts) != 1 || acts[0].SubjectName() != "surface" {
		t.Errorf("actions = %+v", acts)
	}
	if len(file.Domains()) != 1 {
		t.Errorf("domains = %d", len(file.Domains()))
	}
}

func TestSurface_OperationWithoutPhrase(t *testing.T) {
	for _, line := range []string{"query [bare]", "command for  [bare]"} {
		_, _, diags := syntax.Parse("surface concierge {\n    " + line + "\n}\n")
		if len(diags) != 1 || diags[0].Code != "craft/syntax/missing-phrase" {
			t.Errorf("%q: diags = %v, want one missing-phrase", line, diags)
		}
	}
}
