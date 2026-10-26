# Board Backend for the Flow View — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `board` Backend that writes the flow View as a standalone, Miro-style SVG (one use case per board), reachable from `craft generate --format board` and `POST /preview/board/domain`.

**Architecture:** A new writer package `internal/visualizer/board` turns a `view.Flow` into SVG with hand-drawn shapes, a small hand-written layered layout plus board-specific placement rules, and curved arrows. `view.Flow` gains the few facts the board needs (participant kind, owning service, operation verb). `visualizer.Render` routes `Backend: Board` to it and returns SVG directly, never touching the Rasterizer.

**Tech Stack:** Go 1.25 standard library only (`strings`, `strconv`, `math`, `sort`, `html`/`encoding/xml` for escaping and test parsing).

**Spec:** `docs/superpowers/specs/2026-10-26-flow-board-backend-design.md`. Visual reference: `docs/superpowers/specs/assets/2026-10-26-flow-board-poc.html` (open it in a browser; its JS is the reference for shapes, constants and placement rules — port it, do not reinvent it).

## Global Constraints

- No new module dependencies: `go.mod` and `go.sum` are unchanged at the end.
- No Graphviz, no `os/exec`, no JS runtime anywhere on the board path. `Render` with `Backend: Board` never calls the Rasterizer.
- Output is deterministic: the same model and request give byte-identical SVG. Never let map iteration order reach the output; sort first.
- `internal/lang` owns the operation-verb vocabulary (ruling 19); the board compares `lang` constants, never verb string literals.
- An option the board cannot honour returns an error wrapping `visualizer.ErrUnsupported` (ruling 16); nothing is silently ignored.
- Vocabulary from `CONTEXT.md` (View, Backend, Model, Participant, Interaction); no type or function named *renderer* or *engine* in new code.
- Visual values are the POC's: cloud fill `#cdcdfa`, stroke `#6130b8`, stroke width `min(w,h)/45` (at least 1); notes query `#d6e46c`, command `#4a90e2`, event `#f2c33c`, neutral `#ffffff`; canvas `#e6e6e6`; arrow `#333`, inferred arrow `#9a9a9a`.
- Font family attribute: `'Open Sans', Arial, Helvetica, sans-serif` (not embedded).
- Work on branch `flow-board`, one commit per task; run `go test ./...` before each commit.

## Review Focus

