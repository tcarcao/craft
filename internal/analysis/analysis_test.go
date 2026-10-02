package analysis_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/model"
)

func workspaceOf(files map[string]string) analysis.Result {
	var results []analysis.FileResult
	for name, src := range files {
		results = append(results, analysis.File(name, src))
	}
	return analysis.Workspace(results)
}

func TestFile_StampsNameOnDiagnosticsAndUseCases(t *testing.T) {
	f := analysis.File("dir/a.craft", "use_case \"U\" {\n  when Bob does thing\n    Ghost thinks\n}\nservices {\n")
	if len(f.Diagnostics) == 0 {
		t.Fatal("expected diagnostics for broken input")
	}
	for _, d := range f.Diagnostics {
		if d.SourceURI != "dir/a.craft" {
			t.Errorf("diagnostic %s has SourceURI %q, want the file name", d.Code, d.SourceURI)
		}
	}
	if got := f.Doc.UseCases[0].SourceURI; got != "dir/a.craft" {
		t.Errorf("use case SourceURI = %q, want the file name", got)
	}
}

func TestFile_MergesSameNameServiceBlocksWithinAFile(t *testing.T) {
	src := "services {\n  Svc {\n    contexts: A\n    language: go\n  }\n}\nservices {\n  Svc {\n    contexts: B, A\n    language: java\n    data-stores: db\n  }\n}\n"
	svcs := analysis.File("a.craft", src).Doc.Services
	if len(svcs) != 1 {
		t.Fatalf("want one merged service, got %d", len(svcs))
	}
	s := svcs[0]
	if strings.Join(s.Contexts, ",") != "A,B" || strings.Join(s.DataStores, ",") != "db" || s.Language != "go" {
		t.Errorf("merged service = %+v; want contexts A,B, data-stores db, first language go", s)
	}
}

// Across files the first declaration (in sorted file order) wins, as it does
// in sema, and the later one is reported.
func TestWorkspace_FirstDeclarationWinsAcrossFiles(t *testing.T) {
	res := workspaceOf(map[string]string{
		"b.craft": "domain Sales {\n  Billing\n}\nservices {\n  Svc {\n    contexts: Billing\n  }\n}\n",
		"a.craft": "domain Sales {\n  Ordering\n}\nservices {\n  Svc {\n    contexts: Ordering\n  }\n}\n",
	})
	if len(res.Doc.Services) != 1 || res.Doc.Services[0].Contexts[0] != "Ordering" {
		t.Errorf("services = %+v; want only a.craft's Svc", res.Doc.Services)
	}
	if len(res.Doc.Domains) != 1 || res.Doc.Domains[0].BoundedContexts[0] != "Ordering" {
		t.Errorf("domains = %+v; want only a.craft's Sales", res.Doc.Domains)
	}
	var dupDomain, dupService bool
	for _, d := range res.WorkspaceDiagnostics {
		if d.Code == "craft/sema/duplicate-name" && d.SourceURI == "b.craft" {
			dupDomain = dupDomain || strings.Contains(d.Message, "domain")
			dupService = dupService || strings.Contains(d.Message, "service")
		}
	}
	if !dupDomain || !dupService {
		t.Errorf("want duplicate-name warnings on b.craft for the domain and the service, got %+v", res.WorkspaceDiagnostics)
	}
}

func TestWorkspace_IsIndependentOfInputOrder(t *testing.T) {
	files := []analysis.FileResult{
		analysis.File("z.craft", "use_case \"Z\" {\n  when Bob does z\n    Nowhere thinks\n}\n"),
		analysis.File("a.craft", "use_case \"A\" {\n  when Bob does a\n    Ordering notifies \"x happened\"\n}\n"),
		analysis.File("m.craft", "domain D {\n  Ordering\n}\n"),
	}
	first := analysis.Workspace(files)
	reversed := analysis.Workspace([]analysis.FileResult{files[2], files[1], files[0]})
	for i := 0; i < 10; i++ {
		again := analysis.Workspace(files)
		if mustJSON(t, again) != mustJSON(t, first) {
			t.Fatal("repeated Workspace calls disagree")
		}
	}
	if mustJSON(t, reversed) != mustJSON(t, first) {
		t.Error("Workspace result depends on the order files are passed in")
	}
	if first.Doc.UseCases[0].Name != "A" {
		t.Errorf("use cases should follow sorted file order, got %q first", first.Doc.UseCases[0].Name)
	}
}

func TestWorkspace_DiagnosticsIncludePerFileAndWorkspaceSubset(t *testing.T) {
	res := workspaceOf(map[string]string{
		"a.craft": "services {\n  Svc {\n    contexts: Missing\n  }\n}\nuse_case \"U\" {\n",
	})
	codes := map[string]bool{}
	for _, d := range res.Diagnostics {
		codes[d.Code] = true
	}
	hasSyntax := false
	for c := range codes {
		hasSyntax = hasSyntax || strings.HasPrefix(c, "craft/syntax/")
	}
	if !hasSyntax || !codes["craft/sema/unresolved-reference"] {
		t.Errorf("Diagnostics should hold per-file (syntax) and workspace (resolution) findings, got %v", codes)
	}
	for _, d := range res.WorkspaceDiagnostics {
		if strings.HasPrefix(d.Code, "craft/syntax/") {
			t.Errorf("WorkspaceDiagnostics must not repeat per-file diagnostic %s", d.Code)
		}
	}
}

func TestWorkspace_Empty(t *testing.T) {
	res := analysis.Workspace(nil)
	if res.Doc == nil || res.Doc.UseCases == nil {
		t.Fatal("empty workspace must still return a document with a non-nil use case list")
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("empty workspace has diagnostics: %v", res.Diagnostics)
	}
}

func mustJSON(t *testing.T, r analysis.Result) string {
	t.Helper()
	b, err := json.Marshal(struct {
		Doc   *model.CraftDoc
		Diags []model.Diagnostic
	}{r.Doc, r.Diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
