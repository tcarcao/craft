package lsp_test

import (
	"encoding/json"
	"slices"
	"testing"
)

const actorsAIURI = "file:///ws/actors.craft"

const actorsAISrc = `actors {
    system DescriptionGenerator "description generator" {
        ai: genai
        owner: "OLX Data"
    }
    system Siren "Siren"
}

domain Listing {
    ads
}

use_case "Seller writes an ad" {
    when DescriptionGenerator drafts a description
        ads asks Siren to check the text
}
`

func TestActorAI_Hover(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{actorsAIURI, actorsAISrc})
	const dg = "actor: DescriptionGenerator (system)\ndisplay name: description generator\nai: genai (a generative model)\nowner: OLX Data"
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{1, 12, dg},
		{13, 12, dg}, // a reference reads the model
		{5, 12, "actor: Siren (system)\ndisplay name: Siren"},
		{2, 9, "ai: the kind of AI system this actor is: ml (a trained model), genai (a generative model) or agentic (an agent that acts). The use cases that call it count towards AI coverage. Optional"},
		{3, 9, "owner: who runs the system, such as a team or a vendor. Optional"},
	} {
		var h struct {
			Contents struct{ Value string } `json:"contents"`
		}
		json.Unmarshal(c.atURI("textDocument/hover", actorsAIURI, tc.line, tc.char, nil), &h) //nolint:errcheck
		if h.Contents.Value != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, h.Contents.Value, tc.want)
		}
	}
}

func TestActorAI_BodyCompletion(t *testing.T) {
	decl := "actors {\n    system G {\n        ai: \n        \n    }\n}\n\nactor system H {\n    \n}\n"
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{actorsAIURI, decl})
	if got, want := completionLabelsAt(t, c, actorsAIURI, 2, 12), []string{"ml", "genai", "agentic"}; !slices.Equal(got, want) {
		t.Errorf("ai completion = %v, want %v", got, want)
	}
	for _, pos := range [][2]int{{3, 8}, {8, 4}} {
		if got, want := completionLabelsAt(t, c, actorsAIURI, pos[0], pos[1]), []string{"ai:", "owner:", "note"}; !slices.Equal(got, want) {
			t.Errorf("actor body completion at %v = %v, want %v", pos, got, want)
		}
	}
	// Outside a body, the actors block still offers actor kinds.
	if got := completionLabelsAt(t, c, actorsAIURI, 5, 0); slices.Contains(got, "ai:") {
		t.Errorf("completion after the body = %v", got)
	}
}

func TestActorAI_OutlineAndFolding(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{actorsAIURI, actorsAISrc})
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": actorsAIURI}})
	var syms []struct {
		Name, Detail string
		Range        struct {
			Start, End struct{ Line int }
		}
	}
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range syms {
		if s.Name == "DescriptionGenerator" {
			found = true
			if s.Detail != "actor: system, ai: genai" || s.Range.Start.Line != 1 || s.Range.End.Line != 4 {
				t.Errorf("symbol = %+v", s)
			}
		}
	}
	if !found {
		t.Errorf("outline = %+v", syms)
	}
	var folds []struct{ StartLine, EndLine int }
	if err := json.Unmarshal(c.call("textDocument/foldingRange", p), &folds); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(folds, struct{ StartLine, EndLine int }{1, 4}) {
		t.Errorf("folds = %+v, want 1-4", folds)
	}
}
