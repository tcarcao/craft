// Package journey builds a journey spine: the use cases that share a
// `journey` tag, ordered by the hand-offs between them: events, store
// hand-offs, calls to a member's receives, continues and includes links
// (spec §17.2).
package journey

import (
	"cmp"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Slug is a use case's journey: the first value of its journey tag.
func Slug(uc model.UseCase) string {
	if vals := uc.TagValues[model.TagJourney]; len(vals) > 0 {
		return strings.TrimSpace(vals[0])
	}
	return ""
}

// Slugs returns every journey slug of doc, sorted.
func Slugs(doc *model.CraftDoc) []string {
	if doc == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, uc := range doc.UseCases {
		if s := Slug(uc); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// Build returns the spine of slug: every member, the edges between members,
// and a topological order over the edges (Kahn; see topoOrder). Connected
// members come first: among ready ones, a successor of the member placed
// last, then the namesake (the member the journey is named after), then
// (source file, line). When none is ready (a cycle), the cycle is entered
// at a member on it where a placed member hands off into it, else at the
// namesake, else by (source file, line), and Cycle is set; a member
// downstream of the cycle waits for it. Members with no hand-off to or from
// another member come last, namesake first.
func Build(doc *model.CraftDoc, slug string) model.Journey {
	j := model.Journey{Slug: slug}
	if doc == nil || slug == "" {
		return j
	}
	var members []int // doc indices, in doc order
	for i, uc := range doc.UseCases {
		if Slug(uc) == slug {
			members = append(members, i)
		}
	}
	edges := edgesBetween(doc, members) // From/To index members
	order, cycle := topoOrder(doc, slug, members, edges)
	pos := make(map[int]int, len(order))
	for p, m := range order {
		pos[m] = p
		uc := doc.UseCases[members[m]]
		j.Members = append(j.Members, model.JourneyMember{UseCase: members[m], Name: uc.Name, SourceURI: uc.SourceURI})
	}
	for _, e := range edges {
		e.From, e.To = pos[e.From], pos[e.To]
		j.Edges = append(j.Edges, e)
	}
	slices.SortStableFunc(j.Edges, func(x, y model.JourneyEdge) int {
		return cmp.Or(cmp.Compare(x.From, y.From), cmp.Compare(x.To, y.To),
			cmp.Compare(x.Kind, y.Kind), cmp.Compare(x.Event, y.Event))
	})
	j.Cycle = cycle
	return j
}

// handoffs is what one use case hands on and listens for.
type handoffs struct {
	publishes map[string]bool // events (and store refs) of its async actions
	stores    map[string]bool // the subset written to a store
	listens   map[string]bool // events of its listens triggers
	readStore map[string]bool // the subset read from a store
	continues []model.UseCaseRef
	includes  []model.UseCaseRef
	calls     []call        // the operations of its asks, in source order
	receives  map[call]bool // the operations its receives triggers answer
}

// call is one operation asked of, or received by, a context: the context's
// resolved ID (or its name as written) and one operation key.
type call struct{ context, op string }

// contextKey is what a call and a receives compare their context by: the
// resolved participant, else the name as written.
func contextKey(written string, p *model.Participant) string {
	if p != nil {
		return p.ID
	}
	return written
}

// collect gathers uc's hand-offs, pairing as interaction.Derive does: an
// async action with a subject publishes each event of its list; a listens
// trigger with a subject listens to each item of its list.
func collect(uc model.UseCase) handoffs {
	h := handoffs{publishes: map[string]bool{}, stores: map[string]bool{}, listens: map[string]bool{}, readStore: map[string]bool{}, receives: map[call]bool{}}
	for _, sc := range uc.Scenarios {
		t := sc.Trigger
		if t.Type == lang.TriggerReceive && t.Context != "" && t.Operation != nil {
			for _, op := range lang.OperationKeys(t.Operation.Text) {
				h.receives[call{contextKey(t.Context, t.Subject), op}] = true
			}
		}
		if t.Type == lang.TriggerDomainListen && t.Context != "" {
			for _, ev := range t.EventList() {
				if ev != "" {
					h.listens[ev] = true
					if t.Store != nil {
						h.readStore[ev] = true
					}
				}
			}
		}
		for _, a := range sc.Actions() {
			if a.Type == lang.ActionSync && a.TargetContext != "" && a.Operation != nil {
				for _, op := range lang.OperationKeys(a.Operation.Text) {
					h.calls = append(h.calls, call{contextKey(a.TargetContext, a.To), op})
				}
			}
			if a.Type != lang.ActionAsync || a.Context == "" {
				continue
			}
			for _, ev := range a.EventList() {
				if ev != "" {
					h.publishes[ev] = true
					if a.Store != nil {
						h.stores[ev] = true
					}
				}
			}
		}
		sc.EachLink(func(l *model.Link) {
			switch {
			case l.Target == nil:
			case l.Kind == model.LinkContinues:
				h.continues = append(h.continues, *l.Target)
			case l.Kind == model.LinkIncludes:
				h.includes = append(h.includes, *l.Target)
			}
		})
	}
	return h
}

// edgesBetween lists the edges between members (From/To index members), in
// member order, deduplicated.
func edgesBetween(doc *model.CraftDoc, members []int) []model.JourneyEdge {
	hs := make([]handoffs, len(members))
	for k, m := range members {
		hs[k] = collect(doc.UseCases[m])
	}
	seen := map[model.JourneyEdge]bool{}
	var edges []model.JourneyEdge
	add := func(e model.JourneyEdge) {
		if !seen[e] {
			seen[e] = true
			edges = append(edges, e)
		}
	}
	for a := range members {
		events := make([]string, 0, len(hs[a].publishes))
		for ev := range hs[a].publishes {
			events = append(events, ev)
		}
		sort.Strings(events)
		for b := range members {
			if a == b {
				continue
			}
			to := doc.UseCases[members[b]]
			for _, r := range hs[a].continues {
				if r.Name == to.Name && r.SourceURI == to.SourceURI {
					add(model.JourneyEdge{From: a, To: b, Kind: model.JourneyEdgeContinues})
				}
			}
			for _, c := range hs[a].calls {
				if hs[b].receives[c] {
					add(model.JourneyEdge{From: a, To: b, Kind: model.JourneyEdgeCall, Event: c.op})
				}
			}
			for _, ev := range events {
				if !hs[b].listens[ev] {
					continue
				}
				kind := model.JourneyEdgeEvent
				if hs[a].stores[ev] || hs[b].readStore[ev] {
					kind = model.JourneyEdgeStore
				}
				add(model.JourneyEdge{From: a, To: b, Kind: kind, Event: ev})
			}
		}
	}
	return edges
}

// topoOrder orders members (indices into members) by Kahn's algorithm over
// edges (spec §17.2). Members connected by a hand-off come first, then the
// unconnected ones. The next member is chosen by tier, then preference:
//
//	tier 0: a connected member with no unplaced predecessor (ready);
//	tier 1: a connected member on a cycle that a placed member hands off to
//	        (the cycle is entered where the flow reaches it);
//	tier 2: any other connected member on a cycle (a cycle nothing hands
//	        into);
//	tier 3: a connected member that still waits for a member off its cycle
//	        (downstream of a cycle; never chosen while a tier 1 or 2 member
//	        is left);
//	tier 4: an unconnected member.
//
// "On a cycle" here means: every unplaced member that can reach the member
// can also be reached back from it, that is, the member sits in a strongly
// connected component of the unplaced members that no unplaced member
// outside it feeds. So nothing upstream of its cycle, including another
// cycle, is still waiting to be placed.
//
// Within a tier: a successor of the member placed last (so independent
// chains do not interleave), then the namesake (see namesake), then the
// smallest (source file, line). Placing a member of tier 1 or 2 reports a
// cycle.
func topoOrder(doc *model.CraftDoc, slug string, members []int, edges []model.JourneyEdge) (order []int, cycle bool) {
	n := len(members)
	indeg := make([]int, n)
	succ := make([][]int, n)
	connected := make([]bool, n)
	pred := make([][]int, n)
	seen := map[[2]int]bool{}
	for _, e := range edges {
		connected[e.From], connected[e.To] = true, true
		if p := [2]int{e.From, e.To}; !seen[p] {
			seen[p] = true
			indeg[e.To]++
			succ[e.From] = append(succ[e.From], e.To)
			pred[e.To] = append(pred[e.To], e.From)
		}
	}
	before := func(x, y int) bool {
		a, b := doc.UseCases[members[x]], doc.UseCases[members[y]]
		if a.SourceURI != b.SourceURI {
			return a.SourceURI < b.SourceURI
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return x < y
	}
	named := namesake(doc, slug, members, before)
	reached := make([]bool, n)
	placed := make([]bool, n)
	// onCycle[m] reports whether m sits in a strongly connected component
	// of the unplaced members that no unplaced member outside it feeds.
	// Recomputed before each pick.
	onCycle := make([]bool, n)
	sourceSCCs := func() {
		comp := unplacedSCCs(n, succ, placed)
		fed := make(map[int]bool)
		for m := range n {
			if placed[m] {
				continue
			}
			for _, s := range succ[m] {
				if !placed[s] && comp[s] != comp[m] {
					fed[comp[s]] = true
				}
			}
		}
		for m := range n {
			onCycle[m] = !placed[m] && !fed[comp[m]]
		}
	}
	tier := func(m int) int {
		switch {
		case !connected[m]:
			return 4
		case indeg[m] == 0:
			return 0
		case !onCycle[m]:
			return 3
		case reached[m]:
			return 1
		}
		return 2
	}
	last := -1
	follows := func(m int) bool { return last >= 0 && slices.Contains(succ[last], m) }
	better := func(m, cur int) bool {
		if tm, tc := tier(m), tier(cur); tm != tc {
			return tm < tc
		}
		if fm, fc := follows(m), follows(cur); fm != fc {
			return fm
		}
		if m == named || cur == named {
			return m == named
		}
		return before(m, cur)
	}
	for len(order) < n {
		sourceSCCs()
		next := -1
		for m := range n {
			if !placed[m] && (next < 0 || better(m, next)) {
				next = m
			}
		}
		if t := tier(next); t >= 1 && t <= 3 {
			cycle = true
		}
		placed[next] = true
		order = append(order, next)
		last = next
		for _, s := range succ[next] {
			indeg[s]--
			reached[s] = true
		}
	}
	return order, cycle
}

// unplacedSCCs returns, for each unplaced member, the id of its strongly
// connected component in the graph of unplaced members (Tarjan). Placed
// members get -1.
func unplacedSCCs(n int, succ [][]int, placed []bool) []int {
	comp := make([]int, n)
	index := make([]int, n)
	low := make([]int, n)
	onStack := make([]bool, n)
	for m := range n {
		comp[m], index[m] = -1, -1
	}
	var stack []int
	next, ids := 0, 0
	var visit func(v int)
	visit = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range succ[v] {
			switch {
			case placed[w]:
			case index[w] < 0:
				visit(w)
				low[v] = min(low[v], low[w])
			case onStack[w]:
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] == index[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp[w] = ids
				if w == v {
					break
				}
			}
			ids++
		}
	}
	for m := range n {
		if !placed[m] && index[m] < 0 {
			visit(m)
		}
	}
	return comp
}

// namesake returns the member the journey is named after, or -1. In order
// of preference, ties to the first by before: a member whose title slugifies
// to slug; a member in a file whose base name, without .craft, is slug; a
// member in a file named after one of its own formerly_journey values.
func namesake(doc *model.CraftDoc, slug string, members []int, before func(x, y int) bool) int {
	const none = 4
	best, bestRank := -1, none
	for m, i := range members {
		uc := doc.UseCases[i]
		base := strings.TrimSuffix(path.Base(filepath.ToSlash(uc.SourceURI)), ".craft")
		rank := none
		switch {
		case slugify(uc.Name) == slug:
			rank = 1
		case base == slug:
			rank = 2
		case formerlyIn(uc, base):
			rank = 3
		}
		if rank < bestRank || rank == bestRank && rank < none && before(m, best) {
			best, bestRank = m, rank
		}
	}
	return best
}

// formerlyIn reports whether slug is one of uc's formerly_journey values.
func formerlyIn(uc model.UseCase, slug string) bool {
	for _, v := range uc.TagValues[lang.TagFormerlyJourney] {
		for _, former := range strings.Split(v, ";") {
			if strings.TrimSpace(former) == slug {
				return true
			}
		}
	}
	return false
}

// slugify lowercases s and turns every run of characters that are not
// letters or digits into one '-', trimmed at both ends:
// "Seller promotes an advert" → "seller-promotes-an-advert".
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	return b.String()
}
