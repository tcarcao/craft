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
	SyntaxKindKwGlossary   // hard keyword `glossary`

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

	// SyntaxKindKwCatalogRef/SyntaxKindKwRepo are contextual keywords for the
	// service anchor properties `catalog_ref:` / `repo:` (Task 6).
	SyntaxKindKwCatalogRef
	SyntaxKindKwRepo

	// New contextual keywords for triggers and imports
	SyntaxKindKwCron   // contextual keyword `cron` (in when clause)
	SyntaxKindKwEvery  // contextual keyword `every` (in when clause)
	SyntaxKindKwImport // hard keyword `import`

	// SyntaxKindEdgeKw is the contextual keyword for a relationship verb
	// inside a context_map or glossary block: either one of the 8 DDD
	// strategic context-mapping patterns (customer_supplier/conformist/
	// anticorruption_layer/open_host_service/published_language/partnership/
	// shared_kernel/separate_ways) or one of the 3 glossary term-relation
	// verbs (same_as/contrasts/distinct_from). Matched by value from
	// TokenIdent, like asks/notifies elsewhere. Shared between the two block
	// kinds since both wrap a single "ref VERB ref" relation statement and
	// disambiguation is unnecessary — a lookup always scopes to the
	// enclosing EdgeStmt/GlossaryRelation node's own children.
	SyntaxKindEdgeKw

	// SyntaxKindContextMapDomain is the optional domain-scope identifier
	// directly after the `context_map` keyword and before its `{`, e.g. the
	// `re` in `context_map re { ... }` (Task 3). Consumed positionally from
	// TokenIdent — not matched by value, unlike the contextual keywords
	// above.
	SyntaxKindContextMapDomain

	// SyntaxKindGlossaryDomain is the optional domain-scope identifier
	// directly after the `glossary` keyword and before its `{`, e.g. the `re`
	// in `glossary re { ... }`. Consumed positionally from TokenIdent — not
	// matched by value — mirroring SyntaxKindContextMapDomain.
	SyntaxKindGlossaryDomain

	// SyntaxKindKwTags is the contextual keyword `tags` introducing a use_case's
	// tags { } sub-block (Task 3, Slice B). Matched by value from TokenIdent,
	// like when/asks/notifies elsewhere — not a reserved word.
	SyntaxKindKwTags

	// SyntaxKindOpVerb is the recognised uppercase protocol verb at the head of
	// an operation annotation body, e.g. the POST in `[POST /v1/charges]`.
	// Matched by value from TokenIdent, like asks/notifies elsewhere. An
	// unrecognised leading word is emitted as SyntaxKindIdent and the whole
	// body is treated as an opaque payload.
	SyntaxKindOpVerb

	// SyntaxKindKwScenario is the contextual keyword `scenario`, recognised only
	// at the start of a use_case body line when the next token is a string.
	SyntaxKindKwScenario

	// SyntaxKindKwAlt/KwOpt/KwElse are the contextual keywords that head a
	// branch block inside a scenario: recognised only at the start of an
	// action line when followed by a string or nothing else on the line.
	SyntaxKindKwAlt
	SyntaxKindKwOpt
	SyntaxKindKwElse

	// Journey-semantics contextual keywords (v2.23.0). Each is matched by value
	// from TokenIdent only in its slot (spec §3.1); elsewhere the word stays an
	// identifier.
	SyntaxKindKwPar      // `par` block header
	SyntaxKindKwAnd      // `and` branch of a par
	SyntaxKindKwLoop     // `loop` block header
	SyntaxKindKwNote     // `note "…"`
	SyntaxKindKwFinding  // `finding "…"`
	SyntaxKindKwReceives // `when <ctx> receives …`
	SyntaxKindKwSurface  // top-level `surface <name> {`
	SyntaxKindKwCallers  // `callers:` in a surface
	SyntaxKindKwSpec     // `spec:` in a surface
	SyntaxKindKwQuery    // `query` surface line
	SyntaxKindKwCommand  // `command` surface line

	// Journey-semantics-2 contextual keywords (v2.24.0, spec §3.1).
	SyntaxKindKwOne      // `one` of `notifies one of`
	SyntaxKindKwOf       // `of` of `notifies one of`
	SyntaxKindKwStream   // `stream` directly after `returns`
	SyntaxKindKwInferred // `inferred` at the head of an operation bracket

	// Journey-semantics-2 contextual keywords (v2.25.0, spec §12.1).
	SyntaxKindKwIncludes  // includes "<title>"
	SyntaxKindKwContinues // continues "<title>"

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

	SyntaxKindGlossaryDecl     // glossary { relation_stmt* }
	SyntaxKindGlossaryRelation // ref GLOSSARY_VERB ref, inside a glossary block

	SyntaxKindTagsBlock // tags { tag_stmt* } inside a use_case
	SyntaxKindTagStmt   // IDENT ':' (IDENT | STRING | ref), inside a tags block

	// SyntaxKindOpAnnotation wraps a trailing `[ ... ]` operation annotation on
	// an action line, e.g. `[POST /v1/charges]`. It is always the last child of
	// a SyntaxKindAction node.
	SyntaxKindOpAnnotation

	// SyntaxKindScenarioName wraps `scenario "<label>"` at the head of a scenario.
	SyntaxKindScenarioName

	// SyntaxKindBlock is an alt or opt block step: one or more Branch children.
	SyntaxKindBlock
	// SyntaxKindBranch is one branch of a block: a BranchHeader followed by
	// its steps (Action and nested Block nodes).
	SyntaxKindBranch
	// SyntaxKindBranchHeader wraps `alt|opt|else "<label>"`.
	SyntaxKindBranchHeader

	// SyntaxKindNote is a `note "…"` / `finding "…"` statement: a step, a
	// use-case-level line, or a surface line.
	SyntaxKindNote
	// SyntaxKindSurfaceDecl is `surface <name> { … }`.
	SyntaxKindSurfaceDecl
	// SyntaxKindSurfaceField wraps `callers: …` or `spec: "…"`.
	SyntaxKindSurfaceField
	// SyntaxKindSurfaceOperation wraps a `query|command <phrase> [op]` line.
	SyntaxKindSurfaceOperation

	// SyntaxKindEventList wraps the items and commas of a `listens` or
	// `notifies one of` list. It exists only when the slot holds a comma.
	SyntaxKindEventList

	// SyntaxKindLink is an includes/continues step.
	SyntaxKindLink
)
