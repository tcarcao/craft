# UseCase Identity + Tags + Edge-Verb Hardening — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add `Line`/`SourceURI` identity to `UseCase`, a generic `tags {}` sub-block on `use_case`, harden the sema edge-verb switch, and mirror the `tags` grammar in tree-sitter — closing the last regex-parsed gaps in the `svc-re-go-arch-context` consumer.

**Architecture:** Additive changes to the Go source-of-truth (`internal/syntax` + `internal/sema` + `internal/model` + `pkg/craft`) plus a mirrored grammar rule in the separate `tree-sitter-craft` repo. Craft ships as **v2.11.0** (minor, additive).

**Tech Stack:** Go 1.25.0; tree-sitter CLI v0.25.10 (in `tree-sitter-craft/node_modules/.bin`).

## Repos & baselines
- **craft** (`/Users/tiago.carcao/projects/poc/craft-project/craft`, module `github.com/tcarcao/craft/v2`): on `main` at the v2.10.1 merge. Slices A, B, C, changelog/release.
- **tree-sitter-craft** (`/Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft`): on `main`, **clean baseline HEAD `a883992`**, 12/12 corpus tests pass, tree-sitter v0.25.10 available via `node_modules/.bin`. Slice D.
- **craft-vscode-extension** (`/Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension`): only if Slice D finds a highlight config there that needs a `tags` entry (investigate; may be driven entirely by tree-sitter queries).

## Global Constraints
- Everything additive. Preserve all existing `model` struct fields + JSON tags exactly (keyed literals; `omitempty` on new fields). Per `pkg/craft`'s stability contract this is a **minor** bump (v2.11.0).
- Two grammars stay in sync: Go `internal/syntax`/`internal/sema` (source of truth) and `tree-sitter-craft/grammar.js`. Same rule as the vNext spec.
- **Contextual keywords, not reserved words:** `tags` (and any tag key) must remain usable as ordinary identifiers everywhere else — recognized by `tok.Value == "tags"`, exactly as `when` is (`internal/syntax/parser.go:751`).
- Lossless green tree: `tags {}` block text round-trips byte-for-byte.
- `commit.gpgsign` is already `false` locally in both repos.
- Do NOT touch anything under `.worktrees/`.
- **Out of scope (do NOT add):** DDD relationship-pattern verbs in `context_map` (`customer_supplier`, etc.) — see spec §4. `bc-relationships.craft` stays a consumer-side extension.

## Design decisions (baked in; rationale for reviewers)
- **DD1 — `ProjectFromTree` gains a VARIADIC `sourceURI ...string`, not a required param.** The spec assumed 2 callers; there are ~19 (5 production + ~14 test: `cmd/craft/generate.go:101`, `cmd/server/main.go:424`, `internal/processor/processor.go:31`, `pkg/craft/parse.go:39,76`, and many `internal/syntax`/`internal/visualizer` tests). A required param forces 19 edits that all pass `""`. Variadic matches the codebase's own `sema.AnalyzeFile(uri, tree, li ...green.LineIndex)` precedent, so only `parseOne` and `ParseFiles` change; the other 17 callers compile unchanged and get `SourceURI == ""`.
- **DD2 — Corpus goldens gain ONLY `line`, never `sourceUri`.** `TestHarnessA_V2vsGoldens` (`internal/parser_diff/v2_test.go:79`) calls `ProjectFromTree(tree, li)` (no URI → `SourceURI==""`) and structurally compares against the SAME `99_mixed` golden that `TestParse_MatchesCorpusGolden` (`pkg/craft/parse_test.go:82`) currently produces with `Parse("dsl-vnext.craft", src)`. To avoid the two harnesses disagreeing, change `TestParse_MatchesCorpusGolden` to `Parse("", src)` so 99_mixed also yields no `sourceUri`. `SourceURI` behavior is covered by dedicated unit tests, not corpus goldens. This matches spec §6 ("`sourceUri` [only] for ParseFiles-produced goldens"). Portable — no absolute paths ever enter goldens.
- **DD3 — `UseCase.Tags` is `map[string]string`** (spec Decision 3, the crux). `json.Marshal` sorts map keys, so goldens are deterministic. Built by iterating tag-stmt nodes, last-write-wins.
- **DD4 — Duplicate tag key / second `tags` block → WARNING** `craft/sema/duplicate-tag` (`model.SeverityWarning`), last-write-wins in the projection. Mirrors the `duplicate-service-anchor` detection shape but Warning severity (precedent: `craft/lint/deprecated-string-ref` at `validate.go:230`).
- **DD5 — Extract a testable `classifyEdgeVerb` helper** so Slice C's `default:` branch (unreachable from any `.craft` source because `isEdgeKeyword` gates it) can be unit-tested by direct call.
- **DD6 — Single source of truth for edge verbs in `syntax`:** refactor `isEdgeKeyword` to read a package-level `edgeKeywords` slice and expose `syntax.EdgeKeywords() []string`; the sync test compares that against sema's two maps.

