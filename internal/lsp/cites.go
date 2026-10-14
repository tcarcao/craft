package lsp

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// `//@` source cites (spec §14.6): hover on a cite, sources on the hover of
// the step it proves, and document links into a local checkout.

// citeOwner is a model element that can hold sources: an action, a trigger,
// a branch header, a note, a link or a use case.
type citeOwner struct {
	line    int    // the element's own line
	desc    string // how a hover names it
	sources []model.Source
	subject *model.Participant // whose service's repo a cite without one means
}

// citeOwners lists the elements of the use cases of uri that can hold
// sources, with or without any.
func citeOwners(doc *model.CraftDoc, uri string) []citeOwner {
	if doc == nil {
		return nil
	}
	var out []citeOwner
	for _, uc := range doc.UseCases {
		if uc.SourceURI != uri {
			continue
		}
		var first *model.Participant
		if len(uc.Scenarios) > 0 {
			first = uc.Scenarios[0].Trigger.Subject
		}
		out = append(out, citeOwner{line: uc.Line, desc: "use_case " + strconv.Quote(uc.Name), sources: uc.Sources, subject: first})
		for _, sc := range uc.Scenarios {
			t := sc.Trigger
			out = append(out, citeOwner{line: t.Line, desc: t.Description, sources: t.Sources, subject: t.Subject})
			out = appendStepOwners(out, sc.Steps, t.Subject)
		}
	}
	return out
}

func appendStepOwners(out []citeOwner, steps []model.Step, scenarioSubject *model.Participant) []citeOwner {
	for _, st := range steps {
		switch {
		case st.Action != nil:
			a := st.Action
			out = append(out, citeOwner{line: a.Line, desc: a.Description, sources: a.Sources, subject: a.From})
		case st.Note != nil:
			n := st.Note
			out = append(out, citeOwner{line: n.Line, desc: string(n.Kind) + " " + strconv.Quote(n.Text), sources: n.Sources, subject: scenarioSubject})
		case st.Link != nil:
			l := st.Link
			out = append(out, citeOwner{line: l.Line, desc: string(l.Kind) + " " + strconv.Quote(l.Title), sources: l.Sources, subject: scenarioSubject})
		case st.Block != nil:
			for i, br := range st.Block.Branches {
				out = append(out, citeOwner{line: br.Line, desc: branchKeyword(st.Block, i) + " " + strconv.Quote(br.Label), sources: br.Sources, subject: scenarioSubject})
				out = appendStepOwners(out, br.Steps, scenarioSubject)
			}
		}
	}
	return out
}

// branchKeyword is the keyword that opens branch i of b.
func branchKeyword(b *model.Block, i int) string {
	if i > 0 && !b.Split {
		switch b.Kind {
		case model.BlockAlt:
			return "else"
		case model.BlockPar:
			return "and"
		}
	}
	return string(b.Kind)
}

// ownerOfCite returns the element a //@ comment on line is attached to.
func ownerOfCite(owners []citeOwner, line int) (citeOwner, bool) {
	for _, o := range owners {
		for _, src := range o.sources {
			if src.Line == line {
				return o, true
			}
		}
	}
	return citeOwner{}, false
}

// formatCite writes one cite as `[repo ]path[:start[-end]]`.
func formatCite(repo, path string, start, end int) string {
	s := path
	if repo != "" {
		s = repo + " " + path
	}
	if start > 0 {
		s += ":" + strconv.Itoa(start)
		if end > start {
			s += "-" + strconv.Itoa(end)
		}
	}
	return s
}

// withText appends ` — text` when text is set.
func withText(s, text string) string {
	if text == "" {
		return s
	}
	return s + " — " + text
}

// citeHover describes the //@ comment under offset: its cites and the step
// it proves.
func (s *Server) citeHover(f *workspace.File, uri string, offset green.TextSize) (string, bool) {
	tok, ok := tokenUnder(syntax.Root(f.Green), offset)
	if !ok || !syntax.IsSourceComment(tok) {
		return "", false
	}
	cites, text, ok := lang.ParseSourceComment(tok.Text())
	if !ok {
		return "malformed source cite: it stays a plain comment (`//@ [repo] path[:lines][, …] [-- text]`)", true
	}
	lines := []string{"cites:"}
	for _, c := range cites {
		lines = append(lines, "- "+withText(formatCite(c.Repo, c.Path, c.Start, c.End), text))
	}
	line, _ := f.LineIndex.LineCol(tok.Offset())
	if o, ok := ownerOfCite(citeOwners(s.ws.Doc(), uri), line); ok {
		lines = append(lines, "attached to: "+o.desc)
	} else {
		lines = append(lines, "attached to nothing: move it under the step it proves")
	}
	return strings.Join(lines, "\n"), true
}

