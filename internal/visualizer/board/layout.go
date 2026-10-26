package board

import (
	"math"
	"slices"
	"sort"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// placed is a node with its box on the board; X, Y is the centre.
type placed struct {
	Node  view.Node
	Shape shapeKind
	X, Y  float64
	W, H  float64
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
		out[n.ID] = &placed{Node: n, Shape: s, W: nodeSize[s].X, H: nodeSize[s].Y}
	}
	edges := slices.Clone(f.Edges)
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Number < edges[j].Number })

	lookup := map[string]bool{}
	for id := range out {
		lookup[id] = isLookup(f, id)
	}

	// Ranking edges, breaking cycles by dropping the edge that closes one.
	// ponytail: O(n·e) reachability per edge; fine for one use case.
	var kept []view.FlowEdge
	reaches := func(from, to string) bool {
		seen := map[string]bool{from: true}
		stack := []string{from}
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if c == to {
				return true
			}
			for _, e := range kept {
				if e.From == c && !seen[e.To] {
					seen[e.To] = true
					stack = append(stack, e.To)
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
	}

	// Rank: longest path from a source. kept is acyclic, so len(out)
	// relaxation rounds settle it.
	rank := map[string]int{}
	for i := 0; i < len(out); i++ {
		for _, e := range kept {
			if rank[e.To] < rank[e.From]+1 {
				rank[e.To] = rank[e.From] + 1
			}
		}
	}

	// Initial order: first appearance on a kept edge, then the rest by ID.
	var order []string
	seen := map[string]bool{}
	for _, e := range kept {
		for _, id := range []string{e.From, e.To} {
			if !seen[id] {
				seen[id] = true
				order = append(order, id)
			}
		}
	}
	var rest []string
	for id := range out {
		if !seen[id] && !lookup[id] {
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

	// Coordinates: rows stack top-down, each centred on x = 0.
	y, right := 0.0, math.Inf(-1)
	for r := 0; r <= maxRank; r++ {
		h, w := 0.0, nodeGap*float64(len(rows[r])-1)
		for _, id := range rows[r] {
			h = max(h, out[id].H)
			w += out[id].W
		}
		x := -w / 2
		for _, id := range rows[r] {
			p := out[id]
			p.X, p.Y = x+p.W/2, y+rankGap*float64(r)+h/2
			x += p.W + nodeGap
			right = max(right, p.X+p.W/2)
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
		for _, o := range done {
			if math.Abs(p.X-o.X) < (p.W+o.W)/2 && math.Abs(p.Y-o.Y) < (p.H+o.H)/2 {
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
