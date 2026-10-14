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
// A construct the grammar recognises but the parser does not implement emits a
// recoverable "not-yet-implemented" warning, so the parser stays usable on
// partial files. A token it cannot place at all is different: recovery
// discards a region of the file, so that emits a "skipped-construct" error
// covering everything discarded.
package syntax

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
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
	prevEnd green.TextSize // byte offset after last emitted token

	// lastUC records the most recent use_case block that closed cleanly, so a
	// `when` the parser cannot place at top level can blame the `}` that
	// closed the block early rather than the `when` it stranded. Zeroed at the
	// start of every use_case, so a block that bailed to recovery leaves
	// nothing behind to blame.
	lastUC closedUseCase
}

// closedUseCase is the anchor for the stray-`}` hint: the name and lines of a
// use_case whose closing brace has just been consumed. CloseLine == 0 means no
// use_case has closed.
type closedUseCase struct {
	Name      string
	OpenLine  int // 1-based line of the `use_case` keyword
	CloseLine int // 1-based line of the `}` that closed it
}

// Parse parses src and returns the green root, a LineIndex, and any diagnostics.
// A non-nil GreenNode is always returned (island parsing preserved).
func Parse(src string) (*green.GreenNode, green.LineIndex, []model.Diagnostic) {
	li := green.NewLineIndex(src)
	l := lexer.New(src)
	p := &Parser{tokens: l.All(), src: src}
	root, diags := p.parseFile()
	diags = append(diags, checkTreeText(root, src)...)
	return root, li, diags
}

// checkTreeText asserts rowan's invariant: concatenating the tree's tokens
// reproduces the source exactly. This supersedes a width-sum check, which
// could not detect a token whose text diverged from its source range because
// the token's length was derived from that same wrong text.
//
// This panics rather than returning a diagnostic. Token text is a slice of
// the source and error recovery wraps tokens in an ErrorNode rather than
// dropping them, so no input can reach here: a failure is a parser bug.
//
// Only a parser bug can get here: token text is a slice of the source and
// error recovery wraps tokens rather than dropping them, so no input can
// violate this. Fail hard under test so CI cannot miss it, but degrade to a
// diagnostic in production: syntax.Parse is reached from pkg/craft, which is
// a public library API, and panicking there would crash a consumer.
func checkTreeText(root *green.GreenNode, src string) []model.Diagnostic {
	if root == nil {
		return nil
	}
	var sb strings.Builder
	sb.Grow(len(src))
	collectGreenText(root, &sb)
	if sb.String() == src {
		return nil
	}
	msg := fmt.Sprintf(
		"internal parser error: syntax tree does not reproduce its source (tree %d bytes, source %d bytes); positions in this file may be wrong",
		sb.Len(), len(src))
	if testing.Testing() {
		panic(msg)
	}
	return []model.Diagnostic{{
		Code:     "craft/internal/tree-text-mismatch",
		Message:  msg,
		Severity: model.SeverityError,
	}}
}

// collectGreenText walks the green tree in document order, appending every
// leaf token's text to sb. Internal nodes carry no text of their own; only
// tokens do.
func collectGreenText(n *green.GreenNode, sb *strings.Builder) {
	for _, c := range n.Children {
		switch e := c.(type) {
		case *green.GreenToken:
			sb.WriteString(e.Text)
		case *green.GreenNode:
			collectGreenText(e, sb)
		}
	}
}

// --- main parse loop ---

