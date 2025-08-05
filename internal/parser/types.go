package parser

// DSL Model Types for Diagram Generation (Updated for Simplified Grammar)

// DSLModel represents the entire parsed DSL document
type DSLModel struct {
	Services []Service `json:"services,omitempty"`
	UseCases []UseCase `json:"useCases"`
}

// Service represents a service definition with its domains, and other properties
type Service struct {
	Name       string   `json:"name"`
	Domains    []string `json:"domains,omitempty"`
	DataStores []string `json:"dataStores,omitempty"`
	Language   string   `json:"language"`
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

// ActionType defines the different types of actions (simplified)
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
