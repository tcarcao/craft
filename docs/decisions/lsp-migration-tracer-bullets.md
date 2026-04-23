# LSP Migration — Tracer-Bullet Implementation Plan

> **Companion to:** [`lsp-migration-plan.md`](./lsp-migration-plan.md) — that doc holds the *decisions*; this doc holds the *slicing*.
> **Status:** Draft, pending review.
> **Date:** 2026-04-22.

---

## Grilling decisions log

Decisions reached during the 2026-04-22 grill-me session, baked into the slices below.

| # | Branch | Decision | Effect on slices |
|---|--------|----------|------------------|
| Q1 | `CraftDoc` shape evolution | **Freeze full shape in S1** covering every block kind (ANTLR adapter already has the info). v2 parser grows; contract does not. | S1 deliverables expand; S3–S8 no longer risk contract churn |
| Q2 | Golden review cadence | **Drip-feed per slice.** S1 commits schema + one reference golden; S3–S8 each commit + human-review their own kind's goldens. | Source plan's single P0.4 review event dissolves into per-slice mini-reviews |
| Q3 | Contextual keywords (`asks`, `notifies`, `to`, `when`, `returns`, `listens`) | **Lex as plain identifiers; parser disambiguates by position.** Matches ANTLR semantics. Grammar-v2 hard-reserving is explicitly deferred per source plan anti-scope. | S0 `GRAMMAR.md` gains a contextual-keyword checklist section; S3+ lexer design constrained |
| Q4 | ~~macOS code signing (initial call)~~ | **Superseded by Q8** — we had not yet confirmed Apple Developer account availability. Signing deferred to S11b. | — |
| Q5 | Colouring before semantic tokens (S3) | **Superseded by Q6** — no TextMate grammar exists in the current extension; colouring strategy is revisited in Q6. | — |
| Q6 | (Supersedes Q5.) Colouring path | **Author a TextMate grammar in S2.** Port the existing tree-sitter highlighting regexes into a `.tmLanguage.json`. Tree-sitter-in-client deleted per source-plan Q2; semantic tokens (S4) layer on top of TextMate thereafter. | S2 gains a TextMate grammar deliverable (~200–400 lines JSON); `TreeSitterHighlightProvider.ts` deleted in S2 |
| Q7 | Existing client scope (3,366 LOC) | **Keep `providers/`, `commands/`, `services/`, `webview/`, `ui/` running against the existing HTTP `craft server` through v0.1.** Only `server/` (LSP TS code) and `TreeSitterHighlightProvider.ts` are deleted in S2. Diagram preview stays shipping. | Source plan D1's "~100 LOC thin client" is retired as a target; Phase 1.5 scope narrows to "migrate preview from HTTP to LSP" |
| Q8 | macOS code signing timing | **Defer: ship Linux-first through S10; signing slots into S11b.** macOS dogfooding uses `xattr -dr com.apple.quarantine` workaround. Apple Developer account enrolment tracked as a separate prerequisite for S11b. | S2 drops the signing deliverable; S11b inherits it as a gate |
| Q9 | S11 HITL split | **Split into S11a (flip default + pre-release Marketplace publish + dogfood) and S11b (stable publish + tree-sitter CORPUS_VERSION pin + v0.1.0 tag).** | S11 split; pre-release channel from source-plan Q8f activates in S11a |
| Q10 | Visitor-test parity audit (source A4.5) | **Interleave into S3–S8.** Each grammar slice audits its own kind's legacy visitor tests and back-fills corpus gaps in the same PR. S10 becomes a residual audit + perf benchmarks. | S3–S8 deliverables each gain a per-kind audit item; S10 renamed and slimmed |
| Q11 | Harness A regression scope per PR | **Global on every PR; skip future kinds only.** Already-implemented kinds must stay green everywhere; the `skipped` marker tracks implementation frontier. | S0 `AGENT.md` encodes rule; S3+ acceptance criteria already require full un-skipped set green |
| Q12 | Tree-sitter drift cadence | **Per-slice PR in `tree-sitter-craft`.** Each S3–S8 has a paired tree-sitter PR updating `grammar.js` for its kind; `craft`'s warning-only CI job stays meaningful. | S3–S8 gain a paired-PR deliverable; S13 reshapes from "corpus coupling" into "corpus-coupling infrastructure only" |
| Q13 | `craft check --lsp-json` placement | **S3.** First slice with real LSP output to mirror. Extends naturally through S4–S8. | S3 gains the `--lsp-json` flag on `craft check`; every grammar slice thereafter extends coverage |
| Q14 | S11a dogfood criteria | **Codified checklist** (3 distinct real `.craft` files, 30 min active edit each, every LSP handler exercised, intentional break/recovery, `$/logTrace` clean, mid-session restart). | S11a deliverables grow a checklist; "zero P0 issues" remains the bar |
| Q15 | Multi-file workspace design | **Multi-file aware from S3.** `internal/workspace/` indexes all `.craft` files in the root on `initialize` from day one, even though S3 only uses the per-file slice. Matches source-plan Q4a. | S3 workspace deliverable is multi-file; S5 reuses the piping for cross-file definition without retrofitting |
| Q16 | (Corrects Q7.) Preview/sidebar LSP-custom-command dependency | **Port to Go `craft lsp`:** implement `workspace/executeCommand` handlers for `EXTRACT_DOMAINS_FROM_CURRENT` / `EXTRACT_DOMAINS_FROM_WORKSPACE` in the Go server. First feasible at S5/S6 once sema has services + domains. Sidebar degrades gracefully ("waiting for parser v2") from S2 through S4. Q7's "preview commands call the HTTP server" was wrong — they call custom LSP commands. | S5/S6 gain the port; S2 documents the short coverage gap (three jest tests in `server/` deleted, restored in Go tests in S5/S6) |
| Q17 | Panic-recovery granularity | **Four-tier matrix** (lexer / parser / sema / handler) codified in `ARCHITECTURE.md`. Middleware + handler-level recovery lands in S2; parser + lexer recovery in S3; sema recovery in S4+. Each tier logs a structured `slog` event; `$/logTrace` surfaces them. | S0 gains the matrix deliverable; S2/S3/S4 each land their tier; S11a's dogfood checklist verifies no warnings |
| Q18 | `examples/*.craft` freeze policy | **Freeze originals; fork into `testdata/corpus/99_mixed/`.** Copy every `examples/*.craft` into the corpus in S1 with paired goldens (reviewed per Q2). Originals in `examples/` stay untouched through v0.1. Parser conforms to committed behaviour. | S1 deliverable grows by "copy examples + generate goldens"; S8's `examples/` acceptance is auto-satisfied via corpus harness |
| Q19 | Broken-corpus diagnostic wording strictness | **Structured match: assert `code` + `range` + `severity`, not `message`.** Every diagnostic gets a stable `code` (e.g. `craft/syntax/unexpected-token`). Goldens assert only the structured fields; message text stays free-form. Matches TypeScript / rust-analyzer convention. | S0 `ARCHITECTURE.md` gains the diagnostic-code policy; S9 ships a codebook table |
| Q20 | LSP `ServerCapabilities` declaration | **Incremental per-slice.** S2 declares only `textDocumentSync`; each subsequent slice adds exactly the capabilities its handler enables. Declaring absent capabilities mis-routes VSCode UI. | S2 adds the rule to `AGENT.md`; S3+ PRs update capabilities alongside handlers |
| Q21 | Sema compute model | **Eager full-workspace sema on every debounced change** + per-file parse cache with content-hash reuse. Matches source-plan Q4c's "10–100ms" expectation for realistic workspace sizes; Salsa-style incremental remains source-plan Anti-scope. | Performance gate on S5: ≤200ms keystroke-to-diagnostics round-trip on a 20-file workspace |
| Q22 | `internal/processor/` seam rewire | **Rewire to `CraftDoc` in S1.** Single disruption; validates the public contract against a real downstream consumer (visualizer) from day one. Accept increased S1 blast radius; fall back to deferring-to-S12 only if the rewire proves surprisingly big in practice. | S1 deliverable grows by "processor consumes `CraftDoc`"; S1 acceptance adds byte-identical `craft visualize` on 3 examples |
| Q23 | Sema validation-rule placement | **Rule-per-first-applicable-slice.** Each rule lands in the earliest slice where it becomes meaningful: duplicate-name at S3, cross-kind reuse at S4, unresolved-reference + invalid-target at S5, use-case-specific at S6, exposure-specific at S8. Every rule ships with a stable diagnostic code (Q19) + broken-corpus entry + codebook entry. | S3–S8 deliverables each grow by their applicable rules; S9's codebook consolidates entries seeded by prior slices |

