// Package syntax implements the hand-written recursive-descent parser for the
// Craft DSL. It produces a lossless syntax tree by emitting events into a
// green.GreenNodeBuilder.
//
// S3: actors. S4: domains. S5: services + services block.
// S6: use_case "..." { when ... } blocks.
// S7: arch { presentation: ... gateway: ... } blocks with flow (>) and
//
//	component modifiers ([key, key:value]).
//
// S8: exposure <name> { to: ... contexts: ... through: ... } blocks.
// Unsupported top-level keywords emit a recoverable "not-yet-implemented"
// diagnostic so --parser=v2 is usable on partial files.
package syntax

import (
	"fmt"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lexer"
	"github.com/tcarcao/craft/v2/internal/model"
)

// Parser is a recursive-descent parser for Craft DSL.
// It emits events into a GreenNodeBuilder rather than constructing SyntaxNodes directly.
type Parser struct {
	tokens  []lexer.Token
	pos     int
	builder green.GreenNodeBuilder
	src     string
	li      green.LineIndex
	prevEnd green.TextSize // byte offset after last emitted token
}

// Parse parses src and returns the green root, a LineIndex, and any diagnostics.
// A non-nil GreenNode is always returned (island parsing preserved).
func Parse(src string) (*green.GreenNode, green.LineIndex, []model.Diagnostic) {
	li := green.NewLineIndex(src)
	l := lexer.New(src)
	p := &Parser{tokens: l.All(), src: src, li: li}
	root, diags := p.parseFile()
	return root, li, diags
}

// --- main parse loop ---

func (p *Parser) parseFile() (*green.GreenNode, []model.Diagnostic) {
	p.builder.StartNode(SyntaxKindFile)
	var diags []model.Diagnostic

	// Global counter for scenario_N / action_N IDs across all use_cases in the file,
	// matching ANTLR's numbering scheme.
	ucCounter := 0

	for !p.atEOF() {
		tok := p.peek()
		switch tok.Type {
		case lexer.TokenKwImport:
			diags = append(diags, p.parseImportStatement()...)
		case lexer.TokenKwActor:
			diags = append(diags, p.parseActorStatement()...)
		case lexer.TokenKwActors:
			diags = append(diags, p.parseActorsBlock()...)
		case lexer.TokenKwDomain:
			diags = append(diags, p.parseDomainStatement()...)
		case lexer.TokenKwDomains:
			diags = append(diags, p.parseDomainsBlock()...)
		case lexer.TokenKwService:
			diags = append(diags, p.parseServiceStatement()...)
		case lexer.TokenKwServices:
			diags = append(diags, p.parseServicesBlock()...)
		case lexer.TokenKwUseCase:
			diags = append(diags, p.parseUseCaseBlock(&ucCounter)...)
		case lexer.TokenKwArch:
			diags = append(diags, p.parseArchBlock()...)
		case lexer.TokenKwExposure:
			diags = append(diags, p.parseExposureBlock()...)
		case lexer.TokenKwContextMap:
			diags = append(diags, p.parseContextMapBlock()...)
		default:
			// Unrecognised top-level token: emit a diagnostic and resync to
			// the next top-level keyword (island parsing).
			diags = append(diags, p.diagNotImplemented(tok))
			p.resyncToTopLevel()
		}
	}

	// Capture trailing whitespace/newlines after the last token.
	if int(p.prevEnd) < len(p.src) {
		p.builder.Token(SyntaxKindWhitespace, p.src[p.prevEnd:])
	}

	p.builder.FinishNode()
	return p.builder.Finish(), diags
}

// parseActorStatement parses: actor <type> <name>
func (p *Parser) parseActorStatement() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindActorDecl)
	var diags []model.Diagnostic

	// Attach leading trivia (comments).
	p.attachTrivia()

	// Consume `actor` keyword.
	p.consumeAs(SyntaxKindKwActor)

	typeTok := p.peek()
	_, ok := tokenToActorType(typeTok)
	if !ok {
		diags = append(diags, p.diagUnexpected(typeTok, "actor type"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	// Map the actor-type token to the right SyntaxKind.
	typeKind := SyntaxKindIdent
	if k, found := lexerKindToSyntaxKindMap[typeTok.Type]; found {
		typeKind = k
	}
	p.consumeAs(typeKind)

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent {
		diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindIdent)

	p.builder.FinishNode()
	return diags
}

// parseImportStatement parses: import "<path>"
func (p *Parser) parseImportStatement() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindImportDecl)
	var diags []model.Diagnostic

	p.attachTrivia()
	p.consumeAs(SyntaxKindKwImport)

	pathTok := p.peek()
	if pathTok.Type != lexer.TokenString {
		diags = append(diags, p.diagUnexpected(pathTok, "import path string (e.g. \"other.craft\")"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindString)

	p.builder.FinishNode()
	return diags
}

// parseActorsBlock parses: actors { <actor_definition>* }
func (p *Parser) parseActorsBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindActorsBlock)
	var diags []model.Diagnostic

	// Attach leading trivia before `actors`.
	p.attachTrivia()

	p.consumeAs(SyntaxKindKwActors) // consume `actors`

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside block.
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

		tok := p.peek()
		_, ok := tokenToActorType(tok)
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
		p.builder.StartNode(SyntaxKindActorDecl)
		typeKind := SyntaxKindIdent
		if k, found := lexerKindToSyntaxKindMap[tok.Type]; found {
			typeKind = k
		}
		p.consumeAs(typeKind)

		nameTok := p.peek()
		if nameTok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(nameTok, "actor name"))
			p.consume()
			p.builder.FinishNode()
			continue
		}
		p.consumeAs(SyntaxKindIdent)
		p.builder.FinishNode()
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed actors block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseDomainStatement parses: domain <name> { <bounded_context>* }
func (p *Parser) parseDomainStatement() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindDomainDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `domain`.
	p.attachTrivia()

	p.consumeAs(SyntaxKindKwDomain)

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isDomainNameToken(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "domain name"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindIdent)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	d := p.parseBoundedContextList()
	diags = append(diags, d...)

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domain block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(nameTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)

	p.builder.FinishNode()
	return diags
}

