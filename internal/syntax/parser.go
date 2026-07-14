// Package syntax implements the hand-written recursive-descent parser for the
// Craft DSL. It produces a lossless syntax tree by emitting events into a
// green.GreenNodeBuilder.
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

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/lexer"
	"github.com/tcarcao/craft/pkg/craft"
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
func Parse(src string) (*green.GreenNode, green.LineIndex, []craft.Diagnostic) {
	li := green.NewLineIndex(src)
	l := lexer.New(src)
	p := &Parser{tokens: l.All(), src: src, li: li}
	root, diags := p.parseFile()
	return root, li, diags
}

// --- main parse loop ---

func (p *Parser) parseFile() (*green.GreenNode, []craft.Diagnostic) {
	p.builder.StartNode(SyntaxKindFile)
	var diags []craft.Diagnostic

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
func (p *Parser) parseActorStatement() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindActorDecl)
	var diags []craft.Diagnostic

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
func (p *Parser) parseImportStatement() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindImportDecl)
	var diags []craft.Diagnostic

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
func (p *Parser) parseActorsBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindActorsBlock)
	var diags []craft.Diagnostic

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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed actors block (missing `}`)",
			Severity: craft.SeverityError,
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
func (p *Parser) parseDomainStatement() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindDomainDecl)
	var diags []craft.Diagnostic

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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domain block (missing `}`)",
			Severity: craft.SeverityError,
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
func (p *Parser) parseDomainsBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindDomainsBlock)
	var diags []craft.Diagnostic

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
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  "unclosed domain block (missing `}`)",
				Severity: craft.SeverityError,
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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed domains block (missing `}`)",
			Severity: craft.SeverityError,
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
func (p *Parser) parseBoundedContextList() []craft.Diagnostic {
	seen := make(map[string]bool)
	var diags []craft.Diagnostic

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
func (p *Parser) parseServicesBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindServicesBlock)
	var diags []craft.Diagnostic

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
			diags = append(diags, craft.Diagnostic{
				Code:     "craft/syntax/unclosed-block",
				Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
				Severity: craft.SeverityError,
				Range: craft.Range{
					Start: craft.Position{Line: lspLine(nameLine)},
					End:   craft.Position{Line: lspLine(nameLine)},
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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed services block (missing `}`)",
			Severity: craft.SeverityError,
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
func (p *Parser) parseServiceStatement() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindServiceDecl)
	var diags []craft.Diagnostic

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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed service block for %q (missing `}`)", name),
			Severity: craft.SeverityError,
			Range: craft.Range{
				Start: craft.Position{Line: lspLine(nameLine)},
				End:   craft.Position{Line: lspLine(nameLine)},
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
func (p *Parser) parseServiceBody() []craft.Diagnostic {
	var diags []craft.Diagnostic

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
func (p *Parser) parseUseCaseBlock(counter *int) []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindUseCaseDecl)
	var diags []craft.Diagnostic

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
		} else {
			// Skip unknown tokens inside the use_case body.
			diags = append(diags, p.diagUnexpected(tok, "`when` or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed use_case block for %q (missing `}`)", name),
			Severity: craft.SeverityError,
			Range:    tokenRange(ucTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseScenario parses one `when <trigger>` clause plus its following action lines.
// counter is a shared global ID counter (pointer) for scenario_N / action_N IDs,
// matching ANTLR's numbering scheme where both scenarios and actions share one counter.
func (p *Parser) parseScenario(counter *int) []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindScenario)
	var diags []craft.Diagnostic

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
func (p *Parser) parseTrigger(whenLine int) []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindTrigger)
	var diags []craft.Diagnostic

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
		// domain_listen: when <domain> listens "<event>"
		eventTok := p.peek()
		if eventTok.Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else if eventTok.Type == lexer.TokenIdent {
			p.consumeAs(SyntaxKindIdent)
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
func (p *Parser) parseAction(counter *int) []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindAction)
	var diags []craft.Diagnostic

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
	if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
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
func (p *Parser) parseNotifiesAction() []craft.Diagnostic {
	var diags []craft.Diagnostic
	eventTok := p.peek()
	if eventTok.Type == lexer.TokenString {
		p.consumeAs(SyntaxKindString)
	} else if eventTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(eventTok.Type) {
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
func (p *Parser) parseArchBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindArchDecl)
	var diags []craft.Diagnostic

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
				diags = append(diags, craft.Diagnostic{
					Code:     "craft/syntax/unknown-arch-section",
					Message:  fmt.Sprintf("unknown arch section %q; expected presentation or gateway", label),
					Severity: craft.SeverityWarning,
					Range:    craft.Range{Start: craft.Position{Line: lspLine(labelLine)}},
				})
			}
		} else {
			// Unexpected token in arch body — skip.
			diags = append(diags, p.diagUnexpected(tok, "arch section label (presentation or gateway) or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed arch block (missing `}`)",
			Severity: craft.SeverityError,
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
func (p *Parser) parseArchComponentList() []craft.Diagnostic {
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

		d := p.parseArchComponent()
		diags = append(diags, d...)
	}
	return diags
}

// parseArchComponent parses a single component entry as an ArchComponent node.
// Flow chains (a > b > c) become a single ArchComponent node containing all
// sub-components and the `>` tokens (matching prior behavior).
func (p *Parser) parseArchComponent() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindArchComponent)
	var diags []craft.Diagnostic

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
func (p *Parser) parseComponentWithModifiers() (bool, []craft.Diagnostic) {
	var diags []craft.Diagnostic

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
func (p *Parser) parseModifierList() []craft.Diagnostic {
	var diags []craft.Diagnostic

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
	p.prevEnd = start + green.TextSize(len(tok.Value))
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
	p.builder.Token(lexerKindToSyntaxKind(tok.Type), tok.Value)
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
	p.builder.Token(kind, tok.Value)
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

// lspLine converts a 1-based source line to a 0-based LSP line number.
func lspLine(line int) int {
	if line <= 0 {
		return 0
	}
	return line - 1
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
func (p *Parser) parseExposureBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindExposureDecl)
	var diags []craft.Diagnostic

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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed exposure block (missing `}`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(kwTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseIdentList parses a comma-separated ident list, emitting tokens into the
// current builder scope.
func (p *Parser) parseIdentList() {
	for {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString:
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
// scanning code remains correct.
func (p *Parser) parseRefList() {
	for {
		tok := p.peek()
		isIdent := tok.Type == lexer.TokenIdent || tok.Type == lexer.TokenString ||
			isKeywordUsedAsIdent(tok.Type)
		if !isIdent {
			return
		}
		p.builder.StartNode(SyntaxKindRef)
		p.consumeAs(SyntaxKindIdent)
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
func (p *Parser) parseDeploymentSpec() []craft.Diagnostic {
	var diags []craft.Diagnostic

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
		diags = append(diags, craft.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed deployment rule list (missing `)`)",
			Severity: craft.SeverityError,
			Range:    tokenRange(typeTok),
		})
		return diags
	}
	p.consumeAs(SyntaxKindRParen)
	return diags
}

