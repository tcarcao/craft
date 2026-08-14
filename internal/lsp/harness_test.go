package lsp_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/tcarcao/craft/v2/internal/lsp"
)

// These tests are about the test harness itself, not about the server.
//
// Every LSP integration test in this package talks to lsp.Serve over a pair of
// pipes. Once Serve returns — its context expired, or it handled `exit` — it
// stops reading and stops writing. Any test still mid-conversation is then
// talking to nothing. What must NOT happen is that the test blocks forever:
// a blocked test goroutine can never reach its own deferred Close, so nothing
// ever breaks the block, and Go's 10-minute watchdog kills the WHOLE package
// instead of failing the one test. That turns a slow CI runner into a total
// loss of signal for every test in internal/lsp.
//
// Reproduced on the v2.17.0 tip with:
//
//	go test ./internal/lsp/ -run TestIncrementalSync -count=10 -timeout 45s
//
// which passes three or four iterations at ~0.4s each and then wedges forever
// at the final `exit` write.
//
// The bound below is deliberately generous: these tests assert "eventually
// returns", not "returns quickly".
const harnessBound = 5 * time.Second

// stoppedServer starts a server, completes the initialize handshake so the
// connection is genuinely live, then stops the server and waits for it to let
// go of the pipes. What it returns is a connection to a server that is gone.
func stoppedServer(t *testing.T) (io.WriteCloser, *bufio.Reader) {
	t.Helper()

	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		testOut.Close()
		testIn.Close()
	})

	served := make(chan struct{})
	go func() {
		defer close(served)
		defer serverOut.Close()
		lsp.Serve(ctx, serverIn, serverOut) //nolint:errcheck
	}()
	br := bufio.NewReader(testIn)

	id := 1
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatalf("initialize response: %v", err)
	}

	// Cancelling is how the 5-second per-test context ends a server on a slow
	// runner, mid-conversation, with the test none the wiser.
	cancel()
	select {
	case <-served:
	case <-time.After(harnessBound):
		t.Fatal("lsp.Serve did not return after its context was cancelled")
	}
	return testOut, br
}

// The wedge that killed CI: the test's next write goes to a pipe nobody will
// ever read again.
func TestHarness_WriteDoesNotBlockAfterServerStops(t *testing.T) {
	testOut, _ := stoppedServer(t)

	done := make(chan error, 1)
	go func() {
		done <- writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "exit"})
	}()

	select {
	case <-done:
		// Returning at all is the assertion. An error is a fine outcome; the
		// test that called it will fail on its own terms. Blocking is not.
	case <-time.After(harnessBound):
		t.Fatalf("writeMsg blocked for %s after the server stopped reading; "+
			"a test that does this can never reach its deferred Close, so the "+
			"whole package dies at the go test watchdog", harnessBound)
	}
}

// The second wedge, visible in the same CI goroutine dump: a test parked in
// readMsg waiting for a message that is never coming. The per-test read loops
// only check their deadline BETWEEN reads, so one blocking read outlives them.
func TestHarness_ReadDoesNotBlockAfterServerStops(t *testing.T) {
	_, br := stoppedServer(t)

	done := make(chan error, 1)
	go func() {
		_, err := readMsg(br)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error reading from a server that has stopped")
		}
	case <-time.After(harnessBound):
		t.Fatalf("readMsg blocked for %s after the server stopped writing; "+
			"the 4-second deadline loops around readMsg cannot expire while "+
			"the read itself is blocked", harnessBound)
	}
}

// Unlike the two tests above, this one is a forward-looking guard rather than a
// reproduction: it passes on the old io.Pipe transport too, and that result is
// worth writing down, because "the server parks mid-notification and therefore
// stops reading" is the obvious theory of the CI hang and it is WRONG.
// go.lsp.dev/jsonrpc2 dispatches each request in its own goroutine (conn.Go), so
// a notification write that nobody is draining blocks only that goroutine; the
// read loop keeps accepting requests. A live server never wedges the test.
// The only wedge is a server that is GONE, which is what the two tests above
// cover.
//
// What this holds is the new transport's side of that: testPipe buffers without
// bound, so a backlog of unread notifications must not stall the conversation.
// Give testPipe a bounded buffer and this test is what fails.
func TestHarness_ServerKeepsRespondingWhileItsNotificationsGoUnread(t *testing.T) {
	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		testOut.Close()
		testIn.Close()
	})
	go func() { defer serverOut.Close(); lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if _, err := readMsg(br); err != nil {
		t.Fatalf("initialize response: %v", err)
	}
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "initialized", Params: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	// Open enough documents to push well past any single buffer's worth of
	// diagnostics, and deliberately read none of them.
	const docs = 40
	for i := 0; i < docs; i++ {
		openParams, _ := json.Marshal(map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        fmt.Sprintf("file:///backlog%02d.craft", i),
				"languageId": "craft",
				"version":    1,
				// Unresolved on purpose, so every file yields diagnostics to publish.
				"text": fmt.Sprintf("actor user Ghost%02d\n\nuse_case \"U%02d\" {\n  when Nobody%02d does a thing\n    Nowhere%02d notifies \"Order Place\"\n}", i, i, i, i),
			},
		})
		if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: openParams}); err != nil {
			t.Fatalf("didOpen %d: %v", i, err)
		}
	}

	// The server is now holding a backlog of unread notifications. It must still
	// be reading requests.
	id++
	shutdownID := id
	sent := make(chan error, 1)
	go func() {
		sent <- writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &shutdownID, Method: "shutdown"})
	}()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatalf("shutdown write: %v", err)
		}
	case <-time.After(harnessBound):
		t.Fatalf("writing a request blocked for %s while %d documents' diagnostics sat unread; "+
			"the server is parked mid-notification and has stopped reading", harnessBound, docs)
	}

	got := make(chan lspMsg, 1)
	go func() {
		for {
			msg, err := readMsg(br)
			if err != nil {
				return
			}
			if msg.ID != nil && *msg.ID == shutdownID {
				got <- msg
				return
			}
		}
	}()
	select {
	case <-got:
	case <-time.After(harnessBound):
		t.Fatalf("no shutdown response within %s: the server stopped answering once its "+
			"notifications went unread", harnessBound)
	}
}

// A write must still be delivered under normal conditions. This is the guard
// against "fixing" the two tests above by making writes silently drop.
func TestHarness_WriteIsStillDeliveredToALiveServer(t *testing.T) {
	serverIn, testOut := newTestPipe()
	testIn, serverOut := newTestPipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		testOut.Close()
		testIn.Close()
	})
	go func() { defer serverOut.Close(); lsp.Serve(ctx, serverIn, serverOut) }() //nolint:errcheck
	br := bufio.NewReader(testIn)

	id := 1
	if err := writeMsg(testOut, lspMsg{JSONRPC: "2.0", ID: &id, Method: "initialize",
		Params: json.RawMessage(`{"processId":null,"rootUri":null,"capabilities":{}}`)}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	msg, err := readMsg(br)
	if err != nil {
		t.Fatalf("initialize response: %v", err)
	}
	if msg.ID == nil || *msg.ID != id {
		t.Fatalf("expected a response to id %d, got %+v", id, msg)
	}
	if msg.Result == nil {
		t.Fatal("initialize returned no result, so the request did not reach the server")
	}
}
