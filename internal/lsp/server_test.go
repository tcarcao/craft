package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tcarcao/craft/internal/lsp"
)

type lspMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func writeMsg(w io.Writer, msg lspMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func readMsg(r *bufio.Reader) (lspMsg, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return lspMsg{}, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Content-Length:"), " %d", &length)
		}
	}
	if length == 0 {
		return lspMsg{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return lspMsg{}, err
	}
	var msg lspMsg
	return msg, json.Unmarshal(body, &msg)
}

func TestServeLifecycle(t *testing.T) {
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
	id1 := 1

	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0",
		ID:      &id1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := readMsg(br)
	if err != nil {
		t.Fatalf("reading initialize response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize returned error: %s", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("initialize result is nil")
	}

	var result struct {
		Capabilities struct {
			TextDocumentSync       interface{} `json:"textDocumentSync"`
			CompletionProvider     interface{} `json:"completionProvider"`
			DocumentSymbolProvider interface{} `json:"documentSymbolProvider"`
			HoverProvider          interface{} `json:"hoverProvider"`
			SemanticTokensProvider interface{} `json:"semanticTokensProvider"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshaling capabilities: %v", err)
	}
	if result.Capabilities.TextDocumentSync == nil {
		t.Error("textDocumentSync capability missing")
	}
	if result.Capabilities.CompletionProvider == nil {
		t.Error("completionProvider must be declared")
	}
	// S3 capabilities (Q20 rule: declare alongside handler).
	if result.Capabilities.DocumentSymbolProvider == nil {
		t.Error("documentSymbolProvider must be declared (S3, Q20)")
	}
	if result.Capabilities.HoverProvider == nil {
		t.Error("hoverProvider must be declared (S3, Q20)")
	}
	// S4 capability.
	if result.Capabilities.SemanticTokensProvider == nil {
		t.Error("semanticTokensProvider must be declared (S4, Q20)")
	}
	if result.ServerInfo.Name != "craft-lsp" {
		t.Errorf("unexpected server name: %q", result.ServerInfo.Name)
	}

	// initialized notification (no response expected)
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params:  json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	// shutdown request
	id2 := 2
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0",
		ID:      &id2,
		Method:  "shutdown",
	}); err != nil {
		t.Fatal(err)
	}

	resp2, err := readMsg(br)
	if err != nil {
		t.Fatalf("reading shutdown response: %v", err)
	}
	if resp2.Error != nil {
		t.Fatalf("shutdown returned error: %s", resp2.Error)
	}

	// exit notification — server closes
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0",
		Method:  "exit",
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
		t.Fatal("server did not exit within 5s")
	case err := <-errCh:
		if err != nil && !isClosedPipe(err) {
			t.Fatalf("server returned unexpected error: %v", err)
		}
	}
}

func isClosedPipe(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "closed") || strings.Contains(s, "EOF") || strings.Contains(s, "broken pipe")
}

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

	// Single reader goroutine for the rest of the test. collectMsgs spawns a
	// goroutine per call and leaks it after the timer fires; calling it more
	// than once on the same bufio.Reader causes concurrent reads → panic.
	msgCh := make(chan lspMsg, 128)
	go func() {
		for {
			msg, err := readMsg(br)
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- msg
		}
	}()
	drainFor := func(dur time.Duration) []lspMsg {
		timer := time.NewTimer(dur)
		defer timer.Stop()
		var msgs []lspMsg
		for {
			select {
			case msg, ok := <-msgCh:
				if !ok {
					return msgs
				}
				msgs = append(msgs, msg)
			case <-timer.C:
				return msgs
			}
		}
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
	var gotLogTrace bool
	for _, msg := range drainFor(500 * time.Millisecond) {
		if msg.Method == "$/logTrace" {
			gotLogTrace = true
			break
		}
	}

	if !gotLogTrace {
		t.Error("expected at least one $/logTrace notification after $/setTrace verbose + didOpen, got none")
	}

	// Now set trace to "off" — subsequent didChange must NOT produce $/logTrace
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "$/setTrace", Params: json.RawMessage(`{"value":"off"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test.craft","version":2},"contentChanges":[{"text":"actor Bar"}]}`),
	}); err != nil {
		t.Fatal(err)
	}

	var sawLogTraceAfterOff bool
	for _, msg := range drainFor(300 * time.Millisecond) {
		if msg.Method == "$/logTrace" {
			sawLogTraceAfterOff = true
			break
		}
	}
	if sawLogTraceAfterOff {
		t.Error("expected no $/logTrace after $/setTrace off, but got one")
	}

	// messages level must NOT produce $/logTrace (only verbose should)
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "$/setTrace", Params: json.RawMessage(`{"value":"messages"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didChange",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test.craft","version":3},"contentChanges":[{"text":"actor Baz"}]}`),
	}); err != nil {
		t.Fatal(err)
	}
	msgs3 := drainFor(300 * time.Millisecond)
	for _, m := range msgs3 {
		if m.Method == "$/logTrace" {
			t.Error("expected no $/logTrace at trace level 'messages', but got one")
			break
		}
	}

	cancel()
}

func TestExecuteCommand_CraftExtractWorkspace(t *testing.T) {
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
	id := 1

	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id,
		Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	craftSrc := "domain Commerce {\n  Orders\n}\nservices {\n  OrderSvc {\n    contexts: Orders\n  }\n}\n"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///test.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"CRAFT_EXTRACT_WORKSPACE","arguments":[]}`),
	}); err != nil {
		t.Fatal(err)
	}

	// Read messages until we find the command response (skip diagnostics etc.)
	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading message: %v", err)
		}
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var result struct {
		Services []struct {
			Name      string   `json:"name"`
			StartLine int      `json:"startLine"`
			EndLine   int      `json:"endLine"`
			Contexts  []string `json:"contexts"`
		} `json:"services"`
		Domains []struct {
			Name    string `json:"name"`
			EndLine int    `json:"endLine"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}

	if len(result.Services) != 1 || result.Services[0].Name != "OrderSvc" {
		t.Errorf("expected 1 service OrderSvc, got %+v", result.Services)
	}
	if result.Services[0].EndLine == 0 {
		t.Error("expected non-zero EndLine for OrderSvc")
	}
	if len(result.Services[0].Contexts) == 0 || result.Services[0].Contexts[0] != "Orders" {
		t.Errorf("expected contexts=[Orders], got %v", result.Services[0].Contexts)
	}
	if len(result.Domains) != 1 || result.Domains[0].Name != "Commerce" {
		t.Errorf("expected 1 domain Commerce, got %+v", result.Domains)
	}
	if result.Domains[0].EndLine == 0 {
		t.Error("expected non-zero EndLine for Commerce domain")
	}

	cancel()
}

func mustMarshalString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestInlayHints_ServiceContextResolvesToDomain(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- lsp.Serve(ctx, serverIn, serverOut) }()
	defer testOut.Close()

	br := bufio.NewReader(testIn)
	id := 1

	// initialize
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id,
		Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}
	initResp, err := readMsg(br)
	if err != nil {
		t.Fatalf("reading initialize response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize error: %s", initResp.Error)
	}
	// Verify inlayHintProvider capability was patched in.
	var initResult map[string]interface{}
	if err := json.Unmarshal(initResp.Result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	caps, _ := initResult["capabilities"].(map[string]interface{})
	if caps["inlayHintProvider"] != true {
		t.Errorf("expected inlayHintProvider: true in capabilities, got: %v", caps["inlayHintProvider"])
	}

	// initialized
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// open document
	craftSrc := "domain Auth {\n  Login\n  Register\n}\nservices {\n  UserService {\n    contexts: Login, Register\n    language: golang\n  }\n}\n"
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///test.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	// send inlayHint request
	id = 2
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id,
		Method: "textDocument/inlayHint",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test.craft"},"range":{"start":{"line":0,"character":0},"end":{"line":100,"character":0}}}`),
	}); err != nil {
		t.Fatal(err)
	}

	// read messages until we find the response with id=2
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
		t.Fatal("timed out waiting for inlayHint response")
	}
	if resp.Error != nil {
		t.Fatalf("inlayHint returned error: %s", resp.Error)
	}

	var hints []map[string]interface{}
	if err := json.Unmarshal(resp.Result, &hints); err != nil {
		t.Fatalf("unmarshal inlayHint result: %v", err)
	}
	if len(hints) != 2 {
		t.Fatalf("expected 2 inlay hints (Login→Auth, Register→Auth), got %d: %v", len(hints), hints)
	}
	for _, h := range hints {
		label, _ := h["label"].(string)
		if label != " ← Auth" {
			t.Errorf("expected label \" ← Auth\", got %q", label)
		}
	}

	cancel()
}

