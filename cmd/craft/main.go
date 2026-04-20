package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "craft",
		Short: "Craft — DDD modeling CLI",
		Long:  "Parse, lint, inspect, and generate diagrams from .craft files.",
	}

	root.AddCommand(validateCmd(), generateCmd(), inspectCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