func (p *Parser) parseFile() (*green.GreenNode, []model.Diagnostic) {
	p.builder.StartNode(SyntaxKindFile)
	var diags []model.Diagnostic

	// prevWasUseCase gates the stray-`}` hint to a `when` that directly follows
	// a use_case. Further out the hint would point at a brace the reader has
	// no reason to suspect.
	prevWasUseCase := false

	for !p.atEOF() {
		if p.atSurfaceDecl() {
			diags = append(diags, p.parseSurfaceBlock()...)
			prevWasUseCase = false
			continue
		}
		tok := p.peek()
		wasUseCase := tok.Type == lexer.TokenKwUseCase
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
			diags = append(diags, p.parseUseCaseBlock()...)
		case lexer.TokenKwArch:
			diags = append(diags, p.parseArchBlock()...)
		case lexer.TokenKwExposure:
			diags = append(diags, p.parseExposureBlock()...)
		case lexer.TokenKwContextMap:
			diags = append(diags, p.parseContextMapBlock()...)
		case lexer.TokenKwGlossary:
			diags = append(diags, p.parseGlossaryBlock()...)
		default:
			// Unrecognised top-level token: resync to the next top-level
			// keyword (island parsing) and report what that discarded.
			//
			// A `[` here is the one case where the grammar is fine and the
			// parser is behind (a phrase brace makes the trailing annotation
			// unplaceable), so it keeps the not-yet-implemented warning.
			// Anything else means the file is ungrammatical and a region of it
			// is now absent from the model, which is an error.
			if tok.Value == "[" {
				diags = append(diags, p.diagNotImplemented(tok))
				p.resyncToTopLevel()
			} else {
				hint := p.strayBraceHint(tok, prevWasUseCase)
				last := p.resyncToTopLevel()
				diags = append(diags, p.diagSkippedConstruct(tok, last, hint))
			}
		}
		prevWasUseCase = wasUseCase
	}

	// Trailing comments are trivia like any other: peek() skips comment
	// tokens, so at EOF the main loop leaves them unconsumed. Emit them as
	// comment tokens before sweeping the rest, or they land inside one
	// whitespace blob and no token-walking consumer can see them.
	p.attachTrivia()

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

	if p.peek().Type != lexer.TokenRBrace {
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

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace && !p.atColumnOneDecl() {
		// Attach trivia inside block.
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace || p.atColumnOneDecl() {
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

		if p.peek().Type != lexer.TokenRBrace {
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
// Duplicates are silently deduplicated (keeping first occurrence), as the v1
// spec requires.
// Emits BoundedContext nodes into the current builder scope.
func (p *Parser) parseBoundedContextList() []model.Diagnostic {
	seen := make(map[string]bool)
	var diags []model.Diagnostic

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace && !p.atColumnOneDecl() {
		// Attach trivia inside the context list.
		p.attachTrivia()
		if p.atEOF() || p.peek().Type == lexer.TokenRBrace || p.atColumnOneDecl() {
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
				// Duplicate: consume but do not wrap (first occurrence wins).
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
		case "catalog_ref":
			p.consumeAs(SyntaxKindKwCatalogRef)
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
		case "catalog_ref":
			if p.peek().Type == lexer.TokenIdent {
				p.consumeAs(SyntaxKindIdent)
			} else {
				diags = append(diags, p.diagUnexpected(p.peek(), "catalog_ref identifier"))
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
func (p *Parser) parseUseCaseBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindUseCaseDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `use_case`.
	p.attachTrivia()

	// Any earlier close is stale from here on: if this block bails to recovery
	// it must not leave a previous block's `}` behind to be blamed.
	p.lastUC = closedUseCase{}

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

	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace && !p.atColumnOneDecl() {
		tok := p.peek()

		// `when` is a contextual keyword that lexes as TokenIdent.
		if tok.Type == lexer.TokenIdent && (tok.Value == "when" || p.atScenarioName()) {
			d := p.parseScenario()
			diags = append(diags, d...)
		} else if tok.Type == lexer.TokenIdent && tok.Value == "tags" {
			diags = append(diags, p.parseUseCaseTagsBlock()...)
		} else if p.atNoteLine() {
			diags = append(diags, p.parseNote()...)
		} else {
			// Skip unknown tokens inside the use_case body.
			diags = append(diags, p.diagUnexpected(tok, "`when`, `tags`, `note`, `finding`, or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.peek().Type != lexer.TokenRBrace {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  fmt.Sprintf("unclosed use_case block for %q (missing `}`)", name),
			Severity: model.SeverityError,
			Range:    tokenRange(ucTok),
		})
		p.builder.FinishNode()
		return diags
	}
	rbraceTok := p.peek()
	p.consumeAs(SyntaxKindRBrace)
	p.lastUC = closedUseCase{Name: name, OpenLine: ucTok.Line, CloseLine: rbraceTok.Line}
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
// The statement's node kind is decided at the end rather than the start. A
// statement that does not parse is wrapped as SyntaxKindErrorNode instead of
// SyntaxKindTagStmt, so TagsBlock.Tags() cannot see it and no half-parsed tag
// reaches the model. Previously every failure path finished a TagStmt node
// regardless, and `channels: web, mobile` projected as
// `{"channels": "web", "mobile": ""}`: the tail of a value that failed to
// parse, promoted to a key nobody wrote. The diagnostics were correct, but
// consumers of `craft check` read the data channel, not the diagnostic
// channel. Wrapping retroactively is what Checkpoint/StartNodeAt are for; the
// tokens are emitted either way, so the tree stays lossless.
func (p *Parser) parseTagStmt() []model.Diagnostic {
	cp := p.builder.Checkpoint()
	var diags []model.Diagnostic

	// fail closes the statement as an error region rather than a tag.
	fail := func() []model.Diagnostic {
		p.builder.StartNodeAt(cp, SyntaxKindErrorNode)
		p.builder.FinishNode()
		return diags
	}

	keyTok := p.peek()
	if keyTok.Type != lexer.TokenIdent {
		diags = append(diags, p.diagUnexpected(keyTok, "a tag key"))
		p.consumeAs(SyntaxKindError)
		return fail()
	}
	p.consumeAs(SyntaxKindIdent)

	if p.peek().Type != lexer.TokenColon {
		diags = append(diags, p.diagUnexpected(p.peek(), "`:`"))
		return fail()
	}
	p.consumeAs(SyntaxKindColon)

	valueDiags, ok := p.parseTagValueList()
	diags = append(diags, valueDiags...)
	if !ok {
		return fail()
	}

	p.builder.StartNodeAt(cp, SyntaxKindTagStmt)
	p.builder.FinishNode()
	return diags
}

// parseTagValueList parses `tag_value (',' tag_value)*`, the same comma-list
// shape `contexts:` and `data-stores:` already use: each item is a quoted
// string or a bare ref-shaped slug, the two may be mixed, and the list may
// wrap across lines. A bare item goes through parseRef's scanner so a
// slash/hyphen-bearing slug like "re/renewal-flow" is one item rather than
// three.
//
// Unlike parseRefList it refuses to run past a dangling comma. That helper
// stops silently when the item after a `,` is missing, which here would hand
// the model either a truncated list or, worse, the next statement's key as a
// value: `channels: web,` followed by `journey: renewal` would report two
// channels, one of them named journey. A statement that cannot be read whole
// must not reach the model at all, so this reports and the caller discards it.
//
// ok is false when the statement failed; diags explains why.
func (p *Parser) parseTagValueList() ([]model.Diagnostic, bool) {
	var diags []model.Diagnostic
	for {
		tok := p.peek()
		switch {
		case tok.Type == lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case tok.Type == lexer.TokenIdent || isKeywordUsedAsIdent(tok.Type):
			p.parseRef()
		default:
			diags = append(diags, p.diagUnexpected(tok, "a tag value (identifier, string, or ref)"))
			return diags, false
		}

		if p.peek().Type != lexer.TokenComma {
			return diags, true
		}
		commaTok := p.peek()
		p.consumeAs(SyntaxKindComma)

		if !p.startsTagValue() {
			diags = append(diags, p.diagDanglingTagValue(commaTok))
			// Consume the offender only when it cannot start the next
			// statement or close the block, so one typo is reported once.
			if next := p.peek(); next.Type != lexer.TokenRBrace && !p.startsTagKey() {
				p.consumeAs(SyntaxKindError)
			}
			return diags, false
		}
	}
}

// startsTagValue reports whether the next token can begin a list item. An
// identifier immediately followed by `:` is the next tag statement, not an
// item: without that check `channels: web,` followed by `journey: renewal`
// would read journey as a channel and then fail on its colon.
func (p *Parser) startsTagValue() bool {
	tok := p.peek()
	if tok.Type == lexer.TokenString {
		return true
	}
	if !p.startsTagKey() {
		return false
	}
	return p.peekAt(1).Type != lexer.TokenColon
}

// startsTagKey reports whether the next token could open a tag statement.
func (p *Parser) startsTagKey() bool {
	tok := p.peek()
	return tok.Type == lexer.TokenIdent || isKeywordUsedAsIdent(tok.Type)
}

// atScenarioName reports whether the next tokens are the contextual keyword
// `scenario` directly followed by a string, i.e. a `scenario "<label>"` line.
func (p *Parser) atScenarioName() bool {
	tok := p.peek()
	return tok.Type == lexer.TokenIdent && tok.Value == "scenario" &&
		p.peekAt(1).Type == lexer.TokenString
}

// parseScenario parses one `when <trigger>` clause plus its following action lines.
func (p *Parser) parseScenario() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindScenario)
	var diags []model.Diagnostic

	nameWithoutWhen := false
	if p.atScenarioName() {
		scTok := p.peek()
		nt := p.peekAt(2)
		nameWithoutWhen = !(nt.Type == lexer.TokenIdent && nt.Value == "when")
		if nameWithoutWhen {
			// No `when` follows: the name is dropped. The tokens are wrapped
			// in an error node so ScenarioDecl.Name() does not read them.
			p.builder.StartNode(SyntaxKindError)
		} else {
			p.builder.StartNode(SyntaxKindScenarioName)
		}
		p.consumeAs(SyntaxKindKwScenario)
		p.consumeAs(SyntaxKindString)
		p.builder.FinishNode()
		if nameWithoutWhen {
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/scenario-without-when",
				Message:  "`scenario \"…\"` must be followed by a `when` line",
				Severity: model.SeverityError,
				Range:    tokenRange(scTok),
			})
		}
	}

	if nameWithoutWhen {
		// Keep the tree shape uniform for consumers that always expect a
		// trigger child: an empty one.
		p.builder.StartNode(SyntaxKindTrigger)
		p.builder.FinishNode()
	} else {
		// consume `when` as contextual keyword
		whenTok := p.peek()
		p.consumeAs(SyntaxKindKwWhen)

		d := p.parseTrigger(whenTok.Line)
		diags = append(diags, d...)
	}

	// Parse steps until we see `when` (next scenario), `}` (end of use_case), or EOF.
	diags = append(diags, p.parseSteps(0)...)

	p.builder.FinishNode()
	return diags
}

// TabWidth is the tab stop of the offside rule (peekVisualColumn): a tab
// advances the column to the next multiple of TabWidth. The formatter reads
// source columns with the same width, so it is exported.
const TabWidth = 4

// peekVisualColumn returns the 1-based column the offside rule uses for the
// line that the next significant token starts: the line's indentation, with
// tabs expanded to the next multiple of TabWidth. Comments are trivia, so
// `/* c */ else "b"` sits where its line starts, not where `else` does. It is
// used only by the offside rule of block bodies; Token.Column (runes) stays
// the column for everything else.
//
// The caller must have checked peekAtLineStart: the walk back to the line
// start is then bounded by the line's indentation and leading comments,
// which keeps a long line of steps linear rather than quadratic.
func (p *Parser) peekVisualColumn() int {
	tok := p.peek()
	if tok.Type == lexer.TokenEOF || tok.Offset < 0 || tok.Offset > len(p.src) {
		return tok.Column
	}
	lineStart := strings.LastIndexByte(p.src[:tok.Offset], '\n') + 1
	col := 0
	for _, r := range p.src[lineStart:tok.Offset] {
		switch r {
		case '\t':
			col = (col/TabWidth + 1) * TabWidth
		case ' ':
			col++
		default:
			return col + 1 // a leading comment's column
		}
	}
	return col + 1
}

// peekAtLineStart reports whether the next significant token is the first
// significant token on its line: only whitespace and comments (trivia)
// precede it there, so `/* c */ alt "b"` still starts with `alt`.
func (p *Parser) peekAtLineStart() bool {
	tok := p.peek()
	if tok.Offset < 0 || tok.Offset > len(p.src) {
		return false
	}
	prev := p.lastConsumed()
	if prev.Type == lexer.TokenEOF {
		return true // nothing consumed yet
	}
	if prev.End < 0 || prev.End > tok.Offset {
		return false
	}
	return strings.Contains(p.src[prev.End:tok.Offset], "\n")
}

// isHeader reports whether the next tokens are a block header: the contextual
// keyword `alt`, `opt`, `else`, `par`, `and` or `loop` at the start of a line,
// followed by a string label or by nothing else on its line (a missing
// label). Any other token after the keyword makes the line an ordinary action
// whose subject is named after the keyword, so those words stay valid names.
func (p *Parser) isHeader() (string, bool) {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent {
		return "", false
	}
	switch tok.Value {
	case "alt", "opt", "else", "par", "and", "loop":
	default:
		return "", false
	}
	if !p.peekAtLineStart() {
		return "", false
	}
	next := p.peekAt(1)
	if next.Type == lexer.TokenString || next.Type == lexer.TokenEOF || next.Line != tok.Line {
		return tok.Value, true
	}
	return "", false
}

// atNoteLine reports whether the next tokens are a note statement: `note` or
// `finding` at the start of a line, followed by a string on that line or by
// nothing else on it (a missing text). Anything else after the word makes the
// line an ordinary action, so both words stay valid names.
func (p *Parser) atNoteLine() bool {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || (tok.Value != "note" && tok.Value != "finding") || !p.peekAtLineStart() {
		return false
	}
	next := p.peekAt(1)
	return next.Type == lexer.TokenString || next.Type == lexer.TokenEOF || next.Line != tok.Line
}

// parseNote parses `note "<text>"` or `finding "<text>"`. A note is one line.
func (p *Parser) parseNote() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindNote)
	kind := SyntaxKindKwNote
	if p.peek().Value == "finding" {
		kind = SyntaxKindKwFinding
	}
	kwTok := p.consumeAs(kind)
	if t := p.peek(); t.Type == lexer.TokenString && t.Line == kwTok.Line {
		p.consumeAs(SyntaxKindString)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-label",
			Message:  fmt.Sprintf("`%s` needs a quoted text, e.g. `%s \"a lost callback leaves the invoice open\"`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	diags = append(diags, p.sweepRestOfLine(kwTok.Line, "end of line after the note text")...)
	p.builder.FinishNode()
	return diags
}

// sweepRestOfLine wraps whatever is left on line in an error node and reports
// its first token as unexpected. A `}` or EOF ends the sweep.
func (p *Parser) sweepRestOfLine(line int, expected string) []model.Diagnostic {
	t := p.peek()
	if t.Line != line || t.Type == lexer.TokenEOF || t.Type == lexer.TokenRBrace {
		return nil
	}
	d := p.diagUnexpected(t, expected)
	p.builder.StartNode(SyntaxKindErrorNode)
	for t := p.peek(); t.Line == line && t.Type != lexer.TokenEOF && t.Type != lexer.TokenRBrace; t = p.peek() {
		p.consume()
	}
	p.builder.FinishNode()
	return []model.Diagnostic{d}
}

// atStep reports whether the next token can start a step of the current
// scenario: anything but `when`, a `scenario "…"` line, `}` or EOF.
func (p *Parser) atStep() bool {
	if p.atColumnOneDecl() {
		return false // the enclosing use case is unclosed
	}
	tok := p.peek()
	switch tok.Type {
	case lexer.TokenEOF, lexer.TokenRBrace:
		return false
	case lexer.TokenIdent:
		return tok.Value != "when" && !p.atScenarioName()
	}
	return true
}

// parseSteps parses the steps (actions, blocks and notes) of a scenario or of a
// branch body.
//
// minCol is the visual column of the enclosing branch header, or 0 at the
// scenario level. Inside a branch the offside rule applies: the body is every
// following line whose first token is right of minCol, and it ends at the
// first line at or left of it. At the scenario level indentation is
// insignificant. Either way the steps end at `when`, `scenario "…"`, `}` or
// EOF, however deeply those are indented.
//
// Comments are trivia: peek skips them, so they never end a body.
func (p *Parser) parseSteps(minCol int) []model.Diagnostic {
	var diags []model.Diagnostic
	bodyCol := 0 // column of the branch's first body line; 0 until seen
	for p.atStep() {
		tok := p.peek()
		kw, isHdr := p.isHeader()

		// Only the first token of a line is subject to the offside rule: a
		// token later on a line belongs wherever its line's first token went,
		// and can never be a header.
		col := 0
		if p.peekAtLineStart() && (minCol > 0 || isHdr) {
			col = p.peekVisualColumn()
		}
		if minCol > 0 && col > 0 {
			if col <= minCol {
				break
			}
			switch {
			case bodyCol == 0:
				bodyCol = col
			case col < bodyCol && kw != "else" && kw != "and":
				// A mis-indented else/and is reported only as an orphan below.
				diags = append(diags, model.Diagnostic{
					Code:     "craft/syntax/inconsistent-indent",
					Message:  "this line is indented less than the steps above it but more than its block header; align it with one of them",
					Severity: model.SeverityError,
					Range:    tokenRange(tok),
				})
			}
		}

		if !isHdr {
			if p.atNoteLine() {
				diags = append(diags, p.parseNote()...)
			} else {
				diags = append(diags, p.parseAction()...)
			}
			continue
		}
		switch kw {
		case "else":
			// No alt at this column is open (an else at an open alt's column
			// is taken by parseBlock before control gets back here), or the
			// open block is an opt, par or loop.
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/orphan-else",
				Message:  "`else` continues an `alt` at the same column; a `par` continues with `and`",
				Severity: model.SeverityError,
				Range:    tokenRange(tok),
			})
		case "and":
			// No par at this column is open, or the open block is an alt,
			// opt or loop.
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/orphan-and",
				Message:  "`and` continues a `par` at the same column; an `alt` continues with `else`",
				Severity: model.SeverityError,
				Range:    tokenRange(tok),
			})
		}
		diags = append(diags, p.parseBlock(kw, col)...)
	}
	return diags
}

// parseBlock parses an alt, opt, par or loop block whose first header is at
// visual column col. kw is the first header's keyword; an orphan `else` is
// parsed as the first branch of a new alt, an orphan `and` as the first branch
// of a new par. An alt takes every following `else` header at exactly col as a
// further branch, a par every following `and`; an opt and a loop have one
// branch.
func (p *Parser) parseBlock(kw string, col int) []model.Diagnostic {
	p.builder.StartNode(SyntaxKindBlock)
	diags := p.parseBranch(kw, col)
	cont := ""
	switch kw {
	case "alt", "else":
		cont = "else"
	case "par", "and":
		cont = "and"
	}
	if cont != "" {
		for {
			next, ok := p.isHeader()
			// isHeader implies the continuation header starts its line.
			if !ok || next != cont || p.peekVisualColumn() != col {
				break
			}
			diags = append(diags, p.parseBranch(cont, col)...)
		}
	}
	p.builder.FinishNode()
	return diags
}

// parseBranch parses one `alt|opt|else|par|and|loop "<label>"` header and its body, the
// lines right of the header's column col.
func (p *Parser) parseBranch(kw string, col int) []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindBranch)
	p.builder.StartNode(SyntaxKindBranchHeader)

	kind := SyntaxKindKwAlt
	switch kw {
	case "opt":
		kind = SyntaxKindKwOpt
	case "else":
		kind = SyntaxKindKwElse
	case "par":
		kind = SyntaxKindKwPar
	case "and":
		kind = SyntaxKindKwAnd
	case "loop":
		kind = SyntaxKindKwLoop
	}
	kwTok := p.consumeAs(kind)

	if t := p.peek(); t.Type == lexer.TokenString && t.Line == kwTok.Line {
		p.consumeAs(SyntaxKindString)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-label",
			Message:  fmt.Sprintf("`%s` needs a quoted label, e.g. `%s \"payment fails\"`", kw, kw),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}

	// Anything else on the header line after the label is not part of it.
	diags = append(diags, p.sweepRestOfLine(kwTok.Line, "end of line after the block label")...)
	p.builder.FinishNode() // BranchHeader

	// The header line's remainder has been consumed (or is a `}`, which ends
	// the steps), so a next step starts its own line.
	if !p.atStep() || p.peekVisualColumn() <= col {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/empty-branch",
			Message:  fmt.Sprintf("`%s \"…\"` has no steps: indent its steps under it", kw),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	} else {
		diags = append(diags, p.parseSteps(col)...)
	}

	p.builder.FinishNode() // Branch
	return diags
}

