// Package mermaid emits Mermaid diagram source from a CraftDoc.
// Generators in this package are stateless free functions; they take a *craft.CraftDoc
// (already filtered by the caller for --use-case) and return the Mermaid source string.
//
// Output text is renderer-agnostic: it works in GitHub markdown, VS Code's Mermaid
// preview, mermaid-cli, and the Mermaid Live Editor without further processing.
package mermaid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tcarcao/craft/v2/internal/visualizer/view"
)

// safeID renders a Mermaid-safe identifier from a human-readable name.
// Mermaid permits letters, digits, and underscore in node IDs but not
// spaces, hyphens, or emoji. We map disallowed runs to '_'.
func safeID(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prev := true
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		default:
			if !prev {
				b.WriteByte('_')
				prev = true
			}
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// aliases maps view node IDs to unique Mermaid identifiers, registered in
// declaration order so the mapping is deterministic.
type aliases struct {
	byID map[string]string
	used map[string]bool
}

func newAliases() *aliases { return &aliases{byID: map[string]string{}, used: map[string]bool{}} }

func (a *aliases) of(id string) string {
	if al, ok := a.byID[id]; ok {
		return al
	}
	return a.declare(id, id)
}

// declare registers a node, basing its identifier on the label the reader
// sees; a clash with an earlier node gets a numeric suffix.
func (a *aliases) declare(id, label string) string {
	if al, ok := a.byID[id]; ok {
		return al
	}
	base := safeID(label)
	if base == "" {
		base = "node"
	}
	al := base
	for n := 2; a.used[al]; n++ {
		al = fmt.Sprintf("%s_%d", base, n)
	}
	a.byID[id], a.used[al] = al, true
	return al
}

// name is the text drawn for a node: its display name wrapped at
// view.NameWidth with <br/> (which Mermaid's flowchart, sequence and C4
// renderers all break at), else label, which a display name never changes.
func name(display, label string) string {
	if display == "" {
		return label
	}
	return strings.Join(view.WrapName(display), "<br/>")
}

// entity matches what Mermaid decodes as an entity code inside a quoted
// string: "#2;" would be drawn as the character 2.
var entity = regexp.MustCompile(`#(\w+;)`)

// text makes a label safe inside a double-quoted Mermaid string: a '#' that
// would start an entity code is written as "#35;", and '"' as "#quot;". A
// label with no such '#' is unchanged but for its quotes.
func text(s string) string {
	return strings.ReplaceAll(entity.ReplaceAllString(s, "#35;$1"), `"`, "#quot;")
}
