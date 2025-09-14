package visualizer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tcarcao/craft/internal/parser"
)

// C4GenerationMode determines how domains are represented
type C4GenerationMode string

const (
	C4ModeTransparent C4GenerationMode = "transparent" // Current: domains grouped transparently in services
	C4ModeBoundaries  C4GenerationMode = "boundaries"  // New: domains as Container_Boundary within services
)

// C4DiagramGenerator generates C4 diagrams with proper system separation
type C4DiagramGenerator struct {
	model              *parser.DSLModel
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
	focusedSubDomains  map[string]bool // SubDomains to show as internal
	hasFocus           bool            // Whether focus mode is enabled
}

// NewC4DiagramGenerator creates a new redesigned generator
func NewC4DiagramGenerator(mode C4GenerationMode) *C4DiagramGenerator {
	return &C4DiagramGenerator{
		mode:               mode,
		systems:            make(map[string]*C4System),
		containers:         make(map[string]*C4Container),
		relations:          make([]C4Relation, 0),
		actors:             make(map[string]bool),
		systemRelations:    make([]C4Relation, 0),
		userInteractionMap: make(map[string][]string),
		focusedServices:    make(map[string]bool),
		focusedSubDomains:  make(map[string]bool),
		hasFocus:           false,
	}
}

// NewC4DiagramGeneratorWithFocus creates a generator with service focus
func NewC4DiagramGeneratorWithFocus(mode C4GenerationMode, focusedServiceNames []string) *C4DiagramGenerator {
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
		focusedSubDomains:  make(map[string]bool),
		hasFocus:           len(focusedServiceNames) > 0,
	}
}

// NewC4DiagramGeneratorWithFocusAndSubDomains creates a generator with service and subdomain focus
func NewC4DiagramGeneratorWithFocusAndSubDomains(mode C4GenerationMode, focusedServiceNames []string, focusedSubDomainNames []string) *C4DiagramGenerator {
	focusedServices := make(map[string]bool)
	for _, serviceName := range focusedServiceNames {
		focusedServices[serviceName] = true
	}

	focusedSubDomains := make(map[string]bool)
	for _, subDomainName := range focusedSubDomainNames {
		focusedSubDomains[subDomainName] = true
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
		focusedSubDomains:  focusedSubDomains,
		hasFocus:           len(focusedServiceNames) > 0 || len(focusedSubDomainNames) > 0,
	}
}

// GenerateC4Diagram creates a redesigned C4 diagram
func (g *C4DiagramGenerator) GenerateC4Diagram(model *parser.DSLModel, diagramType C4DiagramType) string {
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
				if scenario.Trigger.Actor != "" && !strings.HasPrefix(strings.ToUpper(scenario.Trigger.Actor), "CRON") {
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

		// Create database containers
		g.createDatabaseContainers(service, system)

		g.systems[service.Name] = system
	}
}

// createDomainContainers creates separate containers for each domain (boundaries mode)
func (g *C4DiagramGenerator) createDomainContainers(service parser.Service, system *C4System) {
	for _, domain := range service.Domains {
		// containerName := fmt.Sprintf("%s_%s", service.Name, domain)
		container := &C4Container{
			Name:        domain,
			System:      service.Name,
			Technology:  g.getServiceTechnology(service.Language),
			Description: fmt.Sprintf("%s domain logic", domain),
			Domains:     []string{domain},
			DataStores:  make([]string, 0),
		}
		g.containers[domain] = container
		system.Containers = append(system.Containers, domain)
	}
}

// createApplicationContainer creates single application container (transparent mode)
func (g *C4DiagramGenerator) createApplicationContainer(service parser.Service, system *C4System) {
	if len(service.Domains) > 0 {
		containerName := fmt.Sprintf("%s Application", service.Name)
		container := &C4Container{
			Name:       containerName,
			System:     service.Name,
			Technology: g.getServiceTechnology(service.Language),
			Description: fmt.Sprintf("Core business logic for %s domains: %s",
				service.Name, strings.Join(service.Domains, ", ")),
			Domains:    service.Domains,
			DataStores: make([]string, 0),
		}
		g.containers[containerName] = container
		system.Containers = append(system.Containers, containerName)
	}
}

