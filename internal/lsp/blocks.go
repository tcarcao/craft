package lsp

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"go.lsp.dev/protocol"
)

// nodeLines returns the 1-based first and last lines of node's non-trivia
// tokens, or (0, 0) when it has none.
func nodeLines(node syntax.SyntaxNode, li green.LineIndex) (int, int) {
	toks := node.Tokens()
	if len(toks) == 0 {
		return 0, 0
	}
	start, _ := li.LineCol(toks[0].Offset())
	end, _ := li.LineCol(toks[len(toks)-1].Offset())
	return start, end
}

func symbolLineRange(start, end int) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: lspLine(start)},
		End:   protocol.Position{Line: lspLine(max(end, start))},
	}
}

// blockSymbolName labels a block with its kind and branch labels, e.g.
// "alt: accepted | declined".
func blockSymbolName(b syntax.BlockDecl) string {
	var labels []string
	for _, br := range b.Branches() {
		labels = append(labels, br.Label())
	}
	return string(b.Kind()) + ": " + strings.Join(labels, " | ")
}

// blockSymbols returns the outline symbols for every block among steps,
// recursing into branch bodies.
func blockSymbols(steps []syntax.StepDecl, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, st := range steps {
		b, ok := st.Block()
		if !ok {
			continue
		}
		start, end := nodeLines(b.Node(), li)
		if start == 0 {
			continue
		}
		var children []protocol.DocumentSymbol
		for _, br := range b.Branches() {
			children = append(children, blockSymbols(br.Steps(), li)...)
		}
		r := symbolLineRange(start, end)
		out = append(out, protocol.DocumentSymbol{
			Name:           blockSymbolName(b),
			Kind:           protocol.SymbolKindEnumMember,
			Detail:         string(b.Kind()),
			Range:          r,
			SelectionRange: symbolLineRange(start, start),
			Children:       children,
		})
	}
	return out
}

// scenarioSymbols returns one symbol per scenario of uc: the scenario name
// when it has one, otherwise its trigger line.
func scenarioSymbols(uc syntax.UseCaseDecl, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, sc := range uc.Scenarios() {
		when := sc.When()
		if when == nil {
			continue // ghost scenario left by an orphan `scenario "…"` line
		}
		start, end := nodeLines(sc.Node(), li)
		name := sc.Name()
		if name == "" {
			name = "when " + sc.Trigger().SourceText()
		}
		// Kinds: a scenario is a named behaviour of its use case (Method); a
		// block is one of a closed set of branch variants (EnumMember).
		out = append(out, protocol.DocumentSymbol{
			Name:           name,
			Kind:           protocol.SymbolKindMethod,
			Detail:         "scenario",
			Range:          symbolLineRange(start, end),
			SelectionRange: symbolLineRange(start, start),
			Children:       blockSymbols(sc.Steps(), li),
		})
	}
	return out
}

// blockFoldingRanges returns one fold per block (header line to last body
// line) among the scenarios of uc, including nested blocks.
func blockFoldingRanges(uc syntax.UseCaseDecl, li green.LineIndex) []protocol.FoldingRange {
	var out []protocol.FoldingRange
	var walk func([]syntax.StepDecl)
	walk = func(steps []syntax.StepDecl) {
		for _, st := range steps {
			b, ok := st.Block()
			if !ok {
				continue
			}
			if start, end := nodeLines(b.Node(), li); start > 0 && end > start {
				out = append(out, protocol.FoldingRange{
					StartLine: lspLine(start),
					EndLine:   lspLine(end),
					Kind:      protocol.RegionFoldingRange,
				})
			}
			for _, br := range b.Branches() {
				walk(br.Steps())
			}
		}
	}
	for _, sc := range uc.Scenarios() {
		walk(sc.Steps())
	}
	return out
}

// blockHover describes the block whose header line is cursorLine (1-based).
func blockHover(file syntax.File, li green.LineIndex, cursorLine int) (string, bool) {
	var found string
	var walk func([]syntax.StepDecl)
	walk = func(steps []syntax.StepDecl) {
		for _, st := range steps {
			b, ok := st.Block()
			if !ok || found != "" {
				continue
			}
			for _, br := range b.Branches() {
				if tok := br.HeaderToken(); tok != nil {
					if line, _ := li.LineCol(tok.Offset()); line == cursorLine {
						n := len(b.Branches())
						noun := "branches"
						if n == 1 {
							noun = "branch"
						}
						found = fmt.Sprintf("%s block: %d %s", b.Kind(), n, noun)
						return
					}
				}
				walk(br.Steps())
			}
		}
	}
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			walk(sc.Steps())
		}
	}
	return found, found != ""
}

// blockKeywordCompletions are the snippets offered on a blank line in a
// use-case body, next to `when`.
func blockKeywordCompletions() []protocol.CompletionItem {
	snip := protocol.InsertTextFormatSnippet
	mk := func(label, detail, text string) protocol.CompletionItem {
		return protocol.CompletionItem{
			Label: label, Kind: protocol.CompletionItemKindKeyword,
			Detail: detail, InsertText: text, InsertTextFormat: snip,
		}
	}
	return []protocol.CompletionItem{
		mk("scenario", "name the scenario that follows", `scenario "${1:name}"`),
		mk("alt", "alternative branches: exactly one runs", "alt \"${1:condition}\"\n\t${2:Context} ${3:returns} ${4:result}"),
		mk("opt", "optional branch: runs only under its condition", "opt \"${1:condition}\"\n\t${2:Context} ${3:returns} ${4:result}"),
		mk("else", "next branch of the enclosing alt", "else \"${1:condition}\"\n\t${2:Context} ${3:returns} ${4:result}"),
	}
}
