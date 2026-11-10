package sema

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// FlowSymbol is a critical-flow declaration (v2.35).
type FlowSymbol struct {
	Ref          string
	Name         string
	Line, Column int // 1-based; Column in UTF-16 units, of the ref
	URI          string
}

// collectFlows returns the file's flows as symbols, in source order. A flow
// without a ref is not one.
func collectFlows(uri string, file syntax.File, li green.LineIndex, hasLI bool) []FlowSymbol {
	var out []FlowSymbol
	for _, f := range file.Flows() {
		tok := f.RefToken()
		if tok == nil {
			continue
		}
		sym := FlowSymbol{Ref: f.Ref(), Name: f.Name(), URI: uri}
		if hasLI {
			sym.Line, sym.Column = li.LineCol16(tok.Offset())
		}
		out = append(out, sym)
	}
	return out
}

// tokenDiagRange is the one-line LSP range of tok.
func tokenDiagRange(tok *syntax.SyntaxToken, li green.LineIndex, hasLI bool) model.Range {
	if tok == nil || !hasLI {
		return model.Range{}
	}
	line, col := li.LineCol16(tok.Offset())
	return symbolRange(line, col, tok.Text())
}

// validateFlows checks each flow of one file: `priority:` and `owner:` are
// required (craft/sema/missing-flow-property), the priority is one of
// P0–P3 (craft/sema/invalid-priority), and a property other than `alias:` is
// written once (craft/sema/duplicate-flow-property; the last value wins).
// `alias:` may repeat, one per alias (see checkAliases). A flow without a ref
// is the parser's error and is not checked.
func validateFlows(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(code string, sev model.Severity, r model.Range, msg string) {
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uri, Range: r})
	}
	for _, f := range file.Flows() {
		refTok := f.RefToken()
		if refTok == nil {
			continue
		}
		seen := map[string]bool{}
		for _, p := range f.Properties() {
			if !p.Known() {
				continue // craft/syntax/unknown-flow-property
			}
			key := p.Key()
			if seen[key] && key != lang.FlowAlias {
				add("craft/sema/duplicate-flow-property", model.SeverityWarning, tokenDiagRange(p.KeyToken(), li, hasLI),
					fmt.Sprintf("flow %q: `%s:` is already set; the last value wins", f.Ref(), key))
			}
			seen[key] = true
			if key == lang.FlowPriority && p.ValueToken() != nil && !slices.Contains(lang.FlowPriorities(), p.Value()) {
				add("craft/sema/invalid-priority", model.SeverityError, tokenDiagRange(p.ValueToken(), li, hasLI),
					fmt.Sprintf("flow priority %q is not one of %s; P0 is the highest", p.Value(), strings.Join(lang.FlowPriorities(), ", ")))
			}
		}
		diags = append(diags, checkAliases(uri, "flow", f.Ref(), f.Properties(), li, hasLI)...)
		for _, key := range []string{lang.FlowPriority, lang.FlowOwner} {
			if seen[key] {
				continue
			}
			example := "priority: P1"
			if key == lang.FlowOwner {
				example = "owner: seeker-engagement"
			}
			add("craft/sema/missing-flow-property", model.SeverityError, tokenDiagRange(refTok, li, hasLI),
				fmt.Sprintf("flow %q has no `%s:`; every flow has a priority and an owner, e.g. `%s`", f.Ref(), key, example))
		}
	}
	return diags
}

// mergeFlows indexes the flows of every file by ref (uris in sorted order,
// the first declaration wins) and reports every later declaration of a ref,
// in any file, as craft/sema/duplicate-flow. An error, not a warning as for a
// service declared in two files: a flow is one business outcome, so two
// definitions disagree on its priority or owner.
func mergeFlows(ws *WorkspaceSymbols, perFile map[string]Symbols, uris []string) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range uris {
		for _, f := range perFile[uri].Flows {
			prev, dup := ws.Flows[f.Ref]
			if !dup {
				ws.Flows[f.Ref] = f
				continue
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/duplicate-flow",
				Message:   fmt.Sprintf("flow %q is already declared at %s:%d; a flow ref is declared once in the workspace", f.Ref, prev.URI, prev.Line),
				Severity:  model.SeverityError,
				SourceURI: f.URI,
				Range:     symbolRange(f.Line, f.Column, f.Ref),
			})
		}
	}
	return diags
}

// ── realises ──────────────────────────────────────────────────────────────────
// unresolved-flow (warning: the flow may live in a file that was not
// loaded) and duplicate-flow-in-list (warning), on each value of a use
// case's `realises` tag. Ranges are the values, read from the tree.

func lintRealises(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range doc.UseCases {
		if len(uc.TagValues[lang.TagRealises]) == 0 {
			continue
		}
		values, ranges := tagValuesWithRanges(uc, lang.TagRealises, perFileTrees, lis)
		if values == nil {
			// No tree to read positions from: report at the zero range.
			values = uc.TagValues[lang.TagRealises]
			ranges = make([]model.Range, len(values))
		}
		seen := map[string]bool{}
		for i, v := range values {
			ref := strings.TrimSpace(v)
			if ref == "" {
				continue
			}
			add := func(code, msg string) {
				diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: model.SeverityWarning,
					SourceURI: uc.SourceURI, Range: ranges[i]})
			}
			if seen[ref] {
				add("craft/lint/duplicate-flow-in-list", fmt.Sprintf("flow %q is already in this list; drop the repeat", ref))
				continue
			}
			seen[ref] = true
			if doc.FlowByRef(ref) == nil {
				add("craft/sema/unresolved-flow",
					fmt.Sprintf("no flow is declared as %q in the loaded files; check the ref, or load the file that declares `flow %s`", ref, ref))
			}
		}
	}
	return diags
}

// tagValuesWithRanges returns the values of uc's tag key and the range of
// each, read from the tag the model keeps (the last one with that key in the
// use case's first tags block) in its file's tree; the use case is found by
// its title line. Both are nil when the tree or the tag is not there.
func tagValuesWithRanges(uc model.UseCase, key string, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) ([]string, []model.Range) {
	root, ok := perFileTrees[uc.SourceURI]
	li, hasLI := lis[uc.SourceURI]
	if !ok || !hasLI {
		return nil, nil
	}
	var values []string
	var ranges []model.Range
	for _, decl := range syntax.AsFile(root).UseCases() {
		if decl.Line(li) != uc.Line {
			continue
		}
		if tb := decl.TagsBlock(); tb != nil {
			for _, ts := range tb.Tags() {
				if k := ts.Key(); k == nil || k.Text() != key {
					continue
				}
				values, ranges = ts.Values(), nil
				for _, r := range ts.ValueRanges() {
					sl, sc := li.LineCol16(r.Start)
					el, ec := li.LineCol16(r.End)
					ranges = append(ranges, model.Range{
						Start: model.Position{Line: lineToLSP(sl), Character: colToLSP(sc)},
						End:   model.Position{Line: lineToLSP(el), Character: colToLSP(ec)},
					})
				}
			}
		}
		break
	}
	if len(values) != len(ranges) {
		return nil, nil
	}
	return values, ranges
}
