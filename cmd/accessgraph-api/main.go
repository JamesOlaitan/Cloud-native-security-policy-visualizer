package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jamesolaitan/accessgraph/internal/api/graphql"
	"github.com/jamesolaitan/accessgraph/internal/config"
	redactlog "github.com/jamesolaitan/accessgraph/internal/log"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

var (
	healthy int32 = 1
	ready   int32 = 1
)

func main() {
	// Set the default logger to redact sensitive data (AWS account IDs, ARNs, secrets)
	log.SetOutput(&redactlog.RedactWriter{Out: os.Stderr})

	cfg := config.Load()

	// Configure network mode (IMDS always blocked for security)
	if cfg.Offline {
		config.EnableOfflineMode(true)
		log.Println("Mode: OFFLINE (no network egress, IMDS blocked)")
	} else {
		config.EnableOnlineModeWithIMDSBlock()
		log.Println("Mode: ONLINE (IMDS blocked for security)")
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
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(requestSizeLimiter(10 * 1024 * 1024)) // 10MB limit

	// CORS configuration
	corsOptions := cors.Options{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}

	if cfg.DevMode {
		corsOptions.AllowedOrigins = []string{"http://localhost:3000", "http://localhost:*"}
		corsOptions.AllowCredentials = true
		log.Println("CORS: DEV mode - allowing localhost origins")
	} else if cfg.CORSAllowedOrigins != "" {
		corsOptions.AllowedOrigins = strings.Split(cfg.CORSAllowedOrigins, ",")
		log.Printf("CORS: Allowing origins: %s", cfg.CORSAllowedOrigins)
	} else {
		log.Println("CORS: Disabled (no origins allowed)")
	}

	if len(corsOptions.AllowedOrigins) > 0 {
		r.Use(cors.Handler(corsOptions))
	}

	// GraphQL endpoint
	srv := handler.NewDefaultServer(graphql.NewExecutableSchema(graphql.Config{Resolvers: resolver}))
	r.Handle("/query", srv)

	// GraphQL Playground is only available in development mode to avoid
	// exposing an interactive query interface in production environments.
	if cfg.DevMode {
		r.Handle("/", playground.Handler("AccessGraph GraphQL", "/query"))
	}

	// Health and observability endpoints
	r.Get("/healthz", healthzHandler)
	r.Get("/healthz/live", livenessHandler)
	r.Get("/healthz/ready", readinessHandler)
	r.Get("/metrics", metricsHandler)

	// HTTP server configuration
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server starting on :%s", cfg.Port)
		log.Printf("Mode: %s", logMode(cfg))
		if cfg.DevMode {
			log.Printf("GraphQL playground: http://localhost:%s/", cfg.Port)
		}
		log.Printf("GraphQL endpoint: http://localhost:%s/query", cfg.Port)
		log.Printf("Health endpoint: http://localhost:%s/healthz", cfg.Port)
		log.Printf("Metrics endpoint: http://localhost:%s/metrics", cfg.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server shutting down gracefully...")

	// Mark as not ready during shutdown
	atomic.StoreInt32(&ready, 0)

	// Give the server 30 seconds to finish ongoing requests
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		os.Exit(1)
	}

	log.Println("Server stopped")
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 && atomic.LoadInt32(&ready) == 1 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Not Ready"))
	}
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Alive"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Not Alive"))
	}
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&ready) == 1 {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("Not Ready"))
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Simple Prometheus-compatible metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	healthStatus := float64(atomic.LoadInt32(&healthy))
	readyStatus := float64(atomic.LoadInt32(&ready))

	fmt.Fprintf(w, "# HELP accessgraph_healthy Indicates if the service is healthy (1 = healthy, 0 = unhealthy)\n")
	fmt.Fprintf(w, "# TYPE accessgraph_healthy gauge\n")
	fmt.Fprintf(w, "accessgraph_healthy %.0f\n", healthStatus)

	fmt.Fprintf(w, "# HELP accessgraph_ready Indicates if the service is ready (1 = ready, 0 = not ready)\n")
	fmt.Fprintf(w, "# TYPE accessgraph_ready gauge\n")
	fmt.Fprintf(w, "accessgraph_ready %.0f\n", readyStatus)

	fmt.Fprintf(w, "# HELP accessgraph_info Service information\n")
	fmt.Fprintf(w, "# TYPE accessgraph_info gauge\n")
	fmt.Fprintf(w, "accessgraph_info{version=\"1.1.0\"} 1\n")
}

func requestSizeLimiter(maxBytes int64) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func logMode(cfg *config.Config) string {
	if cfg.Offline {
		return "OFFLINE (no network egress, IMDS blocked)"
	}
	return "ONLINE (IMDS blocked for security)"
}
