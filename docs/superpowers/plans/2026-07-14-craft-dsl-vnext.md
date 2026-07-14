# Craft DSL vNext Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Craft DSL to accept flexible unquoted narrative (special characters) and to speak the locked slug/reference grammar with typed edges and service anchors, validated locally.

**Architecture:** Two implementations move together. The Go `internal/lexer` (hand-written scanner) + `internal/syntax` (recursive-descent → lossless green tree → `ast`) is the source of truth; `tree-sitter-craft` mirrors it for editors. Narrative tails become raw rest-of-line prose (parser-driven, display-only); `notifies`/`listens`/`asks` objects become typed refs (event ids / node slugs); a new `context_map` block holds authored typed edges; `craft validate` checks local well-formedness only (the hub does cross-validation).

**Tech Stack:** Go 1.23; hand-written lexer/parser; tree-sitter (grammar.js + C external scanner); `go test ./...`.

**Spec:** `docs/superpowers/specs/2026-07-14-craft-dsl-vnext-design.md`

## Global Constraints

- Go 1.23; run `go test ./...` from `craft/`.
- **Backward compatible:** every existing `examples/*.craft`, `vas.craft`, and `testdata/corpus/*` must still parse. Quoted `notifies "X"` / `listens "X"` keep working (deprecated lint only).
- **Lossless round-trip:** the green tree must reproduce source byte-for-byte (`roundtrip_test.go`, `lossless_check_test.go` stay green).
- **Differential harness** (`internal/parser_diff`) must stay green: Go parse and tree-sitter parse agree on the corpus.
- `craft` validates local well-formedness only; unresolved refs are **warnings**, never errors — the hub (ticket 015) is authoritative.
- No file paths in service anchors (identity-not-location, ticket 018/007).
- TDD: failing test first; commit after each green task.

---

## File Structure

**Go (source of truth):**
- `internal/lexer/lexer.go` — comment rule (whitespace-preceded `//`); no new token kinds for prose.
- `internal/syntax/parser.go` — `collectPhrase` → raw prose sweep; `parseNotifiesAction`/`parseAsksAction`/`parseReturnsAction`/`parseTrigger` ref scan; new `parseContextMapBlock`; service `opslevel:`/`repo:` properties.
- `internal/syntax/kind.go` — new `SyntaxKind`s: `SyntaxKindProse`, `SyntaxKindRef`, `SyntaxKindContextMapDecl`, `SyntaxKindEdgeStmt`, `SyntaxKindKwContextMap`, edge keyword kinds.
- `internal/syntax/ast.go` — `ActionDecl.PhraseText` raw slice; ref accessors; `ContextMapDecl`/`EdgeDecl` types; service anchor accessors.
- `internal/sema/*.go` — slug-shape validation, edge endpoint-kind checks, deprecation + unresolved-ref lints.
- `pkg/craft/*.go` — surface new fields (`Action.Ref`, `ContextMap`, `Service.OpsLevel`/`Repo`) on the public `CraftDoc`.

**tree-sitter (slice F):**
- `tree-sitter-craft/grammar.js` — `event_ref`, `node_slug`, `context_map`, `edge_stmt`, service props; `notifies`/`listens`/`asks` object rules.
- `tree-sitter-craft/src/scanner.c` — external scanner: `prose` token + `ref` token.

**Fixtures:** `testdata/corpus/*.craft`+`.craftjson`, `testdata/broken/*`+`.diagnostics.json`, `examples/*.craft`.

---

## Task 1: Flexible prose — sweep special characters into the narrative tail (Slice A core)

**Files:**
- Modify: `internal/syntax/parser.go:1018-1057` (`collectPhrase`)
- Modify: `internal/syntax/ast.go:1093-1127` (`PhraseText`)
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: existing `p.peek()`, `p.consumeAs(SyntaxKind)`, `p.atEOF()`, token `.Line`/`.Type`/`.Value`, `lexer.TokenError`, comment token kinds.
- Produces: `collectPhrase` consumes every same-line token (idents, strings, numbers, **and `TokenError` punctuation**) until a line change / `}` / EOF / comment token; `ActionDecl.PhraseText()` returns the exact raw source substring of those tokens.

