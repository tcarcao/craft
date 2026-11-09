package sema_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func TestActorAI_InvalidKind(t *testing.T) {
	for _, k := range []string{"llm", "GenAI", "\"ml \"", "ai"} {
		diags := analyzeFiles(t, map[string]string{"a.craft": "actor system G {\n    ai: " + k + "\n}\n"})
		bad := withCode(diags, "craft/sema/invalid-ai-kind")
		if len(bad) != 1 || bad[0].Severity != model.SeverityError || bad[0].Range.Start.Line != 1 || bad[0].Range.Start.Character != 8 {
			t.Errorf("ai %s: invalid-ai-kind = %+v", k, bad)
		}
	}
	for _, k := range []string{"ml", "genai", "agentic", "\"genai\""} {
		diags := analyzeFiles(t, map[string]string{"a.craft": "actor system G {\n    ai: " + k + "\n    owner: \"OLX Data\"\n}\n"})
		if len(diags) != 0 {
			t.Errorf("ai %s: diags = %+v", k, diags)
		}
	}
}

func TestActorAI_DuplicateProperty(t *testing.T) {
	diags := analyzeFiles(t, map[string]string{
		"a.craft": "actors {\n    system G {\n        ai: ml\n        owner: a\n        ai: genai\n    }\n}\n",
	})
	dup := withCode(diags, "craft/sema/duplicate-actor-property")
	if len(dup) != 1 || dup[0].Severity != model.SeverityWarning || dup[0].Range.Start.Line != 4 || dup[0].Range.Start.Character != 8 {
		t.Errorf("duplicate-actor-property = %+v", dup)
	}
}

// The same actor declared again with another `ai:`: the first declaration
// in sorted file order is the actor's, so the later value is reported. A
// later declaration without `ai:` is not a conflict.
func TestLint_ConflictingAIKind(t *testing.T) {
	diags := lintFiles(t, map[string]string{
		"file:///a.craft": "actor system Gen {\n    ai: ml\n}\nactor system Plain\n",
		"file:///b.craft": "actors {\n    system Gen { ai: genai }\n    system Plain { ai: agentic }\n}\nactor system Gen\nactor system Gen { ai: ml }\n",
	})
	got := byCode(diags, "craft/lint/conflicting-ai-kind")
	if len(got) != 2 {
		t.Fatalf("conflicting-ai-kind = %v", got)
	}
	want := []struct {
		line, start int
		msg         string
	}{
		{1, 21, "actor Gen is first declared with `ai: ml` (file:///a.craft), which the model keeps; use the same kind here or remove this one"},
		{2, 23, "actor Plain is first declared without `ai:` (file:///a.craft), so the model keeps it unclassified; move `ai: agentic` there or remove this one"},
	}
	for i, w := range want {
		d := got[i]
		if d.SourceURI != "file:///b.craft" || d.Severity != model.SeverityWarning || d.Message != w.msg ||
			d.Range.Start.Line != w.line || d.Range.Start.Character != w.start {
			t.Errorf("diag %d = %+v", i, d)
		}
	}
}