## File structure
- Modify: `internal/model/model.go` (UseCase: +Line, +SourceURI, +Tags)
- Modify: `internal/syntax/projection.go` (variadic sourceURI; wire Line/SourceURI/Tags)
- Modify: `pkg/craft/parse.go` (parseOne/ParseFiles pass filename/key)
- Modify: `pkg/craft/parse_test.go` (SourceURI/Line unit tests; 99_mixed → `Parse("", …)`)
- Modify: `internal/syntax/kind.go` (SyntaxKindTagsBlock, SyntaxKindTagStmt)
- Modify: `internal/syntax/parser.go` (parseUseCaseBlock `tags` branch; parseUseCaseTagsBlock; edgeKeywords refactor + EdgeKeywords())
- Modify: `internal/syntax/ast.go` (typed views for the tags block/stmt nodes)
- Modify: `internal/sema/validate.go` (classifyEdgeVerb + default case; duplicate-tag)
- Modify: `internal/sema/validate_test.go` (default-case test; sync test)
- Add: `testdata/corpus/**/tags_*.craft` + `.craftjson`; `testdata/broken/tag_*.{craft,diagnostics.json}`
- Modify: 17 existing `testdata/corpus/**/*.craftjson` (regenerated: +`line` on use cases)
- Modify: `CHANGELOG.md`
- tree-sitter-craft: `grammar.js`, `queries/highlights.scm`, `test/corpus/*`, regenerated `src/*`

---

## Task 1: Slice C — edge-verb sema hardening

Independent of A/B; smallest; do first. Two changes: a defensive `default:` case (runtime) and a vocabulary-sync test (build-time).

**Files:** `internal/syntax/parser.go`, `internal/sema/validate.go`, `internal/sema/validate_test.go`

**Interfaces produced:** `syntax.EdgeKeywords() []string`.

- [ ] **Step 1: Failing tests first**

In `internal/sema/validate_test.go` add both tests. (`reflect` may need importing.)

```go
func TestClassifyEdgeVerb_UnrecognisedVerb(t *testing.T) {
	// A verb outside both maps is unreachable from .craft source (isEdgeKeyword
	// gates it), so test the sema classifier directly.
	diags := classifyEdgeVerb("f.craft", "bogus_verb", "bc", "service", "bc:x", "service:y", 1, 0)
	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if diags[0].Code != "craft/sema/unrecognised-edge-verb" {
		t.Errorf("code = %q, want craft/sema/unrecognised-edge-verb", diags[0].Code)
	}
	if diags[0].Severity != model.SeverityError {
		t.Errorf("severity = %q, want error", diags[0].Severity)
	}
}

func TestEdgeVerbVocabulariesInSync(t *testing.T) {
	parser := map[string]bool{}
	for _, k := range syntax.EdgeKeywords() {
		parser[k] = true
	}
	sema := map[string]bool{}
	for k := range edgeRealizationVerbs {
		sema[k] = true
	}
	for k := range edgeTermVerbs {
		sema[k] = true
	}
	if !reflect.DeepEqual(parser, sema) {
		t.Fatalf("parser EdgeKeywords and sema edge-verb maps disagree:\n parser=%v\n sema=%v", parser, sema)
	}
}
```

