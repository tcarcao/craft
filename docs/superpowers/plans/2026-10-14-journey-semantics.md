# Journey Semantics (v2.23.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task header says which model it needs: **cheap** (mechanical: the code is given, copy it and make the tests pass) or **strong** (judgment: the code is sketched and the tests pin the behaviour, but the implementer must make design calls).

**Goal:** Add six journey constructs to Craft: concurrent and repeated blocks (`par`/`and`, `loop`), `note`/`finding` statements, the callee-side trigger `when <ctx> receives …`, store hand-offs (`store.<name>.<change>`) with a polled-listens schedule, and top-level `surface` blocks with `craft generate --type surface`. Add five new lints, carry everything through formatter, LSP, tree-sitter, the VSCode extension and the arch-context hub, and release v2.23.0.

**Architecture:** Every new word is a contextual keyword in the hand-written parser. `par`/`loop` reuse the alt/opt `Block`/`Branch` machinery and its offside rule; a `par` without `and` is split into one branch per step at projection (`Block.Split`). Notes are a third step kind (`Step.Note`). `receives` is a new trigger kind that the interaction layer turns into a trigger from a separate `Unobserved` endpoint (found/lost messages). Store refs are recognised in projection (`Action.Store`/`Trigger.Store`) and carried into interactions as `Via`; views draw a database participant. Surfaces are a new top-level declaration with their own model, sema checks, coverage view and Markdown writer.

**Tech Stack:** Go 1.25+ (`github.com/tcarcao/craft/v2`), tree-sitter (`~/projects/github/tcarcao/tree-sitter-craft`), TypeScript (`~/projects/github/tcarcao/craft-vscode-extension`), the hub (`~/Downloads/domain-hub/svc-re-go-arch-context`, hexagonal Go).

**Spec:** `docs/superpowers/specs/2026-10-14-journey-semantics-design.md`. Read it before any task. Section numbers below (§4.5 …) refer to it.

**Repo rules** (`docs/AGENT.md`, `CLAUDE.md`):
- Red → green → refactor; `go vet ./... && go test ./...` green at every commit.
- Goldens are regenerated only with `-update`, and the diff is reviewed. Existing `.craftjson` and diagram goldens must not change unless a step says so.
- Never touch `examples/*.craft`.
- Never stage `tools/` or `docs/superpowers/specs/2026-07-15-usecase-identity-tags-and-edge-verb-hardening.md` (both untracked on this branch). Stage by directory (`git add internal pkg cmd testdata docs/…`) or exact path, never `git add -A` at the repo root.
- Commit trailer: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

**Working branch:** `feat/journey-semantics` (checked out; commit 65a4594 holds the spec).

**Commands used everywhere**
- Full suite: `go vet ./... && go test ./...`
- Corpus goldens: `go test ./internal/corpustest/ -run TestHarnessA -update`
- Diagnostics goldens: `go test ./internal/corpustest/ -run TestHarnessC` (hand-authored `.diagnostics.json`; never `-update` them). Harness C runs parse + `AnalyzeFile` + `MergeWorkspaceSymbols` + `AnalyzeWorkspace`, **not** `LintWorkspace`, so lint codes are tested with unit tests in `internal/sema/lint_journey_test.go`.
- Diagram goldens: `go test ./internal/visualizer/ -run TestGolden -update`
- Mermaid/PlantUML render check (needs Docker or Podman; pulls the pinned `minlag/mermaid-cli:12.0.0` and `plantuml/plantuml-server:jetty` images): `make test-integration RUN='Mermaid|Render'`. The make target sets the testcontainers environment podman needs on macOS (Ryuk off, `DOCKER_HOST` at the podman machine socket); with Docker, `go test -tags integration -count=1 ./internal/visualizer/ -run 'Mermaid|Render'` works as well.

---

## Corrections to the spec found while planning

The plan follows the spec except where the code or sibling repos say otherwise:

1. **tree-sitter-craft is at 0.2.0 on npm**, not 0.1.10, and the v2.21 grammar commits (`64ecc9a`, `e66b2ea`) are not released yet. The release is `./release.sh minor` → **0.3.0**, and the extension pins **`^0.3.0`** (spec §10.2/§10.3 say 0.1.11). The extension pins the package in two places: `package.json` and `server/package.json`.
2. **No parser-diff "known divergence" harness exists** (spec §10.2 last line). The only cross-parser check is tree-sitter-craft's `compat.yml`, which parses every `testdata/corpus/**/*.craft` of the `CORPUS_VERSION` tag and fails on any `ERROR`. Nothing to record.
3. **The extension's `TreeSitterCompletionProvider` is text-based and root-only** today (no tree lookup, no alt/opt snippets). Task 26 adds a small context scan.
4. **Hub:** the walk is `branchPaths`, not `branchLabels`. There is no `craft_loader_test.go` (the v2.21 tests are `craft_v221_branches_test.go`), no `make validate-schema` target, and `vendor/` is git-ignored (commit `go.mod`/`go.sum` only).
5. **`operation-not-in-surface`** is implemented tree-based in `LintWorkspace`, not from interactions: its range is the `[op]` bracket, which `Interaction` does not carry.
6. **Data stores reached by `asks`** (`Authentication asks user_db …`) keep their current participant shape. Only hand-off endpoints (`Interaction.Via`) are drawn as `NodeStore`, so no existing golden changes.
7. **Flow:** a store read edge (`store → reader`) is drawn once per listening scenario, not once per writer (one `Listen` exists per writer).
8. **C4:** a store hand-off that is drawn as a database relation is not also routed through the event queue.
9. **Goldens:** `TestGolden` renders every view of every golden model, so `journey-semantics` also gets six C4 goldens next to the six the spec lists.
10. **Opening-block detection skips leading notes:** "the first step" of a scenario means its first step that is not a note (`Scenario.OpeningBlock`).

---

## File map

| File | Responsibility | Tasks |
|---|---|---|
| `internal/lang/lang.go` | `BlockPar`/`BlockLoop`, `TriggerReceive`, `NoteKind`, `SurfaceOpKind`, `StoreTransport`/`StoreChange`, `UnknownCaller`, `ParseStoreRef` | 1, 12 |
| `internal/model/model.go` | `Note`, `Step.Note`, `UseCase.Notes`, `Block.Split`, `Trigger.Operation/Store`, `Action.Store`, `StoreRef`, `Surface`, `SurfaceOperation`, `CraftDoc.Surfaces`, `Scenario.OpeningBlock` | 1 |
| `pkg/craft/craftdoc.go` | aliases, constants, stability note | 1 |
| `internal/syntax/kind.go` | 11 token kinds, 4 node kinds | 2 |
| `internal/syntax/parser.go` | notes, `par`/`and`/`loop`, `receives`, listens tail, `surface` | 2, 5, 9, 11, 14 |
| `internal/syntax/ast.go`, `model_views.go` | `NoteDecl`, `StepDecl.Note`, `UseCaseDecl.Notes`, `BlockDecl.IsFanOut`, trigger op/receive/listen-schedule views, `SurfaceDecl`, `SurfaceOpDecl` | 2, 5, 9, 11, 14 |
| `internal/analysis/project.go`, `merge.go`, `resolve.go` | project notes, split par, receive, listen schedule, store refs, surfaces; resolve stores and surfaces | 3, 6, 9, 11, 12, 15 |
| `internal/sema/sema.go`, `validate.go`, `lint.go` | surface symbols and checks, listen schedule check, 5 lints | 6, 9, 11, 12, 15, 16, 17 |
| `internal/interaction/interaction.go` | par/loop stacks and triggers, `Split`, `Unobserved`, `Via`, `Schedule` | 7, 10, 11, 12 |
| `internal/visualizer/view/{view,sequence,flow,c4}.go` | notes, par/loop, found/lost, stores | 4, 8, 10, 11, 13 |
| `internal/visualizer/view/surface.go`, `internal/visualizer/markdown/surface.go`, `visualizer.go` | surface coverage + Markdown | 16 |
| `internal/visualizer/plantuml/plantuml.go`, `mermaid/{views,mermaid}.go` | writers | 4, 8, 10, 13 |
| `cmd/craft/generate.go` | `--type surface` | 16 |
| `internal/lsp/*` | formatter, outline, folding, hover, completion, definition, semantic tokens | 18, 19, 20 |
| `internal/workspace/workspace.go` | `Doc()` (merged resolved doc) | 20 |
| `testdata/corpus/04_use_cases/*`, `testdata/corpus/09_surfaces/*`, `testdata/broken/*` | fixtures | 3, 6, 9, 11, 12, 14, 15 |
| `internal/visualizer/testdata/journey-semantics.craft` + goldens | diagram goldens | 21 |
| `docs/GRAMMAR.md`, `docs/DIAGNOSTICS.md`, `CONTEXT.md`, `README.md`, `docs/page/language/use-cases.md`, `CHANGELOG.md` | docs | 22, 24 |
| `../tree-sitter-craft/{grammar.js,queries/highlights.scm,test/corpus/*,CORPUS_VERSION}` | highlighting grammar | 25 |
| `../craft-vscode-extension/{package.json,server/package.json,server/src/parser/TreeSitterCompletionProvider.ts}` | extension | 26 |
| `svc-re-go-arch-context/internal/infrastructure/graphstore/*`, `internal/core/*`, `internal/presentation/*` | hub consumer | 27, 28 |

## Task overview

| # | Title | Model |
|---|---|---|
| 1 | Vocabulary and model types, no behaviour change | cheap |
| 2 | Syntax kinds; `note`/`finding` in the parser | strong |
| 3 | Project notes into the model | cheap |
| 4 | Sequence view draws notes and findings | strong |
| 5 | `par`/`and`/`loop` syntax and diagnostics | strong |
| 6 | Project `par`/`loop` (fan-out split); `single-branch-par` lint | cheap |
| 7 | Interaction semantics for `par`/`loop` | strong |
| 8 | Render `par`/`loop` in sequence and flow | strong |
| 9 | `receives` trigger: syntax, model, sema, `receives-on-actor` | strong |
| 10 | `receives` interaction and found/lost rendering | strong |
| 11 | Listens schedule tail | cheap |
| 12 | Store refs: model, resolution, `Via`, `store-without-change` | cheap |
| 13 | Store rendering (sequence, flow, architecture, C4) | strong |
| 14 | `surface` syntax and diagnostics | strong |
| 15 | Surface model, resolution and sema checks | strong |
| 16 | Surface coverage, Markdown, `generate --type surface`, `operation-not-in-surface` | strong |
| 17 | `repeated-request` lint | cheap |
| 18 | Formatter and fuzz | strong |
| 19 | LSP for the use-case constructs | strong |
| 20 | LSP for surfaces and store refs | strong |
| 21 | Visualizer goldens and mermaid-cli check | cheap |
| 22 | Documentation and changelog | cheap |
| 23 | Dogfood on re-arch-central (verification only) | cheap |
| 24 | Release craft v2.23.0 | cheap |
| 25 | tree-sitter-craft: grammar, corpus, CORPUS_VERSION, release 0.3.0 | strong |
| 26 | VSCode extension: bump and completions | cheap |
| 27 | Hub: bump craft, fix `scenarioLabel`, par/loop, receives, store | strong |
| 28 | Hub: findings and surfaces | strong |

---

### Task 1: Vocabulary and model types, no behaviour change — **cheap**

**Files:**
- Modify: `internal/lang/lang.go`, `internal/model/model.go`, `pkg/craft/craftdoc.go`
- Test: `internal/model/model_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/model/model_test.go`)

```go
func TestStepNote_IsNotAnAction(t *testing.T) {
	note := model.Step{Note: &model.Note{ID: "note_3", Kind: model.NoteKindFinding, Text: "no retry"}}
	sc := model.Scenario{Steps: []model.Step{
		note,
		{Action: &model.Action{ID: "action_4"}},
		{Block: &model.Block{ID: "block_5", Kind: model.BlockPar, Split: true, Branches: []model.Branch{
			{Label: "x", Steps: []model.Step{{Note: &model.Note{ID: "note_6"}}}},
			{Label: "x", Steps: []model.Step{{Action: &model.Action{ID: "action_7"}}}},
		}}},
	}}
	var got []string
	for _, a := range sc.Actions() {
		got = append(got, a.ID)
	}
	if j := join(got); j != "action_4 action_7" {
		t.Errorf("Actions() = %q", j)
	}
}

func TestScenarioOpeningBlock_SkipsLeadingNotes(t *testing.T) {
	blk := &model.Block{ID: "block_3", Kind: model.BlockLoop}
	sc := model.Scenario{Steps: []model.Step{{Note: &model.Note{}}, {Block: blk}}}
	if sc.OpeningBlock() != blk {
		t.Error("OpeningBlock should skip the leading note")
	}
	sc = model.Scenario{Steps: []model.Step{{Note: &model.Note{}}, {Action: &model.Action{}}, {Block: blk}}}
	if sc.OpeningBlock() != nil {
		t.Error("a scenario that opens with an action has no opening block")
	}
}

func TestNewFields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.CraftDoc{UseCases: []model.UseCase{{Name: "U", Scenarios: []model.Scenario{{
		ID: "scenario_1", Trigger: model.Trigger{Type: model.TriggerTypeExternal},
		Steps: []model.Step{{Action: &model.Action{ID: "action_2"}}, {Block: &model.Block{ID: "block_3", Kind: model.BlockAlt}}},
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"notes"`, `"note"`, `"split"`, `"store"`, `"surfaces"`, `"operation"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}
```

Add `encoding/json` and `strings` to the test imports.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/model/`
Expected: FAIL to compile (`model.Note`, `model.BlockPar`, `OpeningBlock` undefined).

- [ ] **Step 3: Add the vocabulary** (`internal/lang/lang.go`)

Extend the existing const blocks and add the new types exactly as spec §3.3:

```go
const (
	TriggerExternal     TriggerKind = "external"      // when Actor does …
	TriggerEvent        TriggerKind = "event"         // when "E"
	TriggerDomainListen TriggerKind = "domain_listen" // when Ctx listens "E"
	TriggerReceive      TriggerKind = "receive"       // when <ctx> receives <phrase> [op]
)

const (
	BlockAlt  BlockKind = "alt"  // exactly one of several branches runs
	BlockOpt  BlockKind = "opt"  // the single branch runs only under its condition
	BlockPar  BlockKind = "par"  // every branch runs, concurrently
	BlockLoop BlockKind = "loop" // the single branch repeats while its condition holds
)

// NoteKind is the kind of a note statement.
type NoteKind string

const (
	NoteText    NoteKind = "note"    // context for the reader
	NoteFinding NoteKind = "finding" // a defect, risk or negative fact the hub lists
)

// SurfaceOpKind is the kind of a surface operation line.
type SurfaceOpKind string

const (
	SurfaceQuery   SurfaceOpKind = "query"   // no side effect (Tier A)
	SurfaceCommand SurfaceOpKind = "command" // changes state (Tier B)
)

// StoreTransport is the ref prefix of a store hand-off: store.<name>.<change>.
const StoreTransport = "store"

// StoreChange is the change a store hand-off reports.
type StoreChange string

const (
	StoreInserted StoreChange = "inserted"
	StoreUpdated  StoreChange = "updated"
	StoreDeleted  StoreChange = "deleted"
)

// UnknownCaller is the surface `callers:` value for "no caller observed".
const UnknownCaller = "unknown"
```

- [ ] **Step 4: Add the model types** (`internal/model/model.go`)

1. `CraftDoc` gains, after `Glossary`: `Surfaces []Surface \`json:"surfaces,omitempty"\``.
2. `UseCase` gains, after `TagValues`: `// Notes are the use-case-level note and finding lines, written before the first scenario.` `Notes []Note \`json:"notes,omitempty"\``.
3. Replace `Step` and extend `Block`:

```go
// Step is exactly one of Action, Block or Note.
type Step struct {
	Action *Action `json:"action,omitempty"`
	Block  *Block  `json:"block,omitempty"`
	Note   *Note   `json:"note,omitempty"`
}
```

   In `Block`, after `Branches`:

```go
	// Split is true for a par written without `and`: each Branch holds one step
	// of the header's body, and every Branch carries the header's Label.
	Split bool `json:"split,omitempty"`
```

   Update the `Block` doc comment to name all four kinds, and add `BlockPar = lang.BlockPar`, `BlockLoop = lang.BlockLoop` to the `BlockAlt` const block.
4. New types and aliases (next to `Block`):

```go
// NoteKind is the kind of a note statement (note or finding).
type NoteKind = lang.NoteKind

const (
	NoteKindNote    = lang.NoteText
	NoteKindFinding = lang.NoteFinding
)

// Note is a `note "…"` or `finding "…"` line. It is not an action: it gets no
// interaction and does not advance step numbers.
type Note struct {
	ID     string   `json:"id,omitempty"` // "note_<n>" inside a use case; "" in a surface
	Kind   NoteKind `json:"kind"`         // "note" | "finding"
	Text   string   `json:"text"`         // unquoted
	Line   int      `json:"line,omitempty"`
	Column int      `json:"column,omitempty"`
}

// StoreChange is the change of a store hand-off.
type StoreChange = lang.StoreChange

const (
	StoreInserted = lang.StoreInserted
	StoreUpdated  = lang.StoreUpdated
	StoreDeleted  = lang.StoreDeleted
)

// StoreRef is a store hand-off parsed from a store.<name>.<change> ref.
type StoreRef struct {
	Name   string      `json:"name"`
	Change StoreChange `json:"change,omitempty"`
	// Participant is the data store the name resolves to (a `data-stores:`
	// entry, rulings 9 and 11); nil when it does not resolve.
	Participant *Participant `json:"participant,omitempty"`
}

// SurfaceOpKind is the kind of a surface operation (query or command).
type SurfaceOpKind = lang.SurfaceOpKind

const (
	SurfaceQuery   = lang.SurfaceQuery
	SurfaceCommand = lang.SurfaceCommand
)

// UnknownCaller is the `callers:` value that says no caller was observed.
const UnknownCaller = lang.UnknownCaller

// Surface lists the operations one boundary offers (`surface <boundary> { … }`).
type Surface struct {
	Subject     string             `json:"subject"`               // as written
	Participant *Participant       `json:"participant,omitempty"` // resolved boundary
	Callers     []string           `json:"callers,omitempty"`     // as written; "unknown" kept literally
	Spec        string             `json:"spec,omitempty"`
	Notes       []Note             `json:"notes,omitempty"`
	Operations  []SurfaceOperation `json:"operations"`
	SourceURI   string             `json:"sourceUri,omitempty"`
	Line        int                `json:"line,omitempty"`
}

// SurfaceOperation is one `query`/`command` line of a surface.
type SurfaceOperation struct {
	Kind      SurfaceOpKind `json:"kind"` // "query" | "command"
	Connector string        `json:"connector,omitempty"`
	Phrase    string        `json:"phrase"`
	Operation Operation     `json:"operation"` // same parsing as an action's bracket
	Line      int           `json:"line,omitempty"`
}

// OpeningBlock returns the block the scenario opens with: its first step that
// is not a note, when that step is a block. Nil otherwise.
func (s Scenario) OpeningBlock() *Block {
	for _, st := range s.Steps {
		if st.Note != nil {
			continue
		}
		return st.Block
	}
	return nil
}
```

5. `Trigger` gains, after `ScheduleKind` (and update the `Schedule` comment to say it is also set for a `listens` trigger with a trailing schedule):

```go
	// Operation is the `[...]` bracket of a `receives` trigger; nil otherwise.
	Operation *Operation `json:"operation,omitempty"`
	// Store is set on a `listens` trigger whose ref is a store ref.
	Store *StoreRef `json:"store,omitempty"`
```

   and `TriggerTypeReceive = lang.TriggerReceive` in the `TriggerType` const block.
6. `Action` gains, after `Operation`: `// Store is set on an async action whose ref is a store ref.` `Store *StoreRef \`json:"store,omitempty"\``.
7. `eachAction` already ignores anything that is not an action or a block, so `Actions()`/`EachAction` skip notes. Add a sentence to `Actions()`'s comment: "Notes are skipped."

- [ ] **Step 5: `pkg/craft` aliases** (`pkg/craft/craftdoc.go`)

Add to the type block: `Note = model.Note`, `NoteKind = model.NoteKind`, `Surface = model.Surface`, `SurfaceOperation = model.SurfaceOperation`, `SurfaceOpKind = model.SurfaceOpKind`, `StoreRef = model.StoreRef`, `StoreChange = model.StoreChange`. Add to the const block: `TriggerTypeReceive`, `BlockPar`, `BlockLoop`, `NoteKindNote`, `NoteKindFinding`, `SurfaceQuery`, `SurfaceCommand`, `StoreInserted`, `StoreUpdated`, `StoreDeleted`, `UnknownCaller` (each `= model.X`). Append to the package doc, after the v2.21.0 paragraph:

```go
// Step is exactly one of Action, Block or Note (Note since v2.23.0); a
// consumer that assumes Action-or-Block must handle a Note.
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/model/ ./pkg/craft/` → PASS. Then `go vet ./... && go test ./...` → PASS with **no** golden change (`git status --short testdata internal/visualizer/testdata` is empty).

- [ ] **Step 7: Commit**

```bash
git add internal/lang internal/model pkg/craft
git commit -m "feat(model): vocabulary and model types for journey semantics

Note, Step.Note, UseCase.Notes, Block.Split, Trigger.Operation/Store,
Action.Store, StoreRef, Surface, SurfaceOperation, CraftDoc.Surfaces,
Scenario.OpeningBlock, and the pkg/craft aliases. Nothing produces them yet.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Syntax kinds; `note`/`finding` in the parser — **strong**

**Files:**
- Modify: `internal/syntax/kind.go`, `internal/syntax/parser.go` (`parseUseCaseBlock` ~L845, `parseSteps` ~L1218, `parseBranch` ~L1294), `internal/syntax/ast.go` (`steps` ~L1449, `StepDecl`, `UseCaseDecl`)
- Modify (test helpers that switch on step kinds): `internal/syntax/blocks_test.go` (`outline`), `internal/lsp/formatter_test.go` (`stepOutline`), `internal/lsp/formatter_fuzz_test.go` (`fuzzOutline`), `internal/lsp/formatter.go` (`firstBodyToken`)
- Create: `internal/syntax/notes_test.go`, `testdata/broken/{note_missing_text,note_trailing_tokens}.craft` + `.diagnostics.json`

**Tree shape** (pin this; Tasks 3, 4 and 18 depend on it):

```
UseCaseDecl
  KwUseCase String LBrace
  TagsBlock?
  Note: KwFinding String        (use-case level, before the first scenario)
  Scenario
    Trigger
    Action
    Note: KwNote String          (a step)
    Block > Branch > BranchHeader, Note …
