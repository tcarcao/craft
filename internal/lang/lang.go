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
)

// ParticipantKind is what a name in a use case resolves to.
type ParticipantKind string

const (
	ParticipantBoundedContext ParticipantKind = "bounded_context"
	ParticipantDomain         ParticipantKind = "domain"
	ParticipantService        ParticipantKind = "service"
	ParticipantActor          ParticipantKind = "actor"
	ParticipantDataStore      ParticipantKind = "data_store"
)
