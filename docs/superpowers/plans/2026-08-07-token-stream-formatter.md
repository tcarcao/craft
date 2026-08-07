# Token-Stream Formatter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the formatter's per-construct branches with a single walk over the lossless token stream, so "formatting changes whitespace and nothing else" is true by construction.

**Architecture:** One loop over `root.AllTokens()`. Every non-whitespace token is written verbatim, exactly once, in document order. The only decision is the separator string emitted before each token, computed by a pure function from the preceding whitespace gap, the two adjacent token kinds, and the brace depth. Annotation alignment is a separate post-pass over the emitted lines.

**Tech Stack:** Go 1.23, `internal/lsp` (formatter), `internal/syntax` (rowan-style lossless green tree).

**Spec:** `docs/decisions/token-stream-formatter.md`

## Global Constraints

- Go 1.23. Module `github.com/tcarcao/craft/v2`. **Do not change the module path.**
- The suite is green at **996 passing**. It must be green at or above that when finished.
- The green tree is lossless: `root.Width() == len(src)` is asserted. `TestRoundTrip` and `lossless_check_test.go` must stay green.
- **No em dashes** in code, comments, test names, docs prose, or commit messages.
- Corpus `.craftjson` goldens must not drift. A golden change means model drift, not whitespace drift, and is a defect.
- Leave no scratch or probe files. `git status --short` clean before each commit.
- The wrapped `diff` command returns exit 0 even when files differ. Use `rtk proxy diff` or checksums when branching on exit status.

## Key facts the implementer needs

**Whitespace is a real token.** `internal/syntax/parser.go:1707` emits `SyntaxKindWhitespace` carrying the exact source gap. `emitWhitespaceBefore` only emits when there is a gap, so **adjacent tokens have no whitespace token between them**.

That single fact does most of the work:

- `re/billing` has no gaps, so the separator is `""` and it emits verbatim. **No Ref special case is needed.**
- `bc:re/billing` likewise.
- `contexts: A` has a one-space gap, so the author's space is seen directly.

**Comments are real tokens.** `parser.go:1742` emits `SyntaxKindLineComment` in stream. Walking `AllTokens()` sees them without any filtering, which is why `significantTokens` and `trailingCommentLines` both disappear.

**A comment is trailing if its preceding whitespace gap contains no newline.** That is the whole mechanism for keeping trailing comments in place.

**`arch` blocks stay verbatim.** `FormatDocumentChecked` currently slices arch declarations straight out of the source because arch component chains use free-form indentation. Keep that branch exactly as it is. The walker must not reformat arch.

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/lsp/formatsep.go` | Create | `separatorFor`, the pure separator function, and its indent helper |
| `internal/lsp/formatsep_test.go` | Create | Table-driven unit tests for every separator rule |
| `internal/lsp/formatter.go` | Modify | Replace the reconstructive body with the walker; delete ~300 lines |
| `internal/lsp/formatalign.go` | Create | Annotation alignment as a post-pass over emitted lines |
| `internal/lsp/formatalign_test.go` | Create | Alignment tests, ported from the existing ones |
| `internal/lsp/formatter_test.go` | Modify | Re-bless fixtures affected by the five behaviour changes |
| `internal/lsp/formatter_corpus_test.go` | Modify | Delete `commentTexts`; add the contentDrift-never-fires assertion |
| `docs/decisions/token-stream-formatter.md` | Modify | Mark Status: Implemented |
| `CHANGELOG.md` | Modify | Entry for the next release |

Splitting the separator function and the alignment pass into their own files is deliberate: each is independently testable, and `formatter.go` is currently 626 lines doing four jobs.

---

### Task 1: The separator function

**Files:**
- Create: `internal/lsp/formatsep.go`
- Test: `internal/lsp/formatsep_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `separatorFor(prev *syntax.SyntaxToken, gap string, curr syntax.SyntaxToken, depth int) string` and `indentFor(depth int) string`

This is the whole formatting policy in one pure function. Everything else is plumbing.

- [ ] **Step 1: Write the failing test**

Create `internal/lsp/formatsep_test.go`:

```go
package lsp

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// tok builds a bare token of the given kind and text for separator tests.
// separatorFor only reads Kind(), Text() and Parent(), and a token built by
// the parser carries a real parent, so these cases cover everything except the
// ref-colon rule, which has its own parse-backed test below.
func sepTok(t *testing.T, src string, idx int) syntax.SyntaxToken {
	t.Helper()
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	var out []syntax.SyntaxToken
	for _, tk := range toks {
		if tk.Kind() != syntax.SyntaxKindWhitespace {
			out = append(out, tk)
		}
	}
	if idx >= len(out) {
		t.Fatalf("token index %d out of range (%d tokens) for %q", idx, len(out), src)
	}
	return out[idx]
}

func TestSeparatorFor_FirstTokenHasNoSeparator(t *testing.T) {
	curr := sepTok(t, "domain re {\n  Billing\n}\n", 0)
	if got := separatorFor(nil, "", curr, 0); got != "" {
		t.Errorf("separatorFor(nil, ...) = %q, want empty", got)
	}
}

func TestSeparatorFor_GapWithNewlineBreaksTheLine(t *testing.T) {
	src := "domain re {\n  Billing\n}\n"
	prev := sepTok(t, src, 2) // {
	curr := sepTok(t, src, 3) // Billing
	if got := separatorFor(&prev, "\n  ", curr, 1); got != "\n  " {
		t.Errorf("got %q, want %q", got, "\n  ")
	}
}

func TestSeparatorFor_BlankLineRunCollapsesToOne(t *testing.T) {
	src := "domain re {\n  Billing\n}\n"
	prev := sepTok(t, src, 3)
	curr := sepTok(t, src, 4)
	if got := separatorFor(&prev, "\n\n\n\n", curr, 1); got != "\n\n  " {
		t.Errorf("got %q, want one blank line then indent", got)
	}
}

func TestSeparatorFor_SameLineRunOfSpacesCollapsesToOne(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n}\n"
	prev := sepTok(t, src, 4)
	curr := sepTok(t, src, 5)
	if got := separatorFor(&prev, "     ", curr, 2); got != " " {
		t.Errorf("got %q, want a single space", got)
	}
}

func TestSeparatorFor_AdjacentTokensStayAdjacent(t *testing.T) {
	// `re/billing` has no gaps, so its parts must concatenate. This is what
	// removes the need for any Ref special case.
	src := "use_case \"X\" {\n  when U does x\n    A asks re/billing for c\n}\n"
	gn, _, _ := syntax.Parse(src)
	var prev, curr syntax.SyntaxToken
	var found bool
	toks := syntax.Root(gn).AllTokens()
	for i := 1; i < len(toks); i++ {
		if toks[i].Kind() == syntax.SyntaxKindWhitespace {
			continue
		}
		if toks[i].Text() == "/" {
			prev, curr, found = toks[i-1], toks[i], true
			break
		}
	}
	if !found {
		t.Fatal("no / token found in a qualified ref")
	}
	if got := separatorFor(&prev, "", curr, 2); got != "" {
		t.Errorf("got %q, want empty so re/billing stays joined", got)
	}
}

func TestSeparatorFor_NoSpaceBeforeColonOrComma(t *testing.T) {
	src := "services {\n  S {\n    contexts: A, B\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	for _, want := range []string{":", ","} {
		toks := syntax.Root(gn).AllTokens()
		for i := 1; i < len(toks); i++ {
			if toks[i].Text() != want {
				continue
			}
			prev := toks[i-1]
			if got := separatorFor(&prev, " ", toks[i], 2); got != "" {
				t.Errorf("before %q: got %q, want empty", want, got)
			}
			break
		}
	}
}

func TestSeparatorFor_FieldColonGetsOneSpaceAfter(t *testing.T) {
	// `contexts:A` must normalise to `contexts: A`, so the rule cannot be
	// gap-driven alone.
	src := "services {\n  S {\n    contexts: A\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()
	for i := 0; i < len(toks)-1; i++ {
		if toks[i].Kind() != syntax.SyntaxKindColon {
			continue
		}
		var next syntax.SyntaxToken
		for j := i + 1; j < len(toks); j++ {
			if toks[j].Kind() != syntax.SyntaxKindWhitespace {
				next = toks[j]
				break
			}
		}
		if got := separatorFor(&toks[i], "", next, 2); got != " " {
			t.Errorf("after a field colon: got %q, want one space", got)
		}
		return
	}
	t.Fatal("no colon found")
}

// TestSeparatorFor_ScenarioAlwaysGetsABlankLine pins the one rule where the
// formatter adds vertical space rather than preserving the author's. A `when`
// straight after `{` opens the body and must not get one.
func TestSeparatorFor_ScenarioAlwaysGetsABlankLine(t *testing.T) {
	src := "use_case \"X\" {\n  when A does x\n    P does y\n  when B does z\n    Q does w\n}\n"
	gn, _, _ := syntax.Parse(src)
	toks := syntax.Root(gn).AllTokens()

	var firstWhen, secondWhen int = -1, -1
	for i, tk := range toks {
		if tk.Kind() != syntax.SyntaxKindKwWhen {
			continue
		}
		if firstWhen < 0 {
			firstWhen = i
		} else if secondWhen < 0 {
			secondWhen = i
		}
	}
	if firstWhen < 0 || secondWhen < 0 {
		t.Fatal("expected two when tokens")
	}

	prevOfFirst := prevSignificant(toks, firstWhen)
	if got := separatorFor(prevOfFirst, "\n  ", toks[firstWhen], 1); got != "\n  " {
		t.Errorf("first when: got %q, want a plain newline (it opens the body)", got)
	}

	prevOfSecond := prevSignificant(toks, secondWhen)
	if got := separatorFor(prevOfSecond, "\n  ", toks[secondWhen], 1); got != "\n\n  " {
		t.Errorf("second when: got %q, want a blank line before the scenario", got)
	}
}

// prevSignificant returns the nearest non-whitespace token before i, or nil.
func prevSignificant(toks []syntax.SyntaxToken, i int) *syntax.SyntaxToken {
	for j := i - 1; j >= 0; j-- {
		if toks[j].Kind() == syntax.SyntaxKindWhitespace {
			continue
		}
		t := toks[j]
		return &t
	}
	return nil
}

func TestIndentFor(t *testing.T) {
	for depth, want := range map[int]string{0: "", 1: "  ", 3: "      "} {
		if got := indentFor(depth); got != want {
			t.Errorf("indentFor(%d) = %q, want %q", depth, got, want)
		}
	}
	if got := indentFor(-1); got != "" {
		t.Errorf("indentFor(-1) = %q, want empty (must not panic)", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lsp/ -run 'TestSeparatorFor|TestIndentFor' -v`
Expected: FAIL, `undefined: separatorFor`, `undefined: indentFor`

- [ ] **Step 3: Write the implementation**

Create `internal/lsp/formatsep.go`:

```go
package lsp

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// indentFor returns the indentation for a brace depth. It floors at zero: an
// unbalanced `}` only produces a warning-severity diagnostic, so it can reach
// the formatter with depth already at zero, and strings.Repeat panics on a
// negative count.
func indentFor(depth int) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

// separatorFor returns the whitespace to emit before curr.
//
// This is the entire formatting policy. Every other part of the formatter
// writes token text verbatim, so this function decides everything the
// formatter is allowed to decide.
//
// gap is the raw text of the whitespace token between prev and curr, or "" when
// they were adjacent in the source. Reading the author's gap rather than
// re-deriving line structure from the tree is what preserves author line breaks,
// and it is also what keeps a qualified ref joined: `re/billing` has no gaps, so
// every separator inside it is empty and the parts concatenate.
func separatorFor(prev *syntax.SyntaxToken, gap string, curr syntax.SyntaxToken, depth int) string {
	if prev == nil {
		return ""
	}

	// Never a space before a colon or comma, whatever the author wrote.
	if curr.Kind() == syntax.SyntaxKindColon || curr.Kind() == syntax.SyntaxKindComma {
		return ""
	}

	// One space after a field colon, even if the author wrote none, so
	// `contexts:A` normalises to `contexts: A`. A colon inside a ref
	// (`bc:re/billing`) is not a field colon and must stay tight, which the
	// parent check distinguishes.
	if prev.Kind() == syntax.SyntaxKindColon && !isRefColon(*prev) {
		return " "
	}
	if prev.Kind() == syntax.SyntaxKindComma {
		return " "
	}

	newlines := strings.Count(gap, "\n")

	// A scenario always gets a blank line before it, even if the author wrote
	// none. This is the one place the formatter adds vertical space rather than
	// preserving it, and it matches what the previous formatter did. `when` at
	// the very start of a use_case body is not a new scenario, and prev being
	// `{` is how that case is recognised.
	if curr.Kind() == syntax.SyntaxKindKwWhen && depth == 1 && prev.Kind() != syntax.SyntaxKindLBrace {
		return "\n\n" + indentFor(depth)
	}

	switch newlines {
	case 0:
		if gap == "" {
			return ""
		}
		// Collapse any run of spaces or tabs to one.
		return " "
	case 1:
		return "\n" + indentFor(depth)
	default:
		// Two or more newlines is a blank line. Collapse any longer run to one.
		return "\n\n" + indentFor(depth)
	}
}

// isRefColon reports whether tok is the `:` inside a node slug such as
// `bc:re/billing`, as opposed to a field separator such as `contexts:`.
func isRefColon(tok syntax.SyntaxToken) bool {
	parent := tok.Parent()
	return parent != nil && parent.Kind() == syntax.SyntaxKindRef
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/lsp/ -run 'TestSeparatorFor|TestIndentFor' -v`
Expected: PASS

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS at 996 or higher. Nothing consumes `separatorFor` yet, so this only proves the new file compiles and breaks nothing.

- [ ] **Step 6: Commit**

```bash
git add internal/lsp/formatsep.go internal/lsp/formatsep_test.go
git commit -m "feat(fmt): add the token separator function"
```

---

### Task 2: The walker

**Files:**
- Modify: `internal/lsp/formatter.go`
- Test: `internal/lsp/formatter_test.go`

**Interfaces:**
- Consumes: `separatorFor`, `indentFor`, `isRefColon` from Task 1
- Produces: `writeTokens(sb *strings.Builder, node syntax.SyntaxNode)`

This replaces `formatDecl`, `tokenSeparator`, `isNextColon`, `isSiblingToken`, `significantTokens`, `writeRefWithComments`, `writeBlockStatements`, `formatUseCaseDecl`, `formatContextMapDecl`, and `formatGlossaryDecl` with one loop.

- [ ] **Step 1: Write the failing test**

Append to `internal/lsp/formatter_test.go`:

```go
// TestWriteTokens_PreservesEveryNonWhitespaceToken is the invariant this whole
// rewrite exists to make structural: the walker emits every non-whitespace
// token verbatim, exactly once, in order.
func TestWriteTokens_PreservesEveryNonWhitespaceToken(t *testing.T) {
	srcs := []string{
		"domain re {\n  Billing\n}\n",
		"services {\n  S {\n    contexts: A, B\n    repo: olxeu/realestate/subs\n  }\n}\n",
		"context_map {\n  re/billing customer_supplier re/vas\n}\n",
		"glossary re {\n  billing/Invoice same_as subs/Invoice\n}\n",
		"use_case \"X\" {\n  when U does x\n    A asks re/b for c  [POST /v1/x]\n}\n",
	}
	for _, src := range srcs {
		gn, _, _ := syntax.Parse(src)
		root := syntax.Root(gn)

		var want []string
		for _, tk := range root.AllTokens() {
			if tk.Kind() == syntax.SyntaxKindWhitespace || tk.Kind() == syntax.SyntaxKindEOF {
				continue
			}
			want = append(want, tk.Text())
		}

		var sb strings.Builder
		for el := range root.ChildrenIter() {
			if node, ok := el.(syntax.SyntaxNode); ok {
				writeTokens(&sb, node)
			}
		}

		var got []string
		outGn, _, _ := syntax.Parse(sb.String())
		for _, tk := range syntax.Root(outGn).AllTokens() {
			if tk.Kind() == syntax.SyntaxKindWhitespace || tk.Kind() == syntax.SyntaxKindEOF {
				continue
			}
			got = append(got, tk.Text())
		}

		if !reflect.DeepEqual(want, got) {
			t.Errorf("token stream changed for %q\nwant %v\ngot  %v", src, want, got)
		}
	}
}

// TestWriteTokens_KeepsAuthorLineBreaks pins the fifth behaviour change: a
// value the author wrapped across lines stays wrapped.
func TestWriteTokens_KeepsAuthorLineBreaks(t *testing.T) {
	src := "services {\n  S {\n    contexts: A,\n      B\n  }\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	if !strings.Contains(sb.String(), "A,\n") {
		t.Errorf("author line break inside contexts was joined:\n%s", sb.String())
	}
}

// TestWriteTokens_KeepsTrailingCommentsInPlace pins the first behaviour change.
func TestWriteTokens_KeepsTrailingCommentsInPlace(t *testing.T) {
	src := "use_case \"X\" {\n  when U does x\n    A notifies a.B  // note\n}\n"
	gn, _, _ := syntax.Parse(src)
	var sb strings.Builder
	for el := range syntax.Root(gn).ChildrenIter() {
		if node, ok := el.(syntax.SyntaxNode); ok {
			writeTokens(&sb, node)
		}
	}
	out := sb.String()
	if !strings.Contains(out, "a.B  // note") && !strings.Contains(out, "a.B // note") {
		t.Errorf("trailing comment did not stay on its line:\n%s", out)
	}
}
```

Ensure `reflect` and `strings` are imported in that file.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lsp/ -run TestWriteTokens -v`
Expected: FAIL, `undefined: writeTokens`

- [ ] **Step 3: Write the walker**

Add to `internal/lsp/formatter.go`:

```go
// writeTokens renders one top-level declaration by walking its token stream.
//
// Every non-whitespace token is written verbatim, exactly once, in document
// order. The only decision is the separator before each, which separatorFor
// makes. There are deliberately no per-construct branches: a construct the
// formatter has never seen still round-trips, because nothing here inspects
// what a token means.
func writeTokens(sb *strings.Builder, node syntax.SyntaxNode) {
	depth := 0
	gap := ""
	var prev *syntax.SyntaxToken

	for _, tok := range node.AllTokens() {
		if tok.Kind() == syntax.SyntaxKindEOF {
			continue
		}
		if tok.Kind() == syntax.SyntaxKindWhitespace {
			gap += tok.Text()
			continue
		}

		// `}` closes its block, so it dedents before it is placed.
		if tok.Kind() == syntax.SyntaxKindRBrace && depth > 0 {
			depth--
		}

		sb.WriteString(separatorFor(prev, gap, tok, depth))
		sb.WriteString(tok.Text())

		if tok.Kind() == syntax.SyntaxKindLBrace {
			depth++
		}

		cur := tok
		prev = &cur
		gap = ""
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/lsp/ -run TestWriteTokens -v`
Expected: PASS on all three

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: PASS. `writeTokens` is not wired in yet, so existing formatter behaviour is unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/lsp/formatter.go internal/lsp/formatter_test.go
git commit -m "feat(fmt): add the token-stream walker"
```

---

### Task 3: Wire the walker in and delete the reconstructive code

**Files:**
- Modify: `internal/lsp/formatter.go`
- Modify: `internal/lsp/formatter_test.go`

**Interfaces:**
- Consumes: `writeTokens` from Task 2
- Produces: `FormatDocumentChecked` routing every declaration except `arch` through `writeTokens`

This is where behaviour changes land and fixtures churn. Take it carefully.

- [ ] **Step 1: Replace the dispatch**

In `FormatDocumentChecked`, replace the `switch node.Kind()` block with:

```go
		switch node.Kind() {
		case syntax.SyntaxKindArchDecl:
			// Preserve original formatting: arch component chains use free-form
			// indentation that the formatter does not rewrite. This is the one
			// construct the walker must not touch.
			r := node.TextRange()
			sb.WriteString(strings.TrimSpace(content[r.Start:r.End]))
		default:
			writeTokens(&sb, node)
		}
```

Also delete the `if tok, isTok := el.(syntax.SyntaxToken); isTok && isCommentKind(tok.Kind())` branch and the `trailingCommentLines` call. Root-level comments and end-of-file comments are ordinary tokens now, but note they are children of the root rather than of any declaration, so the loop must emit them too. Replace the non-node branch with:

```go
		if !ok {
			if tok, isTok := el.(syntax.SyntaxToken); isTok && tok.Kind() != syntax.SyntaxKindWhitespace && tok.Kind() != syntax.SyntaxKindEOF {
				if !first {
					sb.WriteString("\n\n")
				}
				first = false
				sb.WriteString(tok.Text())
			}
			continue
		}
```

- [ ] **Step 2: Delete the dead code**

Delete from `internal/lsp/formatter.go`: `formatUseCaseDecl`, `formatContextMapDecl`, `formatGlossaryDecl`, `formatDecl`, `tokenSeparator`, `isNextColon`, `isSiblingToken`, `writeBlockStatements`, `writeRefWithComments`, `writeCommentLines`, `significantTokens`, `isCommentKind`, `trailingCommentLines`.

Keep `contentDrift`, `squashWhitespace`, `bailsFormatting`, and `writeAlignedActions` for now. Task 4 moves alignment.

- [ ] **Step 3: Run the corpus guard**

Run: `go test ./internal/lsp/ -run TestFormatDocument -v 2>&1 | tail -40`
Expected: the content-preservation, reparse, idempotence and model assertions all PASS. Byte-identity assertions will FAIL for fixtures affected by the five behaviour changes. That is expected at this step; do not weaken an assertion to make it pass.

- [ ] **Step 4: Re-bless the affected fixtures, one reviewed diff at a time**

For each failing byte-identity case, look at the actual difference and confirm it is one of the five approved changes:

1. a trailing comment stayed on its line rather than moving above
2. a ref-adjacent comment no longer splits its field across two lines
3. interior multi-space collapsed on a trigger line the same way it does on an action line
4. blank-line handling
5. an author line break inside a value was preserved rather than joined

If a difference is **none** of those five, stop and report it. That is a bug in the walker, not a fixture that needs re-blessing.

Update the fixture's expected string to the new output only after confirming which of the five it is, and note that in your report.

- [ ] **Step 5: Confirm no golden drifted**

```bash
go test ./internal/parser_diff/ ./pkg/craft/ -count=1
rtk proxy git status --short
```
Expected: both packages PASS, and no `.craftjson` file is modified. A golden change here means model drift and is a defect, not a re-blessing.

- [ ] **Step 6: Confirm contentDrift never fires**

```bash
go test ./internal/lsp/ -run TestFormatDocumentCorpus -count=1 -v 2>&1 | rtk proxy grep -i "drift" | head
```
Expected: no drift reported for any corpus file. If `contentDrift` fires, the walker is losing content and the invariant is not yet true by construction.

- [ ] **Step 7: Run the full suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/lsp/formatter.go internal/lsp/formatter_test.go
git commit -m "refactor(fmt): route every declaration through the token walker"
```

---

### Task 4: Move alignment to a post-pass

**Files:**
- Create: `internal/lsp/formatalign.go`
- Create: `internal/lsp/formatalign_test.go`
- Modify: `internal/lsp/formatter.go`

**Interfaces:**
- Consumes: nothing from earlier tasks
- Produces: `alignAnnotations(s string) string`

Alignment currently lives in `writeAlignedActions`, which took a `[]syntax.ActionDecl`. The walker has no action list, so alignment becomes a pass over the emitted text. It only ever rewrites the run of spaces before a `[`, so it cannot affect content.

- [ ] **Step 1: Write the failing test**

Create `internal/lsp/formatalign_test.go`:

```go
package lsp

import "testing"

func TestAlignAnnotations_AlignsAContiguousRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    Subscriptions asks Billing for a charge [POST /v1/charges]\n" +
		"    Billing asks Gateway to authorize [POST /pay/v2/authorize]\n" +
		"}\n"
	want := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    Subscriptions asks Billing for a charge  [POST /v1/charges]\n" +
		"    Billing asks Gateway to authorize        [POST /pay/v2/authorize]\n" +
		"}\n"
	if got := alignAnnotations(in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestAlignAnnotations_IsIdempotent(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    LongerSubject asks B for c [GET /v1/y]\n" +
		"}\n"
	once := alignAnnotations(in)
	if twice := alignAnnotations(once); once != twice {
		t.Errorf("not idempotent\nonce:\n%s\ntwice:\n%s", once, twice)
	}
}

func TestAlignAnnotations_NonAnnotatedLineDoesNotBreakTheRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when U does x\n" +
		"    A asks B for c [POST /v1/x]\n" +
		"    A asks C to do the thing\n" +
		"    A asks D for e [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in)
	if !strings.Contains(got, "for c   [POST /v1/x]") || !strings.Contains(got, "for e   [GET /v1/y]") {
		t.Errorf("run was broken by the unannotated line:\n%s", got)
	}
}

func TestAlignAnnotations_BlankLineResetsTheRun(t *testing.T) {
	in := "use_case \"X\" {\n" +
		"  when A does x\n" +
		"    VeryLongSubjectHere asks B for c [POST /v1/x]\n" +
		"\n" +
		"  when B does y\n" +
		"    A asks C for d [GET /v1/y]\n" +
		"}\n"
	got := alignAnnotations(in)
	if !strings.Contains(got, "for d  [GET /v1/y]") {
		t.Errorf("second run should align independently:\n%s", got)
	}
}

func TestAlignAnnotations_LeavesTextWithoutAnnotationsAlone(t *testing.T) {
	in := "domain re {\n  Billing\n}\n"
	if got := alignAnnotations(in); got != in {
		t.Errorf("unannotated text changed:\ngot:  %q\nwant: %q", got, in)
	}
}
```

Import `strings` and `testing` in this file.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/lsp/ -run TestAlignAnnotations -v`
Expected: FAIL, `undefined: alignAnnotations`

- [ ] **Step 3: Write the implementation**

Create `internal/lsp/formatalign.go`:

```go
package lsp

import (
	"strings"
	"unicode/utf8"
)

// alignAnnotations column-aligns trailing operation annotations.
//
// It runs over the formatter's output rather than over the tree, because
// alignment is the one decision that needs to see whole lines. It only ever
// rewrites the run of spaces before a `[`, so it cannot change content.
//
// A run is a stretch of consecutive lines that carry an annotation, plus any
// unannotated lines between them. A blank line ends a run. That matches the
// worked examples in docs/decisions/action-operation-brackets.md, where an
// unannotated action keeps the column rather than splitting it.
func alignAnnotations(s string) string {
	lines := strings.Split(s, "\n")

	runStart := -1
	flush := func(end int) {
		if runStart < 0 {
			return
		}
		col := 0
		for i := runStart; i < end; i++ {
			if body, _, ok := splitAnnotation(lines[i]); ok {
				if w := utf8.RuneCountInString(body); w+2 > col {
					col = w + 2
				}
			}
		}
		for i := runStart; i < end; i++ {
			body, ann, ok := splitAnnotation(lines[i])
			if !ok {
				continue
			}
			pad := col - utf8.RuneCountInString(body)
			if pad < 1 {
				pad = 1
			}
			lines[i] = body + strings.Repeat(" ", pad) + ann
		}
		runStart = -1
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush(i)
			continue
		}
		if _, _, ok := splitAnnotation(line); ok && runStart < 0 {
			runStart = i
		}
	}
	flush(len(lines))

	return strings.Join(lines, "\n")
}

// splitAnnotation splits a line into the text before its trailing operation
// annotation and the annotation itself. ok is false when the line does not end
// in one.
//
// The boundary matches the parser's: the annotation is the last `[` on the line
// whose `]` is the line's final character. A `[` that does not close at end of
// line is prose, and this must leave it alone.
func splitAnnotation(line string) (body, ann string, ok bool) {
	trimmed := strings.TrimRight(line, " \t")
	if !strings.HasSuffix(trimmed, "]") {
		return "", "", false
	}
	open := strings.LastIndex(trimmed, "[")
	if open <= 0 {
		return "", "", false
	}
	body = strings.TrimRight(trimmed[:open], " \t")
	if body == "" {
		return "", "", false
	}
	return body, trimmed[open:], true
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/lsp/ -run TestAlignAnnotations -v`
Expected: PASS

- [ ] **Step 5: Wire it in and delete the old alignment**

In `FormatDocumentChecked`, apply the pass to the assembled output before the drift check:

```go
	formatted := alignAnnotations(sb.String())
	if drift := contentDrift(content, formatted); drift != nil {
```

Delete `writeAlignedActions` from `internal/lsp/formatter.go`.

- [ ] **Step 6: Run the full suite**

Run: `go test ./... -count=1`
Expected: PASS. If an alignment fixture differs, check it against the run rules above before re-blessing.

- [ ] **Step 7: Commit**

```bash
git add internal/lsp/formatalign.go internal/lsp/formatalign_test.go internal/lsp/formatter.go
git commit -m "refactor(fmt): align annotations as a post-pass over emitted lines"
```

---

### Task 5: Tighten the corpus guard

**Files:**
- Modify: `internal/lsp/formatter_corpus_test.go`

**Interfaces:**
- Consumes: the walker from Tasks 2 and 3
- Produces: a guard whose comment assertion is text-based, plus a contentDrift assertion

- [ ] **Step 1: Delete `commentTexts` and assert on text instead**

`commentTexts` filters on token kind, which is why it could not see a trailing comment living inside the final whitespace trivia. Comments are ordinary tokens in the walk now, so the content-preservation assertion already covers them. Delete `commentTexts` and any assertion that used it.

- [ ] **Step 2: Write the failing test**

Add to `internal/lsp/formatter_corpus_test.go`:

```go
// TestFormatDocumentCorpus_ContentDriftNeverFires is the proof that the
// invariant is structural rather than merely observed. contentDrift is the
// runtime net that returns the input unchanged when formatting would alter a
// non-whitespace byte. Under a token-stream walker it should be unreachable,
// so if it ever fires we want to know immediately rather than discover a file
// silently went unformatted.
func TestFormatDocumentCorpus_ContentDriftNeverFires(t *testing.T) {
	for _, path := range craftFilesUnderTest(t) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		_, diag := FormatDocumentChecked(string(src))
		if diag != nil && diag.Code == "craft/internal/formatter-content-drift" {
			t.Errorf("%s: contentDrift fired, so the walker lost content: %s", path, diag.Message)
		}
	}
}
```

Use whatever helper the file already has for enumerating corpus files; if it is not a named function, extract one called `craftFilesUnderTest` and use it in both places rather than duplicating the walk.

- [ ] **Step 3: Run it**

Run: `go test ./internal/lsp/ -run TestFormatDocumentCorpus -v -count=1`
Expected: PASS. A failure names the file the walker is losing content on.

- [ ] **Step 4: Run the full suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/lsp/formatter_corpus_test.go
git commit -m "test(fmt): assert contentDrift is unreachable, drop kind-based comment check"
```

---

### Task 6: Documentation

**Files:**
- Modify: `docs/decisions/token-stream-formatter.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Mark the decision record implemented**

Change its `**Status:**` line from `Accepted, not yet implemented` to `Implemented`, and add the commit range.

Also update `docs/decisions/action-operation-brackets.md`: its "Follow-up: the formatter is reconstructive, and should not be" section is now done. Replace the section body with a one-line pointer to `token-stream-formatter.md` rather than deleting it, so the history stays readable.

- [ ] **Step 2: Write the changelog entry**

Add an `### Changed` section to the unreleased entry at the top of `CHANGELOG.md`, in the house style (bold lead sentence, then prose). Cover the five behaviour changes:

1. trailing comments stay on their line instead of moving above
2. a comment between a field and its value no longer splits the field across two lines
3. interior runs of spaces collapse consistently on both action and trigger lines
4. blank-line rules are unchanged in effect but now stated and tested
5. line breaks the author wrote inside a value are preserved rather than joined

Then a `### Notes` bullet explaining that the formatter is now a single walk over the lossless token stream, so content preservation is structural rather than checked, and that `contentDrift` remains as a net that should now be unreachable.

Keep the `## [X.Y.Z] — DATE` heading's em dash to match every other heading. No other em dashes.

- [ ] **Step 3: Verify and commit**

```bash
go test ./... -count=1
rtk proxy git status --short
git add docs/decisions/ CHANGELOG.md
git commit -m "docs: record the token-stream formatter"
```

---

## Deferred, deliberately

- **Making `craft fmt --check` clean on the corpus.** 36 of 68 files report as unformatted, all non-canonical fixture shapes that are deliberate parser test surface, with zero formatter defects among them. Whether to re-canonicalise those fixtures is a separate decision. Note this rewrite may reduce the count on its own, since preserving author line breaks means fewer files differ from their formatted form; report the new number but do not chase it.
- **`arch` block formatting.** Still preserved verbatim by slicing source. Free-form component chains are out of scope.
- **Style changes** not among the five listed: indent width, comment reflowing, list wrapping.
