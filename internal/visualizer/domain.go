package visualizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tcarcao/craft/internal/parser"
)

// DomainMode represents different visualization modes for domain diagrams
type DomainMode string

const (
	DomainModeDetailed     DomainMode = "detailed"
	DomainModeArchitecture DomainMode = "architecture"
)

func (v *Visualizer) GenerateDomainDiagram(model *parser.DSLModel) ([]byte, error) {
	return v.GenerateDomainDiagramWithMode(model, DomainModeDetailed)
}

func (v *Visualizer) GenerateDomainDiagramWithMode(model *parser.DSLModel, mode DomainMode) ([]byte, error) {
	data, _, err := v.GenerateDomainDiagramWithModeAndFormat(model, mode, FormatPNG)
	return data, err
}

func (v *Visualizer) GenerateDomainDiagramWithModeAndFormat(model *parser.DSLModel, mode DomainMode, format SupportedFormat) ([]byte, string, error) {
	var diagramTxt string

	switch mode {
	case DomainModeArchitecture:
		generator := NewPlantUMLArchitectureGenerator()
		diagramTxt = generator.GenerateArchitecturePlantUML(model)
	case DomainModeDetailed:
		generator := NewPlantUMLGenerator()
		diagramTxt = generator.GeneratePlantUML(model)
	default:
		generator := NewPlantUMLGenerator()
		diagramTxt = generator.GeneratePlantUML(model)
	}

	fmt.Println(diagramTxt)
	return generatePlantUMLWithFormat(diagramTxt, format)
}

// PlantUMLGenerator generates PlantUML diagrams from DSL models
type PlantUMLGenerator struct {
	model           *parser.DSLModel // Reference to the model for actor information
	domains         map[string]bool
	actors          map[string]bool
	events          map[string]bool
	eventPublishers map[string]string // event -> domain that publishes it
	domainAliases   map[string]string // domain -> unique alias
	flows           []FlowStep
	stepCounter     int
}

// FlowStep represents a single step in the domain flow
type FlowStep struct {
	StepNumber  int
	From        string
	To          string
	Description string
	Type        string // "sync", "async", "trigger", "event_listen"
	UseCase     string
	ScenarioID  string
}

// PlantUMLArchitectureGenerator generates simplified PlantUML architecture diagrams from DSL models
type PlantUMLArchitectureGenerator struct {
	subDomains      map[string]bool
	connections     map[string]bool // key: "from->to" to avoid duplicates
	events          map[string]bool
	eventPublishers map[string]string // event -> domain that publishes it
	services        map[string]bool   // services that contain subdomains
	domainToService map[string]string // subdomain -> service mapping
	domainAliases   map[string]string
	serviceAliases  map[string]string
}

// ArchitectureConnection represents a connection between subdomains
type ArchitectureConnection struct {
	From string
	To   string
}

// NewPlantUMLGenerator creates a new generator instance
func NewPlantUMLGenerator() *PlantUMLGenerator {
	return &PlantUMLGenerator{
		domains:         make(map[string]bool),
		actors:          make(map[string]bool),
		events:          make(map[string]bool),
		eventPublishers: make(map[string]string),
		domainAliases:   make(map[string]string),
		flows:           make([]FlowStep, 0),
		stepCounter:     0,
	}
}

// NewPlantUMLArchitectureGenerator creates a new architecture generator instance
func NewPlantUMLArchitectureGenerator() *PlantUMLArchitectureGenerator {
	return &PlantUMLArchitectureGenerator{
		subDomains:      make(map[string]bool),
		connections:     make(map[string]bool),
		events:          make(map[string]bool),
		eventPublishers: make(map[string]string),
		services:        make(map[string]bool),
		domainToService: make(map[string]string),
		domainAliases:   make(map[string]string),
		serviceAliases:  make(map[string]string),
	}
}

// GeneratePlantUML converts a DSL model to PlantUML code
func (g *PlantUMLGenerator) GeneratePlantUML(model *parser.DSLModel) string {
	// Reset state
	g.model = model
	g.domains = make(map[string]bool)
	g.actors = make(map[string]bool)
	g.events = make(map[string]bool)
	g.eventPublishers = make(map[string]string)
	g.domainAliases = make(map[string]string)
	g.flows = make([]FlowStep, 0)
	g.stepCounter = 0

	// First pass: collect all event publishers
	for _, useCase := range model.UseCases {
		g.collectEventPublishers(useCase)
	}

	// Second pass: process all use cases to extract domains, actors, and flows
	for _, useCase := range model.UseCases {
		g.processUseCase(useCase)
	}

	// Generate unique aliases for all domains
	g.generateUniqueAliases()

	// Generate PlantUML content
	return g.buildPlantUMLContent()
}

