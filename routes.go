package main

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// Route is a single route registration found in source.
type Route struct {
	File    string
	Line    int
	Method  string // the Go method name used to register it, e.g. HandleFunc, Get
	Pattern string
}

// Finding is a single problem reported by a check, tied to a location.
type Finding struct {
	File    string
	Line    int
	Message string
}

// routeMethods are the call names we treat as route registrations. This
// covers net/http's Handle/HandleFunc and the verb methods used by most
// chi/gorilla-style routers. It's a coarse heuristic, not a type check:
// we don't try to resolve whether the receiver is actually a router.
var routeMethods = map[string]bool{
	"Handle":     true,
	"HandleFunc": true,
	"Get":        true,
	"Post":       true,
	"Put":        true,
	"Delete":     true,
	"Patch":      true,
	"Head":       true,
	"Options":    true,
	"Connect":    true,
	"Trace":      true,
}

// extractRoutes walks a parsed file looking for calls of the form
// receiver.Method("pattern", handler, ...) where Method is a known route
// method and the first argument is a string literal.
func extractRoutes(fset *token.FileSet, file *ast.File, filename string) []Route {
	var routes []Route

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !routeMethods[sel.Sel.Name] {
			return true
		}
		// A route registration needs at least a pattern and a handler.
		if len(call.Args) < 2 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		pattern, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		pos := fset.Position(lit.Pos())
		routes = append(routes, Route{
			File:    filename,
			Line:    pos.Line,
			Method:  sel.Sel.Name,
			Pattern: pattern,
		})
		return true
	})

	return routes
}

// checkPatterns runs the built-in static checks against a set of routes
// gathered from one or more files and returns findings sorted by
// file and line.
func checkPatterns(routes []Route) []Finding {
	var findings []Finding

	seen := make(map[string][]Route)
	for _, r := range routes {
		key := r.Method + " " + r.Pattern
		seen[key] = append(seen[key], r)

		if r.Pattern == "" {
			findings = append(findings, Finding{r.File, r.Line, "empty route pattern"})
			continue
		}
		if !strings.HasPrefix(r.Pattern, "/") {
			findings = append(findings, Finding{
				r.File, r.Line,
				"route pattern " + strconv.Quote(r.Pattern) + " does not start with /",
			})
		}
		if strings.Contains(r.Pattern, "//") {
			findings = append(findings, Finding{
				r.File, r.Line,
				"route pattern " + strconv.Quote(r.Pattern) + " contains a doubled slash",
			})
		}
	}

	for key, dupes := range seen {
		if len(dupes) < 2 {
			continue
		}
		sort.Slice(dupes, func(i, j int) bool { return dupes[i].Line < dupes[j].Line })
		first := dupes[0]
		for _, dup := range dupes[1:] {
			findings = append(findings, Finding{
				dup.File, dup.Line,
				"duplicate route " + strconv.Quote(key) + ", first registered at " +
					first.File + ":" + strconv.Itoa(first.Line),
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	return findings
}
