package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/ast"
	"github.com/tcarcao/craft/internal/sema"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

// lspJSONOutput is the shape emitted by `craft check --lsp-json`.
// It mirrors the LSP responses (diagnostics + documentSymbol + hover hint)
// so bugs can be reproduced without running the full LSP server (Q9e / Q13).
type lspJSONOutput struct {
	CraftDoc    interface{}     `json:"craftDoc"`
	Diagnostics json.RawMessage `json:"diagnostics"`
	Symbols     json.RawMessage `json:"symbols"`
}

func checkCmd() *cobra.Command {
	var lspJSON bool

	cmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Parse a .craft file and emit CraftDoc JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", args[0], err)
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			astFile, parseDiags := syntax.Parse(string(content))
			doc := syntax.Project(astFile)

			uri := "file://" + args[0]
			_, semaDiags := sema.AnalyzeFile(uri, astFile)

			allDiags := append(parseDiags, semaDiags...)
			if allDiags == nil {
				allDiags = []craft.Diagnostic{}
			}

			if lspJSON {
				diagBytes, _ := json.Marshal(allDiags)
				symbols := buildSymbolsJSON(astFile)
				symBytes, _ := json.Marshal(symbols)
				return enc.Encode(lspJSONOutput{
					CraftDoc:    doc,
					Diagnostics: diagBytes,
					Symbols:     symBytes,
				})
			}
			return enc.Encode(doc)
		},
	}
	cmd.Flags().BoolVar(&lspJSON, "lsp-json", false, "emit diagnostics+symbols+craftDoc as JSON (mirrors LSP responses)")
	return cmd
}

// symbolInfo is a simplified document-symbol shape for --lsp-json output.
type symbolInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
	Type string `json:"type,omitempty"`
}

func buildSymbolsJSON(f *ast.File) []symbolInfo {
	out := []symbolInfo{}
	for _, a := range f.Actors {
		out = append(out, symbolInfo{
			Name: a.Name,
			Kind: "actor",
			Line: a.Line,
			Type: string(a.Type),
		})
	}
	return out
}