// parseTrigger parses the `<actor/domain> <verb> <phrase>` part after `when`.
// Three forms with a name subject:
//   - external:      `when <actor> <verb> <phrase>`
//   - domain_listen: `when <domain> listens "<event>"`
//   - receive:       `when <ctx> receives [connector] <phrase> [op]`
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
	// The subject of a `listens` trigger names a bounded context, so it accepts
	// the qualified `<domain>/<name>` form. Which trigger form this is is not
	// known until the verb is read, one slot later, so the external form
	// accepts a qualified actor too. A bare subject is still emitted as a
	// single SyntaxKindIdent token, even when the lexer classifies it as a
	// keyword (e.g. "Actor" → TokenKwActor), so ActorName() / ContextName()
	// read it the way they always have.
	diags = append(diags, p.parseNameSlot("a trigger subject")...)

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
		// domain_listen: when <domain> listens event_item (',' NL* event_item)*
		last := verbTok
		if t := p.peek(); t.Line == verbTok.Line && (t.Type == lexer.TokenString || t.Type == lexer.TokenIdent) {
			var d []model.Diagnostic
			last, d = p.parseEventList()
			diags = append(diags, d...)
		}
		// Optional polling cadence after the last item:
		// `listens <ref> cron "…"` / `every "…"`.
		if t := p.peek(); t.Line == last.Line && t.Type == lexer.TokenIdent && (t.Value == "cron" || t.Value == "every") {
			if s := p.peekAt(1); s.Type == lexer.TokenString && s.Line == t.Line {
				kind := SyntaxKindKwCron
				if t.Value == "every" {
					kind = SyntaxKindKwEvery
				}
				p.consumeAs(kind)
				p.consumeAs(SyntaxKindString)
			}
		}
		p.builder.FinishNode()
		return diags
	}

	if verb == "receives" {
		// receive: when <ctx> receives [connector_word] [phrase] [op]
		p.consumeAs(SyntaxKindKwReceives)
		if c := p.peek(); c.Type == lexer.TokenIdent && isConnectorWord(c.Value) && c.Line == verbTok.Line {
			p.consumeAs(SyntaxKindIdent)
		}
		start := p.opAnnotationStart(verbTok.Line)
		p.collectPhrase(verbTok.Line, start)
		if start >= 0 {
			diags = append(diags, p.parseOpAnnotation()...)
		}
		p.builder.FinishNode()
		return diags
	}

	// external: when <actor> <verb> [connector_word] <phrase>
	p.consumeAs(SyntaxKindIdent)

	// connector_word: a|an|the|as|to|from|in|on|at|for|with|by
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == verbTok.Line {
		p.consumeAs(SyntaxKindIdent)
	}
	// Collect phrase tokens on the same line. Trigger phrases don't support a
	// trailing op annotation (out of scope for this feature), so always sweep
	// to end of line.
	p.collectPhrase(verbTok.Line, -1)

	p.builder.FinishNode()
	return diags
}

