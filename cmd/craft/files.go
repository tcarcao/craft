package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// resolveFiles expands glob patterns (including **) into file paths.
// A directory (given literally or matched by a glob) expands recursively to
// the *.craft files under it, sorted; a directory with none is an error.
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
			if err != nil {
				continue
			}
			if info.IsDir() {
				found, err := craftFilesUnder(m)
				if err != nil {
					return nil, err
				}
				for _, f := range found {
					if !seen[f] {
						seen[f] = true
						files = append(files, f)
					}
				}
				continue
			}
			seen[m] = true
			files = append(files, m)
		}
	}

	return files, nil
}

// craftFilesUnder returns the *.craft files below dir in lexical order.
func craftFilesUnder(dir string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".craft") {
			found = append(found, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no .craft files found under directory %s", dir)
	}
	sort.Strings(found)
	return found, nil
}
