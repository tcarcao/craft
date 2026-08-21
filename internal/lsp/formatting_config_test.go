package lsp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
	"github.com/tcarcao/craft/v2/internal/workspace"
)

// formatForURI and formatWithOptions drive Formatting directly against a
// freshly constructed Server, the same direct-call pattern openTwin uses in
// utf16_twin_internal_test.go. These tests live in this internal (package
// lsp) file rather than in the external server_test.go / lsp_test suite:
// that suite drives the server over a real jsonrpc2 pipe via lsp.Serve, and
// this repo has a documented history of that harness deadlocking. A direct
// method call exercises exactly the same handler code with none of that
// risk.
func formatForURI(t *testing.T, u uri.URI, content string) string {
	t.Helper()
	return formatWithOptions(t, u, content, protocol.FormattingOptions{})
}

func formatWithOptions(t *testing.T, u uri.URI, content string, opts protocol.FormattingOptions) string {
	t.Helper()
	s := &Server{ws: workspace.New(nil), logger: slog.Default()}
	docURI := string(u)
	s.ws.Open(docURI, content)
	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
		Options:      opts,
	})
	if err != nil {
		t.Fatalf("Formatting: %v", err)
	}
	if len(edits) == 0 {
		return content
	}
	return edits[0].NewText
}

func TestFormattingUsesCraftfmt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := formatForURI(t, uri.File(path), content)
	if !strings.Contains(got, "\n  Foo {") {
		t.Errorf("server ignored .craftfmt, got\n%q", got)
	}
}

func TestFormattingIgnoresClientTabSize(t *testing.T) {
	// editor.detectIndentation infers tabSize from file content, so honouring
	// it would make the editor and the CLI disagree permanently. The server
	// must ignore it. See docs/decisions/formatting-configuration.md D6.
	dir := t.TempDir()
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := formatWithOptions(t, uri.File(path), content, protocol.FormattingOptions{TabSize: 2})
	if !strings.Contains(got, "\n    Foo {") {
		t.Errorf("server honoured client tabSize, got\n%q", got)
	}
}

// TestFormattingMultiRoot pins that resolution is per-document-path, not
// per-server: two files in two different temp directories, each with its
// own .craftfmt, formatted through the same *Server, must each get their
// own indent.
func TestFormattingMultiRoot(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirA, ".craftfmt"), []byte("indent = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, ".craftfmt"), []byte("indent = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	pathA := filepath.Join(dirA, "a.craft")
	pathB := filepath.Join(dirB, "b.craft")
	if err := os.WriteFile(pathA, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{ws: workspace.New(nil), logger: slog.Default()}
	uriA, uriB := string(uri.File(pathA)), string(uri.File(pathB))
	s.ws.Open(uriA, content)
	s.ws.Open(uriB, content)

	editsA, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uriA)},
	})
	if err != nil {
		t.Fatalf("Formatting a: %v", err)
	}
	editsB, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uriB)},
	})
	if err != nil {
		t.Fatalf("Formatting b: %v", err)
	}
	if len(editsA) == 0 || !strings.Contains(editsA[0].NewText, "\n  Foo {") {
		t.Errorf("dirA (.craftfmt indent=2) got %+v", editsA)
	}
	if len(editsB) == 0 || !strings.Contains(editsB[0].NewText, "\n   Foo {") {
		t.Errorf("dirB (.craftfmt indent=3) got %+v", editsB)
	}
}

func TestFormattingUntitledBufferFallsBackToDefaults(t *testing.T) {
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	got := formatForURI(t, uri.URI("untitled:Untitled-1"), content)
	if !strings.Contains(got, "\n    Foo {") {
		t.Errorf("untitled buffer did not fall back to the default 4-space indent, got\n%q", got)
	}
}

// fakeConn is a minimal jsonrpc2.Conn that records every notification sent
// to it, so a test can assert the server actually told the client
// something, rather than merely that it did not crash.
type fakeConn struct {
	notified []fakeNotify
}

type fakeNotify struct {
	method string
	params interface{}
}

func (f *fakeConn) Call(context.Context, string, interface{}, interface{}) (jsonrpc2.ID, error) {
	return jsonrpc2.ID{}, nil
}
func (f *fakeConn) Notify(_ context.Context, method string, params interface{}) error {
	f.notified = append(f.notified, fakeNotify{method, params})
	return nil
}
func (f *fakeConn) Go(context.Context, jsonrpc2.Handler) {}
func (f *fakeConn) Close() error                         { return nil }
func (f *fakeConn) Done() <-chan struct{}                { return nil }
func (f *fakeConn) Err() error                           { return nil }

