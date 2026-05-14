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
	var useCaseFilter []string
	var split bool

	cmd := &cobra.Command{
		Use:   "generate [files...]",
		Short: "Generate PlantUML diagram files from .craft files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if diagType == "all" || strings.ToLower(diagType) == "c4" {
				switch strings.ToLower(boundaries) {
				case "boundaries", "transparent":
					// valid
				default:
					return fmt.Errorf("--boundaries must be 'boundaries' or 'transparent', got %q", boundaries)
				}
			}
			boundaries = strings.ToLower(boundaries)

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

				greenRoot, li, diags := syntax.Parse(string(content))
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
				tree := syntax.Root(greenRoot)
				doc := syntax.ProjectFromTree(tree, li)

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
				if err := generateForFile(v, doc, base, outDir, diagType, mode, opts, useCaseFilter, split); err != nil {
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
	cmd.Flags().StringSliceVar(&useCaseFilter, "use-case", nil, "comma-separated use case slugs or names to render (detailed-domain and sequence only)")
	cmd.Flags().BoolVar(&split, "split", false, "emit one file per use case (detailed-domain and sequence only)")
	return cmd
}

func generateForFile(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir, diagType, mode string, opts c4Options, useCaseFilter []string, split bool) error {
	// If the user supplied --use-case, validate that every requested value
	// matches some use case in this file. Only used for domain/sequence
	// types — c4 keeps full model for structural completeness.
	if len(useCaseFilter) > 0 {
		_, missing := visualizer.FilterUseCases(model, useCaseFilter)
		if len(missing) > 0 {
			available := make([]string, 0, len(model.UseCases))
			for _, uc := range model.UseCases {
				available = append(available, visualizer.Slugify(uc.Name))
			}
			return fmt.Errorf("no use case matches %v (available: %s)", missing, strings.Join(available, ", "))
		}
	}

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

			hasFocus := len(opts.focusServices) > 0 || len(opts.focusContexts) > 0
			var data []byte
			if hasFocus {
				var err error
				data, _, err = v.GenerateC4WithFocusContextsAndFormat(model, opts.focusServices, opts.focusContexts, boundariesMode, opts.showDatabases, visualizer.FormatPUML)
				if err != nil {
					return fmt.Errorf("c4: %w", err)
				}
			} else {
				var err error
				data, _, err = v.GenerateC4WithFormat(model, boundariesMode, opts.showDatabases, visualizer.FormatPUML)
				if err != nil {
					return fmt.Errorf("c4: %w", err)
				}
			}
			content = data
			outFile = filepath.Join(outDir, base+"-c4.puml")

		case "domain":
			diagModel := model
			if len(useCaseFilter) > 0 {
				diagModel, _ = visualizer.FilterUseCases(model, useCaseFilter)
			}
			// Architecture mode silently ignores --split — it is structural.
			if split && domainMode == visualizer.DomainModeDetailed {
				if err := emitSplitDomainFiles(v, diagModel, base, outDir, domainMode); err != nil {
					return fmt.Errorf("domain: %w", err)
				}
				continue
			}
			data, _, err := v.GenerateDomainDiagramWithModeAndFormat(diagModel, domainMode, visualizer.FormatPUML)
			if err != nil {
				return fmt.Errorf("domain: %w", err)
			}
			content = data
			outFile = filepath.Join(outDir, base+"-domain.puml")

		case "sequence":
			diagModel := model
			if len(useCaseFilter) > 0 {
				diagModel, _ = visualizer.FilterUseCases(model, useCaseFilter)
			}
			if split {
				if err := emitSplitSequenceFiles(v, diagModel, base, outDir, domainMode); err != nil {
					return fmt.Errorf("sequence: %w", err)
				}
				continue
			}
			data, _, err := v.GenerateDomainDiagramWithTypeAndModeAndFormat(diagModel, visualizer.DiagramTypeSequence, domainMode, visualizer.FormatPUML)
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

// emitSplitDomainFiles writes one detailed-domain PUML per use case. Each
// output file contains a single use case from the filtered model and is
// prefixed with a PlantUML `title` directive matching the use case name.
// Use cases with zero scenarios are skipped with a stderr note.
func emitSplitDomainFiles(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir string, mode visualizer.DomainMode) error {
	return emitSplitFiles(model, base, outDir, "domain", func(single *craft.CraftDoc) ([]byte, error) {
		data, _, err := v.GenerateDomainDiagramWithModeAndFormat(single, mode, visualizer.FormatPUML)
		return data, err
	})
}

// emitSplitSequenceFiles writes one sequence PUML per use case. Same
// semantics as emitSplitDomainFiles.
func emitSplitSequenceFiles(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir string, mode visualizer.DomainMode) error {
	return emitSplitFiles(model, base, outDir, "sequence", func(single *craft.CraftDoc) ([]byte, error) {
		data, _, err := v.GenerateDomainDiagramWithTypeAndModeAndFormat(single, visualizer.DiagramTypeSequence, mode, visualizer.FormatPUML)
		return data, err
	})
}

// emitSplitFiles iterates use cases in the model and writes one file per use
// case using the supplied generator. Empty use cases are skipped with a
// stderr message. Each file gets a `title <use case name>` directive
// inserted right after `@startuml`. Detects slug collisions and errors before
// writing the conflicting file.
func emitSplitFiles(model *craft.CraftDoc, base, outDir, diagSuffix string, generate func(*craft.CraftDoc) ([]byte, error)) error {
	seenSlugs := make(map[string]string) // slug -> original name (for collision diagnostics)
	for _, uc := range model.UseCases {
		if len(uc.Scenarios) == 0 {
			fmt.Fprintf(os.Stderr, "craft generate: skipped %q (no scenarios)\n", uc.Name)
			continue
		}
		slug := visualizer.Slugify(uc.Name)
		if prev, ok := seenSlugs[slug]; ok {
			return fmt.Errorf("duplicate slug %q from use cases %q and %q — rename one", slug, prev, uc.Name)
		}
		seenSlugs[slug] = uc.Name

		single := *model
		single.UseCases = []craft.UseCase{uc}
		data, err := generate(&single)
		if err != nil {
			return fmt.Errorf("%s: %w", uc.Name, err)
		}
		titled := injectTitle(data, uc.Name)
		outFile := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.puml", base, diagSuffix, slug))
		if err := os.WriteFile(outFile, titled, 0644); err != nil {
			return err
		}
		fmt.Println(outFile)
	}
	return nil
}

// injectTitle inserts `title <name>\n` immediately after the first
// `@startuml\n` line of puml. If `@startuml\n` isn't found (defensive),
// returns puml unchanged.
func injectTitle(puml []byte, name string) []byte {
	const marker = "@startuml\n"
	idx := strings.Index(string(puml), marker)
	if idx < 0 {
		return puml
	}
	insertAt := idx + len(marker)
	titleLine := []byte("title " + name + "\n")
	out := make([]byte, 0, len(puml)+len(titleLine))
	out = append(out, puml[:insertAt]...)
	out = append(out, titleLine...)
	out = append(out, puml[insertAt:]...)
	return out
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