// TestInlayHints_QuotedServiceContextResolvesToDomain covers the task-8a
// ripple-audit Critical 2 fix: ServiceDecl.ContextTokens() returns raw
// tokens that may be SyntaxKindString (quotes included, per the Bug 8a
// round-trip fix). handleInlayHint must read context names via
// syntax.StringAwareText, not tok.Text(), or a quoted `contexts: "Login"`
// entry silently fails to resolve against the (unquoted) resolution map and
// produces no inlay hint at all.
func TestInlayHints_QuotedServiceContextResolvesToDomain(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	defer testOut.Close()

	br := bufio.NewReader(testIn)
	id := 1

	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	// "Login" is quoted here; "Register" stays a bare ident, matching the
	// mixed-form contexts list ContextTokens()/Contexts() must both handle.
	craftSrc := "domain Auth {\n  Login\n  Register\n}\nservices {\n  UserService {\n    contexts: \"Login\", Register\n    language: golang\n  }\n}\n"
	writeMsg(testOut, lspMsg{ //nolint:errcheck
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///test_quoted_ctx.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	})
	time.Sleep(300 * time.Millisecond)

	id = 2
	writeMsg(testOut, lspMsg{ //nolint:errcheck
		JSONRPC: "2.0", ID: &id,
		Method: "textDocument/inlayHint",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///test_quoted_ctx.craft"},"range":{"start":{"line":0,"character":0},"end":{"line":100,"character":0}}}`),
	})

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
		t.Fatal("timed out waiting for inlayHint response")
	}
	if resp.Error != nil {
		t.Fatalf("inlayHint returned error: %s", resp.Error)
	}

	var hints []map[string]interface{}
	if err := json.Unmarshal(resp.Result, &hints); err != nil {
		t.Fatalf("unmarshal inlayHint result: %v", err)
	}
	// Both the quoted "Login" and bare Register context must resolve to Auth.
	if len(hints) != 2 {
		t.Fatalf("expected 2 inlay hints (Login→Auth, Register→Auth), got %d: %v", len(hints), hints)
	}
	for _, h := range hints {
		label, _ := h["label"].(string)
		if label != " ← Auth" {
			t.Errorf("expected label \" ← Auth\", got %q", label)
		}
	}

	cancel()
}

func TestExtractDslFromBlockRanges_GroupedService(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: services {
	// Line 2:   OrderSvc {
	// Line 3:     contexts: Orders
	// Line 4:   }
	// Line 5: }
	craftSrc := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///grouped.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	// startLine:2 endLine:4 — the OrderSvc block (name line through closing })
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///grouped.craft","startLine":2,"endLine":4}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if !strings.Contains(dsl, "services {") {
		t.Errorf("expected grouped service to be wrapped in 'services {', got:\n%s", dsl)
	}
	if !strings.Contains(dsl, "OrderSvc") {
		t.Errorf("expected service name OrderSvc in output, got:\n%s", dsl)
	}
	if !strings.Contains(dsl, "contexts: Orders") {
		t.Errorf("expected contexts: Orders in output, got:\n%s", dsl)
	}
	cancel()
}

func TestExtractDslFromBlockRanges_TopLevelService(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: service OrderSvc {
	// Line 2:   contexts: Orders
	// Line 3: }
	craftSrc := "service OrderSvc {\n  contexts: Orders\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///toplevel.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	// startLine:1 endLine:3 — the OrderSvc block
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///toplevel.craft","startLine":1,"endLine":3}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if !strings.Contains(dsl, "service OrderSvc") {
		t.Errorf("expected top-level 'service OrderSvc' in output, got:\n%s", dsl)
	}
	// Must NOT be wrapped in services { }
	if strings.HasPrefix(strings.TrimSpace(dsl), "services {") {
		t.Errorf("expected top-level form (not wrapped in services { }), got:\n%s", dsl)
	}
	cancel()
}

func TestExtractDslFromBlockRanges_GroupedDomain(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: domains {
	// Line 2:   Commerce {
	// Line 3:     Orders
	// Line 4:   }
	// Line 5: }
	craftSrc := "domains {\n  Commerce {\n    Orders\n  }\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///domains.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	// startLine:2 endLine:4 — the Commerce block
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///domains.craft","startLine":2,"endLine":4}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if !strings.Contains(dsl, "domains {") {
		t.Errorf("expected grouped domain to be wrapped in 'domains {', got:\n%s", dsl)
	}
	if !strings.Contains(dsl, "Commerce") {
		t.Errorf("expected domain name Commerce in output, got:\n%s", dsl)
	}
	cancel()
}

func TestExtractDslFromBlockRanges_TopLevelDomain(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: domain Commerce {
	// Line 2:   Orders
	// Line 3: }
	craftSrc := "domain Commerce {\n  Orders\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///topdomain.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///topdomain.craft","startLine":1,"endLine":3}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if !strings.Contains(dsl, "domain Commerce") {
		t.Errorf("expected top-level 'domain Commerce' in output, got:\n%s", dsl)
	}
	if strings.HasPrefix(strings.TrimSpace(dsl), "domains {") {
		t.Errorf("expected top-level form (not wrapped in domains { }), got:\n%s", dsl)
	}
	cancel()
}

func TestExtractDslFromBlockRanges_OutOfBoundsRangeReturnsEmpty(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	craftSrc := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///skip.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	// startLine:99 endLine:100 — no declaration at these lines
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///skip.craft","startLine":99,"endLine":100}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if strings.TrimSpace(dsl) != "" {
		t.Errorf("expected empty result for unmatched range, got: %q", dsl)
	}
	cancel()
}

