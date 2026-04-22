// Package craft provides the public API for the Craft DSL toolchain.
package craft

// CraftDoc is the parser-agnostic canonical representation of a parsed .craft file.
// Experimental: stabilizes at v0.1.
type CraftDoc struct {
	Architectures []ArchBlock `json:"architectures,omitempty"`
	Exposures     []Exposure  `json:"exposures,omitempty"`
	Services      []Service   `json:"services,omitempty"`
	UseCases      []UseCase   `json:"useCases"`
	Domains       []Domain    `json:"domains,omitempty"`
	Actors        []Actor     `json:"actors,omitempty"`
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
	Type        TriggerType `json:"type"`
	Actor       string      `json:"actor,omitempty"`
	Verb        string      `json:"verb,omitempty"`
	Phrase      string      `json:"phrase,omitempty"`
	Domain      string      `json:"domain,omitempty"`
	Event       string      `json:"event,omitempty"`
	Description string      `json:"description"`
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
	ID           string     `json:"id"`
	Type         ActionType `json:"type"`
	Domain       string     `json:"domain"`
	Verb         string     `json:"verb,omitempty"`
	TargetDomain string     `json:"targetDomain,omitempty"`
	Event        string     `json:"event,omitempty"`
	Connector    string     `json:"connector,omitempty"`
	Phrase       string     `json:"phrase,omitempty"`
	Description  string     `json:"description"`
	Line         int        `json:"line,omitempty"`
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
