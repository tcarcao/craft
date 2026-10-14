package view

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Flow is the detailed domain diagram: every interaction as a numbered edge,
// events routed through the publishing context's queue.
type Flow struct {
	Contexts  []Node
	Actors    []Node
	Queues    []Node
	Stores    []Node
	Externals []Node
	Edges     []FlowEdge
}

// FlowEdge is one numbered step. Numbers count drawn edges only, so they run
// 1, 2, 3 without gaps across the whole diagram.
type FlowEdge struct {
	From, To string
	Number   int
	Label    string
	Style    EdgeStyle
	// Inferred draws the edge grey; its label ends in "(inferred)".
	Inferred bool
}

// BuildFlow lays out its as a detailed domain diagram.
func BuildFlow(doc *model.CraftDoc, its []interaction.Interaction) Flow {
	ns := newNodeSet(doc)
	var f Flow
	var branch []interaction.BranchRef // branch of the interaction being laid out
	inferred := false                  // the interaction being laid out is inferred
	// polled holds the store reads drawn: one per listening scenario and
	// store, however many writers it has.
	type readKey struct {
		useCase       int
		scenario, via string
	}
	polled := map[readKey]bool{}
	add := func(from, to string, style EdgeStyle, label string) {
		// The flow has no boxes, so an edge inside a block names its
		// innermost branch instead.
		// Listens are never prefixed, and a trigger only when its scenario
		// opens with a block (see interaction.Interaction.Branch).
		// A par or loop edge also names its kind (spec §4.5); alt and opt
		// keep the bare label.
		if len(branch) > 0 {
			br := branch[len(branch)-1]
			switch br.Kind {
			case lang.BlockPar, lang.BlockLoop:
				label = "[" + string(br.Kind) + ": " + br.Label + "] " + label
			default:
				label = "[" + br.Label + "] " + label
			}
		}
		if inferred {
			label += " (inferred)"
		}
		f.Edges = append(f.Edges, FlowEdge{From: from, To: to, Number: len(f.Edges) + 1, Label: label, Style: style, Inferred: inferred})
	}
	for _, it := range its {
		branch = it.Branch
		inferred = it.Inferred
		switch it.Kind {
		case interaction.Trigger:
			if it.From == interaction.Unobserved {
				// A receives trigger: an "unobserved caller" node calls the
				// callee (spec §6.5).
				add(ns.add(it.From), ns.add(it.To), StyleTrigger, it.Label)
				continue
			}
			if it.From.Key == it.To.Key {
				// The actor takes the first step itself; that step connects
				// it, and a flow diagram has no note to carry the trigger.
				ns.addActor(it.From)
				continue
			}
			add(ns.addActor(it.From), ns.add(it.To), StyleTrigger, it.Label)
		case interaction.Request:
			add(ns.add(it.From), ns.add(it.To), StyleCall, it.Label)
		case interaction.Response:
			add(ns.add(it.From), ns.add(it.To), StyleReply, it.Label)
		case interaction.Internal:
			add(ns.add(it.From), ns.add(it.From), StyleSelf, it.Label)
		case interaction.Publish:
			if !it.Via.IsZero() {
				add(ns.add(it.From), ns.addStore(it.Via), StyleAsync, cmp.Or(storeChange(it.Event), "writes"))
				continue
			}
			label := it.Label
			if it.OneOf {
				label = "one of: " + label
			}
			add(ns.add(it.From), ns.addQueue(it.From), StyleAsync, label)
		case interaction.Listen:
			if !it.Via.IsZero() {
				k := readKey{it.UseCaseIndex, it.Scenario, it.Via.Key}
				if !polled[k] {
					polled[k] = true
					add(ns.addStore(it.Via), ns.add(it.To), StyleTrigger, strings.TrimSpace("polls "+storeChange(it.Event)))
				}
				continue
			}
			from := ns.add(it.From)
			if it.From != interaction.External {
				from = ns.addQueue(it.From)
			}
			add(from, ns.add(it.To), StyleTrigger, it.Label)
		}
	}
	f.Contexts = ns.sorted(NodeContext)
	f.Actors = ns.sorted(NodeActor)
	f.Queues = ns.sorted(NodeQueue)
	f.Stores = ns.sorted(NodeStore)
	f.Externals = ns.sorted(NodeExternal)
	return f
}

