package mermaid

import (
	"fmt"
	"strings"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// C4 emits a Mermaid C4Container (experimental syntax) diagram from doc.
// showDatabases is reserved for parity with the PlantUML side; Mermaid C4
// has no element type richer than Container, so data-stores would render
// as plain Containers if represented.
func C4(doc *craft.CraftDoc, showDatabases bool) (string, error) {
	var b strings.Builder
	b.WriteString("C4Container\n")

	for _, a := range doc.Actors {
		switch a.Type {
		case craft.ActorTypeUser:
			fmt.Fprintf(&b, "    Person(%s, %q, \"\")\n", safeID(a.Name), a.Name)
		case craft.ActorTypeSystem, craft.ActorTypeService:
			fmt.Fprintf(&b, "    System_Ext(%s, %q, \"\")\n", safeID(a.Name), a.Name)
		}
	}

	for _, svc := range doc.Services {
		fmt.Fprintf(&b, "    System_Boundary(%s, %q) {\n", safeID(svc.Name), svc.Name)
		for _, ctx := range svc.Contexts {
			fmt.Fprintf(&b, "        Container(%s, %q, \"\", \"\")\n", safeID(ctx), ctx)
		}
		b.WriteString("    }\n")
	}

	if hasAsyncActions(doc) {
		b.WriteString("    ContainerQueue(Event_Queue, \"Event_Queue\", \"Message Queue\", \"\")\n")
		writeC4EventRels(&b, doc)
	}

	writeC4TriggerRels(&b, doc)

	return b.String(), nil
}

func hasAsyncActions(doc *craft.CraftDoc) bool {
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			for _, act := range sc.Actions {
				if act.Type == craft.ActionTypeAsync {
					return true
				}
			}
		}
	}
	return false
}

func writeC4EventRels(b *strings.Builder, doc *craft.CraftDoc) {
	seen := map[string]bool{}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == craft.TriggerTypeDomainListen {
				key := fmt.Sprintf("EQ->%s|%s", sc.Trigger.Context, sc.Trigger.Event)
				if !seen[key] && sc.Trigger.Context != "" {
					seen[key] = true
					fmt.Fprintf(b, "    Rel(Event_Queue, %s, %q)\n", safeID(sc.Trigger.Context), sc.Trigger.Event)
				}
			}
			for _, act := range sc.Actions {
				if act.Type == craft.ActionTypeAsync && act.Context != "" {
					key := fmt.Sprintf("%s->EQ|%s", act.Context, act.Event)
					if !seen[key] {
						seen[key] = true
						fmt.Fprintf(b, "    Rel(%s, Event_Queue, %q)\n", safeID(act.Context), act.Event)
					}
				}
			}
		}
	}
}

func writeC4TriggerRels(b *strings.Builder, doc *craft.CraftDoc) {
	seen := map[string]bool{}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type != craft.TriggerTypeExternal {
				continue
			}
			actor := sc.Trigger.Actor
			if actor == "" || len(sc.Actions) == 0 {
				continue
			}
			target := sc.Actions[0].Context
			if target == "" {
				continue
			}
			key := actor + "->" + target
			if seen[key] {
				continue
			}
			seen[key] = true
			fmt.Fprintf(b, "    Rel(%s, %s, \"Triggers\")\n", safeID(actor), safeID(target))
		}
	}
}
