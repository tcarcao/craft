# Craft DSL vNext — Design Spec

**Date:** 2026-07-14
**Owner:** Tiago Carção
**Status:** Design — ready for review
**Drives:** Knowledge-Hub ticket **018** (Extend the Craft DSL) + free-text flexibility ask.
**Grounded by:** ticket **017** (node-identity, slug grammar — closed), ticket **007** (event identity — closed), PRD §6 (key design contracts).

> Craft binds to code by **identity, never location**. This spec extends the Craft grammar to (a) accept
> flexible unquoted narrative and (b) speak the locked slug/reference grammar with typed edges, so the
> hub (ticket 015) can cross-validate journeys against real code facts.

---

## 1. Goal & scope

Two evolutions, one coherent grammar bump, implemented in dependency-ordered slices:

1. **Flexibility** — unquoted narrative steps accept special characters (`! & * / ? # + …`), so
   `X asks Y for 1! & 2! and/maybe *` is legal without quotes.
2. **Typed, code-anchored references (018)** — `notifies`/`listens` carry *typed refs* (event ids /
   node slugs) instead of free-text strings; new typed edges (`realized_by`/`also_realizes`,
   `same_as`/`contrasts`/`distinct_from`); service code anchors (`repo:`/`opslevel:`); local
   well-formedness validation.

**Two implementations stay in sync:** the Go `internal/lexer` + `internal/syntax` (source of truth for
parse/validate/diagram) and `tree-sitter-craft/grammar.js` (+ the VSCode extension) for editor support.

**Out of scope (belongs to the hub, ticket 015):** cross-validation of refs against fetched SDK/OpenAPI
facts, slug→surrogate-UUID resolution, blast-radius/inverse-index derivation. `craft` checks only *local
well-formedness*; the hub checks *resolution against code*.

### Slices

| # | Slice | Delivers | Depends |
|---|-------|----------|---------|
| **A** | Tokenizer + free-text flexibility | Rest-of-line prose; special chars unquoted; `//` comment rule | — |
| **B** | Typed refs (slug grammar) | Event refs + node slugs on `notifies`/`listens`/`asks`; quoted form deprecated | A |
| **C** | Typed edges | `context_map { }` block: `realized_by`/`also_realizes`/`same_as`/`contrasts`/`distinct_from` | A, B |
| **D** | Service anchors | `repo:` / `opslevel:` service properties | A |
| **E** | `craft validate` local checks | Slug shape, local ref resolution, deprecation lints | B, C, D |
| **F** | Grammar sync | tree-sitter external scanner + rules; VSCode highlighting | A–D |

---

## 2. Slice A — Tokenizer + free-text flexibility

### 2.1 Prose regions (rest-of-line, raw)

After the parser consumes a step's **structured prefix**, the remainder of the line is captured as one
raw **prose run** — display text only, no per-word semantics. Prose positions:

| Construct | Prefix (structured) | Prose tail |
|---|---|---|
| `internal_action` | `subject verb` | rest of line |
| `sync_action` | `subject asks target` | rest of line |
| `return_action` | `subject returns [to target]` | rest of line |
| `external_trigger` | `actor verb` | rest of line |

`notifies` / `listens` are **not** prose — they take typed refs (slice B).

Grounding fact: `ActionDecl.PhraseText()` (ast.go:1093) already joins trailing tokens into one display
string and the visualizer uses it only as a diagram label (`c4_relationships.go:229`). Nothing reads
individual prose words, so capturing the tail as raw text is behavior-preserving.

### 2.2 Implementation (Go side — minimal lexer change)

The parser already batch-tokenizes (`l.All()`, parser.go:38), tracks byte offsets (`prevEnd`), and the
lexer emits a `TokenError` for every unknown char rather than failing. So:

- At a prose position the parser records `start = offset of next token`, consumes tokens until a
  **newline or a comment token**, records `end`, and stores `strings.TrimSpace(src[start:end])`.
- `TokenError` chars swept into a prose run are **not** diagnosed. Specials **outside** prose remain
  errors (prose acceptance is parser-context-driven, not a lexer-global relaxation).
- No new token kinds required for prose; the change is in `internal/syntax` (parse of action/trigger
  tails) + suppressing prose-interior `TokenError` diagnostics.

### 2.3 Comment rule (whitespace-preceded)

A comment begins only at a **whitespace-preceded** `//`, `///`, or `/*`. Consequence:

- `Auth checks uniqueness  // TODO: cache` → prose `Auth checks uniqueness` + trailing comment ✓
- `and/maybe`, `50/50`, `http://api`, `a*b` (unspaced) → stays prose ✓ (URLs & ratios survive)

This is a change from today's "`/` anywhere may start a comment on `//`". Lexer `Next()` gains a
"previous char was whitespace / line-start" guard before treating `//`|`///`|`/*` as a comment.
**[default — veto on review]**: alternative is "`//` always a comment" (simpler, but eats URLs).

