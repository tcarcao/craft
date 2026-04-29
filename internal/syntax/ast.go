package syntax

// isAstFieldSentinel returns true when tokens[i] is an ident followed by a colon —
// the start of a new field definition.
func isAstFieldSentinel(tokens []*SyntaxToken, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	return tokens[i].Kind == SyntaxKindIdent && tokens[i+1].Kind == SyntaxKindColon
}

// collectAstIdentList collects comma-separated ident/string values from tokens[i].
// Stops at a field sentinel (ident+colon), RBrace, or non-ident/string.
func collectAstIdentList(tokens []*SyntaxToken, i int) (names []string, lines []int, newI int) {
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind == SyntaxKindComma {
			i++
			continue
		}
		if (tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString) && !isAstFieldSentinel(tokens, i) {
			names = append(names, tok.Value)
			lines = append(lines, tok.Line)
			i++
		} else {
			break
		}
	}
	return names, lines, i
}

// scanBodyTokens returns tokens inside the first LBrace…RBrace pair of a node.
func scanBodyTokens(node *SyntaxNode) []*SyntaxToken {
	all := node.Tokens()
	for i, tok := range all {
		if tok.Kind == SyntaxKindLBrace {
			return all[i+1:]
		}
	}
	return nil
}

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

// IsGrouped returns true when the domain was declared inside a domains { } block.
// Standalone domains begin with the `domain` keyword; grouped domains begin with their name.
func (d DomainDecl) IsGrouped() bool {
	return d.node.ChildToken(SyntaxKindKwDomain) == nil
}

// EndLine returns the 1-based line of the closing `}`.
func (d DomainDecl) EndLine() int {
	tok := d.node.ChildToken(SyntaxKindRBrace)
	if tok == nil {
		return 0
	}
	return tok.Line
}

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

// ActorsBlock is a typed view over a SyntaxKindActorsBlock node.
type ActorsBlock struct{ node *SyntaxNode }

