# DSL Revision: Bounded Contexts Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename DSL terminology from "subdomains" to "bounded contexts" across the ANTLR grammar, Go parser, visualizer, tree-sitter grammar, VS Code extension, examples, and docs.

**Architecture:** Grammar changes propagate top-down: ANTLR grammar is updated first and regenerated, then Go visitor code is updated to use the new generated types, then the tree-sitter grammar (separate repo) is updated and published, then the VS Code extension consumes the new grammar. Two user-visible DSL keyword changes: `domains:` → `contexts:` in service blocks, `of:` → `contexts:` in exposure blocks.

**Tech Stack:** Go 1.22, ANTLR4 4.13.2, tree-sitter, TypeScript/React (VS Code extension), Podman/Docker for grammar generation.

**Spec:** `docs/superpowers/specs/2026-04-20-dsl-revision-and-docker-publishing-design.md`

---

## Chunk 1: ANTLR Grammar + Go Parser

### Task 1: Update test DSL strings to use new keywords (tests will fail — that's expected)

**Files:**
- Modify: `internal/parser/services_visitor_test.go`
- Modify: `internal/parser/user_management_test.go`
- Modify: `internal/parser/domains_visitor_test.go`

- [ ] **Step 1: Update services_visitor_test.go DSL strings**

Find every occurrence of `domains:` in test DSL strings and replace with `contexts:`. Run:
```bash
grep -n "domains:" /Users/tiago.carcao/projects/poc/craft-project/craft/internal/parser/services_visitor_test.go
```
Replace each match. Example change:
```go
// Before
dsl := `services {
  UserService {
    domains: Authentication, Profile
  }
}`

// After
dsl := `services {
  UserService {
    contexts: Authentication, Profile
  }
}`
```

- [ ] **Step 2: Update user_management_test.go DSL strings**

Same replacement — find `domains:` and `of:` in test DSL strings:
```bash
grep -n "domains:\|of:" /Users/tiago.carcao/projects/poc/craft-project/craft/internal/parser/user_management_test.go
```
Replace `domains:` → `contexts:` and `of:` → `contexts:`.

- [ ] **Step 3: Update domains_visitor_test.go if it contains service or exposure DSL**

```bash
grep -n "domains:\|of:" /Users/tiago.carcao/projects/poc/craft-project/craft/internal/parser/domains_visitor_test.go
```

- [ ] **Step 4: Run tests to confirm they fail for the right reason**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/parser/...
```
Expected: failures due to unrecognized token `contexts` (parser still uses old grammar). If tests fail for a different reason, investigate before continuing.

---

### Task 2: Update ANTLR grammar

**Files:**
- Modify: `tools/antlr-grammar/Craft.g4`

- [ ] **Step 1: Rename subdomain rules**

In `Craft.g4`, make these changes:

```diff
-subdomain_list: subdomain (NEWLINE+ subdomain)* NEWLINE*;
-subdomain: identifier;
+bounded_context_list: bounded_context (NEWLINE+ bounded_context)* NEWLINE*;
+bounded_context: identifier;
```

Update references in `domain_def` and `domain_block`:
```diff
-domain_def: 'domain' domain_name '{' NEWLINE* subdomain_list '}' NEWLINE*;
+domain_def: 'domain' domain_name '{' NEWLINE* bounded_context_list '}' NEWLINE*;

-domain_block: domain_name '{' NEWLINE* subdomain_list '}';
+domain_block: domain_name '{' NEWLINE* bounded_context_list '}';
```

- [ ] **Step 2: Rename domain_list and domain_ref to context_list and context_ref**

```diff
-domain_list: domain_ref (',' domain_ref)* ','?;
-domain_ref: identifier;
+context_list: context_ref (',' context_ref)* ','?;
+context_ref: identifier;
```

Update all references to `domain_list` → `context_list` and `domain_ref` → `context_ref` throughout the file. Check these locations:
```bash
grep -n "domain_list\|domain_ref" /Users/tiago.carcao/projects/poc/craft-project/craft/tools/antlr-grammar/Craft.g4
```

- [ ] **Step 3: Rename DOMAINS token → CONTEXTS**

```diff
-DOMAINS: 'domains';
+CONTEXTS: 'contexts';
```

Update `service_property` to use `CONTEXTS`:
```diff
-service_property: DOMAINS ':' domain_list
+service_property: CONTEXTS ':' context_list
                 | DATA_STORES ':' datastore_list
                 | LANGUAGE ':' identifier
                 | DEPLOYMENT ':' deployment_strategy
                 ;
