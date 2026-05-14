# Mermaid Output Mode — Design

**Status:** Approved (brainstorming complete 2026-05-14)
**Author:** Brainstormed with user, transcribed by Claude
**Companion plan:** to be created via `superpowers:writing-plans` after this spec is approved.

## Goal

Add Mermaid as an alternative output format to `craft generate`, supporting all three diagram types (c4, domain, sequence) and three output shapes (raw `.mmd` files, markdown-wrapped `.md` files, stdout). PlantUML remains the default — no breaking change for existing invocations.

## Why

Mermaid renders natively in GitHub markdown, VS Code preview, GitLab, Notion, and most modern markdown tooling. No PlantUML server, no .puml-to-image build step. The user wants to:

1. Commit diagrams alongside `.md` docs so GitHub renders them inline.
2. Paste diagrams directly into PR descriptions, issues, and chat messages.
3. Preview diagrams locally via VS Code's Mermaid extension or the Mermaid Live Editor.

PlantUML stays in the toolchain because of its richer C4 support (sprites, focus-mode), better Graphviz-driven layouts, and existing image-based artifact workflows.

## Architecture

### Package layout

New package `internal/visualizer/mermaid/` with three generators that mirror existing PlantUML structure:

```
internal/visualizer/
  mermaid/
    domain.go        # detailed + architecture mode → flowchart LR
    sequence.go      # → sequenceDiagram
    c4.go            # → C4Container (experimental Mermaid syntax)
    mermaid.go       # shared helpers, format options
    *_test.go        # unit tests with structural assertions
```

The Mermaid package consumes the same `*craft.CraftDoc` API the PlantUML generators consume. It reuses the existing helpers in `internal/visualizer`:

- `visualizer.Slugify` — split-mode filenames, use-case matching
- `visualizer.FilterUseCases` — `--use-case` filtering before generation

No duplication of model traversal. Where the PlantUML detailed-domain generator already walks scenarios and builds flow steps with numbering, the Mermaid generator does the equivalent walk emitting Mermaid edge syntax. The walks themselves stay in the existing per-type generators; only the *string emission* differs by target format.

### Type-to-Mermaid mapping

| Craft type | Mermaid syntax | Notes |
|---|---|---|
| `domain` (detailed) | `flowchart LR` | Domain nodes as `[X]` rectangles; queues as `(("X events"))` cylinder shape; edges labeled with `-- "N. phrase" -->`. Step numbering matches PlantUML side (resets per file in split mode). |
| `domain` (architecture) | `flowchart LR` + `subgraph` | One `subgraph "ServiceName"` per service, frames inside. Self-loops dropped (same rule as PlantUML side, already implemented in v2 work). |
| `sequence` | `sequenceDiagram` | 1:1 mapping. Use-case section headers become `Note over <all participants>: <name>` lines. |
| `c4` | `C4Container` (experimental) | `Person`, `System_Ext`, `System_Boundary`, `Container_Boundary`, `ContainerQueue`. Language sprites (`$sprite="go"` etc.) are dropped silently — Mermaid C4 doesn't support them. |

### C4 experimental fallback policy

Mermaid's `c4Diagram` syntax is officially experimental as of 2026. Some PlantUML C4 features have no equivalent (sprites is the main one). Policy:

- **Drop silently:** language sprites, layout hints (`LAYOUT_WITH_LEGEND`, `LAYOUT_LANDSCAPE`).
- **Translate:** all element types currently used by Craft's PlantUML C4 output map cleanly.
- **Error:** if a future Craft model feature can't be represented at all, return an error pointing the user back to `--format puml`. No silent garbage output.

## CLI surface

### Flags

One new flag and one new modifier flag:

```
--format puml|mermaid|mermaid-md   default: puml
--stdout                            write to stdout instead of files
--force                             allow overwriting existing .md files
```

### Format values

| Value | File extension | Content |
|---|---|---|
| `puml` (default) | `.puml` | Existing PlantUML output — byte-identical to today. |
| `mermaid` | `.mmd` | Raw Mermaid source. |
| `mermaid-md` | `.md` | Mermaid source wrapped in a fenced ` ```mermaid ` block with a `# <title>` heading above. |

### Orthogonality with existing flags

Every existing `craft generate` flag works identically across formats:

- `--type c4|domain|sequence|all` — chooses what to generate
- `--mode detailed|architecture` — domain mode (no effect on c4/sequence)
- `--use-case <slug-or-name>[,...]` — filter to a subset of use cases (domain + sequence only)
- `--split` — emit one file per use case (domain + sequence only)
- `--focus`, `--focus-context`, `--no-databases`, `--boundaries` — C4-only, same semantics

### `--stdout` semantics

Writes the generated source to stdout. No file is written. Enforces single-output:

- **Errors if:** `--split` is set, `--type all` is set, multiple `--type` values are set, OR multiple input files are passed.
- **Error message** says explicitly what conflicted and suggests `--type <single>` or dropping `--split`.
- Works with all three format values. `--format mermaid-md --stdout` writes a markdown-fenced block to stdout.
- `--stdout` and `--output <dir>` are mutually exclusive.

### `--force` semantics

Bypasses the no-clobber check for `.md` output (see File-collision safety below). No effect on `.puml` or `.mmd` files.

