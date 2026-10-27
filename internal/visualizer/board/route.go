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
	From, To         string // node IDs
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
	bowOffset = 24.0 // control-point shift that bows opposite arrows apart
	nudgeStep = 12.0
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
// (an event delivery shows a badge instead; its note is on the publish).
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
		switch n := at[e.From].Node; {
		case n.Kind == view.NodeActor || n.ID == interaction.Unobserved.Key:
			return noteCommand, true
		case n.ID == interaction.External.Key:
			// No producer in the model carries this event's note, so the
			// listen does.
			return noteEvent, true
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

// tangent is the (unnormalised) direction of a at t.
func tangent(a arrow, t float64) point {
	u := 1 - t
	return point{
		3*u*u*(a.C1.X-a.P0.X) + 6*u*t*(a.C2.X-a.C1.X) + 3*t*t*(a.P3.X-a.C2.X),
		3*u*u*(a.C1.Y-a.P0.Y) + 6*u*t*(a.C2.Y-a.C1.Y) + 3*t*t*(a.P3.Y-a.C2.Y),
	}
}

const arrowSamples = 40

// hits reports whether a passes through the box (x0, y0)-(x1, y1).
func hits(a arrow, x0, y0, x1, y1 float64) bool {
	for i := 1; i < arrowSamples; i++ {
		if c := bezier(a, float64(i)/arrowSamples); c.X > x0 && c.X < x1 && c.Y > y0 && c.Y < y1 {
			return true
		}
	}
	return false
}

// arrowClear is how far an arrow keeps from nodes it passes: room for half
// a note, so the notes on it do not crowd the nodes' own.
const arrowClear = 10.0 + noteW/2

// detourStep and detourLimit bound how far an arrow swings sideways round
// the nodes in its way.
const (
	detourStep  = 30.0
	detourLimit = 14
)

// curve draws the arrow from one node to another: a gentle S along the
// longer axis, bowed bowOffset to its left when the reverse arrow exists,
// and swung sideways past any other node the straight path would cross.
// ponytail: local detour per arrow, tried on both sides in growing steps; an
// arrow boxed in on both sides keeps its straight path through the nodes.
func curve(from, to *placed, bowed bool, ids []string, at map[string]*placed) arrow {
	p0 := rim(from, point{to.X, to.Y})
	p3 := rim(to, point{from.X, from.Y})
	dx, dy := p3.X-p0.X, p3.Y-p0.Y
	d := math.Hypot(dx, dy)
	base := arrow{P0: p0, P3: p3}
	if math.Abs(dx) > math.Abs(dy) {
		base.C1, base.C2 = point{p0.X + dx/2, p0.Y + dy/4}, point{p3.X - dx/2, p3.Y - dy/4}
	} else {
		base.C1, base.C2 = point{p0.X + dx/4, p0.Y + dy/2}, point{p3.X - dx/4, p3.Y - dy/2}
	}
	if d == 0 {
		return base
	}
	nx, ny := -dy/d, dx/d // left of the direction of travel
	bow := 0.0
	if bowed {
		bow = bowOffset
	}
	// A detour is a bracket: control points level with the ends, pushed
	// sideways, so the arrow leaves and enters sideways and keeps its offset
	// over most of its length.
	shift := func(off float64) arrow {
		a := base
		if off != bow {
			a.C1, a.C2 = p0, p3
		}
		a.C1 = point{a.C1.X + nx*off, a.C1.Y + ny*off}
		a.C2 = point{a.C2.X + nx*off, a.C2.Y + ny*off}
		if off != bow {
			a.P0, a.P3 = rim(from, a.C1), rim(to, a.C2)
		}
		return a
	}
	// Swing away from the first node in the way; the bow side breaks a tie.
	blocked := func(a arrow) (*placed, bool) {
		for _, id := range ids {
			o := at[id]
			if o == from || o == to {
				continue
			}
			x0, y0, x1, y1 := o.bounds()
			if hits(a, x0-arrowClear, y0-arrowClear, x1+arrowClear, y1+arrowClear) {
				return o, true
			}
		}
		return nil, false
	}
	a := shift(bow)
	o, ok := blocked(a)
	if !ok {
		return a
	}
	side := 1.0
	if c := (o.X+o.LW/2-p0.X)*nx + (o.Y-p0.Y)*ny; c > 1 {
		side = -1
	}
	for k := 1; k <= detourLimit; k++ {
		for _, s := range []float64{side, -side} {
			// A cubic's middle reaches 3/4 of its control points' offset.
			if a := shift(bow + s*float64(k)*detourStep/0.75); !func() bool { _, b := blocked(a); return b }() {
				return a
			}
		}
	}
	return shift(bow)
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
	ids := make([]string, 0, len(at))
	for id := range at {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Arrows and badges first, so notes can keep off every arrow.
	var out routed
	arrowOf := map[pair]int{}
	for _, p := range pairs {
		if p.from == p.to {
			continue
		}
		es := groups[p]
		a := curve(at[p.from], at[p.to], len(groups[pair{p.to, p.from}]) > 0, ids, at)
		a.Dashed, a.From, a.To = true, p.from, p.to
		for _, e := range es {
			a.Dashed = a.Dashed && e.Style == view.StyleReply
			a.Inferred = a.Inferred || e.Inferred
		}
		arrowOf[p] = len(out.Arrows)
		out.Arrows = append(out.Arrows, a)
		badges := 0
		for _, e := range es {
			if _, ok := noteOf(at, e); !ok {
				c := bezier(a, 0.5+0.1*float64(badges))
				out.Badges = append(out.Badges, badge{c.X, c.Y, e.Number})
				badges++
			}
		}
	}

	var owners []pair // owners[i] is the edge ends of out.Notes[i]
	// free reports whether a note centred at c overlaps no earlier note,
	// badge or node (except skip, the node a pinned note belongs to), and,
	// when strict, no arrow but its own (own's).
	free := func(c point, skip string, own pair, strict bool) bool {
		for _, o := range out.Notes {
			if math.Abs(c.X-o.X) < noteW+10 && math.Abs(c.Y-o.Y) < noteH+14 {
				return false
			}
		}
		for _, b := range out.Badges {
			if math.Abs(c.X-b.X) < noteW/2+10 && math.Abs(c.Y-b.Y) < noteH/2+10 {
				return false
			}
		}
		for _, id := range ids {
			if id == skip {
				continue
			}
			x0, y0, x1, y1 := at[id].bounds()
			if c.X+noteW/2+6 > x0 && c.X-noteW/2-6 < x1 && c.Y+noteH/2+6 > y0 && c.Y-noteH/2-6 < y1 {
				return false
			}
		}
		if strict {
			for _, a := range out.Arrows {
				if a.From == own.from && a.To == own.to {
					continue
				}
				if hits(a, c.X-noteW/2-4, c.Y-noteH/2-4, c.X+noteW/2+4, c.Y+noteH/2+4) {
					return false
				}
			}
		}
		return true
	}
	// place puts n at the first candidate spot that is free of arrows too,
	// else the first free one, else walks on from the first candidate in
	// (sx, sy) until free; with no step it stays on the first candidate.
	// ponytail: greedy first fit in edge order, no global overlap solve.
	place := func(n note, cands []point, sx, sy float64, skip string, p pair) {
		found := false
		for _, strict := range []bool{true, false} {
			for _, c := range cands {
				if free(c, skip, p, strict) {
					n.X, n.Y, found = c.X, c.Y, true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			n.X, n.Y = cands[0].X, cands[0].Y
			for i := 0; i < 200 && (sx != 0 || sy != 0) && !free(point{n.X, n.Y}, skip, p, false); i++ {
				n.X, n.Y = n.X+sx, n.Y+sy
			}
		}
		out.Notes = append(out.Notes, n)
		owners = append(owners, p)
	}

	for _, p := range pairs {
		es := groups[p]
		from := at[p.from]
		if p.from == p.to {
			// Pinned beside the node: a cloud's note overlaps its rim, an
			// icon's sits clear of it on the left (its name is on the right).
			x0, x1 := from.X-from.W/2-noteW/2+16, from.X+from.W/2+noteW/2-16
			y := from.Y + from.H/4
			if from.Shape != shapeCloud {
				x0, x1, y = from.X-from.W/2-noteW/2-10, from.X+from.W/2+from.LW+noteW/2+10, from.Y
			}
			var cands []point
			for _, k := range []float64{0, -1, 1, -2, 2} {
				for _, x := range []float64{x0, x1} {
					cands = append(cands, point{x, y + k*(noteH+16)})
				}
			}
			for _, e := range es {
				place(note{Kind: noteNeutral, Number: e.Number, Label: e.Label, Inferred: e.Inferred},
					cands, 0, nudgeStep, p.from, p)
			}
			continue
		}

		// Notes sit on their arrow, Miro-style, the line running under them;
		// a crowded one slides along it, never off it.
		a := out.Arrows[arrowOf[p]]
		// Several notes spread evenly along the arrow in step order; a
		// request and its reply keep to opposite halves. A note that does
		// not fit at its spot takes the nearest free one along the arrow.
		var noted []view.FlowEdge
		for _, e := range es {
			if _, ok := noteOf(at, e); ok {
				noted = append(noted, e)
			}
		}
		span := 1.0
		if len(groups[pair{p.to, p.from}]) > 0 {
			span = 0.54
		}
		for i, e := range noted {
			k, _ := noteOf(at, e)
			start := span * float64(i+1) / float64(len(noted)+1)
			var ts []float64
			for t := 0.1; t <= 0.9; t += 0.02 {
				ts = append(ts, t)
			}
			sort.SliceStable(ts, func(i, j int) bool { return math.Abs(ts[i]-start) < math.Abs(ts[j]-start) })
			var cands []point
			for _, t := range ts {
				cands = append(cands, bezier(a, t))
			}
			place(note{Kind: k, Number: e.Number, Label: e.Label, Inferred: e.Inferred}, cands, 0, 0, "", p)
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
