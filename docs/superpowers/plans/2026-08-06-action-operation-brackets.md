# Action Operation Brackets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let any Craft action line end with an optional `[POST /v1/charges]` operation annotation, and remove `kind:` prefixes from the action target slot.

**Architecture:** A new `SyntaxKindOpAnnotation` node wraps a trailing bracketed run on an action line. The parser decides where the free-text phrase ends by scanning the action line once for the last `[` whose `]` is the line's final token, so a `[` used in prose that does not close at end of line stays prose. Bracket contents are hybrid: a recognised uppercase protocol verb is tokenised as `SyntaxKindOpVerb`, everything else is swept verbatim as payload. The projection layer lowers the node to a new optional `model.Action.Operation` field, which is a pointer so absent annotations leave every existing `.craftjson` golden byte-identical.

**Tech Stack:** Go 1.23, hand-written lexer (`internal/lexer`), recursive-descent parser over a lossless green tree (`internal/syntax`), `go test ./...`.

**Spec:** `docs/decisions/action-operation-brackets.md`

## Global Constraints

- Go 1.23. Module path `github.com/tcarcao/craft/v2`.
- Token kinds must stay `< 1000`, node kinds `>= 1000` (`internal/syntax/kind.go:6-7`, enforced by `TestSyntaxKind_NodeBoundary`). Append new token kinds immediately before `syntaxKindTokenSentinel`; append new node kinds at the end of the node block.
- The green tree is lossless. Every parse path must use `p.consumeAs(...)` and never synthesise tokens, or `TestRoundTrip` (`internal/syntax/roundtrip_test.go`) and `lossless_check_test.go` will fail.
- The parser must always make forward progress. Every loop must consume at least one token per iteration.
- Protocol verb set, exact and case-sensitive: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `GRPC`, `TOPIC`, `QUERY`.
- `model.Action.Operation` must be a pointer with `json:"operation,omitempty"` so existing corpus goldens do not change.
- No em dashes in code comments or commit messages.
- Run `go test ./...` from the repo root. Full suite must be green before each commit.

## Two things the spec had wrong, now fixed in both documents

Recorded here because they change the shape of the work, and an implementer reading an older copy of the spec would size Tasks 5 and 6 incorrectly.

1. **No `use_case` scoping is being added.** There is no domain scope on `use_case` today. `resolveBCRef` already returns `ambiguous=true` for a bare name owned by two or more domains when `scopeDomain` is `""` (`internal/sema/validate.go:380-394`), so the actionable work is emitting a diagnostic instead of silently dropping the ambiguous case. That is Task 6, and it is smaller than "wire the real scope through" implied.

2. **The span-skipping helpers in `ast.go` are not deletable.** Keeping the `re/billing` qualified form means an action target is still a multi-token `Ref` (`re`, `/`, `billing`), so `Connector()`, `phraseStartIndex`, `PhraseStartIndex()`, and the `refAwareText` branches all stay load-bearing. Dropping `bc:` shortens the ref by two tokens; it does not make it a single token. No cleanup task exists for them.

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/syntax/kind.go` | Modify | Add `SyntaxKindOpVerb` (token), `SyntaxKindOpAnnotation` (node) |
| `internal/syntax/op_verbs.go` | Create | Protocol verb table and `isProtocolVerb` |
| `internal/syntax/op_verbs_test.go` | Create | Verb table tests |
| `internal/syntax/parser.go` | Modify | `opAnnotationStart`, `parseOpAnnotation`, `collectPhrase` bound, wire into all four action forms, reject `kind:` in target |
| `internal/syntax/ast.go` | Modify | `OpAnnotation`/`OpVerb`/`OpPayload`/`OpText` accessors, exclude annotation from `PhraseText` |
| `internal/syntax/projection.go` | Modify | Lower annotation into `model.Action.Operation` |
| `internal/model/model.go` | Modify | Add `Operation` struct and `Action.Operation` field |
| `internal/sema/lint.go` | Modify | Emit `craft/sema/ambiguous-bc` for use-case action refs |
| `internal/lsp/semantic.go` (or `server.go`) | Modify | Classify annotation tokens |
| `internal/lsp/completion.go` | Modify | `[` trigger, protocol verb completion |
| `internal/lsp/format.go` (or wherever `FormatDocument` lives) | Modify | Column-align annotations |
| `testdata/corpus/04_use_cases/op_annotations.craft` + `.craftjson` | Create | Corpus fixture |
| `testdata/corpus/99_mixed/dsl-vnext.craft` + `.craftjson` | Modify | Drop `bc:` prefix |
| `testdata/broken/malformed_slug.craft` + `.diagnostics.json` | Modify | Expect the new prefix diagnostic |
| `docs/page/language/use-cases.md` | Modify | Document the annotation, narrow the punctuation claim |

---

### Task 1: Syntax kinds and the protocol verb table

**Files:**
- Modify: `internal/syntax/kind.go:104-148`
- Create: `internal/syntax/op_verbs.go`
- Test: `internal/syntax/op_verbs_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `syntax.SyntaxKindOpVerb` (token kind), `syntax.SyntaxKindOpAnnotation` (node kind), `isProtocolVerb(s string) bool` (package-private), `ProtocolVerbs() []string` (exported, for LSP completion in Task 7)

- [ ] **Step 1: Write the failing test**

Create `internal/syntax/op_verbs_test.go`:

```go
package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

func TestProtocolVerbs_Contents(t *testing.T) {
	want := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
		"HEAD": true, "OPTIONS": true, "GRPC": true, "TOPIC": true, "QUERY": true,
	}
	got := syntax.ProtocolVerbs()
	if len(got) != len(want) {
		t.Fatalf("ProtocolVerbs() returned %d verbs, want %d: %v", len(got), len(want), got)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected verb %q", v)
		}
	}
}

func TestProtocolVerbs_SortedAndStable(t *testing.T) {
	a := syntax.ProtocolVerbs()
	b := syntax.ProtocolVerbs()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("ProtocolVerbs() is not stable: %v vs %v", a, b)
		}
	}
	for i := 1; i < len(a); i++ {
		if a[i-1] >= a[i] {
			t.Errorf("ProtocolVerbs() not sorted at %d: %q then %q", i, a[i-1], a[i])
		}
	}
}

func TestOpAnnotationKind_IsNode(t *testing.T) {
	if !syntax.SyntaxKindOpAnnotation.IsNode() {
		t.Error("SyntaxKindOpAnnotation should be a node kind")
	}
	if syntax.SyntaxKindOpVerb.IsToken() != true {
		t.Error("SyntaxKindOpVerb should be a token kind")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run 'TestProtocolVerbs|TestOpAnnotationKind' -v`
Expected: FAIL, compile error `undefined: syntax.ProtocolVerbs`, `undefined: syntax.SyntaxKindOpAnnotation`

- [ ] **Step 3: Add the syntax kinds**

In `internal/syntax/kind.go`, add immediately before `syntaxKindTokenSentinel` (currently line 111):

```go
	// SyntaxKindOpVerb is the recognised uppercase protocol verb at the head of
	// an operation annotation body, e.g. the POST in `[POST /v1/charges]`.
	// Matched by value from TokenIdent, like asks/notifies elsewhere. An
	// unrecognised leading word is emitted as SyntaxKindIdent and the whole
	// body is treated as an opaque payload.
	SyntaxKindOpVerb
```

And at the end of the node-kind block (after `SyntaxKindTagStmt`, currently line 147):

```go
	// SyntaxKindOpAnnotation wraps a trailing `[ ... ]` operation annotation on
	// an action line, e.g. `[POST /v1/charges]`. It is always the last child of
	// a SyntaxKindAction node.
	SyntaxKindOpAnnotation
```

- [ ] **Step 4: Create the verb table**

Create `internal/syntax/op_verbs.go`:

```go
package syntax

import "sort"

// protocolVerbs is the recognised set of operation-annotation protocol verbs.
// Matching is exact and case-sensitive: only the uppercase spelling is a verb,
// so a lowercase `post` inside a bracket is payload, not a verb.
//
// Extending this set is additive and non-breaking: an unrecognised leading word
// already parses as opaque payload, so promoting it to a verb later only adds
// structure, it never invalidates a file that already parsed.
var protocolVerbs = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"PATCH":   true,
	"DELETE":  true,
	"HEAD":    true,
	"OPTIONS": true,
	"GRPC":    true,
	"TOPIC":   true,
	"QUERY":   true,
}

// isProtocolVerb reports whether s is a recognised protocol verb.
func isProtocolVerb(s string) bool { return protocolVerbs[s] }

// ProtocolVerbs returns the recognised protocol verbs in sorted order. Exported
// for LSP completion.
func ProtocolVerbs() []string {
	out := make([]string, 0, len(protocolVerbs))
	for v := range protocolVerbs {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/syntax/ -run 'TestProtocolVerbs|TestOpAnnotationKind' -v`
Expected: PASS

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS. Adding kinds at the ends of their blocks does not renumber anything the tests assert on.

- [ ] **Step 7: Commit**

```bash
git add internal/syntax/kind.go internal/syntax/op_verbs.go internal/syntax/op_verbs_test.go
git commit -m "feat(syntax): add op-annotation syntax kinds and protocol verb table"
```

---

### Task 2: Parse the trailing operation bracket

**Files:**
- Modify: `internal/syntax/parser.go:1043-1201` (`parseAction`, `parseAsksAction`, `parseNotifiesAction`, `parseReturnsAction`, `collectPhrase`)
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: `SyntaxKindOpAnnotation`, `SyntaxKindOpVerb`, `isProtocolVerb` from Task 1
- Produces: an `SyntaxKindOpAnnotation` node as the last child of `SyntaxKindAction` when an annotation is present. `collectPhrase(actionLine int, maxTokens int)` where `maxTokens < 0` means "until end of line". `(p *Parser) opAnnotationStart(actionLine int) int` returning a lookahead index or `-1`.

- [ ] **Step 1: Write the failing test**

Append to `internal/syntax/parser_test.go`:

```go
func TestOpAnnotation_AllActionForms(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing notifies billing.ChargeSucceeded [TOPIC billing.v1.charged]
    Gateway returns to Billing the authorization result [200 AuthResult]
    Subscriptions marks the subscription active [op1/op2/op3]
}`
	tree := astParse(src)
	actions := syntax.AsFile(tree).UseCases()[0].Scenarios()[0].Actions()
	if len(actions) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(actions))
	}
	for i, a := range actions {
		if a.OpAnnotation() == nil {
			t.Errorf("action %d (%s): expected an op annotation, got none", i, a.Kind())
		}
	}
}

