package lsp_test

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestCapabilities_Advertised: a handler and its capability land together
// (docs/AGENT.md §3).
func TestCapabilities_Advertised(t *testing.T) {
	_, initResp := newBranchClient(t, "")
	var res struct {
		Capabilities struct {
			ReferencesProvider *bool `json:"referencesProvider"`
			CodeActionProvider *struct {
				CodeActionKinds []string `json:"codeActionKinds"`
			} `json:"codeActionProvider"`
			DocumentLinkProvider *struct{} `json:"documentLinkProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(initResp, &res); err != nil {
		t.Fatal(err)
	}
	caps := res.Capabilities
	if caps.ReferencesProvider == nil || !*caps.ReferencesProvider {
		t.Error("referencesProvider not advertised")
	}
	if caps.CodeActionProvider == nil || !slices.Contains(caps.CodeActionProvider.CodeActionKinds, "quickfix") {
		t.Errorf("codeActionProvider with quickfix not advertised: %+v", caps.CodeActionProvider)
	}
	if caps.DocumentLinkProvider != nil {
		t.Error("documentLinkProvider advertised without sourceRoots")
	}
}

// Document links are advertised only with initializationOptions.sourceRoots
// (spec §14.6): without them the handler has nothing to link.
func TestCapabilities_DocumentLinkWithSourceRoots(t *testing.T) {
	for _, tc := range []struct {
		opts string
		want bool
	}{{`{"sourceRoots": {"*": "/src"}}`, true}, {`{"sourceRoots": {}}`, false}, {`{}`, false}} {
		var res struct {
			Capabilities struct {
				DocumentLinkProvider *struct{} `json:"documentLinkProvider"`
			} `json:"capabilities"`
		}
		if err := json.Unmarshal(initializeWith(t, tc.opts), &res); err != nil {
			t.Fatal(err)
		}
		if got := res.Capabilities.DocumentLinkProvider != nil; got != tc.want {
			t.Errorf("%s: documentLinkProvider advertised = %v, want %v", tc.opts, got, tc.want)
		}
	}
}
