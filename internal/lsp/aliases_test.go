package lsp_test

import (
	"encoding/json"
	"slices"
	"testing"
)

const aliasesDeclSrc = `page account-statistics "Ad Statistics" {
    product_domain: seller-analytics
    alias: performance
    alias: "ad performance"
}

flow vas-activation "VAS activation" {
    priority: P2
    owner: "VAS Excellence"
    alias: promotion
    alias: "promoted ad"
}
`

// The hover of a page or flow header lists its aliases (v2.38), and the
// `alias` key explains itself.
func TestAliases_Hover(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{pagesDeclURI, aliasesDeclSrc})
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{0, 8, "page account-statistics: Ad Statistics\nproduct domain: seller-analytics\naliases: performance, ad performance\n" +
			"used by no use case: add `entry: account-statistics` to the tags of the use cases that start on it"},
		{6, 8, "flow vas-activation: VAS activation\npriority: P2\nowner: VAS Excellence\naliases: promotion, promoted ad\n" +
			"realised by no use case: add `realises: vas-activation` to the tags of the use cases that serve it"},
		{2, 6, "alias: another name people use for the page, in any language or jargon; one `alias:` line per name. Optional"},
		{9, 6, "alias: another name people use for the flow, in any language or jargon; one `alias:` line per name. Optional"},
	} {
		var h struct {
			Contents struct{ Value string } `json:"contents"`
		}
		json.Unmarshal(c.atURI("textDocument/hover", pagesDeclURI, tc.line, tc.char, nil), &h) //nolint:errcheck
		if h.Contents.Value != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, h.Contents.Value, tc.want)
		}
	}
}

// `alias:` is offered in a page body and in a flow body.
func TestAliases_BodyCompletion(t *testing.T) {
	decl := "page p \"P\" {\n    product_domain: x\n    \n}\n\nflow f \"F\" {\n    priority: P1\n    \n}\n"
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{pagesDeclURI, decl})
	if got := completionLabelsAt(t, c, pagesDeclURI, 2, 4); !slices.Contains(got, "alias:") {
		t.Errorf("page body completion = %v", got)
	}
	if got := completionLabelsAt(t, c, pagesDeclURI, 7, 4); !slices.Contains(got, "alias:") {
		t.Errorf("flow body completion = %v", got)
	}
}