```

Update the `identifier` rule — the grammar has TWO places where `domains` appears: the `DOMAINS` token reference (line ~201) and a bare string literal `'domains'` (line ~176). Both must change:

```diff
-          | 'domains'
+          | 'contexts'
```
```diff
-          | DOMAINS      // 'domains' token
+          | CONTEXTS     // 'contexts' token
```

Run this to find both locations before editing:
```bash
grep -n "domains\|DOMAINS" /Users/tiago.carcao/projects/poc/craft-project/craft/tools/antlr-grammar/Craft.g4
```

- [ ] **Step 4: Update exposure_property — change `of:` to `contexts:`**

```diff
 exposure_property: 'to' ':' target_list
-                 | 'of' ':' domain_list
+                 | CONTEXTS ':' context_list
                  | 'through' ':' gateway_list;
```

Note: We reuse the `CONTEXTS` token here so both service and exposure use the same `contexts:` keyword. The grammar rule context (parent rule) disambiguates them at parse time.

The literal `'of'` should be **kept** in the `identifier` rule — `of` remains a valid connector word and can appear as an identifier in phrases. Only removing it as a property keyword (the `exposure_property` rule above) is needed. Verify it stays:
```bash
grep -n "'of'" /Users/tiago.carcao/projects/poc/craft-project/craft/tools/antlr-grammar/Craft.g4
```
Expected: `'of'` appears only in `identifier` and `connector_word` rules — not in `exposure_property`.

- [ ] **Step 5: Verify the grammar file looks correct**

```bash
grep -n "subdomain\|DOMAINS\|domain_list\|domain_ref" /Users/tiago.carcao/projects/poc/craft-project/craft/tools/antlr-grammar/Craft.g4
```
Expected: no matches (all renamed).

```bash
grep -n "'domains'" /Users/tiago.carcao/projects/poc/craft-project/craft/tools/antlr-grammar/Craft.g4
```
Expected: no matches (replaced with `'contexts'` in identifier rule).

---

### Task 3: Regenerate the ANTLR parser

**Files:**
- Auto-generated: `pkg/parser/` (entire directory)

- [ ] **Step 1: Regenerate**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && make generate-grammar
```
Expected: completes without error, `pkg/parser/` files are updated.

- [ ] **Step 2: Verify generated types exist**

```bash
grep -rn "Bounded_context\|CONTEXTS\|Context_list\|Context_ref" /Users/tiago.carcao/projects/poc/craft-project/craft/pkg/parser/ | head -20
```
Expected: matches for `Bounded_contextContext`, `CraftLexerCONTEXTS`, `Context_listContext`, `Context_refContext`.

- [ ] **Step 3: Confirm old types are gone**

```bash
grep -rn "SubdomainContext\|DOMAINS\|Domain_listContext\|Domain_refContext" /Users/tiago.carcao/projects/poc/craft-project/craft/pkg/parser/
```
Expected: no matches.

---

### Task 4: Update Go parser types

**Files:**
- Modify: `internal/parser/types.go`

- [ ] **Step 1: Read the current types.go**

Read the file: `internal/parser/types.go`

- [ ] **Step 2: Rename SubDomains → BoundedContexts on Domain struct**

```go
// Before
// Domain represents a domain definition with its subdomains
type Domain struct {
    Name       string   `json:"name"`
    SubDomains []string `json:"subDomains"`
}

// After
// Domain represents a domain definition with its bounded contexts
type Domain struct {
    Name            string   `json:"name"`
    BoundedContexts []string `json:"boundedContexts"`
}
```

- [ ] **Step 3: Rename Domains → Contexts on Service struct**

Find the `Service` struct. Change:
```go
// Before
Domains    []string           `json:"domains,omitempty"`

// After
Contexts   []string           `json:"contexts,omitempty"`
```

- [ ] **Step 4: Rename Of → Contexts on Exposure struct**

Find the `Exposure` struct. Change:
```go
// Before
Of      []string `json:"of,omitempty"`      // Domains

// After
Contexts []string `json:"contexts,omitempty"` // Bounded contexts
```

---

