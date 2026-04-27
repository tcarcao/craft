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
	if result.Capabilities.CompletionProvider != nil {
		t.Error("completionProvider must not be declared (Q20)")
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

	// 6 tokens × 5 fields each = 30 uint32 values.
	// Expected relative encoding (deltaLine, deltaChar, length, tokenType, modifiers):
	//   Alice     → [1,  9, 5, 0, 1]  type=class(actor),     mod=declaration
	//   Bob       → [1, 11, 3, 0, 1]  type=class(actor),     mod=declaration
	//   Commerce  → [2,  7, 8, 1, 1]  type=namespace(domain), mod=declaration
	//   Orders    → [1,  4, 6, 1, 0]  type=namespace(bc),     mod=0
	//   Payments  → [1,  4, 8, 1, 0]  type=namespace(bc),     mod=0
	//   MyService → [2,  8, 9, 2, 1]  type=interface(service),mod=declaration
	want := []uint32{
		1, 9, 5, 0, 1,
		1, 11, 3, 0, 1,
		2, 7, 8, 1, 1,
		1, 4, 6, 1, 0,
		1, 4, 8, 1, 0,
		2, 8, 9, 2, 1,
	}

	if len(result.Data) != len(want) {
		t.Fatalf("expected %d uint32 values (6 tokens × 5), got %d: %v", len(want), len(result.Data), result.Data)
	}

	type tokenFields struct {
		deltaLine, deltaChar, length, tokenType, modifiers uint32
	}
	names := []string{"Alice", "Bob", "Commerce", "Orders(bc)", "Payments(bc)", "MyService"}
	for i, name := range names {
		base := i * 5
		got := tokenFields{result.Data[base], result.Data[base+1], result.Data[base+2], result.Data[base+3], result.Data[base+4]}
		exp := tokenFields{want[base], want[base+1], want[base+2], want[base+3], want[base+4]}
		if got != exp {
			t.Errorf("token %d (%s): got [%d,%d,%d,%d,%d] want [%d,%d,%d,%d,%d]",
				i, name,
				got.deltaLine, got.deltaChar, got.length, got.tokenType, got.modifiers,
				exp.deltaLine, exp.deltaChar, exp.length, exp.tokenType, exp.modifiers,
			)
		}
	}

	cancel()
}
