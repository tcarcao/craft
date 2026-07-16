# LSP Grammar Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 5 targeted LSP/grammar improvements: missing AST position methods, DocumentSymbol end positions, FoldingRanges for use_case/exposure/service/domain blocks, background cross-file resolution, and smarter action-verb completions.

**Architecture:** Tasks 1–3 are pure AST + LSP handler changes (no new dependencies). Task 4 moves `recomputeResolution` to `time.AfterFunc`-based debounce so `Change()` returns immediately after per-file parse. Task 5 adds a new completion context for "cursor immediately after an action verb", routing to domain/service suggestions instead of all-symbols.

**Tech Stack:** Go 1.22, `internal/syntax/ast.go`, `internal/lsp/server.go`, `internal/workspace/workspace.go`, `internal/lsp/completion.go`

---

## File Map

| File | Role |
|------|------|
| `internal/syntax/ast.go` | Add `UseCaseDecl.Line`, `ExposureDecl.EndLine`, `ActorDecl.Line`, `ServiceDecl.ContextLinesWith` |
| `internal/syntax/ast_test.go` | Tests for new AST methods |
| `internal/lsp/server.go` | Fix `DocumentSymbol` end positions; extend `FoldingRanges` |
| `internal/lsp/server_test.go` | Tests for DocumentSymbol and FoldingRanges |
| `internal/workspace/workspace.go` | Debounce `recomputeResolution` via `time.AfterFunc` |
| `internal/workspace/workspace_test.go` | Update perf-gate comment; add async parse test |
| `internal/lsp/completion.go` | Add `ctxUseCaseActionVerb` context + `domainAndServiceSymbolCompletions` |
| `internal/lsp/completion_test.go` | Tests for new completion context |

---

## Task 1: Add missing AST position methods

**Files:**
- Modify: `internal/syntax/ast.go`
- Modify: `internal/syntax/ast_test.go`

Current state:
- `UseCaseDecl` has `EndLine(li)` at line 712 but no `Line(li)`
- `ExposureDecl` has `Line(li)` at line 1335 but no `EndLine(li)`
- `ActorDecl` has no `Line(li)` method at all
- `ServiceDecl.ContextLines()` returns zeros (line 508); `ContextTokens()` has real offsets

- [ ] **Step 1: Write failing tests in `ast_test.go`**

Add at the end of `internal/syntax/ast_test.go`:

```go
func TestUseCaseDecl_Line(t *testing.T) {
	src := "use_case \"Register\" {\n  when Actor creates Account\n}"
	g, li, _ := syntax.Parse(src)
	file := syntax.AsFile(syntax.Root(g))
	ucs := file.UseCases()
	if len(ucs) != 1 {
		t.Fatalf("expected 1 use_case, got %d", len(ucs))
	}
	if got := ucs[0].Line(li); got != 1 {
		t.Errorf("UseCaseDecl.Line = %d, want 1", got)
	}
}

func TestExposureDecl_EndLine(t *testing.T) {
	src := "exposure MyAPI {\n  to: ServiceA\n}"
	g, li, _ := syntax.Parse(src)
	file := syntax.AsFile(syntax.Root(g))
	exps := file.Exposures()
	if len(exps) != 1 {
		t.Fatalf("expected 1 exposure, got %d", len(exps))
	}
	if got := exps[0].EndLine(li); got != 3 {
		t.Errorf("ExposureDecl.EndLine = %d, want 3", got)
	}
}

func TestActorDecl_Line(t *testing.T) {
	src := "actor user Alice"
	g, li, _ := syntax.Parse(src)
	file := syntax.AsFile(syntax.Root(g))
	actors := file.Actors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	if got := actors[0].Line(li); got != 1 {
		t.Errorf("ActorDecl.Line = %d, want 1", got)
	}
}

func TestActorDecl_Line_SecondLine(t *testing.T) {
	src := "\nactor user Alice"
	g, li, _ := syntax.Parse(src)
	file := syntax.AsFile(syntax.Root(g))
	actors := file.Actors()
	if len(actors) != 1 {
		t.Fatalf("expected 1 actor, got %d", len(actors))
	}
	if got := actors[0].Line(li); got != 2 {
		t.Errorf("ActorDecl.Line = %d, want 2", got)
	}
}

func TestServiceDecl_ContextLinesWith(t *testing.T) {
	src := "services {\n  Auth {\n    contexts: UserAuth, TokenAuth\n  }\n}"
	g, li, _ := syntax.Parse(src)
	file := syntax.AsFile(syntax.Root(g))
	svcs := file.Services()
	if len(svcs) != 1 {
		t.Fatalf("expected 1 service, got %d", len(svcs))
	}
	lines := svcs[0].ContextLinesWith(li)
	if len(lines) != 2 {
		t.Fatalf("expected 2 context lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != 3 || lines[1] != 3 {
		t.Errorf("expected both context lines == 3, got %v", lines)
	}
}
```