// TestFormattingMalformedCraftfmtIsVisibleNotSilent pins the error-surfacing
// mechanism: a malformed .craftfmt must produce neither a crash nor a
// silent default-formatted result -- the author must be told.
func TestFormattingMalformedCraftfmtIsVisibleNotSilent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	conn := &fakeConn{}
	s := &Server{ws: workspace.New(nil), logger: slog.Default(), conn: conn}
	docURI := string(uri.File(path))
	s.ws.Open(docURI, content)

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Formatting: %v", err)
	}
	if edits != nil {
		t.Errorf("malformed .craftfmt should not silently format with defaults, got edits: %+v", edits)
	}
	if len(conn.notified) != 1 || conn.notified[0].method != protocol.MethodWindowShowMessage {
		t.Fatalf("expected exactly one window/showMessage notification, got %+v", conn.notified)
	}
	params, ok := conn.notified[0].params.(*protocol.ShowMessageParams)
	if !ok {
		t.Fatalf("notification params type = %T, want *protocol.ShowMessageParams", conn.notified[0].params)
	}
	if params.Type != protocol.MessageTypeError {
		t.Errorf("message type = %v, want MessageTypeError", params.Type)
	}
	if !strings.Contains(params.Message, "indent") {
		t.Errorf("message %q does not mention the failing field", params.Message)
	}
}

// TestFormattingBlockedIsVisibleNotSilent pins the LSP counterpart of
// `craft fmt`'s "skipped, not formatted: <reason>" stderr line (see runFmt
// in cmd/craft/fmt.go): a document the formatter declines to touch --
// because it carries an error-severity diagnostic too severe to safely
// re-render -- must not come back as a silent no-op edit list. The author
// gets a window/showMessage explaining why, the same mechanism
// publishConfigError already uses for a bad .craftfmt.
func TestFormattingBlockedIsVisibleNotSilent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.craft")
	// Missing `{` after actors: a syntax error, so bailsFormatting declines
	// (FormatDocumentCheckedWith only sees syntax.Parse's diagnostics, not
	// sema's -- a purely semantic error like a duplicate name does not
	// block formatting).
	content := "actors\n  user Bob\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	conn := &fakeConn{}
	s := &Server{ws: workspace.New(nil), logger: slog.Default(), conn: conn}
	docURI := string(uri.File(path))
	s.ws.Open(docURI, content)

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Formatting: %v", err)
	}
	if edits != nil {
		t.Errorf("blocked document should return no edits, got: %+v", edits)
	}
	if len(conn.notified) != 1 || conn.notified[0].method != protocol.MethodWindowShowMessage {
		t.Fatalf("expected exactly one window/showMessage notification, got %+v", conn.notified)
	}
	params, ok := conn.notified[0].params.(*protocol.ShowMessageParams)
	if !ok {
		t.Fatalf("notification params type = %T, want *protocol.ShowMessageParams", conn.notified[0].params)
	}
	if !strings.Contains(params.Message, "not formatted") {
		t.Errorf("message %q does not explain that formatting was skipped", params.Message)
	}
}

// TestFormattingBlockedDoesNotRepeatIdenticalNotification pins the guard
// against an obnoxious failure mode: a client with files.autoSave:
// afterDelay re-requests textDocument/formatting on every keystroke-driven
// save, and a document with a standing parse error would otherwise pop the
// identical "not formatted" message on every single one until the author
// fixes it. The SAME block reason on a second request must not notify
// again; a DIFFERENT reason must.
func TestFormattingBlockedDoesNotRepeatIdenticalNotification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.craft")
	// Missing `{` after actors: craft/syntax/unexpected-token.
	content := "actors\n  user Bob\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	conn := &fakeConn{}
	s := &Server{ws: workspace.New(nil), logger: slog.Default(), conn: conn}
	docURI := string(uri.File(path))
	s.ws.Open(docURI, content)

	params := &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	}
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (1st): %v", err)
	}
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (2nd): %v", err)
	}
	if len(conn.notified) != 1 {
		t.Fatalf("identical block reason notified %d time(s) across 2 requests, want 1", len(conn.notified))
	}

	// A second, DIFFERENT parse error (an unclosed use_case block) must
	// notify again: it is not the same reason repeating.
	content2 := "use_case \"X\" {\n  when U does x\n    A asks B\n"
	s.ws.Open(docURI, content2)
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (3rd, different reason): %v", err)
	}
	if len(conn.notified) != 2 {
		t.Fatalf("a different block reason was not notified: got %d notification(s), want 2\n%+v", len(conn.notified), conn.notified)
	}
}

