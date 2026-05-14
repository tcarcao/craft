package visualizer

import (
	"strings"

	craft "github.com/tcarcao/craft/pkg/craft"
)

// Slugify converts a human-readable name into a deterministic kebab-case slug.
// The mapping must be one-way stable: callers compare two slugs by string
// equality after applying Slugify to both sides.
//
// Rules:
//   - Lowercase.
//   - Any run of characters outside [a-z0-9] collapses to a single '-'.
//   - Leading and trailing '-' are trimmed.
//
// Empty input produces the empty string.
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevDash := true // suppresses leading dashes
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// FilterUseCases returns a shallow copy of doc keeping only UseCases that
// match one of `values`. Each value matches by either exact name or by
// Slugify(name) == Slugify(value). Order of use cases in the result follows
// the source document, not the order of values in the filter.
//
// The second return value lists `values` that did not match any use case in
// the doc, in the order they appeared in `values`. Callers use this to
// surface user-visible errors with the original spelling.
//
// Passing an empty or nil `values` slice returns the source doc unchanged
// (same pointer) and a nil missing slice.
func FilterUseCases(doc *craft.CraftDoc, values []string) (*craft.CraftDoc, []string) {
	if len(values) == 0 {
		return doc, nil
	}

	wanted := make(map[string]bool, len(values))
	for _, v := range values {
		wanted[Slugify(v)] = true
	}

	out := *doc // shallow copy; we replace UseCases below
	out.UseCases = make([]craft.UseCase, 0, len(doc.UseCases))
	matched := make(map[string]bool, len(values))

	for _, uc := range doc.UseCases {
		slug := Slugify(uc.Name)
		if wanted[slug] {
			out.UseCases = append(out.UseCases, uc)
			matched[slug] = true
		}
	}

	var missing []string
	for _, v := range values {
		if !matched[Slugify(v)] {
			missing = append(missing, v)
		}
	}
	return &out, missing
}
