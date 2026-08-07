package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

// repoRoot is this package's path to the top of the repository.
const repoRoot = "../.."

// skippedDirs are directories with no Craft sources of ours in them.
var skippedDirs = map[string]bool{
	".git":         true,
	".worktrees":   true,
	"node_modules": true,
	"vendor":       true,
}

// allCraftFiles walks the whole repository for .craft files.
//
// It walks rather than listing known directories on purpose. A hardcoded root
// list silently stops covering a directory the moment someone adds one, and it
// already had: cmd/server/test_actors.craft was outside the list, so the guard
// was checking 95 of the repository's 96 files and nobody could tell.
func allCraftFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skippedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".craft") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", repoRoot, err)
	}
	if len(files) == 0 {
		t.Fatal("found no .craft files; the walk is wrong and this test is asserting nothing")
	}
	sort.Strings(files)
	return files
}

// commentTexts returns every comment in src, in source order, read out of the
// TEXT rather than out of token kinds.
//
// Reading kinds is what let a deleted comment go unreported. The parser folds
// everything after the last real token into a single Whitespace token, so a
// comment on the final line of a file has no token carrying a comment kind:
// this function used to return nothing for it, before and after formatting
// alike, and dutifully reported no drift on a file whose comment had just been
// deleted. A guard that reads the same broken signal as the code it guards
// cannot fail.
func commentTexts(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for i := 0; i < len(src); {
		switch {
		case src[i] == '"':
			// Skip a string literal, so a `//` inside one is not a comment.
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' && i+1 < len(src) {
					i++
				}
				i++
			}
			i++
		case strings.HasPrefix(src[i:], "//"):
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				out = append(out, strings.TrimSpace(src[i:]))
				return out
			}
			out = append(out, strings.TrimSpace(src[i:i+end]))
			i += end
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				out = append(out, normalizeSpace(src[i:]))
				return out
			}
			out = append(out, normalizeSpace(src[i:i+2+end+2]))
			i += 2 + end + 2
		default:
			i++
		}
	}
	return out
}

// normalizeSpace collapses whitespace runs so a multi-line block comment
// compares equal after re-indentation.
func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// modelOf returns the canonical CraftDoc for src with every position-bearing
// field removed, as a generic JSON tree.
//
// Comparing the whole public model rather than a hand-written shape struct is
// deliberate: this test guards a class of defect, so it has to notice a field
// that a future accessor drops even though nobody thought to add that field to
// a fixture. Line and column numbers are stripped because reformatting is
// expected to move text; anything else changing is the formatter rewriting
// meaning.
func modelOf(t *testing.T, name, src string) any {
	t.Helper()
	doc, _, err := craft.Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshalling model for %s: %v", name, err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshalling model for %s: %v", name, err)
	}
	return stripPositions(tree)
}

// stripPositions removes position-bearing keys anywhere in a decoded JSON tree.
func stripPositions(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			switch k {
			case "line", "column", "col", "sourceUri", "range", "offset":
				continue
			}
			out[k] = stripPositions(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = stripPositions(val)
		}
		return out
	default:
		return v
	}
}

// TestFormatDocument_EveryCraftFileInRepo is the guard that makes `craft fmt`
// safe to ship as a bulk rewriter.
//
// TestFormatDocument_UseCaseRoundTrip asserts the same properties, but only
// over hand-written use_case fixtures. That is exactly why the context_map and
// glossary corruption survived: nothing formatted those blocks, so nothing
// noticed that the old renderer space-joined `billing/Invoice` into
// `billing / Invoice` and collapsed every relation onto one line. The same
// blind spot hid a `repo: olxeu/realestate/subscriptions` service field and the
// silent deletion of every comment in a document.
//
// Every .craft file in the repository is now checked for four properties:
//
//  1. the output reparses with zero error diagnostics
//  2. formatting is idempotent, which is byte-identity on already-formatted
//     input and the strongest form of it that can hold over a corpus whose
//     files are deliberately written in non-canonical shapes (4-space indent,
//     wrapped `contexts:` continuations) to exercise the parser
//  3. the canonical model is unchanged, which is what catches a renderer that
//     rewrites meaning rather than spelling
//  4. no comment is lost, which no model comparison can catch because comments
//     are trivia and never reach the model
//
// A file the parser cannot fully place is not formatted at all, and is
// asserted to come back byte-identical.
func TestFormatDocument_EveryCraftFileInRepo(t *testing.T) {
	for _, file := range allCraftFiles(t) {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			src := string(raw)

			got, blocked := FormatDocumentChecked(src)
			if blocked != nil {
				// contentDrift is not a licence to decline. Declining because
				// the parse was too broken to re-render is correct behaviour;
				// declining because the renderer would have changed content is
				// a formatter bug that this loop must not wave through. Both
				// return the input unchanged, so without naming the code the
				// assertions below are skipped and drift passes in silence.
				if blocked.Code == "craft/internal/formatter-content-drift" {
					t.Fatalf("contentDrift fired: the walker would have changed more than whitespace, so the token stream is losing content")
				}
				if got != src {
					t.Fatalf("a file the formatter declined to format must come back byte-identical\nblocked by: [%s] %s", blocked.Code, blocked.Message)
				}
				return
			}

			// 0. THE invariant: formatting changes whitespace and nothing
			// else. This one assertion subsumes the whole class and would
			// have caught every formatter defect found on this branch, so it
			// runs first and the rest are corroboration.
			if squashWhitespace(src) != squashWhitespace(got) {
				t.Errorf("formatting changed more than whitespace\nin:\n%s\nout:\n%s", src, got)
				return
			}

			// 1. reparses clean
			_, _, diags := syntax.Parse(got)
			for _, d := range diags {
				if d.Severity == craft.SeverityError {
					t.Errorf("formatted output does not parse: [%s] %s\n%s", d.Code, d.Message, got)
					return
				}
			}

			// 2. idempotent
			if again := FormatDocument(got); again != got {
				t.Errorf("formatting is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
			}

			// 3. model preserved
			before, after := modelOf(t, file, src), modelOf(t, file, got)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("formatting changed the canonical model\noutput:\n%s", got)
			}

			// 4. every comment survives
			wantComments := commentTexts(t, src)
			haveComments := commentTexts(t, got)
			if !reflect.DeepEqual(wantComments, haveComments) {
				t.Errorf("comments were lost or reordered\nwant %d: %q\nhave %d: %q",
					len(wantComments), wantComments, len(haveComments), haveComments)
			}
		})
	}
}

// TestFormatDocument_CanonicalCorpusIsByteIdentical asserts the literal
// byte-identity property for every file that is already in canonical form.
//
// It is separate from the guard above so that the count is visible: if a
// formatter change starts rewriting files it used to leave alone, this test
// names them. The corpus is not canonical as a whole and deliberately so, since
// its non-canonical shapes are parser test surface, so this cannot be asserted
// over every file without reformatting the corpus.
func TestFormatDocument_CanonicalCorpusIsByteIdentical(t *testing.T) {
	identical := 0
	for _, file := range allCraftFiles(t) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		src := string(raw)
		once := FormatDocument(src)
		if once == src {
			identical++
		}
		// Whatever the input looked like, its formatted form must be a fixed
		// point: formatting an already-formatted file changes nothing.
		if twice := FormatDocument(once); twice != once {
			t.Errorf("%s: formatted form is not a fixed point", file)
		}
	}
	if identical == 0 {
		t.Error("no file in the corpus is in canonical form; byte-identity is going untested")
	}
	t.Logf("%d file(s) already in canonical form and returned byte-identical", identical)
}
