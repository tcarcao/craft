// Package model holds the canonical Craft document data types. It is a leaf
// package (imports nothing from internal/ or pkg/) so that internal/syntax and
// internal/sema can produce these types without importing pkg/craft, which
// re-exports them as the stable public API.
package model

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/lang"
)

// CraftDoc is the parser-agnostic canonical representation of a parsed .craft file.
// It is re-exported from pkg/craft as the stable public API type; see that
// package's stability contract.
type CraftDoc struct {
	Architectures []ArchBlock    `json:"architectures,omitempty"`
	Exposures     []Exposure     `json:"exposures,omitempty"`
	Services      []Service      `json:"services,omitempty"`
	UseCases      []UseCase      `json:"useCases"`
	Domains       []Domain       `json:"domains,omitempty"`
	Actors        []Actor        `json:"actors,omitempty"`
	ContextMap    []Edge         `json:"contextMap,omitempty"`
	Glossary      []TermRelation `json:"glossary,omitempty"`
	Surfaces      []Surface      `json:"surfaces,omitempty"`
}

// Edge is one authored relationship edge from a context_map block, connecting
// two bounded-context endpoints (bare or domain-qualified names, e.g.
// "re/billing") with a DDD strategic relationship verb (e.g.
// "customer_supplier"). Endpoint resolution/validation is a sema concern, not
// captured here — this is a shape-only projection of the parsed edges.
type Edge struct {
	Left  string `json:"left"`
	Verb  string `json:"verb"`
	Right string `json:"right"`
}

// TermRelation is one authored cross-context term relation from a glossary
// block, connecting two term nodes (bare or domain-qualified, e.g.
// "billing/Invoice") with a symmetric relation verb (same_as/contrasts/
// distinct_from). Resolution/validation is a sema concern; this is a
// shape-only projection.
type TermRelation struct {
	Left  string `json:"left"`
	Verb  string `json:"verb"`
	Right string `json:"right"`
}

// ArchBlock represents a named architecture with presentation and gateway components.
type ArchBlock struct {
	Name         string      `json:"name,omitempty"`
	Presentation []Component `json:"presentation"`
	Gateway      []Component `json:"gateway"`
}

// Component is a named element within an architecture layer.
type Component struct {
	Name      string              `json:"name"`
	Type      ComponentType       `json:"type"`
	Modifiers []ComponentModifier `json:"modifiers,omitempty"`
	Chain     []Component         `json:"chain,omitempty"`
}

// ComponentType classifies a component as simple or flow.
type ComponentType string

const (
	ComponentTypeSimple ComponentType = "simple"
	ComponentTypeFlow   ComponentType = "flow"
)

// ComponentModifier is a key-value pair that modifies a component's behaviour.
type ComponentModifier struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Exposure describes how a domain or service is exposed to consumers.
type Exposure struct {
	Name     string   `json:"name"`
	To       []string `json:"to,omitempty"`
	Contexts []string `json:"contexts,omitempty"`
	Through  []string `json:"through,omitempty"`
}

// Service represents a deployable service in the architecture.
type Service struct {
	Name       string             `json:"name"`
	Contexts   []string           `json:"contexts,omitempty"`
	DataStores []string           `json:"dataStores,omitempty"`
	Language   string             `json:"language,omitempty"`
	Deployment DeploymentStrategy `json:"deployment,omitempty"`
	// CatalogRef is the service's stable identifier in the org's service
	// catalog — an identity anchor, never a location. The language does not
	// name the catalog vendor; which catalog resolves it is deployment
	// configuration. Empty when the service declares no catalog_ref:.
	CatalogRef string `json:"catalogRef,omitempty"`
	// Repo is the source repository reference (a repo slug, not a file path).
	Repo string `json:"repo,omitempty"`
	Line int    `json:"line,omitempty"`
	// ContextIDs holds, position for position with Contexts, each entry's
	// canonical identity: "<domain>/<bc>" for a bounded context, the name for
	// a domain, "" when it does not resolve. Nil when no entry resolves.
	ContextIDs []string `json:"contextIds,omitempty"`
}

