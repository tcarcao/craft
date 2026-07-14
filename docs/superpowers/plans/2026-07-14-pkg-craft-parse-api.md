# pkg/craft Importable Parse API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add stable `pkg/craft.Parse` / `pkg/craft.ParseFiles` functions so external Go modules can parse Craft DSL in-process, and make `cmd/craft`'s `check`/`inspect`/`validate` thin wrappers over them.

**Architecture:** `internal/syntax` and `internal/sema` already import `pkg/craft` (they return `craft.CraftDoc` / `craft.Diagnostic`). That makes the naive "wrapper in `pkg/craft` calling `internal/syntax`" a compile-time import cycle. We break it by moving the type *definitions* down into a new leaf package `internal/model`; `pkg/craft` re-exports them via **type aliases** (`type CraftDoc = model.CraftDoc`) and additionally hosts `Parse`/`ParseFiles`, which import `internal/syntax` + `internal/sema` (which now depend on `internal/model`, not back on `pkg/craft`). Aliases are identical types, so every existing importer of `pkg/craft` compiles unchanged and JSON output is byte-identical.

**Tech Stack:** Go 1.25.0, cobra CLI, existing `internal/syntax` (recursive-descent → green tree → projection) and `internal/sema` (symbols + validation + lint).

## Global Constraints

- **Public import path is `github.com/tcarcao/craft/pkg/craft`.** The consumer must be able to write `import ".../pkg/craft"` and call `craft.Parse(...) (*craft.CraftDoc, []craft.Diagnostic, error)`. Do not introduce a second package for the parse functions.
- **Preserve all `CraftDoc`/`Service`/`Domain`/`UseCase`/`Scenario`/`Action`/`Trigger`/`Actor`/`Diagnostic`/enum fields and JSON tags exactly.** Only additive changes allowed. Moving a type to `internal/model` and aliasing it in `pkg/craft` is NOT a field/tag change — the type identity and tags are preserved. Do not rename fields or alter `json:"..."` tags.
- **Behavior preservation:** after each CLI refactor, the command's output for a given input must be unchanged for the common case. Two intentional, documented, path-only normalizations are permitted (see Design Decisions D5, D6); no other observable output changes.
- **Diagnostics are data, not errors.** `Parse`/`ParseFiles` return diagnostics in the `[]Diagnostic` slot; the `error` slot is reserved for programmer-error/invariant conditions (currently none — return `nil`). Neither function performs file I/O; callers pass `[]byte`.
- **`internal/model` is a leaf.** It must import nothing from `internal/` or `pkg/`. It is pure type/const definitions.
- Module Go floor stays `1.25.0`. Note for consumers (docs/CHANGELOG only): importing modules need Go ≥ 1.25.
- Version bump for the release: `v2.9.0` → **`v2.10.0`** (additive minor).
- Do NOT modify anything under `.worktrees/` (separate detached worktrees).

---

## Design Decisions (baked in; rationale for reviewers)

- **D1 — Types move to `internal/model`, `pkg/craft` aliases them.** Forced by the import cycle (`internal/syntax`+`internal/sema` import `pkg/craft`). Chosen over a separate `pkg/craftparse` package so the consumer gets a single `pkg/craft` import (user requirement).
- **D2 — Only `internal/syntax` and `internal/sema` switch their import** from `pkg/craft` to `internal/model` (incl. their `_test.go` files, to avoid a test-build cycle). `green`/`ast` never imported `pkg/craft`. All other packages keep importing `pkg/craft` (aliases) unchanged.
- **D3 — `Parse`/`ParseFiles` mirror the *current* CLI call sequence, including its quirk of NOT forwarding `green.LineIndex` to `sema.AnalyzeFile`/`LintWorkspace`.** This preserves today's diagnostic positions exactly. The latent position-accuracy improvement (forwarding `li`) is explicitly deferred as a follow-up, not folded in here.
- **D4 — `ParseFiles` runs the full workspace pipeline** (`MergeWorkspaceSymbols → AnalyzeWorkspace → LintWorkspace`) so `validate` can route through it, and merges **all** `CraftDoc` slices (Architectures/Exposures/ContextMap included, beyond the 4 `inspect` merges today). `inspect` only reads 4 of them, so its output is unchanged; the library is more complete.
- **D5 — `ParseFiles` processes files in ascending map-key order** (deterministic). Merged-model array order and diagnostic order therefore follow sorted filename order. For `inspect`/`validate` invoked with args not already in sorted order, multi-file array/line order becomes sorted (was arg/glob order). No existing golden asserts this; it is a deterministic improvement.
- **D6 — Diagnostic `SourceURI` contract:** `ParseFiles` stamps `SourceURI` = the caller's map key (bare filename, not `file://…`) on **every** returned diagnostic. Internally it uses `file://`+key only to correlate sema passes, then strips the prefix before returning. Consequence: `validate`'s workspace-diagnostic text lines lose the stray `file://` prefix they print today, becoming consistent with per-file lines. `Parse` (single file) leaves `SourceURI` as the underlying passes produce it (empty) — the single caller already knows the filename — so `check --lsp-json` output is byte-identical.

---

## File Structure

