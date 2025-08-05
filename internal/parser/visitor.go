package parser

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

// DSLModelBuilder builds the structured model from the parse tree
type DSLModelBuilder struct {
	*parser.BaseArchDSLVisitor
	model           *DSLModel
	currentService  *Service
	currentUC       *UseCase
	currentScenario *Scenario
	idCounter       int
}

func NewDSLModelBuilder() *DSLModelBuilder {
	return &DSLModelBuilder{
		BaseArchDSLVisitor: &parser.BaseArchDSLVisitor{},
		model: &DSLModel{
			Services: make([]Service, 0),
			UseCases: make([]UseCase, 0),
		},
		idCounter: 0,
	}
}

func (b *DSLModelBuilder) GetModel() *DSLModel {
	return b.model
}

func (b *DSLModelBuilder) generateID(prefix string) string {
	b.idCounter++
	return fmt.Sprintf("%s_%d", prefix, b.idCounter)
}

// Visit DSL root
func (b *DSLModelBuilder) VisitDsl(ctx *parser.DslContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.ServicesContext:
			b.VisitServices(c)
		case *parser.Use_caseContext:
			b.VisitUse_case(c)
		}
	}
	return nil
}

// Visit services section
func (b *DSLModelBuilder) VisitServices(ctx *parser.ServicesContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if serviceDefList, ok := child.(*parser.Service_definition_listContext); ok {
			b.VisitService_definition_list(serviceDefList)
		} else {
		}
	}
	return nil
}

// Visit service definition list
func (b *DSLModelBuilder) VisitService_definition_list(ctx *parser.Service_definition_listContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if serviceDef, ok := child.(*parser.Service_definitionContext); ok {
			b.VisitService_definition(serviceDef)
		}
	}
	return nil
}

// Visit service definition
func (b *DSLModelBuilder) VisitService_definition(ctx *parser.Service_definitionContext) interface{} {
	service := Service{
		Domains:    make([]string, 0),
		DataStores: make([]string, 0),
	}

	// Extract service name from service_name rule
	for i := 0; i < ctx.GetChildCount(); i++ {
		if serviceName, ok := ctx.GetChild(i).(*parser.Service_nameContext); ok {
			service.Name = b.extractServiceName(serviceName)
			break
		}
	}

	b.currentService = &service

	// Visit service properties
	for i := 0; i < ctx.GetChildCount(); i++ {
		if serviceProps, ok := ctx.GetChild(i).(*parser.Service_propertiesContext); ok {
			b.VisitService_properties(serviceProps)
		}
	}

	b.model.Services = append(b.model.Services, service)
	b.currentService = nil
	return nil
}

// Visit service name
func (b *DSLModelBuilder) VisitService_name(ctx *parser.Service_nameContext) interface{} {
	return nil // Handled by extractServiceName
}

// Extract service name from service_name context
func (b *DSLModelBuilder) extractServiceName(ctx *parser.Service_nameContext) string {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if terminalNode, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			text := terminalNode.GetText()

			switch tokenType {
			case parser.ArchDSLLexerIDENTIFIER:
				return text
			case parser.ArchDSLLexerSTRING:
				return strings.Trim(text, "\"")
			}
		}
	}
	return ""
}

// Visit service properties
func (b *DSLModelBuilder) VisitService_properties(ctx *parser.Service_propertiesContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if serviceProp, ok := ctx.GetChild(i).(*parser.Service_propertyContext); ok {
			b.VisitService_property(serviceProp)
		}
	}
	return nil
}

// Visit service property
func (b *DSLModelBuilder) VisitService_property(ctx *parser.Service_propertyContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			switch tokenType {
			case parser.ArchDSLLexerDOMAINS:
				// Next non-terminal should be domain_list
				if i+2 < ctx.GetChildCount() {
					if domainList, ok := ctx.GetChild(i + 2).(*parser.Domain_listContext); ok {
						b.VisitDomain_list(domainList)
					}
				}
			case parser.ArchDSLLexerDATA_STORES:
				// Next non-terminal should be datastore_list
				if i+2 < ctx.GetChildCount() {
					if datastoreList, ok := ctx.GetChild(i + 2).(*parser.Datastore_listContext); ok {
						b.VisitDatastore_list(datastoreList)
					}
				}
			case parser.ArchDSLLexerLANGUAGE:
				if i+2 < ctx.GetChildCount() {
					if terminalNode, ok := ctx.GetChild(i + 2).(antlr.TerminalNode); ok {
						tokenType := terminalNode.GetSymbol().GetTokenType()
						if tokenType == parser.ArchDSLLexerIDENTIFIER {
							b.currentService.Language = terminalNode.GetText()
						}
					}
				}
			}
		}
	}
	return nil
}

