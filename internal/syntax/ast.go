package syntax

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/internal/green"
)

// nodeFirstTokenLine returns the 1-based line of the first token in node (via li),
// or 0 if the node has no tokens or li is zero.
func nodeFirstTokenLine(node SyntaxNode, li green.LineIndex) int {
	toks := node.Tokens()
	if len(toks) == 0 {
		return 0
	}
	line, _ := li.LineCol(toks[0].Offset())
	return line
}

// nodeEndLine returns the 1-based line of the last RBrace token in node (via li),
// or 0 if not found.
func nodeEndLine(node SyntaxNode, li green.LineIndex) int {
	toks := node.AllTokens()
	for i := len(toks) - 1; i >= 0; i-- {
		if toks[i].Kind() == SyntaxKindRBrace {
			line, _ := li.LineCol(toks[i].Offset())
			return line
		}
	}
	return 0
}

// nodeFirstTokenCol returns the 1-based column of the first token in node (via li),
// or 0 if the node has no tokens.
func nodeFirstTokenCol(node SyntaxNode, li green.LineIndex) int {
	toks := node.Tokens()
	if len(toks) == 0 {
		return 0
	}
	_, col := li.LineCol(toks[0].Offset())
	return col
}

// connectorAt returns tokens[i] if it is a connector (KwTo or an isConnectorWord
// ident), or nil. Matches the grammar connector_word set plus the `to` keyword.
func connectorAt(tokens []SyntaxToken, i int) *SyntaxToken {
	if i >= len(tokens) {
		return nil
	}
	tok := tokens[i]
	if tok.Kind() == SyntaxKindKwTo || isConnectorWord(tok.Text()) {
		t := tok
		return &t
	}
	return nil
}

// isAstFieldSentinel returns true when tokens[i] is a field keyword or ident followed
// by a colon — the start of a new field definition.
func isAstFieldSentinel(tokens []SyntaxToken, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	if tokens[i+1].Kind() != SyntaxKindColon {
		return false
	}
	k := tokens[i].Kind()
	return k == SyntaxKindIdent || k == SyntaxKindKwContexts || k == SyntaxKindKwDataStores ||
		k == SyntaxKindKwLanguage || k == SyntaxKindKwDeployment
}

// serviceFieldName returns the field name string for a token that is a service
// body field keyword (contexts, data-stores, language, deployment) or a plain
// ident. Returns "" for other token kinds.
func serviceFieldName(tok SyntaxToken) string {
	switch tok.Kind() {
	case SyntaxKindKwContexts:
		return "contexts"
	case SyntaxKindKwDataStores:
		return "data-stores"
	case SyntaxKindKwLanguage:
		return "language"
	case SyntaxKindKwDeployment:
		return "deployment"
	case SyntaxKindIdent:
		return tok.Text()
	}
	return ""
}

// collectAstIdentList collects comma-separated ident/string values from tokens[i].
// Stops at a field sentinel (ident+colon), RBrace, or non-ident/string.
// Returned `lines` slice is filled with zeros — token line positions now require
// a LineIndex (Task 10). Callers that need real line numbers must compute from
// tok.Offset() + LineIndex.LineCol.
func collectAstIdentList(tokens []SyntaxToken, i int) (names []string, lines []int, newI int) {
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if tok.Kind() == SyntaxKindComma {
			i++
			continue
		}
		if (tok.Kind() == SyntaxKindIdent || tok.Kind() == SyntaxKindString) && !isAstFieldSentinel(tokens, i) {
			names = append(names, tok.Text())
			lines = append(lines, 0)
			i++
		} else {
			break
		}
	}
	return names, lines, i
}

// scanBodyTokens returns tokens inside the first LBrace…RBrace pair of a node.
func scanBodyTokens(node SyntaxNode) []SyntaxToken {
	all := node.Tokens()
	for i, tok := range all {
		if tok.Kind() == SyntaxKindLBrace {
			return all[i+1:]
		}
	}
	return nil
}

// AsFile wraps a SyntaxKindFile node as a typed File view.
func AsFile(node SyntaxNode) File { return File{node: node} }

// File is a typed view over a SyntaxKindFile node.
type File struct{ node SyntaxNode }

// isZero reports whether the file view wraps a zero/empty SyntaxNode.
func (f File) isZero() bool { return f.node == (SyntaxNode{}) }

// Actors returns all ActorDecl views — both standalone and those inside actors{} blocks,
// in document order.
func (f File) Actors() []ActorDecl {
	if f.isZero() {
		return nil
	}
	var result []ActorDecl
	for _, child := range f.node.Children() {
		c, ok := child.(SyntaxNode)
		if !ok {
			continue
		}
		switch c.Kind() {
		case SyntaxKindActorDecl:
			result = append(result, ActorDecl{node: c})
		case SyntaxKindActorsBlock:
			for _, n := range c.ChildNodes(SyntaxKindActorDecl) {
				result = append(result, ActorDecl{node: n})
			}
		}
	}
	return result
}

