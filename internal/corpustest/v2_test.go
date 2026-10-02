package corpustest_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
)

var update = flag.Bool("update", false, "rewrite the corpus .craftjson goldens from the current output")

func TestHarnessA_V2vsGoldens(t *testing.T) {
	corpusRoot := "../../testdata/corpus"

	var craftFiles []string
	err := filepath.Walk(corpusRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".craft") {
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

			// Each corpus file is analyzed as a model of one file — exactly what
			// craft.Parse returns — so the goldens show the resolved document
			// consumers see. No file name, so they carry no sourceUri.
			file := analysis.File("", string(content))
			for _, d := range file.Diagnostics {
				if d.Severity == "error" && !strings.HasPrefix(d.Code, "craft/sema/") {
					t.Errorf("unexpected parse error for %s: [%s] %s", craftFile, d.Code, d.Message)
				}
			}
			doc := analysis.Workspace([]analysis.FileResult{file}).Doc

			if *update {
				out, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					t.Fatalf("json.MarshalIndent failed for %s: %v", craftFile, err)
				}
				if err := os.WriteFile(goldenFile, append(out, '\n'), 0o644); err != nil {
					t.Fatalf("writing golden %s: %v", goldenFile, err)
				}
			}

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
		t.Fatalf("no corpus files exercised — check the corpus root path")
	}
}