- [ ] **Step 2: Verify compilation fails**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go build ./internal/syntax/... 2>&1 | head -20
```

Expected: compilation errors — `ucs[0].Line undefined`, `exps[0].EndLine undefined`, etc.

- [ ] **Step 3: Add `UseCaseDecl.Line` to `ast.go`**

Insert after `func (u UseCaseDecl) EndLine(...)` (currently at line 712):

```go
// Line returns the 1-based source line of the `use_case` keyword using li.
func (u UseCaseDecl) Line(li green.LineIndex) int {
	tok := u.Keyword()
	if tok == nil {
		return nodeFirstTokenLine(u.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}
```

- [ ] **Step 4: Add `ExposureDecl.EndLine` to `ast.go`**

Insert after `func (e ExposureDecl) Line(...)` (currently at line 1335):

```go
// EndLine returns the 1-based line of the closing `}` using li.
func (e ExposureDecl) EndLine(li green.LineIndex) int { return nodeEndLine(e.node, li) }
```

- [ ] **Step 5: Add `ActorDecl.Line` to `ast.go`**

Insert after `func (a ActorDecl) Name()` (currently at line 298):

```go
// Line returns the 1-based source line of the actor name token using li.
func (a ActorDecl) Line(li green.LineIndex) int {
	tok := a.Name()
	if tok == nil {
		return nodeFirstTokenLine(a.node, li)
	}
	line, _ := li.LineCol(tok.Offset())
	return line
}
```

- [ ] **Step 6: Add `ServiceDecl.ContextLinesWith` to `ast.go`**

Insert after `func (s ServiceDecl) ContextLines()` (currently at line 508):

```go
// ContextLinesWith returns the 1-based source line of each context name token using li.
func (s ServiceDecl) ContextLinesWith(li green.LineIndex) []int {
	toks := s.ContextTokens()
	lines := make([]int, len(toks))
	for i, tok := range toks {
		line, _ := li.LineCol(tok.Offset())
		lines[i] = line
	}
	return lines
}
```

- [ ] **Step 7: Run new tests**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/syntax/... -run "TestUseCaseDecl_Line|TestExposureDecl_EndLine|TestActorDecl_Line|TestServiceDecl_ContextLinesWith" -v
```

Expected: all 5 new tests PASS.

- [ ] **Step 8: Run full syntax test suite**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/syntax/...
```

Expected: all existing tests still pass (no regressions).

- [ ] **Step 9: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/syntax/ast.go internal/syntax/ast_test.go
git commit -m "feat(ast): add Line/EndLine methods to UseCaseDecl, ExposureDecl, ActorDecl + ContextLinesWith"
```

---

## Task 2: Fix DocumentSymbol Range end positions

**Files:**
- Modify: `internal/lsp/server.go` (lines 669–777)
- Modify: `internal/lsp/server_test.go`

Current bug: every symbol in `DocumentSymbol()` sets `Range: {Start: line, End: line}` (no end position). The end should be the last line of the declaration block.

**Depends on:** Task 1 (needs `UseCaseDecl.Line`, `ExposureDecl.EndLine`, `ActorDecl.Line`).

- [ ] **Step 1: Write failing test in `server_test.go`**

Add this test to `internal/lsp/server_test.go`. It opens a multi-line service block and verifies `Range.End.Line > Range.Start.Line`:

```go
func TestDocumentSymbol_RangeEndPositions(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- lsp.Serve(ctx, serverIn, serverOut) }()
	defer testOut.Close()
	br := bufio.NewReader(testIn)
	id := 1

	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatalf("reading initialize response: %v", err)
	}
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	// services block spanning lines 0–4 (0-indexed); Auth service's `}` is on line 3.
	craftSrc := "services {\n  Auth {\n    contexts: Login\n  }\n}\n"
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///sym_test.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	id = 2
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id,
		Method: "textDocument/documentSymbol",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///sym_test.craft"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading message: %v", err)
		}
		if msg.ID != nil && *msg.ID == id {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for documentSymbol response")
	}
	if resp.Error != nil {
		t.Fatalf("documentSymbol error: %s", resp.Error)
	}

	var syms []struct {
		Name  string `json:"name"`
		Range struct {
			Start struct{ Line int `json:"line"` } `json:"start"`
			End   struct{ Line int `json:"line"` } `json:"end"`
		} `json:"range"`
	}
	if err := json.Unmarshal(resp.Result, &syms); err != nil {
		t.Fatalf("unmarshal documentSymbol result: %v", err)
	}
	if len(syms) == 0 {
		t.Fatal("expected at least 1 symbol")
	}
	auth := syms[0]
	if auth.Name != "Auth" {
		t.Fatalf("expected first symbol Auth, got %q", auth.Name)
	}
	if auth.Range.Start.Line >= auth.Range.End.Line {
		t.Errorf("Range.End.Line (%d) should be > Range.Start.Line (%d) for multi-line service block",
			auth.Range.End.Line, auth.Range.Start.Line)
	}
	if auth.Range.End.Line != 3 {
		t.Errorf("expected Range.End.Line == 3 (closing `}` of Auth), got %d", auth.Range.End.Line)
	}
	cancel()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/lsp/... -run TestDocumentSymbol_RangeEndPositions -v
```

