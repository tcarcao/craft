# Craft Diagnostic Codebook

> **Status:** Seeded in S3; consolidated in S9.
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

---

## Internal diagnostics (`craft/internal/*`)

These codes are never shown to end users by default; they appear only in `$/logTrace` output.

| Code | Severity | Meaning | Origin |
|------|----------|---------|--------|
| `craft/internal/parser-panic` | error | Unexpected panic recovered in the parser tier | S3 |

---

*This file is updated by each grammar slice (S4–S8) as new validation rules land. S9 performs a final review pass.*
