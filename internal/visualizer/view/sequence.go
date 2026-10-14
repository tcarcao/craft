package view

import (
	"cmp"
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
	// MessageFound comes from the unobserved caller of a `receives` trigger
	// (From is empty); MessageLost is a reply to it (To is empty). UML draws
	// both from or to the diagram's edge: the other party is not known.
	MessageFound MessageKind = "found"
	MessageLost  MessageKind = "lost"
	// MessageStepNote is a `note` line and MessageFinding a `finding` line,
	// drawn over the participants in Over.
	MessageStepNote MessageKind = "step-note"
	MessageFinding  MessageKind = "finding"
	// MessageNone draws nothing. It stands for a branch that adds no message
	// (no drawable interaction, or only an inbound event already drawn), so
	// writers still open (or switch to) that branch: an empty outcome is still
	// an outcome. Writers use only its Branch and Scenario.
	MessageNone MessageKind = "none"
)

// Message is one arrow. From is empty for an inbound or found message, To for
// a lost one. A MessageNone
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
	// Branch is the path from the outermost block inward to the
	// branch the message sits in; nil outside blocks. Writers open, switch and
	// close native blocks from the change between consecutive messages (see
	// BlockOps).
	Branch []BranchRef
	// Over lists the participant IDs a MessageStepNote or MessageFinding is
	// drawn over; empty means across the diagram (first to last participant).
	Over []string
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
	BlockOpen  BlockOpKind = "open"  // a block's first branch: <kind> <label>
	BlockElse  BlockOpKind = "else"  // the next branch of the open block: else (or and) <label>
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
	b := sequenceBuilder{ns: newNodeSet(doc), inbound: map[string]*inboundArrow{}}
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
		// Use-case-level notes open the section, across the diagram.
		for _, n := range uc.Notes {
			b.seq.Sections[idx].Messages = append(b.seq.Sections[idx].Messages, Message{Kind: noteKind(n), Label: n.Text})
		}
		for i, sc := range uc.Scenarios {
			l := scenarioLayout{
				b: &b, idx: idx, bySteps: byScenario[scenarioKey{ui, sc.ID}],
				start: len(b.seq.Sections[idx].Messages),
			}
			if len(uc.Scenarios) > 1 {
				l.title = scenarioTitle(i, sc)
			}
			// An opening par or loop gets an ordinary trigger, drawn before
			// its box (spec §4.4); only an alt or opt is triggered per branch.
			// A receives trigger names its target, so it is drawn up front
			// whatever the scenario opens with (spec §6.4).
			ob := sc.OpeningBlock()
			l.opensBlock = ob != nil && (ob.Kind == lang.BlockAlt || ob.Kind == lang.BlockOpt) &&
				sc.Trigger.Type != lang.TriggerReceive
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
	// inbound holds the inbound arrow drawn for "use case index|scenario|event".
	inbound map[string]*inboundArrow
	// onStage marks, per section, the participants already drawn in it.
	onStage []map[string]bool
}

// inboundArrow is an event drawn as arriving from outside a section, or a
// store read drawn from the reader to the store, and the publishers (writers)
// it arrives from.
type inboundArrow struct {
	section, message int
	// head starts the label: the event of an inbound arrow, "polls <change>"
	// of a store read.
	head       string
	publishers []string // display names, in arrival order
	// suffix ends the label: ", <schedule>" for a scheduled listener.
	suffix string
}

// maxNamedPublishers is how many publishers an inbound label names before
// summing up the rest as "+N more".
const maxNamedPublishers = 3

// label is the event, then the publishers it comes from: sorted by display
// name, at most maxNamedPublishers of them. An event with no publisher in the
// model keeps its bare name.
func (a *inboundArrow) label() string {
	if len(a.publishers) == 0 {
		return a.head + a.suffix
	}
	names := slices.Clone(a.publishers)
	slices.Sort(names)
	if len(names) > maxNamedPublishers {
		names = append(names[:maxNamedPublishers], fmt.Sprintf("+%d more", len(a.publishers)-maxNamedPublishers))
	}
	return a.head + " (from " + strings.Join(names, ", ") + ")" + a.suffix
}

