package view

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

const flowSrc = `services {
  S {
    contexts: A
  }
}

domain D {
  A
  B
}

use_case "Run" {
  when A starts
    A asks B to get x  [GET /x]
    A asks B to put y  [PUT /y]
    A notifies a.Done
}
`

func buildFlowFor(t *testing.T) Flow {
	t.Helper()
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("m.craft", flowSrc)}).Doc
	return BuildFlow(doc, interaction.Derive(doc))
}

func TestBuildFlow_EdgeVerb(t *testing.T) {
	var got []string
	for _, e := range buildFlowFor(t).Edges {
		if e.Style == StyleCall || e.Style == StyleAsync {
			got = append(got, e.Verb)
		}
	}
	if want := []string{"GET", "PUT", ""}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("edge verbs = %q; want %q", got, want)
	}
}

func TestBuildFlow_NodeParticipantAndService(t *testing.T) {
	f := buildFlowFor(t)
	byLabel := map[string]Node{}
	for _, n := range append(f.Contexts, f.Queues...) {
		byLabel[n.Label] = n
	}
	if a := byLabel["A"]; a.Participant != model.ParticipantBoundedContext || a.Service != "S" {
		t.Errorf("A = %+v; want bounded context in S", a)
	}
	if b := byLabel["B"]; b.Service != "" {
		t.Errorf("B service = %q; want none", b.Service)
	}
	if q := byLabel["A events"]; q.Participant != "" || q.Service != "" {
		t.Errorf("queue = %+v; want neither", q)
	}
}
