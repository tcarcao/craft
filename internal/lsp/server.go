// Package lsp implements the Craft Language Server Protocol server.
// S3: workspace + diagnostics + documentSymbol + hover handlers.
// S4: semanticTokens (actor vs domain distinct colouring).
// S5: definition (go-to-definition for service contexts→domain); service
//     symbols in documentSymbol/hover/semanticTokens; workspace/executeCommand
//     for EXTRACT_DOMAINS_FROM_CURRENT and EXTRACT_DOMAINS_FROM_WORKSPACE (Q16).
// S6: hover/definition/semanticTokens extended to cover actor/domain/service
//     references inside use-case trigger subjects and action parties.
//     use_case blocks added to documentSymbol outline.
// S7: arch blocks added to documentSymbol outline; FoldingRanges handler
//     returns one fold per arch block plus one per labelled segment inside arch.
// S8: exposure blocks added to documentSymbol outline and hover.
// ServerCapabilities is extended per Q20 (only declare what's implemented).
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
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

	mu         sync.Mutex
	traceLevel protocol.TraceValue

	// debounce state for publishDiagnostics
	debounce struct {
		mu     sync.Mutex
		timers map[string]*time.Timer
	}
}

// lspLine converts a 1-based line to a 0-based LSP line number.
func lspLine(line int) uint32 {
	if line <= 0 {
		return 0
	}
	return uint32(line - 1)
}

// lspPosFromOffset converts a byte offset to an LSP Position using LineIndex.
func lspPosFromOffset(li green.LineIndex, src string, offset green.TextSize) protocol.Position {
	line, _ := li.LineCol(offset)
	startChar := li.UTF16Col(src, offset)
	return protocol.Position{Line: lspLine(line), Character: startChar}
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
		withInlayHints(
			protocol.ServerHandler(srv, jsonrpc2.MethodNotFoundHandler),
			ws,
			logger,
		),
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

	if params != nil && params.Trace != "" {
		s.mu.Lock()
		s.traceLevel = params.Trace
		s.mu.Unlock()
	}

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			// Q20: declare only capabilities we actually serve.
			// S3: documentSymbol, hover.
			// S4: semanticTokens (full) with actor/domain/service token types.
			// S5: definition (go-to-definition), executeCommand (custom LSP commands).
			// S7: foldingRange (arch blocks + labelled segments).
			TextDocumentSync:       protocol.TextDocumentSyncKindFull,
			DocumentSymbolProvider: true,
			HoverProvider:          true,
			DefinitionProvider:     true,
			FoldingRangeProvider:        true,
			DocumentFormattingProvider: true,
			// SemanticTokensProvider is interface{} in this protocol version;
			// use an inline struct so it serialises with Legend + Full fields.
			SemanticTokensProvider: semanticTokensOptions(),
			// CompletionProvider advertises ':' as a completion trigger character.
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":"},
			},
			// ExecuteCommandProvider lists the custom LSP commands we handle.
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{
					"CRAFT_EXTRACT_WORKSPACE",
					"craft.extractDslFromBlockRanges",
				},
			},
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
	s.logger.Debug("$/logTrace (from client)", "message", params.Message)
	return nil
}

// notifyLogTrace sends a $/logTrace notification to the client when the trace
// level is "messages" or "verbose". Safe to call from any goroutine.
func (s *Server) notifyLogTrace(ctx context.Context, message string) {
	s.mu.Lock()
	level := s.traceLevel
	s.mu.Unlock()
	if level != protocol.TraceVerbose {
		return
	}
	_ = s.conn.Notify(ctx, "$/logTrace", &protocol.LogTraceParams{Message: message})
}

func (s *Server) SetTrace(_ context.Context, params *protocol.SetTraceParams) error {
	s.mu.Lock()
	s.traceLevel = params.Value
	s.mu.Unlock()
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

func (s *Server) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if params == nil {
		return nil, nil
	}
	return buildCompletions(s.ws, string(params.TextDocument.URI), params), nil
}

func (s *Server) CompletionResolve(_ context.Context, params *protocol.CompletionItem) (*protocol.CompletionItem, error) {
	return params, nil
}

func (s *Server) Declaration(_ context.Context, _ *protocol.DeclarationParams) ([]protocol.Location, error) {
	return nil, nil
}

func (s *Server) Definition(_ context.Context, params *protocol.DefinitionParams) ([]protocol.Location, error) {
	if params == nil {
		return nil, nil
	}
	uri := string(params.TextDocument.URI)
	f := s.ws.Get(uri)
	if f == nil {
		return nil, nil
	}
	file := syntax.AsFile(syntax.Root(f.Green))

	cursorLine := int(params.Position.Line) + 1     // convert to 1-based
	cursorChar := int(params.Position.Character) + 1 // convert to 1-based
	rm := s.ws.ResolutionMap()

	// Walk services: match cursor against each context reference line so that
	// go-to-definition fires when the cursor is on a `contexts:` entry.
	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		ctxNames := svc.Contexts()
		ctxToks := svc.ContextTokens()
		for i, ctxName := range ctxNames {
			if i >= len(ctxToks) {
				continue
			}
			ctxLine, ctxCol := f.LineIndex.LineCol(ctxToks[i].Offset())
			if ctxLine != cursorLine {
				continue
			}
			// cursorChar is 1-based; ctxCol is 1-based start of this token.
			// Only match if the cursor falls within [ctxCol, ctxCol+len(ctxName)).
			if cursorChar < ctxCol || cursorChar >= ctxCol+len(ctxName) {
				continue
			}
			domSym, ok := sema.ResolveServiceContext(rm, uri, nameTok.Text(), ctxName)
			if !ok {
				continue
			}
			domStartChar := uint32(0)
			if domSym.Column > 0 {
				domStartChar = uint32(domSym.Column - 1)
			}
			return []protocol.Location{{
				URI: protocol.DocumentURI(domSym.URI),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(domSym.Line - 1), Character: domStartChar},
					End:   protocol.Position{Line: uint32(domSym.Line - 1), Character: domStartChar + uint32(len(domSym.Name))},
				},
			}}, nil
		}
	}

	// Walk use-case action parties: cursor on an action line resolves to the
	// actor/domain/service declaration.
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			// Check trigger subject line.
			whenTok := sc.When()
			triggerLine := 0
			if whenTok != nil {
				triggerLine, _ = f.LineIndex.LineCol(whenTok.Offset())
			}
			if triggerLine == cursorLine {
				trigger := sc.Trigger()
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, trigger.ActorName(), triggerLine); ok {
					return []protocol.Location{loc}, nil
				}
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, trigger.ContextName(), triggerLine); ok {
					return []protocol.Location{loc}, nil
				}
			}
			for _, action := range sc.Actions() {
				actionLine := action.Line(f.LineIndex)
				if actionLine != cursorLine {
					continue
				}
				// If cursor falls on or past TargetContextColumn, try TargetContext first.
				targetCol := action.TargetCol(f.LineIndex)
				if targetCol > 0 && action.TargetName() != "" &&
					cursorChar >= targetCol {
					if loc, ok := resolveUseCaseRefToLocation(rm, uri, action.TargetName(), actionLine); ok {
						return []protocol.Location{loc}, nil
					}
					// TargetName unresolved — fall through to "from" party.
				}
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, action.SubjectName(), actionLine); ok {
					return []protocol.Location{loc}, nil
				}
				// When no column info is available, also try TargetName as a plain fallback.
				if targetCol == 0 {
					if loc, ok := resolveUseCaseRefToLocation(rm, uri, action.TargetName(), actionLine); ok {
						return []protocol.Location{loc}, nil
					}
				}
			}
		}
	}

	return nil, nil
}