// Domains returns all DomainDecl views — both standalone and those inside domains{} blocks,
// in document order.
func (f File) Domains() []DomainDecl {
	if f.isZero() {
		return nil
	}
	var result []DomainDecl
	for _, child := range f.node.Children() {
		c, ok := child.(SyntaxNode)
		if !ok {
			continue
		}
		switch c.Kind() {
		case SyntaxKindDomainDecl:
			result = append(result, DomainDecl{node: c})
		case SyntaxKindDomainsBlock:
			for _, n := range c.ChildNodes(SyntaxKindDomainDecl) {
				result = append(result, DomainDecl{node: n})
			}
		}
	}
	return result
}

// Services returns all ServiceDecl views — both standalone and those inside services{} blocks,
// in document order.
func (f File) Services() []ServiceDecl {
	if f.isZero() {
		return nil
	}
	var result []ServiceDecl
	for _, child := range f.node.Children() {
		c, ok := child.(SyntaxNode)
		if !ok {
			continue
		}
		switch c.Kind() {
		case SyntaxKindServiceDecl:
			result = append(result, ServiceDecl{node: c})
		case SyntaxKindServicesBlock:
			for _, n := range c.ChildNodes(SyntaxKindServiceDecl) {
				result = append(result, ServiceDecl{node: n})
			}
		}
	}
	return result
}

// UseCases returns all UseCaseDecl views.
func (f File) UseCases() []UseCaseDecl {
	if f.isZero() {
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
	if f.isZero() {
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
	if f.isZero() {
		return nil
	}
	var result []ExposureDecl
	for _, n := range f.node.ChildNodes(SyntaxKindExposureDecl) {
		result = append(result, ExposureDecl{node: n})
	}
	return result
}

// ActorBlocks returns all top-level actors{} block views in document order.
func (f File) ActorBlocks() []ActorsBlock {
	if f.isZero() {
		return nil
	}
	var result []ActorsBlock
	for _, n := range f.node.ChildNodes(SyntaxKindActorsBlock) {
		result = append(result, ActorsBlock{node: n})
	}
	return result
}

// ActorDecl is a typed view over a SyntaxKindActorDecl node.
type ActorDecl struct{ node SyntaxNode }

// Keyword returns the 'actor' keyword token (present on standalone actor declarations).
// Returns nil for actors inside an actors{} block (which have no 'actor' keyword).
func (a ActorDecl) Keyword() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindKwActor)
}

// ActorType returns the user/system/service type token.
func (a ActorDecl) ActorType() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindKwUser, SyntaxKindKwSystem, SyntaxKindKwService)
}

// ActorTypeValue returns the actor type as a string, handling both keyword types
// (user/system/service) and open-taxonomy ident types (e.g. "actor partner PaymentGateway").
func (a ActorDecl) ActorTypeValue() string {
	if tok := a.ActorType(); tok != nil {
		return tok.Text()
	}
	// Open-taxonomy: first ident is type, second is name.
	tokens := a.node.Tokens()
	for i, tok := range tokens {
		if tok.Kind() == SyntaxKindIdent {
			if i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindIdent {
				return tok.Text()
			}
			break
		}
	}
	return ""
}

// ActorTypeToken returns the token for the actor's open-taxonomy type ident, or nil.
// Returns nil for built-in keyword types (user/system/service) since Pass 1 handles those.
func (a ActorDecl) ActorTypeToken() *SyntaxToken {
	if a.ActorType() != nil {
		return nil // keyword type already emitted by Pass 1
	}
	tokens := a.node.Tokens()
	for i, tok := range tokens {
		if tok.Kind() == SyntaxKindIdent {
			if i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindIdent {
				t := tok
				return &t
			}
			break
		}
	}
	return nil
}

// Name returns the identifier token for the actor's name.
func (a ActorDecl) Name() *SyntaxToken {
	return a.node.ChildToken(SyntaxKindIdent)
}

