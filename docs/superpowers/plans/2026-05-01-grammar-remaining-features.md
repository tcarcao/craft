# Grammar Remaining Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the three genuine grammar gaps that remain after Q5–Q11 (doc comments `///`, CRON/time-based triggers, multi-file imports) plus a trigger error-path correctness fix.

**Architecture:** All changes touch the hand-written lexer (`internal/lexer/lexer.go`) and recursive-descent parser (`internal/syntax/parser.go`), with mirrored updates to the tree-sitter grammar (`tree-sitter-craft/grammar.js`). Every new language construct follows the same pattern: new token constant → new SyntaxKind constant → lexer scan/classify → parser consume/build → test.

**Tech Stack:** Go 1.22 (`go test ./...` from `craft/`), tree-sitter Node.js CLI (`cd tree-sitter-craft && npx tree-sitter generate && npx tree-sitter test`).

**Status note:** Q5–Q11 from the previous plan are already fully implemented in the codebase. This plan covers only what remains.

---

## File Map

| File | Changes |
|---|---|
| `craft/internal/lexer/lexer.go` | Add `TokenDocComment`, `TokenKwImport`; distinguish `///` from `//` |
| `craft/internal/lexer/lexer_test.go` | Tests for `TokenDocComment`, `TokenKwImport` |
| `craft/internal/syntax/kind.go` | Add `SyntaxKindDocComment`, `SyntaxKindKwCron`, `SyntaxKindKwEvery`, `SyntaxKindKwImport`; add `SyntaxKindImportDecl` node kind |
| `craft/internal/syntax/parser.go` | `attachTrivia` + `peek`/`peekAt` for doc comment; `parseTrigger` for cron/every + error-path fix; `parseImportStatement` + wiring in `parseFile`/`isTopLevelKeyword` |
| `craft/internal/syntax/parser_test.go` | Tests for all four features |
| `tree-sitter-craft/grammar.js` | Add `doc_comment` rule; fix `cron_trigger` casing + add `every` variant; add `import_decl` |

---

## Task 1: Lexer — doc comment token (`///`)

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `craft/internal/lexer/lexer_test.go` after the existing `TestLexer_BlockComment` function:

```go
func TestLexer_DocComment(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "doc comment distinguished from line comment",
			src:       "/// doc\nactor user Alice",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "regular line comment still works",
			src:       "// regular\nactor user Alice",
			wantTypes: []lexer.TokenType{lexer.TokenLineComment, lexer.TokenKwActor, lexer.TokenKwUser, lexer.TokenIdent, lexer.TokenEOF},
		},
		{
			name:      "doc comment at EOF no newline",
			src:       "/// doc at eof",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenEOF},
		},
		{
			name:      "doc comment value includes ///",
			src:       "/// hello world",
			wantTypes: []lexer.TokenType{lexer.TokenDocComment, lexer.TokenEOF},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
	// Value check: doc comment value must start with ///
	l := lexer.New("/// my doc")
	toks := l.All()
	if toks[0].Type != lexer.TokenDocComment {
		t.Fatalf("expected TokenDocComment, got %v", toks[0].Type)
	}
	if len(toks[0].Value) < 3 || toks[0].Value[:3] != "///" {
		t.Errorf("doc comment value should start with ///, got %q", toks[0].Value)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd craft && go test ./internal/lexer/ -run TestLexer_DocComment -v
```

Expected: `FAIL` — `lexer.TokenDocComment` undefined.

- [ ] **Step 3: Add `TokenDocComment` constant to lexer.go**

In `craft/internal/lexer/lexer.go`, find the block comment token line:

```go
	TokenLineComment  // // ... single-line comment
	TokenBlockComment // /* ... */ block comment
```

Change to:

```go
	TokenLineComment  // // ... single-line comment (two slashes, not three)
	TokenBlockComment // /* ... */ block comment
	TokenDocComment   // /// ... doc comment (three slashes)
```

- [ ] **Step 4: Add `scanDocComment` and update `Next()` dispatch**

In `craft/internal/lexer/lexer.go`, find the `Next()` switch case:

```go
	case ch == '/' && l.peek(1) == '/':
		return l.scanLineComment()
```

