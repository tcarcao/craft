package processor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/internal/visualizer"
	craft "github.com/tcarcao/craft/pkg/craft"
)

type Processor struct {
	visualizer *visualizer.Visualizer
}

func New() (*Processor, error) {
	return &Processor{
		visualizer: visualizer.New(),
	}, nil
}

func (p *Processor) ProcessFile(inputPath, outputDir string) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	astFile, _ := syntax.Parse(string(content))
	doc := syntax.Project(astFile)

	if err := p.generateDiagrams(doc, outputDir); err != nil {
		return fmt.Errorf("failed to generate diagrams: %v", err)
	}

	return nil
}

func (p *Processor) generateDiagrams(arch *craft.CraftDoc, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	c4Content, err := p.visualizer.GenerateC4(arch, visualizer.C4ModeBoundaries, true)
	if err != nil {
		return fmt.Errorf("failed to generate C4 diagram: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "c4.png"), c4Content, 0644); err != nil {
		return fmt.Errorf("failed to write C4 diagram: %v", err)
	}

	domainContent, err := p.visualizer.GenerateDomainDiagram(arch)
	if err != nil {
		return fmt.Errorf("failed to generate domain diagram: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "domain.png"), domainContent, 0644); err != nil {
		return fmt.Errorf("failed to write domain diagram: %v", err)
	}

	return nil
}
