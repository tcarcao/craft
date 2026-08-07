package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "craft",
		Short:   "Craft — DDD modeling CLI",
		Long:    "Parse, lint, inspect, and generate diagrams from .craft files.",
		Version: version,
	}
	root.AddCommand(validateCmd(), generateCmd(), inspectCmd(), checkCmd(), fmtCmd(), lspCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
