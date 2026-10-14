package analysis

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

func modelDiags(t *testing.T, src, code string) []model.Diagnostic {
	t.Helper()
	var out []model.Diagnostic
	for _, d := range Workspace([]FileResult{File("m.craft", src)}).Diagnostics {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

const dupHead = "domain d {\n  s\n  t\n}\nuse_case \"U\" {\n"

func TestLintModel_DuplicateScenarioBody(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
		fold       bool
	}{
		{"identical listens", "  when s listens a.A\n    s asks t to do it  [x]\n  when s listens b.B\n    s asks t to do it  [x]\n", 1, true},
		{"one phrase differs", "  when s listens a.A\n    s asks t to do it\n  when s listens b.B\n    s asks t to do more\n", 0, false},
		{"only notes differ", "  when s listens a.A\n    s does it\n    note \"x\"\n  when s listens b.B\n    s does it\n", 1, true},
		{"external triggers", "  when O starts it\n    s does it\n  when O resumes it\n    s does it\n", 1, false},
		{"notes only body", "  when s listens a.A\n    note \"x\"\n  when s listens b.B\n    note \"x\"\n", 0, false},
	}
	for _, c := range cases {
		got := modelDiags(t, dupHead+c.body+"}\n", "craft/lint/duplicate-scenario-body")
		if len(got) != c.want {
			t.Errorf("%s: %d warnings, want %d", c.name, len(got), c.want)
			continue
		}
		if c.want == 1 {
			if fold := strings.Contains(got[0].Message, "when s listens a.A, b.B"); fold != c.fold {
				t.Errorf("%s: message %q", c.name, got[0].Message)
			}
			if got[0].Range.Start.Line == 5 {
				t.Errorf("%s: must point at the second scenario", c.name)
			}
		}
	}
}