func TestExtractDslFromBlockRanges_UnmatchedInBoundsRangeFallsBackToRawLines(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: services {
	// Line 2:   OrderSvc {
	// Line 3:     contexts: Orders
	// Line 4:   }
	// Line 5: }
	// OrderSvc has Line=2, EndLine=4. Requesting startLine:3, endLine:4 does not
	// match any service or domain declaration, so the fallback returns raw lines.
	craftSrc := "services {\n  OrderSvc {\n    contexts: Orders\n  }\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///fallback.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	// startLine:3 endLine:4 — in-bounds but does not match any AST declaration
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"craft.extractDslFromBlockRanges","arguments":[[{"fileUri":"file:///fallback.craft","startLine":3,"endLine":4}]]}`),
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
		if msg.ID != nil && *msg.ID == cmdID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for command response")
	}
	if resp.Error != nil {
		t.Fatalf("command returned error: %s", resp.Error)
	}

	var dsl string
	if err := json.Unmarshal(resp.Result, &dsl); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	// The fallback should return the raw lines for the range (lines 3-4 of the file),
	// which is non-empty content (not an AST-reconstructed block).
	if strings.TrimSpace(dsl) == "" {
		t.Errorf("expected non-empty raw-line fallback for in-bounds unmatched range, got empty string")
	}
	// The fallback must NOT produce a reconstructed services { } wrapper.
	if strings.Contains(dsl, "services {") {
		t.Errorf("expected raw-line fallback (no services { } wrapper), got:\n%s", dsl)
	}
	// The raw lines should contain content from lines 3 and 4.
	if !strings.Contains(dsl, "contexts: Orders") {
		t.Errorf("expected raw line content 'contexts: Orders' in fallback output, got:\n%s", dsl)
	}
	cancel()
}

// TestSemanticTokens_ColumnTracking verifies that the relative-encoded uint32
// data returned by textDocument/semanticTokens/full has correct startChar values
// for actors, domain names, bounded contexts, and services. This guards against
// regressions where all tokens are emitted at column 0.
func TestSemanticTokens_ColumnTracking(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1:  actors {
	// Line 2:      user Alice        — Alice at col 10 (1-based) → startChar 9
	// Line 3:      system Bob        — Bob   at col 12           → startChar 11
	// Line 4:  }
	// Line 5:  domain Commerce {     — Commerce at col 8         → startChar 7
	// Line 6:      Orders            — Orders   at col 5         → startChar 4
	// Line 7:      Payments          — Payments at col 5         → startChar 4
	// Line 8:  }
	// Line 9:  service MyService {   — MyService at col 9        → startChar 8
	// Line 10:     contexts: Orders
	// Line 11: }
	craftSrc := "actors {\n    user Alice\n    system Bob\n}\ndomain Commerce {\n    Orders\n    Payments\n}\nservice MyService {\n    contexts: Orders\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///tokens.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///tokens.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling semantic tokens result: %v", err)
	}

	if len(result.Data)%5 != 0 {
		t.Fatalf("data length %d is not a multiple of 5", len(result.Data))
	}

	// Decode relative-encoded stream into absolute (line, startChar, length, tokenType, modifiers) tuples.
	type absToken struct {
		line, startChar, length, tokenType, modifiers uint32
	}
	var decoded []absToken
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		deltaLine := result.Data[i]
		deltaChar := result.Data[i+1]
		length := result.Data[i+2]
		tokenType := result.Data[i+3]
		modifiers := result.Data[i+4]
		curLine += deltaLine
		if deltaLine > 0 {
			curChar = deltaChar
		} else {
			curChar += deltaChar
		}
		decoded = append(decoded, absToken{curLine, curChar, length, tokenType, modifiers})
	}

	// Build a lookup map: (line, startChar) → absToken for quick assertions.
	byPos := make(map[[2]uint32]absToken, len(decoded))
	for _, tok := range decoded {
		byPos[[2]uint32{tok.line, tok.startChar}] = tok
	}

	// Expected ident tokens (absolute 0-based positions).
	// Pass 1 now also emits keyword/punctuation tokens, so total count > 6.
	// We assert only the 6 entity-name tokens produced by Pass 2.
	//
	//   Alice     line=1 col=9  len=5  type=0(craft-actor-definition) mod=1
	//   Bob       line=2 col=11 len=3  type=0(craft-actor-definition) mod=1
	//   Commerce  line=4 col=7  len=8  type=2(craft-domain-name)      mod=1
	//   Orders    line=5 col=4  len=6  type=3(craft-context-name)     mod=1
	//   Payments  line=6 col=4  len=8  type=3(craft-context-name)     mod=1
	//   MyService line=8 col=8  len=9  type=4(craft-service-name)     mod=1
	wantTokens := []struct {
		name                           string
		line, col, length, ttype, mods uint32
	}{
		{"Alice", 1, 9, 5, 0, 1},
		{"Bob", 2, 11, 3, 0, 1},
		{"Commerce", 4, 7, 8, 2, 1},
		{"Orders(bc)", 5, 4, 6, 3, 1},
		{"Payments(bc)", 6, 4, 8, 3, 1},
		{"MyService", 8, 8, 9, 4, 1},
	}

	for _, w := range wantTokens {
		tok, ok := byPos[[2]uint32{w.line, w.col}]
		if !ok {
			t.Errorf("token %s (line=%d col=%d): not found in decoded stream (total tokens=%d)", w.name, w.line, w.col, len(decoded))
			continue
		}
		if tok.length != w.length || tok.tokenType != w.ttype || tok.modifiers != w.mods {
			t.Errorf("token %s (line=%d col=%d): got [len=%d type=%d mods=%d] want [len=%d type=%d mods=%d]",
				w.name, w.line, w.col,
				tok.length, tok.tokenType, tok.modifiers,
				w.length, w.ttype, w.mods,
			)
		}
	}

	cancel()
}

// TestSemanticTokens_TriggerAndActionColumns verifies that semantic tokens for
// use-case trigger actors and action subjects have the correct startChar offset
// (not hardcoded to column 0). This guards the fix for the bug where the token
// started at the beginning of the line, overlapping the `when` keyword.
func TestSemanticTokens_TriggerAndActionColumns(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Layout (1-based lines, 1-based columns):
	// L1: actor user Alice          — Alice at col 11, len 5
	// L2: domain Auth {             — Auth  at col 8,  len 4
	// L3:   Login                   — Login at col 3 (bc)
	// L4: }
	// L5: use_case "Login" {
	// L6:   when Alice initiates x  — Alice (trigger) at col 8, len 5
	// L7:     Auth validates creds  — Auth  (action)  at col 5, len 4
	// L8: }
	const craftSrc = "actor user Alice\ndomain Auth {\n  Login\n}\nuse_case \"Login\" {\n  when Alice initiates x\n    Auth validates creds\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///trigger_cols.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///trigger_cols.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling semantic tokens result: %v", err)
	}

	// Decode the relative-encoded stream into absolute (line, startChar) tuples.
	type tok struct {
		line, startChar, length, tokenType uint32
	}
	var tokens []tok
	var absLine, absChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3]
		absLine += dl
		if dl == 0 {
			absChar += dc
		} else {
			absChar = dc
		}
		tokens = append(tokens, tok{absLine, absChar, ln, tt})
	}

	find := func(label string, wantLine, wantStart, wantLen uint32) {
		t.Helper()
		for _, tk := range tokens {
			if tk.line == wantLine && tk.length == wantLen {
				if tk.startChar != wantStart {
					t.Errorf("%s: startChar = %d, want %d (line %d, len %d); all tokens: %v",
						label, tk.startChar, wantStart, wantLine, wantLen, tokens)
				}
				return
			}
		}
		t.Errorf("%s: no token found at 0-based line %d with length %d; all tokens: %v", label, wantLine, wantLen, tokens)
	}

	// "  when Alice initiates x" — "Alice" is at 1-based col 8 → 0-based startChar 7, on 0-based line 5.
	find("Alice(trigger)", 5, 7, 5)
	// "    Auth validates creds" — "Auth" is at 1-based col 5 → 0-based startChar 4, on 0-based line 6.
	find("Auth(action)", 6, 4, 4)

	cancel()
}

func TestSemanticTokens_TargetContext(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Layout (1-based lines, 1-based columns):
	// L1:  actor user Alice            Alice at col 12, len 5, type=craft-actor-definition(0), mod=1
	// L2:  domain Auth {               Auth  at col 8,  len 4, type=craft-domain-name(2),      mod=1(decl)
	// L3:    Login                     Login at col 3,  len 5, type=craft-context-name(3),      mod=1(decl)
	// L4:  }
	// L5:  domain Profile {            Profile at col 8, len 7, type=craft-domain-name(2),      mod=1(decl)
	// L6:    UserData                  UserData at col 3, len 8, type=craft-context-name(3),    mod=1(decl)
	// L7:  }
	// L8:  use_case "Validate" {
	// L9:    when Alice initiates x    Alice (trigger) at col 8, len 5, type=craft-actor-name(1)
	// L10:   Auth asks Profile to y   Auth at col 5, len 4, type=craft-domain-name(2)
	//                                 Profile at col 15, len 7, type=craft-domain-name(2)  ← NEW
	// L11: }
	const craftSrc = "actor user Alice\ndomain Auth {\n  Login\n}\ndomain Profile {\n  UserData\n}\nuse_case \"Validate\" {\n  when Alice initiates x\n    Auth asks Profile to y\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///target_ctx.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///target_ctx.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}

	// Decode relative-encoded stream into absolute tokens.
	type tok struct{ line, startChar, length, tokenType uint32 }
	var tokens []tok
	var absLine, absChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3]
		absLine += dl
		if dl == 0 {
			absChar += dc
		} else {
			absChar = dc
		}
		tokens = append(tokens, tok{absLine, absChar, ln, tt})
	}

	// Verify Profile (TargetContext) appears on line 9 (0-based), col 14 (0-based), len 7, type=craft-domain-name(2).
	foundProfile := false
	for _, tok := range tokens {
		if tok.line == 9 && tok.startChar == 14 && tok.length == 7 && tok.tokenType == 2 {
			foundProfile = true
		}
	}
	if !foundProfile {
		t.Errorf("expected Profile token at line=9 col=14 len=7 type=2; got tokens: %v", tokens)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

// TestSemanticTokens_StringTypes verifies that use-case title strings are emitted
// as craft-usecase-string (index 30) and event strings in notifies/listens actions
// are emitted as craft-event-string (index 31), rather than craft-regular-string (32).
func TestSemanticTokens_StringTypes(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0 (1-based: 1): use_case "Register User" {
	//   "Register User" starts at col 9 (0-based), length 15 (including quotes)
	// Line 1 (1-based: 2):   when SomeActor does Something
	// Line 2 (1-based: 3):     SomeContext notifies "UserRegistered"
	//   "UserRegistered" starts at col 22 (0-based), length 16 (including quotes)
	// Line 3 (1-based: 4): }
	const craftSrc = "use_case \"Register User\" {\n  when SomeActor does Something\n    SomeContext notifies \"UserRegistered\"\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///strtypes.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///strtypes.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling semantic tokens result: %v", err)
	}

	// Decode relative-encoded stream into absolute tokens.
	type tok struct{ line, startChar, length, tokenType uint32 }
	var tokens []tok
	var absLine, absChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3]
		absLine += dl
		if dl == 0 {
			absChar += dc
		} else {
			absChar = dc
		}
		tokens = append(tokens, tok{absLine, absChar, ln, tt})
	}

	// Build lookup map: (line, startChar) → tok.
	byPos := make(map[[2]uint32]tok, len(tokens))
	for _, tk := range tokens {
		byPos[[2]uint32{tk.line, tk.startChar}] = tk
	}

	// use_case "Register User" {
	// The lexer emits the string content token (without surrounding quotes) starting
	// at the position of the opening quote. col=9 (0-based), length=13 ("Register User").
	// Token type must be craft-usecase-string (index 30).
	if tk, ok := byPos[[2]uint32{0, 9}]; !ok {
		t.Errorf("expected token at line=0, col=9 (use-case title string); all tokens: %v", tokens)
	} else if tk.tokenType != 30 {
		t.Errorf("use-case title string: tokenType=%d, want 30 (craft-usecase-string); all tokens: %v", tk.tokenType, tokens)
	}

	//     SomeContext notifies "UserRegistered"
	// The event string token starts at the opening quote col=25 (0-based after 4-space
	// indent + "SomeContext notifies "), length=14 ("UserRegistered" without quotes).
	// Token type must be craft-event-string (index 31).
	if tk, ok := byPos[[2]uint32{2, 25}]; !ok {
		t.Errorf("expected token at line=2, col=25 (event string); all tokens: %v", tokens)
	} else if tk.tokenType != 31 {
		t.Errorf("event string: tokenType=%d, want 31 (craft-event-string); all tokens: %v", tk.tokenType, tokens)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

func TestSemanticTokens_ExposureAndArch(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: exposure PaymentAPI {    — PaymentAPI at col 9 (0-based), length 10
	// Line 1:   to: external
	// Line 2: }
	// Line 3: arch MyArch {
	// Line 4:   presentation:
	// Line 5:     Gateway             — Gateway at col 4 (0-based), length 7
	// Line 6: }
	// Line 7: actors {
	// Line 8:   partner Alice        — partner (open-taxonomy actor type) at col 2, length 7 → tokenType 28 (craft-actor-type)
	// Line 9: }
	const craftSrc = "exposure PaymentAPI {\n  to: external\n}\narch MyArch {\n  presentation:\n    Gateway\n}\nactors {\n  partner Alice\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///exparch.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///exparch.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling semantic tokens result: %v", err)
	}

	// Decode relative-encoded stream into absolute tokens.
	type tok struct{ line, startChar, length, tokenType uint32 }
	var tokens []tok
	var absLine, absChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3]
		absLine += dl
		if dl == 0 {
			absChar += dc
		} else {
			absChar = dc
		}
		tokens = append(tokens, tok{absLine, absChar, ln, tt})
	}

	// Build lookup map: (line, startChar) → tok.
	byPos := make(map[[2]uint32]tok, len(tokens))
	for _, tk := range tokens {
		byPos[[2]uint32{tk.line, tk.startChar}] = tk
	}

	// exposure PaymentAPI { — PaymentAPI at line=0, col=9, length=10, tokenType=6 (craft-exposure-name)
	if tk, ok := byPos[[2]uint32{0, 9}]; !ok {
		t.Errorf("expected token at line=0, col=9 (exposure name PaymentAPI); all tokens: %v", tokens)
	} else if tk.tokenType != 6 {
		t.Errorf("exposure name: tokenType=%d, want 6 (craft-exposure-name); all tokens: %v", tk.tokenType, tokens)
	}

	//     Gateway — line=5, col=4, length=7, tokenType=5 (craft-component-name)
	if tk, ok := byPos[[2]uint32{5, 4}]; !ok {
		t.Errorf("expected token at line=5, col=4 (arch component Gateway); all tokens: %v", tokens)
	} else if tk.tokenType != 5 {
		t.Errorf("arch component name: tokenType=%d, want 5 (craft-component-name); all tokens: %v", tk.tokenType, tokens)
	}

	// partner (open-taxonomy actor type) at line=8, col=2, len=7 → tokenType 28 (craft-actor-type)
	if tok, ok := byPos[[2]uint32{8, 2}]; !ok {
		t.Error("partner not found at expected position (line=8, col=2)")
	} else if tok.tokenType != 28 {
		t.Errorf("partner: got tokenType %d, want 28 (craft-actor-type)", tok.tokenType)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

// TestSemanticTokens_ServiceBody verifies that data store names, language values,
// deployment types, and deployment targets are classified correctly.
func TestSemanticTokens_ServiceBody(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- lsp.Serve(ctx, serverIn, serverOut) }()
	defer testOut.Close()
	br := bufio.NewReader(testIn)
	id := 1
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 1: service PaymentSvc {
	// Line 2:   contexts: Billing
	// Line 3:   data-stores: payment_db
	// Line 4:   language: golang
	// Line 5:   deployment: canary(50% -> stable, 50% -> canary)
	// Line 6: }
	craftSrc := "service PaymentSvc {\n  contexts: Billing\n  data-stores: payment_db\n  language: golang\n  deployment: canary(50% -> stable, 50% -> canary)\n}"
	id++
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":"file:///svc.craft","languageId":"craft","version":1,"text":%s}}`, mustMarshalString(craftSrc)))}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &tokID, Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///svc.craft"}}`)}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out")
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	type absToken struct{ line, startChar, length, tokenType, modifiers uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt, mod := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3], result.Data[i+4]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt, mod}
	}

	// payment_db at line 2, col 15, len 10 → tokenType 7 (craft-data-store-name)
	if tok, ok := byPos[[2]uint32{2, 15}]; !ok {
		t.Error("payment_db not found at line 2 col 15")
	} else if tok.tokenType != 7 {
		t.Errorf("payment_db: got tokenType %d, want 7 (craft-data-store-name)", tok.tokenType)
	}

	// golang at line 3, col 12, len 6 → tokenType 44 (craft-language-value)
	if tok, ok := byPos[[2]uint32{3, 12}]; !ok {
		t.Error("golang not found at line 3 col 12")
	} else if tok.tokenType != 44 {
		t.Errorf("golang: got tokenType %d, want 44 (craft-language-value)", tok.tokenType)
	}

	// canary at line 4, col 14, len 6 → tokenType 41 (craft-deployment-type)
	if tok, ok := byPos[[2]uint32{4, 14}]; !ok {
		t.Error("canary (deployment type) not found at line 4 col 14")
	} else if tok.tokenType != 41 {
		t.Errorf("canary: got tokenType %d, want 41 (craft-deployment-type)", tok.tokenType)
	}

	// stable at line 4, col 28, len 6 → tokenType 42 (craft-deployment-target)
	if tok, ok := byPos[[2]uint32{4, 28}]; !ok {
		t.Error("stable not found at line 4 col 28")
	} else if tok.tokenType != 42 {
		t.Errorf("stable: got tokenType %d, want 42 (craft-deployment-target)", tok.tokenType)
	}

	// canary (target) at line 4, col 43, len 6 → tokenType 42 (craft-deployment-target)
	if tok, ok := byPos[[2]uint32{4, 43}]; !ok {
		t.Error("canary (target) not found at line 4 col 43")
	} else if tok.tokenType != 42 {
		t.Errorf("canary (target): got tokenType %d, want 42 (craft-deployment-target)", tok.tokenType)
	}
}

