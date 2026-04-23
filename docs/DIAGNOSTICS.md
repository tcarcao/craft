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
| `craft/syntax/unexpected-token` | error | Token found where a different token was expected | `unexpected "domain", expected actor type (user/system/service)` | S3 |
| `craft/syntax/unclosed-block` | error | A `{` was never closed before EOF | `unclosed actors block (missing \`}\`)` | S3 |
| `craft/syntax/not-yet-implemented` | warning | A top-level construct is recognised but not yet supported by parser v2 | `construct starting with "domain" is not yet supported by parser v2; use --parser=antlr for full support` | S3 |

---

## Semantic diagnostics (`craft/sema/*`)

| Code | Severity | Meaning | Example message | Origin |
|------|----------|---------|-----------------|--------|
| `craft/sema/duplicate-name` | error | Two declarations in the same namespace share a name | `actor "Alice" already declared (first seen at line 1)` | S3 |
| `craft/sema/cross-kind-name-reuse` | warning | The same identifier is declared in two different kind-namespaces (e.g. actor and domain) | `"Customer" is declared as both an actor (line 0) and a domain (line 3); consider renaming to avoid confusion` | S4 |
| `craft/sema/sema-panic` | error | Unexpected panic recovered in the sema tier; analysis results may be incomplete | `internal sema error: <panic value>` | S4 |
| `craft/sema/unresolved-reference` | error | A service's `contexts:` list names a bounded context or domain that doesn't exist in any workspace file | `service "UserService" references context "UnknownBC" which is not declared in any domain` | S5 |
| `craft/sema/invalid-exposure-target` | error | An exposure field references an identifier of the wrong kind: `to:` must name actors; `through:` must name services; `contexts:` must name domains, bounded contexts, or services | `exposure "default": \`to:\` target "Payments" is a domain, not an actor` | S8 |

---

## Lint diagnostics (`craft/lint/*`)

Style and consistency warnings produced by `sema.LintWorkspace`. These mirror the heuristics from `internal/linter/` but operate on the v2 AST and workspace symbol table (B5, S9).

| Code | Severity | Meaning | Example message | Origin |
|------|----------|---------|-----------------|--------|
| `craft/lint/dead-event` | warning | An event is published via `notifies` but never consumed by any `when … listens` or event trigger in the workspace | `event "Order Processing" is published but never consumed` | S9 |
| `craft/lint/unused-actor` | warning | An actor is declared but never appears as the subject of an external trigger (`when <Actor> …`) | `actor "Admin" is defined but never used as a trigger subject` | S9 |
| `craft/lint/event-not-past-tense` | warning | An event name does not appear to use past tense (does not contain a word ending in `-ed` or `-en`) | `event "Order Processing" does not appear to use past tense` | S9 |

---

## Internal diagnostics (`craft/internal/*`)

These codes are never shown to end users by default; they appear only in `$/logTrace` output.

| Code | Severity | Meaning | Origin |
|------|----------|---------|--------|
| `craft/internal/parser-panic` | error | Unexpected panic recovered in the parser tier; LastGoodAST is used as fallback for semantic features | S3 |

---

*Final review performed in S9. All codes stable from S9 onward unless a new grammar construct is added.*
