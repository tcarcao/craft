package parser

import (
	"fmt"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

// Enhanced debug visitor to see what's being parsed including services
type DebugVisitor struct {
	*parser.BaseArchDSLVisitor
	depth int
	lexer *parser.ArchDSLLexer // Store lexer reference for token name lookup
}

func NewDebugVisitor() *DebugVisitor {
	return &DebugVisitor{
		BaseArchDSLVisitor: &parser.BaseArchDSLVisitor{},
		depth:              0,
	}
}

func (d *DebugVisitor) setLexer(lexer *parser.ArchDSLLexer) {
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
		case *parser.ServicesContext:
			d.VisitServices(c)
		case *parser.Use_caseContext:
			d.VisitUse_case(c)
		default:
			if terminalNode, ok := child.(antlr.TerminalNode); ok {
				fmt.Printf("%s  Terminal token type: %d\n", d.indent(), terminalNode.GetSymbol().GetTokenType())
			}
		}
	}

	d.depth--
	return nil
}

// New services-related visitor methods
func (d *DebugVisitor) VisitServices(ctx *parser.ServicesContext) interface{} {
	fmt.Printf("%sServices: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Service_definition_listContext:
			d.VisitService_definition_list(c)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitService_definition_list(ctx *parser.Service_definition_listContext) interface{} {
	fmt.Printf("%sService_definition_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if serviceDef, ok := child.(*parser.Service_definitionContext); ok {
			d.VisitService_definition(serviceDef)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitService_definition(ctx *parser.Service_definitionContext) interface{} {
	fmt.Printf("%sService_definition: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Service_nameContext:
			d.VisitService_name(c)
		case *parser.Service_propertiesContext:
			d.VisitService_properties(c)
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

func (d *DebugVisitor) VisitService_name(ctx *parser.Service_nameContext) interface{} {
	fmt.Printf("%sService_name: %d children\n", d.indent(), ctx.GetChildCount())
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

func (d *DebugVisitor) VisitService_properties(ctx *parser.Service_propertiesContext) interface{} {
	fmt.Printf("%sService_properties: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if serviceProp, ok := child.(*parser.Service_propertyContext); ok {
			d.VisitService_property(serviceProp)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitService_property(ctx *parser.Service_propertyContext) interface{} {
	fmt.Printf("%sService_property: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		switch c := child.(type) {
		case *parser.Domain_listContext:
			d.VisitDomain_list(c)
		case *parser.Datastore_listContext:
			d.VisitDatastore_list(c)
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

func (d *DebugVisitor) VisitDomain_list(ctx *parser.Domain_listContext) interface{} {
	fmt.Printf("%sDomain_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if domainOrDatastore, ok := child.(*parser.Domain_or_datastoreContext); ok {
			d.VisitDomain_or_datastore(domainOrDatastore)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitDatastore_list(ctx *parser.Datastore_listContext) interface{} {
	fmt.Printf("%sDatastore_list: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)

		if domainOrDatastore, ok := child.(*parser.Domain_or_datastoreContext); ok {
			d.VisitDomain_or_datastore(domainOrDatastore)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitDomain_or_datastore(ctx *parser.Domain_or_datastoreContext) interface{} {
	fmt.Printf("%sDomain_or_datastore: %d children\n", d.indent(), ctx.GetChildCount())
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

// Existing use case visitor methods (unchanged)
func (d *DebugVisitor) VisitUse_case(ctx *parser.Use_caseContext) interface{} {
	fmt.Printf("%sUse_case: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
		if scenario, ok := child.(*parser.ScenarioContext); ok {
			d.VisitScenario(scenario)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitScenario(ctx *parser.ScenarioContext) interface{} {
	fmt.Printf("%sScenario: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
		switch c := child.(type) {
		case *parser.TriggerContext:
			d.VisitTrigger(c)
		case *parser.Action_blockContext:
			d.VisitAction_block(c)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitTrigger(ctx *parser.TriggerContext) interface{} {
	fmt.Printf("%sTrigger: %d children\n", d.indent(), ctx.GetChildCount())
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

func (d *DebugVisitor) VisitAction_block(ctx *parser.Action_blockContext) interface{} {
	fmt.Printf("%sAction_block: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
		if action, ok := child.(*parser.ActionContext); ok {
			d.VisitAction(action)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitAction(ctx *parser.ActionContext) interface{} {
	fmt.Printf("%sAction: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
		switch c := child.(type) {
		case *parser.Internal_actionContext:
			d.VisitInternal_action(c)
		case *parser.Sync_actionContext:
			d.VisitSync_action(c)
		case *parser.Async_actionContext:
			d.VisitAsync_action(c)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitInternal_action(ctx *parser.Internal_actionContext) interface{} {
	fmt.Printf("%sInternal_action: %d children\n", d.indent(), ctx.GetChildCount())
	d.depth++

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		childText := ""
		if parseTree, ok := child.(antlr.ParseTree); ok {
			childText = parseTree.GetText()
		}
		fmt.Printf("%sChild %d: %T = '%s'\n", d.indent(), i, child, childText)
		if phrase, ok := child.(*parser.PhraseContext); ok {
			d.VisitPhrase(phrase)
		}
	}

	d.depth--
	return nil
}

func (d *DebugVisitor) VisitPhrase(ctx *parser.PhraseContext) interface{} {
	fmt.Printf("%sPhrase: %d children\n", d.indent(), ctx.GetChildCount())
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

// Implement remaining methods as no-ops
func (d *DebugVisitor) VisitSync_action(ctx *parser.Sync_actionContext) interface{} {
	fmt.Printf("%sSync_action: %s\n", d.indent(), ctx.GetText())
	return nil
}
func (d *DebugVisitor) VisitAsync_action(ctx *parser.Async_actionContext) interface{} {
	fmt.Printf("%sAsync_action: %s\n", d.indent(), ctx.GetText())
	return nil
}
func (d *DebugVisitor) VisitExternal_trigger(ctx *parser.External_triggerContext) interface{} {
	return nil
}
func (d *DebugVisitor) VisitWord(ctx *parser.WordContext) interface{}                 { return nil }
func (d *DebugVisitor) VisitActor(ctx *parser.ActorContext) interface{}               { return nil }
func (d *DebugVisitor) VisitDomain(ctx *parser.DomainContext) interface{}             { return nil }
func (d *DebugVisitor) VisitVerb(ctx *parser.VerbContext) interface{}                 { return nil }
func (d *DebugVisitor) VisitQuoted_event(ctx *parser.Quoted_eventContext) interface{} { return nil }
func (d *DebugVisitor) VisitString(ctx *parser.StringContext) interface{}             { return nil }
func (d *DebugVisitor) VisitDatastore(ctx *parser.DatastoreContext) interface{}       { return nil }

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

// New test specifically for services debugging
func TestDebugServicesOnly(t *testing.T) {
	dsl := `services {
  WalletService: {
    domains: Wallet, WalletItemPurchase,
    data-stores: wallet_db
  }
}`

	debugParseDSL(t, dsl)
}

func TestDebugSimpleService(t *testing.T) {
	dsl := `services {
  TestService: {
    domains: TestDomain
  }
}`

	debugParseDSL(t, dsl)
}

func TestDebugServiceWithDataStores(t *testing.T) {
	dsl := `services {
  TestService: {
    data-stores: test_db
  }
}`

	debugParseDSL(t, dsl)
}

func TestDebugMinimalService(t *testing.T) {
	dsl := `services {
}`

	debugParseDSL(t, dsl)
}

// Test the original failing case
func TestDebugOriginalServiceCase(t *testing.T) {
	dsl := `services {
  WalletService: {
    domains: Wallet, WalletItemPurchase
    data-stores: wallet_db
  },
  "Order Service": {
    domains: OrderManagement,
    data-stores: order_db
  }
}`

	debugParseDSL(t, dsl)
}

// Existing test functions from original code
func TestDebugSimpleCase(t *testing.T) {
	dsl := `use_case "Simple Registration" {
		when user creates account
			authentication marks the user as verified
			notification sends welcome email
	}`

	debugParseDSL(t, dsl)
}
