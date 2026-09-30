package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// defaultConfigName is looked up in the working directory when -config is
// not given. A missing default file is fine; a missing explicit one is not.
const defaultConfigName = ".routelint.json"

// Config holds user-supplied exclusions. It is JSON rather than anything
// fancier because the standard library has no YAML or TOML parser.
type Config struct {
	// IgnoreFiles lists globs for files that should not be scanned. A glob
	// is tried against the whole slash-separated path and against the base
	// name alone. A glob ending in "/" excludes everything under that
	// directory prefix.
	IgnoreFiles []string `json:"ignore_files"`

	// IgnorePatterns lists route patterns to drop before any check runs,
	// compared against the effective pattern (group prefixes already
	// applied). Each entry is an exact string or a path.Match glob.
	IgnorePatterns []string `json:"ignore_patterns"`
}

// loadConfig reads a config file. If path is empty it tries the default
// name and returns an empty Config when that file does not exist. Unknown
// keys are rejected so a misspelled option doesn't silently do nothing.
func loadConfig(configPath string) (Config, error) {
	explicit := configPath != ""
	if !explicit {
		configPath = defaultConfigName
	}

	f, err := os.Open(configPath)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	defer f.Close()

	var cfg Config
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", configPath, err)
	}

	// path.Match only reports a bad glob when it gets far enough to see it,
	// so validate every entry up front rather than on first use.
	for _, g := range append(append([]string{}, cfg.IgnoreFiles...), cfg.IgnorePatterns...) {
		if _, err := path.Match(g, ""); err != nil {
			return Config{}, fmt.Errorf("%s: bad glob %q: %w", configPath, g, err)
		}
	}
	return cfg, nil
}

// ignoresFile reports whether the config excludes the given file path.
func (c Config) ignoresFile(file string) bool {
	p := path.Clean(filepath.ToSlash(file))
	base := path.Base(p)
	for _, g := range c.IgnoreFiles {
		if strings.HasSuffix(g, "/") {
			if strings.HasPrefix(p+"/", path.Clean(g)+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, p); ok {
			return true
		}
		if ok, _ := path.Match(g, base); ok {
			return true
		}
	}
	return false
}

// ignoresPattern reports whether the config excludes a route pattern.
func (c Config) ignoresPattern(pattern string) bool {
	for _, g := range c.IgnorePatterns {
		if g == pattern {
			return true
		}
		if ok, _ := path.Match(g, pattern); ok {
			return true
		}
	}
	return false
}

// filterFiles drops ignored files from a list of paths.
func (c Config) filterFiles(files []string) []string {
	var kept []string
	for _, f := range files {
		if !c.ignoresFile(f) {
			kept = append(kept, f)
		}
	}
	return kept
}

// filterRoutes drops routes whose pattern is ignored. This happens before
// the duplicate check, so an ignored route can't be reported as the original
// that a later registration collides with.
func (c Config) filterRoutes(routes []Route) []Route {
	var kept []Route
	for _, r := range routes {
		if !c.ignoresPattern(r.Pattern) {
			kept = append(kept, r)
		}
	}
	return kept
}
