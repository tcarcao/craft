# Action Operation Brackets

**Status:** Accepted, not yet implemented
**Date:** 2026-08-06
**Breaking:** Yes. Acceptable, see Migration.

## Problem

A `use_case` action line could carry a qualified reference only by putting it in the
target noun slot:

```craft
Subscriptions asks bc:re/billing for a fresh charge attempt
```

That slot is the sentence's object. Putting machine identity there destroys the
sentence, and it gets worse as the identifier gets deeper:

```craft
Subscriptions asks Billing.op1.op2/op3/op4/op5 for a fresh charge attempt
```

Two distinct needs were being forced through one syntactic slot:

1. **Which bounded context** is being asked. Disambiguation.
2. **What wire call** is being made. `POST /v1/accounts/{id}/charges`, a gRPC method,
   a topic name.

Need 2 had no home at all, so it leaked into the slot built for need 1.

Underlying this is a decision about what a `use_case` is. It is now explicitly **the
integration contract**, not only a business narrative. A use case says who talks to
whom, in business language, and over what wire.

## Decision

Three changes.

### 1. A trailing bracket on every action

Any action may end with a bracketed operation annotation.

```craft
use_case "Retry a failed charge" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/accounts/{id}/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry                [GRPC ledger.Postings/Create]
    Gateway returns to Billing the authorization result    [200 AuthorizationResult]
    Billing notifies billing.ChargeSucceeded               [TOPIC billing.v1.charge-succeeded]
    Subscriptions marks the subscription active
}
```

The bracket is optional. Lines without one stay exactly as they are today.

Applies to `asks`, `notifies`, `returns`, and internal actions. One uniform rule.

### 2. Bracket contents are hybrid: typed when possible, opaque otherwise

The first whitespace-delimited token inside the bracket is matched against a known
protocol verb set. If it matches, the annotation is structured. If it does not, the
entire bracket content is stored verbatim as an opaque payload and no diagnostic
fires.

| Written | Verb | Payload |
|---|---|---|
| `[POST /v1/charges]` | `POST` | `/v1/charges` |
| `[GET /v1/products?q=]` | `GET` | `/v1/products?q=` |
| `[GRPC ledger.Postings/Create]` | `GRPC` | `ledger.Postings/Create` |
| `[TOPIC billing.v1.charge-succeeded]` | `TOPIC` | `billing.v1.charge-succeeded` |
| `[op1/op2/op3/op4/op5]` | none | `op1/op2/op3/op4/op5` |
| `[legacy-mainframe-txn-44]` | none | `legacy-mainframe-txn-44` |

Initial verb set: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `GRPC`,
`TOPIC`, `QUERY`. Extending it is additive and non-breaking, because unknown leading
tokens already parse as opaque payload.

This gives completion and lint where the shape is known, without ever blocking a
shape nobody anticipated.

### 3. Target slot drops `kind:` prefixes, keeps the namespace path

```craft
Subscriptions asks Billing for a fresh charge attempt        // short form
Subscriptions asks re/billing for a fresh charge attempt     // qualified form
Subscriptions asks bc:re/billing for a fresh charge attempt  // REMOVED
```

New target grammar: `ident ('/' ident)?`. A leading `kind:` prefix in an action target
is a parse error with a fix-it that strips it.

This follows a rule `context_map` already enforces. From
`docs/page/language/context-map.md`: there is no `bc:` prefix inside `context_map`,
because the block itself implies every endpoint is a bounded context. The action
target slot is equally constrained.

The `re/` namespace segment stays. It is genuine identity (domain plus bounded
context), and it is how you disambiguate when two domains both own a `Billing`.

## Grammar

```
action        ::= subject verb [target] [connector] [phrase] [op_bracket]
target        ::= ident ('/' ident)?
op_bracket    ::= '[' op_body ']'
op_body       ::= [protocol_verb WS] payload
payload       ::= any token run up to the closing ']'
```

## Parse rules

**Boundary.** `collectPhrase` (`internal/syntax/parser.go:1164`) currently sweeps every
same-line token into the prose tail. It gains one stop condition: the last `[` on the
line whose matching `]` is line-final opens an operation bracket. Everything before it
is phrase, everything between the brackets is the annotation.

The "last `[`, `]` line-final" rule makes this greedy-safe with no lookahead. A `[` that
does not close at end of line stays prose, so today's sweep-everything behaviour is
preserved for any line that is not using the feature.

**Tokens.** `TokenLBracket` and `TokenRBracket` already exist
(`internal/lexer/lexer.go:57-58`, emitted at `226-228`). No lexer change required.

**Precedent.** `parseComponentWithModifiers` and `parseModifierList`
(`internal/syntax/parser.go:1342`, `1367`) already parse `Name[...]` for arch
components. The bracket convention is established in the language, not new.

**Corpus safety.** Zero of the 450 `asks` lines in the repo contain `[`. Reclaiming the
character costs nothing in practice.

