package syntax_test

import (
	"testing"

	"github.com/tcarcao/craft/v2/internal/syntax"
)

// A tag statement that fails to parse used to land in the model anyway, and
// the tail of its failed value was promoted to a key nobody wrote:
// `channels: web, mobile` produced `{"channels": "web", "mobile": ""}`.
//
// The diagnostics were correct and at error severity, but `craft check`
// consumers read the data channel, not the diagnostic channel, so a tag that
// was never authored entered the graph with nothing marking it synthetic.
//
// A statement that does not parse now leaves a SyntaxKindErrorNode rather than
// a well-formed-looking SyntaxKindTagStmt, so TagsBlock.Tags() never sees it.
// The comma case itself is no longer a failure at all: see tag_list_test.go.

// projectTags returns the tags map the model builder produces for the first
// use case, which is what `craft check` serialises.
func projectTags(t *testing.T, src string) map[string]string {
	t.Helper()
	g, li, _ := syntax.Parse(src)
	doc := syntax.ProjectFromTree(syntax.Root(g), li, "test.craft")
	if len(doc.UseCases) == 0 {
		t.Fatalf("no use cases projected from %q", src)
	}
	return doc.UseCases[0].Tags
}

// A key with a colon and no value produced `{"channels": ""}`: a tag whose
// value is a lie rather than an absence.
func TestTagRecovery_MissingValueMaterialisesNothing(t *testing.T) {
	tags := projectTags(t, "use_case \"A\" {\n    tags {\n        channels:\n    }\n}\n")

	if len(tags) != 0 {
		t.Errorf("got %+v, want no tags: the statement has no value", tags)
	}
}

// A key with no colon at all, the branch the bug report quotes.
func TestTagRecovery_MissingColonMaterialisesNothing(t *testing.T) {
	tags := projectTags(t, "use_case \"A\" {\n    tags {\n        channels\n    }\n}\n")

	if len(tags) != 0 {
		t.Errorf("got %+v, want no tags: the statement has no `:`", tags)
	}
}

// A value the grammar cannot read at all, the shape testdata/broken's
// tag_bad_value fixture carries.
func TestTagRecovery_UnreadableValueMaterialisesNothing(t *testing.T) {
	tags := projectTags(t, "use_case \"A\" {\n    tags {\n        journey: { }\n    }\n}\n")

	if len(tags) != 0 {
		t.Errorf("got %+v, want no tags: the value is not a value", tags)
	}
}

// A failed statement must not take the next one down with it.
func TestTagRecovery_FollowingStatementStillParses(t *testing.T) {
	src := "use_case \"A\" {\n    tags {\n        channels\n        journey: renewal\n    }\n}\n"

	tags := projectTags(t, src)

	if got := tags["journey"]; got != "renewal" {
		t.Errorf("journey = %q, want \"renewal\": a failed statement must not eat the next one", got)
	}
	if len(tags) != 1 {
		t.Errorf("got %+v, want only the journey tag", tags)
	}
}

// Controls: the single-valued forms that always worked must keep working.
func TestTagRecovery_WorkingFormsUnchanged(t *testing.T) {
	tests := []struct {
		name string
		stmt string
		key  string
		want string
	}{
		{"quoted value with comma", `channels: "web, mobile"`, "channels", "web, mobile"},
		{"bare identifier", `channels: web`, "channels", "web"},
		{"bare ref-shaped slug", `journey: re/renewal-flow`, "journey", "re/renewal-flow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "use_case \"A\" {\n    tags {\n        " + tt.stmt + "\n    }\n}\n"

			if _, _, diags := syntax.Parse(src); len(diags) != 0 {
				t.Fatalf("want no diagnostics for a valid statement, got %+v", diags)
			}
			tags := projectTags(t, src)
			if got := tags[tt.key]; got != tt.want {
				t.Errorf("%s = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// Whatever recovery does to the node kinds, the tree still has to reproduce
// its source byte for byte: that is what keeps `craft fmt` from rewriting a
// malformed block into the shape the parser guessed at.
func TestTagRecovery_TreeStillReproducesSource(t *testing.T) {
	for _, src := range []string{
		"use_case \"A\" {\n    tags {\n        channels:\n    }\n}\n",
		"use_case \"A\" {\n    tags {\n        channels\n    }\n}\n",
		"use_case \"A\" {\n    tags {\n        journey: { }\n    }\n}\n",
	} {
		g, _, _ := syntax.Parse(src)
		if got := reassembleGreen(g); got != src {
			t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
		}
	}
}