### 2.4 Backward compatibility

Every existing `.craft` step is already a valid prose tail; quoted strings still parse anywhere. No
corpus file needs editing for slice A. Round-trip (lossless tree) must reproduce prose byte-for-byte.

---

## 3. Slice B — Typed refs (slug grammar)

### 3.1 Reference kinds

Two reference shapes, both scanned in **ref positions** (parser-driven, like prose):

**Event ref** — a qualified, dotted identifier resolving to the code-derived canonical id
(FQ Avro record name / OpenAPI `operationId`). No `kind:` prefix, no `/`.
```
subscriptions.SubscriptionCreated
com.olx.re.subscriptions.SubscriptionCreated   // fully qualified also legal
```

**Node slug** — the locked grammar `[kind:][namespace/]name`, `kind ∈ {domain, bc, term, service}`:
```
domain:re/monetization
bc:re/subscriptions
term:billing/dunning
service:subscriptions-api
```
The parser distinguishes them structurally: a `:` after the leading word ⇒ node slug; dotted-only ⇒
event ref; a bare word ⇒ short form resolved by context (§3.3).

### 3.2 Where refs appear

| Construct | Before | After |
|---|---|---|
| async publication | `notifies "VasApplied"` | `notifies vas.VasApplied` |
| domain listener | `when X listens "VasApplied"` | `when X listens vas.VasApplied` |
| sync target | `Auth asks Database …` | `Auth asks bc:re/billing …` (or bare `Billing`) |

### 3.3 Term module-scoping (from 017)

