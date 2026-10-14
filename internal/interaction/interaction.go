// Package interaction derives, once, every communication a model describes:
// who triggers, asks, answers, publishes and listens to whom. Diagrams and the
// context-map lints read this list instead of each walking the use cases with
// their own rules.
//
// It depends only on the document model: names arrive already resolved
// (model.Participant), so this package never matches names itself.
package interaction

import (
	"slices"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Kind classifies an interaction.
type Kind string

const (
	// Trigger: an actor outside the system starts a scenario; To is the
	// subject of the scenario's first action. For a `receives` trigger From
	// is Unobserved and To is the callee the trigger names.
	Trigger Kind = "trigger"
	// Request: a synchronous `asks`.
	Request Kind = "request"
	// Response: a `returns`, to its explicit target or else to the caller.
	Response Kind = "response"
	// Publish: a `notifies`; To is zero and Event names the event.
	Publish Kind = "publish"
	// Listen: an event delivered from one publisher to one listener: one
	// per event of the trigger's list and per publisher of that event.
	Listen Kind = "listen"
	// Internal: a participant acting on its own; From == To.
	Internal Kind = "internal"
)

// Endpoint is one side of an interaction.
type Endpoint struct {
	// Key identifies the endpoint: the participant's canonical ID, or the
	// name as written when it does not resolve (unresolved names are still
	// drawn — an unresolved participant is only a warning).
	Key string
	// Name is the display name: a bounded context's bare name, qualified
	// only when two endpoints share it; otherwise the canonical or written
	// name.
	Name string
	// Kind is "" when the name does not resolve.
	Kind    model.ParticipantKind
	Service string
}

// IsZero reports whether e is unset.
func (e Endpoint) IsZero() bool { return e.Key == "" }

// External stands for whatever lies outside the model: the source of an event
// nobody in the model publishes, or the caller of a return with no caller.
var External = Endpoint{Key: "External", Name: "External"}

// Unobserved is the caller of a `receives` trigger: someone calls the boundary
// but the model does not know who. It is distinct from External (an event
// source outside the model) so views can tell the two apart.
var Unobserved = Endpoint{Key: "Unobserved", Name: "unobserved caller"}

// Site locates the subject of the action or trigger an interaction came from.
type Site struct {
	URI          string
	Line, Column int // 1-based; Column in UTF-16 units
	Length       int // UTF-16 length of the subject as written
}

// Interaction is one derived communication.
type Interaction struct {
	UseCase string
	// UseCaseIndex is the index of the use case in the derived doc's
	// UseCases: its exact identity within that doc, which UseCase cannot
	// give, because the same name can appear in several files (whose
	// scenario IDs, numbered per file, can collide too). A consumer that
	// filters doc.UseCases must renumber it.
	UseCaseIndex int
	Scenario     string
	// Step is the source position within the scenario: 0 for the trigger,
	// n for the n-th action. Views number what they draw themselves.
	Step      int
	Kind      Kind
	From, To  Endpoint
	Event     string
	Label     string
	Operation *model.Operation
	// Operations is copied from model.Action.Operations: each item of a
	// bracket that lists two or more; nil otherwise.
	Operations []model.Operation
	// OneOf is set on each Publish of a `notifies one of`: one of the
	// action's events is published per run, chosen at run time.
	OneOf bool
	// Inferred is copied from the action: the hop was inferred, not read in
	// code.
	Inferred bool
	Site     Site
	// Targeted is true for a Response written with an explicit `to` target,
	// as opposed to one sent back to the caller found on the call stack.
	Targeted bool
	// Stream is set on a Response from a `returns stream`.
	Stream bool
	// Branch is the path from the outermost block inward to the branch the
	// action sits in; nil outside blocks. Listen always has a nil Branch, and
	// so does the trigger of a `receives` scenario. Any other Trigger has one
	// only when its scenario opens with a block (its first step that is not a
	// note). An opening alt or opt gets one trigger per
	// branch, each reaching that branch's first action; when it is an opt,
	// the path where it is skipped also gets a Trigger with a nil Branch, to
	// the first action after the block (when there is one). An opening par
	// gets one unbranched Trigger to its first action in source order, plus
	// one inside each other branch whose first action has a different
	// subject. An opening loop gets one unbranched Trigger. A branch of an
	// opening alt or opt that itself opens with a par or loop is treated the
	// same way, at that branch's path, before the box. Each interaction owns
	// its slice.
	Branch []BranchRef
	// Via is the store of a store hand-off (Publish and Listen); zero
	// otherwise.
	Via Endpoint
	// Schedule is the polling cadence of a Listen from a scheduled listens
	// trigger, as "cron <expr>" or "every <dur>"; "" otherwise.
	Schedule string
}

// BranchRef locates an interaction inside one branch of a block.
type BranchRef struct {
	// Block is the model.Block.ID. It is unique within the block's source
	// file, hence also within a scenario; consumers compare it to detect a
	// change of block.
	Block string
	Kind  lang.BlockKind
	Label string
	Index int // 0-based branch index within the block
	// Split is copied from model.Block.Split: the branch is one step of a par
	// written without `and`.
	Split bool
}

// Derive lists every interaction of doc in source order: use case, scenario,
// step. A listener's interactions come after its trigger, one per publisher of
// the event, publishers in source order.
func Derive(doc *model.CraftDoc) []Interaction { return derive(doc, nil) }

// AnsweredRequests maps every `returns stream` action of doc to the ID of
// the request it answers (see model.Action.Answers), by the same call-stack
// walk Derive uses; "" when it answers the trigger's actor, the unobserved
// caller or nobody. The keys are the doc's own *Action pointers.
func AnsweredRequests(doc *model.CraftDoc) map[*model.Action]string {
	answers := map[*model.Action]string{}
	derive(doc, answers)
	return answers
}

// derive is Derive; a non-nil answers also collects, per streaming return,
// the request it answers.
func derive(doc *model.CraftDoc, answers map[*model.Action]string) []Interaction {
	publishers := map[string][]Endpoint{}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			for _, a := range sc.Actions() {
				if a.Type != lang.ActionAsync || a.Context == "" {
					continue
				}
				for _, ev := range a.EventList() {
					if ev != "" {
						publishers[ev] = appendEndpoint(publishers[ev], endpoint(a.Context, a.From))
					}
				}
			}
		}
	}

	var out []Interaction
	for i, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			out = append(out, deriveScenario(uc, i, sc, publishers, answers)...)
		}
	}
	nameEndpoints(out)
	return out
}

