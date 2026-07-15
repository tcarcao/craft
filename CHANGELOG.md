# Changelog

## [2.11.0] — 2026-07-15

### Added
- **`UseCase` now carries `Line` and `SourceURI`** (`json:"line,omitempty"` / `"sourceUri,omitempty"`), parity with `Service`/`Actor`/`Action`. `Parse` stamps the filename you pass; `ParseFiles` stamps each use case's originating map key — so a consumer can attribute a use case to its file without re-scanning text.
- **Generic `tags {}` sub-block inside `use_case`** → `UseCase.Tags map[string]string`. An opaque, consumer-defined key/value slot (`tags { journey: re/renewal-flow  owner: "team billing" }`); craft neither validates keys nor interprets values. Values may be bare slugs or quoted strings. `tags` is a contextual keyword (still usable as an ordinary identifier). `Tags` is absent from JSON when no block is present.
- `syntax.EdgeKeywords()` accessor exposing the `context_map` edge verbs.

### Fixed / Hardened
- `internal/sema` edge-verb validation gained a defensive `default:` case (`craft/sema/unrecognised-edge-verb`) plus a build-time test keeping the parser's edge-keyword set and sema's verb maps in sync.
- A duplicate tag key or a second `tags {}` block emits `craft/sema/duplicate-tag` (warning; last value wins).

### Notes
- Additive/minor: existing keyed struct literals and JSON consumers are unaffected. Corpus goldens gained `use_case` `line` fields (regenerated).

## [2.10.1] — 2026-07-15

### Fixed
- **Module path now declares its major version, so `pkg/craft` is actually importable.** Go's semantic import versioning requires a v2+ module to end its path in `/v2`; without it, `go get github.com/tcarcao/craft/pkg/craft@v2.10.0` failed. The module is now `github.com/tcarcao/craft/v2`, so the public API is imported as:

  ```go
  import "github.com/tcarcao/craft/v2/pkg/craft"
  ```

  `go get github.com/tcarcao/craft/v2/pkg/craft@v2.10.1`. Only the import path changes — the API, types, JSON output, and CLI are identical to v2.10.0. (The v2.10.0 CLI binaries, Homebrew tap, and Docker image were unaffected; only the Go-library import was broken.)

## [2.10.0] — 2026-07-14

### Added
- **Importable parse API in `pkg/craft`.** External Go modules can now `import "github.com/tcarcao/craft/pkg/craft"` and parse Craft source in-process instead of shelling out to the CLI:
  - `craft.Parse(filename string, src []byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — single file; every diagnostic's `SourceURI` is normalized to the `filename` you pass (uniform with `ParseFiles`, no `file://` prefix).
  - `craft.ParseFiles(files map[string][]byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — multi-file merge plus cross-file resolution and lint. Files are processed in ascending filename order and each diagnostic batch is stable-sorted, so the merged model and the diagnostic slice are deterministic; each diagnostic's `SourceURI` is the map key you supplied.
  - Diagnostics are returned as data; the `error` slot is reserved for programmer errors. Neither function does file I/O.
- `LICENSE` file (MIT), matching the README's stated license.
- `pkg/craft` now declares a real stability contract (semver, stable as of 2.10.0). Requires Go 1.25+ to import.

### Changed
- `pkg/craft` is no longer marked "Experimental". The `CraftDoc`/`Diagnostic` type *definitions* moved to an internal leaf package (`internal/model`) and are re-exported from `pkg/craft` as type aliases — identity and JSON tags are unchanged, so no existing output or importer is affected.
- `cmd/craft`'s `check`, `inspect`, and `validate` are now thin wrappers over `pkg/craft.Parse`/`ParseFiles` rather than duplicating the parse+sema orchestration, guaranteeing the CLI and library cannot drift.
- `validate`'s workspace-level diagnostic lines now print the bare filename (consistent with per-file lines) instead of a `file://`-prefixed path.
- `check --lsp-json`'s actor symbol list now derives from the parsed document rather than the raw syntax tree, so a malformed name-only actor (no resolvable type) is excluded from symbols, consistent with the canonical model, instead of being emitted with an empty type. Its diagnostics also now carry a uniform `sourceUri` (the file path passed to `check`) rather than a mix of empty and `file://`-prefixed values.

### Notes
- No changes to the Craft language, grammar, or semantics. Diagnostic positions are byte-identical to v2.9.0.

## [2.9.0] — 2026-07-14

