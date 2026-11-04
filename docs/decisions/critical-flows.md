# Critical flows

Status: shipped in v2.35.0 (2026-11-04).

A use case says how one scenario runs: a trigger and the steps it sets off.
The business asks a different question: which outcomes matter most, who owns
them, and which use cases deliver them. "Lead generation" is one outcome, and
it spans several use cases (a seeker sends a message, a seeker reveals the
phone number, a seeker calls). One use case can also serve more than one
outcome. Neither a journey (one ordered spine of hand-offs) nor free tags
could say this: a journey orders use cases but has no priority or owner, and
a free tag has no declaration to check the value against.

This record captures the rulings, so that later reviews do not re-open them.

## Rulings

1. **A flow is a business outcome above use cases.** `flow <ref> "<name>"
   { … }` is a top-level declaration: a named, prioritised, owned group of use
   cases. It is not a diagram and has no steps; the use cases keep the steps.
2. **Product owns the definition; engineering points at it.** The flow block
   holds what product decides: `priority:` (`P0`–`P3`, required), `owner:`
   (required, typically a product-domain slug) and `metric:` (optional). It
   lists no use cases. A use case names the flows it serves in its own
   `realises:` tag, so the people who write use cases keep them current, and
   one use case can serve several flows without the flow file changing.
3. **The reverse index is computed.** `Flow.RealisedBy` lists the use cases
   whose `realises` tag names the flow, in model order. Nobody maintains it by
   hand, so it cannot drift from the use cases.
4. **A ref resolves over the whole workspace, like a link title.** A
   `realises` value that names no flow is a warning
   (`craft/sema/unresolved-flow`), not an error: `craft validate` on one file
   must not fail because the flows live in another file. A flow ref declared
   twice is an error (`craft/sema/duplicate-flow`), unlike a service or a
   surface declared in two files: two definitions of one outcome disagree on
   its priority or owner, and there is no way to tell which one is right.
5. **`flow` is a contextual keyword.** It is a keyword only when `<ref>
   "<name>" {` (or a part of it and `{`) follows on its line, as `surface` is.
   Every file that used `flow` as a context, a subject, a verb or a phrase
   word parses as before.
6. **Diagrams do not draw flows.** `craft generate` ignores them; `craft
   inspect` and `craft check` report them, and `pkg/craft` exposes them
   (`CraftDoc.Flows`, `UseCase.Realises`, `CraftDoc.FlowByRef`) for consumers
   such as the hub.

## Out of scope

- **Tests and expectations.** A flow does not say how to check that it
  works: no synthetic checks, no SLOs, no expected metric values. Those belong
  to the tools that run them, and can point at the flow by its ref.
- **Ordering inside a flow.** A flow is a set of use cases, not a sequence.
  A journey orders use cases when the order matters.
- **A `--flow` filter for `craft generate`.** `--use-case` with the names in
  `RealisedBy` draws the same diagrams.
