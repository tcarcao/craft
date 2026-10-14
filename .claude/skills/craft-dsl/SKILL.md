---
name: craft-dsl
description: "Generate, extend, explain, and analyze Craft DSL (.craft) files — a domain-specific language for modeling DDD architectures with actors, domains, services, use cases, architecture blocks, exposures, context maps, and glossaries. Use this skill whenever the user works with .craft files, mentions the Craft DSL, wants to model a domain or system in Craft, asks to add or modify use cases, services, domains, actors, exposures, arch blocks, context maps, or glossaries in a .craft file, or wants to understand an existing .craft file. Also trigger when the user wants to document domain-driven design artifacts and there are .craft files in the project."
---

# Craft DSL

Craft is a declarative language for documenting domain-driven design architectures. It models **who** interacts with the system (actors), **what** the system does (domains, services), **how** it's structured (architecture, exposures), and **what happens** at runtime (use cases with event-driven flows).

This skill enables you to generate valid `.craft` files from natural language, extend existing ones, explain what they describe, and identify modeling gaps.

---

## Before You Start

1. **Check for existing `.craft` files** — look in `docs/` first, then the project root. Read them to understand naming conventions, domain vocabulary, and style.
2. **Assess context sufficiency** — if the user provides enough detail (domains, actors, key flows), generate directly. If vague ("model a payment system"), ask about: who are the actors? what are the main domain boundaries? what are the key use cases?
3. **When extending**, always read the full existing file first. Match naming style (e.g., if existing services use `golang`, don't switch to `go`).

---

## Grammar (BNF)

This is the complete grammar. All generated `.craft` files must conform to it.

```bnf
<dsl>                ::= (<actor_def> | <actors_def> | <domain_def> | <domains_def>
                         | <service_def> | <services_def> | <context_map> | <glossary> | <arch> | <exposure>
                         | <use_case>)*

// --- Actors: who interacts with the system ---
<actor_def>          ::= "actor" <actor_type> <identifier>
<actors_def>         ::= "actors" "{" (<actor_type> <identifier>)+ "}"
<actor_type>         ::= "user" | "system" | "service" | "scheduler" | "tool" | "infra"
                       // any other identifier parses (drawn as a person); prefer the six known kinds

// --- Domains: business capabilities with contexts ---
// Children (bounded contexts) are NEWLINE-separated, not comma-separated
<domain_def>         ::= "domain" <identifier> "{" <identifier>+ "}"
<domains_def>        ::= "domains" "{" (<identifier> "{" <identifier>+ "}")+ "}"

// --- Services: deployable units grouping bounded contexts ---
<service_def>        ::= "service" <service_name> "{" <service_property>+ "}"
<services_def>       ::= "services" "{" (<service_name> "{" <service_property>+ "}")+ "}"
<service_name>       ::= <identifier> | <string>
<service_property>   ::= "contexts" ":" <identifier_list>
                       | "data-stores" ":" <identifier_list>
                       | "language" ":" <identifier>
                       | "deployment" ":" <deployment_strategy>
                       | "catalog_ref" ":" <identifier>
                       | "repo" ":" <slug_path>
<deployment_strategy>::= ("canary" | "blue_green" | "rolling")
                         ("(" <deployment_rule> ("," <deployment_rule>)* ")")?
<deployment_rule>    ::= <percentage> "->" <identifier>

// --- Node slugs: typed, code-anchored references (vNext) ---
// [kind:][namespace/]name; NOT used in use_case bounded-context slots (see
// bc_ref below); a kind: prefix there is a parse error
<node_slug>          ::= (<slug_kind> ":")? <slug_path>
<slug_kind>          ::= "domain" | "bc" | "term" | "service"
<slug_path>          ::= <identifier> ("/" <identifier>)*
// Event ref: dotted qualified id (FQ Avro record name / OpenAPI operationId)
<event_ref>          ::= <identifier> ("." <identifier>)+ | <string>   // <string> form is DEPRECATED

// --- Context Map: DDD strategic relationship patterns between bounded contexts ---
<context_map>        ::= "context_map" <identifier>? "{" <edge_stmt>+ "}"
<edge_stmt>           ::= <bc_ref> <pattern> <bc_ref>
<bc_ref>              ::= <identifier> ("/" <identifier>)?   // bare, or domain-qualified — no bc:/service:/term: prefix
<pattern>             ::= "customer_supplier" | "conformist" | "anticorruption_layer"
                        | "open_host_service" | "published_language"
                        | "partnership" | "shared_kernel" | "separate_ways"

// --- Glossary: symmetric term relations across bounded contexts ---
<glossary>           ::= "glossary" <identifier>? "{" <term_stmt>+ "}"
<term_stmt>           ::= <term_node> <term_verb> <term_node>
<term_node>           ::= <identifier> "/" <identifier> ("/" <identifier>)?   // <bc>/<term> or <domain>/<bc>/<term>; last segment is the term
<term_verb>           ::= "same_as" | "contrasts" | "distinct_from"           // all symmetric — order carries no meaning

// --- Architecture: component topology ---
<arch>               ::= "arch" <identifier>? "{" <arch_section>+ "}"
<arch_section>       ::= ("presentation" | "gateway") ":" <arch_component>+
<arch_component>     ::= <component_chain> | <simple_component>
<component_chain>    ::= <component_with_mods> (">" <component_with_mods>)+
<simple_component>   ::= <component_with_mods>
<component_with_mods>::= <identifier> ("[" <modifier> ("," <modifier>)* "]")?
<modifier>           ::= <identifier> (":" <identifier>)?

// --- Exposures: external access definitions ---
<exposure>           ::= "exposure" <identifier> "{" <exposure_property>+ "}"
<exposure_property>  ::= "to" ":" <identifier_list>
                       | "contexts" ":" <identifier_list>
                       | "through" ":" <identifier_list>

// --- Use Cases: business scenarios with event-driven flows ---
<use_case>           ::= "use_case" <string> "{" <scenario>+ "}"
<scenario>           ::= <trigger> <action>+
<trigger>            ::= "when" <bc_ref> "listens" <event_list>
                       | "when" <identifier> <verb> <phrase>?
                       | "when" <event_ref>
<action>             ::= <sync_action> | <async_action> | <internal_action> | <return_action>
                       | <link_stmt>                                        // v2.25: a step, not an action
<link_stmt>          ::= ("includes" | "continues") <string>               // a use-case title
<sync_action>        ::= <bc_ref> "asks" <bc_ref> <phrase> <op_annotation>?
<async_action>       ::= <bc_ref> "notifies" (<event_ref> | "one" "of" <event_list>) <op_annotation>?
<internal_action>    ::= <bc_ref> <verb> <phrase> <op_annotation>?
<return_action>      ::= <bc_ref> "returns" "stream"? ("to" <bc_ref>)? <phrase> <op_annotation>?
<event_list>         ::= <event_ref> ("," <newline>* <event_ref>)*   // v2.24; a trailing comma continues on the next line
// bc_ref is the same production used by context_map endpoints (defined
// above): bare or domain-qualified, no kind: prefix. A kind: prefix in any
// of these slots is a craft/syntax/kind-prefix-in-target parse error.

// --- Operation annotations: trailing bracket on any action (vNext) ---
<op_annotation>      ::= "[" "inferred"? (<op_item> (", " <op_item>)*)? "]"   // v2.24: inferred, item lists
<op_item>            ::= (<protocol_verb> <ws>)? <payload>
<protocol_verb>      ::= "GET" | "POST" | "PUT" | "PATCH" | "DELETE"
                       | "HEAD" | "OPTIONS" | "GRPC" | "TOPIC" | "QUERY"
                       | "ADDR" | "SQL"                                       // v2.24
<payload>            ::= <raw-text-up-to-closing-bracket>
// The annotation is the LAST "[" on the line whose "]" is the line's final
// token; a "[" that does not close at end of line stays ordinary prose. An
// empty annotation "[]" is a craft/syntax/empty-op-annotation error.
// Items split at a comma followed by whitespace, outside braces and strings:
// "[GET /p/{lat},{lng}]" is one item. An ADDR address containing " //" must be quoted.

// --- Source cites (v2.25): a line comment that starts with //@ ---
<source_comment>     ::= "//@" " " <cite> (", " <cite>)* (" -- " <text>)?
<cite>               ::= (<repo> " ")? <path> (":" <ranges>)? | ":" <ranges>   // ":" alone reuses the previous path
<repo>               ::= [a-z0-9][a-z0-9._-]*
<path>               ::= [^\s:,()]+                    // must hold "/" or "."
<ranges>             ::= <range> ("," <range>)*        // no spaces: sqs.tf:3,26
<range>              ::= [0-9]+ ("-" [0-9]+)?
// A //@ line attaches to the nearest step line at or above it in the same
// use case (action, branch header, note, link or trigger); before the first
// scenario, to the use case. Outside a use case it is detached (warning).

// --- Terminals ---
<connector>          ::= "a" | "an" | "the" | "as" | "to" | "from" | "in" | "on"
                       | "at" | "for" | "with" | "by"
// <phrase> is captured as raw rest-of-line prose (vNext), not word-by-word —
// special characters (! & * / # ? + ...) are legal unquoted; quoted strings
// still work anywhere a phrase does. See "Flexible Prose" below.
<phrase>             ::= <raw-text-to-end-of-line>
<verb>               ::= <identifier>
<identifier>         ::= [a-zA-Z0-9_][a-zA-Z0-9_.-]*
<string>             ::= '"' [^"\r\n]* '"'
<percentage>         ::= [0-9]+ "%"
// Continuation lines allowed: end intermediate lines with a comma, no trailing comma on last item
<identifier_list>    ::= <identifier> ("," <identifier>)* ","?
```

**Key grammar notes:**
- Newlines are significant — they separate list items, actions, and blocks (not just whitespace)
- Comments use `//` (single-line), but **only when `//` is preceded by whitespace or starts the line**. `http://api`, `50/50`, and `and/maybe` inside a narrative phrase are NOT comments — the `/` there has no whitespace before it. ` // trailing note` (space before `//`) is a comment. Block comments `/* ... */` follow the same whitespace-preceded rule.
- Whitespace (spaces, tabs) is ignored; newlines are not
- **Multi-line lists are supported** — end each intermediate line with a comma and continue on the next line. The last item must not have a trailing comma when followed by another property
- **Flexible prose**: narrative tails (the `<phrase>` in `asks`, internal actions, `returns`, and external triggers) are captured as raw rest-of-line text. Special characters like `! & * / # ? +` don't need quoting: `Subscriptions asks re/billing for a fresh charge attempt (1! & 2!)` is valid as-is. One exception: a bracketed run that closes at the end of the line is an **operation annotation** (see "Operation Annotations" below), not prose.
- **Typed refs replace free-text event strings**: `notifies`/`listens` take an `<event_ref>` — a dotted qualified id (e.g. `vas.VasApplied`), not prose. The old quoted-string form (`notifies "Order Created"`) still parses but is **deprecated** — see "Deprecated: Quoted Event Strings" below.

---

## Construct Reference

### Actors

Actors represent entities that interact with the system. Six known kinds:
- `user` — human users (e.g., Customer, Admin, Business_User)
- `system` — external systems (e.g., PaymentGateway)
- `service` — internal services from other bounded contexts (e.g., Database, NotificationService)
- `scheduler` (v2.24) — a clock that starts work (e.g., CRON, Ticker)
- `tool` (v2.24) — a CLI, script or client a person runs; it, not the person, makes the calls (e.g., OperatorTool)
- `infra` (v2.24) — platform machinery: pod start, probes, CI pipelines, synthetic monitors (e.g., Platform, Pipeline)

Pick the kind that says what the initiator is; do not use `system` for a clock or a CI pipeline. The three new kinds draw as stereotyped boundaries (`<<scheduler>>`), and they need craft ≥ 2.24 (older versions misread the line).

Single: `actor user Customer`
Bulk:
```
actors {
    user Business_User
    scheduler CRON
    service Database
}
```

### Domains

Business capabilities organized hierarchically. The parent is the domain; children are bounded contexts within that domain.

Single:
```
domain User {
    Authentication
    Profile
}
```

Bulk:
```
domains {
    User {
        Authentication
        Profile
    }
    Monetization {
        Wallet
        OrderManagement
    }
}
```

### Services

Deployable units that group bounded contexts with technology stack details.

Properties (all optional except `contexts`):
- `contexts:` — which bounded contexts this service owns
- `data-stores:` — persistence (note the **hyphen**: `data-stores`, not `datastores`)
- `language:` — implementation language
- `deployment:` — strategy: `rolling`, `blue_green`, or `canary(percentage -> target, ...)`
- `catalog_ref:` — the service's stable id in the org's service catalog (identity anchor; vendor-neutral)
- `repo:` — the service's repo slug (code anchor; vNext, no file paths)

```
services {
    UserService {
        contexts: Authentication, Profile
        data-stores: user_db, redis_cache
        language: golang
        deployment: rolling
    }
    "Order Service" {
        contexts: OrderManagement, OrderFulfilment
        data-stores: postgres
        deployment: canary(20% -> staging, 80% -> production)
    }
    SubscriptionsApi {
        contexts: Subscriptions
        catalog_ref: subscriptions-api
        repo: olxeu/realestate/subscriptions
    }
}
```

Service names with spaces must be quoted: `"Order Service"`.

For a single standalone service outside a `services` block, use `service UserService { ... }` instead of wrapping in `services { }`.

**Service anchors (`catalog_ref:`/`repo:`):** these bind a service block to real code identity — `catalog_ref:` is the service's stable identifier in the org's service catalog, `repo:` is a repo slug (not a file path). The principle is **bind by identity, never by location**: no file paths, no line numbers, because those rot. The grammar deliberately does not name the catalog vendor — which catalog resolves the anchor is deployment configuration, so the catalog can be swapped without a DSL migration. (It is `catalog_ref`, not `catalog_slug`: "slug" is reserved here for *mutable* human-readable names.) Each may appear **at most once per service** — a repeated `catalog_ref:` or `repo:` in the same block is a `craft/sema/duplicate-service-anchor` error. Craft only checks local shape; a hub system resolves the anchor against real infrastructure.

### Node Slugs

A **node slug** is the typed, code-anchored way to reference a domain, bounded context, term, or service: `[kind:][namespace/]name`, where `kind ∈ {domain, bc, term, service}`. Each kind has its own namespace shape:

| Kind | Shape | Example |
|------|-------|---------|
| `domain` | `domain:re/<name>` | `domain:re/monetization` |
| `bc` | `bc:<domain>/<name>` | `bc:re/subscriptions` |
| `term` | `term:<bc>/<name>` | `term:billing/dunning` |
| `service` | `service:<alias>` (no namespace) | `service:subscriptions-api` |

A malformed namespace (wrong segment count, empty segment, or an unrecognised `kind:` word) is a `craft/sema/malformed-slug` error.

**`use_case` bounded-context slots do not accept node slugs.** The subject of any action (`asks`, `notifies`, `returns`, or an internal action), the `asks` target, the `returns` target, and the `when ... listens` trigger context each accept a bare name (`Billing`) or a domain-qualified `<domain>/<name>` reference (`re/billing`), never a `kind:` prefix. `Subscriptions asks re/billing for a fresh charge attempt` is correct; `Subscriptions asks bc:re/billing for a fresh charge attempt` is a `craft/syntax/kind-prefix-in-target` parse error.

`context_map` endpoints likewise do **not** use node slugs: they use a simpler bare-or-domain-qualified bounded-context reference (see below).

**Term module-scoping:** a bare term name written inside its own bounded context resolves to that BC's namespace automatically. Any **cross-BC** term reference must use the fully-qualified `term:<bc>/<name>` slug — a bare cross-context term reference can't be locally verified and is a hub-side error.

### Context Map

The `context_map` block declares the DDD **strategic relationship** between two bounded contexts — customer/supplier, conformist, anticorruption layer, open-host service, published language, partnership, shared kernel, or separate ways:

```craft
context_map re {
    billing customer_supplier vas
    billing open_host_service subscriptions
    billing partnership vas
}

context_map {
    re/billing separate_ways legacy/reporting
}
```

- The block is **repeatable** and optionally scoped to a domain (`context_map re { }`); an unscoped `context_map { }` is the shared/global map.
- Endpoints (`bc_ref`) are bare (`billing`) or domain-qualified (`re/billing`) bounded-context names — **no `bc:`/`service:`/`term:` prefix** inside `context_map`. `/` is the domain/bc separator; `.` stays reserved for event refs and must never appear in an endpoint.
- One pattern per statement, one statement per line.

Eight patterns — five **directional** (endpoint order carries meaning, `LEFT = upstream`, `RIGHT = downstream`) and three **symmetric** (order is meaningless):

| Pattern | Class | Left (role) | Right (role) |
|---|---|---|---|
| `customer_supplier` | directional | supplier (upstream) | customer (downstream) |
| `conformist` | directional | upstream (model owner) | conformist (downstream) |
| `anticorruption_layer` | directional | upstream | downstream (owns the ACL) |
| `open_host_service` | directional | host (upstream) | consumer (downstream) |
| `published_language` | directional | publisher (upstream) | consumer (downstream) |
| `partnership` | symmetric | — | — |
| `shared_kernel` | symmetric | — | — |
| `separate_ways` | symmetric | — | — |

Validation is about shape, not modeling correctness:

| Code | Severity | When |
|------|----------|------|
| `craft/sema/edge-endpoint-not-bc` | error | an endpoint resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/self-relationship` | error | both endpoints resolve to the same bounded context (`X <pattern> X`) |
| `craft/sema/ambiguous-bc` | error | a bare endpoint name is a bounded context in two or more domains — qualify it as `<domain>/<name>` |
| `craft/sema/unresolved-bc` | warning | an endpoint doesn't resolve to any declared bounded context |
| `craft/lint/redundant-relationship` | warning | the same unordered pair is declared with the same **symmetric** pattern more than once (directional duplicates in opposite order are not redundant) |

**Cross-validation with the communication view.** When a workspace is validated (multiple files, or one file with both a `context_map` and use cases), craft cross-checks each declared relationship against the communication it *infers* from use-case `asks`/`notifies`. Both interaction primitives reduce to one directed **dependency edge `D → U`** ("downstream depends on upstream"): `X asks Y` gives `X → Y`; `Y notifies E` paired with `when X listens E` gives `X → Y`. With `LEFT = upstream`, every directional pattern expects the observed edge to run `RIGHT → LEFT`. Four signals result:

| Code | Severity | When |
|------|----------|------|
| `craft/lint/separate-ways-violation` | warning | a `separate_ways A B` edge exists, yet *any* dependency edge (either direction, sync or async) is observed between A and B |
| `craft/lint/relationship-direction-inverted` | warning | a directional pattern edge whose *only* observed dependency runs the wrong way (`LEFT → RIGHT`), with no correct-direction edge present |
| `craft/lint/relationship-bidirectional` | hint | a directional pattern edge with observed dependency in *both* directions (consider `partnership`) |
| `craft/lint/unclassified-communication` | hint | a bounded-context pair that clearly communicates has *no* `context_map` edge at all, in any block |

Guiding principle: **absence of communication never warns.** A declared relationship with no observed traffic is silent — the lint only flags observed communication that *contradicts* a declaration (or is entirely unclassified).

### Glossary

The `glossary` block is craft's **ubiquitous-language view**: it relates *terms* across bounded contexts, where `context_map` relates the bounded contexts themselves. The same word can mean the same, a related-but-different, or a deliberately distinct thing depending on which context you stand in — `glossary` records that judgment.

```craft
glossary re {
    billing/Invoice     same_as        subscriptions/Invoice
    billing/dunning     contrasts      subscriptions/dunning
}

glossary {
    re/billing/Invoice  distinct_from  legacy/reporting/Invoice
}
```

- The block is **repeatable** and optionally scoped to a domain (`glossary re { }`); an unscoped `glossary { }` is the shared/global glossary. All blocks merge into one glossary.
- Each statement is `<term_node> <verb> <term_node>`, one relation per line.
- **Term-node identity:** a term node is `<bc>/<term>` or `<domain>/<bc>/<term>` — the **last** `/`-separated segment is the term name, everything before it is the bounded-context reference (bare `bc` or `domain/bc`). `/` is the separator (one level deeper than a `context_map` endpoint); `.` stays reserved for event refs and must never appear in a term node. A bare term with no BC segment (just `Invoice`) is invalid. Terms are only *referenced*, never declared — `glossary` records relations, not definitions.

Three verbs, **all symmetric** (order carries no meaning — `A verb B` equals `B verb A`):

| Verb | Meaning |
|---|---|
| `same_as` | the two terms denote the same concept across the context boundary |
| `contrasts` | the two terms are related but meaningfully different |
| `distinct_from` | the two terms are unrelated beyond sharing a name |

Validation checks shape and resolution, not modeling correctness:

| Code | Severity | When |
|------|----------|------|
| `craft/sema/glossary-endpoint-not-bc` | error | the BC part of a term node resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/glossary-unresolved-bc` | warning | the BC part of a term node doesn't resolve to any declared bounded context |
| `craft/sema/glossary-ambiguous-bc` | error | a bare BC name is a bounded context in two or more domains — qualify it as `domain/bc/term` |
| `craft/sema/glossary-self-relation` | error | both sides resolve to the same term node (same BC and same term) |
| `craft/lint/glossary-redundant` | warning | the same unordered term-node pair is declared with the same verb more than once |
| `craft/lint/glossary-conflicting-relation` | warning | the same unordered term-node pair is declared `same_as` and also `distinct_from` or `contrasts` |

### Architecture

Defines component topology with two sections:
- `presentation:` — frontends and client-facing components
- `gateway:` — API gateways and routing layers

Components can have modifiers `[key:value]` and be chained with `>` to show flows:

```
arch {
    presentation:
        WebApp[framework:react, ssl]
        MobileApp

    gateway:
        LoadBalancer[ssl:true] > APIGateway[type:nginx]
}
```

### Exposures

Define which actors can access which bounded contexts through which gateways:

```
exposure PublicAPI {
    to: Customer, Partner
    contexts: Catalog, Order, Payment
    through: APIGateway
}
```

### Use Cases

The core dynamic modeling construct. Each use case contains scenarios triggered by events or actors, with domains performing actions.

**Trigger types:**

| Type | Syntax | Example |
|------|--------|---------|
| External (actor) | `when <actor> <verb> <phrase>` | `when Customer places order` (actor can be any name, including `CRON` for scheduler-driven triggers) |
| Event | `when <event_ref>` | `when order.OrderPlaced` |
| Bounded context listener | `when <context> listens <event_ref>` | `when Payment listens order.OrderCreated` (`<context>` can be bare or domain-qualified: `when re/billing listens vas.VasApplied`) |

**Action types:**

| Type | Keyword | Syntax | Meaning |
|------|---------|--------|---------|
| Synchronous | `asks` | `Order asks Inventory to reserve items` | Bounded context-to-bounded context call. Subject and target can each be a bare name (`Inventory`) or domain-qualified (`re/billing`), never a `kind:` prefix (`bc:re/billing` is a `craft/syntax/kind-prefix-in-target` error) |
| Asynchronous | `notifies` | `Order notifies order.OrderCreated` | Publish event, referenced by a typed event ref. Subject can be bare or domain-qualified |
| Internal | any verb | `Order validates items` | Bounded context-internal operation. Subject can be bare or domain-qualified, never a `kind:` prefix, same as the other three kinds |
| Return | `returns` | `Database returns to Auth the user record` | Return response. Subject and `to` target can each be bare or domain-qualified |

An `<event_ref>` is a dotted qualified id (the FQ Avro record name / OpenAPI `operationId` the event corresponds to in code) — `vas.VasApplied`, `com.olx.re.subscriptions.SubscriptionCreated`. No `kind:` prefix, no `/`.

**Operation annotations:** any action may end with a bracketed annotation describing the wire call it makes:

```craft
Subscriptions asks Billing for a fresh charge attempt  [POST /v1/accounts/{id}/charges]
Billing asks Ledger to record the entry                [GRPC ledger.Postings/Create]
Billing notifies billing.ChargeSucceeded                [TOPIC billing.v1.charge-succeeded]
Subscriptions asks Legacy for a reconciliation          [legacy-mainframe-txn-44]
```

If the first token inside `[...]` is a recognised protocol verb (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `GRPC`, `TOPIC`, `QUERY`), the annotation is structured (verb + payload); otherwise the whole content is stored verbatim as an opaque payload, with no diagnostic. `[legacy-mainframe-txn-44]` is just as valid as `[POST /v1/charges]`. The annotation is the **last** `[` on the line whose `]` is the line's final token; a `[` that doesn't close at end of line stays prose. An empty `[]` is a `craft/syntax/empty-op-annotation` error.

`craft fmt <files...>` formats in place and column-aligns annotations, one column per contiguous run of annotated lines within a scenario; a non-annotated action inside a run does not reset it, but a blank line or a new scenario does. Trailing `//` comments are aligned the same way, one column per contiguous run within a block. `craft fmt --check <files...>` writes nothing, names the unformatted files, and exits non-zero. Arguments are paths or globs (`**` supported); directories are not walked.

Formatting changes whitespace and nothing else. Since v2.17.0 this is structural rather than a promise: the formatter walks the syntax tree's token stream and writes every non-whitespace token back verbatim, exactly once, in document order, so its only freedom is the whitespace between tokens. It applies a 4-space indent by default, configurable via a `.craftfmt` file or the `--indent` flag, one statement per line, and a blank line between top-level declarations and before each `when` after the first. A comment stays on the line the author put it, including a trailing comment at the end of an action line, which earlier versions lifted onto its own line above; its column may shift to align with other trailing comments in the same block. Runs of interior spaces inside a statement collapse to one, so `Billing    asks    Ledger` becomes `Billing asks Ledger`, but a line break the author wrote inside a wrapped value is preserved rather than joined. Formatting is idempotent for every input, and is verified over every `.craft` file in the repo for reparse-cleanliness, idempotence, model preservation, and content preservation. In `pkg/craft` (the stable public Go API), the annotation is `craft.Operation` (`Verb`/`Payload`/`Text`), the verb set is `craft.OpVerbGET` through `craft.OpVerbQUERY`, and `craft.ProtocolVerbs()` returns it as a `[]string`.

**Use-case diagnostics:**

| Code | Severity | When |
|------|----------|------|
| `craft/syntax/kind-prefix-in-target` | error | a `kind:` prefix in the subject of any action (`asks`, `notifies`, `returns`, or an internal action), the `asks` target, the `returns` target, or the `when ... listens` trigger context |
| `craft/syntax/empty-op-annotation` | error | an action ends in `[]`; or (v2.24) an operation list has an empty item, or `[ADDR]`/`[SQL]` has no payload |
| `craft/sema/store-ref-in-event-list` | error | a `store.` ref in a `listens` or `notifies one of` list of two or more items |
| `craft/lint/duplicate-scenario-body` | warning | two scenarios of one use case run the same steps; fold `listens` triggers into one list |
| `craft/lint/duplicate-event-in-list` / `duplicate-operation-in-list` | warning | the same event twice in one list / the same operation twice in one bracket |
| `craft/lint/one-of-single-event` | warning | `notifies one of` with a single event |
| `craft/lint/stream-without-caller` | warning | a `returns stream` with no open request and no `to` target |
| `craft/lint/egress-on-internal-step` | warning | `[ADDR …]`/`[SQL …]` on an internal step or a return; make it an `asks` |
| `craft/lint/unquoted-address` | warning | a ` //` comment cut an `[ADDR …]` bracket; quote the address |
| `craft/syntax/malformed-source` | warning | (v2.25) a `//@` line that is not a cite; it stays a plain comment |
| `craft/lint/detached-source` | warning | (v2.25) a `//@` cite outside every use case |
| `craft/syntax/missing-label` | error | also (v2.25) a bare `includes`/`continues` with no title |
| `craft/sema/unresolved-use-case` / `ambiguous-use-case` | warning / error | (v2.25) a link title names no loaded use case / two or more |
| `craft/lint/link-to-former-title` / `continues-not-last` / `self-link` | warning | (v2.25) a link through a `formerly_titled` value / a step after `continues` / a link to its own use case |
| `craft/lint/journey-list` / `journey-unconnected` | warning / hint | (v2.25) a `journey` tag with two values / a journey member no hand-off connects |
| `craft/sema/ambiguous-bc` | error | a bare bounded-context name is declared in two or more domains, at exactly four sites: the `sync_action` subject and target, the `async_action` subject, or the `domain_listen` trigger context. Qualify it as `<domain>/<name>`. Does **not** fire for an internal action's subject or a `returns` subject/target |
| `craft/sema/malformed-slug` | error | a qualified `<domain>/<name>` reference has the wrong shape (empty segment, or more than two segments) in any of: any action's subject, the `asks` target, the `returns` target, or the trigger context |

**Event-driven pattern:** Bounded contexts publish events with `notifies`, and other scenarios react with `when <context> listens <event_ref>`. This models async choreography — the heart of good Craft modeling.

```
use_case "Purchase Item" {
    when Customer adds item to cart
        Cart validates item availability
        Cart asks Inventory to reserve items
        Cart notifies cart.ItemAdded

    when Checkout listens cart.ItemAdded
        Checkout asks Payment to process payment
        Payment notifies payment.PaymentProcessed

    when Fulfilment listens payment.PaymentProcessed
        Fulfilment asks Warehouse to ship order
        Fulfilment notifies fulfilment.OrderShipped
}
```

**Journey constructs (v2.23).** Use them instead of prose comments:

```craft
use_case "Seeker reads an advert" {
    finding "nothing retries a failed resolver call"
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        par "field resolvers"
            graphql asks atlas for the SEO title  [postSeoadvertsbatch]
            graphql asks atlas for the targeting data  [postAdverttargetingbatch]
        loop "for each image, until all are signed"
            graphql asks media to sign the image URL  [signUrl]
        note "media signs one URL per call"
}
```

- `par "<label>"` runs steps concurrently. Without `and`, each body step is its own branch (one caller fanning out N requests). With branches of several steps, add `and "<label>"` at the `par`'s column. `loop "<condition>"` repeats its body. Both nest like `alt`/`opt`.
- `note "…"` and `finding "…"` are one-line statements in a step position, at use-case level before the first `when`, or in a surface. A finding records a defect or risk.
- `when <context> receives <phrase> [op]` starts a scenario at a boundary whose caller is not observed. Do not invent a caller actor.
- `notifies store.<name>.<change>` / `when <context> listens store.<name>.<change>` (change `inserted`, `updated` or `deleted`) is a hand-off through a data store. A polling listener adds `cron "<expr>"` or `every "<interval>"` after the ref.
- `surface <boundary> { callers: …  spec: "…"  query <phrase> [op]  command <phrase> [op] }` lists every operation a context, service or external actor offers; `craft generate --type surface` tabulates which use cases call each one, and `craft/lint/operation-not-in-surface` flags an `asks` to an operation the surface does not list.

**Journey constructs (v2.24).** Use them instead of one scenario per event, prose comments or bracket workarounds:

```craft
use_case "seshat records a Mercury domain event in the transaction ledger" {
    when seshat listens sns.mercury-advert-state-changed, sns.mercury-vas-state-changed,
            sns.mercury-message-received, sns.mercury-cooked
        seshat writes one row per event
        seshat notifies one of ledger.RowWritten, ledger.RowSkipped
}
```

- **Roster trigger:** `when <ctx> listens a.A, b.B, …` — any of these events starts the scenario; the body is the same for each. A trailing comma continues the list (continuation 8 columns past `when`). No store ref in a list of two or more. Two scenarios with the same body get `craft/lint/duplicate-scenario-body`, which gives the folded trigger.
- **`notifies one of a.A, b.B`** — exactly one of these events per run, chosen at run time. A bare `notifies a.A, b.B` is an error: to publish several, write one `notifies` per event. A `notifies` event is one name; words after it are an error.
- **Operation list:** `[CreateApiKey, CreateUsagePlanKey]` — one hop that uses several operations. Say in the phrase whether they are alternatives or a sequence. Two hops to two parties are two `asks`.
- **`X returns stream [to Y] <phrase>`** — a reply that repeats (token streaming, SSE). The request stays an `asks`; the stream answers it and is drawn in a `loop stream` box.
- **`[SQL <tables>]`** — direct use of another party's tables. **`[ADDR <address>]`** — egress to an address with no operation id; quote it when it contains ` //` (`[ADDR "//cdn.example.com/x"]`). Write both on an `asks` (on an internal step or a return they get `craft/lint/egress-on-internal-step`); declare an external target as a `system` actor.
- **`[inferred …]`** — the hop was inferred from names or config, not read in code (drawn grey, "(inferred)"). `[inferred]` alone: hop inferred, operation unknown. Use it instead of "probably"/"NOT TRACED" comments.
- **`sites:` / `envs:` tags** scope the whole use case and are printed in the sequence title. Use `opt`/`alt` when only some steps differ by site.

**Evidence, links and journeys (v2.25).** Need craft ≥ 2.25.

```craft
use_case "Seller promotes an advert" {
    tags {
        journey: seller-promotes-an-advert
    }
    when Seller promotes an advert
        includes "Site session preamble"
        frontendplatform asks graphql to book the promotion  [scheduleVas]
        //@ re-graphql src/vas/schedule.ts:40-72 -- the mutation forwards the seller's session
        graphql asks vasscheduler to book it  [createSchedule]
        vasscheduler notifies com.olx.realestate_vas.VasScheduled
        continues "Seller pays at the checkout"
}
```

- **`//@ [repo] path[:lines] [-- text]`** under a step cites the code that proves it. Put it on the line after the step it proves (it attaches to the nearest step line above it; plain comments in between do not matter). Several cites on one line: `//@ a.go:5-9, b.go:12`; `:<lines>` alone reuses the previous path. Free text goes after ` -- `. The repo is optional (resolved from the step's service). Use `//@` instead of `// see path:line` prose; keep file-header cites as plain `//` comments (a `//@` outside a use case is `craft/lint/detached-source`). A `//@` line that does not parse is `craft/syntax/malformed-source` (it stays a comment). Never invent paths or line numbers: cite only what you have read.
- **`includes "<title>"`** — another use case runs here (a shared preamble or sub-flow), then this body goes on. **`continues "<title>"`** — the flow goes on in another use case and this body ends; nothing may follow it in the same body (`continues-not-last`). Both are steps that may sit in any branch. Use a link only for a hand-off with no event (a navigation, a redirect, a web view): an event hand-off is a `notifies`/`listens` pair and needs no link. The title must match a use case's title exactly (or one of its `formerly_titled: A; B` values, which gives `link-to-former-title`). An unknown title is `craft/sema/unresolved-use-case` (a warning: validate every file together); a title shared by two use cases is `craft/sema/ambiguous-use-case`.
- **`journey: <slug>`** puts the use case in a journey (one slug; a list is `journey-list`). `craft generate <files…> --journey <slug>` draws every member as one sequence (the spine), ordered by hand-offs: event and store pairs and `continues` links; a chain stays together, the use case named after the slug leads among equals, unconnected members come last. A member with no hand-off to the others gets a `craft/lint/journey-unconnected` hint: add the `notifies`/`listens` pair or a `continues` link, or move it to its own journey.

### Deprecated: Quoted Event Strings

The old free-text form — `notifies "Order Created"` / `when X listens "Order Created"` — still **parses**, for backward compatibility with existing files. But it is **deprecated**: `craft validate` emits a `craft/lint/deprecated-string-ref` warning on every quoted event string, pointing at the typed-ref form above. Always generate the typed-ref form in new or extended files; only leave quoted strings in place if the user's existing file already uses them and isn't being migrated.

---

## Modeling with Event-Driven Choreography

This is the most important section for producing high-quality `.craft` files. Craft's power comes from modeling **async choreography** — bounded contexts communicating through events rather than direct calls.

### When to use `notifies` / `listens` vs `asks`

- **`asks`** (synchronous) — use when one bounded context needs an immediate response from another to proceed, or when calling an external actor (e.g., `Payment asks PaymentGateway to process payment`). Also appropriate for intra-service calls between contexts that share a deployment unit.
- **`notifies` / `listens`** (asynchronous) — use when bounded contexts from **different services** or **different bounded contexts** communicate. This is the preferred pattern for cross-context communication because it decouples the bounded contexts and reflects how real distributed systems work.

### How to decompose a flow into scenarios

A common mistake is writing one big flat scenario where all actions happen sequentially. Instead, decompose flows into multiple `when` blocks connected by events:

**Bad — flat synchronous sequence spanning multiple bounded contexts:**
```
use_case "Send Notification" {
    when User triggers notification
        TemplateManagement selects template
        TemplateManagement renders content
        EmailDelivery asks EmailProvider to send email
        SMSDelivery sends SMS
        PushDelivery sends push notification
}
```

**Good — event-driven choreography with decoupled bounded contexts:**
```
use_case "Send Notification" {
    when User triggers notification
        TemplateManagement selects template
        TemplateManagement renders content
        TemplateManagement notifies templates.TemplateRendered

    when EmailDelivery listens templates.TemplateRendered
        EmailDelivery asks EmailProvider to deliver email
        EmailDelivery notifies email.EmailSent

    when SMSDelivery listens templates.TemplateRendered
        SMSDelivery delivers SMS via provider
        SMSDelivery notifies sms.SMSSent

    when PushDelivery listens templates.TemplateRendered
        PushDelivery delivers push notification
        PushDelivery notifies push.PushSent
}
```

The good version shows that each delivery channel reacts independently to the same event. This is more realistic (channels can fail independently, scale independently) and produces a richer bounded context model.

### Rules of thumb

1. **Every `notifies` should have at least one `listens`** somewhere in the file. An event with no listener is a dead event — either add the listener or remove the event.
2. **Cross-context actions should be async by default.** If bounded context A and bounded context B belong to different services, prefer `A notifies module.SomethingHappened` + `when B listens module.SomethingHappened` over `A asks B to do something`.
3. **Keep each scenario focused.** A scenario (one `when` block) should represent one bounded context's reaction to a trigger. When a scenario spans multiple bounded contexts, consider whether an intermediate event should split it.
4. **Name events in past tense.** Events describe something that already happened: `order.OrderCreated`, `payment.PaymentProcessed`, `auth.UserRegistered`. Not `order.CreateOrder` or `payment.ProcessPayment`.
5. **Use `asks` for external actors.** When a bounded context calls an external system (actor of type `system` or `service`), `asks` is appropriate: `Payment asks PaymentGateway to process payment`.
6. **Use typed event refs, not quoted strings.** `notifies order.OrderCreated`, not `notifies "Order Created"` — the quoted form is deprecated (see "Deprecated: Quoted Event Strings" above).

---

## File Conventions

**Location:** Default to `docs/` directory. Use kebab-case filenames: `docs/payment-system.craft`.

**Canonical ordering** for new files (existing files should keep their ordering):

1. `actors` — who interacts with the system
2. `domains` — business capability boundaries
3. `services` — deployable units with tech stack
4. `context_map` — DDD strategic relationships between bounded contexts
5. `glossary` — ubiquitous-language term relations across bounded contexts
6. `arch` — component topology
7. `exposure` — external access rules
8. `use_case` — behavioral scenarios

This flows from "who and what exists" to "how it's built" to "what happens."

**Granularity:** One file per system or major domain boundary. The examples show self-contained models per file (banking, e-commerce, user-management).

---

## Tooling

After generating or editing a `.craft` file, the user can run `craft` to validate and inspect it:

- `craft validate <file>` — parse and lint; reports errors and warnings, exits 1 on errors
- `craft generate <file>` — produce diagram files (PlantUML by default; `--format mermaid` or `--format mermaid-md` for Mermaid output, `--split` for one file per use case, `--use-case` to filter, `--journey <slug>|all` for one spine sequence per journey)
- `craft inspect <file>` — dump the parsed model as structured text or JSON

Glob patterns work for file arguments: `craft validate docs/**/*.craft`

If the user asks how to install craft or needs details on flags and options, read `references/cli-reference.md`.

---

## Common Mistakes

These are the most frequent syntax errors. Avoid them:

1. **Quoted event strings (deprecated)** — `notifies "Order Created"` / `listens "Order Created"` still parse but trigger a `craft/lint/deprecated-string-ref` warning. Use the typed ref form instead: `notifies order.OrderCreated`
2. **Malformed node slugs** — each `kind:` has a fixed namespace shape: `domain:re/<name>`, `bc:<domain>/<name>`, `term:<bc>/<name>`, `service:<alias>` (no namespace). Getting the segment count wrong (e.g. `bc:billing` with no name segment, or `service:re/billing` with a namespace) is a `craft/sema/malformed-slug` error
3. **Wrong property keyword** — It's `data-stores:` with a hyphen, not `datastores:` or `data_stores:`
4. **Wrong deployment arrow** — Use `->` not `=>`; e.g., `canary(20% -> staging)`
5. **Missing newlines between items** — Newlines separate actions, list items, and property definitions. Don't put multiple actions on one line
6. **Quoted service names only when needed** — Only quote service names that contain spaces: `"Order Service"`. Plain identifiers like `OrderService` don't need quotes
7. **Invalid identifiers** — Identifiers start with `[a-zA-Z0-9_]` and continue with `[a-zA-Z0-9_.-]`. No spaces, no special characters beyond underscore, hyphen, and dot
8. **Actions outside a when block** — Every action must be inside a scenario (under a `when` trigger). Orphaned actions are invalid
9. **Connector word confusion** — Connector words (`a`, `an`, `the`, `to`, `from`, etc.) are optional in phrases but they make the DSL read naturally. Use them for readability
10. **Trailing comma on last item of a multi-line list** — When splitting a list across lines, the final item must not have a trailing comma. `contexts: Foo, Bar,\n    Baz` is valid; `contexts: Foo, Bar,\n    Baz,\ndata-stores: db` is not
11. **`context_map` endpoint with a `bc:`/`service:`/`term:` prefix** — `context_map` endpoints are bare or domain-qualified bounded-context names only (`billing`, `re/billing`); there is no kind prefix inside a `context_map` block. An endpoint that resolves to a domain, service, or actor instead of a bounded context is a `craft/sema/edge-endpoint-not-bc` error
12. **Unspaced `//` inside prose is NOT a comment** — `http://api`, `50/50`, `and/maybe` stay as prose because there's no whitespace before the `//`/`/`. A comment needs a space (or line-start) before it: `Auth checks token  // TODO`
13. **Malformed `glossary` term node** — a term node is `<bc>/<term>` or `<domain>/<bc>/<term>`; the last `/`-segment is the term. A bare term with no BC (`Invoice same_as Bill`) is invalid, and `.` is never allowed in a term node (it's reserved for event refs). The glossary verbs are `same_as`, `contrasts`, `distinct_from` — all symmetric, so don't worry about which side is which
14. **`kind:` prefix in a `use_case` bounded-context slot**: the subject of any action (`asks`, `notifies`, `returns`, or an internal action), the `asks` target, the `returns` target, and the `when ... listens` trigger context are all bare-or-domain-qualified only, same rule as `context_map`. `Subscriptions asks bc:re/billing for a fresh charge attempt` is a `craft/syntax/kind-prefix-in-target` error; write `Subscriptions asks re/billing for a fresh charge attempt`
15. **Empty operation annotation**: `Subscriptions asks Billing for a fresh charge attempt []` is a `craft/syntax/empty-op-annotation` error, not a silently dropped bracket
16. **Ambiguous bare bounded-context name in a use case**: if a bare name like `Billing` is declared in two or more `domain` blocks, every `use_case` slot that resolves it (`asks` subject/target, `notifies` subject, `listens` trigger context) is a `craft/sema/ambiguous-bc` error. Qualify it as `<domain>/<name>`
17. **Malformed qualified bounded-context reference**: `re/ billing` (space in a segment) and `re//billing` (empty segment) and `re/a/b` (more than two segments) are all `craft/sema/malformed-slug` errors, in any bounded-context slot, not just `asks` targets

---

## Explaining & Analyzing .craft Files

When asked to explain a `.craft` file:

1. **Start with a high-level summary** — what system does this model? How many actors, domains, bounded contexts, services, use cases?
2. **Describe the bounded context model** — what are the bounded context boundaries? Which bounded contexts group together?
3. **Trace the event flows** — follow the `notifies` / `listens` chains across use cases. Which bounded contexts communicate asynchronously?
4. **Note architectural observations** — deployment strategies, data store choices, component flows

**Look for gaps and potential issues:**
- Events published (`notifies`) without any matching listener (`listens`) — dead events
- Actors defined but never used as triggers in use cases
- Domains referenced in use cases but not defined in any `domain` block
- Services without use cases — potentially undocumented behavior
- Asymmetric event flows — one-way communication that might need a response

Report gaps as observations, not errors — the `.craft` file may be intentionally incomplete or in progress.

---

## Self-Check Before Outputting

After generating or extending a `.craft` file, review your own output for these quality signals:

1. **Every `notifies` has a matching `listens`** — scan for published events and confirm each one has at least one listener scenario. If not, either add the listener or explain to the user that the event is a hook for future extension.
2. **Cross-contexts flows use async choreography** — if a scenario has actions spanning domains from different services, it should use `notifies`/`listens` to connect them, not a flat sequence of `asks`.
3. **Every defined actor appears in at least one use case trigger** — if you defined an actor, use it. If the user didn't specify use cases for an actor, note it.
4. **Every bounded context in a domain block is referenced** — either in a service's `contexts:` list, in a use case action, or both.
5. **Events follow past-tense naming** — `order.OrderCreated` not `order.CreateOrder`.
6. **Syntax is clean** — `data-stores:` (hyphenated), `contexts:`, events as typed refs (not quoted strings), valid identifiers/slugs, proper newlines between actions.
7. **No deprecated quoted event strings in new content** — `notifies`/`listens` should use typed refs (`order.OrderCreated`), not `notifies "Order Created"`, unless you're deliberately preserving an existing file's style.

---

## What This Skill Does NOT Do

- **Generate code** from `.craft` files — separate tooling handles that
- **Modify the grammar** — the grammar is defined by the hand-written parser in `internal/syntax` (and mirrored in `tree-sitter-craft/grammar.js` for editor support)
- **Make architecture decisions** — the DSL documents decisions; it doesn't prescribe them
- **Generate diagrams** — use `craft generate` or the VS Code extension for visualization

---

## Complete Example

This example demonstrates all constructs working together, including event-driven choreography across domain boundaries, a multi-line `contexts:` list, a `context_map` block, service anchors, a typed `asks` target, and unquoted flexible prose:

```craft
// Actors
actors {
    user Business_User
    system CronA
    service Database
}

// Domains
domain User {
    Authentication
    Profile
}

domain Communications {
    Notifier
}

// Services
services {
    UserService {
        contexts: Authentication,
            Profile
        data-stores: user_db
        language: golang
        deployment: rolling
        catalog_ref: user-service
        repo: acme/platform/user-service
    }
    CommsService {
        contexts: Notifier
        data-stores: email_queue
    }
}

// Context map: DDD strategic relationships between bounded contexts
context_map {
    Authentication open_host_service Notifier
}

// Glossary: how terms relate across bounded contexts
glossary {
    Authentication/Account same_as Profile/Account
}

// Architecture
arch {
    presentation:
        WebApp[framework:react, ssl]
        MobileApp

    gateway:
        LoadBalancer[ssl:true] > APIGateway[type:nginx]
}

// Exposure
exposure default {
    to: Business_User
    contexts: Authentication, Profile
    through: APIGateway
}

// Use cases
use_case "User Registration" {
    when Business_User creates Account
        Authentication validates email format
        Authentication asks Database to check email uniqueness (retry x3! & backoff)
        Authentication creates user credentials
        Authentication notifies auth.UserRegistered

    when Profile listens auth.UserRegistered
        Profile creates user profile
        Profile asks Database to store profile data
        Profile notifies profile.ProfileCreated

    when Notifier listens auth.UserRegistered
        Notifier sends welcome email
        Notifier notifies notifier.WelcomeEmailSent
}

use_case "Scheduled Cleanup" {
    when CronA triggers inactive account cleanup
        Authentication identifies inactive accounts
        Authentication asks Database to flag inactive users
        Authentication notifies auth.InactiveAccountsFlagged

    when Notifier listens auth.InactiveAccountsFlagged
        Notifier sends reactivation reminders
}
```

Notice how the "User Registration" use case uses event-driven choreography: `Authentication` publishes `auth.UserRegistered`, then both `Profile` and `Notifier` independently react to it in separate scenarios. This models real-world decoupling — the authentication bounded context doesn't need to know about profile creation or email sending. Note also the unquoted prose tail `(retry x3! & backoff)` on the `asks` step — special characters don't need quoting — and the `context_map` block declaring that `Authentication` is an open-host service for `Notifier` (LEFT `Authentication` is the upstream host, RIGHT `Notifier` is the downstream consumer). The `glossary` block records that the `Account` term means the same thing in `Authentication` as it does in `Profile` — a symmetric term relation, distinct from the strategic BC-to-BC relationship the `context_map` captures.
