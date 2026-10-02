// Package analysis turns Craft source files into a model: the merged document
// plus every diagnostic. It is the one place the pipeline is assembled — parse,
// project, collect symbols, merge, resolve, lint — and every caller (pkg/craft,
// the LSP workspace, the CLI, the HTTP server) goes through it.
//
// It has two stages. File is per file and pure, so callers may cache its result
// by content. Workspace combines any set of file results into one model.
package analysis

import (
	"fmt"
	"sort"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// FileResult is the per-file stage's output. Treat it as immutable.
type FileResult struct {
	// Name identifies the file: a path or a URI, as the caller chooses. It is
	// stamped on the file's use cases and as the SourceURI of its diagnostics.
	Name      string
	Green     *green.GreenNode
	LineIndex green.LineIndex
	Symbols   sema.Symbols
	// Doc is this file's own document, before merging and resolution.
	Doc *model.CraftDoc
	// Diagnostics are this file's parse and single-file semantic diagnostics.
	Diagnostics []model.Diagnostic
}

// Tree returns the file's red syntax tree.
func (f FileResult) Tree() syntax.SyntaxNode { return syntax.Root(f.Green) }

// File runs the per-file stage on src. It never fails: a parser panic becomes
// a craft/internal/parser-panic diagnostic on an empty tree.
func File(name, src string) FileResult {
	f := FileResult{Name: name}
	func() {
		defer func() {
			if r := recover(); r != nil {
				f.Green, f.LineIndex, _ = syntax.Parse("")
				f.Diagnostics = append(f.Diagnostics, model.Diagnostic{
					Code:     "craft/internal/parser-panic",
					Message:  fmt.Sprintf("internal parser error: %v", r),
					Severity: model.SeverityError,
				})
			}
		}()
		f.Green, f.LineIndex, f.Diagnostics = syntax.Parse(src)
	}()

	tree := f.Tree()
	f.Doc = project(tree, f.LineIndex, name)

	syms, semaDiags := sema.AnalyzeFile(name, tree, f.LineIndex)
	f.Symbols = syms
	f.Diagnostics = append(f.Diagnostics, semaDiags...)
	for i := range f.Diagnostics {
		f.Diagnostics[i].SourceURI = name
	}
	return f
}

// Result is the workspace stage's output.
type Result struct {
	// Doc is the merged document of every file, with every use-case
	// participant and service context resolved (model.Participant).
	Doc        *model.CraftDoc
	Symbols    sema.WorkspaceSymbols
	Resolution sema.ResolutionMap
	// Diagnostics holds every diagnostic — per-file and workspace-level — in a
	// total order (file, position, code, message), so identical input always
	// yields an identical slice.
	Diagnostics []model.Diagnostic
	// WorkspaceDiagnostics is the cross-file subset of Diagnostics, in the same
	// order. Callers that already show per-file diagnostics as each file changes
	// (the LSP) add these on top.
	WorkspaceDiagnostics []model.Diagnostic
}

// Workspace combines file results into one model. Files are visited in
// ascending Name order whatever order they are passed in, so the merged
// document and the diagnostics do not depend on the caller's ordering.
func Workspace(files []FileResult) Result {
	sorted := append([]FileResult(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	docs := make([]*model.CraftDoc, 0, len(sorted))
	perFileSyms := make(map[string]sema.Symbols, len(sorted))
	perFileTrees := make(map[string]syntax.SyntaxNode, len(sorted))
	perFileLIs := make(map[string]green.LineIndex, len(sorted))
	var all []model.Diagnostic
	for _, f := range sorted {
		docs = append(docs, f.Doc)
		perFileSyms[f.Name] = f.Symbols
		perFileTrees[f.Name] = f.Tree()
		perFileLIs[f.Name] = f.LineIndex
		all = append(all, f.Diagnostics...)
	}

	res := Result{Doc: mergeDocs(docs)}
	if len(sorted) > 0 {
		ws, mergeDiags := sema.MergeWorkspaceSymbols(perFileSyms)
		rm, resolveDiags := sema.AnalyzeWorkspace(perFileSyms, ws)
		lintDiags := sema.LintWorkspace(perFileTrees, ws, perFileLIs)
		res.Symbols, res.Resolution = ws, rm
		resolveParticipants(res.Doc, ws)

		wsDiags := append(append(mergeDiags, resolveDiags...), lintDiags...)
		res.WorkspaceDiagnostics = sortDiagnostics(wsDiags)
		all = append(all, wsDiags...)
	}
	res.Diagnostics = sortDiagnostics(all)
	return res
}

// sortDiagnostics sorts diags into a total order: file, position, code, then
// message. The message tiebreak matters: several rules fire repeatedly under
// one code at one position and draw their findings from maps, so without it
// the relative order of those diagnostics would vary between runs.
func sortDiagnostics(diags []model.Diagnostic) []model.Diagnostic {
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
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	return diags
}
