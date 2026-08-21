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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
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
	runFmt([]string{f}, false, fmtOverride{}, &out, &errOut)

	out.Reset()
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
		t.Fatalf("second pass exit code = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("second pass rewrote the file again, reported: %q", out.String())
	}
}

func TestRunFmt_CheckCleanFileExitsZero(t *testing.T) {
	f := writeCraft(t, t.TempDir(), "a.craft", formattedUseCase)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, true, fmtOverride{}, &out, &errOut); code != 0 {
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
	code := runFmt([]string{clean, dirty}, true, fmtOverride{}, &out, &errOut)
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
		code := runFmt([]string{f}, check, fmtOverride{}, &out, &errOut)
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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code == 0 {
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
	if code := runFmt([]string{missing}, false, fmtOverride{}, &out, &errOut); code == 0 {
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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
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
	runFmt(copied, false, fmtOverride{}, &out, &errOut)
	if strings.Contains(errOut.String(), "would have broken the file") {
		t.Errorf("formatter produced output that does not reparse:\n%s", errOut.String())
	}

	// Whatever was written must itself be a fixed point.
	out.Reset()
	errOut.Reset()
	if code := runFmt(copied, true, fmtOverride{}, &out, &errOut); code != 0 {
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

// TestFirstNewError_IgnoresPreExistingDespiteLineNumberShift is the reason
// diagKey folds digits out of the message rather than comparing it raw.
// craft/sema/duplicate-name embeds the FIRST occurrence's line number in its
// own message text, and formatting is free to shift that line (collapsing a
// run of blank lines above it to the one every top-level declaration pair
// gets, for instance): the error is the same error, but its message text
// differs between the pre-format and post-format parse. Without digit
// folding this reports a pre-existing error as new; this test is what pins
// that it must not.
func TestFirstNewError_IgnoresPreExistingDespiteLineNumberShift(t *testing.T) {
	orig := []craft.Diagnostic{{
		Code:     "craft/sema/duplicate-name",
		Message:  `actor "Alice" already declared (first seen at line 5)`,
		Severity: craft.SeverityError,
	}}
	formatted := []craft.Diagnostic{{
		Code:     "craft/sema/duplicate-name",
		Message:  `actor "Alice" already declared (first seen at line 3)`,
		Severity: craft.SeverityError,
	}}
	if got := firstNewError(formatted, orig); got != nil {
		t.Errorf("firstNewError() = %+v, want nil: same Code and message shape, differing only in a digit, must count as pre-existing", got)
	}
}

// TestFirstNewError_ReportsAGenuinelyNewOne is the other half: an
// error-severity diagnostic with no key match in orig must still be
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
// precedent for firstNewError. Digit-folding does not change this: a
// genuinely different diagnostic (a different duplicated name, a different
// code) still differs outside the digits it strips, so it still keys apart.
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

// TestFirstNewError_CountsRatherThanSetMembership pins the multiset fix: a
// key present once in orig must only absorb ONE matching occurrence in
// diags, not every occurrence. Under set membership a single pre-existing
// error would make an unbounded number of the "same" error in the output
// look pre-existing too, which is exactly the shape a formatter bug that
// duplicated an error-carrying construct would need to hide behind.
//
// This is a direct unit test of firstNewError, not an end-to-end one: the
// reviewer verified the multiplied-error shape is currently unreachable
// through craft fmt (contentDrift's whitespace-squashed byte-for-byte
// comparison means a walker bug duplicating a token cannot reach this guard,
// and no sema diagnostic here depends on whitespace), so there is no real
// .craft input to drive this through runFmt today.
func TestFirstNewError_CountsRatherThanSetMembership(t *testing.T) {
	one := craft.Diagnostic{
		Code:     "craft/sema/duplicate-name",
		Message:  `service "UserService" already declared (first seen at line 2)`,
		Severity: craft.SeverityError,
	}
	orig := []craft.Diagnostic{one}
	diags := []craft.Diagnostic{one, one, one}
	got := firstNewError(diags, orig)
	if got == nil {
		t.Fatal("firstNewError() = nil, want the second occurrence reported: orig only accounts for one")
	}
	if got.Code != one.Code || got.Message != one.Message {
		t.Errorf("firstNewError() = %+v, want a copy of %+v", *got, one)
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
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
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

// TestRunFmt_PreExistingErrorSurvivesALineShift is round 2's fix, pinned end
// to end. This is the exact input round 1's TestRunFmt_ReparseGuardStillFiresOnALineShift
// used, and that test asserted the guard's false refusal on it as CORRECT
// behaviour: "Zero" is followed by three blank lines before the first
// "Alice", so formatting collapses them to the one every top-level
// declaration pair gets, which moves the first "Alice" from line 5 to line
// 3. craft/sema/duplicate-name's message embeds that line number, so the
// pre-format and post-format messages differ in exactly one digit. Ruling
// (round 2): formatting introduced nothing here, so the file must be
// formatted and the pre-existing duplicate-name error left alone, the same
// as TestRunFmt_PreExistingErrorDoesNotBlockFormatting above but with the
// line-shift wrinkle that motivated diagKey's digit folding.
func TestRunFmt_PreExistingErrorSurvivesALineShift(t *testing.T) {
	const src = "actor user Zero\n\n\n\nactor user Alice\nactor user Alice\n"
	const want = "actor user Zero\n\nactor user Alice\n\nactor user Alice\n"
	f := writeCraft(t, t.TempDir(), "shift.craft", src)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{f}, false, fmtOverride{}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0 (a line-shifted pre-existing error must not block formatting)\nstderr: %s", code, errOut.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("a pre-existing error must not be reported as formatting damage, got stderr=%q", errOut.String())
	}
	got, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("re-reading %s: %v", f, err)
	}
	if string(got) != want {
		t.Errorf("file not formatted despite a pre-existing, line-shifted sema error\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestFmtHonoursCraftfmt is the CLI-level pin that a .craftfmt on disk
// actually changes what craft fmt writes, with no flag involved.
func TestFmtHonoursCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	src := filepath.Join(dir, "a.craft")
	os.WriteFile(src, []byte("services {\nFoo {\ncontexts: A\n}\n}\n"), 0o644)

	var out, errOut bytes.Buffer
	if code := runFmt([]string{src}, false, fmtOverride{}, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d, stderr=%s", code, errOut.String())
	}
	got, _ := os.ReadFile(src)
	want := "services {\n  Foo {\n    contexts: A\n  }\n}\n"
	if string(got) != want {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

// TestFmtFlagOverridesCraftfmt pins that an explicitly-set flag beats
// .craftfmt for the field it names, per the resolution order.
func TestFmtFlagOverridesCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	src := filepath.Join(dir, "a.craft")
	os.WriteFile(src, []byte("services {\nFoo {\ncontexts: A\n}\n}\n"), 0o644)

	override := fmtOverride{indent: 8, hasIndent: true}
	var out, errOut bytes.Buffer
	if code := runFmt([]string{src}, false, override, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d", code)
	}
	got, _ := os.ReadFile(src)
	// Indent is applied per nesting depth (cfg.Indent*depth, see
	// internal/lsp/formatsep.go), the same multiplicative model
	// TestFmtHonoursCraftfmt exercises for indent=2: contexts sits two
	// levels deep, so an Indent of 8 puts it at 16 spaces, not 8. A
	// .craftfmt of indent=2 would put it at 4, so 16 is unambiguous
	// evidence the flag (not .craftfmt) won.
	if !strings.Contains(string(got), "\n                contexts: A") {
		t.Errorf("flag did not override .craftfmt, got\n%q", got)
	}
}

// lineContaining returns the first line of content containing marker, or
// fails the test. It is used to inspect leading whitespace on a specific
// line without depending on the exact shape of the rest of the document.
func lineContaining(t *testing.T, content, marker string) string {
	t.Helper()
	for _, l := range strings.Split(content, "\n") {
		if strings.Contains(l, marker) {
			return l
		}
	}
	t.Fatalf("no line containing %q in\n%s", marker, content)
	return ""
}

// TestFmtFlagMergesPerFieldWithCraftfmt is the direct regression test for the
// review round 1 finding: a flag setting one field used to replace the
// ENTIRE resolved configuration, silently discarding any other field
// .craftfmt had set. Here .craftfmt sets continuation_indent, a field no
// flag names in this run, and the flag sets only indent, a different field.
// Both must take effect together, from their two different sources, on the
// same file: TestFmtFlagOverridesCraftfmt alone cannot catch this, because
// its .craftfmt only ever configures the same field the flag sets.
func TestFmtFlagMergesPerFieldWithCraftfmt(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("continuation_indent = 6\n"), 0o644)
	src := filepath.Join(dir, "a.craft")
	os.WriteFile(src, []byte("services {\nF {\ncontexts: A,\nB\n}\n}\n"), 0o644)

	override := fmtOverride{indent: 3, hasIndent: true}
	var out, errOut bytes.Buffer
	if code := runFmt([]string{src}, false, override, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d, stderr=%s", code, errOut.String())
	}
	got := string(mustReadFile(t, src))

	// F sits at depth 1: indent(3)*1 = 3 spaces. This field came from the
	// flag; .craftfmt never mentions indent, so this also confirms the flag
	// is not simply being ignored in favour of .craftfmt.
	if want := strings.Repeat(" ", 3) + "F {"; lineContaining(t, got, "F {") != want {
		t.Errorf("indent (from flag) not applied to %q, got %q, want %q", "F {", lineContaining(t, got, "F {"), want)
	}
	// contexts sits at depth 2: indent(3)*2 = 6 spaces.
	if want := strings.Repeat(" ", 6) + "contexts: A,"; lineContaining(t, got, "contexts: A,") != want {
		t.Errorf("indent (from flag) not applied to the nested line, got %q, want %q", lineContaining(t, got, "contexts: A,"), want)
	}
	// B is the wrapped continuation of contexts: indent(3)*depth(2) +
	// continuation_indent(6, from .craftfmt) = 12 spaces. Getting this right
	// requires continuation_indent to have survived the merge even though
	// only --indent was passed on the command line.
	if want := strings.Repeat(" ", 12) + "B"; lineContaining(t, got, "B") != want {
		t.Errorf("continuation_indent (from .craftfmt) was not preserved under an unrelated flag, got %q, want %q\nfull output:\n%s", lineContaining(t, got, "B"), want, got)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return b
}

// TestRunFmt_ResolvesPerFileAcrossWorkspaces is the direct pin for per-file
// resolution: a single runFmt call spanning two directories, each with its
// own DIFFERENT .craftfmt, must format each file according to its own
// directory's configuration, not one configuration for the whole batch.
func TestRunFmt_ResolvesPerFileAcrossWorkspaces(t *testing.T) {
	root := t.TempDir()
	wsA := filepath.Join(root, "wsA")
	wsB := filepath.Join(root, "wsB")
	if err := os.MkdirAll(wsA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsB, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(wsA, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	os.WriteFile(filepath.Join(wsB, ".craftfmt"), []byte("indent = 6\n"), 0o644)
	a := writeCraft(t, wsA, "a.craft", "services {\nFoo {\ncontexts: A\n}\n}\n")
	b := writeCraft(t, wsB, "b.craft", "services {\nBar {\ncontexts: B\n}\n}\n")

	var out, errOut bytes.Buffer
	if code := runFmt([]string{a, b}, false, fmtOverride{}, &out, &errOut); code != 0 {
		t.Fatalf("runFmt = %d, stderr=%s", code, errOut.String())
	}

	wantA := "services {\n" + strings.Repeat(" ", 2) + "Foo {\n" +
		strings.Repeat(" ", 4) + "contexts: A\n" +
		strings.Repeat(" ", 2) + "}\n}\n"
	if got := string(mustReadFile(t, a)); got != wantA {
		t.Errorf("wsA (indent=2): got\n%q\nwant\n%q", got, wantA)
	}

	wantB := "services {\n" + strings.Repeat(" ", 6) + "Bar {\n" +
		strings.Repeat(" ", 12) + "contexts: B\n" +
		strings.Repeat(" ", 6) + "}\n}\n"
	if got := string(mustReadFile(t, b)); got != wantB {
		t.Errorf("wsB (indent=6): got\n%q\nwant\n%q", got, wantB)
	}
}

// TestRunFmt_MalformedCraftfmtOnlyFailsThatWorkspace pins that a resolution
// failure is scoped to the file(s) under the bad .craftfmt: a good workspace
// in the same invocation must still be formatted and must not be reported as
// failed, even though the run as a whole exits non-zero.
func TestRunFmt_MalformedCraftfmtOnlyFailsThatWorkspace(t *testing.T) {
	root := t.TempDir()
	wsGood := filepath.Join(root, "good")
	wsBad := filepath.Join(root, "bad")
	if err := os.MkdirAll(wsGood, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wsBad, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(wsGood, ".craftfmt"), []byte("indent = 2\n"), 0o644)
	// "indnet" is a typo for "indent"; fmtconfig.Load rejects unknown keys.
	os.WriteFile(filepath.Join(wsBad, ".craftfmt"), []byte("indnet = 4\n"), 0o644)
	const badSrc = "services {\nBar {\ncontexts: B\n}\n}\n"
	good := writeCraft(t, wsGood, "a.craft", "services {\nFoo {\ncontexts: A\n}\n}\n")
	bad := writeCraft(t, wsBad, "b.craft", badSrc)

	var out, errOut bytes.Buffer
	code := runFmt([]string{good, bad}, false, fmtOverride{}, &out, &errOut)
	if code == 0 {
		t.Fatal("exit code = 0, want non-zero: a malformed .craftfmt must fail the run")
	}
	if !strings.Contains(errOut.String(), bad) || !strings.Contains(errOut.String(), "indnet") {
		t.Errorf("want the bad file and the offending key named on stderr, got %q", errOut.String())
	}
	if strings.Contains(errOut.String(), good) {
		t.Errorf("the good workspace's file must not be reported as failed, got %q", errOut.String())
	}

	wantGood := "services {\n" + strings.Repeat(" ", 2) + "Foo {\n" +
		strings.Repeat(" ", 4) + "contexts: A\n" +
		strings.Repeat(" ", 2) + "}\n}\n"
	if got := string(mustReadFile(t, good)); got != wantGood {
		t.Errorf("good workspace was not formatted despite the other workspace's bad .craftfmt\ngot\n%q\nwant\n%q", got, wantGood)
	}
	if got := string(mustReadFile(t, bad)); got != badSrc {
		t.Errorf("bad workspace's file was modified despite the resolution error:\n%q", got)
	}
}

// TestFmtCmd_FlagsWireThroughCobraToTheOutput is the direct regression test
// for two flag-wiring mutations that survive every other test in this file:
// every other override-merge test (TestFmtFlagOverridesCraftfmt,
// TestFmtFlagMergesPerFieldWithCraftfmt, ...) constructs fmtOverride{} by
// hand, so none of them exercises the `cmd.Flags().Changed(...)` gate in
// fmtCmd's RunE, or the assignment that copies each flag variable into the
// override struct. This test drives --indent and --continuation-indent
// through an actual cobra.Command via root.SetArgs, the same path
// `craft fmt` takes from a shell.
//
// It also passes a RELATIVE file path (just the basename, with the process
// cwd changed into the file's directory), which is the shape a real
// invocation like `craft fmt *.craft` from a workspace root actually takes.
// fmtconfig.Resolve's relative-path branch (filepath.Abs(dir) against the
// process cwd) otherwise has no automated coverage: every other test in this
// package passes an absolute path built from t.TempDir().
//
// Two mutations were verified against this test and reverted:
//   - `cmd.Flags().Changed("indent")` replaced with `indent != 0`: caught by
//     TestFmtCmd_IndentZeroFlagAppliesThroughCobra below, since a mutant gate
//     like that treats an explicit `--indent 0` as unset.
//   - `override.continuationIndent, override.hasContinuationIndent =
//     contIndent, true` replaced with `= indent, true`: caught here, because
//     --indent and --continuation-indent are given different values below.
func TestFmtCmd_FlagsWireThroughCobraToTheOutput(t *testing.T) {
	dir := t.TempDir()
	writeCraft(t, dir, "a.craft", "services {\nF {\ncontexts: A,\nB\n}\n}\n")
	t.Chdir(dir)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"fmt", "--indent", "2", "--continuation-indent", "8", "a.craft"})
	if err := root.Execute(); err != nil {
		t.Fatalf("fmt: %v\n%s", err, buf.String())
	}

	got, err := os.ReadFile("a.craft")
	if err != nil {
		t.Fatalf("reading a.craft: %v", err)
	}
	// F: indent(2) * depth(1) = 2 spaces.
	// contexts: indent(2) * depth(2) = 4 spaces.
	// B is the wrapped continuation of contexts: indent(2)*depth(2) +
	// continuation_indent(8) = 12 spaces. If continuation-indent's flag value
	// were silently replaced by indent's (2), this would be 4+2=6 instead.
	want := "services {\n" +
		"  F {\n" +
		"    contexts: A,\n" +
		"            B\n" +
		"  }\n" +
		"}\n"
	if string(got) != want {
		t.Errorf("flags did not wire through cobra as expected\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// TestFmtCmd_IndentZeroFlagAppliesThroughCobra pins that an explicit
// `--indent 0` is honoured, not treated as "flag not passed". `indent` is an
// int flag whose zero value coincides with a legal configuration value (see
// fmtconfig.Config.Validate, which accepts 0), so the RunE gate MUST be
// cmd.Flags().Changed("indent"), never a check on the flag variable's value
// such as `indent != 0`. A .craftfmt with a non-zero indent is present so
// that a mutant gate which treats --indent 0 as unset would visibly fall
// back to it instead of producing zero-indent output.
func TestFmtCmd_IndentZeroFlagAppliesThroughCobra(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".craftfmt"), []byte("indent = 4\n"), 0o644)
	writeCraft(t, dir, "a.craft", "services {\nF {\ncontexts: A\n}\n}\n")
	t.Chdir(dir)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"fmt", "--indent", "0", "a.craft"})
	if err := root.Execute(); err != nil {
		t.Fatalf("fmt: %v\n%s", err, buf.String())
	}

	got, err := os.ReadFile("a.craft")
	if err != nil {
		t.Fatalf("reading a.craft: %v", err)
	}
	want := "services {\nF {\ncontexts: A\n}\n}\n"
	if string(got) != want {
		t.Errorf("--indent 0 was not honoured (fell back to .craftfmt's indent=4?)\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// TestFmtCmd_InvalidFlagValueFailsBeforeTouchingFiles pins that a bad flag
// value is caught before any file is read or written: RunE validates the
// merged-onto-defaults override and returns the error itself, never reaching
// runFmt, let alone writing anything.
func TestFmtCmd_InvalidFlagValueFailsBeforeTouchingFiles(t *testing.T) {
	dir := t.TempDir()
	f := writeCraft(t, dir, "a.craft", unformattedUseCase)

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"fmt", "--align-trailing-comment", "sometimes", f})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error for an invalid --align-trailing-comment value")
	}
	if !strings.Contains(err.Error(), "trailing_comment") {
		t.Errorf("want a validation error mentioning trailing_comment, got %v", err)
	}
	got := string(mustReadFile(t, f))
	if got != unformattedUseCase {
		t.Errorf("file was modified despite an invalid flag value:\n%q", got)
	}
}
