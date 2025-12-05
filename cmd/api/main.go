package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	httphandlers "docs-as-code/internal/http"
)

func main() {
	// Initialize dependencies
	// In a real application, these would be database connections, actual converters, etc.
	// For now, they are nil/placeholder implementations.
	var store interface{} // Placeholder for DocumentStore
	var converter interface{} // Placeholder for Converter

	// Create the HTTP server with injected dependencies
	server := httphandlers.NewServer(nil, nil)

	// Set up the chi router
	router := chi.NewRouter()

	// Define API routes
	router.Post("/v1/documents", server.handleCreateDocument)
	router.Get("/v1/documents/{id}", server.handleGetDocumentStatus)

	// Start the HTTP server
	addr := ":8080"
	fmt.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
