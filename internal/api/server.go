package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

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
	// Set up routes before starting the server
	s.router = s.RegisterRoutes()
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

func (s *Server) transferCredit(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check sender's balance
	var senderBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", req.FromUserID).Scan(&senderBalance)
	if err != nil {
		http.Error(w, "Sender not found", http.StatusNotFound)
		return
	}

	if senderBalance < req.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Update sender's balance
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", req.Amount, req.FromUserID)
	if err != nil {
		http.Error(w, "Failed to update sender balance", http.StatusInternalServerError)
		return
	}

	// Update receiver's balance
	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", req.Amount, req.ToUserID)
	if err != nil {
		http.Error(w, "Failed to update receiver balance", http.StatusInternalServerError)
		return
	}

	// Record the transaction
	_, err = tx.Exec(`
		INSERT INTO transactions (from_user_id, to_user_id, amount, type)
		VALUES ($1, $2, $3, 'transfer')`,
		req.FromUserID, req.ToUserID, req.Amount)
	if err != nil {
		http.Error(w, "Failed to record transaction", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) withdrawCredit(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromPath(r)
	var req models.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check balance
	var balance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&balance)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if balance < req.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Update balance
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", req.Amount, userID)
	if err != nil {
		http.Error(w, "Failed to update balance", http.StatusInternalServerError)
		return
	}

	// Record withdrawal transaction
	_, err = tx.Exec(`
		INSERT INTO transactions (to_user_id, amount, type)
		VALUES ($1, $2, 'withdrawal')`,
		userID, -req.Amount)
	if err != nil {
		http.Error(w, "Failed to record transaction", http.StatusInternalServerError)
		return
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) getBalanceAtTime(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromPath(r)
	timestamp := r.URL.Query().Get("timestamp")
	if timestamp == "" {
		http.Error(w, "Timestamp is required", http.StatusBadRequest)
		return
	}

	parsedTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		http.Error(w, "Invalid timestamp format. Use RFC3339", http.StatusBadRequest)
		return
	}

	// Get current balance and subtract all transactions after the specified time
	query := `
		SELECT COALESCE(
			(SELECT balance + (
				SELECT COALESCE(SUM(CASE 
					WHEN type = 'withdrawal' THEN amount
					WHEN from_user_id = $1 THEN -amount
					WHEN to_user_id = $1 THEN amount
					ELSE 0
				END), 0)
				FROM transactions 
				WHERE (from_user_id = $1 OR to_user_id = $1)
				AND created_at > $2
			)
			FROM users WHERE id = $1),
			0
		) as balance_at_time`

	var balanceAtTime float64
	err = s.db.QueryRow(query, userID, parsedTime).Scan(&balanceAtTime)
	if err != nil {
		http.Error(w, "Failed to get balance", http.StatusInternalServerError)
		return
	}

	response := struct {
		UserID    int64     `json:"user_id"`
		Balance   float64   `json:"balance"`
		Timestamp time.Time `json:"timestamp"`
	}{
		UserID:    userID,
		Balance:   balanceAtTime,
		Timestamp: parsedTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
