# Journey Semantics 2: rosters, scope, initiators, streams, operation lists, confidence, provenance, links, spines — Design Spec

**Date:** 2026-10-14
**Owner:** Tiago Carção (design decisions delegated; each one is recorded with its reason in §20)
**Status:** Part I implemented (v2.24.0)
**Releases:** two minor releases, both additive. No `pkg/craft` name, field or JSON key changes or
disappears.
- **v2.24.0** (Part I): G10, G11, G15, G16, G17, G20. Three inputs that parsed before change meaning
  (`returns stream …`, `[inferred …]`, `[ADDR …]`/`[SQL …]`, §3.1). The corpus, `testdata/` and
  `examples/` use none of them.
- **v2.25.0** (Part II): G4, G6, G7. One input that parsed before changes meaning: a comment line that
  starts with `//@` (zero uses).

**Builds on:** `2026-10-14-journey-semantics-design.md` (v2.23.0: `par`/`loop`, `note`/`finding`,
`receives`, store hand-offs, surfaces, `repeated-request`). Its §15 "Later releases" sketched these
items. This spec replaces those sketches. Where it departs from a sketch, §20 says so and why.
**Drives:** re-arch-central journey assessment, gaps G4, G6, G7, G10, G11, G15, G16, G17 and G20
(`docs/journey-assessment/GOOD-JOURNEY.md` §4 there), and the per-domain reports in the same folder.

## 1. Problem and corpus evidence

These counts come from re-arch-central `docs/journeys/` (370 `.craft` files, 636 use cases), measured on
2026-10-14 after the restructure wave. They are greps, so they are close but not exact; each
implementation slice records the exact baseline from `craft validate` when it lands.

| Gap | What the corpus does today | Count |
|---|---|---|
| **G10** multi-event triggers | One handler serves N events, and the file writes N scenarios. `partner/mercury-transaction-ledger-ingest.craft` has a use case "seshat ledger subscriptions" whose comment says "One scenario per subscribed topic so each producer's notifies pairs; all run the ladder above. Craft has no roster form yet". `platform-ops/event-bus-atlas-delivery.craft` listens to a placeholder `eventbus.Event` and lists the real twelve records in a `records:` tag, so none of them pairs. `billing/nmf-monetization-events-become-warehouse-facts.craft` writes 3 of 22 detail types and lists the rest in a comment ("craft has no multi-event trigger yet"). Two `// OR-TRIGGER:` comments (`vas/ad-lifecycle-vas-cascade.craft:12`: "AdExpired, AdDeactivated, AdRemoved and AdDeleted run one handler"). | 22 use cases where one subject listens to 3 or more events (135 triggers). The ledger roster: 44 scenarios + 1 head. 6 use cases hold 17 scenarios whose bodies are identical. 1 `records:` tag (12 records). |
| **G11** parametrisation | The `sites`/`envs` tags carry scope, but no diagram shows them. Per-site chains are written as named scenarios and really differ: in `analytics/ad-statistics-daily-build.craft` only the otodompl flow "assembles real phone-call activity from the investment tables". The tag values drift: `imovirtualpt` (5 uses) next to `imovirtualpt2` (47); one file lists `imomock, crap`. | 114 `sites:` lines in 90 files, 12 `envs:` lines. 8 distinct site values. 152 `alt`/`opt`/`else` labels name a site, market or environment. 4 site-named scenarios in 2 files. |
| **G15** non-human initiators | `docs/actors.craft` declares 63 actors as `user`, `system` or `service`. Seven `system` actors say in a comment that they are something else: `CRON`, `Ticker` (clocks), `Platform`, `Pipeline`, `SyntheticMonitor` (platform machinery), `OperatorTool`, `ContractConsumer` (tools). The seo report (G9) says "`Platform starts a pod` is not CRON, not Ticker; no actor for 'process start'". | Triggers: `Ticker` 8, `Platform` 5, `Pipeline` 2, `SyntheticMonitor` 1, `ContractConsumer` 1, `CRON` 3 (+104 `when cron`/`when every`). `OperatorTool` is the subject of 30 step lines. |
| **G16** streaming | `seller-tooling/ai-assistant-chat.craft:39-43`: `frontendplatform asks graphql to stream the answer [assistantStream]`, then `graphql relays each upstream event to the client as META, TOKEN or DONE` as an internal step. The seller-tooling report (N2): "`asks` has one response; token streaming … cannot be shown". | 2 files, 4 `asks … to stream`, 2 relay steps. |
| **G17** operation lists, dynamic events | One hop with several operations is already written as a comma list in one bracket: `Partner asks ad/atlas-advert to save or delete the advert [postAdvert, putAdvert, deleteAdvert]`, `[files.list, files.get]`, `[CreateApiKey, CreateUsagePlanKey]`. The hub reads each as one operation named `postAdvert, putAdvert, deleteAdvert`, which resolves to nothing. `billing/nmf-user-event-outbox.craft` reads the detail type "off the stored column rather than naming an event", so it has 0 `notifies` (billing report N3). | 6 step lines with a comma list in a bracket. 1 outbox file with no publish. |
| **G20** confidence and egress | `obido/obido-market-ingestion.craft:12`: `obidoanalytics asks front for the city, region, developer and project reference graph`, no bracket, with the comment "Raw SQL over the monolith's own `investment`, `flat`, `city`, `region` and `developer` tables". `partner/mercury-lead-message-handoff.craft:32`: "Probably whatever serves SiteHooksApi; NOT TRACED". `ad/seller-resumes-an-agentic-selling-draft.craft:50`: "… is inferred from its name, not shown in code". | 20 step `asks` with no bracket in 13 files: 2 are direct table reads, 4 are egress to external parties. 2 inferred hops. |
| **G4** provenance | Evidence is a comment under the step: `// eb_email_sender.go:5`, `// pkg/siren/siren.go:63-115: a 423 for a suppressed address counts as delivered`. The hub cannot see it, so it cannot tell when the cited code moved. | 3,247 comment lines. 1,753 hold a `file:line` cite, 1,588 of them indented under a step. 2,328 cite tokens; 860 carry a directory; 91 name the repo. 906–947 comment lines start with a cite (797 in the first count). 2 `VERIFIED` stamps remain. 349 of 370 files carry cites. |
| **G6** cross-use-case links | A non-event hand-off is a comment: `// Follows web/site-session-preamble.craft (PHP session and active features)`, `// The payment runs through the web view the app opens (vas/seller-promote-an-advert.craft, …)`. | 549 comment lines name another `.craft` file (390 in headers, 159 in bodies). 17 `Follows`, 17 `Hands off to`, 9 `preamble`. 0 duplicate use-case titles across files. |
| **G7** spine view | The hub groups use cases by `journey` tag. Craft cannot draw a spine. | 343 journey slugs; 126 have 2+ use cases; 38 span 2+ files. The largest, `seller-promotes-an-advert`, has 18 use cases in 9 files. 142 publish→listen pairings sit inside one journey (64 journeys), 286 cross journeys. About 69 multi-member journeys have no pairing inside them. |

## 2. Decisions at a glance

| # | Item | Decision |
|---|---|---|
| A | G10 | A comma list in the `listens` ref slot: `when X listens a.A, b.B, c.C`. The list continues after a trailing comma. `Trigger.Events` holds the list; `Event`/`Ref` stay the first item. One `Listen` per event per publisher; the sequence view draws one inbound arrow per trigger. The hub expands to N steps. |
| B | G11 | **Tag-only.** No grammar. The sequence and spine views print `sites`/`envs` in the section title. New lint `duplicate-scenario-body` (shared with G10). The hub gets `site`/`env` filters and a check of tag values against a configured list. |
| C | G15 | Three recognised actor kinds: `scheduler`, `tool`, `infra`, next to `user`/`system`/`service`. The actor-type slot is already open (grammar ruling Q5), so there is **no grammar change**: only vocabulary, rendering (a stereotype), C4, LSP and hub. |
| D | G16 | `X returns stream [to Y] <phrase>`: a reply that repeats. `Action.Stream`. Drawn as a reply arrow inside a `loop stream` box. The hub marks the request the stream answers. |
| E | G17 | A comma list inside one bracket is a list of operations of one hop: `[CreateApiKey, CreateUsagePlanKey]` → `Action.Operations`. `Operation` is unchanged. A dynamic publish is `notifies one of a.A, b.B` → `Action.Events`, one `Publish` per event. |
| F | G20 | `ADDR` and `SQL` become recognised operation verbs (`[ADDR "https://…"]`, `[SQL investment flat]`). `inferred` as the first word of a bracket marks the hop as inferred (`Action.Inferred`), drawn grey with an `(inferred)` suffix. New lints `egress-on-internal-step` and `unquoted-address`. |
| G | G4 | A structured comment `//@ [<repo>] <path>[:<lines>] [, …] [-- <text>]` under a step attaches to it as `Sources`. It replaces today's cite comments one for one, so files do not grow. Freshness comes from git history on both sides, checked by the hub; craft stores no date. |
| H | G6 | Two link steps: `includes "<use case title>"` (a shared sub-flow runs here) and `continues "<use case title>"` (the flow goes on there). `Step.Link`. Drawn as a UML `ref` frame. Event hand-offs need no link: pairing already covers them. |
| I | G7 | `craft generate --journey <slug>`: one sequence over every use case whose `journey` tag is the slug, ordered by publish→listen, store write→read and `continues` edges, ties broken by file and line. Publishers drawn earlier deliver directly. `pkg/craft.BuildJourney` gives the order to the hub. |

# Part I — v2.24.0

## 3. Shared mechanics (v2.24)

### 3.1 Contextual words
All new words are contextual, as in v2.23 §3.1:

| Word | Is a keyword only when | Otherwise |
|---|---|---|
| `,` in a `listens` slot | directly after an event item of a `listens` trigger | (was an error) |
| `one`, `of` | `one` directly after `notifies`, followed by `of` and an event item | an identifier / a connector word |
| `stream` | directly after `returns` | a phrase word |
| `inferred` | the first word inside an operation bracket | a payload word |
| `ADDR`, `SQL` | the first word of a bracket item (protocol verbs are already matched this way, exact and case-sensitive) | a payload word |
| `scheduler`, `tool`, `infra` | never keywords: the actor-type slot accepts any identifier (Q5) | an identifier |

Three of these change the meaning of input that parsed before. In every case there are zero uses in
re-arch-central, `testdata/`, `examples/` and the sibling repos (grep of 2026-10-14):
- `X returns stream …` used to be a return whose phrase starts with "stream".
- `[inferred …]` used to be an operation whose payload starts with "inferred".
- `[ADDR …]` and `[SQL …]` used to be operations with an empty `Verb` and the word in `Payload`.

The CHANGELOG lists them under **Changed** (§11.2). The `listens` list and `notifies one of` were syntax
errors before.

### 3.2 Syntax kinds
`internal/syntax/kind.go` keeps its numbering. New token kinds are appended before
`syntaxKindTokenSentinel`: `KwOne`, `KwOf`, `KwStream`, `KwInferred`. New node kind, appended at the end:
`SyntaxKindEventList`. It wraps the items of a `listens` or `notifies one of` list, and it exists only
when the list has a comma, so a single-ref trigger keeps its v2.23 tree. Operation items (§8.1),
`ADDR`/`SQL` (§9.1) and actor kinds (§6.1) add no node kinds.