1. **Labels with XML-special characters** (`&`, `<`, `>`, `"`) in use-case names, step text or names must be escaped; the SVG still parses as XML. → Task 6 test `TestBoard_EscapesLabels`.
2. **Declared display names** must be what the board shows on nodes (`Node.Text()`), as the other backends do. → Task 6 test `TestBoard_UsesDisplayNames`.
3. **A use case whose only steps are internal actions of one context** (no arrows at all) must still produce a valid board. → Task 6 test `TestBoard_InternalOnlyUseCase`.
4. **Two contexts that call each other and reply** (A→B, B→A, replies both ways) must lay out without hanging, and the opposite arrows must not coincide. → Task 4 `TestLayout_CycleTerminates`, Task 5 `TestRoute_OppositeArrowsBowApart`.
5. **A single very long word** in a label must not panic and stays on its own line. → Task 3 `TestWrap_LongWordOwnLine`.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/lang/verbs.go` (modify) | `MessageKind` + `VerbMessageKind` |
| `internal/visualizer/view/view.go` (modify) | `Node.Participant`, `Node.Service` |
| `internal/visualizer/view/flow.go` (modify) | `FlowEdge.Verb` |
| `internal/visualizer/board/svg.go` | number formatting, escaping, word wrap, tiny SVG element writer |
| `internal/visualizer/board/shapes.go` | cloud (path + outline polygon), cog, person, store, sticky note, legend |
| `internal/visualizer/board/layout.go` | node positions: layered main graph + side lookups |
| `internal/visualizer/board/route.go` | arrows, notes, badges, service boxes |
| `internal/visualizer/board/board.go` | `Flow(f, title) []byte`: assembles the page |
| `internal/visualizer/visualizer.go` (modify) | `Board` backend, validation, routing |
| `internal/visualizer/testdata/invoicing.craft` | the POC scenario as a model |
| `cmd/craft/generate.go` (modify) | `--format board` |
| `cmd/server/main.go` (modify) | `POST /preview/board/domain` |

---

### Task 1: Message kind from the operation verb

**Files:**
- Modify: `internal/lang/verbs.go`
- Test: `internal/lang/verbs_test.go`

**Interfaces:**
- Produces:
  ```go
  type MessageKind string
  const (
      MessageNeutral MessageKind = ""
      MessageQuery   MessageKind = "query"
      MessageCommand MessageKind = "command"
  )
  func VerbMessageKind(verb string) MessageKind
  ```

- [ ] **Step 1: Write the failing test** `TestVerbMessageKind`, table-driven:
  `GET, HEAD, OPTIONS, QUERY → MessageQuery`; `POST, PUT, PATCH, DELETE → MessageCommand`; `GRPC, TOPIC, SQL, ADDR, "", "get", "FETCH" → MessageNeutral` (matching is exact and case-sensitive, like `IsProtocolVerb`).
- [ ] **Step 2: Run** `go test ./internal/lang/ -run TestVerbMessageKind` — expect FAIL (undefined).
- [ ] **Step 3: Implement** in `verbs.go` next to `protocolVerbs`, with a doc comment saying it classifies an `asks` for the board backend.
- [ ] **Step 4: Run** `go test ./internal/lang/` — expect PASS.
- [ ] **Step 5: Commit** `feat(lang): classify operation verbs as query or command`.

---

### Task 2: Flow view carries participant kind, service and verb

**Files:**
- Modify: `internal/visualizer/view/view.go` (`Node`, `nodeSet.add`)
- Modify: `internal/visualizer/view/flow.go` (`FlowEdge`, `BuildFlow`'s `add`)
- Test: `internal/visualizer/view/flow_test.go` (create)

**Interfaces:**
- Produces:
  - `view.Node.Participant model.ParticipantKind` — the endpoint's resolved kind; `""` for unresolved names, queues and the External/Unobserved placeholders.
  - `view.Node.Service string` — the owning service of a context or data store (`interaction.Endpoint.Service` when the endpoint's kind is not `ParticipantService`), else `""`. Queues get `""`.
  - `view.FlowEdge.Verb string` — `it.Operation.Verb` for a Request (`""` when `Operation` is nil); `""` for every other kind.

- [ ] **Step 1: Write failing tests** in `flow_test.go` using the existing `analysis.File`/`analysis.Workspace` pattern (see `internal/visualizer/golden_test.go`) over a small inline model with a service `S { contexts: A }`, contexts `A`, `B` in a domain, `A asks B to get x [GET /x]`, `A asks B to put y [PUT /y]`, `A notifies a.Done`:
  - `TestBuildFlow_EdgeVerb`: the two Request edges have `Verb` `"GET"` and `"PUT"`; the publish edge has `""`.
  - `TestBuildFlow_NodeParticipantAndService`: node `A` has `Participant == model.ParticipantBoundedContext` and `Service == "S"`; `B` has `Service == ""`; the queue node has `Participant == ""`.
- [ ] **Step 2: Run** `go test ./internal/visualizer/view/ -run TestBuildFlow_` — expect FAIL (unknown fields).
- [ ] **Step 3: Implement**: set both node fields in `nodeSet.add` (all views share it; PlantUML/Mermaid writers ignore them); thread the verb through `BuildFlow`'s `add` closure (add a `verb string` parameter, pass it only from the `interaction.Request` case).
- [ ] **Step 4: Run** `go test ./internal/visualizer/...` — expect PASS, including `TestGolden` unchanged (no golden file changes).
- [ ] **Step 5: Commit** `feat(view): flow nodes carry participant kind and service, edges the verb`.

---

### Task 3: SVG primitives and shapes

**Files:**
- Create: `internal/visualizer/board/svg.go`, `internal/visualizer/board/shapes.go`
- Test: `internal/visualizer/board/shapes_test.go`

**Interfaces:**
- Produces (package-private, used by Tasks 4–6):
  ```go
  type point struct{ X, Y float64 }
  func num(v float64) string                 // rounds to 1 decimal, no trailing ".0": strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
  func esc(s string) string                  // XML text/attribute escaping (&, <, >, ", ')
  func wrap(s string, maxChars int) []string // greedy word wrap by rune count; a longer word keeps its own line
  type svg struct{ b strings.Builder }       // writer: methods el(tag string, attrs ...string) / open / close / text(...) — shape is the implementer's call

  type shapeKind int
  const ( shapeCloud shapeKind = iota; shapeCog; shapePerson; shapeStore )
  func shapeOf(n view.Node) shapeKind
  var nodeSize = map[shapeKind]point{shapeCloud: {190, 110}, shapeCog: {46, 46}, shapePerson: {40, 46}, shapeStore: {40, 46}}
  const noteW, noteH = 84.0, 56.0

  func cloudPath(w, h float64) string        // SVG path d, centred on 0,0
  func cloudOutline(w, h float64) []point    // same outline as a polygon (each lobe arc sampled at 12 points), centred on 0,0
  ```
  Drawing functions (`drawCloud`, `drawCog`, `drawPerson`, `drawStore`, `drawNote`, `drawLegend`) take the `*svg`, a centre and sizes; signatures are the implementer's call.

- `shapeOf` (spec "Which shape a node gets"): `NodeContext` with `Participant` bounded context or domain → cloud; `NodeContext` with service, data store → cog / store respectively; `NodeContext` with `""` (unresolved) → cog; `NodeQueue`, `NodeExternal` → cog; `NodeStore` → store; `NodeActor` with `ActorType` user or `""` → person; any other actor type (system, service, scheduler, tool, infra) → cog.
- Cloud geometry, verbatim from the POC (`CLOUD` table: notch x, y as fractions of w×h, clockwise from top-left, then the lobe bulge as sagitta/chord; each lobe is an arc of radius `r = (c²/4 + s²)/(2s)` with `s = k·c`, large-arc flag set when `k > 0.5`, sweep 1):
  ```go
  var cloudNotches = [][3]float64{{-.36, -.197, .24}, {-.164, -.413, .22}, {.128, -.376, .24}, {.364, -.197, .46},
      {.379, .25, .24}, {.128, .395, .25}, {-.143, .387, .26}, {-.399, .132, .53}}
  ```
- Cog, person, sticky note and legend: port the POC's `gear`, `person`, `sticky` and `legend` functions (cog: disc r·0.74, 8 teeth `r·0.38 × r·0.4` rx `r·0.06` rotated by 45°, white ring r·0.44, hub r·0.26). Store (not in the POC): a flat black cylinder (ellipse top + body), same visual weight as the cog. Note: kind fill, stroke `rgba(0,0,0,.25)` 0.8, `filter="url(#shadow)"`, 8.5px bold text wrapped at 18 chars, first line at `y − noteH/2 + 14`; number in a white circle r 8 (stroke `#8a8a8a` 0.8) on the top edge, 8px red `#d33` digits. Inferred notes: whole note group `opacity="0.55"`. Notes without a number (none in this design) draw no circle.

