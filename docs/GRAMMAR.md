# Craft DSL — Grammar (v1)

> **Status:** Draft — P0.6 deliverable of the LSP migration (`docs/decisions/lsp-migration-plan.md`).
> **Scope:** Frozen at v1 (current ANTLR semantics) through v0.1 cut. Grammar v2 (reserved-word hardening, `returns to` pinning) is deferred — see `docs/decisions/grammar-v2-refactor-plan.md`.
> **Source of truth:** the acceptance corpus at `testdata/corpus/`. Where this document and the corpus disagree, **the corpus wins**. This doc is prose + EBNF documentation, not a parallel implementation.
> **Keyed to:** `tools/antlr-grammar/Craft.g4` (v1, 228 lines) as of 2026-04-22. Every production below has a 1:1 counterpart there.

---

## 1. Notation

Standard EBNF with these conventions:

| Notation | Meaning |
|----------|---------|
| `LOWER`  | Non-terminal (grammar production) |
| `UPPER`  | Terminal (lexer token) |
| `'literal'` | Character-exact literal |
| `a | b`  | Alternation |
| `a?`     | Optional |
| `a*`     | Zero-or-more |
| `a+`     | One-or-more |
| `(a b)`  | Grouping |
| `NL`     | `NEWLINE` token (`'\r'? '\n'`) |

Whitespace (`[ \t]+`) and line comments (`'//' ~[\r\n]*`) are skipped by the lexer. `NEWLINE` is significant and drives block/statement separation.

---

## 2. Top-level

```ebnf
file        = NL* top_decl* ;
top_decl    = arch
            | services_def
            | service_def
            | exposure
            | use_case
            | domain_def
            | domains_def
            | actors_def
            | actor_def
            | surface_decl ;      (* 2.23, §8.2 *)
```

Every top-level declaration is recoverable independently: a parse failure inside one block MUST NOT prevent the parser from recognising the next top-level keyword (island parsing — see `ARCHITECTURE.md`).

**Top-level keywords (the re-sync anchor set for error recovery):**
`arch`, `services`, `service`, `exposure`, `use_case`, `domain`, `domains`, `actors`, `actor`, and `surface` when a name and `{` follow on its line.

A use case, domain or surface whose `}` is missing also stops at a declaration that starts at column 1 (`surface <name> {`, `actor <kind> <name>`, a block keyword followed by `{`, or another top-level keyword), so it does not swallow the next declaration (`craft/syntax/unclosed-block`). An indented line is never a declaration.

---

## 3. Actors

```ebnf
actor_def        = 'actor' actor_type actor_name NL* ;
actors_def       = 'actors' '{' NL* actor_definition_list '}' NL* ;
actor_definition_list = actor_definition (NL+ actor_definition)* NL* ;
actor_definition = actor_type actor_name ;
actor_type       = identifier ;       (* known kinds: user, system, service, scheduler, tool, infra *)
actor_name       = identifier ;
```

The actor-type slot accepts any identifier (grammar ruling Q5: tooling decides the known set). Six kinds are known (2.24 added the last three):

| Kind | Meaning | Drawn as |
|---|---|---|
| `user` | a person | a person (`actor`) |
| `system` | an external system | `boundary` |
| `service` | a service outside the model | `control` |
| `scheduler` (2.24) | a clock that starts work: cron, an in-process ticker | `boundary … <<scheduler>>`; Mermaid `participant … (scheduler)`; C4 `System_Ext`, "Scheduler" |
| `tool` (2.24) | a CLI, script or client that a person runs; it, not the person, makes the calls | `boundary … <<tool>>`; Mermaid `participant … (tool)`; C4 `System_Ext`, "Operator tool" |
| `infra` (2.24) | platform machinery: pod start, probes, CI pipelines, synthetic monitors | `boundary … <<infra>>`; Mermaid `participant … (infra)`; C4 `System_Ext`, "Platform infrastructure" |

