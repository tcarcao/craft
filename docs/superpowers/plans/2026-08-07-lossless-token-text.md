# Lossless Token Text Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `concat(AllTokens().Text()) == src` a structural guarantee of craft's parser, then delete the two formatter workarounds that existed only because it was not.

**Architecture:** The lexer records each token's byte end alongside its start, so the parser can slice token text directly from source instead of deriving it from `Value`/`Raw`. Token length stops being inferred from token text, which is what allowed a wrong text to produce a consistently wrong length that the width check could never detect. With text fidelity guaranteed, trailing trivia is tokenized like all other trivia, `trailingCommentLines` is deleted, and the formatter's alignment pass switches from a line-index approximation to exact token ends.

**Tech Stack:** Go 1.23, module `github.com/tcarcao/craft/v2`.

**Design record:** `docs/decisions/lossless-token-text.md`. Read it before Task 1 — it explains why width equality is structurally incapable of catching this bug class, and why a hard failure is safe here.

## Global Constraints

- The module path is `github.com/tcarcao/craft/v2`. Do not modify `go.mod`.
- `lexer.Token.Value` keeps its current meaning: for `TokenString` it is the *unescaped content without quotes*. Content consumers depend on this. Only the green tree's text changes to raw source.
- After Task 3, `concat(AllTokens().Text()) == src` must hold for every `.craft` file in the repository, including `testdata/broken/`.
- The formatter's existing guarantees must not regress: content preservation, reparse-clean, idempotence, model preservation over all `.craft` files.
- Canonical corpus count must be >= 44. Golden files must not change. `arch` blocks must remain verbatim.
- No em dashes in commit messages, changelog entries, or code comments.
- Never run two implementers against the same working tree.

---

### Task 1: Lexer records each token's byte end

**Files:**
- Modify: `internal/lexer/lexer.go:116-138` (the `Token` struct), and the 10 `Token{...}` construction sites at lines 276, 289, 309, 366, 368, 386, 407, 415, 417, 427
- Test: `internal/lexer/lexer_test.go`

**Interfaces:**
- Produces: `lexer.Token.End int` — the 0-based BYTE offset one past the token's last byte. For every token, `src[tok.Offset:tok.End]` is the token's verbatim source text. Task 2 consumes this.

**Context:** `Token.Offset` is the byte start. There is no end. The parser currently infers the end as `tok.Offset + len(tokenText(tok))`, so a token whose text is wrong gets a length that is wrong by exactly the same amount, and `checkTreeWidth` still passes. Recording the true end breaks that circularity.

Each construction site already has the end in hand: `l.pos` is the scanner position just past the token, and `l.offsetAt(n)` converts a rune index to a byte offset. Note the sites use different start expressions (`start`, `rawStart`, `l.pos`) — the end is `l.offsetAt(l.pos)` at sites 276-417, but site 427 constructs its token *before* advancing, so read that function carefully and compute the end that actually matches the token's text.

- [ ] **Step 1: Write the failing test**

```go
// internal/lexer/lexer_test.go

// Every token's [Offset,End) must slice its own source text, and tokens
// plus the gaps between them must tile the source exactly.
func TestToken_EndSlicesSource(t *testing.T) {
	sources := []string{
		"domain re {\n  Billing\n}\n",
		"// lead\ndomain re { Billing }\n",
		"/* block */ domain re { Billing }\n",
		"use_case \"X\" {\n  when U does x A notifies \"Hello world\"\n}\n",
		"use_case \"X\" {\n  when U does x A notifies \"Oops\n}\n", // unterminated
		"domain re { Billing }  [POST /v1/charges]\n",
		"services { S { contexts: A, B\n language: golang } }\n",
		"@#$\n", // lexer error tokens
		"",
	}
	for _, src := range sources {
		toks := lexer.New(src).All()
		prevEnd := 0
		for _, tok := range toks {
			if tok.Type == lexer.TokenEOF {
				continue
			}
			if tok.Offset < prevEnd {
				t.Fatalf("src=%q token %v overlaps previous: Offset=%d prevEnd=%d",
					src, tok.Type, tok.Offset, prevEnd)
			}
			if tok.End < tok.Offset || tok.End > len(src) {
				t.Fatalf("src=%q token %v has bad range [%d,%d) len=%d",
					src, tok.Type, tok.Offset, tok.End, len(src))
			}
			prevEnd = tok.End
		}
	}
}

// The raw slice must be non-empty for every non-EOF token: a zero-width
// token would let a construct vanish from the tree without changing widths.
func TestToken_EndIsNonEmpty(t *testing.T) {
	toks := lexer.New("use_case \"X\" {\n  when U does x A notifies \"Oops\n}\n").All()
	for _, tok := range toks {
		if tok.Type == lexer.TokenEOF {
			continue
		}
		if tok.End <= tok.Offset {
			t.Fatalf("token %v is zero-width at offset %d", tok.Type, tok.Offset)
		}
	}
}
```