// DeploymentStrategy describes how a service is deployed.
type DeploymentStrategy struct {
	Type  string           `json:"type,omitempty"`
	Rules []DeploymentRule `json:"rules,omitempty"`
}

// DeploymentRule defines a percentage-based deployment target.
type DeploymentRule struct {
	Percentage string `json:"percentage"`
	Target     string `json:"target"`
}

// UseCase groups related scenarios under a named use case.
type UseCase struct {
	Name      string     `json:"name"`
	Scenarios []Scenario `json:"scenarios"`
	// Line is the 1-based source line of the use_case's title token, 0 if unknown.
	Line int `json:"line,omitempty"`
	// SourceURI is the originating file (ParseFiles map key / Parse filename).
	// Empty when the caller supplied none.
	SourceURI string `json:"sourceUri,omitempty"`
	// Tags holds the key/value pairs from an optional tags { } sub-block
	// (Slice B). nil when the use_case has no tags block; last-write-wins on
	// duplicate keys within the block (sema validation is a separate task).
	//
	// A tag value is a comma-separated list, and this field carries it joined
	// with ", " so that every consumer written against the single-valued shape
	// keeps working unchanged. The joining is lossy in exactly one way:
	// `channels: web, mobile` and `channels: "web, mobile"` both land here as
	// `web, mobile`. Read TagValues when that difference matters.
	Tags map[string]string `json:"tags,omitempty"`
	// TagValues holds the same tags unjoined, one entry per authored value.
	// Every key present in Tags is present here, single values included, as a
	// one-element slice — so a consumer can read this field uniformly rather
	// than testing which of the two to use. nil exactly when Tags is nil.
	TagValues map[string][]string `json:"tagValues,omitempty"`
	// Notes are the use-case-level note and finding lines, written before the
	// first scenario.
	Notes []Note `json:"notes,omitempty"`
	// Sources are the `//@` cites written before the first scenario.
	Sources []Source `json:"sources,omitempty"`
}

// Scenario is one trigger and the steps it sets off, in source order.
type Scenario struct {
	ID string `json:"id"`
	// Name is the label written in `scenario "…"` before the trigger; "" when
	// the scenario is unnamed.
	Name    string  `json:"name,omitempty"`
	Trigger Trigger `json:"trigger"`
	Steps   []Step  `json:"steps"`
}

// Step is exactly one of Action, Block, Note or Link.
type Step struct {
	Action *Action `json:"action,omitempty"`
	Block  *Block  `json:"block,omitempty"`
	Note   *Note   `json:"note,omitempty"`
	Link   *Link   `json:"link,omitempty"`
}

// Source is one cite of a `//@` line (spec §14.2).
type Source struct {
	Repo  string `json:"repo,omitempty"` // as written; "" when the cite names none
	Path  string `json:"path"`
	Start int    `json:"start,omitempty"` // first cited line, 1-based; 0 when no line is cited
	End   int    `json:"end,omitempty"`   // last cited line; equal to Start for one line
	Text  string `json:"text,omitempty"`  // the text after " -- ", shared by the cites of its line
	Line  int    `json:"line,omitempty"`  // the line of the //@ comment in the .craft file
}

// UseCaseRef names one use case of the workspace.
type UseCaseRef struct {
	Name      string `json:"name"`
	SourceURI string `json:"sourceUri,omitempty"`
}

// LinkKind is the kind of a link step.
type LinkKind = lang.LinkKind

const (
	LinkIncludes  = lang.LinkIncludes
	LinkContinues = lang.LinkContinues
)

// Link is an `includes "<title>"` or `continues "<title>"` step. Like a
// note, it is not an action: no interaction, no step number.
type Link struct {
	ID      string      `json:"id"`               // link_<n>
	Kind    LinkKind    `json:"kind"`             // "includes" | "continues"
	Title   string      `json:"title"`            // as written
	Target  *UseCaseRef `json:"target,omitempty"` // nil when it does not resolve
	Sources []Source    `json:"sources,omitempty"`
	Line    int         `json:"line,omitempty"`
	Column  int         `json:"column,omitempty"` // of the title string
}