Confirm `validate_test.go` imports `github.com/tcarcao/craft/v2/internal/syntax` and `.../internal/model`; add if missing.

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/sema/ -run 'TestClassifyEdgeVerb_UnrecognisedVerb|TestEdgeVerbVocabulariesInSync'`
Expected: FAIL — `undefined: classifyEdgeVerb`, `undefined: syntax.EdgeKeywords`.

- [ ] **Step 3: syntax — single source of truth + accessor**

In `internal/syntax/parser.go`, replace the `isEdgeKeyword` switch (`:1899-1905`) with a slice-backed version and add the accessor:

```go
// edgeKeywords is the single source of truth for context_map edge verbs.
var edgeKeywords = []string{"realized_by", "also_realizes", "same_as", "contrasts", "distinct_from"}

func isEdgeKeyword(s string) bool {
	for _, k := range edgeKeywords {
		if k == s {
			return true
		}
	}
	return false
}

// EdgeKeywords returns the context_map edge verbs the parser accepts. Exported so
// sema (and future docs/tooling) can verify its own verb classification stays in
// sync instead of hand-copying the list.
func EdgeKeywords() []string {
	return append([]string(nil), edgeKeywords...)
}
```

- [ ] **Step 4: sema — extract classifier + add default case**

In `internal/sema/validate.go`, extract the switch from `validateContextMapEdges` (`:270-279`) into a helper, and replace the inline switch with a call:

```go
// classifyEdgeVerb returns endpoint-kind diagnostics for one edge. Extracted from
// the tree walk so the default branch (unreachable via .craft source because the
// parser's isEdgeKeyword gates it) is unit-testable.
func classifyEdgeVerb(uri, verb, leftKind, rightKind, left, right string, leftLine, leftCol int) []model.Diagnostic {
	switch {
	case edgeRealizationVerbs[verb]:
		if leftKind != "bc" || rightKind != "service" {
			return []model.Diagnostic{edgeEndpointKindDiag(uri, verb, "a `bc:` left endpoint and a `service:` right endpoint", left, right, leftLine, leftCol)}
		}
	case edgeTermVerbs[verb]:
		if leftKind != "term" || rightKind != "term" {
			return []model.Diagnostic{edgeEndpointKindDiag(uri, verb, "`term:` endpoints on both sides", left, right, leftLine, leftCol)}
		}
	default:
		startChar := colToLSP(leftCol)
		return []model.Diagnostic{{
			Code:      "craft/sema/unrecognised-edge-verb",
			Message:   fmt.Sprintf("edge verb %q has no endpoint-kind rule (internal: parser accepted it via isEdgeKeyword but sema doesn't classify it — parser/sema drift, not a user error)", verb),
			Severity:  model.SeverityError,
			SourceURI: uri,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
				End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + len(left)},
			},
		}}
	}
	return nil
}
```

Replace the inline switch in `validateContextMapEdges` with:
```go
			diags = append(diags, classifyEdgeVerb(uri, verb, leftKind, rightKind, left, right, leftLine, leftCol)...)
```
(`fmt`, `colToLSP`, `lineToLSP` are already available in `validate.go`.)

- [ ] **Step 5: Run — expect pass**

Run: `go test ./internal/sema/ ./internal/syntax/`
Expected: PASS (new tests + existing). Confirm the existing edge-endpoint-kind fixtures still pass (the refactor is behavior-preserving for the two known verb classes).

- [ ] **Step 6: Commit**
```bash
git add internal/syntax/parser.go internal/sema/validate.go internal/sema/validate_test.go
git commit -m "feat(sema): default case + parser/sema edge-verb sync test (spec Slice C)"
```

---

## Task 2: Slice A — UseCase Line + SourceURI (+ golden regen)

**Files:** `internal/model/model.go`, `internal/syntax/projection.go`, `pkg/craft/parse.go`, `pkg/craft/parse_test.go`, throwaway regen tool, 17 corpus `.craftjson`.

**Interfaces:** `ProjectFromTree(root SyntaxNode, li green.LineIndex, sourceURI ...string) *model.CraftDoc`.

- [ ] **Step 1: Failing unit tests**

Add to `pkg/craft/parse_test.go`:

```go
func TestParse_UseCaseLineAndSourceURI(t *testing.T) {
	src := []byte("use_case \"First\" {\n  when A does x\n}\n\nuse_case \"Second\" {\n  when A does y\n}\n")
	doc, _, _ := craft.Parse("journeys/renewal.craft", src)
	if len(doc.UseCases) != 2 {
		t.Fatalf("want 2 use cases, got %d", len(doc.UseCases))
	}
	if doc.UseCases[0].Line != 1 {
		t.Errorf("UseCases[0].Line = %d, want 1", doc.UseCases[0].Line)
	}
	if doc.UseCases[1].Line != 5 {
		t.Errorf("UseCases[1].Line = %d, want 5", doc.UseCases[1].Line)
	}
	for _, uc := range doc.UseCases {
		if uc.SourceURI != "journeys/renewal.craft" {
			t.Errorf("UseCase %q SourceURI = %q, want journeys/renewal.craft", uc.Name, uc.SourceURI)
		}
	}
}

