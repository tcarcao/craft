# Schedules, cross-use-case events and triggers into branches — Design Spec

**Date:** 2026-10-14
**Owner:** Tiago Carção
**Status:** Implemented (v2.22.0)
**Release:** v2.22.0 (additive; no breaking change)
**Drives:** the three craft gaps found while piloting the conventions v2 rewrite of re-arch-central
journeys (`docs/journey-assessment/pilots/` there): G14 (schedules), the sequence view's handling of
cross-use-case hand-offs, and G1-style client branches as a scenario's first step.

## 1. Scheduled triggers carry a schedule (G14)

### Today
`when cron "0 7 * * 1-5"` and `when every "5m"` parse (`internal/syntax/parser.go`), but projection treats
them as external triggers with `actor: "cron"` / `"every"` and the quoted expression as `verb`. The
diagrams draw an invented actor named `cron`, the declared scheduler actor (`CRON` in most models) is
reported unused, and nothing validates the expression.

### Change
- `model.Trigger` gains:
  ```go
  // Schedule is the expression of a `when cron "…"` or `when every "…"` trigger, unquoted.
  Schedule     string           `json:"schedule,omitempty"`
  // ScheduleKind is "cron" or "every" for a scheduled trigger, "" otherwise.
  ScheduleKind lang.ScheduleKind `json:"scheduleKind,omitempty"`
  ```
  and `lang` gains `ScheduleCron = "cron"` and `ScheduleEvery = "every"`.
- A scheduled trigger projects as `type: external`, `actor: CRON`, `verb: ""`, `Schedule` set, and
  `description: when cron "0 7 * * 1-5"`. `CRON` is the conventional scheduler actor name. It resolves
  like any actor written in a trigger, so a model that declares `actor system CRON` sees it used, and
  one that doesn't gets the usual undeclared-participant warning.
- **Labels:** the trigger arrow is drawn from `CRON` with the label `cron 0 7 * * 1-5` or `every 5m`.
- **Validation (sema)** also accepts the cron macros `@yearly`, `@annually`, `@monthly`, `@weekly`, `@daily`, `@midnight` and `@hourly`, and rejects `@every` (use `when every`). The `cron`/`every` keyword carries a reference to `CRON`, for the undeclared warning, hover and definition.
- **Validation (sema):**
  - `craft/sema/invalid-schedule` (error) when a cron expression doesn't have 5 or 6 space-separated
    fields of `[0-9*/,\-?LW#A-Za-z]`;
  - the same code when an `every` duration isn't a positive Go-style duration (`time.ParseDuration`, plus
    `d` for days).
  - Messages name the expected form.
- **Hub:** consumers read `Trigger.Schedule`/`ScheduleKind`. The actor `CRON` groups scheduled journeys
  with the hand-written `when CRON fires …` ones.

## 2. An event from another use case names its publisher

### Today
`view.BuildSequence` draws a listen from its publisher only when the publisher already appears in the
same use-case section. Otherwise it draws an inbound arrow (`[-> consumer : event`, Mermaid
`External->>consumer`) that says nothing about where the event comes from. Every hand-off between journeys
(`sns.`, `sqs.`, `eventbus.`, `store.`) reads as "from outside".

### Change
- The inbound message label names the publisher(s) the interaction layer already paired:
  `event (from <publisher>)`, or `event (from <a>, <b>)` for several, sorted by display name and capped at 3
  with `+N more`. An event with no publisher in the model keeps today's bare label.
- No new participant is drawn: the section stays about its own use case. Only the label grows.
- Same rule in both backends, via the view (writers stay dumb).

## 3. A trigger whose first step is a block reaches every branch

### Today
`deriveScenario` targets the trigger at `sc.Actions()[0]`, the first action in source order. When a
scenario opens with `alt "on the web" / else "in the app"`, the trigger is drawn to the first branch's
subject only, and the other branches get no incoming arrow.

### Change
- When the scenario's first step is a block, `Derive` emits one Trigger interaction per branch, targeting
  that branch's first action subject. It recurses if a branch also opens with a block. Each such trigger
  carries that branch's `Branch` path.
- A branch with no action gets no trigger. A first step that is an action keeps today's single trigger
  with a nil `Branch`.
- When the opening block is an `opt`, the path where it is skipped is a real path too: `Derive` also emits
  one Trigger with a nil `Branch`, to the first action after the block (when there is one). The view draws
  it just before that action, so the trigger appears inside the opt box and again before the first step
  after it. An `alt` always takes one of its branches, so it gets no such trigger. An actor-is-subject
  trigger is still a note.
- A branch is triggered when it has a target; the first action in source order having no subject does not
  stop the other branches from being triggered.
- The call stack for each branch starts with that branch's trigger frame; the per-branch stack rules of
  v2.21.0 are unchanged.
- The sequence view therefore draws the trigger inside each branch box: `alt on the web` →
  `Seeker -> frontendplatform : opens …`, `else in the app` → `Seeker -> mobile : opens …`. The same
  dedup rules apply (an actor-is-subject trigger is a note).
- Docs: `Interaction.Branch` comment ("Trigger and Listen always have a nil Branch") becomes "Listen
  always has a nil Branch; a Trigger has one only when its scenario opens with a block (an opening `opt` also
  gets one unbranched Trigger for its skipped path)".

## Testing
TDD per item; golden diffs reviewed; `go vet`, `go test ./...` and the fuzz target stay green.
- (1) projection, label, validation (valid and invalid cron/every), resolution against a declared `CRON`
  actor, and the hub-facing JSON (`schedule`, `scheduleKind`). The corpus fixture
  `04_use_cases/scheduled_triggers.craft` gets a golden.
- (2) labels with one, two and four publishers, and no publisher, in PlantUML and Mermaid.
- (3) a trigger per branch with paths; nested first blocks; an empty first branch; the call stack; sequence
  output with the trigger inside each branch.

## Out of scope
Spine view (G7), `par` (G9), store hand-off construct (G8), surfaces (G5).
