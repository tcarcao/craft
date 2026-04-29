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
	tree, _, diags := ParseTree(src)
	return Lower(tree), diags
}

// ParseTree parses the given source text, returning both a lossless syntax tree
// and the legacy *ast.File, along with any diagnostics. The *SyntaxNode is the
// root File node containing all top-level constructs as children.
func ParseTree(src string) (*SyntaxNode, *ast.File, []craft.Diagnostic) {
	l := lexer.New(src)
	p := &Parser{tokens: l.All()}
	return p.parseFile()
}

// --- main parse loop ---

func (p *Parser) parseFile() (*SyntaxNode, *ast.File, []craft.Diagnostic) {
	root := &SyntaxNode{Kind: SyntaxKindFile}
	file := &ast.File{}
	var diags []craft.Diagnostic

	// Global counter for scenario_N / action_N IDs across all use_cases in the file,
	// matching ANTLR's numbering scheme.
	ucCounter := 0

	for !p.atEOF() {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenKwActor:
			node, actor, d := p.parseActorStatement()
			diags = append(diags, d...)
			if actor != nil {
				file.Actors = append(file.Actors, actor)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwActors:
			node, actors, blockRange, d := p.parseActorsBlock()
			diags = append(diags, d...)
			file.Actors = append(file.Actors, actors...)
			if blockRange != nil {
				file.ActorBlocks = append(file.ActorBlocks, blockRange)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwDomain:
			node, domain, d := p.parseDomainStatement()
			diags = append(diags, d...)
			if domain != nil {
				file.Domains = append(file.Domains, domain)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwDomains:
			node, domains, d := p.parseDomainsBlock()
			diags = append(diags, d...)
			file.Domains = append(file.Domains, domains...)
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwService:
			node, svc, d := p.parseServiceStatement()
			diags = append(diags, d...)
			if svc != nil {
				file.Services = append(file.Services, svc)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwServices:
			node, services, d := p.parseServicesBlock()
			diags = append(diags, d...)
			file.Services = append(file.Services, services...)
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwUseCase:
			node, uc, d := p.parseUseCaseBlock(&ucCounter)
			diags = append(diags, d...)
			if uc != nil {
				file.UseCases = append(file.UseCases, uc)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwArch:
			node, arch, d := p.parseArchBlock()
			diags = append(diags, d...)
			if arch != nil {
				file.Archs = append(file.Archs, arch)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		case lexer.TokenKwExposure:
			node, exp, d := p.parseExposureBlock()
			diags = append(diags, d...)
			if exp != nil {
				file.Exposures = append(file.Exposures, exp)
			}
			if node != nil {
				root.Children = append(root.Children, node)
			}
		default:
			// Unrecognised top-level token: emit a diagnostic and resync to
			// the next top-level keyword (island parsing).
			diags = append(diags, p.diagNotImplemented(tok))
			p.resyncToTopLevel()
		}
	}
	return root, file, diags
}

// parseActorStatement parses: actor <type> <name>
func (p *Parser) parseActorStatement() (*SyntaxNode, *ast.ActorDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindActorDecl}
	var diags []craft.Diagnostic

	// Collect and attach leading trivia (comments).
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}

	// Consume `actor` keyword.
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwActor))

	typeTok := p.peek()
	at, ok := tokenToActorType(typeTok)
	if !ok {
		diags = append(diags, p.diagUnexpected(typeTok, "actor type"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	// Map the actor-type token to the right SyntaxKind.
	typeKind := SyntaxKindIdent
	if k, found := lexerKindToSyntaxKind[typeTok.Type]; found {
		typeKind = k
	}
	node.Children = append(node.Children, p.consumeAs(typeKind))

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent {
		diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))

	return node, &ast.ActorDecl{
		Name:   nameTok.Value,
		Type:   at,
		Line:   nameTok.Line,
		Column: nameTok.Column,
	}, diags
}

// parseActorsBlock parses: actors { <actor_definition>* }
func (p *Parser) parseActorsBlock() (*SyntaxNode, []*ast.ActorDecl, *ast.ActorBlockRange, []craft.Diagnostic) {
	blockNode := &SyntaxNode{Kind: SyntaxKindActorsBlock}
	var diags []craft.Diagnostic

	// Collect leading trivia before `actors`.
	for _, t := range p.collectTrivia() {
		blockNode.Children = append(blockNode.Children, t)
	}

	actorsTok := p.consume() // consume `actors`, capture line
	blockNode.Children = append(blockNode.Children, &SyntaxToken{
		Kind:  SyntaxKindKwActors,
		Value: actorsTok.Value,
		Line:  actorsTok.Line,
		Col:   actorsTok.Column,
	})
	blockRange := &ast.ActorBlockRange{Line: actorsTok.Line}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return blockNode, nil, nil, diags
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindLBrace))

	var actors []*ast.ActorDecl
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Collect trivia inside block.
		for _, t := range p.collectTrivia() {
			blockNode.Children = append(blockNode.Children, t)
		}
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

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

		// Build a child ActorDecl node for each entry in the block.
		actorNode := &SyntaxNode{Kind: SyntaxKindActorDecl}
		typeKind := SyntaxKindIdent
		if k, found := lexerKindToSyntaxKind[tok.Type]; found {
			typeKind = k
		}
		actorNode.Children = append(actorNode.Children, p.consumeAs(typeKind))

		nameTok := p.peek()
		if nameTok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
			p.consume()
			continue
		}
		actorNode.Children = append(actorNode.Children, p.consumeAs(SyntaxKindIdent))
		blockNode.Children = append(blockNode.Children, actorNode)

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
		return blockNode, actors, nil, diags // no block range on unclosed block
	}
	blockRange.EndLine = p.peek().Line // capture `}` line
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindRBrace))
	return blockNode, actors, blockRange, diags
}

