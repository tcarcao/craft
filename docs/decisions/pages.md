# Pages (v2.36)

**Status:** accepted, 2026-11-06.

## Context

Use cases already name where they start with a free-text `entry:` tag (`entry: ad-detail, search-results`); the re-arch-central corpus has several hundred of them. Nothing said what `ad-detail` is, who owns it, or at which URLs users open it. Ownership questions ("who owns My Ads?") were answered from memory.

## Decision

- **A page is the owned surface users see.** `page <ref> "<name>" { … }` declares it: the screen or view a seller, seeker or operator opens, such as "My Account: My Ads". It is a top-level, contextual declaration, like `flow` (2.35): `page` is a keyword only when `<ref> "<name>" {` follows on its line.
- **Ownership is the landlord of the product domain.** A page has one required `product_domain:`. Craft does not name a team: the team is the product domain's landlord, synced elsewhere, so it changes in one place.
- **Use cases point at pages with `entry`.** The page ref is the value use cases already write in `entry:`, so declaring a page links the existing use cases with no edit to them. `entry` becomes a recognised tag key (`UseCase.Entries`), and each page gets the reverse index `UsedBy`, as `Flow.RealisedBy`.
- **Where users open it:** `url:` once per site (Otodom, Storia, …), in order and each once; `platform:` is `frontoffice`, `backoffice` or `app`.
- **Adoption without noise.** An `entry` value with no page is `craft/sema/unresolved-page` only when the loaded files declare at least one page. A model that has not started declaring pages is unchanged: same `validate`, `inspect` and `fmt --check` output.

## Out of scope

Migration tracking (old vs new stack, rollout status, traffic share per site) is deliberately left out. It changes weekly, belongs to a tracker, and would make a page declaration go stale. A `note` line can mention it.

## Consequences

- Pages are not drawn in any diagram; `inspect`, `check` and the language server show them.
- A page ref is declared once per workspace (`craft/sema/duplicate-page`, an error), as a flow ref.
