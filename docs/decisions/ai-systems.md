# AI systems (v2.37)

**Status:** accepted, 2026-11-09.

## Context

Real Estate's FY27 goal counts "AI user-journey coverage": the share of user journeys that use an AI system. The journey model already names the external systems use cases call, as actors (`system DescriptionGenerator "description generator"`). Nothing said which of them are AI systems, or of what kind, so the count was kept by hand next to the model.

## Decision

- **Classify the system, not the use case.** An actor may carry an optional body, `{ ai: … owner: … }`, on its declaration. `ai:` is one of `ml` (a trained model), `genai` (a generative model) or `agentic` (an agent that acts); `owner:` is who runs the system ("OLX Data", "Toqan"). A use case that calls a classified actor is an AI use case, and a journey that holds one is an AI journey. Coverage is derived from the model; nobody writes it down.
- **Why not a tag on each use case.** One system is called by many use cases. A tag per use case repeats the same fact many times, drifts when a use case starts or stops calling the system, and is wrong the day the system changes kind. Declared once on the actor, the fact has one place to change.
- **Why not a flag per service.** A service is a deployment unit, not a model. One data API can serve several models, of different kinds; flagging the service as AI counts every use case that calls it for anything, which over-counts. Split such a service into one actor per model it serves (`system PriceModel { ai: ml }`, `system DescriptionGenerator { ai: genai }`), so each call names the model it uses.
- **Three kinds, in ascending order of advancement.** `AIKinds()` returns `ml`, `genai`, `agentic` in that order, so a consumer can report the most advanced kind a journey reaches. The list is closed (`craft/sema/invalid-ai-kind`): a free value would split one kind over spellings.
- **First declaration wins.** An actor declared in several files is the first declaration in sorted file order, as for display names (`CraftDoc.ActorByName`). A later declaration with another `ai:` is `craft/lint/conflicting-ai-kind`.
- **Additive.** An actor without a body parses and serialises as in 2.36 (no `ai`, `owner` or `notes` JSON key); the corpus goldens are unchanged.

## Out of scope

- Computing coverage. Craft gives the classification and the calls; the consumer (the hub, a report) counts journeys.
- Drawing AI systems differently in diagrams.
- Model versions, vendors' model names and evaluation status. They change faster than the model; a `note` line can mention them.
