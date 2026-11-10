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

// PageSymbol is a page declaration (v2.36).
type PageSymbol struct {
	Ref          string
	Name         string
	Line, Column int // 1-based; Column in UTF-16 units, of the ref
	URI          string
}

// collectPages returns the file's pages as symbols, in source order. A page
// without a ref is not one.
func collectPages(uri string, file syntax.File, li green.LineIndex, hasLI bool) []PageSymbol {
	var out []PageSymbol
	for _, pg := range file.Pages() {
		tok := pg.RefToken()
		if tok == nil {
			continue
		}
		sym := PageSymbol{Ref: pg.Ref(), Name: pg.Name(), URI: uri}
		if hasLI {
			sym.Line, sym.Column = li.LineCol16(tok.Offset())
		}
		out = append(out, sym)
	}
	return out
}

// validatePages checks each page of one file: `product_domain:` is required
// (craft/sema/missing-page-property), the platform is one of frontoffice,
// backoffice or app (craft/sema/invalid-platform), and a property other
// than `url:` and `alias:` is written once (craft/sema/duplicate-page-property;
// the last value wins). `url:` may repeat, one per site; `alias:` may repeat,
// one per alias (see checkAliases). A page without a ref is the parser's
// error and is not checked.
func validatePages(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(code string, sev model.Severity, r model.Range, msg string) {
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uri, Range: r})
	}
	for _, pg := range file.Pages() {
		refTok := pg.RefToken()
		if refTok == nil {
			continue
		}
		seen := map[string]bool{}
		for _, p := range pg.Properties() {
			if !p.Known() {
				continue // craft/syntax/unknown-page-property
			}
			key := p.Key()
			if seen[key] && key != lang.PageURL && key != lang.PageAlias {
				add("craft/sema/duplicate-page-property", model.SeverityWarning, tokenDiagRange(p.KeyToken(), li, hasLI),
					fmt.Sprintf("page %q: `%s:` is already set; the last value wins", pg.Ref(), key))
			}
			seen[key] = true
			if key == lang.PagePlatform && p.ValueToken() != nil && !slices.Contains(lang.PagePlatforms(), p.Value()) {
				add("craft/sema/invalid-platform", model.SeverityError, tokenDiagRange(p.ValueToken(), li, hasLI),
					fmt.Sprintf("page platform %q is not one of %s", p.Value(), strings.Join(lang.PagePlatforms(), ", ")))
			}
		}
		diags = append(diags, checkAliases(uri, "page", pg.Ref(), pg.Properties(), li, hasLI)...)
		if !seen[lang.PageProductDomain] {
			add("craft/sema/missing-page-property", model.SeverityError, tokenDiagRange(refTok, li, hasLI),
				fmt.Sprintf("page %q has no `product_domain:`; every page names the product domain that owns it, e.g. `product_domain: ad-inventory`", pg.Ref()))
		}
	}
	return diags
}

// checkAliases checks the `alias:` lines of one page or flow (v2.38): a value
// that is empty or blank is craft/sema/empty-alias, and a value already
// written on an earlier line of the same block (compared trimmed) is
// craft/sema/duplicate-alias. Both are warnings; the model leaves the value
// out, or keeps it once. kind is "page" or "flow".
func checkAliases(uri, kind, ref string, props []syntax.FlowPropertyDecl, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	seen := map[string]bool{}
	for _, p := range props {
		if p.Key() != lang.PageAlias || !p.Known() || p.ValueToken() == nil {
			continue // lang.PageAlias == lang.FlowAlias; a missing value is the parser's error
		}
		v := strings.TrimSpace(p.Value())
		switch {
		case v == "":
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/empty-alias",
				Message:   fmt.Sprintf("%s %q: an alias is empty; write another name for the %s, e.g. `alias: \"promoted ad\"`, or remove the line", kind, ref, kind),
				Severity:  model.SeverityWarning,
				SourceURI: uri,
				Range:     tokenDiagRange(p.ValueToken(), li, hasLI),
			})
		case seen[v]:
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/duplicate-alias",
				Message:   fmt.Sprintf("%s %q: alias %q is already listed; remove this line", kind, ref, v),
				Severity:  model.SeverityWarning,
				SourceURI: uri,
				Range:     tokenDiagRange(p.ValueToken(), li, hasLI),
			})
		}
		seen[v] = true
	}
	return diags
}

// mergePages indexes the pages of every file by ref (uris in sorted order,
// the first declaration wins) and reports every later declaration of a ref,
// in any file, as craft/sema/duplicate-page (an error, as for flows).
func mergePages(ws *WorkspaceSymbols, perFile map[string]Symbols, uris []string) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range uris {
		for _, pg := range perFile[uri].Pages {
			prev, dup := ws.Pages[pg.Ref]
			if !dup {
				ws.Pages[pg.Ref] = pg
				continue
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/duplicate-page",
				Message:   fmt.Sprintf("page %q is already declared at %s:%d; a page ref is declared once in the workspace", pg.Ref, prev.URI, prev.Line),
				Severity:  model.SeverityError,
				SourceURI: pg.URI,
				Range:     symbolRange(pg.Line, pg.Column, pg.Ref),
			})
		}
	}
	return diags
}

// ── entry ─────────────────────────────────────────────────────────────────────
// unresolved-page (warning), on each value of a use case's `entry` tag that
// names no page. Only when the workspace declares at least one page: a model
// that has not started declaring pages uses `entry` as free text.

func lintEntries(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	if len(doc.Pages) == 0 {
		return nil
	}
	var diags []model.Diagnostic
	for _, uc := range doc.UseCases {
		if len(uc.TagValues[lang.TagEntry]) == 0 {
			continue
		}
		values, ranges := tagValuesWithRanges(uc, lang.TagEntry, perFileTrees, lis)
		if values == nil {
			values = uc.TagValues[lang.TagEntry]
			ranges = make([]model.Range, len(values))
		}
		seen := map[string]bool{}
		for i, v := range values {
			ref := strings.TrimSpace(v)
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			if doc.PageByRef(ref) == nil {
				diags = append(diags, model.Diagnostic{
					Code:      "craft/sema/unresolved-page",
					Message:   fmt.Sprintf("no page is declared as %q in the loaded files; check the ref, or load the file that declares `page %s`", ref, ref),
					Severity:  model.SeverityWarning,
					SourceURI: uc.SourceURI,
					Range:     ranges[i],
				})
			}
		}
	}
	return diags
}
