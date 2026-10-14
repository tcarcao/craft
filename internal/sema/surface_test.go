package sema_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// analyzeFiles runs AnalyzeFile, MergeWorkspaceSymbols and AnalyzeWorkspace
// over files and returns every diagnostic.
func analyzeFiles(t *testing.T, files map[string]string) []model.Diagnostic {
	t.Helper()
	perFile := map[string]sema.Symbols{}
	var diags []model.Diagnostic
	for uri, src := range files {
		g, li, _ := syntax.Parse(src)
		syms, d := sema.AnalyzeFile(uri, syntax.Root(g), li)
		perFile[uri] = syms
		diags = append(diags, d...)
	}
	ws, d := sema.MergeWorkspaceSymbols(perFile)
	diags = append(diags, d...)
	_, d = sema.AnalyzeWorkspace(perFile, ws)
	return append(diags, d...)
}

func withCode(diags []model.Diagnostic, code string) []model.Diagnostic {
	var out []model.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

func TestSurface_DuplicateAcrossFilesIsAWarningOnTheSecond(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "domain obido {\n    concierge\n}\nsurface concierge {\n    query the list  [getUserList]\n}\n",
		"b.craft": "surface concierge {\n    query the logs  [getUserLogs]\n}\n",
	})
	dups := withCode(diags, "craft/sema/duplicate-name")
	if len(dups) != 1 || dups[0].SourceURI != "b.craft" || dups[0].Severity != model.SeverityWarning || !strings.Contains(dups[0].Message, "a.craft:4") {
		t.Fatalf("duplicate-name = %+v", dups)
	}
}

func TestSurface_DuplicateInOneFileIsAnError(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "domain obido {\n    concierge\n}\nsurface concierge {\n    query the list  [getUserList]\n}\nsurface obido/concierge {\n    query the logs  [getUserLogs]\n}\n",
	})
	dups := withCode(diags, "craft/sema/duplicate-name")
	if len(dups) != 1 || dups[0].Severity != model.SeverityError || dups[0].Range.Start.Line != 6 {
		t.Fatalf("duplicate-name = %+v", dups)
	}
}

func TestSurface_Callers(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "actor user Advisor\ndomain obido {\n    concierge\n}\nsurface concierge {\n    callers: Advisor, unknown, Ghost\n    query the list  [getUserList]\n}\n",
	})
	unres := withCode(diags, "craft/sema/unresolved-reference")
	if len(unres) != 1 || !strings.Contains(unres[0].Message, `"Ghost"`) || unres[0].Severity != model.SeverityError {
		t.Fatalf("unresolved = %+v", unres)
	}
	if r := unres[0].Range; r.Start.Line != 5 || r.Start.Character != 31 || r.End.Character != 36 {
		t.Errorf("range = %+v", r)
	}
}

func TestSurface_SubjectCheckedWithoutOtherDeclarations(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{"a.craft": "surface ghost {\n    query the list  [x]\n}\n"})
	if len(withCode(diags, "craft/sema/unresolved-reference")) != 1 {
		t.Fatalf("diags = %+v", diags)
	}
}

func TestSurface_DataStoreSubjectIsInvalid(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "services {\n    Svc {\n        data-stores: user_db\n    }\n}\nsurface user_db {\n    query the rows  [x]\n}\n",
	})
	if got := withCode(diags, "craft/sema/invalid-surface-subject"); len(got) != 1 || !strings.Contains(got[0].Message, "data store") {
		t.Fatalf("diags = %+v", diags)
	}
}