### 3.3 Vocabulary (`internal/lang`)
```go
const (
    ActorScheduler = "scheduler" // a clock that starts work: cron, an in-process ticker
    ActorTool      = "tool"      // a CLI, script or client a person runs; it, not the person, calls
    ActorInfra     = "infra"     // platform machinery: pod start, probes, CI pipelines, monitors
)
// KnownActorTypes is user, system, service, scheduler, tool, infra, in that order.
func KnownActorTypes() []string

const (
    OpVerbAddr = "ADDR" // egress to an address that has no operation id
    OpVerbSQL  = "SQL"  // a direct read or write of another party's tables
)
// protocolVerbs gains ADDR and SQL.

const OpInferred = "inferred" // the bracket-head marker of an inferred hop

const (
    TagSites = "sites" // recognised tag keys: the views print them (§5)
    TagEnvs  = "envs"
)
```
`model.ActorType` gains `ActorTypeScheduler`, `ActorTypeTool`, `ActorTypeInfra`. `pkg/craft` re-exports
everything, as for the v2.23 constants.

### 3.4 IDs
No new IDs. A file that uses none of the new constructs gets the same IDs and the same `.craftjson` as
in v2.23.0.

## 4. Item A: multi-event triggers (G10)

### 4.1 Syntax
```craft
use_case "seshat records a Mercury domain event in the transaction ledger" {
    when seshat listens sns.mercury-advert-state-changed, sns.mercury-vas-state-changed,
            sns.mercury-message-received, sns.mercury-cooked
        seshat writes one row per event
}
```
```
domain_listen = 'when' name_slot 'listens' event_list listen_schedule? NL ;
event_list    = event_item (',' NL* event_item)* ;
event_item    = STRING | ref ;
```
Rules:
1. The list means "any of these events starts this scenario". Each delivery is one event; the body is
   the same for all of them. When the body differs by event, write named scenarios (R5), as today.
2. A trailing comma continues the list on the next line, as tag lists do. A continuation line is never
   a step: the parser consumes it inside the trigger, so the offside rule does not see it. Comments
   between continuation lines are trivia.
3. The v2.23 listens schedule (`cron "…"` / `every "…"`) goes after the last item.
4. Store refs (v2.23 §7) may not appear in a list of two or more items (§4.3). A store hand-off is one
   table; a poller of two tables is two scenarios.

### 4.2 Model
```go
// Events holds every event of a listens list, in source order, with the same
// text Event would hold for each. Nil for a single event: read Event.
Events []string `json:"events,omitempty"`

// EventList returns Events, or a one-element slice holding Event.
func (t Trigger) EventList() []string
```
`Event` and `Ref` stay the first item, so a consumer that reads only `Event` sees a valid listen. `Store`
stays nil for a list (rule 4). `Description` lists every item:
`when seshat listens "sns.mercury-advert-state-changed", "sns.mercury-vas-state-changed", …`. Two
scenarios with the same list are therefore indistinguishable, and two with different lists are not.

### 4.3 Sema / lint
| Code | Severity | When |
|---|---|---|
| `craft/sema/store-ref-in-event-list` (new) | error | A `store.` ref in a list of 2+ items. "A store hand-off names one store; write one scenario per store." |
| `craft/lint/duplicate-event-in-list` (new) | warning | The same item twice in one list (exact text). Range: the second one. |
| `craft/lint/dead-event`, `craft/lint/event-not-past-tense` | warning | Unchanged rules, now applied to each item; the range is the item. |
| `craft/lint/duplicate-scenario-body` (new, §5.2) | warning | Scenarios of one use case with identical bodies, reported once per group at its first scenario. When the triggers are `listens` by the same subject, with no store ref and the same schedule, the message says "fold them into one trigger: `when <subject> listens <a>, <b>, <c>`" (every item of the group once); otherwise it names what differs (§5.2). |
| `craft/syntax/unexpected-token` | error | A comma with no item after it (the list ends at a comma followed by a step line or `}`). |

### 4.4 Interaction semantics
- `Derive` collects publishers per event, as before. For each item of `EventList()`, a listener gets one
  `Listen` per publisher of that item, or one from `External` when nobody publishes it. Every one of them
  has `Step` 0 and carries the trigger's `Schedule`.
- `Interaction` is unchanged. The views group the `Listen`s of one trigger by `(UseCaseIndex, Scenario,
  Step 0)`.

### 4.5 Rendering
**Sequence.**
- A publisher already on stage in the section delivers its events itself, as in v2.22: one
  `publisher ->> listener` arrow per on-stage publisher, labelled with the items it publishes (list
  order, capped at 3 with `+N more`).
- Every other item arrives on **one** inbound arrow per trigger. Its label is the remaining items in
  list order, capped at 3 with `(+N more)`, then ` (from <publishers>)` by the v2.22 rule over the
  publishers of every item on that arrow (sorted, capped at 3), then `, <schedule>` when scheduled.
- `inboundKey` becomes `useCase|scenario|step` for a trigger with `Events`; a single-event trigger
  keeps today's `useCase|scenario|event` key, so existing goldens do not change.

Example, with a second use case that publishes one of the events:
```craft
actors {
    user Partner
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
```
PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Partner" as Partner
participant "cooker" as cooker
participant "seshat" as seshat

== Mercury cooks an advert ==

Partner -> cooker : posts an advert
cooker -> cooker : cooks the advert for its target site
cooker ->> cooker : sns.mercury-cooked

== seshat records a Mercury domain event in the transaction ledger ==

[-> seshat : sns.mercury-advert-state-changed, sns.mercury-vas-state-changed, sns.mercury-message-received (+1 more) (from cooker)
seshat -> seshat : writes one row per event

@enduml
```
Mermaid:
```
sequenceDiagram
    actor Partner as Partner
    participant cooker as cooker
    participant seshat as seshat
    participant External as External
    Note over Partner,seshat: Mercury cooks an advert
    Partner->>cooker: posts an advert
    cooker->>cooker: cooks the advert for its target site
    cooker-)cooker: sns.mercury-cooked
    Note over Partner,seshat: seshat records a Mercury domain event in the transaction ledger
    External->>seshat: sns.mercury-advert-state-changed, sns.mercury-vas-state-changed, sns.mercury-message-received (+1 more) (from cooker)
    seshat->>seshat: writes one row per event
```
**Flow and architecture.** One queue node and edge per item, exactly as N separate triggers draw
today. The roster is the truth, and the flow view's job is to show every lane.
**C4.** Unchanged: each `Listen` goes through `Event_Queue` as today.

### 4.6 Formatter, LSP
- **Formatter:** `, ` between items on one line. The author's line breaks are kept. A continuation line
  is indented 8 columns past the trigger's `when` (4 past the body column), so it never lines up with a
  step. The formatter never reorders items.
- **Hover** on the subject or `listens`: "listens to 4 events; publishers: cooker (1 of 4 events)".
  Hover on an item: that item's publishers.
- **Completion:** after `listens <item>, ` offer the event refs already published in the model.
- **Definition** on an item: its publishers' `notifies` lines (as for a single ref today).
- **Semantic tokens:** each item as today's ref; the comma as `operator`.

### 4.7 Tests
Corpus `04_use_cases/listens_list`, `listens_list_multiline`, `listens_list_schedule`; broken
`listens_list_trailing_comma`, `listens_list_store_ref`; lint positive/negative for
`duplicate-event-in-list`; interaction table tests (N items × M publishers, items with no publisher);
sequence goldens with on-stage and off-stage publishers and with more than 3 items; formatter idempotence
on multi-line lists; `FuzzParseUseCaseBody` seeds with lists.

## 5. Item B: parametrisation over sites and environments (G11): tag-only

### 5.1 Decision
**No grammar.** A `for sites otodompl, storiaro` multiplier, or a site parameter on a step, would draw
one chain and hide what differs per site. The corpus shows those differences are the point: the
three per-site flows of `analytics/ad-statistics-daily-build.craft` differ in which bases they load. The
existing tools cover the cases:
- the whole use case runs only on some sites or environments → `sites:` / `envs:` tag (R11);
- a step runs only on some → `opt "…"`; the steps really differ → `alt "…"` or named scenarios.

The gap was that a tag is invisible in the diagram and that nothing checks its values. This item closes
both.

### 5.2 Sema / lint
`craft/lint/duplicate-scenario-body` (new, warning). Scenarios of one use case whose step trees are
equal: the same shape, every action equal on `Type`, `Context`, `Verb`, `TargetContext`, `Event`,
`Connector`, `Phrase` and `Operation.Text`, every block equal on kind and labels. Notes and links are
ignored. The body must hold at least one action. The scenarios are grouped by body, and each group of
two or more is reported **once, at the group head**: the first scenario with that body, whose trigger a
fold rewrites. The message starts "this scenario and the scenario(s) at line(s) <the other members> run
the same steps". Within the group, `listens` triggers by the same subject, with no store ref and the
same schedule, form one fold set; every other trigger is a set of its own.
- One set holds the whole group → the G10 message (§4.3) with every item of the group once, the shared
  schedule kept: "fold them into one trigger: `when s listens a.A, b.B, c.C cron "…"`".
- Some sets hold two or more triggers, but not the whole group → one "fold the scenarios at lines <…>
  into one trigger: `…`" per such set, then what tells the sets apart (as below, "the rest start …,
  so one trigger cannot replace them all").
- No set holds two triggers, and the triggers differ in initiator, schedule, `receives` operation or a
  store ref → "… run the same steps but start <what differs>, so one trigger cannot replace them;
  check that the steps really are the same, and if they differ, write the difference as an `alt`".
  What differs is named between the head's set and the first other set that differs from it.
- Otherwise → "… run the same steps; if they differ only by site, market or environment, write one
  scenario with a `sites`/`envs` tag; if the steps really differ, write the difference as an `alt`".

Today's corpus: 8 warnings: 6 `listens` fold groups in 5 use cases (17 scenarios; they fold into G10
lists), one pair of initiators running one body (`nmf-billing-cycle-rollover:11`, with line 19) and
one pair of `receives` operations (`revise-an-active-poa-adjustment:11`, with line 31). A pairwise
lint gave 13 warnings for the same 17 scenarios.

Craft does not validate tag **values**. The site and environment codes are deployment configuration
that craft cannot see; the hub can (§5.5).

### 5.3 Rendering
The sequence view (and the spine view, §17) appends the scope to the section title:
`<use case> [sites: <values>] [envs: <values>]`. Each part appears only when its tag is present; values
are `TagValues` joined with `, `, in authored order. `SequenceSection.Title` is built in the view, so both
writers print it unchanged.

```craft
actors {
    system CRON
}
use_case "Rebuild a market's daily statistics" {
    tags {
        sites: imovirtualpt, otodompl, storiaro
        envs: production
    }
    when cron "0 1/2 * * *"
        adstatistics rebuilds the day's advert facts
}
```
PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

boundary "CRON" as CRON
participant "adstatistics" as adstatistics

== Rebuild a market's daily statistics [sites: imovirtualpt, otodompl, storiaro] [envs: production] ==

CRON -> adstatistics : cron 0 1/2 * * *
adstatistics -> adstatistics : rebuilds the day's advert facts

@enduml
```
Mermaid:
```
sequenceDiagram
    actor CRON as CRON
    participant adstatistics as adstatistics
    Note over CRON,adstatistics: Rebuild a market's daily statistics [sites: imovirtualpt, otodompl, storiaro] [envs: production]
    CRON->>adstatistics: cron 0 1/2 * * *
    adstatistics->>adstatistics: rebuilds the day's advert facts
```
Flow, architecture and C4 have no use-case titles and do not change.

