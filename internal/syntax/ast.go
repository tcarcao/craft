package syntax

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
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
	_, col := li.LineCol16(toks[0].Offset())
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

// significantElements returns n's direct children, excluding trivia, WITHOUT
// flattening nested nodes — unlike Tokens()/AllTokens(), a SyntaxKindRef
// child (wrapped by parseRef, Task 3) stays a single element here instead of
// expanding into its interior leaf tokens (kind word, ':', segments, '/').
//
// Several action/trigger accessors below used to index into the node's flat
// Tokens() slice on the assumption that every "slot" (target, event) is
// exactly one token wide. That assumption breaks for a kind-prefixed slug
// ref like "bc:re/billing", which is FIVE leaf tokens (bc, :, re, /,
// billing) but must still occupy exactly one logical slot. Accessors that
// need positional slot access use significantElements instead of Tokens().
func significantElements(n SyntaxNode) []SyntaxElement {
	var result []SyntaxElement
	for el := range n.ChildrenIter() {
		if isTrivia(el.Kind()) {
			continue
		}
		result = append(result, el)
	}
	return result
}

// elementSpan returns how many flat (leaf, non-trivia) tokens el occupies in
// the node's Tokens() slice: 1 for a plain token, or the ref node's own
// token count for a SyntaxKindRef node. Used to translate a significantElements
// index into the equivalent Tokens() index (e.g. to locate the connector that
// follows a possibly multi-token ref).
func elementSpan(el SyntaxElement) int {
	if node, ok := el.(SyntaxNode); ok {
		return len(node.Tokens())
	}
	return 1
}

// tokenIndexAt translates a significantElements index into the equivalent
// index in the node's flat Tokens() slice, by summing the spans of all
// elements before it. Positional slot accessors below use it because no slot
// has a fixed token width any more: a target (Task 4) or a subject (Task 6b)
// written as a ref, e.g. "re/billing" or "bc:re/billing", is a single
// SyntaxKindRef element covering several leaf tokens. i is clamped to
// len(elems), so an index past a truncated node yields the end of its tokens
// rather than panicking.
func tokenIndexAt(elems []SyntaxElement, i int) int {
	if i > len(elems) {
		i = len(elems)
	}
	idx := 0
	for j := 0; j < i; j++ {
		idx += elementSpan(elems[j])
	}
	return idx
}

// slotFirstToken returns the first leaf token of a positional slot element:
// the token itself for a leaf, or a SyntaxKindRef node's first token. Returns
// nil when the element holds no tokens.
func slotFirstToken(el SyntaxElement) *SyntaxToken {
	switch v := el.(type) {
	case SyntaxToken:
		return &v
	case SyntaxNode:
		if toks := v.Tokens(); len(toks) > 0 {
			return &toks[0]
		}
	}
	return nil
}

// nameSlotElement returns elems[0] when it holds a name: a plain Ident token
// for a bare name, or a SyntaxKindRef node for a qualified one (Task 6b).
// Returns nil otherwise, e.g. for an error-recovery node whose first element
// is a SyntaxKindError token, or a `when cron "..."` trigger whose first
// element is the cron keyword.
func nameSlotElement(elems []SyntaxElement) SyntaxElement {
	if len(elems) == 0 {
		return nil
	}
	if k := elems[0].Kind(); k != SyntaxKindIdent && k != SyntaxKindRef {
		return nil
	}
	return elems[0]
}

// refAwareText returns the source text of a significant child element: for a
// SyntaxKindRef node (parseRef, Task 3/4) the full reconstructed ref text via
// RefDecl.RefText(); for a leaf token, its own text. Do NOT read a ref
// position with ChildToken(SyntaxKindIdent)/RefDecl.Name() — for a
// kind-prefixed slug like "bc:re/billing" that returns only the leading kind
// word "bc".
func refAwareText(el SyntaxElement) string {
	switch v := el.(type) {
	case SyntaxNode:
		if v.Kind() == SyntaxKindRef {
			return RefDecl{node: v}.RefText()
		}
		return ""
	case SyntaxToken:
		return stringAwareText(v)
	}
	return ""
}

// stringAwareText returns a leaf token's semantic content: for a
// SyntaxKindString token, tok.Text() is the exact raw source text
// including both quotes (Bug 8a fix — see tokenText in parser.go), so this
// strips the surrounding quotes and resolves escape sequences to recover
// the same unescaped value lexer.Token.Value held. For any other kind,
// Text() is already the semantic value (no quotes involved), so it is
// returned unchanged.
func stringAwareText(tok SyntaxToken) string {
	if tok.Kind() != SyntaxKindString {
		return tok.Text()
	}
	return unquoteStringText(tok.Text())
}

// StringAwareText is the exported form of stringAwareText, for content-read
// call sites outside this package (e.g. internal/lsp) that receive a raw
// token — such as one from ServiceDecl.ContextTokens() — and need its
// semantic (unquoted) value rather than its raw source text. Keeping the
// unquoting logic itself unexported and routing all callers, in- and
// out-of-package, through this single entry point avoids a second
// reimplementation of the escape handling (see unquoteStringText).
func StringAwareText(tok SyntaxToken) string {
	return stringAwareText(tok)
}

// unquoteStringText strips the surrounding double quotes from raw (a
// SyntaxKindString token's raw source text) and resolves the lexer's
// escape sequences (\", \\, \n, \t, \r; an unrecognised escape passes
// through as backslash + char), mirroring lexer.Lexer.scanString exactly.
// If raw isn't quote-delimited, it's returned unchanged (defensive).
func unquoteStringText(raw string) string {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return raw
	}
	inner := raw[1 : len(raw)-1]
	if !strings.ContainsRune(inner, '\\') {
		return inner
	}
	var sb strings.Builder
	sb.Grow(len(inner))
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		if ch == '\\' && i+1 < len(inner) {
			i++
			switch inner[i] {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(inner[i])
			}
			continue
		}
		sb.WriteByte(ch)
	}
	return sb.String()
}

