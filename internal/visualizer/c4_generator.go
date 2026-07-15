package visualizer

import (
	"fmt"
	"slices"
	"strings"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// C4GenerationMode determines how domains are represented
type C4GenerationMode string

const (
	C4ModeTransparent C4GenerationMode = "transparent" // Current: domains grouped transparently in services
	C4ModeBoundaries  C4GenerationMode = "boundaries"  // New: domains as Container_Boundary within services
)

// C4DiagramGenerator generates C4 diagrams with proper system separation
type C4DiagramGenerator struct {
	model              *craft.CraftDoc
	mode               C4GenerationMode
	systems            map[string]*C4System
	containers         map[string]*C4Container
	relations          []C4Relation
	actors             map[string]bool
	systemRelations    []C4Relation
	userInteractionMap map[string][]string
	presentationSystem *C4System
	gatewaySystem      *C4System
	focusedServices    map[string]bool // Services to show as internal
	focusedContexts    map[string]bool // Contexts to show as internal
	hasFocus           bool            // Whether focus mode is enabled
	showDatabases      bool            // Whether to show database containers
}

// NewC4DiagramGenerator creates a new redesigned generator
func NewC4DiagramGenerator(mode C4GenerationMode, showDatabases bool) *C4DiagramGenerator {
	return &C4DiagramGenerator{
		mode:               mode,
		systems:            make(map[string]*C4System),
		containers:         make(map[string]*C4Container),
		relations:          make([]C4Relation, 0),
		actors:             make(map[string]bool),
		systemRelations:    make([]C4Relation, 0),
		userInteractionMap: make(map[string][]string),
		focusedServices:    make(map[string]bool),
		focusedContexts:    make(map[string]bool),
		hasFocus:           false,
		showDatabases:      showDatabases,
	}
}

// NewC4DiagramGeneratorWithFocus creates a generator with service focus
func NewC4DiagramGeneratorWithFocus(mode C4GenerationMode, focusedServiceNames []string, showDatabases bool) *C4DiagramGenerator {
	focusedServices := make(map[string]bool)
	for _, serviceName := range focusedServiceNames {
		focusedServices[serviceName] = true
	}

	return &C4DiagramGenerator{
		mode:               mode,
		systems:            make(map[string]*C4System),
		containers:         make(map[string]*C4Container),
		relations:          make([]C4Relation, 0),
		actors:             make(map[string]bool),
		systemRelations:    make([]C4Relation, 0),
		userInteractionMap: make(map[string][]string),
		focusedServices:    focusedServices,
		focusedContexts:    make(map[string]bool),
		hasFocus:           len(focusedServiceNames) > 0,
		showDatabases:      showDatabases,
	}
}

// NewC4DiagramGeneratorWithFocusAndContexts creates a generator with service and context focus
func NewC4DiagramGeneratorWithFocusAndContexts(mode C4GenerationMode, focusedServiceNames []string, focusedContextNames []string, showDatabases bool) *C4DiagramGenerator {
	focusedServices := make(map[string]bool)
	for _, serviceName := range focusedServiceNames {
		focusedServices[serviceName] = true
	}

	focusedContexts := make(map[string]bool)
	for _, contextName := range focusedContextNames {
		focusedContexts[contextName] = true
	}

	return &C4DiagramGenerator{
		mode:               mode,
		systems:            make(map[string]*C4System),
		containers:         make(map[string]*C4Container),
		relations:          make([]C4Relation, 0),
		actors:             make(map[string]bool),
		systemRelations:    make([]C4Relation, 0),
		userInteractionMap: make(map[string][]string),
		focusedServices:    focusedServices,
		focusedContexts:    focusedContexts,
		hasFocus:           len(focusedServiceNames) > 0 || len(focusedContextNames) > 0,
		showDatabases:      showDatabases,
	}
}

// GenerateC4Diagram creates a redesigned C4 diagram
func (g *C4DiagramGenerator) GenerateC4Diagram(model *craft.CraftDoc, diagramType C4DiagramType) string {
	g.model = model
	g.reset()

	// Analyze and build systems
	g.analyzeModel()

	// Generate PlantUML
	return g.buildC4PlantUML(diagramType)
}

// reset clears the generator state
func (g *C4DiagramGenerator) reset() {
	g.systems = make(map[string]*C4System)
	g.containers = make(map[string]*C4Container)
	g.relations = make([]C4Relation, 0)
	g.actors = make(map[string]bool)
	g.systemRelations = make([]C4Relation, 0)
	g.userInteractionMap = make(map[string][]string)
	g.presentationSystem = nil
	g.gatewaySystem = nil
}

// analyzeModel processes the model with proper system separation
func (g *C4DiagramGenerator) analyzeModel() {
	// Step 1: Analyze use cases for user interactions
	g.analyzeUserInteractions()

	// Step 2: Create service systems
	g.createServiceSystems()

	// Step 3: Create infrastructure systems (presentation, gateway, events)
	g.createInfrastructureSystems()

	// Step 4: Create relationships
	g.createRelationships()
}

// analyzeUserInteractions detects user interactions in use cases
func (g *C4DiagramGenerator) analyzeUserInteractions() {
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			if g.isUserInteraction(scenario.Trigger) {
				// For boundaries mode: only the FIRST domain in the action chain should be externally accessible
				// For transparent mode: all domains are grouped in service so it doesn't matter
				if g.mode == C4ModeBoundaries {
					g.analyzeDirectlyAccessibleDomains(scenario)
				} else {
					// Original logic for transparent mode
					involvedDomains := g.extractDomainsFromActions(scenario.Actions)
					for _, domain := range involvedDomains {
						service := g.findServiceForDomain(domain)
						if service != "" {
							if g.userInteractionMap[domain] == nil {
								g.userInteractionMap[domain] = make([]string, 0)
							}
							if !g.containsString(g.userInteractionMap[domain], service) {
								g.userInteractionMap[domain] = append(g.userInteractionMap[domain], service)
							}
						}
					}
				}

				// Only add actors that interact with focused services (or all if no focus)
				// Skip actors that are actually bounded contexts defined in a service
				if scenario.Trigger.Actor != "" && g.findServiceForDomain(scenario.Trigger.Actor) == "" {
					shouldAddActor := !g.hasFocus // No focus - add all actors

					if g.hasFocus {
						// Focus mode - only add if actor interacts with focused services
						involvedDomains := g.extractDomainsFromActions(scenario.Actions)
						for _, domain := range involvedDomains {
							service := g.findServiceForDomain(domain)
							if service != "" && g.focusedServices[service] {
								shouldAddActor = true
								break
							}
						}
					}

					if shouldAddActor {
						g.actors[scenario.Trigger.Actor] = true
					}
				}
			}
		}
	}
}