// parseDomainStatement parses: domain <name> { <bounded_context>* }
func (p *Parser) parseDomainStatement() (*SyntaxNode, *ast.DomainDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindDomainDecl}
	var diags []craft.Diagnostic

	// Collect leading trivia before `domain`.
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}

	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwDomain))

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isDomainNameToken(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "domain name"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLBrace))

	contexts, bcNodes, d := p.parseBoundedContextList()
	diags = append(diags, d...)
	for _, bcNode := range bcNodes {
		node.Children = append(node.Children, bcNode)
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domain block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(nameTok),
		})
		return node, &ast.DomainDecl{
			Name:            nameTok.Value,
			BoundedContexts: contexts,
			Line:            nameTok.Line,
			Column:          nameTok.Column,
		}, diags
	}
	endLine := p.peek().Line // capture `}` line
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRBrace))

	return node, &ast.DomainDecl{
		Name:            nameTok.Value,
		BoundedContexts: contexts,
		Line:            nameTok.Line,
		Column:          nameTok.Column,
		EndLine:         endLine,
	}, diags
}

// parseDomainsBlock parses: domains { <domain_block>* }
// where each domain_block is: <name> { <bounded_context>* }
func (p *Parser) parseDomainsBlock() (*SyntaxNode, []*ast.DomainDecl, []craft.Diagnostic) {
	blockNode := &SyntaxNode{Kind: SyntaxKindDomainsBlock}
	var diags []craft.Diagnostic
	var domains []*ast.DomainDecl

	// Collect leading trivia before `domains`.
	for _, t := range p.collectTrivia() {
		blockNode.Children = append(blockNode.Children, t)
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindKwDomains))

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return blockNode, nil, diags
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindLBrace))

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Collect trivia inside block.
		for _, t := range p.collectTrivia() {
			blockNode.Children = append(blockNode.Children, t)
		}
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

		tok := p.peek()
		if tok.Type != lexer.TokenIdent && !isDomainNameToken(tok.Type) {
			diags = append(diags, p.diagUnexpected(tok, "domain name"))
			p.consume()
			continue
		}
		nameTok := tok

		// Build a child DomainDecl node.
		domainNode := &SyntaxNode{Kind: SyntaxKindDomainDecl}
		domainNode.Children = append(domainNode.Children, p.consumeAs(SyntaxKindIdent))

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			blockNode.Children = append(blockNode.Children, domainNode)
			continue
		}
		domainNode.Children = append(domainNode.Children, p.consumeAs(SyntaxKindLBrace))

		contexts, bcNodes, d := p.parseBoundedContextList()
		diags = append(diags, d...)
		for _, bcNode := range bcNodes {
			domainNode.Children = append(domainNode.Children, bcNode)
		}

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
			blockNode.Children = append(blockNode.Children, domainNode)
			return blockNode, domains, diags
		}
		endLine := p.peek().Line // capture inner `}` line
		domainNode.Children = append(domainNode.Children, p.consumeAs(SyntaxKindRBrace))
		blockNode.Children = append(blockNode.Children, domainNode)

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
		return blockNode, domains, diags
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindRBrace))
	return blockNode, domains, diags
}

// parseBoundedContextList parses a list of identifiers until `}` or EOF.
// These are the bounded context names inside a domain block.
// Duplicates are silently deduplicated (keeping first occurrence), matching
// ANTLR behavior and the v1 spec.
// Returns both the legacy BoundedContextEntry slice and a slice of SyntaxNodes.
func (p *Parser) parseBoundedContextList() ([]ast.BoundedContextEntry, []*SyntaxNode, []craft.Diagnostic) {
	var contexts []ast.BoundedContextEntry
	var bcNodes []*SyntaxNode
	seen := make(map[string]bool)
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Collect trivia inside the context list.
		triviaTokens := p.collectTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			// Attach any trailing trivia to the last node or discard — for now discard
			// (they'll be captured at the parent's RBrace step).
			_ = triviaTokens
			break
		}

		tok := p.peek()
		if tok.Type == lexer.TokenIdent || isDomainNameToken(tok.Type) {
			bcNode := &SyntaxNode{Kind: SyntaxKindBoundedContext}
			// Attach leading trivia to this bounded-context node.
			for _, t := range triviaTokens {
				bcNode.Children = append(bcNode.Children, t)
			}
			bcNode.Children = append(bcNode.Children, p.consumeAs(SyntaxKindIdent))
			if !seen[tok.Value] {
				seen[tok.Value] = true
				contexts = append(contexts, ast.BoundedContextEntry{
					Name:   tok.Value,
					Line:   tok.Line,
					Column: tok.Column,
				})
				bcNodes = append(bcNodes, bcNode)
			}
			// Duplicate: node is built but not appended (matches ANTLR dedup behavior).
		} else if tok.Type == lexer.TokenError {
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			p.consume()
		} else {
			// Unknown token inside domain block — could be a sub-keyword; skip.
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			p.consume()
		}
	}
	return contexts, bcNodes, diags
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
func (p *Parser) parseServicesBlock() (*SyntaxNode, []*ast.ServiceDecl, []craft.Diagnostic) {
	blockNode := &SyntaxNode{Kind: SyntaxKindServicesBlock}
	var diags []craft.Diagnostic
	var services []*ast.ServiceDecl

	// Collect leading trivia before `services`.
	for _, t := range p.collectTrivia() {
		blockNode.Children = append(blockNode.Children, t)
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindKwServices))

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return blockNode, nil, diags
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindLBrace))

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Collect trivia inside block.
		for _, t := range p.collectTrivia() {
			blockNode.Children = append(blockNode.Children, t)
		}
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

		tok := p.peek()

		// Service name: identifier, string literal, or keyword-as-name
		var name string
		var nameLine, nameCol int
		var nameKind SyntaxKind
		switch tok.Type {
		case lexer.TokenIdent:
			name = tok.Value
			nameLine = tok.Line
			nameCol = tok.Column
			nameKind = SyntaxKindIdent
			p.consume()
		case lexer.TokenString:
			name = tok.Value
			nameLine = tok.Line
			nameCol = tok.Column
			nameKind = SyntaxKindString
			p.consume()
		default:
			if isServiceNameKeyword(tok.Type) {
				name = tok.Value
				nameLine = tok.Line
				nameCol = tok.Column
				if k, found := lexerKindToSyntaxKind[tok.Type]; found {
					nameKind = k
				} else {
					nameKind = SyntaxKindIdent
				}
				p.consume()
			} else {
				diags = append(diags, p.diagUnexpected(tok, "service name"))
				p.consume()
				continue
			}
		}

		// Build the child service node.
		svcNode := &SyntaxNode{Kind: SyntaxKindServiceDecl}
		svcNode.Children = append(svcNode.Children, &SyntaxToken{
			Kind:  nameKind,
			Value: name,
			Line:  nameLine,
			Col:   nameCol,
		})

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			blockNode.Children = append(blockNode.Children, svcNode)
			continue
		}
		svcNode.Children = append(svcNode.Children, p.consumeAs(SyntaxKindLBrace))

		svc, d := p.parseServiceBody(name, nameLine, nameCol, svcNode)
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
			blockNode.Children = append(blockNode.Children, svcNode)
			return blockNode, services, diags
		}
		svc.EndLine = p.peek().Line // record `}` line before consuming
		svcNode.Children = append(svcNode.Children, p.consumeAs(SyntaxKindRBrace))
		blockNode.Children = append(blockNode.Children, svcNode)
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed services block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		return blockNode, services, diags
	}
	blockNode.Children = append(blockNode.Children, p.consumeAs(SyntaxKindRBrace))
	return blockNode, services, diags
}

