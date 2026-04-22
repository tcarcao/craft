package visualizer

import (
	"fmt"

	craft "github.com/tcarcao/craft/pkg/craft"
)

func (v *Visualizer) GenerateC4(arch *craft.CraftDoc, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagram(arch, boundariesMode, showDatabases)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

func (v *Visualizer) GenerateC4WithFocusAndContexts(arch *craft.CraftDoc, focusedServiceNames []string, focusedContextNames []string, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, focusedServiceNames, focusedContextNames, showDatabases)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

// New format-aware methods
func (v *Visualizer) GenerateC4WithFormat(arch *craft.CraftDoc, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagram(arch, boundariesMode, showDatabases)

	fmt.Println(diagram)
	return generatePlantUMLWithFormat(diagram, format)
}

func (v *Visualizer) GenerateC4WithFocusContextsAndFormat(arch *craft.CraftDoc, focusedServiceNames []string, focusedContextNames []string, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocusAndContexts(arch, boundariesMode, focusedServiceNames, focusedContextNames, showDatabases)

	fmt.Println(diagram)
	return generatePlantUMLWithFormat(diagram, format)
}