// citeHeadKinds are the tokens that open a step that can hold sources.
var citeHeadKinds = map[syntax.SyntaxKind]bool{
	syntax.SyntaxKindKwUseCase: true, syntax.SyntaxKindKwWhen: true,
	syntax.SyntaxKindKwAlt: true, syntax.SyntaxKindKwElse: true, syntax.SyntaxKindKwOpt: true,
	syntax.SyntaxKindKwPar: true, syntax.SyntaxKindKwAnd: true, syntax.SyntaxKindKwLoop: true,
	syntax.SyntaxKindKwNote: true, syntax.SyntaxKindKwFinding: true,
	syntax.SyntaxKindKwIncludes: true, syntax.SyntaxKindKwContinues: true,
}

// sourcesHover lists the sources of the step whose verb or opening keyword
// is under offset.
func (s *Server) sourcesHover(f *workspace.File, uri string, offset green.TextSize) (string, bool) {
	tok, ok := tokenUnder(syntax.Root(f.Green), offset)
	if !ok {
		return "", false
	}
	line, _ := f.LineIndex.LineCol(tok.Offset())
	if !citeHeadKinds[tok.Kind()] && !isActionVerb(syntax.AsFile(syntax.Root(f.Green)), f.LineIndex, tok, line) {
		return "", false
	}
	for _, o := range citeOwners(s.ws.Doc(), uri) {
		if o.line != line || len(o.sources) == 0 {
			continue
		}
		lines := []string{"sources:"}
		for _, src := range o.sources {
			lines = append(lines, "- "+withText(formatCite(src.Repo, src.Path, src.Start, src.End), src.Text))
		}
		return strings.Join(lines, "\n"), true
	}
	return "", false
}

// isActionVerb reports whether tok is the verb of the action on line.
func isActionVerb(file syntax.File, li green.LineIndex, tok syntax.SyntaxToken, line int) bool {
	found := false
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			eachStep(sc.Steps(), func(st syntax.StepDecl) {
				if a, ok := st.Action(); ok && !found && a.Line(li) == line {
					if v := a.VerbToken(); v != nil && v.Offset() == tok.Offset() {
						found = true
					}
				}
			})
		}
	}
	return found
}

// DocumentLink turns every cite of the file into a link into a local
// checkout, when the client sent initializationOptions.sourceRoots.
func (s *Server) DocumentLink(_ context.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	if params == nil || len(s.sourceRoots) == 0 {
		return []protocol.DocumentLink{}, nil
	}
	uri := string(params.TextDocument.URI)
	f := s.ws.Get(uri)
	if f == nil {
		return []protocol.DocumentLink{}, nil
	}
	doc := s.ws.Doc()
	owners := citeOwners(doc, uri)
	out := []protocol.DocumentLink{}
	for _, tok := range syntax.SourceComments(syntax.Root(f.Green)) {
		cites, _, ok := lang.ParseSourceComment(tok.Text())
		if !ok {
			continue
		}
		line, _ := f.LineIndex.LineCol(tok.Offset())
		owner, _ := ownerOfCite(owners, line)
		seen := map[int]bool{} // one link per written cite, at its first range
		for _, c := range cites {
			if seen[c.At] {
				continue
			}
			seen[c.At] = true
			dir := s.sourceDir(c.Repo, owner.subject, doc)
			if dir == "" {
				continue
			}
			target := "file://" + strings.TrimSuffix(dir, "/") + "/" + strings.TrimPrefix(c.Path, "/")
			if c.Start > 0 {
				target += fmt.Sprintf("#L%d", c.Start)
			}
			start := tok.Offset() + green.TextSize(c.At)
			out = append(out, protocol.DocumentLink{
				Range:  protocol.Range{Start: lspPosFromOffset(f.LineIndex, start), End: lspPosFromOffset(f.LineIndex, start+green.TextSize(c.Len))},
				Target: protocol.DocumentURI(target),
			})
		}
	}
	return out, nil
}

// sourceDir is the local directory of a cite's repo: the written repo, else
// the repo of the subject's service, else the service's name; looked up in
// sourceRoots, else under sourceRoots["*"]. "" when there is none.
func (s *Server) sourceDir(repo string, subject *model.Participant, doc *model.CraftDoc) string {
	if repo == "" && subject != nil && subject.Service != "" {
		repo = subject.Service
		if doc != nil {
			for _, svc := range doc.Services {
				if svc.Name == subject.Service && svc.Repo != "" {
					repo = svc.Repo
				}
			}
		}
	}
	if repo == "" {
		return ""
	}
	if dir, ok := s.sourceRoots[repo]; ok {
		return dir
	}
	if all, ok := s.sourceRoots["*"]; ok {
		return strings.TrimSuffix(all, "/") + "/" + repo
	}
	return ""
}

// documentLinkOptions advertises document links when sourceRoots is set.
func (s *Server) documentLinkOptions() *protocol.DocumentLinkOptions {
	if len(s.sourceRoots) == 0 {
		return nil
	}
	return &protocol.DocumentLinkOptions{}
}

// readSourceRoots reads initializationOptions.sourceRoots.
func readSourceRoots(opts any) map[string]string {
	m, ok := opts.(map[string]any)
	if !ok {
		return nil
	}
	roots, ok := m["sourceRoots"].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(roots))
	for k, v := range roots {
		if dir, ok := v.(string); ok && dir != "" {
			out[k] = dir
		}
	}
	return out
}