func TestOpAnnotation_Absent(t *testing.T) {
	src := `use_case "X" {
  when U does x
    Billing asks Ledger to record the entry
}`
	tree := astParse(src)
	a := syntax.AsFile(tree).UseCases()[0].Scenarios()[0].Actions()[0]
	if a.OpAnnotation() != nil {
		t.Error("expected no op annotation")
	}
	if got := a.PhraseText(); got != "record the entry" {
		t.Errorf("PhraseText() = %q, want %q", got, "record the entry")
	}
}

// A `[` in prose that does not close at end of line stays prose. This preserves
// today's sweep-everything phrase behaviour for any line not using the feature.
func TestOpAnnotation_UnclosedBracketStaysProse(t *testing.T) {
	src := `use_case "X" {
  when U does x
    Billing asks Ledger to record the entry [not closed
}`
	tree := astParse(src)
	a := syntax.AsFile(tree).UseCases()[0].Scenarios()[0].Actions()[0]
	if a.OpAnnotation() != nil {
		t.Error("an unclosed [ must not open an annotation")
	}
	if got := a.PhraseText(); got != "record the entry [not closed" {
		t.Errorf("PhraseText() = %q, want the bracket swept as prose", got)
	}
}

// When the line has more than one `[`, the LAST one whose `]` ends the line
// opens the annotation; earlier brackets stay prose.
func TestOpAnnotation_LastBracketWins(t *testing.T) {
	src := `use_case "X" {
  when U does x
    Billing asks Ledger to record [batch] entries [POST /v1/entries]
}`
	tree := astParse(src)
	a := syntax.AsFile(tree).UseCases()[0].Scenarios()[0].Actions()[0]
	if a.OpAnnotation() == nil {
		t.Fatal("expected an op annotation")
	}
	if got := a.PhraseText(); got != "record [batch] entries" {
		t.Errorf("PhraseText() = %q, want %q", got, "record [batch] entries")
	}
	if got := a.OpText(); got != "POST /v1/entries" {
		t.Errorf("OpText() = %q, want %q", got, "POST /v1/entries")
	}
}

func TestOpAnnotation_RoundTripsExactly(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c  [POST /v1/x?q=1]\n}"
	g, _, _ := syntax.Parse(src)
	if got := reassembleGreen(g); got != src {
		t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run TestOpAnnotation -v`
Expected: FAIL, compile error `a.OpAnnotation undefined`. Task 3 adds the accessors; write them as stubs now so this task's tests can run, then fill them in Task 3.

Add these stubs to `internal/syntax/ast.go` now so Task 2 compiles:

```go
// OpAnnotation returns the action's trailing operation annotation node, or nil
// when the action has none.
func (a ActionDecl) OpAnnotation() *SyntaxNode {
	return a.node.ChildNode(SyntaxKindOpAnnotation)
}

// OpText returns the annotation body verbatim, e.g. "POST /v1/charges", or ""
// when the action has no annotation.
func (a ActionDecl) OpText() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	var sb strings.Builder
	for _, tok := range n.AllTokens() {
		if tok.Kind() == SyntaxKindLBracket || tok.Kind() == SyntaxKindRBracket {
			continue
		}
		sb.WriteString(tok.Text())
	}
	return strings.TrimSpace(sb.String())
}
```

Re-run: `go test ./internal/syntax/ -run TestOpAnnotation -v`
Expected: FAIL with all four annotation tests reporting "expected an op annotation, got none"

- [ ] **Step 3: Add the lookahead scanner**

In `internal/syntax/parser.go`, add immediately above `collectPhrase` (currently line 1164):

```go
// opAnnotationStart scans ahead over the remainder of actionLine and returns the
// lookahead index of the `[` that opens a trailing operation annotation, or -1
// when the line has none.
//
// The annotation is the LAST `[` on the line whose matching `]` is the line's
// final token. Anchoring on the last bracket rather than the first is what lets
// a phrase keep using `[` as prose: `record [batch] entries [POST /v1/entries]`
// puts `[batch]` in the phrase and `[POST /v1/entries]` in the annotation.
// Requiring the line to end in `]` is what lets an unclosed `[` stay prose, so
// files that do not use the feature keep today's sweep-everything behaviour.
func (p *Parser) opAnnotationStart(actionLine int) int {
	lastLBracket, lastTok := -1, -1
	for i := 0; ; i++ {
		t := p.peekAt(i)
		if t.Type == lexer.TokenEOF || t.Type == lexer.TokenRBrace || t.Line != actionLine {
			break
		}
		if t.Type == lexer.TokenLBracket {
			lastLBracket = i
		}
		lastTok = i
	}
	if lastLBracket < 0 || lastTok <= lastLBracket {
		return -1
	}
	if p.peekAt(lastTok).Type != lexer.TokenRBracket {
		return -1
	}
	return lastLBracket
}

