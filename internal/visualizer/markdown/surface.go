// Package markdown writes the views that are tables rather than diagrams.
package markdown

import (
	"strings"

	"github.com/tcarcao/craft/v2/internal/lang"
	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// Surfaces writes one section per surface: its callers, spec and notes, then a
// table of its operations and the use cases that use each one (spec §8.4).
func Surfaces(v view.Surfaces) string {
	var sb strings.Builder
	sb.WriteString("# Surfaces\n")
	for _, t := range v.Tables {
		heading := t.Subject
		if t.DisplayName != "" {
			// The id stays beside the display name: the table is where a
			// reader looks up the boundary in the source.
			heading = t.DisplayName + " (" + t.Subject + ")"
		}
		sb.WriteString("\n## " + heading + "\n\n")
		header := false
		if len(t.Callers) > 0 {
			sb.WriteString("Callers: " + strings.Join(t.Callers, ", ") + "\n")
			header = true
		}
		if t.Spec != "" {
			sb.WriteString("Spec: `" + t.Spec + "`\n")
			header = true
		}
		if header {
			sb.WriteString("\n")
		}
		for _, n := range t.Notes {
			if n.Kind == lang.NoteFinding {
				sb.WriteString("> **Finding:** " + n.Text + "\n\n")
			} else {
				sb.WriteString("> " + n.Text + "\n\n")
			}
		}
		sb.WriteString("| Operation | Kind | What it offers | Used by |\n|---|---|---|---|\n")
		for _, r := range t.Rows {
			usedBy := "—"
			if len(r.UsedBy) > 0 {
				usedBy = strings.Join(r.UsedBy, ", ")
			}
			sb.WriteString("| `" + cell(r.Operation) + "` | " + string(r.Kind) + " | " + cell(r.Offers) + " | " + cell(usedBy) + " |\n")
		}
	}
	return sb.String()
}

// cell escapes the table separator in a cell's text.
func cell(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