```

- [ ] **Step 1: Add every new syntax kind now** (`internal/syntax/kind.go`)

Append just before `syntaxKindTokenSentinel`, in this order (spec §3.2):

```go
	// Journey-semantics contextual keywords (v2.23.0). Each is matched by value
	// from TokenIdent only in its slot (spec §3.1); elsewhere the word stays an
	// identifier.
	SyntaxKindKwPar      // `par` block header
	SyntaxKindKwAnd      // `and` branch of a par
	SyntaxKindKwLoop     // `loop` block header
	SyntaxKindKwNote     // `note "…"`
	SyntaxKindKwFinding  // `finding "…"`
	SyntaxKindKwReceives // `when <ctx> receives …`
	SyntaxKindKwSurface  // top-level `surface <name> {`
	SyntaxKindKwCallers  // `callers:` in a surface
	SyntaxKindKwSpec     // `spec:` in a surface
	SyntaxKindKwQuery    // `query` surface line
	SyntaxKindKwCommand  // `command` surface line
```

Append at the end of the node block:

```go
	// SyntaxKindNote is a `note "…"` / `finding "…"` statement: a step, a
	// use-case-level line, or a surface line.
	SyntaxKindNote
	// SyntaxKindSurfaceDecl is `surface <name> { … }`.
	SyntaxKindSurfaceDecl
	// SyntaxKindSurfaceField wraps `callers: …` or `spec: "…"`.
	SyntaxKindSurfaceField
	// SyntaxKindSurfaceOperation wraps a `query|command <phrase> [op]` line.
	SyntaxKindSurfaceOperation
```

Run `go test ./internal/syntax/ -run Kind` → PASS (the existing kind tests check the token/node boundary).

- [ ] **Step 2: Write the failing tests** (`internal/syntax/notes_test.go`)

```go
package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const notesSrc = `use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session"
        alt "the card is declined"
            finding "no idempotency key"
}
`

func TestNotes_StepsAndUseCaseLevel(t *testing.T) {
	g, _, diags := syntax.Parse(notesSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	uc := syntax.AsFile(syntax.Root(g)).UseCases()[0]
	ns := uc.Notes()
	if len(ns) != 1 || ns[0].Kind() != lang.NoteFinding || ns[0].Text() != "nothing retries a failed callback" {
		t.Fatalf("use-case notes = %+v", ns)
	}
	sc := uc.Scenarios()[0]
	want := "action frontendplatform\nnote note the gateway forwards the seller's session\nalt\n  branch the card is declined\n    note finding no idempotency key\n"
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	if n := len(sc.Actions()); n != 1 {
		t.Errorf("Actions() = %d, want 1 (notes are not actions)", n)
	}
}

func TestNotes_KeywordsStayNames(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    note asks finding to run\n    finding records something\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := outline(sc.Steps(), 0); got != "action note\naction finding\n" {
		t.Errorf("tree:\n%s", got)
	}
}

func TestNotes_Diagnostics(t *testing.T) {
	cases := map[string]string{
		"craft/syntax/missing-label":    "use_case \"U\" {\n  when O does\n    x does it\n    finding\n}",
		"craft/syntax/unexpected-token": "use_case \"U\" {\n  when O does\n    x does it\n    note \"a\" twice\n}",
	}
	for code, src := range cases {
		_, _, diags := syntax.Parse(src)
		found := false
		for _, d := range diags {
			found = found || d.Code == code
		}
		if !found {
			t.Errorf("%s: not reported; got %v", code, diags)
		}
	}
}
```

Update `outline` in `internal/syntax/blocks_test.go` so it never calls `Block()` on a note:

```go
		if n, ok := st.Note(); ok {
			b.WriteString(ind + "note " + string(n.Kind()) + " " + n.Text() + "\n")
			continue
		}
```

(insert after the action case). Make the same insertion in `stepOutline` (`internal/lsp/formatter_test.go` ~L1945) and `fuzzOutline` (`internal/lsp/formatter_fuzz_test.go` ~L15), writing `ind + "note " + string(n.Kind()) + " " + strconv.Quote(n.Text())`.

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestNotes_`
Expected: FAIL to compile (`Notes`, `Note` undefined).

- [ ] **Step 4: Implement the parser**

1. Extract the trailing-token sweep from `parseBranch` into a helper and call it from `parseBranch` (behaviour unchanged):

```go
// sweepRestOfLine wraps whatever is left on line in an error node and reports
// its first token as unexpected. A `}` or EOF ends the sweep.
func (p *Parser) sweepRestOfLine(line int, expected string) []model.Diagnostic {
	t := p.peek()
	if t.Line != line || t.Type == lexer.TokenEOF || t.Type == lexer.TokenRBrace {
		return nil
	}
	d := p.diagUnexpected(t, expected)
	p.builder.StartNode(SyntaxKindErrorNode)
	for t := p.peek(); t.Line == line && t.Type != lexer.TokenEOF && t.Type != lexer.TokenRBrace; t = p.peek() {
		p.consume()
	}
	p.builder.FinishNode()
	return []model.Diagnostic{d}
}
```

2. Recognise and parse a note:

```go
// atNoteLine reports whether the next tokens are a note statement: `note` or
// `finding` at the start of a line, followed by a string on that line or by
// nothing else on it (a missing text). Anything else after the word makes the
// line an ordinary action, so both words stay valid names.
func (p *Parser) atNoteLine() bool {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || (tok.Value != "note" && tok.Value != "finding") || !p.peekAtLineStart() {
		return false
	}
	next := p.peekAt(1)
	return next.Type == lexer.TokenString || next.Type == lexer.TokenEOF || next.Line != tok.Line
}

// parseNote parses `note "<text>"` or `finding "<text>"`. A note is one line.
func (p *Parser) parseNote() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindNote)
	kind := SyntaxKindKwNote
	if p.peek().Value == "finding" {
		kind = SyntaxKindKwFinding
	}
	kwTok := p.consumeAs(kind)
	if t := p.peek(); t.Type == lexer.TokenString && t.Line == kwTok.Line {
		p.consumeAs(SyntaxKindString)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-label",
			Message:  fmt.Sprintf("`%s` needs a quoted text, e.g. `%s \"a lost callback leaves the invoice open\"`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	diags = append(diags, p.sweepRestOfLine(kwTok.Line, "end of line after the note text")...)
	p.builder.FinishNode()
	return diags
}
```

3. `parseSteps`: in the `if !isHdr {` branch, parse a note instead of an action when `p.atNoteLine()`:

```go
		if !isHdr {
			if p.atNoteLine() {
				diags = append(diags, p.parseNote()...)
			} else {
				diags = append(diags, p.parseAction()...)
			}
			continue
		}
```

   The offside rule and `inconsistent-indent` already apply, because the column logic runs before this branch.
4. `parseUseCaseBlock`: add a branch after the `tags` one: `} else if p.atNoteLine() { diags = append(diags, p.parseNote()...)`. Update the unexpected-token hint to "`when`, `tags`, `note`, `finding`, or `}`".

- [ ] **Step 5: Implement the typed views** (`internal/syntax/ast.go`)

```go
// NoteDecl is a typed view over a SyntaxKindNote node: `note "…"` or `finding "…"`.
type NoteDecl struct{ node SyntaxNode }

// KeywordToken returns the `note` / `finding` keyword.
func (n NoteDecl) KeywordToken() *SyntaxToken {
	return n.node.ChildToken(SyntaxKindKwNote, SyntaxKindKwFinding)
}

// Kind returns lang.NoteFinding for `finding`, lang.NoteText otherwise.
func (n NoteDecl) Kind() lang.NoteKind {
	if t := n.KeywordToken(); t != nil && t.Kind() == SyntaxKindKwFinding {
		return lang.NoteFinding
	}
	return lang.NoteText
}

// Text returns the unquoted text, or "" when it is missing.
func (n NoteDecl) Text() string {
	if t := n.node.ChildToken(SyntaxKindString); t != nil {
		return unquoteStringText(t.Text())
	}
	return ""
}

// Node returns the underlying syntax node.
func (n NoteDecl) Node() SyntaxNode { return n.node }

// Note returns the step as a note, if it is one.
func (s StepDecl) Note() (NoteDecl, bool) {
	if s.node.Kind() != SyntaxKindNote {
		return NoteDecl{}, false
	}
	return NoteDecl{node: s.node}, true
}

// Notes returns the use-case-level note and finding lines, in source order.
func (u UseCaseDecl) Notes() []NoteDecl {
	var out []NoteDecl
	for _, n := range u.node.ChildNodes(SyntaxKindNote) {
		out = append(out, NoteDecl{node: n})
	}
	return out
}
```

In `steps()`, accept `SyntaxKindNote` next to `SyntaxKindAction` and `SyntaxKindBlock`; update the `StepDecl` doc comment ("an action, a block or a note"). `flattenActions` needs no change. In `internal/lsp/formatter.go` `firstBodyToken`, accept `SyntaxKindNote` too, so a body that starts with a note keeps its column.

- [ ] **Step 6: Broken fixtures**

`testdata/broken/note_missing_text.craft`:

```craft
use_case "U" {
    when Ops starts it
        x does a
        finding
}
```

`.diagnostics.json`: one entry, `craft/syntax/missing-label`, error, `{"start":{"line":3,"character":8},"end":{"line":3,"character":15}}`.

`testdata/broken/note_trailing_tokens.craft`:

```craft
use_case "U" {
    when Ops starts it
        x does a
        note "the gateway forwards the session" twice
}
```

`.diagnostics.json`: one entry, `craft/syntax/unexpected-token`, error, `{"start":{"line":3,"character":48},"end":{"line":3,"character":53}}` (the word `twice`). Copy the JSON shape from `testdata/broken/orphan_else.diagnostics.json`. Run `go test ./internal/corpustest/ -run TestHarnessC`; adjust only columns if the harness disagrees, and check each range by eye.

- [ ] **Step 7: Full suite and commit**

Run: `go vet ./... && go test ./...` → PASS; corpus goldens unchanged (projection ignores notes until Task 3).

```bash
git add internal testdata
git commit -m "feat(syntax): note and finding statements

Contextual keywords note/finding as steps and at use-case level; also adds
every syntax kind of the journey-semantics release.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Project notes into the model — **cheap**

**Files:**
- Modify: `internal/analysis/project.go` (`projectUseCases`, `projectSteps`)
- Create: `testdata/corpus/04_use_cases/notes_findings.craft` + `.craftjson`

- [ ] **Step 1: Write the fixture (red)**

`testdata/corpus/04_use_cases/notes_findings.craft`:

```craft
actor user Seller

domain payments {
    frontendplatform
    graphql
    paymentgateway
}

use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback; a lost callback leaves the invoice open"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session, not a service token"
        graphql asks paymentgateway to start the payment  [POST /payments]
        finding "no idempotency key: a client retry starts a second payment"
        alt "the payment provider is down"
            note "nothing is retried"
}
```

Run `go test ./internal/corpustest/ -run TestHarnessA -update` and open the new golden: it has no `notes`/`note` keys yet (red). Delete it: `rm testdata/corpus/04_use_cases/notes_findings.craftjson`.

- [ ] **Step 2: Implement**

```go
// projectNote projects one note or finding line. id is "" in a surface.
func projectNote(n syntax.NoteDecl, id string, li green.LineIndex) model.Note {
	out := model.Note{ID: id, Kind: n.Kind(), Text: n.Text()}
	if tok := n.KeywordToken(); tok != nil {
		out.Line, out.Column = li.LineCol16(tok.Offset())
	}
	return out
}
```

In `projectSteps`, before the action case:

```go
		if n, ok := st.Note(); ok {
			*counter++
			note := projectNote(n, fmt.Sprintf("note_%d", *counter), li)
			out = append(out, model.Step{Note: &note})
			continue
		}
```

In `projectUseCases`, after the tags block and before the scenario loop (use-case notes precede every scenario in source order, so they take their numbers first):

```go
		for _, n := range uc.Notes() {
			counter++
			outUC.Notes = append(outUC.Notes, projectNote(n, fmt.Sprintf("note_%d", counter), li))
		}
```

Update the `projectUseCases` doc comment: scenarios, actions, blocks and notes share the counter.

- [ ] **Step 3: Regenerate and review**

Run the `-update` command. In `notes_findings.craftjson` check: use case `"notes": [{"id": "note_1", "kind": "finding", …, "line": 10}]`; `scenario_2`; steps `action_3`, `note_4` (kind `note`), `action_5`, `note_6` (kind `finding`), `block_7` whose only branch holds `note_8`. `git diff --stat testdata/corpus` shows only the new file.

- [ ] **Step 4: Full suite and commit**

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(model): project notes and findings (Step.Note, UseCase.Notes)

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Sequence view draws notes and findings — **strong**

**Files:**
- Modify: `internal/visualizer/view/sequence.go`, `internal/visualizer/plantuml/plantuml.go` (`Sequence`), `internal/visualizer/mermaid/views.go` (`Sequence`)
- Test: `internal/visualizer/view/sequence_test.go`, `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests**

View test (append to `internal/visualizer/view/sequence_test.go`):

```go
func TestNotes_AnchorOverThePreviousMessage(t *testing.T) {
	src := `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "U" {
    finding "use-case level"
    when Customer places an order
        note "after the trigger"
        Ordering asks Billing to charge
        alt "declined"
            Billing returns the refusal
        note "after the block"
        opt "audit"
            note "only a note"
    when "Order Placed"
        note "nothing drawn yet"
        Ordering records it
}
`
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("t.craft", src)}).Doc
	seq := BuildSequence(doc, interaction.Derive(doc))
	type got struct {
		kind  MessageKind
		label string
		over  string
	}
	var notes []got
	for _, m := range seq.Sections[0].Messages {
		if m.Kind == MessageNone {
			t.Errorf("unexpected MessageNone: a branch holding only a note is not empty")
		}
		if m.Kind == MessageStepNote || m.Kind == MessageFinding {
			notes = append(notes, got{m.Kind, m.Label, strings.Join(m.Over, ",")})
		}
	}
	want := []got{
		{MessageFinding, "use-case level", ""},
		{MessageStepNote, "after the trigger", "Customer,shop/Ordering"},
		{MessageStepNote, "after the block", "shop/Billing,shop/Ordering"},
		{MessageStepNote, "only a note", "shop/Billing,shop/Ordering"},
		{MessageStepNote, "nothing drawn yet", ""},
	}
	if !slices.Equal(notes, want) {
		t.Errorf("notes:\n%v\nwant:\n%v", notes, want)
	}
}
```

Extend `TestSequenceStepsMatchInteractionSteps`: add a `note "x"` line inside branch `"a"` and a `finding "y"` line after `Ordering asks Billing to sixth`, and change its `got` filter to skip notes: `if m.Scenario == title && m.Kind != MessageNone && m.Kind != MessageStepNote && m.Kind != MessageFinding`.

Rendering test (append to `internal/visualizer/sequence_rendering_test.go`):

```go
const notesSrc = `actor user Seller
domain payments {
    frontendplatform
    graphql
    paymentgateway
}
use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback; a lost callback leaves the invoice open"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session, not a service token"
        graphql asks paymentgateway to start the payment  [POST /payments]
        finding "no idempotency key: a client retry starts a second payment"
}
`

