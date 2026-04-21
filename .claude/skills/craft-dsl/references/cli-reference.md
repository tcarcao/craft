# craft-cli Reference

## Installation

```bash
brew install tcarcao/craft/craft-cli
```

This installs `craft-cli` via the Homebrew tap. Verify with `craft-cli --help`.

---

## Subcommands

### `validate` — parse and lint

```
craft-cli validate [files...] [flags]
```

Parses each file and runs all lint rules. Prints findings to stdout and exits with code 1 if any errors are found (warnings alone do not cause a non-zero exit).

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |
| `--strict` | false | Treat warnings as errors (exit 1 on any finding) |

**Examples:**
```bash
craft-cli validate docs/payments.craft
craft-cli validate docs/**/*.craft --strict
craft-cli validate docs/payments.craft --format json
```

---

### `generate` — produce PlantUML diagrams

```
craft-cli generate [files...] [flags]
```

Parses the file(s) and writes `.puml` diagram files. Output goes to the same directory as the input file unless `--output` is set.

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, or `all` |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | (same dir as input) | Directory to write output files |

**Examples:**
```bash
craft-cli generate docs/payments.craft
craft-cli generate docs/payments.craft --type c4 --output diagrams/
craft-cli generate docs/**/*.craft --type sequence --mode architecture
```

---

### `inspect` — dump the parsed model

```
craft-cli inspect [files...] [flags]
```

Parses the file(s) and emits the structured model (actors, domains, services, use cases). Useful for debugging or piping into other tools.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text` or `json` |

**Examples:**
```bash
craft-cli inspect docs/payments.craft
craft-cli inspect docs/payments.craft --format json | jq '.services'
```

---

## File arguments

All subcommands accept one or more file paths. Glob patterns using `**` are supported:

```bash
craft-cli validate docs/**/*.craft          # all .craft files under docs/
craft-cli validate payments.craft orders.craft  # multiple explicit files
```

---

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (no errors) |
| 1 | One or more errors found (or warnings with `--strict`) |
| 2 | CLI usage error |
