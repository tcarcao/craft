# Journey Semantics: concurrency, loops, findings, unobserved callers, store hand-offs, surfaces — Design Spec

**Date:** 2026-10-14
**Owner:** Tiago Carção (design decisions delegated; each one is recorded with its reason in §13)
**Status:** Proposed
**Release:** v2.23.0, additive. No `pkg/craft` name, field or JSON key changes or disappears. One
existing input changes meaning (`when X receives …`, §5.1); neither the corpus nor `testdata/` uses it.
**Builds on:** `2026-10-14-scenario-branches-and-names-design.md` (v2.21.0: Step/Block/Branch, offside
rule) and `2026-10-14-schedules-cross-use-case-events-branch-triggers-design.md` (v2.22.0: schedules,
named publishers, triggers into branches).
**Drives:** re-arch-central journey assessment, gaps G1, G5, G8, G9, G12, G13 and G18
(`docs/journey-assessment/GOOD-JOURNEY.md` §4 there). Evidence also comes from the pilot reports in
`domain-hub/journey-assessment/pilots/`, sections "Craft gaps hit".

## 1. Problem and corpus evidence

The v2.21/v2.22 rewrite wave removed the scenario comments. The comments that remain say things that a
diagram and the hub still cannot see. These counts come from re-arch-central `docs/journeys/` (363 `.craft`
files), measured on 2026-10-14 after the tidy wave.

| Gap | What the corpus does today | Count |
|---|---|---|
| **G9** concurrent calls | A gateway resolves fields concurrently, but the calls are written one after another (`ad/seeker-advert-detail.craft:20-24`: five `graphql asks ad/atlas-advert … [postSeoadvertsbatch]`, `[postAdverttargetingbatch]`, …). The diagram shows an order that does not exist. | 39 lines that mention concurrency (`concurrent`, `in parallel`, `fans out`, `errgroup`) in 32 files |
| **G18** loops, retries, polling | `otodomintegration walks the ad ids of one Obido investment and syncs each advert's promotion`; `advertdiscount fetches a whole investment's promotions … once for each investment listed`. | 246 step lines in 137 files use loop words (`for each`, `one at a time`, `walks`, `until`, `retries`). This is an upper bound: some are prose. |
| **G12** findings | Some findings are written as steps: `obidoanalytics holds no contract with the far side, so a column rename upstream breaks this silently`. Others are comments: `// FINDING`, `// DEFECT`, `dead`, `the weekend guard never fires on schedule` (pilot atlas-invoice-sms-reminders). | 85 internal steps start with `never`/`ignores`/`has no`/`holds no`/`keeps no`/`offers no`/`does not`. 7 `FINDING`, 6 `DEFECT`, 68 `dead`, 14 `NOT TRACED` comment lines. |
| **G13** unobserved caller | A fake `system NoObservedCaller` actor (`docs/actors.craft:29`, "NOT a system") draws a fake lane: `when NoObservedCaller sweeps the monolith…` and then `graphql asks ad/atlas-advert …`. | 65 `when NoObservedCaller` triggers in 36 files, and 69 step lines whose subject is `NoObservedCaller` in 30 files. 40 files in total. In about 31 of these triggers the first step's subject is a context. |
| **G8** store hand-offs | The interim convention (DOMAIN-MAP convention 2) is `notifies store.<table>.<change>` / `listens store.<table>.<change>` plus a `polls` tag. The pilot report says: "`[-> atlas_billing : store.sms_queue.inserted` instead of an arrow from the producer." Most hand-offs are still comments. | 3 `store.` refs (2 notifies, 1 listens) in 1 file, 1 `polls` tag. 196 `TERMINUS` comment lines. 103 internal steps in 65 files write a table, row, queue, Redis key, outbox or bucket, and 55 steps read or poll one. |
| **G5** surfaces | R3a interim: one use case per boundary, tagged `scope: surface`. There are also "surface register" files, for example `gateway/gateway-operations-with-no-observed-caller.craft` ("SURFACE REGISTER, NOT A JOURNEY", 23 Atlas operations). | 1 R3a use case (`obido/obido-concierge-operator-back-office.craft` "Concierge back-office reads", 4 ops). At least 5 register files: the gateway file above, `call-tracking/calltracking-surface.craft`, `obido/obido-analytics-read-api.craft` (11 `NoObservedCaller asks`), `media/image-collection-doors-with-no-observed-caller.craft`, `location/legacy-sphere-proxy.craft`. |
| **G1** client alternatives | `frontendplatform asks graphql … [autocomplete]` and then `mobile asks graphql … [autocomplete]` in one step list. They are alternatives, but they are drawn as two calls in a row. | Baseline: 28 scenarios in 16 files. Today: 13 step lists in 11 files, of which 8 lists in 7 files use the **same** operation bracket. 6 `alt "on the web"` / `else "in the app"` pairs already exist. |

## 2. Decisions at a glance

| # | Item | Decision |
|---|---|---|
| A | G9 / G18 | New block kinds `par "…"` (with `and "…"`) and `loop "…"`. They reuse the alt/opt header-plus-offside machinery. A `par` with no `and` runs each body step concurrently ("fan-out form"). Native `par`/`loop` frames in both backends. |
| B | G12 | New step statements `note "…"` and `finding "…"`, allowed in a scenario or branch body, at use-case level before the first scenario, and in a surface. They are drawn as sequence notes; a finding is coloured and prefixed with `Finding:`. Exported as `Step.Note`. They are not actions. |
| C | G13 | New trigger form `when <ctx> receives <phrase> [op]`. It is drawn as a found message (`[->`), and an unaddressed reply to it is drawn as a lost message (`-->]`). This retires `NoObservedCaller`. There is no event-side form (§12). |
| D | G8 (+ G14 follow-up) | No new action grammar. `store.<name>.<change>` refs are recognised as a **store hand-off**: a database participant, writer → store and reader → store arrows, writer/reader pairing. A listens trigger takes an optional trailing schedule, `when X listens <ref> cron "…"` or `every "…"`, which replaces the `polls` tag. |
| E | G5 | New top-level `surface <boundary> { … }` block with `callers:`, `spec:`, notes/findings and `query`/`command` operation lines. It has no sequence view. `craft generate --type surface` writes a Markdown table with computed "used by". New lint `operation-not-in-surface`. |
| F | G1 | **No grammar.** Use a canonical client plus the `channels` tag, or an opening `alt` (which v2.22 already triggers per branch). New lint `repeated-request` points at the remaining 8 cases. |

## 3. Shared mechanics

### 3.1 Contextual keywords
All new words are contextual, like `scenario`/`alt`/`opt`/`else` (GRAMMAR.md §11):

| Word | Is a keyword only when | Otherwise |
|---|---|---|
| `par`, `and`, `loop` | at the start of a step line and followed by a STRING, or alone on the line (`missing-label`) | an identifier (subject, verb, phrase word) |
| `note`, `finding` | at the start of a step line, at use-case level before the first scenario, or in a surface body; followed by a STRING, or alone on the line | an identifier |
| `receives` | in a trigger, as the second token after the subject (the slot where `listens` is recognised) | an identifier |
| `cron`, `every` (listens tail) | directly after the event ref of a `listens` trigger, on the same line, followed by a STRING | an identifier |
| `surface` | at top level, followed by a name and `{` on the same line | an identifier |
| `callers`, `spec` | in a surface body, followed by `:` | an identifier |
| `query`, `command` | at the start of a surface body line | an identifier |

Only `receives` changes the meaning of input that parsed before: `when X receives …` used to be an
external trigger with actor `X` and verb `receives`. There are zero uses in re-arch-central, `testdata/`,
`examples/` and the sibling repos (grep of 2026-10-14). The CHANGELOG lists this under **Changed** (§11).
Every other new form was a syntax error before, so files that parsed keep their meaning.