// Line returns the 1-based source line of the `actors` keyword.
func (b ActorsBlock) Line() int {
	tok := b.node.ChildToken(SyntaxKindKwActors)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// EndLine returns the 1-based line of the closing `}`.
func (b ActorsBlock) EndLine() int {
	tok := b.node.ChildToken(SyntaxKindRBrace)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// ServiceDecl is a typed view over a SyntaxKindServiceDecl node.
type ServiceDecl struct{ node *SyntaxNode }

func (s ServiceDecl) Keyword() *SyntaxToken { return s.node.ChildToken(SyntaxKindKwService) }
func (s ServiceDecl) Name() *SyntaxToken    { return s.node.ChildToken(SyntaxKindIdent) }

// IsGrouped returns true when the service was declared inside a services { } block.
func (s ServiceDecl) IsGrouped() bool {
	return s.node.ChildToken(SyntaxKindKwService) == nil
}

// EndLine returns the 1-based line of the closing `}`.
func (s ServiceDecl) EndLine() int {
	tok := s.node.ChildToken(SyntaxKindRBrace)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// serviceBodyFields holds all parsed service body fields.
type serviceBodyFields struct {
	Contexts        []string
	ContextLines    []int
	DataStores      []string
	Language        string
	DeploymentType  string
	DeploymentRules []struct{ Percentage, Target string }
}

// parseServiceBody scans the service body tokens and extracts field values.
func (s ServiceDecl) parseServiceBody() serviceBodyFields {
	var f serviceBodyFields
	tokens := scanBodyTokens(s.node)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind != SyntaxKindIdent {
			i++
			continue
		}
		fieldName := tok.Value
		if i+1 >= len(tokens) || tokens[i+1].Kind != SyntaxKindColon {
			i++
			continue
		}
		i += 2 // skip fieldName + colon

		switch fieldName {
		case "contexts":
			f.Contexts, f.ContextLines, i = collectAstIdentList(tokens, i)
		case "data-stores":
			f.DataStores, _, i = collectAstIdentList(tokens, i)
		case "language":
			if i < len(tokens) && tokens[i].Kind == SyntaxKindIdent {
				f.Language = tokens[i].Value
				i++
			}
		case "deployment":
			if i < len(tokens) && tokens[i].Kind == SyntaxKindIdent {
				f.DeploymentType = tokens[i].Value
				i++
			}
			if i < len(tokens) && tokens[i].Kind == SyntaxKindLParen {
				i++
				for i < len(tokens) && tokens[i].Kind != SyntaxKindRParen {
					if tokens[i].Kind != SyntaxKindPercentage {
						i++
						continue
					}
					pct := tokens[i].Value
					i++
					if i < len(tokens) && tokens[i].Kind == SyntaxKindArrow {
						i++
					}
					var target string
					if i < len(tokens) && tokens[i].Kind == SyntaxKindIdent {
						target = tokens[i].Value
						i++
					}
					f.DeploymentRules = append(f.DeploymentRules, struct{ Percentage, Target string }{pct, target})
					if i < len(tokens) && tokens[i].Kind == SyntaxKindComma {
						i++
					}
				}
				if i < len(tokens) && tokens[i].Kind == SyntaxKindRParen {
					i++
				}
			}
		default:
			for i < len(tokens) {
				if tokens[i].Kind == SyntaxKindRBrace || tokens[i].Kind == SyntaxKindIdent {
					break
				}
				i++
			}
		}
	}
	return f
}

// Contexts returns the context names listed in the service body.
func (s ServiceDecl) Contexts() []string { return s.parseServiceBody().Contexts }

// ContextLines returns the 1-based source line of each context name token.
func (s ServiceDecl) ContextLines() []int { return s.parseServiceBody().ContextLines }

// DataStores returns the data-store names listed in the service body.
func (s ServiceDecl) DataStores() []string { return s.parseServiceBody().DataStores }

// Language returns the language value, or empty string if absent.
func (s ServiceDecl) Language() string { return s.parseServiceBody().Language }

// DeploymentType returns the deployment strategy type (e.g. "canary"), or empty string.
func (s ServiceDecl) DeploymentType() string { return s.parseServiceBody().DeploymentType }

// DeploymentRules returns the percentage→target rules for parameterised deployment.
func (s ServiceDecl) DeploymentRules() []struct{ Percentage, Target string } {
	return s.parseServiceBody().DeploymentRules
}

// UseCaseDecl is a typed view over a SyntaxKindUseCaseDecl node.
type UseCaseDecl struct{ node *SyntaxNode }

func (u UseCaseDecl) Keyword() *SyntaxToken { return u.node.ChildToken(SyntaxKindKwUseCase) }
func (u UseCaseDecl) Title() *SyntaxToken   { return u.node.ChildToken(SyntaxKindString) }

// EndLine returns the 1-based line of the closing `}`.
func (u UseCaseDecl) EndLine() int {
	tok := u.node.ChildToken(SyntaxKindRBrace)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// Scenarios returns all ScenarioDecl views within this use case.
func (u UseCaseDecl) Scenarios() []ScenarioDecl {
	var result []ScenarioDecl
	for _, n := range u.node.ChildNodes(SyntaxKindScenario) {
		result = append(result, ScenarioDecl{node: n})
	}
	return result
}

// ScenarioDecl is a typed view over a SyntaxKindScenario node.
type ScenarioDecl struct{ node *SyntaxNode }

// When returns the 'when' keyword token that starts this scenario.
func (s ScenarioDecl) When() *SyntaxToken { return s.node.ChildToken(SyntaxKindKwWhen) }

// Trigger returns the trigger sub-node of this scenario.
func (s ScenarioDecl) Trigger() TriggerDecl {
	return TriggerDecl{node: s.node.ChildNode(SyntaxKindTrigger)}
}

// Actions returns all action nodes in this scenario.
func (s ScenarioDecl) Actions() []ActionDecl {
	var result []ActionDecl
	for _, n := range s.node.ChildNodes(SyntaxKindAction) {
		result = append(result, ActionDecl{node: n})
	}
	return result
}

// TriggerDecl is a typed view over a SyntaxKindTrigger node.
type TriggerDecl struct{ node *SyntaxNode }

// Subject returns the subject identifier token of the trigger (actor/domain name).
func (t TriggerDecl) Subject() *SyntaxToken { return t.node.ChildToken(SyntaxKindIdent) }

// Event returns the string token for event-style triggers (when "<EventName>").
func (t TriggerDecl) Event() *SyntaxToken { return t.node.ChildToken(SyntaxKindString) }

// ActionDecl is a typed view over a SyntaxKindAction node.
type ActionDecl struct{ node *SyntaxNode }

// Subject returns the subject identifier token (domain/service name).
func (a ActionDecl) Subject() *SyntaxToken { return a.node.ChildToken(SyntaxKindIdent) }

// Verb returns the action verb token (asks/notifies/listens/returns).
func (a ActionDecl) Verb() *SyntaxToken {
	return a.node.ChildToken(
		SyntaxKindKwAsks, SyntaxKindKwNotifies,
		SyntaxKindKwListens, SyntaxKindKwReturns,
	)
}

// Connector returns the 'to' keyword if present.
func (a ActionDecl) Connector() *SyntaxToken { return a.node.ChildToken(SyntaxKindKwTo) }

// ArchDecl is a typed view over a SyntaxKindArchDecl node.
type ArchDecl struct{ node *SyntaxNode }

// Keyword returns the 'arch' keyword token.
func (a ArchDecl) Keyword() *SyntaxToken { return a.node.ChildToken(SyntaxKindKwArch) }

// Name returns the identifier token for the arch's name (optional).
func (a ArchDecl) Name() *SyntaxToken { return a.node.ChildToken(SyntaxKindIdent) }

// Line returns the 1-based source line of the `arch` keyword.
func (a ArchDecl) Line() int {
	tok := a.node.ChildToken(SyntaxKindKwArch)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// EndLine returns the 1-based line of the closing `}`.
func (a ArchDecl) EndLine() int {
	tok := a.node.ChildToken(SyntaxKindRBrace)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// PresentationLine returns the 1-based line of the `presentation:` label, or 0 if absent.
func (a ArchDecl) PresentationLine() int {
	for _, child := range a.node.Children {
		section, ok := child.(*SyntaxNode)
		if !ok || section.Kind != SyntaxKindArchSection {
			continue
		}
		tok := section.ChildToken(SyntaxKindKwPresentation)
		if tok != nil {
			return tok.Line
		}
	}
	return 0
}

// GatewayLine returns the 1-based line of the `gateway:` label, or 0 if absent.
func (a ArchDecl) GatewayLine() int {
	for _, child := range a.node.Children {
		section, ok := child.(*SyntaxNode)
		if !ok || section.Kind != SyntaxKindArchSection {
			continue
		}
		tok := section.ChildToken(SyntaxKindKwGateway)
		if tok != nil {
			return tok.Line
		}
	}
	return 0
}

// Sections returns all ArchSection views within this arch block.
func (a ArchDecl) Sections() []ArchSection {
	var result []ArchSection
	for _, n := range a.node.ChildNodes(SyntaxKindArchSection) {
		result = append(result, ArchSection{node: n})
	}
	return result
}

// ArchSection is a typed view over a SyntaxKindArchSection node.
type ArchSection struct{ node *SyntaxNode }

// Keyword returns the section label keyword token (presentation or gateway).
func (s ArchSection) Keyword() *SyntaxToken {
	if s.node == nil {
		return nil
	}
	return s.node.ChildToken(SyntaxKindKwPresentation, SyntaxKindKwGateway, SyntaxKindIdent)
}

// Components returns all ArchComponent views within this section.
func (s ArchSection) Components() []ArchComponent {
	if s.node == nil {
		return nil
	}
	var result []ArchComponent
	for _, n := range s.node.ChildNodes(SyntaxKindArchComponent) {
		result = append(result, ArchComponent{node: n})
	}
	return result
}

// ArchComponent is a typed view over a SyntaxKindArchComponent node.
type ArchComponent struct{ node *SyntaxNode }

// Name returns the identifier token for the component's name.
func (c ArchComponent) Name() *SyntaxToken {
	if c.node == nil {
		return nil
	}
	return c.node.ChildToken(SyntaxKindIdent)
}

// Modifiers returns all ArchModifier views within this component.
func (c ArchComponent) Modifiers() []ArchModifier {
	if c.node == nil {
		return nil
	}
	var result []ArchModifier
	for _, n := range c.node.ChildNodes(SyntaxKindArchModifier) {
		result = append(result, ArchModifier{node: n})
	}
	return result
}

// ArchModifier is a typed view over a SyntaxKindArchModifier node.
type ArchModifier struct{ node *SyntaxNode }

// Key returns the identifier token for the modifier key.
func (m ArchModifier) Key() *SyntaxToken {
	if m.node == nil {
		return nil
	}
	return m.node.ChildToken(SyntaxKindIdent)
}

// Value returns the value token for the modifier (ident, string, or number).
// Returns nil if the modifier has no value (key-only modifier).
func (m ArchModifier) Value() *SyntaxToken {
	if m.node == nil {
		return nil
	}
	// The modifier node children are: Ident (key), optional Colon, optional value token.
	// The value token follows the Colon and may be Ident, String, or Number.
	colonSeen := false
	for _, child := range m.node.Children {
		tok, ok := child.(*SyntaxToken)
		if !ok {
			continue
		}
		if tok.Kind == SyntaxKindColon {
			colonSeen = true
			continue
		}
		if colonSeen {
			return tok
		}
	}
	return nil
}

// ExposureDecl is a typed view over a SyntaxKindExposureDecl node.
type ExposureDecl struct{ node *SyntaxNode }

// Keyword returns the 'exposure' keyword token.
func (e ExposureDecl) Keyword() *SyntaxToken { return e.node.ChildToken(SyntaxKindKwExposure) }

// Name returns the identifier token for the exposure's name.
func (e ExposureDecl) Name() *SyntaxToken { return e.node.ChildToken(SyntaxKindIdent) }

// Rules returns all DeploymentRule views within this exposure block.
func (e ExposureDecl) Rules() []DeploymentRule {
	var result []DeploymentRule
	for _, n := range e.node.ChildNodes(SyntaxKindDeploymentRule) {
		result = append(result, DeploymentRule{node: n})
	}
	return result
}

// exposureBodyFields holds parsed exposure body values.
type exposureBodyFields struct {
	To       []string
	Contexts []string
	Through  []string
}

// collectAstExposureIdentList collects idents for exposure fields.
func collectAstExposureIdentList(tokens []*SyntaxToken, i int) ([]string, []int, int) {
	var names []string
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		if tok.Kind == SyntaxKindComma {
			i++
			continue
		}
		isFieldKw := tok.Kind == SyntaxKindKwTo || tok.Kind == SyntaxKindKwContexts ||
			tok.Kind == SyntaxKindKwThrough || tok.Kind == SyntaxKindIdent
		if isFieldKw && i+1 < len(tokens) && tokens[i+1].Kind == SyntaxKindColon {
			break
		}
		if tok.Kind == SyntaxKindIdent || tok.Kind == SyntaxKindString {
			names = append(names, tok.Value)
		}
		i++
	}
	return names, nil, i
}

func (e ExposureDecl) parseExposureBody() exposureBodyFields {
	var f exposureBodyFields
	tokens := scanBodyTokens(e.node)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind == SyntaxKindRBrace {
			break
		}
		var fieldName string
		switch tok.Kind {
		case SyntaxKindKwTo:
			fieldName = "to"
		case SyntaxKindKwContexts:
			fieldName = "contexts"
		case SyntaxKindKwThrough:
			fieldName = "through"
		case SyntaxKindIdent:
			fieldName = tok.Value
		default:
			i++
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].Kind != SyntaxKindColon {
			i++
			continue
		}
		i += 2
		var names []string
		names, _, i = collectAstExposureIdentList(tokens, i)
		switch fieldName {
		case "to":
			f.To = names
		case "contexts":
			f.Contexts = names
		case "through":
			f.Through = names
		}
	}
	return f
}

// Line returns the 1-based source line of the `exposure` keyword.
func (e ExposureDecl) Line() int {
	tok := e.node.ChildToken(SyntaxKindKwExposure)
	if tok == nil {
		return 0
	}
	return tok.Line
}

// To returns the `to:` target names.
func (e ExposureDecl) To() []string { return e.parseExposureBody().To }

// ExposureContexts returns the `contexts:` names.
func (e ExposureDecl) ExposureContexts() []string { return e.parseExposureBody().Contexts }

// Through returns the `through:` names.
func (e ExposureDecl) Through() []string { return e.parseExposureBody().Through }

// DeploymentRule is a typed view over a SyntaxKindDeploymentRule node.
// In exposure blocks this wraps the 'through: <value>' clause.
type DeploymentRule struct{ node *SyntaxNode }

// Through returns the 'through' keyword token.
func (r DeploymentRule) Through() *SyntaxToken {
	if r.node == nil {
		return nil
	}
	return r.node.ChildToken(SyntaxKindKwThrough)
}

// Arrow returns the '->' token (present in service deployment rules).
func (r DeploymentRule) Arrow() *SyntaxToken {
	if r.node == nil {
		return nil
	}
	return r.node.ChildToken(SyntaxKindArrow)
}
