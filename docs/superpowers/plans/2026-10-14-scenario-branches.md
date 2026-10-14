# Scenario Branches and Names Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `alt`/`else`/`opt` branch blocks (header + offside rule) and `scenario "<name>"` lines to the Craft DSL, carried as a step tree in the model, rendered natively in diagrams, supported by the formatter, LSP and tree-sitter, and released as v2.21.0.

**Architecture:** The hand-written parser gains contextual header keywords and a column-based offside rule. The block structure becomes `Block`/`Branch` nodes in the green tree, and `internal/analysis` projects them into `Scenario.Steps` (Action | Block). `internal/interaction` walks the tree, stamps each interaction with its branch path and keeps one call stack per branch. Views open and close native `alt`/`opt` blocks wherever the path changes.

**Tech Stack:** Go 1.25+ (repo `github.com/tcarcao/craft/v2`), tree-sitter (JS grammar in `../tree-sitter-craft`), PlantUML/Mermaid text output.

**Spec:** `docs/superpowers/specs/2026-10-14-scenario-branches-and-names-design.md`. Read it before any task.

**Repo rules** (`docs/AGENT.md`, `CLAUDE.md`):
- Red → green → refactor; `go vet ./... && go test ./...` green at every commit.
- Goldens are regenerated only with `-update`, and the diff is reviewed.
- Never touch `examples/*.craft`.
- Commit trailer: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

**Working branch:** `feat/scenario-branches` (already created; the spec is committed).

**Commands used everywhere**
- Full suite: `go vet ./... && go test ./...`
- Corpus goldens: `go test ./internal/corpustest/ -run TestHarnessA -update`
- Diagnostics goldens: `go test ./internal/corpustest/ -run TestHarnessC` (hand-authored `.diagnostics.json`; never `-update` them blindly)
- Diagram goldens: `go test ./internal/visualizer/ -run TestGolden -update`

---

## File map

| File | Responsibility | Tasks |
|---|---|---|
| `internal/lang/lang.go` | `BlockKind` constants (`alt`, `opt`) | 1 |
| `internal/model/model.go` | `Scenario.Name/Steps`, `Step`, `Block`, `Branch`, `Scenario.Actions()` | 1 |
| `internal/analysis/project.go` | builds `Steps` from the tree; scenario name; block IDs | 1, 2, 4 |
| every reader of `Scenario.Actions` (`analysis/resolve.go`, `interaction/interaction.go`, `lsp/server.go`, `sema/{lint,sema,validate}.go`, `visualizer/view/c4.go`, `cmd/craft/inspect.go`) | field → method | 1 |
| `internal/syntax/kind.go` | `SyntaxKindKwScenario/KwAlt/KwOpt/KwElse`, `SyntaxKindScenarioName`, `SyntaxKindBlock`, `SyntaxKindBranch` | 2, 3 |
| `internal/syntax/parser.go` | `scenario` line; block headers; offside rule; diagnostics | 2, 3 |
| `internal/syntax/ast.go` | typed views `ScenarioDecl.Name()`, `ScenarioDecl.Steps()`, `BlockDecl`, `BranchDecl` | 2, 3 |
| `internal/interaction/interaction.go` | tree walk, `BranchRef`, per-branch call stack | 5 |
| `internal/visualizer/view/sequence.go`, `plantuml/plantuml.go`, `mermaid/views.go` | native blocks, scenario-name frame titles | 6 |
| `internal/visualizer/view/flow.go` | branch label prefix | 7 |
| `internal/sema/lint.go` | `craft/lint/indistinguishable-scenarios` | 8 |
| `internal/lsp/formatter.go` | canonical block indentation, `scenario` spacing, tabs, refusal | 9 |
| `internal/lsp/server.go` (+ capabilities) | symbols, folding, hover, completion | 10 |
| `testdata/corpus/04_use_cases/*`, `testdata/broken/*` | fixtures | 2, 3, 4 |
| `docs/page/language/*`, `CHANGELOG.md`, `docs/GRAMMAR.md` §8 | docs | 11 |
| `../tree-sitter-craft/grammar.js`, queries, corpus | highlighting | 12 |
| re-arch-central journey file | dogfood | 13 |
| `svc-re-go-arch-context/internal/infrastructure/graphstore/craft_loader.go` | consumer update | 14 |

---

### Task 1: Step-tree model, with no behaviour change

**Files:**
- Modify: `internal/lang/lang.go`, `internal/model/model.go`, `internal/analysis/project.go:126-185`
- Modify (mechanical): every non-test reader of `Scenario.Actions`: `cmd/craft/inspect.go`, `internal/analysis/resolve.go`, `internal/interaction/interaction.go`, `internal/lsp/server.go`, `internal/sema/lint.go`, `internal/sema/sema.go`, `internal/sema/validate.go`, `internal/visualizer/view/c4.go`
- Modify: tests that build `model.Scenario{Actions: …}` literals (find with `grep -rn 'Actions:' --include=*_test.go internal pkg cmd`)
- Test: `internal/model/model_test.go` (create)

- [ ] **Step 1: Write the failing test**

```go
// internal/model/model_test.go
package model_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
)

func TestScenarioActions_FlattensStepsInSourceOrder(t *testing.T) {
	a := func(id string) model.Step { return model.Step{Action: &model.Action{ID: id}} }
	sc := model.Scenario{Steps: []model.Step{
		a("action_2"),
		{Block: &model.Block{ID: "block_3", Kind: lang.BlockAlt, Branches: []model.Branch{
			{Label: "ok", Steps: []model.Step{a("action_4")}},
			{Label: "404", Steps: []model.Step{
				a("action_5"),
				{Block: &model.Block{ID: "block_6", Kind: lang.BlockOpt, Branches: []model.Branch{
					{Label: "retry fails", Steps: []model.Step{a("action_7")}},
				}}},
			}},
		}}},
		a("action_8"),
	}}
	var got []string
	for _, act := range sc.Actions() {
		got = append(got, act.ID)
	}
	want := "action_2 action_4 action_5 action_7 action_8"
	if j := join(got); j != want {
		t.Errorf("Actions() = %q, want %q", j, want)
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += " "
		}
		out += x
	}
	return out
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/model/`
Expected: FAIL to compile (`model.Step`, `lang.BlockAlt` undefined).

- [ ] **Step 3: Add the vocabulary and the model**

In `internal/lang/lang.go` (next to the action and trigger kinds):

```go
// BlockKind is the kind of a branch block inside a scenario.
type BlockKind string

const (
	BlockAlt BlockKind = "alt" // exactly one of several branches runs
	BlockOpt BlockKind = "opt" // the single branch runs only under its condition
)
```

In `internal/model/model.go`, replace `Scenario` and add the new types. Keep the package's existing alias style for `lang` kinds; `ActionType`/`TriggerType` show the pattern.

