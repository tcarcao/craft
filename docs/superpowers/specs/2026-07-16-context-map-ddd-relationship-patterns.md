# Craft DSL — DDD strategic relationship patterns in `context_map`

**Date:** 2026-07-16
**Repo:** craft (`github.com/tcarcao/craft/v2`, `/Users/tiago.carcao/projects/poc/craft-project/craft`)
**Status:** spec — approved 2026-07-16; direction convention settled (LEFT = upstream). Ready to implement.
**Depends on:** the v2.11.0 `context_map` edge machinery (`edgeKeywords`, `classifyEdgeVerb`, default-case hardening)

## 1. Thesis

Add the **DDD strategic context-mapping patterns** (customer/supplier, conformist,
anticorruption layer, open-host service, published language, partnership, shared kernel,
separate ways) as first-class `context_map` **edge verbs** connecting two `bc:` endpoints.

They fit craft's existing model with zero new grammar shapes: `context_map` already parses
`<ref> <verb> <ref>` into `model.Edge{Left, Verb, Right}` and validates endpoint kinds per
verb category (`realized_by`/`also_realizes` ⇒ `bc:`→`service:`; `same_as`/`contrasts`/
`distinct_from` ⇒ `term:`→`term:`). This spec adds a **third category** — relationship
verbs ⇒ `bc:`→`bc:` — reusing `parseEdgeStmt`, the `edgeKeywords` list, and `classifyEdgeVerb`.

### 1.1 This reverses a prior "out of scope" decision — deliberately

`docs/superpowers/plans/2026-07-15-usecase-identity-tags-edge-hardening.md` listed
"DDD relationship-pattern verbs in `context_map` (`customer_supplier`, etc.)" as **out of
scope**, with the rationale that these patterns are descriptive/organizational and should
stay a **consumer-side extension** (the RE knowledge-hub currently parses them from a
`bc-relationships.craft` file via its own regex). We are reversing that: the sole consumer
(the hub) is the DSL's author, and keeping a parallel regex grammar in the consumer is
strictly worse than a governed keyword set in craft — one grammar, one validator, one
tree-sitter highlight path, reusable by any future consumer. The counter-argument from §4
of the vNext spec (craft validates "typed, mechanically-checkable edges" only) still holds
and shapes the scope here: **craft validates endpoint *kinds* and shape, not the semantic
*correctness* of a modeled relationship** (it cannot know whether Billing really is
upstream of VAS). That boundary is preserved below.

## 2. Grammar

No new grammar rule. Relationship edges are ordinary `edge_stmt`s inside `context_map`:

```craft
context_map {
    // existing categories, unchanged:
    bc:re/billing realized_by service:billing-api
    term:billing/Invoice contrasts term:subscriptions/Invoice

    // NEW — strategic relationship patterns (bc: -> bc:):
    bc:re/billing customer_supplier    bc:re/vas
    bc:re/vas     anticorruption_layer bc:re/billing
    bc:re/subscriptions partnership    bc:re/billing
}
```

Both endpoints must be `bc:` slugs (`bc:<domain>/<name>`, per the existing slug-shape rule).

## 3. The verb catalog & directionality

Eight verbs, in two classes. **Directionality is the one load-bearing convention** (see §3.1).

| Verb | Class | Left role | Right role |
|---|---|---|---|
| `customer_supplier` | directional | upstream = supplier | downstream = customer |
| `conformist` | directional | upstream | downstream conforms |
| `anticorruption_layer` | directional | upstream | downstream (owns the ACL) |
| `open_host_service` | directional | upstream (provides OHS) | downstream |
| `published_language` | directional | upstream (publishes PL) | downstream |
| `partnership` | symmetric | — | — |
| `shared_kernel` | symmetric | — | — |
| `separate_ways` | symmetric | — | — |

`big_ball_of_mud` is intentionally **excluded** — it is a boundary/zone designation, not a
pairwise edge (see §11).