// parseServiceStatement parses: service <name> { <field>* }
// This is the singular top-level service form (Q11).
func (p *Parser) parseServiceStatement() (*SyntaxNode, *ast.ServiceDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindServiceDecl}
	var diags []craft.Diagnostic

	// Collect leading trivia before `service`.
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwService))

	nameTok := p.peek()
	var name string
	var nameLine, nameCol int
	if nameTok.Type == lexer.TokenIdent || nameTok.Type == lexer.TokenString {
		name = nameTok.Value
		nameLine = nameTok.Line
		nameCol = nameTok.Column
		nameKind := SyntaxKindIdent
		if nameTok.Type == lexer.TokenString {
			nameKind = SyntaxKindString
		}
		node.Children = append(node.Children, p.consumeAs(nameKind))
	} else {
		diags = append(diags, p.diagUnexpected(nameTok, "service name"))
		p.resyncToTopLevel()
		return node, nil, diags
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLBrace))

	svc, d := p.parseServiceBody(name, nameLine, nameCol, node)
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
		return node, svc, diags
	}
	svc.EndLine = p.peek().Line // record `}` line before consuming
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRBrace))
	return node, svc, diags
}

// parseServiceBody parses the fields inside a service { ... } block.
// node receives all consumed tokens as children for lossless tree reconstruction.
func (p *Parser) parseServiceBody(name string, nameLine, nameCol int, node *SyntaxNode) (*ast.ServiceDecl, []craft.Diagnostic) {
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
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent)) // field name

		// Expect colon after field name.
		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			continue
		}
		node.Children = append(node.Children, p.consumeAs(SyntaxKindColon))

		switch fieldName {
		case "contexts":
			svc.Contexts, svc.ContextLines = p.parseIdentListWithLinesIntoNode(node)
		case "data-stores":
			svc.DataStores = p.parseIdentListIntoNode(node)
		case "language":
			if p.peek().Type == lexer.TokenIdent {
				svc.Language = p.peek().Value
				node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
			} else {
				diags = append(diags, p.diagUnexpected(p.peek(), "language identifier"))
			}
		case "deployment":
			dt, rules, dd := p.parseDeploymentSpecIntoNode(node)
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
func (p *Parser) parseUseCaseBlock(counter *int) (*SyntaxNode, *ast.UseCaseDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindUseCaseDecl}
	var diags []craft.Diagnostic

	// Collect leading trivia before `use_case`.
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}

	ucTok := p.peek()
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwUseCase))

	// Expect a quoted string name.
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenString {
		diags = append(diags, p.diagUnexpected(nameTok, "use_case name string"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	name := nameTok.Value
	node.Children = append(node.Children, p.consumeAs(SyntaxKindString))

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLBrace))

	uc := &ast.UseCaseDecl{Name: name, Line: ucTok.Line}

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// `when` is a contextual keyword that lexes as TokenIdent.
		if tok.Type == lexer.TokenIdent && tok.Value == "when" {
			scenarioNode, scenario, d := p.parseScenario(counter)
			diags = append(diags, d...)
			if scenario != nil {
				uc.Scenarios = append(uc.Scenarios, scenario)
			}
			if scenarioNode != nil {
				node.Children = append(node.Children, scenarioNode)
			}
		} else {
			// Skip unknown tokens inside the use_case body.
			diags = append(diags, p.diagUnexpected(tok, "`when` or `}`"))
			node.Children = append(node.Children, p.consumeAs(SyntaxKindError))
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed use_case block for %q (missing `}`)", name),
			Severity: craft.SeverityError,
			Range:    tokenRange(ucTok),
		})
		return node, uc, diags
	}
	uc.EndLine = p.peek().Line // record `}` line
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRBrace))
	return node, uc, diags
}

