# context_map Cross-Validation Lint — Design

**Status:** approved (brainstorm 2026-07-16)
**Feature:** warn when a declared `context_map` strategic relationship contradicts the communication view craft infers from use-case `asks`/`notifies`.
**Follows:** the `context_map` DDD strategic-relationship view shipped in v2.12.0 (`docs/superpowers/specs/2026-07-16-context-map-ddd-relationship-patterns.md`). This is the deferred "Task 9" from that plan, re-scoped as an LSP/editor consistency lint.

---

## 1. Motivation

craft holds two views of how bounded contexts relate:

- **Communication view**, *inferred* from use-case bodies. `sync_action` (`A asks B to …`) and `async_action` (`A notifies E`) plus `domain_listen` triggers (`when X listens E`) describe who talks to whom at runtime.
- **Strategic view**, *declared* in `context_map`. The eight DDD patterns classify each bounded-context relationship (customer_supplier, conformist, anticorruption_layer, open_host_service, published_language, partnership, shared_kernel, separate_ways), with the convention LEFT = upstream.

The strategic classification is deliberately non-inferable: it encodes team/model coupling and influence direction, not just calls. Its payoff is exactly that the two views can be **cross-checked**. When a declared relationship contradicts observed communication, one of them is wrong, and that is worth flagging.

The communication view is always **partial**, because not every relationship is exercised by a modeled use-case. The governing principle is therefore: **absence of communication never warns**. Only observed communication that *contradicts* a declared pattern (or is entirely unclassified) produces a diagnostic.

## 2. The core abstraction: the dependency edge

Both communication primitives reduce to a single directed **dependency edge `D → U`**, read as "downstream depends on upstream":

| Primitive | Source (`Action`/`Trigger`) | Dependency edge |
|---|---|---|
| sync | `X asks Y to …` (`sync_action`: `Context=X`, `TargetContext=Y`) | `X → Y` (asker depends on asked) |
| async | `Y notifies E` (`async_action`: `Context=Y`, `Event=E`) paired with `when X listens E` (`domain_listen` trigger: listener `X`, `Event=E`) | `X → Y` (listener depends on publisher) |

This unification is the design's keystone: **every directional DDD pattern expects the same thing**, namely that the downstream endpoint depends on the upstream one. With LEFT = upstream, the expected edge is `RIGHT → LEFT`. A `customer_supplier` customer calls its supplier (`RIGHT → LEFT`); a `published_language` publisher notifies its consumers, so the consumer/listener depends on the publisher (`RIGHT → LEFT`). No per-pattern special-casing is needed. Symmetric patterns (`partnership`, `shared_kernel`, `separate_ways`) have no expected direction.

## 3. The four signals

All four are lint-namespaced (`craft/lint/*`), consistent with the existing cross-file lints (`dead-event`, `unused-actor`, `redundant-relationship`).

| Code | Severity | Fires when |
|---|---|---|
| `craft/lint/separate-ways-violation` | `warning` | a `separate_ways A B` edge exists, yet *any* dependency edge is observed between A and B (either direction, sync or async). `separate_ways` asserts intentional non-integration, so observed communication directly contradicts it. |
| `craft/lint/relationship-direction-inverted` | `warning` | a directional pattern edge, and the *only* observed dependency between its endpoints runs `LEFT → RIGHT` (upstream depends on downstream), purely inverted, with no correct-direction edge present. |
| `craft/lint/relationship-bidirectional` | `hint` | a directional pattern edge with observed dependency in *both* directions. Message nudges: "bidirectional traffic under an asymmetric pattern, consider `partnership`?" |
| `craft/lint/unclassified-communication` | `hint` | a bounded-context pair that clearly communicates (at least one dependency edge) has *no* `context_map` edge at all in any block. A "you may want to classify this" nudge. |

### Severity rationale

craft has no lint enable/disable configuration. `LintWorkspace` runs every lint unconditionally, from both the LSP in `internal/workspace` and the public `pkg/craft` API. Severity is therefore the noise lever: editors surface `warning` prominently and `hint` subtly. The two high-confidence contradictions (R1, R2a) are `warning`; the two softer completeness signals (R2b, R3) are `hint`.

### Diagnostic ranges

- R1, R2a, and R2b anchor on the offending `context_map` edge (its left endpoint token), since that is the declaration the author would edit.
- R3 has no `context_map` edge to point at, so it anchors on the first observed communicating action between the pair (the `Action` line/col).

