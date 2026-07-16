package craft

import "github.com/tcarcao/craft/v2/internal/syntax"

// GlossaryVerbs returns all glossary relationship verbs supported by the language.
func GlossaryVerbs() []string {
	return syntax.GlossaryVerbs()
}
