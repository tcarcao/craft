package visualizer

import (
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/plantuml"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// C4GenerationMode selects how services are drawn in a C4 diagram.
type C4GenerationMode string

const (
	C4ModeTransparent C4GenerationMode = "transparent" // one application container per service
	C4ModeBoundaries  C4GenerationMode = "boundaries"  // one container per bounded context
)

// c4View builds the C4 view shared by both backends.
func c4View(doc *craft.CraftDoc, focusServices, focusContexts []string, mode C4GenerationMode, showDatabases bool) view.C4 {
	return view.BuildC4(doc, interaction.Derive(doc), view.C4Options{
		Boundaries:    mode == C4ModeBoundaries,
		ShowDatabases: showDatabases,
		FocusServices: focusServices,
		FocusContexts: focusContexts,
	})
}

// GenerateC4ContainerDiagramWithFocusAndContexts returns PlantUML source for a C4 container diagram.
func GenerateC4ContainerDiagramWithFocusAndContexts(doc *craft.CraftDoc, mode C4GenerationMode, focusServices, focusContexts []string, showDatabases bool) string {
	return plantuml.C4(c4View(doc, focusServices, focusContexts, mode, showDatabases))
}

func (v *Visualizer) GenerateC4(arch *craft.CraftDoc, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	data, _, err := generatePlantUMLWithFormat(GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, nil, nil, showDatabases), FormatPNG)
	return data, err
}

func (v *Visualizer) GenerateC4WithFocusAndContexts(arch *craft.CraftDoc, focusedServiceNames []string, focusedContextNames []string, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	data, _, err := generatePlantUMLWithFormat(GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, focusedServiceNames, focusedContextNames, showDatabases), FormatPNG)
	return data, err
}

func (v *Visualizer) GenerateC4WithFormat(arch *craft.CraftDoc, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	return generatePlantUMLWithFormat(GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, nil, nil, showDatabases), format)
}

func (v *Visualizer) GenerateC4WithFocusContextsAndFormat(arch *craft.CraftDoc, focusedServiceNames []string, focusedContextNames []string, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	return generatePlantUMLWithFormat(GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, focusedServiceNames, focusedContextNames, showDatabases), format)
}
