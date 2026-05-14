// Package mermaid emits Mermaid diagram source from a CraftDoc.
// Generators in this package are stateless free functions; they take a *craft.CraftDoc
// (already filtered by the caller for --use-case) and return the Mermaid source string.
//
// Output text is renderer-agnostic: it works in GitHub markdown, VS Code's Mermaid
// preview, mermaid-cli, and the Mermaid Live Editor without further processing.
package mermaid

import craft "github.com/tcarcao/craft/pkg/craft"

// referencedActors returns the set of actor names that appear as the
// triggering actor of any TriggerTypeExternal scenario in doc. Generators
// use this to exclude declared-but-unused actors from rendered output —
// most importantly in split mode, where a single-use-case doc carries
// only the actors that actually drive that use case.
func referencedActors(doc *craft.CraftDoc) map[string]bool {
	out := make(map[string]bool)
	for _, uc := range doc.UseCases {
		for _, sc := range uc.Scenarios {
			if sc.Trigger.Type == craft.TriggerTypeExternal && sc.Trigger.Actor != "" {
				out[sc.Trigger.Actor] = true
			}
		}
	}
	return out
}
