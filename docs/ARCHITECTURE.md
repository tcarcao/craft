# Craft — Architecture (v0.1 migration target)

> **Status:** Draft — P0.7 deliverable of the LSP migration (`docs/decisions/lsp-migration-plan.md`).
> **Scope:** the Go package layout, layer rules, and runtime invariants that the hand-written parser + LSP must satisfy. Derived from source-plan decisions Q1–Q15 and tracer-bullets decisions Q1–Q23.
> **Enforcement:** `docs/AGENT.md` encodes the checks; CI mirrors them. A PR that violates these rules is not merge-ready.

---

## 1. Package layout

```
cmd/
  craft/                  CLI entry (subcommands: lsp, check, visualize, diff-parsers)
  server/                 HTTP diagram server
internal/
  ast/                    v2 AST — pure data, JSON-round-trippable, private
  lexer/                  hand-written scanner
  syntax/                 hand-written recursive-descent parser → ast
  sema/                   symbol tables, ResolutionMap, validation rules
  workspace/              multi-file store, per-file parse cache, URI index
  lint/                   (absorbs existing internal/linter in S9)
  lsp/                    LSP protocol handlers; depends on sema + workspace
  visualizer/             diagram generator (consumes CraftDoc)
  processor/              .craft file processing pipeline (consumes CraftDoc)
  parser/                 EXISTING ANTLR visitor layer — deleted in S12
  parser_antlr_adapter/   S1 bridge: ANTLR visitor output → pkg/craft.CraftDoc — deleted in S12
  parser_diff/            S1/S3 differential harness — deleted in S12
pkg/
  craft/                  STABLE PUBLIC API — CraftDoc canonical type
                          (experimental until v0.1; stable thereafter)
testdata/
  corpus/                 acceptance corpus (valid inputs + .craftjson goldens)
    01_actors/            ... 06_exposures/, 99_mixed/
  broken/                 hand-authored broken inputs + .diagnostics.json
  visualize-baseline/     pre-S1 PlantUML/SVG snapshots for regression check
tools/
  antlr-grammar/          Craft.g4 (v1) — deleted in S12
```

---

## 2. Layer-dependency direction (strict)

```
ast  ←  lexer  ←  syntax  ←  sema  ←  workspace  ←  lsp
                                   ←  lint    ←  cli
```

- Arrow = "may import." Inverse direction is forbidden.
- `ast` is a leaf. It has no imports from our own tree; only stdlib + encoding/json.
- `lexer` imports only `ast` token types (or its own token type — either choice is fine, but it never imports `syntax`).
- `syntax` produces `ast` nodes; MUST NOT import `sema`, `workspace`, or `lsp`.
- `sema` consumes `ast` (via projections) and produces `pkg/craft.CraftDoc` + a `ResolutionMap`. It MUST NOT import `workspace`.
- `workspace` is the only module that holds state per URI and orchestrates re-parse. It imports `syntax` and `sema`; not vice-versa.
- `lsp` may import `workspace`, `sema`, and `pkg/craft`. It is imported only by `cmd/`.
- `lint` is peer to `sema` — it may consume the AST and the `ResolutionMap` but not `workspace` or `lsp`.
- `visualizer` and `processor` depend only on `pkg/craft.CraftDoc` (and stdlib).