### 3.2 Syntax kinds
`internal/syntax/kind.go` keeps its numbering. New token kinds are appended just before
`syntaxKindTokenSentinel`: `KwPar`, `KwAnd`, `KwLoop`, `KwNote`, `KwFinding`, `KwReceives`, `KwSurface`,
`KwCallers`, `KwSpec`, `KwQuery`, `KwCommand`. New node kinds are appended at the end:
`SyntaxKindNote` (a step), `SyntaxKindSurfaceDecl`, `SyntaxKindSurfaceField`, `SyntaxKindSurfaceOperation`.
`par`/`loop` reuse `SyntaxKindBlock` / `SyntaxKindBranch` / `SyntaxKindBranchHeader`. The listens schedule
reuses `KwCron`/`KwEvery` + `String` inside the existing `Trigger` node.

### 3.3 Vocabulary (`internal/lang`, ruling 19)
```go
const (
    BlockPar  BlockKind = "par"  // every branch runs, concurrently
    BlockLoop BlockKind = "loop" // the single branch repeats while its condition holds
)
const TriggerReceive TriggerKind = "receive" // when <ctx> receives <phrase> [op]

type NoteKind string
const (
    NoteText    NoteKind = "note"    // context for the reader
    NoteFinding NoteKind = "finding" // a defect, risk or negative fact the hub lists
)

type SurfaceOpKind string
const (
    SurfaceQuery   SurfaceOpKind = "query"   // no side effect (Tier A)
    SurfaceCommand SurfaceOpKind = "command" // changes state (Tier B)
)

const StoreTransport = "store" // the ref prefix of a store hand-off: store.<name>.<change>
type StoreChange string
const (
    StoreInserted StoreChange = "inserted"
    StoreUpdated  StoreChange = "updated"
    StoreDeleted  StoreChange = "deleted"
)

const UnknownCaller = "unknown" // the surface `callers:` value for "no caller observed"
```
`internal/model` and `pkg/craft` re-export each one as an alias, as they already do for `BlockKind`.

### 3.4 IDs
Notes inside a use case get `note_<n>` from the per-file counter that scenarios, actions and blocks
share, in source order. As before, a file that uses none of the new constructs gets the same IDs as in
v2.22.0. Surface entries have no IDs.

## 4. Item A: `par` / `and` and `loop` (G9, G18)

### 4.1 Syntax
```craft
use_case "Seeker reads an advert" {
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        par "field resolvers"
            graphql asks atlas for the SEO title  [postSeoadvertsbatch]
            graphql asks atlas for the targeting data  [postAdverttargetingbatch]
            graphql asks atlas where the advert came from  [postAdsourcebatch]
        loop "for each image, until all are signed"
            graphql asks media to sign the image URL  [signUrl]
}
```
A branch that has several steps in sequence uses `and`:
```craft
        par "the advert"
            graphql asks adcore for the advert  [GET /ads]
            graphql asks atlas for the owner  [getAdsMappings]
        and "the statistics"
            graphql asks Dapi for the page statistics  [GET /projects/{site}_ad_statistics]
```

Rules. These are the v2.21 rules (§3.1 there) unless stated otherwise:
1. `par "<label>"` opens a concurrent block. `and "<label>"` adds a branch to the nearest open `par` whose
   header is at the same column, in the same way that `else` adds one to an `alt`.
2. **Fan-out form.** A `par` with no `and` runs each step of its body (an action or a nested block) as
   its own concurrent branch. Notes do not count as steps, neither for the split nor for
   `single-branch-par`: a note joins the branch of the step before it, or the first branch when it comes
   before every step. A `par` with at least one `and` has one sequential branch per header. The fan-out form exists because every corpus case is "one caller issues N independent
   requests". With the explicit form, each of the N requests would need an `and "…"` header whose label
   repeats its step (§13 decision 2).
3. `loop "<condition>"` opens a block with exactly one branch, like `opt`. The label is the condition or
   the iteration ("for each owner", "until ten polls").
4. The offside rule, tab width, comments-as-trivia and nesting to any depth are the same as for alt/opt.
   `par`, `and` and `loop` nest inside every block kind, and every block kind nests inside them.
5. An `else` cannot continue a `par` or a `loop`, and an `and` cannot continue an `alt`, `opt` or `loop`.

### 4.2 Diagnostics
| Code | When | Recovery |
|---|---|---|
| `craft/syntax/orphan-and` (new, error) | An `and` has no open `par` at its column. | Parsed as the first branch of a new `par` with that label. |
| `craft/syntax/orphan-else` (message widened) | An `else` whose open block at its column is a `par`, `loop` or `opt`, or that has no block there. Message: "`else` continues an `alt` at the same column; a `par` continues with `and`". | Unchanged. |
| `craft/syntax/empty-branch`, `craft/syntax/missing-label`, `craft/syntax/inconsistent-indent` | Same conditions as for alt/opt, now also for `par`, `and` and `loop`. | Unchanged. |
| `craft/lint/single-branch-par` (new, warning) | A `par` with no `and` whose body has exactly one step that is not a note, or holds only notes. Nothing in it runs concurrently. | n/a |

### 4.3 Model
`Block.Kind` gains `par` and `loop`, and `Block` gains one field:
```go
// Split is true for a par written without `and`: each Branch holds one step
// of the header's body, and every Branch carries the header's Label.
Split bool `json:"split,omitempty"`
```
The projection (`analysis.projectBlock`) does the split. The model therefore always says "one Branch per
concurrent path", and no consumer has to know the fan-out rule. Each split branch's `Line` is the line of
its step.

### 4.4 Interaction semantics
- **Call stack.** Each `par` branch starts from the stack as it was at the `par`, like an `alt` branch.
  A `loop` body continues from the current stack, like an `opt`. After any block the stack is restored to
  its state at the block's start. The body of a `loop` is derived once: diagrams show one iteration.
- `BranchRef` gains `Split bool`, copied from the block.
- **A scenario that opens with a `par`.** A single trigger, with a nil `Branch`, goes to the first action
  in source order, and the view draws it before the `par` box. Each other branch whose first action's
  subject differs from that target also gets a trigger inside it, with that branch's path. The usual
  fan-out (all branches start at the same client) therefore draws one trigger arrow, not N copies.
  Branches that start at different participants still each show what reaches them.
- **A scenario that opens with a `loop`.** One unbranched trigger to the first action, drawn before the
  `loop` box. The actor starts the work once; the loop is not a repeated trigger.
- Alt and opt keep the v2.22 rules. `sequence.go`'s `opensBlock` becomes true only when the opening
  block is an `alt` or `opt`.

### 4.5 Rendering
**Sequence.** The writers already print `<kind> <label>` on `BlockOpen`. Two changes:
- On `BlockElse`, PlantUML prints `else`. Mermaid prints `and` when the block is a `par` and `else`
  otherwise.
- `branchPath` blanks the label of a split branch with index > 0, so the writer prints a bare
  `else`/`and`. A writer never prints a trailing space after an empty label; this also fixes the existing
  `else ` for an empty `alt` label.

The §4.1 example (actor `Seeker` declared as `user`), PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seeker" as Seeker
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
participant "atlas" as atlas
participant "media" as media

== Seeker reads an advert ==

Seeker -> frontendplatform : opens an advert
frontendplatform -> graphql : for the advert [ad]
par field resolvers
graphql -> atlas : for the SEO title [postSeoadvertsbatch]
else
graphql -> atlas : for the targeting data [postAdverttargetingbatch]
else
graphql -> atlas : where the advert came from [postAdsourcebatch]
end
loop for each image, until all are signed
graphql -> media : to sign the image URL [signUrl]
end

