package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func InitDB() (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_NAME"),
	)

	log.Printf("Connecting to database with connection string: %s", connStr)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	log.Printf("Successfully connected to database")

	// Create tables if they don't exist
	if err = createTables(db); err != nil {
		return nil, fmt.Errorf("error creating tables: %v", err)
	}

	log.Printf("Tables created successfully")

	return db, nil
}

func createTables(db *sql.DB) error {
	log.Printf("Starting to create tables...")

	query := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'admin')),
			balance DECIMAL(10,2) DEFAULT 0.00,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS transactions (
			id SERIAL PRIMARY KEY,
			from_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			to_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(10,2) NOT NULL,
			type VARCHAR(20) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT valid_transaction_type CHECK (type IN ('credit', 'transfer', 'withdrawal')),
			CONSTRAINT valid_transaction CHECK (
				(type = 'credit' AND from_user_id IS NULL) OR
				(type = 'transfer' AND from_user_id IS NOT NULL) OR
				(type = 'withdrawal' AND from_user_id IS NULL)
			)
		);

		CREATE INDEX IF NOT EXISTS idx_transactions_users ON transactions(from_user_id, to_user_id);
		CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Error creating tables: %v", err)
		return err
	}

	log.Printf("Tables created successfully")
	return nil
}
