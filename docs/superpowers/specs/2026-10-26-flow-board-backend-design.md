# Board backend for the flow view — design

Status: ready for approval · 2026-10-26

## Intent

Teams draw use-case collaboration diagrams by hand in Miro with a small,
friendly notation: bounded contexts as clouds, systems as cogs, actors as
person icons, and every message as a coloured sticky note beside its arrow
(green query, blue command, yellow event), numbered in order, with a notation
legend on the side. People like that look better than what PlantUML or
Mermaid produce for Craft's **flow** view, even though the content is the
same. The goal is for Craft to draw the flow view in that style itself, from
`.craft` source, so the hand-drawn Miro board is no longer needed.

The throwaway spike that settled the look is at
`docs/superpowers/specs/assets/2026-10-26-flow-board-poc.html` (copy of the
scratchpad POC; open it in a browser). The reference Miro board is the
"Invoicing process starts for a prepaid Customer" scenario reproduced in the
POC's data. **The POC is the visual spec**: shapes, colours, sizes and
placement rules below come from it.

## Goals

- A new **Backend**, `board`, that writes the existing **flow** View as a
  standalone SVG file in the Miro-like style.
- Same content and numbering as the flow view: every flow edge appears, with
  the same step number.
- **One board per use case**, like the hand-drawn Miro boards.
- Available from the **CLI** and the **server preview** in the first release.
- Deterministic output: the same model and request give byte-identical SVG.

## Non-goals (this release)

- VS Code extension and Confluence macro support (follow-up releases; the
  VS Code release channel follows minor-version parity, so it ships after
  craft).
- `cmd/server/sources.go` `sourceViews` / git-source rendering (feeds
  Confluence; later).
- Other views (c4, architecture, sequence, surface) in the board style.
- PNG/PDF output. SVG only.
- Interactive or editable diagrams, export to Miro.

## Anti-goals

- **Shapes that look off.** The first attempt at this failed on the look of
  the shapes. Clouds, cogs and notes must match the POC, not approximate it.
  A generic graph look (boxes, rounded rectangles, Graphviz styling) is a
  failure even if the layout is correct.
- **A new runtime dependency for CLI users.** No Graphviz, Node, browser or
  Java needed to produce a board. The pipeline ruling removed Graphviz from
  the Go code (`docs/decisions/diagram-pipeline-deepening.md`, delivery
  step 0); do not bring it back.
- **Silently dropping things.** Per ruling 16, an option the board cannot
  honour is rejected with `ErrUnsupported`, never ignored.

## Constraints

- Pipeline stays as is: analysis → `internal/interaction` →
  `internal/visualizer/view` → backend writer. The board writer is a sibling
  of `plantuml/` and `mermaid/`: `internal/visualizer/board/`.
- `visualizer.Render(doc, Request)` stays the single entry point (ruling 16).
- `internal/lang` owns the operation-verb vocabulary (ruling 19); the
  query/command classification lives there as a function, not as string
  compares in the writer.
