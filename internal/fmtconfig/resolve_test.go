package fmtconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := Resolve(filepath.Join(dir, "a.craft"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != Defaults() {
		t.Errorf("Resolve() = %+v, want Defaults()", got)
	}
}

func TestResolveFindsNearestAncestor(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ".craftfmt"), "indent = 2\n")
	write(t, filepath.Join(root, "a", ".craftfmt"), "indent = 8\n")

	got, err := Resolve(filepath.Join(nested, "x.craft"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Indent != 8 {
		t.Errorf("Indent = %d, want 8 (nearest ancestor wins)", got.Indent)
	}
	if got.ContinuationIndent != 4 {
		t.Errorf("ContinuationIndent = %d, want 4 (unset key keeps default)", got.ContinuationIndent)
	}
}

func TestResolveKeepsUntouchedAlignDefaults(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "[align]\ntrailing_comment = \"strict\"\n")

	got, err := Resolve(filepath.Join(dir, "x.craft"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Align.TrailingComment != ScopeStrict {
		t.Errorf("Align.TrailingComment = %q, want %q (set key took effect)", got.Align.TrailingComment, ScopeStrict)
	}
	want := Defaults()
	if got.Align.OpAnnotation != want.Align.OpAnnotation {
		t.Errorf("Align.OpAnnotation = %q, want %q (untouched sibling keeps default)", got.Align.OpAnnotation, want.Align.OpAnnotation)
	}
	if got.Align.OutlierRatio != want.Align.OutlierRatio {
		t.Errorf("Align.OutlierRatio = %v, want %v (untouched sibling keeps default)", got.Align.OutlierRatio, want.Align.OutlierRatio)
	}
	if got.Align.OutlierMin != want.Align.OutlierMin {
		t.Errorf("Align.OutlierMin = %d, want %d (untouched sibling keeps default)", got.Align.OutlierMin, want.Align.OutlierMin)
	}
	if got.Indent != want.Indent {
		t.Errorf("Indent = %d, want %d (untouched top-level field keeps default)", got.Indent, want.Indent)
	}
	if got.ContinuationIndent != want.ContinuationIndent {
		t.Errorf("ContinuationIndent = %d, want %d (untouched top-level field keeps default)", got.ContinuationIndent, want.ContinuationIndent)
	}
}

func TestResolveRejectsMalformedFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "indent = = 4\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want a decode error")
	}
}

func TestResolveRejectsInvalidValue(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "[align]\ntrailing_comment = \"sometimes\"\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want a validation error")
	}
}

func TestResolveRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".craftfmt"), "indnet = 4\n")
	if _, err := Resolve(filepath.Join(dir, "x.craft")); err == nil {
		t.Fatal("Resolve() = nil error, want an unknown-key error; a typo must not silently do nothing")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
