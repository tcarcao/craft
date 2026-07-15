package parser_diff_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/sema"
	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// diagGolden is the subset of craft.Diagnostic that goldens assert on.
// Per Q19: code + range + severity only; message is free-form and NOT asserted.
type diagGolden struct {
	Code     string      `json:"code"`
	Severity string      `json:"severity"`
	Range    craft.Range `json:"range"`
}

// TestHarnessC_DiagnosticsVsGoldens is the third harness defined in Q13.
// It walks testdata/broken/, runs the v2 parser + sema on each .craft file,
// and asserts set-equality of (code, range, severity) tuples against the
// paired .diagnostics.json golden. Per Q19, message text is not asserted.
func TestHarnessC_DiagnosticsVsGoldens(t *testing.T) {
	brokenRoot := "../../testdata/broken"

	entries, err := os.ReadDir(brokenRoot)
	if err != nil {
		t.Fatalf("cannot read broken corpus root %s: %v", brokenRoot, err)
	}

	ran := 0

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".craft") {
			continue
		}

		craftFile := filepath.Join(brokenRoot, e.Name())
		goldenFile := strings.TrimSuffix(craftFile, ".craft") + ".diagnostics.json"

		if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
			t.Logf("skipping %s — no .diagnostics.json golden", e.Name())
			continue
		}

		name := e.Name()
		t.Run(name, func(t *testing.T) {
			content, err := os.ReadFile(craftFile)
			if err != nil {
				t.Fatalf("cannot read %s: %v", craftFile, err)
			}

			// Run the v2 parser.
			greenRoot, li, syntaxDiags := syntax.Parse(string(content))
			tree := syntax.Root(greenRoot)

			// Run single-file sema analysis.
			uri := "file://" + craftFile
			syms, semaDiags := sema.AnalyzeFile(uri, tree, li)

			// Run workspace-level analysis treating this as a single-file workspace.
			// This surfaces unresolved-reference and cross-file diagnostics even for
			// standalone broken files.
			perFile := map[string]sema.Symbols{uri: syms}
			ws, mergeDiags := sema.MergeWorkspaceSymbols(perFile)
			_, resolveDiags := sema.AnalyzeWorkspace(perFile, ws)

			// Collect all diagnostics scoped to this file.
			all := make([]craft.Diagnostic, 0, len(syntaxDiags)+len(semaDiags)+len(mergeDiags)+len(resolveDiags))
			all = append(all, syntaxDiags...)
			all = append(all, semaDiags...)
			for _, d := range mergeDiags {
				if d.SourceURI == uri || d.SourceURI == "" {
					all = append(all, d)
				}
			}
			for _, d := range resolveDiags {
				if d.SourceURI == uri || d.SourceURI == "" {
					all = append(all, d)
				}
			}

			// Convert to golden tuples (drop message + sourceUri per Q19).
			got := make([]diagGolden, len(all))
			for i, d := range all {
				got[i] = diagGolden{Code: d.Code, Severity: string(d.Severity), Range: d.Range}
			}

			// Load the golden.
			goldenBytes, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("cannot read golden %s: %v", goldenFile, err)
			}
			var want []diagGolden
			if err := json.Unmarshal(goldenBytes, &want); err != nil {
				t.Fatalf("cannot parse golden %s: %v", goldenFile, err)
			}

			// Set-equality comparison (order-independent, per Q13).
			if !diagSetEqual(got, want) {
				t.Errorf("diagnostic mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
					craftFile, marshalDiagPretty(got), marshalDiagPretty(want))
			}

			ran++
		})
	}

	if ran == 0 {
		t.Fatalf("no broken-corpus files exercised — check broken root path")
	}
}

// diagSetEqual returns true if got and want contain the same set of tuples
// (order-insensitive).
func diagSetEqual(got, want []diagGolden) bool {
	if len(got) != len(want) {
		return false
	}
	sortDiags(got)
	sortDiags(want)
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// sortDiags sorts a slice of diagGolden deterministically by (code, line, char).
func sortDiags(ds []diagGolden) {
	sort.Slice(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		return a.Range.Start.Character < b.Range.Start.Character
	})
}

func marshalDiagPretty(ds []diagGolden) string {
	b, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}