### 5.4 Formatter, LSP
- **Formatter:** no change.
- **Hover** on a `sites`/`envs` key: "scope tag: printed in the sequence title; the hub filters and
  checks its values".
- **Completion:** inside `tags { }`, offer `sites:` and `envs:`; after `sites: `, the values already used
  in the model, most frequent first. This makes `imovirtualpt2` the first suggestion and stops new
  drift.

### 5.5 Hub
- `arch_find_use_cases` gains two optional arguments: `site` ("only use cases whose `sites` tag
  includes this site; use cases with no `sites` tag run on every site and are included") and `env`
  (same for `envs`). The descriptions follow the AGENTS.md rules (more than 15 words, say when to use
  them).
- A validation issue `tag-unknown-value` for a `sites` or `envs` value that is not in the hub's
  configured lists (`KNOWN_SITES`, `KNOWN_ENVS`, comma-separated environment variables). Today's corpus
  would report `imovirtualpt` (5 files), `imovirtualcom`, `imomock` and `crap` (1 file) for review. When
  the lists are unset, the check is off.
- The web UI already shows `sites`/`envs` chips (`web/src/views/journey.ts`). No change.

### 5.6 Tests
Lint positive/negative for `duplicate-scenario-body` (identical bodies, bodies that differ by one phrase,
notes-only difference, listens vs external wording). Sequence goldens with `sites` only, `envs` only,
both, and quoted values. LSP completion of site values.

## 6. Item C: actor kinds (G15)

### 6.1 Syntax: no change
`actor_type` is already any identifier (grammar ruling Q5: "Tooling decides the known set"), so
`actors { scheduler Ticker }` parses in v2.23. This item adds three kinds to the known set.

| Kind | Meaning | Corpus actors |
|---|---|---|
| `scheduler` | a clock that starts work | `CRON`, `Ticker` |
| `tool` | a CLI, script or client that a person runs; it makes the calls | `OperatorTool`, `ContractConsumer` |
| `infra` | platform machinery: pod start, probes, CI pipelines, synthetic monitors | `Platform`, `Pipeline`, `SyntheticMonitor` |

All three are **outside the system**, like `system`: `interaction.isInsideSystem` is false for them, so a
trigger from one draws an arrow, and `receives-on-actor` (v2.23) treats them as actors.

### 6.2 Model
`Actor.Type` holds the kind as written, as today. New constants (§3.3). `Participant.Kind` stays
`actor`.

### 6.3 Sema / lint
No new codes. `craft/lint/unused-actor` treats the three kinds like `system`. An actor type outside the six
known kinds stays legal and gets no diagnostic (the open taxonomy is a ruling). Hover says it is drawn
as a person (§6.5).

### 6.4 Interaction semantics
No change. The kind is a property of the participant, read by the views.

### 6.5 Rendering
| Kind | PlantUML element | Mermaid sequence | Mermaid flow | C4 |
|---|---|---|---|---|
| `user` (and any unknown kind) | `actor` | `actor` | `([…])` | `Person`, "External user" |
| `system` | `boundary` | `actor` | `([…])` | `System_Ext`, "External system" |
| `service` | `control` | `actor` | `([…])` | `System_Ext`, "External service" |
| `scheduler` | `boundary … <<scheduler>>` | `participant … (scheduler)` | `([…])` | `System_Ext`, "Scheduler" |
| `tool` | `boundary … <<tool>>` | `participant … (tool)` | `([…])` | `System_Ext`, "Operator tool" |
| `infra` | `boundary … <<infra>>` | `participant … (infra)` | `([…])` | `System_Ext`, "Platform infrastructure" |

The first three rows are today's behaviour and do not change. The new kinds keep the `boundary` shape
because they sit outside the system, and the stereotype says which kind. In Mermaid they are drawn as
a box, not a person, because the point of the item is that they are not people. Mermaid has no
stereotypes, so the kind is marked in text, as v2.23 marks a store `(store)`.

```craft
actors {
    scheduler Ticker
    tool OperatorTool
    user Operator
}
use_case "The activation sweep starts due schedules" {
    when Ticker fires the vasscheduler activation sweep
        vasscheduler starts every schedule that is due
}
use_case "Operator exports agency portfolio values" {
    when Operator runs the portfolio export
        OperatorTool asks account for the agency owner members  [backofficeMembersList]
}
```
PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

boundary "Ticker" as Ticker <<scheduler>>
participant "vasscheduler" as vasscheduler
actor "Operator" as Operator
boundary "OperatorTool" as OperatorTool <<tool>>
participant "account" as account

== The activation sweep starts due schedules ==

Ticker -> vasscheduler : fires the vasscheduler activation sweep
vasscheduler -> vasscheduler : starts every schedule that is due

== Operator exports agency portfolio values ==

Operator -> OperatorTool : runs the portfolio export
OperatorTool -> account : for the agency owner members [backofficeMembersList]

@enduml
```
Mermaid:
```
sequenceDiagram
    participant Ticker as Ticker (scheduler)
    participant vasscheduler as vasscheduler
    actor Operator as Operator
    participant OperatorTool as OperatorTool (tool)
    participant account as account
    Note over Ticker,account: The activation sweep starts due schedules
    Ticker->>vasscheduler: fires the vasscheduler activation sweep
    vasscheduler->>vasscheduler: starts every schedule that is due
    Note over Ticker,account: Operator exports agency portfolio values
    Operator->>OperatorTool: runs the portfolio export
    OperatorTool->>account: for the agency owner members [backofficeMembersList]
```
The flow and architecture PlantUML writers use the same `actorElement` mapping, so they gain the
stereotype too. C4 PlantUML: `System_Ext(Ticker, "Ticker", "Scheduler")`.

### 6.6 Formatter, LSP
- **Formatter:** no change.
- **Completion:** at the start of an `actors { }` line and after `actor `, the six known kinds, each with
  a one-line description.
- **Hover** on a kind: its meaning from §6.1; on an unknown kind, "not a recognised kind; drawn as a
  person".
- **Semantic tokens:** unchanged (`craft-actor-type` already covers identifier kinds).

### 6.7 Tests
Visualizer goldens for the example (PlantUML, Mermaid, C4); `unused-actor` with each new kind; LSP
completion and hover. The existing `robot` fixture keeps its output.

## 7. Item D: streaming responses (G16)

### 7.1 Syntax
```craft
actors {
    user Seller
    system AiAssistant
}
use_case "Seller asks the assistant a question" {
    when Seller asks the assistant a question
        frontendplatform asks graphql to stream the answer  [assistantStream]
        graphql asks AiAssistant to stream the reply  [POST /api/v1/assistant/chat_streaming]
        AiAssistant returns stream the reply as META, TOKEN and DONE events
        graphql returns stream each event to the client
        frontendplatform appends every TOKEN delta to the message
}
```
```
return_action = name_slot 'returns' 'stream'? ('to' name_slot)? connector_word? phrase? op_annotation? NL ;
```
`stream` is a keyword only directly after `returns`. A stream is a reply that repeats until it ends: the
request stays an ordinary `asks`, and the answer is a `returns stream`.

### 7.2 Model
`Action.Stream bool \`json:"stream,omitempty"\`` on a return action.
`Action.Answers string \`json:"answers,omitempty"\``, set only when `Stream` is true: the ID of the
request action the stream answers (the call-stack frame `interaction.answer` pops), or "" when it answers
the trigger's actor, the unobserved caller or nobody. It is filled by a pass in `analysis` that reuses
the interaction call-stack walk (exported inside the module as `interaction.AnsweredRequests`). It is
limited to streams so that no existing return gains a JSON key. `Description`:
`AiAssistant returns stream the reply as META, TOKEN and DONE events`.

### 7.3 Sema / lint
`craft/lint/stream-without-caller` (new, warning): a `returns stream` that answers `External` (no caller
on the stack, and no `to` target). "A stream answers the request that opened it; write the `asks` that
opens the stream before it, or name the reader with `returns stream to <reader>`."

### 7.4 Interaction semantics
A `Response` as today (same call-stack rules), with a new field `Stream bool`. The label is
`streams <phrase>` instead of `returns <phrase>`.

### 7.5 Rendering
`view.Message` gains `Stream bool`. A writer wraps a stream message in its own one-message box
`loop stream` … `end`. The box opens and closes around that message only, so it does not interact with
the branch-path box operations (`BlockOps`), and it nests inside any block.

PlantUML:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
boundary "AiAssistant" as AiAssistant

== Seller asks the assistant a question ==

Seller -> frontendplatform : asks the assistant a question
frontendplatform -> graphql : to stream the answer [assistantStream]
graphql -> AiAssistant : to stream the reply [POST /api/v1/assistant/chat_streaming]
loop stream
AiAssistant --> graphql : streams the reply as META, TOKEN and DONE events
end
loop stream
graphql --> frontendplatform : streams each event to the client
end
frontendplatform -> frontendplatform : appends every TOKEN delta to the message

@enduml
```
Mermaid:
```
sequenceDiagram
    actor Seller as Seller
    participant frontendplatform as frontendplatform
    participant graphql as graphql
    actor AiAssistant as AiAssistant
    Note over Seller,AiAssistant: Seller asks the assistant a question
    Seller->>frontendplatform: asks the assistant a question
    frontendplatform->>graphql: to stream the answer [assistantStream]
    graphql->>AiAssistant: to stream the reply [POST /api/v1/assistant/chat_streaming]
    loop stream
    AiAssistant-->>graphql: streams the reply as META, TOKEN and DONE events
    end
    loop stream
    graphql-->>frontendplatform: streams each event to the client
    end
    frontendplatform->>frontendplatform: appends every TOKEN delta to the message
```
A stream to the unobserved caller (a `receives` scenario) is a lost message inside the box.
- **Flow:** the reply edge label is `streams …` (the `StyleReply` edge is unchanged).
- **Architecture, C4:** as for any return (C4 draws only targeted returns, with technology
  `Return Data (stream)` for a stream).

### 7.6 Formatter, LSP
- **Formatter:** no change (`returns stream` is two words on the action line).
- **Hover** on `stream`: "streamed reply: drawn in a `loop stream` box; answers <request>".
- **Completion:** after `returns `, offer `stream`.
- **Semantic tokens:** `stream` as `keyword`.

### 7.7 Hub
Return actions are still not steps. A step built from an `asks` whose action ID is the `Answers` of a
stream gets `Streaming: true`, and the use-case API and `arch_explain_use_case` say "streamed reply".
The operation it resolves to is then known to stream.

### 7.8 Tests
Corpus `04_use_cases/returns_stream`, `returns_stream_to`; `Answers` filled from the stack and with a
`to` target; lint positive/negative; sequence goldens (plain, inside `alt`, to an unobserved caller);
Mermaid render test accepts the box.

## 8. Item E: operation lists and dynamic publishes (G17)

### 8.1 Operation lists: syntax
The corpus already writes one hop that uses several operations as a comma list inside one bracket.
Craft now reads it that way:
```craft
        app-manager asks AwsApiGateway to create the key and bind it to the plan  [CreateApiKey, CreateUsagePlanKey]
        Partner asks ad/atlas-advert to save or delete the advert  [postAdvert, putAdvert, deleteAdvert]
```
```
op_annotation = '[' 'inferred'? (op_item (', ' op_item)*)? ']' ;   (* §9 adds 'inferred' *)
op_item       = op_verb? payload ;
```
- The bracket is found as before: the last `[` on the line, and the line ends with `]`. Nothing changes
  in how the annotation is located, so the tree-sitter scanner keeps its rule.
- The body splits into items at a comma that is followed by whitespace, at brace depth 0, outside a
  STRING. `[GET /nearest/paths/{latitude},{longitude}]` stays one item (no space after the comma).
- Each item is parsed like a whole bracket today: an optional leading protocol verb, then the payload.
- The items are alternatives or a sequence; craft does not say which. Write the difference in the
  phrase ("to save or delete", "to create the key and bind it"). Two hops to two parties are two
  `asks`, as R6 already says.
- A route and the client method that calls it are **not** two operations: one is the wire contract and
  the other is code. Write the route in the bracket and cite the method with `//@` (v2.25, §14).

### 8.2 Operation lists: model
```go
// Operations holds each item of a bracket that lists two or more, in order.
// Nil for a single-item bracket: read Operation.
Operations []Operation `json:"operations,omitempty"`

// OperationList returns Operations, or a one-element slice holding *Operation,
// or nil when the action has no bracket.
func (a Action) OperationList() []Operation
```
`Action.Operation` keeps today's value (the whole body: `Text` verbatim, `Verb` from the first word),
byte-identical, so no existing `.craftjson` changes. `Interaction` gains `Operations []model.Operation`,
copied.

### 8.3 Dynamic publish: syntax and model
```craft
use_case "nMF republishes a stored money record onto its bus" {
    when nmf_monetization listens stream.user_event_store
        nmf_monetization reads the detail-type off the stored column
        nmf_monetization notifies one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled,
                nmfbus.CreateAutoSchedule, nmfbus.SubAccountBundleUpdated
}
```
```
async_action = name_slot 'notifies' ( event_item | 'one' 'of' event_list ) op_annotation? NL ;
```
- `notifies one of …` means "publishes exactly one of these events per run, chosen at run time" (an
  event type read from data, a routing table). The list is the declared set the corpus asked for.
- A bare list without `one of` is `craft/syntax/unexpected-token` with the message "to publish one of
  several events write `notifies one of a, b`; to publish several, write one `notifies` per event".
- `Action.Events []string \`json:"events,omitempty"\`` and `Action.EventList()`, as for triggers (§4.2).
  `Event`/`Ref` stay the first item. `Description`: `nmf_monetization notifies one of "nmfbus.FeatureEnabled", …`.
- Store refs are refused in the list (`store-ref-in-event-list`, §4.3).

### 8.4 Sema / lint
| Code | Severity | When |
|---|---|---|
| `craft/syntax/empty-op-annotation` (widened) | error | An empty item: `[a, , b]` or a trailing `, ` before `]`. |
| `craft/lint/one-of-single-event` (new) | warning | `notifies one of` with one item. "Drop `one of`." |
| `craft/lint/duplicate-event-in-list` | warning | Also for `notifies one of`. |
| `craft/lint/operation-not-in-surface` (v2.23) | warning | Now checks each item; one warning per item that the target's surface does not list. |
| `craft/lint/duplicate-operation-in-list` (new) | warning | The same item twice in one operation bracket (exact text): `[opA, opA, opB]`. Range: the repeat. |
| `craft/lint/repeated-request` (v2.23) | warning | Unchanged: it compares the whole `Operation.Text`. |
| `dead-event`, `event-not-past-tense` | warning | Per item of a `one of` list. |

### 8.5 Interaction semantics
`notifies one of` emits one `Publish` per item, all with the same `Step`. Each pairs with its listeners
as today. A listener of any of them therefore sees this publisher.

### 8.6 Rendering
- **Operation lists:** no change. The label already prints the whole bracket text.
- **Dynamic publish, sequence:** the view merges the `Publish` interactions of one step into one message
  labelled `one of <items, capped at 3> (+N more)`. In the outbox example above:
  - PlantUML: `nmf_monetization ->> nmf_monetization : one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled, nmfbus.CreateAutoSchedule (+1 more)`
  - Mermaid: `nmf_monetization-)nmf_monetization: one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled, nmfbus.CreateAutoSchedule (+1 more)`

  The whole example, PlantUML:
  ```
  @startuml
  skinparam backgroundColor white
  skinparam defaultFontName SansSerif
  skinparam sequenceMessageAlign center

  participant "nmf_monetization" as nmf_monetization

  == nMF republishes a stored money record onto its bus ==

  [-> nmf_monetization : stream.user_event_store
  nmf_monetization -> nmf_monetization : reads the detail-type off the stored column
  nmf_monetization ->> nmf_monetization : one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled, nmfbus.CreateAutoSchedule (+1 more)

  @enduml
  ```
  Mermaid:
  ```
  sequenceDiagram
      participant nmf_monetization as nmf_monetization
      participant External as External
      Note over nmf_monetization: nMF republishes a stored money record onto its bus
      External->>nmf_monetization: stream.user_event_store
      nmf_monetization->>nmf_monetization: reads the detail-type off the stored column
      nmf_monetization-)nmf_monetization: one of nmfbus.FeatureEnabled, nmfbus.FeatureDisabled, nmfbus.CreateAutoSchedule (+1 more)
  ```
- **Flow:** one queue node and edge per item, each labelled `one of: <event>`.
- **C4:** unchanged (each `Publish` reaches `Event_Queue`).

### 8.7 Formatter, LSP
- **Formatter:** `, ` between bracket items, and the annotation aligner treats the whole bracket as one
  unit. `notifies one of` lists follow §4.6 (continuation 8 columns past the step's subject).
- **Hover** on a bracket: one line per item ("2 operations: CreateApiKey, CreateUsagePlanKey"); on
  a surface-backed target, each item's surface entry (v2.23 §8.5).
- **Completion:** inside `[` after `asks <target> …, `, the target's remaining surface operations.
  After `notifies `, offer `one of`.
- **Definition** from each item to its surface line.

### 8.8 Hub
- `journeyStepFromAction` returns one `asks` step per item of `OperationList()`, all with the action's
  line. Each item resolves on its own, so `[postAdvert, putAdvert, deleteAdvert]` resolves three
  operations instead of failing on one combined name.
- `notifies one of` returns one `notifies` step per item; each pairs with its listeners.
- `AuthoredActions` still counts the action once.

### 8.9 Tests
Corpus `04_use_cases/operation_list`, `operation_list_path_comma` (`{a},{b}` stays one item),
`notifies_one_of`; broken `operation_list_empty_item`, `notifies_bare_list`; lint tests; interaction
(one Publish per item); sequence golden for the merged publish; `operation-not-in-surface` per item.

## 9. Item F: confidence and egress markers (G20)

### 9.1 Syntax
```craft
actors {
    system CRON
    system ObjectStore
}
use_case "obido analytics refreshes the market structure" {
    when cron "0 18 * * *"
        obidoanalytics asks front for the city, region and developer graph  [SQL investment flat city region developer]
        obidoanalytics asks ObjectStore to take the refreshed snapshot  [ADDR "SNAPSHOT_DESTINATION"]
}
```
```craft
actors {
    user Seller
    system ShaperAgent
}
use_case "Seller resumes a selling draft" {
    when Seller resumes a selling draft
        mobile asks graphql to open a selling chat session  [agenticSellingStart]
        graphql asks ShaperAgent to initiate the chat  [inferred POST /api/v1/assistant/initiate_chat]
}
```
- **`SQL <table> …`**: a direct read or write of another party's tables, with no API between. The payload
  is the table names, separated by spaces (`Operation.Tables()` splits it). It is a protocol verb, so
  it also works as one item of a list.
- **`ADDR <address>`**: egress to an address that has no operation id. The address is a URL, a host, or
  the name of the setting that holds it. **It must be quoted when it contains `//`**, because `//` starts
  a comment (`[ADDR "https://…"]`); a bare host such as `[ADDR api.example.com]` needs no quotes.
  `Operation.Payload` holds the address without quotes.
- **`inferred`**: the first word inside the bracket marks the hop as inferred from names, configuration
  or convention, not read in code. `[inferred]` alone is allowed: the hop is inferred and its operation
  unknown. It works on every action kind that takes a bracket. It applies to the whole action, not to
  one item.

### 9.2 Model
- `Operation.Verb` is `ADDR` or `SQL`, as for any protocol verb. `func (o Operation) Tables() []string`
  returns the payload fields for a `SQL` operation and nil otherwise.
- `Action.Inferred bool \`json:"inferred,omitempty"\``. The `inferred` word is not part of
  `Operation.Text`. A bracket that holds only `inferred` leaves `Action.Operation` nil.
- `inferred` has no meaning in a `receives` trigger or a surface line. There it is an ordinary payload
  word, and the LSP hover says so.
- `Interaction` gains `Inferred bool`, copied from the action.

### 9.3 Sema / lint
| Code | Severity | When |
|---|---|---|
| `craft/lint/egress-on-internal-step` (new) | warning | An internal or return action whose bracket has an `ADDR` or `SQL` item. "`[ADDR …]`/`[SQL …]` name a call to another party; write `<subject> asks <target> … [ADDR …]`, declaring the target as a `system` actor if it is outside the estate." This is the L1 convention check, for the two verbs where it is certain. |
| `craft/lint/unquoted-address` (new) | warning | An action phrase that ends in `[ADDR` followed by text, on a line that ends in a `//` comment. The address was cut by the comment, so the bracket was not recognised. "Quote the address: `[ADDR \"https://…\"]`." |
| `craft/syntax/empty-op-annotation` (widened) | error | `[ADDR]` or `[SQL]` with no payload: these two verbs mean nothing without an address or a table. The other verbs keep today's behaviour. |

### 9.4 Interaction semantics
No change for `ADDR`/`SQL` (a `Request` with its operation). `Inferred` is copied onto every interaction
the action produces.

### 9.5 Rendering
- **Sequence:** an inferred message is drawn grey with the label suffix ` (inferred)`. PlantUML uses
  the coloured arrow `-[#999999]>` (`-[#999999]->` for a reply); Mermaid has no per-arrow colour and uses
  the suffix only. An `ADDR` label prints the address without quotes. `SQL` labels print as written.

PlantUML, first example:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

boundary "CRON" as CRON
participant "obidoanalytics" as obidoanalytics
participant "front" as front
boundary "ObjectStore" as ObjectStore

== obido analytics refreshes the market structure ==

CRON -> obidoanalytics : cron 0 18 * * *
obidoanalytics -> front : for the city, region and developer graph [SQL investment flat city region developer]
obidoanalytics -> ObjectStore : to take the refreshed snapshot [ADDR SNAPSHOT_DESTINATION]

@enduml
```
PlantUML, second example:
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "mobile" as mobile
participant "graphql" as graphql
boundary "ShaperAgent" as ShaperAgent

== Seller resumes a selling draft ==

Seller -> mobile : resumes a selling draft
mobile -> graphql : to open a selling chat session [agenticSellingStart]
graphql -[#999999]> ShaperAgent : to initiate the chat [POST /api/v1/assistant/initiate_chat] (inferred)

@enduml
```
Mermaid, second example:
```
sequenceDiagram
    actor Seller as Seller
    participant mobile as mobile
    participant graphql as graphql
    actor ShaperAgent as ShaperAgent
    Note over Seller,ShaperAgent: Seller resumes a selling draft
    Seller->>mobile: resumes a selling draft
    mobile->>graphql: to open a selling chat session [agenticSellingStart]
    graphql->>ShaperAgent: to initiate the chat [POST /api/v1/assistant/initiate_chat] (inferred)
```
- **Flow:** the edge label gains ` (inferred)`; PlantUML flow edges use `-[#999999]->>`.
- **Architecture:** no change.
- **C4:** a request with a `SQL` item uses technology `SQL (shared tables)`; with an `ADDR` item,
  `HTTP (bare address)`; otherwise `Service API` as today. An inferred relation's label gains
  ` (inferred)`.

### 9.6 Formatter, LSP
- **Formatter:** no change; `[inferred …]` keeps one space after the word.
- **Hover** on `ADDR`: "egress to an address with no operation id; the hub does not resolve it"; on
  `SQL`: "direct use of another party's tables: <tables>"; on `inferred`: "inferred hop: drawn grey".
- **Completion:** at the start of a bracket, `inferred`, then the protocol verbs (now including `ADDR`
  and `SQL`).
- **Semantic tokens:** `inferred` as `modifier`; `ADDR`/`SQL` like the other verbs.

### 9.7 Hub
- `operationName` skips resolution for `ADDR` and `SQL`. An `ADDR` step records `Egress: <address>`,
  and a `SQL` step records `Tables`. Neither counts as an unresolved operation.
- A new validation issue `shared-tables` lists every `SQL` step whose target is another context, with
  the tables. These are the cross-context database couplings the obido report (G11) asked to see.
- An inferred step records `Confidence: "inferred"`. `arch_explain_use_case` prints "(inferred)" after
  it. `[inferred]` with no operation is reported as an inferred hop, not as a missing operation.

### 9.8 Tests
Corpus `04_use_cases/op_sql`, `op_addr_quoted`, `op_inferred`, `op_inferred_only`; lint tests for both
new lints (the unquoted case must reproduce the comment-swallowing behaviour); interaction `Inferred`;
sequence and C4 goldens; `Operation.Tables()`.

## 10. Consumers (v2.24)

### 10.1 `pkg/craft`
New constants: `ActorTypeScheduler`, `ActorTypeTool`, `ActorTypeInfra`, `OpVerbAddr`, `OpVerbSQL`,
`TagSites`, `TagEnvs`, and `KnownActorTypes()`. New fields: `Trigger.Events`, `Action.Events`,
`Action.Operations`, `Action.Stream`, `Action.Answers`, `Action.Inferred`. New methods:
`Trigger.EventList`, `Action.EventList`, `Action.OperationList`, `Operation.Tables`. Every addition is an
optional JSON key; `.craftjson` goldens of files that do not use the new constructs stay byte-identical.
The package doc's stability note gains: "a listens trigger or a notifies may carry several events
(`EventList`), and an action several operations (`OperationList`), since v2.24.0".

### 10.2 tree-sitter-craft
Changes are relative to the v2.23 grammar (`0.1.11`).
- `domain_listener`: the event becomes `seq($._event, repeat(seq(',', repeat($._newline), $._event)))`,
  then the v2.23 optional schedule tail. `_event: choice($.string, $.ref)`.
- `async_action`: `choice($._event, seq('one', 'of', $._event, repeat(seq(',', repeat($._newline), $._event))))`.
  `of` is already a `connector_word` literal; `one` joins `_branch_keyword` so it stays a valid subject.
- `return_action`: `optional('stream')` directly after `'returns'`; `stream` joins `_branch_keyword`.
- `operation_annotation`: `optional(field('marker', 'inferred'))` before the verb. `operation_verb` gains
  `'ADDR'` and `'SQL'`. The payload token already accepts commas, so items are not modelled (highlighting
  only). The external `[` scanner is unchanged.
- `highlights.scm`: `one`/`of` after `notifies`, `stream`, `inferred` as `@craft.flow-keyword`;
  `((actor_type) @type.builtin (#match? @type.builtin "^(user|system|service|scheduler|tool|infra)$"))`.
- **Risk:** `stream`, `one` and `inferred` appear in phrases. `test/corpus/` gains cases where they are
  subjects, verbs and phrase words, with no `ERROR` node.
- `CORPUS_VERSION` → `v2.24.0`; publish `0.1.12`.

### 10.3 VSCode extension
- Bump `tree-sitter-craft` to `^0.1.12`.
- `TreeSitterCompletionProvider`: snippets for a multi-line `listens` list, `notifies one of`,
  `returns stream`, `[inferred …]`, `[SQL …]`, `[ADDR "…"]`; the six actor kinds in an `actors` block;
  `sites:`/`envs:` keys in `tags`.
- Ship in the same window as v2.24.0.

### 10.4 Hub (`svc-re-go-arch-context`)
Bump `github.com/tcarcao/craft/v2` from v2.23.0 to v2.24.0.
- **G10:** `journeyStepFromTrigger` returns `[]*step`, one `listens` step per item of `EventList()`, each
  through `applyEventRef`; the loop in `loadJourneys` appends them all. `scenarioLabel` prints
  `seshat hears <first 3 items> (+N more)`. Each item pairs on its own, so the ledger roster yields 45
  `listens` steps from one trigger and `platform-ops/event-bus-atlas-delivery.craft` 12 (today it yields
  one step for the placeholder `eventbus.Event`, which pairs with nothing).
- **G11:** §5.5.
- **G15:** `loadActors` already carries `Type` verbatim. `archctx://actors` groups actors by kind;
  `actorDescription` names the kind ("scheduler: starts 9 use cases"). The comment above `loadActors`
  ("three different things wearing one keyword") is updated: craft now states the kind.
- **G16:** §7.7. **G17:** §8.8. **G20:** §9.7.

### 10.5 Documentation in craft
`docs/GRAMMAR.md` §3 (actor kinds as the known set), §8 (`event_list`, `notifies one of`,
`returns stream`, bracket items, `inferred`, `ADDR`/`SQL`), §11.1 (the §3.1 table); `docs/DIAGNOSTICS.md`
(every new or widened code); `CONTEXT.md` glossary: **Roster trigger**, **Operation list**,
**Streamed reply**, **Inferred hop**, **Egress**; `README.md` one section per construct.

## 11. Testing, release, migration (v2.24)

### 11.1 Tests
Per item, as listed in §§4.7, 5.6, 6.7, 7.8, 8.9, 9.8, with red-green-refactor (`docs/AGENT.md` §1).
Cross-cutting:
- **Existing goldens** (`.craftjson`, visualizer, formatter) must not change.
- **`TestSequenceStepsMatchInteractionSteps`** keeps passing with merged publishes and grouped inbound
  arrows.
- **Fuzz:** `FuzzParseUseCaseBody` seeds with every example of Part I; `FuzzFormatBranches` seeds with
  multi-line lists.
- **Dogfood:** `craft validate` on re-arch-central stays at 0 errors before migration. The only new
  warnings are `duplicate-scenario-body` (8: 6 fold groups in 5 use cases, 2 others) and `unused-actor` changes (none expected). The ledger
  file, rewritten with a list, and `seller-tooling/ai-assistant-chat.craft`, rewritten with
  `returns stream`, are rendered as the acceptance examples.

### 11.2 Versioning and changelog
v2.24.0, minor. CHANGELOG:
- **Added:** `listens` lists, `notifies one of`, `returns stream`, operation lists, `ADDR`/`SQL`,
  `inferred`, the actor kinds `scheduler`/`tool`/`infra`, scope in sequence titles, the new codes and
  the `pkg/craft` additions.
- **Changed:** `returns stream …`, `[inferred …]`, `[ADDR …]` and `[SQL …]` now have meaning (no known
  uses). Actors typed `scheduler`/`tool`/`infra` render as stereotyped boundaries instead of persons. A
  use case with `sites`/`envs` tags shows them in its sequence title.

### 11.3 Corpus migration (re-arch-central, after craft v2.24.0 and the hub bump)
1. **G10.**
   - `partner/mercury-transaction-ledger-ingest.craft`: one trigger listing all 45 topics. Each per-topic
     line of the roster that states a fact becomes a `finding`/`note` (for example "a reader who sums
     retryable and terminal rows double-counts one failure"); lines that only restate the topic name
     are dropped. Delete the "seshat ledger subscriptions" use case and its `scope: surface` tag.
   - `platform-ops/event-bus-atlas-delivery.craft`: the twelve records become the list, replacing the
     `eventbus.Event` placeholder. Drop the `records:` tag, and delete the `records` key from the
     DOMAIN-MAP tag table.
   - `billing/nmf-monetization-events-become-warehouse-facts.craft`: the 22 detail types become the
     list, and the comment that lists them goes.
   - The 5 use cases flagged with a fold by `duplicate-scenario-body` fold their scenarios into lists. The two
     `vas/ad-lifecycle-vas-cascade.craft` use cases each get a four-item list, and both `OR-TRIGGER`
     comments are removed (in `vas/seller-is-mailed-about-addon-events.craft` the eight mails differ, so
     the scenarios stay and the comment becomes a `note`).
   - The other use cases among the 22 with 3+ listens per subject are reviewed file by file.
2. **G11.** No syntax change. Settle the `imovirtualpt`/`imovirtualpt2` question and the values the hub
   flags. R11 gains: "the sequence title shows `sites`/`envs`".
3. **G15.** In `docs/actors.craft`: `scheduler CRON`, `scheduler Ticker`, `infra Platform`,
   `infra Pipeline`, `infra SyntheticMonitor`, `tool OperatorTool`, `tool ContractConsumer` (7 lines),
   with the "NOT a system" style comments shortened. Convention 21 lists the six kinds.
4. **G16.** The 2 files: the relay steps become `returns stream`, and each upstream stream gets its own
   `returns stream`.
5. **G17.** The 6 comma-list brackets already conform. `billing/nmf-user-event-outbox.craft` gets a
   `notifies one of` with the documented detail types. R6 gains: "one hop that uses several operations:
   `[a, b]` in one bracket".
6. **G20.** Review the 20 bracketless `asks`: the 2 direct table reads in
   `obido/obido-market-ingestion.craft` get `[SQL …]`; external egress gets an operation or `[ADDR …]`;
   the 2 inferred hops get `[inferred …]` and lose their "probably"/"inferred" comments.
7. DOMAIN-MAP header: "Requires craft ≥ 2.24.0".

# Part II — v2.25.0

## 12. Shared mechanics (v2.25)

### 12.1 Contextual words and lexical forms
| Form | Is special only when | Otherwise |
|---|---|---|
| `//@` | a line comment whose first three characters are `//@` | (it was a plain comment) |
| `includes`, `continues` | at the start of a step line and followed by a STRING, or alone on the line (`missing-label`) | an identifier |

`//@` changes the meaning of input that parsed before. There are zero uses in re-arch-central,
`testdata/`, `examples/` and the sibling repos. A malformed `//@` line is only a warning and stays a
comment, so no file that parsed starts to fail. The link words were a syntax error before ("a STRING
can never be a verb").

### 12.2 Syntax kinds
Token kinds appended: `KwIncludes`, `KwContinues`, and the trivia kind `SourceComment` (lexer
`TokenSourceComment`, like `TokenDocComment`). Node kind appended: `SyntaxKindLink` (a step).

### 12.3 Vocabulary
```go
type LinkKind string
const (
    LinkIncludes  LinkKind = "includes"  // the target's steps run here; this body goes on after
    LinkContinues LinkKind = "continues" // the flow goes on in the target; ends this body
)
const (
    TagJourney        = "journey"         // recognised tag keys (§17, §16)
    TagFormerlyTitled = "formerly_titled"
)
type JourneyEdgeKind string
const (
    JourneyEdgeEvent     JourneyEdgeKind = "event"
    JourneyEdgeStore     JourneyEdgeKind = "store"
    JourneyEdgeContinues JourneyEdgeKind = "continues"
)
```

### 12.4 IDs
Links get `link_<n>` from the per-file counter that scenarios, actions, blocks and notes share, in source
order. A file with no links keeps its IDs. Sources have no IDs.

## 13. Steps and links in the step tree
`Step` becomes "exactly one of Action, Block, Note or Link". Like a note (v2.23 §5.2), a link is not an
action: it gets no interaction, it does not advance `Interaction.Step`, it does not count towards the
hub's `AuthoredActions`, and `model.FirstStepIndex` skips it, so a scenario that opens with
`includes "…"` is triggered into its first action as before. A branch that holds only a link is not
an `empty-branch`. `single-branch-par` ignores links, as it ignores notes.

## 14. Item G: structured provenance (G4)

### 14.1 Syntax
```craft
use_case "nMF mails the seller about an add-on event" {
    when nmf_monetization listens nmfbus.AddOnPurchased
        //@ re-monetization-framework functions/eb_email_sender/eb_email_sender.go:5 -- one rule, eight detail-types
        nmf_monetization picks the mail resolver from the event type alone
        //@ eb_email_sender.go:5-40
        nmf_monetization asks Siren to send the purchase mail  [POST /api/v1/event]
        //@ pkg/siren/siren.go:63-115 -- a 423 for a suppressed address counts as delivered
}
```
```
source_comment = '//@' SP cite (',' SP cite)* (SP '--' SP text)? ;
cite           = (repo SP)? path (':' ranges)?
               | ':' ranges ;                       (* the same path as the cite before *)
repo           = /[a-z0-9][a-z0-9._-]*/ ;           (* no '/', no ':'; only when a path follows *)
path           = /[^\s:,()]+/ ;                     (* holds '/' or '.'; no '(' or ')', so prose like `x (a/b.go:3)` is no cite *)
ranges         = range (',' range)* ;               (* no spaces: sqs.tf:3,26 *)
range          = INT ('-' INT)? ;
```
Rules:
1. **Attachment.** A `//@` line attaches to the nearest step line above it in the same use case: an
   action, a block-branch header (`alt`/`else`/`opt`/`par`/`and`/`loop`), a note, a link, or the
   trigger. Plain comments in between do not break the attachment. A `//@` line before the first
   scenario attaches to the use case. Anywhere else (a file header, between use cases) it is detached
   (§14.3). Attachment uses line numbers only, so indentation is not significant.
2. **Repo.** Optional. When it is absent, the hub resolves the repo from the attached step's subject
   (its service's `repo:`, else the service name). Craft records only what is written; it never reads
   a repository.
3. **Text.** Everything after ` -- ` is free text for the reader. It replaces the prose that today
   follows a cite on the same comment line, so a converted line is no longer than the original.
4. **No date.** There is no `verified` stamp. Freshness is computed from git history (§14.6).

### 14.2 Model
```go
// Source is one cite of a //@ line.
type Source struct {
    Repo  string `json:"repo,omitempty"`  // as written; "" when the cite names none
    Path  string `json:"path"`
    Start int    `json:"start,omitempty"` // first cited line, 1-based; 0 when no line is cited
    End   int    `json:"end,omitempty"`   // last cited line; equal to Start for one line
    Text  string `json:"text,omitempty"`  // the text after " -- ", shared by the cites of its line
    Line  int    `json:"line,omitempty"`  // the line of the //@ comment in the .craft file
}
```
`Sources []Source \`json:"sources,omitempty"\`` on `Action`, `Trigger`, `Branch`, `Note`, `Link` and
`UseCase`. One `//@` line with three ranges of one path gives three `Source`s.

### 14.3 Sema / lint
| Code | Severity | When |
|---|---|---|
| `craft/syntax/malformed-source` (new) | warning | A `//@` line whose head does not parse as a cite, or a range whose end is before its start. The line stays an ordinary comment and adds no `Source`. |
| `craft/lint/detached-source` (new) | warning | A well-formed `//@` line with nothing to attach to. "A `//@` cite proves the step above it; move it under that step, or make it a plain `//` comment." |

### 14.4 Interaction semantics
None. Sources are evidence, not behaviour.

### 14.5 Rendering
Not drawn in any view. A diagram is for the behaviour; the hub and the LSP show evidence.

### 14.6 Formatter, LSP, hub
- **Formatter:** a `//@` line is a comment and follows the existing rule: it stays under its step, at
  that step's body column. Its content is not rewritten.
- **LSP:** hover on a step lists its sources (`repo path:start-end`, text). Hover on a `//@` line shows
  the parsed cites and the step it attaches to. Semantic tokens: the `//@` marker and the text as
  `comment`, the repo as `namespace`, the path as `string`, ranges as `number`. **Document links**
  (`textDocument/documentLink`) when the client sends `initializationOptions.sourceRoots`
  (`{"<repo>": "<dir>", "*": "<dir holding every repo>"}`); the link target is
  `<dir>/<path>#L<start>`. Without the option, no links (the capability is advertised only with its
  handler, `docs/AGENT.md` §3).
- **Hub — loading:** `step.Sources` from the action or trigger the step came from, plus
  `useCase.Sources`.
- **Hub — freshness job** (new, in `graphstore`'s validation pass, through `gitlabgw`):
  1. Resolve the repo: the written slug, else the step subject's service. Map it to a GitLab project
     through the catalog directory the hub already uses. Unresolved → status `unresolved`.
  2. Resolve the path at the default branch HEAD. A bare file name (`eb_email_sender.go`) is searched by
     base name (`ListPaths`); more than one match → `ambiguous`; none → `broken`.
  3. A cited range past the end of the file → `broken`.
  4. Compare dates: the date the `//@` line was last changed in re-arch-central (blame from the
     `gitmirror` checkout) against the newest commit touching the cited range (GitLab blame API for the
     file, or the commits API with `path` when no range is cited). Code newer than the cite → `stale`.
     Otherwise `fresh`.
  Results are cached by (project, path, HEAD SHA). `arch_explain_use_case` prints each step's evidence
  with its status, and the validation report counts `stale`, `broken`, `ambiguous` and `unresolved`
  cites per file.

### 14.7 Tree-sitter, VSCode
- tree-sitter: `source_comment: $ => token(prec(1, /\/\/@[^\n]*/))` in `extras`, ahead of `comment`;
  `highlights.scm`: `(source_comment) @comment.documentation`.
- VSCode: a `DocumentLinkProvider` over `source_comment` nodes, driven by a new setting
  `craft.sources.roots` (same shape as the LSP option); hover is not duplicated in the extension.

### 14.8 Tests
Lexer: `//@` vs `//` vs `///`. Projection: attachment to each step kind, across plain comments, at
use-case level, and detached; multiple cites, repeated ranges, `:<range>` reuse of the previous path,
text after ` -- `. Broken: malformed heads, reversed ranges. Formatter: `//@` lines keep their place
inside nested branches. `.craftjson` golden `04_use_cases/sources`. LSP hover and document links.

## 15. Item H: cross-use-case links (G6)

### 15.1 Syntax
```craft
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
```
```
link_stmt = ('includes' | 'continues') STRING NL ;
```
- A link is a step. It obeys the offside rule, so it can sit in any branch.
- **`includes "<title>"`**: the target use case runs at this point (a shared prefix such as a session
  preamble, or a shared sub-flow), then this body goes on.
- **`continues "<title>"`**: the flow goes on in the target, and this body ends. Use it for a hand-off
  that has no event: a navigation, a redirect, a web view the app opens.
- **An event hand-off needs no link.** `notifies E` / `listens E` already pair (R8), and the spine view
  (§17) orders by that pairing.
- The STRING is the target's title. Titles are unique in one file (`duplicate-use-case-name`) and, today,
  across the corpus (0 duplicates in 636), so a title is enough.

### 15.2 Model
```go
type UseCaseRef struct {
    Name      string `json:"name"`
    SourceURI string `json:"sourceUri,omitempty"`
}
type Link struct {
    ID      string      `json:"id"`               // link_<n>
    Kind    LinkKind    `json:"kind"`             // "includes" | "continues"
    Title   string      `json:"title"`            // as written
    Target  *UseCaseRef `json:"target,omitempty"` // nil when it does not resolve
    Sources []Source    `json:"sources,omitempty"`
    Line    int         `json:"line,omitempty"`
    Column  int         `json:"column,omitempty"`
}
```
`Step.Link *Link \`json:"link,omitempty"\``. Resolution runs in `analysis.resolveParticipants` over the
whole workspace: an exact title match first; then a use case whose `formerly_titled` tag lists the title
(values split on `;`, as DOMAIN-MAP defines the tag).

### 15.3 Sema / lint
| Code | Severity | When |
|---|---|---|
| `craft/syntax/missing-label` | error | A bare `includes`/`continues` ("needs a quoted use-case title"). |
| `craft/sema/unresolved-use-case` (new) | warning | No use case has the title (or a former title). A warning, not an error, because `craft validate` on one file must not fail when the target lives in a file that was not loaded. |
| `craft/sema/ambiguous-use-case` (new) | error | The title names two or more use cases (in one file or several, by title or former title). "Rename one of them; a link names one use case." |
| `craft/lint/link-to-former-title` (new) | warning | The link resolved through `formerly_titled`. The LSP offers a quick fix that writes the current title. |
| `craft/lint/continues-not-last` (new) | warning | A `continues` with another step after it in the same body (notes do not count). |
| `craft/lint/self-link` (new) | warning | A link to its own use case. |

### 15.4 Interaction semantics
None. Links are structure across use cases; the spine view (§17) and the hub read them from the model.

### 15.5 Rendering
**Sequence.** A link step appends a message of kind `MessageRef` (new), with `Over` set by the v2.23
note anchor rule (the last message's participants; across the diagram when nothing is drawn yet) and its
branch path. The label is the title for `includes` and `continues: <title>` for `continues`.
- PlantUML: `ref over <participants> : <label>` (the UML interaction use).
- Mermaid has no `ref`: `Note over <participants>: ref: <title>` / `Note over <participants>: continues: <title>`.

The first example, PlantUML (actor `Seller` declared as `user`):
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql

== Seller asks the assistant a question ==

Seller -> frontendplatform : asks the assistant a question
ref over Seller, frontendplatform : Site session preamble
frontendplatform -> graphql : to stream the answer [assistantStream]

== Seller promotes an advert ==

Seller -> frontendplatform : promotes an advert
frontendplatform -> graphql : for the promotion offers [promotionOffers]
ref over frontendplatform, graphql : continues: Seller pays at the checkout

@enduml
```
Mermaid:
```
sequenceDiagram
    actor Seller as Seller
    participant frontendplatform as frontendplatform
    participant graphql as graphql
    Note over Seller,graphql: Seller asks the assistant a question
    Seller->>frontendplatform: asks the assistant a question
    Note over Seller,frontendplatform: ref: Site session preamble
    frontendplatform->>graphql: to stream the answer [assistantStream]
    Note over Seller,graphql: Seller promotes an advert
    Seller->>frontendplatform: promotes an advert
    frontendplatform->>graphql: for the promotion offers [promotionOffers]
    Note over frontendplatform,graphql: continues: Seller pays at the checkout
```
A `MessageRef` counts as a drawn message for the empty-branch check, like a note.
**Flow, architecture, C4.** Not drawn.

### 15.6 Formatter, LSP
- **Formatter:** a link is a step line, indented like an action of its body.
- **Outline:** links appear as children of their scenario, named `includes: <title>` / `continues:
  <title>`, symbol kind `Module`.
- **Definition** on the title: the target's `use_case` line. **References** on a `use_case` title: the
  links to it.
- **Hover:** the target's file, first trigger and journey tag.
- **Completion:** inside the quotes after `includes`/`continues`, the use-case titles of the workspace.
- **Code action:** the `link-to-former-title` quick fix.

### 15.7 Tree-sitter, VSCode, hub
- tree-sitter: `link_stmt: seq(choice('includes', 'continues'), $.string)` in the `when_clause` repeat;
  both words join `_branch_keyword`; highlights as `@craft.flow-keyword`.
- VSCode: snippets `includes "…"` and `continues "…"`; completion of titles from the workspace's
  tree-sitter trees.
- Hub: a link becomes a use_case → use_case edge of kind `includes` or `continues`. The use-case API and
  `arch_explain_use_case` list them ("continues in: Seller pays at the checkout").

### 15.8 Tests
Corpus `04_use_cases/links`; broken `link_missing_title`; workspace tests for resolution (exact, former
title, unresolved, ambiguous across two files); lint tests; sequence goldens (link first in a scenario,
inside an `alt`, `continues` at the end); LSP definition, references and quick fix.

## 16. `journey` as a recognised tag
v2.25 is the first release that gives tag keys a meaning in craft: `journey` (G7) and
`formerly_titled` (G6). `sites`/`envs` got one in v2.24 (§5). Every other key stays free text, and DOMAIN-MAP
remains the authority for the corpus's tag table. A `journey` value is one slug. A `journey` tag with two
or more values is reported as `craft/lint/journey-list` (new, warning: "a use case belongs to one
journey"); `BuildJourney` then uses the first value.

## 17. Item I: the spine view (G7)

### 17.1 CLI
```
craft generate docs/topology.craft docs/actors.craft 'docs/journeys/*/*.craft' --journey seller-promotes-an-advert
```
- `--journey <slug>[,<slug>…]` writes one sequence per slug: `<base>-journey-<slug>.puml` (or `.mmd`,
  or `.md` with `--format mermaid-md`).
- `--journey all` writes one file per journey with 2 or more members. (No corpus slug is `all`.)
- `--journey` implies `--type sequence`; any other `--type` is an error ("`--journey` draws a sequence;
  drop `--type` or use `--type sequence`"). It cannot be combined with `--use-case` or `--split`.
- `--stdout` works with one slug.
- An unknown slug is an error that lists the three closest slugs.

### 17.2 Members, edges and order
`internal/journey` (new package) builds the spine from the model and its interactions:
- **Members:** every use case whose `journey` tag is the slug, across all loaded files.
- **Edges** between two different members A and B:
  - `event`: A has a `Publish` of an event that B's trigger listens to (any item of its list);
  - `store`: the same, for a store hand-off;
  - `continues`: A has a `continues` link to B.
  An `includes` link adds no edge: the included flow is shown in place by its `ref` frame.
- **Order:** a topological sort of the members over the edges (Kahn). When several members are ready,
  the one with the smallest (source file, line) comes first. When none is ready (a cycle), the remaining
  member with the smallest (source file, line) comes next and the result records `Cycle: true`; the CLI
  prints `note: journey <slug> has a hand-off cycle; its members are ordered by source position` to
  stderr. The order is deterministic.

```go
type JourneyMember struct {
    UseCase   int    `json:"useCase"` // index into CraftDoc.UseCases
    Name      string `json:"name"`
    SourceURI string `json:"sourceUri,omitempty"`
}
type JourneyEdge struct {
    From, To int             // member indices
    Kind     JourneyEdgeKind
    Event    string          // the event or store ref; "" for continues
}
type Journey struct {
    Slug    string
    Members []JourneyMember // in spine order
    Edges   []JourneyEdge
    Cycle   bool
}
func BuildJourney(doc *CraftDoc, slug string) Journey
func JourneySlugs(doc *CraftDoc) []string // sorted
```
`pkg/craft` re-exports these, so the hub orders flows with the same function instead of a copy.

### 17.3 Sema / lint
`craft/lint/journey-unconnected` (new, **hint**): a member of a journey with 2+ members that has no edge
to or from another member. The range is the `journey` tag value. "Nothing in the model connects this
use case to the rest of `<slug>`: add the `notifies`/`listens` pair, a `continues` link, or move it to
its own journey." A hint, because today about 69 journeys would raise it: it is the G6 migration
backlog, not an error in the model.

### 17.4 Rendering
`view.BuildSequence` gains options: `SequenceOptions{Order []int, SharedStage bool, Title string}`.
For a spine:
- Sections follow the spine order. Each section is a use case, with the `== title ==` divider (and the
  v2.24 scope suffix), exactly as in today's multi-use-case sequence. Scenarios keep their `group`
  frames.
- `onStage` is shared across sections. A `Listen` whose publisher was drawn in an earlier section is
  drawn as a direct `publisher ->> listener` message, as v2.22 does for a publisher in the same section.
  A store read keeps the v2.23 store-lane form. A publisher outside the spine, or later in it, keeps the
  inbound arrow with `(from …)`.
- The title is `Journey: <slug>`: PlantUML `title Journey: <slug>` after the skinparams; Mermaid a
  front-matter title. If the Mermaid render test rejects front matter, the writer uses a first
  `Note over <first>,<last>: Journey: <slug>` instead.

Example: three use cases in two files (`vas/nmf-books-the-draws.craft` sorts first).
```craft
// vas/nmf-books-the-draws.craft
use_case "nMF books the draws of a scheduled VAS" {
    tags {
        journey: seller-promotes-an-advert
    }
    when nmf_monetization listens com.olx.realestate_vas.VasScheduled
        nmf_monetization books one draw per day of the schedule
}
```
```craft
// vas/seller-promotes-an-advert.craft
use_case "Seller promotes an advert" {
    tags {
        journey: seller-promotes-an-advert
    }
    when Seller promotes an advert
        frontendplatform asks graphql to book the promotion  [scheduleVas]
        graphql asks vasscheduler to book it  [createSchedule]
        vasscheduler notifies com.olx.realestate_vas.VasScheduled
        continues "Seller pays at the checkout"
}

use_case "Seller pays at the checkout" {
    tags {
        journey: seller-promotes-an-advert
    }
    when Seller pays at the checkout
        frontendplatform asks graphql to pay  [payInvoice]
}
```
Edges: promote → books (event), promote → checkout (continues). Order: promote; then books and checkout
are both ready, and books comes first by file.

PlantUML (`Seller` declared as `user`):
```
@startuml
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center
title Journey: seller-promotes-an-advert

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
participant "vasscheduler" as vasscheduler
participant "nmf_monetization" as nmf_monetization

== Seller promotes an advert ==

Seller -> frontendplatform : promotes an advert
frontendplatform -> graphql : to book the promotion [scheduleVas]
graphql -> vasscheduler : to book it [createSchedule]
vasscheduler ->> vasscheduler : com.olx.realestate_vas.VasScheduled
ref over vasscheduler : continues: Seller pays at the checkout

== nMF books the draws of a scheduled VAS ==

vasscheduler ->> nmf_monetization : com.olx.realestate_vas.VasScheduled
nmf_monetization -> nmf_monetization : books one draw per day of the schedule

== Seller pays at the checkout ==

Seller -> frontendplatform : pays at the checkout
frontendplatform -> graphql : to pay [payInvoice]

@enduml
```
Mermaid:
```
---
title: "Journey: seller-promotes-an-advert"
---
sequenceDiagram
    actor Seller as Seller
    participant frontendplatform as frontendplatform
    participant graphql as graphql
    participant vasscheduler as vasscheduler
    participant nmf_monetization as nmf_monetization
    Note over Seller,nmf_monetization: Seller promotes an advert
    Seller->>frontendplatform: promotes an advert
    frontendplatform->>graphql: to book the promotion [scheduleVas]
    graphql->>vasscheduler: to book it [createSchedule]
    vasscheduler-)vasscheduler: com.olx.realestate_vas.VasScheduled
    Note over vasscheduler: continues: Seller pays at the checkout
    Note over Seller,nmf_monetization: nMF books the draws of a scheduled VAS
    vasscheduler-)nmf_monetization: com.olx.realestate_vas.VasScheduled
    nmf_monetization->>nmf_monetization: books one draw per day of the schedule
    Note over Seller,nmf_monetization: Seller pays at the checkout
    Seller->>frontendplatform: pays at the checkout
    frontendplatform->>graphql: to pay [payInvoice]
```
Flow, architecture and C4 do not take `--journey`; `--use-case` with the members' names already gives
those views a member filter.

### 17.5 Formatter, LSP
- **Formatter:** no change.
- **Hover** on a `journey` tag value: the members in spine order, each with its file, and "cycle" when
  there is one.
- **Definition** on a `journey` value: the other members (multiple locations, in spine order).
- **Code lens** on the `journey` tag: "N use cases in this journey". No preview command in the LSP (the
  VSCode extension previews through the craft server, §17.6).

### 17.6 Server, VSCode, tree-sitter, hub
- **craft server** (`cmd/server`): `POST /journeys` returns the journey slugs of the posted files;
  `POST /preview/journey` and `POST /preview/mermaid/journey` take the files and a slug and return the
  diagram, next to the existing `/preview/mermaid/sequence`.
- **VSCode:** a command `craft.previewJourney`: a quick pick of slugs from `/journeys`, defaulting to the
  `journey` tag under the cursor, then the preview from the server, in the existing preview panel.
- **tree-sitter:** no change (the tag syntax exists).
- **Hub:** the "flow contains use_case" edges gain `Order` from `craft.BuildJourney`, and the edges of the
  journey (`event`, `store`, `continues`) become flow-internal hand-off edges. `web/src/views/journey.ts`
  lists the use cases in that order with the hand-offs between them, and shows "cycle" when there is
  one. `arch_find_use_cases` with `journey` returns members in spine order.

### 17.7 Tests
`internal/journey` table tests: a chain, a fan-out, a `continues` edge, a store edge, ties by file and
line, a cycle, members across files, a member with a `listens` list. Sequence goldens for the example
(both backends), a publisher outside the spine, a cycle. CLI: `--journey` one slug, a list, `all`,
`--stdout`, an unknown slug, the `--type` and `--split` errors. Lint `journey-unconnected` and
`journey-list`. Server endpoints. `pkg/craft` API test for `BuildJourney`.

## 18. Consumers (v2.25)

### 18.1 `pkg/craft`
New types: `Source`, `Link`, `LinkKind`, `UseCaseRef`, `Journey`, `JourneyMember`, `JourneyEdge`,
`JourneyEdgeKind`. New constants: `LinkIncludes`, `LinkContinues`, `TagJourney`, `TagFormerlyTitled`,
`JourneyEdgeEvent`, `JourneyEdgeStore`, `JourneyEdgeContinues`. New functions: `BuildJourney`,
`JourneySlugs`. New fields: `Sources` (six types), `Step.Link`. Stability note: "Step is exactly one of
Action, Block, Note or Link (Link since v2.25.0)".

### 18.2 tree-sitter-craft, VSCode, hub
As in §§14.7, 15.7, 17.6. tree-sitter: `CORPUS_VERSION` → `v2.25.0`, publish `0.1.13`; the VSCode
extension bumps to `^0.1.13`. The hub bumps craft to v2.25.0.

### 18.3 Documentation in craft
`docs/GRAMMAR.md` §8 (`link_stmt`), §10 (`//@` source comments), §11.1 (the §12.1 table), a new
"recognised tag keys" subsection; `docs/DIAGNOSTICS.md`; `CONTEXT.md`: **Source cite**, **Link**,
**Journey (spine)**; `README.md`: `--journey`.

## 19. Testing, release, migration (v2.25)

### 19.1 Tests
Per item: §§14.8, 15.8, 17.7. Cross-cutting: existing goldens unchanged; `FuzzParseUseCaseBody` seeds
with `//@` lines and links; the lexer fuzz target with `//@` variants; formatter idempotence on files
with `//@` lines inside nested blocks. **Dogfood:** before migration, `craft validate` stays at 0 errors
and adds only `journey-unconnected` hints. `craft generate --journey seller-promotes-an-advert` (18 use
cases, 9 files) is the acceptance render.

### 19.2 Versioning and changelog
v2.25.0, minor.
- **Added:** `//@` source cites, `includes`/`continues` links, `--journey`, `BuildJourney`, the server
  endpoints, the new codes.
- **Changed:** a comment that starts with `//@` is now read as a source cite (no known uses; a malformed
  one is a warning and stays a comment).

### 19.3 Corpus migration (re-arch-central, after craft v2.25.0 and the hub bump)
1. **G4.** A script rewrites the 906–947 comment lines that start with a cite into `//@ <cites> -- <rest>`
   (the line does not grow: `// ` becomes `//@ `, and the separator replaces the punctuation after the
   cite). The 463 lines that end with a parenthesised cite are converted by hand, file by file, when the
   file is next edited. The 165 header cites stay plain comments (there is no step to attach to).
   DOMAIN-MAP "Comments → Evidence" becomes "`//@ [repo] path:lines -- text` under the step it proves".
   The hub's first freshness report is the backlog. A trial conversion gave 0 `malformed-source`, 76
   `detached-source` (55 in registries, 21 in file headers or surfaces) and 1,070 attached sources.
2. **G6.** The 17 `Follows` and 9 `preamble` headers become `includes "…"` at the point the shared flow
   runs. The non-event hand-offs among the 17 `Hands off to` headers and the 159 in-body file references
   become `continues "…"`; the event ones need nothing. `check-conventions.py` L6 (a reference to a
   missing journey file) is retired for links, which craft now resolves.
3. **G7.** Work off the `journey-unconnected` hints with G6 links, or by moving a use case to its own
   journey. R8 gains: "check a spine with `craft generate --journey <slug>`".
4. DOMAIN-MAP header: "Requires craft ≥ 2.25.0".

## 20. Decisions
1. **G10: a comma list in the existing ref slot**, continuing after a trailing comma like tag lists.
   `Event`/`Ref` stay the first item, so v2.23 consumers keep working. A roster is "any of", so no
   `one of` word is needed for listens. The view draws one arrow per trigger, because 45 arrows for one
   handler would bury the scenario; the flow view keeps one lane per event, because there every lane
   matters. Same as the v2.23 sketch.
2. **G11: tag-only.** A multiplier hides per-site differences, and the corpus shows the differences are
   real. What was missing is visibility and value checks: the scope goes in the section title, the hub
   checks values against configuration it owns, and `duplicate-scenario-body` catches copied
   scenarios. The v2.23 sketch named the lint and left out rendering; this adds the rendering.
3. **G15: `scheduler`, `tool`, `infra`**, not the sketch's `tool`/`job`/`device`. The corpus's seven
   mislabelled actors fall cleanly into these three. `job` overlaps `scheduler` (both start work on a
   clock), and `device` (mail client, browser) has no actor in the corpus: in the one case found, the
   person acts through the device, which stays a `user` trigger. There is no grammar change because the
   type slot is open by ruling Q5. The new kinds keep the `boundary` shape with a stereotype, because
   they are outside the system and not people.
4. **G16: `returns stream`**, as sketched. The request stays an `asks`; only the answer repeats. A
   one-message `loop stream` box is portable to both backends and reads as standard UML. `Answers` is
   limited to streams so existing returns keep their JSON.
5. **G17: operation lists inside one bracket**, not the sketch's several brackets. The corpus already
   writes `[a, b]` (6 lines) and nowhere writes `] [`. Keeping one bracket keeps the "last `[` on the
   line" rule, so neither craft's `opAnnotationStart` nor the tree-sitter scanner changes, and
   `Operation` stays byte-identical. The split needs comma *and* space so `{a},{b}` paths are safe. A
   route plus a client method is not a list: the method is a cite (G4).
6. **G17: `notifies one of`**, as sketched. "one of" makes the run-time choice explicit; a bare list
   would read as "publishes all of them". No `<prefix>.*` wildcard: craft could not pair it, and the
   list *is* the declared set.
7. **G20: `ADDR`/`SQL` as protocol verbs, `inferred` as the bracket head.** Verbs are an open set by
   design, so this is vocabulary, not grammar. `ADDR` addresses are quoted when they hold `//`, unlike
   the sketch's `[ADDR https://…]`, because `//` starts a comment and would cut the bracket; the
   `unquoted-address` lint catches the mistake. `inferred` lives on the action, not the operation, so
   `[inferred]` works when the operation is unknown. Grey plus a suffix, not a dashed arrow, because a
   dashed arrow already means a reply.
8. **G4: `//@` comments, attached by line**, as sketched, with two changes. The repo is optional (the
   hub derives it from the step's service), because 91 of 2,328 cites name it today and making authors
   add it would bloat every line. There is no `verified <date>`: git already knows when the cite and the
   code last changed, so a stamp would be a second, drifting copy. ` -- text` keeps the prose on the
   same line, so conversion does not add lines.
9. **G6: two link words**, `includes` and `continues`, not only the sketch's `continues`. They differ in
   meaning (the flow comes back vs. it does not) and in the spine (only `continues` orders). No
   `file::title` form, because titles are unique across the corpus; `ambiguous-use-case` guards the
   day they are not. An unresolved link is a warning, not the sketch's error, so single-file validation
   keeps working. `formerly_titled` resolution keeps links alive across renames.
10. **G7: dividers, not group frames.** The sequence view already separates use cases with `== title ==`
    and uses `group` for scenarios. Group frames per use case would nest every scenario, `alt` and
    `par` one level deeper. The order is topological over event, store and `continues` edges with a
    source-position tie-break, so it is deterministic and explainable. `BuildJourney` is public so the
    hub does not copy it.
11. Both releases are minor and additive. Every meaning change is a form with zero known uses.

## 21. Deliberately left out
- **Event wildcards** (`notifies nmfbus.*`, `listens nmfbus.*`). Craft cannot pair a wildcard, and a
  `one of` list or a `listens` list states the set (decision 6).
- **Named event sets** (a top-level `events <name> { … }` reused by several triggers). The longest lists
  (45, 22, 12) each appear once; repetition does not justify new top-level grammar yet.
- **A `for sites …` multiplier or a site parameter** (decision 2), and **site vocabularies declared in
  craft**: the site list is deployment configuration, which the hub holds.
- **Actor kinds `job` and `device`** (decision 3). An unknown kind still parses and draws as a person.
- **A Mermaid shape change for `system` actors** (still drawn as persons in Mermaid). It is a separate
  rendering fix with golden churn and no corpus request.
- **Interval data for `Ticker` triggers** (`when Ticker every "30s"`). Today a scheduled trigger always
  draws from `CRON` (v2.22); giving an in-process ticker a schedule needs its own design.
- **A request-side stream marker** (`asks … stream`). The hub derives it from `Answers` (§7.7).
- **Per-item `inferred`** inside an operation list; `inferred` on `receives` triggers and surface lines.
- **Freshness checks in craft**, and **`verified` stamps** (decision 8). Craft never reads repositories.
- **File-header cites** as data. They have no step to attach to and stay plain comments.
- **A `file::title` link form** (decision 9).
- **The `exposure` ↔ `surface` cross-check** sketched in v2.23 §15. The corpus declares no `exposure`
  and, until the v2.23 migration lands, no `surface`, so the lint would have nothing to check. Revisit
  when both exist.
- **Spine views in flow, architecture and C4.** `--use-case` with the members' names already filters
  those views.
- **Defects D1 (verb dropped from asks labels) and D7.** Separate fixes, as in v2.23.

## 22. Implementation order
Each step is one TDD slice, green between commits (`docs/AGENT.md` §4).

**v2.24.0:** (1) actor kinds: vocabulary, rendering, C4, LSP; (2) `listens` lists: syntax, model,
interaction, grouped inbound arrow; (3) `duplicate-scenario-body` and scope in section titles; (4)
operation lists; (5) `notifies one of` and the merged publish; (6) `ADDR`/`SQL`/`inferred` and their
lints; (7) `returns stream` with `Answers`; (8) docs; (9) tree-sitter `0.1.12`; (10) VSCode; (11) hub.

**v2.25.0:** (1) `//@` lexing, attachment and `Sources`; (2) links: syntax, resolution, lints,
`MessageRef`; (3) `internal/journey` and `BuildJourney`; (4) spine rendering and `--journey`; (5)
`journey-unconnected`, `journey-list`; (6) LSP (hover, definition, document links, code lens); (7) server
endpoints; (8) docs; (9) tree-sitter `0.1.13`; (10) VSCode (`craft.previewJourney`, document links);
(11) hub (sources and freshness job, link edges, flow order).

Slices in sibling repos ship in the same window as their release.
