// Package plantuml writes views (internal/visualizer/view) as PlantUML source.
// It decides only how things are spelled: every decision about what to draw
// was made when the view was built.
package plantuml

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// aliases maps node IDs to PlantUML identifiers: letters, digits and '_',
// unique within one diagram. IDs are registered in the order the writer
// declares nodes, so the mapping is deterministic.
type aliases struct {
	byID map[string]string
	used map[string]bool
}

func newAliases() *aliases {
	return &aliases{byID: map[string]string{}, used: map[string]bool{}}
}

func (a *aliases) of(id string) string {
	if al, ok := a.byID[id]; ok {
		return al
	}
	return a.declare(id, id)
}

// declare registers a node, basing its identifier on the label the reader
// sees; a clash with an earlier node gets a numeric suffix.
func (a *aliases) declare(id, label string) string {
	if al, ok := a.byID[id]; ok {
		return al
	}
	base := Identifier(label)
	if base == "" {
		base = "node"
	}
	al := base
	for n := 2; a.used[al]; n++ {
		al = fmt.Sprintf("%s_%d", base, n)
	}
	a.byID[id], a.used[al] = al, true
	return al
}

// Identifier turns any name into a PlantUML identifier: runs of characters
// other than letters and digits become one '_'.
func Identifier(name string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range name {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			if pendingSep && b.Len() > 0 {
				b.WriteByte('_')
			}
			pendingSep = false
			b.WriteRune(r)
			continue
		}
		pendingSep = true
	}
	if b.Len() == 0 {
		return "node"
	}
	return b.String()
}

// text makes a label safe inside a double-quoted PlantUML string.
func text(s string) string { return strings.ReplaceAll(s, `"`, `'`) }

// wrap breaks a long multi-word name over two lines.
func wrap(label string, limit int) string {
	if len(label) <= limit {
		return label
	}
	words := strings.Fields(label)
	if len(words) < 2 {
		return label
	}
	mid := len(words) / 2
	return strings.Join(words[:mid], " ") + `\n` + strings.Join(words[mid:], " ")
}

// actorElement is the PlantUML element of an actor kind. Every kind outside
// the system that is not a person keeps the boundary shape (spec §6.5).
func actorElement(t model.ActorType) string {
	switch t {
	case model.ActorTypeSystem, model.ActorTypeScheduler, model.ActorTypeTool, model.ActorTypeInfra:
		return "boundary"
	case model.ActorTypeService:
		return "control"
	}
	return "actor"
}

// actorStereotype names a scheduler, tool or infra actor on its declaration
// line (` <<scheduler>>`); "" for every other kind.
func actorStereotype(t model.ActorType) string {
	switch t {
	case model.ActorTypeScheduler, model.ActorTypeTool, model.ActorTypeInfra:
		return " <<" + string(t) + ">>"
	}
	return ""
}

const header = "@startuml\nleft to right direction\nskinparam backgroundColor white\nskinparam defaultFontName SansSerif\n\n"

const queueStyle = "skinparam queue {\n  BackgroundColor #FFE4B5\n  BorderColor #666666\n  FontSize 10\n}\n\n"

// Flow writes a detailed domain diagram.
func Flow(f view.Flow) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString(header)
	b.WriteString("' Domain styling with frames\nskinparam frame {\n  BackgroundColor #E1BEE7\n  BorderColor #9370DB\n  BorderThickness 2\n  FontColor black\n  FontSize 11\n  FontStyle bold\n}\n\n")
	b.WriteString(queueStyle)
	b.WriteString("skinparam actor {\n  BackgroundColor white\n  BorderColor black\n}\n\n")

	b.WriteString("' Domains as frames\n")
	for _, n := range f.Contexts {
		fmt.Fprintf(&b, "frame \"%s\" as %s\n", text(wrap(n.Label, 15)), al.declare(n.ID, n.Label))
	}
	b.WriteString("\n")
	if len(f.Actors) > 0 {
		b.WriteString("' Actors\n")
		for _, n := range f.Actors {
			fmt.Fprintf(&b, "%s \"%s\" as %s%s\n", actorElement(n.ActorType), text(n.Label), al.declare(n.ID, n.Label), actorStereotype(n.ActorType))
		}
		b.WriteString("\n")
	}
	for _, n := range f.Externals {
		fmt.Fprintf(&b, "cloud \"%s\" as %s\n\n", text(n.Label), al.declare(n.ID, n.Label))
	}
	if len(f.Queues) > 0 {
		b.WriteString("' Domain queues\n")
		for _, n := range f.Queues {
			fmt.Fprintf(&b, "queue \"%s\" as %s\n", text(n.Label), al.declare(n.ID, n.Label))
		}
		b.WriteString("\n")
	}
	if len(f.Stores) > 0 {
		b.WriteString("' Data stores\n")
		for _, n := range f.Stores {
			fmt.Fprintf(&b, "database \"%s\" as %s\n", text(n.Label), al.declare(n.ID, n.Label))
		}
		b.WriteString("\n")
	}

	b.WriteString("' Workflow flows\n")
	for _, e := range f.Edges {
		fmt.Fprintf(&b, "%s %s %s : %d. %s\n", al.of(e.From), flowArrow(e.Style, e.Inferred), al.of(e.To), e.Number, e.Label)
	}
	b.WriteString("\n@enduml")
	return b.String()
}

