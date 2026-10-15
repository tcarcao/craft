package craft_test

import (
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// The display-name fields are part of the public model (2.27, additive).
func TestDisplayNames_Exported(t *testing.T) {
	doc, _, err := craft.ParseFiles(map[string][]byte{
		"topology.craft": []byte("domain billing {\n  nmf_monetization \"nMF, the money engine\"\n}\nactor user Seller \"the seller\"\n"),
		"journey.craft":  []byte("use_case \"U\" {\n  when Seller pays\n    nmf_monetization charges the card\n}\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var d craft.Domain = doc.Domains[0]
	var a craft.Actor = doc.Actors[0]
	if d.DisplayNames["nmf_monetization"] != "nMF, the money engine" || a.DisplayName != "the seller" {
		t.Fatalf("domain %+v actor %+v", d, a)
	}
	var p *craft.Participant = doc.UseCases[0].Scenarios[0].Actions()[0].From
	if p == nil || p.DisplayName != "nMF, the money engine" {
		t.Fatalf("participant %+v", p)
	}
	if got := doc.DisplayNames()["billing/nmf_monetization"]; got != "nMF, the money engine" {
		t.Fatalf("DisplayNames() = %q", got)
	}
}
