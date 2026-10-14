# Scenario Branches and Names — Design Spec

**Date:** 2026-10-14
**Owner:** Tiago Carção
**Status:** Approved in design review; awaiting written-spec review
**Release:** v2.21.0, a **breaking** change inside the `/v2` module path (decided; see §8)
**Drives:** re-arch-central journey assessment (`docs/journey-assessment/`), gaps G2 (conditional
steps / branches) and G3 (named scenarios). Retires the corpus convention "scenarios are comments".

## 1. Problem

A use case can only express alternatives by repeating a `when` block. Craft cannot name a scenario, and
words after `when X listens <event>` parse as a participant declaration, so authors write:

```craft
    // SCENARIO — nMF holds no cycle covering the requested timestamp.
    when CRON fires the daily snapshot schedule
        subscriptionplan stops retrying the moment nMF answers 404
```

The label is a comment, so diagrams, the model and the hub never see it. The re-arch-central corpus has
472 such comments, and 217 of 360 files repeat an identical trigger. Most of these "scenarios" are not
different ways in. They are different **outcomes of one step** (nMF answers OK / 404 / 5xx), written by
restarting the flow from its trigger. That also drops the steps that led to the branch point.

## 2. Goal and scope

1. **Branches inside a scenario**: `alt` (exactly one of several outcomes) and `opt` (steps that run
   only under a condition).
2. **Named scenarios**: an optional `scenario "<name>"` line before `when`.
3. **A structured model**: a scenario's body becomes a tree of steps (actions and blocks) in source order.
4. **Rendering**: native `alt`/`else`/`opt` boxes in sequence diagrams; branch labels in flow diagrams.
5. **Tooling parity**: formatter, LSP, tree-sitter highlighting, diagnostics.
6. **A lint** that flags indistinguishable scenarios, to guide migration.

Out of scope: `par` (parallel fan-out, G9) and `loop` (G18). They will reuse the same header-plus-offside
block shape later without redesign. The hub UI is also out of scope; only its loader changes (§7).

## 3. Syntax

```craft
use_case "Resolve a subscriber's billing cycle range" {
    scenario "daily snapshot"
    when CRON fires the daily snapshot schedule
        subscriptionplan asks nmf_monetization for the billing cycle range  [getUserBillingCycleRange]
        alt "nMF has a cycle for the timestamp"
            subscriptionplan returns the cycle start, end and last day
        else "nMF answers 404"
            subscriptionplan answers not-found without retrying
        else "nMF is unhealthy"
            subscriptionplan retries once after 100 ms
            opt "the retry also fails"
                subscriptionplan answers a server error
        subscriptionplan logs the run
}
```

### 3.1 Rules

1. **`scenario "<label>"`** is an optional line immediately before a `when`. It is a contextual keyword:
   recognised only at the start of a use-case-body line and only when followed by a STRING. Any token
   other than `when` on the next line is an error.
2. **Block headers.** `alt "<label>"` and `opt "<label>"` open a block. `else "<label>"` adds a branch to
   the nearest open `alt` whose header is at the same column. All three are contextual keywords,
   recognised only at the start of an action line and only when followed by a STRING. `alt`, `opt`,
   `else` and `scenario` therefore remain valid context, actor and verb names (an action always needs a
   verb after its subject, and a STRING can never be a verb). The label is required.
3. **Offside rule.** A header's body is every following line whose first token's column is greater than
   the header's column. The body ends at the first line whose first token is at or left of the header's
   column, or at `when`, `scenario` or `}`. An `else` must start at exactly its `alt`'s column. Blocks
   nest to any depth.
4. **Columns.** The parser compares `lexer.Token.Column` (runes, 1-based). A tab counts as advancing to
   the next multiple of 4 for this comparison only. The formatter rewrites tabs to spaces (§6.1).
5. **Indentation stays insignificant everywhere else.** Outside a block body, action lines keep today's
   rules: they run until `when`, `scenario` or `}`.

### 3.2 Diagnostics (errors, with recovery)

