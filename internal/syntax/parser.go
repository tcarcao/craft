// Package syntax implements the hand-written recursive-descent parser for the
// Craft DSL. It produces an internal/ast.File.
//
// S3: actors. S4: domains. S5: services + services block.
// S6: use_case "..." { when ... } blocks.
// S7: arch { presentation: ... gateway: ... } blocks with flow (>) and
//     component modifiers ([key, key:value]).
// S8: exposure <name> { to: ... contexts: ... through: ... } blocks.
// Unsupported top-level keywords emit a recoverable "not-yet-implemented"
// diagnostic so --parser=v2 is usable on partial files.
package syntax

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/lexer"
	"github.com/tcarcao/craft/pkg/craft"
)

// Parser is a recursive-descent parser for Craft DSL.
type Parser struct {
	tokens []lexer.Token
	pos    int
}

// Parse parses the given source text, returning an AST and any diagnostics.
// A non-nil File is always returned even when diagnostics are present (island
// parsing: each top-level block is parsed independently).
func Parse(src string) (*ast.File, []craft.Diagnostic) {
	l := lexer.New(src)
	p := &Parser{tokens: l.All()}
	return p.parseFile()
}

// --- main parse loop ---

func (p *Parser) parseFile() (*ast.File, []craft.Diagnostic) {
	file := &ast.File{}
	var diags []craft.Diagnostic

	// Global counter for scenario_N / action_N IDs across all use_cases in the file,
	// matching ANTLR's numbering scheme.
	ucCounter := 0

	for !p.atEOF() {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenKwActor:
			actor, d := p.parseActorStatement()
			diags = append(diags, d...)
			if actor != nil {
				file.Actors = append(file.Actors, actor)
			}
		case lexer.TokenKwActors:
			actors, blockRange, d := p.parseActorsBlock()
			diags = append(diags, d...)
			file.Actors = append(file.Actors, actors...)
			if blockRange != nil {
				file.ActorBlocks = append(file.ActorBlocks, blockRange)
			}
		case lexer.TokenKwDomain:
			domain, d := p.parseDomainStatement()
			diags = append(diags, d...)
			if domain != nil {
				file.Domains = append(file.Domains, domain)
			}
		case lexer.TokenKwDomains:
			domains, d := p.parseDomainsBlock()
			diags = append(diags, d...)
			file.Domains = append(file.Domains, domains...)
		case lexer.TokenKwService:
			svc, d := p.parseServiceStatement()
			diags = append(diags, d...)
			if svc != nil {
				file.Services = append(file.Services, svc)
			}
		case lexer.TokenKwServices:
			services, d := p.parseServicesBlock()
			diags = append(diags, d...)
			file.Services = append(file.Services, services...)
		case lexer.TokenKwUseCase:
			uc, d := p.parseUseCaseBlock(&ucCounter)
			diags = append(diags, d...)
			if uc != nil {
				file.UseCases = append(file.UseCases, uc)
			}
		case lexer.TokenKwArch:
			arch, d := p.parseArchBlock()
			diags = append(diags, d...)
			if arch != nil {
				file.Archs = append(file.Archs, arch)
			}
		case lexer.TokenKwExposure:
			exp, d := p.parseExposureBlock()
			diags = append(diags, d...)
			if exp != nil {
				file.Exposures = append(file.Exposures, exp)
			}
		default:
			// Unrecognised top-level token: emit a diagnostic and resync to
			// the next top-level keyword (island parsing).
			diags = append(diags, p.diagNotImplemented(tok))
			p.resyncToTopLevel()
		}
	}
	return file, diags
}

// parseActorStatement parses: actor <type> <name>
func (p *Parser) parseActorStatement() (*ast.ActorDecl, []craft.Diagnostic) {
	p.consume() // consume `actor`
	var diags []craft.Diagnostic

	typeTok := p.peek()
	at, ok := tokenToActorType(typeTok)
	if !ok {
		diags = append(diags, p.diagUnexpected(typeTok, "actor type"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume()

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent {
		diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume()

	// Individual `actor` statements do not carry a source line in the golden
	// contract (ANTLR VisitActor_def does not set Line). Match that behaviour
	// so Harness A passes without a CraftDoc schema change.
	return &ast.ActorDecl{
		Name: nameTok.Value,
		Type: at,
		Line: 0,
	}, diags
}

// parseActorsBlock parses: actors { <actor_definition>* }
func (p *Parser) parseActorsBlock() ([]*ast.ActorDecl, *ast.ActorBlockRange, []craft.Diagnostic) {
	actorsTok := p.consume() // consume `actors`, capture line
	var diags []craft.Diagnostic
	blockRange := &ast.ActorBlockRange{Line: actorsTok.Line}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, nil, diags
	}
	p.consume() // consume `{`

	var actors []*ast.ActorDecl
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		at, ok := tokenToActorType(tok)
		if !ok {
			if tok.Type == lexer.TokenError {
				diags = append(diags, p.diagUnexpected(tok, "actor type"))
				p.consume()
				continue
			}
			// Unknown token inside block — skip to avoid infinite loop.
			diags = append(diags, p.diagUnexpected(tok, "actor type"))
			p.consume()
			continue
		}
		p.consume() // consume type

		nameTok := p.peek()
		if nameTok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
			p.consume()
			continue
		}
		p.consume() // consume name

		actors = append(actors, &ast.ActorDecl{
			Name:   nameTok.Value,
			Type:   at,
			Line:   nameTok.Line,
			Column: nameTok.Column,
		})
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed actors block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		return actors, nil, diags // no block range on unclosed block
	}
	blockRange.EndLine = p.peek().Line // capture `}` line
	p.consume()                        // consume `}`
	return actors, blockRange, diags
}

// parseDomainStatement parses: domain <name> { <bounded_context>* }
func (p *Parser) parseDomainStatement() (*ast.DomainDecl, []craft.Diagnostic) {
	p.consume() // consume `domain`
	var diags []craft.Diagnostic

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isDomainNameToken(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "domain name"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume()

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

	contexts, d := p.parseBoundedContextList()
	diags = append(diags, d...)

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domain block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(nameTok),
		})
		return &ast.DomainDecl{
			Name:            nameTok.Value,
			BoundedContexts: contexts,
			Line:            nameTok.Line,
			Column:          nameTok.Column,
		}, diags
	}
	endLine := p.peek().Line // capture `}` line
	p.consume()              // consume `}`

	return &ast.DomainDecl{
		Name:            nameTok.Value,
		BoundedContexts: contexts,
		Line:            nameTok.Line,
		Column:          nameTok.Column,
		EndLine:         endLine,
	}, diags
}