func TestParseFiles_UseCaseSourceURIIsMapKey(t *testing.T) {
	files := map[string][]byte{
		"a.craft": []byte("use_case \"UA\" {\n  when A does x\n}\n"),
		"b.craft": []byte("use_case \"UB\" {\n  when A does y\n}\n"),
	}
	doc, _, _ := craft.ParseFiles(files)
	got := map[string]string{}
	for _, uc := range doc.UseCases {
		got[uc.Name] = uc.SourceURI
	}
	if got["UA"] != "a.craft" || got["UB"] != "b.craft" {
		t.Errorf("SourceURI attribution wrong: %+v", got)
	}
}
```

(Verify the trigger DSL `when A does x` parses without a fatal error in this codebase; if `does` isn't a valid verb form, use a known-good trigger like `when A creates thing` — check an existing corpus `use_case` fixture and mirror its trigger. The line-number assertions are what matter.)

- [ ] **Step 2: Run — expect failure** (`Line`/`SourceURI` undefined fields).
Run: `go test ./pkg/craft/ -run 'TestParse_UseCaseLineAndSourceURI|TestParseFiles_UseCaseSourceURIIsMapKey'` → FAIL to compile.

- [ ] **Step 3: Model**

`internal/model/model.go`, extend `UseCase`:
```go
type UseCase struct {
	Name      string     `json:"name"`
	Scenarios []Scenario `json:"scenarios"`
	// Line is the 1-based source line of the use_case's title token, 0 if unknown.
	Line int `json:"line,omitempty"`
	// SourceURI is the originating file (ParseFiles map key / Parse filename).
	// Empty when the caller supplied none.
	SourceURI string `json:"sourceUri,omitempty"`
}
```

- [ ] **Step 4: Projection — variadic sourceURI, wire Line + SourceURI**

`internal/syntax/projection.go`: change the signature and the use-case loop.
```go
func ProjectFromTree(root SyntaxNode, li green.LineIndex, sourceURI ...string) *model.CraftDoc {
	src := ""
	if len(sourceURI) > 0 {
		src = sourceURI[0]
	}
	...
	for _, uc := range file.UseCases() {
		if uc.Title() == nil {
			continue
		}
		ucLine, _ := li.LineCol(uc.Title().Offset())
		outUC := model.UseCase{
			Name:      uc.Name(),
			Scenarios: []model.Scenario{},
			Line:      ucLine,
			SourceURI: src,
		}
		...
```
The empty-root early return stays `&model.CraftDoc{UseCases: []model.UseCase{}}`.

- [ ] **Step 5: parse.go call sites**

`pkg/craft/parse.go` — `parseOne` passes `filename`; the `ParseFiles` per-file loop passes `key`:
```go
// parseOne
doc := syntax.ProjectFromTree(tree, li, filename)
// ParseFiles loop
doc := syntax.ProjectFromTree(tree, li, key)
```
No other call site changes (variadic — the other ~17 callers keep passing `(tree, li)` and get `SourceURI==""`).

- [ ] **Step 6: Fix the corpus-golden harness conflict (DD2)**

In `pkg/craft/parse_test.go`, `TestParse_MatchesCorpusGolden`: change `craft.Parse("dsl-vnext.craft", src)` to `craft.Parse("", src)` so the 99_mixed golden carries no `sourceUri` (matching `TestHarnessA`'s no-URI projection). Add a one-line comment citing DD2.

- [ ] **Step 7: Run unit tests — expect pass**
Run: `go test ./pkg/craft/ -run 'TestParse_UseCaseLineAndSourceURI|TestParseFiles_UseCaseSourceURIIsMapKey'` → PASS.

- [ ] **Step 8: Regenerate the corpus goldens (+`line` only)**

The 17 `.craftjson` files with non-empty `useCases` are now stale (projection emits `line`, goldens lack it). There is NO existing regen tool. Create a throwaway generator, run it over the whole corpus, and verify the diff is ONLY `line` additions.

Create `cmd/_regoldens/main.go` (temporary — deleted before commit):
```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func main() {
	root := "testdata/corpus"
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".craft") {
			return nil
		}
		src, _ := os.ReadFile(p)
		greenRoot, li, _ := syntax.Parse(string(src))
		tree := syntax.Root(greenRoot)
		doc := syntax.ProjectFromTree(tree, li) // no sourceURI — DD2
		b, _ := json.MarshalIndent(doc, "", "  ")
		out := strings.TrimSuffix(p, ".craft") + ".craftjson"
		os.WriteFile(out, append(b, '\n'), 0644)
		return nil
	})
}
```
Run: `go run ./cmd/_regoldens`
Then: `git diff --stat testdata/corpus` and `git diff testdata/corpus | grep -E '^[+-]' | grep -v '"line"' | grep -vE '^(\+\+\+|---)'`
Expected: the second command prints NOTHING (every changed line is a `"line":` addition). If it prints other changes, STOP and report — it means either a golden had drifted from the projection or the marshaling differs; investigate before proceeding. Spot-check 2-3 goldens by hand that the `line` values point at the actual `use_case` lines in the paired `.craft`.

Then delete the tool: `rm -rf cmd/_regoldens`.

- [ ] **Step 9: Full suite green**
Run: `go test ./...` (allow up to 5 min). Expected: PASS — `TestHarnessA_V2vsGoldens` and `TestParse_MatchesCorpusGolden` now agree with the regenerated goldens.

- [ ] **Step 10: Commit**
```bash
git add internal/model/model.go internal/syntax/projection.go pkg/craft/parse.go pkg/craft/parse_test.go testdata/corpus
git commit -m "feat(model): UseCase Line + SourceURI; regenerate corpus goldens (spec Slice A)"
```

---

## Task 3: Slice B — `tags {}` block (grammar + model + projection)

**Files:** `internal/syntax/kind.go`, `internal/syntax/parser.go`, `internal/syntax/ast.go`, `internal/syntax/projection.go`, `internal/model/model.go`, plus tests + valid corpus fixtures.

**Interfaces:** `UseCase.Tags map[string]string`; new `SyntaxKindTagsBlock`/`SyntaxKindTagStmt`; typed views `TagsBlock`/`TagStmt` (names per ast.go convention).

- [ ] **Step 1: Failing tests**

Add a projection/round-trip test in `internal/syntax` (mirror `projection_uc_test.go` style) and a `pkg/craft` test:

```go
// pkg/craft/parse_test.go
func TestParse_UseCaseTags(t *testing.T) {
	src := []byte("use_case \"Renewal\" {\n  tags {\n    journey: re/renewal-flow\n    owner: \"team billing\"\n  }\n\n  when A does x\n}\n")
	doc, diags, _ := craft.Parse("x.craft", src)
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Fatalf("unexpected error diag: %s %s", d.Code, d.Message)
		}
	}
	if len(doc.UseCases) != 1 {
		t.Fatalf("want 1 use case, got %d", len(doc.UseCases))
	}
	tags := doc.UseCases[0].Tags
	if tags["journey"] != "re/renewal-flow" {
		t.Errorf("tags[journey] = %q, want re/renewal-flow", tags["journey"])
	}
	if tags["owner"] != "team billing" {
		t.Errorf("tags[owner] = %q, want \"team billing\" (unquoted)", tags["owner"])
	}
}