func TestSequence_NotesAndFindings(t *testing.T) {
	puml, mmd := sequenceSource(t, notesSrc)
	for _, want := range []string{
		"note over Seller, paymentgateway #FFE4E1 : Finding: nothing retries a failed callback; a lost callback leaves the invoice open",
		"note over frontendplatform, graphql : the gateway forwards the seller's session, not a service token",
		"note over graphql, paymentgateway #FFE4E1 : Finding: no idempotency key: a client retry starts a second payment",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{
		"Note over Seller,paymentgateway: Finding: nothing retries a failed callback, a lost callback leaves the invoice open",
		"Note over frontendplatform,graphql: the gateway forwards the seller's session, not a service token",
		"Note over graphql,paymentgateway: Finding: no idempotency key: a client retry starts a second payment",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
	// The use-case finding comes before the trigger.
	if strings.Index(puml, "Finding: nothing retries") > strings.Index(puml, "Seller -> frontendplatform") {
		t.Errorf("use-case-level finding must open the section:\n%s", puml)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/visualizer/... -run 'Notes|NotesAndFindings|StepsMatch'` → FAIL (`MessageStepNote` undefined).

- [ ] **Step 3: Implement the view** (`sequence.go`)

1. `MessageKind` gains `MessageStepNote MessageKind = "step-note"` (a `note` line) and `MessageFinding MessageKind = "finding"`. `Message` gains:

```go
	// Over lists the participant IDs a MessageStepNote or MessageFinding is
	// drawn over; empty means across the diagram (first to last participant).
	Over []string
```

2. `scenarioLayout` gains `start int` (index of the scenario's first message in the section). Set it where the layout is built in `BuildSequence`: `l := scenarioLayout{…, start: len(b.seq.Sections[idx].Messages)}`.
3. Use-case-level notes: in `BuildSequence`, before the scenario loop of each use case, append one message per `uc.Notes` entry to `b.seq.Sections[idx].Messages`: `Message{Kind: noteKind(n), Label: n.Text}` (no Scenario, no Over).
4. In `scenarioLayout.walk`, add `case st.Note != nil: l.note(*st.Note, path)`.

```go
func noteKind(n model.Note) MessageKind {
	if n.Kind == model.NoteKindFinding {
		return MessageFinding
	}
	return MessageStepNote
}

// note draws a note over the participants of the last message drawn before it
// in this scenario (spec §5.4 anchor rule).
func (l *scenarioLayout) note(n model.Note, path []BranchRef) {
	sec := &l.b.seq.Sections[l.idx]
	sec.Messages = append(sec.Messages, Message{
		Kind: noteKind(n), Label: n.Text, Scenario: l.title,
		Branch: slices.Clone(path), Over: l.anchor(),
	})
}

// anchor returns the participants of the last drawn message of this scenario:
// From and To, deduplicated; only To for a message from outside the section,
// only From for one leaving it. Nil when nothing has been drawn yet.
func (l *scenarioLayout) anchor() []string {
	msgs := l.b.seq.Sections[l.idx].Messages
	for i := len(msgs) - 1; i >= l.start; i-- {
		m := msgs[i]
		switch m.Kind {
		case MessageNone, MessageStepNote, MessageFinding:
			continue
		case MessageInbound:
			return []string{m.To}
		case MessageNote:
			return []string{m.From}
		}
		if m.From == m.To {
			return []string{m.From}
		}
		return []string{m.From, m.To}
	}
	return nil
}
```

   (Task 10 adds `MessageFound` → `[]string{m.To}` and `MessageLost` → `[]string{m.From}` to the switch.)
5. The empty-branch check already counts any appended message, so a branch holding only a note gets no `MessageNone`.

- [ ] **Step 4: Implement the writers**

PlantUML, in the message switch:

```go
			case view.MessageStepNote, view.MessageFinding:
				over := noteOver(s, m.Over, al, ", ")
				if over == "" {
					continue
				}
				colour, label := "", m.Label
				if m.Kind == view.MessageFinding {
					colour, label = " #FFE4E1", "Finding: "+m.Label
				}
				fmt.Fprintf(&b, "note over %s%s : %s\n", over, colour, label)
```

Mermaid: `Note over %s: %s` with `noteOver(s, m.Over, al, ",")` and the `Finding: ` prefix (no colour; `label` already has `;` replaced). Both writers share the rule; put one copy in each package:

```go
// noteOver spells the participants a note is drawn over: the listed ones, or
// the first and last participant of the diagram when the list is empty.
func noteOver(s view.Sequence, over []string, al *aliases, sep string) string {
	if len(over) == 0 {
		switch len(s.Participants) {
		case 0:
			return ""
		case 1:
			return al.of(s.Participants[0].ID)
		}
		over = []string{s.Participants[0].ID, s.Participants[len(s.Participants)-1].ID}
	}
	names := make([]string, len(over))
	for i, id := range over {
		names[i] = al.of(id)
	}
	return strings.Join(names, sep)
}
```

Guard `spanAll` in the Mermaid writer against zero participants (return `""` and skip the scenario note).

- [ ] **Step 5: Run** the tests → PASS. `go test ./internal/visualizer/ -run TestGolden` → no golden change. Full suite.

- [ ] **Step 6: Commit**

```bash
git add internal/visualizer
git commit -m "feat(sequence): draw notes and findings over the previous message

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: `par`/`and`/`loop` syntax and diagnostics — **strong**

**Files:**
- Modify: `internal/syntax/parser.go` (`isHeader`, `parseSteps`, `parseBlock`, `parseBranch`), `internal/syntax/ast.go` (`BlockDecl.Kind`, `BlockDecl.IsFanOut`, `BranchDecl.HeaderToken`/`Keyword`)
- Test: `internal/syntax/blocks_test.go`
- Create: `testdata/broken/{orphan_and,else_under_par,par_missing_label}.craft` + `.diagnostics.json`

- [ ] **Step 1: Write the failing tests** (append to `blocks_test.go`)

```go
const parSrc = `use_case "U" {
    when Seeker opens an advert
        fp asks graphql for the advert
        par "field resolvers"
            graphql asks atlas for a
            graphql asks atlas for b
        par "the advert"
            graphql asks adcore for the advert
            graphql asks atlas for the owner
        and "the statistics"
            graphql asks dapi for stats
        loop "for each image"
            graphql asks media to sign
            alt "signing fails"
                graphql logs it
}
`

func TestBlocks_ParAndLoop(t *testing.T) {
	got, diags := outlineAll(t, parSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	want := `when
  action fp
  par
    branch field resolvers
      action graphql
      action graphql
  par
    branch the advert
      action graphql
      action graphql
    branch the statistics
      action graphql
  loop
    branch for each image
      action graphql
      alt
        branch signing fails
          action graphql
`
	if got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	g, _, _ := syntax.Parse(parSrc)
	steps := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Steps()
	fan, _ := steps[1].Block()
	explicit, _ := steps[2].Block()
	loop, _ := steps[3].Block()
	if !fan.IsFanOut() || explicit.IsFanOut() || loop.IsFanOut() {
		t.Errorf("IsFanOut: fan=%v explicit=%v loop=%v", fan.IsFanOut(), explicit.IsFanOut(), loop.IsFanOut())
	}
}

func TestBlocks_ParKeywordsStayNames(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    par asks loop to run\n    and records something\n    loop does it\n}"
	got, diags := outlineAll(t, src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	if got != "when\n  action par\n  action and\n  action loop\n" {
		t.Errorf("tree:\n%s", got)
	}
}

func TestBlocks_ParDiagnostics(t *testing.T) {
	cases := []struct{ code, src string }{
		{"craft/syntax/orphan-and", "use_case \"U\" {\n  when O does\n    x does it\n    and \"b\"\n      x does more\n}"},
		{"craft/syntax/orphan-and", "use_case \"U\" {\n  when O does\n    alt \"a\"\n      x does it\n    and \"b\"\n      x does more\n}"},
		{"craft/syntax/orphan-else", "use_case \"U\" {\n  when O does\n    par \"a\"\n      x does it\n    else \"b\"\n      x does more\n}"},
		{"craft/syntax/orphan-else", "use_case \"U\" {\n  when O does\n    loop \"a\"\n      x does it\n    else \"b\"\n      x does more\n}"},
		{"craft/syntax/missing-label", "use_case \"U\" {\n  when O does\n    loop\n      x does it\n}"},
		{"craft/syntax/empty-branch", "use_case \"U\" {\n  when O does\n    par \"a\"\n    x does it\n}"},
	}
	for _, c := range cases {
		_, _, diags := syntax.Parse(c.src)
		found := false
		for _, d := range diags {
			found = found || d.Code == c.code
		}
		if !found {
			t.Errorf("%s not reported for:\n%s\ngot %v", c.code, c.src, diags)
		}
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/syntax/ -run 'TestBlocks_Par'` → FAIL (`IsFanOut` undefined).

- [ ] **Step 3: Implement the parser**

1. `isHeader`: accept `par`, `and`, `loop` with the same "string, or nothing else on the line" rule (`case "alt", "opt", "else", "par", "and", "loop":`).
2. `parseSteps`: the `inconsistent-indent` case skips continuation headers: `case col < bodyCol && kw != "else" && kw != "and":`. Replace the orphan block with:

```go
		switch kw {
		case "else":
			// No alt at this column is open (an else at an open alt's column
			// is taken by parseBlock before control gets back here), or the
			// open block is an opt, par or loop.
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/orphan-else",
				Message:  "`else` continues an `alt` at the same column; a `par` continues with `and`",
				Severity: model.SeverityError,
				Range:    tokenRange(tok),
			})
		case "and":
			diags = append(diags, model.Diagnostic{
				Code:     "craft/syntax/orphan-and",
				Message:  "`and` continues a `par` at the same column; an `alt` continues with `else`",
				Severity: model.SeverityError,
				Range:    tokenRange(tok),
			})
		}
		diags = append(diags, p.parseBlock(kw, col)...)
```

3. `parseBlock`: the continuation keyword depends on the first header:

```go
	cont := ""
	switch kw {
	case "alt", "else":
		cont = "else"
	case "par", "and":
		cont = "and"
	}
	if cont != "" {
		for {
			next, ok := p.isHeader()
			if !ok || next != cont || p.peekVisualColumn() != col {
				break
			}
			diags = append(diags, p.parseBranch(cont, col)...)
		}
	}
```

   Update its doc comment (an orphan `and` opens a par, as an orphan `else` opens an alt; opt and loop have one branch).
4. `parseBranch`: map the keyword to its kind with a switch over all six (`par` → `SyntaxKindKwPar`, `and` → `SyntaxKindKwAnd`, `loop` → `SyntaxKindKwLoop`), and widen `missing-label`: `fmt.Sprintf("`+"`%s`"+` needs a quoted label, e.g. `+"`%s \"payment fails\"`"+`", kw, kw)` (any equivalent string literal). Use `p.sweepRestOfLine(kwTok.Line, "end of line after the block label")` for the trailing-token sweep.

- [ ] **Step 4: Implement the typed views** (`ast.go`)

```go
// Kind returns the block's kind from its first header: opt, par (also an
// orphan `and`), loop, else alt (also an orphan `else`).
func (b BlockDecl) Kind() lang.BlockKind {
	brs := b.Branches()
	if len(brs) == 0 {
		return lang.BlockAlt
	}
	switch brs[0].Keyword() {
	case "opt":
		return lang.BlockOpt
	case "par", "and":
		return lang.BlockPar
	case "loop":
		return lang.BlockLoop
	}
	return lang.BlockAlt
}

// IsFanOut reports whether the block is a par written without `and`: each
// step of its body runs as its own concurrent branch (spec §4.1 rule 2).
func (b BlockDecl) IsFanOut() bool {
	brs := b.Branches()
	return len(brs) == 1 && brs[0].Keyword() == "par"
}
```

`BranchDecl.HeaderToken` looks for all six keyword kinds; `Keyword()` returns `"par"`, `"and"`, `"loop"` for the new ones (a switch on `t.Kind()`).

- [ ] **Step 5: Broken fixtures**

| File | Source | Diagnostic |
|---|---|---|
| `orphan_and.craft` | `use_case "U" {` / `    when Ops starts it` / `        x does a` / `        and "b"` / `            x does b` / `}` | `craft/syntax/orphan-and`, error, line 3, chars 8–11 |
| `else_under_par.craft` | `use_case "U" {` / `    when Ops starts it` / `        par "a"` / `            x does a` / `        else "b"` / `            x does b` / `}` | `craft/syntax/orphan-else`, error, line 4, chars 8–12 |
| `par_missing_label.craft` | `use_case "U" {` / `    when Ops starts it` / `        par` / `            x does a` / `}` | `craft/syntax/missing-label`, error, line 2, chars 8–11 |

(`/` separates lines; lines and characters are 0-based.) Run `go test ./internal/corpustest/ -run TestHarnessC` and fix only columns.

- [ ] **Step 6: Full suite and commit**

Run `go vet ./... && go test ./...` → PASS, corpus goldens unchanged (no corpus file uses the new headers).

```bash
git add internal testdata
git commit -m "feat(syntax): par/and and loop blocks with the alt/opt offside rule

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Project `par`/`loop` (fan-out split); `single-branch-par` lint — **cheap**

**Files:**
- Modify: `internal/analysis/project.go` (`projectBlock`), `internal/sema/lint.go` (`LintWorkspace`, new `lintSingleBranchPar`)
- Create: `testdata/corpus/04_use_cases/{par_fanout,par_and,loop,nested_par_loop_alt}.craft` + goldens, `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Fixtures (red)**

`par_fanout.craft` is the spec §4.1 first example with `actor user Seeker` and `domain ad { frontendplatform graphql atlas media }` (one name per line). `par_and.craft` puts the §4.1 second snippet (`par "the advert"` … `and "the statistics"`) under `when Seeker opens an advert` with `domain ad { graphql adcore atlas Dapi }`. `loop.craft`:

```craft
actor system CRON

domain obido {
    otodomintegration
    obido
}

use_case "Promotions are synced" {
    when cron "0 * * * *"
        loop "for each ad id of one investment"
            otodomintegration asks obido for the promotion  [getPromotion]
            otodomintegration records the promotion
}
```

`nested_par_loop_alt.craft`: one scenario with `loop "for each owner"` containing `par "lookups"` (two asks) followed by `alt "found"` / `else "missing"` inside the loop body, then a `par "a"` / `and "b"` whose `and` branch holds an `opt`.

Run `-update`: the goldens show `"kind": "par"` but no `"split"`. Delete the four new goldens.

- [ ] **Step 2: Implement the split** (`projectBlock`)

Insert after `mb` is created:

```go
	if b.IsFanOut() {
		br := b.Branches()[0]
		if steps := br.Steps(); len(steps) > 0 {
			// Fan-out: one concurrent branch per body step, every one carrying
			// the header's label (spec §4.3). The block keeps the header's
			// position; each branch takes the line of its step.
			mb.Split = true
			if tok := br.HeaderToken(); tok != nil {
				mb.Line, mb.Column = li.LineCol16(tok.Offset())
			}
			for _, st := range steps {
				mbr := model.Branch{Label: br.Label()}
				if toks := st.Node().Tokens(); len(toks) > 0 {
					mbr.Line, _ = li.LineCol(toks[0].Offset())
				}
				mbr.Steps = projectSteps([]syntax.StepDecl{st}, counter, li)
				mb.Branches = append(mb.Branches, mbr)
			}
			return mb
		}
	}
```

(An empty fan-out body is already an `empty-branch` error; it projects as one empty branch, unsplit.)

- [ ] **Step 3: Regenerate and review**

`par_fanout.craftjson`: `block_N` with `"kind": "par"`, `"split": true`, three branches all labelled `field resolvers`, each with one action whose `line` equals the branch `line`; then a `loop` block with one branch. IDs increase in source order. `par_and.craftjson`: two branches, no `split`. Existing goldens unchanged.

- [ ] **Step 4: Lint test (red)** — create `internal/sema/lint_journey_test.go`:

```go
package sema

import "testing"

func lintCodes(t *testing.T, src, code string) int {
	t.Helper()
	n := 0
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestLint_SingleBranchPar(t *testing.T) {
	one := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      x asks y for a\n}"
	two := "use_case \"U\" {\n  when O does\n    par \"lookups\"\n      x asks y for a\n      x asks y for b\n}"
	explicit := "use_case \"U\" {\n  when O does\n    par \"a\"\n      x asks y for a\n    and \"b\"\n      x asks y for b\n}"
	if n := lintCodes(t, one, "craft/lint/single-branch-par"); n != 1 {
		t.Errorf("one step: %d warnings, want 1", n)
	}
	for _, src := range []string{two, explicit} {
		if n := lintCodes(t, src, "craft/lint/single-branch-par"); n != 0 {
			t.Errorf("%d warnings, want 0:\n%s", n, src)
		}
	}
}
```

Run `go test ./internal/sema/ -run SingleBranchPar` → FAIL.

- [ ] **Step 5: Implement the lint** (`lint.go`; register in `LintWorkspace`)

```go
// ── single-branch-par ─────────────────────────────────────────────────────────
// Warning: a par without `and` whose body has exactly one step: nothing in it
// runs concurrently.

func lintSingleBranchPar(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		var walk func([]syntax.StepDecl)
		walk = func(steps []syntax.StepDecl) {
			for _, st := range steps {
				b, ok := st.Block()
				if !ok {
					continue
				}
				brs := b.Branches()
				if b.IsFanOut() && len(brs[0].Steps()) == 1 {
					if tok := brs[0].HeaderToken(); tok != nil {
						line, col := li.LineCol16(tok.Offset())
						start := colToLSP(col)
						diags = append(diags, model.Diagnostic{
							Code:      "craft/lint/single-branch-par",
							Message:   "this `par` has one step, so nothing in it runs concurrently; drop the `par` or add the steps that run alongside it",
							Severity:  model.SeverityWarning,
							SourceURI: uri,
							Range: model.Range{
								Start: model.Position{Line: lineToLSP(line), Character: start},
								End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(tok.Text())},
							},
						})
					}
				}
				for _, br := range brs {
					walk(br.Steps())
				}
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				walk(sc.Steps())
			}
		}
	}
	return diags
}
```

- [ ] **Step 6: Run, full suite, commit**

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(model): project par (fan-out split) and loop blocks; lint single-branch-par

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Interaction semantics for `par`/`loop` — **strong**

**Files:**
- Modify: `internal/interaction/interaction.go` (`BranchRef`, `deriveScenario`, `walker`, `walk`, `branchTrigger`, `walkBlock`)
- Test: `internal/interaction/interaction_test.go`

- [ ] **Step 1: Write the failing tests** (reuse `derive`, `structure`, `branchSummary`)

```go
func TestDerive_ParBranchesStartFromTheParStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        par "side effects"
            Billing returns the receipt
            Billing asks Bank to log it
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {}
2 response Billing->Ordering {par:side effects#0}
3 request Billing->Bank {par:side effects#1}
4 response Ordering->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, it := range its {
		if len(it.Branch) > 0 && !it.Branch[0].Split {
			t.Errorf("fan-out branch ref must carry Split: %+v", it.Branch[0])
		}
	}
}

func TestDerive_OpeningParTriggersOnceWhenEveryBranchStartsAtTheSameClient(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens the page
        par "fan-out"
            Ordering asks Billing for a
            Ordering asks Shipping for b
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {par:fan-out#0}
2 request Ordering->Shipping {par:fan-out#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_OpeningParTriggersABranchThatStartsElsewhere(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Reads" {
    when Customer opens the page
        par "fan-out"
            Ordering asks Billing for a
            Shipping asks Billing for b
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {par:fan-out#0}
0 trigger Customer->Shipping {par:fan-out#1}
2 request Shipping->Billing {par:fan-out#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_OpeningLoopIsTriggeredOnce(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Batch" {
    when Customer uploads a batch
        loop "for each order"
            Ordering asks Billing to charge
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {loop:for each order#0}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestDerive_LeadingNoteDoesNotHideTheOpeningBlock(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        note "the client retries on timeout"
        alt "with a voucher"
            Billing applies the voucher
        else "without"
            Ordering charges the card
}
`})
	want := `0 trigger Customer->Billing {alt:with a voucher#0}
1 internal Billing->Billing {alt:with a voucher#0}
0 trigger Customer->Ordering {alt:without#1}
2 internal Ordering->Ordering {alt:without#1}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/interaction/ -run 'Par|Loop|LeadingNote'` → FAIL (`Split` undefined).

- [ ] **Step 3: Implement**

1. `BranchRef` gains `Split bool // copied from model.Block.Split`. `walkBlock` sets it: `BranchRef{Block: b.ID, Kind: b.Kind, Label: br.Label, Index: i, Split: b.Split}`.
2. `walkBlock`: every `par` branch starts from the stack at the block, like an alt: `if b.Kind == lang.BlockAlt || b.Kind == lang.BlockPar || i == 0 {`. Update the doc comment (loop and opt continue from it).
3. `walker` gains:

```go
	// reached is the Key the unbranched trigger of an opening par already
	// reaches. A par branch whose first action has that subject needs no
	// trigger of its own (spec §4.4); "" for any other scenario.
	reached string
```

   and `branchTrigger` starts with:

```go
	if w.trigger == nil || a.Context == "" {
		return false
	}
	if w.reached != "" && endpoint(a.Context, a.From).Key == w.reached {
		return true // already reached by the trigger drawn before the par
	}
```

4. `deriveScenario`, external case: replace `if len(sc.Steps) > 0 && sc.Steps[0].Block != nil {` with a switch on `ob := sc.OpeningBlock()`:
   - `ob != nil && (ob.Kind == lang.BlockAlt || ob.Kind == lang.BlockOpt)`: the existing branch-trigger code, unchanged.
   - `ob != nil && ob.Kind == lang.BlockPar`: draw the ordinary trigger to `actions[0]` (the existing fall-through code) **and** keep a template for branches that start elsewhere: after `out = append(out, it)`, set `tmpl := it; trigger = &tmpl` and `reached = first.Key`.
   - otherwise (no block, or a loop): the existing ordinary trigger.
   Pass `reached` into the walker literal.
5. `walk`: the opening check must skip notes. Compute `first := firstStepIndex(steps)` (index of the first step that is not a note, -1 when none) and use `pending && i == first` instead of `pending && i == 0`; the `skipped` opt rule also keys on `st.Block.Kind == lang.BlockOpt` only, so par and loop never set it. Add `case st.Note != nil: continue` to the switch for clarity.

```go
// firstStepIndex returns the index of the first step that is not a note, or -1.
func firstStepIndex(steps []model.Step) int {
	for i, st := range steps {
		if st.Note == nil {
			return i
		}
	}
	return -1
}
```

6. Update the `Interaction.Branch` doc comment: an opening par gets an unbranched trigger plus in-branch triggers only for branches whose first subject differs; an opening loop gets one unbranched trigger.

- [ ] **Step 4: Run** `go test ./internal/interaction/` → PASS (old tests too). Full suite: diagram goldens unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/interaction
git commit -m "feat(interaction): par branches start from the par's stack; one trigger for an opening par or loop

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Render `par`/`loop` in sequence and flow — **strong**

**Files:**
- Modify: `internal/visualizer/view/sequence.go` (`branchPath`, `scenarioLayout.walk`, `BuildSequence` `opensBlock`), `internal/visualizer/view/flow.go`, `internal/visualizer/plantuml/plantuml.go` (`writeBlockOps`), `internal/visualizer/mermaid/views.go` (`writeBlockOps`)
- Test: `internal/visualizer/sequence_rendering_test.go`, `internal/visualizer/view/language_test.go` (or a new `blocks_test.go` in package `view`)

- [ ] **Step 1: Write the failing tests**

```go
const parRenderSrc = `actor user Seeker
domain ad {
    frontendplatform
    graphql
    atlas
    media
    adcore
    Dapi
}
use_case "Seeker reads an advert" {
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        par "field resolvers"
            graphql asks atlas for the SEO title  [postSeoadvertsbatch]
            graphql asks atlas for the targeting data  [postAdverttargetingbatch]
            graphql asks atlas where the advert came from  [postAdsourcebatch]
        loop "for each image, until all are signed"
            graphql asks media to sign the image URL  [signUrl]
        par "the advert"
            graphql asks adcore for the advert  [GET /ads]
        and "the statistics"
            graphql asks Dapi for the page statistics  [GET /stats]
}
`

func TestSequence_ParAndLoop(t *testing.T) {
	puml, mmd := sequenceSource(t, parRenderSrc)
	for _, want := range []string{"par field resolvers", "else", "loop for each image, until all are signed", "par the advert", "else the statistics"} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{"par field resolvers", "and", "loop for each image, until all are signed", "and the statistics"} {
		requireLine(t, "mermaid", mmd, want)
	}
	forbidLine(t, "mermaid", mmd, "else")
	for name, out := range map[string]string{"plantuml": puml, "mermaid": mmd} {
		for _, l := range strings.Split(out, "\n") {
			if strings.HasSuffix(l, " ") {
				t.Errorf("%s: trailing space in %q", name, l)
			}
		}
	}
}

func TestFlow_ParAndLoopEdgesNameTheirKind(t *testing.T) {
	doc := resolvedDoc(t, parRenderSrc)
	out, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[par: field resolvers] for the SEO title", "[loop: for each image, until all are signed] to sign the image URL", "[par: the statistics] for the page statistics"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("flow lacks %q:\n%s", want, out)
		}
	}
}
```

`BlockOps` unit test (package `view`): a split par's refs, as `branchPath` returns them, are `{Block:"b", Kind:par, Label:"x", Index:0}`, `{…, Label:"", Index:1}`, `{…, Label:"", Index:2}`. `BlockOps(nil, r0)` = open par "x"; `BlockOps(r0, r1)` = else ""; `BlockOps(r2, nil)` = close.

Also add `TestSequence_EmptyAltLabelHasNoTrailingSpace`: an `alt ""` / `else ""` block renders `alt` and `else` lines with no trailing space in both backends.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement**

1. One helper builds every view `BranchRef`, so `emitTriggers`' `slices.Equal` keeps matching the walk's paths:

```go
// viewRef is the view's copy of a branch ref. A split par branch after the
// first has a blank label, so writers print a bare separator (spec §4.5).
func viewRef(block string, kind lang.BlockKind, label string, index int, split bool) BranchRef {
	if split && index > 0 {
		label = ""
	}
	return BranchRef{Block: block, Kind: kind, Label: label, Index: index}
}
```

   `branchPath` uses `viewRef(r.Block, r.Kind, r.Label, r.Index, r.Split)`; `scenarioLayout.walk` uses `viewRef(st.Block.ID, st.Block.Kind, br.Label, bi, st.Block.Split)`.
2. `BuildSequence`: `ob := sc.OpeningBlock(); l.opensBlock = ob != nil && (ob.Kind == lang.BlockAlt || ob.Kind == lang.BlockOpt)`. In `walk`, the opt-skip rule uses the first non-note index (`firstStepIndex`, copy the helper into package `view`) instead of `i == 0`.
3. Writers. PlantUML:

```go
		case view.BlockOpen:
			writeHeader(b, string(op.BlockKind), op.Label)
		case view.BlockElse:
			writeHeader(b, "else", op.Label)
```

```go
// writeHeader writes a block line, with no trailing space for an empty label.
func writeHeader(b *strings.Builder, kw, label string) {
	if label == "" {
		fmt.Fprintf(b, "%s\n", kw)
		return
	}
	fmt.Fprintf(b, "%s %s\n", kw, label)
}
```

   Mermaid: the same with the 4-space indent, and on `BlockElse` the keyword is `and` when `op.BlockKind == lang.BlockPar`, else `else`.
4. Flow (`BuildFlow`'s `add`):

```go
		if len(branch) > 0 {
			br := branch[len(branch)-1]
			switch br.Kind {
			case lang.BlockPar, lang.BlockLoop:
				label = "[" + string(br.Kind) + ": " + br.Label + "] " + label
			default:
				label = "[" + br.Label + "] " + label
			}
		}
```

- [ ] **Step 4: Run** the tests → PASS; `go test ./internal/visualizer/ -run TestGolden` → no change. Full suite.

- [ ] **Step 5: Commit**

```bash
git add internal/visualizer
git commit -m "feat(sequence): native par/and and loop frames; flow names par/loop edges

Writers no longer print a trailing space after an empty block label.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: `receives` trigger: syntax, model, sema, `receives-on-actor` — **strong**

**Files:**
- Modify: `internal/syntax/parser.go` (`parseTrigger`), `internal/syntax/ast.go` (`TriggerDecl.Kind`, `ContextName`, `Connector`, `PhraseText`, `rawTextFrom`, new op views), `internal/analysis/project.go` (`projectTrigger`), `internal/analysis/resolve.go`, `internal/sema/lint.go` (`lintUnusedActors`, new `lintReceivesOnActor`)
- Create: `internal/syntax/receives_test.go`, `testdata/corpus/04_use_cases/receives_trigger.craft` + golden
- Test: `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/syntax/receives_test.go
package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestReceives_Trigger(t *testing.T) {
	for _, src := range []string{
		"use_case \"U\" {\n    when graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
		"use_case \"U\" {\n    when ad/graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n}",
	} {
		g, _, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Fatalf("unexpected diags: %v", diags)
		}
		tr := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
		if tr.Kind() != lang.TriggerReceive {
			t.Fatalf("Kind = %s", tr.Kind())
		}
		if tr.Connector() != "a" || tr.PhraseText() != "reindex sweep" || tr.OpText() != "reindexAdverts" {
			t.Errorf("connector=%q phrase=%q op=%q", tr.Connector(), tr.PhraseText(), tr.OpText())
		}
		if tr.ContextName() == "" || tr.ActorName() != "" {
			t.Errorf("context=%q actor=%q", tr.ContextName(), tr.ActorName())
		}
	}
}
```

Lint tests (append to `lint_journey_test.go`):

```go
func TestLint_ReceivesOnActor(t *testing.T) {
	onActor := "actor user Seller\ndomain d {\n  graphql\n}\nuse_case \"U\" {\n  when Seller receives a sweep  [x]\n    graphql does it\n}"
	onContext := "domain d {\n  graphql\n}\nuse_case \"U\" {\n  when graphql receives a sweep  [x]\n    graphql does it\n}"
	if n := lintCodes(t, onActor, "craft/lint/receives-on-actor"); n != 1 {
		t.Errorf("on actor: %d, want 1", n)
	}
	if n := lintCodes(t, onContext, "craft/lint/receives-on-actor"); n != 0 {
		t.Errorf("on context: %d, want 0", n)
	}
}

func TestLint_ReceivesTriggersDifferingByOperationAreDistinguishable(t *testing.T) {
	src := "domain d {\n  graphql\n}\nuse_case \"U\" {\n  when graphql receives a call  [getA]\n    graphql does a\n  when graphql receives a call  [getB]\n    graphql does b\n}"
	if n := lintCodes(t, src, "craft/lint/indistinguishable-scenarios"); n != 0 {
		t.Errorf("%d indistinguishable-scenarios warnings, want 0", n)
	}
}
```

Fixture `receives_trigger.craft`: the spec §6.1 example with `domain ad { graphql atlas }`.

- [ ] **Step 2: Run** → FAIL (`OpText` undefined on `TriggerDecl`).

- [ ] **Step 3: Implement the parser** (`parseTrigger`, after the `listens` branch)

```go
	if verb == "receives" {
		// receive: when <ctx> receives [connector] [phrase] [op]
		p.consumeAs(SyntaxKindKwReceives)
		if c := p.peek(); c.Type == lexer.TokenIdent && isConnectorWord(c.Value) && c.Line == verbTok.Line {
			p.consumeAs(SyntaxKindIdent)
		}
		start := p.opAnnotationStart(verbTok.Line)
		p.collectPhrase(verbTok.Line, start)
		if start >= 0 {
			diags = append(diags, p.parseOpAnnotation()...)
		}
		p.builder.FinishNode()
		return diags
	}
```

- [ ] **Step 4: Implement the views** (`ast.go`, `model_views.go`)

1. `Kind()`: `if len(elems) >= 2 && elems[1].Kind() == SyntaxKindKwReceives { return lang.TriggerReceive }` (before the external fallback).
2. `ContextName()`: return the subject for `TriggerDomainListen` **and** `TriggerReceive`.
3. `Connector()` and `PhraseText()`: accept `TriggerReceive` as well as `TriggerExternal`.
4. Op views shared with actions. Extract from `ActionDecl.OpText/OpVerb/OpPayload` three package helpers over a node and make both decls delegate:

```go
func opAnnotationOf(n SyntaxNode) *SyntaxNode { return n.ChildNode(SyntaxKindOpAnnotation) }
func opTextOf(n SyntaxNode) string    { /* body of today's ActionDecl.OpText, on opAnnotationOf(n) */ }
func opVerbOf(n SyntaxNode) string    { /* body of today's ActionDecl.OpVerb */ }
func opPayloadOf(n SyntaxNode) string { /* body of today's ActionDecl.OpPayload */ }

func (t TriggerDecl) OpAnnotation() *SyntaxNode { return opAnnotationOf(t.node) }
func (t TriggerDecl) OpText() string            { return opTextOf(t.node) }
func (t TriggerDecl) OpVerb() string            { return opVerbOf(t.node) }
func (t TriggerDecl) OpPayload() string         { return opPayloadOf(t.node) }
```

5. `TriggerDecl.rawTextFrom` stops before the annotation, like `ActionDecl.rawTextFrom` (copy its `endOffset` logic). External triggers have no annotation, so their text is unchanged.

- [ ] **Step 5: Projection and resolution**

`projectTrigger`: add a case

```go
	case lang.TriggerReceive:
		context = t.ContextName()
		connector = t.Connector()
		phrase = t.PhraseText()
		if text := t.OpText(); text != "" {
			op = &model.Operation{Verb: t.OpVerb(), Payload: t.OpPayload(), Text: text}
		}
```

(declare `var op *model.Operation` with the other locals), a description case

```go
	case kind == lang.TriggerReceive:
		description = strings.TrimSpace(fmt.Sprintf("when %s receives %s", context, joinConnector(connector, phrase)))
		if op != nil {
			description += " [" + op.Text + "]"
		}
```

and `Operation: op` in the struct literal. `resolveParticipants`: `case lang.TriggerReceive: sc.Trigger.Subject = resolve(sc.Trigger.Context)`.

The subject is already collected as a use-case reference by `AnalyzeFile` (it reads `ContextName()`), so it resolves and warns like any participant.

- [ ] **Step 6: Lints**

1. `lintUnusedActors`: add `case lang.TriggerReceive:` that marks `trigger.ContextName()` used (same as the listens case).
2. New, registered in `LintWorkspace` (it needs `ws`):

```go
// ── receives-on-actor ─────────────────────────────────────────────────────────
// Warning: a `receives` trigger whose subject is an actor.

func lintReceivesOnActor(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				tr := sc.Trigger()
				if tr.Kind() != lang.TriggerReceive || sc.When() == nil {
					continue
				}
				name := tr.ContextName()
				if p, _ := ResolveParticipant(ws, "", name); p.Kind != model.ParticipantActor {
					continue
				}
				line, _ := li.LineCol(sc.When().Offset())
				start := colToLSP(tr.ActorCol(li))
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/receives-on-actor",
					Message:   "an actor starts work with `when <Actor> <verb> …`; `receives` is for a boundary whose caller is not observed",
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(name)},
					},
				})
			}
		}
	}
	return diags
}
```

- [ ] **Step 7: Corpus golden, full suite, commit**

Run `-update`; `receives_trigger.craftjson` has `"type": "receive"`, `"context": "graphql"`, `"connector": "a"`, `"phrase": "reindex sweep"`, `"operation": {"text": "reindexAdverts"}`, `"description": "when graphql receives a reindex sweep [reindexAdverts]"`, and a `subject`. No other golden changes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): when <ctx> receives <phrase> [op], a callee-side trigger; lint receives-on-actor

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: `receives` interaction and found/lost rendering — **strong**

**Files:**
- Modify: `internal/interaction/interaction.go`, `internal/visualizer/view/{view,sequence,flow,c4}.go`, `internal/visualizer/plantuml/plantuml.go`, `internal/visualizer/mermaid/views.go`
- Test: `internal/interaction/interaction_test.go`, `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestDerive_ReceivesIsAFoundMessageAndItsReplyIsLost(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Sweep" {
    when Ordering receives a reindex sweep  [reindex]
        Ordering asks Billing for the totals  [getTotals]
        Billing returns the totals
        Ordering returns the ids
}
`})
	want := `0 trigger unobserved caller->Ordering [a reindex sweep]