**Empty annotation.** `A asks B for c []` satisfies the boundary rule (the line ends in
`]`), so it parses as an annotation with an empty body. That is a user error, not an
empty-but-valid operation, and it emits `craft/syntax/empty-op-annotation`. The model
records no `Operation` for it. Without the diagnostic the `[]` would vanish from both
the phrase and the model with no signal, which is the one outcome worse than either
alternative.

**Why brackets and not a sigil.** `POST /v1/charges` contains a space. A sigil form
such as `@POST /v1/charges` gives the parser no signal for where the payload ends,
because the prose tail is free text. Brackets are self-terminating, so the payload can
contain spaces, slashes, `{}` templates, and query strings without ambiguity.

## Complexity this does not remove

An earlier draft of this decision claimed the span-skipping helpers in
`internal/syntax/ast.go` could be deleted once `kind:` prefixes were gone. That is
wrong, and the correction matters for scoping the work.

Keeping the `re/billing` qualified form means an action target is still a multi-token
`Ref` (three flat tokens: `re`, `/`, `billing`). Every helper that skips a target's
actual span therefore stays load-bearing:

- `Connector()` (`ast.go:~1256`), the `elementSpan(elems[2])` skip
- `phraseStartIndex` (`ast.go:~1620`), the same skip for `sync_action`
- `ActionDecl.PhraseStartIndex()`, exported so `internal/lsp/server.go:2463` does not
  drift
- `refAwareText` / `refAwareOffset` in the `sync_action` branches of `TargetName()`
  and `TargetCol()` (`ast.go:1441`, `1484`)
- `adjacentTokens` (`parser.go:2236`), which edge endpoints also still need

Dropping the prefix shortens the ref by two tokens. It does not make it a single token.
Revisit only if the qualified form is ever removed as well.

## Semantics and storage

`model.Action` gains a structured operation field. The existing `Ref` field is
populated at `internal/syntax/projection.go:245,358` and read by nothing outside
tests. No visualizer touches it. Every generator renders only `Context`,
`TargetContext`, `Connector`, and `Phrase`:

- `internal/visualizer/sequence.go:109`
- `internal/visualizer/domain.go:338,401,461,777`
- `internal/visualizer/c4_relationships.go:195`
- `internal/visualizer/mermaid/sequence.go:85`

**Consequence: this change does not alter any generated diagram.** The annotation is
for tooling that does not exist yet (go-to-definition against an anchored spec,
contract validation, codegen). Diagram rendering of the annotation is deliberately out
of scope for the first cut. Ship the syntax and the model field, decide rendering
separately.

### Public API

`pkg/craft` is the stable public Go API and follows semver. Everything a library
consumer needs to work with an annotation must be reachable there, since
`internal/model` and `internal/syntax` are not importable from outside the module:

- `craft.Operation = model.Operation`, alongside the existing type aliases. Without it
  a consumer can read `action.Operation.Verb` but cannot declare a variable, struct
  field, or parameter of that type.
- `craft.OpVerbGET` … `craft.OpVerbQUERY` constants, matching the constant-block
  convention every other closed set in that file already follows (`ActionType*`,
  `TriggerType*`, `ActorType*`, `ComponentType*`).
- `craft.ProtocolVerbs() []string`, re-exporting `syntax.ProtocolVerbs` so the set is
  iterable for validation and for populating UI.

`Operation.Verb` stays a plain `string` rather than a named enum type. The verb set is
open by design: an unrecognised head word is payload, not an error, and extending the
set later is additive. A named type would imply a closed set and would make `""` (the
opaque-payload case) an awkward enum member.

## Ambiguity resolution for bare targets

`resolveBCRef` (`internal/sema/validate.go:362`) already does scope-first lookup,
qualified lookup, and ambiguity counting, and `craft/sema/ambiguous-bc` already exists
(`validate.go:734-741`). `context_map` and `glossary` pass a real `site.ScopeDomain`.
Use-case actions pass `""` hardcoded (`internal/sema/lint.go:341-348`).

There is no domain scope on `use_case` today, and this decision does not add one. With
`scopeDomain` empty, `resolveBCRef` still returns `ambiguous=true` for a bare name owned
by two or more domains (`validate.go:380-394`). The actionable gap is that
`buildDependencyEdges` silently drops that case instead of reporting it.

So: emit `craft/sema/ambiguous-bc` wherever a use case resolves a bounded context
ambiguously, with the fix being to write `re/billing`. This is what makes dropping `bc:`
safe, the ambiguity that qualification was papering over becomes a diagnostic instead.

`buildDependencyEdges` resolves bounded contexts at three sites, all of which previously
dropped the ambiguous case silently, and all three are now diagnosed:

1. `sync_action` subject and target (`asks`)
2. `async_action` subject (`notifies`)
3. `domain_listen` trigger context (`when X listens ...`)

