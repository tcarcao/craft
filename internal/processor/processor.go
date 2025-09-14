package processor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/internal/visualizer"
)

type Processor struct {
	parser     *parser.Parser
	visualizer *visualizer.Visualizer
}

func New() (*Processor, error) {
	p := parser.NewParser()

	v := visualizer.New()

	return &Processor{
		parser:     p,
		visualizer: v,
	}, nil
}

func (p *Processor) ProcessFile(inputPath, outputDir string) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	arch, err := p.parser.ParseString(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse architecture: %v", err)
	}

	if err := p.generateDiagrams(arch, outputDir); err != nil {
		return fmt.Errorf("failed to generate diagrams: %v", err)
	}

	return nil
}

func (p *Processor) generateDiagrams(arch *parser.DSLModel, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Generate C4 diagram
	c4Content, err := p.visualizer.GenerateC4(arch, visualizer.C4ModeBoundaries)
	if err != nil {
		return fmt.Errorf("failed to generate C4 diagram: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "c4.png"), c4Content, 0644); err != nil {
		return fmt.Errorf("failed to write C4 diagram: %v", err)
	}

	// Generate domain diagram
	domainContent, err := p.visualizer.GenerateDomainDiagram(arch)
	if err != nil {
		return fmt.Errorf("failed to generate domain diagram: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "domain.png"), domainContent, 0644); err != nil {
		return fmt.Errorf("failed to write domain diagram: %v", err)
	}

	return nil
}
