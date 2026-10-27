# Board Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Draw the scenario board from the Confluence macro (inline, git and draft modes) and from a VS Code command.

**Architecture:** The craft server learns `view: "board"` on git-source renders, sharing one use-case chooser with `/preview/board/domain`. The macro adds a "Scenario board" type with a use-case choice. The extension adds `craft.previewBoard`, which posts the active document to `/preview/board/domain` and shows the SVG in a webview.

**Tech Stack:** Go 1.25 (craft), plain ES modules with esbuild (craft-confluence), TypeScript with jest (craft-vscode-extension).

**Spec:** `docs/superpowers/specs/2026-10-27-board-parity-design.md`

## Global Constraints

- Repos: craft `/Users/tiago.carcao/projects/poc/craft-project/craft`, macro `/Users/tiago.carcao/projects/poc/craft-project/craft-confluence`, extension `/Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension`.
- Each repo works on its own branch `board-parity`, created from `main`. Commit there, and never push.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- `/preview/board/domain` answers `data` as plain SVG text. `/sources/{name}/render` answers `data` as base64 for every view, board included.
- No `forge deploy`, no extension publish, no tags. Leave `lspVersion` and the extension `version` unchanged.
- Gates: craft `go test ./...` and `go vet ./...`; extension `npm test` and `npm run compile`; macro `npm run build`.
- Use CONTEXT.md vocabulary (View, Backend, Model, Participant, Interaction) in craft code and comments.

## Review Focus