- Create: `LICENSE` (MIT)
- Create: `internal/model/model.go` (moved type + const definitions; `package model`)
- Rewrite: `pkg/craft/craftdoc.go`, `pkg/craft/diagnostic.go` → alias re-exports of `internal/model`
- Create: `pkg/craft/parse.go` (`Parse`, `ParseFiles`, unexported helpers)
- Create: `pkg/craft/parse_test.go`
- Modify: `internal/syntax/*.go` (incl. tests) — import `internal/model` instead of `pkg/craft`
- Modify: `internal/sema/*.go` (incl. tests) — import `internal/model` instead of `pkg/craft`
- Modify: `cmd/craft/check.go` (route through `craft.Parse`; `buildSymbolsJSON` from doc; write to `cmd.OutOrStdout()`)
- Modify: `cmd/craft/inspect.go` (route through `craft.ParseFiles`; write to `cmd.OutOrStdout()`)
- Modify: `cmd/craft/validate.go` (extract `runValidate`; route through `craft.ParseFiles`)
- Create: `cmd/craft/check_test.go`, `cmd/craft/inspect_test.go`, `cmd/craft/validate_test.go`
- Modify: `CHANGELOG.md`

---

## Task 1: Add LICENSE (MIT)

**Files:**
- Create: `LICENSE`

Independent of all code. The README already claims MIT; there is no `LICENSE` file. Copyright holder confirmed from `git config user.name`: `Tiago Carção`.

- [ ] **Step 1: Write the MIT LICENSE file**

Create `LICENSE` with the standard MIT text:

```
MIT License

Copyright (c) 2026 Tiago Carção

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Verify build still green**

Run: `go build ./... && go test ./... 2>&1 | tail -5`
Expected: PASS (no code changed).

- [ ] **Step 3: Commit**

```bash
git add LICENSE
git commit -m "chore: add MIT LICENSE file"
```

---

## Task 2: Extract types to internal/model; alias in pkg/craft

**Files:**
- Create: `internal/model/model.go`
- Rewrite: `pkg/craft/craftdoc.go`, `pkg/craft/diagnostic.go`
- Modify: every non-`_test` and `_test` `.go` file in `internal/syntax/` and `internal/sema/` that imports `pkg/craft`

**Interfaces:**
- Produces: `internal/model` exporting exactly the symbols currently in `pkg/craft` (types + consts). `pkg/craft` exports the same names as aliases, so `craft.CraftDoc == model.CraftDoc`, etc.

This is a **pure refactor with zero behavior change** — the whole-suite green run is the gate. It is mechanical but touches many files.

- [ ] **Step 1: Create `internal/model/model.go`**

Move the *entire* type/const bodies from `pkg/craft/craftdoc.go` and `pkg/craft/diagnostic.go` into one file `internal/model/model.go` with `package model`. Include every type and const verbatim (identical fields, identical `json:"..."` tags, identical doc comments): from craftdoc.go — `CraftDoc, Edge, ArchBlock, Component, ComponentType (+ ComponentTypeSimple/Flow), ComponentModifier, Exposure, Service, DeploymentStrategy, DeploymentRule, UseCase, Scenario, Trigger, TriggerType (+ TriggerTypeExternal/Event/DomainListen), Action, ActionType (+ ActionTypeSync/Async/Internal/Return), Interaction, Domain, Actor, ActorType (+ ActorTypeUser/System/Service)`; from diagnostic.go — `Diagnostic, Severity (+ SeverityError/Warning/Info/Hint), Range, Position`.

Package comment for `internal/model`:
```go
// Package model holds the canonical Craft document data types. It is a leaf
// package (imports nothing from internal/ or pkg/) so that internal/syntax and
// internal/sema can produce these types without importing pkg/craft, which
// re-exports them as the stable public API.
package model
```

- [ ] **Step 2: Rewrite `pkg/craft/craftdoc.go` and `pkg/craft/diagnostic.go` as alias re-exports**

Replace the bodies with alias + const re-exports. `pkg/craft/craftdoc.go`:

```go
// Package craft is the stable public Go API for the Craft DSL toolchain: the
// canonical CraftDoc document model, the Diagnostic type, and the Parse /
// ParseFiles entry points. (Stability contract set in Task 8.)
package craft

import "github.com/tcarcao/craft/internal/model"

type (
	CraftDoc           = model.CraftDoc
	Edge               = model.Edge
	ArchBlock          = model.ArchBlock
	Component          = model.Component
	ComponentType      = model.ComponentType
	ComponentModifier  = model.ComponentModifier
	Exposure           = model.Exposure
	Service            = model.Service
	DeploymentStrategy = model.DeploymentStrategy
	DeploymentRule     = model.DeploymentRule
	UseCase            = model.UseCase
	Scenario           = model.Scenario
	Trigger            = model.Trigger
	TriggerType        = model.TriggerType
	Action             = model.Action
	ActionType         = model.ActionType
	Interaction        = model.Interaction
	Domain             = model.Domain
	Actor              = model.Actor
	ActorType          = model.ActorType
)

const (
	ComponentTypeSimple = model.ComponentTypeSimple
	ComponentTypeFlow   = model.ComponentTypeFlow

	TriggerTypeExternal     = model.TriggerTypeExternal
	TriggerTypeEvent        = model.TriggerTypeEvent
	TriggerTypeDomainListen = model.TriggerTypeDomainListen

	ActionTypeSync     = model.ActionTypeSync
	ActionTypeAsync    = model.ActionTypeAsync
	ActionTypeInternal = model.ActionTypeInternal
	ActionTypeReturn   = model.ActionTypeReturn

	ActorTypeUser    = model.ActorTypeUser
	ActorTypeSystem  = model.ActorTypeSystem
	ActorTypeService = model.ActorTypeService
)
```

`pkg/craft/diagnostic.go`:

```go
package craft