Replace with (three-slash check MUST come first — Go switch falls through to first match):

```go
	case ch == '/' && l.peek(1) == '/' && l.peek(2) == '/':
		return l.scanDocComment()
	case ch == '/' && l.peek(1) == '/':
		return l.scanLineComment()
```

Then add `scanDocComment` immediately after `scanLineComment`:

```go
// scanDocComment scans a /// doc comment and returns it as a TokenDocComment.
func (l *Lexer) scanDocComment() Token {
	startLine, startCol := l.line, l.col
	start := l.pos
	l.advance() // /
	l.advance() // /
	l.advance() // /
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	return Token{Type: TokenDocComment, Value: string(l.src[start:l.pos]), Line: startLine, Column: startCol}
}
```

- [ ] **Step 5: Run the test to verify it passes**

```bash
cd craft && go test ./internal/lexer/ -run TestLexer_DocComment -v
```

Expected: `PASS`

- [ ] **Step 6: Run full lexer tests to check for regressions**

```bash
cd craft && go test ./internal/lexer/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd craft && git add internal/lexer/lexer.go internal/lexer/lexer_test.go
git commit -m "feat(lexer): add TokenDocComment for /// doc comments"
```

---

## Task 2: Parser — doc comment kind + trivia wiring

**Files:**
- Modify: `craft/internal/syntax/kind.go`
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Write the failing test**

Add to `craft/internal/syntax/parser_test.go`:

```go
func TestParseTree_DocCommentPreserved(t *testing.T) {
	src := "/// Doc for actor\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	tree := syntax.Root(g)
	actors := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	allToks := actors[0].AllTokens()
	hasDoc := false
	for _, tok := range allToks {
		if tok.Kind() == syntax.SyntaxKindDocComment {
			hasDoc = true
			break
		}
	}
	if !hasDoc {
		t.Error("expected leading doc comment to appear in actor node's AllTokens()")
	}
}

func TestParseTree_DocCommentNotMistakenForLineComment(t *testing.T) {
	// /// must produce SyntaxKindDocComment, not SyntaxKindLineComment
	src := "/// Doc\nactor system Foo"
	g, _, _ := syntax.Parse(src)
	tree := syntax.Root(g)
	actors := tree.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	for _, tok := range actors[0].AllTokens() {
		if tok.Kind() == syntax.SyntaxKindLineComment {
			t.Error("/// should produce SyntaxKindDocComment, not SyntaxKindLineComment")
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd craft && go test ./internal/syntax/ -run "TestParseTree_DocComment" -v
```

Expected: `FAIL` — `syntax.SyntaxKindDocComment` undefined.

- [ ] **Step 3: Add `SyntaxKindDocComment` to kind.go**

In `craft/internal/syntax/kind.go`, find:

```go
	SyntaxKindLineComment  // // ...
	SyntaxKindBlockComment // /* ... */
	SyntaxKindWhitespace   // spaces, tabs, newlines between tokens
```

Change to:

```go
	SyntaxKindLineComment  // // ...
	SyntaxKindBlockComment // /* ... */
	SyntaxKindDocComment   // /// ...
	SyntaxKindWhitespace   // spaces, tabs, newlines between tokens
```

Also add token-kind constants for the new contextual keywords and import keyword BEFORE `syntaxKindTokenSentinel`. Find:

```go
	SyntaxKindKwTrue  // future: boolean values in modifier properties
	SyntaxKindKwFalse // future: boolean values in modifier properties

	// syntaxKindTokenSentinel marks the end of token kinds.
	syntaxKindTokenSentinel
```

Change to:

```go
	SyntaxKindKwTrue  // future: boolean values in modifier properties
	SyntaxKindKwFalse // future: boolean values in modifier properties

	// New contextual keywords for triggers and imports
	SyntaxKindKwCron   // contextual keyword `cron` (in when clause)
	SyntaxKindKwEvery  // contextual keyword `every` (in when clause)
	SyntaxKindKwImport // hard keyword `import`

	// syntaxKindTokenSentinel marks the end of token kinds.
	syntaxKindTokenSentinel
```

Also add the import node kind. Find the node kinds block (after `SyntaxKindServiceField`):