// parseDomainsBlock parses: domains { <domain_block>* }
// where each domain_block is: <name> { <bounded_context>* }
func (p *Parser) parseDomainsBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindDomainsBlock)
	var diags []model.Diagnostic

	// Attach leading trivia before `domains`.
	p.attachTrivia()
	p.consumeAs(SyntaxKindKwDomains)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside block.
		p.attachTrivia()
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
		p.builder.StartNode(SyntaxKindDomainDecl)
		p.consumeAs(SyntaxKindIdent)

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			p.builder.FinishNode()
			continue
		}
		p.consumeAs(SyntaxKindLBrace)

		d := p.parseBoundedContextList()
		diags = append(diags, d...)

		if p.atEOF() {
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  "unclosed domain block (missing `}`)",
				Severity: model.SeverityError,
				Range:    tokenRange(nameTok),
			})
			p.builder.FinishNode()
			p.builder.FinishNode()
			return diags
		}
		p.consumeAs(SyntaxKindRBrace)
		p.builder.FinishNode()
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domains block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseBoundedContextList parses a list of identifiers until `}` or EOF.
// These are the bounded context names inside a domain block.
// Duplicates are silently deduplicated (keeping first occurrence), matching
// ANTLR behavior and the v1 spec.
// Emits BoundedContext nodes into the current builder scope.
func (p *Parser) parseBoundedContextList() []model.Diagnostic {
	seen := make(map[string]bool)
	var diags []model.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside the context list.
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

		tok := p.peek()
		if tok.Type == lexer.TokenIdent || isDomainNameToken(tok.Type) {
			if !seen[tok.Value] {
				seen[tok.Value] = true
				p.builder.StartNode(SyntaxKindBoundedContext)
				p.consumeAs(SyntaxKindIdent)
				p.builder.FinishNode()
			} else {
				// Duplicate: consume but do not wrap (matches ANTLR dedup behavior).
				p.consume()
			}
		} else if tok.Type == lexer.TokenError {
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			p.consume()
		} else {
			// Unknown token inside domain block — could be a sub-keyword; skip.
			diags = append(diags, p.diagUnexpected(tok, "bounded context name or `}`"))
			if tok.Type == lexer.TokenLBrace {
				p.resyncToBlock()
			} else {
				p.consume()
			}
		}
	}
	return diags
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
func (p *Parser) parseServicesBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindServicesBlock)
	var diags []model.Diagnostic

	// Attach leading trivia before `services`.
	p.attachTrivia()
	p.consumeAs(SyntaxKindKwServices)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside block.
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}

		tok := p.peek()

		// Service name: identifier, string literal, or keyword-as-name
		var name string
		var nameLine int
		var nameKind SyntaxKind
		switch tok.Type {
		case lexer.TokenIdent:
			name = tok.Value
			nameLine = tok.Line
			nameKind = SyntaxKindIdent
		case lexer.TokenString:
			name = tok.Value
			nameLine = tok.Line
			nameKind = SyntaxKindString
		default:
			if isServiceNameKeyword(tok.Type) {
				name = tok.Value
				nameLine = tok.Line
				if k, found := lexerKindToSyntaxKindMap[tok.Type]; found {
					nameKind = k
				} else {
					nameKind = SyntaxKindIdent
				}
			} else {
				diags = append(diags, p.diagUnexpected(tok, "service name"))
				p.consume()
				continue
			}
		}

		// Build the child service node.
		p.builder.StartNode(SyntaxKindServiceDecl)
		p.consumeAs(nameKind)

		if p.peek().Type != lexer.TokenLBrace {
			diags = append(diags, p.diagUnexpected(p.peek(), "{"))
			p.builder.FinishNode()
			continue
		}
		p.consumeAs(SyntaxKindLBrace)

		d := p.parseServiceBody()
		diags = append(diags, d...)

		if p.atEOF() {
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
				Severity: model.SeverityError,
				Range: model.Range{
					Start: model.Position{Line: lspLine(nameLine)},
					End:   model.Position{Line: lspLine(nameLine)},
				},
			})
			p.builder.FinishNode()
			p.builder.FinishNode()
			return diags
		}
		p.consumeAs(SyntaxKindRBrace)
		p.builder.FinishNode()
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed services block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(p.peek()),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseServiceStatement parses: service <name> { <field>* }
// This is the singular top-level service form (Q11).
func (p *Parser) parseServiceStatement() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindServiceDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `service`.
	p.attachTrivia()
	p.consumeAs(SyntaxKindKwService)

	nameTok := p.peek()
	var name string
	var nameLine int
	if nameTok.Type == lexer.TokenIdent || nameTok.Type == lexer.TokenString {
		name = nameTok.Value
		nameLine = nameTok.Line
		nameKind := SyntaxKindIdent
		if nameTok.Type == lexer.TokenString {
			nameKind = SyntaxKindString
		}
		p.consumeAs(nameKind)
	} else {
		diags = append(diags, p.diagUnexpected(nameTok, "service name"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	d := p.parseServiceBody()
	diags = append(diags, d...)

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
			Severity: model.SeverityError,
			Range: model.Range{
				Start: model.Position{Line: lspLine(nameLine)},
				End:   model.Position{Line: lspLine(nameLine)},
			},
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseServiceBody parses the fields inside a service { ... } block.
// Each field is wrapped in a SyntaxKindServiceField node so that tree-based
// completion context detection can identify which field the cursor is in.
func (p *Parser) parseServiceBody() []model.Diagnostic {
	var diags []model.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		if tok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(tok, "field name (contexts, data-stores, language) or `}`"))
			if tok.Type == lexer.TokenLBrace {
				p.resyncToBlock()
			} else {
				p.consume()
			}
			continue
		}

		fieldName := tok.Value

		p.builder.StartNode(SyntaxKindServiceField)

		switch fieldName {
		case "contexts":
			p.consumeAs(SyntaxKindKwContexts)
		case "data-stores":
			p.consumeAs(SyntaxKindKwDataStores)
		case "language":
			p.consumeAs(SyntaxKindKwLanguage)
		case "deployment":
			p.consumeAs(SyntaxKindKwDeployment)
		case "opslevel":
			p.consumeAs(SyntaxKindKwOpsLevel)
		case "repo":
			p.consumeAs(SyntaxKindKwRepo)
		default:
			p.consumeAs(SyntaxKindIdent)
		}

		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			p.builder.FinishNode()
			// Consume the unexpected token if it cannot start a new field or end
			// the block, to avoid cascade diagnostics on the same position.
			if next := p.peek().Type; next != lexer.TokenIdent && next != lexer.TokenRBrace {
				p.consume()
			}
			continue
		}
		p.consumeAs(SyntaxKindColon)

		switch fieldName {
		case "contexts":
			p.parseIdentListWithLines()
		case "data-stores":
			p.parseIdentList()
		case "language":
			if p.peek().Type == lexer.TokenIdent {
				p.consumeAs(SyntaxKindIdent)
			} else {
				diags = append(diags, p.diagUnexpected(p.peek(), "language identifier"))
			}
		case "deployment":
			dd := p.parseDeploymentSpec()
			diags = append(diags, dd...)
		case "opslevel":
			if p.peek().Type == lexer.TokenIdent {
				p.consumeAs(SyntaxKindIdent)
			} else {
				diags = append(diags, p.diagUnexpected(p.peek(), "opslevel identifier"))
			}
		case "repo":
			p.parseRef()
		default:
			// Unknown field — emit diagnostic and skip to next line.
			diags = append(diags, p.diagUnexpected(tok, "field name (contexts, data-stores, language) or `}`"))
			p.skipToNextField()
		}

		p.builder.FinishNode()
	}

	return diags
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
// Depth-aware: nested { } pairs are skipped as a unit so a value like
// `field: {broken}` does not cause the outer block's `}` to be consumed early.
func (p *Parser) skipToNextField() {
	depth := 0
	for !p.atEOF() {
		tok := p.peek()
		if tok.Type == lexer.TokenLBrace {
			depth++
			p.consume()
			continue
		}
		if tok.Type == lexer.TokenRBrace {
			if depth > 0 {
				depth--
				p.consume()
				continue
			}
			return // stop before the enclosing block's `}`
		}
		if depth == 0 && tok.Type == lexer.TokenIdent {
			return // next field name
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
func (p *Parser) parseUseCaseBlock(counter *int) []model.Diagnostic {
	p.builder.StartNode(SyntaxKindUseCaseDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `use_case`.
	p.attachTrivia()

	ucTok := p.peek()
	p.consumeAs(SyntaxKindKwUseCase)

	// Expect a quoted string name.
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenString {
		diags = append(diags, p.diagUnexpected(nameTok, "use_case name string"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	name := nameTok.Value
	p.consumeAs(SyntaxKindString)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// `when` is a contextual keyword that lexes as TokenIdent.
		if tok.Type == lexer.TokenIdent && tok.Value == "when" {
			d := p.parseScenario(counter)
			diags = append(diags, d...)
		} else if tok.Type == lexer.TokenIdent && tok.Value == "tags" {
			diags = append(diags, p.parseUseCaseTagsBlock()...)
		} else {
			// Skip unknown tokens inside the use_case body.
			diags = append(diags, p.diagUnexpected(tok, "`when`, `tags`, or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed use_case block for %q (missing `}`)", name),
			Severity: model.SeverityError,
			Range:    tokenRange(ucTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseUseCaseTagsBlock parses: tags { tag_stmt* }
// `tags` is a contextual keyword (matched by value from TokenIdent, like
// `when` elsewhere) — not a reserved word. Mirrors parseContextMapBlock
// exactly: StartNode, consume keyword, consume `{`, loop parseTagStmt with a
// forward-progress guard, unclosed-block diagnostic, consume `}`, FinishNode.
func (p *Parser) parseUseCaseTagsBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindTagsBlock)
	var diags []model.Diagnostic

	// Attach leading trivia before `tags`.
	p.attachTrivia()
	kwTok := p.peek()
	p.consumeAs(SyntaxKindKwTags)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside block (also swallows insignificant whitespace/newlines).
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}
		posBefore := p.pos
		diags = append(diags, p.parseTagStmt()...)
		// Belt-and-suspenders forward-progress guard, mirroring
		// parseContextMapBlock's loop.
		if p.pos == posBefore {
			diags = append(diags, p.diagUnexpected(p.peek(), "a tag key or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed tags block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseTagStmt parses one `IDENT ':' (IDENT | STRING | ref-shaped-slug)` tag
// line inside a tags { } block, emitting it as a SyntaxKindTagStmt node.
//
// The value is either a quoted string (consumed as a single SyntaxKindString
// token) or a bare value, which reuses parseRef's exact token-consuming
// logic — the same "ref-shaped" scanner node slugs use elsewhere in this
// parser — so a bare slug like "re/renewal-flow" (three lexer tokens: ident
// "re", the lexer's TokenError "/", ident "renewal-flow") is captured whole
// inside a SyntaxKindRef child node rather than truncated to its first
// token. This mirrors the same multi-token-value problem already solved for
// context_map edge endpoints, notifies/listens events, etc. (see
// refAwareText in ast.go) and keeps the green tree exactly lossless.
func (p *Parser) parseTagStmt() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindTagStmt)
	var diags []model.Diagnostic

	keyTok := p.peek()
	if keyTok.Type != lexer.TokenIdent {
		diags = append(diags, p.diagUnexpected(keyTok, "a tag key"))
		p.consumeAs(SyntaxKindError)
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindIdent)

	if p.peek().Type != lexer.TokenColon {
		diags = append(diags, p.diagUnexpected(p.peek(), "`:`"))
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindColon)

	valTok := p.peek()
	switch {
	case valTok.Type == lexer.TokenString:
		p.consumeAs(SyntaxKindString)
	case valTok.Type == lexer.TokenIdent:
		// Bare value — reuse parseRef's scanner so a slash/hyphen-bearing
		// slug like "re/renewal-flow" is captured whole.
		p.parseRef()
	default:
		diags = append(diags, p.diagUnexpected(valTok, "a tag value (identifier, string, or ref)"))
		p.builder.FinishNode()
		return diags
	}

	p.builder.FinishNode()
	return diags
}

// parseScenario parses one `when <trigger>` clause plus its following action lines.
// counter is a shared global ID counter (pointer) for scenario_N / action_N IDs,
// matching ANTLR's numbering scheme where both scenarios and actions share one counter.
func (p *Parser) parseScenario(counter *int) []model.Diagnostic {
	p.builder.StartNode(SyntaxKindScenario)
	var diags []model.Diagnostic

	// consume `when` as contextual keyword
	whenTok := p.peek()
	p.consumeAs(SyntaxKindKwWhen)

	d := p.parseTrigger(whenTok.Line)
	diags = append(diags, d...)

	*counter++

	// Parse actions until we see `when` (next scenario), `}` (end of use_case), or EOF.
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		// `when` starts the next scenario — stop here.
		if tok.Type == lexer.TokenIdent && tok.Value == "when" {
			break
		}
		d := p.parseAction(counter)
		diags = append(diags, d...)
	}

	p.builder.FinishNode()
	return diags
}

// parseTrigger parses the `<actor/domain> <verb> <phrase>` part after `when`.
// Two forms:
//   - external:      `when <actor> <verb> <phrase>`
//   - domain_listen: `when <domain> listens "<event>"`
func (p *Parser) parseTrigger(whenLine int) []model.Diagnostic {
	p.builder.StartNode(SyntaxKindTrigger)
	var diags []model.Diagnostic

	// event trigger: when "<EventName>"  (no subject identifier)
	if p.peek().Type == lexer.TokenString {
		p.consumeAs(SyntaxKindString)
		p.builder.FinishNode()
		return diags
	}

	// The first token is the actor/domain subject.
	subjectTok := p.peek()

	// cron trigger: when cron "0 * * * *"
	if subjectTok.Type == lexer.TokenIdent && subjectTok.Value == "cron" {
		p.consumeAs(SyntaxKindKwCron)
		if p.peek().Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "cron expression string (e.g. \"0 * * * *\")"))
		}
		p.builder.FinishNode()
		return diags
	}

	// periodic trigger: when every "1h"
	if subjectTok.Type == lexer.TokenIdent && subjectTok.Value == "every" {
		p.consumeAs(SyntaxKindKwEvery)
		if p.peek().Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "duration string (e.g. \"1h\")"))
		}
		p.builder.FinishNode()
		return diags
	}

	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		diags = append(diags, p.diagUnexpected(subjectTok, "trigger subject (actor/domain name)"))
		p.consumeAs(SyntaxKindError) // consume bad token to unblock the action loop
		p.builder.FinishNode()
		return diags
	}
	// Always emit the trigger subject as SyntaxKindIdent so that ActorName() /
	// ContextName() (which call ChildToken(SyntaxKindIdent)) find it correctly,
	// even when the lexer classifies it as a keyword (e.g. "Actor" → TokenKwActor).
	p.consumeAs(SyntaxKindIdent)

	// The second token is the verb.  If it is `listens` (ident), this is domain_listen.
	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb token — treat as a partial trigger.
		p.builder.FinishNode()
		return diags
	}
	verb := verbTok.Value

	if verb == "listens" {
		p.consumeAs(SyntaxKindKwListens)
		// domain_listen: when <domain> listens "<event>" | <ref>
		eventTok := p.peek()
		if eventTok.Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else if eventTok.Type == lexer.TokenIdent {
			p.parseRef()
		}
		p.builder.FinishNode()
		return diags
	}

	// external: when <actor> <verb> [connector_word] <phrase>
	p.consumeAs(SyntaxKindIdent)

	// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == verbTok.Line {
		p.consumeAs(SyntaxKindIdent)
	}
	// Collect phrase tokens on the same line.
	p.collectPhrase(verbTok.Line)

	p.builder.FinishNode()
	return diags
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
func (p *Parser) parseAction(counter *int) []model.Diagnostic {
	p.builder.StartNode(SyntaxKindAction)
	var diags []model.Diagnostic

	subjectTok := p.peek()
	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		// Not an action line — skip the token.
		diags = append(diags, p.diagUnexpected(subjectTok, "action subject (domain/service name) or `when`"))
		p.consumeAs(SyntaxKindError)
		p.builder.FinishNode()
		return diags
	}
	actionLine := subjectTok.Line
	// Always emit the action subject as SyntaxKindIdent so that SubjectName()
	// (which calls ChildToken(SyntaxKindIdent)) finds it correctly, even when
	// the lexer classifies it as a keyword (e.g. "Service" → TokenKwService).
	p.consumeAs(SyntaxKindIdent)

	verbTok := p.peek()
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb — treat as minimal internal action.
		*counter++
		*counter++
		p.builder.FinishNode()
		return diags
	}
	verb := verbTok.Value

	*counter++

	switch verb {
	case "asks":
		p.consumeAs(SyntaxKindKwAsks)
		p.parseAsksAction(actionLine)
		p.builder.FinishNode()
		return diags
	case "notifies":
		p.consumeAs(SyntaxKindKwNotifies)
		d := p.parseNotifiesAction()
		diags = append(diags, d...)
		p.builder.FinishNode()
		return diags
	case "returns":
		p.consumeAs(SyntaxKindKwReturns)
		p.parseReturnsAction(actionLine)
		p.builder.FinishNode()
		return diags
	default:
		// internal_action: <domain> <verb> [connector_word] <phrase>
		p.consumeAs(SyntaxKindIdent)
		// connector_word matches ANTLR grammar: a|an|the|as|to|from|in|on|at|for|with|by
		connTok := p.peek()
		if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == actionLine {
			p.consumeAs(SyntaxKindIdent)
		}
		p.collectPhrase(actionLine)
		p.builder.FinishNode()
		return diags
	}
}

