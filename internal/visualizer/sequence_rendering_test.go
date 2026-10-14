package visualizer

import (
	"strings"
	"testing"
)

// sequenceSource draws src as a sequence diagram in both backends.
func sequenceSource(t *testing.T, src string) (puml, mmd string) {
	t.Helper()
	doc := resolvedDoc(t, src)
	p, _, err := renderSource(doc, ViewSequence, PlantUML)
	if err != nil {
		t.Fatalf("plantuml: %v", err)
	}
	m, _, err := renderSource(doc, ViewSequence, Mermaid)
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	return string(p), string(m)
}

func requireLine(t *testing.T, backend, out, want string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == want {
			return
		}
	}
	t.Errorf("%s output has no line %q:\n%s", backend, want, out)
}

func forbidLine(t *testing.T, backend, out, unwanted string) {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimSpace(l) == unwanted {
			t.Errorf("%s output has line %q:\n%s", backend, unwanted, out)
		}
	}
}

// When the actor that triggers a scenario also takes its first step, the
// trigger is what the actor does, not a call the actor makes to itself: it is
// drawn as a note over the actor.
func TestSequence_TriggerOnTheActorItselfIsANote(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Advisor
domain obido {
    concierge
}
use_case "Advisor reads a seeker file" {
    when Advisor opens a seeker file
        Advisor asks concierge for one seeker file
}
`)
	forbidLine(t, "plantuml", puml, "Advisor -> Advisor : opens a seeker file")
	requireLine(t, "plantuml", puml, "note over Advisor : opens a seeker file")
	requireLine(t, "mermaid", mmd, "Note over Advisor: opens a seeker file")
}

// An action's operation annotation is the wire contract of that step, so the
// sequence diagram shows it after the label.
func TestSequence_OperationAnnotationIsDrawn(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Seeker
domain web {
    frontendplatform
    graphql
}
use_case "Seeker opens an advert" {
    when Seeker opens an advert
        frontendplatform asks graphql for the advert  [ad]
        graphql notifies "AdvertViewed"                [TOPIC web.advert-viewed]
}
`)
	requireLine(t, "plantuml", puml, "frontendplatform -> graphql : for the advert [ad]")
	requireLine(t, "mermaid", mmd, "frontendplatform->>graphql: for the advert [ad]")
	requireLine(t, "plantuml", puml, `graphql ->> graphql : notifies "AdvertViewed" [TOPIC web.advert-viewed]`)
}

// A use case with several scenarios frames each one, numbered and titled by
// its trigger, so alternatives do not read as one long chain. A use case with
// a single scenario is drawn without a frame.
func TestSequence_ScenariosAreFramed(t *testing.T) {
	puml, mmd := sequenceSource(t, `actor user Customer
domain shop {
    Ordering
    Billing
}
use_case "Checkout" {
    when Customer places an order
        Ordering asks Billing to charge the card
    when Customer places an order with an empty basket
        Ordering rejects the order
}
use_case "Single" {
    when Customer cancels an order
        Ordering asks Billing to refund the card
}
`)
	requireLine(t, "plantuml", puml, "group 1: when Customer places an order")
	requireLine(t, "plantuml", puml, "group 2: when Customer places an order with an empty basket")
	if n := strings.Count(puml, "\nend\n"); n != 2 {
		t.Errorf("plantuml: %d group ends, want 2:\n%s", n, puml)
	}
	forbidLine(t, "plantuml", puml, "group 1: when Customer cancels an order")
	if !strings.Contains(mmd, ": 2: when Customer places an order with an empty basket") || strings.Count(mmd, "    rect ") != 2 {
		t.Errorf("mermaid scenarios not framed:\n%s", mmd)
	}
}