- [ ] **Step 1: Write failing tests** in `shapes_test.go`:
  - `TestNum`: `num(1) == "1"`, `num(1.25) == "1.3"`, `num(-0.04) == "0"`, `num(12.345) == "12.3"`.
  - `TestEsc`: `esc(`a & <b> "c"`) == "a &amp; &lt;b&gt; &#34;c&#34;"` (or `&quot;` — assert via `xml` round-trip instead if you choose another entity form).
  - `TestWrap_LongWordOwnLine`: `wrap("get the averyveryverylongwordindeed now", 10)` == `[]string{"get the", "averyveryverylongwordindeed", "now"}`.
  - `TestCloudOutline_FitsBox`: every point of `cloudOutline(190, 110)` lies within `|x| ≤ 95.5`, `|y| ≤ 55.5`, and the outline reaches at least 90% of the half-width and half-height (max |x| ≥ 85.5, max |y| ≥ 49.5).
  - `TestShapeOf`: one case per rule above (build `view.Node` literals).
- [ ] **Step 2: Run** `go test ./internal/visualizer/board/` — expect FAIL.
- [ ] **Step 3: Implement** `svg.go` and `shapes.go`.
- [ ] **Step 4: Run** `go test ./internal/visualizer/board/` — expect PASS.
- [ ] **Step 5: Commit** `feat(board): svg primitives and Miro-style shapes`.

---

### Task 4: Layout

**Files:**
- Create: `internal/visualizer/board/layout.go`
- Test: `internal/visualizer/board/layout_test.go`

**Interfaces:**
- Consumes: `shapeOf`, `nodeSize` (Task 3); `lang.VerbMessageKind` (Task 1).
- Produces:
  ```go
  type placed struct {
      Node  view.Node
      Shape shapeKind
      X, Y  float64 // centre
      W, H  float64
  }
  func layout(f view.Flow) map[string]*placed // keyed by node ID; every node of f is present
  func isLookup(f view.Flow, id string) bool
  ```