// parseAsksAction parses: <target> to|for <phrase>
// The `asks` keyword token has already been consumed by parseAction.
func (p *Parser) parseAsksAction(line int) {
	targetTok := p.peek()
	if targetTok.Type == lexer.TokenIdent {
		p.parseRef()
	} else if isAnyKeywordAsIdent(targetTok.Type) {
		p.consumeAs(SyntaxKindIdent)
	}

	// connector: "to" or "for"
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && (connTok.Value == "to" || connTok.Value == "for") {
		if connTok.Value == "to" {
			p.consumeAs(SyntaxKindKwTo)
		} else {
			p.consumeAs(SyntaxKindIdent)
		}
	}

	p.collectPhrase(line)
}

// parseNotifiesAction parses the "<event>" or event-ident after `notifies`.
func (p *Parser) parseNotifiesAction() []model.Diagnostic {
	var diags []model.Diagnostic
	eventTok := p.peek()
	if eventTok.Type == lexer.TokenString {
		p.consumeAs(SyntaxKindString)
	} else if eventTok.Type == lexer.TokenIdent {
		p.parseRef()
	} else if isAnyKeywordAsIdent(eventTok.Type) {
		p.consumeAs(SyntaxKindIdent)
	} else if eventTok.Type == lexer.TokenError {
		diags = append(diags, p.diagUnterminatedString(eventTok))
		p.consumeAs(SyntaxKindError)
	}
	return diags
}

