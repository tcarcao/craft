package syntax

import "github.com/tcarcao/craft/v2/internal/green"

// SyntaxKind identifies the type of a syntax node or token.
// Token kinds are in (0, 1000). Node kinds are >= 1000.
// SyntaxKindInvalid (0) is the zero value and is neither.
type SyntaxKind = green.SyntaxKind

const (
	SyntaxKindInvalid SyntaxKind = iota // 0 — zero value, never used

	// --- Structural ---
	SyntaxKindEOF
	SyntaxKindError        // unrecognised character (from lexer.TokenError)
	SyntaxKindLineComment  // // ...
	SyntaxKindBlockComment // /* ... */
	SyntaxKindDocComment   // /// ...
	SyntaxKindWhitespace   // spaces, tabs, newlines between tokens
	SyntaxKindIdent        // unresolved name — never a contextual keyword
	SyntaxKindString
	SyntaxKindNumber
	SyntaxKindPercentage

	// --- Punctuation ---
	SyntaxKindLBrace
	SyntaxKindRBrace
	SyntaxKindLParen
	SyntaxKindRParen
	SyntaxKindLBracket
	SyntaxKindRBracket
	SyntaxKindColon
	SyntaxKindComma
	SyntaxKindGT    // >
	SyntaxKindArrow // ->

	// --- Hard keywords (lexer-resolved) ---
	SyntaxKindKwActor
	SyntaxKindKwActors
	SyntaxKindKwUser
	SyntaxKindKwSystem
	SyntaxKindKwService
	SyntaxKindKwDomain
	SyntaxKindKwDomains
	SyntaxKindKwServices
	SyntaxKindKwUseCase
	SyntaxKindKwArch
	SyntaxKindKwExposure
	SyntaxKindKwContextMap // hard keyword `context_map`

	// --- Contextual keywords (parser-resolved from TokenIdent) ---
	SyntaxKindKwWhen
	SyntaxKindKwAsks
	SyntaxKindKwNotifies
	SyntaxKindKwListens
	SyntaxKindKwReturns
	SyntaxKindKwTo
	SyntaxKindKwThrough
	SyntaxKindKwContexts
	SyntaxKindKwLanguage
	SyntaxKindKwDataStores
	SyntaxKindKwDeployment
	SyntaxKindKwPresentation
	SyntaxKindKwGateway
	SyntaxKindKwTrue  // future: boolean values in modifier properties
	SyntaxKindKwFalse // future: boolean values in modifier properties

	// SyntaxKindKwOpsLevel/SyntaxKindKwRepo are contextual keywords for the
	// service anchor properties `opslevel:` / `repo:` (Task 6).
	SyntaxKindKwOpsLevel
	SyntaxKindKwRepo

	// New contextual keywords for triggers and imports
	SyntaxKindKwCron   // contextual keyword `cron` (in when clause)
	SyntaxKindKwEvery  // contextual keyword `every` (in when clause)
	SyntaxKindKwImport // hard keyword `import`

	// SyntaxKindEdgeKw is the contextual keyword for a context_map edge verb:
	// one of the 8 DDD strategic context-mapping patterns (customer_supplier/
	// conformist/anticorruption_layer/open_host_service/published_language/
	// partnership/shared_kernel/separate_ways). Matched by value from
	// TokenIdent, like asks/notifies elsewhere.
	SyntaxKindEdgeKw

	// SyntaxKindContextMapDomain is the optional domain-scope identifier
	// directly after the `context_map` keyword and before its `{`, e.g. the
	// `re` in `context_map re { ... }` (Task 3). Consumed positionally from
	// TokenIdent — not matched by value, unlike the contextual keywords
	// above.
	SyntaxKindContextMapDomain

	// SyntaxKindKwTags is the contextual keyword `tags` introducing a use_case's
	// tags { } sub-block (Task 3, Slice B). Matched by value from TokenIdent,
	// like when/asks/notifies elsewhere — not a reserved word.
	SyntaxKindKwTags

	// syntaxKindTokenSentinel marks the end of token kinds.
	// It must remain < 1000 to preserve the node/token boundary invariant.
	syntaxKindTokenSentinel
)

const (
	// Node kinds — all >= 1000
	SyntaxKindFile SyntaxKind = iota + 1000

	SyntaxKindActorDecl
	SyntaxKindActorsBlock
	SyntaxKindDomainDecl
	SyntaxKindDomainsBlock
	SyntaxKindBoundedContext
	SyntaxKindServiceDecl
	SyntaxKindServicesBlock
	SyntaxKindUseCaseDecl
	SyntaxKindScenario
	SyntaxKindTrigger
	SyntaxKindAction
	SyntaxKindArchDecl
	SyntaxKindArchSection // presentation or gateway section
	SyntaxKindArchComponent
	SyntaxKindArchModifier
	SyntaxKindExposureDecl
	SyntaxKindDeploymentRule
	SyntaxKindErrorNode    // wraps tokens that could not form a valid construct
	SyntaxKindRef          // wraps a name ident at a reference site (e.g. contexts: field values)
	SyntaxKindServiceField // wraps one field declaration (keyword + colon + values) in a service body
	SyntaxKindImportDecl   // import "path/to/file.craft"

	SyntaxKindContextMapDecl // context_map { edge_stmt* }
	SyntaxKindEdgeStmt       // ref EDGE_KW ref, inside a context_map block

	SyntaxKindTagsBlock // tags { tag_stmt* } inside a use_case
	SyntaxKindTagStmt   // IDENT ':' (IDENT | STRING | ref), inside a tags block
)
