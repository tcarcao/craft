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
	// MessageRef is an includes/continues link, drawn over Over as an
	// interaction use; Label is the title, or "continues: <title>".
	MessageRef MessageKind = "ref"
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
	// Over lists the participant IDs a MessageStepNote, MessageFinding or
	// MessageRef is drawn over; empty means across the diagram (first to last participant).
	Over []string
	// Inferred draws the message grey, with an "(inferred)" suffix.
	Inferred bool
	// Stream wraps the message in its own one-message "loop stream" box. The
	// box opens and closes around this message only, inside whatever blocks
	// BlockOps already opened.
	Stream bool
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

// SequenceOptions change how BuildSequenceWith lays out a sequence.
type SequenceOptions struct {
	// SharedStage keeps the stage across sections: a Listen whose publisher
	// published that event in an earlier section is delivered directly, as
	// from a publisher on stage in its own section (spec §17.4). A publisher
	// outside the diagram, or one that publishes only later, keeps the
	// inbound arrow. Likewise a receives trigger whose operation an earlier
	// section asks of the same context is drawn as a call from that caller,
	// and its unaddressed reply goes back to it. The journey spine sets it.
	SharedStage bool
}

// BuildSequence lays out its with the default options; see
// BuildSequenceWith.
func BuildSequence(doc *model.CraftDoc, its []interaction.Interaction) Sequence {
	return BuildSequenceWith(doc, its, SequenceOptions{})
}