All kinds except `user` sit outside the system. Any other identifier stays legal, gets no diagnostic, and is drawn as a person.

Examples:
```craft
actor user Customer
actor system PaymentGateway

actors {
  user Business_User
  scheduler CRON
  tool OperatorTool
  infra Pipeline
  service OrderAPI
}
```

---

## 4. Domains

```ebnf
domain_def        = 'domain' domain_name '{' NL* bounded_context_list '}' NL* ;
domains_def       = 'domains' '{' NL* domain_block_list '}' NL* ;
domain_block_list = domain_block (NL+ domain_block)* NL* ;
domain_block      = domain_name '{' NL* bounded_context_list '}' ;
bounded_context_list = bounded_context (NL+ bounded_context)* NL* ;
domain_name       = identifier ;
bounded_context   = identifier ;
```

Examples:
```craft
domain Billing {
  Invoicing
  Payments
}

domains {
  Identity {
    Authentication
    Profile
  }
  Catalog {
    Listings
    Search
  }
}
```

---

## 5. Services

```ebnf
service_def        = 'service' service_name '{' NL* service_properties '}' NL* ;
services_def       = 'services' '{' NL* service_block_list? '}' NL* ;
service_block_list = service_block (NL+ service_block)* NL* ;
service_block      = service_name '{' NL* service_properties '}' NL* ;
service_name       = identifier | STRING ;
service_properties = service_property (NL+ service_property)* NL* ;

service_property = 'contexts'     ':' context_list
                 | 'data-stores'  ':' datastore_list
                 | 'language'     ':' identifier
                 | 'deployment'   ':' deployment_strategy ;

context_list      = context_ref     (',' NL* context_ref)*     ','? ;
datastore_list    = datastore       (',' NL* datastore)*       ','? ;
context_ref       = identifier ;
datastore         = identifier ;

deployment_strategy = deployment_type ('(' deployment_config ')')? ;
deployment_type     = 'canary' | 'blue_green' | 'rolling' ;
deployment_config   = deployment_rule (',' deployment_rule)* ;
deployment_rule     = PERCENTAGE '->' deployment_target ;
deployment_target   = identifier ;
```

---

## 6. Exposures

```ebnf
exposure            = 'exposure' exposure_name '{' NL+ exposure_properties '}' NL* ;
exposure_name       = identifier ;
exposure_properties = exposure_property (NL+ exposure_property)* NL+ ;

exposure_property = 'to'       ':' target_list
                  | 'contexts' ':' context_list
                  | 'through'  ':' gateway_list ;

target_list  = target  (',' NL* target)*  ','? ;
gateway_list = gateway (',' NL* gateway)* ','? ;
target       = identifier ;
gateway      = identifier ;
```

Every exposure MUST be followed by at least one newline before its first property (grammar rule `'{' NL+`). A single-line exposure is a syntax error.

---

## 7. Architecture blocks

```ebnf
arch           = 'arch' arch_name? '{' NL* arch_sections '}' NL* ;
arch_name      = identifier ;
arch_sections  = (presentation_section | gateway_section)+ ;

presentation_section = 'presentation' ':' NL* arch_component_list NL+ ;
gateway_section      = 'gateway'      ':' NL* arch_component_list NL+ ;

arch_component_list   = arch_component (NL+ arch_component)* ;
arch_component        = simple_component | component_flow ;
simple_component      = component_with_modifiers ;
component_flow        = component_chain ;
component_chain       = component_with_modifiers ('>' component_with_modifiers)* ;
component_with_modifiers = component_name component_modifiers? ;
component_name        = identifier ;

component_modifiers   = '[' modifier_list ']' ;
modifier_list         = modifier (',' modifier)* ;
modifier              = identifier (':' identifier)?  ;
```

At least one of `presentation:` / `gateway:` must appear. Order is not significant.

---

## 8. Use cases

This is the richest construct in the language and the primary source of parser complexity.