func deriveScenario(uc model.UseCase, ucIndex int, sc model.Scenario, publishers map[string][]Endpoint, answers map[*model.Action]string) []Interaction {
	var out []Interaction
	actions := sc.Actions()
	base := Interaction{UseCase: uc.Name, UseCaseIndex: ucIndex, Scenario: sc.ID}
	t := sc.Trigger
	triggerSite := Site{URI: uc.SourceURI, Line: t.Line, Column: t.Column}

	// The call stack answers "who does an untargeted return go back to". It
	// starts with the actor that triggered the scenario, calling the subject of
	// the first action. A scenario that opens with a block is triggered into
	// each branch instead (see walker.trigger); the stack keeps the frame of
	// the first action in source order for what follows the block.
	var callStack []frame
	var trigger *Interaction
	var reached string

	switch t.Type {
	case lang.TriggerExternal:
		if t.Actor == "" || isInsideSystem(t.Subject) {
			// A bounded context, domain or service in the actor slot is part
			// of the system starting its own work, not someone outside it.
			break
		}
		actor := endpoint(t.Actor, t.Subject)
		it := base
		it.Kind, it.From = Trigger, actor
		it.Label = triggerLabel(t)
		it.Site = triggerSite
		it.Site.Length = utf16Len(t.Actor)
		if t.ScheduleKind != "" {
			it.Site.Length = utf16Len(string(t.ScheduleKind)) // the keyword stands for CRON
		}
		ob := sc.OpeningBlock()
		if ob != nil && (ob.Kind == lang.BlockAlt || ob.Kind == lang.BlockOpt) {
			// Opening with an alt or opt: each branch that has a target gets
			// the trigger (walker.branchTrigger checks per branch). The stack
			// keeps the frame of the first action with a subject, for what
			// follows the block.
			trigger = &it
			for _, a := range actions {
				if a.Context != "" {
					callStack = append(callStack, frame{caller: actor, callee: endpoint(a.Context, a.From)})
					break
				}
			}
			break
		}
		// No opening block, or a par or loop: one ordinary trigger to the
		// first action, drawn before the block. A loop is not a repeated
		// trigger: the actor starts the work once.
		if len(actions) > 0 && actions[0].Context != "" {
			first := endpoint(actions[0].Context, actions[0].From)
			callStack = append(callStack, frame{caller: actor, callee: first})
			it.To = first
			out = append(out, it)
			reached = first.Key
		}
		if ob != nil && ob.Kind == lang.BlockPar {
			// Each par branch that starts at another participant still gets
			// a trigger of its own (spec §4.4).
			tmpl := it
			tmpl.To = Endpoint{}
			trigger = &tmpl
		} else {
			reached = ""
		}
	case lang.TriggerDomainListen:
		if t.Context == "" || len(t.EventList()) == 0 {
			break
		}
		listener := endpoint(t.Context, t.Subject)
		// A repeated item (duplicate-event-in-list warns) listens once, so
		// no view draws the same edge twice.
		seen := map[string]bool{}
		for _, ev := range t.EventList() {
			if seen[ev] {
				continue
			}
			seen[ev] = true
			sources := publishers[ev]
			if len(sources) == 0 {
				sources = []Endpoint{External}
			}
			for _, src := range sources {
				it := base
				it.Kind, it.From, it.To, it.Event, it.Label = Listen, src, listener, ev, ev
				it.Site = triggerSite
				it.Site.Length = utf16Len(t.Context)
				it.Schedule = scheduleLabel(t)
				it.Via = storeEndpoint(t.Store)
				out = append(out, it)
			}
		}
	case lang.TriggerReceive:
		if t.Context == "" {
			break
		}
		// The trigger names its target, so the first-action and per-branch
		// rules above do not apply: one trigger, drawn up front, whatever
		// the scenario opens with (trigger stays nil: no branch triggers).
		callee := endpoint(t.Context, t.Subject)
		it := base
		it.Kind, it.From, it.To = Trigger, Unobserved, callee
		it.Label = strings.Join(strings.Fields(t.Connector+" "+t.Phrase), " ")
		it.Operation = t.Operation
		it.Site = triggerSite
		it.Site.Length = utf16Len(t.Context)
		// An unaddressed reply from the callee answers the unobserved caller.
		callStack = []frame{{caller: Unobserved, callee: callee}}
		out = append(out, it)
	}

	w := walker{base: base, uri: uc.SourceURI, callStack: callStack, trigger: trigger, reached: reached, out: out, answers: answers}
	w.walk(sc.Steps, nil, trigger != nil)
	return w.out
}

