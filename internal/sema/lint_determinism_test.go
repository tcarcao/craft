package sema_test

import (
	"fmt"
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// Every dedup-keyed lint rule accumulates into a map keyed by something coarser
// than the file — an event name, an actor name — so the FIRST file to reach a
// given key is the one the finding is reported against and every later file is
// suppressed as a duplicate. Ranging over perFileTrees directly therefore does
// not merely shuffle the output: it changes WHICH file and line a finding
// blames, because a different file wins the race on each run. The finding COUNT
// is identical every time, which is exactly what made this invisible.
//
// The workspaces below deliberately spread one dedup key across many files, so
// the race has many distinct outcomes and a single unsorted run is very unlikely
// to pick the same winner twice.

const lintDeterminismFileCount = 12

// buildDeterminismWorkspace returns a workspace of n files that each publish the
// same never-consumed, non-past-tense event. Every file is a valid, independent
// candidate for the dead-event and event-not-past-tense findings, and exactly
// one of them wins each dedup key.
func buildDeterminismWorkspace(t *testing.T, n int) (map[string]syntax.SyntaxNode, sema.WorkspaceSymbols, map[string]green.LineIndex) {
	t.Helper()
	perFile := make(map[string]sema.Symbols, n)
	trees := make(map[string]syntax.SyntaxNode, n)
	lis := make(map[string]green.LineIndex, n)

	for i := 0; i < n; i++ {
		uri := fmt.Sprintf("file:///f%02d.craft", i)
		src := fmt.Sprintf(`domain d%02d {
  bc%02d
}

use_case "U%02d" {
  when Customer submits Payment
    bc%02d notifies "Order Place"
}`, i, i, i, i)
		g, li, _ := syntax.Parse(src)
		tree := syntax.Root(g)
		syms, _ := sema.AnalyzeFile(uri, tree, li)
		perFile[uri] = syms
		trees[uri] = tree
		lis[uri] = li
	}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	return trees, ws, lis
}

func diagKey(d model.Diagnostic) string {
	return fmt.Sprintf("%s|%s|%d:%d|%s",
		d.Code, d.SourceURI, d.Range.Start.Line, d.Range.Start.Character, d.Message)
}

func findByCode(t *testing.T, diags []model.Diagnostic, code string) model.Diagnostic {
	t.Helper()
	var found []model.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly 1 %s diagnostic, got %d", code, len(found))
	}
	return found[0]
}

// The direct assertion: the winner of a dedup key is the file that sorts first,
// every single time. Without sortedTreeURIs this picks a pseudo-random file per
// run and fails with probability (n-1)/n on any given run.
func TestLintWorkspace_DedupKeyIsAttributedToTheFirstFileInSortedOrder(t *testing.T) {
	trees, ws, lis := buildDeterminismWorkspace(t, lintDeterminismFileCount)
	const wantURI = "file:///f00.craft"

	for run := 0; run < 50; run++ {
		diags := sema.LintWorkspace(trees, ws, lis)

		dead := findByCode(t, diags, "craft/lint/dead-event")
		if dead.SourceURI != wantURI {
			t.Fatalf("run %d: dead-event blamed %q, want the first file in sorted order (%q)",
				run, dead.SourceURI, wantURI)
		}
		pastTense := findByCode(t, diags, "craft/lint/event-not-past-tense")
		if pastTense.SourceURI != wantURI {
			t.Fatalf("run %d: event-not-past-tense blamed %q, want %q",
				run, pastTense.SourceURI, wantURI)
		}
	}
}

// The broader assertion the direct one implies: repeated runs over an identical
// workspace produce an identical SET of findings, not just an identical count.
func TestLintWorkspace_RepeatedRunsProduceTheSameFindings(t *testing.T) {
	trees, ws, lis := buildDeterminismWorkspace(t, lintDeterminismFileCount)

	var want []string
	for run := 0; run < 50; run++ {
		diags := sema.LintWorkspace(trees, ws, lis)
		got := make([]string, 0, len(diags))
		for _, d := range diags {
			got = append(got, diagKey(d))
		}
		// LintWorkspace itself makes no ordering promise (pkg/craft sorts at the
		// boundary), so compare as a set: what must not move is which file and
		// line each finding names.
		if run == 0 {
			want = got
			continue
		}
		if !sameSet(want, got) {
			t.Fatalf("run %d produced a different set of findings:\n first: %v\n   now: %v", run, want, got)
		}
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
		if counts[s] < 0 {
			return false
		}
	}
	return true
}

// The unused-actor rule dedups on the actor name, so an actor declared in one
// file and left unused across a workspace of many files is a second instance of
// the same race — with a different key shape.
func TestLintWorkspace_UnusedActorIsAttributedDeterministically(t *testing.T) {
	perFile := make(map[string]sema.Symbols)
	trees := make(map[string]syntax.SyntaxNode)
	lis := make(map[string]green.LineIndex)

	for i := 0; i < lintDeterminismFileCount; i++ {
		uri := fmt.Sprintf("file:///f%02d.craft", i)
		src := fmt.Sprintf(`actor user Ghost%02d

domain d%02d {
  bc%02d
}`, i, i, i)
		g, li, _ := syntax.Parse(src)
		tree := syntax.Root(g)
		syms, _ := sema.AnalyzeFile(uri, tree, li)
		perFile[uri] = syms
		trees[uri] = tree
		lis[uri] = li
	}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)

	var want []string
	for run := 0; run < 50; run++ {
		got := make([]string, 0)
		for _, d := range sema.LintWorkspace(trees, ws, lis) {
			if d.Code == "craft/lint/unused-actor" {
				got = append(got, diagKey(d))
			}
		}
		if len(got) != lintDeterminismFileCount {
			t.Fatalf("run %d: expected one unused-actor finding per file, got %d", run, len(got))
		}
		if run == 0 {
			want = got
			continue
		}
		if !sameSet(want, got) {
			t.Fatalf("run %d: unused-actor findings moved:\n first: %v\n   now: %v", run, want, got)
		}
	}
}
