package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const notesSrc = `use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session"
        alt "the card is declined"
            finding "no idempotency key"
}
`

func TestNotes_StepsAndUseCaseLevel(t *testing.T) {
	g, _, diags := syntax.Parse(notesSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	uc := syntax.AsFile(syntax.Root(g)).UseCases()[0]
	ns := uc.Notes()
	if len(ns) != 1 || ns[0].Kind() != lang.NoteFinding || ns[0].Text() != "nothing retries a failed callback" {
		t.Fatalf("use-case notes = %+v", ns)
	}
	sc := uc.Scenarios()[0]
	want := "action frontendplatform\nnote note the gateway forwards the seller's session\nalt\n  branch the card is declined\n    note finding no idempotency key\n"
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	if n := len(sc.Actions()); n != 1 {
		t.Errorf("Actions() = %d, want 1 (notes are not actions)", n)
	}
}

func TestNotes_KeywordsStayNames(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    note asks finding to run\n    finding records something\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := outline(sc.Steps(), 0); got != "action note\naction finding\n" {
		t.Errorf("tree:\n%s", got)
	}
}

func TestNotes_Diagnostics(t *testing.T) {
	cases := map[string]string{
		"craft/syntax/missing-label":    "use_case \"U\" {\n  when O does\n    x does it\n    finding\n}",
		"craft/syntax/unexpected-token": "use_case \"U\" {\n  when O does\n    x does it\n    note \"a\" twice\n}",
	}
	for code, src := range cases {
		_, _, diags := syntax.Parse(src)
		found := false
		for _, d := range diags {
			found = found || d.Code == code
		}
		if !found {
			t.Errorf("%s: not reported; got %v", code, diags)
		}
	}
}
