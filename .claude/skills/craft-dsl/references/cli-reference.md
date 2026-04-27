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

### `generate` — produce PlantUML diagrams

```
craft generate [files...] [flags]
```

Parses the file(s) and writes `.puml` diagram files. Output goes to the same directory as the input file unless `--output` is set.

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | (same dir as input) | Directory to write output files |
| `--boundaries` | `boundaries` | *(C4 only)* Boundaries rendering: `boundaries` or `transparent` |
| `--no-databases` | false | *(C4 only)* Hide data-store containers from the C4 diagram |
| `--focus` | (none) | *(C4 only)* Comma-separated service names to focus on; unfocused services render as external |
| `--focus-context` | (none) | *(C4 only)* Comma-separated bounded context names to focus on |

**Examples:**
```bash
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type sequence --mode architecture
craft generate docs/payments.craft --type c4 --boundaries transparent
craft generate docs/payments.craft --type c4 --no-databases
craft generate docs/payments.craft --type c4 --focus PaymentService,OrderService
craft generate docs/payments.craft --type c4 --focus PaymentService --focus-context Checkout,Payment
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