### Task 5: Update Go visitor code

**Files:**
- Modify: `internal/parser/domains_visitor.go`
- Modify: `internal/parser/services_visitor.go`
- Modify: `internal/parser/exposure_visitor.go`

- [ ] **Step 1: Update domains_visitor.go**

Read the file first. Replace all occurrences:
- `SubDomains` → `BoundedContexts`
- `subdomainList` (variable name) → `boundedContextList`
- `extractSubdomainList` → `extractBoundedContextList`
- `mergeSubdomains` → `mergeBoundedContexts`
- `subdomain` (variable names) → `boundedContext`
- `*parser.Subdomain_listContext` → `*parser.Bounded_context_listContext`
- `*parser.SubdomainContext` → `*parser.Bounded_contextContext`
- `VisitSubdomain_list` → `VisitBounded_context_list`
- `VisitSubdomain` → `VisitBounded_context`

- [ ] **Step 2: Update services_visitor.go**

Read the file. Replace:
- `parser.CraftLexerDOMAINS` → `parser.CraftLexerCONTEXTS`
- `*parser.Domain_listContext` → `*parser.Context_listContext`
- The field assignment from `Domains` to `Contexts` on the service struct

- [ ] **Step 3: Update exposure_visitor.go**

Read the file. Replace:
- `*parser.Domain_listContext` → `*parser.Context_listContext`
- `*parser.Domain_refContext` → `*parser.Context_refContext`
- `extractDomainListFromExposure` → `extractContextListFromExposure`
- `b.currentExposure.Of` → `b.currentExposure.Contexts`
- Update the `Exposure{}` initializer: `Of: make([]string, 0)` → `Contexts: make([]string, 0)`

- [ ] **Step 4: Run the compiler to catch any remaining type errors**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go build ./...
```
Expected: no errors. Fix any remaining references to old type names.

- [ ] **Step 5: Run tests**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/parser/...
```
Expected: all pass. If assertion failures, update field names in test expectations (`SubDomains` → `BoundedContexts`, `Domains` → `Contexts`, `Of` → `Contexts`).

- [ ] **Step 6: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add tools/antlr-grammar/Craft.g4 pkg/parser/ internal/parser/
git commit -m "feat(dsl): rename subdomain→bounded_context, domains:→contexts:, of:→contexts:

