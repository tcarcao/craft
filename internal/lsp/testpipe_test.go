package lsp_test

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// testPipe is the transport every LSP integration test in this package uses to
// talk to lsp.Serve. It exists because io.Pipe cannot be used safely for a
// full-duplex protocol when only one side is ever draining.
//
// io.Pipe is unbuffered: a Write blocks until a reader consumes it, forever if
// no reader is coming. Once lsp.Serve returns — its context expired, or it
// handled `exit` — nothing reads the test's pipe and nothing writes to it ever
// again, so a test still mid-conversation parks on its next write or read and
// never comes back.
//
// Forever is the fatal part, not the blocking. A parked test goroutine cannot
// reach its own deferred Close, so nothing breaks the block, and go test's
// watchdog panics the WHOLE package after 10 minutes rather than failing the one
// test. One slow CI runner costs the signal from every test in internal/lsp.
//
// Worth recording what this is NOT, because it is the tempting explanation and
// it is wrong: a live server blocked writing a notification nobody drains does
// not wedge anything. go.lsp.dev/jsonrpc2 dispatches each request in its own
// goroutine, so the read loop keeps accepting requests regardless. Only a
// departed server wedges a test. TestHarness_ServerKeepsRespondingWhileIts-
// NotificationsGoUnread pins that down; it passes on io.Pipe too.
//
// testPipe removes the wedge by construction: writes are buffered and never
// block, and a closed pipe reports EOF instead of parking its reader. The Serve
// goroutines close their output end on return, which is what turns "the server
// is gone" into a prompt EOF rather than a stall. See harness_test.go.
type testPipe struct {
	mu     sync.Mutex
	buf    []byte
	closed bool
	notify chan struct{}
}

// serverLifetime is how long the lsp.Serve goroutine a test starts is allowed to
// live. It is a leash on the goroutine, not a deadline anything asserts on: every
// test ends its own server by cancelling in a defer or a t.Cleanup, and the one
// test that checks the server exits promptly after `exit` times that itself.
//
// It used to be 5 seconds, against tests that allow themselves up to ~4.5s of
// their own waits (two 200-300ms sleeps plus a 4-second read deadline; 39 tests
// use that exact shape). Under 500ms of headroom meant a loaded CI runner could
// expire the server mid-conversation, and the test would then be talking to a
// server that was already gone. Generous headroom costs nothing, because no
// passing test ever waits on this.
const serverLifetime = 60 * time.Second

// testPipeReadBackstop bounds a read that is waiting for a message no one is
// going to send. It is not a timeout any correct test should ever reach: the
// server closing its output end is what normally ends a read. It is here so that
// a shape neither harness_test.go nor this comment anticipated still fails one
// test loudly instead of taking the package down with it.
const testPipeReadBackstop = 30 * time.Second

// newTestPipe returns the two ends of one unidirectional pipe, matching the
// shape of io.Pipe so the call sites read the same. Both ends are the same
// object; the names at the call site say which direction it carries.
func newTestPipe() (*testPipe, *testPipe) {
	p := &testPipe{notify: make(chan struct{}, 1)}
	return p, p
}

// Write never blocks. That is the whole point: a full buffer cannot stall the
// server mid-notification, and a departed reader cannot strand the writer.
func (p *testPipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	p.buf = append(p.buf, b...)
	p.mu.Unlock()
	p.signal()
	return len(b), nil
}

func (p *testPipe) Read(b []byte) (int, error) {
	backstop := time.After(testPipeReadBackstop)
	for {
		p.mu.Lock()
		if len(p.buf) > 0 {
			n := copy(b, p.buf)
			p.buf = p.buf[n:]
			p.mu.Unlock()
			return n, nil
		}
		closed := p.closed
		p.mu.Unlock()

		// Drain before reporting the end of the stream, so a Close that races a
		// final write still hands over what was written.
		if closed {
			return 0, io.EOF
		}

		select {
		case <-p.notify:
		case <-backstop:
			return 0, fmt.Errorf("test pipe: nothing arrived in %s and the peer never closed", testPipeReadBackstop)
		}
	}
}

// Close is idempotent. Several tests close the same end from both a defer and a
// t.Cleanup, and the Serve goroutines close their output end on return.
func (p *testPipe) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	p.signal()
	return nil
}

// signal wakes a parked reader. Dropping the signal when one is already pending
// is safe: the reader re-checks the buffer under the lock after every wake, so a
// coalesced signal cannot lose data.
func (p *testPipe) signal() {
	select {
	case p.notify <- struct{}{}:
	default:
	}
}
