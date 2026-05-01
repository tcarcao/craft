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

func TestCompletion_LanguageValues(t *testing.T) {
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

	const src = "service Foo {\n  language: \n}"
	const uri = "file:///lang_values.craft"
	openParams, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	// cursor at col 12 — after "  language: "
	items := sendCompletion(t, testOut, br, &id, uri, 1, 12)
	for _, lang := range []string{"golang", "python", "java", "typescript", "kotlin", "rust"} {
		if !hasLabel(items, lang) {
			t.Errorf("expected language %q in language completions, got: %v", lang, labelList(items))
		}
	}
}

func TestCompletion_ContextsField_BCSymbols(t *testing.T) {
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

	// Open two files: domain defines Commerce with Orders+Payments; service uses contexts:
	const domainSrc = "domain Commerce {\n  Orders\n  Payments\n}"
	const serviceSrc = "service OrderSvc {\n  contexts: \n}"
	for uri, src := range map[string]string{
		"file:///domain.craft":  domainSrc,
		"file:///service.craft": serviceSrc,
	} {
		p, _ := json.Marshal(map[string]any{
			"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
		})
		writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	}
	time.Sleep(300 * time.Millisecond)

	// cursor at line 1, col 12 — after "  contexts: " in service file
	items := sendCompletion(t, testOut, br, &id, "file:///service.craft", 1, 12)
	for _, bc := range []string{"Orders", "Payments"} {
		if !hasLabel(items, bc) {
			t.Errorf("expected BC %q in contexts: completions, got: %v", bc, labelList(items))
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

func TestCompletion_UseCase_WhenKeyword(t *testing.T) {
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

	// use_case body with blank line — cursor inside
	const src = "use_case \"Foo\" {\n  \n}"
	const uri = "file:///uc_when.craft"
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	items := sendCompletion(t, testOut, br, &id, uri, 1, 2)
	if !hasLabel(items, "when") {
		t.Errorf("expected 'when' in use_case body completions, got: %v", labelList(items))
	}
}

func TestCompletion_UseCase_ActionSymbols(t *testing.T) {
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

	// Workspace: actor, domain (+ BC), service — all in one file
	const src = "actor user Alice\ndomain Commerce {\n  Orders\n}\nservices {\n  OrderSvc {\n    contexts: Orders\n  }\n}\nuse_case \"Checkout\" {\n  when Alice initiates x\n    Orders \n}"
	const uri = "file:///uc_action.craft"
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Line 11: "    Orders " — cursor at col 10 (past first word, so isActionLine=true)
	items := sendCompletion(t, testOut, br, &id, uri, 11, 10)
	for _, sym := range []string{"Alice", "Commerce", "Orders", "OrderSvc"} {
		if !hasLabel(items, sym) {
			t.Errorf("expected symbol %q in use_case action completions, got: %v", sym, labelList(items))
		}
	}
}

func TestCompletion_Expose(t *testing.T) {
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

	// Main file: actor Alice, service OrderSvc, domain Commerce with BC Orders
	// expose block with to:, through:, contexts: lines
	const src = "actor user Alice\nservices {\n  OrderSvc {\n    contexts: Orders\n  }\n}\ndomain Commerce {\n  Orders\n}\nexposure MyAPI {\n  to: \n  through: \n  contexts: \n}"
	const uri = "file:///expose.craft"
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)

	// Test expose field names: open a simple expose file with blank body
	const expFieldSrc = "exposure MyAPI {\n  \n}"
	const expFieldURI = "file:///expose_fields.craft"
	pf, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": expFieldURI, "languageId": "craft", "version": 1, "text": expFieldSrc},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: pf}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	// Sub-test 1: expose field names
	fieldItems := sendCompletion(t, testOut, br, &id, expFieldURI, 1, 2)
	for _, field := range []string{"to:", "through:", "contexts:"} {
		if !hasLabel(fieldItems, field) {
			t.Errorf("expected expose field %q, got: %v", field, labelList(fieldItems))
		}
	}

	// Sub-test 2: to: → actor names (line 10 of src, col 6 = after "  to: ")
	toItems := sendCompletion(t, testOut, br, &id, uri, 10, 6)
	if !hasLabel(toItems, "Alice") {
		t.Errorf("expected actor 'Alice' in to: completions, got: %v", labelList(toItems))
	}

	// Sub-test 3: through: → service names (line 11, col 11 = after "  through: ")
	throughItems := sendCompletion(t, testOut, br, &id, uri, 11, 11)
	if !hasLabel(throughItems, "OrderSvc") {
		t.Errorf("expected service 'OrderSvc' in through: completions, got: %v", labelList(throughItems))
	}

	// Sub-test 4: contexts: → BC names (line 12, col 12 = after "  contexts: ")
	ctxItems := sendCompletion(t, testOut, br, &id, uri, 12, 12)
	if !hasLabel(ctxItems, "Orders") {
		t.Errorf("expected BC 'Orders' in expose contexts: completions, got: %v", labelList(ctxItems))
	}
}

func TestCompletion_ActorsBlock(t *testing.T) {
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

	const src = "actors {\n  \n}"
	const uri = "file:///actors_block.craft"
	p, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "craft", "version": 1, "text": src},
	})
	writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: p}) //nolint:errcheck
	time.Sleep(200 * time.Millisecond)

	items := sendCompletion(t, testOut, br, &id, uri, 1, 2)
	for _, actorType := range []string{"user", "system", "service"} {
		if !hasLabel(items, actorType) {
			t.Errorf("expected actor type %q in actors block completions, got: %v", actorType, labelList(items))
		}
	}
}