// parseOpAnnotation consumes a `[ ... ]` operation annotation at the current
// position and emits it as a SyntaxKindOpAnnotation node. The caller must have
// confirmed via opAnnotationStart that a well-formed annotation begins here.
//
// The body is hybrid: a recognised uppercase protocol verb at the head is
// emitted as SyntaxKindOpVerb, and everything up to the closing `]` is swept in
// verbatim as payload. An unrecognised head word is just the first payload
// token, which is what keeps `[op1/op2/op3]` and `[legacy-txn-44]` legal.
func (p *Parser) parseOpAnnotation() []model.Diagnostic {
	var diags []model.Diagnostic
	p.builder.StartNode(SyntaxKindOpAnnotation)
	p.consumeAs(SyntaxKindLBracket)

	if t := p.peek(); (t.Type == lexer.TokenIdent || isAnyKeywordAsIdent(t.Type)) && isProtocolVerb(t.Value) {
		p.consumeAs(SyntaxKindOpVerb)
	}

	for !p.atEOF() && p.peek().Type != lexer.TokenRBracket {
		switch p.peek().Type {
		case lexer.TokenString:
			p.consumeAs(SyntaxKindString)
		case lexer.TokenNumber, lexer.TokenPercentage:
			p.consumeAs(SyntaxKindNumber)
		default:
			// Idents, keywords-as-idents, and TokenError punctuation (/ ? = { } …)
			// are all swept into the payload as raw tokens, mirroring collectPhrase.
			p.consumeAs(SyntaxKindIdent)
		}
	}

	if p.peek().Type == lexer.TokenRBracket {
		p.consumeAs(SyntaxKindRBracket)
	} else {
		// Defensive: opAnnotationStart guarantees the `]` exists.
		diags = append(diags, p.diagUnexpected(p.peek(), "]"))
	}

	p.builder.FinishNode()
	return diags
}
```

- [ ] **Step 4: Bound collectPhrase**

Replace the signature and loop guard of `collectPhrase` (`internal/syntax/parser.go:1173`):

```go
// collectPhrase emits phrase tokens on actionLine into the current builder scope.
//
// The prose tail is display-only free text (see ActionDecl.PhraseText), so every
// same-line token is swept in verbatim, including TokenError punctuation such as
// `!`, `&`, `*`, `/`, until a line change, `}`, EOF, or a comment token ends it.
//
// maxTokens bounds how many tokens may be consumed: callers pass the lookahead
// index returned by opAnnotationStart so the phrase stops just before a trailing
// operation annotation. Pass -1 for "sweep to end of line".
func (p *Parser) collectPhrase(actionLine int, maxTokens int) {
	if maxTokens == 0 {
		return
	}
	if p.atEOF() || p.peek().Type == lexer.TokenRBrace {
		return
	}
	if p.peek().Line != actionLine {
		return
	}
	consumed := 0
	for {
		if maxTokens >= 0 && consumed >= maxTokens {
			return
		}
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
			p.consumeAs(SyntaxKindIdent)
		}
		consumed++
	}
}
```

- [ ] **Step 5: Wire it into all four action forms**

In `parseAsksAction` (`parser.go:1106`), replace the final `p.collectPhrase(line)` with:

```go
	start := p.opAnnotationStart(line)
	p.collectPhrase(line, start)
	if start >= 0 {
		p.parseOpAnnotation()
	}
```

In `parseReturnsAction` (`parser.go:1145`), replace the final `p.collectPhrase(line)` with the identical three lines.

In `parseAction`'s `default:` internal-action branch (`parser.go:1098`), replace `p.collectPhrase(actionLine)` with the identical three lines using `actionLine`.

In `parseNotifiesAction` (`parser.go:1128`), which has no phrase, append before `return diags`:

```go
	if p.opAnnotationStart(eventTok.Line) >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
