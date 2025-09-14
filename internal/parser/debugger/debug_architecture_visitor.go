package parser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/craft/pkg/parser"
)

// =============================================================================
// Architecture Debug Visitors
// =============================================================================

func (d *DebugVisitor) VisitArch(ctx *parser.ArchContext) interface{} {
	fmt.Printf("%sArch: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Arch_nameContext:
			d.VisitArch_name(c)
		case *parser.Arch_sectionsContext:
			d.VisitArch_sections(c)
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

func (d *DebugVisitor) VisitArch_name(ctx *parser.Arch_nameContext) interface{} {
	fmt.Printf("%sArch_name: %d children\n", d.indent(), ctx.GetChildCount())
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

func (d *DebugVisitor) VisitArch_sections(ctx *parser.Arch_sectionsContext) interface{} {
	fmt.Printf("%sArch_sections: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Presentation_sectionContext:
			d.VisitPresentation_section(c)
		case *parser.Gateway_sectionContext:
			d.VisitGateway_section(c)
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

func (d *DebugVisitor) VisitPresentation_section(ctx *parser.Presentation_sectionContext) interface{} {
	fmt.Printf("%sPresentation_section: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if archComponentList, ok := child.(*parser.Arch_component_listContext); ok {
			d.VisitArch_component_list(archComponentList)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitGateway_section(ctx *parser.Gateway_sectionContext) interface{} {
	fmt.Printf("%sGateway_section: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if archComponentList, ok := child.(*parser.Arch_component_listContext); ok {
			d.VisitArch_component_list(archComponentList)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitArch_component_list(ctx *parser.Arch_component_listContext) interface{} {
	fmt.Printf("%sArch_component_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if archComponent, ok := child.(*parser.Arch_componentContext); ok {
			d.VisitArch_component(archComponent)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitArch_component(ctx *parser.Arch_componentContext) interface{} {
	fmt.Printf("%sArch_component: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Component_flowContext:
			d.VisitComponent_flow(c)
		case *parser.Simple_componentContext:
			d.VisitSimple_component(c)
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

func (d *DebugVisitor) VisitComponent_flow(ctx *parser.Component_flowContext) interface{} {
	fmt.Printf("%sComponent_flow: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitSimple_component(ctx *parser.Simple_componentContext) interface{} {
	fmt.Printf("%sSimple_component: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if componentWithMods, ok := child.(*parser.Component_with_modifiersContext); ok {
			d.VisitComponent_with_modifiers(componentWithMods)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitComponent_with_modifiers(ctx *parser.Component_with_modifiersContext) interface{} {
	fmt.Printf("%sComponent_with_modifiers: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Component_nameContext:
			d.VisitComponent_name(c)
		case *parser.Component_modifiersContext:
			d.VisitComponent_modifiers(c)
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

func (d *DebugVisitor) VisitComponent_name(ctx *parser.Component_nameContext) interface{} {
	fmt.Printf("%sComponent_name: %d children\n", d.indent(), ctx.GetChildCount())
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

func (d *DebugVisitor) VisitComponent_modifiers(ctx *parser.Component_modifiersContext) interface{} {
	fmt.Printf("%sComponent_modifiers: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
	}

	d.depth--
	return nil
}
