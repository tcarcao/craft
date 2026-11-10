package lsp

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// Pages in the editor (v2.36), on the same paths as flows: hover on a page
// declaration or on an `entry` value, definition from an `entry` value to its
// page, an outline entry and a fold per page, and completion of page
// properties, platforms and `entry` refs.

// entryValues lists every `entry` value of file, in source order, from each
// use case's first tags block: the one the model reads.
func entryValues(file syntax.File) []realisesValue {
	var out []realisesValue
	for _, uc := range file.UseCases() {
		if tb := uc.TagsBlock(); tb != nil {
			for _, tag := range tb.Tags() {
				k := tag.Key()
				vals, rngs := tag.Values(), tag.ValueRanges()
				if k == nil || k.Text() != lang.TagEntry || len(vals) != len(rngs) {
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

// entryValueAt returns the `entry` value that holds offset.
func entryValueAt(file syntax.File, offset green.TextSize) (realisesValue, bool) {
	for _, v := range entryValues(file) {
		if offset >= v.rng.Start && offset <= v.rng.End {
			return v, true
		}
	}
	return realisesValue{}, false
}

// pageHeaderAt returns the page whose header (`page <ref> "<name>"`) holds
// offset.
func pageHeaderAt(file syntax.File, offset green.TextSize) (syntax.PageDecl, bool) {
	for _, pg := range file.Pages() {
		for _, tok := range []*syntax.SyntaxToken{pg.Keyword(), pg.RefToken(), pg.NameToken()} {
			if within(tok, offset) {
				return pg, true
			}
		}
	}
	return syntax.PageDecl{}, false
}

// pageHover is the hover of a page header, of an `entry` value, and of the
// keys `entry`, `url`, `platform`, `product_domain` and `alias`.
func (s *Server) pageHover(f *workspace.File, offset green.TextSize) (string, bool) {
	file := syntax.AsFile(syntax.Root(f.Green))
	if d, ok := pageHeaderAt(file, offset); ok {
		// A later declaration of a ref is a duplicate-page error: the model
		// keeps the first, so describe that one and say so.
		if pg := s.ws.Doc().PageByRef(d.Ref()); pg != nil && (pg.SourceURI != f.URI || pg.Line != d.Line(f.LineIndex)) {
			return fmt.Sprintf("page %s is declared again here; the page is the one at %s:%d\n%s",
				d.Ref(), s.displayPath(pg.SourceURI), pg.Line, s.pageSummary(*pg)), true
		}
		pg := model.Page{Ref: d.Ref(), Name: d.Name(), URLs: uniqueValues(d.PropertyValues(lang.PageURL)),
			Aliases: uniqueValues(d.PropertyValues(lang.PageAlias))}
		pg.Platform, _ = d.Property(lang.PagePlatform)
		pg.ProductDomain, _ = d.Property(lang.PageProductDomain)
		return s.pageSummary(pg), true
	}
	if v, ok := entryValueAt(file, offset); ok {
		pg := s.ws.Doc().PageByRef(v.ref)
		if pg == nil {
			return fmt.Sprintf("page %q: no page has this ref in the open workspace", v.ref), true
		}
		return s.pageSummary(*pg), true
	}
	return pageKeyHover(file, offset)
}

// pageSummary describes a page: its name, urls, platform, product domain and aliases,
// then the use cases that start on it, each with its file.
func (s *Server) pageSummary(pg model.Page) string {
	lines := []string{fmt.Sprintf("page %s: %s", pg.Ref, pg.Name)}
	for _, u := range pg.URLs {
		lines = append(lines, "url: "+u)
	}
	if pg.Platform != "" {
		lines = append(lines, "platform: "+pg.Platform)
	}
	if pg.ProductDomain != "" {
		lines = append(lines, "product domain: "+pg.ProductDomain)
	}
	if len(pg.Aliases) > 0 {
		lines = append(lines, "aliases: "+strings.Join(pg.Aliases, ", "))
	}
	var usedBy []model.UseCaseRef
	if p := s.ws.Doc().PageByRef(pg.Ref); p != nil {
		usedBy = p.UsedBy
	}
	if len(usedBy) == 0 {
		lines = append(lines, "used by no use case: add `entry: "+pg.Ref+"` to the tags of the use cases that start on it")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "used by "+useCaseCount(len(usedBy))+":")
	for i, r := range usedBy {
		lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, r.Name, s.displayPath(r.SourceURI)))
	}
	return strings.Join(lines, "\n")
}

// pageKeyHover explains the `entry` tag key and the keys of a page body.
func pageKeyHover(file syntax.File, offset green.TextSize) (string, bool) {
	for _, uc := range file.UseCases() {
		for _, tb := range uc.TagsBlocks() {
			for _, tag := range tb.Tags() {
				if k := tag.Key(); k != nil && k.Text() == lang.TagEntry && within(k, offset) {
					return "entry: the pages this use case starts on, by ref (`page <ref> \"<name>\" { … }`)", true
				}
			}
		}
	}
	for _, pg := range file.Pages() {
		for _, p := range pg.Properties() {
			if !within(p.KeyToken(), offset) {
				continue
			}
			switch p.Key() {
			case lang.PageURL:
				return "url: where users open the page; one `url:` line per site. Optional", true
			case lang.PagePlatform:
				return "platform: " + strings.Join(lang.PagePlatforms(), ", ") + ". Optional", true
			case lang.PageProductDomain:
				return "product_domain: the product domain that owns the page; its landlord owns the page. Required", true
			case lang.PageAlias:
				return "alias: another name people use for the page, in any language or jargon; one `alias:` line per name. Optional", true
			}
			return "", false
		}
	}
	return "", false
}

// pageDefinition goes from an `entry` value to the ref of its page.
func (s *Server) pageDefinition(f *workspace.File, offset green.TextSize) ([]protocol.Location, bool) {
	v, ok := entryValueAt(syntax.AsFile(syntax.Root(f.Green)), offset)
	if !ok {
		return nil, false
	}
	pg := s.ws.Doc().PageByRef(v.ref)
	if pg == nil {
		return nil, true
	}
	target := s.ws.Get(pg.SourceURI)
	if target == nil {
		return nil, true
	}
	for _, d := range syntax.AsFile(syntax.Root(target.Green)).Pages() {
		if t := d.RefToken(); t != nil && d.Ref() == pg.Ref && d.Line(target.LineIndex) == pg.Line {
			return []protocol.Location{{URI: protocol.DocumentURI(pg.SourceURI), Range: tokenRange(target.LineIndex, *t)}}, true
		}
	}
	return nil, true
}

// pageSymbols is the outline of the file's pages: one Interface symbol each.
func pageSymbols(file syntax.File, li green.LineIndex) []protocol.DocumentSymbol {
	var out []protocol.DocumentSymbol
	for _, pg := range file.Pages() {
		if pg.RefToken() == nil {
			continue
		}
		start := pg.Line(li)
		end := max(pg.EndLine(li), start)
		detail := "page"
		if d, ok := pg.Property(lang.PageProductDomain); ok {
			detail += " " + d
		}
		out = append(out, protocol.DocumentSymbol{
			Name:           "page " + pg.Ref(),
			Kind:           protocol.SymbolKindInterface,
			Detail:         detail,
			Range:          symbolLineRange(start, end),
			SelectionRange: symbolLineRange(start, start),
		})
	}
	return out
}

// pageFoldingRanges folds each page block.
func pageFoldingRanges(file syntax.File, li green.LineIndex) []protocol.FoldingRange {
	var out []protocol.FoldingRange
	for _, pg := range file.Pages() {
		if start, end := pg.Line(li), pg.EndLine(li); start > 0 && end > start {
			out = append(out, protocol.FoldingRange{
				StartLine: lspLine(start),
				EndLine:   lspLine(end),
				Kind:      protocol.RegionFoldingRange,
			})
		}
	}
	return out
}

// entryValueRe matches an `entry` tag typed up to a value, and
// pagePlatformRe a page's `platform:` typed up to its value.
var (
	entryValueRe   = regexp.MustCompile(`^\s*entry:\s*(\S+,\s*)*[^\s,]*$`)
	pagePlatformRe = regexp.MustCompile(`^\s*platform:\s*$`)
)

// pageBodyCompletions are the lines of a page body.
func pageBodyCompletions() *protocol.CompletionList {
	snip := protocol.InsertTextFormatSnippet
	field := func(label, detail, text string) protocol.CompletionItem {
		return protocol.CompletionItem{Label: label, Kind: protocol.CompletionItemKindField,
			Detail: detail, InsertText: text, InsertTextFormat: snip}
	}
	return &protocol.CompletionList{Items: []protocol.CompletionItem{
		field("url:", "optional, one per site: where users open the page", `url: "${1:https://}"`),
		field("platform:", "optional: frontoffice, backoffice or app", "platform: ${1|frontoffice,backoffice,app|}"),
		field("product_domain:", "required: the product domain that owns the page", "product_domain: ${1:product-domain}"),
		field("alias:", "optional, one per line: another name people use for the page", `alias: "${1:alias}"`),
		{Label: "note", Kind: protocol.CompletionItemKindKeyword, Detail: "a remark about the page",
			InsertText: `note "${1:text}"`, InsertTextFormat: snip},
	}}
}

// pagePlatformCompletions are frontoffice, backoffice and app.
func pagePlatformCompletions() *protocol.CompletionList {
	var items []protocol.CompletionItem
	for _, p := range lang.PagePlatforms() {
		items = append(items, protocol.CompletionItem{Label: p, Kind: protocol.CompletionItemKindEnumMember})
	}
	return &protocol.CompletionList{Items: items}
}

// entryCompletions offers the page refs of the workspace, leaving out those
// already written on the line.
func entryCompletions(ws *workspace.Workspace, linePrefix string) *protocol.CompletionList {
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
	for _, pg := range doc.Pages {
		if written[pg.Ref] {
			continue
		}
		list.Items = append(list.Items, protocol.CompletionItem{
			Label: pg.Ref, Kind: protocol.CompletionItemKindReference, Detail: pg.Name,
		})
	}
	return list
}

// uniqueValues keeps the first of each value, trimmed and in order, empty
// ones left out: the model's URLs and aliases.
func uniqueValues(values []string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}