1 request Ordering->Billing [for the totals]
2 response Billing->Ordering [returns the totals]
3 response Ordering->unobserved caller [returns the ids]`
	if got := summary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if its[0].From != interaction.Unobserved || its[0].Operation == nil || its[0].Operation.Text != "reindex" {
		t.Errorf("trigger = %+v", its[0])
	}
}

func TestDerive_ReceivesOpeningWithABlockStillTriggersOnceUpFront(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Sweep" {
    when Ordering receives a sweep  [reindex]
        alt "full"
            Ordering asks Billing for all
        else "partial"
            Ordering asks Billing for some
}
`})
	if its[0].Kind != interaction.Trigger || its[0].Branch != nil || its[0].To.Name != "Ordering" {
		t.Errorf("trigger = %+v", its[0])
	}
	n := 0
	for _, it := range its {
		if it.Kind == interaction.Trigger {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d triggers, want 1", n)
	}
}
```

Rendering (spec §6.5):

```go
const receivesSrc = `domain ad {
    graphql
    atlas
}
use_case "Adverts are swept for reindexing" {
    when graphql receives a reindex sweep  [reindexAdverts]
        graphql asks atlas for every advert that needs reindexing  [getAlladvertstoreindex]
        graphql returns the advert ids
}
`

func TestSequence_ReceivesDrawsFoundAndLostMessages(t *testing.T) {
	puml, mmd := sequenceSource(t, receivesSrc)
	for _, want := range []string{
		"[-> graphql : a reindex sweep [reindexAdverts]",
		"graphql -> atlas : for every advert that needs reindexing [getAlladvertstoreindex]",
		"graphql -->] : returns the advert ids",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	if strings.Contains(puml, "Unobserved") || strings.Contains(puml, "unobserved") {
		t.Errorf("plantuml must not declare a participant for the unobserved caller:\n%s", puml)
	}
	for _, want := range []string{
		"participant Unobserved as unobserved caller",
		"Unobserved->>graphql: a reindex sweep [reindexAdverts]",
		"graphql-->>Unobserved: returns the advert ids",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
}
```

Plus: the flow view of `receivesSrc` (PlantUML) contains `cloud "unobserved caller"`; the architecture view contains no `unobserved`; the C4 view (`ViewC4`, `C4: &C4Options{}`) contains no `unobserved`.

- [ ] **Step 2: Run** → FAIL (`interaction.Unobserved` undefined).

- [ ] **Step 3: Interaction**

```go
// Unobserved is the caller of a `receives` trigger: someone calls the boundary
// but the model does not know who. It is distinct from External (an event
// source outside the model) so views can tell the two apart.
var Unobserved = Endpoint{Key: "Unobserved", Name: "unobserved caller"}
```

In `deriveScenario`, a new case:

```go
	case lang.TriggerReceive:
		if t.Context == "" {
			break
		}
		callee := endpoint(t.Context, t.Subject)
		it := base
		it.Kind, it.From, it.To = Trigger, Unobserved, callee
		it.Label = strings.Join(strings.Fields(t.Connector+" "+t.Phrase), " ")
		it.Operation = t.Operation
		it.Site = triggerSite
		it.Site.Length = utf16Len(t.Context)
		// An unaddressed reply to the callee answers the unobserved caller.
		callStack = []frame{{caller: Unobserved, callee: callee}}
		out = append(out, it)
```

Nothing sets `trigger` for this kind, so no branch triggers are made whatever the scenario opens with.

- [ ] **Step 4: Views and writers**

1. `view.MessageKind` gains `MessageFound MessageKind = "found"` (from the unobserved caller; `From` is empty) and `MessageLost MessageKind = "lost"` (a reply to it; `To` is empty).
2. `nodeSet.add`: `case e == interaction.External || e == interaction.Unobserved: n.Kind = NodeExternal`.
3. `sequenceBuilder.add`: at the top of the `Trigger` case, `if it.From == interaction.Unobserved { msg.To, msg.Kind = ns.add(it.To), MessageFound; break }`; at the top of the `Response` case, `if it.To == interaction.Unobserved { msg.From, msg.Kind = ns.add(it.From), MessageLost; break }`. At the end, guard `b.appear(msg.To)`/`onStage[msg.To]` with `if msg.To != ""`.
4. `scenarioLayout.emit`/`opensBlock`: `l.opensBlock` is also false when `sc.Trigger.Type == lang.TriggerReceive`.
5. `anchor()`: `case MessageInbound, MessageFound: return []string{m.To}` and `case MessageNote, MessageLost: return []string{m.From}`.
6. PlantUML: `case view.MessageFound: fmt.Fprintf(&b, "[-> %s : %s\n", al.of(m.To), m.Label)` and `case view.MessageLost: fmt.Fprintf(&b, "%s -->] : %s\n", al.of(m.From), m.Label)`.
7. Mermaid: after the `External` declaration loop, declare `participant Unobserved as unobserved caller` once when any message is `MessageFound` or `MessageLost` (`unobserved := al.of("Unobserved")`). Draw `%s->>%s` (found) and `%s-->>%s` (lost) with it.
8. Flow: in the `Trigger` case, before the self check: `if it.From == interaction.Unobserved { add(ns.add(it.From), ns.add(it.To), StyleTrigger, it.Label); continue }`. A lost reply already reaches the `Response` case and draws a reply edge to the `unobserved caller` node.
9. Architecture: `inside` also excludes `interaction.Unobserved`.
10. C4: `findActors` skips `it.From == interaction.Unobserved`.

- [ ] **Step 5: Run** the tests → PASS; goldens unchanged; full suite.

- [ ] **Step 6: Commit**

```bash
git add internal/interaction internal/visualizer
git commit -m "feat(interaction): receives triggers come from an unobserved caller, drawn as found/lost messages

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Listens schedule tail — **cheap**

**Files:**
- Modify: `internal/syntax/parser.go` (`parseTrigger` listens branch), `internal/syntax/model_views.go` (`ListenSchedule`, `ListenScheduleToken`), `internal/analysis/project.go` (`projectTrigger`), `internal/sema/validate.go` (`validateSchedules`), `internal/interaction/interaction.go` (`Interaction.Schedule`, `scheduleLabel`), `internal/visualizer/view/sequence.go`
- Create: `testdata/corpus/04_use_cases/listens_schedule.craft` + golden, `testdata/broken/listens_invalid_schedule.craft` + `.diagnostics.json`
- Test: `internal/syntax/receives_test.go` (append), `internal/interaction/interaction_test.go`, `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestListens_ScheduleTail(t *testing.T) {
	src := "use_case \"U\" {\n    when smsdispatch listens store.sms_queue.inserted cron \"* 10 * * 1-5\"\n        smsdispatch claims the rows\n    when leadimporter listens \"LeadRequested\" every \"5m\"\n        leadimporter imports it\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	scs := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()
	k, e, ok := scs[0].Trigger().ListenSchedule()
	if !ok || k != lang.ScheduleCron || e != "* 10 * * 1-5" || scs[0].Trigger().EventRef() != "store.sms_queue.inserted" {
		t.Errorf("first: %v %q %v ref=%q", k, e, ok, scs[0].Trigger().EventRef())
	}
	k, e, ok = scs[1].Trigger().ListenSchedule()
	if !ok || k != lang.ScheduleEvery || e != "5m" || scs[1].Trigger().EventValue() != "LeadRequested" {
		t.Errorf("second: %v %q %v", k, e, ok)
	}
	if _, _, ok := scs[0].Trigger().Schedule(); ok {
		t.Error("Schedule() is the subject-slot form only; a listens tail must not make the trigger run as CRON")
	}
}
```

Interaction: a listens scenario with `every "5m"` yields a `Listen` whose `Schedule == "every 5m"`. Rendering: an event with no publisher, `when leadimporter listens sqs.LeadRequested every "5m"`, draws PlantUML `[-> leadimporter : sqs.LeadRequested, every 5m`.

Fixture `listens_schedule.craft`:

```craft
actor system CRON

domain leads {
    leadimporter
    smsdispatch
}

use_case "Lead requests are imported" {
    when leadimporter listens sqs.LeadRequested every "5m"
        leadimporter imports the lead
}

use_case "Queued reminders are sent" {
    when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"
        smsdispatch claims the unsent rows
}
```

- [ ] **Step 2: Run** → FAIL (`ListenSchedule` undefined).

- [ ] **Step 3: Implement**

1. Parser, in the listens branch after the event is consumed:

```go
		// Optional polling cadence: `listens <ref> cron "…"` / `every "…"`.
		if t := p.peek(); t.Line == verbTok.Line && t.Type == lexer.TokenIdent && (t.Value == "cron" || t.Value == "every") {
			if s := p.peekAt(1); s.Type == lexer.TokenString && s.Line == t.Line {
				kind := SyntaxKindKwCron
				if t.Value == "every" {
					kind = SyntaxKindKwEvery
				}
				p.consumeAs(kind)
				p.consumeAs(SyntaxKindString)
			}
		}
```

2. Views (`model_views.go`). `Schedule()`/`ScheduleToken()`/`ScheduleKeyword()` keep meaning the subject-slot form (CRON resolution, `unused-actor` and LSP rely on it).

```go
// ListenSchedule reports the trailing schedule of a listens trigger
// (`listens <ref> cron "…"`), its kind and unquoted expression.
func (t TriggerDecl) ListenSchedule() (kind lang.ScheduleKind, expr string, ok bool) {
	tok := t.ListenScheduleToken()
	if tok == nil {
		return "", "", false
	}
	elems := significantElements(t.node)
	kind = lang.ScheduleCron
	if elems[3].Kind() == SyntaxKindKwEvery {
		kind = lang.ScheduleEvery
	}
	return kind, stringAwareText(*tok), true
}

// ListenScheduleToken returns the quoted expression of a listens trigger's
// trailing schedule, or nil.
func (t TriggerDecl) ListenScheduleToken() *SyntaxToken {
	if t.Kind() != lang.TriggerDomainListen {
		return nil
	}
	elems := significantElements(t.node)
	if len(elems) < 5 || (elems[3].Kind() != SyntaxKindKwCron && elems[3].Kind() != SyntaxKindKwEvery) || elems[4].Kind() != SyntaxKindString {
		return nil
	}
	return slotFirstToken(elems[4])
}
```

3. `projectTrigger`: in the listens case, `if k, e, ok := t.ListenSchedule(); ok { schedKind, schedule = k, e }`. The listens description becomes:

```go
	case kind == lang.TriggerDomainListen:
		description = fmt.Sprintf("when %s listens %q", context, event)
		if schedKind != "" {
			description += fmt.Sprintf(` %s "%s"`, schedKind, schedule)
		}
```

4. `validateSchedules`: check both forms. Replace the head of the loop body with:

```go
			trigger := sc.Trigger()
			kind, expr, ok := trigger.Schedule()
			tok := trigger.ScheduleToken()
			if !ok {
				kind, expr, ok = trigger.ListenSchedule()
				tok = trigger.ListenScheduleToken()
			}
			if !ok || tok == nil {
				continue
			}
```

5. Interaction: `Interaction` gains `// Schedule is the polling cadence of a Listen from a scheduled listens trigger, as "cron <expr>" or "every <dur>".` `Schedule string`. Extract `scheduleLabel(t model.Trigger) string` from `triggerLabel` (`strings.TrimSpace(string(t.ScheduleKind) + " " + t.Schedule)`, "" when no kind), use it in `triggerLabel`, and set `it.Schedule = scheduleLabel(t)` in the listens loop.
6. Sequence (`add`, `Listen` case): when `it.Schedule != ""`, an on-stage delivery's label becomes `it.Label + ", " + it.Schedule`; for the inbound arrow add a `suffix string` field to `inboundArrow`, set to `", " + it.Schedule` when it is created, and make `label()` end with `+ a.suffix` in both return paths.

- [ ] **Step 4: Broken fixture**

`testdata/broken/listens_invalid_schedule.craft`:

```craft
domain messaging {
    smsdispatch
}

use_case "U" {
    when smsdispatch listens store.sms_queue.inserted cron "0 7 * *"
        smsdispatch claims the unsent rows
}
```

`.diagnostics.json`: `craft/sema/invalid-schedule`, error, `{"start":{"line":5,"character":59},"end":{"line":5,"character":68}}`.

- [ ] **Step 5: Goldens, full suite, commit**

Run `-update`. `listens_schedule.craftjson`: both triggers carry `schedule`/`scheduleKind`; descriptions `when leadimporter listens "sqs.LeadRequested" every "5m"` and `when smsdispatch listens "store.sms_queue.inserted" cron "* 10 * * 1-5"`; `actor` stays empty (no CRON). Harness C passes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(trigger): a listens trigger takes an optional cron/every polling schedule

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Store refs: model, resolution, `Via`, `store-without-change` — **cheap**

**Files:**
- Modify: `internal/lang/lang.go` (`ParseStoreRef`), `internal/analysis/project.go`, `internal/analysis/resolve.go`, `internal/interaction/interaction.go`, `internal/sema/lint.go`
- Create: `internal/lang/store_test.go`, `testdata/corpus/04_use_cases/store_handoff.craft` + golden
- Test: `internal/interaction/interaction_test.go`, `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/lang/store_test.go
package lang_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
)

func TestParseStoreRef(t *testing.T) {
	cases := []struct {
		ref, name string
		change    lang.StoreChange
		ok        bool
	}{
		{"store.sms_queue.inserted", "sms_queue", lang.StoreInserted, true},
		{"store.orders.updated", "orders", lang.StoreUpdated, true},
		{"store.redis.leads.pending", "redis.leads.pending", "", true},
		{"store.redis.leads.deleted", "redis.leads", lang.StoreDeleted, true},
		{"store.inserted", "", "", false},
		{"store.", "", "", false},
		{"sqs.LeadRequested", "", "", false},
		{"storefront.OrderPlaced", "", "", false},
	}
	for _, c := range cases {
		name, change, ok := lang.ParseStoreRef(c.ref)
		if name != c.name || change != c.change || ok != c.ok {
			t.Errorf("%q: got (%q, %q, %v)", c.ref, name, change, ok)
		}
	}
}
```

Interaction (uses `structure`, whose `ShopService` owns `orders_db`; add the `internal/model` import to `interaction_test.go` if it is missing):

```go
func TestDerive_StoreHandOffsCarryTheStoreEndpoint(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Write" {
    when Customer places an order
        Ordering notifies store.orders_db.inserted
        Ordering notifies store.carts.updated
}
use_case "Read" {
    when Billing listens store.orders_db.inserted cron "*/5 * * * *"
        Billing charges the order
}
`})
	var pubs, listens []interaction.Interaction
	for _, it := range its {
		switch it.Kind {
		case interaction.Publish:
			pubs = append(pubs, it)
		case interaction.Listen:
			listens = append(listens, it)
		}
	}
	if v := pubs[0].Via; v.Key != "orders_db" || v.Kind != model.ParticipantDataStore || v.Service != "ShopService" {
		t.Errorf("resolved store Via = %+v", v)
	}
	if v := pubs[1].Via; v.Key != "store.carts" || v.Name != "carts" || v.Kind != model.ParticipantDataStore {
		t.Errorf("unresolved store Via = %+v", v)
	}
	if len(listens) != 1 || listens[0].From.Name != "Ordering" || listens[0].Via.Key != "orders_db" || listens[0].Schedule != "cron */5 * * * *" {
		t.Errorf("listen = %+v", listens)
	}
}
```

Lint:

```go
func TestLint_StoreWithoutChange(t *testing.T) {
	src := "domain d {\n  a\n  b\n}\nuse_case \"U\" {\n  when O does\n    a notifies store.redis.leads.pending\n    a notifies store.leads.inserted\n  when b listens store.redis.leads.pending\n    b does it\n}"
	if n := lintCodes(t, src, "craft/lint/store-without-change"); n != 2 {
		t.Errorf("%d warnings, want 2 (the notifies and the listens of the change-less ref)", n)
	}
}
```

Fixture `store_handoff.craft` is the spec §7.5 example plus `services { messaging { contexts: smsdispatch, invoicing  data-stores: sms_queue } }` (one field per line) and `domain billing { invoicing smsdispatch }`, so `sms_queue` resolves.

- [ ] **Step 2: Run** → FAIL (`ParseStoreRef` undefined).

- [ ] **Step 3: Implement**

1. `lang.ParseStoreRef` (add `import "strings"`; the package imports nothing from this module, the standard library is fine):

```go
// ParseStoreRef splits a store ref, store.<name>[.<change>]. The last segment
// is the change when it is inserted, updated or deleted; every other segment
// is the name. ok is false when ref is not a store ref or names no store.
func ParseStoreRef(ref string) (name string, change StoreChange, ok bool) {
	rest, found := strings.CutPrefix(ref, StoreTransport+".")
	if !found || rest == "" {
		return "", "", false
	}
	segs := strings.Split(rest, ".")
	switch c := StoreChange(segs[len(segs)-1]); c {
	case StoreInserted, StoreUpdated, StoreDeleted:
		if len(segs) == 1 {
			return "", "", false
		}
		return strings.Join(segs[:len(segs)-1], "."), c, true
	}
	return rest, "", true
}
```

2. Projection. A store ref is bare: `ObjectRef()`/`EventRef()` are "" for a quoted string, which is exactly the rule.

```go
// storeRef returns the store hand-off a bare ref names, or nil.
func storeRef(ref string) *model.StoreRef {
	if name, change, ok := lang.ParseStoreRef(ref); ok {
		return &model.StoreRef{Name: name, Change: change}
	}
	return nil
}
```

   `projectAction`: `if kind == lang.ActionAsync { store = storeRef(a.ObjectRef()) }` and `Store: store` in the literal. `projectTrigger`: `if kind == lang.TriggerDomainListen { out.Store = storeRef(ref) }`.
3. `resolveParticipants`: resolve the name and keep it only when it is a data store:

```go
	resolveStore := func(s *model.StoreRef) {
		if s == nil {
			return
		}
		if p := resolve(s.Name); p != nil && p.Kind == model.ParticipantDataStore {
			s.Participant = p
		}
	}
