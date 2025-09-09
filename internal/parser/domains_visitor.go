package parser

import (
	"github.com/tcarcao/archdsl/pkg/parser"
)

// =============================================================================
// Domains Visitors
// =============================================================================

// Visit single domain definition - "domain domain_name { subdomain_list }"
func (b *DSLModelBuilder) VisitDomain_def(ctx *parser.Domain_defContext) interface{} {
	domain := Domain{
		SubDomains: make([]string, 0),
	}

	// Extract domain name and subdomain list
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainName, ok := child.(*parser.Domain_nameContext); ok {
			domain.Name = b.extractIdentifier(&domainName.BaseParserRuleContext)
		} else if subdomainList, ok := child.(*parser.Subdomain_listContext); ok {
			domain.SubDomains = b.extractSubdomainList(subdomainList)
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
		SubDomains: make([]string, 0),
	}

	// Extract domain name and subdomain list
	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if domainName, ok := child.(*parser.Domain_nameContext); ok {
			domain.Name = b.extractIdentifier(&domainName.BaseParserRuleContext)
		} else if subdomainList, ok := child.(*parser.Subdomain_listContext); ok {
			domain.SubDomains = b.extractSubdomainList(subdomainList)
		}
	}

	b.addOrMergeDomain(domain)
	return nil
}

// Extract subdomain list from subdomain_list context
func (b *DSLModelBuilder) extractSubdomainList(ctx *parser.Subdomain_listContext) []string {
	subdomainSet := make(map[string]bool)

	for i := 0; i < ctx.GetChildCount(); i++ {
		child := ctx.GetChild(i)
		if subdomain, ok := child.(*parser.SubdomainContext); ok {
			subdomainName := b.extractIdentifier(&subdomain.BaseParserRuleContext)
			if subdomainName != "" {
				subdomainSet[subdomainName] = true
			}
		}
	}

	// Convert set to slice
	subdomains := make([]string, 0, len(subdomainSet))
	for subdomain := range subdomainSet {
		subdomains = append(subdomains, subdomain)
	}

	return subdomains
}

// addOrMergeDomain adds a domain to the model or merges subdomains if domain already exists
func (b *DSLModelBuilder) addOrMergeDomain(newDomain Domain) {
	// Check if domain already exists
	for i := range b.model.Domains {
		if b.model.Domains[i].Name == newDomain.Name {
			// Domain exists, merge subdomains
			b.model.Domains[i].SubDomains = b.mergeSubdomains(b.model.Domains[i].SubDomains, newDomain.SubDomains)
			return
		}
	}
	// Domain doesn't exist, add it
	b.model.Domains = append(b.model.Domains, newDomain)
}

// mergeSubdomains merges two subdomain slices, avoiding duplicates
func (b *DSLModelBuilder) mergeSubdomains(existing, new []string) []string {
	subdomainSet := make(map[string]bool)
	
	// Add existing subdomains to set
	for _, subdomain := range existing {
		subdomainSet[subdomain] = true
	}
	
	// Add new subdomains to set
	for _, subdomain := range new {
		subdomainSet[subdomain] = true
	}
	
	// Convert set back to slice
	merged := make([]string, 0, len(subdomainSet))
	for subdomain := range subdomainSet {
		merged = append(merged, subdomain)
	}
	
	return merged
}

// Domain visitor stubs for completeness
func (b *DSLModelBuilder) VisitDomain_name(ctx *parser.Domain_nameContext) interface{} { return nil }
func (b *DSLModelBuilder) VisitSubdomain_list(ctx *parser.Subdomain_listContext) interface{} {
	return nil
}
func (b *DSLModelBuilder) VisitSubdomain(ctx *parser.SubdomainContext) interface{} { return nil }