// parseScenario parses one `when <trigger>` clause plus its following action lines.
// counter is a shared global ID counter (pointer) for scenario_N / action_N IDs,
// matching ANTLR's numbering scheme where both scenarios and actions share one counter.
func (p *Parser) parseScenario(counter *int) (*SyntaxNode, *ast.ScenarioDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindScenario}
	var diags []craft.Diagnostic

	// consume `when` as contextual keyword
	whenTok := p.peek()
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwWhen))

	triggerNode, trigger, d := p.parseTrigger(whenTok.Line)
	diags = append(diags, d...)
	if triggerNode != nil {
		node.Children = append(node.Children, triggerNode)
	}

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
		actionNode, action, d := p.parseAction(counter)
		diags = append(diags, d...)
		if action != nil {
			scenario.Actions = append(scenario.Actions, action)
		}
		if actionNode != nil {
			node.Children = append(node.Children, actionNode)
		}
	}

	return node, scenario, diags
}

// parseTrigger parses the `<actor/domain> <verb> <phrase>` part after `when`.
// Two forms:
//   - external:      `when <actor> <verb> <phrase>`
//   - domain_listen: `when <domain> listens "<event>"`
func (p *Parser) parseTrigger(whenLine int) (*SyntaxNode, ast.TriggerDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindTrigger}
	var diags []craft.Diagnostic

	// event trigger: when "<EventName>"  (no subject identifier)
	if p.peek().Type == lexer.TokenString {
		eventTok := p.peek()
		node.Children = append(node.Children, p.consumeAs(SyntaxKindString))
		desc := fmt.Sprintf("when %q", eventTok.Value)
		return node, ast.TriggerDecl{
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
		return node, ast.TriggerDecl{Description: "when"}, diags
	}
	subject := subjectTok.Value
	subjectKind := SyntaxKindIdent
	if k, found := lexerKindToSyntaxKind[subjectTok.Type]; found {
		subjectKind = k
	}
	node.Children = append(node.Children, p.consumeAs(subjectKind))

	// The second token is the verb.  If it is `listens` (ident), this is domain_listen.
	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb token — treat as a partial trigger.
		return node, ast.TriggerDecl{
			TriggerType: "external",
			Actor:       subject,
			ActorColumn: subjectTok.Column,
			Description: "when " + subject,
		}, diags
	}
	verb := verbTok.Value

	if verb == "listens" {
		node.Children = append(node.Children, p.consumeAs(SyntaxKindKwListens))
		// domain_listen: when <domain> listens "<event>"
		eventTok := p.peek()
		var event string
		isString := false
		if eventTok.Type == lexer.TokenString {
			event = eventTok.Value
			isString = true
			node.Children = append(node.Children, p.consumeAs(SyntaxKindString))
		} else if eventTok.Type == lexer.TokenIdent {
			event = eventTok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}
		desc := fmt.Sprintf("when %s listens %q", subject, event)
		return node, ast.TriggerDecl{
			TriggerType:   "domain_listen",
			Context:       subject,
			ActorColumn:   subjectTok.Column,
			Event:         event,
			EventColumn:   eventTok.Column,
			EventIsString: isString,
			Description:   desc,
			Line:          whenLine,
		}, diags
	}

	// external: when <actor> <verb> [connector_word] <phrase>
	node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))

	// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
	// When present, it is stripped from the phrase (matching ANTLR trigger description format).
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == verbTok.Line {
		// consume connector_word; not stored in legacy phrase
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
	}
	// Collect phrase: use collectPhraseIntoNode to simultaneously build node children and phrase string.
	phrase := p.collectPhraseIntoNode(verbTok.Line, node)

	fullDesc := fmt.Sprintf("when %s %s %s", subject, verb, phrase)
	return node, ast.TriggerDecl{
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
func (p *Parser) parseAction(counter *int) (*SyntaxNode, *ast.ActionDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindAction}
	var diags []craft.Diagnostic

	subjectTok := p.peek()
	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		// Not an action line — skip the token.
		diags = append(diags, p.diagUnexpected(subjectTok, "action subject (domain/service name) or `when`"))
		node.Children = append(node.Children, p.consumeAs(SyntaxKindError))
		return node, nil, diags
	}
	subject := subjectTok.Value
	subjectCol := subjectTok.Column
	actionLine := subjectTok.Line
	subjectKind := SyntaxKindIdent
	if k, found := lexerKindToSyntaxKind[subjectTok.Type]; found {
		subjectKind = k
	}
	node.Children = append(node.Children, p.consumeAs(subjectKind))

	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb — treat as minimal internal action.
		*counter++
		*counter++
		return node, &ast.ActionDecl{
			ActionType:    "internal_action",
			ActionID:      *counter,
			Context:       subject,
			ContextColumn: subjectCol,
			Description:   subject,
			Line:          actionLine,
		}, diags
	}
	verb := verbTok.Value

	*counter++
	id := *counter

	switch verb {
	case "asks":
		node.Children = append(node.Children, p.consumeAs(SyntaxKindKwAsks))
		decl, d := p.parseAsksAction(id, subject, subjectCol, actionLine, &diags, node)
		return node, decl, d
	case "notifies":
		node.Children = append(node.Children, p.consumeAs(SyntaxKindKwNotifies))
		decl, d := p.parseNotifiesAction(id, subject, subjectCol, actionLine, &diags, node)
		return node, decl, d
	case "returns":
		node.Children = append(node.Children, p.consumeAs(SyntaxKindKwReturns))
		decl, d := p.parseReturnsAction(id, subject, subjectCol, actionLine, &diags, node)
		return node, decl, d
	default:
		// internal_action: <domain> <verb> [connector_word] <phrase>
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
		var connector string
		connTok := p.peek()
		if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == actionLine {
			connector = connTok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}
		phrase := p.collectPhraseIntoNode(actionLine, node)
		desc := subject + " " + verb
		if connector != "" {
			desc += " " + connector
		}
		if phrase != "" {
			desc += " " + phrase
		}
		return node, &ast.ActionDecl{
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
// The `asks` keyword token has already been consumed and added to node by parseAction.
func (p *Parser) parseAsksAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic, node *SyntaxNode) (*ast.ActionDecl, []craft.Diagnostic) {
	targetTok := p.peek()
	var target string
	var targetCol int
	if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
		target = targetTok.Value
		targetCol = targetTok.Column
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
	}

	// connector: "to" or "for"
	connTok := p.peek()
	var connector string
	if connTok.Type == lexer.TokenIdent && (connTok.Value == "to" || connTok.Value == "for") {
		connector = connTok.Value
		if connTok.Value == "to" {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindKwTo))
		} else {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}
	}

	phrase := p.collectPhraseIntoNode(line, node)
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
// The `notifies` keyword token has already been consumed and added to node by parseAction.
func (p *Parser) parseNotifiesAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic, node *SyntaxNode) (*ast.ActionDecl, []craft.Diagnostic) {
	eventTok := p.peek()
	var event string
	var eventCol int
	var eventIsString bool
	if eventTok.Type == lexer.TokenString {
		event = eventTok.Value
		eventCol = eventTok.Column
		eventIsString = true
		node.Children = append(node.Children, p.consumeAs(SyntaxKindString))
	} else if eventTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(eventTok.Type) {
		event = eventTok.Value
		eventCol = eventTok.Column
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
	} else if eventTok.Type == lexer.TokenError {
		*diags = append(*diags, p.diagUnterminatedString(eventTok))
		node.Children = append(node.Children, p.consumeAs(SyntaxKindError))
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
// The `returns` keyword token has already been consumed and added to node by parseAction.
func (p *Parser) parseReturnsAction(id int, subject string, subjectCol int, line int, diags *[]craft.Diagnostic, node *SyntaxNode) (*ast.ActionDecl, []craft.Diagnostic) {
	// Check for optional `to <target>`
	var target string
	var targetCol int
	if p.peek().Type == lexer.TokenIdent && p.peek().Value == "to" {
		node.Children = append(node.Children, p.consumeAs(SyntaxKindKwTo))
		targetTok := p.peek()
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			target = targetTok.Value
			targetCol = targetTok.Column
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}
	}

	// Optional connector_word before phrase (ANTLR grammar: return_action connector_word? phrase)
	var connector string
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == line {
		connector = connTok.Value
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
	}

	phrase := p.collectPhraseIntoNode(line, node)

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


// collectPhraseIntoNode collects phrase tokens on actionLine, appends them as
// SyntaxToken children of node, and returns the phrase string (matching collectPhrase output).
func (p *Parser) collectPhraseIntoNode(actionLine int, node *SyntaxNode) string {
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return ""
	}
	if p.peek().Line != actionLine {
		return ""
	}
	startLine := actionLine
	var parts []string
	for {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenRBrace, lexer.TokenEOF:
			return strings.Join(parts, " ")
		case lexer.TokenIdent:
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			parts = append(parts, tok.Value)
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		case lexer.TokenString:
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			parts = append(parts, tok.Value)
			node.Children = append(node.Children, p.consumeAs(SyntaxKindString))
		case lexer.TokenNumber:
			if tok.Line != startLine {
				return strings.Join(parts, " ")
			}
			parts = append(parts, tok.Value)
			node.Children = append(node.Children, p.consumeAs(SyntaxKindNumber))
		default:
			if isAnyKeywordAsIdent(tok.Type) {
				if tok.Line != startLine {
					return strings.Join(parts, " ")
				}
				parts = append(parts, tok.Value)
				node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
				continue
			}
			return strings.Join(parts, " ")
		}
	}
}

