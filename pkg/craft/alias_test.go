package craft_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// The public API a consumer such as the hub reads (v2.38): Page.Aliases and
// Flow.Aliases, in source order, trimmed, each once, empty values left out,
// and the `aliases` JSON key.
func TestAliases_PublicAPI(t *testing.T) {
	src := "page account-statistics \"Ad Statistics\" {\n    product_domain: seller-analytics\n    alias: performance\n    alias: \"ad performance\"\n" +
		"    alias: \"\"\n    alias: \"performance \"\n    alias: \"estatísticas do anúncio\"\n}\n" +
		"flow vas-activation \"VAS activation\" {\n    priority: P2\n    owner: \"VAS Excellence\"\n    alias: promotion\n    alias: \"promoted ad\"\n}\n"
	doc, diags, err := craft.Parse("a.craft", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, d := range diags {
		codes = append(codes, d.Code)
	}
	if !slices.Equal(codes, []string{"craft/sema/duplicate-alias", "craft/sema/empty-alias"}) &&
		!slices.Equal(codes, []string{"craft/sema/empty-alias", "craft/sema/duplicate-alias"}) {
		t.Errorf("diagnostics = %v", codes)
	}
	pg := doc.PageByRef("account-statistics")
	if pg == nil || !slices.Equal(pg.Aliases, []string{"performance", "ad performance", "estatísticas do anúncio"}) {
		t.Fatalf("page = %+v", pg)
	}
	fl := doc.FlowByRef("vas-activation")
	if fl == nil || !slices.Equal(fl.Aliases, []string{"promotion", "promoted ad"}) {
		t.Fatalf("flow = %+v", fl)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"aliases":["promotion","promoted ad"]`, `"aliases":["performance","ad performance","estatísticas do anúncio"]`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON lacks %s:\n%s", key, b)
		}
	}
}

// A page or flow without aliases has no aliases key.
func TestAliases_AbsentKeyOmitted(t *testing.T) {
	doc, _, err := craft.Parse("a.craft", []byte("page p \"P\" {\n    product_domain: x\n}\nflow f \"F\" {\n    priority: P1\n    owner: o\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(doc)
	if strings.Contains(string(b), `"aliases"`) {
		t.Errorf("JSON has aliases:\n%s", b)
	}
}