`internal/lexer/lexer_test.go` is `package lexer_test`, so use the exported API: `lexer.New(src).All()` returns `[]Token`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lexer/ -run 'TestToken_End' -v`
Expected: FAIL to compile with `tok.End undefined`.

- [ ] **Step 3: Add the field**

```go
	// Offset is the 0-based BYTE offset of the token's first character from
	// the start of the source. This is the authoritative position for building
	// the green tree, whose widths must sum to len(src) exactly.
	Offset int
	// End is the 0-based BYTE offset one past the token's last byte, so that
	// src[Offset:End] is the token's verbatim source text for every token
	// kind, including malformed ones. The green tree slices this rather than
	// deriving length from token text: deriving it meant a wrong text produced
	// a consistently wrong length, which no width check could detect.
	End int
```

- [ ] **Step 4: Populate it at all 10 construction sites**

Add `End: l.offsetAt(l.pos)` to the sites at lines 276, 289, 309, 366, 368, 386, 407, 415, 417. For site 427, determine the correct end from that function's control flow rather than assuming.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/lexer/ -v`
Expected: PASS, including all pre-existing lexer tests.

- [ ] **Step 6: Commit**

```bash
git add internal/lexer/lexer.go internal/lexer/lexer_test.go
git commit -m "feat(lexer): record each token's byte end

Token length was inferred from token text, so a token whose text
diverged from its source produced a matching wrong length and the
tree-width check could never detect it. Recording the true end
breaks that circularity."
```

---

### Task 2: Parser slices token text from source

**Files:**
- Modify: `internal/syntax/parser.go` — `tokenText` (1727-1732), `updatePrevEnd` (1713-1718), and the emit sites at 1742, 1747, 1752, 1771, 1786
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: `lexer.Token.End` from Task 1.
- Produces: green-tree token text that is always a verbatim source slice.

**Context:** Five emit sites build token text three different ways. Lines 1742/1747/1752 pass `tok.Value` for comments; 1771/1786 pass `tokenText(tok)`, which returns `tok.Raw` only for `TokenString` when `Raw != ""` and otherwise falls through to `Value`. An unterminated string never populates `Raw`, so it falls through to `Value` — the string body without its opening quote. That is the defect.

Replace all of it with one method that cannot be bypassed.

- [ ] **Step 1: Write the failing test**

```go
// internal/syntax/parser_test.go

// Rowan's actual invariant: concatenating the tokens reproduces the source.
// Width equality is a checksum that cannot catch a token lying about its text.
func TestParse_ConcatEqualsSource(t *testing.T) {
	sources := map[string]string{
		"normal":            "domain re {\n  Billing\n}\n",
		"string":            "use_case \"X\" {\n  when U does x A notifies \"Hello\"\n}\n",
		"escaped string":    "use_case \"X\" {\n  when U does x A notifies \"a\\\"b\"\n}\n",
		"unterminated":      "use_case \"X\" {\n  when U does x A notifies \"Oops\n}\n",
		"leading comment":   "// lead\ndomain re { Billing }\n",
		"trailing comment":  "domain re { Billing }\n// trail\n",
		"comment only":      "// just this\n",
		"block comment":     "/* a\n   b */\ndomain re { Billing }\n",
		"annotation":        "use_case \"X\" {\n  when U does x A asks B to y  [POST /v1/c]\n}\n",
		"lexer error":       "@#$\n",
		"empty":             "",
	}
	for name, src := range sources {
		gn, _, _ := syntax.Parse(src)
		var sb strings.Builder
		for _, tk := range syntax.Root(gn).AllTokens() {
			sb.WriteString(tk.Text())
		}
		if sb.String() != src {
			t.Errorf("%s: concat != src\n got: %q\nwant: %q", name, sb.String(), src)
		}
	}
}

// Every token's text must equal the source bytes at its own range.
func TestParse_TokenTextMatchesItsRange(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x A notifies \"Oops\n}\n"
	gn, _, _ := syntax.Parse(src)
	for _, tk := range syntax.Root(gn).AllTokens() {
		r := tk.TextRange()
		if int(r.End) > len(src) {
			t.Fatalf("token %v range %v exceeds source length %d", tk.Kind(), r, len(src))
		}
		if got := src[r.Start:r.End]; got != tk.Text() {
			t.Errorf("token %v: Text()=%q but source at range is %q", tk.Kind(), tk.Text(), got)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/syntax/ -run 'TestParse_Concat|TestParse_TokenText' -v`