### Added
- **Craft DSL vNext grammar** — a backward-compatible evolution toward typed, code-anchored references (ticket 018):
  - **Flexible unquoted narrative** — step prose accepts special characters without quotes (`X asks Y for 1! & 2! and/maybe *`). A comment now begins only at a whitespace-preceded `//`, so `http://api`, `50/50`, and `and/maybe` stay in prose.
  - **Typed references** on `notifies`/`listens`/`asks` — event refs (`notifies vas.VasApplied`) and node slugs `[kind:][namespace/]name` (`bc:re/subscriptions`, `term:billing/dunning`, `service:subscriptions-api`). Slug segments may be words that lex as keywords (e.g. `term:user/account`).
  - **`context_map { }` block** with five typed edges: `realized_by`, `also_realizes` (bc → service), `same_as`, `contrasts`, `distinct_from` (term ↔ term).
  - **Service anchors** `opslevel:` and `repo:` (identity, not file paths).
  - **Local validation** — `craft validate` emits `craft/sema/malformed-slug`, `craft/sema/edge-endpoint-kind`, `craft/sema/duplicate-service-anchor` (errors) and `craft/lint/deprecated-string-ref`, `craft/sema/unresolved-ref-local` (warnings). Cross-validation against code facts remains the hub's job.
- Authoring docs updated: the `craft-dsl` skill and the VitePress guide now teach the vNext syntax.

### Changed
- Quoted event strings (`notifies "X"` / `listens "X"`) still parse but are **deprecated** — they emit `craft/lint/deprecated-string-ref`. Migrate to typed refs.

### Fixed
- Quoted string literals now round-trip byte-for-byte (a pre-existing corruption dropped the opening quote and duplicated the last content character; the lossless round-trip test had been silently scanning zero corpus files).
- Flexible-prose punctuation (e.g. `(1! & 2!)`) no longer gains spurious spaces in generated diagram/JSON output.
- Quoted service/domain declaration names now resolve correctly in validation and LSP (duplicate detection, diagnostics, rename).

---

## [2.8.2] — 2026-05-14

### Added
- Three new server endpoints exposing Mermaid source for the VS Code extension to render client-side:
  - `POST /preview/mermaid/domain` → Mermaid `flowchart LR` source
  - `POST /preview/mermaid/sequence` → Mermaid `sequenceDiagram` source
  - `POST /preview/mermaid/c4` → Mermaid `c4Diagram` source
- All three mirror the existing `/preview/{domain,c4}` JSON envelope. `PreviewResponse.Data` carries plain text (not base64) — the client renders the source itself.

---

## [2.8.1] — 2026-05-14

### Fixed
- Docker server image (`tiagocarcao/craft`) build for v2.8.0 failed because `build/package/Dockerfile` pinned `golang:1.23-alpine` for the builder stage, while v2.8.0's `go.mod` requires Go ≥ 1.25. Bumped the builder image to `golang:1.25-alpine`. No code changes from v2.8.0; CLI binaries and Homebrew tap for v2.8.0 were unaffected.

---

## [2.8.0] — 2026-05-14

