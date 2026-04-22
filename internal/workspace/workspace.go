// Package workspace maintains the multi-file index for the Craft LSP server.
// On initialize it scans the workspace root for all .craft files and builds a
// URI-keyed store. Open/change/close/save lifecycle keeps each file's parse
// result current. Per-file ASTs are cached by content hash and reused when
// the content has not changed (Q21 compute model).
//
// S3: multi-file aware from day one (Q15). Only the single-file slice is used
// by S3 handlers, but the index exists so S5+ can add cross-file resolution
// without retrofitting.
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

	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

// File holds the cached parse result for a single .craft document.
type File struct {
	URI         string
	Content     string
	ContentHash [32]byte
	AST         *ast.File
	Diagnostics []craft.Diagnostic
}

// Workspace is the multi-file index. It is safe for concurrent access.
type Workspace struct {
	mu    sync.RWMutex
	files map[string]*File // keyed by URI
	log   *slog.Logger
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
}

// Open registers a file opened by the editor. Content is the initial text.
func (w *Workspace) Open(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.parseAndStore(uri, content)
}

// Change updates a file's content in response to a didChange notification.
func (w *Workspace) Change(uri, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.parseAndStore(uri, content)
}

// Close removes a file from the in-memory index (the on-disk file remains).
func (w *Workspace) Close(uri string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.files, uri)
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
				if file.AST == nil {
					file.AST = &ast.File{}
				}
				file.Diagnostics = append(file.Diagnostics, craft.Diagnostic{
					Code:     "craft/internal/parser-panic",
					Message:  fmt.Sprintf("internal parser error: %v", r),
					Severity: craft.SeverityError,
				})
			}
		}()
		file.AST, file.Diagnostics = syntax.Parse(content)
	}()

	w.files[uri] = file
}

// pathToURI converts an absolute file path to a file:// URI.
func pathToURI(path string) string {
	return "file://" + filepath.ToSlash(path)
}
