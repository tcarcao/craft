package craft_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/pkg/craft"
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
	// DD2: no sourceURI here, matching TestHarnessA_V2vsGoldens' no-URI
	// ProjectFromTree call, so the 99_mixed golden carries no sourceUri.
	doc, _, _ := craft.Parse("", src)
	got := mustMarshalIndent(t, doc)
	want := strings.TrimRight(string(repoFile(t, "testdata/corpus/99_mixed/dsl-vnext.craftjson")), "\n")
	if strings.TrimRight(got, "\n") != want {
		t.Errorf("Parse doc != corpus golden:\n got: %s\nwant: %s", got, want)
	}
}

func TestParse_DiagnosticsCarryFilename(t *testing.T) {
	// Deprecated quoted refs (listens/notifies "X") produce sema diagnostics
	// that internally carry a file:// URI. Parse must normalize every
	// diagnostic's SourceURI to the exact filename argument — uniform with
	// ParseFiles, no file:// leak, never empty.
	src := []byte("use_case \"U\" {\n  when Billing listens \"SomethingHappened\"\n    Billing notifies \"Done\"\n}\n")
	_, diags, _ := craft.Parse("my/path/x.craft", src)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics (deprecated string refs)")
	}
	for _, d := range diags {
		if d.SourceURI != "my/path/x.craft" {
			t.Errorf("SourceURI = %q, want %q (no file:// prefix, not empty)", d.SourceURI, "my/path/x.craft")
		}
	}
}

func TestParse_UseCaseLineAndSourceURI(t *testing.T) {
	src := []byte("use_case \"First\" {\n  when A creates x\n}\n\nuse_case \"Second\" {\n  when A creates y\n}\n")
	doc, _, _ := craft.Parse("journeys/renewal.craft", src)
	if len(doc.UseCases) != 2 {
		t.Fatalf("want 2 use cases, got %d", len(doc.UseCases))
	}
	if doc.UseCases[0].Line != 1 {
		t.Errorf("UseCases[0].Line = %d, want 1", doc.UseCases[0].Line)
	}
	if doc.UseCases[1].Line != 5 {
		t.Errorf("UseCases[1].Line = %d, want 5", doc.UseCases[1].Line)
	}
	for _, uc := range doc.UseCases {
		if uc.SourceURI != "journeys/renewal.craft" {
			t.Errorf("UseCase %q SourceURI = %q, want journeys/renewal.craft", uc.Name, uc.SourceURI)
		}
	}
}

func TestParseFiles_UseCaseSourceURIIsMapKey(t *testing.T) {
	files := map[string][]byte{
		"a.craft": []byte("use_case \"UA\" {\n  when A creates x\n}\n"),
		"b.craft": []byte("use_case \"UB\" {\n  when A creates y\n}\n"),
	}
	doc, _, _ := craft.ParseFiles(files)
	got := map[string]string{}
	for _, uc := range doc.UseCases {
		got[uc.Name] = uc.SourceURI
	}
	if got["UA"] != "a.craft" || got["UB"] != "b.craft" {
		t.Errorf("SourceURI attribution wrong: %+v", got)
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
	count := 0
	for i := range diags {
		if diags[i].Code == "craft/sema/unresolved-reference" {
			found = &diags[i]
			count++
		}
	}
	if found == nil {
		t.Fatalf("expected craft/sema/unresolved-reference diagnostic, got: %+v", diags)
	}
	// Exactly one: Login must resolve via the domain declared in a.craft (proving
	// cross-file merge happened); only UnknownContext stays unresolved. If merge
	// were broken, Login would also be unresolved and count would be 2.
	if count != 1 {
		t.Fatalf("expected exactly 1 unresolved-reference (Login resolved cross-file), got %d: %+v", count, diags)
	}
	if found.SourceURI != "b.craft" {
		t.Errorf("SourceURI = %q, want %q", found.SourceURI, "b.craft")
	}
	if !strings.Contains(found.Message, "UnknownContext") {
		t.Errorf("unresolved-reference message = %q, want it to name UnknownContext", found.Message)
	}
}

// TestParse_UseCaseTags verifies that a use_case's tags { } sub-block
// (Slice B) projects into UseCase.Tags, with a bare ref-shaped value
// ("re/renewal-flow", spanning multiple lexer tokens) captured whole and a
// quoted value ("team billing") unquoted.
func TestParse_UseCaseTags(t *testing.T) {
	src := []byte("use_case \"Renewal\" {\n  tags {\n    journey: re/renewal-flow\n    owner: \"team billing\"\n  }\n\n  when Customer creates Account\n}\n")
	doc, diags, _ := craft.Parse("x.craft", src)
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Fatalf("unexpected error diag: %s %s", d.Code, d.Message)
		}
	}
	if len(doc.UseCases) != 1 {
		t.Fatalf("want 1 use case, got %d", len(doc.UseCases))
	}
	tags := doc.UseCases[0].Tags
	if tags["journey"] != "re/renewal-flow" {
		t.Errorf("tags[journey] = %q, want re/renewal-flow", tags["journey"])
	}
	if tags["owner"] != "team billing" {
		t.Errorf("tags[owner] = %q, want \"team billing\" (unquoted)", tags["owner"])
	}
}

