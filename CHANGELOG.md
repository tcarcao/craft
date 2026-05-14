# Changelog

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
