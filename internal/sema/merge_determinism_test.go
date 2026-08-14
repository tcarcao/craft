package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// MergeWorkspaceSymbols indexes every symbol kind by bare name, so a name
// declared in two files has exactly one winner — and that winner is what
// downstream consumers report as THE declaration: the file and line an
// unused-actor finding blames, the domain a bare bounded-context reference
// resolves to, the position go-to-definition jumps to. Ranging over perFile
// directly let Go's map seed choose it, which is the same defect the lint rules
// had, one layer lower and reachable from both the CLI and the LSP.
func mergeFixture(t *testing.T) map[string]sema.Symbols {
	t.Helper()
	sources := map[string]string{
		// Same actor name, same bounded-context name, same use-case name, each
		// declared in both files: three distinct dedup keys, one race each.
		"file:///a.craft": `actor user Ghost

domain da {
  Billing
}

use_case "Shared" {
  when Ghost submits Payment
    Billing charges the card
}`,
		"file:///b.craft": `actor user Ghost

domain db {
  Billing
}

use_case "Shared" {
  when Ghost submits Payment
    Billing charges the card
}`,
	}
	perFile := make(map[string]sema.Symbols, len(sources))
	for uri, src := range sources {
		g, li, _ := syntax.Parse(src)
		syms, _ := sema.AnalyzeFile(uri, syntax.Root(g), li)
		perFile[uri] = syms
	}
	return perFile
}

func TestMergeWorkspaceSymbols_FirstFileInSortedOrderWins(t *testing.T) {
	perFile := mergeFixture(t)

	for run := 0; run < 50; run++ {
		ws, _ := sema.MergeWorkspaceSymbols(perFile)

		if got := ws.Actors["Ghost"].URI; got != "file:///a.craft" {
			t.Fatalf("run %d: actor Ghost attributed to %q, want the first file in sorted order", run, got)
		}
		if got := ws.BoundedContexts["Billing"].Name; got != "da" {
			t.Fatalf("run %d: bare Billing resolved to domain %q, want da", run, got)
		}
		if got := ws.BCPositions["Billing"].URI; got != "file:///a.craft" {
			t.Fatalf("run %d: Billing position points at %q, want the first file in sorted order", run, got)
		}
		if got := ws.UseCases["Shared"].URI; got != "file:///a.craft" {
			t.Fatalf("run %d: use case Shared attributed to %q, want the first file in sorted order", run, got)
		}
	}
}

// Renaming the files must move the winner, which is what proves the assertion
// above is reading sorted order rather than an accidental insertion order.
func TestMergeWorkspaceSymbols_WinnerFollowsSortOrderNotInsertion(t *testing.T) {
	perFile := mergeFixture(t)
	// Re-key so the file that was second now sorts first.
	renamed := map[string]sema.Symbols{
		"file:///m.craft": perFile["file:///b.craft"],
		"file:///z.craft": perFile["file:///a.craft"],
	}
	ws, _ := sema.MergeWorkspaceSymbols(renamed)
	if got := ws.BoundedContexts["Billing"].Name; got != "db" {
		t.Errorf("expected the domain from m.craft (db) to win, got %q", got)
	}
}
