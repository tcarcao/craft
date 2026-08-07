# Changelog

## [Unreleased]

### Changed
- **Trailing comments stay on their line.** A comment after an action, such as `Billing notifies billing.ChargeSucceeded  // retried once already`, used to be lifted onto its own line above the action it followed. The formatter now walks the token stream directly, so it can tell exactly where a line ends and no longer needs to move a trailing comment to stay safe.
- **A comment between a field and its value no longer splits the field across two lines.** `repo: // note` followed by the value on the next line used to be reflowed so the field and its value landed on separate lines with a hanging indent. The comment and the value now stay exactly where the author put them.
- **Interior runs of spaces collapse consistently on both action and trigger lines.** A run of extra spaces used to be preserved on an action line but collapsed to one space on a trigger (`when`) line. Both now collapse to a single space, since the separator logic no longer distinguishes the two.
- **Blank-line rules are unchanged in effect, but are now stated and tested.** One blank line between top-level declarations, one before a scenario's `when` (except the first in a `use_case`), and a run of two or more blank lines collapsing to one: all preexisting behaviour, now written down as the formatter's contract and covered by tests rather than left as an emergent property of the old reconstruction.
- **Author line breaks inside a value are preserved rather than joined.** A wrapped `contexts: A,\n  B` stays wrapped across lines instead of being collapsed onto one. The old formatter always joined these, which is why several corpus fixtures reported as unformatted; that gap narrows as a side effect of this change, though re-canonicalising the rest of the corpus is a separate decision and out of scope here.
- **Minified declarations partially expand.** A `{` now gets a space before it and a line break after; a `}` gets a line break before it. `service Foo{contexts: A}` expands into a properly indented block instead of staying on one line. This is an accepted limit rather than full normalisation: several statements crammed onto one line, such as `user Alice system Bot`, stay exactly as written. A gap-driven formatter has no other signal for where one statement ends and the next begins; the author's own line breaks are that signal, and deriving a statement boundary structurally instead was tried and produced output that no longer parsed. Braces are visible to the walker as tokens, so they expand; statement boundaries are not, so they don't.

### Fixed
- **The syntax tree now reproduces its source exactly.** Token text is sliced from the source at every emit site rather than rebuilt from the lexer's interpreted values, so concatenating a tree's tokens returns the original file byte for byte. Previously an unterminated string produced a token whose text was the string body without its opening quote, and `testdata/broken/unclosed_notifies_string.craft` carried 331 bytes of tree against 331 bytes of source with different content between them.
- **The parser asserts text equality rather than width equality.** `checkTreeWidth` compared the sum of token widths against the file length. Because a token's length was derived from its own text, a wrong text produced a length wrong by the same amount, so that check could not detect this class of defect at all. `checkTreeText` replaces it and compares the reconstructed text. It panics when built as a test binary so a regression cannot pass CI, and returns a diagnostic otherwise. Note that the split is by build kind and not by whose code it is: `testing.Testing()` is true in any `go test` binary, so a downstream module's own test run takes the panicking branch too. That is intended, since a panic surfaces the defect where a diagnostic would be swallowed, but a released binary built from `pkg/craft` never panics here.
- **Comments after the last declaration are proper comment tokens.** `peek()` skips comment tokens, so at end of file the parser left trailing comments unconsumed and swept them into a single whitespace token, invisible to anything walking the tree. They are now tokenized like any other comment.
- **Trailing comments keep their indentation.** The formatter no longer scrapes them out of the source text as a special case; they go through the same token walk as every other comment. The path that trimmed each line is deleted rather than fixed.
- **Consecutive trailing comments no longer gain a blank line between them.** Two comment lines written adjacent stay adjacent, and a blank line the author put between two comments survives.
- **An annotation on the line where a block comment closes now aligns with its siblings.** The walker treated every line after a token's first as interior to that token, which over-claimed the last line the token shares with whatever follows it. Both closing styles align: a bare `*/ Billing asks Gateway to charge  [POST /pay]` and the leading-star ` */ Billing asks ...` that a `*`-per-line comment produces.
- **Alignment no longer pads a bracket that is comment text.** A block comment opened partway along a line, as in `Billing asks Gateway to charge /* see [1]`, left the `[1]` looking like an annotation to a pass that works on lines, and it was padded out to the surrounding column. The walker now reports every line a token runs off the end of, not only the lines strictly inside it, so the whitespace inside a comment is left as the author wrote it.

### Notes
- **Malformed source is now carried faithfully rather than repaired.** An unterminated string keeps its opening quote where the parser previously dropped it. This is what makes formatting safe on files that do not parse cleanly, but it is a visible change for any consumer that was relying on the repaired form. `lexer.Token` gains an `End` field and loses `Raw`, whose only consumer is gone.
- **The formatter is now a single walk over the lossless token stream.** Every non-whitespace token is written verbatim, exactly once, in document order; the only decision left is the whitespace between tokens. Content preservation is therefore structural rather than something a check confirms afterwards: there is no per-construct branch left that can silently drop a construct, because there are no per-construct branches. `contentDrift` stays in place as a runtime guard against that class of defect, and is now unreachable from any input: the parser defects that used to reach it are fixed (see below), so what it still guards is a bug in the formatter's own walker, which no upstream invariant can rule out. A test asserts it does not fire for any file in the repository's corpus, and it is unit-tested directly since no real input reaches it.
- **The minified-expansion limit above is accepted and deliberate, not a defect.** Please don't file it as a bug: several statements written on one line stay on one line, because the formatter has no structural way to find the boundary between them that doesn't risk breaking the parse.

## [2.16.0] — 2026-08-07

