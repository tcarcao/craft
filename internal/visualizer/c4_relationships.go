package visualizer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

// Relationship creation methods for generator

// createUserToPresentationRelations creates relationships from users to presentation
func (g *C4DiagramGenerator) createUserToPresentationRelations() {
	if g.presentationSystem == nil {
		return
	}

	for actor := range g.actors {
		for _, containerName := range g.presentationSystem.Containers {
			relation := C4Relation{
				From:        actor,
				To:          containerName,
				Description: "Uses",
				Technology:  "HTTPS",
				Type:        "uses",
			}
			g.systemRelations = append(g.systemRelations, relation)
		}
	}
}

// createPresentationToGatewayRelations creates relationships from presentation to gateway
func (g *C4DiagramGenerator) createPresentationToGatewayRelations() {
	if g.presentationSystem == nil || g.gatewaySystem == nil {
		return
	}

	for _, presContainer := range g.presentationSystem.Containers {
		for _, gwContainer := range g.gatewaySystem.Containers {
			relation := C4Relation{
				From:        presContainer,
				To:          gwContainer,
				Description: "API Requests",
				Technology:  "HTTPS/REST",
				Type:        "uses",
			}
			g.relations = append(g.relations, relation)
		}
	}
}

func (g *C4DiagramGenerator) createGatewayToServiceRelations() {
	if g.gatewaySystem == nil {
		return
	}

	// Analyze what each service actually handles from use cases
	serviceCapabilities := g.analyzeServiceCapabilities()

	for _, gwContainer := range g.gatewaySystem.Containers {
		for _, serviceName := range g.getServicesWithUserInteractions() {
			capability := serviceCapabilities[serviceName]
			serviceContainers := g.getServiceContainers(serviceName)

			for _, serviceContainer := range serviceContainers {
				if !g.isDatabaseContainer(g.containers[serviceContainer]) {
					relation := C4Relation{
						From:        gwContainer,
						To:          serviceContainer,
						Description: fmt.Sprintf("Routes %s requests", capability),
						Technology:  "HTTP/gRPC",
						Type:        "uses",
					}
					g.relations = append(g.relations, relation)
				}
			}
		}
	}
}

func (g *C4DiagramGenerator) analyzeServiceCapabilities() map[string]string {
	capabilities := make(map[string]string)

	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Domain != "" {
					service := g.findServiceForDomain(action.Domain)
					if service != "" {
						// TODO: Extract capability from action (authentication, profile, notification, etc.)
						capability := "business logic"
						if capabilities[service] == "" {
							capabilities[service] = capability
						} else if !strings.Contains(capabilities[service], capability) {
							capabilities[service] += ", " + capability
						}
					}
				}
			}
		}
	}

	return capabilities
}

// getServicesWithUserInteractions returns list of services that have user interactions
func (g *C4DiagramGenerator) getServicesWithUserInteractions() []string {
	services := make(map[string]bool)
	for _, serviceList := range g.userInteractionMap {
		for _, service := range serviceList {
			services[service] = true
		}
	}

	result := make([]string, 0, len(services))
	for service := range services {
		result = append(result, service)
	}
	sort.Strings(result)
	return result
}

// createDirectUserToServiceRelations creates direct relationships from users to services
func (g *C4DiagramGenerator) createDirectUserToServiceRelations() {
	for _, services := range g.userInteractionMap {
		for _, serviceName := range services {
			serviceContainers := g.getServiceContainers(serviceName)

			for actor := range g.actors {
				for _, serviceContainer := range serviceContainers {
					// Skip database containers
					if !g.isDatabaseContainer(g.containers[serviceContainer]) {
						relation := C4Relation{
							From:        actor,
							To:          serviceContainer,
							Description: "Interacts directly",
							Technology:  "Direct API",
							Type:        "uses",
						}
						g.systemRelations = append(g.systemRelations, relation)
					}
				}
			}
		}
	}
}

// createServiceRelationships creates relationships between services
func (g *C4DiagramGenerator) createServiceRelationships() {
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Type == parser.ActionTypeSync {
					g.handleSyncAction(action)
				}
			}
		}
	}
}

