# Formatting Configuration and Cell Alignment

**Status:** Accepted, not yet implemented
**Date:** 2026-08-21
**Amends:** `docs/decisions/token-stream-formatter.md`, which listed "changing indent width"
under "Out of scope" and fixed alignment policy to operation annotations only.

## Problem

`craft fmt` rewrote 378 of the 612 lines of a real workspace file,
`re-arch-central/docs/topology.craft`, a registry that cross-references 119 service
manifests. Nothing was lost from the model: `diff -w -B` between input and output is empty,
which is the token-stream invariant doing its job. Every change was whitespace. Three of
those changes cost the reader something.

**1. The trailing-comment column collapsed.** The file carries 111 rows of the shape

```craft
        atlas-selleraccount        // rollup: bc:atlas/account
```

all hand-aligned to a single column. `separatorFor` treats a comment as an ordinary token,
so the gap before it takes the `case 0:` arm in `formatsep.go:155-161` and collapses to one
space. All 111 rows lost their column.

**2. Wrapped list continuations became ambiguous.** Five statements in the file wrap. The
formatter preserves the author's line break, because the comma rule (`formatsep.go:70-73`)
and the colon rule (`formatsep.go:61-64`) both bail when the gap contains a newline, but it
then recomputes the indent from block depth. The result:

```craft
    contexts: atlas-advert, atlas-search, atlas-selleraccount, atlas-identity, atlas-agency,
    atlas-promotion, atlas-billing, atlas-geo, atlas-investment, atlas-messaging,
    connection
```

`connection` carries no trailing comma and sits at the same column as `contexts:`, so it is
visually indistinguishable from a bare statement in the enclosing block. The reader has to
reconstruct list membership from the previous line's trailing comma. Keeping the author's
break while discarding the indent that made the break legible is worse than either joining
the line or leaving it alone.

**3. Indent width is not negotiable.** The file is authored at 4 spaces; the formatter emits
2, hardcoded at `formatsep.go:17`. 45 of the 99 `.craft` files in the craft repo corpus are
4-space. There is no way to express the preference: `cmd/craft/fmt.go:57` declares only
`--check`, and the LSP handler at `internal/lsp/server.go:1327-1335` ignores
`params.Options` entirely.

Loss 1 and loss 3 are preference. Loss 2 is a defect: it destroys information the author
encoded.

## What the prior art actually does

Every claim below was verified by running the formatter, not read from memory.

**Trailing-comment alignment is normal, not exotic.** gofmt aligns trailing line comments and
consecutive column blocks through `text/tabwriter`, with `go/printer` emitting `vtab` as the
cell separator and `formfeed` to end a run. hclwrite, which formats HCL and is the closest
analogue to craft as a configuration DSL, splits each line into `lead`/`assign`/`comment`
cells and aligns each across chains of consecutive rows, unconditionally and with no
configuration. clang-format exposes `AlignTrailingComments`. rustfmt is the dissenter:
`struct_field_align_threshold` and `enum_discrim_align_threshold` both default to 0 and are
nightly-only, on the grounds that alignment makes diffs churn.

**Alignment runs reset at structural boundaries.** gofmt resets the column *inside* a single
`var` block when an entry cannot be split into the same cells:

```go
var (
	a      = 1 // one          column 14
	bb     = 2 // two          column 14
	nested = map[string]int{   no comment cell: run ends
		"x": 1,
	}
	averyveryverylongname = 3 // three    column 29
	c                     = 4 // four     column 30
)
```

`hclwrite.Format` behaves the same way, resetting at `{`, `}`, blank lines, and standalone
comment lines. The trigger in both is "a line that cannot be split into the same cells",
not "a declaration ended".

