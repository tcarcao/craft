# AGENT.md — Operating Rules for the LSP-Migration Agent

> **Status:** Draft — P0.8 deliverable of the LSP migration (`docs/decisions/lsp-migration-plan.md`).
> **Audience:** the AI agent executing slices S1–S13 of `docs/decisions/lsp-migration-tracer-bullets.md`.
> **This document is rules, not tutorial.** Tutorials, background, and decision rationale belong in `docs/decisions/*`. Grammar reference belongs in `docs/GRAMMAR.md`. Architecture reference belongs in `docs/ARCHITECTURE.md`. This file is the **"what you may and may not do"** in the migration.
> This file supersedes `CLAUDE.md` for migration work. `CLAUDE.md` stays in force for everyday queries; when the two disagree within migration scope, this file wins.

---

## 1. The red-green-refactor loop

Every PR-sized task during Track A / S3–S8 follows the same shape:

1. **RED** — pick the next un-skipped file from the current slice's corpus subdirectory (or add a new one). Run Harness A (`go test ./internal/parser_diff/... -run TestV2VsGoldens`). It fails — that is the goal of this step.
2. **GREEN** — write the **smallest** code change that makes Harness A pass for that file while keeping every previously-passing file green. No scope creep.
3. **REFACTOR** — with Harness A green, clean up naming, duplication, and awkward shapes. Re-run the harness after each refactor.
4. **HARNESS B** — verify Harness B (ANTLR-vs-goldens) still green everywhere. If you broke it, you touched something you should not have.
5. **COMMIT** — one logical step per commit. The working tree is green at every commit boundary (see §4).

For slices outside Track A (S0, S2, S9–S13), the loop still holds but the oracle changes: S2 uses the LSP integration test, S9 uses `testdata/broken/*.diagnostics.json`, S10 uses the benchmark budget, etc. Pick the slice's stated oracle and drive red-green against it.

---

## 2. The Harness A regression rule (Q11)

**Every PR MUST show Harness A green on every un-skipped corpus entry.**

Concretely:

- Corpus files are the implementation frontier ledger. The `skipped` marker in `internal/parser_diff/v2_test.go` is **only** valid within the current slice's declared scope.
- A PR that un-skips a new file in its target subdirectory MUST make that file pass, not mark it skipped-again.
- A PR MUST NOT move the `skipped` marker backwards (re-skipping a file that passed on main). Fix the underlying regression instead.
- Harness B MUST remain green on main at every commit; any PR that reddens Harness B is rejected regardless of Harness A status.

CI enforces both harnesses as hard blocks. There is no `--allow-fail` flag.

---

## 3. The capabilities-per-handler rule (Q20)

When you implement an LSP handler, you MUST update the `initialize` response's `ServerCapabilities` in the same PR. Conversely, you MUST NOT advertise a capability whose handler does not exist.

- Each PR that adds a handler lands a unit test asserting the capability appears in `initialize`.
- Each PR that retires a handler (S12 style) lands a test asserting the capability is no longer advertised.
- `internal/lsp/capabilities_test.go` is the home for these assertions.

This rule prevents VSCode UI features from being silently dark or silently broken.

---

## 4. The green-between-commits rule

Every commit on a migration branch MUST satisfy:

- `go build ./...` succeeds.
- `go test ./...` passes. Skipped tests are acceptable; failing tests are not.
- `go vet ./...` is clean.
- If the commit touches LSP code, a minimal `initialize` → `shutdown` integration test continues to pass.

**Intermediate WIP commits are not acceptable.** If a change needs more than one commit to land safely, structure it so each commit is independently green. Stash experimental spikes locally; only commit when green.

---

## 5. The CraftDoc contract rule

- `pkg/craft/craftdoc.go` is experimental until v0.1. Breaking changes are allowed during S1–S11b but MUST:
  1. Be titled `CraftDoc: <short description>` in the PR title.
  2. Regenerate every committed `.craftjson` golden via the ANTLR adapter in the same PR.
  3. Re-run Harness B and commit the regenerated goldens only after verifying the diff is exactly what the schema change intended.
  4. Link the PR in `docs/decisions/lsp-migration-plan.md` Q11 notes for post-hoc review.
