package analysis

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/model"
)

const pageDecls = `page account-adverts "My Account: My Ads" {
    url: "https://www.otodom.pl/wiadomosci/moje-ogloszenia"
    url: "https://www.storia.ro/ro/contul-meu"
    url: "https://www.otodom.pl/wiadomosci/moje-ogloszenia"
    platform: frontoffice
    product_domain: ad-inventory
    note "the old Atlas page is still served for agencies"
}

page ad-detail "Ad detail" {
    product_domain: "seeker experience"
}
`

const pageUseCases = `use_case "Seller promotes an ad" {
    tags {
        entry: account-adverts
    }
    when Seller promotes an ad
        promotion asks billing to charge the seller
}

use_case "Visitor sees an offer" {
    tags {
        entry:  ad-detail , account-adverts, ghost, ad-detail
    }
    when Visitor opens an ad
        promotion asks ads to show the offer
}
`

func TestPages_Model(t *testing.T) {
	res := Workspace([]FileResult{File("b.craft", pageUseCases), File("a.craft", pageDecls)})
	doc := res.Doc
	if len(doc.Pages) != 2 {
		t.Fatalf("pages = %+v", doc.Pages)
	}
	aa := doc.PageByRef("account-adverts")
	if aa == nil || aa.Name != "My Account: My Ads" || aa.Platform != "frontoffice" || aa.ProductDomain != "ad-inventory" ||
		aa.SourceURI != "a.craft" || aa.Line != 1 || len(aa.Notes) != 1 ||
		!slices.Equal(aa.URLs, []string{"https://www.otodom.pl/wiadomosci/moje-ogloszenia", "https://www.storia.ro/ro/contul-meu"}) {
		t.Fatalf("account-adverts = %+v", aa)
	}
	want := []model.UseCaseRef{{Name: "Seller promotes an ad", SourceURI: "b.craft"}, {Name: "Visitor sees an offer", SourceURI: "b.craft"}}
	if !slices.Equal(aa.UsedBy, want) {
		t.Errorf("used by %+v, want %+v", aa.UsedBy, want)
	}
	if ad := doc.PageByRef("ad-detail"); ad == nil || ad.ProductDomain != "seeker experience" || len(ad.UsedBy) != 1 {
		t.Errorf("ad-detail = %+v", ad)
	}
	if got := doc.UseCases[1].Entries; !slices.Equal(got, []string{"ad-detail", "account-adverts", "ghost"}) {
		t.Errorf("entries = %v", got)
	}
	var unresolved []model.Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == "craft/sema/unresolved-page" {
			unresolved = append(unresolved, d)
		}
	}
	if len(unresolved) != 1 || unresolved[0].Severity != model.SeverityWarning || unresolved[0].SourceURI != "b.craft" ||
		unresolved[0].Range.Start.Line != 10 || unresolved[0].Range.Start.Character != 45 {
		t.Errorf("unresolved-page = %+v", unresolved)
	}
}

// A workspace that declares no page keeps `entry` as free text: no
// unresolved-page warning, no pages key.
func TestPages_NoPagesNoWarning(t *testing.T) {
	res := Workspace([]FileResult{File("b.craft", pageUseCases)})
	for _, d := range res.Diagnostics {
		if d.Code == "craft/sema/unresolved-page" {
			t.Errorf("unexpected %+v", d)
		}
	}
	if len(res.Doc.Pages) != 0 || len(res.Doc.UseCases[0].Entries) != 1 {
		t.Errorf("doc = %+v", res.Doc)
	}
}