// Visit domain list
func (b *DSLModelBuilder) VisitDomain_list(ctx *parser.Domain_listContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if domainOrDatastore, ok := ctx.GetChild(i).(*parser.Domain_or_datastoreContext); ok {
			domainName := b.extractDomainOrDatastoreName(domainOrDatastore)
			if domainName != "" {
				b.currentService.Domains = append(b.currentService.Domains, domainName)
			}
		}
	}
	return nil
}

// Visit datastore list
func (b *DSLModelBuilder) VisitDatastore_list(ctx *parser.Datastore_listContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if domainOrDatastore, ok := ctx.GetChild(i).(*parser.Domain_or_datastoreContext); ok {
			datastoreName := b.extractDomainOrDatastoreName(domainOrDatastore)
			if datastoreName != "" {
				b.currentService.DataStores = append(b.currentService.DataStores, datastoreName)
			}
		}
	}
	return nil
}

// Visit domain_or_datastore
func (b *DSLModelBuilder) VisitDomain_or_datastore(ctx *parser.Domain_or_datastoreContext) interface{} {
	return nil // Handled by extractDomainOrDatastoreName
}

// Extract name from domain_or_datastore context
func (b *DSLModelBuilder) extractDomainOrDatastoreName(ctx *parser.Domain_or_datastoreContext) string {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if terminalNode, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			if tokenType == parser.ArchDSLLexerIDENTIFIER {
				return terminalNode.GetText()
			}
		}
	}
	return ""
}

// Visit use case
func (b *DSLModelBuilder) VisitUse_case(ctx *parser.Use_caseContext) interface{} {
	useCase := UseCase{
		Scenarios: make([]Scenario, 0),
	}

	// Extract use case name - now using STRING token directly
	if ctx.String_() != nil {
		name := ctx.String_().GetText()
		useCase.Name = strings.Trim(name, "\"")
	}

	b.currentUC = &useCase

	// Visit scenarios
	for i := 0; i < ctx.GetChildCount(); i++ {
		if scenario, ok := ctx.GetChild(i).(*parser.ScenarioContext); ok {
			b.VisitScenario(scenario)
		}
	}

	b.model.UseCases = append(b.model.UseCases, useCase)
	b.currentUC = nil
	return nil
}

// Visit scenario
func (b *DSLModelBuilder) VisitScenario(ctx *parser.ScenarioContext) interface{} {
	scenario := Scenario{
		ID:      b.generateID("scenario"),
		Actions: make([]Action, 0),
	}

	b.currentScenario = &scenario

	// Visit trigger and actions
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.TriggerContext:
			b.VisitTrigger(c)
		case *parser.Action_blockContext:
			b.VisitAction_block(c)
		}
	}

	if b.currentUC != nil {
		b.currentUC.Scenarios = append(b.currentUC.Scenarios, scenario)
	}
	b.currentScenario = nil
	return nil
}

// Visit trigger
func (b *DSLModelBuilder) VisitTrigger(ctx *parser.TriggerContext) interface{} {
	trigger := Trigger{}

	// Analyze trigger type and extract information
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.External_triggerContext:
			trigger.Type = TriggerTypeExternal
			b.processExternalTrigger(c, &trigger)
		case *parser.Quoted_eventContext:
			if b.hasDomainListens(ctx) {
				trigger.Type = TriggerTypeDomainListen
				trigger.Domain = b.extractDomainFromTrigger(ctx)
				trigger.Event = strings.Trim(c.GetText(), "\"")
			} else {
				trigger.Type = TriggerTypeEvent
				trigger.Event = strings.Trim(c.GetText(), "\"")
			}
		}
	}

	// Generate description
	trigger.Description = b.generateTriggerDescription(trigger)

	if b.currentScenario != nil {
		b.currentScenario.Trigger = trigger
	}
	return nil
}

