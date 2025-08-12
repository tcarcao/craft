package visualizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

func (v *Visualizer) GenerateC4(arch *parser.DSLModel) ([]byte, error) {
	diagram := GenerateC4ContainerDiagram(arch)
	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

// C4DiagramGenerator generates C4 diagrams from DSL models
type C4DiagramGenerator struct {
	model           *parser.DSLModel
	systems         map[string]*C4System
	containers      map[string]*C4Container
	relations       []C4Relation
	actors          map[string]bool
	actorToSystem   map[string]string // actor -> target system
	systemRelations []C4Relation      // system-level relationships
}

// C4System represents a software system in C4
type C4System struct {
	Name        string
	Description string
	Containers  []string
	IsExternal  bool
}

// C4Container represents a container within a system
type C4Container struct {
	Name        string
	System      string
	Technology  string
	Description string
	Domains     []string
	DataStores  []string
}

// C4Relation represents a relationship between C4 elements
type C4Relation struct {
	From        string
	To          string
	Description string
	Technology  string
	Type        string // "uses", "reads", "writes", "triggers"
}

// C4DiagramType determines the level of C4 diagram to generate
type C4DiagramType string

const (
	C4Context    C4DiagramType = "context"   // System Context level
	C4Containers C4DiagramType = "container" // Container level
	C4Components C4DiagramType = "component" // Component level (domains as components)
)

// NewC4DiagramGenerator creates a new C4 diagram generator
func NewC4DiagramGenerator() *C4DiagramGenerator {
	return &C4DiagramGenerator{
		systems:         make(map[string]*C4System),
		containers:      make(map[string]*C4Container),
		relations:       make([]C4Relation, 0),
		actors:          make(map[string]bool),
		actorToSystem:   make(map[string]string),
		systemRelations: make([]C4Relation, 0),
	}
}

// GenerateC4Diagram creates a C4 diagram from the DSL model
func (g *C4DiagramGenerator) GenerateC4Diagram(model *parser.DSLModel, diagramType C4DiagramType) string {
	g.model = model
	g.reset()

	// Analyze the model and build C4 elements
	g.analyzeModel()

	// Generate PlantUML C4 diagram
	return g.buildC4PlantUML(diagramType)
}

// reset clears the generator state
func (g *C4DiagramGenerator) reset() {
	g.systems = make(map[string]*C4System)
	g.containers = make(map[string]*C4Container)
	g.relations = make([]C4Relation, 0)
	g.actors = make(map[string]bool)
	g.actorToSystem = make(map[string]string)
	g.systemRelations = make([]C4Relation, 0)
}

// analyzeModel processes the DSL model to extract C4 elements
func (g *C4DiagramGenerator) analyzeModel() {
	// If we have services, use them as primary structure
	if len(g.model.Services) > 0 {
		g.analyzeServices()
	}

	// Analyze use cases for actors and relationships
	if len(g.model.UseCases) > 0 {
		g.analyzeUseCases()
	}

	// If no services but have use cases, infer structure from use cases
	if len(g.model.Services) == 0 && len(g.model.UseCases) > 0 {
		g.inferServicesFromUseCases()
	}
}

// analyzeServices processes explicit service definitions
func (g *C4DiagramGenerator) analyzeServices() {
	for _, service := range g.model.Services {
		// Create system for each service
		system := &C4System{
			Name:        service.Name,
			Description: fmt.Sprintf("Handles %s related functionality", service.Name),
			Containers:  make([]string, 0),
			IsExternal:  false,
		}

		// Create containers within the service
		if len(service.Domains) > 0 {
			// Main application container with language-specific technology
			containerName := fmt.Sprintf("%s Application", service.Name)
			technology := g.getServiceTechnology(service.Language)
			container := &C4Container{
				Name:        containerName,
				System:      service.Name,
				Technology:  technology,
				Description: fmt.Sprintf("Implements %s business logic", service.Name),
				Domains:     service.Domains,
				DataStores:  make([]string, 0),
			}
			g.containers[containerName] = container
			system.Containers = append(system.Containers, containerName)
		}

		// Create database containers for data stores
		for _, dataStore := range service.DataStores {
			containerName := fmt.Sprintf("%s_%s", service.Name, dataStore)
			container := &C4Container{
				Name:        containerName,
				System:      service.Name,
				Technology:  g.inferDatabaseType(dataStore),
				Description: fmt.Sprintf("Stores %s data for %s", dataStore, service.Name),
				Domains:     make([]string, 0),
				DataStores:  []string{dataStore},
			}
			g.containers[containerName] = container
			system.Containers = append(system.Containers, containerName)
		}

		g.systems[service.Name] = system
	}
}

// analyzeUseCases extracts actors and relationships from use cases
func (g *C4DiagramGenerator) analyzeUseCases() {
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			// Extract actors and create connections
			if scenario.Trigger.Type == parser.TriggerTypeExternal && scenario.Trigger.Actor != "" {
				actor := scenario.Trigger.Actor

				// Handle CRON actors - create service-specific CRONs
				if strings.ToUpper(actor) == "CRON" {
					// Find which domain is triggered first
					if len(scenario.Actions) > 0 {
						firstAction := scenario.Actions[0]
						if firstAction.Domain != "" {
							// Create service-specific CRON
							serviceForDomain := g.findServiceForDomain(firstAction.Domain)
							if serviceForDomain != "" {
								cronActor := fmt.Sprintf("CRON_%s", serviceForDomain)
								g.actors[cronActor] = true

								// Create relationship from service-specific CRON to system
								g.createActorToSystemRelationship(cronActor, serviceForDomain, scenario.Trigger)
							}
						}
					}
				} else {
					// Regular external actor
					g.actors[actor] = true

					// Find target system and create relationship
					if len(scenario.Actions) > 0 {
						firstAction := scenario.Actions[0]
						if firstAction.Domain != "" {
							serviceForDomain := g.findServiceForDomain(firstAction.Domain)
							if serviceForDomain != "" {
								g.createActorToSystemRelationship(actor, serviceForDomain, scenario.Trigger)
							}
						}
					}
				}
			}

			// Extract relationships from actions
			for _, action := range scenario.Actions {
				g.extractRelationshipFromAction(action, useCase.Name)
			}
		}
	}
}

