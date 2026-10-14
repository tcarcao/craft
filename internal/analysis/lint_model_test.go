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
			if got[0].Range.Start.Line != 5 {
				t.Errorf("%s: must point at the group head (line 6), got LSP line %d", c.name, got[0].Range.Start.Line)
			}
		}
	}
}

// TestLintModel_DuplicateScenarioBodyMessage pins the message: a fold only
// when one listens trigger can replace both (no store ref, one schedule),
// with each item once; otherwise the message names what differs.
func TestLintModel_DuplicateScenarioBodyMessage(t *testing.T) {
	const body = "    s asks t to do it  [x]\n"
	cases := []struct {
		name, a, b string
		want       string
	}{
		{"repeated item", "  when s listens a.A, b.B\n", "  when s listens b.B\n", "`when s listens a.A, b.B`"},
		{"same schedule", "  when s listens a.A cron \"0 1 * * *\"\n", "  when s listens b.B cron \"0 1 * * *\"\n", "`when s listens a.A, b.B cron \"0 1 * * *\"`"},
		{"schedules differ", "  when s listens a.A cron \"0 1 * * *\"\n", "  when s listens b.B\n", "different schedules"},
		{"store ref", "  when s listens store.q.inserted\n", "  when s listens b.B\n", "store hand-off"},
		{"initiators differ", "  when O starts it\n", "  when P starts it\n", "different initiators (`O` and `P`)"},
		{"listeners differ", "  when s listens a.A\n", "  when t listens a.A\n", "different initiators (`s` and `t`)"},
		{"operations differ", "  when s receives a call  [opA]\n", "  when s receives a call  [opB]\n", "different operations (`[opA]` and `[opB]`)"},
	}
	for _, c := range cases {
		got := modelDiags(t, dupHead+c.a+body+c.b+body+"}\n", "craft/lint/duplicate-scenario-body")
		if len(got) != 1 {
			t.Errorf("%s: %d warnings", c.name, len(got))
			continue
		}
		if !strings.Contains(got[0].Message, c.want) {
			t.Errorf("%s: message %q lacks %q", c.name, got[0].Message, c.want)
		}
		if strings.Contains(got[0].Message, "sites") {
			t.Errorf("%s: names sites/envs: %q", c.name, got[0].Message)
		}
	}
}

// TestLintModel_DuplicateScenarioBodyGroups pins the grouping: scenarios
// with one body form a group, reported once at the group head, and the
// message names the other members' lines and folds every foldable trigger
// of the group into one list, each item once.
func TestLintModel_DuplicateScenarioBodyGroups(t *testing.T) {
	const x = "    s asks t to do it  [x]\n"
	const y = "    s asks t to do more  [y]\n"
	cases := []struct {
		name, body string
		want       int
		has        []string
		hasNot     []string
	}{
		{"four listens, one list",
			"  when s listens a.A\n" + x + "  when s listens b.B\n" + x + "  when s listens c.C\n" + x + "  when s listens d.D\n" + x,
			1, []string{"the scenarios at lines 8, 10 and 12", "fold them into one trigger: `when s listens a.A, b.B, c.C, d.D`"}, []string{"a.A, b.B`", "sites"}},
		{"two scenarios, one line",
			"  when s listens a.A\n" + x + "  when s listens b.B\n" + x,
			1, []string{"the scenario at line 8 run", "`when s listens a.A, b.B`"}, nil},
		{"items once across the group",
			"  when s listens a.A, b.B\n" + x + "  when s listens b.B\n" + x + "  when s listens c.C, a.A\n" + x,
			1, []string{"`when s listens a.A, b.B, c.C`"}, nil},
		{"two groups interleaved",
			"  when s listens a.A\n" + x + "  when s listens b.B\n" + y + "  when s listens c.C\n" + x + "  when s listens d.D\n" + y,
			2, nil, nil},
		{"foldable part of a mixed group",
			"  when s listens a.A\n" + x + "  when O starts it\n" + x + "  when s listens b.B\n" + x + "  when s listens c.C\n" + x,
			1, []string{"fold the scenarios at lines 6, 10 and 12 into one trigger: `when s listens a.A, b.B, c.C`", "different initiators (`s` and `O`)"}, []string{"sites"}},
		{"schedule splits the fold",
			"  when s listens a.A cron \"0 1 * * *\"\n" + x + "  when s listens b.B\n" + x + "  when s listens c.C cron \"0 1 * * *\"\n" + x,
			1, []string{"fold the scenarios at lines 6 and 10 into one trigger: `when s listens a.A, c.C cron \"0 1 * * *\"`", "different schedules"}, nil},
		{"no fold, three initiators",
			"  when O starts it\n" + x + "  when P starts it\n" + x + "  when Q starts it\n" + x,
			1, []string{"the scenarios at lines 8 and 10", "different initiators (`O` and `P`)"}, []string{"fold"}},
	}
	for _, c := range cases {
		got := modelDiags(t, dupHead+c.body+"}\n", "craft/lint/duplicate-scenario-body")
		if len(got) != c.want {
			t.Errorf("%s: %d warnings, want %d: %v", c.name, len(got), c.want, got)
			continue
		}
		if c.want != 1 {
			continue
		}
		if got[0].Range.Start.Line != 5 {
			t.Errorf("%s: must point at the group head, got LSP line %d", c.name, got[0].Range.Start.Line)
		}
		for _, h := range c.has {
			if !strings.Contains(got[0].Message, h) {
				t.Errorf("%s: message %q lacks %q", c.name, got[0].Message, h)
			}
		}
		for _, h := range c.hasNot {
			if strings.Contains(got[0].Message, h) {
				t.Errorf("%s: message %q has %q", c.name, got[0].Message, h)
			}
		}
	}
}