// Line returns the 1-based source line of the actor name token using li.
func (a ActorDecl) Line(li green.LineIndex) int {
	tok := a.Name()
	if tok == nil {
		return nodeFirstTokenLine(a.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}

// DomainDecl is a typed view over a SyntaxKindDomainDecl node.
type DomainDecl struct{ node SyntaxNode }

// Keyword returns the 'domain' keyword token.
func (d DomainDecl) Keyword() *SyntaxToken { return d.node.ChildToken(SyntaxKindKwDomain) }

// Name returns the identifier token for the domain's name.
func (d DomainDecl) Name() *SyntaxToken { return d.node.ChildToken(SyntaxKindIdent) }

// IsGrouped returns true when the domain was declared inside a domains { } block.
// Standalone domains begin with the `domain` keyword; grouped domains begin with their name.
func (d DomainDecl) IsGrouped() bool {
	return d.node.ChildToken(SyntaxKindKwDomain) == nil
}

// Line returns the 1-based source line of the domain name token using li.
func (d DomainDecl) Line(li green.LineIndex) int {
	tok := d.Name()
	if tok == nil {
		return nodeFirstTokenLine(d.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}

// EndLine returns the 1-based line of the closing `}` using li.
func (d DomainDecl) EndLine(li green.LineIndex) int { return nodeEndLine(d.node, li) }

// BoundedContexts returns all BoundedContext views within this domain.
func (d DomainDecl) BoundedContexts() []BoundedContext {
	var result []BoundedContext
	for _, n := range d.node.ChildNodes(SyntaxKindBoundedContext) {
		result = append(result, BoundedContext{node: n})
	}
	return result
}

// BoundedContext is a typed view over a SyntaxKindBoundedContext node.
type BoundedContext struct{ node SyntaxNode }

// Name returns the identifier token for the bounded context's name.
func (bc BoundedContext) Name() *SyntaxToken { return bc.node.ChildToken(SyntaxKindIdent) }

// RefDecl is a typed view over a SyntaxKindRef node — a name reference wrapped
// by the parser at reference sites such as contexts: field values.
type RefDecl struct{ node SyntaxNode }

// Name returns the identifier token inside this ref node.
func (r RefDecl) Name() *SyntaxToken { return r.node.ChildToken(SyntaxKindIdent) }

// ServiceField is a typed view over a SyntaxKindServiceField node.
type ServiceField struct{ node SyntaxNode }

// IsContexts reports whether this is a contexts: field.
func (sf ServiceField) IsContexts() bool {
	return sf.node.ChildToken(SyntaxKindKwContexts) != nil
}

// IsLanguage reports whether this is a language: field.
func (sf ServiceField) IsLanguage() bool {
	return sf.node.ChildToken(SyntaxKindKwLanguage) != nil
}

// IsDataStores reports whether this is a data-stores: field.
func (sf ServiceField) IsDataStores() bool {
	return sf.node.ChildToken(SyntaxKindKwDataStores) != nil
}

// IsDeployment reports whether this is a deployment: field.
func (sf ServiceField) IsDeployment() bool {
	return sf.node.ChildToken(SyntaxKindKwDeployment) != nil
}

// DomainsBlock is a typed view over a SyntaxKindDomainsBlock node.
type DomainsBlock struct{ node SyntaxNode }

// Domains returns all DomainDecl views within this block.
func (db DomainsBlock) Domains() []DomainDecl {
	var result []DomainDecl
	for _, n := range db.node.ChildNodes(SyntaxKindDomainDecl) {
		result = append(result, DomainDecl{node: n})
	}
	return result
}

// ActorsBlock is a typed view over a SyntaxKindActorsBlock node.
type ActorsBlock struct{ node SyntaxNode }

// Line returns the 1-based source line of the `actors` keyword using li.
func (b ActorsBlock) Line(li green.LineIndex) int { return nodeFirstTokenLine(b.node, li) }

// EndLine returns the 1-based line of the closing `}` using li.
func (b ActorsBlock) EndLine(li green.LineIndex) int { return nodeEndLine(b.node, li) }

// ServiceDecl is a typed view over a SyntaxKindServiceDecl node.
type ServiceDecl struct{ node SyntaxNode }

func (s ServiceDecl) Keyword() *SyntaxToken { return s.node.ChildToken(SyntaxKindKwService) }
func (s ServiceDecl) Name() *SyntaxToken    { return s.node.ChildToken(SyntaxKindIdent) }

// IsGrouped returns true when the service was declared inside a services { } block.
func (s ServiceDecl) IsGrouped() bool {
	return s.node.ChildToken(SyntaxKindKwService) == nil
}

// Line returns the 1-based source line of the service name token using li.
func (s ServiceDecl) Line(li green.LineIndex) int {
	tok := s.Name()
	if tok == nil {
		return nodeFirstTokenLine(s.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}

// EndLine returns the 1-based line of the closing `}` using li.
func (s ServiceDecl) EndLine(li green.LineIndex) int { return nodeEndLine(s.node, li) }

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
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		fieldName := serviceFieldName(tok)
		if fieldName == "" {
			i++
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].Kind() != SyntaxKindColon {
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
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				f.Language = tokens[i].Text()
				i++
			}
		case "deployment":
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				f.DeploymentType = tokens[i].Text()
				i++
			}
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindLParen {
				i++
				for i < len(tokens) && tokens[i].Kind() != SyntaxKindRParen {
					if tokens[i].Kind() != SyntaxKindPercentage {
						i++
						continue
					}
					pct := tokens[i].Text()
					i++
					if i < len(tokens) && tokens[i].Kind() == SyntaxKindArrow {
						i++
					}
					var target string
					if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
						target = tokens[i].Text()
						i++
					}
					f.DeploymentRules = append(f.DeploymentRules, struct{ Percentage, Target string }{pct, target})
					if i < len(tokens) && tokens[i].Kind() == SyntaxKindComma {
						i++
					}
				}
				if i < len(tokens) && tokens[i].Kind() == SyntaxKindRParen {
					i++
				}
			}
		default:
			for i < len(tokens) {
				if tokens[i].Kind() == SyntaxKindRBrace || serviceFieldName(tokens[i]) != "" {
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
// TODO(Task 10): rewire to LineIndex; entries are 0 in interim.
func (s ServiceDecl) ContextLines() []int { return s.parseServiceBody().ContextLines }

// ContextLinesWith returns the 1-based source line of each context name token using li.
func (s ServiceDecl) ContextLinesWith(li green.LineIndex) []int {
	toks := s.ContextTokens()
	lines := make([]int, len(toks))
	for i, tok := range toks {
		line, _ := li.LineCol(tok.Offset())
		lines[i] = line
	}
	return lines
}

// ContextTokens returns the raw SyntaxToken for each name in the contexts: list.
// Use tok.Offset() + a LineIndex to compute LSP positions for inlay hints.
func (s ServiceDecl) ContextTokens() []SyntaxToken {
	var result []SyntaxToken
	tokens := scanBodyTokens(s.node)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		fieldName := serviceFieldName(tok)
		if fieldName == "" {
			i++
			continue
		}
		if i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			isContexts := fieldName == "contexts"
			i += 2 // skip fieldName + colon
			if !isContexts {
				continue
			}
			for i < len(tokens) {
				t := tokens[i]
				if t.Kind() == SyntaxKindRBrace {
					break
				}
				if t.Kind() == SyntaxKindComma {
					i++
					continue
				}
				if (t.Kind() == SyntaxKindIdent || t.Kind() == SyntaxKindString) && !isAstFieldSentinel(tokens, i) {
					result = append(result, t)
					i++
				} else {
					break
				}
			}
			break // contexts: appears at most once
		}
		i++
	}
	return result
}

// ContextRefs returns RefDecl views for each name in the contexts: field.
// Each RefDecl wraps the name ident in a SyntaxKindRef node; call
// ref.Name().Offset() for byte-accurate position info.
func (s ServiceDecl) ContextRefs() []RefDecl {
	toks := s.ContextTokens()
	result := make([]RefDecl, 0, len(toks))
	for _, tok := range toks {
		parent := tok.Parent()
		if parent != nil && parent.Kind() == SyntaxKindRef {
			result = append(result, RefDecl{node: *parent})
		}
	}
	return result
}

// Fields returns all field declarations inside this service body.
func (s ServiceDecl) Fields() []ServiceField {
	children := s.node.ChildNodes(SyntaxKindServiceField)
	result := make([]ServiceField, len(children))
	for i, child := range children {
		result[i] = ServiceField{node: child}
	}
	return result
}

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

// DataStoreTokens returns the SyntaxToken for each data-store name in the service body.
func (s ServiceDecl) DataStoreTokens() []SyntaxToken {
	tokens := scanBodyTokens(s.node)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if serviceFieldName(tok) == "data-stores" &&
			i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			i += 2
			var result []SyntaxToken
			for i < len(tokens) {
				if tokens[i].Kind() == SyntaxKindComma {
					i++
					continue
				}
				if tokens[i].Kind() == SyntaxKindRBrace || isAstFieldSentinel(tokens, i) {
					break
				}
				if tokens[i].Kind() == SyntaxKindIdent {
					result = append(result, tokens[i])
					i++
				} else {
					break
				}
			}
			return result
		}
	}
	return nil
}

// LanguageToken returns the SyntaxToken for the language value in the service body, or nil.
func (s ServiceDecl) LanguageToken() *SyntaxToken {
	tokens := scanBodyTokens(s.node)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if serviceFieldName(tok) == "language" &&
			i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			i += 2
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				t := tokens[i]
				return &t
			}
			return nil
		}
	}
	return nil
}