// Process external trigger (when actor verb phrase)
func (b *DSLModelBuilder) processExternalTrigger(ctx *parser.External_triggerContext, trigger *Trigger) {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.ActorContext:
			trigger.Actor = c.GetText()
		case *parser.VerbContext:
			trigger.Verb = c.GetText()
		case *parser.PhraseContext:
			words := b.extractWordsFromPhrase(c)
			trigger.Phrase = strings.Join(words, " ")
		}
	}
}

// Check if trigger has "domain listens"
func (b *DSLModelBuilder) hasDomainListens(ctx *parser.TriggerContext) bool {
	// Check if we have both domain and "listens" keyword
	hasDomain := false
	hasListens := false

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if _, ok := child.(*parser.DomainContext); ok {
			hasDomain = true
		}
		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			if terminalNode.GetText() == "listens" {
				hasListens = true
			}
		}
	}

	return hasDomain && hasListens
}

// Extract domain from trigger context
func (b *DSLModelBuilder) extractDomainFromTrigger(ctx *parser.TriggerContext) string {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if domain, ok := ctx.GetChild(i).(*parser.DomainContext); ok {
			return domain.GetText()
		}
	}
	return ""
}

// Generate human-readable trigger description
func (b *DSLModelBuilder) generateTriggerDescription(trigger Trigger) string {
	switch trigger.Type {
	case TriggerTypeExternal:
		return fmt.Sprintf("when %s %s %s", trigger.Actor, trigger.Verb, trigger.Phrase)
	case TriggerTypeEvent:
		return fmt.Sprintf("when \"%s\"", trigger.Event)
	case TriggerTypeDomainListen:
		return fmt.Sprintf("when %s listens \"%s\"", trigger.Domain, trigger.Event)
	}
	return "unknown trigger"
}

// Visit action block
func (b *DSLModelBuilder) VisitAction_block(ctx *parser.Action_blockContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if action, ok := ctx.GetChild(i).(*parser.ActionContext); ok {
			b.VisitAction(action)
		}
	}
	return nil
}

// Visit action
func (b *DSLModelBuilder) VisitAction(ctx *parser.ActionContext) interface{} {
	action := Action{
		ID: b.generateID("action"),
	}

	// Determine action type and extract data
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.Sync_actionContext:
			b.processSyncAction(c, &action)
		case *parser.Async_actionContext:
			b.processAsyncAction(c, &action)
		case *parser.Internal_actionContext:
			b.processInternalAction(c, &action)
		}
	}

	// Generate description
	action.Description = b.generateActionDescription(action)

	if b.currentScenario != nil {
		b.currentScenario.Actions = append(b.currentScenario.Actions, action)
	}
	return nil
}

// Process sync action: domain asks domain [connector] phrase
func (b *DSLModelBuilder) processSyncAction(ctx *parser.Sync_actionContext, action *Action) {
	action.Type = ActionTypeSync

	domains := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.DomainContext:
			domains = append(domains, c.GetText())
		case *parser.ConnectorContext:
			action.Connector = c.GetText()
		case *parser.PhraseContext:
			words := b.extractWordsFromPhrase(c)
			action.Phrase = strings.Join(words, " ")
		}
	}

	// Assign domains in order: first is source, second is target
	if len(domains) >= 1 {
		action.Domain = domains[0]
	}
	if len(domains) >= 2 {
		action.TargetDomain = domains[1]
	}
}

// Process async action: domain notifies quoted_event
func (b *DSLModelBuilder) processAsyncAction(ctx *parser.Async_actionContext, action *Action) {
	action.Type = ActionTypeAsync

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.DomainContext:
			action.Domain = c.GetText()
		case *parser.Quoted_eventContext:
			// quoted_event is now just STRING, so extract without extra quotes
			eventText := c.GetText()
			action.Event = strings.Trim(eventText, "\"")
		}
	}
}

