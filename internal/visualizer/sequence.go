package visualizer

import (
	"fmt"
	"strings"

	"github.com/tcarcao/archdsl/internal/parser"
)

// GenerateSequence creates a PlantUML sequence diagram
func (v *Visualizer) GenerateSequence(arch *parser.Architecture) ([]byte, error) {
	var b strings.Builder

	b.WriteString(`@startuml
title System Interactions

`)

	// Collect all participants
	participants := make(map[string]bool)
	for _, flow := range arch.Flows {
		participants[flow.Source] = true
		if flow.Target != nil {
			participants[flow.Target.Context] = true
		}
	}

	// Declare participants
	for participant := range participants {
		b.WriteString(fmt.Sprintf("participant %s\n", participant))
	}
	b.WriteString("\n")

	// Add flows
	for _, flow := range arch.Flows {
		if flow.Target != nil {
			args := strings.Join(flow.Args, ", ")
			b.WriteString(fmt.Sprintf("%s -> %s: %s(%s)\n",
				flow.Source,
				flow.Target.Context,
				flow.Operation,
				args))
		}
	}

	b.WriteString("@enduml")

	return generatePlantUML(b.String())
}
