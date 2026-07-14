package craft

import (
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
)

// Parse parses a single Craft source file and returns its document model plus
// any diagnostics (syntax errors/warnings and single-file semantic checks).
//
// Diagnostics are returned as data, never via the error return: err is reserved
// for programmer-error/invariant conditions and is currently always nil. Parse
// performs no file I/O — the caller supplies src. The returned *CraftDoc is
// never nil, even for empty or badly broken input.
//
// SourceURI on the returned diagnostics is left as produced by the underlying
// passes (empty for single-file input); the caller already knows filename. Use
// ParseFiles when you need every diagnostic attributed to its file.
func Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error) {
	doc, diags := parseOne(filename, string(src))
	return doc, diags, nil
}

// parseOne runs the single-file pipeline exactly as `craft check` does:
// syntax.Parse -> Root -> ProjectFromTree, then sema.AnalyzeFile for
// single-file semantic diagnostics. LineIndex is intentionally NOT forwarded to
// AnalyzeFile, matching the current CLI (see plan D3).
func parseOne(filename, src string) (*CraftDoc, []Diagnostic) {
	uri := "file://" + filename
	greenRoot, li, parseDiags := syntax.Parse(src)
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	_, semaDiags := sema.AnalyzeFile(uri, tree)

	diags := make([]Diagnostic, 0, len(parseDiags)+len(semaDiags))
	diags = append(diags, parseDiags...)
	diags = append(diags, semaDiags...)
	return doc, diags
}
