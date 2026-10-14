# Journey Semantics 2 (v2.24.0, v2.25.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task header says which model it needs: **cheap** (mechanical: the code is given, copy it and make the tests pass) or **strong** (judgment: the code is sketched and the tests pin the behaviour, but the implementer must make design calls).

**Goal:** Part A (v2.24.0) adds `listens` rosters, scope in sequence titles and `duplicate-scenario-body`, the actor kinds `scheduler`/`tool`/`infra`, `returns stream`, operation lists and `notifies one of`, and the `ADDR`/`SQL`/`inferred` markers. Part B (v2.25.0) adds `//@` source cites, `includes`/`continues` links, and the journey spine view (`craft generate --journey`, `BuildJourney`). Each part carries its constructs through formatter, LSP, tree-sitter, the VSCode extension and the arch-context hub, and ends with a dogfood and a release-prep task.

**Architecture:** Every new word is a contextual keyword of the hand-written parser (spec §3.1, §12.1). Lists in an event slot are wrapped in a `SyntaxKindEventList` node only when they hold a comma, so a single-ref trigger keeps its v2.23 tree. Operation items are not tree nodes: `lang.SplitOperationItems` splits the bracket text, so `Operation` stays byte-identical. `inferred` is a bracket-head keyword token; `stream` a keyword after `returns`. Cites stay line comments and are attached by line in projection. Links are a fourth step kind (`Step.Link`). Lints that need the resolved model run in a new `sema.LintModel`, after resolution. The spine is a new package `internal/journey`, rendered by the existing sequence view with a shared stage.

**Tech Stack:** Go 1.25+ (`github.com/tcarcao/craft/v2`), tree-sitter (`~/projects/github/tcarcao/tree-sitter-craft`), TypeScript (`~/projects/github/tcarcao/craft-vscode-extension`), the hub (`~/Downloads/domain-hub/svc-re-go-arch-context`, hexagonal Go).

**Spec:** `docs/superpowers/specs/2026-10-14-journey-semantics-2-design.md`. Read it before any task. Section numbers below (§4.5 …) refer to it. The v2.23 spec and plan (`…/2026-10-14-journey-semantics-design.md`, `…/plans/2026-10-14-journey-semantics.md`) describe the code this plan builds on.

**Prerequisite:** the v2.23 plan is complete: v2.23.0 is tagged, tree-sitter-craft 0.3.0 is on npm, the extension pins `^0.3.0` (release 0.0.11), and the hub is on craft v2.23.0 (its Tasks 27–28). Part B starts after the coordinator tags v2.24.0.

**Repo rules** (`docs/AGENT.md`, `CLAUDE.md`):
- Red → green → refactor; `go vet ./... && go test ./...` green at every commit.
- Goldens are regenerated only with `-update`, and the diff is reviewed. Existing `.craftjson`, diagnostics and diagram goldens must not change unless a step says so.
- Never touch `examples/*.craft`.
- Never stage `tools/`, `internal/lsp/zz_repro_test.go` or `docs/superpowers/specs/2026-07-15-usecase-identity-tags-and-edge-verb-hardening.md` (untracked on the branch). Stage by directory (`git add internal pkg cmd testdata docs/…`) or exact path, never `git add -A` at the repo root.
- An LSP handler and its capability land in the same commit (`docs/AGENT.md` §3), with an assertion in `internal/lsp/capabilities_test.go`.
- Commit trailer: `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

**Working branch:** the coordinator names it; this plan assumes `feat/journey-semantics-2`, cut from `main` at v2.23.0. Part B continues on it after the v2.24.0 tag.

**Commands used everywhere**
- Full suite: `go vet ./... && go test ./...`
- Corpus goldens: `go test ./internal/corpustest/ -run TestHarnessA -update`
- Diagnostics goldens: `go test ./internal/corpustest/ -run TestHarnessC` (hand-authored `.diagnostics.json`; never `-update` them). Harness C runs parse + `AnalyzeFile` + `MergeWorkspaceSymbols` + `AnalyzeWorkspace`. It does **not** run `LintWorkspace`, `LintCommunication` or the new `LintModel`: those codes are tested with unit tests.
- Diagram goldens: `go test ./internal/visualizer/ -run TestGolden -update`
- Render check (needs Docker): `go test -tags integration ./internal/visualizer/ -run 'Mermaid|Render'`
- A local binary: `CRAFT=$(mktemp -d)/craft && go build -o "$CRAFT" ./cmd/craft`

---

## Corrections to the spec found while planning

The plan follows the spec except where the code or the sibling repos say otherwise:

1. **Publish labels.** The sequence view labels a publish `notifies "<event>"` (every golden has it), not the bare event the spec examples print (`cooker ->> cooker : sns.mercury-cooked`). The merged publish of `notifies one of` follows the existing form: `notifies one of "a", "b", "c" (+1 more)`. On-stage deliveries keep the bare event label, as today.
2. **`//` inside an address.** Craft's lexer opens a comment only when `//` follows whitespace (`internal/lexer/lexer.go:198`), so `[ADDR https://s3.example.com/x]` already parses whole (checked with a v2.23 build). Only an address with `//` after a space, such as `[ADDR //cdn.example.com/x]`, is cut. The `unquoted-address` lint stays, for that case; docs say "quote an address that starts with `//`; quoting is always allowed".
3. **`notifies one of …` parsed before.** It was not a syntax error: `notifies one of a.B, c.D` gave `notifies "one"` plus a second, internal action `of a.B , c.D` on the same line. Zero corpus uses. CHANGELOG lists it under **Changed**, next to the spec's three.
4. **Continuation indent.** The formatter already hangs a line that follows a comma by `ContinuationIndent` (4) past the line's indent (`internal/lsp/formatsep.go`, `contIndent`). A `listens` item therefore lands 8 columns past `when` (as the spec says), and a `notifies one of` item 4 columns past the step's subject (the spec says 8). One rule for every wrapped list; no formatter code changes.
5. **Versions.** v2.23 released tree-sitter-craft **0.3.0** and the extension pins **`^0.3.0`**. Part A releases tree-sitter **0.4.0** (`./release.sh minor`) and the extension **0.0.12**; Part B releases **0.5.0** and **0.0.13**. The spec's `0.1.12`/`0.1.13` predate the 0.2.0 npm release.
6. **No new token kind for `//@`.** A cite stays a `SyntaxKindLineComment` token and `syntax.IsSourceComment` classifies it by its `//@` prefix. Eleven sites switch over comment kinds (`parser.go` ×6: `collectPhrase`, `peekAt`, `peek`, `attachTrivia`, the lexer-kind map and its trivia check; `tree.go` `isTrivia`; `ast.go` ×2; `formatter.go` `isCommentKind`; `completion.go:140`); a new kind missed at any one of them would make the comment significant and break the parse. tree-sitter still gets its own `source_comment` node, for highlighting.
7. **Semantic-token names.** The LSP legend is `craft-*` (`internal/lsp/server.go`). `stream`, `one`, `of`, `inferred`, `includes`, `continues` map to `craft-flow-keyword`; a list comma to the existing `craft-comma`; a `//@` line stays one `craft-comment` token (no `namespace`/`number` types exist). The extension highlights cite parts through tree-sitter.
8. **Where model lints run.** `LintWorkspace` runs before resolution and sees only trees. `duplicate-scenario-body`, the link lints and the journey lints read the resolved model, so they live in a new `sema.LintModel(doc)` that `analysis.Workspace` runs after `resolveParticipants`. `stream-without-caller` reads interactions, so it joins `LintCommunication`. Both are unit-tested through `analysis.File`/`analysis.Workspace`.
9. **Cite diagnostics are per file.** `craft/syntax/malformed-source` and `craft/lint/detached-source` need no workspace, so `sema.AnalyzeFile` emits them and Harness C covers them. The code names are the spec's.
10. **Event definition.** No go-to-definition exists today from a `listens` ref to its publishers (spec §4.6 says "as for a single ref today"). Task A14 adds it for single refs and for list items.
11. **Spine title placement.** PlantUML gets `title Journey: <slug>` through the existing `withTitle`, which writes it right after `@startuml` (before the skinparams, not after as the spec listing shows). Mermaid gets the front matter of §17.4.
12. **`Answers` with a `to` target.** The spec does not say. A targeted `returns stream to Y` answers the most recent open request from `Y` to the returner on the call stack, without popping it; `""` when there is none.
13. **JSON tags.** `Journey`, `JourneyMember` and `JourneyEdge` get JSON tags (`slug`, `members`, `edges`, `cycle`, `from`, `to`, `kind`, `event`); the spec listing omits them on two types.
14. **Server requests.** Every preview endpoint takes one `dsl` string. The journey endpoints take `files` (`{"<name>": "<source>"}`), because a journey spans files.
15. **Hub conventions.** Hub issue codes are snake_case (`free_text_ref`, `dead_event`), so the spec's `tag-unknown-value` and `shared-tables` are `tag_unknown_value` and `shared_tables`. The hub reads its corpus from an embedded `fs.FS` with no git history, so the cite freshness job (spec §14.6) reads both sides through the GitLab blame API behind the existing `RepoInspector` port, in its own application service, not inside graphstore's synchronous validation pass.
16. **LSP stubs.** `References`, `CodeLens`, `CodeAction` and `DocumentLink` exist as stubs returning nil and are not advertised. Part B implements them and advertises each (`internal/lsp/capabilities_test.go` is created by the first task that needs it).

---

## File map

| File | Responsibility | Tasks |
|---|---|---|
| `internal/lang/lang.go`, `verbs.go`, new `opitems.go`, new `cite.go` | actor kinds, `OpInferred`, tag keys, `ADDR`/`SQL`, `SplitOperationItems`/`ParseOperationItem`, `ParseSourceComment`, `LinkKind`, `JourneyEdgeKind` | A1, A6, A9, B1, B2 |
| `internal/model/model.go` | `Trigger.Events`, `Action.Events/Operations/Stream/Answers/Inferred`, list methods, `Operation.Tables`, actor-kind constants; `Source`, `Link`, `UseCaseRef`, `Step.Link`, `Sources` ×6, `ResolveUseCaseTitle`, `FirstStepIndex` skips links | A1, B1, B4 |
| `pkg/craft/craftdoc.go`, new `pkg/craft/journey.go` | re-exports, stability note, `BuildJourney`, `JourneySlugs` | A1, B1, B6 |
| `internal/syntax/kind.go` | `KwOne/KwOf/KwStream/KwInferred`, `SyntaxKindEventList`; `KwIncludes/KwContinues`, `SyntaxKindLink` | A3, B3 |
| `internal/syntax/parser.go` | event lists, `notifies one of`, `returns stream`, `inferred`, widened empty-op check, link steps | A3, A7, A9, A11, B3 |
| `internal/syntax/ast.go`, `model_views.go`, new `sources.go` | `EventItems`, `IsOneOf`, `IsStream`, `IsInferred`, return-slot shift, `LinkDecl`, `StepDecl.Link`, `IsSourceComment`, `SourceComments` | A3, A7, A9, A11, B2, B3 |
| `internal/analysis/project.go`, `resolve.go`, `analysis.go` | project lists, ops, stream, inferred, sources, links; resolve links; `Answers` pass; call `LintModel` | A3, A5–A11, B2–B4 |
| `internal/sema/lint.go`, `validate.go`, `surface.go`, `sema.go`, new `lint_model.go`, new `sources.go` | per-item lints, new lints, `LintModel`, cite checks | A3–A11, B2, B4, B6 |
| `internal/interaction/interaction.go` | per-item `Listen`/`Publish`, `OneOf`, `Operations`, `Inferred`, `Stream`, `AnsweredRequests`; links skipped | A4, A6–A11, B4 |
| `internal/journey/journey.go` (new) | members, edges, order, slugs | B6 |
| `internal/visualizer/view/{view,sequence,flow,c4}.go` | roster arrow, scope titles, merged publish, inferred, stream, `MessageRef`, shared stage | A4, A5, A8, A10, A12, B5, B7 |
| `internal/visualizer/plantuml/*.go`, `mermaid/*.go` | stereotypes, grey arrows, `loop stream`, `ref over`, journey title | A2, A10, A12, B5, B7 |
| `internal/visualizer/visualizer.go` | `Request.Journey` | B7 |
| `cmd/craft/generate.go`, `cmd/server/main.go` | `--journey`; `/journeys`, `/preview/journey`, `/preview/mermaid/journey` | B8, B9 |
| `internal/lsp/*` | formatter tests, hover, definition, completion, tokens, references, document links, code lens, code action | A13–A15, B10–B12 |
| `testdata/corpus/04_use_cases/*`, `testdata/broken/*` | fixtures | A3, A6, A7, A9, A11, B2, B3 |
| `internal/visualizer/testdata/journey-semantics-2.craft`, `journey-spine/*.craft` + goldens | diagram goldens | A16, B13 |
| `docs/GRAMMAR.md`, `docs/DIAGNOSTICS.md`, `CONTEXT.md`, `README.md`, `CHANGELOG.md` | docs | A17, A23, B14, B20 |
| `../tree-sitter-craft/*` | grammar, highlights, corpus | A18, B15 |
| `../craft-vscode-extension/*` | bump, completions, previewJourney, document links | A19, B16 |
| `svc-re-go-arch-context/internal/**` | hub consumer | A20, A21, B17, B18 |

## Task overview

**Part A — v2.24.0**

| # | Title | Model |
|---|---|---|
| A1 | Vocabulary and model fields, no behaviour change | cheap |
| A2 | Actor kinds `scheduler`/`tool`/`infra` in every view | cheap |
| A3 | `listens` lists: syntax kinds, parser, views, projection, `store-ref-in-event-list` | strong |
| A4 | `listens` lists: interactions, one inbound arrow per trigger, per-item lints | strong |
| A5 | `LintModel`, `duplicate-scenario-body`, scope in sequence titles | strong |
| A6 | Operation lists | strong |
| A7 | `notifies one of`: syntax, model, lints | strong |
| A8 | `notifies one of`: interactions, merged publish, flow labels | strong |
| A9 | `ADDR`/`SQL`/`inferred`: syntax, model, lints | strong |
| A10 | Inferred and egress rendering | cheap |
| A11 | `returns stream`: syntax, model, `Answers`, `stream-without-caller` | strong |
| A12 | Stream rendering | cheap |
| A13 | Formatter and fuzz | strong |
| A14 | LSP hover, definition and semantic tokens | strong |
| A15 | LSP completion | cheap |
| A16 | Visualizer goldens and render check | cheap |
| A17 | Documentation | cheap |
| A18 | tree-sitter-craft: grammar, corpus, release 0.4.0 | strong |
| A19 | VSCode extension: bump and completions, release 0.0.12 | cheap |
| A20 | Hub: craft v2.24.0, rosters, operation lists, one of, egress, inferred, streams, actor kinds | strong |
| A21 | Hub: `site`/`env` filters and `tag_unknown_value` | strong |
| A22 | Dogfood on re-arch-central (verification only) | cheap |
| A23 | Release prep v2.24.0 (CHANGELOG) | cheap |

**Part B — v2.25.0**

| # | Title | Model |
|---|---|---|
| B1 | Vocabulary and model types, no behaviour change | cheap |
| B2 | `//@` source cites: parse, attach, diagnose | strong |
| B3 | Link steps: syntax and projection | strong |
| B4 | Link resolution, `LintModel` link lints, steps that are not actions | strong |
| B5 | Links in the sequence view (`MessageRef`) | cheap |
| B6 | `internal/journey`, `BuildJourney`, `journey-list`, `journey-unconnected` | strong |
| B7 | Spine rendering: shared stage, `Request.Journey`, title | strong |
| B8 | `craft generate --journey` | cheap |
| B9 | Server journey endpoints | cheap |
| B10 | Formatter and fuzz | cheap |
| B11 | LSP: cites and links | strong |
| B12 | LSP: journeys (hover, definition, code lens) | strong |
| B13 | Visualizer goldens and render check | cheap |
| B14 | Documentation | cheap |
| B15 | tree-sitter-craft: grammar, corpus, release 0.5.0 | strong |
| B16 | VSCode extension: bump, `craft.previewJourney`, cite links, release 0.0.13 | strong |
| B17 | Hub: craft v2.25.0, sources on steps, link edges, spine order | strong |
| B18 | Hub: cite freshness service | strong |
| B19 | Dogfood on re-arch-central (verification only) | cheap |
| B20 | Release prep v2.25.0 (CHANGELOG) | cheap |

---

# Part A — v2.24.0

### Task A1: Vocabulary and model fields, no behaviour change — **cheap**

**Files:**
- Modify: `internal/lang/lang.go`, `internal/lang/verbs.go`, `internal/model/model.go`, `pkg/craft/craftdoc.go`
- Test: `internal/model/model_test.go`, `pkg/craft/craftdoc_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/model/model_test.go`)

```go
func TestEventList_FallsBackToEvent(t *testing.T) {
	if got := (model.Trigger{Event: "a.A"}).EventList(); !slices.Equal(got, []string{"a.A"}) {
		t.Errorf("single = %v", got)
	}
	if got := (model.Trigger{Event: "a.A", Events: []string{"a.A", "b.B"}}).EventList(); !slices.Equal(got, []string{"a.A", "b.B"}) {
		t.Errorf("list = %v", got)
	}
	if got := (model.Trigger{}).EventList(); got != nil {
		t.Errorf("none = %v", got)
	}
	if got := (model.Action{Event: "x.X"}).EventList(); !slices.Equal(got, []string{"x.X"}) {
		t.Errorf("action = %v", got)
	}
}

func TestOperationList(t *testing.T) {
	op := &model.Operation{Text: "a, b"}
	a := model.Action{Operation: op, Operations: []model.Operation{{Text: "a"}, {Text: "b"}}}
	if got := a.OperationList(); len(got) != 2 || got[1].Text != "b" {
		t.Errorf("list = %+v", got)
	}
	if got := (model.Action{Operation: op}).OperationList(); len(got) != 1 || got[0].Text != "a, b" {
		t.Errorf("single = %+v", got)
	}
	if got := (model.Action{}).OperationList(); got != nil {
		t.Errorf("none = %+v", got)
	}
}

func TestOperationTables(t *testing.T) {
	if got := (model.Operation{Verb: "SQL", Payload: "investment flat  city"}).Tables(); !slices.Equal(got, []string{"investment", "flat", "city"}) {
		t.Errorf("SQL = %v", got)
	}
	if got := (model.Operation{Verb: "GET", Payload: "/x"}).Tables(); got != nil {
		t.Errorf("GET = %v", got)
	}
}

func TestV224Fields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.Scenario{
		Trigger: model.Trigger{Type: model.TriggerTypeDomainListen, Event: "a.A"},
		Steps:   []model.Step{{Action: &model.Action{ID: "action_2", Type: model.ActionTypeReturn}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"events"`, `"operations"`, `"stream"`, `"answers"`, `"inferred"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}

func TestKnownActorTypes(t *testing.T) {
	want := []string{"user", "system", "service", "scheduler", "tool", "infra"}
	if got := model.KnownActorTypes(); !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
}
```

Add `slices` to the test imports (`encoding/json` and `strings` are already there from v2.23). Append to `pkg/craft/craftdoc_test.go`:

```go
func TestV224Exports(t *testing.T) {
	_ = []craft.ActorType{craft.ActorTypeScheduler, craft.ActorTypeTool, craft.ActorTypeInfra}
	_ = []string{craft.OpVerbAddr, craft.OpVerbSQL, craft.TagSites, craft.TagEnvs}
	if len(craft.KnownActorTypes()) != 6 {
		t.Error("KnownActorTypes")
	}
	var a craft.Action
	_, _, _ = a.EventList(), a.OperationList(), craft.Operation{}.Tables()
	_ = craft.Trigger{}.EventList()
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/model/ ./pkg/craft/`
Expected: FAIL to compile (`EventList`, `KnownActorTypes`, `ActorTypeScheduler` undefined).

- [ ] **Step 3: Vocabulary** (`internal/lang/lang.go`, after `ParticipantKind`)

```go
// Actor kinds. The actor-type slot is open (grammar ruling Q5): any other
// identifier is legal and is drawn as a person.
const (
	ActorUser      = "user"
	ActorSystem    = "system"
	ActorService   = "service"
	ActorScheduler = "scheduler" // a clock that starts work: cron, an in-process ticker
	ActorTool      = "tool"      // a CLI, script or client a person runs; it, not the person, calls
	ActorInfra     = "infra"     // platform machinery: pod start, probes, CI pipelines, monitors
)

// KnownActorTypes is user, system, service, scheduler, tool, infra, in that order.
func KnownActorTypes() []string {
	return []string{ActorUser, ActorSystem, ActorService, ActorScheduler, ActorTool, ActorInfra}
}

// OpInferred is the bracket-head marker of an inferred hop: `[inferred …]`.
const OpInferred = "inferred"

// Recognised tag keys (spec §3.3). Every other key is free text.
const (
	TagSites = "sites" // printed in the sequence and spine section titles
	TagEnvs  = "envs"
)
```

`internal/lang/verbs.go`, above `protocolVerbs` (they join the map in Task A9, not here):

```go
// OpVerbAddr and OpVerbSQL are the egress protocol verbs (spec §9.1).
const (
	OpVerbAddr = "ADDR" // egress to an address that has no operation id
	OpVerbSQL  = "SQL"  // a direct read or write of another party's tables
)
```

- [ ] **Step 4: Model** (`internal/model/model.go`; add `strings` to the imports)

1. `Trigger`, after `Ref`:

```go
	// Events holds every event of a listens list, in source order, with the
	// same text Event would hold for each. Nil for a single event: read Event.
	// Event and Ref stay the first item.
	Events []string `json:"events,omitempty"`
```

2. `Action`, after `Ref`: the same field with the comment `// Events holds every event of a "notifies one of" list, in source order; nil for a single event.`; after `Operation`:

```go
	// Operations holds each item of a bracket that lists two or more, in
	// order. Nil for a single-item bracket: read Operation, which keeps the
	// whole bracket text.
	Operations []Operation `json:"operations,omitempty"`
```

   and after `Phrase`:

```go
	// Stream is set on a `returns stream` action: a reply that repeats.
	Stream bool `json:"stream,omitempty"`
	// Answers is the ID of the request a stream answers (the asks whose
	// call-stack frame the reply settles); "" when it answers the trigger's
	// actor, the unobserved caller or nobody. Set only when Stream is true.
	Answers string `json:"answers,omitempty"`
	// Inferred is set when the bracket opens with `inferred`: the hop was
	// inferred from names, configuration or convention, not read in code.
	Inferred bool `json:"inferred,omitempty"`
```

3. Methods (next to `Scenario.Actions`):

```go
// EventList returns Events, or a one-element slice holding Event, or nil
// when the trigger names no event.
func (t Trigger) EventList() []string { return eventList(t.Events, t.Event) }

// EventList returns Events, or a one-element slice holding Event, or nil
// when the action names no event.
func (a Action) EventList() []string { return eventList(a.Events, a.Event) }

func eventList(events []string, event string) []string {
	if len(events) > 0 {
		return events
	}
	if event == "" {
		return nil
	}
	return []string{event}
}

// OperationList returns Operations, or a one-element slice holding
// *Operation, or nil when the action has no bracket.
func (a Action) OperationList() []Operation {
	if len(a.Operations) > 0 {
		return a.Operations
	}
	if a.Operation == nil {
		return nil
	}
	return []Operation{*a.Operation}
}

// Tables returns the table names of a SQL operation (its payload fields),
// and nil for any other verb.
func (o Operation) Tables() []string {
	if o.Verb != lang.OpVerbSQL {
		return nil
	}
	return strings.Fields(o.Payload)
}
```

4. Actor kinds and tag keys:

```go
const (
	ActorTypeUser      ActorType = lang.ActorUser
	ActorTypeSystem    ActorType = lang.ActorSystem
	ActorTypeService   ActorType = lang.ActorService
	ActorTypeScheduler ActorType = lang.ActorScheduler
	ActorTypeTool      ActorType = lang.ActorTool
	ActorTypeInfra     ActorType = lang.ActorInfra
)

// KnownActorTypes is user, system, service, scheduler, tool, infra.
func KnownActorTypes() []string { return lang.KnownActorTypes() }

// TagSites and TagEnvs are the scope tag keys the views print.
const (
	TagSites = lang.TagSites
	TagEnvs  = lang.TagEnvs
)
```

   (replacing the existing three-constant block). Update the `Actor` comment: "An entity outside the system that interacts with it: a person (user), an external system or service, a scheduler, a tool or platform infrastructure."

- [ ] **Step 5: `pkg/craft`** (`pkg/craft/craftdoc.go`)

Constants: `ActorTypeScheduler`, `ActorTypeTool`, `ActorTypeInfra`, `TagSites`, `TagEnvs` (each `= model.X`); `OpVerbAddr = "ADDR"` and `OpVerbSQL = "SQL"` in the `OpVerb*` block. Function:

```go
// KnownActorTypes lists the actor kinds Craft recognises, in presentation
// order. Any other kind is legal and is drawn as a person.
func KnownActorTypes() []string { return model.KnownActorTypes() }
```

Append to the package doc, after the v2.23.0 paragraph:

```go
// Since v2.24.0 a listens trigger or a notifies may carry several events
// (EventList), and an action several operations (OperationList).
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/model/ ./pkg/craft/` → PASS. Then the full suite → PASS with no golden change (`git status --short testdata internal/visualizer/testdata` is empty).

- [ ] **Step 7: Commit**

```bash
git add internal/lang internal/model pkg/craft
git commit -m "feat(model): vocabulary and fields for journey semantics 2 (v2.24)

Trigger.Events, Action.Events/Operations/Stream/Answers/Inferred, the
EventList/OperationList/Tables methods, actor kinds scheduler/tool/infra,
ADDR/SQL constants and the sites/envs tag keys. Nothing produces them yet.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A2: Actor kinds `scheduler`/`tool`/`infra` in every view — **cheap**

**Files:**
- Modify: `internal/visualizer/plantuml/plantuml.go` (`actorElement`, new `actorStereotype`, `Flow`, `Sequence`), `internal/visualizer/mermaid/views.go` (`Sequence`), `internal/visualizer/view/c4.go` (`actor`)
- Test: `internal/visualizer/sequence_rendering_test.go`, `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Write the failing tests**

```go
const actorKindsSrc = `actors {
    scheduler Ticker
    tool OperatorTool
    user Operator
}
domain vas {
    vasscheduler
    account
}
use_case "The activation sweep starts due schedules" {
    when Ticker fires the vasscheduler activation sweep
        vasscheduler starts every schedule that is due
}
use_case "Operator exports agency portfolio values" {
    when Operator runs the portfolio export
        OperatorTool asks account for the agency owner members  [backofficeMembersList]
}
`

func TestSequence_ActorKindsAreStereotypedBoundaries(t *testing.T) {
	puml, mmd := sequenceSource(t, actorKindsSrc)
	for _, want := range []string{
		`boundary "Ticker" as Ticker <<scheduler>>`,
		`boundary "OperatorTool" as OperatorTool <<tool>>`,
		`actor "Operator" as Operator`,
		"Ticker -> vasscheduler : fires the vasscheduler activation sweep",
		"Operator -> OperatorTool : runs the portfolio export",
	} {
		requireLine(t, "plantuml", puml, want)
	}
	for _, want := range []string{
		"participant Ticker as Ticker (scheduler)",
		"participant OperatorTool as OperatorTool (tool)",
		"actor Operator as Operator",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
}

func TestFlowAndC4_ActorKinds(t *testing.T) {
	doc := resolvedDoc(t, actorKindsSrc)
	flow, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	requireLine(t, "flow", string(flow), `boundary "Ticker" as Ticker <<scheduler>>`)
	c4, _, err := renderSource(doc, ViewC4, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	requireLine(t, "c4", string(c4), `System_Ext(Ticker, "Ticker", "Scheduler")`)
}
```

`TestFlowAndC4_ActorKinds` fails only if C4 draws `Ticker`; if the C4 view draws no actor for this model (it draws actors that trigger a use case touching a service, `findActors`), add `services { vas { contexts: vasscheduler, account } }` to the source. Lint pin (append to `internal/sema/lint_journey_test.go`):

```go
func TestLint_UnusedActorTreatsNewKindsLikeSystem(t *testing.T) {
	for _, kind := range []string{"scheduler", "tool", "infra"} {
		unused := "actors {\n  " + kind + " X\n}\n"
		used := unused + "use_case \"U\" {\n  when X starts it\n    a does b\n}\n"
		if n := lintCodes(t, unused, "craft/lint/unused-actor"); n != 1 {
			t.Errorf("%s unused: %d", kind, n)
		}
		if n := lintCodes(t, used, "craft/lint/unused-actor"); n != 0 {
			t.Errorf("%s used: %d", kind, n)
		}
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/visualizer/ ./internal/sema/ -run 'ActorKind|NewKinds'` → the visualizer tests FAIL; the lint pin passes already (keep it).

- [ ] **Step 3: PlantUML** (`plantuml.go`)

```go
// actorElement is the PlantUML element of an actor kind. Every kind outside
// the system that is not a person keeps the boundary shape (spec §6.5).
func actorElement(t model.ActorType) string {
	switch t {
	case model.ActorTypeSystem, model.ActorTypeScheduler, model.ActorTypeTool, model.ActorTypeInfra:
		return "boundary"
	case model.ActorTypeService:
		return "control"
	}
	return "actor"
}

// actorStereotype names a scheduler, tool or infra actor on its declaration
// line (` <<scheduler>>`); "" for every other kind.
func actorStereotype(t model.ActorType) string {
	switch t {
	case model.ActorTypeScheduler, model.ActorTypeTool, model.ActorTypeInfra:
		return " <<" + string(t) + ">>"
	}
	return ""
}
```

In `Flow`'s actor loop: `fmt.Fprintf(&b, "%s \"%s\" as %s%s\n", actorElement(n.ActorType), text(n.Label), al.declare(n.ID, n.Label), actorStereotype(n.ActorType))`. In `Sequence`'s participant loop add `stereo := ""`, set `kind, stereo = actorElement(p.ActorType), actorStereotype(p.ActorType)` in the `NodeActor` case, and print `"%s \"%s\" as %s%s\n"` with `stereo` last.

- [ ] **Step 4: Mermaid** (`views.go`, `Sequence`; import `internal/model`)

```go
		case view.NodeActor:
			kind = "actor"
			switch p.ActorType {
			case model.ActorTypeScheduler, model.ActorTypeTool, model.ActorTypeInfra:
				// Not people: a box, with the kind in text (Mermaid has no
				// stereotypes), as a store is marked "(store)".
				kind, label = "participant", label+" ("+string(p.ActorType)+")"
			}
```

The Mermaid flow keeps `([…])` for every actor (spec §6.5 table).

- [ ] **Step 5: C4** (`view/c4.go`, `actor`): add

```go
		case model.ActorTypeScheduler:
			a.Element, a.Description = C4ExternalSystem, "Scheduler"
		case model.ActorTypeTool:
			a.Element, a.Description = C4ExternalSystem, "Operator tool"
		case model.ActorTypeInfra:
			a.Element, a.Description = C4ExternalSystem, "Platform infrastructure"
```

- [ ] **Step 6: Run** the tests → PASS; `go test ./internal/visualizer/ -run TestGolden` → no change (no golden declares the new kinds; `testdata/broken/unexpected_actor_type.craft` keeps its `robot` actor drawn as a person). Full suite.

- [ ] **Step 7: Commit**

```bash
git add internal/visualizer internal/sema
git commit -m "feat(diagrams): scheduler, tool and infra actors draw as stereotyped boundaries

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A3: `listens` lists: syntax kinds, parser, views, projection, `store-ref-in-event-list` — **strong**

**Files:**
- Modify: `internal/syntax/kind.go`, `internal/syntax/parser.go` (`parseTrigger` listens branch; new `parseEventList`, `parseEventItem`, `eventItemAhead`, `refEndAhead`), `internal/syntax/ast.go` (`TriggerDecl.EventValue`, `EventCol`, `EventIsString`; new `EventItem`, `eventItemsOf`, `TriggerDecl.EventItems`), `internal/syntax/model_views.go` (`EventRef`), `internal/analysis/project.go` (`projectTrigger`), `internal/sema/validate.go` (new `validateEventLists`), `internal/sema/sema.go` (`AnalyzeFile`)
- Create: `internal/syntax/eventlist_test.go`, `testdata/corpus/04_use_cases/{listens_list,listens_list_multiline,listens_list_schedule}.craft` + goldens, `testdata/broken/{listens_list_trailing_comma,listens_list_store_ref}.craft` + `.diagnostics.json`

**Tree shape** (pin this; Tasks A4, A7, A13, A14 depend on it):

```
Trigger
  Ident(seshat) KwListens
  EventList                      (only when the slot holds a comma)
    Ref(sns.a) Comma Ref(sns.b) Comma String("c")
  KwCron String                  (optional schedule, after the last item)
```

A single item keeps the v2.23 shape (`Ref` or `String` directly under `Trigger`), so `significantElements(trigger)[2]` is the event slot in both shapes and the schedule stays at `[3]`/`[4]`.

- [ ] **Step 1: Add every Part A syntax kind** (`internal/syntax/kind.go`)

Append before `syntaxKindTokenSentinel`, in this order:

```go
	// Journey-semantics-2 contextual keywords (v2.24.0, spec §3.1).
	SyntaxKindKwOne      // `one` of `notifies one of`
	SyntaxKindKwOf       // `of` of `notifies one of`
	SyntaxKindKwStream   // `stream` directly after `returns`
	SyntaxKindKwInferred // `inferred` at the head of an operation bracket
```

Append at the end of the node block:

```go
	// SyntaxKindEventList wraps the items and commas of a `listens` or
	// `notifies one of` list. It exists only when the slot holds a comma.
	SyntaxKindEventList
```

- [ ] **Step 2: Write the failing tests** (`internal/syntax/eventlist_test.go`)

```go
package syntax_test

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func listensTrigger(t *testing.T, src string) syntax.TriggerDecl {
	t.Helper()
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	return syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Trigger()
}

func itemValues(items []syntax.EventItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Value)
	}
	return out
}

