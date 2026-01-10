package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aristath/gollama-ui/internal/client"
	"github.com/aristath/gollama-ui/internal/handlers"
	"github.com/aristath/gollama-ui/internal/server"
)

func main() {
	var (
		host      = flag.String("host", "0.0.0.0", "Server host")
		port      = flag.String("port", "8080", "Server port")
		ollamaURL = flag.String("ollama", "http://localhost:11434", "Ollama server URL")
		staticDir = flag.String("static", "./web", "Static files directory")
	)
	flag.Parse()

	// Validate static directory exists
	absStaticDir, err := filepath.Abs(*staticDir)
	if err != nil {
		log.Fatalf("Failed to resolve static directory: %v", err)
	}

	if info, err := os.Stat(absStaticDir); err != nil || !info.IsDir() {
		log.Fatalf("Static directory does not exist: %s", absStaticDir)
	}

	// Initialize Ollama client
	ollamaClient, err := client.New(*ollamaURL)
	if err != nil {
		log.Fatalf("Failed to create Ollama client: %v", err)
	}

	// Initialize handlers
	modelsHandler := handlers.NewModelsHandler(ollamaClient)
	chatHandler := handlers.NewChatHandler(ollamaClient)
	unloadHandler := handlers.NewUnloadHandler(ollamaClient)

	// Create server
	srv := server.New(modelsHandler, chatHandler, unloadHandler, absStaticDir)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Starting server on %s", addr)
	log.Printf("Ollama URL: %s", *ollamaURL)
	log.Printf("Serving static files from: %s", absStaticDir)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}