// isConnectorWord reports whether v is a grammar connector_word.
// Grammar rule: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by'
func isConnectorWord(v string) bool {
	switch v {
	case "a", "an", "the", "as", "to", "from", "in", "on", "at", "for", "with", "by":
		return true
	}
	return false
}

// parseAction parses a single action line.
//
// Action forms (subject is always an ident/keyword-as-ident):
//
//	<domain> asks <target> to|for <phrase>   → sync_action
//	<domain> notifies "<event>"              → async_action
//	<domain> returns [to <target>] <phrase>  → return_action
//	<domain> <verb> <phrase>                 → internal_action
func (p *Parser) parseAction() []model.Diagnostic {
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
	// The subject names a bounded context, so it accepts the qualified
	// `<domain>/<name>` form the ambiguous-bc diagnostic recommends. A bare
	// subject is still emitted as a single SyntaxKindIdent token, even when
	// the lexer classifies it as a keyword (e.g. "Service" → TokenKwService),
	// so SubjectName() reads it the way it always has.
	diags = append(diags, p.parseNameSlot("an action subject")...)

	verbTok := p.peek()
	if verbTok.Type == lexer.TokenEOF || verbTok.Type == lexer.TokenRBrace || verbTok.Line != actionLine {
		// Subject-only line. The verb must be on the action's own line: taking
		// the next line's first token as the verb would swallow that line,
		// and across a block boundary that line belongs to another block.
		diags = append(diags, p.diagNoVerb(subjectTok, p.lastConsumed()))
		p.builder.FinishNode()
		return diags
	}
	if verbTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(verbTok.Type) {
		// No verb — treat as minimal internal action.
		p.builder.FinishNode()
		return diags
	}
	verb := verbTok.Value

	switch verb {
	case "asks":
		p.consumeAs(SyntaxKindKwAsks)
		d := p.parseAsksAction(actionLine)
		diags = append(diags, d...)
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
		d := p.parseReturnsAction(actionLine)
		diags = append(diags, d...)
		p.builder.FinishNode()
		return diags
	default:
		// internal_action: <domain> <verb> [connector_word] <phrase>
		p.consumeAs(SyntaxKindIdent)
		// connector_word: a|an|the|as|to|from|in|on|at|for|with|by
		connTok := p.peek()
		if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == actionLine {
			p.consumeAs(SyntaxKindIdent)
		}
		start := p.opAnnotationStart(actionLine)
		p.collectPhrase(actionLine, start)
		if start >= 0 {
			diags = append(diags, p.parseOpAnnotation()...)
		}
		p.builder.FinishNode()
		return diags
	}
}