- Use the project vocabulary from `CONTEXT.md` (View, Backend, Model,
  Participant, Interaction). `CONTEXT.md` defines Backend as "the diagram
  language: PlantUML or Mermaid" — update it to include the board backend
  (Craft's own SVG). Avoid "renderer"/"engine" in names.
- No CI on push: `go test ./...` must pass before tagging.

## Decisions made with the user

1. **It is the flow View, drawn by a new Backend** — not a new View.
2. **Output is a static SVG file.** It scales, opens in any browser, embeds
   in docs and webviews.
3. **Query vs command comes from the operation verb** already written in the
   trailing `[...]` annotation. No new DSL syntax.
4. **First release: CLI and server preview only.**
5. **Arrows are curved** like Miro's curved connectors (see Look below).
6. **One board per use case.** Several use cases are never merged onto one
   canvas.
7. **Steps the Miro notation has no symbol for** (event delivery, replies,
   internal actions) are drawn as in the mapping table below.

## The look (from the POC)

All values are in the POC; this lists what matters.

- **Canvas**: light grey (`#e6e6e6`) page, title band at the top (bold,
  ~30px), white board panel with a thin grey border, notation legend panel on
  the left.
- **Bounded context**: the hand-tuned 8-lobe cloud (`CLOUD` table in the
  POC: notch positions as fractions of the box plus each lobe's bulge), fill
  `#cdcdfa`, stroke `#6130b8`, stroke width `min(w,h)/45`. Node box 190×110.
  Bold 15px centred label, wrapped.
- **Which shape a node gets** (*my call*): bounded contexts and domains are
  clouds; services, unresolved names, externals (External, unobserved
  caller) and queues are cogs; data stores get the store icon. `view.Node`
  does not carry the participant kind today; add it so the writer can choose.
  Service boxes group a service's drawn nodes, including non-context
  members; they appear only for services with two or more drawn nodes.
  (Boomi in the POC is a `system` actor, a cog outside any service box.)
- **Cog**: chunky black cog (body disc, 8 square
  teeth, white ring, black hub), 46px, name in 11px beside it on the side
  away from the nearest note.
- **Actor**: black person icon.
- **Data store**: not in the Miro notation. *Builder's call* on the icon
  (a cylinder in the same flat black style is the obvious choice); it gets a
  legend row only when a store is drawn.
- **Sticky note**: 84×56, kind colour (query `#d6e46c`, command `#4a90e2`,
  event `#f2c33c`), thin translucent border, faint drop shadow, small bold
  text (8.5px) at the top, wrapped. Step number in a small white circle
  centred on the top edge, red digits.
- **Arrows**: dark grey, 1.4px, small solid head. Dashed for async/inferred
  where the flow view uses it. Curved: an S-curve along the dominant axis,
  softened halfway toward the straight line; near-vertical lines stay
  straight. Arrows stop at the shape's real outline (the cloud's lobes, not
  its box) and are drawn above shapes so heads always show.
- **Service box**: dashed grey rectangle around the contexts of one service
  (and the notes between them), tight padding.
- **Legend**: "Notation" panel with Actor, Bounded Context, System, Query,
  Command, Event, Direction of message — always shown, same colours as the
  board. Plus Data store when present.
- **Title**: `Request.Title` if set; else "Scenario: <use case name>" when
  exactly one use case is shown; else no title band.
- **Font**: Open Sans by name with a sans-serif fallback stack; not
  embedded. Text widths are estimated (no font metrics available in Go);
  wrapping is by estimated width.

## Mapping flow edges to the board

Every `view.FlowEdge` is drawn, keeping its `Number`. Edges in the same
direction between the same pair of nodes share one arrow; their notes sit
side by side. A→B and B→A are separate arrows, bowed apart.

The **neutral note** is white/light grey (*builder's call* on the exact
tone) and gets a "Step" row in the legend only when one is drawn (*my
call*).

Arrow dashing follows this table and the flow view's styles, not the POC
(the POC dashes non-event async edges, which was a shortcut).

| Flow edge | Arrow | Note |
|---|---|---|
| Request (`StyleCall`) with verb GET, HEAD, OPTIONS, QUERY | solid | query (green) |
| Request with verb POST, PUT, PATCH, DELETE | solid | command (blue) |
| Request with any other verb or none | solid | neutral note (*builder's call*: white/light grey) |
| Publish to queue (`StyleAsync`) | solid | event (yellow) |
| Store write / poll | as flow | neutral note |
| Actor trigger (`StyleTrigger` from an actor) | solid | command |
| Listen: queue/store → listener (`StyleTrigger`) | solid | none — only the step-number circle, on the arrow (the event's note is on the publish) |
| Response (`StyleReply`) | dashed, back to the caller, curved to the other side so it never sits on the request's arrow | neutral note |
| Internal action (`StyleSelf`) | none | neutral note pinned to the edge of the context's cloud, numbered |
| Inferred edge | greyed arrow | note greyed, label ends "(inferred)" as in flow |

Branch prefixes (`[par: …]`, `[loop: …]`, `[alt label]`) stay in the note
text exactly as the flow view builds them.

The flow view does not carry the operation verb today. Add it to
`view.FlowEdge` (e.g. a `Verb` field filled from `Interaction.Operation`);
PlantUML and Mermaid writers ignore it. The flow view also has no service
groups; the board needs them — reuse the grouping the architecture view
already builds (`view.ServiceGroup`), shared rather than copied.

## Layout

Two layers, both in Go:

1. **A generic layered layout** places nodes top-to-bottom. It must be pure
   Go (no Graphviz, no JS runtime). *Builder's call* which: evaluate an
   existing pure-Go Sugiyama-style library (e.g. `github.com/nulab/autog`)
   against a small hand-written layered layout; pick on output quality for
   the POC scenario and the `examples/` models, determinism, and dependency
   weight.
2. **Board rules on top** (from the POC, in order):
   - A node reached **only by queries** is a side lookup: it is left out of
     the layered layout and placed to the right of its caller — the first
     lookup level with the caller, further ones alternating above and below.
   - One curved arrow per node pair, clipped to the real outlines.
   - Notes: beside the middle of a flat arrow (below it, or above when the
     arrow rises); beside the target end of a steep arrow, alternating right
     and left when several share it. A note is nudged away along that
     direction until it overlaps no other note or node.
   - Gear/icon labels go on the side away from the nearest note.
   - Service boxes are drawn last around their members and the notes between
     them.

The POC's rules are naive (no global overlap solving, lookups of different
callers may collide). Acceptable for the first release; mark known ceilings
in code.

## Surfaces

- **CLI**: `craft generate --type domain --format board` writes the flow view
  as **one `.svg` per use case**, next to the other outputs, using the
  existing `--split` file naming (the board always splits; `--split` is
  accepted and redundant). `--use-case` narrows which use cases are drawn.
  `--stdout` needs exactly one use case (via `--use-case` or a one-use-case
  model), otherwise it is an error.
  `--stdout` prints the SVG. `--format board` with `--mode architecture`,
  `--type c4|sequence|surface|all` or `--journey` is an error naming the
  supported combination (no silent skip).
- **Server**: a new `POST /preview/board/domain` taking the same request
  body as `/preview/domain` plus an optional `useCase` (name or slug). It
  draws that use case, or the first one when `useCase` is absent (agreed
  with the user); an unknown name is a 400 listing the available ones. It
  answers a `PreviewResponse` whose `Data` is
  the SVG text (plain, not base64 — like the Mermaid previews), with
  diagnostics. Architecture mode → 400 with the `ErrUnsupported` message.
- `visualizer`: `Backend` gains `Board`; the only valid `Format` for it is
  `svg` (returned directly, no rasterizer call); any View other than flow is
  `ErrUnsupported`.

## Testing

- Golden SVGs for the POC's Invoicing scenario (Boomi modelled as a `system`
  actor, so it comes out as a cog outside any service box) (written as a `.craft` file)
  and at least two `examples/` models, compared byte for byte.
- Unit tests for the verb classification in `internal/lang`.
- Unit tests for the board rules that are easy to get wrong: lookup
  placement, notes never overlapping each other or nodes, arrow ends on the
  outline, request/reply arrows between the same pair not overlapping.
- Server: body fields the board cannot honour (e.g. `diagramType: sequence`,
  `format: png`, architecture mode) are rejected, never ignored.
- Visual check before release: render the goldens and compare against the
  POC by eye.

## As built

- Boomi is modelled as a `system` actor: a cog outside any service box,
  since actors have no service. (The Invoicing golden therefore does not
  show it inside the Invoicing box.)
- A store poll is event delivery (step badge only); a store write gets a
  neutral note.
- Service boxes appear only for services with two or more drawn nodes.

## Release

`go test ./...` green, CHANGELOG entry, installation guide/docs page for the
new format. The version bump follows the usual rule (judged on the
`pkg/craft` Go API; this design adds nothing there).

## Left to the builder

- Layout library vs hand-written layered layout (criteria above).
- Data store icon, neutral note colour.
- Text-width estimation approach.
- Internal structure of `internal/visualizer/board/`.
- File naming details in the CLI, following existing conventions.
