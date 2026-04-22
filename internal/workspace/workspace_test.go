package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tcarcao/craft/internal/workspace"
)

func TestWorkspace_OpenAndGet(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///a.craft", "actor user Alice")

	f := w.Get("file:///a.craft")
	if f == nil {
		t.Fatal("file not found after Open")
	}
	if len(f.AST.Actors) != 1 {
		t.Errorf("expected 1 actor, got %d", len(f.AST.Actors))
	}
	if f.AST.Actors[0].Name != "Alice" {
		t.Errorf("got actor name %q", f.AST.Actors[0].Name)
	}
}

func TestWorkspace_Change(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///a.craft", "actor user Alice")
	w.Change("file:///a.craft", "actor user Bob")

	f := w.Get("file:///a.craft")
	if f.AST.Actors[0].Name != "Bob" {
		t.Errorf("expected Bob after change, got %q", f.AST.Actors[0].Name)
	}
}

func TestWorkspace_ContentHashReuse(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///a.craft", "actor user Alice")
	f1 := w.Get("file:///a.craft")
	// Same content again should be a no-op; pointer should be the same.
	w.Change("file:///a.craft", "actor user Alice")
	f2 := w.Get("file:///a.craft")
	if f1 != f2 {
		t.Error("expected cached file to be reused for identical content")
	}
}

func TestWorkspace_Close(t *testing.T) {
	w := workspace.New(nil)
	w.Open("file:///a.craft", "actor user Alice")
	w.Close("file:///a.craft")
	if w.Get("file:///a.craft") != nil {
		t.Error("file should be gone after Close")
	}
}

func TestWorkspace_Initialize(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.craft"), []byte("actor user Alice"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "b.craft"), []byte("actor system Bot"), 0644)

	w := workspace.New(nil)
	w.Initialize(dir)

	files := w.AllFiles()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}
