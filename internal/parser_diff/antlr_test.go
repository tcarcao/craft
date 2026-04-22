package parser_diff_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	internalparser "github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/internal/parser_antlr_adapter"
)

func TestHarnessB_ANTLRvsGoldens(t *testing.T) {
	corpusRoot := "../../testdata/corpus"

	var craftFiles []string
	err := filepath.Walk(corpusRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip the 99_mixed directory — pending per-slice human review
		if info.IsDir() && strings.HasSuffix(filepath.ToSlash(path), "99_mixed") {
			return filepath.SkipDir
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
		craftFile := craftFile // capture loop variable
		goldenFile := strings.TrimSuffix(craftFile, ".craft") + ".craftjson"

		t.Run(craftFile, func(t *testing.T) {
			// If no golden exists, skip
			if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
				t.Skipf("no golden for %s", craftFile)
				return
			}

			content, err := os.ReadFile(craftFile)
			if err != nil {
				t.Errorf("cannot read %s: %v", craftFile, err)
				return
			}

			p := internalparser.NewParser()
			model, err := p.ParseString(string(content))
			if err != nil {
				t.Errorf("ANTLR parse failed for %s: %v", craftFile, err)
				return
			}

			doc := parser_antlr_adapter.FromDSLModel(model)

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
				t.Errorf("ANTLR output does not match golden for %s\n--- got ---\n%s\n--- want ---\n%s",
					craftFile, gotBytes, wantBytes)
				return
			}

			ran++
		})
	}

	if ran == 0 {
		t.Skipf("no corpus files exercised")
	}
}
