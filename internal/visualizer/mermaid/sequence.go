package mermaid

import (
	"fmt"
	"strings"

	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// Sequence emits a Mermaid sequenceDiagram from doc. Each use case becomes a
// `Note over <participants>: <name>` divider; each action becomes a
// sequence arrow. The caller is responsible for filtering doc (e.g. via
// FilterUseCases) before passing in.
func Sequence(doc *craft.CraftDoc) (string, error) {
	var b strings.Builder
	b.WriteString("sequenceDiagram\n")

	participants := collectParticipants(doc)
	for _, p := range participants {
		fmt.Fprintf(&b, "    participant %s\n", p)
	}

	for _, uc := range doc.UseCases {
		if len(participants) > 0 {
			fmt.Fprintf(&b, "    Note over %s,%s: %s\n",
				participants[0], participants[len(participants)-1], uc.Name)
		} else {
			fmt.Fprintf(&b, "    Note over : %s\n", uc.Name)
		}
		for _, sc := range uc.Scenarios {
			writeTrigger(&b, sc.Trigger)
			for _, act := range sc.Actions {
				writeAction(&b, act)
			}
		}
	}

	return b.String(), nil
}

func collectParticipants(doc *craft.CraftDoc) []string {
	seen := map[string]bool{}
	var ordered []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		ordered = append(ordered, name)
	}
	used := referencedActors(doc)
	for _, a := range doc.Actors {
		if used[a.Name] {
			add(a.Name)
		}
	}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			add(sc.Trigger.Actor)
			add(sc.Trigger.Context)
			for _, act := range sc.Actions {
				add(act.Context)
				add(act.TargetContext)
			}
		}
	}
	return ordered
}

func writeTrigger(b *strings.Builder, t craft.Trigger) {
	switch t.Type {
	case craft.TriggerTypeExternal:
		if t.Actor != "" && t.Verb != "" {
			fmt.Fprintf(b, "    %s->>%s: %s %s\n", t.Actor, t.Actor, t.Verb, t.Phrase)
		}
	case craft.TriggerTypeDomainListen:
		if t.Context != "" && t.Event != "" {
			fmt.Fprintf(b, "    [->>%s: %s\n", t.Context, t.Event)
		}
	}
}

func writeAction(b *strings.Builder, a craft.Action) {
	switch a.Type {
	case craft.ActionTypeSync:
		if a.Context != "" && a.TargetContext != "" {
			fmt.Fprintf(b, "    %s->>%s: %s\n", a.Context, a.TargetContext, a.Phrase)
		}
	case craft.ActionTypeAsync:
		if a.Context != "" && a.Event != "" {
			fmt.Fprintf(b, "    %s->>%s: notifies %q\n", a.Context, a.Context, a.Event)
		}
	case craft.ActionTypeInternal:
		if a.Context != "" {
			fmt.Fprintf(b, "    %s->>%s: %s\n", a.Context, a.Context, a.Phrase)
		}
	}
}
