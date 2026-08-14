package lsp_test

// Integration tests for use case context extraction via CRAFT_EXTRACT_WORKSPACE.
// Covers the entryPointContext and involvedContexts fields added to craftUseCaseEntry.

import (
	"bufio"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tcarcao/craft/v2/internal/lsp"
)

// ── fixtures ──────────────────────────────────────────────────────────────────

// externalTriggerSrc has an external (actor) trigger followed by two actions:
// an internal action (Auth) and a sync action (Auth → Database).
// Expected: entryPoint = "Auth", involved = ["Auth", "Database"]
const externalTriggerSrc = `actor user Alice
domain Auth {
  Login
}
domain Database {
  Storage
}
use_case "Login Flow" {
  when Alice initiates login
    Auth validates credentials
    Auth asks Database to check session
}`

// domainListenSrc has a domain_listen trigger on Payments followed by two
// actions referencing Payments and Inventory.
// Expected: entryPoint = "Payments", involved = ["Payments", "Inventory"]
const domainListenSrc = `domain Payments {
  Processing
}
domain Inventory {
  Stock
}
use_case "Process Payment" {
  when Payments listens "OrderCreated"
    Payments charges card
    Inventory updates stock
}`

// noActionsSrc has only an external trigger with no following domain actions.
// Expected: entryPoint = "", involved = []
const noActionsSrc = `actor user Alice
use_case "Idle Case" {
  when Alice initiates nothing
}`

// deduplicationSrc has the same domain ("Auth") referenced multiple times
// across different actions, plus one TargetDomain ("Notify").
// Expected: "Auth" appears once in involved, "Notify" also appears once.
const deduplicationSrc = `actor user Alice
domain Auth {
  Login
}
domain Notify {
  Alerts
}
use_case "Repeat Auth" {
  when Alice initiates signup
    Auth creates account
    Auth validates email
    Auth asks Notify to send welcome
    Auth notifies "AccountCreated"
}`

// ── helpers ───────────────────────────────────────────────────────────────────

type useCaseResult struct {
	Name              string   `json:"name"`
	StartLine         int      `json:"startLine"`
	EndLine           int      `json:"endLine"`
	InCurrentFile     bool     `json:"inCurrentFile"`
	EntryPointContext string   `json:"entryPointContext"`
	InvolvedContexts  []string `json:"involvedContexts"`
}

