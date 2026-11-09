package craft_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// The public API a consumer computing AI coverage reads: Actor.AI,
// Actor.Owner, CraftDoc.ActorByName, AIKinds and the JSON keys (v2.37).
func TestActorAI_PublicAPI(t *testing.T) {
	src := "actors {\n  system DescriptionGenerator \"description generator\" {\n    ai: genai\n    owner: \"OLX Data\"\n  }\n  user Seller\n}\n" +
		"use_case \"Seller writes an ad\" {\n  when Seller writes an ad\n    ads asks DescriptionGenerator to draft the description\n}\n"
	doc, diags, err := craft.Parse("a.craft", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("unexpected error %+v", d)
		}
	}
	var a *craft.Actor = doc.ActorByName("DescriptionGenerator")
	if a == nil || a.AI != "genai" || a.Owner != "OLX Data" {
		t.Fatalf("actor = %+v", a)
	}
	if got := craft.AIKinds(); !slices.Equal(got, []string{"ml", "genai", "agentic"}) {
		t.Errorf("AIKinds = %v", got)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"displayName":"description generator","ai":"genai","owner":"OLX Data"}`) {
		t.Errorf("JSON lacks ai/owner:\n%s", b)
	}
	if strings.Contains(string(b), `"name":"Seller","type":"user","line":6,"ai"`) {
		t.Errorf("actor without a body carries ai:\n%s", b)
	}
}

// A model without actor bodies has no ai, owner or notes key on its actors.
func TestActorAI_AbsentKeysOmitted(t *testing.T) {
	doc, _, err := craft.Parse("a.craft", []byte("actor system Siren \"Siren\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc.Actors)
	if string(b) != `[{"name":"Siren","type":"system","line":1,"displayName":"Siren"}]` {
		t.Errorf("JSON = %s", b)
	}
}