```go
	SyntaxKindServiceField // wraps one field declaration (keyword + colon + values) in a service body
)
```

Change to:

```go
	SyntaxKindServiceField // wraps one field declaration (keyword + colon + values) in a service body
	SyntaxKindImportDecl   // import "path/to/file.craft"
)
```

- [ ] **Step 4: Wire doc comment into parser.go**

In `craft/internal/syntax/parser.go`, find `lexerKindToSyntaxKindMap`:

```go
	lexer.TokenLineComment:  SyntaxKindLineComment,
	lexer.TokenBlockComment: SyntaxKindBlockComment,
```

Add after:

```go
	lexer.TokenLineComment:  SyntaxKindLineComment,
	lexer.TokenBlockComment: SyntaxKindBlockComment,
	lexer.TokenDocComment:   SyntaxKindDocComment,
```

Find `peek()`:

```go
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment {
			continue
		}
```

Change to:

```go
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment || tt == lexer.TokenDocComment {
			continue
		}
```

Find `peekAt()` (same pattern, ~line 1223):

```go
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment {
			continue
		}
```

Change to:

```go
		if tt == lexer.TokenLineComment || tt == lexer.TokenBlockComment || tt == lexer.TokenDocComment {
			continue
		}
```

Find `attachTrivia()`:

```go
		case lexer.TokenBlockComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindBlockComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		default:
			return
```

Add a case for `TokenDocComment` between `TokenBlockComment` and `default`:

```go
		case lexer.TokenBlockComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindBlockComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		case lexer.TokenDocComment:
			p.emitWhitespaceBefore(tok)
			p.builder.Token(SyntaxKindDocComment, tok.Value)
			p.updatePrevEnd(tok)
			p.pos++
		default:
			return
```

- [ ] **Step 5: Run to verify tests pass**

```bash
cd craft && go test ./internal/syntax/ -run "TestParseTree_DocComment" -v
```

Expected: `PASS`

- [ ] **Step 6: Run full syntax tests to check for regressions**

```bash
cd craft && go test ./internal/syntax/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd craft && git add internal/syntax/kind.go internal/syntax/parser.go internal/syntax/parser_test.go
git commit -m "feat(parser): wire SyntaxKindDocComment for /// doc comments"
```

---

## Task 3: Tree-sitter — doc comment mirror

**Files:**
- Modify: `tree-sitter-craft/grammar.js`

- [ ] **Step 1: Update comment rule to distinguish doc comments**

In `tree-sitter-craft/grammar.js`, find the `comment` rule:

```js
    comment: $ => choice(
      seq('//', /.*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    ),
```