1. A git-mode board with zero or two ticked use cases. Expect the status error `A board draws one use case: tick exactly one.` and no server call (Task 2, manual check in the build output's logic).
2. Re-saving a stored inline non-board macro must not re-render: `renderKey` must hash exactly as before when `useCase` is empty (Task 2).
3. A draft whose use case exists only in git: the server resolves it against the merged model (Task 1 test).
4. `useCases` naming two use cases on a board render is a 400, not a silent pick of the first (Task 1 test).
5. The VS Code cursor sitting outside every use case in a multi-use-case file shows a QuickPick, not an error (Task 3 test of the pure helper).

---

### Task 1: Craft server: board on git-source renders

**Files:**
- Modify: `cmd/server/main.go` (`handlePreviewBoardDomain`, around line 336)
- Modify: `cmd/server/sources.go` (`sourceViews` at line 380, `handleSourceRender`, `SourceRenderRequest.View` comment)
- Test: `cmd/server/board_test.go`, `cmd/server/sources_test.go`
- Modify: `CHANGELOG.md` (`[Unreleased]`)

**Interfaces:**
- Produces: `POST /sources/{name}/render` accepts `{"view":"board", "useCases":[<one name or slug>]?, "dsl"?, "path"?, "refs"?, "format":"svg"|""}` and answers `SourceRenderResponse` with base64 SVG `data`. Tasks 2 relies on this.
- Produces (internal): a helper, e.g. `boardUseCase(doc *craft.CraftDoc, want string, preferred []string) (name string, err error)`. `err` is a client error whose `Error()` is the 400 message.

- [ ] **Step 1: Write failing tests** in `sources_test.go`, using the existing `sourceServer`/`post` helpers:
  - `TestSources_RenderBoardOneUseCase`: `view:"board"`, `useCases:[<one name>]`, `format:"svg"` → 200, and base64-decoded `data` starts with `<svg`.
  - `TestSources_RenderBoardRejects`: two names in `useCases` → 400 containing `a board draws one use case`; `format:"png"` → 400 containing `svg only`; an unknown name → 400 containing `available:`.
  - `TestSources_RenderBoardDraftDefaultsToDraftUseCase`: a `dsl` with one use case and no `useCases` → 200, and the decoded SVG contains that use case's name (the board title is `Scenario: <name>`).
- [ ] **Step 2: Run** `go test ./cmd/server/ -run 'Board'`. Expected: FAIL (`unknown view "board"`).
- [ ] **Step 3: Implement.**
  - Extract the use-case choice out of `handlePreviewBoardDomain` into the helper, keeping its messages: `the model has no use case to draw`; `no use case matches %q (available: …)`; `more than one use case is named %q; rename one to draw it as a board`.
  - The empty-`want` default is the first of `preferred` that exists, else the model's first use case.
  - `handlePreviewBoardDomain` calls it with `preferred = nil`, so its behaviour is unchanged.
  - In `handleSourceRender`, handle `req.View == "board"` before the generic path:
    - Reject a format other than `""`/`"svg"` with `the board writes svg only`.
    - Reject `len(req.UseCases) > 1` with `a board draws one use case`.
    - Call the helper with `want = UseCases[0]` (or `""`) and `preferred = useCasesIn(doc, req.Path, req.DSL != "")`.
    - Render `visualizer.Request{View: ViewFlow, Backend: visualizer.Board, Format: FormatSVG, UseCases: []string{name}}` and answer as the other views do (base64).
  - Add `"board"` to the `View` comment.
- [ ] **Step 4: Run** `go test ./... && go vet ./...`. Expected: PASS, and the existing `TestPreviewBoard_*` tests are unchanged and green.
- [ ] **Step 5: CHANGELOG** `[Unreleased]` → `### Added`: "**`POST /sources/{name}/render` draws boards**: `view: "board"` with at most one `useCases` entry (default: the draft's or file's first use case, else the model's first); `data` is base64 SVG like every source render."
- [ ] **Step 6: Commit** on branch `board-parity`: `feat(server): board view on git-source renders`.

### Task 2: Confluence macro: "Scenario board" type

**Files:**
- Modify: `src/config.js`, `static/config/index.html`, `docs/usage.md` (all in craft-confluence)

**Interfaces:**
- Consumes: Task 1's `view: "board"` on `/sources/{name}/render`, and the existing `/preview/board/domain` (`{dsl, useCase}` → plain SVG `data`).
- Produces: macro config fields `type: 'board'` and `useCase: <name>` (inline and draft).

- [ ] **Step 1:** `index.html`: add `<option value="board">Scenario board</option>` after Sequence. Add a labelled `<select id="useCase">` in the same form area as the type, hidden by default.
- [ ] **Step 2:** `config.js`:
  - Add `TYPES.board = { label: 'scenario board', plain: true, request: (dsl, useCase) => ['/preview/board/domain', { dsl, useCase }] }`. The other entries ignore the extra argument.
  - `render()` decodes `json.data` with `atob` unless the current request is an inline `plain` type. Git and draft board renders stay base64.
- [ ] **Step 3:** The use-case `<select>`:
  - It is visible when the type is `board` and the mode is not `git`.
  - Options are the `use_case "…"` names in `source()`. They refresh on editor change, keeping the current value if it is still there, else the first. The initial value is `cfg.useCase`.
  - Inline `request()` passes `$('useCase').value`. Draft passes `useCases: [$('useCase').value]`.
  - Git mode with `board` sends `useCases: checked()`.
- [ ] **Step 4:** In `guard`-wrapped Preview and Save, when the type is board and the mode is git and `checked().length !== 1`, set the status error `A board draws one use case: tick exactly one.` and return before `render()`.
- [ ] **Step 5:** `renderKey(type, dsl, useCase = '')` hashes `` `svg\n${type}\n${dsl}` `` plus `` `\n${useCase}` `` only when `useCase` is non-empty. Every inline caller passes the board's use case (and `''` for other types). Save stores `useCase` in config for inline and draft.
- [ ] **Step 6:** `autoTitle`: for board the subject is the chosen use case (the select's value, or the single ticked one in git), giving `` `${subject}: scenario board` ``. Refresh the title when the select changes.
- [ ] **Step 7:** `docs/usage.md`: add a short "Scenario board" paragraph covering what it draws (one use case as a Miro-style board), how the use case is chosen in each mode, and that it is SVG.
- [ ] **Step 8: Run** `npm run build` in craft-confluence. Expected: both bundles build with no errors. Do not commit `static/*/main.js` if `.gitignore` ignores it; check `git status`.
- [ ] **Step 9: Commit** on branch `board-parity`: `feat: scenario board diagram type`.

### Task 3: VS Code extension: `craft.previewBoard`

**Files:**
- Create: `client/src/commands/craftText.ts` (`mask` and `closingBrace` moved from `previewJourney.ts`, plus `useCaseNames` and `useCaseAt`)
- Create: `client/src/commands/craftText.test.ts`
- Create: `client/src/commands/previewBoard.ts`
- Modify: `client/src/commands/previewJourney.ts` (import the moved helpers), `client/src/commands/index.ts` (register), `client/src/commands/previewCommon.ts` (export `saveDownloadedFile` if needed), `package.json` (`contributes.commands`), `CHANGELOG.md` (`[Unreleased]`)

**Interfaces:**
- Consumes: `POST /preview/board/domain` `{dsl, useCase}` → `{success, data: <plain svg>, diagnostics}`.
- Produces: `useCaseNames(text: string): string[]` (in document order, the quoted names of `use_case "…" {` blocks, ignoring strings and comments via `mask`); `useCaseAt(text: string, line: number): string | undefined` (the use case whose block holds `line`, 0-based).

- [ ] **Step 1: Write failing tests** in `craftText.test.ts`:
  - `useCaseNames` of a text with `use_case "A" {…}` and `use_case "B" {…}` → `['A','B']`.
  - A `use_case "X"` inside a `//` comment or a string is not listed.
  - `useCaseAt(text, <line inside B>)` → `'B'`.
  - `useCaseAt(text, <line between blocks>)` → `undefined`.
- [ ] **Step 2: Run** `npx jest client/src/commands/craftText.test.ts`. Expected: FAIL (module missing).
- [ ] **Step 3: Implement** `craftText.ts`. Move `mask` and `closingBrace` there unchanged, and make `previewJourney.ts` import them. `pickDefaultSlug` and its tests are unchanged.
- [ ] **Step 4: Run** `npm test`. Expected: PASS, including `previewJourney.test.ts`.
- [ ] **Step 5: Implement** `previewBoard.ts`, with a `registerPreviewBoard(context)` registered in `index.ts`:
  - Active editor must be `craft`, else `No active Craft file.`
  - `names = useCaseNames(text)`. None → info `No use case in this file.`
  - Choose `useCaseAt(text, cursor line)`, else the single name, else `window.showQuickPick(names, { title: 'Craft: Preview Scenario Board' })`. Cancel → stop.
  - `axios.post(`${serverUrl}/preview/board/domain`, { dsl: text, useCase }, { timeout })` with `getCraftConfig()`. Errors go to `handlePreviewError`.
  - Reuse one panel `craftBoard`, titled `Craft Board: <useCase>`, beside the editor, with `enableScripts: true`. The HTML shows `<img src="data:image/svg+xml;base64,…">` in a scrollable wrapper (`max-width: none`) and one "Download SVG" button that posts `{ command: 'downloadBoard' }`.
  - The extension handles it by saving the SVG it holds through `saveDownloadedFile` (export it) with default file name `<slug>.board.svg`. The slug is the lower-cased name with runs of non-alphanumerics replaced by `-`, trimmed of `-`.
- [ ] **Step 6:** `package.json` `contributes.commands`: `{ "command": "craft.previewBoard", "title": "Craft: Preview Scenario Board" }`. The extension has no editor menus, so it is palette only. Add any activation event the other preview commands use.
- [ ] **Step 7:** `CHANGELOG.md` `[Unreleased]` → `### Added`: "**Craft: Preview Scenario Board** draws the use case under the cursor (or one you pick) as a Miro-style board, from the craft server's `/preview/board/domain` (craft 2.29.0 or later), and saves it as SVG."
- [ ] **Step 8: Run** `npm test && npm run compile && npm run lint`. Expected: all pass. If lint already fails on `main` for unrelated files, say so and confirm no new lint errors.
- [ ] **Step 9: Commit** on branch `board-parity`: `feat: preview a use case as a scenario board`.