Expected: FAIL on `unterminated` in both tests. The other cases should already pass.

- [ ] **Step 3: Add the slicing method and use it everywhere**

```go
// rawText returns the token's verbatim source text. Token text in the green
// tree is always a slice of the source: this is what makes
// concat(AllTokens) == src true by construction rather than by convention.
// Never build token text from tok.Value or tok.Raw, which carry the token's
// interpreted content and diverge from source for strings and error tokens.
func (p *Parser) rawText(tok lexer.Token) string {
	if tok.Offset < 0 || tok.End > len(p.src) || tok.End < tok.Offset {
		return tok.Value // defensive: malformed range, should be unreachable
	}
	return p.src[tok.Offset:tok.End]
}
```

Then:
- lines 1742, 1747, 1752: `tok.Value` becomes `p.rawText(tok)`
- lines 1771, 1786: `tokenText(tok)` becomes `p.rawText(tok)`
- `updatePrevEnd`: `p.prevEnd = green.TextSize(tok.End)`
- delete `tokenText` entirely

`tokenText` at parser.go:1728 is the only consumer of `lexer.Token.Raw` in the entire repository, so deleting `tokenText` makes `Raw` dead. Delete the `Raw` field, its doc comment, and its population at lexer.go:368. Also update the stale comment at parser.go:2514, which describes `tokenText`'s behaviour and outlives it.

Ignore `.worktrees/` when grepping — it holds detached copies of the repo and is not part of the build.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/syntax/ ./internal/lexer/ -v 2>&1 | tail -20`
Expected: PASS. Then `go test ./...` — report any package that newly fails, do not paper over it.

- [ ] **Step 5: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/parser_test.go internal/lexer/lexer.go
git commit -m "fix(syntax): slice token text from source at every emit site

An unterminated string never populated Raw, so tokenText fell through
to Value and the tree carried the string body without its opening
quote. Building text from the source slice closes the class rather
than the instance."
```

---

### Task 3: Text equality replaces width equality

**Files:**
- Modify: `internal/syntax/parser.go:41` and `checkTreeWidth` (53-64)
- Test: `internal/syntax/parser_test.go`

**Interfaces:**
- Consumes: the guarantee from Task 2.

**Context:** `checkTreeWidth` compares `root.Width()` to `len(src)` and emits a diagnostic on mismatch. After Task 2 the stronger property holds, so assert the stronger property. Per the design record this is a hard failure, not a diagnostic: because token text is now a source slice and recovery wraps rather than drops, a malformed `.craft` file cannot trigger it. Only a craft bug can.

"Hard failure" means panic. The LSP must not crash the editor on a malformed file, so the panic can only be reached by a genuine internal inconsistency — which is exactly the guarantee Task 2 provides. Do not add a recover(); if this fires, it is a bug that must be fixed, not tolerated.

- [ ] **Step 1: Write the failing test**

```go
// internal/syntax/parser_test.go

// Every .craft file in the repository must round-trip through the tree.
func TestParse_ConcatEqualsSource_AllRepoFiles(t *testing.T) {
	var files []string
	for _, root := range []string{"../../testdata", "../../examples", "../../docs"} {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".craft") {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) < 90 {
		t.Fatalf("expected to find at least 90 .craft files, found %d", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(b)
		gn, _, _ := syntax.Parse(src)
		var sb strings.Builder
		for _, tk := range syntax.Root(gn).AllTokens() {
			sb.WriteString(tk.Text())
		}
		if sb.String() != src {
			t.Errorf("%s: tree does not reproduce source", f)
		}
	}
}
```

The `len(files) < 90` guard matters: without it, a broken walk finds zero files and the test passes vacuously.

- [ ] **Step 2: Run it**

Run: `go test ./internal/syntax/ -run TestParse_ConcatEqualsSource_AllRepoFiles -v`
Expected: PASS if Tasks 1-2 are correct. If it fails, the named file is a real defect — fix it before continuing rather than adjusting the test.

