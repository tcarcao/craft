package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tcarcao/archdsl/internal/processor"
)

func main() {
	inputFile := flag.String("input", "", "Input DSL file path")
	outputDir := flag.String("output", "", "Output directory for generated diagrams")

	flag.Parse()

	if *inputFile == "" || *outputDir == "" {
		fmt.Println("Usage: archdsl -input <dsl-file> -output <output-dir>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	proc, err := processor.New()
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	if err := proc.ProcessFile(*inputFile, *outputDir); err != nil {
		log.Fatalf("Failed to process file: %v", err)
	}

	if err := generateDiagrams(*outputDir); err != nil {
		log.Fatalf("Failed to generate diagrams: %v", err)
	}

	fmt.Println("Successfully generated architecture diagrams in:", *outputDir)
}

func generateDiagrams(outputDir string) error {
	plantumlFiles, err := filepath.Glob(filepath.Join(outputDir, "*.puml"))
	if err != nil {
		return fmt.Errorf("failed to find PlantUML files: %v", err)
	}

	for _, file := range plantumlFiles {
		if err := runCommand("plantuml", file); err != nil {
			return fmt.Errorf("failed to generate PNG from %s: %v", file, err)
		}
	}

	dotFiles, err := filepath.Glob(filepath.Join(outputDir, "*.dot"))
	if err != nil {
		return fmt.Errorf("failed to find Graphviz files: %v", err)
	}

	for _, file := range dotFiles {
		outFile := file[:len(file)-4] + ".png"
		if err := runCommand("dot", "-Tpng", "-o", outFile, file); err != nil {
			return fmt.Errorf("failed to generate PNG from %s: %v", file, err)
		}
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command failed: %v\nOutput: %s", err, output)
	}
	return nil
}