```go
// Scenario is one trigger and the steps it sets off, in source order.
type Scenario struct {
	ID string `json:"id"`
	// Name is the label written in `scenario "…"` before the trigger; "" when
	// the scenario is unnamed.
	Name    string  `json:"name,omitempty"`
	Trigger Trigger `json:"trigger"`
	Steps   []Step  `json:"steps"`
}

// Step is exactly one of Action or Block.
type Step struct {
	Action *Action `json:"action,omitempty"`
	Block  *Block  `json:"block,omitempty"`
}

// Block is an `alt` (one of several branches) or `opt` (one conditional
// branch) inside a scenario.
type Block struct {
	ID       string         `json:"id"`
	Kind     lang.BlockKind `json:"kind"`
	Line     int            `json:"line,omitempty"`
	Column   int            `json:"column,omitempty"`
	Branches []Branch       `json:"branches"`
}

// Branch is one labelled path of a Block.
type Branch struct {
	Label string `json:"label"`
	Line  int    `json:"line,omitempty"`
	Steps []Step `json:"steps"`
}

// Actions returns every action of the scenario in source order: a depth-first
// walk of Steps.
func (s Scenario) Actions() []Action {
	var out []Action
	var walk func([]Step)
	walk = func(steps []Step) {
		for _, st := range steps {
			switch {
			case st.Action != nil:
				out = append(out, *st.Action)
			case st.Block != nil:
				for _, br := range st.Block.Branches {
					walk(br.Steps)
				}
			}
		}
	}
	walk(s.Steps)
	return out
}
```

If `pkg/craft/craftdoc.go` re-exports model types by alias, add aliases for `Step`, `Block`, `Branch` and `BlockKind` (plus `BlockAlt`/`BlockOpt` constants) next to the existing ones.

- [ ] **Step 4: Run the model test**

Run: `go test ./internal/model/`
Expected: PASS.

- [ ] **Step 5: Project `Steps` instead of `Actions`**

In `internal/analysis/project.go` `projectUseCases`, change the scenario construction:

```go
			outSc := model.Scenario{
				ID:      fmt.Sprintf("scenario_%d", counter),
				Trigger: projectTrigger(sc, li),
				Steps:   []model.Step{},
			}
			for _, action := range sc.Actions() {
				if action.IsIncomplete() {
					counter += 2
					continue
				}
				counter++
				a := projectAction(action, fmt.Sprintf("action_%d", counter), li)
				outSc.Steps = append(outSc.Steps, model.Step{Action: &a})
			}
```

- [ ] **Step 6: Migrate every reader**

Run `go build ./... 2>&1 | grep -n Actions`. In each reported site, replace `sc.Actions` (field) with `sc.Actions()` (method). Where code assigned to `.Actions` or ranged with an index it mutated (for example the resolver filling `From`/`To`), mutate through the tree instead. Add this helper to `internal/model/model.go` and use it:

```go
// EachAction calls fn with a pointer to every action of the scenario in
// source order, so callers can update actions in place.
func (s *Scenario) EachAction(fn func(*Action)) {
	var walk func([]Step)
	walk = func(steps []Step) {
		for _, st := range steps {
			switch {
			case st.Action != nil:
				fn(st.Action)
			case st.Block != nil:
				for i := range st.Block.Branches {
					walk(st.Block.Branches[i].Steps)
				}
			}
		}
	}
	walk(s.Steps)
}
```

Test literals: rewrite each `Actions: []craft.Action{X, Y}` as `Steps: []craft.Step{{Action: &X}, {Action: &Y}}`. Where the literal is inline, take the address of a local, or add a tiny helper in the test file: `func steps(as ...model.Action) []model.Step`.

- [ ] **Step 7: Regenerate the corpus goldens and review**

Run: `go test ./internal/corpustest/ -run TestHarnessA -update`, then `git diff --stat testdata/corpus`.
Review: every `.craftjson` change must be `"actions": [ {...} ]` → `"steps": [ {"action": {...}} ]` and nothing else. Spot-check two files by hand: `git diff -U2 testdata/corpus/04_use_cases/op_annotations.craftjson | head -60`.

- [ ] **Step 8: Run the full suite**

Run: `go vet ./... && go test ./...`
Expected: PASS. Diagram goldens must NOT change; if any do, a reader lost actions. Fix it, don't regenerate.

- [ ] **Step 9: Commit**

```bash
git add -A internal pkg cmd testdata
git commit -m "refactor(model)!: scenarios carry a step tree (Steps) instead of a flat Actions list

Scenario.Actions becomes a method that flattens Steps in source order.
No behaviour change; corpus goldens move from actions to steps.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: `scenario "<name>"` line

**Files:**
- Modify: `internal/syntax/kind.go` (add `SyntaxKindKwScenario`, `SyntaxKindScenarioName`), `internal/syntax/parser.go` (use-case body loop ~L845 and `parseScenario` ~L1057), `internal/syntax/ast.go` (`ScenarioDecl.Name()`), `internal/analysis/project.go`
- Create: `testdata/corpus/04_use_cases/named_scenarios.craft` + `.craftjson`, `testdata/broken/scenario_without_when.craft` + `.diagnostics.json`
- Test: `internal/syntax/parser_test.go` (or the file that holds `TestParseTree_*`)

- [ ] **Step 1: Write the corpus fixture (red)**

`testdata/corpus/04_use_cases/named_scenarios.craft`:

```craft
domain billing {
    billdatamgmt
}

use_case "Apply nMF billing events" {
    scenario "a new cycle opens"
    when billdatamgmt listens nmfbus.BillingCycleOpened
        billdatamgmt records the open cycle

    scenario "the cycle closes with an unpaid balance"
    when billdatamgmt listens nmfbus.BillingCycleClosed
        billdatamgmt flags the unpaid balance

    when billdatamgmt listens nmfbus.BillingCycleVoided
        billdatamgmt drops the cycle
}

use_case "scenario as a context name still works" {
    when Ops restarts the job
        scenario asks billdatamgmt to replay
}

