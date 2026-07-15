package parser_diff_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// skippedSubdirs lists corpus subdirectories that are not yet implemented by
// the v2 parser. The `skipped` marker is the implementation-frontier ledger;
// it advances as each grammar slice lands. Per Q11, it may only move forward —
// never re-skip a directory that has been un-skipped on main.
var skippedSubdirs = map[string]bool{
	// 02_domains: un-skipped in S4
	// 03_services: un-skipped in S5
	// 04_use_cases: un-skipped in S6
	// 05_arch: un-skipped in S7
	// 06_exposures: un-skipped in S8
	// 99_mixed: un-skipped in S8
}

func TestHarnessA_V2vsGoldens(t *testing.T) {
	corpusRoot := "../../testdata/corpus"

	var craftFiles []string
	err := filepath.Walk(corpusRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			rel, _ := filepath.Rel(corpusRoot, path)
			if skippedSubdirs[rel] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".craft") {
			craftFiles = append(craftFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk corpus: %v", err)
	}

	ran := 0

	for _, craftFile := range craftFiles {
		craftFile := craftFile
		goldenFile := strings.TrimSuffix(craftFile, ".craft") + ".craftjson"

		t.Run(craftFile, func(t *testing.T) {
			if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
				t.Skipf("no golden for %s", craftFile)
				return
			}

			content, err := os.ReadFile(craftFile)
			if err != nil {
				t.Errorf("cannot read %s: %v", craftFile, err)
				return
			}

			greenRoot, li, parserDiags := syntax.Parse(string(content))
			if len(parserDiags) > 0 {
				for _, d := range parserDiags {
					if d.Severity == "error" {
						t.Errorf("unexpected parse error for %s: [%s] %s", craftFile, d.Code, d.Message)
					}
				}
			}

			tree := syntax.Root(greenRoot)
			doc := syntax.ProjectFromTree(tree, li)

			gotBytes, err := json.Marshal(doc)
			if err != nil {
				t.Errorf("json.Marshal failed for %s: %v", craftFile, err)
				return
			}

			wantBytes, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Errorf("cannot read golden %s: %v", goldenFile, err)
				return
			}

			var got interface{}
			if err := json.Unmarshal(gotBytes, &got); err != nil {
				t.Errorf("json.Unmarshal (got) failed for %s: %v", craftFile, err)
				return
			}

			var want interface{}
			if err := json.Unmarshal(wantBytes, &want); err != nil {
				t.Errorf("json.Unmarshal (want) failed for %s: %v", craftFile, err)
				return
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("v2 output does not match golden for %s\n--- got ---\n%s\n--- want ---\n%s",
					craftFile, gotBytes, wantBytes)
				return
			}

			ran++
		})
	}

	if ran == 0 {
		t.Fatalf("no corpus files exercised — check corpus root path or skippedSubdirs map")
	}
}
