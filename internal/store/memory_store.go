package store

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
)

// MemoryStore is an in-memory implementation of DocumentStore.
// It is thread-safe and suitable for development and testing.
type MemoryStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemoryStore creates a new in-memory document store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs: make(map[string]*Job),
	}
}

// CreateDocumentJob creates a new document conversion job and stores it in memory.
// It reads the source file into memory, generates a unique job ID, and returns it.
func (ms *MemoryStore) CreateDocumentJob(ctx context.Context, sourceFile io.Reader) (string, error) {
	// Generate unique job ID
	jobID := uuid.New().String()

	// Read the source file into memory (in production, this would be stored in S3, etc.)
	data, err := io.ReadAll(sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	if len(data) == 0 {
		return "", fmt.Errorf("source file is empty")
	}

	// Create the job
	job := &Job{
		ID:     jobID,
		Status: "pending",
	}

	// Store in memory
	ms.mu.Lock()
	ms.jobs[jobID] = job
	ms.mu.Unlock()

	return jobID, nil
}

// GetDocumentJob retrieves a job by ID.
// Returns an error if the job is not found.
func (ms *MemoryStore) GetDocumentJob(ctx context.Context, id string) (Job, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	job, exists := ms.jobs[id]
	if !exists {
		return Job{}, fmt.Errorf("job not found: %s", id)
	}

	return *job, nil
}

// UpdateJobStatus updates the status of a job. Used internally for transitions.
func (ms *MemoryStore) UpdateJobStatus(ctx context.Context, id string, status string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	job, exists := ms.jobs[id]
	if !exists {
		return fmt.Errorf("job not found: %s", id)
	}

	job.Status = status
	return nil
}
