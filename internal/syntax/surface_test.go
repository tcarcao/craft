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
