package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aristath/gollama-ui/internal/client"
	"github.com/aristath/gollama-ui/internal/handlers"
	"github.com/aristath/gollama-ui/internal/modelmanager"
	"github.com/aristath/gollama-ui/internal/server"
)

func main() {
	var (
		host        = flag.String("host", "0.0.0.0", "Server host")
		port        = flag.String("port", "3000", "Server port")
		ollamaURL   = flag.String("ollama", "http://localhost:8080", "llama.cpp server URL")
		ddgsURL     = flag.String("ddgs", "http://localhost:8000", "ddgs search service URL")
		sentinelURL = flag.String("sentinel", "http://localhost:8081", "Sentinel portfolio API URL")
		staticDir   = flag.String("static", "./web", "Static files directory")
		configDir   = flag.String("config", "./config", "Configuration directory")
		chatTimeout = flag.Duration("chat-timeout", 24*time.Hour, "Chat request timeout (e.g., 1h, 24h, 48h) - default 24h for slow hardware like RPi")
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

	// Initialize llama.cpp client
	ollamaClient, err := client.New(*ollamaURL)
	if err != nil {
		log.Fatalf("Failed to create llama.cpp client: %v", err)
	}

	// Initialize search and news clients for web search and news reading
	searchClient := client.NewSearchClient(*ddgsURL)
	customFeedsPath := filepath.Join(*configDir, "custom-feeds.json")
	newsClient := client.NewNewsClient(customFeedsPath)

	// Initialize Sentinel portfolio client
	sentinelClient := client.NewSentinelClient(*sentinelURL)

	// Initialize tool settings for managing which tools are enabled
	toolSettingsPath := filepath.Join(*configDir, "tool-settings.json")
	toolSettings := handlers.NewToolSettings(toolSettingsPath)

	// Initialize chat timeout settings for dynamic timeout adjustment
	chatTimeoutSettingsPath := filepath.Join(*configDir, "chat-timeout-settings.json")
	chatTimeoutSettings := handlers.NewChatTimeoutSettings(chatTimeoutSettingsPath)

	// Use the stored timeout if available, otherwise use the flag
	effectiveTimeout := chatTimeoutSettings.GetDuration()
	if effectiveTimeout == 0 {
		effectiveTimeout = *chatTimeout
	}

	// Health check ddgs service on startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := searchClient.HealthCheck(ctx); err != nil {
		log.Printf("Warning: ddgs service unavailable: %v", err)
		log.Printf("Web search and news functionality will be disabled")
	} else {
		log.Printf("ddgs service OK: %s", *ddgsURL)
	}

	// Health check Sentinel service on startup
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sentinelClient.HealthCheck(ctx); err != nil {
		log.Printf("Warning: Sentinel service unavailable: %v", err)
		log.Printf("Portfolio analysis functionality will be disabled")
	} else {
		log.Printf("Sentinel service OK: %s", *sentinelURL)
	}

	// Initialize tool executor for function calling
	toolExecutor := handlers.NewToolExecutor(searchClient, newsClient, sentinelClient, toolSettings)

	// Initialize handlers
	modelsHandler := handlers.NewModelsHandler(ollamaClient)
	chatHandler := handlers.NewChatHandlerWithTimeout(ollamaClient, toolExecutor, effectiveTimeout)
	unloadHandler := handlers.NewUnloadHandler(ollamaClient)
	settingsHandler := handlers.NewSettingsHandler(newsClient, toolSettings)
	settingsHandler.SetChatTimeoutSettings(chatTimeoutSettings)

	// Initialize model manager for model switching
	manager := modelmanager.New(
		"/mnt/nvme/llm/models",                // Models directory
		"/mnt/nvme/llm/config/llama-server.conf",     // Config file path
		*ollamaURL,                    // Base URL for health checks
	)
	loadHandler := handlers.NewLoadHandler(manager)

	// Create server
	srv := server.New(modelsHandler, chatHandler, unloadHandler, loadHandler, settingsHandler, absStaticDir)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%s", *host, *port)
	log.Printf("Starting server on %s", addr)
	log.Printf("llama.cpp URL: %s", *ollamaURL)
	log.Printf("ddgs search URL: %s", *ddgsURL)
	log.Printf("Sentinel API URL: %s", *sentinelURL)
	log.Printf("Chat timeout: %v", *chatTimeout)
	log.Printf("Serving static files from: %s", absStaticDir)

	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}