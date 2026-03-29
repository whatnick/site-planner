package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/whatnick/site-planner/internal/cadastre"
	"github.com/whatnick/site-planner/internal/config"
	"github.com/whatnick/site-planner/internal/geocode"
	"github.com/whatnick/site-planner/internal/handler"
	"github.com/whatnick/site-planner/internal/imagery"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize service clients
	geocoder, err := geocode.NewClient(cfg.GoogleAPIKey)
	if err != nil {
		log.Fatalf("Failed to create geocoder: %v", err)
	}

	cadastreProvider := cadastre.NewSAProvider()
	imager := imagery.NewClient(cfg.GoogleAPIKey)

	// Locate templates directory relative to the binary or working directory
	templateDir := findTemplateDir()

	h, err := handler.New(geocoder, cadastreProvider, imager, templateDir)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Serve static files
	staticDir := filepath.Join(filepath.Dir(templateDir), "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 90 * time.Second, // PDF generation can take time
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Starting server on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
	log.Println("Server stopped")
}

// findTemplateDir locates the templates directory.
// It checks: working directory, then relative to the source file.
func findTemplateDir() string {
	// Check working directory
	if info, err := os.Stat("templates"); err == nil && info.IsDir() {
		abs, _ := filepath.Abs("templates")
		return abs
	}

	// Check relative to source file (for go run)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Join(filepath.Dir(filename), "..", "..", "templates")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(dir)
			return abs
		}
	}

	log.Fatal("Could not find templates directory. Run from the project root.")
	return ""
}