@enduml
```
Mermaid:
```
sequenceDiagram
    actor Seeker as Seeker
    participant frontendplatform as frontendplatform
    participant graphql as graphql
    participant atlas as atlas
    participant media as media
    Note over Seeker,media: Seeker reads an advert
    Seeker->>frontendplatform: opens an advert
    frontendplatform->>graphql: for the advert [ad]
    par field resolvers
    graphql->>atlas: for the SEO title [postSeoadvertsbatch]
    and
    graphql->>atlas: for the targeting data [postAdverttargetingbatch]
    and
    graphql->>atlas: where the advert came from [postAdsourcebatch]
    end
    loop for each image, until all are signed
    graphql->>media: to sign the image URL [signUrl]
    end
```
The Mermaid render test (`render_integration_test.go`, mermaid-cli) must accept the bare `and` of a
split `par`. If mermaid-cli rejects it, the Mermaid writer repeats the `par` label after each `and`
instead; PlantUML keeps the bare `else`.

(The asks labels show the v2.22 behaviour, where the verb is dropped (defect D1). This spec does not
change that.)

**Flow.** The edge prefix names the kind for the new blocks:
`3. [par: field resolvers] for the SEO title` and `6. [loop: for each image, until all are signed] to
sign the image URL`. `alt`/`opt` keep `[label]`, so the existing goldens do not change.

**Architecture and C4.** No change. Blocks add no edges.

### 4.6 Formatter, LSP
- **Formatter:** the body sits 4 spaces deeper than its header, and `and` aligns with its `par`, as for
  `else`. `orphan-and` joins the refusal set (`formatter.go`, the codes that make `craft fmt` refuse a
  file).
- **Outline:** `par: field resolvers` (fan-out form), `par: the advert | the statistics`,
  `loop: for each image, until all are signed`. They nest by depth. Folding ranges come for free, because
  they are computed from `SyntaxKindBlock`.
- **Hover** on a header: the kind, the branch count, and for the fan-out form "each step runs
  concurrently".
- **Completion:** the snippets `par "…"`, `and "…"` (offered only inside an open `par`) and `loop "…"`,
  next to the existing alt/opt snippets.

## 5. Item B: `note` and `finding` (G12)

### 5.1 Syntax
```craft
use_case "Seller pays an invoice" {
    finding "nothing retries a failed callback; a lost callback leaves the invoice open"
    when Seller pays an invoice
        frontendplatform asks graphql to pay  [payInvoice]
        note "the gateway forwards the seller's session, not a service token"
        graphql asks paymentgateway to start the payment  [POST /payments]
        finding "no idempotency key: a client retry starts a second payment"
}
```
- A note is one line: the keyword and one STRING (STRING never crosses a newline). If you need more, write
  several notes. Tokens after the string are `craft/syntax/unexpected-token`, wrapped in an error node.
  A bare `note`/`finding` is `craft/syntax/missing-label` ("needs a quoted text").
- **Placement:** a note can be any step of a scenario or branch body (it obeys the offside rule like any
  step line), a use-case-level line after `tags { }` and before the first `scenario`/`when`, or a line in
  a surface body (§8). A note after the first scenario belongs to the scenario above it, because steps
  run until `when`, `scenario` or `}`.
- A note never opens or closes a block. It may be the only step of a branch, which then is not an
  `empty-branch`.

### 5.2 Model
```go
type Note struct {
    ID     string   `json:"id,omitempty"` // "note_<n>" inside a use case; "" in a surface
    Kind   NoteKind `json:"kind"`         // "note" | "finding"
    Text   string   `json:"text"`         // unquoted
    Line   int      `json:"line,omitempty"`
    Column int      `json:"column,omitempty"`
}

// Step is exactly one of Action, Block or Note.
type Step struct {
    Action *Action `json:"action,omitempty"`
    Block  *Block  `json:"block,omitempty"`
    Note   *Note   `json:"note,omitempty"`
}
```
`UseCase` gains `Notes []Note \`json:"notes,omitempty"\`` for use-case-level notes. `Actions()` and
`EachAction` skip notes. Notes are not actions: they get no `Interaction`, they do not advance
`Interaction.Step`, and they do not count towards a hub's `AuthoredActions` or the R12 size budget.

### 5.3 Sema / lint
There are no new codes. Because a note is a statement, `craft/lint/indistinguishable-scenarios` is
unchanged: notes do not make two scenarios distinguishable.

### 5.4 Rendering
**Sequence.** `scenarioLayout.walk` already walks `sc.Steps`. A note step appends a message of kind
`MessageStepNote` (note) or `MessageFinding` (finding), with `Over []string` and the note's branch path,
so it lands inside its branch box. **Anchor rule:** the note goes over the participants of the last
message drawn before it in the same scenario, the trigger included: `From` and `To`, deduplicated, and
only `To` for a found or inbound message. When nothing has been drawn yet, `Over` is empty, which means
"across the diagram": the first and last participants, as with the Mermaid section title. Use-case-level
notes are drawn at the top of the section, across the diagram.

`view.Message` gains `Over []string` (participant IDs). `MessageKind` gains `MessageStepNote` and
`MessageFinding`. Writers draw a finding with the text prefix `Finding: ` in both backends, and also in
`#FFE4E1` in PlantUML (Mermaid has no per-note colour).

The §5.1 example in PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
participant "paymentgateway" as paymentgateway

== Seller pays an invoice ==

note over Seller, paymentgateway #FFE4E1 : Finding: nothing retries a failed callback; a lost callback leaves the invoice open
Seller -> frontendplatform : pays an invoice
frontendplatform -> graphql : to pay [payInvoice]
note over frontendplatform, graphql : the gateway forwards the seller's session, not a service token
graphql -> paymentgateway : to start the payment [POST /payments]
note over graphql, paymentgateway #FFE4E1 : Finding: no idempotency key: a client retry starts a second payment

@enduml
```
Mermaid (`;` becomes `,`, as for every Mermaid label):
```
sequenceDiagram
    actor Seller as Seller
    participant frontendplatform as frontendplatform
    participant graphql as graphql
    participant paymentgateway as paymentgateway
    Note over Seller,paymentgateway: Seller pays an invoice
    Note over Seller,paymentgateway: Finding: nothing retries a failed callback, a lost callback leaves the invoice open
    Seller->>frontendplatform: pays an invoice
    frontendplatform->>graphql: to pay [payInvoice]
    Note over frontendplatform,graphql: the gateway forwards the seller's session, not a service token
    graphql->>paymentgateway: to start the payment [POST /payments]
    Note over graphql,paymentgateway: Finding: no idempotency key: a client retry starts a second payment
```
A `MessageStepNote` or `MessageFinding` counts as a drawn message for the "empty branch" check, so a
branch that holds only a note gets no `MessageNone`.

**Flow, architecture, C4.** Notes are not drawn.

### 5.5 Formatter, LSP
- **Formatter:** a note is a step line, indented like an action of its body. A use-case-level note sits
  at the `when` column. A blank line before the first `when`/`scenario` that follows use-case-level notes
  is kept as it is for `tags`.
- **Outline:** findings appear as children of their scenario (or use case), named
  `finding: <first 60 characters>`, with symbol kind `Event`. Notes stay out of the outline, to keep it
  short.
- **Hover** on the keyword: "note — drawn over the previous step" or "finding — drawn highlighted and
  exported to CraftDoc (`Step.Note`)".
- **Completion:** the snippets `note "…"` and `finding "…"` in a step position and in a surface body.
- **Semantic tokens:** the keyword as `keyword`, the text as `string`.

## 6. Item C: unobserved caller, `when <ctx> receives` (G13)

### 6.1 Syntax
```craft
use_case "Adverts are swept for reindexing" {
    when graphql receives a reindex sweep  [reindexAdverts]
        graphql asks atlas for every advert that needs reindexing  [getAlladvertstoreindex]
        graphql returns the advert ids
}
```
`trigger = 'when' name_slot 'receives' connector_word? phrase? op_annotation? NL`. The subject accepts
the qualified `<domain>/<name>` form, as a `listens` subject does. The operation bracket is optional but
expected (R6), and it reuses `opAnnotationStart` / `parseOpAnnotation`.

This replaces the convention-11 statement "the caller is unresolved" (the `NoObservedCaller` actor). The
trigger names the **callee**. Craft does not invent a caller lane. It draws the UML found message, which
is the standard notation for "sender unknown".

### 6.2 Model
- `Trigger.Type = "receive"`, with `Context` (the callee as written), `Connector`, `Phrase`, and a new
  field `Operation *Operation \`json:"operation,omitempty"\`` (used only by this kind).
  `Subject` is the resolved callee.