// Tag keys with a meaning in v2.25 (see lang).
const (
	TagJourney         = lang.TagJourney
	TagFormerlyTitled  = lang.TagFormerlyTitled
	TagFormerlyJourney = lang.TagFormerlyJourney
)

// JourneyEdgeKind is what connects two members of a journey spine.
type JourneyEdgeKind = lang.JourneyEdgeKind

const (
	JourneyEdgeEvent     = lang.JourneyEdgeEvent
	JourneyEdgeStore     = lang.JourneyEdgeStore
	JourneyEdgeContinues = lang.JourneyEdgeContinues
)

// JourneyMember is one use case of a journey spine.
type JourneyMember struct {
	UseCase   int    `json:"useCase"` // index into CraftDoc.UseCases
	Name      string `json:"name"`
	SourceURI string `json:"sourceUri,omitempty"`
}

// JourneyEdge connects two members: From's publish, store write or
// continues link reaches To. From and To index Journey.Members.
type JourneyEdge struct {
	From  int             `json:"from"`
	To    int             `json:"to"`
	Kind  JourneyEdgeKind `json:"kind"`
	Event string          `json:"event,omitempty"` // the event or store ref; "" for continues
}

// Journey is the spine of one journey slug: its members in spine order.
type Journey struct {
	Slug    string          `json:"slug"`
	Members []JourneyMember `json:"members"`
	Edges   []JourneyEdge   `json:"edges,omitempty"`
	Cycle   bool            `json:"cycle,omitempty"`
}

// BlockKind is the kind of a branch block inside a scenario.
type BlockKind = lang.BlockKind

const (
	BlockAlt  = lang.BlockAlt
	BlockOpt  = lang.BlockOpt
	BlockPar  = lang.BlockPar
	BlockLoop = lang.BlockLoop
)

// Block is a branch block inside a scenario: an `alt` (one of several
// branches), an `opt` (one conditional branch), a `par` (every branch runs,
// concurrently) or a `loop` (one branch that repeats).
type Block struct {
	ID       string    `json:"id"`
	Kind     BlockKind `json:"kind"`
	Line     int       `json:"line,omitempty"`
	Column   int       `json:"column,omitempty"`
	Branches []Branch  `json:"branches"`
	// Split is true for a par written without `and`: each Branch holds one step
	// of the header's body, and every Branch carries the header's Label.
	Split bool `json:"split,omitempty"`
}

// NoteKind is the kind of a note statement (note or finding).
type NoteKind = lang.NoteKind

const (
	NoteKindNote    = lang.NoteText
	NoteKindFinding = lang.NoteFinding
)

// Note is a `note "…"` or `finding "…"` line. It is not an action: it gets no
// interaction and does not advance step numbers.
type Note struct {
	ID     string   `json:"id,omitempty"` // "note_<n>" inside a use case; "" in a surface
	Kind   NoteKind `json:"kind"`         // "note" | "finding"
	Text   string   `json:"text"`         // unquoted
	Line   int      `json:"line,omitempty"`
	Column int      `json:"column,omitempty"`
	// Sources are the `//@` cites under the note.
	Sources []Source `json:"sources,omitempty"`
}

// StoreChange is the change of a store hand-off.
type StoreChange = lang.StoreChange

const (
	StoreInserted = lang.StoreInserted
	StoreUpdated  = lang.StoreUpdated
	StoreDeleted  = lang.StoreDeleted
)

// StoreRef is a store hand-off parsed from a store.<name>.<change> ref.
type StoreRef struct {
	Name   string      `json:"name"`
	Change StoreChange `json:"change,omitempty"`
	// Participant is the data store the name resolves to (a `data-stores:`
	// entry, rulings 9 and 11); nil when it does not resolve.
	Participant *Participant `json:"participant,omitempty"`
}

// SurfaceOpKind is the kind of a surface operation (query or command).
type SurfaceOpKind = lang.SurfaceOpKind

const (
	SurfaceQuery   = lang.SurfaceQuery
	SurfaceCommand = lang.SurfaceCommand
)

// UnknownCaller is the `callers:` value that says no caller was observed.
const UnknownCaller = lang.UnknownCaller