```

   Call it for `sc.Trigger.Store` and, inside `EachAction`, for `a.Store`.
4. Interaction: `Interaction` gains `// Via is the store of a store hand-off (Publish and Listen); zero otherwise.` `Via Endpoint`.

```go
// storeEndpoint is the data-store endpoint of a store hand-off: the declared
// data store when the name resolves, else "store.<name>". Kind is always
// data_store, so views know to draw a store.
func storeEndpoint(s *model.StoreRef) Endpoint {
	if s == nil {
		return Endpoint{}
	}
	if p := s.Participant; p != nil {
		return Endpoint{Key: p.ID, Name: p.ID, Kind: model.ParticipantDataStore, Service: p.Service}
	}
	return Endpoint{Key: "store." + s.Name, Name: s.Name, Kind: model.ParticipantDataStore}
}
```

   Set `it.Via = storeEndpoint(a.Store)` on a `Publish` (in `walker.action`) and `it.Via = storeEndpoint(t.Store)` on each `Listen`. `nameEndpoints` leaves `Via` alone.
5. Lint (registered in `LintWorkspace`): walk every scenario; for a listens trigger use `tr.EventRef()`/`tr.EventCol(li)`, for each async action (`sc.Actions()`, `a.Kind() == lang.ActionAsync`) use `a.ObjectRef()`/`a.EventCol(li)`. When `ParseStoreRef(ref)` is ok with an empty change, warn `craft/lint/store-without-change` over the ref (`lspLen(ref)`): "store ref `%s` names no change; end it with `.inserted`, `.updated` or `.deleted`".

- [ ] **Step 4: Golden, full suite, commit**

Run `-update`. `store_handoff.craftjson`: the `notifies` action has `"store": {"name": "sms_queue", "change": "inserted", "participant": {"kind": "data_store", "id": "sms_queue", "service": "messaging"}}`; the listens trigger has the same `store` plus its schedule. `event`/`ref` keep the full text. No other golden changes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(model): store hand-offs (store.<name>.<change>) with a resolved data store and Interaction.Via

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Store rendering (sequence, flow, architecture, C4) — **strong**

**Files:**
- Modify: `internal/visualizer/view/{view,sequence,flow,c4}.go`, `internal/visualizer/plantuml/plantuml.go`, `internal/visualizer/mermaid/{views,mermaid}.go`
- Test: `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests**

```go
const storeSrc = `actors {
    system CRON
    system Siren
}
domain billing {
    invoicing
    smsdispatch
}
services {
    messaging {
        contexts: invoicing, smsdispatch
        data-stores: sms_queue
    }
}
use_case "Invoicing queues a payment reminder" {
    when cron "0 9 * * 1-5"
        invoicing finds the invoices due in three days
        invoicing notifies store.sms_queue.inserted
}
use_case "The queued reminder is sent" {
    when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"
        smsdispatch claims the unsent rows
        smsdispatch asks Siren to send the SMS  [sendSmsNotificationEvent]
}
`

