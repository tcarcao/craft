package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// buildLintWorkspace runs the real single-file pipeline (AnalyzeFile ->
// MergeWorkspaceSymbols) over src and returns the pieces LintWorkspace needs:
// the per-file syntax tree map, the merged workspace symbol table, and the
// per-file line-index map. Mirrors analyzeRelationshipSrc in
// validate_relationship_test.go, but stops short of AnalyzeWorkspace since
// these tests exercise the lint pass directly.
func buildLintWorkspace(t *testing.T, src string) (map[string]syntax.SyntaxNode, WorkspaceSymbols, map[string]green.LineIndex) {
	t.Helper()
	g, li, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	uri := "file:///lint.craft"
	syms, _ := AnalyzeFile(uri, tree, li)
	perFile := map[string]Symbols{uri: syms}
	ws, _ := MergeWorkspaceSymbols(perFile)
	return map[string]syntax.SyntaxNode{uri: tree}, ws, map[string]green.LineIndex{uri: li}
}
