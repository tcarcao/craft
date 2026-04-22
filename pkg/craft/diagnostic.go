// Package craft provides the public API for the Craft DSL toolchain.
// Experimental: stabilizes at v0.1.
package craft

// Diagnostic is a structured error/warning emitted by the v2 parser or sema
// layer. It follows Q19: assertions are on Code + Range + Severity; Message
// is free-form prose.
type Diagnostic struct {
	// Code is a stable, machine-readable identifier (e.g. craft/syntax/unexpected-token).
	Code string `json:"code"`
	// Message is a human-readable description. Not asserted in golden files.
	Message string `json:"message"`
	// Severity is the level of the diagnostic.
	Severity Severity `json:"severity"`
	// Range is the source location the diagnostic points to.
	Range Range `json:"range"`
	// SourceURI identifies the file this diagnostic belongs to when a diagnostic
	// is produced by a cross-file analysis pass (e.g. AnalyzeWorkspace). Empty
	// for diagnostics already scoped to a single file.
	SourceURI string `json:"sourceUri,omitempty"`
}

// Severity classifies the urgency of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityHint    Severity = "hint"
)

// Range is a half-open interval [Start, End) in a text document.
// Positions are 0-based (LSP convention).
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position is a cursor location in a text document.
// Line and Character are 0-based (LSP convention).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
