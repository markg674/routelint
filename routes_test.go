package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestParseMuxPattern(t *testing.T) {
	cases := []struct {
		name       string
		pattern    string
		wantMethod string
		wantHost   string
		wantPath   string
		wantErr    bool
	}{
		{name: "plain path", pattern: "/users", wantPath: "/users"},
		{name: "method and path", pattern: "GET /users", wantMethod: "GET", wantPath: "/users"},
		{name: "method host and path", pattern: "GET example.com/users", wantMethod: "GET", wantHost: "example.com", wantPath: "/users"},
		{name: "host without method", pattern: "example.com/users", wantHost: "example.com", wantPath: "/users"},
		{name: "no slash at all", pattern: "users", wantErr: true},
		{name: "wildcard host is an error", pattern: "GET {host}/users", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method, host, path, err := parseMuxPattern(tc.pattern)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseMuxPattern(%q): expected error, got none", tc.pattern)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMuxPattern(%q): unexpected error: %v", tc.pattern, err)
			}
			if method != tc.wantMethod || host != tc.wantHost || path != tc.wantPath {
				t.Fatalf("parseMuxPattern(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.pattern, method, host, path, tc.wantMethod, tc.wantHost, tc.wantPath)
			}
		})
	}
}

func TestNormalizeMuxPath(t *testing.T) {
	cases := map[string]string{
		"/users/{id}":     "/users/{}",
		"/users/{name}":   "/users/{}",
		"/files/{path...}": "/files/{...}",
		"/exact/{$}":      "/exact/{$}",
		"":                "",
		"/":               "/",
	}
	for in, want := range cases {
		if got := normalizeMuxPath(in); got != want {
			t.Errorf("normalizeMuxPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsHTTPMethod(t *testing.T) {
	cases := map[string]bool{
		"GET":         true,
		"POST-CUSTOM": true,
		"get":         false,
		"":            false,
		"GET1":        false,
	}
	for in, want := range cases {
		if got := isHTTPMethod(in); got != want {
			t.Errorf("isHTTPMethod(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLooksLikeHost(t *testing.T) {
	cases := map[string]bool{
		"localhost":        true,
		"example.com":      true,
		"example.com:8080": true,
		"*.example.com":    true,
		"users":            false,
		"":                 false,
	}
	for in, want := range cases {
		if got := looksLikeHost(in); got != want {
			t.Errorf("looksLikeHost(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCheckPatterns(t *testing.T) {
	routes := []Route{
		{File: "fake.go", Line: 1, Method: "HandleFunc", Pattern: "/users"},
		{File: "fake.go", Line: 2, Method: "HandleFunc", Pattern: "/users"},
		{File: "fake.go", Line: 3, Method: "HandleFunc", Pattern: "GET /items/{id}"},
		{File: "fake.go", Line: 4, Method: "HandleFunc", Pattern: "GET /items/{name}"},
		{File: "fake.go", Line: 5, Method: "HandleFunc", Pattern: "users/missing-slash"},
		{File: "fake.go", Line: 6, Method: "HandleFunc", Pattern: "/double//slash"},
		{File: "fake.go", Line: 7, Method: "HandleFunc", Pattern: ""},
	}

	want := []struct {
		line     int
		contains string
	}{
		{2, "duplicate route"},
		{4, "conflicts with"},
		{5, "does not start with /"},
		{6, "doubled slash"},
		{7, "empty route pattern"},
	}

	findings := checkPatterns(routes)
	if len(findings) != len(want) {
		t.Fatalf("checkPatterns() returned %d findings, want %d: %+v", len(findings), len(want), findings)
	}
	for i, f := range findings {
		if f.Line != want[i].line {
			t.Errorf("finding %d: line = %d, want %d (%q)", i, f.Line, want[i].line, f.Message)
			continue
		}
		if !strings.Contains(f.Message, want[i].contains) {
			t.Errorf("finding %d at line %d: message %q does not contain %q", i, f.Line, f.Message, want[i].contains)
		}
	}
}

// TestExtractAndCheckFixture exercises extraction and checking together
// against a real parsed file, the same way main does, rather than just
// against hand-built Route values.
func TestExtractAndCheckFixture(t *testing.T) {
	const path = "testdata/fixtures/routes.go"

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}

	routes := extractRoutes(fset, file, path)
	if len(routes) != 7 {
		t.Fatalf("extractRoutes() returned %d routes, want 7: %+v", len(routes), routes)
	}
	for _, r := range routes {
		if r.Method != "HandleFunc" {
			t.Errorf("route at line %d: Method = %q, want HandleFunc", r.Line, r.Method)
		}
	}

	findings := checkPatterns(routes)
	wantLines := []int{8, 10, 11, 12, 13}
	if len(findings) != len(wantLines) {
		t.Fatalf("checkPatterns() returned %d findings, want %d: %+v", len(findings), len(wantLines), findings)
	}
	for i, f := range findings {
		if f.File != path {
			t.Errorf("finding %d: File = %q, want %q", i, f.File, path)
		}
		if f.Line != wantLines[i] {
			t.Errorf("finding %d: Line = %d, want %d", i, f.Line, wantLines[i])
		}
	}
}