// handleSyncAction processes synchronous actions between domains/services
func (g *C4DiagramGenerator) handleSyncAction(action parser.Action) {
	if action.Domain == "" || action.TargetDomain == "" {
		return
	}

	// Handle "asks Database" pattern
	if action.TargetDomain == "Database" {
		g.createDatabaseRelationship(action.Domain, action.Phrase)
		return
	}

	fromService := g.findServiceForDomain(action.Domain)
	toService := g.findServiceForDomain(action.TargetDomain)

	// Only create relationships for domains that belong to services
	if fromService != "" && toService != "" && fromService != toService {
		fromContainer := g.findDomainContainer(action.Domain)
		toContainer := g.findDomainContainer(action.TargetDomain)

		if fromContainer != "" && toContainer != "" {
			relation := C4Relation{
				From:        fromContainer,
				To:          toContainer,
				Description: action.Phrase,
				Technology:  "Service API",
				Type:        "uses",
			}
			g.relations = append(g.relations, relation)
		}
	}
}

// createDatabaseRelationships creates relationships from services to databases
func (g *C4DiagramGenerator) createDatabaseRelationships() {
	for _, service := range g.model.Services {
		serviceContainers := g.getServiceContainers(service.Name)
		dbContainers := g.getDatabaseContainers(service.Name)

		for _, serviceContainer := range serviceContainers {
			if !g.isDatabaseContainer(g.containers[serviceContainer]) {
				for _, dbContainer := range dbContainers {
					relation := C4Relation{
						From:        serviceContainer,
						To:          dbContainer,
						Description: "Reads/Writes data",
						Technology:  "Database Protocol",
						Type:        "uses",
					}
					g.relations = append(g.relations, relation)
				}
			}
		}
	}
}

// createDatabaseRelationship creates a specific database relationship
func (g *C4DiagramGenerator) createDatabaseRelationship(domain, phrase string) {
	fromContainer := g.findDomainContainer(domain)
	if fromContainer == "" {
		return
	}

	service := g.findServiceForDomain(domain)
	if service == "" {
		return
	}

	dbContainers := g.getDatabaseContainers(service)
	for _, dbContainer := range dbContainers {
		relation := C4Relation{
			From:        fromContainer,
			To:          dbContainer,
			Description: phrase,
			Technology:  "Database Query",
			Type:        "uses",
		}
		g.relations = append(g.relations, relation)
	}
}

// createEventRelationships creates relationships to event queue
func (g *C4DiagramGenerator) createEventRelationships() {
	eventQueue := "Event_Queue"
	if g.containers[eventQueue] == nil {
		return
	}

	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Type == parser.ActionTypeAsync && action.Domain != "" {
					fromContainer := g.findDomainContainer(action.Domain)
					if fromContainer != "" {
						relation := C4Relation{
							From:        fromContainer,
							To:          eventQueue,
							Description: action.Event,
							Technology:  "Event Publishing",
							Type:        "triggers",
						}
						g.relations = append(g.relations, relation)
					}
				}
			}
		}
	}
}

// Helper methods for finding containers

// getServiceContainers returns all containers for a service
func (g *C4DiagramGenerator) getServiceContainers(serviceName string) []string {
	system := g.systems[serviceName]
	if system == nil {
		return []string{}
	}
	return system.Containers
}

// getDatabaseContainers returns database containers for a service
func (g *C4DiagramGenerator) getDatabaseContainers(serviceName string) []string {
	containers := make([]string, 0)
	for _, containerName := range g.getServiceContainers(serviceName) {
		if container := g.containers[containerName]; container != nil {
			if g.isDatabaseContainer(container) {
				containers = append(containers, containerName)
			}
		}
	}
	return containers
}

func (g *C4DiagramGenerator) findDomainContainer(domain string) string {
	service := g.findServiceForDomain(domain)
	if service == "" {
		return ""
	}

	if g.mode == C4ModeBoundaries {
		// Look for domain-specific container
		containerName := domain
		if g.containers[containerName] != nil {
			return containerName
		}
	}

	containerName := fmt.Sprintf("%s Application", service)
	if g.containers[containerName] != nil {
		return containerName
	}

	return ""
}

// isDatabaseContainer checks if container is a database
func (g *C4DiagramGenerator) isDatabaseContainer(container *C4Container) bool {
	return len(container.DataStores) > 0
}

// Component description and naming methods

// generatePresentationContainerName creates name for presentation containers
func (g *C4DiagramGenerator) generatePresentationContainerName(component parser.Component, _ int) string {
	if component.Type == parser.ComponentTypeFlow && len(component.Chain) > 0 {
		return component.Chain[len(component.Chain)-1].Name
	}
	return component.Name
}

