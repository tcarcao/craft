package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/green"
	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

func lintFiles(t *testing.T, files map[string]string) []model.Diagnostic {
	t.Helper()
	trees := map[string]syntax.SyntaxNode{}
	lis := map[string]green.LineIndex{}
	perFile := map[string]sema.Symbols{}
	for uri, src := range files {
		g, li, diags := syntax.Parse(src)
		if len(diags) != 0 {
			t.Fatalf("%s: parse diags %v", uri, diags)
		}
		trees[uri], lis[uri] = syntax.Root(g), li
		perFile[uri], _ = sema.AnalyzeFile(uri, trees[uri], li)
	}
	ws, _ := sema.MergeWorkspaceSymbols(perFile)
	return sema.LintWorkspace(trees, ws, lis)
}

func byCode(diags []model.Diagnostic, code string) []model.Diagnostic {
	var out []model.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

func TestLint_DuplicateDisplayName(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "domain billing {\n    nmf \"the money engine\"\n    pv \"portfolio\"\n}\n",
		"file:///b.craft": "actors {\n    user Seller \"the money engine\"\n    user Buyer \"portfolio value\"\n}\n",
	})
	got := byCode(diags, "craft/lint/duplicate-display-name")
	if len(got) != 1 {
		t.Fatalf("duplicate-display-name = %v", got)
	}
	d := got[0]
	if d.SourceURI != "file:///b.craft" || d.Severity != model.SeverityWarning {
		t.Errorf("diag = %+v", d)
	}
	// The range is the later string, quotes included.
	want := model.Range{Start: model.Position{Line: 1, Character: 16}, End: model.Position{Line: 1, Character: 34}}
	if d.Range != want {
		t.Errorf("range = %+v, want %+v", d.Range, want)
	}
}

// The same actor declared again in another file is one identity, not two.
func TestLint_DuplicateDisplayName_SameDeclarationIsNotADuplicate(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "actor user Seller \"the seller\"\n",
		"file:///b.craft": "actor user Seller \"the seller\"\n",
	})
	if got := byCode(diags, "craft/lint/duplicate-display-name"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestLint_EmptyDisplayName(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "domain billing {\n    nmf \"\"\n}\nactor user Seller \"\"\n",
	})
	got := byCode(diags, "craft/lint/empty-display-name")
	if len(got) != 2 {
		t.Fatalf("empty-display-name = %v", got)
	}
	if got[0].Range.Start.Line != 1 || got[0].Range.Start.Character != 8 || got[0].Range.End.Character != 10 {
		t.Errorf("range = %+v", got[0].Range)
	}
}

func TestSymbols_CarryDisplayNames(t *testing.T) {
	g, li, _ := syntax.Parse("domain billing {\n    nmf \"the money engine\"\n    pv\n}\nactor user Seller \"the seller\"\n")
	syms, _ := sema.AnalyzeFile("file:///a.craft", syntax.Root(g), li)
	if syms.Domains[0].DisplayNames["nmf"] != "the money engine" || len(syms.Domains[0].DisplayNames) != 1 {
		t.Errorf("domain display names = %v", syms.Domains[0].DisplayNames)
	}
	if syms.Actors[0].DisplayName != "the seller" {
		t.Errorf("actor display name = %q", syms.Actors[0].DisplayName)
	}
}

// The same actor or context declared again with another display name: the
// first one (in sorted file order) is drawn, so the later one is reported.
func TestLint_ConflictingDisplayName(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "actor user Seller \"the seller\"\ndomain billing {\n    nmf \"the money engine\"\n}\n",
		"file:///b.craft": "actor user Seller \"the vendor\"\ndomain billing {\n    nmf \"nMF\"\n}\n",
	})
	got := byCode(diags, "craft/lint/conflicting-display-name")
	if len(got) != 2 {
		t.Fatalf("conflicting-display-name = %v", got)
	}
	want := []struct {
		line, start, end int
		msg              string
	}{
		{0, 18, 30, `Seller already has the display name "the seller" (file:///a.craft), which diagrams draw; use the same name here or remove this one`},
		{2, 8, 13, `billing/nmf already has the display name "the money engine" (file:///a.craft), which diagrams draw; use the same name here or remove this one`},
	}
	for i, w := range want {
		d := got[i]
		if d.SourceURI != "file:///b.craft" || d.Severity != model.SeverityWarning || d.Message != w.msg ||
			d.Range.Start.Line != w.line || d.Range.Start.Character != w.start || d.Range.End.Character != w.end {
			t.Errorf("diag %d = %+v", i, d)
		}
	}
	// A conflict is not also a duplicate.
	if dup := byCode(diags, "craft/lint/duplicate-display-name"); len(dup) != 0 {
		t.Errorf("duplicate-display-name = %v", dup)
	}
}

// A first declaration without a display name wins too: the later name is
// never drawn.
func TestLint_ConflictingDisplayName_FirstHasNone(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "actor user Seller\n",
		"file:///b.craft": "actor user Seller \"the vendor\"\n",
	})
	got := byCode(diags, "craft/lint/conflicting-display-name")
	if len(got) != 1 || got[0].SourceURI != "file:///b.craft" ||
		got[0].Message != "Seller is first declared without a display name (file:///a.craft), so diagrams draw its id; move the display name there or remove this one" {
		t.Fatalf("conflicting-display-name = %v", got)
	}
}
