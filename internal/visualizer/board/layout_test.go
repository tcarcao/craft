package board

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

func flowOf(nodes []view.Node, edges []view.FlowEdge) view.Flow {
	return view.Flow{Contexts: nodes, Edges: edges}
}

func ids(names ...string) []view.Node {
	var ns []view.Node
	for _, n := range names {
		ns = append(ns, view.Node{ID: n, Label: n})
	}
	return ns
}

func edge(n int, from, to string, style view.EdgeStyle, verb string) view.FlowEdge {
	return view.FlowEdge{From: from, To: to, Number: n, Style: style, Verb: verb}
}

func TestLayout_ChainStacksVertically(t *testing.T) {
	p := layout(flowOf(ids("A", "B", "C"), []view.FlowEdge{
		edge(1, "A", "B", view.StyleCall, "POST"), edge(2, "B", "C", view.StyleCall, "POST")}))
	if p["A"].X != p["B"].X || p["B"].X != p["C"].X {
		t.Errorf("X differ: %v %v %v", p["A"].X, p["B"].X, p["C"].X)
	}
	if !(p["A"].Y < p["B"].Y && p["B"].Y < p["C"].Y) {
		t.Errorf("Y not increasing: %v %v %v", p["A"].Y, p["B"].Y, p["C"].Y)
	}
}

func TestLayout_QueryTargetIsSideLookup(t *testing.T) {
	f := flowOf(ids("Inv", "Pay", "Cust"), []view.FlowEdge{
		edge(1, "Inv", "Pay", view.StyleCall, "GET"), edge(2, "Inv", "Cust", view.StyleCall, "GET"),
		edge(3, "Pay", "Inv", view.StyleReply, "")})
	p := layout(f)
	if !isLookup(f, "Pay") || !isLookup(f, "Cust") || isLookup(f, "Inv") {
		t.Fatal("lookup classification wrong")
	}
	if p["Pay"].Y != p["Inv"].Y || p["Cust"].Y != p["Inv"].Y-150 {
		t.Errorf("Y: pay %v cust %v inv %v", p["Pay"].Y, p["Cust"].Y, p["Inv"].Y)
	}
	for _, id := range []string{"Pay", "Cust"} {
		if p[id].X <= p["Inv"].X+p["Inv"].W/2 {
			t.Errorf("%s not right of Inv", id)
		}
	}
}

func TestLayout_CommandTargetIsNotLookup(t *testing.T) {
	p := layout(flowOf(ids("Inv", "Boomi"), []view.FlowEdge{edge(1, "Inv", "Boomi", view.StyleCall, "POST")}))
	if p["Boomi"].Y <= p["Inv"].Y {
		t.Error("Boomi should be below Inv")
	}
}

func TestLayout_CycleTerminates(t *testing.T) {
	p := layout(flowOf(ids("A", "B"), []view.FlowEdge{
		edge(1, "A", "B", view.StyleCall, "POST"), edge(2, "B", "A", view.StyleCall, "POST"),
		edge(3, "A", "B", view.StyleReply, ""), edge(4, "B", "A", view.StyleReply, "")}))
	if len(p) != 2 {
		t.Fatalf("placed %d", len(p))
	}
	a, b := p["A"], p["B"]
	if abs(a.X-b.X) < (a.W+b.W)/2 && abs(a.Y-b.Y) < (a.H+b.H)/2 {
		t.Error("boxes overlap")
	}
}

func TestLayout_Deterministic(t *testing.T) {
	f := flowOf(ids("A", "B", "C", "D", "L"), []view.FlowEdge{
		edge(1, "A", "B", view.StyleCall, "POST"), edge(2, "A", "C", view.StyleCall, "POST"),
		edge(3, "B", "D", view.StyleCall, "POST"), edge(4, "C", "D", view.StyleCall, "POST"),
		edge(5, "D", "L", view.StyleCall, "GET"), edge(6, "L", "D", view.StyleReply, "")})
	want := layout(f)
	for i := 0; i < 20; i++ {
		if got := layout(f); !reflect.DeepEqual(got, want) {
			t.Fatal("layout differs between runs")
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
