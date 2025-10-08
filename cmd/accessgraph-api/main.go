package main

import (
	"log"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jamesolaitan/accessgraph/internal/api/graphql"
	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

func main() {
	cfg := config.Load()

	// Enable offline mode if configured
	if cfg.Offline {
		config.EnableOfflineMode()
		log.Println("Mode: OFFLINE (no network egress). Capabilities: read-only.")
	}

	// Initialize store
	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	// Create GraphQL resolver
	resolver := graphql.NewResolver(st, cfg)

	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// GraphQL endpoint
	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: resolver}))
	r.Handle("/query", srv)

	// Playground for development
	r.Handle("/", playground.Handler("AccessGraph GraphQL", "/query"))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Printf("Server starting on :%s", cfg.Port)
	log.Printf("GraphQL playground: http://localhost:%s/", cfg.Port)
	log.Printf("GraphQL endpoint: http://localhost:%s/query", cfg.Port)

	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