// generateGatewayContainerName creates name for gateway containers
func (g *C4DiagramGenerator) generateGatewayContainerName(component parser.Component, _ int) string {
	if component.Type == parser.ComponentTypeFlow && len(component.Chain) > 0 {
		return component.Chain[len(component.Chain)-1].Name
	}
	return component.Name
}

// buildComponentDescription creates description with modifiers
func (g *C4DiagramGenerator) buildComponentDescription(component parser.Component, layerType string) string {
	// description := fmt.Sprintf("%s component: %s", layerType, component.Name)
	description := ""

	if component.Type == parser.ComponentTypeFlow && len(component.Chain) > 0 {
		chainNames := make([]string, 0, len(component.Chain))
		allModifiers := make([]string, 0)

		for _, chainComponent := range component.Chain {
			chainNames = append(chainNames, chainComponent.Name)

			for _, modifier := range chainComponent.Modifiers {
				modifierStr := modifier.Key
				if modifier.Value != "" {
					modifierStr = fmt.Sprintf("%s:%s", modifier.Key, modifier.Value)
				}
				allModifiers = append(allModifiers, fmt.Sprintf("%s[%s]", chainComponent.Name, modifierStr))
			}
		}

		description = fmt.Sprintf("%s composed by: %s", layerType, strings.Join(chainNames, " → "))
		if len(allModifiers) > 0 {
			description += fmt.Sprintf(" | Modifiers: %s", strings.Join(allModifiers, ", "))
		}
	} else if len(component.Modifiers) > 0 {
		modifierStrings := make([]string, 0, len(component.Modifiers))
		for _, modifier := range component.Modifiers {
			if modifier.Value != "" {
				modifierStrings = append(modifierStrings, fmt.Sprintf("%s:%s", modifier.Key, modifier.Value))
			} else {
				modifierStrings = append(modifierStrings, modifier.Key)
			}
		}
		description += fmt.Sprintf(" [%s]", strings.Join(modifierStrings, ", "))
	}

	return description
}

// Technology inference methods

// inferPresentationTechnology determines technology for presentation components
func (g *C4DiagramGenerator) inferPresentationTechnology(component parser.Component) string {
	componentName := strings.ToLower(component.Name)

	// Check modifiers first
	for _, modifier := range component.Modifiers {
		if modifier.Key == "framework" || modifier.Key == "tech" {
			return fmt.Sprintf("%s Frontend", strings.Title(modifier.Value))
		}
	}

	// Infer from name
	if strings.Contains(componentName, "react") {
		return "React Frontend"
	}
	if strings.Contains(componentName, "vue") {
		return "Vue.js Frontend"
	}
	if strings.Contains(componentName, "angular") {
		return "Angular Frontend"
	}
	if strings.Contains(componentName, "mobile") {
		return "Mobile App"
	}

	return "Frontend Application"
}

// inferGatewayTechnology determines technology for gateway components
func (g *C4DiagramGenerator) inferGatewayTechnology(component parser.Component) string {
	componentName := strings.ToLower(component.Name)

	// Check modifiers first
	for _, modifier := range component.Modifiers {
		if modifier.Key == "type" || modifier.Key == "tech" {
			return fmt.Sprintf("%s Gateway", strings.Title(modifier.Value))
		}
	}

	// Infer from name
	if strings.Contains(componentName, "nginx") {
		return "Nginx Gateway"
	}
	if strings.Contains(componentName, "loadbalancer") {
		return "Load Balancer"
	}
	if strings.Contains(componentName, "api") {
		return "API Gateway"
	}

	return "Gateway"
}

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

// inferDatabaseType determines database technology from datastore name
func (g *C4DiagramGenerator) inferDatabaseType(dataStore string) string {
	lowerStore := strings.ToLower(dataStore)

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

	return "Database"
}

// PlantUML generation methods

