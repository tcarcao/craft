package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func TestFlow_RequiredProperties(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "flow ok \"OK\" {\n    priority: P0\n    owner: growth\n}\nflow bare \"Bare\" {\n    metric: Leads\n}\n",
	})
	missing := withCode(diags, "craft/sema/missing-flow-property")
	if len(missing) != 2 {
		t.Fatalf("missing-flow-property = %+v", missing)
	}
	for _, d := range missing {
		if d.Severity != model.SeverityError || d.Range.Start.Line != 4 || d.Range.Start.Character != 5 || d.Range.End.Character != 9 {
			t.Errorf("diagnostic = %+v, want an error on the ref `bare`", d)
		}
	}
}

func TestFlow_InvalidPriority(t *testing.T) {
	for _, p := range []string{"P4", "p1", "high", "\"P1 \""} {
		diags := analyzeFiles(t, map[string]string{
			"a.craft": "flow f \"F\" {\n    priority: " + p + "\n    owner: growth\n}\n",
		})
		bad := withCode(diags, "craft/sema/invalid-priority")
		if len(bad) != 1 || bad[0].Severity != model.SeverityError || bad[0].Range.Start.Line != 1 || bad[0].Range.Start.Character != 14 {
			t.Errorf("priority %s: invalid-priority = %+v", p, bad)
		}
	}
	for _, p := range []string{"P0", "P1", "P2", "P3", "\"P2\""} {
		diags := analyzeFiles(t, map[string]string{
			"a.craft": "flow f \"F\" {\n    priority: " + p + "\n    owner: growth\n}\n",
		})
		if len(diags) != 0 {
			t.Errorf("priority %s: diags = %+v", p, diags)
		}
	}
}

func TestFlow_DuplicateProperty(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "flow f \"F\" {\n    priority: P1\n    owner: a\n    priority: P2\n}\n",
	})
	dup := withCode(diags, "craft/sema/duplicate-flow-property")
	if len(dup) != 1 || dup[0].Severity != model.SeverityWarning || dup[0].Range.Start.Line != 3 {
		t.Fatalf("duplicate-flow-property = %+v", dup)
	}
}

// A flow ref is declared once in the workspace: every later declaration, in
// the same file or another, is an error.
func TestFlow_DuplicateRef(t *testing.T) {
	const f = "flow lead \"Lead\" {\n    priority: P1\n    owner: a\n}\n"
	diags := analyzeFiles(t, map[string]string{"a.craft": f, "b.craft": f + f})
	dups := withCode(diags, "craft/sema/duplicate-flow")
	if len(dups) != 2 {
		t.Fatalf("duplicate-flow = %+v", dups)
	}
	for _, d := range dups {
		if d.SourceURI != "b.craft" || d.Severity != model.SeverityError {
			t.Errorf("diagnostic = %+v", d)
		}
	}
	if dups[0].Range.Start.Line == dups[1].Range.Start.Line {
		t.Errorf("both on one line: %+v", dups)
	}
}
