package parser

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/tcarcao/archdsl/pkg/parser"
)

// =============================================================================
// Services Visitors
// =============================================================================

// Visit services section
func (b *DSLModelBuilder) VisitServices(ctx *parser.ServicesContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if serviceDefList, ok := child.(*parser.Service_definition_listContext); ok {
			b.VisitService_definition_list(serviceDefList)
		}
	}
	return nil
}

// Visit service definition list
func (b *DSLModelBuilder) VisitService_definition_list(ctx *parser.Service_definition_listContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if serviceDef, ok := child.(*parser.Service_definitionContext); ok {
			b.VisitService_definition(serviceDef)
		}
	}
	return nil
}

// Visit service definition
func (b *DSLModelBuilder) VisitService_definition(ctx *parser.Service_definitionContext) interface{} {
	service := Service{
		Domains:    make([]string, 0),
		DataStores: make([]string, 0),
		Deployment: DeploymentStrategy{
			Rules: make([]DeploymentRule, 0),
		},
	}

	// Extract service name and properties
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if serviceName, ok := child.(*parser.Service_nameContext); ok {
			service.Name = b.extractServiceName(serviceName)
		} else if serviceProps, ok := child.(*parser.Service_propertiesContext); ok {
			b.currentService = &service
			b.VisitService_properties(serviceProps)
		}
	}

	b.model.Services = append(b.model.Services, service)
	b.currentService = nil
	return nil
}

// Extract service name from service_name context
func (b *DSLModelBuilder) extractServiceName(ctx *parser.Service_nameContext) string {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if terminalNode, ok := ctx.GetChild(i).(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			text := terminalNode.GetText()

			switch tokenType {
			case parser.ArchDSLLexerIDENTIFIER:
				return text
			case parser.ArchDSLLexerSTRING:
				return strings.Trim(text, "\"")
			}
		}
	}
	return ""
}

// Visit service properties
func (b *DSLModelBuilder) VisitService_properties(ctx *parser.Service_propertiesContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if serviceProp, ok := ctx.GetChild(i).(*parser.Service_propertyContext); ok {
			b.VisitService_property(serviceProp)
		}
	}
	return nil
}

// Visit service property
func (b *DSLModelBuilder) VisitService_property(ctx *parser.Service_propertyContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			switch tokenType {
			case parser.ArchDSLLexerDOMAINS:
				// Find the domain_list after this token
				for j := i + 1; j < ctx.GetChildCount(); j++ {
					if domainList, ok := ctx.GetChild(j).(*parser.Domain_listContext); ok {
						b.VisitDomain_list(domainList)
						break
					}
				}
			case parser.ArchDSLLexerDATA_STORES:
				// Find the datastore_list after this token
				for j := i + 1; j < ctx.GetChildCount(); j++ {
					if datastoreList, ok := ctx.GetChild(j).(*parser.Datastore_listContext); ok {
						b.VisitDatastore_list(datastoreList)
						break
					}
				}
			case parser.ArchDSLLexerLANGUAGE:
				// Find the IDENTIFIER after this token
				for j := i + 1; j < ctx.GetChildCount(); j++ {
					if terminalNode, ok := ctx.GetChild(j).(antlr.TerminalNode); ok {
						if terminalNode.GetSymbol().GetTokenType() == parser.ArchDSLLexerIDENTIFIER {
							b.currentService.Language = terminalNode.GetText()
							break
						}
					}
				}
			case parser.ArchDSLLexerDEPLOYMENT:
				// Find the deployment_strategy after this token
				for j := i + 1; j < ctx.GetChildCount(); j++ {
					if deploymentStrategy, ok := ctx.GetChild(j).(*parser.Deployment_strategyContext); ok {
						b.VisitDeployment_strategy(deploymentStrategy)
						break
					}
				}
			}
		}
	}
	return nil
}

// Visit deployment strategy
func (b *DSLModelBuilder) VisitDeployment_strategy(ctx *parser.Deployment_strategyContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	// Extract deployment type and config
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if deploymentType, ok := child.(*parser.Deployment_typeContext); ok {
			b.currentService.Deployment.Type = deploymentType.GetText()
		} else if deploymentConfig, ok := child.(*parser.Deployment_configContext); ok {
			b.currentService.Deployment.Rules = b.extractDeploymentConfig(deploymentConfig)
		}
	}

	return nil
}

// Extract deployment configuration
func (b *DSLModelBuilder) extractDeploymentConfig(ctx *parser.Deployment_configContext) []DeploymentRule {
	rules := make([]DeploymentRule, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if deploymentRule, ok := ctx.GetChild(i).(*parser.Deployment_ruleContext); ok {
			rule := b.extractDeploymentRule(deploymentRule)
			if rule != nil {
				rules = append(rules, *rule)
			}
		}
	}

	return rules
}

// Extract deployment rule
func (b *DSLModelBuilder) extractDeploymentRule(ctx *parser.Deployment_ruleContext) *DeploymentRule {
	rule := &DeploymentRule{}

	// Find PERCENTAGE and deployment target
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)

		if terminalNode, ok := child.(antlr.TerminalNode); ok {
			tokenType := terminalNode.GetSymbol().GetTokenType()
			switch tokenType {
			case parser.ArchDSLLexerPERCENTAGE:
				rule.Percentage = terminalNode.GetText()
			}
		} else if deploymentTarget, ok := child.(*parser.Deployment_targetContext); ok {
			rule.Target = deploymentTarget.GetText()
		}
	}

	return rule
}

// Visit domain list - works with domain_ref
func (b *DSLModelBuilder) VisitDomain_list(ctx *parser.Domain_listContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if domainRef, ok := ctx.GetChild(i).(*parser.Domain_refContext); ok {
			domainName := b.extractIdentifier(&domainRef.BaseParserRuleContext)
			if domainName != "" {
				b.currentService.Domains = append(b.currentService.Domains, domainName)
			}
		}
	}
	return nil
}

// Visit datastore list
func (b *DSLModelBuilder) VisitDatastore_list(ctx *parser.Datastore_listContext) interface{} {
	if b.currentService == nil {
		return nil
	}

	for i := 0; i < ctx.GetChildCount(); i++ {
		if datastore, ok := ctx.GetChild(i).(*parser.DatastoreContext); ok {
			datastoreName := b.extractIdentifier(&datastore.BaseParserRuleContext)
			if datastoreName != "" {
				b.currentService.DataStores = append(b.currentService.DataStores, datastoreName)
			}
		}
	}
	return nil
}

// Service visitor stubs
func (b *DSLModelBuilder) VisitService_name(ctx *parser.Service_nameContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitDeployment_type(ctx *parser.Deployment_typeContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitDeployment_config(ctx *parser.Deployment_configContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitDeployment_rule(ctx *parser.Deployment_ruleContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitDeployment_target(ctx *parser.Deployment_targetContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitDomain_ref(ctx *parser.Domain_refContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitDomain(ctx *parser.DomainContext) interface{}         { return nil }
func (b *DSLModelBuilder) VisitDatastore(ctx *parser.DatastoreContext) interface{}   { return nil }