---

## Why re-slice?

The source plan is organised as four **horizontal tracks**: Track A (parser), Track B (semantic layer), Track C (LSP protocol), Track D (VSCode client + release). Those tracks are load-bearing for *reasoning* about the migration but they are the wrong unit of work for an agent-driven execution: finishing Track A in isolation produces nothing a human can demo, and a bug in the Track D packaging pipeline is not discovered until weeks of parser work are already committed.

This doc re-expresses the same plan as **tracer bullets** — thin vertical slices that each cut through *every* relevant layer (lexer → parser → AST → CraftDoc → sema → workspace → LSP handler → VSCode client → packaged VSIX → CI). Two exceptions keep their horizontal shape because they *are* the first tracer bullet's contract and runtime respectively: Slice 1 bootstraps the canonical-output contract, and Slice 2 bootstraps the distribution pipeline. After those, every slice picks one grammar construct and drives it end-to-end.

Each slice is tagged **AFK** (agent-executable, merge without human interaction beyond PR review) or **HITL** (requires human judgment: golden audit, dogfooding sign-off, release cut). We preferred AFK wherever safe.

**Cross-reference map to the source plan:** every slice header cites the P/A/B/C/D task IDs it consumes, so the original plan remains the source of truth for decision rationale and the diff harness contract.

---

## The tracer-bullet graph

```
S0 (HITL) ─┐
           ├─▶ S1 (AFK) ─┬─▶ S3 (AFK) ─▶ S4 ─▶ S5 ─▶ S6 ─▶ S7 ─▶ S8 ─▶ S9 ─▶ S10 ─▶ S11a (HITL) ─▶ S11b (HITL) ─▶ S12 (HITL)
           │             │
S2 (AFK) ──┘             └─▶ S13 (AFK, parallel after S3)
```

- S0 + S1 + S2 are the three prerequisites. S1 (contract) and S2 (distribution) can run in parallel; S0 (guardrail docs) is a Tiago-authored gate.
- S3–S8 are the grammar-construct tracer bullets; they run **sequentially** because each thickens the same parser/sema/LSP layers and the later ones reference symbols introduced by earlier ones. Per Q10, each includes its own visitor-test parity audit for its kind.
- S9 (error recovery), S10 (perf + residual audit), S11a (flip default + pre-release), S11b (stable v0.1.0 release + macOS signing), S12 (delete ANTLR) close out the migration.
- S13 (tree-sitter corpus coupling) is genuinely parallel once S3 has committed its first corpus files.

---

## Slice index

| # | Title | Tag | Status | Source-plan ref | Demoable artefact |
|---|-------|-----|--------|-----------------|-------------------|
| S0 | Phase 0 guardrail docs | HITL | ✅ Drafted 2026-04-22 — awaiting Tiago review | P0.6, P0.7, P0.8 | `GRAMMAR.md`, `ARCHITECTURE.md`, `AGENT.md` committed |
| S1 | Canonical-contract tracer (`CraftDoc` full-grammar schema + ANTLR adapter + processor rewire + examples fork + 1 reviewed golden + Harness B) | AFK | ✅ Complete 2026-04-22 | P0.1, P0.2, P0.3 (seed), P0.4 (1 reviewed file), A0 (B half); expanded by Q1, Q18, Q22 | `craft check --parser=antlr` emits CraftDoc JSON; `craft visualize` byte-identical pre/post rewire; CI Harness B green |
| S2 | Distribution tracer (hello-world LSP + surgery of existing client + TextMate grammar + unsigned VSIX build) | AFK | ✅ Complete 2026-04-22 | C1 (scaffold only), D1 (scoped by Q7), D1b, D2, D3 (build half, publish stub); Q6 adds TextMate | Local VSIX install on Linux, activation shows "craft lsp connected", baseline colouring works, preview commands + sidebar views still functional against existing HTTP server |
| S3 | Actors end-to-end | AFK | ✅ Complete 2026-04-22 | A1 (partial), A2 (partial), A3a, B1 (seed), B2 (actor kind), C2, C3, C5, D-dogfood | `.craft` with only actors: diagnostics + outline + hover for actor names, both parsers agree |
| S4 | Domains end-to-end | AFK | ✅ Complete 2026-04-22 | A3b, B2 (domain kind), C4 (semantic tokens introduced here) | Actors + domains coloured differently, outline expands, hover shows kind |
| S5 | Services end-to-end + custom-command port | AFK | ✅ Complete 2026-04-22 | A3c, B3 (ResolutionMap seed), C6 (definition first use), Q16 custom-command port | Service blocks with `contexts:` jump to domain declarations via go-to-definition; sidebar services/domains views light up under `--parser=v2` |
| S6 | Use cases end-to-end | AFK | ✅ Complete | A3d, B3 (full), B4 (validation rules) | Hover/goto on any identifier inside a `when` clause lands on its declaration across files |
| S7 | Arch blocks end-to-end | AFK | ✅ Complete 2026-04-22 | A3e, C7 (folding) | `arch { presentation: … gateway: … }` folds in VSCode; outline shows segments |
| S8 | Exposures end-to-end | AFK | ✅ Complete 2026-04-23 | A3f, A3g | Cross-kind refs (`to:` actor, `contexts:` domain, `through:` service) resolve; harness A green on 06/99 |
| S9 | Error recovery end-to-end + diagnostic-codebook consolidation | AFK | ✅ Complete 2026-04-23 | A4, P0.5, B5 (lint merge); Q19 codebook consolidation | Harness C green on 13 broken-corpus files; syntax-error corpus added; lint rules ported to sema; last-good AST cache in workspace; DIAGNOSTICS.md consolidated |
| S10 | Perf + residual audit | AFK | ✅ Complete 2026-04-23 | A-bench, A4.5 (residual only per Q10) | Full corpus parses cold ~1.5ms / warm ~0.5ms (33× / 9.6× under budget); 3 bugs/gaps back-filled (keyword-as-context bug, service merging, blue_green+rolling deployment coverage); 1 known deferred gap (complex deployment rules canary(%)) |
| S11a | Flip default to v2 + pre-release publish | HITL | ⏳ Pending | A5a (flip half), C-trace, D4, partial D3 (pre-release channel) | `craft` ships with `--parser=v2` default via VSCode pre-release channel; dogfooding underway |
| S11b | Stable v0.1.0 release + macOS signing + tree-sitter pin | HITL | ⏳ Pending | A5a (release half), D3 (stable publish), D5 (CORPUS_VERSION pin), Phase 1 DoD, **macOS signing deferred from S2 (Q8)** | Signed VSIXs on Marketplace + OpenVSX stable channel; `v0.1.0` tags pushed; `tree-sitter-craft/CORPUS_VERSION` pinned |
| S12 | Delete ANTLR | HITL | ⏳ Pending | A5b | ANTLR, adapter, Harness B, `--parser` flag, `diff-parsers` subcommand all gone |
| S13 | Tree-sitter corpus-coupling infrastructure | AFK (parallel after S3) | ⏳ Pending | D5 (infra half), D6; grammar catch-up itself is in S3–S8 paired PRs per Q12 | `CORPUS_VERSION` + fetch script + bidirectional CI wired; warning-only on craft side |

