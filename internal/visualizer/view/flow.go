package view

import (
	"sort"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Flow is the detailed domain diagram: every interaction as a numbered edge,
// events routed through the publishing context's queue.
type Flow struct {
	Contexts  []Node
	Actors    []Node
	Queues    []Node
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
}

// BuildFlow lays out its as a detailed domain diagram.
func BuildFlow(doc *model.CraftDoc, its []interaction.Interaction) Flow {
	ns := newNodeSet(doc)
	var f Flow
	add := func(from, to string, style EdgeStyle, label string) {
		f.Edges = append(f.Edges, FlowEdge{From: from, To: to, Number: len(f.Edges) + 1, Label: label, Style: style})
	}
	for _, it := range its {
		switch it.Kind {
		case interaction.Trigger:
			add(ns.addActor(it.From), ns.add(it.To), StyleTrigger, it.Label)
		case interaction.Request:
			add(ns.add(it.From), ns.add(it.To), StyleCall, it.Label)
		case interaction.Response:
			add(ns.add(it.From), ns.add(it.To), StyleReply, it.Label)
		case interaction.Internal:
			add(ns.add(it.From), ns.add(it.From), StyleSelf, it.Label)
		case interaction.Publish:
			add(ns.add(it.From), ns.addQueue(it.From), StyleAsync, it.Label)
		case interaction.Listen:
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
	inside := func(e interaction.Endpoint) bool {
		return e != interaction.External && e.Kind != model.ParticipantActor && !(e.Kind == "" && isDeclaredActor(doc, e.Key))
	}
	addContext := func(e interaction.Endpoint) string {
		id := ns.add(e)
		if e.Service != "" && e.Kind != model.ParticipantService {
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
			if inside(it.From) {
				connect(addContext(it.From), ns.addQueue(it.From))
			}
		case interaction.Listen:
			if it.From != interaction.External && inside(it.From) && inside(it.To) {
				connect(ns.addQueue(it.From), addContext(it.To))
			}
		}
	}

	var a Architecture
	grouped := map[string][]Node{}
	for _, n := range ns.sorted(NodeContext) {
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