Replace with (doc_comment must come BEFORE comment in extras so tree-sitter's longest-match picks `///` over `//`):

```js
    doc_comment: $ => /\/\/\/[^\n]*/,

    comment: $ => choice(
      /\/\/[^\n]*/,
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    ),
```

Find the `extras` rule at the top:

```js
  extras: $ => [
    /[ \t]/,
    $._newline,
    $.comment,
  ],
```

Change to:

```js
  extras: $ => [
    /[ \t]/,
    $._newline,
    $.doc_comment,
    $.comment,
  ],
```

(doc_comment listed before comment so tree-sitter resolves `///` correctly.)

- [ ] **Step 2: Generate and test**

```bash
cd tree-sitter-craft && npx tree-sitter generate && npx tree-sitter test
```

Expected: all corpus tests pass.

- [ ] **Step 3: Commit**

```bash
cd tree-sitter-craft && git add grammar.js
git commit -m "feat(tree-sitter): add doc_comment rule for /// comments"
```

---

## Task 4: Trigger error-path fix + CRON/periodic trigger kinds

**Files:**
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

This task fixes the bug where `parseTrigger` silently returns without consuming a bad subject token, then adds `SyntaxKindKwCron` and `SyntaxKindKwEvery` wiring.

- [ ] **Step 1: Write failing tests**

Add to `craft/internal/syntax/parser_test.go`:

```go
func TestRecovery_TriggerBadSubjectConsumed(t *testing.T) {
	// A bad token in the trigger subject position must not leave the token
	// stream stuck, causing parseAction to loop indefinitely on the same token.
	src := "use_case \"X\" {\n  when 42 bad stuff\n    Domain asks Other to something\n}"
	gn, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic for bad trigger subject")
	}
	root := syntax.Root(gn)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	// Parser must not loop: the use_case node must be closed (test completes without hanging).
}

func TestParseTree_CronTrigger(t *testing.T) {
	src := "use_case \"Nightly\" {\n  when cron \"0 0 * * *\"\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	ucNodes := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)
	if len(ucNodes) != 1 {
		t.Fatalf("expected 1 use_case node, got %d", len(ucNodes))
	}
	scenarios := ucNodes[0].ChildNodes(syntax.SyntaxKindScenario)
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	trigger := scenarios[0].ChildNodes(syntax.SyntaxKindTrigger)
	if len(trigger) != 1 {
		t.Fatalf("expected 1 trigger node, got %d", len(trigger))
	}
	cronKw := trigger[0].ChildToken(syntax.SyntaxKindKwCron)
	if cronKw == nil || cronKw.Text() != "cron" {
		t.Errorf("expected cron keyword token in trigger, got %v", cronKw)
	}
}

func TestParseTree_EveryTrigger(t *testing.T) {
	src := "use_case \"Polling\" {\n  when every \"5m\"\n}"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	scenarios := root.ChildNodes(syntax.SyntaxKindUseCaseDecl)[0].ChildNodes(syntax.SyntaxKindScenario)
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	trigger := scenarios[0].ChildNodes(syntax.SyntaxKindTrigger)[0]
	everyKw := trigger.ChildToken(syntax.SyntaxKindKwEvery)
	if everyKw == nil || everyKw.Text() != "every" {
		t.Errorf("expected every keyword token in trigger, got %v", everyKw)
	}
}

func TestParseTree_CronTriggerMissingString(t *testing.T) {
	// when cron without a string expression should produce a diagnostic but not crash.
	src := "use_case \"Bad\" {\n  when cron\n}"
	_, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Error("expected diagnostic for missing cron expression string")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd craft && go test ./internal/syntax/ -run "TestRecovery_TriggerBadSubjectConsumed|TestParseTree_CronTrigger|TestParseTree_EveryTrigger|TestParseTree_CronTriggerMissingString" -v
```

Expected: `FAIL` — `SyntaxKindKwCron` and `SyntaxKindKwEvery` are undefined; the bad-subject test may hang or produce wrong results.

- [ ] **Step 3: Fix the error path in `parseTrigger` and add cron/every handling**

In `craft/internal/syntax/parser.go`, find the `parseTrigger` function. The section after the string-trigger check and before the subject validation looks like:

```go
	// The first token is the actor/domain subject.
	subjectTok := p.peek()
	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		diags = append(diags, p.diagUnexpected(subjectTok, "trigger subject (actor/domain name)"))
		p.builder.FinishNode()
		return diags
	}
```

Replace this entire block with:

```go
	// The first token is the actor/domain subject.
	subjectTok := p.peek()

	// cron trigger: when cron "0 * * * *"
	if subjectTok.Type == lexer.TokenIdent && subjectTok.Value == "cron" {
		p.consumeAs(SyntaxKindKwCron)
		if p.peek().Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "cron expression string (e.g. \"0 * * * *\")"))
		}
		p.builder.FinishNode()
		return diags
	}

	// periodic trigger: when every "1h"
	if subjectTok.Type == lexer.TokenIdent && subjectTok.Value == "every" {
		p.consumeAs(SyntaxKindKwEvery)
		if p.peek().Type == lexer.TokenString {
			p.consumeAs(SyntaxKindString)
		} else {
			diags = append(diags, p.diagUnexpected(p.peek(), "duration string (e.g. \"1h\")"))
		}
		p.builder.FinishNode()
		return diags
	}

	if subjectTok.Type != lexer.TokenIdent && !isAnyKeywordAsIdent(subjectTok.Type) {
		diags = append(diags, p.diagUnexpected(subjectTok, "trigger subject (actor/domain name)"))
		p.consumeAs(SyntaxKindError) // consume bad token to unblock the action loop
		p.builder.FinishNode()
		return diags
	}
```

- [ ] **Step 4: Run to verify tests pass**

```bash
cd craft && go test ./internal/syntax/ -run "TestRecovery_TriggerBadSubjectConsumed|TestParseTree_CronTrigger|TestParseTree_EveryTrigger|TestParseTree_CronTriggerMissingString" -v
```

Expected: `PASS`

- [ ] **Step 5: Run full syntax tests to check for regressions**

```bash
cd craft && go test ./internal/syntax/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd craft && git add internal/syntax/parser.go internal/syntax/parser_test.go
git commit -m "feat(parser): add cron/every triggers and fix trigger error-path recovery"
```

---

## Task 5: Tree-sitter — CRON trigger mirror

**Files:**
- Modify: `tree-sitter-craft/grammar.js`

- [ ] **Step 1: Update `cron_trigger` rule**

In `tree-sitter-craft/grammar.js`, find:

```js
    cron_trigger: $ => prec.left(seq(
      'CRON',
      optional($.phrase),
    )),
```

Replace with:

```js
    cron_trigger: $ => choice(
      seq('cron', $.string),   // when cron "0 * * * *"
      seq('every', $.string),  // when every "1h"
    ),
```

- [ ] **Step 2: Generate and test**

```bash
cd tree-sitter-craft && npx tree-sitter generate && npx tree-sitter test
```

Expected: all corpus tests pass. (The old `CRON` upper-case form is removed — if any corpus test uses it, update the corpus test to use lowercase `cron`.)

- [ ] **Step 3: Commit**

```bash
cd tree-sitter-craft && git add grammar.js
git commit -m "feat(tree-sitter): fix cron_trigger casing, add every variant"
```

---

## Task 6: Lexer — import keyword

**Files:**
- Modify: `craft/internal/lexer/lexer.go`
- Modify: `craft/internal/lexer/lexer_test.go`

- [ ] **Step 1: Write the failing test**

Add to `craft/internal/lexer/lexer_test.go`:

```go
func TestLexer_ImportKeyword(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantTypes []lexer.TokenType
	}{
		{
			name:      "import keyword",
			src:       `import "other.craft"`,
			wantTypes: []lexer.TokenType{lexer.TokenKwImport, lexer.TokenString, lexer.TokenEOF},
		},
		{
			name:      "import not confused with identifier",
			src:       "import_service",
			wantTypes: []lexer.TokenType{lexer.TokenIdent, lexer.TokenEOF},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.New(tc.src)
			toks := l.All()
			if len(toks) != len(tc.wantTypes) {
				t.Fatalf("token count: got %d want %d\ntokens: %v", len(toks), len(tc.wantTypes), toks)
			}
			for i, tok := range toks {
				if tok.Type != tc.wantTypes[i] {
					t.Errorf("token[%d]: got type %v want %v (value=%q)", i, tok.Type, tc.wantTypes[i], tok.Value)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd craft && go test ./internal/lexer/ -run TestLexer_ImportKeyword -v
```

Expected: `FAIL` — `lexer.TokenKwImport` undefined.

- [ ] **Step 3: Add `TokenKwImport` to lexer.go**

In `craft/internal/lexer/lexer.go`, find the sentinel comment near the end of the token enum:

```go
	// Future keyword slots (other slices add their tokens before TokenSentinel)
	TokenSentinel // keep last
```

Change to:

```go
	// Top-level structural keywords
	TokenKwImport // import

	// Future keyword slots (other slices add their tokens before TokenSentinel)
	TokenSentinel // keep last
```

Add `"import"` to the `keywords` map:

```go
var keywords = map[string]TokenType{
	"actor":    TokenKwActor,
	"actors":   TokenKwActors,
	"user":     TokenKwUser,
	"system":   TokenKwSystem,
	"service":  TokenKwService,
	"domain":   TokenKwDomain,
	"domains":  TokenKwDomains,
	"services": TokenKwServices,
	"use_case": TokenKwUseCase,
	"arch":     TokenKwArch,
	"exposure": TokenKwExposure,
	"import":   TokenKwImport,
}
```

- [ ] **Step 4: Run to verify tests pass**

```bash
cd craft && go test ./internal/lexer/ -run TestLexer_ImportKeyword -v
```

Expected: `PASS`

- [ ] **Step 5: Run all lexer tests**

```bash
cd craft && go test ./internal/lexer/ -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd craft && git add internal/lexer/lexer.go internal/lexer/lexer_test.go
git commit -m "feat(lexer): add TokenKwImport keyword"
```

---

## Task 7: Parser — import statement

**Files:**
- Modify: `craft/internal/syntax/parser.go`
- Modify: `craft/internal/syntax/parser_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `craft/internal/syntax/parser_test.go`:

```go
func TestParseTree_ImportDecl(t *testing.T) {
	src := `import "services/payments.craft"`
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	imports := root.ChildNodes(syntax.SyntaxKindImportDecl)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import node, got %d", len(imports))
	}
	kwTok := imports[0].ChildToken(syntax.SyntaxKindKwImport)
	if kwTok == nil || kwTok.Text() != "import" {
		t.Errorf("missing import keyword token")
	}
	pathTok := imports[0].ChildToken(syntax.SyntaxKindString)
	if pathTok == nil || pathTok.Text() != "services/payments.craft" {
		t.Errorf("missing or wrong import path token, got %v", pathTok)
	}
}