**Why a minor version when this release contains breaking changes.** The breaks are in the DSL, not in the Go API. Everything added to `pkg/craft` here is additive (the `Operation` type, the `OpVerbGET` through `OpVerbQUERY` constants, `ProtocolVerbs()`), so no Go consumer stops compiling. Go's semantic import versioning is therefore satisfied and the module path stays `github.com/tcarcao/craft/v2`. Shipping a `v3.0.0` tag against a `/v2` module path would produce a release nobody can `go get`, and because `proxy.golang.org` caches immutably it could not be retracted afterwards; this repo already hit exactly that at v2.10.1. If you write `.craft` files, read `### Breaking` below as you would for a major release. If you only import the Go package, nothing here affects you.

### Breaking
- **`kind:` prefixes are rejected in every bounded-context slot.** `Subscriptions asks bc:re/billing for a charge` is now a parse error, `craft/syntax/kind-prefix-in-target`. This covers the `asks` target, the subject of all four action kinds, the `returns` target, and the `when ... listens` trigger context. Write the bare name (`Billing`) or the domain-qualified form (`re/billing`). The slot already implies a bounded context, the same rule `context_map` has always enforced for its endpoints.
  - Migration is mechanical: strip the prefix. Two files in this repo were affected, both test fixtures.
- **A line-final bracketed run on an action line is now an operation annotation, not prose.** A `[` that does not close at the end of the line is still swept into the phrase, so only lines that look like they carry an annotation change meaning.
- **`craft/sema/malformed-slug` now reports shapes it previously ignored.** `re/ billing`, `re//billing`, and three-segment refs like `re/a/b` are errors. Previously slash-shaped names were never shape-checked at all, because the shape checker returned early on any text without a `:`. Files that parsed silently before may now report errors.

