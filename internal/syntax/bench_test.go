package syntax_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// corpusFiles returns all .craft files under testdata/corpus, walking from
// the repo root (two directories above the package).
func corpusFiles(tb testing.TB) []struct{ path, src string } {
	tb.Helper()
	root := "../../testdata/corpus"
	var files []struct{ path, src string }
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".craft") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, struct{ path, src string }{path: path, src: string(data)})
		return nil
	})
	if err != nil {
		tb.Fatalf("walking corpus: %v", err)
	}
	if len(files) == 0 {
		tb.Fatal("no .craft files found in corpus")
	}
	return files
}

// BenchmarkParse_FullCorpus_Cold measures cold-parse throughput: each
// iteration reads a fresh copy of each file and parses from scratch with no
// cached state. Target: entire corpus in <50ms.
func BenchmarkParse_FullCorpus_Cold(b *testing.B) {
	root := "../../testdata/corpus"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".craft") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			syntax.Parse(string(data))
			return nil
		})
		if err != nil {
			b.Fatalf("walking corpus: %v", err)
		}
	}
}

// BenchmarkParse_FullCorpus_Warm measures warm-parse throughput: all source
// strings are pre-loaded into memory before the timer starts, isolating
// parse time from I/O. Target: entire corpus in <5ms.
func BenchmarkParse_FullCorpus_Warm(b *testing.B) {
	files := corpusFiles(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, f := range files {
			syntax.Parse(f.src)
		}
	}
}

// BenchmarkParse_SingleFile_Warm benchmarks a representative mixed file
// (user-management.craft) in isolation for per-file latency analysis.
func BenchmarkParse_SingleFile_Warm(b *testing.B) {
	data, err := os.ReadFile("../../testdata/corpus/99_mixed/user-management.craft")
	if err != nil {
		b.Fatalf("reading reference file: %v", err)
	}
	src := string(data)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		syntax.Parse(src)
	}
}