// flowArrow is the PlantUML arrow of a flow edge: grey when inferred
// (`-[#999999]->>`, spec §9.5).
func flowArrow(s view.EdgeStyle, inferred bool) string {
	arrow := "-->"
	switch s {
	case view.StyleCall, view.StyleAsync:
		arrow = "->>"
	case view.StyleSelf:
		arrow = "->"
	}
	if inferred {
		return "-[#999999]" + arrow
	}
	return arrow
}

// seqArrow is the PlantUML arrow of a sequence message: grey when inferred
// (`-[#999999]>`, `-[#999999]->` for a reply).
func seqArrow(arrow string, inferred bool) string {
	if !inferred {
		return arrow
	}
	return "-[#999999]" + strings.TrimPrefix(arrow, "-")
}

// Architecture writes a domain architecture diagram.
func Architecture(a view.Architecture) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString(header)
	b.WriteString("' Subdomain styling with frames\nskinparam frame {\n  BackgroundColor #E6F3FF\n  BorderColor #4A90E2\n  BorderThickness 2\n  FontColor black\n  FontSize 12\n  FontStyle bold\n}\n\n")
	b.WriteString(queueStyle)
	b.WriteString("skinparam rectangle {\n  BackgroundColor #F0F8FF\n  BorderColor #4169E1\n  BorderThickness 3\n  FontColor #000080\n  FontSize 14\n  FontStyle bold\n}\n\n")

	if len(a.Groups) > 0 {
		b.WriteString("' Service boundaries\n")
		for _, g := range a.Groups {
			fmt.Fprintf(&b, "rectangle \"%s\" as %s {\n", text(wrap(g.Service.Label, 20)), al.declare(g.Service.ID, g.Service.Label))
			for _, n := range g.Contexts {
				fmt.Fprintf(&b, "  %s \"%s\" as %s\n", archElement(n), text(wrap(n.Label, 20)), al.declare(n.ID, n.Label))
			}
			b.WriteString("}\n")
		}
		b.WriteString("\n")
	}
	if len(a.Ungrouped) > 0 {
		b.WriteString("' Ungrouped subdomains\n")
		for _, n := range a.Ungrouped {
			fmt.Fprintf(&b, "%s \"%s\" as %s\n", archElement(n), text(wrap(n.Label, 20)), al.declare(n.ID, n.Label))
		}
		b.WriteString("\n")
	}
	if len(a.Queues) > 0 {
		b.WriteString("' Domain queues\n")
		for _, n := range a.Queues {
			fmt.Fprintf(&b, "queue \"%s\" as %s\n", text(n.Label), al.declare(n.ID, n.Label))
		}
		b.WriteString("\n")
	}
	b.WriteString("' Subdomain connections\n")
	for _, e := range a.Edges {
		fmt.Fprintf(&b, "%s --> %s\n", al.of(e.From), al.of(e.To))
	}
	b.WriteString("\n@enduml")
	return b.String()
}

// archElement is how an architecture node is drawn: a store as a database,
// anything else as a frame.
func archElement(n view.Node) string {
	if n.Kind == view.NodeStore {
		return "database"
	}
	return "frame"
}

