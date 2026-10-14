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

func linkWorkspace(files map[string]string) Result {
	var rs []FileResult
	for name, src := range files {
		rs = append(rs, File(name, src))
	}
	return Workspace(rs)
}

func countCode(res Result, code string) int {
	n := 0
	for _, d := range res.Diagnostics {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestLinks_ResolveAcrossFiles(t *testing.T) {
	res := linkWorkspace(map[string]string{
		"a.craft": "use_case \"Promote\" {\n  when S promotes\n    x does it\n    continues \"Pay\"\n}\n",
		"b.craft": "use_case \"Pay at the till\" {\n  tags {\n    formerly_titled: \"Pay; Settle\"\n  }\n  when S pays\n    y does it\n}\n",
	})
	link := res.Doc.UseCases[0].Scenarios[0].Steps[1].Link
	if link.Target == nil || link.Target.Name != "Pay at the till" || link.Target.SourceURI != "b.craft" {
		t.Fatalf("target = %+v", link.Target)
	}
	if countCode(res, "craft/lint/link-to-former-title") != 1 {
		t.Error("a link through formerly_titled warns")
	}
}

func TestLinks_Lints(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		code  string
		want  int
	}{
		{"unresolved", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Nowhere\"\n    x does it\n}\n"}, "craft/sema/unresolved-use-case", 1},
		{"ambiguous", map[string]string{
			"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Twin\"\n    x does it\n}\nuse_case \"Twin\" {\n  when S does\n    x does it\n}\n",
			"b.craft": "use_case \"Twin\" {\n  when S does\n    y does it\n}\n"}, "craft/sema/ambiguous-use-case", 1},
		{"resolved has no unresolved", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    includes \"B\"\n    x does it\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/sema/unresolved-use-case", 0},
		{"continues not last", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    continues \"B\"\n    x does it\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/lint/continues-not-last", 1},
		{"continues then note", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    continues \"B\"\n    note \"fine\"\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/lint/continues-not-last", 0},
		{"continues last in a branch", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    alt \"x\"\n      continues \"B\"\n    x does it\n}\nuse_case \"B\" {\n  when S does\n    y does it\n}\n"}, "craft/lint/continues-not-last", 0},
		{"self", map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    x does it\n    continues \"A\"\n}\n"}, "craft/lint/self-link", 1},
	}
	for _, c := range cases {
		if got := countCode(linkWorkspace(c.files), c.code); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

// Two use cases of one file that share a former title are not "in 2 files":
// the message counts use cases, and files only when there are several.
func TestLinks_AmbiguousMessage(t *testing.T) {
	msg := func(files map[string]string) string {
		for _, d := range linkWorkspace(files).Diagnostics {
			if d.Code == "craft/sema/ambiguous-use-case" {
				return d.Message
			}
		}
		t.Fatal("no ambiguous-use-case")
		return ""
	}
	former := func(name string) string {
		return "use_case \"" + name + "\" {\n  tags {\n    formerly_titled: \"Old\"\n  }\n  when S does\n    y does it\n}\n"
	}
	sameFile := msg(map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Old\"\n    x does it\n}\n" + former("B") + former("C")})
	if want := `"Old" names 2 use cases; rename one of them: a link names one use case`; sameFile != want {
		t.Errorf("same file: %q, want %q", sameFile, want)
	}
	twoFiles := msg(map[string]string{
		"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Old\"\n    x does it\n}\n" + former("B"),
		"b.craft": former("C")})
	if want := `"Old" names 2 use cases in 2 files; rename one of them: a link names one use case`; twoFiles != want {
		t.Errorf("two files: %q, want %q", twoFiles, want)
	}
}

func TestLinks_LintRangeIsTheTitle(t *testing.T) {
	res := linkWorkspace(map[string]string{"a.craft": "use_case \"A\" {\n  when S does\n    includes \"Nowhere\"\n    x does it\n}\n"})
	for _, d := range res.Diagnostics {
		if d.Code != "craft/sema/unresolved-use-case" {
			continue
		}
		if d.SourceURI != "a.craft" || d.Range.Start.Line != 2 || d.Range.Start.Character != 13 || d.Range.End.Character != 22 {
			t.Errorf("range = %+v in %q", d.Range, d.SourceURI)
		}
		return
	}
	t.Error("no unresolved-use-case")
}

func journeyUC(name, slug, body string) string {
	return "use_case \"" + name + "\" {\n  tags {\n    journey: " + slug + "\n  }\n" + body + "}\n"
}

func TestLintModel_JourneyList(t *testing.T) {
	got := modelDiags(t, journeyUC("A", "a, b", "  when S does\n    a does it\n"), "craft/lint/journey-list")
	if len(got) != 1 {
		t.Fatalf("%d journey-list warnings, want 1", len(got))
	}
	if got[0].Severity != model.SeverityWarning || got[0].Range.Start.Line != 2 || got[0].Range.Start.Character != 13 || got[0].Range.End.Character != 14 {
		t.Errorf("diag = %+v", got[0])
	}
	if n := len(modelDiags(t, journeyUC("A", "a", "  when S does\n    a does it\n"), "craft/lint/journey-list")); n != 0 {
		t.Errorf("one value: %d warnings, want 0", n)
	}
}

func TestLintModel_JourneyUnconnected(t *testing.T) {
	two := journeyUC("A", "seller-j", "  when S does\n    a does it\n") + journeyUC("B", "seller-j", "  when S does\n    b does it\n")
	got := modelDiags(t, two, "craft/lint/journey-unconnected")
	if len(got) != 2 {
		t.Fatalf("%d journey-unconnected hints, want 2", len(got))
	}
	for i, line := range []int{2, 9} {
		d := got[i]
		if d.Severity != model.SeverityHint || d.Range.Start.Line != line || d.Range.Start.Character != 13 || d.Range.End.Character != 13+len("seller-j") {
			t.Errorf("diag %d = %+v", i, d)
		}
		if !strings.Contains(d.Message, "`seller-j`") {
			t.Errorf("message %q", d.Message)
		}
	}
	one := journeyUC("A", "j", "  when S does\n    a does it\n")
	chain := journeyUC("A", "j", "  when S does\n    a notifies e.A\n") + journeyUC("B", "j", "  when b listens e.A\n    b does it\n")
	quoted := journeyUC("A", "\"j\"", "  when S does\n    a notifies e.A\n    continues \"B\"\n") + journeyUC("B", "\"j\"", "  when S does\n    b does it\n")
	for name, src := range map[string]string{"one member": one, "chain": chain, "continues": quoted} {
		if n := len(modelDiags(t, src, "craft/lint/journey-unconnected")); n != 0 {
			t.Errorf("%s: %d hints, want 0", name, n)
		}
	}
}
