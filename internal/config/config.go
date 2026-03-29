package config

import (
	"fmt"
	"os"
)

type Config struct {
	GoogleAPIKey string
	Port         string
}

func Load() (*Config, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "WARNING: GOOGLE_API_KEY not set. Server will start but PDF generation will fail.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		GoogleAPIKey: apiKey,
		Port:         port,
	}, nil
}
