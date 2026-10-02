// Package view turns derived interactions into what each diagram shows,
// independent of the diagram language. A view is plain, fully ordered data
// with every filter already applied; the plantuml and mermaid writers only
// spell it out, so both backends draw the same thing.
package view

import (
	"sort"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

// NodeKind classifies a drawn node.
type NodeKind string

const (
	NodeContext  NodeKind = "context"  // bounded context, domain, service, data store, or unresolved name
	NodeActor    NodeKind = "actor"    // a declared actor
	NodeQueue    NodeKind = "queue"    // the event queue of one publishing context
	NodeExternal NodeKind = "external" // whatever lies outside the model
)

// Node is one drawn element. ID is stable and unique within a view; writers
// derive diagram identifiers from it. Label is what the reader sees.
type Node struct {
	ID        string
	Label     string
	Kind      NodeKind
	ActorType model.ActorType // for NodeActor; "" when the actor is not declared
}

// EdgeStyle tells a writer how to draw an edge.
type EdgeStyle string

const (
	StyleCall    EdgeStyle = "call"    // request
	StyleReply   EdgeStyle = "reply"   // response
	StyleAsync   EdgeStyle = "async"   // publish to a queue
	StyleSelf    EdgeStyle = "self"    // internal action
	StyleTrigger EdgeStyle = "trigger" // actor starts a scenario, or an event is delivered
)

// queueID is the node ID of a context's event queue.
func queueID(publisher string) string { return "queue:" + publisher }

// nodes collects the endpoints of interactions as nodes, keyed by ID.
type nodeSet struct {
	byID   map[string]Node
	actors map[string]model.ActorType
}

func newNodeSet(doc *model.CraftDoc) *nodeSet {
	ns := &nodeSet{byID: map[string]Node{}, actors: map[string]model.ActorType{}}
	for _, a := range doc.Actors {
		if _, seen := ns.actors[a.Name]; !seen {
			ns.actors[a.Name] = a.Type
		}
	}
	return ns
}

// add records an endpoint as a node and returns its ID.
func (ns *nodeSet) add(e interaction.Endpoint) string {
	if _, ok := ns.byID[e.Key]; ok {
		return e.Key
	}
	n := Node{ID: e.Key, Label: e.Name, Kind: NodeContext}
	switch {
	case e == interaction.External:
		n.Kind = NodeExternal
	case e.Kind == model.ParticipantActor:
		n.Kind, n.ActorType = NodeActor, ns.actors[e.Key]
	case e.Kind == "":
		// An undeclared name in the actor slot of a trigger reads as an actor.
		if t, ok := ns.actors[e.Key]; ok {
			n.Kind, n.ActorType = NodeActor, t
		}
	}
	ns.byID[e.Key] = n
	return e.Key
}

func (ns *nodeSet) addActor(e interaction.Endpoint) string {
	id := ns.add(e)
	if n := ns.byID[id]; n.Kind == NodeContext && e.Kind == "" {
		n.Kind = NodeActor
		ns.byID[id] = n
	}
	return id
}

func (ns *nodeSet) addQueue(publisher interaction.Endpoint) string {
	id := queueID(publisher.Key)
	if _, ok := ns.byID[id]; !ok {
		ns.byID[id] = Node{ID: id, Label: publisher.Name + " events", Kind: NodeQueue}
	}
	return id
}

// sorted returns the nodes of one kind ordered by label, then ID.
func (ns *nodeSet) sorted(kind NodeKind) []Node {
	var out []Node
	for _, n := range ns.byID {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}
