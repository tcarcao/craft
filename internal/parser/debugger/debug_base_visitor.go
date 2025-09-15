package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// Enhanced debug visitor to see what's being parsed
type DebugVisitor struct {
	*parser.BaseCraftVisitor
	depth int
	lexer *parser.CraftLexer // Store lexer reference for token name lookup
}

func NewDebugVisitor() *DebugVisitor {
	return &DebugVisitor{
		BaseCraftVisitor: &parser.BaseCraftVisitor{},
		depth:            0,
	}
}

func (d *DebugVisitor) setLexer(lexer *parser.CraftLexer) {
	d.lexer = lexer
}

func (d *DebugVisitor) getTokenName(tokenType int) string {
	if d.lexer == nil {
		return fmt.Sprintf("TYPE_%d", tokenType)
	}

	symbolicNames := d.lexer.GetSymbolicNames()
	if tokenType >= 0 && tokenType < len(symbolicNames) && symbolicNames[tokenType] != "" {
		return symbolicNames[tokenType]
	}

	literalNames := d.lexer.GetLiteralNames()
	if tokenType >= 0 && tokenType < len(literalNames) && literalNames[tokenType] != "" {
		return literalNames[tokenType]
	}

	return fmt.Sprintf("TYPE_%d", tokenType)
}

func (d *DebugVisitor) indent() string {
	result := ""
	for i := 0; i < d.depth; i++ {
		result += "  "
	}
	return result
}

func (d *DebugVisitor) VisitDsl(ctx *parser.DslContext) interface{} {
	fmt.Printf("%sDsl: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.ArchContext:
			d.VisitArch(c)
		case *parser.Services_defContext:
			d.VisitServices_def(c)
		case *parser.ExposureContext:
			d.VisitExposure(c)
		case *parser.Use_caseContext:
			d.VisitUse_case(c)
		default:
			if terminalNode, ok := child.(antlr.TerminalNode); ok {
				tokenType := terminalNode.GetSymbol().GetTokenType()
				tokenName := d.getTokenName(tokenType)
				fmt.Printf("%s  Terminal: %s='%s' (type=%d)\n", d.indent(),
					tokenName, terminalNode.GetText(), tokenType)
			}
		}
	}

	d.depth--
	return nil
}