// TestFormattingBlockedNotificationClearsOnSuccess pins that fixing the file
// and then reintroducing the SAME error is treated as new, not swallowed by
// the dedup: a successful format in between must forget the remembered
// reason.
func TestFormattingBlockedNotificationClearsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.craft")
	// Missing `{` after actors: craft/syntax/unexpected-token.
	broken := "actors\n  user Bob\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	conn := &fakeConn{}
	s := &Server{ws: workspace.New(nil), logger: slog.Default(), conn: conn}
	docURI := string(uri.File(path))
	params := &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	}

	s.ws.Open(docURI, broken)
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (broken): %v", err)
	}
	if len(conn.notified) != 1 {
		t.Fatalf("first block notified %d time(s), want 1", len(conn.notified))
	}

	fixed := "actors {\n    user Bob\n}\n"
	s.ws.Open(docURI, fixed)
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (fixed): %v", err)
	}

	s.ws.Open(docURI, broken)
	if _, err := s.Formatting(context.Background(), params); err != nil {
		t.Fatalf("Formatting (broken again): %v", err)
	}
	if len(conn.notified) != 2 {
		t.Fatalf("the same reason recurring after a fix was not notified: got %d notification(s), want 2", len(conn.notified))
	}
}

// TestFormatting_CraftfmtEditTakesEffectWithoutReload is what replaced
// TestDidChangeWatchedFiles_CraftfmtChangeInvalidatesCache once
// resolveFmtConfig's per-directory cache was removed (see that function's
// doc comment). The mechanism changed -- there is no cache left to
// invalidate, and DidChangeWatchedFiles is now a documented no-op -- but the
// user-visible property the old test protected must survive unchanged:
// editing .craftfmt on disk changes what the NEXT format request produces,
// with no reload, no client notification, and no server restart. This test
// asserts exactly that, and deliberately never calls
// DidChangeWatchedFiles, to prove the property no longer depends on it.
func TestFormatting_CraftfmtEditTakesEffectWithoutReload(t *testing.T) {
	dir := t.TempDir()
	craftfmtPath := filepath.Join(dir, fmtconfig.FileName)
	if err := os.WriteFile(craftfmtPath, []byte("indent = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{ws: workspace.New(nil), logger: slog.Default()}
	docURI := string(uri.File(path))
	s.ws.Open(docURI, content)

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Formatting (before): %v", err)
	}
	if len(edits) == 0 || !strings.Contains(edits[0].NewText, "\n  Foo {") {
		t.Fatalf("did not observe indent=2 before the edit: %+v", edits)
	}

	if err := os.WriteFile(craftfmtPath, []byte("indent = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No DidChangeWatchedFiles call here. That is the point: the property
	// must hold without one.

	edits, err = s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Formatting (after): %v", err)
	}
	if len(edits) == 0 || !strings.Contains(edits[0].NewText, "\n   Foo {") {
		t.Errorf(".craftfmt edit was not reflected without a reload, got %+v", edits)
	}
}

// TestDidChangeWatchedFiles_IsANoOp pins that the handler required by
// protocol.Server does nothing observable: it must not error, and a format
// request right after it must still reflect the .craftfmt already on disk
// rather than anything the notification's payload claims.
func TestDidChangeWatchedFiles_IsANoOp(t *testing.T) {
	dir := t.TempDir()
	craftfmtPath := filepath.Join(dir, fmtconfig.FileName)
	if err := os.WriteFile(craftfmtPath, []byte("indent = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.craft")
	content := "services {\nFoo {\ncontexts: A\n}\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{ws: workspace.New(nil), logger: slog.Default()}
	docURI := string(uri.File(path))
	s.ws.Open(docURI, content)

	if err := s.DidChangeWatchedFiles(context.Background(), &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{
			{Type: protocol.FileChangeTypeChanged, URI: uri.File(craftfmtPath)},
		},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles: %v", err)
	}
	if err := s.DidChangeWatchedFiles(context.Background(), nil); err != nil {
		t.Fatalf("DidChangeWatchedFiles(nil): %v", err)
	}

	edits, err := s.Formatting(context.Background(), &protocol.DocumentFormattingParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(docURI)},
	})
	if err != nil {
		t.Fatalf("Formatting: %v", err)
	}
	if len(edits) == 0 || !strings.Contains(edits[0].NewText, "\n  Foo {") {
		t.Errorf("expected indent=2 from .craftfmt on disk, got %+v", edits)
	}
}
