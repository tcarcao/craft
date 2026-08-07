# Token-Stream Formatter

**Status:** Implemented
**Date:** 2026-08-07
**Commits:** `d5b8d91..e73297f`
**Supersedes:** the follow-up recorded in `docs/decisions/action-operation-brackets.md` under
"Follow-up: the formatter is reconstructive, and should not be"

## Problem

`FormatDocument` rebuilds each declaration from typed accessors. Every construct needs its
own branch, and any construct without one is dropped in silence. Eight defects of exactly
that shape were found while shipping v2.16.0:

- every comment in the document deleted
- operation annotations deleted
- `tags { }` blocks deleted
- typed event refs requoted into the deprecated form
- `context_map` and `glossary` statements collapsed onto one line with refs split
- qualified field values such as `repo:` split
- the `returns to <target>` target lost from the reparsed model
- a panic on an unbalanced `}`

None had test coverage. Every one was found by widening a test guard rather than by reading
code, which is the signature of a design where correctness depends on remembering to add a
branch.

v2.16.0 shipped `contentDrift`, which compares the output to the input with whitespace
stripped and returns the input untouched if they differ. That converts silent data loss into
a visible no-op, which is the right safety net, but it treats the symptom. The formatter can
still fail to format a construct; it just fails loudly now.

## The invariant

> Every non-whitespace token is written verbatim, exactly once, in document order. The
> formatter's only freedom is the whitespace it emits between them.

Stated that way, content preservation is a property of the emit loop rather than something a
check confirms afterwards. There is no branch that can drop a construct, because there are no
per-construct branches.

## Why this is the right shape here

The green tree is a rowan-style lossless syntax tree. `internal/green/builder.go` says so
explicitly ("matches rowan's `GreenNodeBuilder`", "matches rowan's `start_node_at`"), and
`docs/decisions/lsp-migration-plan.md` records rust-analyzer as the chosen architectural model
for the whole LSP.

In that model every byte of the source, including comments and whitespace, is a token in the
tree, and `root.Width() == len(src)` is asserted. A formatter that walks the token stream is
the natural consumer of such a tree. The formatter is the one component that broke the
pattern, and the eight defects are the cost of that.

The lever is already in place:

- whitespace is a real token, `SyntaxKindWhitespace`, carrying its exact source text
  (`internal/syntax/parser.go:1707`)
- comments are real tokens emitted in stream (`parser.go:1742`)
- so for any token, the preceding whitespace token's text says whether it began a new line

That last point is what makes trailing comments recoverable exactly, which the old design
could not do safely.

## Architecture

One walker over `root.AllTokens()`. It replaces `formatUseCaseDecl`, `formatContextMapDecl`,
`formatGlossaryDecl`, `formatDecl`, `writeBlockStatements`, `writeRefWithComments`, and
`significantTokens`.

`trailingCommentLines` is not among them: it stays, and does not go away, for the reason given
under "What gets deleted" below.

The walker (`writeTokens`, `internal/lsp/formatter.go:233-317`) holds five pieces of state:
brace depth, scenario depth, the raw whitespace text accumulated since the last emitted token
(`gap`), the previous token emitted, and whether that previous token opened the current
scenario. There is no discrete "line just ended" or "blank line pending" boolean: `separatorFor`
derives both directly from the newline count in `gap` (`internal/lsp/formatsep.go:122-134`).
Scenario depth is not optional bookkeeping; the Scenarios section just below depends on it
entirely.

### Indentation

Two spaces per level, from a depth counter with two sources.

**Braces.** `{` increments after emitting, `}` decrements before emitting.

