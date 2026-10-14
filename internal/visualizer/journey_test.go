package visualizer

import (
	"os"
	"strings"
	"testing"

	"github.com/tcarcao/craft/v2/internal/analysis"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

func spineDoc(t *testing.T, extra map[string]string) *craft.CraftDoc {
	t.Helper()
	var rs []analysis.FileResult
	for _, name := range []string{"vas/nmf-books-the-draws.craft", "vas/seller-promotes-an-advert.craft"} {
		src, err := os.ReadFile("testdata/journey-spine/" + name)
		if err != nil {
			t.Fatal(err)
		}
		rs = append(rs, analysis.File(name, string(src)))
	}
	for name, src := range extra {
		rs = append(rs, analysis.File(name, src))
	}
	return analysis.Workspace(rs).Doc
}

const spinePUML = `@startuml
title Journey: seller-promotes-an-advert
skinparam backgroundColor white
skinparam defaultFontName SansSerif
skinparam sequenceMessageAlign center

actor "Seller" as Seller
participant "frontendplatform" as frontendplatform
participant "graphql" as graphql
participant "vasscheduler" as vasscheduler
participant "nmf_monetization" as nmf_monetization

== Seller promotes an advert ==

Seller -> frontendplatform : promotes an advert
frontendplatform -> graphql : to book the promotion [scheduleVas]
graphql -> vasscheduler : to book it [createSchedule]
vasscheduler ->> vasscheduler : notifies "com.olx.realestate_vas.VasScheduled"
ref over vasscheduler : continues: Seller pays at the checkout

== nMF books the draws of a scheduled VAS ==

vasscheduler ->> nmf_monetization : com.olx.realestate_vas.VasScheduled
nmf_monetization -> nmf_monetization : books one draw per day of the schedule

== Seller pays at the checkout ==

Seller -> frontendplatform : pays at the checkout
frontendplatform -> graphql : to pay [payInvoice]

@enduml`

func renderSpine(t *testing.T, doc *craft.CraftDoc, backend Backend, slug string) string {
	t.Helper()
	out, err := New(nil).Render(doc, Request{View: ViewSequence, Backend: backend, Format: FormatSource, Journey: slug})
	if err != nil {
		t.Fatal(err)
	}
	return string(out.Data)
}

func TestJourneySpine(t *testing.T) {
	doc := spineDoc(t, nil)
	if got := renderSpine(t, doc, PlantUML, "seller-promotes-an-advert"); got != spinePUML {
		t.Errorf("got:\n%s", got)
	}
	mmd := renderSpine(t, doc, Mermaid, "seller-promotes-an-advert")
	if !strings.HasPrefix(mmd, "---\ntitle: \"Journey: seller-promotes-an-advert\"\n---\nsequenceDiagram\n") {
		t.Errorf("mermaid head:\n%s", mmd)
	}
	for _, want := range []string{
		"Note over vasscheduler: continues: Seller pays at the checkout",
		"vasscheduler-)nmf_monetization: com.olx.realestate_vas.VasScheduled",
		"Note over Seller,nmf_monetization: nMF books the draws of a scheduled VAS",
	} {
		requireLine(t, "mermaid", mmd, want)
	}
	out, err := New(nil).Render(doc, Request{View: ViewSequence, Backend: Mermaid, Format: FormatMarkdown, Journey: "seller-promotes-an-advert"})
	if err != nil {
		t.Fatal(err)
	}
	if md := string(out.Data); !strings.HasPrefix(md, "# Journey: seller-promotes-an-advert\n\n```mermaid\n---\ntitle: \"Journey: seller-promotes-an-advert\"\n---\nsequenceDiagram\n") {
		t.Errorf("markdown head:\n%s", md)
	}
}

func TestJourneySpine_RequestChecks(t *testing.T) {
	doc := spineDoc(t, nil)
	for _, req := range []Request{
		{View: ViewFlow, Backend: PlantUML, Format: FormatSource, Journey: "seller-promotes-an-advert"},
		{View: ViewSequence, Backend: PlantUML, Format: FormatSource, Journey: "seller-promotes-an-advert", UseCases: []string{"x"}},
		{View: ViewSequence, Backend: PlantUML, Format: FormatSource, Journey: "nope"},
	} {
		if _, err := New(nil).Render(doc, req); err == nil {
			t.Errorf("expected an error for %+v", req)
		}
	}
}

// A publisher outside the spine is not on the shared stage: its delivery
// keeps the inbound arrow, naming it. One on stage but not publishing the
// event earlier in the spine does not deliver it either.
func TestJourneySpine_PublisherOutsideTheSpine(t *testing.T) {
	doc := spineDoc(t, map[string]string{"vas/backoffice.craft": `use_case "Agent schedules a VAS" {
    when Agent schedules a VAS
        graphql notifies com.olx.realestate_vas.VasScheduled
}
`})
	puml := renderSpine(t, doc, PlantUML, "seller-promotes-an-advert")
	requireLine(t, "plantuml", puml, "vasscheduler ->> nmf_monetization : com.olx.realestate_vas.VasScheduled")
	requireLine(t, "plantuml", puml, "[-> nmf_monetization : com.olx.realestate_vas.VasScheduled (from graphql)")
	forbidLine(t, "plantuml", puml, "graphql ->> nmf_monetization : com.olx.realestate_vas.VasScheduled")
}

// A cycle renders its members in source order (file, then line).
func TestJourneySpine_Cycle(t *testing.T) {
	doc := spineDoc(t, map[string]string{
		"loop/b.craft": `use_case "B" {
    tags {
        journey: ping-pong
    }
    when pong listens e.Ping
        pong notifies e.Pong
}
`,
		"loop/a.craft": `use_case "A" {
    tags {
        journey: ping-pong
    }
    when ping listens e.Pong
        ping notifies e.Ping
}
`})
	puml := renderSpine(t, doc, PlantUML, "ping-pong")
	a, b := strings.Index(puml, "== A =="), strings.Index(puml, "== B ==")
	if a < 0 || b < a {
		t.Fatalf("sections out of order:\n%s", puml)
	}
	// A is drawn first, so B's listen comes straight from ping; A's own
	// listen came before any pong was drawn, so it arrives from outside.
	requireLine(t, "plantuml", puml, "ping ->> pong : e.Ping")
	requireLine(t, "plantuml", puml, "[-> ping : e.Pong (from pong)")
}
