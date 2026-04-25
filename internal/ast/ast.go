// Package ast defines the v2 hand-written parser's abstract syntax tree nodes.
// All types here are private to the parser implementation; the public contract
// is pkg/craft.CraftDoc. AST shapes change freely; CraftDoc is frozen at v0.1.
package ast

// LineToLSP converts a 1-based source line (as recorded in AST nodes) to a
// 0-based LSP line number (as required by the LSP protocol).
// Returns 0 for any non-positive input.
func LineToLSP(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
}

// File is the root AST node for a parsed .craft file.
type File struct {
	Actors      []*ActorDecl       `json:"actors,omitempty"`
	ActorBlocks []*ActorBlockRange `json:"actorBlocks,omitempty"` // ranges of actors { } blocks
	Domains     []*DomainDecl      `json:"domains,omitempty"`
	Services    []*ServiceDecl     `json:"services,omitempty"`
	UseCases    []*UseCaseDecl     `json:"useCases,omitempty"`
	Archs       []*ArchDecl        `json:"archs,omitempty"`
	Exposures   []*ExposureDecl    `json:"exposures,omitempty"`
}

// ActorBlockRange tracks the source range of an actors { } block.
type ActorBlockRange struct {
	// Line is the 1-based source line of the `actors` keyword.
	Line int
	// EndLine is the 1-based source line of the closing `}`.
	EndLine int
}

// ExposureDecl represents an exposure block: exposure <name> { to: ... through: ... }.
type ExposureDecl struct {
	Name     string
	To       []string
	Contexts []string
	Through  []string
	// Line is the 1-based source line of the `exposure` keyword.
	Line int
}

// ArchDecl represents an arch { ... } block.
type ArchDecl struct {
	Name         string
	Presentation []*ArchComponent
	Gateway      []*ArchComponent
	// Line is the 1-based source line of the `arch` keyword.
	Line int
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int
	// PresentationLine is the 1-based source line of the `presentation:` label (0 if absent).
	PresentationLine int
	// GatewayLine is the 1-based source line of the `gateway:` label (0 if absent).
	GatewayLine int
}

// ArchComponent represents a single component (simple or flow chain) within an arch section.
type ArchComponent struct {
	Name      string
	Type      string           // "simple" or "flow"
	Modifiers []ArchModifier
	Chain     []*ArchComponent // non-nil when Type == "flow"
}

// ArchModifier is a key-value pair attached to an arch component.
type ArchModifier struct {
	Key   string
	Value string
}

// DeploymentRule is a single percentage-to-target mapping within a deployment spec.
type DeploymentRule struct {
	Percentage string // e.g. "90%"
	Target     string // e.g. "stable"
}

// ServiceDecl represents a service declaration inside a services { ... } block.
type ServiceDecl struct {
	Name       string   `json:"name"`
	Contexts   []string `json:"contexts,omitempty"`
	DataStores []string `json:"dataStores,omitempty"`
	Language   string   `json:"language,omitempty"`
	// DeploymentType is the deployment strategy type (e.g. "canary", "blue_green", "rolling").
	DeploymentType string `json:"deploymentType,omitempty"`
	// DeploymentRules holds the percentage-to-target rules for parameterised deployment types.
	DeploymentRules []DeploymentRule `json:"deploymentRules,omitempty"`
	// Line is the 1-based source line where the service name appears.
	Line int `json:"line,omitempty"`
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int `json:"endLine,omitempty"`
	// ContextLines holds the 1-based source line for each entry in Contexts
	// (parallel slice). Used for go-to-definition cursor matching.
	ContextLines []int `json:"contextLines,omitempty"`
}

// DomainDecl represents either an individual domain declaration
// (domain Name { BoundedContext... }) or an entry inside a domains block.
type DomainDecl struct {
	Name            string `json:"name"`
	BoundedContexts []string `json:"boundedContexts,omitempty"`
	// Line is the 1-based source line where the domain name appears.
	Line int `json:"line,omitempty"`
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int `json:"endLine,omitempty"`
}

// ActorDecl represents either a block entry (actors { user Foo }) or an
// individual declaration (actor user Foo).
type ActorDecl struct {
	Name string    `json:"name"`
	Type ActorType `json:"type"`
	// Line is the 1-based source line where the actor name appears.
	Line int `json:"line,omitempty"`
}

// ActorType classifies an actor.
type ActorType string

const (
	ActorTypeUser    ActorType = "user"
	ActorTypeSystem  ActorType = "system"
	ActorTypeService ActorType = "service"
)

// UseCaseDecl represents a use_case "..." { ... } declaration.
type UseCaseDecl struct {
	// Name is the quoted string title of the use case.
	Name string `json:"name"`
	// Scenarios are the `when ...` clauses inside the use case body.
	Scenarios []*ScenarioDecl `json:"scenarios,omitempty"`
	// Line is the 1-based source line of the use_case keyword.
	Line int `json:"line,omitempty"`
	// EndLine is the 1-based source line of the closing `}`, for folding.
	EndLine int `json:"endLine,omitempty"`
}

// ScenarioDecl represents a single `when ...` clause within a use case.
type ScenarioDecl struct {
	// ID is the ANTLR-compatible scenario identifier (e.g. "scenario_1").
	// Assigned during CraftDoc projection, not parsed directly.
	ID string `json:"id,omitempty"`
	// Trigger describes the initiating condition.
	Trigger TriggerDecl `json:"trigger"`
	// Actions are the ordered steps within the scenario body.
	Actions []*ActionDecl `json:"actions,omitempty"`
}

// TriggerDecl represents the `when ...` line that opens a scenario.
type TriggerDecl struct {
	// TriggerType is one of "external", "event", or "domain_listen".
	TriggerType string `json:"type"`
	// Actor is the initiating actor name for external triggers.
	Actor string `json:"actor,omitempty"`
	// Verb is the action verb for external triggers (e.g. "initiates").
	Verb string `json:"verb,omitempty"`
	// Phrase is the rest of the trigger phrase after verb for external triggers.
	Phrase string `json:"phrase,omitempty"`
	// Domain is the listening domain/service for domain_listen triggers.
	Domain string `json:"domain,omitempty"`
	// Event is the event string for domain_listen and event triggers.
	Event string `json:"event,omitempty"`
	// Description is the human-readable description of the full trigger line.
	Description string `json:"description"`
	// Line is the 1-based source line of the `when` keyword.
	Line int `json:"line,omitempty"`
}

// ActionDecl represents a single action statement within a scenario.
type ActionDecl struct {
	// ActionType is one of "sync_action", "async_action", "internal_action", or "return_action".
	ActionType string `json:"type"`
	// ActionID is the global numeric ID assigned during parsing (for "action_N" CraftDoc output).
	ActionID int `json:"actionId,omitempty"`
	// Domain is the actor/domain/service that performs the action (the "from" party).
	Domain string `json:"domain"`
	// TargetDomain is the recipient for sync/return actions.
	TargetDomain string `json:"targetDomain,omitempty"`
	// Verb is the action verb for internal actions.
	Verb string `json:"verb,omitempty"`
	// Connector is "to" or "for" for sync actions.
	Connector string `json:"connector,omitempty"`
	// Phrase is the descriptive phrase of the action.
	Phrase string `json:"phrase,omitempty"`
	// Event is the event name for async actions (notifies).
	Event string `json:"event,omitempty"`
	// Description is the human-readable full action line.
	Description string `json:"description"`
	// Line is the 1-based source line of this action.
	Line int `json:"line,omitempty"`
}