// parseDomainsBlock parses: domains { <domain_block>* }
// where each domain_block is: <name> { <bounded_context>* }
func (p *Parser) parseDomainsBlock() ([]*ast.DomainDecl, []craft.Diagnostic) {
	p.consume() // consume `domains`
	var diags []craft.Diagnostic
	var domains []*ast.DomainDecl

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type != lexer.TokenIdent && !isDomainNameToken(tok.Type) {
			diags = append(diags, p.diagUnexpected(tok, "domain name"))
			p.consume()
			continue
		}
		nameTok := tok
		p.consume()

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			continue
		}
		p.consume() // consume `{`

		contexts, d := p.parseBoundedContextList()
		diags = append(diags, d...)

		if p.atEOF() {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  "unclosed domain block (missing `}`)",
				Severity: craft.SeverityError,
				Range:    tokenRange(nameTok),
			})
			domains = append(domains, &ast.DomainDecl{
				Name:            nameTok.Value,
				BoundedContexts: contexts,
				Line:            nameTok.Line,
				Column:          nameTok.Column,
				IsGrouped:       true,
			})
			return domains, diags
		}
		endLine := p.peek().Line // capture inner `}` line
		p.consume()              // consume `}`

		domains = append(domains, &ast.DomainDecl{
			Name:            nameTok.Value,
			BoundedContexts: contexts,
			Line:            nameTok.Line,
			Column:          nameTok.Column,
			EndLine:         endLine,
			IsGrouped:       true,
		})
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domains block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		return domains, diags
	}
	p.consume() // consume `}`
	return domains, diags
}

// parseBoundedContextList parses a list of identifiers until `}` or EOF.
// These are the bounded context names inside a domain block.
// Duplicates are silently deduplicated (keeping first occurrence), matching
// ANTLR behavior and the v1 spec.
func (p *Parser) parseBoundedContextList() ([]ast.BoundedContextEntry, []craft.Diagnostic) {
	var contexts []ast.BoundedContextEntry
	seen := make(map[string]bool)
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type == lexer.TokenIdent || isDomainNameToken(tok.Type) {
			if !seen[tok.Value] {
				seen[tok.Value] = true
				contexts = append(contexts, ast.BoundedContextEntry{
					Name:   tok.Value,
					Line:   tok.Line,
					Column: tok.Column,
				})
			}
			p.consume()
		} else if tok.Type == lexer.TokenError {
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			p.consume()
		} else {
			// Unknown token inside domain block — could be a sub-keyword; skip.
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			p.consume()
		}
	}
	return contexts, diags
}

// isDomainNameToken returns true for token types that can legally appear as a
// domain name or bounded context name (keywords that are also valid identifiers
// in context — e.g. `domain User { ... }` where User is also an actor type).
func isDomainNameToken(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwUser, lexer.TokenKwSystem, lexer.TokenKwService,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwServices:
		return true
	}
	return false
}

// parseServicesBlock parses: services { <service_block>* }
// Each service_block is: <name> { <field>* } where name is an ident,
// hyphenated-ident, or quoted string.
func (p *Parser) parseServicesBlock() ([]*ast.ServiceDecl, []craft.Diagnostic) {
	p.consume() // consume `services`
	var diags []craft.Diagnostic
	var services []*ast.ServiceDecl

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume outer `{`

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// Service name: identifier, string literal, or keyword-as-name
		var name string
		var nameLine, nameCol int
		switch tok.Type {
		case lexer.TokenIdent, lexer.TokenString:
			name = tok.Value
			nameLine = tok.Line
			nameCol = tok.Column
			p.consume()
		default:
			if isServiceNameKeyword(tok.Type) {
				name = tok.Value
				nameLine = tok.Line
				nameCol = tok.Column
				p.consume()
			} else {
				diags = append(diags, p.diagUnexpected(tok, "service name"))
				p.consume()
				continue
			}
		}

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			continue
		}
		p.consume() // consume inner `{`

		svc, d := p.parseServiceBody(name, nameLine, nameCol)
		diags = append(diags, d...)
		svc.IsGrouped = true
		services = append(services, svc)

		if p.atEOF() {
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: ast.LineToLSP(nameLine)},
					End:   craft.Position{Line: ast.LineToLSP(nameLine)},
				},
			})
			return services, diags
		}
		svc.EndLine = p.peek().Line // record `}` line before consuming
		p.consume()                 // consume inner `}`
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed services block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		return services, diags
	}
	p.consume() // consume outer `}`
	return services, diags
}