// inferServicesFromUseCases creates services based on domain usage in use cases
func (g *C4DiagramGenerator) inferServicesFromUseCases() {
	domainUsage := make(map[string]int)

	// Count domain usage frequency
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Domain != "" {
					domainUsage[action.Domain]++
				}
				if action.TargetDomain != "" {
					domainUsage[action.TargetDomain]++
				}
			}
		}
	}

	// Group related domains into inferred services
	serviceGroups := g.groupDomainsByAffinity()

	for serviceName, domains := range serviceGroups {
		system := &C4System{
			Name:        serviceName,
			Description: fmt.Sprintf("Inferred service containing %s", strings.Join(domains, ", ")),
			Containers:  make([]string, 0),
			IsExternal:  false,
		}

		containerName := fmt.Sprintf("%s Application", serviceName)
		container := &C4Container{
			Name:        containerName,
			System:      serviceName,
			Technology:  "Application",
			Description: fmt.Sprintf("Handles %s functionality", serviceName),
			Domains:     domains,
			DataStores:  make([]string, 0),
		}

		g.containers[containerName] = container
		system.Containers = append(system.Containers, containerName)
		g.systems[serviceName] = system
	}
}

// groupDomainsByAffinity groups domains that frequently interact
func (g *C4DiagramGenerator) groupDomainsByAffinity() map[string][]string {
	// Simple heuristic: group domains by common prefixes or frequent interactions
	groups := make(map[string][]string)
	processed := make(map[string]bool)

	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Domain != "" && !processed[action.Domain] {
					groupName := g.inferServiceName(action.Domain)
					if groups[groupName] == nil {
						groups[groupName] = make([]string, 0)
					}
					groups[groupName] = append(groups[groupName], action.Domain)
					processed[action.Domain] = true
				}
			}
		}
	}

	return groups
}