Algorithm (not determined by the tests; follow it):

1. **Lookup**: node L is a lookup iff it has at least one incoming edge, every edge into L is `StyleCall` with `VerbMessageKind(e.Verb) == MessageQuery`, and every edge out of L is `StyleReply`.
2. **Ranking edges**: edges between non-lookup nodes, excluding `StyleReply` and `StyleSelf`, taken in `Number` order; skip an edge if its target already reaches its source through edges kept so far (breaks cycles deterministically). *ponytail: O(n·e) reachability per edge; fine for one use case.*
3. **Rank** = longest path from a source (0-based).
4. **Order within a rank**: start from first appearance in `Number` order (nodes on no edge last, by ID). Then 4 passes alternating down/up: sort each rank stably by the mean index of its neighbours in the previous (down) / next (up) rank; a node with no such neighbours keeps its current index as its key.
5. **Coordinates**: row height = max node H in the row; row centre y = sum of previous row heights + `rankGap·r` + height/2 with `rankGap = 90`. Within a row, boxes left to right with `nodeGap = 120` between boxes, the row centred on x = 0.
6. **Lookups**: grouped by caller = `From` of the lowest-numbered edge into them; per caller, in that edge order, centre x = (max right edge of the non-lookup nodes) + 260, y = caller.Y + offset with offsets `0, −150, +150, −300, +300, …`. If a lookup's box overlaps one already placed, move it down by 150 until it does not.
7. **Normalise**: shift everything so the min left edge and min top edge are 60.

- [ ] **Step 1: Write failing tests** (build `view.Flow` literals directly; helper `flowOf(nodes []view.Node, edges []view.FlowEdge)`):
  - `TestLayout_ChainStacksVertically`: A→B→C (`StyleCall`, verb POST): same X for all three, Y strictly increasing.
  - `TestLayout_QueryTargetIsSideLookup`: Inv calls Pay (GET, #1) and Cust (GET, #2), Pay replies to Inv; `isLookup` true for Pay and Cust, false for Inv; Pay.Y == Inv.Y; Cust.Y == Inv.Y − 150; both X > Inv.X + Inv.W/2.
  - `TestLayout_CommandTargetIsNotLookup`: Inv calls Boomi with POST → Boomi below Inv (Boomi.Y > Inv.Y).
  - `TestLayout_CycleTerminates`: A calls B (POST), B calls A (POST), A replies to B, B replies to A → returns, all nodes placed, no two boxes overlap.
  - `TestLayout_Deterministic`: the same flow laid out 20 times gives identical positions.
- [ ] **Step 2: Run** `go test ./internal/visualizer/board/ -run TestLayout` — expect FAIL.
- [ ] **Step 3: Implement** `layout.go`.
- [ ] **Step 4: Run** `go test ./internal/visualizer/board/` — expect PASS.
- [ ] **Step 5: Commit** `feat(board): layered layout with side lookups`.

---

### Task 5: Arrows, notes, badges and service boxes

**Files:**
- Create: `internal/visualizer/board/route.go`
- Test: `internal/visualizer/board/route_test.go`

**Interfaces:**
- Consumes: `placed`, `layout`, `cloudOutline`, `noteW`, `noteH`.
- Produces:
  ```go
  type noteKind string
  const ( noteQuery noteKind = "query"; noteCommand = "command"; noteEvent = "event"; noteNeutral = "neutral" )
  type arrow struct {
      P0, C1, C2, P3   point // cubic Bézier
      Dashed, Inferred bool
  }
  type note struct {
      X, Y     float64 // centre
      Kind     noteKind
      Number   int
      Label    string
      Inferred bool
  }
  type badge struct{ X, Y float64; Number int }
  type box struct{ X0, Y0, X1, Y1 float64 } // already padded
  type routed struct { Arrows []arrow; Notes []note; Badges []badge; Boxes []box }
  func route(f view.Flow, at map[string]*placed) routed
  func rim(p *placed, toward point) point
  ```

Rules (port the POC's `render` routing section; the mapping is the spec's table):

- **Which edges get what** (`noteOf`):
  - `StyleCall` → note of `VerbMessageKind(e.Verb)` (query / command / neutral).
  - `StyleAsync` to a `NodeQueue` → event note; to a store (store write) → neutral note.
  - `StyleTrigger` from a `NodeActor`, or from the Unobserved placeholder (`Node.ID == interaction.Unobserved.Key`) → command note.
  - `StyleTrigger` otherwise (event delivery from a queue, store poll, External) → **no note**: a `badge` with the edge number at Bézier t = 0.5 (t = 0.5 + 0.1·i for the i-th such edge on the same arrow).
  - `StyleReply` → dashed arrow, neutral note.
  - `StyleSelf` → no arrow; neutral note pinned to the node: centre x = node.X − node.W/2 − noteW/2 + 16, y = node.Y + node.H/4; later ones nudged down (step 12). The pinned note ignores its own node in the overlap check.
  - Inferred edges: `Inferred` on both arrow and note.
- **Arrows**: one per ordered pair (From, To) excluding `StyleSelf`, in order of the pair's lowest edge number. Ends: `rim(from, to centre)` and `rim(to, from centre)`. `rim` for a cloud = the farthest intersection of the ray from the centre with `cloudOutline` translated to the node, pushed 1px outward; for other shapes the point at radius `max(W,H)/2·1.15`. Controls: `dx, dy = P3−P0`; if `|dx| > |dy|`: `C1 = P0 + (dx/2, dy/4)`, `C2 = P3 − (dx/2, dy/4)`; else `C1 = P0 + (dx/4, dy/2)`, `C2 = P3 − (dx/4, dy/2)`. When the reverse pair also has an arrow, offset C1 and C2 by 24px along the left normal `(−dy, dx)/|d|` (the two arrows then bow apart). An arrow is dashed iff all its edges are `StyleReply` (a reply runs the opposite way to its request, so it never shares the request's arrow).
- **Notes beside an arrow** (all noted edges of the pair, in number order): `flat = |dx| > |dy|`; anchor at t = 0.5 (flat) or 0.72 (steep) along the straight P0→P3; flat: a row of notes centred on the anchor x, spaced `noteW + 14`, centre y = anchor ± (`noteH/2 + 18 + |dy|·0.25`), minus when the arrow rises (`P3.Y < P0.Y − 20`); steep: i-th note at anchor x + side·(`noteW/2 + 18`) with side = +1 for even i, −1 for odd i. Nudge: while the note overlaps an earlier note (`|Δx| < noteW + 10 && |Δy| < noteH + 14`) or a node (`|Δx| < (W + noteW)/2 + 6 && |Δy| < (H + noteH)/2 + 6`), move 12px in its offset direction (flat: up or down; steep: side), at most 40 moves. *ponytail: greedy nudge, no global overlap solve.*
- **Service boxes**: for each `Node.Service` value with **two or more** placed nodes, a box around those nodes' boxes plus the notes of edges whose both ends are in that service; padding 24 (x) and 18 (y). Services in sorted order. *(my call: a box around a single node is noise.)*

