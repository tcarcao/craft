# Formatting Configuration and Cell Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `craft fmt` configurable, align trailing `//` comments the way it already aligns operation annotations, and give wrapped list continuations a hanging indent so they stop reading as sibling statements.

**Architecture:** A new `internal/fmtconfig` package resolves a `Config` from CLI flags, then the nearest ancestor `.craftfmt`, then built-in defaults. `Config` is threaded into `indentFor`/`separatorFor` (which owns the entire whitespace policy) and into the alignment post-pass. `alignAnnotations` is generalised into `alignCells`, parameterised by a cell extractor and a run scope, and gains a second extractor for trailing comments.

**Tech Stack:** Go 1.25, cobra, `github.com/BurntSushi/toml` (new direct dependency), TypeScript for the VS Code extension.

**Spec:** `docs/decisions/formatting-configuration.md`

## Global Constraints

- **The token-stream invariant holds throughout.** Every non-whitespace token is written verbatim, exactly once, in document order. The formatter's only freedom is the whitespace between tokens. No task may rewrite token text.
- **Idempotence is non-negotiable and must hold under every configuration**, not only the defaults. `format(format(x)) == format(x)`.
- **Continuation and alignment columns must be derived from `depth` or from token text as a pure function, never from the measured column of emitted output.** Deriving from emitted output creates a fixed-point failure; `formatter.go:106-119` and `formatsep.go:131-136` both document prior instances.
- **`params.Options` (LSP `tabSize`/`insertSpaces`) stays ignored.** VS Code's `editor.detectIndentation: true` infers it from file content, which would make the editor and the CLI disagree permanently. See spec D6.
- Default values, exact: `indent = 4`, `continuation_indent = 4`, `align.trailing_comment = "block"`, `align.op_annotation = "block"`, `align.outlier_ratio = 2.5`, `align.outlier_min = 40`.
- Scope values, exact spelling: `"off"`, `"strict"`, `"block"`, `"decl"`, `"file"`.
- Target versions: craft `v2.19.0` (minor), extension `0.2.8` with `lspVersion` `2.19.0`.
- Run `go test ./...` before any commit. There is no CI gate on push in this repo, so local test runs are the only gate.

---

## File Structure

**Created:**
- `internal/fmtconfig/config.go` — `Config`, `Scope`, `Defaults()`, validation
- `internal/fmtconfig/resolve.go` — `.craftfmt` discovery and TOML decode
- `internal/fmtconfig/config_test.go`, `internal/fmtconfig/resolve_test.go`

**Modified:**
- `internal/lsp/formatsep.go` — `indentFor`, `separatorFor` take config; continuation rule
- `internal/lsp/formatalign.go` — `alignAnnotations` becomes `alignCells`; add `splitTrailingComment`
- `internal/lsp/formatter.go` — `commentStart` map; thread `Config`; new exported entry points
- `cmd/craft/fmt.go` — `--indent`, `--continuation-indent`, `--align-trailing-comment` flags
- `internal/lsp/server.go` — `DidChangeWatchedFiles` for `.craftfmt`; config cache
- `internal/lsp/formatter_corpus_test.go` — cross-product property tests, re-bless
- `docs/decisions/token-stream-formatter.md` — amend "Out of scope"
- `docs/page/language/use-cases.md`, `.claude/skills/craft-dsl/SKILL.md`, `~/.agents/skills/craft-dsl/SKILL.md`
- `CHANGELOG.md`

**Extension repo (`craft-vscode-extension/`):**
- `package.json` — `lspVersion`, `contributes.configurationDefaults`
- `client/src/extension.ts` — watcher glob
- `CHANGELOG.md`

---

### Task 1: The `fmtconfig` package

**Files:**
- Create: `internal/fmtconfig/config.go`
- Create: `internal/fmtconfig/config_test.go`

**Interfaces:**
- Produces: `fmtconfig.Config` struct with fields `Indent int`, `ContinuationIndent int`, `Align AlignConfig`; `AlignConfig` with `TrailingComment Scope`, `OpAnnotation Scope`, `OutlierRatio float64`, `OutlierMin int`; `type Scope string` with constants `ScopeOff`, `ScopeStrict`, `ScopeBlock`, `ScopeDecl`, `ScopeFile`; `fmtconfig.Defaults() Config`; `(Config) Validate() error`.

- [ ] **Step 1: Write the failing test**

```go
package fmtconfig

import "testing"

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.Indent != 4 {
		t.Errorf("Indent = %d, want 4", c.Indent)
	}
	if c.ContinuationIndent != 4 {
		t.Errorf("ContinuationIndent = %d, want 4", c.ContinuationIndent)
	}
	if c.Align.TrailingComment != ScopeBlock {
		t.Errorf("TrailingComment = %q, want %q", c.Align.TrailingComment, ScopeBlock)
	}
	if c.Align.OpAnnotation != ScopeBlock {
		t.Errorf("OpAnnotation = %q, want %q", c.Align.OpAnnotation, ScopeBlock)
	}
	if c.Align.OutlierRatio != 2.5 {
		t.Errorf("OutlierRatio = %v, want 2.5", c.Align.OutlierRatio)
	}
	if c.Align.OutlierMin != 40 {
		t.Errorf("OutlierMin = %d, want 40", c.Align.OutlierMin)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Defaults().Validate() = %v, want nil", err)
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"negative indent", func(c *Config) { c.Indent = -1 }, "indent"},
		{"huge indent", func(c *Config) { c.Indent = 17 }, "indent"},
		{"unknown scope", func(c *Config) { c.Align.TrailingComment = "sometimes" }, "trailing_comment"},
		{"negative ratio", func(c *Config) { c.Align.OutlierRatio = -1 }, "outlier_ratio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Defaults()
			tt.mut(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error mentioning %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() = %q, want mention of %q", err, tt.want)
			}
		})
	}
}
```

Imports for this file: `"strings"` and `"testing"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/fmtconfig/ -run TestDefaults -v`
Expected: FAIL, build error, `undefined: Defaults`

- [ ] **Step 3: Write the implementation**

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/fmtconfig/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/fmtconfig/config.go internal/fmtconfig/config_test.go
git commit -m "feat(fmt): add fmtconfig package with defaults and validation"
```

---

### Task 2: `.craftfmt` discovery and decode

**Files:**
- Create: `internal/fmtconfig/resolve.go`
- Create: `internal/fmtconfig/resolve_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: `Config`, `Defaults()`, `Validate()` from Task 1.
- Produces: `fmtconfig.Load(path string) (Config, error)` where `path` is the `.craftfmt` file; `fmtconfig.Find(startDir string) string` returning the nearest ancestor `.craftfmt` or `""`; `fmtconfig.Resolve(filePath string) (Config, error)` combining both and falling back to `Defaults()`.

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get github.com/BurntSushi/toml@v1.4.0
```

TOML rather than YAML: it has no significant whitespace, which matters for a tool whose whole subject is whitespace, and the decoder is small enough to bundle in a goreleaser binary without thought.

- [ ] **Step 2: Write the failing test**

```go
package fmtconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := Resolve(filepath.Join(dir, "a.craft"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != Defaults() {
		t.Errorf("Resolve() = %+v, want Defaults()", got)
	}
}

func TestResolveFindsNearestAncestor(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".craftfmt"), "indent = 2\n")
	write(t, filepath.Join(root, "a", ".craftfmt"), "indent = 8\n")

	got, err := Resolve(filepath.Join(nested, "x.craft"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Indent != 8 {
		t.Errorf("Indent = %d, want 8 (nearest ancestor wins)", got.Indent)
	}
	if got.ContinuationIndent != 4 {
		t.Errorf("ContinuationIndent = %d, want 4 (unset key keeps default)", got.ContinuationIndent)
	}
}

func TestResolveRejectsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "indent = = 4\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want a decode error")
	}
}

