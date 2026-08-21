package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/pkg/craft"
)

const unformattedUseCase = "use_case \"Retry\" {\n" +
	"  when CRON detects a failed charge\n" +
	"    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]\n" +
	"    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]\n" +
	"}\n"

const formattedUseCase = "use_case \"Retry\" {\n" +
	"    when CRON detects a failed charge\n" +
	"        Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]\n" +
	"        Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]\n" +
	"}\n"

func writeCraft(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestRunFmt_FormatsInPlace(t *testing.T) {
	f := writeCraft(t, t.TempDir(), "a.craft", unformattedUseCase)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, errOut.String())
	}

	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("re-reading %s: %v", f, err)
	}
	if string(got) != formattedUseCase {
		t.Errorf("file not formatted in place\nwant:\n%s\ngot:\n%s", formattedUseCase, got)
	}
	if !strings.Contains(out.String(), f) {
		t.Errorf("expected the rewritten file to be named on stdout, got %q", out.String())
	}
}

func TestRunFmt_PreservesFileMode(t *testing.T) {
	f := writeCraft(t, t.TempDir(), "a.craft", unformattedUseCase)
	if err := os.Chmod(f, 0o640); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, errOut.String())
	}

	info, err := os.Stat(f)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("mode = %04o, want 0640 (formatting must not widen permissions)", got)
	}
}

func TestRunFmt_IsIdempotent(t *testing.T) {
	f := writeCraft(t, t.TempDir(), "a.craft", unformattedUseCase)

	var out, errOut bytes.Buffer
	runFmt([]string{f}, false, &out, &errOut)

	out.Reset()
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("second pass exit code = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("second pass rewrote the file again, reported: %q", out.String())
	}
}

func TestRunFmt_CheckCleanFileExitsZero(t *testing.T) {
	f := writeCraft(t, t.TempDir(), "a.craft", formattedUseCase)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, true, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s\nstderr: %s", code, out.String(), errOut.String())
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("--check on formatted input must be silent, got stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestRunFmt_CheckUnformattedFileFailsAndNamesIt(t *testing.T) {
	dir := t.TempDir()
	clean := writeCraft(t, dir, "clean.craft", formattedUseCase)
	dirty := writeCraft(t, dir, "dirty.craft", unformattedUseCase)

	var out, errOut bytes.Buffer
	code := runFmt([]string{clean, dirty}, true, &out, &errOut)
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero\nstdout: %s", out.String())
	}
	if !strings.Contains(out.String(), dirty) {
		t.Errorf("--check must name the unformatted file, got %q", out.String())
	}
	if strings.Contains(out.String(), clean) {
		t.Errorf("--check must not name the already-formatted file, got %q", out.String())
	}

	// --check writes nothing.
	got, _ := os.ReadFile(dirty)
	if string(got) != unformattedUseCase {
		t.Errorf("--check modified the file on disk:\n%s", got)
	}
}

func TestRunFmt_UnparseableFileIsReportedAsSkipped(t *testing.T) {
	// An unclosed brace is an error-severity diagnostic, so FormatDocument
	// declines to rewrite it and returns the input verbatim. Reporting it as
	// "already formatted" would be a lie in both modes.
	const broken = "service Foo {\n  language: golang\n"
	f := writeCraft(t, t.TempDir(), "broken.craft", broken)

	for _, check := range []bool{false, true} {
		var out, errOut bytes.Buffer
		code := runFmt([]string{f}, check, &out, &errOut)
		if code == 0 {
			t.Errorf("check=%v: exit code = 0, want non-zero", check)
		}
		if !strings.Contains(errOut.String(), f) || !strings.Contains(errOut.String(), "skipped") {
			t.Errorf("check=%v: want a skipped report naming %s, got %q", check, f, errOut.String())
		}
		got, _ := os.ReadFile(f)
		if string(got) != broken {
			t.Errorf("check=%v: an unformattable file must be left byte-identical:\n%s", check, got)
		}
	}
}

// A phrase brace leaves the parser unable to place the trailing bracket. That
// is only a warning, but the tree is incomplete, so fmt must skip the file
// rather than rewrite it into `charge {amount  [ POST / pay ]`.
func TestRunFmt_IncompleteParseIsReportedAsSkipped(t *testing.T) {
	const src = "use_case \"X\" {\n  when U does x\n    Billing asks Gateway to charge {amount} [POST /pay]\n}\n"
	f := writeCraft(t, t.TempDir(), "phrase-brace.craft", src)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code == 0 {
		t.Fatalf("exit code = 0, want non-zero\nstderr: %s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "not-yet-implemented") {
		t.Errorf("want the blocking diagnostic code in the report, got %q", errOut.String())
	}
	got, _ := os.ReadFile(f)
	if string(got) != src {
		t.Errorf("file must be left byte-identical:\n%s", got)
	}
}

func TestRunFmt_MissingFileIsReported(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.craft")

	var out, errOut bytes.Buffer
	if code := runFmt([]string{missing}, false, &out, &errOut); code == 0 {
		t.Fatal("exit code = 0, want non-zero for a missing file")
	}
	if !strings.Contains(errOut.String(), missing) {
		t.Errorf("want the missing file named on stderr, got %q", errOut.String())
	}
}

// The command wiring: registered on the root, globs expand, and a directory
// argument gets a pointed error instead of resolveFiles silently dropping it.
func TestFmtCmd_GlobExpands(t *testing.T) {
	dir := t.TempDir()
	writeCraft(t, dir, "a.craft", unformattedUseCase)
	writeCraft(t, dir, "b.craft", unformattedUseCase)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"fmt", filepath.Join(dir, "*.craft")})
	if err := root.Execute(); err != nil {
		t.Fatalf("fmt glob: %v\n%s", err, buf.String())
	}

	for _, name := range []string{"a.craft", "b.craft"} {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		if string(got) != formattedUseCase {
			t.Errorf("%s not formatted:\n%s", name, got)
		}
	}
}