// DeploymentTypeToken returns the SyntaxToken for the deployment type (e.g. "canary"), or nil.
func (s ServiceDecl) DeploymentTypeToken() *SyntaxToken {
	tokens := scanBodyTokens(s.node)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if serviceFieldName(tok) == "deployment" &&
			i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			i += 2
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				t := tokens[i]
				return &t
			}
			return nil
		}
	}
	return nil
}

// DeploymentTargetTokens returns the SyntaxToken for each deployment rule target.
func (s ServiceDecl) DeploymentTargetTokens() []SyntaxToken {
	tokens := scanBodyTokens(s.node)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if serviceFieldName(tok) == "deployment" &&
			i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			i += 2
			// Skip deployment type ident.
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				i++
			}
			// Enter parenthesised rule list.
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindLParen {
				i++
			}
			var result []SyntaxToken
			for i < len(tokens) && tokens[i].Kind() != SyntaxKindRParen && tokens[i].Kind() != SyntaxKindRBrace {
				// Each rule: <percentage> -> <target>
				if tokens[i].Kind() == SyntaxKindArrow {
					i++
					if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
						result = append(result, tokens[i])
					}
				}
				i++
			}
			return result
		}
	}
	return nil
}

// AsUseCaseDecl wraps a SyntaxKindUseCaseDecl node as a typed view.
func AsUseCaseDecl(n SyntaxNode) UseCaseDecl { return UseCaseDecl{node: n} }

// UseCaseDecl is a typed view over a SyntaxKindUseCaseDecl node.
type UseCaseDecl struct{ node SyntaxNode }

func (u UseCaseDecl) Keyword() *SyntaxToken { return u.node.ChildToken(SyntaxKindKwUseCase) }
func (u UseCaseDecl) Title() *SyntaxToken   { return u.node.ChildToken(SyntaxKindString) }

// EndLine returns the 1-based line of the closing `}` using li.
func (u UseCaseDecl) EndLine(li green.LineIndex) int { return nodeEndLine(u.node, li) }

