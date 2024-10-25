package processor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tcarcao/archdsl/internal/parser"
	"github.com/tcarcao/archdsl/internal/visualizer"
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

func (p *Processor) generateDiagrams(arch *parser.Architecture, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Generate C4 diagram
	c4Content, err := p.visualizer.GenerateC4(arch)
	if err != nil {
		return fmt.Errorf("failed to write C4 diagram: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "c4.puml"), []byte(c4Content), 0644); err != nil {
		return fmt.Errorf("failed to write C4 diagram: %v", err)
	}

	// Generate context map
	contextMapContent, err := p.visualizer.GenerateContextMap(arch)
	if err != nil {
		return fmt.Errorf("failed to write context map: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "context-map.dot"), []byte(contextMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write context map: %v", err)
	}

	// Generate sequence diagrams
	seqContent, err := p.visualizer.GenerateSequence(arch)
	if err != nil {
		return fmt.Errorf("failed to write sequence diagrams: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "sequences.puml"), []byte(seqContent), 0644); err != nil {
		return fmt.Errorf("failed to write sequence diagrams: %v", err)
	}

	return nil
}