// resolveUseCaseRefToLocation looks up a name from a use-case body in the
// ResolutionMap and returns a protocol.Location pointing at the declaration.
func resolveUseCaseRefToLocation(rm sema.ResolutionMap, uri, name string, line int) (protocol.Location, bool) {
	if name == "" {
		return protocol.Location{}, false
	}
	target, ok := sema.ResolveUseCaseRef(rm, uri, name, line)
	if !ok {
		return protocol.Location{}, false
	}
	switch target.Kind {
	case "actor":
		sym := target.Actor
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		startChar := uint32(0)
		if sym.Column > 0 {
			startChar = uint32(sym.Column - 1)
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: startChar},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: startChar + uint32(len(sym.Name))},
			},
		}, true
	case "domain":
		sym := target.Domain
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		startChar := uint32(0)
		if sym.Column > 0 {
			startChar = uint32(sym.Column - 1)
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: startChar},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: startChar + uint32(len(sym.Name))},
			},
		}, true
	case "bounded_context":
		if target.BCLine > 0 {
			col := uint32(0)
			if target.BCColumn > 0 {
				col = uint32(target.BCColumn - 1)
			}
			return protocol.Location{
				URI: protocol.DocumentURI(target.BCURI),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(target.BCLine - 1), Character: col},
					End:   protocol.Position{Line: uint32(target.BCLine - 1), Character: col + uint32(len(name))},
				},
			}, true
		}
		// Fallback: navigate to the parent domain if BC position is missing.
		sym := target.Domain
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		domFbStartChar := uint32(0)
		if sym.Column > 0 {
			domFbStartChar = uint32(sym.Column - 1)
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: domFbStartChar},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: domFbStartChar + uint32(len(sym.Name))},
			},
		}, true
	case "service":
		sym := target.Service
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		startChar := uint32(0)
		if sym.Column > 0 {
			startChar = uint32(sym.Column - 1)
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: startChar},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: startChar + uint32(len(sym.Name))},
			},
		}, true
	}
	return protocol.Location{}, false
}

