// Package config loads application configuration from environment variables
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
}

// Load reads .env (if present) and returns the resolved config
func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("[config] no .env file found, relying on real environment variables")
	}

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("PORT"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("[config] DATABASE_URL is not set")
	}
	if cfg.Port == "" {
		cfg.Port = "8080" // sensible default so local dev works without extra .env setup
	}

	return cfg
}
