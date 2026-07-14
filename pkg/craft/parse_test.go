package craft_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/pkg/craft"
)

// repoFile resolves a path relative to the repo root (pkg/craft is two levels down).
func repoFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

// mustMarshalIndent matches how the corpus goldens are encoded (see
// internal/syntax/projection_uc_test.go:73, internal/parser_diff/diagnostics_test.go:151).
func mustMarshalIndent(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	return string(b)
}

func TestParse_ValidMultiDomain(t *testing.T) {
	src := repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craft")
	doc, diags, err := craft.Parse("dsl-vnext.craft", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("nil doc")
	}
	if len(doc.UseCases) == 0 {
		t.Error("expected at least one use case")
	}
	if len(doc.Services) == 0 {
		t.Error("expected at least one service")
	}
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("unexpected error diagnostic: %s %s", d.Code, d.Message)
		}
	}
}

func TestParse_SyntaxError(t *testing.T) {
	// A block opened but never closed / garbage tokens => at least one parse diagnostic.
	src := []byte("use_case \"x\" {\n  when \n")
	doc, diags, err := craft.Parse("bad.craft", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("Parse must always return a non-nil doc")
	}
	if len(diags) == 0 {
		t.Error("expected at least one diagnostic for malformed input")
	}
}

func TestParse_DiagnosticsAreDataNotError(t *testing.T) {
	_, _, err := craft.Parse("bad.craft", []byte("services {"))
	if err != nil {
		t.Errorf("err slot must be nil for content diagnostics, got %v", err)
	}
}

func TestParse_MatchesCorpusGolden(t *testing.T) {
	// Parse's doc must equal the existing projection golden byte-for-byte,
	// proving Parse is a faithful wrapper of ProjectFromTree.
	src := repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craft")
	doc, _, _ := craft.Parse("dsl-vnext.craft", src)
	got := mustMarshalIndent(t, doc)
	want := strings.TrimRight(string(repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craftjson")), "\n")
	if strings.TrimRight(got, "\n") != want {
		t.Errorf("Parse doc != corpus golden:\n got: %s\nwant: %s", got, want)
	}
}

func TestParseFiles_MergeMultiDomain(t *testing.T) {
	files := map[string][]byte{
		"a.craft": []byte("domain Billing {\n  Invoicing\n}\n"),
		"b.craft": []byte("domain Catalog {\n  Products\n}\n"),
	}
	doc, _, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Domains) != 2 {
		t.Fatalf("expected 2 merged domains, got %d", len(doc.Domains))
	}
	// D5: deterministic ascending-key order => a.craft (Billing) before b.craft (Catalog).
	if doc.Domains[0].Name != "Billing" || doc.Domains[1].Name != "Catalog" {
		t.Errorf("merge order not sorted by filename: %+v", doc.Domains)
	}
}

func TestParseFiles_DeterministicRegardlessOfMapOrder(t *testing.T) {
	files := map[string][]byte{
		"z.craft": []byte("domain Zeta {\n  Z\n}\n"),
		"a.craft": []byte("domain Alpha {\n  A\n}\n"),
	}
	var first string
	for i := 0; i < 5; i++ {
		doc, _, _ := craft.ParseFiles(files)
		got := doc.Domains[0].Name
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("non-deterministic merge order: %q vs %q", first, got)
		}
	}
	if first != "Alpha" {
		t.Errorf("expected Alpha first (sorted), got %q", first)
	}
}

func TestParseFiles_DiagnosticsCarrySourceURI(t *testing.T) {
	files := map[string][]byte{
		"broken.craft": []byte("use_case \"x\" {\n  when\n"),
	}
	_, diags, _ := craft.ParseFiles(files)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	for _, d := range diags {
		if d.SourceURI != "broken.craft" {
			t.Errorf("SourceURI = %q, want bare map key %q (no file:// prefix)", d.SourceURI, "broken.craft")
		}
	}
}

func TestParseFiles_Empty(t *testing.T) {
	doc, diags, err := craft.ParseFiles(map[string][]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("nil doc")
	}
	if diags == nil {
		t.Error("diags should be non-nil empty slice, not nil")
	}
}

func TestParseFiles_CrossFileUnresolvedRef(t *testing.T) {
	// Mirrors internal/sema/sema_test.go:TestAnalyzeWorkspace_UnresolvedContext,
	// but spread across two files (domain declared in a.craft, service
	// referencing it plus an unknown context in b.craft) to exercise
	// ParseFiles' cross-file workspace resolution end to end.
	files := map[string][]byte{
		"a.craft": []byte("domain Auth {\n  Login\n  Registration\n}\n"),
		"b.craft": []byte("services {\n  UserService {\n    contexts: Login, UnknownContext\n  }\n}\n"),
	}
	_, diags, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found *craft.Diagnostic
	for i := range diags {
		if diags[i].Code == "craft/sema/unresolved-reference" {
			found = &diags[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected craft/sema/unresolved-reference diagnostic, got: %+v", diags)
	}
	if found.SourceURI != "b.craft" {
		t.Errorf("SourceURI = %q, want %q", found.SourceURI, "b.craft")
	}
}