// GenerateArchitecturePlantUML converts a DSL model to simplified architecture PlantUML code
func (g *PlantUMLArchitectureGenerator) GenerateArchitecturePlantUML(model *parser.DSLModel) string {
	// Reset state
	g.subDomains = make(map[string]bool)
	g.connections = make(map[string]bool)
	g.events = make(map[string]bool)
	g.eventPublishers = make(map[string]string)
	g.services = make(map[string]bool)
	g.domainToService = make(map[string]string)
	g.domainAliases = make(map[string]string)
	g.serviceAliases = make(map[string]string)

	// First pass: collect services and their domain mappings
	g.collectServicesForArchitecture(model)

	// Second pass: collect all event publishers
	for _, useCase := range model.UseCases {
		g.collectEventPublishersForArchitecture(useCase)
	}

	// Third pass: process all use cases to extract subdomain connections
	for _, useCase := range model.UseCases {
		g.processUseCaseForArchitecture(useCase)
	}

	// Generate unique aliases for all subdomains and services
	g.generateUniqueAliasesForArchitecture()

	// Generate PlantUML content
	return g.buildArchitecturePlantUMLContent()
}

// collectEventPublishers maps events to their publishing domains
func (g *PlantUMLGenerator) collectEventPublishers(useCase parser.UseCase) {
	for _, scenario := range useCase.Scenarios {
		for _, action := range scenario.Actions {
			if action.Type == parser.ActionTypeAsync && action.Domain != "" && action.Event != "" {
				g.eventPublishers[action.Event] = action.Domain
			}
		}
	}
}

// getDomainQueueName generates a consistent queue name for a domain
func (g *PlantUMLGenerator) getDomainQueueName(domain string) string {
	// Convert domain name to a queue identifier
	queueName := strings.ReplaceAll(domain, " ", "_")
	queueName = strings.ToLower(queueName)
	return queueName + "_queue"
}

// findEventPublisher returns the domain that publishes a given event
func (g *PlantUMLGenerator) findEventPublisher(event string) string {
	if publisher, exists := g.eventPublishers[event]; exists {
		return publisher
	}
	return ""
}

// processUseCase extracts information from a single use case
func (g *PlantUMLGenerator) processUseCase(useCase parser.UseCase) {
	for _, scenario := range useCase.Scenarios {
		g.processScenario(useCase.Name, scenario)
	}
}

// processScenario extracts flows from a scenario
func (g *PlantUMLGenerator) processScenario(useCaseName string, scenario parser.Scenario) {
	// Initialize call stack for this scenario with the external trigger
	callStack := make([]string, 0)
	
	// Add the triggering actor to call stack if it's an external trigger
	if scenario.Trigger.Type == parser.TriggerTypeExternal && scenario.Trigger.Actor != "" {
		callStack = append(callStack, scenario.Trigger.Actor)
	}
	
	// Process trigger
	g.processTrigger(useCaseName, scenario)

	// Process actions with call stack tracking
	for _, action := range scenario.Actions {
		g.processActionWithCallStack(useCaseName, scenario.ID, action, &callStack)
	}
}