func TestParse_NoTagsBlockLeavesTagsNil(t *testing.T) {
	doc, _, _ := craft.Parse("x.craft", []byte("use_case \"U\" {\n  when A does x\n}\n"))
	if doc.UseCases[0].Tags != nil {
		t.Errorf("Tags should be nil when no tags block, got %v", doc.UseCases[0].Tags)
	}
}
```

Add a round-trip assertion (mirror the existing round-trip test harness) that a `.craft` with a `tags {}` block reproduces byte-for-byte through the green tree.

- [ ] **Step 2: Run — expect failure.**

- [ ] **Step 3: SyntaxKinds**

`internal/syntax/kind.go` — append to the NODE iota block (after `SyntaxKindEdgeStmt`, `:115`):
```go
	SyntaxKindTagsBlock // tags { tag_stmt* } inside a use_case
	SyntaxKindTagStmt   // IDENT ':' (IDENT | STRING | ref), inside a tags block
```
(Node kinds are `iota + 1000`; keep them within the node block. No String()/name table exists — nothing else to update.)

- [ ] **Step 4: Parser**

In `parseUseCaseBlock` (`parser.go:747-758`), add a `tags` branch before the `else`:
```go
		} else if tok.Type == lexer.TokenIdent && tok.Value == "tags" {
			diags = append(diags, p.parseUseCaseTagsBlock()...)
		} else {
			diags = append(diags, p.diagUnexpected(tok, "`when`, `tags`, or `}`"))
			p.consumeAs(SyntaxKindError)
		}