// parseReturnsAction parses [to <target>] [connector_word] <phrase> after `returns`.
func (p *Parser) parseReturnsAction(line int) {
	// Check for optional `to <target>`
	if p.peek().Type == lexer.TokenIdent && p.peek().Value == "to" {
		p.consumeAs(SyntaxKindKwTo)
		targetTok := p.peek()
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			p.consumeAs(SyntaxKindIdent)
		}
	}

	// Optional connector_word before phrase (ANTLR grammar: return_action connector_word? phrase)
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == line {
		p.consumeAs(SyntaxKindIdent)
	}

	p.collectPhrase(line)
}

// collectPhrase emits phrase tokens on actionLine into the current builder scope.
//
// The prose tail is display-only free text (see ActionDecl.PhraseText), so every
// same-line token is swept in verbatim — including TokenError punctuation such as
// `!`, `&`, `*`, `/` — until a line change, `}`, EOF, or a comment token ends it.
// (peek() already skips comment tokens transparently when scanning for the next
// token, so a trailing `//...` comment naturally falls on a later line or is
// filtered before the line check below; the explicit comment case exists for
// clarity/defensiveness even though peek() means it is not normally reachable.)
func (p *Parser) collectPhrase(actionLine int) {
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return
	}
	if p.peek().Line != actionLine {
		return
	}
	for {
		tok := p.peek()
		if tok.Type == lexer.TokenRBrace || tok.Type == lexer.TokenEOF {
			return
		}
		if tok.Line != actionLine {
			return
		}
		switch tok.Type {
		case lexer.TokenLineComment, lexer.TokenDocComment, lexer.TokenBlockComment:
			return // trailing comment ends prose; trivia attaches normally
		case lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case lexer.TokenNumber:
			p.consumeAs(SyntaxKindNumber)
		default:
			// Idents, keywords-as-idents, and TokenError punctuation (! & * / # …)
			// are all swept into prose as raw tokens.
			p.consumeAs(SyntaxKindIdent)
		}
	}
}

// --- arch parsing ---

// parseArchBlock parses: arch <name>? { <arch_sections> }
// where arch_sections is one or more presentation: or gateway: labelled lists.
func (p *Parser) parseArchBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindArchDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `arch`.
	p.attachTrivia()

	archTok := p.peek()
	p.consumeAs(SyntaxKindKwArch)

	// Optional name: an identifier that is NOT `{`.
	if p.peek().Type == lexer.TokenIdent || isAnyKeywordAsIdent(p.peek().Type) {
		if p.peek().Type != lexer.TokenLBrace {
			p.consumeAs(SyntaxKindIdent)
		}
	}

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	// Parse sections until `}` or EOF.
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()

		// Section label detection: identifier followed by `:` at this position.
		// presentation: or gateway: are the only valid section labels.
		if (tok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(tok.Type)) && p.peekAt(1).Type == lexer.TokenColon {
			label := tok.Value
			labelLine := tok.Line

			p.builder.StartNode(SyntaxKindArchSection)

			// Consume label as contextual keyword.
			switch label {
			case "presentation":
				p.consumeAs(SyntaxKindKwPresentation)
			case "gateway":
				p.consumeAs(SyntaxKindKwGateway)
			default:
				p.consumeAs(SyntaxKindIdent)
			}
			p.consumeAs(SyntaxKindColon)

			d := p.parseArchComponentList()
			diags = append(diags, d...)
			p.builder.FinishNode()

			if label != "presentation" && label != "gateway" {
				// Unknown section label — warn.
				diags = append(diags, model.Diagnostic{
					Code:     "craft/syntax/unknown-arch-section",
					Message:  fmt.Sprintf("unknown arch section %q; expected presentation or gateway", label),
					Severity: model.SeverityWarning,
					Range:    model.Range{Start: model.Position{Line: lspLine(labelLine)}},
				})
			}
		} else {
			// Unexpected token in arch body — skip.
			diags = append(diags, p.diagUnexpected(tok, "arch section label (presentation or gateway) or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed arch block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(archTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseArchComponentList parses an arch component list, emitting ArchComponent
// nodes into the current builder scope.
func (p *Parser) parseArchComponentList() []model.Diagnostic {
	var diags []model.Diagnostic

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

		d := p.parseArchComponent()
		diags = append(diags, d...)
	}
	return diags
}

// parseArchComponent parses a single component entry as an ArchComponent node.
// Flow chains (a > b > c) become a single ArchComponent node containing all
// sub-components and the `>` tokens (matching prior behavior).
func (p *Parser) parseArchComponent() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindArchComponent)
	var diags []model.Diagnostic

	// Parse the first component with optional modifiers.
	ok, d := p.parseComponentWithModifiers()
	diags = append(diags, d...)
	if !ok {
		p.builder.FinishNode()
		return diags
	}

	// Flow chain: collect all components separated by `>`.
	for p.peek().Type == lexer.TokenGT {
		p.consumeAs(SyntaxKindGT)
		_, d := p.parseComponentWithModifiers()
		diags = append(diags, d...)
	}

	p.builder.FinishNode()
	return diags
}