### 3.1 Direction convention — LEFT = upstream, RIGHT = downstream

For every **directional** verb, the edge reads **`bc:<upstream> <verb> bc:<downstream>`** —
the standard context-map diagram reading (arrow points upstream → downstream). This is
uniform across all five directional verbs regardless of whether the pattern names an
upstream role (OHS, PL) or a downstream role (conformist, ACL, customer): the *slot* is
fixed (left=upstream), the *verb* names the pattern.

For **symmetric** verbs, endpoint order carries no meaning; `A partnership B` ≡ `B partnership A`.

> **Decision (settled 2026-07-16): LEFT = upstream.** Chosen for diagram fidelity and
> uniformity across all five directional verbs, over the LEFT = downstream alternative (which
> reads more naturally for downstream-role verbs like `conformist`/ACL but inverts for
> OHS/PL). This convention is encoded once in the verb-metadata table (§4) so consumers read
> direction from craft and never re-derive it. Every directional edge reads
> `bc:<upstream> <verb> bc:<downstream>`.

## 4. Model & exported API

`model.Edge{Left, Verb, Right}` is **unchanged** — the verb already carries the pattern.
Add exported classification so consumers (the hub, docs, tooling) read verb semantics from
craft instead of hard-coding them, mirroring the existing `syntax.EdgeKeywords()` accessor:

- In `internal/syntax/parser.go`, extend `edgeKeywords` (the single source of truth) with the
  eight new verbs.
- Add a verb-category/metadata table. Recommended home: a small exported surface in
  `pkg/craft` (so it ships with the stable API), e.g.:

```go
// pkg/craft (new: edges.go)

type EdgeCategory string
const (
    EdgeRealization  EdgeCategory = "realization"   // bc: -> service:
    EdgeTermRelation EdgeCategory = "term_relation" // term: -> term:
    EdgeRelationship EdgeCategory = "relationship"  // bc: -> bc:  (NEW)
)

// EdgeVerbInfo describes one context_map edge verb.
type EdgeVerbInfo struct {
    Verb       string
    Category   EdgeCategory
    Symmetric  bool // true => endpoint order is meaningless (partnership/shared_kernel/separate_ways)
    LeftKind   string // "bc" | "service" | "term"
    RightKind  string // "bc" | "service" | "term"
}

func EdgeVerbs() []EdgeVerbInfo        // all verbs across all categories
func LookupEdgeVerb(verb string) (EdgeVerbInfo, bool)
```

This lets the hub map a parsed edge to `{Upstream, Downstream, Bidirectional}` with **no
verb list of its own**: `Symmetric ⇒ Bidirectional`; else `Upstream=Left, Downstream=Right`
under the §3.1 convention. Keep `internal/syntax`’s existing `edgeKeywords` as the parser’s
gate and have the `pkg/craft` table derive from / stay in sync with it (add a sync test,
matching how the tags plan added the `EdgeKeywords()` sync test).

## 5. Parser changes (`internal/syntax/parser.go`)