// inferServiceName creates a service name from a domain name
func (g *C4DiagramGenerator) inferServiceName(domain string) string {
	// Extract base name by removing common suffixes
	name := domain
	suffixes := []string{"Management", "Service", "Handler", "Processor", "Controller"}

	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	return name + " Service"
}

// createActorToSystemRelationship creates a relationship from actor to system
func (g *C4DiagramGenerator) createActorToSystemRelationship(actor, targetSystem string, trigger parser.Trigger) {
	g.actorToSystem[actor] = targetSystem

	description := fmt.Sprintf("%s %s", trigger.Verb, trigger.Phrase)
	if description == " " {
		description = "triggers"
	}

	relation := C4Relation{
		From:        actor,
		To:          targetSystem,
		Description: description,
		Technology:  "External Trigger",
		Type:        "triggers",
	}
	g.systemRelations = append(g.systemRelations, relation)
}

// findServiceForDomain finds which service a domain belongs to
func (g *C4DiagramGenerator) findServiceForDomain(domain string) string {
	for _, service := range g.model.Services {
		for _, serviceDomain := range service.Domains {
			if serviceDomain == domain {
				return service.Name
			}
		}
	}

	// If no explicit service, try to find in inferred systems
	for systemName, system := range g.systems {
		for _, containerName := range system.Containers {
			if container := g.containers[containerName]; container != nil {
				for _, d := range container.Domains {
					if d == domain {
						return systemName
					}
				}
			}
		}
	}

	return ""
}

// extractRelationshipFromAction creates relationships from actions
func (g *C4DiagramGenerator) extractRelationshipFromAction(action parser.Action, useCase string) {
	switch action.Type {
	case parser.ActionTypeSync:
		if action.Domain != "" && action.TargetDomain != "" {
			relation := C4Relation{
				From:        action.Domain,
				To:          action.TargetDomain,
				Description: action.Phrase,
				Technology:  "Synchronous Call",
				Type:        "uses",
			}
			g.relations = append(g.relations, relation)
		}
	case parser.ActionTypeAsync:
		if action.Domain != "" && action.Event != "" {
			relation := C4Relation{
				From:        action.Domain,
				To:          "Event System",
				Description: action.Event,
				Technology:  "Event Publishing",
				Type:        "triggers",
			}
			g.relations = append(g.relations, relation)
		}
	}
}

// getServiceTechnology returns the technology description based on language
func (g *C4DiagramGenerator) getServiceTechnology(language string) string {
	if language == "" {
		return "Application"
	}

	lowerLang := strings.ToLower(language)

	if strings.Contains(lowerLang, "go") || strings.Contains(lowerLang, "golang") {
		return "Go Application"
	}
	if strings.Contains(lowerLang, "java") {
		return "Java Application"
	}
	if strings.Contains(lowerLang, "python") {
		return "Python Application"
	}
	if strings.Contains(lowerLang, "javascript") || strings.Contains(lowerLang, "js") || strings.Contains(lowerLang, "node") {
		return "Node.js Application"
	}
	if strings.Contains(lowerLang, "typescript") || strings.Contains(lowerLang, "ts") {
		return "TypeScript Application"
	}
	if strings.Contains(lowerLang, "rust") {
		return "Rust Application"
	}
	if strings.Contains(lowerLang, "csharp") || strings.Contains(lowerLang, "c#") || strings.Contains(lowerLang, "dotnet") {
		return ".NET Application"
	}
	if strings.Contains(lowerLang, "php") {
		return "PHP Application"
	}
	if strings.Contains(lowerLang, "ruby") {
		return "Ruby Application"
	}
	if strings.Contains(lowerLang, "kotlin") {
		return "Kotlin Application"
	}
	if strings.Contains(lowerLang, "swift") {
		return "Swift Application"
	}

	// Default fallback
	return fmt.Sprintf("%s Application", strings.Title(language))
}