// buildC4PlantUML generates the PlantUML for architecture
func (g *C4DiagramGenerator) buildC4PlantUML(diagramType C4DiagramType) string {
	var sb strings.Builder

	sb.WriteString("@startuml\n")

	switch diagramType {
	case C4Context:
		sb.WriteString("!include <C4/C4_Context.puml>\n")
		g.addIconIncludes(&sb)
		sb.WriteString("\n\nLAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString("title System Context Diagram - Architecture\n\n")
		g.buildContextDiagram(&sb)
	case C4Containers:
		sb.WriteString("!include <C4/C4_Container.puml>\n")
		g.addIconIncludes(&sb)
		sb.WriteString("\n\nLAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString(fmt.Sprintf("title Container Diagram - Architecture (%s mode)\n\n", g.mode))
		g.buildContainerDiagram(&sb)
	case C4Components:
		sb.WriteString("!include <C4/C4_Component.puml>\n\n")
		sb.WriteString("LAYOUT_WITH_LEGEND()\n\n")
		sb.WriteString("title Component Diagram - Architecture\n\n")
		g.buildComponentDiagram(&sb)
	}

	sb.WriteString("\n@enduml")
	return sb.String()
}

// addIconIncludes adds necessary icon includes
func (g *C4DiagramGenerator) addIconIncludes(sb *strings.Builder) {
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
	sb.WriteString("!include <tupadr3/devicons2/rust>\n")
	sb.WriteString("!include <tupadr3/devicons2/dot_net>\n")
	sb.WriteString("!include <tupadr3/devicons2/php>\n")
	sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
	sb.WriteString("!include <tupadr3/devicons2/kotlin>\n")
	sb.WriteString("!include <tupadr3/devicons2/ruby>\n")
	sb.WriteString("!include <tupadr3/font-awesome-5/code>\n")
	sb.WriteString("!include <tupadr3/font-awesome-5/mobile>\n")
	sb.WriteString("!include <tupadr3/font-awesome-5/globe>\n")
	sb.WriteString("!include <tupadr3/font-awesome-5/shield_alt>\n")
	sb.WriteString("!include <tupadr3/font-awesome-5/list>\n")
}

// buildContextDiagram builds system context diagram
func (g *C4DiagramGenerator) buildContextDiagram(sb *strings.Builder) {
	// Add actors
	actors := g.getSortedActors()
	for _, actor := range actors {
		sb.WriteString(fmt.Sprintf("Person(%s, \"%s\", \"External user\")\n",
			g.sanitizeIdentifier(actor), actor))
	}
	if len(actors) > 0 {
		sb.WriteString("\n")
	}

	// Add external systems
	externalSystems := g.getExternalSystems()
	for _, systemName := range externalSystems {
		system := g.systems[systemName]
		sb.WriteString(fmt.Sprintf("System_Ext(%s, \"%s\", \"%s\")\n",
			g.sanitizeIdentifier(systemName), systemName, system.Description))
	}
	if len(externalSystems) > 0 {
		sb.WriteString("\n")
	}

	// Add internal systems (presentation, gateway, services)
	internalSystems := g.getInternalSystems()
	for _, systemName := range internalSystems {
		system := g.systems[systemName]
		icon := g.getSystemIcon(systemName)
		sb.WriteString(fmt.Sprintf("System(%s, \"%s\", \"%s\"%s)\n",
			g.sanitizeIdentifier(systemName), systemName, system.Description, icon))
	}
	sb.WriteString("\n")

	// Add system-level relationships
	for _, relation := range g.systemRelations {
		sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
			g.sanitizeIdentifier(relation.From),
			g.sanitizeIdentifier(relation.To),
			relation.Description))
	}
}

// buildContainerDiagram builds container diagram with proper system separation
func (g *C4DiagramGenerator) buildContainerDiagram(sb *strings.Builder) {
	// Add actors
	actors := g.getSortedActors()
	for _, actor := range actors {
		sb.WriteString(fmt.Sprintf("Person(%s, \"%s\", \"External user\")\n",
			g.sanitizeIdentifier(actor), actor))
	}
	if len(actors) > 0 {
		sb.WriteString("\n")
	}

	// Add external systems with their containers
	externalSystems := g.getExternalSystems()
	for _, systemName := range externalSystems {
		g.buildSystemBoundary(sb, systemName, true)
	}

	// Add internal systems with their containers
	internalSystems := g.getInternalSystems()
	for _, systemName := range internalSystems {
		g.buildSystemBoundary(sb, systemName, false)
	}

	// Add all relationships
	g.addAllRelationships(sb)
}

// buildSystemBoundary builds a system boundary with its containers
func (g *C4DiagramGenerator) buildSystemBoundary(sb *strings.Builder, systemName string, isExternal bool) {
	system := g.systems[systemName]
	if system == nil || len(system.Containers) == 0 {
		return
	}

	sb.WriteString(fmt.Sprintf("System_Boundary(%s_boundary, \"%s\") {\n",
		g.sanitizeIdentifier(systemName), systemName))

	if g.mode == C4ModeBoundaries && g.isServiceSystem(systemName) {
		// For service systems in boundaries mode, group domains
		g.buildDomainBoundaries(sb, systemName)
	} else {
		// Standard container listing
		g.buildStandardContainers(sb, systemName)
	}

	sb.WriteString("}\n\n")
}