// Surface lists the operations one boundary offers (`surface <boundary> { … }`).
type Surface struct {
	Subject     string             `json:"subject"`               // as written
	Participant *Participant       `json:"participant,omitempty"` // resolved boundary
	Callers     []string           `json:"callers,omitempty"`     // as written; "unknown" kept literally
	Spec        string             `json:"spec,omitempty"`
	Notes       []Note             `json:"notes,omitempty"`
	Operations  []SurfaceOperation `json:"operations"`
	SourceURI   string             `json:"sourceUri,omitempty"`
	Line        int                `json:"line,omitempty"`
}

// SurfaceBoundary returns the ID of the surface that covers requests to p:
// p's own surface when it has one, otherwise, for a bounded context, the
// surface of the service that deploys it (spec §8.4). hasSurface reports
// whether a surface exists for a resolved boundary ID. ok is false when no
// surface covers p.
func SurfaceBoundary(p Participant, hasSurface func(id string) bool) (id string, ok bool) {
	if p.ID == "" {
		return "", false
	}
	if hasSurface(p.ID) {
		return p.ID, true
	}
	if p.Kind == ParticipantBoundedContext && p.Service != "" && hasSurface(p.Service) {
		return p.Service, true
	}
	return "", false
}

// SurfaceOperation is one `query`/`command` line of a surface.
type SurfaceOperation struct {
	Kind      SurfaceOpKind `json:"kind"` // "query" | "command"
	Connector string        `json:"connector,omitempty"`
	Phrase    string        `json:"phrase"`
	Operation Operation     `json:"operation"` // same parsing as an action's bracket
	Line      int           `json:"line,omitempty"`
}

// OpeningBlock returns the block the scenario opens with: its first step that
// is not a note or a link, when that step is a block. Nil otherwise.
func (s Scenario) OpeningBlock() *Block {
	if i := FirstStepIndex(s.Steps); i >= 0 {
		return s.Steps[i].Block
	}
	return nil
}

// FirstStepIndex returns the index of the first step that is not a note or a
// link, or -1 when there is none. It is "the first step" of a scenario or branch
// wherever opening rules apply (OpeningBlock, triggers).
func FirstStepIndex(steps []Step) int {
	for i, st := range steps {
		if st.Note == nil && st.Link == nil {
			return i
		}
	}
	return -1
}

// FirstAction returns the block's first action in source order (a
// depth-first walk of its branches) that has a subject, or nil.
func (b *Block) FirstAction() *Action {
	var first *Action
	for i := range b.Branches {
		eachAction(b.Branches[i].Steps, func(a *Action) {
			if first == nil && a.Context != "" {
				first = a
			}
		})
	}
	return first
}

// Branch is one labelled path of a Block.
type Branch struct {
	Label string `json:"label"`
	Line  int    `json:"line,omitempty"`
	Steps []Step `json:"steps"`
	// Sources are the `//@` cites under the branch header.
	Sources []Source `json:"sources,omitempty"`
}

// EventList returns Events, or a one-element slice holding Event, or nil
// when the trigger names no event.
func (t Trigger) EventList() []string { return eventList(t.Events, t.Event) }

// EventList returns Events, or a one-element slice holding Event, or nil
// when the action names no event.
func (a Action) EventList() []string { return eventList(a.Events, a.Event) }

func eventList(events []string, event string) []string {
	if len(events) > 0 {
		return events
	}
	if event == "" {
		return nil
	}
	return []string{event}
}

// OperationList returns Operations, or a one-element slice holding
// *Operation, or nil when the action has no bracket.
func (a Action) OperationList() []Operation {
	if len(a.Operations) > 0 {
		return a.Operations
	}
	if a.Operation == nil {
		return nil
	}
	return []Operation{*a.Operation}
}

// Tables returns the table names of a SQL operation (its payload fields),
// and nil for any other verb.
func (o Operation) Tables() []string {
	if o.Verb != lang.OpVerbSQL {
		return nil
	}
	return strings.Fields(o.Payload)
}

// Actions returns every action of the scenario in source order: a depth-first
// walk of Steps. Notes are skipped. The result holds copies; use EachAction to
// update in place.
func (s Scenario) Actions() []Action {
	var out []Action
	eachAction(s.Steps, func(a *Action) { out = append(out, *a) })
	return out
}

