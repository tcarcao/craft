// Package lsp implements the Craft Language Server Protocol server.
// S3: workspace + diagnostics + documentSymbol + hover handlers.
// S4: semanticTokens (actor vs domain distinct colouring).
// ServerCapabilities is extended per Q20 (only declare what's implemented).
package lsp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/workspace"
	"github.com/tcarcao/craft/pkg/craft"
)

// Server is the Craft LSP server. It satisfies protocol.Server.
type Server struct {
	conn     jsonrpc2.Conn
	logger   *slog.Logger
	ws       *workspace.Workspace
	shutdown bool
	exitCh   chan struct{}

	// debounce state for publishDiagnostics
	debounce struct {
		mu     sync.Mutex
		timers map[string]*time.Timer
	}
}

// Serve reads JSON-RPC messages from r and writes responses to w until the
// client sends exit or the context is cancelled.
func Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	rwc := &readWriteCloser{r: r, w: w}
	stream := jsonrpc2.NewStream(rwc)
	conn := jsonrpc2.NewConn(stream)

	logger := slog.Default()
	ws := workspace.New(logger)

	srv := &Server{
		conn:   conn,
		logger: logger,
		ws:     ws,
		exitCh: make(chan struct{}),
	}
	srv.debounce.timers = make(map[string]*time.Timer)

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

func (s *Server) Initialize(_ context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.logger.Info("craft lsp: initialize")

	// Initialize workspace from rootUri/rootPath if provided (Q15).
	if params != nil && params.RootURI != "" {
		rootPath := uriToPath(string(params.RootURI))
		if rootPath != "" {
			go s.ws.Initialize(rootPath)
		}
	}

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			// Q20: declare only capabilities we actually serve.
			// S3: documentSymbol, hover.
			// S4: semanticTokens (full) with actor/domain token types.
			TextDocumentSync:       protocol.TextDocumentSyncKindFull,
			DocumentSymbolProvider: true,
			HoverProvider:          true,
			// SemanticTokensProvider is interface{} in this protocol version;
			// use an inline struct so it serialises with Legend + Full fields.
			SemanticTokensProvider: semanticTokensOptions(),
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

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if params == nil || len(params.ContentChanges) == 0 {
		return nil
	}
	uri := string(params.TextDocument.URI)
	content := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.ws.Change(uri, content)
	s.scheduleDiagnostics(ctx, uri)
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

func (s *Server) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if params == nil {
		return nil
	}
	s.ws.Close(string(params.TextDocument.URI))
	return nil
}

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if params == nil {
		return nil
	}
	uri := string(params.TextDocument.URI)
	s.ws.Open(uri, params.TextDocument.Text)
	s.scheduleDiagnostics(ctx, uri)
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

func (s *Server) DocumentSymbol(_ context.Context, params *protocol.DocumentSymbolParams) ([]interface{}, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil || f.AST == nil {
		return nil, nil
	}

	var syms []interface{}
	for _, a := range f.AST.Actors {
		line := 0
		if a.Line > 0 {
			line = a.Line - 1
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           a.Name,
			Kind:           protocol.SymbolKindObject,
			Detail:         "actor: " + string(a.Type),
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
		})
	}
	for _, d := range f.AST.Domains {
		line := 0
		if d.Line > 0 {
			line = d.Line - 1
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           d.Name,
			Kind:           protocol.SymbolKindNamespace,
			Detail:         "domain",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
		})
	}
	return syms, nil
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

func (s *Server) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil || f.AST == nil {
		return nil, nil
	}

	cursorLine := int(params.Position.Line) + 1 // convert to 1-based

	for _, a := range f.AST.Actors {
		declLine := a.Line
		if declLine == 0 {
			// Individual `actor` statements don't have a line recorded in AST;
			// hover will not resolve them until full position tracking lands.
			continue
		}
		if declLine == cursorLine {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: "actor: " + a.Name + " (" + string(a.Type) + ")",
				},
			}, nil
		}
	}

	for _, d := range f.AST.Domains {
		if d.Line == 0 {
			continue
		}
		if d.Line == cursorLine {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: "domain: " + d.Name,
				},
			}, nil
		}
	}

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

// semanticTokenTypeActor and semanticTokenTypeDomain are the two custom token
// types advertised in the legend. Their index in the legend slice is their
// numeric encoding in the data stream.
const (
	semanticTokenTypeActor  protocol.SemanticTokenTypes = "class"     // index 0
	semanticTokenTypeDomain protocol.SemanticTokenTypes = "namespace" // index 1
)

// semanticTokensOptions returns the capability descriptor for semantic tokens.
// The go.lsp.dev/protocol version we use has an incomplete SemanticTokensOptions
// struct (missing Legend/Full), so we use an inline anonymous struct that
// marshals to the correct JSON shape.
func semanticTokensOptions() interface{} {
	return struct {
		Legend struct {
			TokenTypes     []string `json:"tokenTypes"`
			TokenModifiers []string `json:"tokenModifiers"`
		} `json:"legend"`
		Full bool `json:"full"`
	}{
		Legend: struct {
			TokenTypes     []string `json:"tokenTypes"`
			TokenModifiers []string `json:"tokenModifiers"`
		}{
			TokenTypes:     []string{string(semanticTokenTypeActor), string(semanticTokenTypeDomain)},
			TokenModifiers: []string{string(protocol.SemanticTokenModifierDeclaration)},
		},
		Full: true,
	}
}

