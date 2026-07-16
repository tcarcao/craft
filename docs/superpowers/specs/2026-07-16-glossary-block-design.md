# Craft DSL — `glossary { }` block for cross-context term relations

**Status:** approved (brainstorm 2026-07-16)
**Feature:** a dedicated `glossary { }` block declaring symmetric relationships between ubiquitous-language terms across bounded contexts.
**Follows:** the `context_map` DDD strategic-relationship view shipped in v2.12.0 (`docs/superpowers/specs/2026-07-16-context-map-ddd-relationship-patterns.md`, §9). That work removed term relations (`same_as`/`contrasts`/`distinct_from`) from `context_map` and named `glossary { }` as their new home. This spec builds that home. It deliberately mirrors the shipped `context_map` design grain, so most machinery is reused.

---

## 1. Motivation and scope

The same word often means different things in different bounded contexts (an `Invoice` in `billing` is not the `Invoice` in `subscriptions`). DDD calls the map that records this the translation map or glossary. craft previously expressed it as term edges inside `context_map`; those were removed in v2.12.0 to keep `context_map` homogeneous (BC-to-BC relationships only). This spec restores the capability in its own block.

**In scope:** cross-context term *relations* (the three symmetric verbs), with `context_map`-style resolution and lints.

**Out of scope (explicitly):** term *definitions* / descriptions. craft has no term-declaration concept and this spec does not add one; term nodes are referenced, never declared. (This was considered and cut as unrequested scope.) Also out: any lint enable/disable configuration.

## 2. Syntax

```craft
glossary {
  billing/Invoice        same_as        subscriptions/Invoice
  re/billing/Invoice     contrasts       legacy/reporting/Invoice
  ordering/order         distinct_from   offering/order
}
```

- A statement is `<term_node> <verb> <term_node>`, one relation per line, mirroring `context_map`'s `Left Verb Right` shape.
- **Term node identity.** A term node is `<bc>/<term>` or `<domain>/<bc>/<term>`. The **last** `/`-separated segment is the term name; the segment(s) before it are the bounded-context identity (bare `bc`, or `domain/bc`). This reuses `context_map`'s rules: `/` is the node-identity separator, one level deeper than a BC endpoint; `.` stays reserved for events and must never appear in a term node. A node with only one segment (a bare term, no BC) is invalid.
- **Three verbs, all symmetric:** `same_as`, `contrasts`, `distinct_from`. There is no upstream/downstream direction; `A verb B` and `B verb A` mean the same thing.
- **Repeatable and optionally domain-scoped:** `glossary { }` and `glossary <domain> { }` are both legal, blocks are repeatable, and all blocks merge into a single projection. A domain scope lets a bare `bc/term` node resolve its BC within that domain, exactly as `context_map <domain> { }` does for BC endpoints.

## 3. Model and projection

- New `model.TermRelation{ Left string; Verb string; Right string }` (JSON `left`/`verb`/`right`), structurally parallel to `model.Edge`. `model.Edge` is unchanged.
- New `CraftDoc.Glossary []TermRelation` (JSON `glossary`, `omitempty`), populated by projection from all `glossary` blocks in file order.
- `Left`/`Right` hold the raw term-node text (bare or domain-qualified), lossless, consistent with how `context_map` stores raw endpoint text.

## 4. Grammar (internal/syntax)

- `glossary` is a contextual keyword (recognized by token value, usable as an ordinary identifier elsewhere), consistent with craft's keyword discipline.
- New `parseGlossaryBlock` mirroring `parseContextMapBlock`: optional domain-scope identifier, `{`, newline-framed term-relation statements, `}`. New `SyntaxKind` values for the block, the optional scope, and the relation statement, following the `context_map` kinds.
- A `GlossaryDecl` typed AST view with `Domain()` (scope or `""`) and `Relations()` accessors, mirroring `ContextMapDecl`.
- The three verbs live in a single-sourced set (a `glossaryVerbs` table in `internal/syntax`, with an accessor), kept in sync with sema by a build-time test, exactly as `edgeKeywords` / `edge_meta.go` are. Because all three are symmetric, no role/class metadata table is needed (unlike `context_map`).
- Byte-for-byte lossless round-trip of the green tree is required, as everywhere in craft.

## 5. Resolution and validation (sema)

