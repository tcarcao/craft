package green

// SyntaxKind is the type for all syntax node and token kinds.
// Token kinds are in (0, 1000). Node kinds are >= 1000.
// The canonical constants are defined in internal/syntax/kind.go.
// internal/green stores and forwards kind values but never inspects them.
type SyntaxKind int
