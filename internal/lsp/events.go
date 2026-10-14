package lsp

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
	"go.lsp.dev/protocol"
)

// v2.24 hovers and definitions (spec §4.6, §5.4, §6.6, §7.6, §8.7, §9.6):
// actor kinds, scope tags, listens rosters and their items, streamed replies,
// operation brackets with several items, and the SQL/ADDR egress verbs.

// actorKinds describes each known actor kind: its meaning (spec §6.1) and
// the shape the diagrams draw it as (§6.5).
var actorKinds = map[string]struct{ meaning, shape string }{
	lang.ActorUser:      {"a person", "a person"},
	lang.ActorSystem:    {"an external system", "a boundary"},
	lang.ActorService:   {"an external service", "a control"},
	lang.ActorScheduler: {"a clock that starts work (cron, an in-process ticker)", "a boundary"},
	lang.ActorTool:      {"a CLI, script or client a person runs; it, not the person, makes the calls", "a boundary"},
	lang.ActorInfra:     {"platform machinery: pod start, probes, CI pipelines, monitors", "a boundary"},
}

const inferredPayloadHover = "inferred: an ordinary payload word here; it marks an inferred hop only at the head of an action's bracket"

// v224Hover runs the v2.24 hovers in order; the first that applies wins.
func (s *Server) v224Hover(file syntax.File, f *workspace.File, offset green.TextSize) (string, bool) {
	if val, ok := actorKindHover(file, offset); ok {
		return val, true
	}
	if val, ok := tagKeyHover(file, offset); ok {
		return val, true
	}
	if val, ok := inferredPayloadWordHover(syntax.Root(f.Green), offset); ok {
		return val, true
	}
	if val, ok := s.streamHover(file, f, offset); ok {
		return val, true
	}
	if val, ok := s.listensHover(file, offset); ok {
		return val, true
	}
	return s.bracketHover(file, offset)
}

// within reports whether offset falls on the token (its end included, so a
// cursor right after the last character still hits it).
func within(tok *syntax.SyntaxToken, offset green.TextSize) bool {
	if tok == nil {
		return false
	}
	r := tok.TextRange()
	return offset >= r.Start && offset <= r.End
}

// actorKindHover describes the kind token of an actor declaration.
func actorKindHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, a := range file.Actors() {
		tok := a.ActorType()
		if tok == nil {
			tok = a.ActorTypeToken()
		}
		if !within(tok, offset) {
			continue
		}
		kind := tok.Text()
		if k, ok := actorKinds[kind]; ok {
			return kind + ": " + k.meaning + "; drawn as " + k.shape, true
		}
		return kind + ": not a recognised kind; drawn as a person", true
	}
	return "", false
}

// tagKeyHover describes the key of a `sites` or `envs` tag.
func tagKeyHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, tb := range uc.TagsBlocks() {
			for _, tag := range tb.Tags() {
				key := tag.Key()
				if key == nil || !within(key, offset) {
					continue
				}
				if k := key.Text(); k == lang.TagSites || k == lang.TagEnvs {
					return "scope tag: printed in the sequence title; the hub filters and checks its values", true
				}
			}
		}
	}
	return "", false
}

// inferredPayloadWordHover explains `inferred` heading the bracket of a
// receives trigger or a surface line, where it is plain payload (spec §9.2).
func inferredPayloadWordHover(root syntax.SyntaxNode, offset green.TextSize) (string, bool) {
	tok, ok := tokenUnder(root, offset)
	if !ok || tok.Kind() != syntax.SyntaxKindIdent || tok.Text() != lang.OpInferred {
		return "", false
	}
	op := tok.Parent()
	if op == nil || op.Kind() != syntax.SyntaxKindOpAnnotation {
		return "", false
	}
	if owner := op.Parent(); owner != nil && owner.Kind() == syntax.SyntaxKindAction {
		return "", false
	}
	// Only the bracket's first word: elsewhere `inferred` is just a word.
	for _, t := range op.AllTokens() {
		switch t.Kind() {
		case syntax.SyntaxKindLBracket, syntax.SyntaxKindWhitespace:
			continue
		}
		if t.Offset() == tok.Offset() {
			return inferredPayloadHover, true
		}
		return "", false
	}
	return "", false
}