// buildDomainBoundaries creates Container_Boundary for each domain in boundaries mode
func (g *C4DiagramGenerator) buildDomainBoundaries(sb *strings.Builder, serviceName string) {
	service := g.findService(serviceName)
	if service == nil {
		return
	}

	// Group containers by domain
	domainContainers := make(map[string][]string)
	dbContainers := make([]string, 0)

	for _, containerName := range g.systems[serviceName].Containers {
		container := g.containers[containerName]
		if container == nil {
			continue
		}

		if g.isDatabaseContainer(container) {
			dbContainers = append(dbContainers, containerName)
		} else if len(container.Domains) > 0 {
			domain := container.Domains[0] // Each domain container has one domain
			if domainContainers[domain] == nil {
				domainContainers[domain] = make([]string, 0)
			}
			domainContainers[domain] = append(domainContainers[domain], containerName)
		}
	}

	// Create boundaries for each domain
	for domain, containers := range domainContainers {
		sb.WriteString(fmt.Sprintf("    Container_Boundary(%s_%s_boundary, \"%s Domain\") {\n",
			g.sanitizeIdentifier(serviceName), g.sanitizeIdentifier(domain), domain))

		for _, containerName := range containers {
			container := g.containers[containerName]
			icon := g.getContainerIcon(containerName, serviceName)
			sb.WriteString(fmt.Sprintf("        Container(%s, \"%s\", \"%s\", \"%s\"%s)\n",
				g.sanitizeIdentifier(containerName), containerName,
				container.Technology, container.Description, icon))
		}

		sb.WriteString("    }\n")
	}

	// Add database containers outside domain boundaries
	if len(dbContainers) > 0 {
		sb.WriteString("    ' Data Layer\n")
		for _, containerName := range dbContainers {
			container := g.containers[containerName]
			icon := g.getDatabaseIcon(container.Technology)
			sb.WriteString(fmt.Sprintf("    ContainerDb(%s, \"%s\", \"%s\", \"%s\"%s)\n",
				g.sanitizeIdentifier(containerName), containerName,
				container.Technology, container.Description, icon))
		}
	}
}

// buildStandardContainers builds containers without domain boundaries
func (g *C4DiagramGenerator) buildStandardContainers(sb *strings.Builder, systemName string) {
	system := g.systems[systemName]

	for _, containerName := range system.Containers {
		container := g.containers[containerName]
		if container == nil {
			continue
		}

		if g.isDatabaseContainer(container) {
			icon := g.getDatabaseIcon(container.Technology)
			sb.WriteString(fmt.Sprintf("    ContainerDb(%s, \"%s\", \"%s\", \"%s\"%s)\n",
				g.sanitizeIdentifier(containerName), containerName,
				container.Technology, container.Description, icon))
		} else if containerName == "Event_Queue" {
			sb.WriteString(fmt.Sprintf("    ContainerQueue(%s, \"%s\", \"%s\", \"%s\", $sprite=\"list\")\n",
				g.sanitizeIdentifier(containerName), containerName,
				container.Technology, container.Description))
		} else {
			icon := g.getContainerIcon(containerName, systemName)
			sb.WriteString(fmt.Sprintf("    Container(%s, \"%s\", \"%s\", \"%s\"%s)\n",
				g.sanitizeIdentifier(containerName), containerName,
				container.Technology, container.Description, icon))
		}
	}
}

// buildComponentDiagram builds component diagram focusing on main service
func (g *C4DiagramGenerator) buildComponentDiagram(sb *strings.Builder) {
	mainService := g.getMainService()
	if mainService == "" {
		sb.WriteString("' No main service found\n")
		return
	}

	service := g.findService(mainService)
	if service == nil {
		return
	}

	sb.WriteString(fmt.Sprintf("Container_Boundary(%s_boundary, \"%s Service\") {\n",
		g.sanitizeIdentifier(mainService), mainService))

	for _, domain := range service.Domains {
		sb.WriteString(fmt.Sprintf("    Component(%s, \"%s\", \"Domain Component\", \"Handles %s business logic\")\n",
			g.sanitizeIdentifier(domain), domain, domain))
	}

	sb.WriteString("}\n\n")

	// Add component relationships (domain to domain within service)
	g.addComponentRelationships(sb, service.Domains)
}