// EachAction calls fn with a pointer to every action of the scenario in
// source order, so callers can update actions in place. The receiver is a
// value, but Steps holds *Action pointers shared with the original, so
// mutations through fn reach the scenario's tree.
func (s Scenario) EachAction(fn func(*Action)) {
	eachAction(s.Steps, fn)
}

func eachAction(steps []Step, fn func(*Action)) {
	for _, st := range steps {
		if st.Action != nil {
			fn(st.Action)
		}
		if st.Block != nil {
			for i := range st.Block.Branches {
				eachAction(st.Block.Branches[i].Steps, fn)
			}
		}
	}
}

// Trigger describes what initiates a scenario.
type Trigger struct {
	Type  TriggerType `json:"type"`
	Actor string      `json:"actor,omitempty"`
	Verb  string      `json:"verb,omitempty"`
	// Connector is the connector word between the verb and the phrase of an
	// external trigger (`places an order` → "an"); Phrase excludes it.
	Connector string `json:"connector,omitempty"`
	Phrase    string `json:"phrase,omitempty"`
	Context   string `json:"context,omitempty"`
	Event     string `json:"event,omitempty"`
	// Ref holds the full typed-ref text (e.g. "vas.VasApplied" or
	// "bc:re/billing") for a domain_listen trigger whose event was written
	// as a ref rather than a quoted string. Empty for the legacy
	// `listens "X"` form and for non-domain_listen triggers — Event still
	// carries the value either way, for existing consumers.
	Ref string `json:"ref,omitempty"`
	// Events holds every event of a listens list, in source order, with the
	// same text Event would hold for each. Nil for a single event: read Event.
	// Event and Ref stay the first item.
	Events []string `json:"events,omitempty"`
	// Schedule is the expression of a `when cron "…"` or `when every "…"`
	// trigger, unquoted. It is also set for a `listens` trigger with a
	// trailing schedule.
	Schedule string `json:"schedule,omitempty"`
	// ScheduleKind is "cron" or "every" for a scheduled trigger, "" otherwise.
	ScheduleKind ScheduleKind `json:"scheduleKind,omitempty"`
	// Operation is the `[...]` bracket of a `receives` trigger; nil otherwise.
	Operation *Operation `json:"operation,omitempty"`
	// Store is set on a `listens` trigger whose ref is a store ref.
	Store       *StoreRef `json:"store,omitempty"`
	Description string    `json:"description"`
	// Subject is what the trigger's actor (external) or context (domain
	// listen) resolves to across the whole model. Nil when it does not
	// resolve, and for event triggers.
	Subject *Participant `json:"subject,omitempty"`
	// Line is the 1-based source line of the `when` keyword; Column is the
	// 1-based UTF-16 column of the subject (0 for an event trigger).
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	// Sources are the `//@` cites under the trigger line.
	Sources []Source `json:"sources,omitempty"`
}

// Participant is the resolved identity behind a name used in a use case. It is
// filled in by workspace analysis, so it reflects every file of the model.
type Participant struct {
	Kind ParticipantKind `json:"kind"`
	// ID is the canonical name: "<domain>/<bc>" for a bounded context, the
	// declared name for anything else.
	ID string `json:"id"`
	// Service is the service that deploys a bounded context or owns a data
	// store, or the service itself; empty when there is none.
	Service string `json:"service,omitempty"`
}

// ParticipantKind is what a use-case name resolves to.
type ParticipantKind = lang.ParticipantKind

const (
	ParticipantBoundedContext = lang.ParticipantBoundedContext
	ParticipantDomain         = lang.ParticipantDomain
	ParticipantService        = lang.ParticipantService
	ParticipantActor          = lang.ParticipantActor
	ParticipantDataStore      = lang.ParticipantDataStore
)

// ScheduleKind is the form of a scheduled trigger (cron or every).
type ScheduleKind = lang.ScheduleKind

const (
	ScheduleCron  = lang.ScheduleCron
	ScheduleEvery = lang.ScheduleEvery
)

// TriggerType classifies the origin of a scenario trigger.
type TriggerType = lang.TriggerKind

