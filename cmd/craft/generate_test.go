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

func TestGenerateCmd_C4BoundariesFlag(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	craftSrc := []byte("actor user Foo\n\nservices {\n  MyService {\n    contexts: Ctx\n  }\n}\n")
	if err := os.WriteFile(src, craftSrc, 0644); err != nil {
		t.Fatal(err)
	}

	for _, boundaries := range []string{"boundaries", "transparent"} {
		t.Run(boundaries, func(t *testing.T) {
			root := newRootCmd()
			root.SetArgs([]string{"generate", src, "--type", "c4", "--boundaries", boundaries, "--output", tmp})
			if err := root.Execute(); err != nil {
				t.Fatalf("generate --boundaries %s: %v", boundaries, err)
			}
			out := filepath.Join(tmp, "test-c4.puml")
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("reading output: %v", err)
			}
			if !strings.Contains(string(data), "@startuml") {
				t.Errorf("expected @startuml in output for --boundaries %s", boundaries)
			}
		})
	}
}

func TestGenerateCmd_C4NoDatabases(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	// Service with a data-store so the flag has something to hide
	craftSrc := []byte("actor user Foo\n\nservices {\n  MyService {\n    contexts: Ctx\n    data-stores: my_db\n  }\n}\n")
	if err := os.WriteFile(src, craftSrc, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("databases shown by default", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "c4", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate default: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-c4.puml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "my_db") {
			t.Error("expected data-store to appear when --no-databases is not set")
		}
	})

	t.Run("databases hidden with --no-databases", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "c4", "--no-databases", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate --no-databases: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-c4.puml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "my_db") {
			t.Error("expected data-store to be hidden when --no-databases is set")
		}
	})
}

func TestGenerateCmd_C4Focus(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	// Two services so focus can include one and exclude the other
	craftSrc := []byte(`actor user Foo

services {
  ServiceA {
    contexts: CtxA
  }
  ServiceB {
    contexts: CtxB
  }
}
`)
	if err := os.WriteFile(src, craftSrc, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("no focus includes all services", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "c4", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate no focus: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-c4.puml"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "ServiceA") || !strings.Contains(content, "ServiceB") {
			t.Error("expected both services when no --focus is set")
		}
	})

	t.Run("focus on one service", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "c4", "--focus", "ServiceA", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate --focus ServiceA: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-c4.puml"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "ServiceA") {
			t.Error("expected focused service ServiceA to appear")
		}
		if strings.Contains(content, "System_Boundary(ServiceB_boundary") {
			t.Error("expected unfocused service ServiceB to not appear as System_Boundary when focusing on ServiceA")
		}
	})
}

func TestGenerateCmd_C4InvalidBoundaries(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, []byte("actor user Foo\n\nservices {\n  MyService {\n    contexts: Ctx\n  }\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	root.SetArgs([]string{"generate", src, "--type", "c4", "--boundaries", "invalid", "--output", tmp})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for --boundaries invalid, got nil")
	}
}
