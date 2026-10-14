# Craft Diagnostic Codebook

> **Status:** Consolidated in S9. ✅ Complete.
> **Convention (Q19):** golden files assert on `code` + `range` + `severity` only. The `message` field is free-form prose and is NOT asserted in tests.

Each entry lists:
- **Code** — stable machine-readable identifier
- **Severity** — error | warning | info | hint
- **Meaning** — what the diagnostic means
- **Example message** — sample prose (not normative)
- **Slice-of-origin** — slice that first introduced this code

---

## Syntax diagnostics (`craft/syntax/*`)

| Code | Severity | Meaning | Example message | Origin |
|------|----------|---------|-----------------|--------|
| `craft/syntax/unexpected-token` | error | Token found where a different token was expected, including a verbless action line, tokens after an `alt`/`opt`/`else`/`par`/`and`/`loop` label, tokens after a note's text, tokens left on a line after its operation annotation's `]` (`[0]{]`), and any line in a surface body that is not `callers:`, `spec:`, a note or a `query`/`command` line | `unexpected "domain", expected actor type (user/system/service)`; `action "x" has no verb: write \`x <verb> …\` on one line`; `unexpected "…", expected end of line after the block label` | S3 |
| `craft/syntax/unclosed-block` | error | A `{` was never closed before EOF. A use case, a domain or a surface without its `}` also stops at a declaration that starts at column 1 (`surface <name> {`, `actor <kind> <name>`, or another top-level keyword), so it does not swallow the next declaration; an indented `surface asks …` stays an action | `unclosed actors block (missing \`}\`)`; `unclosed surface block (missing \`}\`)` | S3; 2.23 |
| `craft/syntax/not-yet-implemented` | warning | A construct is recognised by the grammar but not yet supported by parser v2. The tool is behind; the file is fine | `a trailing [operation] annotation is not recognised on a line whose phrase contains braces` | S3 |
| `craft/syntax/skipped-construct` | error | A token could not be placed at all, and the region top-level recovery discarded because of it is absent from the model | `unexpected "when" at top level: lines 6-8 were skipped and are not part of the model` | 2.17.2 |
| `craft/syntax/empty-branch` | error | An `alt`, `opt`, `else`, `par`, `and` or `loop` header has no body line. The block is kept with an empty branch. A branch whose only step is a `note`/`finding` is not empty | `\`alt "…"\` has no steps: indent its steps under it` | 2.21; 2.23 |
| `craft/syntax/inconsistent-indent` | error | A line sits left of its block's first body line but right of the header (any block kind: `alt`, `opt`, `par`, `loop`). It joins the innermost block whose header is left of it | `this line is indented less than the steps above it but more than its block header; align it with one of them` | 2.21 |
| `craft/syntax/orphan-else` | error | An `else` has no open `alt` at its column, or its block is an `opt`, a `par` or a `loop`. Parsed as a new `alt` with that label | `\`else\` continues an \`alt\` at the same column; a \`par\` continues with \`and\`` | 2.21; message widened in 2.23 |
| `craft/syntax/orphan-and` | error | An `and` has no open `par` at its column, or its block is an `alt`, an `opt` or a `loop`. Parsed as the first branch of a new `par` with that label | `\`and\` continues a \`par\` at the same column; an \`alt\` continues with \`else\`` | 2.23 |
| `craft/syntax/missing-label` | error | `alt`, `opt`, `else`, `par`, `and` or `loop` stands alone on its line (treated as a header with an empty label), or `note`/`finding` has no quoted text (the note is dropped) | `\`alt\` needs a quoted label, e.g. \`alt "payment fails"\``; `\`note\` needs a quoted text, e.g. \`note "a lost callback leaves the invoice open"\`` | 2.21; 2.23 |
| `craft/syntax/missing-operation` | error | A surface `query`/`command` line has no trailing `[op]`. The line is dropped from the model | `` a `query` line needs its operation in brackets, e.g. `query the seeker list  [getUserList]` `` | 2.23 |
| `craft/syntax/missing-phrase` | error | A surface `query`/`command` line has its `[op]` but nothing before it that says what the operation offers (`query [getUserList]`) | `` a `query` line says what the operation offers before its bracket, e.g. `query the seeker list  [getUserList]` `` | 2.23 |
| `craft/syntax/scenario-without-when` | error | A `scenario "…"` line is not followed by `when`. The name is dropped | `\`scenario "…"\` must be followed by a \`when\` line` | 2.21 |