const (
	TriggerTypeExternal     = lang.TriggerExternal
	TriggerTypeEvent        = lang.TriggerEvent
	TriggerTypeDomainListen = lang.TriggerDomainListen
	TriggerTypeReceive      = lang.TriggerReceive
)

// Operation is the trailing `[...]` operation annotation on an action line, for
// example `[POST /v1/charges]`. It records the wire call an action corresponds
// to, which makes a use_case readable as an integration contract.
//
// Verb is a recognised protocol verb (GET/POST/GRPC/TOPIC/...) or "" when the
// annotation body did not start with one. Payload is the body with the verb
// stripped. Text is the whole body verbatim, and is always populated.
type Operation struct {
	Verb    string `json:"verb,omitempty"`
	Payload string `json:"payload,omitempty"`
	Text    string `json:"text"`
}

// Action is a step that takes place within a scenario.
type Action struct {
	ID            string     `json:"id"`
	Type          ActionType `json:"type"`
	Context       string     `json:"context"`
	Verb          string     `json:"verb,omitempty"`
	TargetContext string     `json:"targetContext,omitempty"`
	Event         string     `json:"event,omitempty"`
	// Ref holds the full typed-ref text (e.g. "vas.VasFulfilled" or
	// "bc:re/billing") when the sync_action target or async_action (notifies)
	// event was written as a ref rather than a quoted string / plain name.
	// Empty for the legacy quoted `notifies "X"` form and for
	// return_action/internal_action — TargetContext/Event still carry the
	// value either way, for existing consumers.
	Ref string `json:"ref,omitempty"`
	// Events holds every event of a "notifies one of" list, in source order;
	// nil for a single event.
	Events []string `json:"events,omitempty"`
	// Operation is nil when the action carries no `[...]` annotation. It is a
	// pointer with omitempty so an absent annotation emits no JSON key at all,
	// keeping .craftjson goldens for unannotated files byte-identical.
	Operation *Operation `json:"operation,omitempty"`
	// Operations holds each item of a bracket that lists two or more, in
	// order. Nil for a single-item bracket: read Operation, which keeps the
	// whole bracket text.
	Operations []Operation `json:"operations,omitempty"`
	// Store is set on an async action whose ref is a store ref.
	Store     *StoreRef `json:"store,omitempty"`
	Connector string    `json:"connector,omitempty"`
	Phrase    string    `json:"phrase,omitempty"`
	// Stream is set on a `returns stream` action: a reply that repeats.
	Stream bool `json:"stream,omitempty"`
	// Answers is the ID of the request a stream answers (the asks whose
	// call-stack frame the reply settles); "" when it answers the trigger's
	// actor, the unobserved caller or nobody. Set only when Stream is true.
	Answers string `json:"answers,omitempty"`
	// Inferred is set when the bracket opens with `inferred`: the hop was
	// inferred from names, configuration or convention, not read in code.
	Inferred    bool   `json:"inferred,omitempty"`
	Description string `json:"description"`
	Line        int    `json:"line,omitempty"`
	// Column is the 1-based UTF-16 column of the subject on Line.
	Column int `json:"column,omitempty"`
	// From is what the subject (Context) resolves to; To is what the target
	// (TargetContext) of a sync or return action resolves to. Both are nil
	// when the name does not resolve, and To is nil for other action kinds.
	From *Participant `json:"from,omitempty"`
	To   *Participant `json:"to,omitempty"`
	// Sources are the `//@` cites under the action line.
	Sources []Source `json:"sources,omitempty"`
}

// ActionType classifies the interaction pattern of an action.
type ActionType = lang.ActionKind

const (
	ActionTypeSync     = lang.ActionSync
	ActionTypeAsync    = lang.ActionAsync
	ActionTypeInternal = lang.ActionInternal
	ActionTypeReturn   = lang.ActionReturn
)

// Interaction is a derived type computed from scenario actions; it represents a directed
// communication between two parties in a sequence diagram. Not a top-level CraftDoc field —
// produced by the visualizer layer from CraftDoc.UseCases.
type Interaction struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Type        string `json:"type"`
	Description string `json:"description"`
	UseCase     string `json:"useCase"`
	ScenarioID  string `json:"scenarioId"`
}

