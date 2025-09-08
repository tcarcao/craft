package visualizer

import (
	"fmt"

	"github.com/tcarcao/archdsl/internal/parser"
)

func (v *Visualizer) GenerateC4(arch *parser.DSLModel, boundariesMode C4GenerationMode) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagram(arch, boundariesMode)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

func (v *Visualizer) GenerateC4WithFocus(arch *parser.DSLModel, focusedServiceNames []string, boundariesMode C4GenerationMode) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocus(arch, boundariesMode, focusedServiceNames)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

func (v *Visualizer) GenerateC4WithFocusAndSubDomains(arch *parser.DSLModel, focusedServiceNames []string, focusedSubDomainNames []string, boundariesMode C4GenerationMode) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocusAndSubDomains(arch, boundariesMode, focusedServiceNames, focusedSubDomainNames)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}