// parseComponentWithModifiers emits component tokens (name + optional modifiers)
// directly into the current builder scope. Returns true if a component was parsed.
func (p *Parser) parseComponentWithModifiers() (bool, []model.Diagnostic) {
	var diags []model.Diagnostic

	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "component name"))
		return false, diags
	}
	p.consumeAs(SyntaxKindIdent)

	if p.peek().Type == lexer.TokenLBracket {
		p.consumeAs(SyntaxKindLBracket)
		d := p.parseModifierList()
		diags = append(diags, d...)
		if p.peek().Type == lexer.TokenRBracket {
			p.consumeAs(SyntaxKindRBracket)
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "]"))
		}
	}

	return true, diags
}

// parseModifierList emits ArchModifier nodes into the current builder scope.
func (p *Parser) parseModifierList() []model.Diagnostic {
	var diags []model.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBracket {
		keyTok := p.peek()
		if keyTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(keyTok.Type) {
			diags = append(diags, p.diagUnexpected(keyTok, "modifier key"))
			p.consume()
			continue
		}
		p.builder.StartNode(SyntaxKindArchModifier)
		p.consumeAs(SyntaxKindIdent)

		if p.peek().Type == lexer.TokenColon {
			p.consumeAs(SyntaxKindColon)
			valTok := p.peek()
			switch valTok.Type {
			case lexer.TokenIdent:
				p.consumeAs(SyntaxKindIdent)
			case lexer.TokenString:
				p.consumeAs(SyntaxKindString)
			case lexer.TokenNumber, lexer.TokenPercentage:
				p.consumeAs(SyntaxKindNumber)
			default:
				if isAnyKeywordAsIdent(valTok.Type) {
					p.consumeAs(SyntaxKindIdent)
				} else {
					diags = append(diags, p.diagUnexpected(valTok, "modifier value (identifier, string, or number)"))
				}
			}
		}

		p.builder.FinishNode()

		if p.peek().Type == lexer.TokenComma {
			p.consume() // consume `,`
		} else {
			break
		}
	}
	return diags
}

