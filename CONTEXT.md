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
A person or external system that starts a use case.

**Use case**:
A business scenario: a trigger followed by an ordered list of actions.

**Action**:
One step of a use case: a participant doing something, asking another participant, notifying an event, or returning.

**Event**:
A named fact published by a bounded context and listened to by others.

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

### Diagrams

**View**:
What a diagram shows, independent of how it is written: sequence, flow, architecture or C4.
_Avoid_: diagram type, mode

**Backend**:
The diagram language a view is written in: PlantUML or Mermaid.
_Avoid_: renderer, engine