// processTrigger handles the scenario trigger
func (g *PlantUMLGenerator) processTrigger(useCaseName string, scenario parser.Scenario) {
	trigger := scenario.Trigger

	switch trigger.Type {
	case parser.TriggerTypeExternal:
		// External actor triggers the flow
		if trigger.Actor != "" {
			g.actors[trigger.Actor] = true
			g.stepCounter++

			// Find the first domain in the actions to connect to
			if len(scenario.Actions) > 0 {
				firstAction := scenario.Actions[0]
				if firstAction.Domain != "" {
					g.domains[firstAction.Domain] = true

					description := fmt.Sprintf("%s %s", trigger.Verb, trigger.Phrase)
					g.flows = append(g.flows, FlowStep{
						StepNumber:  g.stepCounter,
						From:        trigger.Actor,
						To:          firstAction.Domain,
						Description: description,
						Type:        "trigger",
						UseCase:     useCaseName,
						ScenarioID:  scenario.ID,
					})
				}
			}
		}
	case parser.TriggerTypeEvent:
		// Event-based trigger - will be handled via event queues
		if trigger.Event != "" {
			g.events[trigger.Event] = true
		}
	case parser.TriggerTypeDomainListen:
		// Domain listening to event - create flow from publishing domain's queue to listening domain
		if trigger.Domain != "" && trigger.Event != "" {
			g.domains[trigger.Domain] = true
			g.events[trigger.Event] = true
			g.stepCounter++

			// Find which domain published this event
			publishingDomain := g.findEventPublisher(trigger.Event)
			if publishingDomain != "" {
				domainQueue := g.getDomainQueueName(publishingDomain)
				g.flows = append(g.flows, FlowStep{
					StepNumber:  g.stepCounter,
					From:        domainQueue,
					To:          trigger.Domain,
					Description: trigger.Event,
					Type:        "event_listen",
					UseCase:     useCaseName,
					ScenarioID:  scenario.ID,
				})
			}
		}
	}
}

// processActionWithCallStack handles individual actions with call stack tracking
func (g *PlantUMLGenerator) processActionWithCallStack(useCaseName, scenarioID string, action parser.Action, callStack *[]string) {
	switch action.Type {
	case parser.ActionTypeSync:
		// Synchronous call between domains - push caller to stack
		if action.Domain != "" && action.TargetDomain != "" {
			g.domains[action.Domain] = true
			g.domains[action.TargetDomain] = true
			g.stepCounter++

			// Push the calling domain onto the stack
			*callStack = append(*callStack, action.Domain)

			description := g.buildActionDescription(action)
			g.flows = append(g.flows, FlowStep{
				StepNumber:  g.stepCounter,
				From:        action.Domain,
				To:          action.TargetDomain,
				Description: description,
				Type:        "sync",
				UseCase:     useCaseName,
				ScenarioID:  scenarioID,
			})
		}
	case parser.ActionTypeReturn:
		// Return action - flow back to caller
		if action.Domain != "" {
			g.domains[action.Domain] = true
			g.stepCounter++

			description := g.buildActionDescription(action)
			from := action.Domain
			var to string

			// If target domain is specified, use it
			if action.TargetDomain != "" {
				to = action.TargetDomain
				g.domains[to] = true
			} else if len(*callStack) > 0 {
				// Pop from call stack to find the caller
				to = (*callStack)[len(*callStack)-1]
				*callStack = (*callStack)[:len(*callStack)-1]
			} else {
				// No caller in stack, return to external
				to = "External"
			}

			g.flows = append(g.flows, FlowStep{
				StepNumber:  g.stepCounter,
				From:        from,
				To:          to,
				Description: description,
				Type:        "return",
				UseCase:     useCaseName,
				ScenarioID:  scenarioID,
			})
		}
	default:
		// For other action types, use the original logic without call stack
		g.processAction(useCaseName, scenarioID, action)
	}
}

// processAction handles individual actions (legacy method for non-call-stack actions)
func (g *PlantUMLGenerator) processAction(useCaseName, scenarioID string, action parser.Action) {
	switch action.Type {
	case parser.ActionTypeSync:
		// Synchronous call between domains
		if action.Domain != "" && action.TargetDomain != "" {
			g.domains[action.Domain] = true
			g.domains[action.TargetDomain] = true
			g.stepCounter++

			description := g.buildActionDescription(action)
			g.flows = append(g.flows, FlowStep{
				StepNumber:  g.stepCounter,
				From:        action.Domain,
				To:          action.TargetDomain,
				Description: description,
				Type:        "sync",
				UseCase:     useCaseName,
				ScenarioID:  scenarioID,
			})
		}
	case parser.ActionTypeAsync:
		// Asynchronous notification (domain to its own queue)
		if action.Domain != "" && action.Event != "" {
			g.domains[action.Domain] = true
			g.events[action.Event] = true
			g.stepCounter++

			// Domain publishes to its own queue, not an event-specific queue
			domainQueue := g.getDomainQueueName(action.Domain)
			g.flows = append(g.flows, FlowStep{
				StepNumber:  g.stepCounter,
				From:        action.Domain,
				To:          domainQueue,
				Description: action.Event,
				Type:        "async",
				UseCase:     useCaseName,
				ScenarioID:  scenarioID,
			})
		}
	case parser.ActionTypeInternal:
		// Internal domain action - might generate events or trigger other flows
		if action.Domain != "" {
			g.domains[action.Domain] = true
			// Internal actions might be shown as self-loops or annotations
		}
	}
}