// parseServiceStatement parses: service <name> { <field>* }
// This is the singular top-level service form (Q11).
func (p *Parser) parseServiceStatement() (*ast.ServiceDecl, []craft.Diagnostic) {
	p.consume() // consume `service`
	var diags []craft.Diagnostic

	nameTok := p.peek()
	var name string
	var nameLine, nameCol int
	if nameTok.Type == lexer.TokenIdent || nameTok.Type == lexer.TokenString {
		name = nameTok.Value
		nameLine = nameTok.Line
		nameCol = nameTok.Column
		p.consume()
	} else {
		diags = append(diags, p.diagUnexpected(nameTok, "service name"))
		p.resyncToTopLevel()
		return nil, diags
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume '{'

	svc, d := p.parseServiceBody(name, nameLine, nameCol)
	diags = append(diags, d...)

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
			Severity: craft.SeverityError,
			Range: craft.Range{
				Start: craft.Position{Line: ast.LineToLSP(nameLine)},
				End:   craft.Position{Line: ast.LineToLSP(nameLine)},
			},
		})
		return svc, diags
	}
	svc.EndLine = p.peek().Line // record `}` line before consuming
	p.consume()                 // consume '}'
	return svc, diags
}

// parseServiceBody parses the fields inside a service { ... } block.
func (p *Parser) parseServiceBody(name string, nameLine, nameCol int) (*ast.ServiceDecl, []craft.Diagnostic) {
	svc := &ast.ServiceDecl{Name: name, Line: nameLine, Column: nameCol}
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// Field names are contextual keywords: contexts, data-stores, language.
		// They lex as TokenIdent (or hyphenated ident). We match on Value.
		if tok.Type != lexer.TokenIdent {
			// Unknown or error token inside service body — skip.
			diags = append(diags, p.diagUnexpected(tok, "field name (contexts, data-stores, language) or `}`"))
			p.consume()
			continue
		}

		fieldName := tok.Value
		p.consume() // consume field name

		// Expect colon after field name.
		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			continue
		}
		p.consume() // consume `:`

		switch fieldName {
		case "contexts":
			svc.Contexts, svc.ContextLines = p.parseIdentListWithLines()
		case "data-stores":
			svc.DataStores = p.parseIdentList()
		case "language":
			if p.peek().Type == lexer.TokenIdent {
				svc.Language = p.peek().Value
				p.consume()
			} else {
				diags = append(diags, p.diagUnexpected(p.peek(), "language identifier"))
			}
		case "deployment":
			dt, rules, dd := p.parseDeploymentSpec()
			diags = append(diags, dd...)
			svc.DeploymentType = dt
			svc.DeploymentRules = rules
		default:
			// Unknown field — skip to next line (consume until next ident that
			// could be a field name, or `}`).
			p.skipToNextField()
		}
	}

	return svc, diags
}

// parseDeploymentSpec parses: deployment_type ('(' deployment_rule (',' deployment_rule)* ')')?
func (p *Parser) parseDeploymentSpec() (string, []ast.DeploymentRule, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	typeTok := p.peek()
	var dt string
	if typeTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(typeTok.Type) {
		dt = typeTok.Value
		p.consume()
	} else {
		diags = append(diags, p.diagUnexpected(typeTok, "deployment type identifier"))
		return "", nil, diags
	}

	if p.peek().Type != lexer.TokenLParen {
		return dt, nil, diags
	}
	p.consume() // consume '('

	var rules []ast.DeploymentRule
	for !p.atEOF() && p.peek().Type != lexer.TokenRParen {
		pctTok := p.peek()
		if pctTok.Type != lexer.TokenPercentage {
			diags = append(diags, p.diagUnexpected(pctTok, "percentage (e.g. 90%)"))
			p.consume()
			continue
		}
		pct := pctTok.Value
		p.consume()

		if p.peek().Type != lexer.TokenArrow {
			diags = append(diags, p.diagUnexpected(p.peek(), "->"))
			p.consume()
			continue
		}
		p.consume() // consume '->'

		targetTok := p.peek()
		var target string
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			target = targetTok.Value
			p.consume()
		} else {
			diags = append(diags, p.diagUnexpected(targetTok, "deployment target identifier"))
		}

		rules = append(rules, ast.DeploymentRule{Percentage: pct, Target: target})

		if p.peek().Type == lexer.TokenComma {
			p.consume()
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed deployment rule list (missing `)`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(typeTok),
		})
		return dt, rules, diags
	}
	p.consume() // consume ')'
	return dt, rules, diags
}

