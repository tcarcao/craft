package visualizer

import (
	"fmt"

	"github.com/tcarcao/craft/internal/parser"
)

func (v *Visualizer) GenerateC4(arch *parser.DSLModel, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagram(arch, boundariesMode, showDatabases)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

func (v *Visualizer) GenerateC4WithFocusAndSubDomains(arch *parser.DSLModel, focusedServiceNames []string, focusedSubDomainNames []string, boundariesMode C4GenerationMode, showDatabases bool) ([]byte, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocusAndSubDomains(arch, boundariesMode, focusedServiceNames, focusedSubDomainNames, showDatabases)

	fmt.Println(diagram)
	return generatePlantUML(diagram)
}

// New format-aware methods
func (v *Visualizer) GenerateC4WithFormat(arch *parser.DSLModel, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagram(arch, boundariesMode, showDatabases)

	fmt.Println(diagram)
	return generatePlantUMLWithFormat(diagram, format)
}

func (v *Visualizer) GenerateC4WithFocusSubDomainsAndFormat(arch *parser.DSLModel, focusedServiceNames []string, focusedSubDomainNames []string, boundariesMode C4GenerationMode, showDatabases bool, format SupportedFormat) ([]byte, string, error) {
	fmt.Println(boundariesMode)
	diagram := GenerateC4ContainerDiagramWithFocusAndSubDomains(arch, boundariesMode, focusedServiceNames, focusedSubDomainNames, showDatabases)

	fmt.Println(diagram)
	return generatePlantUMLWithFormat(diagram, format)
}
