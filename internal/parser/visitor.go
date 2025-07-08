package parser

import (
	"fmt"
	"strings"

	"github.com/tcarcao/archdsl/pkg/parser"
)

// DSLModelBuilder builds the structured model from the parse tree
type DSLModelBuilder struct {
	*parser.BaseArchDSLVisitor
	model           *DSLModel
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
		case *parser.Use_caseContext:
			b.VisitUse_case(c)
		}
	}
	return nil
}

// Visit use case
func (b *DSLModelBuilder) VisitUse_case(ctx *parser.Use_caseContext) interface{} {
	useCase := UseCase{
		Scenarios: make([]Scenario, 0),
	}

	// Extract use case name from STRING token
	if ctx.String_() != nil {
		name := ctx.String_().GetText()
		useCase.Name = strings.Trim(name, "\"")
	}

	b.currentUC = &useCase

	// Visit scenarios (placeholder - will implement fully next)
	for i := 0; i < ctx.GetChildCount(); i++ {
		if scenario, ok := ctx.GetChild(i).(*parser.ScenarioContext); ok {
			b.VisitScenario(scenario)
		}
	}

	b.model.UseCases = append(b.model.UseCases, useCase)
	b.currentUC = nil
	return nil
}

// Visit scenario (basic implementation)
func (b *DSLModelBuilder) VisitScenario(ctx *parser.ScenarioContext) interface{} {
	scenario := Scenario{
		ID:      b.generateID("scenario"),
		Actions: make([]Action, 0),
	}

	// Placeholder implementation - will complete with trigger and action parsing
	if b.currentUC != nil {
		b.currentUC.Scenarios = append(b.currentUC.Scenarios, scenario)
	}
	return nil
}