// createServiceSystems creates separate systems for each service
func (g *C4DiagramGenerator) createServiceSystems() {
	for _, service := range g.model.Services {
		// In focus mode, mark non-focused services as external
		isExternal := g.hasFocus && !g.focusedServices[service.Name]

		system := &C4System{
			Name:        service.Name,
			Description: fmt.Sprintf("%s Service - Handles business logic", service.Name),
			Containers:  make([]string, 0),
			IsExternal:  isExternal,
		}

		if g.mode == C4ModeBoundaries {
			// Create domain containers within service system
			g.createDomainContainers(service, system)
		} else {
			// Create single application container (transparent mode)
			g.createApplicationContainer(service, system)
		}

		// Create database containers if enabled
		if g.showDatabases {
			g.createDatabaseContainers(service, system)
		}

		g.systems[service.Name] = system
	}
}

// createDomainContainers creates separate containers for each domain (boundaries mode)
func (g *C4DiagramGenerator) createDomainContainers(service craft.Service, system *C4System) {
	for _, domain := range service.Contexts {
		// containerName := fmt.Sprintf("%s_%s", service.Name, domain)
		container := &C4Container{
			Name:        domain,
			System:      service.Name,
			Technology:  g.getServiceTechnology(service.Language),
			Description: fmt.Sprintf("%s [Context]", domain),
			Domains:     []string{domain},
			DataStores:  make([]string, 0),
		}
		g.containers[domain] = container
		system.Containers = append(system.Containers, domain)
	}
}

