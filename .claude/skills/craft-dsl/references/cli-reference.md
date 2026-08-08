# craft Reference

## Installation

```bash
brew install tcarcao/craft/craft
```

This installs `craft` via the Homebrew tap. Verify with `craft --help`.

---

## Subcommands

### `validate` — parse and lint

```
craft validate [files...] [flags]
```

Parses each file and runs all lint rules. When multiple files are passed, also performs **cross-file workspace analysis** — resolves references across files and runs workspace-level lint rules (e.g. undefined bounded contexts, duplicate service names). Prints findings to stderr and exits with code 1 if any errors are found (warnings alone do not cause a non-zero exit).

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | false | Treat warnings as errors (exit 1 on any finding) |

**Examples:**
```bash
craft validate docs/payments.craft
craft validate docs/**/*.craft --strict
craft validate docs/payments.craft --format json
craft validate docs/**/*.craft --format json   # cross-file workspace analysis
```

**vNext diagnostic codes.** `craft validate` checks *local well-formedness only* — node-slug shape, `context_map` endpoint/pattern shape, and deprecated syntax. Cross-repo/cross-context resolution against real code is a hub concern, not `craft`'s.

| Code | Severity | Meaning |
|------|----------|---------|
| `craft/sema/malformed-slug` | error | A `kind:`-prefixed node slug (`domain:`/`bc:`/`term:`/`service:`) doesn't match its kind's namespace shape (e.g. `domain:re/<name>`, `bc:<domain>/<name>`, `term:<bc>/<name>`, `service:<alias>` with no namespace), or uses an unrecognised `kind:` word |
| `craft/sema/edge-endpoint-not-bc` | error | A `context_map` endpoint resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/self-relationship` | error | A `context_map` statement's two endpoints resolve to the same bounded context (`X <pattern> X`) |
| `craft/sema/ambiguous-bc` | error | A bare `context_map` endpoint name is a bounded context in two or more domains — qualify it as `<domain>/<name>` |
| `craft/sema/unresolved-bc` | warning | A `context_map` endpoint doesn't resolve to any declared bounded context |
| `craft/lint/redundant-relationship` | warning | The same unordered bounded-context pair is declared with the same **symmetric** pattern (`partnership`/`shared_kernel`/`separate_ways`) more than once — directional duplicates in opposite order are not redundant |
| `craft/lint/separate-ways-violation` | warning | *(cross-validation)* A `separate_ways` pair nonetheless communicates — a dependency edge (from `asks`/`notifies`, either direction) is observed between the two bounded contexts |
| `craft/lint/relationship-direction-inverted` | warning | *(cross-validation)* A directional `context_map` pattern whose only observed dependency runs the wrong way (`LEFT → RIGHT`), with no correct-direction traffic present |
| `craft/lint/relationship-bidirectional` | hint | *(cross-validation)* A directional `context_map` pattern with observed dependency in both directions — consider `partnership` |
| `craft/lint/unclassified-communication` | hint | *(cross-validation)* A bounded-context pair that clearly communicates has no `context_map` edge at all, in any block |
| `craft/sema/glossary-endpoint-not-bc` | error | The BC part of a `glossary` term node resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/glossary-unresolved-bc` | warning | The BC part of a `glossary` term node doesn't resolve to any declared bounded context |
| `craft/sema/glossary-ambiguous-bc` | error | A bare BC name in a `glossary` term node is a bounded context in two or more domains — qualify it as `domain/bc/term` |
| `craft/sema/glossary-self-relation` | error | A `glossary` statement's two sides resolve to the same term node (same BC and same term) |
| `craft/lint/glossary-redundant` | warning | The same unordered `glossary` term-node pair is declared with the same verb more than once |
| `craft/lint/glossary-conflicting-relation` | warning | The same unordered `glossary` term-node pair is declared `same_as` and also `distinct_from` or `contrasts` |
| `craft/sema/duplicate-service-anchor` | error | A service block declares `catalog_ref:` or `repo:` more than once |
| `craft/lint/deprecated-string-ref` | warning | A `notifies`/`listens` uses the legacy quoted-string event form instead of a typed event ref — migrate to the dotted-id form (e.g. `order.OrderCreated`) |
| `craft/sema/unresolved-ref-local` | warning | A node-slug ref (`domain:`/`bc:`/`service:`) doesn't resolve to anything declared in the current file-set. This is a **local, best-effort** check only — an unresolved ref may still be valid once cross-repo/hub resolution is applied, so it's a warning, not an error |