// Process internal action: domain verb [connector] phrase
func (b *DSLModelBuilder) processInternalAction(ctx *parser.Internal_actionContext, action *Action) {
	action.Type = ActionTypeInternal

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.DomainContext:
			action.Domain = c.GetText()
		case *parser.VerbContext:
			action.Verb = c.GetText()
		case *parser.ConnectorContext:
			action.Connector = c.GetText()
		case *parser.PhraseContext:
			// Extract words and rebuild with spaces
			words := b.extractWordsFromPhrase(c)
			action.Phrase = strings.Join(words, " ")
		}
	}
}

// Generate human-readable action description
func (b *DSLModelBuilder) generateActionDescription(action Action) string {
	switch action.Type {
	case ActionTypeSync:
		if action.Connector != "" {
			return fmt.Sprintf("%s asks %s %s %s", action.Domain, action.TargetDomain, action.Connector, action.Phrase)
		}
		return fmt.Sprintf("%s asks %s %s", action.Domain, action.TargetDomain, action.Phrase)
	case ActionTypeAsync:
		return fmt.Sprintf("%s notifies \"%s\"", action.Domain, action.Event)
	case ActionTypeInternal:
		if action.Connector != "" {
			return fmt.Sprintf("%s %s %s %s", action.Domain, action.Verb, action.Connector, action.Phrase)
		}
		return fmt.Sprintf("%s %s %s", action.Domain, action.Verb, action.Phrase)
	}
	return "unknown action"
}

// Extract words from phrase context
func (b *DSLModelBuilder) extractWordsFromPhrase(ctx *parser.PhraseContext) []string {
	words := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if wordCtx, ok := ctx.GetChild(i).(*parser.WordContext); ok {
			words = append(words, wordCtx.GetText())
		}
	}

	return words
}

// Implement remaining visitor methods as no-ops (required by interface)
func (b *DSLModelBuilder) VisitExternal_trigger(ctx *parser.External_triggerContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitSync_action(ctx *parser.Sync_actionContext) interface{}   { return nil }
func (b *DSLModelBuilder) VisitAsync_action(ctx *parser.Async_actionContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitInternal_action(ctx *parser.Internal_actionContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitPhrase(ctx *parser.PhraseContext) interface{}             { return nil }
func (b *DSLModelBuilder) VisitWord(ctx *parser.WordContext) interface{}                 { return nil }
func (b *DSLModelBuilder) VisitActor(ctx *parser.ActorContext) interface{}               { return nil }
func (b *DSLModelBuilder) VisitDomain(ctx *parser.DomainContext) interface{}             { return nil }
func (b *DSLModelBuilder) VisitVerb(ctx *parser.VerbContext) interface{}                 { return nil }
func (b *DSLModelBuilder) VisitQuoted_event(ctx *parser.Quoted_eventContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitString(ctx *parser.StringContext) interface{}             { return nil }
func (b *DSLModelBuilder) VisitDatastore(ctx *parser.DatastoreContext) interface{}       { return nil }

// Utility function to parse DSL content into model
func ParseDSLToModel(dslContent string) (*DSLModel, error) {
	inputStream := antlr.NewInputStream(dslContent)
	lexer := parser.NewArchDSLLexer(inputStream)
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := parser.NewArchDSLParser(tokenStream)

	// Add error listener
	errorListener := &CustomErrorListener{
		DefaultErrorListener: &antlr.DefaultErrorListener{},
		errors:               make([]string, 0),
	}
	p.RemoveErrorListeners()
	p.AddErrorListener(errorListener)

	tree := p.Dsl()
	if tree == nil {
		return nil, fmt.Errorf("failed to parse DSL")
	}

	if len(errorListener.errors) > 0 {
		return nil, fmt.Errorf("parse errors: %v", errorListener.errors)
	}

	builder := NewDSLModelBuilder()
	builder.VisitDsl(tree.(*parser.DslContext))

	return builder.GetModel(), nil
}

type CustomErrorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

func (c *CustomErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	errorMsg := fmt.Sprintf("line %d:%d %s", line, column, msg)
	c.errors = append(c.errors, errorMsg)
	fmt.Printf("ERROR: %s\n", errorMsg)
}
