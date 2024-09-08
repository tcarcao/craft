package parser

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
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

func (p *Parser) ParseString(input string) (*Architecture, error) {
	inputStream := antlr.NewInputStream(input)

	lexer := parser.NewArchDSLLexer(inputStream)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(p.errorListener)

	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	parser := parser.NewArchDSLParser(stream)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(p.errorListener)

	tree := parser.Architecture()

	if len(p.errorListener.Errors) > 0 {
		return nil, fmt.Errorf("parse errors: %s", strings.Join(p.errorListener.Errors, "; "))
	}

	listener := newArchitectureListener()
	antlr.ParseTreeWalkerDefault.Walk(listener, tree)

	return listener.getArchitecture(), nil
}
