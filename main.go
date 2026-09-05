// Command routelint scans Go source for URL route registrations
// (net/http and chi/gorilla-style routers) and reports suspicious
// patterns: duplicates, missing leading slashes, doubled slashes.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"."}
	}

	var files []string
	for _, arg := range args {
		found, err := collectGoFiles(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "routelint: %v\n", err)
			os.Exit(2)
		}
		files = append(files, found...)
	}

	fset := token.NewFileSet()
	var routes []Route
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "routelint: %v\n", err)
			os.Exit(2)
		}
		routes = append(routes, extractRoutes(fset, file, path)...)
	}

	findings := checkPatterns(routes)
	for _, f := range findings {
		fmt.Printf("%s:%d: %s\n", f.File, f.Line, f.Message)
	}

	if len(findings) > 0 {
		os.Exit(1)
	}
}

// collectGoFiles resolves an argument into a list of .go files. A plain
// file path is returned as-is; a directory is walked recursively,
// skipping vendor directories and anything hidden.
func collectGoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(path, ".go") {
			return []string{path}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name != "." && (name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}
