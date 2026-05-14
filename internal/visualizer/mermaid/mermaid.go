// Package mermaid emits Mermaid diagram source from a CraftDoc.
// Generators in this package are stateless free functions; they take a *craft.CraftDoc
// (already filtered by the caller for --use-case) and return the Mermaid source string.
//
// Output text is renderer-agnostic: it works in GitHub markdown, VS Code's Mermaid
// preview, mermaid-cli, and the Mermaid Live Editor without further processing.
package mermaid
