package lsp

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// Links (`includes "<title>"` / `continues "<title>"`, spec §15.6): outline,
// definition, references, hover, title completion and the former-title quick
// fix.

// eachStep calls fn for every step of steps, depth first, branch bodies
// included.
func eachStep(steps []syntax.StepDecl, fn func(syntax.StepDecl)) {
	for _, st := range steps {
		fn(st)
		if b, ok := st.Block(); ok {
			for _, br := range b.Branches() {
				eachStep(br.Steps(), fn)
			}
		}
	}
}

// fileLinks returns every link step of file, in source order.
func fileLinks(file syntax.File) []syntax.LinkDecl {
	var out []syntax.LinkDecl
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			eachStep(sc.Steps(), func(st syntax.StepDecl) {
				if l, ok := st.Link(); ok {
					out = append(out, l)
				}
			})
		}
	}
	return out
}

// linkAt returns the link whose title holds offset.
func linkAt(file syntax.File, offset green.TextSize) (syntax.LinkDecl, bool) {
	for _, l := range fileLinks(file) {
		if t := l.TitleToken(); t != nil {
			if r := t.TextRange(); offset >= r.Start && offset < r.End {
				return l, true
			}
		}
	}
	return syntax.LinkDecl{}, false
}

// linkOnLine returns the link written on line (1-based).
func linkOnLine(file syntax.File, li green.LineIndex, line int) (syntax.LinkDecl, bool) {
	for _, l := range fileLinks(file) {
		if t := l.TitleToken(); t != nil {
			if ln, _ := li.LineCol(t.Offset()); ln == line {
				return l, true
			}
		}
	}
	return syntax.LinkDecl{}, false
}

// tokenRange is the LSP range of tok.
func tokenRange(li green.LineIndex, tok syntax.SyntaxToken) protocol.Range {
	r := tok.TextRange()
	return protocol.Range{Start: lspPosFromOffset(li, r.Start), End: lspPosFromOffset(li, r.End)}
}

// eachModelLink calls fn for every link of doc with the use case holding it.
func eachModelLink(doc *model.CraftDoc, fn func(uc model.UseCase, l *model.Link)) {
	if doc == nil {
		return
	}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			sc.EachLink(func(l *model.Link) { fn(uc, l) })
		}
	}
}

// modelLink returns the resolved link of uri written on line.
func modelLink(doc *model.CraftDoc, uri string, line int) *model.Link {
	var found *model.Link
	eachModelLink(doc, func(uc model.UseCase, l *model.Link) {
		if found == nil && uc.SourceURI == uri && l.Line == line {
			found = l
		}
	})
	return found
}

// modelUseCase returns the use case ref names.
func modelUseCase(doc *model.CraftDoc, ref model.UseCaseRef) (model.UseCase, bool) {
	if doc != nil {
		for _, uc := range doc.UseCases {
			if uc.Name == ref.Name && uc.SourceURI == ref.SourceURI {
				return uc, true
			}
		}
	}
	return model.UseCase{}, false
}

// useCaseLocation returns the location of the title of the use case ref names.
func (s *Server) useCaseLocation(ref model.UseCaseRef) (protocol.Location, bool) {
	f := s.ws.Get(ref.SourceURI)
	if f == nil {
		return protocol.Location{}, false
	}
	for _, uc := range syntax.AsFile(syntax.Root(f.Green)).UseCases() {
		if t := uc.Title(); t != nil && uc.Name() == ref.Name {
			return protocol.Location{URI: protocol.DocumentURI(ref.SourceURI), Range: tokenRange(f.LineIndex, *t)}, true
		}
	}
	return protocol.Location{}, false
}

// displayPath is uri as a path, relative to the workspace root when it lies
// under it.
func (s *Server) displayPath(uri string) string {
	p := uriToPath(uri)
	if p == "" {
		return uri
	}
	if root := strings.TrimSuffix(s.rootPath, "/"); root != "" && strings.HasPrefix(p, root+"/") {
		return p[len(root)+1:]
	}
	return p
}

// linkDefinition goes from a link title to its target's use_case title.
func (s *Server) linkDefinition(f *workspace.File, uri string, offset green.TextSize) ([]protocol.Location, bool) {
	file := syntax.AsFile(syntax.Root(f.Green))
	l, ok := linkAt(file, offset)
	if !ok {
		return nil, false
	}
	line, _ := f.LineIndex.LineCol(l.TitleToken().Offset())
	ml := modelLink(s.ws.Doc(), uri, line)
	if ml == nil || ml.Target == nil {
		return nil, true // a link title, but unresolved: nothing else applies
	}
	loc, ok := s.useCaseLocation(*ml.Target)
	if !ok {
		return nil, true
	}
	return []protocol.Location{loc}, true
}

// linkHover describes a link's target: its file, first trigger and journey.
func (s *Server) linkHover(f *workspace.File, uri string, offset green.TextSize) (string, bool) {
	file := syntax.AsFile(syntax.Root(f.Green))
	l, ok := linkAt(file, offset)
	if !ok {
		return "", false
	}
	line, _ := f.LineIndex.LineCol(l.TitleToken().Offset())
	ml := modelLink(s.ws.Doc(), uri, line)
	if ml == nil || ml.Target == nil {
		return fmt.Sprintf("%s %q: no use case has this title in the open workspace", l.Kind(), l.Title()), true
	}
	uc, ok := modelUseCase(s.ws.Doc(), *ml.Target)
	if !ok {
		return "", false
	}
	out := fmt.Sprintf("use case %q in %s", uc.Name, s.displayPath(uc.SourceURI))
	if len(uc.Scenarios) > 0 {
		out += "\nfirst trigger: " + uc.Scenarios[0].Trigger.Description
	}
	if slug := journey.Slug(uc); slug != "" {
		out += "\njourney: " + slug
	}
	return out, true
}