```
(Update the existing `else`'s expectation string from ``"`when` or `}`"`` to ``"`when`, `tags`, or `}`"``.)

Add `parseUseCaseTagsBlock` and `parseTagStmt`, mirroring `parseContextMapBlock`/`parseEdgeStmt` (`parser.go:1748-1810`) exactly: `StartNode(SyntaxKindTagsBlock)`; consume the `tags` ident (as a contextual keyword token — `consumeAs` with an appropriate token kind, e.g. reuse the ident kind used for `when`/other contextual keywords — check how `when` is consumed in `parseScenario` and mirror it); consume `{`; loop `parseTagStmt` until `}` with the same forward-progress guard and unclosed-block diagnostic; consume `}`; `FinishNode()`.

`parseTagStmt`: `StartNode(SyntaxKindTagStmt)`; expect key `IDENT`; expect `:`; expect value = either a `TokenString` OR a bare ref-shaped value. **For the bare value, reuse the exact node-slug/ref value scanning** the parser already uses (the spec calls this "ref-shaped lexing … used for node slugs" — find it: it's how `parseEdgeStmt`/`parseRef` capture a value like `re/renewal-flow` spanning `/` and `-`; read that code and reuse the same helper so `journey: re/renewal-flow` captures the whole slug). `FinishNode()`. Emit a `craft/syntax/...` diagnostic (mirror `diagUnexpected`) if `:` or a value is missing.

- [ ] **Step 5: Typed views (ast.go)**

Add `TagsBlock`/`TagStmt` typed-view wrappers mirroring the context-map/edge views (`ast.go` — find `ContextMapDecl`/`EdgeDecl` views and copy their shape). Add `UseCaseDecl.TagsBlock() *TagsBlock` (returns the single tags-block child or nil), `TagsBlock.Tags() []TagStmt`, and `TagStmt.Key() *SyntaxToken` / `TagStmt.Value() *SyntaxToken` (value token, unquoted via the same `stringAwareText` path used for string values).

- [ ] **Step 6: Projection**

In `projection.go`'s use-case loop, after building `outUC`, project tags (last-write-wins). Mirror the `ComponentModifier` key/value pattern (`projection.go:465-478`) but into a map:
```go
if tb := uc.TagsBlock(); tb != nil {
	tags := map[string]string{}
	for _, ts := range tb.Tags() {
		keyTok, valTok := ts.Key(), ts.Value()
		if keyTok == nil || valTok == nil {
			continue
		}
		tags[keyTok.Text()] = stringAwareText(*valTok) // last-write-wins
	}
	if len(tags) > 0 {
		outUC.Tags = tags
	}
}
```
`Tags` stays `nil` when absent (DD3) — only assign when non-empty.

- [ ] **Step 7: Model**

`internal/model/model.go` — add to `UseCase`:
```go
	Tags map[string]string `json:"tags,omitempty"`