```

Note `parseNotifiesAction` already captures `eventTok` at the top, so its line is available.

- [ ] **Step 6: Run the annotation tests**

Run: `go test ./internal/syntax/ -run TestOpAnnotation -v`
Expected: PASS on all five, except `TestOpAnnotation_LastBracketWins`'s `OpText()` assertion and `TestOpAnnotation_Absent`'s `PhraseText()` assertion, which Task 3 completes. If `PhraseText()` returns `"record [batch] entries [POST /v1/entries]"`, that is the expected intermediate state.

- [ ] **Step 7: Run the round-trip and corpus tests**

Run: `go test ./internal/syntax/ -run 'TestRoundTrip|TestLossless' -v`
Expected: PASS. Nothing in the corpus uses `[` on an action line, so behaviour is unchanged.

- [ ] **Step 8: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/ast.go internal/syntax/parser_test.go
git commit -m "feat(syntax): parse trailing operation brackets on all action forms"
```

---

### Task 3: AST accessors and phrase exclusion

**Files:**
- Modify: `internal/syntax/ast.go:1591-1613` (`PhraseText`), and the stubs added in Task 2
- Test: `internal/syntax/ast_test.go`

**Interfaces:**
- Consumes: `SyntaxKindOpAnnotation`, `SyntaxKindOpVerb`
- Produces: `(a ActionDecl) OpAnnotation() *SyntaxNode`, `OpVerb() string`, `OpPayload() string`, `OpText() string`. `PhraseText()` no longer includes annotation text.

**Why `PhraseText` must change:** it walks `a.node.AllTokens()` from the phrase start offset to the end of the node and concatenates everything (`ast.go:1603-1611`). Because the annotation is a child of the action node, its tokens are in `AllTokens()` and would be appended to the phrase.

- [ ] **Step 1: Write the failing test**

Append to `internal/syntax/ast_test.go`:

```go
func TestActionDecl_OpVerbAndPayload(t *testing.T) {
	cases := []struct {
		name        string
		line        string
		wantVerb    string
		wantPayload string
		wantText    string
	}{
		{"http", "A asks B for c [POST /v1/charges]", "POST", "/v1/charges", "POST /v1/charges"},
		{"grpc", "A asks B for c [GRPC ledger.Postings/Create]", "GRPC", "ledger.Postings/Create", "GRPC ledger.Postings/Create"},
		{"topic", "A asks B for c [TOPIC billing.v1.charged]", "TOPIC", "billing.v1.charged", "TOPIC billing.v1.charged"},
		{"opaque path", "A asks B for c [op1/op2/op3/op4/op5]", "", "op1/op2/op3/op4/op5", "op1/op2/op3/op4/op5"},
		{"opaque words", "A asks B for c [legacy-mainframe-txn-44]", "", "legacy-mainframe-txn-44", "legacy-mainframe-txn-44"},
		{"lowercase is not a verb", "A asks B for c [post /v1/x]", "", "post /v1/x", "post /v1/x"},
		{"query string", "A asks B for c [GET /v1/products?q=]", "GET", "/v1/products?q=", "GET /v1/products?q="},
		{"templated path", "A asks B for c [POST /v1/accounts/{id}/charges]", "POST", "/v1/accounts/{id}/charges", "POST /v1/accounts/{id}/charges"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "use_case \"X\" {\n  when U does x\n    " + tc.line + "\n}"
			a := syntax.AsFile(astParse(src)).UseCases()[0].Scenarios()[0].Actions()[0]
			if got := a.OpVerb(); got != tc.wantVerb {
				t.Errorf("OpVerb() = %q, want %q", got, tc.wantVerb)
			}
			if got := a.OpPayload(); got != tc.wantPayload {
				t.Errorf("OpPayload() = %q, want %q", got, tc.wantPayload)
			}
			if got := a.OpText(); got != tc.wantText {
				t.Errorf("OpText() = %q, want %q", got, tc.wantText)
			}
			if got := a.PhraseText(); got != "c" {
				t.Errorf("PhraseText() = %q, want %q (annotation must be excluded)", got, "c")
			}
		})
	}
}

func TestActionDecl_OpAccessors_NoAnnotation(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c\n}"
	a := syntax.AsFile(astParse(src)).UseCases()[0].Scenarios()[0].Actions()[0]
	if a.OpAnnotation() != nil {
		t.Error("OpAnnotation() should be nil")
	}
	if a.OpVerb() != "" || a.OpPayload() != "" || a.OpText() != "" {
		t.Errorf("op accessors should be empty, got verb=%q payload=%q text=%q",
			a.OpVerb(), a.OpPayload(), a.OpText())
	}
}

// The description string must not leak the annotation, since it is what the
// visualizers render as the edge label.
func TestActionDecl_Description_ExcludesAnnotation(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for a fresh charge [POST /v1/charges]\n}"
	a := syntax.AsFile(astParse(src)).UseCases()[0].Scenarios()[0].Actions()[0]
	if got := a.Description(); got != "A asks B for a fresh charge" {
		t.Errorf("Description() = %q, want %q", got, "A asks B for a fresh charge")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run 'TestActionDecl_Op|TestActionDecl_Description_Excludes' -v`
Expected: FAIL, `a.OpVerb undefined` and `PhraseText()` including the annotation text

- [ ] **Step 3: Exclude the annotation from PhraseText**

In `internal/syntax/ast.go`, replace the body of `PhraseText` after `startOffset` is computed (lines 1600-1612) with:

```go
	startOffset := tokens[start].Offset()
	// An operation annotation is a child of the action node, so its tokens are in
	// AllTokens() and would otherwise be appended to the phrase. Stop before it.
	endOffset := -1
	if op := a.OpAnnotation(); op != nil {
		if opToks := op.AllTokens(); len(opToks) > 0 {
			endOffset = opToks[0].Offset()
		}
	}
	var sb strings.Builder
	writing := false
	for _, tok := range a.node.AllTokens() {
		if endOffset >= 0 && tok.Offset() >= endOffset {
			break
		}
		if !writing {
			if tok.Offset() < startOffset {
				continue
			}
			writing = true
		}
		sb.WriteString(tok.Text())
	}
	return strings.TrimSpace(sb.String())
```

- [ ] **Step 4: Add the remaining accessors**

In `internal/syntax/ast.go`, next to the `OpAnnotation` and `OpText` stubs from Task 2, add:

```go
// OpVerb returns the recognised protocol verb of the action's operation
// annotation (e.g. "POST", "GRPC"), or "" when the annotation has no recognised
// verb or the action has no annotation.
func (a ActionDecl) OpVerb() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	if tok := n.ChildToken(SyntaxKindOpVerb); tok != nil {
		return tok.Text()
	}
	return ""
}

// OpPayload returns the annotation body with any recognised protocol verb
// stripped, e.g. "/v1/charges" for `[POST /v1/charges]` and the whole body for
// `[op1/op2/op3]`. Returns "" when the action has no annotation.
func (a ActionDecl) OpPayload() string {
	n := a.OpAnnotation()
	if n == nil {
		return ""
	}
	verb := a.OpVerb()
	if verb == "" {
		return a.OpText()
	}
	return strings.TrimSpace(strings.TrimPrefix(a.OpText(), verb))
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/syntax/ -run 'TestActionDecl_Op|TestActionDecl_Description_Excludes|TestOpAnnotation' -v`
Expected: PASS

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/syntax/ast.go internal/syntax/ast_test.go
git commit -m "feat(syntax): add op-annotation accessors, exclude annotation from phrase"
```

---

### Task 4: Lower the annotation into the model

**Files:**
- Modify: `internal/model/model.go:158-176`
- Modify: `internal/syntax/projection.go:251-364`
- Test: `internal/syntax/projection_uc_test.go`

**Interfaces:**
- Consumes: `ActionDecl.OpVerb()`, `OpPayload()`, `OpText()` from Task 3
- Produces: `model.Operation{Verb, Payload, Text string}` and `model.Action.Operation *Operation`

- [ ] **Step 1: Write the failing test**

Append to `internal/syntax/projection_uc_test.go`:

```go
func TestProject_ActionOperation(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing asks Ledger to record the entry
}`
	g, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	actions := doc.UseCases[0].Scenarios[0].Actions

	if actions[0].Operation == nil {
		t.Fatal("action 0: expected Operation to be set")
	}
	if got := actions[0].Operation.Verb; got != "POST" {
		t.Errorf("Verb = %q, want POST", got)
	}
	if got := actions[0].Operation.Payload; got != "/v1/charges" {
		t.Errorf("Payload = %q, want /v1/charges", got)
	}
	if got := actions[0].Operation.Text; got != "POST /v1/charges" {
		t.Errorf("Text = %q, want %q", got, "POST /v1/charges")
	}
	if got := actions[0].Phrase; got != "a fresh charge attempt" {
		t.Errorf("Phrase = %q, want %q", got, "a fresh charge attempt")
	}
	if got := actions[0].Description; got != "Subscriptions asks Billing for a fresh charge attempt" {
		t.Errorf("Description = %q, must not contain the annotation", got)
	}

	if actions[1].Operation != nil {
		t.Errorf("action 1: expected nil Operation, got %+v", actions[1].Operation)
	}
}

// An absent annotation must marshal to no `operation` key at all, so existing
// .craftjson goldens stay byte-identical.
func TestProject_ActionOperation_OmittedWhenAbsent(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A asks B for c\n}"
	g, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(g), li)
	b, err := json.Marshal(doc.UseCases[0].Scenarios[0].Actions[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "operation") {
		t.Errorf("absent annotation must not emit an operation key, got %s", b)
	}
}
```

Ensure `encoding/json` and `strings` are imported in that test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run TestProject_ActionOperation -v`
Expected: FAIL, `actions[0].Operation undefined`

- [ ] **Step 3: Add the model type**

In `internal/model/model.go`, add above `type Action struct` (line 158):

```go
// Operation is the trailing `[...]` operation annotation on an action line, for
// example `[POST /v1/charges]`. It records the wire call an action corresponds
// to, which makes a use_case readable as an integration contract.
//
// Verb is a recognised protocol verb (GET/POST/GRPC/TOPIC/...) or "" when the
// annotation body did not start with one. Payload is the body with the verb
// stripped. Text is the whole body verbatim, and is always populated.
type Operation struct {
	Verb    string `json:"verb,omitempty"`
	Payload string `json:"payload,omitempty"`
	Text    string `json:"text"`
}
```

And add to `Action` after the `Ref` field (line 171):

```go
	// Operation is nil when the action carries no `[...]` annotation. It is a
	// pointer with omitempty so an absent annotation emits no JSON key at all,
	// keeping .craftjson goldens for unannotated files byte-identical.
	Operation *Operation `json:"operation,omitempty"`
```

- [ ] **Step 4: Populate it in the projection**

In `internal/syntax/projection.go`, add just before the `return model.Action{...}` (currently line 351):

```go
	var operation *model.Operation
	if text := a.OpText(); text != "" {
		operation = &model.Operation{
			Verb:    a.OpVerb(),
			Payload: a.OpPayload(),
			Text:    text,
		}
	}
```

And add the field to the returned struct literal, after `Ref: ref,`:

```go
		Operation:     operation,
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/syntax/ -run TestProject_ActionOperation -v`
Expected: PASS

- [ ] **Step 6: Verify no golden drifted**

Run: `go test ./internal/parser_diff/ ./pkg/craft/ -v`
Expected: PASS. No corpus file uses `[` on an action line yet, and the pointer plus omitempty means absent annotations emit nothing.

- [ ] **Step 7: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/model/model.go internal/syntax/projection.go internal/syntax/projection_uc_test.go
git commit -m "feat(model): lower operation annotations into Action.Operation"
```

---

### Task 5: Reject `kind:` prefixes in the action target slot

**Files:**
- Modify: `internal/syntax/parser.go:1074-1078` (`parseAction` asks branch), `1106-1125` (`parseAsksAction`)
- Modify: `testdata/corpus/99_mixed/dsl-vnext.craft` and `.craftjson`
- Modify: `testdata/broken/malformed_slug.craft` and `.diagnostics.json`
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: `parseRef()` returning the leading kind word (`parser.go:2183`)
- Produces: `parseAsksAction(line int) []model.Diagnostic` (signature change, was no return). Diagnostic code `craft/syntax/kind-prefix-in-target`.

**Rationale:** `context_map` already forbids `bc:` inside its block because the block implies the endpoint kind. The action target slot is equally constrained. The `ns/name` qualified form stays legal.

- [ ] **Step 1: Write the failing test**

Append to `internal/syntax/parser_test.go`:

```go
func TestAsksTarget_RejectsKindPrefix(t *testing.T) {
	cases := []string{"bc:re/billing", "domain:re/monetization", "service:billing-api", "term:billing/dunning"}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			src := "use_case \"X\" {\n  when U does x\n    A asks " + target + " for c\n}"
			_, _, diags := syntax.Parse(src)
			found := false
			for _, d := range diags {
				if d.Code == "craft/syntax/kind-prefix-in-target" {
					found = true
				}
			}
			if !found {
				t.Errorf("expected craft/syntax/kind-prefix-in-target for %q, got %+v", target, diags)
			}
		})
	}
}

func TestAsksTarget_QualifiedPathStillLegal(t *testing.T) {
	for _, target := range []string{"re/billing", "Billing"} {
		t.Run(target, func(t *testing.T) {
			src := "use_case \"X\" {\n  when U does x\n    A asks " + target + " for c\n}"
			tree, _, diags := syntax.Parse(src)
			for _, d := range diags {
				if d.Severity == "error" {
					t.Errorf("unexpected error for %q: [%s] %s", target, d.Code, d.Message)
				}
			}
			a := syntax.AsFile(syntax.Root(tree)).UseCases()[0].Scenarios()[0].Actions()[0]
			if got := a.TargetName(); got != target {
				t.Errorf("TargetName() = %q, want %q", got, target)
			}
		})
	}
}
```

Note: `syntax.Parse` returns `(green root, LineIndex, diags)`; adapt the `syntax.Root(tree)` call to match the existing helper usage in this file if `astParse` already wraps it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/syntax/ -run TestAsksTarget -v`
Expected: FAIL, no `craft/syntax/kind-prefix-in-target` diagnostic emitted

- [ ] **Step 3: Emit the diagnostic**

In `internal/syntax/parser.go`, change `parseAsksAction` to return diagnostics and check the kind word:

```go
// parseAsksAction parses: <target> to|for <phrase> [op_annotation]
// The `asks` keyword token has already been consumed by parseAction.
//
// A `kind:` prefix in the target slot is rejected: the slot already implies a
// bounded context, the same rule context_map enforces for its endpoints. The
// qualified `<domain>/<name>` form stays legal.
func (p *Parser) parseAsksAction(line int) []model.Diagnostic {
	var diags []model.Diagnostic

	targetTok := p.peek()
	if targetTok.Type == lexer.TokenIdent {
		if kind := p.parseRef(); kind != "" {
			diags = append(diags, model.Diagnostic{
				Code: "craft/syntax/kind-prefix-in-target",
				Message: fmt.Sprintf(
					"remove the %q prefix: an asks target is always a bounded context, write the bare or <domain>/<name> form",
					kind+":"),
				Severity: model.SeverityError,
				Range:    tokenRange(targetTok),
			})
		}
	} else if isAnyKeywordAsIdent(targetTok.Type) {
		p.consumeAs(SyntaxKindIdent)
	}

	// connector: "to" or "for"
	connTok := p.peek()
	if connTok.Type == lexer.TokenIdent && (connTok.Value == "to" || connTok.Value == "for") {
		if connTok.Value == "to" {
			p.consumeAs(SyntaxKindKwTo)
		} else {
			p.consumeAs(SyntaxKindIdent)
		}
	}

	start := p.opAnnotationStart(line)
	p.collectPhrase(line, start)
	if start >= 0 {
		diags = append(diags, p.parseOpAnnotation()...)
	}
	return diags
}
```

`tokenRange` is the package-level helper at `parser.go:1751`. Note that parser diagnostics do not set `SourceURI` (see `diagUnexpected` at `parser.go:1699-1706`); the workspace layer fills it in.

Then update the caller in `parseAction` (`parser.go:1074-1078`):

```go
	case "asks":
		p.consumeAs(SyntaxKindKwAsks)
		diags = append(diags, p.parseAsksAction(actionLine)...)
		p.builder.FinishNode()
		return diags
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/syntax/ -run TestAsksTarget -v`
Expected: PASS

- [ ] **Step 5: Update the two affected fixtures**

In `testdata/corpus/99_mixed/dsl-vnext.craft`, change the asks line to drop the prefix and exercise the new annotation:

```craft
    Subscriptions asks re/billing for a fresh charge attempt (1! & 2!) [POST /v1/charges]
```

In `testdata/broken/malformed_slug.craft`, the fixture's asks target `foo:re/x` now produces `craft/syntax/kind-prefix-in-target` instead of `craft/sema/malformed-slug`. Read the file and its `.diagnostics.json`, then update the expected code and message. If the fixture's purpose was specifically to exercise `malformed-slug`, move that assertion to a `context_map` endpoint in the same file, which still validates slug shape.

- [ ] **Step 6: Regenerate the corpus golden**

There is no `-update` flag in this repo. Regenerate `dsl-vnext.craftjson` by hand:

```bash
go run ./cmd/craft parse testdata/corpus/99_mixed/dsl-vnext.craft > /tmp/dsl-vnext.json
```

Check `cmd/craft`'s actual subcommand name first with `go run ./cmd/craft --help`. If no JSON-emitting subcommand exists, write a throwaway `TestMain`-style helper that calls `syntax.Parse` then `syntax.ProjectFromTree` then `json.Marshal` and writes the file, matching exactly what `internal/parser_diff/v2_test.go:80-85` does, then delete the helper.

Diff the result against the old golden and confirm the only changes are the target text losing `bc:` and a new `operation` object on that one action.

- [ ] **Step 7: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/parser_test.go testdata/
git commit -m "feat(syntax): reject kind: prefixes in the asks target slot"
```

---

### Task 6: Diagnose ambiguous bare targets in use cases

**Files:**
- Modify: `internal/sema/lint.go:338-356`
- Test: `internal/sema/validate_relationship_test.go` or a new `internal/sema/lint_usecase_test.go`

**Interfaces:**
- Consumes: `resolveBCRef(ws WorkspaceSymbols, scopeDomain, ref string) (resolved, kind string, ambiguous bool)` (`internal/sema/validate.go:362`)
- Produces: diagnostic `craft/sema/ambiguous-bc` emitted for use-case action subjects and targets

**Rationale:** `resolveBCRef` already detects ambiguity (`validate.go:392-394`), and `buildDependencyEdges` already calls it (`lint.go:341-342`) but silently drops the ambiguous case. Dropping `bc:` prefixes in Task 5 is only safe if that ambiguity becomes visible.

- [ ] **Step 1: Write the failing test**

Create `internal/sema/lint_usecase_test.go`:

```go
package sema

import "testing"

// Two domains both own a BC named Billing, so a bare `Billing` in a use_case
// action is genuinely ambiguous and must be reported rather than silently
// dropped from the dependency graph.
func TestAmbiguousBC_InUseCaseAction(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Subscriptions asks Billing for a charge
}`
	diags := analyzeSingleFileForTest(t, src)
	found := false
	for _, d := range diags {
		if d.Code == "craft/sema/ambiguous-bc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected craft/sema/ambiguous-bc, got %+v", diags)
	}
}