func TestListensList_OneLine(t *testing.T) {
	tr := listensTrigger(t, "use_case \"U\" {\n    when seshat listens sns.a-changed, \"b changed\", c.C\n        seshat writes the row\n}")
	if got := itemValues(tr.EventItems()); !slices.Equal(got, []string{"sns.a-changed", "b changed", "c.C"}) {
		t.Errorf("items = %v", got)
	}
	if tr.EventValue() != "sns.a-changed" || tr.EventRef() != "sns.a-changed" {
		t.Errorf("first item: value %q ref %q", tr.EventValue(), tr.EventRef())
	}
	if items := tr.EventItems(); !items[1].IsString || items[1].Ref != "" || items[2].Ref != "c.C" {
		t.Errorf("item kinds = %+v", items)
	}
}

func TestListensList_ContinuesAfterATrailingComma(t *testing.T) {
	src := "use_case \"U\" {\n    when seshat listens sns.a,\n            // the vas topics\n            sns.b,\n            sns.c every \"5m\"\n        seshat writes the row\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
	if got := itemValues(sc.Trigger().EventItems()); !slices.Equal(got, []string{"sns.a", "sns.b", "sns.c"}) {
		t.Errorf("items = %v", got)
	}
	if _, e, ok := sc.Trigger().ListenSchedule(); !ok || e != "5m" {
		t.Errorf("schedule = %q %v", e, ok)
	}
	if n := len(sc.Steps()); n != 1 {
		t.Errorf("%d steps, want 1: a continuation line is never a step", n)
	}
}

func TestListensList_SingleItemKeepsTheV223Tree(t *testing.T) {
	tr := listensTrigger(t, "use_case \"U\" {\n    when seshat listens sns.a\n        seshat writes the row\n}")
	if tr.Node().ChildNode(syntax.SyntaxKindEventList) != nil {
		t.Error("a single item must not be wrapped in an EventList")
	}
	if got := itemValues(tr.EventItems()); !slices.Equal(got, []string{"sns.a"}) {
		t.Errorf("items = %v", got)
	}
}

func TestListensList_DanglingCommaBeforeAStep(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when seshat listens sns.a,\n        seshat writes the row\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" || diags[0].Range.Start.Line != 1 {
		t.Fatalf("diags = %v", diags)
	}
}
```

If `TriggerDecl` has no `Node()` accessor, add `func (t TriggerDecl) Node() SyntaxNode { return t.node }`.

- [ ] **Step 3: Run** `go test ./internal/syntax/ -run ListensList` → FAIL (`EventItems` undefined).

- [ ] **Step 4: Parser** (`parser.go`)

1. Lookahead helpers, next to `refShapedAhead`:

```go
// refEndAhead returns the lookahead index just past the ref-shaped run that
// starts at offset: contiguous ident, number, '/' and kind-colon tokens on
// one line, the run parseRef would consume.
func (p *Parser) refEndAhead(offset int) int {
	first := p.peekAt(offset)
	i := offset
	prev := first
	for {
		t := p.peekAt(i)
		ok := t.Type == lexer.TokenIdent || t.Type == lexer.TokenNumber || isAnyKeywordAsIdent(t.Type) ||
			(t.Type == lexer.TokenError && t.Value == "/") || t.Type == lexer.TokenColon
		if !ok || t.Line != first.Line || (i > offset && !adjacentTokens(prev, t)) {
			return i
		}
		prev = t
		i++
	}
}

// eventItemAhead reports whether the token at offset starts an event item of
// a list: a string, or a ref whose run is followed by a comma, the end of
// its line, `}`, EOF, an operation bracket or a `cron`/`every` schedule.
// Anything else is a step line (`seshat writes the row`), which the list
// must not swallow.
func (p *Parser) eventItemAhead(offset int) bool {
	t := p.peekAt(offset)
	switch {
	case t.Type == lexer.TokenString:
		return true
	case t.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(t.Type):
		return false
	}
	end := p.refEndAhead(offset)
	next := p.peekAt(end)
	switch {
	case next.Type == lexer.TokenEOF, next.Type == lexer.TokenRBrace, next.Line != t.Line,
		next.Type == lexer.TokenComma, next.Type == lexer.TokenLBracket:
		return true
	case next.Type == lexer.TokenIdent && (next.Value == "cron" || next.Value == "every"):
		return p.peekAt(end+1).Type == lexer.TokenString
	}
	return false
}
```

2. The list:

```go
// parseEventItem consumes one event item: a quoted string or a ref.
func (p *Parser) parseEventItem() {
	switch t := p.peek(); {
	case t.Type == lexer.TokenString:
		p.consumeAs(SyntaxKindString)
	case t.Type == lexer.TokenIdent:
		p.parseRef()
	default:
		p.consumeAs(SyntaxKindIdent)
	}
}

// parseEventList parses `event_item (',' NL* event_item)*` (spec §4.1). The
// items are wrapped in a SyntaxKindEventList only when a comma is seen, so a
// single item keeps the v2.23 tree. A comma with no item after it is
// reported on the comma; the next line then parses as a step. It returns the
// last token consumed, whose line is where a trailing schedule or bracket
// must sit.
func (p *Parser) parseEventList() (lexer.Token, []model.Diagnostic) {
	var diags []model.Diagnostic
	cp := p.builder.Checkpoint()
	p.parseEventItem()
	commas := 0
	for p.peek().Type == lexer.TokenComma {
		comma := p.consumeAs(SyntaxKindComma)
		commas++
		if !p.eventItemAhead(0) {
			diags = append(diags, p.diagUnexpected(comma, "an event after `,`"))
			break
		}
		p.parseEventItem()
	}
	if commas > 0 {
		p.builder.StartNodeAt(cp, SyntaxKindEventList)
		p.builder.FinishNode()
	}
	return p.lastConsumed(), diags
}
```

   `diagUnexpected(comma, …)` reports `unexpected ","`; keep the message.
3. In `parseTrigger`'s listens branch, replace the event `if/else` with:

```go
		last := verbTok
		if t := p.peek(); t.Line == verbTok.Line && (t.Type == lexer.TokenString || t.Type == lexer.TokenIdent) {
			var d []model.Diagnostic
			last, d = p.parseEventList()
			diags = append(diags, d...)
		}
```

   and test the schedule against `last.Line` instead of `verbTok.Line` (twice). Comments between continuation lines are trivia: `peek` already skips them.

- [ ] **Step 5: Views** (`ast.go`)

```go
// EventItem is one item of an event slot: a `listens` trigger's events or a
// `notifies` action's.
type EventItem struct {
	Value    string         // unquoted string, or the ref text
	Ref      string         // the ref text when written as a ref; "" for a string
	IsString bool
	Offset   green.TextSize // offset of the item's first token
}

// eventItemsOf reads an event slot element: one item, or an EventList node.
func eventItemsOf(el SyntaxElement) []EventItem {
	if n, ok := el.(SyntaxNode); ok && n.Kind() == SyntaxKindEventList {
		var out []EventItem
		for _, c := range significantElements(n) {
			if c.Kind() == SyntaxKindComma {
				continue
			}
			out = append(out, eventItemsOf(c)...)
		}
		return out
	}
	value := refAwareText(el)
	if value == "" {
		return nil
	}
	return []EventItem{{Value: value, Ref: refIfWrapped(el), IsString: el.Kind() == SyntaxKindString, Offset: refAwareOffset(el)}}
}

// EventItems returns the events of a domain-listen trigger in source order:
// one for a single event, every item of a list.
func (t TriggerDecl) EventItems() []EventItem {
	if t.Kind() != lang.TriggerDomainListen {
		return nil
	}
	if elems := significantElements(t.node); len(elems) >= 3 {
		return eventItemsOf(elems[2])
	}
	return nil
}
```

   Make the single-event views read the first item, so every existing caller sees the first item of a list:
   - `EventValue`, listens case: `if items := t.EventItems(); len(items) > 0 { return items[0].Value }`.
   - `EventCol`, listens case: `_, col := li.LineCol16(items[0].Offset)`.
   - `EventIsString`, listens case: `items[0].IsString`.
   - `EventRef` (`model_views.go`): `if items := t.EventItems(); len(items) > 0 { return items[0].Ref }; return ""`.
   - `TriggerDecl.SourceText` already copies the raw tail after the subject, so a list round-trips with its line breaks; Task A13 pins it.

- [ ] **Step 6: Projection** (`projectTrigger`, listens case)

```go
		if items := t.EventItems(); len(items) > 1 {
			for _, it := range items {
				events = append(events, it.Value)
			}
		}
```

(declare `var events []string` with the other locals), and replace the listens description case with:

```go
	case kind == lang.TriggerDomainListen:
		list := events
		if list == nil {
			list = []string{event}
		}
		quoted := make([]string, len(list))
		for i, ev := range list {
			quoted[i] = fmt.Sprintf("%q", ev)
		}
		description = fmt.Sprintf("when %s listens %s", context, strings.Join(quoted, ", "))
		if schedKind != "" {
			description += fmt.Sprintf(` %s "%s"`, schedKind, schedule)
		}
```

A single event gives the old `when X listens "E"` byte for byte. Set `Events: events` in the literal, and `out.Store = storeRef(ref)` only when `len(events) == 0` (spec §4.1 rule 4: a list never carries a store).

- [ ] **Step 7: `store-ref-in-event-list`** (`validate.go`, called from `AnalyzeFile` next to `validateSchedules`)

```go
// validateEventLists reports a store ref inside a listens or notifies-one-of
// list of two or more items (spec §4.3): a store hand-off names one store.
func validateEventLists(uri string, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	var diags []model.Diagnostic
	check := func(items []syntax.EventItem) {
		if len(items) < 2 {
			return
		}
		for _, it := range items {
			if _, _, ok := lang.ParseStoreRef(it.Ref); !ok {
				continue
			}
			var r model.Range
			if hasLI {
				line, col := li.LineCol16(it.Offset)
				r = model.Range{
					Start: model.Position{Line: lineToLSP(line), Character: colToLSP(col)},
					End:   model.Position{Line: lineToLSP(line), Character: colToLSP(col) + lspLen(it.Ref)},
				}
			}
			diags = append(diags, model.Diagnostic{
				Code:      "craft/sema/store-ref-in-event-list",
				Message:   "a store hand-off names one store; write one scenario per store",
				Severity:  model.SeverityError,
				SourceURI: uri,
				Range:     r,
			})
		}
	}
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			check(sc.Trigger().EventItems())
		}
	}
	return diags
}
```

Task A7 adds the `notifies one of` items to the same walk.

- [ ] **Step 8: Fixtures**

`testdata/corpus/04_use_cases/listens_list.craft`:

```craft
domain partner {
    cooker
    seshat
}

use_case "seshat records a Mercury domain event in the transaction ledger" {
    when seshat listens sns.mercury-advert-state-changed, sns.mercury-vas-state-changed, "mercury message received"
        seshat writes one row per event
}
```

`listens_list_multiline.craft`: the spec §4.1 example (four items over two lines) with `domain partner { seshat }`, plus a `// the remaining topics` comment line between two continuation lines.
`listens_list_schedule.craft`: `when poller listens sqs.LeadRequested, sqs.LeadUpdated every "5m"` with `domain leads { poller }` and one step.

`testdata/broken/listens_list_trailing_comma.craft`:

```craft
domain ledger {
    seshat
}

use_case "U" {
    when seshat listens sns.a-changed,
        seshat writes the row
}
```

`.diagnostics.json`: `craft/syntax/unexpected-token`, error, `{"start":{"line":5,"character":37},"end":{"line":5,"character":38}}` (the comma).

`testdata/broken/listens_list_store_ref.craft`: the same file with line 6 `    when seshat listens sns.a-changed, store.rows.inserted`. `.diagnostics.json`: `craft/sema/store-ref-in-event-list`, error, `{"start":{"line":5,"character":39},"end":{"line":5,"character":58}}`.

If Harness C also reports a diagnostic these goldens do not list, the fixture lacks a declaration; fix the fixture, not the golden.

- [ ] **Step 9: Goldens, full suite, commit**

Run `go test ./internal/corpustest/ -run TestHarnessA -update`. `listens_list.craftjson` has `"event": "sns.mercury-advert-state-changed"`, `"ref": "sns.mercury-advert-state-changed"`, `"events": ["sns.mercury-advert-state-changed", "sns.mercury-vas-state-changed", "mercury message received"]`, no `store`, and `"description": "when seshat listens \"sns.mercury-advert-state-changed\", \"sns.mercury-vas-state-changed\", \"mercury message received\""`. `listens_list_schedule.craftjson` carries `schedule: "5m"`. No other golden changes. Harness C passes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): a listens trigger takes a comma list of events (Trigger.Events); sema store-ref-in-event-list

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A4: `listens` lists: interactions, one inbound arrow per trigger, per-item lints — **strong**

**Files:**
- Modify: `internal/interaction/interaction.go` (`Derive`, `deriveScenario` listens case), `internal/visualizer/view/sequence.go` (`sequenceBuilder`, `BuildSequence`, `scenarioLayout.emit`, new `addRoster`, `capList`), `internal/sema/lint.go` (`lintDeadEvents`, `lintEventPastTense`, `lintStoreWithoutChange`, new `lintDuplicateEventInList`, `LintWorkspace`)
- Test: `internal/interaction/interaction_test.go`, `internal/visualizer/sequence_rendering_test.go`, `internal/visualizer/view/sequence_test.go`, `internal/sema/lint_journey_test.go`

- [ ] **Step 1: Write the failing tests**

Interaction (reuse `derive`, `structure`, `summary`):

```go
func TestDerive_ListensListGivesOneListenPerItemPerPublisher(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Pay" {
    when Customer pays
        Billing notifies sns.paid
        Shipping notifies sns.paid
}
use_case "Ledger" {
    when Ordering listens sns.paid, sns.refunded every "5m"
        Ordering writes a row
}
`})
	var got []string
	for _, it := range its {
		if it.Kind == interaction.Listen {
			got = append(got, it.From.Name+"->"+it.To.Name+" "+it.Event+" "+it.Schedule)
		}
	}
	want := []string{"Billing->Ordering sns.paid every 5m", "Shipping->Ordering sns.paid every 5m", "External->Ordering sns.refunded every 5m"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}
```

Sequence (spec §4.5):

```go
const rosterSrc = `actors {
    user Partner
}
domain partner {
    cooker
    seshat
}
use_case "Mercury cooks an advert" {
    when Partner posts an advert
        cooker cooks the advert for its target site
        cooker notifies sns.mercury-cooked
}
use_case "seshat records a Mercury domain event in the transaction ledger" {
    when seshat listens sns.mercury-advert-state-changed, sns.mercury-vas-state-changed,
            sns.mercury-message-received, sns.mercury-cooked
        seshat writes one row per event
}
`

func TestSequence_ListensListDrawsOneInboundArrow(t *testing.T) {
	puml, mmd := sequenceSource(t, rosterSrc)
	label := "sns.mercury-advert-state-changed, sns.mercury-vas-state-changed, sns.mercury-message-received (+1 more) (from cooker)"
	requireLine(t, "plantuml", puml, "[-> seshat : "+label)
	requireLine(t, "mermaid", mmd, "External->>seshat: "+label)
	if n := strings.Count(puml, "[-> seshat"); n != 1 {
		t.Errorf("%d inbound arrows, want 1", n)
	}
}

func TestSequence_ListensListOnStagePublisherDeliversItsItems(t *testing.T) {
	puml, _ := sequenceSource(t, `domain d {
    a
    b
}
use_case "U" {
    when a listens x.Started
        a notifies x.Done
        b notifies x.Failed
    scenario "after"
    when b listens x.Done, x.Failed, x.Other
        b records it
}
`)
	// Both scenarios share one section, so a and b are on stage when the
	// roster is drawn.
	requireLine(t, "plantuml", puml, "a ->> b : x.Done")
	requireLine(t, "plantuml", puml, "b ->> b : x.Failed")
	requireLine(t, "plantuml", puml, "[-> b : x.Other")
}
```

In `TestSequenceStepsMatchInteractionSteps` (`view/sequence_test.go`) add a fourth scenario `scenario "roster"` / `when Billing listens shop.Paid, shop.Refunded` / `Billing records it`; the test keeps passing because it ignores step 0.

Lints (append to `lint_journey_test.go`):

```go
func TestLint_DuplicateEventInList(t *testing.T) {
	dup := "use_case \"U\" {\n  when s listens a.A, b.B, a.A\n    s does it\n}"
	if n := lintCodes(t, dup, "craft/lint/duplicate-event-in-list"); n != 1 {
		t.Errorf("dup: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when s listens a.A, b.B\n    s does it\n}", "craft/lint/duplicate-event-in-list"); n != 0 {
		t.Errorf("clean: %d", n)
	}
}

func TestLint_PerItemEventLints(t *testing.T) {
	src := "use_case \"P\" {\n  when O does\n    p notifies a.Charged\n}\nuse_case \"U\" {\n  when s listens a.Charged,\n      a.Refund\n    s does it\n}"
	diags := analyzeSingleFileForTest(t, src)
	var past []model.Diagnostic
	for _, d := range diags {
		if d.Code == "craft/lint/event-not-past-tense" {
			past = append(past, d)
		}
		if d.Code == "craft/lint/dead-event" {
			t.Errorf("a.Charged is consumed by the list: %v", d)
		}
	}
	if len(past) != 1 || past[0].Range.Start.Line != 6 || past[0].Range.Start.Character != 6 {
		t.Errorf("past-tense = %v (want one, on the continuation line)", past)
	}
}
```

(`model` is already imported in the sema test package; add it if not.)

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Interaction**

1. `Derive` keeps one publisher list per event; unchanged in this task (Task A8 extends it for `one of`).
2. `deriveScenario`, listens case: guard `if t.Context == "" || len(t.EventList()) == 0 { break }`, then loop `for _, ev := range t.EventList()` around today's per-publisher loop, using `ev` for `publishers[ev]`, `Event` and `Label`. `Via` stays `storeEndpoint(t.Store)` (nil for a list).
3. Update the `Listen` doc comment: "one per event of the trigger's list and per publisher of that event".

- [ ] **Step 4: Sequence**

1. `sequenceBuilder` gains `rosters map[rosterKey]bool` with `type rosterKey struct{ useCase int; scenario string }`, filled at the top of `BuildSequence` for every scenario whose `Trigger.Events` is non-empty.
2. `scenarioLayout.emit(0)`: when the scenario is a roster, collect its step-0 `Listen` interactions and call `l.b.addRoster(l.idx, listens, l.title)` instead of `add` for each; the other step-0 interactions go through `add` as today.

```go
// capList joins items with ", ", capped at maxNamedPublishers with
// " (+N more)" for the rest (spec §4.5).
func capList(items []string) string {
	if len(items) <= maxNamedPublishers {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:maxNamedPublishers], ", ") + fmt.Sprintf(" (+%d more)", len(items)-maxNamedPublishers)
}

// addRoster draws the listens of a trigger with an event list: each
// publisher already on stage delivers its items on one arrow; every other
// item arrives on one inbound arrow, labelled with those items, the
// publishers they come from and the schedule (spec §4.5).
func (b *sequenceBuilder) addRoster(idx int, listens []interaction.Interaction, title string) {
	ns, onStage := b.ns, b.onStage[idx]
	var listener interaction.Endpoint
	var schedule string
	direct := map[string][]string{} // on-stage publisher key → items, list order
	var directOrder []interaction.Endpoint
	var inboundItems, from []string
	for _, it := range listens {
		listener, schedule = it.To, it.Schedule
		if it.From != interaction.External && onStage[it.From.Key] {
			if _, seen := direct[it.From.Key]; !seen {
				directOrder = append(directOrder, it.From)
			}
			if !slices.Contains(direct[it.From.Key], it.Event) {
				direct[it.From.Key] = append(direct[it.From.Key], it.Event)
			}
			continue
		}
		if !slices.Contains(inboundItems, it.Event) {
			inboundItems = append(inboundItems, it.Event)
		}
		if it.From != interaction.External && !slices.Contains(from, it.From.Name) {
			from = append(from, it.From.Name)
		}
	}
	suffix := ""
	if schedule != "" {
		suffix = ", " + schedule
	}
	sec := &b.seq.Sections[idx]
	to := ns.add(listener)
	for _, pub := range directOrder {
		sec.Messages = append(sec.Messages, Message{From: ns.add(pub), To: to, Kind: MessageAsync,
			Label: capList(direct[pub.Key]) + suffix, Scenario: title})
		b.appear(ns.add(pub))
	}
	if len(inboundItems) > 0 {
		label := capList(inboundItems)
		if len(from) > 0 {
			slices.Sort(from)
			label += " (from " + capList(from) + ")"
		}
		sec.Messages = append(sec.Messages, Message{To: to, Kind: MessageInbound, Label: label + suffix, Scenario: title})
	}
	b.appear(to)
	onStage[to] = true
}
```

   A single-event trigger never reaches `addRoster`, so `inboundKey` keeps `useCase|scenario|event` and no existing golden changes.

- [ ] **Step 5: Lints** (`lint.go`)

1. `lintDeadEvents`: mark every `trigger.EventItems()` value consumed (keep the `TriggerEvent` case).
2. `lintEventPastTense`: for a listens trigger, check each item, with its own line: `line, col := li.LineCol16(it.Offset); check(it.Value, uri, line, col, it.IsString)`. The `check` helper takes the line it is given, so continuation items report on their own line.
3. `lintStoreWithoutChange`: the listens case checks every item's `Ref` at `li.LineCol16(it.Offset)`.
4. New, registered in `LintWorkspace`:

```go
// ── duplicate-event-in-list ───────────────────────────────────────────────────
// Warning: the same item (exact text) twice in one event list. The range is
// the second one.

func lintDuplicateEventInList(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		check := func(items []syntax.EventItem) {
			seen := map[string]bool{}
			for _, it := range items {
				if !seen[it.Value] {
					seen[it.Value] = true
					continue
				}
				line, col := li.LineCol16(it.Offset)
				start := colToLSP(col)
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/duplicate-event-in-list",
					Message:   fmt.Sprintf("event %q is already in this list", it.Value),
					Severity:  model.SeverityWarning,
					SourceURI: uri,
					Range: model.Range{
						Start: model.Position{Line: lineToLSP(line), Character: start},
						End:   model.Position{Line: lineToLSP(line), Character: start + eventTokenLen(it.Value, it.IsString)},
					},
				})
			}
		}
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				check(sc.Trigger().EventItems())
			}
		}
	}
	return diags
}
```

- [ ] **Step 6: Run** the tests → PASS. `go test ./internal/visualizer/ -run TestGolden` → no change. Full suite.

- [ ] **Step 7: Commit**

```bash
git add internal
git commit -m "feat(interaction): one Listen per event of a listens list; one inbound arrow per roster trigger; per-item event lints

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A5: `LintModel`, `duplicate-scenario-body`, scope in sequence titles — **strong**

**Files:**
- Create: `internal/sema/lint_model.go`, `internal/analysis/lint_model_test.go`
- Modify: `internal/analysis/analysis.go` (`Workspace`), `internal/visualizer/view/sequence.go` (`BuildSequence`, new `sectionTitle`)
- Test: `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests** (`internal/analysis/lint_model_test.go`)

```go
package analysis

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func modelDiags(t *testing.T, src, code string) []model.Diagnostic {
	t.Helper()
	var out []model.Diagnostic
	for _, d := range Workspace([]FileResult{File("m.craft", src)}).Diagnostics {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

const dupHead = "domain d {\n  s\n  t\n}\nuse_case \"U\" {\n"

func TestLintModel_DuplicateScenarioBody(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
		fold       bool
	}{
		{"identical listens", "  when s listens a.A\n    s asks t to do it  [x]\n  when s listens b.B\n    s asks t to do it  [x]\n", 1, true},
		{"one phrase differs", "  when s listens a.A\n    s asks t to do it\n  when s listens b.B\n    s asks t to do more\n", 0, false},
		{"only notes differ", "  when s listens a.A\n    s does it\n    note \"x\"\n  when s listens b.B\n    s does it\n", 1, true},
		{"external triggers", "  when O starts it\n    s does it\n  when O resumes it\n    s does it\n", 1, false},
		{"notes only body", "  when s listens a.A\n    note \"x\"\n  when s listens b.B\n    note \"x\"\n", 0, false},
	}
	for _, c := range cases {
		got := modelDiags(t, dupHead+c.body+"}\n", "craft/lint/duplicate-scenario-body")
		if len(got) != c.want {
			t.Errorf("%s: %d warnings, want %d", c.name, len(got), c.want)
			continue
		}
		if c.want == 1 {
			if fold := strings.Contains(got[0].Message, "when s listens a.A, b.B"); fold != c.fold {
				t.Errorf("%s: message %q", c.name, got[0].Message)
			}
			if got[0].Range.Start.Line == 5 {
				t.Errorf("%s: must point at the second scenario", c.name)
			}
		}
	}
}
```

Rendering (append to `sequence_rendering_test.go`):

```go
func TestSequence_SectionTitleShowsScope(t *testing.T) {
	src := `actors {
    system CRON
}
domain analytics {
    adstatistics
}
use_case "Rebuild a market's daily statistics" {
    tags {
        sites: imovirtualpt, otodompl, storiaro
        envs: production
    }
    when cron "0 1/2 * * *"
        adstatistics rebuilds the day's advert facts
}
use_case "Sites only" {
    tags {
        sites: "otodompl"
    }
    when cron "0 2 * * *"
        adstatistics rebuilds it
}
`
	puml, mmd := sequenceSource(t, src)
	title := "Rebuild a market's daily statistics [sites: imovirtualpt, otodompl, storiaro] [envs: production]"
	requireLine(t, "plantuml", puml, "== "+title+" ==")
	requireLine(t, "plantuml", puml, "== Sites only [sites: otodompl] ==")
	requireLine(t, "mermaid", mmd, "Note over CRON,adstatistics: "+title)
}
```

- [ ] **Step 2: Run** `go test ./internal/analysis/ ./internal/visualizer/ -run 'DuplicateScenarioBody|ShowsScope'` → FAIL.

- [ ] **Step 3: `LintModel`** (`internal/sema/lint_model.go`)

```go
// Package sema — LintModel: lints that read the merged, resolved document.
// They run after analysis has resolved participants (internal/analysis),
// because the syntax trees LintWorkspace reads cannot see across files.
package sema

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// LintModel runs the lints that need the resolved model. Every diagnostic
// carries the use case's SourceURI. The trees and line indexes are for the
// lints that point at a position the model does not keep, such as a tag
// value (Part B); the v2.24 lints read the model only.
func LintModel(doc *model.CraftDoc, perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	diags = append(diags, lintDuplicateScenarioBodies(doc)...)
	return diags
}

// ── duplicate-scenario-body ───────────────────────────────────────────────────
// Warning: two scenarios of one use case whose step trees are equal (spec
// §5.2). Notes (and, from v2.25, links) are ignored; the body must hold an
// action. The range is the second scenario's trigger subject.

func lintDuplicateScenarioBodies(doc *model.CraftDoc) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uc := range doc.UseCases {
		for j := 1; j < len(uc.Scenarios); j++ {
			b := uc.Scenarios[j]
			if len(b.Actions()) == 0 {
				continue
			}
			for i := 0; i < j; i++ {
				a := uc.Scenarios[i]
				if !stepsEqual(a.Steps, b.Steps) {
					continue
				}
				diags = append(diags, model.Diagnostic{
					Code:      "craft/lint/duplicate-scenario-body",
					Message:   duplicateBodyMessage(a.Trigger, b.Trigger),
					Severity:  model.SeverityWarning,
					SourceURI: uc.SourceURI,
					Range:     triggerRange(b.Trigger),
				})
				break
			}
		}
	}
	return diags
}

func duplicateBodyMessage(a, b model.Trigger) string {
	if a.Type == lang.TriggerDomainListen && b.Type == lang.TriggerDomainListen && a.Context == b.Context {
		items := append(append([]string{}, refOrEvents(a)...), refOrEvents(b)...)
		return fmt.Sprintf("the two scenarios run the same steps; fold them into one trigger: `when %s listens %s`", a.Context, strings.Join(items, ", "))
	}
	return "the two scenarios run the same steps; if they differ only by site, market or environment, write one scenario with a `sites`/`envs` tag; if the steps really differ, write the difference as an `alt`"
}

// refOrEvents is a listens trigger's items as an author writes them: a ref
// bare, a free-text event (one with a space) quoted.
func refOrEvents(t model.Trigger) []string {
	var out []string
	for _, ev := range t.EventList() {
		if strings.ContainsAny(ev, " \t") {
			ev = fmt.Sprintf("%q", ev)
		}
		out = append(out, ev)
	}
	return out
}

// triggerRange spans the trigger's subject (or `when` for an event trigger).
func triggerRange(t model.Trigger) model.Range {
	name := t.Context
	if t.Type == lang.TriggerExternal {
		name = t.Actor
	}
	start := colToLSP(t.Column)
	length := lspLen(name)
	if t.Column == 0 || name == "" {
		length = 4
	}
	return model.Range{
		Start: model.Position{Line: lineToLSP(t.Line), Character: start},
		End:   model.Position{Line: lineToLSP(t.Line), Character: start + length},
	}
}

// stepsEqual compares two step lists for duplicate-scenario-body: same
// shape; actions equal on Type, Context, Verb, TargetContext, Event,
// Connector, Phrase and Operation.Text; blocks equal on kind and labels.
// Notes are skipped on both sides.
func stepsEqual(a, b []model.Step) bool {
	a, b = withoutNotes(a), withoutNotes(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		switch {
		case x.Action != nil && y.Action != nil:
			if !actionsEqual(*x.Action, *y.Action) {
				return false
			}
		case x.Block != nil && y.Block != nil:
			if x.Block.Kind != y.Block.Kind || len(x.Block.Branches) != len(y.Block.Branches) {
				return false
			}
			for k := range x.Block.Branches {
				bx, by := x.Block.Branches[k], y.Block.Branches[k]
				if bx.Label != by.Label || !stepsEqual(bx.Steps, by.Steps) {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

func withoutNotes(steps []model.Step) []model.Step {
	var out []model.Step
	for _, st := range steps {
		if st.Action != nil || st.Block != nil {
			out = append(out, st)
		}
	}
	return out
}

func actionsEqual(x, y model.Action) bool {
	opText := func(o *model.Operation) string {
		if o == nil {
			return ""
		}
		return o.Text
	}
	return x.Type == y.Type && x.Context == y.Context && x.Verb == y.Verb && x.TargetContext == y.TargetContext &&
		x.Event == y.Event && x.Connector == y.Connector && x.Phrase == y.Phrase && opText(x.Operation) == opText(y.Operation)
}
```

- [ ] **Step 4: Wire it** (`analysis.Workspace`, after `resolveParticipants`):

```go
		modelDiags := sema.LintModel(res.Doc, perFileTrees, perFileLIs)
		wsDiags := append(append(append(append(mergeDiags, resolveDiags...), lintDiags...), commDiags...), modelDiags...)
```

- [ ] **Step 5: Scope titles** (`sequence.go`)

```go
// sectionTitle is a use case's section title, with its scope tags appended:
// `<name> [sites: a, b] [envs: c]` (spec §5.3). Values keep authored order.
func sectionTitle(uc model.UseCase) string {
	title := uc.Name
	for _, key := range []string{model.TagSites, model.TagEnvs} {
		if vals := uc.TagValues[key]; len(vals) > 0 {
			title += " [" + key + ": " + strings.Join(vals, ", ") + "]"
		}
	}
	return title
}
```

`BuildSequence` uses `SequenceSection{Title: sectionTitle(uc)}`.

- [ ] **Step 6: Run** → PASS; diagram goldens unchanged (no golden has `sites`/`envs`). Full suite.

- [ ] **Step 7: Commit**

```bash
git add internal
git commit -m "feat(lint): sema.LintModel over the resolved model; duplicate-scenario-body; sequence titles show sites/envs

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A6: Operation lists — **strong**

**Files:**
- Create: `internal/lang/opitems.go`, `internal/lang/opitems_test.go`, `testdata/corpus/04_use_cases/{operation_list,operation_list_path_comma}.craft` + goldens, `testdata/broken/operation_list_empty_item.craft` + `.diagnostics.json`
- Modify: `internal/syntax/parser.go` (`parseOpAnnotation`), `internal/analysis/project.go` (`projectAction`), `internal/interaction/interaction.go` (`Interaction.Operations`, `walker.action`), `internal/sema/surface.go` (`lintOperationNotInSurface`, new `opItemRange`)
- Test: `internal/sema/surface_test.go`, `internal/interaction/interaction_test.go`

- [ ] **Step 1: Write the failing tests** (`internal/lang/opitems_test.go`)

```go
package lang

import "testing"

func TestSplitOperationItems(t *testing.T) {
	cases := []struct {
		body string
		want []string
	}{
		{"CreateApiKey, CreateUsagePlanKey", []string{"CreateApiKey", "CreateUsagePlanKey"}},
		{"postAdvert, putAdvert, deleteAdvert", []string{"postAdvert", "putAdvert", "deleteAdvert"}},
		{"GET /nearest/paths/{latitude},{longitude}", []string{"GET /nearest/paths/{latitude},{longitude}"}},
		{`ADDR "https://a.b/x, y", SQL flat`, []string{`ADDR "https://a.b/x, y"`, "SQL flat"}},
		{"GET /a/{x, y}", []string{"GET /a/{x, y}"}},
		{"a, , b", []string{"a", "", "b"}},
		{"a, ", []string{"a", ""}},
		{"createSchedule", []string{"createSchedule"}},
	}
	for _, c := range cases {
		var got []string
		for _, it := range SplitOperationItems(c.body) {
			got = append(got, it.Text)
			if it.Text != "" && c.body[it.Start:it.Start+len(it.Text)] != it.Text {
				t.Errorf("%q: item %q has wrong Start %d", c.body, it.Text, it.Start)
			}
		}
		if len(got) != len(c.want) {
			t.Errorf("%q: got %q, want %q", c.body, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: got %q, want %q", c.body, got, c.want)
			}
		}
	}
}

func TestParseOperationItem(t *testing.T) {
	for _, c := range []struct{ in, verb, payload string }{
		{"POST /v1/x", "POST", "/v1/x"},
		{"createSchedule", "", "createSchedule"},
		{"POSTMAN x", "", "POSTMAN x"},
	} {
		v, p := ParseOperationItem(c.in)
		if v != c.verb || p != c.payload {
			t.Errorf("%q: got (%q, %q), want (%q, %q)", c.in, v, p, c.verb, c.payload)
		}
	}
}
```

`ADDR` and `SQL` become protocol verbs in Task A9, which adds their rows (`TestParseOperationItem_Egress`).

Interaction:

```go
func TestDerive_OperationListIsCopied(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Keys" {
    when Customer asks for a key
        Ordering asks Billing to create the key and bind it  [CreateApiKey, CreateUsagePlanKey]
}
`})
	req := its[1]
	if req.Operation == nil || req.Operation.Text != "CreateApiKey, CreateUsagePlanKey" || len(req.Operations) != 2 || req.Operations[1].Text != "CreateUsagePlanKey" {
		t.Errorf("request = %+v", req)
	}
}
```

Surface lint (append to `internal/sema/surface_test.go`, following its helpers):

```go
func TestLint_OperationNotInSurfaceChecksEachItem(t *testing.T) {
	src := "domain ad {\n  atlas\n  partner\n}\nsurface atlas {\n  callers: partner\n  command save the advert  [postAdvert]\n  command delete it  [deleteAdvert]\n}\nuse_case \"U\" {\n  when partner listens x.Y\n    partner asks atlas to save or delete the advert  [postAdvert, putAdvert, deleteAdvert]\n}"
	var got []model.Diagnostic
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/operation-not-in-surface" {
			got = append(got, d)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0].Message, "[putAdvert]") {
		t.Fatalf("got %v", got)
	}
	// "    partner asks atlas to save or delete the advert  [" is 54 columns; putAdvert starts after "postAdvert, ".
	if got[0].Range.Start.Character != 66 || got[0].Range.End.Character != 75 {
		t.Errorf("range = %+v", got[0].Range)
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/lang/ ./internal/interaction/ ./internal/sema/ -run 'OperationItem|OperationList|EachItem'` → FAIL.

- [ ] **Step 3: `lang`** (`internal/lang/opitems.go`)

```go
package lang

import "strings"

// OpItem is one item of an operation bracket: its text, trimmed, and the
// byte offset of that text in the bracket body (spec §8.1).
type OpItem struct {
	Text  string
	Start int
}

// SplitOperationItems splits a bracket body into its items: at a comma that
// is followed by whitespace, at brace depth 0, outside a "…" string.
// `{latitude},{longitude}` stays one item. An empty item (`a, , b`, `a, `)
// is returned with empty Text so the parser can report it. A body without a
// separator is one item.
func SplitOperationItems(body string) []OpItem {
	var out []OpItem
	depth, inString, start := 0, false, 0
	emit := func(end int) {
		raw := body[start:end]
		lead := len(raw) - len(strings.TrimLeft(raw, " \t"))
		out = append(out, OpItem{Text: strings.TrimSpace(raw), Start: start + lead})
	}
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case c == '"':
			inString = !inString
		case inString:
		case c == '{':
			depth++
		case c == '}' && depth > 0:
			depth--
		case c == ',' && depth == 0 && i+1 < len(body) && (body[i+1] == ' ' || body[i+1] == '\t'):
			emit(i)
			start = i + 1
		case c == ',' && depth == 0 && i+1 == len(body):
			// A trailing comma with nothing after it is not a separator.
		}
	}
	emit(len(body))
	return out
}

