package visualizer

import (
	"github.com/tcarcao/craft/v2/internal/visualizer/mermaid"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
)

// GenerateDomainDiagramMermaid renders a Craft domain diagram as Mermaid source.
// Both DomainModeDetailed and DomainModeArchitecture are supported.
func (v *Visualizer) GenerateDomainDiagramMermaid(doc *craft.CraftDoc, mode DomainMode) (string, error) {
	architecture := mode == DomainModeArchitecture
	return mermaid.Domain(doc, architecture)
}

// GenerateSequenceDiagramMermaid renders a Craft sequence diagram as Mermaid source.
func (v *Visualizer) GenerateSequenceDiagramMermaid(doc *craft.CraftDoc, mode DomainMode) (string, error) {
	return mermaid.Sequence(doc)
}

// GenerateC4Mermaid renders a Craft C4 diagram as Mermaid source using the
// experimental c4Diagram syntax. Sprites are dropped (Mermaid C4 doesn't
// support them) — see docs/superpowers/specs/2026-05-14-mermaid-output-mode-design.md
// for the fallback policy.
func (v *Visualizer) GenerateC4Mermaid(doc *craft.CraftDoc, c4Mode C4GenerationMode, showDatabases bool) (string, error) {
	// c4Mode is accepted for parity with the PlantUML side, but Mermaid C4
	// has no boundaries/transparent distinction.
	_ = c4Mode
	return mermaid.C4(doc, showDatabases)
}
