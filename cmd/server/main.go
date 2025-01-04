package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"ledger/internal/api"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://tolgahan.feyizoglu@localhost:5432/ledger?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error opening database: ", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Error connecting to the database: ", err)
	}

	log.Printf("Successfully connected to database")

	server := api.NewServer(db)
	router := server.RegisterRoutes()

	log.Printf("Server starting on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}
