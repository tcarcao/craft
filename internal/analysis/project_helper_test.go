package analysis_test

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// projectFile builds the document model of a parsed tree through
// internal/analysis. The tree is lossless, so its tokens (trivia included)
// spell out the original source.
func projectFile(root syntax.SyntaxNode, name string) *model.CraftDoc {
	return analysis.File(name, sourceOf(root)).Doc
}

// sourceOf reassembles the source text of a lossless tree.
func sourceOf(root syntax.SyntaxNode) string {
	var sb strings.Builder
	for _, tok := range root.AllTokens() {
		sb.WriteString(tok.Text())
	}
	return sb.String()
}