domain tooling {
    scenario
}
```

Run `go test ./internal/corpustest/ -run TestHarnessA -update`, then open the new `.craftjson`. It must NOT yet contain `"name"` on scenarios; that is the red state. Revert the golden: `git checkout -- testdata/corpus/04_use_cases/named_scenarios.craftjson 2>/dev/null; rm -f testdata/corpus/04_use_cases/named_scenarios.craftjson`.

Write the parser unit test:

```go
func TestParseTree_ScenarioName(t *testing.T) {
	src := "use_case \"U\" {\n  scenario \"a new cycle opens\"\n  when b listens nmfbus.X\n    b records it\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	uc := syntax.AsFile(syntax.Root(g)).UseCases()[0]
	scs := uc.Scenarios()
	if len(scs) != 1 {
		t.Fatalf("got %d scenarios, want 1", len(scs))
	}
	if got := scs[0].Name(); got != "a new cycle opens" {
		t.Errorf("Name() = %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestParseTree_ScenarioName`
Expected: FAIL (`Name` undefined, or the `scenario` token is rejected as unexpected).

- [ ] **Step 3: Implement**

1. `kind.go`: add `SyntaxKindKwScenario` and `SyntaxKindScenarioName` (append at the end of the const block to keep existing numeric values stable; follow how `SyntaxKindKwWhen` is declared and stringified).
2. `parser.go`, use-case body loop: before the `when` check, add:

```go
		} else if tok.Type == lexer.TokenIdent && tok.Value == "scenario" && p.peekN(1).Type == lexer.TokenString {
			d := p.parseScenario()
			diags = append(diags, d...)
```

   Use the parser's existing lookahead helper; if it isn't called `peekN`, find it with `grep -n 'func (p \*Parser) peek' internal/syntax/parser.go`.
3. `parseScenario`: at the start, if the current token is `scenario` followed by a STRING, open `SyntaxKindScenarioName`, consume `scenario` as `SyntaxKindKwScenario` and the string as `SyntaxKindString`, and close the node. Skip newlines. Then if the next token is not `when`, emit:

```go
model.Diagnostic{Code: "craft/syntax/scenario-without-when", Severity: model.SeverityError,
	Message: "`scenario \"…\"` must be followed by a `when` line", Range: tokenRange(nameTok)}
```

   finish the scenario node and return. In the action loop of `parseScenario`, also stop at `scenario` + STRING (the next scenario).
4. `ast.go`:

```go
// Name returns the label of the `scenario "…"` line before the trigger, or "".
func (s ScenarioDecl) Name() string {
	n := s.node.ChildNode(SyntaxKindScenarioName)
	if n == nil {
		return ""
	}
	if tok := n.ChildToken(SyntaxKindString); tok != nil {
		return strings.Trim(tok.Text(), `"`)
	}
	return ""
}
```

   (If `ChildNode` doesn't exist, mirror how `UseCaseDecl.TagsBlock()` finds its child node.)
5. `project.go`: set `Name: sc.Name()` on `outSc`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/syntax/ -run TestParseTree_ScenarioName && go test ./internal/corpustest/ -run TestHarnessA -update`
Then inspect `named_scenarios.craftjson`: the first two scenarios carry `"name"`, the third has none, and the second use case parses `scenario asks …` as a sync action.

- [ ] **Step 5: Broken fixture**

`testdata/broken/scenario_without_when.craft`:

```craft
use_case "U" {
    scenario "orphan"
    billdatamgmt records the open cycle
}
```

Hand-author `testdata/broken/scenario_without_when.diagnostics.json` in the same JSON shape as the neighbouring files (copy `duplicate_actor.diagnostics.json` as the template). It holds one entry: code `craft/syntax/scenario-without-when`, severity error, and a range on line 2 (LSP 0-based line 1) covering `scenario`. Run `go test ./internal/corpustest/ -run TestHarnessC`; adjust only the range columns if the harness reports a mismatch, and verify they point at the `scenario` keyword.

- [ ] **Step 6: Full suite and commit**

Run: `go vet ./... && go test ./...` → PASS.

```bash
git add -A internal testdata
git commit -m "feat(syntax): named scenarios with a scenario \"…\" line before when

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Block headers and the offside rule (syntax tree and diagnostics)

**Files:**
- Modify: `internal/syntax/kind.go` (add `SyntaxKindKwAlt`, `SyntaxKindKwOpt`, `SyntaxKindKwElse`, `SyntaxKindBlock`, `SyntaxKindBranch`, `SyntaxKindBranchHeader`), `internal/syntax/parser.go` (`parseScenario` action loop), `internal/syntax/ast.go` (`ScenarioDecl.Steps()`, `StepDecl`, `BlockDecl`, `BranchDecl`)
- Create: `testdata/broken/{empty_branch,inconsistent_indent,orphan_else,missing_label}.craft` + `.diagnostics.json`
- Test: `internal/syntax/blocks_test.go` (create)

**Tree shape** (pin this; Task 4 depends on it):

```
Scenario
  Trigger
  Action                      (step)
  Block                       (step)
    Branch
      BranchHeader: KwAlt String
      Action …                (steps of the branch)
      Block …                 (nested)
    Branch
      BranchHeader: KwElse String
      Action …
  Action                      (step after the block)
```

An `opt` is `Block > Branch(BranchHeader: KwOpt String) …`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/syntax/blocks_test.go
package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

const blocksSrc = `use_case "U" {
    when CRON fires the schedule
        sp asks nmf for the range
        alt "nMF has a cycle"
            sp returns the cycle
        else "nMF answers 404"
            sp answers not-found
        else "nMF is unhealthy"
            sp retries once
            opt "the retry also fails"
                sp answers a server error
        sp logs the run
}
`

// outline renders the step tree of the first scenario compactly.
func outline(steps []syntax.StepDecl, depth int) string {
	var b strings.Builder
	for _, st := range steps {
		ind := strings.Repeat("  ", depth)
		if a, ok := st.Action(); ok {
			b.WriteString(ind + "action " + a.SubjectName() + "\n")
			continue
		}
		blk, _ := st.Block()
		b.WriteString(ind + string(blk.Kind()) + "\n")
		for _, br := range blk.Branches() {
			b.WriteString(ind + "  branch " + br.Label() + "\n")
			b.WriteString(outline(br.Steps(), depth+2))
		}
	}
	return b.String()
}

func TestBlocks_TreeShape(t *testing.T) {
	g, _, diags := syntax.Parse(blocksSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	want := `action sp
alt
  branch nMF has a cycle
    action sp
  branch nMF answers 404
    action sp
  branch nMF is unhealthy
    action sp
    opt
      branch the retry also fails
        action sp
action sp
`
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s\nwant:\n%s", got, want)
	}
	if n := len(sc.Actions()); n != 6 {
		t.Errorf("Actions() = %d, want 6 (flattened)", n)
	}
}

func TestBlocks_KeywordsStayUsableAsNames(t *testing.T) {
	src := "use_case \"U\" {\n  when Ops does it\n    alt asks opt to run\n    else records something\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := outline(sc.Steps(), 0); got != "action alt\naction else\n" {
		t.Errorf("tree:\n%s", got)
	}
}

func TestBlocks_TabsCountAsFourColumns(t *testing.T) {
	src := "use_case \"U\" {\n\twhen Ops does it\n\t\talt \"a\"\n\t\t\tx does a\n\t\telse \"b\"\n\t\t\tx does b\n\t\tx ends\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	want := "alt\n  branch a\n    action x\n  branch b\n    action x\naction x\n"
	if got := outline(sc.Steps(), 0); got != want {
		t.Errorf("tree:\n%s", got)
	}
}

func TestBlocks_Diagnostics(t *testing.T) {
	cases := map[string]string{
		"craft/syntax/empty-branch":        "use_case \"U\" {\n  when O does\n    alt \"a\"\n    x does it\n}",
		"craft/syntax/inconsistent-indent": "use_case \"U\" {\n  when O does\n    alt \"a\"\n        x does it\n      x does more\n}",
		"craft/syntax/orphan-else":         "use_case \"U\" {\n  when O does\n    opt \"a\"\n      x does it\n    else \"b\"\n      x does more\n}",
		"craft/syntax/missing-label":       "use_case \"U\" {\n  when O does\n    alt\n      x does it\n}",
	}
	for code, src := range cases {
		_, _, diags := syntax.Parse(src)
		found := false
		for _, d := range diags {
			if d.Code == code {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: not reported; got %v", code, diags)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestBlocks_`
Expected: FAIL to compile (`StepDecl`, `Steps`, `Block` undefined).

- [ ] **Step 3: Implement the parser**

In `parseScenario`, replace the action loop with a call to a new recursive function. The column rule uses `lexer.Token.Column` adjusted for tabs:

```go
// visualColumn is the column of tok counting a tab as advancing to the next
// multiple of 4. Only the offside rule uses it (spec §3.1.4).
func (p *Parser) visualColumn(tok lexer.Token) int {
	line := p.lineText(tok.Line) // add if absent: the source line of tok, from the lexer input
	col := 0
	for i, r := range []rune(line) {
		if i >= tok.Column-1 {
			break
		}
		if r == '\t' {
			col = (col/4 + 1) * 4
		} else {
			col++
		}
	}
	return col + 1
}

// isHeader reports whether the current line opens or continues a block:
// `alt|opt|else` followed by a STRING, or standing alone on its line
// (missing label). Anything else after the keyword is an ordinary action.
func (p *Parser) isHeader() (kw string, ok bool) {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || (tok.Value != "alt" && tok.Value != "opt" && tok.Value != "else") {
		return "", false
	}
	next := p.peekN(1)
	if next.Type == lexer.TokenString || next.Type == lexer.TokenNewline || next.Type == lexer.TokenEOF {
		return tok.Value, true
	}
	return "", false
}
```

`parseSteps(minCol int)` parses steps while the next significant token is not `when`, not `scenario`+STRING, not `}` and not EOF, and while `visualColumn(tok) > minCol`. For the scenario level, `minCol` is 0.
- **Action line:** `parseAction()` as today. If `bodyCol` was set by the first body line of the enclosing branch and this line's column is `< bodyCol`, report `craft/syntax/inconsistent-indent` on the line's first token and still parse it.
- **`alt`/`opt` header at column C:** `StartNode(SyntaxKindBlock)`, then `parseBranch(kind, C)` once. For `alt`, while the next line is an `else` header at exactly column C, call `parseBranch("else", C)` again. Then `FinishNode()`.
- **`else` header** reached in `parseSteps` (no open `alt` at its column, or under an `opt`): report `craft/syntax/orphan-else`, then parse it as a new `alt` Block whose first branch is the else.

`parseBranch(kw string, headerCol int)`:
1. `StartNode(SyntaxKindBranch)`, `StartNode(SyntaxKindBranchHeader)`.
2. Consume the keyword as `SyntaxKindKwAlt`/`KwOpt`/`KwElse`.
3. If a STRING follows, consume it; otherwise report `craft/syntax/missing-label`.
4. `FinishNode()` the header; skip newlines.
5. If the next token is not deeper than `headerCol` (or is `when`/`}`/EOF), report `craft/syntax/empty-branch` on the keyword.
6. Otherwise `parseSteps(headerCol)`.
7. `FinishNode()` the branch.

Diagnostic messages:
- `empty-branch`: "`alt "…"` has no steps: indent its steps under it"
- `inconsistent-indent`: "this line is indented less than the steps above it but more than its block header; align it with one of them"
- `orphan-else`: "`else` must sit at the same column as an `alt` above it (an `opt` has no else)"
- `missing-label`: "`alt`/`opt`/`else` needs a quoted label, e.g. `alt \"payment fails\"`"

All are `SeverityError`, with `Range: tokenRange(tok)`.

Comments and blank lines between steps keep today's handling (the lexer delivers them as trivia; check `parseAction` for how NEWLINE runs are skipped and reuse that).

- [ ] **Step 4: Implement the typed views (`ast.go`)**

```go
// StepDecl is one step of a scenario or branch: an action or a block.
type StepDecl struct{ node SyntaxNode }

func (s StepDecl) Action() (ActionDecl, bool) {
	if s.node.Kind() == SyntaxKindAction {
		return ActionDecl{node: s.node}, true
	}
	return ActionDecl{}, false
}

func (s StepDecl) Block() (BlockDecl, bool) {
	if s.node.Kind() == SyntaxKindBlock {
		return BlockDecl{node: s.node}, true
	}
	return BlockDecl{}, false
}

// steps lists the Action and Block children of n in source order.
func steps(n SyntaxNode) []StepDecl {
	var out []StepDecl
	for _, c := range n.ChildNodes() { // use the existing all-children accessor
		if c.Kind() == SyntaxKindAction || c.Kind() == SyntaxKindBlock {
			out = append(out, StepDecl{node: c})
		}
	}
	return out
}

// Steps returns the scenario's top-level steps in source order.
func (s ScenarioDecl) Steps() []StepDecl { return steps(s.node) }

type BlockDecl struct{ node SyntaxNode }

// Kind is alt or opt (a block opened by an orphan else is alt).
func (b BlockDecl) Kind() lang.BlockKind {
	brs := b.Branches()
	if len(brs) > 0 && brs[0].Keyword() == "opt" {
		return lang.BlockOpt
	}
	return lang.BlockAlt
}

func (b BlockDecl) Branches() []BranchDecl {
	var out []BranchDecl
	for _, n := range b.node.ChildNodes(SyntaxKindBranch) {
		out = append(out, BranchDecl{node: n})
	}
	return out
}

func (b BlockDecl) Node() SyntaxNode { return b.node }

type BranchDecl struct{ node SyntaxNode }

func (br BranchDecl) header() SyntaxNode { return br.node.ChildNode(SyntaxKindBranchHeader) }

// Keyword is "alt", "opt" or "else".
func (br BranchDecl) Keyword() string {
	h := br.header()
	for _, k := range []SyntaxKind{SyntaxKindKwAlt, SyntaxKindKwOpt, SyntaxKindKwElse} {
		if t := h.ChildToken(k); t != nil {
			return t.Text()
		}
	}
	return ""
}

func (br BranchDecl) Label() string {
	if t := br.header().ChildToken(SyntaxKindString); t != nil {
		return strings.Trim(t.Text(), `"`)
	}
	return ""
}

