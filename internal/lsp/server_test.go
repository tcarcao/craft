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

// collectMsgs reads messages from br for up to dur, returning all collected.
// It uses a background goroutine so the deadline is respected even when readMsg blocks.
func collectMsgs(br *bufio.Reader, dur time.Duration) []lspMsg {
	type result struct {
		msg lspMsg
		err error
	}
	ch := make(chan result, 64)
	go func() {
		for {
			msg, err := readMsg(br)
			ch <- result{msg, err}
			if err != nil {
				return
			}
		}
	}()
	timer := time.NewTimer(dur)
	defer timer.Stop()
	var msgs []lspMsg
	for {
		select {
		case r := <-ch:
			if r.err != nil {
				return msgs
			}
			msgs = append(msgs, r.msg)
		case <-timer.C:
			return msgs
		}
	}
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
	for _, msg := range collectMsgs(br, 500*time.Millisecond) {
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
	for _, msg := range collectMsgs(br, 300*time.Millisecond) {
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
	msgs3 := collectMsgs(br, 300*time.Millisecond)
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
