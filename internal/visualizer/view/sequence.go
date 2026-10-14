package view

import (
	"fmt"
	"slices"
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
	// MessageNone draws nothing. It stands for a branch with no drawable
	// interaction, so writers still open (or switch to) that branch: an empty
	// outcome is still an outcome. Writers use only its Branch and Scenario.
	MessageNone MessageKind = "none"
)

// Message is one arrow. From is empty for an inbound message.
type Message struct {
	From, To string
	Kind     MessageKind
	Label    string
	// Scenario titles the scenario the message belongs to when its use case
	// has more than one: "2: <name>", or "2: when …" for an unnamed one;
	// writers frame each one. Empty for a use case with a single scenario.
	Scenario string
	// Branch is the path from the outermost alt/opt block inward to the
	// branch the message sits in; nil outside blocks. Writers open, switch and
	// close native blocks from the change between consecutive messages (see
	// BlockOps).
	Branch []BranchRef
}

// BranchRef locates a message inside one branch of a block.
type BranchRef struct {
	// Block identifies the block within its scenario.
	Block string
	Kind  string // "alt" or "opt"
	Label string
	Index int // 0-based branch index within the block
}

// Block line kinds of a BlockOp.
const (
	BlockOpen  = "open"
	BlockElse  = "else"
	BlockClose = "close"
)

// BlockOp is one block line to emit between two messages.
type BlockOp struct {
	Kind      string // BlockOpen, BlockElse or BlockClose
	BlockKind string // "alt" or "opt"
	Label     string
}

// BlockOps returns the block lines to emit when moving from a message in
// prev's branch path to one in next's: closes (innermost first), then an
// else-switch or opens. BlockOps(prev, nil) closes every open block.
func BlockOps(prev, next []BranchRef) []BlockOp {
	d := 0
	for d < len(prev) && d < len(next) && prev[d].Block == next[d].Block && prev[d].Index == next[d].Index {
		d++
	}
	var ops []BlockOp
	closeFrom := func(depth int) {
		for i := len(prev) - 1; i >= depth; i-- {
			ops = append(ops, BlockOp{Kind: BlockClose, BlockKind: prev[i].Kind, Label: prev[i].Label})
		}
	}
	openFrom := func(depth int) {
		for _, r := range next[depth:] {
			ops = append(ops, BlockOp{Kind: BlockOpen, BlockKind: r.Kind, Label: r.Label})
		}
	}
	if d < len(prev) && d < len(next) && prev[d].Block == next[d].Block {
		// Another branch of the same block.
		closeFrom(d + 1)
		ops = append(ops, BlockOp{Kind: BlockElse, BlockKind: next[d].Kind, Label: next[d].Label})
		openFrom(d + 1)
		return ops
	}
	closeFrom(d)
	openFrom(d)
	return ops
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
	for _, uc := range doc.UseCases {
		byUseCase[uc.Name] = len(seq.Sections)
		seq.Sections = append(seq.Sections, SequenceSection{Title: uc.Name})
	}

	// Interactions by source position, so each scenario's messages follow its
	// step tree, which also places branches that draw nothing.
	type stepKey struct {
		useCase, scenario string
		step              int
	}
	byStep := map[stepKey][]interaction.Interaction{}
	for _, it := range its {
		k := stepKey{it.UseCase, it.Scenario, it.Step}
		byStep[k] = append(byStep[k], it)
	}

	inbound := map[string]bool{}
	onStage := make([]map[string]bool, len(seq.Sections))
	for i := range onStage {
		onStage[i] = map[string]bool{}
	}
	// add appends the message of it to section idx, if it draws one.
	add := func(idx int, it interaction.Interaction, title string) {
		msg := Message{Label: it.Label, Scenario: title, Branch: branchPath(it.Branch)}
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
				return
			}
			inbound[key] = true
			msg.To, msg.Kind = ns.add(it.To), MessageInbound
		default:
			return
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

	for _, uc := range doc.UseCases {
		idx := byUseCase[uc.Name]
		for i, sc := range uc.Scenarios {
			title := ""
			if len(uc.Scenarios) > 1 {
				title = scenarioTitle(i, sc)
			}
			emit := func(step int) {
				k := stepKey{uc.Name, sc.ID, step}
				for _, it := range byStep[k] {
					add(idx, it, title)
				}
				delete(byStep, k) // a use case name given twice draws each step once
			}
			emit(0) // trigger and listens
			step := 0
			var walk func(steps []model.Step, path []BranchRef)
			walk = func(steps []model.Step, path []BranchRef) {
				for _, st := range steps {
					switch {
					case st.Action != nil:
						step++
						emit(step)
					case st.Block != nil:
						for bi, br := range st.Block.Branches {
							inner := append(slices.Clone(path), BranchRef{Block: st.Block.ID, Kind: string(st.Block.Kind), Label: br.Label, Index: bi})
							before := len(seq.Sections[idx].Messages)
							walk(br.Steps, inner)
							if len(seq.Sections[idx].Messages) == before {
								seq.Sections[idx].Messages = append(seq.Sections[idx].Messages,
									Message{Kind: MessageNone, Scenario: title, Branch: inner})
							}
						}
					}
				}
			}
			walk(sc.Steps, nil)
		}
	}
	for _, id := range order {
		seq.Participants = append(seq.Participants, ns.byID[id])
	}
	return seq
}

// scenarioTitle titles the i-th scenario's frame: its name, else its trigger.
func scenarioTitle(i int, sc model.Scenario) string {
	if name := strings.TrimSpace(sc.Name); name != "" {
		return fmt.Sprintf("%d: %s", i+1, name)
	}
	return fmt.Sprintf("%d: %s", i+1, strings.TrimSpace(sc.Trigger.Description))
}

func branchPath(refs []interaction.BranchRef) []BranchRef {
	if refs == nil {
		return nil
	}
	out := make([]BranchRef, len(refs))
	for i, r := range refs {
		out[i] = BranchRef{Block: r.Block, Kind: string(r.Kind), Label: r.Label, Index: r.Index}
	}
	return out
}