| Code | When | Recovery |
|---|---|---|
| `craft/syntax/empty-branch` | A header (`alt`, `opt`, `else`) has no body line. | Block kept with an empty branch. |
| `craft/syntax/inconsistent-indent` | A line is left of its block's first body line but right of the header. | The line joins the innermost block whose header is left of it. |
| `craft/syntax/orphan-else` | An `else` has no open `alt` at its column, or its block is an `opt`. | Parsed as the start of a new `alt` with that label. |
| `craft/syntax/missing-label` | `alt`, `opt` or `else` stands alone on its line (the next token is a NEWLINE). Any other token after it means the line is an ordinary action whose subject is named `alt`/`opt`/`else`. | Treated as a header with an empty label. |
| `craft/syntax/scenario-without-when` | A `scenario "…"` line is not followed by `when`. | Name dropped. |

An unterminated block at end of file needs no diagnostic of its own: the use case's missing `}` is
already reported (`craft/syntax/unclosed-block`).

### 3.3 Lint

`craft/lint/indistinguishable-scenarios` (warning): two scenarios in one use case whose trigger
descriptions are identical and where at least one has no `scenario` name. The message suggests `alt`
(different outcomes) or `scenario` names (different ways in).

## 4. Model (`internal/model`, aliased by `pkg/craft`)

```go
type Scenario struct {
    ID      string  `json:"id"`
    Name    string  `json:"name,omitempty"` // from `scenario "…"`
    Trigger Trigger `json:"trigger"`
    Steps   []Step  `json:"steps"`          // replaces Actions; source order
}

// Step is exactly one of Action or Block.
type Step struct {
    Action *Action `json:"action,omitempty"`
    Block  *Block  `json:"block,omitempty"`
}

type BlockKind string

const (
    BlockAlt BlockKind = "alt"
    BlockOpt BlockKind = "opt"
)

type Block struct {
    ID       string    `json:"id"`   // "block_<n>", same counter as action IDs
    Kind     BlockKind `json:"kind"`
    Line     int       `json:"line,omitempty"`
    Column   int       `json:"column,omitempty"`
    Branches []Branch  `json:"branches"` // an opt has exactly one
}

type Branch struct {
    Label string `json:"label"`
    Line  int    `json:"line,omitempty"`
    Steps []Step `json:"steps"`
}

// Actions returns every action of the scenario in source order (a depth-first
// walk of Steps). For consumers that need the flat list.
func (s Scenario) Actions() []Action
```

- **ID counter.** IDs keep one counter per use case across actions and blocks, in source order. A file
  with no blocks gets the same action IDs as today.
- **Kinds.** `BlockKind` constants live in `internal/lang` (ruling 19 of `diagram-pipeline-deepening.md`:
  the fixed vocabulary has one owner).
- **Mechanical migration.** Every in-repo reader of `Scenario.Actions` (sema, lint, interaction,
  visualizer, LSP, server) moves to `Scenario.Actions()` or walks `Steps` when it needs structure.

## 5. Semantics (`internal/interaction`)

- `Derive` walks `Steps` depth-first. `Interaction.Step` keeps its meaning: the n-th action in source order.
- `Interaction` gains `Branch []BranchRef`, the path from the outermost block inward, nil outside blocks:
  ```go
  type BranchRef struct {
      Block string         // Block.ID
      Kind  lang.BlockKind
      Label string
      Index int            // 0-based branch index within the block
  }
  ```
- **Call stack.** Every branch of an `alt` starts from the stack as it was at the `alt`. An `opt` body
  continues from the current stack. After any block closes, the stack is restored to its state at the
  block's start, because which branch ran is unknown. A return inside a branch answers correctly, and
  steps after the block are not distorted by one branch's unanswered calls.
- **Events.** Publish/listen pairing is unchanged: a `notifies` inside a branch pairs with every listener.
- **The trigger.** Its target is still the first action's subject (ruling 14). The first action is found in
  source order, so it may sit inside a block.

## 6. Rendering and tooling

### 6.1 Formatter (`craft fmt`)
- Canonical indentation: actions 4 spaces under `when`, a block body 4 more under its header, `else`
  aligned with its `alt`, `scenario` at the `when` column.
- A blank line before `scenario`, except as the first item of the use case.
- Tabs become spaces.
- The formatter refuses to format a file with any §3.2 diagnostic, so it never moves a line between blocks.
- Idempotent: `fmt(fmt(x)) == fmt(x)`, and block membership is preserved (round-trip test).

### 6.2 Sequence view
- `Message` gains the branch path. Writers open and close native blocks whenever the path changes between
  consecutive messages:
  - PlantUML: `alt <label>` / `else <label>` / `opt <label>` / `end`
  - Mermaid: `alt <label>` / `else <label>` / `opt <label>` / `end`
