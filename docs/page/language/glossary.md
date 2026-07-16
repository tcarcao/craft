# Glossary

`glossary` is craft's **ubiquitous-language view**: a set of statements that relate terms across bounded contexts — the same word, or related words, meaning the same thing, a related-but-different thing, or a deliberately distinct thing depending on which context you're standing in.

## Distinct from the context map

[`context_map`](/language/context-map) classifies **bounded-context-to-bounded-context** relationships using DDD strategic patterns (customer/supplier, conformist, open-host service, and so on). `glossary` classifies **term-to-term** relationships instead — it doesn't say anything about how two bounded contexts integrate, only about how a word used in one context relates to a word used in another.

An `Invoice` in `billing` is not the `Invoice` in `subscriptions` — they may share a name but mean different things, or they may genuinely be the same concept translated across a context boundary. That's a modeling judgment a human makes; `glossary` is where craft records it.

## Basic Syntax

```craft
glossary {
  billing/Invoice        same_as        subscriptions/Invoice
  ordering/order         distinct_from  offering/order
}
```

**Syntax:**
```craft
glossary <domain>? {
  <term_node> <verb> <term_node>
}
```

- The `glossary` block is **repeatable** — declare as many as you like across a file or workspace, and every relation merges into one glossary.
- The optional `<domain>` scopes bare term nodes to that domain, e.g. `glossary re { ... }`. An unscoped `glossary { }` is the shared/global glossary.
- Each statement is **one relation per line** — `<term_node> <verb> <term_node>`.

## Term Node Identity

A term node is `<bc>/<term>` or `<domain>/<bc>/<term>` — the **last** `/`-separated segment is always the term name, and everything before it identifies the bounded context (bare `bc`, or `domain/bc`):

```craft
billing/Invoice              // bc/term — resolves within the block's domain scope (or globally if unscoped)
re/billing/Invoice           // domain/bc/term — crosses a domain boundary, or disambiguates a colliding BC name
```

- `/` (slash) is the node-identity separator, one level deeper than a `context_map` BC endpoint — a term node is a BC reference plus a trailing term segment.
- `.` (dot) stays reserved for event refs (`vas.VasApplied`) — it must never appear in a `glossary` term node.
- A bare term with no BC segment (just `Invoice`) is invalid — every term node must identify a bounded context.

Craft has no term-*declaration* concept: terms are never declared, only referenced inside relation statements. `glossary` records relations between terms, not definitions of them.

## The Three Verbs

All three verbs are **symmetric** — there is no upstream/downstream direction, so `A verb B` and `B verb A` mean exactly the same thing:

| Verb | Meaning |
|---|---|
| `same_as` | the two terms denote the same concept across the context boundary |
| `contrasts` | the two terms are related but meaningfully different |
| `distinct_from` | the two terms are unrelated beyond sharing a name |

## Domain-Scoped vs. Shared Blocks

```craft
glossary re {
  billing/Invoice     same_as    subscriptions/Invoice
  billing/dunning     contrasts  subscriptions/dunning
}

glossary {
  re/billing/Invoice  distinct_from  legacy/reporting/Invoice
}
```

The first block is scoped to domain `re`, so `billing` and `subscriptions` resolve as bounded contexts declared under `re`. The second block is unscoped (shared/global), so its term nodes must resolve unambiguously — here `re/billing/Invoice` and `legacy/reporting/Invoice` are both domain-qualified since they cross a domain boundary.

## Validation

Like `context_map`, `glossary` statements are checked for shape and resolution, not for modeling "correctness" — craft won't second-guess whether two terms really are the same concept, only whether the statement is well-formed and non-contradictory:

| Code | Severity | When |
|---|---|---|
| `craft/sema/glossary-endpoint-not-bc` | error | the BC part of a term node resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/glossary-unresolved-bc` | warning | the BC part of a term node doesn't resolve to any declared bounded context |
| `craft/sema/glossary-ambiguous-bc` | error | a bare BC name is a bounded context in two or more domains — qualify it as `domain/bc/term` |
| `craft/sema/glossary-self-relation` | error | both sides resolve to the same term node (same BC and same term) |
| `craft/lint/glossary-redundant` | warning | the same unordered term-node pair is declared with the same verb more than once |
| `craft/lint/glossary-conflicting-relation` | warning | the same unordered term-node pair is declared `same_as` and also `distinct_from` or `contrasts` (asserting identity and difference at once) |

The redundant and conflicting lints key on the **unordered** pair, so `billing/Invoice` and `re/billing/Invoice` resolving to the same BC collapse correctly, and `A verb B` is treated the same as `B verb A`.

Run `craft validate` to see these diagnostics surface for your file.

## Complete Example

```craft
domain re {
  billing
  subscriptions
}

domain legacy {
  reporting
}

glossary re {
  billing/Invoice        same_as    subscriptions/Invoice
  billing/dunning        contrasts  subscriptions/dunning
}

glossary {
  re/billing/Invoice     distinct_from  legacy/reporting/Invoice
}
```

## Next Steps

- Learn about [domains](/language/domains) to declare the bounded contexts a glossary term node references
- See [Context Map](/language/context-map) for the strategic BC-to-BC relationship view that `glossary` complements
- See the [language overview](/language/overview) for the full construct list
