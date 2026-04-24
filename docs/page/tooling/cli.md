# craft

Command-line tool for validating, inspecting, and generating diagrams from `.craft` files.

## Installation

```bash
brew install tcarcao/craft/craft
```

Verify:

```bash
craft --help
```

---

## Commands

### `validate`

Parse and lint one or more `.craft` files.

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
```

---

### `generate`

Produce PlantUML diagram files from `.craft` sources.

```bash
craft generate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | same dir as input | Directory to write output files |

```bash
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type sequence --mode architecture
```

---

### `inspect`

Dump the parsed model as structured text or JSON.

```bash
craft inspect [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

```bash
craft inspect docs/payments.craft
craft inspect docs/payments.craft --format json | jq '.services'
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