- Add the eight verbs to `edgeKeywords`. That alone makes `isEdgeKeyword` accept them,
  `parseEdgeStmt` parse them, and the "expected an edge keyword (…)" diagnostic list grow —
  **update that diagnostic message** (it currently enumerates the five verbs) to avoid a
  stale hint. Consider grouping the message by category so it doesn't become an unreadable
  8-item list (e.g. "an edge keyword: a realization (realized_by/also_realizes), a term
  relation (same_as/contrasts/distinct_from), or a relationship pattern
  (customer_supplier/conformist/…)").
- No changes to `parseEdgeEndpoint` — `bc:` endpoints already parse (they were always legal
  ref shapes; only sema constrained which verb allows which kind).

## 6. Sema validation (`internal/sema/validate.go`)

Add a third verb→endpoint-kind map beside `edgeRealizationVerbs` and `edgeTermVerbs`:

```go
// edgeRelationshipVerbs require a bc: left endpoint and a bc: right endpoint.
var edgeRelationshipVerbs = map[string]bool{
    "customer_supplier": true, "conformist": true, "anticorruption_layer": true,
    "open_host_service": true, "published_language": true,
    "partnership": true, "shared_kernel": true, "separate_ways": true,
}
```

Extend `classifyEdgeVerb` with the relationship branch: **both endpoints must be `bc:`**;
otherwise emit `craft/sema/edge-endpoint-kind` (reuse the existing endpoint-kind diagnostic
code/shape used by the realization and term branches — keep it uniform). The v2.11.0
default-case (unknown-verb error) now never fires for these verbs since they're in
`edgeKeywords`.

Add two **relationship-specific checks** (both warning-level unless noted):

1. **Self-relationship** — `bc:X <verb> bc:X` is meaningless. `craft/sema/self-relationship`,
   **error** (a genuine authoring bug, cheaply provable). Applies to all relationship verbs.
2. **Redundant symmetric duplicate** — the same unordered pair declared twice with the same
   symmetric verb (e.g. `A partnership B` and `B partnership A`). `craft/lint/redundant-relationship`,
   warning. (Directional duplicates in opposite orders are *not* redundant — they may model a
   genuinely different-direction relationship — so only flag symmetric verbs.)

**Explicitly NOT validated** (preserves the §1.1 boundary — craft checks shape, not truth):
no check that a directional relationship's direction is "correct", no check for conflicting
patterns on the same pair (e.g. both `conformist` and `partnership`), no completeness/
"every BC pair must have a relationship" check. Those are modeling judgments; if the hub
wants them, they live hub-side as cross-validation.

## 7. Diagnostics summary

| Code | Severity | When |
|---|---|---|
| `craft/sema/edge-endpoint-kind` (existing) | error | a relationship verb with a non-`bc:` endpoint |
| `craft/sema/self-relationship` (new) | error | `bc:X <relationship-verb> bc:X` |
| `craft/lint/redundant-relationship` (new) | warning | same unordered pair + same symmetric verb twice |

All flow through the existing `internal/sema` path and the `testdata/broken/*.diagnostics.json`
harness (see §9).

## 8. tree-sitter grammar sync (`tree-sitter-craft`)

Following the vNext Slice F / tags-plan precedent:

- If the grammar enumerates edge verbs as literals, add the eight; if it treats the verb as
  a generic identifier slot validated downstream, no grammar change is needed — **verify
  which** in `grammar.js` (the `edge_stmt` / edge-verb rule) before editing.
- `queries/highlights.scm`: ensure the new verbs highlight as edge keywords (extend the
  existing edge-keyword capture list if it's an explicit set).
- `test/corpus/*`: add cases covering a directional and a symmetric relationship edge.
- Regenerate `src/*`; keep the 12/12 corpus baseline green (+ the new cases).
- `craft-vscode-extension`: only if it carries a separate TextMate keyword list (not
  tree-sitter-query-driven) — add the verbs there too; otherwise nothing to do.

## 9. Test plan

- `testdata/corpus/**/` — add `context_map` fixtures with relationship edges (a directional
  `customer_supplier`/`anticorruption_layer`, a symmetric `partnership`), plus their
  regenerated `.craftjson` goldens. Confirm `Edge{Left, Verb, Right}` round-trips.
- `testdata/broken/` — `relationship_bad_endpoint.{craft,diagnostics.json}` (a `term:` or
  `service:` endpoint on a relationship verb ⇒ `edge-endpoint-kind`);
  `relationship_self.{craft,diagnostics.json}` (`bc:X partnership bc:X` ⇒ `self-relationship`).
- `internal/sema/validate_test.go` — table cases for `classifyEdgeVerb` covering each new
  verb (bc→bc ok; wrong-kind error; self-relationship error), and the symmetric-duplicate lint.
- `pkg/craft` — a sync test asserting `EdgeVerbs()` / `edgeRelationshipVerbs` /
  `edgeKeywords` agree (no verb in one set missing from another), mirroring the tags plan's
  `EdgeKeywords()` sync test.

## 10. Consumer migration (the RE knowledge-hub) — informational, not part of this craft change

Once craft ships this, the hub (`svc-re-go-arch-context`,
`internal/infrastructure/graphstore`) deletes `bc-relationships.craft` + its regex
(`reBCRelationshipsHdr`, `reBCRelationshipEdge`, `loadBCRelationships`) — the last non-craft
parsing in the loader. Relationship edges move into the central `context_map.craft`, parsed
by `craft.Parse`. The hub maps each `Edge` via craft's `LookupEdgeVerb`:
`Symmetric ⇒ contextMapEdge.Bidirectional`; else `Upstream=Left, Downstream=Right`;
`Patterns = [verb]`. Note the modeling shift: the hub's old syntax allowed a bracket list of
patterns per edge (`billing -> vas [customer_supplier, published_language]`); craft models
**one pattern per edge statement** (clearer, individually diagnosable) — a pairing like OHS+PL
becomes two statements. `readLines`/`numberedLine` in the hub loader can then also go if
nothing else uses them.

## 11. Out of scope (name them so they aren't silently assumed)

- **`big_ball_of_mud`** — a zone/boundary marker over one-or-more BCs, not a pairwise edge;
  needs a different construct (a BC annotation or a named region), deferred.
- **Multiple patterns per edge statement** — modeled as multiple statements (§10); no
  bracket-list grammar.
- **Role sub-annotations** (e.g. naming the ACL, or the published language) — deferred; the
  verb is the whole payload for v1.
- **Semantic-correctness / conflict / completeness validation** — modeling judgment, not
  craft's job (§6).

## 12. File-by-file change list

- `internal/syntax/parser.go` — extend `edgeKeywords` (+8); update the "expected an edge
  keyword" diagnostic message (categorized).
- `internal/sema/validate.go` — `edgeRelationshipVerbs` map; relationship branch in
  `classifyEdgeVerb` (bc→bc); `self-relationship` check; `redundant-relationship` symmetric lint.
- `internal/sema/validate_test.go` — table cases for the new verbs + lints.
- `pkg/craft/edges.go` (new) — `EdgeCategory`, `EdgeVerbInfo`, `EdgeVerbs()`,
  `LookupEdgeVerb()`; the §3 verb-metadata table (direction convention baked in here).
- `pkg/craft/edges_test.go` (new) — sync test (`EdgeVerbs` ↔ `edgeKeywords` ↔ sema maps).
- `testdata/corpus/**/context_map_relationships.{craft,craftjson}` — goldens.
- `testdata/broken/relationship_bad_endpoint.*`, `testdata/broken/relationship_self.*`.
- `CHANGELOG.md` — new minor (v2.12.0): "context_map DDD strategic relationship patterns".
- `tree-sitter-craft`: `grammar.js` (if verbs are literal), `queries/highlights.scm`,
  `test/corpus/*`, regenerated `src/*`.
- (consumer, separate repo/PR) hub: delete `bc-relationships.craft` + regex; author in
  `context_map.craft`; map via `LookupEdgeVerb`.

## 13. Suggested slicing

- **Slice 1 — core:** `edgeKeywords` + sema endpoint-kind branch + `self-relationship` +
  corpus/broken goldens + unit tests. (Shippable: authors can write relationship edges,
  craft validates shape.)
- **Slice 2 — API + lint:** `pkg/craft` verb-metadata/classification API + sync test +
  `redundant-relationship` lint. (Unblocks the hub to drop its regex.)
- **Slice 3 — grammar sync:** tree-sitter + highlights + vscode.
- **Slice 4 — release:** CHANGELOG, tag v2.12.0.
