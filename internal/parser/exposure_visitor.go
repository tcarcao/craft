package parser

import (
	"github.com/tcarcao/craft/pkg/parser"
)

// =============================================================================
// Exposure Visitors
// =============================================================================

// Visit exposure definition
func (b *DSLModelBuilder) VisitExposure(ctx *parser.ExposureContext) interface{} {
	exposure := Exposure{
		To:      make([]string, 0),
		Of:      make([]string, 0),
		Through: make([]string, 0),
	}

	// Extract exposure name and properties
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if exposureName, ok := child.(*parser.Exposure_nameContext); ok {
			exposure.Name = exposureName.GetText()
		} else if exposureProps, ok := child.(*parser.Exposure_propertiesContext); ok {
			b.currentExposure = &exposure
			b.VisitExposure_properties(exposureProps)
		}
	}

	b.model.Exposures = append(b.model.Exposures, exposure)
	b.currentExposure = nil
	return nil
}

// Visit exposure properties
func (b *DSLModelBuilder) VisitExposure_properties(ctx *parser.Exposure_propertiesContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		if exposureProp, ok := ctx.GetChild(i).(*parser.Exposure_propertyContext); ok {
			b.VisitExposure_property(exposureProp)
		}
	}
	return nil
}

// Visit exposure property
func (b *DSLModelBuilder) VisitExposure_property(ctx *parser.Exposure_propertyContext) interface{} {
	if b.currentExposure == nil {
		return nil
	}

	// Check for different property types by iterating through children
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if targetList, ok := child.(*parser.Target_listContext); ok {
			b.currentExposure.To = b.extractTargetList(targetList)
		} else if domainList, ok := child.(*parser.Domain_listContext); ok {
			b.currentExposure.Of = b.extractDomainListFromExposure(domainList)
		} else if gatewayList, ok := child.(*parser.Gateway_listContext); ok {
			b.currentExposure.Through = b.extractGatewayList(gatewayList)
		}
	}

	return nil
}

// Extract target list
func (b *DSLModelBuilder) extractTargetList(ctx *parser.Target_listContext) []string {
	targets := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if target, ok := ctx.GetChild(i).(*parser.TargetContext); ok {
			targets = append(targets, target.GetText())
		}
	}

	return targets
}

// Extract domain list for exposure
func (b *DSLModelBuilder) extractDomainListFromExposure(ctx *parser.Domain_listContext) []string {
	domains := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if domainRef, ok := ctx.GetChild(i).(*parser.Domain_refContext); ok {
			domainName := b.extractIdentifier(&domainRef.BaseParserRuleContext)
			if domainName != "" {
				domains = append(domains, domainName)
			}
		}
	}

	return domains
}

// Extract gateway list
func (b *DSLModelBuilder) extractGatewayList(ctx *parser.Gateway_listContext) []string {
	gateways := make([]string, 0)

	for i := 0; i < ctx.GetChildCount(); i++ {
		if gateway, ok := ctx.GetChild(i).(*parser.GatewayContext); ok {
			gateways = append(gateways, gateway.GetText())
		}
	}

	return gateways
}

// Exposure visitor stubs
func (b *DSLModelBuilder) VisitExposure_name(ctx *parser.Exposure_nameContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitTarget_list(ctx *parser.Target_listContext) interface{}   { return nil }
func (b *DSLModelBuilder) VisitTarget(ctx *parser.TargetContext) interface{}             { return nil }
func (b *DSLModelBuilder) VisitGateway_list(ctx *parser.Gateway_listContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitGateway(ctx *parser.GatewayContext) interface{}           { return nil }