Term-relation sites are collected per file in `AnalyzeFile` (a new `Symbols.GlossaryRelations []GlossaryRelationSite`, mirroring `ContextMapEdges`) and resolved in `AnalyzeWorkspace` against the merged `WorkspaceSymbols`. This reuses the exact collect-then-resolve split the `context_map` resolution uses, and runs in the broken-fixture golden harness (`AnalyzeFile` -> `MergeWorkspaceSymbols` -> `AnalyzeWorkspace`).

**Resolving a term node:** split on `/`. The last segment is the term name (a free string, never validated, since nothing declares terms). The remaining segment(s) form a BC reference (bare `bc`, or `domain/bc`) resolved via the existing `resolveBCRef(ws, scopeDomain, bcRef)`. A term node's canonical identity for comparison is `resolvedBC + "/" + termName`.

| Code | Severity | When |
|---|---|---|
| `craft/sema/glossary-endpoint-not-bc` | error | the BC part resolves to a domain, service, or actor, not a bounded context |
| `craft/sema/glossary-unresolved-bc` | warning | the BC part resolves to nothing declared |
| `craft/sema/glossary-ambiguous-bc` | error | a bare BC name is a bounded context in two or more domains; qualify as `domain/bc/term` |
| `craft/sema/glossary-self-relation` | error | both sides resolve to the same term node (same BC and same term) |
| `craft/lint/glossary-redundant` | warning | the same unordered term-node pair is declared with the same verb more than once |
| `craft/lint/glossary-conflicting-relation` | warning | the same unordered term-node pair is declared `same_as` and also `distinct_from` or `contrasts` (asserting identity and difference at once) |

Notes:
- A one-segment (BC-less) term node such as `Invoice` has an empty BC reference, which resolves to nothing and therefore surfaces as `glossary-unresolved-bc` (warning). No separate shape diagnostic is added; the resolution path already covers it.
- The redundant and conflicting lints key on the **unordered** canonical term-node pair, so `billing/Invoice` and `re/billing/Invoice` for the same BC collapse correctly, and `A verb B` equals `B verb A`.
- No double-reporting: a node whose BC does not resolve gets only the resolution diagnostic, not redundant/conflicting checks.

## 6. Public API (pkg/craft)

- Export `TermRelation` (alias of `model.TermRelation`) alongside the existing `Edge` alias.
- Export `GlossaryVerbs() []string` returning the three verbs (all symmetric; no metadata struct needed). Additive only; no existing export changes.

## 7. tree-sitter (separate repo)

- Add a `glossary_block` rule to `tree-sitter-craft/grammar.js` mirroring `context_map_block`: optional domain scope, the three verbs, term-node refs (the existing `slug` rule already accepts multi-segment `a/b/c` paths, so term nodes parse without a new ref rule; verify). Add corpus cases, regenerate `src/`, update `queries/highlights.scm` if verbs are enumerated. Branch, do not push; the `CORPUS_VERSION` bump and push happen at the coordinated release, as with the `context_map` grammar work.

## 8. Docs, example, corpus

- New `docs/page/language/glossary.md` reference page (mirror `context-map.md`: the block, term-node identity, the three verbs, the six diagnostics, a worked example), wired into the VitePress sidebar (`docs/page/.vitepress/config.mts`) and cross-linked from `overview.md` and `context-map.md`.
- A `glossary` row in `CLAUDE.md`'s DSL keyword table plus a short example.
- A corpus fixture `testdata/corpus/08_glossary/relations.{craft,craftjson}` (scoped + shared blocks, bare + domain-qualified nodes, all three verbs), plus broken fixtures for the new diagnostics under `testdata/broken/`.

## 9. Testing

- Parser: round-trip + projection tests for scoped, unscoped, and merged blocks with bare and domain-qualified nodes.
- Sema: whitebox tests per diagnostic (each fires on its trigger and stays silent on the valid case), driven through the real `AnalyzeFile` -> `Merge` -> `AnalyzeWorkspace` pipeline, plus broken-corpus fixtures with `.diagnostics.json` goldens.
- Verb-sync build-time test (`syntax.glossaryVerbs` equals sema's set).
- Corpus golden + byte-for-byte round-trip for the new fixture.
- `pkg/craft` test for `TermRelation` + `GlossaryVerbs()`.

## 10. Relationship to the cross-validation lint

This is an independent subsystem from the `context_map` cross-validation lint (`docs/superpowers/specs/2026-07-16-context-map-cross-validation-lint-design.md`). They share no code beyond the resolution machinery both reuse. Per the user's direction, both are implemented in one planning cycle, but they are separable and neither gates the other.