```ebnf
use_case      = 'use_case' STRING '{' NL* use_case_body '}' NL* ;
use_case_body = tags_block? (note_stmt NL+)* scenario* ;
scenario      = scenario_name? trigger step* ;   (* trigger includes 'when' and its NL+ *)
scenario_name = 'scenario' STRING NL+ ;
step          = action | block | note_stmt NL+ ;
block         = alt_block | opt_block | par_block | loop_block ;
alt_block     = 'alt' STRING NL+ body ('else' STRING NL+ body)* ;
opt_block     = 'opt' STRING NL+ body ;
par_block     = 'par' STRING NL+ body ('and' STRING NL+ body)* ;   (* no `and`: each body step is a branch *)
loop_block    = 'loop' STRING NL+ body ;
note_stmt     = ('note' | 'finding') STRING ;
body          = step+ ;        (* offside rule: every line deeper than its header's column *)

trigger      = 'when' domain_listen                NL+
             | 'when' receive_trigger              NL+
             | 'when' ('cron' | 'every') STRING    NL+
             | 'when' external_trigger             NL+
             | 'when' quoted_event                 NL+ ;

domain_listen   = name_slot 'listens' event_list (('cron' | 'every') STRING)? ;
event_list      = event_item (',' NL* event_item)* ;   (* 2.24; a continuation line is never a step *)
event_item      = quoted_event | ref ;
receive_trigger = name_slot 'receives' connector_word? phrase? op_annotation? ;

external_trigger = actor verb connector_word? phrase? ;

action_block = action* ;

action       = async_action    NL+
             | sync_action     NL+
             | return_action   NL+
             | internal_action NL+ ;

sync_action     = domain 'asks' domain connector_word phrase
                | domain 'asks' domain phrase ;
async_action    = domain 'notifies' ( event_item | 'one' 'of' event_list ) op_annotation? ;
internal_action = domain verb connector_word? phrase ;
return_action   = domain 'returns' 'stream'? ('to' domain)? connector_word? phrase? op_annotation? ;

phrase       = (phrase_word | STRING)+ ;
phrase_word  = identifier
             | connector_word
             | 'when'
             | 'use_case' ;

actor  = identifier ;
domain = identifier ;
verb   = identifier ;

quoted_event = STRING ;

name_slot     = identifier | identifier '/' identifier ;  (* a name, or the qualified <domain>/<name> form *)
ref           = identifier ;                              (* a bare event ref; IDENTIFIER admits `.`, e.g. store.sms_queue.inserted *)
op_annotation = '[' 'inferred'? (op_item (', ' op_item)*)? ']' ;  (* the last `[` on a line that ends in `]`; tokens after its first `]` are unexpected *)
op_item       = op_verb? payload ;                        (* 2.24: op_verb includes ADDR and SQL *)
op_verb       = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS'
              | 'GRPC' | 'TOPIC' | 'QUERY' | 'ADDR' | 'SQL' ;
name_list     = name_slot (',' name_slot)* ;
tags_block    = 'tags' '{' NL* (identifier ':' tag_value NL+)* '}' NL+ ;
```

**Offside rule.** A block header's body is every following line whose first token sits in a column greater than the header's. The body ends at the first line at or left of the header's column, or at `when`, `scenario` or `}`. An `else` must start at exactly the column of its `alt`. Blocks nest to any depth. For the comparison a tab advances to the next multiple of 4, and comments are trivia: a comment line never closes a body. Outside a block body, indentation stays insignificant.

**Contextual keywords.** `scenario`, `alt`, `opt` and `else` are keywords only at the start of a line and only when a STRING follows. A line such as `alt asks billing for a quote` is still an ordinary action whose subject is named `alt`. A header with no label is an error (`craft/syntax/missing-label`) and is parsed as a header with an empty label.

