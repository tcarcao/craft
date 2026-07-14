package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runCheck(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"check"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("check %v: %v", args, err)
	}
	return buf.String()
}

func TestCheckCmd_DefaultEmitsDoc(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "x.craft")
	os.WriteFile(src, []byte("actor user Foo\n\nservices {\n  S {\n    contexts: Ctx\n  }\n}\n"), 0644)

	out := runCheck(t, src)
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if _, ok := doc["services"]; !ok {
		t.Errorf("expected services in doc output: %s", out)
	}
}

func TestCheckCmd_LSPJSONShape(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "x.craft")
	os.WriteFile(src, []byte("actor user Foo\n"), 0644)

	out := runCheck(t, src, "--lsp-json")
	var env struct {
		CraftDoc    json.RawMessage `json:"craftDoc"`
		Diagnostics json.RawMessage `json:"diagnostics"`
		Symbols     []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			Type string `json:"type"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("lsp-json not valid: %v\n%s", err, out)
	}
	if len(env.Symbols) != 1 || env.Symbols[0].Name != "Foo" || env.Symbols[0].Kind != "actor" || env.Symbols[0].Type != "user" {
		t.Errorf("symbols mismatch: %+v", env.Symbols)
	}
}

func TestCheckCmd_LSPJSONOmitsTypelessActor(t *testing.T) {
	// "actor Foo" has no resolvable type; the projection drops it from
	// CraftDoc.Actors, so --lsp-json omits it too. This is a deliberate,
	// documented change from the old raw-tree walk (which emitted it with an
	// empty type): the symbol list now tracks the canonical model. A
	// well-formed actor in the same file is still emitted.
	tmp := t.TempDir()
	src := filepath.Join(tmp, "x.craft")
	os.WriteFile(src, []byte("actor user Alice\nactor Foo\n"), 0644)

	out := runCheck(t, src, "--lsp-json")
	var env struct {
		Symbols []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("lsp-json not valid: %v\n%s", err, out)
	}
	hasAlice := false
	for _, s := range env.Symbols {
		if s.Name == "Foo" {
			t.Errorf("typeless actor Foo should be omitted from symbols, got %+v", env.Symbols)
		}
		if s.Name == "Alice" {
			hasAlice = true
		}
	}
	if !hasAlice {
		t.Errorf("well-formed actor Alice should be present, got %+v", env.Symbols)
	}
}