// addAllRelationships adds all container relationships
func (g *C4DiagramGenerator) addAllRelationships(sb *strings.Builder) {
	// System-level relationships
	for _, relation := range g.systemRelations {
		sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
			g.sanitizeIdentifier(relation.From),
			g.sanitizeIdentifier(relation.To),
			relation.Description))
	}

	// Container-level relationships
	for _, relation := range g.relations {
		if relation.From != "" && relation.To != "" {
			sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\", \"%s\")\n",
				g.sanitizeIdentifier(relation.From),
				g.sanitizeIdentifier(relation.To),
				relation.Description,
				relation.Technology))
		}
	}
}

// addComponentRelationships adds relationships between components
func (g *C4DiagramGenerator) addComponentRelationships(sb *strings.Builder, domains []string) {
	// Create relationships from use case actions
	for _, useCase := range g.model.UseCases {
		for _, scenario := range useCase.Scenarios {
			for _, action := range scenario.Actions {
				if action.Type == parser.ActionTypeSync &&
					g.containsString(domains, action.Domain) &&
					g.containsString(domains, action.TargetDomain) {
					sb.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\")\n",
						g.sanitizeIdentifier(action.Domain),
						g.sanitizeIdentifier(action.TargetDomain),
						action.Phrase))
				}
			}
		}
	}
}

// Helper utility methods

// getSortedActors returns sorted list of actors
func (g *C4DiagramGenerator) getSortedActors() []string {
	actors := make([]string, 0, len(g.actors))
	for actor := range g.actors {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	return actors
}

// getExternalSystems returns sorted list of external systems
func (g *C4DiagramGenerator) getExternalSystems() []string {
	systems := make([]string, 0)
	for systemName, system := range g.systems {
		if system.IsExternal {
			systems = append(systems, systemName)
		}
	}
	sort.Strings(systems)
	return systems
}

// getInternalSystems returns sorted list of internal systems
func (g *C4DiagramGenerator) getInternalSystems() []string {
	systems := make([]string, 0)
	for systemName, system := range g.systems {
		if !system.IsExternal {
			systems = append(systems, systemName)
		}
	}
	sort.Strings(systems)
	return systems
}

// isServiceSystem checks if a system represents a business service
func (g *C4DiagramGenerator) isServiceSystem(systemName string) bool {
	return systemName != "Presentation" && systemName != "Gateway" && systemName != "Event_System"
}

// findService finds a service by name
func (g *C4DiagramGenerator) findService(serviceName string) *parser.Service {
	for _, service := range g.model.Services {
		if service.Name == serviceName {
			return &service
		}
	}
	return nil
}

// getMainService returns the service with most user interactions
func (g *C4DiagramGenerator) getMainService() string {
	serviceCount := make(map[string]int)

	for _, services := range g.userInteractionMap {
		for _, service := range services {
			serviceCount[service]++
		}
	}

	maxCount := 0
	mainService := ""
	for service, count := range serviceCount {
		if count > maxCount {
			maxCount = count
			mainService = service
		}
	}

	return mainService
}

// Icon methods

// getSystemIcon returns icon for system based on type
func (g *C4DiagramGenerator) getSystemIcon(systemName string) string {
	if systemName == "Presentation" {
		return ", $sprite=\"globe\""
	}
	if systemName == "Gateway" {
		return ", $sprite=\"shield_alt\""
	}

	// For service systems, use language icon
	service := g.findService(systemName)
	if service != nil {
		return g.getServiceIcon(service.Language)
	}

	return ""
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
		return ", $sprite=\"rust\""
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

// getContainerIcon returns icon for container
func (g *C4DiagramGenerator) getContainerIcon(containerName, systemName string) string {
	if strings.Contains(containerName, "Presentation") {
		return ", $sprite=\"globe\""
	}
	if strings.Contains(containerName, "Gateway") {
		return ", $sprite=\"shield_alt\""
	}

	// For service containers, use language icon
	service := g.findService(systemName)
	if service != nil {
		return g.getServiceIcon(service.Language)
	}

	return ", $sprite=\"code\""
}

// getDatabaseIcon returns icon for database
func (g *C4DiagramGenerator) getDatabaseIcon(technology string) string {
	lowerTech := strings.ToLower(technology)

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

	return ", $sprite=\"database\""
}

// sanitizeIdentifier replaces special characters for PlantUML
func (g *C4DiagramGenerator) sanitizeIdentifier(name string) string {
	result := strings.ReplaceAll(name, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	return result
}
