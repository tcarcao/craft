package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// DSLModelBuilder builds the structured model from the parse tree
type DSLModelBuilder struct {
	*parser.BaseCraftVisitor
	model               *DSLModel
	currentArchitecture *Architecture
	currentExposure     *Exposure
	currentService      *Service
	currentUC           *UseCase
	currentScenario     *Scenario
	idCounter           int
}

func NewDSLModelBuilder() *DSLModelBuilder {
	return &DSLModelBuilder{
		BaseCraftVisitor: &parser.BaseCraftVisitor{},
		model: &DSLModel{
			Architectures: make([]Architecture, 0),
			Exposures:     make([]Exposure, 0),
			Services:      make([]Service, 0),
			UseCases:      make([]UseCase, 0),
			Domains:       make([]Domain, 0),
			Actors:        make([]Actor, 0),
		},
		idCounter: 0,
	}
}

func (b *DSLModelBuilder) GetModel() *DSLModel {
	// Apply service merging before returning the model
	b.model.Services = MergeServices(b.model.Services)
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
		case *parser.ArchContext:
			b.VisitArch(c)
		case *parser.Services_defContext:
			b.VisitServices_def(c)
		case *parser.Service_defContext:
			b.VisitService_def(c)
		case *parser.ExposureContext:
			b.VisitExposure(c)
		case *parser.Use_caseContext:
			b.VisitUse_case(c)
		case *parser.Domain_defContext:
			b.VisitDomain_def(c)
		case *parser.Domains_defContext:
			b.VisitDomains_def(c)
		case *parser.Actor_defContext:
			b.VisitActor_def(c)
		case *parser.Actors_defContext:
			b.VisitActors_def(c)
		}
	}
	return nil
}

func (b *DSLModelBuilder) extractIdentifier(ctx *antlr.BaseParserRuleContext) string {
	if ctx == nil {
		return ""
	}
	
	// Check if this is actually an IdentifierContext (from the new grammar)
	if identifierCtx, ok := interface{}(ctx).(*parser.IdentifierContext); ok {
		return identifierCtx.GetText()
	}
	
	// Otherwise, use the original logic for terminal nodes
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		
		// Check if it's an IdentifierContext child
		if identifierCtx, ok := child.(*parser.IdentifierContext); ok {
			return identifierCtx.GetText()
		}
		
		// Check for terminal IDENTIFIER nodes (original logic)
		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			if tokenType == parser.CraftLexerIDENTIFIER {
				return terminalNode.GetText()
			}
		}
	}
	
	// Fallback: if we can't find a proper identifier, return the text
	return ctx.GetText()
}

// =============================================================================
// Utility functions
// =============================================================================

// Utility function to parse DSL content into model
func ParseDSLToModel(dslContent string) (*DSLModel, error) {
	inputStream := antlr.NewInputStream(dslContent)
	lexer := parser.NewCraftLexer(inputStream)
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := parser.NewCraftParser(tokenStream)

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
}