// refAwareOffset returns the byte offset of a significant child element,
// whether it is a leaf token or a SyntaxKindRef node.
//
// For a node, this must NOT use v.Offset() directly: leading whitespace
// trivia attaches as a node's own first child throughout this parser (the
// same convention nodeFirstTokenCol/nodeFirstTokenLine above already exist
// to work around), so a SyntaxKindRef node's raw offset is the start of the
// whitespace BEFORE the ref, not the ref's first real character. Use its
// first non-trivia token's offset instead, matching that existing pattern.
func refAwareOffset(el SyntaxElement) green.TextSize {
	switch v := el.(type) {
	case SyntaxNode:
		toks := v.Tokens()
		if len(toks) > 0 {
			return toks[0].Offset()
		}
		return v.Offset()
	case SyntaxToken:
		return v.Offset()
	}
	return 0
}

// refIfWrapped returns el's RefText() if it is a SyntaxKindRef node — i.e.
// the slot was parsed via parseRef (Task 4) rather than the legacy quoted
// string form — else "". Used by the pkg/craft projection to populate the
// additive Ref field only for typed-ref object positions, leaving it empty
// for `notifies "X"` / `listens "X"`.
func refIfWrapped(el SyntaxElement) string {
	if node, ok := el.(SyntaxNode); ok && node.Kind() == SyntaxKindRef {
		return RefDecl{node: node}.RefText()
	}
	return ""
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
		k == SyntaxKindKwLanguage || k == SyntaxKindKwDeployment ||
		k == SyntaxKindKwCatalogRef || k == SyntaxKindKwRepo
}

// serviceFieldName returns the field name string for a token that is a service
// body field keyword (contexts, data-stores, language, deployment, catalog_ref,
// repo) or a plain ident. Returns "" for other token kinds.
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
	case SyntaxKindKwCatalogRef:
		return "catalog_ref"
	case SyntaxKindKwRepo:
		return "repo"
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
			names = append(names, stringAwareText(tok))
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

// ContextMaps returns all ContextMapDecl views.
func (f File) ContextMaps() []ContextMapDecl {
	if f.isZero() {
		return nil
	}
	var result []ContextMapDecl
	for _, n := range f.node.ChildNodes(SyntaxKindContextMapDecl) {
		result = append(result, ContextMapDecl{node: n})
	}
	return result
}

