package parser

import (
	"fmt"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

// =============================================================================
// Utility Functions and Test Cases
// =============================================================================

type CustomErrorListener struct {
	*antlr.DefaultErrorListener
	errors []string
}

func (c *CustomErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	errorMsg := fmt.Sprintf("line %d:%d %s", line, column, msg)
	c.errors = append(c.errors, errorMsg)
}

func debugParseDSL(t *testing.T, dsl string) {
	fmt.Printf("\n=== DEBUGGING DSL PARSING ===\n")
	fmt.Printf("Input DSL:\n%s\n\n", dsl)

	inputStream := antlr.NewInputStream(dsl)
	lexer := parser.NewArchDSLLexer(inputStream)

	// Debug tokens with better formatting
	fmt.Println("=== TOKENS ===")
	tokenStream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	tokenStream.Fill()
	tokens := tokenStream.GetAllTokens()

	for i, token := range tokens {
		var tokenName string
		symbolicNames := lexer.GetSymbolicNames()
		tokenType := token.GetTokenType()

		if tokenType >= 0 && tokenType < len(symbolicNames) && symbolicNames[tokenType] != "" {
			tokenName = symbolicNames[tokenType]
		} else {
			// Try literal names if symbolic names don't exist
			literalNames := lexer.GetLiteralNames()
			if tokenType >= 0 && tokenType < len(literalNames) && literalNames[tokenType] != "" {
				tokenName = literalNames[tokenType]
			} else {
				tokenName = fmt.Sprintf("TYPE_%d", tokenType)
			}
		}

		fmt.Printf("Token %d: %s='%s' (type=%d, channel=%d)\n",
			i, tokenName, token.GetText(), token.GetTokenType(), token.GetChannel())
	}

	// Reset for parsing
	lexer2 := parser.NewArchDSLLexer(antlr.NewInputStream(dsl))
	tokenStream2 := antlr.NewCommonTokenStream(lexer2, antlr.TokenDefaultChannel)
	p := parser.NewArchDSLParser(tokenStream2)

	// Add error listener
	errorListener := &CustomErrorListener{
		DefaultErrorListener: antlr.NewDefaultErrorListener(),
		errors:               make([]string, 0),
	}
	p.RemoveErrorListeners()
	p.AddErrorListener(errorListener)

	fmt.Println("\n=== PARSING ===")
	tree := p.Dsl()

	if len(errorListener.errors) > 0 {
		fmt.Println("PARSE ERRORS:")
		for _, err := range errorListener.errors {
			fmt.Printf("  - %s\n", err)
		}
		t.Logf("Parse failed with errors: %v", errorListener.errors)
	}

	if tree != nil {
		fmt.Println("\n=== PARSE TREE STRUCTURE ===")
		visitor := NewDebugVisitor()
		visitor.setLexer(lexer2) // Pass lexer reference for token name lookup
		visitor.VisitDsl(tree.(*parser.DslContext))
	} else {
		fmt.Println("No parse tree generated")
		t.Log("No parse tree generated")
	}
}

// Additional stub methods for the remaining visitor interfaces
func (d *DebugVisitor) VisitServices_def(ctx *parser.Services_defContext) interface{} {
	fmt.Printf("%sServices: %d children\n", d.indent(), ctx.GetChildCount())
	return nil
}

func (d *DebugVisitor) VisitUse_case(ctx *parser.Use_caseContext) interface{} {
	fmt.Printf("%sUse_case: %d children\n", d.indent(), ctx.GetChildCount())
	return nil
}

func (d *DebugVisitor) VisitDomain_list(ctx *parser.Domain_listContext) interface{} {
	fmt.Printf("%sDomain_list: %d children\n", d.indent(), ctx.GetChildCount())
	return nil
}