// inferDatabaseType infers the database technology from datastore name
func (g *C4DiagramGenerator) inferDatabaseType(dataStore string) string {
	lowerStore := strings.ToLower(dataStore)

	// Common database types
	if strings.Contains(lowerStore, "postgres") || strings.Contains(lowerStore, "pg") {
		return "PostgreSQL Database"
	}
	if strings.Contains(lowerStore, "mysql") {
		return "MySQL Database"
	}
	if strings.Contains(lowerStore, "redis") || strings.Contains(lowerStore, "cache") {
		return "Redis Cache"
	}
	if strings.Contains(lowerStore, "mongo") {
		return "MongoDB Database"
	}
	if strings.Contains(lowerStore, "queue") || strings.Contains(lowerStore, "mq") {
		return "Message Queue"
	}
	if strings.Contains(lowerStore, "elastic") || strings.Contains(lowerStore, "search") {
		return "Elasticsearch"
	}
	if strings.HasSuffix(lowerStore, "_db") || strings.HasSuffix(lowerStore, "db") {
		return "Database"
	}

	// Default fallback
	return "Data Store"
}

// isDatabaseContainer checks if a container represents a database/data store
func (g *C4DiagramGenerator) isDatabaseContainer(container *C4Container) bool {
	return len(container.DataStores) > 0
}

// getDatabaseIcon returns the appropriate icon for database types
func (g *C4DiagramGenerator) getDatabaseIcon(technology string) string {
	lowerTech := strings.ToLower(technology)

	// Return icon parameter for C4 PlantUML - using correct sprite names
	if strings.Contains(lowerTech, "postgresql") || strings.Contains(lowerTech, "postgres") {
		return ", $sprite=\"postgresql\""
	}
	if strings.Contains(lowerTech, "mysql") {
		return ", $sprite=\"mysql\""
	}
	if strings.Contains(lowerTech, "redis") {
		return ", $sprite=\"redis\""
	}
	if strings.Contains(lowerTech, "mongodb") || strings.Contains(lowerTech, "mongo") {
		return ", $sprite=\"mongodb\""
	}
	if strings.Contains(lowerTech, "elasticsearch") || strings.Contains(lowerTech, "elastic") {
		return ", $sprite=\"database\"" // Using generic database icon for Elasticsearch
	}
	if strings.Contains(lowerTech, "queue") || strings.Contains(lowerTech, "message") {
		return ", $sprite=\"database\"" // Using generic database icon for queues
	}

	// Default database icon
	return ", $sprite=\"database\""
}

// getServiceIcon returns the appropriate icon for service languages
func (g *C4DiagramGenerator) getServiceIcon(language string) string {
	if language == "" {
		return ""
	}

	lowerLang := strings.ToLower(language)

	// Return icon parameter for C4 PlantUML - using correct sprite names
	if strings.Contains(lowerLang, "go") || strings.Contains(lowerLang, "golang") {
		return ", $sprite=\"go\""
	}
	if strings.Contains(lowerLang, "java") {
		return ", $sprite=\"java\""
	}
	if strings.Contains(lowerLang, "python") {
		return ", $sprite=\"python\""
	}
	if strings.Contains(lowerLang, "javascript") || strings.Contains(lowerLang, "js") || strings.Contains(lowerLang, "node") {
		return ", $sprite=\"nodejs\""
	}
	if strings.Contains(lowerLang, "typescript") || strings.Contains(lowerLang, "ts") {
		return ", $sprite=\"javascript\"" // Using javascript icon for TypeScript
	}
	if strings.Contains(lowerLang, "rust") {
		return ", $sprite=\"rust_original\""
	}
	if strings.Contains(lowerLang, "csharp") || strings.Contains(lowerLang, "c#") || strings.Contains(lowerLang, "dotnet") {
		return ", $sprite=\"dot_net\""
	}
	if strings.Contains(lowerLang, "php") {
		return ", $sprite=\"php\""
	}
	if strings.Contains(lowerLang, "ruby") {
		return ", $sprite=\"ruby\""
	}
	if strings.Contains(lowerLang, "kotlin") {
		return ", $sprite=\"kotlin\""
	}
	if strings.Contains(lowerLang, "swift") {
		return ", $sprite=\"swift\""
	}

	// Default to a generic application icon
	return ", $sprite=\"code\""
}

