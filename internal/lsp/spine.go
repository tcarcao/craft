package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// Journey spines in the editor (spec §17.5): hover on a `journey` tag value
// lists the members in spine order, definition goes to the other members,
// and a code lens counts them. (Named spine.go so it is not confused with
// v2.23's journeyHover, which is about blocks, surfaces and stores.)

// journeyTag is the first value of a `journey` tag: the slug and its range.
type journeyTag struct {
	slug    string
	useCase string // the use case the tag belongs to
	rng     green.TextRange
}

// journeyTags lists every `journey` tag of file.
func journeyTags(file syntax.File) []journeyTag {
	var out []journeyTag
	for _, uc := range file.UseCases() {
		for _, tb := range uc.TagsBlocks() {
			for _, tag := range tb.Tags() {
				k := tag.Key()
				vals, rngs := tag.Values(), tag.ValueRanges()
				if k == nil || k.Text() != model.TagJourney || len(vals) == 0 || len(rngs) == 0 {
					continue
				}
				out = append(out, journeyTag{slug: strings.TrimSpace(vals[0]), useCase: uc.Name(), rng: rngs[0]})
			}
		}
	}
	return out
}

// journeyTagAt returns the journey tag whose first value holds offset.
func journeyTagAt(file syntax.File, offset green.TextSize) (journeyTag, bool) {
	for _, t := range journeyTags(file) {
		if offset >= t.rng.Start && offset <= t.rng.End {
			return t, true
		}
	}
	return journeyTag{}, false
}

// spineHover lists the members of the journey under offset in spine order,
// each with its file.
func (s *Server) spineHover(f *workspace.File, offset green.TextSize) (string, bool) {
	t, ok := journeyTagAt(syntax.AsFile(syntax.Root(f.Green)), offset)
	if !ok || t.slug == "" {
		return "", false
	}
	j := journey.Build(s.ws.Doc(), t.slug)
	if len(j.Members) == 0 {
		return "", false
	}
	lines := []string{fmt.Sprintf("journey %s: %s", t.slug, useCaseCount(len(j.Members)))}
	for i, m := range j.Members {
		lines = append(lines, fmt.Sprintf("%d. %s (%s)", i+1, m.Name, s.displayPath(m.SourceURI)))
	}
	if j.Cycle {
		lines = append(lines, "cycle")
	}
	return strings.Join(lines, "\n"), true
}

// useCaseCount is "1 use case" or "<n> use cases".
func useCaseCount(n int) string {
	if n == 1 {
		return "1 use case"
	}
	return fmt.Sprintf("%d use cases", n)
}

// spineDefinition goes from a journey tag value to the other members, in
// spine order.
func (s *Server) spineDefinition(f *workspace.File, uri string, offset green.TextSize) ([]protocol.Location, bool) {
	t, ok := journeyTagAt(syntax.AsFile(syntax.Root(f.Green)), offset)
	if !ok || t.slug == "" {
		return nil, false
	}
	var out []protocol.Location
	for _, m := range journey.Build(s.ws.Doc(), t.slug).Members {
		if m.Name == t.useCase && m.SourceURI == uri {
			continue
		}
		if loc, ok := s.useCaseLocation(model.UseCaseRef{Name: m.Name, SourceURI: m.SourceURI}); ok {
			out = append(out, loc)
		}
	}
	return out, true
}

// CodeLens puts "N use cases in this journey" on every journey tag.
func (s *Server) CodeLens(_ context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil {
		return nil, nil
	}
	out := []protocol.CodeLens{}
	counts := map[string]int{}
	for _, t := range journeyTags(syntax.AsFile(syntax.Root(f.Green))) {
		if t.slug == "" {
			continue
		}
		n, ok := counts[t.slug]
		if !ok {
			n = len(journey.Build(s.ws.Doc(), t.slug).Members)
			counts[t.slug] = n
		}
		out = append(out, protocol.CodeLens{
			Range:   protocol.Range{Start: lspPosFromOffset(f.LineIndex, t.rng.Start), End: lspPosFromOffset(f.LineIndex, t.rng.End)},
			Command: &protocol.Command{Title: useCaseCount(n) + " in this journey"},
		})
	}
	return out, nil
}
