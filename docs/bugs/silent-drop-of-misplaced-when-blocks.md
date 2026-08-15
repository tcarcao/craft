# A misplaced `when` block is dropped from the model and reported only as a warning

> **Status: fixed** in craft 2.17.2. Suggested fixes 1, 2 and 3
> are implemented: `craft/syntax/skipped-construct` at error severity, a range
> and message covering the whole discarded region, and a stray-`}` hint naming
> the `use_case` that closed early. Fix 4 (a distinct exit code) was not
> implemented and is no longer needed for the CI case: content loss is an error
> on its own, so `validate` fails on it without `--strict` promoting every lint
> finding. Regression tests live in `internal/syntax/skipped_construct_test.go`,
> `cmd/craft/validate_test.go`, and `testdata/broken/when_outside_use_case.craft`.

**Version:** craft 2.17.1 (homebrew)
**Area:** `internal/syntax/parser.go` — top-level recovery
**Impact:** silent content loss; `craft validate` exits 0

## Summary

When a `when` block ends up outside any `use_case` — the usual cause being one
stray `}` earlier in the file — the parser discards the block and everything in
it. The block does not reach the model: no scenario, no actions, no
participants, no operations. Every downstream consumer (`check`, `inspect`,
`generate`, LSP, and any tooling built on them) behaves as though those lines
were never written.

The only signal is a single **warning**, and `validate` exits 0.

## Reproducer

```craft
use_case "A real use case" {
    when Seeker opens the page
        frontend asks graphql for the page  [getPage]
}

when Seeker opens the payments page
    graphql asks billing for the invoices  [getInvoices]
    billing answers with the archived rows
```

```console
$ craft validate repro.craft --format json
[
  {
    "file": "repro.craft",
    "line": 6,
    "severity": "warning",
    "message": "unexpected \"when\" here; this construct is not part of the Craft grammar and was skipped"
  }
]
$ echo $?
0
```

`craft check repro.craft` returns one use_case, one scenario, one action. The
second block is gone in full — the two steps under it included.

## The shape it actually takes in a real file

Nobody writes a top-level `when` on purpose. What happens is an extra `}`:

```craft
use_case "Settle a seller invoice" {
    when Seller opens the payments pages
        graphql asks atlas/billing for invoice details  [getInvoiceDetails]
}                                    ← closes the use_case early

    // SCENARIO — the invoice-archive migration
    when Seller opens the payments pages
        graphql consults the isInvoiceArchiveEnabled flag
        graphql asks invoicearchive for the archived positions
}                                    ← now unmatched
```

Braces still *look* plausible on a skim, and the diagnostic points at the
`when` (the symptom) rather than at the `}` four lines above (the cause).

This is not hypothetical. It occurred in a 211-file corpus and survived
several review passes. Measured against that file:

| | use_cases | scenarios | actions |
|---|---|---|---|
| with the stray `}` | 6 | 14 | 57 |
| after removing it | 6 | 15 | **61** |

One scenario and four steps had been absent from the model for as long as the
typo had been in the file, and every consistency check run over the corpus had
been counting the file as sound.

## Why the warning does not carry

Three things compound:

1. **Severity contradicts the codebook.** `docs/DIAGNOSTICS.md` puts every
   `craft/syntax/*` entry at `error` — `unexpected-token` and `unclosed-block`
   both — and reserves `warning` for `craft/lint/*` style findings. This
   diagnostic is emitted at `warning`, so a file whose content is being
   discarded ranks alongside `event "X" is published but never consumed`.

2. **The code is being asked to mean two different things.**
   `parser.go:1977` returns `craft/syntax/not-yet-implemented`, whose codebook
   definition is *"a top-level construct is recognised but not yet supported by
   parser v2"* — the tool is behind, the file is fine, and `warning` is the
   right call. It is now also the catch-all for *the file is ungrammatical and
   we threw part of it away*, which is the opposite situation. The function's
   own doc comment records the drift already: the stale `--parser=antlr`
   advice was removed, and a `[`-specific message was special-cased into it.

3. **`--strict` cannot separate them.** It promotes *all* warnings, so on the
   corpus above enabling it means accepting 100+ pre-existing lint findings as
   build failures. There is no way to ask for "fail on dropped content, keep
   linting advisory" — the one thing a CI gate most wants.

Net effect: the diagnostic that means *your file is not what you think it is*
is the one most likely to be scrolled past.

## Extent of the discard is never reported

`parser.go:135-139`:

```go
default:
    // Unrecognised top-level token: emit a diagnostic and resync to
    // the next top-level keyword (island parsing).
    diags = append(diags, p.diagNotImplemented(tok))
    p.resyncToTopLevel()
```

The diagnostic's range is `tokenRange(tok)` — the four characters of `when`.
`resyncToTopLevel()` then consumes everything up to the next top-level keyword,
which can be dozens of lines, and says nothing about it. A reader is told one
token was unexpected; they are not told a block was deleted.

## Suggested fixes

In rough order of value:

1. **Split the code.** Keep `craft/syntax/not-yet-implemented` (warning) for
   constructs the grammar recognises but parser v2 does not implement. Add
   something like `craft/syntax/skipped-construct` at **error** for a token
   that cannot be placed at all and whose region is discarded. This aligns the
   behaviour with the codebook rather than changing policy.

2. **Report the extent.** Widen the range to cover everything
   `resyncToTopLevel()` consumed, and say so: `` `when` block at top level:
   lines 6-8 were skipped and are not part of the model ``. Silent loss becomes
   visible loss.

3. **Name the cause for the brace case.** When the unplaced token is `when`
   and a `use_case` closed recently, point at that `}`: *"`when` outside any
   use_case — the block at line 1 was closed at line 4; is that `}`
   extra?"* Brace-balance-aware recovery would catch the whole class.

4. **Consider a distinct exit code** (or a narrower flag than `--strict`) for
   "parsed, but content was discarded", so CI can gate on it without adopting
   every lint rule at once.

Fix 1 alone would have caught this instance.

## Not affected: `fmt`

`craft fmt` round-trips the orphaned block byte-for-byte rather than dropping
or reflowing it, so the loss is confined to the model. Checked at 4- and
8-space step indentation.