**`par`, `and` and `loop` (2.23).** `par "<label>"` opens a concurrent block, and `and "<label>"` adds a branch to the nearest open `par` whose header is at the same column, as `else` does for an `alt`. A `par` with no `and` is the **fan-out form**: each step of its body (an action or a nested block) runs as its own concurrent branch, and every branch carries the header's label. Notes do not count as steps: a note joins the branch of the step before it. `loop "<condition>"` opens a block with one branch, like `opt`; diagrams draw one iteration. An `else` cannot continue a `par` or a `loop` (`craft/syntax/orphan-else`), and an `and` cannot continue an `alt`, `opt` or `loop` (`craft/syntax/orphan-and`).

**Notes and findings (2.23).** `note "…"` and `finding "…"` are one-line statements: the keyword and one STRING. A note is a step anywhere a step may stand (it obeys the offside rule, and a branch whose only step is a note is not empty), a use-case-level line after `tags { }` and before the first `scenario`/`when`, or a line in a surface body. A note never opens or closes a block, gets no interaction and does not count as an action. A `finding` is a note that records a defect or risk; diagrams draw it highlighted.

**`receives` (2.23).** `when <ctx> receives <phrase> [op]` is a callee-side trigger: it names the boundary that receives a request whose caller is not observed. The subject takes the qualified `<domain>/<name>` form. The operation bracket is optional. Sequence diagrams draw a found message (and a lost message for an unaddressed `returns`).

**Listens schedule (2.23).** A `listens` trigger takes an optional trailing schedule, `when smsdispatch listens store.sms_queue.inserted cron "* 10 * * 1-5"` or `… every "5m"`, with the same rules as `when cron` / `when every` (`craft/sema/invalid-schedule`).

**Store refs (2.23, no new grammar).** A bare (unquoted) ref whose first segment is `store` is a **store hand-off**: `notifies store.<name>.<change>` writes to a data store and `when <ctx> listens store.<name>.<change>` polls it. When the last segment is `inserted`, `updated` or `deleted` it is the change, and the segments before it (joined with `.`) are the store name; otherwise every segment after `store.` is the name and the change is empty (`craft/lint/store-without-change`). A ref that names no store (`store.inserted`, `store.`) is a plain event (`craft/lint/store-without-name`). The name resolves to a `data-stores:` entry when one matches; the views draw a database participant either way.

**`listens` lists (2.24).** A `listens` trigger takes a comma list of events: `when seshat listens sns.mercury-advert-state-changed, sns.mercury-cooked`. The list means "any of these events starts this scenario"; the body is the same for each. A trailing comma continues the list on the next line, as tag lists do, and the parser consumes the continuation line inside the trigger, so it is never a step and the offside rule does not see it. Comments between continuation lines are trivia. A listens schedule goes after the last item. A comma with no item after it is `craft/syntax/unexpected-token`. A store ref may not stand in a list of two or more items (`craft/sema/store-ref-in-event-list`): a store hand-off names one store, so a poller of two stores is two scenarios. The model keeps the list in `Trigger.Events` (`Trigger.EventList()`); `Event` and `Ref` stay the first item.

**`notifies one of` (2.24).** `X notifies one of a.A, b.B` publishes exactly one of the listed events per run, chosen at run time (an event type read from data, a routing table). The list wraps like a `listens` list. `one` is a keyword only directly after `notifies` and followed by `of` and an event item, so `notifies one` stays an event named `one`. A bare list without `one of` (`notifies a.A, b.B`) is `craft/syntax/unexpected-token`: to publish several events, write one `notifies` per event. A `notifies` event is one name: words after it (`sync notifies the owners team`) are `craft/syntax/unexpected-token` at the first of them. The model keeps the list in `Action.Events` (`Action.EventList()`).

**`returns stream` (2.24).** `X returns stream [to Y] <phrase>` is a reply that repeats until it ends: the request stays an ordinary `asks`, and its answer is a `returns stream`. `stream` is a keyword only directly after `returns`. `Action.Stream` is true, and `Action.Answers` holds the ID of the `asks` the stream answers: the open request on the call stack (the frame an ordinary return pops), or, with `to Y`, the most recent open request from `Y` to the streamer, without popping it. It is empty when the stream answers the trigger's actor, the unobserved caller or nobody (`craft/lint/stream-without-caller` reports the last case).

