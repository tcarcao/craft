# Aliases on pages and flows (v2.38)

**Status:** accepted, 2026-11-10.

## Context

People look pages, critical flows and use cases up by names the model does not hold: Polish or Portuguese words ("podbicie", "estatísticas do anúncio"), or team jargon ("pushup", "bump"). A use case can already list them in a free `alias` tag, because `tags { }` keys are free and keep comma lists. A page or flow could not: `alias:` in its body was `craft/syntax/unknown-page-property` or `unknown-flow-property` and was left out of the model.

## Decision

- **One word, `alias`, everywhere.** On a use case it stays a tag (`alias: pushup, bump`). On a page or a flow it is a property.
- **One alias per line, repeatable, like `url:`.** A page or flow property holds one value (an identifier or a quoted string). A comma list on one line is `craft/syntax/unexpected-token`, with a message that says to write one line per alias. This keeps every page and flow property one value per line, and keeps a comma inside a quoted alias unambiguous.
- **The model keeps the aliases in source order, trimmed, each once.** `Page.Aliases` and `Flow.Aliases` (JSON `aliases`). An empty alias is `craft/sema/empty-alias`; the same alias twice on one entity is `craft/sema/duplicate-alias` (both warnings; compared after trimming, case-sensitive).
- **No language or site qualifier.** A consumer that matches aliases folds case and diacritics itself, so a qualifier would change no lookup, and it would need structured values.

## Out of scope

- Comparing aliases across entities (an alias that equals another entity's ref or name, or one shared by two pages). That needs the whole corpus and a folding rule; the consumer's lint does it.
- A recognised `alias` tag key on use cases. It stays free text.
- Aliases on journeys, contexts or services.

## Consequences

- Pages and flows written with `alias:` lines need craft 2.38 or later; 2.37 warns and drops them.
- Diagrams do not show aliases; `inspect`, `check` and the language server do.
