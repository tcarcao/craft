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
		// A cycle is entered where a placed member hands off into it, not
		// at the cycle's first member in the file (billing-cycle-rollover).
		{"cycle entered from a placed member", map[string]string{"a.craft": uc("Open", "j", "  when S opens\n    o notifies e.End\n") +
			uc("Moves", "j", "  when m listens e.Ended\n    m does it\n") +
			uc("Turn", "j", "  when t listens e.End\n    t notifies e.Ended\n") +
			uc("Rearm", "j", "  when r listens e.Ended\n    r notifies e.End\n")}, []string{"Open", "Turn", "Moves", "Rearm"}, true},
		// A member downstream of a cycle waits for it: the cycle is entered
		// on the cycle, not at the downstream member that sorts first by
		// file (re-arch-central seller-cross-posts-an-advert-to-olx, B19).
		{"cycle entered on the cycle, not downstream of it", map[string]string{
			"a-loc.craft": uc("Map", "j", "  when S maps\n    m does it\n"),
			"b-olx.craft": uc("Post", "j", "  when p listens e.Changed\n    p notifies e.Export\n") +
				uc("Change", "j", "  when c listens e.Export\n    c notifies e.Changed\n    continues \"Map\"\n")}, []string{"Post", "Change", "Map"}, true},
		// A cycle fed by another cycle waits for it: X<->Y hands off to
		// Z<->W, so X<->Y is entered first even though W sorts first by file.
		{"cycle fed by a cycle waits for it", map[string]string{
			"a.craft": uc("W", "j", "  when w listens e.Z\n    w notifies e.W\n") +
				uc("Z", "j", "  when z listens e.ToZ, e.W\n    z notifies e.Z\n"),
			"b.craft": uc("X", "j", "  when x listens e.Y\n    x notifies e.X\n") +
				uc("Y", "j", "  when y listens e.X\n    y notifies e.Y\n    y notifies e.ToZ\n")}, []string{"X", "Y", "Z", "W"}, true},
		// A call to a member's receives orders the caller first (§17.2).
		{"call to a member's receives", map[string]string{"a.craft": uc("Callee", "j", "  when c receives a request to rebuild  [rebuild]\n    c does it\n") + uc("Caller", "j", "  when S does\n    a asks c to rebuild it  [rebuild]\n")}, []string{"Caller", "Callee"}, false},
		{"call matches an item of a list", map[string]string{"a.craft": uc("Callee", "j", "  when c receives a request to rebuild  [rebuild]\n    c does it\n") + uc("Caller", "j", "  when S does\n    a asks c to drop and rebuild it  [drop, rebuild]\n")}, []string{"Caller", "Callee"}, false},
		{"call to another operation adds no edge", map[string]string{"a.craft": uc("Callee", "j", "  when c receives a request to rebuild  [rebuild]\n    c does it\n") + uc("Caller", "j", "  when S does\n    a asks c to drop it  [drop]\n")}, []string{"Callee", "Caller"}, false},
		{"call to another context adds no edge", map[string]string{"a.craft": uc("Callee", "j", "  when c receives a request to rebuild  [rebuild]\n    c does it\n") + uc("Caller", "j", "  when S does\n    a asks d to rebuild it  [rebuild]\n")}, []string{"Callee", "Caller"}, false},
		// Only a receives trigger is a callee: a plain trigger whose first
		// action is the called context is not guessed.
		{"plain trigger is no callee", map[string]string{"a.craft": uc("Callee", "j", "  when S needs a rebuild\n    c asks e to rebuild it  [rebuild]\n") + uc("Caller", "j", "  when S does\n    a asks c to rebuild it  [rebuild]\n")}, []string{"Callee", "Caller"}, false},
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

