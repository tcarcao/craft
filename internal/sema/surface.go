package sema

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// SurfaceSymbol is a surface declaration, resolved in AnalyzeWorkspace.
type SurfaceSymbol struct {
	Subject      string
	Line, Column int          // 1-based; Column in UTF-16 units, of the subject
	Callers      []UseCaseRef // `unknown` excluded
	URI          string
}

// collectSurfaces returns the file's surfaces as symbols, in source order.
func collectSurfaces(uri string, file syntax.File, li green.LineIndex, hasLI bool) []SurfaceSymbol {
	var out []SurfaceSymbol
	for _, s := range file.Surfaces() {
		tok := s.SubjectToken()
		if tok == nil {
			continue
		}
		sym := SurfaceSymbol{Subject: s.Subject(), URI: uri}
		if hasLI {
			sym.Line, sym.Column = li.LineCol16(tok.Offset())
		}
		for i, r := range s.CallerRefs() {
			name := s.Callers()[i]
			if name == lang.UnknownCaller {
				continue
			}
			ref := UseCaseRef{Name: name}
			if hasLI {
				ref.Line, ref.Column = r.Line(li), r.Col(li)
			}
			sym.Callers = append(sym.Callers, ref)
		}
		out = append(out, sym)
	}
	return out
}

// CollapseOperation normalises an operation text for comparison: runs of
// whitespace become one space.
func CollapseOperation(text string) string { return strings.Join(strings.Fields(text), " ") }

// validateSurfaces reports an operation listed twice in one surface.
func validateSurfaces(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, s := range file.Surfaces() {
		seen := map[string]bool{}
		for _, op := range s.Operations() {
			text := CollapseOperation(op.OpText())
			if text == "" {
				continue
			}
			if !seen[text] {
				seen[text] = true
				continue
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/duplicate-operation",
				Message:   fmt.Sprintf("operation `[%s]` is already listed in this surface", text),
				Severity:  model.SeverityError,
				SourceURI: uri,
				Range:     annotationRange(op.OpAnnotation(), li, hasLI),
			})
		}
	}
	return diags
}

// annotationRange is the LSP range of an `[op]` bracket, first `[` to last `]`.
func annotationRange(op *syntax.SyntaxNode, li green.LineIndex, hasLI bool) model.Range {
	if op == nil || !hasLI {
		return model.Range{}
	}
	toks := op.Tokens() // without the whitespace before `[`
	if len(toks) == 0 {
		return model.Range{}
	}
	first, last := toks[0], toks[len(toks)-1]
	sl, sc := li.LineCol16(first.Offset())
	el, ec := li.LineCol16(last.Offset())
	return model.Range{
		Start: model.Position{Line: lineToLSP(sl), Character: colToLSP(sc)},
		End:   model.Position{Line: lineToLSP(el), Character: colToLSP(ec) + lspLen(last.Text())},
	}
}

