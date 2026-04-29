package syntax

// AsFile wraps a SyntaxKindFile node as a typed File view.
// Returns a zero File if node is nil or wrong kind.
func AsFile(node *SyntaxNode) File { return File{node: node} }

// File is a typed view over a SyntaxKindFile node.
type File struct{ node *SyntaxNode }

// Actors returns all ActorDecl views — both standalone and those inside actors{} blocks,
// in document order.
func (f File) Actors() []ActorDecl {
	if f.node == nil {
		return nil
	}
	var result []ActorDecl
	for _, child := range f.node.Children {
		switch c := child.(type) {
		case *SyntaxNode:
			switch c.Kind {
			case SyntaxKindActorDecl:
				result = append(result, ActorDecl{node: c})
			case SyntaxKindActorsBlock:
				for _, n := range c.ChildNodes(SyntaxKindActorDecl) {
					result = append(result, ActorDecl{node: n})
				}
			}
		}
	}
	return result
}

// Domains returns all DomainDecl views — both standalone and those inside domains{} blocks,
// in document order.
func (f File) Domains() []DomainDecl {
	if f.node == nil {
		return nil
	}
	var result []DomainDecl
	for _, child := range f.node.Children {
		switch c := child.(type) {
		case *SyntaxNode:
			switch c.Kind {
			case SyntaxKindDomainDecl:
				result = append(result, DomainDecl{node: c})
			case SyntaxKindDomainsBlock:
				for _, n := range c.ChildNodes(SyntaxKindDomainDecl) {
					result = append(result, DomainDecl{node: n})
				}
			}
		}
	}
	return result
}

// Services returns all ServiceDecl views — both standalone and those inside services{} blocks,
// in document order.
func (f File) Services() []ServiceDecl {
	if f.node == nil {
		return nil
	}
	var result []ServiceDecl
	for _, child := range f.node.Children {
		switch c := child.(type) {
		case *SyntaxNode:
			switch c.Kind {
			case SyntaxKindServiceDecl:
				result = append(result, ServiceDecl{node: c})
			case SyntaxKindServicesBlock:
				for _, n := range c.ChildNodes(SyntaxKindServiceDecl) {
					result = append(result, ServiceDecl{node: n})
				}
			}
		}
	}
	return result
}

// UseCases returns all UseCaseDecl views.
func (f File) UseCases() []UseCaseDecl {
	if f.node == nil {
		return nil
	}
	var result []UseCaseDecl
	for _, n := range f.node.ChildNodes(SyntaxKindUseCaseDecl) {
		result = append(result, UseCaseDecl{node: n})
	}
	return result
}

// Archs returns all ArchDecl views.
func (f File) Archs() []ArchDecl {
	if f.node == nil {
		return nil
	}
	var result []ArchDecl
	for _, n := range f.node.ChildNodes(SyntaxKindArchDecl) {
		result = append(result, ArchDecl{node: n})
	}
	return result
}

// Exposures returns all ExposureDecl views.
func (f File) Exposures() []ExposureDecl {
	if f.node == nil {
		return nil
	}
	var result []ExposureDecl
	for _, n := range f.node.ChildNodes(SyntaxKindExposureDecl) {
		result = append(result, ExposureDecl{node: n})
	}
	return result
}

// ActorDecl is a typed view over a SyntaxKindActorDecl node.
type ActorDecl struct{ node *SyntaxNode }

// Keyword returns the 'actor' keyword token (present on standalone actor declarations).
// Returns nil for actors inside an actors{} block (which have no 'actor' keyword).
func (a ActorDecl) Keyword() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindKwActor)
}

// ActorType returns the user/system/service type token.
func (a ActorDecl) ActorType() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindKwUser, SyntaxKindKwSystem, SyntaxKindKwService)
}

// Name returns the identifier token for the actor's name.
func (a ActorDecl) Name() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindIdent)
}

// DomainDecl is a typed view over a SyntaxKindDomainDecl node.
type DomainDecl struct{ node *SyntaxNode }

// Keyword returns the 'domain' keyword token.
func (d DomainDecl) Keyword() *SyntaxToken { return d.node.ChildToken(SyntaxKindKwDomain) }

// Name returns the identifier token for the domain's name.
func (d DomainDecl) Name() *SyntaxToken { return d.node.ChildToken(SyntaxKindIdent) }

// BoundedContexts returns all BoundedContext views within this domain.
func (d DomainDecl) BoundedContexts() []BoundedContext {
	var result []BoundedContext
	for _, n := range d.node.ChildNodes(SyntaxKindBoundedContext) {
		result = append(result, BoundedContext{node: n})
	}
	return result
}

// BoundedContext is a typed view over a SyntaxKindBoundedContext node.
type BoundedContext struct{ node *SyntaxNode }

// Name returns the identifier token for the bounded context's name.
func (bc BoundedContext) Name() *SyntaxToken { return bc.node.ChildToken(SyntaxKindIdent) }

// DomainsBlock is a typed view over a SyntaxKindDomainsBlock node.
type DomainsBlock struct{ node *SyntaxNode }

// Domains returns all DomainDecl views within this block.
func (db DomainsBlock) Domains() []DomainDecl {
	var result []DomainDecl
	for _, n := range db.node.ChildNodes(SyntaxKindDomainDecl) {
		result = append(result, DomainDecl{node: n})
	}
	return result
}

// ServiceDecl is a typed view over a SyntaxKindServiceDecl node.
// Methods will be implemented in a later task.
type ServiceDecl struct{ node *SyntaxNode }

// UseCaseDecl is a typed view over a SyntaxKindUseCaseDecl node.
// Methods will be implemented in a later task.
type UseCaseDecl struct{ node *SyntaxNode }

// ArchDecl is a typed view over a SyntaxKindArchDecl node.
// Methods will be implemented in a later task.
type ArchDecl struct{ node *SyntaxNode }

// ExposureDecl is a typed view over a SyntaxKindExposureDecl node.
// Methods will be implemented in a later task.
type ExposureDecl struct{ node *SyntaxNode }
