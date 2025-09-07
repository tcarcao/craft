package visualizer

import (
	"fmt"

	"github.com/tcarcao/archdsl/internal/parser"
)

func (v *Visualizer) GenerateC4(arch *parser.DSLModel) ([]byte, error) {
	diagram := GenerateC4ContainerDiagram(arch, C4ModeBoundaries)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

func (v *Visualizer) GenerateC4WithFocus(arch *parser.DSLModel, focusedServiceNames []string) ([]byte, error) {
	diagram := GenerateC4ContainerDiagramWithFocus(arch, C4ModeBoundaries, focusedServiceNames)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}