func TestDefinition_TargetContextColumnAware(t *testing.T) {
	// Fixture (1-based lines):
	// L1:  domain Auth {               — Auth domain at line 1
	// L2:    Login
	// L3:  }
	// L4:  domain Profile {            — Profile domain at line 4
	// L5:    UserData
	// L6:  }
	// L7:  use_case "T" {
	// L8:    when Login initiates x
	// L9:      Auth asks Profile to y  — Auth at col 7, Profile at col 17
	// L10: }
	const craftSrc = "domain Auth {\n  Login\n}\ndomain Profile {\n  UserData\n}\nuse_case \"T\" {\n  when Login initiates x\n    Auth asks Profile to y\n}"

	tests := []struct {
		name       string
		cursorLine uint32 // 0-based LSP
		cursorChar uint32 // 0-based LSP
		wantLine   uint32 // 0-based LSP line of expected definition location
		wantChar   uint32 // 0-based LSP start character of expected definition location
	}{
		// Cursor on "Auth" (col 4, 0-based) → navigate to Auth domain (line 0, char 7 — after "domain ")
		{name: "from-party", cursorLine: 8, cursorChar: 4, wantLine: 0, wantChar: 7},
		// Cursor on "Profile" (col 14, 0-based) → navigate to Profile domain (line 3, char 7 — after "domain ")
		{name: "to-party", cursorLine: 8, cursorChar: 14, wantLine: 3, wantChar: 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverIn, testOut := io.Pipe()
			testIn, serverOut := io.Pipe()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer testOut.Close()

			go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
			br := bufio.NewReader(testIn)

			id := 1
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
				Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
			readMsg(br)                                                                                     //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

			const uri = "file:///def_colaware.craft"
			openParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
				},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
			time.Sleep(300 * time.Millisecond)

			id++
			defID := id
			defParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": tc.cursorLine, "character": tc.cursorChar},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

			var defResp lspMsg
			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) {
				msg, err := readMsg(br)
				if err != nil {
					t.Fatalf("reading message: %v", err)
				}
				if msg.ID != nil && *msg.ID == defID {
					defResp = msg
					break
				}
			}
			if defResp.ID == nil {
				t.Fatal("timed out waiting for definition response")
			}
			if defResp.Error != nil {
				t.Fatalf("definition returned error: %s", defResp.Error)
			}

			var locs []struct {
				Range struct {
					Start struct {
						Line      uint32 `json:"line"`
						Character uint32 `json:"character"`
					} `json:"start"`
				} `json:"range"`
			}
			if err := json.Unmarshal(defResp.Result, &locs); err != nil {
				t.Fatalf("unmarshalling definition result: %v", err)
			}
			if len(locs) == 0 {
				t.Fatalf("got empty locations; cursor at (%d,%d)", tc.cursorLine, tc.cursorChar)
			}
			if locs[0].Range.Start.Line != tc.wantLine {
				t.Errorf("definition line: got %d want %d", locs[0].Range.Start.Line, tc.wantLine)
			}
			if locs[0].Range.Start.Character != tc.wantChar {
				t.Errorf("definition character: got %d want %d", locs[0].Range.Start.Character, tc.wantChar)
			}

			id2 := 99
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
			readMsg(br)                                                             //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
		})
	}
}