func TestAmbiguousBC_QualifiedTargetIsClean(t *testing.T) {
	src := `domain re {
  Billing
  Subscriptions
}

domain ops {
  Billing
}

use_case "X" {
  when U does x
    Subscriptions asks re/billing for a charge
}`
	for _, d := range analyzeSingleFileForTest(t, src) {
		if d.Code == "craft/sema/ambiguous-bc" {
			t.Errorf("qualified target must not be ambiguous, got %+v", d)
		}
	}
}
```

Read `internal/sema/validate_relationship_test.go:14-50` to find the existing helper that builds a workspace from a source string and runs analysis. Name the helper `analyzeSingleFileForTest` if none exists, or use the existing one and adjust these two tests to call it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/sema/ -run TestAmbiguousBC -v`
Expected: FAIL on the first test, no `craft/sema/ambiguous-bc` emitted

- [ ] **Step 3: Emit the diagnostic**

`buildDependencyEdges` returns only `deps`. Rather than change its signature, add a sibling pass in `internal/sema/lint.go` that walks the same trees and reports ambiguity. Its parameter list mirrors `buildDependencyEdges` exactly (`lint.go:309`).

Do not reuse `endpointDiag` (`validate.go:727`) wholesale: its non-ambiguous branches emit context_map-specific messages ("context_map endpoint %q resolves to a %s") that would be wrong here. Only the ambiguous case applies, so emit it directly with the same code and message shape.