Only site 1 lost its `kind:` escape hatch in this release, so covering 2 and 3 is not
strictly required to make the breaking change safe. It is covered anyway, because a
value that silently vanishes from the dependency graph is a bug regardless of whether
the author had a workaround, and because one rule is easier to document than three.

Adding `use_case "..." in <domain>` scoping is deferred until someone hits a case where
per-file scoping would genuinely reduce noise.

## Migration

Across 320 `.craft` files, exactly **2** use a qualified or kind-prefixed target, both
test fixtures:

- `testdata/corpus/99_mixed/dsl-vnext.craft` and its `.craftjson` golden
- `testdata/broken/malformed_slug.craft` and its `.diagnostics.json`

Zero occurrences in `examples/` (8 files, 53 `asks` lines). Zero qualified `notifies` or
`listens` anywhere.

No codemod. Two hand edits, one regenerated golden, one adjusted diagnostics
expectation. Docs prose to update: `docs/page/language/use-cases.md:123,125` and
`.claude/skills/craft-dsl/SKILL.md:124,227`.

## Formatting

`craft fmt` aligns operation brackets into a column, per contiguous run of annotated
lines, resetting on a blank line. This is the gofmt struct-tag rule and it is what
makes the right-hand column readable as a unit: you read down the brackets and see the
entire integration surface of the use case.

Alignment is cosmetic. The grammar stays whitespace-insensitive, and the formatter must
be idempotent. `DocumentFormattingProvider` and `FormatDocument` already exist
(`internal/lsp/server.go:1325`), so this is an extension, not a new tool.

## LSP

- Completion: `[` becomes a trigger character. Inside a bracket at position 0, complete
  the protocol verb set. After a known verb, complete against the anchored spec if one
  is resolvable.
- Semantic tokens: classify the bracket contents as a distinct token type so themes can
  dim it. The annotation should recede visually, it is plumbing.
- Go-to-definition on the payload, once spec anchoring exists.

## Rejected alternatives

Recorded because each was seriously considered and each failure mode is informative.

| Alternative | Why rejected |
|---|---|
| `A asks B using op/id <phrase>` | Misparses legal prose today. `Fulfilment asks Warehouse using automated pickers` silently reparses as target `Warehouse`, qualifier `automated`, phrase `pickers`. No error, wrong dependency edge. |
| Qualifier on an indented continuation line | Doubles the vertical cost per annotated action and creates a second indent level competing with `when` nesting. |
| Separate `contract` / `refs` / alias block | Requires a second place to look to understand one line. |
| Trailing sigil (`@`, `#`, `;`, `\|`) | Cannot hold a payload containing a space, which `POST /v1/charges` requires. |
| Bracket between target and connector | Splits subject-verb-object from the phrase, and the annotations never align into a column. |
| Naked trailing token, no delimiter | Silently steals any phrase ending in a slashed word, for example "approve the request and/or escalate". Undiagnosable. |
| `published_language` block at the provider | Strategically the more correct DDD home, and it keeps `use_case` lines pure. Rejected because a use case is now explicitly the integration contract, so the wire call belongs at the call site. Revisit if per-call-site version drift becomes painful. |
| Derive the identifier from the phrase | Cannot express a legacy path that does not correspond to the business wording. |
| Inlay hints / editor concealment | `git diff`, GitHub review, `cat`, and grep have no LSP. Inlay hints can only add virtual text, never hide file text, and VS Code folding is whole-line only. |

## Known costs

- `[` is no longer freely usable at the end of a phrase. `docs/page/language/use-cases.md:125`
  advertises the phrase tail as accepting punctuation unquoted, and that claim needs
  narrowing.
- A `/v1/` written at every call site means a version bump is an unvalidated sweep
  across many files. Accepted for now. This is the strongest argument for the
  `published_language` alternative and the reason it is recorded rather than discarded.
- The annotation is unvalidated until spec anchoring exists, so it can drift from
  reality. In OpenAPI terms it will behave like `summary`, not like `operationId`,
  because nothing breaks when it is wrong.

## Out of scope

- Rendering the annotation in generated diagrams
- Validating the payload against an anchored OpenAPI or protobuf spec
- Request and response schema modeling
- `import "<path>"`, which lexes and parses (`parser.go:79,165`) but has no consumer in
  sema, workspace, or `pkg/craft`. Dead syntax. Wire it or delete it, separately.

## Testing

- Lexer: unchanged, existing bracket token tests cover it
- Parser: bracket present and absent on all four action forms; `[` in prose that does
  not close at line end stays prose; unterminated bracket diagnostic
- Round-trip: `roundtrip_test.go` byte-exact reassembly, `lossless_check_test.go` root
  green width equals `len(src)`
- Sema: verb recognized, verb unrecognized and opaque, `craft/sema/ambiguous-bc` fires
  on a genuinely ambiguous bare target, `kind:` prefix in target rejected with a fix-it
- Corpus: new fixture exercising all protocol verbs plus an opaque payload
- Formatter: alignment idempotent, `craft fmt --check` clean on the corpus