// parseIdentList parses a comma-separated list of identifiers (or strings).
// Used for data-stores: values.
func (p *Parser) parseIdentList() []string {
	items, _ := p.parseIdentListWithLines()
	return items
}

// parseIdentListWithLines parses a comma-separated list of identifiers (or
// strings) and returns both the values and their 1-based source lines.
// Used for contexts: so go-to-definition can match the cursor line.
// Per Q3, contextual keywords (e.g. user, domain, service) are valid
// identifiers when they appear in list position — accept any keyword token.
func (p *Parser) parseIdentListWithLines() ([]string, []int) {
	var items []string
	var lines []int
	for {
		tok := p.peek()
		var val string
		switch {
		case tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString:
			val = tok.Value
			p.consume()
		case isKeywordUsedAsIdent(tok.Type):
			val = tok.Value
			p.consume()
		default:
			return items, lines
		}
		items = append(items, val)
		lines = append(lines, tok.Line)
		// Optional comma separator.
		if p.peek().Type == lexer.TokenComma {
			p.consume()
		} else {
			break
		}
	}
	return items, lines
}

// isKeywordUsedAsIdent returns true for keyword token types that the grammar
// allows as plain identifiers in list and name positions (Q3: contextual
// keywords are only keywords by position, not globally reserved).
func isKeywordUsedAsIdent(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwUser, lexer.TokenKwSystem, lexer.TokenKwService,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwServices, lexer.TokenKwUseCase,
		lexer.TokenKwArch, lexer.TokenKwExposure:
		return true
	}
	return false
}

// skipToNextField advances tokens until it finds what looks like the start of
// a field name (TokenIdent) or the closing brace of the current block.
func (p *Parser) skipToNextField() {
	for !p.atEOF() {
		tok := p.peek()
		if tok.Type == lexer.TokenRBrace || tok.Type == lexer.TokenIdent {
			return
		}
		p.consume()
	}
}

// isServiceNameKeyword returns true for token types that can legally appear as
// a service name — i.e., any keyword that a human might use as an identifier.
func isServiceNameKeyword(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwUser, lexer.TokenKwSystem, lexer.TokenKwService,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwServices:
		return true
	}
	return false
}

// --- use_case parsing ---

// parseUseCaseBlock parses: use_case "<name>" { <scenario>* }
// A scenario is: when <trigger> <action>*
// counter is the global ID counter shared across all use_cases in the file.
func (p *Parser) parseUseCaseBlock(counter *int) (*ast.UseCaseDecl, []craft.Diagnostic) {
	ucTok := p.consume() // consume `use_case`
	var diags []craft.Diagnostic

	// Expect a quoted string name.
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenString {
		diags = append(diags, p.diagUnexpected(nameTok, "use_case name string"))
		p.resyncToTopLevel()
		return nil, diags
	}
	name := nameTok.Value
	p.consume()

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

	uc := &ast.UseCaseDecl{Name: name, Line: ucTok.Line}

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// `when` is a contextual keyword that lexes as TokenIdent.
		if tok.Type == lexer.TokenIdent && tok.Value == "when" {
			scenario, d := p.parseScenario(counter)
			diags = append(diags, d...)
			if scenario != nil {
				uc.Scenarios = append(uc.Scenarios, scenario)
			}
		} else {
			// Skip unknown tokens inside the use_case body.
			diags = append(diags, p.diagUnexpected(tok, "`when` or `}`"))
			p.consume()
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed use_case block for %q (missing `}`)", name),
			Severity: craft.SeverityError,
			Range:    tokenRange(ucTok),
		})
		return uc, diags
	}
	uc.EndLine = p.peek().Line // record `}` line
	p.consume()                // consume `}`
	return uc, diags
}

// parseScenario parses one `when <trigger>` clause plus its following action lines.
// counter is a shared global ID counter (pointer) for scenario_N / action_N IDs,
// matching ANTLR's numbering scheme where both scenarios and actions share one counter.
func (p *Parser) parseScenario(counter *int) (*ast.ScenarioDecl, []craft.Diagnostic) {
	var diags []craft.Diagnostic
	whenTok := p.consume() // consume `when`

	trigger, d := p.parseTrigger(whenTok.Line)
	diags = append(diags, d...)

	*counter++
	scenario := &ast.ScenarioDecl{
		ID:      fmt.Sprintf("scenario_%d", *counter),
		Trigger: trigger,
	}

	// Parse actions until we see `when` (next scenario), `}` (end of use_case), or EOF.
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		// `when` starts the next scenario — stop here.
		if tok.Type == lexer.TokenIdent && tok.Value == "when" {
			break
		}
		action, d := p.parseAction(counter)
		diags = append(diags, d...)
		if action != nil {
			scenario.Actions = append(scenario.Actions, action)
		}
	}

	return scenario, diags
}