**Continuation indent has converged on block indent, away from visual indent.** rustfmt
defaults to `indent_style = "Block"`, and `"Visual"` is rejected on the stable channel
("unstable features are only available in nightly channel", tracking issue #3346). The Rust
Style Guide states the reason: *"Prefer block indent over visual indent... This makes for
smaller diffs (e.g., if `a_function_call` is renamed in the above example) and less rightward
drift."* scalafmt flipped `align.openParenCallSite` from `true` to `false` in v1.6. Google
Java Style §4.6.3 says horizontal alignment is *"never required"* and cites the cost
directly: *"Introducing formatting changes on otherwise unaffected lines corrupts version
history, slows down reviewers, and exacerbates merge conflicts."* black, prettier, gofmt and
hclwrite never produce visual alignment. PEP 8 permits both, which is evidence this was
historically a genuine fork rather than an obvious call.

clang-format is the sole holdout, defaulting to `AlignAfterOpenBracket: Align` in every
bundled style. It is also the oldest and most configuration-driven of the set.

**Exploding to one item per line is not available to craft.** prettier and rustfmt explode
by default and black explodes as a fallback, but all three own line breaking and have a
`max_width`. The two formatters that do not re-wrap, gofmt and hclwrite, both use hang and
both preserve the author's breaks. Craft is in that family. Adopting explode would mean
acquiring a line-width budget and replacing "preserve the author's wrap" with "compute the
wrap".

## Decisions

### D1: One alignment engine, many cell types

`alignAnnotations` (`internal/lsp/formatalign.go:42-93`) is already generic. Nothing in it
knows what an action is; the only annotation-specific part is `splitAnnotation`
(`:130-149`), a pure `(line, from) -> (body, cell, ok)` predicate. Aligning operation
annotations but not trailing comments is not a defensible line, it is one `split` function
short of consistent. Both are the same construct:

```
    ␣␣␣␣␣␣␣␣ atlas-selleraccount ␣␣␣␣␣␣␣ // rollup: bc:atlas/account
    └──┬───┘ └──────┬───────────┘ └─┬──┘ └──────────────┬─────────┘
     indent      cell 0: body      pad            cell 1: trailing

    column = max(body width over the run) + 1
```

The generalisation is a second extractor plus a `commentStart` map produced by the walker.
`writeTokens` already tracks `col` incrementally (`formatter.go:274-285`) and records
`commentEnd[line] = col` after `advance(tok.Text())` at `:383`; capturing the pre-advance
value is a two-line change. `splitAnnotation`'s `body == ""` guard (`:146-148`) already gives
the correct behaviour for comment-only lines, which must never align.

### D2: Alignment scope is configurable, and defaults to `block`

Scope determines what ends a run.

| Value | A run ends at | Precedent |
|---|---|---|
| `off` | n/a, single space | rustfmt default |
| `strict` | any line lacking the cell | gofmt, hclwrite |
| `block` | blank line, `{`, `}`, comment-only line | |
| `file` | a blank line, and nothing else | |
| `decl` | nothing; the column spans the whole declaration | |

Ordered least-aligning to most-aligning. `decl` sits at the far end, not next to `block`: a
declaration boundary is rarer than a blank line, so a run that survives blank lines aligns
*more* than one that does not. An earlier draft of this document had `decl` and `file` the
other way round and described `decl` as breaking at declaration boundaries; since the aligner
is already invoked once per top-level declaration, "breaks at a declaration boundary" and
"never breaks" are the same rule, and writing it the first way made `decl` an exact alias of
`file` that broke at every blank line. `decl` is the value that gives one column across a whole
`domains { }` listing, which is its entire reason for existing.

`strict` subsumes the brace rule, because `account {` and `}` carry no comment cell. It is
kept as a distinct value because for a construct where cell-less lines are common inside a
block, such as a `services` block whose `contexts:` lines have no comments, `strict` and
`block` differ.

The default is `block`. The reasoning is churn blast radius, measured on `topology.craft`,
where the 111 aligned rows span 20 domain sub-blocks:

| Scope | Rows re-indented by one new 26-char context name |
|---|---|
| `decl` or `file` | 111 |
| `block` or `strict` | 4, worst case 35 |

The file's own author chose a single global column, driven by one 25-character name,
`partner-user-provisioning`, 300 lines away from most of the rows it pads. That is the shape
gofmt and hclwrite both avoid.

The cost of `block` is real and is accepted: 6 of the 20 sub-blocks contain exactly one row,
and a run of one aligns to nothing, so those rows render at a single space. Three more blocks
have only two rows. Column widths vary from 7 to 26 across the file. A workspace that
prefers uniformity over blame locality sets `trailing_comment = "decl"`.

The default for `op_annotation` is also `block`, which preserves the behaviour documented in
`token-stream-formatter.md` under "Annotation alignment": a non-annotated action inside a run
does not reset it, and a blank line or a new scenario does. Note this is *not* the same as
`strict`; the two cell types genuinely want different scopes, which is the argument for
scope being per-cell rather than global.

### D3: Outlier guard

gofmt excludes a size outlier from setting the column and ends the run around it, using an
explicit guard on the `exprList` path: `smallSize = 40`, `r = 2.5`, compared against the
geometric mean of prior cell sizes. rustfmt's opt-in alignment has the same idea baked into
its threshold semantics, described in its docs as *"the longest variant name that doesn't get
ignored when aligning"*.

Craft adopts it as `outlier_ratio = 2.5` and `outlier_min = 40`. On `topology.craft` the
guard never fires: the geometric mean of the 111 body widths is 11.4, below `outlier_min`.
It is insurance against a single 90-character context name dragging a whole block's column,
not an active rule. Note that the guard does **not** substitute for scope: a 26-character
name in a population averaging 11.4 is not an outlier by any ratio test, so only scope bounds
ordinary churn.

### D4: Continuation lines hang at parent depth plus one continuation unit

Per the convergence documented above, block indent, not visual indent. The value is
`continuation_indent`, defaulting to 4.

```craft
    atlas-web-dist {
        contexts: atlas-advert, atlas-search, atlas-selleraccount, atlas-identity,
            atlas-promotion, atlas-billing, atlas-geo, atlas-investment,
            connection
    }
```

The column **must** be derived from `depth + 1` or from token text as a pure function, and
**never** from the measured column of the emitted previous line. Deriving it from emitted
output creates a fixed-point failure: if alignment or indent shifts the previous line on a
later pass, the continuation moves again. `formatter.go:106-119` and `formatsep.go:131-136`
both document instances where exactly that class of bug has already bitten.

This is the one change of the four that fixes a defect rather than a preference, and it is
also the one that most endangers the design. `separatorFor`'s contract is
`(prev, gap, curr, depth, startsScenario) -> string`, and its doc comment at
`formatsep.go:22-24` states that the separator is a local decision. A continuation indent
needs to know that the current token is mid-statement, which is the first piece of non-local
state in the design. It is threaded as an explicit parameter rather than derived, so the
function stays pure.

### D5: Indent width defaults to 4

No information is encoded in indent width, and every formatter surveyed treats it as either
fiat or a plain integer knob. The default changes from 2 to 4 because the corpus and the
real workspace files are predominantly 4-space.

This is a **breaking output change** for every existing user, not an additive one. It is the
main reason configuration ships in the same release: a workspace that wants the old output
sets `indent = 2`.

### D6: Configuration surface

A `.craftfmt` file in TOML, resolved from the formatted file's directory upward to the
nearest ancestor. TOML rather than YAML because it has no significant whitespace, which
matters for a tool whose entire subject is whitespace, and no dependency beyond
`BurntSushi/toml`.

```toml
indent = 4                   # 2 | 4 | "tab"
continuation_indent = 4

[align]
trailing_comment = "block"   # off | strict | block | decl | file
op_annotation    = "block"
outlier_ratio    = 2.5       # 0 disables
outlier_min      = 40
```

Resolution order, highest wins:

1. CLI flags (`--indent`, `--align-trailing-comment`), which exist so CI can override
2. the nearest ancestor `.craftfmt`
3. built-in defaults

**LSP `params.Options` is deliberately excluded**, which is the opposite of what most language
servers do and needs justifying. The client does send it: the extension registers no
formatting provider of its own, but `vscode-languageclient`'s `DocumentFormattingFeature`
sends `tabSize`/`insertSpaces` on every request, and `internal/lsp/server.go:1327-1335`
currently discards them. Honouring them would be worse than ignoring them, because VS Code
ships `editor.detectIndentation: true`, which infers `tabSize` per file *from that file's
existing content*. An existing 2-space `.craft` file would therefore format at 2 in the
editor and at 4 from the CLI, permanently and with no error, and `craft fmt --check` in CI
would contradict what the author sees on save.

The dependency runs the other way instead. The extension declares craft's width to the
editor through `contributes.configurationDefaults`:

```json
"[craft]": { "editor.tabSize": 4, "editor.detectIndentation": false }
```

This keeps one source of truth, and it also fixes autoindent-on-Enter, which today falls back
to VS Code's brace heuristic at whatever `editor.tabSize` happens to be
(`language-configuration.json` declares no `indentationRules`).

A workspace that wants a different width sets it in `.craftfmt`, and the extension's
declared default is overridden by the user's own `[craft]` settings if they disagree.

## Architecture

```
cmd/craft/fmt.go ──┐
                   ├─→ config.Resolve(path) ──→ Config ──┐
LSP textDocument/  ─┘   (flags, .craftfmt, defaults;     │
  formatting             params.Options ignored, see D6) │
                                                         ▼
                        writeTokens ──→ separatorFor(…, cfg) ──→ text
                             │                                     │
                             └── commentStart[], commentEnd[] ─────┤
                                                                   ▼
                                            alignCells(lines, cfg, extractors)
                                              ├─ splitAnnotation      (existing)
                                              └─ splitTrailingComment (new)
```

`alignCells` is `alignAnnotations` generalised over an extractor and a scope. It stays a
line-oriented post-pass run per declaration (`formatter.go:100-102`), which is what keeps it
away from the verbatim `arch` slice (`formatter.go:82-88`).

### Cell precedence, and a documented limitation

A line can in principle carry both cells:

```craft
Billing asks Ledger to record the entry  [GRPC ledger.Postings/Create]  // legacy path
```

`splitAnnotation` requires the line to end in `]`, so it cannot see an annotation that a
trailing comment follows. Such a line therefore joins the **comment** column only, and keeps
whatever spacing its author gave the annotation.

This is a deliberate limitation, not an oversight. There are **zero** such lines in the 99-file
repo corpus and zero in the workspace registry that motivated this change, so the case is
hypothetical. Closing it properly would mean either bounding `splitAnnotation`'s search at the
comment start and reattaching the tail, or replacing the two passes with a single two-column
aligner. Both are real designs; neither is worth carrying for a shape nothing produces. If such
lines ever appear, the single-pass aligner is the better of the two, because it computes both
columns from one decomposition and cannot desynchronise offsets between passes.

**Pass order is load-bearing even so.** Annotations align first, trailing comments second, and
the two passes operate on disjoint line sets: a line `splitAnnotation` accepts ends in `]` and
therefore carries no trailing comment, while a line carrying a trailing comment is rejected by
`splitAnnotation`. That disjointness is what keeps the walker's byte offsets valid across both
passes. Reversing the order would break it: the comment pass shifts a line such as
`A asks B to c  // see note [1]`, whose recorded `commentEnd` would then be too small when the
annotation pass read it, and a `from` that is too small is precisely the direction that makes
comment text be read as an annotation and whitespace inside a comment rewritten.

### Minimum gap differs by cell type

The annotation column is `max(body) + 2`; the trailing-comment column is `max(body) + 1`. The
two-space annotation gap is existing shipped behaviour pinned by four tests, and the one-space
comment gap is what reproduces a hand-aligned column. `alignCells` therefore takes the minimum
gap as a parameter rather than hardcoding one.

### Why not text/tabwriter

Rejected, for three reasons.

1. It needs `\t` delimiters, which the formatter does not emit. The walker would have to
   decide alignment points anyway, at which point the existing 30-line pad loop is simpler
   and already correct.
2. **It has no notion of `interior`.** The bug recorded in `token-stream-formatter.md:189-200`
   was exactly a line-oriented pass rewriting whitespace *inside* a multi-line comment token,
   and `contentDrift` (`formatter.go:171-180`) is blind to it by construction because it
   compares whitespace-squashed strings. `alignAnnotations` carries an `interior` set
   (`formatalign.go:52-53, 63-64, 79-80`) precisely to avoid this. tabwriter would reintroduce
   the class with no guard able to see it.
3. Its width metric is `utf8.RuneCount`, identical to `formatalign.go:56`, so there is no
   accuracy gain to offset the risk.

## Behaviour changes

| Input | Before | After |
|---|---|---|
| any file | 2-space indent | 4-space indent |
| trailing `//` comment after content | one space | aligned within its run |
| comment-only line | unchanged | unchanged |
| wrapped list continuation | parent depth | parent depth + 4 |
| line with both annotation and comment | neither aligned | both aligned |
| `.craftfmt` present | ignored, file did not exist | honoured |
| LSP `tabSize` | ignored | still ignored, deliberately (D6) |
| editor indent for `.craft` | VS Code brace heuristic | extension declares 4, detect off |

`craft fmt --check` will report every previously-canonical 2-space file as unformatted until
that workspace adds `indent = 2` or reformats. This is the migration, and it is the reason
for the release note.

## Testing

The existing corpus harness (`internal/lsp/formatter_corpus_test.go:33-59`) walks all 99
`.craft` files in the repo and asserts four properties per file (`:177-200`): content changes
are whitespace-only, output reparses, formatting is idempotent, and the model is preserved.
Every property must continue to hold under every configuration, not only the defaults.

New coverage required:

1. **Config resolution.** Precedence across all four sources, nearest-ancestor lookup, a
   malformed `.craftfmt` producing a diagnostic rather than a panic, and an absent file
   yielding defaults.
2. **Scope semantics.** One table test per scope value over a fixture containing a blank
   line, a `{`, a `}`, a comment-only line, and a cell-less content line, asserting exactly
   where runs break.
3. **Idempotence under every config.** The corpus properties re-run across the cross product
   of `indent` in {2, 4} and `trailing_comment` in {off, strict, block, decl, file}. This is
   the guard against the fixed-point failure D4 describes.
4. **Continuation indent purity.** A test that formatting a file whose earlier lines change
   width does not move a later continuation line, which is the regression test for deriving
   the column from emitted output.
5. **Cell precedence.** A scenario with lines carrying annotation only, comment only, both,
   and neither, asserting two independent columns.

`TestFormatDocument_CanonicalCorpusIsByteIdentical` (`:213-234`) currently logs 47 canonical
files. Under the new defaults that set changes; the test is re-blessed against 4-space
output, and the count is recorded in the commit message.

There is currently **no CI gate running `craft fmt --check`** in either the `Makefile` or
`.github/`. Adding one is out of scope here but should follow, otherwise corpus drift stays
ungated.

## Risks

**Idempotence under interacting features.** Continuation indent and comment alignment both
run over the same lines, and a continuation line carries no comment cell. Under `strict` it
therefore breaks a comment run, under `block` it does not. The cross-product idempotence test
is what catches an interaction here, and it is the highest-value test in the plan.

**The default indent flip is user-visible churn.** Roughly half the corpus is rewritten. The
mitigation is that configuration lands in the same release, plus a release note.

**`.craftfmt` edits do not reach a running server.** The extension's watcher is
`workspace.createFileSystemWatcher('**/*.craft')` (`client/src/extension.ts:53`), which does
not match `.craftfmt`, and the server's `DidChangeWatchedFiles` and `DidChangeConfiguration`
handlers are both `return nil` stubs (`internal/lsp/server.go:633-639`). Without widening the
glob and implementing the handler, editing `.craftfmt` has no effect until the window
reloads, and stale cached config silently produces the wrong indentation. This is in scope.

**Non-local state in `separatorFor`.** D4 adds the first parameter that is not a property of
the adjacent token pair. If a second such parameter is ever needed, that is the signal the
separator abstraction has stopped paying for itself, and the whitespace decision should move
into the walker.

## Out of scope

- Line-width budget and re-wrapping. Craft preserves the author's line breaks; adopting
  `max_width` and exploding lists is a separate decision with a much larger blast radius.
- Aligning `contexts:` values or any cell other than the two named here.
- A `craft fmt --check` CI gate, which should follow but is not part of this change.
- Sorting or reordering anything. The token-stream invariant forbids it.