func TestDefinition_BoundedContextOwnLine(t *testing.T) {
	// Verify that clicking a BC name navigates to the BC's own line inside the
	// domain body, not to the parent domain keyword line.
	//
	// L1:  domain Auth {               — domain line (0-based: 0)
	// L2:    Login                     — Login BC at line 2 (0-based: 1)
	// L3:    Session                   — Session BC at line 3 (0-based: 2)
	// L4:  }
	// L5:  use_case "T" {
	// L6:    when Login initiates x
	// L7:      Session validates tok   — action.Context = Session, line 7 (0-based: 6)
	// L8:  }
	const craftSrc = "domain Auth {\n  Login\n  Session\n}\nuse_case \"T\" {\n  when Login initiates x\n    Session validates tok\n}"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///bc_own_line.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	id++
	defID := id
	// Cursor on "Session" in the action line (line 6, col 4).
	defParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 6, "character": 4},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

	var defResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading message: %v", err)
		}
		if msg.ID != nil && *msg.ID == defID {
			defResp = msg
			break
		}
	}
	if defResp.ID == nil {
		t.Fatal("timed out waiting for definition response")
	}
	if defResp.Error != nil {
		t.Fatalf("definition returned error: %s", defResp.Error)
	}

	var locs []struct {
		Range struct {
			Start struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(defResp.Result, &locs); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("got empty locations")
	}
	// Session is on line 3 (1-based) = line 2 (0-based LSP).
	if locs[0].Range.Start.Line != 2 {
		t.Errorf("BC navigation line: got %d want 2 (Session's own line)", locs[0].Range.Start.Line)
	}
	// Session starts at column 3 (1-based, after 2-space indent) = character 2 (0-based LSP).
	if locs[0].Range.Start.Character != 2 {
		t.Errorf("BC navigation character: got %d want 2 (Session's own column)", locs[0].Range.Start.Character)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

// TestSemanticTokens_VerbsAndPhrases verifies that action verbs, connector words,
// and phrase words in use-case bodies are classified with correct craft-* types.
func TestSemanticTokens_VerbsAndPhrases(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: use_case "Test" {
	// Line 1:   when SomeActor does Something
	// Line 2:     Auth validates email format
	// Line 3: }
	// col math (0-based): "    Auth validates email format"
	//   col 4: Auth (len 4)
	//   col 9: validates (len 9)
	//   col 19: email (len 5)
	//   col 25: format (len 6)
	craftSrc := "use_case \"Test\" {\n  when SomeActor does Something\n    Auth validates email format\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///verbs.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///verbs.craft"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out")
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	type absToken struct{ line, startChar, length, tokenType, modifiers uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt, mod := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3], result.Data[i+4]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt, mod}
	}

	// validates at line 2, col 9, len 9 → tokenType 27 (craft-regular-verb)
	if tok, ok := byPos[[2]uint32{2, 9}]; !ok {
		t.Error("validates not found at line 2 col 9")
	} else if tok.tokenType != 27 {
		t.Errorf("validates: got tokenType %d, want 27 (craft-regular-verb)", tok.tokenType)
	}

	// email at line 2, col 19, len 5 → tokenType 33 (craft-phrase-word)
	if tok, ok := byPos[[2]uint32{2, 19}]; !ok {
		t.Error("email not found at line 2 col 19")
	} else if tok.tokenType != 33 {
		t.Errorf("email: got tokenType %d, want 33 (craft-phrase-word)", tok.tokenType)
	}

	// format at line 2, col 25, len 6 → tokenType 33 (craft-phrase-word)
	if tok, ok := byPos[[2]uint32{2, 25}]; !ok {
		t.Error("format not found at line 2 col 25")
	} else if tok.tokenType != 33 {
		t.Errorf("format: got tokenType %d, want 33 (craft-phrase-word)", tok.tokenType)
	}

	t.Run("sync_action", func(t *testing.T) {
		sIn, tOut2 := io.Pipe()
		tIn2, sOut2 := io.Pipe()

		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		go func() { lsp.Serve(ctx2, sIn, sOut2) }() //nolint:errcheck
		defer tOut2.Close()

		br2 := bufio.NewReader(tIn2)
		id2 := 1

		if err := writeMsg(tOut2, lspMsg{
			JSONRPC: "2.0", ID: &id2, Method: "initialize",
			Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := readMsg(br2); err != nil {
			t.Fatal(err)
		}
		if err := writeMsg(tOut2, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
			t.Fatal(err)
		}

		// Line 0: use_case "SyncTest" {
		// Line 1:   when SomeActor does SomeThing
		// Line 2:     Auth asks Database to verify identity
		// Line 3: }
		// col math (0-based): "    Auth asks Database to verify identity"
		//   col 4:  Auth     (len 4)  — subject
		//   col 9:  asks     (len 4)  — keyword craft-asks-verb (Pass 1, SyntaxKindKwAsks)
		//   col 14: Database (len 8)  — target
		//   col 23: to       (len 2)  — SyntaxKindKwTo → craft-connector-word (index 29)
		//   col 26: verify   (len 6)  — phrase word → craft-phrase-word (index 33)
		//   col 33: identity (len 8)  — phrase word → craft-phrase-word (index 33)
		syncSrc := "use_case \"SyncTest\" {\n  when SomeActor does SomeThing\n    Auth asks Database to verify identity\n}"
		id2++
		if err := writeMsg(tOut2, lspMsg{
			JSONRPC: "2.0", Method: "textDocument/didOpen",
			Params: json.RawMessage(fmt.Sprintf(
				`{"textDocument":{"uri":"file:///sync.craft","languageId":"craft","version":1,"text":%s}}`,
				mustMarshalString(syncSrc),
			)),
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond)

		id2++
		tokID2 := id2
		if err := writeMsg(tOut2, lspMsg{
			JSONRPC: "2.0", ID: &tokID2,
			Method: "textDocument/semanticTokens/full",
			Params: json.RawMessage(`{"textDocument":{"uri":"file:///sync.craft"}}`),
		}); err != nil {
			t.Fatal(err)
		}

		var resp2 lspMsg
		deadline2 := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline2) {
			msg, err := readMsg(br2)
			if err != nil {
				t.Fatalf("reading: %v", err)
			}
			if msg.ID != nil && *msg.ID == tokID2 {
				resp2 = msg
				break
			}
		}
		if resp2.ID == nil {
			t.Fatal("timed out waiting for sync_action token response")
		}

		var result2 struct {
			Data []uint32 `json:"data"`
		}
		if err := json.Unmarshal(resp2.Result, &result2); err != nil {
			t.Fatal(err)
		}

		type absToken2 struct{ line, startChar, length, tokenType, modifiers uint32 }
		byPos2 := make(map[[2]uint32]absToken2)
		var curLine2, curChar2 uint32
		for i := 0; i+4 < len(result2.Data); i += 5 {
			dl, dc, ln, tt, mod := result2.Data[i], result2.Data[i+1], result2.Data[i+2], result2.Data[i+3], result2.Data[i+4]
			curLine2 += dl
			if dl > 0 {
				curChar2 = dc
			} else {
				curChar2 += dc
			}
			byPos2[[2]uint32{curLine2, curChar2}] = absToken2{curLine2, curChar2, ln, tt, mod}
		}

		// "to" at line 2, col 23, len 2 → tokenType 29 (craft-connector-word)
		// classified via SyntaxKindKwTo path in sync_action branch
		if tok, ok := byPos2[[2]uint32{2, 23}]; !ok {
			t.Error("to not found at line 2 col 23")
		} else if tok.tokenType != 29 {
			t.Errorf("to: got tokenType %d, want 29 (craft-connector-word)", tok.tokenType)
		}

		// "verify" at line 2, col 26, len 6 → tokenType 33 (craft-phrase-word)
		if tok, ok := byPos2[[2]uint32{2, 26}]; !ok {
			t.Error("verify not found at line 2 col 26")
		} else if tok.tokenType != 33 {
			t.Errorf("verify: got tokenType %d, want 33 (craft-phrase-word)", tok.tokenType)
		}

		// "identity" at line 2, col 33, len 8 → tokenType 33 (craft-phrase-word)
		if tok, ok := byPos2[[2]uint32{2, 33}]; !ok {
			t.Error("identity not found at line 2 col 33")
		} else if tok.tokenType != 33 {
			t.Errorf("identity: got tokenType %d, want 33 (craft-phrase-word)", tok.tokenType)
		}
	})
}

// TestSemanticTokens_SlugTarget is a regression test for a bug where
// classifyActionIdents (sync_action case) hardcoded the target width to a
// single token (start := 3), so a typed node-slug target (e.g.
// "bc:re/billing", which flattens to 5 leaf tokens: bc, :, re, /, billing)
// caused classification to start mid-target, mis-emitting the ref's own
// tokens (re, billing) as craft-phrase-word and missing the real connector
// ("for") that follows the ref. Phrase classification must start at the
// same token as ActionDecl.PhraseText() (see phraseStartIndex in
// internal/syntax/ast.go).
func TestSemanticTokens_SlugTarget(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: use_case "SlugTarget" {
	// Line 1:   when SomeActor does SomeThing
	// Line 2:     Subscriptions asks bc:re/billing for a fresh charge attempt
	// Line 3: }
	// col math (0-based): "    Subscriptions asks bc:re/billing for a fresh charge attempt"
	//   col 4:  Subscriptions (len 13) — subject
	//   col 18: asks          (len 4)  — keyword craft-asks-verb (Pass 1)
	//   col 23: bc            (len 2)  — ref target (kind word), NOT craft-phrase-word
	//   col 25: :             (len 1)  — ref punctuation
	//   col 26: re            (len 2)  — ref target (namespace segment), NOT craft-phrase-word
	//   col 28: /             (len 1)  — ref punctuation
	//   col 29: billing       (len 7)  — ref target (name segment), NOT craft-phrase-word
	//   col 37: for           (len 3)  — connector → craft-connector-word (index 29)
	//   col 43: fresh         (len 5)  — phrase word → craft-phrase-word (index 33)
	//   col 49: charge        (len 6)  — phrase word → craft-phrase-word (index 33)
	//   col 56: attempt       (len 7)  — phrase word → craft-phrase-word (index 33)
	slugSrc := "use_case \"SlugTarget\" {\n  when SomeActor does SomeThing\n    Subscriptions asks bc:re/billing for a fresh charge attempt\n}"
	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///slug_target.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(slugSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///slug_target.craft"}}`),
	}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for slug target token response")
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	type absToken struct{ line, startChar, length, tokenType uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt}
	}

	const craftPhraseWord = 33
	const craftConnectorWord = 29

	// "re" at line 2, col 26 must NOT be craft-phrase-word (it is part of the
	// ref target, not the free-text phrase).
	if tok, ok := byPos[[2]uint32{2, 26}]; ok && tok.tokenType == craftPhraseWord {
		t.Errorf("re: got tokenType %d (craft-phrase-word), want anything else (it is a ref target token)", tok.tokenType)
	}

	// "billing" at line 2, col 29 must NOT be craft-phrase-word.
	if tok, ok := byPos[[2]uint32{2, 29}]; ok && tok.tokenType == craftPhraseWord {
		t.Errorf("billing: got tokenType %d (craft-phrase-word), want anything else (it is a ref target token)", tok.tokenType)
	}

	// "for" at line 2, col 37, len 3 → connector word, tokenType 29.
	if tok, ok := byPos[[2]uint32{2, 37}]; !ok {
		t.Error("for not found at line 2 col 37")
	} else if tok.tokenType != craftConnectorWord {
		t.Errorf("for: got tokenType %d, want %d (craft-connector-word)", tok.tokenType, craftConnectorWord)
	}

	// "fresh" at line 2, col 43, len 5 → phrase word, tokenType 33.
	if tok, ok := byPos[[2]uint32{2, 43}]; !ok {
		t.Error("fresh not found at line 2 col 43")
	} else if tok.tokenType != craftPhraseWord {
		t.Errorf("fresh: got tokenType %d, want %d (craft-phrase-word)", tok.tokenType, craftPhraseWord)
	}

	// "charge" at line 2, col 49, len 6 → phrase word, tokenType 33.
	if tok, ok := byPos[[2]uint32{2, 49}]; !ok {
		t.Error("charge not found at line 2 col 49")
	} else if tok.tokenType != craftPhraseWord {
		t.Errorf("charge: got tokenType %d, want %d (craft-phrase-word)", tok.tokenType, craftPhraseWord)
	}

	// "attempt" at line 2, col 56, len 7 → phrase word, tokenType 33.
	if tok, ok := byPos[[2]uint32{2, 56}]; !ok {
		t.Error("attempt not found at line 2 col 56")
	} else if tok.tokenType != craftPhraseWord {
		t.Errorf("attempt: got tokenType %d, want %d (craft-phrase-word)", tok.tokenType, craftPhraseWord)
	}
}

// TestSemanticTokens_DomainListenTriggerColumn is a regression test for a bug
// where domain_listen trigger subjects (e.g. "VASScheduling" in
// "when VASScheduling listens ...") were emitted at col 0 instead of their
// actual column, because ActorCol() guards on Kind()!="external".
func TestSemanticTokens_DomainListenTriggerColumn(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- lsp.Serve(ctx, serverIn, serverOut) }()
	defer testOut.Close()
	br := bufio.NewReader(testIn)
	id := 1
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: domain Biz {
	// Line 1:   MyContext
	// Line 2: }
	// Line 3: use_case "Test" {
	// Line 4:   when MyContext listens "SomeEvent"
	// Line 5: }
	// "MyContext" on line 4 starts at col 7 (after "  when "), len 9.
	craftSrc := "domain Biz {\n  MyContext\n}\nuse_case \"Test\" {\n  when MyContext listens \"SomeEvent\"\n}"
	id++
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":"file:///dl.craft","languageId":"craft","version":1,"text":%s}}`, mustMarshalString(craftSrc)))}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &tokID, Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///dl.craft"}}`)}); err != nil {
		t.Fatal(err)
	}

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out")
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}

	type absToken struct{ line, startChar, length, tokenType, modifiers uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt, mod := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3], result.Data[i+4]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt, mod}
	}

	// "MyContext" on line 4 must start at col 7, not col 0.
	// If it lands at col 0 the bug is present (ActorCol() returned 0 for domain_listen).
	if _, atZero := byPos[[2]uint32{4, 0}]; atZero {
		t.Error("domain_listen subject emitted at col 0 — ActorCol() bug not fixed")
	}
	if tok, ok := byPos[[2]uint32{4, 7}]; !ok {
		t.Errorf("MyContext not found at line 4 col 7 (found tokens: %v)", byPos)
	} else if tok.length != 9 {
		t.Errorf("MyContext: got length %d, want 9", tok.length)
	}

	_ = errCh
}

