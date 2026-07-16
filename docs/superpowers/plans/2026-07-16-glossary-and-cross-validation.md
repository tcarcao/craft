# Glossary Block + context_map Cross-Validation Lint — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ship two independent features that complete the DDD strategic-modeling surface: (A) a `glossary { }` block for cross-context term relations, and (B) a cross-validation lint that flags when a `context_map` relationship contradicts the inferred communication view.

**Architecture:** both mirror the shipped `context_map` machinery (v2.12.0). Glossary is a full parallel block (parser -> projection -> sema collect/resolve -> pkg/craft -> tree-sitter -> docs -> corpus). The cross-validation lint is a new `LintWorkspace` pass reusing `resolveBCRef` and the `dead-event` event-pairing idiom. The two groups are independent; Group A first (foundational, larger), then Group B.

**Tech Stack:** Go 1.23; hand-written lexer/parser producing a lossless green tree; `internal/syntax` (parse + project) -> `internal/model` (types) -> `internal/sema` (resolution + lint) -> `pkg/craft` (public API); tree-sitter-craft (separate repo).

**Specs:**
- `docs/superpowers/specs/2026-07-16-glossary-block-design.md`
- `docs/superpowers/specs/2026-07-16-context-map-cross-validation-lint-design.md`

## Global Constraints

- Byte-for-byte lossless round-trip of the green tree is mandatory; any task that changes a `.craft` regenerates its own golden.
- `model.Edge{Left,Verb,Right}` is unchanged. Glossary adds a new `model.TermRelation{Left,Verb,Right}`; do not overload `Edge`.
- Glossary verbs are `same_as`, `contrasts`, `distinct_from`, all symmetric (no direction/role metadata table). The verb set is single-sourced in `internal/syntax` and kept in sync with sema by a build-time test, exactly like `edgeKeywords`.
- Term node identity: `<bc>/<term>` or `<domain>/<bc>/<term>`. Last `/`-segment is the term; preceding segment(s) are the BC reference resolved via `resolveBCRef`. `.` stays events-only.
- No `internal/sema` -> `pkg/craft` import. No lint enable/disable config system (severity is the only noise lever).
- Diagnostic namespacing: resolution/shape under `craft/sema/*`, style/consistency lints under `craft/lint/*`.
- The cross-validation lint never warns on absence of communication (the communication view is partial).
- Commits use `git -c commit.gpgsign=false` (GPG signing fails in these repos).
- Release (Group C) is GATED on explicit user confirmation and coordinates the tree-sitter `CORPUS_VERSION` bump, exactly as v2.12.0 did.

**Reference interfaces (verified current):**
- `model.CraftDoc` (model.go:10) has `ContextMap []Edge` at line 17; add `Glossary []TermRelation` after it.
- Projection loop for context_map: `internal/syntax/projection.go:166-181` iterates `file.ContextMaps()` -> `cm.Edges()` -> `model.Edge`. Mirror for glossary.
- `parseContextMapBlock` (parser.go:1854), `ContextMapDecl` with `Domain()`/`Edges()` (ast.go:1974+), `SyntaxKindContextMapDecl`/`SyntaxKindContextMapDomain` (kind.go). Mirror for glossary.
- Sema resolution: `Symbols.ContextMapEdges` (sema.go:154), `collectContextMapEdgeSites` (validate.go:322), `resolveBCRef(ws, scopeDomain, ref) (resolved, kind string, ambiguous bool)` (validate.go:362), `relationshipEdgeDiags` (validate.go:420), `redundantRelationshipDiag` (validate.go:458), the AnalyzeWorkspace loop (sema.go:829-838). `WorkspaceSymbols{Domains, BoundedContexts, Services, Actors}`, `DomainSymbol{Name, BoundedContexts}`.
- `LintWorkspace(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, perFileLineIndices ...map[string]green.LineIndex)` (lint.go:23) already runs `lintDeadEvents`/`lintUnusedActors`/`lintEventPastTense`. Add the cross-validation lint here.
- Use-case AST walk: `syntax.AsFile(tree).UseCases()` -> `uc.Scenarios()` -> `sc.Trigger()` / `sc.Actions()`. `ActionDecl.Kind()` ("sync_action"/"async_action"/...), `.SubjectName()`, `.TargetName()`, `.EventValue()`, `.Line(li)`, `.SubjectCol(li)`, `.TargetCol(li)`. `TriggerDecl.Kind()` ("domain_listen"/...), `.ContextName()` (listener subject), `.EventValue()`.
- pkg/craft aliases in `pkg/craft/craftdoc.go` (`Edge = model.Edge` at line 15); edge API in `pkg/craft/edges.go`.