// Glossaries returns all GlossaryDecl views.
func (f File) Glossaries() []GlossaryDecl {
	if f.isZero() {
		return nil
	}
	var result []GlossaryDecl
	for _, n := range f.node.ChildNodes(SyntaxKindGlossaryDecl) {
		result = append(result, GlossaryDecl{node: n})
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

// Name returns the identifier-or-string token for the domain's name. A
// quoted domain name (if the grammar ever allows one — service names
// already do, per docs/GRAMMAR.md service_name = identifier | STRING) lexes
// as SyntaxKindString, not SyntaxKindIdent, so both kinds are accepted here
// (mirrors ServiceDecl.Name() below). Content-reading callers must go
// through StringAwareText, not Text(), to get the unquoted value.
func (d DomainDecl) Name() *SyntaxToken { return d.node.ChildToken(SyntaxKindIdent, SyntaxKindString) }

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

// RefText returns the raw source text of a reference wrapped by parseRef —
// e.g. "vas.VasApplied" or "bc:re/subscriptions". A valid ref has no
// whitespace between its tokens, so concatenating the node's non-trivia
// token texts reconstructs the exact original span.
func (r RefDecl) RefText() string {
	var sb strings.Builder
	for _, tok := range r.node.Tokens() {
		sb.WriteString(tok.Text())
	}
	return sb.String()
}

// RefKind returns the leading kind word ("domain"/"bc"/"term"/"service") if
// this ref used a recognised `kind:` prefix, else "".
func (r RefDecl) RefKind() string {
	toks := r.node.Tokens()
	if len(toks) >= 2 && toks[0].Kind() == SyntaxKindIdent && toks[1].Kind() == SyntaxKindColon && isSlugKind(toks[0].Text()) {
		return toks[0].Text()
	}
	return ""
}

// Line returns the 1-based source line of this ref's first token using li
// (Task 7 — position for slug-shape diagnostics).
func (r RefDecl) Line(li green.LineIndex) int { return nodeFirstTokenLine(r.node, li) }

// Col returns the 1-based column of this ref's first token using li
// (Task 7 — position for slug-shape diagnostics).
func (r RefDecl) Col(li green.LineIndex) int { return nodeFirstTokenCol(r.node, li) }

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

// IsCatalogRef reports whether this is a catalog_ref: field.
func (sf ServiceField) IsCatalogRef() bool {
	return sf.node.ChildToken(SyntaxKindKwCatalogRef) != nil
}

// IsRepo reports whether this is a repo: field.
func (sf ServiceField) IsRepo() bool {
	return sf.node.ChildToken(SyntaxKindKwRepo) != nil
}

// Ref returns the RefDecl view of this field's value when it was parsed via
// parseRef (e.g. repo:), or nil if the field has no ref-wrapped value.
func (sf ServiceField) Ref() *RefDecl {
	n := sf.node.ChildNode(SyntaxKindRef)
	if n == nil {
		return nil
	}
	return &RefDecl{node: *n}
}

// Line returns the 1-based source line of this field's first token using li
// (Task 7 — position for duplicate-service-anchor diagnostics).
func (sf ServiceField) Line(li green.LineIndex) int { return nodeFirstTokenLine(sf.node, li) }

// Col returns the 1-based column of this field's first token using li
// (Task 7 — position for duplicate-service-anchor diagnostics).
func (sf ServiceField) Col(li green.LineIndex) int { return nodeFirstTokenCol(sf.node, li) }

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

// Name returns the identifier-or-string token for the service's name. The
// grammar allows a QUOTED service name (docs/GRAMMAR.md: service_name =
// identifier | STRING), which lexes as SyntaxKindString, not SyntaxKindIdent
// — so both kinds are accepted here (previously Ident-only, which made
// Name() return nil for a quoted service and silently broke callers that
// used it for duplicate-name detection; see serviceNameTok in projection.go
// for the same first-Ident-or-String pattern this mirrors). Content-reading
// callers must go through StringAwareText, not Text(), to get the unquoted
// value; position/offset-only callers are unaffected.
func (s ServiceDecl) Name() *SyntaxToken { return s.node.ChildToken(SyntaxKindIdent, SyntaxKindString) }

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
	CatalogRef      string
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
		case "catalog_ref":
			if i < len(tokens) && tokens[i].Kind() == SyntaxKindIdent {
				f.CatalogRef = tokens[i].Text()
				i++
			}
		case "repo":
			// The repo: value is a ref (possibly multi-token, e.g. a
			// slash-bearing slug) wrapped in a SyntaxKindRef node by
			// parseRef. Its text is read via the tree (ServiceDecl.Repo,
			// ServiceField.Ref) rather than here; just skip past its
			// tokens to keep this flat scan's cursor in sync.
			for i < len(tokens) {
				isNextFieldSentinel := serviceFieldName(tokens[i]) != "" &&
					i+1 < len(tokens) && tokens[i+1].Kind() == SyntaxKindColon
				if tokens[i].Kind() == SyntaxKindRBrace || isNextFieldSentinel {
					break
				}
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

// CatalogRef returns the catalog_ref: value — the service's stable identifier
// in the org's service catalog — or "" if absent. The language deliberately
// does not name the catalog vendor: which catalog resolves the anchor is
// deployment configuration, not part of the grammar.
func (s ServiceDecl) CatalogRef() string { return s.parseServiceBody().CatalogRef }

// Repo returns the repo: value as its full ref text (e.g.
// "olxeu/realestate/subscriptions"), or "" if absent. The value was parsed
// via parseRef, so a slash-bearing slug is read back as one contiguous
// string via RefDecl.RefText().
func (s ServiceDecl) Repo() string {
	// Mirrors Language()'s duplicate handling: if repo: appears more than
	// once, the last occurrence wins (no diagnostic — parser-level dup
	// detection is out of scope for Task 6; see task brief).
	var repo string
	for _, f := range s.Fields() {
		if !f.IsRepo() {
			continue
		}
		if ref := f.Ref(); ref != nil {
			repo = ref.RefText()
		}
	}
	return repo
}

// DeploymentRules returns the percentage→target rules for parameterised deployment.
func (s ServiceDecl) DeploymentRules() []struct{ Percentage, Target string } {
	return s.parseServiceBody().DeploymentRules
}

// DataStoreTokens returns the SyntaxToken for each data-store name in the
// service body. A data-store entry may be SyntaxKindIdent or SyntaxKindString
// (a quoted name, e.g. `data-stores: "user db"`) — mirrors the Ident-or-String
// filter used by collectAstIdentList (which DataStores() delegates to) so
// this raw-token scan doesn't silently truncate the list at a quoted entry.
// Callers reading token content must unquote via stringAwareText; callers
// only reading position/length (as today's semantic-highlighting caller
// does) can use the raw token directly.
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
				if tokens[i].Kind() == SyntaxKindIdent || tokens[i].Kind() == SyntaxKindString {
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

// Title returns the raw quoted-string title token (Text() includes both
// quotes — Bug 8a fix). Content consumers that want the unquoted use_case
// name should call Name() instead.
func (u UseCaseDecl) Title() *SyntaxToken { return u.node.ChildToken(SyntaxKindString) }

// Name returns the unquoted use_case title text.
func (u UseCaseDecl) Name() string {
	tok := u.Title()
	if tok == nil {
		return ""
	}
	return stringAwareText(*tok)
}

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

// TagsBlock returns the single tags { } block child of this use case, or nil
// if it has none (Task 3, Slice B). The grammar allows at most one tags
// block per use_case; if more than one were somehow present, this returns
// the first (document order).
func (u UseCaseDecl) TagsBlock() *TagsBlock {
	n := u.node.ChildNode(SyntaxKindTagsBlock)
	if n == nil {
		return nil
	}
	return &TagsBlock{node: *n}
}

// TagsBlocks returns all tags { } block children of this use case, in
// document order. The grammar allows at most one; a second block is a
// craft/sema/duplicate-tag condition (Task 4, Slice B). TagsBlock() above
// returns just the first (the common case); this enumerates all of them so
// sema can flag a second one.
func (u UseCaseDecl) TagsBlocks() []TagsBlock {
	var result []TagsBlock
	for _, n := range u.node.ChildNodes(SyntaxKindTagsBlock) {
		result = append(result, TagsBlock{node: n})
	}
	return result
}

// TagsBlock is a typed view over a SyntaxKindTagsBlock node: `tags { tag_stmt* }`.
type TagsBlock struct{ node SyntaxNode }

// Keyword returns the `tags` keyword token introducing this block.
func (b TagsBlock) Keyword() *SyntaxToken { return b.node.ChildToken(SyntaxKindKwTags) }

// Tags returns all TagStmt views within this tags block, in document order.
func (b TagsBlock) Tags() []TagStmt {
	var result []TagStmt
	for _, n := range b.node.ChildNodes(SyntaxKindTagStmt) {
		result = append(result, TagStmt{node: n})
	}
	return result
}

// TagStmt is a typed view over a SyntaxKindTagStmt node:
// `IDENT ':' (IDENT | STRING | ref-shaped-slug)`.
type TagStmt struct{ node SyntaxNode }

// Key returns the tag key identifier token.
func (t TagStmt) Key() *SyntaxToken { return t.node.ChildToken(SyntaxKindIdent) }

// Value returns the tag's value token when the value is a single leaf token
// — i.e. a quoted string (SyntaxKindString). Returns nil when the value is a
// bare ref-shaped value (wrapped in a SyntaxKindRef child by parseTagStmt,
// since a bare value like "re/renewal-flow" may span multiple leaf tokens —
// see parseTagStmt's doc comment). Use ValueText for the general case that
// handles both shapes uniformly.
func (t TagStmt) Value() *SyntaxToken {
	return t.node.ChildToken(SyntaxKindString)
}

// ValueText returns the tag's semantic (unquoted) value text, handling both
// a quoted string (unquoted via stringAwareText, the same path used for
// other string values) and a bare ref-shaped value (reconstructed via
// RefDecl.RefText(), the same path used elsewhere in this file for
// multi-token ref-shaped values — see refAwareText).
func (t TagStmt) ValueText() string {
	if n := t.node.ChildNode(SyntaxKindRef); n != nil {
		return RefDecl{node: *n}.RefText()
	}
	if tok := t.Value(); tok != nil {
		return stringAwareText(*tok)
	}
	return ""
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

// subjectElement returns the trigger's subject slot: significant element 0,
// which is a plain Ident token for a bare name or a SyntaxKindRef node for a
// qualified one (Task 6b, e.g. `when re/billing listens ...`). Returns nil
// when the trigger has no name subject (event/cron/every forms).
func (t TriggerDecl) subjectElement() SyntaxElement {
	return nameSlotElement(significantElements(t.node))
}

// Subject returns the first token of the trigger subject (actor/domain name).
// For a qualified subject that is the ref's leading segment, which is where
// the whole reference starts.
func (t TriggerDecl) Subject() *SyntaxToken {
	el := t.subjectElement()
	if el == nil {
		return nil
	}
	return slotFirstToken(el)
}

// SubjectSpan returns how many flat tokens the trigger subject occupies in
// Tokens(): 1 for a bare name, more for a qualified ref ("re/billing" is
// three). Exported so internal/lsp can skip exactly the subject when colouring
// the remaining trigger phrase words, instead of skipping one token and
// mis-colouring the rest of a qualified subject as phrase text.
func (t TriggerDecl) SubjectSpan() int {
	elems := significantElements(t.node)
	if len(elems) == 0 {
		return 0
	}
	return elementSpan(elems[0])
}

// Tokens returns all syntax tokens in the trigger node (mirrors ActionDecl.Tokens).
func (t TriggerDecl) Tokens() []SyntaxToken { return t.node.Tokens() }

// Event returns the string token for event-style triggers (when "<EventName>").
func (t TriggerDecl) Event() *SyntaxToken { return t.node.ChildToken(SyntaxKindString) }

// Kind returns "external", "event", or "domain_listen" based on token structure.
// Mirrors lowerTrigger classification in lower.go.
func (t TriggerDecl) Kind() string {
	// Read slots as elements, not flat tokens: a qualified subject (Task 6b)
	// is one Ref element but several tokens, which would push `listens` past
	// a fixed token index 1.
	elems := significantElements(t.node)
	if len(elems) == 0 {
		return "external"
	}
	// event trigger: first element is a string literal
	if elems[0].Kind() == SyntaxKindString {
		return "event"
	}
	// domain_listen: second element is `listens`
	if len(elems) >= 2 && elems[1].Kind() == SyntaxKindKwListens {
		return "domain_listen"
	}
	return "external"
}

// ActorName returns the trigger subject for external triggers (the actor/domain
// name). The subject may be a parseRef-wrapped qualified ref (Task 6b), so
// refAwareText reads the full "re/billing" rather than its first segment.
func (t TriggerDecl) ActorName() string {
	if t.Kind() != "external" {
		return ""
	}
	el := t.subjectElement()
	if el == nil {
		return ""
	}
	return refAwareText(el)
}

// ActorCol returns the 1-based column of the subject using li.
// Works for both external and domain_listen triggers.
func (t TriggerDecl) ActorCol(li green.LineIndex) int {
	el := t.subjectElement()
	if el == nil {
		return 0
	}
	_, col := li.LineCol16(refAwareOffset(el))
	return col
}

// ContextName returns the subject name for domain_listen triggers, reading a
// qualified ref (Task 6b) in full via refAwareText.
func (t TriggerDecl) ContextName() string {
	if t.Kind() != "domain_listen" {
		return ""
	}
	el := t.subjectElement()
	if el == nil {
		return ""
	}
	return refAwareText(el)
}

// EventValue returns the event name (for event and domain_listen triggers).
// For domain_listen the event slot may be a quoted string or a parseRef-
// wrapped ref (Task 4) — use refAwareText so a kind-prefixed slug like
// "bc:re/billing" round-trips in full rather than truncating to its first
// leaf token.
func (t TriggerDecl) EventValue() string {
	switch t.Kind() {
	case "event":
		tokens := t.node.Tokens()
		if len(tokens) > 0 {
			return stringAwareText(tokens[0])
		}
	case "domain_listen":
		elems := significantElements(t.node)
		if len(elems) >= 3 {
			return refAwareText(elems[2])
		}
	}
	return ""
}

// Description returns the human-readable trigger line (without the leading `when`).
// String tokens' raw Text() already includes both quotes (Bug 8a fix), so no
// manual quote-wrapping is needed here — that used to double the quotes.
//
// This walks significant ELEMENTS, not flat tokens. A space-joined token walk
// is a one-token-per-slot assumption in disguise: a SyntaxKindRef slot such as
// a qualified subject (Task 6b) or a ref-wrapped listens event (Task 4)
// flattens into several leaf tokens, so joining them with spaces emitted
// "re / billing". The LSP formatter rebuilds trigger lines from this string,
// so that turned a valid document into one that no longer parsed. A ref
// contributes its reconstructed text as a single part; every other shape
// contributes its leaf tokens individually, exactly as before.
func (t TriggerDecl) Description() string {
	var parts []string
	for _, el := range significantElements(t.node) {
		switch v := el.(type) {
		case SyntaxNode:
			if v.Kind() == SyntaxKindRef {
				parts = append(parts, RefDecl{node: v}.RefText())
				continue
			}
			for _, tok := range v.Tokens() {
				parts = append(parts, tok.Text())
			}
		case SyntaxToken:
			parts = append(parts, v.Text())
		}
	}
	return strings.Join(parts, " ")
}

// PhraseText returns the free-text phrase for an "external" trigger (e.g.
// `when Actor verb <phrase>`), as the exact raw source substring spanning
// the phrase tokens (including any inter-token whitespace) — mirrors
// ActionDecl.PhraseText (Task 1 / Bug 8a) so prose with tight punctuation
// keeps its original spacing instead of being space-joined.
func (t TriggerDecl) PhraseText() string {
	if t.Kind() != "external" {
		return ""
	}
	tokens := t.node.Tokens()
	// subject, verb — the subject may be a multi-token qualified ref (Task 6b),
	// so convert from element positions instead of starting at token 2.
	phraseStart := tokenIndexAt(significantElements(t.node), 2)
	if phraseStart >= len(tokens) {
		return ""
	}
	if isConnectorWord(tokens[phraseStart].Text()) {
		phraseStart++
	}
	if phraseStart >= len(tokens) {
		return ""
	}
	startOffset := tokens[phraseStart].Offset()
	var sb strings.Builder
	writing := false
	for _, tok := range t.node.AllTokens() {
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

// EventCol returns the 1-based column of the event token using li.
func (t TriggerDecl) EventCol(li green.LineIndex) int {
	switch t.Kind() {
	case "event":
		tokens := t.node.Tokens()
		if len(tokens) > 0 {
			_, col := li.LineCol16(tokens[0].Offset())
			return col
		}
	case "domain_listen":
		elems := significantElements(t.node)
		if len(elems) >= 3 {
			_, col := li.LineCol16(refAwareOffset(elems[2]))
			return col
		}
	}
	return 0
}

// EventIsString returns true when the event token is a quoted string literal.
func (t TriggerDecl) EventIsString() bool {
	switch t.Kind() {
	case "event":
		tokens := t.node.Tokens()
		return len(tokens) > 0 && tokens[0].Kind() == SyntaxKindString
	case "domain_listen":
		elems := significantElements(t.node)
		return len(elems) >= 3 && elems[2].Kind() == SyntaxKindString
	}
	return false
}

// ActionDecl is a typed view over a SyntaxKindAction node.
type ActionDecl struct{ node SyntaxNode }

// subjectElement returns the action's subject slot: significant element 0,
// which is a plain Ident token for a bare subject or a SyntaxKindRef node for
// a qualified one (Task 6b, e.g. `re/billing asks ...`). Returns nil when the
// action node has no subject, e.g. an error-recovery node.
//
// This must not be read with ChildToken(SyntaxKindIdent): once the subject is
// wrapped in a Ref it is no longer a direct Ident child, and for an
// internal_action that call would return the verb instead.
func (a ActionDecl) subjectElement() SyntaxElement {
	return nameSlotElement(significantElements(a.node))
}

// Subject returns the first token of the subject (domain/service name). For a
// qualified subject that is the ref's leading segment, which is where the
// whole reference starts.
func (a ActionDecl) Subject() *SyntaxToken {
	el := a.subjectElement()
	if el == nil {
		return nil
	}
	return slotFirstToken(el)
}

// Verb returns the action verb token (asks/notifies/listens/returns).
func (a ActionDecl) Verb() *SyntaxToken {
	return a.node.ChildToken(
		SyntaxKindKwAsks, SyntaxKindKwNotifies,
		SyntaxKindKwListens, SyntaxKindKwReturns,
	)
}

// Connector returns the connector token between verb and phrase, or nil.
// For sync_action: the token after subject, asks and target.
// For internal_action: the token after subject and verb.
// For return_action: the KwTo token (different positional semantics — `to <target>`).
//
// Both the subject (Task 6b) and the sync_action target (Task 4) may be
// multi-token Refs, so the connector is located from element positions via
// slotEndIndex rather than a fixed token index.
func (a ActionDecl) Connector() *SyntaxToken {
	tokens := a.node.Tokens()
	switch a.Kind() {
	case "sync_action", "internal_action":
		return connectorAt(tokens, slotEndIndex(a))
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

// SubjectName returns the action subject (the "from" party). The subject may
// be a parseRef-wrapped qualified ref (Task 6b), so refAwareText returns the
// full "re/billing" rather than truncating to its first segment.
func (a ActionDecl) SubjectName() string {
	el := a.subjectElement()
	if el == nil {
		return ""
	}
	return refAwareText(el)
}

// SubjectCol returns the 1-based column where the subject starts, using li.
func (a ActionDecl) SubjectCol(li green.LineIndex) int {
	el := a.subjectElement()
	if el == nil {
		return 0
	}
	_, col := li.LineCol16(refAwareOffset(el))
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

// targetElement returns the action's target slot, or nil when it has none.
// Both slots may be a parseRef-wrapped ref: the sync_action target since
// Task 4 (e.g. bc:re/billing), the return_action target since Task 6b.
//
// The return_action target is the element right after `to`, which is always
// element 2 because parseReturnsAction only emits SyntaxKindKwTo there;
// a later `to` inside the phrase is an ordinary ident.
func (a ActionDecl) targetElement() SyntaxElement {
	elems := significantElements(a.node)
	i := -1
	switch a.Kind() {
	case "sync_action":
		i = 2
	case "return_action":
		if len(elems) > 2 && elems[2].Kind() == SyntaxKindKwTo {
			i = 3
		}
	}
	if i < 0 || i >= len(elems) {
		return nil
	}
	if k := elems[i].Kind(); k != SyntaxKindIdent && k != SyntaxKindRef {
		return nil
	}
	return elems[i]
}

// TargetName returns the target context for sync_action and return_action,
// reading a multi-token ref in full via refAwareText rather than truncating
// to a single leaf token.
func (a ActionDecl) TargetName() string {
	el := a.targetElement()
	if el == nil {
		return ""
	}
	return refAwareText(el)
}

// TargetCol returns the 1-based column where the target starts, using li.
func (a ActionDecl) TargetCol(li green.LineIndex) int {
	el := a.targetElement()
	if el == nil {
		return 0
	}
	_, col := li.LineCol16(refAwareOffset(el))
	return col
}

// EventValue returns the event name for async_action (notifies). The event
// slot may be a quoted string or a parseRef-wrapped ref (Task 4); refAwareText
// returns the full ref text for a kind-prefixed slug rather than truncating.
func (a ActionDecl) EventValue() string {
	if a.Kind() != "async_action" {
		return ""
	}
	elems := significantElements(a.node)
	if len(elems) >= 3 {
		return refAwareText(elems[2])
	}
	return ""
}

// EventCol returns the 1-based column of the event token for async_action using li.
func (a ActionDecl) EventCol(li green.LineIndex) int {
	if a.Kind() != "async_action" {
		return 0
	}
	elems := significantElements(a.node)
	if len(elems) >= 3 {
		_, col := li.LineCol16(refAwareOffset(elems[2]))
		return col
	}
	return 0
}

// EventIsString returns true when the event was a quoted string literal.
func (a ActionDecl) EventIsString() bool {
	if a.Kind() != "async_action" {
		return false
	}
	elems := significantElements(a.node)
	return len(elems) >= 3 && elems[2].Kind() == SyntaxKindString
}

// VerbToken returns the action's verb token: the asks/notifies/returns keyword,
// or for an internal_action the plain ident verb that follows the subject.
// Returns nil when the action has no verb at all (a bare subject line).
//
// The internal_action verb sits at element 1, not token 1: a qualified subject
// (Task 6b) spans several tokens.
func (a ActionDecl) VerbToken() *SyntaxToken {
	if tok := a.Verb(); tok != nil {
		return tok
	}
	elems := significantElements(a.node)
	if len(elems) < 2 {
		return nil
	}
	tokens := a.node.Tokens()
	i := tokenIndexAt(elems, 1)
	if i >= len(tokens) {
		return nil
	}
	return &tokens[i]
}

// VerbValue returns the verb text.
func (a ActionDecl) VerbValue() string {
	tok := a.VerbToken()
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// ConnectorValue returns the connector word text (e.g. "to", "for"), or empty.
func (a ActionDecl) ConnectorValue() string {
	tok := a.Connector()
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// slotEndIndex returns the index into a.Tokens() just past the action's
// structural slots: subject and verb for every kind, plus the target for a
// sync_action and the `to <target>` pair for a return_action. That is where an
// optional connector word sits, and where the phrase begins when there is none.
//
// Every slot is counted as one element and converted to a token index via
// tokenIndexAt, because none of them has a fixed token width: the subject
// (Task 6b) and both targets (Tasks 4 and 6b) may be multi-token Refs.
func slotEndIndex(a ActionDecl) int {
	elems := significantElements(a.node)
	switch a.Kind() {
	case "sync_action":
		return tokenIndexAt(elems, 3) // subject, asks, target
	case "return_action":
		if len(elems) > 2 && elems[2].Kind() == SyntaxKindKwTo {
			return tokenIndexAt(elems, 4) // subject, returns, to, target
		}
		return tokenIndexAt(elems, 2) // subject, returns
	default: // internal_action, async_action
		return tokenIndexAt(elems, 2) // subject, verb
	}
}

// phraseStartIndex returns the index into tokens (a.node.Tokens(), i.e.
// non-trivia tokens) at which the free-text phrase begins for this action's
// kind, skipping subject/verb/target/connector tokens. Returns len(tokens)
// for action kinds that have no phrase (async_action).
func phraseStartIndex(a ActionDecl, tokens []SyntaxToken) int {
	if a.Kind() == "async_action" {
		return len(tokens) // no phrase for notifies
	}
	start := slotEndIndex(a)
	if connectorAt(tokens, start) != nil {
		start++ // skip connector
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
	return a.rawTextFrom(phraseStartIndex(a, a.node.Tokens()))
}

// rawTextFrom returns the exact raw source substring of this action from
// token index start up to (but excluding) any trailing operation annotation,
// trimmed. Inter-token whitespace is preserved, so tight punctuation such as
// `(1! & 2!)` keeps its original spacing instead of being space-joined.
//
// PhraseText starts at the phrase; SourceText starts earlier, at the connector
// or the verb, so it can rebuild the line without re-deriving those words.
func (a ActionDecl) rawTextFrom(start int) string {
	tokens := a.node.Tokens()
	if start < 0 || start >= len(tokens) {
		return ""
	}
	startOffset := tokens[start].Offset()
	// An operation annotation is a child of the action node, so its tokens are in
	// AllTokens() and would otherwise be appended to the phrase. Stop before it.
	var endOffset green.TextSize = -1
	if op := a.OpAnnotation(); op != nil {
		if opToks := op.AllTokens(); len(opToks) > 0 {
			endOffset = opToks[0].Offset()
		}
	}
	var sb strings.Builder
	writing := false
	for _, tok := range a.node.AllTokens() {
		if endOffset >= 0 && tok.Offset() >= endOffset {
			break
		}
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

// SourceText renders the action as canonical .craft source: the exact line the
// LSP formatter writes back into the document.
//
// This is deliberately NOT Description(). The two have opposed contracts and
// one method cannot satisfy both:
//
//   - Description() is a display label. Its shape is mirrored by
//     projection.go into model.Action.Description, which the visualizers render
//     as an edge label, and Task 6 decided a label must NOT leak the `[...]`
//     operation annotation (see TestActionDecl_Description_ExcludesAnnotation).
//     It also always quotes the async event and reflows `returns to <target>`
//     into `returns <phrase> to <target>`, both of which read better as prose.
//   - SourceText() must reparse to the same action. It therefore KEEPS the
//     annotation, quotes an event only when the source quoted it, and preserves
//     the `returns to <target> <phrase>` word order.
//
// The formatter used to call Description(), which is how Format Document came
// to silently delete `[POST /v1/charges]` annotations, downgrade typed event
// refs to the deprecated quoted-string form, and move a returns target into the
// phrase. Anything rendering source belongs here; anything rendering a label
// belongs in Description.
func (a ActionDecl) SourceText() string {
	var sb strings.Builder
	sb.WriteString(a.SubjectName())
	switch a.Kind() {
	case "async_action":
		sb.WriteString(" notifies ")
		sb.WriteString(a.eventSourceText())
	case "sync_action":
		sb.WriteString(" asks ")
		sb.WriteString(a.TargetName())
		appendSpaced(&sb, a.rawTextFrom(slotEndIndex(a)))
	case "return_action":
		sb.WriteString(" returns")
		if target := a.TargetName(); target != "" {
			sb.WriteString(" to ")
			sb.WriteString(target)
		}
		appendSpaced(&sb, a.rawTextFrom(slotEndIndex(a)))
	default: // internal_action
		// The verb sits one token before the slot end, so reading raw from
		// there carries verb, connector and phrase in their original words.
		// With no verb at all there is nothing after the subject to emit.
		if a.VerbValue() != "" {
			appendSpaced(&sb, a.rawTextFrom(slotEndIndex(a)-1))
		}
	}
	if op := a.OpText(); op != "" {
		sb.WriteString(" [")
		sb.WriteString(op)
		sb.WriteString("]")
	}
	return sb.String()
}

// eventSourceText returns the async_action event exactly as it was written: the
// raw quoted literal for the legacy string form, so escapes and non-ASCII
// survive verbatim rather than being re-escaped by a %q round trip, or the
// full ref text otherwise. EventValue() unquotes, which is right for a label
// and wrong for source.
func (a ActionDecl) eventSourceText() string {
	elems := significantElements(a.node)
	if len(elems) < 3 {
		return ""
	}
	if tok, ok := elems[2].(SyntaxToken); ok {
		return tok.Text()
	}
	return refAwareText(elems[2])
}

// appendSpaced appends s to sb preceded by a single space, or does nothing when
// s is empty, so callers never emit a trailing or doubled space.
func appendSpaced(sb *strings.Builder, s string) {
	if s == "" {
		return
	}
	sb.WriteByte(' ')
	sb.WriteString(s)
}

// Tokens returns the raw token list for the action node.
func (a ActionDecl) Tokens() []SyntaxToken { return a.node.Tokens() }

// OpAnnotation returns the action's trailing operation annotation node, or nil
// when the action has none.
func (a ActionDecl) OpAnnotation() *SyntaxNode {
	return a.node.ChildNode(SyntaxKindOpAnnotation)
}

// OpText returns the annotation body verbatim, e.g. "POST /v1/charges", or ""
// when the action has no annotation.
func (a ActionDecl) OpText() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	var sb strings.Builder
	for _, tok := range n.AllTokens() {
		if tok.Kind() == SyntaxKindLBracket || tok.Kind() == SyntaxKindRBracket {
			continue
		}
		sb.WriteString(tok.Text())
	}
	return strings.TrimSpace(sb.String())
}

// OpVerb returns the recognised protocol verb of the action's operation
// annotation (e.g. "POST", "GRPC"), or "" when the annotation has no recognised
// verb or the action has no annotation.
func (a ActionDecl) OpVerb() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	if tok := n.ChildToken(SyntaxKindOpVerb); tok != nil {
		return tok.Text()
	}
	return ""
}

// OpPayload returns the annotation body with any recognised protocol verb
// stripped, e.g. "/v1/charges" for `[POST /v1/charges]` and the whole body for
// `[op1/op2/op3]`. Returns "" when the action has no annotation.
func (a ActionDecl) OpPayload() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	verb := a.OpVerb()
	if verb == "" {
		return a.OpText()
	}
	return strings.TrimSpace(strings.TrimPrefix(a.OpText(), verb))
}

// PhraseStartIndex returns the index into a.Tokens() at which the free-text
// phrase begins, i.e. the same index PhraseText() starts reading from. It
// accounts for multi-token (Ref) subjects and targets (e.g. "re/billing",
// "bc:re/billing") by skipping their full span rather than assuming a fixed
// token width. Exported minimally so internal/lsp's classifyActionIdents
// (semantic-token phrase highlighting) can start at the same token as
// PhraseText(), instead of duplicating/hardcoding this offset logic and
// drifting out of sync with it.
func (a ActionDecl) PhraseStartIndex() int {
	return phraseStartIndex(a, a.node.Tokens())
}

// SlotEndIndex returns the index into a.Tokens() just past the action's
// structural slots (subject, verb, and any target), i.e. where an optional
// connector word may sit. PhraseStartIndex is this index, plus one when a
// connector is actually present. Exported for the same reason as
// PhraseStartIndex: internal/lsp classifies the connector word itself for
// internal_action and return_action, and must locate it from the same
// element-based arithmetic rather than a fixed token index.
func (a ActionDecl) SlotEndIndex() int { return slotEndIndex(a) }

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
			names = append(names, stringAwareText(tok))
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

// ContextMapDecl is a typed view over a SyntaxKindContextMapDecl node — the
// context_map { } block of authored typed edges between node slugs (Task 5).
type ContextMapDecl struct{ node SyntaxNode }

// Keyword returns the 'context_map' keyword token.
func (c ContextMapDecl) Keyword() *SyntaxToken {
	return c.node.ChildToken(SyntaxKindKwContextMap)
}

// Domain returns the optional domain-scope identifier (e.g. "re" in
// `context_map re { ... }`), or "" if the block is unscoped (Task 3).
func (c ContextMapDecl) Domain() string {
	tok := c.node.ChildToken(SyntaxKindContextMapDomain)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// Line returns the 1-based source line of the `context_map` keyword using li.
func (c ContextMapDecl) Line(li green.LineIndex) int { return nodeFirstTokenLine(c.node, li) }

// EndLine returns the 1-based line of the closing `}` using li.
func (c ContextMapDecl) EndLine(li green.LineIndex) int { return nodeEndLine(c.node, li) }

// Edges returns all EdgeDecl views within this context_map block.
func (c ContextMapDecl) Edges() []EdgeDecl {
	var result []EdgeDecl
	for _, n := range c.node.ChildNodes(SyntaxKindEdgeStmt) {
		result = append(result, EdgeDecl{node: n})
	}
	return result
}

// EdgeDecl is a typed view over a SyntaxKindEdgeStmt node — a single typed
// edge `ref EDGE_KW ref` inside a context_map block (Task 5).
type EdgeDecl struct{ node SyntaxNode }

// Left returns the raw source text of the left-hand endpoint reference (via
// RefText — NEVER Name(), which would truncate a kind-prefixed slug like
// "bc:re/subscriptions" down to just "bc"). Empty if malformed.
func (e EdgeDecl) Left() string {
	refs := e.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 1 {
		return ""
	}
	return RefDecl{node: refs[0]}.RefText()
}

// Right returns the raw source text of the right-hand endpoint reference
// (via RefText — see Left). Empty if malformed.
func (e EdgeDecl) Right() string {
	refs := e.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 2 {
		return ""
	}
	return RefDecl{node: refs[1]}.RefText()
}

// Verb returns the edge keyword token text (e.g. "customer_supplier"), or "" if malformed.
func (e EdgeDecl) Verb() string {
	tok := e.node.ChildToken(SyntaxKindEdgeKw)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// LeftRef returns the RefDecl view of the left-hand endpoint (for position
// info via RefDecl.Line/Col), or nil if malformed. See Left() for the text
// accessor (Task 7).
func (e EdgeDecl) LeftRef() *RefDecl {
	refs := e.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 1 {
		return nil
	}
	return &RefDecl{node: refs[0]}
}

// RightRef returns the RefDecl view of the right-hand endpoint (for position
// info via RefDecl.Line/Col), or nil if malformed. See Right() for the text
// accessor (Task 7).
func (e EdgeDecl) RightRef() *RefDecl {
	refs := e.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 2 {
		return nil
	}
	return &RefDecl{node: refs[1]}
}

// GlossaryDecl is a typed view over a SyntaxKindGlossaryDecl node — the
// glossary { } block of cross-context term relations. Mirrors ContextMapDecl.
type GlossaryDecl struct{ node SyntaxNode }

// Keyword returns the 'glossary' keyword token.
func (g GlossaryDecl) Keyword() *SyntaxToken {
	return g.node.ChildToken(SyntaxKindKwGlossary)
}

// Domain returns the optional domain-scope identifier (e.g. "re" in
// `glossary re { ... }`), or "" if the block is unscoped.
func (g GlossaryDecl) Domain() string {
	tok := g.node.ChildToken(SyntaxKindGlossaryDomain)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// Line returns the 1-based source line of the `glossary` keyword using li.
func (g GlossaryDecl) Line(li green.LineIndex) int { return nodeFirstTokenLine(g.node, li) }

// EndLine returns the 1-based line of the closing `}` using li.
func (g GlossaryDecl) EndLine(li green.LineIndex) int { return nodeEndLine(g.node, li) }

// Relations returns all GlossaryRelationDecl views within this glossary block.
func (g GlossaryDecl) Relations() []GlossaryRelationDecl {
	var result []GlossaryRelationDecl
	for _, n := range g.node.ChildNodes(SyntaxKindGlossaryRelation) {
		result = append(result, GlossaryRelationDecl{node: n})
	}
	return result
}

// GlossaryRelationDecl is a typed view over a SyntaxKindGlossaryRelation
// node — a single term relation `ref GLOSSARY_VERB ref` inside a glossary
// block. Mirrors EdgeDecl.
type GlossaryRelationDecl struct{ node SyntaxNode }

// Left returns the raw source text of the left-hand term reference (via
// RefText — NEVER Name(), which would truncate a multi-segment slug like
// "billing/Invoice" down to just "billing"). Empty if malformed.
func (g GlossaryRelationDecl) Left() string {
	refs := g.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 1 {
		return ""
	}
	return RefDecl{node: refs[0]}.RefText()
}

// Right returns the raw source text of the right-hand term reference (via
// RefText — see Left). Empty if malformed.
func (g GlossaryRelationDecl) Right() string {
	refs := g.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 2 {
		return ""
	}
	return RefDecl{node: refs[1]}.RefText()
}

// Verb returns the relation keyword token text (e.g. "same_as"), or "" if malformed.
func (g GlossaryRelationDecl) Verb() string {
	tok := g.node.ChildToken(SyntaxKindEdgeKw)
	if tok == nil {
		return ""
	}
	return tok.Text()
}

// LeftRef returns the RefDecl view of the left-hand term reference (for
// position info via RefDecl.Line/Col), or nil if malformed. See Left() for
// the text accessor.
func (g GlossaryRelationDecl) LeftRef() *RefDecl {
	refs := g.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 1 {
		return nil
	}
	return &RefDecl{node: refs[0]}
}

// RightRef returns the RefDecl view of the right-hand term reference (for
// position info via RefDecl.Line/Col), or nil if malformed. See Right() for
// the text accessor.
func (g GlossaryRelationDecl) RightRef() *RefDecl {
	refs := g.node.ChildNodes(SyntaxKindRef)
	if len(refs) < 2 {
		return nil
	}
	return &RefDecl{node: refs[1]}
}