// TestSemanticTokens_TriggerPhrase verifies that uppercase idents and connector
// words inside a trigger phrase (e.g. "VAS" and "to" in
// "when Business_User purchases VAS to wallet") get craft-phrase-word tokens,
// overriding the TextMate entity-name / connector-keywords patterns that would
// otherwise color them as entity types or operators.
func TestSemanticTokens_TriggerPhrase(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: actor user Business_User
	// Line 1: use_case "Test" {
	// Line 2:   when Business_User purchases VAS to wallet
	//            0123456789012345678901234567890123456789
	//            w=2, B=7, VAS=29, to=33
	// Line 3: }
	craftSrc := "actor user Business_User\nuse_case \"Test\" {\n  when Business_User purchases VAS to wallet\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///trigger.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///trigger.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if len(result.Data)%5 != 0 {
		t.Fatalf("data length %d not a multiple of 5", len(result.Data))
	}

	type absToken struct{ line, startChar, length, tokenType, modifiers uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt, mod := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3], result.Data[i+4]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt, mod}
	}

	// Line 2: "  when Business_User purchases VAS to wallet"
	//          0         1         2         3         4
	//          0123456789012345678901234567890123456789012345
	//          "  " = 2 spaces, "when" at 2, "Business_User" at 7, "purchases" at 21
	//          "VAS" at 31, "to" at 35, "wallet" at 38
	//
	// Recount: "  when Business_User purchases VAS to wallet"
	// 0: ' '
	// 1: ' '
	// 2: 'w' (when starts)
	// 6: ' '
	// 7: 'B' (Business_User starts, len 13)
	// 20: ' '
	// 21: 'p' (purchases starts, len 9)
	// 30: ' '
	// 31: 'V' (VAS starts, len 3)
	// 34: ' '
	// 35: 't' (to starts, len 2)
	// 37: ' '
	// 38: 'w' (wallet starts, len 6)
	vasLine, vasCol := uint32(2), uint32(31)
	toLine, toCol := uint32(2), uint32(35)

	vasTok, vasOk := byPos[[2]uint32{vasLine, vasCol}]
	toTok, toOk := byPos[[2]uint32{toLine, toCol}]

	if !vasOk {
		t.Errorf("VAS not found at line %d col %d — trigger phrase should emit craft-phrase-word", vasLine, vasCol)
	}
	if !toOk {
		t.Errorf(`"to" not found at line %d col %d — trigger phrase should emit craft-phrase-word`, toLine, toCol)
	}
	if vasOk && toOk && vasTok.tokenType != toTok.tokenType {
		t.Errorf("VAS (type=%d) and to (type=%d) should both be craft-phrase-word", vasTok.tokenType, toTok.tokenType)
	}
	if vasOk && vasTok.length != 3 {
		t.Errorf("VAS length = %d, want 3", vasTok.length)
	}
	if toOk && toTok.length != 2 {
		t.Errorf(`"to" length = %d, want 2`, toTok.length)
	}

	cancel()
	_ = errCh
}

