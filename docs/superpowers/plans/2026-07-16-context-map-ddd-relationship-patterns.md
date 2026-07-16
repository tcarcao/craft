# `context_map` Strategic Relationship View — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Redesign `context_map` into craft's strategic BC-relationship view: eight DDD-canonical relationship patterns between bounded contexts, bare/domain-qualified endpoints (no `bc:`), `LEFT = upstream`, repeatable + optionally domain-scoped blocks, with realization/term edges removed from the block.

**Architecture:** Reuse the existing `context_map '{' edge_stmt* '}'` shape. Replace the verb vocabulary; drop the `bc:` kind requirement on endpoints; add an optional domain-scope identifier after the keyword. Verb *semantics* (class, direction roles, symmetry) live in one metadata table in `internal/syntax`, consumed by `internal/sema` and re-exported by `pkg/craft`. Endpoint validation moves from kind-prefix checks to **resolution** against the workspace BC index.

**Tech Stack:** Go 1.23 (`github.com/tcarcao/craft/v2`), tree-sitter (`tree-sitter-craft`), goreleaser on `v*` tags.

**Spec:** `docs/superpowers/specs/2026-07-16-context-map-ddd-relationship-patterns.md` (approved 2026-07-16).

## Global Constraints

- **The eight patterns (exact spelling):** directional — `customer_supplier`, `conformist`, `anticorruption_layer`, `open_host_service`, `published_language`; symmetric — `partnership`, `shared_kernel`, `separate_ways`. `big_ball_of_mud` is NOT added.
- **Direction: LEFT = upstream, RIGHT = downstream** for directional patterns; symmetric patterns are order-free. Direction is verb metadata, not stored per edge.
- **Endpoints are bounded contexts named bare (`billing`) or domain-qualified (`re/billing`).** No `bc:` prefix. `/` = node-identity separator; `.` is reserved for events and must never denote a BC.
- **Realization (`realized_by`/`also_realizes`) and term (`same_as`/`contrasts`/`distinct_from`) verbs are REMOVED from `context_map`.** Term relations' future home is a `glossary { }` block (separate spec) — not built here.
- **`context_map` is repeatable** (per file and workspace; all blocks merge into `CraftDoc.ContextMap []Edge`) and takes an **optional domain scope**: `context_map <domain> { … }`.
- **Endpoint resolution:** bare name → the BC of that name (scoped block: within its domain; unscoped: globally unique or error-ambiguous); `domain/name` → explicit. Endpoint that resolves to a non-BC ⇒ error; unresolved ⇒ warning.
- **`internal/syntax.edgeKeywords` stays the parser gate / single source of truth;** the metadata table and sema maps must agree with it (sync test).
- **No import cycles:** metadata table in `internal/syntax`; `pkg/craft` wraps it; `internal/sema` must NOT import `pkg/craft`.
- **Clean slate:** no `.craft` files or external consumers exist — remove old verbs outright, no deprecation. `model.Edge{Left,Verb,Right}` shape unchanged (values now hold bare/qualified BC names + pattern verbs).
- **Module path `/v2`; lossless round-trip preserved** (verbs are ordinary `SyntaxKindEdgeKw` tokens; endpoints ordinary refs).

## Interfaces that already exist (verified — use these, don't reinvent)

- `internal/syntax/parser.go`: `parseContextMapBlock()` (consumes `SyntaxKindKwContextMap`, then `{`); `parseEdgeStmt()` → `parseEdgeEndpoint()` → `parseRef()` (parseRef already accepts bare slugs like `re/billing` and single idents like `billing`); `var edgeKeywords []string` (~2003); `isEdgeKeyword`; `EdgeKeywords()`.
- `internal/syntax/ast.go`: `File.ContextMaps() []ContextMapDecl`; `ContextMapDecl.Edges() []EdgeDecl`, `.Keyword()`, `.Line(li)`; `EdgeDecl.Left()/.Right()/.Verb() string`, `.LeftRef()/.RightRef() *RefDecl`.
- `internal/sema/validate.go`: `validateContextMapEdges(uri, file, li, hasLI)` (loop `for cm := range file.ContextMaps() { for edge := range cm.Edges() { left,right,verb := edge.Left(),edge.Right(),edge.Verb() … } }`); `classifyEdgeVerb(...)`; `edgeEndpointKindDiag(...)`; `refKindWord(text)`; `colToLSP`/`lineToLSP`.
- `internal/sema/sema.go`: `WorkspaceSymbols{ Domains map[string]DomainSymbol; BoundedContexts map[string]DomainSymbol }` (`BoundedContexts` maps **BC name → owning DomainSymbol** — the resolution index); `DomainSymbol{ Name string; BoundedContexts []string }`.
- `pkg/craft/craftdoc.go`: type aliases (`Edge = model.Edge`).
- `tree-sitter-craft/grammar.js`: `context_map_block`, `edge_stmt: seq($.ref,$.edge_verb,$.ref)`, `edge_verb: choice(...)`, `ref`, `ref_kind`. `queries/highlights.scm`: `(edge_verb) @craft.edge-verb` (captures whole node).

