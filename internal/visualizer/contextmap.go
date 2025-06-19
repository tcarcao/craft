package visualizer

import (
	// "fmt"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

// GenerateContextMap creates a Graphviz context map
func (v *Visualizer) GenerateContextMap(arch *parser.DSLModel) ([]byte, error) {
	var b strings.Builder

	b.WriteString(`digraph ContextMap {
    rankdir=TB;
    
    // Styling
    node [shape=box,style=rounded,fontname="Arial",fontsize=10];
    edge [fontname="Arial",fontsize=8];
    
    // Custom styling for event streams
    node [shape=box,style="rounded,dashed"] [fillcolor="#f0f0f0"];
    
`)

	// Add subgraphs for each system
	// for _, sys := range arch.Systems {
	// 	b.WriteString(fmt.Sprintf("    subgraph cluster_%s {\n", sys.Name))
	// 	b.WriteString(fmt.Sprintf("        label=\"%s\";\n", sys.Name))
	// 	b.WriteString("        style=rounded;\n")
	// 	b.WriteString("        bgcolor=\"#f8f8f8\";\n\n")

	// 	// Add bounded contexts
	// 	for _, ctx := range sys.Contexts {
	// 		// Context node
	// 		b.WriteString(fmt.Sprintf("        %s [label=<<table border='0' cellborder='1' cellspacing='0'>\n", ctx.Name))
	// 		b.WriteString(fmt.Sprintf("            <tr><td port='header' bgcolor='#e0e0e0'><b>%s</b></td></tr>\n", ctx.Name))

	// 		// Add aggregates section if exists
	// 		if len(ctx.Aggregates) > 0 {
	// 			b.WriteString("            <tr><td bgcolor='#f5f5f5'>Aggregates:<br/>\n")
	// 			for _, agg := range ctx.Aggregates {
	// 				b.WriteString(fmt.Sprintf("            • %s<br/>\n", agg))
	// 			}
	// 			b.WriteString("            </td></tr>\n")
	// 		}

	// 		// Add event streams if exists
	// 		if len(ctx.Streams) > 0 {
	// 			b.WriteString("            <tr><td bgcolor='#f5f5f5'>Events:<br/>\n")
	// 			for _, stream := range ctx.Streams {
	// 				b.WriteString(fmt.Sprintf("            • %s (%s)<br/>\n", stream.Topic, stream.Platform))
	// 				for _, event := range stream.Events {
	// 					b.WriteString(fmt.Sprintf("              ↳ %s<br/>\n", event))
	// 				}
	// 			}
	// 			b.WriteString("            </td></tr>\n")
	// 		}

	// 		b.WriteString("        </table>>];\n\n")

	// 		// Add relationships
	// 		for _, rel := range ctx.Relations {
	// 			attrs := []string{}

	// 			// Style based on relationship type
	// 			switch rel.Type {
	// 			case "upstream":
	// 				attrs = append(attrs, "color=\"#2E7D32\"", "fontcolor=\"#2E7D32\"")
	// 			case "downstream":
	// 				attrs = append(attrs, "color=\"#1976D2\"", "fontcolor=\"#1976D2\"")
	// 			}

	// 			// Label with pattern
	// 			attrs = append(attrs, fmt.Sprintf("label=\"%s\\n(%s)\"", rel.Type, rel.Pattern))

	// 			// Style based on pattern
	// 			switch rel.Pattern {
	// 			case "acl":
	// 				attrs = append(attrs, "style=dashed", "penwidth=2.0")
	// 			case "ohs":
	// 				attrs = append(attrs, "style=bold", "penwidth=2.0")
	// 			case "conformist":
	// 				attrs = append(attrs, "style=dotted", "penwidth=1.5")
	// 			}

	// 			b.WriteString(fmt.Sprintf("        %s -> %s [%s];\n",
	// 				ctx.Name,
	// 				rel.Target,
	// 				strings.Join(attrs, ",")))
	// 		}
	// 	}

	// 	b.WriteString("    }\n\n")
	// }

	// // Add event flow relationships between contexts
	// for _, sys := range arch.Systems {
	// 	for _, ctx := range sys.Contexts {
	// 		// Add event handler relationships
	// 		for _, handler := range ctx.EventHandlers {
	// 			// Find source context of the event
	// 			sourceCtx := findContextForEvent(arch, handler.Event)
	// 			if sourceCtx != nil && sourceCtx.Name != ctx.Name {
	// 				b.WriteString(fmt.Sprintf("    %s -> %s [style=dotted,color=\"#FF5722\",label=\"handles %s\"];\n",
	// 					sourceCtx.Name,
	// 					ctx.Name,
	// 					handler.Event))
	// 			}
	// 		}
	// 	}
	// }

	b.WriteString("}\n")

	return generateGraphviz(b.String())
}

// // Helper function to find which context contains an event
// func findContextForEvent(arch *parser.Architecture, eventName string) *parser.Context {
// 	for _, sys := range arch.Systems {
// 		for _, ctx := range sys.Contexts {
// 			for _, stream := range ctx.Streams {
// 				for _, event := range stream.Events {
// 					if event == eventName {
// 						return ctx
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }
