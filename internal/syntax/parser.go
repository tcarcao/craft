// Package syntax implements the hand-written recursive-descent parser for the
// Craft DSL. It produces an internal/ast.File.
//
// S3: actors. S4: domains. Unsupported top-level keywords emit a recoverable
// "not-yet-implemented" diagnostic so --parser=v2 is usable on partial files.
package syntax

import (
	"fmt"

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
			actors, d := p.parseActorsBlock()
			diags = append(diags, d...)
			file.Actors = append(file.Actors, actors...)
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
		diags = append(diags, p.diagUnexpected(typeTok, "actor type (user/system/service)"))
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
func (p *Parser) parseActorsBlock() ([]*ast.ActorDecl, []craft.Diagnostic) {
	p.consume() // consume `actors`
	var diags []craft.Diagnostic
	var actors []*ast.ActorDecl

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return nil, diags
	}
	p.consume() // consume `{`

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
			diags = append(diags, p.diagUnexpected(tok, "actor type (user/system/service)"))
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
			Name: nameTok.Value,
			Type: at,
			Line: nameTok.Line,
		})
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed actors block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    craft.Range{Start: craft.Position{Line: p.peek().Line - 1}},
		})
		return actors, diags
	}
	p.consume() // consume `}`
	return actors, diags
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
			Range:    craft.Range{Start: craft.Position{Line: ast.LineToLSP(nameTok.Line)}},
		})
		return &ast.DomainDecl{Name: nameTok.Value, BoundedContexts: contexts, Line: nameTok.Line}, diags
	}
	p.consume() // consume `}`

	return &ast.DomainDecl{
		Name:            nameTok.Value,
		BoundedContexts: contexts,
		Line:            nameTok.Line,
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
				Range:    craft.Range{Start: craft.Position{Line: ast.LineToLSP(nameTok.Line)}},
			})
			domains = append(domains, &ast.DomainDecl{
				Name:            nameTok.Value,
				BoundedContexts: contexts,
				Line:            nameTok.Line,
			})
			return domains, diags
		}
		p.consume() // consume `}`

		domains = append(domains, &ast.DomainDecl{
			Name:            nameTok.Value,
			BoundedContexts: contexts,
			Line:            nameTok.Line,
		})
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domains block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    craft.Range{Start: craft.Position{Line: p.peek().Line - 1}},
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
func (p *Parser) parseBoundedContextList() ([]string, []craft.Diagnostic) {
	var contexts []string
	seen := make(map[string]bool)
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type == lexer.TokenIdent || isDomainNameToken(tok.Type) {
			if !seen[tok.Value] {
				seen[tok.Value] = true
				contexts = append(contexts, tok.Value)
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
		lexer.TokenKwDomain, lexer.TokenKwDomains:
		return true
	}
	return false
}

// --- helpers ---

func tokenToActorType(tok lexer.Token) (ast.ActorType, bool) {
	switch tok.Type {
	case lexer.TokenKwUser:
		return ast.ActorTypeUser, true
	case lexer.TokenKwSystem:
		return ast.ActorTypeSystem, true
	case lexer.TokenKwService:
		return ast.ActorTypeService, true
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
		lexer.TokenKwDomain, lexer.TokenKwDomains:
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