> **`not-yet-implemented` vs `skipped-construct`.** The two used to share a code, which put a file whose content was being thrown away at the same severity as `event "X" is published but never consumed`. They are opposites: `not-yet-implemented` says the file is fine and the parser will catch up, so it stays advisory; `skipped-construct` says a region of the file is gone from the model and every consumer (`check`, `inspect`, `generate`, LSP) will agree it never existed, so it fails `validate` without `--strict`. Its range spans the whole discarded region, not just the token that triggered recovery.

---

## Semantic diagnostics (`craft/sema/*`)

| Code | Severity | Meaning | Example message | Origin |
|------|----------|---------|-----------------|--------|
| `craft/sema/duplicate-name` | error / warning | Two declarations in the same namespace share a name. An error within one file; a warning when a service, domain or surface is declared again in another file, where the first declaration (sorted file order) wins. Two `surface` blocks for the same resolved subject count as duplicates | `actor "Alice" already declared (first seen at line 1)`; `surface "concierge" is already declared at a.craft:3; the first one is used` | S3; surfaces 2.23 |
| `craft/sema/bc-in-multiple-services` | warning | A bounded context is listed in the `contexts:` of two services; a bounded context belongs to one service, and the first declaration keeps it | `bounded context "re/Billing" is already deployed by service "First"; a bounded context belongs to one service, so "Second" is ignored here` | 2.20 |
| `craft/sema/cross-kind-name-reuse` | warning | The same identifier is declared in two different kind-namespaces (e.g. actor and domain) | `"Customer" is declared as both an actor (line 0) and a domain (line 3); consider renaming to avoid confusion` | S4 |
| `craft/sema/sema-panic` | error | Unexpected panic recovered in the sema tier; analysis results may be incomplete | `internal sema error: <panic value>` | S4 |
| `craft/sema/unresolved-reference` | error | A service's `contexts:` list names a bounded context or domain that doesn't exist in any workspace file; or a surface's subject, or a `callers:` entry other than `unknown`, does not resolve | `service "UserService" references context "UnknownBC" which is not declared in any domain`; `surface subject "concierge" is not declared as a bounded context, service or actor` | S5; surfaces 2.23 |
| `craft/sema/invalid-surface-subject` | error | A surface's subject resolves to a domain or a data store. A surface belongs to a bounded context, a service or an actor (an external system) | `a surface belongs to a bounded context, a service or an actor (an external system); "obido" is a domain` | 2.23 |
| `craft/sema/duplicate-operation` | error | The same operation text appears twice in one surface (compared with whitespace collapsed) | `` operation `[getUserList]` is already listed in this surface `` | 2.23 |
| `craft/sema/invalid-exposure-target` | error | An exposure field references an identifier of the wrong kind: `to:` must name actors; `through:` must name services; `contexts:` must name domains, bounded contexts, or services | `exposure "default": \`to:\` target "Payments" is a domain, not an actor` | S8 |
| `craft/sema/invalid-schedule` | error | The string of a `when cron "…"` trigger is not 5 or 6 space-separated cron fields or one of the macros `@yearly`/`@annually`/`@monthly`/`@weekly`/`@daily`/`@midnight`/`@hourly` (`@every` is rejected: use `when every`), or the string of a `when every "…"` trigger is not a positive duration (`5m`, `1h30m`, or `<n>d` for days). The same rules apply to the trailing schedule of a listens trigger (`when X listens <ref> cron "…"` / `every "…"`, 2.23). The range is the string | `cron expression must have 5 or 6 fields like "0 7 * * 1-5" (or a macro such as @daily), got "0 7 * *"` | 2.22 |