func TestParseTree_MultipleImports(t *testing.T) {
	src := "import \"a.craft\"\nimport \"b.craft\"\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	root := syntax.Root(g)
	imports := root.ChildNodes(syntax.SyntaxKindImportDecl)
	if len(imports) != 2 {
		t.Fatalf("expected 2 import nodes, got %d", len(imports))
	}
	actors := root.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor node, got %d", len(actors))
	}
}

func TestParseTree_ImportMissingPath(t *testing.T) {
	// import without a string path should produce a diagnostic but not crash.
	src := "import\nactor user Alice"
	g, _, diags := syntax.Parse(src)
	if len(diags) == 0 {
		t.Error("expected diagnostic for missing import path")
	}
	root := syntax.Root(g)
	// actor should still parse after the bad import
	actors := root.ChildNodes(syntax.SyntaxKindActorDecl)
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor node after bad import, got %d", len(actors))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd craft && go test ./internal/syntax/ -run "TestParseTree_ImportDecl|TestParseTree_MultipleImports|TestParseTree_ImportMissingPath" -v
```

Expected: `FAIL` — `SyntaxKindImportDecl` and `SyntaxKindKwImport` already added in Task 2's kind.go changes but `parseImportStatement` is not wired yet.

- [ ] **Step 3: Add `parseImportStatement` function to parser.go**

In `craft/internal/syntax/parser.go`, add the following function (place it near `parseActorStatement`):

```go
// parseImportStatement parses: import "<path>"
func (p *Parser) parseImportStatement() []craft.Diagnostic {
	p.builder.StartNode(SyntaxKindImportDecl)
	var diags []craft.Diagnostic

	p.attachTrivia()
	p.consumeAs(SyntaxKindKwImport)

	pathTok := p.peek()
	if pathTok.Type != lexer.TokenString {
		diags = append(diags, p.diagUnexpected(pathTok, "import path string (e.g. \"other.craft\")"))
		p.resyncToTopLevel()
		p.builder.FinishNode()
		return diags
	}
	p.consumeAs(SyntaxKindString)

	p.builder.FinishNode()
	return diags
}
```

- [ ] **Step 4: Wire import into `parseFile` and `isTopLevelKeyword`**

In `parseFile`'s main switch, add a case for `TokenKwImport`. Find:

```go
		case lexer.TokenKwActor:
			diags = append(diags, p.parseActorStatement()...)
```

Add BEFORE it (imports typically appear at the top of a file):

```go
		case lexer.TokenKwImport:
			diags = append(diags, p.parseImportStatement()...)
```

In `isTopLevelKeyword`, find:

```go
func isTopLevelKeyword(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwService, lexer.TokenKwServices,
		lexer.TokenKwUseCase, lexer.TokenKwArch, lexer.TokenKwExposure:
		return true
	}
	return false
}
```

Change to:

```go
func isTopLevelKeyword(tt lexer.TokenType) bool {
	switch tt {
	case lexer.TokenKwImport,
		lexer.TokenKwActor, lexer.TokenKwActors,
		lexer.TokenKwDomain, lexer.TokenKwDomains,
		lexer.TokenKwService, lexer.TokenKwServices,
		lexer.TokenKwUseCase, lexer.TokenKwArch, lexer.TokenKwExposure:
		return true
	}
	return false
}
```

Also add to `lexerKindToSyntaxKindMap`:

```go
	lexer.TokenKwImport: SyntaxKindKwImport,