// ParseOperationItem splits one item into its protocol verb and payload, as
// the parser splits a whole bracket. An ADDR address loses its quotes.
func ParseOperationItem(text string) (verb, payload string) {
	head, rest, _ := strings.Cut(strings.TrimSpace(text), " ")
	if !IsProtocolVerb(head) {
		return "", strings.TrimSpace(text)
	}
	payload = strings.TrimSpace(rest)
	if head == OpVerbAddr {
		payload = UnquoteAddress(payload)
	}
	return head, payload
}

// UnquoteAddress drops the quotes of a quoted ADDR address.
func UnquoteAddress(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
```

   The body `"a, "` must give `["a", ""]`: the `, ` before the end is a separator because the body handed in keeps its trailing space (the parser passes the raw text between the brackets). `TrimSpace` is applied per item only.

- [ ] **Step 4: Parser** (`parseOpAnnotation`): after the closing `]` is consumed, check the items:

```go
	if rb := p.lastConsumed(); rb.Type == lexer.TokenRBracket && lbTok.End >= 0 && rb.Offset >= lbTok.End {
		body := p.src[lbTok.End:rb.Offset]
		if items := lang.SplitOperationItems(body); len(items) > 1 {
			for _, it := range items {
				if it.Text == "" {
					diags = append(diags, p.diagEmptyOpAnnotationItem(lbTok, rb))
					break
				}
			}
		}
	}
```

with

```go
// diagEmptyOpAnnotationItem reports an empty item of an operation list
// (`[a, , b]`) or an egress verb with nothing after it (`[ADDR]`), over the
// whole bracket.
func (p *Parser) diagEmptyOpAnnotationItem(lb, rb lexer.Token) model.Diagnostic {
	d := p.diagEmptyOpAnnotation(lb, rb)
	d.Message = "an operation bracket item is empty; write each item, e.g. [CreateApiKey, CreateUsagePlanKey], or [ADDR \"https://…\"] / [SQL <table>]"
	return d
}
```

   Task A9 moves this check after the `inferred` head and adds the egress case.

- [ ] **Step 5: Projection and interaction**

`projectAction`, after `operation` is built:

```go
	var operations []model.Operation
	if operation != nil {
		if items := lang.SplitOperationItems(operation.Text); len(items) > 1 {
			for _, it := range items {
				verb, payload := lang.ParseOperationItem(it.Text)
				operations = append(operations, model.Operation{Verb: verb, Payload: payload, Text: it.Text})
			}
		}
	}
```

and `Operations: operations` in the literal. `Interaction` gains `// Operations is copied from model.Action.Operations.` `Operations []model.Operation`; `walker.action` sets `it.Operations = a.Operations`.

- [ ] **Step 6: `operation-not-in-surface` per item** (`surface.go`)

Replace the single text check with a loop over `lang.SplitOperationItems(a.OpText())`: one item → today's check and range (`annotationRange`); two or more → check `CollapseOperation(it.Text)` per item and report it with

```go
// opItemRange is the range of one item of an action's bracket: the body
// starts at the first token after `[` that is neither whitespace nor the
// `inferred` marker, and the item sits it.Start bytes into the body text.
func opItemRange(op *syntax.SyntaxNode, it lang.OpItem, li green.LineIndex) model.Range {
	for _, tok := range op.AllTokens() {
		switch tok.Kind() {
		case syntax.SyntaxKindLBracket, syntax.SyntaxKindWhitespace, syntax.SyntaxKindKwInferred:
			continue
		}
		start := tok.Offset() + green.TextSize(it.Start)
		line, col := li.LineCol16(start)
		_, endCol := li.LineCol16(start + green.TextSize(len(it.Text)))
		return model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: colToLSP(col)},
			End:   model.Position{Line: lineToLSP(line), Character: colToLSP(endCol)},
		}
	}
	return model.Range{}
}
```

   The message names the item: ``fmt.Sprintf("`[%s]` is not an operation of the `%s` surface; …", item, s.subject)``. `repeated-request` keeps comparing the whole text (spec §8.4).

- [ ] **Step 7: Fixtures**

`testdata/corpus/04_use_cases/operation_list.craft`:

```craft
actors {
    system AwsApiGateway
}

domain ad {
    appmanager
    atlas
    partner
}

use_case "Partner keys and adverts" {
    when partner listens sns.partner-onboarded
        appmanager asks AwsApiGateway to create the key and bind it to the plan  [CreateApiKey, CreateUsagePlanKey]
        partner asks atlas to save or delete the advert  [postAdvert, putAdvert, deleteAdvert]
}
```

`operation_list_path_comma.craft`: one action `x asks y for the nearest paths  [GET /nearest/paths/{latitude},{longitude}]` (with `domain geo { x y }` and a listens trigger).

`testdata/broken/operation_list_empty_item.craft`:

```craft
domain keys {
    appmanager
    gateway
}

use_case "U" {
    when appmanager listens sns.key-requested
        appmanager asks gateway to create the key  [CreateApiKey, , CreateUsagePlanKey]
}
```

`.diagnostics.json`: `craft/syntax/empty-op-annotation`, error, `{"start":{"line":7,"character":51},"end":{"line":7,"character":87}}`.

- [ ] **Step 8: Goldens, full suite, commit**

`-update`: in `operation_list.craftjson` the first action keeps `"operation": {"payload": "CreateApiKey, CreateUsagePlanKey", "text": "CreateApiKey, CreateUsagePlanKey"}` and gains `"operations": [{"payload": "CreateApiKey", "text": "CreateApiKey"}, {"payload": "CreateUsagePlanKey", "text": "CreateUsagePlanKey"}]`. `operation_list_path_comma.craftjson` has no `operations`. `op_annotations.craftjson` and every other golden are unchanged (check `git status`).

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(operations): a comma list in one bracket is a list of operations (Action.Operations); operation-not-in-surface checks each

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A7: `notifies one of`: syntax, model, lints — **strong**

**Files:**
- Modify: `internal/syntax/parser.go` (`parseNotifiesAction`), `internal/syntax/ast.go` (new `ActionDecl.eventSlot`, `IsOneOf`, `EventItems`; `EventValue`, `EventCol`, `EventIsString`, `SourceTextWithoutOp`), `internal/syntax/model_views.go` (`ObjectRef`), `internal/analysis/project.go` (`projectAction`), `internal/sema/validate.go` (`validateEventLists`), `internal/sema/lint.go` (`lintDeadEvents`, `lintEventPastTense`, `lintStoreWithoutChange`, `lintDuplicateEventInList`, new `lintOneOfSingleEvent`)
- Create: `testdata/corpus/04_use_cases/notifies_one_of.craft` + golden, `testdata/broken/notifies_bare_list.craft` + `.diagnostics.json`
- Test: `internal/syntax/eventlist_test.go`, `internal/sema/lint_journey_test.go`

**Tree shape:** `Action > Ident(subject) KwNotifies KwOne KwOf (EventList | item) OpAnnotation?`. A plain `notifies E` keeps its v2.23 tree.

- [ ] **Step 1: Write the failing tests** (append to `eventlist_test.go`)

```go
func TestNotifiesOneOf(t *testing.T) {
	src := "use_case \"U\" {\n    when outbox listens stream.user_event_store\n        outbox notifies one of nmfbus.FeatureEnabled,\n                nmfbus.FeatureDisabled  [TOPIC nmfbus]\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	a := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Actions()[0]
	if !a.IsOneOf() || a.EventValue() != "nmfbus.FeatureEnabled" || a.ObjectRef() != "nmfbus.FeatureEnabled" {
		t.Errorf("oneOf=%v value=%q ref=%q", a.IsOneOf(), a.EventValue(), a.ObjectRef())
	}
	if got := itemValues(a.EventItems()); !slices.Equal(got, []string{"nmfbus.FeatureEnabled", "nmfbus.FeatureDisabled"}) {
		t.Errorf("items = %v", got)
	}
	if a.OpText() != "TOPIC nmfbus" {
		t.Errorf("op = %q", a.OpText())
	}
}

func TestNotifiesOneStaysAnEventName(t *testing.T) {
	for _, line := range []string{"x notifies one", "x notifies one [TOPIC t]", "one notifies of.Ended"} {
		g, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
		if len(diags) != 0 {
			t.Fatalf("%s: %v", line, diags)
		}
		a := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Actions()[0]
		if a.IsOneOf() {
			t.Errorf("%s: read as one of", line)
		}
	}
}

func TestNotifiesBareListIsAnError(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        x notifies a.A, b.B\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" || !strings.Contains(diags[0].Message, "notifies one of") {
		t.Fatalf("diags = %v", diags)
	}
}
```

(add `strings` to the imports). Lints:

```go
func TestLint_OneOfSingleEvent(t *testing.T) {
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended\n}", "craft/lint/one-of-single-event"); n != 1 {
		t.Errorf("single: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended, b.Ended\n}", "craft/lint/one-of-single-event"); n != 0 {
		t.Errorf("two: %d", n)
	}
	if n := lintCodes(t, "use_case \"U\" {\n  when O does\n    x notifies one of a.Ended, b.Ended, a.Ended\n}", "craft/lint/duplicate-event-in-list"); n != 1 {
		t.Errorf("duplicate: %d", n)
	}
}

func TestLint_OneOfItemsAreEachPublished(t *testing.T) {
	src := "use_case \"P\" {\n  when O does\n    x notifies one of a.Ended, b.Ended\n}\nuse_case \"L\" {\n  when y listens b.Ended\n    y does it\n}"
	var dead []string
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/lint/dead-event" {
			dead = append(dead, d.Message)
		}
	}
	if len(dead) != 1 || !strings.Contains(dead[0], "a.Ended") {
		t.Errorf("dead = %v", dead)
	}
}
```

- [ ] **Step 2: Run** → FAIL (`IsOneOf` undefined).

- [ ] **Step 3: Parser** (`parseNotifiesAction`)

```go
func (p *Parser) parseNotifiesAction() []model.Diagnostic {
	var diags []model.Diagnostic
	eventTok := p.peek()
	of := p.peekAt(1)
	switch {
	case eventTok.Type == lexer.TokenIdent && eventTok.Value == "one" &&
		of.Type == lexer.TokenIdent && of.Value == "of" && of.Line == eventTok.Line &&
		p.peekAt(2).Line == eventTok.Line && p.eventItemAhead(2):
		// notifies one of <event list>: one of these, chosen at run time.
		p.consumeAs(SyntaxKindKwOne)
		p.consumeAs(SyntaxKindKwOf)
		_, d := p.parseEventList()
		diags = append(diags, d...)
	case eventTok.Type == lexer.TokenString:
		p.consumeAs(SyntaxKindString)
	case eventTok.Type == lexer.TokenIdent:
		p.parseRef()
	case isAnyKeywordAsIdent(eventTok.Type):
		p.consumeAs(SyntaxKindIdent)
	case eventTok.Type == lexer.TokenError:
		diags = append(diags, p.diagUnterminatedString(eventTok))
		p.consumeAs(SyntaxKindError)
	}
	last := p.lastConsumed()
	if c := p.peek(); c.Type == lexer.TokenComma && c.Line == last.Line {
		// A bare list: say which form was meant, and keep the rest of the
		// line out of the next step.
		d := p.diagUnexpected(c, "end of line")
		d.Message = "to publish one of several events write `notifies one of a, b`; to publish several, write one `notifies` per event"
		diags = append(diags, d)
		p.builder.StartNode(SyntaxKindErrorNode)
		for t := p.peek(); t.Line == c.Line && t.Type != lexer.TokenEOF && t.Type != lexer.TokenRBrace; t = p.peek() {
			p.consume()
		}
		p.builder.FinishNode()
		return diags
	}
	// The bracket sits on the line of the last item (a list may wrap).
	if p.opAnnotationStart(last.Line) == 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
	return diags
}
```

(Task A9 renames the annotation call to `parseActionOpAnnotation`.)

- [ ] **Step 4: Views** (`ast.go`)

```go
// eventSlot returns an async action's event slot element: element 2, or
// element 4 after `one of`. Nil for any other action.
func (a ActionDecl) eventSlot() SyntaxElement {
	if a.Kind() != lang.ActionAsync {
		return nil
	}
	elems := significantElements(a.node)
	i := 2
	if a.IsOneOf() {
		i = 4
	}
	if i >= len(elems) {
		return nil
	}
	return elems[i]
}

// IsOneOf reports whether the action is `notifies one of …`.
func (a ActionDecl) IsOneOf() bool {
	elems := significantElements(a.node)
	return len(elems) > 2 && elems[2].Kind() == SyntaxKindKwOne
}

// EventItems returns the events of an async action in source order.
func (a ActionDecl) EventItems() []EventItem {
	if el := a.eventSlot(); el != nil {
		return eventItemsOf(el)
	}
	return nil
}
```

`EventValue`, `EventCol`, `EventIsString` and `ObjectRef`'s async case read `a.EventItems()[0]` (guarding an empty slice). `eventSourceText` (used by `SourceTextWithoutOp`) becomes:

```go
func (a ActionDecl) eventSourceText() string {
	elems := significantElements(a.node)
	if len(elems) < 3 {
		return ""
	}
	if a.IsOneOf() {
		// `one of` and the items as written, line breaks included.
		return a.rawTextFrom(tokenIndexAt(elems, 2))
	}
	return refAwareRawText(elems[2])
}
```

- [ ] **Step 5: Projection** (`projectAction`)

```go
	var events []string
	if kind == lang.ActionAsync && a.IsOneOf() {
		items := a.EventItems()
		quoted := make([]string, len(items))
		for i, it := range items {
			quoted[i] = fmt.Sprintf("%q", it.Value)
			if len(items) > 1 {
				events = append(events, it.Value)
			}
		}
		description = fmt.Sprintf("%s notifies one of %s", subject, strings.Join(quoted, ", "))
	}
```

after the description switch; `Events: events` in the literal; `store` only when `!a.IsOneOf()`.

- [ ] **Step 6: Lints**
  - `validateEventLists`: also `check(a.EventItems())` for every action with `a.IsOneOf()` (`sc.Actions()`).
  - `lintDeadEvents`: record every item of an async action as published, each at its own position (`li.LineCol16(it.Offset)`, `eventTokenLen(it.Value, it.IsString)`).
  - `lintEventPastTense`, `lintStoreWithoutChange`, `lintDuplicateEventInList`: loop over `a.EventItems()` for async actions, as Task A4 did for triggers.
  - New, registered in `LintWorkspace`:

```go
// ── one-of-single-event ───────────────────────────────────────────────────────
// Warning: `notifies one of` with a single event. The range is `one of`.

func lintOneOfSingleEvent(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if !a.IsOneOf() || len(a.EventItems()) != 1 {
						continue
					}
					one, of := a.OneOfTokens()
					line, start := li.LineCol16(one.Offset())
					_, end := li.LineCol16(of.Offset() + green.TextSize(len(of.Text())))
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/one-of-single-event",
						Message:   "`notifies one of` names a single event; drop `one of`",
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(line), Character: colToLSP(start)},
							End:   model.Position{Line: lineToLSP(line), Character: colToLSP(end)},
						},
					})
				}
			}
		}
	}
	return diags
}
```

   with `func (a ActionDecl) OneOfTokens() (one, of *SyntaxToken)` returning `a.node.ChildToken(SyntaxKindKwOne)` and `…(SyntaxKindKwOf)`.

- [ ] **Step 7: Fixtures**

`testdata/corpus/04_use_cases/notifies_one_of.craft`:

```craft
domain billing {
    nmf_monetization
}

use_case "nMF republishes a stored money record onto its bus" {
    when nmf_monetization listens stream.user_event_store
        nmf_monetization reads the detail-type off the stored column
        nmf_monetization notifies one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled,
                nmfbus.CreateAutoSchedule, nmfbus.SubAccountBundleUpdated
}
```

`testdata/broken/notifies_bare_list.craft`:

```craft
domain billing {
    outbox
}

use_case "U" {
    when outbox listens stream.user_event_store
        outbox notifies nmfbus.FeatureEnabled, nmfbus.FeatureDisabled
}
```

`.diagnostics.json`: `craft/syntax/unexpected-token`, error, `{"start":{"line":6,"character":45},"end":{"line":6,"character":46}}`.

- [ ] **Step 8: Goldens, full suite, commit**

`-update`: the third action of `notifies_one_of.craftjson` has `"event": "nmfbus.FeatureEnabled"`, `"ref": "nmfbus.FeatureEnabled"`, `"events": [the four]`, no `store`, and `"description": "nmf_monetization notifies one of \"nmfbus.FeatureEnabled\", \"nmfbus.FeatureDisabled\", \"nmfbus.CreateAutoSchedule\", \"nmfbus.SubAccountBundleUpdated\""`. No other golden changes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): notifies one of <event list> (Action.Events); lint one-of-single-event; event lints per item

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A8: `notifies one of`: interactions, merged publish, flow labels — **strong**

**Files:**
- Modify: `internal/interaction/interaction.go` (`Interaction.OneOf`, `Derive`, `walker.action`), `internal/visualizer/view/sequence.go` (`scenarioLayout.emit`, new `addOneOf`), `internal/visualizer/view/flow.go` (`BuildFlow` publish case)
- Test: `internal/interaction/interaction_test.go`, `internal/visualizer/sequence_rendering_test.go`, `internal/visualizer/view/sequence_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestDerive_NotifiesOneOfPublishesEachEventAtOneStep(t *testing.T) {
	its := derive(t, map[string]string{"a.craft": structure + `
use_case "Outbox" {
    when Customer pays
        Billing notifies one of shop.Paid, shop.Failed
}
use_case "Ship" {
    when Shipping listens shop.Paid
        Shipping ships it
}
`})
	var pubs []string
	for _, it := range its {
		if it.Kind == interaction.Publish {
			pubs = append(pubs, fmt.Sprintf("%d %s %v", it.Step, it.Event, it.OneOf))
		}
		if it.Kind == interaction.Listen && it.From.Name != "Billing" {
			t.Errorf("shop.Paid must pair with Billing: %+v", it)
		}
	}
	if want := []string{"1 shop.Paid true", "1 shop.Failed true"}; !slices.Equal(pubs, want) {
		t.Errorf("publishes = %v", pubs)
	}
}
```

Rendering (spec §8.6, correction 1):

```go
func TestSequence_NotifiesOneOfIsOneMessage(t *testing.T) {
	src := `domain billing {
    nmf_monetization
}
use_case "nMF republishes a stored money record onto its bus" {
    when nmf_monetization listens stream.user_event_store
        nmf_monetization reads the detail-type off the stored column
        nmf_monetization notifies one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled,
                nmfbus.CreateAutoSchedule, nmfbus.SubAccountBundleUpdated
}
`
	puml, mmd := sequenceSource(t, src)
	label := `notifies one of "nmfbus.FeatureEnabled", "nmfbus.FeatureDisabled", "nmfbus.CreateAutoSchedule" (+1 more)`
	requireLine(t, "plantuml", puml, "nmf_monetization ->> nmf_monetization : "+label)
	requireLine(t, "mermaid", mmd, "nmf_monetization-)nmf_monetization: "+label)
	if n := strings.Count(puml, "nmf_monetization ->> nmf_monetization"); n != 1 {
		t.Errorf("%d publish arrows, want 1", n)
	}
	doc := resolvedDoc(t, src)
	flow, _, _ := renderSource(doc, ViewFlow, PlantUML)
	if !strings.Contains(string(flow), "one of: nmfbus.CreateAutoSchedule") {
		t.Errorf("flow:\n%s", flow)
	}
}
```

`TestSequenceStepsMatchInteractionSteps`: add `Billing notifies one of shop.Paid, shop.Failed` as a step of the "batch order" scenario, and when building `want`, drop an interaction whose `Kind == Publish && OneOf` and whose `Step` equals the previous kept interaction's `Step` (one message per one-of action).

- [ ] **Step 2: Run** → FAIL (`OneOf` undefined).

- [ ] **Step 3: Interaction**

1. `Interaction` gains `// OneOf is set on each Publish of a "notifies one of": one of the action's events is published per run, chosen at run time.` `OneOf bool`.
2. `Derive`: the publishers loop reads `for _, ev := range a.EventList()` instead of `a.Event`.
3. `walker.action`, async case:

```go
	case lang.ActionAsync:
		events := a.EventList()
		if len(events) == 0 {
			return
		}
		for _, ev := range events {
			p := it
			p.Kind, p.Event, p.Label = Publish, ev, ev
			p.OneOf = len(a.Events) > 0
			p.Via = storeEndpoint(a.Store)
			p.Branch = slices.Clone(path)
			w.out = append(w.out, p)
		}
		return
```

- [ ] **Step 4: Sequence and flow**

```go
// addOneOf draws the publishes of one `notifies one of` action as one
// message on the publisher's lifeline (spec §8.6).
func (b *sequenceBuilder) addOneOf(idx int, pubs []interaction.Interaction, title string) {
	quoted := make([]string, len(pubs))
	for i, it := range pubs {
		quoted[i] = fmt.Sprintf("%q", it.Event)
	}
	from := b.ns.add(pubs[0].From)
	msg := Message{From: from, To: from, Kind: MessageAsync, Scenario: title, Branch: branchPath(pubs[0].Branch),
		Label: "notifies one of " + capList(quoted)}
	if op := pubs[0].Operation; op != nil && op.Text != "" {
		msg.Label += " [" + op.Text + "]"
	}
	b.appear(from)
	b.onStage[idx][from] = true
	b.seq.Sections[idx].Messages = append(b.seq.Sections[idx].Messages, msg)
}
```

   In `scenarioLayout.emit`, before the loop: `if its := l.bySteps[step]; step > 0 && len(its) > 0 && its[0].Kind == interaction.Publish && its[0].OneOf { l.b.addOneOf(l.idx, its, l.title); return }`.
   Flow, publish case (non-store): `label := it.Label; if it.OneOf { label = "one of: " + label }` and pass `label` to `add`.

- [ ] **Step 5: Run** → PASS; goldens unchanged; full suite.

- [ ] **Step 6: Commit**

```bash
git add internal
git commit -m "feat(interaction): one Publish per event of notifies one of; one sequence message; flow names each 'one of:'

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A9: `ADDR`/`SQL`/`inferred`: syntax, model, lints — **strong**

**Files:**
- Modify: `internal/lang/verbs.go` (`protocolVerbs`), `internal/lang/verbs_test.go`, `internal/lang/opitems_test.go`, `internal/syntax/parser.go` (`parseOpAnnotation` → `parseOpAnnotationWith`; new `parseActionOpAnnotation`, `checkOpItems`; the four action call sites), `internal/syntax/ast.go` (`opTextOf`, new `opInferredOf`, `ActionDecl.IsInferred`, `ActionDecl.SourceText`), `internal/analysis/project.go` (`projectAction`), `internal/interaction/interaction.go` (`Interaction.Inferred`), `internal/sema/lint.go` (new `lintEgressOnInternalStep`, `lintUnquotedAddress`)
- Create: `internal/syntax/opmarkers_test.go`, `testdata/corpus/04_use_cases/{op_sql,op_addr_quoted,op_inferred,op_inferred_only}.craft` + goldens, `testdata/broken/op_addr_empty.craft` + `.diagnostics.json`
- Test: `internal/sema/lint_journey_test.go`, `internal/interaction/interaction_test.go`

- [ ] **Step 1: Write the failing tests** (`internal/syntax/opmarkers_test.go`)

```go
package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func firstAction(t *testing.T, line string) syntax.ActionDecl {
	t.Helper()
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
	if len(diags) != 0 {
		t.Fatalf("%s: %v", line, diags)
	}
	return syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Actions()[0]
}

func TestOpMarkers(t *testing.T) {
	cases := []struct {
		line, text, verb string
		inferred         bool
		source           string
	}{
		{"g asks s to initiate the chat  [inferred POST /api/v1/x]", "POST /api/v1/x", "POST", true, "g asks s to initiate the chat [inferred POST /api/v1/x]"},
		{"g asks s to initiate the chat  [inferred]", "", "", true, "g asks s to initiate the chat [inferred]"},
		{"o asks f for the graph  [SQL investment flat]", "SQL investment flat", "SQL", false, ""},
		{`o asks s to take it  [ADDR "SNAPSHOT_DESTINATION"]`, `ADDR "SNAPSHOT_DESTINATION"`, "ADDR", false, ""},
		{"o asks s to take it  [inferred, x]", "inferred, x", "", false, ""},
	}
	for _, c := range cases {
		a := firstAction(t, c.line)
		if a.OpText() != c.text || a.OpVerb() != c.verb || a.IsInferred() != c.inferred {
			t.Errorf("%s: text %q verb %q inferred %v", c.line, a.OpText(), a.OpVerb(), a.IsInferred())
		}
		if c.source != "" && a.SourceText() != c.source {
			t.Errorf("%s: SourceText %q", c.line, a.SourceText())
		}
	}
}

func TestOpMarkers_InferredIsAPayloadWordOnReceivesAndSurfaces(t *testing.T) {
	g, _, diags := syntax.Parse("use_case \"U\" {\n    when g receives a sweep  [inferred x]\n        g does it\n}\nsurface g {\n    query the list  [inferred y]\n}")
	if len(diags) != 0 {
		t.Fatalf("%v", diags)
	}
	f := syntax.AsFile(syntax.Root(g))
	if got := f.UseCases()[0].Scenarios()[0].Trigger().OpText(); got != "inferred x" {
		t.Errorf("receives op = %q", got)
	}
	if got := f.Surfaces()[0].Operations()[0].OpText(); got != "inferred y" {
		t.Errorf("surface op = %q", got)
	}
}

func TestOpMarkers_EmptyEgress(t *testing.T) {
	for _, line := range []string{"o asks s to take it  [ADDR]", "o asks s for it  [SQL]", "o asks s for it  [a, SQL]"} {
		_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
		if len(diags) != 1 || diags[0].Code != "craft/syntax/empty-op-annotation" {
			t.Errorf("%s: %v", line, diags)
		}
	}
}
```

In the last case `inferred` is followed by a comma: it is the bracket head only when whitespace or `]` follows it, so there it is payload. `opitems_test.go` gains:

```go
func TestParseOperationItem_Egress(t *testing.T) {
	for _, c := range []struct{ in, verb, payload string }{
		{`ADDR "https://a.b/c"`, "ADDR", "https://a.b/c"},
		{"ADDR api.example.com", "ADDR", "api.example.com"},
		{"SQL investment flat", "SQL", "investment flat"},
	} {
		if v, p := ParseOperationItem(c.in); v != c.verb || p != c.payload {
			t.Errorf("%q: got (%q, %q)", c.in, v, p)
		}
	}
}
```

Lints:

```go
func TestLint_EgressOnInternalStep(t *testing.T) {
	bad := "use_case \"U\" {\n  when O does\n    o reads the tables  [SQL investment]\n    o returns the rows  [ADDR api.example.com]\n}"
	good := "use_case \"U\" {\n  when O does\n    o asks front for the tables  [SQL investment]\n}"
	if n := lintCodes(t, bad, "craft/lint/egress-on-internal-step"); n != 2 {
		t.Errorf("bad: %d", n)
	}
	if n := lintCodes(t, good, "craft/lint/egress-on-internal-step"); n != 0 {
		t.Errorf("good: %d", n)
	}
}

func TestLint_UnquotedAddress(t *testing.T) {
	cut := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR //cdn.example.com/x]\n}"
	whole := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR https://cdn.example.com/x]\n}"
	quoted := "use_case \"U\" {\n  when O does\n    o asks s to take it  [ADDR \"//cdn.example.com/x\"]\n}"
	if n := lintCodes(t, cut, "craft/lint/unquoted-address"); n != 1 {
		t.Errorf("cut: %d", n)
	}
	for _, src := range []string{whole, quoted} {
		if n := lintCodes(t, src, "craft/lint/unquoted-address"); n != 0 {
			t.Errorf("%d:\n%s", n, src)
		}
	}
}
```

Interaction: a scenario with `Ordering asks Billing to charge  [inferred charge]` yields a `Request` with `Inferred == true` and `Operation.Text == "charge"`.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Vocabulary.** `protocolVerbs` gains `"ADDR": true, "SQL": true`. Update the expected list in `verbs_test.go` (sorted: `ADDR, DELETE, GET, GRPC, HEAD, OPTIONS, PATCH, POST, PUT, QUERY, SQL, TOPIC`).

- [ ] **Step 4: Parser**

```go
// parseOpAnnotation parses the bracket of a receives trigger or a surface
// line, where `inferred` is an ordinary payload word (spec §9.2).
func (p *Parser) parseOpAnnotation() []model.Diagnostic { return p.parseOpAnnotationWith(false) }

// parseActionOpAnnotation parses an action's bracket: `inferred` may head it.
func (p *Parser) parseActionOpAnnotation() []model.Diagnostic { return p.parseOpAnnotationWith(true) }
```

`parseOpAnnotationWith(allowInferred bool)` is today's body with:
- after `lbTok := p.consumeAs(SyntaxKindLBracket)`:

```go
	bodyStart := lbTok.End
	if t, next := p.peek(), p.peekAt(1); allowInferred && t.Type == lexer.TokenIdent && t.Value == lang.OpInferred &&
		(next.Type == lexer.TokenRBracket || next.Offset > t.End) {
		bodyStart = p.consumeAs(SyntaxKindKwInferred).End
	}