// TestSemanticTokens_StringLength verifies that string semantic tokens cover the
// full "content" span including both opening and closing quote characters.
// Previously, Token.Length() returned only the content length (quotes excluded),
// so the last 1-2 characters of every string fell back to TextMate highlighting,
// producing a visible color split in themes where string.other ≠ string.quoted.double.
func TestSemanticTokens_StringLength(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}

	// Line 0: use_case "Purchase" {
	//           ^col 0             ^col 9 → "Purchase" = 10 chars (with quotes)
	// Line 1:   when User does thing
	// Line 2:     Service notifies "VAS added"
	//                              ^col 21 → "VAS added" = 11 chars (with quotes)
	// Line 3: }
	craftSrc := "use_case \"Purchase\" {\n  when User does thing\n    Service notifies \"VAS added\"\n}"

	id++
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(fmt.Sprintf(
			`{"textDocument":{"uri":"file:///strings.craft","languageId":"craft","version":1,"text":%s}}`,
			mustMarshalString(craftSrc),
		)),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)

	id++
	tokID := id
	if err := writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &tokID,
		Method: "textDocument/semanticTokens/full",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///strings.craft"}}`),
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
		if msg.ID != nil && *msg.ID == tokID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for semanticTokens/full response")
	}
	if resp.Error != nil {
		t.Fatalf("semanticTokens/full returned error: %s", resp.Error)
	}

	var result struct {
		Data []uint32 `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	if len(result.Data)%5 != 0 {
		t.Fatalf("data length %d not a multiple of 5", len(result.Data))
	}

	type absToken struct{ line, startChar, length, tokenType, modifiers uint32 }
	byPos := make(map[[2]uint32]absToken)
	var curLine, curChar uint32
	for i := 0; i+4 < len(result.Data); i += 5 {
		dl, dc, ln, tt, mod := result.Data[i], result.Data[i+1], result.Data[i+2], result.Data[i+3], result.Data[i+4]
		curLine += dl
		if dl > 0 {
			curChar = dc
		} else {
			curChar += dc
		}
		byPos[[2]uint32{curLine, curChar}] = absToken{curLine, curChar, ln, tt, mod}
	}

	// "Purchase" at line 0, col 9 — full string with quotes = 10 chars.
	if tok, ok := byPos[[2]uint32{0, 9}]; !ok {
		t.Error(`"Purchase" string token not found at line 0 col 9`)
	} else if tok.length != 10 {
		t.Errorf(`"Purchase" token length = %d, want 10 (quotes included)`, tok.length)
	}

	// "VAS added" at line 2, col 21 — full string with quotes = 11 chars.
	if tok, ok := byPos[[2]uint32{2, 21}]; !ok {
		t.Error(`"VAS added" string token not found at line 2 col 21`)
	} else if tok.length != 11 {
		t.Errorf(`"VAS added" token length = %d, want 11 (quotes included)`, tok.length)
	}

	cancel()
	_ = errCh
}

func TestDefinition_ServiceContextTokenOffset(t *testing.T) {
	// L1:  domain Auth {
	// L2:    Login
	// L3:  }
	// L4:  domain Profile {
	// L5:    UserData
	// L6:  }
	// L7:  services {
	// L8:    UserSvc {
	// L9:      contexts: Auth, Profile
	// L10:   }
	// L11: }
	const craftSrc = "domain Auth {\n  Login\n}\ndomain Profile {\n  UserData\n}\nservices {\n  UserSvc {\n    contexts: Auth, Profile\n  }\n}"

	tests := []struct {
		name       string
		cursorLine uint32
		cursorChar uint32
		wantLine   uint32
		wantChar   uint32
	}{
		// "Auth" on L9 (0-based: line 8), col 14 → Auth domain at line 0 col 7
		{name: "first-context", cursorLine: 8, cursorChar: 14, wantLine: 0, wantChar: 7},
		// "Profile" on L9 col 20 → Profile domain at line 3 col 7
		{name: "second-context", cursorLine: 8, cursorChar: 20, wantLine: 3, wantChar: 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverIn, testOut := io.Pipe()
			testIn, serverOut := io.Pipe()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer testOut.Close()
			go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
			br := bufio.NewReader(testIn)

			id := 1
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
				Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
			readMsg(br)                                                                                     //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

			const uri = "file:///ctx_tokoffset.craft"
			openParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
				},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
			time.Sleep(300 * time.Millisecond)

			id++
			defID := id
			defParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": tc.cursorLine, "character": tc.cursorChar},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

			var defResp lspMsg
			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) {
				msg, err := readMsg(br)
				if err != nil {
					t.Fatalf("reading: %v", err)
				}
				if msg.ID != nil && *msg.ID == defID {
					defResp = msg
					break
				}
			}
			if defResp.ID == nil {
				t.Fatal("timed out waiting for definition response")
			}

			var locs []struct {
				Range struct {
					Start struct {
						Line      uint32 `json:"line"`
						Character uint32 `json:"character"`
					} `json:"start"`
				} `json:"range"`
			}
			if err := json.Unmarshal(defResp.Result, &locs); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(locs) == 0 {
				t.Fatalf("empty locations; cursor at (%d,%d)", tc.cursorLine, tc.cursorChar)
			}
			if locs[0].Range.Start.Line != tc.wantLine {
				t.Errorf("line: got %d want %d", locs[0].Range.Start.Line, tc.wantLine)
			}
			if locs[0].Range.Start.Character != tc.wantChar {
				t.Errorf("char: got %d want %d", locs[0].Range.Start.Character, tc.wantChar)
			}

			id2 := 99
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
			readMsg(br)                                                             //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
		})
	}
}

// TestDefinition_ServiceContext_QuotedName is part of the task-8a ripple-audit
// coverage: ServiceDecl.ContextTokens() returns raw tokens that may be
// SyntaxKindString. Go-to-definition must use the token's raw span width
// (quotes included) for the cursor hit-test, since that's what's actually on
// screen — not the unquoted ctxName's length, which is too short. Before the
// fix, a cursor over the last character of a quoted `contexts: "Login"` entry
// (just before the closing quote) fell outside the (too-narrow) hit-test
// window and go-to-definition silently failed.
func TestDefinition_ServiceContext_QuotedName(t *testing.T) {
	// L0: domain Login {
	// L1:   Login
	// L2: }
	// L3: services {
	// L4:   UserSvc {
	// L5:     contexts: "Login"
	// L6:   }
	// L7: }
	const craftSrc = "domain Login {\n  Login\n}\nservices {\n  UserSvc {\n    contexts: \"Login\"\n  }\n}"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///ctx_quoted.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Cursor over the 'n' of "Login" on L5 (0-based line 5, char 19) — the
	// last character of the quoted name, just before the closing quote.
	id++
	defID := id
	defParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 5, "character": 19},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

	var defResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		if msg.ID != nil && *msg.ID == defID {
			defResp = msg
			break
		}
	}
	if defResp.ID == nil {
		t.Fatal("timed out waiting for definition response")
	}

	var locs []struct {
		Range struct {
			Start struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(defResp.Result, &locs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("expected a definition location for quoted contexts: \"Login\", got none")
	}
	// domain Login is declared at line 0, char 7 (0-based).
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("line: got %d want 0", locs[0].Range.Start.Line)
	}
	if locs[0].Range.Start.Character != 7 {
		t.Errorf("char: got %d want 7", locs[0].Range.Start.Character)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

func TestRename_ActorName(t *testing.T) {
	// L1: actor user Alice
	// L2: use_case "T" {
	// L3:   when Alice creates Session
	// L4:     Alice asks Auth to validate
	// L5: }
	const craftSrc = "actor user Alice\nuse_case \"T\" {\n  when Alice creates Session\n    Alice asks Auth to validate\n}"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///rename_actor.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Cursor on "Alice" in line 0 (declaration), char 11 → rename to "Carol"
	id++
	renameID := id
	renameParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 0, "character": 11},
		"newName":      "Carol",
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &renameID, Method: "textDocument/rename", Params: renameParams}) //nolint:errcheck

	var renameResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == renameID {
			renameResp = msg
			break
		}
	}
	if renameResp.ID == nil {
		t.Fatal("timed out waiting for rename response")
	}
	if renameResp.Error != nil {
		t.Fatalf("rename error: %s", renameResp.Error)
	}

	var rawEdit struct {
		Changes map[string]json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(renameResp.Result, &rawEdit); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(rawEdit.Changes) == 0 {
		t.Fatal("no changes in workspace edit")
	}

	fileEdits, ok := rawEdit.Changes[uri]
	if !ok {
		t.Fatalf("no edits for %s; got keys: %v", uri, rawEdit.Changes)
	}
	type textEdit struct {
		NewText string `json:"newText"`
		Range   struct {
			Start struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	var edits []textEdit
	if err := json.Unmarshal(fileEdits, &edits); err != nil {
		t.Fatalf("unmarshal edits: %v", err)
	}
	// Expect 3 edits: declaration (line 0), trigger (line 2), action subject (line 3)
	if len(edits) != 3 {
		t.Fatalf("want 3 edits, got %d: %+v", len(edits), edits)
	}
	for _, e := range edits {
		if e.NewText != "Carol" {
			t.Errorf("edit newText = %q, want %q", e.NewText, "Carol")
		}
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

// TestPrepareRename_QuotedServiceName is the Fix-2 regression lock:
// ServiceDecl.Name() now resolves a QUOTED service name (a9efed0) to its
// SyntaxKindString token, so PrepareRename on `service "Payment Service" {`
// is reachable. For a String token, tok.Text() is the raw source text
// INCLUDING both quotes (Bug 8a), so the returned Range must span the FULL
// quoted token — both quote characters included — not just the inner name.
//
// Line 0: service "Payment Service" {
// col 0-6:  service
// col 8:    opening quote
// col 9-23: Payment Service (15 chars)
// col 24:   closing quote
// → full quoted token: col 8, length 17 (col 8..25 exclusive).
func TestPrepareRename_QuotedServiceName(t *testing.T) {
	const craftSrc = "service \"Payment Service\" {\n  contexts: Payment\n}\n"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///prepare_rename_quoted.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Cursor inside "Payment Service", e.g. col 12 (on the "m" of Payment).
	id++
	prepID := id
	prepParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 0, "character": 12},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &prepID, Method: "textDocument/prepareRename", Params: prepParams}) //nolint:errcheck

	var resp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == prepID {
			resp = msg
			break
		}
	}
	if resp.ID == nil {
		t.Fatal("timed out waiting for prepareRename response")
	}
	if resp.Error != nil {
		t.Fatalf("prepareRename error: %s", resp.Error)
	}

	var rng struct {
		Start struct {
			Line      uint32 `json:"line"`
			Character uint32 `json:"character"`
		} `json:"start"`
		End struct {
			Line      uint32 `json:"line"`
			Character uint32 `json:"character"`
		} `json:"end"`
	}
	if err := json.Unmarshal(resp.Result, &rng); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %s)", err, resp.Result)
	}

	if rng.Start.Line != 0 || rng.Start.Character != 8 {
		t.Errorf("start: got (%d,%d), want (0,8) — must be the opening quote, not the inner name", rng.Start.Line, rng.Start.Character)
	}
	if rng.End.Line != 0 || rng.End.Character != 25 {
		t.Errorf("end: got (%d,%d), want (0,25) — must include the closing quote", rng.End.Line, rng.End.Character)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

// TestRename_QuotedServiceName is the Fix-2 regression lock: renaming a
// QUOTED service declaration name must replace the FULL quoted token
// (including both quotes) with a correctly re-quoted, escaped NewText —
// never leave the source with mismatched/missing quotes. Before the fix,
// Rename used the raw newName verbatim as NewText over a range that (once
// combined with client placeholder behavior) could desync from the quotes,
// producing a corrupt (unquoted) declaration. This also exercises escaping:
// the new name itself contains a literal `"`, which must come back as `\"`
// so the produced source stays lexically valid.
func TestRename_QuotedServiceName(t *testing.T) {
	const craftSrc = "service \"Payment Service\" {\n  contexts: Payment\n}\n"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///rename_quoted_service.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	id++
	renameID := id
	renameParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 0, "character": 12},
		"newName":      `Billing "Prime" Service`,
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &renameID, Method: "textDocument/rename", Params: renameParams}) //nolint:errcheck

	var renameResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == renameID {
			renameResp = msg
			break
		}
	}
	if renameResp.ID == nil {
		t.Fatal("timed out waiting for rename response")
	}
	if renameResp.Error != nil {
		t.Fatalf("rename error: %s", renameResp.Error)
	}

	var rawEdit struct {
		Changes map[string]json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(renameResp.Result, &rawEdit); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	fileEdits, ok := rawEdit.Changes[uri]
	if !ok {
		t.Fatalf("no edits for %s; got keys: %v", uri, rawEdit.Changes)
	}
	type textEdit struct {
		NewText string `json:"newText"`
		Range   struct {
			Start struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"start"`
			End struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"end"`
		} `json:"range"`
	}
	var edits []textEdit
	if err := json.Unmarshal(fileEdits, &edits); err != nil {
		t.Fatalf("unmarshal edits: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("want 1 edit (declaration only), got %d: %+v", len(edits), edits)
	}
	e := edits[0]

	wantNewText := `"Billing \"Prime\" Service"`
	if e.NewText != wantNewText {
		t.Errorf("newText = %q, want %q (must be re-quoted and escaped)", e.NewText, wantNewText)
	}
	if e.Range.Start.Line != 0 || e.Range.Start.Character != 8 {
		t.Errorf("range start = (%d,%d), want (0,8) — full quoted span, opening quote included", e.Range.Start.Line, e.Range.Start.Character)
	}
	if e.Range.End.Line != 0 || e.Range.End.Character != 25 {
		t.Errorf("range end = (%d,%d), want (0,25) — full quoted span, closing quote included", e.Range.End.Line, e.Range.End.Character)
	}

	// Simulate applying the edit and confirm the result re-parses cleanly
	// with the new (quoted, escaped) service name — i.e. the edit does not
	// corrupt the source.
	lines := strings.Split(craftSrc, "\n")
	line := lines[e.Range.Start.Line]
	patched := line[:e.Range.Start.Character] + e.NewText + line[e.Range.End.Character:]
	lines[e.Range.Start.Line] = patched
	patchedSrc := strings.Join(lines, "\n")
	if !strings.Contains(patchedSrc, `service "Billing \"Prime\" Service" {`) {
		t.Errorf("patched source does not contain the expected re-quoted declaration; got:\n%s", patchedSrc)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

func TestDefinition_ExposureToTarget(t *testing.T) {
	// L1: actor user Alice
	// L2: service PaySvc {
	// L3:   language: golang
	// L4: }
	// L5: exposure default {
	// L6:   to: Alice
	// L7:   through: PaySvc
	// L8: }
	const craftSrc = "actor user Alice\nservice PaySvc {\n  language: golang\n}\nexposure default {\n  to: Alice\n  through: PaySvc\n}"

	tests := []struct {
		name       string
		cursorLine uint32 // 0-based
		cursorChar uint32
		wantLine   uint32
		wantChar   uint32
	}{
		// "Alice" at line 5 (0-based), char 6 → actor Alice at line 0, char 11
		{name: "to-actor", cursorLine: 5, cursorChar: 6, wantLine: 0, wantChar: 11},
		// "PaySvc" at line 6, char 11 → service PaySvc at line 1, char 8
		{name: "through-service", cursorLine: 6, cursorChar: 11, wantLine: 1, wantChar: 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serverIn, testOut := io.Pipe()
			testIn, serverOut := io.Pipe()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			defer testOut.Close()
			go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
			br := bufio.NewReader(testIn)

			id := 1
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
				Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}) //nolint:errcheck
			readMsg(br)                                                                                     //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

			const uri = "file:///exposure_def.craft"
			openParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
				},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
			time.Sleep(300 * time.Millisecond)

			id++
			defID := id
			defParams, _ := json.Marshal(map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": tc.cursorLine, "character": tc.cursorChar},
			})
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

			var defResp lspMsg
			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) {
				msg, err := readMsg(br)
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				if msg.ID != nil && *msg.ID == defID {
					defResp = msg
					break
				}
			}
			if defResp.ID == nil {
				t.Fatal("timed out")
			}
			var locs []struct {
				Range struct {
					Start struct {
						Line      uint32 `json:"line"`
						Character uint32 `json:"character"`
					} `json:"start"`
				} `json:"range"`
			}
			if err := json.Unmarshal(defResp.Result, &locs); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(locs) == 0 {
				t.Fatalf("empty locations; cursor (%d,%d)", tc.cursorLine, tc.cursorChar)
			}
			if locs[0].Range.Start.Line != tc.wantLine {
				t.Errorf("line: got %d want %d", locs[0].Range.Start.Line, tc.wantLine)
			}
			if locs[0].Range.Start.Character != tc.wantChar {
				t.Errorf("char: got %d want %d", locs[0].Range.Start.Character, tc.wantChar)
			}

			id2 := 99
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
			readMsg(br)                                                             //nolint:errcheck
			writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
		})
	}
}

