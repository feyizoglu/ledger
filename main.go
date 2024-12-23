package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"ledger/internal/api"
	"ledger/internal/db"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	database, err := db.InitDB()
	if err != nil {
		log.Fatal("Error initializing database:", err)
	}
	defer database.Close()

	server := api.NewServer(database)
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := server.Start(":" + port); err != nil {
		log.Fatal("Error starting server:", err)
	}
}