---

### `generate` — produce diagrams

```
craft generate [files...] [flags]
```

Parses the file(s) and writes diagram files. Default output is PlantUML (`.puml`); `--format` switches to Mermaid (`.mmd`) or markdown-wrapped Mermaid (`.md` with a fenced ` ```mermaid ` block, ready to commit alongside docs). Output goes to the same directory as the input file unless `--output` is set.

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | (same dir as input) | Directory to write output files |
| `--format` | `puml` | Output format: `puml`, `mermaid`, or `mermaid-md` |
| `--use-case` | (none) | Comma-separated use case slugs or names to render *(detailed-domain and sequence only)* |
| `--split` | false | Emit one file per use case *(detailed-domain and sequence only)* |
| `--stdout` | false | Write to stdout instead of files (single diagram only; mutually exclusive with `--output`, `--split`, `--type all`) |
| `--force` | false | Allow overwriting existing `.md` files when `--format=mermaid-md` |
| `--boundaries` | `boundaries` | *(C4 only)* Boundaries rendering: `boundaries` or `transparent` |
| `--no-databases` | false | *(C4 only)* Hide data-store containers from the C4 diagram |
| `--focus` | (none) | *(C4 only)* Comma-separated service names to focus on; unfocused services render as external |
| `--focus-context` | (none) | *(C4 only)* Comma-separated bounded context names to focus on |

**Use case slug rules.** When passing `--use-case`, values can be either the exact use case name (quote if it contains spaces) or its slug — kebab-case, lowercase, non-alphanumeric collapsed to `-`. Example: `"One-time VAS — fulfillment through provider apply"` → `one-time-vas-fulfillment-through-provider-apply`. Slug is also the filename suffix used in `--split` mode.

**Format details.**
- `puml` — PlantUML source. Render with PlantUML server, `plantuml.jar`, or a CI image like `plantuml/plantuml-server:jetty`.
- `mermaid` — raw Mermaid source. Renders in VS Code's Mermaid Preview, `mmdc`, and the Mermaid Live Editor.
- `mermaid-md` — Mermaid wrapped in a fenced ` ```mermaid ` block with a `# title` heading above. Commit alongside docs and GitHub renders it inline. By default refuses to overwrite an existing `.md` (use `--force` to opt in).

**Examples:**
```bash
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type sequence --mode architecture
craft generate docs/payments.craft --type c4 --boundaries transparent
craft generate docs/payments.craft --type c4 --no-databases
craft generate docs/payments.craft --type c4 --focus PaymentService,OrderService
craft generate docs/payments.craft --type c4 --focus PaymentService --focus-context Checkout,Payment

# Render only specific use cases (slug or quoted exact name):
craft generate docs/payments.craft --type domain --use-case "Purchase Item,Refund"
craft generate docs/payments.craft --type sequence --use-case purchase-item

# Per-use-case files (one diagram per scenario):
craft generate docs/payments.craft --type domain --split -o diagrams/
# → diagrams/payments-domain-purchase-item.puml, payments-domain-refund.puml, …

# Mermaid output for GitHub-rendered docs:
craft generate docs/payments.craft --type all --format mermaid -o diagrams/
craft generate docs/payments.craft --type sequence --format mermaid-md -o docs/

# Pipe to clipboard for PR / chat:
craft generate docs/payments.craft --type sequence --format mermaid --stdout | pbcopy
```

---

### `inspect` — dump the parsed model

```
craft inspect [files...] [flags]
```

Parses one or more files and emits a **merged** structured model (actors, domains, services, use cases across all input files). Useful for debugging or piping into other tools.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

**JSON output shape:**
```json
{
  "files": ["docs/payments.craft"],
  "actors": [...],
  "domains": [...],
  "services": [...],
  "use_cases": [
    {
      "name": "Purchase Item",
      "events_published": ["Payment Processed"],
      "events_consumed": ["Item Added to Cart"]
    }
  ]
}
```

**Examples:**
```bash
craft inspect docs/payments.craft
craft inspect docs/**/*.craft --format json | jq '.use_cases[].events_published'
craft inspect docs/payments.craft --format json | jq '.services'
```

---

### `fmt` — format .craft files to canonical form

```
craft fmt <files...> [flags]
```

Formats in place. Changes whitespace and nothing else: the formatter walks the syntax tree's token stream and writes every non-whitespace token back verbatim, exactly once, in document order, so it cannot drop or reorder content. Applies 2-space indent, one statement per line, a blank line between top-level declarations and before each `when` after the first, and column-aligns trailing operation annotations across each contiguous run.

Comments stay where the author put them, including a trailing comment at the end of an action line. Interior runs of spaces inside a statement collapse to one; a line break the author wrote inside a wrapped value is preserved. `arch` blocks are left byte-for-byte verbatim, since their component chains use free-form indentation the formatter does not own. Formatting is idempotent for every input.

If a file has a parse error too severe to re-render from, `craft fmt` leaves it untouched and reports it as skipped rather than silently rewriting it or reporting it as clean.

| Flag | Default | Description |
|------|---------|-------------|
| `--check` | false | Write nothing; name each unformatted file and exit non-zero |

**Examples:**
```bash
craft fmt docs/payments.craft
craft fmt "docs/**/*.craft"          # quote the glob so the shell does not expand it
craft fmt --check "docs/**/*.craft"  # CI gate: exits 1 if any file is unformatted
```

Arguments are paths or globs (`**` supported). Directories are not walked: `craft fmt docs/` is an error, use `craft fmt "docs/**/*.craft"`.

---

### `check` — parse and emit CraftDoc JSON

```
craft check <file> [flags]
```

Parses a single `.craft` file and emits the `CraftDoc` model as JSON. With `--lsp-json`, also includes diagnostics and document symbols — mirroring what the LSP server returns. Intended for debugging the parser and LSP behaviour without running the full language server.

| Flag | Default | Description |
|------|---------|-------------|
| `--lsp-json` | false | Emit diagnostics + symbols + CraftDoc as a combined JSON object |

**Examples:**
```bash
craft check docs/payments.craft
craft check docs/payments.craft --lsp-json
craft check docs/payments.craft --lsp-json | jq '.diagnostics'
```

---

### `lsp` — start the language server

```
craft lsp [flags]
```

Starts the Craft Language Server Protocol server on stdin/stdout (JSON-RPC 2.0). This subcommand is intended to be spawned by LSP clients such as the VS Code extension — not invoked manually. Logs go to stderr by default.

| Flag | Default | Description |
|------|---------|-------------|
| `--log-file` | (stderr) | Redirect log output to a file |
| `--stdio` | false | Accepted for LSP client compatibility; stdio is always the transport |

**Examples:**
```bash
craft lsp                                        # spawned by VS Code extension
craft lsp --log-file /tmp/craft-lsp-debug.log    # enable file logging for debugging
```

---

## File arguments

All subcommands accept one or more file paths. Glob patterns using `**` are supported:

```bash
craft validate docs/**/*.craft          # all .craft files under docs/
craft validate payments.craft orders.craft  # multiple explicit files
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (no errors) |
| 1 | One or more errors found (or warnings with `--strict`) |
| 2 | CLI usage error |
