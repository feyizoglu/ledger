package api

import (
	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes() *chi.Mux {
	r := chi.NewRouter()

	r.Post("/api/users", s.createUser)
	r.Post("/api/users/{id}/credit", s.addCredit)
	r.Get("/api/users/{id}/balance", s.getUserBalance)
	r.Get("/api/users/balances", s.getAllBalances)

	r.Post("/api/transfers", s.transferCredit)
	r.Post("/api/users/{id}/withdraw", s.withdrawCredit)

	r.Get("/api/users/{id}/balance-at-time", s.getBalanceAtTime)

	return r
}
