# Diagram pipeline deepening

Status: accepted (2026-10-02). Implementation is staged; see "Delivery".

An architecture review of the path from `.craft` text to diagrams found that
sema's name resolution never reached the model the diagrams read, and that
"who talks to whom" was derived seven separate times (three PlantUML
generators, three Mermaid generators, and the context-map lint). The copies
disagreed with each other, and a qualified participant (`re/billing asks …`)
passed `craft validate` but silently lost its edge in C4 and domain diagrams.

This record captures the rulings made while designing the fix, so that later
reviews do not re-open them.

## Rulings

### Analysis

1. **One analysis module for every caller.** `internal/analysis` has two
   stages: a per-file stage (parse, per-file symbols, per-file diagnostics;
   cacheable) and a workspace stage (merge, resolve, lint, build the
   `CraftDoc`, sort diagnostics into a total order). `pkg/craft`, the LSP
   workspace, the CLI and the HTTP server all go through it. Nothing calls
   `syntax.Parse → ProjectFromTree` directly.
2. **The projection lives in `analysis`, not `syntax`.** `syntax` owns tokens,
   trees and typed views only.
3. **A folder is one model, also for `craft generate`.** Input files are merged
   and drawn together (grammar decision Q14). There is no per-file mode.
4. **`craft generate` stops on any error-severity diagnostic**, parse or sema.
   `--allow-errors` draws anyway. Warnings and hints are printed, never fatal.
5. **The HTTP preview stays tolerant.** It always draws whatever partial model
   exists, and returns diagnostics in an optional `diagnostics` response field.
6. **Same-name services and domains merge across files** with the same rule as
   within a file: list fields are unioned in order, scalar fields take the first
   non-empty value, files are visited in sorted URI order. Other named
   declarations keep their duplicate-name diagnostics.
7. **`craft.Parse` is a workspace of one file.** It returns the same resolution
   and workspace diagnostics `ParseFiles` does (minor version bump).

### Resolution

8. **Resolved identity is carried on the model.** Actions and triggers carry a
   `Participant{Kind, ID, Service}`; services carry canonical context IDs
   aligned with `Contexts`. Fields are additive (`CraftDoc` only grows).
9. **One participant resolver, BC first.** Order: qualified `domain/bc` → BC in
   the owning block's domain → unique bare BC → ambiguous (`ambiguous-bc`
   error) → domain → service → actor → data store. Use cases have no owning
   domain scope. The LSP's resolution map and the model are filled from this
   one call.
10. **A bounded context belongs to one service.** Listing it in two services'
    `contexts:` raises `bc-in-multiple-services` (warning); the owner is the
    first declaration in sorted URI order.
11. **Data stores are participants.** Names declared in `data-stores:` resolve
    to kind `data_store`, owned by their service. There is no magic
    `"Database"` target.

### Communication

12. **Interactions are derived once.** `internal/interaction` turns a resolved
    `CraftDoc` into an ordered list of interactions (sync, async, return,
    listen, trigger). Diagrams and the context-map lints both read it. It stays
    internal until a consumer needs it in `pkg/craft`.
13. **Every publisher pairs with every listener of an event.** A listener's
    interaction belongs to the listener's use case. An event with listeners and
    no publisher is triggered by an external source.
14. **PlantUML's semantics are the reference** where backends used to disagree:
    a return without a target goes to the caller on the call stack; an external
    trigger targets the first action's subject; labels carry the full action
    description; steps keep their source position and each view numbers only
    what it draws.
15. **Backends draw the same views.** Interactions feed per-view builders
    (sequence, flow, architecture, C4); PlantUML and Mermaid writers only read
    those views. Mermaid C4 has full parity with PlantUML C4. A feature a
    backend cannot express is an error, never a silent omission.

### Rendering

16. **One render entry point**: `Render(doc, Request) (Output, error)`. Options
    that do not apply to the requested view or backend are rejected.
17. **Rasterising sits behind a seam.** Production executes PlantUML; tests use
    a fake. Mermaid to PNG/SVG/PDF is unsupported until an adapter exists.
18. **The HTTP API does not change shape** as part of this work.

### Vocabulary

19. **`internal/lang` owns the fixed language vocabulary**: action and trigger
    kinds, context-map edge verbs, operation verbs, glossary verbs. Every other
    package compares against its constants, never string literals.

## Delivery

One branch per step, each with a green test suite:

0. Remove dead code (ANTLR leftovers, Graphviz, unreachable C4 views,
   `internal/processor`) and fix documentation drift.
1. Analysis module (rulings 1–7).
2. Resolved model (8–11, plus the action and trigger kinds of 19).
3. Interactions and views (12–15).
4. Render entry point (16–18).
5. Remaining vocabulary (19).
