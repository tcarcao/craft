# S11a — Flip Default to v2 + Pre-Release Publish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flip all CLI/HTTP parser defaults from `antlr` to `v2`, complete `$/setTrace`/`$/logTrace` with real trace-level gating, update the release pipelines for pre-release publishing, bump the extension version to `0.1.0`, and author CHANGELOG entries — so Tiago can tag `v0.1.0` in both repos to trigger the pre-release publish.

**Architecture:** Four independent code changes (trace, parser defaults, HTTP server, release pipeline) plus a CHANGELOG. All ship as one PR or sequential small commits; all are AFK code changes. The actual `git tag v0.1.0` + dogfood (Step 5 deliverable) is HITL and is intentionally NOT automated here.

**Tech Stack:** Go 1.22 (`go.lsp.dev/protocol`, `cobra`, `slog`), TypeScript (`vscode-languageclient`), GitHub Actions (`@vscode/vsce`, `ovsx`).

---

## File Map

| File | Change |
|------|--------|
| `craft/internal/lsp/server.go` | Add `traceLevel` field; wire `SetTrace`/`LogTrace` to store level and gate outbound notifications |
| `craft/internal/lsp/server_test.go` | Add `TestSetTrace_LogTrace` integration test |
| `craft/cmd/craft/check.go` | Change `--parser` default from `"antlr"` to `"v2"` |
| `craft/cmd/craft/generate.go` | Add `--parser` flag (default `"v2"`); wire v2 parse path |
| `craft/cmd/server/main.go` | Extract `parseDSL` helper; all handlers read `?parser=` query param, default `v2` |
| `craft/CHANGELOG.md` | Create; v0.1.0 entry |
| `craft/.github/workflows/release.yml` | Already correct — no changes needed |
| `craft-vscode-extension/package.json` | Bump version `0.0.11` → `0.1.0` |
| `craft-vscode-extension/.github/workflows/release.yml` | Add `--pre-release` to both vsce and ovsx publish steps |
| `craft-vscode-extension/CHANGELOG.md` | Create; v0.1.0 entry |

---

## Task 1: C-trace — SetTrace/LogTrace with real trace-level gating

**Source-plan ref:** C-trace (from tracer bullets S11a deliverables)

**Why:** Currently `SetTrace` and `LogTrace` are no-ops (they log at `Debug` and discard the level). C-trace means: store the trace level set by the client; gate outbound `$/logTrace` notifications on that level; emit those notifications from key handler entry points so the client can watch server internals.

**Files:**
- Modify: `craft/internal/lsp/server.go`
- Modify: `craft/internal/lsp/server_test.go`

- [ ] **Step 1.1: Write the failing test**

Add to `craft/internal/lsp/server_test.go`, after `TestServeLifecycle`:

```go
func TestSetTrace_LogTrace(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- lsp.Serve(ctx, serverIn, serverOut)
	}()

	defer testOut.Close()
	br := bufio.NewReader(testIn)

	// initialize
	id1 := 1
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id1, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{},"trace":"verbose"}`),
	}); err != nil {
		t.Fatal(err)
	}
	initResp, err := readMsg(br)
	if err != nil || initResp.Error != nil {
		t.Fatalf("initialize failed: err=%v resp=%s", err, initResp.Error)
	}

	// initialized (no response expected)
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// $/setTrace verbose
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "$/setTrace", Params: json.RawMessage(`{"value":"verbose"}`)}); err != nil {
		t.Fatal(err)
	}

	// open a document to trigger a parse + logTrace notification
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test.craft","languageId":"craft","version":1,"text":"actor Foo"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	// Collect messages for 500ms; expect at least one $/logTrace notification
	deadline := time.Now().Add(500 * time.Millisecond)
	var gotLogTrace bool
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			break
		}
		if msg.Method == "$/logTrace" {
			gotLogTrace = true
			break
		}
		// diagnostics or other notifications are fine to skip
	}

	if !gotLogTrace {
		t.Error("expected at least one $/logTrace notification after $/setTrace verbose + didOpen, got none")
	}

	// Now set trace to "off" — subsequent didChange must NOT produce $/logTrace
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "$/setTrace", Params: json.RawMessage(`{"value":"off"}`)}); err != nil {
		t.Fatal(err)
	}
	id2 := 2
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test.craft","version":2},"contentChanges":[{"text":"actor Bar"}]}`),
		ID: &id2,
	}); err != nil {
		t.Fatal(err)
	}

	deadline2 := time.Now().Add(300 * time.Millisecond)
	var sawLogTraceAfterOff bool
	for time.Now().Before(deadline2) {
		msg, err := readMsg(br)
		if err != nil {
			break
		}
		if msg.Method == "$/logTrace" {
			sawLogTraceAfterOff = true
			break
		}
	}
	if sawLogTraceAfterOff {
		t.Error("expected no $/logTrace after $/setTrace off, but got one")
	}

	cancel()
}
```

- [ ] **Step 1.2: Run the test to confirm it fails red**

```bash
cd craft && go test ./internal/lsp/... -run TestSetTrace_LogTrace -v
```
Expected: `FAIL` — either times out or never gets a `$/logTrace` notification.

- [ ] **Step 1.3: Add `traceLevel` field and `notifyLogTrace` helper to server.go**

In `craft/internal/lsp/server.go`, locate the `Server` struct (around line 37). Add `traceLevel` field:

```go
// Server is the Craft LSP server. It satisfies protocol.Server.
type Server struct {
	conn     jsonrpc2.Conn
	logger   *slog.Logger
	ws       *workspace.Workspace
	shutdown bool
	exitCh   chan struct{}

	// traceLevel is set by $/setTrace. Guarded by mu.
	mu         sync.Mutex
	traceLevel protocol.TraceValue

	// debounce state for publishDiagnostics
	debounce struct {
		mu     sync.Mutex
		timers map[string]*time.Timer
	}
}
```

Note: the existing `sync` import covers this. `protocol.TraceValue` is already imported.

- [ ] **Step 1.4: Wire `SetTrace` to store the level**

Replace the existing `SetTrace` stub (around line 167):

```go
func (s *Server) SetTrace(_ context.Context, params *protocol.SetTraceParams) error {
	s.mu.Lock()
	s.traceLevel = params.Value
	s.mu.Unlock()
	s.logger.Debug("$/setTrace", "value", params.Value)
	return nil
}
```

- [ ] **Step 1.5: Add `notifyLogTrace` helper and update `LogTrace`**

Add this helper just before `SetTrace` (after `WorkDoneProgressCancel`):

```go
// notifyLogTrace sends a $/logTrace notification to the client if the current
// trace level is "messages" or "verbose". Safe to call from any goroutine.
func (s *Server) notifyLogTrace(ctx context.Context, message string) {
	s.mu.Lock()
	level := s.traceLevel
	s.mu.Unlock()
	if level == protocol.TraceOff || level == "" {
		return
	}
	// fire-and-forget; errors are expected if client disconnects
	_ = s.conn.Notify(ctx, "$/logTrace", &protocol.LogTraceParams{Message: message})
}
```

Update the existing `LogTrace` stub (client→server direction; rare, just log it):

```go
func (s *Server) LogTrace(_ context.Context, params *protocol.LogTraceParams) error {
	s.logger.Debug("$/logTrace (from client)", "message", params.Message)
	return nil
}
```

- [ ] **Step 1.6: Emit `notifyLogTrace` from `DidOpen` and `DidChange`**

Locate `DidOpen` (around line 360) and `DidChange` (around line 324). Add one call each, after the workspace update:

In `DidChange`:
```go
func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if len(params.ContentChanges) == 0 {
		return nil
	}
	uri := string(params.TextDocument.URI)
	content := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.ws.Change(uri, content)
	s.notifyLogTrace(ctx, fmt.Sprintf("didChange: parsed %s", uri))
	s.scheduleDiagnostics(ctx, uri)
	return nil
}
```

In `DidOpen`:
```go
func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := string(params.TextDocument.URI)
	s.ws.Open(uri, params.TextDocument.Text)
	s.notifyLogTrace(ctx, fmt.Sprintf("didOpen: parsed %s", uri))
	s.scheduleDiagnostics(ctx, uri)
	return nil
}
```

Check the exact current body of both functions by reading lines 324–375 of server.go and replicate any additional logic they contain (the above shows the key additions; don't drop existing lines).

- [ ] **Step 1.7: Also honour the `trace` field from `InitializeParams`**

The `initialize` request can carry `"trace": "verbose"` in its params. Find the `Initialize` handler (search for `func (s *Server) Initialize`) in server.go and store the initial trace value:

```go
// Near the top of the Initialize handler body, after extracting rootUri/rootPath:
if params.Trace != "" {
    s.mu.Lock()
    s.traceLevel = params.Trace
    s.mu.Unlock()
}
```

The `params.Trace` field is of type `protocol.TraceValue`. Verify the field name by checking:
```bash
grep -n "Trace" $(go env GOPATH)/pkg/mod/go.lsp.dev/protocol@v0.12.0/general.go | grep "Params\|Trace " | head -10
```
If the field is named differently, adapt accordingly.

- [ ] **Step 1.8: Run the test green**

```bash
cd craft && go test ./internal/lsp/... -run TestSetTrace_LogTrace -v -timeout 10s
```
Expected: `PASS`

- [ ] **Step 1.9: Run the full test suite**

```bash
cd craft && go test ./...
```
Expected: all packages `ok`.

- [ ] **Step 1.10: Commit**

```bash
cd craft
git add internal/lsp/server.go internal/lsp/server_test.go
git commit -m "feat(lsp): implement C-trace — SetTrace/LogTrace with real trace-level gating"
```

---

## Task 2: Default-parser flip — `check` and `generate` CLI commands

**Source-plan ref:** A5a (flip half); tracer bullets S11a.

**Files:**
- Modify: `craft/cmd/craft/check.go`
- Modify: `craft/cmd/craft/generate.go`

### 2a — `check.go`: flip the default

- [ ] **Step 2a.1: Change the default**

In `craft/cmd/craft/check.go` line 88, change:

```go
cmd.Flags().StringVar(&parserFlag, "parser", "antlr", "parser to use: antlr|v2")
```

to:

```go
cmd.Flags().StringVar(&parserFlag, "parser", "v2", "parser to use: v2|antlr  (antlr kept as escape hatch)")
```

- [ ] **Step 2a.2: Run existing tests**

```bash
cd craft && go test ./...
```
Expected: all `ok` (no tests call `check` directly; parser_diff harnesses cover the underlying parsers).

- [ ] **Step 2a.3: Commit**

```bash
git add cmd/craft/check.go
git commit -m "feat(cli): flip check --parser default from antlr to v2"
```

### 2b — `generate.go`: add `--parser` flag with v2 default

The `generate` command currently hardcodes ANTLR. Add a `--parser` flag and a v2 code path.

- [ ] **Step 2b.1: Write the test**

There are no existing unit tests for `generate` (it has no test file). Add a quick integration test to catch parser wiring regressions. Create `craft/cmd/craft/generate_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCmd_V2Default(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, []byte("actor Foo"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetArgs([]string{"generate", src, "--type", "c4", "--output", tmp})
	if err := root.Execute(); err != nil {
		t.Fatalf("generate with v2 default: %v", err)
	}

	got, err := filepath.Glob(filepath.Join(tmp, "*.puml"))
	if err != nil || len(got) == 0 {
		t.Fatal("expected at least one .puml output file")
	}
}

func TestGenerateCmd_AntlrEscapeHatch(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, []byte("actor Foo"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetArgs([]string{"generate", src, "--parser", "antlr", "--type", "c4", "--output", tmp})
	if err := root.Execute(); err != nil {
		// antlr requires a running docker image in some envs; accept error but ensure no panic
		if strings.Contains(err.Error(), "panic") {
			t.Fatalf("antlr path panicked: %v", err)
		}
	}
}
```

For this test to compile you must also expose `newRootCmd()` from `cmd/craft/main.go` (currently the root cmd is built inline). Check `main.go` — if the root is already exported as a function, use it. If `main.go` builds the root inline, extract it:

In `craft/cmd/craft/main.go`, find the root command construction and wrap it:

```go
func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "craft"}
	root.AddCommand(checkCmd())
	root.AddCommand(generateCmd())
	root.AddCommand(lspCmd())
	root.AddCommand(inspectCmd())
	root.AddCommand(filesCmd())
	root.AddCommand(validateCmd())
	// add any other subcommands that exist
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
```

Read the actual `main.go` to see what subcommands exist and replicate the exact list.

- [ ] **Step 2b.2: Run the test to confirm it fails red**

```bash
cd craft && go test ./cmd/craft/... -run TestGenerateCmd -v
```
Expected: compile error (generate still uses antlr only) or FAIL.

- [ ] **Step 2b.3: Implement the v2 path in `generate.go`**

Replace `craft/cmd/craft/generate.go` with the following (keep all existing helper functions `generateForFile`, `baseName`, `expandType` unchanged; only modify `generateCmd` and add imports):

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/internal/parser_antlr_adapter"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/internal/visualizer"
	craft "github.com/tcarcao/craft/pkg/craft"
)

func generateCmd() *cobra.Command {
	var diagType string
	var mode string
	var outputDir string
	var parserFlag string

	cmd := &cobra.Command{
		Use:   "generate [files...]",
		Short: "Generate PlantUML diagram files from .craft files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveFiles(args)
			if err != nil {
				return err
			}

			v := visualizer.New()

			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}

				var doc *craft.CraftDoc
				switch parserFlag {
				case "antlr":
					p := parser.NewParser()
					model, err := p.ParseString(string(content))
					if err != nil {
						return fmt.Errorf("%s: parse error: %w", file, err)
					}
					doc = parser_antlr_adapter.FromDSLModel(model)
				case "v2":
					astFile, diags := syntax.Parse(string(content))
					if len(diags) > 0 {
						for _, d := range diags {
							fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", file, d.Range.Start.Line+1, d.Severity, d.Message)
						}
					}
					doc = syntax.Project(astFile)
				default:
					return fmt.Errorf("unknown --parser value %q; want v2|antlr", parserFlag)
				}

				outDir := outputDir
				if outDir == "" {
					outDir = filepath.Dir(file)
				}
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return err
				}

				base := baseName(file)

				if err := generateForFile(v, doc, base, outDir, diagType, mode); err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&diagType, "type", "all", "diagram type: c4|domain|sequence|all")
	cmd.Flags().StringVar(&mode, "mode", "detailed", "domain diagram mode: detailed|architecture")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory (default: same as input file)")
	cmd.Flags().StringVar(&parserFlag, "parser", "v2", "parser to use: v2|antlr  (antlr kept as escape hatch)")
	return cmd
}
```

Keep `generateForFile`, `baseName`, `expandType` exactly as they are in the current file.

- [ ] **Step 2b.4: Run tests green**

```bash
cd craft && go test ./cmd/craft/... -run TestGenerateCmd -v
```
Expected: both tests `PASS` (the `AntlrEscapeHatch` test may print a parse error to stderr but must not panic).

- [ ] **Step 2b.5: Run full suite**

```bash
cd craft && go test ./...
```
Expected: all `ok`.

- [ ] **Step 2b.6: Commit**

```bash
git add cmd/craft/generate.go cmd/craft/generate_test.go cmd/craft/main.go
git commit -m "feat(cli): add --parser to generate command, default v2; extract newRootCmd for tests"
```

---

## Task 3: HTTP server — `?parser=` query param support, default v2

**Source-plan ref:** Source plan Q14 ("CLI + HTTP server expose `--parser={antlr,v2}`").

**Files:**
- Modify: `craft/cmd/server/main.go`

The HTTP server has five independent parse sites:
- `handleGenerate` (HTML form)
- `handlePreviewDomain`
- `handlePreviewC4`
- `handleDownloadDomain`
- `handleDownloadC4`

All currently call `parser.NewParser()` + `parser_antlr_adapter.FromDSLModel()` inline. We extract one shared helper and route each handler through it.

- [ ] **Step 3.1: Write the test**

Create `craft/cmd/server/main_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDSL_V2Default(t *testing.T) {
	doc, err := parseDSL("actor Foo", "v2")
	if err != nil {
		t.Fatalf("v2 parse failed: %v", err)
	}
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor in CraftDoc")
	}
}

func TestParseDSL_AntlrFallback(t *testing.T) {
	doc, err := parseDSL("actor Foo", "antlr")
	if err != nil {
		t.Fatalf("antlr parse failed: %v", err)
	}
	if len(doc.Actors) == 0 {
		t.Error("expected at least one actor in CraftDoc")
	}
}

func TestHandlePreviewDomain_DefaultsToV2(t *testing.T) {
	srv, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}

	body := `{"dsl":"actor Foo","diagramType":"domain"}`
	req := httptest.NewRequest(http.MethodPost, "/preview/domain", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.handlePreviewDomain()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 3.2: Run the test — confirm red**

```bash
cd craft && go test ./cmd/server/... -run TestParseDSL -v
```
Expected: compile error (`parseDSL` not defined).

- [ ] **Step 3.3: Add `parseDSL` helper and rewire all handlers**

At the top of `craft/cmd/server/main.go`, after imports, add:

```go
import (
	// existing imports...
	"github.com/tcarcao/craft/internal/syntax"
)
```

Add the helper function (place just after the `respondWithError` function):

```go
// parseDSL parses craft DSL source using the specified parser ("v2" or "antlr").
// "v2" is the default; "antlr" is available as an escape hatch during migration.
func parseDSL(src, parserName string) (*craftpkg.CraftDoc, error) {
	switch parserName {
	case "antlr":
		p := parser.NewParser()
		model, err := p.ParseString(src)
		if err != nil {
			return nil, err
		}
		return parser_antlr_adapter.FromDSLModel(model), nil
	default: // "v2" and any unknown value
		astFile, _ := syntax.Parse(src)
		return syntax.Project(astFile), nil
	}
}
```

Ensure the import alias: the existing `craft` import alias in `cmd/server/main.go` may conflict. Check the file's existing imports. If `"github.com/tcarcao/craft/pkg/craft"` is NOT currently imported, add:
```go
craftpkg "github.com/tcarcao/craft/pkg/craft"
```
and adjust the function return type to `*craftpkg.CraftDoc`. If it IS already imported with an alias, use that alias.

Now update each handler to call `parseDSL` and read `?parser=` from the request. For each handler listed below, make the following substitution pattern:

**Pattern:** replace the inline `parser.NewParser()` + `p.ParseString(...)` + `parser_antlr_adapter.FromDSLModel(...)` block with a single `parseDSL(dsl, r.URL.Query().Get("parser"))` call (defaulting to "v2" inside `parseDSL` when the param is empty).

**`handleGenerate`** (HTML form, uses form value not JSON):
```go
// Replace:
//   p := parser.NewParser()
//   arch, err := p.ParseString(input)
//   if err != nil { s.respondWithError(...); return }
//   doc := parser_antlr_adapter.FromDSLModel(arch)
// With:
doc, err := parseDSL(input, r.FormValue("parser"))
if err != nil {
    s.respondWithError(w, err, input, generateC4, generateContext, generateSequence)
    return
}
```

**`handlePreviewDomain`** (JSON body):
```go
// Replace the parser.NewParser() / p.ParseString / parser_antlr_adapter block with:
model, err := parseDSL(req.DSL, r.URL.Query().Get("parser"))
if err != nil {
    respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
    return
}
```

**`handlePreviewC4`** (JSON body):
```go
arch, err := parseDSL(req.DSL, r.URL.Query().Get("parser"))
if err != nil {
    respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
    return
}
```

Apply the same substitution to `handleDownloadDomain` and `handleDownloadC4` — read the rest of `main.go` (lines 300–450) to find those handlers and replace their inline parse blocks identically.

After all substitutions, remove the now-unused top-level `parser` and `parser_antlr_adapter` imports from `main.go` **only if** they are no longer referenced anywhere else in the file. Run `go build ./cmd/server/` to check for unused imports.

- [ ] **Step 3.4: Run the test green**

```bash
cd craft && go test ./cmd/server/... -v
```
Expected: `PASS`.

- [ ] **Step 3.5: Run full suite**

```bash
cd craft && go test ./...
```
Expected: all `ok`.

- [ ] **Step 3.6: Commit**

```bash
git add cmd/server/main.go cmd/server/main_test.go
git commit -m "feat(server): add parseDSL helper + ?parser= query param, default v2"
```

---

## Task 4: Extension version bump + release pipeline pre-release support

**Source-plan ref:** S11a distribution deliverables; D-3 (version coupling), D-15 (OpenVSX).

**Files:**
- Modify: `craft-vscode-extension/package.json`
- Modify: `craft-vscode-extension/.github/workflows/release.yml`

### 4a — Bump extension version to 0.1.0

VSCode Marketplace requires clean `major.minor.patch` versions. `0.1.0` is the pre-release stream (odd minor per VSCode convention); `0.2.0` will be the first stable stream in S11b.

- [ ] **Step 4a.1: Update `package.json`**

In `craft-vscode-extension/package.json`, change:
```json
"version": "0.0.11",
```
to:
```json
"version": "0.1.0",
```

- [ ] **Step 4a.2: Build the extension to verify**

```bash
cd craft-vscode-extension && npm run compile 2>&1 | tail -5
```
Expected: exits 0, no type errors.

- [ ] **Step 4a.3: Commit**

```bash
cd craft-vscode-extension
git add package.json
git commit -m "chore: bump extension version to 0.1.0 (pre-release stream)"
```

### 4b — Add `--pre-release` to release workflow

The existing `release.yml` publishes to stable channel. S11a publishes pre-release. Add `--pre-release` to both vsce and ovsx publish steps. S11b will introduce a separate stable workflow.

- [ ] **Step 4b.1: Locate publish steps in `.github/workflows/release.yml`**

The current file ends with:
```yaml
      - name: Publish to VS Code Marketplace
        run: npx @vscode/vsce publish -p ${{ secrets.DEV_AZURE_PERSONAL_TOKEN }}
        env:
          VSCE_PAT: ${{ secrets.DEV_AZURE_PERSONAL_TOKEN }}

      - name: Publish to Open VSX Registry
        run: npx ovsx publish ./${{ steps.package.outputs.name }}-${{ steps.package.outputs.version }}.vsix -p ${{ secrets.OVSX_PAT }}
```

- [ ] **Step 4b.2: Add `--pre-release` to both steps**

Replace those two steps with:

```yaml
      - name: Publish to VS Code Marketplace (pre-release)
        run: npx @vscode/vsce publish --pre-release -p ${{ secrets.DEV_AZURE_PERSONAL_TOKEN }}
        env:
          VSCE_PAT: ${{ secrets.DEV_AZURE_PERSONAL_TOKEN }}

      - name: Publish to Open VSX Registry (pre-release)
        run: npx ovsx publish --pre-release ./${{ steps.package.outputs.name }}-${{ steps.package.outputs.version }}.vsix -p ${{ secrets.OVSX_PAT }}
```

- [ ] **Step 4b.3: Verify the smoke-test step still references `craft lsp --stdio`**

The smoke-test step runs:
```bash
./craft lsp --stdio
```
But `craft lsp` doesn't accept `--stdio` until the fix committed at the top of this session. Make sure the fix to `craft/cmd/craft/lsp.go` (adding `--stdio` as a noop flag) has been committed before this release workflow runs. That commit is a prerequisite and should already be in the `craft` repo.

Double-check:
```bash
cd craft && grep "stdio" cmd/craft/lsp.go
```
Expected: `cmd.Flags().Bool("stdio", ...)` is present.

If not present, add it now:
```bash
cd craft
# Edit cmd/craft/lsp.go to add the flag (already done earlier in this session)
git add cmd/craft/lsp.go
git commit -m "fix(cli): accept --stdio flag in lsp subcommand for LSP client compatibility"
```

- [ ] **Step 4b.4: Commit the workflow change**

```bash
cd craft-vscode-extension
git add .github/workflows/release.yml
git commit -m "ci: add --pre-release flag to Marketplace and OpenVSX publish steps (S11a)"
```

---

## Task 5: CHANGELOG entries

**Source-plan ref:** S11a deliverable "CHANGELOG.md draft entry for v0.1.0-rc1".

**Files:**
- Create: `craft/CHANGELOG.md`
- Create: `craft-vscode-extension/CHANGELOG.md`

- [ ] **Step 5.1: Create `craft/CHANGELOG.md`**

```markdown
# Changelog

## [0.1.0] — 2026-04-24 (pre-release)

### Added
- Hand-written Go parser (`--parser=v2`) covering the full Craft grammar: actors, domains, services, use cases, arch blocks, and exposures.
- LSP server (`craft lsp`) with diagnostics, document symbols, hover, semantic tokens, go-to-definition, folding ranges, and `workspace/executeCommand` for domain/service extraction.
- Island parsing and error recovery — broken blocks don't poison the rest of the file.
- `$/setTrace` / `$/logTrace` trace-level protocol support.
- `craft check --lsp-json` CLI mirror for LSP-response debugging.
- `craft diff-parsers` subcommand for interactive parser comparison.
- Acceptance corpus at `testdata/corpus/` with 80+ `.craft` files and paired `.craftjson` goldens.

### Changed
- `craft check` and `craft generate` now default to `--parser=v2`. Use `--parser=antlr` as an escape hatch.
- HTTP server (`craft server`) now defaults to the v2 parser; `?parser=antlr` query param available as escape hatch.

### Notes
- ANTLR parser remains available throughout the `0.1.x` pre-release stream. It will be removed in `0.2.0` after the stable release.
- macOS users: if Gatekeeper blocks the binary after download, run `xattr -dr com.apple.quarantine /path/to/craft` once. Proper code signing lands in `0.2.0`.
```

- [ ] **Step 5.2: Create `craft-vscode-extension/CHANGELOG.md`**

```markdown
# Changelog

## [0.1.0] — 2026-04-24 (pre-release)

### Added
- LSP-powered editing: diagnostics, document outline, hover, semantic colouring, go-to-definition, folding ranges.
- Automatic `craft` binary download on first `.craft` file open (rust-analyzer style). Binary is cached in VS Code global storage and auto-updated on extension update.
- `craft.lsp.executablePath` setting to override the managed binary with a local build.
- TextMate grammar for baseline syntax colouring (keyword, string, comment highlighting) independent of the LSP.

### Changed
- `server/` directory (old Node.js LSP) removed. All language intelligence now served by the Go `craft lsp` subprocess.
- Tree-sitter client-side highlighting removed. TextMate grammar takes over for editor colouring.
- Diagram preview (`previewC4`, `previewDomain`) continues to use the HTTP `craft server` — no change needed.

### Notes
- This is a pre-release. Install via the VS Code "Pre-release" channel or with `code --install-extension craft-arch-diagrams-0.1.0.vsix`.
- macOS: see the `craft` CHANGELOG for the Gatekeeper workaround.
```

- [ ] **Step 5.3: Commit both changelogs**

```bash
cd craft
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG for v0.1.0 pre-release"

cd ../craft-vscode-extension
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG for v0.1.0 pre-release"
```

---

## Task 6: Final verification before handing off to Tiago

- [ ] **Step 6.1: Run all Go tests**

```bash
cd craft && go test ./...
```
Expected: all packages `ok`.

- [ ] **Step 6.2: Build the binary and verify `craft lsp` accepts `--stdio`**

```bash
cd craft
go build -o /tmp/craft-s11a ./cmd/craft/
/tmp/craft-s11a --version 2>&1 || true
/tmp/craft-s11a lsp --stdio --help 2>&1 | head -3
```
Expected: no "unknown flag" error.

- [ ] **Step 6.3: Smoke `craft check` defaults to v2**

```bash
/tmp/craft-s11a check testdata/corpus/01_actors/simple_actor.craft 2>&1 | head -5
```
Expected: valid CraftDoc JSON output, no "not yet implemented" error.

- [ ] **Step 6.4: Build the extension and package a local VSIX**

```bash
cd craft-vscode-extension
npm ci
npm run compile
npx @vscode/vsce package --no-dependencies
ls *.vsix
```
Expected: `craft-arch-diagrams-0.1.0.vsix` produced.

- [ ] **Step 6.5: Run extension tests**

```bash
cd craft-vscode-extension && npm test 2>&1 | tail -20
```
Expected: all tests pass.

---

## Handoff note for Tiago (HITL steps — NOT automated)

After the above code changes are committed and CI is green:

1. **Tag `craft` first:**
   ```bash
   cd craft && git tag v0.1.0 && git push origin v0.1.0
   ```
   Wait for GoReleaser CI to complete and all GitHub Release assets to appear (`craft_0.1.0_linux_amd64.tar.gz`, etc. + `checksums.txt`).

2. **Tag `craft-vscode-extension`:**
   ```bash
   cd craft-vscode-extension && git tag v0.1.0 && git push origin v0.1.0
   ```
   The release workflow will:
   - Update `package.json` version to `0.1.0` (already matches tag — no-op)
   - Smoke-test the binary download from the craft release
   - Package the VSIX
   - Publish pre-release to Marketplace + OpenVSX

3. **Dogfood** using the pre-release VSIX per the S11a checklist in `lsp-migration-tracer-bullets.md` §S11a.

---

## Self-review

**Spec coverage check:**
- ✅ Default-parser flip: Task 2 (check), Task 2b (generate), Task 3 (HTTP server)
- ✅ `$/setTrace` + `$/logTrace`: Task 1
- ✅ Pre-release publish: Task 4b (workflow `--pre-release` flag)
- ✅ CHANGELOG: Task 5
- ✅ Version bump: Task 4a (`0.0.11` → `0.1.0`)
- ✅ `--stdio` fix: prerequisite in Task 4b.3 (and already committed earlier)
- ✅ Dogfood handoff: Task 6 + Handoff note
- ⚠️ `generate_test.go` requires `newRootCmd()` exposure — covered in Step 2b.1 with explicit instruction to read current `main.go` and replicate command list

**Type consistency:** `parseDSL` returns `*craftpkg.CraftDoc` matching the `*craft.CraftDoc` type used throughout `cmd/server/main.go`. Verify the import alias matches.

**Placeholder scan:** No TBDs. Every code block is complete.
