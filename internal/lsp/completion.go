package lsp

import (
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

type completionContext int

const (
	ctxTopLevel completionContext = iota
	ctxServiceField
	ctxServiceLang
	ctxServiceContexts
	ctxDomainBody
	ctxActorsBlock
	ctxUseCaseTop
	ctxUseCaseAction
	ctxUseCaseActionVerb // cursor after `asks` or `returns` — suggest domain/service names
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

// treeEnclosingBlock returns the enclosing DSL keyword string for the cursor
// position by walking the syntax tree — the same values findEnclosingKeyword
// returns ("service", "domain", "actors", "use_case", "expose", "arch", or "").
// Tree-based lookup correctly handles `}` characters inside comments and strings.
func treeEnclosingBlock(root syntax.SyntaxNode, li green.LineIndex, cursorLine, cursorCol int) string {
	offset := li.Offset(cursorLine+1, cursorCol+1)
	node := root.NodeAt(green.TextSize(offset))
	for node != nil {
		switch node.Kind() {
		case syntax.SyntaxKindServiceField:
			if node.ChildToken(syntax.SyntaxKindKwContexts) != nil {
				return "service.contexts"
			}
			if node.ChildToken(syntax.SyntaxKindKwLanguage) != nil {
				return "service.language"
			}
			if node.ChildToken(syntax.SyntaxKindKwDataStores) != nil {
				return "service.data-stores"
			}
			return "service"
		case syntax.SyntaxKindServiceDecl:
			return "service"
		case syntax.SyntaxKindServicesBlock:
			return "services"
		case syntax.SyntaxKindDomainDecl:
			return "domain"
		case syntax.SyntaxKindDomainsBlock:
			return "domains"
		case syntax.SyntaxKindActorsBlock:
			return "actors"
		case syntax.SyntaxKindUseCaseDecl, syntax.SyntaxKindScenario:
			return "use_case"
		case syntax.SyntaxKindExposureDecl:
			return "expose"
		case syntax.SyntaxKindArchDecl:
			return "arch"
		case syntax.SyntaxKindFile:
			return ""
		}
		parent := node.Parent()
		if parent == nil {
			break
		}
		node = parent
	}
	return ""
}

// detectContext determines what kind of completion to offer based on where
// the cursor sits in the grammar. It uses the syntax tree to find the nearest
// enclosing keyword, and checks the current line for field-value patterns
// (e.g. "language: ").
func detectContext(root syntax.SyntaxNode, li green.LineIndex, lines []string, cursorLine, cursorCol int) completionContext {
	linePrefix := ""
	if cursorLine < len(lines) {
		l := lines[cursorLine]
		if cursorCol <= len(l) {
			linePrefix = l[:cursorCol]
		} else {
			linePrefix = l
		}
	}

	enclosing := treeEnclosingBlock(root, li, cursorLine, cursorCol)

	// Tree-based service field detection (takes priority over regex).
	switch enclosing {
	case "service.contexts":
		return ctxServiceContexts
	case "service.language":
		return ctxServiceLang
	case "service.data-stores":
		return ctxServiceField
	}

	// Regex-based field detection: handles empty-value positions where the
	// ServiceField node may not span the cursor (e.g. "contexts: " with no values).
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
			if isAfterActionVerb(linePrefix) {
				return ctxUseCaseActionVerb
			}
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

// isAfterActionVerb returns true when linePrefix has exactly two whitespace-separated
// tokens and the second is `asks` or `returns`.
func isAfterActionVerb(linePrefix string) bool {
	fields := strings.Fields(strings.TrimSpace(linePrefix))
	if len(fields) != 2 {
		return false
	}
	return fields[1] == "asks" || fields[1] == "returns"
}

// buildCompletions is the main dispatcher: detects cursor context and returns
// the appropriate CompletionList. Returns nil when no completions apply.
func buildCompletions(ws *workspace.Workspace, uri string, params *protocol.CompletionParams) *protocol.CompletionList {
	f := ws.Get(uri)
	if f == nil {
		return nil
	}

	lines := splitLines(f.Content)
	cursorLine := int(params.Position.Line)
	cursorCol := int(params.Position.Character)

	root := syntax.Root(f.Green)
	ctx := detectContext(root, f.LineIndex, lines, cursorLine, cursorCol)

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
	case ctxUseCaseActionVerb:
		return domainAndServiceSymbolCompletions(ws)
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

func topLevelKeywordCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	items := []protocol.CompletionItem{
		{
			Label:            "service",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a top-level service",
			InsertText:       "service ${1:Name} {\n\tcontexts: ${2:Context}\n\tlanguage: ${3:golang}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "services",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a grouped services block",
			InsertText:       "services {\n\t${1:Name} {\n\t\tcontexts: ${2:Context}\n\t}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "domain",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a domain with bounded contexts",
			InsertText:       "domain ${1:Name} {\n\t${2:BoundedContext}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "domains",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a grouped domains block",
			InsertText:       "domains {\n\t${1:Name} {\n\t\t${2:BoundedContext}\n\t}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "actor",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a single actor",
			InsertText:       "actor ${1|user,system,service|} ${2:Name}",
			InsertTextFormat: snip,
		},
		{
			Label:            "actors",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare an actors block",
			InsertText:       "actors {\n\t${1|user,system,service|} ${2:Name}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "use_case",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare a use case with scenarios",
			InsertText:       "use_case \"${1:Name}\" {\n\twhen ${2:Actor} ${3:initiates} ${4:action}\n\t\t${5:Context} ${6:validates} ${7:request}\n}",
			InsertTextFormat: snip,
		},
		{
			Label:            "expose",
			Kind:             protocol.CompletionItemKindKeyword,
			Detail:           "declare an API exposure",
			InsertText:       "expose ${1:Name} {\n\tto: ${2:Actor}\n\tthrough: ${3:Service}\n}",
			InsertTextFormat: snip,
		},
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func serviceFieldCompletions() *protocol.CompletionList {
	items := []protocol.CompletionItem{
		{Label: "contexts:", Kind: protocol.CompletionItemKindField, Detail: "bounded context references"},
		{Label: "data-stores:", Kind: protocol.CompletionItemKindField, Detail: "data store names"},
		{Label: "language:", Kind: protocol.CompletionItemKindField, Detail: "implementation language"},
		{Label: "deployment:", Kind: protocol.CompletionItemKindField, Detail: "deployment strategy"},
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func languageValueCompletions() *protocol.CompletionList {
	langs := []string{"golang", "python", "java", "typescript", "kotlin", "rust", "scala", "csharp", "ruby", "php"}
	items := make([]protocol.CompletionItem, len(langs))
	for i, l := range langs {
		items[i] = protocol.CompletionItem{Label: l, Kind: protocol.CompletionItemKindValue}
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func bcSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for domName, dom := range wsSym.Domains {
		for _, bc := range dom.BoundedContexts {
			items = append(items, protocol.CompletionItem{
				Label:  bc,
				Kind:   protocol.CompletionItemKindModule,
				Detail: "bounded context (domain: " + domName + ")",
			})
		}
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}

// domainAndServiceSymbolCompletions returns bounded contexts and services.
// Used after action verbs (asks, returns) where the target is a domain context
// or service, not an actor.
func domainAndServiceSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for name := range wsSym.Services {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "service",
		})
	}
	for domName, dom := range wsSym.Domains {
		for _, bc := range dom.BoundedContexts {
			items = append(items, protocol.CompletionItem{
				Label:  bc,
				Kind:   protocol.CompletionItemKindModule,
				Detail: "bounded context (domain: " + domName + ")",
			})
		}
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}

func domainBodyCompletions() *protocol.CompletionList {
	return &protocol.CompletionList{IsIncomplete: false, Items: nil}
}
func actorTypeCompletions() *protocol.CompletionList {
	items := []protocol.CompletionItem{
		{Label: "user", Kind: protocol.CompletionItemKindKeyword, Detail: "human actor"},
		{Label: "system", Kind: protocol.CompletionItemKindKeyword, Detail: "external system actor"},
		{Label: "service", Kind: protocol.CompletionItemKindKeyword, Detail: "internal service actor"},
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func whenKeywordCompletion() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items: []protocol.CompletionItem{
			{
				Label:            "when",
				Kind:             protocol.CompletionItemKindKeyword,
				Detail:           "scenario trigger",
				InsertText:       "when ${1:Actor} ${2:initiates} ${3:action}\n\t${4:Context} ${5:validates} ${6:request}",
				InsertTextFormat: snip,
			},
		},
	}
}

func allSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for name, actor := range wsSym.Actors {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindVariable,
			Detail: "actor (" + string(actor.Type) + ")",
		})
	}
	for name := range wsSym.Services {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "service",
		})
	}
	for domName, dom := range wsSym.Domains {
		items = append(items, protocol.CompletionItem{
			Label:  domName,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "domain",
		})
		for _, bc := range dom.BoundedContexts {
			items = append(items, protocol.CompletionItem{
				Label:  bc,
				Kind:   protocol.CompletionItemKindModule,
				Detail: "bounded context (domain: " + domName + ")",
			})
		}
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func exposeFieldCompletions() *protocol.CompletionList {
	items := []protocol.CompletionItem{
		{Label: "to:", Kind: protocol.CompletionItemKindField, Detail: "target actors"},
		{Label: "through:", Kind: protocol.CompletionItemKindField, Detail: "gateway services"},
		{Label: "contexts:", Kind: protocol.CompletionItemKindField, Detail: "exposed bounded contexts"},
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func actorSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for name, actor := range wsSym.Actors {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindVariable,
			Detail: "actor (" + string(actor.Type) + ")",
		})
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
func serviceSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for name := range wsSym.Services {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "service",
		})
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}

// ExportedBuildCompletions is a test-only export of buildCompletions.
var ExportedBuildCompletions = buildCompletions
