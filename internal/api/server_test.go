package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"ledger/internal/models"
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