```go
// lintAmbiguousUseCaseRefs reports bare bounded-context names in use_case
// actions that resolve to two or more domains. resolveBCRef already detects
// this (validate.go:392), but buildDependencyEdges silently drops the ambiguous
// case, which would let an unresolvable model look clean now that `bc:`
// prefixes are no longer accepted in the target slot.
//
// Only the subject and target of a sync_action are checked. Internal and return
// actions do not participate in the dependency graph, so an ambiguous name there
// is not yet actionable.
func lintAmbiguousUseCaseRefs(
	perFileTrees map[string]syntax.SyntaxNode,
	ws WorkspaceSymbols,
	lis map[string]green.LineIndex,
) []model.Diagnostic {
	var diags []model.Diagnostic
	for uri, tree := range perFileTrees {
		li := lis[uri]
		file := syntax.AsFile(tree)
		for _, uc := range file.UseCases() {
			for _, sc := range uc.Scenarios() {
				for _, action := range sc.Actions() {
					if action.Kind() != "sync_action" {
						continue
					}
					if name := action.TargetName(); name != "" {
						if _, _, ambiguous := resolveBCRef(ws, "", name); ambiguous {
							diags = append(diags, ambiguousUseCaseRefDiag(
								uri, name, action.Line(li), action.TargetCol(li)))
						}
					}
					if name := action.SubjectName(); name != "" {
						if _, _, ambiguous := resolveBCRef(ws, "", name); ambiguous {
							diags = append(diags, ambiguousUseCaseRefDiag(
								uri, name, action.Line(li), action.SubjectCol(li)))
						}
					}
				}
			}
		}
	}
	return diags
}

// ambiguousUseCaseRefDiag mirrors the ambiguous-bc branch of endpointDiag
// (validate.go:734-741) so the code and wording stay identical across the
// context_map and use_case sites.
func ambiguousUseCaseRefDiag(uri, ref string, line, col int) model.Diagnostic {
	startChar := colToLSP(col)
	return model.Diagnostic{
		Code: "craft/sema/ambiguous-bc",
		Message: fmt.Sprintf(
			"bounded context %q is declared in multiple domains; qualify it as <domain>/%s", ref, ref),
		Severity:  model.SeverityError,
		SourceURI: uri,
		Range: model.Range{
			Start: model.Position{Line: lineToLSP(line), Character: startChar},
			End:   model.Position{Line: lineToLSP(line), Character: startChar + lspLen(ref)},
		},
	}
}
```

Then call `lintAmbiguousUseCaseRefs(perFileTrees, ws, lis)` from the same place `buildDependencyEdges` is called in the workspace analysis entry point, appending its diagnostics to the returned set. Find that call site with `grep -n buildDependencyEdges internal/sema/*.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/sema/ -run TestAmbiguousBC -v`
Expected: PASS

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS. If a corpus or broken fixture now reports a new ambiguity, that is a real finding: either the fixture has two same-named BCs and should qualify the reference, or the resolution logic needs narrowing. Do not suppress it without understanding which.

- [ ] **Step 6: Commit**

```bash
git add internal/sema/lint.go internal/sema/validate.go internal/sema/lint_usecase_test.go
git commit -m "feat(sema): report ambiguous bare bounded-context refs in use cases"
```

---

### Task 7: LSP semantic tokens and completion

**Files:**
- Modify: `internal/lsp/server.go` (semantic token classification, around `classifyActionIdents` at ~L2388-2463; `CompletionOptions.TriggerCharacters` at ~L221)
- Modify: `internal/lsp/completion.go`
- Test: existing LSP test files in `internal/lsp/`

**Interfaces:**
- Consumes: `syntax.SyntaxKindOpAnnotation`, `syntax.SyntaxKindOpVerb`, `syntax.ProtocolVerbs()`
- Produces: annotation tokens classified distinctly from phrase tokens; protocol verb completion after `[`

- [ ] **Step 1: Write the failing test**

Read `internal/lsp/completion.go` and its test file to find the existing table-driven completion test, then add a case:

