package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

type validateResult struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// runValidate parses the given file contents through pkg/craft.ParseFiles and
// maps diagnostics to validateResult. No I/O, no os.Exit — testable.
func runValidate(contents map[string][]byte, _ bool) []validateResult {
	_, diags, _ := craft.ParseFiles(contents)
	results := make([]validateResult, 0, len(diags))
	for _, d := range diags {
		results = append(results, validateResult{
			File:     d.SourceURI, // bare filename (plan D6)
			Line:     d.Range.Start.Line + 1,
			Severity: string(d.Severity),
			Message:  d.Message,
		})
	}
	return results
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

			contents := make(map[string][]byte, len(files))
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
				contents[file] = content
			}

			results = append(results, runValidate(contents, strict)...)

			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
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
