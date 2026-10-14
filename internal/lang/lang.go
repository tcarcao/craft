// Package lang holds Craft's fixed vocabulary: the kinds of use-case triggers
// and actions. Every other package compares against these constants rather
// than spelling the strings itself. It imports nothing from this module.
package lang

// ActionKind classifies a use-case action by its verb.
type ActionKind string

const (
	ActionSync     ActionKind = "sync_action"     // X asks Y …
	ActionAsync    ActionKind = "async_action"    // X notifies "E"
	ActionInternal ActionKind = "internal_action" // X <verb> …
	ActionReturn   ActionKind = "return_action"   // X returns … [to Y]
)

// TriggerKind classifies the `when` clause that starts a scenario.
type TriggerKind string

const (
	TriggerExternal     TriggerKind = "external"      // when Actor does …
	TriggerEvent        TriggerKind = "event"         // when "E"
	TriggerDomainListen TriggerKind = "domain_listen" // when Ctx listens "E"
	TriggerReceive      TriggerKind = "receive"       // when <ctx> receives <phrase> [op]
)

// ScheduleKind is the form of a scheduled trigger: `when cron "…"` or
// `when every "…"`.
type ScheduleKind string

const (
	ScheduleCron  ScheduleKind = "cron"  // when cron "0 7 * * 1-5"
	ScheduleEvery ScheduleKind = "every" // when every "5m"
)

// SchedulerActor is the conventional name of the actor a scheduled trigger
// runs as.
const SchedulerActor = "CRON"

// BlockKind is the kind of a branch block inside a scenario.
type BlockKind string

const (
	BlockAlt  BlockKind = "alt"  // exactly one of several branches runs
	BlockOpt  BlockKind = "opt"  // the single branch runs only under its condition
	BlockPar  BlockKind = "par"  // every branch runs, concurrently
	BlockLoop BlockKind = "loop" // the single branch repeats while its condition holds
)

// NoteKind is the kind of a note statement.
type NoteKind string

const (
	NoteText    NoteKind = "note"    // context for the reader
	NoteFinding NoteKind = "finding" // a defect, risk or negative fact the hub lists
)

// SurfaceOpKind is the kind of a surface operation line.
type SurfaceOpKind string

const (
	SurfaceQuery   SurfaceOpKind = "query"   // no side effect (Tier A)
	SurfaceCommand SurfaceOpKind = "command" // changes state (Tier B)
)

// StoreTransport is the ref prefix of a store hand-off: store.<name>.<change>.
const StoreTransport = "store"

// StoreChange is the change a store hand-off reports.
type StoreChange string

const (
	StoreInserted StoreChange = "inserted"
	StoreUpdated  StoreChange = "updated"
	StoreDeleted  StoreChange = "deleted"
)

// UnknownCaller is the surface `callers:` value for "no caller observed".
const UnknownCaller = "unknown"

// ParticipantKind is what a name in a use case resolves to.
type ParticipantKind string

const (
	ParticipantBoundedContext ParticipantKind = "bounded_context"
	ParticipantDomain         ParticipantKind = "domain"
	ParticipantService        ParticipantKind = "service"
	ParticipantActor          ParticipantKind = "actor"
	ParticipantDataStore      ParticipantKind = "data_store"
)