- [ ] **Step 3: Replace the check**

```go
// checkTreeText asserts rowan's invariant: concatenating the tree's tokens
// reproduces the source exactly. This supersedes a width-sum check, which
// could not detect a token whose text diverged from its source range because
// the token's length was derived from that same wrong text.
//
// This panics rather than returning a diagnostic. Token text is a slice of
// the source and error recovery wraps tokens in an ErrorNode rather than
// dropping them, so no input can reach here: a failure is a parser bug.
func checkTreeText(root *green.GreenNode, src string) {
	if root == nil {
		return
	}
	var sb strings.Builder
	sb.Grow(len(src))
	collectGreenText(root, &sb)
	if sb.String() != src {
		panic(fmt.Sprintf(
			"internal parser error: syntax tree does not reproduce its source (tree %d bytes, source %d bytes)",
			sb.Len(), len(src)))
	}
}
```

Write `collectGreenText` to walk the green tree appending every token's text, or reuse an existing traversal if one already does this. Update the call site at line 41: it no longer contributes to `diags`.

- [ ] **Step 4: Verify the whole suite**

Run: `go test ./... 2>&1 | grep -v '^ok' | head -20`
Expected: no failures. A panic here means a real violation — report it, do not weaken the check.

- [ ] **Step 5: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/parser_test.go
git commit -m "feat(syntax): assert text equality instead of width equality

Width equality proved the tree spans the source, not that it
reproduces it. After token text became a source slice the stronger
invariant holds by construction, so assert it."
```

---

### Task 4: Tokenize trailing trivia

**Files:**
- Modify: `internal/syntax/parser.go:111-114` (end of `parseFile`)
- Test: `internal/syntax/parser_test.go`

**Context:** `peek()` (line 1687) skips comment tokens. At end of file it therefore skips any trailing comments and returns EOF, so `atEOF()` is true, the main loop exits with `p.pos` still positioned before those comments, and lines 112-113 sweep everything from `prevEnd` to end of source into a single `Whitespace` token. Result:

```
trailing comment   domain re { Billing }  Whitespace:"\n// trail\n"
comment only       Whitespace:"// just this\n"
```

`attachTrivia` (line 1736) already emits comment tokens correctly. Call it before the trailing-whitespace capture.

- [ ] **Step 1: Write the failing test**

```go
// internal/syntax/parser_test.go