// parseTrigger parses the `<actor/domain> <verb> <phrase>` part after `when`.
// Two forms:
//   - external:      `when <actor> <verb> <phrase>`
//   - domain_listen: `when <domain> listens "<event>"`
func (p *Parser) parseTrigger(whenLine int) (ast.TriggerDecl, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	// event trigger: when "<EventName>"  (no subject identifier)
	if p.peek().Type == lexer.TokenString {
		eventTok := p.consume()
		desc := fmt.Sprintf("when %q", eventTok.Value)
		return ast.TriggerDecl{
			TriggerType:   "event",
			Event:         eventTok.Value,
			EventColumn:   eventTok.Column,
			EventIsString: true,
			Description:   desc,
			Line:          whenLine,
		}, diags
	}

	// The first token is the actor/domain subject.
	subjectTok := p.peek()
	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		diags = append(diags, p.diagUnexpected(subjectTok, "trigger subject (actor/domain name)"))
		return ast.TriggerDecl{Description: "when"}, diags
	}
	subject := subjectTok.Value
	p.consume()

	// The second token is the verb.  If it is `listens` (ident), this is domain_listen.
	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb token — treat as a partial trigger.
		return ast.TriggerDecl{
			TriggerType: "external",
			Actor:       subject,
			ActorColumn: subjectTok.Column,
			Description: "when " + subject,
		}, diags
	}
	verb := verbTok.Value
	p.consume()

	if verb == "listens" {
		// domain_listen: when <domain> listens "<event>"
		eventTok := p.peek()
		var event string
		isString := false
		if eventTok.Type == lexer.TokenString {
			event = eventTok.Value
			isString = true
			p.consume()
		} else if eventTok.Type == lexer.TokenIdent {
			event = eventTok.Value
			p.consume()
		}
		desc := fmt.Sprintf("when %s listens %q", subject, event)
		return ast.TriggerDecl{
			TriggerType:   "domain_listen",
			Context:       subject,
			Event:         event,
			EventColumn:   eventTok.Column,
			EventIsString: isString,
			Description:   desc,
			Line:          whenLine,
		}, diags
	}

	// external: when <actor> <verb> [connector_word] <phrase>
	// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
	// When present, it is stripped from the phrase (matching ANTLR trigger description format).
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == verbTok.Line {
		p.consume() // strip connector_word; triggers don't store it anywhere
	}
	phrase := p.collectPhrase(verbTok.Line)
	// ANTLR builds description as "when actor verb phrase" (always appends phrase, even when empty).
	fullDesc := fmt.Sprintf("when %s %s %s", subject, verb, phrase)
	return ast.TriggerDecl{
		TriggerType: "external",
		Actor:       subject,
		ActorColumn: subjectTok.Column,
		Verb:        verb,
		Phrase:      phrase,
		Description: fullDesc,
		Line:        whenLine,
	}, diags
}

// isConnectorWord returns true for ANTLR grammar connector_word tokens.
// Grammar rule: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by'
func isConnectorWord(v string) bool {
	switch v {
	case "a", "an", "the", "as", "to", "from", "in", "on", "at", "for", "with", "by":
		return true
	}
	return false
}

// parseAction parses a single action line. counter is the shared global ID counter;
// it is incremented for each action parsed.
//
// Action forms (subject is always an ident/keyword-as-ident):
//
//	<domain> asks <target> to|for <phrase>   → sync_action
//	<domain> notifies "<event>"              → async_action
//	<domain> returns [to <target>] <phrase>  → return_action
//	<domain> <verb> <phrase>                 → internal_action
func (p *Parser) parseAction(counter *int) (*ast.ActionDecl, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	subjectTok := p.peek()
	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		// Not an action line — skip the token.
		diags = append(diags, p.diagUnexpected(subjectTok, "action subject (domain/service name) or `when`"))
		p.consume()
		return nil, diags
	}
	subject := subjectTok.Value
	subjectCol := subjectTok.Column
	actionLine := subjectTok.Line
	p.consume()

	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb — treat as minimal internal action.
		*counter++
		*counter++
		return &ast.ActionDecl{
			ActionType:    "internal_action",
			ActionID:      *counter,
			Context:       subject,
			ContextColumn: subjectCol,
			Description:   subject,
			Line:          actionLine,
		}, diags
	}
	verb := verbTok.Value
	p.consume()

	*counter++
	id := *counter

	switch verb {
	case "asks":
		return p.parseAsksAction(id, subject, subjectCol, actionLine, &diags)
	case "notifies":
		return p.parseNotifiesAction(id, subject, subjectCol, actionLine, &diags)
	case "returns":
		return p.parseReturnsAction(id, subject, subjectCol, actionLine, &diags)
	default:
		// internal_action: <domain> <verb> [connector_word] <phrase>
		// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
		var connector string
		connTok := p.peek()
		if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == actionLine {
			connector = connTok.Value
			p.consume()
		}
		phrase := p.collectPhrase(actionLine)
		desc := subject + " " + verb
		if connector != "" {
			desc += " " + connector
		}
		if phrase != "" {
			desc += " " + phrase
		}
		return &ast.ActionDecl{
			ActionType:    "internal_action",
			ActionID:      id,
			Context:       subject,
			ContextColumn: subjectCol,
			Verb:          verb,
			Connector:     connector,
			Phrase:        phrase,
			Description:   desc,
			Line:          actionLine,
		}, diags
	}
}