// semanticTokenTypeIndex returns the legend index for a token type.
func semanticTokenTypeIndex(t protocol.SemanticTokenTypes) uint32 {
	switch t {
	case semanticTokenTypeActor:
		return 0
	case semanticTokenTypeDomain:
		return 1
	default:
		return 0
	}
}

// semanticToken records one token before encoding.
type semanticToken struct {
	line      uint32 // 0-based LSP line
	startChar uint32 // 0-based character offset
	length    uint32
	tokenType uint32
	modifiers uint32
}

// encodeSemanticTokens converts a slice of semanticToken (already sorted by
// line, then startChar) into the LSP relative-encoding uint32 stream.
func encodeSemanticTokens(tokens []semanticToken) []uint32 {
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevChar uint32
	for _, t := range tokens {
		deltaLine := t.line - prevLine
		deltaChar := t.startChar
		if deltaLine == 0 {
			deltaChar = t.startChar - prevChar
		}
		data = append(data, deltaLine, deltaChar, t.length, t.tokenType, t.modifiers)
		prevLine = t.line
		prevChar = t.startChar
	}
	return data
}

func (s *Server) SemanticTokensFull(_ context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil || f.AST == nil {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}

	var tokens []semanticToken

	// Actors: token type index 0 ("class"), modifier 1 (declaration).
	for _, a := range f.AST.Actors {
		if a.Line <= 0 {
			continue // individual `actor` stmts without position — skip for now
		}
		tokens = append(tokens, semanticToken{
			line:      uint32(a.Line - 1),
			startChar: 0, // actor name position — approximate until full position tracking
			length:    uint32(len([]rune(a.Name))),
			tokenType: semanticTokenTypeIndex(semanticTokenTypeActor),
			modifiers: 1, // declaration
		})
	}

	// Domains: token type index 1 ("namespace"), modifier 1 (declaration).
	for _, d := range f.AST.Domains {
		if d.Line <= 0 {
			continue
		}
		tokens = append(tokens, semanticToken{
			line:      uint32(d.Line - 1),
			startChar: 0,
			length:    uint32(len([]rune(d.Name))),
			tokenType: semanticTokenTypeIndex(semanticTokenTypeDomain),
			modifiers: 1, // declaration
		})
	}

	// Sort tokens by line, then character (required for relative encoding).
	sortSemanticTokens(tokens)

	return &protocol.SemanticTokens{
		Data: encodeSemanticTokens(tokens),
	}, nil
}

// sortSemanticTokens sorts tokens ascending by (line, startChar).
func sortSemanticTokens(tokens []semanticToken) {
	for i := 1; i < len(tokens); i++ {
		for j := i; j > 0; j-- {
			a, b := tokens[j-1], tokens[j]
			if a.line > b.line || (a.line == b.line && a.startChar > b.startChar) {
				tokens[j-1], tokens[j] = tokens[j], tokens[j-1]
			} else {
				break
			}
		}
	}
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

// scheduleDiagnostics debounces publishDiagnostics for the given URI using
// a 150ms timer (Q3a / C2). Concurrent calls for the same URI reset the timer.
func (s *Server) scheduleDiagnostics(ctx context.Context, uri string) {
	s.debounce.mu.Lock()
	defer s.debounce.mu.Unlock()
	if t, ok := s.debounce.timers[uri]; ok {
		t.Stop()
	}
	s.debounce.timers[uri] = time.AfterFunc(150*time.Millisecond, func() {
		s.publishDiagnostics(ctx, uri)
	})
}

// publishDiagnostics runs sema on the cached parse result and pushes
// textDocument/publishDiagnostics to the client.
func (s *Server) publishDiagnostics(ctx context.Context, uri string) {
	f := s.ws.Get(uri)
	if f == nil {
		return
	}

	_, semaDiags := sema.AnalyzeFile(uri, f.AST)
	allDiags := append(f.Diagnostics, semaDiags...)

	var lspDiags []protocol.Diagnostic
	for _, d := range allDiags {
		lspDiags = append(lspDiags, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(d.Range.Start.Line),
					Character: uint32(d.Range.Start.Character),
				},
				End: protocol.Position{
					Line:      uint32(d.Range.End.Line),
					Character: uint32(d.Range.End.Character),
				},
			},
			Severity: lspSeverity(d.Severity),
			Code:     d.Code,
			Message:  d.Message,
			Source:   "craft",
		})
	}
	if lspDiags == nil {
		lspDiags = []protocol.Diagnostic{}
	}

	if err := s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics,
		&protocol.PublishDiagnosticsParams{
			URI:         protocol.DocumentURI(uri),
			Diagnostics: lspDiags,
		}); err != nil {
		s.logger.Warn("publishDiagnostics: notify failed", "uri", uri, "err", err)
	}
}

func lspSeverity(s craft.Severity) protocol.DiagnosticSeverity {
	switch s {
	case craft.SeverityError:
		return protocol.DiagnosticSeverityError
	case craft.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case craft.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	default:
		return protocol.DiagnosticSeverityHint
	}
}

// uriToPath converts a file:// URI to an OS path.
func uriToPath(uri string) string {
	const prefix = "file://"
	if len(uri) > len(prefix) && uri[:len(prefix)] == prefix {
		return uri[len(prefix):]
	}
	return ""
}

// readWriteCloser wraps a separate reader and writer into an io.ReadWriteCloser.
type readWriteCloser struct {
	r io.Reader
	w io.Writer
}

func (rwc *readWriteCloser) Read(p []byte) (int, error)  { return rwc.r.Read(p) }
func (rwc *readWriteCloser) Write(p []byte) (int, error) { return rwc.w.Write(p) }
func (rwc *readWriteCloser) Close() error                { return nil }