// BuildSequenceWith lays out its as a sequence diagram. Use cases with no
// interactions still get a (empty) section, so a filtered diagram shows what
// was asked for. Each interaction's UseCaseIndex must index doc.UseCases.
//
// Each scenario's messages follow its step tree, so a branch that draws
// nothing still gets a MessageNone. Use cases that share a name are drawn in
// the section of the last of them, in source order; the earlier sections stay
// empty.
func BuildSequenceWith(doc *model.CraftDoc, its []interaction.Interaction, opts SequenceOptions) Sequence {
	b := sequenceBuilder{ns: newNodeSet(doc), inbound: map[string]*inboundArrow{}, shared: opts.SharedStage, published: map[string]int{},
		called: map[string]stagedCall{}, callerOf: map[string]interaction.Endpoint{}}
	byUseCase := map[string]int{}
	for _, uc := range doc.UseCases {
		byUseCase[uc.Name] = len(b.seq.Sections)
		b.seq.Sections = append(b.seq.Sections, SequenceSection{Title: sectionTitle(uc)})
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
				start:  len(b.seq.Sections[idx].Messages),
				roster: len(sc.Trigger.Events) > 0,
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
	// shared is SequenceOptions.SharedStage; published maps
	// "<publisher key>|<event>" to the first section that drew the publish.
	shared    bool
	published map[string]int
	// called maps "<callee key>|<operation key>" to the first section that
	// drew that call, and its caller (shared stage only); callerOf maps
	// "<use case index>|<scenario>" to the caller a receives trigger was
	// drawn from.
	called   map[string]stagedCall
	callerOf map[string]interaction.Endpoint
}

// stagedCall is a call drawn in section, by caller.
type stagedCall struct {
	section int
	caller  interaction.Endpoint
}

// operationKeys are the operation keys of it's bracket (lang.OperationKeys).
func operationKeys(it interaction.Interaction) []string {
	if it.Operation == nil {
		return nil
	}
	return lang.OperationKeys(it.Operation.Text)
}

// calls records that it, a request, is drawn in section idx.
func (b *sequenceBuilder) calls(idx int, it interaction.Interaction) {
	if !b.shared {
		return
	}
	for _, k := range operationKeys(it) {
		if _, ok := b.called[it.To.Key+"|"+k]; !ok {
			b.called[it.To.Key+"|"+k] = stagedCall{section: idx, caller: it.From}
		}
	}
}

// stagedCaller returns the caller of it, a receives trigger in section idx,
// when an earlier section asked its context for one of its operations.
func (b *sequenceBuilder) stagedCaller(idx int, it interaction.Interaction) (interaction.Endpoint, bool) {
	if !b.shared {
		return interaction.Endpoint{}, false
	}
	for _, k := range operationKeys(it) {
		if c, ok := b.called[it.To.Key+"|"+k]; ok && c.section < idx {
			return c.caller, true
		}
	}
	return interaction.Endpoint{}, false
}

func scenarioKeyOf(it interaction.Interaction) string {
	return fmt.Sprintf("%d|%s", it.UseCaseIndex, it.Scenario)
}

// staged reports whether from delivers event directly to a listener in
// section idx: it is drawn in that section, or, with a shared stage, it
// published event in a section drawn before.
func (b *sequenceBuilder) staged(idx int, from interaction.Endpoint, event string) bool {
	if from == interaction.External {
		return false
	}
	if b.onStage[idx][from.Key] {
		return true
	}
	sec, ok := b.published[from.Key+"|"+event]
	return b.shared && ok && sec < idx
}

// publishes records that from publishes event in section idx.
func (b *sequenceBuilder) publishes(idx int, from interaction.Endpoint, event string) {
	if _, ok := b.published[from.Key+"|"+event]; !ok {
		b.published[from.Key+"|"+event] = idx
	}
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

// capList joins items with ", ", capped at maxNamedPublishers with
// " (+N more)" for the rest (spec §4.5).
func capList(items []string) string {
	if len(items) <= maxNamedPublishers {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:maxNamedPublishers], ", ") + fmt.Sprintf(" (+%d more)", len(items)-maxNamedPublishers)
}

// addRoster draws the listens of a trigger with an event list (spec §4.5):
// each publisher already on stage delivers its items on one arrow; every
// other item arrives on one inbound arrow, labelled with those items, the
// publishers they come from (by the rule of inboundArrow.label) and the
// schedule. A single-event trigger never comes here, so it keeps its
// per-event inbound arrow (inboundKey).
func (b *sequenceBuilder) addRoster(idx int, listens []interaction.Interaction, title string) {
	ns, onStage := b.ns, b.onStage[idx]
	var listener interaction.Endpoint
	var schedule string
	direct := map[string][]string{} // on-stage publisher key → items, list order
	var directOrder []interaction.Endpoint
	inbound := &inboundArrow{}
	var inboundItems []string
	for _, it := range listens {
		listener, schedule = it.To, it.Schedule
		if b.staged(idx, it.From, it.Event) {
			if _, seen := direct[it.From.Key]; !seen {
				directOrder = append(directOrder, it.From)
			}
			if !slices.Contains(direct[it.From.Key], it.Event) {
				direct[it.From.Key] = append(direct[it.From.Key], it.Event)
			}
			continue
		}
		if !slices.Contains(inboundItems, it.Event) {
			inboundItems = append(inboundItems, it.Event)
		}
		inbound.addPublisher(it.From)
	}
	suffix := ""
	if schedule != "" {
		suffix = ", " + schedule
	}
	sec := &b.seq.Sections[idx]
	to := ns.add(listener)
	for _, pub := range directOrder {
		from := ns.add(pub)
		b.appear(from)
		sec.Messages = append(sec.Messages, Message{From: from, To: to, Kind: MessageAsync,
			Label: capList(direct[pub.Key]) + suffix, Scenario: title})
	}
	if len(inboundItems) > 0 {
		inbound.head, inbound.suffix = capList(inboundItems), suffix
		sec.Messages = append(sec.Messages, Message{To: to, Kind: MessageInbound, Label: inbound.label(), Scenario: title})
	}
	b.appear(to)
	onStage[to] = true
}

// addOneOf draws the publishes of one `notifies one of` action as one
// message on the publisher's lifeline (spec §8.6), labelled with its events,
// capped as capList caps them.
func (b *sequenceBuilder) addOneOf(idx int, pubs []interaction.Interaction, title string) {
	quoted := make([]string, len(pubs))
	for i, it := range pubs {
		quoted[i] = fmt.Sprintf("%q", it.Event)
		b.publishes(idx, it.From, it.Event)
	}
	from := b.ns.add(pubs[0].From)
	msg := Message{From: from, To: from, Kind: MessageAsync, Scenario: title, Branch: branchPath(pubs[0].Branch),
		Label: "notifies one of " + capList(quoted)}
	if op := opLabel(pubs[0]); op != "" {
		msg.Label += " [" + op + "]"
	}
	if pubs[0].Inferred {
		msg.Label += " (inferred)"
		msg.Inferred = true
	}
	b.appear(from)
	b.onStage[idx][from] = true
	b.seq.Sections[idx].Messages = append(b.seq.Sections[idx].Messages, msg)
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
			if caller, ok := b.stagedCaller(idx, it); ok {
				// A member drawn earlier makes this call (spec §17.4).
				b.callerOf[scenarioKeyOf(it)] = caller
				msg.From, msg.To, msg.Kind = ns.add(caller), ns.add(it.To), MessageCall
				break
			}
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
		b.calls(idx, it)
	case interaction.Response:
		msg.Stream = it.Stream
		if it.To == interaction.Unobserved {
			if caller, ok := b.callerOf[scenarioKeyOf(it)]; ok {
				msg.From, msg.To, msg.Kind = ns.add(it.From), ns.add(caller), MessageReply
				break
			}
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
		b.publishes(idx, it.From, it.Event)
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
		if b.staged(idx, it.From, it.Event) {
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
	if op := opLabel(it); op != "" {
		// The operation is the step's wire contract; a sequence diagram
		// is where a reader looks for it. A trigger with no phrase is
		// labelled by the operation alone.
		msg.Label = strings.TrimSpace(msg.Label + " [" + op + "]")
	}
	if it.Inferred {
		msg.Label += " (inferred)"
		msg.Inferred = true
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

// opLabel is the bracket a label ends with: each operation as written,
// except that an ADDR address drops its quotes (spec §9.5).
func opLabel(it interaction.Interaction) string {
	ops := it.Operations
	if len(ops) == 0 && it.Operation != nil {
		ops = []model.Operation{*it.Operation}
	}
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		if op.Text == "" {
			continue
		}
		if op.Verb == lang.OpVerbAddr {
			parts = append(parts, strings.TrimSpace(op.Verb+" "+lang.UnquoteAddress(op.Payload)))
			continue
		}
		parts = append(parts, op.Text)
	}
	return strings.Join(parts, ", ")
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
	// roster is true when the trigger listens to a list of events: its
	// listens are drawn together by addRoster.
	roster bool
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
	if its := l.bySteps[step]; step > 0 && len(its) > 0 && its[0].Kind == interaction.Publish && its[0].OneOf {
		l.b.addOneOf(l.idx, its, l.title)
		return
	}
	var roster []interaction.Interaction
	for _, it := range l.bySteps[step] {
		if step == 0 && it.Kind == interaction.Trigger && (it.Branch != nil || l.opensBlock) {
			continue
		}
		if step == 0 && l.roster && it.Kind == interaction.Listen {
			roster = append(roster, it)
			continue
		}
		l.b.add(l.idx, it, l.title)
	}
	if len(roster) > 0 {
		l.b.addRoster(l.idx, roster, l.title)
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
		case st.Link != nil:
			l.ref(*st.Link, path)
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

// ref draws a link step as an interaction use over the participants of the
// last message (spec §15.5).
func (l *scenarioLayout) ref(link model.Link, path []BranchRef) {
	label := link.Title
	if link.Kind == model.LinkContinues {
		label = "continues: " + link.Title
	}
	sec := &l.b.seq.Sections[l.idx]
	sec.Messages = append(sec.Messages, Message{Kind: MessageRef, Label: label, Scenario: l.title,
		Branch: slices.Clone(path), Over: l.anchor()})
}

// anchor returns the participants of the last drawn message of this scenario:
// From and To, deduplicated; only To for a message from outside the section,
// only From for what an actor does itself. Nil when nothing has been drawn yet.
func (l *scenarioLayout) anchor() []string {
	msgs := l.b.seq.Sections[l.idx].Messages
	for i := len(msgs) - 1; i >= l.start; i-- {
		m := msgs[i]
		switch m.Kind {
		case MessageNone, MessageStepNote, MessageFinding, MessageRef:
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

// sectionTitle is a use case's section title, with its scope tags appended:
// `<name> [sites: a, b] [envs: c]` (spec §5.3). Values keep authored order.
func sectionTitle(uc model.UseCase) string {
	title := uc.Name
	for _, key := range []string{model.TagSites, model.TagEnvs} {
		if vals := uc.TagValues[key]; len(vals) > 0 {
			title += " [" + key + ": " + strings.Join(vals, ", ") + "]"
		}
	}
	return title
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