// buildActionDescription creates a readable description for actions
func (g *PlantUMLGenerator) buildActionDescription(action parser.Action) string {
	switch action.Type {
	case parser.ActionTypeSync:
		phrase := action.Phrase
		if action.Connector != "" && phrase != "" {
			phrase = action.Connector + " " + phrase
		}
		return phrase
	case parser.ActionTypeAsync:
		return action.Event
	case parser.ActionTypeInternal:
		phrase := action.Phrase
		if action.Connector != "" && phrase != "" {
			phrase = action.Connector + " " + phrase
		}
		if action.Verb != "" {
			return action.Verb + " " + phrase
		}
		return phrase
	case parser.ActionTypeReturn:
		phrase := action.Phrase
		if action.TargetDomain != "" {
			if action.Connector != "" {
				return "returns " + phrase + " " + action.Connector + " " + action.TargetDomain
			}
			return "returns " + phrase + " to " + action.TargetDomain
		}
		return "returns " + phrase
	}
	return ""
}

// generateUniqueAliases creates unique aliases for all domains
func (g *PlantUMLGenerator) generateUniqueAliases() {
	usedAliases := make(map[string]bool)

	for domain := range g.domains {
		baseAlias := g.createBaseAlias(domain)
		finalAlias := baseAlias
		counter := 1

		// If alias already exists, append numbers until we find a unique one
		for usedAliases[finalAlias] {
			finalAlias = fmt.Sprintf("%s%d", baseAlias, counter)
			counter++
		}

		g.domainAliases[domain] = finalAlias
		usedAliases[finalAlias] = true
	}
}

// createBaseAlias creates a base alias from domain name
func (g *PlantUMLGenerator) createBaseAlias(domain string) string {
	// Remove common words and create acronym
	words := strings.Fields(strings.ReplaceAll(domain, "_", " "))

	// Filter out common words
	commonWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"of": true, "to": true, "for": true, "with": true, "in": true,
	}

	var meaningfulWords []string
	for _, word := range words {
		if !commonWords[strings.ToLower(word)] && len(word) > 0 {
			meaningfulWords = append(meaningfulWords, word)
		}
	}

	if len(meaningfulWords) == 0 {
		meaningfulWords = words // fallback to original words
	}

	// Create alias based on number of meaningful words
	if len(meaningfulWords) == 1 {
		word := meaningfulWords[0]
		if len(word) <= 4 {
			return strings.ToLower(word)
		}
		return strings.ToLower(word[:4])
	}

	// Multiple words: take first letter of each word
	alias := ""
	for _, word := range meaningfulWords {
		if len(word) > 0 {
			alias += strings.ToLower(string(word[0]))
		}
		if len(alias) >= 5 { // Limit alias length
			break
		}
	}

	return alias
}