func TestFmtCmd_DirectoryArgumentIsRejected(t *testing.T) {
	dir := t.TempDir()
	writeCraft(t, dir, "a.craft", unformattedUseCase)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"fmt", dir})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for a directory argument")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("want a directory-specific error, got %v", err)
	}
}

func TestFmtCmd_IsRegistered(t *testing.T) {
	for _, c := range newRootCmd().Commands() {
		if c.Name() == "fmt" {
			return
		}
	}
	t.Error("fmt is not registered on the root command")
}

// TestRunFmt_TrailingCommentIsNotDeleted is the shape the coordinator measured
// on the built binary: a file ending in a comment went from 43 bytes to 24,
// with the comment gone and exit 0. Content loss with a success exit code is
// the worst failure an in-place bulk rewriter can have.
func TestRunFmt_TrailingCommentIsNotDeleted(t *testing.T) {
	const src = "actor user Alice\n\n// trailing note\n"
	f := writeCraft(t, t.TempDir(), "a.craft", src)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, errOut.String())
	}
	got, _ := os.ReadFile(f)
	if !strings.Contains(string(got), "// trailing note") {
		t.Errorf("the trailing comment was deleted:\n%q", got)
	}
	if len(got) < len(src) {
		t.Errorf("file shrank from %d to %d bytes:\n%q", len(src), len(got), got)
	}
}

// TestRunFmt_CommentOnlyFileIsNotTruncated covers the second measured case: an
// 18-byte comment-only file was rewritten to 0 bytes, exit 0.
func TestRunFmt_CommentOnlyFileIsNotTruncated(t *testing.T) {
	const src = "// only a comment\n"
	f := writeCraft(t, t.TempDir(), "b.craft", src)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, errOut.String())
	}
	got, _ := os.ReadFile(f)
	if len(got) == 0 {
		t.Fatal("file was truncated to zero bytes")
	}
	if string(got) != src {
		t.Errorf("comment-only file must round-trip\nwant: %q\ngot:  %q", src, got)
	}
}

// TestRunFmt_NeverWritesUnverifiedOutput asserts the belt-and-braces reparse:
// whatever the formatter returns is read back before it reaches the disk. The
// formatter's own content-drift check should mean this never fires, which is
// exactly why it needs its own test rather than waiting for a bug to exercise
// it. Every corpus file must survive a full format-and-reparse round trip.
func TestRunFmt_NeverWritesUnverifiedOutput(t *testing.T) {
	dir := t.TempDir()
	matches, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "*", "*.craft"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no corpus files found: %v", err)
	}

	copied := make([]string, 0, len(matches))
	for i, src := range matches {
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("reading %s: %v", src, err)
		}
		copied = append(copied, writeCraft(t, dir, fmt.Sprintf("f%03d.craft", i), string(raw)))
	}

	var out, errOut bytes.Buffer
	runFmt(copied, false, &out, &errOut)
	if strings.Contains(errOut.String(), "would have broken the file") {
		t.Errorf("formatter produced output that does not reparse:\n%s", errOut.String())
	}

	// Whatever was written must itself be a fixed point.
	out.Reset()
	errOut.Reset()
	if code := runFmt(copied, true, &out, &errOut); code != 0 {
		t.Errorf("formatted corpus is not a fixed point under --check\nstdout: %s\nstderr: %s",
			out.String(), errOut.String())
	}
}

// TestFirstNewError_IgnoresPreExisting is the unit-level guarantee behind the
// reparse guard: an error present in orig, with the same Code and Message,
// must not be reported as something formatting introduced.
func TestFirstNewError_IgnoresPreExisting(t *testing.T) {
	dup := craft.Diagnostic{
		Code:     "craft/sema/duplicate-name",
		Message:  `service "UserService" already declared (first seen at line 2)`,
		Severity: craft.SeverityError,
	}
	orig := []craft.Diagnostic{dup}
	formatted := []craft.Diagnostic{dup}
	if got := firstNewError(formatted, orig); got != nil {
		t.Errorf("firstNewError() = %+v, want nil: identical Code+Message must count as pre-existing", got)
	}
}

