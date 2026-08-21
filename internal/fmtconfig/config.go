// Package fmtconfig resolves the formatter's configuration.
//
// The formatter's whitespace policy used to be entirely hardcoded. This package
// exists so a workspace can express indent width and alignment scope without
// each call site growing its own knob, and so the CLI and the language server
// answer identically for the same file.
package fmtconfig

import "fmt"

// Scope decides what ends an alignment run. The values are ordered from least
// to most alignment: off < strict < block < file < decl. decl sits at the far
// end, not next to block: a declaration boundary is rarer than a blank line,
// so a run that survives blank lines aligns MORE than one that does not. See
// docs/decisions/formatting-configuration.md D2.
type Scope string

const (
	// ScopeOff never aligns: one space before the cell.
	ScopeOff Scope = "off"
	// ScopeStrict ends a run at any line that does not carry the cell. This is
	// what gofmt and hclwrite do.
	ScopeStrict Scope = "strict"
	// ScopeBlock ends a run at a blank line, a `{`, a `}`, or a comment-only
	// line. Lines that merely lack the cell pass through.
	ScopeBlock Scope = "block"
	// ScopeFile ends a run only at a blank line; unlike ScopeBlock, a brace or
	// a comment-only line does not.
	ScopeFile Scope = "file"
	// ScopeDecl never ends a run. The aligner already runs once per top-level
	// declaration, so "ends at a declaration boundary" and "never ends" are
	// the same rule from inside a single call: this is the value that gives
	// one shared column across an entire declaration, blank lines included.
	ScopeDecl Scope = "decl"
)

// AlignConfig is the alignment half of the configuration. The two cell types
// carry separate scopes because they are independent decisions, even though
// both currently default to ScopeBlock; see
// docs/decisions/formatting-configuration.md D2 for the churn-blast-radius
// measurements behind that default.
type AlignConfig struct {
	TrailingComment Scope   `toml:"trailing_comment"`
	OpAnnotation    Scope   `toml:"op_annotation"`
	OutlierRatio    float64 `toml:"outlier_ratio"`
	OutlierMin      int     `toml:"outlier_min"`
}

// Config is the whole formatter configuration.
type Config struct {
	Indent             int         `toml:"indent"`
	ContinuationIndent int         `toml:"continuation_indent"`
	Align              AlignConfig `toml:"align"`
}

// Defaults returns the built-in configuration, which is what a workspace with
// no .craftfmt gets.
func Defaults() Config {
	return Config{
		Indent:             4,
		ContinuationIndent: 4,
		Align: AlignConfig{
			TrailingComment: ScopeBlock,
			OpAnnotation:    ScopeBlock,
			OutlierRatio:    2.5,
			OutlierMin:      40,
		},
	}
}

// maxIndent bounds indent width. The formatter multiplies it by nesting depth,
// and strings.Repeat on an unbounded value read from a file on disk is how a
// config file turns into an out-of-memory.
const maxIndent = 16

// Validate reports the first problem with the configuration. A bad .craftfmt
// must produce a diagnostic rather than silently formatting to something
// surprising, so every caller checks this.
func (c Config) Validate() error {
	if err := validBounded("indent", c.Indent, maxIndent); err != nil {
		return err
	}
	if err := validBounded("continuation_indent", c.ContinuationIndent, maxIndent); err != nil {
		return err
	}
	if err := validScope("trailing_comment", c.Align.TrailingComment); err != nil {
		return err
	}
	if err := validScope("op_annotation", c.Align.OpAnnotation); err != nil {
		return err
	}
	if c.Align.OutlierRatio < 0 {
		return fmt.Errorf("outlier_ratio must not be negative, got %v", c.Align.OutlierRatio)
	}
	if c.Align.OutlierMin < 0 {
		return fmt.Errorf("outlier_min must not be negative, got %d", c.Align.OutlierMin)
	}
	return nil
}

func validScope(key string, s Scope) error {
	switch s {
	case ScopeOff, ScopeStrict, ScopeBlock, ScopeFile, ScopeDecl:
		return nil
	}
	return fmt.Errorf("%s must be one of off, strict, block, file, decl; got %q", key, s)
}

// validBounded checks that v is within [0, max], for the two integer fields
// (Indent, ContinuationIndent) that share that bound. See maxIndent's doc
// comment for why the bound exists.
func validBounded(key string, v, max int) error {
	if v < 0 || v > max {
		return fmt.Errorf("%s must be between 0 and %d, got %d", key, max, v)
	}
	return nil
}