- [ ] **Step 1: Write failing tests**:
  - `TestRim_OnCloudOutline`: for a cloud at (0,0) and targets in 8 directions, `rim` lies within 1.5px of the outline polygon's boundary (distance to nearest segment ≤ 1.5).
  - `TestRoute_ListenGetsBadgeNotNote`: queue→ctx `StyleTrigger` edge #2 yields one `badge{Number: 2}` and no note numbered 2.
  - `TestRoute_NoteKinds`: GET call → query, POST call → command, GRPC call → neutral, publish to queue → event, reply → neutral, self → neutral, actor trigger → command.
  - `TestRoute_NotesDoNotOverlap`: the Invoicing-like flow (Order→queue event #1, queue→Inv #2, Inv→Pay GET #3, Inv→Cust GET #4, Inv→Boomi POST #5 and #6, Inv self #7): no two notes overlap and no note overlaps a node box other than its own pinned node.
  - `TestRoute_OppositeArrowsBowApart`: A→B call and B→A reply: the two arrows' control points differ by ≥ 40px.
  - `TestRoute_ServiceBoxOnlyForTwoOrMore`: a service with one node gets no box; one with two gets one box containing both nodes.
- [ ] **Step 2: Run** `go test ./internal/visualizer/board/ -run 'TestRim|TestRoute'` — expect FAIL.
- [ ] **Step 3: Implement** `route.go`.
- [ ] **Step 4: Run** `go test ./internal/visualizer/board/` — expect PASS.
- [ ] **Step 5: Commit** `feat(board): curved arrows, sticky notes, badges and service boxes`.

---

### Task 6: Assemble the board and wire it into Render

**Files:**
- Create: `internal/visualizer/board/board.go`
- Create: `internal/visualizer/testdata/invoicing.craft` (content below)
- Modify: `internal/visualizer/visualizer.go`
- Modify: `CONTEXT.md` (Backend entry)
- Test: `internal/visualizer/board_test.go` (create), golden files under `internal/visualizer/testdata/golden/board/`

**Interfaces:**
- Consumes: `layout`, `route`, shape drawers (Tasks 3–5).
- Produces:
  - `func board.Flow(f view.Flow, title string) []byte` — the whole SVG document; `title == ""` means no title band.
  - `visualizer.Board Backend = "board"` — Craft's own SVG writer; flow view only, `FormatSVG` only.

Page assembly (POC `render` frame section): canvas `#e6e6e6`; title band height 76 (0 when no title), bold 30px centred over the board panel; white board panel with `#bdbdbd` stroke; legend panel at (14, 70), width 260, rows 70px apart starting at y 90: Actor / User / Persona, Bounded Context, System, Query, Command, Event, Direction of message (+ 8px "From producer to consumer"), then **Data store** only if a store shape is drawn and **Step** (neutral note) only if a neutral note is drawn; panel height 560 + 70 per extra row. Board size: content bounds + 60 margin; `W = 290 + 2·28 + max(boardW, 1000)`, `H = titleH + max(boardH, 620) + 28`. Draw order: background, title, panel, legend, service boxes (dashed `#8a8a8a` 1.2, `4 4`), shapes with cloud labels (bold 15px, wrap 10), arrows (1.4px, `marker-end`; dashed `6 5`; inferred uses the grey marker), badges (white circle r 8 + number), notes, icon labels (11px, on the side away from the nearest note, 8px off the icon). `<defs>`: `feDropShadow dx=1 dy=1.5 stdDeviation=1 flood-opacity=0.18` as `#shadow`; arrow markers `#arrow` (`#333`) and `#arrow-grey` (`#9a9a9a`), path `M0,1L10,5L0,9Z`, viewBox `0 0 10 10`, refX 9, refY 5, size 7, `orient="auto-start-reverse"`. Root `<svg xmlns="http://www.w3.org/2000/svg" width height viewBox font-family>`.

Render wiring in `visualizer.go`:
- `validate`: accept `Board` in the backend switch; for `Board`, `View != ViewFlow` → `ErrUnsupported` "the board backend draws the flow view only"; `Format != FormatSVG` → `ErrUnsupported` "the board backend writes svg only". The existing "svg only for plantuml" rule must allow `Board`. Surface + Board stays rejected by `validateSurface`.
- `Render`: after use-case filtering, if `req.Backend == Board`: when `len(shown.UseCases) != 1` return `ErrUnsupported` "the board draws one use case at a time; pick one (available: <slugs>)"; title = `req.Title`, else `"Scenario: " + shown.UseCases[0].Name`; return `Output{Data: board.Flow(view.BuildFlow(shown, its), title), ContentType: "image/svg+xml", Ext: "svg"}`.
- `CONTEXT.md` Backend: "The diagram language a view is written in: PlantUML or Mermaid — or Board, Craft's own SVG in the Miro-like notation (flow view only)."

`internal/visualizer/testdata/invoicing.craft` (verified to analyse cleanly; Boomi is a `system` actor, so it is a cog — outside any service box, since actors have no service):

```craft
actors {
  system Boomi
}

domain Sales {
  Order
}

domain Billing {
  Invoicing
  Payment
}

domain Customer {
  CustomerProfile
}

services {
  OrderService {
    contexts: Order
  }
  InvoicingService {
    contexts: Invoicing
  }
  PaymentService {
    contexts: Payment
  }
  CustomerService {
    contexts: CustomerProfile
  }
}

use_case "Invoicing process starts for a prepaid Customer" {
  when Order changes an order
    Order notifies order.OrderUpdated

  when Invoicing listens order.OrderUpdated
    Invoicing asks Payment to get transaction details  [GET /v1/transactions]
    Invoicing asks CustomerProfile to get customer details  [GET /v1/customers]
    Invoicing asks Boomi to upload customer  [POST /customers]
    Invoicing asks Boomi to upload invoice  [POST /invoices]
    Invoicing validates the invoice
}
```

- [ ] **Step 1: Write failing tests** in `board_test.go` (package `visualizer`, reuse `resolvedDoc`/`analysis` helpers from existing tests):
  - `TestValidate_Board`: `{ViewFlow, Board, FormatSVG}` ok; `{ViewSequence, Board, FormatSVG}`, `{ViewFlow, Board, FormatPNG}`, `{ViewFlow, Board, FormatSource}`, `{ViewSurface, Board, FormatMarkdown}` and `{ViewFlow, Board, FormatSVG, Journey: "x"}` each `errors.Is(err, ErrUnsupported)`.
  - `TestRender_BoardNeedsOneUseCase`: the corpus `banking-system.craft` without `UseCases` → `ErrUnsupported`, message lists slugs; with `UseCases: ["Money Transfer"]` → `Ext == "svg"`, `ContentType == "image/svg+xml"`, data starts with `<svg`, and parses with `encoding/xml`.
  - `TestRender_BoardDefaultTitle`: invoicing → contains `Scenario: Invoicing process starts for a prepaid Customer`; with `Title: "X"` → contains `>X<` and not `Scenario:`.
  - `TestRender_BoardNeverRasterizes`: `New(fakeRasterizer that fails the test)` renders a board fine.
  - `TestBoard_EscapesLabels`: a model whose use case is named `Pay & <check>` and a step `A asks B to "fetch" & go [GET /x]` → output parses as XML and contains `&amp;`.
  - `TestBoard_UsesDisplayNames`: `testdata/display-names.craft`, first use case → every declared display name of a drawn node appears in the SVG.
  - `TestBoard_InternalOnlyUseCase`: a use case of only `A validates x` steps → parses as XML, contains one cloud path and a note.
  - `TestGolden_Board`: for `testdata/invoicing.craft`, `../../testdata/corpus/99_mixed/banking-system.craft` and `../../testdata/corpus/99_mixed/e-comerce.craft`, every use case with scenarios → `testdata/golden/board/<model>.<slug>.svg`; identical across 20 runs; `-update` rewrites (same pattern as `TestGolden`).
- [ ] **Step 2: Run** `go test ./internal/visualizer/ -run 'Board|TestValidate_Board'` — expect FAIL.
- [ ] **Step 3: Implement** `board.go`, the wiring, the fixture, the `CONTEXT.md` line.
- [ ] **Step 4: Generate goldens** `go test ./internal/visualizer/ -run TestGolden_Board -update`, then **look at them**: convert the invoicing golden with headless Chrome (`"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --disable-gpu --screenshot=<out>.png --window-size=1400,1000 file://<abs path>.svg`) and compare with the POC page side by side; fix any shape, colour or placement that differs from the POC beyond the expected differences (Boomi outside a box; a step 7 internal note; step numbers 1–7 including the delivery badge #2).
- [ ] **Step 5: Run** `go test ./...` — expect PASS.
- [ ] **Step 6: Commit** `feat(visualizer): board backend draws the flow view as a Miro-style SVG`.

---

### Task 7: CLI `--format board`

**Files:**
- Modify: `cmd/craft/generate.go`
- Test: `cmd/craft/generate_test.go`

**Interfaces:**
- Consumes: `visualizer.Board`, `visualizer.FormatSVG`.

Behaviour:
- `--format` accepts `board`; help text and the "must be one of" error list it: `puml|mermaid|mermaid-md|board`.
- With `board`, reject before generating (plain errors, exit non-zero): `--journey` given → "--format board draws the detailed domain diagram; it does not draw journeys"; `--type` other than `domain` (including the default `all`) → "--format board draws the detailed domain diagram only: use --type domain"; `--mode architecture` → "--format board draws the detailed domain diagram only: drop --mode architecture".
- `generateDiagrams`: `board` → `backend, outFormat = visualizer.Board, visualizer.FormatSVG`. Without `--stdout` the domain diagram always goes through `emitSplitFiles` (one `<base>-domain-<slug>.svg` per use case; `--split` accepted, redundant). With `--stdout` render once with `UseCases: useCaseFilter`; `Render`'s one-use-case error surfaces as is.
- `emitSplitFiles`: leave `one.Title` empty when `req.Backend == visualizer.Board` so the board's "Scenario: <name>" title applies.

- [ ] **Step 1: Write failing tests** following the file's existing patterns:
  - `TestGenerate_BoardWritesOneSVGPerUseCase`: `examples/banking-system.craft --type domain --format board -o <tmp>` → one `banking-system-domain-<slug>.svg` per use case with scenarios, each starting `<svg`, none other written.
  - `TestGenerate_BoardRejectsOtherTypes`: `--format board` with no `--type`, with `--type c4`, with `--mode architecture`, and with `--journey all` → error containing "--format board".
  - `TestGenerate_BoardStdoutNeedsOneUseCase`: `--stdout` without `--use-case` on banking-system → error mentioning "one use case"; with `--use-case money-transfer` → SVG on stdout.
- [ ] **Step 2: Run** `go test ./cmd/craft/ -run Board` — expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./cmd/craft/` — expect PASS.
- [ ] **Step 5: Commit** `feat(cli): craft generate --format board`.

---

### Task 8: Server `POST /preview/board/domain`

**Files:**
- Modify: `cmd/server/main.go` (request type, handler, route)
- Test: `cmd/server/board_test.go` (create)

**Interfaces:**
- Produces:
  ```go
  type BoardPreviewRequest struct {
      DSL         string `json:"dsl"`
      DiagramType string `json:"diagramType,omitempty"` // only "" or "domain"
      DomainMode  string `json:"domainMode,omitempty"`  // "detailed" (default); "architecture" is rejected
      Format      string `json:"format,omitempty"`      // only "" or "svg"
      UseCase     string `json:"useCase,omitempty"`     // name or slug; default: the first use case
  }
  func (s *Server) handlePreviewBoardDomain() http.HandlerFunc
  ```
  Route: `r.HandleFunc("/preview/board/domain", server.handlePreviewBoardDomain()).Methods("POST")` next to the other previews.

Behaviour: decode; `400` for `DiagramType` not `""`/`"domain"` ("the board preview draws the domain diagram only") and `Format` not `""`/`"svg"` ("the board preview writes svg only"); parse with `parseDSL`; no use cases → `400` "the model has no use case to draw"; `UseCase` given but matching no use case by name or `visualizer.Slugify` → `400` "no use case matches %q (available: <slugs>)"; otherwise render via `s.render` with `View: domainView("domain", req.DomainMode), Backend: visualizer.Board, Format: visualizer.FormatSVG, UseCases: []string{name}` (architecture mode comes back `ErrUnsupported` → 400 through `s.render`); respond `PreviewResponse{Success: true, Data: string(out.Data), Diagnostics: diags}` (plain SVG text, not base64).

- [ ] **Step 1: Write failing tests** (pattern: `cmd/server/main_test.go`):
  - `TestPreviewBoard_FirstUseCaseByDefault`: two use cases, no `useCase` → 200, `Data` starts `<svg` and contains the first use case's name.
  - `TestPreviewBoard_PicksUseCaseBySlug`: `useCase` = the second's slug → its name in `Data`.
  - `TestPreviewBoard_Rejects`: unknown `useCase`, `diagramType: "sequence"`, `format: "png"`, `domainMode: "architecture"`, a model with no use cases → each 400.
  - `TestPreviewBoard_ReportsDiagnostics`: a model with an unresolved participant → 200 with non-empty `Diagnostics`.
- [ ] **Step 2: Run** `go test ./cmd/server/ -run PreviewBoard` — expect FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./cmd/server/` — expect PASS.
- [ ] **Step 5: Commit** `feat(server): board preview endpoint`.

---

### Task 9: Documentation and release readiness

**Files:**
- Modify: `CHANGELOG.md` (Unreleased section, following its format)
- Modify: the docs page that documents `craft generate --format` (find it with `grep -rln "mermaid-md" docs/page --include=*.md`, excluding `node_modules`) and the server API docs if previews are documented there (`grep -rln "preview/mermaid" docs/ README.md`)
- Modify: `docs/superpowers/specs/2026-10-26-flow-board-backend-design.md` — record two as-built facts: Boomi is modelled as a `system` actor (cog, outside any service box, since actors have no service); a store **poll** is event delivery (badge only), a store **write** gets a neutral note; service boxes appear only for services with two or more drawn nodes.

- [ ] **Step 1: Write** the CHANGELOG entry (new `--format board`, new `POST /preview/board/domain`, one board per use case, query/command from the operation verb) and the docs-page sections with one example command and the request/response shape.
- [ ] **Step 2: Run** `go test ./...` — expect PASS. Run `go vet ./...` — expect no output.
- [ ] **Step 3: Smoke-test the CLI**: `go run ./cmd/craft generate internal/visualizer/testdata/invoicing.craft --type domain --format board -o <tmp>` → prints one `.svg` path; open it in a browser and confirm it matches the golden.
- [ ] **Step 4: Commit** `docs: board backend`.
