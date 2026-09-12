# routelint

A static linter for URL route registrations in Go HTTP services.

Route tables tend to grow by copy-paste: someone duplicates a
`mux.HandleFunc("/users/create", ...)` line to add `/users/update` and
forgets to change the pattern, or a pattern loses its leading slash after
a refactor and silently stops matching. `net/http`'s `ServeMux` won't
catch a duplicate pattern spread across two files until you register both
at runtime (and then it panics), and a malformed pattern like `"users"`
(no leading slash) just never matches anything, with no error at all.

routelint reads your source with `go/parser`, finds calls that look like
route registrations, and reports problems with file and line numbers
before any of this reaches production.

## What it looks for

- Duplicate patterns registered under the same method, even across files
- Patterns missing a leading `/`
- Patterns containing a doubled `/`
- Empty patterns
- Malformed Go 1.22 `ServeMux` patterns: a pattern with no path at all, or
  a host segment that contains `{`

It recognizes `Handle` and `HandleFunc` (the `net/http` and
`http.ServeMux` style) plus the verb methods used by chi, gorilla/mux and
similar routers: `Get`, `Post`, `Put`, `Delete`, `Patch`, `Head`,
`Options`, `Connect`, `Trace`. It matches on method name and call shape,
not on the receiver's type, so it works without needing to type-check
against your actual router dependency.

It also understands Go 1.22's enhanced `ServeMux` pattern syntax,
`[METHOD ][HOST]/PATH`, so a pattern like `"GET /users/{id}"` is checked
against its path (`/users/{id}`), not the literal string. Two patterns
that only differ by call style, e.g. one file using
`mux.HandleFunc("GET /users", ...)` and another using
`mux.Handle("GET /users", ...)`, are still recognized as the same
duplicate route. A pattern with no method prefix that doesn't start with
`/`, such as `"users/update"`, is flagged as before; routelint parses the
part before the first `/` as a host under 1.22 rules, so the message
notes that `"users"` is being read as a host rather than assuming it's
always a typo.

## Usage

```
go build -o routelint .
./routelint ./...
```

or point it at specific files or directories:

```
./routelint internal/api
./routelint internal/api/routes.go
```

Given:

```go
func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/users", listUsers)
	mux.HandleFunc("/users/create", createUser)
	mux.HandleFunc("users/update", updateUser)
	mux.HandleFunc("/users", listUsersV2)
}
```

routelint reports:

```
internal/api/routes.go:3: route pattern "users/update" does not start with /
internal/api/routes.go:4: duplicate route "HandleFunc /users", first registered at internal/api/routes.go:1
```

Exit status is 1 if any findings were reported, 0 otherwise, so it can
be wired into CI as a plain build step.

## Status

Early. The checks above are deliberately simple string checks; there's
no understanding yet of path parameters (`/users/{id}`) for overlap
detection, or route groups/mounts that prefix a whole set of children
with a shared path.

## License

MIT, see LICENSE.