func TestBuild_CallEdge(t *testing.T) {
	j := journey.Build(docOf(t, map[string]string{"a.craft": uc("Callee", "j", "  when c receives a request  [GET /x, rebuild]\n    c does it\n") +
		uc("Caller", "j", "  when S does\n    a asks c to drop and rebuild it  [drop, rebuild, GET /x]\n")}), "j")
	want := []model.JourneyEdge{{From: 0, To: 1, Kind: model.JourneyEdgeCall, Event: "GET /x"}, {From: 0, To: 1, Kind: model.JourneyEdgeCall, Event: "rebuild"}}
	if !slices.Equal(j.Edges, want) || order(j)[0] != "Caller" {
		t.Errorf("order %v edges = %+v", order(j), j.Edges)
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

// ucFormer is uc with a formerly_journey tag.
func ucFormer(name, slug, former, body string) string {
	return "use_case \"" + name + "\" {\n  tags {\n    journey: " + slug + "\n    formerly_journey: " + former + "\n  }\n" + body + "}\n"
}

// TestBuild_TieBreak pins the spec §17.2 choice among ready members: the
// last placed member's successors first, then the namesake, then file and
// line; unconnected members last.
func TestBuild_TieBreak(t *testing.T) {
	chain := func(head, next, ev string) string {
		return uc(head, "j-k", "  when S starts "+ev+"\n    p notifies e."+ev+"\n") +
			uc(next, "j-k", "  when n listens e."+ev+"\n    n does it\n")
	}
	cases := []struct {
		name  string
		files map[string]string
		want  []string
		cycle bool
	}{
		// Two independent chains, interleaved by line in the file, do not
		// interleave in the spine.
		{"two independent chains", map[string]string{"a.craft": uc("E1", "j-k", "  when S starts\n    p notifies e.One\n") +
			uc("E2", "j-k", "  when S starts again\n    q notifies e.Two\n") +
			uc("E1 next", "j-k", "  when n listens e.One\n    n does it\n") +
			uc("E2 next", "j-k", "  when m listens e.Two\n    m does it\n")}, []string{"E1", "E1 next", "E2", "E2 next"}, false},
		// The member whose title slugifies to the slug starts the spine.
		{"namesake by title", map[string]string{"a.craft": chain("X", "X then", "X") + chain("J, k!", "J then", "J")}, []string{"J, k!", "J then", "X", "X then"}, false},
		// A file named after the slug.
		{"namesake by file", map[string]string{"a.craft": chain("X", "X then", "X"), "j-k.craft": chain("N", "N then", "N")}, []string{"N", "N then", "X", "X then"}, false},
		// A file named after one of the member's former journeys.
		{"namesake by former journey file", map[string]string{"a.craft": chain("X", "X then", "X"),
			"old-j.craft": ucFormer("N", "j-k", "old-j", "  when S starts N\n    p notifies e.N\n") + uc("N then", "j-k", "  when n listens e.N\n    n does it\n")}, []string{"N", "N then", "X", "X then"}, false},
		// A file named after the slug beats one named after a member's
		// former journey.
		{"slug file beats former journey file", map[string]string{"a-old.craft": ucFormer("O", "j-k", "a-old", "  when S starts O\n    p notifies e.O\n") + uc("O then", "j-k", "  when o listens e.O\n    o does it\n"),
			"j-k.craft": chain("N", "N then", "N")}, []string{"N", "N then", "O", "O then"}, false},
		// A title match beats a file match.
		{"title beats file", map[string]string{"j-k.craft": chain("N", "N then", "N"), "z.craft": chain("J K", "J then", "J")}, []string{"J K", "J then", "N", "N then"}, false},
		// Unconnected members come last, even the namesake; among them the
		// namesake first, then file and line.
		{"unconnected namesake last", map[string]string{"a.craft": uc("U", "j-k", "  when S looks\n    u does it\n") + chain("A", "B", "A") +
			uc("J K", "j-k", "  when S reads\n    v does it\n")}, []string{"A", "B", "J K", "U"}, false},
		// Nothing hands into the cycle: it is entered at the namesake.
		{"cycle entered at the namesake", map[string]string{"a.craft": uc("A", "j-k", "  when a listens e.B\n    a notifies e.A\n") +
			uc("J K", "j-k", "  when b listens e.A\n    b notifies e.B\n")}, []string{"J K", "A"}, true},
	}
	for _, c := range cases {
		j := journey.Build(docOf(t, c.files), "j-k")
		if got := order(j); !slices.Equal(got, c.want) || j.Cycle != c.cycle {
			t.Errorf("%s: got %v cycle=%v, want %v cycle=%v", c.name, got, j.Cycle, c.want, c.cycle)
		}
	}
}
