package main

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	var journeys []string

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
			case "puml", "mermaid", "mermaid-md", "board":
				// valid
			default:
				return fmt.Errorf("--format must be one of puml|mermaid|mermaid-md|board, got %q", format)
			}
			if format == "board" {
				switch {
				case len(journeys) > 0:
					return fmt.Errorf("--format board draws the detailed domain diagram; it does not draw journeys")
				case strings.ToLower(diagType) != "domain":
					return fmt.Errorf("--format board draws the detailed domain diagram only: use --type domain")
				case strings.ToLower(mode) == "architecture":
					return fmt.Errorf("--format board draws the detailed domain diagram only: drop --mode architecture")
				}
			}

			if len(journeys) > 0 {
				journeys = dedupeSlugs(journeys)
				switch {
				case cmd.Flags().Changed("type") && strings.ToLower(diagType) != "sequence":
					return fmt.Errorf("`--journey` draws a sequence; drop `--type` or use `--type sequence`")
				case split:
					return fmt.Errorf("`--journey` cannot be combined with --split")
				case len(useCaseFilter) > 0:
					return fmt.Errorf("`--journey` cannot be combined with --use-case: it draws its own members")
				case stdout && (len(journeys) != 1 || journeys[0] == "all"):
					return fmt.Errorf("--stdout requires a single journey")
				}
				// The checks below are about --type; a journey is one sequence.
				diagType = "sequence"
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
			if len(files) == 0 {
				return fmt.Errorf("no files matched %v", args)
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

			r := visualizer.New(visualizer.ExecPlantUML{})
			if len(journeys) > 0 {
				return generateJourneys(r, doc, journeys, modelBaseName(files), outDir, format, stdout, cmd.OutOrStdout(), force)
			}
			opts := c4Options{
				boundaries:    boundaries,
				showDatabases: !noDatabases,
				focusServices: focusServices,
				focusContexts: focusContexts,
			}
			return generateDiagrams(r, doc, modelBaseName(files), outDir, diagType, mode, opts, useCaseFilter, split, format, stdout, cmd.OutOrStdout(), force)
		},
	}

	cmd.Flags().StringVar(&diagType, "type", "all", "diagram type: c4|domain|sequence|surface|all")
	cmd.Flags().StringVar(&mode, "mode", "detailed", "domain diagram mode: detailed|architecture")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory (default: same as input file)")
	cmd.Flags().StringVar(&boundaries, "boundaries", "boundaries", "C4 boundaries mode: boundaries|transparent")
	cmd.Flags().BoolVar(&noDatabases, "no-databases", false, "hide data-store containers from C4 diagram")
	cmd.Flags().StringSliceVar(&focusServices, "focus", nil, "comma-separated services to focus on in C4 diagram")
	cmd.Flags().StringSliceVar(&focusContexts, "focus-context", nil, "comma-separated bounded contexts to focus on in C4 diagram")
	cmd.Flags().StringSliceVar(&useCaseFilter, "use-case", nil, "comma-separated use case slugs or names to render (detailed-domain and sequence only)")
	cmd.Flags().BoolVar(&split, "split", false, "emit one file per use case (detailed-domain and sequence only)")
	cmd.Flags().StringVar(&format, "format", "puml", "output format: puml|mermaid|mermaid-md|board")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "write to stdout instead of files (single diagram only)")
	cmd.Flags().BoolVar(&force, "force", false, "allow overwriting existing .md files when --format=mermaid-md")
	cmd.Flags().StringSliceVar(&journeys, "journey", nil, "journey slugs to draw as one sequence each (comma-separated), or all")
	cmd.Flags().BoolVar(&allowErrors, "allow-errors", false, "draw diagrams even when the model has error diagnostics")
	return cmd
}

