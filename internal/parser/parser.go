package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

type Parser struct {
	errorListener *errorListener
}

type errorListener struct {
	*antlr.DefaultErrorListener
	Errors []string
}

func (e *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e2 antlr.RecognitionException) {
	e.Errors = append(e.Errors, fmt.Sprintf("line %d:%d %s", line, column, msg))
}

func NewParser() *Parser {
	return &Parser{
		errorListener: &errorListener{DefaultErrorListener: antlr.NewDefaultErrorListener()},
	}
}

func (p *Parser) ParseString(dslContent string) (*DSLModel, error) {
	inputStream := antlr.NewInputStream(dslContent)
	lexer := parser.NewCraftLexer(inputStream)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(p.errorListener)

	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	dslParser := parser.NewCraftParser(tokenStream)

	dslParser.RemoveErrorListeners()
	dslParser.AddErrorListener(p.errorListener)

	tree := dslParser.Dsl()
	if tree == nil {
		return nil, fmt.Errorf("failed to parse DSL")
	}

	if len(p.errorListener.Errors) > 0 {
		return nil, fmt.Errorf("parse errors: %v", p.errorListener.Errors)
	}

	builder := NewDSLModelBuilder()
	builder.VisitDsl(tree.(*parser.DslContext))

	return builder.GetModel(), nil
}