---

## Slice details

Each slice is self-contained: the sections below can be pasted as-is into individual GitHub issues.

---

### S0 — Phase 0 guardrail docs *(HITL)*

**Source-plan ref:** P0.6 (`GRAMMAR.md`), P0.7 (`ARCHITECTURE.md`), P0.8 (`AGENT.md`).

**Why this is HITL:** these documents encode human judgement (layer boundaries, operating rules, EBNF against which every future slice is measured). An agent can *draft* them but Tiago owns acceptance.

**Why it's still a "slice" even though it touches no code:** without the layer rules in `ARCHITECTURE.md` and the red-green loop definition in `AGENT.md`, every subsequent slice has no objective merge gate. It is the prerequisite contract for AFK execution.

**Deliverables**
- `craft/docs/GRAMMAR.md` — EBNF draft keyed to existing ANTLR `.g4`; prose sections per block kind; edge cases noted from the tree-sitter grammar's experience. **Includes a dedicated "Contextual keywords" section** listing every token that is a keyword only in context (`asks`, `notifies`, `to`, `when`, `returns`, `listens`, and any others surfaced by `.g4` audit) — this is the checklist grammar-v2 will consume post-v0.1 (Q3).
- `craft/docs/ARCHITECTURE.md` — layer diagram, allowed/forbidden imports, Q1–Q20 decisions distilled in one place. **Includes:**
  - **Panic-recovery tier matrix (Q17):** lexer / parser / sema / handler — each tier's expected observable behaviour on panic (recover + log + which feature degrades).
  - **Diagnostic-code policy (Q19):** every emitted diagnostic carries a stable `code` (e.g. `craft/syntax/unexpected-token`). Goldens assert `code` + `range` + `severity`; `message` is free text.
- `craft/docs/AGENT.md` — per-track checklists, red-green loop, green-between-commits rule, never-do list. **Includes Harness A rule (Q11):** "Every PR must show Harness A green on every un-skipped corpus entry. The `skipped` marker is the implementation-frontier ledger; it may only move forward within the current slice's declared scope."

**Acceptance**
- Tiago reviews and approves each doc; changes land as a single PR per doc (three PRs total).
- `craft/CLAUDE.md` gains a "See `docs/AGENT.md` for operating rules" pointer *after* merge (do not touch CLAUDE.md inside S0 PRs — a later PR once AGENT.md exists).

**Dependencies:** none.

**Risks**
- AGENT.md scope creep into a full playbook — keep it to **rules the agent must follow**, not tutorials.

---

### S1 — Canonical-contract tracer *(AFK)*

**Source-plan ref:** P0.1, P0.2, P0.3 (just enough seed — one file per block kind), P0.4 (goldens for that seed only), A0 (Harness B half).

**Goal:** freeze the public `CraftDoc` contract once, for every block kind, before any v2 parser code is written. Prove the full stripe (contract → adapter → CLI → corpus → golden → Harness B → CI) end-to-end on a **single** reference corpus file. Subsequent slices grow the v2 parser; they do not grow the contract (Q1).

**Layers touched (proof of verticality)**
- Public API: `pkg/craft/craftdoc.go` — types covering **every block kind** (actor, domain, service, use_case, arch, exposure); JSON round-trip.
- Adapter: `internal/parser_antlr_adapter/` — ANTLR visitor → `CraftDoc` for the full shape (ANTLR already parses every kind, so the adapter can fill the schema from day one).
- CLI: `cmd/craft/` — `craft check <file> --parser=antlr` emits `CraftDoc` JSON to stdout.
- Test oracle: `testdata/corpus/01_actors/simple.craft` + paired `simple.craftjson` — one reference file that exercises the end-to-end pipe. Human-reviewed as part of this PR.
- Harness: `internal/parser_diff/antlr_test.go` (B) iterating corpus; starts with 1 file, grows per slice (Q2).
- CI: GitHub Actions step running `go test ./internal/parser_diff/...`.

**Deliverables**
- `CraftDoc` Go types for the **full grammar** — all block kinds modelled, marked `// Experimental: stabilizes at v0.1`. Round-trip test: `doc → json → doc`.
- ANTLR adapter emits that full shape for any input file; tested on the one reference file.
- `craft check` accepts `--parser={antlr,v2}` with `v2` producing a "not yet implemented" error.
- Harness B runs on every PR and hard-blocks on diff.
- `testdata/corpus/README.md` explains the golden format and the per-slice review cadence (Q2).
- **Examples-freeze fork (Q18):** copy every current `examples/*.craft` into `testdata/corpus/99_mixed/`. Generate paired `.craftjson` goldens for all of them via the ANTLR adapter. Only the one reference file (`01_actors/simple.craft`) is human-reviewed as part of S1; the `99_mixed/` goldens enter `skipped` state and are reviewed as their grammar constructs land per-slice. Originals in `examples/` stay frozen through v0.1 — docs, HTTP server, and user-facing examples don't churn.
- **Processor rewire (Q22):** convert `internal/processor/` and downstream `internal/visualizer/` consumers to accept `pkg/craft.CraftDoc` instead of the current `internal/parser/types.go:DSLModel`. The ANTLR adapter feeds `CraftDoc` all the way through. Single disruption, one PR, same S1.

**Acceptance**
- `craft check testdata/corpus/01_actors/simple.craft --parser=antlr` exits 0 with golden JSON on stdout.
- CI shows Harness B green with exactly one file iterated.
- Reverting the adapter to emit `null` makes CI red — the harness actually catches drift.
- **Visualizer regression check (Q22):** running `craft visualize` against ≥3 representative `examples/*.craft` files produces **byte-identical** output (PlantUML + SVG) before and after the processor rewire. Baselines captured pre-S1 and committed to `testdata/visualize-baseline/`. CI runs the visualize-diff on every S1-or-later PR.