// Comments after the last declaration must be comment tokens, not folded
// into a trailing whitespace blob. The formatter walks tokens, so a comment
// hidden inside whitespace cannot be formatted like every other comment.
func TestParse_TrailingCommentsAreTokens(t *testing.T) {
	cases := map[string]string{
		"trailing line":    "domain re { Billing }\n// trail\n",
		"comment only":     "// just this\n",
		"trailing block":   "domain re { Billing }\n/* trail */\n",
		"two trailing":     "domain re { Billing }\n// one\n// two\n",
		"trailing doc":     "domain re { Billing }\n/// doc\n",
	}
	for name, src := range cases {
		gn, _, _ := syntax.Parse(src)
		comments := 0
		for _, tk := range syntax.Root(gn).AllTokens() {
			switch tk.Kind() {
			case syntax.SyntaxKindLineComment, syntax.SyntaxKindBlockComment, syntax.SyntaxKindDocComment:
				comments++
			case syntax.SyntaxKindWhitespace:
				if strings.Contains(tk.Text(), "//") || strings.Contains(tk.Text(), "/*") {
					t.Errorf("%s: comment folded into whitespace token %q", name, tk.Text())
				}
			}
		}
		if comments == 0 {
			t.Errorf("%s: expected at least one comment token, got none", name)
		}
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/syntax/ -run TestParse_TrailingCommentsAreTokens -v`
Expected: FAIL — zero comment tokens, and the whitespace-contains-comment check fires.

- [ ] **Step 3: Attach trailing trivia**

```go
	// Trailing comments are trivia like any other: peek() skips comment
	// tokens, so at EOF the main loop leaves them unconsumed. Emit them as
	// comment tokens before sweeping the rest, or they land inside one
	// whitespace blob and no token-walking consumer can see them.
	p.attachTrivia()

	// Capture trailing whitespace/newlines after the last token.
	if int(p.prevEnd) < len(p.src) {
		p.builder.Token(SyntaxKindWhitespace, p.src[p.prevEnd:])
	}
```

- [ ] **Step 4: Verify**

Run: `go test ./internal/syntax/ ./internal/lsp/ 2>&1 | tail -20`
Expected: PASS, including the Task 3 round-trip test. If `TestParse_ConcatEqualsSource_AllRepoFiles` fails, `attachTrivia` is double-emitting or skipping — fix that, it is a real bug.

- [ ] **Step 5: Commit**

```bash
git add internal/syntax/parser.go internal/syntax/parser_test.go
git commit -m "fix(syntax): tokenize comments after the last declaration

peek() skips comment tokens, so trailing comments were never consumed
and got swept into one whitespace token. This was the only reason the
formatter needed a separate text-scraping path for them."
```

---

### Task 5: Delete `trailingCommentLines`

**Files:**
- Modify: `internal/lsp/formatter.go` — delete `trailingCommentLines` and its call site
- Test: `internal/lsp/formatter_test.go`

**Context:** `trailingCommentLines` scrapes trailing comments out of the source as text because the parser did not tokenize them. Task 4 removed that reason. It is also the one remaining path that renders a comment without going through the token walk, and it trims each line, so `   note [1]` came back as `note [1]`.

A previous attempt to delete this function truncated comment-only files to zero bytes, because the walker never saw the comment. Task 4 is what makes deletion safe. Verify that before deleting: confirm `TestParse_TrailingCommentsAreTokens` passes at your HEAD.

- [ ] **Step 1: Write the failing test**

```go
// internal/lsp/formatter_test.go

// Trailing comments go through the token walk like every other comment,
// which means their indentation survives.
func TestFormatDocument_TrailingCommentIndentation(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "comment only file",
			src:  "// just this\n",
			want: "// just this\n",
		},
		{
			name: "trailing comment after decl",
			src:  "domain re {\n  Billing\n}\n\n// trail\n",
			want: "domain re {\n  Billing\n}\n\n// trail\n",
		},
		{
			name: "indented trailing block comment",
			src:  "domain re {\n  Billing\n}\n\n/* a\n   b */\n",
			want: "domain re {\n  Billing\n}\n\n/* a\n   b */\n",
		},
	}
	for _, c := range cases {
		got := FormatDocument(c.src)
		if got != c.want {
			t.Errorf("%s:\n got: %q\nwant: %q", c.name, got, c.want)
		}
	}
}
```

`internal/lsp/formatter_test.go` is `package lsp`, so call `FormatDocument(content string) string` unqualified. Interior indentation of the block comment must survive verbatim: that is the point of the test.

- [ ] **Step 2: Run it**

Run: `go test ./internal/lsp/ -run TestFormatDocument_TrailingCommentIndentation -v`
Expected: FAIL on the indented block comment, which currently loses its leading spaces.

- [ ] **Step 3: Delete the function and its call site**

Remove `trailingCommentLines` and wherever it is invoked. Do not replace it with anything — the walker already emits these tokens after Task 4.

- [ ] **Step 4: Verify nothing regressed**

Run: `go test ./internal/lsp/ -v 2>&1 | tail -30`
Expected: PASS, including the corpus test. Pay particular attention to `TestFormatDocument_EveryCraftFileInRepo` — it asserts content preservation, reparse-clean, idempotence, and model preservation over every `.craft` file.

Then confirm the canonical count has not regressed:

Run: `go test ./internal/lsp/ -run TestFormatDocument_EveryCraftFileInRepo -v 2>&1 | grep -i "canonical form"`
Expected: >= 44. The count is logged by `formatter_corpus_test.go:234`, and `-v` is required for `t.Logf` output to appear.

- [ ] **Step 5: Commit**

```bash
git add internal/lsp/formatter.go internal/lsp/formatter_test.go
git commit -m "refactor(lsp): delete trailingCommentLines