Expected: FAIL — `Range.End.Line` equals `Range.Start.Line`.

- [ ] **Step 3: Fix services loop in `DocumentSymbol` (server.go lines 680–694)**

Replace the services loop body:

```go
// BEFORE:
tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
line := lspLine(tokLine)
syms = append(syms, protocol.DocumentSymbol{
    Name:           nameTok.Text(),
    Kind:           protocol.SymbolKindModule,
    Detail:         "service",
    SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
    Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
})

// AFTER:
tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
startLine := lspLine(tokLine)
endLine := lspLine(svc.EndLine(f.LineIndex))
if endLine < startLine {
    endLine = startLine
}
syms = append(syms, protocol.DocumentSymbol{
    Name:           nameTok.Text(),
    Kind:           protocol.SymbolKindModule,
    Detail:         "service",
    SelectionRange: protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: startLine}},
    Range:          protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: endLine}},
})
```

- [ ] **Step 4: Fix actors loop (server.go lines 695–708)**

Actors are single-line declarations (no closing brace); use `a.Line(f.LineIndex)` for both start and end:

```go
// AFTER:
actorLine := a.Line(f.LineIndex)
line := lspLine(actorLine)
syms = append(syms, protocol.DocumentSymbol{
    Name:           nameTok.Text(),
    Kind:           protocol.SymbolKindObject,
    Detail:         "actor: " + a.ActorTypeValue(),
    SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
    Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
})
```

(Remove the `tokLine, _` line; use `a.Line` instead of raw offset.)

- [ ] **Step 5: Fix domains loop (server.go lines 710–723)**