// createApplicationContainer creates single application container (transparent mode)
func (g *C4DiagramGenerator) createApplicationContainer(service craft.Service, system *C4System) {
	if len(service.Contexts) > 0 {
		containerName := fmt.Sprintf("%s Application", service.Name)
		container := &C4Container{
			Name:       containerName,
			System:     service.Name,
			Technology: g.getServiceTechnology(service.Language),
			Description: fmt.Sprintf("Core business logic for %s domains: %s",
				service.Name, strings.Join(service.Contexts, ", ")),
			Domains:    service.Contexts,
			DataStores: make([]string, 0),
		}
		g.containers[containerName] = container
		system.Containers = append(system.Containers, containerName)
	}
}

// createDatabaseContainers creates database containers for each service
func (g *C4DiagramGenerator) createDatabaseContainers(service craft.Service, system *C4System) {
	for _, dataStore := range service.DataStores {
		containerName := fmt.Sprintf("%s_%s", service.Name, dataStore)
		container := &C4Container{
			Name:        containerName,
			System:      service.Name,
			Technology:  g.inferDatabaseType(dataStore),
			Description: fmt.Sprintf("Stores %s data", dataStore),
			Domains:     make([]string, 0),
			DataStores:  []string{dataStore},
		}
		g.containers[containerName] = container
		system.Containers = append(system.Containers, containerName)
	}
}

// createInfrastructureSystems creates presentation, gateway, and event systems
func (g *C4DiagramGenerator) createInfrastructureSystems() {
	// In focus mode, only create infrastructure if focused services have interactions
	shouldCreateInfrastructure := g.shouldCreateInfrastructure()

	if shouldCreateInfrastructure && g.hasArchitectureComponents() {
		g.createPresentationSystem()
		g.createGatewaySystem()
	}

	// Create event system if needed
	g.createEventSystemIfNeeded()
}

// shouldCreateInfrastructure determines if infrastructure should be created based on focus
func (g *C4DiagramGenerator) shouldCreateInfrastructure() bool {
	if !g.hasFocus {
		// No focus mode - use original logic
		return len(g.userInteractionMap) > 0
	}

	// Focus mode - only create if focused services have user interactions
	for _, services := range g.userInteractionMap {
		for _, serviceName := range services {
			if g.focusedServices[serviceName] {
				// At least one focused service has user interactions
				return true
			}
		}
	}

	return false
}

// createPresentationSystem creates the presentation system
func (g *C4DiagramGenerator) createPresentationSystem() {
	g.presentationSystem = &C4System{
		Name:        "Presentation",
		Description: "User Interface Layer",
		Containers:  make([]string, 0),
		IsExternal:  false,
	}

	for _, arch := range g.model.Architectures {
		for i, component := range arch.Presentation {
			containerName := g.generatePresentationContainerName(component, i)
			if _, exists := g.containers[containerName]; exists {
				continue
			}
			container := &C4Container{
				Name:        containerName,
				System:      "Presentation",
				Technology:  g.inferPresentationTechnology(component),
				Description: g.buildComponentDescription(component, "Presentation"),
				Domains:     make([]string, 0),
				DataStores:  make([]string, 0),
			}
			g.containers[containerName] = container
			g.presentationSystem.Containers = append(g.presentationSystem.Containers, containerName)
		}
	}

	if len(g.presentationSystem.Containers) > 0 {
		g.systems["Presentation"] = g.presentationSystem
	}
}

