package analysis_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
)

const registrySrc = `domains {
    billing {
        nmf_monetization "nMF, the money engine"
        portfoliovalue
    }
}
actors {
    user Seller "the seller"
    scheduler CRON
}
surface nmf_monetization {
    query the price list  [getPrices]
}
`

const journeySrc = `use_case "Seller reads prices" {
    when Seller opens the price page
        portfoliovalue asks nmf_monetization for the prices  [getPrices]
        nmf_monetization notifies stream.pricing
}
`

func TestDisplayNames_ProjectAndResolve(t *testing.T) {
	res := analysis.Workspace([]analysis.FileResult{
		analysis.File("topology.craft", registrySrc),
		analysis.File("journey.craft", journeySrc),
	})
	doc := res.Doc
	if got := doc.Domains[0].DisplayNames; len(got) != 1 || got["nmf_monetization"] != "nMF, the money engine" {
		t.Errorf("Domain.DisplayNames = %v", got)
	}
	for _, a := range doc.Actors {
		want := map[string]string{"Seller": "the seller", "CRON": ""}[a.Name]
		if a.DisplayName != want {
			t.Errorf("actor %s DisplayName = %q, want %q", a.Name, a.DisplayName, want)
		}
	}
	dn := doc.DisplayNames()
	if dn["billing/nmf_monetization"] != "nMF, the money engine" || dn["Seller"] != "the seller" || len(dn) != 2 {
		t.Errorf("DisplayNames() = %v", dn)
	}

	sc := doc.UseCases[0].Scenarios[0]
	if s := sc.Trigger.Subject; s == nil || s.DisplayName != "the seller" {
		t.Errorf("trigger subject = %+v", s)
	}
	acts := sc.Actions()
	if acts[0].From == nil || acts[0].From.DisplayName != "" {
		t.Errorf("a context without a display name: %+v", acts[0].From)
	}
	if acts[0].To == nil || acts[0].To.DisplayName != "nMF, the money engine" {
		t.Errorf("asks target = %+v", acts[0].To)
	}
	if acts[1].From == nil || acts[1].From.DisplayName != "nMF, the money engine" {
		t.Errorf("notifies subject = %+v", acts[1].From)
	}
	if p := doc.Surfaces[0].Participant; p == nil || p.DisplayName != "nMF, the money engine" {
		t.Errorf("surface participant = %+v", p)
	}
}

// Without display names the JSON has no new key, so every consumer and
// every golden sees exactly what 2.26 produced.
func TestDisplayNames_AbsentAddsNoJSON(t *testing.T) {
	plain := strings.NewReplacer(` "nMF, the money engine"`, "", ` "the seller"`, "").Replace(registrySrc)
	doc := analysis.Workspace([]analysis.FileResult{
		analysis.File("topology.craft", plain),
		analysis.File("journey.craft", journeySrc),
	}).Doc
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(b)), "displayname") {
		t.Errorf("JSON carries a display-name key: %s", b)
	}
}

// An empty display name declares none.
func TestDisplayNames_EmptyIsNone(t *testing.T) {
	doc := analysis.Workspace([]analysis.FileResult{analysis.File("a.craft", "domain b {\n  x \"\"\n}\nactor user U \"\"\n")}).Doc
	if doc.Domains[0].DisplayNames != nil || doc.Actors[0].DisplayName != "" || len(doc.DisplayNames()) != 0 {
		t.Errorf("empty display names leaked: %+v %+v", doc.Domains, doc.Actors)
	}
}