// --- arch parsing ---

// parseArchBlock parses: arch <name>? { <arch_sections> }
// where arch_sections is one or more presentation: or gateway: labelled lists.
func (p *Parser) parseArchBlock() (*SyntaxNode, *ast.ArchDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindArchDecl}
	var diags []craft.Diagnostic

	// Collect leading trivia before `arch`.
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}

	archTok := p.peek()
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwArch))

	arch := &ast.ArchDecl{Line: archTok.Line}

	// Optional name: an identifier that is NOT `{`.
	if p.peek().Type == lexer.TokenIdent || isAnyKeywordAsIdent(p.peek().Type) {
		if p.peek().Type != lexer.TokenLBrace {
			arch.Name = p.peek().Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLBrace))

	// Parse sections until `}` or EOF.
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// Section label detection: identifier followed by `:` at this position.
		// presentation: or gateway: are the only valid section labels.
		if (tok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(tok.Type)) && p.peekAt(1).Type == lexer.TokenColon {
			label := tok.Value
			labelLine := tok.Line

			sectionNode := &SyntaxNode{Kind: SyntaxKindArchSection}

			// Consume label as contextual keyword.
			switch label {
			case "presentation":
				sectionNode.Children = append(sectionNode.Children, p.consumeAs(SyntaxKindKwPresentation))
			case "gateway":
				sectionNode.Children = append(sectionNode.Children, p.consumeAs(SyntaxKindKwGateway))
			default:
				sectionNode.Children = append(sectionNode.Children, p.consumeAs(SyntaxKindIdent))
			}
			sectionNode.Children = append(sectionNode.Children, p.consumeAs(SyntaxKindColon))

			components, componentNodes, d := p.parseArchComponentListWithNodes()
			diags = append(diags, d...)
			for _, cn := range componentNodes {
				sectionNode.Children = append(sectionNode.Children, cn)
			}
			node.Children = append(node.Children, sectionNode)

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
			node.Children = append(node.Children, p.consumeAs(SyntaxKindError))
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed arch block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(archTok),
		})
		return node, arch, diags
	}
	arch.EndLine = p.peek().Line
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRBrace))
	return node, arch, diags
}


