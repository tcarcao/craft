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

func generatePlantUML(content string) ([]byte, error) {
	cmd := exec.Command("plantuml", "-pipe", "-tpng")
	cmd.Stdin = strings.NewReader(content)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plantuml error: %v, stderr: %s", err, stderr.String())
	}
	return out, nil
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