**Operation lists (2.24).** The body of an operation bracket splits into items at a comma that is followed by whitespace, at brace depth 0, outside a STRING: `[CreateApiKey, CreateUsagePlanKey]` is two items, and `[GET /nearest/paths/{latitude},{longitude}]` stays one (no space after the comma). Each item is read like a whole bracket: an optional protocol verb, then the payload. The items are one hop that uses several operations; craft does not say whether they are alternatives or a sequence, so say which in the phrase. Two hops to two parties are two `asks`. The model keeps every item in `Action.Operations` (`Action.OperationList()`); `Action.Operation` keeps the whole bracket, byte-identical to 2.23. An empty item (`[a, , b]`, `[a, ]`) is `craft/syntax/empty-op-annotation`.

**`ADDR`, `SQL` and `inferred` (2.24).** `[SQL <table> …]` is a direct read or write of another party's tables, with no API between; the payload is the table names, separated by spaces (`Operation.Tables()`). `[ADDR <address>]` is egress to an address that has no operation id: a URL, a host, or the name of the setting that holds it; `Operation.Payload` holds it without quotes. Both are protocol verbs, so either can be one item of a list, and both are errors without a payload (`craft/syntax/empty-op-annotation`). `inferred` as the first word of a bracket marks the whole hop as inferred from names, configuration or convention, not read in code (`Action.Inferred`); it is not part of `Operation.Text`, and `[inferred]` alone leaves `Action.Operation` nil. `inferred` has no meaning in a `receives` trigger or a surface line, where it is an ordinary payload word.

**Quote an address that starts with `//`.** A `//` opens a comment only after whitespace, so `[ADDR https://s3.example.com/x]` parses whole. `[ADDR //cdn.example.com/x]` does not: the comment cuts the bracket, and `craft/lint/unquoted-address` reports it. Write `[ADDR "//cdn.example.com/x"]`; quoting is always allowed.

**Trigger disambiguation rule:** after `when`, the parser must look ahead to decide between the three trigger alternatives:
1. If the next token is a STRING → `'when' quoted_event NL+` (event-on-its-own).
2. Else if `identifier 'listens' STRING` matches → domain listener.
3. Else → external-trigger (actor verb connector? phrase?).

The second alternative shadows the first identifier in external-trigger when followed by `listens`. In the same slot, `receives` selects `receive_trigger` (2.23). This is a **contextual** disambiguation; neither `listens` nor `receives` is a reserved word (see §11). `when X receives …` used to be an external trigger with actor `X` and verb `receives`.

### 8.2 Surfaces (2.23)

A surface is the operation inventory of one boundary: a bounded context, a service, or an actor that stands for an external system.

```ebnf
surface_decl  = 'surface' name_slot '{' NL* (surface_item NL+)* '}' ;
surface_item  = 'callers' ':' name_list           (* actors, contexts, services, or `unknown` *)
              | 'spec' ':' STRING
              | note_stmt
              | ('query' | 'command') connector_word? phrase op_annotation ;
```

```craft
surface concierge {
    callers: Advisor
    spec: "obidozoe/docs/openapi.yaml"
    note "every route takes the shared HTTP Basic pair plus a bidder identity header"
    query the seeker list, unpaged, newest first, filtered by a typed status  [getUserList]
    command delete a seeker's account                                        [deleteAccount]
}
```

A `query` has no side effect and a `command` changes state. The operation bracket is required (`craft/syntax/missing-operation`), and so is the phrase before it (`craft/syntax/missing-phrase`). `callers: unknown` says that no caller was observed. `spec:` is kept verbatim; Craft does not read it. Properties keep their source order. `craft generate --type surface` writes the surfaces as a Markdown table with a computed "used by" column.

