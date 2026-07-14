package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunValidate_ReportsDiagnostics(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "a.craft")
	// deprecated quoted event ref => a warning (craft/lint/deprecated-string-ref)
	src := []byte("use_case \"U\" {\n  when Billing listens \"SomethingHappened\"\n    Billing notifies \"Done\"\n}\n")
	os.WriteFile(f, src, 0644)

	b, _ := os.ReadFile(f)
	results := runValidate(map[string][]byte{f: b}, false)

	if len(results) == 0 {
		t.Fatal("expected at least one diagnostic result")
	}
	for _, r := range results {
		if r.File != f {
			t.Errorf("File = %q, want bare path %q (no file:// prefix)", r.File, f)
		}
	}
}
