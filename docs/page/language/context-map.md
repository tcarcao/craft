# Context Map

`context_map` is craft's **strategic view** of bounded-context relationships: a set of statements that classify each bounded-context-to-bounded-context relationship using a recognized DDD strategic context-mapping pattern (customer/supplier, conformist, anticorruption layer, open-host service, published language, partnership, shared kernel, separate ways).

## Two views

Craft already derives a **communication view** — *who calls whom* — from use-case interactions (`asks`, `notifies`/`listens`), and renders it as C4, domain-flow, and sequence diagrams. That graph shows that two bounded contexts talk to each other.

It does not, and cannot, show *how they relate strategically*. `billing asks vas` says nothing about whether `vas` conforms to `billing`'s model, wraps it behind an anticorruption layer, or the two teams are partners — that classification is a human modeling judgment craft can't infer. `context_map` supplies exactly that non-inferable layer. It is not a call graph; it's the strategic overlay that turns the communication graph into a *context map* in the DDD sense.

## Basic Syntax

```craft
context_map {
  billing customer_supplier vas
  billing open_host_service subscriptions
}
```

**Syntax:**
```craft
context_map <domain>? {
  <bc_ref> <pattern> <bc_ref>
}
```

- The `context_map` block is **repeatable** — declare as many as you like across a file or workspace, and every edge merges into one context map.
- The optional `<domain>` scopes bare endpoint names to that domain, e.g. `context_map re { ... }`. An unscoped `context_map { }` is the shared/global map.
- Each statement is **one pattern per line** — a pair that is both, say, open-host-service and published-language becomes two separate statements, not a bracketed list.

## Endpoints

Endpoints (`bc_ref`) name a **bounded context**, either bare or domain-qualified:

```craft
billing        // bare — resolves within the block's domain scope (or globally if unscoped)
re/billing     // domain-qualified — crosses a domain boundary, or disambiguates a colliding name
```

- There is **no `bc:` prefix** inside `context_map` — the block itself implies every endpoint is a bounded context.
- `/` (slash) is the node-identity separator (`domain/bc`), consistent with craft's slug system elsewhere.
- `.` (dot) stays reserved for event refs (`vas.VasApplied`) — it must never appear in a `context_map` endpoint.

## Domain-Scoped vs. Shared Blocks

```craft
context_map re {
  billing customer_supplier vas
  billing anticorruption_layer subscriptions
  billing partnership vas
}

context_map {
  re/billing separate_ways legacy/reporting
}
```

The first block is scoped to domain `re`, so `billing`, `vas`, and `subscriptions` resolve as bounded contexts declared under `re`. The second block is unscoped (shared/global), so its bare or qualified endpoints must resolve unambiguously across all declared domains — here `re/billing` is qualified, and `legacy/reporting` crosses into a different domain.

## The Pattern Catalog

Eight patterns, using the community-recognized DDD names. **Direction convention: LEFT = upstream, RIGHT = downstream** for every directional statement — read it as `<upstream> <pattern> <downstream>`, matching how a DDD context-map diagram is drawn (the arrow points upstream → downstream).

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

For the five **directional** patterns, endpoint order carries meaning: `billing open_host_service subscriptions` says `billing` is the host and `subscriptions` is the consumer — reversing the endpoints reverses the claim.

For the three **symmetric** patterns, endpoint order is meaningless: `a partnership b` and `b partnership a` say the same thing.

::: tip
`big_ball_of_mud` is intentionally not a `context_map` pattern — it's a zone/boundary marker over one or more bounded contexts, not a pairwise edge, so it doesn't fit the `<left> <pattern> <right>` shape.
:::

## Validation

`context_map` endpoints and statements are checked for shape, not for modeling "correctness" — craft won't second-guess whether a stated pattern is the right one, only whether the statement is well-formed:

| Code | Severity | When |
|---|---|---|
| `craft/sema/edge-endpoint-not-bc` | error | an endpoint resolves to a domain, service, or actor — not a bounded context |
| `craft/sema/self-relationship` | error | both endpoints resolve to the same bounded context (`X <pattern> X`) |
| `craft/sema/unresolved-bc` | warning | an endpoint doesn't resolve to any declared bounded context |
| `craft/lint/redundant-relationship` | warning | the same unordered pair is declared with the same **symmetric** pattern more than once (directional duplicates in opposite order are *not* redundant — `a customer_supplier b` and `b customer_supplier a` are different claims) |

Run `craft validate` to see these diagnostics surface for your file.

## Complete Example

```craft
domain re {
  billing
  vas
  subscriptions
}

domain legacy {
  reporting
}

context_map re {
  billing customer_supplier vas
  billing open_host_service  subscriptions
  subscriptions conformist   billing
  billing anticorruption_layer subscriptions
}

context_map {
  re/billing separate_ways legacy/reporting
}
```

## Next Steps

- Learn about [domains](/language/domains) to declare the bounded contexts a context map references
- Model interactions with [use cases](/language/use-cases) — the communication view that `context_map` complements
- See the [language overview](/language/overview) for the full construct list
