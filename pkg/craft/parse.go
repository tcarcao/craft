package craft

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// Parse parses a single Craft source file and returns its document model plus
// any diagnostics (syntax errors/warnings and single-file semantic checks).
//
// Diagnostics are returned as data, never via the error return: err is reserved
// for programmer-error/invariant conditions and is currently always nil. Parse
// performs no file I/O — the caller supplies src. The returned *CraftDoc is
// never nil, even for empty or badly broken input.
//
// Every returned diagnostic's SourceURI is set to filename (the exact string you
// pass). This gives single-file callers the same uniform attribution ParseFiles
// provides — no empty and no file://-prefixed values leak through, even though
// some underlying sema passes label their diagnostics with an internal file://
// URI.
func Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error) {
	doc, diags := parseOne(filename, string(src))
	return doc, diags, nil
}

// parseOne runs the single-file pipeline exactly as `craft check` does:
// syntax.Parse -> Root -> ProjectFromTree, then sema.AnalyzeFile for
// single-file semantic diagnostics. Every diagnostic's SourceURI is normalized
// to filename (the sema passes set an internal file:// URI on some of them;
// stampURI overwrites it uniformly).
//
// The LineIndex IS forwarded to AnalyzeFile. It deliberately was not during the
// LSP migration (plan D3: keep CLI diagnostic bytes identical while the parser
// was swapped), but that left every sema diagnostic from this path pinned to
// line 0 / column 0 — `craft validate` reported them all on line 1 regardless of
// where the offending construct was. The LSP never had the problem because
// internal/workspace passes its per-file index through.
func parseOne(filename, src string) (*CraftDoc, []Diagnostic) {
	uri := "file://" + filename
	greenRoot, li, parseDiags := syntax.Parse(src)
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li, filename)

	_, semaDiags := sema.AnalyzeFile(uri, tree, li)

	diags := make([]Diagnostic, 0, len(parseDiags)+len(semaDiags))
	diags = append(diags, parseDiags...)
	diags = append(diags, semaDiags...)
	return doc, stampURI(diags, filename)
}

// ParseFiles parses and merges multiple Craft source files into one document
// model, mirroring `craft inspect` (merge) plus `craft validate` (full
// workspace analysis: cross-file resolution and lint). Files are processed in
// ascending map-key order, so the merged model and the diagnostic slice are
// deterministic regardless of Go map iteration order.
//
// Every returned diagnostic's SourceURI is set to the originating map key (the
// bare filename you supplied — no file:// prefix). Diagnostics are data, not
// errors; err is currently always nil. Performs no file I/O.
func ParseFiles(files map[string][]byte) (*CraftDoc, []Diagnostic, error) {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	merged := &CraftDoc{}
	diags := []Diagnostic{}
	perFileTrees := make(map[string]syntax.SyntaxNode, len(files))
	perFileSyms := make(map[string]sema.Symbols, len(files))
	perFileLIs := make(map[string]green.LineIndex, len(files))

	for _, key := range keys {
		uri := "file://" + key
		greenRoot, li, parseDiags := syntax.Parse(string(files[key]))
		tree := syntax.Root(greenRoot)
		perFileTrees[uri] = tree
		perFileLIs[uri] = li

		doc := syntax.ProjectFromTree(tree, li, key)
		mergeDoc(merged, doc)

		diags = append(diags, stampURI(parseDiags, key)...)

		syms, semaDiags := sema.AnalyzeFile(uri, tree, li)
		perFileSyms[uri] = syms
		diags = append(diags, stampURI(semaDiags, key)...)
	}

	if len(perFileSyms) > 0 {
		ws, wsDiags := sema.MergeWorkspaceSymbols(perFileSyms)
		diags = append(diags, sortDiags(remapURIs(wsDiags))...)

		_, resDiags := sema.AnalyzeWorkspace(perFileSyms, ws)
		diags = append(diags, sortDiags(remapURIs(resDiags))...)

		diags = append(diags, sortDiags(remapURIs(sema.LintWorkspace(perFileTrees, ws, perFileLIs)))...)
	}

	return merged, diags, nil
}

// ParseDir recursively collects every *.craft file under root in fsys, parses
// and merges them via ParseFiles, and returns the same merged CraftDoc +
// diagnostics contract ParseFiles provides (deterministic ordering, cross-file
// resolution, LintWorkspace) — without the caller hand-assembling a file map.
//
// fsys is any fs.FS: an embed.FS, an fstest.MapFS, an fs.Sub-scoped subtree, or
// os.DirFS(dir) for a real directory. File keys in the returned diagnostics are
// the walked paths (root-relative to fsys, as fs.WalkDir yields them).
//
// Diagnostics are data, never surfaced via error; err is reserved for I/O
// failures during the directory walk (an unreadable directory, or a file that
// vanishes between WalkDir and ReadFile). Since ParseFiles currently always
// returns a nil error, ParseDir's error is only reachable via the walk.
func ParseDir(fsys fs.FS, root string) (*CraftDoc, []Diagnostic, error) {
	files := make(map[string][]byte)
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".craft") {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		files[p] = data
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("craft: walk %s: %w", root, err)
	}
	return ParseFiles(files)
}

// sortDiags stable-sorts a batch of diagnostics into a deterministic order (by
// file, then source position, then code). The workspace passes range over maps
// internally and therefore return their diagnostics in Go's randomized map
// order; ParseFiles is the boundary that promises deterministic output (plan
// D5), so it orders each workspace batch here before appending. The per-file
// parse/sema diagnostics are already deterministic (the file loop is sorted).
func sortDiags(diags []Diagnostic) []Diagnostic {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.SourceURI != b.SourceURI {
			return a.SourceURI < b.SourceURI
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		if a.Range.Start.Character != b.Range.Start.Character {
			return a.Range.Start.Character < b.Range.Start.Character
		}
		return a.Code < b.Code
	})
	return diags
}

// mergeDoc appends every slice field of src onto dst. All CraftDoc slices are
// merged (not just the four `inspect` reads) so the library returns a complete
// model.
func mergeDoc(dst, src *CraftDoc) {
	dst.Architectures = append(dst.Architectures, src.Architectures...)
	dst.Exposures = append(dst.Exposures, src.Exposures...)
	dst.Services = append(dst.Services, src.Services...)
	dst.UseCases = append(dst.UseCases, src.UseCases...)
	dst.Domains = append(dst.Domains, src.Domains...)
	dst.Actors = append(dst.Actors, src.Actors...)
	dst.ContextMap = append(dst.ContextMap, src.ContextMap...)
	dst.Glossary = append(dst.Glossary, src.Glossary...)
}

// stampURI sets SourceURI = key on a copy-safe slice (the passes return fresh
// slices per file, so in-place mutation is fine).
func stampURI(diags []Diagnostic, key string) []Diagnostic {
	for i := range diags {
		diags[i].SourceURI = key
	}
	return diags
}

// remapURIs converts the internal "file://"+key SourceURI set by the workspace
// passes back to the bare map key the caller supplied.
func remapURIs(diags []Diagnostic) []Diagnostic {
	for i := range diags {
		diags[i].SourceURI = strings.TrimPrefix(diags[i].SourceURI, "file://")
	}
	return diags
}