- [ ] **Step 1: Write the failing test**

```go
// internal/syntax/parser_test.go
func TestProse_SpecialCharsUnquoted(t *testing.T) {
	src := `use_case "x" {
  when User taps Button
    Auth asks Billing for 1! & 2! and/maybe *
}`
	doc := ParseString(src) // existing entry point; adjust to the real one if named differently
	act := firstSyncAction(t, doc)
	if got := act.PhraseText(); got != "for 1! & 2! and/maybe *" {
		t.Fatalf("prose = %q, want %q", got, "for 1! & 2! and/maybe *")
	}
	if diags := doc.Diagnostics(); len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}
```

(If `ParseString`/`firstSyncAction`/`Diagnostics` aren't the exact helpers, mirror the ones already used in `parser_test.go` — grep for `func Test` there and copy the setup.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run TestProse_SpecialCharsUnquoted -v`
Expected: FAIL — phrase truncates at `1` (current `collectPhrase` returns at the `!` TokenError), and/or a stray-token diagnostic appears.

- [ ] **Step 3: Rewrite `collectPhrase` to sweep the raw tail**

Replace the body of `collectPhrase` (parser.go:1018) so the `default:` branch consumes any same-line token (including `TokenError`) instead of returning, and stops only at line change / `}` / EOF / a comment token:

```go
func (p *Parser) collectPhrase(actionLine int) {
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return
	}
	if p.peek().Line != actionLine {
		return
	}
	for {
		tok := p.peek()
		if tok.Type == lexer.TokenRBrace || tok.Type == lexer.TokenEOF {
			return
		}
		if tok.Line != actionLine {
			return
		}
		switch tok.Type {
		case lexer.TokenLineComment, lexer.TokenDocComment, lexer.TokenBlockComment:
			return // trailing comment ends prose; trivia attaches normally
		case lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case lexer.TokenNumber:
			p.consumeAs(SyntaxKindNumber)
		default:
			// idents, keywords-as-idents, and TokenError punctuation (! & * / # …)
			// are all swept into prose as raw tokens.
			p.consumeAs(SyntaxKindIdent)
		}
	}
}
```

- [ ] **Step 4: Make `PhraseText` return the exact raw span**

In `ast.go:1093`, replace the space-join with a raw substring over the prose tokens so `1! & 2!` keeps its spacing:

```go
func (a ActionDecl) PhraseText() string {
	tokens := a.node.Tokens()
	start := phraseStartIndex(a, tokens) // extract the existing start-computation switch into a helper
	if start >= len(tokens) {
		return ""
	}
	lo := tokens[start].Span().Start
	hi := tokens[len(tokens)-1].Span().End
	return strings.TrimSpace(a.node.SourceSlice(lo, hi)) // use the green tree's source accessor
}
```

If the green node exposes offsets differently, use whatever `SyntaxToken`/`SyntaxNode` API yields absolute byte offsets (the lossless tree tracks them — see `tree.go`/`green` package). Keep the `phraseStartIndex` switch identical to the current `start=` logic (sync/return/internal cases).

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/syntax/ -run TestProse -v` then `go test ./internal/syntax/ ./internal/sema/ -v`
Expected: PASS, and `roundtrip_test.go` / `lossless_check_test.go` still green.

- [ ] **Step 6: Regression — full corpus still parses**

Run: `go test ./...`
Expected: PASS. If any `testdata/corpus` `.craftjson` now differs because a phrase reads exactly instead of space-joined, regenerate that golden and eyeball the diff.

- [ ] **Step 7: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/ast.go internal/syntax/parser_test.go
git commit -m "feat(dsl): sweep special chars into unquoted narrative prose"
```

---

## Task 2: Whitespace-preceded comment rule (Slice A, `//` in URLs/ratios)

**Files:**
- Modify: `internal/lexer/lexer.go:148-153` (comment dispatch in `Next`)
- Test: `internal/lexer/lexer_test.go`, `internal/syntax/parser_test.go`

**Interfaces:**
- Produces: `//`, `///`, `/*` begin a comment **only** when the preceding char is whitespace or the token is at line start; otherwise `/` lexes as a `TokenError` (and is swept into prose by Task 1).

- [ ] **Step 1: Write the failing test**

```go
// internal/lexer/lexer_test.go
func TestComment_OnlyWhenWhitespacePreceded(t *testing.T) {
	toks := New("Auth calls http://api now // note").All()
	// no comment token until the whitespace-preceded "//"
	joined := renderKinds(toks) // helper: comma-joined token Values
	if strings.Count(joined, "TokenLineComment") != 1 {
		t.Fatalf("want exactly one line comment, got: %s", joined)
	}
	// the "//" inside http://api must NOT be a comment
	if strings.Contains(commentText(toks), "api now") {
		t.Fatalf("url slashes were mis-lexed as a comment")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lexer/ -run TestComment_OnlyWhenWhitespacePreceded -v`
Expected: FAIL — `http://api` currently starts a comment at the first `//`.

- [ ] **Step 3: Add the whitespace guard**

In `lexer.go` `Next()`, gate the three comment cases on a helper. Track the previous rune (add `prevRune rune` to the `Lexer` struct, set in `advance()`), and:

```go
precededByWS := l.pos == 0 || l.prevRune == ' ' || l.prevRune == '\t' || l.prevRune == '\n' || l.prevRune == '\r'
switch {
case precededByWS && ch == '/' && l.peek(1) == '/' && l.peek(2) == '/':
	return l.scanDocComment()
case precededByWS && ch == '/' && l.peek(1) == '/':
	return l.scanLineComment()
case precededByWS && ch == '/' && l.peek(1) == '*':
	return l.scanBlockComment()
// ... existing cases unchanged; a non-ws-preceded '/' falls through to default → TokenError
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/lexer/ ./internal/syntax/ -v`
Expected: PASS. Add a parser-level test that `Auth checks x  // TODO` yields prose `Auth checks x` + a trailing comment trivia.

- [ ] **Step 5: Regression**

Run: `go test ./...`
Expected: PASS. Existing full-line comments (`// ...` at line start, preceded by newline) still lex as comments.

- [ ] **Step 6: Commit**

```bash
git add internal/lexer/lexer.go internal/lexer/lexer_test.go internal/syntax/parser_test.go
git commit -m "feat(lexer): only start comments at whitespace-preceded // (URLs survive in prose)"
```

---

## Task 3: Ref scanning helper — event refs & node slugs (Slice B foundation)

**Files:**
- Modify: `internal/syntax/kind.go` (add `SyntaxKindRef`)
- Modify: `internal/syntax/parser.go` (add `parseRef` helper)
- Modify: `internal/syntax/ast.go` (add `RefText`/`RefKind` accessors)
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Produces: `func (p *Parser) parseRef() (kind string)` — consumes a reference at the current position and emits a single `SyntaxKindRef` node. Grammar: `(kindWord ':')? (segment '/')* name`, segment/name = runs of `[A-Za-z0-9_.-]`. Returns the leading kind word if a `:` was present (`domain`/`bc`/`term`/`service`), else `""` (event ref / bare). Does **not** validate the kind — that's sema (Task 8).

- [ ] **Step 1: Write the failing test**

```go
func TestParseRef_Shapes(t *testing.T) {
	cases := []struct{ src, wantText, wantKind string }{
		{"vas.VasApplied", "vas.VasApplied", ""},
		{"com.olx.re.subscriptions.SubscriptionCreated", "com.olx.re.subscriptions.SubscriptionCreated", ""},
		{"bc:re/subscriptions", "bc:re/subscriptions", "bc"},
		{"term:billing/dunning", "term:billing/dunning", "term"},
		{"service:subscriptions-api", "service:subscriptions-api", "service"},
	}
	for _, c := range cases {
		txt, kind := parseRefText(t, c.src) // test helper wrapping a Parser over c.src
		if txt != c.wantText || kind != c.wantKind {
			t.Fatalf("%q -> (%q,%q), want (%q,%q)", c.src, txt, kind, c.wantText, c.wantKind)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestParseRef_Shapes -v`
Expected: FAIL — `parseRef`/`parseRefText` undefined.

- [ ] **Step 3: Implement `parseRef`**

Since `:` and `/` lex as `TokenColon` / `TokenError`, `parseRef` consumes the contiguous run of ident/number/`:`/`/`/`.`/`-` tokens on one line into a `SyntaxKindRef` node, capturing the kind word when the second token is `TokenColon`:

```go
func (p *Parser) parseRef() string {
	p.builder.StartNode(SyntaxKindRef)
	line := p.peek().Line
	kind := ""
	first := p.peek()
	// leading kind word + ':'
	if (first.Type == lexer.TokenIdent) && p.peekAt(1).Type == lexer.TokenColon && p.peekAt(1).Line == line {
		if isSlugKind(first.Value) { kind = first.Value }
		p.consumeAs(SyntaxKindIdent) // kind
		p.consumeAs(SyntaxKindColon) // ':'
	}
	for !p.atEOF() && p.peek().Line == line {
		t := p.peek()
		if t.Type == lexer.TokenIdent || t.Type == lexer.TokenNumber ||
			(t.Type == lexer.TokenError && (t.Value == "/" )) {
			p.consumeAs(SyntaxKindIdent)
			continue
		}
		break
	}
	p.builder.FinishNode()
	return kind
}

func isSlugKind(s string) bool {
	switch s { case "domain", "bc", "term", "service": return true }
	return false
}
```

Add `SyntaxKindRef` (and reuse existing `SyntaxKindColon`) in `kind.go`. Add `RefText`/`RefKind` on the relevant AST decls reading the `SyntaxKindRef` child's raw source span.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/syntax/ -run TestParseRef_Shapes -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/syntax/kind.go internal/syntax/parser.go internal/syntax/ast.go internal/syntax/parser_test.go
git commit -m "feat(dsl): add reference scanner for event refs and node slugs"
```

---

## Task 4: Typed refs on notifies / listens / asks-target (Slice B wiring)

**Files:**
- Modify: `internal/syntax/parser.go` — `parseNotifiesAction` (982), `parseTrigger` listens branch (795), `parseAsksAction` target (963)
- Modify: `internal/syntax/ast.go` — event/target accessors return ref text
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: `parseRef` (Task 3).
- Produces: `notifies <ref>` and `X listens <ref>` accept event refs; the `asks` **target** accepts a node slug or bare identifier. Quoted string form still parses (kept for Task 8's deprecation lint).

- [ ] **Step 1: Write the failing test**

```go
func TestTypedRefs_NotifiesListensAsks(t *testing.T) {
	src := `use_case "x" {
  when Subscriptions listens vas.VasApplied
    Fulfillment asks bc:re/billing to record outcome
    Fulfillment notifies vas.VasFulfilled
}`
	doc := ParseString(src)
	if d := doc.Diagnostics(); len(d) != 0 {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	// assert the async action's ref text == "vas.VasFulfilled"
	// assert the listens trigger ref == "vas.VasApplied"
	// assert the asks target ref == "bc:re/billing" and prose == "record outcome"
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestTypedRefs -v`
Expected: FAIL — `notifies vas.VasFulfilled` currently stops at `vas` (dots ok) but `bc:re/billing` breaks at `:`.

- [ ] **Step 3: Wire `parseRef` in**

In `parseNotifiesAction`: when the object is not a `TokenString`, call `p.parseRef()` instead of a single `consumeAs(SyntaxKindIdent)`. In the `listens` branch of `parseTrigger`: same. In `parseAsksAction`: replace the single-ident target consume with `p.parseRef()` when the next token starts a ref (ident, optionally followed by `:`), then keep the existing `to`/`for` connector handling before `collectPhrase`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/syntax/ -run TestTypedRefs -v`
Expected: PASS.

- [ ] **Step 5: Regression + surface on public API**

Add `Ref string` to `pkg/craft` Action/Trigger as appropriate; run `go test ./...`. Update any `.craftjson` goldens that now carry ref text.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(dsl): typed refs on notifies/listens/asks-target"
```

---

## Task 5: `context_map` block with typed edges (Slice C)

**Files:**
- Modify: `internal/lexer/lexer.go` — add `context_map` keyword to `keywords` map (→ `TokenKwContextMap`)
- Modify: `internal/syntax/kind.go` — `SyntaxKindContextMapDecl`, `SyntaxKindEdgeStmt`, `SyntaxKindKwContextMap`, `SyntaxKindEdgeKw`
- Modify: `internal/syntax/parser.go` — `parseContextMapBlock`; dispatch from top-level switch (near line 56-74)
- Modify: `internal/syntax/ast.go` — `ContextMapDecl`, `EdgeDecl{Left, Verb, Right string}`
- Modify: `pkg/craft` — expose `ContextMap []Edge`
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: `parseRef` (Task 3).
- Produces: top-level `context_map { edge_stmt* }`; `edge_stmt := ref EDGE_KW ref`; `EDGE_KW ∈ {realized_by, also_realizes, same_as, contrasts, distinct_from}` (matched by value as contextual keywords, like `asks`).

- [ ] **Step 1: Write the failing test**

```go
func TestContextMap_Edges(t *testing.T) {
	src := `context_map {
  bc:re/subscriptions realized_by service:subscriptions-api
  term:subscriptions/dunning contrasts term:billing/dunning
}`
	doc := ParseString(src)
	if d := doc.Diagnostics(); len(d) != 0 { t.Fatalf("diags: %v", d) }
	edges := doc.ContextMap()
	if len(edges) != 2 { t.Fatalf("want 2 edges, got %d", len(edges)) }
	if edges[0].Left != "bc:re/subscriptions" || edges[0].Verb != "realized_by" || edges[0].Right != "service:subscriptions-api" {
		t.Fatalf("edge0 = %+v", edges[0])
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestContextMap_Edges -v`
Expected: FAIL — `context_map` unknown top-level keyword.

- [ ] **Step 3: Implement lexer keyword + parser block**

Add `"context_map": TokenKwContextMap` to `keywords`. Add the top-level dispatch case calling `parseContextMapBlock`:

```go
func (p *Parser) parseContextMapBlock() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindContextMapDecl)
	var diags []craft.Diagnostic
	p.attachTrivia()
	p.consumeAs(SyntaxKindKwContextMap)
	if p.peek().Type != lexer.TokenLBrace {
		diags = append(diags, p.diagUnexpected(p.peek(), "{"))
		p.resyncToTopLevel(); p.builder.FinishNode(); return diags
	}
	p.consumeAs(SyntaxKindLBrace)
	for !p.atEOF() && p.peek().Type != lexer.TokenRBrace {
		if p.peek().Type == lexer.TokenNewline { p.consumeTrivia(); continue }
		diags = append(diags, p.parseEdgeStmt()...)
	}
	if p.peek().Type == lexer.TokenRBrace { p.consumeAs(SyntaxKindRBrace) }
	p.builder.FinishNode()
	return diags
}

func (p *Parser) parseEdgeStmt() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindEdgeStmt)
	var diags []craft.Diagnostic
	p.parseRef() // left
	verb := p.peek()
	if verb.Type == lexer.TokenIdent && isEdgeKeyword(verb.Value) {
		p.consumeAs(SyntaxKindEdgeKw)
	} else {
		diags = append(diags, p.diagUnexpected(verb, "an edge keyword (realized_by/also_realizes/same_as/contrasts/distinct_from)"))
	}
	p.parseRef() // right
	p.builder.FinishNode()
	return diags
}

func isEdgeKeyword(s string) bool {
	switch s {
	case "realized_by", "also_realizes", "same_as", "contrasts", "distinct_from": return true
	}
	return false
}
```

Adjust helper names (`consumeTrivia`/`attachTrivia`/`resyncToTopLevel`) to the real ones in `parser.go`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/syntax/ -run TestContextMap_Edges -v`
Expected: PASS.

- [ ] **Step 5: Surface + regression**

Add `Edge{Left,Verb,Right string}` and `ContextMap()` to `pkg/craft`; run `go test ./...`.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(dsl): context_map block with typed BC/term edges"
```

---

## Task 6: Service anchors `opslevel:` / `repo:` (Slice D)

**Files:**
- Modify: `internal/syntax/parser.go` — service property parsing (find `parseServiceProperty`/`service_contexts_property` analog; grep `contexts`)
- Modify: `internal/syntax/ast.go` — `ServiceDecl.OpsLevel()`, `ServiceDecl.Repo()`
- Modify: `pkg/craft` — `Service.OpsLevel`, `Service.Repo`
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Produces: inside a service block, `opslevel: <ident>` and `repo: <ident-or-string>` parse as service properties; each at most once.

- [ ] **Step 1: Write the failing test**

```go
func TestServiceAnchors(t *testing.T) {
	src := `services {
  SubscriptionsApi {
    contexts: Subscriptions
    opslevel: subscriptions-api
    repo: olxeu/realestate/subscriptions
  }
}`
	doc := ParseString(src)
	if d := doc.Diagnostics(); len(d) != 0 { t.Fatalf("diags: %v", d) }
	svc := doc.Services()[0]
	if svc.OpsLevel != "subscriptions-api" || svc.Repo != "olxeu/realestate/subscriptions" {
		t.Fatalf("anchors = %q / %q", svc.OpsLevel, svc.Repo)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/syntax/ -run TestServiceAnchors -v`
Expected: FAIL — `opslevel:`/`repo:` not recognized (and `repo:` value has `/` which needs the ref scan).

- [ ] **Step 3: Implement**

In the service-property loop, add `opslevel` and `repo` as contextual property keywords (same pattern as `contexts`/`language`). Their value is a single ref (`p.parseRef()`) so slashes in a repo slug are captured. Record onto the service node; map into `pkg/craft`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/syntax/ -run TestServiceAnchors -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(dsl): opslevel/repo service anchors"
```

---

## Task 7: Local validation — slug shape, edge endpoints, deprecation lints (Slice E)

**Files:**
- Modify: `internal/sema/*.go` (add a `validateRefs`/`validateEdges` pass; follow the existing rule-registration pattern — grep `Diagnostic` in `internal/sema`)
- Test: `internal/sema/*_test.go`, `testdata/broken/*` + `.diagnostics.json`

**Interfaces:**
- Consumes: `RefText`/`RefKind` (Task 3-4), `EdgeDecl` (Task 5), `ActionDecl` (async/string).
- Produces diagnostics:
  - `malformed-slug` (error): a `kind:` prefix present but kind ∉ {domain,bc,term,service}, or namespace shape wrong per kind (`domain:re/…`, `bc:<domain>/…`, `term:<bc>/…`, `service:<alias>` with no ns).
  - `edge-endpoint-kind` (error): `realized_by`/`also_realizes` not `bc:`→`service:`; term edges not `term:`→`term:`.
  - `deprecated-string-ref` (warning): quoted `notifies "X"` / `listens "X"`.
  - `unresolved-ref-local` (warning): ref names no node declared in the file-set (never an error — hub is authoritative).
  - `duplicate-service-anchor` (error): `opslevel:`/`repo:` repeated.

- [ ] **Step 1: Write the failing tests** (one per diagnostic)

```go
func TestValidate_MalformedSlug(t *testing.T) {
	doc := ParseString(`context_map {
  foo:re/x same_as term:billing/y
}`)
	assertHasDiag(t, doc, "malformed-slug")
}

func TestValidate_EdgeEndpointKind(t *testing.T) {
	doc := ParseString(`context_map {
  bc:re/x same_as service:y
}`)
	assertHasDiag(t, doc, "edge-endpoint-kind")
}

func TestValidate_DeprecatedStringRef(t *testing.T) {
	doc := ParseString(`use_case "x" {
  when A listens "Legacy"
    A notifies "AlsoLegacy"
}`)
	assertDiagCount(t, doc, "deprecated-string-ref", 2)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/sema/ -run TestValidate_ -v`
Expected: FAIL — no such diagnostics yet.

- [ ] **Step 3: Implement the validation pass**

Walk the AST: for every `SyntaxKindRef`, check slug shape by kind; for every `EdgeDecl`, check endpoint kinds against the verb; for every async/listens with a `SyntaxKindString` object, emit `deprecated-string-ref`; track `opslevel:`/`repo:` counts per service. Register rule codes exactly as above.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/sema/ -v`
Expected: PASS.

- [ ] **Step 5: Add broken-corpus fixtures**

Create `testdata/broken/malformed-slug.craft` + `.diagnostics.json` (and one per rule). Run the broken-corpus harness test.
Run: `go test ./... -run Corpus -v` (adjust to the real harness test name)
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(sema): local validation for slugs, edges, deprecated string refs"
```

---

## Task 8: Migrate examples + corpus to vNext, keep back-compat green

**Files:**
- Modify: `examples/*.craft`, `vas.craft` (author typed refs where illustrative; keep at least one quoted-string file to exercise the deprecation lint)
- Add: `testdata/corpus/dsl-vnext.craft` + `.craftjson`

**Interfaces:** none new — exercises Tasks 1-7 end to end.

- [ ] **Step 1: Add the canonical vNext corpus fixture**

Create `testdata/corpus/dsl-vnext.craft`:

```craft
services {
  SubscriptionsApi {
    contexts: Subscriptions
    opslevel: subscriptions-api
    repo: olxeu/realestate/subscriptions
  }
}

context_map {
  bc:re/subscriptions realized_by service:subscriptions-api
  term:subscriptions/dunning contrasts term:billing/dunning
}

use_case "Renewal with typed refs and flexible prose" {
  when Subscriptions listens vas.VasApplied
    Subscriptions asks bc:re/billing for a fresh charge attempt (1! & 2!)
    Subscriptions notifies vas.VasFulfilled  // TODO: confirm subject
}
```

- [ ] **Step 2: Generate + verify the golden**

Run the corpus golden-gen (grep the Makefile / `parser_diff` for the regenerate command), then:
Run: `go test ./... `
Expected: PASS, golden committed.

- [ ] **Step 3: Verify every legacy file still parses**

Run: `go test ./... -run Corpus -v` and a quick `go run ./cmd/craft <file>` over each `examples/*.craft`.
Expected: PASS; quoted-ref files emit exactly the expected `deprecated-string-ref` warnings.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test(dsl): vNext corpus fixture + migrate examples, keep back-compat"
```

---

## Task 9: tree-sitter sync — external scanner + grammar rules (Slice F)

**Files:**
- Create: `tree-sitter-craft/src/scanner.c` (external scanner: `prose`, `ref`)
- Modify: `tree-sitter-craft/grammar.js` (externals; `event_ref`/`node_slug`/`context_map`/`edge_stmt`; service props; notifies/listens/asks objects)
- Modify: `tree-sitter-craft/CORPUS_VERSION`
- Test: `tree-sitter-craft/test/corpus/*.txt`; `internal/parser_diff`

**Interfaces:**
- Produces: tree-sitter parses the Task 8 fixture identically (structurally) to the Go parser; `parser_diff` harness green.

- [ ] **Step 1: Write failing tree-sitter corpus tests**

Add `tree-sitter-craft/test/corpus/vnext.txt` cases for: prose with `! & * / #`; `http://` not a comment; `notifies vas.VasFulfilled`; `context_map` edges; `opslevel:`/`repo:`.

- [ ] **Step 2: Run to verify they fail**

Run: `cd tree-sitter-craft && npm test`
Expected: FAIL — new syntax not recognized.

- [ ] **Step 3: Add the external scanner**

Declare `externals: $ => [$.prose, $.ref]` in grammar.js. In `scanner.c`, implement:
- `prose`: from the current position, consume to end-of-line, stopping before a whitespace-preceded `//`/`/*`; emit if non-empty.
- `ref`: consume `[A-Za-z0-9_.:/-]+` (one run) as a ref token.
Wire `notifies`/`listens` objects to `choice($.string, $.ref)`, `asks` target to `$.ref`, and the phrase tails to `$.prose`. Add `context_map`, `edge_stmt` (with the five edge keyword literals), and `opslevel:`/`repo:` service properties mirroring §3-§5 of the spec.

- [ ] **Step 4: Regenerate + run**

Run: `cd tree-sitter-craft && npm run build && npm test`
Expected: PASS. Rebuild `craft.dylib`/`tree-sitter-craft.wasm` per `PUBLISHING.md`.

- [ ] **Step 5: Differential harness green**

Run: `cd craft && go test ./internal/parser_diff/ -v`
Expected: PASS — Go and tree-sitter agree on the corpus.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(tree-sitter): external scanner + vNext grammar rules (prose, refs, context_map, anchors)"
```

---

## Task 10: VSCode extension highlighting (Slice F finish)

**Files:**
- Modify: `craft-vscode-extension` grammar/queries (`tree-sitter-craft/queries/*.scm` if highlighting is query-driven)
- Test: manual + any existing highlight snapshot tests

**Interfaces:** none — editor UX only.

- [ ] **Step 1: Update highlight queries**

Add captures for `context_map` keyword, the five edge keywords, slug `kind:` prefixes, `ref` vs `prose`, and `opslevel:`/`repo:` properties in `queries/highlights.scm`.

- [ ] **Step 2: Verify**

Run the extension's highlight test if present; otherwise open the Task 8 fixture in the dev host and confirm refs/prose/edges highlight distinctly.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat(vscode): highlight context_map, edges, slugs, anchors"
```

---

## Self-Review Notes

- **Spec coverage:** A→Task 1-2; B→Task 3-4; C→Task 5; D→Task 6; E→Task 7; F→Task 9-10; migration/back-compat→Task 8. All six slices + testing section covered.
- **Type consistency:** `parseRef` returns kind string (Tasks 3-6); `EdgeDecl{Left,Verb,Right}` used in Tasks 5 & 7; diagnostic codes (`malformed-slug`, `edge-endpoint-kind`, `deprecated-string-ref`, `unresolved-ref-local`, `duplicate-service-anchor`) consistent between Task 7 definition and fixtures.
- **Known adaptation points** (verify against real signatures when implementing, not placeholders): `ParseString`/`Diagnostics`/`Services`/`ContextMap` public entry names; the green tree's absolute-offset/`SourceSlice` accessor; the corpus golden-regeneration command; trivia helper names in `parser.go`. Each is a "match the existing name" step, not undefined behavior.
