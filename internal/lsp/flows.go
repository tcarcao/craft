package lsp

import (
	"fmt"
	"regexp"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// Critical flows in the editor (v2.35): hover on a flow declaration or on a
// `realises` value, definition from a `realises` value to its flow, an
// outline entry and a fold per flow, and completion of flow properties,
// priorities and `realises` refs.

// realisesValue is one value of a use case's `realises` tag: the ref and
// its range.
type realisesValue struct {
	ref string
	rng green.TextRange
}

// realisesValues lists every `realises` value of file, in source order,
// from each use case's first tags block: the one the model reads.
func realisesValues(file syntax.File) []realisesValue {
	var out []realisesValue
	for _, uc := range file.UseCases() {
		if tb := uc.TagsBlock(); tb != nil {
			for _, tag := range tb.Tags() {
				k := tag.Key()
				vals, rngs := tag.Values(), tag.ValueRanges()
				if k == nil || k.Text() != lang.TagRealises || len(vals) != len(rngs) {
					continue
				}
				for i, v := range vals {
					out = append(out, realisesValue{ref: strings.TrimSpace(v), rng: rngs[i]})
				}
			}
		}
	}
	return out
}

// realisesValueAt returns the `realises` value that holds offset.
func realisesValueAt(file syntax.File, offset green.TextSize) (realisesValue, bool) {
	for _, v := range realisesValues(file) {
		if offset >= v.rng.Start && offset <= v.rng.End {
			return v, true
		}
	}
	return realisesValue{}, false
}

// flowHeaderAt returns the flow whose header (`flow <ref> "<name>"`) holds
// offset.
func flowHeaderAt(file syntax.File, offset green.TextSize) (syntax.FlowDecl, bool) {
	for _, f := range file.Flows() {
		for _, tok := range []*syntax.SyntaxToken{f.Keyword(), f.RefToken(), f.NameToken()} {
			if within(tok, offset) {
				return f, true
			}
		}
	}
	return syntax.FlowDecl{}, false
}

// flowHover is the hover of a flow header, of a `realises` value, and of
// the keys `realises`, `priority`, `owner` and `metric`.
func (s *Server) flowHover(f *workspace.File, offset green.TextSize) (string, bool) {
	file := syntax.AsFile(syntax.Root(f.Green))
	if d, ok := flowHeaderAt(file, offset); ok {
		// A later declaration of a ref is a duplicate-flow error: the model
		// keeps the first, so describe that one and say so.
		if fl := s.ws.Doc().FlowByRef(d.Ref()); fl != nil && (fl.SourceURI != f.URI || fl.Line != d.Line(f.LineIndex)) {
			return fmt.Sprintf("flow %s is declared again here; the flow is the one at %s:%d\n%s",
				d.Ref(), s.displayPath(fl.SourceURI), fl.Line, s.flowSummary(fl.Ref, fl.Name, fl.Priority, fl.Owner, fl.Metric)), true
		}
		name := d.Name()
		priority, _ := d.Property(lang.FlowPriority)
		owner, _ := d.Property(lang.FlowOwner)
		metric, _ := d.Property(lang.FlowMetric)
		return s.flowSummary(d.Ref(), name, priority, owner, metric), true
	}
	if v, ok := realisesValueAt(file, offset); ok {
		fl := s.ws.Doc().FlowByRef(v.ref)
		if fl == nil {
			return fmt.Sprintf("flow %q: no flow has this ref in the open workspace", v.ref), true
		}
		return s.flowSummary(fl.Ref, fl.Name, fl.Priority, fl.Owner, fl.Metric), true
	}
	return flowKeyHover(file, offset)
}

// flowSummary describes a flow: its name, priority, owner and metric, then
// the use cases that realise it, each with its file.
func (s *Server) flowSummary(ref, name, priority, owner, metric string) string {
	lines := []string{fmt.Sprintf("flow %s: %s", ref, name)}
	add := func(label, v string) {
		if v != "" {
			lines = append(lines, label+": "+v)
		}
	}
	add("priority", priority)
	add("owner", owner)
	add("metric", metric)
	var realisedBy []model.UseCaseRef
	if fl := s.ws.Doc().FlowByRef(ref); fl != nil {
		realisedBy = fl.RealisedBy
	}
	if len(realisedBy) == 0 {
		lines = append(lines, "realised by no use case: add `realises: "+ref+"` to the tags of the use cases that serve it")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "realised by "+useCaseCount(len(realisedBy))+":")
	for i, r := range realisedBy {
		lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, r.Name, s.displayPath(r.SourceURI)))
	}
	return strings.Join(lines, "\n")
}

// flowKeyHover explains the `realises` tag key and the keys of a flow body.
func flowKeyHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, tb := range uc.TagsBlocks() {
			for _, tag := range tb.Tags() {
				if k := tag.Key(); k != nil && k.Text() == lang.TagRealises && within(k, offset) {
					return "realises: the critical flows this use case serves, by ref (`flow <ref> \"<name>\" { … }`)", true
				}
			}
		}
	}
	for _, f := range file.Flows() {
		for _, p := range f.Properties() {
			if !within(p.KeyToken(), offset) {
				continue
			}
			switch p.Key() {
			case lang.FlowPriority:
				return "priority: " + strings.Join(lang.FlowPriorities(), ", ") + "; P0 is the highest. Required", true
			case lang.FlowOwner:
				return "owner: who owns the flow's definition, such as a product domain. Required", true
			case lang.FlowMetric:
				return "metric: the business metric the flow moves. Optional", true
			}
			return "", false
		}
	}
	return "", false
}

