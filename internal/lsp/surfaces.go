package lsp

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// ── Surfaces ──────────────────────────────────────────────────────────────────

// surfaceSymbols is the outline of the file's surfaces (spec §8.5): one
// Interface per surface, one Method per operation, named after its `[op]`.
func surfaceSymbols(file syntax.File, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, s := range file.Surfaces() {
		if s.SubjectToken() == nil {
			continue
		}
		start := s.Line(li)
		end := max(s.EndLine(li), start)
		var children []protocol.DocumentSymbol
		for _, op := range s.Operations() {
			text := sema.CollapseOperation(op.OpText())
			line := op.Line(li)
			if text == "" || line == 0 {
				continue
			}
			children = append(children, protocol.DocumentSymbol{
				Name:           text,
				Kind:           protocol.SymbolKindMethod,
				Detail:         string(op.Kind()),
				Range:          symbolLineRange(line, line),
				SelectionRange: symbolLineRange(line, line),
			})
		}
		out = append(out, protocol.DocumentSymbol{
			Name:           "surface " + s.Subject(),
			Kind:           protocol.SymbolKindInterface,
			Detail:         "surface",
			Range:          symbolLineRange(start, end),
			SelectionRange: symbolLineRange(start, start),
			Children:       children,
		})
	}
	return out
}

// surfaceFoldingRanges folds each surface block.
func surfaceFoldingRanges(file syntax.File, li green.LineIndex) []protocol.FoldingRange {
	var out []protocol.FoldingRange
	for _, s := range file.Surfaces() {
		if start, end := s.Line(li), s.EndLine(li); start > 0 && end > start {
			out = append(out, protocol.FoldingRange{
				StartLine: lspLine(start),
				EndLine:   lspLine(end),
				Kind:      protocol.RegionFoldingRange,
			})
		}
	}
	return out
}

// surfaceSubjectHover describes the surface whose header is on cursorLine
// (1-based): "surface of concierge: 3 operations (2 query, 1 command);
// callers: Advisor".
func surfaceSubjectHover(file syntax.File, li green.LineIndex, cursorLine int) (string, bool) {
	for _, s := range file.Surfaces() {
		if s.SubjectToken() == nil || s.Line(li) != cursorLine {
			continue
		}
		var queries, commands int
		for _, op := range s.Operations() {
			if op.Kind() == lang.SurfaceCommand {
				commands++
			} else {
				queries++
			}
		}
		n := queries + commands
		noun := "operations"
		if n == 1 {
			noun = "operation"
		}
		val := fmt.Sprintf("surface of %s: %d %s (%d query, %d command)", s.Subject(), n, noun, queries, commands)
		if callers := s.Callers(); len(callers) > 0 {
			val += "; callers: " + strings.Join(callers, ", ")
		}
		return val, true
	}
	return "", false
}

// surfaceOf finds the surface that covers requests to target
// (model.SurfaceBoundary): of each boundary, the first surface in sorted URI
// order, as the model keeps it (see sema.resolveSurfaces).
func surfaceOf(ws *workspace.Workspace, target string) (syntax.SurfaceDecl, *workspace.File, bool) {
	syms := ws.WorkspaceSymbols()
	p, ambiguous := sema.ResolveParticipant(syms, "", target)
	if ambiguous || p.Kind == "" {
		return syntax.SurfaceDecl{}, nil, false
	}
	type located struct {
		decl syntax.SurfaceDecl
		file *workspace.File
	}
	first := map[string]located{}
	files := ws.AllFiles()
	sort.Slice(files, func(i, j int) bool { return files[i].URI < files[j].URI })
	for _, f := range files {
		if f.Green == nil {
			continue
		}
		for _, s := range syntax.AsFile(syntax.Root(f.Green)).Surfaces() {
			sp, amb := sema.ResolveParticipant(syms, "", s.Subject())
			if amb || sp.Kind == "" {
				continue
			}
			if _, seen := first[sp.ID]; !seen {
				first[sp.ID] = located{s, f}
			}
		}
	}
	id, ok := model.SurfaceBoundary(p, func(id string) bool { _, has := first[id]; return has })
	if !ok {
		return syntax.SurfaceDecl{}, nil, false
	}
	return first[id].decl, first[id].file, true
}

// surfaceOpFor returns the operation of target's surface whose `[op]` is op.
func surfaceOpFor(ws *workspace.Workspace, target, op string) (syntax.SurfaceOpDecl, *workspace.File, bool) {
	s, f, ok := surfaceOf(ws, target)
	if !ok {
		return syntax.SurfaceOpDecl{}, nil, false
	}
	for _, o := range s.Operations() {
		if sema.CollapseOperation(o.OpText()) == op {
			return o, f, true
		}
	}
	return syntax.SurfaceOpDecl{}, nil, false
}