```

- the empty `[]` check runs only when `bodyStart == lbTok.End` (so `[inferred]` is legal);
- after the closing bracket, `diags = append(diags, p.checkOpItems(lbTok, rb, p.src[bodyStart:rb.Offset])...)`, replacing Task A6's inline check:

```go
// checkOpItems reports, once per bracket, an empty item of a list or an
// ADDR/SQL item with nothing after the verb (spec §8.4, §9.3).
func (p *Parser) checkOpItems(lb, rb lexer.Token, body string) []model.Diagnostic {
	items := lang.SplitOperationItems(body)
	for _, it := range items {
		if it.Text == "" && len(items) > 1 {
			return []model.Diagnostic{p.diagEmptyOpAnnotationItem(lb, rb)}
		}
		if verb, payload := lang.ParseOperationItem(it.Text); (verb == lang.OpVerbAddr || verb == lang.OpVerbSQL) && payload == "" {
			return []model.Diagnostic{p.diagEmptyOpAnnotationItem(lb, rb)}
		}
	}
	return nil
}
```

The four action sites (`parseAction`'s internal default, `parseAsksAction`, `parseNotifiesAction`, `parseReturnsAction`) call `parseActionOpAnnotation`; the receives trigger and `parseSurfaceOperation` keep `parseOpAnnotation`.

- [ ] **Step 5: Views** (`ast.go`)
  - `opTextOf` also skips `SyntaxKindKwInferred` tokens (the `TrimSpace` drops the space after it).
  - `func opInferredOf(n SyntaxNode) bool { op := opAnnotationOf(n); return op != nil && op.ChildToken(SyntaxKindKwInferred) != nil }`; `func (a ActionDecl) IsInferred() bool { return opInferredOf(a.node) }`.
  - `ActionDecl.SourceText` keeps the marker:

```go
func (a ActionDecl) SourceText() string {
	body := a.SourceTextWithoutOp()
	op := a.OpText()
	if a.IsInferred() {
		op = strings.TrimSpace(lang.OpInferred + " " + op)
	}
	if op != "" {
		return body + " [" + op + "]"
	}
	return body
}
```

- [ ] **Step 6: Projection and interaction**
  - `projectAction`: `Inferred: a.IsInferred()`; after `operation` is built, `if operation != nil && operation.Verb == lang.OpVerbAddr { operation.Payload = lang.UnquoteAddress(operation.Payload) }`. `[inferred]` alone has `OpText() == ""`, so `Operation` stays nil.
  - `Interaction` gains `// Inferred is copied from the action: the hop was inferred, not read in code.` `Inferred bool`; `walker.action` sets `it.Inferred = a.Inferred` before the switch, so every interaction of the action carries it (the `one of` copies too).

- [ ] **Step 7: Lints** (`lint.go`, both registered in `LintWorkspace`)

```go
// ── egress-on-internal-step ───────────────────────────────────────────────────
// Warning: an internal or return action whose bracket has an ADDR or SQL
// item. Both name a call to another party, so the step should be an asks.

func lintEgressOnInternalStep(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if k := a.Kind(); k != lang.ActionInternal && k != lang.ActionReturn {
						continue
					}
					for _, it := range lang.SplitOperationItems(a.OpText()) {
						if verb, _ := lang.ParseOperationItem(it.Text); verb == lang.OpVerbAddr || verb == lang.OpVerbSQL {
							diags = append(diags, model.Diagnostic{
								Code:      "craft/lint/egress-on-internal-step",
								Message:   fmt.Sprintf("`[%s …]` names a call to another party; write `%s asks <target> … [%s …]`, declaring the target as a `system` actor if it is outside the estate", verb, a.SubjectName(), verb),
								Severity:  model.SeverityWarning,
								SourceURI: uri,
								Range:     annotationRange(a.OpAnnotation(), li, true),
							})
							break
						}
					}
				}
			}
		}
	}
	return diags
}

// ── unquoted-address ──────────────────────────────────────────────────────────
// Warning: an action whose phrase ends in `[ADDR` because a `//` after a
// space opened a comment and cut the bracket (spec §9.3; plan correction 2).

func lintUnquotedAddress(perFileTrees map[string]syntax.SyntaxNode, lis map[string]green.LineIndex) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, uri := range sortedTreeURIs(perFileTrees) {
		li := lis[uri]
		src := li.Src()
		for _, uc := range syntax.AsFile(perFileTrees[uri]).UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, a := range sc.Actions() {
					if a.OpAnnotation() != nil || !strings.HasSuffix(strings.TrimSpace(a.PhraseText()), "[ADDR") {
						continue
					}
					line := a.Line(li)
					start := int(li.OffsetFromUTF16(line, 0))
					end := strings.IndexByte(src[start:], '\n')
					if end < 0 {
						end = len(src) - start
					}
					text := src[start : start+end]
					at := strings.LastIndex(text, "[ADDR")
					if at < 0 || !strings.Contains(text[at:], "//") {
						continue
					}
					col := lspLen(text[:at])
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/unquoted-address",
						Message:   "the address was cut by a `//` comment; quote it: `[ADDR \"//…\"]`",
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(line), Character: col},
							End:   model.Position{Line: lineToLSP(line), Character: col + len("[ADDR")},
						},
					})
				}
			}
		}
	}
	return diags
}
```

- [ ] **Step 8: Fixtures**

`testdata/corpus/04_use_cases/op_sql.craft` and `op_addr_quoted.craft`: the first spec §9.1 example split in two (the `SQL` action, and the `ADDR "SNAPSHOT_DESTINATION"` action), each with `actors { system CRON  system ObjectStore }` (one per line) and `domain obido { obidoanalytics  front }`. `op_inferred.craft`: the second §9.1 example with `domain seller { mobile  graphql }`. `op_inferred_only.craft`: the same with `[inferred]`.

`testdata/broken/op_addr_empty.craft`:

```craft
actors {
    system ObjectStore
}

domain obido {
    obidoanalytics
}

use_case "U" {
    when obidoanalytics listens sns.market-refreshed
        obidoanalytics asks ObjectStore to take the snapshot  [ADDR]
}
```

`.diagnostics.json`: `craft/syntax/empty-op-annotation`, error, `{"start":{"line":10,"character":62},"end":{"line":10,"character":68}}`.

- [ ] **Step 9: Goldens, full suite, commit**

`-update`: `op_sql.craftjson` has `"operation": {"verb": "SQL", "payload": "investment flat city region developer", "text": "SQL investment flat city region developer"}`; `op_addr_quoted.craftjson` `{"verb": "ADDR", "payload": "SNAPSHOT_DESTINATION", "text": "ADDR \"SNAPSHOT_DESTINATION\""}`; `op_inferred.craftjson` `"inferred": true` and `"operation": {"verb": "POST", "payload": "/api/v1/assistant/initiate_chat", "text": "POST /api/v1/assistant/initiate_chat"}`; `op_inferred_only.craftjson` `"inferred": true` and no `operation`. `op_annotations.craftjson` is unchanged (check it holds no `ADDR`/`SQL` head word: `grep -c 'ADDR\|SQL' testdata/corpus/04_use_cases/op_annotations.craft` → 0).

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(operations): ADDR and SQL protocol verbs, the inferred bracket marker; lints egress-on-internal-step and unquoted-address

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A10: Inferred and egress rendering — **cheap**

**Files:**
- Modify: `internal/visualizer/view/sequence.go` (`Message.Inferred`, `add`, new `opLabel`), `internal/visualizer/view/flow.go` (`FlowEdge.Inferred`, `BuildFlow`), `internal/visualizer/view/c4.go` (`relateServices`, new `requestTechnology`), `internal/visualizer/plantuml/plantuml.go` (`Sequence`, `flowArrow`), `internal/visualizer/mermaid/views.go` (no arrow change)
- Test: `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Write the failing tests** (spec §9.5)

```go
const egressSrc = `actors {
    system CRON
    system ObjectStore
}
domain obido {
    obidoanalytics
    front
}
use_case "obido analytics refreshes the market structure" {
    when cron "0 18 * * *"
        obidoanalytics asks front for the city, region and developer graph  [SQL investment flat city region developer]
        obidoanalytics asks ObjectStore to take the refreshed snapshot  [ADDR "SNAPSHOT_DESTINATION"]
}
`

const inferredSrc = `actors {
    user Seller
    system ShaperAgent
}
domain seller {
    mobile
    graphql
}
use_case "Seller resumes a selling draft" {
    when Seller resumes a selling draft
        mobile asks graphql to open a selling chat session  [agenticSellingStart]
        graphql asks ShaperAgent to initiate the chat  [inferred POST /api/v1/assistant/initiate_chat]
}
`

func TestSequence_EgressAndInferred(t *testing.T) {
	puml, _ := sequenceSource(t, egressSrc)
	requireLine(t, "plantuml", puml, "obidoanalytics -> front : for the city, region and developer graph [SQL investment flat city region developer]")
	requireLine(t, "plantuml", puml, "obidoanalytics -> ObjectStore : to take the refreshed snapshot [ADDR SNAPSHOT_DESTINATION]")
	puml, mmd := sequenceSource(t, inferredSrc)
	requireLine(t, "plantuml", puml, "graphql -[#999999]> ShaperAgent : to initiate the chat [POST /api/v1/assistant/initiate_chat] (inferred)")
	requireLine(t, "mermaid", mmd, "graphql->>ShaperAgent: to initiate the chat [POST /api/v1/assistant/initiate_chat] (inferred)")
	flow, _, _ := renderSource(resolvedDoc(t, inferredSrc), ViewFlow, PlantUML)
	if !strings.Contains(string(flow), "-[#999999]->> ShaperAgent : 3. to initiate the chat (inferred)") {
		t.Errorf("flow:\n%s", flow)
	}
}
```

A C4 test: a model with two services, `a asks b for rows  [SQL t1 t2]` and `a asks b for x  [inferred getX]` between their contexts, renders `Rel(…, "for rows", "SQL (shared tables)")` and a relation label `for x (inferred)` with technology `Service API`.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement**
  1. `view.Message` gains `// Inferred draws the message grey, with an "(inferred)" suffix.` `Inferred bool`. In `add`, replace the operation suffix with:

```go
	if op := opLabel(it); op != "" {
		msg.Label = strings.TrimSpace(msg.Label + " [" + op + "]")
	}
	if it.Inferred {
		msg.Label += " (inferred)"
		msg.Inferred = true
	}
```

```go
// opLabel is the bracket a label ends with: each operation as written,
// except that an ADDR address drops its quotes (spec §9.5).
func opLabel(it interaction.Interaction) string {
	ops := it.Operations
	if len(ops) == 0 && it.Operation != nil {
		ops = []model.Operation{*it.Operation}
	}
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		if op.Verb == lang.OpVerbAddr {
			parts = append(parts, strings.TrimSpace(op.Verb+" "+op.Payload))
			continue
		}
		parts = append(parts, op.Text)
	}
	return strings.Join(parts, ", ")
}
```

     (`addOneOf` from Task A8 uses `opLabel(pubs[0])` too.)
  2. PlantUML sequence arrows:

```go
// seqArrow is the PlantUML arrow of a message: grey when inferred.
func seqArrow(arrow string, inferred bool) string {
	if !inferred {
		return arrow
	}
	return "-[#999999]" + strings.TrimPrefix(arrow, "-")
}
```

     used for the `-->` reply, `->>` async and `->` default cases (`seqArrow("->", m.Inferred)` gives `-[#999999]>`, `seqArrow("-->", …)` gives `-[#999999]->`).
  3. Flow: `FlowEdge` gains `Inferred bool`. In `BuildFlow` keep a per-interaction `inferred := it.Inferred` next to `branch`, and in `add` append `" (inferred)"` to the label and set the field. PlantUML `flowArrow(s, inferred)` returns `"-[#999999]" + strings.TrimPrefix(arrow, "-")` when inferred; Mermaid flow uses the label suffix only.
  4. C4 (`relateServices`, request case):

```go
				label := it.Label
				if it.Inferred {
					label += " (inferred)"
				}
				b.relate(b.containerOf(it.From), b.containerOf(it.To), label, requestTechnology(it))
```

```go
// requestTechnology names a service-to-service request: shared tables for
// a SQL item, a bare address for an ADDR item, else a service API.
func requestTechnology(it interaction.Interaction) string {
	ops := it.Operations
	if len(ops) == 0 && it.Operation != nil {
		ops = []model.Operation{*it.Operation}
	}
	for _, op := range ops {
		switch op.Verb {
		case lang.OpVerbSQL:
			return "SQL (shared tables)"
		case lang.OpVerbAddr:
			return "HTTP (bare address)"
		}
	}
	return "Service API"
}
```

- [ ] **Step 4: Run** → PASS; diagram goldens unchanged; full suite.
- [ ] **Step 5: Commit** `feat(diagrams): inferred hops draw grey with "(inferred)"; ADDR labels drop quotes; C4 names SQL and ADDR technologies`.

---

### Task A11: `returns stream`: syntax, model, `Answers`, `stream-without-caller` — **strong**

**Files:**
- Modify: `internal/syntax/parser.go` (`parseReturnsAction`), `internal/syntax/ast.go` (new `returnBase`, `IsStream`; `targetElement`, `slotEndIndex`, `SourceTextWithoutOp`), `internal/analysis/project.go` (`projectAction`), `internal/analysis/analysis.go` (`Workspace`), `internal/interaction/interaction.go` (`frame.request`, `Interaction.Stream`, `label`, `walker`, new `derive`, `AnsweredRequests`, `openRequest`), `internal/sema/lint.go` (`LintCommunication`, new `lintStreamWithoutCaller`)
- Create: `internal/syntax/stream_test.go`, `testdata/corpus/04_use_cases/{returns_stream,returns_stream_to}.craft` + goldens
- Test: `internal/interaction/interaction_test.go`, `internal/analysis/communication_lint_test.go`

**Tree shape:** `Action > Ident KwReturns KwStream (KwTo target)? connector? phrase OpAnnotation?`.

- [ ] **Step 1: Write the failing tests** (`internal/syntax/stream_test.go`)

```go
package syntax_test

import "testing"

func TestReturnsStream(t *testing.T) {
	a := firstAction(t, "AiAssistant returns stream the reply as META, TOKEN and DONE events")
	if !a.IsStream() || a.PhraseText() != "the reply as META, TOKEN and DONE events" || a.TargetName() != "" {
		t.Errorf("stream=%v phrase=%q target=%q", a.IsStream(), a.PhraseText(), a.TargetName())
	}
	a = firstAction(t, "graphql returns stream to frontendplatform each event")
	if !a.IsStream() || a.TargetName() != "frontendplatform" || a.PhraseText() != "each event" {
		t.Errorf("to: stream=%v target=%q phrase=%q", a.IsStream(), a.TargetName(), a.PhraseText())
	}
	if got := a.SourceText(); got != "graphql returns stream to frontendplatform each event" {
		t.Errorf("SourceText = %q", got)
	}
	if a := firstAction(t, "graphql returns streams of data"); a.IsStream() {
		t.Error("`streams` is a phrase word")
	}
	if a := firstAction(t, "stream returns the data"); a.IsStream() || a.SubjectName() != "stream" {
		t.Error("`stream` stays a valid subject")
	}
}
```

Interaction:

```go
func TestDerive_StreamAnswersTheRequestThatOpenedIt(t *testing.T) {
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("a.craft", structure+`
use_case "Chat" {
    when Customer asks a question
        Ordering asks Billing to stream the answer  [streamAnswer]
        Billing returns stream the answer
        Ordering returns stream to Customer each token
        Shipping returns stream nothing
}
`)}).Doc
	acts := doc.UseCases[0].Scenarios[0].Actions()
	if acts[1].Answers != acts[0].ID {
		t.Errorf("Billing's stream answers %q, want %q", acts[1].Answers, acts[0].ID)
	}
	if acts[2].Answers != "" || acts[3].Answers != "" {
		t.Errorf("answers = %q, %q", acts[2].Answers, acts[3].Answers)
	}
	for _, it := range interaction.Derive(doc) {
		if it.Kind == interaction.Response && (!it.Stream || !strings.HasPrefix(it.Label, "streams ")) {
			t.Errorf("response = %+v", it)
		}
	}
}
```

(`Ordering returns stream to Customer`: no open request from Customer to Ordering exists, because the trigger frame is caller Customer → callee Ordering, and it is the one it answers: per correction 12 `openRequest` finds that trigger frame, whose `request` is `""`.) Lint (append to `internal/analysis/communication_lint_test.go`, through `Workspace`): a scenario `when Shipping listens shop.Paid` / `Shipping returns stream the label` (a listens trigger pushes no frame, so the reply answers `External`) gives one `craft/lint/stream-without-caller` on the line of the return; `Billing returns stream the answer` after `Ordering asks Billing …` gives none. (In an externally triggered scenario an untargeted reply answers the trigger's actor, so the lint does not fire there.)

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Parser** (`parseReturnsAction`, first lines)

```go
	if t := p.peek(); t.Type == lexer.TokenIdent && t.Value == "stream" && t.Line == line {
		p.consumeAs(SyntaxKindKwStream)
	}
```

- [ ] **Step 4: Views** (`ast.go`)

```go
// returnBase is the element index right after `returns` (and `stream`).
func returnBase(elems []SyntaxElement) int {
	if len(elems) > 2 && elems[2].Kind() == SyntaxKindKwStream {
		return 3
	}
	return 2
}

// IsStream reports whether the action is `returns stream …`.
func (a ActionDecl) IsStream() bool {
	elems := significantElements(a.node)
	return a.Kind() == lang.ActionReturn && returnBase(elems) == 3
}
```

`targetElement`'s return case reads `b := returnBase(elems); if len(elems) > b && elems[b].Kind() == SyntaxKindKwTo { i = b + 1 }`; `slotEndIndex`'s return case returns `tokenIndexAt(elems, b+2)` with a target and `tokenIndexAt(elems, b)` without; `SourceTextWithoutOp` writes `" returns"`, then `" stream"` when `a.IsStream()`, then the target and phrase as today.

- [ ] **Step 5: Projection.** `Stream: a.IsStream()`. Description: `subject + " returns stream " + phrase` (and `" to " + target`) when streaming, else today's text.

- [ ] **Step 6: Interaction**

1. `frame` gains `request string // the asks action's ID; "" for a trigger frame`; `walker.action`'s sync case pushes `frame{caller: it.From, callee: it.To, request: a.ID}`.
2. `answer` also returns the popped frame: `func answer(stack []frame, returner Endpoint) (Endpoint, []frame, frame)`.
3. `Interaction` gains `// Stream is set on a Response from a "returns stream".` `Stream bool`; `label` returns `"streams " + …` for a streaming return.
4. `walker` gains `answers map[*model.Action]string` (nil in `Derive`) and `answered string`; `action`'s return case sets `w.answered` from the popped frame (untargeted) or from `openRequest(w.callStack, it.To, it.From)` (targeted), and `it.Stream = a.Stream`. `walk`, after `w.action(*st.Action, path)`: `if w.answers != nil && st.Action.Stream { w.answers[st.Action] = w.answered }`.

```go
// openRequest returns the request ID of the most recent open call from
// caller to callee, without settling it; "" when there is none.
func openRequest(stack []frame, caller, callee Endpoint) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].caller.Key == caller.Key && stack[i].callee.Key == callee.Key {
			return stack[i].request
		}
	}
	return ""
}

// AnsweredRequests maps every `returns stream` action of doc to the ID of
// the request it answers (see model.Action.Answers), by the same call-stack
// walk Derive uses. The keys are the doc's own *Action pointers.
func AnsweredRequests(doc *model.CraftDoc) map[*model.Action]string {
	answers := map[*model.Action]string{}
	derive(doc, answers)
	return answers
}
```

   `Derive` becomes `func Derive(doc *model.CraftDoc) []Interaction { return derive(doc, nil) }`, with today's body in `derive`, threading `answers` into `deriveScenario` and the walker literal.
5. `analysis.Workspace`, right after `resolveParticipants(res.Doc, ws)`:

```go
		for a, id := range interaction.AnsweredRequests(res.Doc) {
			a.Answers = id
		}
```

- [ ] **Step 7: Lint** (`lint.go`, called from `LintCommunication`)

```go
// ── stream-without-caller ─────────────────────────────────────────────────────
// Warning: a `returns stream` with no caller on the stack and no `to`
// target: the stream answers nobody in the model.

func lintStreamWithoutCaller(its []interaction.Interaction) []model.Diagnostic {
	var diags []model.Diagnostic
	for _, it := range its {
		if it.Kind != interaction.Response || !it.Stream || it.Targeted || it.To != interaction.External {
			continue
		}
		start := colToLSP(it.Site.Column)
		diags = append(diags, model.Diagnostic{
			Code:      "craft/lint/stream-without-caller",
			Message:   "a stream answers the request that opened it; write the `asks` that opens the stream before it, or name the reader with `returns stream to <reader>`",
			Severity:  model.SeverityWarning,
			SourceURI: it.Site.URI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(it.Site.Line), Character: start},
				End:   model.Position{Line: lineToLSP(it.Site.Line), Character: start + it.Site.Length},
			},
		})
	}
	return diags
}
```

- [ ] **Step 8: Fixtures.** `returns_stream.craft`: the spec §7.1 example with `domain seller { frontendplatform  graphql }` (one per line). `returns_stream_to.craft`: `graphql returns stream to frontendplatform each event` after `frontendplatform asks graphql to stream it  [assistantStream]`.

- [ ] **Step 9: Goldens, full suite, commit**

`-update`: in `returns_stream.craftjson` the `AiAssistant` action has `"stream": true`, `"answers": "action_<id of graphql asks AiAssistant>"`, `"description": "AiAssistant returns stream the reply as META, TOKEN and DONE events"`; the `graphql` one answers the `frontendplatform asks graphql` action. No existing return gains a key.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): returns stream — Action.Stream, Action.Answers from the call stack; lint stream-without-caller

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task A12: Stream rendering — **cheap**

**Files:** Modify `internal/visualizer/view/sequence.go` (`Message.Stream`, `add`), `internal/visualizer/plantuml/plantuml.go` (`Sequence`), `internal/visualizer/mermaid/views.go` (`Sequence`), `internal/visualizer/view/c4.go` (`relateServices`); Test `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Failing tests** (spec §7.5)

```go
const streamSrc = `actors {
    user Seller
    system AiAssistant
}
domain seller {
    frontendplatform
    graphql
}
use_case "Seller asks the assistant a question" {
    when Seller asks the assistant a question
        frontendplatform asks graphql to stream the answer  [assistantStream]
        graphql asks AiAssistant to stream the reply  [POST /api/v1/assistant/chat_streaming]
        AiAssistant returns stream the reply as META, TOKEN and DONE events
        graphql returns stream each event to the client
        frontendplatform appends every TOKEN delta to the message
}
`

func TestSequence_StreamIsALoopBox(t *testing.T) {
	puml, mmd := sequenceSource(t, streamSrc)
	wantP := "loop stream\nAiAssistant --> graphql : streams the reply as META, TOKEN and DONE events\nend\nloop stream\ngraphql --> frontendplatform : streams each event to the client\nend\n"
	if !strings.Contains(puml, wantP) {
		t.Errorf("plantuml:\n%s", puml)
	}
	wantM := "    loop stream\n    AiAssistant-->>graphql: streams the reply as META, TOKEN and DONE events\n    end\n"
	if !strings.Contains(mmd, wantM) {
		t.Errorf("mermaid:\n%s", mmd)
	}
}
```

Also: the stream inside an `alt` branch renders `alt …`, `loop stream`, the reply, `end`, `end`; a `receives` scenario whose callee `returns stream` renders `loop stream` / `graphql -->] : streams …` / `end`.

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** `view.Message` gains `// Stream wraps the message in its own one-message "loop stream" box.` `Stream bool`; `add` sets `msg.Stream = it.Stream` for a `Response`. PlantUML `Sequence`: before writing a message with `m.Stream`, `b.WriteString("loop stream\n")`; after it, `b.WriteString("end\n")`. Mermaid: the same with the four-space indent. The box wraps the message only, inside whatever block ops were already written, so `BlockOps` is untouched. C4 `relateServices` response case: technology `"Return Data (stream)"` when `it.Stream`, else `"Return Data"`. Flow needs nothing (the label already reads `streams …`).
- [ ] **Step 4: Run** → PASS; goldens unchanged; full suite.
- [ ] **Step 5: Commit** `feat(sequence): a streamed reply draws in its own loop stream box`.

---

### Task A13: Formatter and fuzz — **strong**

**Files:** Test `internal/lsp/formatter_test.go`, `internal/lsp/formatter_fuzz_test.go`, `internal/lsp/formatter_corpus_test.go`, `internal/syntax/usecase_body_fuzz_test.go`; Modify `internal/lsp/formatter.go` only if a test shows a defect (see Step 3)

- [ ] **Step 1: Failing tests** (append to `formatter_test.go`)

```go
func TestFormat_EventListsHangOneUnit(t *testing.T) {
	in := "use_case \"U\" {\n  when seshat listens sns.a,   sns.b,\n   sns.c,\n        // the rest\n  sns.d every   \"5m\"\n      seshat writes the row\n      outbox notifies one of x.A,\n  x.B\n}\n"
	want := "use_case \"U\" {\n    when seshat listens sns.a, sns.b,\n            sns.c,\n            // the rest\n            sns.d every \"5m\"\n        seshat writes the row\n        outbox notifies one of x.A,\n            x.B\n}\n"
	got := FormatDocument(in)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
}

func TestFormat_BracketItemsAndMarkers(t *testing.T) {
	in := "use_case \"U\" {\n    when O does\n        a asks b to create it  [CreateApiKey,   CreateUsagePlanKey]\n        a asks b for paths  [GET /p/{lat},{lon}]\n        a asks b to start  [inferred   POST /x]\n        a asks c for rows  [SQL investment   flat]\n        b returns   stream   to a each token\n}\n"
	got := FormatDocument(in)
	for _, want := range []string{
		"[CreateApiKey, CreateUsagePlanKey]",
		"[GET /p/{lat},{lon}]",
		"[inferred POST /x]",
		"[SQL investment flat]",
		"        b returns stream to a each token\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if again := FormatDocument(got); again != got {
		t.Errorf("not idempotent:\n%s", again)
	}
}
```

The new corpus files of Tasks A3, A6, A7, A9 and A11 format idempotently and keep their `stepOutline` (the loop in `formatter_corpus_test.go` over `testdata/corpus/04_use_cases/*.craft` covers them once they exist; if it lists files by name, add them).

- [ ] **Step 2: Run** `go test ./internal/lsp/ -run 'EventLists|BracketItems|Corpus'`. Correction 4 predicts PASS without code: the walker already hangs a line after a comma by `ContinuationIndent` (`continuing` reads `prevReal`, so the comment between items keeps hanging too), and a bracket comma is an `Ident` token to `separatorFor`, so `{lat},{lon}` stays tight. Keep the tests as pins.
- [ ] **Step 3: If a test fails:**
  - continuation column wrong → the `when` line's `scenarioDepth` bump happens after the token is written; check that the trigger's continuation tokens see `braceDepth+scenarioDepth == 2`. Fix in `writeTokens`, not with a keyword case in `separatorFor`.
  - the comment between items is written at body depth → `keepInBody` treats it as a body comment; make `keepInBody` return the current depth when `prevReal` is a `SyntaxKindComma`.
  - a list action loses its bracket alignment → the aligner sees the continuation line as a cell; it must split only the line holding `[`.
- [ ] **Step 4: Fuzz.** `FuzzParseUseCaseBody` (`internal/syntax/usecase_body_fuzz_test.go`) gains one `f.Add(body, true)` per Part A example: the §4.1 roster, the §7.1 stream, the §8.1 lists, the §8.3 outbox, both §9.1 examples, and `"    when s listens a.A,\n"` (a dangling comma at EOF). `FuzzFormatBranches` gains seeds `listens_list_multiline` and `notifies_one_of` (the corpus files' bodies). Run each for 60 s: `go test ./internal/syntax/ -run XXX -fuzz FuzzParseUseCaseBody -fuzztime 60s`, `go test ./internal/lsp/ -run XXX -fuzz FuzzFormatBranches -fuzztime 60s` → no failure.
- [ ] **Step 5: Run** the full suite; commit `test(fmt): event lists, bracket items, inferred and returns stream format canonically; fuzz seeds for v2.24`.

---

### Task A14: LSP hover, definition and semantic tokens — **strong**

**Files:** Modify `internal/lsp/blocks.go` (`keywordHovers`, `keywordHover`), `internal/lsp/server.go` (`Hover`, `Definition`, `syntaxKindToCraftToken`), new `internal/lsp/events.go`; Create `internal/lsp/journey2_test.go`

- [ ] **Step 1: Failing tests** (`journey2_test.go`, package `lsp_test`, with the `newBranchClient`/`hoverText`/`semanticTokensAt` helpers of `journey_test.go`)

```go
const rosterLSPSrc = `actors {
    scheduler Ticker
    robot Bot
}
domain partner {
    cooker
    seshat
}
use_case "Cook" {
    tags {
        sites: otodompl
    }
    when Ticker fires the cook
        cooker notifies sns.cooked
        cooker returns stream the parts
}
use_case "Ledger" {
    when seshat listens sns.cooked, sns.a-changed,
            sns.b-changed, sns.c-changed
        seshat asks cooker to replay  [inferred replayAll, replayOne]
        seshat asks cooker for rows  [SQL ledger rows]
}
`

func TestHover_V224(t *testing.T) {
	c, _ := newBranchClient(t, rosterLSPSrc)
	for _, tc := range []struct {
		line, char int
		want       string
	}{
		{1, 6, "scheduler: a clock that starts work (cron, an in-process ticker); drawn as a boundary"},
		{2, 6, "robot: not a recognised kind; drawn as a person"},
		{10, 9, "scope tag: printed in the sequence title; the hub filters and checks its values"},
		{17, 17, "listens to 4 events; publishers: cooker (1 of 4 events)"},
		{17, 26, "sns.cooked: published by cooker"},
		{18, 14, "sns.b-changed: no publisher in the model"},
		{14, 23, "streamed reply: drawn in a `loop stream` box; answers no request in the model"},
		{19, 39, "inferred hop: drawn grey"},
		{19, 49, "2 operations: replayAll, replayOne"},
		{20, 39, "direct use of another party's tables: ledger, rows"},
	} {
		if got := hoverText(c, tc.line, tc.char); got != tc.want {
			t.Errorf("hover %d:%d = %q, want %q", tc.line, tc.char, got, tc.want)
		}
	}
}

func TestDefinition_ListensItemGoesToItsPublishers(t *testing.T) {
	c, _ := newBranchClient(t, rosterLSPSrc)
	var locs []struct {
		Range struct{ Start struct{ Line int } } `json:"range"`
	}
	json.Unmarshal(c.at("textDocument/definition", 17, 26), &locs) //nolint:errcheck
	if len(locs) != 1 || locs[0].Range.Start.Line != 13 {
		t.Errorf("definition = %+v (want the `cooker notifies sns.cooked` line)", locs)
	}
}

func TestSemanticTokens_V224(t *testing.T) {
	const flowKeyword, comma = 8, 36
	toks := semanticTokensAt(t, rosterLSPSrc)
	for _, tc := range []struct{ line, char, want uint32 }{
		{14, 23, flowKeyword}, // stream
		{19, 39, flowKeyword}, // inferred
		{17, 34, comma},
	} {
		if got := toks[[2]uint32{tc.line, tc.char}]; got != tc.want {
			t.Errorf("%d:%d = %d, want %d", tc.line, tc.char, got, tc.want)
		}
	}
}
```

Column notes: line 14 `        cooker returns stream the parts` — `stream` at 23; line 17 `    when seshat listens sns.cooked, …` — `seshat` at 9, `listens` at 16, `sns.cooked` at 24, its comma at 34; line 19 `        seshat asks cooker to replay  [inferred replayAll, replayOne]` — `[` at 38, `inferred` at 39, `replayAll` at 48. Recount against the final source before trusting a failing assertion; adjust the numbers, not the behaviour. Add the `one of` hover (`notifies one of: one event per run, chosen at run time`) with a line `cooker notifies one of x.Ended, y.Ended` if you extend the source.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Implement**
  1. `keywordHovers` gains `SyntaxKindKwInferred: "inferred hop: drawn grey"`, `SyntaxKindKwOne`/`SyntaxKindKwOf: "notifies one of: one event per run, chosen at run time"`. `stream` needs the answered request, so it is handled in `events.go`: hover on a `SyntaxKindKwStream` token finds the action in `s.ws.Doc()` with the same `SourceURI` and `Line`, and prints `streamed reply: drawn in a ` + "`loop stream`" + ` box; answers <request>`, where `<request>` is the answered action's `Description` (looked up by `Answers` in the same use case) or `no request in the model` when `Answers` is `""` (the stream answers the trigger's actor or an unobserved caller).
  2. `events.go`:
     - `actorKindHover(file, li, line, char)`: the cursor on an `ActorDecl.ActorTypeToken()` → `<kind>: <meaning>; drawn as <shape>` for the six kinds (meanings from spec §6.1; user "a person", system "an external system", service "an external service"; shapes "a person" for user, "a boundary" for system/scheduler/tool/infra, "a control" for service) and `<kind>: not a recognised kind; drawn as a person` otherwise. It runs before the existing `actor: X (type)` line hover.
     - `tagKeyHover`: the cursor on the key of a `sites`/`envs` tag → the §5.4 text.
     - `listensHover`: on the subject or `listens` of a trigger with `len(EventItems()) > 1` → `listens to N events; publishers: p1 (k of N events), …` (publishers from `interaction.Derive(s.ws.Doc())` `Publish` interactions, by display name, sorted; `publishers: none in the model` when there are none); on an item → `<item>: published by a, b` or `<item>: no publisher in the model`. A single-item trigger keeps today's hover.
     - `bracketHover`: on an operation bracket with two or more items → `N operations: a, b` (each line of a surface-backed item adds `\n<item>: <query|command>: <phrase>` via `surfaceOpFor`); on a `SQL` verb token → `direct use of another party's tables: t1, t2`; on an `ADDR` verb token → `egress to an address with no operation id; the hub does not resolve it`.
     - Per-item brackets: `askBracketAt` (v2.23, `surfaces.go`) returns the item under the cursor when the bracket lists two or more (`lang.SplitOperationItems` over `OpText()`, offsets from the first body token as in `opItemRange`), so the v2.23 surface hover and definition work per item (spec §8.7).
     - `eventDefinition`: on a listens item (single or list) → one `protocol.Location` per `notifies` action in the workspace whose `EventItems()` contains the item's value (files in sorted URI order, then source order).
     `Hover` calls the new hovers after `keywordHover` and before `journeyHover`; `Definition` calls `eventDefinition` before `journeyDefinition`.
  3. `syntaxKindToCraftToken`: `SyntaxKindKwOne`, `KwOf`, `KwStream`, `KwInferred` → `craft-flow-keyword`. A list comma is `SyntaxKindComma` → `craft-comma` already. `OpVerb` (now including `ADDR`/`SQL`) keeps its mapping.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): hover for actor kinds, scope tags, rosters, streams, inferred and egress brackets; definition from a listens item to its publishers`.

---

### Task A15: LSP completion — **cheap**

**Files:** Modify `internal/lsp/completion.go` (`actorTypeCompletions`, `detectContext`, new contexts and lists), `internal/lsp/blocks.go` if the tags context lives there; Test `internal/lsp/journey2_test.go`

- [ ] **Step 1: Failing tests**

```go
func TestCompletion_V224(t *testing.T) {
	items := completionAt(t, "actors {\n    \n}\n", 1, 4)
	for _, k := range []string{"user", "system", "service", "scheduler", "tool", "infra"} {
		if !hasLabel(items, k) {
			t.Errorf("actors block lacks %q: %v", k, labelList(items))
		}
	}
	items = completionAt(t, rosterLSPSrc+"use_case \"N\" {\n    when seshat listens sns.cooked, \n        seshat does it\n}\n", 23, 36)
	if !hasLabel(items, "sns.cooked") {
		t.Errorf("listens list lacks published events: %v", labelList(items))
	}
	items = completionAt(t, "use_case \"N\" {\n    when O does\n        x returns \n}\n", 2, 18)
	if !hasLabel(items, "stream") {
		t.Errorf("after returns: %v", labelList(items))
	}
	requireSnippet(t, completionAt(t, "use_case \"N\" {\n    when O does\n        x notifies \n}\n", 2, 19), "one of", "one of ${1:event}, ${2:event}")
	items = completionAt(t, "use_case \"N\" {\n    when O does\n        x asks y for it  []\n}\n", 2, 26)
	for _, l := range []string{"inferred", "ADDR", "SQL", "POST"} {
		if !hasLabel(items, l) {
			t.Errorf("bracket head lacks %q: %v", l, labelList(items))
		}
	}
	tags := "use_case \"A\" {\n    tags {\n        sites: imovirtualpt2, imovirtualpt\n    }\n    when O does\n        x does it\n}\nuse_case \"B\" {\n    tags {\n        sites: imovirtualpt2\n        \n    }\n    when O does\n        x does it\n}\n"
	items = completionAt(t, tags, 10, 8)
	requireSnippet(t, items, "sites", "sites: ${1:site}")
	requireSnippet(t, items, "envs", "envs: ${1:env}")
	withC := tags + "use_case \"C\" {\n    tags {\n        sites: \n    }\n    when O does\n        x does it\n}\n"
	items = completionAt(t, withC, 17, 15)
	if len(items) != 2 || items[0].Label != "imovirtualpt2" || items[1].Label != "imovirtualpt" {
		t.Errorf("site values, most frequent first: %v", labelList(items))
	}
}
```

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  - `actorTypeCompletions` lists `lang.KnownActorTypes()` with these details: user "a person", system "an external system", service "an external service", scheduler "a clock that starts work", tool "a CLI, script or client a person runs", infra "platform machinery: pod start, probes, CI, monitors". It is already offered in an actors block; also offer it after `actor ` at top level (`detectContext`: `ctxTopLevel` with prefix `^\s*actor\s+$` → `ctxActorsBlock`).
  - New contexts, checked in the `use_case` branch of `detectContext` before `isActionLine`:
    - `listensListRe = regexp.MustCompile(`^\s*(when\s+\S+\s+listens\s+|notifies\s+one\s+of\s+|\s+)(\S+,\s*)+$`)` → `ctxEventItem`: every event published in `s.ws.Doc()` (`Action.EventList()` of async actions), sorted, deduplicated, kind `Event`. A continuation line (only whitespace before the cursor, previous line ends in `,`) also yields `ctxEventItem`.
    - `returnsRe = regexp.MustCompile(`^\s*\S+\s+returns\s+$`)` → `stream` (detail "a reply that repeats: drawn in a loop stream box").
    - `notifiesRe = regexp.MustCompile(`^\s*\S+\s+notifies\s+$`)` → the snippet `one of ${1:event}, ${2:event}` (label `one of`).
  - Inside an `asks` bracket after `, ` (`unclosedBracketRe` or the annotation's interior, with the prefix ending in `, `), offer the target's surface operations not yet in the bracket (`surfaceOpCompletions` minus the items already written; spec §8.7).
  - The bracket head (`ctxOpAnnotationVerb`) offers `inferred` (detail "the hop is inferred, not read in code") before the protocol verbs; `ADDR`/`SQL` arrive through `lang.ProtocolVerbs()`.
  - Inside `tags { }` (`treeEnclosingBlock` returns `"tags"` inside a `SyntaxKindTagsBlock`; add the case): a blank line offers the snippets `sites: ${1:site}` and `envs: ${1:env}`; after `sites: ` (or `envs: `) the values already used in `s.ws.Doc()` `TagValues[key]`, by descending frequency, then alphabetically.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): completion of actor kinds, event list items, stream, one of, inferred and scope tags`.