// parseAsksAction parses: <target> to|for <phrase>
// The `asks` keyword token has already been consumed by parseAction.
//
// A `kind:` prefix in the target slot is rejected: the slot already implies a
// bounded context, the same rule context_map enforces for its endpoints. The
// qualified `<domain>/<name>` form stays legal.
func (p *Parser) parseAsksAction(line int) []model.Diagnostic {
	var diags []model.Diagnostic
	targetTok := p.peek()
	if targetTok.Type == lexer.TokenIdent {
		if kind := p.parseRef(); kind != "" {
			diags = append(diags, p.diagKindPrefixInSlot(targetTok, kind, "an asks target"))
		}
	} else if isAnyKeywordAsIdent(targetTok.Type) {
		// "domain" and "service" lex as hard keywords rather than TokenIdent,
		// so a kind-prefixed target using one of them (e.g.
		// "domain:re/monetization", "service:billing-api") lands here, not in
		// the TokenIdent branch above. Only hand it to parseRef when it is
		// immediately followed by ':', since parseRef's own kind-prefix branch
		// handles that shape correctly. A bare keyword-as-ident with no colon
		// is not a ref shape; consume it directly instead, mirroring
		// parseEdgeEndpoint's guard against parseRef's zero-progress hole on
		// that shape (see the comment on parseEdgeEndpoint).
		hasColon := p.peekAt(1).Type == lexer.TokenColon && p.peekAt(1).Line == targetTok.Line && adjacentTokens(targetTok, p.peekAt(1))
		if hasColon {
			if kind := p.parseRef(); kind != "" {
				diags = append(diags, p.diagKindPrefixInSlot(targetTok, kind, "an asks target"))
			}
		} else {
			p.consumeAs(SyntaxKindIdent)
		}
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

	start := p.opAnnotationStart(line)
	p.collectPhrase(line, start)
	if start >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
	return diags
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
	// notifies has no phrase between the event and a possible annotation, so
	// the cursor is only correctly positioned on `[` when the annotation
	// starts immediately (lookahead index 0). A stray token in between (e.g.
	// a malformed multi-word event) must not be swallowed as if it were `[`.
	if p.opAnnotationStart(eventTok.Line) == 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
	return diags
}

// parseReturnsAction parses [to <target>] [connector_word] <phrase> after `returns`.
//
// The target names a bounded context, so it takes the same qualified
// `<domain>/<name>` form as the asks target and the action subject, and
// rejects a `kind:` prefix for the same reason.
func (p *Parser) parseReturnsAction(line int) []model.Diagnostic {
	var diags []model.Diagnostic
	// Check for optional `to <target>`
	if p.peek().Type == lexer.TokenIdent && p.peek().Value == "to" {
		p.consumeAs(SyntaxKindKwTo)
		targetTok := p.peek()
		if targetTok.Type == lexer.TokenIdent || isAnyKeywordAsIdent(targetTok.Type) {
			diags = append(diags, p.parseNameSlot("a returns target")...)
		}
	}

	// Optional connector_word before phrase (return_action: connector_word? phrase)
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && isConnectorWord(connTok.Value) && connTok.Line == line {
		p.consumeAs(SyntaxKindIdent)
	}

	start := p.opAnnotationStart(line)
	p.collectPhrase(line, start)
	if start >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
	return diags
}

// opAnnotationStart scans ahead over the remainder of actionLine and returns the
// lookahead index of the `[` that opens a trailing operation annotation, or -1
// when the line has none.
//
// The annotation is the LAST `[` on a line that ends in `]`. parseOpAnnotation
// stops at the first `]` after it and reports any token left on the line
// (`[0]{]`, `[GET /a[0]]`) as unexpected. Anchoring on the last bracket rather
// than the first is what lets a phrase keep using `[` as prose: `record [batch] entries [POST /v1/entries]` puts `[batch]` in
// the phrase and `[POST /v1/entries]` in the annotation. Requiring the line to
// end in `]` is what lets an unclosed `[` stay prose, so files that do not use
// the feature keep today's sweep-everything behaviour.
//
// A `}` terminates the scan unless it closes a `{` opened inside a still-open
// `[`. Braces are only tracked inside a bracket, because a templated path
// payload such as `[POST /v1/accounts/{id}/charges]` is by definition inside
// the annotation. Restricting depth tracking that way keeps this scan in step
// with collectPhrase, which returns unconditionally at the first `}`: a
// balanced `{...}` in the PHRASE (`charge {amount} [POST /pay]`, also after a
// closed bracket: `[x] {amount} [POST /pay]`) must terminate both, or the two
// disagree about where the line ends and parseOpAnnotation mis-anchors on the
// `}` instead of the `[`. A genuine block-closing `}` still terminates the
// scan exactly as before.
func (p *Parser) opAnnotationStart(actionLine int) int {
	lastLBracket, lastTok := -1, -1
	inBracket := false
	depth := 0
scan:
	for i := 0; ; i++ {
		t := p.peekAt(i)
		if t.Type == lexer.TokenEOF || t.Line != actionLine {
			break
		}
		switch t.Type {
		case lexer.TokenRBrace:
			if !inBracket || depth == 0 {
				break scan
			}
			depth--
		case lexer.TokenLBrace:
			if inBracket {
				depth++
			}
		case lexer.TokenLBracket:
			lastLBracket, inBracket, depth = i, true, 0
		case lexer.TokenRBracket:
			inBracket, depth = false, 0
		}
		lastTok = i
	}
	if lastLBracket < 0 || lastTok <= lastLBracket {
		return -1
	}
	if p.peekAt(lastTok).Type != lexer.TokenRBracket {
		return -1
	}
	return lastLBracket
}

// parseOpAnnotation consumes a `[ ... ]` operation annotation at the current
// position and emits it as a SyntaxKindOpAnnotation node. The caller must have
// confirmed via opAnnotationStart that a well-formed annotation begins here.
//
// The body is hybrid: a recognised uppercase protocol verb at the head is
// emitted as SyntaxKindOpVerb, and everything up to the closing `]` is swept in
// verbatim as payload. An unrecognised head word is just the first payload
// token, which is what keeps `[op1/op2/op3]` and `[legacy-txn-44]` legal.
func (p *Parser) parseOpAnnotation() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindOpAnnotation)
	lbTok := p.consumeAs(SyntaxKindLBracket)

	if rbTok := p.peek(); rbTok.Type == lexer.TokenRBracket {
		// Empty `[]` annotation: the body is missing, not empty-but-valid.
		diags = append(diags, p.diagEmptyOpAnnotation(lbTok, rbTok))
	}

	if t := p.peek(); (t.Type == lexer.TokenIdent || isAnyKeywordAsIdent(t.Type)) && lang.IsProtocolVerb(t.Value) {
		p.consumeAs(SyntaxKindOpVerb)
	}

	for !p.atEOF() && p.peek().Type != lexer.TokenRBracket {
		switch p.peek().Type {
		case lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case lexer.TokenNumber, lexer.TokenPercentage:
			p.consumeAs(SyntaxKindNumber)
		default:
			// Idents, keywords-as-idents, and TokenError punctuation (/ ? = { } …)
			// are all swept into the payload as raw tokens, mirroring collectPhrase.
			p.consumeAs(SyntaxKindIdent)
		}
	}

	rbLine := -1
	if rbTok := p.peek(); rbTok.Type == lexer.TokenRBracket {
		rbLine = rbTok.Line
		p.consumeAs(SyntaxKindRBracket)
		// An operation list (`[a, b]`, spec §8.1) must not hold an empty item.
		if rbTok.Offset > lbTok.End {
			if items := lang.SplitOperationItems(p.src[lbTok.End:rbTok.Offset]); len(items) > 1 {
				for _, it := range items {
					if it.Text == "" {
						diags = append(diags, p.diagEmptyOpAnnotationItem(lbTok, rbTok))
						break
					}
				}
			}
		}
	} else {
		// Defensive: opAnnotationStart guarantees the `]` exists.
		diags = append(diags, p.diagUnexpected(p.peek(), "]"))
	}

	p.builder.FinishNode()
	// The annotation closes at its first `]`. When the line ends in a later
	// `]` (`[0]{]`, `[GET /a[0]]`), the tokens in between are not prose and
	// not payload: report them, so the formatter refuses the file instead of
	// moving them.
	if rbLine >= 0 {
		diags = append(diags, p.sweepRestOfLine(rbLine, "the end of the line after the operation annotation")...)
	}
	return diags
}

// collectPhrase emits phrase tokens on actionLine into the current builder scope.
//
// The prose tail is display-only free text (see ActionDecl.PhraseText), so every
// same-line token is swept in verbatim, including TokenError punctuation such as
// `!`, `&`, `*`, `/`, until a line change, `}`, EOF, or a comment token ends it.
//
// maxTokens bounds how many tokens may be consumed: callers pass the lookahead
// index returned by opAnnotationStart so the phrase stops just before a trailing
// operation annotation. Pass -1 for "sweep to end of line".
func (p *Parser) collectPhrase(actionLine int, maxTokens int) {
	if maxTokens == 0 {
		return
	}
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return
	}
	if p.peek().Line != actionLine {
		return
	}
	consumed := 0
	for {
		if maxTokens >= 0 && consumed >= maxTokens {
			return
		}
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
			p.consumeAs(SyntaxKindIdent)
		}
		consumed++
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
//
// It returns the last token it discarded, which is what lets a caller report
// the extent of the loss instead of just the token that triggered it. The
// returned token is TokenEOF when nothing was discarded; most callers have
// already reported their own diagnostic and ignore it.
func (p *Parser) resyncToTopLevel() lexer.Token {
	last := lexer.Token{Type: lexer.TokenEOF}
	if p.atEOF() || isTopLevelKeyword(p.peek().Type) || p.atSurfaceDecl() {
		return last
	}
	p.builder.StartNode(SyntaxKindErrorNode)
	for !p.atEOF() {
		if isTopLevelKeyword(p.peek().Type) || p.atSurfaceDecl() {
			break
		}
		last = p.consume()
	}
	p.builder.FinishNode()
	return last
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
		lexer.TokenKwContextMap, lexer.TokenKwGlossary:
		return true
	}
	return false
}

