package main

import (
	"fmt"
	"os"

	"github.com/bmatcuk/doublestar/v4"
)

// resolveFiles expands glob patterns (including **) into file paths.
// Literal paths that don't match a glob are passed through as-is.
func resolveFiles(patterns []string) ([]string, error) {
	var files []string
	seen := map[string]bool{}

	for _, pattern := range patterns {
		matches, err := doublestar.FilepathGlob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob %q: %w", pattern, err)
		}

		if len(matches) == 0 {
			// treat as a literal path — let callers handle missing files
			if !seen[pattern] {
				seen[pattern] = true
				files = append(files, pattern)
			}
			continue
		}

		for _, m := range matches {
			if seen[m] {
				continue
			}
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			seen[m] = true
			files = append(files, m)
		}
	}

	return files, nil
}
