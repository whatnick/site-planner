package config

import (
	"fmt"
	"os"
)

type Config struct {
	GoogleAPIKey string
	OpenAIAPIKey string
	SAMApiURL    string // optional SAM segmentation endpoint
	Port         string
}

func Load() (*Config, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "WARNING: GOOGLE_API_KEY not set. Server will start but PDF generation will fail.")
	}

	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		fmt.Fprintln(os.Stderr, "INFO: OPENAI_API_KEY not set. AI structure detection will be unavailable.")
	}

	samURL := os.Getenv("SAM_API_URL")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		GoogleAPIKey: apiKey,
		OpenAIAPIKey: openAIKey,
		SAMApiURL:    samURL,
		Port:         port,
	}, nil
}