// triggerLabel is the text on a trigger arrow: the schedule for a scheduled
// trigger (`cron 0 7 * * 1-5`, `every 5m`), else what the actor does.
func triggerLabel(t model.Trigger) string {
	if l := scheduleLabel(t); l != "" {
		return l
	}
	return strings.Join(strings.Fields(t.Verb+" "+t.Connector+" "+t.Phrase), " ")
}

// scheduleLabel is a trigger's schedule as `cron 0 7 * * 1-5` or `every 5m`,
// or "" when the trigger has none.
func scheduleLabel(t model.Trigger) string {
	if t.ScheduleKind == "" {
		return ""
	}
	return strings.TrimSpace(string(t.ScheduleKind) + " " + t.Schedule)
}

// walker visits a scenario's steps depth-first, in source order, numbering
// actions and keeping the call stack per branch.
type walker struct {
	base      Interaction
	uri       string
	callStack []frame
	// trigger, when set, is the trigger of a scenario that opens with a
	// block, To unset: each branch the scenario opens with gets a copy.
	trigger *Interaction
	// reached is the Key the trigger drawn before an opening par (or loop)
	// already reaches. A par branch whose first action has that subject needs
	// no trigger of its own (spec §4.4); "" outside such a block.
	reached string
	step    int
	out     []Interaction
	// answers, when non-nil, collects the request each streaming return
	// answers (AnsweredRequests); answered is that request for the action
	// just derived.
	answers  map[*model.Action]string
	answered string
}

