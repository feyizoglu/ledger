package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	// Credit-related routes
	r.Post("/api/users", s.createUser)
	r.Post("/api/users/{id}/credit", s.addCredit)
	r.Get("/api/users/{id}/balance", s.getUserBalance)
	r.Get("/api/users/balances", s.getAllBalances)

	return r
}