```

- [ ] **Step 8: Valid corpus fixtures**

Add `testdata/corpus/04_use_cases/tags_multi.craft` (+ `.craftjson`): a use case with `tags {}` (bare slug value + quoted value + a second key), and confirm the golden's `tags` object has sorted keys (json map marshaling). Regenerate its golden with the same throwaway approach as Task 2 Step 8 (or hand-write and let `TestHarnessA` validate). Also confirm an existing no-tags fixture still shows no `tags` key.

- [ ] **Step 9: Run tests + round-trip + full suite** → PASS.

- [ ] **Step 10: Commit**
```bash
git add internal/syntax internal/model pkg/craft testdata/corpus
git commit -m "feat(syntax): tags {} block on use_case → UseCase.Tags (spec Slice B)"
```

---

## Task 4: Slice B — duplicate-tag sema warning + broken fixtures

**Files:** `internal/sema/validate.go`, `internal/sema/validate_test.go`, `testdata/broken/tag_*`.

- [ ] **Step 1: Failing test**

`internal/sema/validate_test.go`:
```go
func TestValidate_DuplicateTag_Warning(t *testing.T) {
	src := "use_case \"U\" {\n  tags {\n    journey: a\n    journey: b\n  }\n  when A does x\n}\n"
	// parse → AnalyzeFile; expect a craft/sema/duplicate-tag WARNING for the 2nd journey
	// (mirror the existing TestValidate_* setup in this file: syntax.Parse → Root → AnalyzeFile)
	...
	if !hasDiag(diags, "craft/sema/duplicate-tag", model.SeverityWarning) {
		t.Fatalf("want craft/sema/duplicate-tag warning, got %+v", diags)
	}
}
```
(Copy the exact parse→AnalyzeFile harness from a neighboring `TestValidate_*` test in the same file.)

- [ ] **Step 2: Run — expect failure.**

- [ ] **Step 3: Implement**

Add `validateUseCaseTags` to `validate.go` (called from the same place `validateServiceAnchors`/`validateContextMapEdges` are invoked in `AnalyzeFile` — find that call site and add it). Detect (a) a duplicate key within one `tags` block, and (b) a second `tags` block in one use case; emit `craft/sema/duplicate-tag` (`model.SeverityWarning`) at each offending occurrence after the first. Mirror the `seenX bool` detection of `validateServiceAnchors` (`:309-322`) and the diagnostic shape of `duplicateAnchorDiag` (`:328-344`) but with `model.SeverityWarning` and message like `use case %q: tag %q is already set; last value wins`.

- [ ] **Step 4: Broken fixtures**

Add to `testdata/broken/` with paired `.diagnostics.json`:
- `tag_duplicate_key.craft` → `craft/sema/duplicate-tag` warning.
- `tag_missing_colon.craft` (`journey re/x` — no `:`) → a `craft/syntax/...` error.
- `tag_bad_value.craft` (value that is neither ident/ref nor string, e.g. `journey: { }`) → a `craft/syntax/...` error.
Follow the exact `.diagnostics.json` format of existing `testdata/broken/*` (assert on Code + Range + Severity, per the established harness).

- [ ] **Step 5: Run** `go test ./internal/sema/ ./internal/parser_diff/` → PASS.

- [ ] **Step 6: Commit**
```bash
git add internal/sema testdata/broken
git commit -m "feat(sema): craft/sema/duplicate-tag warning + broken tag fixtures (spec Slice B)"
```

---

## Task 5: CHANGELOG + version note

**Files:** `CHANGELOG.md`

- [ ] **Step 1: Add the 2.11.0 entry** at the top:
```markdown
## [2.11.0] — 2026-07-15

### Added
- **`UseCase` now carries `Line` and `SourceURI`** (`json:"line,omitempty"` / `"sourceUri,omitempty"`), parity with `Service`/`Actor`/`Action`. `Parse` stamps the filename you pass; `ParseFiles` stamps each use case's originating map key — so a consumer can attribute a use case to its file without re-scanning text.
- **Generic `tags {}` sub-block inside `use_case`** → `UseCase.Tags map[string]string`. An opaque, consumer-defined key/value slot (`tags { journey: re/renewal-flow  owner: "team billing" }`); craft neither validates keys nor interprets values. Values may be bare slugs or quoted strings. `Tags` is absent from JSON when no block is present.
- `syntax.EdgeKeywords()` accessor exposing the context_map edge verbs.

### Fixed / Hardened
- `internal/sema` edge-verb validation gained a defensive `default:` case (`craft/sema/unrecognised-edge-verb`) and a build-time test keeping the parser's edge-keyword set and sema's verb maps in sync.
- Duplicate tag key or a second `tags {}` block emits `craft/sema/duplicate-tag` (warning; last value wins).

### Notes
- Additive/minor: existing keyed struct literals and JSON consumers are unaffected. Corpus goldens gained `use_case` `line` fields (regenerated).
```

- [ ] **Step 2: Commit**
```bash
git add CHANGELOG.md
git commit -m "docs: changelog for 2.11.0 (usecase identity, tags, edge hardening)"
```

---

## Task 6: Slice D — tree-sitter grammar sync (+ VSCode highlight)

**Repo:** `tree-sitter-craft` (baseline `a883992`, tree-sitter v0.25.10 via `node_modules/.bin`). Work on a branch.

- [ ] **Step 1: Branch** `git -C <tree-sitter-craft> checkout -b feat/tags-block`.

- [ ] **Step 2: Grammar** — in `grammar.js`, extend `use_case_block` (`:315-323`) to allow an optional `tags_block` among its children, and add the rules:
```js
tags_block: $ => seq('tags', '{', repeat($.tag_stmt), '}'),
tag_stmt: $ => seq(field('key', $.identifier), ':', field('value', choice($.string, $._slug_value))),
```
Reuse whatever rule the grammar already uses for node-slug/ref values (`re/renewal-flow`) for `_slug_value` — inspect how `context_map` edges/refs are defined in `grammar.js` and mirror it; do NOT invent a new token if an equivalent exists. Keep `tags` a bare string literal in the seq (contextual — tree-sitter handles this without reserving the word, matching how `when`/`use_case` are literals here).

- [ ] **Step 3: Regenerate + test**
Run (from the repo): `node_modules/.bin/tree-sitter generate` then `npm test`.
Add a corpus test in `test/corpus/` for a `use_case` with a `tags {}` block (multiple keys, bare + quoted value). Expected: all corpus tests pass, including the new one.

- [ ] **Step 4: Highlighting** — add a `tags`/key highlight rule to `queries/highlights.scm` (mirror how `context_map`/other block keywords and field keys are highlighted). Then check `craft-vscode-extension`: determine whether its highlighting is driven by these tree-sitter queries (nothing to do there) or a separate TextMate grammar that needs a `tags` entry — if the latter, add the minimal keyword rule; if the former, note it in the report and make no vscode change.

- [ ] **Step 5: Commit (two commits per the baseline discipline if regen is large)**
```bash
git -C <tree-sitter-craft> add grammar.js queries test
git -C <tree-sitter-craft> commit -m "feat(tree-sitter): tags {} block on use_case + highlight + corpus test"
git -C <tree-sitter-craft> add src
git -C <tree-sitter-craft> commit -m "chore(tree-sitter): regenerate parser for tags {} rule"
```
Do NOT push. Report the branch + commit SHAs; the human decides merge/publish (tree-sitter-craft has its own release flow and `main` is already ahead of origin).

---

## Task 7: Release craft v2.11.0 (gated on explicit user confirmation)

Outward action — do NOT run without explicit go-ahead.
- [ ] Final whole-branch review; `go test ./...` green.
- [ ] Confirm with user → merge to `main`, tag `v2.11.0`, push main + tag; confirm Release/Docker workflows.

---

## Self-Review
- **Spec coverage:** Slice A → Task 2; Slice B → Tasks 3+4; Slice C → Task 1; Slice D → Task 6; testing (§6) distributed across tasks (corpus, broken, round-trip, sema, differential via tree-sitter corpus); changelog/release → Tasks 5+7. ✓
- **Grounding corrections folded in:** ProjectFromTree 19-caller reality (DD1 variadic); no golden-regen tool (Task 2 throwaway + diff-verify); two-harness SourceURI conflict (DD2); duplicate-tag Warning not Error (DD4); default-case untestable-from-source (DD5 helper extraction); cross-package sync test needs an accessor (DD6). ✓
- **Type consistency:** `ProjectFromTree(root, li, sourceURI ...string)` used identically in Task 2 Steps 4–5; `UseCase.Tags map[string]string` (DD3) consistent Tasks 3/4; `classifyEdgeVerb` signature consistent Task 1 Steps 1/4. ✓
- **Placeholders:** the value-lexing reuse (Task 3 Step 4) and the exact `.diagnostics.json`/round-trip harness (Tasks 3/4) point the implementer at named existing patterns to read rather than inlining code that must match an unseen format — deliberate, not a gap.
