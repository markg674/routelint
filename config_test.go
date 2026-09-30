package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return p
}

func TestLoadConfig(t *testing.T) {
	p := writeConfig(t, `{"ignore_files": ["*_gen.go"], "ignore_patterns": ["/healthz"]}`)
	cfg, err := loadConfig(p)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.IgnoreFiles) != 1 || cfg.IgnoreFiles[0] != "*_gen.go" {
		t.Errorf("IgnoreFiles = %v", cfg.IgnoreFiles)
	}
	if len(cfg.IgnorePatterns) != 1 || cfg.IgnorePatterns[0] != "/healthz" {
		t.Errorf("IgnorePatterns = %v", cfg.IgnorePatterns)
	}
}

func TestLoadConfigMissingExplicit(t *testing.T) {
	if _, err := loadConfig(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected an error for a missing explicit config file")
	}
}

func TestLoadConfigMissingDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig(\"\"): %v", err)
	}
	if len(cfg.IgnoreFiles) != 0 || len(cfg.IgnorePatterns) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoadConfigRejectsUnknownKey(t *testing.T) {
	p := writeConfig(t, `{"ignore_file": ["a.go"]}`)
	if _, err := loadConfig(p); err == nil {
		t.Fatal("expected an error for an unknown key")
	}
}

func TestLoadConfigRejectsBadGlob(t *testing.T) {
	p := writeConfig(t, `{"ignore_patterns": ["/a/[b"]}`)
	if _, err := loadConfig(p); err == nil {
		t.Fatal("expected an error for a malformed glob")
	}
}

func TestIgnoresFile(t *testing.T) {
	cfg := Config{IgnoreFiles: []string{"*_gen.go", "internal/legacy/", "cmd/old/main.go"}}
	tests := []struct {
		file string
		want bool
	}{
		{"api/users_gen.go", true},
		{"api/users.go", false},
		{"internal/legacy/routes.go", true},
		{"./internal/legacy/deep/routes.go", true},
		{"internal/legacy2/routes.go", false},
		{"cmd/old/main.go", true},
		{"cmd/new/main.go", false},
	}
	for _, tt := range tests {
		if got := cfg.ignoresFile(tt.file); got != tt.want {
			t.Errorf("ignoresFile(%q) = %v, want %v", tt.file, got, tt.want)
		}
	}
}

func TestIgnoresPattern(t *testing.T) {
	cfg := Config{IgnorePatterns: []string{"/healthz", "GET /debug/*"}}
	tests := []struct {
		pattern string
		want    bool
	}{
		{"/healthz", true},
		{"GET /debug/pprof", true},
		{"GET /debug/pprof/heap", false},
		{"/users", false},
	}
	for _, tt := range tests {
		if got := cfg.ignoresPattern(tt.pattern); got != tt.want {
			t.Errorf("ignoresPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

func TestFilterRoutesRemovesIgnoredDuplicate(t *testing.T) {
	cfg := Config{IgnorePatterns: []string{"/healthz"}}
	routes := []Route{
		{File: "a.go", Line: 1, Method: "HandleFunc", Pattern: "/healthz"},
		{File: "b.go", Line: 1, Method: "HandleFunc", Pattern: "/healthz"},
		{File: "a.go", Line: 2, Method: "HandleFunc", Pattern: "/users"},
	}
	if got := checkPatterns(cfg.filterRoutes(routes)); len(got) != 0 {
		t.Fatalf("expected no findings, got %+v", got)
	}
	if got := checkPatterns(routes); len(got) != 1 {
		t.Fatalf("sanity check: expected the duplicate without a config, got %+v", got)
	}
}

func TestFilterFiles(t *testing.T) {
	cfg := Config{IgnoreFiles: []string{"skip.go"}}
	got := cfg.filterFiles([]string{"a/skip.go", "a/keep.go"})
	if len(got) != 1 || got[0] != "a/keep.go" {
		t.Fatalf("filterFiles = %v", got)
	}
}
