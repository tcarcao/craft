package board

import (
	"math"
	"testing"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

func segDist(p, a, b point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	t := math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}

func TestRim_OnCloudOutline(t *testing.T) {
	c := &placed{Shape: shapeCloud, W: 190, H: 110}
	pts := cloudOutline(c.W, c.H)
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		r := rim(c, point{500 * math.Cos(ang), 500 * math.Sin(ang)})
		best := math.Inf(1)
		for j, a := range pts {
			best = math.Min(best, segDist(r, a, pts[(j+1)%len(pts)]))
		}
		if best > 1.5 {
			t.Errorf("direction %d: rim %v is %.2f from the outline", i, r, best)
		}
	}
}

func routeOf(nodes []view.Node, edges ...view.FlowEdge) (routed, map[string]*placed) {
	f := flowOf(nodes, edges)
	at := layout(f)
	return route(f, at), at
}

func TestRoute_ListenGetsBadgeNotNote(t *testing.T) {
	f := view.Flow{
		Contexts: ids("Inv"), Queues: []view.Node{{ID: "Q", Kind: view.NodeQueue}},
		Edges: []view.FlowEdge{edge(2, "Q", "Inv", view.StyleTrigger, "")},
	}
	r := route(f, layout(f))
	if len(r.Badges) != 1 || r.Badges[0].Number != 2 || len(r.Notes) != 0 {
		t.Errorf("got badges %v notes %v", r.Badges, r.Notes)
	}
}

func TestRoute_NoteKinds(t *testing.T) {
	f := view.Flow{
		Contexts: ids("A", "B", "C", "D"),
		Actors:   []view.Node{{ID: "U", Kind: view.NodeActor}},
		Queues:   []view.Node{{ID: "Q", Kind: view.NodeQueue}},
		Edges: []view.FlowEdge{
			edge(1, "A", "B", view.StyleCall, "GET"), edge(2, "A", "C", view.StyleCall, "POST"),
			edge(3, "A", "D", view.StyleCall, "GRPC"), edge(4, "A", "Q", view.StyleAsync, ""),
			edge(5, "B", "A", view.StyleReply, ""), edge(6, "A", "A", view.StyleSelf, ""),
			edge(7, "U", "A", view.StyleTrigger, ""),
		},
	}
	want := map[int]noteKind{1: noteQuery, 2: noteCommand, 3: noteNeutral, 4: noteEvent, 5: noteNeutral, 6: noteNeutral, 7: noteCommand}
	r := route(f, layout(f))
	if len(r.Notes) != len(want) {
		t.Fatalf("got %d notes, want %d", len(r.Notes), len(want))
	}
	for _, n := range r.Notes {
		if want[n.Number] != n.Kind {
			t.Errorf("note %d kind = %v, want %v", n.Number, n.Kind, want[n.Number])
		}
	}
}

func TestRoute_UnobservedTriggerIsCommand(t *testing.T) {
	f := view.Flow{Contexts: ids("A"), Externals: []view.Node{{ID: interaction.Unobserved.Key, Kind: view.NodeExternal}},
		Edges: []view.FlowEdge{edge(1, interaction.Unobserved.Key, "A", view.StyleTrigger, "")}}
	r := route(f, layout(f))
	if len(r.Notes) != 1 || r.Notes[0].Kind != noteCommand {
		t.Errorf("got %v", r.Notes)
	}
}

func TestRoute_NotesDoNotOverlap(t *testing.T) {
	f := view.Flow{
		Contexts:  ids("Order", "Inv", "Pay", "Cust"),
		Queues:    []view.Node{{ID: "Q", Kind: view.NodeQueue}},
		Externals: []view.Node{{ID: "Boomi", Kind: view.NodeExternal}},
		Edges: []view.FlowEdge{
			edge(1, "Order", "Q", view.StyleAsync, ""), edge(2, "Q", "Inv", view.StyleTrigger, ""),
			edge(3, "Inv", "Pay", view.StyleCall, "GET"), edge(4, "Inv", "Cust", view.StyleCall, "GET"),
			edge(5, "Inv", "Boomi", view.StyleCall, "POST"), edge(6, "Inv", "Boomi", view.StyleCall, "POST"),
			edge(7, "Inv", "Inv", view.StyleSelf, ""),
		},
	}
	at := layout(f)
	r := route(f, at)
	if len(r.Notes) != 6 {
		t.Fatalf("got %d notes", len(r.Notes))
	}
	for i, a := range r.Notes {
		for _, b := range r.Notes[i+1:] {
			if math.Abs(a.X-b.X) < noteW && math.Abs(a.Y-b.Y) < noteH {
				t.Errorf("notes %d and %d overlap", a.Number, b.Number)
			}
		}
		for id, p := range at {
			if a.Number == 7 && id == "Inv" {
				continue
			}
			if math.Abs(a.X-p.X) < (p.W+noteW)/2 && math.Abs(a.Y-p.Y) < (p.H+noteH)/2 {
				t.Errorf("note %d overlaps node %s", a.Number, id)
			}
		}
	}
}

func TestRoute_OppositeArrowsBowApart(t *testing.T) {
	r, _ := routeOf(ids("A", "B"), edge(1, "A", "B", view.StyleCall, "POST"), edge(2, "B", "A", view.StyleReply, ""))
	if len(r.Arrows) != 2 {
		t.Fatalf("got %d arrows", len(r.Arrows))
	}
	a, b := r.Arrows[0], r.Arrows[1]
	if d := math.Hypot(a.C1.X-b.C2.X, a.C1.Y-b.C2.Y); d < 40 {
		t.Errorf("control points only %.1f apart", d)
	}
	if a.Dashed || !b.Dashed {
		t.Errorf("dashed = %v, %v", a.Dashed, b.Dashed)
	}
}

func TestRoute_ServiceBoxOnlyForTwoOrMore(t *testing.T) {
	nodes := ids("A", "B", "C")
	nodes[0].Service, nodes[1].Service, nodes[2].Service = "S", "S", "T"
	r, at := routeOf(nodes, edge(1, "A", "B", view.StyleCall, "POST"), edge(2, "B", "C", view.StyleCall, "POST"))
	if len(r.Boxes) != 1 {
		t.Fatalf("got %d boxes", len(r.Boxes))
	}
	b := r.Boxes[0]
	for _, id := range []string{"A", "B"} {
		p := at[id]
		if p.X-p.W/2 < b.X0 || p.X+p.W/2 > b.X1 || p.Y-p.H/2 < b.Y0 || p.Y+p.H/2 > b.Y1 {
			t.Errorf("box %v does not contain %s", b, id)
		}
	}
}