import "github.com/tcarcao/craft/internal/model"

type (
	Diagnostic = model.Diagnostic
	Severity   = model.Severity
	Range      = model.Range
	Position   = model.Position
)

const (
	SeverityError   = model.SeverityError
	SeverityWarning = model.SeverityWarning
	SeverityInfo    = model.SeverityInfo
	SeverityHint    = model.SeverityHint
)
```

Keep the "Experimental: stabilizes at v0.1" wording OUT — but do not yet write the final stability contract (that is Task 8). A neutral package comment as shown above is fine for now.

- [ ] **Step 3: Repoint `internal/syntax` and `internal/sema` at `internal/model`**

In every `.go` file (including `_test.go`) under `internal/syntax/` and `internal/sema/` that imports `github.com/tcarcao/craft/pkg/craft`:
- change the import to `github.com/tcarcao/craft/internal/model`
- change the local qualifier from `craft.` to `model.` for every reference (e.g. `craft.Diagnostic` → `model.Diagnostic`, `craft.CraftDoc` → `model.CraftDoc`, `craft.SeverityError` → `model.SeverityError`).

Find them:
```bash
grep -rl "tcarcao/craft/pkg/craft" internal/syntax internal/sema
```
`_test.go` files must switch too: an internal (`package syntax`) test importing `pkg/craft` while the package imports `internal/model` would re-form the cycle in the test binary.

- [ ] **Step 4: Verify build + full suite green (the behavior-preservation gate)**

Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: PASS, all packages. No test should change output — aliases are identical types. If anything fails to compile with "import cycle", a `pkg/craft` reference was missed in `internal/syntax`/`internal/sema`.

- [ ] **Step 5: Commit**

```bash
git add internal/model pkg/craft/craftdoc.go pkg/craft/diagnostic.go internal/syntax internal/sema
git commit -m "refactor: move CraftDoc/Diagnostic types to internal/model, alias in pkg/craft"
```

---

## Task 3: pkg/craft.Parse (single file)

**Files:**
- Create: `pkg/craft/parse.go`
- Create: `pkg/craft/parse_test.go`

**Interfaces:**
- Consumes: `internal/syntax.Parse(string) (*green.GreenNode, green.LineIndex, []model.Diagnostic)`, `internal/syntax.Root`, `internal/syntax.ProjectFromTree`, `internal/sema.AnalyzeFile(uri, tree)`.
- Produces: `func Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error)`. Mirrors `craft check`'s core (`cmd/craft/check.go:39-49`). Diagnostics are `parseDiags` then `semaDiags`; `SourceURI` left as produced (see D6); `error` is always `nil`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/craft/parse_test.go`. Use `testdata` at the repo's `testdata/corpus/99_mixed/dsl-vnext.craft` (multi-domain, vNext) and a syntactically broken snippet. Reference the module root via `runtime`/relative path helper as other tests do, or embed source strings inline.

```go
package craft_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/pkg/craft"
)

// repoFile resolves a path relative to the repo root (pkg/craft is two levels down).
func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

func TestParse_ValidMultiDomain(t *testing.T) {
	src := repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craft")
	doc, diags, err := craft.Parse("dsl-vnext.craft", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("nil doc")
	}
	if len(doc.UseCases) == 0 {
		t.Error("expected at least one use case")
	}
	if len(doc.Services) == 0 {
		t.Error("expected at least one service")
	}
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("unexpected error diagnostic: %s %s", d.Code, d.Message)
		}
	}
}

func TestParse_SyntaxError(t *testing.T) {
	// A block opened but never closed / garbage tokens => at least one parse diagnostic.
	src := []byte("use_case \"x\" {\n  when \n")
	doc, diags, err := craft.Parse("bad.craft", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("Parse must always return a non-nil doc")
	}
	if len(diags) == 0 {
		t.Error("expected at least one diagnostic for malformed input")
	}
}

func TestParse_DiagnosticsAreDataNotError(t *testing.T) {
	_, _, err := craft.Parse("bad.craft", []byte("services {"))
	if err != nil {
		t.Errorf("err slot must be nil for content diagnostics, got %v", err)
	}
}

func TestParse_MatchesCorpusGolden(t *testing.T) {
	// Parse's doc must equal the existing projection golden byte-for-byte,
	// proving Parse is a faithful wrapper of ProjectFromTree.
	src := repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craft")
	doc, _, _ := craft.Parse("dsl-vnext.craft", src)
	got := mustMarshalIndent(t, doc)
	want := strings.TrimRight(string(repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craftjson")), "\n")
	if strings.TrimRight(got, "\n") != want {
		t.Errorf("Parse doc != corpus golden:\n got: %s\nwant: %s", got, want)
	}
}
```

Add a `mustMarshalIndent` helper matching how the corpus goldens are encoded (check an existing corpus test, e.g. `internal/parser_diff/v2_test.go`, for the exact `json.MarshalIndent(doc, "", "  ")` settings and whether a trailing newline is present; mirror it exactly so the golden compare is valid).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/craft/ -run TestParse -v`
Expected: FAIL — `undefined: craft.Parse`.

- [ ] **Step 3: Implement `Parse`**

Create `pkg/craft/parse.go`:

```go
package craft

