package visualizer

import (
	// "fmt"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

// GenerateSequence creates a PlantUML sequence diagram
func (v *Visualizer) GenerateSequence(arch *parser.DSLModel) ([]byte, error) {
	var b strings.Builder

	b.WriteString(`@startuml
!include /opt/plantuml/styles/styles.puml

title System Interactions

`)

	// Collect all participants
	// participants := make(map[string]struct{})
	// eventStreams := make(map[string]struct{})

	// Collect from flows and event handlers
	// for _, sys := range arch.Systems {
	// 	for _, ctx := range sys.Contexts {
	// 		// Collect from flows
	// 		for _, flow := range ctx.Flows {
	// 			participants[flow.Service] = struct{}{}

	// 			for _, action := range flow.Actions {
	// 				if action.Type == "call" {
	// 					participants[action.Target] = struct{}{}
	// 				}
	// 			}
	// 		}

	// 		// Collect from event handlers
	// 		for _, handler := range ctx.EventHandlers {
	// 			for _, action := range handler.Actions {
	// 				if action.Type == "call" {
	// 					participants[action.Target] = struct{}{}
	// 				}
	// 			}
	// 		}

	// 		// Collect event streams
	// 		for _, stream := range ctx.Streams {
	// 			eventStreams[sanitizeStreamName(stream.Topic)] = struct{}{}
	// 		}
	// 	}
	// }

	// // Declare participants
	// b.WriteString("box \"Services\" #LightBlue\n")
	// for participant := range participants {
	// 	b.WriteString(fmt.Sprintf("    participant %s\n", participant))
	// }
	// b.WriteString("end box\n\n")

	// // Declare event streams
	// b.WriteString("box \"Event Streams\" #LightYellow\n")
	// for stream := range eventStreams {
	// 	b.WriteString(fmt.Sprintf("    queue %s\n", stream))
	// }
	// b.WriteString("end box\n\n")

	// // Add flows and their actions
	// for _, sys := range arch.Systems {
	// 	for _, ctx := range sys.Contexts {
	// 		// Handle context flows
	// 		for _, flow := range ctx.Flows {
	// 			// Show API endpoint call
	// 			b.WriteString(fmt.Sprintf("[--> %s: %s %s\n",
	// 				flow.Service,
	// 				flow.Endpoint.Method,
	// 				flow.Endpoint.Path))

	// 			// Show activation
	// 			b.WriteString(fmt.Sprintf("activate %s\n", flow.Service))

	// 			// Process actions
	// 			for _, action := range flow.Actions {
	// 				if action.Type == "call" {
	// 					args := strings.Join(action.Args, ", ")
	// 					if action.Variable != "" {
	// 						b.WriteString(fmt.Sprintf("%s -> %s: %s(%s)\n",
	// 							flow.Service,
	// 							action.Target,
	// 							action.Operation,
	// 							args))
	// 						b.WriteString(fmt.Sprintf("activate %s\n", action.Target))
	// 						b.WriteString(fmt.Sprintf("%s --> %s: %s\n",
	// 							action.Target,
	// 							flow.Service,
	// 							action.Variable))
	// 						b.WriteString(fmt.Sprintf("deactivate %s\n", action.Target))
	// 					} else {
	// 						b.WriteString(fmt.Sprintf("%s -> %s: %s(%s)\n",
	// 							flow.Service,
	// 							action.Target,
	// 							action.Operation,
	// 							args))
	// 					}
	// 				} else if action.Type == "emit" {
	// 					streamName := findStreamForEvent(ctx, action.Event)
	// 					b.WriteString(fmt.Sprintf("%s ->> %s: %s\n",
	// 						flow.Service,
	// 						streamName,
	// 						action.Event))
	// 				}
	// 			}

	// 			b.WriteString(fmt.Sprintf("deactivate %s\n", flow.Service))
	// 			b.WriteString(fmt.Sprintf("[--> %s: response\n\n", flow.Service))
	// 		}

	// 		// Handle event handlers
	// 		for _, handler := range ctx.EventHandlers {
	// 			sourceStream := findStreamForEvent(ctx, handler.Event)

	// 			b.WriteString(fmt.Sprintf("%s -> %s: %s\n",
	// 				sourceStream,
	// 				ctx.Name,
	// 				handler.Event))

	// 			b.WriteString(fmt.Sprintf("activate %s\n", ctx.Name))

	// 			for _, action := range handler.Actions {
	// 				if action.Type == "call" {
	// 					args := strings.Join(action.Args, ", ")
	// 					if action.Variable != "" {
	// 						b.WriteString(fmt.Sprintf("%s -> %s: %s(%s)\n",
	// 							ctx.Name,
	// 							action.Target,
	// 							action.Operation,
	// 							args))
	// 						b.WriteString(fmt.Sprintf("activate %s\n", action.Target))
	// 						b.WriteString(fmt.Sprintf("%s --> %s: %s\n",
	// 							action.Target,
	// 							ctx.Name,
	// 							action.Variable))
	// 						b.WriteString(fmt.Sprintf("deactivate %s\n", action.Target))
	// 					} else {
	// 						b.WriteString(fmt.Sprintf("%s -> %s: %s(%s)\n",
	// 							ctx.Name,
	// 							action.Target,
	// 							action.Operation,
	// 							args))
	// 					}
	// 				} else if action.Type == "emit" {
	// 					targetStream := findStreamForEvent(ctx, action.Event)
	// 					b.WriteString(fmt.Sprintf("%s ->> %s: %s\n",
	// 						ctx.Name,
	// 						targetStream,
	// 						action.Event))
	// 				}
	// 			}

	// 			b.WriteString(fmt.Sprintf("deactivate %s\n\n", ctx.Name))
	// 		}
	// 	}
	// }

	b.WriteString("@enduml")

	return generatePlantUML(b.String())
}

func (v *Visualizer) GenerateSequenceWithFormat(arch *parser.DSLModel, format SupportedFormat) ([]byte, string, error) {
	diagramSource := v.generateSequenceDiagramSource(arch)
	return generatePlantUMLWithFormat(diagramSource, format)
}

// Extract diagram generation logic to be shared between both methods
func (v *Visualizer) generateSequenceDiagramSource(arch *parser.DSLModel) string {
	var b strings.Builder

	b.WriteString(`@startuml
!include /opt/plantuml/styles/styles.puml

title System Interactions

`)

	// Copy the existing logic from the original GenerateSequence method
	// (Simplified for now - the original method should be refactored to call this)
	
	b.WriteString("@enduml")
	return b.String()
}