```

- [ ] **Step 5: Run to verify tests pass**

```bash
cd craft && go test ./internal/syntax/ -run "TestParseTree_ImportDecl|TestParseTree_MultipleImports|TestParseTree_ImportMissingPath" -v
```

Expected: `PASS`

- [ ] **Step 6: Run full test suite**

```bash
cd craft && go test ./... -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd craft && git add internal/syntax/kind.go internal/syntax/parser.go internal/syntax/parser_test.go
git commit -m "feat(parser): add import statement parsing"
```

---

## Task 8: Tree-sitter — import_decl mirror

**Files:**
- Modify: `tree-sitter-craft/grammar.js`

- [ ] **Step 1: Add `import_decl` to grammar.js**

In `tree-sitter-craft/grammar.js`, find `_top_level_item`:

```js
    _top_level_item: $ => choice(
      $.arch_block,
      $.services_block,
      $.service_block,
      $.domain_block,
      $.domains_block,
      $.actors_block,
      $.actor_block,
      $.exposure_block,
      $.use_case_block,
    ),
```

Change to:

```js
    _top_level_item: $ => choice(
      $.import_decl,
      $.arch_block,
      $.services_block,
      $.service_block,
      $.domain_block,
      $.domains_block,
      $.actors_block,
      $.actor_block,
      $.exposure_block,
      $.use_case_block,
    ),
