package parser

import (
	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

// =============================================================================
// Architecture Visitors
// =============================================================================

// Visit architecture definition
func (b *DSLModelBuilder) VisitArch(ctx *parser.ArchContext) interface{} {
	arch := Architecture{
		Presentation: make([]Component, 0),
		Gateway:      make([]Component, 0),
	}

	b.currentArchitecture = &arch

	// Extract optional architecture name and sections
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if archName, ok := child.(*parser.Arch_nameContext); ok {
			// Extract name from arch_name context
			for j := 0; j < archName.GetChildCount(); j++ {
				if terminalNode, ok := archName.GetChild(j).(antlr.TerminalNode); ok {
					if terminalNode.GetSymbol().GetTokenType() == parser.ArchDSLLexerIDENTIFIER {
						arch.Name = terminalNode.GetText()
						break
					}
				}
			}
		} else if archSections, ok := child.(*parser.Arch_sectionsContext); ok {
			b.VisitArch_sections(archSections)
		}
	}

	b.model.Architectures = append(b.model.Architectures, arch)
	b.currentArchitecture = nil
	return nil
}

// Visit architecture sections (presentation and gateway)
func (b *DSLModelBuilder) VisitArch_sections(ctx *parser.Arch_sectionsContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.Presentation_sectionContext:
			b.VisitPresentation_section(c)
		case *parser.Gateway_sectionContext:
			b.VisitGateway_section(c)
		}
	}
	return nil
}

// Visit presentation section
func (b *DSLModelBuilder) VisitPresentation_section(ctx *parser.Presentation_sectionContext) interface{} {
	if b.currentArchitecture == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if componentList, ok := ctx.GetChild(i).(*parser.Arch_component_listContext); ok {
			components := b.extractArchComponentList(componentList)
			b.currentArchitecture.Presentation = components
		}
	}
	return nil
}

// Visit gateway section
func (b *DSLModelBuilder) VisitGateway_section(ctx *parser.Gateway_sectionContext) interface{} {
	if b.currentArchitecture == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if componentList, ok := ctx.GetChild(i).(*parser.Arch_component_listContext); ok {
			components := b.extractArchComponentList(componentList)
			b.currentArchitecture.Gateway = components
		}
	}
	return nil
}

// Extract component list from arch_component_list context
func (b *DSLModelBuilder) extractArchComponentList(ctx *parser.Arch_component_listContext) []Component {
	components := make([]Component, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if archComponent, ok := ctx.GetChild(i).(*parser.Arch_componentContext); ok {
			component := b.extractArchComponent(archComponent)
			if component != nil {
				components = append(components, *component)
			}
		}
	}

	return components
}

// Extract single component from arch_component context
func (b *DSLModelBuilder) extractArchComponent(ctx *parser.Arch_componentContext) *Component {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		switch c := child.(type) {
		case *parser.Component_flowContext:
			return b.extractComponentFlow(c)
		case *parser.Simple_componentContext:
			return b.extractSimpleComponent(c)
		}
	}
	return nil
}

// Extract component flow (A > B > C)
func (b *DSLModelBuilder) extractComponentFlow(ctx *parser.Component_flowContext) *Component {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if componentChain, ok := ctx.GetChild(i).(*parser.Component_chainContext); ok {
			return b.extractComponentChain(componentChain)
		}
	}
	return nil
}

// Extract component chain
func (b *DSLModelBuilder) extractComponentChain(ctx *parser.Component_chainContext) *Component {
	chain := make([]Component, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if componentWithMods, ok := ctx.GetChild(i).(*parser.Component_with_modifiersContext); ok {
			component := b.extractComponentWithModifiers(componentWithMods)
			if component != nil {
				chain = append(chain, *component)
			}
		}
	}

	if len(chain) == 0 {
		return nil
	}

	// Create a flow component with the chain
	return &Component{
		Type:  ComponentTypeFlow,
		Chain: chain,
	}
}

// Extract simple component
func (b *DSLModelBuilder) extractSimpleComponent(ctx *parser.Simple_componentContext) *Component {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if componentWithMods, ok := ctx.GetChild(i).(*parser.Component_with_modifiersContext); ok {
			component := b.extractComponentWithModifiers(componentWithMods)
			if component != nil {
				component.Type = ComponentTypeSimple
			}
			return component
		}
	}
	return nil
}

// Extract component with modifiers
func (b *DSLModelBuilder) extractComponentWithModifiers(ctx *parser.Component_with_modifiersContext) *Component {
	component := &Component{
		Type:      ComponentTypeSimple,
		Modifiers: make([]ComponentModifier, 0),
	}

	// Extract component name and modifiers
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if componentName, ok := child.(*parser.Component_nameContext); ok {
			// Extract IDENTIFIER from component_name context
			for j := 0; j < componentName.GetChildCount(); j++ {
				if terminalNode, ok := componentName.GetChild(j).(antlr.TerminalNode); ok {
					if terminalNode.GetSymbol().GetTokenType() == parser.ArchDSLLexerIDENTIFIER {
						component.Name = terminalNode.GetText()
						break
					}
				}
			}
		} else if componentMods, ok := child.(*parser.Component_modifiersContext); ok {
			component.Modifiers = b.extractComponentModifiers(componentMods)
		}
	}

	return component
}

// Extract component modifiers [ssl, cache:aggressive]
func (b *DSLModelBuilder) extractComponentModifiers(ctx *parser.Component_modifiersContext) []ComponentModifier {
	modifiers := make([]ComponentModifier, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if modifierList, ok := ctx.GetChild(i).(*parser.Modifier_listContext); ok {
			for j := 0; j < modifierList.GetChildCount(); j++ {
				if modifier, ok := modifierList.GetChild(j).(*parser.ModifierContext); ok {
					mod := b.extractModifier(modifier)
					if mod != nil {
						modifiers = append(modifiers, *mod)
					}
				}
			}
		}
	}

	return modifiers
}

// Extract single modifier
func (b *DSLModelBuilder) extractModifier(ctx *parser.ModifierContext) *ComponentModifier {
	modifier := &ComponentModifier{}

	// Find IDENTIFIER tokens
	identifiers := make([]string, 0)
	for i := 0; i < ctx.GetChildCount(); i++ {
		if terminalNode, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			if terminalNode.GetSymbol().GetTokenType() == parser.ArchDSLLexerIDENTIFIER {
				identifiers = append(identifiers, terminalNode.GetText())
			}
		}
	}

	if len(identifiers) >= 1 {
		modifier.Key = identifiers[0]
	}
	if len(identifiers) >= 2 {
		modifier.Value = identifiers[1]
	}

	return modifier
}

// Architecture visitor stubs
func (b *DSLModelBuilder) VisitArch_name(ctx *parser.Arch_nameContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitArch_component(ctx *parser.Arch_componentContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitComponent_flow(ctx *parser.Component_flowContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitComponent_chain(ctx *parser.Component_chainContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitComponent_with_modifiers(ctx *parser.Component_with_modifiersContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitComponent_name(ctx *parser.Component_nameContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitComponent_modifiers(ctx *parser.Component_modifiersContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitModifier_list(ctx *parser.Modifier_listContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitModifier(ctx *parser.ModifierContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitSimple_component(ctx *parser.Simple_componentContext) interface{} {
	return nil
}
