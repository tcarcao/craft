package board

import (
	"math"
	"sort"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// arrow is a cubic Bézier between two node rims.
type arrow struct {
	P0, C1, C2, P3   point
	Dashed, Inferred bool
}

// note is a sticky note on the board; X, Y is its centre. Number 0 draws no
// number badge.
type note struct {
	X, Y     float64
	Kind     noteKind
	Number   int
	Label    string
	Inferred bool
}

// badge is a bare step number riding on an arrow.
type badge struct {
	X, Y   float64
	Number int
}

// box is a service outline, already padded.
type box struct{ X0, Y0, X1, Y1 float64 }

// routed is everything drawn between and around the nodes.
type routed struct {
	Arrows []arrow
	Notes  []note
	Badges []badge
	Boxes  []box
}

const (
	bowOffset  = 24.0 // control-point shift that bows opposite arrows apart
	nudgeStep  = 12.0
	nudgeLimit = 40
)

// rim is where an arrow leaves p towards a point: the cloud outline for a
// cloud, a circle just outside the node for everything else.
func rim(p *placed, toward point) point {
	dx, dy := toward.X-p.X, toward.Y-p.Y
	d := math.Hypot(dx, dy)
	if d == 0 {
		return point{p.X, p.Y}
	}
	ux, uy := dx/d, dy/d
	if p.Shape != shapeCloud {
		r := math.Max(p.W, p.H) / 2 * 1.15
		return point{p.X + ux*r, p.Y + uy*r}
	}
	// Farthest crossing of the ray from the centre with the outline.
	pts := cloudOutline(p.W, p.H)
	far := 0.0
	for i, a := range pts {
		b := pts[(i+1)%len(pts)]
		ex, ey := b.X-a.X, b.Y-a.Y
		den := ux*ey - uy*ex
		if den == 0 {
			continue
		}
		t := (a.X*ey - a.Y*ex) / den
		s := (a.X*uy - a.Y*ux) / den
		if t > far && s >= 0 && s <= 1 {
			far = t
		}
	}
	far++
	return point{p.X + ux*far, p.Y + uy*far}
}

// noteOf says which note an edge carries; ok is false when it carries none
// (an event delivery shows a badge instead).
func noteOf(at map[string]*placed, e view.FlowEdge) (k noteKind, ok bool) {
	switch e.Style {
	case view.StyleCall:
		switch lang.VerbMessageKind(e.Verb) {
		case lang.MessageQuery:
			return noteQuery, true
		case lang.MessageCommand:
			return noteCommand, true
		}
		return noteNeutral, true
	case view.StyleAsync:
		if at[e.To].Node.Kind == view.NodeQueue {
			return noteEvent, true
		}
		return noteNeutral, true
	case view.StyleTrigger:
		if n := at[e.From].Node; n.Kind == view.NodeActor || n.ID == interaction.Unobserved.Key {
			return noteCommand, true
		}
		return 0, false
	}
	return noteNeutral, true // reply, self
}

func bezier(a arrow, t float64) point {
	u := 1 - t
	return point{
		u*u*u*a.P0.X + 3*u*u*t*a.C1.X + 3*u*t*t*a.C2.X + t*t*t*a.P3.X,
		u*u*u*a.P0.Y + 3*u*u*t*a.C1.Y + 3*u*t*t*a.C2.Y + t*t*t*a.P3.Y,
	}
}

// route lays out the arrows, notes, badges and service boxes of f around the
// placed nodes.
func route(f view.Flow, at map[string]*placed) routed {
	type pair struct{ from, to string }
	groups := map[pair][]view.FlowEdge{}
	for _, e := range f.Edges {
		if at[e.From] != nil && at[e.To] != nil {
			groups[pair{e.From, e.To}] = append(groups[pair{e.From, e.To}], e)
		}
	}
	var pairs []pair
	for k, es := range groups {
		sort.SliceStable(es, func(i, j int) bool { return es[i].Number < es[j].Number })
		pairs = append(pairs, k)
	}
	sort.Slice(pairs, func(i, j int) bool { return groups[pairs[i]][0].Number < groups[pairs[j]][0].Number })

	var out routed
	var owners []pair // owners[i] is the edge ends of out.Notes[i]
	pinned := map[string]int{}

	// place nudges n step by step in (sx, sy) until clear of earlier notes and
	// of nodes (except skip, the node a pinned note belongs to).
	// ponytail: greedy nudge, no global overlap solve.
	place := func(n note, sx, sy float64, skip string, p pair, edge view.FlowEdge) {
		clear := func() bool {
			for _, o := range out.Notes {
				if math.Abs(n.X-o.X) < noteW+10 && math.Abs(n.Y-o.Y) < noteH+14 {
					return false
				}
			}
			ids := make([]string, 0, len(at))
			for id := range at {
				ids = append(ids, id)
			}
			for _, id := range ids {
				if o := at[id]; id != skip && math.Abs(n.X-o.X) < (o.W+noteW)/2+6 && math.Abs(n.Y-o.Y) < (o.H+noteH)/2+6 {
					return false
				}
			}
			return true
		}
		for i := 0; i < nudgeLimit && !clear(); i++ {
			n.X, n.Y = n.X+sx, n.Y+sy
		}
		out.Notes = append(out.Notes, n)
		owners = append(owners, p)
	}

	for _, p := range pairs {
		es := groups[p]
		from, to := at[p.from], at[p.to]
		if p.from == p.to {
			for _, e := range es {
				k := pinned[p.from]
				pinned[p.from]++
				n := note{X: from.X - from.W/2 - noteW/2 + 16, Y: from.Y + from.H/4 + float64(k)*nudgeStep,
					Kind: noteNeutral, Number: e.Number, Label: e.Label, Inferred: e.Inferred}
				place(n, 0, nudgeStep, p.from, p, e)
			}
			continue
		}

		p0 := rim(from, point{to.X, to.Y})
		p3 := rim(to, point{from.X, from.Y})
		dx, dy := p3.X-p0.X, p3.Y-p0.Y
		flat := math.Abs(dx) > math.Abs(dy)
		a := arrow{P0: p0, P3: p3, Dashed: true}
		if flat {
			a.C1, a.C2 = point{p0.X + dx/2, p0.Y + dy/4}, point{p3.X - dx/2, p3.Y - dy/4}
		} else {
			a.C1, a.C2 = point{p0.X + dx/4, p0.Y + dy/2}, point{p3.X - dx/4, p3.Y - dy/2}
		}
		if len(groups[pair{p.to, p.from}]) > 0 {
			if d := math.Hypot(dx, dy); d > 0 {
				ox, oy := -dy/d*bowOffset, dx/d*bowOffset
				a.C1, a.C2 = point{a.C1.X + ox, a.C1.Y + oy}, point{a.C2.X + ox, a.C2.Y + oy}
			}
		}
		for _, e := range es {
			a.Dashed = a.Dashed && e.Style == view.StyleReply
			a.Inferred = a.Inferred || e.Inferred
		}
		out.Arrows = append(out.Arrows, a)

		var noted []view.FlowEdge
		badges := 0
		for _, e := range es {
			if _, ok := noteOf(at, e); ok {
				noted = append(noted, e)
				continue
			}
			c := bezier(a, 0.5+0.1*float64(badges))
			out.Badges = append(out.Badges, badge{c.X, c.Y, e.Number})
			badges++
		}

		t := 0.72
		if flat {
			t = 0.5
		}
		ax, ay := p0.X+dx*t, p0.Y+dy*t
		for i, e := range noted {
			k, _ := noteOf(at, e)
			n := note{Kind: k, Number: e.Number, Label: e.Label, Inferred: e.Inferred}
			if flat {
				side := 1.0
				if p3.Y < p0.Y-20 {
					side = -1
				}
				n.X = ax + (float64(i)-float64(len(noted)-1)/2)*(noteW+14)
				n.Y = ay + side*(noteH/2+18+math.Abs(dy)*0.25)
				place(n, 0, side*nudgeStep, "", p, e)
			} else {
				side := 1.0
				if i%2 == 1 {
					side = -1
				}
				n.X, n.Y = ax+side*(noteW/2+18), ay
				place(n, side*nudgeStep, 0, "", p, e)
			}
		}
	}

	// Service boxes: two or more nodes of one service, and the notes between
	// them. ponytail: a note nudged outside its service still stretches the box.
	var services []string
	members := map[string][]*placed{}
	for _, p := range at {
		if s := p.Node.Service; s != "" {
			if members[s] == nil {
				services = append(services, s)
			}
			members[s] = append(members[s], p)
		}
	}
	sort.Strings(services)
	for _, s := range services {
		if len(members[s]) < 2 {
			continue
		}
		b := box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		grow := func(x, y, w, h float64) {
			b.X0, b.Y0 = min(b.X0, x-w/2), min(b.Y0, y-h/2)
			b.X1, b.Y1 = max(b.X1, x+w/2), max(b.Y1, y+h/2)
		}
		for _, p := range members[s] {
			grow(p.X, p.Y, p.W, p.H)
		}
		for i, n := range out.Notes {
			if at[owners[i].from].Node.Service == s && at[owners[i].to].Node.Service == s {
				grow(n.X, n.Y, noteW, noteH)
			}
		}
		out.Boxes = append(out.Boxes, box{b.X0 - 24, b.Y0 - 18, b.X1 + 24, b.Y1 + 18})
	}
	return out
}
