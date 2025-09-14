package visualizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

func (v *Visualizer) GenerateDomainDiagram(model *parser.DSLModel) ([]byte, error) {
	generator := NewPlantUMLGenerator()
	diagramTxt := generator.GeneratePlantUML(model)
	fmt.Println(diagramTxt)
	return generatePlantUML(diagramTxt)
}

func (v *Visualizer) GenerateDomainDiagramWithFormat(model *parser.DSLModel, format SupportedFormat) ([]byte, string, error) {
	generator := NewPlantUMLGenerator()
	diagramTxt := generator.GeneratePlantUML(model)
	fmt.Println(diagramTxt)
	return generatePlantUMLWithFormat(diagramTxt, format)
}

// PlantUMLGenerator generates PlantUML diagrams from DSL models
type PlantUMLGenerator struct {
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

// GeneratePlantUML converts a DSL model to PlantUML code
func (g *PlantUMLGenerator) GeneratePlantUML(model *parser.DSLModel) string {
	// Reset state
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
	// Process trigger
	g.processTrigger(useCaseName, scenario)

	// Process actions
	for _, action := range scenario.Actions {
		g.processAction(useCaseName, scenario.ID, action)
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

// processAction handles individual actions
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

	// Define actors
	if len(g.actors) > 0 {
		sb.WriteString("' Actors\n")
		actors := g.getSortedActors()
		for _, actor := range actors {
			sb.WriteString(fmt.Sprintf("actor %s\n", actor))
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

		sb.WriteString(fmt.Sprintf("%s --> %s : %d. %s\n",
			fromAlias, toAlias, flow.StepNumber, flow.Description))
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
