package sema

import (
	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// validateSources reports //@ lines that do not parse (malformed-source) and
// well-formed ones outside every use-case body (detached-source), spec §14.3.
// The range is the whole comment.
func validateSources(uri string, root syntax.SyntaxNode, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	toks := syntax.SourceComments(root)
	if len(toks) == 0 {
		return nil
	}
	var bodies []green.TextRange
	for _, uc := range file.UseCases() {
		if s, e, ok := uc.BodySpan(); ok {
			bodies = append(bodies, green.TextRange{Start: s, End: e})
		}
	}
	inBody := func(off green.TextSize) bool {
		for _, b := range bodies {
			if off >= b.Start && off < b.End {
				return true
			}
		}
		return false
	}
	var diags []model.Diagnostic
	for _, tok := range toks {
		var r model.Range
		if hasLI {
			line, col := li.LineCol16(tok.Offset())
			r = model.Range{
				Start: model.Position{Line: lineToLSP(line), Character: colToLSP(col)},
				End:   model.Position{Line: lineToLSP(line), Character: colToLSP(col) + lspLen(tok.Text())},
			}
		}
		if _, _, ok := lang.ParseSourceComment(tok.Text()); !ok {
			diags = append(diags, model.Diagnostic{Code: "craft/syntax/malformed-source", Severity: model.SeverityWarning, SourceURI: uri, Range: r,
				Message: "this `//@` line is not a source cite (`//@ [repo] path[:lines] [-- text]`, with a range's end not before its start), so it stays a plain comment"})
			continue
		}
		if !inBody(tok.Offset()) {
			diags = append(diags, model.Diagnostic{Code: "craft/lint/detached-source", Severity: model.SeverityWarning, SourceURI: uri, Range: r,
				Message: "a `//@` cite proves the step above it; move it under that step, or make it a plain `//` comment"})
		}
	}
	return diags
}
