package lsp

import (
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/internal/workspace"
)

type completionContext int

const (
	ctxTopLevel        completionContext = iota
	ctxServiceField
	ctxServiceLang
	ctxServiceContexts
	ctxDomainBody
	ctxActorsBlock
	ctxUseCaseTop
	ctxUseCaseAction
	ctxExposeField
	ctxExposeTo
	ctxExposeThrough
	ctxExposeContexts
	ctxUnknown
)

// splitLines splits content into lines.
func splitLines(content string) []string {
	return strings.Split(content, "\n")
}

// detectContext determines what kind of completion to offer based on where
// the cursor sits in the grammar. It scans lines backwards counting { / } depth
// to find the nearest enclosing keyword, and checks the current line for
// field-value patterns (e.g. "language: ").
func detectContext(lines []string, cursorLine, cursorCol int) completionContext {
	linePrefix := ""
	if cursorLine < len(lines) {
		l := lines[cursorLine]
		if cursorCol <= len(l) {
			linePrefix = l[:cursorCol]
		} else {
			linePrefix = l
		}
	}

	enclosing := findEnclosingKeyword(lines, cursorLine)

	if field, ok := fieldBeforeCursor(linePrefix); ok {
		switch field {
		case "language":
			if enclosing == "service" || enclosing == "services" {
				return ctxServiceLang
			}
		case "contexts":
			if enclosing == "service" || enclosing == "services" {
				return ctxServiceContexts
			}
			if enclosing == "expose" {
				return ctxExposeContexts
			}
		case "to":
			if enclosing == "expose" {
				return ctxExposeTo
			}
		case "through":
			if enclosing == "expose" {
				return ctxExposeThrough
			}
		}
	}

	switch enclosing {
	case "":
		return ctxTopLevel
	case "service", "services":
		return ctxServiceField
	case "domain", "domains":
		return ctxDomainBody
	case "actors":
		return ctxActorsBlock
	case "use_case":
		if isActionLine(linePrefix) {
			return ctxUseCaseAction
		}
		return ctxUseCaseTop
	case "expose":
		return ctxExposeField
	default:
		return ctxUnknown
	}
}

// fieldBeforeCursor returns the field name if linePrefix looks like "  fieldname:  "
// (single word followed by colon, optional trailing spaces). Returns "", false otherwise.
func fieldBeforeCursor(linePrefix string) (string, bool) {
	trimmed := strings.TrimLeft(linePrefix, " \t")
	idx := strings.Index(trimmed, ":")
	if idx < 0 {
		return "", false
	}
	field := strings.TrimSpace(trimmed[:idx])
	if field == "" || strings.ContainsAny(field, " \t") {
		return "", false
	}
	return field, true
}

// knownKeywords is the set of top-level block-opening keywords in the Craft DSL.
var knownKeywords = map[string]bool{
	"service":  true,
	"services": true,
	"domain":   true,
	"domains":  true,
	"actors":   true,
	"use_case": true,
	"expose":   true,
	"arch":     true,
}

// findEnclosingKeyword scans lines backwards from cursorLine-1, tracking { / }
// depth, and returns the leading keyword of the line that opens the nearest
// enclosing block. When the immediate enclosing opener is not a known DSL
// keyword (e.g. a service name inside "services { Foo { ... } }"), the scan
// continues outward to find the containing block's keyword.
// Returns "" when the cursor is at the top level.
func findEnclosingKeyword(lines []string, cursorLine int) string {
	// needEncloser tracks how many levels out we still need to resolve.
	// 0 means: looking for the first unmatched '{'.
	// 1 means: found an unknown-keyword block; need the block that wraps it.
	needEncloser := 0
	depth := 0
	for i := cursorLine - 1; i >= 0; i-- {
		line := lines[i]
		for j := len(line) - 1; j >= 0; j-- {
			ch := line[j]
			if ch == '}' {
				depth++
			} else if ch == '{' {
				if depth > 0 {
					depth--
				} else if needEncloser > 0 {
					// This '{' closes the block that contains the unknown-keyword
					// block we found earlier. Return this line's keyword.
					kw := extractLeadingKeyword(line)
					if knownKeywords[kw] {
						return kw
					}
					// Still unknown; keep going one more level out.
					needEncloser++
				} else {
					kw := extractLeadingKeyword(line)
					if knownKeywords[kw] {
						return kw
					}
					// Unknown identifier opening a block (e.g. a service name
					// inside "services { }"). Look for the block that wraps this.
					needEncloser = 1
				}
			}
		}
	}
	return ""
}

// extractLeadingKeyword returns the first whitespace-separated token of a line.
func extractLeadingKeyword(line string) string {
	fields := strings.Fields(strings.TrimLeft(line, " \t"))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// isActionLine returns true when the line prefix starts with an identifier that
// is not "when" — indicating a use-case action line rather than a trigger line.
func isActionLine(linePrefix string) bool {
	trimmed := strings.TrimSpace(linePrefix)
	if trimmed == "" {
		return false
	}
	firstWord := strings.Fields(trimmed)[0]
	return firstWord != "when"
}

// buildCompletions is the main dispatcher: detects cursor context and returns
// the appropriate CompletionList. Returns nil when no completions apply.
func buildCompletions(ws *workspace.Workspace, uri string, params *protocol.CompletionParams) *protocol.CompletionList {
	f := ws.Get(uri)
	var lines []string
	if f != nil {
		lines = splitLines(f.Content)
	}

	cursorLine := int(params.Position.Line)
	cursorCol := int(params.Position.Character)

	ctx := detectContext(lines, cursorLine, cursorCol)

	switch ctx {
	case ctxTopLevel:
		return topLevelKeywordCompletions()
	case ctxServiceField:
		return serviceFieldCompletions()
	case ctxServiceLang:
		return languageValueCompletions()
	case ctxServiceContexts:
		return bcSymbolCompletions(ws)
	case ctxDomainBody:
		return domainBodyCompletions()
	case ctxActorsBlock:
		return actorTypeCompletions()
	case ctxUseCaseTop:
		return whenKeywordCompletion()
	case ctxUseCaseAction:
		return allSymbolCompletions(ws)
	case ctxExposeField:
		return exposeFieldCompletions()
	case ctxExposeTo:
		return actorSymbolCompletions(ws)
	case ctxExposeThrough:
		return serviceSymbolCompletions(ws)
	case ctxExposeContexts:
		return bcSymbolCompletions(ws)
	default:
		return nil
	}
}

// --- item builder stubs (implemented in Tasks 3–8) ---

func topLevelKeywordCompletions() *protocol.CompletionList                     { return nil }
func serviceFieldCompletions() *protocol.CompletionList                        { return nil }
func languageValueCompletions() *protocol.CompletionList                       { return nil }
func bcSymbolCompletions(_ *workspace.Workspace) *protocol.CompletionList      { return nil }
func domainBodyCompletions() *protocol.CompletionList                          { return nil }
func actorTypeCompletions() *protocol.CompletionList                           { return nil }
func whenKeywordCompletion() *protocol.CompletionList                          { return nil }
func allSymbolCompletions(_ *workspace.Workspace) *protocol.CompletionList     { return nil }
func exposeFieldCompletions() *protocol.CompletionList                         { return nil }
func actorSymbolCompletions(_ *workspace.Workspace) *protocol.CompletionList   { return nil }
func serviceSymbolCompletions(_ *workspace.Workspace) *protocol.CompletionList { return nil }