- `Description`: `when graphql receives a reindex sweep [reindexAdverts]`. The bracket is included, so
  two `receives` scenarios that differ only by operation are distinguishable for
  `indistinguishable-scenarios`.

### 6.3 Sema / lint
- The callee resolves like any participant (the usual undeclared-participant warning).
- `craft/lint/receives-on-actor` (new, warning): the subject resolves to an actor. "An actor starts work
  with `when <Actor> <verb> …`; `receives` is for a boundary whose caller is not observed."

### 6.4 Interaction semantics
- `interaction.Unobserved = Endpoint{Key: "Unobserved", Name: "unobserved caller"}`. It is a separate
  value from `External` (an event source outside the model), because the hub and the views need to tell
  the two apart.
- The trigger is one `Trigger` interaction: `From: Unobserved`, `To: callee`, label
  `connector + phrase`, `Operation` set. Its target is explicit, so v2.22's "trigger the first action /
  each branch" rules do not apply. A scenario that opens with a block still gets this single trigger,
  before the block, drawn up front: `opensBlock` stays false for a `receives` scenario whatever its
  first step is.
- The call stack is seeded with `frame{caller: Unobserved, callee: callee}`. An unaddressed
  `graphql returns …` therefore answers `Unobserved`.

### 6.5 Rendering
`view.MessageKind` gains `MessageFound` (from `Unobserved`) and `MessageLost` (a reply to `Unobserved`).

PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

participant "graphql" as graphql
participant "atlas" as atlas

== Adverts are swept for reindexing ==

[-> graphql : a reindex sweep [reindexAdverts]
graphql -> atlas : for every advert that needs reindexing [getAlladvertstoreindex]
graphql -->] : returns the advert ids

@enduml
```
Mermaid has no found or lost message. The writer declares `participant Unobserved as unobserved caller`,
once, after `External` and only when a found or lost message exists:
```
sequenceDiagram
    participant graphql as graphql
    participant atlas as atlas
    participant Unobserved as unobserved caller
    Note over graphql,atlas: Adverts are swept for reindexing
    Unobserved->>graphql: a reindex sweep [reindexAdverts]
    graphql->>atlas: for every advert that needs reindexing [getAlladvertstoreindex]
    graphql-->>Unobserved: returns the advert ids
```
- **Flow:** an `unobserved caller` node (`NodeExternal`), with a `StyleTrigger` edge to the callee.
- **Architecture and C4:** nothing for the found or lost message. There is no known party, which is the
  same treatment as `External`.

### 6.6 Formatter, LSP
- **Formatter:** the trigger line is formatted like any trigger. Its bracket joins the annotation
  alignment run of the action lines below it.
- **Hover** on `receives`: "callee-side trigger: the caller is not observed; drawn as a found message".
- **Completion:** after `when <name> ` offer `receives` next to `listens`.
- **Semantic tokens:** `receives` is a keyword. Go-to-definition on the subject works as for `listens`.

## 7. Item D: store hand-offs and polled listeners (G8, G14 follow-up)

### 7.1 Syntax: no new action grammar
The corpus convention is already the right shape: the producer `notifies store.<name>.<change>` and the
consumer `when <ctx> listens store.<name>.<change>`. Craft now gives that ref prefix a meaning instead of
treating it as an opaque event. `store` is a **transport** in the sense of DOMAIN-MAP convention 2.

A ref is a **store ref** when it is written bare (not quoted) and its first segment is `store`. The
segments after `store.` are split as follows: when the last one is `inserted`, `updated` or `deleted`, it
is the change and the segments before it (joined with `.`) are the store name. Otherwise the change is
empty and every remaining segment is the name. `store.sms_queue.inserted` → name `sms_queue`, change
`inserted`. `store.redis.leads.pending` → name `redis.leads.pending`, no change (warned, §7.3).

**The one grammar addition** is an optional trailing schedule on any `listens` trigger. It is the cadence
slot that the atlas-invoice pilot report asked for:
```craft
    when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"
    when leadimporter listens sqs.lead-requests every "5m"
```
`domain_listen = 'when' name_slot 'listens' (STRING | ref) (('cron' | 'every') STRING)? NL`. This
replaces the `polls` tag (DOMAIN-MAP "Tags"; 1 use in the corpus).

### 7.2 Model
```go
// StoreRef is a store hand-off parsed from a store.<name>.<change> ref.
type StoreRef struct {
    Name   string      `json:"name"`
    Change StoreChange `json:"change,omitempty"`
    // Participant is the data store the name resolves to (a `data-stores:`
    // entry, rulings 9 and 11); nil when it does not resolve.
    Participant *Participant `json:"participant,omitempty"`
}
```
- `Action.Store *StoreRef \`json:"store,omitempty"\`` is set on an async action whose ref is a store ref.
- `Trigger.Store *StoreRef \`json:"store,omitempty"\`` is set on a `listens` trigger whose ref is a store
  ref.
- `Event` and `Ref` keep the full text as before. The existing consumers and the publish/listen pairing
  are unchanged.
- `Trigger.Schedule` / `ScheduleKind` (v2.22) are now also filled for a `listens` trigger with a trailing
  schedule. `Description` becomes `when smsdispatch listens "store.sms_queue.inserted" cron "* 10 * * 1-5"`,
  that is, the v2.22 listen form plus the schedule.

### 7.3 Sema / lint
- `craft/sema/invalid-schedule` also checks a listens tail, with the same rules and range as in v2.22.
- `craft/lint/store-without-change` (new, warning): a store ref with no recognised change. The message
  names the three changes.
- `craft/lint/store-without-name` (new, warning): a ref that starts with `store.` but names no store
  (`store.inserted`, a bare `store.`). It is not a store ref, so it is drawn as a plain event; the
  message says so and shows the `store.<name>.<change>` form.
- `dead-event` and `event-not-past-tense` apply to store refs as to any event. The changes are past
  participles, so the second lint is quiet.
- The context-map lints (`LintCommunication`) still see a `Listen` from writer to reader. Two contexts
  that share a store are a real dependency, and C4 draws it as one (§7.5): a store hand-off is never
  routed through the event queue, declared store or not.

### 7.4 Interaction semantics
- `Interaction` gains `Via Endpoint`: the store for a store hand-off, zero otherwise. The store endpoint
  is `{Key: <data store ID>, Kind: data_store, Service: <owner>}` when the name resolves. Otherwise it is
  `{Key: "store." + name, Name: name, Kind: data_store}`. The views therefore always know the endpoint is
  a store.
- A `notifies store.…` is still a `Publish`, now with `Via` set. Each listener still gets one `Listen`
  per writer (or one from `External` when nobody writes), with `Via` set.