---

### Task A16: Visualizer goldens and render check — **cheap**

**Files:** Create `internal/visualizer/testdata/journey-semantics-2.craft`, `internal/visualizer/testdata/golden/journey-semantics-2.*`; Modify `internal/visualizer/golden_test.go` (`goldenModels`), `internal/visualizer/render_integration_test.go`

- [ ] **Step 1:** `journey-semantics-2.craft` holds the spec examples of §4.5 (roster), §5.3 (scope), §6.5 (actor kinds), §7.1 (stream), §8.3 (outbox), §8.1 (operation lists, one use case) and both §9.1 examples, with the actors, domains and services that make every participant resolve and one service per domain so C4 has containers. Append `"testdata/journey-semantics-2.craft"` to `goldenModels`.
- [ ] **Step 2:** `go test ./internal/visualizer/ -run TestGolden -update`. Review the sequence goldens against the spec listings (§4.5, §5.3, §6.5, §7.5, §8.6, §9.5) with corrections 1 and 11 applied: the same lines, in the same order, within each section. `git status` shows only new `journey-semantics-2.*` files.
- [ ] **Step 3:** In `render_integration_test.go`, add `TestJourneySemantics2_MermaidSequenceRenders` and `TestJourneySemantics2_PlantUMLRenders` (copy the v2.23 pair). Run `go test -tags integration ./internal/visualizer/ -run JourneySemantics2`. Watch for:
  - **mmdc rejects `participant X as X (scheduler)`** (parentheses in an alias label): write the label as `X [scheduler]` in the Mermaid writer and in Task A2's test.
  - **PlantUML rejects `-[#999999]>`**: use `-[#999999]->` for a call and `-[#999999]-->` for a reply, and update Task A10's tests.
- [ ] **Step 4:** Full suite; commit `test(visualizer): journey-semantics-2 goldens; mermaid-cli and PlantUML render checks`.

---

### Task A17: Documentation — **cheap**

**Files:** `docs/GRAMMAR.md`, `docs/DIAGNOSTICS.md`, `CONTEXT.md`, `README.md`, `docs/page/language/use-cases.md`

- [ ] **Step 1: GRAMMAR.md.** §3 (Actors): the known kinds are `user`, `system`, `service`, `scheduler`, `tool`, `infra`; any other identifier stays legal and is drawn as a person. §8 gains:

```ebnf
domain_listen = name_slot 'listens' event_list listen_schedule? ;
event_list    = event_item (',' NL* event_item)* ;           (* a continuation line is never a step *)
event_item    = STRING | ref ;
async_action  = name_slot 'notifies' ( event_item | 'one' 'of' event_list ) op_annotation? ;
return_action = name_slot 'returns' 'stream'? ('to' name_slot)? connector_word? phrase? op_annotation? ;
op_annotation = '[' 'inferred'? (op_item (', ' op_item)*)? ']' ;
op_item       = op_verb? payload ;                            (* op_verb includes ADDR and SQL *)
```

  with a paragraph on item splitting (comma followed by whitespace, outside braces and strings) and on quoting an `ADDR` address that starts with `//`. §11.1 gains the spec §3.1 table.
- [ ] **Step 2: DIAGNOSTICS.md.** Add `craft/sema/store-ref-in-event-list` (error), `craft/lint/duplicate-event-in-list`, `craft/lint/duplicate-scenario-body`, `craft/lint/one-of-single-event`, `craft/lint/stream-without-caller`, `craft/lint/egress-on-internal-step`, `craft/lint/unquoted-address` (warnings), each with a one-line meaning. Widen `craft/syntax/empty-op-annotation` (an empty list item; `[ADDR]`/`[SQL]` without payload), `craft/syntax/unexpected-token` (a dangling list comma; a bare `notifies a, b`), `dead-event`, `event-not-past-tense`, `store-without-change` (per item) and `operation-not-in-surface` (per item).
- [ ] **Step 3: CONTEXT.md** glossary: **Roster trigger**, **Operation list**, **Streamed reply**, **Inferred hop**, **Egress**, in the existing style.
- [ ] **Step 4: README.md and `docs/page/language/use-cases.md`.** One short section per construct with the spec example: rosters, `notifies one of`, operation lists, `returns stream`, `ADDR`/`SQL`/`inferred`, actor kinds, and the `sites`/`envs` scope in sequence titles.
- [ ] **Step 5: Commit** `docs: journey semantics 2, part A (rosters, one of, operation lists, streams, egress, inferred, actor kinds, scope)`.

---

### Task A18: tree-sitter-craft: grammar, corpus, release 0.4.0 — **strong**

**Repo:** `~/projects/github/tcarcao/tree-sitter-craft` (branch `feat/journey-semantics-2` off `main` at `v0.3.0`)

**Files:** `grammar.js`, `queries/highlights.scm`, new `test/corpus/journey_semantics_2.txt`, `test/corpus/branches.txt`, `CORPUS_VERSION`; regenerated `src/grammar.json`, `src/node-types.json`, `src/parser.c`

- [ ] **Step 1: Failing corpus tests.** In `test/corpus/journey_semantics_2.txt` (the `===`/`---` format of `branches.txt`), one case each for: a one-line `listens` list; the §4.1 multi-line list with a comment between continuation lines; a list followed by `every "5m"`; `notifies one of` over two lines with a trailing `[TOPIC nmfbus]`; `returns stream`; `returns stream to x each event`; `[CreateApiKey, CreateUsagePlanKey]`; `[GET /p/{lat},{lon}]`; `[inferred POST /x]`; `[inferred]`; `[SQL investment flat]`; `[ADDR "https://a.b/c"]`; `actors { scheduler Ticker  tool OperatorTool  infra Platform }` (one per line). In `branches.txt` add the risk cases (spec §10.2): `stream`, `one`, `inferred` as action subjects (`stream returns the data`, `one notifies of.Ended`, `inferred does it`), as phrase words (`x asks y for one stream of inferred data`), and `x notifies one` (an event named `one`). Write each expected tree from `npx tree-sitter parse` once the grammar is in; until then `npx tree-sitter test` fails.
- [ ] **Step 2: Grammar.**

```js
    _event: $ => choice($.string, $.ref),

    domain_listener: $ => seq(
      $.listener_subject,
      'listens',
      $._event,
      repeat(seq(',', repeat($._newline), $._event)),
      optional($.cron_trigger),
    ),

    async_action: $ => seq(
      $.action_subject,
      'notifies',
      choice(
        $._event,
        seq('one', 'of', $._event, repeat(seq(',', repeat($._newline), $._event))),
      ),
      optional($.operation_annotation),
    ),
```

   `return_action`: insert `optional('stream')` directly after `'returns'` in both alternatives. `operation_annotation`: `seq(alias($._annotation_start, '['), optional(field('marker', 'inferred')), optional(field('verb', $.operation_verb)), repeat(field('payload', $.operation_payload)), ']')`. `operation_verb` gains `'ADDR'` and `'SQL'`. `_branch_keyword` gains `'one'`, `'stream'` and `'inferred'`, so each stays a valid subject through `slug`'s alias. The payload token already accepts commas, so items are not modelled (spec §10.2), and the external `[` scanner is unchanged (correction 2: `https://` is not a comment start in the scanner either).
- [ ] **Step 3: Highlights.**

