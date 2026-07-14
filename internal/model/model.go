// Package model holds the canonical Craft document data types. It is a leaf
// package (imports nothing from internal/ or pkg/) so that internal/syntax and
// internal/sema can produce these types without importing pkg/craft, which
// re-exports them as the stable public API.
package model

// CraftDoc is the parser-agnostic canonical representation of a parsed .craft file.
// It is re-exported from pkg/craft as the stable public API type; see that
// package's stability contract.
type CraftDoc struct {
	Architectures []ArchBlock `json:"architectures,omitempty"`
	Exposures     []Exposure  `json:"exposures,omitempty"`
	Services      []Service   `json:"services,omitempty"`
	UseCases      []UseCase   `json:"useCases"`
	Domains       []Domain    `json:"domains,omitempty"`
	Actors        []Actor     `json:"actors,omitempty"`
	ContextMap    []Edge      `json:"contextMap,omitempty"`
}

// Edge is one authored typed edge from a context_map block, connecting two
// node slugs (e.g. "bc:re/subscriptions") with a verb (e.g. "realized_by").
// Endpoint-kind validation (bc -> service, etc.) is a sema concern, not
// captured here — this is a shape-only projection of the parsed edges.
type Edge struct {
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
	OpsLevel   string             `json:"opsLevel,omitempty"`
	Repo       string             `json:"repo,omitempty"`
	Line       int                `json:"line,omitempty"`
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
}

// Scenario is a specific path through a use case.
type Scenario struct {
	ID      string   `json:"id"`
	Trigger Trigger  `json:"trigger"`
	Actions []Action `json:"actions"`
}

// Trigger describes what initiates a scenario.
type Trigger struct {
	Type    TriggerType `json:"type"`
	Actor   string      `json:"actor,omitempty"`
	Verb    string      `json:"verb,omitempty"`
	Phrase  string      `json:"phrase,omitempty"`
	Context string      `json:"context,omitempty"`
	Event   string      `json:"event,omitempty"`
	// Ref holds the full typed-ref text (e.g. "vas.VasApplied" or
	// "bc:re/billing") for a domain_listen trigger whose event was written
	// as a ref rather than a quoted string. Empty for the legacy
	// `listens "X"` form and for non-domain_listen triggers — Event still
	// carries the value either way, for existing consumers.
	Ref         string `json:"ref,omitempty"`
	Description string `json:"description"`
}

// TriggerType classifies the origin of a scenario trigger.
type TriggerType string

const (
	TriggerTypeExternal     TriggerType = "external"
	TriggerTypeEvent        TriggerType = "event"
	TriggerTypeDomainListen TriggerType = "domain_listen"
)

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
	Ref         string `json:"ref,omitempty"`
	Connector   string `json:"connector,omitempty"`
	Phrase      string `json:"phrase,omitempty"`
	Description string `json:"description"`
	Line        int    `json:"line,omitempty"`
}

// ActionType classifies the interaction pattern of an action.
type ActionType string

const (
	ActionTypeSync     ActionType = "sync_action"
	ActionTypeAsync    ActionType = "async_action"
	ActionTypeInternal ActionType = "internal_action"
	ActionTypeReturn   ActionType = "return_action"
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

// Actor is an entity (user, system, or service) that interacts with the system.
type Actor struct {
	Name string    `json:"name"`
	Type ActorType `json:"type"`
	Line int       `json:"line,omitempty"`
}

// ActorType classifies an actor.
type ActorType string

const (
	ActorTypeUser    ActorType = "user"
	ActorTypeSystem  ActorType = "system"
	ActorTypeService ActorType = "service"
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
