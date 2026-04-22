package workspace_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestWorkspace_PerformanceGate_S5 verifies the Q21 performance gate:
// a synthetic 20-file workspace (5 actor files, 5 domain files, 10 service files
// referencing the domains) completes a full Change+resolution cycle in ≤200ms.
func TestWorkspace_PerformanceGate_S5(t *testing.T) {
	const (
		nActorFiles   = 5
		nDomainFiles  = 5
		nServiceFiles = 10
		budgetMs      = 200
	)

	w := workspace.New(nil)

	// Seed actor files.
	for i := 0; i < nActorFiles; i++ {
		uri := fmt.Sprintf("file:///actors_%d.craft", i)
		content := fmt.Sprintf("actor user User%d\nactor system System%d", i, i)
		w.Open(uri, content)
	}

	// Seed domain files with bounded contexts.
	for i := 0; i < nDomainFiles; i++ {
		uri := fmt.Sprintf("file:///domain_%d.craft", i)
		content := fmt.Sprintf("domain Domain%d {\n  BoundedContext%dA\n  BoundedContext%dB\n}", i, i, i)
		w.Open(uri, content)
	}

	// Seed service files that reference the domain bounded contexts.
	for i := 0; i < nServiceFiles; i++ {
		domIdx := i % nDomainFiles
		uri := fmt.Sprintf("file:///service_%d.craft", i)
		content := fmt.Sprintf("services {\n  Service%d {\n    contexts: BoundedContext%dA, BoundedContext%dB\n    language: golang\n  }\n}", i, domIdx, domIdx)
		w.Open(uri, content)
	}

	// Now simulate a keystroke: Change one of the service files and time it.
	changeURI := "file:///service_0.craft"
	newContent := "services {\n  Service0 {\n    contexts: BoundedContext0A\n    language: golang\n  }\n}"

	start := time.Now()
	w.Change(changeURI, newContent)
	elapsed := time.Since(start)

	if elapsed > time.Duration(budgetMs)*time.Millisecond {
		t.Errorf("S5 perf gate: Change+resolution took %v, budget is %dms", elapsed, budgetMs)
	} else {
		t.Logf("S5 perf gate: Change+resolution took %v (budget %dms) ✓", elapsed, budgetMs)
	}
}
