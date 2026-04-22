package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/parser"
	"github.com/tcarcao/craft/internal/parser_antlr_adapter"
)

func checkCmd() *cobra.Command {
	var parserFlag string

	cmd := &cobra.Command{
		Use:   "check <file>",
		Short: "Parse a .craft file and emit CraftDoc JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("cannot read %s: %w", args[0], err)
			}
			switch parserFlag {
			case "antlr":
				p := parser.NewParser()
				model, err := p.ParseString(string(content))
				if err != nil {
					for _, se := range p.SyntaxErrors() {
						fmt.Fprintf(os.Stderr, "%s:%d: error: %s\n", args[0], se.Line, se.Message)
					}
					return fmt.Errorf("parse failed")
				}
				doc := parser_antlr_adapter.FromDSLModel(model)
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(doc)
			case "v2":
				return fmt.Errorf("v2 parser: not yet implemented")
			default:
				return fmt.Errorf("unknown --parser value %q; want antlr|v2", parserFlag)
			}
		},
	}
	cmd.Flags().StringVar(&parserFlag, "parser", "antlr", "parser to use: antlr|v2")
	return cmd
}
