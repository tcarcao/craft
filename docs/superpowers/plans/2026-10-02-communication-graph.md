# Communication Graph — implementation plan (step 3)

Status: done (2026-10-02).

Implements rulings 12–15 of `docs/decisions/diagram-pipeline-deepening.md`.
Input: the resolved `CraftDoc` (steps 1–2). Output: every diagram drawn from
one derivation of "who talks to whom", through views both backends read.

## Shape

```
CraftDoc (resolved) ──► interaction.Derive ──► []Interaction
                                   │
          ┌────────────┬───────────┼──────────────┬──────────────┐
          ▼            ▼           ▼              ▼              ▼
   SequenceView    FlowView  ArchitectureView   C4View     context-map lints
          └────────────┴─────┬─────┴──────────────┘
                     plantuml/ and mermaid/ writers (text only)
```

### `internal/interaction` (leaf: model, lang)

```go
type Kind string // Trigger, Request, Response, Publish, Listen, Internal

type Endpoint struct {
    Key     string                // Participant.ID, or the written name when unresolved
    Name    string                // display name (see "Names" below)
    Kind    model.ParticipantKind // "" when unresolved — still drawn
    Service string
}

type Interaction struct {
    UseCase, Scenario string
    Step      int        // source position: 0 = trigger, n = n-th action
    Kind      Kind
    From, To  Endpoint   // To is zero for Publish; From is External for a
                         // Listen whose event has no publisher
    Event     string
    Label     string     // full action description (PlantUML's buildActionDescription)
    Operation *model.Operation
    Site      Site       // URI, line, column of the subject — for diagnostics
}

func Derive(doc *model.CraftDoc) []Interaction
```

Rules, each implemented once:
- **Trigger**: external actor → the `From` of the scenario's first action.
- **Request** (sync), **Internal** (self), **Publish** (async, to the event).
- **Response**: explicit `returns to X`, else the caller popped from the
  scenario's call stack (seeded with the triggering actor), else `External`.
- **Listen**: every publisher × every listener of the event (ruling 13),
  owned by the listener's scenario; no publisher → from `External`.
- Output order: use case, scenario, step — source order, deterministic.

### Names

An endpoint's display name is the bounded context's bare name, unless two
drawn endpoints share it (the same name in two domains), in which case both
show the qualified `<domain>/<bc>`. Unresolved names display as written.
Identifiers in diagram source are derived by the writer from `Key`, never from
the display name, so `re/billing` can no longer produce invalid PlantUML.

### Views (`internal/visualizer`)

Pure functions, plain sorted data, filters already applied:
- `SequenceView`: participants in first-appearance order; one section per use
  case; messages with kind, label and per-section number.
- `FlowView` (domain, detailed): context frames, actor nodes (with actor type),
  one queue per publishing context, numbered edges (numbering counts only
  drawn edges, contiguous per diagram).
- `ArchitectureView` (domain, architecture): contexts grouped by owning
  service, ungrouped contexts, queues, unlabeled deduplicated edges, no
  self-loops.
- `C4View`: actors (triggering ones only), systems (internal/external by
  focus) holding containers (context or application per boundaries mode,
  databases if shown, presentation/gateway layers, event queue only when an
  async interaction is in scope), relations with label and technology.
  Technology and icon are data (`Icon: "go"`); writers map them to sprites.

### Writers

`plantuml.{Sequence,Flow,Architecture,C4}(view) string` and the same in
`mermaid`. No lookups, no `if action.Type`. Mermaid C4 reaches parity.

### Lint

`craft/lint/{unclassified-communication,relationship-direction-inverted,
relationship-bidirectional,separate-ways-violation}` move from sema's tree walk
to `analysis`, reading BC→BC dependencies from interactions (Request: subject
depends on target; Listen: listener depends on publisher). `buildDependencyEdges`
is deleted. The diagnostics goldens must stay byte-identical.

## Positions

Lint diagnostics are positioned at the subject's column. Decided 2026-10-02:
`model.Action` gains `Column` and `model.Trigger` gains `Line`/`Column` (the
`when` line and the subject's column), additive and `omitempty`, so
interactions carry full sites and lint never walks the trees for them.

## Sequence of commits

1. `interaction.Derive` + table tests (pairing, returns, triggers, qualified
   refs, unresolved names, determinism).
2. Sequence, flow and architecture views + both writers + golden tests; switch
   the existing entry points to them; delete `sequence.go`, `domain.go`,
   `mermaid/{sequence,domain}.go` logic.
3. C4 view + both writers + golden tests; delete `C4DiagramGenerator`.
4. Communication lints onto interactions; delete `buildDependencyEdges`.
5. Regenerate `testdata/visualize-baseline`, retire the baseline script in
   favour of Go goldens, changelog.

Golden tests render each corpus model through every view and both backends,
20 times each, asserting identical output; `-update` rewrites them.