Breaking change: update ANTLR grammar and regenerate parser.
Service property 'domains:' is now 'contexts:'.
Exposure property 'of:' is now 'contexts:'."
```

---

## Chunk 2: Visualizer, Server, Examples, Docs

### Task 6: Update the visualizer

**Files:**
- Modify: `internal/visualizer/c4_generator.go`
- Modify: `internal/visualizer/c4_main.go`
- Modify: `internal/visualizer/domain.go`

- [ ] **Step 0: Confirm build fails due to type mismatches before touching visualizer**

After completing Task 5, the parser types changed (`SubDomains` → `BoundedContexts`, `Domains` → `Contexts`, `Of` → `Contexts`) but the visualizer still references the old field names. Confirm the scope of breakage:
```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go build ./internal/visualizer/... 2>&1
```
Expected: compiler errors referencing `SubDomains`, `Domains`, `Of`. These are exactly what we'll fix. If the build passes unexpectedly, check that parser type changes from Task 4/5 were applied correctly.

- [ ] **Step 1: Update c4_generator.go — rename SubDomain → Context throughout**

Read the file. Replace:
- `focusedSubDomains` (field name) → `focusedContexts`
- `focusedSubDomainNames` (param name) → `focusedContextNames`
- `NewC4DiagramGeneratorWithFocusAndSubDomains` → `NewC4DiagramGeneratorWithFocusAndContexts`
- `GenerateC4ContainerDiagramWithFocusAndSubDomains` → `GenerateC4ContainerDiagramWithFocusAndContexts`
- `subDomainName` (variable) → `contextName`
- `subDomain` (variable in loops) → `context`
- `subDomains` (variable names) → `contexts`
- `serviceToSubDomains` → `serviceToContexts`
- `ungroupedSubDomains` → `ungroupedContexts`
- `service.Domains` → `service.Contexts` (accessing Service struct field)

In any diagram label text that shows "subdomain", update to render as `[Context]`. Search for stereotype strings:
```bash
grep -n '"\[' /Users/tiago.carcao/projects/poc/craft-project/craft/internal/visualizer/c4_generator.go | head -20
```

- [ ] **Step 2: Update c4_main.go — rename public function**

Read the file. Replace:
- `GenerateC4WithFocusAndSubDomains` → `GenerateC4WithFocusAndContexts`
- `GenerateC4WithFocusSubDomainsAndFormat` → `GenerateC4WithFocusContextsAndFormat`
- `focusedSubDomainNames` (param) → `focusedContextNames`
- `GenerateC4ContainerDiagramWithFocusAndSubDomains` → `GenerateC4ContainerDiagramWithFocusAndContexts`

- [ ] **Step 3: Update domain.go — rename subdomain references in architecture generator**

Read the file. Replace:
- `subDomain` (variable names) → `context`
- `subDomains` → `contexts`
- `serviceToSubDomains` → `serviceToContexts`
- `ungroupedSubDomains` → `ungroupedContexts`
- `domain.SubDomains` → `domain.BoundedContexts`

Also ensure that when bounded contexts are rendered in domain diagrams, they are labeled `[Context]`. Search for any hardcoded "subdomain" stereotype strings in the PlantUML generation:
```bash
grep -n "subdomain\|SubDomain\|sub_domain" /Users/tiago.carcao/projects/poc/craft-project/craft/internal/visualizer/domain.go
```

- [ ] **Step 4: Update cmd/server/main.go — rename API struct fields**

Read the file. Replace:
- `FocusedSubDomainNames` → `FocusedContextNames` (in `FocusInfo` struct, line ~175)
- `HasFocusedSubDomains` → `HasFocusedContexts` (in `FocusInfo` struct)
- `GenerateC4WithFocusAndSubDomains` → `GenerateC4WithFocusAndContexts`
- `GenerateC4WithFocusSubDomainsAndFormat` → `GenerateC4WithFocusContextsAndFormat`
- `req.FocusInfo.HasFocusedSubDomains` → `req.FocusInfo.HasFocusedContexts`
- `req.FocusInfo.FocusedSubDomainNames` → `req.FocusInfo.FocusedContextNames`
- JSON tags: `"focusedSubDomainNames"` → `"focusedContextNames"` and `"hasFocusedSubDomains"` → `"hasFocusedContexts"`

- [ ] **Step 5: Build and test**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go build ./... && go test ./...
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/visualizer/ cmd/server/main.go
git commit -m "feat(visualizer): rename SubDomain→Context in visualizer and server API"
```

---

### Task 7: Update example .craft files

**Files:**
- Modify: `examples/banking-system.craft`
- Modify: `examples/e-comerce.craft`
- Modify: `examples/user-management.craft`
- Modify: `examples/screenshot-examples.craft`
- Modify all other `.craft` files in `examples/`

- [ ] **Step 1: Find all files with old keywords**

```bash
grep -rln "domains:\|  of:" /Users/tiago.carcao/projects/poc/craft-project/craft/examples/
```

- [ ] **Step 2: Replace `domains:` → `contexts:` in each file**

For each file found, replace `domains:` with `contexts:`. Example for banking-system.craft:
```
// Before
AccountService {
    domains: AccountManagement, BalanceTracking
}

// After
AccountService {
    contexts: AccountManagement, BalanceTracking
}
```

- [ ] **Step 3: Replace `of:` → `contexts:` in exposure blocks**

```bash
grep -rln "  of:" /Users/tiago.carcao/projects/poc/craft-project/craft/examples/
```
Replace each `of:` → `contexts:` within exposure blocks.

- [ ] **Step 4: Verify all example files parse correctly**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/parser/... -run TestParse
```
If there's no integration parse test for examples, verify manually by checking the server starts and processes files correctly.

- [ ] **Step 5: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add examples/
git commit -m "chore(examples): update domains:→contexts: and of:→contexts: in all example files"
```

---