---

## 9. Common productions

```ebnf
connector_word = 'a' | 'an' | 'the' | 'as'
               | 'to' | 'from' | 'in' | 'on' | 'at'
               | 'for' | 'with' | 'by' ;

identifier = IDENTIFIER
           | 'actor' | 'user' | 'system' | 'service'
           | 'arch' | 'presentation' | 'gateway'
           | 'domain' | 'contexts' | 'actors'
           | 'exposure' | 'to' | 'of' | 'through'
           | 'services'
           | 'canary' | 'blue_green' | 'rolling'
           | 'listens' | 'asks' | 'notifies' | 'returns'
           | 'a' | 'an' | 'the' | 'as'
           | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by'
           | CONTEXTS | DATA_STORES | LANGUAGE | DEPLOYMENT ;
```

The identifier rule is deliberately permissive: nearly every keyword that only has meaning in a specific structural position can also appear as a plain identifier elsewhere. See §11.

---

## 10. Lexer tokens

```ebnf
CONTEXTS    = 'contexts' ;
DATA_STORES = 'data-stores' ;
LANGUAGE    = 'language' ;
DEPLOYMENT  = 'deployment' ;

PERCENTAGE  = [0-9]+ '%' ;
IDENTIFIER  = [a-zA-Z0-9_] [a-zA-Z0-9_.-]* ;
STRING      = '"' ~["\r\n]* '"' ;
NEWLINE     = '\r'? '\n' ;

WS          = [ \t]+       (* skipped *) ;
COMMENT     = '//' ~[\r\n]* (* skipped *) ;
```

Notes:
- `IDENTIFIER` may start with a digit (e.g. `3rd_party_api` is valid). This is a v1 quirk preserved by the hand-written lexer; grammar v2 may reconsider.
- `IDENTIFIER` admits `.` and `-` in non-leading positions (e.g. `auth.v2`, `user-service`).
- `STRING` does not support escape sequences and does not cross newlines.
- Line comments only; there is no block-comment form.

---

## 11. Contextual keywords (**the v1 checklist that grammar v2 will consume**)

A **contextual keyword** is a token that the grammar treats as a keyword in some positions and as a plain identifier in others. v1 leans heavily on contextual keywords; v2 will harden many of these into truly reserved words (see `docs/decisions/grammar-v2-refactor-plan.md`). Every entry below is a concrete spot the hand-written parser must disambiguate by position.

### 11.1 Structural keywords that can also be identifiers

All of the following appear in the `identifier` production (§9) and can therefore be used as, e.g., an actor name, context name, or verb:

| Keyword | Structural role | Appears as identifier in |
|---------|-----------------|--------------------------|
| `actor` / `actors` | Actor-block keyword | Any `identifier` position |
| `user` / `system` / `service` | Actor-type token | Any `identifier` position |
| `arch` | Architecture-block keyword | Any `identifier` position |
| `presentation` / `gateway` | Arch-section label | Any `identifier` position |
| `domain` / `domains` | Domain-block keyword | Any `identifier` position |
| `contexts` | Service property / exposure property | Any `identifier` position |
| `services` | Service-block keyword | Any `identifier` position |
| `exposure` | Exposure keyword | Any `identifier` position |
| `to` | Exposure property / return target / ask connector | Any `identifier` position |
| `of` | Legacy exposure property (not emitted by grammar, reserved in identifier set for forward-compat) | Any `identifier` position |
| `through` | Exposure property | Any `identifier` position |
| `canary` / `blue_green` / `rolling` | Deployment strategy | Any `identifier` position |
| `listens` | Use-case trigger keyword | Any `identifier` position (incl. verb, phrase word) |
| `asks` | Sync-action connector | Any `identifier` position |
| `notifies` | Async-action keyword | Any `identifier` position |
| `returns` | Return-action keyword | Any `identifier` position |
| `data-stores` | Service property | Any `identifier` position (note the hyphen — special lexing) |
| `language` | Service property | Any `identifier` position |
| `deployment` | Service property | Any `identifier` position |

