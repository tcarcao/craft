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
	var format string

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

			switch format {
			case "puml", "mermaid", "mermaid-md":
				// valid
			default:
				return fmt.Errorf("--format must be one of puml|mermaid|mermaid-md, got %q", format)
			}

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
				if err := generateForFile(v, doc, base, outDir, diagType, mode, opts, useCaseFilter, split, format); err != nil {
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
	cmd.Flags().StringVar(&format, "format", "puml", "output format: puml|mermaid|mermaid-md")
	return cmd
}

func generateForFile(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir, diagType, mode string, opts c4Options, useCaseFilter []string, split bool, format string) error {
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
			out, ext, err := renderC4(v, model, opts, format)
			if err != nil {
				return fmt.Errorf("c4: %w", err)
			}
			content = out
			outFile = filepath.Join(outDir, base+"-c4."+ext)

		case "domain":
			diagModel := model
			if len(useCaseFilter) > 0 {
				diagModel, _ = visualizer.FilterUseCases(model, useCaseFilter)
			}
			// Architecture mode silently ignores --split — it is structural.
			if split && domainMode == visualizer.DomainModeDetailed {
				if err := emitSplitDomainFiles(v, diagModel, base, outDir, domainMode, format); err != nil {
					return fmt.Errorf("domain: %w", err)
				}
				continue
			}
			out, ext, err := renderDomain(v, diagModel, domainMode, format)
			if err != nil {
				return fmt.Errorf("domain: %w", err)
			}
			content = out
			outFile = filepath.Join(outDir, base+"-domain."+ext)

		case "sequence":
			diagModel := model
			if len(useCaseFilter) > 0 {
				diagModel, _ = visualizer.FilterUseCases(model, useCaseFilter)
			}
			if split {
				if err := emitSplitSequenceFiles(v, diagModel, base, outDir, domainMode, format); err != nil {
					return fmt.Errorf("sequence: %w", err)
				}
				continue
			}
			out, ext, err := renderSequence(v, diagModel, domainMode, format)
			if err != nil {
				return fmt.Errorf("sequence: %w", err)
			}
			content = out
			outFile = filepath.Join(outDir, base+"-sequence."+ext)
		}

		if err := os.WriteFile(outFile, content, 0644); err != nil {
			return err
		}
		fmt.Println(outFile)
	}
	return nil
}

// emitSplitDomainFiles writes one detailed-domain file per use case. Each
// output file contains a single use case from the filtered model and is
// prefixed with a per-format title (PlantUML `title`, mermaid `%% comment`,
// or markdown `# heading`). Use cases with zero scenarios are skipped with
// a stderr note.
func emitSplitDomainFiles(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir string, mode visualizer.DomainMode, format string) error {
	return emitSplitFiles(model, base, outDir, "domain", format, func(single *craft.CraftDoc) ([]byte, string, error) {
		return renderDomain(v, single, mode, format)
	})
}

// emitSplitSequenceFiles writes one sequence file per use case. Same
// semantics as emitSplitDomainFiles.
func emitSplitSequenceFiles(v *visualizer.Visualizer, model *craft.CraftDoc, base, outDir string, mode visualizer.DomainMode, format string) error {
	return emitSplitFiles(model, base, outDir, "sequence", format, func(single *craft.CraftDoc) ([]byte, string, error) {
		return renderSequence(v, single, mode, format)
	})
}

// emitSplitFiles iterates use cases in the model and writes one file per use
// case using the supplied generator. Empty use cases are skipped with a
// stderr message. Each file gets a per-format title injected by injectTitle.
// Detects slug collisions and errors before writing the conflicting file.
func emitSplitFiles(model *craft.CraftDoc, base, outDir, diagSuffix, format string, generate func(*craft.CraftDoc) ([]byte, string, error)) error {
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
		data, ext, err := generate(&single)
		if err != nil {
			return fmt.Errorf("%s: %w", uc.Name, err)
		}
		titled := injectTitle(data, uc.Name, format)
		outFile := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.%s", base, diagSuffix, slug, ext))
		if err := os.WriteFile(outFile, titled, 0644); err != nil {
			return err
		}
		fmt.Println(outFile)
	}
	return nil
}

