package view

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/lang"
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
	// MessageNone draws nothing. It stands for a branch that adds no message
	// (no drawable interaction, or only an inbound event already drawn), so
	// writers still open (or switch to) that branch: an empty outcome is still
	// an outcome. Writers use only its Branch and Scenario.
	MessageNone MessageKind = "none"
)

// Message is one arrow. From is empty for an inbound message. A MessageNone
// is no arrow at all, only a branch position: consumers that draw or count
// arrows must skip it.
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
	// Block identifies the block (model.Block.ID). It is unique within the
	// source file, hence also within a scenario, which is all BlockOps
	// compares it across.
	Block string
	Kind  lang.BlockKind
	Label string
	Index int // 0-based branch index within the block
}

// BlockOpKind is what a BlockOp draws.
type BlockOpKind string

const (
	BlockOpen  BlockOpKind = "open"  // a block's first branch: alt/opt <label>
	BlockElse  BlockOpKind = "else"  // the next branch of the open block: else <label>
	BlockClose BlockOpKind = "close" // end
)

// BlockOp is one block line to emit between two messages. On a BlockClose,
// BlockKind and Label name the block being closed; they are informational
// only, since both backends close every block with a bare end.
type BlockOp struct {
	Kind      BlockOpKind
	BlockKind lang.BlockKind
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
// was asked for. Each interaction's UseCaseIndex must index doc.UseCases.
//
// Each scenario's messages follow its step tree, so a branch that draws
// nothing still gets a MessageNone. Use cases that share a name are drawn in
// the section of the last of them, in source order; the earlier sections stay
// empty.
func BuildSequence(doc *model.CraftDoc, its []interaction.Interaction) Sequence {
	b := sequenceBuilder{ns: newNodeSet(doc), inbound: map[string]bool{}}
	byUseCase := map[string]int{}
	for _, uc := range doc.UseCases {
		byUseCase[uc.Name] = len(b.seq.Sections)
		b.seq.Sections = append(b.seq.Sections, SequenceSection{Title: uc.Name})
		b.onStage = append(b.onStage, map[string]bool{})
	}

	// Interactions by scenario, then by step. UseCaseIndex, not the name,
	// identifies the use case: names (and per-file scenario IDs) can repeat.
	type scenarioKey struct {
		useCase  int
		scenario string
	}
	byScenario := map[scenarioKey]map[int][]interaction.Interaction{}
	for _, it := range its {
		k := scenarioKey{it.UseCaseIndex, it.Scenario}
		if byScenario[k] == nil {
			byScenario[k] = map[int][]interaction.Interaction{}
		}
		byScenario[k][it.Step] = append(byScenario[k][it.Step], it)
	}

	for ui, uc := range doc.UseCases {
		idx := byUseCase[uc.Name]
		for i, sc := range uc.Scenarios {
			l := scenarioLayout{b: &b, idx: idx, bySteps: byScenario[scenarioKey{ui, sc.ID}]}
			if len(uc.Scenarios) > 1 {
				l.title = scenarioTitle(i, sc)
			}
			l.emit(0) // the trigger and listens
			l.walk(sc.Steps, nil)
		}
	}
	for _, id := range b.order {
		b.seq.Participants = append(b.seq.Participants, b.ns.byID[id])
	}
	return b.seq
}

// sequenceBuilder holds what BuildSequence tracks across use cases.
type sequenceBuilder struct {
	ns  *nodeSet
	seq Sequence
	// order lists participant IDs in order of first appearance.
	order []string
	// inbound marks "use case index|scenario|event" already drawn as inbound.
	inbound map[string]bool
	// onStage marks, per section, the participants already drawn in it.
	onStage []map[string]bool
}

func (b *sequenceBuilder) appear(id string) {
	if !slices.Contains(b.order, id) {
		b.order = append(b.order, id)
	}
}

// add appends the message of it to section idx, if it draws one. It draws
// none for an unknown kind or for an inbound event already drawn.
func (b *sequenceBuilder) add(idx int, it interaction.Interaction, title string) {
	ns, onStage := b.ns, b.onStage[idx]
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
		if onStage[it.From.Key] {
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageAsync
			break
		}
		key := fmt.Sprintf("%d|%s|%s", it.UseCaseIndex, it.Scenario, it.Event)
		if b.inbound[key] {
			return
		}
		b.inbound[key] = true
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
		b.appear(msg.From)
		onStage[msg.From] = true
	}
	b.appear(msg.To)
	onStage[msg.To] = true
	sec := &b.seq.Sections[idx]
	sec.Messages = append(sec.Messages, msg)
}

// scenarioLayout draws one scenario into section idx by walking its step
// tree in source order.
type scenarioLayout struct {
	b       *sequenceBuilder
	idx     int
	title   string // frame title; "" for a use case with one scenario
	bySteps map[int][]interaction.Interaction
	step    int // actions seen so far, numbered as interaction numbers them
}

// emit draws the interactions of the step-th action (0: the trigger and
// listens). A trigger that sits in a branch (its scenario opens with a block)
// is drawn by emitTriggers when the walk enters that branch instead.
func (l *scenarioLayout) emit(step int) {
	for _, it := range l.bySteps[step] {
		if step == 0 && it.Branch != nil {
			continue
		}
		l.b.add(l.idx, it, l.title)
	}
}

// emitTriggers draws the triggers that reach the branch at path, so they open
// that branch, before its first message.
func (l *scenarioLayout) emitTriggers(path []BranchRef) {
	for _, it := range l.bySteps[0] {
		if it.Kind == interaction.Trigger && it.Branch != nil && slices.Equal(branchPath(it.Branch), path) {
			l.b.add(l.idx, it, l.title)
		}
	}
}

// walk draws steps, which sit at path. A branch counts as empty when walking
// it adds no message to the section, including when its only interactions
// draw nothing (an inbound event already drawn); it then gets a MessageNone,
// so writers still draw its header.
func (l *scenarioLayout) walk(steps []model.Step, path []BranchRef) {
	sec := &l.b.seq.Sections[l.idx]
	for _, st := range steps {
		switch {
		case st.Action != nil:
			// Keep in step with interaction's walker (walk in
			// interaction.go): both number actions in the same order.
			// TestSequenceStepsMatchInteractionSteps guards the pairing.
			l.step++
			l.emit(l.step)
		case st.Block != nil:
			for i, br := range st.Block.Branches {
				inner := append(slices.Clone(path), BranchRef{Block: st.Block.ID, Kind: st.Block.Kind, Label: br.Label, Index: i})
				before := len(sec.Messages)
				l.emitTriggers(inner)
				l.walk(br.Steps, inner)
				if len(sec.Messages) == before {
					sec.Messages = append(sec.Messages, Message{Kind: MessageNone, Scenario: l.title, Branch: inner})
				}
			}
		}
	}
}

// scenarioTitle titles the i-th scenario's frame: its name, else its trigger.
func scenarioTitle(i int, sc model.Scenario) string {
	if name := strings.TrimSpace(sc.Name); name != "" {
		return fmt.Sprintf("%d: %s", i+1, name)
	}
	return fmt.Sprintf("%d: %s", i+1, strings.TrimSpace(sc.Trigger.Description))
}

// branchPath copies an interaction's branch path into the view's own type,
// so writers need not import interaction.
func branchPath(refs []interaction.BranchRef) []BranchRef {
	if refs == nil {
		return nil
	}
	out := make([]BranchRef, len(refs))
	for i, r := range refs {
		out[i] = BranchRef{Block: r.Block, Kind: r.Kind, Label: r.Label, Index: r.Index}
	}
	return out
}
