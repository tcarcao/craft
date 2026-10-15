package visualizer

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

func displayNamesDoc(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("testdata/display-names.craft")
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

// A display name replaces the text the reader sees; the diagram identifier
// stays the id.
func TestDisplayNames_Sequence(t *testing.T) {
	puml, mmd := sequenceSource(t, displayNamesDoc(t))
	requireLine(t, "plantuml", puml, `participant "nMF, the money engine" as nmf_monetization`)
	requireLine(t, "plantuml", puml, `participant "portfolio value,\nrecomputed on every\nprice change" as portfoliovalue`)
	requireLine(t, "plantuml", puml, `participant "invoicearchive" as invoicearchive`)
	requireLine(t, "plantuml", puml, `actor "the seller" as Seller`)
	requireLine(t, "plantuml", puml, `boundary "the hourly clock that\nrebuilds the prices" as CRON <<scheduler>>`)
	requireLine(t, "mermaid", mmd, `participant nmf_monetization as nMF, the money engine`)
	requireLine(t, "mermaid", mmd, `actor Seller as the seller`)
	requireLine(t, "mermaid", mmd, `participant CRON as the hourly clock that<br/>rebuilds the prices (scheduler)`)
	requireLine(t, "mermaid", mmd, `participant portfoliovalue as portfolio value,<br/>recomputed on every<br/>price change`)
}

// Mermaid reads '#' in a sequence participant as the start of a comment and
// ';' as the end of a statement, so the name would be cut silently. Both are
// replaced by look-alikes; PlantUML keeps them.
func TestDisplayNames_SequenceHashAndSemicolon(t *testing.T) {
	puml, mmd := sequenceSource(t, displayNamesDoc(t))
	requireLine(t, "plantuml", puml, `boundary "price project #2; EU" as PriceProject`)
	requireLine(t, "mermaid", mmd, "actor PriceProject as price project \uff032, EU")
}

// In a quoted Mermaid label, "#<word>;" is an entity code: "#2;" would be
// drawn as the control character 2. The '#' is written as "#35;".
func TestDisplayNames_FlowHashEntity(t *testing.T) {
	m, _, err := renderSource(resolvedDoc(t, displayNamesDoc(t)), ViewFlow, Mermaid)
	if err != nil {
		t.Fatal(err)
	}
	requireLine(t, "mermaid", string(m), `PriceProject(["price project #35;2; EU"])`)
}

func TestDisplayNames_Flow(t *testing.T) {
	doc := resolvedDoc(t, displayNamesDoc(t))
	p, _, err := renderSource(doc, ViewFlow, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := renderSource(doc, ViewFlow, Mermaid)
	if err != nil {
		t.Fatal(err)
	}
	requireLine(t, "plantuml", string(p), `frame "nMF, the money engine" as nmf_monetization`)
	requireLine(t, "plantuml", string(p), `frame "portfolio value,\nrecomputed on every\nprice change" as portfoliovalue`)
	requireLine(t, "plantuml", string(p), `boundary "the hourly clock that\nrebuilds the prices" as CRON <<scheduler>>`)
	requireLine(t, "plantuml", string(p), `queue "nMF, the money engine\nevents" as nmf_monetization_events`)
	requireLine(t, "mermaid", string(m), `nmf_monetization["nMF, the money engine"]`)
	requireLine(t, "mermaid", string(m), `Seller(["the seller"])`)
	requireLine(t, "mermaid", string(m), `portfoliovalue["portfolio value,<br/>recomputed on every<br/>price change"]`)
}

func TestDisplayNames_ArchitectureAndC4(t *testing.T) {
	doc := resolvedDoc(t, displayNamesDoc(t))
	a, _, err := renderSource(doc, ViewArchitecture, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a), `frame "nMF, the money engine" as nmf_monetization`) ||
		!strings.Contains(string(a), `frame "portfolio value,\nrecomputed on every\nprice change" as portfoliovalue`) {
		t.Errorf("architecture:\n%s", a)
	}
	c, _, err := renderSource(doc, ViewC4, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"nMF, the money engine\nDomain"`, `Container(nmf_monetization, "nMF, the money engine", "Go Application"`, `Person(Seller, "the seller"`,
		`Container(portfoliovalue, "portfolio value,\nrecomputed on every\nprice change", "PHP Application"`,
		`"portfolio value,\nrecomputed on every\nprice change Domain"`} {
		if !strings.Contains(string(c), want) {
			t.Errorf("c4 lacks %s:\n%s", want, c)
		}
	}
}

func TestDisplayNames_SurfaceHeading(t *testing.T) {
	doc := resolvedDoc(t, displayNamesDoc(t))
	out, err := New(nil).Render(doc, Request{View: ViewSurface, Backend: PlantUML, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out.Data), "\n## nMF, the money engine (nmf_monetization)\n") {
		t.Errorf("surface:\n%s", out.Data)
	}
}

// An inbound arrow names its publishers by display name, in id order: here
// the id order (alpha, mid, zeta) is not the display-name order.
func TestDisplayNames_InboundFromLabel(t *testing.T) {
	puml, mmd := sequenceSource(t, `domains {
    billing {
        zeta "Alpha pricing"
        alpha "Zulu quotes"
        mid "mid #3"
        listener "the listener"
    }
}

use_case "Zeta publishes" {
    when CRON fires
        zeta notifies pricing.PricesChanged
}

use_case "Alpha publishes" {
    when CRON fires
        alpha notifies pricing.PricesChanged
        mid notifies pricing.PricesChanged
}

use_case "Listener reacts" {
    when listener listens pricing.PricesChanged
        listener recomputes the prices
}
`)
	requireLine(t, "plantuml", puml, "[-> listener : pricing.PricesChanged (from Zulu quotes, mid #3, Alpha pricing)")
	// Mermaid message text ends at '#' too.
	requireLine(t, "mermaid", mmd, "External->>listener: pricing.PricesChanged (from Zulu quotes, mid \uff033, Alpha pricing)")
}

// In a journey spine, a publisher outside the spine keeps the inbound arrow,
// named by its display name.
func TestDisplayNames_JourneyInboundFromLabel(t *testing.T) {
	doc := spineDoc(t, map[string]string{
		"registry.craft": `domains {
    vas {
        graphql "the GraphQL gateway"
        nmf_monetization "nMF, the money engine"
    }
}
`,
		"vas/backoffice.craft": `use_case "Agent schedules a VAS" {
    when Agent schedules a VAS
        graphql notifies com.olx.realestate_vas.VasScheduled
}
`})
	puml := renderSpine(t, doc, PlantUML, "seller-promotes-an-advert")
	requireLine(t, "plantuml", puml, "[-> nmf_monetization : com.olx.realestate_vas.VasScheduled (from the GraphQL gateway)")
}

// Mermaid's C4 renderer decodes no entity codes, so "#quot;" would be drawn
// literally: a '"' becomes "'", as in PlantUML, and '#' stays as written.
func TestDisplayNames_C4MermaidQuotes(t *testing.T) {
	doc := resolvedDoc(t, `actors {
    user Seller "the \"best\" seller #1;"
}

domains {
    billing {
        nmf "nMF, \"the\" engine #2;"
    }
}

services {
    money {
        contexts: nmf
        language: golang
    }
}

use_case "Seller pays" {
    when Seller pays the invoice
        nmf records the payment
}
`)
	out, err := New(nil).Render(doc, Request{View: ViewC4, Backend: Mermaid, Format: FormatSource, C4: &C4Options{Boundaries: true}})
	if err != nil {
		t.Fatal(err)
	}
	m := string(out.Data)
	requireLine(t, "mermaid", m, `Person(Seller, "the 'best' seller #1;", "External user")`)
	requireLine(t, "mermaid", m, `Container_Boundary(money_nmf_Domain_boundary, "nMF, 'the' engine #2;<br/>Domain") {`)
	requireLine(t, "mermaid", m, `Container(nmf, "nMF, 'the' engine #2;", "Go Application", "nmf [Context]")`)
	if strings.Contains(m, "#quot;") || strings.Contains(m, "#35;") {
		t.Errorf("entity code in C4:\n%s", m)
	}
}

// A long display name wraps at view.NameWidth on word boundaries; a word
// longer than the width keeps its own line, and ids never wrap.
func TestWrapName(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"nMF, the money engine", []string{"nMF, the money engine"}},
		{"portfolio value, recomputed on every price change", []string{"portfolio value,", "recomputed on every", "price change"}},
		{"a supercalifragilisticexpialidocious-engine x", []string{"a", "supercalifragilisticexpialidocious-engine", "x"}},
		{"exactly twenty-four chars", []string{"exactly twenty-four", "chars"}},
		{"", []string{""}},
	} {
		if got := view.WrapName(c.in); !slices.Equal(got, c.want) {
			t.Errorf("WrapName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Ids are never wrapped: a long id without a display name is drawn as is.
func TestDisplayNames_LongIdNotWrapped(t *testing.T) {
	puml, mmd := sequenceSource(t, `use_case "Long ids" {
    when Seller opens the page
        a_very_long_bounded_context_identifier_name asks another_very_long_bounded_context_identifier to do it
}
`)
	requireLine(t, "plantuml", puml, `participant "a_very_long_bounded_context_identifier_name" as a_very_long_bounded_context_identifier_name`)
	requireLine(t, "mermaid", mmd, `participant a_very_long_bounded_context_identifier_name as a_very_long_bounded_context_identifier_name`)
}
