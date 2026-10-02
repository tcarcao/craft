package mermaid

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// C4 writes a C4 container diagram with Mermaid's C4Container syntax. It draws
// the same view as the PlantUML writer; Mermaid has no sprites, so icons are
// left out.
func C4(c view.C4) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("C4Container\n")
	fmt.Fprintf(&b, "    title %s\n", c.Title)
	for _, a := range c.Actors {
		element := "Person"
		if a.Element == view.C4ExternalSystem {
			element = "System_Ext"
		}
		fmt.Fprintf(&b, "    %s(%s, \"%s\", \"%s\")\n", element, al.declare(a.ID, a.Label), text(a.Label), text(a.Description))
	}
	for _, sys := range c.Systems {
		boundary := "System_Boundary"
		if sys.External {
			boundary = "Enterprise_Boundary"
		}
		fmt.Fprintf(&b, "    %s(%s, \"%s\") {\n", boundary, al.declare("system:"+sys.ID, sys.Label+" boundary"), text(sys.Label))
		for _, g := range sys.Groups {
			fmt.Fprintf(&b, "        Container_Boundary(%s, \"%s\") {\n", al.declare("group:"+g.ID, sys.Label+" "+g.Label+" boundary"), text(g.Label))
			for _, ct := range g.Containers {
				c4Container(&b, "            ", al, ct, sys.External)
			}
			b.WriteString("        }\n")
		}
		for _, ct := range sys.Containers {
			c4Container(&b, "        ", al, ct, sys.External)
		}
		b.WriteString("    }\n")
	}
	for _, r := range c.Relations {
		if r.Technology == "" {
			fmt.Fprintf(&b, "    Rel(%s, %s, \"%s\")\n", al.of(r.From), al.of(r.To), text(r.Label))
			continue
		}
		fmt.Fprintf(&b, "    Rel(%s, %s, \"%s\", \"%s\")\n", al.of(r.From), al.of(r.To), text(r.Label), text(r.Technology))
	}
	return b.String()
}

func c4Container(b *strings.Builder, indent string, al *aliases, c view.C4Container, external bool) {
	element := map[view.C4ContainerKind]string{view.C4App: "Container", view.C4Database: "ContainerDb", view.C4Queue: "ContainerQueue"}[c.Kind]
	if external {
		element += "_Ext"
	}
	fmt.Fprintf(b, "%s%s(%s, \"%s\", \"%s\", \"%s\")\n", indent, element, al.declare(c.ID, c.Label), text(c.Label), text(c.Technology), text(c.Description))
}
