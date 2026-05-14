package mermaid

import (
	"fmt"
	"strings"

	craft "github.com/tcarcao/craft/pkg/craft"
)

// Domain emits a Mermaid flowchart LR for a Craft domain diagram.
// If architecture is true, services become subgraphs containing their
// contexts and self-loops are filtered. Otherwise the function emits the
// numbered detailed view: every action contributes a labeled edge.
func Domain(doc *craft.CraftDoc, architecture bool) (string, error) {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	if architecture {
		writeArchitectureSubgraphs(&b, doc)
		writeArchitectureEdges(&b, doc)
		return b.String(), nil
	}

	writeDetailedNodes(&b, doc)
	writeDetailedEdges(&b, doc)
	return b.String(), nil
}

func writeArchitectureSubgraphs(b *strings.Builder, doc *craft.CraftDoc) {
	for _, svc := range doc.Services {
		fmt.Fprintf(b, "    subgraph %s[%q]\n", safeID(svc.Name), svc.Name)
		for _, ctx := range svc.Contexts {
			fmt.Fprintf(b, "        %s[%q]\n", safeID(ctx), ctx)
		}
		b.WriteString("    end\n")
	}
}

func writeArchitectureEdges(b *strings.Builder, doc *craft.CraftDoc) {
	type edge struct{ from, to string }
	seen := map[edge]bool{}
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			for _, act := range sc.Actions {
				if act.Context == "" {
					continue
				}
				if act.Type == craft.ActionTypeSync && act.TargetContext != "" && act.TargetContext != act.Context {
					e := edge{safeID(act.Context), safeID(act.TargetContext)}
					if !seen[e] {
						seen[e] = true
						fmt.Fprintf(b, "    %s --> %s\n", e.from, e.to)
					}
				}
			}
		}
	}
}

func writeDetailedNodes(b *strings.Builder, doc *craft.CraftDoc) {
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		fmt.Fprintf(b, "    %s[%q]\n", safeID(name), name)
	}
	for _, a := range doc.Actors {
		add(a.Name)
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
}

func writeDetailedEdges(b *strings.Builder, doc *craft.CraftDoc) {
	step := 0
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			for _, act := range sc.Actions {
				step++
				switch act.Type {
				case craft.ActionTypeInternal:
					if act.Context != "" {
						fmt.Fprintf(b, "    %s -- \"%d. %s\" --> %s\n",
							safeID(act.Context), step, act.Phrase, safeID(act.Context))
					}
				case craft.ActionTypeSync:
					if act.Context != "" && act.TargetContext != "" {
						fmt.Fprintf(b, "    %s -- \"%d. %s\" --> %s\n",
							safeID(act.Context), step, act.Phrase, safeID(act.TargetContext))
					}
				case craft.ActionTypeAsync:
					if act.Context != "" && act.Event != "" {
						fmt.Fprintf(b, "    %s -- \"%d. notifies %q\" --> %s\n",
							safeID(act.Context), step, act.Event, safeID(act.Context))
					}
				}
			}
		}
	}
}

// safeID renders a Mermaid-safe identifier from a human-readable name.
// Mermaid permits letters, digits, and underscore in node IDs but not
// spaces, hyphens, or emoji. We map disallowed runs to '_'.
func safeID(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prev := true
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		default:
			if !prev {
				b.WriteByte('_')
				prev = true
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}
