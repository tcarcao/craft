package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/parser"
)

type inspectOutput struct {
	Files    []string        `json:"files"`
	Actors   []parser.Actor  `json:"actors,omitempty"`
	Domains  []parser.Domain `json:"domains,omitempty"`
	Services []parser.Service `json:"services,omitempty"`
	UseCases []useCaseSummary `json:"use_cases,omitempty"`
}

type useCaseSummary struct {
	Name            string   `json:"name"`
	EventsPublished []string `json:"events_published,omitempty"`
	EventsConsumed  []string `json:"events_consumed,omitempty"`
}

func inspectCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "inspect [files...]",
		Short: "Parse .craft files and emit the structured model",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := resolveFiles(args)
			if err != nil {
				return err
			}

			var models []*parser.DSLModel
			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				p := parser.NewParser()
				model, err := p.ParseString(string(content))
				if err != nil {
					return fmt.Errorf("%s: parse error: %w", file, err)
				}
				models = append(models, model)
			}

			merged := parser.MergeModels(models)
			out := buildInspectOutput(files, merged)

			switch format {
			case "json":
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			default:
				return printInspectText(out)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text|json")
	return cmd
}

func buildInspectOutput(files []string, model *parser.DSLModel) inspectOutput {
	out := inspectOutput{
		Files:    files,
		Actors:   model.Actors,
		Domains:  model.Domains,
		Services: model.Services,
	}

	for _, uc := range model.UseCases {
		sum := useCaseSummary{Name: uc.Name}
		pubSeen := map[string]bool{}
		conSeen := map[string]bool{}
		for _, sc := range uc.Scenarios {
			if (sc.Trigger.Type == parser.TriggerTypeEvent || sc.Trigger.Type == parser.TriggerTypeDomainListen) && !conSeen[sc.Trigger.Event] {
				conSeen[sc.Trigger.Event] = true
				sum.EventsConsumed = append(sum.EventsConsumed, sc.Trigger.Event)
			}
			for _, a := range sc.Actions {
				if a.Type == parser.ActionTypeAsync && !pubSeen[a.Event] {
					pubSeen[a.Event] = true
					sum.EventsPublished = append(sum.EventsPublished, a.Event)
				}
			}
		}
		out.UseCases = append(out.UseCases, sum)
	}

	return out
}

func printInspectText(out inspectOutput) error {
	fmt.Printf("Files: %v\n", out.Files)
	fmt.Printf("Actors (%d):\n", len(out.Actors))
	for _, a := range out.Actors {
		fmt.Printf("  %s (%s)\n", a.Name, a.Type)
	}
	fmt.Printf("Domains (%d):\n", len(out.Domains))
	for _, d := range out.Domains {
		fmt.Printf("  %s: %v\n", d.Name, d.BoundedContexts)
	}
	fmt.Printf("Services (%d):\n", len(out.Services))
	for _, s := range out.Services {
		fmt.Printf("  %s (contexts: %v)\n", s.Name, s.Contexts)
	}
	fmt.Printf("Use cases (%d):\n", len(out.UseCases))
	for _, uc := range out.UseCases {
		fmt.Printf("  %s\n", uc.Name)
		if len(uc.EventsPublished) > 0 {
			fmt.Printf("    publishes: %v\n", uc.EventsPublished)
		}
		if len(uc.EventsConsumed) > 0 {
			fmt.Printf("    consumes: %v\n", uc.EventsConsumed)
		}
	}
	return nil
}
