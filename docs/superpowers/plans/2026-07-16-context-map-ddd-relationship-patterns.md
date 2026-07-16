# context_map DDD Strategic Relationship Patterns — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the eight DDD strategic context-mapping relationship patterns as first-class `context_map` edge verbs connecting two `bc:` endpoints, validated by shape (not semantic truth), exported for consumers, and mirrored in the tree-sitter grammar.

**Architecture:** No new grammar shape — relationship edges are ordinary `edge_stmt`s (`<ref> <verb> <ref>`). Extend the single source-of-truth verb list (`internal/syntax.edgeKeywords`), add a third endpoint-kind category (`bc:`→`bc:`) to sema's `classifyEdgeVerb`, add a canonical verb-metadata table in `internal/syntax` (category + symmetry + endpoint kinds) that both `internal/sema` and `pkg/craft` derive from, expose a stable `pkg/craft` API, and add the verb literals to the tree-sitter grammar.

**Tech Stack:** Go 1.23 (`github.com/tcarcao/craft/v2`), tree-sitter (`tree-sitter-craft`, JS grammar + generated C), goreleaser on `v*` tags.

**Spec:** `docs/superpowers/specs/2026-07-16-context-map-ddd-relationship-patterns.md` (approved 2026-07-16; direction convention settled: **LEFT = upstream**).

## Global Constraints

- **Direction convention: LEFT = upstream, RIGHT = downstream** for all five directional verbs. Every directional edge reads `bc:<upstream> <verb> bc:<downstream>`. Encoded once in the metadata table; consumers read direction from craft, never re-derive.
- **The eight verbs (exact spelling):** directional — `customer_supplier`, `conformist`, `anticorruption_layer`, `open_host_service`, `published_language`; symmetric — `partnership`, `shared_kernel`, `separate_ways`.
- **`big_ball_of_mud` is NOT added** (it is a zone marker, not a pairwise edge — spec §11).
- **Relationship edges require `bc:` on BOTH endpoints.** `bc:<domain>/<name>` slug shape, per the existing ref rule.
- **craft validates shape/kinds, NOT semantic truth.** No check that a direction is "correct", no conflicting-pattern check, no completeness check. (Spec §1.1, §6.)
- **One pattern per edge statement.** No bracket-list grammar (spec §11).
- **`internal/syntax.edgeKeywords` stays the single source of truth** gating the parser; sema maps and the `pkg/craft` API must agree with it, enforced by sync tests (mirrors the existing `EdgeKeywords()` sync-test discipline).
- **No import cycles:** types live in `internal/model`; the verb-metadata table lives in `internal/syntax` (importable by both `internal/sema` and `pkg/craft`); `pkg/craft` re-exports via wrappers. `sema` must NOT import `pkg/craft`.
- **Module path is `/v2`** — all internal imports use `github.com/tcarcao/craft/v2/...`.
- **Lossless green tree / byte-for-byte round-trip** is preserved (no parser structural change; verbs are ordinary `SyntaxKindEdgeKw` tokens).
- **New minor release: v2.12.0.**

---

## File Structure

- `internal/syntax/parser.go` — extend `edgeKeywords` (+8); categorize the "expected an edge keyword" diagnostic.
- `internal/syntax/edge_meta.go` *(new)* — canonical verb-metadata table (category, symmetric, endpoint kinds) + exported accessors. Single source of truth for verb *semantics*; `edgeKeywords` remains the parser gate and is asserted equal to this table's keys.
- `internal/sema/validate.go` — `edgeRelationshipVerbs` membership map; `bc:`→`bc:` branch in `classifyEdgeVerb`; `self-relationship` error; `redundant-relationship` symmetric lint in `validateContextMapEdges`.
- `internal/sema/edge_verb_sync_test.go` — extend the vocabulary-sync test to include `edgeRelationshipVerbs` and to assert agreement with the `internal/syntax` metadata table.
- `internal/sema/validate_relationship_test.go` *(new, whitebox)* — `classifyEdgeVerb` relationship cases + self-relationship + redundant lint.
- `pkg/craft/edges.go` *(new)* — `EdgeCategory`, `EdgeVerbInfo`, `EdgeVerbs()`, `LookupEdgeVerb()` wrapping the `internal/syntax` table.
- `pkg/craft/edges_test.go` *(new)* — sync/coverage test.
- `testdata/corpus/05_context_map/relationships.craft` + `.craftjson` *(new)* — golden.
- `testdata/broken/relationship_bad_endpoint.{craft,diagnostics.json}`, `testdata/broken/relationship_self.{craft,diagnostics.json}`, `testdata/broken/relationship_redundant.{craft,diagnostics.json}` *(new)*.
- `CHANGELOG.md` — v2.12.0 entry.
- `tree-sitter-craft/grammar.js` — `edge_verb` += 8 literals; regenerate `src/`; corpus test.

