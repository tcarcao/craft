package view

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Sequence is a sequence diagram: one section per use case, messages in
// source order.
type Sequence struct {
	// Participants are listed in order of first appearance.
	Participants []Node
	Sections     []SequenceSection
}

// SequenceSection is the messages of one use case.
type SequenceSection struct {
	Title    string
	Messages []Message
}

// MessageKind tells a writer how to draw a message.
type MessageKind string

const (
	MessageCall    MessageKind = "call"    // request, or an actor starting the scenario
	MessageReply   MessageKind = "reply"   // response
	MessageSelf    MessageKind = "self"    // internal action
	MessageAsync   MessageKind = "async"   // publish: drawn on the publisher's own lifeline
	MessageInbound MessageKind = "inbound" // an event arriving from outside the section
	MessageNote    MessageKind = "note"    // what an actor does, drawn over its own lifeline
)

// Message is one arrow. From is empty for an inbound message.
type Message struct {
	From, To string
	Kind     MessageKind
	Label    string
	// Scenario titles the scenario the message belongs to when its use case
	// has more than one ("2: when …"); writers frame each one. Empty for a
	// use case with a single scenario.
	Scenario string
}

// BuildSequence lays out its as a sequence diagram. Use cases with no
// interactions still get a (empty) section, so a filtered diagram shows what
// was asked for.
func BuildSequence(doc *model.CraftDoc, its []interaction.Interaction) Sequence {
	ns := newNodeSet(doc)
	var order []string
	appear := func(id string) {
		for _, seen := range order {
			if seen == id {
				return
			}
		}
		order = append(order, id)
	}

	var seq Sequence
	byUseCase := map[string]int{}
	scenarioTitle := map[string]string{} // use case + scenario ID → frame title
	for _, uc := range doc.UseCases {
		byUseCase[uc.Name] = len(seq.Sections)
		seq.Sections = append(seq.Sections, SequenceSection{Title: uc.Name})
		if len(uc.Scenarios) > 1 {
			for i, sc := range uc.Scenarios {
				scenarioTitle[uc.Name+"|"+sc.ID] = fmt.Sprintf("%d: %s", i+1, strings.TrimSpace(sc.Trigger.Description))
			}
		}
	}

	inbound := map[string]bool{}
	onStage := make([]map[string]bool, len(seq.Sections))
	for i := range onStage {
		onStage[i] = map[string]bool{}
	}
	for _, it := range its {
		idx, ok := byUseCase[it.UseCase]
		if !ok {
			continue
		}
		msg := Message{Label: it.Label, Scenario: scenarioTitle[it.UseCase+"|"+it.Scenario]}
		switch it.Kind {
		case interaction.Trigger:
			msg.From, msg.To, msg.Kind = ns.addActor(it.From), ns.add(it.To), MessageCall
			if it.From.Key == it.To.Key {
				// The actor takes the first step itself: the trigger is what
				// it does, not a call it makes to itself.
				msg.Kind = MessageNote
			}
		case interaction.Request:
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageCall
		case interaction.Response:
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageReply
		case interaction.Internal:
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.From), MessageSelf
		case interaction.Publish:
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.From), MessageAsync
			msg.Label = "notifies \"" + it.Event + "\""
		case interaction.Listen:
			// A publisher already on stage in this use case delivers the
			// event itself; otherwise the event arrives from outside, once
			// per listening scenario however many publishers it has.
			if onStage[idx][it.From.Key] {
				msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageAsync
				break
			}
			key := it.Scenario + "|" + it.Event
			if inbound[key] {
				continue
			}
			inbound[key] = true
			msg.To, msg.Kind = ns.add(it.To), MessageInbound
		default:
			continue
		}
		if it.Operation != nil && it.Operation.Text != "" {
			// The operation is the step's wire contract; a sequence diagram
			// is where a reader looks for it.
			msg.Label += " [" + it.Operation.Text + "]"
		}
		if msg.From != "" {
			appear(msg.From)
			onStage[idx][msg.From] = true
		}
		appear(msg.To)
		onStage[idx][msg.To] = true
		seq.Sections[idx].Messages = append(seq.Sections[idx].Messages, msg)
	}
	for _, id := range order {
		seq.Participants = append(seq.Participants, ns.byID[id])
	}
	return seq
}
