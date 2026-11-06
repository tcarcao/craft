package syntax_test

import (
	"slices"
	"testing"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/syntax"
)

const pageSrc = `page account-adverts "My Account: My Ads" {
    url: "https://www.otodom.pl/wiadomosci/moje-ogloszenia"
    url: "https://www.storia.ro/ro/contul-meu"
    platform: frontoffice
    product_domain: ad-inventory
    note "the old Atlas page is still served for agencies"
}
`

func TestPage_Parse(t *testing.T) {
	g, _, diags := syntax.Parse(pageSrc)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	ps := syntax.AsFile(syntax.Root(g)).Pages()
	if len(ps) != 1 {
		t.Fatalf("pages = %d", len(ps))
	}
	p := ps[0]
	if p.Ref() != "account-adverts" || p.Name() != "My Account: My Ads" {
		t.Errorf("ref/name = %q %q", p.Ref(), p.Name())
	}
	if got := p.PropertyValues(lang.PageURL); !slices.Equal(got, []string{"https://www.otodom.pl/wiadomosci/moje-ogloszenia", "https://www.storia.ro/ro/contul-meu"}) {
		t.Errorf("urls = %v", got)
	}
	if v, _ := p.Property(lang.PagePlatform); v != "frontoffice" {
		t.Errorf("platform = %q", v)
	}
	if v, _ := p.Property(lang.PageProductDomain); v != "ad-inventory" {
		t.Errorf("product_domain = %q", v)
	}
	if len(p.Notes()) != 1 {
		t.Errorf("notes = %d", len(p.Notes()))
	}
	for _, prop := range p.Properties() {
		if !prop.Known() {
			t.Errorf("property %q not known", prop.Key())
		}
	}
}

// `page` is contextual: as a name it parses as before.
func TestPage_WordAsName(t *testing.T) {
	src := "domain page {\n    page\n}\n\nuse_case \"Page\" {\n    when page opens the page\n        page asks page to page the seller\n}\n"
	g, _, diags := syntax.Parse(src)
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diags)
	}
	if n := len(syntax.AsFile(syntax.Root(g)).Pages()); n != 0 {
		t.Errorf("pages = %d", n)
	}
}

func TestPage_Diagnostics(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"page p {\n    product_domain: x\n}\n", "craft/syntax/missing-label"},
		{"page \"Name\" {\n    product_domain: x\n}\n", "craft/syntax/unexpected-token"},
		{"page p \"P\" {\n    owner: x\n    product_domain: x\n}\n", "craft/syntax/unknown-page-property"},
		{"page p \"P\" {\n    url:\n    product_domain: x\n}\n", "craft/syntax/unexpected-token"},
		{"page p \"P\" {\n    product_domain: x\n\nuse_case \"U\" {\n    when A does it\n        b does it\n}\n", "craft/syntax/unclosed-block"},
		// A keyword used as a key is an unknown property, not the end of the body.
		{"page p \"P\" {\n    domain: x\n    product_domain: x\n}\n", "craft/syntax/unknown-page-property"},
	} {
		_, _, diags := syntax.Parse(tc.src)
		if got := flowCodes(diags); got != tc.want {
			t.Errorf("%q: codes = %s, want %s", tc.src, got, tc.want)
		}
	}
}