- After v0.1 (S11b), `CraftDoc` is stable: backward-compatible additions only.
- Internal AST types under `internal/ast/` are **private** and change freely.

---

## 6. Per-slice checklists

Each slice in `docs/decisions/lsp-migration-tracer-bullets.md` enumerates its own acceptance criteria. Do not diverge from them. The checklists below summarise recurring steps across every grammar slice (S3–S8).

### 6.1 Grammar slice (S3–S8) template

- [ ] Corpus subdirectory populated (`testdata/corpus/NN_kind/`) with ≥8 `.craft` files covering the kind's normal and edge cases.
- [ ] Paired `.craftjson` goldens generated via ANTLR adapter; reviewed by Tiago in the PR (per Q2 drip-feed policy).
- [ ] v2 parser extended to handle the new construct; Harness A green on the newly un-skipped subdirectory.
- [ ] Harness B still green everywhere.
- [ ] Sema rule(s) scheduled for this slice (see `docs/ARCHITECTURE.md` §8.2) implemented with matching broken-corpus entries.
- [ ] `ServerCapabilities` extended in the same PR if the slice adds a new LSP handler.
- [ ] `craft check --lsp-json` output coverage extended to include the new handler's response (S3+).
- [ ] Visitor-test parity audit (Q10): every assertion in `internal/parser/*<kind>*_test.go` covered by a corpus entry or back-filled as one.
- [ ] Paired `tree-sitter-craft` PR (Q12) updating `grammar.js` for the new construct. Merge order: craft first, then tree-sitter.
- [ ] `docs/DIAGNOSTICS.md` entries added for every new diagnostic code.
- [ ] Dogfood screenshot or terminal capture in the PR description showing the feature live.

### 6.2 HITL slice (S0, S11a, S11b, S12) template

- [ ] Deliverables produced (drafted by agent).
- [ ] Explicit "ready for Tiago review" tag in the PR description.
- [ ] **Do not merge** without Tiago's approving comment. Auto-merge rules do not apply.
- [ ] External credentials (Apple signing for S11b, Marketplace/OpenVSX tokens for S11a/b) are held by Tiago; the agent requests usage, it does not execute.

### 6.3 Infrastructure slice (S1, S2, S9, S10, S13) template

