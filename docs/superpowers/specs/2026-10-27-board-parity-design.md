# Board parity: Confluence macro and VS Code extension

**Status:** approved to build (2026-10-27).
**Builds on:** `2026-10-26-flow-board-backend-design.md` (the `board` Backend, CLI `--format board`, `POST /preview/board/domain`).

## Why

The board (the flow View drawn Miro-style as SVG, one use case per board) only reaches people through the CLI and a raw server endpoint. People use it in two places:

- **Design pages:** pasting a scenario board into a Confluence page, next to the prose, and keeping it current from git.
- **While editing:** checking the use case they are writing in VS Code, the way they already preview the domain flow.

Both clients already handle SVG: the Confluence macro renders and stores every diagram as an SVG attachment, and VS Code webviews show SVG like any image. So this work wires the board into each client's diagram menu. It does not need a new format.

## Goals

1. Confluence macro: "Scenario board" is a diagram type in every mode: inline ("Written here"), git ("From git") and draft ("Written here, on top of git").
2. VS Code extension: a command that previews the board of one use case of the active file, and saves it as SVG.
3. Craft server: git-source renders can draw a board, which Confluence's git and draft modes need.

## Non-goals

- Boards from the VS Code Domains/Services tree views (they select domains, but a board is one use case).
- Auto-refreshing the VS Code preview on edit. No other preview auto-refreshes.
- Multi-file models in the VS Code board preview. Like `craft.previewDomain`, it sends the active document's text only.
- PNG/PDF boards. The board is SVG only, everywhere.
- Deploying the macro (`forge deploy`), publishing the extension or tagging releases. Those are separate steps the owner runs or approves.

## Anti-goals

- Changing what the board looks like. Layout and notation are settled in the backend spec.
- Changing the response shape of an existing endpoint.

## Constraints

- Vocabulary from `CONTEXT.md`: View, Backend, Model, Participant, Interaction.
- `docs/decisions/diagram-pipeline-deepening.md` is settled and stays closed.
- `POST /preview/board/domain` answers `data` as **plain SVG text** (not base64). `POST /sources/{name}/render` answers `data` as **base64** for every view; the board follows that rule there too.
- Craft has no CI: run `go test ./...` before a release. The extension runs `npm test` (jest) and `npm run compile`. The macro has no tests: `npm run build` must succeed.

## Design

### 1. Craft server: `view: "board"` on `POST /sources/{name}/render`

- `sourceViews` gets `"board"` → the flow View, drawn by the `Board` Backend with `FormatSVG`.
- `format` must be `""` or `"svg"`. Anything else is a 400 (`the board writes svg only`).
- A board draws **exactly one use case**, chosen in this order:
  1. `useCases` given: it must name exactly one use case, by name or slug. More than one is a 400 (`a board draws one use case`). A name that matches no use case is a 400 that lists the available slugs, as `/preview/board/domain` does.
  2. `useCases` empty: the first use case that `useCasesIn(doc, path, dsl != "")` returns (the draft's own, or the file's).
  3. Otherwise, the first use case of the model.
  4. A model with no use case is a 400 (`the model has no use case to draw`).
- Two use cases that share a slug are a 400, as on `/preview/board/domain`.
- The response is the usual `SourceRenderResponse`, with `data` base64, `contentId` and `commits`.
- The use-case choice is shared with `handlePreviewBoardDomain`: extract one helper that both handlers call. Its error messages stay as they are today.
- CHANGELOG `[Unreleased]`: an Added line. `pkg/craft` is unchanged.

### 2. Confluence macro (`craft-confluence`)

- `TYPES` gains `board: { label: 'scenario board', … }`. `static/config/index.html` gains `<option value="board">Scenario board</option>` after Sequence.
- **Inline mode** posts to `/preview/board/domain` with `{ dsl, useCase }`. The `data` in the reply is plain SVG, so `render()` must not `atob` it. Mark the type (e.g. `plain: true`) and decode by that flag.
- **Git and draft modes** post to `/sources/{name}/render` with `view: 'board'`. The reply is base64, as for every other view.
- **Choosing the use case:**
  - *Inline and draft:* a "Use case" `<select>`, shown only when the type is board. It lists the `use_case "…"` names in the editor text (the regex `autoTitle` already uses), refreshes as the text changes, and keeps the current choice while it still exists. Otherwise it falls back to the first. Inline sends `useCase: <name>`; draft sends `useCases: [<name>]`.
  - *Git:* the existing checkbox picker. With board selected, exactly one use case must be ticked. Otherwise Preview/Save stop with the status error `A board draws one use case: tick exactly one.` and make no server call.