// modelAction finds the model action projected from the syntax action at
// line (1-based) of uri, with the use case that holds it.
func modelAction(doc *model.CraftDoc, uri string, line int) (*model.UseCase, *model.Action) {
	if doc == nil {
		return nil, nil
	}
	for i := range doc.UseCases {
		uc := &doc.UseCases[i]
		if uc.SourceURI != uri {
			continue
		}
		for _, sc := range uc.Scenarios {
			for _, a := range sc.Actions() {
				if a.Line == line {
					return uc, &a
				}
			}
		}
	}
	return nil, nil
}

// streamHover describes the `stream` of a `returns stream`, naming the
// request it answers (model.Action.Answers).
func (s *Server) streamHover(file syntax.File, f *workspace.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			for _, a := range sc.Actions() {
				if !a.IsStream() || !within(streamToken(a), offset) {
					continue
				}
				answers := "no request in the model"
				if muc, ma := modelAction(s.ws.Doc(), f.URI, a.Line(f.LineIndex)); ma != nil && ma.Answers != "" {
					for _, msc := range muc.Scenarios {
						for _, req := range msc.Actions() {
							if req.ID == ma.Answers {
								answers = req.Description
							}
						}
					}
				}
				return "streamed reply: drawn in a `loop stream` box; answers " + answers, true
			}
		}
	}
	return "", false
}

// streamToken returns the `stream` keyword of a `returns stream` action.
func streamToken(a syntax.ActionDecl) *syntax.SyntaxToken {
	for _, tok := range a.Tokens() {
		if tok.Kind() == syntax.SyntaxKindKwStream {
			return &tok
		}
	}
	return nil
}

// itemEnd is the end offset of an event item's source text.
func itemEnd(it syntax.EventItem) green.TextSize {
	n := len(it.Ref)
	if it.IsString || it.Ref == "" {
		n = len(it.Value) + 2
	}
	return it.Offset + green.TextSize(n)
}

// eventItemAt returns the item of a listens trigger under offset.
func eventItemAt(file syntax.File, offset green.TextSize) (syntax.EventItem, bool) {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			for _, it := range sc.Trigger().EventItems() {
				if offset >= it.Offset && offset <= itemEnd(it) {
					return it, true
				}
			}
		}
	}
	return syntax.EventItem{}, false
}

// publishersByEvent maps every published event to the display names of its
// publishers.
func publishersByEvent(doc *model.CraftDoc) map[string][]string {
	out := map[string][]string{}
	if doc == nil {
		return out
	}
	for _, it := range interaction.Derive(doc) {
		if it.Kind == interaction.Publish && !slices.Contains(out[it.Event], it.From.Name) {
			out[it.Event] = append(out[it.Event], it.From.Name)
		}
	}
	return out
}

// listensHover describes a listens roster (on its subject or `listens`) or
// one of its items. A single-event trigger keeps the hovers it had.
func (s *Server) listensHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			tr := sc.Trigger()
			items := tr.EventItems()
			if len(items) < 2 {
				continue
			}
			pubs := publishersByEvent(s.ws.Doc())
			for _, it := range items {
				if offset < it.Offset || offset > itemEnd(it) {
					continue
				}
				if p := pubs[it.Value]; len(p) > 0 {
					sorted := slices.Sorted(slices.Values(p))
					return it.Value + ": published by " + strings.Join(sorted, ", "), true
				}
				return it.Value + ": no publisher in the model", true
			}
			subj := tr.Subject()
			var listens *syntax.SyntaxToken
			for _, tok := range tr.Tokens() {
				if tok.Kind() == syntax.SyntaxKindKwListens {
					listens = &tok
					break
				}
			}
			if subj == nil || listens == nil || offset < subj.Offset() || offset > listens.TextRange().End {
				continue
			}
			count := map[string]int{}
			for _, it := range items {
				for _, p := range pubs[it.Value] {
					count[p]++
				}
			}
			if len(count) == 0 {
				return fmt.Sprintf("listens to %d events; publishers: none in the model", len(items)), true
			}
			var parts []string
			for _, p := range sortedKeys(count) {
				parts = append(parts, fmt.Sprintf("%s (%d of %d events)", p, count[p], len(items)))
			}
			return fmt.Sprintf("listens to %d events; publishers: %s", len(items), strings.Join(parts, ", ")), true
		}
	}
	return "", false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// bracketBody returns the offset of the first body token of an operation