// extractUseCases boots a server, opens one file, calls CRAFT_EXTRACT_WORKSPACE,
// and returns the use cases from the result.
func extractUseCases(t *testing.T, craftSrc string) []useCaseResult {
	t.Helper()

	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()

	ctx, cancel := context.WithTimeout(context.Background(), serverLifetime)
	t.Cleanup(func() {
		cancel()
		testOut.Close()
	})

	go func() { defer serverOut.Close(); lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck

	br := bufio.NewReader(testIn)
	id := 1

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`),
	}))
	if _, err := readMsg(br); err != nil {
		t.Fatal(err)
	}
	must(writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}))

	id++
	must(writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", Method: "textDocument/didOpen",
		Params: json.RawMessage(`{"textDocument":{"uri":"file:///extract_test.craft","languageId":"craft","version":1,"text":` +
			mustMarshalString(craftSrc) + `}}`),
	}))
	time.Sleep(300 * time.Millisecond)

	id++
	cmdID := id
	must(writeMsg(testOut, lspMsg{
		JSONRPC: "2.0", ID: &cmdID,
		Method: "workspace/executeCommand",
		Params: json.RawMessage(`{"command":"CRAFT_EXTRACT_WORKSPACE","arguments":[]}`),
	}))

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readMsg(br)
		if err != nil {
			t.Fatalf("reading message: %v", err)
		}
		if msg.ID == nil || *msg.ID != cmdID {
			continue
		}
		if msg.Error != nil {
			t.Fatalf("CRAFT_EXTRACT_WORKSPACE error: %s", msg.Error)
		}
		var wrapper struct {
			UseCases []useCaseResult `json:"useCases"`
		}
		if err := json.Unmarshal(msg.Result, &wrapper); err != nil {
			t.Fatalf("unmarshaling result: %v", err)
		}
		return wrapper.UseCases
	}
	t.Fatal("timed out waiting for CRAFT_EXTRACT_WORKSPACE response")
	return nil
}

// containsAll checks that every element of want appears in got.
func containsAll(got []string, want ...string) bool {
	set := make(map[string]struct{}, len(got))
	for _, s := range got {
		set[s] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

// countOccurrences counts how many times name appears in list.
func countOccurrences(list []string, name string) int {
	n := 0
	for _, s := range list {
		if s == name {
			n++
		}
	}
	return n
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestUseCaseExtract_EntryPoint_ExternalTrigger verifies that for an external
// (actor) trigger the entryPointContext is the first action's domain, not the
// actor name.
func TestUseCaseExtract_EntryPoint_ExternalTrigger(t *testing.T) {
	ucs := extractUseCases(t, externalTriggerSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d: %+v", len(ucs), ucs)
	}
	uc := ucs[0]
	if uc.Name != "Login Flow" {
		t.Errorf("name: got %q, want %q", uc.Name, "Login Flow")
	}
	if uc.EntryPointContext != "Auth" {
		t.Errorf("entryPointContext: got %q, want %q", uc.EntryPointContext, "Auth")
	}
}

// TestUseCaseExtract_EntryPoint_DomainListenTrigger verifies that for a
// domain_listen trigger the listening domain becomes the entryPointContext,
// even though it would also appear in the first action.
func TestUseCaseExtract_EntryPoint_DomainListenTrigger(t *testing.T) {
	ucs := extractUseCases(t, domainListenSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d: %+v", len(ucs), ucs)
	}
	uc := ucs[0]
	if uc.EntryPointContext != "Payments" {
		t.Errorf("entryPointContext: got %q, want %q", uc.EntryPointContext, "Payments")
	}
}

// TestUseCaseExtract_InvolvedContexts_ExternalTrigger verifies that both the
// action domain and the sync-action target domain appear in involvedContexts.
func TestUseCaseExtract_InvolvedContexts_ExternalTrigger(t *testing.T) {
	ucs := extractUseCases(t, externalTriggerSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(ucs))
	}
	uc := ucs[0]
	if !containsAll(uc.InvolvedContexts, "Auth", "Database") {
		t.Errorf("involvedContexts %v must contain Auth and Database", uc.InvolvedContexts)
	}
}

// TestUseCaseExtract_InvolvedContexts_DomainListenTrigger verifies that the
// trigger domain and action domains all appear in involvedContexts.
func TestUseCaseExtract_InvolvedContexts_DomainListenTrigger(t *testing.T) {
	ucs := extractUseCases(t, domainListenSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(ucs))
	}
	uc := ucs[0]
	if !containsAll(uc.InvolvedContexts, "Payments", "Inventory") {
		t.Errorf("involvedContexts %v must contain Payments and Inventory", uc.InvolvedContexts)
	}
}

// TestUseCaseExtract_EmptyContexts_NoActions verifies that a use case whose
// body has only an external trigger and no actions produces an empty
// entryPointContext and no involvedContexts.
func TestUseCaseExtract_EmptyContexts_NoActions(t *testing.T) {
	ucs := extractUseCases(t, noActionsSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d: %+v", len(ucs), ucs)
	}
	uc := ucs[0]
	if uc.EntryPointContext != "" {
		t.Errorf("entryPointContext: got %q, want empty string", uc.EntryPointContext)
	}
	if len(uc.InvolvedContexts) != 0 {
		t.Errorf("involvedContexts: got %v, want empty", uc.InvolvedContexts)
	}
}

// TestUseCaseExtract_InvolvedContexts_Deduplicated verifies that a domain
// referenced multiple times across different actions only appears once in
// involvedContexts.
func TestUseCaseExtract_InvolvedContexts_Deduplicated(t *testing.T) {
	ucs := extractUseCases(t, deduplicationSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d: %+v", len(ucs), ucs)
	}
	uc := ucs[0]
	if n := countOccurrences(uc.InvolvedContexts, "Auth"); n != 1 {
		t.Errorf("Auth appears %d times in involvedContexts %v, want exactly 1", n, uc.InvolvedContexts)
	}
	if n := countOccurrences(uc.InvolvedContexts, "Notify"); n != 1 {
		t.Errorf("Notify appears %d times in involvedContexts %v, want exactly 1", n, uc.InvolvedContexts)
	}
}

// TestUseCaseExtract_BlockRange verifies that startLine and endLine are
// populated so the extension can navigate to the use case source.
func TestUseCaseExtract_BlockRange(t *testing.T) {
	ucs := extractUseCases(t, externalTriggerSrc)

	if len(ucs) != 1 {
		t.Fatalf("expected 1 use case, got %d", len(ucs))
	}
	uc := ucs[0]
	if uc.StartLine == 0 {
		t.Error("startLine must be non-zero")
	}
	if uc.EndLine < uc.StartLine {
		t.Errorf("endLine %d must be >= startLine %d", uc.EndLine, uc.StartLine)
	}
}