// dedupeSlugs drops repeated slugs, keeping the first of each in order.
func dedupeSlugs(slugs []string) []string {
	seen := make(map[string]bool, len(slugs))
	out := slugs[:0:0]
	for _, s := range slugs {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// generateJourneys writes one spine sequence per journey slug; "all" stands
// for every journey with two or more members.
func generateJourneys(r *visualizer.Renderer, model *craft.CraftDoc, journeys []string, base, outDir, format string, stdout bool, stdoutW io.Writer, force bool) error {
	backend, outFormat := visualizer.PlantUML, visualizer.FormatSource
	switch format {
	case "mermaid":
		backend = visualizer.Mermaid
	case "mermaid-md":
		backend, outFormat = visualizer.Mermaid, visualizer.FormatMarkdown
	}
	all := craft.JourneySlugs(model)
	var slugs []string
	for _, slug := range journeys {
		if slug != "all" {
			slugs = append(slugs, slug)
			continue
		}
		for _, s := range all {
			if len(craft.BuildJourney(model, s).Members) >= 2 {
				slugs = append(slugs, s)
			}
		}
	}
	slugs = dedupeSlugs(slugs) // a slug named and also covered by all
	if len(slugs) == 0 {
		fmt.Fprintln(os.Stderr, "note: no journey has two or more members; nothing to draw")
		return nil
	}
	// Every slug is checked before any file is written.
	spines := make([]craft.Journey, len(slugs))
	for i, slug := range slugs {
		spines[i] = craft.BuildJourney(model, slug)
		if len(spines[i].Members) > 0 {
			continue
		}
		closest := closestSlugs(all, slug, 3)
		if len(closest) == 0 {
			return fmt.Errorf("unknown journey %q; no use case has a journey tag", slug)
		}
		return fmt.Errorf("unknown journey %q; closest: %s", slug, strings.Join(closest, ", "))
	}
	for i, slug := range slugs {
		if spines[i].Cycle {
			fmt.Fprintf(os.Stderr, "note: journey %s has a hand-off cycle; it is entered where the flow reaches it\n", slug)
		}
		out, err := r.Render(model, visualizer.Request{View: visualizer.ViewSequence, Backend: backend, Format: outFormat, Journey: slug})
		if err != nil {
			return fmt.Errorf("journey %s: %w", slug, err)
		}
		if stdout {
			if _, err := stdoutW.Write(out.Data); err != nil {
				return err
			}
			continue
		}
		name := base + "-journey-" + fileSlug(slug) + "." + out.Ext
		if err := writeOutput(filepath.Join(outDir, name), out.Data, force); err != nil {
			return err
		}
	}
	return nil
}

// fileSlug makes slug safe to use in a file name: each / and \ becomes -,
// and every .. is removed, so the file stays in the output directory.
func fileSlug(slug string) string {
	s := strings.NewReplacer("/", "-", `\`, "-").Replace(slug)
	for strings.Contains(s, "..") {
		s = strings.ReplaceAll(s, "..", "")
	}
	return s
}

// closestSlugs returns the n slugs nearest to want by edit distance, ties
// broken alphabetically.
func closestSlugs(slugs []string, want string, n int) []string {
	dist := make(map[string]int, len(slugs))
	for _, s := range slugs {
		dist[s] = levenshtein(s, want)
	}
	sorted := slices.Clone(slugs)
	slices.SortFunc(sorted, func(a, b string) int {
		return cmp.Or(cmp.Compare(dist[a], dist[b]), cmp.Compare(a, b))
	})
	return sorted[:min(n, len(sorted))]
}

// levenshtein is the edit distance between a and b, in bytes.
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func generateDiagrams(r *visualizer.Renderer, model *craft.CraftDoc, base, outDir, diagType, mode string, opts c4Options, useCaseFilter []string, split bool, format string, stdout bool, stdoutW io.Writer, force bool) error {
	backend, outFormat := visualizer.PlantUML, visualizer.FormatSource
	switch format {
	case "mermaid":
		backend = visualizer.Mermaid
	case "mermaid-md":
		backend, outFormat = visualizer.Mermaid, visualizer.FormatMarkdown
	case "board":
		backend, outFormat = visualizer.Board, visualizer.FormatSVG
	}
	domainView := visualizer.ViewFlow
	if strings.ToLower(mode) == "architecture" {
		domainView = visualizer.ViewArchitecture
	}

	for _, t := range expandType(diagType, len(model.Surfaces) > 0) {
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
		case "surface":
			// A table, not a diagram: Markdown whatever --format says, over
			// every use case (--use-case and --split do not apply).
			req.View, req.Format = visualizer.ViewSurface, visualizer.FormatMarkdown
		default:
			return fmt.Errorf("unknown diagram type %q", t)
		}

		// Split applies to per-use-case views; the architecture view is
		// structural, so it ignores --split.
		// The board draws one use case per image, so without --stdout it is
		// always split.
		if (split || (format == "board" && !stdout)) && (req.View == visualizer.ViewFlow || req.View == visualizer.ViewSequence) {
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
		if req.Backend == visualizer.Board {
			one.Title = "" // the board titles itself "Scenario: <name>"
		}
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
	// Absolute paths, so a relative and an absolute input meet at their real
	// common directory instead of climbing to "." forever.
	abs := make([]string, len(files))
	for i, f := range files {
		a, err := filepath.Abs(f)
		if err != nil {
			return "model"
		}
		abs[i] = a
	}
	common := filepath.Dir(abs[0])
	for _, f := range abs[1:] {
		for !isWithin(f, common) {
			parent := filepath.Dir(common)
			if parent == common {
				return "model"
			}
			common = parent
		}
	}
	if filepath.Base(common) == string(filepath.Separator) {
		return "model"
	}
	return filepath.Base(common)
}

// isWithin reports whether path lies inside dir.
func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// expandType lists the diagram types of --type. `all` includes the surface
// table only when the model declares a surface, so a model without one gets
// the same files as before surfaces existed.
func expandType(t string, hasSurfaces bool) []string {
	if t == "all" {
		if hasSurfaces {
			return []string{"c4", "domain", "sequence", "surface"}
		}
		return []string{"c4", "domain", "sequence"}
	}
	return []string{strings.ToLower(t)}
}