// Architecture is the domain architecture diagram: contexts grouped by the
// service that deploys them, and one unlabeled edge per communicating pair.
type Architecture struct {
	Groups    []ServiceGroup
	Ungrouped []Node
	Queues    []Node
	Edges     []Edge
}

// ServiceGroup is a service and the drawn contexts it deploys.
type ServiceGroup struct {
	Service  Node
	Contexts []Node
}

// Edge is an unlabeled connection.
type Edge struct{ From, To string }

// BuildArchitecture lays out its as an architecture diagram. Actors are not
// drawn, and neither is anything only an actor touches; self-loops are
// dropped and duplicate edges collapse.
func BuildArchitecture(doc *model.CraftDoc, its []interaction.Interaction) Architecture {
	ns := newNodeSet(doc)
	services := map[string]string{} // context node ID → owning service
	// An undeclared name that only ever triggers scenarios, and never acts in
	// one, is someone outside the system: an answer sent back to it is not an
	// edge between contexts.
	triggering, acting := map[string]bool{}, map[string]bool{}
	for _, it := range its {
		switch it.Kind {
		case interaction.Trigger:
			triggering[it.From.Key] = true
		case interaction.Request, interaction.Response, interaction.Internal, interaction.Publish:
			acting[it.From.Key] = true
		}
	}
	outsider := func(e interaction.Endpoint) bool {
		return e.Kind == "" && (isDeclaredActor(doc, e.Key) || triggering[e.Key] && !acting[e.Key])
	}
	inside := func(e interaction.Endpoint) bool {
		return e != interaction.External && e != interaction.Unobserved && e.Kind != model.ParticipantActor && !outsider(e)
	}
	addContext := func(e interaction.Endpoint) string {
		id := ns.add(e)
		if e.Service != "" && e.Kind != model.ParticipantService {
			services[id] = e.Service
		}
		return id
	}
	addStore := func(e interaction.Endpoint) string {
		id := ns.addStore(e)
		if e.Service != "" {
			services[id] = e.Service
		}
		return id
	}
	edges := map[Edge]bool{}
	connect := func(from, to string) {
		if from != to {
			edges[Edge{from, to}] = true
		}
	}
	for _, it := range its {
		switch it.Kind {
		case interaction.Request, interaction.Response:
			if inside(it.From) && inside(it.To) {
				connect(addContext(it.From), addContext(it.To))
			}
		case interaction.Internal:
			if inside(it.From) {
				addContext(it.From)
			}
		case interaction.Publish:
			if !it.Via.IsZero() {
				if inside(it.From) {
					connect(addContext(it.From), addStore(it.Via))
				}
				continue
			}
			if inside(it.From) {
				connect(addContext(it.From), ns.addQueue(it.From))
			}
		case interaction.Listen:
			if !it.Via.IsZero() {
				// The store is read whoever wrote it.
				if inside(it.To) {
					connect(addStore(it.Via), addContext(it.To))
				}
				continue
			}
			if it.From != interaction.External && inside(it.From) && inside(it.To) {
				connect(ns.addQueue(it.From), addContext(it.To))
			}
		}
	}

	var a Architecture
	grouped := map[string][]Node{}
	for _, n := range slices.Concat(ns.sorted(NodeContext), ns.sorted(NodeStore)) {
		if svc, ok := services[n.ID]; ok {
			grouped[svc] = append(grouped[svc], n)
		} else {
			a.Ungrouped = append(a.Ungrouped, n)
		}
	}
	var names []string
	for svc := range grouped {
		names = append(names, svc)
	}
	sort.Strings(names)
	for _, svc := range names {
		a.Groups = append(a.Groups, ServiceGroup{
			Service:  Node{ID: "service:" + svc, Label: svc, Kind: NodeContext},
			Contexts: grouped[svc],
		})
	}
	a.Queues = ns.sorted(NodeQueue)
	for e := range edges {
		a.Edges = append(a.Edges, e)
	}
	sort.Slice(a.Edges, func(i, j int) bool {
		if a.Edges[i].From != a.Edges[j].From {
			return a.Edges[i].From < a.Edges[j].From
		}
		return a.Edges[i].To < a.Edges[j].To
	})
	return a
}

func isDeclaredActor(doc *model.CraftDoc, name string) bool {
	for _, a := range doc.Actors {
		if a.Name == name {
			return true
		}
	}
	return false
}
