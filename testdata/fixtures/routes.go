package fixtures

import "net/http"

func setup() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", usersHandler)
	mux.HandleFunc("/users", usersHandlerV2)
	mux.HandleFunc("GET /items/{id}", itemHandler)
	mux.HandleFunc("GET /items/{name}", itemHandler2)
	mux.HandleFunc("users/missing-slash", missingSlash)
	mux.HandleFunc("/double//slash", doubled)
	mux.HandleFunc("", emptyPattern)
}
