# A tag statement that fails to parse still lands in the model, and its tail becomes a fabricated key

> **Status: fixed** in craft 2.18.0. All three suggested fixes
> are implemented. Fix 1: a statement that fails to parse closes as a
> `SyntaxKindErrorNode` rather than a `TagStmt`, on every failure path, so
> nothing half-parsed reaches the model. Fix 2: a bare comma is now accepted,
> as a real list rather than a joined string, and `UseCase.TagValues` carries
> the items unjoined; a dangling comma is reported at the comma and fails its
> statement. Fix 3: the `tags` block is documented in
> `docs/page/language/use-cases.md` rather than `docs/GRAMMAR.md`, which is
> frozen at v1 and keyed to the removed ANTLR grammar.
>
> Tests: `internal/syntax/tag_list_test.go`,
> `internal/syntax/tag_recovery_test.go`,
> `testdata/corpus/04_use_cases/tags_list.craft`, and
> `testdata/broken/tag_dangling_comma.craft`.

**Version:** craft 2.17.2 (homebrew)
**Area:** `internal/syntax/parser.go` — `parseTagStmt` error recovery
**Impact:** `craft check` emits tags that are not in the source

## Summary

`tags { channels: web, mobile }` is rejected with two errors — correctly, a bare
tag value cannot contain a comma. But the model `craft check` produces is not the
source minus the bad line. It is the source **plus a tag nobody wrote**:

```console
$ craft check t.craft | jq '.useCases[0].tags'
{
  "channels": "web",
  "mobile": ""
}
```

`mobile` is not a tag key in the input. It is the tail of a value that failed to
parse, promoted to a key by error recovery.

## Reproducer

```craft
use_case "A" {
    tags {
        channels: web, mobile
    }

    when Seeker enters the page
        frontendplatform asks graphql for it  [ad]
}
```

```console
$ craft validate t.craft --format json
[
  { "line": 3, "severity": "error", "message": "unexpected \",\", expected a tag key" },
  { "line": 4, "severity": "error", "message": "unexpected \"}\", expected `:`" }
]
```

## Behaviour table

| input | errors | `craft check` tags |
|---|---|---|
| `channels: web, mobile` | 2 | `{"channels": "web", "mobile": ""}` |
| `channels: web,mobile` | 2 | `{"channels": "web", "mobile": ""}` |
| `entry: a, b` | 2 | `{"entry": "a", "b": ""}` |
| `channels:` (no value) | 1 | `{"channels": ""}` |
| `channels: "web, mobile"` | 0 | `{"channels": "web, mobile"}` ✓ |
| `channels: web` | 0 | `{"channels": "web"}` ✓ |

The quoted form is correct and is the workaround. Everything above the last two
rows puts something in the model that is not in the file.

## Mechanism

`parseTagStmt` (`parser.go:947`) parses `IDENT ':' (IDENT | STRING | ref-shaped-slug)`.
The bare-value branch delegates to `parseRef`, whose scanner deliberately spans
slashes and hyphens so `re/renewal-flow` survives as one value. A comma is not in
that set, so the scanner stops at it and the statement ends early. Recovery then
re-enters `parseTagStmt` at the comma:

1. `,` is not an identifier → `diagUnexpected(keyTok, "a tag key")`, and the token
   is consumed as `SyntaxKindError`. Good.
2. `mobile` **is** an identifier, so it is consumed as a key via
   `p.consumeAs(SyntaxKindIdent)`.
3. `}` is not a colon, so the second diagnostic fires — and the recovery path is:

```go
if p.peek().Type != lexer.TokenColon {
    diags = append(diags, p.diagUnexpected(p.peek(), "`:`"))
    p.builder.FinishNode()
    return diags
}
```

`FinishNode()` closes a `SyntaxKindTagStmt` that holds an identifier and no value.
Nothing marks it bad. The model builder reads that node as a tag whose value is the
empty string.

Note the contrast with the failure one branch above it, which does
`p.consumeAs(SyntaxKindError)` before finishing. That path leaves an error node;
this one leaves a well-formed-looking `TagStmt`.

## Why this matters more than the error count suggests

The diagnostics are correct, at error severity, and a human running `craft
validate` will see them. The problem is that **the diagnostic channel and the data
channel disagree**, and consumers read the data channel.

A tool that runs `craft check` and ingests the JSON — which is what that
subcommand is for — receives `mobile: ""` with nothing in the payload marking it
synthetic. It has to separately run `validate`, parse diagnostics, correlate them
by line, and decide which parts of its own input to distrust. In practice it will
not, and a tag that was never authored enters the graph.

This is the same shape as the misplaced-`when` bug filed alongside this one: the
emitted model does not correspond to the source, and the only signal that it does
not is in a different output stream.

## Suggested fixes

1. **Do not materialise a `TagStmt` that failed its value parse.** In the colon
   branch, mark the node the way the key branch already does
   (`consumeAs(SyntaxKindError)`), so the model builder skips it. That alone
   removes the fabricated key.
2. **Consider accepting a bare comma in tag values.** `channels: web, mobile` is
   the natural way to write a list and at least one downstream consumer documents
   that exact form (a `parseChannels` helper that splits on commas). If commas stay
   unsupported, the diagnostic could name the fix rather than the symptom —
   `bare tag values cannot contain commas; quote the value: channels: "web, mobile"`
   would be more useful than `unexpected "," expected a tag key`.
3. **Document the tags block.** `docs/GRAMMAR.md` does not mention `tags` at all,
   so the quoted-value form — the one that works — is discoverable only by reading
   `parseTagStmt`. The bare-slug support for `re/renewal-flow` is a deliberate and
   non-obvious feature that deserves the same.

Fix 1 is the correctness issue. Fixes 2 and 3 are ergonomics.

## Not affected

`craft fmt` round-trips the malformed block byte-for-byte rather than
"correcting" it into the fabricated shape, so the file on disk stays honest. The
divergence is confined to the parsed model.
