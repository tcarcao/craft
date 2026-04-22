package parser

import (
	"github.com/tcarcao/craft/pkg/parser"
)

// =============================================================================
// Domains Visitors
// =============================================================================

// Visit single domain definition - "domain domain_name { bounded_context_list }"
func (b *DSLModelBuilder) VisitDomain_def(ctx *parser.Domain_defContext) interface{} {
	domain := Domain{
		BoundedContexts: make([]string, 0),
	}

	// Extract domain name and bounded context list
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainName, ok := child.(*parser.Domain_nameContext); ok {
			domain.Name = b.extractIdentifier(&domainName.BaseParserRuleContext)
		} else if subdomainList, ok := child.(*parser.Subdomain_listContext); ok {
			domain.BoundedContexts = b.extractSubdomainList(subdomainList)
		}
	}

	b.addOrMergeDomain(domain)
	return nil
}

// Visit multiple domains definition - "domains { domain_block_list }"
func (b *DSLModelBuilder) VisitDomains_def(ctx *parser.Domains_defContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainBlockList, ok := child.(*parser.Domain_block_listContext); ok {
			b.VisitDomain_block_list(domainBlockList)
		}
	}
	return nil
}

// Visit domain block list
func (b *DSLModelBuilder) VisitDomain_block_list(ctx *parser.Domain_block_listContext) interface{} {
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainBlock, ok := child.(*parser.Domain_blockContext); ok {
			b.VisitDomain_block(domainBlock)
		}
	}
	return nil
}

// Visit individual domain block
func (b *DSLModelBuilder) VisitDomain_block(ctx *parser.Domain_blockContext) interface{} {
	domain := Domain{
		BoundedContexts: make([]string, 0),
	}

	// Extract domain name and bounded context list
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainName, ok := child.(*parser.Domain_nameContext); ok {
			domain.Name = b.extractIdentifier(&domainName.BaseParserRuleContext)
		} else if subdomainList, ok := child.(*parser.Subdomain_listContext); ok {
			domain.BoundedContexts = b.extractSubdomainList(subdomainList)
		}
	}

	b.addOrMergeDomain(domain)
	return nil
}

// extractSubdomainList extracts bounded context names from subdomain_list context
func (b *DSLModelBuilder) extractSubdomainList(ctx *parser.Subdomain_listContext) []string {
	contextSet := make(map[string]bool)

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if sd, ok := child.(*parser.SubdomainContext); ok {
			name := b.extractIdentifier(&sd.BaseParserRuleContext)
			if name != "" {
				contextSet[name] = true
			}
		}
	}

	contexts := make([]string, 0, len(contextSet))
	for name := range contextSet {
		contexts = append(contexts, name)
	}

	return contexts
}

// addOrMergeDomain adds a domain to the model or merges bounded contexts if domain already exists
func (b *DSLModelBuilder) addOrMergeDomain(newDomain Domain) {
	// Check if domain already exists
	for i := range b.model.Domains {
		if b.model.Domains[i].Name == newDomain.Name {
			// Domain exists, merge bounded contexts
			b.model.Domains[i].BoundedContexts = b.mergeBoundedContexts(b.model.Domains[i].BoundedContexts, newDomain.BoundedContexts)
			return
		}
	}
	// Domain doesn't exist, add it
	b.model.Domains = append(b.model.Domains, newDomain)
}

// mergeBoundedContexts merges two bounded context slices, avoiding duplicates
func (b *DSLModelBuilder) mergeBoundedContexts(existing, new []string) []string {
	contextSet := make(map[string]bool)

	for _, name := range existing {
		contextSet[name] = true
	}

	for _, name := range new {
		contextSet[name] = true
	}

	merged := make([]string, 0, len(contextSet))
	for name := range contextSet {
		merged = append(merged, name)
	}

	return merged
}

// Domain visitor stubs for completeness
func (b *DSLModelBuilder) VisitDomain_name(ctx *parser.Domain_nameContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitSubdomain_list(ctx *parser.Subdomain_listContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitSubdomain(ctx *parser.SubdomainContext) interface{} {
	return nil
}