// renderDomain dispatches the domain generator by format. Returns (content, ext, error).
func renderDomain(v *visualizer.Visualizer, doc *craft.CraftDoc, mode visualizer.DomainMode, format string) ([]byte, string, error) {
	switch format {
	case "mermaid":
		src, err := v.GenerateDomainDiagramMermaid(doc, mode)
		return []byte(src), "mmd", err
	case "mermaid-md":
		src, err := v.GenerateDomainDiagramMermaid(doc, mode)
		if err != nil {
			return nil, "", err
		}
		title := titleFor(doc, "Domain")
		return []byte(wrapMarkdown(src, title)), "md", nil
	default:
		data, _, err := v.GenerateDomainDiagramWithModeAndFormat(doc, mode, visualizer.FormatPUML)
		return data, "puml", err
	}
}

func renderSequence(v *visualizer.Visualizer, doc *craft.CraftDoc, mode visualizer.DomainMode, format string) ([]byte, string, error) {
	switch format {
	case "mermaid":
		src, err := v.GenerateSequenceDiagramMermaid(doc, mode)
		return []byte(src), "mmd", err
	case "mermaid-md":
		src, err := v.GenerateSequenceDiagramMermaid(doc, mode)
		if err != nil {
			return nil, "", err
		}
		title := titleFor(doc, "Sequence")
		return []byte(wrapMarkdown(src, title)), "md", nil
	default:
		data, _, err := v.GenerateDomainDiagramWithTypeAndModeAndFormat(doc, visualizer.DiagramTypeSequence, mode, visualizer.FormatPUML)
		return data, "puml", err
	}
}

func renderC4(v *visualizer.Visualizer, doc *craft.CraftDoc, opts c4Options, format string) ([]byte, string, error) {
	boundariesMode := visualizer.C4ModeBoundaries
	if strings.ToLower(opts.boundaries) == string(visualizer.C4ModeTransparent) {
		boundariesMode = visualizer.C4ModeTransparent
	}
	switch format {
	case "mermaid":
		src, err := v.GenerateC4Mermaid(doc, boundariesMode, opts.showDatabases)
		return []byte(src), "mmd", err
	case "mermaid-md":
		src, err := v.GenerateC4Mermaid(doc, boundariesMode, opts.showDatabases)
		if err != nil {
			return nil, "", err
		}
		title := titleFor(doc, "C4")
		return []byte(wrapMarkdown(src, title)), "md", nil
	default:
		hasFocus := len(opts.focusServices) > 0 || len(opts.focusContexts) > 0
		if hasFocus {
			data, _, err := v.GenerateC4WithFocusContextsAndFormat(doc, opts.focusServices, opts.focusContexts, boundariesMode, opts.showDatabases, visualizer.FormatPUML)
			return data, "puml", err
		}
		data, _, err := v.GenerateC4WithFormat(doc, boundariesMode, opts.showDatabases, visualizer.FormatPUML)
		return data, "puml", err
	}
}

// wrapMarkdown wraps Mermaid source in a fenced markdown block with a title heading.
func wrapMarkdown(source, title string) string {
	return "# " + title + "\n\n```mermaid\n" + source + "\n```\n"
}

// titleFor produces the markdown heading shown above the fenced block.
// For single-use-case (split-mode) docs the use case name is preferred;
// otherwise a generic "<base> — <type>" form is used.
func titleFor(doc *craft.CraftDoc, diagramType string) string {
	if len(doc.UseCases) == 1 {
		return doc.UseCases[0].Name
	}
	return diagramType
}

// injectTitle prepends a per-format title for split-mode files.
//   - mermaid-md: replace the wrapper's '# <generic title>' heading with the use case name.
//   - mermaid: prepend a '%% <name>' comment line (Mermaid has no title directive).
//   - puml: insert `title <name>` after the first @startuml.
func injectTitle(content []byte, useCaseName, format string) []byte {
	switch format {
	case "mermaid-md":
		s := string(content)
		if strings.HasPrefix(s, "# ") {
			newline := strings.IndexByte(s, '\n')
			if newline > 0 {
				return []byte("# " + useCaseName + s[newline:])
			}
		}
		return content
	case "mermaid":
		return append([]byte("%% "+useCaseName+"\n"), content...)
	default:
		const marker = "@startuml\n"
		idx := strings.Index(string(content), marker)
		if idx < 0 {
			return content
		}
		insertAt := idx + len(marker)
		titleLine := []byte("title " + useCaseName + "\n")
		out := make([]byte, 0, len(content)+len(titleLine))
		out = append(out, content[:insertAt]...)
		out = append(out, titleLine...)
		out = append(out, content[insertAt:]...)
		return out
	}
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