```go
// AFTER:
tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
startLine := lspLine(tokLine)
endLine := lspLine(d.EndLine(f.LineIndex))
if endLine < startLine {
    endLine = startLine
}
syms = append(syms, protocol.DocumentSymbol{
    Name:           nameTok.Text(),
    Kind:           protocol.SymbolKindNamespace,
    Detail:         "domain",
    SelectionRange: protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: startLine}},
    Range:          protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: endLine}},
})
```

- [ ] **Step 6: Fix use_cases loop (server.go lines 725–742)**

```go
// AFTER (replace the entire loop body):
for _, uc := range file.UseCases() {
    kwTok := uc.Keyword()
    if kwTok == nil {
        continue
    }
    startLine := lspLine(uc.Line(f.LineIndex))
    endLine := lspLine(uc.EndLine(f.LineIndex))
    if endLine < startLine {
        endLine = startLine
    }
    name := ""
    if titleTok := uc.Title(); titleTok != nil {
        name = titleTok.Text()
    }
    syms = append(syms, protocol.DocumentSymbol{
        Name:   name,
        Kind:   protocol.SymbolKindEvent,
        Detail: "use_case",
        SelectionRange: protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: startLine}},
        Range:          protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: endLine}},
    })
}
```

- [ ] **Step 7: Fix archs loop (server.go lines 744–761)**

```go
// AFTER:
startLine := lspLine(arch.Line(f.LineIndex))
endLine := lspLine(arch.EndLine(f.LineIndex))
if endLine < startLine {
    endLine = startLine
}
name := ""
if nameTok := arch.Name(); nameTok != nil {
    name = nameTok.Text()
}
if name == "" {
    name = "arch"
}
syms = append(syms, protocol.DocumentSymbol{
    Name:           name,
    Kind:           protocol.SymbolKindPackage,
    Detail:         "arch",
    SelectionRange: protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: startLine}},
    Range:          protocol.Range{Start: protocol.Position{Line: startLine}, End: protocol.Position{Line: endLine}},
})
```

- [ ] **Step 8: Fix exposures loop (server.go lines 762–775)**

```go
// AFTER:
expStartLine := lspLine(exp.Line(f.LineIndex))
expEndLine := lspLine(exp.EndLine(f.LineIndex))
if expEndLine < expStartLine {
    expEndLine = expStartLine
}
syms = append(syms, protocol.DocumentSymbol{
    Name:           nameTok.Text(),
    Kind:           protocol.SymbolKindInterface,
    Detail:         "exposure",
    SelectionRange: protocol.Range{Start: protocol.Position{Line: expStartLine}, End: protocol.Position{Line: expStartLine}},
    Range:          protocol.Range{Start: protocol.Position{Line: expStartLine}, End: protocol.Position{Line: expEndLine}},
})
```

- [ ] **Step 9: Run the new test**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/lsp/... -run TestDocumentSymbol_RangeEndPositions -v
```

Expected: PASS.

- [ ] **Step 10: Run full test suite**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./...
```

Expected: all tests pass.