---

# GROUP A — `glossary { }` block

### Task A1: Parser + AST for the glossary block

**Files:**
- Modify: `internal/syntax/parser.go` (add `parseGlossaryBlock`, wire into top-level dispatch beside `parseContextMapBlock` at parser.go:79; add `glossaryVerbs` set + lexer keyword mapping), `internal/syntax/kind.go` (new `SyntaxKindKwGlossary`, `SyntaxKindGlossaryDecl`, `SyntaxKindGlossaryDomain`, `SyntaxKindGlossaryRelation`), `internal/syntax/ast.go` (`GlossaryDecl` view: `Domain()`, `Relations()`; `GlossaryRelationDecl` view: `Left()`, `Verb()`, `Right()`, `LeftRef()`, `RightRef()`), `internal/syntax/glossary_meta.go` (new: `glossaryVerbs` accessor `GlossaryVerbs() []string`)
- Test: `internal/syntax/parser_test.go`, `internal/syntax/glossary_meta_test.go`

**Interfaces:**
- Produces: `File.Glossaries() []GlossaryDecl`; `GlossaryDecl.Domain() string` ("" if unscoped) and `.Relations() []GlossaryRelationDecl`; `GlossaryRelationDecl.Left()/.Verb()/.Right() string`; `syntax.GlossaryVerbs() []string` = `["same_as","contrasts","distinct_from"]`.

- [ ] **Step 1: Write the failing test** (`parser_test.go`):

```go
func TestParse_Glossary_ScopedAndShared(t *testing.T) {
	src := "glossary re {\n  billing/Invoice same_as subscriptions/Invoice\n}\nglossary {\n  ordering/order distinct_from offering/order\n}\n"
	greenRoot, li, diags := syntax.Parse(src)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Fatalf("unexpected parse error: [%s] %s", d.Code, d.Message)
		}
	}
	// Round-trip must be byte-identical.
	if got := syntax.Reassemble(syntax.Root(greenRoot)); got != src {
		t.Fatalf("round-trip mismatch:\n got: %q\nwant: %q", got, src)
	}
	file := syntax.AsFile(syntax.Root(greenRoot))
	gs := file.Glossaries()
	if len(gs) != 2 {
		t.Fatalf("want 2 glossary blocks, got %d", len(gs))
	}
	if gs[0].Domain() != "re" || gs[1].Domain() != "" {
		t.Fatalf("domains: got %q,%q want re,\"\"", gs[0].Domain(), gs[1].Domain())
	}
	rels := gs[0].Relations()
	if len(rels) != 1 || rels[0].Left() != "billing/Invoice" || rels[0].Verb() != "same_as" || rels[0].Right() != "subscriptions/Invoice" {
		t.Fatalf("unexpected relation: %+v", rels)
	}
	_ = li
}
```

