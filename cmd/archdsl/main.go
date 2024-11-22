package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/tcarcao/archdsl/internal/processor"
)

// Version will be set at build time
var Version = "dev"

func main() {
	var (
		inputFile = flag.String("input", "", "Input DSL file")
		outputDir = flag.String("output", "output", "Output directory for generated diagrams")
		help      = flag.Bool("help", false, "Show help message")
		version   = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *version {
		fmt.Printf("ArchDSL version %s\n", Version)
		return
	}

	if *help {
		printUsage()
		return
	}

	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: input file is required\n")
		printUsage()
		os.Exit(1)
	}

	processor, err := processor.New()
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	if err := processor.ProcessFile(*inputFile, *outputDir); err != nil {
		log.Fatalf("Failed to process file: %v", err)
	}

	fmt.Printf("Successfully generated diagrams in %s/\n", *outputDir)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `ArchDSL - Architecture DSL Processor v%s

Usage:
  archdsl -input <file.dsl> [-output <dir>]

Options:
  -input    Input DSL file (required)
  -output   Output directory (default: output)
  -version  Show version information
  -help     Show this help message

Examples:
  archdsl -input examples/e-commerce.dsl
  archdsl -input system.dsl -output diagrams/
  archdsl -version

`, Version)
}