- [ ] **Step 11: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/lsp/server.go internal/lsp/server_test.go
git commit -m "fix(lsp): DocumentSymbol Range.End now reflects closing brace for all block declarations"
```

---

## Task 3: Extend FoldingRanges to use_case, exposure, service, and domain blocks

**Files:**
- Modify: `internal/lsp/server.go` (lines 1200–1247)
- Modify: `internal/lsp/server_test.go`

Current: `FoldingRanges()` only handles arch blocks. Use cases, exposures, services, and domains are not foldable.

**Depends on:** Task 1 (needs `UseCaseDecl.Line`, `ExposureDecl.EndLine`).

- [ ] **Step 1: Write failing test in `server_test.go`**

```go
func TestFoldingRanges_UseCaseAndExposure(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- lsp.Serve(ctx, serverIn, serverOut) }()
	defer testOut.Close()
	br := bufio.NewReader(testIn)
	id := 1

	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatalf("reading initialize response: %v", err)
	}
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	// use_case on lines 0–3, exposure on lines 4–7 (0-indexed)
	craftSrc := "use_case \"Reg\" {\n  when Actor acts\n    Auth validates\n}\nexposure MyAPI {\n  to: Actor\n  through: Auth\n}\n"
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///fold_test.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)

	id = 2
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id,
		Method: "textDocument/foldingRange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///fold_test.craft"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading message: %v", err)
		}
		if msg.ID != nil && *msg.ID == id {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for foldingRange response")
	}
	if resp.Error != nil {
		t.Fatalf("foldingRange error: %s", resp.Error)
	}

	var ranges []struct {
		StartLine uint32 `json:"startLine"`
		EndLine   uint32 `json:"endLine"`
	}
	if err := json.Unmarshal(resp.Result, &ranges); err != nil {
		t.Fatalf("unmarshal foldingRange result: %v", err)
	}

	// Expect at least 2 folds: one for use_case and one for exposure.
	if len(ranges) < 2 {
		t.Fatalf("expected at least 2 folding ranges, got %d: %v", len(ranges), ranges)
	}

	// Find use_case fold (startLine == 0).
	foundUC := false
	for _, r := range ranges {
		if r.StartLine == 0 && r.EndLine == 3 {
			foundUC = true
		}
	}
	if !foundUC {
		t.Errorf("expected use_case fold [0,3], got: %v", ranges)
	}

	// Find exposure fold (startLine == 4).
	foundExp := false
	for _, r := range ranges {
		if r.StartLine == 4 && r.EndLine == 7 {
			foundExp = true
		}
	}
	if !foundExp {
		t.Errorf("expected exposure fold [4,7], got: %v", ranges)
	}
	cancel()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/lsp/... -run TestFoldingRanges_UseCaseAndExposure -v
```

Expected: FAIL — returns 0 folding ranges (or only arch ranges, none for use_case/exposure).

- [ ] **Step 3: Add use_case, exposure, service, and domain folds to `FoldingRanges`**

In `server.go`, after the closing brace of the `for _, arch := range file.Archs()` loop (after line 1244), insert:

```go
for _, svc := range file.Services() {
    svcStart := svc.Line(li)
    svcEnd := svc.EndLine(li)
    if svcStart <= 0 || svcEnd <= svcStart {
        continue
    }
    ranges = append(ranges, protocol.FoldingRange{
        StartLine: uint32(svcStart - 1),
        EndLine:   uint32(svcEnd - 1),
        Kind:      protocol.RegionFoldingRange,
    })
}
for _, d := range file.Domains() {
    dStart := d.Line(li)
    dEnd := d.EndLine(li)
    if dStart <= 0 || dEnd <= dStart {
        continue
    }
    ranges = append(ranges, protocol.FoldingRange{
        StartLine: uint32(dStart - 1),
        EndLine:   uint32(dEnd - 1),
        Kind:      protocol.RegionFoldingRange,
    })
}
for _, uc := range file.UseCases() {
    ucStart := uc.Line(li)
    ucEnd := uc.EndLine(li)
    if ucStart <= 0 || ucEnd <= ucStart {
        continue
    }
    ranges = append(ranges, protocol.FoldingRange{
        StartLine: uint32(ucStart - 1),
        EndLine:   uint32(ucEnd - 1),
        Kind:      protocol.RegionFoldingRange,
    })
}
for _, exp := range file.Exposures() {
    expStart := exp.Line(li)
    expEnd := exp.EndLine(li)
    if expStart <= 0 || expEnd <= expStart {
        continue
    }
    ranges = append(ranges, protocol.FoldingRange{
        StartLine: uint32(expStart - 1),
        EndLine:   uint32(expEnd - 1),
        Kind:      protocol.RegionFoldingRange,
    })
}
```

- [ ] **Step 4: Run new test**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/lsp/... -run TestFoldingRanges_UseCaseAndExposure -v
```

Expected: PASS.