// bracket: past `[`, whitespace and the `inferred` marker. OpText starts
// there, so an item's Start is relative to it.
func bracketBody(op *syntax.SyntaxNode) (green.TextSize, bool) {
	if op == nil {
		return 0, false
	}
	for _, tok := range op.AllTokens() {
		switch tok.Kind() {
		case syntax.SyntaxKindLBracket, syntax.SyntaxKindWhitespace, syntax.SyntaxKindKwInferred:
			continue
		case syntax.SyntaxKindRBracket:
			return 0, false
		}
		return tok.Offset(), true
	}
	return 0, false
}

// bracketItemAt returns the items of a's bracket and the index of the one
// under offset (-1 when the cursor is on the bracket but between items).
func bracketItemAt(a syntax.ActionDecl, offset green.TextSize) ([]lang.OpItem, int, bool) {
	ann := a.OpAnnotation()
	if ann == nil {
		return nil, -1, false
	}
	if r := ann.TextRange(); offset < r.Start || offset > r.End {
		return nil, -1, false
	}
	items := lang.SplitOperationItems(a.OpText())
	body, ok := bracketBody(ann)
	if !ok {
		return items, -1, true
	}
	rel := int(offset - body)
	for i, it := range items {
		if rel >= it.Start && rel <= it.Start+len(it.Text) {
			return items, i, true
		}
	}
	return items, -1, true
}

// bracketHover describes an operation bracket: a SQL or ADDR verb under the
// cursor, or a bracket of two or more items with each item's surface entry.
func (s *Server) bracketHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			for _, a := range sc.Actions() {
				items, i, ok := bracketItemAt(a, offset)
				if !ok {
					continue
				}
				if i >= 0 {
					body, _ := bracketBody(a.OpAnnotation())
					verb, payload := lang.ParseOperationItem(items[i].Text)
					onVerb := int(offset-body) <= items[i].Start+len(verb)
					switch {
					case onVerb && verb == lang.OpVerbSQL:
						return "direct use of another party's tables: " + strings.Join(strings.Fields(payload), ", "), true
					case onVerb && verb == lang.OpVerbAddr:
						return "egress to an address with no operation id; the hub does not resolve it", true
					}
				}
				if len(items) < 2 {
					return "", false
				}
				var names []string
				for _, it := range items {
					names = append(names, it.Text)
				}
				val := fmt.Sprintf("%d operations: %s", len(items), strings.Join(names, ", "))
				if a.Kind() == lang.ActionSync {
					for _, it := range items {
						op := sema.CollapseOperation(it.Text)
						if o, _, ok := surfaceOpFor(s.ws, a.TargetName(), op); ok {
							val += "\n" + op + ": " + string(o.Kind()) + ": " + o.PhraseText()
						}
					}
				}
				return val, true
			}
		}
	}
	return "", false
}

// eventDefinition goes from a listens item (single or in a list) to every
// `notifies` action that publishes it: files in sorted URI order, then
// source order. A store ref is left to the store definition.
func (s *Server) eventDefinition(file syntax.File, offset green.TextSize) ([]protocol.Location, bool) {
	item, ok := eventItemAt(file, offset)
	if !ok || strings.HasPrefix(item.Value, lang.StoreTransport+".") {
		return nil, false
	}
	files := s.ws.AllFiles()
	sort.Slice(files, func(i, j int) bool { return files[i].URI < files[j].URI })
	var locs []protocol.Location
	for _, f := range files {
		if f.Green == nil {
			continue
		}
		for _, uc := range syntax.AsFile(syntax.Root(f.Green)).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if a.Kind() != lang.ActionAsync {
						continue
					}
					for _, it := range a.EventItems() {
						if it.Value != item.Value {
							continue
						}
						start := it.Offset
						if subj := a.Subject(); subj != nil {
							start = subj.Offset()
						}
						locs = append(locs, protocol.Location{
							URI: protocol.DocumentURI(f.URI),
							Range: protocol.Range{
								Start: lspPosFromOffset(f.LineIndex, start),
								End:   lspPosFromOffset(f.LineIndex, itemEnd(it)),
							},
						})
						break
					}
				}
			}
		}
	}
	return locs, len(locs) > 0
}