// parseAsksAction parses: <domain> asks <target> to|for <phrase>
func (p *Parser) parseAsksAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic) (*ast.ActionDecl, []craft.Diagnostic) {
	targetTok := p.peek()
	var target string
	var targetCol int
	if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
		target = targetTok.Value
		targetCol = targetTok.Column
		p.consume()
	}

	// connector: "to" or "for"
	connTok := p.peek()
	var connector string
	if connTok.Type == lexer.TokenIdent && (connTok.Value == "to" || connTok.Value == "for") {
		connector = connTok.Value
		p.consume()
	}

	phrase := p.collectPhrase(line)
	desc := subject + " asks " + target + " " + connector
	if phrase != "" {
		desc += " " + phrase
	}

	return &ast.ActionDecl{
		ActionType:          "sync_action",
		ActionID:            id,
		Context:             subject,
		ContextColumn:       subjectCol,
		TargetContext:       target,
		TargetContextColumn: targetCol,
		Connector:           connector,
		Phrase:              phrase,
		Description:         desc,
		Line:                line,
	}, *diags
}

// parseNotifiesAction parses: <domain> notifies "<event>"
func (p *Parser) parseNotifiesAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic) (*ast.ActionDecl, []craft.Diagnostic) {
	eventTok := p.peek()
	var event string
	var eventCol int
	var eventIsString bool
	if eventTok.Type == lexer.TokenString {
		event = eventTok.Value
		eventCol = eventTok.Column
		eventIsString = true
		p.consume()
	} else if eventTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(eventTok.Type) {
		event = eventTok.Value
		eventCol = eventTok.Column
		p.consume()
	}

	desc := fmt.Sprintf("%s notifies %q", subject, event)
	return &ast.ActionDecl{
		ActionType:    "async_action",
		ActionID:      id,
		Context:       subject,
		ContextColumn: subjectCol,
		Event:         event,
		EventColumn:   eventCol,
		EventIsString: eventIsString,
		Description:   desc,
		Line:          line,
	}, *diags
}

// parseReturnsAction parses: <domain> returns [to <target>] [connector_word] <phrase>
func (p *Parser) parseReturnsAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic) (*ast.ActionDecl, []craft.Diagnostic) {
	// Check for optional `to <target>`
	var target string
	var targetCol int
	if p.peek().Type == lexer.TokenIdent && p.peek().Value == "to" {
		p.consume() // consume `to`
		targetTok := p.peek()
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			target = targetTok.Value
			targetCol = targetTok.Column
			p.consume()
		}
	}

	// Optional connector_word before phrase (ANTLR grammar: return_action connector_word? phrase)
	var connector string
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == line {
		connector = connTok.Value
		p.consume()
	}

	phrase := p.collectPhrase(line)

	// Build description matching ANTLR format.
	// ANTLR: connector is NOT included in description for no-target returns.
	var desc string
	if target != "" {
		desc = fmt.Sprintf("%s returns %s to %s", subject, phrase, target)
	} else {
		desc = fmt.Sprintf("%s returns %s", subject, phrase)
	}

	return &ast.ActionDecl{
		ActionType:          "return_action",
		ActionID:            id,
		Context:             subject,
		ContextColumn:       subjectCol,
		TargetContext:       target,
		TargetContextColumn: targetCol,
		Connector:           connector,
		Phrase:              phrase,
		Description:         desc,
		Line:                line,
	}, *diags
}

// collectPhrase collects the remaining "phrase" tokens on the current logical line.
// It stops before the next action line or scenario boundary.
//
// The phrase ends when we encounter a token on a **different source line** from
// the current token (since the lexer preserves Line info even when skipping
// whitespace). This correctly handles multi-word phrases without requiring
// newline tokens.
//
// actionLine is the 1-based source line of the action/trigger that owns this phrase.
// If the first available token is already on a different line, the phrase is empty.
// Additionally, we stop on structural boundaries: `}` and EOF.
func (p *Parser) collectPhrase(actionLine int) string {
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return ""
	}
	// If the next token is already past the action's line, the phrase is empty.
	if p.peek().Line != actionLine {
		return ""
	}
	// The phrase starts at the current token's line (== actionLine).
	startLine := actionLine
	var parts []string
	for {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenRBrace, lexer.TokenEOF:
			return strings.Join(parts, " ")
		case lexer.TokenIdent:
			// Stop when we've moved to a different source line.
			// `when` on a new line is a scenario boundary; `when` on the same
			// line is a valid phrase_word per the ANTLR grammar and is collected.
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			parts = append(parts, tok.Value)
			p.consume()
		case lexer.TokenString:
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			// Use the raw value without re-quoting to match ANTLR phrase output.
			parts = append(parts, tok.Value)
			p.consume()
		case lexer.TokenNumber:
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			parts = append(parts, tok.Value)
			p.consume()
		default:
			// Keywords that act as identifiers in phrase context (e.g. `user`, `system`,
			// `service`, `domain`, `arch`) are valid phrase_words per the ANTLR grammar.
			if isAnyKeywordAsIdent(tok.Type) {
				if tok.Line != startLine {
					return strings.Join(parts, " ")
				}
				parts = append(parts, tok.Value)
				p.consume()
				continue
			}
			return strings.Join(parts, " ")
		}
	}
}


// --- arch parsing ---

