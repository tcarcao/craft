# Changelog

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
