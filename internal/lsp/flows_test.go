package lsp_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const flowsRoot = "file:///ws"
const flowsDeclURI = "file:///ws/flows.craft"
const flowsUCURI = "file:///ws/uc.craft"

const flowsDeclSrc = `flow lead-message "Lead generation (phone and message)" {
    priority: P1
    owner: seeker-engagement
    metric: "Leads"
}

flow lead-phone "Lead by phone" {
    priority: P2
    owner: seeker-engagement
}
`

const flowsUCSrc = `use_case "Seeker sends a message" {
    tags {
        realises: lead-message, nope
    }

    when Seeker sends a message
        chat asks messaging to deliver the message
}
`

func flowsClient(t *testing.T, ucSrc string) *branchClient {
	return newWorkspaceClient(t, flowsRoot, "", wsFile{flowsUCURI, ucSrc}, wsFile{flowsDeclURI, flowsDeclSrc})
}

func TestFlows_Hover(t *testing.T) {
	c := flowsClient(t, flowsUCSrc)
	const leadMessage = "flow lead-message: Lead generation (phone and message)\npriority: P1\nowner: seeker-engagement\nmetric: Leads\n" +
		"realised by 1 use case:\n1. Seeker sends a message (uc.craft)"
	for _, tc := range []struct {
		uri        string
		line, char int
		want       string
	}{
		{flowsUCURI, 2, 22, leadMessage},
		{flowsUCURI, 2, 34, `flow "nope": no flow has this ref in the open workspace`},
		{flowsUCURI, 2, 10, "realises: the critical flows this use case serves, by ref (`flow <ref> \"<name>\" { … }`)"},
		{flowsDeclURI, 0, 8, leadMessage},
		{flowsDeclURI, 0, 1, leadMessage},
		{flowsDeclURI, 6, 6, "flow lead-phone: Lead by phone\npriority: P2\nowner: seeker-engagement\nrealised by no use case: add `realises: lead-phone` to the tags of the use cases that serve it"},
		{flowsDeclURI, 1, 6, "priority: P0, P1, P2, P3; P0 is the highest. Required"},
	} {
		var h struct {
			Contents struct{ Value string } `json:"contents"`
		}
		json.Unmarshal(c.atURI("textDocument/hover", tc.uri, tc.line, tc.char, nil), &h) //nolint:errcheck
		if h.Contents.Value != tc.want {
			t.Errorf("hover %s %d:%d = %q, want %q", tc.uri, tc.line, tc.char, h.Contents.Value, tc.want)
		}
	}
}

func TestFlows_Definition(t *testing.T) {
	c := flowsClient(t, flowsUCSrc)
	var locs []lspLocation
	json.Unmarshal(c.atURI("textDocument/definition", flowsUCURI, 2, 22, nil), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].URI != flowsDeclURI || locs[0].Range.Start.Line != 0 || locs[0].Range.Start.Character != 5 {
		t.Errorf("definition on a realises value = %+v, want flows.craft 0:5", locs)
	}
}

func completionLabelsAt(t *testing.T, c *branchClient, uri string, line, char int) []string {
	t.Helper()
	var list struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(c.atURI("textDocument/completion", uri, line, char, nil), &list); err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, it := range list.Items {
		labels = append(labels, it.Label)
	}
	return labels
}

func TestFlows_Completion(t *testing.T) {
	src := strings.Replace(flowsUCSrc, "realises: lead-message, nope", "realises: lead-message, ", 1)
	c := flowsClient(t, src)
	if got, want := completionLabelsAt(t, c, flowsUCURI, 2, 32), []string{"lead-phone"}; !slices.Equal(got, want) {
		t.Errorf("realises completion = %v, want %v", got, want)
	}
}

func TestFlows_BodyCompletion(t *testing.T) {
	decl := "flow f \"F\" {\n    priority: \n    \n}\n"
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{flowsDeclURI, decl})
	if got, want := completionLabelsAt(t, c, flowsDeclURI, 1, 14), []string{"P0", "P1", "P2", "P3"}; !slices.Equal(got, want) {
		t.Errorf("priority completion = %v, want %v", got, want)
	}
	if got := completionLabelsAt(t, c, flowsDeclURI, 2, 4); !slices.Contains(got, "owner:") || !slices.Contains(got, "metric:") {
		t.Errorf("flow body completion = %v", got)
	}
}

func TestFlows_TagKeyAndTopLevelCompletion(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{flowsUCURI, "use_case \"U\" {\n    tags {\n        \n    }\n    when A does it\n        b does it\n}\n\n"})
	if got := completionLabelsAt(t, c, flowsUCURI, 2, 8); !slices.Contains(got, "realises") {
		t.Errorf("tag keys = %v, want realises among them", got)
	}
	if got := completionLabelsAt(t, c, flowsUCURI, 7, 0); !slices.Contains(got, "flow") {
		t.Errorf("top-level keywords = %v, want flow among them", got)
	}
}

func TestFlows_OutlineAndFolding(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{flowsDeclURI, flowsDeclSrc})
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": flowsDeclURI}})
	var syms []outlineSymKind
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range syms {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"flow lead-message", "flow lead-phone"}) {
		t.Errorf("outline = %v", names)
	}
	var folds []struct{ StartLine, EndLine int }
	if err := json.Unmarshal(c.call("textDocument/foldingRange", p), &folds); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(folds, struct{ StartLine, EndLine int }{0, 4}) {
		t.Errorf("folds = %+v, want 0-4", folds)
	}
}
