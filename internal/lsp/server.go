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

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/tcarcao/craft/internal/ast"
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

	mu         sync.Mutex
	traceLevel protocol.TraceValue

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
			FoldingRangeProvider:   true,
			// SemanticTokensProvider is interface{} in this protocol version;
			// use an inline struct so it serialises with Legend + Full fields.
			SemanticTokensProvider: semanticTokensOptions(),
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

func (s *Server) Completion(_ context.Context, _ *protocol.CompletionParams) (*protocol.CompletionList, error) {
	return nil, nil
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
	if f == nil || f.AST == nil {
		return nil, nil
	}

	cursorLine := int(params.Position.Line) + 1 // convert to 1-based
	rm := s.ws.ResolutionMap()

	// Walk services: match cursor against each context reference line so that
	// go-to-definition fires when the cursor is on a `contexts:` entry.
	for _, svc := range f.AST.Services {
		for i, ctxName := range svc.Contexts {
			// Match by context token line when available; fall back to the
			// service name line for services parsed without line tracking
			// (e.g. from the ANTLR adapter path).
			ctxLine := svc.Line
			if i < len(svc.ContextLines) {
				ctxLine = svc.ContextLines[i]
			}
			if ctxLine != cursorLine {
				continue
			}
			domSym, ok := sema.ResolveServiceContext(rm, uri, svc.Name, ctxName)
			if !ok {
				continue
			}
			return []protocol.Location{{
				URI: protocol.DocumentURI(domSym.URI),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(domSym.Line - 1)},
					End:   protocol.Position{Line: uint32(domSym.Line - 1), Character: uint32(len(domSym.Name))},
				},
			}}, nil
		}
	}

	// Walk use-case action parties: cursor on an action line resolves to the
	// actor/domain/service declaration.
	for _, uc := range f.AST.UseCases {
		for _, sc := range uc.Scenarios {
			// Check trigger subject line.
			if sc.Trigger.Line == cursorLine {
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, sc.Trigger.Actor, sc.Trigger.Line); ok {
					return []protocol.Location{loc}, nil
				}
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, sc.Trigger.Context, sc.Trigger.Line); ok {
					return []protocol.Location{loc}, nil
				}
			}
			for _, action := range sc.Actions {
				if action.Line != cursorLine {
					continue
				}
				// Try domain (the "from" party) first.
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, action.Context, action.Line); ok {
					return []protocol.Location{loc}, nil
				}
				// Then targetDomain (the "to" party).
				if loc, ok := resolveUseCaseRefToLocation(rm, uri, action.TargetContext, action.Line); ok {
					return []protocol.Location{loc}, nil
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
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1)},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(len(sym.Name))},
			},
		}, true
	case "domain", "bounded_context":
		sym := target.Domain
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1)},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(len(sym.Name))},
			},
		}, true
	case "service":
		sym := target.Service
		if sym.Line == 0 {
			return protocol.Location{}, false
		}
		return protocol.Location{
			URI: protocol.DocumentURI(sym.URI),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1)},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(len(sym.Name))},
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
	if f == nil || f.AST == nil {
		return nil, nil
	}

	var syms []interface{}
	for _, svc := range f.AST.Services {
		line := 0
		if svc.Line > 0 {
			line = svc.Line - 1
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           svc.Name,
			Kind:           protocol.SymbolKindModule,
			Detail:         "service",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
		})
	}
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
	for _, uc := range f.AST.UseCases {
		line := 0
		if uc.Line > 0 {
			line = uc.Line - 1
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:   uc.Name,
			Kind:   protocol.SymbolKindEvent, // closest match for use-case-level interactions
			Detail: "use_case",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
		})
	}
	for _, arch := range f.AST.Archs {
		line := 0
		if arch.Line > 0 {
			line = arch.Line - 1
		}
		name := arch.Name
		if name == "" {
			name = "arch"
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           name,
			Kind:           protocol.SymbolKindPackage,
			Detail:         "arch",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
		})
	}
	for _, exp := range f.AST.Exposures {
		line := 0
		if exp.Line > 0 {
			line = exp.Line - 1
		}
		syms = append(syms, protocol.DocumentSymbol{
			Name:           exp.Name,
			Kind:           protocol.SymbolKindInterface,
			Detail:         "exposure",
			SelectionRange: protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
			Range:          protocol.Range{Start: protocol.Position{Line: uint32(line)}, End: protocol.Position{Line: uint32(line)}},
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
		if f.AST == nil {
			continue
		}
		for _, uc := range f.AST.UseCases {
			entryPoint, involved := extractUseCaseContexts(uc)
			result.UseCases = append(result.UseCases, craftUseCaseEntry{
				Name:              uc.Name,
				URI:               f.URI,
				StartLine:         uc.Line,
				EndLine:           uc.EndLine,
				InCurrentFile:     currentFileURI != "" && f.URI == currentFileURI,
				EntryPointContext: entryPoint,
				InvolvedContexts:  involved,
			})
		}
		for _, blockRange := range f.AST.ActorBlocks {
			result.ActorBlocks = append(result.ActorBlocks, craftBlockRef{
				FileURI:   f.URI,
				StartLine: blockRange.Line,
				EndLine:   blockRange.EndLine,
			})
		}
		for _, arch := range f.AST.Archs {
			result.Archs = append(result.Archs, craftArchEntry{
				Name:          arch.Name,
				URI:           f.URI,
				StartLine:     arch.Line,
				EndLine:       arch.EndLine,
				InCurrentFile: currentFileURI != "" && f.URI == currentFileURI,
			})
		}
	}

	return result
}

// extractUseCaseContexts derives the entry-point bounded context and all involved contexts
// from a use case declaration by inspecting its scenario triggers and actions.
// The entry point is the first domain that appears (trigger domain for domain_listen,
// otherwise the first action Domain). Involved is the deduplicated set of all Domain
// and TargetDomain values across every scenario.
func extractUseCaseContexts(uc *ast.UseCaseDecl) (entryPoint string, involved []string) {
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
	for _, sc := range uc.Scenarios {
		if sc.Trigger.TriggerType == "domain_listen" {
			add(sc.Trigger.Context)
		}
		for _, action := range sc.Actions {
			add(action.Context)
			add(action.TargetContext)
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
		if f == nil || f.AST == nil {
			continue
		}
		contentLines := strings.Split(f.Content, "\n")

		for _, r := range byFile[fileURI] {
			part := reconstructDSLRange(f.AST, contentLines, r.startLine, r.endLine)
			if part != "" {
				parts = append(parts, part)
			}
		}
	}

	return strings.Join(parts, "\n\n")
}

// reconstructDSLRange finds the AST declaration whose line range matches startLine:endLine
// and returns well-formed DSL. Falls back to raw line slicing for non-service/domain ranges.
// Returns empty string if no content is available for the range.
func reconstructDSLRange(file *ast.File, contentLines []string, startLine, endLine int) string {
	for _, svc := range file.Services {
		if svc.Line == startLine && svc.EndLine == endLine {
			return reconstructService(svc)
		}
	}
	for _, dom := range file.Domains {
		if dom.Line == startLine && dom.EndLine == endLine {
			return reconstructDomain(dom)
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

// reconstructService builds valid DSL for a single service declaration.
// Grouped services (from services { }) are wrapped; top-level services use the service keyword.
func reconstructService(svc *ast.ServiceDecl) string {
	var sb strings.Builder
	indent := "  "
	fieldIndent := "    "
	if !svc.IsGrouped {
		sb.WriteString("service ")
		sb.WriteString(svc.Name)
		sb.WriteString(" {\n")
		indent = ""
		fieldIndent = "  "
	} else {
		sb.WriteString("services {\n")
		sb.WriteString(indent)
		sb.WriteString(svc.Name)
		sb.WriteString(" {\n")
	}
	if len(svc.Contexts) > 0 {
		sb.WriteString(fieldIndent)
		sb.WriteString("contexts: ")
		sb.WriteString(strings.Join(svc.Contexts, ", "))
		sb.WriteString("\n")
	}
	if len(svc.DataStores) > 0 {
		sb.WriteString(fieldIndent)
		sb.WriteString("data-stores: ")
		sb.WriteString(strings.Join(svc.DataStores, ", "))
		sb.WriteString("\n")
	}
	if svc.Language != "" {
		sb.WriteString(fieldIndent)
		sb.WriteString("language: ")
		sb.WriteString(svc.Language)
		sb.WriteString("\n")
	}
	if svc.DeploymentType != "" {
		sb.WriteString(fieldIndent)
		sb.WriteString("deployment: ")
		sb.WriteString(svc.DeploymentType)
		if len(svc.DeploymentRules) > 0 {
			sb.WriteString("(")
			for i, rule := range svc.DeploymentRules {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(rule.Percentage)
				sb.WriteString(" -> ")
				sb.WriteString(rule.Target)
			}
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	if svc.IsGrouped {
		sb.WriteString(indent)
		sb.WriteString("}\n}")
	} else {
		sb.WriteString("}")
	}
	return sb.String()
}

// reconstructDomain builds valid DSL for a single domain declaration.
// Grouped domains (from domains { }) are wrapped; top-level domains use the domain keyword.
func reconstructDomain(dom *ast.DomainDecl) string {
	var sb strings.Builder
	indent := "  "
	bcIndent := "    "
	if !dom.IsGrouped {
		sb.WriteString("domain ")
		sb.WriteString(dom.Name)
		sb.WriteString(" {\n")
		indent = ""
		bcIndent = "  "
	} else {
		sb.WriteString("domains {\n")
		sb.WriteString(indent)
		sb.WriteString(dom.Name)
		sb.WriteString(" {\n")
	}
	for _, bc := range dom.BoundedContexts {
		sb.WriteString(bcIndent)
		sb.WriteString(bc.Name)
		sb.WriteString("\n")
	}
	if dom.IsGrouped {
		sb.WriteString(indent)
		sb.WriteString("}\n}")
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
	if f == nil || f.AST == nil {
		return nil, nil
	}

	var ranges []protocol.FoldingRange

	for _, arch := range f.AST.Archs {
		if arch.Line <= 0 || arch.EndLine <= arch.Line {
			continue
		}
		// One fold for the entire arch block.
		startLine := uint32(arch.Line - 1)
		endLine := uint32(arch.EndLine - 1)
		ranges = append(ranges, protocol.FoldingRange{
			StartLine: startLine,
			EndLine:   endLine,
			Kind:      protocol.RegionFoldingRange,
		})

		// One fold per labelled section inside arch.
		// presentation: folds from its label line to the line before gateway: (or arch end).
		if arch.PresentationLine > 0 {
			sectionStart := uint32(arch.PresentationLine - 1)
			sectionEnd := endLine - 1 // default: end just before closing `}`
			if arch.GatewayLine > 0 && arch.GatewayLine > arch.PresentationLine {
				sectionEnd = uint32(arch.GatewayLine - 2) // end line before gateway label
			}
			if sectionEnd > sectionStart {
				ranges = append(ranges, protocol.FoldingRange{
					StartLine: sectionStart,
					EndLine:   sectionEnd,
					Kind:      protocol.RegionFoldingRange,
				})
			}
		}
		if arch.GatewayLine > 0 {
			sectionStart := uint32(arch.GatewayLine - 1)
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

	for _, svc := range f.AST.Services {
		if svc.Line == 0 {
			continue
		}
		if svc.Line == cursorLine {
			detail := "service: " + svc.Name
			if svc.Language != "" {
				detail += " (" + svc.Language + ")"
			}
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.PlainText,
					Value: detail,
				},
			}, nil
		}
	}

	for _, exp := range f.AST.Exposures {
		if exp.Line == 0 || exp.Line != cursorLine {
			continue
		}
		detail := "exposure: " + exp.Name
		if len(exp.To) > 0 {
			detail += " → " + strings.Join(exp.To, ", ")
		}
		return &protocol.Hover{
			Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: detail},
		}, nil
	}

	// Walk use-case bodies: hover on an action line shows the resolved declaration.
	rm := s.ws.ResolutionMap()
	uri := string(params.TextDocument.URI)
	for _, uc := range f.AST.UseCases {
		for _, sc := range uc.Scenarios {
			// Hover on the use_case name itself (line of use_case keyword).
			if uc.Line == cursorLine {
				return &protocol.Hover{
					Contents: protocol.MarkupContent{
						Kind:  protocol.PlainText,
						Value: "use_case: " + uc.Name,
					},
				}, nil
			}
			// Hover on trigger line.
			if sc.Trigger.Line == cursorLine {
				name := sc.Trigger.Actor
				if name == "" {
					name = sc.Trigger.Context
				}
				if name != "" {
					if val := hoverForUseCaseRef(rm, uri, name, sc.Trigger.Line); val != "" {
						return &protocol.Hover{
							Contents: protocol.MarkupContent{Kind: protocol.PlainText, Value: val},
						}, nil
					}
				}
			}
			// Hover on action lines.
			for _, action := range sc.Actions {
				if action.Line != cursorLine {
					continue
				}
				// Show hover for the "from" domain first.
				if val := hoverForUseCaseRef(rm, uri, action.Context, action.Line); val != "" {
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

// Semantic token type constants for the three kinds in scope through S5.
// Their index in the legend slice is their numeric encoding in the data stream.
const (
	semanticTokenTypeActor   protocol.SemanticTokenTypes = "class"     // index 0
	semanticTokenTypeDomain  protocol.SemanticTokenTypes = "namespace" // index 1
	semanticTokenTypeService protocol.SemanticTokenTypes = "interface" // index 2
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
			TokenTypes:     []string{string(semanticTokenTypeActor), string(semanticTokenTypeDomain), string(semanticTokenTypeService)},
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
	case semanticTokenTypeService:
		return 2
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
		col := uint32(0)
		if a.Column > 0 {
			col = uint32(a.Column - 1)
		}
		tokens = append(tokens, semanticToken{
			line:      uint32(a.Line - 1),
			startChar: col,
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
		col := uint32(0)
		if d.Column > 0 {
			col = uint32(d.Column - 1)
		}
		tokens = append(tokens, semanticToken{
			line:      uint32(d.Line - 1),
			startChar: col,
			length:    uint32(len([]rune(d.Name))),
			tokenType: semanticTokenTypeIndex(semanticTokenTypeDomain),
			modifiers: 1, // declaration
		})
		// Bounded contexts within the domain also get namespace tokens.
		for _, bc := range d.BoundedContexts {
			if bc.Line <= 0 {
				continue
			}
			bcCol := uint32(0)
			if bc.Column > 0 {
				bcCol = uint32(bc.Column - 1)
			}
			tokens = append(tokens, semanticToken{
				line:      uint32(bc.Line - 1),
				startChar: bcCol,
				length:    uint32(len([]rune(bc.Name))),
				tokenType: semanticTokenTypeIndex(semanticTokenTypeDomain),
				modifiers: 0,
			})
		}
	}

	// Services: token type index 2 ("interface"), modifier 1 (declaration).
	for _, svc := range f.AST.Services {
		if svc.Line <= 0 {
			continue
		}
		col := uint32(0)
		if svc.Column > 0 {
			col = uint32(svc.Column - 1)
		}
		tokens = append(tokens, semanticToken{
			line:      uint32(svc.Line - 1),
			startChar: col,
			length:    uint32(len([]rune(svc.Name))),
			tokenType: semanticTokenTypeIndex(semanticTokenTypeService),
			modifiers: 1, // declaration
		})
	}

	// Use-case action parties: colour actors/domains/services referenced inside
	// when clauses using their respective semantic token types (S6).
	rm := s.ws.ResolutionMap()
	uri := string(params.TextDocument.URI)
	for _, uc := range f.AST.UseCases {
		for _, sc := range uc.Scenarios {
			// Trigger subject.
			triggerName := sc.Trigger.Actor
			if triggerName == "" {
				triggerName = sc.Trigger.Context
			}
			if triggerName != "" && sc.Trigger.Line > 0 {
				if tt, ok := useCaseRefTokenType(rm, uri, triggerName, sc.Trigger.Line); ok {
					tokens = append(tokens, semanticToken{
						line:      uint32(sc.Trigger.Line - 1),
						startChar: 0,
						length:    uint32(len([]rune(triggerName))),
						tokenType: tt,
					})
				}
			}
			// Action domain and targetDomain.
			for _, action := range sc.Actions {
				if action.Line <= 0 {
					continue
				}
				if action.Context != "" {
					if tt, ok := useCaseRefTokenType(rm, uri, action.Context, action.Line); ok {
						tokens = append(tokens, semanticToken{
							line:      uint32(action.Line - 1),
							startChar: 0,
							length:    uint32(len([]rune(action.Context))),
							tokenType: tt,
						})
					}
				}
			}
		}
	}

	// Sort tokens by line, then character (required for relative encoding).
	sortSemanticTokens(tokens)

	return &protocol.SemanticTokens{
		Data: encodeSemanticTokens(tokens),
	}, nil
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
		return semanticTokenTypeIndex(semanticTokenTypeActor), true
	case "domain", "bounded_context":
		return semanticTokenTypeIndex(semanticTokenTypeDomain), true
	case "service":
		return semanticTokenTypeIndex(semanticTokenTypeService), true
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