---

## Task 1: Recognize relationship verbs + `bc:`→`bc:` endpoint-kind validation

**Files:**
- Modify: `internal/syntax/parser.go` (`edgeKeywords` ~line 2003; the "an edge keyword (…)" diagnostic ~line 1924)
- Modify: `internal/sema/validate.go` (`edgeTermVerbs` ~line 25; `classifyEdgeVerb` ~line 279)
- Modify: `internal/sema/edge_verb_sync_test.go` (`TestEdgeVerbVocabulariesInSync`)
- Create: `internal/sema/validate_relationship_test.go`

**Interfaces:**
- Consumes: existing `classifyEdgeVerb(uri, verb, leftKind, rightKind, left, right string, leftLine, leftCol int) []model.Diagnostic`; `edgeEndpointKindDiag(uri, verb, requirement, left, right string, line, col int) model.Diagnostic`; `syntax.EdgeKeywords() []string`.
- Produces: `edgeRelationshipVerbs` (unexported `map[string]bool` in `internal/sema`); the 8 verbs present in `syntax.edgeKeywords`.

- [ ] **Step 1: Write the failing test** — `internal/sema/validate_relationship_test.go` (whitebox `package sema`):

```go
// Whitebox: classifyEdgeVerb and edgeRelationshipVerbs are unexported.
package sema

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func TestClassifyEdgeVerb_Relationship_BcToBc_OK(t *testing.T) {
	for _, verb := range []string{
		"customer_supplier", "conformist", "anticorruption_layer",
		"open_host_service", "published_language",
		"partnership", "shared_kernel", "separate_ways",
	} {
		diags := classifyEdgeVerb("f.craft", verb, "bc", "bc", "bc:re/a", "bc:re/b", 1, 0)
		if len(diags) != 0 {
			t.Errorf("%s bc->bc: want 0 diagnostics, got %d: %+v", verb, len(diags), diags)
		}
	}
}

func TestClassifyEdgeVerb_Relationship_WrongKind(t *testing.T) {
	diags := classifyEdgeVerb("f.craft", "customer_supplier", "bc", "service", "bc:re/a", "service:x", 1, 0)
	if len(diags) != 1 || diags[0].Code != "craft/sema/edge-endpoint-kind" {
		t.Fatalf("want 1 edge-endpoint-kind diag, got %+v", diags)
	}
	if diags[0].Severity != model.SeverityError {
		t.Errorf("severity = %q, want error", diags[0].Severity)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/sema/ -run TestClassifyEdgeVerb_Relationship`
Expected: FAIL — the verbs fall to the `default:` branch and return a `craft/sema/unrecognised-edge-verb` diagnostic (so `BcToBc_OK` gets 1 diag not 0, and `WrongKind` gets the wrong code).

- [ ] **Step 3: Extend `edgeKeywords`** in `internal/syntax/parser.go`:

```go
// edgeKeywords is the single source of truth for context_map edge verbs.
var edgeKeywords = []string{
	"realized_by", "also_realizes", // realization: bc: -> service:
	"same_as", "contrasts", "distinct_from", // term relation: term: -> term:
	// relationship patterns: bc: -> bc: (LEFT = upstream)
	"customer_supplier", "conformist", "anticorruption_layer",
	"open_host_service", "published_language",
	"partnership", "shared_kernel", "separate_ways",
}
```

- [ ] **Step 4: Categorize the edge-keyword diagnostic** in `internal/syntax/parser.go` (~line 1924). Replace the single-line message:

