package lsp_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

const pagesDeclURI = "file:///ws/pages.craft"

const pagesDeclSrc = `page account-adverts "My Account: My Ads" {
    url: "https://www.otodom.pl/wiadomosci/moje-ogloszenia"
    platform: frontoffice
    product_domain: ad-inventory
}

page ad-detail "Ad detail" {
    product_domain: seeker-experience
}
`

const pagesUCSrc = `use_case "Seller promotes an ad" {
    tags {
        entry: account-adverts, nope
    }

    when Seller promotes an ad
        promotion asks billing to charge the seller
}
`

func pagesClient(t *testing.T, ucSrc string) *branchClient {
	return newWorkspaceClient(t, flowsRoot, "", wsFile{flowsUCURI, ucSrc}, wsFile{pagesDeclURI, pagesDeclSrc})
}

func TestPages_Hover(t *testing.T) {
	c := pagesClient(t, pagesUCSrc)
	const accountAdverts = "page account-adverts: My Account: My Ads\nurl: https://www.otodom.pl/wiadomosci/moje-ogloszenia\nplatform: frontoffice\nproduct domain: ad-inventory\n" +
		"used by 1 use case:\n1. Seller promotes an ad (uc.craft)"
	for _, tc := range []struct {
		uri        string
		line, char int
		want       string
	}{
		{flowsUCURI, 2, 18, accountAdverts},
		{flowsUCURI, 2, 33, `page "nope": no page has this ref in the open workspace`},
		{flowsUCURI, 2, 10, "entry: the pages this use case starts on, by ref (`page <ref> \"<name>\" { … }`)"},
		{pagesDeclURI, 0, 8, accountAdverts},
		{pagesDeclURI, 0, 1, accountAdverts},
		{pagesDeclURI, 6, 6, "page ad-detail: Ad detail\nproduct domain: seeker-experience\nused by no use case: add `entry: ad-detail` to the tags of the use cases that start on it"},
		{pagesDeclURI, 2, 6, "platform: frontoffice, backoffice, app. Optional"},
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

func TestPages_Definition(t *testing.T) {
	c := pagesClient(t, pagesUCSrc)
	var locs []lspLocation
	json.Unmarshal(c.atURI("textDocument/definition", flowsUCURI, 2, 18, nil), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].URI != pagesDeclURI || locs[0].Range.Start.Line != 0 || locs[0].Range.Start.Character != 5 {
		t.Errorf("definition on an entry value = %+v, want pages.craft 0:5", locs)
	}
}

func TestPages_Completion(t *testing.T) {
	src := strings.Replace(pagesUCSrc, "entry: account-adverts, nope", "entry: account-adverts, ", 1)
	c := pagesClient(t, src)
	if got, want := completionLabelsAt(t, c, flowsUCURI, 2, 32), []string{"ad-detail"}; !slices.Equal(got, want) {
		t.Errorf("entry completion = %v, want %v", got, want)
	}
}

func TestPages_BodyCompletion(t *testing.T) {
	decl := "page p \"P\" {\n    platform: \n    \n}\n"
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{pagesDeclURI, decl})
	if got, want := completionLabelsAt(t, c, pagesDeclURI, 1, 14), []string{"frontoffice", "backoffice", "app"}; !slices.Equal(got, want) {
		t.Errorf("platform completion = %v, want %v", got, want)
	}
	if got := completionLabelsAt(t, c, pagesDeclURI, 2, 4); !slices.Contains(got, "url:") || !slices.Contains(got, "product_domain:") {
		t.Errorf("page body completion = %v", got)
	}
}

func TestPages_TagKeyAndTopLevelCompletion(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{flowsUCURI, "use_case \"U\" {\n    tags {\n        \n    }\n    when A does it\n        b does it\n}\n\n"})
	if got := completionLabelsAt(t, c, flowsUCURI, 2, 8); !slices.Contains(got, "entry") {
		t.Errorf("tag keys = %v, want entry among them", got)
	}
	if got := completionLabelsAt(t, c, flowsUCURI, 7, 0); !slices.Contains(got, "page") {
		t.Errorf("top-level keywords = %v, want page among them", got)
	}
}

func TestPages_OutlineAndFolding(t *testing.T) {
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{pagesDeclURI, pagesDeclSrc})
	p, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": pagesDeclURI}})
	var syms []outlineSymKind
	if err := json.Unmarshal(c.call("textDocument/documentSymbol", p), &syms); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range syms {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"page account-adverts", "page ad-detail"}) {
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

// A second declaration of a ref says which declaration the model keeps.
func TestPages_HoverOnDuplicate(t *testing.T) {
	dup := "page account-adverts \"Again\" {\n    product_domain: other\n}\n"
	c := newWorkspaceClient(t, flowsRoot, "", wsFile{"file:///ws/zz.craft", dup}, wsFile{pagesDeclURI, pagesDeclSrc})
	var h struct {
		Contents struct{ Value string } `json:"contents"`
	}
	json.Unmarshal(c.atURI("textDocument/hover", "file:///ws/zz.craft", 0, 8, nil), &h) //nolint:errcheck
	if !strings.HasPrefix(h.Contents.Value, "page account-adverts is declared again here; the page is the one at pages.craft:1\npage account-adverts: My Account: My Ads") {
		t.Errorf("hover = %q", h.Contents.Value)
	}
}