import (
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
)

// Parse parses a single Craft source file and returns its document model plus
// any diagnostics (syntax errors/warnings and single-file semantic checks).
//
// Diagnostics are returned as data, never via the error return: err is reserved
// for programmer-error/invariant conditions and is currently always nil. Parse
// performs no file I/O — the caller supplies src. The returned *CraftDoc is
// never nil, even for empty or badly broken input.
//
// SourceURI on the returned diagnostics is left as produced by the underlying
// passes (empty for single-file input); the caller already knows filename. Use
// ParseFiles when you need every diagnostic attributed to its file.
func Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error) {
	doc, diags := parseOne(filename, string(src))
	return doc, diags, nil
}

// parseOne runs the single-file pipeline exactly as `craft check` does:
// syntax.Parse -> Root -> ProjectFromTree, then sema.AnalyzeFile for
// single-file semantic diagnostics. LineIndex is intentionally NOT forwarded to
// AnalyzeFile, matching the current CLI (see plan D3).
func parseOne(filename, src string) (*CraftDoc, []Diagnostic) {
	uri := "file://" + filename
	greenRoot, li, parseDiags := syntax.Parse(src)
	tree := syntax.Root(greenRoot)
	doc := syntax.ProjectFromTree(tree, li)

	_, semaDiags := sema.AnalyzeFile(uri, tree)

	diags := make([]Diagnostic, 0, len(parseDiags)+len(semaDiags))
	diags = append(diags, parseDiags...)
	diags = append(diags, semaDiags...)
	return doc, diags
}
```

(`syntax.ProjectFromTree` now returns `*model.CraftDoc`, which is identical to `*CraftDoc` via the alias, so it satisfies the return type directly. Same for `[]model.Diagnostic` vs `[]Diagnostic`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/craft/ -run TestParse -v`
Expected: PASS. If `TestParse_MatchesCorpusGolden` fails on formatting, align `mustMarshalIndent` with the corpus encoder settings (not the production code).

- [ ] **Step 5: Commit**

```bash
git add pkg/craft/parse.go pkg/craft/parse_test.go
git commit -m "feat(pkg/craft): add Parse for single-file in-process parsing"
```

---

## Task 4: pkg/craft.ParseFiles (multi-file merge + workspace analysis)

**Files:**
- Modify: `pkg/craft/parse.go` (add `ParseFiles` + helpers)
- Modify: `pkg/craft/parse_test.go` (add multi-file tests)

**Interfaces:**
- Consumes: same as Task 3 plus `sema.MergeWorkspaceSymbols(map[string]sema.Symbols)`, `sema.AnalyzeWorkspace(perFile, ws)`, `sema.LintWorkspace(map[string]syntax.SyntaxNode, ws)`, and `sema.Symbols` / `syntax.SyntaxNode` types.
- Produces: `func ParseFiles(files map[string][]byte) (*CraftDoc, []Diagnostic, error)`. Merges all `CraftDoc` slices in ascending key order (D5); runs the workspace pipeline (D4); every diagnostic's `SourceURI` = its map key (D6). `error` always nil.

- [ ] **Step 1: Write the failing tests**

Append to `pkg/craft/parse_test.go`:

```go
func TestParseFiles_MergeMultiDomain(t *testing.T) {
	files := map[string][]byte{
		"a.craft": []byte("domain Billing {\n  Invoicing\n}\n"),
		"b.craft": []byte("domain Catalog {\n  Products\n}\n"),
	}
	doc, _, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Domains) != 2 {
		t.Fatalf("expected 2 merged domains, got %d", len(doc.Domains))
	}
	// D5: deterministic ascending-key order => a.craft (Billing) before b.craft (Catalog).
	if doc.Domains[0].Name != "Billing" || doc.Domains[1].Name != "Catalog" {
		t.Errorf("merge order not sorted by filename: %+v", doc.Domains)
	}
}

func TestParseFiles_DeterministicRegardlessOfMapOrder(t *testing.T) {
	files := map[string][]byte{
		"z.craft": []byte("domain Zeta {\n  Z\n}\n"),
		"a.craft": []byte("domain Alpha {\n  A\n}\n"),
	}
	var first string
	for i := 0; i < 5; i++ {
		doc, _, _ := craft.ParseFiles(files)
		got := doc.Domains[0].Name
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("non-deterministic merge order: %q vs %q", first, got)
		}
	}
	if first != "Alpha" {
		t.Errorf("expected Alpha first (sorted), got %q", first)
	}
}

func TestParseFiles_DiagnosticsCarrySourceURI(t *testing.T) {
	files := map[string][]byte{
		"broken.craft": []byte("use_case \"x\" {\n  when\n"),
	}
	_, diags, _ := craft.ParseFiles(files)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	for _, d := range diags {
		if d.SourceURI != "broken.craft" {
			t.Errorf("SourceURI = %q, want bare map key %q (no file:// prefix)", d.SourceURI, "broken.craft")
		}
	}
}

func TestParseFiles_Empty(t *testing.T) {
	doc, diags, err := craft.ParseFiles(map[string][]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("nil doc")
	}
	if diags == nil {
		t.Error("diags should be non-nil empty slice, not nil")
	}
}
```

