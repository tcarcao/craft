// Package interaction derives, once, every communication a model describes:
// who triggers, asks, answers, publishes and listens to whom. Diagrams and the
// context-map lints read this list instead of each walking the use cases with
// their own rules.
//
// It depends only on the document model: names arrive already resolved
// (model.Participant), so this package never matches names itself.
package interaction

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Kind classifies an interaction.
type Kind string

const (
	// Trigger: an actor outside the system starts a scenario; To is the
	// subject of the scenario's first action.
	Trigger Kind = "trigger"
	// Request: a synchronous `asks`.
	Request Kind = "request"
	// Response: a `returns`, to its explicit target or else to the caller.
	Response Kind = "response"
	// Publish: a `notifies`; To is zero and Event names the event.
	Publish Kind = "publish"
	// Listen: an event delivered from one publisher to one listener.
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

// Site locates the subject of the action or trigger an interaction came from.
type Site struct {
	URI          string
	Line, Column int // 1-based; Column in UTF-16 units
	Length       int // UTF-16 length of the subject as written
}

// Interaction is one derived communication.
type Interaction struct {
	UseCase  string
	Scenario string
	// Step is the source position within the scenario: 0 for the trigger,
	// n for the n-th action. Views number what they draw themselves.
	Step      int
	Kind      Kind
	From, To  Endpoint
	Event     string
	Label     string
	Operation *model.Operation
	Site      Site
	// Targeted is true for a Response written with an explicit `to` target,
	// as opposed to one sent back to the caller found on the call stack.
	Targeted bool
}

// Derive lists every interaction of doc in source order: use case, scenario,
// step. A listener's interactions come after its trigger, one per publisher of
// the event, publishers in source order.
func Derive(doc *model.CraftDoc) []Interaction {
	publishers := map[string][]Endpoint{}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			for _, a := range sc.Actions {
				if a.Type == lang.ActionAsync && a.Context != "" && a.Event != "" {
					publishers[a.Event] = appendEndpoint(publishers[a.Event], endpoint(a.Context, a.From))
				}
			}
		}
	}

	var out []Interaction
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			out = append(out, deriveScenario(uc, sc, publishers)...)
		}
	}
	nameEndpoints(out)
	return out
}

func deriveScenario(uc model.UseCase, sc model.Scenario, publishers map[string][]Endpoint) []Interaction {
	var out []Interaction
	base := Interaction{UseCase: uc.Name, Scenario: sc.ID}
	t := sc.Trigger
	triggerSite := Site{URI: uc.SourceURI, Line: t.Line, Column: t.Column}

	// The call stack answers "who does an untargeted return go back to". It
	// starts with the actor that triggered the scenario, calling the subject of
	// the first action.
	var callStack []frame

	switch t.Type {
	case lang.TriggerExternal:
		if t.Actor == "" || isInsideSystem(t.Subject) {
			// A bounded context, domain or service in the actor slot is part
			// of the system starting its own work, not someone outside it.
			break
		}
		actor := endpoint(t.Actor, t.Subject)
		if len(sc.Actions) > 0 && sc.Actions[0].Context != "" {
			first := sc.Actions[0]
			it := base
			it.Kind, it.From, it.To = Trigger, actor, endpoint(first.Context, first.From)
			callStack = append(callStack, frame{caller: actor, callee: it.To})
			it.Label = strings.Join(strings.Fields(t.Verb+" "+t.Connector+" "+t.Phrase), " ")
			it.Site = triggerSite
			it.Site.Length = utf16Len(t.Actor)
			out = append(out, it)
		}
	case lang.TriggerDomainListen:
		if t.Context == "" || t.Event == "" {
			break
		}
		listener := endpoint(t.Context, t.Subject)
		sources := publishers[t.Event]
		if len(sources) == 0 {
			sources = []Endpoint{External}
		}
		for _, src := range sources {
			it := base
			it.Kind, it.From, it.To, it.Event, it.Label = Listen, src, listener, t.Event, t.Event
			it.Site = triggerSite
			it.Site.Length = utf16Len(t.Context)
			out = append(out, it)
		}
	}

	for i, a := range sc.Actions {
		if a.Context == "" {
			continue
		}
		it := base
		it.Step = i + 1
		it.From = endpoint(a.Context, a.From)
		it.Label = label(a)
		it.Operation = a.Operation
		it.Site = Site{URI: uc.SourceURI, Line: a.Line, Column: a.Column, Length: utf16Len(a.Context)}
		switch a.Type {
		case lang.ActionSync:
			if a.TargetContext == "" {
				continue
			}
			it.Kind, it.To = Request, endpoint(a.TargetContext, a.To)
			callStack = append(callStack, frame{caller: it.From, callee: it.To})
		case lang.ActionAsync:
			if a.Event == "" {
				continue
			}
			it.Kind, it.Event = Publish, a.Event
		case lang.ActionInternal:
			it.Kind, it.To = Internal, it.From
		case lang.ActionReturn:
			it.Kind = Response
			switch {
			case a.TargetContext != "":
				it.To, it.Targeted = endpoint(a.TargetContext, a.To), true
			default:
				it.To, callStack = answer(callStack, it.From)
			}
		default:
			continue
		}
		out = append(out, it)
	}
	return out
}

// frame is one outstanding call: caller asked callee and has not been
// answered yet.
type frame struct{ caller, callee Endpoint }

// answer resolves an untargeted return by returner. It answers the most recent
// call made TO the returner, which also settles every call opened after it.
// A returner nobody called answers the latest caller other than itself, and
// with no such caller the answer leaves the model (External).
func answer(stack []frame, returner Endpoint) (Endpoint, []frame) {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].callee.Key == returner.Key {
			return stack[i].caller, stack[:i]
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].caller.Key != returner.Key {
			return stack[i].caller, stack[:i]
		}
	}
	return External, nil
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