```

Add the `import_decl` rule (add it near other declaration rules, before `arch_block`):

```js
    import_decl: $ => seq(
      'import',
      $.string,
    ),
```

- [ ] **Step 2: Generate and test**

```bash
cd tree-sitter-craft && npx tree-sitter generate && npx tree-sitter test
```

Expected: all corpus tests pass.

- [ ] **Step 3: Verify parse of an import statement**

```bash
cd tree-sitter-craft && echo 'import "other.craft"' | npx tree-sitter parse /dev/stdin
```

Expected output contains `(import_decl` with a `string` child.

- [ ] **Step 4: Commit**

```bash
cd tree-sitter-craft && git add grammar.js
git commit -m "feat(tree-sitter): add import_decl rule"
```

---

## Task 9: Final integration check

- [ ] **Step 1: Run the complete Go test suite**

```bash
cd craft && go test ./... 2>&1 | grep -E "FAIL|ok|---"
```

Expected: all packages report `ok`.

- [ ] **Step 2: Run tree-sitter tests**

```bash
cd tree-sitter-craft && npx tree-sitter generate && npx tree-sitter test
```

Expected: all corpus tests pass.

- [ ] **Step 3: Smoke-test a combined .craft file**

Create a temp file and parse it end-to-end:

```bash
cat > /tmp/test_grammar.craft << 'EOF'
/// Documentation for this module
import "shared/actors.craft"

/* Define actors */
actors {
  user Customer
  bot AutomationBot
}

domain Payments {
  Billing
  Invoicing
}

service PaymentService {
  contexts: Billing, Invoicing
  language: golang
  deployment: canary(90%->stable, 10%->canary)
}

use_case "Nightly Reconciliation" {
  when cron "0 0 * * *"
    Billing asks Invoicing to reconcile accounts
}

use_case "Polling Feed" {
  when every "5m"
    Billing asks Invoicing to fetch updates
}

arch Payments {
  presentation: Dashboard[sla:"99.9%", timeout:30]
  gateway: API > Auth > PaymentService
}
EOF
cd craft && go run ./cmd/craft/... --parser=v2 /tmp/test_grammar.craft 2>&1 || true
```

Expected: parses without panics. (Diagnostic output is acceptable since `actors.craft` doesn't exist for the import.)

- [ ] **Step 4: Final commit**

```bash
cd craft && git add -A
git commit -m "feat(grammar): complete remaining grammar features — doc comments, cron triggers, imports"
```
