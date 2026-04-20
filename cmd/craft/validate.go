package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/linter"
	"github.com/tcarcao/craft/internal/parser"
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

			var results []validateResult
			var models []*parser.DSLModel

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

				p := parser.NewParser()
				model, err := p.ParseString(string(content))
				if err != nil {
					for _, se := range p.SyntaxErrors() {
						results = append(results, validateResult{
							File:     file,
							Line:     se.Line,
							Severity: "error",
							Message:  se.Message,
						})
					}
					continue
				}
				models = append(models, model)
			}

			if len(models) > 0 {
				merged := parser.MergeModels(models)
				for _, r := range linter.Lint(files, merged) {
					results = append(results, validateResult{
						File:     r.File,
						Line:     r.Line,
						Severity: string(r.Severity),
						Message:  r.Message,
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
				if r.Severity == "error" || (strict && r.Severity == "warning") {
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
