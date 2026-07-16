package craft_test

import (
	"reflect"
	"testing"

	"github.com/tcarcao/craft/v2/pkg/craft"
)

func TestGlossaryVerbs(t *testing.T) {
	if got := craft.GlossaryVerbs(); !reflect.DeepEqual(got, []string{"same_as", "contrasts", "distinct_from"}) {
		t.Fatalf("GlossaryVerbs() = %v", got)
	}
}

func TestParse_ExposesGlossary(t *testing.T) {
	doc, _, _ := craft.Parse("f.craft", []byte("glossary { billing/Invoice same_as subscriptions/Invoice }\n"))
	if len(doc.Glossary) != 1 || doc.Glossary[0].Verb != "same_as" {
		t.Fatalf("glossary not exposed: %+v", doc.Glossary)
	}
}
