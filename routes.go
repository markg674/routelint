package main

import (
	"fmt"
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

// parseMuxPattern splits a route pattern into the components defined by Go
// 1.22's enhanced http.ServeMux syntax: an optional "METHOD ", an optional
// host, and a path. A pre-1.22 pattern like "/users" parses as method="",
// host="", path="/users", so callers don't need to special-case old style
// patterns. It returns an error for the same cases ServeMux itself rejects
// at registration time: no path at all, or a host containing "{".
func parseMuxPattern(pattern string) (method, host, path string, err error) {
	rest := pattern
	if i := strings.IndexAny(pattern, " \t"); i >= 0 {
		candidate := pattern[:i]
		if isHTTPMethod(candidate) {
			method = candidate
			rest = strings.TrimLeft(pattern[i+1:], " \t")
		}
	}

	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return method, "", rest, fmt.Errorf("no path found (missing /)")
	}
	host = rest[:slash]
	path = rest[slash:]
	if strings.Contains(host, "{") {
		return method, host, path, fmt.Errorf("host %s contains '{', did you forget the leading /?", strconv.Quote(host))
	}
	return method, host, path, nil
}

// isHTTPMethod reports whether s looks like an HTTP method token, e.g. GET
// or CONNECT. It's deliberately restricted to uppercase letters and hyphens
// so an ordinary lowercase path segment before a space is never mistaken
// for a method.
func isHTTPMethod(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z') && c != '-' {
			return false
		}
	}
	return true
}

// looksLikeHost reports whether a host component extracted from a pattern
// resembles an actual hostname, as opposed to a path segment that ended up
// there because the pattern is missing its leading slash.
func looksLikeHost(host string) bool {
	if host == "localhost" {
		return true
	}
	return strings.Contains(host, ".") || strings.Contains(host, ":") || strings.HasPrefix(host, "*.")
}

// checkPatterns runs the built-in static checks against a set of routes
// gathered from one or more files and returns findings sorted by
// file and line.
func checkPatterns(routes []Route) []Finding {
	var findings []Finding

	seen := make(map[string][]Route)
	for _, r := range routes {
		if r.Pattern == "" {
			findings = append(findings, Finding{r.File, r.Line, "empty route pattern"})
			continue
		}

		method, host, path, err := parseMuxPattern(r.Pattern)
		if err != nil {
			findings = append(findings, Finding{
				r.File, r.Line,
				"malformed route pattern " + strconv.Quote(r.Pattern) + ": " + err.Error(),
			})
			continue
		}

		// A route method embedded in the pattern (Go 1.22+ ServeMux syntax)
		// is what net/http actually keys duplicates on; fall back to the Go
		// call name (Get, Post, HandleFunc, ...) for routers that carry the
		// verb there instead.
		effectiveMethod := method
		if effectiveMethod == "" {
			effectiveMethod = r.Method
		}
		key := effectiveMethod + " " + host + path
		seen[key] = append(seen[key], r)

		if host != "" && !looksLikeHost(host) {
			findings = append(findings, Finding{
				r.File, r.Line,
				"route pattern " + strconv.Quote(r.Pattern) + " does not start with /" +
					" (" + strconv.Quote(host) + " is parsed as a host, not part of the path)",
			})
		}
		if strings.Contains(path, "//") {
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
