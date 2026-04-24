package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCmd_V2Default(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, []byte("actor user Foo\n\nservices {\n  MyService {\n    contexts: Ctx\n  }\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetArgs([]string{"generate", src, "--type", "c4", "--output", tmp})
	if err := root.Execute(); err != nil {
		t.Fatalf("generate with v2 default: %v", err)
	}

	got, err := filepath.Glob(filepath.Join(tmp, "*.puml"))
	if err != nil || len(got) == 0 {
		t.Fatal("expected at least one .puml output file")
	}
	for _, f := range got {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		content := string(data)
		if !strings.Contains(content, "@startuml") {
			t.Errorf("%s: expected @startuml in output", f)
		}
		if !strings.Contains(content, "@enduml") {
			t.Errorf("%s: expected @enduml in output", f)
		}
	}
}

func TestGenerateCmd_V2ErrorsOnBrokenInput(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "broken.craft")
	// Unclosed block — parser emits SeverityError
	if err := os.WriteFile(src, []byte("services {\n  MyService: {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetArgs([]string{"generate", src, "--output", tmp})
	if err := root.Execute(); err == nil {
		t.Error("expected error for broken input with v2 parser, got nil")
	}
}
