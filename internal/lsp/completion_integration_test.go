package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/tcarcao/craft/internal/lsp"
)

// completionItem is the subset of fields used in completion assertions.
type completionItem struct {
	Label            string  `json:"label"`
	Kind             float64 `json:"kind"`
	Detail           string  `json:"detail"`
	InsertText       string  `json:"insertText"`
	InsertTextFormat float64 `json:"insertTextFormat"`
}

// sendCompletion sends textDocument/completion and returns the items list.
func sendCompletion(t *testing.T, testOut io.Writer, br *bufio.Reader, id *int, uri string, line, char uint32) []completionItem {
	t.Helper()
	*id++
	reqID := *id
	params, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
	})
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &reqID, Method: "textDocument/completion", Params: params}); err != nil {
		t.Fatalf("sendCompletion writeMsg: %v", err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("sendCompletion readMsg: %v", err)
		}
		if msg.ID != nil && *msg.ID == reqID {
			var result struct {
				Items []completionItem `json:"items"`
			}
			if msg.Result != nil {
				json.Unmarshal(msg.Result, &result) //nolint:errcheck
			}
			return result.Items
		}
	}
	t.Fatal("sendCompletion: timed out")
	return nil
}

// hasLabel returns true if items contains an entry with the given label.
func hasLabel(items []completionItem, label string) bool {
	for _, it := range items {
		if it.Label == label {
			return true
		}
	}
	return false
}

// labelList extracts all labels for error messages.
func labelList(items []completionItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func TestCompletion_ServiceFields(t *testing.T) {
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
	readMsg(br) //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const src = "service Foo {\n  \n}"
	const uri = "file:///svc_fields.craft"
	openParams, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	items := sendCompletion(t, testOut, br, &id, uri, 1, 2)
	for _, field := range []string{"contexts:", "data-stores:", "language:", "deployment:"} {
		if !hasLabel(items, field) {
			t.Errorf("expected field %q in service completions, got: %v", field, labelList(items))
		}
	}
}

func TestCompletion_TopLevelKeywords(t *testing.T) {
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
	readMsg(br) //nolint:errcheck
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}) //nolint:errcheck

	const uri = "file:///top.craft"
	openParams, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri": uri, "languageId": "craft", "version": 1, "text": "\n",
		},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	items := sendCompletion(t, testOut, br, &id, uri, 0, 0)

	for _, kw := range []string{"service", "services", "domain", "domains", "actor", "actors", "use_case", "expose"} {
		if !hasLabel(items, kw) {
			t.Errorf("expected keyword %q in top-level completions, got: %v", kw, labelList(items))
		}
	}
	for _, it := range items {
		if it.Label == "service" && it.InsertTextFormat != 2 {
			t.Errorf("service completion: expected InsertTextFormat=2 (snippet), got %v", it.InsertTextFormat)
		}
	}
}