// Sequence writes a sequence diagram.
func Sequence(s view.Sequence) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("@startuml\nskinparam backgroundColor white\nskinparam defaultFontName SansSerif\nskinparam sequenceMessageAlign center\n\n")
	for _, p := range s.Participants {
		kind, stereo := "participant", ""
		switch p.Kind {
		case view.NodeActor:
			kind, stereo = actorElement(p.ActorType), actorStereotype(p.ActorType)
		case view.NodeStore:
			kind = "database"
		}
		fmt.Fprintf(&b, "%s \"%s\" as %s%s\n", kind, text(p.Label), al.declare(p.ID, p.Label), stereo)
	}
	if len(s.Participants) > 0 {
		b.WriteString("\n")
	}
	for _, sec := range s.Sections {
		fmt.Fprintf(&b, "== %s ==\n\n", sec.Title)
		group := ""
		var path []view.BranchRef
		for _, m := range sec.Messages {
			if m.Scenario != group {
				writeBlockOps(&b, view.BlockOps(path, nil))
				path = nil
				if group != "" {
					b.WriteString("end\n")
				}
				if m.Scenario != "" {
					fmt.Fprintf(&b, "group %s\n", m.Scenario)
				}
				group = m.Scenario
			}
			writeBlockOps(&b, view.BlockOps(path, m.Branch))
			path = m.Branch
			if m.Stream {
				b.WriteString("loop stream\n")
			}
			switch m.Kind {
			case view.MessageNone:
			case view.MessageInbound, view.MessageFound:
				fmt.Fprintf(&b, "[-> %s : %s\n", al.of(m.To), m.Label)
			case view.MessageLost:
				fmt.Fprintf(&b, "%s -->] : %s\n", al.of(m.From), m.Label)
			case view.MessageNote:
				fmt.Fprintf(&b, "note over %s : %s\n", al.of(m.From), m.Label)
			case view.MessageStepNote, view.MessageFinding:
				over := noteOver(s, m.Over, al, ", ")
				if over == "" {
					continue
				}
				colour, label := "", m.Label
				if m.Kind == view.MessageFinding {
					colour, label = " #FFE4E1", "Finding: "+m.Label
				}
				fmt.Fprintf(&b, "note over %s%s : %s\n", over, colour, label)
			case view.MessageRef:
				if over := noteOver(s, m.Over, al, ", "); over != "" {
					fmt.Fprintf(&b, "ref over %s : %s\n", over, m.Label)
				}
			case view.MessageReply:
				fmt.Fprintf(&b, "%s %s %s : %s\n", al.of(m.From), seqArrow("-->", m.Inferred), al.of(m.To), m.Label)
			case view.MessageAsync:
				fmt.Fprintf(&b, "%s %s %s : %s\n", al.of(m.From), seqArrow("->>", m.Inferred), al.of(m.To), m.Label)
			default:
				fmt.Fprintf(&b, "%s %s %s : %s\n", al.of(m.From), seqArrow("->", m.Inferred), al.of(m.To), m.Label)
			}
			if m.Stream {
				b.WriteString("end\n")
			}
		}
		writeBlockOps(&b, view.BlockOps(path, nil))
		if group != "" {
			b.WriteString("end\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("@enduml")
	return b.String()
}

// noteOver spells the participants a note is drawn over: the listed ones, or
// the first and last participant of the diagram when the list is empty.
func noteOver(s view.Sequence, over []string, al *aliases, sep string) string {
	if len(over) == 0 {
		switch len(s.Participants) {
		case 0:
			return ""
		case 1:
			return al.of(s.Participants[0].ID)
		}
		over = []string{s.Participants[0].ID, s.Participants[len(s.Participants)-1].ID}
	}
	names := make([]string, len(over))
	for i, id := range over {
		names[i] = al.of(id)
	}
	return strings.Join(names, sep)
}

// writeBlockOps draws alt/opt/par/loop block lines. PlantUML separates the
// branches of every block kind, par included, with else. A close is a bare
// end: its BlockKind and Label are informational only.
func writeBlockOps(b *strings.Builder, ops []view.BlockOp) {
	for _, op := range ops {
		switch op.Kind {
		case view.BlockOpen:
			writeHeader(b, string(op.BlockKind), op.Label)
		case view.BlockElse:
			writeHeader(b, "else", op.Label)
		case view.BlockClose:
			b.WriteString("end\n")
		}
	}
}

// writeHeader writes a block line, with no trailing space for an empty label.
func writeHeader(b *strings.Builder, kw, label string) {
	if label == "" {
		fmt.Fprintf(b, "%s\n", kw)
		return
	}
	fmt.Fprintf(b, "%s %s\n", kw, label)
}