### Added
- **Operation annotations.** Any action may end with a bracketed annotation recording the wire call it corresponds to, which makes a `use_case` readable as an integration contract:

      use_case "Retry a failed charge" {
        when CRON detects a failed charge
          Subscriptions asks Billing for a fresh charge attempt  [POST /v1/accounts/{id}/charges]
          Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
          Billing asks Ledger to record the entry                [GRPC ledger.Postings/Create]
          Gateway returns to Billing the authorization result    [200 AuthorizationResult]
          Billing notifies billing.ChargeSucceeded               [TOPIC billing.v1.charge-succeeded]
          Subscriptions marks the subscription active

  Contents are hybrid: a recognised uppercase protocol verb (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `GRPC`, `TOPIC`, `QUERY`) is parsed as structure, and anything else is kept verbatim as an opaque payload, so `[op1/op2/op3]` and `[legacy-mainframe-txn-44]` are equally valid annotations. See `docs/decisions/action-operation-brackets.md` for the full design.
  - Surfaces as `Action.Operation` (`{verb, payload, text}`). The field is omitted entirely when absent, so existing golden files are unchanged.
- **Qualified `<domain>/<name>` references are accepted wherever a bounded context is named**, not only in the `asks` target: action subjects for all four kinds, the `returns` target, and the `when ... listens` trigger context.
- **`craft/sema/ambiguous-bc` fires for use cases.** A bare bounded-context name owned by two or more domains was previously dropped silently from the dependency graph. It is now an error naming the candidates. It fires at four sites: `sync_action` subject and target, `async_action` subject, and the `domain_listen` trigger context. It does not fire for internal actions or for `returns`, which do not participate in the dependency graph.
- **`craft/syntax/empty-op-annotation`.** An empty `[]` is an error rather than being silently dropped.
- **`craft fmt`.** The formatter is now a CLI command, not only an LSP request. `craft fmt <files...>` formats in place; `craft fmt --check <files...>` writes nothing, lists every file that is not already formatted, and exits non-zero, which is the CI gate. Arguments are paths or globs (including `**`) resolved exactly as `craft validate` resolves them; directories are not walked. A file the parser cannot fully place is never rewritten, and is reported as skipped with the diagnostic that blocked it rather than being passed over in silence.
- **`craft fmt` column-aligns operation annotations** per contiguous run within a scenario. A non-annotated action does not reset the run; a blank line or a new scenario does.
- **Public API additions in `pkg/craft`**: the `Operation` type alias, the `OpVerbGET` through `OpVerbQUERY` constants, and `ProtocolVerbs()`.
- **Editor support**: protocol verb completion at the head of an annotation, and distinct semantic-token classification so the annotation recedes from the business prose.

### Fixed
- **Formatting now guarantees it changes whitespace and nothing else.** Every non-whitespace byte of the input appears in the output, in the same order. The check runs inside `FormatDocument` on every call: if the output would lose, duplicate or reorder any content, the input is returned untouched and a `craft/internal/formatter-content-drift` diagnostic says so. `craft fmt` reports such a file as skipped instead of writing it, and `craft fmt` additionally reparses its own output before touching the disk.
  - This is the structural fix, not another bug fix. `FormatDocument` rebuilds each declaration from typed accessors, so every construct needs its own branch and a construct without one was dropped in silence. That is how twelve separate defects arose. The guarantee turns the next missed construct into a harmless no-op that reports itself, rather than a file quietly losing content.
- **`textDocument/formatting` had twelve pre-existing defects, all found while building this release.** Each silently mutated a valid document, and none had test coverage before now. Formatting is now verified over **every** `.craft` file in the repository, not just the hand-written fixtures: whitespace-only, reparse-clean, idempotent, model-preserving and comment-preserving.
  - Format Document deleted operation annotations outright.
  - Format Document rewrote typed event refs into the deprecated quoted form, so `Billing notifies billing.ChargeSucceeded` became `notifies "billing.ChargeSucceeded"` and then tripped `craft/lint/deprecated-string-ref`. Present since the typed-ref form was introduced.
  - Format Document reflowed `returns to <target> <phrase>` such that the target was lost from the reparsed model.
  - Format Document mangled punctuation in trigger phrases, so `when User creates (1! & 2!)` became `( 1 ! & 2 ! )`.
  - Format Document corrupted qualified references, rendering `when re/billing listens vas.VasApplied` as `when re / billing listens vas.VasApplied`, which no longer parsed.
  - Format Document corrupted every `context_map` and `glossary` block: it split qualified endpoints and term nodes (`billing/Invoice` became `billing / Invoice`, and a three-segment `re/billing/Invoice` became `re / billing / Invoice`) and collapsed every statement in the block onto one line. Both make the block unparseable. These are the blocks where qualified refs are the native spelling, so they were the worst hit.
  - Format Document split a qualified value in any field, so a service's `repo: olxeu/realestate/subscriptions` came back as `olxeu / realestate / subscriptions`. Fixed in the shared renderer rather than per block, so exposure and domain values are covered too.
  - Format Document deleted every `tags { }` block in a use case, along with the qualified refs inside it.
  - Format Document deleted every comment in the document. Line and block comments are trivia, and every renderer read the token list that excludes trivia. `internal/visualizer/testdata/vas.craft` lost all 47 of its comments to a single format request.
  - Format Document deleted a comment on the last line of a file, and truncated a file that was nothing but a comment to zero bytes. Anything after the last real token is folded into a single whitespace token, so those comments carry no comment token kind and no filter on kind could see them.
  - Format Document deleted a comment written between a field's `:` or `,` and its value, as in `repo: // note` with the value on the next line.
  - Format Document reported success while doing all of the above. `craft fmt` exited 0 on a file it had just emptied.
- **Format Document no longer rewrites a document the parser could not fully place.** A construct the parser cannot place reports `craft/syntax/not-yet-implemented`, which is only warning-severity, so it slipped past the formatter's error-only bail-out and the tree got re-rendered anyway, dropping and duplicating text. That bail-out now covers the code.
- **Format Document no longer panics on an unbalanced `}`.** A stray closing brace produces only a warning, so it reached the indentation logic at depth 0 and crashed with `strings: negative Repeat count`, taking down the request.

### Notes
- **No generated diagram changes.** No visualizer reads `Action.Operation`, exactly as none read `Action.Ref`. Every generator still renders `Context`, `TargetContext`, `Connector`, and `Phrase` only. The annotation is stored for tooling that consumes the contract; rendering it is a separate decision.
- The `<phrase>` tail still accepts `! & * / # ? +` unquoted. Only a line-final bracketed run is now reserved.
- **Known limitation: braces in a phrase disable the annotation on that line.** A balanced `{...}` in an action's phrase and a trailing `[...]` annotation cannot both be recognised on the same line, because the annotation scan and the phrase scan have to agree on where the line ends and the phrase scan stops at the first `}`. `Billing asks Gateway to charge {amount} [POST /pay]` parses with a phrase of `charge {amount` and reports the rest, rather than as an annotated action. Braces **inside** the annotation are fine, which is the case that matters in practice: `[POST /v1/accounts/{id}/charges]` parses correctly. The diagnostic now says which of the two to move.
- **Formatting canonicalises layout, and preserves everything inside a statement verbatim.** Indentation becomes 2 spaces per level, each statement gets its own line, top-level declarations are separated by a blank line, and a comment gets a line of its own at the indent of what it precedes. Within a statement, nothing is respaced: `A asks B for (1! & 2!)` and `billing   customer_supplier   vas` keep the spacing you wrote. This is why files written with 4-space indent or wrapped `contexts:` continuations are reported by `craft fmt --check`. They are not broken, they are simply not in canonical form.
- **Known limitation: qualified `<domain>/<name>` references have no hover or go-to-definition.** `internal/sema` resolves bounded contexts by bare name, so `re/billing` resolves to nothing and the editor offers nothing on it. This is awkward now that `craft/sema/ambiguous-bc` tells authors to qualify an ambiguous name: the spelling the tool recommends is the one the tool cannot navigate. Pre-existing for `asks` targets, and now reachable in more slots because qualified refs are accepted in more slots. Diagnostics, parsing, and diagram generation are unaffected; only navigation is. A follow-up will teach the resolver the qualified form.

## [2.15.2] — 2026-08-05

### Fixed
- **`textDocument/semanticTokens/full` no longer crashes on files containing non-ASCII characters.** Editing a `.craft` file with an accented name, an em dash in a comment, or an emoji produced `internal error: runtime error: slice bounds out of range` for every semantic-token request, so syntax highlighting silently died for that file.
  - Cause: the lexer scans `[]rune`, so `Token.Column` counts runes, but its doc comment claimed bytes and the parser fed that column to `LineIndex.Offset`, which adds it to a *byte* line-start. On a line with multi-byte characters, every token after the first one got a byte start that was too small. When the drift pushed a token's computed start behind the previous token's end, `emitWhitespaceBefore` dropped the negative gap while `prevEnd` still advanced, so those bytes were counted twice and the green tree ended up wider than the source. Red-tree offsets accumulate green widths, so the trailing tokens reported offsets past EOF; converting one to an LSP position sliced the source out of range.
  - `lexer.Token` now carries a true byte `Offset`, and the parser builds the green tree from it. The tree-width invariant (`root.Width() == len(src)`) holds for non-ASCII sources again, which also restores the lossless round-trip: `internal/visualizer/testdata/vas.craft` previously reassembled 3 bytes longer than the original.
- **Incremental `didChange` edits no longer corrupt documents containing non-ASCII characters.** `applyIncrementalChange` treated LSP `Position.Character` (a count of UTF-16 code units) as a byte column, so an edit on a line with an accented character or emoji was applied at the wrong offset, sometimes splitting a rune and leaving invalid UTF-8 in the server's copy of the buffer, which then disagreed with the editor.
- **Completion resolves the cursor correctly on non-ASCII lines.** The same UTF-16-as-bytes confusion in `treeEnclosingBlock` and in the line-prefix slice could pick the wrong enclosing block.
- **Every position the LSP publishes is now a real UTF-16 column.** `internal/sema` derived symbol and diagnostic columns with `LineIndex.LineCol`, which counts *bytes*, and then emitted them as LSP `Position.Character`, which counts UTF-16 code units. On any line containing a multi-byte character, everything to the right of it was reported too far right by one column per extra byte. This affected go-to-definition targets and hit-testing, document symbols, hovers, all sema and lint diagnostic ranges, semantic-token lengths, and the end position of the whole-document formatting edit. Range *ends* were equally wrong: they were computed as `start + len(name)`, a byte count.
  - Every column bound for LSP now comes from the new `LineIndex.LineCol16`, and every span length from `green.UTF16Len`. Semantic-token lengths, `rawLen` cursor hit-tests, and the formatting edit's end position all moved over.
- **Semantic-token lengths were byte counts.** A token containing an accented character was highlighted several columns too wide, bleeding the colour onto whatever followed it.

### Added
- `green.LineIndex.OffsetFromUTF16(line, utf16col)`: the inverse of `UTF16Col`, and the only correct way to turn an LSP position into a byte offset. `Offset(line, col)` remains byte-in/byte-out and must not be used for LSP positions.
- `green.LineIndex.LineCol16(offset)`: `LineCol` with the column in UTF-16 code units. Every column destined for an LSP position must come from this.
- `green.UTF16Len(s)`: a string's width in UTF-16 code units, for computing range ends and token lengths.
- `LineIndex` now carries the source it was built from, so `UTF16Col`/`OffsetFromUTF16` can no longer be handed a mismatched string. Both lost their `src` parameter; `Src()` exposes it for callers that were threading it separately.
- `UTF16Col` clamps an out-of-range offset instead of panicking. A position-conversion bug should degrade one token, not take down the request handler.
- **Tree-width invariant check in `syntax.Parse`.** If the green tree's width ever again disagrees with `len(src)`, the parser emits `craft/internal/tree-width-mismatch` naming both numbers, instead of the corruption surfacing as a slice-bounds panic several layers away.
- Non-ASCII coverage: lossless and tree-width cases in `internal/syntax`, UTF-16 round-trip cases in `internal/green`, and an `internal/lsp` regression test that runs `SemanticTokensFull` over every `.craft` file in `testdata/corpus`, `examples`, and `internal/visualizer/testdata`, checking that every emitted position exists in the source. The previous test corpus was entirely ASCII, which is why the invariant went unguarded.
- **ASCII-twin oracle** (`internal/lsp/utf16_twin_internal_test.go`). A non-ASCII fixture is paired with an ASCII document built to be identical in UTF-16 columns (`Ação`/`Acao`, `café`/`cafe`, `🚀`/`xx`), and `DocumentSymbol`, `Definition`, `Hover`, `Formatting`, `SemanticTokensFull`, and both per-file and workspace diagnostics must return identical geometry for the two. A self-check asserts the fixture really is UTF-16-identical and byte-different, and each comparison fails loudly if it collected nothing to compare. Any future byte column or byte length reaching an LSP position breaks this immediately.

### Notes
- `Token.Column` is unchanged and still counts runes, which is what the parser's same-line and adjacency checks want. Only the byte-offset consumers moved to `Token.Offset`, and only the LSP-bound columns moved to `LineCol16`.
- Diagnostic and definition *positions* on files containing non-ASCII characters change (they were wrong; they are now right). Codes, severities, messages, and counts are unchanged. Pure-ASCII files are byte-for-byte unaffected.

## [2.15.1] — 2026-07-27

### Fixed
- **`craft validate`, `craft check`, and `pkg/craft` now report sema and lint diagnostics at their real source position.** Every diagnostic from these paths carried line 0 / column 0 (printed as `file:1:` by the CLI), so a `deprecated-string-ref` warning about line 40 pointed at line 1, and multiple warnings in one file were indistinguishable. Some lints were worse than useless: `dead-event` leaked a raw byte offset as its column.
  - Cause: `pkg/craft`'s `parseOne` called `sema.AnalyzeFile` without the `LineIndex`, and `ParseFiles` additionally omitted the per-file `LineIndex` map from `sema.LintWorkspace`. Both were deliberate during the LSP migration (plan D3 kept CLI diagnostic bytes identical while the parser was swapped underneath) and were never revisited after the swap completed. `internal/workspace` always passed its indexes through, so **the LSP and the VS Code extension were never affected** — this was a CLI/library-only defect.
  - Positions now match what the LSP publishes for the same file. Affects `craft/lint/deprecated-string-ref`, `craft/sema/malformed-slug`, `craft/lint/dead-event`, `craft/lint/event-not-past-tense`, the `context_map` cross-validation lints, and every other position-bearing sema/lint diagnostic.
  - Regression tests in `pkg/craft/parse_test.go` assert positions for both the per-file and workspace-lint paths; they derive the expected line/column from the source text rather than hardcoding numbers.

### Notes
- Diagnostic *positions* from the CLI change (they were wrong; they are now right). Codes, severities, messages, and counts are unchanged. Any downstream consumer that pinned to the old line-1 behaviour — or that parsed `craft validate` output positionally — should expect real line numbers now.

## [2.15.0] — 2026-07-21

### Changed (breaking, within the `service` block)
- **The `opslevel:` service anchor is renamed to `catalog_ref:`.** The property is unchanged in meaning and shape — it is still the service's stable identifier in the org's service catalog, still optional, still at most once per service — but the language no longer names the catalog vendor. Which catalog resolves the anchor is deployment configuration, not part of the grammar, so the catalog can be swapped without a DSL migration. The name is `catalog_ref`, not `catalog_slug`: this field is an *immutable identity anchor*, and "slug" is reserved in this design's vocabulary for *mutable* human-readable names.
  - **The old `opslevel:` spelling is removed, not deprecated.** It no longer parses; a service block declaring it now gets a `craft/syntax/unexpected-token` error, the same as any other unknown service field. There is no alias and no migration period — rename the property in your `.craft` files.
  - **`model.Service.OpsLevel` → `model.Service.CatalogRef`**, exported through `pkg/craft.Service`. The JSON tag changes from `opsLevel` to `catalogRef` (still `omitempty`), so `craft inspect --json`, the LSP, and every `.craftjson` golden move with it.
  - `internal/syntax` follows: `ServiceDecl.OpsLevel()` → `ServiceDecl.CatalogRef()`, `ServiceField.IsOpsLevel()` → `ServiceField.IsCatalogRef()`, `SyntaxKindKwOpsLevel` → `SyntaxKindKwCatalogRef`.
  - The `craft/sema/duplicate-service-anchor` diagnostic keeps its code and severity; only the field name in its message text changes (`service "Foo": "catalog_ref" is already declared; only one `catalog_ref:` is allowed per service`).
- **`repo:` is unchanged** — same spelling, same shape, same `model.Service.Repo` / `repo` JSON key. It already bound by identity (a repo slug, never a checkout path) and needed no migration.

### Added
- **Code anchors are documented in the language reference.** `docs/page/language/services.md` gained a `catalog_ref:` / `repo:` property reference and a "Code Anchors" section stating the governing principle — **bind by identity, never by location** — and the division of labour: craft validates anchor *shape* only; resolving an anchor against a real catalog or repository is the consuming system's job. The anchors shipped in v2.9.0 but had never been covered outside the authoring skill.

### Notes
- Minor, not major: the version line continues at 2.15.0. `pkg/craft`'s stability promise exists to protect downstream consumers, and the sole consumer of the anchor field is first-party and is being bumped in lockstep — so the rename is taken outright rather than carried as a dual-spelling alias. Consumers reading `Service.OpsLevel` must move to `Service.CatalogRef`; consumers reading the `opsLevel` JSON key must move to `catalogRef`.
- The `tree-sitter-craft` grammar renames its `opslevel_property` rule to `catalog_ref_property` and its highlight query to match; its corpus-compat pin (`CORPUS_VERSION`) bumps to `v2.15.0` at release time.

## [2.14.0] — 2026-07-16

### Added
- **`pkg/craft.ParseDir(fsys fs.FS, root string) (*CraftDoc, []Diagnostic, error)`** — recursively collects every `*.craft` file under `root` in `fsys` and parses them together through `ParseFiles`, giving directory-wide parsing (cross-file resolution + `LintWorkspace` diagnostics) without the caller hand-assembling a file map. Takes an `fs.FS`, so `os.DirFS(dir)`, `embed.FS`, `fstest.MapFS`, and `fs.Sub` subtrees all work; the walk is fully recursive (arbitrary directory depth) and `*.craft`-only. Diagnostics remain data; the `error` return is reserved for I/O failures during the walk. No grammar or CLI surface change (the CLI already parses trees via recursive globs, e.g. `craft validate 'dir/**/*.craft'`).

### Fixed
- **`ParseFiles`/`ParseDir` now merge `glossary { }` relations across files.** `mergeDoc` omitted `CraftDoc.Glossary`, so any multi-file parse silently dropped glossary relations when the `glossary` block lived in a different file from other declarations — no diagnostic emitted, just missing data. All `CraftDoc` slices are now merged, matching the merge contract's stated "all CraftDoc slices are merged." Regression introduced with the `glossary` block in v2.13.0.

## [2.13.0] — 2026-07-16

### Added
- **`glossary { }` block for cross-context term relations.** A dedicated view for ubiquitous-language terms, restoring the term-relation capability removed from `context_map` in v2.12.0. Statements are `<term_node> <verb> <term_node>`, where a term node is `<bc>/<term>` or `<domain>/<bc>/<term>` — the last `/`-segment is the term name, the segment(s) before it are the bounded-context identity (resolved exactly like a `context_map` endpoint). Three verbs, all **symmetric**: `same_as`, `contrasts`, `distinct_from`. Blocks are repeatable and optionally domain-scoped (`glossary re { … }`); all merge into `CraftDoc.Glossary []TermRelation` (`model.TermRelation{Left,Verb,Right}`, `omitempty`). `glossary` is a contextual keyword. Term *definitions* are deliberately out of scope — nodes are referenced, never declared.
  - Resolution diagnostics: `craft/sema/glossary-endpoint-not-bc` (error), `craft/sema/glossary-unresolved-bc` (warning), `craft/sema/glossary-ambiguous-bc` (error), `craft/sema/glossary-self-relation` (error), plus `craft/lint/glossary-redundant` (warning — same unordered pair + verb repeated) and `craft/lint/glossary-conflicting-relation` (warning — a pair asserted `same_as` and also `distinct_from`/`contrasts`).
  - Exports `pkg/craft.TermRelation` (alias of `model.TermRelation`) and `GlossaryVerbs() []string`, single-sourced from `internal/syntax` and kept in sync by a build-time test. Language reference at `docs/page/language/glossary.md`.
- **`context_map` cross-validation lint.** Warns when a declared strategic relationship contradicts the communication view craft infers from use-case `asks`/`notifies`. Both primitives reduce to one directed **dependency edge `D → U`** ("downstream depends on upstream"): sync `X asks Y` yields `X → Y`; async `Y notifies E` paired with `when X listens E` yields `X → Y`. With LEFT = upstream, every directional pattern expects `RIGHT → LEFT`. Four signals: `craft/lint/separate-ways-violation` (warning — `separate_ways` pair that nonetheless communicates), `craft/lint/relationship-direction-inverted` (warning — the only observed dependency runs the wrong way), `craft/lint/relationship-bidirectional` (hint — an asymmetric pattern with traffic both ways, "consider `partnership`?"), and `craft/lint/unclassified-communication` (hint — a communicating pair with no `context_map` edge). Runs in `LintWorkspace`, so it surfaces in both the LSP and `pkg/craft` with no new public API. Absence of communication never warns (partial-view principle). Documented in `docs/page/language/context-map.md`.

### Notes
- Additive/minor: both features are new syntax and new diagnostics; no existing export, JSON field, or diagnostic changes. `model.Edge` is unchanged. The `tree-sitter-craft` grammar gains an additive `glossary_block` rule; its corpus-compat pin (`CORPUS_VERSION`) bumps to `v2.13.0` at release time.

## [2.12.0] — 2026-07-16

### Changed (breaking, within `context_map`)
- **`context_map` is redesigned as the DDD strategic relationship view.** Its edge verbs are now the eight canonical DDD context-mapping patterns — directional `customer_supplier`, `conformist`, `anticorruption_layer`, `open_host_service`, `published_language` (convention: LEFT endpoint is upstream) and symmetric `partnership`, `shared_kernel`, `separate_ways` — one pattern per statement. The previous realization/term edge verbs (`realized_by`, `also_realizes`, `same_as`, `contrasts`, `distinct_from`) are **removed**; realization is derivable from a service's `contexts:`, and term relations are deferred to a future `glossary {}` block (separate spec).
- **Endpoints are bounded contexts named bare (`billing`) or domain-qualified (`re/billing`).** The `bc:`/`service:`/`term:` kind prefix is gone inside `context_map`; `/` denotes node identity and `.` remains reserved for events.
- **Blocks are repeatable and optionally domain-scoped** (`context_map re { … }`); all blocks merge into `CraftDoc.ContextMap`. `model.Edge{Left,Verb,Right}` is unchanged.

### Added
- **Exported edge-verb metadata API** in `pkg/craft`: `EdgeVerbInfo{Verb, Class, UpstreamRole, DownstreamRole, Symmetric}`, `EdgeVerbs()`, `LookupEdgeVerb(verb)`, and `EdgeClass`/`EdgeDirectional`/`EdgeSymmetric`. Single-sourced from `internal/syntax` and kept in sync with the parser vocabulary by a build-time test.
- **Endpoint-resolution diagnostics** (resolved against the merged workspace symbols): `craft/sema/edge-endpoint-not-bc` (error — endpoint is a domain/service/actor, not a BC), `craft/sema/self-relationship` (error), `craft/sema/ambiguous-bc` (error — bare name is a BC in ≥2 domains; qualify it), `craft/sema/unresolved-bc` (warning), and the `craft/lint/redundant-relationship` warning for a duplicated symmetric pair.

### Notes
- The old `craft/sema/edge-endpoint-kind` diagnostic (bc→service / term→term kind checks) is removed along with the old verbs. The `context_map` language reference is documented at `docs/page/language/context-map.md`. The `tree-sitter-craft` grammar is updated to match; its corpus-compat pin (`CORPUS_VERSION`) bumps to `v2.12.0` at release time.

## [2.11.0] — 2026-07-15

### Added
- **`UseCase` now carries `Line` and `SourceURI`** (`json:"line,omitempty"` / `"sourceUri,omitempty"`), parity with `Service`/`Actor`/`Action`. `Parse` stamps the filename you pass; `ParseFiles` stamps each use case's originating map key — so a consumer can attribute a use case to its file without re-scanning text.
- **Generic `tags {}` sub-block inside `use_case`** → `UseCase.Tags map[string]string`. An opaque, consumer-defined key/value slot (`tags { journey: re/renewal-flow  owner: "team billing" }`); craft neither validates keys nor interprets values. Values may be bare slugs or quoted strings. `tags` is a contextual keyword (still usable as an ordinary identifier). `Tags` is absent from JSON when no block is present.
- `syntax.EdgeKeywords()` accessor exposing the `context_map` edge verbs.

### Fixed / Hardened
- `internal/sema` edge-verb validation gained a defensive `default:` case (`craft/sema/unrecognised-edge-verb`) plus a build-time test keeping the parser's edge-keyword set and sema's verb maps in sync.
- A duplicate tag key or a second `tags {}` block emits `craft/sema/duplicate-tag` (warning; last value wins).

### Notes
- Additive/minor: existing keyed struct literals and JSON consumers are unaffected. Corpus goldens gained `use_case` `line` fields (regenerated).

## [2.10.1] — 2026-07-15

### Fixed
- **Module path now declares its major version, so `pkg/craft` is actually importable.** Go's semantic import versioning requires a v2+ module to end its path in `/v2`; without it, `go get github.com/tcarcao/craft/pkg/craft@v2.10.0` failed. The module is now `github.com/tcarcao/craft/v2`, so the public API is imported as:

  ```go
  import "github.com/tcarcao/craft/v2/pkg/craft"
  ```

  `go get github.com/tcarcao/craft/v2/pkg/craft@v2.10.1`. Only the import path changes — the API, types, JSON output, and CLI are identical to v2.10.0. (The v2.10.0 CLI binaries, Homebrew tap, and Docker image were unaffected; only the Go-library import was broken.)

## [2.10.0] — 2026-07-14

### Added
- **Importable parse API in `pkg/craft`.** External Go modules can now `import "github.com/tcarcao/craft/pkg/craft"` and parse Craft source in-process instead of shelling out to the CLI:
  - `craft.Parse(filename string, src []byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — single file; every diagnostic's `SourceURI` is normalized to the `filename` you pass (uniform with `ParseFiles`, no `file://` prefix).
  - `craft.ParseFiles(files map[string][]byte) (*craft.CraftDoc, []craft.Diagnostic, error)` — multi-file merge plus cross-file resolution and lint. Files are processed in ascending filename order and each diagnostic batch is stable-sorted, so the merged model and the diagnostic slice are deterministic; each diagnostic's `SourceURI` is the map key you supplied.
  - Diagnostics are returned as data; the `error` slot is reserved for programmer errors. Neither function does file I/O.
- `LICENSE` file (MIT), matching the README's stated license.
- `pkg/craft` now declares a real stability contract (semver, stable as of 2.10.0). Requires Go 1.25+ to import.

### Changed
- `pkg/craft` is no longer marked "Experimental". The `CraftDoc`/`Diagnostic` type *definitions* moved to an internal leaf package (`internal/model`) and are re-exported from `pkg/craft` as type aliases — identity and JSON tags are unchanged, so no existing output or importer is affected.
- `cmd/craft`'s `check`, `inspect`, and `validate` are now thin wrappers over `pkg/craft.Parse`/`ParseFiles` rather than duplicating the parse+sema orchestration, guaranteeing the CLI and library cannot drift.
- `validate`'s workspace-level diagnostic lines now print the bare filename (consistent with per-file lines) instead of a `file://`-prefixed path.
- `check --lsp-json`'s actor symbol list now derives from the parsed document rather than the raw syntax tree, so a malformed name-only actor (no resolvable type) is excluded from symbols, consistent with the canonical model, instead of being emitted with an empty type. Its diagnostics also now carry a uniform `sourceUri` (the file path passed to `check`) rather than a mix of empty and `file://`-prefixed values.

### Notes
- No changes to the Craft language, grammar, or semantics. Diagnostic positions are byte-identical to v2.9.0.

## [2.9.0] — 2026-07-14

### Added
- **Craft DSL vNext grammar** — a backward-compatible evolution toward typed, code-anchored references (ticket 018):
  - **Flexible unquoted narrative** — step prose accepts special characters without quotes (`X asks Y for 1! & 2! and/maybe *`). A comment now begins only at a whitespace-preceded `//`, so `http://api`, `50/50`, and `and/maybe` stay in prose.
  - **Typed references** on `notifies`/`listens`/`asks` — event refs (`notifies vas.VasApplied`) and node slugs `[kind:][namespace/]name` (`bc:re/subscriptions`, `term:billing/dunning`, `service:subscriptions-api`). Slug segments may be words that lex as keywords (e.g. `term:user/account`).
  - **`context_map { }` block** with five typed edges: `realized_by`, `also_realizes` (bc → service), `same_as`, `contrasts`, `distinct_from` (term ↔ term).
  - **Service anchors** `opslevel:` and `repo:` (identity, not file paths).
  - **Local validation** — `craft validate` emits `craft/sema/malformed-slug`, `craft/sema/edge-endpoint-kind`, `craft/sema/duplicate-service-anchor` (errors) and `craft/lint/deprecated-string-ref`, `craft/sema/unresolved-ref-local` (warnings). Cross-validation against code facts remains the hub's job.
- Authoring docs updated: the `craft-dsl` skill and the VitePress guide now teach the vNext syntax.

### Changed
- Quoted event strings (`notifies "X"` / `listens "X"`) still parse but are **deprecated** — they emit `craft/lint/deprecated-string-ref`. Migrate to typed refs.

### Fixed
- Quoted string literals now round-trip byte-for-byte (a pre-existing corruption dropped the opening quote and duplicated the last content character; the lossless round-trip test had been silently scanning zero corpus files).
- Flexible-prose punctuation (e.g. `(1! & 2!)`) no longer gains spurious spaces in generated diagram/JSON output.
- Quoted service/domain declaration names now resolve correctly in validation and LSP (duplicate detection, diagnostics, rename).

---

## [2.8.2] — 2026-05-14

### Added
- Three new server endpoints exposing Mermaid source for the VS Code extension to render client-side:
  - `POST /preview/mermaid/domain` → Mermaid `flowchart LR` source
  - `POST /preview/mermaid/sequence` → Mermaid `sequenceDiagram` source
  - `POST /preview/mermaid/c4` → Mermaid `c4Diagram` source
- All three mirror the existing `/preview/{domain,c4}` JSON envelope. `PreviewResponse.Data` carries plain text (not base64) — the client renders the source itself.

---

## [2.8.1] — 2026-05-14

### Fixed
- Docker server image (`tiagocarcao/craft`) build for v2.8.0 failed because `build/package/Dockerfile` pinned `golang:1.23-alpine` for the builder stage, while v2.8.0's `go.mod` requires Go ≥ 1.25. Bumped the builder image to `golang:1.25-alpine`. No code changes from v2.8.0; CLI binaries and Homebrew tap for v2.8.0 were unaffected.

---

## [2.8.0] — 2026-05-14

### Added
- **Per-use-case filtering and splitting.** `craft generate` gains `--use-case <slug-or-name>[,...]` (filter detailed-domain + sequence to specific use cases) and `--split` (emit one file per use case). Use case names are matched by either the exact (quoted) name or a deterministic kebab-case slug; the same slug is used as the filename suffix in split mode.
- **Mermaid output format.** New `--format puml|mermaid|mermaid-md` flag selects the output format. `mermaid` writes `.mmd` files (raw Mermaid source); `mermaid-md` writes `.md` files with the source wrapped in a fenced ` ```mermaid ` block plus a `# title` heading — ready to commit alongside docs for GitHub-rendered inline preview. PlantUML (`puml`) remains the default and is byte-identical to previous releases.
- **Stdout output.** `--stdout` prints the generated diagram to stdout instead of writing to a file. Mutually exclusive with `--output`, `--split`, `--type all`, and multi-file invocations. Useful for `craft generate ... --stdout | pbcopy`.
- **`.md` no-clobber safety.** `--format mermaid-md` refuses to overwrite an existing `.md` file by default (those are often hand-written docs). `--force` opts in to overwriting. `.puml` and `.mmd` keep silent-overwrite behaviour.
- **Mermaid generator.** Three generators in `internal/visualizer/mermaid/`: `Sequence` (→ `sequenceDiagram`), `Domain` (→ `flowchart LR`, both detailed and architecture modes), `C4` (→ Mermaid's experimental `c4Diagram`).
- **`make test-integration` target.** New Makefile target runs build-tag-gated integration tests against real PlantUML and mermaid-cli renderers via testcontainers-go. Auto-detects podman on macOS and configures `DOCKER_HOST` / `TESTCONTAINERS_RYUK_DISABLED` accordingly.

### Fixed
- **PlantUML stdlib include drift.** Removed `!include <tupadr3/devicons2/rust>` from generated C4 PUML — that path does not exist in the PlantUML stdlib, breaking strict renderers (PlantUML 1.2025+). Rust now correctly resolves via `tupadr3/devicons/rust`. Language sprite includes are also now conditional: only languages actually declared by services in the model emit their include line, instead of all 11 supported sprites every time.
- **`skinparam handwritten` deprecation banner.** Domain and sequence diagrams emitted `skinparam handwritten false`, which PlantUML 1.2025+ flags with a visible yellow banner above every rendered diagram. Removed (the value was already the default).
- **Unicode em-dash mojibake.** Use case titles containing `—` rendered as Latin-1 mojibake in PlantUML output. Generators now declare `skinparam defaultFontName SansSerif` to select a system font with full Unicode coverage.
- **Bounded contexts rendered as actors.** When a use case trigger named a bounded context (`when VASFulfillment starts ...`), the detailed-domain generator emitted both a domain frame AND a stickman actor for the same name. Now checks against declared bounded contexts and treats the trigger as internal.
- **Architecture-mode self-loops.** Each domain in the architecture-mode domain diagram showed an unlabeled self-arrow. Self-loop edges are now filtered at emission.
- **CRON missing from C4 actors.** The C4 generator hard-coded a skip for any actor whose name began with `CRON`. Removed; CRON now appears like any other system actor.
- **C4 actor fan-out.** `Interacts directly` edges fanned out from every actor to every domain in services they were involved with, producing N×M clutter. Now scoped to the specific domain each actor actually triggers in its scenarios.
- **C4 event edges deduplicated by pair only.** Multiple events between the same domain pair (publisher → Event_Queue → listener) collapsed to a single edge. Dedup key now includes the event description, so all events render.
- **Mermaid Domain detailed had no trigger edges.** External triggers (`when CRON advances schedule`) now produce a labeled edge from the actor to the first action's context; listen-triggers route through the publishing domain.
- **Mermaid Domain and Sequence included unreferenced actors.** Both generators previously emitted every declared actor as a node/participant regardless of whether any included scenario used them. Now restricted to actors that trigger at least one scenario in the (possibly filtered) doc — especially noticeable in split-mode files.

### Changed
- **Go toolchain floor raised to 1.25.0.** A transitive dependency of `testcontainers-go` requires Go ≥ 1.25. Run `go install` / `make test` with Go 1.25 or newer.
- **`craft generate` documented behaviour.** The skill reference (`.claude/skills/craft-dsl/references/cli-reference.md`) and public CLI docs (`docs/page/tooling/cli.md`) now cover all new flags with examples.

### Notes
- Default invocation (`craft generate file.craft`) produces byte-identical PlantUML output to v2.7.0. All new behaviour is opt-in.
- Integration test coverage now exercises every diagram type against a real renderer (PlantUML server for `puml`, mermaid-cli for `mermaid`/`mermaid-md`) instead of only asserting structural string properties.

---

## [2.5.2] — 2026-04-27

### Fixed
- Numbers are now valid in action phrases. `asks X to check 3 transfer limits` previously produced a parse error on the numeric token; `collectPhrase` now collects `TokenNumber` alongside identifiers and strings.

---

## [2.4.0] — 2026-04-24

### Removed
- ANTLR parser fully removed — `--parser=antlr` flag no longer exists on `craft check`, `craft generate`, or `craft inspect`.
- `?parser=` query parameter removed from the HTTP server.
- `github.com/antlr4-go/antlr/v4` dependency removed.

### Changed
- `cmd/craft/validate` now uses the v2 semantic analysis layer (`internal/sema`) for all lint checks.
- `cmd/craft/inspect` output now uses `pkg/craft.CraftDoc` types.

---

## [0.1.0] — 2026-04-24

### Added
- Hand-written Go parser (`--parser=v2`) covering the full Craft grammar: actors, domains, services, use cases, arch blocks, and exposures.
- LSP server (`craft lsp`) with diagnostics, document symbols, hover, semantic tokens, go-to-definition, folding ranges, and `workspace/executeCommand` for domain/service extraction.
- Island parsing and error recovery — broken blocks do not cascade errors across the file.
- `$/setTrace` / `$/logTrace` trace-level support for LSP client debugging.
- `craft check --lsp-json` CLI flag for reproducing LSP responses without a running server.
- Acceptance corpus at `testdata/corpus/` with 80+ `.craft` files and paired `.craftjson` goldens.

### Changed
- CLI executable renamed from `craft-cli` to `craft`. The Homebrew name is also updated to `craft`.
- `craft check` and `craft generate` now default to `--parser=v2`. Use `--parser=antlr` as an escape hatch.
- HTTP server (`craft server`) defaults to the v2 parser; `?parser=antlr` query param available as escape hatch.

### Notes
- The ANTLR parser (`--parser=antlr`) remains available as an escape hatch and will be removed in `0.2.0`.
- macOS users: if Gatekeeper blocks the downloaded binary, run `xattr -dr com.apple.quarantine /path/to/craft` once. Code signing is planned for `0.2.0`.