func TestSequence_StoreHandOff(t *testing.T) {
	puml, mmd := sequenceSource(t, storeSrc)
	for _, want := range []string{
		`database "sms_queue" as sms_queue`,
		"invoicing -> sms_queue : inserted",
		"smsdispatch -> sms_queue : polls inserted (from invoicing), cron * 10 * * 1-5",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	if strings.Contains(puml, `notifies "store.`) {
		t.Errorf("the self-arrow is replaced:\n%s", puml)
	}
	// Participants in order of first appearance.
	order := []string{`boundary "CRON"`, `participant "invoicing"`, `database "sms_queue"`, `participant "smsdispatch"`, `boundary "Siren"`}
	last := -1
	for _, p := range order {
		i := strings.Index(puml, p)
		if i < last {
			t.Errorf("participant %s out of order:\n%s", p, puml)
		}
		last = i
	}
	for _, want := range []string{
		"participant sms_queue as sms_queue (store)",
		"invoicing->>sms_queue: inserted",
		"smsdispatch->>sms_queue: polls inserted (from invoicing), cron * 10 * * 1-5",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
}

func TestFlowArchitectureC4_StoreHandOff(t *testing.T) {
	doc := resolvedDoc(t, storeSrc)
	flow, _, _ := renderSource(doc, ViewFlow, PlantUML)
	for _, want := range []string{`database "sms_queue" as sms_queue`, "invoicing ->> sms_queue : 3. inserted", "sms_queue --> smsdispatch : 4. polls inserted"} {
		if !strings.Contains(string(flow), want) {
			t.Errorf("flow lacks %q:\n%s", want, flow)
		}
	}
	arch, _, _ := renderSource(doc, ViewArchitecture, PlantUML)
	for _, want := range []string{"invoicing --> sms_queue", "sms_queue --> smsdispatch"} {
		if !strings.Contains(string(arch), want) {
			t.Errorf("architecture lacks %q:\n%s", want, arch)
		}
	}
	if strings.Contains(string(arch), "events") {
		t.Errorf("a store hand-off is not routed through an event queue:\n%s", arch)
	}
}
```

(Edges: 1 the CRON trigger, 2 `finds the invoices…`, 3 the write, 4 the read, which a listens scenario derives before its actions.) C4: a unit test renders `storeSrc` with `C4Options{ShowDatabases: true}` and expects a relation from the messaging container to the `sms_queue` database labelled `writes inserted` and one labelled `polls`.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement**

1. `view.NodeKind` gains `NodeStore NodeKind = "store" // a data store reached by a store hand-off`.

```go
// addStore records a store hand-off endpoint as a store node.
func (ns *nodeSet) addStore(e interaction.Endpoint) string {
	id := ns.add(e)
	n := ns.byID[id]
	n.Kind = NodeStore
	ns.byID[id] = n
	return id
}

// storeChange is the change a store hand-off's event names ("" for none).
func storeChange(event string) string {
	_, ch, _ := lang.ParseStoreRef(event)
	return string(ch)
}
```

2. Sequence `add`:
   - `Publish`: `if !it.Via.IsZero() { msg.From, msg.To, msg.Kind = ns.add(it.From), ns.addStore(it.Via), MessageCall; msg.Label = cmp.Or(storeChange(it.Event), "writes"); break }`.
   - `Listen` with `Via`: always the reader's read, once per listening scenario, even when a writer is on stage. Reuse `b.inbound` (same key); `inboundArrow` gains `head string` (the event for an inbound arrow, `strings.TrimSpace("polls " + change)` for a store read) and `label()` returns `a.head + from + a.suffix`. On creation: `msg.From, msg.To, msg.Kind, msg.Label = ns.add(it.To), ns.addStore(it.Via), MessageCall, arrow.label()`. Later publishers only grow the label, as today. Set `head: it.Event` for the existing inbound arrows.
3. Writers: PlantUML sequence declares `database "<label>" as <alias>` for `NodeStore`; Mermaid sequence declares `participant <alias> as <label> (store)`. Flow/architecture: PlantUML `database "<label>" as <alias>` (also inside a service `rectangle`, indented like a frame); Mermaid `node()` draws `NodeStore` as `id[("label")]`.
4. Flow: `Flow` gains `Stores []Node` (`ns.sorted(NodeStore)`). `Publish` with `Via`: `add(ns.add(it.From), ns.addStore(it.Via), StyleAsync, cmp.Or(storeChange(it.Event), "writes"))`. `Listen` with `Via`: once per `(UseCaseIndex, Scenario, Via.Key)`, `add(ns.addStore(it.Via), ns.add(it.To), StyleTrigger, strings.TrimSpace("polls "+storeChange(it.Event)))`. PlantUML flow writes a `' Data stores` section; Mermaid adds `f.Stores` to the node groups.
5. Architecture: `Publish` with `Via` and `inside(it.From)`: `connect(addContext(it.From), addStore(it.Via))`; `Listen` with `Via` and `inside(it.To)`: `connect(addStore(it.Via), addContext(it.To))` (whoever wrote it). `addStore` records `services[id] = e.Service` when set. Grouping iterates `ns.sorted(NodeContext)` **and** `ns.sorted(NodeStore)`, so a resolved store sits in its service group and an unresolved one is ungrouped. Interactions with `Via` skip the queue paths.
6. C4 (`relateServices`): with `b.opts.ShowDatabases`, a `Publish` whose `Via.Kind` is `data_store` and whose `b.containerOf(it.Via)` is non-empty relates `b.containerOf(it.From)` → db with `strings.TrimSpace("writes " + storeChange(it.Event))`, technology `Database Query`; a `Listen` relates `b.containerOf(it.To)` → db with `polls`. Skip a relation when either container is "". `relateEvents` skips an interaction whose db relation was drawn (`!it.Via.IsZero() && b.containerOf(it.Via) != ""`).

- [ ] **Step 4: Run** the tests → PASS. `go test ./internal/visualizer/ -run TestGolden` → no change (no golden model has store refs). Full suite.

- [ ] **Step 5: Commit**

```bash
git add internal/visualizer
git commit -m "feat(diagrams): store hand-offs draw a database participant, writer and reader arrows

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 14: `surface` syntax and diagnostics — **strong**

**Files:**
- Modify: `internal/syntax/parser.go` (`parseFile`, `resyncToTopLevel`, new `atSurfaceDecl`, `parseSurfaceBlock`, `parseSurfaceItem`, `parseSurfaceOperation`), `internal/syntax/ast.go` (`File.Surfaces`, `SurfaceDecl`, `SurfaceOpDecl`)
- Create: `internal/syntax/surface_test.go`, `testdata/broken/{surface_missing_operation,surface_unclosed,surface_unknown_line}.craft` + `.diagnostics.json`

**Tree shape:**

```
SurfaceDecl
  KwSurface Ident|Ref LBrace
  SurfaceField: KwCallers Colon Ref (Comma Ref)*
  SurfaceField: KwSpec Colon String
  Note: KwNote String
  SurfaceOperation: KwQuery|KwCommand Ident(connector)? <phrase tokens> OpAnnotation
  RBrace
```

- [ ] **Step 1: Write the failing tests**

```go
// internal/syntax/surface_test.go
package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const surfaceSrc = `surface concierge {
    callers: Advisor, unknown
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair"
    finding "a read-only credential is refused only at persistence"
    query the seeker list, unpaged, newest first  [getUserList]
    query the seeker audit trail                  [getUserLogs]
    command delete a seeker's account             [deleteAccount]
}
`

func TestSurface_Parse(t *testing.T) {
	g, _, diags := syntax.Parse(surfaceSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	ss := syntax.AsFile(syntax.Root(g)).Surfaces()
	if len(ss) != 1 {
		t.Fatalf("surfaces = %d", len(ss))
	}
	s := ss[0]
	if s.Subject() != "concierge" || strings.Join(s.Callers(), ",") != "Advisor,unknown" || s.Spec() != "obidozoe/docs/openapi.yaml" {
		t.Errorf("subject=%q callers=%v spec=%q", s.Subject(), s.Callers(), s.Spec())
	}
	if ns := s.Notes(); len(ns) != 2 || ns[1].Kind() != lang.NoteFinding {
		t.Errorf("notes = %+v", ns)
	}
	ops := s.Operations()
	var got []string
	for _, op := range ops {
		got = append(got, string(op.Kind())+"|"+op.Connector()+"|"+op.PhraseText()+"|"+op.OpText())
	}
	want := "query||the seeker list, unpaged, newest first|getUserList;query||the seeker audit trail|getUserLogs;command||delete a seeker's account|deleteAccount"
	if strings.Join(got, ";") != want {
		t.Errorf("ops:\n%s\nwant:\n%s", strings.Join(got, ";"), want)
	}
}

func TestSurface_WordStaysAName(t *testing.T) {
	src := "domain d {\n    surface\n}\nuse_case \"U\" {\n  when O does\n    surface asks query to run\n}"
	if _, _, diags := syntax.Parse(src); len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
}
```

- [ ] **Step 2: Run** → FAIL (`Surfaces` undefined).

- [ ] **Step 3: Implement the parser**

```go
// atSurfaceDecl reports whether the next tokens open a surface: the contextual
// keyword `surface`, then a name (bare or `<domain>/<name>`) and `{`, all on
// one line. Anywhere else `surface` is an identifier.
func (p *Parser) atSurfaceDecl() bool {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || tok.Value != "surface" {
		return false
	}
	name := p.peekAt(1)
	if (name.Type != lexer.TokenIdent && !isKeywordUsedAsIdent(name.Type)) || name.Line != tok.Line {
		return false
	}
	next := p.peekAt(2)
	if next.Type == lexer.TokenLBrace {
		return next.Line == tok.Line
	}
	brace := p.peekAt(4)
	return next.Value == "/" && p.peekAt(3).Type == lexer.TokenIdent && brace.Type == lexer.TokenLBrace && brace.Line == tok.Line
}
```

1. `parseFile`: at the top of the loop body, before the `switch`, `if p.atSurfaceDecl() { diags = append(diags, p.parseSurfaceBlock()...); prevWasUseCase = false; continue }`.
2. `resyncToTopLevel`: stop at a surface too (`if p.atEOF() || isTopLevelKeyword(p.peek().Type) || p.atSurfaceDecl()`, in both places).
3. The block:

```go
// parseSurfaceBlock parses: surface <name> { surface_item* }
func (p *Parser) parseSurfaceBlock() []model.Diagnostic {
	p.builder.StartNode(SyntaxKindSurfaceDecl)
	p.attachTrivia()
	kwTok := p.consumeAs(SyntaxKindKwSurface)
	diags := p.parseNameSlot("a surface subject")
	p.consumeAs(SyntaxKindLBrace) // atSurfaceDecl checked it
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace && !isTopLevelKeyword(p.peek().Type) && !p.atSurfaceDecl() {
		diags = append(diags, p.parseSurfaceItem()...)
	}
	if p.peek().Type != lexer.TokenRBrace {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/unclosed-block",
			Message:  "unclosed surface block (missing `}`)",
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindRBrace)
	p.builder.FinishNode()
	return diags
}

// parseSurfaceItem parses one line of a surface body.
func (p *Parser) parseSurfaceItem() []model.Diagnostic {
	tok := p.peek()
	var diags []model.Diagnostic
	switch {
	case tok.Type == lexer.TokenIdent && (tok.Value == "callers" || tok.Value == "spec") && p.peekAt(1).Type == lexer.TokenColon:
		p.builder.StartNode(SyntaxKindSurfaceField)
		if tok.Value == "callers" {
			p.consumeAs(SyntaxKindKwCallers)
			p.consumeAs(SyntaxKindColon)
			p.parseRefList()
		} else {
			p.consumeAs(SyntaxKindKwSpec)
			p.consumeAs(SyntaxKindColon)
			if s := p.peek(); s.Type == lexer.TokenString && s.Line == tok.Line {
				p.consumeAs(SyntaxKindString)
			} else {
				diags = append(diags, p.diagUnexpected(s, "a quoted spec path, e.g. `spec: \"docs/openapi.yaml\"`"))
			}
		}
		p.builder.FinishNode()
	case p.atNoteLine():
		return p.parseNote()
	case tok.Type == lexer.TokenIdent && (tok.Value == "query" || tok.Value == "command"):
		return p.parseSurfaceOperation()
	default:
		return p.sweepRestOfLine(tok.Line, "`callers:`, `spec:`, `note`, `finding`, `query`, `command` or `}`")
	}
	return append(diags, p.sweepRestOfLine(tok.Line, "end of line")...)
}

// parseSurfaceOperation parses: query|command [connector] phrase [op]. The
// operation is the entry's identity, so a line without one is an error and is
// left out of the model.
func (p *Parser) parseSurfaceOperation() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindSurfaceOperation)
	kind := SyntaxKindKwQuery
	if p.peek().Value == "command" {
		kind = SyntaxKindKwCommand
	}
	kwTok := p.consumeAs(kind)
	if c := p.peek(); c.Type == lexer.TokenIdent && isConnectorWord(c.Value) && c.Line == kwTok.Line {
		p.consumeAs(SyntaxKindIdent)
	}
	start := p.opAnnotationStart(kwTok.Line)
	p.collectPhrase(kwTok.Line, start)
	if start >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-operation",
			Message:  fmt.Sprintf("a `%s` line needs its operation in brackets, e.g. `%s the seeker list  [getUserList]`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	p.builder.FinishNode()
	return diags
}
```

   `sweepRestOfLine` returns nil when nothing is left, so the default branch also consumes a lone unknown token. If an unknown first token could be a `}` or EOF, the outer loop has already stopped.

- [ ] **Step 4: Implement the typed views** (`ast.go`)

`File.Surfaces()` (children of kind `SyntaxKindSurfaceDecl`). `SurfaceDecl`: `Keyword()`, `SubjectToken()` (the first token of significant element 1, via `slotFirstToken`), `Subject()` (`refAwareText` of element 1), `field(kw SyntaxKind) *SyntaxNode` (the first `SyntaxKindSurfaceField` child holding `kw`), `Callers() []string` and `CallerRefs() []RefDecl` (the `SyntaxKindRef` children of the callers field, `RefText()`), `Spec() string` (unquoted string of the spec field), `Notes() []NoteDecl`, `Operations() []SurfaceOpDecl`, `Line(li)`, `EndLine(li)`, `Node()`. `SurfaceOpDecl`: `KeywordToken()`, `Kind() lang.SurfaceOpKind`, `Connector()` (token 1 when it is a connector word), `PhraseText()` (`rawSourceFrom` from after the connector, stopping before the annotation), `OpAnnotation()`/`OpText()`/`OpVerb()`/`OpPayload()` (the Task 9 helpers), `Line(li)`.

- [ ] **Step 5: Broken fixtures**

All three start with `domain obido {` / `    concierge` / `}` / `` (lines 0–3).

| File | Lines 4… | Diagnostic |
|---|---|---|
| `surface_missing_operation.craft` | `surface concierge {` / `    query the seeker list` / `}` | `craft/syntax/missing-operation`, error, line 5, chars 4–9 |
| `surface_unclosed.craft` | `surface concierge {` / `    query the seeker list  [getUserList]` (EOF) | `craft/syntax/unclosed-block`, error, line 4, chars 0–7 |
| `surface_unknown_line.craft` | `surface concierge {` / `    lists the seekers  [getUserList]` / `    query the seeker audit trail  [getUserLogs]` / `}` | `craft/syntax/unexpected-token`, error, line 5, chars 4–9 |

Run Harness C; fix only columns.

- [ ] **Step 6: Full suite and commit**

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): top-level surface blocks with callers, spec, notes and query/command lines

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 15: Surface model, resolution and sema checks — **strong**

**Files:**
- Modify: `internal/analysis/project.go`, `internal/analysis/merge.go`, `internal/analysis/resolve.go`, `internal/sema/sema.go` (`Symbols`, `AnalyzeFile`, `AnalyzeWorkspace`), `internal/sema/validate.go`
- Create: `testdata/corpus/09_surfaces/{surface_basic,surface_notes_callers_unknown}.craft` + goldens, `testdata/broken/{surface_unresolved_subject,surface_domain_subject,surface_duplicate_operation}.craft` + `.diagnostics.json`
- Test: `internal/sema/sema_test.go` (cross-file duplicate), `internal/analysis/analysis_test.go` (merge keeps the first)

- [ ] **Step 1: Fixtures (red)**

`09_surfaces/surface_basic.craft`:

```craft
actor user Advisor

domain obido {
    concierge
}

surface concierge {
    callers: Advisor
    spec: "obidozoe/docs/openapi.yaml"
    query the seeker list, unpaged, newest first, filtered by a typed status  [getUserList]
    query the seeker audit trail                                             [getUserLogs]
    command delete a seeker's account                                        [deleteAccount]
}
```

`surface_notes_callers_unknown.craft`: a `domain obido { obidoanalytics }` and `surface obidoanalytics { callers: unknown … }` with one `note`, one `finding`, a `query` with a connector (`query for the daily totals  [GET /totals]`) and a `command`.

Broken fixtures (lines 0–3 are `domain obido {` / `    concierge` / `}` / ``):

| File | Lines 4… | Diagnostic |
|---|---|---|
| `surface_unresolved_subject.craft` | `surface conciergex {` / `    query the seeker list  [getUserList]` / `}` | `craft/sema/unresolved-reference`, **error**, line 4, chars 8–18 |
| `surface_domain_subject.craft` | `surface obido {` / `    query the seeker list  [getUserList]` / `}` | `craft/sema/invalid-surface-subject`, error, line 4, chars 8–13 |
| `surface_duplicate_operation.craft` | `surface concierge {` / `    query the seeker list  [getUserList]` / `    query the seeker list again  [getUserList]` / `}` | `craft/sema/duplicate-operation`, error, line 6, chars 33–46 |

Run Harness A `-update` (goldens have no `surfaces` yet — delete them) and Harness C (fails).

- [ ] **Step 2: Projection and merge**

`project()` after the exposures loop:

```go
	for _, s := range file.Surfaces() {
		if s.SubjectToken() == nil {
			continue
		}
		line, _ := li.LineCol(s.Keyword().Offset())
		ms := model.Surface{
			Subject: s.Subject(), Callers: s.Callers(), Spec: s.Spec(),
			Operations: []model.SurfaceOperation{}, SourceURI: sourceURI, Line: line,
		}
		for _, n := range s.Notes() {
			ms.Notes = append(ms.Notes, projectNote(n, "", li))
		}
		for _, op := range s.Operations() {
			text := op.OpText()
			if text == "" {
				continue // craft/syntax/missing-operation: not an entry
			}
			opLine, _ := li.LineCol(op.KeywordToken().Offset())
			ms.Operations = append(ms.Operations, model.SurfaceOperation{
				Kind: op.Kind(), Connector: op.Connector(), Phrase: op.PhraseText(),
				Operation: model.Operation{Verb: op.OpVerb(), Payload: op.OpPayload(), Text: text},
				Line:      opLine,
			})
		}
		doc.Surfaces = append(doc.Surfaces, ms)
	}
```

`mergeDocs`: `merged.Surfaces = append(merged.Surfaces, d.Surfaces...)`. `resolveParticipants`: resolve each surface's `Participant = resolve(s.Subject)`, then keep the first surface per key (`Participant.ID`, or `"?" + Subject` when unresolved), so the first in sorted file order wins (ruling 6).

- [ ] **Step 3: Sema**

1. `Symbols` gains `Surfaces []SurfaceSymbol`:

```go
// SurfaceSymbol is a surface declaration, resolved in AnalyzeWorkspace.
type SurfaceSymbol struct {
	Subject      string
	Line, Column int          // 1-based; Column in UTF-16 units, of the subject
	Callers      []UseCaseRef // `unknown` excluded
	URI          string
}
```

   `AnalyzeFile` collects them (callers from `CallerRefs()` with their line/col) and appends `validateSurfaces(uri, file, lineIdx, hasLI)` (shaped like `validateSchedules`): per surface, an op text seen before (whitespace collapsed with `strings.Join(strings.Fields(text), " ")`) is a `craft/sema/duplicate-operation` error over its annotation (first `[` to last `]` of `op.OpAnnotation().AllTokens()`): "operation `[%s]` is already listed in this surface".
2. `AnalyzeWorkspace` appends `resolveSurfaces(perFile, ws)`, which walks URIs in sorted order:
   - `p, ambiguous := ResolveParticipant(ws, "", s.Subject)`; ambiguous → skip (ambiguous-bc covers it).
   - `p.Kind == ""` → `craft/sema/unresolved-reference`, **error**: "surface subject %q is not declared as a bounded context, service or actor".
   - `p.Kind` is domain or data store → `craft/sema/invalid-surface-subject`, error: "a surface belongs to a bounded context, a service or an actor (an external system); %q is a %s".
   - a second surface with the same `p.ID` → `craft/sema/duplicate-name`, error when in the same file as the first, warning otherwise: "surface %q is already declared at %s:%d; the first one is used".
   - each caller that does not resolve → `craft/sema/unresolved-reference`, error: "surface caller %q is not declared; write `unknown` when no caller is observed".
   Unlike use-case references this runs even when the workspace declares nothing.
3. Unit tests: two files each declaring `surface concierge` → one `duplicate-name` **warning** on the second file; `analysis.Workspace` of those two files keeps the first surface only.

- [ ] **Step 4: Run and review**

Harness C passes. Run Harness A `-update`; `surface_basic.craftjson` has `"surfaces": [{"subject": "concierge", "participant": {"kind": "bounded_context", "id": "obido/concierge"}, "callers": ["Advisor"], "spec": …, "operations": [3 entries with kind, phrase, operation.text, line]}]`. `surface_notes_callers_unknown.craftjson` keeps `"callers": ["unknown"]`, two notes with no `id`, and `connector: "for"`. Existing goldens unchanged.

- [ ] **Step 5: Full suite and commit**

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(sema): surfaces in the model; unresolved, invalid-subject, duplicate-name and duplicate-operation checks

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 16: Surface coverage, Markdown, `generate --type surface`, `operation-not-in-surface` — **strong**

**Files:**
- Create: `internal/visualizer/view/surface.go`, `internal/visualizer/markdown/surface.go`, `internal/visualizer/surface_test.go`
- Modify: `internal/visualizer/visualizer.go` (`ViewSurface`, `validate`, `Render`), `cmd/craft/generate.go` (`--type`, `expandType`), `internal/sema/lint.go`
- Test: `cmd/craft/generate_test.go`, `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/visualizer/surface_test.go
package visualizer

import "testing"

const surfaceModel = `actors {
    user Advisor
    user Seeker
}
domain obido {
    advisorapp
    concierge
}
surface concierge {
    callers: Advisor
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair plus a bidder identity header"
    finding "a read-only credential is refused only at persistence, after the write ran"
    query the seeker list, unpaged, newest first, filtered by a typed status  [getUserList]
    query the seeker audit trail                                             [getUserLogs]
    command delete a seeker's account                                        [deleteAccount]
}
use_case "Advisor works a task" {
    when Advisor opens the task list
        advisorapp asks concierge for the seekers  [getUserList]
}
use_case "Seeker removes their concierge account" {
    when concierge receives an account deletion  [deleteAccount]
        concierge forgets the seeker
}
`

const surfaceMarkdown = "# Surfaces\n\n## concierge\n\nCallers: Advisor\nSpec: `obidozoe/docs/openapi.yaml`\n\n" +
	"> every route takes the shared HTTP Basic pair plus a bidder identity header\n\n" +
	"> **Finding:** a read-only credential is refused only at persistence, after the write ran\n\n" +
	"| Operation | Kind | What it offers | Used by |\n|---|---|---|---|\n" +
	"| `getUserList` | query | the seeker list, unpaged, newest first, filtered by a typed status | Advisor works a task |\n" +
	"| `getUserLogs` | query | the seeker audit trail | — |\n" +
	"| `deleteAccount` | command | delete a seeker's account | Seeker removes their concierge account (unobserved caller) |\n"

func TestSurface_Markdown(t *testing.T) {
	doc := resolvedDoc(t, surfaceModel)
	out, err := New(nil).Render(doc, Request{View: ViewSurface, Backend: PlantUML, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out.Data); got != surfaceMarkdown {
		t.Errorf("got:\n%s\nwant:\n%s", got, surfaceMarkdown)
	}
	if out.Ext != "md" {
		t.Errorf("ext = %q", out.Ext)
	}
}

func TestSurface_RejectsOtherFormatsAndFilters(t *testing.T) {
	doc := resolvedDoc(t, surfaceModel)
	for _, req := range []Request{
		{View: ViewSurface, Backend: PlantUML, Format: FormatSource},
		{View: ViewSurface, Backend: Mermaid, Format: FormatMarkdown, UseCases: []string{"Advisor works a task"}},
	} {
		if _, err := New(nil).Render(doc, req); err == nil {
			t.Errorf("expected ErrUnsupported for %+v", req)
		}
	}
}
```

Also a test that a `|` in a phrase is written `\|`.

CLI (`cmd/craft/generate_test.go`, follow the existing `t.Run` setup that writes a source into `t.TempDir()`):
- `--type surface --output dir` writes `<base>-surface.md` containing ``| `getUserList` | query |``.
- `--type surface --stdout` prints the same Markdown.
- a second `--type surface` run into the same dir fails with `refusing to overwrite`, and succeeds with `--force`.
- `--type all` on a model **without** a surface writes no `-surface.md`; on `surfaceModel` it writes one.

Lint:

```go
func TestLint_OperationNotInSurface(t *testing.T) {
	src := "domain d {\n  app\n  concierge\n}\nsurface concierge {\n  query the list  [getUserList]\n}\nuse_case \"U\" {\n  when O does\n    app asks concierge for the list  [getUserList]\n    app asks concierge to book  [addMortgagesMeeting]\n}"
	if n := lintCodes(t, src, "craft/lint/operation-not-in-surface"); n != 1 {
		t.Errorf("%d warnings, want 1 (addMortgagesMeeting)", n)
	}
}
```

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: View and writer**

```go
// internal/visualizer/view/surface.go
package view

// Surfaces is the surface tables of a model, ordered by subject.
type Surfaces struct{ Tables []SurfaceTable }

// SurfaceTable is one surface: its header facts and one row per operation, in
// source order.
type SurfaceTable struct {
	Subject string
	Callers []string
	Spec    string
	Notes   []model.Note
	Rows    []SurfaceRow
}

// SurfaceRow is one operation and the use cases that use it.
type SurfaceRow struct {
	Operation string
	Kind      model.SurfaceOpKind
	Offers    string   // connector + phrase
	UsedBy    []string // use-case names, sorted, deduplicated
}

// BuildSurfaces computes coverage (spec §8.4): an entry is used by every use
// case with a Request to the surface's boundary whose operation text equals the
// entry's (whitespace collapsed); a receives trigger on the boundary with that
// operation counts as "<use case> (unobserved caller)".
func BuildSurfaces(doc *model.CraftDoc, its []interaction.Interaction) Surfaces
```

   Key the usage map by `it.To.Key` (a resolved boundary's `Participant.ID`, else the written name) and the collapsed `it.Operation.Text`; a surface looks itself up with `Participant.ID` or, unresolved, `Subject`. Sort tables with `slices.SortFunc` by `Subject`.

`internal/visualizer/markdown/surface.go`: `func Surfaces(v view.Surfaces) string` writes exactly the `surfaceMarkdown` shape: `# Surfaces`, then per table `## <subject>`, a `Callers: a, b` line when there are callers, a ``Spec: `…` `` line when set, a blank line, one `> text` (note) or `> **Finding:** text` paragraph per note, the table header, and rows ``| `op` | kind | offers | used by |`` with `—` when unused. Escape `|` as `\|` in every cell.

`visualizer.go`: `ViewSurface View = "surface" // operation table per surface (Markdown only)`. In `validate`, handle it first: it needs `FormatMarkdown` (any backend), and rejects `UseCases` and `C4`. In `Render`, after `validate`: `if req.View == ViewSurface { md := markdown.Surfaces(view.BuildSurfaces(doc, interaction.Derive(doc))); return Output{Data: []byte(md), ContentType: "text/markdown; charset=utf-8", Ext: "md"}, nil }`.

- [ ] **Step 4: CLI**

`generateDiagrams`: `case "surface": req.View, req.Format = visualizer.ViewSurface, visualizer.FormatMarkdown` (whatever `--format` says; `--use-case` and `--split` do not apply, and the split branch only fires for flow/sequence). `expandType(t string, hasSurfaces bool)`: `all` → `c4, domain, sequence` plus `surface` when `hasSurfaces`; call it with `len(model.Surfaces) > 0`. Help text: `"diagram type: c4|domain|sequence|surface|all"`. `writeOutput` already refuses an existing `.md` without `--force`.

- [ ] **Step 5: Lint** (`lintOperationNotInSurface`, registered in `LintWorkspace`)

Build `surfaceOps map[string]map[string]bool` (resolved subject ID → collapsed op texts) from every file's `Surfaces()` in sorted order, first surface per ID only. Then for every sync action (`a.Kind() == lang.ActionSync`) with a non-empty `OpText()`: resolve `a.TargetName()` (`ResolveParticipant(ws, "", …)`); if the target has a surface that lacks the op, warn `craft/lint/operation-not-in-surface` over the annotation (first to last token of `a.OpAnnotation()`): "`[%s]` is not an operation of the `%s` surface; add it to the surface or fix the operation name".

- [ ] **Step 6: Run, full suite, commit**

```bash
go vet ./... && go test ./...
git add internal cmd
git commit -m "feat(generate): --type surface writes a Markdown operation table with computed coverage; lint operation-not-in-surface

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 17: `repeated-request` lint — **cheap**

**Files:** Modify `internal/sema/lint.go` (`lintRepeatedRequests`, registered in `LintWorkspace`); Test `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Failing test**

```go
func TestLint_RepeatedRequest(t *testing.T) {
	src := `domain web {
  frontendplatform
  mobile
  graphql
}
use_case "Autocomplete" {
  when Seeker types a query
    frontendplatform asks graphql for suggestions  [autocomplete]
    graphql returns the suggestions
    mobile asks graphql for suggestions  [autocomplete]
  when Seeker types again
    frontendplatform asks graphql for suggestions  [autocomplete]
    frontendplatform asks graphql for more  [autocomplete]
    mobile asks graphql for suggestions  [suggest]
    alt "on the web"
      frontendplatform asks graphql for x  [search]
    else "in the app"
      mobile asks graphql for x  [search]
}`
	if n := lintCodes(t, src, "craft/lint/repeated-request"); n != 1 {
		t.Errorf("%d warnings, want 1 (only the mobile line of the first scenario)", n)
	}
}
```

(Same subject twice, a different operation, and alternatives in separate branch bodies are all quiet.)

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**

```go
// ── repeated-request ──────────────────────────────────────────────────────────
// Warning: two asks in one step list (a scenario body or a branch body, at any
// distance) from different subjects to the same resolved target with the same
// operation. Usually two clients making one request: write it once.

func lintRepeatedRequests(perFileTrees map[string]syntax.SyntaxNode, ws WorkspaceSymbols, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	id := func(name string) string {
		if p, _ := ResolveParticipant(ws, "", name); p.Kind != "" {
			return p.ID
		}
		return name
	}
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		var visit func([]syntax.StepDecl)
		visit = func(steps []syntax.StepDecl) {
			type first struct{ subject, subjectID string }
			seen := map[string]first{}
			for _, st := range steps {
				if b, ok := st.Block(); ok {
					for _, br := range b.Branches() {
						visit(br.Steps())
					}
					continue
				}
				a, ok := st.Action()
				op := strings.Join(strings.Fields(a.OpText()), " ")
				if !ok || a.Kind() != lang.ActionSync || op == "" || a.TargetName() == "" || a.SubjectName() == "" {
					continue
				}
				key := id(a.TargetName()) + "\x00" + op
				subj := id(a.SubjectName())
				f, dup := seen[key]
				if !dup {
					seen[key] = first{a.SubjectName(), subj}
					continue
				}
				if f.subjectID == subj {
					continue
				}
				line, start := a.Line(li), colToLSP(a.SubjectCol(li))
				diags = append(diags, model.Diagnostic{
					Code: "craft/lint/repeated-request",
					Message: fmt.Sprintf("`%s` and `%s` make the same request `[%s]` to `%s`; if they are alternatives, write it once with a `channels` tag, or open the scenario with `alt \"on the web\"` / `else \"in the app\"`",
						a.SubjectName(), f.subject, op, a.TargetName()),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + lspLen(a.SubjectName())},
					},
				})
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				visit(sc.Steps())
			}
		}
	}
	return diags
}
```

- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lint): repeated-request points at two clients making the same request`.

---

### Task 18: Formatter and fuzz — **strong**

**Files:** Modify `internal/lsp/formatter.go` (`refusingSyntaxCodes`, `firstBodyToken` already done in Task 2); Test `internal/lsp/formatter_test.go`, `internal/lsp/formatter_corpus_test.go` (if it lists files), `internal/lsp/formatter_fuzz_test.go`; Create `internal/syntax/usecase_body_fuzz_test.go`

- [ ] **Step 1: Failing tests** (append to `formatter_test.go`)

```go
func TestFormat_JourneySemanticsCanonical(t *testing.T) {
	in := "use_case \"U\" {\n  finding \"nothing retries a failed callback\"\n  when Seeker opens an advert\n      x asks y for it\n      par \"field resolvers\"\n        y asks z for a\n        y asks z for b\n      loop \"for each image\"\n         y asks m to sign\n      par \"the advert\"\n          y asks a for the advert\n      and \"the statistics\"\n          y asks d for stats\n          note \"cached for a minute\"\n}\n"
	want := "use_case \"U\" {\n    finding \"nothing retries a failed callback\"\n\n    when Seeker opens an advert\n        x asks y for it\n        par \"field resolvers\"\n            y asks z for a\n            y asks z for b\n        loop \"for each image\"\n            y asks m to sign\n        par \"the advert\"\n            y asks a for the advert\n        and \"the statistics\"\n            y asks d for stats\n            note \"cached for a minute\"\n}\n"
	got := FormatDocument(in)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
	if stepOutline(t, in) != stepOutline(t, got) {
		t.Error("block membership changed")
	}
}

func TestFormat_ReceivesBracketJoinsTheActionRun(t *testing.T) {
	in := "use_case \"U\" {\n    when graphql receives a reindex sweep [reindexAdverts]\n        graphql asks atlas for every advert [getAll]\n}\n"
	lines := strings.Split(FormatDocument(in), "\n")
	if a, b := strings.Index(lines[1], "["), strings.Index(lines[2], "["); a != b || a < 0 {
		t.Errorf("brackets not aligned:\n%s\n%s", lines[1], lines[2])
	}
}

func TestFormat_ListensTailSingleSpaces(t *testing.T) {
	in := "use_case \"U\" {\n    when smsdispatch listens store.sms_queue.inserted   cron   \"* 10 * * 1-5\"\n        smsdispatch claims the rows\n}\n"
	if got := FormatDocument(in); !strings.Contains(got, "listens store.sms_queue.inserted cron \"* 10 * * 1-5\"\n") {
		t.Errorf("got:\n%s", got)
	}
}

func TestFormat_Surface(t *testing.T) {
	in := "surface concierge {\n  callers: Advisor\n  spec:  \"obido/openapi.yaml\"\n  note \"shared basic auth\"\n  query the seeker list [getUserList]\n  command delete an account   [deleteAccount]\n}\n"
	want := "surface concierge {\n    callers: Advisor\n    spec: \"obido/openapi.yaml\"\n    note \"shared basic auth\"\n    query the seeker list      [getUserList]\n    command delete an account  [deleteAccount]\n}\n"
	if got := FormatDocument(in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormat_RefusesOrphanAnd(t *testing.T) {
	in := "use_case \"U\" {\n  when O does\n    x does it\n    and \"b\"\n      x does more\n}\n"
	if _, d := FormatDocumentChecked(in); d == nil {
		t.Error("expected the formatter to refuse a file with an orphan and")
	}
}
```

Also: every new corpus file from Tasks 3, 6, 9, 11, 12 and 15 formats idempotently and keeps its `stepOutline` (extend the loop in `formatter_corpus_test.go`, or add one over `testdata/corpus/04_use_cases/*.craft` and `testdata/corpus/09_surfaces/*.craft`).

- [ ] **Step 2: Run** → FAIL (at least the refusal; the rest may already pass because indentation comes from the tree — if so, keep them as pins).
- [ ] **Step 3: Implement**
  - `refusingSyntaxCodes` gains `"craft/syntax/orphan-and": true` and `"craft/syntax/missing-operation": true`.
  - If `TestFormat_JourneySemanticsCanonical` shows a note or a `par`/`loop` line at the wrong depth, the fix is in `branchDepth`/`keepInBody` (they must treat `SyntaxKindNote` like `SyntaxKindAction`); do not special-case keywords in `separatorFor`.
  - If the surface body is not indented, check that `writeTokens` sees `SyntaxKindSurfaceDecl` through the default case of the declaration loop (it should: only `ArchDecl` is copied verbatim).
