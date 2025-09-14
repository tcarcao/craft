package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// =============================================================================
// Exposure Debug Visitors
// =============================================================================

func (d *DebugVisitor) VisitExposure(ctx *parser.ExposureContext) interface{} {
	fmt.Printf("%sExposure: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Exposure_nameContext:
			d.VisitExposure_name(c)
		case *parser.Exposure_propertiesContext:
			d.VisitExposure_properties(c)
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

func (d *DebugVisitor) VisitExposure_name(ctx *parser.Exposure_nameContext) interface{} {
	fmt.Printf("%sExposure_name: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			tokenName := d.getTokenName(tokenType)
			fmt.Printf("%s  Terminal: %s='%s' (type=%d)\n", d.indent(),
				tokenName, terminalNode.GetText(), tokenType)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitExposure_properties(ctx *parser.Exposure_propertiesContext) interface{} {
	fmt.Printf("%sExposure_properties: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Exposure_propertyContext:
			d.VisitExposure_property(c)
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

func (d *DebugVisitor) VisitExposure_property(ctx *parser.Exposure_propertyContext) interface{} {
	fmt.Printf("%sExposure_property: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Target_listContext:
			d.VisitTarget_list(c)
		case *parser.Domain_listContext:
			d.VisitDomain_list(c)
		case *parser.Gateway_listContext:
			d.VisitGateway_list(c)
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

func (d *DebugVisitor) VisitTarget_list(ctx *parser.Target_listContext) interface{} {
	fmt.Printf("%sTarget_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if target, ok := child.(*parser.TargetContext); ok {
			d.VisitTarget(target)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitTarget(ctx *parser.TargetContext) interface{} {
	fmt.Printf("%sTarget: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			tokenName := d.getTokenName(tokenType)
			fmt.Printf("%s  Terminal: %s='%s' (type=%d)\n", d.indent(),
				tokenName, terminalNode.GetText(), tokenType)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitGateway_list(ctx *parser.Gateway_listContext) interface{} {
	fmt.Printf("%sGateway_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if gateway, ok := child.(*parser.GatewayContext); ok {
			d.VisitGateway(gateway)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitGateway(ctx *parser.GatewayContext) interface{} {
	fmt.Printf("%sGateway: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			tokenName := d.getTokenName(tokenType)
			fmt.Printf("%s  Terminal: %s='%s' (type=%d)\n", d.indent(),
				tokenName, terminalNode.GetText(), tokenType)
		}
	}

	d.depth--
	return nil
}