**Dependencies:** S0 (for `ARCHITECTURE.md`'s layer rules that this slice must respect).

**Non-goals (explicitly deferred to later slices)**
- Full corpus seeding — Slice 3 onward adds one kind at a time.
- Hand-authored broken files + `.diagnostics.json` — Slice 9.
- v2 parser implementation — Slice 3 onward.

**Risks / watch-outs**
- `CraftDoc` shape bikeshed: resolve by picking the shape ANTLR's visitor already naturally produces; we optimize for "mechanical adapter, no translation" over "pretty API."
- JSON ordering: use `encoding/json` with sorted map keys to keep structural-equality trivial.
- Per Q1, the contract covers the full grammar from day one. If an unforeseen shape issue surfaces during S3–S8, treat it as a breaking change to an experimental API — PR title `CraftDoc: schema change`, regenerate all committed goldens via the adapter, re-review in the same PR.

---

### S2 — Distribution tracer *(AFK)*

**Source-plan ref:** C1 (scaffold only — no diagnostics), D1, D1b, D2, D3 (build half; publish to Marketplace/OpenVSX deferred — pre-release in S11a, stable in S11b).

**Goal:** prove `craft lsp --stdio` can hand-shake with a thin TS VSCode client and that GoReleaser can produce 6-target binaries packaged into per-platform VSIXs — *before* any LSP feature exists. Separates "did the distribution pipeline break?" from "did a parser change break?" for the rest of the migration.

**Layers touched**
- Binary: `cmd/craft/lsp.go` — `craft lsp --stdio` subcommand using `go.lsp.dev/protocol`; implements `initialize`, `initialized`, `shutdown`, `exit`; panic-recovery middleware (Q17 handler tier); slog stderr logger.
- LSP package: `internal/lsp/` — minimal server struct, no handlers beyond lifecycle. `ServerCapabilities` in `initialize` declares **only** `textDocumentSync` per Q20; later slices extend it alongside their handlers.
- `AGENT.md` rule (Q20): "When implementing a new LSP handler, update `ServerCapabilities` in the same PR. Never declare a capability the server doesn't actually serve."
- VSCode client surgery (Q7 + Q16): **keep** `client/src/extension.ts` (activation), `providers/` (services + domains sidebar views), `commands/` (preview commands), `services/` (DSL extraction), `webview/`, `ui/`, `utils/`, `types/`. **Delete** `TreeSitterHighlightProvider.ts` (Q6 — replaced by TextMate). **Delete** `craft-vscode-extension/server/` entire directory. Add an LSP spawn in `extension.ts` via `vscode-languageclient` (~50 LOC delta). **Correction to prior plan (Q16):** `dslExtractService.ts` sends `workspace/executeCommand` requests (`EXTRACT_DOMAINS_FROM_CURRENT`, `EXTRACT_DOMAINS_FROM_WORKSPACE`) that the old TS server implemented. From S2 through S4 these commands return an empty/"waiting for parser v2" response from the Go server; the sidebar views degrade gracefully. S5/S6 port the commands properly. Preview webview commands (`previewC4`, `previewDomain`) call the HTTP `craft server` directly (confirm by code-reading `commands/previewC4.ts` during the PR — correct if they do; wire them to HTTP if they were inadvertently routed through LSP).
- TextMate grammar (Q6): author `craft-vscode-extension/syntaxes/craft.tmLanguage.json`; wire it into `package.json` via the `contributes.grammars` stanza. Port the regexes from `tree-sitter-craft/queries/highlights.scm`. Keep `language-configuration.json`.
- Packaging: `.goreleaser.yml` building darwin-x64/arm64, linux-x64/arm64, win32-x64/arm64. **Unsigned** (Q8 defers signing to S11b). macOS users during S3–S10 dogfooding run `xattr -dr com.apple.quarantine /Applications/.../craft` once after install.
- CI: workflow that on every push builds 6 binaries, packages 6 per-platform VSIXs, uploads them as artifacts (publish to Marketplace/OpenVSX is stubbed — S11a flips pre-release, S11b flips stable).

**Deliverables**
- Working `craft lsp --stdio` that survives `initialize` → `shutdown` round-trip.
- TS client activation shows "Craft language server: connected" in the status bar; existing preview commands + sidebar views continue working against the HTTP server (Q7).
- `craft.tmLanguage.json` shipping in the VSIX; opening a `.craft` file colours keywords, strings, comments, identifiers (Q6).
- 6 unsigned VSIXs produced by CI and downloadable as artefacts. macOS install note in `docs/AGENT.md`: `xattr -dr com.apple.quarantine` one-liner (Q8).
- Docs: `docs/AGENT.md` gains a "how to run the LSP locally" section (if not already written in S0).

**Acceptance**
- Install the linux-x64 artefact VSIX in a clean VSCode profile; open any `.craft` file; extension activates without error; server log shows the lifecycle events. Baseline TextMate colouring visible (Q6). (macOS install works the same after `xattr -dr com.apple.quarantine` on the unzipped binary; not part of auto-acceptance per Q8.)
- `craft lsp --stdio` tested headlessly with a Go integration test using a stub client that sends `initialize` and expects proper capabilities response.
- `ServerCapabilities` in the `initialize` response declares **only** `textDocumentSync` (Q20); no handlers beyond lifecycle advertised.
- Cursor-friendly smoke test: same VSIX installs via OpenVSX packaging locally (actual publish still deferred).

**Dependencies:** S0 (operating rules). Independent of S1 (distribution doesn't touch parsing yet).

**Non-goals**
- Publishing to Marketplace or OpenVSX — S11a (pre-release), S11b (stable).
- Diagnostics, hover, any real handler — S3 onward.
- macOS code signing — deferred to S11b per Q8.
- Migrating preview commands off the HTTP server onto the LSP — that's Phase 1.5's job per Q7.

**Risks**
- `vscode-languageclient` version pinning: pick the latest stable; document in the release runbook.
- Per-platform VSIX size: monitor — target <20 MB per platform.
- TextMate regex port from tree-sitter highlights may miss edge cases; acceptable for S2 (tracked as a polish item for S11a, not a release blocker).
- Unsigned macOS binaries trigger Gatekeeper dialogs for every dogfood session through S10; the `xattr` workaround is one-shot per install but is annoying. If it becomes a blocker, we re-open Q8.

---

### S3 — Actors end-to-end *(AFK)*

**Source-plan ref:** A1 (partial — only ast nodes for actors), A2 (partial — lexer tokens used by actors), A3a, B1 (workspace skeleton), B2 (actor namespace only), C2 (publishDiagnostics), C3 (documentSymbol), C5 (hover). First "real" tracer bullet.

**Goal:** with only the `actor` keyword and `actors { ... }` block supported, a user can open a `.craft` file containing actors, see an outline, hover to see `actor: Name`, and — if an actor is duplicated — see a red squiggle. The same file round-trips through Harness A (v2 vs goldens) and Harness B (ANTLR vs goldens) identically.

**Layers touched**
- Lexer: `internal/lexer/` — identifiers, keywords (`actor`, `actors`), braces, newlines. Only what actors need; everything else is an error token for now.
- Parser: `internal/syntax/` — recursive-descent entry point, actor block parsing, produces `internal/ast/` nodes.
- AST: `internal/ast/` — `File`, `ActorDecl`. JSON round-trip test.
- Projection: AST → `CraftDoc` (extend the P0.1 schema only if actor shape requires it).
- Sema: `internal/sema/` — symbol collection in the actor namespace; `ResolutionMap` type introduced but not populated yet. **Validation rule (Q23):** `craft/sema/duplicate-name` (error) within the actor namespace. Stable code, broken-corpus entry under `testdata/broken/duplicate_actor.craft` with its `.diagnostics.json`, entry in `docs/DIAGNOSTICS.md` codebook.
- Workspace: `internal/workspace/` — **multi-file aware from day one (Q15)**. On `initialize`, scans workspace root for `.craft` files and builds a URI-keyed index; open/change/close/save lifecycle; per-file parse cache. S3 uses only the single-file slice but the multi-file index exists for S5 to pick up without refactoring.
- CLI mirror: `craft check --lsp-json <file>` (Q13) emits JSON containing the diagnostics + document symbols + hover-result-at-position that the LSP would produce. First shipped here; extended by every later grammar slice.
- LSP handlers (capability additions per Q20 land in the same PR: `documentSymbolProvider`, `hoverProvider`, diagnostic push):
  - `textDocument/publishDiagnostics` with 150ms debounce (from C2).
  - `textDocument/documentSymbol` (from C3).
  - `textDocument/hover` (from C5) — resolves cursor position to actor decl.
- Panic-recovery tiers (Q17): lexer + parser recovery wired to middleware. On parse panic, middleware recovers, logs via `slog`, returns the last-good AST from the workspace cache so documentSymbol/hover continue answering. Test: deliberately inject a panicking input; confirm other open files stay responsive.
- VSCode client: no new code — the generic `vscode-languageclient` surfaces the above.
- Tests:
  - Harness A: un-skip `testdata/corpus/01_actors/*.craft` (populate with ~8–12 files covering `actor Foo`, `actors { Foo Bar }`, nested edge cases).
  - Harness B: same files, same goldens — ANTLR adapter must still match.
  - LSP integration test: stub client opens an actor file, asserts documentSymbol and hover responses.

**Deliverables**
- Working v2 parser for actors only; everything else emits a recoverable "not yet implemented at line N" diagnostic so `--parser=v2` is usable on actor-only files.
- Populated `testdata/corpus/01_actors/` with paired `.craftjson` goldens (bootstrapped via adapter + reviewed as part of this PR — the S1 review process scales per-kind).
- LSP diagnostics/outline/hover working with both `--parser=antlr` and `--parser=v2`.
- **Visitor-test parity audit for actors (Q10):** scan `internal/parser/*actor*_test.go` for assertions not covered by the new `01_actors/` corpus entries; back-fill any gap as new corpus entries in this PR. Report the audit in the PR description.
- **Paired tree-sitter PR (Q12):** `tree-sitter-craft/` gets a PR covering actor grammar so `tree-sitter parse` succeeds on the new `01_actors/` corpus files. Landed alongside this slice; merge order: craft first, tree-sitter second.
- Dogfood note in the PR description: a screenshot of the outline sidebar on an actors-only file.

**Acceptance**
- Harness A green on `01_actors/*` with `--parser=v2`.
- Harness B still green everywhere (prevents accidental adapter regression).
- Manual dogfood: install the S2 VSIX updated with this build; open `examples/*actors*.craft` (or a newly committed example); outline appears, hover works, duplicate actor shows squiggle.

**Dependencies:** S1 (CraftDoc contract), S2 (LSP scaffold + VSIX pipeline), S0 (docs).

**Non-goals**
- Semantic tokens (colouring) — introduced in S4. Baseline colouring comes from the TextMate grammar authored in S2 (Q6), so S3 still looks alive in the editor.
- Go-to-definition — introduced in S5 where the first cross-reference exists.
- Error recovery for unclosed blocks — S9.

**Risks**
- Lexer tokens not defined here (e.g. `domain`) must still parse cleanly as identifiers-or-errors without panicking. Cover this with a "v2-on-mixed-file" test that expects a graceful partial-parse diagnostic.
- Position tracking bugs surface early — invest in a position-equality helper in Harness A from day one.

---

### S4 — Domains end-to-end *(AFK)*

**Source-plan ref:** A3b, B2 (domain namespace), C4 (`textDocument/semanticTokens` first use).

**Goal:** extend the S3 stripe to the `domain` / `domains { ... }` construct and introduce semantic tokens so actors and domains are visually distinguishable in the editor. This is where "it looks like a real editor" first kicks in.

**Layers touched (delta from S3)**
- Lexer: add `domain`, `domains` keywords.
- Parser: `DomainDecl` nodes and the `domains { ... }` block.
- AST: extend with `DomainDecl`.
- CraftDoc: add domain node shape.
- Sema: domain namespace — independent from actor namespace (source-plan Q4b). **Validation rules (Q23):** extend `craft/sema/duplicate-name` to the domain namespace; introduce `craft/sema/cross-kind-name-reuse` (warning) — fires when the same identifier is declared in two different kind-namespaces. Each rule gets a broken-corpus entry + codebook entry.
- LSP: implement `textDocument/semanticTokens` — distinct semantic types for actor vs domain. Declare `semanticTokensProvider` in `ServerCapabilities` in the same PR (Q20).
- Panic-recovery tier (Q17): sema-level recovery lands here (first slice where sema does real work). On sema panic, file falls back to "syntactic-only" features — outline still works from parser output; hover/definition return `null`.
- Corpus: populate `testdata/corpus/02_domains/` with goldens.

**Acceptance**
- Harness A green on 01 + 02. Harness B still green everywhere.
- Opening a file with both actors and domains: two visually distinct colours rendered by VSCode's semantic-tokens-aware theme.
- Cross-kind name reuse (`User` as actor and domain) — warning diagnostic surfaces (source-plan Q4b).
- **Visitor-test parity audit for domains (Q10):** `internal/parser/*domain*_test.go` reviewed; gaps back-filled as corpus entries in this PR.
- **Paired tree-sitter PR (Q12)** covering domain grammar.

**Dependencies:** S3.

---

### S5 — Services end-to-end *(AFK)*

**Source-plan ref:** A3c, B3 (ResolutionMap populated for the first time), C6 (`textDocument/definition`).

**Goal:** service blocks introduce the first cross-kind reference — `contexts:` points at domain declarations. This slice turns `ResolutionMap` from an empty struct into a working name-resolution pass and lights up go-to-definition.

**Layers touched (delta)**
- Lexer/parser: `service`, `services`, `contexts:`, `data-stores:`, `language:` keywords and their list values.
- AST: `ServiceDecl` with reference nodes (not yet resolved).
- Sema: resolution pass that walks service bodies, looks up each context name in the domain namespace, and populates `ResolutionMap`. **Validation rules (Q23):** `craft/sema/unresolved-reference` (error — `contexts:` names nothing) and `craft/sema/invalid-reference-target` (error — `contexts:` names something that isn't a domain, e.g. an actor). Each with broken-corpus entry + codebook entry. Compute model (Q21): eager full-workspace sema on debounced change; per-file parse cache keyed by content hash so unchanged files re-use their cached AST.
- LSP: `textDocument/definition` — cursor on a context reference jumps to the domain declaration (possibly in a different file — exercises multi-file workspace). Declare `definitionProvider` in `ServerCapabilities` (Q20).
- Custom commands port (Q16): implement `workspace/executeCommand` handlers for `EXTRACT_DOMAINS_FROM_CURRENT` and `EXTRACT_DOMAINS_FROM_WORKSPACE`. They walk `sema` to produce the same payload shape the old TS `server/` returned (shape defined in `craft-vscode-extension/shared/lib/types/domain-extraction.ts`). VSCode sidebar views in `client/src/providers/` light up for the first time under `--parser=v2`. Declare `executeCommandProvider` in `ServerCapabilities` naming both commands.
- Corpus: `testdata/corpus/03_services/` including a multi-file case (service in file A, domain in file B).

**Acceptance**
- Harness A green on 01 + 02 + 03.
- Integration test: stub client opens two files, issues `textDocument/definition` on a `contexts:` reference, receives a location in the *other* file.
- Dogfood: live go-to-definition in VSCode on a real example.
- **Performance gate (Q21):** a CI benchmark exercises a synthetic 20-file workspace (5 actor files, 5 domain files, 10 service files referencing them); keystroke-to-published-diagnostics round-trip must be ≤200ms on GitHub-hosted runners. Surfaced as PR comment; blocks merge if over budget.
- **Visitor-test parity audit for services (Q10):** `internal/parser/*service*_test.go` reviewed; gaps back-filled as corpus entries in this PR.
- **Paired tree-sitter PR (Q12)** covering service grammar.

**Dependencies:** S4. Multi-file workspace index (`internal/workspace/`) gets stress-tested here for the first time — if it was weak in S3, this slice will expose it.

**Risks**
- The `data-stores:` keyword collides with things in the grammar — check against `GRAMMAR.md` before lexing. Breaking change from v2.0 (`domains:` → `contexts:`, per CLAUDE.md) means older example files may fail; either update the examples or pin them to `--parser=antlr` until corpus entries are regenerated.

---

### S6 — Use cases end-to-end ✅ Complete

**Source-plan ref:** A3d, B3 (full ResolutionMap across all kinds so far), B4 (validation rules: duplicate names, invalid references), C4 extension (semantic tokens for use-case bodies).

**Goal:** the richest construct in the language. `use_case "..." { when ... }` contains actor references, domain references, service references, `asks`, `notifies`, event names, etc. Most of the editor's "feels alive" comes from this slice.

**Layers touched (delta)**
- Lexer/parser: `use_case`, `when`, `asks ... to`, `notifies`, quoted string literals, action verbs.
- AST: `UseCaseDecl`, `WhenClause`, `ActionStmt` (ask / notify / generic).
- Sema: **Validation rules (Q23):** `craft/sema/duplicate-use-case-name` (error); extend `craft/sema/unresolved-reference` to cover `when` / `asks` / `notifies` / action bodies (actors, domains, services referenced inside use-case clauses must resolve); cross-kind name collision already handled by S4's `craft/sema/cross-kind-name-reuse`. Each new rule gets a broken-corpus entry + codebook entry.
- LSP: hover now shows reference targets; go-to-definition works inside use-case bodies; semantic tokens colour actors/domains/services inside `when` clauses consistently with standalone declarations.
- Corpus: `testdata/corpus/04_use_cases/` — cover the full DSL example from CLAUDE.md plus edge cases.

**Acceptance**
- Harness A green on 01–04.
- Hover on `Authentication` inside `Authentication asks Database to ...` shows `domain: Authentication`.
- Validation diagnostics match hand-authored expectations.
- **Visitor-test parity audit for use cases (Q10):** `internal/parser/*use_case*_test.go` (and `*trigger*`, `*action*`) reviewed; gaps back-filled.
- **Paired tree-sitter PR (Q12)** covering `use_case`, `when`, `asks … to`, `notifies`, return actions.

**Dependencies:** S5.

**Risks**
- Quoted-string titles for use cases: lexer must not be confused by spaces/punctuation inside quotes. Add an explicit corpus entry: `use_case "User's \"primary\" registration" { ... }`.

---

### S7 — Arch blocks end-to-end *(AFK)*

**Source-plan ref:** A3e, C7 (`textDocument/foldingRange`).

**Goal:** `arch { presentation: ... gateway: ... }` blocks. First slice where folding ranges become genuinely useful (these blocks are often the largest in a file). Folding is introduced here rather than earlier because smaller block kinds didn't benefit from it enough to justify the LSP surface.

**Layers touched (delta)**
- Lexer/parser: `arch`, and the labelled-segment syntax (`presentation:`, `gateway:`, other labels per `GRAMMAR.md`).
- AST: `ArchDecl` with labelled-segment children.
- LSP: `textDocument/foldingRange` handler — return one fold per top-level block in the file, plus one per labelled segment inside `arch`.
- Corpus: `testdata/corpus/05_arch/`.

**Acceptance**
- Harness A green on 01–05.
- VSCode: folding triangles appear next to each `arch` segment; collapse/expand works.
- **Visitor-test parity audit for arch (Q10):** `internal/parser/*arch*_test.go` reviewed; gaps back-filled.
- **Paired tree-sitter PR (Q12)** covering `arch` blocks + labelled segments.

**Dependencies:** S6.

---

### S8 — Exposures end-to-end ✅ Complete 2026-04-23

**Source-plan ref:** A3f, A3g.

**Goal:** exposure blocks pull together every prior kind (`to:` actor, `contexts:` domain or service, `through:` service). First slice that exercises all three namespaces in a single reference cluster. Also runs the mixed-corpus tests that validate nothing regresses under heterogeneous inputs.

**Layers touched (delta)**
- Lexer/parser: `exposure`, `expose`, `to:`, `through:`, plus reuse of `contexts:` from S5.
- AST: `ExposureDecl`.
- Sema: cross-kind resolution finalised — any reference site can target any namespace declared by syntax. **Validation rules (Q23):** `craft/sema/invalid-exposure-target` (error — `to:` must name an actor; `through:` must name a service; `contexts:` must name a domain or service depending on exposure shape). Each rule variant gets a broken-corpus entry + codebook entry.
- Corpus: `testdata/corpus/06_exposures/` and `testdata/corpus/99_mixed/` (the cross-kind integration set).

**Acceptance**
- Harness A green on 01–06 + 99. The hand-written parser now covers the full grammar.
- Every `examples/*.craft` parses under `--parser=v2` matching `--parser=antlr` byte-for-byte in CraftDoc JSON.
- **Visitor-test parity audit for exposures + mixed (Q10):** remaining `internal/parser/*_test.go` files reviewed; final gaps back-filled. At the end of S8, every legacy visitor-test assertion has a corpus equivalent.
- **Paired tree-sitter PR (Q12)** covering `exposure` + any mixed-scenario refinements. At the end of S8, `tree-sitter-craft/grammar.js` covers the full grammar and the warning-only CI job in `craft` is green.

**Dependencies:** S7.

---

### S9 — Error recovery end-to-end *(AFK)*

**Source-plan ref:** A4 (island parsing), P0.5 (`testdata/broken/` hand-authored), B5 (merge existing `internal/linter/`).

**Goal:** until now the v2 parser has treated syntactic errors as "emit a diagnostic and stop." This slice adds island parsing so a broken block doesn't poison the rest of the file, and it teaches the LSP to publish error diagnostics with per-block granularity. Also folds the existing `internal/linter/` tests under `sema` (or adjacent) so nothing is lost from the legacy tree.

**Layers touched**
- Parser: top-level resync loop — on block-parse failure, consume tokens until next top-level keyword, re-enter the dispatch.
- Diagnostics: structured error messages matching `*.diagnostics.json` goldens.
- LSP: publishDiagnostics carries multiple independent errors per file with correct ranges.
- Workspace: last-good per-block AST cache so hover/definition/outline stay responsive on broken files.
- Corpus: `testdata/broken/` populated with hand-authored cases (Q12 / P0.5 rule: **not** bootstrapped from ANTLR).
- Harness: new third suite in `internal/parser_diff/` comparing v2 emitted diagnostics to `.diagnostics.json`. **Per Q19, the harness asserts on `code` + `range` + `severity` only** — `message` is rendered but not asserted, so prose polish never breaks CI.
- Diagnostic codebook consolidation: `docs/DIAGNOSTICS.md` (seeded incrementally by S3, S4, S5, S6, S8 per Q23) gets a final review pass here — S9 introduces syntax/recovery codes (`craft/syntax/unexpected-token`, `craft/syntax/missing-terminator`, `craft/syntax/unclosed-block`, …) alongside the sema codes landed earlier. Final codebook table: stable identifier, example message prose, severity, slice-of-origin.

**Acceptance**
- Hand-authored broken files produce the exact expected diagnostic set (set-equality on `(code, range, severity)` tuples per Q19) — no extra, no missing.
- Open a broken file in VSCode: exactly the failing block has a red squiggle, other blocks still render in the outline with hover working.
- `internal/linter/` tests green under their new home.

**Dependencies:** S8 (needs full grammar coverage so recovery doesn't mask "haven't implemented that yet").

**Risks**
- Diagnostic wording prose is not asserted (Q19), so regressions there aren't caught by CI. Mitigate with a codebook-review checklist in S11a's dogfood (Q14): messages read clearly during real editing sessions.

---

### S10 — Perf benchmarks + residual audit *(AFK)*

**Source-plan ref:** A-bench, A4.5 (residual only — the bulk of A4.5 was done per-slice during S3–S8 per Q10).

**Goal:** close the last two safety gates before we flip the default. A-bench proves the v2 parser is fast enough; the residual audit confirms S3–S8 left no legacy visitor-test assertion uncovered.

**Layers touched**
- Benchmarks: `internal/syntax/bench_test.go` — full-corpus cold <50 ms, warm <5 ms, run in CI with numbers surfaced in the PR log.
- Residual audit: one sweep across `internal/parser/*_test.go` to catch any file not covered by per-kind audits in S3–S8 (e.g. cross-kind or helper tests that didn't fit neatly into one slice). Expected to surface few or no gaps if Q10 was followed.

**Acceptance**
- CI benchmark job shows numbers under the stated budgets on GitHub-hosted runners.
- Residual audit report (in the PR description) confirms zero remaining gaps. If any are found, they are back-filled as corpus entries in the same PR — but this should be rare.

**Dependencies:** S9.

**Why this is still its own slice rather than folded into S9:** benchmarks are a dedicated performance PR (noisy if mixed with functional changes), and the residual audit wants a fresh pair of eyes over the whole test tree rather than the S9 author's context.

---

### S11a — Flip default to v2 + pre-release publish + dogfood *(HITL)*

**Source-plan ref:** A5a (flip half), C-trace (`$/setTrace`/`$/logTrace`), D4 (dogfood in Cursor). Per Q9, this is the "land and let it soak" half of the original A5a.

**Why HITL:** flipping the default parser and publishing (even to a pre-release channel) are user-visible irreversible-ish actions. Source plan explicitly gates A5a on Tiago's sign-off.

**Why split from S11b:** the pre-release channel is purpose-built for this soak (source-plan Q8f). Finding a v2 regression during pre-release dogfooding lets you hold S11b; finding one after a stable release is noticeably more expensive.

**Deliverables**
- Default-parser flip from `antlr` to `v2` (`--parser=antlr` remains as escape hatch).
- `$/setTrace` and `$/logTrace` implemented and tested — agent-debuggability requirement, deliberately landed close to release to avoid protocol churn earlier.
- Pre-release VSIXs published to Marketplace + OpenVSX using VSCode's `--pre-release` flag. Users on stable see nothing; users opting into pre-release get v2 default.
- CHANGELOG.md draft entry for `v0.1.0-rc1` (not yet the stable tag).
- Tag `v0.1.0-rc1` pushed in both `craft` and `craft-vscode-extension`.
- Tiago's one-working-day dogfooding completed on the pre-release VSIX; zero P0 issues open at the end.

**Dogfood checklist (Q14)**
The "one working day, zero P0 issues" gate is judged against this explicit list:
- Open at least 3 distinct real `.craft` files from `examples/` (e.g. `banking-system.craft`, `e-comerce.craft`, `user-management.craft`).
- Active editing for ≥30 min on each — exercises debounce, re-parse, semantic refresh on user input.
- Every LSP handler exercised at least once: diagnostics visible, outline panel expanded, hover opened on ≥5 distinct references, go-to-definition taken across files, folding used on arch + use-case blocks.
- Intentionally break each file (syntax error inside one block) — confirm island recovery keeps the other blocks responsive.
- Inspect `$/logTrace` output for a full session — no panic-recovery middleware warnings.
- Restart the server mid-session via the VSCode command — confirm clean recovery, no stuck state.

**Acceptance**
- Pre-release VSIX installs cleanly on clean Linux (the supported dogfood platform per Q8). macOS still requires `xattr` workaround.
- Every item on the dogfood checklist completed; findings logged in the PR.
- Gate to S11b: Tiago declares dogfood successful; pre-release has been live at least a Tiago-judged stretch with no regressions reported.

**Dependencies:** S10.

**HITL steps that cannot be AFK-merged**
- Marketplace/OpenVSX credential use.
- Sign-off on "v2 default is safe for pre-release channel."

---

### S11b — Stable v0.1.0 release + macOS signing + tree-sitter pin *(HITL)*

**Source-plan ref:** A5a (release half), D3 (stable publish path), D5 (tree-sitter `CORPUS_VERSION` pin), full Phase 1 Definition of Done. Inherits the macOS signing work deferred from S2 per Q8.

**Why HITL:** stable-channel publish is the most externally-visible action in the whole migration; everything it does is practically irreversible.

**Why now, not sooner:** waiting for S11a's pre-release soak to conclude is the whole point of splitting Q9. Also: signing setup (Q8) requires an Apple Developer account — that's a real external prerequisite Tiago tracks independently.

**Prerequisites (outside the PR)**
- Active Apple Developer Program membership.
- Developer ID Application certificate installed on the signing machine (or imported into GitHub Actions secrets).
- App-specific password for `notarytool`.

**Deliverables**
- GoReleaser update: darwin-x64 and darwin-arm64 binaries signed via `codesign` + notarised via `notarytool`. Signing secrets stored as GitHub Actions secrets.
- Stable VSIXs published to Marketplace + OpenVSX (no `--pre-release` flag).
- `tree-sitter-craft/CORPUS_VERSION` set to `v0.1.0`; upstream tree-sitter-craft CI hard-blocks on parse failures against the pinned corpus.
- CHANGELOG.md finalised for `v0.1.0`; release notes published.
- Tags `v0.1.0` pushed in `craft` and `craft-vscode-extension`.
- Fresh-install smoke test on clean macOS (signed path: no Gatekeeper dialog) + clean Linux.

**Acceptance:** every checkbox of the source plan's Phase 1 Definition of Done.

**Dependencies:** S11a completed; Apple Developer account active.

**HITL steps that cannot be AFK-merged**
- Apple signing credential use.
- Marketplace/OpenVSX stable-channel credential use.
- Final "this is 1.0-worthy" sign-off.

---

### S12 — Delete ANTLR *(HITL)*

**Source-plan ref:** A5b.

**Why HITL:** irreversible removal of a parser that's been shipping alongside v2 for a soak period. Gated on Tiago's explicit call per the source plan.

**Deliverables (every bullet from the source plan's A5b)**
- Remove `craft/antlr4/`, `craft/pkg/parser/`, `craft/tools/antlr-grammar/`.
- Remove `antlr4-go/antlr/v4` from `go.mod`.
- Remove `internal/parser/` (ANTLR visitor layer).
- Remove `internal/parser_antlr_adapter/`.
- Remove `internal/parser_diff/antlr_test.go` (Harness B).
- Remove `--parser` CLI flag and HTTP server `?parser=` query param.
- Remove `craft diff-parsers` subcommand.
- Remove `internal/parser/*_test.go` (safe because S10 back-filled them).

**Acceptance**
- `go.mod` no longer references ANTLR.
- `make fresh-setup` on a clean checkout no longer pulls `tiagocarcao/antlr4-craft` and build still succeeds.
- CLAUDE.md updated to remove ANTLR-specific sections.

**Dependencies:** S11b has shipped and soaked for a Tiago-judged duration.

---

### S13 — Tree-sitter corpus-coupling infrastructure *(AFK, parallel after S3)*

**Source-plan ref:** D5 (infrastructure half), D6.

**Scope change (Q12):** grammar catch-up itself happens per-slice in the S3–S8 paired PRs, not in S13. S13 now only ships the *infrastructure* that makes those paired PRs verifiable: the corpus-fetch pipeline, `CORPUS_VERSION` mechanism, and the bidirectional CI wiring. The actual tree-sitter grammar evolution is distributed across S3–S8.

**Why parallel:** once S3 has committed its first `01_actors/` corpus + paired tree-sitter PR, the infrastructure work does not block and is not blocked by later grammar slices.

**Deliverables**
- In `tree-sitter-craft/`:
  - `CORPUS_VERSION` file pinning the `craft` ref from which to pull corpus. Starts pointing at S3's merge commit; advances with each later slice.
  - `scripts/fetch-corpus.sh` pulling `.craft` files (not the `.craftjson` — tree-sitter has its own expectations, see source-plan Q7c).
  - `.github/workflows/compat.yml` — hard-blocks tree-sitter PRs if `tree-sitter parse` fails on any fetched corpus file (acceptance contract, not structural-equivalence).
- In `craft/`:
  - Warning-only CI job that runs `tree-sitter parse` using `tree-sitter-craft/main` against current corpus and posts a PR comment on failure (no block). Per Q12, this job being green is a per-slice signal — red warning comments during S3–S8 indicate the paired tree-sitter PR hasn't landed yet, which is expected but temporary.

**Acceptance**
- Intentional divergence test: temporarily commit a file that tree-sitter can't highlight; tree-sitter CI goes red; craft CI posts a warning comment.
- On S3 merge, the warning-only job is green (paired PR landed). Subsequent slices may briefly show the warning between craft-side merge and tree-sitter-side merge — acceptable per Q12's merge-order rule.

**Dependencies:** S3 (first corpus file + first paired tree-sitter PR).

---

## Re-slicing rationale (what changed vs the source plan)

Where the source plan's Track A/B/C/D stops one layer at a time, this doc collapses them into grammar-construct-shaped stripes. Concretely:

- **A3a–A3g became S3–S8.** Each source sub-track was already per-construct; we just pulled the matching slivers of A1/A2 (AST/lexer), B2–B4 (sema), and C2–C7 (LSP handlers) into the same PR so every merge is demoable end-to-end. Per Q10, each grammar slice also absorbs its own share of A4.5 (visitor-test parity audit) and a paired `tree-sitter-craft` PR (Q12).
- **A0 + P0.1–P0.4 became S1.** Canonical contract + one reviewed golden + Harness B is the minimum vertical proof that the contract works. Per Q1, S1 freezes the full `CraftDoc` shape up-front; per Q18, S1 forks every `examples/*.craft` into `testdata/corpus/99_mixed/`; per Q22, S1 also rewires `internal/processor/` + `internal/visualizer/` to consume `CraftDoc`. The rest of the goldens accrete slice-by-slice (Q2).
- **C1 + D1/D1b/D2/D3 became S2.** Hello-world LSP + surgical client edits (Q7: keep `providers/`, `commands/`, `services/`, `webview/`, `ui/`; delete `server/` and `TreeSitterHighlightProvider.ts`) + TextMate grammar (Q6) + unsigned per-platform VSIXs (Q8) is the minimum proof that distribution works. Publishing held back to S11a (pre-release) and S11b (stable) per Q9.
- **A4 + P0.5 + B5 became S9.** Error recovery, hand-authored broken corpus, and linter merge all land together — each alone is incomplete; together they're demoable as "broken file feels correct in editor." S9 also consolidates the `docs/DIAGNOSTICS.md` codebook that was seeded by every earlier sema-rule-landing slice per Q19 + Q23.
- **A4.5 + A-bench became S10 (residual + bench).** Per Q10, A4.5's bulk was interleaved into S3–S8, so S10 is now just the residual sweep + perf benchmarks.
- **A5a + C-trace + D4 became S11a; A5a (release half) + D3 + D5 + DoD + deferred signing became S11b.** Per Q9, pre-release publish (S11a) and stable publish (S11b) are deliberately two slices: the pre-release channel is the soak vehicle, and splitting lets you hold the stable release without rolling back a live default-parser flip. Q14 codifies S11a's dogfood checklist.
- **A5b stayed S12** — genuinely separate from S11b because it requires the post-stable soak period.
- **P0.6–P0.8 became S0.** Docs-only HITL slice; no code, but is a hard prerequisite. Per Q11, Q17, Q19, Q20 it now also codifies: Harness A regression rule, panic-recovery tier matrix, diagnostic-code policy, and the `ServerCapabilities`-per-handler rule.
- **D5/D6 became S13 (infrastructure only).** Per Q12, grammar catch-up is distributed across S3–S8 paired PRs; S13 is now solely the corpus-fetch pipeline + bidirectional CI wiring.

The Q1–Q15 decisions of the *source plan* are preserved verbatim. The Q1–Q23 decisions of the *grilling log above* refine how the work is sliced and sequenced, not what is being built. Where a grill decision contradicts a claim I originally made in this doc (Q5 superseded by Q6; Q7 corrected by Q16), both are retained in the log so the reasoning trail is visible.

---

## Open questions

1. **Should S1 include the `testdata/corpus/README.md` that documents the golden format, or defer to S0's doc pass?** — currently inlined in S1; move if S0 grows.
2. **Can S13 run *before* S3 merges (using just the seed file from S1)?** — probably yes, but deferring until S3 lands gives real corpus content to exercise; low-risk either way.
3. **Apple Developer enrolment lead time for S11b** — not in any slice's critical path until S11b; tracked as external prerequisite (Q8). Start enrolment no later than S9 to avoid stalling S11b.