- A **bare** term inside its own BC's artifact ⇒ that BC's namespace (clean local vocabulary).
- Any **cross-BC** term ref **must** be prefixed with the full BC slug (`term:<bc>/<name>`). A bare
  cross-context term is a **hub** error (craft can't see other BCs; it checks shape only — §5).
- Agent-facing output always renders fully qualified.

### 3.4 Lexing slugs (`:` and `/` in ref position)

In ref positions the parser requests a **ref scan** that reads
`(kind ':')? (segment '/')* name`, each segment/name matching `[A-Za-z0-9_.-]+`. Because ref scanning is
parser-context-driven (only after `notifies`/`listens`/`asks`/in `context_map` endpoints), `:` and `/`
keep their structural meanings everywhere else. `//` never appears inside a well-formed slug.

### 3.5 Deprecated fallback (migration)

`notifies "X"` / `listens "X"` (quoted string) **still parse** but emit a `deprecated-string-ref` lint
pointing at the typed form. Keeps the existing corpus (e.g. `vas.craft`) green while nudging migration.
**[default — veto on review]**: alternative is a hard break (reject strings immediately). Recommend a
deprecation window; a `craft migrate` codemod (string→ref, best-effort) is a **fast-follow, not v1**.

---

## 4. Slice C — Typed edges (`context_map` block)

A new top-level block holding **authored inter-BC edges** (the central Craft layer's job per PRD §4).
DDD-native name; one home for all five typed edges.

```craft
context_map {
    // BC ↔ service realization (usually hub-derived from repo→opslevel.yml;
    // authored here only for the multi-repo BC case, per 017 Q4)
    bc:re/subscriptions realized_by service:subscriptions-api
    bc:re/vas           also_realizes service:vas-application-api

    // cross-BC term relations (lower-trust authored assertions; hub checks endpoints resolve)
    term:subscriptions/dunning contrasts     term:billing/dunning
    term:ordering/order        same_as       term:offering/order
    term:vas/apply             distinct_from term:billing/apply
}
```

**Grammar:** `context_map '{' NEWLINE* (edge_stmt NEWLINE*)* '}'` where
`edge_stmt := ref EDGE_KW ref`, `EDGE_KW ∈ {realized_by, also_realizes, same_as, contrasts, distinct_from}`.
Endpoints are node slugs (§3.1). `realized_by`/`also_realizes` require a `service:` slug on the right and a
`bc:` slug on the left; term edges require `term:` slugs on both sides — checked locally for **shape**
(§5), resolved by the hub.

**[default — veto on review]:** the alternative placement is hanging `realized_by` off each BC inside
`domains { }`. Rejected: term edges have no BC to hang on, so they'd need a block anyway — one `context_map`
block is more coherent than two mechanisms.

---

## 5. Slice D — Service anchors

Extend `service_property` with two code anchors (no file paths, per 018/007):

```craft
services {
    SubscriptionsApi {
        contexts: Subscriptions
        language: golang
        opslevel: subscriptions-api      // the OpsLevel alias == service surrogate id (017)
        repo: olxeu/realestate/subscriptions
    }
}
```

- `opslevel:` value = the alias identifier (same token as `@opslevel_component_id:<alias>` in CODEOWNERS).
- `repo:` value = a repo slug (identifier/slug or quoted string).
- Both optional; each may appear at most once (duplicate ⇒ diagnostic).

---

## 6. Slice E — `craft validate` local checks

`craft` validates **well-formedness only** (local, in the choreography repo). The hub does cross-validation.

- **Slug shape:** `kind ∈ {domain,bc,term,service}`; namespace matches the per-kind parent rule
  (`domain:re/…`, `bc:<domain>/…`, `term:<bc>/…`, `service:<alias>` with no namespace). Malformed ⇒ error.
- **Event ref shape:** non-empty dotted qualified name; no `kind:`/`/`. Malformed ⇒ error.
- **Local resolution (best-effort):** refs to nodes declared in the same file-set resolve; unknown refs are
  a **warning** (`unresolved-ref-local`), not an error — the hub is authoritative (a step whose event the
  code doesn't emit is the hub's red build, per 018).
- **Edge endpoint kinds:** `realized_by`/`also_realizes` ⇒ `bc:` ⟶ `service:`; term edges ⇒ `term:`⟶`term:`.
  Wrong kind ⇒ error.
- **Lints:** `deprecated-string-ref` (§3.5); duplicate `opslevel:`/`repo:`; bare cross-BC term left as a
  local warning (`possible-unqualified-term`) since craft can't prove cross-BC.

All diagnostics flow through the existing `internal/sema` diagnostic path and the `testdata/broken/*`
`.diagnostics.json` harness.

---

## 7. Slice F — Grammar sync (tree-sitter + extension)

- **External scanner (C)** in `tree-sitter-craft/src/scanner.c` for the **prose run** and the **ref scan**
  (tree-sitter can't do the Go parser's context-driven raw slice). The scanner emits a `prose` token to
  end-of-line (honoring the whitespace-`//` rule) and a `ref` token for slugs/event ids.
- New grammar rules mirroring §3–§5: `event_ref`, `node_slug`, `context_map` + `edge_stmt`, `opslevel:`/
  `repo:` service properties; `notifies`/`listens`/`asks` object positions switch from `string`/`phrase`
  to `ref`/`prose`.
- VSCode extension: highlight `context_map`, the edge keywords, slug `kind:` prefixes, and prose vs ref;
  update the grammar's `CORPUS_VERSION` and regenerate `craft.dylib`/`.wasm`.
- Keep the differential harness (`internal/parser_diff`) green — Go parse vs tree-sitter parse must agree
  on the corpus.

---

## 8. Testing

- **Corpus (`testdata/corpus/*.craft` + `.craftjson`):** add vNext fixtures — prose with `! & * / #`,
  `http://` URLs, `//`-comment-after-prose; typed `notifies`/`listens`; `context_map` with all five edges;
  service anchors. Existing corpus must stay green (back-compat).
- **Broken (`testdata/broken/*`):** malformed slug, wrong edge endpoint kind, duplicate `opslevel:`,
  event ref with a `/`, unterminated prose across lines. Each with expected `.diagnostics.json`.
- **Round-trip:** lossless tree reproduces prose byte-for-byte (spaces, specials, trailing comment split).
- **Canonical acceptance line:** `X asks Y for 1! & 2! and/maybe *` parses as `sync_action`:
  subject `X`, target `Y`, prose = `for 1! & 2! and/maybe *` (everything after the target is prose, so the
  connector `for` is part of it — no special-casing of connector words in the tail).
- **Differential:** Go vs tree-sitter agree on the full corpus (`parser_diff`).
- **Migration:** every current `examples/*.craft` and `vas.craft` still parse; quoted refs emit exactly one
  `deprecated-string-ref` each.

---

## 9. Decisions taken (defaults — flag any to change)

1. **Rest-of-line raw prose** for narrative tails (vs widened token). *(approved)*
2. **Whitespace-preceded `//`** starts a comment (URLs/ratios survive). *(default)*
3. **Quoted `notifies "X"` deprecated, not removed**, with a `deprecated-string-ref` lint + fast-follow
   codemod. *(default)*
4. **One `context_map` block** for all five typed edges (vs per-BC annotations). *(default)*
5. **`craft` = local well-formedness only**; unresolved refs are warnings, hub is authoritative. *(from 018)*
6. **tree-sitter external scanner** for prose + ref (slice F). *(forced by tree-sitter limits)*

---

## 10. Open follow-ups (not v1 of this spec)

- `craft migrate` codemod (string refs → typed refs).
- Whether `arch`/`exposure`/`domain` blocks also gain slug refs (this spec covers use-case flow refs +
  service anchors + context_map; other blocks are unchanged).
- Rendering: agent-facing fully-qualified term rendering lives in the hub (015), not `craft`.
