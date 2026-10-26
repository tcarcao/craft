package board

import (
	"math"
	"slices"
	"sort"
	"unicode/utf8"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// placed is a node with its box on the board; X, Y is the centre.
type placed struct {
	Node  view.Node
	Shape shapeKind
	X, Y  float64
	W, H  float64
	LW    float64 // width of the name drawn right of an icon (0 for a cloud)
}

// labelW estimates the width of an icon's name at the 11px label font.
// ponytail: average glyph width, no font metrics; long names in wide glyphs
// may run a little past the reserved space.
func labelW(n view.Node, s shapeKind) float64 {
	if s == shapeCloud {
		return 0
	}
	return 8 + 6.2*float64(utf8.RuneCountInString(n.Text()))
}

// bounds is the area p covers on the board: the icon and its name.
func (p *placed) bounds() (x0, y0, x1, y1 float64) {
	return p.X - p.W/2, p.Y - p.H/2, p.X + p.W/2 + p.LW, p.Y + p.H/2
}

const (
	rankGap     = 90.0  // vertical space between rows
	nodeGap     = 120.0 // horizontal space between boxes in a row
	lookupDX    = 260.0 // lookup centre, right of the rightmost main node's edge
	lookupDY    = 150.0 // vertical step between stacked lookups
	boardMargin = 60.0
)

// isLookup reports whether id is only ever queried and answers back: it has
// incoming edges, all of them query calls, and all its outgoing edges are
// replies. Lookups sit beside their caller instead of taking a row.
func isLookup(f view.Flow, id string) bool {
	in := false
	for _, e := range f.Edges {
		if e.To == id {
			in = true
			if e.Style != view.StyleCall || lang.VerbMessageKind(e.Verb) != lang.MessageQuery {
				return false
			}
		}
		if e.From == id && e.Style != view.StyleReply {
			return false
		}
	}
	return in
}

// layout positions every node of f: a layered top-down layout of the main
// flow, with lookups stacked to the right of it.
func layout(f view.Flow) map[string]*placed {
	var nodes []view.Node
	for _, g := range [][]view.Node{f.Contexts, f.Actors, f.Queues, f.Stores, f.Externals} {
		nodes = append(nodes, g...)
	}
	out := map[string]*placed{}
	for _, n := range nodes {
		if out[n.ID] != nil {
			continue
		}
		s := shapeOf(n)
		out[n.ID] = &placed{Node: n, Shape: s, W: nodeSize[s].X, H: nodeSize[s].Y, LW: labelW(n, s)}
	}
	edges := slices.Clone(f.Edges)
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Number < edges[j].Number })

	lookup := map[string]bool{}
	for id := range out {
		lookup[id] = isLookup(f, id)
	}

	// Ranking edges, breaking cycles by dropping the edge that closes one.
	// A step weighs 1 (one row down); see the zero-weight edges below.
	// ponytail: O(n·e) reachability per edge; fine for one use case.
	type rankEdge struct {
		from, to string
		w        int
	}
	var kept []view.FlowEdge
	var rk []rankEdge
	reaches := func(from, to string) bool {
		seen := map[string]bool{from: true}
		stack := []string{from}
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if c == to {
				return true
			}
			for _, e := range rk {
				if e.from == c && !seen[e.to] {
					seen[e.to] = true
					stack = append(stack, e.to)
				}
			}
		}
		return false
	}
	for _, e := range edges {
		if lookup[e.From] || lookup[e.To] || e.Style == view.StyleReply || e.Style == view.StyleSelf ||
			out[e.From] == nil || out[e.To] == nil || e.From == e.To || reaches(e.To, e.From) {
			continue
		}
		kept = append(kept, e)
		rk = append(rk, rankEdge{e.From, e.To, 1})
	}

	// A node nothing leads to (an internal-only context, a later scenario's
	// actor) would otherwise pile into the top row. It joins the row of the
	// node acting at the step before it first appears instead: a zero-weight
	// ranking edge, skipped if it would close a cycle.
	hasIn := map[string]bool{}
	for _, e := range kept {
		hasIn[e.To] = true
	}
	active := func(e view.FlowEdge) string {
		if e.Style == view.StyleTrigger {
			return e.To
		}
		return e.From
	}
	first := map[string]bool{}
	var order []string // nodes by first appearance on any step
	for i, e := range edges {
		for _, id := range []string{e.From, e.To} {
			if out[id] == nil || first[id] {
				continue
			}
			first[id] = true
			if !lookup[id] {
				order = append(order, id)
			}
			if hasIn[id] || lookup[id] || i == 0 {
				continue
			}
			if prev := active(edges[i-1]); prev != id && out[prev] != nil && !lookup[prev] && !reaches(id, prev) {
				rk = append(rk, rankEdge{prev, id, 0})
			}
		}
	}

	// Rank: longest path from a source. rk is acyclic, so len(out)
	// relaxation rounds settle it.
	rank := map[string]int{}
	for i := 0; i < len(out); i++ {
		for _, e := range rk {
			if rank[e.to] < rank[e.from]+e.w {
				rank[e.to] = rank[e.from] + e.w
			}
		}
	}

	// Initial order: first appearance on a step, then the rest by ID.
	var rest []string
	for id := range out {
		if !first[id] && !lookup[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)
	maxRank := 0
	rows := map[int][]string{}
	for _, id := range order {
		rows[rank[id]] = append(rows[rank[id]], id)
		maxRank = max(maxRank, rank[id])
	}

	// Barycentre passes: down, up, down, up.
	// ponytail: 4 fixed passes, no crossing minimisation beyond that.
	for pass := 0; pass < 4; pass++ {
		down := pass%2 == 0
		for i := 1; i <= maxRank; i++ {
			r, nr := i, i-1
			if !down {
				r, nr = maxRank-i, maxRank-i+1
			}
			idx := map[string]int{}
			for j, id := range rows[nr] {
				idx[id] = j
			}
			key := map[string]float64{}
			for j, id := range rows[r] {
				sum, n := 0.0, 0
				for _, e := range kept {
					o := e.To
					if e.To == id {
						o = e.From
					} else if e.From != id {
						continue
					}
					if k, ok := idx[o]; ok {
						sum += float64(k)
						n++
					}
				}
				key[id] = float64(j)
				if n > 0 {
					key[id] = sum / float64(n)
				}
			}
			sort.SliceStable(rows[r], func(a, b int) bool { return key[rows[r][a]] < key[rows[r][b]] })
		}
	}
	// Keep each row's members of one service side by side, at their mean
	// place, so service boxes overlap less.
	// ponytail: per row only; boxes spanning rows can still overlap.
	for r := 0; r <= maxRank; r++ {
		row := rows[r]
		sum, n := map[string]float64{}, map[string]float64{}
		for j, id := range row {
			if s := out[id].Node.Service; s != "" {
				sum[s] += float64(j)
				n[s]++
			}
		}
		key := map[string]float64{}
		for j, id := range row {
			key[id] = float64(j)
			if s := out[id].Node.Service; s != "" {
				key[id] = sum[s] / n[s]
			}
		}
		sort.SliceStable(row, func(a, b int) bool { return key[row[a]] < key[row[b]] })
	}

	// Coordinates: rows stack top-down, the icons of each centred on x = 0
	// (an icon's name hangs to its right and takes its own room in the row).
	y, right := 0.0, math.Inf(-1)
	for r := 0; r <= maxRank; r++ {
		row := rows[r]
		if len(row) == 0 {
			continue
		}
		h, w := 0.0, nodeGap*float64(len(row)-1)-out[row[len(row)-1]].LW
		for _, id := range row {
			h = max(h, out[id].H)
			w += out[id].W + out[id].LW
		}
		x := -w / 2
		for _, id := range row {
			p := out[id]
			p.X, p.Y = x+p.W/2, y+rankGap*float64(r)+h/2
			x += p.W + p.LW + nodeGap
			right = max(right, p.X+p.W/2+p.LW)
		}
		y += h
	}

	// Lookups: right of the main flow, stacked around their caller.
	callers := map[string][]string{}
	var callerIDs []string
	seenLookup := map[string]bool{}
	for _, e := range edges {
		if lookup[e.To] && !seenLookup[e.To] {
			seenLookup[e.To] = true
			if callers[e.From] == nil {
				callerIDs = append(callerIDs, e.From)
			}
			callers[e.From] = append(callers[e.From], e.To)
		}
	}
	var done []*placed
	for _, p := range out {
		if !lookup[p.Node.ID] {
			done = append(done, p)
		}
	}
	overlaps := func(p *placed) bool {
		ax0, ay0, ax1, ay1 := p.bounds()
		for _, o := range done {
			if bx0, by0, bx1, by1 := o.bounds(); ax0 < bx1 && bx0 < ax1 && ay0 < by1 && by0 < ay1 {
				return true
			}
		}
		return false
	}
	for _, c := range callerIDs {
		for i, id := range callers[c] {
			p := out[id]
			off := lookupDY * float64(i/2+i%2)
			if i%2 == 1 {
				off = -off
			}
			p.X, p.Y = right+lookupDX, out[c].Y+off
			for overlaps(p) {
				p.Y += lookupDY
			}
			done = append(done, p)
		}
	}

	// Normalise: min left and top edges at the board margin.
	minX, minY := math.Inf(1), math.Inf(1)
	for _, p := range out {
		minX, minY = min(minX, p.X-p.W/2), min(minY, p.Y-p.H/2)
	}
	for _, p := range out {
		p.X += boardMargin - minX
		p.Y += boardMargin - minY
	}
	return out
}
