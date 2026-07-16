# Craft DSL — `context_map` as the strategic relationship view (DDD context mapping)

**Date:** 2026-07-16
**Repo:** craft (`github.com/tcarcao/craft/v2`, `/Users/tiago.carcao/projects/poc/craft-project/craft`)
**Status:** approved 2026-07-16 (settled via design interview). Supersedes the earlier "add 8 verbs to context_map" draft of the same date.
**Clean slate:** no `.craft` files exist in the wild yet and there is no external consumer of the v2.11.0 `context_map` shape, so this redesign carries **no deprecation burden** — we change the block outright.

---

## 1. Thesis

`context_map` becomes craft's **strategic view** of bounded-context relationships: a set of statements that classify each BC↔BC relationship with a recognized **DDD strategic context-mapping pattern** (customer/supplier, conformist, anticorruption layer, open-host service, published language, partnership, shared kernel, separate ways).

Each statement is a plain triple — `<upstream> <pattern> <downstream>` — with bare bounded-context names (no `bc:` prefix), e.g.:

```craft
context_map re {
  billing customer_supplier vas
  billing open_host_service  subscriptions
  subscriptions conformist   billing
  subscriptions anticorruption_layer billing
  billing partnership        payments/ledger    // cross-domain: qualify the outsider
}
```

### 1.1 Two views, and why this one earns its place

craft already derives a **communication view** — *who calls whom* — from use-case interactions (`Action.Context → Action.TargetContext` for `asks`, `Context → Event` for `notifies`), and materializes it as C4 / domain-flow / sequence diagrams (`internal/visualizer/c4_relationships.go`, `domain.go`). That graph shows that two contexts talk.

It does **not** and **cannot** show *how they relate strategically*: "billing asks vas" says nothing about whether vas conforms to billing's model, wraps it behind an ACL, or they're partners. That characterization is a human modeling judgment — the one thing craft can't infer. **`context_map` supplies exactly that non-inferable layer.** It is not a call graph; it is the strategic overlay that makes the communication graph a *context map* in the DDD sense.

### 1.2 The payoff: cross-validation (later slice)

Because craft owns both views, it can cross-check them:
- classify `billing customer_supplier vas` and they do interact in a use case → consistent;
- declare `separate_ways` between two contexts that interact in UC-N → **contradiction, warn**;
- two contexts interact all over the use cases but carry no classification → **info/lint: "unclassified relationship."**

This is scoped to a later slice (§10), but it is the reason the block is worth its keep and it shapes the decisions below (endpoints must share the identity that use cases use for contexts).

### 1.3 What changed from the prior draft (and why)

| Prior draft | Now | Why |
|---|---|---|
| Add 8 verbs beside `realized_by`/term edges | `context_map` holds **only** BC↔BC relationships | realization is derivable; terms are a different concern — see below |
| `realized_by` / `also_realizes` (bc→service) kept | **Removed** | Redundant: a service already declares `contexts:` + `opslevel:`, so bc→service is derivable; the original vNext design itself said realization is "usually hub-derived, authored only for the multi-repo case" |
| term edges (`same_as`/`contrasts`/`distinct_from`) in `context_map` | **Moved out** to a future `glossary { }` block (§9) | Term relations are a vocabulary/translation concern, not context mapping; keeping them made the block heterogeneous and un-droppable-prefix |
| `bc:re/billing` typed slugs | bare `billing` / qualified `re/billing` | inside `context_map` every endpoint is a BC, so `bc:` is pure noise; `/` = node identity, `.` = events (unchanged) |
| single block | **repeatable + optionally domain-scoped** blocks | per-domain and shared maps; ergonomic bare names within a domain |

---

## 2. Grammar

### 2.1 Block

```
context_map_block := 'context_map' domain_scope? '{' NEWLINE* (edge_stmt NEWLINE*)* '}'
domain_scope      := IDENT            // optional domain name, e.g. `context_map re { … }`
edge_stmt         := bc_ref PATTERN bc_ref
```

