package visualizer

import (
	"github.com/tcarcao/craft/v2/internal/interaction"
	"github.com/tcarcao/craft/v2/internal/visualizer/mermaid"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// GenerateDomainDiagramMermaid renders a Craft domain diagram as Mermaid source.
// Both DomainModeDetailed and DomainModeArchitecture are supported.
func (v *Visualizer) GenerateDomainDiagramMermaid(doc *craft.CraftDoc, mode DomainMode) (string, error) {
	its := interaction.Derive(doc)
	if mode == DomainModeArchitecture {
		return mermaid.Architecture(view.BuildArchitecture(doc, its)), nil
	}
	return mermaid.Flow(view.BuildFlow(doc, its)), nil
}

// GenerateSequenceDiagramMermaid renders a Craft sequence diagram as Mermaid source.
func (v *Visualizer) GenerateSequenceDiagramMermaid(doc *craft.CraftDoc, mode DomainMode) (string, error) {
	return mermaid.Sequence(view.BuildSequence(doc, interaction.Derive(doc))), nil
}

// GenerateC4Mermaid renders a C4 container diagram as Mermaid source. It draws
// the same view as the PlantUML side — focus, boundaries mode and databases
// included — without sprites, which Mermaid C4 cannot show.
func (v *Visualizer) GenerateC4Mermaid(doc *craft.CraftDoc, focusServices, focusContexts []string, c4Mode C4GenerationMode, showDatabases bool) (string, error) {
	return mermaid.C4(c4View(doc, focusServices, focusContexts, c4Mode, showDatabases)), nil
}
