package craft_test

import (
	"slices"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func TestBuildJourney(t *testing.T) {
	files := map[string][]byte{
		"b.craft": []byte("use_case \"Pay\" {\n  tags {\n    journey: buy\n  }\n  when S pays\n    p does it\n}\n"),
		"a.craft": []byte("use_case \"Choose\" {\n  tags {\n    journey: buy\n  }\n  when S chooses\n    c does it\n    continues \"Pay\"\n}\n" +
			"use_case \"Browse\" {\n  tags {\n    journey: look\n  }\n  when S browses\n    c does it\n}\n"),
	}
	doc, _, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if got := craft.JourneySlugs(doc); !slices.Equal(got, []string{"buy", "look"}) {
		t.Errorf("JourneySlugs = %v", got)
	}
	j := craft.BuildJourney(doc, "buy")
	var names []string
	for _, m := range j.Members {
		names = append(names, m.Name)
	}
	if !slices.Equal(names, []string{"Choose", "Pay"}) || j.Cycle {
		t.Errorf("BuildJourney = %v cycle=%v", names, j.Cycle)
	}
	want := []craft.JourneyEdge{{From: 0, To: 1, Kind: craft.JourneyEdgeContinues}}
	if !slices.Equal(j.Edges, want) {
		t.Errorf("edges = %+v", j.Edges)
	}
}
