package craft_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// The public API a consumer such as the hub reads: CraftDoc.Pages,
// Page.UsedBy, UseCase.Entries, CraftDoc.PageByRef and the JSON keys.
func TestPages_PublicAPI(t *testing.T) {
	files := map[string][]byte{
		"pages.craft": []byte("page account-adverts \"My Account: My Ads\" {\n  url: \"https://www.otodom.pl/wiadomosci/moje-ogloszenia\"\n  platform: frontoffice\n  product_domain: ad-inventory\n}\n"),
		"uc.craft": []byte("use_case \"Seller promotes an ad\" {\n  tags {\n    " + craft.TagEntry + ": account-adverts\n  }\n" +
			"  when Seller promotes an ad\n    promotion asks billing to charge it\n}\n"),
	}
	doc, diags, err := craft.ParseFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if d.Severity == craft.SeverityError {
			t.Errorf("unexpected error %+v", d)
		}
	}
	var p *craft.Page = doc.PageByRef("account-adverts")
	if p == nil || p.Name != "My Account: My Ads" || p.Platform != "frontoffice" || p.ProductDomain != "ad-inventory" ||
		!slices.Equal(p.URLs, []string{"https://www.otodom.pl/wiadomosci/moje-ogloszenia"}) {
		t.Fatalf("page = %+v", p)
	}
	if want := []craft.UseCaseRef{{Name: "Seller promotes an ad", SourceURI: "uc.craft"}}; !slices.Equal(p.UsedBy, want) {
		t.Errorf("UsedBy = %+v", p.UsedBy)
	}
	if got := doc.UseCases[0].Entries; !slices.Equal(got, []string{"account-adverts"}) {
		t.Errorf("Entries = %v", got)
	}
	if got := craft.PagePlatforms(); !slices.Equal(got, []string{"frontoffice", "backoffice", "app"}) {
		t.Errorf("PagePlatforms = %v", got)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"pages":[{"ref":"account-adverts"`, `"urls":["https://`, `"productDomain":"ad-inventory"`,
		`"usedBy":[{"name":"Seller promotes an ad"`, `"entries":["account-adverts"]`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON lacks %s:\n%s", key, b)
		}
	}
}

// A model without pages or entry tags has no pages key and no entries key.
func TestPages_AbsentKeysOmitted(t *testing.T) {
	doc, _, err := craft.Parse("a.craft", []byte("use_case \"U\" {\n  when S does it\n    c does it\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc)
	if strings.Contains(string(b), `"pages"`) || strings.Contains(string(b), `"entries"`) {
		t.Errorf("JSON = %s", b)
	}
}
