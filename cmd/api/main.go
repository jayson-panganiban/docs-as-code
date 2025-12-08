package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"docs-as-code/internal/converter"
	httphandlers "docs-as-code/internal/http"
	"docs-as-code/internal/store"
)

func main() {
	// Initialize dependencies
	documentStore := store.NewMemoryStore()
	pdfConverter := converter.NewPDFConverter()

	// Create the HTTP server with injected dependencies
	server := httphandlers.NewServer(documentStore, pdfConverter)

	// Set up the chi router
	router := chi.NewRouter()

	// Define API routes
	router.Post("/v1/documents", server.HandleCreateDocument)
	router.Get("/v1/documents/{id}", server.HandleGetDocumentStatus)

	// Start the HTTP server
	addr := ":8080"
	fmt.Printf("Starting server on %s\n", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
