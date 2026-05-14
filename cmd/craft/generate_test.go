package main

import (
	"bytes"
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

func TestGenerateCmd_UseCaseFilter(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	craftSrc := []byte(`actor user Bob

services {
  Svc {
    contexts: BC1
  }
}

use_case "Alpha use case" {
    when Bob does thing
        BC1 thinks something
        BC1 notifies "AlphaEvent"
}

use_case "Beta use case" {
    when Bob does other
        BC1 reacts
        BC1 notifies "BetaEvent"
}
`)
	if err := os.WriteFile(src, craftSrc, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("filter detailed-domain by slug", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--use-case", "alpha-use-case", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-domain.puml"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "AlphaEvent") {
			t.Errorf("expected AlphaEvent in domain output, got: %s", content)
		}
		if strings.Contains(content, "BetaEvent") {
			t.Errorf("unexpected BetaEvent in filtered domain output, got: %s", content)
		}
	})

	t.Run("filter sequence by exact name", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "sequence", "--use-case", "Beta use case", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(tmp, "test-sequence.puml"))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "== Beta use case ==") {
			t.Errorf("expected Beta section header, got: %s", content)
		}
		if strings.Contains(content, "== Alpha use case ==") {
			t.Errorf("unexpected Alpha section in filtered sequence, got: %s", content)
		}
	})

	t.Run("no match exits non-zero with available slugs in error", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--use-case", "nonexistent", "--output", tmp})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error for nonexistent use case, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "nonexistent") {
			t.Errorf("expected error to mention requested value 'nonexistent', got: %s", msg)
		}
		if !strings.Contains(msg, "alpha-use-case") || !strings.Contains(msg, "beta-use-case") {
			t.Errorf("expected error to list available slugs, got: %s", msg)
		}
	})

	t.Run("c4 silently ignores --use-case", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "c4", "--use-case", "alpha-use-case", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate c4: %v", err)
		}
		// File should exist and contain BOTH events (full model rendered, filter ignored)
		data, err := os.ReadFile(filepath.Join(tmp, "test-c4.puml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "AlphaEvent") || !strings.Contains(string(data), "BetaEvent") {
			t.Errorf("--use-case should not filter c4; expected both events in output")
		}
	})
}

func TestGenerateCmd_Split(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	craftSrc := []byte(`actor user Bob

services {
  Svc {
    contexts: BC1
  }
}

use_case "Alpha use case" {
    when Bob does thing
        BC1 thinks something
        BC1 notifies "AlphaEvent"
}

use_case "Beta use case" {
    when Bob does other
        BC1 reacts
        BC1 notifies "BetaEvent"
}
`)
	if err := os.WriteFile(src, craftSrc, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("split domain emits one file per use case", func(t *testing.T) {
		dir := filepath.Join(tmp, "split-domain")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--split", "--output", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		alphaPath := filepath.Join(dir, "test-domain-alpha-use-case.puml")
		betaPath := filepath.Join(dir, "test-domain-beta-use-case.puml")
		if _, err := os.Stat(alphaPath); err != nil {
			t.Fatalf("expected alpha split file: %v", err)
		}
		if _, err := os.Stat(betaPath); err != nil {
			t.Fatalf("expected beta split file: %v", err)
		}
		// Monolithic file should NOT be present.
		if _, err := os.Stat(filepath.Join(dir, "test-domain.puml")); !os.IsNotExist(err) {
			t.Errorf("split mode should not emit monolithic test-domain.puml")
		}
	})

	t.Run("split sequence emits one file per use case with title directive", func(t *testing.T) {
		dir := filepath.Join(tmp, "split-seq")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "sequence", "--split", "--output", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "test-sequence-alpha-use-case.puml"))
		if err != nil {
			t.Fatalf("expected alpha sequence file: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "title Alpha use case") {
			t.Errorf("expected `title Alpha use case` in split file, got: %s", content)
		}
	})

	t.Run("type all split — c4 single, domain and sequence split", func(t *testing.T) {
		dir := filepath.Join(tmp, "split-all")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "all", "--split", "--output", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		expected := []string{
			"test-c4.puml",
			"test-domain-alpha-use-case.puml",
			"test-domain-beta-use-case.puml",
			"test-sequence-alpha-use-case.puml",
			"test-sequence-beta-use-case.puml",
		}
		for _, name := range expected {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("expected %s to exist: %v", name, err)
			}
		}
		// Monolithic domain/sequence files NOT present.
		if _, err := os.Stat(filepath.Join(dir, "test-domain.puml")); !os.IsNotExist(err) {
			t.Errorf("expected no monolithic test-domain.puml")
		}
		if _, err := os.Stat(filepath.Join(dir, "test-sequence.puml")); !os.IsNotExist(err) {
			t.Errorf("expected no monolithic test-sequence.puml")
		}
	})

	t.Run("split combined with use-case filter emits only filtered files", func(t *testing.T) {
		dir := filepath.Join(tmp, "split-filter")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--split", "--use-case", "beta-use-case", "--output", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "test-domain-beta-use-case.puml")); err != nil {
			t.Errorf("expected beta split file: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "test-domain-alpha-use-case.puml")); !os.IsNotExist(err) {
			t.Errorf("expected no alpha file when filtered to beta")
		}
	})

	t.Run("split with --mode architecture emits single file", func(t *testing.T) {
		dir := filepath.Join(tmp, "split-arch")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--mode", "architecture", "--split", "--output", dir})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "test-domain.puml")); err != nil {
			t.Errorf("expected single test-domain.puml in architecture mode, got: %v", err)
		}
		// No per-use-case files.
		matches, _ := filepath.Glob(filepath.Join(dir, "test-domain-*.puml"))
		if len(matches) > 0 {
			t.Errorf("expected no split files in architecture mode, got: %v", matches)
		}
	})
}