// createDatabaseContainers creates database containers for each service
func (g *C4DiagramGenerator) createDatabaseContainers(service parser.Service, system *C4System) {
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
				if action.Type == parser.ActionTypeAsync {
					if !g.hasFocus {
						// No focus mode - include all async actions
						hasRelevantAsyncActions = true
						break
					}

					// Focus mode - only include if action involves focused services
					actionService := g.findServiceForDomain(action.Domain)
					if actionService != "" && g.focusedServices[actionService] {
						hasRelevantAsyncActions = true
						break
					}

					// Also check target domain
					if action.TargetDomain != "" {
						targetService := g.findServiceForDomain(action.TargetDomain)
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
	g.createDatabaseRelationships()
	g.createEventRelationships()
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

func (g *C4DiagramGenerator) isUserInteraction(trigger parser.Trigger) bool {
	return trigger.Type == parser.TriggerTypeExternal &&
		trigger.Actor != "" &&
		!strings.HasPrefix(strings.ToUpper(trigger.Actor), "CRON")
}

func (g *C4DiagramGenerator) extractDomainsFromActions(actions []parser.Action) []string {
	domains := make([]string, 0)
	seen := make(map[string]bool)

	for _, action := range actions {
		if action.Domain != "" && !seen[action.Domain] {
			domains = append(domains, action.Domain)
			seen[action.Domain] = true
		}
		if action.TargetDomain != "" && !seen[action.TargetDomain] {
			domains = append(domains, action.TargetDomain)
			seen[action.TargetDomain] = true
		}
	}

	return domains
}

func (g *C4DiagramGenerator) containsString(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

func (g *C4DiagramGenerator) findServiceForDomain(domain string) string {
	for _, service := range g.model.Services {
		if slices.Contains(service.Domains, domain) {
			return service.Name
		}
	}
	return ""
}

// analyzeDirectlyAccessibleDomains identifies domains that should be directly accessible via gateway
func (g *C4DiagramGenerator) analyzeDirectlyAccessibleDomains(scenario parser.Scenario) {
	// Find the first domain that is actually triggered by user action
	// This is typically the first action in the scenario
	for _, action := range scenario.Actions {
		if action.Domain != "" {
			// Only the first domain encountered should be externally accessible
			service := g.findServiceForDomain(action.Domain)
			if service != "" {
				if g.userInteractionMap[action.Domain] == nil {
					g.userInteractionMap[action.Domain] = make([]string, 0)
				}
				if !g.containsString(g.userInteractionMap[action.Domain], service) {
					g.userInteractionMap[action.Domain] = append(g.userInteractionMap[action.Domain], service)
				}
			}
			// Only process the first domain, break after that
			break
		}
	}
}

// Main generation functions
func GenerateC4ContextDiagram(model *parser.DSLModel, mode C4GenerationMode) string {
	generator := NewC4DiagramGenerator(mode)
	return generator.GenerateC4Diagram(model, C4Context)
}

func GenerateC4ContainerDiagram(model *parser.DSLModel, mode C4GenerationMode) string {
	generator := NewC4DiagramGenerator(mode)
	return generator.GenerateC4Diagram(model, C4Containers)
}

func GenerateC4ContainerDiagramWithFocusAndSubDomains(model *parser.DSLModel, mode C4GenerationMode, focusedServiceNames []string, focusedSubDomainNames []string) string {
	generator := NewC4DiagramGeneratorWithFocusAndSubDomains(mode, focusedServiceNames, focusedSubDomainNames)
	return generator.GenerateC4Diagram(model, C4Containers)
}

func GenerateC4ComponentDiagram(model *parser.DSLModel, mode C4GenerationMode) string {
	generator := NewC4DiagramGenerator(mode)
	return generator.GenerateC4Diagram(model, C4Components)
}