// resolveSurfaces checks every surface's subject and callers against the
// workspace, and reports a second surface for the same boundary. Files are
// walked in sorted order, so the first surface in that order is the one the
// model keeps. Unlike use-case references, these checks run even when the
// workspace declares nothing: a surface always names a declared boundary.
func resolveSurfaces(perFile map[string]Symbols, ws WorkspaceSymbols) []model.Diagnostic {
	var diags []model.Diagnostic
	first := map[string]SurfaceSymbol{}
	for _, uri := range slices.Sorted(maps.Keys(perFile)) {
		for _, s := range perFile[uri].Surfaces {
			subject := symbolRange(s.Line, s.Column, s.Subject)
			p, ambiguous := ResolveParticipant(ws, "", s.Subject)
			switch {
			case ambiguous:
				// ambiguous-bc reports it.
			case p.Kind == "":
				diags = append(diags, model.Diagnostic{
					Code:      "craft/sema/unresolved-reference",
					Message:   fmt.Sprintf("surface subject %q is not declared as a bounded context, service or actor", s.Subject),
					Severity:  model.SeverityError,
					SourceURI: uri,
					Range:     subject,
				})
			case p.Kind == lang.ParticipantDomain || p.Kind == lang.ParticipantDataStore:
				diags = append(diags, model.Diagnostic{
					Code:      "craft/sema/invalid-surface-subject",
					Message:   fmt.Sprintf("a surface belongs to a bounded context, a service or an actor (an external system); %q is a %s", s.Subject, kindPhrase(p.Kind)),
					Severity:  model.SeverityError,
					SourceURI: uri,
					Range:     subject,
				})
			default:
				if prev, dup := first[p.ID]; dup {
					sev := model.SeverityWarning
					if prev.URI == uri {
						sev = model.SeverityError
					}
					diags = append(diags, model.Diagnostic{
						Code:      "craft/sema/duplicate-name",
						Message:   fmt.Sprintf("surface %q is already declared at %s:%d; the first one is used", s.Subject, prev.URI, prev.Line),
						Severity:  sev,
						SourceURI: uri,
						Range:     subject,
					})
				} else {
					first[p.ID] = s
				}
			}
			for _, c := range s.Callers {
				if cp, amb := ResolveParticipant(ws, "", c.Name); amb || cp.Kind != "" {
					continue
				}
				diags = append(diags, model.Diagnostic{
					Code:      "craft/sema/unresolved-reference",
					Message:   fmt.Sprintf("surface caller %q is not declared; write `unknown` when no caller is observed", c.Name),
					Severity:  model.SeverityError,
					SourceURI: uri,
					Range:     symbolRange(c.Line, c.Column, c.Name),
				})
			}
		}
	}
	return diags
}

// symbolRange is the one-line LSP range of name at a 1-based line and column.
func symbolRange(line, col int, name string) model.Range {
	start := colToLSP(col)
	return model.Range{
		Start: model.Position{Line: lineToLSP(line), Character: start},
		End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(name)},
	}
}

// kindPhrase names a participant kind in prose.
func kindPhrase(k lang.ParticipantKind) string {
	if k == lang.ParticipantDataStore {
		return "data store"
	}
	return string(k)
}

// ── operation-not-in-surface ─────────────────────────────────────────────────
// Warning: an `asks <target> … [op]` whose target has a surface that does not
// list op. A context without a surface of its own is covered by the surface of
// the service that deploys it (model.SurfaceBoundary). It is the op-name check of spec §8.2, done on the tree because the
// range is the `[op]` bracket, which interactions do not carry.

func lintOperationNotInSurface(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	type surfaceOps struct {
		subject string
		ops     map[string]bool
	}
	uris := sortedTreeURIs(perFileTrees)
	surfaces := map[string]surfaceOps{} // resolved boundary ID → its first surface
	for _, uri := range uris {
		for _, s := range syntax.AsFile(perFileTrees[uri]).Surfaces() {
			p, ambiguous := ResolveParticipant(ws, "", s.Subject())
			if ambiguous || p.Kind == "" {
				continue
			}
			if _, seen := surfaces[p.ID]; seen {
				continue
			}
			ops := map[string]bool{}
			for _, op := range s.Operations() {
				if text := CollapseOperation(op.OpText()); text != "" {
					ops[text] = true
				}
			}
			surfaces[p.ID] = surfaceOps{subject: s.Subject(), ops: ops}
		}
	}
	if len(surfaces) == 0 {
		return nil
	}

	var diags []model.Diagnostic
	for _, uri := range uris {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					text := CollapseOperation(a.OpText())
					if a.Kind() != lang.ActionSync || text == "" {
						continue
					}
					p, ambiguous := ResolveParticipant(ws, "", a.TargetName())
					if ambiguous || p.Kind == "" {
						continue
					}
					boundary, ok := model.SurfaceBoundary(p, func(id string) bool { _, has := surfaces[id]; return has })
					if !ok {
						continue
					}
					s := surfaces[boundary]
					if s.ops[text] {
						continue
					}
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/operation-not-in-surface",
						Message:   fmt.Sprintf("`[%s]` is not an operation of the `%s` surface; add it to the surface or fix the operation name", text, s.subject),
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range:     annotationRange(a.OpAnnotation(), li, true),
					})
				}
			}
		}
	}
	return diags
}