If a cross-file unresolved-reference fixture is available in the sema tests (see `internal/sema/validate_test.go:TestValidate_UnresolvedRefLocal_Warning`), add a `TestParseFiles_CrossFileUnresolvedRef` that reproduces its two-file setup and asserts the `craft/sema/unresolved-ref-local` (or equivalent) warning appears with the correct `SourceURI`. Read that test first to copy the exact DSL that triggers it; do not invent DSL that may not trigger the rule.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/craft/ -run TestParseFiles -v`
Expected: FAIL — `undefined: craft.ParseFiles`.

- [ ] **Step 3: Implement `ParseFiles` + helpers**

Add to `pkg/craft/parse.go` (extend the import block with `"sort"` and `"strings"`):

```go
// ParseFiles parses and merges multiple Craft source files into one document
// model, mirroring `craft inspect` (merge) plus `craft validate` (full
// workspace analysis: cross-file resolution and lint). Files are processed in
// ascending map-key order, so the merged model and the diagnostic slice are
// deterministic regardless of Go map iteration order.
//
// Every returned diagnostic's SourceURI is set to the originating map key (the
// bare filename you supplied — no file:// prefix). Diagnostics are data, not
// errors; err is currently always nil. Performs no file I/O.
func ParseFiles(files map[string][]byte) (*CraftDoc, []Diagnostic, error) {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	merged := &CraftDoc{}
	diags := []Diagnostic{}
	perFileTrees := make(map[string]syntax.SyntaxNode, len(files))
	perFileSyms := make(map[string]sema.Symbols, len(files))

	for _, key := range keys {
		uri := "file://" + key
		greenRoot, li, parseDiags := syntax.Parse(string(files[key]))
		tree := syntax.Root(greenRoot)
		perFileTrees[uri] = tree

		doc := syntax.ProjectFromTree(tree, li)
		mergeDoc(merged, doc)

		diags = append(diags, stampURI(parseDiags, key)...)

		syms, semaDiags := sema.AnalyzeFile(uri, tree)
		perFileSyms[uri] = syms
		diags = append(diags, stampURI(semaDiags, key)...)
	}

	if len(perFileSyms) > 0 {
		ws, wsDiags := sema.MergeWorkspaceSymbols(perFileSyms)
		diags = append(diags, remapURIs(wsDiags)...)

		_, resDiags := sema.AnalyzeWorkspace(perFileSyms, ws)
		diags = append(diags, remapURIs(resDiags)...)

		diags = append(diags, remapURIs(sema.LintWorkspace(perFileTrees, ws))...)
	}

	return merged, diags, nil
}

// mergeDoc appends every slice field of src onto dst. All CraftDoc slices are
// merged (not just the four `inspect` reads) so the library returns a complete
// model.
func mergeDoc(dst, src *CraftDoc) {
	dst.Architectures = append(dst.Architectures, src.Architectures...)
	dst.Exposures = append(dst.Exposures, src.Exposures...)
	dst.Services = append(dst.Services, src.Services...)
	dst.UseCases = append(dst.UseCases, src.UseCases...)
	dst.Domains = append(dst.Domains, src.Domains...)
	dst.Actors = append(dst.Actors, src.Actors...)
	dst.ContextMap = append(dst.ContextMap, src.ContextMap...)
}

// stampURI sets SourceURI = key on a copy-safe slice (the passes return fresh
// slices per file, so in-place mutation is fine).
func stampURI(diags []Diagnostic, key string) []Diagnostic {
	for i := range diags {
		diags[i].SourceURI = key
	}
	return diags
}

// remapURIs converts the internal "file://"+key SourceURI set by the workspace
// passes back to the bare map key the caller supplied.
func remapURIs(diags []Diagnostic) []Diagnostic {
	for i := range diags {
		diags[i].SourceURI = strings.TrimPrefix(diags[i].SourceURI, "file://")
	}
	return diags
}
```

Confirm the exact signatures of `MergeWorkspaceSymbols`, `AnalyzeWorkspace`, `LintWorkspace`, and the `sema.Symbols` / `syntax.SyntaxNode` type names against the source before compiling (they are: `MergeWorkspaceSymbols(map[string]Symbols) (WorkspaceSymbols, []Diagnostic)`, `AnalyzeWorkspace(map[string]Symbols, WorkspaceSymbols) (ResolutionMap, []Diagnostic)`, `LintWorkspace(map[string]syntax.SyntaxNode, WorkspaceSymbols, ...map[string]green.LineIndex) []Diagnostic`). Do not pass the optional `perFileLineIndices` (D3).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/craft/ -v`
Expected: PASS (all Parse + ParseFiles tests).

- [ ] **Step 5: Commit**

```bash
git add pkg/craft/parse.go pkg/craft/parse_test.go
git commit -m "feat(pkg/craft): add ParseFiles for multi-file merge + workspace analysis"
```

---

## Task 5: Refactor `check` to use craft.Parse

**Files:**
- Modify: `cmd/craft/check.go`
- Create: `cmd/craft/check_test.go`

**Interfaces:**
- Consumes: `craft.Parse`. `buildSymbolsJSON` changes signature to take `*craft.CraftDoc`.

