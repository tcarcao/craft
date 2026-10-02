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
	FormatPNG       SupportedFormat = "png"
	FormatSVG       SupportedFormat = "svg"
	FormatPDF       SupportedFormat = "pdf"
	FormatPUML      SupportedFormat = "puml"
	FormatMermaid   SupportedFormat = "mermaid"
	FormatMermaidMD SupportedFormat = "mermaid-md"
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
