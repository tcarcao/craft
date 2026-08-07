# Lossless token text

## Status

Accepted. Supersedes the three "Known limitations" recorded at the end of
[token-stream-formatter.md](token-stream-formatter.md).

## The problem

The token-stream formatter was built on a stated invariant: every non-whitespace token is
written verbatim, exactly once, in document order. That invariant is only as strong as the
tree's own guarantee about token text, and craft's guarantee was weaker than it looked.

`syntax.Parse` asserts `checkTreeWidth(root, len(src))`, which says only that token widths *sum* to the file
length. Rowan, the model this parser cites, guarantees something stronger: concatenating the
tokens *reproduces the source*. Width equality is a checksum. It catches length drift and
nothing else.

Two defects hid behind it.

**Tokens could lie about their own text.** `tokenText` (parser.go) returns raw source only when
`tok.Raw != ""`, and three emit sites bypass it entirely with `tok.Value`. For an unterminated
string the tree carries text `Oops` at a range whose bytes are `"Oop`: same width, different
content. `testdata/broken/unclosed_notifies_string.craft` demonstrates it at `len(src) == 331`
and `len(concat) == 331`. One file in 94 violates text equality; every one of the 94 passes the
width check.

**Comments stopped being tokens at end of file.** The trivia loop runs only *before* a token, so
a comment after the last declaration was folded into a single trailing `Whitespace` blob:

```
leading comment    LineComment:"// lead"  domain  re  {  Billing  }
trailing comment   domain  re  {  Billing  }  Whitespace:"\n// trail\n"
comment only       Whitespace:"// just this\n"
```

This is the sole reason `trailingCommentLines` existed. It was not a formatter wart but a
prosthetic for a parser gap, and being a text-scraping path outside the walker is why it
stripped indentation.

## How rust-analyzer handles this

Four properties, in dependency order:

1. **The lexer is infallible and total.** `rustc_lexer` never fails and never skips a byte. An
   unterminated string is still a token, `Str { terminated: false }`, spanning from the
   opening quote onward, text intact, quote included.
2. **Errors travel out of band.** Malformedness is carried by the token *kind* plus a parallel
   error list keyed by offset. Never by mutating the token's text.
3. **Recovery wraps, never deletes.** Tokens that cannot form a construct go inside an `ERROR`
   node, keeping their text and position. Craft already has `SyntaxKindErrorNode` for this, and
   `skipToNextField` already routes through `consume()` rather than dropping.
4. **Round-trip is asserted.** `to_string()` reproduces the source.

The fourth is only safe because of the first. Because tokens are slices of source and nothing is
ever repaired or dropped, no *input* can violate round-trip; only a *bug in the parser* can.

That is the load-bearing insight here. The question is not how loudly to complain about
malformed files; it is to make the violation structurally impossible and then assert as a
regression tripwire. A malformed `.craft` file cannot reach the assertion.

## The design

**Token text is a slice of source, always.** Every emit site writes `p.src` at the token's own
range rather than trusting `Value` or `Raw`. This closes the class, not the instance: a token
kind added later cannot reintroduce the bug, because there is no longer a code path that
constructs token text from anything but the source.

**Text equality replaces width equality.** `checkTreeWidth` becomes `checkTreeText`.

Its severity is split by build context, and the reason is worth stating because it is where craft
diverges from rust-analyzer. After the emit-site change a violation can only indicate a craft bug,
never bad input, so in a test binary it panics: loud, unmissable, and impossible for CI to skip.
In a normal build it returns a diagnostic instead, because `syntax.Parse` is reached from
`pkg/craft/parse.go`, which is craft's *public library API*, and a panic there crashes the
consumer's process rather than ours. rust-analyzer can assert freely because it is only ever an
LSP, where the worst case is one degraded request; craft is both an LSP and an embedded library,
so it does not get that freedom. The LSP itself is shielded either way by the per-handler recovery
at `internal/lsp/middleware.go`.

`testing.Testing()` selects between the two, and it is worth being exact about what it selects on,
because it is not what it looks like. The go command sets it at link time for **every** `go test`
binary, not only craft's own. A downstream module that imports `pkg/craft` and runs `go test ./...`
therefore takes the panicking branch. The split is by build kind, not by ownership. That is
defensible and is the behaviour we want, on the same reasoning as above: in a test run a panic
surfaces the defect, where a diagnostic returned into someone's assertion would be swallowed. But
"it does not panic in a consumer's process" is only true of a consumer's *released* binary, and
the record used to say it without that qualification.

The known cost is a dependency-surface change: `internal/syntax` imports `testing` from non-test
code, so `go list -deps ./pkg/craft` now includes `testing`, `flag` and `runtime/pprof`, and
`cmd/craft` links them. Importing `testing` from a library is a long-standing Go smell for exactly
this reason. No functional harm was found (flag registration moved to `testing.Init()` long ago),
and the alternative, a package variable defaulting to `testing.Testing()` that craft's own test
setup can override, would remove the import and make both branches reachable from tests. That is the
follow-up if the surface ever becomes a problem; it is not one today.

**Trailing trivia is tokenized.** The trivia loop runs after the last declaration, so a comment
at end of file becomes a `LineComment`/`BlockComment` token like every other comment.
`trailingCommentLines` is then deleted, not fixed: the walker handles trailing comments through
the same path as the rest, and the indentation strip disappears with the function.

**Alignment tracks real token ends.** `writeTokens` marked every emitted line after a token's
first as interior to it. That is an approximation of "inside a multi-line token", and it is wrong
in both directions. It over-claimed the token's last line, which the token shares with whatever
follows its close, so an annotated action written after a `*/` was excluded from alignment. And it
under-claimed the token's first line, which the token runs off the end of, so a comment opened
partway along a line left its body text looking like an alignable action.

The rule that replaced it is "the token's text runs to the end of this line", which marks the
token's first line and every line in between, and only ever releases the last. An annotation is by
definition the end of its line, so a line the token runs off the end of can never carry one, and
excluding it can only ever remove a false positive.

That was necessary but not sufficient. `splitAnnotation`, the only consumer of the set, held an
independent rule of its own: any line whose first non-space character was `*` is a comment
continuation. It vetoed exactly the line the interior change had just released, so for the
idiomatic `*`-per-line comment style the two rules cancelled and nothing changed. It now
disqualifies such a line only when the comment does not CLOSE on it, and searches for the
annotation after the `*/`. Block comments do not nest, so a `*/` cannot occur on a line a comment
is merely passing through; that is what makes the widening safe rather than lucky.

## Consequences

All three recorded limitations are removed rather than mitigated.

`contentDrift` stops being load-bearing. It stays in place as belt-and-braces, but the property
it was defending is now guaranteed upstream, so it becomes a tripwire rather than a safety net.

The tree now faithfully carries malformed source. An unterminated string keeps its opening
quote, where previously the parser silently repaired it. This is the correct behaviour, and it is
what makes the formatter safe on files that do not parse cleanly, but it is a visible change
for any consumer that was relying on the repaired form.