// parseArchBlock parses: arch <name>? { <arch_sections> }
// where arch_sections is one or more presentation: or gateway: labelled lists.
func (p *Parser) parseArchBlock() (*ast.ArchDecl, []craft.Diagnostic) {
	archTok := p.consume() // consume `arch`
	var diags []craft.Diagnostic

	arch := &ast.ArchDecl{Line: archTok.Line}

	// Optional name: an identifier that is NOT `{`.
	if p.peek().Type == lexer.TokenIdent || isAnyKeywordAsIdent(p.peek().Type) {
		if p.peek().Type != lexer.TokenLBrace {
			arch.Name = p.peek().Value
			p.consume()
		}
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

	// Parse sections until `}` or EOF.
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// Section label detection: identifier followed by `:` at this position.
		// presentation: or gateway: are the only valid section labels.
		if (tok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(tok.Type)) && p.peekAt(1).Type == lexer.TokenColon {
			label := tok.Value
			labelLine := tok.Line
			p.consume() // consume label ident
			p.consume() // consume `:`

			components, d := p.parseArchComponentList()
			diags = append(diags, d...)

			switch label {
			case "presentation":
				arch.Presentation = components
				arch.PresentationLine = labelLine
			case "gateway":
				arch.Gateway = components
				arch.GatewayLine = labelLine
			default:
				// Unknown section label — warn and discard components.
				diags = append(diags, craft.Diagnostic{
					Code:     "craft/syntax/unknown-arch-section",
					Message:  fmt.Sprintf("unknown arch section %q; expected presentation or gateway", label),
					Severity: craft.SeverityWarning,
					Range:    craft.Range{Start: craft.Position{Line: ast.LineToLSP(labelLine)}},
				})
			}
		} else {
			// Unexpected token in arch body — skip.
			diags = append(diags, p.diagUnexpected(tok, "arch section label (presentation or gateway) or `}`"))
			p.consume()
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed arch block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(archTok),
		})
		return arch, diags
	}
	arch.EndLine = p.peek().Line
	p.consume() // consume `}`
	return arch, diags
}

// parseArchComponentList parses a list of arch components until a section label
// (ident followed by `:`), `}`, or EOF. Each component is on its own logical line.
func (p *Parser) parseArchComponentList() ([]*ast.ArchComponent, []craft.Diagnostic) {
	var components []*ast.ArchComponent
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Stop if we see the start of another section label (ident:).
		if (p.peek().Type == lexer.TokenIdent || isAnyKeywordAsIdent(p.peek().Type)) &&
			p.peekAt(1).Type == lexer.TokenColon {
			break
		}
		// Skip unexpected non-ident tokens.
		if p.peek().Type != lexer.TokenIdent && !isAnyKeywordAsIdent(p.peek().Type) {
			diags = append(diags, p.diagUnexpected(p.peek(), "component name or section label"))
			p.consume()
			continue
		}

		comp, d := p.parseArchComponent()
		diags = append(diags, d...)
		if comp != nil {
			components = append(components, comp)
		}
	}
	return components, diags
}

// parseArchComponent parses a single component entry. May be a simple component
// or a flow chain (A > B > C), each component optionally bearing modifiers.
func (p *Parser) parseArchComponent() (*ast.ArchComponent, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	first, d := p.parseComponentWithModifiers()
	diags = append(diags, d...)
	if first == nil {
		return nil, diags
	}

	// Check for flow operator `>`.
	if p.peek().Type != lexer.TokenGT {
		first.Type = "simple"
		return first, diags
	}

	// Flow chain: collect all components separated by `>`.
	chain := []*ast.ArchComponent{first}
	for p.peek().Type == lexer.TokenGT {
		p.consume() // consume `>`
		next, d := p.parseComponentWithModifiers()
		diags = append(diags, d...)
		if next != nil {
			next.Type = "simple"
			chain = append(chain, next)
		}
	}

	return &ast.ArchComponent{
		Type:  "flow",
		Chain: chain,
	}, diags
}

// parseComponentWithModifiers parses: <name> ('[' modifier_list ']')?
func (p *Parser) parseComponentWithModifiers() (*ast.ArchComponent, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "component name"))
		return nil, diags
	}
	name := nameTok.Value
	p.consume()

	comp := &ast.ArchComponent{Name: name, Type: "simple"}

	if p.peek().Type == lexer.TokenLBracket {
		p.consume() // consume `[`
		mods, d := p.parseModifierList()
		diags = append(diags, d...)
		comp.Modifiers = mods

		if p.peek().Type == lexer.TokenRBracket {
			p.consume() // consume `]`
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "]"))
		}
	}

	return comp, diags
}

// parseModifierList parses: modifier (',' modifier)* inside `[...]`.
// A modifier is: identifier (':' identifier)?
func (p *Parser) parseModifierList() ([]ast.ArchModifier, []craft.Diagnostic) {
	var mods []ast.ArchModifier
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBracket {
		keyTok := p.peek()
		if keyTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(keyTok.Type) {
			diags = append(diags, p.diagUnexpected(keyTok, "modifier key"))
			p.consume()
			continue
		}
		key := keyTok.Value
		p.consume()

		var value string
		if p.peek().Type == lexer.TokenColon {
			p.consume() // consume `:`
			valTok := p.peek()
			switch valTok.Type {
			case lexer.TokenIdent:
				value = valTok.Value
				p.consume()
			case lexer.TokenString:
				value = valTok.Value // already unquoted by lexer
				p.consume()
			case lexer.TokenNumber, lexer.TokenPercentage:
				value = valTok.Value
				p.consume()
			default:
				if isAnyKeywordAsIdent(valTok.Type) {
					value = valTok.Value
					p.consume()
				} else {
					diags = append(diags, p.diagUnexpected(valTok, "modifier value (identifier, string, or number)"))
				}
			}
		}

		mods = append(mods, ast.ArchModifier{Key: key, Value: value})

		if p.peek().Type == lexer.TokenComma {
			p.consume() // consume `,`
		} else {
			break
		}
	}
	return mods, diags
}

