package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const unformattedUseCase = "use_case \"Retry\" {\n" +
	"  when CRON detects a failed charge\n" +
	"    Subscriptions asks Billing for a fresh charge attempt [POST /v1/charges]\n" +
	"    Billing asks Gateway to authorize the card [POST /pay/v2/authorize]\n" +
	"}\n"

const formattedUseCase = "use_case \"Retry\" {\n" +
	"  when CRON detects a failed charge\n" +
	"    Subscriptions asks Billing for a fresh charge attempt  [POST /v1/charges]\n" +
	"    Billing asks Gateway to authorize the card             [POST /pay/v2/authorize]\n" +
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
