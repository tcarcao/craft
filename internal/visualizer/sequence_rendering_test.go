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