**Enforcement:** a CI check using `go list` + `go-cleanarch` (or a simple grep-driven script in `AGENT.md`'s toolbelt) rejects any import that crosses these arrows.

---

## 3. Public vs private API

| Surface | Stability | Owner |
|---------|-----------|-------|
| `pkg/craft.CraftDoc` (and all exported types in `pkg/craft/`) | Experimental until v0.1; stable thereafter. Backward-compatible changes only after v0.1. | Parser team |
| Every `internal/...` package | Private; may change in any PR. | Parser team |
| `tools/antlr-grammar/Craft.g4` | Frozen v1 through S11b. Deleted in S12. | — |
| LSP protocol surface (method set + capability set) | Incremental per-slice per Q20 (see §7). | LSP team |
| CLI flag set | `--parser` exists S1–S11b; deleted S12. Other flags are user-facing and follow semver from v0.1. | CLI team |

---

## 4. The canonical-output contract (`CraftDoc`)

Defined in `pkg/craft/craftdoc.go` at the start of S1 with coverage for **every** block kind (actors, domains, services, use cases, arch, exposures). Invariants:

- **JSON round-trip:** `CraftDoc → MarshalJSON → UnmarshalJSON → CraftDoc` is identity for every corpus file.
- **Deterministic serialisation:** maps use sorted keys; array order is load-bearing and preserved from source.
- **Position preservation:** every node carries `Start`/`End` as (line, column, byte-offset) triples, 1-indexed for display, 0-indexed for LSP translation.
- **Parser-agnostic:** both the ANTLR adapter and the hand-written parser emit the same shape. Internal AST shapes under `internal/ast/` are private and may be different.
- **Experimental until v0.1:** every exported type in `pkg/craft/` carries `// Experimental: stabilizes at v0.1`. Breaking changes during migration are permitted but must regenerate every `.craftjson` golden in the same PR.

---

## 5. The dual-parser runtime

Both parsers coexist from S1 to S12, selected per-process via:

| Entry point | Selector |
|-------------|----------|
| `cmd/craft` (CLI) | `--parser={antlr,v2}` flag; default flips `antlr → v2` in S11a |
| `cmd/server` (HTTP) | `?parser={antlr,v2}` query param on diagram-generation endpoints |
| `cmd/craft lsp` (LSP) | **v2-only**, no selector. Per Q14: the LSP has new code paths (workspace, sema) that ANTLR cannot feed. |

Rationale: a half-enabled LSP escape-hatch is misleading — a flag that selects "ANTLR" in the LSP would silently skip every handler that depends on `workspace`/`sema`.

The ANTLR visitor layer continues to produce its current types (`internal/parser/types.go:DSLModel`); the `internal/parser_antlr_adapter/` bridge projects those into `pkg/craft.CraftDoc`. The bridge is the reference implementation of the canonical output.

---

## 6. Panic-recovery tier matrix (Q17)

Parsing and analysis CAN panic (nil dereferences, bounds errors in hand-written code, buggy visitor migrations). The LSP MUST NOT crash the user's VSCode session on such a panic — the TypeScript client is entitled to assume the server keeps answering.

Four tiers of `recover()` are wired, each logging a structured `slog` event and surfacing on `$/logTrace`. Each tier's observable behaviour is documented below.

| Tier | Recovers | Lands in slice | On panic: degraded behaviour |
|------|----------|----------------|------------------------------|
| **Handler** | `*lsp.Server` method dispatch — every `textDocument/*` and `workspace/*` handler | S2 | The handler returns a properly-shaped "empty" response (`null`, `[]`, `false` depending on protocol). The panic is logged with file URI + method + stack trace. Client never sees a JSON-RPC error for this call. |
| **Parser** | `internal/syntax` recursive descent | S3 | The affected **top-level block** is discarded; the rest of the file continues parsing. A `craft/syntax/parser-panic` diagnostic is published with the block's range. Last-good AST for that block, if cached in workspace, is preserved for hover/outline. |
| **Lexer** | `internal/lexer.Next()` | S3 | The whole file is marked un-parsable for this pass. Workspace returns the last-good AST for the file (if any). Diagnostic `craft/lexer/lexer-panic` at line 1. |
| **Sema** | `internal/sema` resolution + validation | S4 | The file's AST stands; sema state is reset to "syntactic-only". Hover/definition/references return `null`. Outline (which is purely syntactic) keeps working. Diagnostic `craft/sema/sema-panic` at line 1. |

**Rules:**
- Recovery happens at the outermost call of each tier — inner code does not re-`recover()`.
- Every tier logs at `slog.LevelError` with structured fields: `tier`, `uri`, `range` (if known), `stack`.
- The dogfood checklist in S11a (tracer-bullets Q14) inspects `$/logTrace` for absence of tier panics; a single panic in a dogfood session is a P0 bug.

---

## 7. LSP `ServerCapabilities` policy (Q20)

**Rule:** every `ServerCapabilities` field the server advertises in its `initialize` response MUST correspond to a handler that is actually implemented. Conversely, implementing a handler without advertising the capability leaves VSCode's UI features dark.

The capability set is therefore grown **incrementally per slice**, never ahead of implementation:

| Slice | Capabilities added |
|-------|--------------------|
| S2 | `textDocumentSync` (open/change/close/save only; no handlers beyond lifecycle) |
| S3 | `documentSymbolProvider`, `hoverProvider` + diagnostic push (no capability needed for push) |
| S4 | `semanticTokensProvider` |
| S5 | `definitionProvider`, `executeCommandProvider` (for `EXTRACT_DOMAINS_FROM_CURRENT` and `EXTRACT_DOMAINS_FROM_WORKSPACE` per Q16) |
| S7 | `foldingRangeProvider` |

**AGENT.md encodes:** every PR that adds a handler MUST update the `initialize` response in the same diff, and include a unit test that asserts the capability appears.

---

## 8. Diagnostic-code policy (Q19)

Every LSP diagnostic emitted by the Craft server carries a stable `code` drawn from the namespace `craft/<category>/<slug>`. The free-form `message` field is allowed to evolve; goldens never assert on it.

### 8.1 Assertion surface

`testdata/broken/*.diagnostics.json` files assert on the tuple `(code, range, severity)` per file. Set-equality (no extra, no missing) holds the bar at CI time. Wording polish is verified during S11a dogfooding, not in CI.

### 8.2 Category list (initial, to be extended per-slice)

| Category | Landing slice | Severity default | Examples |
|----------|---------------|------------------|----------|
| `craft/lexer/*` | S3 | Error | `craft/lexer/unterminated-string`, `craft/lexer/lexer-panic` |
| `craft/syntax/*` | S3 | Error | `craft/syntax/unexpected-token`, `craft/syntax/missing-terminator`, `craft/syntax/unclosed-block`, `craft/syntax/parser-panic` |
| `craft/sema/*` | S3+ | Error (warn for cross-kind) | `craft/sema/duplicate-name` (S3), `craft/sema/cross-kind-name-reuse` (S4, **warn**), `craft/sema/unresolved-reference` (S5), `craft/sema/invalid-reference-target` (S5), `craft/sema/duplicate-use-case-name` (S6), `craft/sema/invalid-exposure-target` (S8), `craft/sema/sema-panic` (S4) |
| `craft/lint/*` | S9 | Info/Warn | (Scope defined when linter merges into sema; see P0.5.) |

### 8.3 Codebook (docs/DIAGNOSTICS.md)

`docs/DIAGNOSTICS.md` is seeded incrementally: each sema rule that lands in S3–S8 adds its entry; S9 consolidates and publishes the full table (stable identifier, example message prose, severity, slice-of-origin, URL anchor). The codebook is user-facing documentation; it is the source of truth for the set of codes the client may receive.

---

## 9. Workspace semantics

- **Model:** "open projects implicitly span every `.craft` file reachable from the workspace root(s)" — source-plan Q4a.
- **Indexing trigger:** `initialize` walks the workspace, finds all `.craft` files, and seeds the parse cache. Every subsequent `didOpen`/`didChange`/`didSave` replaces that file's cache entry.
- **Cache key:** `(uri, content-hash)`. Unchanged files retained across debounced re-parses reuse their AST without re-scanning. S5's performance gate (≤200ms keystroke-to-diagnostics on a 20-file workspace) depends on this cache being honest.
- **Compute model (Q21):** on every debounced change (150ms debounce — source-plan Q3a), run **full-workspace sema** from the current per-file ASTs. No incremental query engine. Rationale: Craft files are small, workspaces are small, and Salsa-style incrementality is source-plan Anti-scope.
- **Multi-file awareness:** present from S3 on, even though S3 uses only the single-file slice. Rationale per Q15: wiring cross-file resolution later would be a retrofit, which is more dangerous than having the index available from day one.

---

## 10. Error-recovery strategy (island parsing)

Source-plan Q3b mandates **island parsing at top-level block boundaries**. The hand-written parser implements this as:

1. Each top-level block production returns either a node or `nil` + a diagnostic.
2. On `nil`, the resync loop consumes tokens until the next top-level keyword (from the §2 anchor set in GRAMMAR.md) or EOF.
3. The outer file production continues from there.
4. No cascading errors: a failure inside `service_block` does not re-emit diagnostics for the `services_def` body or anything after.

This is not a fallback — it is the **primary** error path, and the one exercised by S9's `testdata/broken/*.craft` corpus.

---

## 11. Serialisation + determinism

- `pkg/craft.CraftDoc` JSON encoding MUST be deterministic: `encoding/json` default + `json.Marshaler` implementations for any type with map fields emit keys in sorted order.
- Structural JSON equality (Q13) is the comparison primitive in both diff harnesses. No tolerances, no "ignore whitespace" hooks.
- Array order is always load-bearing (source order matters for outline/folding).
- Position fields (`Start`, `End`) are serialized as `{"line": 1, "column": 1, "offset": 0}` tuples.

---

## 12. Examples-freeze policy (Q18)

- `examples/*.craft` originals are **frozen** through v0.1. They are user-facing documentation and appear in the VitePress site.
- S1 copies every example into `testdata/corpus/99_mixed/` with paired `.craftjson` goldens. The v2 parser conforms to those goldens.
- If a language-level issue in an example surfaces during migration, the fix lives in the corpus copy, not in `examples/`. `examples/` is touched again only at v0.2.0 (grammar v2 cut).

---

## 13. Decision-to-module map

For auditability, here is where each source-plan and tracer-bullets decision lives:

| Decision | Home |
|----------|------|
| Q1 Go subprocess LSP | `cmd/craft/lsp.go` |
| Q2 tree-sitter kept non-VSCode | `../tree-sitter-craft/`, `CORPUS_VERSION` |
| Q2.5 Hand-written parser | `internal/lexer`, `internal/syntax` |
| Q3a Full reparse + 150ms debounce | `internal/workspace` |
| Q3b Island parsing | `internal/syntax` resync loop |
| Q3c Last-good per-block AST | `internal/workspace` cache |
| Q3d Diagnostics push | `internal/lsp` handler (S3) |
| Q4a Project-wide sema | `internal/sema`, `internal/workspace` |
| Q4b Per-kind namespaces | `internal/sema` symbol tables (S3 actor, S4 domain, …) |
| Q4c Per-file cache + full sema | `internal/workspace` + `internal/sema` |
| Q4d Package layout | this doc §1 |
| Q4e ResolutionMap side-table | `internal/sema` |
| Q10 Dual-parser flag | `cmd/craft`, `cmd/server` (not `cmd/craft lsp`) |
| Q11 `CraftDoc` contract | `pkg/craft/craftdoc.go` |
| Q13 Differential harness | `internal/parser_diff` |
| Q15 Grammar v2 deferred | — (explicitly not in v0.1) |
| Q17 Panic-recovery tiers | this doc §6; wired in S2/S3/S4 |
| Q19 Diagnostic-code policy | this doc §8; consolidated in `docs/DIAGNOSTICS.md` |
| Q20 Capabilities-per-slice | this doc §7 |
| Q21 Eager full-workspace sema | `internal/sema`, `internal/workspace` |
| Q22 Processor rewired to CraftDoc | `internal/processor`, `internal/visualizer` in S1 |
| Q23 Sema rules per slice | `internal/sema` validation package; see `docs/DIAGNOSTICS.md` |

---

## 14. Cross-references

- **Grammar spec:** `docs/GRAMMAR.md`
- **Agent operating rules:** `docs/AGENT.md`
- **Canonical-output contract:** `pkg/craft/craftdoc.go` *(to be added in S1)*
- **Diagnostic codebook:** `docs/DIAGNOSTICS.md` *(seeded S3+, consolidated in S9)*
- **LSP migration plan:** `docs/decisions/lsp-migration-plan.md`
- **Tracer-bullet slicing:** `docs/decisions/lsp-migration-tracer-bullets.md`
- **Deferred grammar-v2 plan:** `docs/decisions/grammar-v2-refactor-plan.md`
