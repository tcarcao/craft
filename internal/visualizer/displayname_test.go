package visualizer

import (
	"os"
	"strings"
	"testing"
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
	requireLine(t, "plantuml", puml, `participant "portfolio value" as portfoliovalue`)
	requireLine(t, "plantuml", puml, `participant "invoicearchive" as invoicearchive`)
	requireLine(t, "plantuml", puml, `actor "the seller" as Seller`)
	requireLine(t, "plantuml", puml, `boundary "the hourly clock" as CRON <<scheduler>>`)
	requireLine(t, "mermaid", mmd, `participant nmf_monetization as nMF, the money engine`)
	requireLine(t, "mermaid", mmd, `actor Seller as the seller`)
	requireLine(t, "mermaid", mmd, `participant CRON as the hourly clock (scheduler)`)
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
	requireLine(t, "plantuml", string(p), `frame "nMF, the\nmoney engine" as nmf_monetization`)
	requireLine(t, "plantuml", string(p), `queue "nMF, the money engine events" as nmf_monetization_events`)
	requireLine(t, "mermaid", string(m), `nmf_monetization["nMF, the money engine"]`)
	requireLine(t, "mermaid", string(m), `Seller(["the seller"])`)
}

func TestDisplayNames_ArchitectureAndC4(t *testing.T) {
	doc := resolvedDoc(t, displayNamesDoc(t))
	a, _, err := renderSource(doc, ViewArchitecture, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a), `frame "nMF, the\nmoney engine" as nmf_monetization`) {
		t.Errorf("architecture:\n%s", a)
	}
	c, _, err := renderSource(doc, ViewC4, PlantUML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"nMF, the money engine Domain"`, `Container(nmf_monetization, "nMF, the money engine", "Go Application"`, `Person(Seller, "the seller"`} {
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
