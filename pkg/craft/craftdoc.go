// Package craft is the stable public Go API for the Craft DSL toolchain.
//
// It provides the canonical document model (CraftDoc and its component types),
// the Diagnostic type, and the Parse / ParseFiles entry points for parsing
// Craft source in-process. As of v2.10.0 this package is stable and follows
// semantic versioning: existing exported names, struct fields, and JSON tags
// will not change or be removed within a major version; additions are allowed
// in minor releases. Requires Go 1.25 or newer to import.
package craft

import "github.com/tcarcao/craft/v2/internal/model"

type (
	CraftDoc           = model.CraftDoc
	Edge               = model.Edge
	ArchBlock          = model.ArchBlock
	Component          = model.Component
	ComponentType      = model.ComponentType
	ComponentModifier  = model.ComponentModifier
	Exposure           = model.Exposure
	Service            = model.Service
	DeploymentStrategy = model.DeploymentStrategy
	DeploymentRule     = model.DeploymentRule
	UseCase            = model.UseCase
	Scenario           = model.Scenario
	Trigger            = model.Trigger
	TriggerType        = model.TriggerType
	Action             = model.Action
	ActionType         = model.ActionType
	Operation          = model.Operation
	Interaction        = model.Interaction
	Domain             = model.Domain
	Actor              = model.Actor
	ActorType          = model.ActorType
	TermRelation       = model.TermRelation
)

const (
	ComponentTypeSimple = model.ComponentTypeSimple
	ComponentTypeFlow   = model.ComponentTypeFlow

	TriggerTypeExternal     = model.TriggerTypeExternal
	TriggerTypeEvent        = model.TriggerTypeEvent
	TriggerTypeDomainListen = model.TriggerTypeDomainListen

	ActionTypeSync     = model.ActionTypeSync
	ActionTypeAsync    = model.ActionTypeAsync
	ActionTypeInternal = model.ActionTypeInternal
	ActionTypeReturn   = model.ActionTypeReturn

	// OpVerb* are the recognised protocol verbs for an Operation annotation's
	// Verb field. Untyped string constants, not a named type: the verb set is
	// open by design (an unrecognised leading word is payload, not an error),
	// so Operation.Verb stays a plain string rather than a closed enum.
	OpVerbGET     = "GET"
	OpVerbPOST    = "POST"
	OpVerbPUT     = "PUT"
	OpVerbPATCH   = "PATCH"
	OpVerbDELETE  = "DELETE"
	OpVerbHEAD    = "HEAD"
	OpVerbOPTIONS = "OPTIONS"
	OpVerbGRPC    = "GRPC"
	OpVerbTOPIC   = "TOPIC"
	OpVerbQUERY   = "QUERY"

	ActorTypeUser    = model.ActorTypeUser
	ActorTypeSystem  = model.ActorTypeSystem
	ActorTypeService = model.ActorTypeService
)