func (s *Server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	if params == nil || len(params.ContentChanges) == 0 {
		return nil
	}
	uri := string(params.TextDocument.URI)
	content := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.ws.Change(uri, content)
	s.notifyLogTrace(ctx, fmt.Sprintf("didChange: parsed %s", uri))
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

func (s *Server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	if params == nil {
		return nil
	}
	s.ws.Close(string(params.TextDocument.URI))
	// Re-publish diagnostics for all remaining files: closing a domain file
	// can resolve or create unresolved-reference diagnostics in service files.
	for _, f := range s.ws.AllFiles() {
		s.scheduleDiagnostics(ctx, f.URI)
	}
	return nil
}

func (s *Server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	if params == nil {
		return nil
	}
	uri := string(params.TextDocument.URI)
	s.ws.Open(uri, params.TextDocument.Text)
	s.notifyLogTrace(ctx, fmt.Sprintf("didOpen: parsed %s", uri))
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
	if f == nil {
		return nil, nil
	}
	file := syntax.AsFile(syntax.Root(f.Green))

	var syms []interface{}
	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		line := lspLine(tokLine)
		syms = append(syms, protocol.DocumentSymbol{
			Name:           nameTok.Text(),
			Kind:           protocol.SymbolKindModule,
			Detail:         "service",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil {
			continue
		}
		tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		line := lspLine(tokLine)
		syms = append(syms, protocol.DocumentSymbol{
			Name:           nameTok.Text(),
			Kind:           protocol.SymbolKindObject,
			Detail:         "actor: " + a.ActorTypeValue(),
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		tokLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		line := lspLine(tokLine)
		syms = append(syms, protocol.DocumentSymbol{
			Name:           nameTok.Text(),
			Kind:           protocol.SymbolKindNamespace,
			Detail:         "domain",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	for _, uc := range file.UseCases() {
		kwTok := uc.Keyword()
		if kwTok == nil {
			continue
		}
		tokLine, _ := f.LineIndex.LineCol(kwTok.Offset())
		line := lspLine(tokLine)
		name := ""
		if titleTok := uc.Title(); titleTok != nil {
			name = titleTok.Text()
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:   name,
			Kind:   protocol.SymbolKindEvent, // closest match for use-case-level interactions
			Detail: "use_case",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	for _, arch := range file.Archs() {
		archLine := arch.Line(f.LineIndex)
		line := lspLine(archLine)
		name := ""
		if nameTok := arch.Name(); nameTok != nil {
			name = nameTok.Text()
		}
		if name == "" {
			name = "arch"
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           name,
			Kind:           protocol.SymbolKindPackage,
			Detail:         "arch",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	for _, exp := range file.Exposures() {
		nameTok := exp.Name()
		if nameTok == nil {
			continue
		}
		expLine := exp.Line(f.LineIndex)
		line := lspLine(expLine)
		syms = append(syms, protocol.DocumentSymbol{
			Name:           nameTok.Text(),
			Kind:           protocol.SymbolKindInterface,
			Detail:         "exposure",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
			Range:          protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		})
	}
	return syms, nil
}

// ExecuteCommand handles workspace/executeCommand for custom Craft LSP commands.
func (s *Server) ExecuteCommand(_ context.Context, params *protocol.ExecuteCommandParams) (interface{}, error) {
	if params == nil {
		return nil, nil
	}
	switch params.Command {
	case "CRAFT_EXTRACT_WORKSPACE":
		return s.craftExtractWorkspace(params.Arguments), nil
	case "craft.extractDslFromBlockRanges":
		return s.extractPartialDslFromBlockRanges(params.Arguments), nil
	}
	return nil, nil
}

// craftExtractWorkspace returns the full flat DSL schema across all workspace files.
// The optional first argument is the current file URI used to set inCurrentFile flags.
func (s *Server) craftExtractWorkspace(args []interface{}) interface{} {
	var currentFileURI string
	if len(args) > 0 {
		switch v := args[0].(type) {
		case string:
			currentFileURI = v
		case json.RawMessage:
			_ = json.Unmarshal(v, &currentFileURI)
		}
	}

	wsSym := s.ws.WorkspaceSymbols()
	result := craftExtractionResult{}

	for _, svc := range wsSym.Services {
		result.Services = append(result.Services, craftServiceEntry{
			Name:          svc.Name,
			URI:           svc.URI,
			StartLine:     svc.Line,
			EndLine:       svc.EndLine,
			Contexts:      svc.Contexts,
			InCurrentFile: currentFileURI != "" && svc.URI == currentFileURI,
		})
	}

	for _, dom := range wsSym.Domains {
		var bcs []craftBCEntry
		for _, bcName := range dom.BoundedContexts {
			bcs = append(bcs, craftBCEntry{Name: bcName})
		}
		result.Domains = append(result.Domains, craftDomainEntry{
			Name:            dom.Name,
			URI:             dom.URI,
			StartLine:       dom.Line,
			EndLine:         dom.EndLine,
			BoundedContexts: bcs,
			InCurrentFile:   currentFileURI != "" && dom.URI == currentFileURI,
		})
	}

	for _, actor := range wsSym.Actors {
		result.Actors = append(result.Actors, craftActorEntry{
			Name:          actor.Name,
			ActorType:     string(actor.Type),
			URI:           actor.URI,
			StartLine:     actor.Line,
			EndLine:       actor.Line, // actors are single-line within a block
			InCurrentFile: currentFileURI != "" && actor.URI == currentFileURI,
		})
	}

	for _, f := range s.ws.AllFiles() {
		if f.Green == nil {
			continue
		}
		fFile := syntax.AsFile(syntax.Root(f.Green))
		for _, uc := range fFile.UseCases() {
			titleTok := uc.Title()
			kwTok := uc.Keyword()
			if titleTok == nil {
				continue
			}
			ucLine := 0
			if kwTok != nil {
				ucLine, _ = f.LineIndex.LineCol(kwTok.Offset())
			}
			entryPoint, involved := extractUseCaseContextsFromView(uc)
			result.UseCases = append(result.UseCases, craftUseCaseEntry{
				Name:              titleTok.Text(),
				URI:               f.URI,
				StartLine:         ucLine,
				EndLine:           uc.EndLine(f.LineIndex),
				InCurrentFile:     currentFileURI != "" && f.URI == currentFileURI,
				EntryPointContext: entryPoint,
				InvolvedContexts:  involved,
			})
		}
		for _, block := range fFile.ActorBlocks() {
			result.ActorBlocks = append(result.ActorBlocks, craftBlockRef{
				FileURI:   f.URI,
				StartLine: block.Line(f.LineIndex),
				EndLine:   block.EndLine(f.LineIndex),
			})
		}
		for _, arch := range fFile.Archs() {
			nameTok := arch.Name()
			name := ""
			if nameTok != nil {
				name = nameTok.Text()
			}
			result.Archs = append(result.Archs, craftArchEntry{
				Name:          name,
				URI:           f.URI,
				StartLine:     arch.Line(f.LineIndex),
				EndLine:       arch.EndLine(f.LineIndex),
				InCurrentFile: currentFileURI != "" && f.URI == currentFileURI,
			})
		}
	}

	return result
}

// extractUseCaseContextsFromView derives the entry-point bounded context and all involved contexts
// from a use case declaration by inspecting its scenario triggers and actions.
// The entry point is the first domain that appears (trigger domain for domain_listen,
// otherwise the first action subject). Involved is the deduplicated set of all subject
// and target values across every scenario.
func extractUseCaseContextsFromView(uc syntax.UseCaseDecl) (entryPoint string, involved []string) {
	seen := make(map[string]struct{})
	add := func(name string) {
		if name == "" {
			return
		}
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			involved = append(involved, name)
		}
		if entryPoint == "" {
			entryPoint = name
		}
	}
	for _, sc := range uc.Scenarios() {
		trigger := sc.Trigger()
		if trigger.Kind() == "domain_listen" {
			add(trigger.ContextName())
		}
		for _, action := range sc.Actions() {
			add(action.SubjectName())
			add(action.TargetName())
		}
	}
	return entryPoint, involved
}

// extractPartialDslFromBlockRanges reconstructs well-formed DSL for the given block ranges.
// Ranges are grouped by file so each file's AST is accessed only once.
// Services and domains are reconstructed from AST fields with correct wrapping.
// Ranges that don't match any declaration are silently skipped.
// Remaining ranges (actor blocks, arch blocks) fall back to raw line slicing.
func (s *Server) extractPartialDslFromBlockRanges(args []interface{}) interface{} {
	if len(args) == 0 {
		return ""
	}

	type blockRange struct {
		FileURI   string `json:"fileUri"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
	}

	rawBytes, err := json.Marshal(args[0])
	if err != nil {
		return ""
	}
	var ranges []blockRange
	if err := json.Unmarshal(rawBytes, &ranges); err != nil {
		return ""
	}

	// Group ranges by file URI to access each file's AST only once.
	type fileRange struct {
		startLine int
		endLine   int
	}
	byFile := make(map[string][]fileRange)
	// Preserve input order by tracking file URIs in order of first appearance.
	var fileOrder []string
	seen := make(map[string]bool)
	for _, r := range ranges {
		if !seen[r.FileURI] {
			seen[r.FileURI] = true
			fileOrder = append(fileOrder, r.FileURI)
		}
		byFile[r.FileURI] = append(byFile[r.FileURI], fileRange{r.StartLine, r.EndLine})
	}

	var parts []string
	for _, fileURI := range fileOrder {
		f := s.ws.Get(fileURI)
		if f == nil {
			continue
		}
		fFile := syntax.AsFile(syntax.Root(f.Green))
		contentLines := strings.Split(f.Content, "\n")

		for _, r := range byFile[fileURI] {
			part := reconstructDSLRangeFromView(fFile, f.LineIndex, contentLines, r.startLine, r.endLine)
			if part != "" {
				parts = append(parts, part)
			}
		}
	}

	return strings.Join(parts, "\n\n")
}

// reconstructDSLRangeFromView finds the typed-view declaration whose line range matches
// startLine:endLine and returns well-formed DSL. Falls back to raw line slicing for
// non-service/domain ranges. Returns empty string if no content is available.
func reconstructDSLRangeFromView(file syntax.File, li green.LineIndex, contentLines []string, startLine, endLine int) string {
	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		tokLine, _ := li.LineCol(nameTok.Offset())
		if tokLine == startLine && svc.EndLine(li) == endLine {
			return reconstructServiceFromView(svc)
		}
	}
	for _, dom := range file.Domains() {
		nameTok := dom.Name()
		if nameTok == nil {
			continue
		}
		tokLine, _ := li.LineCol(nameTok.Offset())
		if tokLine == startLine && dom.EndLine(li) == endLine {
			return reconstructDomainFromView(dom)
		}
	}
	// Fallback: raw line slice for actor blocks, arch blocks, and other top-level ranges.
	start := startLine - 1
	end := endLine
	if start < 0 {
		start = 0
	}
	if end > len(contentLines) {
		end = len(contentLines)
	}
	if start >= end {
		return ""
	}
	return strings.Join(contentLines[start:end], "\n")
}

// reconstructServiceFromView builds valid DSL for a single service declaration.
// Grouped services (from services { }) are wrapped; top-level services use the service keyword.
func reconstructServiceFromView(svc syntax.ServiceDecl) string {
	nameTok := svc.Name()
	if nameTok == nil {
		return ""
	}
	var sb strings.Builder
	indent := "  "
	fieldIndent := "    "
	if !svc.IsGrouped() {
		sb.WriteString("service ")
		sb.WriteString(nameTok.Text())
		sb.WriteString(" {\n")
		indent = ""
		fieldIndent = "  "
	} else {
		sb.WriteString("services {\n")
		sb.WriteString(indent)
		sb.WriteString(nameTok.Text())
		sb.WriteString(" {\n")
	}
	if ctxs := svc.Contexts(); len(ctxs) > 0 {
		sb.WriteString(fieldIndent + "contexts: " + strings.Join(ctxs, ", ") + "\n")
	}
	if ds := svc.DataStores(); len(ds) > 0 {
		sb.WriteString(fieldIndent + "data-stores: " + strings.Join(ds, ", ") + "\n")
	}
	if lang := svc.Language(); lang != "" {
		sb.WriteString(fieldIndent + "language: " + lang + "\n")
	}
	if dt := svc.DeploymentType(); dt != "" {
		sb.WriteString(fieldIndent + "deployment: " + dt)
		if rules := svc.DeploymentRules(); len(rules) > 0 {
			sb.WriteString("(")
			for i, r := range rules {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(r.Percentage + " -> " + r.Target)
			}
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	if svc.IsGrouped() {
		sb.WriteString(indent + "}\n}")
	} else {
		sb.WriteString("}")
	}
	return sb.String()
}

// reconstructDomainFromView builds valid DSL for a single domain declaration.
// Grouped domains (from domains { }) are wrapped; top-level domains use the domain keyword.
func reconstructDomainFromView(dom syntax.DomainDecl) string {
	nameTok := dom.Name()
	if nameTok == nil {
		return ""
	}
	var sb strings.Builder
	indent := "  "
	bcIndent := "    "
	if !dom.IsGrouped() {
		sb.WriteString("domain ")
		sb.WriteString(nameTok.Text())
		sb.WriteString(" {\n")
		indent = ""
		bcIndent = "  "
	} else {
		sb.WriteString("domains {\n")
		sb.WriteString(indent)
		sb.WriteString(nameTok.Text())
		sb.WriteString(" {\n")
	}
	for _, bc := range dom.BoundedContexts() {
		bcTok := bc.Name()
		if bcTok == nil {
			continue
		}
		sb.WriteString(bcIndent + bcTok.Text() + "\n")
	}
	if dom.IsGrouped() {
		sb.WriteString(indent + "}\n}")
	} else {
		sb.WriteString("}")
	}
	return sb.String()
}

type craftExtractionResult struct {
	Services    []craftServiceEntry `json:"services,omitempty"`
	Domains     []craftDomainEntry  `json:"domains,omitempty"`
	UseCases    []craftUseCaseEntry `json:"useCases,omitempty"`
	Actors      []craftActorEntry   `json:"actors,omitempty"`
	ActorBlocks []craftBlockRef     `json:"actorBlocks,omitempty"`
	Archs       []craftArchEntry    `json:"archs,omitempty"`
}

type craftBlockRef struct {
	FileURI   string `json:"fileUri"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

type craftServiceEntry struct {
	Name          string   `json:"name"`
	URI           string   `json:"uri"`
	StartLine     int      `json:"startLine"`
	EndLine       int      `json:"endLine"`
	Contexts      []string `json:"contexts,omitempty"`
	InCurrentFile bool     `json:"inCurrentFile,omitempty"`
}

type craftDomainEntry struct {
	Name            string         `json:"name"`
	URI             string         `json:"uri"`
	StartLine       int            `json:"startLine"`
	EndLine         int            `json:"endLine"`
	BoundedContexts []craftBCEntry `json:"boundedContexts,omitempty"`
	InCurrentFile   bool           `json:"inCurrentFile,omitempty"`
}

type craftBCEntry struct {
	Name      string `json:"name"`
	StartLine int    `json:"startLine"`
}

type craftUseCaseEntry struct {
	Name              string   `json:"name"`
	URI               string   `json:"uri"`
	StartLine         int      `json:"startLine"`
	EndLine           int      `json:"endLine"`
	InCurrentFile     bool     `json:"inCurrentFile,omitempty"`
	EntryPointContext string   `json:"entryPointContext,omitempty"`
	InvolvedContexts  []string `json:"involvedContexts,omitempty"`
}

type craftActorEntry struct {
	Name          string `json:"name"`
	ActorType     string `json:"type"`
	URI           string `json:"uri"`
	StartLine     int    `json:"startLine"`
	EndLine       int    `json:"endLine"`
	InCurrentFile bool   `json:"inCurrentFile,omitempty"`
}

type craftArchEntry struct {
	Name          string `json:"name"`
	URI           string `json:"uri"`
	StartLine     int    `json:"startLine"`
	EndLine       int    `json:"endLine"`
	InCurrentFile bool   `json:"inCurrentFile,omitempty"`
}

// FoldingRanges returns folding ranges for the document.
// S7: one fold per arch block, plus one per labelled segment (presentation/gateway)
// inside each arch block. LSP line numbers are 0-based.
func (s *Server) FoldingRanges(_ context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil {
		return nil, nil
	}
	file := syntax.AsFile(syntax.Root(f.Green))

	var ranges []protocol.FoldingRange

	li := f.LineIndex
	for _, arch := range file.Archs() {
		archStart := arch.Line(li)
		archEnd := arch.EndLine(li)
		if archStart <= 0 || archEnd <= archStart {
			continue
		}
		// One fold for the entire arch block.
		startLine := uint32(archStart - 1)
		endLine := uint32(archEnd - 1)
		ranges = append(ranges, protocol.FoldingRange{
			StartLine: startLine,
			EndLine:   endLine,
			Kind:      protocol.RegionFoldingRange,
		})

		// One fold per labelled section inside arch.
		// presentation: folds from its label line to the line before gateway: (or arch end).
		if pl := arch.PresentationLine(li); pl > 0 {
			sectionStart := uint32(pl - 1)
			sectionEnd := endLine - 1 // default: end just before closing `}`
			if gl := arch.GatewayLine(li); gl > 0 && gl > pl {
				sectionEnd = uint32(gl - 2) // end line before gateway label
			}
			if sectionEnd > sectionStart {
				ranges = append(ranges, protocol.FoldingRange{
					StartLine: sectionStart,
					EndLine:   sectionEnd,
					Kind:      protocol.RegionFoldingRange,
				})
			}
		}
		if gl := arch.GatewayLine(li); gl > 0 {
			sectionStart := uint32(gl - 1)
			sectionEnd := endLine - 1
			if sectionEnd > sectionStart {
				ranges = append(ranges, protocol.FoldingRange{
					StartLine: sectionStart,
					EndLine:   sectionEnd,
					Kind:      protocol.RegionFoldingRange,
				})
			}
		}
	}

	return ranges, nil
}


func (s *Server) Formatting(_ context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil {
		return nil, nil
	}
	formatted := FormatDocument(f.Content)
	if formatted == f.Content {
		return nil, nil
	}
	lines := strings.Split(f.Content, "\n")
	lastLine := uint32(len(lines) - 1)
	lastChar := uint32(len(lines[len(lines)-1]))
	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: lastLine, Character: lastChar},
			},
			NewText: formatted,
		},
	}, nil
}

func (s *Server) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil {
		return nil, nil
	}
	file := syntax.AsFile(syntax.Root(f.Green))

	cursorLine := int(params.Position.Line) + 1 // convert to 1-based

	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil {
			continue
		}
		nameLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		if nameLine == 0 {
			// Individual `actor` statements don't have a line recorded in AST;
			// hover will not resolve them until full position tracking lands.
			continue
		}
		if nameLine == cursorLine {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: "actor: " + nameTok.Text() + " (" + a.ActorTypeValue() + ")",
				},
			}, nil
		}
	}

	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		nameLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		if nameLine == 0 {
			continue
		}
		if nameLine == cursorLine {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: "domain: " + nameTok.Text(),
				},
			}, nil
		}
	}

	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		nameLine, _ := f.LineIndex.LineCol(nameTok.Offset())
		if nameLine == 0 {
			continue
		}
		if nameLine == cursorLine {
			detail := "service: " + nameTok.Text()
			if lang := svc.Language(); lang != "" {
				detail += " (" + lang + ")"
			}
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: detail,
				},
			}, nil
		}
	}

	for _, exp := range file.Exposures() {
		expLine := exp.Line(f.LineIndex)
		if expLine == 0 || expLine != cursorLine {
			continue
		}
		nameTok := exp.Name()
		name := ""
		if nameTok != nil {
			name = nameTok.Text()
		}
		detail := "exposure: " + name
		if to := exp.To(); len(to) > 0 {
			detail += " → " + strings.Join(to, ", ")
		}
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: detail},
		}, nil
	}

	// Walk use-case bodies: hover on an action line shows the resolved declaration.
	rm := s.ws.ResolutionMap()
	uri := string(params.TextDocument.URI)
	for _, uc := range file.UseCases() {
		kwTok := uc.Keyword()
		ucLine := 0
		if kwTok != nil {
			ucLine, _ = f.LineIndex.LineCol(kwTok.Offset())
		}
		for _, sc := range uc.Scenarios() {
			// Hover on the use_case name itself (line of use_case keyword).
			if ucLine == cursorLine {
				name := ""
				if titleTok := uc.Title(); titleTok != nil {
					name = titleTok.Text()
				}
				return &protocol.Hover{
					Contents: protocol.MarkupContent{
						Kind:  protocol.PlainText,
						Value: "use_case: " + name,
					},
				}, nil
			}
			// Hover on trigger line.
			trigger := sc.Trigger()
			whenTok := sc.When()
			triggerLine := 0
			if whenTok != nil {
				triggerLine, _ = f.LineIndex.LineCol(whenTok.Offset())
			}
			if triggerLine == cursorLine {
				name := trigger.ActorName()
				if name == "" {
					name = trigger.ContextName()
				}
				if name != "" {
					if val := hoverForUseCaseRef(rm, uri, name, triggerLine); val != "" {
						return &protocol.Hover{
							Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: val},
						}, nil
					}
				}
			}
			// Hover on action lines.
			for _, action := range sc.Actions() {
				actionLine := action.Line(f.LineIndex)
				if actionLine != cursorLine {
					continue
				}
				// Show hover for the "from" domain first.
				if val := hoverForUseCaseRef(rm, uri, action.SubjectName(), actionLine); val != "" {
					return &protocol.Hover{
						Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: val},
					}, nil
				}
			}
		}
	}

	return nil, nil
}

