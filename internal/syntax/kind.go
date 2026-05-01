package syntax

import "github.com/tcarcao/craft/internal/green"

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
	SyntaxKindArchSection   // presentation or gateway section
	SyntaxKindArchComponent
	SyntaxKindArchModifier
	SyntaxKindExposureDecl
	SyntaxKindDeploymentRule
	SyntaxKindErrorNode // wraps tokens that could not form a valid construct
)
