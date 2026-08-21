// Package fmtconfig resolves the formatter's configuration.
//
// The formatter's whitespace policy used to be entirely hardcoded. This package
// exists so a workspace can express indent width and alignment scope without
// each call site growing its own knob, and so the CLI and the language server
// answer identically for the same file.
package fmtconfig

import "fmt"

// Scope decides what ends an alignment run. The values are ordered from least
// to most alignment; see docs/decisions/formatting-configuration.md D2.
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
	// ScopeDecl ends a run only at a top-level declaration boundary.
	ScopeDecl Scope = "decl"
	// ScopeFile ends a run only at a blank line.
	ScopeFile Scope = "file"
)

// AlignConfig is the alignment half of the configuration. The two cell types
// carry separate scopes because they genuinely want different ones: the
// annotation column is documented to survive an unannotated action inside a
// run, which is ScopeBlock, while ScopeStrict is the right default shape for a
// comment column in a block whose other lines carry no comment.
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
	if c.Indent < 0 || c.Indent > maxIndent {
		return fmt.Errorf("indent must be between 0 and %d, got %d", maxIndent, c.Indent)
	}
	if c.ContinuationIndent < 0 || c.ContinuationIndent > maxIndent {
		return fmt.Errorf("continuation_indent must be between 0 and %d, got %d", maxIndent, c.ContinuationIndent)
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
	case ScopeOff, ScopeStrict, ScopeBlock, ScopeDecl, ScopeFile:
		return nil
	}
	return fmt.Errorf("%s must be one of off, strict, block, decl, file; got %q", key, s)
}
