# craft-cli

Command-line tool for validating, inspecting, and generating diagrams from `.craft` files.

## Installation

```bash
brew install tcarcao/craft/craft-cli
```

Verify:

```bash
craft-cli --help
```

---

## Commands

### `validate`

Parse and lint one or more `.craft` files.

```bash
craft-cli validate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | false | Treat warnings as errors (exit 1 on any finding) |

```bash
craft-cli validate docs/payments.craft
craft-cli validate docs/**/*.craft --strict
craft-cli validate docs/payments.craft --format json
```

---

### `generate`

Produce PlantUML diagram files from `.craft` sources.

```bash
craft-cli generate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | same dir as input | Directory to write output files |

```bash
craft-cli generate docs/payments.craft
craft-cli generate docs/payments.craft --type c4 --output diagrams/
craft-cli generate docs/**/*.craft --type sequence --mode architecture
```

---

### `inspect`

Dump the parsed model as structured text or JSON.

```bash
craft-cli inspect [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

```bash
craft-cli inspect docs/payments.craft
craft-cli inspect docs/payments.craft --format json | jq '.services'
```

---

## File arguments

All commands accept one or more file paths. Glob patterns with `**` are supported:

```bash
craft-cli validate docs/**/*.craft
craft-cli generate payments.craft orders.craft
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | One or more errors (or warnings with `--strict`) |
| 2 | CLI usage error |