// atColumnOneDecl reports whether the next token starts a top-level
// declaration at column 1: a surface, or a top-level keyword. A block body
// that meets one before its `}` is unclosed, and the parse resumes there
// instead of swallowing the declaration. A keyword that can also name a
// bounded context (`service`, `domain`, …) counts only when a `{` follows on
// its line (`actor` only when an actor kind and a name follow), so an
// unindented context list keeps parsing; an indented line is
// never a declaration, so `surface asks …` in a scenario stays an action.
func (p *Parser) atColumnOneDecl() bool {
	tok := p.peek()
	if tok.Column != 1 || !p.peekAtLineStart() {
		return false
	}
	if p.atSurfaceDecl() {
		return true
	}
	if !isTopLevelKeyword(tok.Type) {
		return false
	}
	if !isDomainNameToken(tok.Type) {
		return true
	}
	if tok.Type == lexer.TokenKwActor {
		// `actor <kind> <name>` on one line: a bare `actor` stays a context.
		kind, name := p.peekAt(1), p.peekAt(2)
		_, isKind := tokenToActorType(kind)
		return isKind && kind.Line == tok.Line && name.Line == tok.Line &&
			(name.Type == lexer.TokenIdent || name.Type == lexer.TokenString || isAnyKeywordAsIdent(name.Type))
	}
	for i := 1; i <= 2; i++ {
		n := p.peekAt(i)
		if n.Line != tok.Line {
			return false
		}
		if n.Type == lexer.TokenLBrace {
			return true
		}
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
	currentStart := green.TextSize(tok.Offset)
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
	p.prevEnd = green.TextSize(tok.End)
}

// rawText returns the token's verbatim source text. Token text in the green
// tree is always a slice of the source: this is what makes
// concat(AllTokens) == src true by construction rather than by convention.
// Never build token text from tok.Value or tok.Raw, which carry the token's
// interpreted content and diverge from source for strings and error tokens.
func (p *Parser) rawText(tok lexer.Token) string {
	if tok.Offset < 0 || tok.End > len(p.src) || tok.End < tok.Offset {
		return tok.Value // defensive: malformed range, should be unreachable
	}
	return p.src[tok.Offset:tok.End]
}

// attachTrivia emits any line/block comment tokens at p.pos into the current
// builder scope, advancing p.pos past each one collected.
func (p *Parser) attachTrivia() {
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		switch tok.Type {
		case lexer.TokenLineComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindLineComment, p.rawText(tok))
			p.updatePrevEnd(tok)
			p.pos++
		case lexer.TokenBlockComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindBlockComment, p.rawText(tok))
			p.updatePrevEnd(tok)
			p.pos++
		case lexer.TokenDocComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindDocComment, p.rawText(tok))
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
	p.builder.Token(lexerKindToSyntaxKind(tok.Type), p.rawText(tok))
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
	p.builder.Token(kind, p.rawText(tok))
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
	lexer.TokenKwGlossary:   SyntaxKindKwGlossary,
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

// lastConsumed returns the last significant token emitted into the tree.
func (p *Parser) lastConsumed() lexer.Token {
	for j := p.pos - 1; j >= 0; j-- {
		switch p.tokens[j].Type {
		case lexer.TokenLineComment, lexer.TokenBlockComment, lexer.TokenDocComment:
			continue
		}
		return p.tokens[j]
	}
	return lexer.Token{Type: lexer.TokenEOF}
}

// diagNoVerb reports an action line that has a subject but no verb on its
// line. first and last are the first and last tokens of the subject slot, so
// a qualified subject (`re/billing`) is named and spanned whole.
func (p *Parser) diagNoVerb(first, last lexer.Token) model.Diagnostic {
	name := p.src[first.Offset:last.End]
	return model.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  fmt.Sprintf("action %q has no verb: write `%s <verb> …` on one line", name, name),
		Severity: model.SeverityError,
		Range:    model.Range{Start: tokenRange(first).Start, End: p.tokenEndPosition(last)},
	}
}

func (p *Parser) diagUnexpected(tok lexer.Token, expected string) model.Diagnostic {
	return model.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  fmt.Sprintf("unexpected %q, expected %s", tok.Value, expected),
		Severity: model.SeverityError,
		Range:    tokenRange(tok),
	}
}

// diagEmptyOpAnnotation produces a craft/syntax/empty-op-annotation diagnostic
// for a `[]` operation annotation whose body is missing. lb and rb are the
// bracket tokens; the range spans from `[` through `]`.
func (p *Parser) diagEmptyOpAnnotation(lb, rb lexer.Token) model.Diagnostic {
	r := tokenRange(lb)
	r.End = tokenRange(rb).End
	return model.Diagnostic{
		Code:     "craft/syntax/empty-op-annotation",
		Message:  "empty operation annotation `[]`; expected a verb and/or payload, e.g. [POST /v1/charges]",
		Severity: model.SeverityError,
		Range:    r,
	}
}

// diagEmptyOpAnnotationItem reports an empty item of an operation list
// (`[a, , b]`, `[a, ]`), over the whole bracket.
func (p *Parser) diagEmptyOpAnnotationItem(lb, rb lexer.Token) model.Diagnostic {
	d := p.diagEmptyOpAnnotation(lb, rb)
	d.Message = "an operation bracket item is empty; write each item, e.g. [CreateApiKey, CreateUsagePlanKey]"
	return d
}

