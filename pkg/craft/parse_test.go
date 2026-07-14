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
