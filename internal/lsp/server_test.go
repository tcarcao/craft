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