### Task 8: Update CLAUDE.md and VitePress docs

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/page/guide/introduction.md`
- Modify: `docs/page/guide/quickstart.md`
- Modify: `docs/page/guide/installation.md`
- Modify: `docs/page/extension/features.md`
- Modify: `docs/page/extension/installation.md`

- [ ] **Step 1: Update CLAUDE.md**

Read `CLAUDE.md`. Update:
- The DSL Language Features section: change "Domains" description from "Core business domains" to "Bounded contexts — solution-space units grouped within a domain"
- Any code examples showing `domains:` → `contexts:` or `of:` → `contexts:`

- [ ] **Step 2: Update docs/page/guide/introduction.md**

Read the file. Replace:
- "subdomains" → "bounded contexts" throughout
- Any code examples using `domains:` or `of:` → update to `contexts:`

- [ ] **Step 3: Update docs/page/guide/quickstart.md**

Read the file. Find code blocks with `domains:` or `of:` and update to `contexts:`.

- [ ] **Step 4: Update docs/page/extension/features.md**

Read the file. Update any DSL syntax examples and terminology.

- [ ] **Step 5: Update docs/page/guide/installation.md and docs/page/extension/installation.md**

Read both files. Update any code examples containing `domains:` or `of:`.

- [ ] **Step 6: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add CLAUDE.md docs/page/
git commit -m "docs: update terminology subdomain→bounded context in all documentation"
```

---

## Chunk 3: tree-sitter-craft Repo

### Task 9: Update tree-sitter grammar

**Files:** (in `/Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft/`)
- Modify: `grammar.js`
- Modify: `queries/highlights.scm`
- Modify: `package.json`

- [ ] **Step 1: Update grammar.js — rename subdomain rule**

Read `grammar.js`. Apply these changes:

```diff
-    subdomain: $ => $.identifier,
+    bounded_context: $ => $.identifier,
```

Update `domain_block` to reference `bounded_context`:
```diff
     domain_block: $ => seq(
       'domain',
       $.identifier,
       '{',
       optional($._newline),
-      repeat(seq($.subdomain, optional($._newline))),
+      repeat(seq($.bounded_context, optional($._newline))),
       '}',
       repeat($._newline),
     ),
```

Update `domain_definition` similarly:
```diff
     domain_definition: $ => seq(
       $.identifier,
       '{',
       optional($._newline),
-      repeat(seq($.subdomain, optional($._newline))),
+      repeat(seq($.bounded_context, optional($._newline))),
       '}',
     ),
```

- [ ] **Step 2: Update grammar.js — rename domains_property to service_contexts_property**

```diff
-    domains_property: $ => seq(
-      'domains',
+    service_contexts_property: $ => seq(
+      'contexts',
       ':',
       $.identifier_list,
     ),
```

Update the `service_property` rule to reference the new name:
```diff
     service_property: $ => choice(
-      $.domains_property,
+      $.service_contexts_property,
       $.language_property,
       $.data_stores_property,
       $.deployment_property,
     ),
```

- [ ] **Step 3: Update grammar.js — rename of_property to exposure_contexts_property**

```diff
-    of_property: $ => seq(
-      'of',
+    exposure_contexts_property: $ => seq(
+      'contexts',
       ':',
       $.identifier_list,
     ),
```

Update `exposure_property` rule:
```diff
     exposure_property: $ => choice(
       $.to_property,
       $.through_property,
-      $.of_property,
+      $.exposure_contexts_property,
     ),
```

Also remove `'of'` from the `connector_word` rule if present — check:
```bash
grep -n "'of'" /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft/grammar.js
```
The `connector_word` rule currently includes `'of'`. This is fine to keep since `'of'` can still appear in phrases. The parser will pick `exposure_contexts_property` over `connector_word` in the right context due to tree-sitter's precedence rules. Verify this doesn't cause conflicts after rebuild.

- [ ] **Step 4: Update queries/highlights.scm**

Read `queries/highlights.scm`. Apply these changes:

```diff
 ; === PROPERTY KEYWORDS ===
-"domains" @craft.domains-property
+"contexts" @craft.contexts-property
 "language" @craft.language-property
 "data-stores" @craft.data-stores-property
 "deployment" @craft.deployment-property
 "to" @craft.to-property
 "through" @craft.through-property
-"of" @craft.to-property
```

Note: `"of"` is removed from property highlights because it's no longer a property keyword — it's only a connector word now. `"contexts"` replaces `"domains"` for property highlighting.

```diff
-; Subdomain names
-(subdomain (identifier) @craft.subdomain-name)
+; Bounded context names
+(bounded_context (identifier) @craft.context-name)
```