```go
	diags = append(diags, p.diagUnexpected(verb, "an edge keyword: a realization (realized_by/also_realizes), a term relation (same_as/contrasts/distinct_from), or a relationship pattern (customer_supplier/conformist/anticorruption_layer/open_host_service/published_language/partnership/shared_kernel/separate_ways)"))
```

- [ ] **Step 5: Add the sema membership map + relationship branch** in `internal/sema/validate.go`. After `edgeTermVerbs` (~line 25):

```go
// edgeRelationshipVerbs require a bc: left endpoint and a bc: right endpoint.
var edgeRelationshipVerbs = map[string]bool{
	"customer_supplier": true, "conformist": true, "anticorruption_layer": true,
	"open_host_service": true, "published_language": true,
	"partnership": true, "shared_kernel": true, "separate_ways": true,
}
```

In `classifyEdgeVerb`, add a case before `default:`:

```go
	case edgeRelationshipVerbs[verb]:
		if leftKind != "bc" || rightKind != "bc" {
			return []model.Diagnostic{edgeEndpointKindDiag(uri, verb, "`bc:` endpoints on both sides", left, right, leftLine, leftCol)}
		}
```

- [ ] **Step 6: Keep the vocabulary sync test honest** — in `internal/sema/edge_verb_sync_test.go`, `TestEdgeVerbVocabulariesInSync`, add the relationship map to the `sema` set:

```go
	for k := range edgeRelationshipVerbs {
		sema[k] = true
	}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/sema/ ./internal/syntax/`