// getSystemIcon returns the appropriate icon for a system based on its service language
func (g *C4DiagramGenerator) getSystemIcon(systemName string) string {
	// Find the service for this system
	for _, service := range g.model.Services {
		if service.Name == systemName {
			return g.getServiceIcon(service.Language)
		}
	}
	return ""
}

// getContainerIcon returns the appropriate icon for a container based on its service language
func (g *C4DiagramGenerator) getContainerIcon(containerName, systemName string) string {
	// Find the service for this system
	for _, service := range g.model.Services {
		if service.Name == systemName {
			// Only show icon for application containers, not databases
			if strings.Contains(containerName, "Application") {
				return g.getServiceIcon(service.Language)
			}
		}
	}
	return ""
}

// buildC4PlantUML generates the PlantUML C4 diagram
func (g *C4DiagramGenerator) buildC4PlantUML(diagramType C4DiagramType) string {
	var sb strings.Builder

	// C4 PlantUML header
	sb.WriteString("@startuml\n")

	switch diagramType {
	case C4Context:
		sb.WriteString("!include <C4/C4_Context.puml>\n")
		sb.WriteString("!include <tupadr3/devicons/database>\n")
		sb.WriteString("!include <tupadr3/devicons2/postgresql>\n")
		sb.WriteString("!include <tupadr3/devicons/mysql>\n")
		sb.WriteString("!include <tupadr3/devicons2/redis>\n")
		sb.WriteString("!include <tupadr3/devicons2/mongodb>\n")
		sb.WriteString("!include <tupadr3/devicons2/go>\n")
		sb.WriteString("!include <tupadr3/devicons2/java>\n")
		sb.WriteString("!include <tupadr3/devicons2/python>\n")
		sb.WriteString("!include <tupadr3/devicons2/nodejs>\n")
		sb.WriteString("!include <tupadr3/devicons2/javascript>\n")
		sb.WriteString("!include <tupadr3/devicons2/rust_original>\n")
		sb.WriteString("!include <tupadr3/devicons2/dot_net>\n")
		sb.WriteString("!include <tupadr3/devicons2/php>\n")
		sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
		sb.WriteString("!include <tupadr3/devicons2/kotlin>\n")
		sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
		sb.WriteString("!include <tupadr3/font-awesome-5/code>\n")
		sb.WriteString("\n\nLAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString("title System Context Diagram\n\n")
		g.buildContextDiagram(&sb)
	case C4Containers:
		sb.WriteString("!include <C4/C4_Container.puml>\n")
		sb.WriteString("!include <tupadr3/devicons/database>\n")
		sb.WriteString("!include <tupadr3/devicons2/postgresql>\n")
		sb.WriteString("!include <tupadr3/devicons/mysql>\n")
		sb.WriteString("!include <tupadr3/devicons2/redis>\n")
		sb.WriteString("!include <tupadr3/devicons2/mongodb>\n")
		sb.WriteString("!include <tupadr3/devicons2/go>\n")
		sb.WriteString("!include <tupadr3/devicons2/java>\n")
		sb.WriteString("!include <tupadr3/devicons2/python>\n")
		sb.WriteString("!include <tupadr3/devicons2/nodejs>\n")
		sb.WriteString("!include <tupadr3/devicons2/javascript>\n")
		sb.WriteString("!include <tupadr3/devicons2/rust_original>\n")
		sb.WriteString("!include <tupadr3/devicons2/dot_net>\n")
		sb.WriteString("!include <tupadr3/devicons2/php>\n")
		sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
		sb.WriteString("!include <tupadr3/devicons2/kotlin>\n")
		sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
		sb.WriteString("!include <tupadr3/font-awesome-5/code>\n")
		sb.WriteString("\n\nLAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString("title Container Diagram\n\n")
		g.buildContainerDiagram(&sb)
	case C4Components:
		sb.WriteString("!include <C4/C4_Component.puml>\n\n")
		sb.WriteString("LAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString("title Component Diagram\n\n")
		g.buildComponentDiagram(&sb)
	}

	sb.WriteString("\n@enduml")
	return sb.String()
}

// buildContextDiagram builds a system context diagram
func (g *C4DiagramGenerator) buildContextDiagram(sb *strings.Builder) {
	// Add actors (external persons and CRONs)
	actors := g.getSortedActors()
	for _, actor := range actors {
		if strings.HasPrefix(strings.ToUpper(actor), "CRON") {
			// CRON actors are systems, not persons
			sb.WriteString(fmt.Sprintf("System_Ext(%s, \"%s\", \"Scheduled task system\")\n",
				g.sanitizeIdentifier(actor), actor))
		} else {
			sb.WriteString(fmt.Sprintf("Person(%s, \"%s\", \"External user or system\")\n",
				g.sanitizeIdentifier(actor), actor))
		}
	}
	if len(actors) > 0 {
		sb.WriteString("\n")
	}

	// Add systems
	systems := g.getSortedSystems()
	for _, systemName := range systems {
		system := g.systems[systemName]
		// Find the service to get its language for the icon
		serviceIcon := g.getSystemIcon(systemName)
		sb.WriteString(fmt.Sprintf("System(%s, \"%s\", \"%s\"%s)\n",
			g.sanitizeIdentifier(systemName), systemName, system.Description, serviceIcon))
	}
	sb.WriteString("\n")

	// Add system-level relationships (actor to system)
	for _, relation := range g.systemRelations {
		sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
			g.sanitizeIdentifier(relation.From),
			g.sanitizeIdentifier(relation.To),
			relation.Description))
	}
}

// buildContainerDiagram builds a container diagram
func (g *C4DiagramGenerator) buildContainerDiagram(sb *strings.Builder) {
	// Add actors
	actors := g.getSortedActors()
	for _, actor := range actors {
		if strings.HasPrefix(strings.ToUpper(actor), "CRON") {
			// CRON actors are external systems
			sb.WriteString(fmt.Sprintf("System_Ext(%s, \"%s\", \"Scheduled task system\")\n",
				g.sanitizeIdentifier(actor), actor))
		} else {
			sb.WriteString(fmt.Sprintf("Person(%s, \"%s\", \"External user\")\n",
				g.sanitizeIdentifier(actor), actor))
		}
	}
	if len(actors) > 0 {
		sb.WriteString("\n")
	}

	// Add systems and their containers
	systems := g.getSortedSystems()
	for _, systemName := range systems {
		system := g.systems[systemName]

		sb.WriteString(fmt.Sprintf("System_Boundary(%s_boundary, \"%s\") {\n",
			g.sanitizeIdentifier(systemName), systemName))

		for _, containerName := range system.Containers {
			container := g.containers[containerName]
			if g.isDatabaseContainer(container) {
				// Use ContainerDb for database containers (cylinder shape)
				icon := g.getDatabaseIcon(container.Technology)
				sb.WriteString(fmt.Sprintf("    ContainerDb(%s, \"%s\", \"%s\", \"%s\"%s)\n",
					g.sanitizeIdentifier(containerName), containerName,
					container.Technology, container.Description, icon))
			} else {
				// Regular container for applications with language icon
				serviceIcon := g.getContainerIcon(containerName, container.System)
				sb.WriteString(fmt.Sprintf("    Container(%s, \"%s\", \"%s\", \"%s\"%s)\n",
					g.sanitizeIdentifier(containerName), containerName,
					container.Technology, container.Description, serviceIcon))
			}
		}

		sb.WriteString("}\n\n")
	}

	// Add system-level relationships (actor to system)
	for _, relation := range g.systemRelations {
		// For container diagram, connect to the first container of the target system
		targetSystem := relation.To
		if system := g.systems[targetSystem]; system != nil && len(system.Containers) > 0 {
			firstContainer := system.Containers[0]
			sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
				g.sanitizeIdentifier(relation.From),
				g.sanitizeIdentifier(firstContainer),
				relation.Description))
		}
	}

	// Add container relationships
	g.buildContainerRelationships(sb)
}