// publishedEventCompletions offers every event published in the model (the
// items of each notifies), sorted, for an item of a listens or one-of list.
// Store refs are left out: a list may not hold one.
func publishedEventCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	seen := map[string]bool{}
	if doc := ws.Doc(); doc != nil {
		for _, uc := range doc.UseCases {
			for _, sc := range uc.Scenarios {
				for _, a := range sc.Actions() {
					if a.Type != model.ActionTypeAsync {
						continue
					}
					for _, ev := range a.EventList() {
						if ev != "" && !strings.HasPrefix(ev, lang.StoreTransport+".") {
							seen[ev] = true
						}
					}
				}
			}
		}
	}
	var items []protocol.CompletionItem
	for _, ev := range sortedKeys(seen) {
		items = append(items, protocol.CompletionItem{Label: ev, Kind: protocol.CompletionItemKindEvent, Detail: "published in the model"})
	}
	return &protocol.CompletionList{Items: items}
}

// opItemCompletions offers the next item of an asks bracket: the target's
// surface operations not yet in the bracket (spec §8.7), then the protocol
// verbs.
func opItemCompletions(ws *workspace.Workspace, root syntax.SyntaxNode, li green.LineIndex, cursorLine, cursorCol int) *protocol.CompletionList {
	lineStart := li.OffsetFromUTF16(cursorLine+1, 0)
	prefix := li.Src()[lineStart:li.OffsetFromUTF16(cursorLine+1, cursorCol)]
	written := map[string]bool{}
	if i := strings.LastIndexByte(prefix, '['); i >= 0 {
		for _, it := range lang.SplitOperationItems(prefix[i+1:]) {
			written[sema.CollapseOperation(it.Text)] = true
		}
	}
	var items []protocol.CompletionItem
	if target := syncTargetOnLine(syntax.AsFile(root), li, cursorLine+1); target != "" {
		for _, it := range surfaceOpCompletions(ws, target) {
			if !written[it.Label] {
				items = append(items, it)
			}
		}
	}
	items = append(items, protocolVerbCompletions().Items...)
	return &protocol.CompletionList{Items: items}
}

// tagKeyCompletions are the recognised scope tags, offered on a blank line of
// a tags block (spec §5.4).
func tagKeyCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	return &protocol.CompletionList{Items: []protocol.CompletionItem{
		{Label: lang.TagSites, Kind: protocol.CompletionItemKindField, Detail: "the sites this use case runs on; printed in the sequence title",
			InsertText: "sites: ${1:site}", InsertTextFormat: snip},
		{Label: lang.TagEnvs, Kind: protocol.CompletionItemKindField, Detail: "the environments this use case runs in; printed in the sequence title",
			InsertText: "envs: ${1:env}", InsertTextFormat: snip},
	}}
}

// tagValueCompletions offers the values already used for the tag on the
// cursor's line, most frequent first, then alphabetically, leaving out the
// values already written on the line. This keeps one spelling per site.
func tagValueCompletions(ws *workspace.Workspace, li green.LineIndex, cursorLine, cursorCol int) *protocol.CompletionList {
	lineStart := li.OffsetFromUTF16(cursorLine+1, 0)
	prefix := li.Src()[lineStart:li.OffsetFromUTF16(cursorLine+1, cursorCol)]
	key, rest, _ := strings.Cut(strings.TrimSpace(prefix), ":")
	written := map[string]bool{}
	for _, v := range strings.Split(rest, ",") {
		written[strings.TrimSpace(v)] = true
	}
	count := map[string]int{}
	if doc := ws.Doc(); doc != nil {
		for _, uc := range doc.UseCases {
			for _, v := range uc.TagValues[key] {
				if v != "" && !written[v] {
					count[v]++
				}
			}
		}
	}
	values := sortedKeys(count)
	sort.SliceStable(values, func(i, j int) bool { return count[values[i]] > count[values[j]] })
	var items []protocol.CompletionItem
	for i, v := range values {
		items = append(items, protocol.CompletionItem{
			Label: v, Kind: protocol.CompletionItemKindValue,
			Detail:   fmt.Sprintf("used by %d use case(s)", count[v]),
			SortText: fmt.Sprintf("%04d", i),
		})
	}
	return &protocol.CompletionList{Items: items}
}
