package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/internal/syntax"
	"github.com/tcarcao/craft/pkg/craft"
)

type inspectOutput struct {
	Files    []string        `json:"files"`
	Actors   []craft.Actor   `json:"actors,omitempty"`
	Domains  []craft.Domain  `json:"domains,omitempty"`
	Services []craft.Service `json:"services,omitempty"`
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

			merged := &craft.CraftDoc{}
			for _, file := range files {
				content, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				greenRoot, li, _ := syntax.Parse(string(content))
				tree := syntax.Root(greenRoot)
				doc := syntax.ProjectFromTree(tree, li)
				merged.Actors = append(merged.Actors, doc.Actors...)
				merged.Domains = append(merged.Domains, doc.Domains...)
				merged.Services = append(merged.Services, doc.Services...)
				merged.UseCases = append(merged.UseCases, doc.UseCases...)
			}

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

func buildInspectOutput(files []string, doc *craft.CraftDoc) inspectOutput {
	out := inspectOutput{
		Files:    files,
		Actors:   doc.Actors,
		Domains:  doc.Domains,
		Services: doc.Services,
	}

	for _, uc := range doc.UseCases {
		sum := useCaseSummary{Name: uc.Name}
		pubSeen := map[string]bool{}
		conSeen := map[string]bool{}
		for _, sc := range uc.Scenarios {
			if (sc.Trigger.Type == craft.TriggerTypeEvent || sc.Trigger.Type == craft.TriggerTypeDomainListen) && sc.Trigger.Event != "" && !conSeen[sc.Trigger.Event] {
				conSeen[sc.Trigger.Event] = true
				sum.EventsConsumed = append(sum.EventsConsumed, sc.Trigger.Event)
			}
			for _, a := range sc.Actions {
				if a.Type == craft.ActionTypeAsync && a.Event != "" && !pubSeen[a.Event] {
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