// flowDefinition goes from a `realises` value to the ref of its flow.
func (s *Server) flowDefinition(f *workspace.File, offset green.TextSize) ([]protocol.Location, bool) {
	v, ok := realisesValueAt(syntax.AsFile(syntax.Root(f.Green)), offset)
	if !ok {
		return nil, false
	}
	fl := s.ws.Doc().FlowByRef(v.ref)
	if fl == nil {
		return nil, true // a realises value, but unresolved: nothing else applies
	}
	target := s.ws.Get(fl.SourceURI)
	if target == nil {
		return nil, true
	}
	for _, d := range syntax.AsFile(syntax.Root(target.Green)).Flows() {
		if t := d.RefToken(); t != nil && d.Ref() == fl.Ref && d.Line(target.LineIndex) == fl.Line {
			return []protocol.Location{{URI: protocol.DocumentURI(fl.SourceURI), Range: tokenRange(target.LineIndex, *t)}}, true
		}
	}
	return nil, true
}

// flowSymbols is the outline of the file's flows: one Event symbol each.
func flowSymbols(file syntax.File, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, f := range file.Flows() {
		if f.RefToken() == nil {
			continue
		}
		start := f.Line(li)
		end := max(f.EndLine(li), start)
		detail := "flow"
		if p, ok := f.Property(lang.FlowPriority); ok {
			detail += " " + p
		}
		out = append(out, protocol.DocumentSymbol{
			Name:           "flow " + f.Ref(),
			Kind:           protocol.SymbolKindEvent,
			Detail:         detail,
			Range:          symbolLineRange(start, end),
			SelectionRange: symbolLineRange(start, start),
		})
	}
	return out
}

// flowFoldingRanges folds each flow block.
func flowFoldingRanges(file syntax.File, li green.LineIndex) []protocol.FoldingRange {
	var out []protocol.FoldingRange
	for _, f := range file.Flows() {
		if start, end := f.Line(li), f.EndLine(li); start > 0 && end > start {
			out = append(out, protocol.FoldingRange{
				StartLine: lspLine(start),
				EndLine:   lspLine(end),
				Kind:      protocol.RegionFoldingRange,
			})
		}
	}
	return out
}

// realisesValueRe matches a `realises` tag typed up to a value, and
// flowPriorityRe a flow's `priority:` typed up to its value.
var (
	realisesValueRe = regexp.MustCompile(`^\s*realises:\s*(\S+,\s*)*[^\s,]*$`)
	flowPriorityRe  = regexp.MustCompile(`^\s*priority:\s*$`)
)

// flowBodyCompletions are the lines of a flow body.
func flowBodyCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	field := func(label, detail, text string) protocol.CompletionItem {
		return protocol.CompletionItem{Label: label, Kind: protocol.CompletionItemKindField,
			Detail: detail, InsertText: text, InsertTextFormat: snip}
	}
	return &protocol.CompletionList{Items: []protocol.CompletionItem{
		field("priority:", "required: P0 (highest) to P3", "priority: ${1|P0,P1,P2,P3|}"),
		field("owner:", "required: who owns the flow's definition", "owner: ${1:product-domain}"),
		field("metric:", "optional: the business metric the flow moves", `metric: "${1:metric}"`),
		{Label: "note", Kind: protocol.CompletionItemKindKeyword, Detail: "a remark about the flow",
			InsertText: `note "${1:text}"`, InsertTextFormat: snip},
	}}
}

// flowPriorityCompletions are P0 to P3.
func flowPriorityCompletions() *protocol.CompletionList {
	var items []protocol.CompletionItem
	for i, p := range lang.FlowPriorities() {
		detail := ""
		if i == 0 {
			detail = "the highest"
		}
		items = append(items, protocol.CompletionItem{Label: p, Kind: protocol.CompletionItemKindEnumMember, Detail: detail})
	}
	return &protocol.CompletionList{Items: items}
}

// realisesCompletions offers the flow refs of the workspace, leaving out
// those already written on the line.
func realisesCompletions(ws *workspace.Workspace, linePrefix string) *protocol.CompletionList {
	_, rest, _ := strings.Cut(strings.TrimSpace(linePrefix), ":")
	written := map[string]bool{}
	for _, v := range strings.Split(rest, ",") {
		written[strings.TrimSpace(v)] = true
	}
	list := &protocol.CompletionList{}
	doc := ws.Doc()
	if doc == nil {
		return list
	}
	for _, f := range doc.Flows {
		if written[f.Ref] {
			continue
		}
		detail := f.Name
		if f.Priority != "" {
			detail = f.Priority + " " + detail
		}
		list.Items = append(list.Items, protocol.CompletionItem{
			Label: f.Ref, Kind: protocol.CompletionItemKindReference, Detail: detail,
		})
	}
	return list
}