// walk visits steps that sit at path. opening is true when the steps are where
// the scenario opens: the body of the scenario, or of a branch of a block that
// opens it. Each such body gets one trigger, to its first step that can take
// one: the action itself, or an inner block's branches. When an opening block
// reaches no action (every branch empty), the body's first action in source
// order takes the trigger instead. walk reports whether it emitted a trigger.
func (w *walker) walk(steps []model.Step, path []BranchRef, opening bool) (triggered bool) {
	pending := opening
	// skipped: the scenario opens with an opt, so the path where it is not
	// taken still needs a trigger, into the first action after the block.
	skipped := false
	first := model.FirstStepIndex(steps)
	for i, st := range steps {
		switch {
		case st.Note != nil:
			continue
		case st.Block != nil:
			if pending && i == first && w.boxTrigger(st.Block, path) {
				pending, triggered = false, true
			} else if pending && i == first {
				if w.walkBlock(st.Block, path, true) {
					pending, triggered = false, true
				}
				skipped = len(path) == 0 && w.trigger != nil && st.Block.Kind == lang.BlockOpt
			} else {
				w.walkBlock(st.Block, path, false)
			}
		case st.Action != nil:
			if pending && w.branchTrigger(*st.Action, path) {
				pending, triggered, skipped = false, true, false
			}
			if skipped && w.branchTrigger(*st.Action, nil) {
				skipped = false
			}
			// Keep in step with scenarioLayout.walk in
			// visualizer/view/sequence.go: both number actions in the same
			// order. TestSequenceStepsMatchInteractionSteps guards the pairing.
			w.step++
			w.action(*st.Action, path)
			if w.answers != nil && st.Action.Stream {
				w.answers[st.Action] = w.answered
			}
		}
	}
	return triggered
}

// boxTrigger triggers a body at path that opens with a par or loop the way a
// scenario that opens with one is triggered (spec §4.4): once, at path, to the
// block's first action, drawn before the box. A loop body is then walked
// without triggers; a par branch whose first action has another subject still
// gets one of its own (w.reached). It reports false, and does nothing, for any
// other block or when no trigger applies, so the caller falls back to the
// per-branch rule.
func (w *walker) boxTrigger(b *model.Block, path []BranchRef) bool {
	if w.trigger == nil || (b.Kind != lang.BlockPar && b.Kind != lang.BlockLoop) {
		return false
	}
	a := b.FirstAction()
	if a == nil || !w.branchTrigger(*a, path) {
		return false
	}
	saved := w.reached
	w.reached = endpoint(a.Context, a.From).Key
	w.walkBlock(b, path, b.Kind == lang.BlockPar)
	w.reached = saved
	return true
}

