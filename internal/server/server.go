package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/aristath/gollama-ui/internal/handlers"
)

// Server holds the HTTP server and dependencies
type Server struct {
	router        *chi.Mux
	modelsHandler *handlers.ModelsHandler
	chatHandler   *handlers.ChatHandler
	unloadHandler *handlers.UnloadHandler
	staticDir     string
}

// New creates a new server instance
func New(modelsHandler *handlers.ModelsHandler, chatHandler *handlers.ChatHandler, unloadHandler *handlers.UnloadHandler, staticDir string) *Server {
	s := &Server{
		router:        chi.NewRouter(),
		modelsHandler: modelsHandler,
		chatHandler:   chatHandler,
		unloadHandler: unloadHandler,
		staticDir:     staticDir,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// CORS middleware - allow all origins for local development
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Request logging
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	// API routes - must be registered before catch-all
	// Order matters: more specific routes first
	s.router.Route("/api", func(r chi.Router) {
		r.Post("/models/{model}/unload", s.unloadHandler.Unload)
		r.Get("/models", s.modelsHandler.List)
		r.Post("/chat", s.chatHandler.Stream)
	})

	// Serve static files - root path serves index.html
	s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// For root path, serve index.html
		if r.URL.Path == "/" {
			http.ServeFile(w, r, s.staticDir+"/index.html")
			return
		}
		
		// Serve other static files
		fs := http.FileServer(http.Dir(s.staticDir))
		fs.ServeHTTP(w, r)
	})
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}