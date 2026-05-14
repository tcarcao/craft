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