// peekAt returns the Nth non-comment token at or after p.pos without advancing.
// offset=0 is equivalent to peek(); offset=1 is the token after that, etc.
// Comment tokens are skipped when counting.
func (p *Parser) peekAt(offset int) lexer.Token {
	count := 0
	for i := p.pos; i < len(p.tokens); i++ {
		tt := p.tokens[i].Type
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment || tt == lexer.TokenDocComment {
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

// tokenToActorType returns the actor-type string and true if tok is a valid
// actor type. Q5: any identifier is a valid actor type (open taxonomy).
func tokenToActorType(tok lexer.Token) (string, bool) {
	switch tok.Type {
	case lexer.TokenKwUser:
		return "user", true
	case lexer.TokenKwSystem:
		return "system", true
	case lexer.TokenKwService:
		return "service", true
	case lexer.TokenIdent:
		return tok.Value, true
	}
	if isAnyKeywordAsIdent(tok.Type) {
		return tok.Value, true
	}
	return "", false
}

// resyncToTopLevel discards tokens until it finds a known top-level keyword
// or EOF, so the main loop can continue from a clean state. The skipped tokens
// are wrapped in a SyntaxKindErrorNode so the lossless invariant is preserved
// and error recovery is visible in the tree.
func (p *Parser) resyncToTopLevel() {
	if p.atEOF() || isTopLevelKeyword(p.peek().Type) {
		return
	}
	p.builder.StartNode(SyntaxKindErrorNode)
	for !p.atEOF() {
		if isTopLevelKeyword(p.peek().Type) {
			break
		}
		p.consume()
	}
	p.builder.FinishNode()
}

// resyncToBlock wraps unrecognised tokens inside a construct in a
// SyntaxKindErrorNode, consuming until the nearest `}` at depth 0 or a
// top-level keyword (failsafe). The closing `}` is NOT consumed so the
// enclosing loop can handle it normally.
func (p *Parser) resyncToBlock() {
	p.builder.StartNode(SyntaxKindErrorNode)
	depth := 0
	for !p.atEOF() {
		tok := p.peek()
		if tok.Type == lexer.TokenLBrace {
			depth++
			p.consume()
			continue
		}
		if tok.Type == lexer.TokenRBrace {
			if depth > 0 {
				depth--
				p.consume()
				continue
			}
			break // stop before the block's own `}`
		}
		if depth == 0 && isTopLevelKeyword(tok.Type) {
			break // failsafe: don't skip past a new top-level construct
		}
		p.consume()
	}
	p.builder.FinishNode()
}

func isTopLevelKeyword(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwImport,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwService, lexer.TokenKwServices,
		lexer.TokenKwUseCase, lexer.TokenKwArch, lexer.TokenKwExposure,
		lexer.TokenKwContextMap:
		return true
	}
	return false
}

// peek returns the next non-comment token without advancing p.pos.
// Comment tokens are skipped transparently for parse decisions.
func (p *Parser) peek() lexer.Token {
	for i := p.pos; i < len(p.tokens); i++ {
		tt := p.tokens[i].Type
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment || tt == lexer.TokenDocComment {
			continue
		}
		return p.tokens[i]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

// emitWhitespaceBefore emits a whitespace token for any gap between the last
// emitted token and the start of tok. This makes the green tree lossless.
func (p *Parser) emitWhitespaceBefore(tok lexer.Token) {
	if tok.Type == lexer.TokenEOF {
		return
	}
	currentStart := p.li.Offset(tok.Line, tok.Column)
	if currentStart > p.prevEnd {
		ws := p.src[p.prevEnd:currentStart]
		p.builder.Token(SyntaxKindWhitespace, ws)
		p.prevEnd = currentStart
	}
}

// updatePrevEnd records the byte end of the last emitted token.
func (p *Parser) updatePrevEnd(tok lexer.Token) {
	if tok.Type == lexer.TokenEOF {
		return
	}
	start := p.li.Offset(tok.Line, tok.Column)
	p.prevEnd = start + green.TextSize(len(tokenText(tok)))
}

// tokenText returns the exact raw source text for tok, for green-tree Text
// emission. For most token types this is tok.Value. TokenString is the
// exception: Value is the unescaped string CONTENT without surrounding
// quotes (kept as-is for content consumers — see EventValue()/Title()/etc.),
// while Raw carries the verbatim source slice including both quotes and any
// escape sequences. Using Value there would silently drop the quotes and
// corrupt the round-trip (Bug 1); Raw is what makes the green tree lossless.
func tokenText(tok lexer.Token) string {
	if tok.Type == lexer.TokenString && tok.Raw != "" {
		return tok.Raw
	}
	return tok.Value
}

// attachTrivia emits any line/block comment tokens at p.pos into the current
// builder scope, advancing p.pos past each one collected.
func (p *Parser) attachTrivia() {
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		switch tok.Type {
		case lexer.TokenLineComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindLineComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		case lexer.TokenBlockComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindBlockComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		case lexer.TokenDocComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindDocComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		default:
			return
		}
	}
}

// consume advances past the current non-trivia token and emits it into the builder
// using its lexer-mapped SyntaxKind. Trivia before it is attached automatically.
func (p *Parser) consume() lexer.Token {
	p.attachTrivia()
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	tok := p.tokens[p.pos]
	p.pos++
	p.emitWhitespaceBefore(tok)
	p.builder.Token(lexerKindToSyntaxKind(tok.Type), tokenText(tok))
	p.updatePrevEnd(tok)
	return tok
}

// consumeAs advances past the current token, emitting it into the builder with
// the given kind. Trivia before it is attached automatically.
func (p *Parser) consumeAs(kind SyntaxKind) lexer.Token {
	p.attachTrivia()
	if p.pos >= len(p.tokens) {
		return lexer.Token{Type: lexer.TokenEOF}
	}
	tok := p.tokens[p.pos]
	p.pos++
	p.emitWhitespaceBefore(tok)
	p.builder.Token(kind, tokenText(tok))
	p.updatePrevEnd(tok)
	return tok
}

func (p *Parser) atEOF() bool {
	return p.peek().Type == lexer.TokenEOF
}

// snapshot saves builder + parser position for speculative parsing.
func (p *Parser) snapshot() green.BuilderSnapshot {
	return green.BuilderSnapshot{
		ParentsLen:  len(p.builder.Parents()),
		ChildrenLen: len(p.builder.Children()),
		TokPos:      p.pos,
	}
}

// rollback restores builder + parser to a prior snapshot.
func (p *Parser) rollback(s green.BuilderSnapshot) {
	p.builder.SetParents(p.builder.Parents()[:s.ParentsLen])
	p.builder.SetChildren(p.builder.Children()[:s.ChildrenLen])
	p.pos = s.TokPos
}

// lexerKindToSyntaxKind maps a lexer.TokenType to its default SyntaxKind for
// emission into the green tree. Token types without a direct mapping fall back
// to SyntaxKindIdent.
func lexerKindToSyntaxKind(tt lexer.TokenType) SyntaxKind {
	if k, ok := lexerKindToSyntaxKindMap[tt]; ok {
		return k
	}
	return SyntaxKindIdent
}

// lexerKindToSyntaxKindMap maps lexer token types to SyntaxKind values for
// default emission. Several call sites also use this for keyword-specific kinds
// (e.g., mapping TokenKwUser to SyntaxKindKwUser when used as an actor type).
var lexerKindToSyntaxKindMap = map[lexer.TokenType]SyntaxKind{
	lexer.TokenKwImport:     SyntaxKindKwImport,
	lexer.TokenKwActor:      SyntaxKindKwActor,
	lexer.TokenKwActors:     SyntaxKindKwActors,
	lexer.TokenKwUser:       SyntaxKindKwUser,
	lexer.TokenKwSystem:     SyntaxKindKwSystem,
	lexer.TokenKwService:    SyntaxKindKwService,
	lexer.TokenKwDomain:     SyntaxKindKwDomain,
	lexer.TokenKwDomains:    SyntaxKindKwDomains,
	lexer.TokenKwServices:   SyntaxKindKwServices,
	lexer.TokenKwUseCase:    SyntaxKindKwUseCase,
	lexer.TokenKwArch:       SyntaxKindKwArch,
	lexer.TokenKwExposure:   SyntaxKindKwExposure,
	lexer.TokenKwContextMap: SyntaxKindKwContextMap,
	lexer.TokenIdent:        SyntaxKindIdent,
	lexer.TokenString:       SyntaxKindString,
	lexer.TokenNumber:       SyntaxKindNumber,
	lexer.TokenPercentage:   SyntaxKindPercentage,
	lexer.TokenLBrace:       SyntaxKindLBrace,
	lexer.TokenRBrace:       SyntaxKindRBrace,
	lexer.TokenLParen:       SyntaxKindLParen,
	lexer.TokenRParen:       SyntaxKindRParen,
	lexer.TokenLBracket:     SyntaxKindLBracket,
	lexer.TokenRBracket:     SyntaxKindRBracket,
	lexer.TokenColon:        SyntaxKindColon,
	lexer.TokenComma:        SyntaxKindComma,
	lexer.TokenGT:           SyntaxKindGT,
	lexer.TokenArrow:        SyntaxKindArrow,
	lexer.TokenError:        SyntaxKindError,
	lexer.TokenEOF:          SyntaxKindEOF,
	lexer.TokenLineComment:  SyntaxKindLineComment,
	lexer.TokenBlockComment: SyntaxKindBlockComment,
	lexer.TokenDocComment:   SyntaxKindDocComment,
}

func (p *Parser) diagUnexpected(tok lexer.Token, expected string) model.Diagnostic {
	return model.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  fmt.Sprintf("unexpected %q, expected %s", tok.Value, expected),
		Severity: model.SeverityError,
		Range:    tokenRange(tok),
	}
}

// diagUnterminatedString produces a craft/syntax/unterminated-string diagnostic.
// tok is the TokenError produced by the lexer; tok.Column is the 1-based column
// of the opening `"`, and tok.Value is the partial content (no quotes).
// The range spans from the opening `"` through the last consumed character.
func (p *Parser) diagUnterminatedString(tok lexer.Token) model.Diagnostic {
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
	return model.Diagnostic{
		Code:     "craft/syntax/unterminated-string",
		Message:  fmt.Sprintf("unterminated string literal %q", tok.Value),
		Severity: model.SeverityError,
		Range: model.Range{
			Start: model.Position{Line: line, Character: col},
			End:   model.Position{Line: line, Character: end},
		},
	}
}

func (p *Parser) diagNotImplemented(tok lexer.Token) model.Diagnostic {
	return model.Diagnostic{
		Code:     "craft/syntax/not-yet-implemented",
		Message:  fmt.Sprintf("construct starting with %q is not yet supported by parser v2; use --parser=antlr for full support", tok.Value),
		Severity: model.SeverityWarning,
		Range:    tokenRange(tok),
	}
}

// lspLine converts a 1-based source line to a 0-based LSP line number.
func lspLine(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
}

func tokenRange(tok lexer.Token) model.Range {
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
	return model.Range{
		Start: model.Position{Line: line, Character: col},
		End:   model.Position{Line: line, Character: end},
	}
}

// --- exposure parsing (S8) ---

// parseExposureBlock parses: exposure <name> { to: ... contexts: ... through: ... }
// Field keywords (to, contexts, through) are contextual identifiers per Q3.
func (p *Parser) parseExposureBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindExposureDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `exposure`.
	p.attachTrivia()

	kwTok := p.peek()
	p.consumeAs(SyntaxKindKwExposure)

	// Exposure name: any identifier or keyword-used-as-identifier (including "default").
	nameTok := p.peek()
	if nameTok.Type != lexer.TokenIdent && !isKeywordUsedAsIdent(nameTok.Type) {
		diags = append(diags, p.diagUnexpected(nameTok, "exposure name"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindIdent)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		tok := p.peek()
		if tok.Type != lexer.TokenIdent {
			diags = append(diags, p.diagUnexpected(tok, "field name (to, contexts, through) or `}`"))
			p.consumeAs(SyntaxKindError)
			continue
		}
		fieldName := tok.Value

		if fieldName == "through" {
			// Wrap the through field (kw + colon + value list) in a
			// DeploymentRule node via a checkpoint so it becomes a single subtree.
			cp := p.builder.Checkpoint()
			p.consumeAs(SyntaxKindKwThrough)

			if p.peek().Type != lexer.TokenColon {
				diags = append(diags, p.diagUnexpected(p.peek(), ":"))
				// Wrap what we have so far as the rule node and continue.
				p.builder.StartNodeAt(cp, SyntaxKindDeploymentRule)
				p.builder.FinishNode()
				continue
			}
			p.consumeAs(SyntaxKindColon)
			p.parseIdentList()
			p.builder.StartNodeAt(cp, SyntaxKindDeploymentRule)
			p.builder.FinishNode()
			continue
		}

		// Consume field name as contextual keyword.
		switch fieldName {
		case "to":
			p.consumeAs(SyntaxKindKwTo)
		case "contexts":
			p.consumeAs(SyntaxKindKwContexts)
		default:
			p.consumeAs(SyntaxKindIdent)
		}

		if p.peek().Type != lexer.TokenColon {
			diags = append(diags, p.diagUnexpected(p.peek(), ":"))
			continue
		}
		p.consumeAs(SyntaxKindColon)

		switch fieldName {
		case "to":
			p.parseIdentList()
		case "contexts":
			p.parseIdentList()
		default:
			p.skipToNextField()
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed exposure block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseContextMapBlock parses: context_map { edge_stmt* }
// edge_stmt := ref EDGE_KW ref, where EDGE_KW is a contextual keyword — one of
// the 8 DDD strategic context-mapping patterns (customer_supplier/conformist/
// anticorruption_layer/open_host_service/published_language/partnership/
// shared_kernel/separate_ways) — matched by value, like asks/notifies
// elsewhere (Q3). Endpoint resolution/kind validation is sema's job, not the
// parser's.
func (p *Parser) parseContextMapBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindContextMapDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `context_map`.
	p.attachTrivia()
	kwTok := p.peek()
	p.consumeAs(SyntaxKindKwContextMap)

	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindLBrace)

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		// Attach trivia inside block (also swallows insignificant whitespace/newlines).
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
			break
		}
		posBefore := p.pos
		diags = append(diags, p.parseEdgeStmt()...)
		// Belt-and-suspenders forward-progress guard: parseEdgeStmt (via
		// parseEdgeEndpoint) is written to always consume at least one
		// token, but if that invariant is ever broken by a future edit,
		// force one token of progress here rather than spinning forever on
		// the same position.
		if p.pos == posBefore {
			diags = append(diags, p.diagUnexpected(p.peek(), "a node reference, edge keyword, or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed context_map block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseEdgeStmt parses a single `ref EDGE_KW ref` edge statement inside a
// context_map block, emitting it as a SyntaxKindEdgeStmt node wrapping two
// SyntaxKindRef children (left/right endpoints) and one SyntaxKindEdgeKw
// token (the verb).
func (p *Parser) parseEdgeStmt() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindEdgeStmt)
	var diags []model.Diagnostic

	left := p.peek()
	if left.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(left.Type) {
		diags = append(diags, p.diagUnexpected(left, "a node reference (e.g. billing or re/billing)"))
		p.consumeAs(SyntaxKindError)
		p.builder.FinishNode()
		return diags
	}
	diags = append(diags, p.parseEdgeEndpoint()...) // left endpoint

	verb := p.peek()
	if verb.Type == lexer.TokenIdent && isEdgeKeyword(verb.Value) {
		p.consumeAs(SyntaxKindEdgeKw)
	} else {
		diags = append(diags, p.diagUnexpected(verb, "a relationship pattern: directional (customer_supplier/conformist/anticorruption_layer/open_host_service/published_language) or symmetric (partnership/shared_kernel/separate_ways)"))
		p.builder.FinishNode()
		return diags
	}

	right := p.peek()
	if right.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(right.Type) {
		diags = append(diags, p.diagUnexpected(right, "a node reference (e.g. billing or re/billing)"))
		p.builder.FinishNode()
		return diags
	}
	diags = append(diags, p.parseEdgeEndpoint()...) // right endpoint

	p.builder.FinishNode()
	return diags
}

// parseEdgeEndpoint parses one context_map edge endpoint (the left or right
// side of the edge verb). The caller has already gated entry to
// TokenIdent||isAnyKeywordAsIdent.
//
// It deliberately does NOT call parseRef() unconditionally the way the
// original Task 5 implementation did. parseRef()'s kind-prefix branch only
// consumes a token when it is immediately followed by ':', and its
// fallback continuation loop only recognises the TokenIdent/TokenNumber
// token TYPES — not the distinct keyword token types that keyword-as-ident
// words (e.g. "domain"/"service"/"bc"/"term") lex as. So calling parseRef
// on a BARE keyword-as-ident endpoint with no following ':' made it consume
// zero tokens; parseEdgeStmt then also made zero progress, and
// parseContextMapBlock's loop called it again on the same position forever
// — the Task 5 hang (`context_map { service realized_by service:x }`,
// `context_map { term:x contrasts domain }`).
//
// The fix mirrors the established idiom at the other parseRef call sites
// (parseAsksAction, parseNotifiesAction, parseTrigger's listens branch):
// only call parseRef() when the token is TokenIdent, or when a
// keyword-as-ident is immediately followed by ':' (parseRef's own
// kind-prefix branch handles that shape correctly, e.g. "service:x"). A
// bare keyword-as-ident with no colon is instead consumed directly here via
// consumeAs, guaranteeing forward progress without ever entering parseRef's
// zero-progress hole. This local fix — rather than patching the shared
// parseRef — keeps the other 3 parseRef call sites (which already silently
// accept a bare keyword-as-ident target/subject with no diagnostic, by
// design) completely unchanged.
//
// It additionally flags one specific shape as a diagnostic: a bare
// occurrence of one of the reserved node-slug kind words (domain/bc/term/
// service — see isSlugKind) with no following ':' is almost certainly a
// missing "`:name`" typo rather than an intentional unqualified reference,
// so it is reported here. This is a pure ref-shape/syntax check — it does
// NOT validate which endpoint kinds may legally connect to which (that
// endpoint-kind pairing validation is sema's job, Task 7, and is
// deliberately out of scope here).
func (p *Parser) parseEdgeEndpoint() []model.Diagnostic {
	var diags []model.Diagnostic
	tok := p.peek()
	if tok.Type == lexer.TokenIdent {
		p.parseRef()
		return diags
	}
	// keyword-as-ident.
	hasColon := p.peekAt(1).Type == lexer.TokenColon && p.peekAt(1).Line == tok.Line && adjacentTokens(tok, p.peekAt(1))
	if hasColon {
		p.parseRef() // e.g. "service:subscriptions-api" — parseRef's kind-prefix branch handles this shape.
		return diags
	}
	if isSlugKind(tok.Value) {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/dangling-slug-kind",
			Message:  fmt.Sprintf("%q is a reserved node-slug kind word and must be followed by ':<name>' (e.g. %s:name); used bare it is not a valid reference", tok.Value, tok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(tok),
		})
	}
	p.consumeAs(SyntaxKindIdent) // guarantee forward progress either way
	return diags
}

// edgeKeywords is the single source of truth for context_map relationship verbs
// (DDD strategic context-mapping patterns). LEFT = upstream for directional verbs.
var edgeKeywords = []string{
	// directional
	"customer_supplier", "conformist", "anticorruption_layer",
	"open_host_service", "published_language",
	// symmetric
	"partnership", "shared_kernel", "separate_ways",
}

// isEdgeKeyword reports whether s is a recognised context_map edge verb.
func isEdgeKeyword(s string) bool {
	for _, k := range edgeKeywords {
		if k == s {
			return true
		}
	}
	return false
}

// EdgeKeywords returns the context_map edge verbs the parser accepts. Exported so
// sema (and future docs/tooling) can verify its own verb classification stays in
// sync instead of hand-copying the list.
func EdgeKeywords() []string {
	return append([]string(nil), edgeKeywords...)
}

// parseRef consumes ONE reference at the current position and emits it as a
// single SyntaxKindRef node, wrapping every token that belongs to it. A ref
// is a contiguous same-line run of ident/number/`:`/`/` tokens, in one of two
// shapes:
//
//   - event ref (dotted, no `kind:` prefix): vas.VasApplied
//   - node slug: [kind:][ns/]name, e.g. bc:re/subscriptions
//
// Returns the leading kind word ("domain"/"bc"/"term"/"service") if a
// recognised `kind:` prefix was present, else "". It does not validate the
// kind's legitimacy beyond capturing it — sema (a later task) does that.
//
// parseRef is a standalone helper: as of Task 3 it is not called from any
// production parse path. Tasks 4-6 wire it into notifies/listens/asks,
// context_map edges, and repo: anchors.
func (p *Parser) parseRef() string {
	p.builder.StartNode(SyntaxKindRef)
	line := p.peek().Line
	kind := ""
	first := p.peek()
	var prev lexer.Token
	havePrev := false
	// leading kind word + ':'. "domain"/"service" lex as hard keywords, not
	// TokenIdent, so accept the same keyword-as-ident set the rest of the
	// parser uses (isAnyKeywordAsIdent) in addition to plain idents.
	if (first.Type == lexer.TokenIdent || isAnyKeywordAsIdent(first.Type)) &&
		p.peekAt(1).Type == lexer.TokenColon && p.peekAt(1).Line == line &&
		adjacentTokens(first, p.peekAt(1)) {
		if isSlugKind(first.Value) {
			kind = first.Value
		}
		p.consumeAs(SyntaxKindIdent) // kind word
		colonTok := p.peek()
		p.consumeAs(SyntaxKindColon) // ':'
		prev = colonTok
		havePrev = true
	}
	// Bug fix (Task 4): the original Task 3 implementation only checked
	// p.peek().Line == line here, with no adjacency check between
	// consecutive tokens. That over-consumes past a ref's true boundary when
	// it is immediately followed by whitespace-separated prose on the same
	// line — e.g. `bc:re/billing to record outcome` (an asks target followed
	// by connector + phrase) would swallow "to", "record", and "outcome" as
	// if they were more ref segments, since they are also bare TokenIdent
	// tokens on the same line. A ref must be a CONTIGUOUS run with no gaps.
	for !p.atEOF() && p.peek().Line == line {
		t := p.peek()
		if t.Type != lexer.TokenIdent && t.Type != lexer.TokenNumber &&
			!(t.Type == lexer.TokenError && t.Value == "/") &&
			!isAnyKeywordAsIdent(t.Type) {
			break
		}
		if havePrev && !adjacentTokens(prev, t) {
			break
		}
		p.consumeAs(SyntaxKindIdent)
		prev = t
		havePrev = true
	}
	p.builder.FinishNode()
	return kind
}

// adjacentTokens reports whether b begins immediately where a ends, with no
// intervening whitespace, on the same source line. parseRef uses this to
// keep a reference to a single contiguous token run (e.g. "bc:re/billing")
// instead of swallowing separate whitespace-delimited words that happen to
// also lex as bare idents (e.g. a trailing "to record outcome" phrase).
func adjacentTokens(a, b lexer.Token) bool {
	return a.Line == b.Line && a.Column+len([]rune(a.Value)) == b.Column
}

// isSlugKind reports whether s is a recognised node-slug kind word.
func isSlugKind(s string) bool {
	switch s {
	case "domain", "bc", "term", "service":
		return true
	}
	return false
}

// parseIdentList parses a comma-separated ident list, emitting tokens into the
// current builder scope. A TokenString entry (a quoted name, e.g.
// `contexts: "Some Name"`) is emitted as SyntaxKindString, not
// SyntaxKindIdent — consumeAs always stores the raw source text regardless
// of the kind passed in (tokenText returns tok.Raw for TokenString either
// way, so this does not affect Token.Value/Raw or round-trip output), but
// content-read call sites (stringAwareText and friends) dispatch on Kind()
// to decide whether to unquote. Mislabeling a quoted entry as Ident would
// make them silently skip unquoting and leak raw quotes into content.
func (p *Parser) parseIdentList() {
	for {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case tok.Type == lexer.TokenIdent:
			p.consumeAs(SyntaxKindIdent)
		case isKeywordUsedAsIdent(tok.Type):
			p.consumeAs(SyntaxKindIdent)
		default:
			return
		}
		if p.peek().Type == lexer.TokenComma {
			p.consumeAs(SyntaxKindComma)
		} else {
			break
		}
	}
}

// parseRefList parses a comma-separated ident list, wrapping each name in a
// SyntaxKindRef node so that reference sites are structurally distinct from
// declaration sites. The flat Tokens() result is unchanged — existing AST
// scanning code remains correct. As in parseIdentList above, a TokenString
// entry is emitted as SyntaxKindString (not SyntaxKindIdent) so Kind()-based
// content dispatch (stringAwareText) can tell it apart from a bare ident.
func (p *Parser) parseRefList() {
	for {
		tok := p.peek()
		isIdent := tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString ||
			isKeywordUsedAsIdent(tok.Type)
		if !isIdent {
			return
		}
		p.builder.StartNode(SyntaxKindRef)
		if tok.Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else {
			p.consumeAs(SyntaxKindIdent)
		}
		p.builder.FinishNode()
		if p.peek().Type == lexer.TokenComma {
			p.consumeAs(SyntaxKindComma)
		} else {
			break
		}
	}
}

// parseIdentListWithLines parses a comma-separated ident list, wrapping each
// name in a SyntaxKindRef node (reference tracking). Lines are derived from
// SyntaxToken offsets + LineIndex — no explicit line tracking needed.
func (p *Parser) parseIdentListWithLines() {
	p.parseRefList()
}

// parseDeploymentSpec parses a deployment spec, emitting tokens into the
// current builder scope.
func (p *Parser) parseDeploymentSpec() []model.Diagnostic {
	var diags []model.Diagnostic

	typeTok := p.peek()
	if typeTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(typeTok.Type) {
		p.consumeAs(SyntaxKindIdent)
	} else {
		diags = append(diags, p.diagUnexpected(typeTok, "deployment type identifier"))
		return diags
	}

	if p.peek().Type != lexer.TokenLParen {
		return diags
	}
	p.consumeAs(SyntaxKindLParen)

	for !p.atEOF() && p.peek().Type != lexer.TokenRParen {
		pctTok := p.peek()
		if pctTok.Type != lexer.TokenPercentage {
			diags = append(diags, p.diagUnexpected(pctTok, "percentage (e.g. 90%)"))
			p.consume()
			continue
		}
		p.consumeAs(SyntaxKindPercentage)

		if p.peek().Type != lexer.TokenArrow {
			diags = append(diags, p.diagUnexpected(p.peek(), "->"))
			p.consume()
			continue
		}
		p.consumeAs(SyntaxKindArrow)

		targetTok := p.peek()
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			p.consumeAs(SyntaxKindIdent)
		} else {
			diags = append(diags, p.diagUnexpected(targetTok, "deployment target identifier"))
		}

		if p.peek().Type == lexer.TokenComma {
			p.consumeAs(SyntaxKindComma)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed deployment rule list (missing `)`)",
			Severity: model.SeverityError,
			Range:    tokenRange(typeTok),
		})
		return diags
	}
	p.consumeAs(SyntaxKindRParen)
	return diags
}