```scheme
(async_action ["one" "of"] @craft.flow-keyword)
(return_action "stream" @craft.flow-keyword)
(operation_annotation "inferred" @craft.flow-keyword)
(domain_listener "," @craft.comma)
((actor_type) @type.builtin
  (#match? @type.builtin "^(user|system|service|scheduler|tool|infra)$"))
```

  (`@craft.comma` exists in the extension's token list; if it does not, use `@punctuation.delimiter`.)
- [ ] **Step 4: Generate and test.** `npx tree-sitter generate && npx tree-sitter test`. Resolve conflict reports with `prec`/`prec.dynamic` (as `branch_header` does), never by dropping a form. Every case parses with no `ERROR` or `MISSING` node. If `one` or `stream` in a phrase comes out as a keyword, add `alias($._branch_keyword, $.identifier)` to `phrase`'s choices. Commit `feat(grammar): listens lists, notifies one of, returns stream, inferred, ADDR and SQL, actor kinds`.
- [ ] **Step 5: CORPUS_VERSION** (after the coordinator tags craft v2.24.0). Set `CORPUS_VERSION` to `v2.24.0`, run `scripts/fetch-corpus.sh`, then the parse command `compat.yml` runs (`grep -n 'tree-sitter parse' .github/workflows/compat.yml`) over `test/corpus-from-craft`. Fix the grammar until every file parses; commit `chore: bump CORPUS_VERSION to v2.24.0` (that file only).
- [ ] **Step 6: Release.** `git switch main && git merge --ff-only feat/journey-semantics-2 && git push origin main`, then `./release.sh minor` (→ `v0.4.0`). Watch `release.yml` (`gh run watch <id> --repo tcarcao/tree-sitter-craft --exit-status`); `npm view tree-sitter-craft version` → `0.4.0`.

---

### Task A19: VSCode extension: bump and completions, release 0.0.12 — **cheap**

**Repo:** `~/projects/github/tcarcao/craft-vscode-extension` (branch `feat/journey-semantics-2` off `main` at 0.0.11)

**Files:** `package.json`, `server/package.json` (+ both lock files), `server/src/parser/TreeSitterCompletionProvider.ts`, `server/src/parser/TreeSitterCompletionProvider.test.ts`

- [ ] **Step 1: Failing tests** (append to the v2.23 test file; `labels` is its helper)

```ts
test('actors block offers the six kinds', async () => {
  expect(await labels('actors {\n    \n}\n', 1, 4)).toEqual(expect.arrayContaining(['user', 'system', 'service', 'scheduler', 'tool', 'infra']));
});

test('use-case body offers the v2.24 snippets', async () => {
  const text = 'use_case "U" {\n    when O does it\n        x does a\n        \n}\n';
  expect(await labels(text, 3, 8)).toEqual(expect.arrayContaining(['listens list', 'notifies one of', 'returns stream']));
});

test('bracket head offers inferred, SQL and ADDR', async () => {
  const text = 'use_case "U" {\n    when O does it\n        x asks y for it  [\n}\n';
  expect(await labels(text, 2, 26)).toEqual(expect.arrayContaining(['inferred', 'SQL', 'ADDR']));
});

test('tags block offers sites and envs', async () => {
  const text = 'use_case "U" {\n    tags {\n        \n    }\n    when O does it\n        x does a\n}\n';
  expect(await labels(text, 2, 8)).toEqual(expect.arrayContaining(['sites', 'envs']));
});
```

- [ ] **Step 2: Run** `npm test -- TreeSitterCompletionProvider` → FAIL.
- [ ] **Step 3: Implement** (text-based, reusing the v2.23 `enclosingBlock`; teach it `actors` → `'actors'` and a `tags {` line inside a use case → `'tags'`):
  - `'actors'` + blank prefix → the six kinds, `kind: Keyword`, with the details of Task A15.
  - `'use_case'` + blank prefix also offers `listens list` (`when ${1:ctx} listens ${2:a.Event},\n        ${3:b.Event}`), `notifies one of` (`${1:ctx} notifies one of ${2:a.Event}, ${3:b.Event}`), `returns stream` (`${1:ctx} returns stream ${2:each event}`).
  - A prefix ending in `[` (no `]` after it on the line) → `inferred`, `SQL ${1:table}`, `ADDR "${1:https://…}"`, then the existing verbs.
  - `'tags'` + blank prefix → `sites: ${1:site}`, `envs: ${1:env}`.
  All with `insertTextFormat: 2`.
- [ ] **Step 4: Bump** (after Task A18 Step 6). `"tree-sitter-craft": "^0.4.0"` in `package.json` and `server/package.json`; `npm install && (cd server && npm install) && npm run compile` (refreshes `resources/tree-sitter-craft.wasm` and `resources/queries`). Open `testdata/corpus/04_use_cases/listens_list_multiline.craft`, `notifies_one_of.craft` and `op_inferred.craft` from the craft repo in the Extension Development Host: no syntax-error squiggles; `one of`, `stream`, `inferred` highlighted.
- [ ] **Step 5: Commit and release.** `npm test` → PASS. Commit `chore: bump tree-sitter-craft to 0.4.0; completions for actor kinds, rosters, one of, streams, egress and scope tags` (this task's files only). Merge to `main`, push, run `./release.sh 0.0.12`.

---

### Task A20: Hub: craft v2.24.0, rosters, operation lists, one of, egress, inferred, streams, actor kinds — **strong**

**Repo:** `~/Downloads/domain-hub/svc-re-go-arch-context`, branch `feature/craft-v2.21-branches` (it carries the v2.23 work of the v2.23 plan's Tasks 27–28)

**Files:**
- Modify: `go.mod`, `go.sum`, `internal/infrastructure/graphstore/craft_loader.go` (`loadJourneys`, `scenarioLabel`, new `journeyStepsFromTrigger`, `journeyStepsFromAction`, `loadActors` comment), `internal/infrastructure/graphstore/model.go` (`step`, `typedRef`, `parseStepRef`), `internal/infrastructure/graphstore/join.go` (`resolveOne`), `internal/infrastructure/graphstore/validate.go` (`validateUnannotatedAsks`, new `validateSharedTables`, `crossValidate`), `internal/infrastructure/graphstore/use_case.go` (`buildOne`, `actorDescription`), `internal/core/domain/use_case.go` (`Step`), `internal/presentation/rest/graphhdl/response.go`, `internal/presentation/mcp/explainusecasehdl/response.go`, `internal/presentation/mcp/hubresourcehdl/handler.go` (actors listing), `web/src/lib/api.ts`
- Create: `internal/infrastructure/graphstore/craft_v224_test.go`

- [ ] **Step 1: Bump.** `go get github.com/tcarcao/craft/v2@v2.24.0 && make mod` (needs the coordinator's tag; before it exists, use the pseudo-version of the pushed release-prep commit, `go get github.com/tcarcao/craft/v2@<sha>`, and re-run with `@v2.24.0` before merging). `go build ./...` passes unchanged: v2.24 is additive.
- [ ] **Step 2: Failing tests** (`craft_v224_test.go`, package `graphstore_test`; reuse `refsFixture`, `refsManifest`, `refsTopology`, `refsOpenAPI` from `craft_v216_refs_test.go`, whose advert spec has `postSchedulepromotion`; add `putSchedulepromotion` to that spec if it lacks a second operation)
  - **roster:** `when billing listens advert.PromotionScheduled, advert.PromotionCancelled` gives two `listens` steps, each resolved on its own, and `ScenarioLabel` (unnamed) `billing hears advert.PromotionScheduled, advert.PromotionCancelled`; a 5-item list gives `… (+2 more)` after three items.
  - **operation list:** `billing asks advert to schedule or change it  [postSchedulepromotion, putSchedulepromotion]` gives two `asks` steps, both resolved (before the change the combined name failed), one `AuthoredActions`.
  - **one of:** `advert notifies one of PromotionScheduled, PromotionCancelled` gives two `notifies` steps.
  - **egress:** `billing asks advert for its rows  [SQL promotion schedule]` gives a step with `Tables == ["promotion", "schedule"]`, not unresolved, and one `shared_tables` validation issue naming both tables; `billing asks advert to post it  [ADDR "https://x.example/y"]` gives `Egress == "https://x.example/y"`, not unresolved.
  - **inferred:** `[inferred postSchedulepromotion]` gives `Confidence == "inferred"` and resolves; `[inferred]` alone gives an inferred step and no `unannotated_asks` issue.
  - **stream:** `billing asks advert to stream the schedule  [postSchedulepromotion]` then `advert returns stream each day` marks the asks step `Streaming`.
  - **actors:** an `actors { scheduler Ticker }` fixture: `Type == "scheduler"` and the actor's description starts `Craft scheduler actor (a clock that starts work).`
- [ ] **Step 3: Run** `go test ./internal/infrastructure/graphstore/ -run V224` → FAIL.
- [ ] **Step 4: Implement**
  1. `step` gains:

```go
	// Egress is the address of an ADDR operation: a call with no operation id,
	// never resolved against a spec.
	Egress string
	// Tables are the tables of a SQL operation: another party's tables used
	// directly, never resolved against a spec.
	Tables []string
	// Confidence is "inferred" when the hop was inferred, not read in code.
	Confidence string
	// Streaming marks an asks whose answer is a `returns stream`.
	Streaming bool
```

     `typedRef` gains `Egress string; Tables []string`, copied by `parseStepRef`.
  2. Loader:

```go
// journeyStepsFromTrigger maps a trigger to its steps: one listens step per
// event of a roster (craft v2.24 `listens a, b`), else what
// journeyStepFromTrigger returns.
func journeyStepsFromTrigger(t craft.Trigger, entry string) []*step {
	if t.Type == craft.TriggerTypeDomainListen && len(t.Events) > 0 {
		out := make([]*step, 0, len(t.Events))
		for _, ev := range t.EventList() {
			s := &step{Actor: t.Context, Verb: verbListens, RefRaw: ev}
			applyEventRef(s, ev)
			out = append(out, s)
		}
		return out
	}
	if s := journeyStepFromTrigger(t, entry); s != nil {
		return []*step{s}
	}
	return nil
}

// journeyStepsFromAction maps an action to its steps: one per event of a
// `notifies one of`, one per operation of an asks bracket list, else what
// journeyStepFromAction returns. Every step carries the action's line, its
// confidence, and the egress address or tables of its operation.
func journeyStepsFromAction(a craft.Action) []*step {
	var out []*step
	switch {
	case a.Type == craft.ActionTypeAsync && len(a.Events) > 0:
		for _, ev := range a.EventList() {
			s := &step{Actor: a.Context, Verb: verbNotifies, RefRaw: ev, Line: a.Line}
			applyEventRef(s, ev)
			out = append(out, s)
		}
	case a.Type == craft.ActionTypeSync && len(a.OperationList()) > 0:
		for _, op := range a.OperationList() {
			one := a
			one.Operation, one.Operations = &op, nil
			s := journeyStepFromAction(one)
			if s == nil {
				continue
			}
			switch op.Verb {
			case craft.OpVerbAddr:
				s.Egress = op.Payload
			case craft.OpVerbSQL:
				s.Tables = op.Tables()
			}
			out = append(out, s)
		}
	default:
		if s := journeyStepFromAction(a); s != nil {
			out = append(out, s)
		}
	}
	if a.Inferred {
		for _, s := range out {
			s.Confidence = "inferred"
		}
	}
	return out
}
```

     In `loadJourneys`, the trigger loop calls `journeyStepsFromTrigger` and appends every step; the action loop calls `journeyStepsFromAction(a)` and appends every step (the branch path and labels apply to each), after building `answered := map[string]bool{}` per scenario with `sc.EachAction(func(a *craft.Action) { if a.Stream && a.Answers != "" { answered[a.Answers] = true } })` and setting `s.Streaming = answered[a.ID]` on asks steps. `AuthoredActions` still counts once per action.
  3. `scenarioLabel`, listens case: when `len(t.Events) > 0`, `t.Context + " hears " + strings.Join(first three items, ", ")` plus `fmt.Sprintf(" (+%d more)", n-3)` beyond three.
  4. `resolveOne`, `verbAsks` case: once the target context has resolved, return without matching an operation when `ref.Egress != "" || len(ref.Tables) > 0` (neither is unresolved).
  5. `validateUnannotatedAsks` skips steps with `Confidence == "inferred"`. New `validateSharedTables(g)` (modelled on `validateUnannotatedAsks`, same severity) adds one `shared_tables` issue per step with `Tables` whose target context is not the actor's: ``fmt.Sprintf("%s uses %s's tables directly: %s", actor, target, strings.Join(tables, ", "))``. Register it in `crossValidate`.
  6. `domain.Step` gains `Egress string`, `Tables []string`, `Confidence string`, `Streaming bool`; `buildOne` maps them; the REST response adds `egress`, `tables`, `confidence`, `streaming` (`omitempty`, mirroring how `f381e0f` added the branch fields) and `web/src/lib/api.ts` the optional fields. `arch_explain_use_case` prints, after a step's text, ` (inferred)`, ` — streamed reply`, ` — egress: <address>` and ` — tables: a, b` as they apply.
  7. Actor kinds: `actorDescription` starts with `Craft <kind> actor (<meaning>).` using the meanings of spec §6.1 for `scheduler`/`tool`/`infra` and today's text for the others; `archctx://actors` lists actors grouped by kind in `craft.KnownActorTypes()` order, unknown kinds last, by name inside a kind. Replace the `loadActors` comment's "three different things wearing one keyword" sentence with: "craft v2.24 states the kind (user, system, service, scheduler, tool, infra); the hub carries it verbatim."
- [ ] **Step 5: Run** `make lint && make test` → PASS.
- [ ] **Step 6: Commit and push** (vendor/ is ignored):

```bash
git add go.mod go.sum internal web/src/lib/api.ts
git commit -m "feat(graphstore): craft v2.24 — roster listens, operation lists, notifies one of, egress and shared tables, inferred hops, streamed replies, actor kinds

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
git push
```

---

### Task A21: Hub: `site`/`env` filters and `tag_unknown_value` — **strong**

**Repo:** same branch as Task A20.

**Files:**
- Modify: `internal/core/domain/use_case.go` (`FindUseCasesParams`), `internal/presentation/mcp/findusecaseshdl/request.go`, `internal/presentation/rest/graphhdl/handler.go` (the `FindUseCases` call at ~L319), `internal/infrastructure/graphstore/graphstore.go` (`Repository.FindUseCases`, `Config`, the `crossValidate` call at ~L184), `internal/infrastructure/graphstore/validate.go` (new `validateTagValues`), `cmd/sharedconfig.go`, `internal/config/app.go`, `internal/config/repocfg/repocfg.go`
- Test: `internal/infrastructure/graphstore/craft_v224_test.go`, `internal/presentation/mcp/mcpsurface_test.go` (if it pins tool schemas)

- [ ] **Step 1: Failing tests**
  - Three use cases: one with `sites: otodompl`, one with `sites: storiaro`, one with no `sites` tag. `FindUseCases({Site: "otodompl"})` returns the first and the third (a use case with no `sites` tag runs on every site); `{Env: "production"}` returns all three when none has `envs`.
  - `graphstore.New(Config{KnownSites: []string{"otodompl"}})` over a fixture whose use case has `sites: otodompl, imomock` reports one `tag_unknown_value` issue naming `imomock`; with `KnownSites` unset there is none.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  1. `domain.FindUseCasesParams` gains `// Site keeps use cases whose sites tag includes it, and those with no sites tag.` `Site string` and the same `Env string`. `Repository.FindUseCases` filters with `scopeMatches(uc.Sites, params.Site)`:

```go
// scopeMatches reports whether a use case scoped to values runs where want
// names: an empty want or an unscoped use case always matches.
func scopeMatches(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" || len(values) == 0 {
		return true
	}
	for _, v := range values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
```

  2. MCP `Request` gains

```go
	Site string `json:"site,omitempty" jsonschema:"only use cases that run on this site or market, e.g. 'otodompl'; use cases with no sites tag run on every site and are included. Use it to narrow a question to one market's journeys"`
	Env  string `json:"env,omitempty" jsonschema:"only use cases that run in this environment, e.g. 'production'; use cases with no envs tag run in every environment and are included. Use it when a question is about one environment"`
```

     (both over 15 words, saying when to use them, per AGENTS.md), mapped in `toParams`; the REST handler reads `site` and `env` query parameters.
  3. Config: `cmd/sharedconfig.go` gains `&cli.StringSliceFlag{Name: knownSitesProp, EnvVars: []string{"KNOWN_SITES"}, Usage: "comma-separated site codes a craft sites tag may name; empty turns the check off"}` and the same for `KNOWN_ENVS`; `config.App` gains `KnownSites, KnownEnvs []string`; `repocfg.WithRepos` passes them in `graphstore.Config{KnownSites: …, KnownEnvs: …}`.
  4. `validateTagValues(g, knownSites, knownEnvs)` adds one `tag_unknown_value` issue per (use case, key, value) not in the configured list (case-sensitive: `imovirtualpt` vs `imovirtualpt2` is the drift being caught), with `SourceFile` set; it returns nil for a key whose list is empty. Call it from `crossValidate` (pass the two lists through from `Config`).
- [ ] **Step 4: Run** `make lint && make test` → PASS.
- [ ] **Step 5: Commit and push** `feat(hub): site and env filters on arch_find_use_cases; tag_unknown_value against KNOWN_SITES/KNOWN_ENVS`. Opening the MR (`glab mr create`) needs the user's go-ahead in the session.

---

### Task A22: Dogfood on re-arch-central (verification only) — **cheap**

**Repo:** `~/Downloads/domain-hub/re-arch-central` (read only; the corpus migration of spec §11.3 comes after the release)

- [ ] **Step 1:** In the craft repo: `CRAFT=$(mktemp -d)/craft && go build -o "$CRAFT" ./cmd/craft`.
- [ ] **Step 2:** In re-arch-central: `$CRAFT validate docs/topology.craft docs/actors.craft docs/context_map.craft 'docs/journeys/*/*.craft' > /tmp/js2-after.txt`, and the same with the released v2.23.0 binary into `/tmp/js2-before.txt`. Expect **0 errors** in both. Compare code counts (`grep -o 'craft/[a-z-]*/[a-z-]*' FILE | sort | uniq -c`): the only new code is `craft/lint/duplicate-scenario-body`, about 11 hits in 6 use cases (spec §11.1), all with the "fold them into one trigger" message. `unused-actor` counts are unchanged. Any other difference is a bug: fix it in craft before going on.
- [ ] **Step 3:** Copy `docs/journeys/partner/mercury-transaction-ledger-ingest.craft` and `docs/journeys/seller-tooling/ai-assistant-chat.craft` to a temp dir. Rewrite the first with one `listens` list of the roster's topics (delete the "seshat ledger subscriptions" use case), and the second with `returns stream` for the relay steps. Validate them with the topology/actors files (0 errors), and render `sequence` and `domain` in `puml` and `mmd` into `~/Downloads/domain-hub/journey-assessment/_render-journey-semantics-2/`. Check by eye: one inbound arrow for the roster, capped at 3 items with `(+N more)`; `loop stream` boxes around the two relays.
- [ ] **Step 4:** Nothing is committed in re-arch-central. Record the before/after counts for the release-prep commit (Task A23).

**Result (2026-10-14, branch at d531b41 vs released 2.23.0, on a scratch copy of re-arch-central; nothing there was edited).**
- `validate docs/topology.craft docs/actors.craft docs/context_map.craft 'docs/journeys/*/*.craft'`: 0 errors with both binaries. 2.23.0: 133 warnings, 3 hints; the branch: 146 warnings, 3 hints. The 13 extra lines are all `craft/lint/duplicate-scenario-body`; every other line is byte-identical (event-not-past-tense 83, unused-actor 43, dead-event 4, repeated-request 1, relationship-direction-inverted 2, relationship-bidirectional 3 hints). The 13 split as the spec §5.2 predicts, 11 folds + 2 others, but the 11 folds sit in **5** use cases, not 6:
  - "fold them into one trigger" ×11: `billing/nmf-monetization-events-become-warehouse-facts.craft:25`; `billing/portfolio-inventory-from-posting.craft:21,25,29`; `billing/sub-account-is-kept-in-step-with-its-parent.craft:18,21`; `vas/ad-lifecycle-vas-cascade.craft:29,34`; `vas/nmf-keeps-warehouse-facts-of-feature-records.craft:21,25,29`. The second use case of `ad-lifecycle-vas-cascade` has a different phrase per scenario, so it is rightly not reported.
  - "different initiators (`Seller` and `Operator`)" ×1: `billing/nmf-billing-cycle-rollover.craft:19`.
  - "different operations" ×1: `subscriptions/revise-an-active-poa-adjustment.craft:31`.
  - The fold hint is pairwise: in a use case with four same-body scenarios, each of the three warnings suggests a two-item list (scenario 1 + scenario n), not one list of all four. Folding by hand gives the full list; the hint is correct but partial.
- `fmt --check` on the same files: clean with both binaries, identical output.
- `generate --type all` (puml and mermaid): C4, domain and surface output byte-identical. In the sequence files only the 121 section titles of use cases with `sites`/`envs` tags change (`[sites: …]`, 12 of them also `[envs: …]`); with that suffix stripped, both files equal 2.23.0's.
- Migrations on the scratch copy (branch binary):
  - **Ledger roster** (`partner/mercury-transaction-ledger-ingest.craft`): one `listens` list of the 45 topics (trailing commas, items 8 columns past `when`); the "seshat ledger subscriptions" use case and its `scope: surface` tag deleted; 5 per-topic facts became `note`s and the 4 `finding`s were kept, each prefixed with its topic. 0 errors, warnings unchanged at 146 (one event-not-past-tense hit moves line), `fmt --check` clean. Inbound arrow: `[-> seshat : sns.mercury-advert-state-changed, sns.mercury-advert-state-changed-failed, sns.mercury-advert-state-changed-failed-retryable (+42 more) (from advertgw, advertmetadata, atlas-advert, +12 more)` (Mermaid the same on `External->>seshat`).
  - **Actor kinds** (`docs/actors.craft`: `scheduler CRON`, `scheduler Ticker`, `infra Platform`, `infra Pipeline`, `infra SyntheticMonitor`, `tool OperatorTool`, `tool ContractConsumer`): validate output identical (unused-actor unchanged; `when cron` still resolves to `CRON`). `fmt --check` flags only the realigned trailing comments of `actors.craft`. Output changes: PlantUML `boundary "CRON" as CRON <<scheduler>>` (and `<<tool>>`, `<<infra>>`), Mermaid sequence `participant CRON as CRON (scheduler)`, C4 `System_Ext(…, "Scheduler" / "Operator tool" / "Platform infrastructure")`; Mermaid domain unchanged. **2.23.0 cannot read the migrated file** (4 errors): it takes the first identifier after an unknown kind as the name, so `scheduler CRON` declares an actor named `scheduler` and the next `scheduler …` line is a duplicate. Fixed on this branch in 9cfc24a (`ActorDecl.Name`); the migration needs craft ≥ 2.24.0.
  - **Event-bus records** (`platform-ops/event-bus-atlas-delivery.craft`): the 12 `records:` entries replace the `eventbus.Event` placeholder in a `listens` list (the placeholder is itself one of the 12) and the tag is dropped. 0 errors, warnings unchanged (the event-not-past-tense hit for `eventbus.CreateAutoSchedule` moves here from the vas file, as the lint reports each event at its first occurrence). Arrow: `[-> eventbus : eventbus.Event, eventbus.ExportToOlxSuccess, eventbus.ExportToOlxFailure (+9 more) (from advertdiscount, midgard, nmf_monetization, +2 more)`; before, a bare `eventbus.Event` with no publisher.
  - **Streams** (`seller-tooling/ai-assistant-chat.craft`): the relay step became `AiAssistant returns stream the reply as META, TOKEN and DONE events` + `graphql returns stream each upstream event to the client as META, TOKEN or DONE`. 0 errors, no new warning (no `stream-without-caller`), `fmt --check` clean; the sequence draws two `loop stream` boxes, `AiAssistant --> graphql` and `graphql --> frontendplatform`.
  - Renders: `<scratchpad>/a22/render/` (ledger, event-bus and ai-chat sequence; ledger domain).
- Found on the way: `craft inspect`'s `events_published`/`events_consumed` read only `Event`, so a `listens` list or a `notifies one of` reported its first item only (`cmd/craft/inspect.go:83-90`). Fixed before the release: both read `EventList()` (`TestInspectCmd_EventListsReportEveryItem`).

---

### Task A23: Release prep v2.24.0 (CHANGELOG) — **cheap**

The coordinator tags; this task prepares the release commit, mirroring v2.22.0 (`git show 2554aff --stat`).

- [ ] **Step 1: CHANGELOG.md.** Add `## [2.24.0] — <release date, YYYY-MM-DD>` above `## [2.23.0]`:
  - `### Added`: `listens` lists (`Trigger.Events`, `Trigger.EventList()`); `notifies one of` (`Action.Events`, `Action.EventList()`); operation lists in one bracket (`Action.Operations`, `Action.OperationList()`); `returns stream` (`Action.Stream`, `Action.Answers`); the `ADDR` and `SQL` protocol verbs (`Operation.Tables()`) and the `[inferred …]` marker (`Action.Inferred`); the actor kinds `scheduler`/`tool`/`infra` (`KnownActorTypes()`); `sites`/`envs` in sequence titles; the codes `craft/sema/store-ref-in-event-list`, `craft/lint/duplicate-event-in-list`, `duplicate-scenario-body`, `one-of-single-event`, `stream-without-caller`, `egress-on-internal-step`, `unquoted-address`; the `pkg/craft` additions (`ActorTypeScheduler/Tool/Infra`, `OpVerbAddr`, `OpVerbSQL`, `TagSites`, `TagEnvs`, `KnownActorTypes`); LSP hover, definition and completion for all of them.
  - `### Changed`: `X returns stream …`, `[inferred …]`, `[ADDR …]`, `[SQL …]` and `notifies one of …` now have meaning (no known uses; correction 3). `craft/syntax/empty-op-annotation` also reports an empty list item and `[ADDR]`/`[SQL]` without payload. Event lints report per list item.
  - `### Changed — diagrams`: `scheduler`/`tool`/`infra` actors draw as stereotyped boundaries (Mermaid: a participant marked `(kind)`); a roster trigger draws one inbound arrow; `notifies one of` draws one message; inferred hops draw grey with `(inferred)`; streamed replies sit in a `loop stream` box; section titles show `[sites: …] [envs: …]`; C4 names SQL and ADDR relations.
- [ ] **Step 2:** In the spec, set `**Status:** Part I implemented (v2.24.0)`.
- [ ] **Step 3:** `go vet ./... && go test ./...` and `go test -tags integration ./internal/visualizer/` → PASS.
- [ ] **Step 4: Commit** (the coordinator then tags `v2.24.0`, pushes, and runs Task A18 Step 5–6, Task A19 Step 4–5 and Task A20 Step 1):

```bash
git add CHANGELOG.md docs/superpowers/specs/2026-10-14-journey-semantics-2-design.md
git commit -m "chore(release): v2.24.0

Dogfood on re-arch-central: 0 errors; the only new warning code is
duplicate-scenario-body (<N> hits in <M> use cases).

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

# Part B — v2.25.0

### Task B1: Vocabulary and model types, no behaviour change — **cheap**

**Files:**
- Modify: `internal/lang/lang.go`, `internal/model/model.go`, `pkg/craft/craftdoc.go`
- Test: `internal/model/model_test.go`, `pkg/craft/craftdoc_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/model/model_test.go`)

```go
func TestStepLink_IsNotAnActionAndNotTheFirstStep(t *testing.T) {
	blk := &model.Block{ID: "block_4", Kind: model.BlockAlt}
	sc := model.Scenario{Steps: []model.Step{
		{Link: &model.Link{ID: "link_2", Kind: model.LinkIncludes, Title: "Site session preamble"}},
		{Note: &model.Note{ID: "note_3"}},
		{Block: blk},
	}}
	if sc.OpeningBlock() != blk {
		t.Error("OpeningBlock must skip a leading link")
	}
	if len(sc.Actions()) != 0 {
		t.Error("a link is not an action")
	}
}

func TestV225Fields_AreOmittedWhenEmpty(t *testing.T) {
	out, err := json.Marshal(model.UseCase{Name: "U", Scenarios: []model.Scenario{{
		Trigger: model.Trigger{Type: model.TriggerTypeExternal},
		Steps:   []model.Step{{Action: &model.Action{ID: "action_2"}}, {Note: &model.Note{}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"sources"`, `"link"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("%s must be omitted when empty: %s", key, out)
		}
	}
}
```

`pkg/craft/craftdoc_test.go`:

```go
func TestV225Exports(t *testing.T) {
	_ = craft.Step{Link: &craft.Link{Kind: craft.LinkContinues, Target: &craft.UseCaseRef{Name: "x"}, Sources: []craft.Source{{Path: "a.go", Start: 1, End: 2}}}}
	_ = []string{craft.TagJourney, craft.TagFormerlyTitled}
	_ = []craft.JourneyEdgeKind{craft.JourneyEdgeEvent, craft.JourneyEdgeStore, craft.JourneyEdgeContinues}
	_ = craft.LinkIncludes
}
```

- [ ] **Step 2: Run** `go test ./internal/model/ ./pkg/craft/` → FAIL to compile.

- [ ] **Step 3: Vocabulary** (`lang.go`)

```go
// LinkKind is the kind of a link step (spec §12.3).
type LinkKind string

const (
	LinkIncludes  LinkKind = "includes"  // the target's steps run here; this body goes on after
	LinkContinues LinkKind = "continues" // the flow goes on in the target; ends this body
)

// Tag keys with a meaning in v2.25 (spec §16).
const (
	TagJourney        = "journey"         // the spine a use case belongs to
	TagFormerlyTitled = "formerly_titled" // earlier titles, `;`-separated
)

// JourneyEdgeKind is what connects two members of a journey spine.
type JourneyEdgeKind string

const (
	JourneyEdgeEvent     JourneyEdgeKind = "event"
	JourneyEdgeStore     JourneyEdgeKind = "store"
	JourneyEdgeContinues JourneyEdgeKind = "continues"
)
```

- [ ] **Step 4: Model** (`model.go`)

```go
// Source is one cite of a `//@` line (spec §14.2).
type Source struct {
	Repo  string `json:"repo,omitempty"`  // as written; "" when the cite names none
	Path  string `json:"path"`
	Start int    `json:"start,omitempty"` // first cited line, 1-based; 0 when no line is cited
	End   int    `json:"end,omitempty"`   // last cited line; equal to Start for one line
	Text  string `json:"text,omitempty"`  // the text after " -- ", shared by the cites of its line
	Line  int    `json:"line,omitempty"`  // the line of the //@ comment in the .craft file
}

// UseCaseRef names one use case of the workspace.
type UseCaseRef struct {
	Name      string `json:"name"`
	SourceURI string `json:"sourceUri,omitempty"`
}

// LinkKind is the kind of a link step.
type LinkKind = lang.LinkKind

const (
	LinkIncludes  = lang.LinkIncludes
	LinkContinues = lang.LinkContinues
)

// Link is an `includes "<title>"` or `continues "<title>"` step. Like a
// note, it is not an action: no interaction, no step number.
type Link struct {
	ID      string      `json:"id"`               // link_<n>
	Kind    LinkKind    `json:"kind"`             // "includes" | "continues"
	Title   string      `json:"title"`            // as written
	Target  *UseCaseRef `json:"target,omitempty"` // nil when it does not resolve
	Sources []Source    `json:"sources,omitempty"`
	Line    int         `json:"line,omitempty"`
	Column  int         `json:"column,omitempty"` // of the title string
}

// Tag keys with a meaning, and journey edge kinds (see lang).
const (
	TagJourney        = lang.TagJourney
	TagFormerlyTitled = lang.TagFormerlyTitled
)

type JourneyEdgeKind = lang.JourneyEdgeKind

const (
	JourneyEdgeEvent     = lang.JourneyEdgeEvent
	JourneyEdgeStore     = lang.JourneyEdgeStore
	JourneyEdgeContinues = lang.JourneyEdgeContinues
)
```

   `Step` becomes "exactly one of Action, Block, Note or Link" and gains `Link *Link \`json:"link,omitempty"\``. `Sources []Source \`json:"sources,omitempty"\`` is added (after the position fields) to `UseCase`, `Trigger`, `Action`, `Branch` and `Note`. `FirstStepIndex` skips links too (`if st.Note == nil && st.Link == nil`), and its comment says so; `eachAction` already ignores them.

- [ ] **Step 5: `pkg/craft`.** Aliases `Source`, `Link`, `LinkKind`, `UseCaseRef`, `JourneyEdgeKind`; constants `LinkIncludes`, `LinkContinues`, `TagJourney`, `TagFormerlyTitled`, `JourneyEdgeEvent`, `JourneyEdgeStore`, `JourneyEdgeContinues`. Replace the v2.23 stability sentence with:

```go
// Step is exactly one of Action, Block, Note or Link (Note since v2.23.0,
// Link since v2.25.0); a consumer that assumes Action-or-Block must handle
// the others.
```

- [ ] **Step 6: Run** → PASS; full suite with no golden change.
- [ ] **Step 7: Commit** `feat(model): Source, Link, UseCaseRef, Step.Link, Sources on six types, journey vocabulary (v2.25)`.

---

### Task B2: `//@` source cites: parse, attach, diagnose — **strong**

**Files:**
- Create: `internal/lang/cite.go`, `internal/lang/cite_test.go`, `internal/syntax/sources.go`, `internal/sema/sources.go`, `internal/analysis/sources_test.go`, `testdata/corpus/04_use_cases/sources.craft` + golden, `testdata/broken/{source_malformed,source_detached}.craft` + `.diagnostics.json`
- Modify: `internal/syntax/ast.go` (`UseCaseDecl.BodySpan`), `internal/analysis/project.go` (`projectUseCases`, new `attachSources`), `internal/sema/sema.go` (`AnalyzeFile`), `internal/lexer/lexer_test.go`

- [ ] **Step 1: Write the failing tests** (`internal/lang/cite_test.go`)

```go
package lang

import (
	"reflect"
	"testing"
)

func TestParseSourceComment(t *testing.T) {
	cases := []struct {
		in    string
		cites []Cite
		text  string
		ok    bool
	}{
		{"//@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types",
			[]Cite{{Repo: "re-monetization-framework", Path: "functions/eb_email_sender/eb_email_sender.go", Start: 5, End: 5}}, "one rule, eight detail-types", true},
		{"//@ eb_email_sender.go:5-40", []Cite{{Path: "eb_email_sender.go", Start: 5, End: 40}}, "", true},
		{"//@ sqs.tf:3,26", []Cite{{Path: "sqs.tf", Start: 3, End: 3}, {Path: "sqs.tf", Start: 26, End: 26}}, "", true},
		{"//@ a/b.go:10, :40-42, c/d.go", []Cite{{Path: "a/b.go", Start: 10, End: 10}, {Path: "a/b.go", Start: 40, End: 42}, {Path: "c/d.go"}}, "", true},
		{"//@ pkg/siren/siren.go:63-115 -- a 423 for a suppressed address counts as delivered",
			[]Cite{{Path: "pkg/siren/siren.go", Start: 63, End: 115}}, "a 423 for a suppressed address counts as delivered", true},
		{"//@ pkg/render.go:40-12 -- reversed", nil, "", false},
		{"//@ eb_email_sender.go:5 one rule", nil, "", false},
		{"//@ README", nil, "", false},
		{"//@", nil, "", false},
		{"//@ :12", nil, "", false},
	}
	for _, c := range cases {
		cites, text, ok := ParseSourceComment(c.in)
		for i := range cites {
			cites[i].At, cites[i].Len = 0, 0 // spans are checked below
		}
		if ok != c.ok || text != c.text || !reflect.DeepEqual(cites, c.cites) {
			t.Errorf("%q: got %+v %q %v", c.in, cites, text, ok)
		}
	}
	cites, _, _ := ParseSourceComment("//@ repo a/b.go:5")
	if got := "//@ repo a/b.go:5"[cites[0].At : cites[0].At+cites[0].Len]; got != "repo a/b.go:5" {
		t.Errorf("span = %q", got)
	}
}
```

Attachment (`internal/analysis/sources_test.go`):

```go
package analysis

import "testing"

const sourcesSrc = `use_case "nMF mails the seller about an add-on event" {
    //@ uc/top.go:1
    when nmf_monetization listens nmfbus.AddOnPurchased
        //@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types
        nmf_monetization picks the mail resolver from the event type alone
        // a plain comment does not break the attachment
        //@ eb_email_sender.go:5-40
        alt "suppressed"
            nmf_monetization asks Siren to send the purchase mail  [POST /api/v1/event]
            //@ pkg/siren/siren.go:63-115 -- a 423 counts as delivered
            note "no retry"
            //@ notes.md:3
}
`

func TestAttachSources(t *testing.T) {
	uc := File("s.craft", sourcesSrc).Doc.UseCases[0]
	if len(uc.Sources) != 1 || uc.Sources[0].Path != "uc/top.go" {
		t.Errorf("use case sources = %+v", uc.Sources)
	}
	sc := uc.Scenarios[0]
	if got := sc.Trigger.Sources; len(got) != 1 || got[0].Repo != "re-monetization-framework" || got[0].Text != "one rule, eight detail-types" || got[0].Line != 4 {
		t.Errorf("trigger sources = %+v", got)
	}
	picks := sc.Steps[0].Action
	if len(picks.Sources) != 1 || picks.Sources[0].End != 40 {
		t.Errorf("action sources = %+v", picks.Sources)
	}
	br := sc.Steps[1].Block.Branches[0]
	if len(br.Sources) != 0 {
		t.Errorf("branch header has no cite under it: %+v", br.Sources)
	}
	if asks := br.Steps[0].Action; len(asks.Sources) != 1 || asks.Sources[0].Path != "pkg/siren/siren.go" {
		t.Errorf("asks sources = %+v", asks.Sources)
	}
	if note := br.Steps[1].Note; len(note.Sources) != 1 || note.Sources[0].Path != "notes.md" {
		t.Errorf("note sources = %+v", note.Sources)
	}
}
```

(Line 4 is the 1-based line of the `//@ re-monetization…` comment.) Lexer pin (`lexer_test.go`): `//@ a.go:1` and `// a` lex as `TokenLineComment`, `/// a` as `TokenDocComment`.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: `lang/cite.go`**

```go
package lang

import (
	"regexp"
	"strconv"
	"strings"
)

// Cite is one cite of a `//@` line. At and Len are its byte span (repo
// through ranges) within the comment text.
type Cite struct {
	Repo       string
	Path       string
	Start, End int
	At, Len    int
}

var repoRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ParseSourceComment parses a `//@ [repo] path[:lines][, …] [-- text]`
// comment (spec §14.1). ok is false for anything else: the line then stays
// an ordinary comment.
func ParseSourceComment(comment string) (cites []Cite, text string, ok bool) {
	body, found := strings.CutPrefix(comment, "//@")
	if !found {
		return nil, "", false
	}
	head, note, _ := strings.Cut(body, " -- ")
	text = strings.TrimSpace(note)
	offset := len("//@")
	var prevRepo, prevPath string
	for _, part := range strings.Split(head, ", ") {
		at := offset + len(part) - len(strings.TrimLeft(part, " "))
		offset += len(part) + len(", ")
		fields := strings.Fields(part)
		var repo, loc string
		switch len(fields) {
		case 1:
			loc = fields[0]
		case 2:
			repo, loc = fields[0], fields[1]
			if !repoRe.MatchString(repo) {
				return nil, "", false
			}
		default:
			return nil, "", false
		}
		span := len(strings.TrimSpace(part))
		path, ranges, hasRanges := strings.Cut(loc, ":")
		switch {
		case path == "" && (prevPath == "" || repo != ""):
			return nil, "", false
		case path == "":
			path, repo = prevPath, prevRepo
		case !strings.ContainsAny(path, "/."):
			return nil, "", false
		}
		prevRepo, prevPath = repo, path
		if !hasRanges {
			cites = append(cites, Cite{Repo: repo, Path: path, At: at, Len: span})
			continue
		}
		for _, r := range strings.Split(ranges, ",") {
			a, b, isRange := strings.Cut(r, "-")
			start, err := strconv.Atoi(a)
			if err != nil || start < 1 {
				return nil, "", false
			}
			end := start
			if isRange {
				if end, err = strconv.Atoi(b); err != nil || end < start {
					return nil, "", false
				}
			}
			cites = append(cites, Cite{Repo: repo, Path: path, Start: start, End: end, At: at, Len: span})
		}
	}
	if len(cites) == 0 {
		return nil, "", false
	}
	return cites, text, true
}
```

- [ ] **Step 4: Syntax views** (`internal/syntax/sources.go`)

```go
package syntax

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/green"
)

// IsSourceComment reports whether tok is a `//@` source cite. A cite stays a
// line comment (trivia); only its text marks it (plan correction 6).
func IsSourceComment(tok SyntaxToken) bool {
	return tok.Kind() == SyntaxKindLineComment && strings.HasPrefix(tok.Text(), "//@")
}

// SourceComments returns every `//@` token under n, in source order.
func SourceComments(n SyntaxNode) []SyntaxToken {
	var out []SyntaxToken
	for _, tok := range n.AllTokens() {
		if IsSourceComment(tok) {
			out = append(out, tok)
		}
	}
	return out
}

// BodySpan returns the offsets just after the use case's `{` and at its `}`
// (the node's end when it is unclosed). ok is false without a `{`.
func (u UseCaseDecl) BodySpan() (start, end green.TextSize, ok bool) {
	lb := u.node.ChildToken(SyntaxKindLBrace)
	if lb == nil {
		return 0, 0, false
	}
	end = u.node.Offset() + u.node.TextLen()
	if rb := u.node.ChildToken(SyntaxKindRBrace); rb != nil {
		end = rb.Offset()
	}
	return lb.Offset() + green.TextSize(len(lb.Text())), end, true
}
```

   (Use the node's own offset/length accessors; if `TextLen` is named differently in `tree.go`, use that.) A comment directly above `use_case` is a leading child of the `UseCaseDecl` node, which is why attachment filters by the body span.

- [ ] **Step 5: Attachment** (`project.go`). In `projectUseCases`, after the use case's scenarios are projected:

```go
		if start, end, ok := uc.BodySpan(); ok {
			var lines []sourceLine
			for _, tok := range syntax.SourceComments(uc.Node()) {
				if tok.Offset() < start || tok.Offset() >= end {
					continue
				}
				cites, text, ok := lang.ParseSourceComment(tok.Text())
				if !ok {
					continue // craft/syntax/malformed-source, reported by sema
				}
				line, _ := li.LineCol(tok.Offset())
				sl := sourceLine{line: line}
				for _, c := range cites {
					sl.sources = append(sl.sources, model.Source{Repo: c.Repo, Path: c.Path, Start: c.Start, End: c.End, Text: text, Line: line})
				}
				lines = append(lines, sl)
			}
			attachSources(&outUC, lines)
		}
```

```go
type sourceLine struct {
	line    int
	sources []model.Source
}

// anchor is a line a //@ cite may attach to, and where its sources go.
type anchor struct {
	line    int
	sources *[]model.Source
}

// attachSources attaches each //@ line to the nearest step line at or above
// it: an action, a branch header, a note, a link or the trigger; before the
// first scenario, a use-case-level note or the use case itself (spec §14.1).
// Plain comments in between do not matter: only lines are compared.
func attachSources(uc *model.UseCase, lines []sourceLine) {
	if len(lines) == 0 {
		return
	}
	anchors := []anchor{{line: uc.Line, sources: &uc.Sources}}
	for i := range uc.Notes {
		anchors = append(anchors, anchor{uc.Notes[i].Line, &uc.Notes[i].Sources})
	}
	var walk func(steps []model.Step)
	walk = func(steps []model.Step) {
		for _, st := range steps {
			switch {
			case st.Action != nil:
				anchors = append(anchors, anchor{st.Action.Line, &st.Action.Sources})
			case st.Note != nil:
				anchors = append(anchors, anchor{st.Note.Line, &st.Note.Sources})
			case st.Link != nil:
				anchors = append(anchors, anchor{st.Link.Line, &st.Link.Sources})
			case st.Block != nil:
				b := st.Block
				if b.Split && len(b.Branches) > 0 {
					// One header for every fan-out branch: its cites go to the first.
					anchors = append(anchors, anchor{b.Line, &b.Branches[0].Sources})
				}
				for i := range b.Branches {
					if !b.Split {
						anchors = append(anchors, anchor{b.Branches[i].Line, &b.Branches[i].Sources})
					}
					walk(b.Branches[i].Steps)
				}
			}
		}
	}
	for i := range uc.Scenarios {
		sc := &uc.Scenarios[i]
		anchors = append(anchors, anchor{sc.Trigger.Line, &sc.Trigger.Sources})
		walk(sc.Steps)
	}
	sort.SliceStable(anchors, func(i, j int) bool { return anchors[i].line < anchors[j].line })
	for _, sl := range lines {
		var target *[]model.Source
		for _, a := range anchors {
			if a.line > sl.line {
				break
			}
			target = a.sources
		}
		if target != nil {
			*target = append(*target, sl.sources...)
		}
	}
}
```

   Steps hold `*Action`, `*Note` and (Task B3) `*Link` pointers, so appending through them updates the model in place. `Trigger` is a value inside the scenario, so the scenario is taken by pointer.

- [ ] **Step 6: Diagnostics** (`internal/sema/sources.go`, called from `AnalyzeFile` next to `validateSchedules`)

```go
// validateSources reports //@ lines that do not parse (malformed-source) and
// well-formed ones outside every use-case body (detached-source).
func validateSources(uri string, root syntax.SyntaxNode, file syntax.File, li green.LineIndex, hasLI bool) []model.Diagnostic {
	type span struct{ start, end green.TextSize }
	var bodies []span
	for _, uc := range file.UseCases() {
		if s, e, ok := uc.BodySpan(); ok {
			bodies = append(bodies, span{s, e})
		}
	}
	inBody := func(off green.TextSize) bool {
		for _, b := range bodies {
			if off >= b.start && off < b.end {
				return true
			}
		}
		return false
	}
	var diags []model.Diagnostic
	for _, tok := range syntax.SourceComments(root) {
		var r model.Range
		if hasLI {
			line, col := li.LineCol16(tok.Offset())
			r = model.Range{
				Start: model.Position{Line: lineToLSP(line), Character: colToLSP(col)},
				End:   model.Position{Line: lineToLSP(line), Character: colToLSP(col) + lspLen(tok.Text())},
			}
		}
		if _, _, ok := lang.ParseSourceComment(tok.Text()); !ok {
			diags = append(diags, model.Diagnostic{Code: "craft/syntax/malformed-source", Severity: model.SeverityWarning, SourceURI: uri, Range: r,
				Message: "this `//@` line is not a source cite (`//@ [repo] path[:lines] [-- text]`), so it stays a plain comment"})
			continue
		}
		if !inBody(tok.Offset()) {
			diags = append(diags, model.Diagnostic{Code: "craft/lint/detached-source", Severity: model.SeverityWarning, SourceURI: uri, Range: r,
				Message: "a `//@` cite proves the step above it; move it under that step, or make it a plain `//` comment"})
		}
	}
	return diags
}
```

- [ ] **Step 7: Fixtures**

`testdata/corpus/04_use_cases/sources.craft`: `sourcesSrc` above, preceded by `actors {\n    system Siren\n}\n\ndomain nmf {\n    nmf_monetization\n}\n` so every participant resolves.

`testdata/broken/source_malformed.craft`:

```craft
domain web {
    frontendplatform
}

use_case "U" {
    when frontendplatform listens sns.session-opened
        frontendplatform renders the page
        //@ pkg/render.go:40-12 -- reversed
}
```

`.diagnostics.json`: `craft/syntax/malformed-source`, warning, `{"start":{"line":7,"character":8},"end":{"line":7,"character":43}}`.

`testdata/broken/source_detached.craft`:

```craft
//@ pkg/topology.go:1
domain web {
    frontendplatform
}
```

`.diagnostics.json`: `craft/lint/detached-source`, warning, `{"start":{"line":0,"character":0},"end":{"line":0,"character":21}}`.

- [ ] **Step 8: Goldens, full suite, commit**

`-update`: `sources.craftjson` carries `sources` on the use case, the trigger, the first action, the `asks` and the note, each with `line`; no other golden changes (no existing fixture has `//@`: `grep -rl '//@' testdata examples` is empty).

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(sources): //@ source cites attach to the step above them (Sources); malformed-source and detached-source

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task B3: Link steps: syntax and projection — **strong**

**Files:**
- Modify: `internal/syntax/kind.go`, `internal/syntax/parser.go` (`parseSteps`, new `atLinkLine`, `parseLink`), `internal/syntax/ast.go` (`steps`, `StepDecl.Link`, new `LinkDecl`), `internal/analysis/project.go` (`projectSteps`, `fanOutGroups`, `firstNonNote`), `internal/sema/lint.go` (`countNonNotes`)
- Modify (helpers that switch on step kinds): `internal/syntax/blocks_test.go` (`outline`), `internal/lsp/formatter_test.go` (`stepOutline`), `internal/lsp/formatter_fuzz_test.go` (`fuzzOutline`), `internal/lsp/formatter.go` (`firstBodyToken`)
- Create: `internal/syntax/links_test.go`, `testdata/corpus/04_use_cases/links.craft` + golden, `testdata/broken/link_missing_title.craft` + `.diagnostics.json`

**Tree shape:** `Link: KwIncludes|KwContinues String`, a step like `Note`.

- [ ] **Step 1: Kinds.** Append before `syntaxKindTokenSentinel`: `SyntaxKindKwIncludes // includes "<title>"` and `SyntaxKindKwContinues // continues "<title>"`. Append at the end of the node block: `// SyntaxKindLink is an includes/continues step.` `SyntaxKindLink`.

- [ ] **Step 2: Write the failing tests** (`internal/syntax/links_test.go`)

```go
package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestLinks(t *testing.T) {
	src := "use_case \"U\" {\n    when Seller promotes an advert\n        includes \"Site session preamble\"\n        alt \"pay now\"\n            continues \"Seller pays at the checkout\"\n        includes ships the order\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	steps := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0].Steps()
	l, ok := steps[0].Link()
	if !ok || l.Kind() != lang.LinkIncludes || l.Title() != "Site session preamble" {
		t.Fatalf("first step = %+v", steps[0])
	}
	blk, _ := steps[1].Block()
	if l, ok := blk.Branches()[0].Steps()[0].Link(); !ok || l.Kind() != lang.LinkContinues {
		t.Error("continues inside an alt")
	}
	if a, ok := steps[2].Action(); !ok || a.SubjectName() != "includes" {
		t.Error("`includes ships the order` is an action whose subject is includes")
	}
}

func TestLinks_MissingTitle(t *testing.T) {
	_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        includes\n}")
	if len(diags) != 1 || diags[0].Code != "craft/syntax/missing-label" {
		t.Fatalf("diags = %v", diags)
	}
}
```

- [ ] **Step 3: Run** → FAIL.

- [ ] **Step 4: Parser**

```go
// atLinkLine reports whether the next tokens are a link step: `includes` or
// `continues` at the start of a line, followed by a string on that line or
// by nothing else on it (a missing title). Anything else keeps the line an
// action, so both words stay valid names.
func (p *Parser) atLinkLine() bool {
	tok := p.peek()
	if tok.Type != lexer.TokenIdent || (tok.Value != "includes" && tok.Value != "continues") || !p.peekAtLineStart() {
		return false
	}
	next := p.peekAt(1)
	return next.Type == lexer.TokenString || next.Type == lexer.TokenEOF || next.Line != tok.Line
}

// parseLink parses `includes "<title>"` or `continues "<title>"`.
func (p *Parser) parseLink() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindLink)
	kind := SyntaxKindKwIncludes
	if p.peek().Value == "continues" {
		kind = SyntaxKindKwContinues
	}
	kwTok := p.consumeAs(kind)
	if t := p.peek(); t.Type == lexer.TokenString && t.Line == kwTok.Line {
		p.consumeAs(SyntaxKindString)
	} else {
		diags = append(diags, model.Diagnostic{
			Code:     "craft/syntax/missing-label",
			Message:  fmt.Sprintf("`%s` needs a quoted use-case title, e.g. `%s \"Seller pays at the checkout\"`", kwTok.Value, kwTok.Value),
			Severity: model.SeverityError,
			Range:    tokenRange(kwTok),
		})
	}
	diags = append(diags, p.sweepRestOfLine(kwTok.Line, "end of line after the use-case title")...)
	p.builder.FinishNode()
	return diags
}
```

   In `parseSteps`: `if p.atNoteLine() { … } else if p.atLinkLine() { diags = append(diags, p.parseLink()...) } else { … parseAction … }`.

- [ ] **Step 5: Views** (`ast.go`): `steps` also lists `SyntaxKindLink` nodes;

```go
// Link returns the step as a link, if it is one.
func (s StepDecl) Link() (LinkDecl, bool) {
	if s.node.Kind() != SyntaxKindLink {
		return LinkDecl{}, false
	}
	return LinkDecl{node: s.node}, true
}

// LinkDecl is a typed view over a SyntaxKindLink node.
type LinkDecl struct{ node SyntaxNode }

// KeywordToken returns the `includes` / `continues` keyword.
func (l LinkDecl) KeywordToken() *SyntaxToken {
	return l.node.ChildToken(SyntaxKindKwIncludes, SyntaxKindKwContinues)
}

// Kind returns lang.LinkContinues for `continues`, lang.LinkIncludes otherwise.
func (l LinkDecl) Kind() lang.LinkKind {
	if t := l.KeywordToken(); t != nil && t.Kind() == SyntaxKindKwContinues {
		return lang.LinkContinues
	}
	return lang.LinkIncludes
}

// TitleToken returns the quoted title, or nil when it is missing.
func (l LinkDecl) TitleToken() *SyntaxToken { return l.node.ChildToken(SyntaxKindString) }

// Title returns the unquoted title, or "".
func (l LinkDecl) Title() string {
	if t := l.TitleToken(); t != nil {
		return unquoteStringText(t.Text())
	}
	return ""
}

// Node returns the underlying syntax node.
func (l LinkDecl) Node() SyntaxNode { return l.node }
```

   Update the `StepDecl` comment (four kinds). The four test/format helpers listed under **Files** treat `SyntaxKindLink` exactly like `SyntaxKindNote` (`firstBodyToken` accepts it as a body node; the outline helpers print `link:<kind>:<title>`).

- [ ] **Step 6: Projection.** In `projectSteps`, before the action case:

```go
		if l, ok := st.Link(); ok {
			*counter++
			link := model.Link{ID: fmt.Sprintf("link_%d", *counter), Kind: l.Kind(), Title: l.Title()}
			if tok := l.KeywordToken(); tok != nil {
				link.Line, _ = li.LineCol(tok.Offset())
			}
			if tok := l.TitleToken(); tok != nil {
				_, link.Column = li.LineCol16(tok.Offset())
			}
			out = append(out, model.Step{Link: &link})
			continue
		}
```

   A link is "not a step" for a fan-out par, like a note (spec §13): `fanOutGroups` and `firstNonNote` test `isNoteOrLink(st)` (`_, n := st.Note(); _, l := st.Link(); return n || l`) where they tested `st.Note()`; `countNonNotes` in `lint.go` (for `single-branch-par`) skips links too.

- [ ] **Step 7: Fixtures.** `testdata/corpus/04_use_cases/links.craft`: the spec §15.1 example with `actors { user Seller }`, `domain seller { frontendplatform  graphql }`, and two more use cases titled `Site session preamble` and `Seller pays at the checkout` (one step each) so both links resolve once Task B4 lands. `testdata/broken/link_missing_title.craft`:

```craft
domain web {
    frontendplatform
}

use_case "U" {
    when frontendplatform listens sns.session-opened
        includes
}
```

`.diagnostics.json`: `craft/syntax/missing-label`, error, `{"start":{"line":6,"character":8},"end":{"line":6,"character":16}}`.

- [ ] **Step 8: Goldens, full suite, commit**

`-update`: `links.craftjson` has `{"link": {"id": "link_<n>", "kind": "includes", "title": "Site session preamble", "line": …, "column": …}}` (no `target` yet) and the `continues` link; the IDs of later actions shift past the links, inside this file only. No other golden changes.

```bash
go vet ./... && go test ./...
git add internal testdata
git commit -m "feat(syntax): includes/continues link steps (Step.Link)

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task B4: Link resolution, `LintModel` link lints, steps that are not actions — **strong**

**Files:**
- Modify: `internal/model/model.go` (new `ResolveUseCaseTitle`), `internal/analysis/resolve.go` (`resolveParticipants`), `internal/sema/lint_model.go` (link lints), `internal/interaction/interaction.go` (`walk`), `internal/visualizer/view/sequence.go` (`walk` skips links until Task B5)
- Test: `internal/model/model_test.go`, `internal/analysis/lint_model_test.go`, `internal/interaction/interaction_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/analysis/lint_model_test.go
func linkWorkspace(files map[string]string) Result {
	var rs []FileResult
	for name, src := range files {
		rs = append(rs, File(name, src))
	}
	return Workspace(rs)
}

func codes(res Result, code string) int {
	n := 0
	for _, d := range res.Diagnostics {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestLinks_ResolveAcrossFiles(t *testing.T) {
	res := linkWorkspace(map[string]string{
		"a.craft": "use_case \"Promote\" {\n  when S promotes\n    x does it\n    continues \"Pay\"\n}\n",
		"b.craft": "use_case \"Pay at the till\" {\n  tags {\n    formerly_titled: \"Pay; Settle\"\n  }\n  when S pays\n    y does it\n}\n",
	})
	link := res.Doc.UseCases[0].Scenarios[0].Steps[1].Link
	if link.Target == nil || link.Target.Name != "Pay at the till" || link.Target.SourceURI != "b.craft" {
		t.Fatalf("target = %+v", link.Target)
	}
	if codes(res, "craft/lint/link-to-former-title") != 1 {
		t.Error("a link through formerly_titled warns")
	}
}

func TestLinks_Lints(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		code  string
		want  int
	}{
		{"unresolved", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Nowhere\"\n    x does it\n}\n"}, "craft/sema/unresolved-use-case", 1},
		{"ambiguous", map[string]string{
			"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Twin\"\n    x does it\n}\nuse_case \"Twin\" {\n  when S does\n    x does it\n}\n",
			"b.craft": "use_case \"Twin\" {\n  when S does\n    y does it\n}\n"}, "craft/sema/ambiguous-use-case", 1},
		{"continues not last", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    continues \"B\"\n    x does it\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/lint/continues-not-last", 1},
		{"continues then note", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    continues \"B\"\n    note \"fine\"\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/lint/continues-not-last", 0},
		{"self", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    x does it\n    continues \"A\"\n}\n"}, "craft/lint/self-link", 1},
	}
	for _, c := range cases {
		if got := codes(linkWorkspace(c.files), c.code); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
```

Interaction (`structure` helpers): a scenario that opens with `includes "X"` then `Ordering asks Billing …` triggers into `Ordering` exactly as without the link; `summary` shows no interaction for the link and steps numbered 1, 2 for the two actions after it.

- [ ] **Step 2: Run** → FAIL.

- [ ] **Step 3: Model helper**

```go
// ResolveUseCaseTitle finds the use cases a link title names: every use
// case with that exact name; when there is none, every use case whose
// formerly_titled tag lists it (values split on `;`, trimmed). viaFormer is
// true in the second case. One match is a resolved link; two or more in
// different files are ambiguous.
func ResolveUseCaseTitle(doc *CraftDoc, title string) (matches []UseCaseRef, viaFormer bool) {
	seen := map[UseCaseRef]bool{}
	add := func(uc UseCase) {
		r := UseCaseRef{Name: uc.Name, SourceURI: uc.SourceURI}
		if !seen[r] {
			seen[r] = true
			matches = append(matches, r)
		}
	}
	for _, uc := range doc.UseCases {
		if uc.Name == title {
			add(uc)
		}
	}
	if len(matches) > 0 {
		return matches, false
	}
	for _, uc := range doc.UseCases {
		for _, v := range uc.TagValues[TagFormerlyTitled] {
			for _, former := range strings.Split(v, ";") {
				if strings.TrimSpace(former) == title {
					add(uc)
				}
			}
		}
	}
	return matches, len(matches) > 0
}
```

- [ ] **Step 4: Resolution** (`resolveParticipants`): walk every scenario's steps (recursing into blocks) and, for each `st.Link`, set `Target = &matches[0]` when `len(matches) == 1`.
- [ ] **Step 5: Lints** (`lint_model.go`, added to `LintModel`)

```go
// ── links ─────────────────────────────────────────────────────────────────────
// unresolved-use-case (warning: the target may live in a file not loaded),
// ambiguous-use-case (error), link-to-former-title, continues-not-last,
// self-link. The range is the title string.

func lintLinks(doc *model.CraftDoc) []model.Diagnostic {
	var diags []model.Diagnostic
	add := func(uc model.UseCase, l *model.Link, code string, sev model.Severity, msg string) {
		start := colToLSP(l.Column)
		diags = append(diags, model.Diagnostic{Code: code, Message: msg, Severity: sev, SourceURI: uc.SourceURI,
			Range: model.Range{
				Start: model.Position{Line: lineToLSP(l.Line), Character: start},
				End:   model.Position{Line: lineToLSP(l.Line), Character: start + lspLen(l.Title) + 2},
			}})
	}
	var walk func(uc model.UseCase, steps []model.Step)
	walk = func(uc model.UseCase, steps []model.Step) {
		for i, st := range steps {
			if st.Block != nil {
				for _, br := range st.Block.Branches {
					walk(uc, br.Steps)
				}
			}
			l := st.Link
			if l == nil || l.Title == "" {
				continue
			}
			matches, former := model.ResolveUseCaseTitle(doc, l.Title)
			switch {
			case len(matches) == 0:
				add(uc, l, "craft/sema/unresolved-use-case", model.SeverityWarning, fmt.Sprintf("no use case is titled %q in the loaded files", l.Title))
			case len(matches) > 1:
				add(uc, l, "craft/sema/ambiguous-use-case", model.SeverityError, fmt.Sprintf("%q names use cases in %d files; rename one of them: a link names one use case", l.Title, len(matches)))
			case former:
				add(uc, l, "craft/lint/link-to-former-title", model.SeverityWarning, fmt.Sprintf("%q is a former title of %q; link to its current title", l.Title, matches[0].Name))
			}
			if len(matches) == 1 && matches[0].Name == uc.Name && matches[0].SourceURI == uc.SourceURI {
				add(uc, l, "craft/lint/self-link", model.SeverityWarning, "a use case links to itself")
			}
			if l.Kind == model.LinkContinues {
				for _, after := range steps[i+1:] {
					if after.Note == nil {
						add(uc, l, "craft/lint/continues-not-last", model.SeverityWarning, "`continues` ends this body: the steps after it never run here; move them to the target or use `includes`")
						break
					}
				}
			}
		}
	}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			walk(uc, sc.Steps)
		}
	}
	return diags
}
```

   (`fmt.Sprintf("%q", …)` matches the `ambiguous` message style of the other codes; keep the spec's wording where it gives one.)
- [ ] **Step 6: Steps that are not actions.** `interaction.walker.walk`: `case st.Note != nil || st.Link != nil: continue`. `view.scenarioLayout.walk`: no case for links yet (Task B5 draws them), so they are skipped. `TestSequenceStepsMatchInteractionSteps` keeps passing.
- [ ] **Step 7: Run** → PASS; `links.craftjson` regenerated (`-update`) now carries both `target`s; full suite.
- [ ] **Step 8: Commit** `feat(links): resolve link titles across the workspace (exact, then formerly_titled); lints unresolved/ambiguous use case, link-to-former-title, continues-not-last, self-link`.

---

### Task B5: Links in the sequence view (`MessageRef`) — **cheap**

**Files:** Modify `internal/visualizer/view/sequence.go` (`MessageRef`, `walk`, new `ref`, `anchor`), `internal/visualizer/plantuml/plantuml.go`, `internal/visualizer/mermaid/views.go`, `internal/visualizer/view/sequence_test.go`; Test `internal/visualizer/sequence_rendering_test.go`

- [ ] **Step 1: Failing test** (spec §15.5)

```go
const linksSrc = `actors {
    user Seller
}
domain seller {
    frontendplatform
    graphql
}
use_case "Seller asks the assistant a question" {
    when Seller asks the assistant a question
        includes "Site session preamble"
        frontendplatform asks graphql to stream the answer  [assistantStream]
}
use_case "Seller promotes an advert" {
    when Seller promotes an advert
        frontendplatform asks graphql for the promotion offers  [promotionOffers]
        continues "Seller pays at the checkout"
}
`

func TestSequence_LinksAreRefFrames(t *testing.T) {
	puml, mmd := sequenceSource(t, linksSrc)
	requireLine(t, "plantuml", puml, "ref over Seller, frontendplatform : Site session preamble")
	requireLine(t, "plantuml", puml, "ref over frontendplatform, graphql : continues: Seller pays at the checkout")
	requireLine(t, "mermaid", mmd, "Note over Seller,frontendplatform: ref: Site session preamble")
	requireLine(t, "mermaid", mmd, "Note over frontendplatform,graphql: continues: Seller pays at the checkout")
}
```

Plus: a link inside an `alt` branch with nothing else keeps the branch drawn (no `MessageNone`), and a link as the first step of a scenario is drawn across the diagram (empty `Over`). `TestSequenceStepsMatchInteractionSteps` excludes `MessageRef` from `got`, like notes.

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** `MessageRef MessageKind = "ref"` (comment: "an includes/continues link, drawn over Over; Label is the title, or `continues: <title>`"). `walk` gains `case st.Link != nil: l.ref(*st.Link, path)`:

```go
// ref draws a link step as an interaction use over the participants of the
// last message (spec §15.5).
func (l *scenarioLayout) ref(link model.Link, path []BranchRef) {
	label := link.Title
	if link.Kind == model.LinkContinues {
		label = "continues: " + link.Title
	}
	sec := &l.b.seq.Sections[l.idx]
	sec.Messages = append(sec.Messages, Message{Kind: MessageRef, Label: label, Scenario: l.title,
		Branch: slices.Clone(path), Over: l.anchor()})
}
```

   `anchor` skips `MessageRef` like a note. PlantUML: `case view.MessageRef: fmt.Fprintf(&b, "ref over %s : %s\n", noteOver(s, m.Over, al, ", "), m.Label)` (skip when `noteOver` is `""`). Mermaid: `Note over <over>: <label>` where an includes label is prefixed `ref: ` (a label that does not start with `continues: `).
- [ ] **Step 4: Run** → PASS; goldens unchanged; full suite.
- [ ] **Step 5: Commit** `feat(sequence): links draw as ref frames (PlantUML ref over, Mermaid note)`.

---

### Task B6: `internal/journey`, `BuildJourney`, `journey-list`, `journey-unconnected` — **strong**

**Files:**
- Modify: `internal/model/model.go` (`Journey`, `JourneyMember`, `JourneyEdge`), `internal/sema/lint_model.go` (journey lints), `pkg/craft/craftdoc.go` (aliases)
- Create: `internal/journey/journey.go`, `internal/journey/journey_test.go`, `pkg/craft/journey.go`, `pkg/craft/journey_test.go`
- Test: `internal/analysis/lint_model_test.go`

- [ ] **Step 1: Model types** (`model.go`; correction 13)

```go
// JourneyMember is one use case of a journey spine.
type JourneyMember struct {
	UseCase   int    `json:"useCase"` // index into CraftDoc.UseCases
	Name      string `json:"name"`
	SourceURI string `json:"sourceUri,omitempty"`
}

// JourneyEdge connects two members: From's publish, store write or
// continues link reaches To. From and To index Journey.Members.
type JourneyEdge struct {
	From  int             `json:"from"`
	To    int             `json:"to"`
	Kind  JourneyEdgeKind `json:"kind"`
	Event string          `json:"event,omitempty"` // the event or store ref; "" for continues
}

// Journey is the spine of one journey slug: its members in spine order.
type Journey struct {
	Slug    string          `json:"slug"`
	Members []JourneyMember `json:"members"`
	Edges   []JourneyEdge   `json:"edges,omitempty"`
	Cycle   bool            `json:"cycle,omitempty"`
}
```

- [ ] **Step 2: Write the failing tests** (`internal/journey/journey_test.go`)

```go
package journey_test

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
)

func docOf(t *testing.T, files map[string]string) *model.CraftDoc {
	t.Helper()
	var rs []analysis.FileResult
	for name, src := range files {
		rs = append(rs, analysis.File(name, src))
	}
	return analysis.Workspace(rs).Doc
}

func uc(name, slug, body string) string {
	return "use_case \"" + name + "\" {\n  tags {\n    journey: " + slug + "\n  }\n" + body + "}\n"
}

func order(j model.Journey) []string {
	var out []string
	for _, m := range j.Members {
		out = append(out, m.Name)
	}
	return out
}

func TestBuild(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
		cycle bool
	}{
		{"chain", map[string]string{"a.craft": uc("C", "j", "  when c listens e.B\n    c does it\n") + uc("A", "j", "  when S does\n    a notifies e.A\n") + uc("B", "j", "  when b listens e.A\n    b notifies e.B\n")}, []string{"A", "B", "C"}, false},
		{"ties by file then line", map[string]string{"b.craft": uc("Y", "j", "  when S does\n    y does it\n"), "a.craft": uc("X", "j", "  when S does\n    x does it\n")}, []string{"X", "Y"}, false},
		{"continues", map[string]string{"a.craft": uc("Checkout", "j", "  when S pays\n    p does it\n") + uc("Promote", "j", "  when S promotes\n    q does it\n    continues \"Checkout\"\n")}, []string{"Promote", "Checkout"}, false},
		{"store", map[string]string{"a.craft": uc("Read", "j", "  when r listens store.q.inserted\n    r does it\n") + uc("Write", "j", "  when S does\n    w notifies store.q.inserted\n")}, []string{"Write", "Read"}, false},
		{"roster member", map[string]string{"a.craft": uc("L", "j", "  when l listens e.X, e.A\n    l does it\n") + uc("P", "j", "  when S does\n    p notifies e.A\n")}, []string{"P", "L"}, false},
		{"cycle", map[string]string{"a.craft": uc("A", "j", "  when a listens e.B\n    a notifies e.A\n") + uc("B", "j", "  when b listens e.A\n    b notifies e.B\n")}, []string{"A", "B"}, true},
		{"other journeys ignored", map[string]string{"a.craft": uc("A", "j", "  when S does\n    a does it\n") + uc("Z", "k", "  when S does\n    z does it\n")}, []string{"A"}, false},
	}
	for _, c := range cases {
		j := journey.Build(docOf(t, c.files), "j")
		if got := order(j); !slices.Equal(got, c.want) || j.Cycle != c.cycle {
			t.Errorf("%s: got %v cycle=%v, want %v cycle=%v", c.name, got, j.Cycle, c.want, c.cycle)
		}
	}
}

func TestBuild_Edges(t *testing.T) {
	j := journey.Build(docOf(t, map[string]string{"a.craft": uc("A", "j", "  when S does\n    a notifies e.A\n    continues \"B\"\n") + uc("B", "j", "  when b listens e.A\n    b does it\n")}), "j")
	want := []model.JourneyEdge{{From: 0, To: 1, Kind: model.JourneyEdgeContinues}, {From: 0, To: 1, Kind: model.JourneyEdgeEvent, Event: "e.A"}}
	if !slices.Equal(j.Edges, want) {
		t.Errorf("edges = %+v", j.Edges)
	}
}

func TestSlugs(t *testing.T) {
	doc := docOf(t, map[string]string{"a.craft": uc("A", "b-j", "  when S does\n    a does it\n") + uc("B", "a-j", "  when S does\n    b does it\n") + uc("C", "a-j", "  when S does\n    c does it\n")})
	if got := journey.Slugs(doc); !slices.Equal(got, []string{"a-j", "b-j"}) {
		t.Errorf("slugs = %v", got)
	}
}
```

Lints (`lint_model_test.go`): a use case with `journey: a, b` gives one `craft/lint/journey-list` warning on the tag; a journey of two members with no edge gives two `craft/lint/journey-unconnected` hints (severity `hint`), one per member, on each member's `journey` value; a single-member journey gives none; the `chain` case gives none.

- [ ] **Step 3: Run** → FAIL.

- [ ] **Step 4: Implement** (`internal/journey/journey.go`)

```go
// Package journey builds a journey spine: the use cases that share a
// `journey` tag, ordered by the hand-offs between them (spec §17.2).
package journey

import (
	"sort"
	"strings"

	"github.com/tcarcao/craft/v2/internal/model"
)

// Slug is a use case's journey: the first value of its journey tag.
func Slug(uc model.UseCase) string {
	if vals := uc.TagValues[model.TagJourney]; len(vals) > 0 {
		return strings.TrimSpace(vals[0])
	}
	return ""
}

// Slugs returns every journey slug of doc, sorted.
func Slugs(doc *model.CraftDoc) []string {
	seen := map[string]bool{}
	var out []string
	for _, uc := range doc.UseCases {
		if s := Slug(uc); s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// Build returns the spine of slug: every member, edges between members, and
// a topological order over the edges (Kahn); among ready members the one
// with the smallest (source file, line) goes first, and when none is ready
// (a cycle) the remaining one with the smallest (source file, line) does.
func Build(doc *model.CraftDoc, slug string) model.Journey {
	j := model.Journey{Slug: slug}
	var members []int // doc indices, in doc order
	for i, uc := range doc.UseCases {
		if Slug(uc) == slug {
			members = append(members, i)
		}
	}
	edges := edgesBetween(doc, members) // From/To index members
	order, cycle := topoOrder(doc, members, edges)
	pos := make(map[int]int, len(order))
	for p, m := range order {
		pos[m] = p
		uc := doc.UseCases[members[m]]
		j.Members = append(j.Members, model.JourneyMember{UseCase: members[m], Name: uc.Name, SourceURI: uc.SourceURI})
	}
	for _, e := range edges {
		e.From, e.To = pos[e.From], pos[e.To]
		j.Edges = append(j.Edges, e)
	}
	sort.SliceStable(j.Edges, func(a, b int) bool {
		x, y := j.Edges[a], j.Edges[b]
		if x.From != y.From {
			return x.From < y.From
		}
		if x.To != y.To {
			return x.To < y.To
		}
		return x.Kind < y.Kind
	})
	j.Cycle = cycle
	return j
}
```

   with:
   - `edgesBetween`: for each ordered pair (A, B), A ≠ B: an `event` edge per event of an async action of A (`Action.EventList()`) that some scenario trigger of B listens to (`Trigger.EventList()`); `store` instead of `event` when either side has a `Store`; one `continues` edge when A has a `continues` link (any depth) whose `Target` is B (same `Name` and `SourceURI`). `includes` adds nothing. Deduplicate (From, To, Kind, Event).
   - `topoOrder(doc, members, edges) (order []int, cycle bool)`: in-degrees over the edges; repeatedly pick, among members with in-degree 0 not yet placed, the one with the smallest `(SourceURI, Line)`; when none has in-degree 0, pick the smallest remaining by the same key and set `cycle`; decrement its successors.

   `pkg/craft/journey.go`:

```go
package craft

import (
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
)

type (
	Journey       = model.Journey
	JourneyMember = model.JourneyMember
	JourneyEdge   = model.JourneyEdge
)

// BuildJourney orders the use cases whose journey tag is slug by the
// hand-offs between them (publish→listen, store write→read, continues
// links), ties broken by file and line. The hub orders flows with it.
func BuildJourney(doc *CraftDoc, slug string) Journey { return journey.Build(doc, slug) }

// JourneySlugs returns every journey slug of doc, sorted.
func JourneySlugs(doc *CraftDoc) []string { return journey.Slugs(doc) }
```

   with a `pkg/craft/journey_test.go` that parses two files with `craft.ParseFiles` and checks `BuildJourney` order and `JourneySlugs`.
   - Lints (`lint_model.go`, added to `LintModel`; `sema` imports `internal/journey`, which imports only `model`): `journey-list` — a use case whose `TagValues["journey"]` has two or more values, message "a use case belongs to one journey; BuildJourney uses the first value"; `journey-unconnected` (severity `hint`) — for every slug with two or more members, each member that is the `From` or `To` of no edge, message ``fmt.Sprintf("nothing in the model connects this use case to the rest of `%s`: add the notifies/listens pair, a continues link, or move it to its own journey", slug)``. Both point at the journey tag's (first) value token: find the `UseCaseDecl` in `perFileTrees[uc.SourceURI]` whose title line is `uc.Line`, then its `TagsBlock().Tags()` entry with key `journey`, and range its `Value()` token with `lis[uc.SourceURI]`.
- [ ] **Step 5: Run** → PASS; full suite.
- [ ] **Step 6: Commit** `feat(journey): BuildJourney orders a journey's use cases by publish/listen, store and continues edges; lints journey-list and journey-unconnected`.

---

### Task B7: Spine rendering: shared stage, `Request.Journey`, title — **strong**

**Files:**
- Modify: `internal/visualizer/view/sequence.go` (new `SequenceOptions`, `BuildSequenceWith`; `sequenceBuilder.everOnStage`), `internal/visualizer/visualizer.go` (`Request.Journey`, `validate`, `Render`, `write`, new `spine`)
- Create: `internal/visualizer/testdata/journey-spine/vas/nmf-books-the-draws.craft`, `…/journey-spine/vas/seller-promotes-an-advert.craft`, `internal/visualizer/journey_test.go`

- [ ] **Step 1: Fixtures** (spec §17.4): the two files exactly as the spec lists them, plus, in `seller-promotes-an-advert.craft`, `actors {\n    user Seller\n}\n` and `domain vas {\n    frontendplatform\n    graphql\n    vasscheduler\n    nmf_monetization\n}\n` at the top so every participant resolves.

- [ ] **Step 2: Write the failing test** (`journey_test.go`)

```go
package visualizer

import (
	"os"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func spineDoc(t *testing.T) *craft.CraftDoc {
	t.Helper()
	var rs []analysis.FileResult
	for _, name := range []string{"vas/nmf-books-the-draws.craft", "vas/seller-promotes-an-advert.craft"} {
		src, err := os.ReadFile("testdata/journey-spine/" + name)
		if err != nil {
			t.Fatal(err)
		}
		rs = append(rs, analysis.File(name, string(src)))
	}
	return analysis.Workspace(rs).Doc
}

const spinePUML = `@startuml
title Journey: seller-promotes-an-advert
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
participant "vasscheduler" as vasscheduler
participant "nmf_monetization" as nmf_monetization

== Seller promotes an advert ==

Seller -> frontendplatform : promotes an advert
frontendplatform -> graphql : to book the promotion [scheduleVas]
graphql -> vasscheduler : to book it [createSchedule]
vasscheduler ->> vasscheduler : notifies "com.olx.realestate_vas.VasScheduled"
ref over vasscheduler : continues: Seller pays at the checkout

== nMF books the draws of a scheduled VAS ==

vasscheduler ->> nmf_monetization : com.olx.realestate_vas.VasScheduled
nmf_monetization -> nmf_monetization : books one draw per day of the schedule

== Seller pays at the checkout ==

Seller -> frontendplatform : pays at the checkout
frontendplatform -> graphql : to pay [payInvoice]

@enduml`

func TestJourneySpine(t *testing.T) {
	doc := spineDoc(t)
	out, err := New(nil).Render(doc, Request{View: ViewSequence, Backend: PlantUML, Format: FormatSource, Journey: "seller-promotes-an-advert"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out.Data) != spinePUML {
		t.Errorf("got:\n%s", out.Data)
	}
	out, err = New(nil).Render(doc, Request{View: ViewSequence, Backend: Mermaid, Format: FormatSource, Journey: "seller-promotes-an-advert"})
	if err != nil {
		t.Fatal(err)
	}
	mmd := string(out.Data)
	if !strings.HasPrefix(mmd, "---\ntitle: \"Journey: seller-promotes-an-advert\"\n---\nsequenceDiagram\n") {
		t.Errorf("mermaid head:\n%s", mmd)
	}
	for _, want := range []string{
		"Note over vasscheduler: continues: Seller pays at the checkout",
		"vasscheduler-)nmf_monetization: com.olx.realestate_vas.VasScheduled",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
}

func TestJourneySpine_RequestChecks(t *testing.T) {
	doc := spineDoc(t)
	for _, req := range []Request{
		{View: ViewFlow, Backend: PlantUML, Format: FormatSource, Journey: "seller-promotes-an-advert"},
		{View: ViewSequence, Backend: PlantUML, Format: FormatSource, Journey: "seller-promotes-an-advert", UseCases: []string{"x"}},
		{View: ViewSequence, Backend: PlantUML, Format: FormatSource, Journey: "nope"},
	} {
		if _, err := New(nil).Render(doc, req); err == nil {
			t.Errorf("expected an error for %+v", req)
		}
	}
}
```

A second test pins "a publisher outside the spine keeps the inbound arrow with `(from …)`" (a third file, not tagged, publishing the event too) and "a cycle renders in source order".

- [ ] **Step 3: Run** → FAIL (`Request.Journey` undefined).

- [ ] **Step 4: View**

```go
// SequenceOptions change how BuildSequenceWith lays out a sequence.
type SequenceOptions struct {
	// SharedStage keeps participants on stage across sections: a Listen
	// whose publisher was drawn in an earlier section is delivered directly
	// (spec §17.4). The journey spine sets it.
	SharedStage bool
}

// BuildSequence lays out its with the default options.
func BuildSequence(doc *model.CraftDoc, its []interaction.Interaction) Sequence {
	return BuildSequenceWith(doc, its, SequenceOptions{})
}
```

   Today's `BuildSequence` body becomes `BuildSequenceWith(doc, its, opts)`. `sequenceBuilder` gains `shared bool` and `everOnStage map[string]bool`; wherever a participant is marked on stage (`onStage[x] = true`), mark `everOnStage[x]` too; the two places that test whether a publisher is on stage (the single-event `Listen` case and `addRoster`) test `b.staged(idx, key)`:

```go
// staged reports whether key is on stage for section idx: drawn in it, or,
// with a shared stage, in any section drawn before it.
func (b *sequenceBuilder) staged(idx int, key string) bool {
	return b.onStage[idx][key] || b.shared && b.everOnStage[key]
}
```

   (The store-read case keeps its store lane; `Order` and `Title` of the spec's options are the visualizer's job, below.)

- [ ] **Step 5: Visualizer**
  1. `Request` gains `// Journey draws the spine of this journey slug (sequence only): its member use cases, in spine order, on one shared stage.` `Journey string`.
  2. `validate`: with `Journey` set, the view must be `ViewSequence` and `UseCases` empty (`ErrUnsupported`: "`journey` draws a sequence over its own members; drop the use-case filter").
  3. `Render`, before the use-case filter:

```go
	if req.Journey != "" {
		j := journey.Build(doc, req.Journey)
		if len(j.Members) == 0 {
			return Output{}, fmt.Errorf("%w: no use case has journey %q", ErrUnsupported, req.Journey)
		}
		shown, its = spine(doc, its, j)
		if req.Title == "" {
			req.Title = "Journey: " + req.Journey
		}
	}
```

```go
// spine keeps a journey's members, in spine order, and renumbers the
// interactions' UseCaseIndex to index the result. Interactions are derived
// over the whole model first, so a listener still pairs with publishers
// outside the spine.
func spine(doc *craft.CraftDoc, its []interaction.Interaction, j craft.Journey) (*craft.CraftDoc, []interaction.Interaction) {
	shown := *doc
	shown.UseCases = make([]craft.UseCase, len(j.Members))
	index := make(map[int]int, len(j.Members))
	for k, m := range j.Members {
		shown.UseCases[k] = doc.UseCases[m.UseCase]
		index[m.UseCase] = k
	}
	var out []interaction.Interaction
	for _, it := range its {
		if k, ok := index[it.UseCaseIndex]; ok {
			it.UseCaseIndex = k
			out = append(out, it)
		}
	}
	return &shown, out
}
```

  4. `write` passes `view.SequenceOptions{SharedStage: req.Journey != ""}` to `BuildSequenceWith`.
  5. Titles: PlantUML keeps `withTitle` (correction 11). Mermaid with `Journey` set gets front matter instead of the `%%` comment: in `withTitle`, `if req.Journey != "" && req.Backend == Mermaid { return "---\ntitle: \"" + req.Title + "\"\n---\n" + src }`. `FormatMarkdown` puts the same source in its fence. If the Task B13 render check shows mmdc rejecting front matter, write `Note over <first>,<last>: Journey: <slug>` as the first line after `sequenceDiagram` instead (spec §17.4 fallback) and update this test.
- [ ] **Step 6: Run** → PASS; all goldens unchanged; full suite.
- [ ] **Step 7: Commit** `feat(sequence): journey spine — members in spine order on a shared stage, titled "Journey: <slug>"`.

---

### Task B8: `craft generate --journey` — **cheap**

**Files:** Modify `cmd/craft/generate.go` (flag, checks, new `generateJourneys`, `closestSlugs`); Test `cmd/craft/generate_test.go`

- [ ] **Step 1: Failing tests**

```go
func journeyFiles(t *testing.T) (string, []string) {
	t.Helper()
	tmp := t.TempDir()
	var paths []string
	for _, name := range []string{"nmf-books-the-draws.craft", "seller-promotes-an-advert.craft"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "internal", "visualizer", "testdata", "journey-spine", "vas", name))
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, src, 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return tmp, paths
}

func TestGenerateCmd_Journey(t *testing.T) {
	tmp, files := journeyFiles(t)
	root := newRootCmd()
	root.SetArgs(append(append([]string{"generate"}, files...), "--journey", "seller-promotes-an-advert", "--output", tmp))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, filepath.Base(tmp)+"-journey-seller-promotes-an-advert.puml"))
	if err != nil || !strings.Contains(string(data), "title Journey: seller-promotes-an-advert") {
		t.Fatalf("output: %v\n%s", err, data)
	}
}

func TestGenerateCmd_JourneyErrors(t *testing.T) {
	_, files := journeyFiles(t)
	for _, extra := range [][]string{
		{"--journey", "seller-promotes-an-adver"},             // unknown: lists the closest
		{"--journey", "seller-promotes-an-advert", "--type", "c4"},
		{"--journey", "seller-promotes-an-advert", "--split"},
		{"--journey", "seller-promotes-an-advert", "--use-case", "x"},
		{"--journey", "a,b", "--stdout"},
	} {
		root := newRootCmd()
		root.SetArgs(append(append([]string{"generate"}, files...), extra...))
		err := root.Execute()
		if err == nil {
			t.Errorf("%v: expected an error", extra)
		}
		if extra[1] == "seller-promotes-an-adver" && err != nil && !strings.Contains(err.Error(), "seller-promotes-an-advert") {
			t.Errorf("unknown slug error lacks the closest slug: %v", err)
		}
	}
}
```

Plus `--journey all` writes one file per journey with two or more members, and `--journey <slug> --stdout` writes the diagram to stdout.

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  - Flag: `cmd.Flags().StringSliceVar(&journeys, "journey", nil, "journey slugs to draw as one sequence each (comma-separated), or all")`.
  - Checks, before reading files: with `len(journeys) > 0`, `cmd.Flags().Changed("type") && diagType != "sequence"` → "`--journey` draws a sequence; drop `--type` or use `--type sequence`"; `split` → "`--journey` cannot be combined with --split"; `len(useCaseFilter) > 0` → "… with --use-case"; `stdout && (len(journeys) != 1 || journeys[0] == "all")` → "--stdout requires a single journey".
  - After parsing, `return generateJourneys(r, doc, journeys, base, outDir, format, stdout, cmd.OutOrStdout(), force)`:
    - `all` expands to every `craft.JourneySlugs(doc)` whose `craft.BuildJourney` has two or more members.
    - per slug: `j := craft.BuildJourney(doc, slug)`; no members → `fmt.Errorf("unknown journey %q; closest: %s", slug, strings.Join(closestSlugs(craft.JourneySlugs(doc), slug, 3), ", "))`; `j.Cycle` → `fmt.Fprintf(os.Stderr, "note: journey %s has a hand-off cycle; its members are ordered by source position\n", slug)`; render `visualizer.Request{View: visualizer.ViewSequence, Backend: backend, Format: outFormat, Journey: slug}`; write to stdout or `filepath.Join(outDir, base+"-journey-"+strings.ReplaceAll(slug, "/", "-")+"."+out.Ext)` with `writeOutput`.
    - `closestSlugs(slugs, want, n)` sorts by Levenshtein distance to `want` (a small dynamic-programming helper in the same file), then alphabetically, and returns the first `n`.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(generate): --journey <slug>[,…]|all draws a journey spine per slug`.

---

### Task B9: Server journey endpoints — **cheap**

**Files:** Modify `cmd/server/main.go` (types, handlers, routes); Test `cmd/server/main_test.go`

- [ ] **Step 1: Failing tests:** `POST /journeys` with `{"files": {"a.craft": "<tagged use case j>", "b.craft": "<tagged use case j>"}}` returns `{"success": true, "slugs": ["j"]}`; `POST /preview/mermaid/journey` with the same files and `"slug": "j"` returns `success` and `data` starting with `---\ntitle: "Journey: j"`; an unknown slug returns 400 with the error text; `POST /preview/journey` with `"format": "puml"` returns base64 PlantUML whose decoded text holds `title Journey: j`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**

```go
// JourneyRequest names a journey over several files (a journey spans files).
type JourneyRequest struct {
	Files  map[string]string `json:"files"`
	Slug   string            `json:"slug,omitempty"`
	Format string            `json:"format,omitempty"` // png, svg, pdf, puml (PlantUML preview only)
}

// JourneysResponse lists the journey slugs of the posted files.
type JourneysResponse struct {
	Success     bool               `json:"success"`
	Slugs       []string           `json:"slugs"`
	Diagnostics []craft.Diagnostic `json:"diagnostics,omitempty"`
}

// parseFiles analyses the posted files as one model.
func parseFiles(files map[string]string) (*craft.CraftDoc, []craft.Diagnostic) {
	in := make(map[string][]byte, len(files))
	for name, src := range files {
		in[name] = []byte(src)
	}
	doc, diags, _ := craft.ParseFiles(in)
	return doc, diags
}
```

   Handlers `handleJourneys`, `handlePreviewJourney` (PlantUML, `plantUMLFormat(req.Format)`, base64 as `/preview/domain` does) and `handlePreviewMermaidJourney` (source text), all through `s.render` with `visualizer.Request{View: visualizer.ViewSequence, …, Journey: req.Slug}`; an unknown slug comes back from `Render` wrapped in `ErrUnsupported` (Task B7), so `s.render` answers 400. Routes: `r.HandleFunc("/journeys", …).Methods("POST")`, `/preview/journey`, `/preview/mermaid/journey`.
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(server): /journeys, /preview/journey and /preview/mermaid/journey`.

---

### Task B10: Formatter and fuzz — **cheap**

**Files:** Test `internal/lsp/formatter_test.go`, `internal/lsp/formatter_fuzz_test.go`, `internal/syntax/usecase_body_fuzz_test.go`

- [ ] **Step 1: Pins** (append to `formatter_test.go`)

```go
func TestFormat_SourcesAndLinks(t *testing.T) {
	in := "use_case \"U\" {\n  when S does it\n     //@ a/b.go:1 -- trigger\n   includes   \"Site session preamble\"\n      alt \"x\"\n        x asks y for it  [op]\n          //@ c/d.go:5-9\n      else \"y\"\n        continues \"Pay\"\n}\n"
	want := "use_case \"U\" {\n    when S does it\n        //@ a/b.go:1 -- trigger\n        includes \"Site session preamble\"\n        alt \"x\"\n            x asks y for it  [op]\n            //@ c/d.go:5-9\n        else \"y\"\n            continues \"Pay\"\n}\n"
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
```

  The `//@` under the `asks` is a trailing body comment that `keepInBody` keeps at body depth; the one under the trigger leads the `includes` line and indents with it.
- [ ] **Step 2: Run.** Expected PASS (cites are line comments; links are steps like notes after Task B3). If the cite after the last body step is written at header depth, extend `keepInBody` to treat a `//@` comment exactly like any other full-line comment (it should already).
- [ ] **Step 3: Fuzz.** There is no lexer fuzz target (`ls internal/lexer/*fuzz*` is empty; spec §19.1 assumes one); Task B2's lexer pin covers the `//@` / `//` / `///` split. `FuzzParseUseCaseBody` seeds: the §14.1 and §15.1 examples, `"    when S does\n        //@\n"`, `"    when S does\n        includes\n"`. `FuzzFormatBranches` seeds: `links` and `sources` (the corpus files' bodies). Run each 60 s.
- [ ] **Step 4: Commit** `test(fmt): //@ cites and link steps keep their place; fuzz seeds for v2.25`.

---

### Task B11: LSP: cites and links — **strong**

**Files:**
- Modify: `internal/lsp/server.go` (`Initialize`: capabilities and `initializationOptions.sourceRoots`; `Hover`; `Definition`; `References`; `CodeAction`; `DocumentLink`; `DocumentSymbol`; `syntaxKindToCraftToken`), `internal/lsp/blocks.go` (`blockSymbols`), `internal/lsp/completion.go`
- Create: `internal/lsp/links.go`, `internal/lsp/capabilities_test.go`, `internal/lsp/journey3_test.go`

- [ ] **Step 1: Failing tests** (`journey3_test.go`, same helpers; the source uses the `links.craft` corpus file content plus a cited step, and a second file opened through another `didOpen` when a test needs two files)
  - Outline: the scenario's children include `includes: Site session preamble` and `continues: Seller pays at the checkout`, kind `Module`.
  - Definition on the title of `continues "Seller pays at the checkout"` → the `use_case "Seller pays at the checkout"` line. References on that `use_case` title → the link's location.
  - Hover on a link title → `use case "Seller pays at the checkout" in links.craft; first trigger: when Seller pays at the checkout; journey: seller-promotes-an-advert` (the journey part only when tagged).
  - Hover on a `//@` line → `cites: pkg/siren/siren.go:63-115\nattached to: nmf_monetization asks Siren to send the purchase mail` (one line per cite; ` — <text>` after a cite when the line has text). Hover on the verb of a cited step (`asks`) → `sources:\n- pkg/siren/siren.go:63-115 — a 423 counts as delivered`.
  - Completion inside `includes "` → every use-case title of the workspace (`Kind: Reference`).
  - Code action: for a `craft/lint/link-to-former-title` diagnostic sent in `context.diagnostics`, one `quickfix` action whose edit replaces `"Pay"` with `"Pay at the till"`.
  - Document links: with `initializationOptions: {"sourceRoots": {"*": "/src"}}`, a `//@ re-monetization-framework functions/x.go:5` line gives one link `file:///src/re-monetization-framework/functions/x.go#L5` over `re-monetization-framework functions/x.go:5`; a cite without repo on a step whose subject's service declares `repo: re-x` resolves through `/src/re-x/…`; without `sourceRoots`, no links.
  - Semantic tokens: `includes`/`continues` → `craft-flow-keyword` (8); the link title → `craft-usecase-string` (30); a `//@` comment → `craft-comment` (34).
  - `capabilities_test.go`: `initialize` advertises `referencesProvider`, `codeActionProvider` (with `quickfix` kind) and `documentLinkProvider`.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** (`links.go`)
  - `linkAt(file, li, offset) (syntax.LinkDecl, bool)` over every scenario's steps (recursive); `targetOf(doc, link)` finds the resolved `model.Link` in `s.ws.Doc()` by `SourceURI` and `Line` and returns its `Target`; `useCaseLocation(ws, ref)` opens the target file (`s.ws.Get(ref.SourceURI)`; `AllFiles` when the key is a path) and returns its `use_case` title range.
  - `Definition`: a link title → `useCaseLocation`. `References`: on a `use_case` title, every `model.Link` in `s.ws.Doc()` whose `Target` is that use case → its `Line`/`Column` range in the use case's `SourceURI`.
  - `Hover`: link title → the text above (`first trigger` is the target's first scenario `Trigger.Description` without `when `; journey from `journey.Slug`). `//@` token under the cursor (`syntax.IsSourceComment`) → cites (`lang.ParseSourceComment`) and the description of the element whose `Sources` hold a `Source` with that `Line`. On a step's verb or block/note/link keyword, or on `when`, of an element with `Sources` → `sources:` list appended to whatever keyword hover that token already has.
  - Completion: `detectContext` gains `ctxLinkTitle` when the line prefix matches `^\s*(includes|continues)\s+"[^"]*$`; items are the titles of `s.ws.Doc().UseCases`, sorted, deduplicated.
  - `CodeAction`: for each diagnostic with code `craft/lint/link-to-former-title` in `params.Context.Diagnostics`, find the link at its range start, its target's current name, and return `protocol.CodeAction{Title: "Link to the current title \"" + name + "\"", Kind: protocol.QuickFix, Diagnostics: []protocol.Diagnostic{d}, Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{uri: {{Range: d.Range, NewText: quoteCraftString(name)}}}}}`.
  - `DocumentLink`: `Initialize` reads `params.InitializationOptions` as `map[string]any`, and `sourceRoots` as `map[string]string`, into `s.sourceRoots`. For each `//@` token of the file and each cite: repo = the cite's `Repo`, else the attached element's subject's service `Repo` (from `s.ws.Doc().Services`), else the service name; dir = `sourceRoots[repo]`, else `sourceRoots["*"] + "/" + repo`; no dir → no link. Target `file://<dir>/<path>` plus `#L<start>` when a line is cited; range = the cite's `At`/`Len` inside the token.
  - Capabilities: `ReferencesProvider: true`, `CodeActionProvider: &protocol.CodeActionOptions{CodeActionKinds: []protocol.CodeActionKind{protocol.QuickFix}}`, `DocumentLinkProvider: &protocol.DocumentLinkOptions{}`.
  - Outline: `blockSymbols` emits a `Module` symbol for each link step (`includes: <title>` / `continues: <title>`). Tokens: `SyntaxKindKwIncludes`/`KwContinues` → `craft-flow-keyword`; the link's string → `craft-usecase-string` (through `applyStringTypeOverrides`, as for a note's text).
- [ ] **Step 4: Run** → PASS; full suite (the `initialize` → `shutdown` test still passes).
- [ ] **Step 5: Commit** `feat(lsp): links — outline, definition, references, hover, title completion, former-title quick fix; cites — hover and document links`.

---

### Task B12: LSP: journeys (hover, definition, code lens) — **strong**

**Files:** Modify `internal/lsp/server.go` (`Hover`, `Definition`, `CodeLens`, `Initialize`), new `internal/lsp/spine.go`; Test `internal/lsp/journey3_test.go`, `internal/lsp/capabilities_test.go`

- [ ] **Step 1: Failing tests** (open the two `journey-spine` fixtures as two documents)
  - Hover on `seller-promotes-an-advert` in a `journey:` tag → `journey seller-promotes-an-advert: 3 use cases\n1. Seller promotes an advert (vas/seller-promotes-an-advert.craft)\n2. nMF books the draws of a scheduled VAS (vas/nmf-books-the-draws.craft)\n3. Seller pays at the checkout (vas/seller-promotes-an-advert.craft)`, plus a last line `cycle` when the journey has one.
  - Definition on that value in "Seller promotes an advert" → two locations, the other members' `use_case` lines, in spine order.
  - Code lens: one lens per `journey` tag, on its line, with command title `3 use cases in this journey` (command id `""`, no arguments).
  - `capabilities_test.go`: `codeLensProvider` is advertised.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** (`spine.go`, named so it is not confused with v2.23's `journeyHover`): `journeyTagAt(file, li, line, char) (slug string, ok bool)` (the cursor inside a `TagStmt` whose key is `journey`, on its first value token); `spineHover` from `journey.Build(s.ws.Doc(), slug)` (file names as stored in `SourceURI`, relative to the workspace root when it has one); `spineDefinition` (members except the one under the cursor → their `use_case` title locations via Task B11's `useCaseLocation`); `CodeLens` returns `protocol.CodeLens{Range: <tag value range>, Command: &protocol.Command{Title: fmt.Sprintf("%d use cases in this journey", n)}}` for every `journey` tag of the file. `Initialize` advertises `CodeLensProvider: &protocol.CodeLensOptions{ResolveProvider: false}`. No preview command in the LSP (the extension previews through the server).
- [ ] **Step 4: Run** → PASS; full suite.
- [ ] **Step 5: Commit** `feat(lsp): journey tag hover (spine order), definition to the other members, code lens`.

---

### Task B13: Visualizer goldens and render check — **cheap**

**Files:** Create `internal/visualizer/testdata/journey-semantics-3.craft`, goldens `journey-semantics-3.*`, `golden/journey-spine.sequence.{puml,mmd}`; Modify `internal/visualizer/golden_test.go`, `internal/visualizer/render_integration_test.go`

- [ ] **Step 1:** `journey-semantics-3.craft`: the §14.1 example (cites), the §15.1 example (both links, with the two target use cases), declarations so everything resolves. Append it to `goldenModels`. Add `TestGolden_JourneySpine`: render `spineDoc` (Task B7) with `Journey: "seller-promotes-an-advert"` in both backends into `golden/journey-spine.sequence.puml` / `.mmd` (same `-update` flag).
- [ ] **Step 2:** `go test ./internal/visualizer/ -run 'TestGolden' -update`; review: cites draw nothing; links draw `ref over`/`Note over` lines; flow, architecture and C4 goldens of the new model hold no link or cite text. `git status` shows only new files.
- [ ] **Step 3:** `TestJourneySemantics3_*Renders` and `TestJourneySpine_*Renders` in `render_integration_test.go`; `go test -tags integration ./internal/visualizer/ -run 'JourneySemantics3|JourneySpine'`. If mmdc rejects the front matter, apply Task B7's fallback. If PlantUML rejects `ref over A, B : …` for a single participant list, it accepts `ref over A : …`; check both forms are what the writer emits.
- [ ] **Step 4:** Full suite; commit `test(visualizer): cite/link and journey spine goldens; render checks`.

---

### Task B14: Documentation — **cheap**

**Files:** `docs/GRAMMAR.md`, `docs/DIAGNOSTICS.md`, `CONTEXT.md`, `README.md`, `docs/page/language/use-cases.md`

- [ ] **Step 1: GRAMMAR.md.** §8: `step = action | block | note_stmt | link_stmt ;` and `link_stmt = ('includes' | 'continues') STRING ;`. §10 (Lexer tokens): the `//@` source comment with the spec §14.1 EBNF and the attachment rule. §11.1: the spec §12.1 table. A new subsection "Recognised tag keys": `sites`, `envs` (v2.24), `journey`, `formerly_titled` (v2.25); every other key is free text.
- [ ] **Step 2: DIAGNOSTICS.md.** `craft/syntax/malformed-source`, `craft/lint/detached-source`, `craft/sema/unresolved-use-case` (warning), `craft/sema/ambiguous-use-case` (error), `craft/lint/link-to-former-title`, `craft/lint/continues-not-last`, `craft/lint/self-link`, `craft/lint/journey-list`, `craft/lint/journey-unconnected` (hint); widen `craft/syntax/missing-label` (a bare `includes`/`continues`).
- [ ] **Step 3: CONTEXT.md:** **Source cite**, **Link**, **Journey (spine)**.
- [ ] **Step 4: README.md / use-cases page:** `//@` cites, `includes`/`continues`, and `craft generate --journey` with the §17.1 command.
- [ ] **Step 5: Commit** `docs: journey semantics 2, part B (source cites, links, journey spines)`.

---

### Task B15: tree-sitter-craft: grammar, corpus, release 0.5.0 — **strong**

**Repo:** `~/projects/github/tcarcao/tree-sitter-craft` (branch `feat/journey-semantics-3` off `main` at `v0.4.0`)

- [ ] **Step 1: Failing corpus tests** (`test/corpus/journey_semantics_3.txt`): `//@` lines under an action, under a trigger and at file level (all `source_comment`, never `comment`); `// @ x` (a plain comment); `includes "X"`, `continues "Y"` as steps and inside an `alt`; `includes ships the order` and `continues to run` as actions (risk cases).
- [ ] **Step 2: Grammar.**

```js
  extras: $ => [
    /[ \t]/,
    $._newline,
    $.doc_comment,
    $.source_comment,
    $.comment,
  ],
  …
    source_comment: $ => token(prec(1, /\/\/@[^\n]*/)),

    link_stmt: $ => prec.dynamic(10, seq(choice('includes', 'continues'), $.string)),
```

   `link_stmt` joins the `when_clause` repeat (`choice($.action, $.branch_header, $.scenario_name, $.note_stmt, $.link_stmt)`) and `scenario_continuation`; `'includes'` and `'continues'` join `_branch_keyword`.
- [ ] **Step 3: Highlights.**

```scheme
(source_comment) @comment.documentation
(link_stmt ["includes" "continues"] @craft.flow-keyword)
(link_stmt (string) @craft.usecase-string)
```

- [ ] **Step 4: Generate and test** (`npx tree-sitter generate && npx tree-sitter test`, no `ERROR`/`MISSING`); commit `feat(grammar): //@ source comments and includes/continues link steps`.
- [ ] **Step 5: CORPUS_VERSION** → `v2.25.0` after the coordinator's tag; fetch and parse the craft corpus as in Task A18 Step 5; commit `chore: bump CORPUS_VERSION to v2.25.0`.
- [ ] **Step 6: Release** `./release.sh minor` → `v0.5.0`; `npm view tree-sitter-craft version` → `0.5.0`.

---

### Task B16: VSCode extension: bump, `craft.previewJourney`, cite links, release 0.0.13 — **strong**

**Repo:** `~/projects/github/tcarcao/craft-vscode-extension` (branch `feat/journey-semantics-3` off `main` at 0.0.12)

**Files:** `package.json` (command, setting, dependency), `server/package.json`, `client/src/commands/index.ts`, new `client/src/commands/previewJourney.ts`, `client/src/commands/previewCommon.ts` (request helper), `client/src/types/api.ts`, new `client/src/providers/sourceLinkProvider.ts`, `client/src/extension.ts` (registration), `server/src/parser/TreeSitterCompletionProvider.ts` (+ test)

- [ ] **Step 1: Failing tests** (jest)
  - Completion: a blank use-case body line offers `includes` and `continues` snippets (`includes "${1:use case title}"`); inside `includes "` the titles of the open document's `use_case` lines.
  - `sourceLinkProvider`: for the text `        //@ re-x pkg/a.go:5-9 -- why` and `craft.sources.roots = {"*": "/src"}`, one link to `file:///src/re-x/pkg/a.go#L5` over `re-x pkg/a.go:5-9`; for `{}` no links; a cite without repo yields no link (the extension does not resolve services; the LSP does).
  - `previewJourney`: `pickDefaultSlug(text, line)` returns the `journey:` value of the enclosing use case's `tags` block, or `undefined`.
- [ ] **Step 2: Run** `npm test` → FAIL.
- [ ] **Step 3: Implement**
  - `package.json`: command `craft.previewJourney` ("Craft: Preview Journey"); setting `craft.sources.roots` (`object`, default `{}`, description "Directories that hold the repos `//@` cites name: {\"<repo>\": \"<dir>\", \"*\": \"<dir holding every repo>\"}"); `tree-sitter-craft` `^0.5.0` in both `package.json` files.
  - `previewJourney.ts`: collect every `**/*.craft` of the workspace (`workspace.findFiles`) into `{path: text}`; `POST ${serverUrl}/journeys`; `window.showQuickPick(slugs, {placeHolder})` with the default slug first; `POST ${serverUrl}/preview/journey` with `{files, slug, format: 'png'}`; show `response.data` in the existing preview panel (`updateWebviewContent(panel, 'Journey', …)`). Errors go through `handlePreviewError`.
  - `sourceLinkProvider.ts`: a `DocumentLinkProvider` for `craft` documents that scans lines starting (after whitespace) with `//@`, parses cites with the same rules as `lang.ParseSourceComment` (head before ` -- `, cites split on `, `, optional repo, `:` ranges), and builds the links from `craft.sources.roots`; registered in `extension.ts`.
  - Completion as in Step 1.
- [ ] **Step 4: Bump and check.** `npm install && (cd server && npm install) && npm run compile`; in the Extension Development Host, open the `journey-spine` fixtures, run "Craft: Preview Journey" against a local `craft` server (`go run ./cmd/server` in the craft repo), and click a `//@` link.
- [ ] **Step 5: Commit and release.** `npm test` → PASS; commit `feat: preview a journey spine; //@ cite links; includes/continues completions; tree-sitter-craft 0.5.0`; merge to `main`, push, `./release.sh 0.0.13`.

---

### Task B17: Hub: craft v2.25.0, sources on steps, link edges, spine order — **strong**

**Repo:** `~/Downloads/domain-hub/svc-re-go-arch-context`, the branch of Task A20.

**Files:**
- Modify: `go.mod`, `go.sum`; `internal/infrastructure/graphstore/model.go` (`step.Sources`, `useCase.Sources`, `useCase.Links`), `craft_loader.go` (`loadJourneys`), the graph build that turns use cases into nodes/edges (`translate.go`/`use_case.go`: new `includes`/`continues` use_case→use_case edges; flow-contains edges gain `Order`; journey hand-off edges), `internal/core/domain/use_case.go` (`Source`, `UseCaseLink`, `Step.Sources`, `UseCase.Links`, `UseCase.Order`), `internal/presentation/rest/graphhdl/response.go`, `internal/presentation/mcp/explainusecasehdl/response.go`, `internal/infrastructure/graphstore/graphstore.go` (`FindUseCases` journey order), `web/src/lib/api.ts`, `web/src/views/journey.ts`
- Create: `internal/infrastructure/graphstore/craft_v225_test.go`

- [ ] **Step 1: Bump** to `v2.25.0` (as Task A20 Step 1).
- [ ] **Step 2: Failing tests**
  - A step with a `//@ repo-x a/b.go:5-9 -- why` cite carries `Sources == [{Repo: "repo-x", Path: "a/b.go", Start: 5, End: 9, Text: "why"}]`; a use-case-level cite lands on the use case.
  - `continues "B"` in use case A gives an edge A → B of kind `continues` and `ExplainUseCase(A)` lists `continues in: B`; `includes "C"` gives kind `includes` and `includes: C`.
  - Three use cases of journey `j` (the spec §17.4 example) come back from `FindUseCases({Journey: "j"})` in spine order (promote, books, checkout), each with `Order` 0, 1, 2, and the journey view's hand-offs are the two `craft.BuildJourney` edges.
- [ ] **Step 3: Run** → FAIL.
- [ ] **Step 4: Implement**
  1. Loader: `step.Sources` from the action or trigger the step came from (every step of a list carries the action's sources); `useCase.Sources = uc.Sources`; `useCase.Links` from a walk over every scenario's steps (`Kind`, `Title`, `Target`).
  2. Graph: one use_case → use_case edge per resolved link (an unresolved link is a validation issue `unresolved_link`, reusing the free-text issue's severity). `craft.BuildJourney(doc, slug)` for each journey slug: the flow-contains-use_case edges get `Order` (spine position), and each journey edge becomes a flow-internal hand-off edge (kind `event`/`store`/`continues`, with the event).
  3. Domain/API: `domain.Source{Repo, Path string; Start, End int; Text string}`, `domain.UseCaseLink{Kind, Title, TargetID string}`; `Step.Sources`, `UseCase.Links`, `UseCase.Order`; REST fields `sources`, `links`, `order` (omitempty); `arch_explain_use_case` prints the links (`continues in: <title>`, `includes: <title>`) and, under each step, its cites (`evidence: repo path:start-end`; Task B18 adds the status).
  4. `FindUseCases` with `Journey` sorts by `Order`; `web/src/views/journey.ts` lists members in that order with the hand-offs between them and a `cycle` badge when `BuildJourney` reports one.
- [ ] **Step 5: Run** `make lint && make test` → PASS.
- [ ] **Step 6: Commit and push** `feat(graphstore): craft v2.25 — source cites on steps, includes/continues edges, journey spine order and hand-offs`.

---

### Task B18: Hub: cite freshness service — **strong**

**Repo:** same branch.

**Files:**
- Modify: `internal/core/outboundprt/repo_inspector.go` (new methods), `internal/infrastructure/gitlabgw/repo_inspector.go` (GitLab blame and commits calls), `internal/core/outboundprt/graph_repository.go` (`ListCites`), `internal/infrastructure/graphstore/graphstore.go` (`ListCites` from the loaded steps), `internal/core/domain/` (new `cite_freshness.go`), `internal/core/inboundprt/hub_service.go`, `internal/presentation/mcp/explainusecasehdl/response.go`, the validation report
- Create: `internal/core/application/citefreshsvc/service.go` + `service_test.go`; regenerate mocks (`make mockgen`)

- [ ] **Step 1: Failing tests** (`service_test.go`, with mockery mocks of `RepoInspector`, `CatalogDirectory` and `GraphRepository`)
  - repo resolution: a cite with `Repo: "re-x"` maps through the catalog directory to project `olxeu/realestate/re-x`; a cite without repo uses the step subject's service; neither resolves → `unresolved`.
  - path: `ListPaths(project, HEAD, "", "eb_email_sender.go")` with one match → that path; two → `ambiguous`; none → `broken`.
  - range past the end of the file (the blame's last line) → `broken`.
  - dates: the cite line's last change in the central repo (blame of the `.craft` file at the cite's `Line`) older than the newest commit touching the cited range (blame of the cited file over `Start..End`) → `stale`; otherwise `fresh`. A cite with no range compares against the file's last commit (commits API with `path`).
  - results are cached by (project, path, HEAD SHA): a second run with the same HEAD makes no new blame call.
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  1. `outboundprt.RepoInspector` gains `FileBlame(ctx, projectPath, ref, path string) ([]domain.BlameRange, error)` (`GET /projects/:id/repository/files/:path/blame?ref=`; each range: first/last line and the commit's `committed_date`) and `LastCommitForPath(ctx, projectPath, ref, path string) (time.Time, error)` (`GET /projects/:id/repository/commits?path=&ref_name=&per_page=1`), implemented in `gitlabgw` with the existing `getJSON`.
  2. `domain.CiteStatus` (`fresh`, `stale`, `broken`, `ambiguous`, `unresolved`) and `domain.CiteCheck{UseCaseID string; StepIndex int; Source Source; Project, Path string; Status CiteStatus}`.
  3. `citefreshsvc.Service.Check(ctx)`: reads every cite from the graph (`GraphRepository.ListCites`), resolves, checks and caches; runs after each graph reload (wire it where the reloader adapter fires) and on demand; never blocks graph serving (errors are logged, statuses `unresolved`).
  4. `arch_explain_use_case` prints each cite with its status; the validation report counts `stale`, `broken`, `ambiguous` and `unresolved` cites per file.
  5. The central repo's project path is `Config.CentralRepoProjectPath`; the `.craft` path is the use case's `SourceFile` under the corpus directory the mirror uses.
- [ ] **Step 4: Run** `make mockgen && make lint && make test` → PASS.
- [ ] **Step 5: Commit and push** `feat(hub): cite freshness — resolve each //@ cite in GitLab and report fresh, stale, broken, ambiguous or unresolved`. Opening the MR needs the user's go-ahead.

---

### Task B19: Dogfood on re-arch-central (verification only) — **cheap**

- [ ] **Step 1:** Build the local `craft` (as in Task A22).
- [ ] **Step 2:** Validate the corpus with the new binary and with v2.24.0. Expect **0 errors** in both. The only new codes are `craft/lint/journey-unconnected` hints (about 69 journeys' members, spec §17.3) and no `malformed-source`/`detached-source` (the corpus has no `//@` yet: `grep -rc '//@' docs/journeys | grep -v ':0' | wc -l` → 0). Any other difference is a bug.
- [ ] **Step 3:** `$CRAFT generate docs/topology.craft docs/actors.craft 'docs/journeys/*/*.craft' --journey seller-promotes-an-advert --format mermaid --output ~/Downloads/domain-hub/journey-assessment/_render-journey-semantics-3/` (18 use cases in 9 files, spec §19.1): the file renders in mermaid-cli; check by eye that a publisher drawn earlier delivers directly. Then copy two files to a temp dir, convert three cite comments to `//@` and one `// Follows …` header into `includes "…"`, validate (0 errors) and render.
- [ ] **Step 4:** Nothing is committed in re-arch-central. Record the counts for Task B20.

---

### Task B20: Release prep v2.25.0 (CHANGELOG) — **cheap**

- [ ] **Step 1: CHANGELOG.md.** `## [2.25.0] — <date>`:
  - `### Added`: `//@` source cites (`Source`, `Sources` on use cases, triggers, actions, branches, notes and links); `includes`/`continues` link steps (`Step.Link`, `Link`, `UseCaseRef`); `craft generate --journey <slug>[,…]|all`; `pkg/craft.BuildJourney`, `JourneySlugs`, `Journey`, `JourneyMember`, `JourneyEdge`, `JourneyEdgeKind` and the `Link*`/`Tag*`/`JourneyEdge*` constants; server `/journeys`, `/preview/journey`, `/preview/mermaid/journey`; the codes `malformed-source`, `detached-source`, `unresolved-use-case`, `ambiguous-use-case`, `link-to-former-title`, `continues-not-last`, `self-link`, `journey-list`, `journey-unconnected`; LSP references, document links (with `initializationOptions.sourceRoots`), code lens and the former-title quick fix.
  - `### Changed`: a comment that starts with `//@` is read as a source cite (no known uses; a malformed one is a warning and stays a comment). `Step` may now be a `Link`.
  - `### Changed — diagrams`: links draw as `ref over` frames (Mermaid notes).
- [ ] **Step 2:** Spec status `**Status:** Implemented (v2.24.0, v2.25.0)`.
- [ ] **Step 3:** Full suite and the integration render check → PASS.
- [ ] **Step 4: Commit** `chore(release): v2.25.0` with the dogfood counts in the body (files: `CHANGELOG.md` and the spec). The coordinator tags.

---

## Self-review notes

- **Spec coverage (Part I):**
  - §2 decisions: A → A3, A4; B → A5; C → A2; D → A11, A12; E → A6, A7, A8; F → A9, A10.
  - §3.1 contextual words: A3 (list comma), A7 (`one`/`of`, with a stays-a-name test), A11 (`stream`, subject test), A9 (`inferred` as head only; payload on receives/surfaces), A9 (`ADDR`/`SQL`), A2 (kinds need no grammar). §3.2 kinds: A3 (all four tokens and `EventList`). §3.3 vocabulary: A1 (+ `SplitOperationItems` in A6). §3.4 IDs: no new IDs; A3/A7 goldens unchanged for files without the constructs.
  - §4: syntax A3; model A1, A3; sema/lint A3 (`store-ref-in-event-list`), A4 (`duplicate-event-in-list`, per-item `dead-event`/`event-not-past-tense`), A3 (dangling comma); interaction A4; rendering A4 (one arrow; flow/C4 unchanged by construction); formatter A13; LSP A14, A15; tests A3, A4, A13.
  - §5: decision (no grammar); lint A5; rendering A5; LSP A14, A15; hub A21; tests A5, A15.
  - §6: A2 (rendering, C4, `unused-actor` pin), A14 (hover), A15 (completion), A16 (goldens); `isInsideSystem` needs no change (kinds resolve as actors).
  - §7: syntax/model/`Answers`/lint A11; rendering A12; LSP A14, A15; hub A20; tests A11, A12.
  - §8: operation lists A6; `one of` A7, A8; lints A6–A8; rendering A8 (correction 1); formatter A13; LSP A14, A15; hub A20.
  - §9: A9 (syntax, model, lints, correction 2), A10 (rendering, C4), A14/A15 (LSP), A20 (hub `shared_tables`, egress, confidence).
  - §10: `pkg/craft` A1; tree-sitter A18 (correction 5); VSCode A19; hub A20, A21; docs A17. §11.1 cross-cutting: goldens unchanged (every task's run step), `TestSequenceStepsMatchInteractionSteps` (A4, A8), fuzz (A13), dogfood (A22). §11.2: A23. §11.3 migration: out of scope (after release).
- **Spec coverage (Part II):** §12 vocabulary B1, B3; §13 steps that are not actions B3, B4; §14 cites B2 (syntax, model, attachment, diagnostics), B10 (formatter), B11 (LSP), B15/B16 (tree-sitter, VSCode), B17/B18 (hub); §15 links B3 (syntax), B4 (resolution, lints), B5 (rendering), B10, B11, B15, B16, B17; §16 recognised tags B1, B6 (`journey-list`); §17 spine B6 (`internal/journey`, `BuildJourney`, `journey-unconnected`), B7 (rendering), B8 (CLI), B9 (server), B12 (LSP), B13 (goldens), B16 (VSCode), B17 (hub); §18 consumers B1, B6, B15–B17, B14; §19 B10, B13, B19, B20; §19.3 migration out of scope.
- **Names used consistently:**
  - lang: `ActorScheduler/Tool/Infra`, `KnownActorTypes`, `OpVerbAddr`, `OpVerbSQL`, `OpInferred`, `TagSites`, `TagEnvs`, `OpItem`, `SplitOperationItems`, `ParseOperationItem`, `UnquoteAddress`, `Cite`, `ParseSourceComment`, `LinkKind`, `LinkIncludes/LinkContinues`, `TagJourney`, `TagFormerlyTitled`, `JourneyEdgeKind`, `JourneyEdgeEvent/Store/Continues`.
  - model: `Trigger.Events`, `Action.Events/Operations/Stream/Answers/Inferred`, `EventList()`, `OperationList()`, `Operation.Tables()`, `ActorTypeScheduler/Tool/Infra`, `Source`, `UseCaseRef`, `Link`, `Step.Link`, `Sources`, `ResolveUseCaseTitle`, `Journey`, `JourneyMember`, `JourneyEdge`.
  - syntax: `SyntaxKindKwOne/KwOf/KwStream/KwInferred`, `SyntaxKindEventList`, `SyntaxKindKwIncludes/KwContinues`, `SyntaxKindLink`; `EventItem`, `TriggerDecl.EventItems`, `ActionDecl.EventItems/IsOneOf/OneOfTokens/IsStream/IsInferred`, `LinkDecl`, `StepDecl.Link`, `UseCaseDecl.BodySpan`, `IsSourceComment`, `SourceComments`; parser `refEndAhead`, `eventItemAhead`, `parseEventItem`, `parseEventList`, `parseOpAnnotationWith`, `parseActionOpAnnotation`, `checkOpItems`, `diagEmptyOpAnnotationItem`, `atLinkLine`, `parseLink`.
  - sema: `LintModel(doc, perFileTrees, lis)`, `validateEventLists`, `validateSources`, `lintDuplicateEventInList`, `lintOneOfSingleEvent`, `lintEgressOnInternalStep`, `lintUnquotedAddress`, `lintStreamWithoutCaller`, `lintDuplicateScenarioBodies`, `lintLinks`, `opItemRange`.
  - interaction/view: `Interaction.Operations/OneOf/Inferred/Stream`, `AnsweredRequests`, `openRequest`, `frame.request`; `capList`, `addRoster`, `addOneOf`, `sectionTitle`, `opLabel`, `seqArrow`, `requestTechnology`, `Message.Inferred/Stream`, `MessageRef`, `FlowEdge.Inferred`, `SequenceOptions`, `BuildSequenceWith`, `staged`; visualizer `Request.Journey`, `spine`; `internal/journey` `Build`, `Slugs`, `Slug`.
  - CLI/server/LSP: `--journey`, `generateJourneys`, `closestSlugs`; `JourneyRequest`, `JourneysResponse`, `parseFiles`; `ctxEventItem`, `ctxLinkTitle`, `sourceRoots`, `spineHover`, `spineDefinition`.
  - hub: `journeyStepsFromTrigger`, `journeyStepsFromAction`, `step.Egress/Tables/Confidence/Streaming/Sources`, `validateSharedTables`, `validateTagValues`, `scopeMatches`, `KnownSites/KnownEnvs`, `citefreshsvc`.
- **Placeholders:** none. Where a step depends on a value only the code can settle (a column in an LSP test, a hub file the v2.23 tasks reshape), the step says what to check and what must not change.