func TestGenerateCmd_FormatMermaid(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, simpleCraft(), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("default format is puml", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		if _, err := os.Stat(filepath.Join(tmp, "test-domain.puml")); err != nil {
			t.Errorf("expected test-domain.puml: %v", err)
		}
	})

	t.Run("format mermaid emits .mmd with flowchart header", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--format", "mermaid", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		path := filepath.Join(tmp, "test-domain.mmd")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if !strings.HasPrefix(string(data), "flowchart LR\n") {
			t.Errorf(".mmd contents should start with 'flowchart LR':\n%s", data)
		}
	})

	t.Run("format mermaid-md emits .md with fenced block", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "sequence", "--format", "mermaid-md", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		path := filepath.Join(tmp, "test-sequence.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		content := string(data)
		if !strings.Contains(content, "```mermaid\n") {
			t.Errorf(".md contents should contain fenced mermaid block:\n%s", content)
		}
		if !strings.Contains(content, "sequenceDiagram\n") {
			t.Errorf(".md contents should contain mermaid source:\n%s", content)
		}
		if !strings.HasPrefix(content, "# ") {
			t.Errorf(".md contents should start with '# <title>':\n%s", content)
		}
	})

	t.Run("invalid format errors with allowed list", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--format", "nonsense", "--output", tmp})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error for invalid format")
		}
		msg := err.Error()
		if !strings.Contains(msg, "puml") || !strings.Contains(msg, "mermaid") {
			t.Errorf("error should list allowed formats, got: %s", msg)
		}
	})
}

func TestGenerateCmd_Stdout(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, simpleCraft(), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("stdout single type writes to cmd.OutOrStdout", func(t *testing.T) {
		var buf bytes.Buffer
		root := newRootCmd()
		root.SetOut(&buf)
		root.SetArgs([]string{"generate", src, "--type", "domain", "--format", "mermaid", "--stdout"})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate --stdout: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "flowchart LR") {
			t.Errorf("expected mermaid source on stdout, got:\n%s", out)
		}
	})

	t.Run("stdout with split errors", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--stdout", "--split"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error for --stdout --split")
		}
		if !strings.Contains(err.Error(), "split") {
			t.Errorf("error should mention --split, got: %s", err)
		}
	})

	t.Run("stdout with type all errors", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "all", "--stdout"})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error for --stdout --type all")
		}
		if !strings.Contains(err.Error(), "single") {
			t.Errorf("error should mention single-diagram requirement, got: %s", err)
		}
	})

	t.Run("stdout with output errors", func(t *testing.T) {
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--stdout", "--output", tmp})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error for --stdout --output")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("error should mention mutual exclusion, got: %s", err)
		}
	})
}

func TestGenerateCmd_MermaidMDNoClobber(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "test.craft")
	if err := os.WriteFile(src, simpleCraft(), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("refuses to overwrite existing .md without --force", func(t *testing.T) {
		target := filepath.Join(tmp, "test-domain.md")
		if err := os.WriteFile(target, []byte("# pre-existing\n"), 0644); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--format", "mermaid-md", "--output", tmp})
		err := root.Execute()
		if err == nil {
			t.Fatal("expected error refusing to overwrite existing .md")
		}
		if !strings.Contains(err.Error(), "refusing to overwrite") {
			t.Errorf("expected refusal message, got: %s", err)
		}
		data, _ := os.ReadFile(target)
		if string(data) != "# pre-existing\n" {
			t.Errorf("pre-existing file was modified despite refusal")
		}
	})

	t.Run("--force overwrites existing .md", func(t *testing.T) {
		target := filepath.Join(tmp, "test-sequence.md")
		if err := os.WriteFile(target, []byte("# pre-existing\n"), 0644); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "sequence", "--format", "mermaid-md", "--output", tmp, "--force"})
		if err := root.Execute(); err != nil {
			t.Fatalf("--force should allow overwrite: %v", err)
		}
		data, _ := os.ReadFile(target)
		if !strings.Contains(string(data), "```mermaid") {
			t.Errorf("expected overwritten content to be mermaid-md output, got:\n%s", data)
		}
	})

	t.Run("puml format does not trigger no-clobber (overwrites silently)", func(t *testing.T) {
		target := filepath.Join(tmp, "test-domain.puml")
		if err := os.WriteFile(target, []byte("stale-content"), 0644); err != nil {
			t.Fatal(err)
		}
		root := newRootCmd()
		root.SetArgs([]string{"generate", src, "--type", "domain", "--format", "puml", "--output", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("puml overwrite should be silent: %v", err)
		}
		data, _ := os.ReadFile(target)
		if !strings.Contains(string(data), "@startuml") {
			t.Errorf("puml file should be overwritten with generated content")
		}
	})
}

// simpleCraft returns a minimal valid .craft source for CLI tests.
func simpleCraft() []byte {
	return []byte(`actor user Bob

services {
  Svc {
    contexts: BC1
  }
}

use_case "Alpha" {
    when Bob does thing
        BC1 thinks something
        BC1 notifies "AlphaEvent"
}
`)
}
