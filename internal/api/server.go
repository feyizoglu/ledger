package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ledger/internal/models"
	"ledger/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	db     *sql.DB
	router *chi.Mux
	logger *log.Logger
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func NewServer(db *sql.DB, logger *log.Logger) *Server {
	if err := db.Ping(); err != nil {
		logger.Printf("Database connection error: %v", err)
		return nil
	}
	logger.Printf("Successfully connected to database")
	return &Server{
		db:     db,
		router: chi.NewRouter(),
		logger: logger,
	}
}

func (s *Server) Start(addr string) error {
	// Set up routes before starting the server
	s.router = s.RegisterRoutes()
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	// Read and log the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Printf("Error reading body: %v", err)
		http.Error(w, "Error reading request", http.StatusBadRequest)
		return
	}
	s.logger.Printf("Received request body: %s", string(body))

	// Restore the body
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Printf("Error decoding JSON: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.Name == "" {
		s.logger.Printf("Missing required fields: name=%s, email=%s", req.Name, req.Email)
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Set default role if not provided
	if req.Role == "" {
		req.Role = "user"
	}

	// Validate role
	if req.Role != "user" && req.Role != "admin" {
		s.logger.Printf("Invalid role: %s", req.Role)
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}

	s.logger.Printf("Attempting to create user: name=%s, email=%s, role=%s", req.Name, req.Email, req.Role)

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Printf("Error hashing password: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		s.logger.Printf("Error beginning transaction: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Insert user
	query := `
		INSERT INTO users (name, email, password_hash, role, balance)
		VALUES ($1, $2, $3, $4, 0)
		RETURNING id`

	var userID int
	err = tx.QueryRow(query, req.Name, req.Email, string(hashedPassword), req.Role).Scan(&userID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			s.logger.Printf("PostgreSQL error: %s, Detail: %s, Code: %s", pqErr.Message, pqErr.Detail, pqErr.Code)
		} else {
			s.logger.Printf("Database error: %v", err)
		}
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		s.logger.Printf("Error committing transaction: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      userID,
		"message": "User created successfully",
	})
}

func (s *Server) addCredit(w http.ResponseWriter, r *http.Request) {
	userID, err := utils.GetUserIDFromPath(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

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
	err = s.db.QueryRow(query, req.Amount, userID).Scan(&newBalance)
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
	userID, err := utils.GetUserIDFromPath(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var balance float64
	err = s.db.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&balance)
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

func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromUserID int     `json:"from_user_id"`
		ToUserID   int     `json:"to_user_id"`
		Amount     float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user_id from token
	userID := r.Header.Get("user_id")
	userRole := r.Header.Get("user_role")

	// Convert userID from string to int
	tokenUserID, err := strconv.Atoi(userID)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check if user has permission to transfer
	if userRole != "admin" && tokenUserID != req.FromUserID {
		http.Error(w, "Unauthorized to transfer from this account", http.StatusForbidden)
		return
	}

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Check if from_user has sufficient balance
	var fromUserBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", req.FromUserID).Scan(&fromUserBalance)
	if err != nil {
		http.Error(w, "Failed to get user balance", http.StatusInternalServerError)
		return
	}

	if fromUserBalance < req.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}

	// Update balances
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", req.Amount, req.FromUserID)
	if err != nil {
		http.Error(w, "Failed to update from_user balance", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", req.Amount, req.ToUserID)
	if err != nil {
		http.Error(w, "Failed to update to_user balance", http.StatusInternalServerError)
		return
	}

	// Record the transaction
	_, err = tx.Exec(`
		INSERT INTO transactions (from_user_id, to_user_id, amount, type)
		VALUES ($1, $2, $3, 'transfer')
	`, req.FromUserID, req.ToUserID, req.Amount)
	if err != nil {
		http.Error(w, "Failed to record transaction", http.StatusInternalServerError)
		return
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Transfer successful",
	})
}

func (s *Server) withdrawCredit(w http.ResponseWriter, r *http.Request) {
	userID, err := utils.GetUserIDFromPath(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
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
	userID, err := utils.GetUserIDFromPath(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
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

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user from database
	var user struct {
		ID           int
		PasswordHash string
		Role         string
	}
	err := s.db.QueryRow("SELECT id, password_hash, role FROM users WHERE email = $1",
		req.Email).Scan(&user.ID, &user.PasswordHash, &user.Role)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Verify password
	if !utils.CheckPasswordHash(req.Password, user.PasswordHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}
