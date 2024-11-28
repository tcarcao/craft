package visualizer

import (
	"fmt"
	"github.com/tcarcao/archdsl/internal/parser"
	"log"
	"strings"
)

func (v *Visualizer) GenerateC4(arch *parser.Architecture) ([]byte, error) {
	var b strings.Builder

	b.WriteString(`@startuml
!include /opt/plantuml/styles/styles.puml

LAYOUT_WITH_LEGEND()

`)

	// Generate systems and contexts
	for _, sys := range arch.Systems {
		b.WriteString(fmt.Sprintf("System_Boundary(%s, \"%s\") {\n", sys.Name, sys.Name))

		for _, ctx := range sys.Contexts {
			b.WriteString(fmt.Sprintf("    Container_Boundary(%s, \"%s\") {\n",
				ctx.Name, ctx.Name))

			// Components
			for _, comp := range ctx.Components {
				techTag := ""
				if comp.Tech != nil && comp.Tech.Language != "" {
					techTag = fmt.Sprintf("$tags=\"%s\"", comp.Tech.Language)
				}

				if techTag == "" {
					b.WriteString(fmt.Sprintf("        Component(%s, \"%s\")\n",
						comp.Name, comp.Name))
				} else {
					b.WriteString(fmt.Sprintf("        Component(%s, \"%s\", %s)\n",
						comp.Name, comp.Name, techTag))
				}
			}

			// Services
			for _, svc := range ctx.Services {
				tags := []string{}
				if svc.Tech != nil && svc.Tech.Language != "" {
					tags = append(tags, svc.Tech.Language)
				}
				if svc.Platform != "" {
					tags = append(tags, svc.Platform)
				}

				techTag := ""
				if len(tags) > 0 {
					techTag = fmt.Sprintf("$tags=\"%s\"", strings.Join(tags, "+"))
				}

				if techTag == "" {
					b.WriteString(fmt.Sprintf("        Component(%s_Service, \"%s Service\")\n",
						svc.Name, svc.Name))
				} else {
					b.WriteString(fmt.Sprintf("        Component(%s_Service, \"%s Service\", %s)\n",
						svc.Name, svc.Name, techTag))
				}
			}

			b.WriteString("    }\n")
		}
		b.WriteString("}\n\n")
	}

	// Add relationships
	for _, flow := range arch.Flows {
		if flow.Target != nil {
			b.WriteString(fmt.Sprintf("Rel(%s, %s, \"%s\", \"async\")\n",
				flow.Source,
				flow.Target.Context,
				flow.Operation))
		}
	}

	b.WriteString("@enduml")

	content := b.String()
	log.Printf("Generated PlantUML content:\n%s", content)

	return generatePlantUML(content)
}
