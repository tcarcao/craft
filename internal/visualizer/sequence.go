package visualizer

import (
	"fmt"
	"strings"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// PlantUMLSequenceGenerator generates PlantUML sequence diagrams from DSL models
type PlantUMLSequenceGenerator struct {
	model           *craft.CraftDoc
	participants    map[string]bool
	actors          map[string]bool
	eventPublishers map[string]string // event -> domain that publishes it
}

// NewPlantUMLSequenceGenerator creates a new sequence generator instance
func NewPlantUMLSequenceGenerator() *PlantUMLSequenceGenerator {
	return &PlantUMLSequenceGenerator{
		participants:    make(map[string]bool),
		actors:          make(map[string]bool),
		eventPublishers: make(map[string]string),
	}
}

// GenerateSequenceDiagram converts a DSL model to PlantUML sequence diagram
func (g *PlantUMLSequenceGenerator) GenerateSequenceDiagram(model *craft.CraftDoc) string {
	g.model = model
	g.participants = make(map[string]bool)
	g.actors = make(map[string]bool)
	g.eventPublishers = make(map[string]string)

	// First pass: collect all event publishers
	for _, useCase := range model.UseCases {
		g.collectEventPublishersForSequence(useCase)
	}

	// Build the sequence diagram
	var sb strings.Builder
	sb.WriteString("@startuml\n")
	sb.WriteString("skinparam backgroundColor white\n")
	sb.WriteString("skinparam defaultFontName SansSerif\n")
	sb.WriteString("skinparam sequenceMessageAlign center\n\n")

	// Process each use case
	for _, useCase := range model.UseCases {
		sb.WriteString(fmt.Sprintf("== %s ==\n\n", useCase.Name))
		for _, scenario := range useCase.Scenarios {
			g.processScenarioForSequence(&sb, scenario)
		}
	}

	sb.WriteString("@enduml")
	return sb.String()
}

// collectEventPublishersForSequence maps events to their publishing domains
func (g *PlantUMLSequenceGenerator) collectEventPublishersForSequence(useCase craft.UseCase) {
	for _, scenario := range useCase.Scenarios {
		for _, action := range scenario.Actions {
			if action.Type == craft.ActionTypeAsync && action.Context != "" && action.Event != "" {
				g.eventPublishers[action.Event] = action.Context
			}
		}
	}
}

// processScenarioForSequence generates sequence diagram flows for a scenario
func (g *PlantUMLSequenceGenerator) processScenarioForSequence(sb *strings.Builder, scenario craft.Scenario) {
	// Track call stack for returns
	callStack := make([]string, 0)

	// Handle trigger
	trigger := scenario.Trigger
	switch trigger.Type {
	case craft.TriggerTypeExternal:
		if trigger.Actor != "" && len(scenario.Actions) > 0 {
			g.actors[trigger.Actor] = true
			firstAction := scenario.Actions[0]
			if firstAction.Context != "" {
				g.participants[firstAction.Context] = true
				description := fmt.Sprintf("%s %s", trigger.Verb, trigger.Phrase)
				sb.WriteString(fmt.Sprintf("%s -> %s : %s\n", trigger.Actor, firstAction.Context, description))
				callStack = append(callStack, trigger.Actor)
			}
		}
	case craft.TriggerTypeDomainListen:
		if trigger.Context != "" && trigger.Event != "" {
			g.participants[trigger.Context] = true
			publishingDomain := g.eventPublishers[trigger.Event]
			if publishingDomain != "" {
				sb.WriteString(fmt.Sprintf("[-> %s : %s\n", trigger.Context, trigger.Event))
			}
		}
	}

	// Process actions
	for _, action := range scenario.Actions {
		g.processActionForSequence(sb, action, &callStack)
	}

	sb.WriteString("\n")
}

// processActionForSequence generates sequence diagram notation for an action
func (g *PlantUMLSequenceGenerator) processActionForSequence(sb *strings.Builder, action craft.Action, callStack *[]string) {
	switch action.Type {
	case craft.ActionTypeSync:
		if action.Context != "" && action.TargetContext != "" {
			g.participants[action.Context] = true
			g.participants[action.TargetContext] = true

			// Push caller to stack
			*callStack = append(*callStack, action.Context)

			description := action.Phrase
			if action.Connector != "" && description != "" {
				description = action.Connector + " " + description
			}
			sb.WriteString(fmt.Sprintf("%s -> %s : %s\n", action.Context, action.TargetContext, description))
		}
	case craft.ActionTypeAsync:
		if action.Context != "" && action.Event != "" {
			g.participants[action.Context] = true
			sb.WriteString(fmt.Sprintf("%s ->> %s : notifies \"%s\"\n", action.Context, action.Context, action.Event))
		}
	case craft.ActionTypeInternal:
		if action.Context != "" {
			g.participants[action.Context] = true
			description := action.Phrase
			if action.Verb != "" {
				description = action.Verb + " " + description
			}
			if action.Connector != "" && action.Phrase != "" {
				description = action.Connector + " " + action.Phrase
			}
			sb.WriteString(fmt.Sprintf("%s -> %s : %s\n", action.Context, action.Context, description))
		}
	case craft.ActionTypeReturn:
		if action.Context != "" {
			g.participants[action.Context] = true
			description := "returns " + action.Phrase

			var to string
			if action.TargetContext != "" {
				to = action.TargetContext
				g.participants[to] = true
			} else if len(*callStack) > 0 {
				to = (*callStack)[len(*callStack)-1]
				*callStack = (*callStack)[:len(*callStack)-1]
			}

			if to != "" {
				sb.WriteString(fmt.Sprintf("%s --> %s : %s\n", action.Context, to, description))
			}
		}
	}
}