```go
{
	name: "protocol verbs after open bracket",
	src: "use_case \"X\" {\n  when U does x\n    A asks B for c [\n}",
	// position: immediately after the `[`
	wantContains: []string{"POST", "GET", "GRPC", "TOPIC"},
},
```

Match the existing test's struct fields and position-specification style exactly; do not invent a new harness.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run Completion -v`
Expected: FAIL, no protocol verbs offered

- [ ] **Step 3: Add `[` as a trigger character**

In `internal/lsp/server.go`, find `CompletionOptions` (around L221, currently `TriggerCharacters: []string{":"}`) and add `"["`.

- [ ] **Step 4: Offer protocol verbs at annotation position 0**

In `internal/lsp/completion.go`, add a context detector alongside the existing ones (see `ctxUseCaseActionVerb` and `isAfterActionVerb`, ~L24 and ~L181-187):

```go
// ctxOpAnnotationVerb is the completion context immediately inside an operation
// annotation's `[`, where a protocol verb is the only useful suggestion. Once a
// verb or any payload token has been typed the payload is opaque and Craft has
// nothing to offer, so this context applies only at the head position.
```

Implement the detector by walking to the token at the cursor and checking whether its parent node is `SyntaxKindOpAnnotation` and whether the only preceding sibling is the `SyntaxKindLBracket`. Return `syntax.ProtocolVerbs()` as completion items with kind `Keyword`.

- [ ] **Step 5: Classify annotation tokens for semantic highlighting**

In `classifyActionIdents` (`internal/lsp/server.go` ~L2388-2463), skip tokens whose parent node is `SyntaxKindOpAnnotation` from phrase classification, and emit them as a distinct semantic token type instead. Use the existing token-type registration list in the same file; if no suitable type exists, register the annotation payload as `string` and the verb as `keyword`, which every default theme dims and colours appropriately without extra client config.

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/lsp/ -v`
Expected: PASS

- [ ] **Step 7: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/lsp/
git commit -m "feat(lsp): complete protocol verbs and highlight operation annotations"
```

---

### Task 8: Column-align annotations in the formatter

**Files:**
- Modify: whichever file implements `FormatDocument` (`internal/lsp/server.go:1325` registers `DocumentFormattingProvider`; follow it to the implementation)
- Test: a new formatter test file alongside the implementation

**Interfaces:**
- Consumes: `ActionDecl.OpAnnotation()`
- Produces: annotations aligned per contiguous run of annotated action lines

**Rules:** alignment is cosmetic and the grammar stays whitespace-insensitive. Alignment must be idempotent: formatting an already-formatted document produces no edits. A run is broken by a blank line or by any non-action line. Within a run, the annotation column is `max(len(line before annotation)) + 2` over the run's annotated lines.

- [ ] **Step 1: Write the failing test**

```go
func TestFormat_AlignsOpAnnotations(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]
    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
}`
	want := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry
}`
	got := formatSource(t, src)
	if got != want {
		t.Errorf("format mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFormat_AlignmentIsIdempotent(t *testing.T) {
	src := `use_case "Retry" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
}`
	once := formatSource(t, src)
	twice := formatSource(t, once)
	if once != twice {
		t.Errorf("format is not idempotent\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestFormat_BlankLineResetsRun(t *testing.T) {
	src := `use_case "Retry" {
  when A does x
    Subscriptions asks Billing for a very long fresh charge attempt [POST /v1/charges]

  when B does y
    A asks C for d [GET /v1/d]
}`
	got := formatSource(t, src)
	// The short second run must NOT be padded out to the first run's column.
	if strings.Contains(got, "A asks C for d  [GET /v1/d]") == false {
		t.Errorf("second run should align independently, got:\n%s", got)
	}
}
```

Write `formatSource` as a helper that calls the same entry point the LSP formatting handler uses.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lsp/ -run TestFormat_Align -v`
Expected: FAIL, annotations unaligned

- [ ] **Step 3: Implement the alignment pass**

Add a post-pass over the formatted output: group consecutive action lines that carry an annotation, compute the target column as described, and rewrite the whitespace immediately before each annotation's `[`. Preserve everything else byte-for-byte.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/lsp/ -run TestFormat -v`
Expected: PASS

- [ ] **Step 5: Verify the formatter is stable on the whole corpus**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/lsp/
git commit -m "feat(fmt): column-align operation annotations per contiguous run"
```

---

### Task 9: Corpus fixture and documentation

**Files:**
- Create: `testdata/corpus/04_use_cases/op_annotations.craft` and `.craftjson`
- Modify: `docs/page/language/use-cases.md`
- Modify: `.claude/skills/craft-dsl/SKILL.md`
- Modify: `CLAUDE.md` (DSL keyword table)

- [ ] **Step 1: Write the corpus fixture**

Create `testdata/corpus/04_use_cases/op_annotations.craft`:

```craft
domain re {
  Subscriptions
  Billing
  Ledger
}

use_case "Retry a failed charge" {
  when CRON detects a failed charge
    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/accounts/{id}/charges]
    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]
    Billing asks Ledger to record the entry                [GRPC ledger.Postings/Create]
    Gateway returns to Billing the authorization result    [200 AuthorizationResult]
    Billing notifies billing.ChargeSucceeded               [TOPIC billing.v1.charge-succeeded]
    Subscriptions marks the subscription active
    Subscriptions asks Audit to log the outcome            [op1/op2/op3/op4/op5]
    Subscriptions asks Legacy for a reconciliation         [legacy-mainframe-txn-44]
}
```

This exercises: every action form, a recognised verb, an unrecognised head word, a templated path, a query-free path, a dotted gRPC method, a numeric-headed body (`200`), and two opaque payloads.

- [ ] **Step 2: Run round-trip and confirm it parses clean**

Run: `go test ./internal/syntax/ -run TestRoundTrip -v`
Expected: PASS, including the new fixture

- [ ] **Step 3: Generate the golden**

Generate `op_annotations.craftjson` using the same method established in Task 5 Step 6. Inspect it and confirm every annotated action carries an `operation` object with the expected `verb`/`payload`/`text`, and that `Subscriptions marks the subscription active` has no `operation` key.

- [ ] **Step 4: Run the golden test**

Run: `go test ./internal/parser_diff/ -v`
Expected: PASS

- [ ] **Step 5: Update the language docs**

In `docs/page/language/use-cases.md`:

- Add an "Operation Annotations" section after "Actions", documenting the trailing bracket, the hybrid contents, the protocol verb set, and the last-bracket-wins rule.
- Update the "Synchronous Actions" section: replace the `bc:re/billing` example (line ~123) with `re/billing`, and state that `kind:` prefixes are not accepted in the target slot.
- Narrow the punctuation claim at line ~125: the phrase still accepts `! & * / # ? +` unquoted, but a `[` whose `]` closes the line now opens an operation annotation.

In `.claude/skills/craft-dsl/SKILL.md`, update the node-slug section (lines ~124 and ~227) the same way.

In `CLAUDE.md`, add a row to the DSL keyword table for the operation annotation and update the "Key DSL Example" to show one.

- [ ] **Step 6: Run the full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add testdata/corpus/04_use_cases/ docs/page/language/use-cases.md .claude/skills/craft-dsl/SKILL.md CLAUDE.md
git commit -m "docs: document operation annotations, add corpus fixture"
```

---

---

### Task 10: Changelog and version documentation

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/page/language/use-cases.md` (breaking-change callout)

**Version:** `v3.0.0`. This is a **breaking** change: `asks bc:re/billing` no longer parses. Craft's precedent is that DSL breaking changes bump major (v2.0.0 was the `domains:` to `contexts:` rename). Current tag is `v2.15.2`.

- [ ] **Step 1: Write the changelog entry**

Add at the top of `CHANGELOG.md`, below the `# Changelog` heading, matching the existing style (bold lead sentence per bullet, cause-and-consequence prose, a `### Notes` section for consumer impact):

