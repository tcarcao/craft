package main

import (
	"os"
	"path/filepath"
	"strings"
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

// A `when` block stranded outside any use_case is discarded in full: the
// scenario, its actions and its participants never reach the model, and every
// consumer behaves as though the lines were never written. That used to be a
// warning, so `validate` exited 0 and CI counted the file as sound unless the
// user opted into --strict, which also promotes every lint finding.
//
// validate has no exit-code logic of its own beyond severity, so asserting the
// severity here is asserting the exit code.
func TestRunValidate_StrandedWhenBlockFailsWithoutStrict(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "stray-brace.craft")
	src := []byte("use_case \"Settle a seller invoice\" {\n" +
		"    when Seller opens the payments page\n" +
		"        Invoicing charges the card\n" +
		"}\n" +
		"\n" +
		"    when Seller opens the archive page\n" +
		"        Invoicing lists the archived positions\n" +
		"}\n")
	os.WriteFile(f, src, 0644)

	b, _ := os.ReadFile(f)
	results := runValidate(map[string][]byte{f: b}, false)

	var found bool
	for _, r := range results {
		if r.Severity != "error" {
			continue
		}
		if strings.Contains(r.Message, "not part of the model") {
			found = true
			if r.Line != 6 {
				t.Errorf("Line = %d, want 6 (the stranded `when`)", r.Line)
			}
			if !strings.Contains(r.Message, "lines 6-8") {
				t.Errorf("message must report the extent of the discard, got %q", r.Message)
			}
		}
	}
	if !found {
		t.Errorf("no error-severity diagnostic for the discarded block; got %+v", results)
	}
}