// diagKindPrefixInSlot produces a craft/syntax/kind-prefix-in-target
// diagnostic for a name slot that carries a recognised `kind:` prefix
// (bc:/domain:/service:/term:). Such a slot already implies what it names, so
// the prefix is rejected; the bare or qualified `<domain>/<name>` form is the
// only legal spelling. slot names the position, with its article, e.g.
// "an asks target". tok is the slot's leading token, captured before parseRef
// consumed it.
//
// The code string still says "in-target" even though the rule now fires in
// three slots that are not targets. That is deliberate: it shipped as a
// published diagnostic code in Task 5, and renaming it would break anyone
// filtering or suppressing on it. Do not "fix" it without a migration.
func (p *Parser) diagKindPrefixInSlot(tok lexer.Token, kind, slot string) model.Diagnostic {
	return model.Diagnostic{
		Code: "craft/syntax/kind-prefix-in-target",
		Message: fmt.Sprintf(
			"remove the %q prefix: %s is written as a bare name or in the <domain>/<name> form",
			kind+":", slot),
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

// diagNotImplemented reports a token the parser could not place.
//
// The message used to end with "use --parser=antlr for full support". No such
// flag exists on any subcommand; the ANTLR parser was removed when parser v2
// landed, and the text outlived it. Telling someone to pass a flag that does
// not exist is worse than saying nothing, and the C3 fix routes a real and
// reachable case here: a `{}` in an action's phrase means a trailing
// `[...]` annotation on that line cannot be recognised, so its `[` arrives
// unplaced.
func (p *Parser) diagNotImplemented(tok lexer.Token) model.Diagnostic {
	msg := fmt.Sprintf("unexpected %q here; this construct is not part of the Craft grammar and was skipped", tok.Value)
	if tok.Value == "[" {
		msg = "a trailing [operation] annotation is not recognised on a line whose phrase contains braces; " +
			"move the `{...}` out of the phrase, or drop the annotation from this line"
	}
	return model.Diagnostic{
		Code:     "craft/syntax/not-yet-implemented",
		Message:  msg,
		Severity: model.SeverityWarning,
		Range:    tokenRange(tok),
	}
}

// diagDanglingTagValue reports a comma in a tag value with no value after it.
// It is reported at the comma rather than at whatever follows, because what
// follows is usually the next statement and blaming it sends the reader to the
// wrong line.
func (p *Parser) diagDanglingTagValue(tok lexer.Token) model.Diagnostic {
	return model.Diagnostic{
		Code:     "craft/syntax/unexpected-token",
		Message:  "a tag value is missing after this comma; drop the comma, or add the next value",
		Severity: model.SeverityError,
		Range:    tokenRange(tok),
	}
}

// diagSkippedConstruct reports a top-level token the parser could not place at
// all, together with the region top-level recovery discarded because of it.
//
// This is deliberately separate from diagNotImplemented. That code means "the
// grammar recognises this, parser v2 does not implement it yet": the tool is
// behind, the file is fine, and a warning is right. This one means the file is
// ungrammatical and a region of it is now absent from the model, which every
// downstream consumer (check, inspect, generate, LSP) will silently agree
// never existed. That is content loss, so it is an error and `craft validate`
// fails on it without needing --strict.
//
// The range spans the whole discard, not just the offending token. The
// diagnostic used to point at the four characters of `when` while recovery ate
// the rest of the block: a reader was told one token was unexpected, not that
// a block had been deleted.
func (p *Parser) diagSkippedConstruct(tok, last lexer.Token, hint string) model.Diagnostic {
	r := tokenRange(tok)
	endLine := tok.Line
	if last.Type != lexer.TokenEOF {
		r.End = p.tokenEndPosition(last)
		endLine = last.Line
	}

	extent := fmt.Sprintf("line %d was skipped and is not part of the model", tok.Line)
	if endLine > tok.Line {
		extent = fmt.Sprintf("lines %d-%d were skipped and are not part of the model", tok.Line, endLine)
	}

	msg := fmt.Sprintf("unexpected %q at top level: %s", tok.Value, extent)
	if hint != "" {
		msg += "; " + hint
	}
	return model.Diagnostic{
		Code:     "craft/syntax/skipped-construct",
		Message:  msg,
		Severity: model.SeverityError,
		Range:    r,
	}
}

// strayBraceHint names the likely cause when a `when` block is stranded at top
// level. Nobody writes a top-level `when` on purpose; what happens is one
// extra `}` that closed the use_case early. Pointing at the `when` names the
// symptom, so point at the brace instead.
//
// It fires only when a use_case closed cleanly and nothing else was parsed
// since. A use_case that bailed to recovery never records a close, so there is
// no brace to blame and none is guessed at.
func (p *Parser) strayBraceHint(tok lexer.Token, prevWasUseCase bool) string {
	if !prevWasUseCase || p.lastUC.CloseLine == 0 {
		return ""
	}
	if tok.Type != lexer.TokenIdent || tok.Value != "when" {
		return ""
	}
	return fmt.Sprintf(
		"a `when` block belongs inside a use_case, and %q (opened at line %d) was already closed at line %d, so that `}` may be extra",
		p.lastUC.Name, p.lastUC.OpenLine, p.lastUC.CloseLine)
}

// tokenEndPosition returns the position one past tok's last character. Columns
// are rune-based and line-relative, matching tokenRange, so a token whose text
// spans lines has to restart the column count on its final line.
func (p *Parser) tokenEndPosition(tok lexer.Token) model.Position {
	raw := p.rawText(tok)
	line := lspLine(tok.Line)
	col := tok.Column - 1
	if col < 0 {
		col = 0
	}
	if nl := strings.LastIndexByte(raw, '\n'); nl >= 0 {
		line += strings.Count(raw, "\n")
		col = 0
		raw = raw[nl+1:]
	}
	return model.Position{Line: line, Character: col + len([]rune(raw))}
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

// parseContextMapBlock parses: context_map [domain] { edge_stmt* }
// The domain scope is an optional bare identifier between the `context_map`
// keyword and `{` (e.g. `context_map re { ... }`); it is stored as
// SyntaxKindContextMapDomain and surfaced via ContextMapDecl.Domain() (Task
// 3). Blocks are repeatable — a file may contain several context_map blocks,
// scoped or not, and their edges are merged by the projection layer.
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

	// Optional domain scope: `context_map <domain> { ... }`.
	if p.peek().Type == lexer.TokenIdent {
		p.consumeAs(SyntaxKindContextMapDomain)
	}

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
	if verb.Type == lexer.TokenIdent && lang.IsEdgeVerb(verb.Value) {
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
// — the Task 5 hang (`context_map { service customer_supplier billing }`,
// `context_map { billing separate_ways domain }`).
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

// parseGlossaryBlock parses: glossary [domain] { relation_stmt* }
// Mirrors parseContextMapBlock exactly: the domain scope is an optional bare
// identifier between the `glossary` keyword and `{` (e.g. `glossary re {
// ... }`), stored as SyntaxKindGlossaryDomain and surfaced via
// GlossaryDecl.Domain(). Blocks are repeatable — a file may contain several
// glossary blocks, scoped or not.
// relation_stmt := ref GLOSSARY_VERB ref, where GLOSSARY_VERB is a
// contextual keyword — one of the 3 symmetric term-relation verbs
// (same_as/contrasts/distinct_from) — matched by value, like context_map's
// edge verbs. Endpoint resolution/kind validation is sema's job, not the
// parser's.
func (p *Parser) parseGlossaryBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindGlossaryDecl)
	var diags []model.Diagnostic

	// Attach leading trivia before `glossary`.
	p.attachTrivia()
	kwTok := p.peek()
	p.consumeAs(SyntaxKindKwGlossary)

	// Optional domain scope: `glossary <domain> { ... }`.
	if p.peek().Type == lexer.TokenIdent {
		p.consumeAs(SyntaxKindGlossaryDomain)
	}

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
		diags = append(diags, p.parseGlossaryRelation()...)
		// Belt-and-suspenders forward-progress guard, mirroring
		// parseContextMapBlock's loop.
		if p.pos == posBefore {
			diags = append(diags, p.diagUnexpected(p.peek(), "a term reference, glossary verb, or `}`"))
			p.consumeAs(SyntaxKindError)
		}
	}

	if p.atEOF() {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed glossary block (missing `}`)",
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

// parseGlossaryRelation parses a single `ref GLOSSARY_VERB ref` term relation
// inside a glossary block, emitting it as a SyntaxKindGlossaryRelation node
// wrapping two SyntaxKindRef children (left/right endpoints) and one
// SyntaxKindEdgeKw token (the verb). Mirrors parseEdgeStmt; endpoints reuse
// parseEdgeEndpoint (they already accept bare/slash-path slugs such as
// billing/Invoice or re/billing/Invoice).
func (p *Parser) parseGlossaryRelation() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindGlossaryRelation)
	var diags []model.Diagnostic

	left := p.peek()
	if left.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(left.Type) {
		diags = append(diags, p.diagUnexpected(left, "a term reference (e.g. billing/Invoice)"))
		p.consumeAs(SyntaxKindError)
		p.builder.FinishNode()
		return diags
	}
	diags = append(diags, p.parseEdgeEndpoint()...) // left endpoint

	verb := p.peek()
	if verb.Type == lexer.TokenIdent && lang.IsGlossaryVerb(verb.Value) {
		p.consumeAs(SyntaxKindEdgeKw)
	} else {
		diags = append(diags, p.diagUnexpected(verb, "a glossary relation verb: same_as, contrasts, or distinct_from"))
		p.builder.FinishNode()
		return diags
	}

	right := p.peek()
	if right.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(right.Type) {
		diags = append(diags, p.diagUnexpected(right, "a term reference (e.g. billing/Invoice)"))
		p.builder.FinishNode()
		return diags
	}
	diags = append(diags, p.parseEdgeEndpoint()...) // right endpoint

	p.builder.FinishNode()
	return diags
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

// refShapedAhead reports whether the token at the cursor starts a MULTI-token
// reference: an ident (or keyword-as-ident) immediately followed, with no
// intervening whitespace and on the same line, by '/' (a qualified
// `<domain>/<name>` path) or by ':' after a recognised kind word (a `kind:`
// prefix, which the slot then rejects).
//
// Slots that accept a name use this to decide between parseRef and a plain
// consumeAs(SyntaxKindIdent). Routing only genuinely ref-shaped names through
// parseRef keeps the tree shape of a bare single-ident name exactly as it has
// always been, which matters most for the action/trigger subject: that is
// every action line in every existing .craft file, and every accessor that
// reads it positionally. Requiring the '/' or ':' also keeps the cursor off
// parseRef's zero-progress path for a bare keyword-as-ident.
//
// The colon case is deliberately narrower than parseRef's own kind-prefix
// branch: only `bc:`/`domain:`/`service:`/`term:` count. An unrecognised
// `foo: bar` in a subject slot is a typo, not a reference, and keeps
// reporting the unexpected-token error it always did instead of being
// silently absorbed into a ref node.
func (p *Parser) refShapedAhead() bool {
	first := p.peek()
	if first.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(first.Type) {
		return false
	}
	next := p.peekAt(1)
	isKindPrefix := next.Type == lexer.TokenColon && isSlugKind(first.Value)
	isPathSlash := next.Type == lexer.TokenError && next.Value == "/"
	if !isKindPrefix && !isPathSlash {
		return false
	}
	return next.Line == first.Line && adjacentTokens(first, next)
}

// refEndAhead returns the lookahead index just past the ref-shaped run that
// starts at offset: contiguous ident, number, '/' and colon tokens on one
// line, the run parseRef would consume.
func (p *Parser) refEndAhead(offset int) int {
	first := p.peekAt(offset)
	i := offset
	prev := first
	for {
		t := p.peekAt(i)
		ok := t.Type == lexer.TokenIdent || t.Type == lexer.TokenNumber || isAnyKeywordAsIdent(t.Type) ||
			(t.Type == lexer.TokenError && t.Value == "/") || t.Type == lexer.TokenColon
		if !ok || t.Line != first.Line || (i > offset && !adjacentTokens(prev, t)) {
			return i
		}
		prev = t
		i++
	}
}

// eventItemAhead reports whether the token at offset starts an event item of
// a list: a string, or a ref whose run is followed by a comma, the end of
// its line, `}`, EOF, an operation bracket or a `cron`/`every` schedule.
// Anything else is a step line (`seshat writes the row`), which the list
// must not swallow.
func (p *Parser) eventItemAhead(offset int) bool {
	t := p.peekAt(offset)
	switch {
	case t.Type == lexer.TokenString:
		return true
	case t.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(t.Type):
		return false
	}
	end := p.refEndAhead(offset)
	next := p.peekAt(end)
	switch {
	case next.Type == lexer.TokenEOF, next.Type == lexer.TokenRBrace, next.Line != t.Line,
		next.Type == lexer.TokenComma, next.Type == lexer.TokenLBracket:
		return true
	case next.Type == lexer.TokenIdent && (next.Value == "cron" || next.Value == "every"):
		return p.peekAt(end+1).Type == lexer.TokenString
	}
	return false
}

// parseEventItem consumes one event item: a quoted string or a ref.
func (p *Parser) parseEventItem() {
	switch t := p.peek(); {
	case t.Type == lexer.TokenString:
		p.consumeAs(SyntaxKindString)
	case t.Type == lexer.TokenIdent:
		p.parseRef()
	default:
		p.consumeAs(SyntaxKindIdent)
	}
}

// parseEventList parses `event_item (',' NL* event_item)*` (spec §4.1). The
// items are wrapped in a SyntaxKindEventList only when a comma is seen, so a
// single item keeps the v2.23 tree. A comma with no item after it is
// reported on the comma; the next line then parses as a step. It returns the
// last token consumed, whose line is where a trailing schedule or bracket
// must sit.
func (p *Parser) parseEventList() (lexer.Token, []model.Diagnostic) {
	var diags []model.Diagnostic
	cp := p.builder.Checkpoint()
	p.parseEventItem()
	commas := 0
	for p.peek().Type == lexer.TokenComma {
		comma := p.consumeAs(SyntaxKindComma)
		commas++
		if !p.eventItemAhead(0) {
			diags = append(diags, p.diagUnexpected(comma, "an event after `,`"))
			break
		}
		p.parseEventItem()
	}
	if commas > 0 {
		p.builder.StartNodeAt(cp, SyntaxKindEventList)
		p.builder.FinishNode()
	}
	return p.lastConsumed(), diags
}

// parseNameSlot consumes one name slot: a qualified or kind-prefixed ref via
// parseRef when the cursor is on a ref shape, else a single ident token. slot
// names the position for the diagnostic raised when a `kind:` prefix is
// present, e.g. "an action subject".
func (p *Parser) parseNameSlot(slot string) []model.Diagnostic {
	tok := p.peek()
	if !p.refShapedAhead() {
		p.consumeAs(SyntaxKindIdent)
		return nil
	}
	if kind := p.parseRef(); kind != "" {
		return []model.Diagnostic{p.diagKindPrefixInSlot(tok, kind, slot)}
	}
	return nil
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
// SyntaxKindIdent — consumeAs always stores the raw source slice regardless
// of the kind passed in (rawText slices the source the same way no matter
// which kind is passed, so this does not affect round-trip output), but
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

// --- surface parsing (v2.23) ---

// atSurfaceDecl reports whether the next tokens open a surface: the contextual
// keyword `surface`, then a name (bare or `<domain>/<name>`) and `{`, all on
// one line. Anywhere else `surface` is an identifier.
func (p *Parser) atSurfaceDecl() bool {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || tok.Value != "surface" {
		return false
	}
	name := p.peekAt(1)
	if (name.Type != lexer.TokenIdent && !isKeywordUsedAsIdent(name.Type)) || name.Line != tok.Line {
		return false
	}
	next := p.peekAt(2)
	if next.Type == lexer.TokenLBrace {
		return next.Line == tok.Line
	}
	brace := p.peekAt(4)
	return next.Value == "/" && p.peekAt(3).Type == lexer.TokenIdent && brace.Type == lexer.TokenLBrace && brace.Line == tok.Line
}

// parseSurfaceBlock parses: surface <name> { surface_item* }. The body ends
// at `}`; a top-level keyword or another surface before it means the block is
// unclosed, and the parse resumes there.
func (p *Parser) parseSurfaceBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindSurfaceDecl)
	p.attachTrivia()
	kwTok := p.consumeAs(SyntaxKindKwSurface)
	diags := p.parseNameSlot("a surface subject")
	p.consumeAs(SyntaxKindLBrace) // atSurfaceDecl checked it
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace && !isTopLevelKeyword(p.peek().Type) && !p.atSurfaceDecl() {
		diags = append(diags, p.parseSurfaceItem()...)
	}
	if p.peek().Type != lexer.TokenRBrace {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed surface block (missing `}`)",
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

// parseSurfaceItem parses one line of a surface body: `callers:`, `spec:`, a
// note or finding, or a `query` / `command` line. Any other line is reported
// and skipped.
func (p *Parser) parseSurfaceItem() []model.Diagnostic {
	tok := p.peek()
	var diags []model.Diagnostic
	switch {
	case tok.Type == lexer.TokenIdent && (tok.Value == "callers" || tok.Value == "spec") && p.peekAt(1).Type == lexer.TokenColon:
		p.builder.StartNode(SyntaxKindSurfaceField)
		if tok.Value == "callers" {
			p.consumeAs(SyntaxKindKwCallers)
			p.consumeAs(SyntaxKindColon)
			if n := p.peek(); n.Line != tok.Line || (n.Type != lexer.TokenIdent && n.Type != lexer.TokenString && !isKeywordUsedAsIdent(n.Type)) {
				diags = append(diags, p.diagUnexpected(n, "a caller name or `unknown`, e.g. `callers: Advisor`"))
			} else {
				p.parseRefList()
			}
		} else {
			p.consumeAs(SyntaxKindKwSpec)
			p.consumeAs(SyntaxKindColon)
			if s := p.peek(); s.Type == lexer.TokenString && s.Line == tok.Line {
				p.consumeAs(SyntaxKindString)
			} else {
				diags = append(diags, p.diagUnexpected(s, "a quoted spec path, e.g. `spec: \"docs/openapi.yaml\"`"))
			}
		}
		p.builder.FinishNode()
	case p.atNoteLine():
		return p.parseNote()
	case tok.Type == lexer.TokenIdent && (tok.Value == "query" || tok.Value == "command"):
		return p.parseSurfaceOperation()
	default:
		return p.sweepRestOfLine(tok.Line, "`callers:`, `spec:`, `note`, `finding`, `query`, `command` or `}`")
	}
	if len(diags) > 0 {
		// The line is already reported; skip its rest silently.
		return append(diags, p.skipRestOfLine(tok.Line)...)
	}
	return p.sweepRestOfLine(tok.Line, "end of line")
}

// skipRestOfLine wraps whatever is left on line in an error node without a
// diagnostic of its own, for a line that has already been reported.
func (p *Parser) skipRestOfLine(line int) []model.Diagnostic {
	t := p.peek()
	if t.Line != line || t.Type == lexer.TokenEOF || t.Type == lexer.TokenRBrace {
		return nil
	}
	p.builder.StartNode(SyntaxKindErrorNode)
	for t := p.peek(); t.Line == line && t.Type != lexer.TokenEOF && t.Type != lexer.TokenRBrace; t = p.peek() {
		p.consume()
	}
	p.builder.FinishNode()
	return nil
}

// parseSurfaceOperation parses: query|command [connector] phrase [op]. The
// operation is the entry's identity, so a line without one is an error and is
// left out of the model.
func (p *Parser) parseSurfaceOperation() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindSurfaceOperation)
	kind := SyntaxKindKwQuery
	if p.peek().Value == "command" {
		kind = SyntaxKindKwCommand
	}
	kwTok := p.consumeAs(kind)
	if c := p.peek(); c.Type == lexer.TokenIdent && isSurfaceConnector(c.Value) && c.Line == kwTok.Line {
		p.consumeAs(SyntaxKindIdent)
	}
	start := p.opAnnotationStart(kwTok.Line)
	p.collectPhrase(kwTok.Line, start)
	if start == 0 {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-phrase",
			Message:  fmt.Sprintf("a `%s` line says what the operation offers before its bracket, e.g. `%s the seeker list  [getUserList]`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	if start >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-operation",
			Message:  fmt.Sprintf("a `%s` line needs its operation in brackets, e.g. `%s the seeker list  [getUserList]`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	p.builder.FinishNode()
	return diags
}

// isSurfaceConnector reports whether v is the connector of a surface line: a
// connector word that is not an article. An article belongs to the noun the
// line offers (`query the seeker list`), so it stays in the phrase, while a
// preposition (`query for the daily totals`) is the connector.
func isSurfaceConnector(v string) bool {
	switch v {
	case "a", "an", "the":
		return false
	}
	return isConnectorWord(v)
}