// createGatewaySystem creates the gateway system
func (g *C4DiagramGenerator) createGatewaySystem() {
	g.gatewaySystem = &C4System{
		Name:        "Gateway",
		Description: "Gateway Layer",
		Containers:  make([]string, 0),
		IsExternal:  false,
	}

	for _, arch := range g.model.Architectures {
		for i, component := range arch.Gateway {
			containerName := g.generateGatewayContainerName(component, i)
			if _, exists := g.containers[containerName]; exists {
				continue
			}
			container := &C4Container{
				Name:        containerName,
				System:      "Gateway",
				Technology:  g.inferGatewayTechnology(component),
				Description: g.buildComponentDescription(component, "Gateway"),
				Domains:     make([]string, 0),
				DataStores:  make([]string, 0),
			}
			g.containers[containerName] = container
			g.gatewaySystem.Containers = append(g.gatewaySystem.Containers, containerName)
		}
	}

	if len(g.gatewaySystem.Containers) > 0 {
		g.systems["Gateway"] = g.gatewaySystem
	}
}

// createEventSystemIfNeeded creates event system with queue container
func (g *C4DiagramGenerator) createEventSystemIfNeeded() {
	// Check if any async actions exist in focused services (or all if no focus)
	hasRelevantAsyncActions := false

	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Type == craft.ActionTypeAsync {
					if !g.hasFocus {
						// No focus mode - include all async actions
						hasRelevantAsyncActions = true
						break
					}

					// Focus mode - only include if action involves focused services
					actionService := g.findServiceForDomain(action.Context)
					if actionService != "" && g.focusedServices[actionService] {
						hasRelevantAsyncActions = true
						break
					}

					// Also check target domain
					if action.TargetContext != "" {
						targetService := g.findServiceForDomain(action.TargetContext)
						if targetService != "" && g.focusedServices[targetService] {
							hasRelevantAsyncActions = true
							break
						}
					}
				}
			}
			if hasRelevantAsyncActions {
				break
			}
		}
		if hasRelevantAsyncActions {
			break
		}
	}

	if hasRelevantAsyncActions {
		// Event system is internal if we have relevant async actions (used by focused services)
		eventSystem := &C4System{
			Name:        "Event_System",
			Description: "Event Processing Infrastructure",
			Containers:  make([]string, 0),
			IsExternal:  false,
		}

		// Create queue container
		queueContainer := &C4Container{
			Name:        "Event_Queue",
			System:      "Event_System",
			Technology:  "Message Queue",
			Description: "Handles asynchronous event processing and routing",
			Domains:     make([]string, 0),
			DataStores:  make([]string, 0),
		}

		g.containers["Event_Queue"] = queueContainer
		eventSystem.Containers = append(eventSystem.Containers, "Event_Queue")
		g.systems["Event_System"] = eventSystem
	}
}

// createRelationships creates all relationships in the architecture
func (g *C4DiagramGenerator) createRelationships() {
	if g.hasArchitectureComponents() {
		// Layered: User -> Presentation -> Gateway -> Services
		g.createUserToPresentationRelations()
		g.createPresentationToGatewayRelations()
		g.createGatewayToServiceRelations()
	} else {
		// Direct: User -> Services
		g.createDirectUserToServiceRelations()
	}

	// Create inter-service and internal relationships
	g.createServiceRelationships()
	if g.showDatabases {
		g.createDatabaseRelationships()
	}
	g.createEventRelationships()

	// Deduplicate relationships to prevent duplicate arrows
	g.deduplicateRelationships()
}

// deduplicateRelationships removes duplicate relationships based on From->To pairs
func (g *C4DiagramGenerator) deduplicateRelationships() {
	// Deduplicate system-level relationships
	g.systemRelations = g.deduplicateRelationshipSlice(g.systemRelations)

	// Deduplicate container-level relationships
	g.relations = g.deduplicateRelationshipSlice(g.relations)
}

