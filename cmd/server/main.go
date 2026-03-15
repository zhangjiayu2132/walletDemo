package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wallet/internal/api"
	"wallet/internal/wallet"
)

func main() {
	// Initialize the core service
	svc := wallet.NewService()

	// Initialize the HTTP handler with the service
	handler := api.NewHandler(svc)

	// Set up the router
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Configure the HTTP server
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Channel to listen for errors from the server
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		log.Printf("wallet service listening on port %s", port)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for interrupt signals for graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block main and wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		log.Fatalf("error starting server: %v", err)
	case sig := <-shutdown:
		log.Printf("starting shutdown... received signal: %v", sig)

		// Create context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			if err := srv.Close(); err != nil {
				log.Fatalf("could not stop http server: %v", err)
			}
		}
	}

	log.Println("wallet service stopped")
}
