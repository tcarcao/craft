---
name: craft-dsl
description: "Generate, extend, explain, and analyze Craft DSL (.craft) files — a domain-specific language for modeling DDD architectures with actors, domains, services, use cases, architecture blocks, and exposures. Use this skill whenever the user works with .craft files, mentions the Craft DSL, wants to model a domain or system in Craft, asks to add or modify use cases, services, domains, actors, exposures, or arch blocks in a .craft file, or wants to understand an existing .craft file. Also trigger when the user wants to document domain-driven design artifacts and there are .craft files in the project."
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
                         | <service_def> | <services_def> | <arch> | <exposure>
                         | <use_case>)*

// --- Actors: who interacts with the system ---
<actor_def>          ::= "actor" <actor_type> <identifier>
<actors_def>         ::= "actors" "{" (<actor_type> <identifier>)+ "}"
<actor_type>         ::= "user" | "system" | "service"

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
<deployment_strategy>::= ("canary" | "blue_green" | "rolling")
                         ("(" <deployment_rule> ("," <deployment_rule>)* ")")?
<deployment_rule>    ::= <percentage> "->" <identifier>

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
<trigger>            ::= "when" <identifier> "listens" <string>
                       | "when" <identifier> <verb> <phrase>?
                       | "when" <string>
<action>             ::= <sync_action> | <async_action> | <internal_action> | <return_action>
<sync_action>        ::= <identifier> "asks" <identifier> <connector>? <phrase>
<async_action>       ::= <identifier> "notifies" <string>
<internal_action>    ::= <identifier> <verb> <connector>? <phrase>
<return_action>      ::= <identifier> "returns" ("to" <identifier>)? <connector>? <phrase>

// --- Terminals ---
<connector>          ::= "a" | "an" | "the" | "as" | "to" | "from" | "in" | "on"
                       | "at" | "for" | "with" | "by"
