package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// A `when` block that ends up outside any use_case (the usual cause being one
// stray `}` earlier in the file) is discarded in full by top-level recovery:
// no scenario, no actions, no participants reach the model. That is content
// loss, not a tool limitation, so it must be reported at error severity under
// its own code.
//
// It used to be a `craft/syntax/not-yet-implemented` warning, which the
// codebook defines as "recognised but not yet supported by parser v2" (the
// tool is behind, the file is fine). `craft validate` therefore exited 0 on a
// file whose content was being thrown away.
const topLevelWhenSrc = `use_case "A real use case" {
    when Seeker opens the page
        frontend asks graphql for the page  [getPage]
}

when Seeker opens the payments page
    graphql asks billing for the invoices  [getInvoices]
    billing answers with the archived rows
`

// findDiag returns the first diagnostic with the given code.
func findDiag(diags []model.Diagnostic, code string) (model.Diagnostic, bool) {
	for _, d := range diags {
		if d.Code == code {
			return d, true
		}
	}
	return model.Diagnostic{}, false
}

func TestSkippedConstruct_TopLevelWhenIsAnError(t *testing.T) {
	_, _, diags := syntax.Parse(topLevelWhenSrc)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}
	if d.Severity != model.SeverityError {
		t.Errorf("severity = %q, want %q: the block is discarded, so validate must fail without --strict",
			d.Severity, model.SeverityError)
	}
	if _, stale := findDiag(diags, "craft/syntax/not-yet-implemented"); stale {
		t.Error("a discarded region must not be reported as not-yet-implemented; that code means the grammar is fine and the parser is behind")
	}
}

// The diagnostic used to point at the four characters of `when` while recovery
// silently ate everything up to the next top-level keyword. A reader was told
// one token was unexpected; they were not told a block had been deleted.
func TestSkippedConstruct_RangeCoversEverythingDiscarded(t *testing.T) {
	_, _, diags := syntax.Parse(topLevelWhenSrc)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}

	// 0-based: `when` is on line 6, the last discarded token (`rows`) on line 8.
	if d.Range.Start.Line != 5 || d.Range.Start.Character != 0 {
		t.Errorf("Range.Start = %+v, want line 5 character 0 (the `when`)", d.Range.Start)
	}
	if d.Range.End.Line != 7 {
		t.Errorf("Range.End.Line = %d, want 7 (the last discarded line), not the end of the `when` token",
			d.Range.End.Line)
	}
	if d.Range.End.Character != 42 {
		t.Errorf("Range.End.Character = %d, want 42 (one past `rows`)", d.Range.End.Character)
	}

	// And the prose has to say it, because the range alone is easy to skim past.
	if !strings.Contains(d.Message, "lines 6-8") {
		t.Errorf("message must name the skipped lines, got %q", d.Message)
	}
	if !strings.Contains(d.Message, "not part of the model") {
		t.Errorf("message must say the region is absent from the model, got %q", d.Message)
	}
}

// A one-line discard reads as a single line, not "lines 3-3".
func TestSkippedConstruct_SingleLineWording(t *testing.T) {
	src := "actor user Bob\n?\nactor user Carol\n"

	_, _, diags := syntax.Parse(src)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}
	if !strings.Contains(d.Message, "line 2 was skipped") {
		t.Errorf("message = %q, want singular wording for a one-line discard", d.Message)
	}
}

// Nobody writes a top-level `when` on purpose. What happens is an extra `}`,
// and the diagnostic pointed at the `when` (the symptom) rather than at the
// `}` above it (the cause). When a use_case closed immediately before the
// unplaceable `when`, name it.
func TestSkippedConstruct_NamesTheStrayBrace(t *testing.T) {
	src := `use_case "Settle a seller invoice" {
    when Seller opens the payments pages
        graphql asks atlas/billing for invoice details  [getInvoiceDetails]
}

    when Seller opens the payments pages
        graphql consults the isInvoiceArchiveEnabled flag
}
`

	_, _, diags := syntax.Parse(src)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}
	if !strings.Contains(d.Message, "Settle a seller invoice") {
		t.Errorf("message must name the use_case that closed early, got %q", d.Message)
	}
	if !strings.Contains(d.Message, "line 4") {
		t.Errorf("message must point at the `}` on line 4, got %q", d.Message)
	}
}

// The brace hint is only earned when a use_case actually closed. With no
// use_case in the file there is no `}` to blame, and inventing a line number
// would send the reader somewhere arbitrary.
func TestSkippedConstruct_NoBraceHintWhenNoUseCaseClosed(t *testing.T) {
	src := `when Seeker opens the payments page
    graphql asks billing for the invoices
`

	_, _, diags := syntax.Parse(src)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}
	if strings.Contains(d.Message, "may be extra") {
		t.Errorf("no use_case closed, so no `}` may be blamed; got %q", d.Message)
	}
}

// The hint is also gated on adjacency. Once another top-level construct has
// been parsed, the last `}` a use_case closed with is no longer a brace the
// reader has any reason to suspect.
func TestSkippedConstruct_NoBraceHintAcrossAnotherConstruct(t *testing.T) {
	src := `use_case "A" {
    when Seeker opens the page
        frontend asks graphql for the page
}

actor user Seeker

when Seeker opens the payments page
    graphql asks billing for the invoices
`

	_, _, diags := syntax.Parse(src)

	d, ok := findDiag(diags, "craft/syntax/skipped-construct")
	if !ok {
		t.Fatalf("no craft/syntax/skipped-construct diagnostic; got %+v", diags)
	}
	if strings.Contains(d.Message, "may be extra") {
		t.Errorf("a use_case two constructs back must not be blamed; got %q", d.Message)
	}
}

// The counterpart the split preserves: a construct the grammar recognises but
// parser v2 cannot place stays a `not-yet-implemented` warning. A phrase brace
// leaves the trailing `[operation]` annotation unplaceable, and that is the
// parser being behind, not the file being wrong.
func TestSkippedConstruct_PhraseBraceStaysNotYetImplemented(t *testing.T) {
	src := `use_case "X" {
  when U does x
    Billing asks Gateway to charge {amount} [POST /pay]
}`

	_, _, diags := syntax.Parse(src)

	d, ok := findDiag(diags, "craft/syntax/not-yet-implemented")
	if !ok {
		t.Fatalf("no craft/syntax/not-yet-implemented diagnostic; got %+v", diags)
	}
	if d.Severity != model.SeverityWarning {
		t.Errorf("severity = %q, want %q", d.Severity, model.SeverityWarning)
	}
	if !strings.Contains(d.Message, "braces") {
		t.Errorf("message should keep its specific advice, got %q", d.Message)
	}
}