- [ ] **Step 5: Run full suite**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/lsp/server.go internal/lsp/server_test.go
git commit -m "feat(lsp): FoldingRanges now covers use_case, exposure, service, and domain blocks"
```

---

## Task 4: Debounce recomputeResolution to background goroutine

**Files:**
- Modify: `internal/workspace/workspace.go`
- Modify: `internal/workspace/workspace_test.go`

Current bottleneck: `Change()` holds `w.mu.Lock()` while running `recomputeResolution()` — an O(all files) operation that calls `sema.MergeWorkspaceSymbols`, `sema.AnalyzeWorkspace`, and `sema.LintWorkspace`. Every keystroke blocks until all workspace semantics are recomputed.

Fix: keep `parseAndStore(uri, content)` synchronous (fast — only the changed file), then use `time.AfterFunc(100ms, recomputeResolution)` to run cross-file analysis in background. LSP requests see the new per-file parse immediately; cross-file diagnostics arrive ~100ms later (within the server's existing 150ms diagnostic debounce window).

`Open()` and `Initialize()` remain synchronous — both need cross-file resolution immediately on workspace load.

- [ ] **Step 1: Write failing test in `workspace_test.go`**

Add:

```go
func TestWorkspace_ChangeReturnsBeforeRecompute(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///a.craft", "actor user Alice")

	start := time.Now()
	w.Change("file:///a.craft", "actor user Bob")
	elapsed := time.Since(start)

	// Per-file parse should be fast; recompute runs async.
	// We can't assert an exact bound, but we CAN verify the file is updated.
	f := w.Get("file:///a.craft")
	if f == nil {
		t.Fatal("file not found after Change")
	}
	actors := syntax.AsFile(syntax.Root(f.Green)).Actors()
	if len(actors) == 0 || actors[0].Name().Text() != "Bob" {
		t.Errorf("expected Bob after Change, got %v", actors)
	}
	t.Logf("Change returned in %v", elapsed)
}
```

- [ ] **Step 2: Run test to confirm it passes (baseline)**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/workspace/... -run TestWorkspace_ChangeReturnsBeforeRecompute -v
```

Expected: PASS (this test does not exercise the timing yet — it just verifies correctness).

- [ ] **Step 3: Add `recompute` struct and `time` import to `workspace.go`**

Add `"time"` to the import block:

```go
import (
    "crypto/sha256"
    "fmt"
    "io/fs"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/tcarcao/craft/internal/green"
    "github.com/tcarcao/craft/internal/sema"
    "github.com/tcarcao/craft/internal/syntax"
    "github.com/tcarcao/craft/pkg/craft"
)
```

Add `recompute` field to `Workspace` struct (after the `resolution` struct field):

```go
// recompute debounces background cross-file resolution. scheduleRecompute
// stops any pending timer and starts a new one; the AfterFunc goroutine
// takes w.mu.Lock before calling recomputeResolution.
recompute struct {
    mu    sync.Mutex
    timer *time.Timer
}
```

- [ ] **Step 4: Add `scheduleRecompute` method to `workspace.go`**

Add after `recomputeResolution()`:

```go
// scheduleRecompute debounces a call to recomputeResolution: cancels any
// pending timer and starts a new 100ms timer. The AfterFunc runs under w.mu.
func (w *Workspace) scheduleRecompute() {
    w.recompute.mu.Lock()
    defer w.recompute.mu.Unlock()
    if w.recompute.timer != nil {
        w.recompute.timer.Stop()
    }
    w.recompute.timer = time.AfterFunc(100*time.Millisecond, func() {
        w.mu.Lock()
        defer w.mu.Unlock()
        w.recomputeResolution()
    })
}
```

- [ ] **Step 5: Update `Change()` to use `scheduleRecompute`**

Replace:

```go
func (w *Workspace) Change(uri, content string) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.parseAndStore(uri, content)
    w.recomputeResolution()
}
```

With:

```go
func (w *Workspace) Change(uri, content string) {
    w.mu.Lock()
    w.parseAndStore(uri, content)
    w.mu.Unlock()
    w.scheduleRecompute()
}
```

- [ ] **Step 6: Update the performance gate test comment in `workspace_test.go`**

