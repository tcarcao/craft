package board

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// A small board with every optional legend row still fits the legend.
func TestFlow_CanvasFitsLegend(t *testing.T) {
	f := view.Flow{Contexts: ids("A"), Stores: []view.Node{{ID: "db", Kind: view.NodeStore}},
		Edges: []view.FlowEdge{edge(1, "A", "db", view.StyleCall, ""), edge(2, "A", "A", view.StyleSelf, "")}}
	m := regexp.MustCompile(`height="([0-9.]+)"`).FindSubmatch(Flow(f, "T"))
	h, _ := strconv.ParseFloat(string(m[1]), 64)
	if want := legendY + legendHeight(true, true) + pad; h < want {
		t.Errorf("canvas height %v < legend bottom %v", h, want)
	}
}
