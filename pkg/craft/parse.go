package craft

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/tcarcao/craft/v2/internal/analysis"
)

// Parse parses a single Craft source file as a model of one file: it returns
// the document plus every diagnostic `craft validate` would report for that
// file alone — syntax, semantic, cross-reference and lint.
//
// Diagnostics are returned as data, never via the error return: err is reserved
// for programmer-error/invariant conditions and is currently always nil. Parse
// performs no file I/O — the caller supplies src. The returned *CraftDoc is
// never nil, even for empty or badly broken input.
//
// Every returned diagnostic's SourceURI is set to filename (the exact string you
// pass).
func Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error) {
	return ParseFiles(map[string][]byte{filename: src})
}

// ParseFiles parses and merges multiple Craft source files into one document
// model, with full workspace analysis: cross-file resolution and lint. Files
// are processed in ascending map-key order, so the merged model and the
// diagnostic slice are deterministic regardless of Go map iteration order. A
// service or domain declared in more than one file keeps its first declaration
// (the later ones are reported as duplicate-name warnings).
//
// Every returned diagnostic's SourceURI is set to the originating map key (the
// bare filename you supplied). Diagnostics are data, not errors; err is
// currently always nil. Performs no file I/O.
func ParseFiles(files map[string][]byte) (*CraftDoc, []Diagnostic, error) {
	results := make([]analysis.FileResult, 0, len(files))
	for name, src := range files {
		results = append(results, analysis.File(name, string(src)))
	}
	res := analysis.Workspace(results)
	diags := res.Diagnostics
	if diags == nil {
		diags = []Diagnostic{}
	}
	return res.Doc, diags, nil
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