- [ ] **Step 1: Write the characterization test (against current, pre-refactor code)**

Create `cmd/craft/check_test.go`. Capture output via `cmd.SetOut` after the refactor switches to `cmd.OutOrStdout()`. Since this test must pass BEFORE the refactor too, and the current code writes to `os.Stdout`, first assert against a golden captured from the current binary. Simplest robust approach: assert the `check` default output equals `craft.Parse`'s doc JSON, and `--lsp-json` shape is stable.

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runCheck(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"check"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("check %v: %v", args, err)
	}
	return buf.String()
}

func TestCheckCmd_DefaultEmitsDoc(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "x.craft")
	os.WriteFile(src, []byte("actor user Foo\n\nservices {\n  S {\n    contexts: Ctx\n  }\n}\n"), 0644)

	out := runCheck(t, src)
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if _, ok := doc["services"]; !ok {
		t.Errorf("expected services in doc output: %s", out)
	}
}

func TestCheckCmd_LSPJSONShape(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "x.craft")
	os.WriteFile(src, []byte("actor user Foo\n"), 0644)

	out := runCheck(t, src, "--lsp-json")
	var env struct {
		CraftDoc    json.RawMessage `json:"craftDoc"`
		Diagnostics json.RawMessage `json:"diagnostics"`
		Symbols     []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			Type string `json:"type"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("lsp-json not valid: %v\n%s", err, out)
	}
	if len(env.Symbols) != 1 || env.Symbols[0].Name != "Foo" || env.Symbols[0].Kind != "actor" || env.Symbols[0].Type != "user" {
		t.Errorf("symbols mismatch: %+v", env.Symbols)
	}
}
```

- [ ] **Step 2: Run against current code**

Run: `go test ./cmd/craft/ -run TestCheckCmd -v`
Expected: `TestCheckCmd_DefaultEmitsDoc` and `TestCheckCmd_LSPJSONShape` — this requires `newRootCmd()` output routing. If the current commands write to `os.Stdout` (not `cmd.OutOrStdout()`), these will fail to capture; that is expected — proceed to Step 3, which fixes routing, then both pass. (This is the one case where the characterization test and the minimal refactor land together.)

- [ ] **Step 3: Refactor `check.go`**

Rewrite the `RunE` and `buildSymbolsJSON`. Remove the `internal/sema` and `internal/syntax` imports.

```go
func checkCmd() *cobra.Command {
	var lspJSON bool

	cmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Parse a .craft file and emit CraftDoc JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", args[0], err)
			}

			doc, allDiags, err := craft.Parse(args[0], content)
			if err != nil {
				return err
			}
			if allDiags == nil {
				allDiags = []craft.Diagnostic{}
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			if lspJSON {
				diagBytes, _ := json.Marshal(allDiags)
				symbols := buildSymbolsJSON(doc)
				symBytes, _ := json.Marshal(symbols)
				return enc.Encode(lspJSONOutput{
					CraftDoc:    doc,
					Diagnostics: diagBytes,
					Symbols:     symBytes,
				})
			}
			return enc.Encode(doc)
		},
	}
	cmd.Flags().BoolVar(&lspJSON, "lsp-json", false, "emit diagnostics+symbols+craftDoc as JSON (mirrors LSP responses)")
	return cmd
}

// buildSymbolsJSON derives the --lsp-json actor symbol list from the parsed
// document (Line is 0, matching prior behavior). No syntax tree needed.
func buildSymbolsJSON(doc *craft.CraftDoc) []symbolInfo {
	var out []symbolInfo
	for _, a := range doc.Actors {
		out = append(out, symbolInfo{
			Name: a.Name,
			Kind: "actor",
			Line: 0,
			Type: string(a.Type),
		})
	}
	return out
}
```

Keep the `lspJSONOutput` and `symbolInfo` type declarations unchanged.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/craft/ -run TestCheckCmd -v && go test ./cmd/craft/ 2>&1 | tail -3`
Expected: PASS (new check tests + existing generate tests).

- [ ] **Step 5: Commit**

```bash
git add cmd/craft/check.go cmd/craft/check_test.go
git commit -m "refactor(cmd): check now uses pkg/craft.Parse"
```

---

## Task 6: Refactor `inspect` to use craft.ParseFiles

**Files:**
- Modify: `cmd/craft/inspect.go`
- Create: `cmd/craft/inspect_test.go`

**Interfaces:**
- Consumes: `craft.ParseFiles`. `buildInspectOutput` and `printInspectText` unchanged.

- [ ] **Step 1: Write the characterization test**

Create `cmd/craft/inspect_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runInspect(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"inspect"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("inspect %v: %v", args, err)
	}
	return buf.String()
}

