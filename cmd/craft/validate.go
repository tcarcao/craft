package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

type validateResult struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

func validateCmd() *cobra.Command {
	var strict bool
	var format string

	cmd := &cobra.Command{
		Use:   "validate [files...]",
		Short: "Parse and lint .craft files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveFiles(args)
			if err != nil {
				return err
			}

			perFileTrees := make(map[string]*syntax.SyntaxNode)
			perFileSyms := make(map[string]sema.Symbols)
			var results []validateResult

			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					results = append(results, validateResult{
						File:     file,
						Severity: "error",
						Message:  err.Error(),
					})
					continue
				}

				uri := "file://" + file
				tree, _, parseDiags := syntax.ParseTree(string(content))
				perFileTrees[uri] = tree

				for _, d := range parseDiags {
					results = append(results, validateResult{
						File:     file,
						Line:     d.Range.Start.Line + 1,
						Severity: string(d.Severity),
						Message:  d.Message,
					})
				}

				syms, semaDiags := sema.AnalyzeFile(uri, tree)
				perFileSyms[uri] = syms
				for _, d := range semaDiags {
					results = append(results, validateResult{
						File:     file,
						Line:     d.Range.Start.Line + 1,
						Severity: string(d.Severity),
						Message:  d.Message,
					})
				}
			}

			// Cross-file resolution and workspace-level lint.
			if len(perFileSyms) > 0 {
				ws, wsDiags := sema.MergeWorkspaceSymbols(perFileSyms)
				for _, d := range wsDiags {
					results = append(results, validateResult{
						File:     d.SourceURI,
						Line:     d.Range.Start.Line + 1,
						Severity: string(d.Severity),
						Message:  d.Message,
					})
				}

				_, resDiags := sema.AnalyzeWorkspace(perFileSyms, ws)
				for _, d := range resDiags {
					results = append(results, validateResult{
						File:     d.SourceURI,
						Line:     d.Range.Start.Line + 1,
						Severity: string(d.Severity),
						Message:  d.Message,
					})
				}

				for _, d := range sema.LintWorkspace(perFileTrees, ws) {
					results = append(results, validateResult{
						File:     d.SourceURI,
						Line:     d.Range.Start.Line + 1,
						Severity: string(d.Severity),
						Message:  d.Message,
					})
				}
			}

			switch format {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.Encode(results)
			default:
				for _, r := range results {
					if r.Line > 0 {
						fmt.Fprintf(os.Stderr, "%s:%d: %s: %s\n", r.File, r.Line, r.Severity, r.Message)
					} else {
						fmt.Fprintf(os.Stderr, "%s: %s: %s\n", r.File, r.Severity, r.Message)
					}
				}
			}

			hasFailure := false
			for _, r := range results {
				if r.Severity == string(craft.SeverityError) || (strict && r.Severity == string(craft.SeverityWarning)) {
					hasFailure = true
					break
				}
			}
			if hasFailure {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "promote warnings to errors")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text|json")
	return cmd
}