// HeaderToken is the keyword token, for positions.
func (br BranchDecl) HeaderToken() *SyntaxToken {
	h := br.header()
	for _, k := range []SyntaxKind{SyntaxKindKwAlt, SyntaxKindKwOpt, SyntaxKindKwElse} {
		if t := h.ChildToken(k); t != nil {
			return t
		}
	}
	return nil
}

func (br BranchDecl) Steps() []StepDecl { return steps(br.node) }
```

Keep `ScenarioDecl.Actions()` working as the flattened list (depth-first over `Steps()`), because many callers rely on it (lint, LSP, validate).

- [ ] **Step 5: Run the block tests**

Run: `go test ./internal/syntax/ -run 'TestBlocks_|TestParseTree'`
Expected: PASS.

- [ ] **Step 6: Broken fixtures**

Create one `.craft` per diagnostic from the four sources in `TestBlocks_Diagnostics` (one file each, named after the code), plus a hand-authored `.diagnostics.json` per file with the exact code, severity and range on the reported token. Run `go test ./internal/corpustest/ -run TestHarnessC` and fix ranges until it passes, checking each range by eye against the source.

- [ ] **Step 7: Full suite and commit**

Run: `go vet ./... && go test ./...`. Corpus goldens must not change yet: projection still ignores blocks, and `ScenarioDecl.Actions()` flattens them.

```bash
git add -A internal testdata
git commit -m "feat(syntax): alt/opt/else branch blocks with an offside rule

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Project blocks into `Scenario.Steps`

