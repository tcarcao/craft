// Package journey builds a journey spine: the use cases that share a
// `journey` tag, ordered by the hand-offs between them (spec §17.2).
package journey

import (
	"cmp"
	"slices"
	"sort"
	"strings"

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
// and a topological order over the edges (Kahn). Among ready members the one
// with the smallest (source file, line) goes first; when none is ready (a
// cycle) the remaining one with the smallest (source file, line) does, and
// Cycle is set.
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
	order, cycle := topoOrder(doc, members, edges)
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
}

// collect gathers uc's hand-offs, pairing as interaction.Derive does: an
// async action with a subject publishes each event of its list; a listens
// trigger with a subject listens to each item of its list.
func collect(uc model.UseCase) handoffs {
	h := handoffs{publishes: map[string]bool{}, stores: map[string]bool{}, listens: map[string]bool{}, readStore: map[string]bool{}}
	for _, sc := range uc.Scenarios {
		t := sc.Trigger
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
			if l.Kind == model.LinkContinues && l.Target != nil {
				h.continues = append(h.continues, *l.Target)
			}
		})
	}
	return h
}

// edgesBetween lists the edges between members (From/To index members), in
// member order, deduplicated. An includes link adds none: the included flow
// is drawn in place by its ref frame.
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
// edges, ties broken by (source file, line); it reports whether it had to
// break a cycle.
func topoOrder(doc *model.CraftDoc, members []int, edges []model.JourneyEdge) (order []int, cycle bool) {
	n := len(members)
	indeg := make([]int, n)
	succ := make([][]int, n)
	seen := map[[2]int]bool{}
	for _, e := range edges {
		if p := [2]int{e.From, e.To}; !seen[p] {
			seen[p] = true
			indeg[e.To]++
			succ[e.From] = append(succ[e.From], e.To)
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
	placed := make([]bool, n)
	for len(order) < n {
		next, ready := -1, false
		for m := range n {
			if placed[m] {
				continue
			}
			r := indeg[m] == 0
			switch {
			case next < 0, r && !ready, r == ready && before(m, next):
				next, ready = m, r
			}
		}
		if !ready {
			cycle = true
		}
		placed[next] = true
		order = append(order, next)
		for _, s := range succ[next] {
			indeg[s]--
		}
	}
	return order, cycle
}