- **Repeatable.** Any number of `context_map` blocks per file, and across the workspace; all edges merge into one context map (`CraftDoc.ContextMap []Edge`, blocks append — no model change needed for merging).
- **Optional domain scope.** `context_map re { … }` scopes bare endpoint names to domain `re`. `context_map { … }` (unscoped) is the shared/global map.

### 2.2 Endpoints (`bc_ref`)

```
bc_ref := IDENT              // bare name, e.g. `billing`
        | IDENT '/' IDENT    // domain-qualified, e.g. `re/billing`
```

- No `bc:` kind prefix — the block implies it.
- **`/` (slash) is the node-identity separator** (domain/bc), consistent with craft's existing slug system (`term:<bc>/<name>`, `service:` etc.). **`.` (dot) stays reserved for event refs** (`vas.VasApplied`); it must never denote a BC.
- Resolution (§6): bare names resolve within the block's domain scope (or globally for an unscoped block); qualify with `domain/` to cross a domain boundary or to disambiguate a colliding name.

### 2.3 Patterns (`PATTERN`)

A closed set of DDD-canonical keywords (§3). Each is a single identifier token (underscore-joined), matched contextually like the existing edge keywords — no new lexer machinery.

---

## 3. The pattern catalog & direction

**Eight patterns, community-recognized DDD names** (chosen over plainer verbs deliberately: recognizability for DDD practitioners, and the readability problem was the `bc:` noise, now gone).

### 3.1 Direction convention — **LEFT = upstream, RIGHT = downstream** (settled)

A pattern keyword is a relationship *label*, not a directional verb, so direction is a **convention, documented once and applied uniformly**: every directional statement reads `<upstream> <pattern> <downstream>`. This matches how a DDD context-map diagram is read (arrow points upstream→downstream). Consumers read direction from the metadata table (§4), never re-derive it.

| Pattern | Class | Left (role) | Right (role) |
|---|---|---|---|
| `customer_supplier`    | directional | supplier (upstream) | customer (downstream) |
| `conformist`           | directional | upstream (model owner) | conformist (downstream) |
| `anticorruption_layer` | directional | upstream | downstream (owns the ACL) |
| `open_host_service`    | directional | upstream (provides OHS) | downstream (consumer) |
| `published_language`   | directional | upstream (publishes PL) | downstream (consumer) |
| `partnership`          | symmetric | — | — |
| `shared_kernel`        | symmetric | — | — |
| `separate_ways`        | symmetric | — | — |

For **symmetric** patterns endpoint order is meaningless: `a partnership b` ≡ `b partnership a`.

`big_ball_of_mud` is intentionally **excluded** — it is a zone/boundary marker over one-or-more BCs, not a pairwise edge (§11).

### 3.2 One pattern per statement

A BC pair that is both open-host-service and published-language upstream becomes **two statements**, not a bracket list — clearer and individually diagnosable.

---

## 4. Model & exported API

`model.Edge{ Left, Verb, Right string }` is **retained as-is**; `Verb` carries the pattern keyword, `Left`/`Right` carry the (bare or qualified) BC references as authored. Direction/symmetry are **not** stored on the edge — they are properties of the verb, read from a metadata table.

Single source of truth in `internal/syntax` (importable by both `internal/sema` and `pkg/craft`, no cycle):

```go
// internal/syntax/edge_meta.go
type EdgeClass string
const (
    EdgeDirectional EdgeClass = "directional"
    EdgeSymmetric   EdgeClass = "symmetric"
)
type EdgeVerbMeta struct {
    Verb      string     // e.g. "customer_supplier"
    Class     EdgeClass  // directional | symmetric
    // For directional verbs the left endpoint is upstream, right is downstream.
    UpstreamRole   string // human label for the left role, e.g. "supplier"
    DownstreamRole string // human label for the right role, e.g. "customer"
}
func EdgeVerbMetas() []EdgeVerbMeta
func LookupEdgeVerbMeta(verb string) (EdgeVerbMeta, bool)
func EdgeVerbSymmetric(verb string) bool
```