// askBracketAt returns the `asks` action whose `[op]` bracket contains
// offset, and its collapsed operation text. When the bracket lists two or
// more operations, the text is the item under the cursor (spec §8.7), and
// ok is false between items.
func askBracketAt(file syntax.File, offset green.TextSize) (syntax.ActionDecl, string, bool) {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			for _, a := range sc.Actions() {
				if a.Kind() != lang.ActionSync {
					continue
				}
				items, i, ok := bracketItemAt(a, offset)
				switch {
				case !ok:
					continue
				case len(items) < 2:
					return a, sema.CollapseOperation(a.OpText()), true
				case i >= 0:
					return a, sema.CollapseOperation(items[i].Text), true
				}
				return syntax.ActionDecl{}, "", false
			}
		}
	}
	return syntax.ActionDecl{}, "", false
}

// surfaceOpLocation is the location of a surface operation line, from its
// keyword to the end of its bracket.
func surfaceOpLocation(o syntax.SurfaceOpDecl, f *workspace.File) (protocol.Location, bool) {
	kw := o.KeywordToken()
	if kw == nil {
		return protocol.Location{}, false
	}
	end := o.Node().TextRange().End
	if ann := o.OpAnnotation(); ann != nil {
		end = ann.TextRange().End
	}
	return protocol.Location{
		URI: protocol.DocumentURI(f.URI),
		Range: protocol.Range{
			Start: lspPosFromOffset(f.LineIndex, kw.Offset()),
			End:   lspPosFromOffset(f.LineIndex, end),
		},
	}, true
}

// surfaceBodyCompletions are the lines of a surface body.
func surfaceBodyCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	kw := func(label, detail, text string) protocol.CompletionItem {
		return protocol.CompletionItem{Label: label, Kind: protocol.CompletionItemKindKeyword,
			Detail: detail, InsertText: text, InsertTextFormat: snip}
	}
	return &protocol.CompletionList{Items: []protocol.CompletionItem{
		kw("query", "an operation that answers without changing state", "query ${1:what it answers}  [${2:operation}]"),
		kw("command", "an operation that changes state", "command ${1:what it changes}  [${2:operation}]"),
		{Label: "callers:", Kind: protocol.CompletionItemKindField, Detail: "who calls this boundary (`unknown` when not observed)"},
		{Label: "spec:", Kind: protocol.CompletionItemKindField, Detail: "path of the API specification"},
		kw("note", "a remark about the surface", `note "${1:text}"`),
		kw("finding", "a highlighted problem, exported to CraftDoc", `finding "${1:text}"`),
	}}
}

// surfaceOpCompletions lists the operations of target's surface.
func surfaceOpCompletions(ws *workspace.Workspace, target string) []protocol.CompletionItem {
	s, _, ok := surfaceOf(ws, target)
	if !ok {
		return nil
	}
	var items []protocol.CompletionItem
	for _, o := range s.Operations() {
		text := sema.CollapseOperation(o.OpText())
		if text == "" {
			continue
		}
		items = append(items, protocol.CompletionItem{
			Label:  text,
			Kind:   protocol.CompletionItemKindMethod,
			Detail: string(o.Kind()) + ": " + o.PhraseText(),
		})
	}
	return items
}

// syncTargetOnLine returns the target of the `asks` action on line (1-based).
func syncTargetOnLine(file syntax.File, li green.LineIndex, line int) string {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			for _, a := range sc.Actions() {
				if a.Kind() == lang.ActionSync && a.Line(li) == line {
					return a.TargetName()
				}
			}
		}
	}
	return ""
}

// ── Store hand-offs ───────────────────────────────────────────────────────────

// storeRefAt returns the store ref (`store.<name>[.<change>]`) under the
// cursor, in a `notifies` action or a `listens` trigger. line is 1-based and
// char a 0-based UTF-16 column.
func storeRefAt(file syntax.File, li green.LineIndex, line, char int) (string, bool) {
	hit := func(ref string, refLine, col int) bool {
		if refLine != line || !strings.HasPrefix(ref, lang.StoreTransport+".") {
			return false
		}
		start := col - 1
		return char >= start && char <= start+int(green.UTF16Len(ref))
	}
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			if tr := sc.Trigger(); tr.Kind() == lang.TriggerDomainListen && sc.When() != nil {
				l, _ := li.LineCol(sc.When().Offset())
				if ref := tr.EventRef(); hit(ref, l, tr.EventCol(li)) {
					return ref, true
				}
			}
			for _, a := range sc.Actions() {
				if a.Kind() != lang.ActionAsync {
					continue
				}
				if ref := a.ObjectRef(); hit(ref, a.Line(li), a.EventCol(li)) {
					return ref, true
				}
			}
		}
	}
	return "", false
}

// storeHover describes a store hand-off from the derived interactions:
// "store hand-off: sms_queue, change inserted; written by invoicing; polled
// by smsdispatch".
func storeHover(doc *model.CraftDoc, ref string) string {
	name, change, ok := lang.ParseStoreRef(ref)
	if !ok {
		return ""
	}
	val := "store hand-off: " + name
	if change != "" {
		val += ", change " + string(change)
	}
	var writers, readers []string
	polled := false
	if doc != nil {
		for _, it := range interaction.Derive(doc) {
			if it.Event != ref {
				continue
			}
			switch it.Kind {
			case interaction.Publish:
				writers = append(writers, it.From.Name)
			case interaction.Listen:
				readers = append(readers, it.To.Name)
				polled = polled || it.Schedule != ""
			}
		}
	}
	list := func(names []string) string {
		slices.Sort(names)
		return strings.Join(slices.Compact(names), ", ")
	}
	if len(writers) > 0 {
		val += "; written by " + list(writers)
	} else {
		val += "; no writer in the model"
	}
	verb := "read by "
	if polled {
		verb = "polled by "
	}
	if len(readers) > 0 {
		val += "; " + verb + list(readers)
	} else {
		val += "; no reader in the model"
	}
	return val
}

