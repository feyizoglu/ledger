package api

import (
	"ledger/internal/middleware"

	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Public routes (no auth required)
	r.Post("/api/login", s.login)
	r.Post("/api/users", s.createUser) // Allow signup without auth

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware)

		// User routes (protected by OwnerOrAdmin)
		r.Group(func(r chi.Router) {
			r.Use(middleware.OwnerOrAdmin)
			r.Get("/api/users/{id}/balance", s.getUserBalance)
			r.Post("/api/users/{id}/withdraw", s.withdrawCredit)
			r.Get("/api/users/{id}/balance-at-time", s.getBalanceAtTime)
		})

		// Admin routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AdminOnly)
			r.Get("/api/users/balances", s.getAllBalances)
			r.Post("/api/users/{id}/credit", s.addCredit)
		})

		// Transfer requires special handling
		r.Post("/api/transfer", func(w http.ResponseWriter, r *http.Request) {
			s.transfer(w, r)
		})

		// Protected routes
		r.Get("/api/users/{id}/balance", func(w http.ResponseWriter, r *http.Request) {
			s.getUserBalance(w, r)
		})
		r.Post("/api/users/{id}/credit", func(w http.ResponseWriter, r *http.Request) {
			s.addCredit(w, r)
		})
	})

	return r
}
