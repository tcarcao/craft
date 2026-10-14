package lsp

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
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

// first60 cuts s to its first 60 runes.
func first60(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:60])
	}
	return s
}

// findingSymbol is the outline entry of a `finding` (spec §5.5): named after
// the first 60 characters of its text, kind Event. Plain notes return false:
// they stay out of the outline to keep it short.
func findingSymbol(n syntax.NoteDecl, li green.LineIndex) (protocol.DocumentSymbol, bool) {
	if n.Kind() != lang.NoteFinding {
		return protocol.DocumentSymbol{}, false
	}
	start, end := nodeLines(n.Node(), li)
	if start == 0 {
		return protocol.DocumentSymbol{}, false
	}
	return protocol.DocumentSymbol{
		Name:           "finding: " + first60(n.Text()),
		Kind:           protocol.SymbolKindEvent,
		Detail:         "finding",
		Range:          symbolLineRange(start, end),
		SelectionRange: symbolLineRange(start, start),
	}, true
}

// findingSymbols returns the outline entries of the findings among notes.
func findingSymbols(notes []syntax.NoteDecl, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, n := range notes {
		if sym, ok := findingSymbol(n, li); ok {
			out = append(out, sym)
		}
	}
	return out
}

// blockSymbols returns the outline symbols for every block and finding among
// steps, recursing into branch bodies.
func blockSymbols(steps []syntax.StepDecl, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, st := range steps {
		if n, ok := st.Note(); ok {
			if sym, ok := findingSymbol(n, li); ok {
				out = append(out, sym)
			}
			continue
		}
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
						n, tail := len(b.Branches()), ""
						if b.IsFanOut() {
							// Each step that is not a note or a link is its own branch.
							n, tail = 0, "; each step runs concurrently"
							for _, s := range br.Steps() {
								if !s.IsNoteOrLink() {
									n++
								}
							}
						}
						noun := "branches"
						if n == 1 {
							noun = "branch"
						}
						found = fmt.Sprintf("%s block: %d %s%s", b.Kind(), n, noun, tail)
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

// keywordHovers are the hover texts of the journey-semantics keywords that
// have no declaration to resolve (spec §5.5, §6.6; v2.24 §8.7, §9.6).
var keywordHovers = map[syntax.SyntaxKind]string{
	syntax.SyntaxKindKwNote:     "note — drawn over the previous step",
	syntax.SyntaxKindKwFinding:  "finding — drawn highlighted and exported to CraftDoc (`Step.Note`)",
	syntax.SyntaxKindKwReceives: "callee-side trigger: the caller is not observed; drawn as a found message",
	syntax.SyntaxKindKwInferred: "inferred hop: drawn grey",
	syntax.SyntaxKindKwOne:      "notifies one of: one event per run, chosen at run time",
	syntax.SyntaxKindKwOf:       "notifies one of: one event per run, chosen at run time",
}

// keywordHover returns the hover text of the keyword under offset, if any.
func keywordHover(root syntax.SyntaxNode, offset green.TextSize) (string, bool) {
	var toks []syntax.SyntaxToken
	// The token whose first character is under the cursor comes first:
	// TokenAtOffset reports a token's start offset as Between its
	// neighbours, which misses `inferred` right after `[`.
	if tok, ok := tokenUnder(root, offset); ok {
		toks = append(toks, tok)
	}
	switch at := root.TokenAtOffset(offset).(type) {
	case syntax.Single:
		toks = append(toks, at.Token)
	case syntax.Between:
		toks = append(toks, at.Right, at.Left)
	}
	for _, tok := range toks {
		if v, ok := keywordHovers[tok.Kind()]; ok {
			return v, true
		}
	}
	return "", false
}

// tokenUnder returns the token holding the character at offset.
func tokenUnder(root syntax.SyntaxNode, offset green.TextSize) (syntax.SyntaxToken, bool) {
	for _, tok := range root.AllTokens() {
		if r := tok.TextRange(); offset >= r.Start && offset < r.End {
			return tok, true
		}
	}
	return syntax.SyntaxToken{}, false
}

// prevLineInPar reports whether the last non-blank line above cursorLine
// (0-based) ends inside a par block: only there does `and` continue a par.
func prevLineInPar(root syntax.SyntaxNode, li green.LineIndex, cursorLine int) bool {
	src := li.Src()
	for l := cursorLine - 1; l >= 0; l-- {
		start := int(li.OffsetFromUTF16(l+1, 0))
		end := strings.IndexByte(src[start:], '\n')
		if end < 0 {
			end = len(src) - start
		}
		text := strings.TrimRight(src[start:start+end], " \t\r")
		if strings.TrimSpace(text) == "" {
			continue
		}
		for node := root.NodeAt(green.TextSize(start + len(text) - 1)); node != nil; node = node.Parent() {
			if node.Kind() != syntax.SyntaxKindBlock {
				continue
			}
			if br := node.ChildNode(syntax.SyntaxKindBranch); br != nil {
				if h := br.ChildNode(syntax.SyntaxKindBranchHeader); h != nil &&
					h.ChildToken(syntax.SyntaxKindKwPar, syntax.SyntaxKindKwAnd) != nil {
					return true
				}
			}
		}
		return false
	}
	return false
}

// blockKeywordCompletions are the snippets offered on a blank line in a
// use-case body, next to `when`. `and` is offered only when inPar: it
// continues an open par.
func blockKeywordCompletions(inPar bool) []protocol.CompletionItem {
	snip := protocol.InsertTextFormatSnippet
	mk := func(label, detail, text string) protocol.CompletionItem {
		return protocol.CompletionItem{
			Label: label, Kind: protocol.CompletionItemKindKeyword,
			Detail: detail, InsertText: text, InsertTextFormat: snip,
		}
	}
	items := []protocol.CompletionItem{
		mk("scenario", "name the scenario that follows", `scenario "${1:name}"`),
		mk("alt", "alternative branches: exactly one runs", `alt "${1:condition}"`),
		mk("opt", "optional branch: runs only under its condition", `opt "${1:condition}"`),
		mk("else", "next branch of the enclosing alt", `else "${1:condition}"`),
		mk("par", "concurrent steps: each step, or each and-branch, runs alongside the others", `par "${1:label}"`),
		mk("loop", "repeated steps: the body runs once per item or until the condition", `loop "${1:condition}"`),
		mk("note", "a remark drawn over the previous step", `note "${1:text}"`),
		mk("finding", "a highlighted problem, exported to CraftDoc", `finding "${1:text}"`),
	}
	if inPar {
		items = append(items, mk("and", "next concurrent branch of the enclosing par", `and "${1:label}"`))
	}
	return items
}
