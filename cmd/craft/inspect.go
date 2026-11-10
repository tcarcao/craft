package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tcarcao/craft/v2/pkg/craft"
)

type inspectOutput struct {
	Files    []string         `json:"files"`
	Actors   []craft.Actor    `json:"actors,omitempty"`
	Domains  []craft.Domain   `json:"domains,omitempty"`
	Services []craft.Service  `json:"services,omitempty"`
	UseCases []useCaseSummary `json:"use_cases,omitempty"`
	Flows    []flowSummary    `json:"flows,omitempty"`
	Pages    []pageSummary    `json:"pages,omitempty"`
}

type useCaseSummary struct {
	Name            string   `json:"name"`
	EventsPublished []string `json:"events_published,omitempty"`
	EventsConsumed  []string `json:"events_consumed,omitempty"`
	Realises        []string `json:"realises,omitempty"`
}

// flowSummary is one critical flow (v2.35) and the use cases that realise it.
type flowSummary struct {
	Ref        string   `json:"ref"`
	Name       string   `json:"name"`
	Priority   string   `json:"priority,omitempty"`
	Owner      string   `json:"owner,omitempty"`
	Metric     string   `json:"metric,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	RealisedBy []string `json:"realised_by,omitempty"`
}

// pageSummary is one page (v2.36) and the use cases that start on it.
type pageSummary struct {
	Ref           string   `json:"ref"`
	Name          string   `json:"name"`
	URLs          []string `json:"urls,omitempty"`
	Platform      string   `json:"platform,omitempty"`
	ProductDomain string   `json:"product_domain,omitempty"`
	Aliases       []string `json:"aliases,omitempty"`
	UsedBy        []string `json:"used_by,omitempty"`
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

			contents := make(map[string][]byte, len(files))
			for _, file := range files {
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				contents[file] = b
			}

			merged, _, err := craft.ParseFiles(contents)
			if err != nil {
				return err
			}

			out := buildInspectOutput(files, merged)

			switch format {
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
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
		sum := useCaseSummary{Name: uc.Name, Realises: uc.Realises}
		pubSeen := map[string]bool{}
		conSeen := map[string]bool{}
		for _, sc := range uc.Scenarios {
			// EventList, not Event: a `listens` list and a `notifies one of`
			// carry every item there; Event holds the first only.
			if sc.Trigger.Type == craft.TriggerTypeEvent || sc.Trigger.Type == craft.TriggerTypeDomainListen {
				for _, ev := range sc.Trigger.EventList() {
					if !conSeen[ev] {
						conSeen[ev] = true
						sum.EventsConsumed = append(sum.EventsConsumed, ev)
					}
				}
			}
			for _, a := range sc.Actions() {
				if a.Type != craft.ActionTypeAsync {
					continue
				}
				for _, ev := range a.EventList() {
					if !pubSeen[ev] {
						pubSeen[ev] = true
						sum.EventsPublished = append(sum.EventsPublished, ev)
					}
				}
			}
		}
		out.UseCases = append(out.UseCases, sum)
	}

	for _, f := range doc.Flows {
		fs := flowSummary{Ref: f.Ref, Name: f.Name, Priority: f.Priority, Owner: f.Owner, Metric: f.Metric, Aliases: f.Aliases}
		for _, r := range f.RealisedBy {
			fs.RealisedBy = append(fs.RealisedBy, r.Name)
		}
		out.Flows = append(out.Flows, fs)
	}

	for _, pg := range doc.Pages {
		ps := pageSummary{Ref: pg.Ref, Name: pg.Name, URLs: pg.URLs, Platform: pg.Platform, ProductDomain: pg.ProductDomain, Aliases: pg.Aliases}
		for _, r := range pg.UsedBy {
			ps.UsedBy = append(ps.UsedBy, r.Name)
		}
		out.Pages = append(out.Pages, ps)
	}

	return out
}

func printInspectText(out inspectOutput) error {
	fmt.Printf("Files: %v\n", out.Files)
	fmt.Printf("Actors (%d):\n", len(out.Actors))
	for _, a := range out.Actors {
		// An actor's body (v2.37) adds its AI kind and owner; one without
		// a body prints as before.
		detail := string(a.Type)
		if a.AI != "" {
			detail += ", ai: " + a.AI
		}
		if a.Owner != "" {
			detail += ", owner: " + a.Owner
		}
		fmt.Printf("  %s (%s)\n", a.Name, detail)
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
		if len(uc.Realises) > 0 {
			fmt.Printf("    realises: %v\n", uc.Realises)
		}
	}
	if len(out.Flows) > 0 {
		fmt.Printf("Flows (%d):\n", len(out.Flows))
		for _, f := range out.Flows {
			fmt.Printf("  %s %q (%s, owner: %s)\n", f.Ref, f.Name, f.Priority, f.Owner)
			if f.Metric != "" {
				fmt.Printf("    metric: %s\n", f.Metric)
			}
			if len(f.Aliases) > 0 {
				fmt.Printf("    aliases: %s\n", strings.Join(f.Aliases, ", "))
			}
			if len(f.RealisedBy) == 0 {
				fmt.Printf("    realised by: none\n")
			} else {
				fmt.Printf("    realised by: %v\n", f.RealisedBy)
			}
		}
	}
	if len(out.Pages) > 0 {
		fmt.Printf("Pages (%d):\n", len(out.Pages))
		for _, pg := range out.Pages {
			fmt.Printf("  %s %q (product domain: %s)\n", pg.Ref, pg.Name, pg.ProductDomain)
			if pg.Platform != "" {
				fmt.Printf("    platform: %s\n", pg.Platform)
			}
			for _, u := range pg.URLs {
				fmt.Printf("    url: %s\n", u)
			}
			if len(pg.Aliases) > 0 {
				fmt.Printf("    aliases: %s\n", strings.Join(pg.Aliases, ", "))
			}
			if len(pg.UsedBy) == 0 {
				fmt.Printf("    used by: none\n")
			} else {
				fmt.Printf("    used by: %v\n", pg.UsedBy)
			}
		}
	}
	return nil
}