`scenario`, `alt`, `opt` and `else` are also contextual (see the offside rule in §8). They remain identifiers: each is recognised as a keyword only at the start of a use-case-body line and only when followed by a STRING, and an action always needs a verb after its subject.

The 2.23 words are contextual in the same way:

| Word | Is a keyword only when | Otherwise |
|---|---|---|
| `par`, `and`, `loop` | at the start of a step line and followed by a STRING, or alone on the line (`missing-label`) | an identifier (subject, verb, phrase word) |
| `note`, `finding` | at the start of a step line, at use-case level before the first scenario, or in a surface body; followed by a STRING, or alone on the line | an identifier |
| `receives` | in a trigger, as the second token after the subject (the slot where `listens` is recognised) | an identifier |
| `cron`, `every` (listens tail) | directly after the event ref of a `listens` trigger, on the same line, followed by a STRING | an identifier |
| `surface` | at top level, followed by a name and `{` on the same line | an identifier |
| `callers`, `spec` | in a surface body, followed by `:` | an identifier |
| `query`, `command` | at the start of a surface body line | an identifier |

Only `receives` changes the meaning of input that parsed before; every other form was a syntax error before 2.23.

The 2.24 words:

| Word | Is a keyword only when | Otherwise |
|---|---|---|
| `,` in a `listens` slot | directly after an event item of a `listens` trigger | (was an error) |
| `one`, `of` | `one` directly after `notifies`, followed by `of` and an event item | an identifier / a connector word |
| `stream` | directly after `returns` | a phrase word |
| `inferred` | the first word inside an operation bracket | a payload word |
| `ADDR`, `SQL` | the first word of a bracket item (protocol verbs are matched this way, exact and case-sensitive) | a payload word |
| `scheduler`, `tool`, `infra` | never keywords: the actor-type slot accepts any identifier (Q5) | an identifier |

Three of these change the meaning of input that parsed before, with no known uses: `X returns stream …` was a return whose phrase starts with "stream"; `[inferred …]` was an operation whose payload starts with "inferred"; `[ADDR …]` and `[SQL …]` were operations with an empty verb and the word in the payload. `notifies one of a.B, c.D` used to parse as the event `one` plus a second, internal step `of a.B , c.D`. The `listens` list was a syntax error.

### 11.2 Connector words (part of `phrase_word`)

Connector words appear verbatim in sync-action and external-trigger phrases; they are both `connector_word` and `identifier`.

`a`, `an`, `the`, `as`, `to`, `from`, `in`, `on`, `at`, `for`, `with`, `by`.

### 11.3 Truly reserved words in v1

Only two words cannot appear as an `identifier` in v1:
- `use_case`
- `when`

These are never allowed as an identifier or phrase word outside their structural role. Note, however, that `phrase_word` explicitly permits `'when'` and `'use_case'` — so they **can** appear inside action phrases, just not as actor / domain / service / context names.

### 11.4 Disambiguation strategy (required behaviour of the hand-written parser)

| Position | Rule |
|----------|------|
| After top-level `NL*` | Peek one token. One of the nine top-level keywords (§2) enters its production; otherwise emit `craft/syntax/unexpected-token` and run island re-sync. |
| Inside service property | `contexts` / `data-stores` / `language` / `deployment` are keywords only when followed by `:` at the property position. Anywhere else in the same block body (e.g. as a context_ref name), they are identifiers. |
| Inside exposure property | `to` / `contexts` / `through` are keywords only when followed by `:` at the property position. |
| After `when` | See trigger-disambiguation rule in §8. `listens` is a keyword only when it appears as the second identifier of the trigger; elsewhere in the same trigger it is a plain identifier. |
| Inside action clauses | `asks`, `notifies`, `returns` are keywords only in the positions `domain KEYWORD ...`. If the same token appears as a `verb` or inside a `phrase`, it is an identifier. |
| `returns to` two-token form | In v1 the form is `'returns' 'to' domain ...`. `to` here is matched as a literal string, not as the `to:` exposure keyword. Grammar v2 will pin this (`returns to <target>`) as a single structural phrase. |
| Inside `phrase_word` | All connector words plus `when` and `use_case` are valid phrase words. The parser stops phrase consumption only at NEWLINE. |