`pkg/craft` re-exports a stable public surface wrapping it (`EdgeVerbInfo`, `EdgeVerbs()`, `LookupEdgeVerb()`), so consumers read pattern semantics from craft rather than hard-coding them. `internal/syntax.edgeKeywords` remains the parser gate; a sync test asserts `edgeKeywords`, the metadata table, and the sema membership maps all agree.

---

## 5. Parser changes (`internal/syntax`)

- **Replace `edgeKeywords`** with the eight relationship patterns. **Remove** `realized_by`, `also_realizes`, `same_as`, `contrasts`, `distinct_from` from `context_map` (term verbs relocate to `glossary`, §9; realization is removed outright). Verify no internal consumer depends on the removed verbs (the visualizer derives from use cases, not from these edges).
- **`context_map` gains an optional domain-scope identifier** after the keyword.
- **`edge_stmt` endpoints become `bc_ref`** (bare or `domain/name`), no `kind:` prefix. Reuse the existing slug parsing; drop the `bc:` requirement.
- Update the "expected an edge keyword" diagnostic to enumerate the eight patterns (grouped: directional vs symmetric).

## 6. Endpoint resolution & sema validation (`internal/sema`)

Resolution:
- In `context_map <d> { }`, a bare `name` resolves to BC `name` declared in domain `d`.
- In an unscoped `context_map { }`, a bare `name` must resolve to a BC that is **globally unique** across declared domains; if two domains declare that name, it is **ambiguous ⇒ error**, "qualify as `<domain>/<name>`".
- `domain/name` resolves explicitly.
- A ref that resolves to no declared BC ⇒ **warning** `craft/sema/unresolved-bc` (best-effort, mirrors the existing `unresolved-ref-local` philosophy — the file-set may be partial).

Validation:
| Code | Severity | When |
|---|---|---|
| `craft/sema/edge-endpoint-not-bc` | error | an endpoint resolves to a domain/service/actor, not a bounded context |
| `craft/sema/self-relationship` | error | `X <pattern> X` (same resolved BC both sides) |
| `craft/lint/redundant-relationship` | warning | same unordered pair + same **symmetric** pattern declared twice (directional duplicates in opposite order are NOT redundant) |
| `craft/sema/unresolved-bc` | warning | endpoint resolves to no declared BC |

**Explicitly NOT validated** (craft checks shape, not truth): whether a stated direction is "correct"; conflicting patterns on one pair; completeness. Those are modeling judgments (or the cross-validation lint, §10), not shape checks.

## 7. Diagnostics summary

All flow through `internal/sema` and the `testdata/broken/*.diagnostics.json` harness. Codes as in §6.

## 8. tree-sitter grammar sync (`tree-sitter-craft`)

- `context_map` rule: add the optional domain-scope identifier; endpoints become bare/`domain/name` refs (drop the `bc:` requirement); `edge_verb` becomes the eight relationship patterns (remove the five old verbs from this block).
- `queries/highlights.scm`: `(edge_verb)` already captures the whole node, so new verbs highlight automatically; verify.
- `test/corpus/*`: cases for a directional pattern, a symmetric pattern, a domain-scoped block, and a cross-domain qualified endpoint.
- Regenerate `src/*`; keep the corpus + full-craft-corpus compat green.
- `craft-vscode-extension`: update its keyword list only if it enumerates edge verbs separately.

## 9. `glossary { }` block — term relations' new home (sibling feature)

Term relations leave `context_map`. Their intended home is a dedicated `glossary { }` block with the same design grain — homogeneous endpoints (all terms), prefix dropped, canonical relation verbs:

```craft
glossary {
  billing/Invoice      same_as       subscriptions/Invoice
  ordering/order       distinct_from  offering/order
}
```