```diff
-; Domain list values (in domains property)
-(domains_property (identifier_list (identifier) @craft.domain-list))
+; Context list values (in service contexts property)
+(service_contexts_property (identifier_list (identifier) @craft.domain-list))
+
+; Context list values (in exposure contexts property)
+(exposure_contexts_property (identifier_list (identifier) @craft.domain-list))
```

- [ ] **Step 5: Rebuild the grammar**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft && npx tree-sitter generate
```
Expected: `src/parser.c` and related files regenerated without errors.

- [ ] **Step 6: Update test corpus files before running tests**

The test corpus files use old keywords and must be updated before `tree-sitter test` will pass:
```bash
# Replace domains: → contexts: in all corpus files
find /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft/test/corpus/ -name "*.txt" \
  -exec sed -i '' 's/domains:/contexts:/g' {} \;

# Replace 'of:' → 'contexts:' in exposure blocks in corpus files
find /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft/test/corpus/ -name "*.txt" \
  -exec sed -i '' 's/  of:/  contexts:/g' {} \;

# Also update expected node type names: subdomain → bounded_context, domains_property → service_contexts_property
find /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft/test/corpus/ -name "*.txt" \
  -exec sed -i '' 's/subdomain/bounded_context/g; s/domains_property/service_contexts_property/g; s/of_property/exposure_contexts_property/g' {} \;
```

- [ ] **Step 7: Run tree-sitter tests**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft && npx tree-sitter test
```
Expected: all pass. If failures remain, read the failing test output and fix the corpus `.txt` file to match the new expected node tree.

- [ ] **Step 7: Build wasm**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft && npx tree-sitter build-wasm
```
Expected: `tree-sitter-craft.wasm` updated.

- [ ] **Step 8: Bump version in package.json**

Read `package.json`. Change `"version": "0.1.5"` → `"version": "0.2.0"`.

- [ ] **Step 9: Commit and publish**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/tree-sitter-craft
git add grammar.js queries/highlights.scm src/ tree-sitter-craft.wasm package.json
git commit -m "feat!: rename subdomain→bounded_context, domains→contexts, of→contexts

Breaking change: node type 'subdomain' renamed to 'bounded_context'.
DSL property 'domains:' renamed to 'contexts:' in service blocks.
DSL property 'of:' renamed to 'contexts:' in exposure blocks.
Bumps to 0.2.0."
git push
npm publish
```

---

## Chunk 4: craft-vscode-extension Repo

### Task 10: Update tree-sitter dependency and highlight provider

**Files:** (in `/Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension/`)
- Modify: `package.json`
- Modify: `client/src/TreeSitterHighlightProvider.ts`
- Run: grammar update script

- [ ] **Step 1: Update tree-sitter-craft dependency**

Read `package.json`. Find `tree-sitter-craft` version and update to `^0.2.0`.

- [ ] **Step 2: Install updated dependency**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension && npm install
```

- [ ] **Step 3: Run the grammar update script**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension && ./update-grammar.sh
```
This copies the new wasm file into the extension. If the script doesn't exist or fails:
```bash
cat /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension/update-grammar.sh
```

- [ ] **Step 4: Update TreeSitterHighlightProvider.ts**

Read `client/src/TreeSitterHighlightProvider.ts`. Change:
```diff
-    'craft-subdomain-name',
+    'craft-context-name',
```

Also check if `'craft-domains-property'` exists and update to `'craft-contexts-property'`:
```bash
grep -n "craft-" /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension/client/src/TreeSitterHighlightProvider.ts
```
Update any capture names that changed in `highlights.scm` (any `@craft.xxx` renamed).

---

### Task 11: Update TypeScript domain types

**Files:**
- Modify: `client/src/types/domain.ts`
- Modify: `client/src/types/api.ts`

- [ ] **Step 1: Update domain.ts — rename SubDomain interface and related**

Read `client/src/types/domain.ts`. Apply:
- Interface `SubDomain` → `BoundedContext`
- `GenerateSubDomainId` function → `GenerateContextId`
- All references to `subDomains` field → `boundedContexts`
- All references to `selectedSubDomains` → `selectedContexts`
- All references to `SubDomain[]` type → `BoundedContext[]`
- Export name `GenerateSubDomainId` → `GenerateContextId`

The `Domain` interface's `subDomains: SubDomain[]` field becomes `boundedContexts: BoundedContext[]`.

