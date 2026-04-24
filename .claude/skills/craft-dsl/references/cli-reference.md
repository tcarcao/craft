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

Parses each file and runs all lint rules. Prints findings to stdout and exits with code 1 if any errors are found (warnings alone do not cause a non-zero exit).

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | false | Treat warnings as errors (exit 1 on any finding) |

**Examples:**
```bash
craft validate docs/payments.craft
craft validate docs/**/*.craft --strict
craft validate docs/payments.craft --format json
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

**Examples:**
```bash
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type sequence --mode architecture
```

---

### `inspect` — dump the parsed model

```
craft inspect [files...] [flags]
```

Parses the file(s) and emits the structured model (actors, domains, services, use cases). Useful for debugging or piping into other tools.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

**Examples:**
```bash
craft inspect docs/payments.craft
craft inspect docs/payments.craft --format json | jq '.services'
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