### 11.5 Watch-outs for the hand-written lexer

- **`data-stores`** contains a hyphen. The lexer must prefer the longest match: when it sees `d-a-t-a-'-'-s-t-o-r-e-s` it emits `DATA_STORES`, not `IDENTIFIER('data') '-' IDENTIFIER('stores')`. Equivalent care is needed if user-code declares an identifier starting with `data` that is not `data-stores`.
- **`blue_green`** is a single identifier. Because `IDENTIFIER` permits `_`, it naturally lexes as one token; no special handling needed.
- **Percentages** (`PERCENTAGE`) collide with the prefix of an `IDENTIFIER` that starts with a digit. v1 grammar resolves by context (percentages only appear inside `deployment(...)`). The hand-written lexer should emit `PERCENTAGE` whenever the numeric prefix is followed immediately by `%` and let the parser reject mis-placements.
- **Comments** can appear anywhere after a token on the same line. The lexer discards them before they reach the parser; position information for diagnostics must point to the source line, not to the comment-stripped stream.

---

## 12. Known v1 quirks preserved as "spec"

These are observed ANTLR behaviours that the hand-written parser MUST match (because they are already in shipped `.craft` files) but which are candidates for cleanup in v2:

1. **Trailing-newline permissiveness.** Many productions require `NL+` but accept `NL*` in practice because the lexer compresses runs of blank lines. Corpus goldens capture the current behaviour; any divergence is a bug.
2. **Leading-digit identifiers.** `3rd_party` parses. Not idiomatic but shipping.
3. **Identifier-as-verb.** Any `identifier` (per §9) may appear in a `verb` position. A common surprise is `Order returns a draft Order` — `returns` is a keyword here because `returns <connector>? <phrase>` is a return_action production, not an internal_action. The parser tries `return_action` before `internal_action`.
4. **Quoted strings with embedded `"`.** Not supported; v1 `STRING` rule is `~["\r\n]*`. Corpus MUST NOT contain escaped quotes; goldens assume their absence.
5. **Case sensitivity.** All keywords are strictly lowercase; `Actor` (capital A) is an identifier, not the `actor` keyword. Case collisions are encouraged for natural-English domain names.

---

## 13. Changes from tree-sitter `grammar.js`

The tree-sitter grammar at `../tree-sitter-craft/grammar.js` (481 lines) covers the same language but at a different fidelity. Where the two disagree, the hand-written parser follows the ANTLR `.g4` / this document / the corpus — tree-sitter is an editor-highlighting grammar and is permitted extra permissiveness (see `docs/decisions/lsp-migration-plan.md` Q7c, "highlighting sufficiency"). Notable current divergences are tracked in the corpus per-file README files under `testdata/corpus/*/README.md`.

---

## 14. Cross-references

- **ANTLR grammar (authoritative for v1):** `tools/antlr-grammar/Craft.g4`
- **Acceptance corpus (spec by example):** `testdata/corpus/` *(to be seeded in S1)*
- **Broken-input spec:** `testdata/broken/` *(to be hand-authored in S9 per `lsp-migration-plan.md` P0.5)*
- **Canonical AST contract:** `pkg/craft/craftdoc.go` *(to be defined in S1)*
- **Architecture rules:** `docs/ARCHITECTURE.md`
- **Agent operating rules:** `docs/AGENT.md`
- **Grammar v2 plan (deferred):** `docs/decisions/grammar-v2-refactor-plan.md`