- The chosen use case is stored in the macro config as `useCase` (inline and draft). Git already stores `useCases`.
- **The inline render hash must cover the use case.** `renderKey(type, dsl)` becomes `renderKey(type, dsl, useCase = '')`, and `useCase` joins the hashed text only when it is non-empty. That way the hashes of stored non-board diagrams don't change, and they don't re-render.
- **Auto title:** for board, the subject is the chosen use case (the ticked one in git mode), giving `"<use case>: scenario board"`.
- `docs/usage.md` gets a short "Scenario board" paragraph: what it draws, one use case, SVG.

### 3. VS Code extension (`craft-vscode-extension`)

- **New command** `craft.previewBoard`, titled "Craft: Preview Scenario Board", in a new module `client/src/commands/previewBoard.ts`, registered beside the other previews.
- **Choosing the use case:**
  - If the cursor is inside a `use_case "…" { … }` block of the active `.craft` document, use that use case.
  - Otherwise, if the document has one use case, use it.
  - If it has several, show a QuickPick of their names.
  - If it has none, show an info message (`No use case in this file.`) and stop.
  - Finding the use case under the cursor reuses `mask`/`closingBrace` from `previewJourney.ts`. Move them into a small shared module rather than copying them. The function `useCaseAt(text, line): string | undefined` and `useCaseNames(text): string[]` are pure and unit-tested with jest.
- **Request:** `POST ${serverUrl}/preview/board/domain` with `{ dsl: <document text>, useCase: <name> }`, using the same timeout and error handling (`handlePreviewError`) as the other previews. `data` is plain SVG text.
- **Panel:** one reused webview panel ("Craft Board: <use case>"), beside the editor. The SVG is shown as an `<img>` with a `data:image/svg+xml;base64,…` source, inside the same scrollable wrapper the domain preview uses, so the board is never shrunk to fit. One button, "Download SVG", posts a message back. The extension saves the SVG it already holds through the existing save flow (`saveDownloadedFile`, exported if needed) with default name `<use-case-slug>.board.svg`. There is no second server call.
- **Discoverability:** the command appears in the palette, and in the editor title and context menus when `editorLangId == craft`. If the extension has no such menus yet, palette only.
- `CHANGELOG.md` `[Unreleased]`: an Added line.
- `lspVersion` and the extension version stay as they are. Bumping them belongs to the release step.

## Settled after the builder check (my calls)

- **Use cases are matched by name or slug across the whole model, never by file.** `useCasesIn` returns names. Git mode sends no `path`, and draft mode sends the name from the editor. The server resolves it against the merged model, so a draft may name a use case that exists only in git, and that is fine. Shared slugs stay a 400, as today.
- **The fallback chain, spelled out:** the names `useCasesIn` returns (the draft's or the file's use cases) are tried first, and the first of them wins. If that list is empty, the model's first use case wins. `/preview/board/domain` has no path or draft, so it keeps its current behaviour: the model's first use case.
- **The macro does not clear a stored `useCase`** when the type changes away from board. It does no harm. A stored board whose use case was renamed in git gets the server's 400 on the next Preview/Save, shown in the status line. The page view (`view.js`) shows the stored attachment and never re-renders, so it needs no change.
- **`mask`/`closingBrace` are generic.** `pickDefaultSlug` already uses them to find `use_case` blocks. They move into a shared module unchanged.
- **The VS Code extension doesn't pre-check duplicate names.** The server's 400 is shown through `handlePreviewError`.
- **The macro has no CHANGELOG.** `docs/usage.md` is its only doc change.

## Left to the builder

- Element ids, CSS and exact copy beyond what is quoted above.
- The helper's name and signature in the craft server.
- Test shape in Go (follow `cmd/server`'s existing handler tests) and jest (follow `previewJourney.test.ts`).

## As built

- Server: `boardUseCase` in `cmd/server/main.go` is the one use-case chooser. `renderSourceBoard` in `cmd/server/sources.go` handles `view: "board"`.
- Macro: a draft board saves `useCases: checked()` like every draft type, plus its own `useCase`, and its render request sends `useCases: [<chosen>]`. An inline or draft board whose editor has no `use_case` stops with the message `Add a use_case to the model to draw a board.` The use-case list matches only `use_case` lines, not ones inside comments. `docs/usage.md` gives the server versions it needs.
- Extension: `craftText.ts` holds `mask`, `closingBrace`, `useCaseNames` and `useCaseAt`. `previewBoard.ts` has no unit test, because the vscode mock has no webview or QuickPick APIs.
