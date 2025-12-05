package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// Response represents a standard API response.
type Response struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// HandleCreateDocument handles the POST /v1/documents endpoint.
// It expects a multipart form with a "file" field containing a markdown file.
// Returns 202 Accepted with the job ID and pending status.
func (s *Server) HandleCreateDocument(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form with 10MB max size
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "failed to parse form: " + err.Error()})
		return
	}

	// Retrieve the uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "file not provided or invalid"})
		return
	}
	defer file.Close()

	// Create a document job
	jobID, err := s.store.CreateDocumentJob(r.Context(), file)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: "failed to create job: " + err.Error()})
		return
	}

	// Return 202 Accepted with job details
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(Response{
		ID:     jobID,
		Status: "pending",
	})
}

// HandleGetDocumentStatus handles the GET /v1/documents/{id} endpoint.
// Returns the current status of a document conversion job.
func (s *Server) HandleGetDocumentStatus(w http.ResponseWriter, r *http.Request) {
	// Extract the job ID from the URL parameter
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{Error: "missing job id"})
		return
	}

	// Retrieve the job from the store
	job, err := s.store.GetDocumentJob(r.Context(), jobID)
	if err != nil {
		// Check if it's a not found error
		if errors.Is(err, errors.New("job not found: "+jobID)) || 
		   (err != nil && errors.Is(err, err)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(Response{Error: "job not found"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Error: "failed to retrieve job: " + err.Error()})
		return
	}

	// Return the job status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		ID:     job.ID,
		Status: job.Status,
	})
}