**Files:**
- Modify: `internal/analysis/project.go` (`projectUseCases`)
- Create: `testdata/corpus/04_use_cases/{alt_else,opt,nested_blocks,branch_tabs,branches_full}.craft` + goldens

- [ ] **Step 1: Write the fixtures (red)**

`branches_full.craft` is the spec §3 example with a domain declaring `subscriptionplan`, `nmf_monetization`, an `actor system CRON` and an `account` domain. `alt_else` has a single alt with 3 branches. `opt` has a single opt. `nested_blocks` has opt-in-alt-in-alt. `branch_tabs` is the same as `alt_else` but tab-indented.

Run `go test ./internal/corpustest/ -run TestHarnessA -update`. The red state: the new goldens show a FLAT `steps` list (no `block` entries). Delete those five new goldens again before implementing.

- [ ] **Step 2: Implement recursive projection**

Replace the action loop in `projectUseCases` with a recursive projector sharing `counter`:

```go
			outSc.Name = sc.Name()
			outSc.Steps = projectSteps(sc.Steps(), &counter, li)
```

```go
// projectSteps projects syntax steps in source order. Blocks take a number
// from the same counter as actions (block_N), so IDs stay in source order.
func projectSteps(steps []syntax.StepDecl, counter *int, li green.LineIndex) []model.Step {
	out := []model.Step{}
	for _, st := range steps {
		if a, ok := st.Action(); ok {
			if a.IsIncomplete() {
				*counter += 2
				continue
			}
			*counter++
			act := projectAction(a, fmt.Sprintf("action_%d", *counter), li)
			out = append(out, model.Step{Action: &act})
			continue
		}
		blk, _ := st.Block()
		*counter++
		mb := &model.Block{ID: fmt.Sprintf("block_%d", *counter), Kind: blk.Kind(), Branches: []model.Branch{}}
		for i, br := range blk.Branches() {
			mbr := model.Branch{Label: br.Label()}
			if tok := br.HeaderToken(); tok != nil {
				line, col := li.LineCol(tok.Offset())
				mbr.Line = line
				if i == 0 {
					mb.Line, mb.Column = line, col
				}
			}
			mbr.Steps = projectSteps(br.Steps(), counter, li)
			mb.Branches = append(mb.Branches, mbr)
		}
		out = append(out, model.Step{Block: mb})
	}
	return out
}
```

(Check that `li.LineCol` returns a 1-based column in the unit the model uses elsewhere, as `Action.Column` does; mirror `projectAction`.)

- [ ] **Step 3: Regenerate and review**

Run `go test ./internal/corpustest/ -run TestHarnessA -update`. Read `branches_full.craftjson` and confirm:
- the block IDs and action IDs increase in source order;
- 3 branches with the right labels;
- the nested opt sits inside the third branch;
- the last action comes after the block;
- `"name": "daily snapshot"`.

`branch_tabs.craftjson` must match `alt_else.craftjson` structurally (diff them after stripping line/column). Existing goldens must not change.

- [ ] **Step 4: Full suite and commit**

```bash
go vet ./... && go test ./...
git add -A internal testdata
git commit -m "feat(model): project alt/opt blocks into the scenario step tree

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Interaction — branch paths and per-branch call stack

**Files:**
- Modify: `internal/interaction/interaction.go`
- Test: `internal/interaction/interaction_test.go`

- [ ] **Step 1: Write the failing tests** (append to `interaction_test.go`; reuse `derive`, `summary` and `structure`)

```go
func branchSummary(its []interaction.Interaction) string {
	var lines []string
	for _, it := range its {
		var path []string
		for _, b := range it.Branch {
			path = append(path, fmt.Sprintf("%s:%s#%d", b.Kind, b.Label, b.Index))
		}
		to := it.To.Name
		if it.To.IsZero() {
			to = "·"
		}
		lines = append(lines, fmt.Sprintf("%d %s %s->%s {%s}", it.Step, it.Kind, it.From.Name, to, strings.Join(path, " > ")))
	}
	return strings.Join(lines, "\n")
}

func TestDerive_BranchPathsAndPerBranchCallStack(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
        else "the card is declined"
            Billing asks Bank for a second opinion
            opt "the bank also declines"
                Bank returns the refusal
            Billing returns the refusal
        Ordering returns the confirmation
}
`})
	want := `0 trigger Customer->Ordering {}