// peekAt returns the token at pos+offset without advancing the lexer.
func (p *Parser) peekAt(offset int) lexer.Token {
	idx := p.pos + offset
	if idx < len(p.tokens) {
		return p.tokens[idx]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

// isAnyKeywordAsIdent returns true for keyword token types that can appear as
// identifiers in use-case bodies or component names (e.g. keywords used as names).
func isAnyKeywordAsIdent(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwUser, lexer.TokenKwSystem, lexer.TokenKwService,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwServices, lexer.TokenKwUseCase,
		lexer.TokenKwArch:
		return true
	}
	return false
}

// --- helpers ---

// tokenToActorType converts a token to an ActorType.
// Q5: any identifier is a valid actor type (open taxonomy).
// The canonical types (user/system/service) are preserved as constants.
func tokenToActorType(tok lexer.Token) (ast.ActorType, bool) {
	switch tok.Type {
	case lexer.TokenKwUser:
		return ast.ActorTypeUser, true
	case lexer.TokenKwSystem:
		return ast.ActorTypeSystem, true
	case lexer.TokenKwService:
		return ast.ActorTypeService, true
	case lexer.TokenIdent:
		return ast.ActorType(tok.Value), true
	}
	if isAnyKeywordAsIdent(tok.Type) {
		return ast.ActorType(tok.Value), true
	}
	return "", false
}

// resyncToTopLevel discards tokens until it finds a known top-level keyword
// or EOF, so the main loop can continue from a clean state.
func (p *Parser) resyncToTopLevel() {
	for !p.atEOF() {
		tok := p.peek()
		if isTopLevelKeyword(tok.Type) {
			return
		}
		p.consume()
	}
}

func isTopLevelKeyword(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwService, lexer.TokenKwServices,
		lexer.TokenKwUseCase, lexer.TokenKwArch, lexer.TokenKwExposure:
		return true
	}
	return false
}

func (p *Parser) peek() lexer.Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

func (p *Parser) consume() lexer.Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) atEOF() bool {
	return p.peek().Type == lexer.TokenEOF
}

func (p *Parser) diagUnexpected(tok lexer.Token, expected string) craft.Diagnostic {
	return craft.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  fmt.Sprintf("unexpected %q, expected %s", tok.Value, expected),
		Severity: craft.SeverityError,
		Range:    tokenRange(tok),
	}
}

func (p *Parser) diagNotImplemented(tok lexer.Token) craft.Diagnostic {
	return craft.Diagnostic{
		Code:     "craft/syntax/not-yet-implemented",
		Message:  fmt.Sprintf("construct starting with %q is not yet supported by parser v2; use --parser=antlr for full support", tok.Value),
		Severity: craft.SeverityWarning,
		Range:    tokenRange(tok),
	}
}

func tokenRange(tok lexer.Token) craft.Range {
	// LSP lines are 0-based; lexer lines are 1-based.
	line := tok.Line - 1
	if line < 0 {
		line = 0
	}
	col := tok.Column - 1
	if col < 0 {
		col = 0
	}
	end := col + len([]rune(tok.Value))
	return craft.Range{
		Start: craft.Position{Line: line, Character: col},
		End:   craft.Position{Line: line, Character: end},
	}
}

// --- exposure parsing (S8) ---

// parseExposureBlock parses: exposure <name> { to: ... contexts: ... through: ... }
// Field keywords (to, contexts, through) are contextual identifiers per Q3.
func (p *Parser) parseExposureBlock() (*ast.ExposureDecl, []craft.Diagnostic) {
	kwTok := p.peek()
	kwLine := kwTok.Line
	p.consume() // consume `exposure`
	var diags []craft.Diagnostic

	// Exposure name: any identifier or keyword-used-as-identifier (including "default").
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isKeywordUsedAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "exposure name"))
		p.resyncToTopLevel()
		return nil, diags
	}
	name := nameTok.Value
	p.consume()

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

	exp := &ast.ExposureDecl{Name: name, Line: kwLine}

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(tok, "field name (to, contexts, through) or `}`"))
			p.consume()
			continue
		}
		fieldName := tok.Value
		p.consume()

		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			continue
		}
		p.consume() // consume `:`

		switch fieldName {
		case "to":
			exp.To = p.parseIdentList()
		case "contexts":
			exp.Contexts = p.parseIdentList()
		case "through":
			exp.Through = p.parseIdentList()
		default:
			p.skipToNextField()
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed exposure block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(kwTok),
		})
		return exp, diags
	}
	p.consume() // consume `}`
	return exp, diags
}
