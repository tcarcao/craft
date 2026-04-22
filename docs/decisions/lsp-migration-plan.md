# LSP Migration Plan — Craft v0.1 Language Server

> **Status:** Plan accepted, implementation not yet started.
> **Owner:** Tiago Carção.
> **Date:** 2026-04-21 (rev. 2026-04-22: dual-path migration, corpus-first TDD, grammar-v2 deferral).
> **Implementer:** AI agent (decisions optimized for agent execution).

---

## TL;DR

Unify Craft's parser and language tooling around a **hand-written Go
parser**, migrated behind a dual-path switch so both parsers run side-by-side
during the transition. The VSCode extension becomes a thin TypeScript client
that spawns `craft lsp` as a subprocess. Tree-sitter is demoted from "LSP
parser" to "editor-ecosystem asset" (Neovim, Helix, Zed, GitHub syntax
highlighting) and stops participating in VSCode. ANTLR stays operational
throughout migration and is deleted in a separate, gated step (A5b) well
after v0.1 ships — **not** at the moment the hand-written parser lands. The
grammars are kept in sync via a shared acceptance corpus living in this repo,
with a differential harness comparing both parsers on every PR.

**Grammar scope is frozen at v1 (current ANTLR semantics) from A1 start
through v0.1 cut.** The separate `grammar-v2-refactor-plan.md` (breaking
changes to reserved words and `returns to` semantics) is **deferred to
post-v0.1**; it becomes far cheaper once the hand-written parser is the only
implementation. See [Deferred work](#deferred-work).

---

## Goal

Ship a `.craft` language server that:

1. Delivers a professional VSCode editing experience (diagnostics, outline,
   semantic coloring, hover, go-to-definition, folding) in v0.1.
2. Reuses the existing Go parsing/analysis investment without translating it
   to TypeScript.
3. Eliminates the current duplicated-grammar / duplicated-parser tax.
4. Produces a release pipeline an AI agent can run hands-off.

**Non-goals for v0.1:** completion, rename, find-references, formatting, live
diagram preview, VSCode Web, telemetry, incremental parsing.
(See [Anti-scope](#anti-scope).)

---

## Current state (what exists today)

| Repo | Role | Tech |
|------|------|------|
| `craft/` | CLI + diagram generator + HTTP server | Go, ANTLR4 grammar at `tools/antlr-grammar/Craft.g4` (228 lines), generated parser in `pkg/parser/`, visitors in `internal/parser/` |
| `craft-vscode-extension/` | VSCode extension | TypeScript client + Node.js LSP server; **also** uses ANTLR4 (TS-generated parser from the same `.g4`) **and** tree-sitter; has `CraftParser.ts`, `TreeSitterDiagnosticProvider.ts`, `TreeSitterCompletionProvider.ts`, `TreeSitterFormatterProvider.ts` |
| `tree-sitter-craft/` | Tree-sitter grammar | `grammar.js` (481 lines — larger than the ANTLR grammar), WASM-built, consumed by the VSCode LSP |

Resulting pain points:

- **Two grammars for one language** — the ANTLR `.g4` and the tree-sitter
  `grammar.js` drift whenever the language evolves.
- **Two parser runtimes** — ANTLR Go runtime in the CLI, ANTLR TS runtime +
  tree-sitter in the VSCode LSP server.
- **Duplicated LSP logic** across TS (in `craft-vscode-extension/server/`) and
  semantic logic (in `craft/internal/parser/`).
- **No shared semantic model** — cross-file symbol resolution, references,
  rename, etc. have no home today.

---

## Industry context (what other languages do)

| Approach | Examples | Our fit |
|----------|----------|---------|
| A — Hand-written parser + LSP in the language's own impl language, VSCode is a thin subprocess client | gopls, rust-analyzer, zls, clangd, terraform-ls, TypeScript (tsserver) | **Chosen** |
| B — Hand-written parser in TypeScript running in Node | TypeScript, Pyright | Rejected — throws away Go investment |
| C — Parser generator with multi-target codegen (ANTLR) | Our current state | Rejected — maintenance cost, performance, error-message quality |
| D — Tree-sitter as the *primary* parser everywhere | Nickel, some Neovim-first tools | Rejected — tree-sitter optimized for editors, not strict semantics |
| E — Go parser compiled to WASM, run in Node LSP | Rare | Deferred — only needed if we ship to VSCode Web |
| F — Dual grammar: semantic parser for the compiler, tree-sitter only for editor features | Roc, Nix | **Adopted for the tree-sitter side only** |

The chosen approach is **A + F combined**: Go owns the semantic parser and
the LSP; tree-sitter is kept solely as a non-VSCode editor asset.

---

## Decision summary

Every decision from the planning session, with the chosen answer and a
one-line rationale.

| # | Decision | Chosen | Rationale |
|---|----------|--------|-----------|
| Q1 | LSP runtime | Go binary subprocess, desktop-only, platform-specific VSIX bundled | Orthodox for new languages in 2026; reuses the Go parser natively |
| Q2 | Role of tree-sitter | Keep for Neovim/Helix/Zed/GitHub; not used in VSCode LSP | Path 2 — editor ecosystem value without VSCode coupling |
| Q2.5 | Parser technology | Hand-written recursive descent in Go; migrate now; delete ANTLR | What every mature compiler does (Go, Rust, TS, C#, Swift, Zig, Kotlin, Clang); better errors, faster, one fewer dep |
| Q3a | Reparse strategy | Full reparse, 150ms debounce | Files are tiny; incremental is over-engineering |
| Q3b | Error recovery | Island parsing at top-level block boundaries | Matches Craft's block-structured shape; 10-line resync loop |
| Q3c | Broken-state UX | Current parse for diagnostics; last-good per-block AST for semantic features | How rust-analyzer/gopls feel "always responsive" |
| Q3d | Diagnostics transport | Push (`textDocument/publishDiagnostics`) | Cheap diagnostics; push is universal |
| Q4a | Analysis scope | Project-wide implicit merge of all `.craft` files in workspace | Craft is a modeling language — one model spanning multiple files |
| Q4b | Symbol namespaces | Per-kind (actor / domain / service / …); lint warning on cross-kind name reuse | `User` the actor vs `User` the domain is natural in DDD |
| Q4c | Compute model | Per-file parse cache + full sema rebuild on change | 10–100ms; simpler than incremental query engines |
| Q4d | Package layout | `internal/{ast,lexer,syntax,sema,workspace,lint,lsp}` + stable `pkg/craft` | Matches gopls/rust-analyzer/tsc layout |
| Q4e | Resolved references | Side-table `sema.ResolutionMap`; AST stays pure data | AST remains serializable; no mutation-after-parse complexity |
| Q5 | Phase 1 feature set | diagnostics, documentSymbol, semanticTokens, hover, definition, foldingRange | Maximum perceived quality per implementation unit |
| Q6 | Execution model | Phase 0 spec artifacts first; tracks A→B→C→D; hard green-between-commits | Agent-optimized: tight red-green loops |
| Q7a | Corpus location | `craft/testdata/corpus/` | Spec lives with the authoritative parser |
| Q7b | Tree-sitter consumption | CI clones `craft` at pinned `CORPUS_VERSION` | Explicit, simple, no submodule pain |
| Q7c | Compatibility contract | Acceptance + highlighting-sufficiency (not structural equivalence) | Mechanically testable; no over-specification |
| Q7d | CI enforcement | Bidirectional; hard-block on tree-sitter side, warning-only on craft side | Catches drift at source without blocking craft releases |
| Q7e | Corpus size | Feature-matrix (~80–120 files); add fuzzing in Phase 2 | Deterministic coverage, documentation-by-example |
| Q8a | Binary packaging | Single `craft` binary with subcommands (`craft lsp`, `craft check`, …) | One artifact to build/sign/distribute |
| Q8b | Versioning | Extension version == binary version; tree-sitter independent; `CORPUS_VERSION` records coupling | Coherent with bundled-binary choice |
| Q8c | Release trigger | Manual `git tag` push; GoReleaser + GitHub Actions | Simplest, explicit, agent-safe |
| Q8d | Distribution channels | VSCode Marketplace **and** OpenVSX | Cursor/VSCodium/Gitpod users need OpenVSX |
| Q8e | Binary update UX | Bundled in VSIX; extension auto-update delivers it | No separate downloader state machine |
| Q8f | Pre-release channel | VSCode's native pre-release flag | Zero-maintenance early-access channel |
| Q9a | Log destination | stderr by default; `--log-file` opt-in | `vscode-languageclient` captures stderr automatically |
| Q9b | Logger | `log/slog` (stdlib) with JSON handler | No dependency, structured, agent-familiar |
| Q9c | LSP tracing | Implement `$/setTrace` + `$/logTrace` | Single most useful debugging tool |
| Q9d | Telemetry | None in v0.1 | Privacy-first; zero infrastructure |
| Q9e | Bug repro | Logs + `craft check --lsp-json` CLI mirror | Reuses `sema`; no protocol plumbing needed |
| Q9f | Crash recovery | Default `vscode-languageclient` restart + per-handler panic-recovery middleware | Orthodox + resilient |
| Q10 | Dual-path migration | Both parsers live side-by-side via `--parser={antlr,v2}`; A5 splits into A5a (flip default) and A5b (delete ANTLR); gates are human-judgment, not fixed durations | Hand-written parser can ship broken without taking down `craft` users; rollback is a flag flip |
| Q11 | Canonical output contract | `pkg/craft/CraftDoc` — parser-agnostic JSON shape emitted by both ANTLR (via adapter) and hand-written parser; frozen in P0.1 | Decouples AST shape (private) from the diff/snapshot contract (public); enables differential testing |
| Q12 | Corpus TDD shape | Corpus entry = `.craft` + `.craftjson`; broken entries = `.craft` + `.diagnostics.json`; goldens bootstrapped from ANTLR in P0.4 with human review, then frozen; new entries hand-authored first | Corpus-first TDD with a cheap bootstrap; goldens are the oracle, not ANTLR-live |
| Q13 | Differential harness | Two Go test suites under `internal/parser_diff/` (v2-vs-goldens + ANTLR-vs-goldens) + `craft diff-parsers` CLI subcommand; structural JSON equality, no tolerances; hard-blocks every PR | Without a diff harness, Q10's dual-path is unobservable |
| Q14 | `--parser` flag surface | CLI + HTTP server expose `--parser={antlr,v2}`; LSP is v2-only from C1 (no escape hatch) | LSP has new code paths (workspace, sema) ANTLR can't feed; partial escape hatch would be misleading |
| Q15 | Grammar-v2 refactor | Deferred to post-v0.1; added to Anti-scope; revisit trigger documented in [Deferred work](#deferred-work) | Compound migrations multiply risk; v2 becomes far cheaper once hand-written is the only parser |

---

## Dual-path operation and the differential harness

Both parsers work in parallel from P0.4 through A5b. The switch is the
`--parser={antlr,v2}` flag on the CLI and HTTP server (the LSP is v2-only;
see Q14).

**Canonical output: `pkg/craft/CraftDoc`.**
Every parser produces a `CraftDoc` — a parser-agnostic, JSON-serializable
representation of a parsed file. `CraftDoc` is the public API in `pkg/craft`;
internal AST shapes (ANTLR visitor types, `internal/ast/`) are private
implementation details of each parser. This contract is frozen in Phase 0
step P0.1 and marked experimental (`v0.x.x`) until v0.1 ships.

**Golden format: `*.craftjson`.**
Every corpus entry has a paired `.craftjson` file containing the expected
`CraftDoc` JSON. Goldens are bootstrapped in P0.4 by running ANTLR over
`testdata/corpus/` and human-reviewing the output before committing. The
goldens are the oracle from that moment on; both parsers answer to them.

**Two harnesses:**

1. **Harness A — v2 vs goldens.** `internal/parser_diff/v2_test.go`
   iterates `testdata/corpus/`, parses each file with the hand-written
   parser, asserts structural equality with the committed `.craftjson`.
   This is the TDD driver for Track A3.
2. **Harness B — ANTLR adapter vs goldens.**
   `internal/parser_diff/antlr_test.go` does the same for ANTLR via the
   P0.2 adapter. This is the drift guard on the reference implementation —
   prevents silent breakage of the `--parser=antlr` path during migration.

Both harnesses disappear at A5b together with ANTLR.

**CLI subcommand: `craft diff-parsers`.**
Wraps the same comparison logic for interactive debugging. When the agent
hits a diff, it runs `craft diff-parsers testdata/corpus/01_actors/foo.craft`
and gets both outputs plus a structural diff, without rebuilding the Go test
harness.

**Comparison rules.**
Structural JSON equality (order-insensitive maps, order-sensitive arrays).
No tolerance hooks — if positions disagree, that's a real bug.

**CI behavior.**
Both harnesses run on every PR in `craft/`. Either harness failing
hard-blocks the PR. Diff output is surfaced in the log and posted as a PR
comment.

---

## Target architecture

```
craft/                              ← Go repo; single binary, semantic source of truth
  cmd/
    craft/                          ← CLI: `craft lsp`, `craft check`, `craft visualize`,
                                      `craft diff-parsers`, … ; honors `--parser={antlr,v2}`
    server/                         ← HTTP server; honors `?parser={antlr,v2}` query param
  internal/
    ast/                            ← v2 AST — pure data types (JSON-serializable), private
    lexer/                          ← Hand-written scanner
    syntax/                         ← Hand-written recursive-descent parser → ast
    sema/                           ← Symbol tables, ResolutionMap, validation
    workspace/                      ← Multi-file store, per-file parse cache, URI index
    lint/                           ← (migrated from `internal/linter/`)
    lsp/                            ← LSP protocol handlers (v2-only; uses sema + workspace)
    visualizer/                     ← Diagram generator (unchanged)
    processor/                      ← File processing pipeline (accepts either parser's CraftDoc)
    parser/                         ← EXISTING ANTLR visitor layer (kept until A5b)
    parser_antlr_adapter/           ← P0.2 bridge: ANTLR visitor output → `pkg/craft.CraftDoc`
    parser_diff/                    ← Differential harness (Q13); deleted at A5b
  pkg/
    craft/                          ← Stable public Go API — `CraftDoc` canonical type (P0.1),
                                      marked experimental until v0.1, stable thereafter
  testdata/
    corpus/                         ← Acceptance corpus, subdirectoried by block kind
      01_actors/                    ←   `.craft` + `.craftjson` pairs
      02_domains/
      03_services/
      04_use_cases/
      05_arch/
      06_exposures/
      99_mixed/                     ← Cross-kind integration files
    broken/                         ← Intentionally broken files + expected
                                      `.diagnostics.json` (hand-authored, not bootstrapped)
  docs/
    decisions/lsp-migration-plan.md ← (this document)
    decisions/grammar-v2-refactor-plan.md ← Deferred to post-v0.1; see [Deferred work](#deferred-work)
    GRAMMAR.md                      ← EBNF spec (Phase 0 artifact P0.6)
    ARCHITECTURE.md                 ← Layered architecture + constraints (Phase 0 artifact P0.7)
    AGENT.md                        ← Operating rules for the implementing agent (Phase 0 artifact P0.8)
  .goreleaser.yml                   ← Cross-compile 6 targets on tag push

craft-vscode-extension/             ← Thin TS client only; server/ directory deleted
  client/                           ← ~100 LOC; spawns `craft lsp --stdio` via vscode-languageclient
  .github/workflows/release.yml     ← Per-platform VSIX → Marketplace + OpenVSX

tree-sitter-craft/                  ← Independent editor-ecosystem grammar
  grammar.js                        ← Maintained for highlighting sufficiency
  CORPUS_VERSION                    ← Pinned `craft` tag the grammar is compatible with
  scripts/fetch-corpus.sh           ← Pulls corpus from pinned craft ref
  .github/workflows/compat.yml      ← Hard-blocks if corpus doesn't parse cleanly
```

**Layer dependency direction (strict, enforced in `AGENT.md`):**

```
ast  ←  lexer  ←  syntax  ←  sema  ←  workspace  ←  lsp
                                   ←  lint    ←  cli
```

Never import `lsp` from anywhere but `cmd/`. Never import `workspace` from
`sema`. Keep `ast` a leaf (pure data).

---

## Implementation plan

Four tracks, mostly sequential. Each task is one PR-sized unit with its own
green tests. Agent-driven work runs in tight red-green loops.

### Phase 0 — Guardrail artifacts and the canonical-output bootstrap

Phase 0 is **sequenced**, not unordered. Each step has concrete prerequisites
on earlier steps. P0.1 through P0.5 together establish the corpus-first TDD
spec that Track A codes against; P0.6–P0.8 are the human-authored guardrails.

- [ ] **P0.1.** `pkg/craft/craftdoc.go` — define `CraftDoc` Go types + JSON
      schema; the parser-agnostic public API. Round-trip tests
      (`CraftDoc → JSON → CraftDoc`). Marked experimental
      (`// Experimental: stabilizes at v0.1`) until v0.1.
- [ ] **P0.2.** `internal/parser_antlr_adapter/` — thin bridge from the
      current ANTLR visitor output (`internal/parser/types.go:DSLModel`) to
      `pkg/craft.CraftDoc`. Does NOT modify visitors; purely additive.
      Golden round-trip tests on one reference file.
- [ ] **P0.3.** `testdata/corpus/` seeded with `.craft` files — sourced from
      `examples/`, relevant tree-sitter `test/corpus/*.txt` fixtures, and
      hand-crafted edge cases; organized into subdirectories per block
      kind (`01_actors/` through `99_mixed/`). Target: ~80–120 files.
- [ ] **P0.4.** Run ANTLR + adapter over every `testdata/corpus/*.craft`,
      write paired `*.craftjson` goldens. **Human review step**: Tiago
      audits every golden before commit. Any ANTLR quirks surfaced here are
      either (a) fixed and re-generated, or (b) documented in the file's
      nearby `README.md` as "known ANTLR behavior frozen as v1 spec."
- [ ] **P0.5.** `testdata/broken/` — hand-authored intentionally broken
      `.craft` files + expected `.diagnostics.json`. **Not** bootstrapped
      from ANTLR (whose error output is exactly what we're replacing).
      Covers: syntax errors at every block-kind boundary, missing
      terminators, unexpected tokens, deeply nested failures.
- [x] **P0.6.** `docs/GRAMMAR.md` — EBNF spec, written against the corpus.
      Corpus is authoritative on any conflict; GRAMMAR.md is prose+EBNF
      documentation, not a parallel source of truth.
      *(Drafted 2026-04-22 as part of S0. Awaiting Tiago review.)*
- [x] **P0.7.** `docs/ARCHITECTURE.md` — layer boundaries, interfaces,
      Q1–Q15 decisions distilled. Codifies the layer dependency direction.
      *(Drafted 2026-04-22 as part of S0. Awaiting Tiago review.)*
- [x] **P0.8.** `docs/AGENT.md` — operating rules, per-track task
      checklists, hard rules (never-do list), explicit red-green-refactor
      loop definition, green-between-commits rule.
      *(Drafted 2026-04-22 as part of S0. Awaiting Tiago review.)*

**Phase 0 exit criteria:** Harness B (ANTLR-vs-goldens; see Q13) is green
on CI; the goldens are the committed source of truth. At this point Track A
has a fixed TDD target.

### Track A — Parser migration

Red-green-refactor loop (per `docs/AGENT.md`): pick next unsupported corpus
file → run harness A (test fails red) → implement minimum parser code to
make it green → move to next file. One PR per block kind; each PR
progressively un-skips files in its `testdata/corpus/NN_kind/` subdirectory.

- [ ] **A0.** `internal/parser_diff/` — the differential harness from Q13.
      - [ ] Harness A: `v2_test.go` — v2 parser vs `*.craftjson` goldens.
            Starts empty (all files marked skipped) and un-skips as Track A
            progresses.
      - [ ] Harness B: `antlr_test.go` — ANTLR adapter vs same goldens.
            Required green from P0.4 exit onward.
      - [ ] `craft diff-parsers` CLI subcommand wrapping the same logic.
- [ ] **A1.** `internal/ast/` complete with JSON round-trip tests. AST shape
      is private; public contract is `pkg/craft.CraftDoc`. A1 includes the
      AST → `CraftDoc` projection function.
- [ ] **A2.** `internal/lexer/` — hand-written scanner; golden token-stream
      tests for the full corpus (`testdata/corpus/*.tokens` added
      alongside `*.craftjson` as part of A2 if and only if you find
      lexer-level regressions need snapshot coverage; otherwise skip and
      rely on parser-level harness A).
- [ ] **A3.** `internal/syntax/` — one block kind per PR. Each PR
      un-skips every file in its corpus subdirectory on harness A; each
      PR is merged only when harness A is green on that subdirectory AND
      harness B remains green everywhere.
  - [ ] A3a. actors (`actor`, `actors { … }`) — un-skips `corpus/01_actors/`
  - [ ] A3b. domains (`domain`, `domains { … }`) — un-skips `corpus/02_domains/`
  - [ ] A3c. services (`service`, `services { … }`) — un-skips `corpus/03_services/`
  - [ ] A3d. use cases (`use_case "…" { when … }`) — un-skips `corpus/04_use_cases/`
  - [ ] A3e. architecture blocks (`arch { presentation: … gateway: … }`) — un-skips `corpus/05_arch/`
  - [ ] A3f. exposures (`exposure X { to: …, contexts: …, through: … }`) — un-skips `corpus/06_exposures/`
  - [ ] A3g. cross-kind integration — un-skips `corpus/99_mixed/`
- [ ] **A4.** Island parsing + error recovery; `testdata/broken/` produces
      expected diagnostics with no cascading errors. Asserted by a third
      test suite comparing v2-emitted diagnostics to `*.diagnostics.json`.
- [ ] **A4.5.** **Visitor-test parity audit.** Scan
      `internal/parser/*_test.go` for assertions not covered by
      `testdata/corpus/`. Back-fill missing cases as new corpus entries
      (with goldens generated via P0.2 adapter + human review). This is the
      safety net that lets A5b delete the visitor tests en bloc.
- [ ] **A-bench.** `internal/syntax/bench_test.go` — full corpus parses
      cold <50ms, warm <5ms (documented in CI).
- [ ] **A5a.** **Flip default parser** from `antlr` to `v2`. Release v0.1.0.
      Gate (human sign-off by Tiago; no fixed duration):
      - A3a–A3g + A4 + A4.5 + A-bench all green
      - Harness A has been green on main for a stretch Tiago considers
        enough to trust (his call; the defaults are there to rule out noise)
      - Harness B still green
      - Tiago has run `craft visualize` on ≥3 real `.craft` files with
        `--parser=v2` and hand-verified diagram output matches
        `--parser=antlr`
      - `--parser=antlr` remains available as an escape hatch
- [ ] **A5b.** **Delete ANTLR.** Gate (human sign-off by Tiago; no fixed
      duration): v0.1.0 has been in users' hands long enough for Tiago to
      call it safe; no parser-regression issues open; no credible need for
      `--parser=antlr`.
      Removes:
      - [ ] `craft/antlr4/`, `craft/pkg/parser/`, `craft/tools/antlr-grammar/`
      - [ ] `antlr4-go/antlr/v4` from `go.mod`
      - [ ] `internal/parser/` (ANTLR visitor layer)
      - [ ] `internal/parser_antlr_adapter/` (no longer needed)
      - [ ] `internal/parser_diff/antlr_test.go` (harness B)
      - [ ] `--parser` CLI flag + HTTP server `?parser=` param
      - [ ] `craft diff-parsers` CLI subcommand
      - [ ] Existing `internal/parser/*_test.go` test files

### Track B — Semantic layer

- [ ] **B1.** `internal/workspace/` — multi-file store, parse cache, URI index; tests for open/change/close/save lifecycle.
- [ ] **B2.** `internal/sema/` — symbol collection (per-kind namespaces); corpus → expected symbol tables.
- [ ] **B3.** `internal/sema/` — `ResolutionMap`; every reference in corpus resolves correctly or emits expected "unresolved" diagnostic.
- [ ] **B4.** `internal/sema/` — validation rules: duplicate names (error), cross-kind name reuse (warning), invalid references (error); per-rule test corpus.
- [ ] **B5.** Existing `internal/linter/` merged under `sema` or adjacent; existing tests green.

### Track C — LSP server

- [ ] **C1.** LSP scaffold using `go.lsp.dev/protocol`: `initialize`/`initialized`/`shutdown`/`exit`; stdio transport; document lifecycle wired to `workspace`; panic-recovery middleware.
- [ ] **C2.** `textDocument/publishDiagnostics` with 150ms debounce; integration tests with a fake client.
- [ ] **C3.** `textDocument/documentSymbol` — corpus → expected outline snapshots.
- [ ] **C4.** `textDocument/semanticTokens` — actors/domains/services/usecases each get a distinct semantic type; corpus snapshots.
- [ ] **C5.** `textDocument/hover` — shows kind + declaration location; cursor-position test matrix.
- [ ] **C6.** `textDocument/definition` — cursor matrix.
- [ ] **C7.** `textDocument/foldingRange` — block-level folds; corpus snapshots.
- [ ] **C-trace.** `$/setTrace` + `$/logTrace` implemented and tested.

### Track D — VSCode client + release pipeline

- [ ] **D1.** Rewrite `craft-vscode-extension/client/` as a thin ~100 LOC TS extension that spawns `craft lsp --stdio` via `vscode-languageclient`.
- [ ] **D1b.** Delete `craft-vscode-extension/server/` entirely (ANTLR-TS generated parser, tree-sitter providers — all gone).
- [ ] **D2.** `.goreleaser.yml` in `craft/` builds 6 targets: darwin-x64, darwin-arm64, linux-x64, linux-arm64, win32-x64, win32-arm64.
- [ ] **D3.** Extension release workflow: on tag push, download matching binaries, package 6 per-platform VSIXs, publish to Marketplace + OpenVSX.
- [ ] **D4.** Dogfood: install in Cursor (OpenVSX path), confirm activation.
- [ ] **D5.** `tree-sitter-craft/CORPUS_VERSION` pinned; `tree-sitter-craft` CI clones `craft` at that ref and runs acceptance check (hard-blocks).
- [ ] **D6.** `craft` CI warning-only job: run `tree-sitter parse` with latest `tree-sitter-craft/main` on `testdata/corpus/`; post PR comment on failure.

---

## Phase 1 Definition of Done

v0.1.0 ships **when and only when** every checkbox above is ticked **and**:

1. CI green on `main` for 48 hours, no hot-fix commits.
2. Author has used the extension to edit a real `.craft` file for one full working day, zero P0 issues open.
3. `docs/CHANGELOG.md` written, release notes published.
4. Tags `v0.1.0` pushed in `craft` and `craft-vscode-extension`.
5. VSIXs visible in Marketplace **and** OpenVSX.
6. Fresh install on clean macOS + clean Linux succeeds without manual steps.

---

## Anti-scope

**These cannot sneak into v0.1.** If they feel tempting, file as Phase 2+ issues.

- `textDocument/completion`
- `textDocument/references`, `rename`, `codeAction`
- `textDocument/formatting`
- Live diagram preview (webview + re-render on change)
- Incremental parsing, Salsa-style query engines
- Record/replay bug reproducer
- Telemetry / crash reporting of any kind
- VSCode Web / WASM builds
- LSP binary download-on-activation (we bundle)
- New DSL features — language scope is **frozen** from A1 start to v0.1 cut
- **Grammar-v2 refactor** (see `docs/decisions/grammar-v2-refactor-plan.md`
  and [Deferred work](#deferred-work)) — explicitly not in v0.1

---

## Follow-on phases

| Phase | Headline | Target |
|-------|----------|--------|
| **1.5** | Live diagram preview (webview, auto-refresh on save) — Craft's marketing feature | ~2 weeks after v0.1 |
| **2** | Completion, find-references, rename, code actions, workspace-symbol | ~2–3 weeks |
| **3** | Formatting, code lens, inlay hints, document-link | ~1–2 weeks |
| **4** | Flagship: rename-preview diagram diff, model-impact visualization | Opportunistic |

---

## Risks and mitigations

| Risk | Mitigation |
|------|-----------|
| Hand-written parser migration takes longer than estimated | Dual-path `--parser={antlr,v2}` flag keeps ANTLR operational throughout; block-by-block migration behind harness A; each PR leaves repo green; ANTLR lives until A5b (post-v0.1) |
| Hand-written parser has subtle semantic drift from ANTLR | Harness A (`.craftjson` equality on every PR); goldens bootstrapped+human-reviewed in P0.4; A4.5 visitor-test parity audit back-fills corpus gaps before A5a |
| Reference (ANTLR) implementation drifts during migration | Harness B (ANTLR-adapter-vs-goldens) hard-blocks every PR — any ANTLR-side change that breaks the adapter surfaces immediately |
| Error messages from hand-written parser regress vs ANTLR | Golden-file diagnostic snapshots in `testdata/broken/` — any regression is a failing test; diagnostics are hand-authored (P0.5), not bootstrapped, so the bar is deliberately set higher than ANTLR's |
| `CraftDoc` (P0.1) public API turns out wrong-shaped mid-migration | Marked experimental until v0.1; regenerating goldens is mechanical (run ANTLR adapter over corpus, review diffs) |
| Tree-sitter grammar falls behind | Bidirectional CI; warning-only on craft side surfaces drift in the PR that caused it |
| Agent-authored parser has subtle bugs | Golden JSON snapshots for every corpus file; `go test -fuzz` in CI (light fuzz in Phase 1) |
| Marketplace / OpenVSX publish secrets expire | Documented in `AGENT.md` and release runbook; rotate annually |
| Platform-specific VSIX bloats to 100+ MB combined | Using **per-platform** VSIX (`vsce publish --target <platform>`) each user only downloads ~15 MB |

---

## Deferred work

Things intentionally pushed past v0.1. Each entry names what to revisit,
why it was deferred, and the trigger for picking it back up.

### Grammar v2 refactor

- **Reference:** `docs/decisions/grammar-v2-refactor-plan.md` (marked "Ready
  to implement" at time of that plan's writing).
- **Status at LSP-migration-plan time:** Deferred to **post-v0.1**.
- **Why deferred:**
  1. The LSP migration plan freezes language scope from A1 start to v0.1
     cut. Running a parser rewrite and a language-breaking refactor
     concurrently compounds risk and makes every failing test ambiguous
     ("did my parser break, or did I mean to break that?").
  2. Corpus-first TDD (Q12) requires the spec to be stable while the
     hand-written parser is being built. Moving the spec mid-rewrite
     defeats the forcing function.
  3. Grammar v2's breaking changes (Q1: `asks`/`notifies`/`returns`/
     `listens` hard-reserved; Q2: `returns to <target>` pinned; Q4:
     structural keywords removed from `identifier`) become **far cheaper**
     once the hand-written parser is the only implementation: it's one
     grammar to update (not ANTLR + hand-written), and the corpus
     expectations diff is a mechanical regeneration.
- **Pick-up trigger:** After v0.1.0 + A5b complete (ANTLR deleted).
- **Sequencing when picked up:**
  1. Update `docs/GRAMMAR.md` to the v2 target spec.
  2. Update `pkg/craft/CraftDoc` if v2 requires new node shapes (e.g.
     reserved-word diagnostics, inferred-target returns). Bump
     experimental marker if needed — `CraftDoc` will have stabilized at
     v0.1, so a v2 change is a public API bump.
  3. Update `testdata/corpus/` and `testdata/broken/` — add new
     corpus entries covering v2 semantics; revise existing entries whose
     expectations change.
  4. Migrate the hand-written parser block-by-block, same red-green loop
     as Track A3 — but now against the v2 corpus.
  5. Release as v0.2.0 with clear "breaking: reserved words" notes.
- **Watch-outs recorded during LSP migration:**
  - When updating `pkg/craft.CraftDoc`, check consumers in
    `internal/visualizer/`, `internal/processor/`, `internal/sema/`,
    `internal/lsp/`, and `cmd/server/` — these are the call sites that
    expect today's shape.
  - Any Craft files in `examples/` or user-provided test data that use
    `asks`/`notifies`/`returns`/`listens` or structural keywords
    (`arch`, `services`, `use_case`, etc.) as identifier names will need
    hand-migration — flag in release notes.

---

## Next steps

In order:

1. **Phase 0 sequenced bootstrap** (see [Phase 0](#phase-0--guardrail-artifacts-and-the-canonical-output-bootstrap)):
   - P0.1 → P0.2 → P0.3 → P0.4 (human review) → P0.5 → P0.6–P0.8.
2. **Human sign-off on Phase 0 exit** — Harness B green on CI, goldens
   hand-reviewed and committed. Nothing in Track A starts until this
   sign-off is explicit.
3. **Begin Track A** with A0 (differential harness skeleton) then A1 (AST
   types + `CraftDoc` projection).

Everything after point 3 is agent-executable with per-PR review.

---

## Cross-references

- **Grammar spec:** `docs/GRAMMAR.md` *(to be drafted in P0.6)*
- **Architecture spec:** `docs/ARCHITECTURE.md` *(to be drafted in P0.7)*
- **Agent operating rules:** `docs/AGENT.md` *(to be drafted in P0.8)*
- **Canonical output contract:** `pkg/craft/craftdoc.go` *(to be defined in P0.1; experimental until v0.1)*
- **Current CLAUDE.md:** `CLAUDE.md` *(will be updated to point at `docs/AGENT.md` after Phase 0)*
- **Current ANTLR grammar:** `tools/antlr-grammar/Craft.g4` *(kept through A5a; deleted in A5b)*
- **Tree-sitter grammar:** `../tree-sitter-craft/grammar.js` *(kept; independent release cadence)*
- **Grammar v2 refactor plan:** `docs/decisions/grammar-v2-refactor-plan.md` *(deferred to post-v0.1; see [Deferred work](#deferred-work))*

---

## Glossary

- **Island parsing** — error-recovery technique where the parser treats each top-level construct as an independent unit, re-syncing on known top-level keywords when a block fails to parse. Prevents cascading errors.
- **ResolutionMap** — side-table mapping AST reference sites to their resolved declaration symbols, populated by the name-resolution pass of `sema`.
- **Acceptance corpus** — set of valid `.craft` files that every parser (hand-written Go, tree-sitter, ANTLR during migration) must parse identically. The mechanical drift-detection contract.
- **Highlighting sufficiency** — weaker tree-sitter contract: the grammar must expose enough node structure for `queries/highlights.scm` to produce correct syntax highlighting, but tree shape need not match the hand-written parser's AST.
- **Path 2** (from Q2) — the chosen strategy where tree-sitter exists solely for non-VSCode editors (Neovim, Helix, Zed) and GitHub code view. Not used by the LSP.
- **`CraftDoc`** (Q11) — parser-agnostic, JSON-serializable representation of a parsed `.craft` file. Defined in `pkg/craft/craftdoc.go` and emitted by both parsers via their respective adapters. The public contract; internal AST shapes are private.
- **`.craftjson` golden** (Q12) — committed expected `CraftDoc` JSON output paired with each `testdata/corpus/*.craft` file. Bootstrapped via ANTLR in P0.4 with human review; frozen thereafter.
- **Dual-path** (Q10) — during migration, both parsers are compiled into the `craft` binary and selected via `--parser={antlr,v2}` on the CLI or `?parser={antlr,v2}` on the HTTP server. The LSP is v2-only (Q14).
- **Harness A / Harness B** (Q13) — the two differential test suites under `internal/parser_diff/`. A compares the v2 hand-written parser against `.craftjson` goldens; B compares the ANTLR adapter against the same goldens. Both hard-block PRs; both disappear at A5b.
- **A5a / A5b** (Q10) — A5a flips the default `--parser` from `antlr` to `v2` and releases v0.1.0. A5b is a later, separate PR that deletes ANTLR and the differential harness. Both gated on human sign-off, not fixed timers.