// branchTrigger draws the scenario's trigger into the branch at path, to a,
// that branch's first action. The branch's call stack starts with it alone.
//
// A branch trigger is how the sequence view knows where to draw it: it has
// Step 0 and a Branch. The view skips Step 0 triggers that have a Branch when
// it draws the step-0 interactions, and draws them when its walk enters that
// branch (scenarioLayout.emitTriggers in visualizer/view/sequence.go). An
// opening opt also gets an unbranched Step 0 trigger to the first action after
// it (the skipped path); the view draws that one just before that action
// (scenarioLayout.emitSkipTrigger).
func (w *walker) branchTrigger(a model.Action, path []BranchRef) bool {
	if w.trigger == nil || a.Context == "" {
		return false
	}
	if w.reached != "" && endpoint(a.Context, a.From).Key == w.reached {
		return true // already reached by the trigger drawn before the par
	}
	it := *w.trigger
	it.To = endpoint(a.Context, a.From)
	it.Branch = slices.Clone(path)
	w.callStack = []frame{{caller: it.From, callee: it.To}}
	w.out = append(w.out, it)
	return true
}

// walkBlock visits each branch of b. Every branch of an alt or a par starts
// from the call stack as it was at the block; an opt or loop body continues
// from it (a loop body is derived once: one iteration). After the block the
// stack is restored to its state at the block's start, because which branch
// ran, or how the concurrent ones interleaved, is unknown. It reports whether
// any branch got a trigger.
func (w *walker) walkBlock(b *model.Block, path []BranchRef, opening bool) (triggered bool) {
	saved := slices.Clone(w.callStack)
	for i, br := range b.Branches {
		if b.Kind == lang.BlockAlt || b.Kind == lang.BlockPar || i == 0 {
			w.callStack = slices.Clone(saved)
		}
		ref := BranchRef{Block: b.ID, Kind: b.Kind, Label: br.Label, Index: i, Split: b.Split}
		if w.walk(br.Steps, append(slices.Clone(path), ref), opening) {
			triggered = true
		}
	}
	w.callStack = saved
	return triggered
}

// action derives the interaction of the w.step-th action, if it has one.
func (w *walker) action(a model.Action, path []BranchRef) {
	if a.Context == "" {
		return
	}
	it := w.base
	it.Step = w.step
	it.Branch = slices.Clone(path)
	it.From = endpoint(a.Context, a.From)
	it.Label = label(a)
	it.Operation = a.Operation
	it.Operations = a.Operations
	it.Inferred = a.Inferred
	it.Site = Site{URI: w.uri, Line: a.Line, Column: a.Column, Length: utf16Len(a.Context)}
	switch a.Type {
	case lang.ActionSync:
		if a.TargetContext == "" {
			return
		}
		it.Kind, it.To = Request, endpoint(a.TargetContext, a.To)
		w.callStack = append(w.callStack, frame{caller: it.From, callee: it.To, request: a.ID})
	case lang.ActionAsync:
		// One Publish per event; a `notifies one of` publishes each of its
		// events at the same step (spec §8.5). A repeated item
		// (duplicate-event-in-list warns) publishes once.
		seen := map[string]bool{}
		for _, ev := range a.EventList() {
			if ev == "" || seen[ev] {
				continue
			}
			seen[ev] = true
			p := it
			p.Kind, p.Event, p.Label = Publish, ev, ev
			p.OneOf = len(a.Events) > 0
			p.Via = storeEndpoint(a.Store)
			p.Branch = slices.Clone(path)
			w.out = append(w.out, p)
		}
		return
	case lang.ActionInternal:
		it.Kind, it.To = Internal, it.From
	case lang.ActionReturn:
		it.Kind, it.Stream = Response, a.Stream
		switch {
		case a.TargetContext != "":
			it.To, it.Targeted = endpoint(a.TargetContext, a.To), true
			w.answered = openRequest(w.callStack, it.To, it.From)
		default:
			var settled frame
			it.To, w.callStack, settled = answer(w.callStack, it.From)
			w.answered = settled.request
		}
	default:
		return
	}
	w.out = append(w.out, it)
}

// frame is one outstanding call: caller asked callee and has not been
// answered yet.
type frame struct {
	caller, callee Endpoint
	request        string // the asks action's ID; "" for a trigger frame
}