// parseArchComponentListWithNodes is like parseArchComponentList but also returns SyntaxNodes.
func (p *Parser) parseArchComponentListWithNodes() ([]*ast.ArchComponent, []*SyntaxNode, []craft.Diagnostic) {
	var components []*ast.ArchComponent
	var nodes []*SyntaxNode
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

		compNode, comp, d := p.parseArchComponentWithNode()
		diags = append(diags, d...)
		if comp != nil {
			components = append(components, comp)
		}
		if compNode != nil {
			nodes = append(nodes, compNode)
		}
	}
	return components, nodes, diags
}

// parseArchComponentWithNode parses a single component entry and builds a SyntaxNode.
// Mirrors parseArchComponent but also builds the syntax tree.
func (p *Parser) parseArchComponentWithNode() (*SyntaxNode, *ast.ArchComponent, []craft.Diagnostic) {
	compNode := &SyntaxNode{Kind: SyntaxKindArchComponent}
	var diags []craft.Diagnostic

	// Parse the first component with optional modifiers.
	first, firstNode, d := p.parseComponentWithModifiersNode(compNode)
	diags = append(diags, d...)
	if first == nil {
		return compNode, nil, diags
	}

	// Check for flow operator `>`.
	if p.peek().Type != lexer.TokenGT {
		first.Type = "simple"
		return compNode, first, diags
	}

	// Flow chain: collect all components separated by `>`.
	// Use parseComponentWithModifiers (not recursive) to match original flat-chain logic.
	_ = firstNode
	chain := []*ast.ArchComponent{first}
	for p.peek().Type == lexer.TokenGT {
		compNode.Children = append(compNode.Children, p.consumeAs(SyntaxKindGT))
		nextCompNode := &SyntaxNode{Kind: SyntaxKindArchComponent}
		next, _, d := p.parseComponentWithModifiersNode(nextCompNode)
		diags = append(diags, d...)
		if next != nil {
			next.Type = "simple"
			chain = append(chain, next)
		}
		// Flatten nextCompNode children into compNode.
		compNode.Children = append(compNode.Children, nextCompNode.Children...)
	}

	flowComp := &ast.ArchComponent{Type: "flow", Chain: chain}
	return compNode, flowComp, diags
}

// parseComponentWithModifiersNode is like parseComponentWithModifiers but appends tokens to node.
func (p *Parser) parseComponentWithModifiersNode(node *SyntaxNode) (*ast.ArchComponent, *SyntaxNode, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "component name"))
		return nil, nil, diags
	}
	name := nameTok.Value
	node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))

	comp := &ast.ArchComponent{Name: name, Type: "simple"}

	if p.peek().Type == lexer.TokenLBracket {
		node.Children = append(node.Children, p.consumeAs(SyntaxKindLBracket))
		mods, modNodes, d := p.parseModifierListWithNodes()
		diags = append(diags, d...)
		comp.Modifiers = mods
		for _, mn := range modNodes {
			node.Children = append(node.Children, mn)
		}
		if p.peek().Type == lexer.TokenRBracket {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindRBracket))
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "]"))
		}
	}

	return comp, node, diags
}

// parseModifierListWithNodes is like parseModifierList but also returns SyntaxNodes.
func (p *Parser) parseModifierListWithNodes() ([]ast.ArchModifier, []*SyntaxNode, []craft.Diagnostic) {
	var mods []ast.ArchModifier
	var nodes []*SyntaxNode
	var diags []craft.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBracket {
		keyTok := p.peek()
		if keyTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(keyTok.Type) {
			diags = append(diags, p.diagUnexpected(keyTok, "modifier key"))
			p.consume()
			continue
		}
		modNode := &SyntaxNode{Kind: SyntaxKindArchModifier}
		modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindIdent))
		key := keyTok.Value

		var value string
		if p.peek().Type == lexer.TokenColon {
			modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindColon))
			valTok := p.peek()
			switch valTok.Type {
			case lexer.TokenIdent:
				value = valTok.Value
				modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindIdent))
			case lexer.TokenString:
				value = valTok.Value
				modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindString))
			case lexer.TokenNumber, lexer.TokenPercentage:
				value = valTok.Value
				modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindNumber))
			default:
				if isAnyKeywordAsIdent(valTok.Type) {
					value = valTok.Value
					modNode.Children = append(modNode.Children, p.consumeAs(SyntaxKindIdent))
				} else {
					diags = append(diags, p.diagUnexpected(valTok, "modifier value (identifier, string, or number)"))
				}
			}
		}

		mods = append(mods, ast.ArchModifier{Key: key, Value: value})
		nodes = append(nodes, modNode)

		if p.peek().Type == lexer.TokenComma {
			p.consume() // consume `,`
		} else {
			break
		}
	}
	return mods, nodes, diags
}



