package mermaid

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// node writes one flowchart node in the shape of its kind.
func node(b *strings.Builder, indent string, al *aliases, n view.Node) {
	id, label := al.declare(n.ID, n.Label), text(n.Label)
	switch n.Kind {
	case view.NodeActor:
		fmt.Fprintf(b, "%s%s([\"%s\"])\n", indent, id, label)
	case view.NodeQueue:
		fmt.Fprintf(b, "%s%s[(\"%s\")]\n", indent, id, label)
	case view.NodeExternal:
		fmt.Fprintf(b, "%s%s{{\"%s\"}}\n", indent, id, label)
	default:
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, id, label)
	}
}

// Flow writes a detailed domain diagram as a flowchart.
func Flow(f view.Flow) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("flowchart LR\n")
	for _, group := range [][]view.Node{f.Actors, f.Contexts, f.Queues, f.Externals} {
		for _, n := range group {
			node(&b, "    ", al, n)
		}
	}
	for _, e := range f.Edges {
		label := text(fmt.Sprintf("%d. %s", e.Number, e.Label))
		if e.Style == view.StyleReply {
			fmt.Fprintf(&b, "    %s -. \"%s\" .-> %s\n", al.of(e.From), label, al.of(e.To))
			continue
		}
		fmt.Fprintf(&b, "    %s -- \"%s\" --> %s\n", al.of(e.From), label, al.of(e.To))
	}
	return b.String()
}

// Architecture writes a domain architecture diagram: services as subgraphs.
func Architecture(a view.Architecture) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("flowchart LR\n")
	for _, g := range a.Groups {
		fmt.Fprintf(&b, "    subgraph %s[\"%s\"]\n", al.declare(g.Service.ID, g.Service.Label), text(g.Service.Label))
		for _, n := range g.Contexts {
			node(&b, "        ", al, n)
		}
		b.WriteString("    end\n")
	}
	for _, group := range [][]view.Node{a.Ungrouped, a.Queues} {
		for _, n := range group {
			node(&b, "    ", al, n)
		}
	}
	for _, e := range a.Edges {
		fmt.Fprintf(&b, "    %s --> %s\n", al.of(e.From), al.of(e.To))
	}
	return b.String()
}

// Sequence writes a sequence diagram. Each use case opens with a note across
// all participants. Mermaid has no "found message", so an event arriving from
// outside the section comes from an External participant.
func Sequence(s view.Sequence) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("sequenceDiagram\n")
	for _, p := range s.Participants {
		kind := "participant"
		if p.Kind == view.NodeActor {
			kind = "actor"
		}
		fmt.Fprintf(&b, "    %s %s as %s\n", kind, al.declare(p.ID, p.Label), strings.ReplaceAll(p.Label, ";", ","))
	}
	external := ""
	for _, sec := range s.Sections {
		for _, m := range sec.Messages {
			if m.Kind == view.MessageInbound && external == "" {
				external = al.of("External")
				fmt.Fprintf(&b, "    participant %s as External\n", external)
			}
		}
	}
	for _, sec := range s.Sections {
		switch {
		case len(s.Participants) > 1:
			fmt.Fprintf(&b, "    Note over %s,%s: %s\n", al.of(s.Participants[0].ID), al.of(s.Participants[len(s.Participants)-1].ID), sec.Title)
		case len(s.Participants) == 1:
			fmt.Fprintf(&b, "    Note over %s: %s\n", al.of(s.Participants[0].ID), sec.Title)
		}
		group := ""
		for _, m := range sec.Messages {
			if m.Scenario != group {
				if group != "" {
					b.WriteString("    end\n")
				}
				if m.Scenario != "" {
					b.WriteString("    rect rgb(245, 245, 245)\n")
					fmt.Fprintf(&b, "    Note over %s: %s\n", spanAll(s, al), strings.ReplaceAll(m.Scenario, ";", ","))
				}
				group = m.Scenario
			}
			label := strings.ReplaceAll(m.Label, ";", ",")
			switch m.Kind {
			case view.MessageInbound:
				fmt.Fprintf(&b, "    %s->>%s: %s\n", external, al.of(m.To), label)
			case view.MessageNote:
				fmt.Fprintf(&b, "    Note over %s: %s\n", al.of(m.From), label)
			case view.MessageReply:
				fmt.Fprintf(&b, "    %s-->>%s: %s\n", al.of(m.From), al.of(m.To), label)
			case view.MessageAsync:
				fmt.Fprintf(&b, "    %s-)%s: %s\n", al.of(m.From), al.of(m.To), label)
			default:
				fmt.Fprintf(&b, "    %s->>%s: %s\n", al.of(m.From), al.of(m.To), label)
			}
		}
		if group != "" {
			b.WriteString("    end\n")
		}
	}
	return b.String()
}

// spanAll is the participant range a note across the whole diagram covers.
func spanAll(s view.Sequence, al *aliases) string {
	if len(s.Participants) > 1 {
		return al.of(s.Participants[0].ID) + "," + al.of(s.Participants[len(s.Participants)-1].ID)
	}
	return al.of(s.Participants[0].ID)
}