func TestResolveRejectsInvalidValue(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "[align]\ntrailing_comment = \"sometimes\"\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want a validation error")
	}
}

func TestResolveRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "indnet = 4\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want an unknown-key error; a typo must not silently do nothing")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/fmtconfig/ -run TestResolve -v`
Expected: FAIL, build error, `undefined: Resolve`

- [ ] **Step 4: Write the implementation**

```go
package fmtconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// FileName is the per-workspace configuration file.
const FileName = ".craftfmt"

// Find returns the path of the nearest .craftfmt at or above startDir, or ""
// when there is none. It stops at the filesystem root rather than at a repo
// boundary, because a workspace is not necessarily a git repo and the formatter
// has no other notion of where a project stops.
func Find(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, FileName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Load decodes one .craftfmt over the defaults, so an absent key keeps its
// default rather than becoming the zero value. An unknown key is an error:
// silently ignoring `indnet = 4` would leave the author believing they had
// configured something.
func Load(path string) (Config, error) {
	cfg := Defaults()
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return Config{}, fmt.Errorf("%s: unknown key %q", path, undecoded[0].String())
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Resolve returns the configuration that applies to one .craft file: the
// nearest ancestor .craftfmt if there is one, otherwise the defaults.
//
// CLI flags are applied by the caller on top of this, because only the caller
// knows which flags the user actually set; see cmd/craft/fmt.go.
func Resolve(filePath string) (Config, error) {
	dir := filepath.Dir(filePath)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	found := Find(dir)
	if found == "" {
		return Defaults(), nil
	}
	return Load(found)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/fmtconfig/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/fmtconfig/ go.mod go.sum
git commit -m "feat(fmt): resolve .craftfmt from nearest ancestor directory"
```

---

### Task 3: Thread `Config` through the whitespace policy

**Files:**
- Modify: `internal/lsp/formatsep.go:13-18` (`indentFor`), `:35` (`separatorFor` signature)
- Modify: `internal/lsp/formatter.go:19-22, 35, 101, 263, 343`
- Modify: `internal/lsp/formatsep_test.go`, `internal/lsp/formatter_test.go`

**Interfaces:**
- Consumes: `fmtconfig.Config` from Task 1.
- Produces: `indentFor(depth int, cfg fmtconfig.Config) string`; `separatorFor(prev *syntax.SyntaxToken, gap string, curr syntax.SyntaxToken, depth int, startsScenario bool, cfg fmtconfig.Config) string`; `lsp.FormatDocumentWith(content string, cfg fmtconfig.Config) string`; `lsp.FormatDocumentCheckedWith(content string, cfg fmtconfig.Config) (string, *craft.Diagnostic)`. `FormatDocument` and `FormatDocumentChecked` keep their current signatures and delegate with `fmtconfig.Defaults()`.

- [ ] **Step 1: Write the failing test**

```go
func TestIndentForRespectsConfig(t *testing.T) {
	four := fmtconfig.Defaults()
	two := fmtconfig.Defaults()
	two.Indent = 2

	if got := indentFor(2, four); got != "        " {
		t.Errorf("indentFor(2, indent=4) = %q, want 8 spaces", got)
	}
	if got := indentFor(2, two); got != "    " {
		t.Errorf("indentFor(2, indent=2) = %q, want 4 spaces", got)
	}
	if got := indentFor(0, four); got != "" {
		t.Errorf("indentFor(0) = %q, want empty", got)
	}
	if got := indentFor(-1, four); got != "" {
		t.Errorf("indentFor(-1) = %q, want empty (floors at zero)", got)
	}
}

func TestFormatDocumentDefaultsToFourSpaces(t *testing.T) {
	src := "services {\nFoo {\ncontexts: A\n}\n}\n"
	want := "services {\n    Foo {\n        contexts: A\n    }\n}\n"
	if got := FormatDocument(src); got != want {
		t.Errorf("FormatDocument() =\n%q\nwant\n%q", got, want)
	}
}

func TestFormatDocumentWithTwoSpaces(t *testing.T) {
	cfg := fmtconfig.Defaults()
	cfg.Indent = 2
	src := "services {\nFoo {\ncontexts: A\n}\n}\n"
	want := "services {\n  Foo {\n    contexts: A\n  }\n}\n"
	if got := FormatDocumentWith(src, cfg); got != want {
		t.Errorf("FormatDocumentWith() =\n%q\nwant\n%q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run 'TestIndentForRespectsConfig|TestFormatDocumentWith' -v`
Expected: FAIL, build error, `too many arguments in call to indentFor` and `undefined: FormatDocumentWith`

- [ ] **Step 3: Change `indentFor`**

```go
// indentFor returns the indentation for a brace depth. It floors at zero: an
// unbalanced `}` only produces a warning-severity diagnostic, so it can reach
// the formatter with depth already at zero, and strings.Repeat panics on a
// negative count.
//
// Width comes from the configuration rather than being fixed, because a
// workspace's existing files are the thing being formatted and half the known
// corpus is 4-space. Config.Validate bounds the width, so the multiplication
// here cannot be driven to exhaustion by a file on disk.
func indentFor(depth int, cfg fmtconfig.Config) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat(" ", cfg.Indent*depth)
}
```

- [ ] **Step 4: Thread the parameter**

Add `cfg fmtconfig.Config` as the final parameter of `separatorFor` (`formatsep.go:35`) and of `rootGapSeparator` (`formatsep.go:170`). Update all six `indentFor` call sites inside `separatorFor` (`formatsep.go:115, 118, 138, 152, 163, 166`) to pass `cfg`. Add `cfg` to `writeTokens` (`formatter.go:263`) and pass it at the `separatorFor` call (`formatter.go:343`).

Then split the entry points in `formatter.go`:

```go
// FormatDocument formats with the built-in defaults.
func FormatDocument(content string) string {
	return FormatDocumentWith(content, fmtconfig.Defaults())
}

// FormatDocumentWith formats under an explicit configuration.
func FormatDocumentWith(content string, cfg fmtconfig.Config) string {
	out, _ := FormatDocumentCheckedWith(content, cfg)
	return out
}

// FormatDocumentChecked is FormatDocumentCheckedWith under the defaults.
func FormatDocumentChecked(content string) (string, *craft.Diagnostic) {
	return FormatDocumentCheckedWith(content, fmtconfig.Defaults())
}
```

Rename the existing body of `FormatDocumentChecked` to `FormatDocumentCheckedWith(content string, cfg fmtconfig.Config)`, keeping its doc comment, and pass `cfg` to `writeTokens` and `alignAnnotations`.

- [ ] **Step 5: Update the existing expectations**

`formatter_test.go` carries 123 literal `"\n  X"` expectations and `formatsep_test.go:342-352` has `TestIndentFor`. Convert them mechanically. Run:

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
go test ./internal/lsp/ 2>&1 | head -40
```

Fix each failing expectation by doubling the indent, since the default moved from 2 to 4. Do **not** change a test's intent, only its whitespace. Where a test is specifically about indent width, pass an explicit `cfg` instead of relying on the default.

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/lsp/ internal/fmtconfig/
git commit -m "feat(fmt): make indent width configurable, default 4"
```

---

### Task 4: Hanging continuation indent

**Files:**
- Modify: `internal/lsp/formatsep.go:155-167` (the newline switch), `:35` (signature)
- Modify: `internal/lsp/formatter.go` (`writeTokens`, to track mid-statement state)
- Test: `internal/lsp/formatsep_test.go`

**Interfaces:**
- Consumes: `indentFor(depth, cfg)` from Task 3.
- Produces: `separatorFor(..., cfg fmtconfig.Config, continuing bool) string`. `continuing` is true when the token being separated is a continuation of a wrapped property value rather than the start of a new statement.

**Why `continuing` is a parameter and not derived:** `separatorFor` sees one token pair at a time. Whether the current token continues a wrapped value is statement-level knowledge only the walker has. It is passed in rather than measured, which is what keeps the column a pure function of `depth` and preserves idempotence. Never compute it from the emitted column of the previous line.

- [ ] **Step 1: Write the failing test**

```go
func TestWrappedListContinuationHangs(t *testing.T) {
	src := "services {\n" +
		"    Foo {\n" +
		"        contexts: A, B,\n" +
		"        C, D\n" +
		"    }\n" +
		"}\n"
	want := "services {\n" +
		"    Foo {\n" +
		"        contexts: A, B,\n" +
		"            C, D\n" +
		"    }\n" +
		"}\n"
	if got := FormatDocument(src); got != want {
		t.Errorf("FormatDocument() =\n%s\nwant\n%s", got, want)
	}
}

func TestContinuationIndentIsIdempotent(t *testing.T) {
	src := "services {\n    Foo {\n        contexts: A, B,\n        C\n    }\n}\n"
	once := FormatDocument(src)
	twice := FormatDocument(once)
	if once != twice {
		t.Errorf("not idempotent:\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestContinuationDoesNotMoveWhenEarlierLineWidens(t *testing.T) {
	// The continuation column must come from depth, never from the measured
	// column of the line above. Widening the key must not move it.
	narrow := FormatDocument("services {\n    F {\n        contexts: A,\n        B\n    }\n}\n")
	wide := FormatDocument("services {\n    F {\n        data-stores: A,\n        B\n    }\n}\n")
	narrowCont := lineWithPrefix(t, narrow, "B")
	wideCont := lineWithPrefix(t, wide, "B")
	if narrowCont != wideCont {
		t.Errorf("continuation moved with key width: %q vs %q", narrowCont, wideCont)
	}
}

func lineWithPrefix(t *testing.T, s, token string) string {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) == token {
			return l
		}
	}
	t.Fatalf("no line %q in\n%s", token, s)
	return ""
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run 'TestWrappedList|TestContinuation' -v`
Expected: FAIL, `TestWrappedListContinuationHangs` shows `C, D` at 8 spaces instead of 12

- [ ] **Step 3: Track mid-statement state in `writeTokens`**

In `writeTokens`, maintain a `continuing` flag. A statement's value begins after a field colon and ends at the next line that is not a continuation. Concretely, set it when the previous non-whitespace token was a comma or a field colon **and** the gap contains a newline; clear it at any `{`, `}`, or when a statement ends.

```go
	// continuing says the token about to be written carries on a property
	// value the author wrapped onto another line, rather than starting a new
	// statement. separatorFor cannot work this out: it sees one token pair and
	// a wrapped `contexts: A,\n B` looks exactly like two statements.
	continuing := false
```

Inside the loop, before the `separatorFor` call:

```go
		if strings.Contains(gap, "\n") {
			continuing = prev != nil &&
				(prev.Kind() == syntax.SyntaxKindComma ||
					(prev.Kind() == syntax.SyntaxKindColon && !isRefColon(*prev)))
		}
		if tok.Kind() == syntax.SyntaxKindLBrace || tok.Kind() == syntax.SyntaxKindRBrace {
			continuing = false
		}
```

and pass `continuing` to `separatorFor`.

- [ ] **Step 4: Use it in the newline switch**

Replace `formatsep.go:155-167` with:

```go
	// A continuation of a wrapped property value hangs one continuation unit
	// past the block indent, so it cannot be mistaken for a sibling statement.
	// Block indent, not visual indent aligned under the value: every formatter
	// that revisited this since 2016 moved the same way, because aligning under
	// the value re-wraps the whole list when the key is renamed. See
	// docs/decisions/formatting-configuration.md D4.
	//
	// The column is a pure function of depth. Deriving it from the emitted
	// column of the previous line would move it again whenever that line
	// shifted, which is a fixed-point failure and has bitten this file twice.
	contIndent := func() string {
		if !continuing {
			return indentFor(depth, cfg)
		}
		return indentFor(depth, cfg) + strings.Repeat(" ", cfg.ContinuationIndent)
	}

	switch strings.Count(gap, "\n") {
	case 0:
		if gap == "" {
			return ""
		}
		// Collapse any run of spaces or tabs to one.
		return " "
	case 1:
		return "\n" + contIndent()
	default:
		// Two or more newlines is a blank line. Collapse any longer run to one.
		return "\n\n" + contIndent()
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/lsp/ -run 'TestWrappedList|TestContinuation' -v`
Expected: PASS

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS. If a corpus file changes shape, inspect the diff before adjusting anything: the corpus harness asserts whitespace-only changes, so a content change is a real bug in this task.

- [ ] **Step 7: Commit**

```bash
git add internal/lsp/
git commit -m "fix(fmt): hang wrapped list continuations instead of flattening them"
```

---

### Task 5: `commentStart` and `splitTrailingComment`

**Files:**
- Modify: `internal/lsp/formatter.go:263` (`writeTokens` return), `:349-384`
- Modify: `internal/lsp/formatalign.go` (add `splitTrailingComment`)
- Test: `internal/lsp/formatalign_test.go`

**Interfaces:**
- Produces: `writeTokens` returns a third map, `commentStart map[int]int`, giving the byte offset where a single-line trailing comment begins on that line; `splitTrailingComment(line string, start int) (body, cell string, ok bool)`.

**Why only single-line comments:** a `//` line comment runs to end of line, so it is always the last thing on its line and is always a trailing cell. A block comment that spans lines has its start on a line the `interior` set already excludes. A block comment closing mid-line is followed by more content and is therefore not trailing. Recording `commentStart` only for comment tokens whose text contains no newline is exactly the set that can be a trailing cell.

- [ ] **Step 1: Write the failing test**

```go
func TestSplitTrailingComment(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		start      int
		wantBody   string
		wantCell   string
		wantOK     bool
	}{
		{"simple", "    kybc   // rollup: x", 11, "    kybc", "// rollup: x", true},
		{"no comment", "    kybc", 0, "", "", false},
		{"comment only line", "    // a note", 4, "", "", false},
		{"start zero means none", "    kybc // x", 0, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, cell, ok := splitTrailingComment(tt.line, tt.start)
			if ok != tt.wantOK || body != tt.wantBody || cell != tt.wantCell {
				t.Errorf("splitTrailingComment(%q, %d) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, tt.start, body, cell, ok, tt.wantBody, tt.wantCell, tt.wantOK)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run TestSplitTrailingComment -v`
Expected: FAIL, `undefined: splitTrailingComment`

- [ ] **Step 3: Implement `splitTrailingComment`**

```go
// splitTrailingComment splits a line into the text before its trailing comment
// and the comment itself. ok is false when the line carries no trailing comment
// cell.
//
// start is the byte offset where the comment token begins, as recorded by
// writeTokens. Like splitAnnotation's `from`, this function does not work it
// out for itself, and for the same reason: pattern-matching `//` cannot tell a
// comment from the `//` inside a narrative phrase such as
// `A asks B for http://x`, and the lexer already knew. A start of zero means
// the walker recorded no trailing comment for this line.
//
// A comment-only line yields an empty body and ok false. Aligning those would
// push a whole run out to match a full-width comment, and a comment on its own
// line is not a cell in the same column as one that follows content.
func splitTrailingComment(line string, start int) (body, cell string, ok bool) {
	if start <= 0 || start > len(line) {
		return "", "", false
	}
	body = strings.TrimRight(line[:start], " \t")
	if body == "" {
		return "", "", false
	}
	cell = strings.TrimRight(line[start:], " \t")
	if cell == "" {
		return "", "", false
	}
	return body, cell, true
}
```

- [ ] **Step 4: Record `commentStart` in `writeTokens`**

Change the signature to return three maps and add the capture. In the loop, capture the pre-advance column and line **before** the newline-counting block at `formatter.go:349`:

```go
		startCol, startLine := col, line
```

Then after `advance(tok.Text())` at `:370`, alongside the existing `commentEnd` block:

```go
		// Where a trailing comment BEGINS is what the comment-alignment pass
		// needs, and it is only meaningful for a comment that does not span
		// lines: such a comment runs to end of line, so it is always the last
		// thing there. A comment carrying newlines starts on a line the
		// interior set already excludes, so recording it would be recording a
		// column for a line this pass never looks at.
		if isCommentKind(tok.Kind()) && !strings.Contains(tok.Text(), "\n") {
			if commentStart == nil {
				commentStart = make(map[int]int, 1)
			}
			commentStart[startLine] = startCol
		}
```

Declare `var commentStart map[int]int` beside the other two, and return it. Update the call site at `formatter.go:101`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/lsp/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/lsp/
git commit -m "feat(fmt): record trailing comment start column in the walker"
```

---

### Task 6: Generalise the aligner into `alignCells` with scope

**Files:**
- Modify: `internal/lsp/formatalign.go:42-93`
- Modify: `internal/lsp/formatter.go:100-102`
- Test: `internal/lsp/formatalign_test.go`

**Interfaces:**
- Consumes: `splitAnnotation`, `splitTrailingComment`, `fmtconfig.Scope`.
- Produces: `type cellSplitter func(line string, hint int) (body, cell string, ok bool)`; `alignCells(s string, interior map[int]bool, hints map[int]int, split cellSplitter, scope fmtconfig.Scope, cfg fmtconfig.Config) string`. `alignAnnotations` is removed; `FormatDocumentCheckedWith` calls `alignCells` twice, annotations first, then trailing comments.

- [ ] **Step 1: Write the failing test**

```go
func TestAlignCellsScopes(t *testing.T) {
	// Two sub-blocks with different natural widths. Under block/strict scope
	// each gets its own column; under decl/file scope they share one.
	in := "domains {\n" +
		"    account {\n" +
		"        aaaaaaaaaa // one\n" +
		"        bb // two\n" +
		"    }\n" +
		"    ad {\n" +
		"        cc // three\n" +
		"    }\n" +
		"}"

	tests := []struct {
		scope    fmtconfig.Scope
		wantCC   string
	}{
		{fmtconfig.ScopeOff, "        cc // three"},
		{fmtconfig.ScopeStrict, "        cc // three"},
		{fmtconfig.ScopeBlock, "        cc // three"},
		{fmtconfig.ScopeDecl, "        cc         // three"},
	}
	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			hints := hintsFor(in)
			got := alignCells(in, nil, hints, splitTrailingComment, tt.scope, fmtconfig.Defaults())
			line := lineContaining(t, got, "three")
			if line != tt.wantCC {
				t.Errorf("scope %s: got %q, want %q", tt.scope, line, tt.wantCC)
			}
		})
	}
}

func TestCellPrecedenceTwoIndependentColumns(t *testing.T) {
	// A line can carry both an annotation and a trailing comment. Each is a
	// separate cell in its own column, and a line carrying only one still takes
	// part in that one's run. Today neither aligner touches such a line.
	src := "use_case \"X\" {\n" +
		"    when A does b\n" +
		"        A asks B to ccccc  [POST /v1/x]  // note\n" +
		"        A asks B to d  [GET /y]\n" +
		"        A asks B to eeeeeeee  // only a comment\n" +
		"        A asks B to f\n" +
		"}\n"
	got := FormatDocument(src)

	annCols := map[string]int{}
	comCols := map[string]int{}
	for _, l := range strings.Split(got, "\n") {
		if i := strings.Index(l, "["); i > 0 {
			annCols[strings.TrimSpace(l)] = i
		}
		if i := strings.Index(l, "//"); i > 0 {
			comCols[strings.TrimSpace(l)] = i
		}
	}
	if len(annCols) != 2 {
		t.Fatalf("expected 2 annotated lines, got %d: %v", len(annCols), annCols)
	}
	if !allEqual(values(annCols)) {
		t.Errorf("annotation column not shared: %v", annCols)
	}
	if !allEqual(values(comCols)) {
		t.Errorf("comment column not shared: %v", comCols)
	}
}

func values(m map[string]int) []int {
	out := make([]int, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func allEqual(xs []int) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i] != xs[0] {
			return false
		}
	}
	return true
}

func TestAlignCellsAlignsWithinABlock(t *testing.T) {
	in := "domains {\n" +
		"    account {\n" +
		"        aaaaaaaaaa // one\n" +
		"        bb // two\n" +
		"    }\n" +
		"}"
	got := alignCells(in, nil, hintsFor(in), splitTrailingComment, fmtconfig.ScopeBlock, fmtconfig.Defaults())
	if l := lineContaining(t, got, "two"); l != "        bb         // two" {
		t.Errorf("got %q", l)
	}
}

// hintsFor builds the commentStart map the walker would produce, for tests that
// exercise the aligner directly rather than through the formatter.
func hintsFor(s string) map[int]int {
	m := map[int]int{}
	for i, l := range strings.Split(s, "\n") {
		if idx := strings.Index(l, "//"); idx > 0 && strings.TrimSpace(l[:idx]) != "" {
			m[i] = idx
		}
	}
	return m
}

func lineContaining(t *testing.T, s, sub string) string {
	t.Helper()
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	t.Fatalf("no line containing %q in\n%s", sub, s)
	return ""
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run TestAlignCells -v`
Expected: FAIL, `undefined: alignCells`

- [ ] **Step 3: Implement `alignCells`**

Replace `alignAnnotations` with the generalised version. Keep the entire existing doc comment about `interior` and `commentEnd`, since it still applies, and add the scope paragraph.

```go
// cellSplitter splits a line into the text before an alignable cell and the
// cell. hint is the per-line offset the walker recorded for this cell type:
// commentEnd for annotations, commentStart for trailing comments.
type cellSplitter func(line string, hint int) (body, cell string, ok bool)

// endsRun reports whether a line terminates an alignment run under a scope.
// A line already known to be interior never reaches here.
func endsRun(line string, hasCell bool, scope fmtconfig.Scope) bool {
	trimmed := strings.TrimSpace(line)
	switch scope {
	case fmtconfig.ScopeFile, fmtconfig.ScopeDecl:
		// Each declaration is aligned separately by the caller, so within one
		// declaration decl and file behave identically: only a blank line ends
		// a run.
		return trimmed == ""
	case fmtconfig.ScopeBlock:
		if trimmed == "" {
			return true
		}
		if strings.HasSuffix(trimmed, "{") || strings.HasPrefix(trimmed, "}") {
			return true
		}
		// A comment on its own line ends a run, matching hclwrite.
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*")
	case fmtconfig.ScopeStrict:
		return !hasCell
	}
	return true
}

func alignCells(s string, interior map[int]bool, hints map[int]int, split cellSplitter, scope fmtconfig.Scope, cfg fmtconfig.Config) string {
	if scope == fmtconfig.ScopeOff {
		return s
	}
	lines := strings.Split(s, "\n")

	runStart := -1
	flush := func(end int) {
		if runStart < 0 {
			return
		}
		widths := make([]int, 0, end-runStart)
		for i := runStart; i < end; i++ {
			if interior[i] {
				continue
			}
			if body, _, ok := split(lines[i], hints[i]); ok {
				widths = append(widths, utf8.RuneCountInString(body))
			}
		}
		col := columnFor(widths, cfg)
		for i := runStart; i < end; i++ {
			if interior[i] {
				continue
			}
			body, cell, ok := split(lines[i], hints[i])
			if !ok {
				continue
			}
			pad := col - utf8.RuneCountInString(body)
			if pad < 1 {
				pad = 1
			}
			lines[i] = body + strings.Repeat(" ", pad) + cell
		}
		runStart = -1
	}

	for i, line := range lines {
		if interior[i] {
			continue
		}
		_, _, hasCell := split(line, hints[i])
		if endsRun(line, hasCell, scope) {
			flush(i)
			continue
		}
		if hasCell && runStart < 0 {
			runStart = i
		}
	}
	flush(len(lines))

	return strings.Join(lines, "\n")
}
```

- [ ] **Step 4: Add a plain `columnFor`**

`flush` above calls `columnFor`, which Task 7 replaces with the outlier-guarded version. Add
the trivial one here so this task builds and its tests pass on their own:

```go
// columnFor picks the alignment column for a run from its body widths.
// Task 7 replaces this with a version that excludes width outliers.
func columnFor(widths []int, cfg fmtconfig.Config) int {
	col := 0
	for _, w := range widths {
		if w+2 > col {
			col = w + 2
		}
	}
	return col
}
```

- [ ] **Step 5: Update the caller**

`formatter.go:100-102` becomes:

```go
			var decl strings.Builder
			interior, commentEnd, commentStart := writeTokens(&decl, node, cfg)
			text := decl.String()
			text = alignCells(text, interior, commentEnd, splitAnnotation, cfg.Align.OpAnnotation, cfg)
			text = alignCells(text, interior, commentStart, splitTrailingComment, cfg.Align.TrailingComment, cfg)
			sb.WriteString(text)
```

The annotation pass runs first so that a line carrying both cells has its annotation column settled before the comment column is measured from the resulting body. This is the cell precedence rule from the spec: annotation is cell 1, trailing comment is cell 2.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/lsp/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/lsp/
git commit -m "feat(fmt): align trailing comments, reusing the annotation aligner"
```

---

### Task 7: The outlier guard

**Files:**
- Modify: `internal/lsp/formatalign.go` (add `columnFor`)
- Test: `internal/lsp/formatalign_test.go`

**Interfaces:**
- Produces: `columnFor(widths []int, cfg fmtconfig.Config) int`, used by `alignCells`'s `flush` in Task 6.

- [ ] **Step 1: Write the failing test**

```go
func TestColumnForIgnoresOutliers(t *testing.T) {
	cfg := fmtconfig.Defaults() // ratio 2.5, min 40

	// Mean below outlier_min: the guard is inert and the widest body wins.
	if got := columnFor([]int{10, 12, 25}, cfg); got != 27 {
		t.Errorf("columnFor(small population) = %d, want 27", got)
	}

	// Mean above outlier_min with one extreme member: the extreme is excluded.
	widths := []int{50, 50, 50, 50, 300}
	got := columnFor(widths, cfg)
	if got > 60 {
		t.Errorf("columnFor(with outlier) = %d, want the outlier excluded (~52)", got)
	}
}

func TestColumnForDisabledGuard(t *testing.T) {
	cfg := fmtconfig.Defaults()
	cfg.Align.OutlierRatio = 0
	widths := []int{50, 50, 50, 50, 300}
	if got := columnFor(widths, cfg); got != 302 {
		t.Errorf("columnFor(ratio=0) = %d, want 302 (guard disabled)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run TestColumnFor -v`
Expected: FAIL, `undefined: columnFor`

- [ ] **Step 3: Implement**

```go
// columnFor picks the alignment column for a run from its body widths.
//
// The column is the widest body plus two, except that a body far wider than the
// rest is excluded. gofmt does the same on its exprList path, comparing against
// the geometric mean of the run's sizes with a ratio of 2.5 and a floor of 40,
// and rustfmt's opt-in alignment builds the same idea into its threshold. One
// 300-character line should not push forty 50-character lines out to match it.
//
// The guard is deliberately weak. It excludes extremes, not ordinary variation:
// a 26-wide body in a population averaging 11 is not an outlier by any ratio
// test. Bounding ordinary churn is the scope setting's job, not this one's.
func columnFor(widths []int, cfg fmtconfig.Config) int {
	col := 0
	if len(widths) == 0 {
		return 0
	}
	if cfg.Align.OutlierRatio <= 0 {
		for _, w := range widths {
			if w+2 > col {
				col = w + 2
			}
		}
		return col
	}

	sum := 0.0
	n := 0
	for _, w := range widths {
		if w <= 0 {
			continue
		}
		sum += math.Log(float64(w))
		n++
	}
	limit := math.Inf(1)
	if n > 0 {
		mean := math.Exp(sum / float64(n))
		if mean > float64(cfg.Align.OutlierMin) {
			limit = cfg.Align.OutlierRatio * mean
		}
	}

	for _, w := range widths {
		if float64(w) > limit {
			continue
		}
		if w+2 > col {
			col = w + 2
		}
	}
	// Every member excluded means the run is all outliers relative to itself,
	// which cannot happen for a non-empty set with a finite limit, but a zero
	// column would silently collapse every cell to one space, so fall back.
	if col == 0 {
		for _, w := range widths {
			if w+2 > col {
				col = w + 2
			}
		}
	}
	return col
}
```

Add `"math"` to the imports.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/lsp/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lsp/
git commit -m "feat(fmt): exclude width outliers from alignment columns"
```

---

### Task 8: CLI flags

**Files:**
- Modify: `cmd/craft/fmt.go:50-70`
- Test: `cmd/craft/fmt_test.go`

**Interfaces:**
- Consumes: `fmtconfig.Resolve`, `lsp.FormatDocumentCheckedWith`.
- Produces: `--indent int`, `--continuation-indent int`, `--align-trailing-comment string` on `craft fmt`. A flag the user did not set does not override `.craftfmt`; only an explicitly-set flag does, which is what `cmd.Flags().Changed(name)` reports.

- [ ] **Step 1: Write the failing test**

```go
func TestFmtHonoursCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	src := filepath.Join(dir, "a.craft")
	os.WriteFile(src, []byte("services {\nFoo {\ncontexts: A\n}\n}\n"), 0o644)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{src}, false, fmtconfig.Config{}, false, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d, stderr=%s", code, errOut.String())
	}
	got, _ := os.ReadFile(src)
	want := "services {\n  Foo {\n    contexts: A\n  }\n}\n"
	if string(got) != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestFmtFlagOverridesCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	src := filepath.Join(dir, "a.craft")
	os.WriteFile(src, []byte("services {\nFoo {\ncontexts: A\n}\n}\n"), 0o644)

	override := fmtconfig.Defaults()
	override.Indent = 8
	var out, errOut bytes.Buffer
	if code := runFmt([]string{src}, false, override, true, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d", code)
	}
	got, _ := os.ReadFile(src)
	if !strings.Contains(string(got), "\n        contexts: A") {
		t.Errorf("flag did not override .craftfmt, got\n%q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/craft/ -run TestFmt -v`
Expected: FAIL, `too many arguments in call to runFmt`

- [ ] **Step 3: Implement**

Change the signature to `runFmt(files []string, check bool, override fmtconfig.Config, hasOverride bool, out, errOut io.Writer) int`. Per file:

```go
		// Resolve .craftfmt for EVERY file, whether or not flags were set, then
		// layer only the flags the user actually named on top. Swapping in a
		// Defaults()-seeded override wholesale would let `--indent 4` silently
		// discard a .craftfmt's trailing_comment, and would skip opening the
		// file at all so a malformed one went unreported.
		cfg, err := fmtconfig.Resolve(file)
		if err != nil {
			fmt.Fprintf(errOut, "%s: %v\n", file, err)
			failed = true
			continue
		}
		applyOverrides(&cfg, set)
```

where `set` records which flags were `Changed` and `applyOverrides` assigns only those
fields. Per file, per field.


then pass `cfg` to `lsp.FormatDocumentCheckedWith`. Add the flags:

```go
	var indent, contIndent int
	var trailingComment string
	cmd.Flags().IntVar(&indent, "indent", 0,
		"indent width in spaces; overrides .craftfmt")
	cmd.Flags().IntVar(&contIndent, "continuation-indent", 0,
		"extra indent for a wrapped list continuation; overrides .craftfmt")
	cmd.Flags().StringVar(&trailingComment, "align-trailing-comment", "",
		"trailing comment alignment scope: off, strict, block, decl, file")
```

and in `RunE`, build the override only from flags the user actually set:

```go
			cfg := fmtconfig.Defaults()
			hasOverride := false
			if cmd.Flags().Changed("indent") {
				cfg.Indent, hasOverride = indent, true
			}
			if cmd.Flags().Changed("continuation-indent") {
				cfg.ContinuationIndent, hasOverride = contIndent, true
			}
			if cmd.Flags().Changed("align-trailing-comment") {
				cfg.Align.TrailingComment, hasOverride = fmtconfig.Scope(trailingComment), true
			}
			if hasOverride {
				if err := cfg.Validate(); err != nil {
					return err
				}
			}
```

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/craft/ -v`
Expected: PASS

- [ ] **Step 5: Update the command help text**

The `fmt` command's `Long` description must mention `.craftfmt` and the resolution order, since the existing text describes behaviour that is no longer complete.

- [ ] **Step 6: Commit**

```bash
git add cmd/craft/
git commit -m "feat(cli): add fmt flags and .craftfmt resolution"
```

---

### Task 9: LSP config resolution and invalidation

**Files:**
- Modify: `internal/lsp/server.go:633-639` (`DidChangeWatchedFiles`, `DidChangeConfiguration`), `:1327-1335` (formatting handler)
- Test: `internal/lsp/server_test.go`

**Interfaces:**
- Consumes: `fmtconfig.Resolve`.
- Produces: the formatting handler resolves config from the document's path and formats with it; `DidChangeWatchedFiles` clears a per-directory config cache.

- [ ] **Step 1: Write the failing test**

```go
func TestFormattingUsesCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	os.WriteFile(path, []byte(content), 0o644)

	got := formatForURI(t, uri.File(path), content)
	if !strings.Contains(got, "\n  Foo {") {
		t.Errorf("server ignored .craftfmt, got\n%q", got)
	}
}

func TestFormattingIgnoresClientTabSize(t *testing.T) {
	// editor.detectIndentation infers tabSize from file content, so honouring
	// it would make the editor and the CLI disagree permanently. The server
	// must ignore it. See docs/decisions/formatting-configuration.md D6.
	dir := t.TempDir()
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	os.WriteFile(path, []byte(content), 0o644)

	got := formatWithOptions(t, uri.File(path), content, protocol.FormattingOptions{"tabSize": float64(2)})
	if !strings.Contains(got, "\n    Foo {") {
		t.Errorf("server honoured client tabSize, got\n%q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run TestFormatting -v`
Expected: FAIL, first test shows 4-space output

- [ ] **Step 3: Implement**

In the formatting handler, derive the filesystem path from the document URI and resolve. A resolution error becomes a diagnostic, not a silent fallback, so a typo in `.craftfmt` is visible:

```go
	cfg := fmtconfig.Defaults()
	if p := uri.New(string(params.TextDocument.URI)).Filename(); p != "" {
		resolved, err := fmtconfig.Resolve(p)
		if err != nil {
			s.publishConfigError(ctx, params.TextDocument.URI, err)
			return nil, nil
		}
		cfg = resolved
	}
	formatted, diag := FormatDocumentCheckedWith(content, cfg)
```

Implement `DidChangeWatchedFiles` to drop any cached config when a `.craftfmt` changes. If no cache is introduced, the handler still needs to exist and re-resolve, because `Resolve` reads from disk on every call and that is correct but must not be defeated by a stale cache added later.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/lsp/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lsp/
git commit -m "feat(lsp): resolve .craftfmt per document, keep ignoring tabSize"
```

---

### Task 10: Corpus properties across the configuration cross-product

**Files:**
- Modify: `internal/lsp/formatter_corpus_test.go:177-234`

**Interfaces:**
- Consumes: everything above.

This is the highest-value task in the plan. Continuation indent and comment alignment both run over the same lines, and a continuation line carries no comment cell, so under `strict` it breaks a comment run and under `block` it does not. Only a cross-product test catches an interaction.

- [ ] **Step 1: Write the failing test**

```go
func TestCorpusPropertiesAcrossConfigs(t *testing.T) {
	files := allCraftFiles(t)
	configs := []fmtconfig.Config{}
	for _, indent := range []int{2, 4} {
		for _, scope := range []fmtconfig.Scope{
			fmtconfig.ScopeOff, fmtconfig.ScopeStrict,
			fmtconfig.ScopeBlock, fmtconfig.ScopeDecl, fmtconfig.ScopeFile,
		} {
			c := fmtconfig.Defaults()
			c.Indent = indent
			c.Align.TrailingComment = scope
			configs = append(configs, c)
		}
	}

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, cfg := range configs {
			name := fmt.Sprintf("%s/indent=%d/scope=%s", filepath.Base(path), cfg.Indent, cfg.Align.TrailingComment)
			t.Run(name, func(t *testing.T) {
				once := FormatDocumentWith(string(src), cfg)
				twice := FormatDocumentWith(once, cfg)
				if once != twice {
					t.Errorf("not idempotent under %+v", cfg)
				}
				assertWhitespaceOnlyChange(t, string(src), once)
				assertReparses(t, once)
				assertModelPreserved(t, string(src), once)
			})
		}
	}
}
```

Reuse the existing helpers for the three non-idempotence properties rather than writing new ones; if they are currently inline in `TestFormatDocument_Corpus`, extract them first as a mechanical refactor with no behaviour change.

- [ ] **Step 2: Run test to verify it fails or passes**

Run: `go test ./internal/lsp/ -run TestCorpusPropertiesAcrossConfigs 2>&1 | tail -30`
Expected: this may pass immediately. If it fails, the failure is a real interaction bug from Tasks 4 through 7 and must be fixed there, not by weakening this test.

- [ ] **Step 3: Re-bless the canonical corpus**

`TestFormatDocument_CanonicalCorpusIsByteIdentical` (`:213-234`) logs the count of already-canonical files. Under the new defaults that set changes. Reformat the corpus:

```bash
go run ./cmd/craft fmt 'testdata/**/*.craft' 'examples/**/*.craft' 'internal/**/*.craft'
go test ./internal/lsp/ -run TestFormatDocument_CanonicalCorpus -v 2>&1 | tail -5
```

Record the new canonical count in the commit message.

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lsp/ testdata/ examples/
git commit -m "test(fmt): assert corpus properties across the config cross-product"
```

---

### Task 11: Documentation in the craft repo

**Files:**
- Modify: `docs/decisions/token-stream-formatter.md:333-337`
- Modify: `docs/page/language/use-cases.md:237`
- Modify: `.claude/skills/craft-dsl/SKILL.md:403-405`
- Modify: `~/.agents/skills/craft-dsl/SKILL.md:403-405`

- [ ] **Step 1: Amend the superseded ADR**

`token-stream-formatter.md` lists "changing indent width, or any style decision not listed under Behaviour changes" under **Out of scope**. That is now contradicted. Add, immediately under the "Out of scope" heading:

```markdown
> **Amended 2026-08-21 by `docs/decisions/formatting-configuration.md`.** Indent width,
> trailing-comment alignment, and continuation indent are now configurable. The invariant
> above is unchanged: the formatter still writes every non-whitespace token verbatim,
> exactly once, in document order, and configuration only widens what it may choose for the
> whitespace between them.
```

- [ ] **Step 2: Fix the docs site**

`docs/page/language/use-cases.md:237` says *"Formatting canonicalises layout only: 2-space indentation…"*. Replace the width claim with the configurable one and remove the stale *"comments on their own line"* clause, which has been wrong since the token-stream rewrite.

- [ ] **Step 3: Fix both copies of the skill**

Both `.claude/skills/craft-dsl/SKILL.md` and `~/.agents/skills/craft-dsl/SKILL.md` say *"It applies 2-space indent"* and describe the annotation-alignment run rule. The two files are byte-identical, so make the same edit in both. Update to: 4-space default, configurable via `.craftfmt`, and note that trailing comments are aligned per block.

**The second path is outside every repo**, so it will not appear in `git status`. Do not skip it.

- [ ] **Step 4: Verify the skill copies still match**

Run: `diff .claude/skills/craft-dsl/SKILL.md ~/.agents/skills/craft-dsl/SKILL.md`
Expected: no output. Note that `diff` here may be wrapped by a tool that always exits 0, so read the output rather than trusting the exit status.

- [ ] **Step 5: Commit**

```bash
git add docs/ .claude/skills/
git commit -m "docs: record formatting configuration and amend the formatter ADR"
```

---

### Task 12: VS Code extension

**Files:**
- Modify: `craft-vscode-extension/package.json:6` (`lspVersion`), `contributes.configurationDefaults`
- Modify: `craft-vscode-extension/client/src/extension.ts:53` (watcher glob)
- Modify: `craft-vscode-extension/CHANGELOG.md`

**Interfaces:**
- Consumes: craft `v2.19.0`, which must be tagged and released first (Task 13). The extension's release workflow downloads `craft_${lspVersion}_linux_amd64.tar.gz` and fails if that tag is not published.

- [ ] **Step 1: Declare craft's indent to the editor**

The extension must tell VS Code craft's width, rather than the server asking VS Code. `editor.detectIndentation` defaults to true and infers `tabSize` from file content, so without this an existing 2-space file autoindents at 2 while the formatter emits 4. Add to `contributes.configurationDefaults`, alongside the existing `[craft]` entry:

```json
"[craft]": {
    "editor.semanticHighlighting.enabled": true,
    "editor.tabSize": 4,
    "editor.detectIndentation": false
}
```

- [ ] **Step 2: Make `.craftfmt` edits reach the server**

`client/src/extension.ts:53` watches `'**/*.craft'`, which does not match `.craftfmt`. Widen it:

```ts
    const watcher = workspace.createFileSystemWatcher('**/{*.craft,.craftfmt}');
```

Without this, and without the `DidChangeWatchedFiles` handler from Task 9, editing `.craftfmt` has no effect until the window reloads.

- [ ] **Step 3: Bump the server pin**

`package.json:6`: `"lspVersion": "2.19.0"`.

Do **not** edit `update-grammar.sh`. It is dead code: it `cd server` at `:61` and `server/` does not exist, and it fetches an ANTLR-era grammar path the repo abandoned. It pins nothing.

- [ ] **Step 4: Add the changelog entry**

Match the existing format: `## [0.2.8] — <date>`, then `### Added` / `### Fixed`, and open the server-moving entry with the house phrasing:

```markdown
## [0.2.8] — 2026-08-21

### Changed

- Bundled LSP binary updated to `craft v2.19.0`.
- **Craft files now indent at 4 spaces by default**, and the editor is told so
  explicitly, so autoindent and format-on-save agree. A workspace that prefers a
  different width adds a `.craftfmt` file with `indent = 2`.
- Trailing `//` comments are now column-aligned within their block, and wrapped
  `contexts:` lists indent their continuation lines.

### Fixed

- Editing `.craftfmt` now takes effect without reloading the window.
```

- [ ] **Step 5: Build and test**

Run:
```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension
npm run compile && npx jest
```
Expected: PASS. The five existing suites do not assert formatted output and the extension contains no `.craft` fixtures, so test churn here should be zero. If a `binaryManager` test fails, check whether it hardcodes a version fixture (`binaryManager.test.ts:90,161,179` use a synthetic `v0.1.0`, which is unrelated to the real pin).

- [ ] **Step 6: Commit**

```bash
git add package.json client/src/extension.ts CHANGELOG.md
git commit -m "feat: craft v2.19.0, 4-space default, .craftfmt watching"
```

---

### Task 13: Release

**Files:**
- Modify: `craft/CHANGELOG.md`

**Ordering is mechanical, not advisory:** the extension's release workflow (`craft-vscode-extension/.github/workflows/release.yml:112-132`) downloads the craft release asset and verifies its checksum. If craft is not tagged first, the extension release fails.

- [ ] **Step 1: Write the craft changelog entry**

The house format requires a `**Why a minor version.**` paragraph justifying the bump against `pkg/craft`. The honest framing, following the precedent 2.17.2 set for a behaviour break:

```markdown
## [2.19.0] — 2026-08-21

**Why a minor version.** `pkg/craft`'s exported surface is unchanged: the formatter lives in
`internal/lsp` and no exported symbol was removed or altered. Under Go module semantics that
is a minor. But read the first bullet before upgrading, because **output changes for every
existing file**.

### Changed

- **The default indent is now 4 spaces, was 2.** Every `.craft` file reformats on the next
  `craft fmt`, and `craft fmt --check` will report previously-clean files as unformatted
  until the workspace either accepts the reflow or adds a `.craftfmt` containing
  `indent = 2`.

### Added

- **`.craftfmt` configuration.** A TOML file resolved from the nearest ancestor directory,
  with `indent`, `continuation_indent`, and an `[align]` table. CLI flags `--indent`,
  `--continuation-indent` and `--align-trailing-comment` override it.
- **Trailing `//` comments are column-aligned**, using the same run machinery that already
  aligned `[POST /v1/...]` operation annotations. The default scope is `block`: a run ends at
  a blank line, a `{`, a `}`, or a comment-only line, which keeps a new long name from
  re-indenting an entire file.

### Fixed

- **Wrapped list continuations now hang** one continuation unit past the block indent.
  Previously the formatter kept the author's line break but reset the indent to block depth,
  so the last item of a wrapped `contexts:` list was indistinguishable from a sibling
  statement.

**Note:** the language server ignores the editor's `tabSize`, deliberately. VS Code infers it
from file content, which would make the editor and the CLI disagree permanently. The
extension declares craft's width to the editor instead.
```

- [ ] **Step 2: Verify the tree is green**

Run: `go test ./...`
Expected: PASS. There is no CI gate on push in this repo, so this local run is the only gate before tagging.

- [ ] **Step 3: Tag and push craft**

```bash
git tag v2.19.0
git push origin main --tags
```

- [ ] **Step 4: Verify the registries, not the CI run**

Wait for goreleaser, then confirm the artifacts actually exist:

```bash
gh release view v2.19.0 --repo tcarcao/craft --json assets --jq '.assets[].name'
curl -sI https://github.com/tcarcao/craft/releases/download/v2.19.0/checksums.txt | head -1
brew info tcarcao/craft/craft 2>/dev/null | head -3
```

Expected: the platform archives and `checksums.txt` are listed, and brew reports 2.19.0. Note that the local `homebrew-craft` clone is stale at 2.3.3; check the remote tap, not the clone.

- [ ] **Step 5: Release the extension**

Only after step 4 confirms the craft assets exist:

```bash
cd ../craft-vscode-extension
./release.sh 0.2.8
```

`0.2.8` is an even minor, which the workflow (`release.yml:80-103`) routes to the stable Marketplace channel. Use `0.3.0` instead if this should soak on the pre-release channel first, which is worth considering given the default-output change. Minor parity here means odd/even minor selecting the channel, and has nothing to do with matching craft's version number.

- [ ] **Step 6: Verify the Marketplace**

```bash
npx vsce show tcarcao.craft-arch-diagrams --json | head -20
```

Expected: the published version matches what was released.

---

## Deferred, deliberately

- **A `craft fmt --check` CI gate.** Neither repo runs one today, so nothing catches corpus drift automatically. It should follow this change, but adding it in the same release would turn every downstream repo's CI red at the same moment their files reformat.
- **`tree-sitter-craft`.** `queries/format.scm` carries no indent width, so it needs no change. Its `CORPUS_VERSION` is already one release behind at `v2.17.0`; that is pre-existing drift, not caused here.
- **`craft/.github/workflows/tree-sitter-compat.yml`** triggers on `testdata/corpus/**` and will fire on the reformatted corpus. It is `continue-on-error: true` and whitespace-only changes parse fine, so expect noise, not failure.
