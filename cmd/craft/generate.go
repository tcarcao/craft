package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/internal/visualizer"
	craft "github.com/tcarcao/craft/pkg/craft"
)

type c4Options struct {
	boundaries    string
	showDatabases bool
	focusServices []string
	focusContexts []string
}

func generateCmd() *cobra.Command {
	var diagType string
	var mode string
	var outputDir string
	var boundaries string
	var noDatabases bool
	var focusServices []string
	var focusContexts []string

	cmd := &cobra.Command{
		Use:   "generate [files...]",
		Short: "Generate PlantUML diagram files from .craft files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveFiles(args)
			if err != nil {
				return err
			}

			v := visualizer.New()

			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}

				astFile, diags := syntax.Parse(string(content))
				hasError := false
				for _, d := range diags {
					fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", file, d.Range.Start.Line+1, d.Severity, d.Message)
					if d.Severity == craft.SeverityError {
						hasError = true
					}
				}
				if hasError {
					return fmt.Errorf("%s: parse errors prevent diagram generation", file)
				}
				doc := syntax.Project(astFile)

				outDir := outputDir
				if outDir == "" {
					outDir = filepath.Dir(file)
				}
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return err
				}

				base := baseName(file)

				opts := c4Options{
					boundaries:    boundaries,
					showDatabases: !noDatabases,
					focusServices: focusServices,
					focusContexts: focusContexts,
				}
				if err := generateForFile(v, doc, base, outDir, diagType, mode, opts); err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&diagType, "type", "all", "diagram type: c4|domain|sequence|all")
	cmd.Flags().StringVar(&mode, "mode", "detailed", "domain diagram mode: detailed|architecture")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory (default: same as input file)")
	cmd.Flags().StringVar(&boundaries, "boundaries", "boundaries", "C4 boundaries mode: boundaries|transparent")
	cmd.Flags().BoolVar(&noDatabases, "no-databases", false, "hide data-store containers from C4 diagram")
	cmd.Flags().StringSliceVar(&focusServices, "focus", nil, "comma-separated services to focus on in C4 diagram")
	cmd.Flags().StringSliceVar(&focusContexts, "focus-context", nil, "comma-separated bounded contexts to focus on in C4 diagram")
	return cmd
}

func generateForFile(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir, diagType, mode string, opts c4Options) error {
	domainMode := visualizer.DomainModeDetailed
	if strings.ToLower(mode) == "architecture" {
		domainMode = visualizer.DomainModeArchitecture
	}

	types := expandType(diagType)

	for _, t := range types {
		var content []byte
		var outFile string

		switch t {
		case "c4":
			boundariesMode := visualizer.C4ModeBoundaries
			if strings.ToLower(opts.boundaries) == string(visualizer.C4ModeTransparent) {
				boundariesMode = visualizer.C4ModeTransparent
			}
			data, _, err := v.GenerateC4WithFormat(model, boundariesMode, opts.showDatabases, visualizer.FormatPUML)
			if err != nil {
				return fmt.Errorf("c4: %w", err)
			}
			content = data
			outFile = filepath.Join(outDir, base+"-c4.puml")

		case "domain":
			data, _, err := v.GenerateDomainDiagramWithModeAndFormat(model, domainMode, visualizer.FormatPUML)
			if err != nil {
				return fmt.Errorf("domain: %w", err)
			}
			content = data
			outFile = filepath.Join(outDir, base+"-domain.puml")

		case "sequence":
			data, _, err := v.GenerateDomainDiagramWithTypeAndModeAndFormat(model, visualizer.DiagramTypeSequence, domainMode, visualizer.FormatPUML)
			if err != nil {
				return fmt.Errorf("sequence: %w", err)
			}
			content = data
			outFile = filepath.Join(outDir, base+"-sequence.puml")
		}

		if err := os.WriteFile(outFile, content, 0644); err != nil {
			return err
		}
		fmt.Println(outFile)
	}
	return nil
}

// baseName strips directory and extension from a file path.
func baseName(file string) string {
	name := filepath.Base(file)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

func expandType(t string) []string {
	if t == "all" {
		return []string{"c4", "domain", "sequence"}
	}
	return []string{strings.ToLower(t)}
}