// Line returns the 1-based source line of the `use_case` keyword using li.
func (u UseCaseDecl) Line(li green.LineIndex) int {
	tok := u.Keyword()
	if tok == nil {
		return nodeFirstTokenLine(u.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
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
type ScenarioDecl struct{ node SyntaxNode }

// When returns the 'when' keyword token that starts this scenario.
func (s ScenarioDecl) When() *SyntaxToken { return s.node.ChildToken(SyntaxKindKwWhen) }

// Trigger returns the trigger sub-node of this scenario.
func (s ScenarioDecl) Trigger() TriggerDecl {
	if n := s.node.ChildNode(SyntaxKindTrigger); n != nil {
		return TriggerDecl{node: *n}
	}
	return TriggerDecl{}
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
type TriggerDecl struct{ node SyntaxNode }

// Subject returns the subject identifier token of the trigger (actor/domain name).
func (t TriggerDecl) Subject() *SyntaxToken { return t.node.ChildToken(SyntaxKindIdent) }

// Tokens returns all syntax tokens in the trigger node (mirrors ActionDecl.Tokens).
func (t TriggerDecl) Tokens() []SyntaxToken { return t.node.Tokens() }

// Event returns the string token for event-style triggers (when "<EventName>").
func (t TriggerDecl) Event() *SyntaxToken { return t.node.ChildToken(SyntaxKindString) }

// Kind returns "external", "event", or "domain_listen" based on token structure.
// Mirrors lowerTrigger classification in lower.go.
func (t TriggerDecl) Kind() string {
	tokens := t.node.Tokens()
	if len(tokens) == 0 {
		return "external"
	}
	// event trigger: first token is a string literal
	if tokens[0].Kind() == SyntaxKindString {
		return "event"
	}
	// domain_listen: second token is `listens`
	if len(tokens) >= 2 && tokens[1].Kind() == SyntaxKindKwListens {
		return "domain_listen"
	}
	return "external"
}

// ActorName returns the trigger subject for external triggers (the actor/domain name).
func (t TriggerDecl) ActorName() string {
	if t.Kind() != "external" {
		return ""
	}
	tok := t.node.ChildToken(SyntaxKindIdent)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// ActorCol returns the 1-based column of the subject token using li.
// Works for both external and domain_listen triggers.
func (t TriggerDecl) ActorCol(li green.LineIndex) int {
	tok := t.node.ChildToken(SyntaxKindIdent)
	if tok == nil {
		return 0
	}
	_, col := li.LineCol(tok.Offset())
	return col
}

// ContextName returns the subject name for domain_listen triggers.
func (t TriggerDecl) ContextName() string {
	if t.Kind() != "domain_listen" {
		return ""
	}
	tok := t.node.ChildToken(SyntaxKindIdent)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// EventValue returns the event name (for event and domain_listen triggers).
func (t TriggerDecl) EventValue() string {
	tokens := t.node.Tokens()
	switch t.Kind() {
	case "event":
		if len(tokens) > 0 {
			return tokens[0].Text()
		}
	case "domain_listen":
		if len(tokens) >= 3 {
			return tokens[2].Text()
		}
	}
	return ""
}

// Description returns the human-readable trigger line (without the leading `when`).
// String tokens are wrapped in double quotes.
func (t TriggerDecl) Description() string {
	tokens := t.node.Tokens()
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Kind() == SyntaxKindString {
			parts = append(parts, `"`+tok.Text()+`"`)
		} else {
			parts = append(parts, tok.Text())
		}
	}
	return strings.Join(parts, " ")
}

// EventCol returns the 1-based column of the event token using li.
func (t TriggerDecl) EventCol(li green.LineIndex) int {
	tokens := t.node.Tokens()
	switch t.Kind() {
	case "event":
		if len(tokens) > 0 {
			_, col := li.LineCol(tokens[0].Offset())
			return col
		}
	case "domain_listen":
		if len(tokens) >= 3 {
			_, col := li.LineCol(tokens[2].Offset())
			return col
		}
	}
	return 0
}

// EventIsString returns true when the event token is a quoted string literal.
func (t TriggerDecl) EventIsString() bool {
	tokens := t.node.Tokens()
	switch t.Kind() {
	case "event":
		return len(tokens) > 0 && tokens[0].Kind() == SyntaxKindString
	case "domain_listen":
		return len(tokens) >= 3 && tokens[2].Kind() == SyntaxKindString
	}
	return false
}

// ActionDecl is a typed view over a SyntaxKindAction node.
type ActionDecl struct{ node SyntaxNode }

// Subject returns the subject identifier token (domain/service name).
func (a ActionDecl) Subject() *SyntaxToken { return a.node.ChildToken(SyntaxKindIdent) }

// Verb returns the action verb token (asks/notifies/listens/returns).
func (a ActionDecl) Verb() *SyntaxToken {
	return a.node.ChildToken(
		SyntaxKindKwAsks, SyntaxKindKwNotifies,
		SyntaxKindKwListens, SyntaxKindKwReturns,
	)
}

// Connector returns the connector token between verb and phrase, or nil.
// For sync_action: tokens[3] (after subject, asks, target).
// For internal_action: tokens[2] (after subject, verb).
// For return_action: the KwTo token (different positional semantics — `to <target>`).
func (a ActionDecl) Connector() *SyntaxToken {
	tokens := a.node.Tokens()
	switch a.Kind() {
	case "sync_action":
		return connectorAt(tokens, 3)
	case "internal_action":
		return connectorAt(tokens, 2)
	default:
		return a.node.ChildToken(SyntaxKindKwTo)
	}
}

// Kind returns "sync_action", "async_action", "return_action", or "internal_action".
// Mirrors lowerAction classification in lower.go.
func (a ActionDecl) Kind() string {
	verb := a.Verb()
	if verb == nil {
		return "internal_action"
	}
	switch verb.Kind() {
	case SyntaxKindKwAsks:
		return "sync_action"
	case SyntaxKindKwNotifies:
		return "async_action"
	case SyntaxKindKwReturns:
		return "return_action"
	default:
		return "internal_action"
	}
}

// SubjectName returns the action subject (the "from" party).
func (a ActionDecl) SubjectName() string {
	tok := a.Subject()
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// SubjectCol returns the 1-based column of the subject token using li.
func (a ActionDecl) SubjectCol(li green.LineIndex) int {
	tok := a.Subject()
	if tok == nil {
		return 0
	}
	_, col := li.LineCol(tok.Offset())
	return col
}

// Line returns the 1-based source line of the action subject token using li.
func (a ActionDecl) Line(li green.LineIndex) int {
	tok := a.Subject()
	if tok == nil {
		return nodeFirstTokenLine(a.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}

// TargetName returns the target context for sync_action and return_action.
func (a ActionDecl) TargetName() string {
	tokens := a.node.Tokens()
	switch a.Kind() {
	case "sync_action":
		if len(tokens) >= 3 && tokens[2].Kind() == SyntaxKindIdent {
			return tokens[2].Text()
		}
	case "return_action":
		for i, tok := range tokens {
			if tok.Kind() == SyntaxKindKwTo && i+1 < len(tokens) {
				return tokens[i+1].Text()
			}
		}
	}
	return ""
}

// TargetCol returns the 1-based column of the target name token using li.
func (a ActionDecl) TargetCol(li green.LineIndex) int {
	tokens := a.node.Tokens()
	switch a.Kind() {
	case "sync_action":
		if len(tokens) >= 3 && tokens[2].Kind() == SyntaxKindIdent {
			_, col := li.LineCol(tokens[2].Offset())
			return col
		}
	case "return_action":
		for i, tok := range tokens {
			if tok.Kind() == SyntaxKindKwTo && i+1 < len(tokens) {
				_, col := li.LineCol(tokens[i+1].Offset())
				return col
			}
		}
	}
	return 0
}

// EventValue returns the event name for async_action (notifies).
func (a ActionDecl) EventValue() string {
	if a.Kind() != "async_action" {
		return ""
	}
	tokens := a.node.Tokens()
	if len(tokens) >= 3 {
		return tokens[2].Text()
	}
	return ""
}

// EventCol returns the 1-based column of the event token for async_action using li.
func (a ActionDecl) EventCol(li green.LineIndex) int {
	if a.Kind() != "async_action" {
		return 0
	}
	tokens := a.node.Tokens()
	if len(tokens) >= 3 {
		_, col := li.LineCol(tokens[2].Offset())
		return col
	}
	return 0
}

// EventIsString returns true when the event was a quoted string literal.
func (a ActionDecl) EventIsString() bool {
	if a.Kind() != "async_action" {
		return false
	}
	tokens := a.node.Tokens()
	return len(tokens) >= 3 && tokens[2].Kind() == SyntaxKindString
}

// VerbValue returns the verb text.
func (a ActionDecl) VerbValue() string {
	tok := a.Verb()
	if tok != nil {
		return tok.Text()
	}
	// internal_action: verb is the ident at tokens[1]
	tokens := a.node.Tokens()
	if len(tokens) >= 2 {
		return tokens[1].Text()
	}
	return ""
}

// ConnectorValue returns the connector word text (e.g. "to", "for"), or empty.
func (a ActionDecl) ConnectorValue() string {
	tok := a.Connector()
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// phraseStartIndex returns the index into tokens (a.node.Tokens(), i.e.
// non-trivia tokens) at which the free-text phrase begins for this action's
// kind, skipping subject/verb/target/connector tokens. Returns len(tokens)
// for action kinds that have no phrase (async_action).
func phraseStartIndex(a ActionDecl, tokens []SyntaxToken) int {
	start := 2
	switch a.Kind() {
	case "sync_action":
		start = 3 // subject, asks, target
		if connectorAt(tokens, start) != nil {
			start++ // skip connector
		}
	case "return_action":
		start = 2
		if start < len(tokens) && tokens[start].Kind() == SyntaxKindKwTo {
			start += 2 // skip `to target`
		}
		if connectorAt(tokens, start) != nil {
			start++
		}
	case "internal_action":
		start = 2 // subject, verb
		if connectorAt(tokens, start) != nil {
			start++
		}
	case "async_action":
		return len(tokens) // no phrase for notifies
	}
	return start
}

// PhraseText returns the description phrase for the action, as the exact raw
// source substring spanning the phrase tokens (including any inter-token
// whitespace), so special characters like `1! & 2! and/maybe *` keep their
// original spacing instead of being space-joined.
//
// The green tree is lossless but does not retain a pointer to the original
// source string on SyntaxNode/ActionDecl, so this reconstructs the exact
// slice from the node's own tokens (including whitespace/trivia tokens,
// which the parser emits for every gap — see parser.go emitWhitespaceBefore)
// rather than from a raw offset-based source slice.
func (a ActionDecl) PhraseText() string {
	tokens := a.node.Tokens()
	if len(tokens) == 0 {
		return ""
	}
	start := phraseStartIndex(a, tokens)
	if start >= len(tokens) {
		return ""
	}
	startOffset := tokens[start].Offset()
	var sb strings.Builder
	writing := false
	for _, tok := range a.node.AllTokens() {
		if !writing {
			if tok.Offset() < startOffset {
				continue
			}
			writing = true
		}
		sb.WriteString(tok.Text())
	}
	return strings.TrimSpace(sb.String())
}

// Tokens returns the raw token list for the action node.
func (a ActionDecl) Tokens() []SyntaxToken { return a.node.Tokens() }

// Description returns the human-readable full action line.
func (a ActionDecl) Description() string {
	subject := a.SubjectName()
	verb := a.VerbValue()
	switch a.Kind() {
	case "sync_action":
		target := a.TargetName()
		connector := a.ConnectorValue()
		phrase := a.PhraseText()
		desc := subject + " asks " + target
		if connector != "" {
			desc += " " + connector
		}
		if phrase != "" {
			desc += " " + phrase
		}
		return desc
	case "async_action":
		return fmt.Sprintf("%s notifies %q", subject, a.EventValue())
	case "return_action":
		phrase := a.PhraseText()
		target := a.TargetName()
		if target != "" {
			return fmt.Sprintf("%s returns %s to %s", subject, phrase, target)
		}
		return fmt.Sprintf("%s returns %s", subject, phrase)
	default: // internal_action
		connector := a.ConnectorValue()
		phrase := a.PhraseText()
		desc := subject + " " + verb
		if connector != "" {
			desc += " " + connector
		}
		if phrase != "" {
			desc += " " + phrase
		}
		return desc
	}
}

// ArchDecl is a typed view over a SyntaxKindArchDecl node.
type ArchDecl struct{ node SyntaxNode }

// Keyword returns the 'arch' keyword token.
func (a ArchDecl) Keyword() *SyntaxToken { return a.node.ChildToken(SyntaxKindKwArch) }

// Name returns the identifier token for the arch's name (optional).
func (a ArchDecl) Name() *SyntaxToken { return a.node.ChildToken(SyntaxKindIdent) }

// Line returns the 1-based source line of the `arch` keyword using li.
func (a ArchDecl) Line(li green.LineIndex) int { return nodeFirstTokenLine(a.node, li) }

// EndLine returns the 1-based line of the closing `}` using li.
func (a ArchDecl) EndLine(li green.LineIndex) int { return nodeEndLine(a.node, li) }

// PresentationLine returns the 1-based line of the `presentation:` label using li, or 0 if absent.
func (a ArchDecl) PresentationLine(li green.LineIndex) int {
	for _, section := range a.Sections() {
		kw := section.Keyword()
		if kw != nil && kw.Kind() == SyntaxKindKwPresentation {
			line, _ := li.LineCol(kw.Offset())
			return line
		}
	}
	return 0
}

// GatewayLine returns the 1-based line of the `gateway:` label using li, or 0 if absent.
func (a ArchDecl) GatewayLine(li green.LineIndex) int {
	for _, section := range a.Sections() {
		kw := section.Keyword()
		if kw != nil && kw.Kind() == SyntaxKindKwGateway {
			line, _ := li.LineCol(kw.Offset())
			return line
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
type ArchSection struct{ node SyntaxNode }

// isZero reports whether the section view wraps a zero/empty SyntaxNode.
func (s ArchSection) isZero() bool { return s.node == (SyntaxNode{}) }

// Keyword returns the section label keyword token (presentation or gateway).
func (s ArchSection) Keyword() *SyntaxToken {
	if s.isZero() {
		return nil
	}
	return s.node.ChildToken(SyntaxKindKwPresentation, SyntaxKindKwGateway, SyntaxKindIdent)
}

// Components returns all ArchComponent views within this section.
func (s ArchSection) Components() []ArchComponent {
	if s.isZero() {
		return nil
	}
	var result []ArchComponent
	for _, n := range s.node.ChildNodes(SyntaxKindArchComponent) {
		result = append(result, ArchComponent{node: n})
	}
	return result
}

// ArchComponent is a typed view over a SyntaxKindArchComponent node.
type ArchComponent struct{ node SyntaxNode }

// isZero reports whether the component view wraps a zero/empty SyntaxNode.
func (c ArchComponent) isZero() bool { return c.node == (SyntaxNode{}) }

// Name returns the identifier token for the component's name.
func (c ArchComponent) Name() *SyntaxToken {
	if c.isZero() {
		return nil
	}
	return c.node.ChildToken(SyntaxKindIdent)
}

// Modifiers returns all ArchModifier views within this component.
func (c ArchComponent) Modifiers() []ArchModifier {
	if c.isZero() {
		return nil
	}
	var result []ArchModifier
	for _, n := range c.node.ChildNodes(SyntaxKindArchModifier) {
		result = append(result, ArchModifier{node: n})
	}
	return result
}

// ArchModifier is a typed view over a SyntaxKindArchModifier node.
type ArchModifier struct{ node SyntaxNode }

// isZero reports whether the modifier view wraps a zero/empty SyntaxNode.
func (m ArchModifier) isZero() bool { return m.node == (SyntaxNode{}) }

// Key returns the identifier token for the modifier key.
func (m ArchModifier) Key() *SyntaxToken {
	if m.isZero() {
		return nil
	}
	return m.node.ChildToken(SyntaxKindIdent)
}

// Value returns the value token for the modifier (ident, string, or number).
// Returns nil if the modifier has no value (key-only modifier).
func (m ArchModifier) Value() *SyntaxToken {
	if m.isZero() {
		return nil
	}
	// The modifier node children are: Ident (key), optional Colon, optional value token.
	// The value token follows the Colon and may be Ident, String, or Number.
	colonSeen := false
	for _, child := range m.node.Children() {
		tok, ok := child.(SyntaxToken)
		if !ok {
			continue
		}
		if tok.Kind() == SyntaxKindColon {
			colonSeen = true
			continue
		}
		if colonSeen {
			t := tok
			return &t
		}
	}
	return nil
}

// ExposureDecl is a typed view over a SyntaxKindExposureDecl node.
type ExposureDecl struct{ node SyntaxNode }

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
func collectAstExposureIdentList(tokens []SyntaxToken, i int) ([]string, int) {
	var names []string
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		if tok.Kind() == SyntaxKindComma {
			i++
			continue
		}
		isFieldKw := tok.Kind() == SyntaxKindKwTo || tok.Kind() == SyntaxKindKwContexts ||
			tok.Kind() == SyntaxKindKwThrough || tok.Kind() == SyntaxKindIdent
		if isFieldKw && i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
			break
		}
		if tok.Kind() == SyntaxKindIdent || tok.Kind() == SyntaxKindString {
			names = append(names, tok.Text())
		}
		i++
	}
	return names, i
}

func (e ExposureDecl) parseExposureBody() exposureBodyFields {
	var f exposureBodyFields
	tokens := scanBodyTokens(e.node)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Kind() == SyntaxKindRBrace {
			break
		}
		var fieldName string
		switch tok.Kind() {
		case SyntaxKindKwTo:
			fieldName = "to"
		case SyntaxKindKwContexts:
			fieldName = "contexts"
		case SyntaxKindKwThrough:
			fieldName = "through"
		case SyntaxKindIdent:
			fieldName = tok.Text()
		default:
			i++
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].Kind() != SyntaxKindColon {
			i++
			continue
		}
		i += 2
		var names []string
		names, i = collectAstExposureIdentList(tokens, i)
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

// Line returns the 1-based source line of the `exposure` keyword using li.
func (e ExposureDecl) Line(li green.LineIndex) int { return nodeFirstTokenLine(e.node, li) }

// EndLine returns the 1-based line of the closing `}` using li.
func (e ExposureDecl) EndLine(li green.LineIndex) int { return nodeEndLine(e.node, li) }

// To returns the `to:` target names.
func (e ExposureDecl) To() []string { return e.parseExposureBody().To }

// Contexts returns the `contexts:` names.
func (e ExposureDecl) Contexts() []string { return e.parseExposureBody().Contexts }

// Through returns the `through:` names.
func (e ExposureDecl) Through() []string { return e.parseExposureBody().Through }

// exposureFieldTokens returns the ident tokens for a named exposure field.
// kind must be the token kind used as the field keyword
// (SyntaxKindKwTo, SyntaxKindKwThrough, SyntaxKindKwContexts).
func (e ExposureDecl) exposureFieldTokens(kind SyntaxKind) []SyntaxToken {
	tokens := scanBodyTokens(e.node)
	for i, tok := range tokens {
		if tok.Kind() != kind {
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].Kind() != SyntaxKindColon {
			continue
		}
		i += 2
		var result []SyntaxToken
		for i < len(tokens) {
			t := tokens[i]
			if t.Kind() == SyntaxKindRBrace {
				break
			}
			if t.Kind() == SyntaxKindComma {
				i++
				continue
			}
			// Another field keyword followed by colon = new field.
			if i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon {
				break
			}
			if t.Kind() == SyntaxKindIdent || t.Kind() == SyntaxKindString {
				result = append(result, t)
			}
			i++
		}
		return result
	}
	return nil
}

// ToTokens returns the ident tokens for the `to:` field values.
func (e ExposureDecl) ToTokens() []SyntaxToken { return e.exposureFieldTokens(SyntaxKindKwTo) }

// ThroughTokens returns the ident tokens for the `through:` field values.
func (e ExposureDecl) ThroughTokens() []SyntaxToken {
	return e.exposureFieldTokens(SyntaxKindKwThrough)
}

// ContextsTokens returns the ident tokens for the `contexts:` field values.
func (e ExposureDecl) ContextsTokens() []SyntaxToken {
	return e.exposureFieldTokens(SyntaxKindKwContexts)
}

// DeploymentRule is a typed view over a SyntaxKindDeploymentRule node.
// In exposure blocks this wraps the 'through: <value>' clause.
type DeploymentRule struct{ node SyntaxNode }

// isZero reports whether the rule view wraps a zero/empty SyntaxNode.
func (r DeploymentRule) isZero() bool { return r.node == (SyntaxNode{}) }

// Through returns the 'through' keyword token.
func (r DeploymentRule) Through() *SyntaxToken {
	if r.isZero() {
		return nil
	}
	return r.node.ChildToken(SyntaxKindKwThrough)
}

// Arrow returns the '->' token (present in service deployment rules).
func (r DeploymentRule) Arrow() *SyntaxToken {
	if r.isZero() {
		return nil
	}
	return r.node.ChildToken(SyntaxKindArrow)
}
