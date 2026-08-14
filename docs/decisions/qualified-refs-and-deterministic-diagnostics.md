# Qualified use-case refs, and deterministic diagnostics

Status: shipped in v2.17.1 (2026-08-14).

Two independent bugs, found by running `craft validate` over a 193-file
real-world corpus (an architecture-documentation repo, ~543 use_cases). Between
them they accounted for essentially all of that corpus's reported findings. A
third instance of bug 2, in `MergeWorkspaceSymbols`, was found while writing the
regression tests and is recorded below.

Files touched: `internal/sema/sema.go`, `internal/sema/lint.go`, `pkg/craft/parse.go`.

---

## 1. A domain-qualified participant did not resolve

`resolveUseCaseRef` only ever looked a participant up by its bare name. A
participant written as `<domain>/<name>` fell through every branch and was
reported as unknown.

That is a bug rather than a style restriction, for two reasons:

- The grammar defines the use-case participant slot as the same `bc_ref`
  production that `context_map` endpoints use, and `resolveBCRef` has always
  accepted both the bare and the qualified form. One half of the language
  implemented what the grammar says; the other half did not.
- Qualification is the *only* way to write some references. A bounded context
  whose name is declared in two domains cannot be named unambiguously in the
  bare form. Rejecting the qualified form makes those references unwritable.

The fix adds a qualified branch at the top of `resolveUseCaseRef`: split on the
first `/`, resolve the domain, confirm it declares that bounded context, and
return a `bounded_context` target. Positions stay indexed by bare BC name, as
elsewhere.

**Effect on the corpus: 940 findings → 98.** 842 of the 940 were this one bug.

## 2. Diagnostics were nondeterministic — and not only in their order

The headline symptom was that two runs over identical input returned the same
diagnostics in a different order. The real defect underneath was worse.

Every lint rule walks the workspace file by file while accumulating into a map
keyed by something *coarser* than the file — `published[event]`,
`reported[event]`. The first file to reach a given key is the
one recorded; every later file with the same key is suppressed as a duplicate.
Because the walk was `for uri, tree := range perFileTrees`, which file won that
race was decided by Go's map seed.

(`used[actor]` in `lintUnusedActors` is *not* one of these: it is a plain set,
so the walk order does not affect it. That rule's attribution comes from
`ws.Actors[name].URI` instead, which is the third instance below.)

So the output did not merely shuffle. **The set of findings changed between
runs**: on the corpus, 16 of 98 findings named a different file and line on each
run — same rule, same event, different blame. The finding *count* was identical
every time, which is exactly what makes this easy to miss and what let it live
this long.

Three changes:

- **`sortedTreeURIs` helper**, used at all seven `range perFileTrees` sites.
  This makes "the first file wins" a deterministic statement about the corpus
  instead of a statement about the map seed. This is the fix that matters; the
  other two are completeness.
- **`Message` tiebreak in `sortDiags`.** Without it the comparator returns
  "equal" for two diagnostics sharing a position and a code, and `SliceStable`
  faithfully preserves whatever order the upstream map iteration produced. The
  tiebreak makes the comparator a total order.
- **One global `sortDiags` before `ParseFiles` returns.** Each block was sorted
  on its own, which ordered diagnostics *within* a block but said nothing about
  the concatenation of blocks.

**Verification: byte-identical output across 8 consecutive runs** on the 193-file
corpus (previously: 8 distinct outputs from 8 runs).

## 3. The same race in `MergeWorkspaceSymbols`

Found while writing the regression tests for bug 2, and fixed in the same
release. This one sits a layer below the lint rules and is reachable from the
LSP as well as the CLI.

`MergeWorkspaceSymbols` folded the per-file symbol tables together with
`for _, syms := range perFile` and last-write-wins into maps keyed by bare name.
So a name declared in two files had a winner chosen by the map seed. That winner
is what every downstream consumer treats as *the* declaration:

- `ws.Actors[name].URI` and `.Line` are the file and line an `unused-actor`
  finding is reported against, which is why the doc for bug 2 originally listed
  that rule. The rule's own `used` set is order-independent; this is the actual
  source.
- `ws.BoundedContexts[bc]` is the domain a *bare* BC reference resolves to.
- `ws.BCPositions[bc]` is where go-to-definition jumps, including from the
  qualified refs bug 1 just enabled.

Measured directly: 200 merges of a two-file fixture split roughly 28/172 between
the two files, on all three maps.

The fix iterates in ascending URI order and keeps the **first** declaration, for
every symbol kind. Sorting alone would not be enough — with last-write-wins the
winner becomes the alphabetically *last* file, which contradicts the rule the
duplicate-service branch in the same function already documented ("Keep the
first declaration in the map so resolution is stable").

### Known limitation, not addressed here

`BCPositions` is keyed by bare BC name at both the per-file and workspace level.
When one BC name is declared in two domains, all of its references share a single
position, so go-to-definition on `ops/Billing` can land on `re`'s declaration.
The fix above makes that choice deterministic; making it *correct* means keying
positions by `domain/name`, which is a separate change.

Separately, `ws.Domains[name]` still overwrites rather than merges, so a domain
whose bounded contexts are split across two files silently loses one file's
worth. Now deterministic, still lossy. Also out of scope here.

---

## Testing

`go test ./...` — 1094 passed across 14 packages, 0 failures. `go vet ./...`
clean.

No test covered any of the three bugs. All are workspace-scale behaviours: the
resolution bug needs a BC name declared in two domains, and the attribution bugs
are invisible unless you run the same multi-file workspace twice and compare.
16 regression tests were added, and each was confirmed to fail against the
un-fixed code before being kept:

| File | Covers |
|---|---|
| `internal/sema/qualified_usecase_ref_test.go` | bug 1: qualified refs resolve, select the named domain, carry the BC position, work cross-file; unknown domain and non-member BC stay unresolved; bare refs still resolve |
| `internal/sema/lint_determinism_test.go` | bug 2: a dedup key is blamed on the first file in sorted order, and repeated runs produce the same finding *set* |
| `internal/sema/merge_determinism_test.go` | bug 3: first-in-sorted-order wins for actors, BCs, positions and use cases, and the winner follows sort order rather than insertion order |
| `pkg/craft/parse_determinism_test.go` | end to end: 25 runs byte-identical, count stable, slice totally ordered, message tiebreak exercised |

End-to-end check on the repository's own `examples/`: 6 of 7 runs differed from
the first before the fix, 8 of 8 identical after, with the finding count
unchanged at 218 throughout. That is the same signature as the 193-file corpus,
reproduced on a workspace that ships with the repo.

## Release note

Shipped as **v2.17.1**, a patch. `pkg/craft`'s exported API is unchanged, so no
consumer stops compiling, and there are no new features: this is three bug fixes.
Behaviour reachable through the API does change in two visible ways: documents
that previously reported errors now validate, and the diagnostic slice is now
stably ordered and stably attributed. Anyone pinning golden diagnostic output
will see it move once, to a correct value.

## One caveat worth carrying forward

Any file:line attribution captured from a `craft validate` run *before* v2.17.1
may name the wrong file, for the dedup-keyed rules specifically
(dead-event, unused-actor, event-past-tense). The finding itself was real; the
location may not have been. Anything downstream that recorded those locations
should be re-derived from a current run rather than trusted.

The same applies, via bug 3, to any recorded go-to-definition target or
bare-bounded-context resolution for a name that is declared in more than one
file.