- `Interaction` gains `Schedule string` on a `Listen` of a scheduled listens trigger, in the trigger-label
  form `cron * 10 * * 1-5` / `every 5m`.

### 7.5 Rendering
**Sequence.**
- New node kind `NodeStore` for data-store endpoints.
  - PlantUML: `database "<label>" as <alias>`.
  - Mermaid: `participant <alias> as <label> (store)`. Mermaid's portable sequence syntax has no database
    shape, so the writer marks the shape in text, as it already marks found messages with `External`.
- A store `Publish` draws `writer -> store : <change>` (`MessageCall`; the label is `writes` when there
  is no change). This replaces the self-arrow `notifies "store.…"`.
- A store `Listen` draws the read once per listening scenario: `reader -> store : polls <change>`, then
  ` (from <writers>)` by the v2.22 publisher-label rule (sorted, capped at 3, `+N more`), then
  `, <schedule>` when the trigger is scheduled. A store read is always drawn this way, even when a writer
  is on stage, because the store lane itself shows the hand-off.
- A scheduled listen that is **not** a store ref keeps the v2.22 inbound arrow, with `, <schedule>`
  appended to its label.

Example:
```craft
actors {
    system CRON
    system Siren
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
```
PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

boundary "CRON" as CRON
participant "invoicing" as invoicing
database "sms_queue" as sms_queue
participant "smsdispatch" as smsdispatch
boundary "Siren" as Siren

== Invoicing queues a payment reminder ==

CRON -> invoicing : cron 0 9 * * 1-5
invoicing -> invoicing : finds the invoices due in three days
invoicing -> sms_queue : inserted

== The queued reminder is sent ==

smsdispatch -> sms_queue : polls inserted (from invoicing), cron * 10 * * 1-5
smsdispatch -> smsdispatch : claims the unsent rows
smsdispatch -> Siren : to send the SMS [sendSmsNotificationEvent]

@enduml
```
Mermaid:
```
sequenceDiagram
    actor CRON as CRON
    participant invoicing as invoicing
    participant sms_queue as sms_queue (store)
    participant smsdispatch as smsdispatch
    actor Siren as Siren
    Note over CRON,Siren: Invoicing queues a payment reminder
    CRON->>invoicing: cron 0 9 * * 1-5
    invoicing->>invoicing: finds the invoices due in three days
    invoicing->>sms_queue: inserted
    Note over CRON,Siren: The queued reminder is sent
    smsdispatch->>sms_queue: polls inserted (from invoicing), cron * 10 * * 1-5
    smsdispatch->>smsdispatch: claims the unsent rows
    smsdispatch->>Siren: to send the SMS [sendSmsNotificationEvent]
```
- **Flow:** a store node (PlantUML `database`, Mermaid flowchart cylinder `[(sms_queue)]`). The edges are
  writer → store (`StyleAsync`, label = change) and store → reader (`StyleTrigger`, label
  `polls <change>`). These replace the writer's event queue node for store refs.
- **Architecture:** the same store node and the same unlabeled edges. A resolved store sits inside its
  owning service's group, and an unresolved one is ungrouped.
- **C4:** a store hand-off never goes through `Event_Queue`. When the store resolves to a declared
  data store and `--show-databases` is set, a `Rel(writer, db, "writes <change>")` and a
  `Rel(reader, db, "polls")`, through the existing `Database Query` path. Otherwise (an undeclared
  store, or databases hidden) a `Rel(writer, reader, "<store> <change>", "Shared store")` per writer
  and reader in different containers.

### 7.6 Formatter, LSP
- **Formatter:** the listens tail is written with single spaces, `listens <ref> cron "<expr>"`.
- **Hover** on a store ref: "store hand-off: `sms_queue`, change `inserted`; written by …; polled by …"
  (from the derived interactions).
- **Definition** on a resolved store name goes to its `data-stores:` entry.
- **Completion:** after a `listens <ref> ` offer `cron "…"` and `every "…"`. After `store.` offer the
  store names already used in the model.

## 8. Item E: surfaces (G5)

### 8.1 Syntax
```craft
surface concierge {
    callers: Advisor
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair plus a bidder identity header"
    finding "a read-only credential is refused only at persistence, after the write ran"
    query the seeker list, unpaged, newest first, filtered by a typed status  [getUserList]
    query the seeker audit trail                                             [getUserLogs]
    command delete a seeker's account                                        [deleteAccount]
}
```
```
surface_decl = 'surface' name_slot '{' NL* (surface_item NL+)* '}' ;
surface_item = 'callers' ':' name_list          (* actors, contexts, services, or `unknown` *)
             | 'spec' ':' STRING
             | note_stmt
             | ('query' | 'command') connector_word? phrase op_annotation ;
