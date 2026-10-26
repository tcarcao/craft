package board

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// exampleFlows is the flow of every use case of every error-free
// examples/*.craft model, keyed "model/use case".
func exampleFlows(t *testing.T) map[string]view.Flow {
	t.Helper()
	paths, _ := filepath.Glob("../../../examples/*.craft")
	if len(paths) == 0 {
		t.Fatal("no examples found")
	}
	flows := map[string]view.Flow{}
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		res := analysis.Workspace([]analysis.FileResult{analysis.File(filepath.Base(path), string(src))})
		bad := false
		for _, d := range res.Diagnostics {
			bad = bad || d.Severity == model.SeverityError
		}
		if bad {
			continue
		}
		its := interaction.Derive(res.Doc)
		for i, uc := range res.Doc.UseCases {
			shown := *res.Doc
			shown.UseCases = []model.UseCase{uc}
			var mine []interaction.Interaction
			for _, it := range its {
				if it.UseCaseIndex == i {
					it.UseCaseIndex = 0
					mine = append(mine, it)
				}
			}
			flows[filepath.Base(path)+"/"+uc.Name] = view.BuildFlow(&shown, mine)
		}
	}
	return flows
}

func overlap(ax0, ay0, ax1, ay1, bx0, by0, bx1, by1 float64) bool {
	return ax0 < bx1 && bx0 < ax1 && ay0 < by1 && by0 < ay1
}

// Every board of the examples keeps its notes off each other and off every
// node and its name (a pinned note may touch its own node), and its arrows
// off every node they do not join.
func TestExamples_BoardsReadCleanly(t *testing.T) {
	for name, f := range exampleFlows(t) {
		at := layout(f)
		r := route(f, at)
		self := map[int]string{}
		for _, e := range f.Edges {
			if e.From == e.To {
				self[e.Number] = e.From
			}
		}
		for i, n := range r.Notes {
			for _, m := range r.Notes[i+1:] {
				if overlap(n.X-noteW/2, n.Y-noteH/2, n.X+noteW/2, n.Y+noteH/2, m.X-noteW/2, m.Y-noteH/2, m.X+noteW/2, m.Y+noteH/2) {
					t.Errorf("%s: notes %d and %d overlap", name, n.Number, m.Number)
				}
			}
			for id, p := range at {
				x0, y0, x1, y1 := p.bounds()
				if self[n.Number] != id && overlap(n.X-noteW/2, n.Y-noteH/2, n.X+noteW/2, n.Y+noteH/2, x0, y0, x1, y1) {
					t.Errorf("%s: note %d overlaps node %s", name, n.Number, id)
				}
			}
		}
		for _, a := range r.Arrows {
			for id, p := range at {
				if id == a.From || id == a.To {
					continue
				}
				if x0, y0, x1, y1 := p.bounds(); hits(a, x0, y0, x1, y1) {
					t.Errorf("%s: arrow %s -> %s crosses node %s", name, a.From, a.To, id)
				}
			}
		}
	}
}
