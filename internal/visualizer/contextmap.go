package visualizer

import (
	"fmt"
	"github.com/tcarcao/archdsl/internal/parser"
	"strings"
)

// GenerateContextMap creates a Graphviz context map
func (v *Visualizer) GenerateContextMap(arch *parser.Architecture) ([]byte, error) {
	var b strings.Builder

	b.WriteString(`digraph ContextMap {
    rankdir=TB;
    node [shape=box,style=rounded];
    
`)

	// Add bounded contexts
	for _, sys := range arch.Systems {
		for _, ctx := range sys.Contexts {
			// Node definition with system name
			b.WriteString(fmt.Sprintf("    %s [label=\"%s\\n(%s)\"];\n",
				ctx.Name, ctx.Name, sys.Name))

			// Add relationships
			for _, rel := range ctx.Relations {
				attrs := []string{fmt.Sprintf("label=\"%s\"", rel.Type)}

				if rel.Pattern != "" {
					attrs = append(attrs, "style=dashed")
					attrs = append(attrs, fmt.Sprintf("headlabel=\"%s\"", rel.Pattern))
				}

				b.WriteString(fmt.Sprintf("    %s -> %s [%s];\n",
					ctx.Name,
					rel.Target,
					strings.Join(attrs, ",")))
			}
		}
	}

	b.WriteString("}\n")

	return generateGraphviz(b.String())
}
