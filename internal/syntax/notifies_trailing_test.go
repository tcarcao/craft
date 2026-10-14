package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// An event is one name. Words after it on the line were dropped silently
// (`sync notifies the owners team` read as the event `the`); they are an
// error at the first extra word.
func TestNotifies_ExtraWordsAfterTheEventAreAnError(t *testing.T) {
	cases := []struct {
		line   string
		col    int // 0-based character of the first extra word
		hasOp  bool
		events string
	}{
		{"sync notifies the owners team", 26, false, "the"},
		{"sync notifies a.Done now  [TOPIC t]", 29, true, "a.Done"},
		{"sync notifies \"a done\" twice", 31, false, "a done"},
	}
	for _, c := range cases {
		src := "use_case \"U\" {\n    when O does\n        " + c.line + "\n        sync records it\n}"
		g, _, diags := syntax.Parse(src)
		if len(diags) != 1 || diags[0].Code != "craft/syntax/unexpected-token" {
			t.Errorf("%s: diags = %v", c.line, diags)
			continue
		}
		if r := diags[0].Range; r.Start.Line != 2 || r.Start.Character != c.col {
			t.Errorf("%s: range = %+v, want line 2 character %d", c.line, r, c.col)
		}
		sc := syntax.AsFile(syntax.Root(g)).UseCases()[0].Scenarios()[0]
		acts := sc.Actions()
		if len(acts) != 2 {
			t.Errorf("%s: %d actions, want 2", c.line, len(acts))
			continue
		}
		if acts[0].EventValue() != c.events {
			t.Errorf("%s: event = %q", c.line, acts[0].EventValue())
		}
		if got := acts[0].OpText() != ""; got != c.hasOp {
			t.Errorf("%s: op = %q", c.line, acts[0].OpText())
		}
	}
}

func TestNotifies_EventThenBracketOrCommentIsClean(t *testing.T) {
	for _, line := range []string{"sync notifies a.Done", "sync notifies a.Done  [TOPIC t]", "sync notifies a.Done // why", "sync notifies \"a done\""} {
		_, _, diags := syntax.Parse("use_case \"U\" {\n    when O does\n        " + line + "\n}")
		if len(diags) != 0 {
			t.Errorf("%s: %v", line, diags)
		}
	}
}