// deduplicateRelationshipSlice removes duplicates from a relationship slice.
//
// The key MUST include Description: two events between the same domain pair
// (e.g. VasApplication→Event_Queue "VasApplied" and VasApplication→Event_Queue
// "VasApplicationFailed") share From/To but are distinct edges that must both
// render. Keying by (From,To) alone collapsed N events down to 1 per pair.
// We preserve insertion order so diagram layout stays stable run-to-run.
func (g *C4DiagramGenerator) deduplicateRelationshipSlice(relationships []C4Relation) []C4Relation {
	seen := make(map[string]struct{}, len(relationships))
	result := make([]C4Relation, 0, len(relationships))

	for _, relation := range relationships {
		key := fmt.Sprintf("%s->%s|%s", relation.From, relation.To, relation.Description)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, relation)
	}

	return result
}

// Helper methods (continuing in next part due to length)
func (g *C4DiagramGenerator) hasArchitectureComponents() bool {
	for _, arch := range g.model.Architectures {
		if len(arch.Presentation) > 0 || len(arch.Gateway) > 0 {
			return true
		}
	}
	return false
}

func (g *C4DiagramGenerator) isUserInteraction(trigger craft.Trigger) bool {
	return trigger.Type == craft.TriggerTypeExternal &&
		trigger.Actor != ""
}

func (g *C4DiagramGenerator) extractDomainsFromActions(actions []craft.Action) []string {
	domains := make([]string, 0)
	seen := make(map[string]bool)

	for _, action := range actions {
		if action.Context != "" && !seen[action.Context] {
			domains = append(domains, action.Context)
			seen[action.Context] = true
		}
		if action.TargetContext != "" && !seen[action.TargetContext] {
			domains = append(domains, action.TargetContext)
			seen[action.TargetContext] = true
		}
	}

	return domains
}

func (g *C4DiagramGenerator) containsString(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func (g *C4DiagramGenerator) findServiceForDomain(domain string) string {
	for _, service := range g.model.Services {
		if slices.Contains(service.Contexts, domain) {
			return service.Name
		}
	}
	return ""
}

// analyzeDirectlyAccessibleDomains identifies domains that should be directly accessible via gateway
func (g *C4DiagramGenerator) analyzeDirectlyAccessibleDomains(scenario craft.Scenario) {
	// Find the first domain that is actually triggered by user action
	// This is typically the first action in the scenario
	for _, action := range scenario.Actions {
		if action.Context != "" {
			// Only the first domain encountered should be externally accessible
			service := g.findServiceForDomain(action.Context)
			if service != "" {
				if g.userInteractionMap[action.Context] == nil {
					g.userInteractionMap[action.Context] = make([]string, 0)
				}
				if !g.containsString(g.userInteractionMap[action.Context], service) {
					g.userInteractionMap[action.Context] = append(g.userInteractionMap[action.Context], service)
				}
			}
			// Only process the first domain, break after that
			break
		}
	}
}

// Main generation functions
func GenerateC4ContextDiagram(model *craft.CraftDoc, mode C4GenerationMode, showDatabases bool) string {
	generator := NewC4DiagramGenerator(mode, showDatabases)
	return generator.GenerateC4Diagram(model, C4Context)
}

func GenerateC4ContainerDiagram(model *craft.CraftDoc, mode C4GenerationMode, showDatabases bool) string {
	generator := NewC4DiagramGenerator(mode, showDatabases)
	return generator.GenerateC4Diagram(model, C4Containers)
}

func GenerateC4ContainerDiagramWithFocusAndContexts(model *craft.CraftDoc, mode C4GenerationMode, focusedServiceNames []string, focusedContextNames []string, showDatabases bool) string {
	generator := NewC4DiagramGeneratorWithFocusAndContexts(mode, focusedServiceNames, focusedContextNames, showDatabases)
	return generator.GenerateC4Diagram(model, C4Containers)
}

func GenerateC4ComponentDiagram(model *craft.CraftDoc, mode C4GenerationMode, showDatabases bool) string {
	generator := NewC4DiagramGenerator(mode, showDatabases)
	return generator.GenerateC4Diagram(model, C4Components)
}
