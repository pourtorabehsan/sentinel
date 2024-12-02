package api

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewRouter(handler *Handler) *mux.Router {
	r := mux.NewRouter()

	// Add middleware for logging, CORS, etc.
	r.Use(loggingMiddleware)

	// Register routes
	handler.RegisterRoutes(r)

	return r
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add proper logging
		next.ServeHTTP(w, r)
	})
}