// peekAt returns the Nth non-comment token at or after p.pos without advancing.
// offset=0 is equivalent to peek(); offset=1 is the token after that, etc.
// Comment tokens are skipped when counting.
func (p *Parser) peekAt(offset int) lexer.Token {
	count := 0
	for i := p.pos; i < len(p.tokens); i++ {
		tt := p.tokens[i].Type
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment {
			continue
		}
		if count == offset {
			return p.tokens[i]
		}
		count++
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

// peek returns the next non-comment token without advancing p.pos.
// Comment tokens are skipped transparently for parse decisions.
func (p *Parser) peek() lexer.Token {
	for i := p.pos; i < len(p.tokens); i++ {
		tt := p.tokens[i].Type
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment {
			continue
		}
		return p.tokens[i]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

// consume advances past any comment tokens at p.pos, then consumes and returns
// the next meaningful token (advancing p.pos past it as well).
func (p *Parser) consume() lexer.Token {
	// skip leading comments
	for p.pos < len(p.tokens) {
		tt := p.tokens[p.pos].Type
		if tt != lexer.TokenLineComment && tt != lexer.TokenBlockComment {
			break
		}
		p.pos++
	}
	if p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		p.pos++
		return tok
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

func (p *Parser) atEOF() bool {
	return p.peek().Type == lexer.TokenEOF
}

// collectTrivia collects any comment tokens at p.pos and returns them as
// []*SyntaxToken, advancing p.pos past each one collected.
// Call this before consumeAs to capture leading trivia.
func (p *Parser) collectTrivia() []*SyntaxToken {
	var trivia []*SyntaxToken
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		switch tok.Type {
		case lexer.TokenLineComment:
			trivia = append(trivia, &SyntaxToken{
				Kind:  SyntaxKindLineComment,
				Value: tok.Value,
				Line:  tok.Line,
				Col:   tok.Column,
			})
			p.pos++
		case lexer.TokenBlockComment:
			trivia = append(trivia, &SyntaxToken{
				Kind:  SyntaxKindBlockComment,
				Value: tok.Value,
				Line:  tok.Line,
				Col:   tok.Column,
			})
			p.pos++
		default:
			return trivia
		}
	}
	return trivia
}

// consumeAs converts the current non-comment lexer token into a *SyntaxToken
// with the given kind, advancing p.pos past it.
// Any buffered comment tokens before the meaningful token are skipped
// (they should be collected via collectTrivia beforehand).
func (p *Parser) consumeAs(kind SyntaxKind) *SyntaxToken {
	// skip any leading comments (trivia already collected by caller)
	for p.pos < len(p.tokens) {
		tt := p.tokens[p.pos].Type
		if tt != lexer.TokenLineComment && tt != lexer.TokenBlockComment {
			break
		}
		p.pos++
	}
	tok := p.consume()
	return &SyntaxToken{
		Kind:  kind,
		Value: tok.Value,
		Line:  tok.Line,
		Col:   tok.Column,
	}
}

// lexerKindToSyntaxKind maps hard lexer token types to SyntaxKind values.
// Used in Tasks 5+6 when building SyntaxTokens from lexer tokens.
var lexerKindToSyntaxKind = map[lexer.TokenType]SyntaxKind{
	lexer.TokenKwActor:       SyntaxKindKwActor,
	lexer.TokenKwActors:      SyntaxKindKwActors,
	lexer.TokenKwUser:        SyntaxKindKwUser,
	lexer.TokenKwSystem:      SyntaxKindKwSystem,
	lexer.TokenKwService:     SyntaxKindKwService,
	lexer.TokenKwDomain:      SyntaxKindKwDomain,
	lexer.TokenKwDomains:     SyntaxKindKwDomains,
	lexer.TokenKwServices:    SyntaxKindKwServices,
	lexer.TokenKwUseCase:     SyntaxKindKwUseCase,
	lexer.TokenKwArch:        SyntaxKindKwArch,
	lexer.TokenKwExposure:    SyntaxKindKwExposure,
	lexer.TokenIdent:         SyntaxKindIdent,
	lexer.TokenString:        SyntaxKindString,
	lexer.TokenNumber:        SyntaxKindNumber,
	lexer.TokenPercentage:    SyntaxKindPercentage,
	lexer.TokenLBrace:        SyntaxKindLBrace,
	lexer.TokenRBrace:        SyntaxKindRBrace,
	lexer.TokenLParen:        SyntaxKindLParen,
	lexer.TokenRParen:        SyntaxKindRParen,
	lexer.TokenLBracket:      SyntaxKindLBracket,
	lexer.TokenRBracket:      SyntaxKindRBracket,
	lexer.TokenColon:         SyntaxKindColon,
	lexer.TokenComma:         SyntaxKindComma,
	lexer.TokenGT:            SyntaxKindGT,
	lexer.TokenArrow:         SyntaxKindArrow,
	lexer.TokenError:         SyntaxKindError,
	lexer.TokenEOF:           SyntaxKindEOF,
	lexer.TokenLineComment:   SyntaxKindLineComment,
	lexer.TokenBlockComment:  SyntaxKindBlockComment,
}

func (p *Parser) diagUnexpected(tok lexer.Token, expected string) craft.Diagnostic {
	return craft.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  fmt.Sprintf("unexpected %q, expected %s", tok.Value, expected),
		Severity: craft.SeverityError,
		Range:    tokenRange(tok),
	}
}

// diagUnterminatedString produces a craft/syntax/unterminated-string diagnostic.
// tok is the TokenError produced by the lexer; tok.Column is the 1-based column
// of the opening `"`, and tok.Value is the partial content (no quotes).
// The range spans from the opening `"` through the last consumed character.
func (p *Parser) diagUnterminatedString(tok lexer.Token) craft.Diagnostic {
	line := tok.Line - 1
	if line < 0 {
		line = 0
	}
	col := tok.Column - 1
	if col < 0 {
		col = 0
	}
	// +1 for the opening `"` that is part of the token but not in tok.Value.
	end := col + 1 + len([]rune(tok.Value))
	return craft.Diagnostic{
		Code:     "craft/syntax/unterminated-string",
		Message:  fmt.Sprintf("unterminated string literal %q", tok.Value),
		Severity: craft.SeverityError,
		Range: craft.Range{
			Start: craft.Position{Line: line, Character: col},
			End:   craft.Position{Line: line, Character: end},
		},
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
func (p *Parser) parseExposureBlock() (*SyntaxNode, *ast.ExposureDecl, []craft.Diagnostic) {
	node := &SyntaxNode{Kind: SyntaxKindExposureDecl}
	var diags []craft.Diagnostic

	// Collect leading trivia before `exposure`.
	for _, t := range p.collectTrivia() {
		node.Children = append(node.Children, t)
	}

	kwTok := p.peek()
	kwLine := kwTok.Line
	node.Children = append(node.Children, p.consumeAs(SyntaxKindKwExposure))

	// Exposure name: any identifier or keyword-used-as-identifier (including "default").
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isKeywordUsedAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "exposure name"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	name := nameTok.Value
	node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		return node, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLBrace))

	exp := &ast.ExposureDecl{Name: name, Line: kwLine}

	// throughRuleNode accumulates a single DeploymentRule node for the through: field.
	// We defer creation until we actually find `through:`.
	var throughRuleNode *SyntaxNode

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(tok, "field name (to, contexts, through) or `}`"))
			node.Children = append(node.Children, p.consumeAs(SyntaxKindError))
			continue
		}
		fieldName := tok.Value

		// Consume field name as contextual keyword.
		switch fieldName {
		case "to":
			node.Children = append(node.Children, p.consumeAs(SyntaxKindKwTo))
		case "contexts":
			node.Children = append(node.Children, p.consumeAs(SyntaxKindKwContexts))
		case "through":
			node.Children = append(node.Children, p.consumeAs(SyntaxKindKwThrough))
		default:
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		}

		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			continue
		}
		node.Children = append(node.Children, p.consumeAs(SyntaxKindColon))

		switch fieldName {
		case "to":
			exp.To = p.parseIdentListIntoNode(node)
		case "contexts":
			exp.Contexts = p.parseIdentListIntoNode(node)
		case "through":
			// Build a DeploymentRule node containing the `through` keyword token (already added)
			// and value tokens.
			if throughRuleNode == nil {
				// Rebuild: the `through` kw and `:` tokens were already added to node.Children,
				// so we create a new DeploymentRule node that holds only the value tokens.
				// For structural clarity, the DeploymentRule node holds the `through` keyword
				// and value, so we must wrap them. We'll remove the last 2 tokens from
				// node.Children (through kw + colon) and put them into the rule node instead.
				throughRuleNode = &SyntaxNode{Kind: SyntaxKindDeploymentRule}
				// Move the last 2 children (through kw + colon) from node to throughRuleNode.
				n := len(node.Children)
				throughRuleNode.Children = append(throughRuleNode.Children, node.Children[n-2], node.Children[n-1])
				node.Children = node.Children[:n-2]
			}
			exp.Through = p.parseIdentListIntoNode(throughRuleNode)
			node.Children = append(node.Children, throughRuleNode)
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
		return node, exp, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRBrace))
	return node, exp, diags
}