- [ ] **Step 4: Fuzz**
  - `FuzzFormatBranches`: add seeds `par_fanout`, `par_and`, `loop`, `nested_par_loop_alt`, `notes_findings` (same loop).
  - New `internal/syntax/usecase_body_fuzz_test.go` (package `syntax`, so `Parse` panics on a lossy tree via `checkTreeText`):

```go
package syntax

import "testing"

// FuzzParseUseCaseBody wraps random bodies in a use case (or parses them as a
// whole file) and checks the parser never panics and the tree reproduces its
// source (checkTreeText panics under test).
func FuzzParseUseCaseBody(f *testing.F) {
	for _, body := range []string{
		"    when Seeker opens an advert\n        frontendplatform asks graphql for the advert  [ad]\n        par \"field resolvers\"\n            graphql asks atlas for a  [x]\n            graphql asks atlas for b  [y]\n        loop \"for each image\"\n            graphql asks media to sign  [signUrl]\n",
		"    finding \"nothing retries\"\n    when Seller pays an invoice\n        a asks b to pay  [payInvoice]\n        note \"session forwarded\"\n",
		"    when graphql receives a reindex sweep  [reindexAdverts]\n        graphql returns the ids\n",
		"    when smsdispatch listens store.sms_queue.inserted cron \"* 10 * * 1-5\"\n        smsdispatch claims the rows\n",
	} {
		f.Add(body, true)
	}
	f.Add("surface concierge {\n    callers: Advisor, unknown\n    query the list  [getUserList]\n    command delete it  [deleteAccount]\n}\n", false)
	f.Fuzz(func(t *testing.T, body string, wrap bool) {
		src := body
		if wrap {
			src = "use_case \"F\" {\n" + body + "}\n"
		}
		Parse(src)
	})
}
```

- [ ] **Step 5: Run** `go test ./internal/lsp/ ./internal/syntax/` → PASS; `go test ./internal/syntax/ -run XXX -fuzz FuzzParseUseCaseBody -fuzztime 60s` and `go test ./internal/lsp/ -run XXX -fuzz FuzzFormatBranches -fuzztime 60s` find nothing. Full suite.
- [ ] **Step 6: Commit** `feat(fmt): par/and/loop, notes, receives, listens tail and surfaces format canonically; fuzz the use-case body parser`.

---

### Task 19: LSP for the use-case constructs — **strong**

**Files:** Modify `internal/lsp/blocks.go` (`blockSymbols`, `scenarioSymbols`, `blockHover`, `blockKeywordCompletions`), `internal/lsp/server.go` (`DocumentSymbol`, `Hover`, `syntaxKindToCraftToken`), `internal/lsp/completion.go`; Test `internal/lsp/blocks_test.go`, `internal/lsp/completion_test.go`, `internal/lsp/server_test.go`

