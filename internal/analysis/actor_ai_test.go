package analysis

import (
	"testing"
)

const actorAIDecls = `actors {
    system DescriptionGenerator "description generator" {
        ai: genai
        owner: "OLX Data"
        note "drafts the description"
    }
    system Siren "Siren"
}

actor system Bad {
    ai: llm
    owner: Toqan
}

actor system Twice {
    ai: ml
    ai: agentic
}
`

func TestActorAI_Model(t *testing.T) {
	res := Workspace([]FileResult{File("b.craft", "actor system DescriptionGenerator { ai: ml }\n"), File("a.craft", actorAIDecls)})
	doc := res.Doc
	dg := doc.ActorByName("DescriptionGenerator")
	if dg == nil || dg.AI != "genai" || dg.Owner != "OLX Data" || dg.DisplayName != "description generator" || len(dg.Notes) != 1 ||
		dg.Notes[0].Text != "drafts the description" || dg.Notes[0].Line != 5 {
		t.Fatalf("DescriptionGenerator = %+v (first declaration in sorted file order wins)", dg)
	}
	if s := doc.ActorByName("Siren"); s == nil || s.AI != "" || s.Owner != "" || s.Notes != nil {
		t.Errorf("Siren = %+v", s)
	}
	// An invalid kind is left out of the model; the owner stays.
	if b := doc.ActorByName("Bad"); b == nil || b.AI != "" || b.Owner != "Toqan" {
		t.Errorf("Bad = %+v", b)
	}
	if tw := doc.ActorByName("Twice"); tw == nil || tw.AI != "agentic" {
		t.Errorf("Twice = %+v (the last value wins)", tw)
	}
	if doc.ActorByName("Nobody") != nil {
		t.Error("ActorByName found an undeclared actor")
	}
}