func TestInspectCmd_JSONMerge(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.craft")
	b := filepath.Join(tmp, "b.craft")
	os.WriteFile(a, []byte("domain Billing {\n  Invoicing\n}\n"), 0644)
	os.WriteFile(b, []byte("domain Catalog {\n  Products\n}\n"), 0644)

	// pass in sorted order so output is order-stable (see plan D5)
	out := runInspect(t, "--format", "json", a, b)
	var got struct {
		Domains []struct{ Name string } `json:"domains"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d: %s", len(got.Domains), out)
	}
}
```

- [ ] **Step 2: Run to verify it drives the refactor**

Run: `go test ./cmd/craft/ -run TestInspectCmd -v`
Expected: FAIL to capture until Step 3 routes output to `cmd.OutOrStdout()`.

- [ ] **Step 3: Refactor `inspect.go`**

Replace the parse/merge loop with `craft.ParseFiles`; remove the `internal/syntax` import; write to `cmd.OutOrStdout()`.

```go
RunE: func(cmd *cobra.Command, args []string) error {
	files, err := resolveFiles(args)
	if err != nil {
		return err
	}

	contents := make(map[string][]byte, len(files))
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		contents[file] = b
	}

	merged, _, err := craft.ParseFiles(contents)
	if err != nil {
		return err
	}

	out := buildInspectOutput(files, merged)

	switch format {
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	default:
		return printInspectText(out)
	}
},
```

`printInspectText` still uses `fmt.Printf` to real stdout; leave it (text mode is not under test and unchanged). Only the JSON branch needs `cmd.OutOrStdout()`.

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/craft/ -run TestInspectCmd -v && go test ./cmd/craft/ 2>&1 | tail -3`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/craft/inspect.go cmd/craft/inspect_test.go
git commit -m "refactor(cmd): inspect now uses pkg/craft.ParseFiles"
```

---

## Task 7: Refactor `validate` to use craft.ParseFiles

**Files:**
- Modify: `cmd/craft/validate.go`
- Create: `cmd/craft/validate_test.go`

**Interfaces:**
- Consumes: `craft.ParseFiles`. Extracts `runValidate(map[string][]byte, bool) []validateResult` for testability (no `os.Exit`). Removes `internal/syntax` + `internal/sema` imports.

- [ ] **Step 1: Write the failing test**

Create `cmd/craft/validate_test.go`, testing the extracted `runValidate` directly (the cobra `RunE` calls `os.Exit`, which cannot run under `go test`):

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunValidate_ReportsDiagnostics(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.craft")
	// deprecated quoted event ref => a warning (craft/lint/deprecated-string-ref)
	src := []byte("use_case \"U\" {\n  when Billing listens \"SomethingHappened\"\n    Billing notifies \"Done\"\n}\n")
	os.WriteFile(f, src, 0644)

	b, _ := os.ReadFile(f)
	results := runValidate(map[string][]byte{f: b}, false)

	if len(results) == 0 {
		t.Fatal("expected at least one diagnostic result")
	}
	for _, r := range results {
		if r.File != f {
			t.Errorf("File = %q, want bare path %q (no file:// prefix)", r.File, f)
		}
	}
}
```

If the exact DSL above does not produce a diagnostic in this codebase, replace it with a snippet known to warn/error — confirm by reading `internal/sema/validate_test.go` / `lint_test.go` for a minimal triggering input. The assertion that matters: results are produced and `r.File` is the bare path.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/craft/ -run TestRunValidate -v`
Expected: FAIL — `undefined: runValidate`.

- [ ] **Step 3: Refactor `validate.go`**

Extract `runValidate` and make `RunE` a thin wrapper. Read files in `RunE` (I/O stays in the CLI); read errors become error results there.

```go
// runValidate parses the given file contents through pkg/craft.ParseFiles and
// maps diagnostics to validateResult. No I/O, no os.Exit — testable.
func runValidate(contents map[string][]byte, _ bool) []validateResult {
	_, diags, _ := craft.ParseFiles(contents)
	results := make([]validateResult, 0, len(diags))
	for _, d := range diags {
		results = append(results, validateResult{
			File:     d.SourceURI, // bare filename (plan D6)
			Line:     d.Range.Start.Line + 1,
			Severity: string(d.Severity),
			Message:  d.Message,
		})
	}
	return results
}

func validateCmd() *cobra.Command {
	var strict bool
	var format string

	cmd := &cobra.Command{
		Use:   "validate [files...]",
		Short: "Parse and lint .craft files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveFiles(args)
			if err != nil {
				return err
			}

			contents := make(map[string][]byte, len(files))
			var results []validateResult
			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					results = append(results, validateResult{
						File:     file,
						Severity: "error",
						Message:  err.Error(),
					})
					continue
				}
				contents[file] = content
			}

			results = append(results, runValidate(contents, strict)...)

			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.Encode(results)
			default:
				for _, r := range results {
					if r.Line > 0 {
						fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", r.File, r.Line, r.Severity, r.Message)
					} else {
						fmt.Fprintf(os.Stderr, "%s: %s: %s\n", r.File, r.Severity, r.Message)
					}
				}
			}

			hasFailure := false
			for _, r := range results {
				if r.Severity == string(craft.SeverityError) || (strict && r.Severity == string(craft.SeverityWarning)) {
					hasFailure = true
					break
				}
			}
			if hasFailure {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "promote warnings to errors")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text|json")
	return cmd
}
```

Note `runValidate`'s `strict` param is unused (kept for signature clarity / future); the `_ bool` silences it. Remove now-unused `internal/syntax` and `internal/sema` imports.

- [ ] **Step 4: Run tests**

Run: `go test ./cmd/craft/ -run TestRunValidate -v && go test ./cmd/craft/ 2>&1 | tail -3`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/craft/validate.go cmd/craft/validate_test.go
git commit -m "refactor(cmd): validate now uses pkg/craft.ParseFiles"
```

---

## Task 8: Stability contract doc comment + CHANGELOG

**Files:**
- Modify: `pkg/craft/craftdoc.go` (package doc comment)
- Modify: `CHANGELOG.md`

Now that the API lands and the CLI exercises it, replace the neutral package comment with a real stability statement (spec item 4). The "Experimental: stabilizes at v0.1" wording was already removed in Task 2.

- [ ] **Step 1: Set the stability contract on the package**

Set the `pkg/craft/craftdoc.go` package comment (the single `package craft` doc comment for the package) to:

```go
// Package craft is the stable public Go API for the Craft DSL toolchain.
//
// It provides the canonical document model (CraftDoc and its component types),
// the Diagnostic type, and the Parse / ParseFiles entry points for parsing
// Craft source in-process. As of v2.10.0 this package is stable and follows
// semantic versioning: existing exported names, struct fields, and JSON tags
// will not change or be removed within a major version; additions are allowed
// in minor releases. Requires Go 1.25 or newer to import.
```

Ensure only ONE file carries the package doc comment (Go emits a vet warning for duplicate package comments); leave `diagnostic.go` with a plain `package craft` and no doc comment.

- [ ] **Step 2: Add the CHANGELOG entry**

Insert at the top of `CHANGELOG.md` (above `## [2.9.0]`):

```markdown
## [2.10.0] — 2026-07-14

### Added
- **Importable parse API in `pkg/craft`.** External Go modules can now `import "github.com/tcarcao/craft/pkg/craft"` and parse Craft source in-process instead of shelling out to the CLI:
  - `craft.Parse(filename string, src []byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — single file.
  - `craft.ParseFiles(files map[string][]byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — multi-file merge plus cross-file resolution and lint. Files are processed in ascending filename order for deterministic output; each diagnostic's `SourceURI` is the map key you supplied.
  - Diagnostics are returned as data; the `error` slot is reserved for programmer errors. Neither function does file I/O.
- `LICENSE` file (MIT), matching the README's stated license.
- `pkg/craft` now declares a real stability contract (semver, stable as of 2.10.0). Requires Go 1.25+ to import.

### Changed
- `pkg/craft` is no longer marked "Experimental". The `CraftDoc`/`Diagnostic` type *definitions* moved to an internal leaf package (`internal/model`) and are re-exported from `pkg/craft` as type aliases — identity and JSON tags are unchanged, so no existing output or importer is affected.
- `cmd/craft`'s `check`, `inspect`, and `validate` are now thin wrappers over `pkg/craft.Parse`/`ParseFiles` rather than duplicating the parse+sema orchestration, guaranteeing the CLI and library cannot drift.
- `validate`'s workspace-level diagnostic lines now print the bare filename (consistent with per-file lines) instead of a `file://`-prefixed path.

### Notes
- No changes to the Craft language, grammar, or semantics. Diagnostic positions are byte-identical to v2.9.0.
```

- [ ] **Step 3: Verify build + vet + full suite**

Run: `go vet ./... && go build ./... && go test ./... 2>&1 | tail -10`
Expected: PASS, no vet warnings.

- [ ] **Step 4: Commit**

```bash
git add pkg/craft/craftdoc.go pkg/craft/diagnostic.go CHANGELOG.md
git commit -m "docs(pkg/craft): declare stable API contract; changelog for 2.10.0"
```

---

## Task 9: Release v2.10.0 (gated on explicit user confirmation)

**Files:** none (tagging only).

> This task performs an outward action (pushing a tag that triggers goreleaser + Docker + homebrew). Do NOT run it without explicit user go-ahead, per the established release flow. Present the diff summary and ask first.

- [ ] **Step 1: Final whole-branch review + green suite** (`go test ./...`).
- [ ] **Step 2:** Confirm with the user, then merge the branch to `main` (finishing-a-development-branch skill).
- [ ] **Step 3:** After merge, tag and push:
```bash
git tag -a v2.10.0 -m "v2.10.0 — importable pkg/craft parse API"
git push origin main
git push origin v2.10.0
```
- [ ] **Step 4:** Confirm the Release + Publish workflows triggered (`gh run list`).

---

## Self-Review

- **Spec coverage:** Goal 1 (Parse/ParseFiles) → Tasks 3–4. Goal 2 (CLI wraps API) → Tasks 5–7. Goal 3 (LICENSE) → Task 1. Goal 4 (drop Experimental, real contract) → Tasks 2 + 8. Goal 5 (preserve fields/tags) → Global Constraints + Task 2 (aliases). Testing requirements → Tasks 3,4 (unit: valid/syntax/semantic/multi-file) + 5,6,7 (characterization) + Task 3 golden tie-in. Release → Task 9. ✓
- **Import-cycle blocker** (not in the original spec) → resolved by Task 2 (Option D, user-approved). ✓
- **Type consistency:** `Parse(filename string, src []byte) (*CraftDoc, []Diagnostic, error)` and `ParseFiles(map[string][]byte) (*CraftDoc, []Diagnostic, error)` used identically in tasks and CLI call sites. `buildSymbolsJSON(*craft.CraftDoc)` signature consistent between Task 5 def and use. ✓
- **Placeholders:** none — all steps carry real code. Two spots flagged for in-task verification against existing tests (corpus encoder settings in Task 3; the exact diagnostic-triggering DSL in Tasks 4 & 7) — these are "confirm against source," not placeholders.
