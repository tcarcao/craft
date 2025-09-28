package parser

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// =============================================================================
// Use Case Visitors
// =============================================================================

// Visit use case
func (b *DSLModelBuilder) VisitUse_case(ctx *parser.Use_caseContext) interface{} {
	useCase := UseCase{
		Scenarios: make([]Scenario, 0),
	}

	b.currentUC = &useCase

	// Try to extract use case name using the string context method
	// The grammar rule: use_case: 'use_case' string '{' NEWLINE* scenario* '}' NEWLINE*;
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if stringCtx, ok := child.(*parser.StringContext); ok {
			// Extract the STRING token from the string context
			for j := 0; j < stringCtx.GetChildCount(); j++ {
				if terminalNode, ok := stringCtx.GetChild(j).(antlr.TerminalNode); ok {
					if terminalNode.GetSymbol().GetTokenType() == parser.CraftLexerSTRING {
						name := terminalNode.GetText()
						useCase.Name = strings.Trim(name, "\"")
						break
					}
				}
			}
		}
	}

	// Process scenarios
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if scenario, ok := child.(*parser.ScenarioContext); ok {
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

	// Handle the three trigger patterns properly
	if externalTrigger := ctx.External_trigger(); externalTrigger != nil {
		// Pattern 1: 'when' external_trigger NEWLINE+
		trigger.Type = TriggerTypeExternal
		b.processExternalTrigger(externalTrigger.(*parser.External_triggerContext), &trigger)
	} else if domain := ctx.Domain(); domain != nil {
		// Pattern 3: 'when' domain 'listens' quoted_event NEWLINE+
		trigger.Type = TriggerTypeDomainListen
		trigger.Domain = domain.GetText()
		if quotedEvent := ctx.Quoted_event(); quotedEvent != nil {
			trigger.Event = strings.Trim(quotedEvent.GetText(), "\"")
		}
	} else if quotedEvent := ctx.Quoted_event(); quotedEvent != nil {
		// Pattern 2: 'when' quoted_event NEWLINE+
		trigger.Type = TriggerTypeEvent
		trigger.Event = strings.Trim(quotedEvent.GetText(), "\"")
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

// Process sync action: domain asks domain [connector_word] phrase
func (b *DSLModelBuilder) processSyncAction(ctx *parser.Sync_actionContext, action *Action) {
	action.Type = ActionTypeSync

	domains := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.DomainContext:
			domains = append(domains, c.GetText())
		case *parser.Connector_wordContext:
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
			eventText := c.GetText()
			action.Event = strings.Trim(eventText, "\"")
		}
	}
}

// Process internal action: domain verb [connector_word] phrase
func (b *DSLModelBuilder) processInternalAction(ctx *parser.Internal_actionContext, action *Action) {
	action.Type = ActionTypeInternal

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.DomainContext:
			action.Domain = c.GetText()
		case *parser.VerbContext:
			action.Verb = c.GetText()
		case *parser.Connector_wordContext:
			action.Connector = c.GetText()
		case *parser.PhraseContext:
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
		child := ctx.GetChild(i)

		switch c := child.(type) {
		case *parser.IdentifierContext:
			// Handle the new identifier rule context
			words = append(words, c.GetText())
		case *parser.StringContext:
			// Handle string context
			words = append(words, strings.Trim(c.GetText(), "\""))
		case *parser.Connector_wordContext:
			// Handle connector words
			words = append(words, c.GetText())
		case antlr.TerminalNode:
			// Handle any remaining terminal nodes (fallback)
			tokenType := c.GetSymbol().GetTokenType()
			text := c.GetText()
			switch tokenType {
			case parser.CraftLexerIDENTIFIER:
				words = append(words, text)
			case parser.CraftLexerSTRING:
				words = append(words, strings.Trim(text, "\""))
			}
		}
	}

	return words
}

// Use case visitor stubs
func (b *DSLModelBuilder) VisitExternal_trigger(ctx *parser.External_triggerContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitSync_action(ctx *parser.Sync_actionContext) interface{}   { return nil }
func (b *DSLModelBuilder) VisitAsync_action(ctx *parser.Async_actionContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitInternal_action(ctx *parser.Internal_actionContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitPhrase(ctx *parser.PhraseContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitConnector_word(ctx *parser.Connector_wordContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitActor(ctx *parser.ActorContext) interface{}               { return nil }
func (b *DSLModelBuilder) VisitVerb(ctx *parser.VerbContext) interface{}                 { return nil }
func (b *DSLModelBuilder) VisitQuoted_event(ctx *parser.Quoted_eventContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitString(ctx *parser.StringContext) interface{}             { return nil }
