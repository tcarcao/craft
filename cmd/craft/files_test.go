package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const dirTestSrc = "actor user Foo\n\ndomain D {\n  Ctx\n}\n\nservices {\n  S {\n    contexts: Ctx\n  }\n}\n"

func mkTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, rel := range []string{"z.craft", "a/b.craft", "a/a.craft", "a/notes.md", "m/c.craft"} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(dirTestSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestResolveFiles_DirectoryExpandsSorted(t *testing.T) {
	dir := mkTree(t)
	got, err := resolveFiles([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(dir, "a", "a.craft"),
		filepath.Join(dir, "a", "b.craft"),
		filepath.Join(dir, "m", "c.craft"),
		filepath.Join(dir, "z.craft"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestResolveFiles_EmptyDirectoryIsError(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveFiles([]string{dir})
	if err == nil || !strings.Contains(err.Error(), "no .craft files") {
		t.Fatalf("want a 'no .craft files' error, got %v", err)
	}
}

func TestGenerateCmd_DirectoryArgument(t *testing.T) {
	dir := mkTree(t)
	out := t.TempDir()
	root := newRootCmd()
	root.SetArgs([]string{"generate", dir, "--type", "c4", "--output", out})
	if err := root.Execute(); err != nil {
		t.Fatalf("generate dir: %v", err)
	}
}

func TestGenerateCmd_EmptyDirectoryErrors(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"generate", t.TempDir()})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no .craft files") {
		t.Fatalf("want 'no .craft files' error, got %v", err)
	}
}

func TestValidateAndInspect_EmptyDirectoryErrors(t *testing.T) {
	for _, cmd := range []string{"validate", "inspect"} {
		root := newRootCmd()
		root.SetArgs([]string{cmd, t.TempDir()})
		if err := root.Execute(); err == nil {
			t.Errorf("%s on empty dir: want error", cmd)
		}
	}
}

func TestResolveFiles_GlobMatchedEmptyDirIsSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.craft"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveFiles([]string{filepath.Join(root, "*")})
	if err != nil {
		t.Fatalf("glob over an empty dir: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "a.craft" {
		t.Fatalf("got %v, want only a.craft", got)
	}
}
