# Craft

Craft is a language for describing a business domain — its bounded contexts, the services that deploy them, and the use cases that move through them — and for drawing diagrams from that description.

## Language

### Modeling

**Bounded context**:
A solution-space unit of the domain model, declared inside a domain and deployed by exactly one service.
_Avoid_: BC (in prose), subdomain, module

**Domain**:
A named grouping of bounded contexts.

**Service**:
A deployable unit that hosts one or more bounded contexts and owns its data stores.
_Avoid_: microservice, app

**Data store**:
A persistent store owned by a service.
_Avoid_: database (as a participant name)

**Actor**:
A person or external system that starts a use case. Its kind is one of `user`, `system`, `service`, `scheduler` (a clock), `tool` (a CLI or client a person runs) or `infra` (platform machinery); any other kind is drawn as a person.

**Use case**:
A business scenario: a trigger followed by an ordered list of actions.

**Action**:
One step of a use case: a participant doing something, asking another participant, notifying an event, or returning.

**Event**:
A named fact published by a bounded context and listened to by others.

**Store hand-off**:
An event whose ref names a data store, `store.<name>.<change>`: the writer notifies it and the reader polls it with `listens`. Views draw the store as its own participant between them.
_Avoid_: queue event, table event

**Note**:
A one-line remark (`note "…"`) attached to a use case, a step position or a surface. It is not an action: it gets no interaction and draws over the step before it.

**Finding**:
A note (`finding "…"`) that records a defect or a risk the author found. It is exported to the model and drawn highlighted.
_Avoid_: issue, warning (reserved for diagnostics)

**Surface**:
The inventory of operations one boundary offers (`surface <name> { … }`), each marked `query` or `command`, with its callers. Coverage ("used by") is computed from the use cases.
_Avoid_: API, exposure (an exposure says who may reach a context, not what it offers)

**Roster trigger**:
A `listens` trigger with a list of events (`when seshat listens a.A, b.B`): any one of them starts the scenario, and the body is the same for each.
_Avoid_: multi-event trigger, OR-trigger

**Operation list**:
Two or more operations of one hop, written as a comma list in one bracket (`[CreateApiKey, CreateUsagePlanKey]`). Two hops to two parties are two `asks`, not a list.
_Avoid_: multi-operation, batch

**Streamed reply**:
A reply that repeats until it ends (`returns stream`). It answers the `asks` that opened it and is drawn in a `loop stream` box.
_Avoid_: stream (alone, it also names event streams), SSE

**Inferred hop**:
A step marked `[inferred …]`: the hop is inferred from names, configuration or convention, not read in code. Drawn grey with "(inferred)".
_Avoid_: probable call, guess

**Egress**:
A call that leaves through an address with no operation id (`[ADDR …]`), or a direct use of another party's tables (`[SQL …]`).
_Avoid_: outbound call (for an ordinary `asks`)

### Analysis

**Model**:
Everything declared across the files of one folder, merged and resolved. A single file is a folder of one.
_Avoid_: document, project

**Participant**:
The resolved identity behind a name in a use case: an actor, bounded context, domain, service or data store.
_Avoid_: node, party, entity

**Interaction**:
One derived communication between two participants — a request, a return, an event publication, an event listen, or an external trigger.
_Avoid_: relationship (reserved for context-map relationships), edge, call

**Unobserved caller**:
The unknown sender of a request that a `when <ctx> receives …` trigger starts from. It is not a participant: views draw a found message into the callee and a lost message for its reply.
_Avoid_: NoObservedCaller, anonymous actor

### Diagrams

**View**:
What a diagram shows, independent of how it is written: sequence, flow, architecture or C4.
_Avoid_: diagram type, mode

**Backend**:
The diagram language a view is written in: PlantUML or Mermaid.
_Avoid_: renderer, engine
