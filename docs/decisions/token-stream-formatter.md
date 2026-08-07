# Token-Stream Formatter

**Status:** Implemented
**Date:** 2026-08-07
**Commits:** `d5b8d91..HEAD` (`e73297f` at merge time)
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

That last point is what makes trailing comments recoverable exactly, which the current design
could not do safely.

## Architecture

One walker over `root.AllTokens()`. It replaces `formatUseCaseDecl`, `formatContextMapDecl`,
`formatGlossaryDecl`, `formatDecl`, `writeBlockStatements`, `writeRefWithComments`,
`trailingCommentLines`, and `significantTokens`.

The walker holds three pieces of state: brace depth, whether the previous emitted token ended
a line, and whether a blank line is pending. Nothing else.

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
not authorial line-breaking, and a formatter that cannot expand minified input is not doing
its job.

### Line breaks and blank lines

- one blank line between top-level declarations
- one blank line before a `when` at depth 1 inside a `use_case`, except the first
- runs of two or more author blank lines collapse to one
- never a blank line immediately after `{` or immediately before `}`

**Author line breaks within a statement are preserved.** A field whose value the author
wrapped across lines stays wrapped:

    contexts: Authentication,
      Profile

The current formatter joins these, which is why several corpus fixtures report as unformatted.
Preserving them is what a token-stream walker does naturally, since a newline in the source is
just a whitespace token containing `\n`, and it is the more faithful behaviour. It is listed
here as a fifth deliberate change rather than left implicit, because it is a behaviour change
that was not among the four approved quirks.

`when` is `SyntaxKindKwWhen`, so the scenario rule is a token-kind test plus a depth test, not
a tree query.

### Annotation alignment

A post-pass over the emitted lines. It groups contiguous annotated action lines, computes the
column as `max(rune width of the line before the annotation) + 2`, and rewrites only the run
of spaces before each `[`. A non-annotated action does not break a run; a blank line or a new
scenario does.

This pass never touches token text, so it cannot affect content. Keeping it separate from the
walker is deliberate: it is the one decision that needs to see whole lines rather than a token
at a time.

## Behaviour changes

Four current behaviours change. Each is a quirk rather than a decision, and each was approved
explicitly.

| | Current | After |
|---|---|---|
| Trailing comment | preserved but moved to its own line above | stays on its line |
| Ref-adjacent comment | splits the field across two lines | field stays on one line, comment follows |
| Interior multi-space | preserved in actions, collapsed in triggers | collapsed everywhere |
| Blank lines | as described above, but implicit | as described above, stated and tested |

The trailing-comment move exists because the current design cannot tell reliably where a line
ends, and placing a comment wrongly could comment out real content. The token stream removes
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

`contentDrift` stays as a runtime guard and should become unreachable. Add a test asserting it
does not fire for any file in the corpus, so that if the invariant is ever broken it surfaces
immediately rather than as a silent no-op.

## Re-blessing

The three quirk fixes change output for some fixtures. Each diff is reviewed individually
rather than accepted wholesale, and any fixture with a `.craftjson` pairing has its golden
re-verified, since a golden change would mean model drift rather than whitespace drift and is
a defect.

`craft fmt --check` currently reports 36 of 68 corpus files, all non-canonical fixture shapes
that are deliberate parser test surface with zero formatter defects among them. This work does
not aim to make `--check` clean; that is a separate fixture decision and is explicitly out of
scope here.

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