// inboundKey identifies the one arrow a listening scenario draws for an event.
func inboundKey(it interaction.Interaction) string {
	return fmt.Sprintf("%d|%s|%s", it.UseCaseIndex, it.Scenario, it.Event)
}

// growInbound adds it's publisher to the arrow already drawn for its
// listening scenario and event, and reports whether there was one.
func (b *sequenceBuilder) growInbound(it interaction.Interaction) bool {
	arrow := b.inbound[inboundKey(it)]
	if arrow == nil {
		return false
	}
	arrow.addPublisher(it.From)
	b.seq.Sections[arrow.section].Messages[arrow.message].Label = arrow.label()
	return true
}

// newInbound records the arrow it starts, about to be appended to section
// idx, with its label head.
func (b *sequenceBuilder) newInbound(idx int, it interaction.Interaction, head string) *inboundArrow {
	arrow := &inboundArrow{section: idx, message: len(b.seq.Sections[idx].Messages), head: head}
	if it.Schedule != "" {
		arrow.suffix = ", " + it.Schedule
	}
	arrow.addPublisher(it.From)
	b.inbound[inboundKey(it)] = arrow
	return arrow
}

func (a *inboundArrow) addPublisher(from interaction.Endpoint) {
	if from != interaction.External && !slices.Contains(a.publishers, from.Name) {
		a.publishers = append(a.publishers, from.Name)
	}
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
		if it.From == interaction.Unobserved {
			msg.To, msg.Kind = ns.add(it.To), MessageFound
			break
		}
		msg.From, msg.To, msg.Kind = ns.addActor(it.From), ns.add(it.To), MessageCall
		if it.From.Key == it.To.Key {
			// The actor takes the first step itself: the trigger is what
			// it does, not a call it makes to itself.
			msg.Kind = MessageNote
		}
	case interaction.Request:
		msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageCall
	case interaction.Response:
		if it.To == interaction.Unobserved {
			msg.From, msg.Kind = ns.add(it.From), MessageLost
			break
		}
		msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageReply
	case interaction.Internal:
		msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.From), MessageSelf
	case interaction.Publish:
		if !it.Via.IsZero() {
			// A store write: the store lane shows the hand-off.
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.addStore(it.Via), MessageCall
			msg.Label = cmp.Or(storeChange(it.Event), "writes")
			break
		}
		msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.From), MessageAsync
		msg.Label = "notifies \"" + it.Event + "\""
	case interaction.Listen:
		if !it.Via.IsZero() {
			// A store read: the reader polls the store, once per listening
			// scenario, even when a writer is on stage (spec §7.5).
			if b.growInbound(it) {
				return
			}
			arrow := b.newInbound(idx, it, strings.TrimSpace("polls "+storeChange(it.Event)))
			msg.From, msg.To, msg.Kind, msg.Label = ns.add(it.To), ns.addStore(it.Via), MessageCall, arrow.label()
			break
		}
		// A publisher already on stage in this use case delivers the
		// event itself; otherwise the event arrives from outside, once
		// per listening scenario however many publishers it has. The
		// arrow names those publishers but draws none of them: the section
		// stays about its own use case.
		if onStage[it.From.Key] {
			msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(it.To), MessageAsync
			if it.Schedule != "" {
				msg.Label = it.Label + ", " + it.Schedule
			}
			break
		}
		if b.growInbound(it) {
			return
		}
		arrow := b.newInbound(idx, it, it.Event)
		msg.To, msg.Kind, msg.Label = ns.add(it.To), MessageInbound, arrow.label()
	default:
		return
	}
	if it.Operation != nil && it.Operation.Text != "" {
		// The operation is the step's wire contract; a sequence diagram
		// is where a reader looks for it. A trigger with no phrase is
		// labelled by the operation alone.
		msg.Label = strings.TrimSpace(msg.Label + " [" + it.Operation.Text + "]")
	}
	if msg.From != "" {
		b.appear(msg.From)
		onStage[msg.From] = true
	}
	if msg.To != "" {
		b.appear(msg.To)
		onStage[msg.To] = true
	}
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
	// start is the index of the scenario's first message in the section.
	start int
	// opensBlock is true when the scenario's first step that is not a note
	// is an alt or opt, and its trigger is not a receives. Its triggers are then not drawn up front (see emit).
	opensBlock bool
}

