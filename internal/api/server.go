package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"ledger/internal/models"
	"go.uber.org/zap"
)

type Server struct {
	db     *sql.DB
	router *mux.Router
	logger *zap.Logger
}

func NewServer(db *sql.DB) *Server {
	logger, _ := zap.NewProduction()
	s := &Server{
		db:     db,
		router: mux.NewRouter(),
		logger: logger,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.HandleFunc("/api/users", s.createUser).Methods("POST")
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	var user models.User
	err := s.db.QueryRow(
		"INSERT INTO users (name) VALUES ($1) RETURNING id, name, created_at",
		req.Name,
	).Scan(&user.ID, &user.Name, &user.CreatedAt)

	if err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