// buildPlantUMLContent generates the final PlantUML content
func (g *PlantUMLGenerator) buildPlantUMLContent() string {
	var sb strings.Builder

	// Header and styling
	sb.WriteString("@startuml\n")
	sb.WriteString("skinparam backgroundColor white\n")
	sb.WriteString("skinparam handwritten false\n\n")

	// Domain styling
	sb.WriteString("' Domain styling with frames\n")
	sb.WriteString("skinparam frame {\n")
	sb.WriteString("  BackgroundColor #E1BEE7\n")
	sb.WriteString("  BorderColor #9370DB\n")
	sb.WriteString("  BorderThickness 2\n")
	sb.WriteString("  FontColor black\n")
	sb.WriteString("  FontSize 11\n")
	sb.WriteString("  FontStyle bold\n")
	sb.WriteString("}\n\n")

	// Queue styling
	sb.WriteString("skinparam queue {\n")
	sb.WriteString("  BackgroundColor #FFE4B5\n")
	sb.WriteString("  BorderColor #666666\n")
	sb.WriteString("  FontSize 10\n")
	sb.WriteString("}\n\n")

	// Actor styling
	sb.WriteString("skinparam actor {\n")
	sb.WriteString("  BackgroundColor white\n")
	sb.WriteString("  BorderColor black\n")
	sb.WriteString("}\n\n")

	// Define domains as frames
	sb.WriteString("' Domains as frames\n")
	domains := g.getSortedDomains()
	for _, domain := range domains {
		alias := g.domainAliases[domain]
		// Format domain name for display (handle long names)
		displayName := g.formatDomainName(domain)
		sb.WriteString(fmt.Sprintf("frame \"%s\" as %s\n", displayName, alias))
	}
	sb.WriteString("\n")

	// Define actors with proper types
	if len(g.actors) > 0 {
		sb.WriteString("' Actors\n")
		actors := g.getSortedActors()
		for _, actorName := range actors {
			actorInfo := g.getActorInfoFromModel(actorName)
			elementType := g.getActorPlantUMLElement(actorInfo)
			sb.WriteString(fmt.Sprintf("%s %s\n", elementType, actorName))
		}
		sb.WriteString("\n")
	}

	// Define event queues (domain-specific queues)
	if len(g.eventPublishers) > 0 {
		sb.WriteString("' Domain queues\n")
		domainQueues := make(map[string]bool)
		for _, domain := range g.eventPublishers {
			if !domainQueues[domain] {
				queueName := g.getDomainQueueName(domain)
				sb.WriteString(fmt.Sprintf("queue \"%s events\" as %s\n", domain, queueName))
				domainQueues[domain] = true
			}
		}
		sb.WriteString("\n")
	}

	// Generate flows
	sb.WriteString("' Workflow flows\n")
	for _, flow := range g.flows {
		fromAlias := g.getElementAlias(flow.From)
		toAlias := g.getElementAlias(flow.To)

		// Use different arrow styles based on flow type
		var arrow string
		switch flow.Type {
		case "return":
			arrow = "-->"  // Return arrow - flows back
		case "sync":
			arrow = "->>"   // Synchronous call - solid arrow
		case "async":
			arrow = "->>"   // Asynchronous - solid arrow
		default:
			arrow = "-->"   // Default arrow
		}

		sb.WriteString(fmt.Sprintf("%s %s %s : %d. %s\n",
			fromAlias, arrow, toAlias, flow.StepNumber, flow.Description))
	}

	sb.WriteString("\n@enduml")
	return sb.String()
}

