// Package lsp implements the Craft Language Server Protocol server.
// S2: lifecycle scaffold only — initialize/initialized/shutdown/exit.
// ServerCapabilities declares only textDocumentSync (Q20 rule: never declare
// a capability the server doesn't actually serve).
package lsp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
)

// Server is the Craft LSP server. It satisfies protocol.Server.
type Server struct {
	conn     jsonrpc2.Conn
	logger   *slog.Logger
	shutdown bool
	exitCh   chan struct{}
}

// Serve reads JSON-RPC messages from r and writes responses to w until the
// client sends exit or the context is cancelled.
func Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	rwc := &readWriteCloser{r: r, w: w}
	stream := jsonrpc2.NewStream(rwc)
	conn := jsonrpc2.NewConn(stream)

	logger := slog.Default()

	srv := &Server{
		conn:   conn,
		logger: logger,
		exitCh: make(chan struct{}),
	}

	// The zap logger is required by go.lsp.dev/protocol's ServerHandler.
	// We use a no-op production logger; slog handles real logging on stderr.
	zapLogger, _ := zap.NewProduction(zap.WithCaller(false))
	defer zapLogger.Sync() //nolint:errcheck

	handler := withPanicRecovery(
		protocol.ServerHandler(srv, jsonrpc2.MethodNotFoundHandler),
		logger,
	)

	conn.Go(ctx, handler)

	select {
	case <-ctx.Done():
		return conn.Close()
	case <-conn.Done():
		return conn.Err()
	case <-srv.exitCh:
		return conn.Close()
	}
}

// --- lifecycle ---

func (s *Server) Initialize(_ context.Context, _ *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.logger.Info("craft lsp: initialize")
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			// Q20: declare only textDocumentSync for now.
			// Later slices add capabilities alongside their handlers.
			TextDocumentSync: protocol.TextDocumentSyncKindFull,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "craft-lsp",
			Version: "0.0.0",
		},
	}, nil
}

func (s *Server) Initialized(_ context.Context, _ *protocol.InitializedParams) error {
	s.logger.Info("craft lsp: initialized")
	return nil
}

func (s *Server) Shutdown(_ context.Context) error {
	s.logger.Info("craft lsp: shutdown")
	s.shutdown = true
	return nil
}

func (s *Server) Exit(_ context.Context) error {
	s.logger.Info("craft lsp: exit")
	if !s.shutdown {
		// LSP spec: if client exits without a preceding shutdown, exit with code 1.
		os.Exit(1)
	}
	close(s.exitCh)
	return nil
}

// --- unimplemented stubs (S3+ will fill these in) ---

func (s *Server) WorkDoneProgressCancel(_ context.Context, _ *protocol.WorkDoneProgressCancelParams) error {
	return nil
}

func (s *Server) LogTrace(_ context.Context, params *protocol.LogTraceParams) error {
	s.logger.Debug("$/logTrace", "message", params.Message)
	return nil
}

func (s *Server) SetTrace(_ context.Context, params *protocol.SetTraceParams) error {
	s.logger.Debug("$/setTrace", "value", params.Value)
	return nil
}

func (s *Server) CodeAction(_ context.Context, _ *protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	return nil, nil
}

func (s *Server) CodeLens(_ context.Context, _ *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	return nil, nil
}

func (s *Server) CodeLensResolve(_ context.Context, params *protocol.CodeLens) (*protocol.CodeLens, error) {
	return params, nil
}

func (s *Server) ColorPresentation(_ context.Context, _ *protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	return nil, nil
}

func (s *Server) Completion(_ context.Context, _ *protocol.CompletionParams) (*protocol.CompletionList, error) {
	return nil, nil
}

func (s *Server) CompletionResolve(_ context.Context, params *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return params, nil
}

func (s *Server) Declaration(_ context.Context, _ *protocol.DeclarationParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) Definition(_ context.Context, _ *protocol.DefinitionParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) DidChange(_ context.Context, _ *protocol.DidChangeTextDocumentParams) error {
	return nil
}

func (s *Server) DidChangeConfiguration(_ context.Context, _ *protocol.DidChangeConfigurationParams) error {
	return nil
}

func (s *Server) DidChangeWatchedFiles(_ context.Context, _ *protocol.DidChangeWatchedFilesParams) error {
	return nil
}

func (s *Server) DidChangeWorkspaceFolders(_ context.Context, _ *protocol.DidChangeWorkspaceFoldersParams) error {
	return nil
}

func (s *Server) DidClose(_ context.Context, _ *protocol.DidCloseTextDocumentParams) error {
	return nil
}

func (s *Server) DidOpen(_ context.Context, _ *protocol.DidOpenTextDocumentParams) error {
	return nil
}

func (s *Server) DidSave(_ context.Context, _ *protocol.DidSaveTextDocumentParams) error {
	return nil
}

