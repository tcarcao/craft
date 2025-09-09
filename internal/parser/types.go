package parser

// DSLModel represents the entire parsed DSL document
type DSLModel struct {
	Architectures []Architecture `json:"architectures,omitempty"`
	Exposures     []Exposure     `json:"exposures,omitempty"`
	Services      []Service      `json:"services,omitempty"`
	UseCases      []UseCase      `json:"useCases"`
	Domains       []Domain       `json:"domains,omitempty"`
}

// Architecture represents an architecture definition
type Architecture struct {
	Name         string      `json:"name,omitempty"` // Optional name
	Presentation []Component `json:"presentation"`
	Gateway      []Component `json:"gateway"`
}

// Component represents a component in an architecture
type Component struct {
	Name      string              `json:"name"`
	Type      ComponentType       `json:"type"`
	Modifiers []ComponentModifier `json:"modifiers,omitempty"`
	Chain     []Component         `json:"chain,omitempty"` // For component flows
}

// ComponentType defines the type of component
type ComponentType string

const (
	ComponentTypeSimple ComponentType = "simple" // Single component
	ComponentTypeFlow   ComponentType = "flow"   // Component chain (A > B > C)
)

// ComponentModifier represents a component modifier like [ssl, cache:aggressive]
type ComponentModifier struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // Empty for flags like [ssl], populated for key:value like [cache:aggressive]
}

// Exposure represents an exposure definition
type Exposure struct {
	Name    string   `json:"name"`
	To      []string `json:"to,omitempty"`      // Targets
	Of      []string `json:"of,omitempty"`      // Domains
	Through []string `json:"through,omitempty"` // Gateways
}

// Service represents a service definition with enhanced deployment support
type Service struct {
	Name       string             `json:"name"`
	Domains    []string           `json:"domains,omitempty"`
	DataStores []string           `json:"dataStores,omitempty"`
	Language   string             `json:"language,omitempty"`
	Deployment DeploymentStrategy `json:"deployment,omitempty"`
}

// DeploymentStrategy represents deployment configuration
type DeploymentStrategy struct {
	Type  string           `json:"type,omitempty"`  // canary, blue_green, rolling
	Rules []DeploymentRule `json:"rules,omitempty"` // Deployment rules with percentages
}

// DeploymentRule represents a single deployment rule
type DeploymentRule struct {
	Percentage string `json:"percentage"` // e.g., "10%"
	Target     string `json:"target"`     // e.g., "staging"
}

// UseCase represents a single use case with its scenarios
type UseCase struct {
	Name      string     `json:"name"`
	Scenarios []Scenario `json:"scenarios"`
}

// Scenario represents a complete scenario with trigger and actions
type Scenario struct {
	ID      string   `json:"id"`
	Trigger Trigger  `json:"trigger"`
	Actions []Action `json:"actions"`
}

// Trigger represents what initiates a scenario
type Trigger struct {
	Type        TriggerType `json:"type"`
	Actor       string      `json:"actor,omitempty"`  // For external triggers
	Verb        string      `json:"verb,omitempty"`   // For external triggers
	Phrase      string      `json:"phrase,omitempty"` // For external triggers (rest of phrase)
	Domain      string      `json:"domain,omitempty"` // For domain listeners
	Event       string      `json:"event,omitempty"`  // For events
	Description string      `json:"description"`      // Human readable
}

// TriggerType defines the different types of triggers
type TriggerType string

const (
	TriggerTypeExternal     TriggerType = "external"      // "when actor verb remainder"
	TriggerTypeEvent        TriggerType = "event"         // "when 'event_occurred'"
	TriggerTypeDomainListen TriggerType = "domain_listen" // "when domain listens 'event'"
)

// Action represents an action taken in response to a trigger
type Action struct {
	ID           string     `json:"id"`
	Type         ActionType `json:"type"`
	Domain       string     `json:"domain"`
	Verb         string     `json:"verb,omitempty"`         // For internal actions
	TargetDomain string     `json:"targetDomain,omitempty"` // For sync actions
	Event        string     `json:"event,omitempty"`        // For async actions
	Connector    string     `json:"connector,omitempty"`    // "to", "as", "the", etc.
	Phrase       string     `json:"phrase,omitempty"`       // The action phrase
	Description  string     `json:"description"`            // Full human readable action
}

// ActionType defines the different types of actions
type ActionType string

const (
	ActionTypeSync     ActionType = "sync_action"     // "domain asks domain [connector] phrase"
	ActionTypeAsync    ActionType = "async_action"    // "domain notifies 'event'"
	ActionTypeInternal ActionType = "internal_action" // "domain verb [connector] phrase"
)

// Interaction represents domain-to-domain interactions for sequence diagrams
type Interaction struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Type        string `json:"type"` // "sync", "async"
	Description string `json:"description"`
	UseCase     string `json:"useCase"`
	ScenarioID  string `json:"scenarioId"`
}

// Domain represents a domain definition with its subdomains
type Domain struct {
	Name       string   `json:"name"`
	SubDomains []string `json:"subDomains"`
}