// storeDefinition locates the `data-stores:` entry a store ref names: in the
// owning service when the name resolves, else the first entry with that name
// in sorted URI order.
func storeDefinition(ws *workspace.Workspace, ref string) (protocol.Location, bool) {
	name, _, ok := lang.ParseStoreRef(ref)
	if !ok {
		return protocol.Location{}, false
	}
	owner := ""
	if p, _ := sema.ResolveParticipant(ws.WorkspaceSymbols(), "", name); p.Kind == model.ParticipantDataStore {
		owner = p.Service
	}
	files := ws.AllFiles()
	sort.Slice(files, func(i, j int) bool { return files[i].URI < files[j].URI })
	var fallback *protocol.Location
	for _, f := range files {
		if f.Green == nil {
			continue
		}
		for _, svc := range syntax.AsFile(syntax.Root(f.Green)).Services() {
			svcName := ""
			if t := svc.Name(); t != nil {
				svcName = syntax.StringAwareText(*t)
			}
			for _, tok := range svc.DataStoreTokens() {
				if syntax.StringAwareText(tok) != name {
					continue
				}
				loc := protocol.Location{
					URI: protocol.DocumentURI(f.URI),
					Range: protocol.Range{
						Start: lspPosFromOffset(f.LineIndex, tok.Offset()),
						End:   lspPosFromOffset(f.LineIndex, tok.TextRange().End),
					},
				}
				if owner == "" || svcName == owner {
					return loc, true
				}
				if fallback == nil {
					fallback = &loc
				}
			}
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return protocol.Location{}, false
}

// storeNameCompletions offers the store names of the workspace: every
// `data-stores:` entry and every name already used in a store ref.
func storeNameCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	seen := map[string]bool{}
	var items []protocol.CompletionItem
	add := func(name, detail string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		items = append(items, protocol.CompletionItem{Label: name, Kind: protocol.CompletionItemKindValue, Detail: detail})
	}
	files := ws.AllFiles()
	sort.Slice(files, func(i, j int) bool { return files[i].URI < files[j].URI })
	for _, f := range files {
		if f.Green == nil {
			continue
		}
		file := syntax.AsFile(syntax.Root(f.Green))
		for _, svc := range file.Services() {
			for _, ds := range svc.DataStores() {
				add(ds, "data store")
			}
		}
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				if tr := sc.Trigger(); tr.Kind() == lang.TriggerDomainListen {
					if name, _, ok := lang.ParseStoreRef(tr.EventRef()); ok {
						add(name, "store used in a hand-off")
					}
				}
				for _, a := range sc.Actions() {
					if a.Kind() == lang.ActionAsync {
						if name, _, ok := lang.ParseStoreRef(a.ObjectRef()); ok {
							add(name, "store used in a hand-off")
						}
					}
				}
			}
		}
	}
	return &protocol.CompletionList{Items: items}
}

// journeyHover is the hover of a surface header, of an `asks` bracket that
// names a surface operation, or of a store ref. cursorLine is 1-based and
// cursorChar a 0-based UTF-16 column.
func (s *Server) journeyHover(file syntax.File, li green.LineIndex, cursorLine, cursorChar int, offset green.TextSize) (string, bool) {
	if val, ok := surfaceSubjectHover(file, li, cursorLine); ok {
		return val, true
	}
	if a, op, ok := askBracketAt(file, offset); ok {
		if o, _, ok := surfaceOpFor(s.ws, a.TargetName(), op); ok {
			return string(o.Kind()) + ": " + o.PhraseText(), true
		}
	}
	if ref, ok := storeRefAt(file, li, cursorLine, cursorChar); ok {
		if val := storeHover(s.ws.Doc(), ref); val != "" {
			return val, true
		}
	}
	return "", false
}

// journeyDefinition goes from an `asks` bracket to its surface operation, and
// from a store ref to its `data-stores:` entry.
func (s *Server) journeyDefinition(file syntax.File, li green.LineIndex, cursorLine, cursorChar int) (protocol.Location, bool) {
	offset := li.OffsetFromUTF16(cursorLine, cursorChar)
	if a, op, ok := askBracketAt(file, offset); ok {
		if o, f, ok := surfaceOpFor(s.ws, a.TargetName(), op); ok {
			return surfaceOpLocation(o, f)
		}
	}
	if ref, ok := storeRefAt(file, li, cursorLine, cursorChar); ok {
		return storeDefinition(s.ws, ref)
	}
	return protocol.Location{}, false
}
