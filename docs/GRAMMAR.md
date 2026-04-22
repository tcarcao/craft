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
            | actor_def ;
```

Every top-level declaration is recoverable independently: a parse failure inside one block MUST NOT prevent the parser from recognising the next top-level keyword (island parsing — see `ARCHITECTURE.md`).

**Top-level keywords (the re-sync anchor set for error recovery):**
`arch`, `services`, `service`, `exposure`, `use_case`, `domain`, `domains`, `actors`, `actor`.

---

## 3. Actors

```ebnf
actor_def        = 'actor' actor_type actor_name NL* ;
actors_def       = 'actors' '{' NL* actor_definition_list '}' NL* ;
actor_definition_list = actor_definition (NL+ actor_definition)* NL* ;
actor_definition = actor_type actor_name ;
actor_type       = 'user' | 'system' | 'service' ;
actor_name       = identifier ;
```

Examples:
```craft
actor user Customer
actor system PaymentGateway

actors {
  user Business_User
  system CRON
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
use_case     = 'use_case' STRING '{' NL* scenario* '}' NL* ;
scenario     = trigger action_block ;

trigger      = 'when' domain 'listens' quoted_event NL+
             | 'when' external_trigger             NL+
             | 'when' quoted_event                 NL+ ;

external_trigger = actor verb connector_word? phrase? ;

action_block = action* ;

action       = async_action    NL+
             | sync_action     NL+
             | return_action   NL+
             | internal_action NL+ ;

sync_action     = domain 'asks' domain connector_word phrase
                | domain 'asks' domain phrase ;
async_action    = domain 'notifies' quoted_event ;
internal_action = domain verb connector_word? phrase ;
return_action   = domain 'returns' 'to' domain connector_word? phrase
                | domain 'returns' connector_word? phrase ;

phrase       = (phrase_word | STRING)+ ;
phrase_word  = identifier
             | connector_word
             | 'when'
             | 'use_case' ;

actor  = identifier ;
domain = identifier ;
verb   = identifier ;

quoted_event = STRING ;
```

**Trigger disambiguation rule:** after `when`, the parser must look ahead to decide between the three trigger alternatives:
1. If the next token is a STRING → `'when' quoted_event NL+` (event-on-its-own).
2. Else if `identifier 'listens' STRING` matches → domain listener.
3. Else → external-trigger (actor verb connector? phrase?).

The second alternative shadows the first identifier in external-trigger when followed by `listens`. This is a **contextual** disambiguation; `listens` is not a reserved word (see §11).

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
