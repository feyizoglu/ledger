package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ledger/internal/models"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("Error loading .env file: %v", err)
	}

	connStr := "host=localhost port=5432 user=tolgahan.feyizoglu dbname=ledger_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Clean up test data
	_, err = db.Exec("TRUNCATE users CASCADE")
	if err != nil {
		t.Fatalf("Failed to clean test database: %v", err)
	}

	return db
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)

	tests := []struct {
		name           string
		payload        models.CreateUserRequest
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "Valid User",
			payload:        models.CreateUserRequest{Name: "John Doe"},
			expectedStatus: http.StatusCreated,
			expectedError:  false,
		},
		{
			name:           "Empty Name",
			payload:        models.CreateUserRequest{Name: ""},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Failed to marshal request payload: %v", err)
			}

			req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.createUser(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.expectedError {
				var response models.User
				err = json.NewDecoder(w.Body).Decode(&response)
				if err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if response.Name != tt.payload.Name {
					t.Errorf("Expected name %s, got %s", tt.payload.Name, response.Name)
				}

				if response.ID == 0 {
					t.Error("Expected non-zero ID")
				}
			}
		})
	}
}

func TestAddCredit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)

	// First create a user
	user := createTestUser(t, server)

	tests := []struct {
		name           string
		userID         int64
		amount         float64
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "Valid Credit Addition",
			userID:         user.ID,
			amount:         100.50,
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:           "Invalid User ID",
			userID:         999,
			amount:         50.00,
			expectedStatus: http.StatusNotFound,
			expectedError:  true,
		},
		{
			name:           "Negative Amount",
			userID:         user.ID,
			amount:         -50.00,
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"amount": tt.amount,
			}
			payloadBytes, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/api/users/"+fmt.Sprint(tt.userID)+"/credit", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.addCredit(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetUserBalance(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)
	user := createTestUser(t, server)

	// Add some initial credit
	addTestCredit(t, server, user.ID, 100.00)

	tests := []struct {
		name            string
		userID          int64
		expectedStatus  int
		expectedBalance float64
		expectedError   bool
	}{
		{
			name:            "Valid User",
			userID:          user.ID,
			expectedStatus:  http.StatusOK,
			expectedBalance: 100.00,
			expectedError:   false,
		},
		{
			name:            "Invalid User",
			userID:          999,
			expectedStatus:  http.StatusNotFound,
			expectedBalance: 0,
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/users/"+fmt.Sprint(tt.userID)+"/balance", nil)
			w := httptest.NewRecorder()

			server.getUserBalance(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if !tt.expectedError {
				var response map[string]float64
				json.NewDecoder(w.Body).Decode(&response)
				if response["balance"] != tt.expectedBalance {
					t.Errorf("Expected balance %.2f, got %.2f", tt.expectedBalance, response["balance"])
				}
			}
		})
	}
}

func TestGetAllBalances(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	server := NewServer(db)

	// Create multiple users with different balances
	user1 := createTestUser(t, server)
	user2 := createTestUser(t, server)

	addTestCredit(t, server, user1.ID, 100.00)
	addTestCredit(t, server, user2.ID, 200.00)

	req := httptest.NewRequest("GET", "/api/users/balances", nil)
	w := httptest.NewRecorder()

	server.getAllBalances(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response []models.UserBalance
	json.NewDecoder(w.Body).Decode(&response)

	if len(response) != 2 {
		t.Errorf("Expected 2 balances, got %d", len(response))
	}
}

// Helper functions
func createTestUser(t *testing.T, server *Server) models.User {
	payload := models.CreateUserRequest{Name: "Test User"}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.createUser(w, req)

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)
	return user
}

func addTestCredit(t *testing.T, server *Server, userID int64, amount float64) {
	payload := map[string]interface{}{"amount": amount}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/api/users/"+fmt.Sprint(userID)+"/credit", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.addCredit(w, req)
}
