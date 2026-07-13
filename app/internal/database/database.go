package database

import (
	"fmt"
	"database/sql"
	"os"
	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect() (*sql.DB, error){

	err := godotenv.Load()
	if err != nil{
		return nil, fmt.Errorf("failed to load .env: %w", err)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	db, err := sql.Open("pgx",dsn)
	if err != nil{
		return nil, err
	}

	err = db.Ping()
	if err != nil{
		return nil, err
	}

	fmt.Println("Connected to PostgreSQL!")

	return db, nil
}

