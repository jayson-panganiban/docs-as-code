package http

import (
	"encoding/json"
	"net/http"

	"docs-as-code/internal/converter"
	"docs-as-code/internal/store"
)

// Server holds the service dependencies for the HTTP handlers.
type Server struct {
	store     store.DocumentStore
	converter converter.Converter
}

// NewServer creates a new HTTP server with the provided dependencies.
func NewServer(store store.DocumentStore, converter converter.Converter) *Server {
	return &Server{
		store:     store,
		converter: converter,
	}
}

// handleCreateDocument handles the POST /v1/documents endpoint.
func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     "placeholder-job-id",
		"status": "pending",
	})
}

// handleGetDocumentStatus handles the GET /v1/documents/{id} endpoint.
func (s *Server) handleGetDocumentStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     "placeholder-job-id",
		"status": "completed",
	})
}