**Scenarios.** Brace depth alone is not enough. A `use_case` body nests without braces:

    use_case "X" {          brace depth 1
      when U does x         a scenario opens here, with no brace
        A asks B for c      but its actions sit one level deeper

An earlier draft of this record claimed indentation was "purely lexical, already how
`tokenSeparator` derives it". That was wrong, and it blocked the first attempt at this work.
`tokenSeparator` derived line structure from node parentage, not from braces, which is
precisely why it could reproduce the 2/4 shape and a naive brace counter cannot.

So a `when` at brace depth 1 opens a scenario scope, and lines until the next `when` at that
depth or the enclosing `}` sit one level deeper. This stays token-driven: `when` is
`SyntaxKindKwWhen`, and the walker already special-cases it for the blank-line rule.

### Block boundaries

A `{` forces a line break after it and a `}` forces one before, whatever the author wrote.
This is the one place besides the scenario blank line where the formatter adds vertical space
rather than preserving it, and it is what lets it expand a minified declaration:

    service Foo{contexts: A}

becomes

    service Foo {
      contexts: A
    }

Preserving author line breaks applies *within* a statement. Block boundaries are structure,
not authorial line-breaking, and a formatter that leaves every minified brace untouched is not
doing its job.

Expansion stops at the brace. Several statements crammed onto one line, such as
`user Alice system Bot`, stay exactly as the author wrote them. A brace is a token the walker
can see and act on; the gap between two statements on the same line is indistinguishable from
the gap inside one, so there is no signal here to expand on. Deriving a statement boundary
structurally from the tree instead of from the author's own newlines was tried, and it produced
source that no longer parsed, because it split an action's event ref from its `[op]` annotation
across the manufactured line. The author's newlines are the only statement-boundary signal this
design has, so where the author wrote none, the walker adds none. This is an accepted limit, not
a defect: see `internal/lsp/formatsep.go:85-91` and
`TestSeparatorFor_SeveralStatementsOnOneLineStayThere`
(`internal/lsp/formatsep_test.go:153`).

### Line breaks and blank lines

- one blank line between top-level declarations
- one blank line before a `when` at depth 1 inside a `use_case`, except the first
- runs of two or more author blank lines collapse to one
- never a blank line immediately after `{` or immediately before `}`

**Author line breaks within a statement are preserved.** A field whose value the author
wrapped across lines stays wrapped:

    contexts: Authentication,
      Profile

The previous formatter joined these, which is why several corpus fixtures reported as
unformatted before this rewrite. Preserving them is what a token-stream walker does naturally,
since a newline in the source is just a whitespace token containing `\n`, and it is the more
faithful behaviour. It is the fifth row in the Behaviour changes table below rather than left
implicit, because it is a behaviour change that was not among the four quirks approved before
implementation started.

`when` is `SyntaxKindKwWhen`, so the scenario rule is a token-kind test plus a depth test, not
a tree query.

### Annotation alignment

A post-pass over the emitted lines. It groups contiguous annotated action lines, computes the
column as `max(rune width of the line before the annotation) + 2`, and rewrites only the run
of spaces before each `[`. A non-annotated action does not break a run; a blank line or a new
scenario does.

Keeping it separate from the walker is deliberate: it is the one decision that needs to see
whole lines rather than a token at a time.

That separation is also its one hazard, and the claim originally made here, that the pass
never touches token text so cannot affect content, was false as written. A multi-line block
comment is ONE token carrying newlines, so splitting the output into lines hands this pass the
comment's interior lines with nothing to distinguish them from real ones. An interior line
ending in `]` looked exactly like an annotated action, so it was padded to the run's column,
which rewrote whitespace inside comment text. Nothing caught it: `contentDrift` is
whitespace-blind, the output is still idempotent, and the corpus model comparison excludes
comment trivia.

The fix is at the emit site, not a better heuristic. `writeTokens` returns the set of written
line indices that fall INSIDE a token rather than between two of them, and `alignAnnotations`
skips those lines for both the column computation and the rewrite. Tracking `/* */` nesting in
the alignment pass would have worked, but it would put a heuristic where the walker has the
exact answer, which is the same trade this branch already refused once. With the set in place
the pass really does only rewrite whitespace between two tokens, which is what makes it unable
to affect content.

The set is expressed in terms of tokens rather than of comments. Only comments produce a
multi-line token today, but the invariant is that whitespace BETWEEN tokens is the only thing
any pass may touch, and that holds whatever kind of token grows a newline next.

## Behaviour changes

Six behaviours change. The first four rows below were approved explicitly before implementation
started. The fifth, author line breaks inside a value, surfaced during implementation and was
approved as an additional deliberate change. The sixth, minified declarations, follows the same
brace-versus-statement-boundary reasoning already given under Block boundaries above: a brace is
a token the walker can act on, a statement boundary shared with another statement on one line is
not. All six are quirk fixes rather than style decisions.

| | Before | After |
|---|---|---|
| Trailing comment | preserved but moved to its own line above | stays on its line |
| Ref-adjacent comment | splits the field across two lines | field stays on one line, comment follows |
| Interior multi-space | preserved in actions, collapsed in triggers | collapsed everywhere |
| Blank lines | as described above, but implicit | as described above, stated and tested |
| Author line breaks in a value | joined onto one line | preserved, so `contexts: A,\n  B` stays wrapped |
| Minified declarations | stayed on one line | `{`/`}` force a break, so `service Foo{contexts: A}` expands; several statements on one line still don't split (see Block boundaries) |

The trailing-comment move existed because the old design could not tell reliably where a line
ended, and placing a comment wrongly could comment out real content. The token stream removes
that uncertainty, so the reason for the move goes away.

## What gets deleted

Roughly 300 of `internal/lsp/formatter.go`'s 626 lines:

- `formatUseCaseDecl`, `formatContextMapDecl`, `formatGlossaryDecl`
- `formatDecl`, `tokenSeparator`, `isNextColon`, `isSiblingToken`
- `writeBlockStatements`, `writeRefWithComments`, `writeCommentLines`
- `significantTokens`, `isCommentKind`

**`trailingCommentLines` stays.** An earlier draft listed it for deletion on the belief that
end-of-file comments are ordinary tokens the walker would pick up. They are not: the parser
folds everything past the last real token into a single `SyntaxKindWhitespace` token, so the
bytes survive in the tree but no token carries a comment kind. That is exactly why a file
ending in a comment lost it, and a comment-only file was truncated to zero bytes, before
v2.16.0 fixed both. Deleting the function reintroduces those two defects. It reaches the text
through the trivia rather than through the kind, which is the only way to recover it.

`writeAlignedActions` survives in spirit as the alignment post-pass.

## Testing

The corpus guard in `internal/lsp/formatter_corpus_test.go` keeps its per-file assertions:
content preservation, reparse-clean, idempotent, and model-preserving. It walks every `.craft`
file in the repository. Byte-identity against the input is tracked separately by
`TestFormatDocument_CanonicalCorpusIsByteIdentical`, which counts how many files are already
canonical, so the gap stays visible rather than being asserted away.

`TestFormatDocument_UseCaseRoundTrip` keeps byte-identity unconditionally over its own fixture
table, since those fixtures are written in canonical form by construction. Fixtures affected
by the behaviour changes get updated there, one reviewed diff at a time.

Its `commentTexts` helper is deleted. It filtered on token kind, which is why it could not see
a trailing comment living inside the final whitespace trivia, and comments are now ordinary
tokens covered by the strongest assertion:

    squashWhitespace(input) == squashWhitespace(output)

Under this design that assertion should be impossible to fail rather than merely observed to
pass, which is the difference the rewrite buys.

`contentDrift` stays as a runtime guard. It is unreachable for any document the parser accepts
without diagnostics, and load bearing for those it does not.
`TestFormatDocument_EveryCraftFileInRepo` (`internal/lsp/formatter_corpus_test.go:154-161`)
asserts it does not fire for any file in the corpus, so that if the invariant is ever broken it
surfaces immediately rather than as a silent no-op.

The reachable half is real and is pinned by
`TestFormatDocumentChecked_DriftPathIsReachable`. An unterminated string at end of line, as in

    use_case "X" {
      when U does x A notifies "Oops
    }

produces ZERO diagnostics, so `bailsFormatting` lets it through. The lexer yields an `Ident`
whose `Text()` is `Oops` at the offset of the `"`, with the leftover byte landing in a
`Whitespace` token as `s\n`. The widths still sum, so the tree passes the losslessness check
that compares `root.Width()` against `len(src)`, but concatenating `AllTokens()` text does not
reproduce the source, and no walk over those tokens can. `contentDrift` catches it and the
formatter returns the input untouched.

That is a lexer defect upstream of the formatter and predates this branch: a token whose
`Text()` differs from the source bytes at its own range is a losslessness violation, and every
consumer of the token stream rests on that property. It is worth fixing where it lives rather
than here. The formatter's message therefore does not ask for a bug report, since for the only
known trigger the bug is not the formatter's and the file is a typo.

## Re-blessing

The six behaviour changes above change output for some fixtures. Each diff is reviewed individually
rather than accepted wholesale, and any fixture with a `.craftjson` pairing has its golden
re-verified, since a golden change would mean model drift rather than whitespace drift and is
a defect.

`craft fmt --check` reports 28 of the 60 files under `testdata/corpus` as unformatted, all
non-canonical fixture shapes that are deliberate parser test surface with zero formatter defects
among them. This work does not aim to make `--check` clean; that is a separate fixture decision
and is explicitly out of scope here.

## Risks

The alignment post-pass and the blank-line rules are the two places where output can change
unintentionally. Both are covered by the corpus guard's byte-identity assertion on canonical
files, so an unintended change fails a test rather than shipping.

The walker must handle a malformed document without panicking. `FormatDocument` already bails
out and returns the input unchanged when parsing produced an error-severity diagnostic, and
that bail-out stays in front of the walker.

## Out of scope

- making `craft fmt --check` clean on the corpus
- changing indent width, or any style decision not listed under Behaviour changes
- LSP navigation for qualified names, which is tracked separately

## Known limitations, found during review and deliberately not fixed here

**A comment closing on a line that also carries an annotation loses alignment on that line.**
`writeTokens` marks every emitted line after a token's first as interior to it, so when a
multi-line comment's `*/` shares a line with a following annotated action, that whole physical
line is excluded from the alignment run. The annotation keeps whatever spacing the author
wrote while its siblings align to a column. No content is altered and the result is
idempotent, so this is a missed alignment rather than a correctness problem. No file in the
repository has the shape.

**`trailingCommentLines` strips interior indentation.** A comment after the last declaration
goes through `trailingCommentLines` rather than the walker, and that function trims each line,
so `   note [1]` comes back as `note [1]`. This predates the rewrite and is untouched by it.
It is the one remaining path that renders a comment without going through the token walk.

**The lexer can emit a token whose text does not match its own source range.** For an
unterminated string at end of line it yields an Ident whose `Text()` is the string body while
the leftover byte lands in a Whitespace token. Widths still sum, so `root.Width() == len(src)`
passes, which makes that assertion weaker than it reads: it proves the tree spans the source,
not that concatenating token text reproduces it. This is upstream of the formatter and affects
every consumer that rebuilds text from tokens. It is why `contentDrift` remains reachable and
therefore load-bearing.
