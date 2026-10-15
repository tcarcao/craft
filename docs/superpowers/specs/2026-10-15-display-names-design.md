# Display Names for Contexts and Actors — Design Spec

**Date:** 2026-10-15
**Owner:** Tiago Carção
**Status:** Implemented on feat/display-names
**Release:** v2.27.0 (minor: additive syntax, additive `pkg/craft` fields, two new lint codes)

## 1. Problem

Every journey step starts with a context id: `nmf_monetization`, `vasscheduler`, `odynrelocations`.
The id is the right identity for the model, but a domain reader does not recognise it. Renaming the
contexts would break every reference and every link to the hub. The owner decided to add an optional
**display name**, declared once in the registry. Views use it so steps read naturally, and no journey
file changes.

## 2. Syntax

A display name is an optional quoted string after the name, **on the same line**, at the two places
where contexts and actors are declared:

```craft
domains {
    billing {
        nmf_monetization  "nMF, the money engine"
        portfoliovalue
    }
}

domain Payments {
    ledger "the ledger"
}

actors {
    user Seller        "the seller"
    scheduler CRON     "the nightly clock"
}

actor system AdCore "OLX ad-core"
```

Rules:

1. `bounded_context = identifier STRING?` and `actor_definition = actor_type actor_name STRING?`
   (also for the standalone `actor` form). The string must start on the name's line.
2. **Backward compatible.** Before 2.27 a string in these places was a syntax error
   (`unexpected-token`), so no file that parsed changes meaning. A file without display names parses,
   projects and renders byte-for-byte as in 2.26.1.
3. **Unambiguous.** A string on its own line after a name is still `unexpected-token`: a domain body is a
   list of names, and the parser never has to guess which name a string belongs to.
4. **Contexts are named in the domain registry only**, not in a service's `contexts:` list. `contexts:`
   references contexts; the domain block declares them. One declaration site means no conflict rule.
5. The string is unquoted with the usual escapes (`\"`, `\\`). `""` declares no display name and is a
   warning (§6).
6. A second occurrence of a context in one domain body is dropped as today, together with its string.
7. Domains and services have no display name in this release.

## 3. Model and export

All additions are optional JSON keys (`omitempty`), so `.craftjson` goldens do not change.

| Type | Field | JSON | Meaning |
|---|---|---|---|
| `Domain` | `DisplayNames map[string]string` | `displayNames` | bounded-context name → display name, only for contexts that declare one |
| `Actor` | `DisplayName string` | `displayName` | the actor's display name |
| `Participant` | `DisplayName string` | `displayName` | the resolved participant's display name (bounded context or actor) |

`CraftDoc.DisplayNames()` returns a map from participant ID (`<domain>/<bc>` or the actor name) to
display name. `pkg/craft` re-exports these through the existing aliases; nothing is removed.

Resolution fills `Participant.DisplayName` on trigger subjects, action subjects and targets, store
participants and surface participants, so the hub reads it per step without a lookup.

## 4. Rendering

The display name replaces the text the reader sees. The diagram identifier stays derived from the id, so
the id is still in the source, and the element order (sorted by id) does not change.

| View | Without | With a display name |
|---|---|---|
| PlantUML sequence | `participant "nmf_monetization" as nmf_monetization` | `participant "nMF, the money engine" as nmf_monetization` |
| Mermaid sequence | `participant nmf_monetization as nmf_monetization` | `participant nmf_monetization as nMF, the money engine` |
| PlantUML flow | `frame "nmf_monetization" as nmf_monetization` | `frame "nMF, the money engine" as nmf_monetization` (wrapped at 24 characters, see below) |
| Mermaid flow | `nmf_monetization["nmf_monetization"]` | `nmf_monetization["nMF, the money engine"]` |
| Event queue | `queue "nmf_monetization events"` | `queue "nMF, the money engine events"` |
| Architecture | context frame shows the id | context frame shows the display name |
| C4 (boundaries) | container `nmf_monetization`, group `nmf_monetization Domain` | container and group show the display name |
| C4 actors | `Person(Seller, "Seller", …)` | `Person(Seller, "the seller", …)` |
| Surface table | `## nmf_monetization` | `## nMF, the money engine (nmf_monetization)` |

A display name longer than 24 characters wraps on word boundaries wherever it is drawn: `\n` in PlantUML
(sequence participants, flow and architecture frames, actors, event queues, C4 actors, groups and containers)
and `<br/>` in Mermaid, whose sequence, flowchart and C4 renderers all break at it. A word longer than 24
characters keeps its own line. Ids are never wrapped, and the surface table heading does not wrap.

Actors follow the same rules (`actor "the seller" as Seller`). In C4 transparent mode a service is one
container and no context name is drawn, so nothing changes there. Messages, notes and step labels are
not rewritten: they are the author's text.

## 5. Language server

- **Hover** on a context or actor reference in a use case, on an actor declaration, and on a bounded
  context in a domain body adds a second line `display name: nMF, the money engine`.
- **Completion** of contexts and actors appends the display name to the item detail:
  `bounded context (domain: billing): nMF, the money engine`.
- **Editor extraction** (`craft.extractWorkspace`) adds `displayName` to each bounded-context and actor
  entry, and the partial-DSL reconstruction of a domain keeps each context's display name, so editor
  previews draw it.

## 6. Diagnostics

| Code | Severity | When |
|---|---|---|
| `craft/lint/duplicate-display-name` | warning | Two contexts or actors in the workspace share a display name (exact text). Reported on each later string, in sorted file order. Readers could not tell the two apart in a diagram. |
| `craft/lint/empty-display-name` | warning | The display name is `""`. It is ignored; remove it or write the name. |

## 7. Formatter

Canonical form: one space between the name and the string, both on one line
(`nmf_monetization "nMF, the money engine"`). Trailing comments keep today's alignment rules. The formatter
never moves a string onto another line, and a file that has none is formatted as before.

## 8. Tests

- Parser: grouped, standalone and actor forms; a string on the next line stays an error; the tree
  round-trips its text.
- Projection and resolution: `Domain.DisplayNames`, `Actor.DisplayName`, `Participant.DisplayName`; a file
  without display names gives the same JSON.
- Lints: duplicate (across files and kinds) and empty.
- Visualizer: goldens for a `display-names.craft` model across every view and backend, plus
  `make test-integration` render checks through PlantUML and mermaid-cli (podman).
- LSP: hover, completion detail, extraction, formatter idempotence.
- Corpus regression: every view of re-arch-central renders byte-identical to 2.26.1 when no display name
  is declared.