- [ ] **Step 1: Failing tests** (follow the helpers in `blocks_test.go`: `outlineOf`, and the hover/completion helpers in `server_test.go`)
  - Outline of `parRenderSrc` (Task 8): the scenario's children are `par: field resolvers`, `loop: for each image, until all are signed`, `par: the advert | the statistics`.
  - Outline of `notesSrc` (Task 4): the use case's first child is `finding: nothing retries a failed callback; a lost callback leaves the i` (60 characters), kind `Event`; the scenario's children include `finding: no idempotency key: a client retry starts a second payment` (kind `Event`); no symbol starts with `note:`.
  - Folding ranges include one per `par` and `loop` block (comes from `SyntaxKindBlock`; this is a pin).
  - Hover on the `par "field resolvers"` line: `par block: 3 branches; each step runs concurrently`; on `par "the advert"`: `par block: 2 branches`; on `loop …`: `loop block: 1 branch`. Hover on a `note` keyword: `note — drawn over the previous step`; on `finding`: ``finding — drawn highlighted and exported to CraftDoc (`Step.Note`)``; on `receives`: `callee-side trigger: the caller is not observed; drawn as a found message`.
  - Completion on a blank line in a scenario offers `par`, `and` (only when the line above sits in an open `par`), `loop`, `note`, `finding` next to `alt`/`opt`/`else`/`scenario`, with snippets `par "${1:label}"`, `and "${1:label}"`, `loop "${1:condition}"`, `note "${1:text}"`, `finding "${1:text}"`. Completion after `when graphql ` offers `listens` and `receives`. Completion after `when x listens store.a.inserted ` offers `cron "${1:* * * * *}"` and `every "${1:5m}"`.
  - Semantic tokens: `par`, `and`, `loop`, `note`, `finding`, `receives` are `craft-flow-keyword`; a note's text is `craft-regular-string`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  - `blockSymbols` also emits a finding symbol for each `Note` step of kind finding (`Name: "finding: " + first60(text)`, `Kind: protocol.SymbolKindEvent`, range = its line). `DocumentSymbol` prepends use-case-level findings (from `uc.Notes()`) to the use case's children, before `scenarioSymbols`. `first60` cuts at 60 runes.
  - `blockHover`: for `b.IsFanOut()` report `len(branch body steps)` branches plus `; each step runs concurrently`. Add a keyword hover: when the cursor's token kind is `SyntaxKindKwNote`, `SyntaxKindKwFinding` or `SyntaxKindKwReceives`, return the texts above (find the token with the existing token-at-position helper used by `Definition`; `grep -n "TokenAt" internal/lsp/server.go`).
  - `blockKeywordCompletions(inPar bool)` adds the new snippets; `and` only when `inPar`. `inPar`: the tree node at the end of the previous non-blank line has a `SyntaxKindBlock` ancestor whose `BlockDecl.Kind()` is `par`.
  - `detectContext` gains `ctxTriggerVerb` (line prefix matches `^\s*when\s+\S+\s+$`) → `listens`, `receives`; and `ctxListensTail` (prefix matches `^\s*when\s+\S+\s+listens\s+\S+\s+$`) → the two schedule snippets.
  - `syntaxKindToCraftToken` maps the six keyword kinds to `craft-flow-keyword`.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): outline, hover, completion and tokens for par/loop, notes, findings and receives`.

---

### Task 20: LSP for surfaces and store refs — **strong**

**Files:** Modify `internal/lsp/server.go` (`DocumentSymbol`, `FoldingRanges`, `Hover`, `Definition`), `internal/lsp/completion.go`, `internal/lsp/blocks.go` (or a new `internal/lsp/surfaces.go`), `internal/workspace/workspace.go` (`Doc`); Test `internal/lsp/server_test.go`, `internal/lsp/completion_test.go`

- [ ] **Step 1: Failing tests**
  - Outline of `surfaceModel` (Task 16): a top-level `surface concierge` symbol (kind `Interface`, detail `surface`) with children `getUserList`, `getUserLogs`, `deleteAccount` (kind `Method`, detail `query`/`command`). A folding range spans the surface block.
  - Hover on the surface subject: `surface of concierge: 3 operations (2 query, 1 command); callers: Advisor`. Hover on the `[getUserList]` of `advisorapp asks concierge for the seekers  [getUserList]`: `query: the seeker list, unpaged, newest first, filtered by a typed status`.
  - Definition from that bracket goes to the `query … [getUserList]` line of the surface.
  - Completion inside `[` after `advisorapp asks concierge for x  [` offers `getUserList`, `getUserLogs`, `deleteAccount`. Completion on a blank top-level line offers a `surface` snippet (`surface ${1:boundary} {\n    callers: ${2:unknown}\n    query ${3:what it answers}  [${4:operation}]\n}`). Completion on a blank line inside a surface offers `query`, `command`, `callers:`, `spec:`, `note`, `finding`.
  - Hover on `store.sms_queue.inserted` in `storeSrc` (Task 13): `store hand-off: sms_queue, change inserted; written by invoicing; polled by smsdispatch`. Definition on it goes to the `data-stores:` entry `sms_queue`. Completion after `notifies store.` offers `sms_queue` (store names already used in the workspace).
  - `initialize` still advertises only capabilities that have handlers (`docs/AGENT.md` §3): nothing new is advertised here, all handlers already exist.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  - `Workspace` keeps the merged resolved document: `w.resolution.doc = res.Doc` in `recomputeResolution`, and `func (w *Workspace) Doc() *craft.CraftDoc` (read lock).
  - Surfaces in `DocumentSymbol`/`FoldingRanges` come from `file.Surfaces()` (tree), like exposures.
  - Hover/definition/completion for brackets: resolve the action's target with `sema.ResolveParticipant(s.ws.WorkspaceSymbols(), "", target)`, then find the surface whose subject resolves to the same ID across `s.ws.AllFiles()` (first in sorted URI order), and match the collapsed op text.
  - `treeEnclosingBlock` returns `"surface"` inside a `SyntaxKindSurfaceDecl`; `detectContext` maps it to a new `ctxSurfaceBody`. `ctxOpAnnotationVerb` (the `[` head) also adds the target's surface operations when the line is an `asks`.
  - Store hover: find the `Publish`/`Listen` interactions of `interaction.Derive(s.ws.Doc())` with that event; list writers (`From` of publishes) and readers (`To` of listens), sorted and deduplicated. Store definition: the `data-stores:` token of the owning service (`syntax.ServiceDecl.DataStoreTokens()`).
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): surfaces in outline, hover, definition and completion; store hand-off hover and definition`.

---

### Task 21: Visualizer goldens and mermaid-cli check — **cheap**

**Files:** Create `internal/visualizer/testdata/journey-semantics.craft`, `internal/visualizer/testdata/golden/journey-semantics.*`, `internal/visualizer/testdata/golden/surface.md`; Modify `internal/visualizer/golden_test.go`, `internal/visualizer/render_integration_test.go`

- [ ] **Step 1:** `journey-semantics.craft` holds the spec examples of §4.1 (with the `and` snippet as a second scenario), §5.1, §6.1 and §7.5, with the actors (`Seeker`, `Seller`, `CRON`, `Siren`) and domains/services that make every participant and `sms_queue` resolve. Append `"testdata/journey-semantics.craft"` to `goldenModels`.
- [ ] **Step 2:** `go test ./internal/visualizer/ -run TestGolden -update`. Review the six spec views by hand against the spec's PlantUML/Mermaid listings (§4.5, §5.4, §6.5, §7.5): the same lines must appear, in the same order, within each section. The six C4 goldens are new too; check they contain no `unobserved` and a `sms_queue` database only in the `-db`/`-focus` variants. `git status` shows only new `journey-semantics.*` files.
- [ ] **Step 3:** Add `TestGolden_SurfaceMarkdown`: render `surfaceModel` (Task 16) as `ViewSurface` and compare with `testdata/golden/surface.md` (same `-update` flag).
- [ ] **Step 4:** In `render_integration_test.go`, copy `TestVAS_MermaidSequenceRenders` into `TestJourneySemantics_MermaidSequenceRenders` and `TestJourneySemantics_PlantUMLRenders` (via `assertRendersClean`) for `journey-semantics.craft`. Run `go test -tags integration ./internal/visualizer/ -run JourneySemantics`.
  - **If mmdc rejects the bare `and`** (spec §4.5 fallback): add `Split bool` to `view.BranchRef` and `view.BlockOp`, stop blanking labels in `viewRef` (keep comparing by Block/Index), and let the PlantUML writer print a bare `else` for a split branch while the Mermaid writer prints `and <label>`. Update Task 8's tests and regenerate the goldens.
- [ ] **Step 5:** Full suite; commit `test(visualizer): journey-semantics and surface goldens; mermaid-cli and PlantUML render checks`.

---

### Task 22: Documentation and changelog — **cheap**

**Files:** `docs/GRAMMAR.md`, `docs/DIAGNOSTICS.md`, `CONTEXT.md`, `README.md`, `docs/page/language/use-cases.md`, `CHANGELOG.md`

- [ ] **Step 1: GRAMMAR.md.** §2 top-level gains `surface_decl`. §8 gains:

```ebnf
step          = action | block | note_stmt ;
block         = alt_block | opt_block | par_block | loop_block ;
par_block     = 'par' STRING NL+ body ('and' STRING NL+ body)* ;   (* no `and`: each body step is a branch *)
loop_block    = 'loop' STRING NL+ body ;
note_stmt     = ('note' | 'finding') STRING ;
use_case_body = tags_block? (note_stmt NL+)* scenario* ;
trigger       = … | receive_trigger ;
receive_trigger = name_slot 'receives' connector_word? phrase? op_annotation? ;
domain_listen = name_slot 'listens' (STRING | ref) (('cron' | 'every') STRING)? ;
surface_decl  = 'surface' name_slot '{' NL* (surface_item NL+)* '}' ;
surface_item  = 'callers' ':' name_list | 'spec' ':' STRING | note_stmt
              | ('query' | 'command') connector_word? phrase op_annotation ;
```

  Add a paragraph on store refs (`store.<name>.<change>`, no new grammar). §11.1 gains the spec §3.1 table.
- [ ] **Step 2: DIAGNOSTICS.md.** Add `craft/syntax/orphan-and`, `craft/syntax/missing-operation`, `craft/sema/invalid-surface-subject`, `craft/sema/duplicate-operation`, `craft/lint/single-branch-par`, `craft/lint/store-without-change`, `craft/lint/receives-on-actor`, `craft/lint/operation-not-in-surface`, `craft/lint/repeated-request`, each with severity and a one-line meaning. Widen `orphan-else`, `missing-label`, `empty-branch`, `inconsistent-indent`, `unresolved-reference` (surfaces, error), `duplicate-name` (surfaces) and `invalid-schedule` (listens tail).
- [ ] **Step 3: CONTEXT.md** glossary: **Surface**, **Finding**, **Note**, **Store hand-off**, **Unobserved caller**, one or two sentences each in the existing style.
- [ ] **Step 4: README.md and `docs/page/language/use-cases.md`.** A short section per construct with the spec example; a "Surfaces" section with the §8.1 example and the §8.4 table.
- [ ] **Step 5: CHANGELOG.md.** New `## [Unreleased]` at the top:
  - `### Added`: `par`/`and` (fan-out form), `loop`; `note`/`finding` (`Step.Note`, `UseCase.Notes`); `when <ctx> receives <phrase> [op]` (`Trigger.Operation`, found/lost messages); store hand-offs (`Action.Store`, `Trigger.Store`, `StoreRef`) and the listens `cron`/`every` tail; `surface` blocks (`CraftDoc.Surfaces`) and `craft generate --type surface`; the new codes; the `pkg/craft` additions (list them).
  - `### Changed`: `when X receives …` is now a callee-side trigger (it used to be an external trigger with verb `receives`; no known uses). Store refs draw a store participant instead of a self-arrow `notifies "store.…"`. `orphan-else` widens its message. Writers no longer print a trailing space after an empty block label. `--type all` also writes `<base>-surface.md` when the model declares a surface (overwriting it needs `--force`).
  - `### Changed — diagrams`: par/loop frames (Mermaid `and`), notes and findings, found/lost messages, store lanes; flow `[par: …]`/`[loop: …]` prefixes.
- [ ] **Step 6: Commit** `docs: journey semantics (par/loop, notes, receives, store hand-offs, surfaces)`.

---

### Task 23: Dogfood on re-arch-central (verification only) — **cheap**

**Repo:** `~/Downloads/domain-hub/re-arch-central` (read only; corpus migration comes after the release, spec §11.3)

- [ ] **Step 1:** `CRAFT=$(mktemp -d)/craft && go build -o "$CRAFT" ./cmd/craft` (in the craft repo).
- [ ] **Step 2:** In re-arch-central, run `$CRAFT validate docs/topology.craft docs/actors.craft docs/context_map.craft 'docs/journeys/*/*.craft' > /tmp/js-after.txt` and the same with the released `craft` (v2.22.0) into `/tmp/js-before.txt`. Expect **0 errors** in both. Compare code counts (`grep -o 'craft/[a-z-]*/[a-z-]*' FILE | sort | uniq -c`): the only new code is `craft/lint/repeated-request`, about 8 hits in 7 files (spec §9). Any other new code is a bug: fix it in craft before going on.
- [ ] **Step 3:** Copy `docs/journeys/invoicing/atlas-invoice-sms-reminders.craft` and `docs/journeys/ad/seeker-advert-detail.craft` to a temp dir. Rewrite the first with `listens store.sms_queue.inserted cron "* 10 * * 1-5"` (drop the `polls` tag) and the second with a fan-out `par` around the concurrent field-resolver asks. Validate them together with the topology/actors files (0 errors), and render sequence and flow in `puml` and `mmd` into `~/Downloads/domain-hub/journey-assessment/_render-journey-semantics/`. Check by eye: one `par` box with one arrow per resolver; a `database "sms_queue"` lane with `polls inserted (from …), cron * 10 * * 1-5`.
- [ ] **Step 4:** Nothing is committed in re-arch-central. Record the before/after code counts in the release commit message body (Task 24).

---

### Task 24: Release craft v2.23.0 — **cheap**

Mirror v2.22.0 (`git show 2554aff --stat`: CHANGELOG, spec status, pre-release fixes in one `chore(release)` commit; an annotated tag).

- [ ] **Step 1:** In `CHANGELOG.md`, rename `## [Unreleased]` → `## [2.23.0] — <release date, YYYY-MM-DD>`. In the spec, set `**Status:** Implemented (v2.23.0)`.
- [ ] **Step 2:** `go vet ./... && go test ./...` and `go test -tags integration ./internal/visualizer/` → PASS.
- [ ] **Step 3:** Commit:

```bash
git add CHANGELOG.md docs/superpowers/specs/2026-10-14-journey-semantics-design.md
git commit -m "chore(release): v2.23.0

Dogfood on re-arch-central: 0 errors; the only new warning code is
repeated-request (<N> hits).

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 4:** `git push -u origin feat/journey-semantics`; `git switch main && git merge --ff-only feat/journey-semantics && git push origin main`.
- [ ] **Step 5:** `git tag -a v2.23.0 -m "v2.23.0: journey semantics — par/loop, notes and findings, receives, store hand-offs, surfaces"` and `git push origin v2.23.0`.
- [ ] **Step 6:** `gh run list --repo tcarcao/craft --limit 4`, then `gh run watch <id> --repo tcarcao/craft --exit-status` for `release.yml` and `publish.yml`. Then `brew upgrade tcarcao/craft/craft && craft --version` → `2.23.0`, and `GOPROXY=https://proxy.golang.org go list -m github.com/tcarcao/craft/v2@v2.23.0` resolves (the hub needs it).

---

### Task 25: tree-sitter-craft: grammar, corpus, CORPUS_VERSION, release 0.3.0 — **strong**

**Repo:** `~/projects/github/tcarcao/tree-sitter-craft` (branch `feat/journey-semantics` off `main`; `main` holds the unreleased v2.21 grammar)

**Files:** `grammar.js`, `queries/highlights.scm`, `test/corpus/branches.txt`, new `test/corpus/journey_semantics.txt`, `CORPUS_VERSION`; regenerated `src/grammar.json`, `src/node-types.json`, `src/parser.c`

- [ ] **Step 1: Failing corpus tests.** In `test/corpus/journey_semantics.txt` (same `===` / `---` format as `branches.txt`), one case each for: a fan-out `par`; `par`/`and`; `loop`; use-case-level and step `note`/`finding`; `when graphql receives a reindex sweep [reindexAdverts]`; `when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"`; the spec §8.1 surface. In `test/corpus/branches.txt`, add the `and` risk cases (spec §10.2): phrases containing `and`, `loop`, `note`, `query` (`billing charges the card and notifies the seller`, `graphql asks atlas for the loop and note fields`), and those words as action subjects (`and asks loop to run`, `note records it`, `query does it`). Write each expected tree from `npx tree-sitter parse` once the grammar is in; until then `npx tree-sitter test` fails.
- [ ] **Step 2: Grammar.**

```js
    _branch_keyword: $ => choice('alt', 'opt', 'else', 'scenario', 'par', 'and', 'loop', 'note', 'finding'),

    branch_header: $ => prec.dynamic(10, seq(
      choice('alt', 'opt', 'else', 'par', 'and', 'loop'),
      optional($.string),
    )),

    note_stmt: $ => prec.dynamic(10, seq(choice('note', 'finding'), optional($.string))),

    receive_trigger: $ => prec.dynamic(10, seq(
      $.listener_subject,
      'receives',
      optional($.connector_word),
      optional($.phrase),
    )),

    domain_listener: $ => seq(
      $.listener_subject,
      'listens',
      choice($.string, $.ref),
      optional($.cron_trigger),
    ),

    surface_block: $ => seq(
      'surface', $.ref, '{', repeat1($._newline),
      repeat(seq(choice($.surface_property, $.note_stmt, $.surface_operation), repeat1($._newline))),
      '}', repeat($._newline),
    ),
    surface_property: $ => choice(
      seq('callers', ':', $.identifier_list),
      seq('spec', ':', $.string),
    ),
    surface_operation: $ => seq(choice('query', 'command'), optional($.connector_word), $.phrase),
```

  (`phrase` already contains `operation_annotation`, so the bracket of a `receives` trigger and a surface line parses inside it.) Wire them in: `surface_block` in `_top_level_item`; `seq($.receive_trigger, $._newline)` in `when_clause`'s choice; `$.note_stmt` in the `when_clause` repeat (`choice($.action, $.branch_header, $.scenario_name, $.note_stmt)`) and in a new `scenario_continuation` alternative; `repeat(seq($.note_stmt, repeat1($._newline)))` in `use_case_block` after the optional `tags_block`.
- [ ] **Step 3: Highlights.**

```scheme
(branch_header ["alt" "opt" "else" "par" "and" "loop"] @craft.flow-keyword)
(note_stmt ["note" "finding"] @craft.flow-keyword)
(note_stmt (string) @craft.usecase-string)
(receive_trigger "receives" @craft.flow-keyword)
(domain_listener (cron_trigger ["cron" "every"] @craft.flow-keyword))
(surface_block "surface" @craft.flow-keyword)
(surface_property ["callers" "spec"] @craft.flow-keyword)
(surface_operation ["query" "command"] @craft.flow-keyword)
(surface_operation (phrase) @craft.usecase-string)
```

  (Replace the existing `branch_header` line.) These capture names already exist in the extension's token list.
- [ ] **Step 4: Generate and test.** `npx tree-sitter generate && npx tree-sitter test`. A conflict report is resolved with `prec`/`prec.dynamic` (as `branch_header` does), not by removing a form. **Every** case must parse with no `ERROR` or `MISSING` node: the extension turns each into a diagnostic. If a phrase word (`and`) comes out as a keyword, add `alias($._branch_keyword, $.identifier)` to `phrase`'s choices. Commit `feat(grammar): par/and/loop, notes and findings, receives, listens schedule, surface blocks`.
- [ ] **Step 5: CORPUS_VERSION** (needs the v2.23.0 tag from Task 24). Set `CORPUS_VERSION` to `v2.23.0`, run `scripts/fetch-corpus.sh`, then the parse command `compat.yml` runs (`grep -n 'tree-sitter parse' .github/workflows/compat.yml`) over `test/corpus-from-craft`. It covers the v2.22 fixtures too (`scheduled_triggers.craft`). Fix the grammar until every file parses; commit `chore: bump CORPUS_VERSION to v2.23.0` (that file only, as `9d2b144` did).
- [ ] **Step 6: Release.** Merge to `main` (`git switch main && git merge --ff-only feat/journey-semantics && git push origin main`), then `./release.sh minor` (creates the `v0.3.0` commit and tag and pushes them). Watch `release.yml` (`gh run watch <id> --repo tcarcao/tree-sitter-craft --exit-status`) and check `npm view tree-sitter-craft version` → `0.3.0`.

---

### Task 26: VSCode extension: bump and completions — **cheap**

**Repo:** `~/projects/github/tcarcao/craft-vscode-extension` (branch `feat/journey-semantics` off `main`)

**Files:** `package.json`, `server/package.json` (+ both lock files), `server/src/parser/TreeSitterCompletionProvider.ts`; Create `server/src/parser/TreeSitterCompletionProvider.test.ts`

- [ ] **Step 1: Failing test** (`jest`, ESM like the existing tests)

```ts
import { TextDocument } from 'vscode-languageserver-textdocument';
import { TreeSitterCompletionProvider } from './TreeSitterCompletionProvider.js';

const labels = async (text: string, line: number, character: number) => {
  const doc = TextDocument.create('file:///t.craft', 'craft', 1, text);
  const items = await new TreeSitterCompletionProvider({}).getCompletions(doc, { line, character });
  return items.map(i => i.label);
};

test('use-case body offers the block and note snippets', async () => {
  const text = 'use_case "U" {\n    when O does it\n        x does a\n        \n}\n';
  expect(await labels(text, 3, 8)).toEqual(expect.arrayContaining(['par', 'and', 'loop', 'note', 'finding']));
});

test('after when <name> offers listens and receives', async () => {
  const text = 'use_case "U" {\n    when graphql \n}\n';
  expect(await labels(text, 1, 17)).toEqual(expect.arrayContaining(['listens', 'receives']));
});

test('after a listens ref offers the schedule tail', async () => {
  const text = 'use_case "U" {\n    when x listens store.a.inserted \n}\n';
  expect(await labels(text, 1, 36)).toEqual(expect.arrayContaining(['cron', 'every']));
});

test('surface body offers its items; the root offers surface', async () => {
  expect(await labels('surface s {\n    \n}\n', 1, 4)).toEqual(expect.arrayContaining(['query', 'command', 'callers', 'spec', 'note', 'finding']));
  expect(await labels('\n', 0, 0)).toEqual(expect.arrayContaining(['surface', 'use_case']));
});
```

- [ ] **Step 2: Run** `npm test -- TreeSitterCompletionProvider` → FAIL.
- [ ] **Step 3: Implement.** Keep it text-based (the provider never used the tree). Add `enclosingBlock(lines, line)`: scan upward counting `{`/`}` outside strings and `//` comments; the first unbalanced `{` line decides (`use_case` → `'use_case'`, `surface` → `'surface'`, none → `'root'`, anything else → `'other'`). Then: root + blank prefix → existing root items plus `surface`; `use_case` + blank prefix → `par "$1"`, `and "$1"`, `loop "$1"`, `note "$1"`, `finding "$1"` (and `alt`/`opt`/`else`/`scenario` while there); prefix matching `/^\s*when\s+\S+\s+$/` → `listens`, `receives`; `/^\s*when\s+\S+\s+listens\s+\S+\s+$/` → `cron "$1"`, `every "$1"`; `surface` + blank prefix → `query $1  [$2]`, `command $1  [$2]`, `callers: $1`, `spec: "$1"`, `note "$1"`, `finding "$1"`. All with `insertTextFormat: 2`.
- [ ] **Step 4: Bump.** `"tree-sitter-craft": "^0.3.0"` in `package.json` and `server/package.json`; `npm install && (cd server && npm install) && npm run compile` (runs `copy-tree-sitter`, which refreshes `resources/tree-sitter-craft.wasm` and `resources/queries`). Open `testdata/corpus/04_use_cases/par_fanout.craft` and `09_surfaces/surface_basic.craft` from the craft repo in the Extension Development Host: no syntax-error squiggles, and the new keywords are highlighted.
- [ ] **Step 5: Commit and release.** `npm test` → PASS. Commit `chore: bump tree-sitter-craft to 0.3.0; completions for par/and/loop, notes, receives, listens schedule and surfaces` (files from this task only). Merge to `main`, push, and run `./release.sh 0.0.11` (the GitHub Action bumps the version and publishes, as for 0.0.10).

---

### Task 27: Hub: bump craft, fix `scenarioLabel`, par/loop, receives, store — **strong**

**Repo:** `~/Downloads/domain-hub/svc-re-go-arch-context`, branch `feature/craft-v2.21-branches` (checked out, clean)

**Files:**
- Modify: `go.mod`, `go.sum`, `internal/infrastructure/graphstore/craft_loader.go` (`scenarioLabel`, `branchPaths`, `journeyStepFromTrigger`, `journeyStepFromAction`, `applyEventRef` call sites), `internal/infrastructure/graphstore/model.go` (`refVerb`, `step`), `internal/infrastructure/graphstore/join.go` (`resolveOne`), the `parseStepRef`/`typedRef` file (`grep -n "func parseStepRef" internal/infrastructure/graphstore/*.go`), `internal/core/domain/use_case.go` (`BranchKind`)
- Create: `internal/infrastructure/graphstore/scenario_label_internal_test.go`, `internal/infrastructure/graphstore/craft_v223_journey_semantics_test.go`

- [ ] **Step 1: Bump.** `go get github.com/tcarcao/craft/v2@v2.23.0 && make mod`. `go build ./...` must pass unchanged (v2.22/v2.23 are additive).
- [ ] **Step 2: Failing tests.**

```go
// scenario_label_internal_test.go
package graphstore

import (
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func TestScenarioLabelKeepsTheVerbAndConnector(t *testing.T) {
	cases := []struct {
		t    craft.Trigger
		want string
	}{
		{craft.Trigger{Type: craft.TriggerTypeExternal, Actor: "Seller", Verb: "publishes", Connector: "the", Phrase: "draft"}, "Seller publishes the draft"},
		{craft.Trigger{Type: craft.TriggerTypeExternal, Actor: "CRON", Verb: "fires", Phrase: "otodompl main statistics flow"}, "CRON fires otodompl main statistics flow"},
		{craft.Trigger{Type: craft.TriggerTypeExternal, Actor: "CRON"}, "started by CRON"},
		{craft.Trigger{Type: craft.TriggerTypeReceive, Context: "graphql", Connector: "a", Phrase: "reindex sweep", Operation: &craft.Operation{Text: "reindexAdverts"}}, "graphql receives a reindex sweep [reindexAdverts]"},
	}
	for _, c := range cases {
		if got := scenarioLabel(c.t); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
```

`craft_v223_journey_semantics_test.go` (package `graphstore_test`; reuse `refsFixture`, `refsManifest`, `refsTopology`, `refsOpenAPI` from `craft_v216_refs_test.go`; the advert spec publishes `postSchedulepromotion`):
- **par/loop:** a use case with `par "lookups"` (two `advert asks billing …` lines) and `loop "for each ad"` (one asks): the steps carry `Branch[0].Kind == domain.BranchKindPar` / `domain.BranchKindLoop` and the header label as `BranchLabel` (for both fan-out branches).
- **receives:** `when advert receives a promotion request  [postSchedulepromotion]` then `advert notifies PromotionScheduled`: `ExplainUseCase` returns a first step with verb `receives` that is **not** unresolved and whose operation is `postSchedulepromotion`; no step has the same source and target context.
- **store:** use case A `advert notifies store.promo_queue.inserted`, use case B `when billing listens store.promo_queue.inserted cron "*/5 * * * *"`: neither step is unresolved (before the change both were `dangling ref … no manifest emits`), and both carry the same canonical id `store.promo_queue.inserted`.
- **notes:** a `note`/`finding` inside the scenario changes neither `AuthoredActions` nor the step count.
- **scenario label:** a scenario `when Seller publishes the draft` has `ScenarioTrigger` (or `ScenarioLabel`, when unnamed) `Seller publishes the draft`.
- [ ] **Step 3: Run** `go test ./internal/infrastructure/graphstore/ -run 'ScenarioLabel|V223|JourneySemantics'` → FAIL.
- [ ] **Step 4: Implement.**
  1. `scenarioLabel`, external case: build the authored wording from all four fields (the `Phrase` field excludes the verb and the connector, which is the bug):

```go
	case craft.TriggerTypeExternal:
		wording := strings.Join(strings.Fields(t.Verb+" "+t.Connector+" "+t.Phrase), " ")
		switch {
		case t.Actor != "" && wording != "":
			return t.Actor + " " + wording
		case t.Actor != "":
			return "started by " + t.Actor
		case wording != "":
			return wording
		}
	case craft.TriggerTypeReceive:
		label := strings.Join(strings.Fields(t.Context+" receives "+t.Connector+" "+t.Phrase), " ")
		if op := operationName(t.Operation); op != "" {
			label += " [" + op + "]"
		}
		return label
```

     Fix the comment above the old code (it claimed `Phrase` was the whole wording).
  2. `domain.BranchKindPar BranchKind = "par"`, `domain.BranchKindLoop BranchKind = "loop"`; `branchPaths` maps `craft.BlockPar`/`craft.BlockLoop` to them (a `switch st.Block.Kind`). Update the `BranchFrame.Index` comment (`and` branches too).
  3. `model.go`: `verbReceives refVerb = "receives"`; `step` gains `// Store is the store a store hand-off writes or reads; it is never resolved against manifests (none declares a table).` `Store string`.
  4. `journeyStepFromTrigger`:

```go
	case craft.TriggerTypeReceive:
		op := operationName(t.Operation)
		s := &step{Actor: t.Context, Verb: verbReceives, RefRaw: t.Context, BCRef: t.Context, Name: op}
		if op != "" {
			s.RefRaw = t.Context + " [" + op + "]"
		}
		return s
```

     and in the listens case set `s.Store = t.Store.Name` when `t.Store != nil`; `journeyStepFromAction` does the same for an async action's `a.Store`.
  5. `parseStepRef`/`typedRef`: treat `verbReceives` like `verbAsks` (same `<ctx> [op]` shape) and copy `s.Store` into the typed ref (`Store string`).
  6. `resolveOne`: before the `IsCanonical` branch, a store hand-off resolves like free text:

```go
	if ref.Store != "" {
		r.Kind = refKindAsync
		r.CanonicalID = ref.Name // the full store.<name>.<change> ref: writer and reader pair on it
		r.SourceBC, r.TargetBC = ref.ActingBC, ref.ActingBC
		return r, nil
	}
```

     and `case verbAsks, verbReceives:` resolves the operation against `ref.BCShort`'s manifest; for `verbReceives` set `r.SourceBC = r.TargetBC` (the callee; the caller is not observed). If `translate.go` builds an edge for a sync ref whose source equals its target (`grep -n refKindSync internal/infrastructure/graphstore/translate.go`), skip edge creation for `verbReceives` there; the receives test pins "no self-edge".
  7. Leave the `NoObservedCaller` sentence in `hubresourcehdl/definitions.go` until the corpus migrates (spec §10.4).
- [ ] **Step 5: Run** `make lint && make test` → PASS (there is no `validate-schema` target).
- [ ] **Step 6: Commit and push** (vendor/ is ignored; `go.mod`/`go.sum` plus the changed files):

```bash
git add go.mod go.sum internal/core/domain/use_case.go internal/infrastructure/graphstore
git commit -m "feat(graphstore): craft v2.23 — par/loop branches, receives triggers, store hand-offs; fix scenarioLabel dropping the verb

scenarioLabel joined Actor and Phrase, but craft's Phrase excludes the verb and
the connector: \"when Seller publishes the draft\" rendered \"Seller draft\".

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
git push
```

---

### Task 28: Hub: findings and surfaces — **strong**

**Repo:** same branch as Task 27.

**Files:**
- Modify: `internal/infrastructure/graphstore/model.go` (`finding`, `useCase.Findings`, `surface`, `craftCorpus.Surfaces`), `craft_loader.go` (`loadJourneys`, new `collectFindings`, new `loadSurfaces`), `use_case.go` (`buildOne`), `graphstore.go` (a `FindSurfaces` repository method), `internal/core/domain/use_case.go` (`Finding`, `UseCase.Findings`), a new `internal/core/domain/surface.go`, `internal/core/outboundprt/graph_repository.go`, `internal/core/inboundprt/hub_service.go`, `internal/core/application/hubsvc/hub_service.go`, `internal/presentation/rest/graphhdl/{handler,response}.go`, `internal/presentation/mcp/explainusecasehdl/response.go`, `internal/presentation/mcp/hubresourcehdl/{definitions,handler}.go`, `web/src/lib/api.ts`
- Test: `internal/infrastructure/graphstore/craft_v223_journey_semantics_test.go`, `internal/presentation/mcp/hubresourcehdl/` (a surfaces resource test next to `contextmap_test.go`)

- [ ] **Step 1: Failing tests.**
  - A use case with a use-case-level `finding "no retry"`, a `note "context"` and, inside `alt "declined"`, `finding "no idempotency key"`: `ExplainUseCase(...).Findings` = `[{Text: "no retry", Scenario: -1}, {Text: "no idempotency key", Scenario: 0, BranchLabel: "declined", Line: <its line>}]`; notes are not stored.
  - A `surface advert { callers: unknown  query the promotion schedule  [postSchedulepromotion]  command …  finding "…" }` (one item per line): `repo.FindSurfaces(ctx)` returns one surface with boundary `advert`, callers `["unknown"]`, the two operations (kind, name via `operationName`, phrase) and the finding.
  - Reading `archctx://surfaces` returns that surface as JSON.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.**
  1. Loader: `type finding struct{ Text string; Line, Scenario int; ScenarioLabel, BranchLabel string }`; `useCase.Findings []finding`. In `loadJourneys`, append `uc.Notes` findings with `Scenario: -1` (the whole use case), then per scenario a `collectFindings(sc.Steps, scenarioIdx, label, "")` walk that records `Step.Note` with `Kind == craft.NoteKindFinding` and the innermost branch label.
  2. Domain: `type Finding struct{ Text string; Line, Scenario int; ScenarioLabel, BranchLabel string }` (doc: Scenario is -1 for a use-case-level finding); `UseCase.Findings []Finding`; map it in `buildOne`. REST `response.go`: `Findings []findingResponse \`json:"findings,omitempty"\`` mirroring how `f381e0f` added the branch fields; `api.ts` gains the optional field. MCP `explainusecasehdl/response.go`: after the steps, a `Findings:` section, one line per finding (`- [scenario label / branch] text`).
  3. Surfaces: `domain.Surface{Boundary string; Callers []string; Spec string; Notes, Findings []string; Operations []SurfaceOperation; SourceFile string}`, `domain.SurfaceOperation{Kind, Name, Phrase string}`. Loader `loadSurfaces(doc, sourceFile, corpus)` (call it next to `loadActors`) fills `corpus.Surfaces`, splitting notes by kind and naming each operation with `operationName(&op.Operation)`. Port `FindSurfaces(ctx) ([]domain.Surface, error)` on `outboundprt.GraphRepository`, the hub service and `inboundprt`; run `make mockgen`. MCP: `surfacesURI = "archctx://surfaces"` in `definitions.go` (description: "Every operation table the Craft corpus declares with `surface` blocks: one per boundary, each operation marked query (no side effect) or command (changes state), with its observed callers or `unknown`, the spec it comes from, and findings about the boundary. Use it to see what a boundary offers beyond the journeys that use it."), served by a `readSurfaces` modelled on `readContextMap`.
  4. `Scope == "surface"` use cases are still read as today (no special case exists); delete nothing until the corpus migrates.
- [ ] **Step 4: Run** `make lint && make test` → PASS.
- [ ] **Step 5: Commit and push** `feat(graphstore): findings on use cases and surface tables from craft v2.23` (the files of this task), then `git push`. Opening the MR (`glab mr create`) needs the user's go-ahead in the session.

---

## Self-review notes

- **Spec coverage:**
  - §2 decisions A–F: A → Tasks 5–8; B → 2–4; C → 9–10; D → 11–13; E → 14–16; F → 17.
  - §3.1 contextual keywords: Tasks 2, 5, 9, 11, 14; tests keep every word usable as a name (`KeywordsStayNames`, `ParKeywordsStayNames`, `WordStaysAName`). §3.2 kinds: Task 2. §3.3 vocabulary: Task 1. §3.4 IDs: Tasks 3 and 6.
  - §4.2 diagnostics: Task 5 (orphan-and, widened orphan-else, missing-label, empty-branch), Task 6 (single-branch-par). §4.3 model: Tasks 1, 6. §4.4 semantics: Task 7. §4.5 rendering: Task 8 (+ fallback in Task 21). §4.6 formatter/LSP: Tasks 18, 19.
  - §5.1–5.2: Tasks 2, 3. §5.3: no new code; indistinguishable unchanged (Task 9 test). §5.4: Task 4. §5.5: Tasks 18, 19.
  - §6.1–6.3: Task 9. §6.4–6.5: Task 10. §6.6: Tasks 18, 19.
  - §7.1–7.3: Tasks 11, 12. §7.4: Tasks 11, 12. §7.5: Task 13. §7.6: Tasks 18, 19, 20.
  - §8.1–8.2: Tasks 14, 15 (+ the lint in 16). §8.3: Tasks 1, 15. §8.4: Task 16. §8.5: Tasks 18, 20.
  - §9: Task 17. §10.1: Task 1. §10.2: Task 25. §10.3: Task 26. §10.4: Tasks 27–28. §10.5: Task 22.
  - §11.1 tests: per task (corpus 3, 6, 9, 11, 12, 15; broken 2, 5, 11, 14, 15; lint 6, 9, 12, 16, 17; interaction 7, 10, 11, 12; goldens 21; formatter/fuzz 18; LSP 19, 20; CLI 16; dogfood 23). §11.2: Tasks 22, 24. §11.3 migration: out of scope (after release).
  - User-requested extras: hub `scenarioLabel` verb fix (Task 27), release pattern of v2.22.0 (Task 24).
- **Names used consistently:**
  - lang/model: `lang.BlockPar`/`BlockLoop`, `lang.TriggerReceive` (`model.TriggerTypeReceive`), `lang.NoteText`/`NoteFinding` (`model.NoteKindNote`/`NoteKindFinding`), `lang.SurfaceQuery`/`SurfaceCommand`, `lang.StoreInserted`/`Updated`/`Deleted`, `lang.UnknownCaller`, `lang.ParseStoreRef`; `model.Note`, `Step.Note`, `UseCase.Notes`, `Block.Split`, `Trigger.Operation`, `Trigger.Store`, `Action.Store`, `StoreRef{Name, Change, Participant}`, `Surface`, `SurfaceOperation`, `CraftDoc.Surfaces`, `Scenario.OpeningBlock()`.
  - syntax: `SyntaxKindKwPar/KwAnd/KwLoop/KwNote/KwFinding/KwReceives/KwSurface/KwCallers/KwSpec/KwQuery/KwCommand`, `SyntaxKindNote/SurfaceDecl/SurfaceField/SurfaceOperation`; `atNoteLine`, `parseNote`, `sweepRestOfLine`, `atSurfaceDecl`, `parseSurfaceBlock`, `parseSurfaceItem`, `parseSurfaceOperation`; `NoteDecl{KeywordToken, Kind, Text, Node}`, `StepDecl.Note()`, `UseCaseDecl.Notes()`, `BlockDecl.IsFanOut()`, `TriggerDecl.OpAnnotation/OpText/OpVerb/OpPayload/ListenSchedule/ListenScheduleToken`, `File.Surfaces()`, `SurfaceDecl`, `SurfaceOpDecl`.
  - interaction/view: `interaction.Unobserved`, `Interaction.Via`, `Interaction.Schedule`, `BranchRef.Split`, `walker.reached`, `firstStepIndex`, `storeEndpoint`, `scheduleLabel`; `view.MessageStepNote/MessageFinding/MessageFound/MessageLost`, `Message.Over`, `NodeStore`, `nodeSet.addStore`, `viewRef`, `Flow.Stores`, `BuildSurfaces`, `Surfaces/SurfaceTable/SurfaceRow`, `markdown.Surfaces`, `visualizer.ViewSurface`.
  - sema: `lintSingleBranchPar`, `lintReceivesOnActor`, `lintOperationNotInSurface`, `lintRepeatedRequests`, the `store-without-change` lint, `validateSurfaces`, `resolveSurfaces`, `SurfaceSymbol`, `Symbols.Surfaces`.
  - hub: `verbReceives`, `step.Store`, `domain.BranchKindPar/BranchKindLoop`, `finding`/`domain.Finding`, `domain.Surface`, `FindSurfaces`, `archctx://surfaces`.
- **Placeholders:** none. Where a step depends on a value the implementer must read from the code (a flow edge number, a hub field type), the step says what to check and what must not change.