The **detailed glossary design is a separate spec**; this spec only records the decision (terms move out of `context_map`) and removes term verbs from the `context_map` grammar. If the glossary block is not built in the same cycle, term relations are simply unavailable until it is (acceptable — no files use them).

## 10. Cross-validation lint (later slice)

Derive the BC interaction graph from use-case `asks`/`notifies` (the visualizer already builds an equivalent) and cross-check classifications:
- `separate_ways` between interacting BCs ⇒ `craft/lint/contradicts-interaction` (warning).
- directional integration patterns (customer_supplier/conformist/ACL/OHS/PL) between non-interacting BCs ⇒ `craft/lint/unbacked-relationship` (warning, info-level).
- interacting BC pair with no classification ⇒ `craft/lint/unclassified-relationship` (info).

Warnings only — craft cannot assume the interaction graph is complete. Scoped to its own slice so the core ships first.

## 11. Broader schism (adjacent, note-and-defer)

use-case targets and `notifies`/`listens` events also use `bc:`/typed slugs today, while subjects use bare names. The tidy end-state is one context-identity notation (`domain/name`, bare when unambiguous) used **everywhere**. This spec adopts it for `context_map` and keeps the notation compatible for use cases to follow, but the use-case ref migration is **out of scope here** — a separate consistency pass.

## 12. Out of scope (named so they aren't silently assumed)

- `big_ball_of_mud` (zone marker, not a pairwise edge).
- Multiple patterns per statement / bracket lists (use multiple statements).
- Role sub-annotations (naming the ACL, the published language).
- Semantic-correctness / conflict / completeness validation (modeling judgment; see §10 for the lint that gets closest).
- Full `glossary` block design (§9) and use-case ref unification (§11) — separate specs.

## 13. Data model / migration

No `.craft` files and no external consumer exist, so realization and term edges are **removed outright** from `context_map`, no deprecation shim. `model.Edge` shape is unchanged (`Left`/`Verb`/`Right`); its values now hold bare/qualified BC names and relationship-pattern verbs. Regenerate all affected corpus `.craftjson` goldens.

## 14. Suggested slicing

1. **Core:** replace `edgeKeywords` with the eight patterns; `context_map` endpoints become bare/`domain/name` (drop `bc:`); remove realization/term verbs from the block; endpoint-kind + `self-relationship` validation; corpus + broken goldens; unit tests.
2. **Blocks + resolution:** optional domain scope; repeatable/merged blocks; bare-name resolution (scoped/global/ambiguous/unresolved); `redundant-relationship` lint.
3. **Exported API:** `internal/syntax` metadata table + `pkg/craft` `EdgeVerbs`/`LookupEdgeVerb` + sync test.
4. **Grammar sync:** tree-sitter (scope, endpoints, verbs) + highlights + corpus; vscode if needed.
5. **Cross-validation lint** (§10).
6. **Release:** CHANGELOG, tag (next minor).
7. *(sibling specs, not gated on this)* `glossary { }` block; use-case ref unification.

## 15. File-by-file (core + API)

- `internal/syntax/parser.go` — `edgeKeywords` → 8 patterns; `context_map` optional domain scope; `bc_ref` endpoints (no `bc:`); categorized diagnostic.
- `internal/syntax/edge_meta.go` *(new)* — metadata table + accessors (single source of truth).
- `internal/sema/validate.go` — endpoint-must-be-BC; `self-relationship`; `redundant-relationship`; bare-name resolution + `unresolved-bc`.
- `internal/sema/*_test.go` — table tests + sync test.
- `pkg/craft/edges.go` *(new)* + `edges_test.go` — public metadata API + sync test.
- `testdata/corpus/05_context_map/*` — goldens (directional, symmetric, scoped, cross-domain).
- `testdata/broken/relationship_*` — endpoint-not-bc, self, redundant, ambiguous-bare.
- `CHANGELOG.md` — next minor.
- `tree-sitter-craft/*` — grammar (scope/endpoints/verbs), highlights, corpus, regenerated `src/`.
