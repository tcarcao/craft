package visualizer

import (
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/plantuml"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// DomainMode represents different visualization modes for domain diagrams
type DomainMode string

const (
	DomainModeDetailed     DomainMode = "detailed"
	DomainModeArchitecture DomainMode = "architecture"
)

// DiagramType represents different types of diagrams
type DiagramType string

const (
	DiagramTypeDomain   DiagramType = "domain"
	DiagramTypeSequence DiagramType = "sequence"
)

func (v *Visualizer) GenerateDomainDiagramWithModeAndFormat(model *craft.CraftDoc, mode DomainMode, format SupportedFormat) ([]byte, string, error) {
	return v.GenerateDomainDiagramWithTypeAndModeAndFormat(model, DiagramTypeDomain, mode, format)
}

// GenerateDomainDiagramWithTypeAndModeAndFormat generates a diagram with specified type, mode, and format
func (v *Visualizer) GenerateDomainDiagramWithTypeAndModeAndFormat(model *craft.CraftDoc, diagramType DiagramType, mode DomainMode, format SupportedFormat) ([]byte, string, error) {
	its := interaction.Derive(model)
	var src string
	switch {
	case diagramType == DiagramTypeSequence:
		src = plantuml.Sequence(view.BuildSequence(model, its))
	case mode == DomainModeArchitecture:
		src = plantuml.Architecture(view.BuildArchitecture(model, its))
	default:
		src = plantuml.Flow(view.BuildFlow(model, its))
	}
	return generatePlantUMLWithFormat(src, format)
}
