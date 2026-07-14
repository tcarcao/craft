package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runInspect(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"inspect"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("inspect %v: %v", args, err)
	}
	return buf.String()
}

func TestInspectCmd_JSONMerge(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a.craft")
	b := filepath.Join(tmp, "b.craft")
	os.WriteFile(a, []byte("domain Billing {\n  Invoicing\n}\n"), 0644)
	os.WriteFile(b, []byte("domain Catalog {\n  Products\n}\n"), 0644)

	// pass in sorted order so output is order-stable (see plan D5)
	out := runInspect(t, "--format", "json", a, b)
	var got struct {
		Domains []struct{ Name string } `json:"domains"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("bad json: %v\n%s", err, out)
	}
	if len(got.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d: %s", len(got.Domains), out)
	}
}