// emit draws the interactions of the step-th action (0: the trigger and
// listens).
//
// Two mechanisms draw a trigger when the scenario opens with a block, both
// keyed on interaction.walker.branchTrigger: it gives each trigger Step 0
// plus a Branch, and emit skips those at step 0. emitTriggers draws them when
// the walk enters that branch. The unbranched trigger of an opening opt (the
// path where the opt is skipped) is skipped here too and drawn by
// emitSkipTrigger just before the first action after the block.
func (l *scenarioLayout) emit(step int) {
	for _, it := range l.bySteps[step] {
		if step == 0 && it.Kind == interaction.Trigger && (it.Branch != nil || l.opensBlock) {
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

// emitSkipTrigger draws the unbranched trigger of a scenario that opens with
// an opt, for the path where the opt is skipped.
func (l *scenarioLayout) emitSkipTrigger() {
	for _, it := range l.bySteps[0] {
		if it.Kind == interaction.Trigger && it.Branch == nil {
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
	// skip: the scenario opens with an opt; the first action after it is
	// reached by the unbranched trigger.
	skip := false
	first := model.FirstStepIndex(steps)
	for i, st := range steps {
		switch {
		case st.Action != nil:
			if skip {
				l.emitSkipTrigger()
				skip = false
			}
			// Keep in step with interaction's walker (walk in
			// interaction.go): both number actions in the same order.
			// TestSequenceStepsMatchInteractionSteps guards the pairing.
			l.step++
			l.emit(l.step)
		case st.Note != nil:
			l.note(*st.Note, path)
		case st.Block != nil:
			skip = skip || (len(path) == 0 && i == first && l.opensBlock && st.Block.Kind == lang.BlockOpt)
			for bi, br := range st.Block.Branches {
				inner := append(slices.Clone(path), viewRef(st.Block.ID, st.Block.Kind, br.Label, bi, st.Block.Split))
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

// noteKind is the message kind that draws n.
func noteKind(n model.Note) MessageKind {
	if n.Kind == model.NoteKindFinding {
		return MessageFinding
	}
	return MessageStepNote
}

// note draws a note over the participants of the last message drawn before it
// in this scenario (spec §5.4 anchor rule).
func (l *scenarioLayout) note(n model.Note, path []BranchRef) {
	sec := &l.b.seq.Sections[l.idx]
	sec.Messages = append(sec.Messages, Message{
		Kind: noteKind(n), Label: n.Text, Scenario: l.title,
		Branch: slices.Clone(path), Over: l.anchor(),
	})
}

// anchor returns the participants of the last drawn message of this scenario:
// From and To, deduplicated; only To for a message from outside the section,
// only From for what an actor does itself. Nil when nothing has been drawn yet.
func (l *scenarioLayout) anchor() []string {
	msgs := l.b.seq.Sections[l.idx].Messages
	for i := len(msgs) - 1; i >= l.start; i-- {
		m := msgs[i]
		switch m.Kind {
		case MessageNone, MessageStepNote, MessageFinding:
			continue
		case MessageInbound, MessageFound:
			return []string{m.To}
		case MessageNote, MessageLost:
			return []string{m.From}
		}
		if m.From == m.To {
			return []string{m.From}
		}
		return []string{m.From, m.To}
	}
	return nil
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
		out[i] = viewRef(r.Block, r.Kind, r.Label, r.Index, r.Split)
	}
	return out
}

// viewRef is the view's copy of a branch ref. Every view BranchRef is built
// here, so emitTriggers' comparison of an interaction's path with the walk's
// path keeps matching. A split par branch after the first has a blank label,
// so writers print a bare separator (spec §4.5).
func viewRef(block string, kind lang.BlockKind, label string, index int, split bool) BranchRef {
	if split && index > 0 {
		label = ""
	}
	return BranchRef{Block: block, Kind: kind, Label: label, Index: index}
}
