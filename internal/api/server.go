package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"ledger/internal/models"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	db     *sql.DB
	router *chi.Mux
	logger *log.Logger
}

func NewServer(db *sql.DB) *Server {
	return &Server{
		db:     db,
		router: chi.NewRouter(),
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func getUserIDFromPath(r *http.Request) int64 {
	id := chi.URLParam(r, "id")
	userID, _ := strconv.ParseInt(id, 10, 64)
	return userID
}

func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.logger.Printf("Attempting to create user with name: %s", req.Name)

	if err := s.db.Ping(); err != nil {
		s.logger.Printf("Database connection error: %v", err)
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	var user models.User
	query := "INSERT INTO users (name, balance) VALUES ($1, 0) RETURNING id, name, balance"
	s.logger.Printf("Executing query: %s", query)

	err := s.db.QueryRow(query, req.Name).Scan(&user.ID, &user.Name, &user.Balance)
	if err != nil {
		s.logger.Printf("Database error creating user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	s.logger.Printf("Successfully created user: %+v", user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) addCredit(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromPath(r)

	var req models.AddCreditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users 
		SET balance = balance + $1 
		WHERE id = $2 
		RETURNING balance`

	var newBalance float64
	err := s.db.QueryRow(query, req.Amount, userID).Scan(&newBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]float64{"balance": newBalance})
}

func (s *Server) getUserBalance(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromPath(r)

	var balance float64
	err := s.db.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]float64{"balance": balance})
}

func (s *Server) getAllBalances(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, name, balance FROM users")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var balances []models.UserBalance
	for rows.Next() {
		var balance models.UserBalance
		if err := rows.Scan(&balance.UserID, &balance.Name, &balance.Balance); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		balances = append(balances, balance)
	}

	json.NewEncoder(w).Encode(balances)
}
