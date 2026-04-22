// capabilities_test.go is the canonical home for assertions that each
// ServerCapabilities field declared in Initialize is actually served by a
// handler. Per AGENT.md §3 (Q20): every PR that adds a handler lands a test
// here; every PR that retires a handler removes its test here.
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

// TestServerCapabilities_S5 asserts exactly the capabilities declared after S5.
// Slices S6+ extend this list alongside their handlers (Q20).
func TestServerCapabilities_S5(t *testing.T) {
	serverIn, testOut := io.Pipe()
	testIn, serverOut := io.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go lsp.Serve(ctx, serverIn, serverOut) //nolint:errcheck

	br := bufio.NewReader(testIn)
	id := 1

	if err := writeCapMsg(testOut, capMsg{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := readCapMsg(br)
	if err != nil {
		t.Fatalf("reading initialize response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize returned error: %s", resp.Error)
	}

	var result struct {
		Capabilities struct {
			TextDocumentSync       interface{} `json:"textDocumentSync"`
			DocumentSymbolProvider interface{} `json:"documentSymbolProvider"`
			HoverProvider          interface{} `json:"hoverProvider"`
			SemanticTokensProvider interface{} `json:"semanticTokensProvider"`
			DefinitionProvider     interface{} `json:"definitionProvider"`
			ExecuteCommandProvider interface{} `json:"executeCommandProvider"`
			// Capabilities that MUST NOT be declared yet (not yet implemented).
			CompletionProvider   interface{} `json:"completionProvider"`
			FoldingRangeProvider interface{} `json:"foldingRangeProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshaling result: %v", err)
	}

	// S2 capability — must stay.
	if result.Capabilities.TextDocumentSync == nil {
		t.Error("textDocumentSync must be declared (S2)")
	}
	// S3 capabilities — added alongside their handlers.
	if result.Capabilities.DocumentSymbolProvider == nil {
		t.Error("documentSymbolProvider must be declared (S3, Q20)")
	}
	if result.Capabilities.HoverProvider == nil {
		t.Error("hoverProvider must be declared (S3, Q20)")
	}
	// S4 capability — added alongside SemanticTokensFull handler.
	if result.Capabilities.SemanticTokensProvider == nil {
		t.Error("semanticTokensProvider must be declared (S4, Q20)")
	}
	// S5 capabilities — added alongside definition and executeCommand handlers.
	if result.Capabilities.DefinitionProvider == nil {
		t.Error("definitionProvider must be declared (S5, Q20)")
	}
	if result.Capabilities.ExecuteCommandProvider == nil {
		t.Error("executeCommandProvider must be declared (S5, Q20)")
	}
	// Future capabilities — must not be declared yet.
	if result.Capabilities.CompletionProvider != nil {
		t.Error("completionProvider must not be declared yet (Q20)")
	}
	if result.Capabilities.FoldingRangeProvider != nil {
		t.Error("foldingRangeProvider must not be declared yet (Q20); add in S7")
	}

	testOut.Close()
}

// capMsg is a minimal JSON-RPC message for capability testing.
type capMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func writeCapMsg(w io.Writer, msg capMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func readCapMsg(r *bufio.Reader) (capMsg, error) {
	var length int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return capMsg{}, err
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
		return capMsg{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return capMsg{}, err
	}
	var msg capMsg
	return msg, json.Unmarshal(body, &msg)
}
