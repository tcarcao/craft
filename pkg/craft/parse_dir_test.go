package craft_test

import (
	"testing"
	"testing/fstest"

	"github.com/tcarcao/craft/v2/pkg/craft"
)

// All ParseDir tests are backed by fstest.MapFS (never os.DirFS), which proves
// ParseDir is not accidentally coupled to OS filesystem paths — the crux of why
// it takes fs.FS rather than a root string (embed.FS / fs.Sub subtrees satisfy
// fs.FS but not filepath.WalkDir/os.ReadFile).

func TestParseDir_NoCraftFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"notes.txt":      {Data: []byte("not craft")},
		"docs/readme.md": {Data: []byte("# hi")},
	}
	doc, diags, err := craft.ParseDir(fsys, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("nil doc")
	}
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d: %+v", len(diags), diags)
	}
	if len(doc.Glossary) != 0 || len(doc.Services) != 0 || len(doc.ContextMap) != 0 {
		t.Errorf("expected empty doc, got %+v", doc)
	}
}

func TestParseDir_OnlyCraftFilesParsed(t *testing.T) {
	// b.txt LOOKS like craft but must be ignored purely because of its extension.
	fsys := fstest.MapFS{
		"a.craft": {Data: []byte("glossary { billing/Invoice same_as subscriptions/Invoice }\n")},
		"b.txt":   {Data: []byte("glossary { ignored/Term same_as other/Term }\n")},
	}
	doc, _, err := craft.ParseDir(fsys, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Glossary) != 1 {
		t.Fatalf("expected exactly 1 glossary relation (only a.craft), got %d: %+v", len(doc.Glossary), doc.Glossary)
	}
	if doc.Glossary[0].Left != "billing/Invoice" {
		t.Errorf("wrong relation parsed: %+v", doc.Glossary[0])
	}
}

func TestParseDir_RecursesIntoSubdirectories(t *testing.T) {
	// Files sit at depths 0..4 down two independent branches; top-level ReadDir
	// alone would miss all but the first. Finding every one proves fs.WalkDir
	// descends the full tree to arbitrary depth (no depth cap), and the buried
	// non-.craft file proves the extension filter still applies deep down.
	fsys := fstest.MapFS{
		"top.craft":                      {Data: []byte("glossary { a/L0 same_as b/L0 }\n")},
		"one/l1.craft":                   {Data: []byte("glossary { a/L1 same_as b/L1 }\n")},
		"one/two/l2.craft":               {Data: []byte("glossary { a/L2 same_as b/L2 }\n")},
		"one/two/three/l3.craft":         {Data: []byte("glossary { a/L3 same_as b/L3 }\n")},
		"alpha/beta/gamma/delta/z.craft": {Data: []byte("glossary { a/L4 same_as b/L4 }\n")},
		"one/two/three/notes.md":         {Data: []byte("ignore me")},
	}
	doc, _, err := craft.ParseDir(fsys, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Glossary) != 5 {
		t.Fatalf("expected 5 glossary relations across the nested tree, got %d: %+v", len(doc.Glossary), doc.Glossary)
	}
}

func TestParseDir_GlossaryAndContextMapTogether(t *testing.T) {
	// A directory whose glossary block lives in a different file from its other
	// declarations — regression coverage that ParseDir surfaces glossary relations
	// through the multi-file merge (locks in the mergeDoc fix under ParseDir).
	fsys := fstest.MapFS{
		"glossary.craft":    {Data: []byte("glossary { billing/Invoice same_as subscriptions/Invoice }\n")},
		"context_map.craft": {Data: []byte("context_map { billing customer_supplier subscriptions }\n")},
	}
	doc, _, err := craft.ParseDir(fsys, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Glossary) != 1 {
		t.Fatalf("glossary dropped from merged dir doc: %+v", doc.Glossary)
	}
	if len(doc.ContextMap) != 1 {
		t.Fatalf("context_map missing from merged dir doc: %+v", doc.ContextMap)
	}
}

// TestParseDir_MatchesParseFiles proves ParseDir changes only discovery
// ergonomics, not behavior: ParseDir(fsys, ".") is byte-identical to
// ParseFiles built from the same files under the same keys.
func TestParseDir_MatchesParseFiles(t *testing.T) {
	glos := []byte("glossary { billing/Invoice same_as subscriptions/Invoice }\n")
	cmap := []byte("context_map { billing customer_supplier subscriptions }\n")
	fsys := fstest.MapFS{
		"glossary.craft":         {Data: glos},
		"maps/context_map.craft": {Data: cmap},
	}
	dirDoc, dirDiags, err := craft.ParseDir(fsys, ".")
	if err != nil {
		t.Fatalf("ParseDir error: %v", err)
	}

	// fs.WalkDir yields the map keys verbatim, so ParseFiles with the same keys
	// must produce an identical result.
	filesDoc, filesDiags, err := craft.ParseFiles(map[string][]byte{
		"glossary.craft":         glos,
		"maps/context_map.craft": cmap,
	})
	if err != nil {
		t.Fatalf("ParseFiles error: %v", err)
	}

	if got, want := mustMarshalIndent(t, dirDoc), mustMarshalIndent(t, filesDoc); got != want {
		t.Errorf("doc mismatch:\nParseDir:\n%s\nParseFiles:\n%s", got, want)
	}
	if got, want := mustMarshalIndent(t, dirDiags), mustMarshalIndent(t, filesDiags); got != want {
		t.Errorf("diagnostics mismatch:\nParseDir:\n%s\nParseFiles:\n%s", got, want)
	}
}

// --- mergeDoc Glossary merge regression (Proposal 2) ---

// TestParseFiles_MergesGlossaryAcrossFiles fails before the mergeDoc fix: the
// glossary block lives in a different file from the context_map block, and the
// merged doc silently dropped its relations.
func TestParseFiles_MergesGlossaryAcrossFiles(t *testing.T) {
	doc, _, err := craft.ParseFiles(map[string][]byte{
		"glossary.craft":    []byte("glossary { billing/Invoice same_as subscriptions/Invoice }\n"),
		"context_map.craft": []byte("context_map { billing customer_supplier subscriptions }\n"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Glossary) != 1 {
		t.Fatalf("glossary relations dropped in multi-file merge, got %d: %+v", len(doc.Glossary), doc.Glossary)
	}
	r := doc.Glossary[0]
	if r.Left != "billing/Invoice" || r.Verb != "same_as" || r.Right != "subscriptions/Invoice" {
		t.Errorf("wrong merged relation: %+v", r)
	}
}

// TestParseFiles_MergesGlossaryFromBothFiles proves glossary relations from
// multiple files accumulate in file (map-key) order, matching ContextMap.
func TestParseFiles_MergesGlossaryFromBothFiles(t *testing.T) {
	doc, _, err := craft.ParseFiles(map[string][]byte{
		"a.craft": []byte("glossary { a/One same_as b/One }\n"),
		"b.craft": []byte("glossary { a/Two same_as b/Two }\n"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Glossary) != 2 {
		t.Fatalf("expected 2 merged relations, got %d: %+v", len(doc.Glossary), doc.Glossary)
	}
	// Keys sort ascending (a.craft before b.craft), so One precedes Two.
	if doc.Glossary[0].Left != "a/One" || doc.Glossary[1].Left != "a/Two" {
		t.Errorf("relations out of file order: %+v", doc.Glossary)
	}
}