func (s *Server) DocumentColor(_ context.Context, _ *protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	return nil, nil
}

func (s *Server) DocumentHighlight(_ context.Context, _ *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	return nil, nil
}

func (s *Server) DocumentLink(_ context.Context, _ *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	return nil, nil
}

func (s *Server) DocumentLinkResolve(_ context.Context, params *protocol.DocumentLink) (*protocol.DocumentLink, error) {
	return params, nil
}

func (s *Server) DocumentSymbol(_ context.Context, _ *protocol.DocumentSymbolParams) ([]interface{}, error) {
	return nil, nil
}

func (s *Server) ExecuteCommand(_ context.Context, _ *protocol.ExecuteCommandParams) (interface{}, error) {
	return nil, nil
}

func (s *Server) FoldingRanges(_ context.Context, _ *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	return nil, nil
}

func (s *Server) Formatting(_ context.Context, _ *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	return nil, nil
}

func (s *Server) Hover(_ context.Context, _ *protocol.HoverParams) (*protocol.Hover, error) {
	return nil, nil
}

func (s *Server) Implementation(_ context.Context, _ *protocol.ImplementationParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) OnTypeFormatting(_ context.Context, _ *protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, nil
}

func (s *Server) PrepareRename(_ context.Context, _ *protocol.PrepareRenameParams) (*protocol.Range, error) {
	return nil, nil
}

func (s *Server) RangeFormatting(_ context.Context, _ *protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	return nil, nil
}

func (s *Server) References(_ context.Context, _ *protocol.ReferenceParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) Rename(_ context.Context, _ *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	return nil, nil
}

func (s *Server) SignatureHelp(_ context.Context, _ *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	return nil, nil
}

func (s *Server) Symbols(_ context.Context, _ *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	return nil, nil
}

func (s *Server) TypeDefinition(_ context.Context, _ *protocol.TypeDefinitionParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) WillSave(_ context.Context, _ *protocol.WillSaveTextDocumentParams) error {
	return nil
}

func (s *Server) WillSaveWaitUntil(_ context.Context, _ *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	return nil, nil
}

func (s *Server) ShowDocument(_ context.Context, _ *protocol.ShowDocumentParams) (*protocol.ShowDocumentResult, error) {
	return &protocol.ShowDocumentResult{Success: false}, nil
}

func (s *Server) WillCreateFiles(_ context.Context, _ *protocol.CreateFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, nil
}

func (s *Server) DidCreateFiles(_ context.Context, _ *protocol.CreateFilesParams) error {
	return nil
}

func (s *Server) WillRenameFiles(_ context.Context, _ *protocol.RenameFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, nil
}

func (s *Server) DidRenameFiles(_ context.Context, _ *protocol.RenameFilesParams) error {
	return nil
}

func (s *Server) WillDeleteFiles(_ context.Context, _ *protocol.DeleteFilesParams) (*protocol.WorkspaceEdit, error) {
	return nil, nil
}

func (s *Server) DidDeleteFiles(_ context.Context, _ *protocol.DeleteFilesParams) error {
	return nil
}

func (s *Server) CodeLensRefresh(_ context.Context) error {
	return nil
}

func (s *Server) PrepareCallHierarchy(_ context.Context, _ *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	return nil, nil
}

func (s *Server) IncomingCalls(_ context.Context, _ *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	return nil, nil
}

func (s *Server) OutgoingCalls(_ context.Context, _ *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	return nil, nil
}

func (s *Server) SemanticTokensFull(_ context.Context, _ *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	return nil, nil
}

func (s *Server) SemanticTokensFullDelta(_ context.Context, _ *protocol.SemanticTokensDeltaParams) (interface{}, error) {
	return nil, nil
}

func (s *Server) SemanticTokensRange(_ context.Context, _ *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	return nil, nil
}

func (s *Server) SemanticTokensRefresh(_ context.Context) error {
	return nil
}

func (s *Server) LinkedEditingRange(_ context.Context, _ *protocol.LinkedEditingRangeParams) (*protocol.LinkedEditingRanges, error) {
	return nil, nil
}

func (s *Server) Moniker(_ context.Context, _ *protocol.MonikerParams) ([]protocol.Moniker, error) {
	return nil, nil
}

func (s *Server) Request(_ context.Context, method string, _ interface{}) (interface{}, error) {
	return nil, fmt.Errorf("unknown request: %s", method)
}

// readWriteCloser wraps a separate reader and writer into an io.ReadWriteCloser.
type readWriteCloser struct {
	r io.Reader
	w io.Writer
}

func (rwc *readWriteCloser) Read(p []byte) (int, error)  { return rwc.r.Read(p) }
func (rwc *readWriteCloser) Write(p []byte) (int, error) { return rwc.w.Write(p) }
func (rwc *readWriteCloser) Close() error                { return nil }
