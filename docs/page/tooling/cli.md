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

Every diagnostic code, with its severity and meaning, is listed in [`docs/DIAGNOSTICS.md`](https://github.com/tcarcao/craft/blob/main/docs/DIAGNOSTICS.md). Version 2.24 adds `craft/sema/store-ref-in-event-list` (error) and the warnings `craft/lint/duplicate-scenario-body`, `duplicate-event-in-list`, `duplicate-operation-in-list`, `one-of-single-event`, `stream-without-caller`, `egress-on-internal-step` and `unquoted-address`. `duplicate-scenario-body` reads the resolved model, so pass every file the use case depends on.

Version 2.25 adds the source-cite codes `craft/syntax/malformed-source` and `craft/lint/detached-source` (warnings, per file), the link codes `craft/sema/unresolved-use-case` (warning), `craft/sema/ambiguous-use-case` (error), `craft/lint/link-to-former-title`, `continues-not-last` and `self-link` (warnings), and the journey codes `craft/lint/journey-list` (warning) and `craft/lint/journey-unconnected` (hint). A link resolves over every file passed, so an `unresolved-use-case` on one file often goes away when you validate the whole folder.

---

### `generate`

Produce diagram files from `.craft` sources. Default output is PlantUML (`.puml`); `--format` switches to Mermaid (`.mmd`) or markdown-wrapped Mermaid (`.md` with a fenced ` ```mermaid ` block, ready to commit alongside docs).

**All inputs form one model.** A use case in one file can run through services and domains declared in another, exactly as `craft validate` sees them. One diagram per kind is written: a single input keeps its own name (`payments.craft` → `payments-c4.puml`); several inputs are named after the folder they share (`docs/**/*.craft` → `docs-c4.puml`).

**Errors stop generation.** Every diagnostic is printed to stderr as `file:line: severity: message`. If any is an error — syntax or semantic, such as a context declared in no domain — no diagram is written, because it would silently miss parts of the model. Pass `--allow-errors` to draw anyway. Warnings and hints never stop generation.

```bash
craft generate [files...] [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `all` | Diagram type: `c4`, `domain`, `sequence`, `surface`, or `all`. `surface` writes `<base>-surface.md`, a Markdown table of each `surface` block's operations and the use cases that call them, whatever `--format` says; `all` includes it only when the model declares a surface |
| `--mode` | `detailed` | Domain diagram detail: `detailed` or `architecture` |
| `-o, --output` | dir of the first input | Directory to write output files |
| `--format` | `puml` | Output format: `puml`, `mermaid`, or `mermaid-md` |
| `--use-case` | — | Comma-separated use case slugs or names to render *(detailed-domain and sequence only)* |
| `--split` | false | Emit one file per use case *(detailed-domain and sequence only)* |
| `--stdout` | false | Write to stdout instead of files (single diagram only; mutually exclusive with `--output`, `--split`, `--type all`) |
| `--force` | false | Allow overwriting existing `.md` files (`--format=mermaid-md`, or `--type surface`) |
| `--allow-errors` | false | Draw diagrams even when the model has error diagnostics |
| `--journey` | — | Comma-separated journey slugs, or `all`: one spine sequence per slug, `<base>-journey-<slug>.puml` (`.mmd`, or `.md` with `mermaid-md`). `all` draws every journey with 2+ members. Implies `--type sequence`; cannot be combined with `--use-case` or `--split`; `--stdout` takes one slug *(2.25)* |
| `--boundaries` | `boundaries` | *(C4 only)* Boundaries rendering: `boundaries` or `transparent` |
| `--no-databases` | false | *(C4 only)* Hide data-store containers from the diagram |
| `--focus` | — | *(C4 only)* Comma-separated service names to focus on; unfocused services render as external |
| `--focus-context` | — | *(C4 only)* Comma-separated bounded context names to focus on |

**Use case slugs.** `--use-case` accepts either the exact use case name (quote if it contains spaces) or its kebab-case slug. Example: `"One-time VAS — fulfillment"` → `one-time-vas-fulfillment`. The same slug is used as the filename suffix in `--split` mode.

**Journey spines (2.25).** A journey is every use case whose `journey` tag is the slug, across all inputs. `--journey <slug>` draws them as one sequence diagram titled `Journey: <slug>`, with a `== title ==` section per use case. A publisher drawn in an earlier section delivers its event directly (`publisher ->> listener`) instead of the inbound `(from …)` arrow. The sections follow the spine order, which `pkg/craft.BuildJourney` also returns:

- **Edges.** A → B when A publishes an event (or writes a store ref) that B's trigger listens to, or when A has a `continues "B"` link. `includes` adds no edge.
- **Namesake.** The member the journey is named after: a member whose title, slugified, is the slug; else one in a file named `<slug>.craft`; else one in a file named after one of its `formerly_journey` values.
- **Order.** A topological sort over the edges. Connected members come first; among the members that are ready, a successor of the member placed last wins (so a chain is not interleaved with another), then the namesake, then the earliest file and line. Members with no edge come last, the namesake first.
- **Cycles.** When no connected member is ready, the next is one that a placed member hands off to (the cycle is entered where the flow reaches it), else the namesake, else the earliest. `craft generate` prints `note: journey <slug> has a hand-off cycle; it is entered where the flow reaches it` to stderr.

An unknown slug is an error that lists the three closest slugs.

**Output formats.**

- `puml` — PlantUML source. Render with the PlantUML server, `plantuml.jar`, or a CI image like `plantuml/plantuml-server:jetty`.
- `mermaid` — raw Mermaid source. Renders in VS Code's Mermaid Preview, `mmdc`, and the [Mermaid Live Editor](https://mermaid.live/).
- `mermaid-md` — Mermaid wrapped in a fenced ` ```mermaid ` block with a `# title` heading. Commit alongside `.md` docs and GitHub renders the diagram inline. By default refuses to overwrite an existing `.md` (`--force` to opt in).

```bash
# PlantUML (default)
craft generate docs/payments.craft
craft generate docs/payments.craft --type c4 --output diagrams/
craft generate docs/**/*.craft --type domain --mode architecture   # one model → docs-domain.puml
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
craft generate docs/**/*.craft --type surface -o docs/   # one model → docs-surface.md

# Journey spines: one sequence per journey
craft generate docs/topology.craft docs/actors.craft 'docs/journeys/*/*.craft' --journey seller-promotes-an-advert
# → docs-journey-seller-promotes-an-advert.puml
craft generate 'docs/**/*.craft' --journey all --format mermaid -o diagrams/

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

Since 2.24 the model carries optional keys for the new constructs, each present only when used: `events` on a trigger (a `listens` list) and on an action (`notifies one of`), `operations` on an action (an operation list), `stream` and `answers` on a `returns stream`, and `inferred` on an `[inferred …]` hop. `event`, `ref` and `operation` keep their 2.23 values, so a file that uses none of the new constructs gives the same JSON as before.

Since 2.25: `sources` (the `//@` cites: `repo`, `path`, `start`, `end`, `text`, `line`) on use cases, triggers, actions, branches, notes and links, and `link` on a step (`id`, `kind`, `title`, `target`, `sources`). A step is exactly one of `action`, `block`, `note` or `link`.

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

Language features for cites, links and journeys (2.25):

| Feature | Where | What |
|---|---|---|
| Hover | a `//@` line | its parsed cites and the step it attaches to |
| Hover | the verb or opening keyword of a cited step | its sources after the usual hover |
| Hover | a link title | the target's file, first trigger and journey |
| Hover | a `journey` tag value | the members in spine order with their files, and "cycle" when there is one |
| Definition | a link title | the target's `use_case` line |
| Definition | a `journey` tag value | the other members, in spine order |
| References | a `use_case` title | the links to it |
| Completion | inside the quotes after `includes`/`continues` | every use-case title of the workspace; `"` is a trigger character |
| Code action | a `link-to-former-title` diagnostic | quick fix: write the current title |
| Code lens | a `journey` tag | "N use cases in this journey" |
| Document links | each cite of a `//@` line | `<dir>/<path>#L<start>`, where `<dir>` comes from `initializationOptions.sourceRoots` |
| Outline | a link | `includes: <title>` / `continues: <title>` under its scenario |

`sourceRoots` maps repos to local directories: `{"sourceRoots": {"re-x": "/src/re-x", "*": "/src"}}`. A repo with its own entry uses it; any other repo is `<"*">/<repo>`. A cite without a repo resolves through the attached step's service (`repo:`, else its name). Without `sourceRoots` the server does not advertise document links.

---

### Preview server

`cmd/server` (the Docker image) serves diagrams over HTTP for the VS Code extension. Every preview endpoint takes one `dsl` string; the journey endpoints take `files` instead, because a journey spans files:

| Endpoint | Body | Answer |
|---|---|---|
| `POST /journeys` | `{"files": {"<name>": "<source>", …}}` | `{"success": true, "slugs": [...], "diagnostics": [...]}`, slugs sorted |
| `POST /preview/journey` | `{"files": {…}, "slug": "<slug>", "format": "png"}` (`png`, `svg`, `pdf` or `puml`) | the PlantUML spine, base64 in `data` |
| `POST /preview/mermaid/journey` | `{"files": {…}, "slug": "<slug>"}` | the Mermaid spine source, plain text in `data` |

An empty or unknown slug answers 400.

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