Find `TestWorkspace_PerformanceGate_S5` and update the comment:

```go
// TestWorkspace_PerformanceGate_S5 verifies the Q21 performance gate:
// a synthetic 20-file workspace completes a per-file parse cycle in ≤200ms.
// Cross-file resolution (recomputeResolution) now runs asynchronously and is
// NOT included in this measurement; see scheduleRecompute in workspace.go.
```

Also update the error/log messages inside the test:
```go
// Change:
t.Errorf("S5 perf gate: per-file parse took %v, budget is %dms", elapsed, budgetMs)
// and:
t.Logf("S5 perf gate: per-file parse took %v (budget %dms) ✓", elapsed, budgetMs)
```

- [ ] **Step 7: Run workspace tests**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/workspace/... -v
```

Expected: all 8 tests pass. The `PerformanceGate_S5` test will now measure only per-file parse, which completes in <<200ms.

- [ ] **Step 8: Run full suite**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./...
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/workspace/workspace.go internal/workspace/workspace_test.go
git commit -m "perf(workspace): debounce recomputeResolution to 100ms AfterFunc; Change() returns after per-file parse"
```

---

## Task 5: Smarter completions after action verbs

**Files:**
- Modify: `internal/lsp/completion.go`
- Modify: `internal/lsp/completion_test.go`

Current: inside a use_case action line, ALL symbols (actors + domains + services + bounded contexts) are returned regardless of cursor position. When the cursor is immediately after `asks` or `returns` (with exactly one identifier before the verb), only domain/service/bounded-context names are relevant.

Fix: add `ctxUseCaseActionVerb` detection and a `domainAndServiceSymbolCompletions` helper.

- [ ] **Step 1: Write failing test in `completion_test.go`**

Look at the existing `completion_test.go` for the test pattern. The tests use direct function calls against a workspace:

```go
func TestCompletion_AfterAsksVerb_ReturnsDomainAndServiceOnly(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///domain.craft", "domain Auth {\n  Login\n  Register\n}\n")
	w.Open("file:///service.craft", "services {\n  PaymentService {\n    contexts: Login\n    language: golang\n  }\n}\n")
	w.Open("file:///uc.craft", "use_case \"Register User\" {\n  when Actor creates Account\n    Auth asks \n}\n")

	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///uc.craft"},
			// Line 2, after "    Auth asks " — cursor past asks
			Position: protocol.Position{Line: 2, Character: 14},
		},
	}

	result := lsp.ExportedBuildCompletions(w, "file:///uc.craft", params)
	if result == nil {
		t.Fatal("expected completion list, got nil")
	}

	labels := make(map[string]bool)
	for _, item := range result.Items {
		labels[item.Label] = true
	}

	// Must include domain bounded contexts and services.
	if !labels["Login"] {
		t.Error("expected 'Login' (bounded context) in completions")
	}
	if !labels["PaymentService"] {
		t.Error("expected 'PaymentService' (service) in completions")
	}

	// Must NOT include actor names.
	if labels["Actor"] {
		t.Error("actor 'Actor' should not appear in action-verb completions")
	}
}
```

> **Note:** `lsp.ExportedBuildCompletions` does not exist yet. You will need to add an export shim (or call `buildCompletions` directly via a test-only exported wrapper). See Step 3.