- [ ] **Step 2: Update api.ts — rename SubDomain API fields**

Read `client/src/types/api.ts`. Apply:
- `focusedSubDomainNames: string[]` → `focusedContextNames: string[]`
- `hasFocusedSubDomains: boolean` → `hasFocusedContexts: boolean`
- `domainDiagramType` field keep as-is (unrelated)
- `DomainDiagramType` type keep as-is (unrelated)
- Any `SubDomain` type references → `BoundedContext`

---

### Task 12: Update all files that reference SubDomain/subDomains

**Files:**
- Modify: `client/src/providers/domainsViewProvider.ts`
- Modify: `client/src/providers/servicesViewProvider.ts`
- Modify: `client/src/providers/servicesViewProviderReact.ts`
- Modify: `client/src/services/domainsViewService.ts`
- Modify: `client/src/services/servicesViewService.ts`
- Modify: `client/src/services/dslExtractService.ts`
- Modify: `client/src/commands/index.ts`
- Modify: `client/src/commands/previewCommon.ts`
- Modify: `client/src/extension.ts`
- Modify: `client/src/webview/components/ServicesView.tsx`
- Modify: `client/src/webview/components/DomainsView.tsx`

- [ ] **Step 1: Find all TypeScript files with SubDomain references**

```bash
grep -rln "SubDomain\|subDomains\|focusedSubDomain\|GenerateSubDomainId" /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension/client/src/
```

- [ ] **Step 2: For each file — update SubDomain → BoundedContext**

For each file found, apply these replacements:
- `SubDomain` (type) → `BoundedContext`
- `subDomains` (field/variable) → `boundedContexts`
- `GenerateSubDomainId` → `GenerateContextId`
- `focusedSubDomainNames` → `focusedContextNames`
- `hasFocusedSubDomains` → `hasFocusedContexts`
- `selectedSubDomains` → `selectedContexts`
- Import: `import { ..., SubDomain, ... }` → `import { ..., BoundedContext, ... }`

Work through each file methodically. Read it first, then apply edits.

- [ ] **Step 3: Compile to catch type errors**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension && npx tsc --noEmit
```
Expected: no errors. Fix any remaining `SubDomain` references.

- [ ] **Step 4: Also update JSON API field names sent to server**

The extension sends `focusedSubDomainNames` in the request body to the craft server's C4 endpoint. After this change, the server expects `focusedContextNames`. Search for where the request body is constructed:

```bash
grep -rn "focusedSubDomainNames\|FocusedSubDomain" /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension/client/src/
```
Ensure the JSON field name in the request matches the server's new `focusedContextNames` field.

- [ ] **Step 5: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft-vscode-extension
git add .
git commit -m "feat!: update to tree-sitter-craft 0.2.0, rename SubDomain→BoundedContext

Breaking change aligned with DSL revision:
- 'domains:' property renamed to 'contexts:' in .craft files
- 'of:' property renamed to 'contexts:' in exposure blocks
- Internal type SubDomain renamed to BoundedContext throughout
- API field focusedSubDomainNames renamed to focusedContextNames"
git push
```

---

### Task 13: Final integration smoke test

- [ ] **Step 1: Start the craft server**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && make docker-build && make docker-run
```

- [ ] **Step 2: Send a test request with the new `contexts:` keyword**

```bash
curl -s -X POST http://localhost:8080/preview/domain \
  -H "Content-Type: application/json" \
  -d '{
    "dsl": "domain User {\n  Auth\n  Profile\n}\nservices {\n  UserService {\n    contexts: Auth, Profile\n  }\n}\nuse_case \"Test\" {\n  when User does thing\n    Auth validates credentials\n}",
    "domainMode": "detailed"
  }' | head -c 100
```
Expected: a base64 PNG response (starts with `eyJ` or similar base64).

- [ ] **Step 3: Confirm old keyword is rejected**

```bash
curl -s -X POST http://localhost:8080/preview/domain \
  -H "Content-Type: application/json" \
  -d '{
    "dsl": "services {\n  TestService {\n    domains: Auth, Profile\n  }\n}",
    "domainMode": "detailed"
  }'
```
Expected: error response (parse failure — `domains:` is no longer valid).

- [ ] **Step 4: Push craft repo**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && git push
```

---

*End of Plan 1: DSL Bounded Contexts*
