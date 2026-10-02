package visualizer

import (
	"fmt"
	"strings"
	"unicode"
)

// serviceLanguage is what the C4 diagram knows about a service's `language:`.
type serviceLanguage struct {
	technology string // container technology label
	sprite     string // tupadr3 sprite name; a key of languageIncludePaths
}

// languageAliases maps a whole word of a `language:` value to its language.
// Matching is by word, never by substring: "go" must not match "django" or
// "mongo", and "java" must not match "javascript".
var languageAliases = map[string]serviceLanguage{
	"go":         {"Go Application", "go"},
	"golang":     {"Go Application", "go"},
	"java":       {"Java Application", "java"},
	"python":     {"Python Application", "python"},
	"javascript": {"Node.js Application", "nodejs"},
	"js":         {"Node.js Application", "nodejs"},
	"node":       {"Node.js Application", "nodejs"},
	"nodejs":     {"Node.js Application", "nodejs"},
	"typescript": {"TypeScript Application", "javascript"},
	"ts":         {"TypeScript Application", "javascript"},
	"rust":       {"Rust Application", "rust"},
	"csharp":     {".NET Application", "dot_net"},
	"c#":         {".NET Application", "dot_net"},
	"dotnet":     {".NET Application", "dot_net"},
	"php":        {"PHP Application", "php"},
	"ruby":       {"Ruby Application", "ruby"},
	"kotlin":     {"Kotlin Application", "kotlin"},
	"swift":      {"Swift Application", "swift"},
}

// lookupLanguage resolves a `language:` value by its first recognised word.
// Words are split on anything that is not a letter, digit or '#', so
// "Node.js", "golang/gin" and "C# 12" all resolve.
func lookupLanguage(language string) (serviceLanguage, bool) {
	words := strings.FieldsFunc(strings.ToLower(language), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '#'
	})
	for _, w := range words {
		if l, ok := languageAliases[w]; ok {
			return l, true
		}
	}
	return serviceLanguage{}, false
}

func (g *C4DiagramGenerator) getServiceTechnology(language string) string {
	if language == "" {
		return "Application"
	}
	if l, ok := lookupLanguage(language); ok {
		return l.technology
	}
	return fmt.Sprintf("%s Application", strings.Title(language))
}

// getServiceIcon returns the C4 sprite attribute for a service language.
func (g *C4DiagramGenerator) getServiceIcon(language string) string {
	if language == "" {
		return ""
	}
	if l, ok := lookupLanguage(language); ok {
		return fmt.Sprintf(", $sprite=%q", l.sprite)
	}
	return ", $sprite=\"code\""
}