// hoverForUseCaseRef resolves a use-case body reference and returns a hover string.
func hoverForUseCaseRef(rm sema.ResolutionMap, uri, name string, line int) string {
	if name == "" {
		return ""
	}
	target, ok := sema.ResolveUseCaseRef(rm, uri, name, line)
	if !ok {
		return ""
	}
	switch target.Kind {
	case "actor":
		return "actor: " + target.Actor.Name + " (" + string(target.Actor.Type) + ")"
	case "domain":
		return "domain: " + target.Domain.Name
	case "bounded_context":
		return "bounded context: " + name + " (domain: " + target.Domain.Name + ")"
	case "service":
		detail := "service: " + target.Service.Name
		if target.Service.Language != "" {
			detail += " (" + target.Service.Language + ")"
		}
		return detail
	}
	return ""
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

// Semantic token type constants matching the extension's craft-* custom types.
// Their index in the legend slice is their numeric encoding in the data stream.
// craft-actor-definition: actor name at the declaration site.
// craft-actor-name: actor referenced inside a use_case body.
// craft-domain-name: domain declaration and domain refs in use_case.
// craft-context-name: bounded context declaration and BC refs in use_case.
// craft-service-name: service declaration and service refs in use_case.
const (
	semanticTokenTypeActorDecl   protocol.SemanticTokenTypes = "craft-actor-definition" // index 0
	semanticTokenTypeActorRef    protocol.SemanticTokenTypes = "craft-actor-name"        // index 1
	semanticTokenTypeDomainName  protocol.SemanticTokenTypes = "craft-domain-name"       // index 2
	semanticTokenTypeContextName protocol.SemanticTokenTypes = "craft-context-name"      // index 3
	semanticTokenTypeServiceName protocol.SemanticTokenTypes = "craft-service-name"      // index 4
)

// semanticTokenLegend is the ordered list of all craft-* token types.
// The index of each entry is the numeric value emitted in the encoded token stream.
// Entries 0-4 are the entity-name types used by Pass 2 (AST ident classification).
// Entries 5+ are the keyword/punctuation types used by Pass 1 (syntax tree walk).
var semanticTokenLegend = []string{
	"craft-actor-definition",    // 0
	"craft-actor-name",          // 1
	"craft-domain-name",         // 2
	"craft-context-name",        // 3
	"craft-service-name",        // 4
	"craft-component-name",      // 5
	"craft-exposure-name",       // 6
	"craft-data-store-name",     // 7
	"craft-flow-keyword",        // 8
	"craft-services-keyword",    // 9
	"craft-arch-keyword",        // 10
	"craft-exposure-keyword",    // 11
	"craft-domain-keyword",      // 12
	"craft-actor-keyword",       // 13
	"craft-actors-keyword",      // 14
	"craft-contexts-property",   // 15
	"craft-language-property",   // 16
	"craft-data-stores-property", // 17
	"craft-deployment-property", // 18
	"craft-to-property",         // 19
	"craft-through-property",    // 20
	"craft-presentation-section", // 21
	"craft-gateway-section",     // 22
	"craft-asks-verb",           // 23
	"craft-notifies-verb",       // 24
	"craft-listens-verb",        // 25
	"craft-returns-verb",        // 26
	"craft-regular-verb",        // 27
	"craft-actor-type",          // 28
	"craft-connector-word",      // 29
	"craft-usecase-string",      // 30
	"craft-event-string",        // 31
	"craft-regular-string",      // 32
	"craft-phrase-word",         // 33
	"craft-comment",             // 34
	"craft-boolean",             // 35
	"craft-comma",               // 36
	"craft-percentage",          // 37
	"craft-parenthesis",         // 38
	"craft-deployment-arrow",    // 39
	"craft-braces",              // 40
	"craft-deployment-type",     // 41
	"craft-deployment-target",   // 42
	"craft-domain-list",         // 43
	"craft-language-value",      // 44
}

// semanticLegendIndex maps token type name → legend index for O(1) lookup.
var semanticLegendIndex = func() map[string]int {
	m := make(map[string]int, len(semanticTokenLegend))
	for i, name := range semanticTokenLegend {
		m[name] = i
	}
	return m
}()

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
			TokenTypes:     semanticTokenLegend,
			TokenModifiers: []string{string(protocol.SemanticTokenModifierDeclaration)},
		},
		Full: true,
	}
}

