package visualizer

import (
	"fmt"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
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

				if scenario.Trigger.Actor != "" && !strings.HasPrefix(strings.ToUpper(scenario.Trigger.Actor), "CRON") {
					g.actors[scenario.Trigger.Actor] = true
				}
			}
		}
	}
}

// createServiceSystems creates separate systems for each service
func (g *C4DiagramGenerator) createServiceSystems() {
	for _, service := range g.model.Services {
		system := &C4System{
			Name:        service.Name,
			Description: fmt.Sprintf("%s Service - Handles business logic", service.Name),
			Containers:  make([]string, 0),
			IsExternal:  false,
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
	// Only create if there are user interactions and architecture components exist
	if len(g.userInteractionMap) > 0 && g.hasArchitectureComponents() {
		g.createPresentationSystem()
		g.createGatewaySystem()
	}

	// Create event system if needed
	g.createEventSystemIfNeeded()
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
	// Check if any async actions exist
	hasAsyncActions := false
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Type == parser.ActionTypeAsync {
					hasAsyncActions = true
					break
				}
			}
		}
	}

	if hasAsyncActions {
		eventSystem := &C4System{
			Name:        "Event_System",
			Description: "Event Processing Infrastructure",
			Containers:  make([]string, 0),
			IsExternal:  true,
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
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (g *C4DiagramGenerator) findServiceForDomain(domain string) string {
	for _, service := range g.model.Services {
		for _, serviceDomain := range service.Domains {
			if serviceDomain == domain {
				return service.Name
			}
		}
	}
	return ""
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

func GenerateC4ComponentDiagram(model *parser.DSLModel, mode C4GenerationMode) string {
	generator := NewC4DiagramGenerator(mode)
	return generator.GenerateC4Diagram(model, C4Components)
}