<phrase>             ::= (<identifier> | <connector> | <string>)+
<verb>               ::= <identifier>
<identifier>         ::= [a-zA-Z0-9_][a-zA-Z0-9_.-]*
<string>             ::= '"' [^"\r\n]* '"'
<percentage>         ::= [0-9]+ "%"
// Continuation lines allowed: end intermediate lines with a comma, no trailing comma on last item
<identifier_list>    ::= <identifier> ("," <identifier>)* ","?
```

**Key grammar notes:**
- Newlines are significant — they separate list items, actions, and blocks (not just whitespace)
- Comments use `//` (single-line)
- Whitespace (spaces, tabs) is ignored; newlines are not
- **Multi-line lists are supported** — end each intermediate line with a comma and continue on the next line. The last item must not have a trailing comma when followed by another property

---

## Construct Reference

### Actors

Actors represent entities that interact with the system. Three types:
- `user` — human users (e.g., Customer, Admin, Business_User)
- `system` — external systems (e.g., CronA, PaymentGateway)
- `service` — internal services from other bounded contexts (e.g., Database, NotificationService)

Single: `actor user Customer`
Bulk:
```
actors {
    user Business_User
    system CronA
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
}
```

Service names with spaces must be quoted: `"Order Service"`.

For a single standalone service outside a `services` block, use `service UserService { ... }` instead of wrapping in `services { }`.

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
| Event | `when "<event>"` | `when "Order Placed"` |
| Bounded context listener | `when <context> listens "<event>"` | `when Payment listens "Order Created"` |

**Action types:**

| Type | Keyword | Syntax | Meaning |
|------|---------|--------|---------|
| Synchronous | `asks` | `Order asks Inventory to reserve items` | Bounded context-to-bounded context call |
| Asynchronous | `notifies` | `Order notifies "Order Created"` | Publish event |
| Internal | any verb | `Order validates items` | Bounded context-internal operation |
| Return | `returns` | `Database returns to Auth the user record` | Return response |

**Event-driven pattern:** Bounded contexts publish events with `notifies`, and other scenarios react with `when <context> listens "<event>"`. This models async choreography — the heart of good Craft modeling.

```
use_case "Purchase Item" {
    when Customer adds item to cart
        Cart validates item availability
        Cart asks Inventory to reserve items
        Cart notifies "Item Added to Cart"

    when Checkout listens "Item Added to Cart"
        Checkout asks Payment to process payment
        Payment notifies "Payment Processed"

    when Fulfilment listens "Payment Processed"
        Fulfilment asks Warehouse to ship order
        Fulfilment notifies "Order Shipped"
}
```

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
        TemplateManagement notifies "Template Rendered"

    when EmailDelivery listens "Template Rendered"
        EmailDelivery asks EmailProvider to deliver email
        EmailDelivery notifies "Email Sent"

    when SMSDelivery listens "Template Rendered"
        SMSDelivery delivers SMS via provider
        SMSDelivery notifies "SMS Sent"

    when PushDelivery listens "Template Rendered"
        PushDelivery delivers push notification
        PushDelivery notifies "Push Sent"
}
```

The good version shows that each delivery channel reacts independently to the same event. This is more realistic (channels can fail independently, scale independently) and produces a richer bounded context model.

### Rules of thumb

1. **Every `notifies` should have at least one `listens`** somewhere in the file. An event with no listener is a dead event — either add the listener or remove the event.
2. **Cross-context actions should be async by default.** If bounded context A and bounded context B belong to different services, prefer `A notifies "Something Happened"` + `when B listens "Something Happened"` over `A asks B to do something`.
3. **Keep each scenario focused.** A scenario (one `when` block) should represent one bounded context's reaction to a trigger. When a scenario spans multiple bounded contexts, consider whether an intermediate event should split it.
4. **Name events in past tense.** Events describe something that already happened: `"Order Created"`, `"Payment Processed"`, `"User Registered"`. Not `"Create Order"` or `"Process Payment"`.
5. **Use `asks` for external actors.** When a bounded context calls an external system (actor of type `system` or `service`), `asks` is appropriate: `Payment asks PaymentGateway to process payment`.

---

## File Conventions

**Location:** Default to `docs/` directory. Use kebab-case filenames: `docs/payment-system.craft`.

**Canonical ordering** for new files (existing files should keep their ordering):

1. `actors` — who interacts with the system
2. `domains` — business capability boundaries
3. `services` — deployable units with tech stack
4. `arch` — component topology
5. `exposure` — external access rules
6. `use_case` — behavioral scenarios

This flows from "who and what exists" to "how it's built" to "what happens."

**Granularity:** One file per system or major domain boundary. The examples show self-contained models per file (banking, e-commerce, user-management).

---

## Tooling

After generating or editing a `.craft` file, the user can run `craft` to validate and inspect it:

- `craft validate <file>` — parse and lint; reports errors and warnings, exits 1 on errors
- `craft generate <file>` — produce PlantUML diagram files
- `craft inspect <file>` — dump the parsed model as structured text or JSON

Glob patterns work for file arguments: `craft validate docs/**/*.craft`

If the user asks how to install craft or needs details on flags and options, read `references/cli-reference.md`.

---

## Common Mistakes

These are the most frequent syntax errors. Avoid them:

1. **Unquoted event names** — Events must be double-quoted strings: `notifies "Order Created"`, not `notifies Order Created`
2. **Wrong property keyword** — It's `data-stores:` with a hyphen, not `datastores:` or `data_stores:`
3. **Wrong deployment arrow** — Use `->` not `=>`; e.g., `canary(20% -> staging)`
4. **Missing newlines between items** — Newlines separate actions, list items, and property definitions. Don't put multiple actions on one line
5. **Quoted service names only when needed** — Only quote service names that contain spaces: `"Order Service"`. Plain identifiers like `OrderService` don't need quotes
6. **Invalid identifiers** — Identifiers start with `[a-zA-Z0-9_]` and continue with `[a-zA-Z0-9_.-]`. No spaces, no special characters beyond underscore, hyphen, and dot
7. **Actions outside a when block** — Every action must be inside a scenario (under a `when` trigger). Orphaned actions are invalid
8. **Connector word confusion** — Connector words (`a`, `an`, `the`, `to`, `from`, etc.) are optional in phrases but they make the DSL read naturally. Use them for readability
9. **Trailing comma on last item of a multi-line list** — When splitting a list across lines, the final item must not have a trailing comma. `contexts: Foo, Bar,\n    Baz` is valid; `contexts: Foo, Bar,\n    Baz,\ndata-stores: db` is not

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
5. **Events follow past-tense naming** — `"Order Created"` not `"Create Order"`.
6. **Syntax is clean** — `data-stores:` (hyphenated), `contexts:`, event names double-quoted, valid identifiers, proper newlines between actions.

---

## What This Skill Does NOT Do

- **Generate code** from `.craft` files — separate tooling handles that
- **Modify the grammar** — the grammar is defined in `Craft.g4` and `tree-sitter-craft/grammar.js`
- **Make architecture decisions** — the DSL documents decisions; it doesn't prescribe them
- **Generate diagrams** — use `craft generate` or the VS Code extension for visualization

---

## Complete Example

This example demonstrates all constructs working together, including event-driven choreography across domain boundaries and a multi-line `contexts:` list:

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
    }
    CommsService {
        contexts: Notifier
        data-stores: email_queue
    }
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
        Authentication asks Database to check email uniqueness
        Authentication creates user credentials
        Authentication notifies "User Registered"

    when Profile listens "User Registered"
        Profile creates user profile
        Profile asks Database to store profile data
        Profile notifies "Profile Created"

    when Notifier listens "User Registered"
        Notifier sends welcome email
        Notifier notifies "Welcome Email Sent"
}

use_case "Scheduled Cleanup" {
    when CronA triggers inactive account cleanup
        Authentication identifies inactive accounts
        Authentication asks Database to flag inactive users
        Authentication notifies "Inactive Accounts Flagged"

    when Notifier listens "Inactive Accounts Flagged"
        Notifier sends reactivation reminders
}
```

Notice how the "User Registration" use case uses event-driven choreography: `Authentication` publishes `"User Registered"`, then both `Profile` and `Notifier` independently react to it in separate scenarios. This models real-world decoupling — the authentication bounded context doesn't need to know about profile creation or email sending.