```
- A surface line has the same shape as an action line without a subject: the subject is the surface's
  boundary. The operation bracket is required, because it is the identity of the entry. Its phrase says
  what the operation does or answers, together with any behaviour it adds (paging, typed filters, unusual
  status codes; R3a).
- `query` = no side effect (Tier A), `command` = changes state (Tier B). This is the one mark the
  obido pilot asked for in its need 1.
- `callers:` covers need 4. `unknown` means no caller was observed, which replaces `NoObservedCaller` on
  surfaces and the "the human calls the backend directly" R2 exception. A surface can list
  `unknown` next to named callers.
- `spec:` is stored verbatim (need 5). Craft does not read the spec file; drift checks are left to the
  hub, which holds the repos (§12).
- Notes and findings hold facts that apply to the whole surface (need 3).
- Order is source order and carries no meaning.

**Why a new block and not an `exposure` field.** The obido pilot proposed extending `exposure`. An
exposure is many-to-many (`to:` actors × `contexts:` × `through:` gateways), but an operation belongs to
exactly one boundary. Putting operations inside an exposure would need a per-operation context qualifier
and would mix "who may reach it" with "what it offers". The new block is keyed by the boundary.
`exposure` keeps the reachability role, and §15 v2.25 lists the cross-check between the two.

### 8.2 Diagnostics
| Code | Severity | When |
|---|---|---|
| `craft/syntax/missing-operation` (new) | error | A `query`/`command` line has no trailing `[op]`. The line is dropped from the model. |
| `craft/syntax/missing-phrase` (new) | error | A `query`/`command` line has its `[op]` but nothing before it that says what the operation offers (`query [getUserList]`). |
| `craft/syntax/unexpected-token` | error | Any other line in a surface body. Recovery: skip to the next line. |
| `craft/syntax/unclosed-block` | error | The surface has no `}`. A use case or a domain without its `}` also stops at a declaration that starts at column 1 (`surface <name> {` or a top-level keyword), so it does not swallow the next surface; an indented `surface asks …` stays an action. |
| `craft/sema/unresolved-reference` | error | The surface subject, or a `callers:` entry other than `unknown`, does not resolve. |
| `craft/sema/invalid-surface-subject` (new) | error | The subject resolves to a domain or a data store. A surface belongs to a bounded context, a service or an actor (an external system). |
| `craft/sema/duplicate-name` | error in one file, warning across files | A second `surface` for the same resolved subject. The first one in sorted file order wins, as for services (ruling 6). |
| `craft/sema/duplicate-operation` (new) | error | The same operation text appears twice in one surface (compared with whitespace collapsed). |
| `craft/lint/operation-not-in-surface` (new) | warning | An `asks <target> … [op]` whose target is covered by a surface (§8.4) that does not list `op`. This is the op-name validation of need 6. It would have caught the removed `addMortgagesMeeting` route. Implemented next to `LintCommunication`, which already receives the `Request` interactions. |

### 8.3 Model
```go
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

type SurfaceOperation struct {
    Kind      SurfaceOpKind `json:"kind"`      // "query" | "command"
    Connector string        `json:"connector,omitempty"`
    Phrase    string        `json:"phrase"`
    Operation Operation     `json:"operation"` // same parsing as an action's bracket
    Line      int           `json:"line,omitempty"`
}
```
`CraftDoc` gains `Surfaces []Surface \`json:"surfaces,omitempty"\``. It is merged across files like
`Services`, and resolved by `analysis.resolveParticipants`.

### 8.4 Coverage and output
A surface is never a sequence or a spine member. Need 7 asks for a table. **Coverage is computed** (need
2): an entry is "used by" every use case that has a `Request` interaction whose `To` is the surface's
resolved participant and whose operation text equals the entry's (whitespace collapsed). A `receives`
trigger on that boundary with the same operation counts as `<use case> (unobserved caller)`.

**A surface on a service covers its contexts.** The corpus writes the surface on the deployable
(`surface obidozoe`) and the `asks` on its contexts (`advisorapp asks concierge …`). A request to a
bounded context that has no surface of its own is therefore matched against the surface of the service
that deploys it (`contexts:`). A context with its own surface is matched only against that one. The same
rule decides which surface `operation-not-in-surface` and the LSP (hover, definition, completion) use.

**A surface is the full inventory of a boundary's operations.** It lists every operation, whether or not
a journey uses it, and the residue is visible as entries that no use case uses. R3a's rule "an operation
used by a journey is never repeated in the surface" is retired. There is no completeness flag: a surface
does not claim to be complete, and Craft does not check that it is.

`craft generate --type surface` writes `<base>-surface.md` (Markdown with any `--format`, because it is
a table and not a diagram). `--stdout` works as for the other types. Overwriting an existing `.md` needs
`--force`, as for `mermaid-md`. `--type all` includes `surface` only when the model declares at least one
surface, so existing `all` runs produce the same files. `--use-case` and `--split` do not apply. For the
§8.1 example:
```markdown
# Surfaces

## concierge

Callers: Advisor
Spec: `obidozoe/docs/openapi.yaml`

> every route takes the shared HTTP Basic pair plus a bidder identity header

> **Finding:** a read-only credential is refused only at persistence, after the write ran

| Operation | Kind | What it offers | Used by |
|---|---|---|---|
| `getUserList` | query | the seeker list, unpaged, newest first, filtered by a typed status | Advisor works a task |
| `getUserLogs` | query | the seeker audit trail | — |
| `deleteAccount` | command | delete a seeker's account | Seeker removes their concierge account (unobserved caller) |
```
Surfaces are ordered by subject (ignoring case), and rows by source order. "Used by" lists use-case names, sorted and
deduplicated. `|` in text is escaped as `\|`. The renderer lives in `internal/visualizer/view/surface.go`
(view data) plus `internal/visualizer/markdown/surface.go` (writer), with
`visualizer.ViewSurface` / `visualizer.FormatMarkdown`. The flow, architecture, sequence and C4 views
ignore surfaces.

### 8.5 Formatter, LSP
- **Formatter:** `surface <name> {` on one line, a 4-space body, `}` at column 1, and one blank line
  between top-level declarations. Properties stay where the author put them (the formatter never
  reorders). Brackets of consecutive operation lines align through the existing annotation aligner
  (`formatalign.go`, minimum gap 2), under the configured scope.
- **Outline:** `surface concierge` (kind `Interface`) with one child per operation (kind `Method`, name
  `getUserList`, detail `query`). Folding range over the block.
- **Hover:** on the subject, `surface of concierge: 3 operations (2 query, 1 command); callers: Advisor`.
  On the bracket of an `asks <target> … [op]` whose target has a surface, the entry's kind and phrase.
- **Definition:** from such an `asks` bracket to the surface line.
- **Completion:** inside `[` after `asks <target> …`, the target's surface operations. A top-level
  `surface` snippet. `query`/`command`/`callers:`/`spec:`/`note`/`finding` inside a surface body.
- **Capabilities** are advertised only together with their handlers (`docs/AGENT.md` §3).

## 9. Item F: participant alternatives (G1): no grammar

**Options weighed** (seeker-advert-detail pilot report, GOOD-JOURNEY §2 "two clients" case):
1. Subject alternation `frontendplatform | mobile asks graphql …`. This turns `Action.Context`/`From`
   into a set, which every consumer (interaction, four views, hub, lints) would have to handle. It solves
   8 step lists.
2. A use-case `clients:` list that the trigger expands. It duplicates the `channels` tag that the hub
   already reads, and it would need a hidden rule for which steps the expansion rewrites.
3. **Existing constructs (chosen).** One canonical client plus `tags { channels: web, mobile }` when the
   clients make the same request. An opening `alt "on the web"` / `else "in the app"` when the first hop
   really differs. v2.22 already draws the trigger into each branch, which closes the pilot report's
   remaining complaint ("the `mobile` branch has no incoming arrow").

The corpus confirms option 3. The baseline was 28 scenarios in 16 files; after the tidy wave only 13
step lists in 11 files still have web and mobile asking the same target, and 5 of those use different
operations (genuinely different requests). 15 use cases already declare `channels: web, mobile`.

**Lint.** `craft/lint/repeated-request` (new, warning): two `asks` in the same step list (the same
scenario or branch body, at any distance) with different subjects, the same resolved target, and the same
non-empty operation text. The range is the second one. Message: "`mobile` and `frontendplatform` make the
same request `[autocomplete]` to `graphql`; if they are alternatives, write it once with a `channels` tag,
or open the scenario with `alt "on the web"` / `else "in the app"`". Today's corpus has 8 hits in 7 files.

## 10. Consumers and cross-cutting work

### 10.1 `pkg/craft`
New aliases: `Note`, `NoteKind`, `Surface`, `SurfaceOperation`, `SurfaceOpKind`, `StoreRef`,
`StoreChange`. New constants: `BlockPar`, `BlockLoop`, `TriggerTypeReceive`, `NoteKindNote`,
`NoteKindFinding`, `SurfaceQuery`, `SurfaceCommand`, `StoreInserted`, `StoreUpdated`, `StoreDeleted`,
`UnknownCaller`. The package doc's stability note gains one line: "Step is exactly one of Action, Block or
Note (Note since v2.23.0); a consumer that assumes Action-or-Block must handle a Note". Every addition is
an optional JSON key. `.craftjson` goldens of files that do not use the new constructs stay byte-identical.

### 10.2 tree-sitter-craft (`~/projects/github/tcarcao/tree-sitter-craft`)
Highlighting-level grammar. As in v2.21, it does not model nesting (`GRAMMAR.md` §13).
- `branch_header`: `choice('alt', 'opt', 'else', 'par', 'and', 'loop')`. `_branch_keyword` gains `par`,
  `and`, `loop`, `note`, `finding`, so they stay valid subjects via the `slug` alias.
- `note_stmt: seq(choice('note', 'finding'), $.string)`, accepted in the `when_clause` repeat,
  in `use_case_block` after `tags_block`, and in `surface_block`.
- `receive_trigger: seq($.listener_subject, 'receives', optional($.connector_word), optional($.phrase),
  optional($.operation_annotation))`, added to `when_clause`'s choice.
- `domain_listener` gains `optional($.cron_trigger)` after the event.
- `surface_block` in `source_file`: `'surface'`, `$.ref`, `'{'`, then lines of `surface_property`
  (`callers`/`spec`), `note_stmt` or `surface_operation` (`choice('query','command')`, `$.phrase`,
  `$.operation_annotation`), then `'}'`.
- `highlights.scm`: the new keywords as `@craft.flow-keyword`, and the note text / surface phrase as
  `@craft.usecase-string`.
- **Risk:** `and` is the most common phrase word in the corpus. `test/corpus/branches.txt` gains cases
  where `and`, `loop`, `note` and `query` appear inside phrases and as subjects. They must parse with no
  `ERROR` node, because the VSCode extension turns every `ERROR` into a diagnostic.
- `CORPUS_VERSION` → `v2.23.0`. Regenerate the wasm/dylib and publish `0.1.11`. The parser-diff harness
  keeps recording block structure as a known divergence.

### 10.3 VSCode extension (`~/projects/github/tcarcao/craft-vscode-extension`)
The extension highlights, diagnoses and completes through tree-sitter
(`client/src/TreeSitterHighlightProvider.ts`, `server/src/TreeSitterDiagnosticProvider.ts`,
`server/src/parser/TreeSitterCompletionProvider.ts`). Diagrams come from the craft server.
- Bump `tree-sitter-craft` to `^0.1.11` (`copy-tree-sitter` picks up the wasm and queries).
- `TreeSitterCompletionProvider`: snippets for `par`, `and`, `loop`, `note`, `finding`, `receives`, the
  listens `cron`/`every` tail, and `surface`.
- Ship it in the same window as v2.23.0. Before the bump, the new syntax shows as false syntax errors.

### 10.4 Hub (`svc-re-go-arch-context`)
`internal/infrastructure/graphstore/craft_loader.go`. Bump `github.com/tcarcao/craft/v2` from v2.21.0 to
v2.23.0 (this also brings the v2.22 schedule fields).
- **Branches:** `branchLabels` already walks every `Block` kind, so `par`/`loop` labels flow into
  `BranchLabel` unchanged. A split `par` gives every branch the header label (§4.3).
- **Findings:** a new walk collects `Step.Note` with `Kind == "finding"`, together with
  `UseCase.Notes` findings, into `useCase.Findings []finding{Text, Line, Scenario, BranchLabel}`. Plain
  notes are not stored.
- **`receives`:** `journeyStepFromTrigger` gets the case `TriggerTypeReceive` →
  `&step{Actor: t.Context, Verb: verbReceives, RefRaw: t.Context + " [" + op + "]", BCRef: t.Context,
  Name: op}` (new `verbReceives refVerb = "receives"`). It resolves the operation against the callee's
  own manifest, as an `asks` does against its target. The `NoObservedCaller` mention in
  `hubresourcehdl/definitions.go` is removed once the corpus migrates.
- **Store:** `applyEventRef` checks `a.Store`/`t.Store` before the dotted-ref rule. A store hand-off is
  marked `Store: <name>` and is **not** resolved against manifests (no manifest declares a table). The
  writer and the reader pair by identical ref, so the 2.22 "missing manifest" noise for `store.` refs
  goes away.
- **Surfaces:** `loadSurfaces(doc.Surfaces)` → a surface table per boundary. `operationName` is reused
  for the operation identity. `Scope == "surface"` use cases are still read until the corpus migrates,
  then the case is deleted.

### 10.5 Documentation in craft
- `docs/GRAMMAR.md`: §2 top-level gains `surface_decl`; §8 gains `par`/`and`/`loop`, `note_stmt`, the
  `receives` trigger and the listens schedule; §11.1 gains the §3.1 table.
- `docs/DIAGNOSTICS.md`: every new or widened code from §§4–9.
- `CONTEXT.md` glossary: **Surface**, **Finding**, **Note**, **Store hand-off**, **Unobserved caller**.
- `README.md`: a short section per construct.

## 11. Testing, release, migration

### 11.1 Tests (red-green-refactor, `docs/AGENT.md` §1)
- **Corpus** (`testdata/corpus/`, with `.craftjson` goldens): `04_use_cases/par_fanout`, `par_and`,
  `loop`, `nested_par_loop_alt`, `notes_findings`, `receives_trigger`, `store_handoff`,
  `listens_schedule`; new folder `09_surfaces/surface_basic`, `surface_notes_callers_unknown`. Existing
  goldens must not change.
- **Broken** (`testdata/broken/`, hand-authored `.diagnostics.json`): `orphan_and`, `else_under_par`,
  `par_missing_label`, `note_missing_text`, `note_trailing_tokens`, `surface_missing_operation`,
  `surface_unclosed`, `surface_unknown_line`, `listens_invalid_schedule`, `surface_unresolved_subject`,
  `surface_domain_subject`, `surface_duplicate_operation`.
- **Lint** (positive and negative, `sema/lint_*_test.go`): `single-branch-par`, `store-without-change`,
  `receives-on-actor`, `operation-not-in-surface`, `repeated-request`, and `indistinguishable-scenarios`
  with two `receives` that differ only by operation.
- **Interaction** (table tests): par branch stacks and stack restoration; the opening-par trigger rule
  (same subject → one trigger; differing subject → extra in-branch trigger); an opening loop; `receives`
  seeding and the lost reply; store `Via` endpoints, resolved and unresolved; `Schedule` on Listen.
  `TestSequenceStepsMatchInteractionSteps` must keep passing with notes in the step tree.
- **Visualizer goldens:** `journey-semantics.{sequence,flow,architecture}.{puml,mmd}` built from the
  §4.1, §5.1, §6.1 and §7.5 examples, plus `surface.md`. Unit tests for `BlockOps` with `par` (`and` in
  Mermaid), a split `par` (bare separators), note anchoring (after a trigger, after a block, first in a
  scenario, use-case-level), and a branch that holds only a note.
- **Formatter:** idempotence and block-membership round-trip on every new corpus file, and refusal on
  `orphan_and`. `FuzzFormatBranches` gains seeds with `par`/`and`/`loop`/`note`.
- **Fuzz (new):** `FuzzParseUseCaseBody` in `internal/syntax`. It wraps random bodies in a use case and
  asserts that there is no panic and that `checkTreeText` stays lossless. Seeds: the four use-case
  examples above and a surface.
- **LSP:** document symbols (par/loop/findings/surface), folding, hover on each new keyword, surface
  operation completion and definition.
- **CLI:** `generate --type surface` (file, `--stdout`, `--force`), and `--type all` with and without
  surfaces.
- **Dogfood:** `craft validate` on re-arch-central stays at 0 errors before migration. The only new
  warnings are `repeated-request` (8) and none of the others. The atlas-invoice SMS file, rewritten with
  the listens schedule, and `seeker-advert-detail.craft`, rewritten with a fan-out `par`, are rendered as
  the acceptance examples.

### 11.2 Versioning and changelog
v2.23.0, module path `github.com/tcarcao/craft/v2`, minor (additive). The CHANGELOG sections are:
- **Added:** the six constructs, `generate --type surface`, the new codes, and the `pkg/craft` additions.
- **Changed:** `when X receives …` is now a callee-side trigger (it used to be an external trigger with
  verb `receives`; no known uses). Store refs draw a store participant instead of a self-arrow
  `notifies "store.…"`. `orphan-else` widens its message. Writers no longer print a trailing space after
  an empty block label.

### 11.3 Corpus migration (re-arch-central, after craft v2.23.0 and the hub bump)
1. **G13.** Rewrite the 65 `when NoObservedCaller …` triggers (36 files). Where the next step's subject
   is a context (about 31 triggers), use `when <that context> receives <phrase> [op]` and drop the line
   that only restated the call. Where the file lists `NoObservedCaller asks X … [op]` operations (69
   lines, 30 files, mostly censuses such as `obido/obido-analytics-read-api.craft`), move the operations
   to `surface X { callers: unknown … }`. Then delete `NoObservedCaller` from `docs/actors.craft`, and
   rewrite convention 11 to say "`receives` / `callers: unknown`".
2. **G5.** Turn "Concierge back-office reads" into `surface concierge { callers: Advisor … }`. Convert
   the register files listed in §1 the same way, one surface per boundary. Rewrite R3a as "Surfaces are
   `surface` blocks" and drop `surface` from the `scope` tag values.
3. **G8.** In convention 2, drop "interim" and add the listens schedule. In
   `invoicing/atlas-invoice-sms-reminders.craft`, replace the `polls` tag with
   `listens store.sms_queue.inserted cron "* 10 * * 1-5"`, and drop `polls` from the tag table. The 196
   `TERMINUS` comments and the store-writing internal steps become `notifies store.<table>.<change>` /
   `listens` pairs file by file, starting with the 65 files that have such steps. This is a follow-up
   wave, not part of the release.
4. **G12.** Comments rule: a defect, risk or negative fact that a reader of the diagram or the hub needs
   becomes `finding "…"`. An internal step that only states such a fact (for example "holds no
   contract…") becomes a finding. Open questions and `NOT TRACED` stay comments.
5. **G9/G18.** R5 gains "concurrent requests → `par`; repetition, retries and polling loops → `loop`".
   Pilot: the native-branch batch calls in `ad/seeker-advert-detail.craft`.
6. **G1.** Work off the 8 `repeated-request` warnings with `channels` or an opening `alt`.
7. DOMAIN-MAP header: "Requires craft ≥ 2.23.0".

## 12. Deliberately left out
- **An event-side unobserved marker (`when <ctx> receives <event>`).** A `listens` whose event has no
  publisher in the model is already drawn as a found message (v2.22). Whether nobody publishes it, or
  the publisher is only outside the loaded files, is a question for the hub, which has the manifests. A
  craft lint for unpaired listens would flood partial models.
- **Spec drift checks for `surface spec:`.** Craft never reads repositories. The hub does this.
- **A database shape in Mermaid sequence diagrams.** It needs `participant X@{type: database}` (Mermaid
  ≥ 11.3), which GitHub/GitLab renderers do not reliably support. Revisit when they do.
- **Severity or category on findings** (`finding security "…"`). One kind until the hub shows a need.
- **Multi-line note text.** STRING does not cross lines, and several notes do the job.
- **A G1 grammar** (§9).
- **Defects D1 (verb dropped from asks labels) and D7.** They are separate fixes and not part of this
  design.

## 13. Decisions
1. `par`/`and` and `loop` as new kinds of the existing block machinery (header, quoted label, offside
   rule), with no braces and no `end`. Consistent with v2.21.
2. The fan-out form of `par` (no `and` → each body step is a branch). Every corpus case is a fan-out.
   Writing a labelled `and` per request would double the lines and repeat each step's text. The explicit
   `and` form stays for multi-step branches. The model always holds one Branch per concurrent path
   (`Block.Split` records the form).
3. An opening `par` draws one trigger, plus in-branch triggers only where a branch starts elsewhere. An
   opening `loop` draws one trigger before the box. Alt and opt keep the v2.22 rules.
4. `note`/`finding` are step statements (`Step.Note`). They are drawn over the previous message's
   participants, they are not actions, and findings are exported for the hub.
5. `when <ctx> receives <phrase> [op]` with a found/lost message and an `Unobserved` endpoint separate
   from `External`. There is no event-side form.
6. `store.` is a recognised transport and needs no new action grammar. The store participant is a
   database lane, and the read is drawn reader → store. The listens schedule tail replaces the `polls`
   tag.
7. A top-level `surface` block (not an `exposure` extension) with `query`/`command` lines, computed
   coverage, Markdown output and an op-name lint.
8. G1: no grammar. `channels` or an opening `alt`, plus the `repeated-request` lint.
9. v2.23.0 is minor and additive. The only meaning change is `receives`, which has zero known uses.

## 14. Implementation order
Each step is one TDD slice that is green between commits (`docs/AGENT.md` §4): (1) notes/findings,
(2) `par`/`and`/`loop`, (3) `receives`, (4) listens schedule + store hand-offs, (5) surfaces + `generate
--type surface`, (6) `repeated-request`, (7) docs, (8) tree-sitter, (9) VSCode, (10) hub. Slices 8–10
are in sibling repos and are released in the same window.

## 15. Later releases (sketches, with a recommended design for each)

### v2.24: trigger and action expressiveness
- **G10 multi-event triggers.** `when X listens a.A, b.B, c.C` (a comma list in the existing ref slot,
  continuing across lines after a comma as tag lists do). `Trigger.Events []string` holds the whole list.
  `Event`/`Ref` stay the first item, for compatibility. `Derive` emits one `Listen` per event per
  publisher. The sequence view draws one inbound arrow labelled `a.A, b.B (+N more)`. The hub expands to
  N pairings. Migration: `partner/mercury-transaction-ledger-ingest` (43 listens blocks).
- **G11 parametrisation: tag-only (decided direction).** The `sites` tag (already read by the hub) plus
  `alt` labels for site behaviour that really differs. Add a lint `craft/lint/duplicate-scenario-body`
  (two scenarios with identical steps whose triggers differ only by one word) that suggests the tag. No
  `for sites …` grammar: a multiplier on the diagram hides the per-site differences that matter.
- **G15 actor kinds.** `actor_type` gains `tool` (CLI, script, Terraform transformer), `job` (pod start,
  scheduler other than CRON) and `device` (mail client, browser redirect). PlantUML: `tool` → `control`,
  `job` → `entity`, `device` → `participant` with a stereotype. `lint/unused-actor` and the views treat
  them like `system`.
- **G16 streaming.** `X returns stream <phrase>` (contextual `stream` after `returns`) →
  `Action.Stream bool`, drawn as a `loop "stream"` box around a reply arrow (`-->>` in Mermaid), so no
  new arrow syntax is needed. The hub verb is `streams`.
- **G17 compound and dynamic operations.** Several brackets per action (`[POST /v1/x] [clientMethod]`)
  → `Action.Operations []Operation`, with `Operation` as the first one. Dynamic events reuse the G10
  list: `notifies one of nmfbus.A, nmfbus.B` → `Action.Events`, with one Publish per event.
- **G20 confidence and egress.** `ADDR` and `SQL` become recognised op verbs (`[ADDR https://…]`,
  `[SQL table]`). `Operation.Verb` is already open, so this only adds constants and hover. `inferred`
  as an op-bracket prefix (`[inferred getX]`) → `Operation.Inferred bool`, drawn as a dashed arrow. It
  lives inside the bracket grammar, so no new line form is needed. A lint flags an internal step whose
  bracket is `ADDR` (it should be an `asks` to a declared actor).

### v2.25: structure across use cases
- **G4 structured provenance.** A trailing structured comment `//@ <repo> <path>:<line>[-<line>]
  [verified <date>]`, attached by the parser to the step above it as `Action.Sources []Source{Repo,
  Path, Lines, Verified}`. It stays comment-shaped, so files keep reading naturally and older craft
  versions ignore it. The hub checks freshness. The formatter keeps it under its step.
- **G6 cross-use-case links.** A step `continues "<use case title>"` (or `continues "<file>::<title>"`
  when the title is ambiguous) → `Step.Link`. Sema resolves it, with `craft/sema/unresolved-use-case`
  as an error. It is drawn as PlantUML `ref over <subject> : <title>` and as a Mermaid note. The hub
  draws an edge. Also an `exposure`↔`surface` cross-check: a surface caller that no exposure lets reach
  the boundary is a warning.
- **G7 spine view.** `craft generate --journey <slug>`: one sequence over every use case whose `journey`
  tag equals the slug, ordered topologically by publish → listen (and store write → read) pairing, with
  ties broken by file and line. Each use case is a `group` frame. A hand-off whose publisher and listener
  are both in the spine is drawn as a direct arrow, not an inbound one.
