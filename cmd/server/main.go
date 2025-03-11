package main

import (
	"ledger/internal/api"
	"ledger/internal/db"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	log.Printf("DB_HOST: %s", os.Getenv("DB_HOST"))
	log.Printf("DB_PORT: %s", os.Getenv("DB_PORT"))
	log.Printf("DB_USER: %s", os.Getenv("DB_USER"))
	log.Printf("DB_NAME: %s", os.Getenv("DB_NAME"))

	db, err := db.InitDB()
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}
	defer db.Close()

	server := api.NewServer(db, log.New(os.Stdout, "", log.LstdFlags))

	log.Printf("Server starting on :%s", os.Getenv("SERVER_PORT"))
	if err := server.Start(":" + os.Getenv("SERVER_PORT")); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