// buildComponentDiagram builds a component diagram (domains as components)
func (g *C4DiagramGenerator) buildComponentDiagram(sb *strings.Builder) {
	// Focus on the largest or most important system
	if len(g.systems) == 0 {
		return
	}

	mainSystem := g.getMainSystem()

	sb.WriteString(fmt.Sprintf("Container_Boundary(%s_boundary, \"%s Application\") {\n",
		g.sanitizeIdentifier(mainSystem.Name), mainSystem.Name))

	// Add domains as components
	for _, containerName := range mainSystem.Containers {
		container := g.containers[containerName]
		for _, domain := range container.Domains {
			sb.WriteString(fmt.Sprintf("    Component(%s, \"%s\", \"Domain Component\", \"Handles %s logic\")\n",
				g.sanitizeIdentifier(domain), domain, domain))
		}
	}

	sb.WriteString("}\n\n")

	// Add component relationships
	g.buildComponentRelationships(sb)
}

// buildContainerRelationships builds relationships between containers
func (g *C4DiagramGenerator) buildContainerRelationships(sb *strings.Builder) {
	for _, relation := range g.relations {
		fromContainer := g.findContainerForDomain(relation.From)
		toContainer := g.findContainerForDomain(relation.To)

		if fromContainer != "" && toContainer != "" && fromContainer != toContainer {
			sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\", \"%s\")\n",
				g.sanitizeIdentifier(fromContainer),
				g.sanitizeIdentifier(toContainer),
				relation.Description,
				relation.Technology))
		}
	}
}