## 4. Matching and resolution

- **Identity resolution.** Communication subjects (`Action.Context`, `Action.TargetContext`, and the publisher/listener contexts of async pairs) are resolved to a canonical bounded-context identity using the same workspace-symbol machinery the resolution lints use (`resolveBCRef` against `WorkspaceSymbols`), so bare `billing` and qualified `re/billing` compare equal, and a scoped `context_map` block's bare endpoints resolve within their domain scope.
- **Skip non-BCs.** An endpoint or subject that does not resolve to a bounded context is skipped. Those already receive their own diagnostics (`unresolved-bc`, `edge-endpoint-not-bc` from the shipped resolution pass), so the cross-validation lint never double-reports resolution failures.
- **Async pairing.** `notifies E` to `listens E` matching reuses the event-pairing idiom already implemented in `lintDeadEvents` (event value equality across the workspace).
- **Pair keys.** R1 and R3 compare the **unordered** BC pair; R2a and R2b compare the **ordered** (upstream, downstream) pair against the observed dependency direction(s).
- **Self-pairs.** An edge whose endpoints resolve to the same BC is skipped (it is already a `self-relationship` error).

## 5. Architecture

- A new `lintContextMapConsistency` function in `internal/sema/lint.go`, invoked from `LintWorkspace` alongside the existing `lintDeadEvents`, `lintUnusedActors`, and `lintEventPastTense`. `LintWorkspace` already receives the per-file syntax trees, the merged `WorkspaceSymbols`, and per-file line indices, which is everything the lint needs.
- It walks use-case scenarios once to build two adjacency structures over resolved BC identities: the set of sync dependency edges, and the set of async (publisher, event, listener) dependency edges (via the shared event-pairing helper). It then iterates every `context_map` edge across all blocks and every communicating pair, emitting the four signals per the rules above.
- No changes to `AnalyzeFile`, `AnalyzeWorkspace`, or the broken-fixture golden harness. Those cover hard resolution diagnostics; these soft lints live in the lint pass and are covered by `LintWorkspace` unit tests (the same place `dead-event` is tested).
- The lint surfaces automatically in the editor (LSP, via `internal/workspace`) and in `pkg/craft`'s diagnostics, because both already call `LintWorkspace`. No new public API.

## 6. False-positive guards (explicit)

1. **Partial-view principle.** No observed communication between a pair means no diagnostic for that pair. The lint never says "you failed to model this relationship."
2. **R2a purity.** Direction-inverted fires only when *every* observed dependency between the endpoints runs the wrong way. Any correct-direction edge silences it; mixed traffic degrades to the softer R2b hint rather than a warning.
3. **No double-reporting.** Unresolved and non-BC endpoints and self-pairs are skipped; they are handled by the shipped resolution diagnostics.
4. **Symmetric patterns.** `partnership`, `shared_kernel`, and `separate_ways` are never subject to direction rules (only `separate_ways` participates, via R1).

## 7. Testing

- `internal/sema` `LintWorkspace` unit tests, one focused case per signal:
  - R1: `separate_ways` plus a use-case where one endpoint asks the other yields one `separate-ways-violation`; the same pair with no communication stays silent.
  - R2a: `customer_supplier A B` with only `A asks B` (upstream asks downstream) yields one `relationship-direction-inverted`; with `B asks A` present it stays silent.
  - R2b: the same pattern with both `A asks B` and `B asks A` yields one `relationship-bidirectional` hint and no direction-inverted warning.
  - R3: a communicating pair with no `context_map` edge yields one `unclassified-communication` hint; a classified pair stays silent.
  - Async coverage: at least one rule exercised via `notifies`/`listens` rather than `asks`, proving the dependency edge is built from events too.
  - A "consistent, fully-classified model" fixture asserting **zero** cross-validation diagnostics (guards against noise on well-formed input).
- No corpus-golden or tree-sitter changes (this is pure sema lint behavior, with no grammar surface).

## 8. Out of scope

- `partnership` and `shared_kernel` validation (not observable from `asks`/`notifies`).
- Any lint enable/disable/severity-override configuration system (that would be a separate feature; severity is the only noise lever here).
- Tree-sitter and `pkg/craft` API changes (none needed).
- The future `glossary { }` block for term relations (separate spec, unrelated).
