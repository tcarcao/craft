package visualizer

import "strings"

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
