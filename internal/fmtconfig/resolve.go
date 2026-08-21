package fmtconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the per-workspace configuration file.
const FileName = ".craftfmt"

// Find returns the path of the nearest .craftfmt at or above startDir, or ""
// when there is none. It stops at the filesystem root rather than at a repo
// boundary, because a workspace is not necessarily a git repo and the formatter
// has no other notion of where a project stops.
func Find(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, FileName)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Load decodes one .craftfmt over the defaults, so an absent key keeps its
// default rather than becoming the zero value. An unknown key is an error:
// silently ignoring `indnet = 4` would leave the author believing they had
// configured something. Every unknown key is reported at once -- md.Undecoded
// already has the full list cheaply -- so a file with two typos does not cost
// the author two runs to find them both.
func Load(path string) (Config, error) {
	cfg := Defaults()
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return Config{}, fmt.Errorf("%s: unknown key(s): %s", path, strings.Join(keys, ", "))
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Resolve returns the configuration that applies to one .craft file: the
// nearest ancestor .craftfmt if there is one, otherwise the defaults.
//
// CLI flags are applied by the caller on top of this, because only the caller
// knows which flags the user actually set; see cmd/craft/fmt.go.
func Resolve(filePath string) (Config, error) {
	dir := filepath.Dir(filePath)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	found := Find(dir)
	if found == "" {
		return Defaults(), nil
	}
	return Load(found)
}