// TestFirstNewError_ReportsAGenuinelyNewOne is the other half: an
// error-severity diagnostic with no Code+Message match in orig must still be
// reported. This is exercised directly on firstNewError with a synthetic
// diagnostic rather than through a full craft-fmt round trip.
//
// I could not construct a real .craft input where FORMATTING a file that
// parses clean introduces a brand new semantic error: the formatter's own
// token-stream invariant and its contentDrift check together guarantee that
// only whitespace changes, and every sema/syntax diagnostic this codebase
// raises depends on token content or structure, never on whitespace. That is
// the same situation contentDrift itself documents ("No known real input
// reaches it anymore") and the same reason TestContentDrift_RefusesToLoseContent
// exercises it directly rather than end to end. This test follows that
// precedent for firstNewError.
//
// TestRunFmt_ReparseGuardStillFiresOnALineShift below exercises the guard
// end to end on a real, honestly-constructed input, but that input trips the
// guard via the DOCUMENTED conservative edge case (a pre-existing error's own
// message text embeds a line number that formatting's blank-line
// normalisation shifts), not via a genuinely new defect. Both are covered so
// this is not a gap, just not the same claim.
func TestFirstNewError_ReportsAGenuinelyNewOne(t *testing.T) {
	orig := []craft.Diagnostic{}
	brandNew := craft.Diagnostic{
		Code:     "craft/sema/duplicate-name",
		Message:  `actor "Alice" already declared (first seen at line 1)`,
		Severity: craft.SeverityError,
	}
	formatted := []craft.Diagnostic{brandNew}
	got := firstNewError(formatted, orig)
	if got == nil {
		t.Fatal("firstNewError() = nil, want the new diagnostic")
	}
	if got.Code != brandNew.Code || got.Message != brandNew.Message {
		t.Errorf("firstNewError() = %+v, want %+v", *got, brandNew)
	}
}

// TestRunFmt_PreExistingErrorDoesNotBlockFormatting is the end-to-end case
// that was silently wrong before this fix: a file with a genuine, pre-
// existing sema error that is NOT already in canonical form must still be
// formatted (its whitespace normalised), with the pre-existing error left
// alone rather than reported as formatting damage. UserService is declared
// twice on purpose, in two non-overlapping services blocks: this is the same
// shape as testdata/corpus/03_services/service_merging.craft, which is a
// genuinely invalid document under the current duplicate-name rule (not a
// bug in that fixture; see task-3-report.md round 1) and is exactly the case
// that used to make craft fmt refuse to ever format it.
func TestRunFmt_PreExistingErrorDoesNotBlockFormatting(t *testing.T) {
	const src = "services {\n  UserService {\n    contexts: A\n  }\n}\n\nservices {\n  UserService {\n    contexts: B\n  }\n}\n"
	const want = "services {\n    UserService {\n        contexts: A\n    }\n}\n\nservices {\n    UserService {\n        contexts: B\n    }\n}\n"

	f := writeCraft(t, t.TempDir(), "dup.craft", src)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0 (a pre-existing error must not block formatting)\nstderr: %s", code, errOut.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("a pre-existing error must not be reported as formatting damage, got stderr=%q", errOut.String())
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("re-reading %s: %v", f, err)
	}
	if string(got) != want {
		t.Errorf("file not formatted despite a pre-existing sema error\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestRunFmt_ReparseGuardStillFiresOnALineShift is the safety-net check: the
// guard must still refuse a file when the reformatted content's diagnostics
// do not match the original's. This is a real, non-fabricated input, not a
// forced formatter bug (see the note on TestFirstNewError_ReportsAGenuinelyNewOne
// for why a formatter-bug trigger could not be constructed): "Zero" is
// followed by three blank lines before the first "Alice", so the author's
// spacing is not already canonical, and formatting collapses those three
// blank lines to the one every top-level declaration pair gets. That moves
// the first "Alice" from line 5 to line 3, and craft/sema/duplicate-name's
// own message embeds the first occurrence's line number, so the formatted
// diagnostic's message no longer matches the original's byte for byte. The
// guard's Code+Message identity is deliberately conservative (see the
// firstNewError doc comment in fmt.go), and this is that conservatism
// firing on a real input, not a bug in the guard.
func TestRunFmt_ReparseGuardStillFiresOnALineShift(t *testing.T) {
	const src = "actor user Zero\n\n\n\nactor user Alice\nactor user Alice\n"
	f := writeCraft(t, t.TempDir(), "shift.craft", src)

	var out, errOut bytes.Buffer
	code := runFmt([]string{f}, false, &out, &errOut)
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero: the guard should have fired\nstdout: %s", out.String())
	}
	if !strings.Contains(errOut.String(), "would have broken the file") {
		t.Errorf("want the reparse-guard message, got %q", errOut.String())
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("re-reading %s: %v", f, err)
	}
	if string(got) != src {
		t.Errorf("a file the guard refused must be left byte-identical\nwant:\n%s\ngot:\n%s", src, got)
	}
}
