package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

const displayNameDoc = `actors {
    user Seller "the seller"
}
domains {
    billing {
        nmf_monetization "nMF, the money engine"
        portfoliovalue
    }
}
use_case "U" {
    when Seller opens the price page
        nmf_monetization answers with the prices
        portfoliovalue computes the value
}
`

func hoverAt(t *testing.T, s *Server, doc protocol.TextDocumentIdentifier, line, char uint32) string {
	t.Helper()
	h, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: doc, Position: protocol.Position{Line: line, Character: char}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		return ""
	}
	return h.Contents.Value
}

func TestDisplayNames_Hover(t *testing.T) {
	s, doc := openTwin(t, displayNameDoc)
	cases := []struct {
		name       string
		line, char uint32
		want       string
	}{
		{"context reference", 11, 10, "bounded context: nmf_monetization (domain: billing)\ndisplay name: nMF, the money engine"},
		{"actor reference", 10, 10, "actor: Seller (user)\ndisplay name: the seller"},
		{"actor declaration", 1, 10, "actor: Seller (user)\ndisplay name: the seller"},
		{"context declaration", 5, 12, "bounded context: nmf_monetization (domain: billing)\ndisplay name: nMF, the money engine"},
		{"context without a display name", 12, 10, "bounded context: portfoliovalue (domain: billing)"},
	}
	for _, c := range cases {
		if got := hoverAt(t, s, doc, c.line, c.char); got != c.want {
			t.Errorf("%s: hover = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDisplayNames_CompletionDetail(t *testing.T) {
	s, _ := openTwin(t, displayNameDoc)
	details := map[string]string{}
	for _, it := range allSymbolCompletions(s.ws).Items {
		details[it.Label] = it.Detail
	}
	for _, it := range bcSymbolCompletions(s.ws).Items {
		if it.Label == "nmf_monetization" && it.Detail != details["nmf_monetization"] {
			t.Errorf("bc completion detail %q differs from %q", it.Detail, details["nmf_monetization"])
		}
	}
	want := map[string]string{
		"nmf_monetization": "bounded context (domain: billing): nMF, the money engine",
		"portfoliovalue":   "bounded context (domain: billing)",
		"Seller":           "actor (user): the seller",
	}
	for label, w := range want {
		if details[label] != w {
			t.Errorf("%s detail = %q, want %q", label, details[label], w)
		}
	}
}

func TestDisplayNames_Extraction(t *testing.T) {
	s, _ := openTwin(t, displayNameDoc)
	res := s.craftExtractWorkspace(nil).(craftExtractionResult)
	got := map[string]string{}
	for _, d := range res.Domains {
		for _, bc := range d.BoundedContexts {
			got[bc.Name] = bc.DisplayName
		}
	}
	for _, a := range res.Actors {
		got[a.Name] = a.DisplayName
	}
	if got["nmf_monetization"] != "nMF, the money engine" || got["portfoliovalue"] != "" || got["Seller"] != "the seller" {
		t.Errorf("extraction display names = %v", got)
	}

	g, _, _ := syntax.Parse(displayNameDoc)
	dsl := reconstructDomainFromView(syntax.AsFile(syntax.Root(g)).Domains()[0])
	if !strings.Contains(dsl, "\n    nmf_monetization \"nMF, the money engine\"\n") || !strings.Contains(dsl, "\n    portfoliovalue\n") {
		t.Errorf("reconstructed domain lost its display names:\n%s", dsl)
	}
}

// The canonical form keeps the string on the name's line, one space after it.
func TestDisplayNames_Format(t *testing.T) {
	in := "domains {\n  b {\n    x     \"X one\"   // why\n    y // y\n  }\n}\nactors {\n  user U    \"u\"\n}\nactor system   S \"s\"\n"
	want := "domains {\n    b {\n        x \"X one\" // why\n        y         // y\n    }\n}\n\nactors {\n    user U \"u\"\n}\n\nactor system S \"s\"\n"
	got := FormatDocument(in)
	if got != want {
		t.Errorf("format:\n%s\nwant:\n%s", got, want)
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
}
