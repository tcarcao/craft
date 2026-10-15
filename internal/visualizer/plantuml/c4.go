package plantuml

import (
	"fmt"
	"strings"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// languageIncludes maps language sprite names to their tupadr3 stdlib paths.
// `rust` lives under devicons/: devicons2/rust does not exist in the PlantUML
// stdlib and breaks renderers >= v1.2025.x.
var languageIncludes = map[string]string{
	"go":         "tupadr3/devicons2/go",
	"java":       "tupadr3/devicons2/java",
	"python":     "tupadr3/devicons2/python",
	"nodejs":     "tupadr3/devicons2/nodejs",
	"javascript": "tupadr3/devicons2/javascript",
	"rust":       "tupadr3/devicons/rust",
	"dot_net":    "tupadr3/devicons2/dot_net",
	"php":        "tupadr3/devicons2/php",
	"ruby":       "tupadr3/devicons2/ruby",
	"kotlin":     "tupadr3/devicons2/kotlin",
	"swift":      "tupadr3/devicons2/swift",
}

// LanguageIncludes exposes the sprite → include table for render tests.
func LanguageIncludes() map[string]string { return languageIncludes }

// C4 writes a C4 container diagram with the PlantUML C4 stdlib.
func C4(c view.C4) string {
	var b strings.Builder
	al := newAliases()
	b.WriteString("@startuml\n!include <C4/C4_Container.puml>\n")
	for _, inc := range []string{"tupadr3/devicons/database", "tupadr3/devicons2/postgresql", "tupadr3/devicons/mysql", "tupadr3/devicons2/redis", "tupadr3/devicons2/mongodb"} {
		fmt.Fprintf(&b, "!include <%s>\n", inc)
	}
	for _, lang := range c.Languages {
		if inc, ok := languageIncludes[lang]; ok {
			fmt.Fprintf(&b, "!include <%s>\n", inc)
		}
	}
	for _, inc := range []string{"code", "mobile", "globe", "shield_alt", "list"} {
		fmt.Fprintf(&b, "!include <tupadr3/font-awesome-5/%s>\n", inc)
	}
	fmt.Fprintf(&b, "\n\nLAYOUT_WITH_LEGEND()\n\ntitle %s\n\n", c.Title)

	for _, a := range c.Actors {
		element := "Person"
		if a.Element == view.C4ExternalSystem {
			element = "System_Ext"
		}
		fmt.Fprintf(&b, "%s(%s, \"%s\", \"%s\")\n", element, al.declare(a.ID, a.Label), text(a.Text()), text(a.Description))
	}
	if len(c.Actors) > 0 {
		b.WriteString("\n")
	}

	for _, sys := range c.Systems {
		boundary := "System_Boundary"
		if sys.External {
			boundary = "Enterprise_Boundary"
		}
		fmt.Fprintf(&b, "%s(%s, \"%s\") {\n", boundary, al.declare("system:"+sys.ID, sys.Label+" boundary"), text(sys.Label))
		for _, g := range sys.Groups {
			fmt.Fprintf(&b, "    Container_Boundary(%s, \"%s\") {\n", al.declare("group:"+g.ID, sys.Label+" "+g.Label+" boundary"), text(g.Text()))
			for _, ct := range g.Containers {
				container(&b, "        ", al, ct, sys.External)
			}
			b.WriteString("    }\n")
		}
		if len(sys.Groups) > 0 && len(sys.Containers) > 0 {
			b.WriteString("    ' Data Layer\n")
		}
		for _, ct := range sys.Containers {
			container(&b, "    ", al, ct, sys.External)
		}
		b.WriteString("}\n\n")
	}

	for _, r := range c.Relations {
		if r.Technology == "" {
			fmt.Fprintf(&b, "Rel(%s, %s, \"%s\")\n", al.of(r.From), al.of(r.To), text(r.Label))
			continue
		}
		fmt.Fprintf(&b, "Rel(%s, %s, \"%s\", \"%s\")\n", al.of(r.From), al.of(r.To), text(r.Label), text(r.Technology))
	}
	b.WriteString("\n@enduml")
	return b.String()
}

func container(b *strings.Builder, indent string, al *aliases, c view.C4Container, external bool) {
	element := map[view.C4ContainerKind]string{view.C4App: "Container", view.C4Database: "ContainerDb", view.C4Queue: "ContainerQueue"}[c.Kind]
	if external {
		element += "_Ext"
	}
	sprite := ""
	if c.Icon != "" {
		sprite = fmt.Sprintf(", $sprite=\"%s\"", c.Icon)
	}
	fmt.Fprintf(b, "%s%s(%s, \"%s\", \"%s\", \"%s\"%s)\n", indent, element, al.declare(c.ID, c.Label),
		text(c.Text()), text(c.Technology), text(c.Description), sprite)
}