- Blocks nest inside the scenario frame (`group` / `rect`).
- **Frame titles.** A scenario frame is titled `<n>: <name>` when the scenario has a name, and
  `<n>: <trigger description>` otherwise. A single-scenario use case keeps no frame, as in v2.20.0.
- **Empty branches.** A branch with no drawable interaction (for example only an unresolved subject) is
  still drawn, with no messages, so the reader sees that the outcome exists.

### 6.3 Flow view
It has no boxes. An edge inside a block is prefixed with its innermost branch label:
`5. [nMF answers 404] answers not-found without retrying`.

### 6.4 Architecture and C4
Unchanged. Branches add no edges; edges are deduplicated anyway.

### 6.5 Lints
The context-map lints are unchanged; they read interactions, and every branch's interactions are real
dependencies.

### 6.6 LSP
- **Document symbols.** A scenario with a name shows its name. `alt` and `opt` branches are child symbols
  labelled with their branch label.
- **Folding ranges** for blocks.
- **Hover** on a header shows the block kind and branch count.
- **Completion** offers snippets for `alt "…"`, `else "…"`, `opt "…"` and `scenario "…"` inside a use-case body.
- **Capabilities.** Advertised only together with their handlers (`docs/AGENT.md` §3).

### 6.7 tree-sitter and the VSCode extension
- New rules in `tree-sitter-craft/grammar.js` for `scenario`, `alt`, `opt` and `else` header lines
  (keyword plus string), highlighted as keywords. tree-sitter does **not** model nesting; it is an
  editor-highlighting grammar, allowed to be more permissive (`GRAMMAR.md` §13).
- The parser-diff harness records block structure as a known divergence.
- Bump `CORPUS_VERSION` and regenerate the wasm/dylib.

## 7. Consumer: svc-re-go-arch-context
`internal/infrastructure/graphstore/craft_loader.go`:
- Iterates `sc.Actions()` instead of `sc.Actions`.
- Stores the scenario name, falling back to `scenarioLabel(sc.Trigger)`.
- Stores each step's innermost branch label (from a `Steps` walk) on the hub step.
- Ships in the same release window as craft v2.21.0. The hub UI change is a separate item.

## 8. Versioning
v2.21.0 with the module path still `github.com/tcarcao/craft/v2`. `Scenario.Actions` changes from a
field to a method, which is a source-breaking change for any `pkg/craft` consumer. This was accepted
knowingly because the only consumers are ours. The changelog opens with a **Breaking** section that
explains the change and the one-line migration (`sc.Actions` → `sc.Actions()`).

## 9. Testing (red-green-refactor, `docs/AGENT.md` §1)
- **Corpus** (`testdata/corpus/04_use_cases/`, with `.craftjson` goldens): `alt_else`, `opt`,
  `nested_blocks`, `named_scenarios`, `branch_tabs`, `branches_full` (the §3 example). Every existing
  golden is regenerated for `actions` → `steps`; the review shows only the shape change.
- **Broken** (`testdata/broken/`, hand-authored `.diagnostics.json`): one file per §3.2 code.
- **Interaction:** table tests for branch paths, the call stack per branch and after a block, the trigger
  target when the first action is inside a block, and step numbering.
- **Visualizer:** goldens for sequence (both backends) and flow on the new corpus models, plus a unit test
  for path-change block opening and closing, including nested blocks and a branch with no messages.
- **Formatter:** idempotence and block-membership round-trip on every new corpus file; refusal on each
  broken file.
- **Lint:** `indistinguishable-scenarios`, both positive and negative.
- **LSP:** document symbols and folding ranges for a nested example.
- **Dogfood:** `craft validate` on re-arch-central stays at 0 errors, and new warnings come only from the
  new lint. `subscriptions/subscriber-billing-cycle-resolution.craft` is rewritten with `alt` and rendered
  as the acceptance example.

## 10. Decisions
1. Branch kinds `alt` + `opt` now; `par` and `loop` later in the same shape. *(approved)*
2. Header with a quoted label plus the offside rule: no braces, no `end`. *(approved)*
3. An optional `scenario "<name>"` line before `when`, plus the `indistinguishable-scenarios` lint. *(approved)*
4. A tree model: `Scenario.Steps` with Step = Action | Block, and `Actions()` as a convenience. *(approved)*
5. v2.21.0, breaking within `/v2`. *(approved)*
6. tree-sitter highlights the headers but does not model nesting. *(approved in Section 3)*