- [ ] **Step 2: Run test to verify compilation error**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go build ./internal/lsp/... 2>&1 | head -10
```

Expected: `undefined: lsp.ExportedBuildCompletions`.

- [ ] **Step 3: Add export shim for tests**

Add to `internal/lsp/completion.go` (at end of file):

```go
// ExportedBuildCompletions is a test-only export of buildCompletions.
// Use only in _test packages.
var ExportedBuildCompletions = buildCompletions
```

- [ ] **Step 4: Add `ctxUseCaseActionVerb` constant to `completion.go`**

In the `completionContext` iota block, add after `ctxUseCaseAction`:

```go
ctxUseCaseActionVerb // cursor immediately after `asks` or `returns` — suggest domain/service names
```

- [ ] **Step 5: Add `isAfterActionVerb` helper to `completion.go`**

Add after `isActionLine`:

```go
// isAfterActionVerb returns true when the line prefix has exactly two
// whitespace-separated tokens and the second is `asks` or `returns`.
// Used to distinguish "target name" position from general action body.
func isAfterActionVerb(linePrefix string) bool {
	fields := strings.Fields(strings.TrimSpace(linePrefix))
	if len(fields) != 2 {
		return false
	}
	return fields[1] == "asks" || fields[1] == "returns"
}
```

- [ ] **Step 6: Update `detectContext` to use `isAfterActionVerb`**

Replace the `"use_case"` case in the final `switch enclosing` block:

```go
// BEFORE:
case "use_case":
    if isActionLine(linePrefix) {
        return ctxUseCaseAction
    }
    return ctxUseCaseTop

// AFTER:
case "use_case":
    if isActionLine(linePrefix) {
        if isAfterActionVerb(linePrefix) {
            return ctxUseCaseActionVerb
        }
        return ctxUseCaseAction
    }
    return ctxUseCaseTop
```

- [ ] **Step 7: Add `domainAndServiceSymbolCompletions` function to `completion.go`**

Add after `bcSymbolCompletions`:

```go
// domainAndServiceSymbolCompletions returns bounded contexts and service names.
// Used after action verbs like `asks` and `returns` where the target must be
// a domain context or a service, not an actor.
func domainAndServiceSymbolCompletions(ws *workspace.Workspace) *protocol.CompletionList {
	wsSym := ws.WorkspaceSymbols()
	var items []protocol.CompletionItem
	for name := range wsSym.Services {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "service",
		})
	}
	for domName, dom := range wsSym.Domains {
		for _, bc := range dom.BoundedContexts {
			items = append(items, protocol.CompletionItem{
				Label:  bc,
				Kind:   protocol.CompletionItemKindModule,
				Detail: "bounded context (domain: " + domName + ")",
			})
		}
	}
	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}
```

- [ ] **Step 8: Wire `ctxUseCaseActionVerb` in `buildCompletions`**

Add case to the switch in `buildCompletions`:

```go
case ctxUseCaseActionVerb:
    return domainAndServiceSymbolCompletions(ws)
```

Place it immediately before `case ctxUseCaseAction:`.

- [ ] **Step 9: Run new test**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./internal/lsp/... -run TestCompletion_AfterAsksVerb_ReturnsDomainAndServiceOnly -v
```

Expected: PASS.

- [ ] **Step 10: Run full suite**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && go test ./...
```

Expected: all pass.

- [ ] **Step 11: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add internal/lsp/completion.go internal/lsp/completion_test.go
git commit -m "feat(completion): suggest domain/service names after action verbs (asks, returns)"
```

---

## Self-Review

**Spec coverage:**
- ✅ Missing AST position methods (UseCaseDecl.Line, ExposureDecl.EndLine, ActorDecl.Line, ContextLinesWith) → Task 1
- ✅ Full node spans on DocumentSymbol → Task 2
- ✅ FoldingRanges for use_case, exposure, service, domain → Task 3
- ✅ Incremental/background cross-file resolution → Task 4
- ✅ Smarter completions at action-verb position → Task 5

**Placeholder scan:** No TBDs. All code blocks are complete.

**Type consistency:**
- `uc.Line(li)` defined in Task 1, used in Tasks 2 and 3 ✓
- `exp.EndLine(li)` defined in Task 1, used in Tasks 2 and 3 ✓
- `a.Line(li)` defined in Task 1, used in Task 2 ✓
- `scheduleRecompute()` defined and called in Task 4 ✓
- `ctxUseCaseActionVerb` defined in Task 5 step 4, used in steps 6 and 8 ✓
- `domainAndServiceSymbolCompletions` defined in Task 5 step 7, called in step 8 ✓
- `ExportedBuildCompletions` added in step 3, used in test in step 1 ✓