### Added
- **Per-use-case filtering and splitting.** `craft generate` gains `--use-case <slug-or-name>[,...]` (filter detailed-domain + sequence to specific use cases) and `--split` (emit one file per use case). Use case names are matched by either the exact (quoted) name or a deterministic kebab-case slug; the same slug is used as the filename suffix in split mode.
- **Mermaid output format.** New `--format puml|mermaid|mermaid-md` flag selects the output format. `mermaid` writes `.mmd` files (raw Mermaid source); `mermaid-md` writes `.md` files with the source wrapped in a fenced ` ```mermaid ` block plus a `# title` heading — ready to commit alongside docs for GitHub-rendered inline preview. PlantUML (`puml`) remains the default and is byte-identical to previous releases.
- **Stdout output.** `--stdout` prints the generated diagram to stdout instead of writing to a file. Mutually exclusive with `--output`, `--split`, `--type all`, and multi-file invocations. Useful for `craft generate ... --stdout | pbcopy`.
- **`.md` no-clobber safety.** `--format mermaid-md` refuses to overwrite an existing `.md` file by default (those are often hand-written docs). `--force` opts in to overwriting. `.puml` and `.mmd` keep silent-overwrite behaviour.
- **Mermaid generator.** Three generators in `internal/visualizer/mermaid/`: `Sequence` (→ `sequenceDiagram`), `Domain` (→ `flowchart LR`, both detailed and architecture modes), `C4` (→ Mermaid's experimental `c4Diagram`).
- **`make test-integration` target.** New Makefile target runs build-tag-gated integration tests against real PlantUML and mermaid-cli renderers via testcontainers-go. Auto-detects podman on macOS and configures `DOCKER_HOST` / `TESTCONTAINERS_RYUK_DISABLED` accordingly.

### Fixed
- **PlantUML stdlib include drift.** Removed `!include <tupadr3/devicons2/rust>` from generated C4 PUML — that path does not exist in the PlantUML stdlib, breaking strict renderers (PlantUML 1.2025+). Rust now correctly resolves via `tupadr3/devicons/rust`. Language sprite includes are also now conditional: only languages actually declared by services in the model emit their include line, instead of all 11 supported sprites every time.
- **`skinparam handwritten` deprecation banner.** Domain and sequence diagrams emitted `skinparam handwritten false`, which PlantUML 1.2025+ flags with a visible yellow banner above every rendered diagram. Removed (the value was already the default).
- **Unicode em-dash mojibake.** Use case titles containing `—` rendered as Latin-1 mojibake in PlantUML output. Generators now declare `skinparam defaultFontName SansSerif` to select a system font with full Unicode coverage.
- **Bounded contexts rendered as actors.** When a use case trigger named a bounded context (`when VASFulfillment starts ...`), the detailed-domain generator emitted both a domain frame AND a stickman actor for the same name. Now checks against declared bounded contexts and treats the trigger as internal.
- **Architecture-mode self-loops.** Each domain in the architecture-mode domain diagram showed an unlabeled self-arrow. Self-loop edges are now filtered at emission.
- **CRON missing from C4 actors.** The C4 generator hard-coded a skip for any actor whose name began with `CRON`. Removed; CRON now appears like any other system actor.
- **C4 actor fan-out.** `Interacts directly` edges fanned out from every actor to every domain in services they were involved with, producing N×M clutter. Now scoped to the specific domain each actor actually triggers in its scenarios.
- **C4 event edges deduplicated by pair only.** Multiple events between the same domain pair (publisher → Event_Queue → listener) collapsed to a single edge. Dedup key now includes the event description, so all events render.
- **Mermaid Domain detailed had no trigger edges.** External triggers (`when CRON advances schedule`) now produce a labeled edge from the actor to the first action's context; listen-triggers route through the publishing domain.
- **Mermaid Domain and Sequence included unreferenced actors.** Both generators previously emitted every declared actor as a node/participant regardless of whether any included scenario used them. Now restricted to actors that trigger at least one scenario in the (possibly filtered) doc — especially noticeable in split-mode files.

### Changed
- **Go toolchain floor raised to 1.25.0.** A transitive dependency of `testcontainers-go` requires Go ≥ 1.25. Run `go install` / `make test` with Go 1.25 or newer.
- **`craft generate` documented behaviour.** The skill reference (`.claude/skills/craft-dsl/references/cli-reference.md`) and public CLI docs (`docs/page/tooling/cli.md`) now cover all new flags with examples.

### Notes
- Default invocation (`craft generate file.craft`) produces byte-identical PlantUML output to v2.7.0. All new behaviour is opt-in.
- Integration test coverage now exercises every diagram type against a real renderer (PlantUML server for `puml`, mermaid-cli for `mermaid`/`mermaid-md`) instead of only asserting structural string properties.

---

## [2.5.2] — 2026-04-27

### Fixed
- Numbers are now valid in action phrases. `asks X to check 3 transfer limits` previously produced a parse error on the numeric token; `collectPhrase` now collects `TokenNumber` alongside identifiers and strings.

---

## [2.4.0] — 2026-04-24

### Removed
- ANTLR parser fully removed — `--parser=antlr` flag no longer exists on `craft check`, `craft generate`, or `craft inspect`.
- `?parser=` query parameter removed from the HTTP server.
- `github.com/antlr4-go/antlr/v4` dependency removed.

### Changed
- `cmd/craft/validate` now uses the v2 semantic analysis layer (`internal/sema`) for all lint checks.
- `cmd/craft/inspect` output now uses `pkg/craft.CraftDoc` types.

---

## [0.1.0] — 2026-04-24

### Added
- Hand-written Go parser (`--parser=v2`) covering the full Craft grammar: actors, domains, services, use cases, arch blocks, and exposures.
- LSP server (`craft lsp`) with diagnostics, document symbols, hover, semantic tokens, go-to-definition, folding ranges, and `workspace/executeCommand` for domain/service extraction.
- Island parsing and error recovery — broken blocks do not cascade errors across the file.
- `$/setTrace` / `$/logTrace` trace-level support for LSP client debugging.
- `craft check --lsp-json` CLI flag for reproducing LSP responses without a running server.
- Acceptance corpus at `testdata/corpus/` with 80+ `.craft` files and paired `.craftjson` goldens.

### Changed
- CLI executable renamed from `craft-cli` to `craft`. The Homebrew name is also updated to `craft`.
- `craft check` and `craft generate` now default to `--parser=v2`. Use `--parser=antlr` as an escape hatch.
- HTTP server (`craft server`) defaults to the v2 parser; `?parser=antlr` query param available as escape hatch.

### Notes
- The ANTLR parser (`--parser=antlr`) remains available as an escape hatch and will be removed in `0.2.0`.
- macOS users: if Gatekeeper blocks the downloaded binary, run `xattr -dr com.apple.quarantine /path/to/craft` once. Code signing is planned for `0.2.0`.