---

## Task 1: Swap verb vocabulary + drop `bc:` on endpoints

**Files:**
- Modify: `internal/syntax/parser.go` (`edgeKeywords` ~2003; the edge-keyword diagnostic ~1924; `parseEdgeEndpoint` if it *requires* a kind)
- Modify: any corpus fixture using the OLD `context_map` syntax — at least `testdata/corpus/99_mixed/dsl-vnext.craft` (has `bc:… realized_by service:…` and `term:… contrasts term:…`)
- Test: `internal/syntax/parser_test.go` (or the package's edge test)

**Interfaces:**
- Produces: `edgeKeywords` = the 8 patterns; parser accepts `billing customer_supplier vas` and `re/billing partnership payments/ledger`, producing `model.Edge{Left,Verb,Right}` with bare/qualified names in Left/Right.

- [ ] **Step 1: Write the failing test** — parse a minimal new-form block and assert the Edge:

```go
func TestParse_ContextMap_RelationshipEdge(t *testing.T) {
	doc, _ := craft.Parse("f.craft", "context_map {\n  billing customer_supplier vas\n  re/billing partnership payments/ledger\n}\n")
	if len(doc.ContextMap) != 2 {
		t.Fatalf("want 2 edges, got %d: %+v", len(doc.ContextMap), doc.ContextMap)
	}
	e := doc.ContextMap[0]
	if e.Left != "billing" || e.Verb != "customer_supplier" || e.Right != "vas" {
		t.Errorf("edge0 = %+v, want {billing customer_supplier vas}", e)
	}
	if doc.ContextMap[1].Left != "re/billing" || doc.ContextMap[1].Right != "payments/ledger" {
		t.Errorf("edge1 = %+v, want qualified endpoints", doc.ContextMap[1])
	}
}
```
(Use whatever the package's existing parse-helper is; mirror a sibling test's construction.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./pkg/craft/ -run TestParse_ContextMap_RelationshipEdge`
Expected: FAIL — `customer_supplier` is not in `edgeKeywords`, so `parseEdgeStmt` rejects it.

- [ ] **Step 3: Replace `edgeKeywords`** in `internal/syntax/parser.go`:

```go
// edgeKeywords is the single source of truth for context_map relationship verbs
// (DDD strategic context-mapping patterns). LEFT = upstream for directional verbs.
var edgeKeywords = []string{
	// directional
	"customer_supplier", "conformist", "anticorruption_layer",
	"open_host_service", "published_language",
	// symmetric
	"partnership", "shared_kernel", "separate_ways",
}
```

- [ ] **Step 4: Ensure endpoints parse WITHOUT a `bc:` kind.** Inspect `parseEdgeEndpoint`: if it *requires* a `kind:` prefix or special-cases keyword-as-ident kinds, relax it so a bare ident (`billing`) or a bare slug (`re/billing`) parses via `parseRef` as an ordinary ref. Bare refs are already supported by `parseRef`; the change is to stop *expecting* a kind. Keep `EdgeDecl.Left()/.Right()` returning the raw ref text (`"billing"`, `"re/billing"`).

- [ ] **Step 5: Update the edge-keyword diagnostic** (~1924) to the new vocabulary:

```go
diags = append(diags, p.diagUnexpected(verb, "a relationship pattern: directional (customer_supplier/conformist/anticorruption_layer/open_host_service/published_language) or symmetric (partnership/shared_kernel/separate_ways)"))
```

- [ ] **Step 6: Migrate corpus fixtures** to the new syntax. In `testdata/corpus/99_mixed/dsl-vnext.craft`, replace the `context_map { … }` body's `bc:… realized_by service:…` and `term:… contrasts term:…` lines with relationship edges, e.g. `bc:re/billing customer_supplier bc:re/vas` → `re/billing customer_supplier re/vas`. Grep the corpus for any other `context_map` blocks and migrate them too. (Goldens regenerate in Task 7; here just make the `.craft` parse.)

- [ ] **Step 7: Run tests**

Run: `go test ./pkg/craft/ ./internal/syntax/`
Expected: PASS — new form parses; migrated fixtures parse.

- [ ] **Step 8: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/parser_test.go testdata/corpus
git commit -m "feat(syntax): context_map relationship verbs + bare/qualified BC endpoints (drop bc:)"
```

---

## Task 2: Verb metadata table (single source of truth)

**Files:**
- Create: `internal/syntax/edge_meta.go`
- Test: `internal/syntax/edge_meta_test.go`

**Interfaces:**
- Produces:
```go
type EdgeClass string
const ( EdgeDirectional EdgeClass = "directional"; EdgeSymmetric EdgeClass = "symmetric" )
type EdgeVerbMeta struct {
	Verb           string
	Class          EdgeClass
	UpstreamRole   string // left role for directional (e.g. "supplier"); "" for symmetric
	DownstreamRole string // right role for directional (e.g. "customer"); "" for symmetric
}
func EdgeVerbMetas() []EdgeVerbMeta            // stable order, one per edgeKeywords entry
func LookupEdgeVerbMeta(verb string) (EdgeVerbMeta, bool)
func EdgeVerbSymmetric(verb string) bool
```

- [ ] **Step 1: Write the failing test** — `internal/syntax/edge_meta_test.go`:

```go
package syntax

import "testing"

func TestEdgeVerbMetas_MatchKeywordsAndDirection(t *testing.T) {
	if len(EdgeVerbMetas()) != len(edgeKeywords) {
		t.Fatalf("metas=%d keywords=%d", len(EdgeVerbMetas()), len(edgeKeywords))
	}
	cs, ok := LookupEdgeVerbMeta("customer_supplier")
	if !ok || cs.Class != EdgeDirectional || cs.UpstreamRole != "supplier" || cs.DownstreamRole != "customer" {
		t.Fatalf("customer_supplier meta = %+v ok=%v", cs, ok)
	}
	if !EdgeVerbSymmetric("partnership") || EdgeVerbSymmetric("conformist") {
		t.Errorf("symmetry flags wrong")
	}
	for _, k := range edgeKeywords {
		if _, ok := LookupEdgeVerbMeta(k); !ok {
			t.Errorf("keyword %q missing from metas", k)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestEdgeVerbMetas`
Expected: FAIL (undefined symbols).

- [ ] **Step 3: Write `internal/syntax/edge_meta.go`** with all 8 entries. Directional roles: customer_supplier(supplier,customer), conformist(upstream,conformist), anticorruption_layer(upstream,downstream), open_host_service(host,consumer), published_language(publisher,consumer). Symmetric (partnership/shared_kernel/separate_ways): Class=EdgeSymmetric, roles "". `EdgeVerbMetas()` returns them in `edgeKeywords` order; `EdgeVerbSymmetric` returns `Class==EdgeSymmetric`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/syntax/ -run TestEdgeVerbMetas`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/syntax/edge_meta.go internal/syntax/edge_meta_test.go
git commit -m "feat(syntax): edge-verb metadata table (class + upstream/downstream roles)"
```

---

## Task 3: Optional domain scope on the block

**Files:**
- Modify: `internal/syntax/parser.go` (`parseContextMapBlock`), `internal/syntax/kind.go` (new `SyntaxKindContextMapDomain`), `internal/syntax/ast.go` (`ContextMapDecl.Domain()`)
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Produces: `ContextMapDecl.Domain() string` returning the scope identifier, or `""` if unscoped. Parser accepts `context_map re { … }`.

- [ ] **Step 1: Write the failing test**

```go
func TestParse_ContextMap_DomainScope(t *testing.T) {
	doc, _ := craft.Parse("f.craft", "context_map re {\n  billing customer_supplier vas\n}\ncontext_map {\n  re/billing partnership payments/ledger\n}\n")
	if len(doc.ContextMap) != 2 { // repeatable + merged
		t.Fatalf("want 2 merged edges, got %d", len(doc.ContextMap))
	}
}
```
Plus a whitebox `internal/syntax` test asserting `File.ContextMaps()[0].Domain() == "re"` and `[1].Domain() == ""`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/craft/ ./internal/syntax/ -run 'DomainScope'`
Expected: FAIL — the identifier before `{` is unexpected today.

- [ ] **Step 3: Parse the optional scope.** In `parseContextMapBlock`, after `p.consumeAs(SyntaxKindKwContextMap)` and before the `{` check, if `p.peek().Type == lexer.TokenIdent`, `p.consumeAs(SyntaxKindContextMapDomain)`. Add `SyntaxKindContextMapDomain` to `kind.go`. Add `ContextMapDecl.Domain()` in `ast.go` reading the first `SyntaxKindContextMapDomain` child token text (or `""`).

- [ ] **Step 4: Confirm repeatable/merge already works.** `File.ContextMaps()` already returns all blocks and `validateContextMapEdges` iterates all; the projection appends all edges to `CraftDoc.ContextMap`. Verify the pkg/craft test above sees 2 merged edges (no projection change expected — confirm, and only touch projection if the count is wrong).

- [ ] **Step 5: Run tests**

Run: `go test ./pkg/craft/ ./internal/syntax/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/kind.go internal/syntax/ast.go internal/syntax/parser_test.go
git commit -m "feat(syntax): optional domain scope on context_map; blocks repeatable + merged"
```

---

## Task 4: Sema — endpoint-is-BC resolution + self-relationship

**Files:**
- Modify: `internal/sema/validate.go` (`validateContextMapEdges`, `classifyEdgeVerb` → replace kind logic; remove `edgeRealizationVerbs`/`edgeTermVerbs`; add `edgeRelationshipVerbs`), `internal/sema/edge_verb_sync_test.go`
- Create: `internal/sema/validate_relationship_test.go`, `testdata/broken/relationship_endpoint_not_bc.{craft,diagnostics.json}`, `testdata/broken/relationship_self.{craft,diagnostics.json}`

**Interfaces:**
- Consumes: `WorkspaceSymbols.BoundedContexts` (BC name → owning domain) and `.Domains`; `ContextMapDecl.Domain()`; `syntax.EdgeVerbMetas()`.
- Produces: resolution helper `resolveBCRef(ws, scopeDomain, ref string) (resolved string, kind string /*"bc"|"domain"|"service"|"actor"|""*/, ambiguous bool)`; diagnostics `craft/sema/edge-endpoint-not-bc` (error), `craft/sema/self-relationship` (error), `craft/sema/unresolved-bc` (warning), `craft/sema/ambiguous-bc` (error).

- [ ] **Step 1: Write the failing tests** (`validate_relationship_test.go`, whitebox `package sema`): a valid `context_map re { billing customer_supplier vas }` with domains declaring `billing`,`vas` in `re` → 0 diagnostics; an endpoint resolving to a domain/service → one `edge-endpoint-not-bc` error; `billing partnership billing` → one `self-relationship` error. Build the `syntax.File` + `WorkspaceSymbols` with the helper pattern used in the existing `internal/sema/validate_test.go` (grep it).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run Relationship`
Expected: FAIL (compile / wrong behavior — kind-based classify still in place).

- [ ] **Step 3: Rework `classifyEdgeVerb` → resolution-based.** Replace `edgeRealizationVerbs`/`edgeTermVerbs` with `edgeRelationshipVerbs` (the 8). Write `resolveBCRef(ws, scopeDomain, ref)`:
  - if `ref` is `domain/name`: look up `ws.Domains[domain]`; if it lists `name` → ("<domain>/<name>","bc",false) else fall through to unresolved.
  - else bare `name`: if `scopeDomain != ""` and `ws.Domains[scopeDomain]` lists `name` → resolved in scope; else consult `ws.BoundedContexts[name]` — resolved to that domain's BC; detect ambiguity by counting domains that declare `name` (needs a small count; if the workspace can't distinguish, treat a name present in ≥2 domains as ambiguous). If it resolves to a domain/service/actor symbol instead of a BC → kind accordingly.
  For each edge: resolve both endpoints; if either kind is non-`bc` and non-empty → `edge-endpoint-not-bc` error; if unresolved (kind=="") → `unresolved-bc` warning; if ambiguous → `ambiguous-bc` error ("qualify as `<domain>/<name>`"). If both resolve to the SAME bc → `self-relationship` error. Thread `scopeDomain` from `edge`'s owning `ContextMapDecl.Domain()`.
  Update the `validateContextMapEdges` loop to pass the block's domain scope and to use resolution instead of `refKindWord`.

- [ ] **Step 4: Update the sync test.** In `edge_verb_sync_test.go`, replace the realization/term unions with `edgeRelationshipVerbs`; assert `syntax.EdgeKeywords()` == keys(`edgeRelationshipVerbs`) == verbs in `syntax.EdgeVerbMetas()`.

- [ ] **Step 5: Add broken fixtures** `relationship_endpoint_not_bc.*` (an endpoint that is a declared domain, not a BC → error) and `relationship_self.*` (`billing partnership billing`). Generate `.diagnostics.json` matching a sibling's schema.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/sema/ && go test ./... -run Broken`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/sema/validate.go internal/sema/edge_verb_sync_test.go internal/sema/validate_relationship_test.go testdata/broken/relationship_endpoint_not_bc.* testdata/broken/relationship_self.*
git commit -m "feat(sema): resolve context_map endpoints to BCs; endpoint-not-bc + self-relationship"
```

---

## Task 5: Sema — `redundant-relationship` symmetric lint

**Files:**
- Modify: `internal/sema/validate.go` (`validateContextMapEdges`), `internal/sema/validate_relationship_test.go`
- Create: `testdata/broken/relationship_redundant.{craft,diagnostics.json}`

**Interfaces:**
- Consumes: `syntax.EdgeVerbSymmetric(verb)`, the loop's resolved endpoints.
- Produces: `craft/lint/redundant-relationship` (warning) on the 2nd occurrence of the same unordered *resolved* pair + same symmetric verb.

- [ ] **Step 1: Write the failing test** — two symmetric duplicates (`a partnership b` / `b partnership a`) → one warning; a directional pair in both orders → zero. (Drive through `validateContextMapEdges` with the file+workspace helper.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run Redundant`
Expected: FAIL.

- [ ] **Step 3: Implement.** In the edge loop, keep `seen := map[string]bool{}`. When `edgeRelationshipVerbs[verb] && syntax.EdgeVerbSymmetric(verb)`, key on sorted *resolved* endpoints + verb; 2nd hit → append `craft/lint/redundant-relationship` (SeverityWarning, range at left endpoint). Use resolved names so `a`/`re/a` for the same BC dedupe correctly.

- [ ] **Step 4: Add fixture** `relationship_redundant.*` (mirror how the tags feature recorded warning-level diagnostics).

- [ ] **Step 5: Run tests**

Run: `go test ./internal/sema/ && go test ./... -run Broken`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/validate.go internal/sema/validate_relationship_test.go testdata/broken/relationship_redundant.*
git commit -m "feat(sema): redundant-relationship lint for duplicated symmetric pairs"
```

---

## Task 6: `pkg/craft` exported edge-verb API

**Files:**
- Create: `pkg/craft/edges.go`, `pkg/craft/edges_test.go`

**Interfaces:**
- Produces: `EdgeClass` consts (`EdgeDirectional`/`EdgeSymmetric`), `EdgeVerbInfo{Verb, Class, UpstreamRole, DownstreamRole string; Symmetric bool}`, `EdgeVerbs() []EdgeVerbInfo`, `LookupEdgeVerb(verb) (EdgeVerbInfo, bool)` — all wrapping `internal/syntax` metadata.

- [ ] **Step 1: Write the failing test** — `pkg/craft/edges_test.go`: `LookupEdgeVerb("customer_supplier")` is directional, `UpstreamRole=="supplier"`, `!Symmetric`; `LookupEdgeVerb("partnership").Symmetric`; `len(EdgeVerbs())==8`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/craft/ -run TestEdgeVerbs`
Expected: FAIL (undefined).

- [ ] **Step 3: Write `pkg/craft/edges.go`** wrapping `syntax.EdgeVerbMetas()`/`LookupEdgeVerbMeta` into the public shapes (`Symmetric = Class==EdgeSymmetric`).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./pkg/craft/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/craft/edges.go pkg/craft/edges_test.go
git commit -m "feat(craft): exported edge-verb metadata API (class/roles/symmetry)"
```

---

## Task 7: Corpus goldens + round-trip

**Files:**
- Create: `testdata/corpus/05_context_map/relationships.craft` + `.craftjson`
- Regenerate: any `.craftjson` whose `.craft` changed in Task 1 (e.g. `99_mixed/dsl-vnext.craftjson`)

**Interfaces:** consumes `craft.Parse`, the corpus round-trip + golden harness.

- [ ] **Step 1: Add the fixture** `testdata/corpus/05_context_map/relationships.craft` (confirm/`ls` the corpus dir naming first):

```craft
context_map re {
  billing customer_supplier vas
  billing anticorruption_layer subscriptions
  billing partnership vas
}

context_map {
  re/billing separate_ways legacy/reporting
}
```

- [ ] **Step 2: Regenerate goldens** for this fixture and for every `.craft` changed in Task 1, using the corpus golden method. Confirm each golden's `contextMap` array holds the authored `Edge{Left,Verb,Right}` (bare/qualified names, pattern verbs), and that byte-for-byte round-trip holds.

- [ ] **Step 3: Run corpus + round-trip tests**

Run: `go test ./... -run 'Corpus|RoundTrip'`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add testdata/corpus
git commit -m "test: corpus goldens for context_map relationship edges (scoped + shared)"
```

---

## Task 8: tree-sitter grammar sync

**Repo:** `tree-sitter-craft` (separate; branch off current `main`; CLI at `node_modules/.bin/tree-sitter`; two commits — source then regenerated `src/`; `git config commit.gpgsign false` if signing fails; do NOT push).

**Files:** `grammar.js`, `test/corpus/*`, regenerated `src/*`; check `queries/highlights.scm`.

- [ ] **Step 1: Update `grammar.js`.** `context_map_block`: add an optional domain-scope identifier after `'context_map'` before `'{'`. `edge_stmt` endpoints: accept bare/`domain/name` refs (the existing `ref`/`slug` rules already cover these). `edge_verb`: replace the five old verbs with the eight patterns.

- [ ] **Step 2: Regenerate + run corpus**

Run: `cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft && node_modules/.bin/tree-sitter generate && npm test`
Expected: PASS (existing corpus green after updating any context_map cases to new syntax).

- [ ] **Step 3: Add corpus cases** — a domain-scoped block with a directional + a symmetric edge, and a shared block with a cross-domain qualified endpoint. Generate expected trees with `tree-sitter parse`.

- [ ] **Step 4: Verify highlights.** `(edge_verb) @craft.edge-verb` captures the whole node, so new verbs highlight automatically — confirm with `tree-sitter query`. Check `craft-vscode-extension` only if it enumerates edge verbs separately.

- [ ] **Step 5: Full-corpus compat** — migrate the fetched craft corpus expectations if pinned; ensure `bash scripts/fetch-corpus.sh` + parse loop stays `0 failed`.

- [ ] **Step 6: Commit (two commits, no push).** Report branch + SHAs.

---

## Task 9: Cross-validation lint (optional later slice — may defer)

**Files:** Modify `internal/sema/` (new lint pass reading use-case interactions + context_map).

- [ ] **Step 1:** Build the BC interaction set from use-case actions (`Action.Context`→`Action.TargetContext` for `asks`; note `notifies` is async — attribute only where a target BC is resolvable). Reuse/borrow the derivation the visualizer already performs if practical.
- [ ] **Step 2:** For each classified edge: `separate_ways` between interacting BCs → `craft/lint/contradicts-interaction` (warning); directional integration pattern between non-interacting BCs → `craft/lint/unbacked-relationship` (warning); interacting pair with no classification → `craft/lint/unclassified-relationship` (info).
- [ ] **Step 3:** Tests + fixtures; warnings only.
- [ ] **Step 4:** Commit.

> This slice is independently shippable and may be deferred to a follow-up cycle — the core (Tasks 1–8) delivers authoring + shape validation without it. Confirm with the human whether to build it now or defer.

---

## Task 10: CHANGELOG + release (GATED on user confirmation)

- [ ] **Step 1:** Add a `## [next-minor]` CHANGELOG entry: "context_map redesigned as the DDD strategic relationship view — eight relationship patterns (customer_supplier … separate_ways), bare/domain-qualified BC endpoints, repeatable + domain-scoped blocks; realization/term edges removed; exported edge-verb metadata API." Match the existing entry format.
- [ ] **Step 2:** Commit.
- [ ] **Step 3:** `go build ./... && go test ./...` — all green.
- [ ] **Step 4:** (After explicit user confirmation) merge to `main` `--no-ff`, retest, tag the next minor, push `main` + tag; push the tree-sitter branch. `glossary { }` and use-case ref unification are sibling specs, not part of this release.

---

## Notes carried from the spec (do not re-litigate during implementation)

- Realization and term verbs are REMOVED from `context_map`; term relations' `glossary { }` home is a separate spec, not built here.
- `context_map` endpoints are BCs named bare/`domain/name`; `bc:` is gone; `.` stays events-only.
- Direction = LEFT upstream (metadata); one pattern per statement; `big_ball_of_mud` excluded.
- craft validates shape/resolution, not semantic truth (the closest thing is the optional cross-validation lint, Task 9).