- [ ] Cross-layer seam is demoable end-to-end.
- [ ] Every new CI job is hard-block unless explicitly designated warning-only (tree-sitter drift in `craft`'s CI per Q12).
- [ ] The slice's dependency claim from the tracer-bullets graph is respected — do not start a slice whose upstream is unmerged on main.

---

## 7. Never-do list

These are hard rules. Violations block merge.

1. **Never edit files under `pkg/parser/`** (ANTLR-generated).
2. **Never edit `tools/antlr-grammar/Craft.g4`** during migration — grammar is frozen at v1 through v0.1 cut. Grammar-v2 is a separate post-v0.1 workstream.
3. **Never introduce a new dependency on `antlr4-go/antlr/v4`**. Anything new belongs in the hand-written parser.
4. **Never import `internal/lsp` from outside `cmd/`.** (Layer rule; see `ARCHITECTURE.md` §2.)
5. **Never import `internal/workspace` from `internal/sema`.** (Layer rule.)
6. **Never bootstrap `testdata/broken/*.diagnostics.json` from ANTLR.** Error messages are what we are replacing; bootstrapping from ANTLR locks us into ANTLR-quality diagnostics. Hand-authored only (source-plan P0.5).
7. **Never bypass `recover()` tiers.** Adding a fifth recovery boundary requires updating `ARCHITECTURE.md` §6 and this doc in the same PR.
8. **Never commit a red Harness A or Harness B.** See §4.
9. **Never mark a previously-passing corpus entry as `skipped`** to get a PR green. Fix the regression.
10. **Never touch `examples/*.craft`** during migration. They are frozen through v0.1 (tracer-bullets Q18). Modify `testdata/corpus/99_mixed/` copies instead.
11. **Never publish to Marketplace or OpenVSX.** These actions belong to Tiago in S11a/S11b.
12. **Never advertise an LSP capability whose handler is a no-op or panics.** See §3.
13. **Never ignore `go vet` output.** Treat it as a failing build.
14. **Never rename or restructure a public `pkg/craft/` type without following §5.**
15. **Never skip hooks or bypass signing** on `git commit` or `git push`. If a hook fails, fix the underlying issue.
16. **Never force-push to `main`.** Feature-branch force-pushes are fine; `main` is protected.
17. **Never delete a legacy test (`internal/parser/*_test.go`)** before the S3–S8 visitor-test parity audit has back-filled its assertions as corpus entries. The bulk removal is S12.
18. **Never touch `CLAUDE.md` from within an S0 PR.** The pointer to this doc is added in a **later** PR once S0 has merged.

---

## 8. Local commands you will run

```bash
# regenerate ANTLR parser (needed only if you touch the generated layer — which you should not)
make generate-grammar

# full test run
go test ./...

# differential harness (A = v2 vs goldens, B = ANTLR vs goldens)
go test ./internal/parser_diff/...

# benchmark (S10 gate)
go test -bench . -run '^$' ./internal/syntax/

# LSP subcommand smoke test
go build -o /tmp/craft ./cmd/craft
/tmp/craft lsp --stdio < testdata/lsp/initialize.jsonrpc

# canonical-output dump (useful during Track A debugging)
/tmp/craft check --parser=v2 --lsp-json path/to/file.craft
/tmp/craft check --parser=antlr --lsp-json path/to/file.craft
/tmp/craft diff-parsers path/to/file.craft
```

macOS dogfood note: the VSIXs built by CI through S11a are unsigned (Q8 defers signing to S11b). After installing a dogfood VSIX on macOS, run once:
```bash
xattr -dr com.apple.quarantine "$HOME/.vscode/extensions/<id>/server/craft"
```

---

## 9. Commit, PR, and branching hygiene

- One slice = one working branch, off `main`. Name: `migration/s<N>-<short-slug>` (e.g. `migration/s3-actors-end-to-end`).
- Each sub-deliverable is its own commit; see §4. Squash-merges are acceptable; rebase-merges are preferred so intermediate commits stay green on main.
- PR titles follow `[S<N>] <subject>`. Scope creep outside the declared slice rejects the PR.
- PR descriptions include: slice ref + source-plan ref + acceptance-checklist + dogfood note/screenshot if UI-facing.
- Co-author tag on every commit:
  ```
  Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
  ```
  (If a different agent model lands commits, replace the model name accordingly.)
- Never auto-commit design or spec docs (`docs/**.md`) without explicit user request. Draft them, then wait for sign-off.

---

## 10. When you get stuck

- **The red stays red.** Narrow the corpus file to a minimal reproducer and commit it to `testdata/corpus/NN_kind/`; then ask for help in the PR description. Do not silently widen `skipped`.
- **Harness B goes red on main.** Stop. Open a dedicated PR reverting the adapter regression; do not layer more work on top of red main.
- **CraftDoc shape surprises.** See §5. Schema changes are allowed during migration but must regenerate all goldens in the same PR.
- **LSP client misbehaves.** `$/setTrace=verbose` + `$/logTrace` is your primary tool (landed in S11a, but available earlier in local dev). If the server appears stuck, check the handler-tier recovery log — a panic may have left a handler returning `null` silently.
- **You are unsure whether a decision applies.** `docs/decisions/lsp-migration-plan.md` (source plan) + `docs/decisions/lsp-migration-tracer-bullets.md` (slicing) are authoritative. If they disagree (they should not after 2026-04-22 rev), the tracer-bullets doc wins for *how*, the source plan wins for *what*.

---

## 11. Cross-references

- **Source plan:** `docs/decisions/lsp-migration-plan.md`
- **Tracer-bullet slicing:** `docs/decisions/lsp-migration-tracer-bullets.md`
- **Grammar spec:** `docs/GRAMMAR.md`
- **Architecture + layer rules:** `docs/ARCHITECTURE.md`
- **Diagnostic codebook:** `docs/DIAGNOSTICS.md` *(seeded S3+, consolidated S9)*
- **Default repository rules:** `CLAUDE.md` *(pointer to this doc added in a post-S0 PR)*
