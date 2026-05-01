// Package workspace maintains the multi-file index for the Craft LSP server.
// On initialize it scans the workspace root for all .craft files and builds a
// URI-keyed store. Open/change/close/save lifecycle keeps each file's parse
// result current. Per-file syntax trees are cached by content hash and reused
// when the content has not changed (Q21 compute model).
//
// S3: multi-file aware from day one (Q15). Only the single-file slice is used
// by S3 handlers, but the index exists so S5+ can add cross-file resolution
// without retrofitting.
// S5: ResolutionMap computed after every workspace change; sema.AnalyzeWorkspace
//     runs eagerly on the full workspace after each debounced parse.
package workspace

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tcarcao/craft/internal/green"
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

// File holds the cached parse result for a single .craft document.
type File struct {
	URI         string
	Content     string         // kept: hash check and UTF16Col source
	ContentHash [32]byte
	Green       *green.GreenNode  // replaces SyntaxTree
	LineIndex   green.LineIndex   // built once per open/change
	Diagnostics []craft.Diagnostic
	// Symbols is the per-file symbol table, computed after parsing.
	Symbols sema.Symbols
}

// Workspace is the multi-file index. It is safe for concurrent access.
type Workspace struct {
	mu    sync.RWMutex
	files map[string]*File // keyed by URI
	log   *slog.Logger

	// resolution holds the latest workspace-level symbol table, resolution
	// map, and cross-file diagnostics, recomputed after every change.
	resolution struct {
		ws   sema.WorkspaceSymbols
		rm   sema.ResolutionMap
		diags map[string][]craft.Diagnostic // keyed by source-file URI
	}
}

// New creates an empty workspace. Call Initialize to populate it from disk.
func New(log *slog.Logger) *Workspace {
	if log == nil {
		log = slog.Default()
	}
	return &Workspace{
		files: make(map[string]*File),
		log:   log,
	}
}

// Initialize scans rootPath for .craft files and parses each one.
// It is safe to call Initialize multiple times; re-scanning replaces the
// existing index.
func (w *Workspace) Initialize(rootPath string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".craft") {
			uri := pathToURI(path)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				w.log.Warn("workspace: cannot read file", "path", path, "err", readErr)
				return nil
			}
			w.parseAndStore(uri, string(content))
		}
		return nil
	})
	if err != nil {
		w.log.Warn("workspace: walk error", "root", rootPath, "err", err)
	}
	w.recomputeResolution()
}

// Open registers a file opened by the editor. Content is the initial text.
func (w *Workspace) Open(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.parseAndStore(uri, content)
	w.recomputeResolution()
}

// Change updates a file's content in response to a didChange notification.
func (w *Workspace) Change(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.parseAndStore(uri, content)
	w.recomputeResolution()
}

// Close removes a file from the in-memory index (the on-disk file remains).
func (w *Workspace) Close(uri string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.files, uri)
	w.recomputeResolution()
}

// Get returns the cached file for uri, or nil if not present.
func (w *Workspace) Get(uri string) *File {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.files[uri]
}

// AllFiles returns a snapshot of all indexed files.
func (w *Workspace) AllFiles() []*File {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*File, 0, len(w.files))
	for _, f := range w.files {
		out = append(out, f)
	}
	return out
}

// WorkspaceSymbols returns the current merged symbol table (all files).
func (w *Workspace) WorkspaceSymbols() sema.WorkspaceSymbols {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.resolution.ws
}

// ResolutionMap returns the current cross-file resolution map.
func (w *Workspace) ResolutionMap() sema.ResolutionMap {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.resolution.rm
}

// WorkspaceDiagnostics returns the cross-file diagnostics for a specific URI,
// produced by the last AnalyzeWorkspace run. Returns nil if none.
func (w *Workspace) WorkspaceDiagnostics(uri string) []craft.Diagnostic {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.resolution.diags[uri]
}

// parseAndStore parses content and stores the result. Must be called with w.mu held.
func (w *Workspace) parseAndStore(uri, content string) {
	hash := sha256.Sum256([]byte(content))

	if existing, ok := w.files[uri]; ok && existing.ContentHash == hash {
		return // nothing changed
	}

	file := &File{
		URI:         uri,
		Content:     content,
		ContentHash: hash,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				w.log.Error("workspace: parser panic recovered",
					"uri", uri,
					"panic", fmt.Sprintf("%v", r),
				)
				file.Diagnostics = append(file.Diagnostics, craft.Diagnostic{
					Code:     "craft/internal/parser-panic",
					Message:  fmt.Sprintf("internal parser error: %v", r),
					Severity: craft.SeverityError,
				})
			}
		}()
		file.Green, file.LineIndex, file.Diagnostics = syntax.Parse(content)
	}()

	// Collect per-file symbols (no cross-file resolution yet).
	syms, semaDiags := sema.AnalyzeFile(uri, syntax.Root(file.Green), file.LineIndex)
	file.Symbols = syms
	file.Diagnostics = append(file.Diagnostics, semaDiags...)

	w.files[uri] = file
}

// recomputeResolution rebuilds the workspace-level symbol table, resolution
// map, and cross-file diagnostics from the current per-file symbol tables.
// Must be called with w.mu held.
func (w *Workspace) recomputeResolution() {
	perFile := make(map[string]sema.Symbols, len(w.files))
	perFileTrees := make(map[string]syntax.SyntaxNode, len(w.files))
	for uri, f := range w.files {
		perFile[uri] = f.Symbols
		perFileTrees[uri] = syntax.Root(f.Green)
	}
	ws, mergeDiags := sema.MergeWorkspaceSymbols(perFile)
	rm, resolveDiags := sema.AnalyzeWorkspace(perFile, ws)
	lintDiags := sema.LintWorkspace(perFileTrees, ws)
	w.resolution.ws = ws
	w.resolution.rm = rm

	// Index all workspace-level diagnostics by source file URI so
	// publishDiagnostics can merge them into the per-file payload.
	allWSDiags := append(mergeDiags, resolveDiags...)
	allWSDiags = append(allWSDiags, lintDiags...)
	byURI := make(map[string][]craft.Diagnostic, len(allWSDiags))
	for _, d := range allWSDiags {
		byURI[d.SourceURI] = append(byURI[d.SourceURI], d)
	}
	w.resolution.diags = byURI
}

// pathToURI converts an absolute file path to a file:// URI.
func pathToURI(path string) string {
	return "file://" + filepath.ToSlash(path)
}