// References lists the links to the use case whose `use_case` line holds the
// cursor.
func (s *Server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	if params == nil {
		return nil, nil
	}
	uri := string(params.TextDocument.URI)
	f := s.ws.Get(uri)
	if f == nil {
		return nil, nil
	}
	cursorLine := int(params.Position.Line) + 1
	var name string
	var decl protocol.Location
	for _, uc := range syntax.AsFile(syntax.Root(f.Green)).UseCases() {
		if t := uc.Title(); t != nil {
			if line, _ := f.LineIndex.LineCol(t.Offset()); line == cursorLine {
				name = uc.Name()
				decl = protocol.Location{URI: params.TextDocument.URI, Range: tokenRange(f.LineIndex, *t)}
			}
		}
	}
	if name == "" {
		return nil, nil
	}
	var out []protocol.Location
	if params.Context.IncludeDeclaration {
		out = append(out, decl)
	}
	eachModelLink(s.ws.Doc(), func(uc model.UseCase, l *model.Link) {
		if l.Target == nil || l.Target.Name != name || l.Target.SourceURI != uri {
			return
		}
		lf := s.ws.Get(uc.SourceURI)
		if lf == nil {
			return
		}
		if sl, ok := linkOnLine(syntax.AsFile(syntax.Root(lf.Green)), lf.LineIndex, l.Line); ok {
			out = append(out, protocol.Location{URI: protocol.DocumentURI(uc.SourceURI), Range: tokenRange(lf.LineIndex, *sl.TitleToken())})
		}
	})
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].URI != out[j].URI {
			return out[i].URI < out[j].URI
		}
		return out[i].Range.Start.Line < out[j].Range.Start.Line
	})
	return out, nil
}

// linkTitleRe matches a line prefix inside the quotes of a link title.
var linkTitleRe = regexp.MustCompile(`^\s*(includes|continues)\s+"[^"]*$`)

// linkTitleCompletions offers every use-case title of the workspace, sorted,
// replacing what is typed between the opening quote and the cursor.
func linkTitleCompletions(ws *workspace.Workspace, linePrefix string, cursorLine int) *protocol.CompletionList {
	seen := map[string]bool{}
	var titles []string
	if doc := ws.Doc(); doc != nil {
		for _, uc := range doc.UseCases {
			if uc.Name != "" && !seen[uc.Name] {
				seen[uc.Name] = true
				titles = append(titles, uc.Name)
			}
		}
	}
	sort.Strings(titles)
	quote := strings.LastIndex(linePrefix, `"`)
	r := protocol.Range{
		Start: protocol.Position{Line: uint32(cursorLine), Character: green.UTF16Len(linePrefix[:quote+1])},
		End:   protocol.Position{Line: uint32(cursorLine), Character: green.UTF16Len(linePrefix)},
	}
	list := &protocol.CompletionList{}
	for _, t := range titles {
		list.Items = append(list.Items, protocol.CompletionItem{
			Label: t, Kind: protocol.CompletionItemKindReference, Detail: "use case",
			TextEdit: &protocol.TextEdit{Range: r, NewText: t},
		})
	}
	return list
}

// formerTitleCode is the lint the quick fix answers.
const formerTitleCode = "craft/lint/link-to-former-title"

// CodeAction offers, for each link-to-former-title diagnostic of the
// request, a quick fix that writes the target's current title.
func (s *Server) CodeAction(_ context.Context, params *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	if params == nil {
		return nil, nil
	}
	uri := string(params.TextDocument.URI)
	f := s.ws.Get(uri)
	if f == nil {
		return nil, nil
	}
	file := syntax.AsFile(syntax.Root(f.Green))
	var out []protocol.CodeAction
	for _, d := range params.Context.Diagnostics {
		if fmt.Sprint(d.Code) != formerTitleCode {
			continue
		}
		line := int(d.Range.Start.Line) + 1
		l, ok := linkOnLine(file, f.LineIndex, line)
		if !ok {
			continue
		}
		ml := modelLink(s.ws.Doc(), uri, line)
		if ml == nil || ml.Target == nil || ml.Target.Name == l.Title() {
			continue
		}
		name := ml.Target.Name
		out = append(out, protocol.CodeAction{
			Title:       "Link to the current title " + quoteCraftString(name),
			Kind:        protocol.QuickFix,
			Diagnostics: []protocol.Diagnostic{d},
			IsPreferred: true,
			Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				params.TextDocument.URI: {{Range: tokenRange(f.LineIndex, *l.TitleToken()), NewText: quoteCraftString(name)}},
			}},
		})
	}
	return out, nil
}

// linkSymbol is the outline entry of a link: `includes: <title>`, kind
// Module.
func linkSymbol(l syntax.LinkDecl, li green.LineIndex) (protocol.DocumentSymbol, bool) {
	start, end := nodeLines(l.Node(), li)
	if start == 0 || l.Title() == "" {
		return protocol.DocumentSymbol{}, false
	}
	return protocol.DocumentSymbol{
		Name:           string(l.Kind()) + ": " + l.Title(),
		Kind:           protocol.SymbolKindModule,
		Detail:         string(l.Kind()),
		Range:          symbolLineRange(start, end),
		SelectionRange: symbolLineRange(start, start),
	}, true
}