// Helper methods for sorting and formatting
func (g *PlantUMLGenerator) getSortedDomains() []string {
	domains := make([]string, 0, len(g.domains))
	for domain := range g.domains {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains
}

func (g *PlantUMLGenerator) getSortedActors() []string {
	actors := make([]string, 0, len(g.actors))
	for actor := range g.actors {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	return actors
}

func (g *PlantUMLGenerator) formatDomainName(domain string) string {
	// Break long domain names into multiple lines
	if len(domain) > 15 {
		words := strings.Fields(domain)
		if len(words) > 1 {
			mid := len(words) / 2
			line1 := strings.Join(words[:mid], " ")
			line2 := strings.Join(words[mid:], " ")
			return line1 + "\\n" + line2
		}
	}
	return domain
}

func (g *PlantUMLGenerator) getElementAlias(element string) string {
	// Check if it's a domain
	if alias, exists := g.domainAliases[element]; exists {
		return alias
	}

	// Check if it's a domain queue
	for _, domain := range g.eventPublishers {
		if g.getDomainQueueName(domain) == element {
			return element
		}
	}

	// Must be an actor or other element
	return element
}

// Main function to generate PlantUML from DSL model
func GenerateDomainFlowDiagram(model *parser.DSLModel) string {
	generator := NewPlantUMLGenerator()
	return generator.GeneratePlantUML(model)
}

// Architecture generator methods
// collectServicesForArchitecture collects services and maps domains to services
func (g *PlantUMLArchitectureGenerator) collectServicesForArchitecture(model *parser.DSLModel) {
	for _, service := range model.Services {
		g.services[service.Name] = true

		// Map each domain in this service to the service
		for _, domain := range service.Domains {
			g.domainToService[domain] = service.Name
		}
	}
}

// collectEventPublishersForArchitecture maps events to their publishing domains
func (g *PlantUMLArchitectureGenerator) collectEventPublishersForArchitecture(useCase parser.UseCase) {
	for _, scenario := range useCase.Scenarios {
		for _, action := range scenario.Actions {
			if action.Type == parser.ActionTypeAsync && action.Domain != "" && action.Event != "" {
				g.eventPublishers[action.Event] = action.Domain
				g.events[action.Event] = true
			}
		}
	}
}

// processUseCaseForArchitecture extracts subdomain connections from use cases
func (g *PlantUMLArchitectureGenerator) processUseCaseForArchitecture(useCase parser.UseCase) {
	for _, scenario := range useCase.Scenarios {
		g.processScenarioForArchitecture(scenario)
	}
}

// processScenarioForArchitecture extracts subdomain connections from a scenario
func (g *PlantUMLArchitectureGenerator) processScenarioForArchitecture(scenario parser.Scenario) {
	// Track all subdomains involved in actions
	for _, action := range scenario.Actions {
		switch action.Type {
		case parser.ActionTypeSync:
			// Synchronous call between subdomains
			if action.Domain != "" && action.TargetDomain != "" {
				g.subDomains[action.Domain] = true
				g.subDomains[action.TargetDomain] = true

				// Create connection key to avoid duplicates
				connectionKey := action.Domain + "->" + action.TargetDomain
				g.connections[connectionKey] = true
			}
		case parser.ActionTypeAsync:
			// Async events - domain publishes to its own queue
			if action.Domain != "" && action.Event != "" {
				g.subDomains[action.Domain] = true
				g.events[action.Event] = true

				// Create connection from domain to its queue
				domainQueue := g.getDomainQueueNameForArchitecture(action.Domain)
				connectionKey := action.Domain + "->" + domainQueue
				g.connections[connectionKey] = true
			}
		case parser.ActionTypeInternal:
			// Internal subdomain action
			if action.Domain != "" {
				g.subDomains[action.Domain] = true
			}
		case parser.ActionTypeReturn:
			// Return action - data flowing back
			if action.Domain != "" {
				g.subDomains[action.Domain] = true
				
				if action.TargetDomain != "" {
					g.subDomains[action.TargetDomain] = true
					// Create return connection
					connectionKey := action.Domain + "->" + action.TargetDomain
					g.connections[connectionKey] = true
				}
			}
		}
	}

	// Handle triggers that involve domains
	trigger := scenario.Trigger
	switch trigger.Type {
	case parser.TriggerTypeDomainListen:
		// Domain listening to event - create flow from publishing domain's queue to listening domain
		if trigger.Domain != "" && trigger.Event != "" {
			g.subDomains[trigger.Domain] = true
			g.events[trigger.Event] = true

			// Find which domain published this event
			publishingDomain := g.findEventPublisherForArchitecture(trigger.Event)
			if publishingDomain != "" {
				domainQueue := g.getDomainQueueNameForArchitecture(publishingDomain)
				connectionKey := domainQueue + "->" + trigger.Domain
				g.connections[connectionKey] = true
			}
		}
	case parser.TriggerTypeEvent:
		// Event-based trigger
		if trigger.Event != "" {
			g.events[trigger.Event] = true
		}
	}
}

// getDomainQueueNameForArchitecture generates a consistent queue name for a domain
func (g *PlantUMLArchitectureGenerator) getDomainQueueNameForArchitecture(domain string) string {
	// Convert domain name to a queue identifier
	queueName := strings.ReplaceAll(domain, " ", "_")
	queueName = strings.ToLower(queueName)
	return queueName + "_queue"
}

// findEventPublisherForArchitecture returns the domain that publishes a given event
func (g *PlantUMLArchitectureGenerator) findEventPublisherForArchitecture(event string) string {
	if publisher, exists := g.eventPublishers[event]; exists {
		return publisher
	}
	return ""
}

// generateUniqueAliasesForArchitecture creates unique aliases for all subdomains and services
func (g *PlantUMLArchitectureGenerator) generateUniqueAliasesForArchitecture() {
	usedAliases := make(map[string]bool)

	// Generate service aliases first
	for service := range g.services {
		baseAlias := g.createBaseAliasForArchitecture(service)
		finalAlias := baseAlias + "_svc"
		counter := 1

		// If alias already exists, append numbers until we find a unique one
		for usedAliases[finalAlias] {
			finalAlias = fmt.Sprintf("%s_svc%d", baseAlias, counter)
			counter++
		}

		g.serviceAliases[service] = finalAlias
		usedAliases[finalAlias] = true
	}

	// Generate subdomain aliases
	for subDomain := range g.subDomains {
		baseAlias := g.createBaseAliasForArchitecture(subDomain)
		finalAlias := baseAlias
		counter := 1

		// If alias already exists, append numbers until we find a unique one
		for usedAliases[finalAlias] {
			finalAlias = fmt.Sprintf("%s%d", baseAlias, counter)
			counter++
		}

		g.domainAliases[subDomain] = finalAlias
		usedAliases[finalAlias] = true
	}
}

// createBaseAliasForArchitecture creates a base alias from subdomain name
func (g *PlantUMLArchitectureGenerator) createBaseAliasForArchitecture(subDomain string) string {
	// Similar to the original createBaseAlias but simpler for architecture view
	words := strings.Fields(strings.ReplaceAll(subDomain, "_", " "))

	// Filter out common words
	commonWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"of": true, "to": true, "for": true, "with": true, "in": true,
	}

	var meaningfulWords []string
	for _, word := range words {
		if !commonWords[strings.ToLower(word)] && len(word) > 0 {
			meaningfulWords = append(meaningfulWords, word)
		}
	}

	if len(meaningfulWords) == 0 {
		meaningfulWords = words // fallback to original words
	}

	// Create alias based on number of meaningful words
	if len(meaningfulWords) == 1 {
		word := meaningfulWords[0]
		if len(word) <= 4 {
			return strings.ToLower(word)
		}
		return strings.ToLower(word[:4])
	}

	// Multiple words: take first letter of each word
	alias := ""
	for _, word := range meaningfulWords {
		if len(word) > 0 {
			alias += strings.ToLower(string(word[0]))
		}
		if len(alias) >= 5 { // Limit alias length
			break
		}
	}

	return alias
}

// buildArchitecturePlantUMLContent generates the final architecture PlantUML content
func (g *PlantUMLArchitectureGenerator) buildArchitecturePlantUMLContent() string {
	var sb strings.Builder

	// Header and styling
	sb.WriteString("@startuml\n")
	sb.WriteString("skinparam backgroundColor white\n")
	sb.WriteString("skinparam handwritten false\n\n")

	// Subdomain styling - use different styling for architecture view
	sb.WriteString("' Subdomain styling with frames\n")
	sb.WriteString("skinparam frame {\n")
	sb.WriteString("  BackgroundColor #E6F3FF\n")
	sb.WriteString("  BorderColor #4A90E2\n")
	sb.WriteString("  BorderThickness 2\n")
	sb.WriteString("  FontColor black\n")
	sb.WriteString("  FontSize 12\n")
	sb.WriteString("  FontStyle bold\n")
	sb.WriteString("}\n\n")

	// Queue styling
	sb.WriteString("skinparam queue {\n")
	sb.WriteString("  BackgroundColor #FFE4B5\n")
	sb.WriteString("  BorderColor #666666\n")
	sb.WriteString("  FontSize 10\n")
	sb.WriteString("}\n\n")

	// Service boundary styling
	sb.WriteString("skinparam rectangle {\n")
	sb.WriteString("  BackgroundColor #F0F8FF\n")
	sb.WriteString("  BorderColor #4169E1\n")
	sb.WriteString("  BorderThickness 3\n")
	sb.WriteString("  FontColor #000080\n")
	sb.WriteString("  FontSize 14\n")
	sb.WriteString("  FontStyle bold\n")
	sb.WriteString("}\n\n")

	// Define service boundaries and subdomains
	g.defineServiceBoundaries(&sb)

	// Define event queues (domain-specific queues)
	if len(g.eventPublishers) > 0 {
		sb.WriteString("' Domain queues\n")
		domainQueues := make(map[string]bool)
		for _, domain := range g.eventPublishers {
			if !domainQueues[domain] {
				queueName := g.getDomainQueueNameForArchitecture(domain)
				sb.WriteString(fmt.Sprintf("queue \"%s events\" as %s\n", domain, queueName))
				domainQueues[domain] = true
			}
		}
		sb.WriteString("\n")
	}

	// Generate connections (unlabeled and unduplicated)
	sb.WriteString("' Subdomain connections\n")
	for connectionKey := range g.connections {
		parts := strings.Split(connectionKey, "->")
		if len(parts) == 2 {
			fromAlias := g.getElementAliasForArchitecture(parts[0])
			toAlias := g.getElementAliasForArchitecture(parts[1])
			if fromAlias != "" && toAlias != "" {
				sb.WriteString(fmt.Sprintf("%s --> %s\n", fromAlias, toAlias))
			}
		}
	}

	sb.WriteString("\n@enduml")
	return sb.String()
}

// defineServiceBoundaries creates service boundary rectangles containing subdomains
func (g *PlantUMLArchitectureGenerator) defineServiceBoundaries(sb *strings.Builder) {
	if len(g.services) == 0 {
		return
	}

	sb.WriteString("' Service boundaries\n")

	// Group subdomains by service
	serviceToSubDomains := make(map[string][]string)
	ungroupedSubDomains := make([]string, 0)

	for subDomain := range g.subDomains {
		if service, exists := g.domainToService[subDomain]; exists {
			serviceToSubDomains[service] = append(serviceToSubDomains[service], subDomain)
		} else {
			ungroupedSubDomains = append(ungroupedSubDomains, subDomain)
		}
	}

	// Create service boundary rectangles
	for service, subDomains := range serviceToSubDomains {
		if len(subDomains) > 0 {
			serviceAlias := g.serviceAliases[service]
			displayName := g.formatSubDomainName(service)

			sb.WriteString(fmt.Sprintf("rectangle \"%s\" as %s {\n", displayName, serviceAlias))

			// Add subdomains inside the service boundary
			for _, subDomain := range subDomains {
				alias := g.domainAliases[subDomain]
				subDomainDisplayName := g.formatSubDomainName(subDomain)
				sb.WriteString(fmt.Sprintf("  frame \"%s\" as %s\n", subDomainDisplayName, alias))
			}

			sb.WriteString("}\n")
		}
	}

	// Add ungrouped subdomains (not part of any service) outside service boundaries
	if len(ungroupedSubDomains) > 0 {
		sb.WriteString("\n' Ungrouped subdomains\n")
		for _, subDomain := range ungroupedSubDomains {
			alias := g.domainAliases[subDomain]
			displayName := g.formatSubDomainName(subDomain)
			sb.WriteString(fmt.Sprintf("frame \"%s\" as %s\n", displayName, alias))
		}
	}

	sb.WriteString("\n")
}

// Helper methods for architecture generator
func (g *PlantUMLArchitectureGenerator) getSortedSubDomains() []string {
	subDomains := make([]string, 0, len(g.subDomains))
	for subDomain := range g.subDomains {
		subDomains = append(subDomains, subDomain)
	}
	sort.Strings(subDomains)
	return subDomains
}

func (g *PlantUMLArchitectureGenerator) formatSubDomainName(subDomain string) string {
	// Simple formatting for architecture view
	if len(subDomain) > 20 {
		words := strings.Fields(subDomain)
		if len(words) > 1 {
			mid := len(words) / 2
			line1 := strings.Join(words[:mid], " ")
			line2 := strings.Join(words[mid:], " ")
			return line1 + "\\n" + line2
		}
	}
	return subDomain
}

func (g *PlantUMLArchitectureGenerator) getElementAliasForArchitecture(element string) string {
	// Check if it's a domain
	if alias, exists := g.domainAliases[element]; exists {
		return alias
	}

	// Check if it's a domain queue
	for _, domain := range g.eventPublishers {
		if g.getDomainQueueNameForArchitecture(domain) == element {
			return element
		}
	}

	// Must be an actor or other element
	return element
}

// getActorInfoFromModel finds actor information from the DSL model
func (g *PlantUMLGenerator) getActorInfoFromModel(actorName string) *parser.Actor {
	if g.model == nil {
		return nil
	}

	for _, actor := range g.model.Actors {
		if actor.Name == actorName {
			return &actor
		}
	}
	return nil
}

// getActorPlantUMLElement returns the appropriate PlantUML element type for an actor
func (g *PlantUMLGenerator) getActorPlantUMLElement(actor *parser.Actor) string {
	if actor == nil {
		// Default fallback for actors not found in the model (legacy behavior)
		return "actor"
	}

	switch actor.Type {
	case parser.ActorTypeUser:
		return "actor"
	case parser.ActorTypeSystem:
		return "boundary" // Use boundary for external systems in domain diagrams
	case parser.ActorTypeService:
		return "control" // Use control for external services in domain diagrams
	default:
		// Default fallback
		return "actor"
	}
}