---

## Lint diagnostics (`craft/lint/*`)

Style and consistency warnings produced by `sema.LintWorkspace`. These mirror the heuristics from `internal/linter/` but operate on the v2 AST and workspace symbol table (B5, S9).

| Code | Severity | Meaning | Example message | Origin |
|------|----------|---------|-----------------|--------|
| `craft/lint/dead-event` | warning | An event is published via `notifies` but never consumed by any `when … listens` or event trigger in the workspace | `event "Order Processing" is published but never consumed` | S9 |
| `craft/lint/unused-actor` | warning | An actor is declared but never appears as the subject of an external trigger (`when <Actor> …`); a `when cron` / `when every` trigger counts as a use of `CRON` | `actor "Admin" is defined but never used as a trigger subject` | S9 |
| `craft/lint/indistinguishable-scenarios` | warning | Two scenarios in one use case have identical trigger text and at least one has no `scenario "…"` name. Use `alt` for different outcomes, or `scenario` names for different ways in | `scenario repeats the trigger "when CRON fires nightly"; use alt "…" for different outcomes, or scenario "…" names for different ways in` | 2.21 |
| `craft/lint/single-branch-par` | warning | A `par` with no `and` whose body has exactly one step that is not a note, or holds only notes. Nothing in it runs concurrently | `` this `par` has one step, so nothing in it runs concurrently; drop the `par` or add the steps that run alongside it `` | 2.23 |
| `craft/lint/receives-on-actor` | warning | The subject of a `when <ctx> receives …` trigger resolves to an actor. `receives` names a boundary whose caller is not observed | `` an actor starts work with `when <Actor> <verb> …`; `receives` is for a boundary whose caller is not observed `` | 2.23 |
| `craft/lint/store-without-change` | warning | A store ref (`store.<name>.<change>`, in a `notifies` or a `listens`) does not end in `inserted`, `updated` or `deleted`, so its name takes every segment and it has no change | `` store ref `store.redis.leads.pending` names no change; end it with `.inserted`, `.updated` or `.deleted` `` | 2.23 |
| `craft/lint/store-without-name` | warning | A ref starts with `store.` but names no store (`store.inserted`, a bare `store.`). It is not a store ref and is drawn as a plain event | `` store ref `store.inserted` names no store, so it is drawn as a plain event; write `store.<name>.<change>`, e.g. `store.sms_queue.inserted` `` | 2.23 |
| `craft/lint/operation-not-in-surface` | warning | An `asks <target> … [op]` whose target is covered by a surface that does not list `op`. A bounded context without its own surface is covered by the surface of the service that deploys it. The range is the bracket | `` `[addMortgagesMeeting]` is not an operation of the `obidozoe` surface; add it to the surface or fix the operation name `` | 2.23 |
| `craft/lint/repeated-request` | warning | Two `asks` in one step list (a scenario or branch body, at any distance) from different subjects to the same resolved target with the same operation: usually two clients making one request. Not reported when the second subject was asked earlier in the list (a call chain), and once per subject per request. The range is the second subject | `` `mobile` and `frontendplatform` make the same request `[autocomplete]` to `graphql`; if they are alternatives, write it once with a `channels` tag, or open the scenario with `alt "on the web"` / `else "in the app"` `` | 2.23 |
| `craft/lint/event-not-past-tense` | warning | An event name does not appear to use past tense (does not contain a word ending in `-ed` or `-en`) | `event "Order Processing" does not appear to use past tense` | S9 |

---

## Internal diagnostics (`craft/internal/*`)

These codes are never shown to end users by default; they appear only in `$/logTrace` output.

| Code | Severity | Meaning | Origin |
|------|----------|---------|--------|
| `craft/internal/parser-panic` | error | Unexpected panic recovered in the parser tier; LastGoodAST is used as fallback for semantic features | S3 |

---

*Final review performed in S9. All codes stable from S9 onward unless a new grammar construct is added.*
