package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/fmtconfig"
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

// assertWhitespaceOnlyChange asserts that got is src with only whitespace
// changed, checked byte-for-byte with all whitespace stripped from both
// sides. It reports whether the assertion held, so a caller that treats this
// as a prerequisite for the checks that follow can stop early exactly as
// TestFormatDocument_EveryCraftFileInRepo always has.
func assertWhitespaceOnlyChange(t *testing.T, src, got string) bool {
	t.Helper()
	if squashWhitespace(src) != squashWhitespace(got) {
		t.Errorf("formatting changed more than whitespace\nin:\n%s\nout:\n%s", src, got)
		return false
	}
	return true
}

// assertReparses asserts that got reparses with zero error diagnostics. It
// reports whether the assertion held, for the same early-stop reason as
// assertWhitespaceOnlyChange.
func assertReparses(t *testing.T, got string) bool {
	t.Helper()
	_, _, diags := syntax.Parse(got)
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("formatted output does not parse: [%s] %s\n%s", d.Code, d.Message, got)
			return false
		}
	}
	return true
}

// assertModelPreserved asserts that the canonical model of got matches the
// canonical model of src, position fields aside. name is passed through to
// modelOf for parse error messages; a file path is the natural choice.
func assertModelPreserved(t *testing.T, name, src, got string) {
	t.Helper()
	before, after := modelOf(t, name, src), modelOf(t, name, got)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("formatting changed the canonical model\noutput:\n%s", got)
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
//  0. formatting changes whitespace and nothing else, checked byte-for-byte
//     with all whitespace stripped from both sides. This is the one that
//     catches a lost comment: comments are non-whitespace content, so a
//     dropped one shows up here without needing a comment-shaped assertion of
//     its own, and it cannot be fooled by how a comment happens to be
//     tokenised (a trailing comment with nothing after it carries no comment
//     kind at all, since the parser folds it into the final whitespace token,
//     which is exactly what let it go unreported under an earlier version of
//     this guard that compared comments by filtering on token kind)
//  1. the output reparses with zero error diagnostics
//  2. formatting is idempotent, which is byte-identity on already-formatted
//     input and the strongest form of it that can hold over a corpus whose
//     files are deliberately written in non-canonical shapes (4-space indent,
//     wrapped `contexts:` continuations) to exercise the parser
//  3. the canonical model is unchanged, which is what catches a renderer that
//     rewrites meaning rather than spelling
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
			// runs first and the rest are corroboration. It is whitespace-blind
			// in one specific way worth knowing: squashWhitespace strips
			// whitespace inside a token's own text too, so it cannot see a
			// change to the spacing INSIDE a single comment (see
			// TestFormatDocument_CommentInternalSpacingSurvives).
			if !assertWhitespaceOnlyChange(t, src, got) {
				return
			}

			// 1. reparses clean
			if !assertReparses(t, got) {
				return
			}

			// 2. idempotent
			if again := FormatDocument(got); again != got {
				t.Errorf("formatting is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
			}

			// 3. model preserved
			assertModelPreserved(t, file, src, got)
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

// TestCorpusPropertiesAcrossConfigs asserts the same non-idempotence
// properties as TestFormatDocument_EveryCraftFileInRepo, but over the
// cross-product of indent width and trailing-comment alignment scope rather
// than at the default configuration alone.
//
// This is not redundant with the default-only guard. Continuation indent and
// comment alignment both run over the same lines, and a continuation line
// carries no comment cell: under ScopeStrict that breaks a comment run,
// under ScopeBlock it does not. A defect in that interaction only shows up
// at a specific (indent, scope) pair, and the default configuration is only
// one point in that space. Idempotence in particular is asserted at every
// point: a config under which formatting does not converge is exactly the
// kind of bug this test exists to catch, and it would pass a default-only
// idempotence check completely undetected.
//
// The matrix is 2 indents x 5 scopes = 10 configurations over the whole
// corpus (currently ~98 files), each doing two format passes. That ran in
// under two seconds locally alongside the rest of the package, so nothing is
// bounded or sampled: every file is checked under every configuration.
func TestCorpusPropertiesAcrossConfigs(t *testing.T) {
	files := allCraftFiles(t)

	var configs []fmtconfig.Config
	for _, indent := range []int{2, 4} {
		for _, scope := range []fmtconfig.Scope{
			fmtconfig.ScopeOff, fmtconfig.ScopeStrict,
			fmtconfig.ScopeBlock, fmtconfig.ScopeDecl, fmtconfig.ScopeFile,
		} {
			c := fmtconfig.Defaults()
			c.Indent = indent
			c.Align.TrailingComment = scope
			configs = append(configs, c)
		}
	}
	t.Logf("checking %d file(s) across %d configuration(s) (%d total cases)", len(files), len(configs), len(files)*len(configs))

	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		src := string(raw)
		for _, cfg := range configs {
			name := fmt.Sprintf("%s/indent=%d/scope=%s", filepath.Base(path), cfg.Indent, cfg.Align.TrailingComment)
			t.Run(name, func(t *testing.T) {
				// A handful of corpus files (testdata/broken/*) have syntax
				// errors severe enough that the parser cannot place them at
				// all, at any configuration; the formatter declines to
				// rewrite those rather than guess, and FormatDocumentWith
				// returns them unchanged. Checking such a file's declined
				// output for reparse-clean or idempotence would fail for a
				// reason that has nothing to do with indent or alignment
				// scope, so this mirrors
				// TestFormatDocument_EveryCraftFileInRepo's handling: confirm
				// the decline was a genuine "too broken to touch" and not the
				// formatter quietly dropping content, then move on.
				once, blocked := FormatDocumentCheckedWith(src, cfg)
				if blocked != nil {
					if blocked.Code == "craft/internal/formatter-content-drift" {
						t.Fatalf("contentDrift fired under %+v: the walker would have changed more than whitespace", cfg)
					}
					if once != src {
						t.Fatalf("a file the formatter declined to format must come back byte-identical under %+v\nblocked by: [%s] %s", cfg, blocked.Code, blocked.Message)
					}
					return
				}

				twice := FormatDocumentWith(once, cfg)
				if once != twice {
					t.Errorf("not idempotent under %+v\nfirst:\n%s\nsecond:\n%s", cfg, once, twice)
				}
				assertWhitespaceOnlyChange(t, src, once)
				assertReparses(t, once)
				assertModelPreserved(t, path, src, once)
			})
		}
	}
}