// TestParse_UseCaseTags_LastWriteWins is Task 4's cheap future-proofing for
// a Task-3 minor: a repeated tag key projects last-write-wins (the second
// occurrence's value survives), matching sema's craft/sema/duplicate-tag
// WARNING (not error) for the same input — duplication is flagged, not
// rejected.
func TestParse_UseCaseTags_LastWriteWins(t *testing.T) {
	src := []byte("use_case \"Renewal\" {\n  tags {\n    journey: a\n    journey: b\n  }\n\n  when Customer creates Account\n}\n")
	doc, diags, _ := craft.Parse("x.craft", src)
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Fatalf("unexpected error diag: %s %s", d.Code, d.Message)
		}
	}
	if len(doc.UseCases) != 1 {
		t.Fatalf("want 1 use case, got %d", len(doc.UseCases))
	}
	if got := doc.UseCases[0].Tags["journey"]; got != "b" {
		t.Errorf("tags[journey] = %q, want %q (last-write-wins)", got, "b")
	}
}

// TestParse_NoTagsBlockLeavesTagsNil verifies that a use_case with no tags {
// } block leaves UseCase.Tags nil (not an empty map), so it's omitted from
// JSON output via the `omitempty` tag.
func TestParse_NoTagsBlockLeavesTagsNil(t *testing.T) {
	doc, _, _ := craft.Parse("x.craft", []byte("use_case \"U\" {\n  when A creates B\n}\n"))
	if doc.UseCases[0].Tags != nil {
		t.Errorf("Tags should be nil when no tags block, got %v", doc.UseCases[0].Tags)
	}
}

func TestParseFiles_WorkspaceDiagnosticOrderDeterministic(t *testing.T) {
	// Two files that each emit a workspace-level (cross-file resolution)
	// diagnostic. The sema workspace passes range over maps internally, so
	// without ParseFiles stabilizing each batch the relative order of these two
	// diagnostics varies across runs (Go randomizes map iteration). Run many
	// times and require a stable, filename-sorted order.
	files := map[string][]byte{
		"a.craft": []byte("services {\n  SvcA {\n    contexts: UnknownA\n  }\n}\n"),
		"b.craft": []byte("services {\n  SvcB {\n    contexts: UnknownB\n  }\n}\n"),
	}
	var want []string
	for i := 0; i < 20; i++ {
		_, diags, _ := craft.ParseFiles(files)
		var seq []string
		for _, d := range diags {
			if d.Code == "craft/sema/unresolved-reference" {
				seq = append(seq, d.SourceURI)
			}
		}
		if len(seq) < 2 {
			t.Fatalf("expected >=2 cross-file unresolved-reference diags, got %d: %+v", len(seq), diags)
		}
		if i == 0 {
			want = seq
			if seq[0] != "a.craft" {
				t.Errorf("workspace diagnostics not sorted by file: %v", seq)
			}
			continue
		}
		if len(seq) != len(want) {
			t.Fatalf("run %d: diag count changed: got %v, want %v", i, seq, want)
		}
		for j := range seq {
			if seq[j] != want[j] {
				t.Fatalf("run %d: non-deterministic workspace diagnostic order: got %v, want %v", i, seq, want)
			}
		}
	}
}

// TestParse_ContextMap_RelationshipEdge is the Task 1 TDD lock for the
// context_map redesign: edge verbs are now the 8 DDD strategic
// context-mapping patterns (not the old realized_by/also_realizes/same_as/
// contrasts/distinct_from), and endpoints are bare or domain-qualified
// bounded-context names with NO `bc:`/kind prefix required.
func TestParse_ContextMap_RelationshipEdge(t *testing.T) {
	doc, _, err := craft.Parse("f.craft", []byte("context_map {\n  billing customer_supplier vas\n  re/billing partnership payments/ledger\n}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.ContextMap) != 2 {
		t.Fatalf("want 2 edges, got %d: %+v", len(doc.ContextMap), doc.ContextMap)
	}
	e := doc.ContextMap[0]
	if e.Left != "billing" || e.Verb != "customer_supplier" || e.Right != "vas" {
		t.Errorf("edge0 = %+v, want {billing customer_supplier vas}", e)
	}
	if doc.ContextMap[1].Left != "re/billing" || doc.ContextMap[1].Right != "payments/ledger" {
		t.Errorf("edge1 = %+v, want qualified endpoints", doc.ContextMap[1])
	}
}

// TestParse_ContextMap_DomainScope is the Task 3 TDD lock for the optional
// domain scope on a context_map block (`context_map re { ... }`) and for
// context_map blocks being repeatable + merged: two blocks (one scoped, one
// unscoped) must contribute all of their edges to the single projected
// doc.ContextMap slice.
func TestParse_ContextMap_DomainScope(t *testing.T) {
	doc, _, err := craft.Parse("f.craft", []byte("context_map re {\n  billing customer_supplier vas\n}\ncontext_map {\n  re/billing partnership payments/ledger\n}\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.ContextMap) != 2 { // repeatable + merged
		t.Fatalf("want 2 merged edges, got %d", len(doc.ContextMap))
	}
}