// buildComponentRelationships builds relationships between components
func (g *C4DiagramGenerator) buildComponentRelationships(sb *strings.Builder) {
	for _, relation := range g.relations {
		if relation.From != "" && relation.To != "" && relation.From != relation.To {
			sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
				g.sanitizeIdentifier(relation.From),
				g.sanitizeIdentifier(relation.To),
				relation.Description))
		}
	}
}

// Helper methods
func (g *C4DiagramGenerator) getSortedActors() []string {
	actors := make([]string, 0, len(g.actors))
	for actor := range g.actors {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	return actors
}

func (g *C4DiagramGenerator) getSortedSystems() []string {
	systems := make([]string, 0, len(g.systems))
	for system := range g.systems {
		systems = append(systems, system)
	}
	sort.Strings(systems)
	return systems
}

func (g *C4DiagramGenerator) getMainSystem() *C4System {
	// Return the system with most containers or first one
	var mainSystem *C4System
	maxContainers := 0

	for _, system := range g.systems {
		if len(system.Containers) > maxContainers {
			maxContainers = len(system.Containers)
			mainSystem = system
		}
	}

	// If no system found, return first one
	if mainSystem == nil {
		for _, system := range g.systems {
			return system
		}
	}

	return mainSystem
}

func (g *C4DiagramGenerator) findSystemForDomain(domain string) string {
	for systemName, system := range g.systems {
		for _, containerName := range system.Containers {
			if container := g.containers[containerName]; container != nil {
				for _, d := range container.Domains {
					if d == domain {
						return systemName
					}
				}
			}
		}
	}
	return ""
}

func (g *C4DiagramGenerator) findContainerForDomain(domain string) string {
	for containerName, container := range g.containers {
		for _, d := range container.Domains {
			if d == domain {
				return containerName
			}
		}
	}
	return ""
}

func (g *C4DiagramGenerator) sanitizeIdentifier(name string) string {
	// Replace spaces and special characters with underscores
	result := strings.ReplaceAll(name, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return result
}

// Main function to generate C4 diagrams
func GenerateC4ContextDiagram(model *parser.DSLModel) string {
	generator := NewC4DiagramGenerator()
	return generator.GenerateC4Diagram(model, C4Context)
}

func GenerateC4ContainerDiagram(model *parser.DSLModel) string {
	generator := NewC4DiagramGenerator()
	return generator.GenerateC4Diagram(model, C4Containers)
}

func GenerateC4ComponentDiagram(model *parser.DSLModel) string {
	generator := NewC4DiagramGenerator()
	return generator.GenerateC4Diagram(model, C4Components)
}
