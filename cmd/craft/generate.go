package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/internal/visualizer"
	craft "github.com/tcarcao/craft/v2/pkg/craft"
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
	var stdout bool
	var force bool
	var allowErrors bool

	cmd := &cobra.Command{
		Use:   "generate [files...]",
		Short: "Generate diagram files from .craft files (all inputs form one model)",
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

			if stdout {
				if outputDir != "" {
					return fmt.Errorf("--stdout and --output are mutually exclusive")
				}
				if split {
					return fmt.Errorf("--stdout is incompatible with --split (only one diagram per invocation)")
				}
				if diagType == "all" || strings.Contains(diagType, ",") {
					return fmt.Errorf("--stdout requires a single diagram (drop --type all or pick one type)")
				}
			}

			files, err := resolveFiles(args)
			if err != nil {
				return err
			}
			contents := make(map[string][]byte, len(files))
			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				contents[file] = content
			}

			// All inputs are one model: a use case in one file may run through
			// services and domains declared in another.
			doc, diags, _ := craft.ParseFiles(contents)
			errorCount := 0
			for _, d := range diags {
				fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", d.SourceURI, d.Range.Start.Line+1, d.Severity, d.Message)
				if d.Severity == craft.SeverityError {
					errorCount++
				}
			}
			if errorCount > 0 && !allowErrors {
				return fmt.Errorf("%d error(s) prevent diagram generation (use --allow-errors to draw anyway)", errorCount)
			}

			outDir := outputDir
			if outDir == "" {
				outDir = filepath.Dir(files[0])
			}
			if !stdout {
				if err := os.MkdirAll(outDir, 0755); err != nil {
					return err
				}
			}

			opts := c4Options{
				boundaries:    boundaries,
				showDatabases: !noDatabases,
				focusServices: focusServices,
				focusContexts: focusContexts,
			}
			return generateDiagrams(visualizer.New(visualizer.ExecPlantUML{}), doc, modelBaseName(files), outDir, diagType, mode, opts, useCaseFilter, split, format, stdout, cmd.OutOrStdout(), force)
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
	cmd.Flags().BoolVar(&stdout, "stdout", false, "write to stdout instead of files (single diagram only)")
	cmd.Flags().BoolVar(&force, "force", false, "allow overwriting existing .md files when --format=mermaid-md")
	cmd.Flags().BoolVar(&allowErrors, "allow-errors", false, "draw diagrams even when the model has error diagnostics")
	return cmd
}

func generateDiagrams(r *visualizer.Renderer, model *craft.CraftDoc, base, outDir, diagType, mode string, opts c4Options, useCaseFilter []string, split bool, format string, stdout bool, stdoutW io.Writer, force bool) error {
	backend, outFormat := visualizer.PlantUML, visualizer.FormatSource
	switch format {
	case "mermaid":
		backend = visualizer.Mermaid
	case "mermaid-md":
		backend, outFormat = visualizer.Mermaid, visualizer.FormatMarkdown
	}
	domainView := visualizer.ViewFlow
	if strings.ToLower(mode) == "architecture" {
		domainView = visualizer.ViewArchitecture
	}

	for _, t := range expandType(diagType) {
		req := visualizer.Request{Backend: backend, Format: outFormat}
		switch t {
		case "c4":
			// C4 always shows the whole architecture: --use-case and --split
			// do not apply to it.
			req.View = visualizer.ViewC4
			req.C4 = &visualizer.C4Options{
				Boundaries:    opts.boundaries != "transparent",
				ShowDatabases: opts.showDatabases,
				FocusServices: opts.focusServices,
				FocusContexts: opts.focusContexts,
			}
		case "domain":
			req.View, req.UseCases = domainView, useCaseFilter
		case "sequence":
			req.View, req.UseCases = visualizer.ViewSequence, useCaseFilter
		default:
			return fmt.Errorf("unknown diagram type %q", t)
		}

		// Split applies to per-use-case views; the architecture view is
		// structural, so it ignores --split.
		if split && (req.View == visualizer.ViewFlow || req.View == visualizer.ViewSequence) {
			if err := emitSplitFiles(r, model, req, base, outDir, t, useCaseFilter, force); err != nil {
				return fmt.Errorf("%s: %w", t, err)
			}
			continue
		}

		out, err := r.Render(model, req)
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
		if stdout {
			if _, err := stdoutW.Write(out.Data); err != nil {
				return err
			}
			continue
		}
		if err := writeOutput(filepath.Join(outDir, base+"-"+t+"."+out.Ext), out.Data, force); err != nil {
			return err
		}
	}
	return nil
}

// emitSplitFiles writes one diagram per use case, titled with its name.
// Use cases with no scenarios are skipped with a note on stderr, and two use
// cases whose names slugify alike are an error rather than one overwriting the
// other.
func emitSplitFiles(r *visualizer.Renderer, model *craft.CraftDoc, req visualizer.Request, base, outDir, diagSuffix string, useCaseFilter []string, force bool) error {
	wanted := map[string]bool{}
	for _, v := range useCaseFilter {
		wanted[visualizer.Slugify(v)] = true
	}
	if len(useCaseFilter) > 0 {
		// Let Render report unknown names, with the available slugs.
		if _, err := r.Render(model, req); err != nil {
			return err
		}
	}
	seenSlugs := make(map[string]string) // slug -> original name
	for _, uc := range model.UseCases {
		slug := visualizer.Slugify(uc.Name)
		if len(wanted) > 0 && !wanted[slug] {
			continue
		}
		if len(uc.Scenarios) == 0 {
			fmt.Fprintf(os.Stderr, "craft generate: skipped %q (no scenarios)\n", uc.Name)
			continue
		}
		if prev, ok := seenSlugs[slug]; ok {
			return fmt.Errorf("duplicate slug %q from use cases %q and %q — rename one", slug, prev, uc.Name)
		}
		seenSlugs[slug] = uc.Name

		one := req
		one.UseCases, one.Title = []string{uc.Name}, uc.Name
		out, err := r.Render(model, one)
		if err != nil {
			return fmt.Errorf("%s: %w", uc.Name, err)
		}
		if err := writeOutput(filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.%s", base, diagSuffix, slug, out.Ext)), out.Data, force); err != nil {
			return err
		}
	}
	return nil
}

// writeOutput writes one diagram file and prints its path. An existing .md
// file is only overwritten with --force: it may hold hand-written prose.
func writeOutput(outFile string, data []byte, force bool) error {
	if strings.HasSuffix(outFile, ".md") && !force {
		if _, err := os.Stat(outFile); err == nil {
			return fmt.Errorf("refusing to overwrite %q (use --force to overwrite)", outFile)
		}
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return err
	}
	fmt.Println(outFile)
	return nil
}

// modelBaseName names the output files of a model: the input's own name
// without extension for a single file, else the name of the directory the
// inputs share (e.g. "examples" for examples/*.craft).
func modelBaseName(files []string) string {
	if len(files) == 1 {
		name := filepath.Base(files[0])
		return strings.TrimSuffix(name, filepath.Ext(name))
	}
	common := filepath.Dir(files[0])
	for _, f := range files[1:] {
		for !isWithin(f, common) {
			common = filepath.Dir(common)
		}
	}
	abs, err := filepath.Abs(common)
	if err != nil || filepath.Base(abs) == string(filepath.Separator) {
		return "model"
	}
	return filepath.Base(abs)
}

// isWithin reports whether path lies inside dir.
func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func expandType(t string) []string {
	if t == "all" {
		return []string{"c4", "domain", "sequence"}
	}
	return []string{strings.ToLower(t)}
}
