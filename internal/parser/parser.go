package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// SyntaxError is a structured parse error with position information.
type SyntaxError struct {
	Line    int
	Column  int
	Message string
}

type Parser struct {
	errorListener *errorListener
}

type errorListener struct {
	*antlr.DefaultErrorListener
	Errors       []string
	syntaxErrors []SyntaxError
}

func (e *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int, msg string, e2 antlr.RecognitionException) {
	e.Errors = append(e.Errors, fmt.Sprintf("line %d:%d %s", line, column, msg))
	e.syntaxErrors = append(e.syntaxErrors, SyntaxError{Line: line, Column: column, Message: msg})
}

// SyntaxErrors returns structured parse errors after a ParseString call.
func (p *Parser) SyntaxErrors() []SyntaxError {
	return p.errorListener.syntaxErrors
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