// semanticTokenTypeIndex returns the legend index for a token type name.
// Returns -1 if the type is not in the legend.
func (s *Server) semanticTokenTypeIndex(name string) int {
	idx, ok := semanticLegendIndex[name]
	if !ok {
		return -1
	}
	return idx
}

// semanticTokenTypeIndexConst returns the legend index for a protocol.SemanticTokenTypes constant.
// Used by Pass 2 (entity name classification).
func semanticTokenTypeIndexConst(t protocol.SemanticTokenTypes) uint32 {
	idx, ok := semanticLegendIndex[string(t)]
	if !ok {
		return 0
	}
	return uint32(idx)
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

// syntaxKindToCraftToken maps SyntaxKind values for non-ident tokens to their
// craft-* semantic token type names. SyntaxKindIdent is intentionally absent —
// ident tokens are classified by the AST-walk Pass 2 (semanticIdentTokens).
var syntaxKindToCraftToken = map[syntax.SyntaxKind]string{
	syntax.SyntaxKindKwUseCase:      "craft-flow-keyword",
	syntax.SyntaxKindKwWhen:         "craft-flow-keyword",
	syntax.SyntaxKindKwActor:        "craft-actor-keyword",
	syntax.SyntaxKindKwActors:       "craft-actors-keyword",
	syntax.SyntaxKindKwDomain:       "craft-domain-keyword",
	syntax.SyntaxKindKwDomains:      "craft-domain-keyword",
	syntax.SyntaxKindKwServices:     "craft-services-keyword",
	syntax.SyntaxKindKwArch:         "craft-arch-keyword",
	syntax.SyntaxKindKwExposure:     "craft-exposure-keyword",
	syntax.SyntaxKindKwUser:         "craft-actor-type",
	syntax.SyntaxKindKwSystem:       "craft-actor-type",
	syntax.SyntaxKindKwService:      "craft-actor-type",
	syntax.SyntaxKindKwAsks:         "craft-asks-verb",
	syntax.SyntaxKindKwNotifies:     "craft-notifies-verb",
	syntax.SyntaxKindKwListens:      "craft-listens-verb",
	syntax.SyntaxKindKwReturns:      "craft-returns-verb",
	syntax.SyntaxKindKwTo:           "craft-connector-word",
	syntax.SyntaxKindKwThrough:      "craft-through-property",
	syntax.SyntaxKindKwContexts:     "craft-contexts-property",
	syntax.SyntaxKindKwLanguage:     "craft-language-property",
	syntax.SyntaxKindKwDataStores:   "craft-data-stores-property",
	syntax.SyntaxKindKwDeployment:   "craft-deployment-property",
	syntax.SyntaxKindKwPresentation: "craft-presentation-section",
	syntax.SyntaxKindKwGateway:      "craft-gateway-section",
	syntax.SyntaxKindKwTrue:         "craft-boolean",
	syntax.SyntaxKindKwFalse:        "craft-boolean",
	syntax.SyntaxKindString:         "craft-regular-string",
	syntax.SyntaxKindLineComment:    "craft-comment",
	syntax.SyntaxKindBlockComment:   "craft-comment",
	syntax.SyntaxKindLBrace:         "craft-braces",
	syntax.SyntaxKindRBrace:         "craft-braces",
	syntax.SyntaxKindLBracket:       "craft-braces",
	syntax.SyntaxKindRBracket:       "craft-braces",
	syntax.SyntaxKindGT:             "craft-braces",
	syntax.SyntaxKindLParen:         "craft-parenthesis",
	syntax.SyntaxKindRParen:         "craft-parenthesis",
	syntax.SyntaxKindComma:          "craft-comma",
	syntax.SyntaxKindArrow:          "craft-deployment-arrow",
	syntax.SyntaxKindPercentage:     "craft-percentage",
}

func (s *Server) SemanticTokensFull(_ context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	if params == nil {
		return nil, nil
	}
	f := s.ws.Get(string(params.TextDocument.URI))
	if f == nil {
		return &protocol.SemanticTokens{Data: []uint32{}}, nil
	}

	var tokens []semanticToken

	// Pass 1: walk all tokens in the syntax tree, emit craft-* types for non-ident tokens.
	if f.Green != nil {
		root := syntax.Root(f.Green)
		for _, tok := range root.AllTokens() {
			craftType, ok := syntaxKindToCraftToken[tok.Kind()]
			if !ok {
				continue // SyntaxKindIdent and others handled in Pass 2
			}
			idx := s.semanticTokenTypeIndex(craftType)
			if idx < 0 {
				continue
			}
			length := uint32(len(tok.Text()))
			if tok.Kind() == syntax.SyntaxKindString {
				// Text() stores content without quotes; add 2 to cover "content".
				length += 2
			}
			pos := lspPosFromOffset(f.LineIndex, f.Content, tok.Offset())
			tokens = append(tokens, semanticToken{
				line:      pos.Line,
				startChar: pos.Character,
				length:    length,
				tokenType: uint32(idx),
			})
		}
	}

	// Pass 2: entity name classification using AST walk.
	// Classifies SyntaxKindIdent tokens as actor names, domain names, etc.
	tokens = append(tokens, s.semanticIdentTokens(f, string(params.TextDocument.URI))...)

	// Pass 3: reclassify string tokens that carry semantic meaning (use-case titles,
	// event strings) by overriding the tokenType set in Pass 1.
	s.applyStringTypeOverrides(tokens, f)

	// Sort tokens by line, then character (required for relative encoding).
	sortSemanticTokens(tokens)

	return &protocol.SemanticTokens{
		Data: encodeSemanticTokens(tokens),
	}, nil
}

// semanticIdentTokens classifies identifier tokens by walking the AST and the
// use-case resolution map. This is the existing Pass 2 logic extracted into a helper.
func (s *Server) semanticIdentTokens(f *workspace.File, uri string) []semanticToken {
	var tokens []semanticToken
	file := syntax.AsFile(syntax.Root(f.Green))
	li := f.LineIndex
	src := f.Content

	// tokPos returns the LSP line/char for a *SyntaxToken (pointer variant).
	tokPos := func(tok *syntax.SyntaxToken) (line, startChar uint32, valid bool) {
		l, _ := li.LineCol(tok.Offset())
		if l <= 0 {
			return 0, 0, false
		}
		return lspLine(l), li.UTF16Col(src, tok.Offset()), true
	}

	// tokPosVal returns the LSP line/char for a SyntaxToken value.
	tokPosVal := func(tok syntax.SyntaxToken) (line, startChar uint32, valid bool) {
		l, _ := li.LineCol(tok.Offset())
		if l <= 0 {
			return 0, 0, false
		}
		return lspLine(l), li.UTF16Col(src, tok.Offset()), true
	}

	// Actors: craft-actor-definition, modifier 1 (declaration).
	for _, a := range file.Actors() {
		nameTok := a.Name()
		if nameTok == nil {
			continue // individual `actor` stmts without position — skip for now
		}
		line, col, ok := tokPos(nameTok)
		if !ok {
			continue
		}
		tokens = append(tokens, semanticToken{
			line:      line,
			startChar: col,
			length:    uint32(len(nameTok.Text())),
			tokenType: semanticTokenTypeIndexConst(semanticTokenTypeActorDecl),
			modifiers: 1, // declaration
		})
	}

	// Domains: craft-domain-name declaration; bounded contexts: craft-context-name declaration.
	for _, d := range file.Domains() {
		nameTok := d.Name()
		if nameTok == nil {
			continue
		}
		line, col, ok := tokPos(nameTok)
		if !ok {
			continue
		}
		tokens = append(tokens, semanticToken{
			line:      line,
			startChar: col,
			length:    uint32(len(nameTok.Text())),
			tokenType: semanticTokenTypeIndexConst(semanticTokenTypeDomainName),
			modifiers: 1, // declaration
		})
		// Each bounded context name inside the domain body is a declaration.
		for _, bc := range d.BoundedContexts() {
			bcTok := bc.Name()
			if bcTok == nil {
				continue
			}
			bcLine, bcCol, bcOk := tokPos(bcTok)
			if !bcOk {
				continue
			}
			tokens = append(tokens, semanticToken{
				line:      bcLine,
				startChar: bcCol,
				length:    uint32(len(bcTok.Text())),
				tokenType: semanticTokenTypeIndexConst(semanticTokenTypeContextName),
				modifiers: 1, // declaration
			})
		}
	}

	// Services: craft-service-name, modifier 1 (declaration).
	for _, svc := range file.Services() {
		nameTok := svc.Name()
		if nameTok == nil {
			continue
		}
		line, col, ok := tokPos(nameTok)
		if !ok {
			continue
		}
		tokens = append(tokens, semanticToken{
			line:      line,
			startChar: col,
			length:    uint32(len(nameTok.Text())),
			tokenType: semanticTokenTypeIndexConst(semanticTokenTypeServiceName),
			modifiers: 1, // declaration
		})
	}

	// Service body items: data stores, language value, deployment type/target.
	svcDsIdx := s.semanticTokenTypeIndex("craft-data-store-name")
	svcLangIdx := s.semanticTokenTypeIndex("craft-language-value")
	svcDtIdx := s.semanticTokenTypeIndex("craft-deployment-type")
	svcTargIdx := s.semanticTokenTypeIndex("craft-deployment-target")
	for _, svc := range file.Services() {
		if svcDsIdx >= 0 {
			for _, tok := range svc.DataStoreTokens() {
				tokLine, tokCol, tokOk := tokPosVal(tok)
				if !tokOk {
					continue
				}
				tokens = append(tokens, semanticToken{
					line:      tokLine,
					startChar: tokCol,
					length:    uint32(len(tok.Text())),
					tokenType: uint32(svcDsIdx),
				})
			}
		}

		if svcLangIdx >= 0 {
			if tok := svc.LanguageToken(); tok != nil {
				tokLine, tokCol, tokOk := tokPos(tok)
				if tokOk {
					tokens = append(tokens, semanticToken{
						line:      tokLine,
						startChar: tokCol,
						length:    uint32(len(tok.Text())),
						tokenType: uint32(svcLangIdx),
					})
				}
			}
		}

		if svcDtIdx >= 0 {
			if tok := svc.DeploymentTypeToken(); tok != nil {
				tokLine, tokCol, tokOk := tokPos(tok)
				if tokOk {
					tokens = append(tokens, semanticToken{
						line:      tokLine,
						startChar: tokCol,
						length:    uint32(len(tok.Text())),
						tokenType: uint32(svcDtIdx),
					})
				}
			}
		}

		if svcTargIdx >= 0 {
			for _, tok := range svc.DeploymentTargetTokens() {
				tokLine, tokCol, tokOk := tokPosVal(tok)
				if !tokOk {
					continue
				}
				tokens = append(tokens, semanticToken{
					line:      tokLine,
					startChar: tokCol,
					length:    uint32(len(tok.Text())),
					tokenType: uint32(svcTargIdx),
				})
			}
		}
	}

	// Exposures: craft-exposure-name.
	expIdx := s.semanticTokenTypeIndex("craft-exposure-name")
	if expIdx >= 0 {
		for _, e := range file.Exposures() {
			nameTok := e.Name()
			if nameTok == nil {
				continue
			}
			line, col, ok := tokPos(nameTok)
			if !ok {
				continue
			}
			tokens = append(tokens, semanticToken{
				line:      line,
				startChar: col,
				length:    uint32(len(nameTok.Text())),
				tokenType: uint32(expIdx),
			})
		}
	}

	// Arch component names: craft-component-name (primary name per component).
	compIdx := s.semanticTokenTypeIndex("craft-component-name")
	if compIdx >= 0 {
		for _, arch := range file.Archs() {
			for _, section := range arch.Sections() {
				for _, comp := range section.Components() {
					nameTok := comp.Name()
					if nameTok == nil {
						continue
					}
					line, col, ok := tokPos(nameTok)
					if !ok {
						continue
					}
					tokens = append(tokens, semanticToken{
						line:      line,
						startChar: col,
						length:    uint32(len(nameTok.Text())),
						tokenType: uint32(compIdx),
					})
				}
			}
		}
	}

	// Open-taxonomy actor types: craft-actor-type (ident types only; keyword types handled by Pass 1).
	actorTypeIdx := s.semanticTokenTypeIndex("craft-actor-type")
	if actorTypeIdx >= 0 {
		for _, a := range file.Actors() {
			typTok := a.ActorTypeToken()
			if typTok == nil {
				continue
			}
			line, col, ok := tokPos(typTok)
			if !ok {
				continue
			}
			tokens = append(tokens, semanticToken{
				line:      line,
				startChar: col,
				length:    uint32(len(typTok.Text())),
				tokenType: uint32(actorTypeIdx),
			})
		}
	}

	// Use-case action parties: colour actors/domains/services referenced inside
	// when clauses using their respective semantic token types (S6).
	rm := s.ws.ResolutionMap()
	for _, uc := range file.UseCases() {
		for _, sc := range uc.Scenarios() {
			// Trigger subject.
			trigger := sc.Trigger()
			whenTok := sc.When()
			triggerLine := 0
			if whenTok != nil {
				triggerLine, _ = li.LineCol(whenTok.Offset())
			}
			triggerName := trigger.ActorName()
			if triggerName == "" {
				triggerName = trigger.ContextName()
			}
			if triggerName != "" && triggerLine > 0 {
				if tt, ok := useCaseRefTokenType(rm, uri, triggerName, triggerLine); ok {
					col := uint32(0)
					if subj := trigger.Subject(); subj != nil {
						col = li.UTF16Col(src, subj.Offset())
					}
					tokens = append(tokens, semanticToken{
						line:      lspLine(triggerLine),
						startChar: col,
						length:    uint32(len(triggerName)),
						tokenType: tt,
					})
				}
			}
			// Trigger phrase: emit craft-phrase-word for idents after the subject
			// that TextMate would otherwise color incorrectly (uppercase idents →
			// entity.name.type, connector words like "to" → keyword.operator).
			phraseWordIdx := s.semanticTokenTypeIndex("craft-phrase-word")
			if phraseWordIdx >= 0 && trigger.Kind() == "external" {
				subjectSkipped := false
				for _, tok := range trigger.Tokens() {
					if tok.Kind() != syntax.SyntaxKindIdent {
						continue
					}
					tokLine, _, tokOk := tokPosVal(tok)
					if !tokOk {
						continue
					}
					if !subjectSkipped {
						subjectSkipped = true
						continue // subject already handled above
					}
					val := tok.Text()
					if len(val) == 0 {
						continue
					}
					// Only override tokens that TextMate would color: uppercase idents
					// (entity-name pattern) or connector words (connector-keywords pattern).
					if !unicode.IsUpper([]rune(val)[0]) && !isConnectorWord(val) {
						continue
					}
					tokCol := li.UTF16Col(src, tok.Offset())
					tokens = append(tokens, semanticToken{
						line:      tokLine,
						startChar: tokCol,
						length:    uint32(len(val)),
						tokenType: uint32(phraseWordIdx),
					})
				}
			}

			// Action domain and targetDomain.
			for _, action := range sc.Actions() {
				actionLine := action.Line(li)
				if actionLine <= 0 {
					continue
				}
				if subj := action.SubjectName(); subj != "" {
					if tt, ok := useCaseRefTokenType(rm, uri, subj, actionLine); ok {
						subjCol := action.SubjectCol(li)
						col := uint32(0)
						if subjCol > 0 {
							col = uint32(subjCol - 1)
						}
						tokens = append(tokens, semanticToken{
							line:      uint32(actionLine - 1),
							startChar: col,
							length:    uint32(len(subj)),
							tokenType: tt,
						})
					}
				}
				if target := action.TargetName(); target != "" {
					if tt, ok := useCaseRefTokenType(rm, uri, target, actionLine); ok {
						targCol := action.TargetCol(li)
						col := uint32(0)
						if targCol > 0 {
							col = uint32(targCol - 1)
						}
						tokens = append(tokens, semanticToken{
							line:      uint32(actionLine - 1),
							startChar: col,
							length:    uint32(len(target)),
							tokenType: tt,
						})
					}
				}
				tokens = append(tokens, s.classifyActionIdents(action, li, src)...)
			}
		}
	}

	return tokens
}

// isConnectorWord reports whether v is a grammatical connector/preposition in
// use-case action bodies. Must stay in sync with parser.go (package-private copy).
func isConnectorWord(v string) bool {
	switch v {
	case "a", "an", "the", "as", "to", "from", "in", "on", "at", "for", "with", "by":
		return true
	}
	return false
}

// classifyActionIdents emits semantic tokens for the verb, connector, and phrase
// words inside a single action. Subjects and targets are already classified by
// the resolution-map loop; this covers the remaining ident tokens.
func (s *Server) classifyActionIdents(action syntax.ActionDecl, li green.LineIndex, src string) []semanticToken {
	kind := action.Kind()
	actionLine := action.Line(li)
	if actionLine <= 0 {
		return nil
	}

	verbIdx := s.semanticTokenTypeIndex("craft-regular-verb")
	connIdx := s.semanticTokenTypeIndex("craft-connector-word")
	phraseIdx := s.semanticTokenTypeIndex("craft-phrase-word")

	toks := action.Tokens()
	var result []semanticToken

	emit := func(tok syntax.SyntaxToken, typeIdx int) {
		if typeIdx < 0 {
			return
		}
		l, _ := li.LineCol(tok.Offset())
		if l <= 0 {
			return
		}
		result = append(result, semanticToken{
			line:      lspLine(l),
			startChar: li.UTF16Col(src, tok.Offset()),
			length:    uint32(len(tok.Text())),
			tokenType: uint32(typeIdx),
		})
	}

	// tokLineNum returns the 1-based line for a token value (0 if invalid).
	tokLineNum := func(tok syntax.SyntaxToken) int {
		l, _ := li.LineCol(tok.Offset())
		return l
	}

	switch kind {
	case "internal_action":
		// toks: [subject, verb, (connector?), phrase...]
		if len(toks) < 2 {
			return nil
		}
		emit(toks[1], verbIdx) // verb
		start := 2
		// connector is always SyntaxKindIdent for internal_action (parser guarantee)
		if start < len(toks) && isConnectorWord(toks[start].Text()) && tokLineNum(toks[start]) == actionLine {
			emit(toks[start], connIdx)
			start++
		}
		for _, tok := range toks[start:] {
			if tok.Kind() != syntax.SyntaxKindIdent || tokLineNum(tok) != actionLine {
				continue
			}
			emit(tok, phraseIdx)
		}

	case "sync_action":
		// toks: [subject, asks, target, (connector?), phrase...]
		// subject and target classified by resolution map; skip them.
		start := 3
		if start < len(toks) && tokLineNum(toks[start]) == actionLine {
			if toks[start].Kind() == syntax.SyntaxKindKwTo {
				// Pass 1 already emits SyntaxKindKwTo as craft-connector-word; just advance.
				start++
			} else if isConnectorWord(toks[start].Text()) {
				emit(toks[start], connIdx)
				start++
			}
		}
		for _, tok := range toks[start:] {
			if tok.Kind() != syntax.SyntaxKindIdent || tokLineNum(tok) != actionLine {
				continue
			}
			emit(tok, phraseIdx)
		}

	case "return_action":
		// toks: [subject, returns, (to, target)?, (connector?), phrase...]
		start := 2
		if start < len(toks) && toks[start].Kind() == syntax.SyntaxKindKwTo {
			start += 2 // skip "to target"
		}
		// connector is always SyntaxKindIdent for return_action (parser guarantee)
		if start < len(toks) && isConnectorWord(toks[start].Text()) && tokLineNum(toks[start]) == actionLine {
			emit(toks[start], connIdx)
			start++
		}
		for _, tok := range toks[start:] {
			if tok.Kind() != syntax.SyntaxKindIdent || tokLineNum(tok) != actionLine {
				continue
			}
			emit(tok, phraseIdx)
		}
	}

	return result
}

// useCaseRefTokenType resolves a use-case body reference name to its semantic
// token type. Returns the token type index and true if the name is resolved.
func useCaseRefTokenType(rm sema.ResolutionMap, uri, name string, line int) (uint32, bool) {
	target, ok := sema.ResolveUseCaseRef(rm, uri, name, line)
	if !ok {
		return 0, false
	}
	switch target.Kind {
	case "actor":
		return semanticTokenTypeIndexConst(semanticTokenTypeActorRef), true
	case "domain":
		return semanticTokenTypeIndexConst(semanticTokenTypeDomainName), true
	case "bounded_context":
		return semanticTokenTypeIndexConst(semanticTokenTypeContextName), true
	case "service":
		return semanticTokenTypeIndexConst(semanticTokenTypeServiceName), true
	}
	return 0, false
}

// sortSemanticTokens sorts tokens ascending by (line, startChar).
func sortSemanticTokens(tokens []semanticToken) {
	sort.Slice(tokens, func(i, j int) bool {
		a, b := tokens[i], tokens[j]
		if a.line != b.line {
			return a.line < b.line
		}
		return a.startChar < b.startChar
	})
}

// applyStringTypeOverrides reclassifies string tokens that carry semantic meaning:
// use-case title strings → craft-usecase-string (index 30)
// event strings in async actions and event/domain_listen triggers → craft-event-string (index 31)
// All other SyntaxKindString tokens remain craft-regular-string (index 32) from Pass 1.
func (s *Server) applyStringTypeOverrides(tokens []semanticToken, f *workspace.File) {
	if f.Green == nil {
		return
	}
	type posKey struct{ line, startChar uint32 }
	overrides := make(map[posKey]uint32)

	ucIdx := s.semanticTokenTypeIndex("craft-usecase-string")
	evIdx := s.semanticTokenTypeIndex("craft-event-string")
	if ucIdx < 0 || evIdx < 0 {
		return
	}
	ucStringIdx := uint32(ucIdx)
	evStringIdx := uint32(evIdx)

	file := syntax.AsFile(syntax.Root(f.Green))
	li := f.LineIndex
	src := f.Content

	for _, uc := range file.UseCases() {
		if title := uc.Title(); title != nil {
			l, _ := li.LineCol(title.Offset())
			if l > 0 {
				pos := lspPosFromOffset(li, src, title.Offset())
				overrides[posKey{pos.Line, pos.Character}] = ucStringIdx
			}
		}
		for _, sc := range uc.Scenarios() {
			trigger := sc.Trigger()
			if trigger.Kind() == "event" || trigger.Kind() == "domain_listen" {
				if evTok := trigger.Event(); evTok != nil && trigger.EventIsString() {
					pos := lspPosFromOffset(li, src, evTok.Offset())
					overrides[posKey{pos.Line, pos.Character}] = evStringIdx
				}
			}
			for _, action := range sc.Actions() {
				if action.Kind() == "async_action" && action.EventIsString() {
					actionLine := action.Line(li)
					if actionLine <= 0 {
						continue
					}
					eventCol := action.EventCol(li)
					overrides[posKey{uint32(actionLine - 1), uint32(eventCol - 1)}] = evStringIdx
				}
			}
		}
	}

	for i := range tokens {
		key := posKey{tokens[i].line, tokens[i].startChar}
		if newType, ok := overrides[key]; ok {
			tokens[i].tokenType = newType
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

// publishDiagnostics pushes textDocument/publishDiagnostics to the client.
// Diagnostics (parse + sema) are already stored on the workspace File since S5.
// We also include any workspace-level cross-file diagnostics for this URI.
func (s *Server) publishDiagnostics(ctx context.Context, uri string) {
	f := s.ws.Get(uri)
	if f == nil {
		return
	}

	// f.Diagnostics contains parse + per-file sema diagnostics (set during
	// workspace.parseAndStore). Append workspace-level cross-file diagnostics
	// (e.g. unresolved-reference from AnalyzeWorkspace) for this URI.
	allDiags := append(f.Diagnostics, s.ws.WorkspaceDiagnostics(uri)...)

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
