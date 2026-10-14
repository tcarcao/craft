package journey_test

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	"github.com/tcarcao/craft/v2/internal/journey"
	"github.com/tcarcao/craft/v2/internal/model"
)

func docOf(t *testing.T, files map[string]string) *model.CraftDoc {
	t.Helper()
	var rs []analysis.FileResult
	for name, src := range files {
		rs = append(rs, analysis.File(name, src))
	}
	return analysis.Workspace(rs).Doc
}

func uc(name, slug, body string) string {
	return "use_case \"" + name + "\" {\n  tags {\n    journey: " + slug + "\n  }\n" + body + "}\n"
}

func order(j model.Journey) []string {
	var out []string
	for _, m := range j.Members {
		out = append(out, m.Name)
	}
	return out
}

func TestBuild(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
		cycle bool
	}{
		{"chain", map[string]string{"a.craft": uc("C", "j", "  when c listens e.B\n    c does it\n") + uc("A", "j", "  when S does\n    a notifies e.A\n") + uc("B", "j", "  when b listens e.A\n    b notifies e.B\n")}, []string{"A", "B", "C"}, false},
		{"fan-out", map[string]string{"a.craft": uc("C", "j", "  when c listens e.A\n    c does it\n") + uc("B", "j", "  when b listens e.A\n    b does it\n") + uc("A", "j", "  when S does\n    a notifies e.A\n")}, []string{"A", "C", "B"}, false},
		{"ties by file then line", map[string]string{"b.craft": uc("Y", "j", "  when S does\n    y does it\n"), "a.craft": uc("X", "j", "  when S does\n    x does it\n")}, []string{"X", "Y"}, false},
		{"members across files", map[string]string{"a.craft": uc("L", "j", "  when l listens e.A\n    l does it\n"), "b.craft": uc("P", "j", "  when S does\n    p notifies e.A\n")}, []string{"P", "L"}, false},
		{"continues", map[string]string{"a.craft": uc("Checkout", "j", "  when S pays\n    p does it\n") + uc("Promote", "j", "  when S promotes\n    q does it\n    continues \"Checkout\"\n")}, []string{"Promote", "Checkout"}, false},
		{"includes adds no edge", map[string]string{"a.craft": uc("Pre", "j", "  when S opens\n    p does it\n") + uc("Main", "j", "  when S starts\n    includes \"Pre\"\n    q does it\n")}, []string{"Pre", "Main"}, false},
		{"store", map[string]string{"a.craft": uc("Read", "j", "  when r listens store.q.inserted\n    r does it\n") + uc("Write", "j", "  when S does\n    w notifies store.q.inserted\n")}, []string{"Write", "Read"}, false},
		{"roster member", map[string]string{"a.craft": uc("L", "j", "  when l listens e.X, e.A\n    l does it\n") + uc("P", "j", "  when S does\n    p notifies e.A\n")}, []string{"P", "L"}, false},
		{"cycle", map[string]string{"a.craft": uc("A", "j", "  when a listens e.B\n    a notifies e.A\n") + uc("B", "j", "  when b listens e.A\n    b notifies e.B\n")}, []string{"A", "B"}, true},
		{"other journeys ignored", map[string]string{"a.craft": uc("A", "j", "  when S does\n    a does it\n") + uc("Z", "k", "  when S does\n    z does it\n")}, []string{"A"}, false},
	}
	for _, c := range cases {
		j := journey.Build(docOf(t, c.files), "j")
		if got := order(j); !slices.Equal(got, c.want) || j.Cycle != c.cycle {
			t.Errorf("%s: got %v cycle=%v, want %v cycle=%v", c.name, got, j.Cycle, c.want, c.cycle)
		}
	}
}

func TestBuild_Edges(t *testing.T) {
	j := journey.Build(docOf(t, map[string]string{"a.craft": uc("A", "j", "  when S does\n    a notifies e.A\n    continues \"B\"\n") + uc("B", "j", "  when b listens e.A\n    b does it\n")}), "j")
	want := []model.JourneyEdge{{From: 0, To: 1, Kind: model.JourneyEdgeContinues}, {From: 0, To: 1, Kind: model.JourneyEdgeEvent, Event: "e.A"}}
	if !slices.Equal(j.Edges, want) {
		t.Errorf("edges = %+v", j.Edges)
	}
	if j.Members[0].UseCase != 0 || j.Members[0].SourceURI != "a.craft" {
		t.Errorf("member 0 = %+v", j.Members[0])
	}
}

func TestBuild_StoreEdge(t *testing.T) {
	j := journey.Build(docOf(t, map[string]string{"a.craft": uc("Read", "j", "  when r listens store.q.inserted\n    r does it\n") + uc("Write", "j", "  when S does\n    w notifies store.q.inserted\n")}), "j")
	if len(j.Edges) != 1 || j.Edges[0].Kind != model.JourneyEdgeStore || j.Edges[0].From != 0 || j.Edges[0].To != 1 {
		t.Errorf("edges = %+v", j.Edges)
	}
}

func TestSlugs(t *testing.T) {
	doc := docOf(t, map[string]string{"a.craft": uc("A", "b-j", "  when S does\n    a does it\n") + uc("B", "a-j", "  when S does\n    b does it\n") + uc("C", "a-j", "  when S does\n    c does it\n")})
	if got := journey.Slugs(doc); !slices.Equal(got, []string{"a-j", "b-j"}) {
		t.Errorf("slugs = %v", got)
	}
}