It existed only because the parser folded trailing comments into a
whitespace token. It was also the last path that rendered a comment
without going through the token walk, which is why it stripped
interior indentation."
```

---

### Task 6: Alignment tracks real token ends

**Files:**
- Modify: `internal/lsp/formatter.go` (`writeTokens`, the interior-line bookkeeping) and `internal/lsp/formatalign.go` if the interior map's shape changes
- Test: `internal/lsp/formatter_test.go`

**Context:** `writeTokens` returns a set of "interior" line indices that `alignAnnotations` must skip, so that lines inside a multi-line comment are not mistaken for alignable action lines. It currently marks every emitted line after a token's first as interior to that token. That over-claims the token's *last* line, which the token shares with whatever follows it.

Concretely: when a block comment's closing `*/` sits on the same line as a following annotated action, that whole physical line is excluded from alignment, so the annotation keeps whatever spacing the author wrote while its siblings align to a column.

The fix is to stop treating "line index after the token's first" as a proxy for "inside the token". A line is interior only if the token still covers the start of the alignable content on that line. Track the token's actual end position when writing it, and mark as interior only the lines strictly between its first and last.

- [ ] **Step 1: Write the failing test**

```go
// internal/lsp/formatter_test.go

// A multi-line comment whose */ shares a line with an annotated action must
// not exclude that line from alignment: the comment ends before the action
// begins, so the action is alignable like any other.
func TestFormatDocument_AlignsAfterMultilineCommentClose(t *testing.T) {
	src := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B to aaa  [POST /v1/a]\n" +
		"    /* note\n" +
		"       more */ A asks B to b  [POST /v1/b]\n" +
		"}\n"
	got := FormatDocument(src)

	// The two annotations must start at the same column.
	var cols []int
	for _, line := range strings.Split(got, "\n") {
		if i := strings.Index(line, "[POST"); i >= 0 {
			cols = append(cols, i)
		}
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 annotations, found %d in:\n%s", len(cols), got)
	}
	if cols[0] != cols[1] {
		t.Errorf("annotations not aligned: columns %d and %d in:\n%s", cols[0], cols[1], got)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/lsp/ -run TestFormatDocument_AlignsAfterMultilineCommentClose -v`
Expected: FAIL — the two columns differ, because the `*/` line is excluded from the alignment run.

- [ ] **Step 3: Track real token ends**

Change the interior bookkeeping in `writeTokens` so a token marks only the lines strictly between its first and last emitted line, not every line after its first. Read the existing implementation and make the minimal change that achieves this; do not restructure the walker.

Verify the reverse case still holds: a line genuinely inside a multi-line comment must stay excluded, or alignment will corrupt comment bodies. Add that as a second case in the same test if it is not already covered elsewhere.

- [ ] **Step 4: Verify**

Run: `go test ./internal/lsp/ -v 2>&1 | tail -30`
Expected: PASS, corpus test included, canonical count >= 44, goldens unchanged.

- [ ] **Step 5: Mutation-check the fix**

Revert only the Step 3 change (keep the test), re-run `TestFormatDocument_AlignsAfterMultilineCommentClose`, and confirm it fails. Restore the fix. Report both outcomes. A test that passes both with and without the fix is not testing the fix.

- [ ] **Step 6: Commit**

```bash
git add internal/lsp/formatter.go internal/lsp/formatalign.go internal/lsp/formatter_test.go
git commit -m "fix(lsp): mark only lines strictly inside a token as interior

Marking every line after a token's first over-claimed the token's last
line, which it shares with whatever follows. A block comment closing on
an annotated action's line excluded that action from alignment."
```

---

### Task 7: Update the design records and changelog

**Files:**
- Modify: `docs/decisions/token-stream-formatter.md` (the "Known limitations" section, lines 302-323)
- Modify: `CHANGELOG.md` (the `[Unreleased]` entry)

**Context:** All three limitations recorded at the end of `token-stream-formatter.md` are now fixed. Replace that section with a pointer to `docs/decisions/lossless-token-text.md` rather than deleting it silently, so the history stays legible.

- [ ] **Step 1: Rewrite the limitations section**

Replace lines 302-323 with a short section stating that all three were fixed, naming `docs/decisions/lossless-token-text.md`, and noting that `contentDrift` is now belt-and-braces rather than load-bearing because the property it defended is guaranteed upstream.

- [ ] **Step 2: Add the changelog entries**

Under `[Unreleased]`, add entries for: the lexer's byte-end field, token text becoming a source slice, the text-equality invariant, trailing comments being tokenized, and the two formatter fixes. Note as a behaviour change that the tree now carries malformed source faithfully — an unterminated string keeps its opening quote where the parser previously repaired it.

No em dashes.

- [ ] **Step 3: Verify and commit**

Run: `go test ./... 2>&1 | grep -v '^ok' | head`
Expected: no failures.

```bash
git add docs/decisions/token-stream-formatter.md CHANGELOG.md
git commit -m "docs: record that the three formatter limitations are fixed"
```