Expected: PASS — relationship verbs classify bc→bc with 0 diags, wrong-kind emits `edge-endpoint-kind`, and `TestEdgeVerbVocabulariesInSync` passes (parser's 13 keywords == union of the three sema maps).

- [ ] **Step 8: Commit**

```bash
git add internal/syntax/parser.go internal/sema/validate.go internal/sema/edge_verb_sync_test.go internal/sema/validate_relationship_test.go
git commit -m "feat(sema): context_map relationship verbs (bc:->bc:) + endpoint-kind validation"
```

---

## Task 2: `self-relationship` error

**Files:**
- Modify: `internal/sema/validate.go` (`classifyEdgeVerb` relationship branch)
- Modify: `internal/sema/validate_relationship_test.go`
- Create: `testdata/broken/relationship_self.craft`, `testdata/broken/relationship_self.diagnostics.json`

**Interfaces:**
- Consumes: `edgeRelationshipVerbs`, `colToLSP`, `lineToLSP` (existing helpers used by `classifyEdgeVerb`).
- Produces: diagnostic code `craft/sema/self-relationship` (severity error), emitted when `left == right` for a relationship verb with both `bc:` endpoints.

- [ ] **Step 1: Write the failing test** — add to `internal/sema/validate_relationship_test.go`:

```go
func TestClassifyEdgeVerb_SelfRelationship(t *testing.T) {
	diags := classifyEdgeVerb("f.craft", "partnership", "bc", "bc", "bc:re/a", "bc:re/a", 1, 0)
	if len(diags) != 1 || diags[0].Code != "craft/sema/self-relationship" {
		t.Fatalf("want 1 self-relationship diag, got %+v", diags)
	}
	if diags[0].Severity != model.SeverityError {
		t.Errorf("severity = %q, want error", diags[0].Severity)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run TestClassifyEdgeVerb_SelfRelationship`
Expected: FAIL — currently `bc:re/a partnership bc:re/a` classifies as valid bc→bc (0 diags).

- [ ] **Step 3: Add the self-check** in `classifyEdgeVerb`, relationship branch (after the kind check, both endpoints known `bc:`):

```go
	case edgeRelationshipVerbs[verb]:
		if leftKind != "bc" || rightKind != "bc" {
			return []model.Diagnostic{edgeEndpointKindDiag(uri, verb, "`bc:` endpoints on both sides", left, right, leftLine, leftCol)}
		}
		if left == right {
			startChar := colToLSP(leftCol)
			return []model.Diagnostic{{
				Code:      "craft/sema/self-relationship",
				Message:   fmt.Sprintf("edge %q %s %q relates a bounded context to itself", left, verb, right),
				Severity:  model.SeverityError,
				SourceURI: uri,
				Range: model.Range{
					Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
					End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + len(left)},
				},
			}}
		}
```

- [ ] **Step 4: Add the broken fixture** — `testdata/broken/relationship_self.craft`:

```craft
context_map {
  bc:re/billing partnership bc:re/billing
}
```

Generate `testdata/broken/relationship_self.diagnostics.json` by running the broken-corpus harness in update/print mode (same mechanism the existing `testdata/broken/*.diagnostics.json` use — inspect a sibling like `testdata/broken/edge_endpoint_kind.diagnostics.json` for the exact JSON shape and the field ordering, then write the file to match the actual emitted diagnostic: one `craft/sema/self-relationship` error at the left endpoint's range). Verify by running the broken-fixture test (below) and copying the produced diagnostics if the harness supports an update flag; otherwise hand-write to match the sibling's schema exactly.

- [ ] **Step 5: Run the broken-fixture + unit tests**

Run: `go test ./internal/sema/ && go test ./... -run Broken`
Expected: PASS — `relationship_self` produces exactly the recorded `self-relationship` diagnostic; unit test passes.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/validate.go internal/sema/validate_relationship_test.go testdata/broken/relationship_self.craft testdata/broken/relationship_self.diagnostics.json
git commit -m "feat(sema): self-relationship error for bc:X <verb> bc:X"
```

---

## Task 3: Corpus golden + broken-endpoint fixture (round-trip)

**Files:**
- Create: `testdata/corpus/05_context_map/relationships.craft` + `testdata/corpus/05_context_map/relationships.craftjson`
- Create: `testdata/broken/relationship_bad_endpoint.craft` + `.diagnostics.json`

**Interfaces:**
- Consumes: `craft.Parse`, the corpus round-trip harness, the corpus golden regeneration method (the same one used for the tags feature — parse the `.craft` and serialize the resulting `CraftDoc` to the `.craftjson` schema).
- Produces: a valid corpus fixture exercising a directional and a symmetric relationship edge; goldens containing `Edge{Left, Verb, Right}` entries.

> **First:** confirm the corpus directory layout — `ls testdata/corpus/`. If a context_map-specific directory already exists, use it; otherwise create `05_context_map/`. Match the numeric-prefix convention of the siblings.

- [ ] **Step 1: Write the corpus fixture** — `testdata/corpus/05_context_map/relationships.craft`:

```craft
context_map {
  bc:re/billing customer_supplier bc:re/vas
  bc:re/billing anticorruption_layer bc:re/subscriptions
  bc:re/subscriptions partnership bc:re/billing
}
```

- [ ] **Step 2: Generate the golden** — regenerate `testdata/corpus/05_context_map/relationships.craftjson` using the corpus golden tool/method (verify the diff contains three `Edge` entries with `left`/`verb`/`right` exactly as authored, and that the file matches the schema of an existing `*.craftjson` that has a `contextMap` array — grep the corpus for one).

- [ ] **Step 3: Run the corpus round-trip + acceptance tests**

Run: `go test ./... -run 'Corpus|RoundTrip'`
Expected: PASS — the fixture parses, the golden matches, and byte-for-byte round-trip holds (verb tokens are ordinary `SyntaxKindEdgeKw`, no trivia lost).

- [ ] **Step 4: Add the broken-endpoint fixture** — `testdata/broken/relationship_bad_endpoint.craft`:

```craft
context_map {
  bc:re/billing customer_supplier term:billing/Invoice
}
```

Generate `testdata/broken/relationship_bad_endpoint.diagnostics.json` (matching the sibling schema; expected: one `craft/sema/edge-endpoint-kind` error — right endpoint is `term:`, not `bc:`).

- [ ] **Step 5: Run broken-fixture tests**

Run: `go test ./... -run Broken`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add testdata/corpus/05_context_map/ testdata/broken/relationship_bad_endpoint.craft testdata/broken/relationship_bad_endpoint.diagnostics.json
git commit -m "test: corpus golden + broken-endpoint fixtures for relationship edges"
```

---

## Task 4: Canonical verb-metadata table + `pkg/craft` public API

**Files:**
- Create: `internal/syntax/edge_meta.go`
- Modify: `internal/sema/edge_verb_sync_test.go` (assert sema maps agree with the metadata table)
- Create: `pkg/craft/edges.go`
- Create: `pkg/craft/edges_test.go`

**Interfaces:**
- Produces (internal/syntax):
  ```go
  type EdgeCategory string
  const (
      EdgeRealization  EdgeCategory = "realization"
      EdgeTermRelation EdgeCategory = "term_relation"
      EdgeRelationship EdgeCategory = "relationship"
  )
  type EdgeVerbMeta struct {
      Verb      string
      Category  EdgeCategory
      Symmetric bool
      LeftKind  string // "bc" | "service" | "term"
      RightKind string // "bc" | "service" | "term"
  }
  func EdgeVerbMetas() []EdgeVerbMeta            // all verbs, all categories (ordered)
  func LookupEdgeVerbMeta(verb string) (EdgeVerbMeta, bool)
  func EdgeVerbSymmetric(verb string) bool       // convenience for sema's lint
  ```
- Produces (pkg/craft): thin wrappers `EdgeCategory`/`EdgeVerbInfo`/`EdgeVerbs()`/`LookupEdgeVerb()` over the internal table (spec §4 public surface). `EdgeVerbInfo` mirrors `EdgeVerbMeta` field-for-field.
- Consumes: `edgeKeywords` (assert `EdgeVerbMetas()` keys == `edgeKeywords`).

- [ ] **Step 1: Write the failing test** — `pkg/craft/edges_test.go`:

```go
package craft_test

import (
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func TestEdgeVerbs_CoverAllVerbsWithDirection(t *testing.T) {
	got := map[string]craft.EdgeVerbInfo{}
	for _, v := range craft.EdgeVerbs() {
		got[v.Verb] = v
	}
	// Directional relationship verb: left=upstream, right=downstream, not symmetric.
	cs, ok := craft.LookupEdgeVerb("customer_supplier")
	if !ok || cs.Category != craft.EdgeRelationship || cs.Symmetric || cs.LeftKind != "bc" || cs.RightKind != "bc" {
		t.Fatalf("customer_supplier = %+v (ok=%v), want relationship/bc->bc/asymmetric", cs, ok)
	}
	// Symmetric relationship verb.
	p, _ := craft.LookupEdgeVerb("partnership")
	if !p.Symmetric {
		t.Errorf("partnership.Symmetric = false, want true")
	}
	// Existing categories still present.
	if r, _ := craft.LookupEdgeVerb("realized_by"); r.Category != craft.EdgeRealization || r.RightKind != "service" {
		t.Errorf("realized_by = %+v, want realization/->service", r)
	}
	if len(got) == 0 {
		t.Fatal("EdgeVerbs() returned nothing")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./pkg/craft/ -run TestEdgeVerbs`
Expected: FAIL — `pkg/craft` has no `EdgeVerbs`/`EdgeVerbInfo`/`EdgeCategory` yet (compile error).

- [ ] **Step 3: Write the metadata table** — `internal/syntax/edge_meta.go`. Include ALL 13 verbs across the three categories. Directional relationship verbs: `Symmetric:false`; `partnership`/`shared_kernel`/`separate_ways`: `Symmetric:true`; all relationship verbs `LeftKind:"bc", RightKind:"bc"`. Realization: `bc`→`service`. Term relation: `term`→`term`, symmetric where it applies (`same_as` symmetric; `contrasts`/`distinct_from` — set `Symmetric:false` unless the term category already documents symmetry; default false, note in a comment). Provide `EdgeVerbMetas()` returning them in a stable order, `LookupEdgeVerbMeta`, and `EdgeVerbSymmetric`.

- [ ] **Step 4: Write the public wrapper** — `pkg/craft/edges.go`:

```go
package craft

import "github.com/tcarcao/craft/v2/internal/syntax"

type EdgeCategory string

const (
	EdgeRealization  EdgeCategory = EdgeCategory(syntax.EdgeRealization)
	EdgeTermRelation EdgeCategory = EdgeCategory(syntax.EdgeTermRelation)
	EdgeRelationship EdgeCategory = EdgeCategory(syntax.EdgeRelationship)
)

// EdgeVerbInfo describes one context_map edge verb. LEFT = upstream for
// directional relationship verbs (see the spec's direction convention).
type EdgeVerbInfo struct {
	Verb      string
	Category  EdgeCategory
	Symmetric bool
	LeftKind  string
	RightKind string
}

func EdgeVerbs() []EdgeVerbInfo {
	metas := syntax.EdgeVerbMetas()
	out := make([]EdgeVerbInfo, 0, len(metas))
	for _, m := range metas {
		out = append(out, fromMeta(m))
	}
	return out
}

func LookupEdgeVerb(verb string) (EdgeVerbInfo, bool) {
	m, ok := syntax.LookupEdgeVerbMeta(verb)
	if !ok {
		return EdgeVerbInfo{}, false
	}
	return fromMeta(m), true
}

func fromMeta(m syntax.EdgeVerbMeta) EdgeVerbInfo {
	return EdgeVerbInfo{Verb: m.Verb, Category: EdgeCategory(m.Category), Symmetric: m.Symmetric, LeftKind: m.LeftKind, RightKind: m.RightKind}
}
```

- [ ] **Step 5: Add the source-of-truth sync assertions** — in `internal/sema/edge_verb_sync_test.go`, add a test that the metadata table's keys equal `syntax.EdgeKeywords()`, and that every relationship-category verb in the table is in `edgeRelationshipVerbs` (and vice versa), every realization verb in `edgeRealizationVerbs`, every term verb in `edgeTermVerbs`:

```go
func TestEdgeMetaMatchesSemaMaps(t *testing.T) {
	byVerb := map[string]syntax.EdgeVerbMeta{}
	for _, m := range syntax.EdgeVerbMetas() {
		byVerb[m.Verb] = m
	}
	// keys == edgeKeywords
	if len(byVerb) != len(syntax.EdgeKeywords()) {
		t.Fatalf("EdgeVerbMetas has %d verbs, EdgeKeywords has %d", len(byVerb), len(syntax.EdgeKeywords()))
	}
	for _, k := range syntax.EdgeKeywords() {
		m, ok := byVerb[k]
		if !ok {
			t.Fatalf("edgeKeywords verb %q missing from EdgeVerbMetas", k)
		}
		switch m.Category {
		case syntax.EdgeRelationship:
			if !edgeRelationshipVerbs[k] {
				t.Errorf("%q is relationship in meta but not in edgeRelationshipVerbs", k)
			}
		case syntax.EdgeRealization:
			if !edgeRealizationVerbs[k] {
				t.Errorf("%q is realization in meta but not in edgeRealizationVerbs", k)
			}
		case syntax.EdgeTermRelation:
			if !edgeTermVerbs[k] {
				t.Errorf("%q is term in meta but not in edgeTermVerbs", k)
			}
		}
	}
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/syntax/ ./internal/sema/ ./pkg/craft/`
Expected: PASS — public API resolves direction/symmetry from the single table; all sync assertions hold.

- [ ] **Step 7: Commit**

```bash
git add internal/syntax/edge_meta.go internal/sema/edge_verb_sync_test.go pkg/craft/edges.go pkg/craft/edges_test.go
git commit -m "feat(craft): exported edge-verb metadata API (category/symmetry/kinds), single source in internal/syntax"
```

---

## Task 5: `redundant-relationship` symmetric-duplicate lint

**Files:**
- Modify: `internal/sema/validate.go` (`validateContextMapEdges` loop, ~line 249)
- Modify: `internal/sema/validate_relationship_test.go`
- Create: `testdata/broken/relationship_redundant.craft` + `.diagnostics.json`

**Interfaces:**
- Consumes: `syntax.EdgeVerbSymmetric(verb) bool` (from Task 4); the loop's existing `left, right, verb` strings and `leftLine, leftCol`.
- Produces: diagnostic code `craft/lint/redundant-relationship` (severity **warning**), emitted on the SECOND occurrence of the same unordered pair with the same symmetric verb. Directional duplicates in opposite order are NOT flagged.

- [ ] **Step 1: Write the failing test** — add to `internal/sema/validate_relationship_test.go` a blackbox-style check via the file validator, OR a focused unit test if `validateContextMapEdges` is callable. Prefer a broken-fixture-driven test (Step 4). For a unit-level guard, add:

```go
func TestRedundantSymmetric_FlaggedOnce(t *testing.T) {
	// Same unordered pair + same symmetric verb, twice -> one warning.
	// A partnership B then B partnership A.
	// (Exercise through validateContextMapEdges via a parsed file; see helper
	// pattern in validate_test.go for constructing a syntax.File from source.)
}
```
Fill this in using the same file-construction helper the existing `internal/sema/validate_test.go` uses (find it: `grep -n "func Test" internal/sema/validate_test.go` and reuse its parse-to-`syntax.File` setup). Assert exactly one `craft/lint/redundant-relationship` warning for the duplicated symmetric pair, and ZERO for a directional pair authored in both orders.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/sema/ -run Redundant`
Expected: FAIL — no such lint yet.

- [ ] **Step 3: Implement the lint** in `validateContextMapEdges`. Before the `cm` loop, declare `seen := map[string]bool{}`. Inside the edge loop, after `classifyEdgeVerb`:

```go
			if edgeRelationshipVerbs[verb] && syntax.EdgeVerbSymmetric(verb) {
				a, b := left, right
				if a > b {
					a, b = b, a
				}
				key := a + "\x00" + b + "\x00" + verb
				if seen[key] {
					startChar := colToLSP(leftCol)
					diags = append(diags, model.Diagnostic{
						Code:      "craft/lint/redundant-relationship",
						Message:   fmt.Sprintf("redundant %s relationship: %q and %q already declared this symmetric pair", verb, left, right),
						Severity:  model.SeverityWarning,
						SourceURI: uri,
						Range: model.Range{
							Start: model.Position{Line: lineToLSP(leftLine), Character: startChar},
							End:   model.Position{Line: lineToLSP(leftLine), Character: startChar + len(left)},
						},
					})
				}
				seen[key] = true
			}
```

(Confirm `syntax` is already imported in `validate.go`; it is — the file uses `syntax.File`.)

- [ ] **Step 4: Add the broken/lint fixture** — `testdata/broken/relationship_redundant.craft`:

```craft
context_map {
  bc:re/a partnership bc:re/b
  bc:re/b partnership bc:re/a
}
```

Generate `.diagnostics.json`: exactly one `craft/lint/redundant-relationship` warning on the second line's left endpoint. (If the broken harness only accepts errors, place this as a corpus-with-diagnostics fixture matching whatever mechanism records warning-level diagnostics — check how `craft/sema/duplicate-tag` warnings are captured in `testdata/broken/` from the tags feature and mirror it.)

- [ ] **Step 5: Run tests**

Run: `go test ./internal/sema/ && go test ./... -run Broken`
Expected: PASS — one warning for the symmetric duplicate; the directional-both-orders case yields none.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/validate.go internal/sema/validate_relationship_test.go testdata/broken/relationship_redundant.craft testdata/broken/relationship_redundant.diagnostics.json
git commit -m "feat(sema): redundant-relationship lint for duplicated symmetric pairs"
```

---

## Task 6: tree-sitter grammar sync

**Repo:** `tree-sitter-craft` (SEPARATE repo at `/Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft`). Branch off current `main`. tree-sitter CLI at `node_modules/.bin/tree-sitter` (0.25.10). Two commits (source, then regenerated `src/`). Do NOT push. `git config commit.gpgsign false` in this repo if signing fails.

**Files:**
- Modify: `grammar.js` (`edge_verb` rule ~line 48)
- Modify: `test/corpus/*` (add relationship edge cases)
- Regenerate: `src/grammar.json`, `src/node-types.json`, `src/parser.c`

**Interfaces:**
- Consumes: existing `edge_verb: $ => choice('realized_by', ...)` and `edge_stmt: $ => seq($.ref, $.edge_verb, $.ref)`.
- Produces: `edge_verb` accepting all 13 verbs. NO highlight change needed — `queries/highlights.scm` captures the whole `(edge_verb)` node (verified: `(edge_verb) @craft.edge-verb`).

- [ ] **Step 1: Add the 8 literals** to `edge_verb` in `grammar.js`:

```js
edge_verb: $ => choice(
  'realized_by', 'also_realizes',
  'same_as', 'contrasts', 'distinct_from',
  'customer_supplier', 'conformist', 'anticorruption_layer',
  'open_host_service', 'published_language',
  'partnership', 'shared_kernel', 'separate_ways',
),
```

- [ ] **Step 2: Regenerate and run existing corpus**

Run: `cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft && node_modules/.bin/tree-sitter generate && npm test`
Expected: PASS — existing corpus tests still green (no ambiguity: verbs are distinct literals in the `edge_verb` slot).

- [ ] **Step 3: Add a corpus test** — in `test/corpus/real_examples.txt` (or a context_map corpus file), add a case with a directional and a symmetric relationship edge inside `context_map`. Generate the expected tree with `node_modules/.bin/tree-sitter parse` (do not hand-write blind) and paste it as the expected S-expression.

- [ ] **Step 4: Run corpus + full craft corpus compat locally**

Run: `npm test` — new case passes.
Run (full corpus gate parity): `echo` the current `CORPUS_VERSION`; the hard-fail compat gate fetches the craft corpus at that pin. Since the new craft relationship corpus fixture ships in the NEXT craft tag, this grammar change does not need a `CORPUS_VERSION` bump now; the existing 57-file corpus must still parse: `bash scripts/fetch-corpus.sh && F=0;T=0; while IFS= read -r -d '' f; do T=$((T+1)); node_modules/.bin/tree-sitter parse "$f" >/dev/null 2>&1 || { F=$((F+1)); echo "FAIL $f"; }; done < <(find test/corpus-from-craft -name '*.craft' -print0); echo "$T files, $F failed"`
Expected: `57 files, 0 failed`.

- [ ] **Step 5: Check the VS Code extension** — determine whether `craft-vscode-extension` highlights edge verbs via a separate TextMate grammar list (as it does for block keywords). If it enumerates edge verbs, add the 8 there (uncommitted note for the human, per prior precedent — separate repo, no commit protocol). If edge verbs are not separately listed, nothing to do; state which.

- [ ] **Step 6: Commit (two commits, no push)**

```bash
git add grammar.js queries test/corpus
git -c commit.gpgsign=false commit -m "feat(tree-sitter): DDD relationship edge verbs in context_map"
git add src
git -c commit.gpgsign=false commit -m "chore(tree-sitter): regenerate parser for relationship edge verbs"
```

Report the two commit SHAs and branch; do NOT push.

---

## Task 7: CHANGELOG + release v2.12.0 (GATED on user confirmation)

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add the CHANGELOG entry** — a new `## [2.12.0]` section: "context_map DDD strategic relationship patterns (customer_supplier, conformist, anticorruption_layer, open_host_service, published_language, partnership, shared_kernel, separate_ways) as bc:→bc: edge verbs; exported `pkg/craft` edge-verb metadata API (`EdgeVerbs`/`LookupEdgeVerb`); `self-relationship` error and `redundant-relationship` lint." Match the format of the existing `## [2.11.0]` entry.

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog for 2.12.0 (context_map DDD relationship patterns)"
```

- [ ] **Step 3: Whole-branch verification**

Run: `go build ./... && go test ./...`
Expected: all green.

- [ ] **Step 4: Release (only after explicit user confirmation).** Merge `feat/context-map-ddd-relationships` → `main` (`--no-ff`), re-run `go test ./...` on the merged result, tag `v2.12.0`, push `main` + tag (triggers goreleaser + Docker publish). Push the tree-sitter `main` changes from Task 6. Consumer migration (the RE knowledge-hub dropping `bc-relationships.craft`) is **informational only** (spec §10) — a separate repo/PR, not part of this branch.

---

## Notes carried from the spec (do not re-litigate during implementation)

- **Explicitly NOT validated:** direction "correctness", conflicting patterns on a pair, completeness. Craft checks shape, not truth (spec §6).
- **Out of scope:** `big_ball_of_mud`; multiple patterns per edge statement; role sub-annotations (naming the ACL / published language). Spec §11.
- **Consumer migration** is documented in spec §10 for the hub team; it is not a craft-repo change and is not in this plan's scope beyond shipping the API that unblocks it.
