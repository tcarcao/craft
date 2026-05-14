# craft

Command-line tool for validating, inspecting, and generating diagrams from `.craft` files.

## Installation

```bash
brew install tcarcao/craft/craft
```

Verify:

```bash
craft --version
```

---

## Commands

### `validate`

Parse and lint one or more `.craft` files. When multiple files are passed, cross-file workspace analysis is also performed — resolving references across files and running workspace-level rules (e.g. undefined bounded contexts, duplicate service names).

```bash
craft validate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | false | Treat warnings as errors (exit 1 on any finding) |

```bash
craft validate docs/payments.craft
craft validate docs/**/*.craft --strict
craft validate docs/payments.craft --format json
craft validate docs/**/*.craft --format json   # cross-file workspace analysis
```

---

### `generate`

Produce diagram files from `.craft` sources. Default output is PlantUML (`.puml`); `--format` switches to Mermaid (`.mmd`) or markdown-wrapped Mermaid (`.md` with a fenced ` ```mermaid ` block, ready to commit alongside docs).

```bash
craft generate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | same dir as input | Directory to write output files |
| `--format` | `puml` | Output format: `puml`, `mermaid`, or `mermaid-md` |
| `--use-case` | — | Comma-separated use case slugs or names to render *(detailed-domain and sequence only)* |
| `--split` | false | Emit one file per use case *(detailed-domain and sequence only)* |
| `--stdout` | false | Write to stdout instead of files (single diagram only; mutually exclusive with `--output`, `--split`, `--type all`) |
| `--force` | false | Allow overwriting existing `.md` files when `--format=mermaid-md` |
| `--boundaries` | `boundaries` | *(C4 only)* Boundaries rendering: `boundaries` or `transparent` |
| `--no-databases` | false | *(C4 only)* Hide data-store containers from the diagram |
| `--focus` | — | *(C4 only)* Comma-separated service names to focus on; unfocused services render as external |
| `--focus-context` | — | *(C4 only)* Comma-separated bounded context names to focus on |

**Use case slugs.** `--use-case` accepts either the exact use case name (quote if it contains spaces) or its kebab-case slug. Example: `"One-time VAS — fulfillment"` → `one-time-vas-fulfillment`. The same slug is used as the filename suffix in `--split` mode.

**Output formats.**

- `puml` — PlantUML source. Render with the PlantUML server, `plantuml.jar`, or a CI image like `plantuml/plantuml-server:jetty`.
- `mermaid` — raw Mermaid source. Renders in VS Code's Mermaid Preview, `mmdc`, and the [Mermaid Live Editor](https://mermaid.live/).
- `mermaid-md` — Mermaid wrapped in a fenced ` ```mermaid ` block with a `# title` heading. Commit alongside `.md` docs and GitHub renders the diagram inline. By default refuses to overwrite an existing `.md` (`--force` to opt in).

```bash
# PlantUML (default)
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type sequence --mode architecture
craft generate docs/payments.craft --type c4 --boundaries transparent
craft generate docs/payments.craft --type c4 --no-databases
craft generate docs/payments.craft --type c4 --focus PaymentService,OrderService
craft generate docs/payments.craft --type c4 --focus PaymentService --focus-context Checkout,Payment

# Render only specific use cases
craft generate docs/payments.craft --type domain --use-case "Purchase Item,Refund"
craft generate docs/payments.craft --type sequence --use-case purchase-item

# Per-use-case files (one diagram per scenario)
craft generate docs/payments.craft --type domain --split -o diagrams/
# → diagrams/payments-domain-purchase-item.puml, payments-domain-refund.puml, …

# Mermaid for GitHub-rendered docs
craft generate docs/payments.craft --type all --format mermaid -o diagrams/
craft generate docs/payments.craft --type sequence --format mermaid-md -o docs/

# Pipe to clipboard for PR / chat
craft generate docs/payments.craft --type sequence --format mermaid --stdout | pbcopy
```

---

### `inspect`

Parse one or more files and emit a merged structured model (actors, domains, services, use cases across all inputs).

```bash
craft inspect [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

```bash
craft inspect docs/payments.craft
craft inspect docs/**/*.craft --format json | jq '.use_cases[].name'
craft inspect docs/payments.craft --format json | jq '.services'
```

---

### `check`

Parse a single `.craft` file and emit the full `CraftDoc` model as JSON. With `--lsp-json`, also includes diagnostics and document symbols — mirroring what the LSP server returns. Useful for debugging parser and LSP behaviour without running the language server.

```bash
craft check <file> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--lsp-json` | false | Emit diagnostics + symbols + CraftDoc as a combined JSON object |

```bash
craft check docs/payments.craft
craft check docs/payments.craft --lsp-json
craft check docs/payments.craft --lsp-json | jq '.diagnostics'
```

---

### `lsp`

Start the Craft Language Server Protocol server on stdin/stdout (JSON-RPC 2.0). Intended to be spawned by LSP clients such as the VS Code extension — not invoked manually.

```bash
craft lsp [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--log-file` | stderr | Redirect log output to a file |
| `--stdio` | false | Accepted for LSP client compatibility; stdio is always the transport |

```bash
craft lsp                                        # spawned by VS Code extension
craft lsp --log-file /tmp/craft-lsp-debug.log    # enable file logging for debugging
```

---

## File arguments

All commands accept one or more file paths. Glob patterns with `**` are supported:

```bash
craft validate docs/**/*.craft
craft generate payments.craft orders.craft
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | One or more errors (or warnings with `--strict`) |
| 2 | CLI usage error |
