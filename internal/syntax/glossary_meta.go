package syntax

// glossaryVerbs is the single source of truth for glossary relation verbs —
// cross-context term relations declared inside a `glossary { }` block.
// Unlike context_map's edge verbs (see edgeKeywords in parser.go), all three
// glossary verbs are symmetric (no upstream/downstream distinction), so
// there is no accompanying role/class metadata — just the ordered verb list.
var glossaryVerbs = []string{"same_as", "contrasts", "distinct_from"}

// isGlossaryVerb reports whether s is a recognised glossary relation verb.
func isGlossaryVerb(s string) bool {
	for _, v := range glossaryVerbs {
		if v == s {
			return true
		}
	}
	return false
}

// GlossaryVerbs returns the glossary relation verbs the parser accepts.
// Exported so sema (and future docs/tooling) can verify its own verb
// classification stays in sync instead of hand-copying the list.
func GlossaryVerbs() []string {
	return append([]string(nil), glossaryVerbs...)
}