// answer resolves an untargeted return by returner. It answers the most recent
// call made TO the returner, which also settles every call opened after it.
// A returner nobody called answers the latest caller other than itself, and
// with no such caller the answer leaves the model (External). It also
// returns the frame it settled (zero for External).
func answer(stack []frame, returner Endpoint) (Endpoint, []frame, frame) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].callee.Key == returner.Key {
			return stack[i].caller, stack[:i], stack[i]
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].caller.Key != returner.Key {
			return stack[i].caller, stack[:i], stack[i]
		}
	}
	return External, nil, frame{}
}

// openRequest returns the request ID of the most recent open call from
// caller to callee, without settling it; "" when there is none (plan
// correction 12: a targeted stream answers that call).
func openRequest(stack []frame, caller, callee Endpoint) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].caller.Key == caller.Key && stack[i].callee.Key == callee.Key {
			return stack[i].request
		}
	}
	return ""
}

// label is the action's text as diagrams show it.
func label(a model.Action) string {
	withConnector := func(phrase string) string {
		if a.Connector != "" && phrase != "" {
			return a.Connector + " " + phrase
		}
		return phrase
	}
	switch a.Type {
	case lang.ActionSync:
		return withConnector(a.Phrase)
	case lang.ActionAsync:
		return a.Event
	case lang.ActionInternal:
		if a.Verb != "" {
			return strings.TrimSpace(a.Verb + " " + withConnector(a.Phrase))
		}
		return withConnector(a.Phrase)
	case lang.ActionReturn:
		if a.Stream {
			return strings.TrimSpace("streams " + withConnector(a.Phrase))
		}
		return strings.TrimSpace("returns " + withConnector(a.Phrase))
	}
	return ""
}

func endpoint(written string, p *model.Participant) Endpoint {
	if p == nil {
		return Endpoint{Key: written, Name: written}
	}
	return Endpoint{Key: p.ID, Name: p.ID, Kind: p.Kind, Service: p.Service}
}

// storeEndpoint is the data-store endpoint of a store hand-off: the declared
// data store when the name resolves, else "store.<name>". Kind is always
// data_store, so views know to draw a store.
func storeEndpoint(s *model.StoreRef) Endpoint {
	if s == nil {
		return Endpoint{}
	}
	if p := s.Participant; p != nil {
		return Endpoint{Key: p.ID, Name: p.ID, Kind: model.ParticipantDataStore, Service: p.Service}
	}
	return Endpoint{Key: lang.StoreTransport + "." + s.Name, Name: s.Name, Kind: model.ParticipantDataStore}
}

func isInsideSystem(p *model.Participant) bool {
	if p == nil {
		return false
	}
	switch p.Kind {
	case model.ParticipantBoundedContext, model.ParticipantDomain, model.ParticipantService:
		return true
	}
	return false
}

func appendEndpoint(list []Endpoint, e Endpoint) []Endpoint {
	for _, existing := range list {
		if existing.Key == e.Key {
			return list
		}
	}
	return append(list, e)
}

// nameEndpoints sets display names: a bounded context shows its bare name
// unless another drawn bounded context has the same one.
func nameEndpoints(its []Interaction) {
	keysByBare := map[string]map[string]bool{}
	visit := func(e Endpoint) {
		if e.Kind != model.ParticipantBoundedContext {
			return
		}
		bare := bareName(e.Key)
		if keysByBare[bare] == nil {
			keysByBare[bare] = map[string]bool{}
		}
		keysByBare[bare][e.Key] = true
	}
	for _, it := range its {
		visit(it.From)
		visit(it.To)
	}
	rename := func(e *Endpoint) {
		if e.Kind == model.ParticipantBoundedContext && len(keysByBare[bareName(e.Key)]) == 1 {
			e.Name = bareName(e.Key)
		}
	}
	for i := range its {
		rename(&its[i].From)
		rename(&its[i].To)
	}
}

func bareName(id string) string {
	if i := strings.LastIndexByte(id, '/'); i >= 0 {
		return id[i+1:]
	}
	return id
}

func utf16Len(s string) int { return int(green.UTF16Len(s)) }