1 request Ordering->Billing {}
2 response Billing->Ordering {alt:the card is accepted#0}
3 request Billing->Bank {alt:the card is declined#1}
4 response Bank->Billing {alt:the card is declined#1 > opt:the bank also declines#0}
5 response Billing->Ordering {alt:the card is declined#1}
6 response Ordering->Customer {}`
	if got := branchSummary(its); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// The trigger reaches the first action in source order, even inside a block.
func TestDerive_TriggerTargetsFirstActionInsideABlock(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Checkout" {
    when Customer places an order
        alt "with a voucher"
            Billing applies the voucher
        else "without"
            Ordering charges the card
}
`})
	if its[0].Kind != interaction.Trigger || its[0].To.Name != "Billing" {
		t.Errorf("trigger = %+v", its[0])
	}
}
```

Note line 6: after the block closes, the stack is restored to its state at the `alt` (Customer→Ordering, Ordering→Billing). `Ordering returns` then answers the most recent frame whose callee is Ordering, which is Customer. The steps inside the branches never leak.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/interaction/ -run 'Branch|InsideABlock'`
Expected: FAIL (`it.Branch` undefined).

- [ ] **Step 3: Implement**

1. Add the type and field:

```go
// BranchRef locates an interaction inside one branch of a block.
type BranchRef struct {
	Block string
	Kind  lang.BlockKind
	Label string
	Index int
}
```

   In `Interaction`, add `Branch []BranchRef` (path from the outermost block inward; nil outside blocks).
2. In `deriveScenario`, the trigger's first action is `sc.Actions()[0]`.
3. Replace the action loop with a recursive walk:

```go
	step := 0
	var walk func(steps []model.Step, path []BranchRef)
	walk = func(steps []model.Step, path []BranchRef) {
		for _, st := range steps {
			if st.Block != nil {
				saved := append([]frame(nil), callStack...)
				for i, br := range st.Block.Branches {
					callStack = append([]frame(nil), saved...)
					p := append(append([]BranchRef(nil), path...), BranchRef{Block: st.Block.ID, Kind: st.Block.Kind, Label: br.Label, Index: i})
					walk(br.Steps, p)
				}
				callStack = saved
				continue
			}
			step++
			a := *st.Action
			// … existing per-action body, unchanged, using `step` for it.Step
			// and setting it.Branch = path before appending.
		}
	}
	walk(sc.Steps, nil)
```

   Move the existing per-action `switch` into the walk body without changing its logic. Note: an `opt` body continues from the current stack (`saved` is the current stack) and is restored afterwards, which matches the spec.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/interaction/` → PASS (old tests too). Run `go vet ./... && go test ./...`: diagram goldens must not change (the examples have no blocks).

- [ ] **Step 5: Commit**

```bash
git add internal/interaction
git commit -m "feat(interaction): branch paths and a call stack per branch

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Sequence view — native alt/opt blocks and scenario-name frames

**Files:**
- Modify: `internal/visualizer/view/sequence.go`, `internal/visualizer/plantuml/plantuml.go` (`Sequence`), `internal/visualizer/mermaid/views.go` (`Sequence`)
- Test: `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests** (append; helpers `sequenceSource`, `requireLine` and `forbidLine` exist)

```go
const branchesSrc = `actor user Customer
actor system Bank
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    scenario "online order"
    when Customer places an order
        Ordering asks Billing to charge the card
        alt "the card is accepted"
            Billing returns the receipt
        else "the card is declined"
            Billing asks Bank for a second opinion
            opt "the bank also declines"
                Bank returns the refusal
        Ordering returns the confirmation
    scenario "phone order"
    when Customer phones in an order
        Ordering asks Billing to charge the card
}
`

func TestSequence_BranchesAreNativeBlocks(t *testing.T) {
	puml, mmd := sequenceSource(t, branchesSrc)
	for _, want := range []string{
		"group 1: online order",
		"alt the card is accepted",
		"else the card is declined",
		"opt the bank also declines",
		"group 2: phone order",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	// alt/else/opt open and close in order: the opt closes before the alt.
	i := strings.Index(puml, "opt the bank also declines")
	rest := puml[i:]
	if !strings.Contains(rest, "end\nend\n") {
		t.Errorf("plantuml: opt and alt not both closed after the opt:\n%s", puml)
	}
	for _, want := range []string{"alt the card is accepted", "else the card is declined", "opt the bank also declines"} {
		requireLine(t, "mermaid", mmd, want)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/visualizer/ -run TestSequence_BranchesAreNativeBlocks` → FAIL.

- [ ] **Step 3: Implement**

1. `view.Message` gains `Branch []BranchRef`, where `BranchRef` is a view-local copy `{Block, Kind string; Label string; Index int}`; views don't import interaction types into writers. Fill it from `it.Branch`.
2. Frame title: in `BuildSequence`, use `sc.Name` when non-empty: `fmt.Sprintf("%d: %s", i+1, name)`. Otherwise keep the trigger description.
3. Shared writer helper in package `view`, so both writers use one algorithm:

```go
// BlockOps returns the block lines to emit between two consecutive messages:
// closes for blocks prev is in and next is not (innermost first), then opens
// or else-switches for next. Each op is {Kind: "open"|"else"|"close", Block, Label}.
func BlockOps(prev, next []BranchRef) []BlockOp
```

   Algorithm: find the longest common prefix by `Block` ID.
   - At the first differing depth, if both have an entry for the **same** `Block` with a different `Index`, it is an `else` switch: close everything deeper in `prev` (innermost first), emit `else <label>`, then open everything deeper in `next`.
   - Otherwise close every `prev` entry past the common prefix (innermost first) and open every `next` entry past it.
   - At the end of a section (and before a scenario frame changes), close everything open.

   Unit-test `BlockOps` directly in `internal/visualizer/view/language_test.go` (or a new `blocks_test.go`) with 4 cases: enter, else-switch, nested exit, full exit.
4. PlantUML writes `open` → `alt <label>` / `opt <label>`, `else` → `else <label>`, `close` → `end`. Mermaid writes the same keywords, indented 4 spaces. Emit scenario `group`/`rect` before block ops and close the blocks before `end`ing a group.
5. **Empty branch:** a branch with no message still needs its `alt`/`else` line. Generate block ops from the model's branch structure for the section, not only from messages. Simplest correct approach: the view also receives each scenario's step tree (`doc.UseCases[].Scenarios[].Steps`). It emits a zero-message placeholder `Message{Kind: MessageNone, Branch: path}` for each branch that produced no message, and writers skip `MessageNone` except for block ops. Add a test case with an `else` branch containing only an unresolved internal action if needed; otherwise a branch whose steps all draw nothing.

- [ ] **Step 4: Run tests, then regenerate the goldens**

Run: `go test ./internal/visualizer/ -run 'TestSequence_|TestBlockOps'` → PASS. Then `go test ./internal/visualizer/ -run TestGolden -update`. The examples have no blocks and no names, so expect **no** golden change. If any changes, investigate.

- [ ] **Step 5: Commit**

```bash
go vet ./... && go test ./...
git add internal/visualizer
git commit -m "feat(sequence): draw alt/opt branches as native blocks; frames use scenario names

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Flow view — branch label prefix

**Files:** Modify `internal/visualizer/view/flow.go`; Test `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Failing test**

```go
func TestFlow_EdgesInsideABranchCarryTheBranchLabel(t *testing.T) {
	doc := resolvedDoc(t, branchesSrc)
	out, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[the card is declined] for a second opinion") {
		t.Errorf("flow edge lacks branch label:\n%s", out)
	}
}
```

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** In `BuildFlow`'s `add`, when `len(it.Branch) > 0`, prefix the label with `"[" + it.Branch[len(it.Branch)-1].Label + "] "`. The number prefix is added by the writers, so the result reads `5. [label] text`.
- [ ] **Step 4: Run** → PASS; `go vet ./... && go test ./...`; no golden change expected.
- [ ] **Step 5: Commit** `feat(flow): prefix edges inside a branch with the branch label`.

---

### Task 8: Lint `craft/lint/indistinguishable-scenarios`

**Files:** Modify `internal/sema/lint.go` (new `lintIndistinguishableScenarios`, registered in `LintWorkspace`); Test: the sema lint test file (`grep -ln 'lintUnusedActors\|unused-actor' internal/sema/*_test.go`); Create `testdata/broken/indistinguishable_scenarios.craft` + `.diagnostics.json`

- [ ] **Step 1: Failing fixture and test.**

`testdata/broken/indistinguishable_scenarios.craft`:

```craft
domain d {
    sp
}
use_case "U" {
    when CRON fires the schedule
        sp asks d for a range
    when CRON fires the schedule
        sp answers not-found
    scenario "named twin"
    when CRON fires the schedule
        sp retries
}
```

The expected diagnostics are warnings on the 2nd and 3rd `when` lines: each repeats an earlier trigger, and in each pair at least one scenario is unnamed. Message: `scenario repeats the trigger "when CRON fires the schedule"; use alt "…" for different outcomes, or scenario "…" names for different ways in`.

Also add a negative unit test: two scenarios with identical triggers that are BOTH named produce no diagnostic.
- [ ] **Step 2: Run** `go test ./internal/corpustest/ -run TestHarnessC` and the unit test → FAIL.
- [ ] **Step 3: Implement.** Walk each use case's scenarios in the per-file trees. Key by the normalised trigger text (`TriggerDecl.SourceText()`, whitespace collapsed). For each scenario after the first with an already-seen key, warn when it or the earlier one has an empty `Name()`. The range is the `when` token. Severity: warning.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lint): indistinguishable-scenarios warns about repeated unnamed triggers`.

---

### Task 9: Formatter

**Files:** Modify `internal/lsp/formatter.go` (`writeTokens`, `bailsFormatting`); Test: `internal/lsp/formatter_test.go` (existing; `grep -ln FormatDocument internal/lsp/*_test.go`)

- [ ] **Step 1: Failing tests**

```go
func TestFormat_BranchesCanonical(t *testing.T) {
	in := "use_case \"U\" {\n  scenario \"a\"\n  when O does it\n      x asks y for z\n      alt \"one\"\n          x does a\n      else \"two\"\n          x does b\n          opt \"three\"\n              x does c\n      x ends\n  scenario \"b\"\n  when O does other\n      x does d\n}\n"
	want := "use_case \"U\" {\n    scenario \"a\"\n    when O does it\n        x asks y for z\n        alt \"one\"\n            x does a\n        else \"two\"\n            x does b\n            opt \"three\"\n                x does c\n        x ends\n\n    scenario \"b\"\n    when O does other\n        x does d\n}\n"
	got := FormatDocument(in)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
}

func TestFormat_TabsBecomeSpacesAndMembershipIsKept(t *testing.T) {
	in := "use_case \"U\" {\n\twhen O does it\n\t\talt \"a\"\n\t\t\tx does a\n\t\tx ends\n}\n"
	got := FormatDocument(in)
	if strings.Contains(got, "\t") {
		t.Errorf("tabs left:\n%s", got)
	}
	// membership: re-parse and compare the step outline before and after
	assertSameOutline(t, in, got)
}

func TestFormat_RefusesOnIndentDiagnostics(t *testing.T) {
	in := "use_case \"U\" {\n  when O does\n    alt \"a\"\n        x does it\n      x does more\n}\n"
	if _, d := FormatDocumentChecked(in); d == nil {
		t.Error("expected the formatter to refuse a file with inconsistent indentation")
	}
}
```

`assertSameOutline` parses both strings with `syntax.Parse` and compares an outline built like `outline` in Task 3 (copy that helper into the lsp test package).

Also add: every new `testdata/corpus/04_use_cases/*` file from Tasks 2–4 formats idempotently and keeps its outline. Loop over the files.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** In `writeTokens`, compute each action and branch-header line's indent from its tree depth:
  - `when` and `scenario` at the scenario indent;
  - actions at +1 level;
  - each `Block > Branch` adds +1 for its body;
  - `else` and `alt`/`opt` headers sit at their block's level.

  Insert one blank line before a `scenario` line unless it is the first item in the use-case body. Tabs vanish naturally once indentation is regenerated (verify that no tab survives inside leading whitespace). In `bailsFormatting`, return true for the four `craft/syntax/*` codes from Task 3 and `scenario-without-when`.
- [ ] **Step 4: Run** → PASS; full suite (the existing formatter goldens must not change).
- [ ] **Step 5: Commit** `feat(fmt): canonical indentation for branch blocks and scenario names`.

---

### Task 10: LSP — symbols, folding, hover, completion

**Files:** Modify `internal/lsp/server.go` (`DocumentSymbol` ~L762, `Hover`, `Completion`), the capabilities builder (`grep -n 'ServerCapabilities' internal/lsp/*.go`), and a new `FoldingRange` handler if absent; Test: `internal/lsp/usecase_test.go` / `server_test.go`

- [ ] **Step 1: Failing tests**
  - `DocumentSymbol` on `branchesSrc`: the use-case symbol has children `online order`, then `alt: the card is accepted | the card is declined` with child `opt: the bank also declines`, then `phone order`. Assert names and nesting.
  - `FoldingRange` returns a range spanning each block (header line → last body line).
  - `Hover` on `alt` returns text containing `alt — 2 branches`.
  - `Completion` at the start of an empty line inside a scenario offers items labelled `alt`, `opt`, `else`, `scenario` with snippet text `alt "${1:condition}"`.
  - The `initialize` response advertises `foldingRangeProvider` (rule §3 of AGENT.md: only together with its handler).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** with `syntax` typed views (`ScenarioDecl.Name()`, `Steps()`, `BlockDecl`, `BranchDecl`). Positions come from `HeaderToken()` and the line index; end lines come from the node's last token.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): outline, folding, hover and completion for branches and scenario names`.

---

### Task 11: Documentation and changelog

**Files:** `docs/GRAMMAR.md` §8 (add the productions), `docs/page/language/` (the use-case page: `grep -rl 'use_case' docs/page/language`), `CHANGELOG.md`, `docs/DIAGNOSTICS.md` (new codes)

- [ ] **Step 1:** In GRAMMAR.md §8, add:

```ebnf
scenario      = scenario_name? 'when' trigger NL+ step* ;
scenario_name = 'scenario' STRING NL+ ;
step          = action | block ;
block         = alt_block | opt_block ;
alt_block     = 'alt' STRING NL+ body ('else' STRING NL+ body)* ;
opt_block     = 'opt' STRING NL+ body ;
body          = step+        (* offside rule: every line deeper than its header's column *)
```

  Add a short prose paragraph on the offside rule and the contextual keywords.
- [ ] **Step 2:** On the language page, add a "Branches and named scenarios" section with the spec §3 example and a rendered PlantUML snippet (copy the output from Task 6's test).
- [ ] **Step 3:** In CHANGELOG.md, add a new `## [Unreleased]` at the top:
  - `### Breaking`: `Scenario.Actions` field → `Scenario.Steps` tree + `Actions()` method; JSON `actions` → `steps`; migration one-liner.
  - `### Added`: `alt`/`else`/`opt`, `scenario "…"`, the lint, LSP features.
  - `### Changed — diagrams`: native blocks, scenario-name frames, the flow label prefix.
- [ ] **Step 4:** In DIAGNOSTICS.md, add the 5 syntax codes and the lint code, with one-line meanings.
- [ ] **Step 5: Commit** `docs: branches and named scenarios`. (The user approved the design, which covers these docs.)

---

### Task 12: tree-sitter highlighting

**Repo:** `~/projects/github/tcarcao/tree-sitter-craft` (branch `feat/scenario-branches` off `main`)

**Files:** `grammar.js` (`scenario` ~L320, `action` ~L372), `queries/highlights.scm`, `test/corpus/*.txt` (add `branches.txt`)

- [ ] **Step 1: Failing corpus test** `test/corpus/branches.txt`. Input: the spec §3 example. Expected tree: the `scenario_name`, `branch_header` (with `keyword` and `string`) and action nodes appear as **siblings** inside the scenario, because there is no nesting.
- [ ] **Step 2:** Run `npx tree-sitter test` (or the repo's `pnpm test`) → FAIL.
- [ ] **Step 3: Implement:**

```js
scenario_name: $ => seq('scenario', $.string),
branch_header: $ => seq(choice('alt', 'opt', 'else'), optional($.string)),
```

  - Allow `scenario_name` before `when_clause` in `scenario`.
  - Allow `branch_header` wherever `action` is allowed (`scenario_continuation` and the action repeat).
  - Use `prec` so that `alt asks x to …` still parses as an action: give `branch_header` lower precedence, and require its STRING when it isn't followed by a newline.
  - Highlights: `["scenario" "alt" "opt" "else"] @keyword`.
- [ ] **Step 4:** Run the tests → PASS; regenerate (`npx tree-sitter generate`), build the wasm the way the repo does (`grep -n wasm package.json`), and bump `CORPUS_VERSION` wherever the craft repo or extension reads it (`grep -rn CORPUS_VERSION ../craft ../craft-vscode-extension --include=*.go --include=*.ts --include=*.json`).
- [ ] **Step 5:** In craft `internal/corpustest` (the parser-diff/tree-sitter-compat test, `grep -rln tree-sitter internal/corpustest .github/workflows`), record the block nesting as a known divergence for the new corpus files, following the existing divergence-note pattern.
- [ ] **Step 6: Commit** in both repos. Push both branches with `git push -u origin feat/scenario-branches` (GitHub, authenticated via gh).

---

### Task 13: Dogfood on re-arch-central

**Repo:** `~/Downloads/domain-hub/re-arch-central` (new branch `feature/branches-pilot` off `feature/journey-assessment-2026-10-14`)

- [ ] **Step 1:** Build craft from the branch (`go build -o /tmp/craft-branches ./cmd/craft`). Run `/tmp/craft-branches validate docs/topology.craft docs/actors.craft docs/context_map.craft 'docs/journeys/*/*.craft'`. Expect 0 errors, and warnings = the previous 107 plus N new `indistinguishable-scenarios`. Record N; it should be in the low hundreds, matching the 217 files.
- [ ] **Step 2:** Rewrite `docs/journeys/subscriptions/subscriber-billing-cycle-resolution.craft` so that its three CRON scenarios become one scenario with `alt "nMF has a cycle…" / else "nMF answers 404" / else "nMF is unhealthy"` after the nMF call. Move the comment content into labels where it is a condition. Keep the evidence cites as trailing comments.
- [ ] **Step 3:** Validate the file (0 errors, no new lint warning for it). Render the sequence view in puml and mmd into `~/Downloads/domain-hub/journey-assessment/_render-branches/` and check that the `alt` box contains three branches.
- [ ] **Step 4:** `make fmt` must leave the file stable (run it twice, then `git diff` is empty the second time).
- [ ] **Step 5:** Commit on `feature/branches-pilot`; push.

---

### Task 14: Release v2.21.0 and update the arch-context loader

- [ ] **Step 1 (craft):** Rename `[Unreleased]` → `[2.21.0] — <date>` in CHANGELOG.md. `go vet ./... && go test ./...`. Fast-forward `main` to `feat/scenario-branches` (`git switch main && git merge --ff-only feat/scenario-branches`), then `git push origin main`. Create an **annotated** tag: `git tag -a v2.21.0 -m "v2.21.0"` and `git push origin v2.21.0`. Watch the release with `gh run watch <id> --repo tcarcao/craft --exit-status`. Then `brew upgrade tcarcao/craft/craft` → `craft --version` prints `2.21.0`.
- [ ] **Step 2 (arch-context):** In `~/Downloads/domain-hub/svc-re-go-arch-context`, branch `feature/craft-v2.21-branches` off `main`. Run `go get github.com/tcarcao/craft/v2@v2.21.0 && make mod`.
- [ ] **Step 3: Failing test** in `internal/infrastructure/graphstore/craft_loader_test.go` (follow the existing loader tests): a craft source with a named scenario and an `alt`. Assert that the loaded use case carries the scenario label `"online order"` and that the step inside the `else` branch carries `BranchLabel == "the card is declined"`.
- [ ] **Step 4: Implement:**
  - L293: `for _, a := range sc.Actions()`.
  - Add `ScenarioName string` (or reuse `ScenarioLabel`, set to `sc.Name` when non-empty) and `BranchLabel string` to the hub step struct.
  - Compute `BranchLabel` with a `Steps` walk that maps action ID → innermost branch label.
- [ ] **Step 5:** `make lint && make test && make validate-schema` (repo AGENTS.md) → PASS. Commit, push, open an MR with `glab mr create` (the user approves MR creation in the session).
- [ ] **Step 6:** Update `docs/journey-assessment/PLAN.md` in re-arch-central (Phase 3: G2 and G3 done) and push.

---

## Self-review notes
- **Spec coverage:**
  - §3 syntax: Tasks 2 and 3.
  - §3.2 diagnostics: Tasks 2 and 3.
  - §3.3 lint: Task 8.
  - §4 model: Tasks 1 and 4.
  - §5 semantics: Task 5.
  - §6.1 formatter: Task 9.
  - §6.2–6.5 views: Tasks 6 and 7 (architecture and C4 unchanged; verified by the no-golden-change checks).
  - §6.6 LSP: Task 10.
  - §6.7 tree-sitter: Task 12.
  - §7 consumer: Task 14.
  - §8 versioning: Task 14.
  - §9 testing: covered per task.
  - §9 dogfood: Task 13.
- **Type names used consistently:**
  - model and lang: `lang.BlockKind`, `lang.BlockAlt`/`BlockOpt`, `model.Step`/`Block`/`Branch`, `Scenario.Actions()`, `Scenario.EachAction`.
  - syntax views: `ScenarioDecl.Name()`, `ScenarioDecl.Steps()`, `StepDecl.Action()`/`Block()`, `BlockDecl.Kind()`/`Branches()`, `BranchDecl.Keyword()`/`Label()`/`HeaderToken()`/`Steps()`.
  - interaction and view: `interaction.BranchRef{Block, Kind, Label, Index}`, `Interaction.Branch`, `view.BlockOps`.
