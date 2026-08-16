package syntax_test

import (
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

// A tag value is a comma-separated list, mirroring the shape `contexts:` and
// `data-stores:` already use: each item is a quoted string or a bare
// ref-shaped slug, the two may be mixed, and the list may wrap across lines.
//
// The model carries both forms. Tags keeps its existing type and joins a list
// with ", ", so no consumer of the old shape stops working; TagValues carries
// the split form, which is the only one that can tell `channels: web, mobile`
// apart from `channels: "web, mobile"`.

func projectUseCase(t *testing.T, stmts string) model.UseCase {
	t.Helper()
	src := "use_case \"A\" {\n    tags {\n" + stmts + "\n    }\n}\n"
	g, li, diags := syntax.Parse(src)
	for _, d := range diags {
		t.Fatalf("unexpected diagnostic for %q: %+v", stmts, d)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li, "test.craft")
	if len(doc.UseCases) == 0 {
		t.Fatalf("no use cases projected from %q", stmts)
	}
	return doc.UseCases[0]
}

func TestTagList_ParsesAndSplits(t *testing.T) {
	tests := []struct {
		name      string
		stmt      string
		key       string
		wantJoin  string
		wantSplit []string
	}{
		{"bare list", `channels: web, mobile`, "channels", "web, mobile", []string{"web", "mobile"}},
		{"no space after comma", `channels: web,mobile`, "channels", "web, mobile", []string{"web", "mobile"}},
		{"mixed quoting", `channels: "web app", mobile`, "channels", "web app, mobile", []string{"web app", "mobile"}},
		{"quoted items only", `channels: "web app", "mobile app"`, "channels", "web app, mobile app", []string{"web app", "mobile app"}},
		{"ref-shaped items", `journey: re/renewal-flow, re/other`, "journey", "re/renewal-flow, re/other", []string{"re/renewal-flow", "re/other"}},
		{"three items", `channels: web, mobile, kiosk`, "channels", "web, mobile, kiosk", []string{"web", "mobile", "kiosk"}},
		{"single bare stays single", `journey: re/renewal-flow`, "journey", "re/renewal-flow", []string{"re/renewal-flow"}},
		{"single quoted stays single", `channels: "web, mobile"`, "channels", "web, mobile", []string{"web, mobile"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := projectUseCase(t, "        "+tt.stmt)

			if got := uc.Tags[tt.key]; got != tt.wantJoin {
				t.Errorf("Tags[%q] = %q, want %q", tt.key, got, tt.wantJoin)
			}
			got := uc.TagValues[tt.key]
			if len(got) != len(tt.wantSplit) {
				t.Fatalf("TagValues[%q] = %q, want %q", tt.key, got, tt.wantSplit)
			}
			for i := range got {
				if got[i] != tt.wantSplit[i] {
					t.Errorf("TagValues[%q][%d] = %q, want %q", tt.key, i, got[i], tt.wantSplit[i])
				}
			}
		})
	}
}

// The one thing the joined form cannot express: a list of two and a single
// value containing a comma collapse to the same string. TagValues is what
// separates them, which is the reason it exists.
func TestTagList_JoinedFormIsAmbiguousButSplitIsNot(t *testing.T) {
	list := projectUseCase(t, `        channels: web, mobile`)
	single := projectUseCase(t, `        channels: "web, mobile"`)

	if list.Tags["channels"] != single.Tags["channels"] {
		t.Fatal("precondition: the joined forms are meant to be identical here")
	}
	if len(list.TagValues["channels"]) == len(single.TagValues["channels"]) {
		t.Errorf("TagValues must distinguish a 2-item list from a 1-item value with a comma: %q vs %q",
			list.TagValues["channels"], single.TagValues["channels"])
	}
}

// Lists wrap across lines, the same way a wrapped `contexts:` does.
func TestTagList_WrapsAcrossLines(t *testing.T) {
	uc := projectUseCase(t, "        channels: web,\n            mobile,\n            kiosk")

	want := []string{"web", "mobile", "kiosk"}
	got := uc.TagValues["channels"]
	if len(got) != len(want) {
		t.Fatalf("TagValues = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A list must not run into the next statement. `contexts:` stops silently when
// the item after a `,` is missing, which here would take the next tag's key as
// a list item and put a value in the model that nobody wrote.
func TestTagList_DanglingCommaDoesNotEatTheNextStatement(t *testing.T) {
	src := "use_case \"A\" {\n    tags {\n        channels: web,\n        journey: renewal\n    }\n}\n"

	g, li, diags := syntax.Parse(src)

	if len(diags) != 1 {
		t.Fatalf("want exactly 1 diagnostic for the dangling comma, got %d: %+v", len(diags), diags)
	}
	if diags[0].Severity != model.SeverityError {
		t.Errorf("severity = %q, want %q", diags[0].Severity, model.SeverityError)
	}

	doc := syntax.ProjectFromTree(syntax.Root(g), li, "test.craft")
	uc := doc.UseCases[0]
	if _, fabricated := uc.TagValues["channels"]; fabricated {
		t.Errorf("the channels statement did not parse, so it must not reach the model; got %q", uc.TagValues)
	}
	if got := uc.Tags["journey"]; got != "renewal" {
		t.Errorf("journey = %q, want \"renewal\": the failed list must not consume the next statement", got)
	}
}

// A trailing comma at the end of the block is the same failure.
func TestTagList_TrailingCommaIsAnError(t *testing.T) {
	src := "use_case \"A\" {\n    tags {\n        channels: web,\n    }\n}\n"

	g, li, diags := syntax.Parse(src)

	if len(diags) != 1 {
		t.Fatalf("want exactly 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "comma") {
		t.Errorf("message should name the dangling comma, got %q", diags[0].Message)
	}
	doc := syntax.ProjectFromTree(syntax.Root(g), li, "test.craft")
	if tags := doc.UseCases[0].TagValues; len(tags) != 0 {
		t.Errorf("got %q, want nothing: the statement did not parse", tags)
	}
}

// Every tag gets a TagValues entry, not only the ones written as a list, so a
// consumer can read one field uniformly instead of testing for the other.
func TestTagList_SingleValuesAlsoAppearInTagValues(t *testing.T) {
	uc := projectUseCase(t, "        journey: renewal\n        owner: billing-team")

	for _, key := range []string{"journey", "owner"} {
		if got := uc.TagValues[key]; len(got) != 1 {
			t.Errorf("TagValues[%q] = %q, want a one-item list", key, got)
		}
	}
}

// A use case with no tags block must not gain an empty TagValues map in the
// JSON, matching how Tags is already omitted.
func TestTagList_NoTagsBlockLeavesBothFieldsNil(t *testing.T) {
	g, li, _ := syntax.Parse("use_case \"A\" {\n    when S enters the page\n        f asks g for it\n}\n")
	doc := syntax.ProjectFromTree(syntax.Root(g), li, "test.craft")

	if uc := doc.UseCases[0]; uc.Tags != nil || uc.TagValues != nil {
		t.Errorf("Tags = %v, TagValues = %v, want both nil", uc.Tags, uc.TagValues)
	}
}

// The tree still has to reproduce its source byte for byte, lists included.
func TestTagList_TreeReproducesSource(t *testing.T) {
	for _, src := range []string{
		"use_case \"A\" {\n    tags {\n        channels: web, mobile\n    }\n}\n",
		"use_case \"A\" {\n    tags {\n        channels: web,\n            mobile\n    }\n}\n",
		"use_case \"A\" {\n    tags {\n        channels: \"web app\", re/tablet-web\n    }\n}\n",
		"use_case \"A\" {\n    tags {\n        channels: web,\n        journey: renewal\n    }\n}\n",
		"use_case \"A\" {\n    tags { channels: web, mobile }\n}\n",
	} {
		g, _, _ := syntax.Parse(src)
		if got := reassembleGreen(g); got != src {
			t.Errorf("round-trip mismatch\nwant: %q\ngot:  %q", src, got)
		}
	}
}