```markdown
## [3.0.0] — <release date>

### Breaking
- **`kind:` prefixes are no longer accepted in an `asks` target.** `Subscriptions asks bc:re/billing for a charge` is now a parse error (`craft/syntax/kind-prefix-in-target`). Write the bare form `Billing` or the qualified form `re/billing`. The slot already implies a bounded context, which is the same rule `context_map` has always enforced for its endpoints.
  - Migration is mechanical: strip the `bc:` / `domain:` / `service:` / `term:` prefix. Two files in this repo were affected, both test fixtures.
- **A `[` that closes at end of an action line now opens an operation annotation** instead of being swept into the free-text phrase. A `[` that does not close at end of line is still prose, so only lines that look like they carry an annotation change meaning.

### Added
- **Operation annotations.** Any action may end with a bracketed annotation recording the wire call it corresponds to, which makes a `use_case` readable as an integration contract:

      Subscriptions asks Billing for a fresh charge attempt  [POST /v1/accounts/{id}/charges]
      Billing asks Ledger to record the entry                [GRPC ledger.Postings/Create]
      Billing notifies billing.ChargeSucceeded               [TOPIC billing.v1.charge-succeeded]

  Contents are hybrid: a recognised uppercase protocol verb (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`, `GRPC`, `TOPIC`, `QUERY`) is parsed as structure, and anything else is kept verbatim as an opaque payload, so `[op1/op2/op3]` and `[legacy-mainframe-txn-44]` are equally valid.
  - Surfaces as `Action.Operation` (`{verb, payload, text}`) in `pkg/craft` and the `.craftjson` projection. The field is omitted entirely when absent, so existing golden files are unchanged.
  - `craft fmt` column-aligns annotations per contiguous run of annotated lines.
  - Completion offers the protocol verbs after `[`.
- **`craft/sema/ambiguous-bc` now fires for use-case action subjects and targets.** A bare bounded-context name owned by two or more domains was previously dropped silently from the dependency graph. It is now an error naming the candidates, fixable by writing `<domain>/<name>`.

### Notes
- **No generated diagram changes.** No visualizer reads `Action.Operation`, exactly as none read `Action.Ref`. Every generator still renders `Context`, `TargetContext`, `Connector`, and `Phrase` only. The annotation is stored for tooling that consumes the contract; rendering it is a separate decision.
- The `<phrase>` tail still accepts `! & * / # ? +` unquoted. Only a line-final bracketed run is now reserved.
```

- [ ] **Step 2: Verify the changelog renders**

Run: `head -60 CHANGELOG.md`
Expected: the new section is first, heading levels match the surrounding entries

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md docs/page/language/use-cases.md
git commit -m "docs: changelog for v3.0.0 operation annotations"
```

---

### Task 11: Release craft v3.0.0

**Confirm with the user before running Step 3.** Tagging is irreversible: it fires `release.yml` (goreleaser to GitHub Releases plus the `tcarcao/homebrew-craft` tap) and `publish.yml` (multi-arch Docker Hub push) simultaneously. Do not tag on a whim.

There is no version constant in the source. Goreleaser injects the tag into `main.version`.

- [ ] **Step 1: Confirm the whole suite is green on the feature branch**

```bash
go test ./...
```
Expected: PASS, no skips that were previously running

- [ ] **Step 2: Merge to main with the house message form**

```bash
git checkout main
git merge --no-ff <branch> -m "Merge <branch>: operation annotations and target-slot cleanup (v3.0.0)"
git push origin main
```

- [ ] **Step 3: Tag and push (CONFIRM FIRST)**

```bash
git tag -a v3.0.0 -m "v3.0.0: operation annotations, kind: prefixes removed from asks targets"
git push origin v3.0.0
```

- [ ] **Step 4: Verify the external result, not the workflow conclusion**

A green workflow does not prove anything landed. Check each registry directly. Indexing lags by a few minutes, so re-query before concluding something is wrong.

```bash
# GitHub release assets exist and the checksum matches
gh release view v3.0.0 --repo tcarcao/craft
gh release download v3.0.0 --repo tcarcao/craft --pattern '*checksums.txt' --output - | head

# Homebrew tap formula picked up the version
gh api repos/tcarcao/homebrew-craft/contents/Formula/craft.rb --jq '.content' | base64 -d | grep -n 'version\|url'

# Docker Hub tag exists
curl -s 'https://hub.docker.com/v2/repositories/tiagocarcao/craft/tags/v3.0.0' | head -c 400
```

Expected: release has binaries for every goreleaser target, the tap formula names 3.0.0, and the Docker tag resolves.

- [ ] **Step 5: Smoke-test the published binary**

```bash
brew update && brew upgrade craft 2>/dev/null || brew install tcarcao/craft/craft
craft --version
printf 'domain re {\n  Billing\n}\n\nuse_case "X" {\n  when U does x\n    A asks Billing for c [POST /v1/charges]\n}\n' > /tmp/smoke.craft
craft validate /tmp/smoke.craft
```
Expected: version reports 3.0.0, and the annotation parses with no diagnostics

---

### Task 12: Update the VS Code extension

**Repo:** `../craft-vscode-extension` (separate git repo). Current `version` 0.2.2, `lspVersion` 2.15.2.

**Hard ordering constraint:** the extension's release workflow smoke-tests the craft binary download by `lspVersion` against the craft GitHub release. **Task 11 must be fully released and verified before this task starts**, or the extension build fails.

**Channel:** derived from minor-version parity. Odd minor = pre-release, even minor = stable. `0.2.2` to `0.2.3` keeps minor 2, so it stays on the **stable** channel. Confirm that is intended before tagging, given the craft side is a major bump.

- [ ] **Step 1: Bump both version fields**

In `../craft-vscode-extension/package.json`:

```json
  "version": "0.2.3",
  "lspVersion": "3.0.0",
```

Do **not** bump `package-lock.json`'s version field. It has been stale since 0.1.10 and is deliberately left alone.

- [ ] **Step 2: Update the extension changelog and any syntax-highlighting grammar**

The extension ships its own TextMate grammar. Check whether it needs a rule for the operation bracket:

```bash
grep -rn "asks\|notifies" ../craft-vscode-extension/syntaxes/ | head
```

If the grammar tokenises the phrase tail, add a pattern for a line-final `[...]` so the annotation is coloured distinctly rather than as prose. If the LSP's semantic tokens (Task 7) already cover it, no grammar change is needed. Verify which by opening a `.craft` file with an annotation after installing the built VSIX.

- [ ] **Step 3: Commit and tag**

Commit signing is configured (`commit.gpgsign=true`) but the key is unavailable here, and recent history is already unsigned. Use `--no-gpg-sign` rather than changing the config.

```bash
cd ../craft-vscode-extension
git add package.json CHANGELOG.md
git commit --no-gpg-sign -m "chore: bump to 0.2.3, lspVersion 3.0.0"
git tag -a v0.2.3 -m "v0.2.3: craft 3.0.0 with operation annotations"
git push origin main v0.2.3
```

- [ ] **Step 4: Verify both registries, checking the channel**

```bash
# Open VSX
curl -s https://open-vsx.org/api/tcarcao/craft-arch-diagrams | head -c 400

# Marketplace, including the pre-release flag
curl -s -X POST 'https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery' \
  -H 'Content-Type: application/json' -H 'Accept: application/json;api-version=3.0-preview.1' \
  -d '{"filters":[{"criteria":[{"filterType":7,"value":"tcarcao.craft-arch-diagrams"}]}],"flags":914}' \
  | python3 -m json.tool | grep -A2 'PreRelease\|"version"' | head -30
```

Expected: both registries report 0.2.3, and the Marketplace `Microsoft.VisualStudio.Code.PreRelease` property confirms the **stable** channel. This is the exact check that was missing in the v0.2.1 incident, where every release silently went to the wrong channel while CI stayed green.

---

## Deferred, deliberately

These are in the spec's "Out of scope" section. Do not implement them in this plan.

- Rendering the annotation in generated diagrams. No visualizer reads `Action.Ref` today and none will read `Action.Operation` after this plan. The field exists for future tooling.
- Validating the payload against an anchored OpenAPI or protobuf spec.
- `use_case "..." in <domain>` scoping.
- Deleting the now-simplifiable `refAwareText` call sites in `ast.go` (`Connector()` at ~L1256, `phraseStartIndex` at ~L1620, `ActionDecl.PhraseStartIndex()`, and the `sync_action` branches of `TargetName`/`TargetCol` at ~L1441/L1484). Task 5 removes `kind:` prefixes but a target can still be a two-segment `re/billing` ref, so the span-skipping logic is still load-bearing. Revisit only if the qualified form is ever dropped.
- `import "<path>"`, which lexes and parses (`parser.go:79,165`) but has no consumer. Separate decision.