## File naming

| Invocation | Pattern | Example |
|---|---|---|
| Default | `<base>-<type>.<ext>` | `vas-c4.puml`, `vas-domain.mmd`, `vas-sequence.md` |
| `--split` | `<base>-<type>-<slug>.<ext>` | `vas-domain-time-based-vas-scheduled-finish.mmd` |

The `<base>` is `filepath.Base` of the input minus the `.craft` extension. The `<type>` is one of `c4`, `domain`, `sequence`. The `<ext>` is `puml`, `mmd`, or `md` per the format.

### File-collision safety for `--format mermaid-md`

`.md` files in a repo are often hand-written (READMEs, docs). Silently overwriting them is destructive. Policy:

- **Before writing any `.md` file:** check if it exists. If yes and `--force` is not set, error: `craft generate: refusing to overwrite existing file "<path>" (use --force to overwrite)`.
- **`--force`:** opt-in flag that bypasses the check.
- **`.puml` and `.mmd` files:** keep today's overwrite-by-default behaviour. These are generated artefacts.

## Markdown wrapper shape

For `--format mermaid-md`, each output file:

````markdown
# <title>

```mermaid
<generated Mermaid source>
```
````

Title line above the fence:

- **Default (non-split):** `<base> — <Diagram type>`, where `<base>` is the source filename without its `.craft` extension (e.g. `vas — C4`). Provides scanning context when several `.md` files sit in one docs directory.
- **Split mode:** the use case name verbatim (em-dashes preserved).
- The title heading is NOT part of the Mermaid source — it's markdown above the fence. The same source rendered through `.mmd` doesn't include it.

## Testing strategy

Three layers, mirroring how PlantUML output is verified today.

### Unit tests for generators

File: `internal/visualizer/mermaid/<type>_test.go`. Pattern: same as `diagram_quality_test.go`. Each test builds a small `*craft.CraftDoc` literal, calls the generator, asserts string-level structural properties.

Example properties to assert:

- Detailed-domain: output begins with `flowchart LR`, contains expected node tokens for each context, contains numbered edge labels.
- Sequence: output begins with `sequenceDiagram`, contains `participant <Name>` lines for every distinct domain.
- C4: output begins with `C4Container`, contains expected `Container_Boundary(...)` lines.

### Unit tests for CLI

File: append to `cmd/craft/generate_test.go`. Cover:

- Flag parsing for `--format puml|mermaid|mermaid-md`, default is `puml`.
- File extension changes correctly per format.
- Markdown wrapper contains fenced block with correct language tag.
- `--stdout` writes to the cobra command's stdout; rejects multi-output invocations.
- `--force` bypasses the no-clobber check.
- No-clobber default produces a clear error.

### Integration test (1 smoke test)

File: append to `internal/visualizer/render_integration_test.go` (build tag `integration`). Pattern: mirror the existing `plantUMLEndpoint(t)` helper.

- Helper `mermaidCLIEndpoint(t)` starts a `minlag/mermaid-cli` testcontainer.
- One test renders a canonical diagram (the same em-dash use case used in `TestVAS_SplitRenders`) by piping Mermaid source to `mmdc` and asserting non-empty output with no stderr errors.
- Skips cleanly if Docker/Podman unavailable.

Per-format CI cost stays low: unit tests run on every PR; integration runs via `make test-integration` which is already opt-in.

## Edge cases the design handles

| Scenario | Behaviour |
|---|---|
| `--type all --format mermaid` | Three `.mmd` files: c4, domain, sequence. |
| `--type all --format mermaid-md` | Three `.md` files. Each has its own title heading. |
| `--type all --split --format mermaid` | One `c4.mmd` (single, structural) + N×`domain-<slug>.mmd` + N×`sequence-<slug>.mmd`. C4 and architecture mode silently ignore `--split` (already established for PlantUML). |
| `--use-case foo --format mermaid-md` | Filtered model rendered as a single .md file. Same no-clobber check applies. |
| `--stdout --type all` | Errors with: `--stdout requires a single diagram (drop --type all or pick one type)`. |
| `--stdout --split` | Errors with: `--stdout is incompatible with --split (only one diagram per invocation)`. |
| `--stdout --output out/` | Errors: `--stdout and --output are mutually exclusive`. |
| Existing PlantUML invocation (no new flags) | Byte-identical output to today. |
| `--format mermaid-md` and target file exists | Errors with `refusing to overwrite "<path>" (use --force)`. |
| `--format mermaid-md --force` and target file exists | Overwrites without warning. |

## Out of scope (explicitly deferred)

These are noted to prevent scope creep, not to forbid future addition:

- Hand-written templating / per-project customisation of the Mermaid output style (themes, colours, layout hints).
- Replacing PlantUML as the default format.
- Round-trip validation (regenerating Mermaid from rendered output).
- Mermaid-specific output features that have no PlantUML equivalent (e.g., interactive click handlers).

## Migration / compatibility

- No breaking changes. Default `--format puml` produces byte-identical output to today.
- New flag values fail closed (Cobra rejects unknown values with a helpful error).
- New `--stdout` and `--force` flags are independent and additive.
- Help text for `craft generate` gains four lines (one per new flag, one for `--format`'s extended description).