// TestDefinition_ExposureToTarget_QuotedName is part of the task-8a
// ripple-audit coverage: ExposureDecl.ToTokens() returns raw tokens that may
// be SyntaxKindString. Go-to-definition must (a) use the token's raw span
// width (quotes included) for the cursor hit-test, since that's what's
// actually on screen, but (b) look the target up in the workspace symbol
// table via its unquoted content, since actor/service names in that table
// are never quoted. Before the fix, a quoted `to: "Alice"` never resolved
// because the raw `"Alice"` (with quotes) was used as the map key.
func TestDefinition_ExposureToTarget_QuotedName(t *testing.T) {
	// L0: actor user Alice
	// L1: service PaySvc {
	// L2:   language: golang
	// L3: }
	// L4: exposure default {
	// L5:   to: "Alice"
	// L6:   through: PaySvc
	// L7: }
	const craftSrc = "actor user Alice\nservice PaySvc {\n  language: golang\n}\nexposure default {\n  to: \"Alice\"\n  through: PaySvc\n}"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///exposure_def_quoted.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Cursor inside the quoted "Alice" token, line 5 char 8 (0-based) — lands
	// on the 'l' of Alice, between the opening quote at char 6 and the name.
	id++
	defID := id
	defParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 5, "character": 8},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &defID, Method: "textDocument/definition", Params: defParams}) //nolint:errcheck

	var defResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == defID {
			defResp = msg
			break
		}
	}
	if defResp.ID == nil {
		t.Fatal("timed out")
	}
	var locs []struct {
		Range struct {
			Start struct {
				Line      uint32 `json:"line"`
				Character uint32 `json:"character"`
			} `json:"start"`
		} `json:"range"`
	}
	if err := json.Unmarshal(defResp.Result, &locs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(locs) == 0 {
		t.Fatal("expected a definition location for quoted to: \"Alice\", got none")
	}
	// Actor Alice is declared at line 0, char 11 (0-based) — same position
	// TestDefinition_ExposureToTarget expects for the unquoted form.
	if locs[0].Range.Start.Line != 0 {
		t.Errorf("line: got %d want 0", locs[0].Range.Start.Line)
	}
	if locs[0].Range.Start.Character != 11 {
		t.Errorf("char: got %d want 11", locs[0].Range.Start.Character)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

func TestIncrementalSync(t *testing.T) {
	// Start with "actor user Alice" and incrementally rename "Alice" to "Carol"
	// by replacing chars 11–16 (0-based) on line 0 with "Carol".
	const initial = "actor user Alice"

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize", //nolint:errcheck
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br)                                                                                     //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///incremental.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": initial,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	// Send incremental change: replace "Alice" (chars 11–16) with "Carol"
	changeParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri, "version": 2},
		"contentChanges": []map[string]interface{}{
			{
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 0, "character": 11},
					"end":   map[string]interface{}{"line": 0, "character": 16},
				},
				"text": "Carol",
			},
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didChange", Params: changeParams}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	// Request hover on the name to verify it was parsed as "Carol"
	id++
	hoverID := id
	hoverParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 0, "character": 12},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &hoverID, Method: "textDocument/hover", Params: hoverParams}) //nolint:errcheck

	var hoverResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == hoverID {
			hoverResp = msg
			break
		}
	}
	if hoverResp.ID == nil {
		t.Fatal("timed out waiting for hover response")
	}

	// The hover result must mention "Carol", not "Alice"
	if hoverResp.Result == nil {
		t.Fatal("hover returned null result — parse may have used stale content")
	}
	if !strings.Contains(string(hoverResp.Result), "Carol") {
		t.Errorf("hover result does not contain 'Carol': %s", hoverResp.Result)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"}) //nolint:errcheck
	readMsg(br)                                                             //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})               //nolint:errcheck
}

func TestCompletion_ServiceContextsFieldValue(t *testing.T) {
	// Cursor is after "contexts: " on line 4 (0-based).
	// "Auth" is a bounded context defined in domain Payments — should appear in completions.
	const craftSrc = "domain Payments {\n  Auth\n}\nservice PaySvc {\n  contexts: \n}"
	// Line 4, char 12 = after "contexts: " (10 chars + "  " indent = 12)

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br) //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)})

	const uri = "file:///completion_ctx.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams})
	time.Sleep(300 * time.Millisecond)

	id++
	compID := id
	compParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 4, "character": 12},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &compID, Method: "textDocument/completion", Params: compParams})

	var compResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == compID {
			compResp = msg
			break
		}
	}
	if compResp.ID == nil {
		t.Fatal("timed out waiting for completion response")
	}

	var list struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(compResp.Result, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var found bool
	for _, item := range list.Items {
		if item.Label == "Auth" {
			found = true
		}
	}
	if !found {
		labels := make([]string, len(list.Items))
		for i, it := range list.Items {
			labels[i] = it.Label
		}
		t.Errorf("completion did not include 'Auth'; got: %v", labels)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"})
	readMsg(br)
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})
}

func TestCompletion_ServiceContextsFieldValue_TreePath(t *testing.T) {
	// Cursor is AFTER an existing value ("Auth ") in contexts: — the SyntaxKindServiceField
	// node spans this position, so treeEnclosingBlock returns "service.contexts" via the
	// tree path (not the regex fallback).
	// "Profile" is a second bounded context that should appear in completions.
	const craftSrc = "domain Payments {\n  Auth\n  Profile\n}\nservice PaySvc {\n  contexts: Auth \n}"
	// Line 5 (0-based), char 16 = after "  contexts: Auth " (17 chars, cursor at 16 = after space)

	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer testOut.Close()
	go func() { lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)})
	readMsg(br) //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)})

	const uri = "file:///completion_ctx_tree.craft"
	openParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "craft", "version": 1, "text": craftSrc,
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams})
	time.Sleep(300 * time.Millisecond)

	id++
	compID := id
	compParams, _ := json.Marshal(map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 5, "character": 16},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &compID, Method: "textDocument/completion", Params: compParams})

	var compResp lspMsg
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if msg.ID != nil && *msg.ID == compID {
			compResp = msg
			break
		}
	}
	if compResp.ID == nil {
		t.Fatal("timed out waiting for completion response")
	}

	var list struct {
		Items []struct {
			Label string `json:"label"`
		} `json:"items"`
	}
	if err := json.Unmarshal(compResp.Result, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var found bool
	for _, item := range list.Items {
		if item.Label == "Profile" {
			found = true
		}
	}
	if !found {
		labels := make([]string, len(list.Items))
		for i, it := range list.Items {
			labels[i] = it.Label
		}
		t.Errorf("tree-path completion did not include 'Profile'; got: %v", labels)
	}

	id2 := 99
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id2, Method: "shutdown"})
	readMsg(br)
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})
}

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

	// services block: line 0 "services {", line 1 "  Auth {", line 2 "    contexts: Login", line 3 "  }", line 4 "}"
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
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
			End struct {
				Line int `json:"line"`
			} `json:"end"`
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
		t.Errorf("expected Range.End.Line == 3 (closing `}` of Auth service), got %d", auth.Range.End.Line)
	}
	cancel()
}

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

	// use_case on lines 0-3, exposure on lines 4-7 (0-indexed)
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

	if len(ranges) < 2 {
		t.Fatalf("expected at least 2 folding ranges, got %d: %v", len(ranges), ranges)
	}

	foundUC := false
	for _, r := range ranges {
		if r.StartLine == 0 && r.EndLine == 3 {
			foundUC = true
		}
	}
	if !foundUC {
		t.Errorf("expected use_case fold [0,3], got: %v", ranges)
	}

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