// Domain groups related bounded contexts.
type Domain struct {
	Name            string   `json:"name"`
	BoundedContexts []string `json:"boundedContexts"`
}

// Actor is an entity outside the system that interacts with it: a person
// (user), an external system or service, a scheduler, a tool or platform
// infrastructure.
type Actor struct {
	Name string    `json:"name"`
	Type ActorType `json:"type"`
	Line int       `json:"line,omitempty"`
}

// ActorType classifies an actor.
type ActorType string

const (
	ActorTypeUser      ActorType = lang.ActorUser
	ActorTypeSystem    ActorType = lang.ActorSystem
	ActorTypeService   ActorType = lang.ActorService
	ActorTypeScheduler ActorType = lang.ActorScheduler
	ActorTypeTool      ActorType = lang.ActorTool
	ActorTypeInfra     ActorType = lang.ActorInfra
)

// KnownActorTypes is user, system, service, scheduler, tool, infra.
func KnownActorTypes() []string { return lang.KnownActorTypes() }

// TagSites and TagEnvs are the scope tag keys the views print.
const (
	TagSites = lang.TagSites
	TagEnvs  = lang.TagEnvs
)

// Diagnostic is a structured error/warning emitted by the v2 parser or sema
// layer. It follows Q19: assertions are on Code + Range + Severity; Message
// is free-form prose.
type Diagnostic struct {
	// Code is a stable, machine-readable identifier (e.g. craft/syntax/unexpected-token).
	Code string `json:"code"`
	// Message is a human-readable description. Not asserted in golden files.
	Message string `json:"message"`
	// Severity is the level of the diagnostic.
	Severity Severity `json:"severity"`
	// Range is the source location the diagnostic points to.
	Range Range `json:"range"`
	// SourceURI identifies the file this diagnostic belongs to when a diagnostic
	// is produced by a cross-file analysis pass (e.g. AnalyzeWorkspace). Empty
	// for diagnostics already scoped to a single file.
	SourceURI string `json:"sourceUri,omitempty"`
}

// Severity classifies the urgency of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityHint    Severity = "hint"
)

// Range is a half-open interval [Start, End) in a text document.
// Positions are 0-based (LSP convention).
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position is a cursor location in a text document.
// Line and Character are 0-based (LSP convention).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// ResolveUseCaseTitle finds the use cases a link title names: every use
// case with that exact name; when there is none, every use case whose
// formerly_titled tag lists it (values split on `;`, trimmed). viaFormer is
// true in the second case. One match is a resolved link; two or more (use
// cases of one title in different files) are ambiguous. Use cases are
// matched once per (name, file).
func ResolveUseCaseTitle(doc *CraftDoc, title string) (matches []UseCaseRef, viaFormer bool) {
	if doc == nil || title == "" {
		return nil, false
	}
	seen := map[UseCaseRef]bool{}
	add := func(uc UseCase) {
		r := UseCaseRef{Name: uc.Name, SourceURI: uc.SourceURI}
		if !seen[r] {
			seen[r] = true
			matches = append(matches, r)
		}
	}
	for _, uc := range doc.UseCases {
		if uc.Name == title {
			add(uc)
		}
	}
	if len(matches) > 0 {
		return matches, false
	}
	for _, uc := range doc.UseCases {
		for _, v := range uc.TagValues[TagFormerlyTitled] {
			for _, former := range strings.Split(v, ";") {
				if strings.TrimSpace(former) == title {
					add(uc)
				}
			}
		}
	}
	return matches, len(matches) > 0
}

// EachLink calls fn with every link of the scenario in source order (a
// depth-first walk of Steps).
func (s Scenario) EachLink(fn func(*Link)) {
	eachLink(s.Steps, fn)
}

func eachLink(steps []Step, fn func(*Link)) {
	for _, st := range steps {
		if st.Link != nil {
			fn(st.Link)
		}
		if st.Block != nil {
			for i := range st.Block.Branches {
				eachLink(st.Block.Branches[i].Steps, fn)
			}
		}
	}
}