(Use the reassemble/round-trip helper the existing round-trip tests use — grep `internal/syntax/roundtrip_test.go` for the exact function name, e.g. `reassembleGreen` or `Reassemble`, and match it.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run 'Glossary'`
Expected: FAIL (`glossary` is an unknown top-level construct today).

- [ ] **Step 3: Implement the parser.** Mirror `parseContextMapBlock` exactly:
  - Add lexer keyword recognition for `glossary` (contextual keyword; find where `context_map` maps to `TokenKwContextMap` and add the analogous `TokenKwGlossary`, or reuse the ident-based contextual recognition the parser uses at top level).
  - `parseGlossaryBlock`: `attachTrivia`; consumeAs `SyntaxKindKwGlossary`; if next is `TokenIdent`, `consumeAs(SyntaxKindGlossaryDomain)` (optional scope); require `{`; loop over relation statements each `<ref> <glossary_verb> <ref>` framed by newlines; `}`. A relation statement node is `SyntaxKindGlossaryRelation` wrapping left ref, verb token, right ref. Endpoints reuse `parseRef`/`parseEdgeEndpoint` (they already accept bare/slash-path slugs). The verb must be one of `glossaryVerbs`.
  - Add the four `SyntaxKind`s to `kind.go` following the `context_map` kinds.
  - Add `GlossaryDecl`/`GlossaryRelationDecl` AST views in `ast.go` mirroring `ContextMapDecl`/`EdgeDecl` (`Domain()` reads the `SyntaxKindGlossaryDomain` child token or ""; `Relations()` returns child `SyntaxKindGlossaryRelation` nodes; `Left()/Right()` read the ref text, `Verb()` the verb token).
  - `File.Glossaries()` returns `SyntaxKindGlossaryDecl` child nodes (mirror `File.ContextMaps()` at ast.go:406).
  - `glossary_meta.go`: `var glossaryVerbs = []string{"same_as","contrasts","distinct_from"}` (or a `map[string]bool` plus an ordered accessor) and `func GlossaryVerbs() []string`.

- [ ] **Step 4: Write the verb-meta test** (`glossary_meta_test.go`):

```go
func TestGlossaryVerbs(t *testing.T) {
	got := syntax.GlossaryVerbs()
	want := []string{"same_as", "contrasts", "distinct_from"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GlossaryVerbs() = %v, want %v", got, want)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/syntax/`
Expected: PASS. Also run `go test ./...` to confirm no round-trip regression, `gofmt -l internal/syntax/` and `go vet ./internal/syntax/` clean.

- [ ] **Step 6: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/kind.go internal/syntax/ast.go internal/syntax/glossary_meta.go internal/syntax/parser_test.go internal/syntax/glossary_meta_test.go
git -c commit.gpgsign=false commit -m "feat(syntax): glossary block parser + AST (same_as/contrasts/distinct_from term relations)"
```

---

### Task A2: Model type + projection

**Files:**
- Modify: `internal/model/model.go` (add `TermRelation` type + `CraftDoc.Glossary` field), `internal/syntax/projection.go` (project `file.Glossaries()` into `doc.Glossary`)
- Test: `internal/syntax/projection_test.go` (or the projection test file the context_map projection uses — grep for `contextMap`/`ProjectFromTree` in `internal/syntax/*_test.go`)

**Interfaces:**
- Consumes: `File.Glossaries()`, `GlossaryDecl.Relations()`, `GlossaryRelationDecl.Left()/.Verb()/.Right()` (Task A1).
- Produces: `model.TermRelation{Left,Verb,Right string}` (JSON `left`/`verb`/`right`); `CraftDoc.Glossary []TermRelation` (JSON `glossary,omitempty`).

- [ ] **Step 1: Write the failing test:**

```go
func TestProject_Glossary(t *testing.T) {
	src := "glossary re {\n  billing/Invoice same_as subscriptions/Invoice\n}\n"
	greenRoot, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(greenRoot), li)
	if len(doc.Glossary) != 1 {
		t.Fatalf("want 1 term relation, got %d", len(doc.Glossary))
	}
	got := doc.Glossary[0]
	if got.Left != "billing/Invoice" || got.Verb != "same_as" || got.Right != "subscriptions/Invoice" {
		t.Fatalf("unexpected: %+v", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run 'Project_Glossary'`
Expected: FAIL (`doc.Glossary` undefined).

- [ ] **Step 3: Implement.**
  - In `model.go`, add after `Edge` (line 29):

```go
// TermRelation is one authored cross-context term relation from a glossary
// block, connecting two term nodes (bare or domain-qualified, e.g.
// "billing/Invoice") with a symmetric relation verb (same_as/contrasts/
// distinct_from). Resolution/validation is a sema concern; this is a
// shape-only projection.
type TermRelation struct {
	Left  string `json:"left"`
	Verb  string `json:"verb"`
	Right string `json:"right"`
}
```

  and add `Glossary []TermRelation` json:"glossary,omitempty"` to `CraftDoc` after `ContextMap`.
  - In `projection.go`, after the ContextMap loop (line 181), add a mirrored loop over `file.Glossaries()` -> `g.Relations()` skipping any with an empty left/verb/right, appending `model.TermRelation`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/syntax/ ./internal/model/ && go test ./...`
Expected: PASS. `gofmt -l` / `go vet` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/model/model.go internal/syntax/projection.go internal/syntax/projection_test.go
git -c commit.gpgsign=false commit -m "feat(model): TermRelation + CraftDoc.Glossary projected from glossary blocks"
```

---

### Task A3: Sema — term-node resolution + shape diagnostics

**Files:**
- Modify: `internal/sema/sema.go` (add `Symbols.GlossaryRelations []GlossaryRelationSite`; collect in `AnalyzeFile` next to `collectContextMapEdgeSites`; resolve in `AnalyzeWorkspace` loop), `internal/sema/validate.go` (add `GlossaryRelationSite` type, `collectGlossaryRelationSites`, `resolveTermNode`, `glossaryRelationDiags`), `internal/sema/edge_verb_sync_test.go` or a new `glossary_verb_sync_test.go`
- Create: `internal/sema/validate_glossary_test.go`, `testdata/broken/glossary_endpoint_not_bc.{craft,diagnostics.json}`, `testdata/broken/glossary_self_relation.{craft,diagnostics.json}`

**Interfaces:**
- Consumes: `WorkspaceSymbols` (`Domains`/`BoundedContexts`), `resolveBCRef` (validate.go:362), `file.Glossaries()`, `GlossaryDecl.Domain()/.Relations()`.
- Produces: `resolveTermNode(ws, scopeDomain, node) (canonical string, bcKind string, ambiguous bool, term string)`; diagnostics `craft/sema/glossary-endpoint-not-bc` (error), `craft/sema/glossary-unresolved-bc` (warning), `craft/sema/glossary-ambiguous-bc` (error), `craft/sema/glossary-self-relation` (error).

- [ ] **Step 1: Write the failing tests** (`validate_glossary_test.go`, whitebox `package sema`, using the same `analyzeRelationshipSrc`-style pipeline helper the context_map resolution tests use; grep `internal/sema/validate_relationship_test.go` for it and reuse):
  - `domain re { billing subscriptions }` + `glossary re { billing/Invoice same_as subscriptions/Invoice }` -> 0 diagnostics.
  - a term node whose BC part is a declared domain (not a BC) -> one `glossary-endpoint-not-bc`.
  - `glossary re { billing/Invoice same_as billing/Invoice }` -> one `glossary-self-relation`.
  - (recommended) an ambiguous bare BC name across two domains -> one `glossary-ambiguous-bc`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run 'Glossary'`
Expected: FAIL.

- [ ] **Step 3: Implement.**
  - `GlossaryRelationSite` mirrors `ContextMapEdgeSite` (ScopeDomain, Left, Right, Verb, URI, LeftLine/Col, RightLine/Col). `collectGlossaryRelationSites` mirrors `collectContextMapEdgeSites`. Append to `syms.GlossaryRelations` in `AnalyzeFile`.
  - `resolveTermNode(ws, scopeDomain, node)`: split `node` on `/`. `term` = last segment; if fewer than 2 segments, the BC reference is empty. Reconstruct the BC reference from the remaining segments (`bc` or `domain/bc`) and call `resolveBCRef(ws, scopeDomain, bcRef)`. Return the canonical identity `resolvedBC + "/" + term`, the BC kind, ambiguity, and term.
  - `glossaryRelationDiags(ws, site)`: resolve both nodes. Per node: ambiguous -> `glossary-ambiguous-bc` (error); bcKind non-empty and != "bc" -> `glossary-endpoint-not-bc` (error); bcKind == "" -> `glossary-unresolved-bc` (warning). If both resolve to the same canonical term node -> one `glossary-self-relation` (error). Anchor ranges on the offending node using the site's Line/Col, mirroring `relationshipEdgeDiags`.
  - In `AnalyzeWorkspace`, add a loop over `syms.GlossaryRelations` calling `glossaryRelationDiags`, mirroring the context_map loop.
  - Add a verb-sync test asserting `syntax.GlossaryVerbs()` equals the sema-side recognized set (define a `glossaryRelationVerbs` map in sema if the recognition needs one; otherwise assert against `syntax.GlossaryVerbs()` directly).
  - Generate the two broken `.diagnostics.json` goldens matching a sibling `testdata/broken/*.diagnostics.json` schema; the `.craft` must declare the domains/BCs it references.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sema/ ./internal/parser_diff/ && go test ./...`
Expected: PASS. `gofmt -l internal/sema/` / `go vet ./internal/sema/` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/sema.go internal/sema/validate.go internal/sema/validate_glossary_test.go internal/sema/glossary_verb_sync_test.go testdata/broken/glossary_endpoint_not_bc.* testdata/broken/glossary_self_relation.*
git -c commit.gpgsign=false commit -m "feat(sema): resolve glossary term nodes to BCs; endpoint-not-bc + self-relation"
```

---

### Task A4: Sema — glossary redundant + conflicting lints

**Files:**
- Modify: `internal/sema/sema.go` (extend the AnalyzeWorkspace glossary loop with a `seen` map, mirroring the context_map redundant lint), `internal/sema/validate.go` (add `glossaryLintDiags`)
- Modify: `internal/sema/validate_glossary_test.go`
- Create: `testdata/broken/glossary_redundant.{craft,diagnostics.json}`, `testdata/broken/glossary_conflicting.{craft,diagnostics.json}`

**Interfaces:**
- Produces: `craft/lint/glossary-redundant` (warning) on the 2nd+ occurrence of the same unordered canonical term-node pair + same verb; `craft/lint/glossary-conflicting-relation` (warning) when a pair has `same_as` together with `distinct_from` or `contrasts`.

- [ ] **Step 1: Write the failing tests:**
  - two identical `same_as` relations for the same pair -> one `glossary-redundant`.
  - `billing/Invoice same_as subscriptions/Invoice` and `billing/Invoice distinct_from subscriptions/Invoice` (same pair) -> one `glossary-conflicting-relation`.
  - a single relation, and two different-verb non-conflicting relations (e.g. `distinct_from` + `contrasts`) -> zero lint diagnostics.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run 'GlossaryLint|Redundant|Conflict'`
Expected: FAIL.

- [ ] **Step 3: Implement.** In the AnalyzeWorkspace glossary loop, keep two maps keyed on the sorted canonical term-node pair:
  - `seenVerb map[string]bool` keyed on `pair + "\x00" + verb`; a 2nd hit -> `glossary-redundant` (warning, range at left node). Only when both nodes resolve to a bc (skip unresolved/ambiguous, like the context_map redundant lint).
  - `verbsForPair map[string]map[string]bool` (pair -> set of verbs); after recording, if the pair's verb set contains `same_as` AND (`distinct_from` OR `contrasts`), emit `glossary-conflicting-relation` once per pair (guard with a `conflictReported` set so it fires once). Range at the left node of the relation that completed the conflict.
  Factor these into `glossaryLintDiags` or inline in the loop, whichever reads cleaner; declare the maps once above the outer per-file loop so they span the workspace (mirror `redundantRelationshipDiag`'s `seen` placement).
  - Generate the two broken `.diagnostics.json` goldens.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sema/ ./internal/parser_diff/ && go test ./...`
Expected: PASS. `gofmt`/`vet` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/sema.go internal/sema/validate.go internal/sema/validate_glossary_test.go testdata/broken/glossary_redundant.* testdata/broken/glossary_conflicting.*
git -c commit.gpgsign=false commit -m "feat(sema): glossary redundant + conflicting-relation lints"
```

---

### Task A5: Public API (pkg/craft)

**Files:**
- Modify: `pkg/craft/craftdoc.go` (add `TermRelation = model.TermRelation` alias)
- Create: `pkg/craft/glossary.go`, `pkg/craft/glossary_test.go`

**Interfaces:**
- Produces: `craft.TermRelation` (alias); `craft.GlossaryVerbs() []string` wrapping `syntax.GlossaryVerbs()`.

- [ ] **Step 1: Write the failing test** (`glossary_test.go`):

```go
func TestGlossaryVerbs(t *testing.T) {
	if got := craft.GlossaryVerbs(); !reflect.DeepEqual(got, []string{"same_as", "contrasts", "distinct_from"}) {
		t.Fatalf("GlossaryVerbs() = %v", got)
	}
}

func TestParse_ExposesGlossary(t *testing.T) {
	doc, _, _ := craft.Parse("f.craft", "glossary { billing/Invoice same_as subscriptions/Invoice }\n")
	if len(doc.Glossary) != 1 || doc.Glossary[0].Verb != "same_as" {
		t.Fatalf("glossary not exposed: %+v", doc.Glossary)
	}
}
```

(Confirm the real `craft.Parse` signature/return arity from `pkg/craft/parse.go` and match it.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/craft/ -run 'Glossary'`
Expected: FAIL.

- [ ] **Step 3: Implement.**
  - In `craftdoc.go`'s alias block add `TermRelation = model.TermRelation`.
  - `pkg/craft/glossary.go`: `func GlossaryVerbs() []string { return syntax.GlossaryVerbs() }` (import `internal/syntax`, already imported by the package).

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/craft/ && go test ./...`
Expected: PASS. `gofmt`/`vet` clean.

- [ ] **Step 5: Commit**

```bash
git add pkg/craft/craftdoc.go pkg/craft/glossary.go pkg/craft/glossary_test.go
git -c commit.gpgsign=false commit -m "feat(craft): exported TermRelation + GlossaryVerbs()"
```

---

### Task A6: Corpus goldens + round-trip

**Files:**
- Create: `testdata/corpus/08_glossary/relations.craft` + `.craftjson`

**Interfaces:** consumes `syntax.ProjectFromTree` + the corpus/round-trip harness (`internal/parser_diff/v2_test.go`, `internal/syntax/roundtrip_test.go`).

- [ ] **Step 1: Add the fixture** `testdata/corpus/08_glossary/relations.craft`:

```craft
glossary re {
  billing/Invoice same_as subscriptions/Invoice
  billing/dunning contrasts subscriptions/dunning
}

glossary {
  re/billing/Invoice distinct_from legacy/reporting/Invoice
}
```

- [ ] **Step 2: Generate the golden.** Goldens are `json.MarshalIndent(syntax.ProjectFromTree(syntax.Root(greenRoot), li), "", "  ")`, pretty-printed 2-space; the `glossary` array is `{"left","verb","right"}` with RAW node text. No `-update` flag exists: generate via a TEMPORARY throwaway test in `internal/parser_diff/` that writes the `.craftjson`, run it once, then DELETE it (leave no scaffolding; verify `git status` shows only the two fixture files). The corpus comparison is structural (unmarshal + DeepEqual).

- [ ] **Step 3: Run tests**

Run: `go test ./internal/parser_diff/ ./internal/syntax/ && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add testdata/corpus/08_glossary
git -c commit.gpgsign=false commit -m "test: corpus goldens for glossary term relations (scoped + shared)"
```

---

### Task A7: tree-sitter glossary grammar (separate repo)

**Repo:** `/Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft`. Branch off current `main`. CLI: `node_modules/.bin/tree-sitter`. Two commits (source then regenerated `src/`). `git -c commit.gpgsign=false`. **Do NOT push.** Report branch + both SHAs. Do NOT bump `CORPUS_VERSION` (that is a release step, Group C).

**Files:** `grammar.js`, `test/corpus/*`, regenerated `src/*`, `queries/highlights.scm` (if verbs enumerated).

- [ ] **Step 1: Add `glossary_block` to `grammar.js`** mirroring `context_map_block` (which has `context_map_scope: $ => $.identifier` and `edge_stmt: seq($.ref, $.edge_verb, $.ref)`): `glossary_block: seq('glossary', optional($.glossary_scope), '{', repeat1($._newline), repeat(seq($.glossary_stmt, repeat1($._newline))), '}', repeat($._newline))`; `glossary_scope: $ => $.identifier`; `glossary_stmt: seq($.ref, $.glossary_verb, $.ref)`; `glossary_verb: choice('same_as','contrasts','distinct_from')`. Add `glossary_block` to the top-level `choice`. The existing `slug`/`ref` rules already accept `a/b/c` paths, so term nodes parse with no new ref rule (verify `billing/Invoice` and `re/billing/Invoice` parse).

- [ ] **Step 2: Regenerate + corpus.** `node_modules/.bin/tree-sitter generate && npm test`. Add `test/corpus/` cases: a scoped `glossary re { }` with two verbs, and a shared block with a domain-qualified node. Generate expected trees with `tree-sitter parse`.

- [ ] **Step 3: highlights.** If `queries/highlights.scm` captures `(glossary_verb)` as a whole node it highlights automatically; otherwise add the capture. Confirm with `tree-sitter query`.

- [ ] **Step 4: Commit (two commits, NO push).** Report branch + both SHAs.

---

### Task A8: Docs + example + CLI verification

**Files:**
- Create: `docs/page/language/glossary.md`
- Modify: `docs/page/.vitepress/config.mts` (sidebar item), `docs/page/language/overview.md` + `docs/page/language/context-map.md` (cross-links), `CLAUDE.md` (keyword row + example), `examples/e-comerce.craft` (a `glossary { }` block)

- [ ] **Step 1: Write `docs/page/language/glossary.md`** mirroring `context-map.md`: what a glossary is (cross-context ubiquitous-language term relations, distinct from the strategic BC map), the block (`glossary { }` / `glossary <domain> { }`, repeatable + merged), term-node identity (`<bc>/<term>` and `<domain>/<bc>/<term>`; `.` is events-only), the three symmetric verbs, the six diagnostics table (copy codes/severities verbatim from the spec), and a worked example. Read `context-map.md` first for tone.

- [ ] **Step 2: Wire nav + cross-links.** Add `{ text: 'Glossary', link: '/language/glossary' }` to the Language Reference sidebar group in `config.mts` (after Context Map). Reference from `overview.md` and add a "see also" from `context-map.md`. Attempt `cd docs/page && npm run docs:build` (best-effort; if the toolchain is unavailable, verify links by inspection and report).

- [ ] **Step 3: CLI verification (no code change).** Author a `.craft` with a `glossary` block and run `go run ./cmd/craft check <file>` (JSON contains a `glossary` array), `validate <file>` (clean for valid; surfaces `glossary-self-relation`/`glossary-unresolved-bc` on broken input), `inspect <file>`. Report actual output.

- [ ] **Step 4: CLAUDE.md + example.** Add a `glossary` row to the DSL keyword table + a short example; add a coherent `glossary { }` block to `examples/e-comerce.craft` referencing BCs declared there. Confirm `go run ./cmd/craft validate examples/e-comerce.craft` stays clean.

- [ ] **Step 5: Run suite** `go test ./cmd/... && go build ./...`. PASS.

- [ ] **Step 6: Commit**

```bash
git add docs/page/language/glossary.md docs/page/.vitepress/config.mts docs/page/language/overview.md docs/page/language/context-map.md CLAUDE.md examples/e-comerce.craft
git -c commit.gpgsign=false commit -m "docs: glossary language reference + example; CLI verified"
```

---

# GROUP B — context_map cross-validation lint

### Task B1: Dependency-edge builder + separate-ways-violation (R1)

**Files:**
- Modify: `internal/sema/lint.go` (add `lintContextMapConsistency`, invoke from `LintWorkspace` at lint.go:29 area), `internal/sema/validate.go` if a shared helper is cleaner
- Create: `internal/sema/lint_contextmap_test.go`

**Interfaces:**
- Produces: `craft/lint/separate-ways-violation` (warning). Internal: a dependency-edge builder mapping each communicating BC pair to the set of observed directions.

- [ ] **Step 1: Write the failing test** (`lint_contextmap_test.go`, whitebox `package sema`, drive `LintWorkspace` the way `internal/sema/lint_test.go` drives `lintDeadEvents` — grep it for the helper that builds `perFileTrees` + `WorkspaceSymbols` from source):
  - `domain re { billing vas }`, `context_map re { billing separate_ways vas }`, and a use case where `billing asks vas to ...` -> one `separate-ways-violation`.
  - same context_map but NO communication between billing and vas -> zero diagnostics.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run 'ContextMapConsistency|SeparateWays'`
Expected: FAIL.

- [ ] **Step 3: Implement `lintContextMapConsistency(perFileTrees, ws, lis)`.**
  - Build the dependency edges over resolved BC identities. For each `perFileTrees` file -> `syntax.AsFile(tree).UseCases()` -> scenarios:
    - sync: for each action with `Kind()=="sync_action"`, resolve `SubjectName()` and `TargetName()` via `resolveBCRef(ws, "", name)`; if both resolve to a bc and differ, record a dependency edge `subject -> target`.
    - async: for each action with `Kind()=="async_action"`, note (publisher=SubjectName, event=EventValue()); for each trigger with `Kind()=="domain_listen"`, note (listener=ContextName(), event=EventValue()). Pair by equal event value across the workspace; for each (publisher,listener) with distinct resolved bcs, record `listener -> publisher`.
    - Store as `map[pairKey]struct{ fwd, rev bool }` where the pair key is the sorted resolved BC pair and fwd/rev track direction relative to the sorted order; plus a `firstSite` position per pair for R3 ranges (Task B3).
  - For each `context_map` edge across `file.ContextMaps()` whose verb is `separate_ways` and whose endpoints both resolve to bcs: if any dependency (fwd or rev) exists for that pair -> emit `separate-ways-violation` (warning) anchored on the edge's left endpoint.
  - Invoke `lintContextMapConsistency` from `LintWorkspace` alongside the existing lints.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sema/ && go test ./...`
Expected: PASS. `gofmt`/`vet` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/lint.go internal/sema/validate.go internal/sema/lint_contextmap_test.go
git -c commit.gpgsign=false commit -m "feat(sema): cross-validation lint scaffold + separate-ways-violation"
```

---

### Task B2: Direction-inverted (R2a) + bidirectional (R2b)

**Files:**
- Modify: `internal/sema/lint.go` (`lintContextMapConsistency`), `internal/sema/lint_contextmap_test.go`

**Interfaces:**
- Produces: `craft/lint/relationship-direction-inverted` (warning), `craft/lint/relationship-bidirectional` (hint).

- [ ] **Step 1: Write the failing tests:**
  - `customer_supplier billing vas` (billing upstream=LEFT), use case `billing asks vas ...` only (upstream asks downstream = inverted, RIGHT does not ask LEFT) -> one `relationship-direction-inverted`; with `vas asks billing` present instead -> zero.
  - both `billing asks vas` and `vas asks billing` -> one `relationship-bidirectional` hint, no direction-inverted.
  - a `published_language` edge exercised via `notifies`/`listens` in the correct direction (LEFT publishes, RIGHT listens) -> zero (proves async dependency direction is handled).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run 'DirectionInverted|Bidirectional'`
Expected: FAIL.

- [ ] **Step 3: Implement.** For each `context_map` edge whose verb is directional (use `syntax.LookupEdgeVerbMeta(verb)` / `!syntax.EdgeVerbSymmetric(verb)`), both endpoints resolving to bcs, look up the dependency record for the pair. Expected dependency is `RIGHT -> LEFT` (downstream depends on upstream). Translate fwd/rev (relative to the sorted pair) into "correct-direction present" and "inverted-direction present":
  - inverted present AND correct absent -> `relationship-direction-inverted` (warning).
  - both present -> `relationship-bidirectional` (hint).
  - correct only, or neither -> silent (neither: partial-view principle).
  Anchor ranges on the edge's left endpoint.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sema/ && go test ./...`
Expected: PASS. `gofmt`/`vet` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/lint.go internal/sema/lint_contextmap_test.go
git -c commit.gpgsign=false commit -m "feat(sema): relationship direction-inverted + bidirectional cross-validation"
```

---

### Task B3: Unclassified-communication (R3)

**Files:**
- Modify: `internal/sema/lint.go` (`lintContextMapConsistency`), `internal/sema/lint_contextmap_test.go`

**Interfaces:**
- Produces: `craft/lint/unclassified-communication` (hint).

- [ ] **Step 1: Write the failing tests:**
  - a use case `billing asks ledger to ...` with billing+ledger both BCs and NO `context_map` edge between them -> one `unclassified-communication` hint anchored on the first communicating action.
  - the same pair with any `context_map` edge (e.g. `customer_supplier`) -> zero.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run 'Unclassified'`
Expected: FAIL.

- [ ] **Step 3: Implement.** Build the set of BC pairs that have any `context_map` edge (unordered, resolved). For each communicating pair in the dependency map whose pair is NOT in that set, emit `unclassified-communication` (hint) once, anchored on the pair's recorded `firstSite` action position. Guard with a `reported` set so each pair fires once.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/sema/ && go test ./...`
Expected: PASS. `gofmt`/`vet` clean.

- [ ] **Step 5: Commit**

```bash
git add internal/sema/lint.go internal/sema/lint_contextmap_test.go
git -c commit.gpgsign=false commit -m "feat(sema): unclassified-communication cross-validation hint"
```

---

### Task B4: Docs for the cross-validation lint

**Files:**
- Modify: `docs/page/language/context-map.md` (a "Cross-validation" section documenting the four signals + the partial-view principle), `CLAUDE.md` (mention the lint if the context_map row references diagnostics)

- [ ] **Step 1:** Add a "Cross-validation with the communication view" section to `context-map.md` explaining the dependency-edge intuition and a table of the four codes (`separate-ways-violation` warning, `relationship-direction-inverted` warning, `relationship-bidirectional` hint, `unclassified-communication` hint) with a one-line "fires when" each, plus the "absence of communication never warns" caveat. Attempt `npm run docs:build` (best-effort).

- [ ] **Step 2: Run** `go build ./...` (docs-only change; nothing to unit-test) and confirm no broken internal links.

- [ ] **Step 3: Commit**

```bash
git add docs/page/language/context-map.md CLAUDE.md
git -c commit.gpgsign=false commit -m "docs: document context_map cross-validation lint signals"
```

---

# GROUP C — Release (GATED on explicit user confirmation)

- [ ] **Step 1:** Add a `## [2.13.0]` CHANGELOG entry covering both features (glossary block + cross-validation lint), matching the existing entry format.
- [ ] **Step 2:** Commit. `go build ./... && go test ./...` green.
- [ ] **Step 3 (after explicit user confirmation):** merge to `main` `--no-ff`, retest, tag `v2.13.0`, push `main` + tag (triggers Docker publish + docs deploy). THEN in tree-sitter-craft: bump `CORPUS_VERSION` to `v2.13.0`, merge the glossary-grammar branch to `main`, push; verify the corpus-compat gate is green against `v2.13.0`. Order matters: the craft `v2.13.0` tag must exist before the tree-sitter compat can fetch it.

---

## Self-Review

- **Spec coverage:** glossary spec §2-§9 map to A1 (syntax), A2 (model/projection), A3 (resolution + 3 shape diagnostics), A4 (redundant + conflicting lints), A5 (pkg/craft), A6 (corpus), A7 (tree-sitter), A8 (docs/example/CLI). Cross-validation spec §2-§7 map to B1 (dependency edge + R1), B2 (R2a/R2b), B3 (R3), B4 (docs). Release coordination -> Group C.
- **Placeholder scan:** each code step carries real code or a concrete mirror target (a named existing function to copy) plus exact file:line references; no "TBD"/"add validation".
- **Type consistency:** `model.TermRelation{Left,Verb,Right}` used identically in A2/A5; `GlossaryRelationSite`/`resolveTermNode`/`glossaryRelationDiags` names consistent across A3/A4; `lintContextMapConsistency` + the dependency `map[pairKey]{fwd,rev,firstSite}` consistent across B1/B2/B3; `syntax.GlossaryVerbs()` consistent across A1/A5.
