// internal/visualizer/visualizer.go
package visualizer

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Visualizer struct{}

func New() *Visualizer {
	return &Visualizer{}
}

// SupportedFormat represents supported output formats
type SupportedFormat string

const (
	FormatPNG  SupportedFormat = "png"
	FormatSVG  SupportedFormat = "svg"
	FormatPDF  SupportedFormat = "pdf"
	FormatPUML SupportedFormat = "puml"
)

// GeneratePlantUMLWithFormat generates PlantUML diagram in specified format
func generatePlantUMLWithFormat(content string, format SupportedFormat) ([]byte, string, error) {
	switch format {
	case FormatPUML:
		// Return raw PlantUML source
		return []byte(content), "text/plain", nil
	case FormatPNG:
		return generatePlantUMLBinary(content, "png")
	case FormatSVG:
		return generatePlantUMLBinary(content, "svg")
	case FormatPDF:
		return generatePlantUMLBinary(content, "pdf")
	default:
		return generatePlantUMLBinary(content, "png")
	}
}

func generatePlantUMLBinary(content string, format string) ([]byte, string, error) {
	cmd := exec.Command("plantuml", "-pipe", "-t"+format)
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("plantuml error: %v, stderr: %s", err, stderr.String())
	}
	
	var contentType string
	switch format {
	case "png":
		contentType = "image/png"
	case "svg":
		contentType = "image/svg+xml"
	case "pdf":
		contentType = "application/pdf"
	default:
		contentType = "application/octet-stream"
	}
	
	return out, contentType, nil
}

// Legacy function for backward compatibility
func generatePlantUML(content string) ([]byte, error) {
	data, _, err := generatePlantUMLBinary(content, "png")
	return data, err
}

func generateGraphviz(content string) ([]byte, error) {
	cmd := exec.Command("dot", "-Tpng")
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("graphviz error: %v, stderr: %s", err, stderr.String())
	}
	return out, nil
}

// GenerateGraphvizWithFormat generates Graphviz diagram in specified format
func generateGraphvizWithFormat(content string, format SupportedFormat) ([]byte, string, error) {
	switch format {
	case FormatPUML:
		// Return raw Graphviz source (not applicable, but for consistency)
		return []byte(content), "text/plain", nil
	case FormatPNG:
		return generateGraphvizBinary(content, "png")
	case FormatSVG:
		return generateGraphvizBinary(content, "svg")
	case FormatPDF:
		return generateGraphvizBinary(content, "pdf")
	default:
		return generateGraphvizBinary(content, "png")
	}
}

func generateGraphvizBinary(content string, format string) ([]byte, string, error) {
	cmd := exec.Command("dot", "-T"+format)
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("graphviz error: %v, stderr: %s", err, stderr.String())
	}
	
	var contentType string
	switch format {
	case "png":
		contentType = "image/png"
	case "svg":
		contentType = "image/svg+xml"
	case "pdf":
		contentType = "application/pdf"
	default:
		contentType = "application/octet-stream"
	}
	
	return out, contentType, nil
}