// parseIdentListIntoNode parses a comma-separated ident list and appends tokens to node.
func (p *Parser) parseIdentListIntoNode(node *SyntaxNode) []string {
	var items []string
	for {
		tok := p.peek()
		var val string
		switch {
		case tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString:
			val = tok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		case isKeywordUsedAsIdent(tok.Type):
			val = tok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		default:
			return items
		}
		items = append(items, val)
		if p.peek().Type == lexer.TokenComma {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindComma))
		} else {
			break
		}
	}
	return items
}

// parseIdentListWithLinesIntoNode parses a comma-separated ident list, appends tokens to node,
// and returns both the values and their 1-based source lines.
// Used for service contexts: field so that go-to-definition can match the cursor line.
func (p *Parser) parseIdentListWithLinesIntoNode(node *SyntaxNode) ([]string, []int) {
	var items []string
	var lines []int
	for {
		tok := p.peek()
		var val string
		switch {
		case tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString:
			val = tok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		case isKeywordUsedAsIdent(tok.Type):
			val = tok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		default:
			return items, lines
		}
		items = append(items, val)
		lines = append(lines, tok.Line)
		if p.peek().Type == lexer.TokenComma {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindComma))
		} else {
			break
		}
	}
	return items, lines
}

// parseDeploymentSpecIntoNode parses deployment spec and appends tokens to node.
func (p *Parser) parseDeploymentSpecIntoNode(node *SyntaxNode) (string, []ast.DeploymentRule, []craft.Diagnostic) {
	var diags []craft.Diagnostic

	typeTok := p.peek()
	var dt string
	if typeTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(typeTok.Type) {
		dt = typeTok.Value
		node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
	} else {
		diags = append(diags, p.diagUnexpected(typeTok, "deployment type identifier"))
		return "", nil, diags
	}

	if p.peek().Type != lexer.TokenLParen {
		return dt, nil, diags
	}
	node.Children = append(node.Children, p.consumeAs(SyntaxKindLParen))

	var rules []ast.DeploymentRule
	for !p.atEOF() && p.peek().Type != lexer.TokenRParen {
		pctTok := p.peek()
		if pctTok.Type != lexer.TokenPercentage {
			diags = append(diags, p.diagUnexpected(pctTok, "percentage (e.g. 90%)"))
			p.consume()
			continue
		}
		pct := pctTok.Value
		node.Children = append(node.Children, p.consumeAs(SyntaxKindPercentage))

		if p.peek().Type != lexer.TokenArrow {
			diags = append(diags, p.diagUnexpected(p.peek(), "->"))
			p.consume()
			continue
		}
		node.Children = append(node.Children, p.consumeAs(SyntaxKindArrow))

		targetTok := p.peek()
		var target string
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			target = targetTok.Value
			node.Children = append(node.Children, p.consumeAs(SyntaxKindIdent))
		} else {
			diags = append(diags, p.diagUnexpected(targetTok, "deployment target identifier"))
		}

		rules = append(rules, ast.DeploymentRule{Percentage: pct, Target: target})

		if p.peek().Type == lexer.TokenComma {
			node.Children = append(node.Children, p.consumeAs(SyntaxKindComma))
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
	node.Children = append(node.Children, p.consumeAs(SyntaxKindRParen))
	return dt, rules, diags
}
